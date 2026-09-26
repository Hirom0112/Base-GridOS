from math import isfinite
from time import monotonic

import grpc

from gridos.fallback.planner import (
    DeviceState,
    FallbackPlan,
    PlanningInterval,
    plan_fallback,
)
from gridos.v1 import dispatch_pb2, optimization_pb2, telemetry_pb2
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
    eligible_ids = set(request.eligibility_snapshot.eligible_device_ids)
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
    plan: FallbackPlan,
) -> optimization_pb2.OptimizeResponse:
    by_id = {device.device_id: device for device in devices}
    response = optimization_pb2.OptimizeResponse()
    response.plan.plan_id = f"{request.request_id}-fallback"
    response.plan.event_id = request.event_id
    response.plan.plan_version = request.plan_version
    response.plan.fallback_used = True
    response.plan.fallback_reason = "DETERMINISTIC_FALLBACK"
    response.plan.solver_version = "fallback"
    response.plan.model_version = "1"
    response.plan.created_at.CopyFrom(request.requested_at)
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
        output_shortfall.feasible_kw = shortfall.allocated_kw
        output_shortfall.shortfall_kw = shortfall.shortfall_kw
        if shortfall.shortfall_kw > 0.0:
            output_shortfall.reasons.append("INSUFFICIENT_FEASIBLE_CAPACITY")
    return response


class OptimizationServer:
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
            started = monotonic()
            plan = plan_fallback(devices, intervals)
        except ValueError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
        if monotonic() - started > budget:
            context.abort(grpc.StatusCode.DEADLINE_EXCEEDED, "optimization budget exceeded")
        violations = validate_plan(plan, devices, intervals)
        if violations:
            context.abort(grpc.StatusCode.INTERNAL, "fallback validation failed")
        return _response(request, devices, plan)
