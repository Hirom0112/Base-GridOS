import argparse
import csv
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from math import isfinite
from pathlib import Path
from zoneinfo import ZoneInfo

import grpc

from gridos.fallback.planner import (
    DeviceState,
    PlanningInterval,
    plan_fallback,
)
from gridos.forecasting.availability import ReliabilityTrait, forecast_availability
from gridos.forecasting.load import LoadObservation, forecast_load
from gridos.optimization.model import OptimizedPlan, optimize
from gridos.solver.bounded import Decision, Planner, resolve, solve_within_budget
from gridos.v1 import device_pb2, dispatch_pb2, optimization_pb2, telemetry_pb2
from gridos.validation.plan import validate_plan

_PUBLIC = Path(__file__).resolve().parents[3] / "testdata/fixtures/public"
_CENTRAL = ZoneInfo("America/Chicago")
_TRAITS: dict[str, ReliabilityTrait] = {"HIGH": "HIGH", "MEDIUM": "MEDIUM", "LOW": "LOW"}


def _site_load_history(profile: str) -> list[LoadObservation]:
    path = _PUBLIC / "load-profiles/residential-week.csv"
    if not path.exists():
        return []
    observations: list[LoadObservation] = []
    with path.open(newline="") as source:
        for row in csv.DictReader(source):
            if row["PType_WZ"] != profile:
                continue
            day = datetime.strptime(row["Date"], "%Y-%m-%d %H:%M:%S").replace(tzinfo=_CENTRAL)
            for slot in range(96):
                value = row[f"int_kWh{slot + 1}"]
                if value:
                    observations.append(
                        LoadObservation(day + timedelta(minutes=15 * slot), float(value))
                    )
    return observations


def _site_load_at(
    observations: list[LoadObservation], begin: datetime, end: datetime
) -> tuple[float, float, float]:
    total = lower = upper = 0.0
    current = begin.astimezone(_CENTRAL)
    local_end = end.astimezone(_CENTRAL)
    while current < local_end:
        midnight = current.replace(hour=0, minute=0, second=0, microsecond=0)
        daily = forecast_load(
            [item for item in observations if item.timestamp < midnight],
            midnight,
            timedelta(days=1),
        )
        slot = current.hour * 4 + current.minute // 15
        next_slot = midnight + timedelta(minutes=15 * (slot + 1))
        segment_end = min(next_slot, local_end)
        fraction = (segment_end - current).total_seconds() / 900
        total += daily.values_kwh[slot] * fraction
        lower += daily.lower_kwh[slot] * fraction
        upper += daily.upper_kwh[slot] * fraction
        current = segment_end
    return total, lower, upper


def _day_ahead_prices() -> dict[tuple[str, datetime], float]:
    path = _PUBLIC / "ercot-prices/dam-spp-week.csv"
    if not path.exists():
        return {}
    prices: dict[tuple[str, datetime], float] = {}
    with path.open(newline="") as source:
        for row in csv.DictReader(source):
            day = datetime.strptime(row["Delivery Date"], "%m/%d/%Y").replace(tzinfo=_CENTRAL)
            hour = int(row["Hour Ending"].split(":", 1)[0]) - 1
            prices[row["Settlement Point"], day + timedelta(hours=hour)] = float(
                row["Settlement Point Price"]
            )
    return prices


def _forecast_value(
    bounds: tuple[float, float, float],
    feature: str,
    model: str,
    kind: str,
    provenance: device_pb2.DataProvenance,
) -> optimization_pb2.ForecastValue:
    return optimization_pb2.ForecastValue(
        value=bounds[0],
        lower=bounds[1],
        upper=bounds[2],
        feature_version=feature,
        model_version=model,
        value_kind=kind,
        provenance=provenance,
    )


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
            reserve_percent=device.effective_reserve_kwh / device.usable_energy_kwh * 100.0
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


def _exclusion_reason(reason: str) -> dispatch_pb2.ExclusionReason:
    return {
        "RESERVE": dispatch_pb2.EXCLUSION_REASON_RESERVE,
        "STALE_TELEMETRY": dispatch_pb2.EXCLUSION_REASON_STALE_TELEMETRY,
        "UNAVAILABLE": dispatch_pb2.EXCLUSION_REASON_UNAVAILABLE,
        "MAINTENANCE_LOCK": dispatch_pb2.EXCLUSION_REASON_MAINTENANCE_LOCK,
    }.get(reason, dispatch_pb2.EXCLUSION_REASON_UNSPECIFIED)


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
    def __init__(self, solver: Planner = optimize) -> None:
        self._solver = solver

    def Forecast(
        self,
        wrapper: optimization_pb2.ForecastRequest,
        context: grpc.ServicerContext,
    ) -> optimization_pb2.ForecastResponse:
        if not wrapper.HasField("request"):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "request is required")
        request = wrapper.request
        if not request.HasField("requested_at") or not request.HasField("budget"):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "forecast time and budget are required")
        _duration_seconds(request)
        issued_at = request.requested_at.ToDatetime(tzinfo=UTC)
        response = optimization_pb2.ForecastResponse()
        histories: dict[str, list[LoadObservation]] = {}
        loads: dict[tuple[str, datetime, datetime], tuple[float, float, float] | None] = {}
        prices = _day_ahead_prices()
        for interval in request.intervals:
            if not interval.HasField("begin_time") or not interval.HasField("end_time"):
                context.abort(grpc.StatusCode.INVALID_ARGUMENT, "forecast interval is required")
            begin = interval.begin_time.ToDatetime(tzinfo=UTC)
            end = interval.end_time.ToDatetime(tzinfo=UTC)
            if end <= begin:
                context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT, "forecast interval must be positive"
                )
            seen_zones: set[str] = set()
            for site in request.sites:
                if site.load_profile_type not in histories:
                    histories[site.load_profile_type] = _site_load_history(site.load_profile_type)
                history = histories[site.load_profile_type]
                key = (site.load_profile_type, begin, end)
                if key not in loads:
                    try:
                        loads[key] = _site_load_at(history, begin, end) if history else None
                    except ValueError:
                        loads[key] = None
                predicted_load = loads[key]
                if predicted_load is None:
                    response.unavailable_sources.append(f"site_load:{site.site_id}")
                else:
                    load_output = response.site_loads.add(site_id=site.site_id)
                    load_output.interval_begin_time.CopyFrom(interval.begin_time)
                    load_output.load_kwh.CopyFrom(
                        _forecast_value(
                            predicted_load,
                            "similar-day-weekday-v1",
                            "load-baseline-v1",
                            "modeled_estimate",
                            device_pb2.DATA_PROVENANCE_DERIVED,
                        )
                    )
                if site.load_zone in seen_zones:
                    continue
                seen_zones.add(site.load_zone)
                price = prices.get(
                    (
                        site.load_zone,
                        begin.astimezone(_CENTRAL).replace(minute=0, second=0, microsecond=0),
                    )
                )
                if price is None:
                    response.unavailable_sources.append(f"regional_price:{site.load_zone}")
                else:
                    price_output = response.regional_prices.add(load_zone=site.load_zone)
                    price_output.interval_begin_time.CopyFrom(interval.begin_time)
                    price_output.price_per_mwh.CopyFrom(
                        _forecast_value(
                            (price, price, price),
                            "ercot-dam-spp-v1",
                            "day-ahead-v1",
                            "confirmed_public_forward",
                            device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC,
                        )
                    )
                response.unavailable_sources.append(f"outage_risk:{site.county or site.site_id}")
            for device in request.devices:
                trait = _TRAITS.get(device.reliability_trait)
                if trait is None or not device.HasField("telemetry_observed_at"):
                    response.unavailable_sources.append(f"device_availability:{device.device_id}")
                    continue
                age = issued_at - device.telemetry_observed_at.ToDatetime(tzinfo=UTC)
                if age < timedelta(0):
                    response.unavailable_sources.append(f"device_availability:{device.device_id}")
                    continue
                prediction = forecast_availability(trait, 0, 0, age)
                availability_output = response.device_availability.add(device_id=device.device_id)
                availability_output.interval_begin_time.CopyFrom(interval.begin_time)
                availability_output.probability.CopyFrom(
                    _forecast_value(
                        (
                            prediction.availability_probability,
                            prediction.availability_probability,
                            prediction.availability_probability,
                        ),
                        "reliability-freshness-v1",
                        "availability-baseline-v1",
                        "modeled_estimate",
                        device_pb2.DATA_PROVENANCE_DERIVED,
                    )
                )
        return response

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
        outcome = solve_within_budget(self._solver, devices, intervals, budget)
        return _response(request, devices, resolve(outcome, fallback, devices, intervals))


def _port(value: str) -> int:
    port = int(value)
    if not 1 <= port <= 65_535:
        raise argparse.ArgumentTypeError("port must be in [1, 65535]")
    return port


def serve(port: int) -> None:
    server = grpc.server(ThreadPoolExecutor())
    optimize_handler = grpc.unary_unary_rpc_method_handler(
        OptimizationServer().Optimize,
        request_deserializer=optimization_pb2.OptimizeRequest.FromString,
        response_serializer=optimization_pb2.OptimizeResponse.SerializeToString,
    )
    forecast_handler = grpc.unary_unary_rpc_method_handler(
        OptimizationServer().Forecast,
        request_deserializer=optimization_pb2.ForecastRequest.FromString,
        response_serializer=optimization_pb2.ForecastResponse.SerializeToString,
    )
    server.add_generic_rpc_handlers(
        (
            grpc.method_handlers_generic_handler(
                "gridos.v1.OptimizationService",
                {"Optimize": optimize_handler, "Forecast": forecast_handler},
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
