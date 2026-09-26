import os
from collections.abc import Callable
from multiprocessing import active_children

import pytest
from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback
from gridos.server import OptimizationServer
from gridos.solver.bounded import Planner, SolverFault, solve_within_budget
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def raising_solver(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
    raise RuntimeError("solver is unhealthy")


def exiting_solver(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
    os._exit(3)


def unshippable_solver() -> Planner:
    def solve(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
        return plan_fallback(devices, intervals)

    return solve


@pytest.mark.parametrize(
    "solver",
    [raising_solver, exiting_solver, unshippable_solver()],
    ids=["raises", "exits", "unshippable"],
)
def test_no_incumbent_unhealthy_solver_returns_fallback(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
    solver: Planner,
) -> None:
    response = serve(OptimizationServer(solver=solver)).Optimize(optimize_request)

    assert response.plan.plan_version == 1
    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "SOLVER_FAILED"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert response.plan.shortfalls[0].shortfall_kw == 0.0


def test_no_incumbent_launch_failure_leaves_no_child() -> None:
    device = DeviceState(
        device_id="device-a",
        usable_energy_kwh=10.0,
        energy_kwh=8.0,
        reserve_percent=40.0,
        hardware_floor_percent=10.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=5.0,
        discharge_efficiency=0.95,
        home_load_kw=0.0,
    )

    outcome = solve_within_budget(
        unshippable_solver(), [device], [PlanningInterval(3.0, 0.25)], 1.0
    )

    assert outcome is SolverFault.SOLVER_FAILED
    assert active_children() == []
