import time
from collections.abc import Callable
from multiprocessing import active_children

from google.protobuf.duration_pb2 import Duration
from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback
from gridos.server import OptimizationServer
from gridos.solver.bounded import SolverFault, solve_within_budget
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def hanging_solver(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
    time.sleep(60.0)
    return plan_fallback(devices, intervals)


def test_timeout_returns_fallback_with_timeout_reason(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    stub = serve(OptimizationServer(solver=hanging_solver))
    optimize_request.request.budget.CopyFrom(Duration(nanos=200_000_000))
    started = time.monotonic()

    response = stub.Optimize(optimize_request)

    assert time.monotonic() - started < 5.0
    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "TIMEOUT"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert response.plan.shortfalls[0].shortfall_kw == 0.0


def test_timeout_kills_solver_subprocess() -> None:
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
    started = time.monotonic()

    outcome = solve_within_budget(hanging_solver, [device], [PlanningInterval(3.0, 0.25)], 0.2)

    assert outcome is SolverFault.TIMEOUT
    assert time.monotonic() - started < 5.0
    assert active_children() == []
