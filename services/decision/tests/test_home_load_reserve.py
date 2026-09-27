from collections.abc import Callable

from gridos.server import OptimizationServer
from gridos.v1 import dispatch_pb2, optimization_pb2, optimization_pb2_grpc

INTERVAL_HOURS = 300 / 3600
HOME_LOAD_KWH = 0.1
HOME_LOAD_KW = HOME_LOAD_KWH / INTERVAL_HOURS


def _near_reserve(request: optimization_pb2.OptimizationRequest) -> None:
    request.devices[0].energy_kwh = 4.2
    request.forecast.site_loads[0].load_kwh.value = HOME_LOAD_KWH
    request.forecast.site_loads[0].load_kwh.upper = HOME_LOAD_KWH


def test_meter_net_export_setpoint_keeps_reserve_including_home_load(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    _near_reserve(optimize_request.request)
    device = optimize_request.request.devices[0]

    response = serve(OptimizationServer()).Optimize(optimize_request)

    setpoint = response.plan.device_schedules[0].intervals[0].setpoint_kw
    assert setpoint > 0.0
    drawn_kwh = (setpoint + HOME_LOAD_KW) * INTERVAL_HOURS / device.discharge_efficiency
    assert device.energy_kwh - drawn_kwh >= device.effective_reserve_kwh - 1e-9


def test_meter_net_export_excludes_device_without_site_load_forecast(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    del optimize_request.request.forecast.site_loads[:]

    response = serve(OptimizationServer()).Optimize(optimize_request)

    assert not response.plan.device_schedules
    assert [item.reason for item in response.plan.exclusions] == [
        dispatch_pb2.EXCLUSION_REASON_UNAVAILABLE
    ]
    assert response.plan.shortfalls[0].shortfall_kw == 3.0
