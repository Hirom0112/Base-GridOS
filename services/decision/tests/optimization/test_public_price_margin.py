from collections.abc import Callable
from decimal import Decimal

from gridos.server import OptimizationServer, _conservative_public_margin
from gridos.v1 import device_pb2, optimization_pb2, optimization_pb2_grpc


def test_public_price_margin_from_frozen_forecast(
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    device = request.devices[0]
    device.base_reserve_kwh = 4.0
    device.travel_flex_reserve_kwh = 2.0
    request.conservative_margin = 0.0
    price = request.forecast.regional_prices.add(load_zone="LZ_AEN")
    price.interval_begin_time.CopyFrom(request.intervals[0].begin_time)
    price.price_per_mwh.value = -100.0
    price.price_per_mwh.lower = -100.0
    price.price_per_mwh.upper = -100.0
    price.price_per_mwh.feature_version = "ercot-dam-spp-v1"
    price.price_per_mwh.model_version = "day-ahead-v1"
    price.price_per_mwh.value_kind = "confirmed_public_forward"
    price.price_per_mwh.provenance = device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC

    assert _conservative_public_margin(request) == Decimal("-0.19")
    price.price_per_mwh.value = 100.0
    price.price_per_mwh.lower = 100.0
    price.price_per_mwh.upper = 100.0
    assert _conservative_public_margin(request) == Decimal(0)
    price.price_per_mwh.Clear()
    assert _conservative_public_margin(request) == Decimal(0)


def test_negative_public_price_keeps_base_reserve(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    request.intervals[0].end_time.seconds = request.intervals[0].begin_time.seconds + 3600
    request.devices[0].energy_kwh = 5.0
    request.devices[0].discharge_efficiency = 1.0
    request.devices[0].base_reserve_kwh = 4.0
    request.devices[0].travel_flex_reserve_kwh = 2.0
    request.devices[0].effective_reserve_kwh = 4.0
    price = request.forecast.regional_prices.add(load_zone="LZ_AEN")
    price.interval_begin_time.CopyFrom(request.intervals[0].begin_time)
    price.price_per_mwh.value = -50.0
    price.price_per_mwh.lower = -50.0
    price.price_per_mwh.upper = -50.0
    price.price_per_mwh.feature_version = "ercot-dam-spp-v1"
    price.price_per_mwh.model_version = "day-ahead-v1"
    price.price_per_mwh.value_kind = "confirmed_public_forward"
    price.price_per_mwh.provenance = device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC

    response = serve(OptimizationServer()).Optimize(optimize_request)

    assert response.plan.device_schedules[0].intervals[0].setpoint_kw <= 1.0
    assert response.plan.shortfalls[0].shortfall_kw >= 2.0
