from dataclasses import dataclass
from math import isfinite

from gridos.fallback.planner import (
    DeviceSchedule,
    DeviceState,
    FallbackPlan,
    PlanningInterval,
    exclusion_reason,
)


@dataclass(frozen=True, slots=True)
class Violation:
    code: str
    device_id: str = ""
    interval_index: int = -1


def _schedule_violations(
    schedule: DeviceSchedule,
    device: DeviceState,
    intervals: list[PlanningInterval],
    tolerance: float,
) -> list[Violation]:
    violations: list[Violation] = []
    ineligible = exclusion_reason(device)
    if ineligible is not None:
        violations.append(Violation(ineligible, device.device_id))
    if len(schedule.intervals) != len(intervals):
        violations.append(Violation("VECTOR_LENGTH", device.device_id))
    previous_energy = device.energy_kwh
    reserve_percent = max(
        device.reserve_percent,
        device.hardware_floor_percent,
        device.dynamic_override_percent,
    )
    reserve_kwh = device.usable_energy_kwh * reserve_percent / 100.0
    for index, (planned, interval) in enumerate(zip(schedule.intervals, intervals, strict=False)):
        values = (
            planned.grid_service_kw,
            planned.discharge_kw,
            planned.expected_energy_kwh,
        )
        if not all(isfinite(value) for value in values):
            violations.append(Violation("NONFINITE_VALUE", device.device_id, index))
            continue
        if planned.discharge_kw < 0.0 or planned.discharge_kw > device.max_discharge_kw + tolerance:
            violations.append(Violation("POWER_BOUND", device.device_id, index))
        export_limit = max(0.0, planned.discharge_kw - device.home_load_kw)
        if planned.grid_service_kw < 0.0 or planned.grid_service_kw > export_limit + tolerance:
            violations.append(Violation("EXPORT_BOUND", device.device_id, index))
        reconstructed = previous_energy - (
            planned.discharge_kw * interval.duration_hours / device.discharge_efficiency
        )
        if abs(reconstructed - planned.expected_energy_kwh) > tolerance:
            violations.append(Violation("ENERGY_BALANCE", device.device_id, index))
        if planned.expected_energy_kwh < reserve_kwh - tolerance:
            violations.append(Violation("RESERVE", device.device_id, index))
        if planned.expected_energy_kwh > device.usable_energy_kwh + tolerance:
            violations.append(Violation("ENERGY_BOUND", device.device_id, index))
        previous_energy = planned.expected_energy_kwh
    return violations


def _shortfall_violations(
    plan: FallbackPlan,
    probabilities: dict[str, float],
    intervals: list[PlanningInterval],
    tolerance: float,
) -> list[Violation]:
    if len(plan.shortfalls) != len(intervals):
        return [Violation("VECTOR_LENGTH")]
    violations: list[Violation] = []
    for index, (shortfall, interval) in enumerate(zip(plan.shortfalls, intervals, strict=True)):
        values = (
            shortfall.requested_kw,
            shortfall.allocated_kw,
            shortfall.expected_kw,
            shortfall.shortfall_kw,
        )
        if not all(isfinite(value) for value in values):
            violations.append(Violation("NONFINITE_VALUE", interval_index=index))
            continue
        allocated = 0.0
        expected = 0.0
        for schedule in plan.schedules:
            if index < len(schedule.intervals):
                grid_service_kw = schedule.intervals[index].grid_service_kw
                allocated += grid_service_kw
                expected += grid_service_kw * probabilities.get(schedule.device_id, 0.0)
        if abs(shortfall.requested_kw - interval.target_kw) > tolerance:
            violations.append(Violation("TARGET_MISMATCH", interval_index=index))
        if abs(shortfall.allocated_kw - allocated) > tolerance:
            violations.append(Violation("ALLOCATION_MISMATCH", interval_index=index))
        if abs(shortfall.expected_kw - expected) > tolerance:
            violations.append(Violation("EXPECTED_MISMATCH", interval_index=index))
        expected_shortfall = max(0.0, interval.target_kw - expected)
        if (
            shortfall.shortfall_kw < 0.0
            or abs(shortfall.shortfall_kw - expected_shortfall) > tolerance
        ):
            violations.append(Violation("SHORTFALL_MISMATCH", interval_index=index))
    return violations


def validate_plan(
    plan: FallbackPlan,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    tolerance: float = 1e-9,
) -> tuple[Violation, ...]:
    if not isfinite(tolerance) or tolerance < 0.0:
        raise ValueError("tolerance must be finite and nonnegative")
    violations: list[Violation] = []
    by_id = {device.device_id: device for device in devices}
    scheduled_ids: set[str] = set()
    for schedule in plan.schedules:
        if schedule.device_id in scheduled_ids:
            violations.append(Violation("DUPLICATE_DEVICE", schedule.device_id))
            continue
        scheduled_ids.add(schedule.device_id)
        device = by_id.get(schedule.device_id)
        if device is None:
            violations.append(Violation("UNKNOWN_DEVICE", schedule.device_id))
            continue
        violations.extend(_schedule_violations(schedule, device, intervals, tolerance))
    probabilities = {device.device_id: device.availability_probability for device in devices}
    violations.extend(_shortfall_violations(plan, probabilities, intervals, tolerance))
    return tuple(violations)
