from collections.abc import Callable
from dataclasses import dataclass
from math import isfinite

import highspy

from gridos.fallback.planner import (
    DeviceSchedule,
    DeviceState,
    Exclusion,
    FallbackPlan,
    PlanningInterval,
    ScheduleInterval,
    Shortfall,
    effective_reserve_kwh,
    exclusion_reason,
    plan_fallback,
)
from gridos.solver.bounded import SolverFault, solve_within_budget


@dataclass(frozen=True, slots=True)
class ObjectiveBreakdown:
    delivered_value: float
    shortfall_penalty: float
    cycling_cost: float
    uncertainty_cost: float
    total_cost: float


@dataclass(frozen=True, slots=True)
class ConstraintMargins:
    reserve_kwh: float
    power_kw: float


@dataclass(frozen=True, slots=True, kw_only=True)
class OptimizedPlan(FallbackPlan):
    objective: ObjectiveBreakdown
    margins: ConstraintMargins
    feasible_fallback: FallbackPlan
    fallback: bool = False


def _build_highs(
    eligible: list[DeviceState], intervals: list[PlanningInterval]
) -> tuple[highspy.Highs, dict[tuple[int, int], int], dict[tuple[int, int], int], list[int]]:
    solver_type: Callable[[], highspy.Highs] = highspy.Highs
    solver = solver_type()
    solver.setOptionValue("output_flag", False)
    index: dict[tuple[int, int], int] = {}
    activation_index: dict[tuple[int, int], int] = {}
    for device_index, device in enumerate(eligible):
        service_limit = max(0.0, device.max_discharge_kw - device.home_load_kw)
        for interval_index in range(len(intervals)):
            column = solver.getNumCol()
            index[device_index, interval_index] = column
            solver.addCol(
                1.0 + (1.0 - device.availability_probability), 0.0, service_limit, 0, [], []
            )
            if device.home_load_kw > 0.0 and service_limit > 0.0:
                activation = solver.getNumCol()
                activation_index[device_index, interval_index] = activation
                solver.addCol(device.home_load_kw, 0.0, 1.0, 0, [], [])
                solver.changeColIntegrality(activation, highspy.HighsVarType.kInteger)
    shortfall_indices: list[int] = []
    for interval in intervals:
        shortfall_indices.append(solver.getNumCol())
        solver.addCol(1000.0, 0.0, interval.target_kw, 0, [], [])
    for device_index, device in enumerate(eligible):
        service_limit = max(0.0, device.max_discharge_kw - device.home_load_kw)
        if service_limit == 0.0:
            continue
        columns = [index[device_index, interval_index] for interval_index in range(len(intervals))]
        coefficients = [
            interval.duration_hours / device.discharge_efficiency for interval in intervals
        ]
        for interval_index, interval in enumerate(intervals):
            activation_column = activation_index.get((device_index, interval_index))
            if activation_column is not None:
                columns.append(activation_column)
                coefficients.append(
                    device.home_load_kw * interval.duration_hours / device.discharge_efficiency
                )
                solver.addRow(
                    -highspy.kHighsInf,
                    0.0,
                    2,
                    [index[device_index, interval_index], activation_column],
                    [1.0, -service_limit],
                )
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
    return solver, index, activation_index, shortfall_indices


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
    solver, index, activation_index, shortfall_indices = _build_highs(eligible, intervals)
    solver.run()
    if solver.getModelStatus() != highspy.HighsModelStatus.kOptimal:
        raise RuntimeError(f"optimizer status: {solver.getModelStatus()}")
    values = solver.getSolution().col_value
    by_id = {device.device_id: device for device in eligible}
    schedules: list[DeviceSchedule] = []
    for device_index, device in enumerate(eligible):
        energy = device.energy_kwh
        planned: list[ScheduleInterval] = []
        for interval_index, interval in enumerate(intervals):
            service = max(0.0, values[index[device_index, interval_index]])
            activation = activation_index.get((device_index, interval_index))
            discharge = service + (
                device.home_load_kw * values[activation] if activation is not None else 0.0
            )
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
    cycling_cost = sum(item.discharge_kw for schedule in schedules for item in schedule.intervals)
    uncertainty_cost = sum(
        (1.0 - by_id[schedule.device_id].availability_probability) * item.grid_service_kw
        for schedule in schedules
        for item in schedule.intervals
    )
    shortfall_penalty = 1000.0 * sum(item.shortfall_kw for item in shortfalls)
    objective = ObjectiveBreakdown(
        delivered_value=1000.0 * sum(item.expected_kw for item in shortfalls),
        shortfall_penalty=shortfall_penalty,
        cycling_cost=cycling_cost,
        uncertainty_cost=uncertainty_cost,
        total_cost=shortfall_penalty + cycling_cost + uncertainty_cost,
    )
    reserve_margins = [
        interval.expected_energy_kwh - effective_reserve_kwh(by_id[schedule.device_id])
        for schedule in schedules
        for interval in schedule.intervals
    ]
    power_margins = [
        by_id[schedule.device_id].max_discharge_kw - interval.discharge_kw
        for schedule in schedules
        for interval in schedule.intervals
    ]
    margins = ConstraintMargins(min(reserve_margins, default=0.0), min(power_margins, default=0.0))
    return OptimizedPlan(
        tuple(schedules),
        tuple(exclusions),
        tuple(shortfalls),
        objective=objective,
        margins=margins,
        feasible_fallback=plan_fallback(devices, intervals),
    )


def optimize_with_budget(
    devices: list[DeviceState], intervals: list[PlanningInterval], budget_seconds: float
) -> FallbackPlan:
    outcome = solve_within_budget(optimize, devices, intervals, budget_seconds)
    if isinstance(outcome, SolverFault):
        return plan_fallback(devices, intervals)
    return outcome
