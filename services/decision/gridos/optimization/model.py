from collections.abc import Callable
from dataclasses import dataclass
from math import isfinite

import highspy

from gridos.fallback.planner import (
    DeviceSchedule,
    DeviceState,
    Exclusion,
    PlanningInterval,
    ScheduleInterval,
    Shortfall,
    effective_reserve_kwh,
    exclusion_reason,
)


@dataclass(frozen=True, slots=True)
class OptimizedPlan:
    schedules: tuple[DeviceSchedule, ...]
    exclusions: tuple[Exclusion, ...]
    shortfalls: tuple[Shortfall, ...]
    fallback: bool = False


def optimize(devices: list[DeviceState], intervals: list[PlanningInterval]) -> OptimizedPlan:
    if any(not isfinite(item.target_kw) or item.target_kw < 0 for item in intervals):
        raise ValueError("target must be finite and nonnegative")
    if any(not isfinite(item.duration_hours) or item.duration_hours <= 0 for item in intervals):
        raise ValueError("duration must be finite and positive")
    eligible: list[DeviceState] = []
    exclusions: list[Exclusion] = []
    seen: set[str] = set()
    for device in devices:
        if device.device_id in seen:
            exclusions.append(Exclusion(device.device_id, "DUPLICATE_DEVICE"))
            continue
        seen.add(device.device_id)
        reason = exclusion_reason(device)
        if reason is None:
            eligible.append(device)
        else:
            exclusions.append(Exclusion(device.device_id, reason))
    solver_type: Callable[[], highspy.Highs] = highspy.Highs
    solver = solver_type()
    solver.setOptionValue("output_flag", False)
    index: dict[tuple[int, int], int] = {}
    for device_index, device in enumerate(eligible):
        service_limit = max(0.0, device.max_discharge_kw - device.home_load_kw)
        for interval_index in range(len(intervals)):
            column = solver.getNumCol()
            index[device_index, interval_index] = column
            solver.addCol(
                1.0 + (1.0 - device.availability_probability), 0.0, service_limit, 0, [], []
            )
    shortfall_indices: list[int] = []
    for interval in intervals:
        shortfall_indices.append(solver.getNumCol())
        solver.addCol(1000.0, 0.0, interval.target_kw, 0, [], [])
    for device_index, device in enumerate(eligible):
        service_limit = max(0.0, device.max_discharge_kw - device.home_load_kw)
        if service_limit == 0.0:
            continue
        discharge_ratio = device.max_discharge_kw / service_limit
        columns = [index[device_index, interval_index] for interval_index in range(len(intervals))]
        coefficients = [
            interval.duration_hours * discharge_ratio / device.discharge_efficiency
            for interval in intervals
        ]
        solver.addRow(
            0.0,
            max(0.0, device.energy_kwh - effective_reserve_kwh(device)),
            len(columns),
            columns,
            coefficients,
        )
    for interval_index, interval in enumerate(intervals):
        columns = [index[device_index, interval_index] for device_index in range(len(eligible))]
        coefficients = [device.availability_probability for device in eligible]
        columns.append(shortfall_indices[interval_index])
        coefficients.append(1.0)
        solver.addRow(interval.target_kw, interval.target_kw, len(columns), columns, coefficients)
    solver.run()
    if solver.getModelStatus() != highspy.HighsModelStatus.kOptimal:
        raise RuntimeError(f"optimizer status: {solver.getModelStatus()}")
    values = solver.getSolution().col_value
    schedules: list[DeviceSchedule] = []
    for device_index, device in enumerate(eligible):
        service_limit = max(0.0, device.max_discharge_kw - device.home_load_kw)
        energy = device.energy_kwh
        planned: list[ScheduleInterval] = []
        for interval_index, interval in enumerate(intervals):
            service = max(0.0, values[index[device_index, interval_index]])
            discharge = service * device.max_discharge_kw / service_limit if service_limit else 0.0
            energy -= discharge * interval.duration_hours / device.discharge_efficiency
            planned.append(ScheduleInterval(service, discharge, energy))
        if any(item.grid_service_kw > 1e-9 for item in planned):
            schedules.append(DeviceSchedule(device.device_id, tuple(planned)))
    shortfalls: list[Shortfall] = []
    for interval_index, interval in enumerate(intervals):
        allocated = sum(
            values[index[device_index, interval_index]] for device_index in range(len(eligible))
        )
        expected = sum(
            device.availability_probability * values[index[device_index, interval_index]]
            for device_index, device in enumerate(eligible)
        )
        shortfalls.append(
            Shortfall(
                interval.target_kw,
                allocated,
                expected,
                max(0.0, values[shortfall_indices[interval_index]]),
            )
        )
    return OptimizedPlan(tuple(schedules), tuple(exclusions), tuple(shortfalls))
