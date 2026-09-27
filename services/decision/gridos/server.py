import argparse
import os
from concurrent.futures import ThreadPoolExecutor
from decimal import Decimal
from math import isfinite

import grpc

from gridos.economics.margin import eligible_additional_capacity
from gridos.fallback.planner import (
    DeviceSchedule,
    DeviceState,
    FallbackPlan,
    PlanningInterval,
    ScheduleInterval,
    plan_fallback,
)
from gridos.fallback.replacement import replace_dropped
from gridos.forecasting.serve import forecast_response
from gridos.optimization.model import OptimizedPlan, optimize
from gridos.solver.bounded import Decision, Planner, resolve, solve_within_budget
from gridos.v1 import device_pb2, dispatch_pb2, optimization_pb2, telemetry_pb2
from gridos.validation.plan import validate_plan


def _duration_seconds(request: optimization_pb2.OptimizationRequest) -> float:
    budget = float(request.budget.seconds + request.budget.nanos / 1_000_000_000)
    if not isfinite(budget) or budget <= 0.0:
        raise ValueError("budget must be positive")
    return budget


def _planning_intervals(
    request: optimization_pb2.OptimizationRequest,
) -> list[PlanningInterval]:
    intervals: list[PlanningInterval] = []
    for interval in request.intervals:
        begin = interval.begin_time.seconds + interval.begin_time.nanos / 1_000_000_000
        end = interval.end_time.seconds + interval.end_time.nanos / 1_000_000_000
        intervals.append(PlanningInterval(interval.target_kw, (end - begin) / 3600.0))
    return intervals


def _device_states(request: optimization_pb2.OptimizationRequest) -> list[DeviceState]:
    if (
        not isfinite(request.conservative_margin)
        or not isfinite(request.margin_hurdle)
        or request.margin_hurdle < 0
    ):
        raise ValueError("margin bounds must be finite and hurdle nonnegative")
    margin = (
        Decimal(str(request.conservative_margin))
        if request.conservative_margin != 0
        else _conservative_public_margin(request)
    )
    eligible_ids = set(request.eligibility_snapshot.eligible_device_ids)
    known_ids = {device.device_id for device in request.devices}
    forecast_availability_by_id: dict[str, float] = {}
    for prediction in request.forecast.device_availability:
        probability = prediction.probability.value
        if (
            prediction.device_id not in known_ids
            or not isfinite(probability)
            or not 0 <= probability <= 1
            or not prediction.probability.feature_version
            or not prediction.probability.model_version
            or prediction.probability.value_kind != "modeled_estimate"
        ):
            raise ValueError("invalid device availability forecast")
        forecast_availability_by_id[prediction.device_id] = min(
            forecast_availability_by_id.get(prediction.device_id, 1.0), probability
        )
    return [
        DeviceState(
            device_id=device.device_id,
            usable_energy_kwh=device.usable_energy_kwh,
            energy_kwh=device.energy_kwh,
            reserve_percent=_selected_reserve_kwh(request, device, margin)
            / device.usable_energy_kwh
            * 100.0
            if device.usable_energy_kwh > 0.0
            else 0.0,
            hardware_floor_percent=device.hardware_floor_kwh / device.usable_energy_kwh * 100.0
            if device.usable_energy_kwh > 0.0
            else 0.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=device.max_discharge_kw,
            discharge_efficiency=device.discharge_efficiency,
            home_load_kw=0.0,
            availability_probability=forecast_availability_by_id.get(
                device.device_id, device.availability_probability
            ),
            available=not eligible_ids or device.device_id in eligible_ids,
            stale=device.stale,
        )
        for device in request.devices
    ]


def _selected_reserve_kwh(
    request: optimization_pb2.OptimizationRequest,
    device: optimization_pb2.DeviceState,
    margin: Decimal,
) -> float:
    base = (
        device.base_reserve_kwh
        if device.HasField("base_reserve_kwh")
        else device.effective_reserve_kwh
    )
    if not isfinite(base) or base < device.hardware_floor_kwh:
        raise ValueError("base reserve must honor the hardware floor")
    selected = base
    if device.HasField("travel_flex_reserve_kwh"):
        flex = device.travel_flex_reserve_kwh
        if (
            not device.HasField("base_reserve_kwh")
            or not isfinite(flex)
            or not device.hardware_floor_kwh <= flex <= base
        ):
            raise ValueError("active Travel Flex reserve must be between hardware and base reserve")
        available = Decimal(str(base - flex))
        selected -= float(
            eligible_additional_capacity(
                available,
                margin,
                Decimal(str(request.margin_hurdle)),
            )
        )
    if (
        not isfinite(device.effective_reserve_kwh)
        or abs(device.effective_reserve_kwh - selected) > 1e-9
    ):
        raise ValueError("effective reserve does not match margin-gated reserve")
    return selected


def _conservative_public_margin(request: optimization_pb2.OptimizationRequest) -> Decimal:
    intervals = {(item.begin_time.seconds, item.begin_time.nanos) for item in request.intervals}
    prices: dict[tuple[str, int, int], Decimal] = {}
    for forecast in request.forecast.regional_prices:
        value = forecast.price_per_mwh
        begin = (forecast.interval_begin_time.seconds, forecast.interval_begin_time.nanos)
        if (
            begin not in intervals
            or not forecast.load_zone
            or not value.model_version
            or value.value_kind != "confirmed_public_forward"
            or value.provenance != device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC
        ):
            continue
        if not all(isfinite(number) for number in (value.lower, value.value, value.upper)):
            raise ValueError("public price forecast must be finite")
        if not value.lower <= value.value <= value.upper:
            raise ValueError("public price forecast bounds must be ordered")
        key = (forecast.load_zone, *begin)
        lower = Decimal(str(value.lower))
        prices[key] = min(prices.get(key, lower), lower)
    margin = Decimal(0)
    for device in request.devices:
        if not device.HasField("travel_flex_reserve_kwh") or not device.HasField(
            "base_reserve_kwh"
        ):
            continue
        zone_prices = [lower for (zone, _, _), lower in prices.items() if zone == device.load_zone]
        if not zone_prices:
            continue
        lower = min(zone_prices)
        if lower < 0:
            incremental_kwh = Decimal(str(device.base_reserve_kwh)) - Decimal(
                str(device.travel_flex_reserve_kwh)
            )
            margin += lower * incremental_kwh * Decimal(str(device.discharge_efficiency)) / 1000
    return margin


def _exclusion_reason(reason: str) -> dispatch_pb2.ExclusionReason:
    return {
        "RESERVE": dispatch_pb2.EXCLUSION_REASON_RESERVE,
        "STALE_TELEMETRY": dispatch_pb2.EXCLUSION_REASON_STALE_TELEMETRY,
        "UNAVAILABLE": dispatch_pb2.EXCLUSION_REASON_UNAVAILABLE,
        "MAINTENANCE_LOCK": dispatch_pb2.EXCLUSION_REASON_MAINTENANCE_LOCK,
    }.get(reason, dispatch_pb2.EXCLUSION_REASON_UNSPECIFIED)


def _margin_explanation(
    request: optimization_pb2.OptimizationRequest,
) -> optimization_pb2.MarginExplanation:
    explicit = request.conservative_margin != 0
    public_margin = _conservative_public_margin(request) if not explicit else Decimal(0)
    margin = Decimal(str(request.conservative_margin)) if explicit else public_margin
    explanation = optimization_pb2.MarginExplanation(
        conservative_margin=float(margin), margin_hurdle=request.margin_hurdle
    )
    dispatch = explanation.terms.add(name="DISPATCH_VALUE", low=float(margin), high=float(margin))
    if explicit:
        dispatch.source = "FROZEN_REQUEST_MARGIN"
    elif public_margin < 0:
        dispatch.source = "FROZEN_PUBLIC_PRICE"
    else:
        dispatch.source = "UNAVAILABLE"
        dispatch.unavailable = True
    for name in (
        "AVOIDED_PEAK_COST",
        "COMMITMENT_RELIABILITY_VALUE",
        "CHARGING_ENERGY",
        "INCREMENTAL_DEGRADATION",
        "PENALTY_EXPOSURE",
        "MEMBER_REWARD",
        "SUPPORT_AND_RISK_COST",
    ):
        explanation.terms.add(name=name, source="UNAVAILABLE", unavailable=True)
    return explanation


def _response(
    request: optimization_pb2.OptimizationRequest,
    devices: list[DeviceState],
    decision: Decision,
) -> optimization_pb2.OptimizeResponse:
    plan = decision.plan
    by_id = {device.device_id: device for device in devices}
    response = optimization_pb2.OptimizeResponse()
    optimized = isinstance(plan, OptimizedPlan)
    response.plan.plan_id = f"{request.request_id}-{'highs' if optimized else 'fallback'}"
    response.plan.event_id = request.event_id
    response.plan.plan_version = request.plan_version
    response.plan.fallback_used = not optimized
    response.plan.fallback_reason = "" if optimized else decision.fallback_reason
    response.plan.solver_version = "highs" if optimized else "fallback"
    response.plan.model_version = "1"
    response.plan.created_at.CopyFrom(request.requested_at)
    response.plan.margin_explanation.CopyFrom(_margin_explanation(request))
    if isinstance(plan, OptimizedPlan):
        response.plan.objective_breakdown.degradation_cost = plan.objective.cycling_cost
        response.plan.objective_breakdown.penalty_exposure = plan.objective.shortfall_penalty
        response.plan.objective_breakdown.reliability_risk_cost = plan.objective.uncertainty_cost
        response.plan.objective_breakdown.objective_value = -plan.objective.total_cost
        for name, value, units in (
            ("RESERVE", plan.margins.reserve_kwh, "kWh"),
            ("DISCHARGE_POWER", plan.margins.power_kw, "kW"),
        ):
            margin = response.plan.constraint_margins.add()
            margin.constraint_name = name
            margin.margin = value
            margin.units = units
    for exclusion in plan.exclusions:
        item = response.plan.exclusions.add()
        item.device_id = exclusion.device_id
        item.reason = _exclusion_reason(exclusion.reason)
        item.detail = exclusion.reason
    for schedule in plan.schedules:
        device = by_id[schedule.device_id]
        output_schedule = response.plan.device_schedules.add()
        output_schedule.device_id = schedule.device_id
        for index, planned in enumerate(schedule.intervals):
            output_interval = output_schedule.intervals.add()
            output_interval.begin_time.CopyFrom(request.intervals[index].begin_time)
            output_interval.end_time.CopyFrom(request.intervals[index].end_time)
            output_interval.setpoint_kw = planned.discharge_kw
            output_interval.expected_energy_kwh = planned.expected_energy_kwh
            output_interval.expected_state_of_energy_percent = (
                planned.expected_energy_kwh / device.usable_energy_kwh * 100.0
                if device.usable_energy_kwh > 0.0
                else 0.0
            )
    for index, shortfall in enumerate(plan.shortfalls):
        output_shortfall = response.plan.shortfalls.add()
        output_shortfall.interval_begin_time.CopyFrom(request.intervals[index].begin_time)
        output_shortfall.interval_end_time.CopyFrom(request.intervals[index].end_time)
        output_shortfall.requested_kw = shortfall.requested_kw
        output_shortfall.feasible_kw = shortfall.expected_kw
        output_shortfall.shortfall_kw = shortfall.shortfall_kw
        if shortfall.shortfall_kw > 0.0:
            output_shortfall.reasons.append("INSUFFICIENT_FEASIBLE_CAPACITY")
    return response


class OptimizationServer:
    def __init__(
        self, solver: Planner = optimize, solver_budget_seconds: float | None = None
    ) -> None:
        if solver_budget_seconds is not None and (
            not isfinite(solver_budget_seconds) or solver_budget_seconds <= 0.0
        ):
            raise ValueError("solver budget must be positive and finite")
        self._solver = solver
        self._solver_budget_seconds = solver_budget_seconds

    def Forecast(
        self,
        wrapper: optimization_pb2.ForecastRequest,
        context: grpc.ServicerContext,
    ) -> optimization_pb2.ForecastResponse:
        return forecast_response(wrapper, context)

    def Optimize(
        self,
        wrapper: optimization_pb2.OptimizeRequest,
        context: grpc.ServicerContext,
    ) -> optimization_pb2.OptimizeResponse:
        if not wrapper.HasField("request"):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "request is required")
        request = wrapper.request
        try:
            budget = _duration_seconds(request)
            if request.measurement_boundary == telemetry_pb2.MEASUREMENT_BOUNDARY_UNSPECIFIED:
                raise ValueError("measurement boundary is required")
            intervals = _planning_intervals(request)
            devices = _device_states(request)
            fallback = plan_fallback(devices, intervals)
        except ValueError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
        if validate_plan(fallback, devices, intervals):
            context.abort(grpc.StatusCode.INTERNAL, "fallback validation failed")
        if "forecast_transport_timeout" in request.forecast.unavailable_sources:
            return _response(request, devices, Decision(fallback, "FORECAST_TIMEOUT"))
        solver_budget = (
            min(budget, self._solver_budget_seconds)
            if self._solver_budget_seconds is not None
            else budget
        )
        outcome = solve_within_budget(self._solver, devices, intervals, solver_budget)
        return _response(request, devices, resolve(outcome, fallback, devices, intervals))

    def Replace(
        self,
        wrapper: optimization_pb2.ReplaceRequest,
        context: grpc.ServicerContext,
    ) -> optimization_pb2.ReplaceResponse:
        if not wrapper.HasField("current") or not wrapper.HasField("approved_plan"):
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "current state and approved plan required"
            )
        request = wrapper.current
        approved = wrapper.approved_plan
        envelope = frozenset(wrapper.envelope_device_ids)
        dropped = frozenset(wrapper.dropped_device_ids)
        scheduled = {schedule.device_id for schedule in approved.device_schedules}
        if (
            not wrapper.idempotency_key
            or not envelope
            or not dropped
            or len(envelope) != len(wrapper.envelope_device_ids)
            or len(dropped) != len(wrapper.dropped_device_ids)
            or not dropped <= scheduled
            or not scheduled <= envelope
            or approved.event_id != request.event_id
        ):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "invalid replacement boundary")
        try:
            intervals = _planning_intervals(request)
            devices = _device_states(request)
            if len({device.device_id for device in devices}) != len(devices):
                raise ValueError("duplicate current device")
            if any(
                len(schedule.intervals) != len(intervals) for schedule in approved.device_schedules
            ):
                raise ValueError("approved schedule length does not match current intervals")
            prior = FallbackPlan(
                schedules=tuple(
                    DeviceSchedule(
                        schedule.device_id,
                        tuple(
                            ScheduleInterval(
                                item.setpoint_kw,
                                item.setpoint_kw,
                                item.expected_energy_kwh,
                            )
                            for item in schedule.intervals
                        ),
                    )
                    for schedule in approved.device_schedules
                ),
                exclusions=(),
                shortfalls=(),
            )
            replacement = replace_dropped(prior, devices, envelope, dropped, intervals)
            targets = [
                PlanningInterval(item.requested_kw, interval.duration_hours)
                for item, interval in zip(replacement.shortfalls, intervals, strict=True)
            ]
            if validate_plan(replacement, devices, targets):
                context.abort(grpc.StatusCode.INTERNAL, "replacement validation failed")
        except ValueError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
        result = _response(request, devices, Decision(replacement, "COHORT_REPLACEMENT"))
        result.plan.plan_id = f"{approved.plan_id}-replacement-{wrapper.idempotency_key}"
        return optimization_pb2.ReplaceResponse(replacement_plan=result.plan)


def _port(value: str) -> int:
    port = int(value)
    if not 1 <= port <= 65_535:
        raise argparse.ArgumentTypeError("port must be in [1, 65535]")
    return port


def _solver_budget_from_env() -> float | None:
    raw = os.getenv("GRIDOS_SOLVER_BUDGET_SECONDS")
    if raw is None:
        return None
    try:
        budget = float(raw)
    except ValueError as error:
        raise ValueError("solver budget must be positive and finite") from error
    if not isfinite(budget) or budget <= 0.0:
        raise ValueError("solver budget must be positive and finite")
    return budget


def serve(port: int) -> None:
    server = grpc.server(ThreadPoolExecutor())
    optimizer = OptimizationServer(solver_budget_seconds=_solver_budget_from_env())
    optimize_handler = grpc.unary_unary_rpc_method_handler(
        optimizer.Optimize,
        request_deserializer=optimization_pb2.OptimizeRequest.FromString,
        response_serializer=optimization_pb2.OptimizeResponse.SerializeToString,
    )
    forecast_handler = grpc.unary_unary_rpc_method_handler(
        optimizer.Forecast,
        request_deserializer=optimization_pb2.ForecastRequest.FromString,
        response_serializer=optimization_pb2.ForecastResponse.SerializeToString,
    )
    replace_handler = grpc.unary_unary_rpc_method_handler(
        optimizer.Replace,
        request_deserializer=optimization_pb2.ReplaceRequest.FromString,
        response_serializer=optimization_pb2.ReplaceResponse.SerializeToString,
    )
    server.add_generic_rpc_handlers(
        (
            grpc.method_handlers_generic_handler(
                "gridos.v1.OptimizationService",
                {
                    "Optimize": optimize_handler,
                    "Forecast": forecast_handler,
                    "Replace": replace_handler,
                },
            ),
        )
    )
    if server.add_insecure_port(f"[::]:{port}") == 0:
        raise RuntimeError(f"could not bind port {port}")
    server.start()
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        server.stop(0).wait()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=_port, required=True)
    arguments = parser.parse_args()
    serve(arguments.port)


if __name__ == "__main__":
    main()
