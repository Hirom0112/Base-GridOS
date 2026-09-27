from collections.abc import Callable

from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def test_flex_selection_is_declared_with_selected_value(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    request.intervals[0].end_time.seconds = request.intervals[0].begin_time.seconds + 3600
    device = request.devices[0]
    device.energy_kwh = 5.0
    device.discharge_efficiency = 1.0
    device.base_reserve_kwh = 4.0
    device.travel_flex_reserve_kwh = 2.0
    device.effective_reserve_kwh = 4.0
    request.conservative_margin = 20.0
    request.margin_hurdle = 10.0
    client = serve(OptimizationServer())

    active = client.Optimize(optimize_request)

    schedule = active.plan.device_schedules[0]
    assert schedule.reserve_selection == optimization_pb2.RESERVE_SELECTION_TRAVEL_FLEX
    assert schedule.selected_reserve_kwh == 2.0
    assert schedule.intervals[0].setpoint_kw == 3.0
    request.conservative_margin = 5.0

    below_hurdle = client.Optimize(optimize_request)

    schedule = below_hurdle.plan.device_schedules[0]
    assert schedule.reserve_selection == optimization_pb2.RESERVE_SELECTION_BASE
    assert schedule.selected_reserve_kwh == 4.0
    assert schedule.intervals[0].setpoint_kw <= 1.0
