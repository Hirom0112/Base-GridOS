from collections.abc import Callable

from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def test_travel_flex_capacity_requires_active_policy_and_margin(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    request.intervals[0].end_time.seconds = request.intervals[0].begin_time.seconds + 3600
    request.devices[0].energy_kwh = 5.0
    request.devices[0].discharge_efficiency = 1.0
    request.devices[0].base_reserve_kwh = 4.0
    request.devices[0].travel_flex_reserve_kwh = 2.0
    request.devices[0].effective_reserve_kwh = 2.0
    request.conservative_margin = 20.0
    request.margin_hurdle = 10.0
    client = serve(OptimizationServer())

    active = client.Optimize(optimize_request)

    assert active.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert active.plan.shortfalls[0].shortfall_kw == 0.0

    request.conservative_margin = 5.0
    request.devices[0].effective_reserve_kwh = 4.0

    below_hurdle = client.Optimize(optimize_request)

    assert below_hurdle.plan.device_schedules[0].intervals[0].setpoint_kw <= 1.0
    assert below_hurdle.plan.shortfalls[0].shortfall_kw >= 2.0

    request.devices[0].ClearField("travel_flex_reserve_kwh")

    inactive = client.Optimize(optimize_request)

    assert inactive.plan.device_schedules[0].intervals[0].setpoint_kw <= 1.0
