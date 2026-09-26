from collections.abc import Callable
from dataclasses import replace

import pytest
from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback
from gridos.server import OptimizationServer
from gridos.solver.bounded import Planner, resolve
from gridos.v1 import optimization_pb2, optimization_pb2_grpc
from gridos.validation.plan import validate_plan


def nan_solver(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    corrupted = replace(schedule.intervals[0], discharge_kw=float("nan"))
    return replace(plan, schedules=(replace(schedule, intervals=(corrupted,)),))


def wrong_length_solver(
    devices: list[DeviceState], intervals: list[PlanningInterval]
) -> FallbackPlan:
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    return replace(plan, schedules=(replace(schedule, intervals=schedule.intervals * 2),))


def power_bound_solver(
    devices: list[DeviceState], intervals: list[PlanningInterval]
) -> FallbackPlan:
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    corrupted = replace(schedule.intervals[0], discharge_kw=devices[0].max_discharge_kw + 1.0)
    return replace(plan, schedules=(replace(schedule, intervals=(corrupted,)),))


def export_bound_solver(
    devices: list[DeviceState], intervals: list[PlanningInterval]
) -> FallbackPlan:
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    corrupted = replace(schedule.intervals[0], grid_service_kw=1_000_000.0)
    shortfall = replace(plan.shortfalls[0], allocated_kw=1_000_000.0, shortfall_kw=0.0)
    return replace(
        plan, schedules=(replace(schedule, intervals=(corrupted,)),), shortfalls=(shortfall,)
    )


@pytest.mark.parametrize(
    "solver",
    [nan_solver, wrong_length_solver, power_bound_solver, export_bound_solver],
    ids=["nan", "wrong_length", "power_bound", "export_bound"],
)
def test_invalid_vector_is_replaced_by_fallback(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
    solver: Planner,
) -> None:
    response = serve(OptimizationServer(solver=solver)).Optimize(optimize_request)

    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "INVALID_VECTOR"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert response.plan.shortfalls[0].feasible_kw == 3.0
    assert response.plan.shortfalls[0].shortfall_kw == 0.0


def decision_inputs() -> tuple[list[DeviceState], list[PlanningInterval]]:
    return (
        [
            DeviceState(
                device_id="device-a",
                usable_energy_kwh=10.0,
                energy_kwh=8.0,
                reserve_percent=40.0,
                hardware_floor_percent=10.0,
                dynamic_override_percent=0.0,
                max_discharge_kw=5.0,
                discharge_efficiency=0.95,
                home_load_kw=1.0,
            )
        ],
        [PlanningInterval(target_kw=3.0, duration_hours=0.25)],
    )


def test_invalid_vector_valid_solver_result_is_kept() -> None:
    devices, intervals = decision_inputs()
    fallback = plan_fallback(devices, intervals)
    solved = replace(fallback, exclusions=())

    decision = resolve(solved, fallback, devices, intervals)

    assert decision.plan is solved
    assert decision.fallback_reason == "DETERMINISTIC_FALLBACK"


def test_invalid_vector_export_above_discharge_is_a_bound_violation() -> None:
    devices, intervals = decision_inputs()
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    planned = schedule.intervals[0]
    above = replace(planned, grid_service_kw=planned.discharge_kw - devices[0].home_load_kw + 0.5)
    negative = replace(planned, grid_service_kw=-0.5)
    exact = replace(planned, grid_service_kw=planned.discharge_kw - devices[0].home_load_kw)

    def codes(interval: object) -> set[str]:
        corrupted = replace(plan, schedules=(replace(schedule, intervals=(interval,)),))
        return {violation.code for violation in validate_plan(corrupted, devices, intervals)}

    assert "EXPORT_BOUND" in codes(above)
    assert "EXPORT_BOUND" in codes(negative)
    assert "EXPORT_BOUND" not in codes(exact)


def test_invalid_vector_schedule_for_ineligible_device_is_rejected() -> None:
    devices, intervals = decision_inputs()
    plan = plan_fallback(devices, intervals)
    stale = [replace(devices[0], stale=True)]
    locked = [replace(devices[0], maintenance=True)]
    offline = [replace(devices[0], available=False)]

    assert "STALE_TELEMETRY" in {v.code for v in validate_plan(plan, stale, intervals)}
    assert "MAINTENANCE_LOCK" in {v.code for v in validate_plan(plan, locked, intervals)}
    assert "UNAVAILABLE" in {v.code for v in validate_plan(plan, offline, intervals)}
    assert validate_plan(plan, devices, intervals) == ()
