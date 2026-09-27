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

    assert _conservative_public_margin(request) == (Decimal("-0.19"), "FROZEN_PUBLIC_PRICE")
    price.price_per_mwh.value = 100.0
    price.price_per_mwh.lower = 100.0
    price.price_per_mwh.upper = 100.0
    assert _conservative_public_margin(request) == (Decimal(0), "UNAVAILABLE")
    price.price_per_mwh.Clear()
    assert _conservative_public_margin(request) == (Decimal(0), "UNAVAILABLE")


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
    explanation = response.plan.margin_explanation
    assert explanation.conservative_margin == -0.1
    assert explanation.margin_hurdle == 0
    terms = {term.name: term for term in explanation.terms}
    assert terms["DISPATCH_VALUE"].low == -0.1
    assert terms["DISPATCH_VALUE"].source == "FROZEN_PUBLIC_PRICE"
    assert not terms["DISPATCH_VALUE"].unavailable
    assert terms["MEMBER_REWARD"].unavailable
    assert terms["MEMBER_REWARD"].high == 0


def test_plain_grid_service_has_sourced_zero_value_terms(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    explanation = serve(OptimizationServer()).Optimize(optimize_request).plan.margin_explanation
    terms = {term.name: term for term in explanation.terms}
    assert terms["AVOIDED_PEAK_COST"].source == "ABSENT_PEAK_AVOIDANCE_CONTRACT"
    assert terms["COMMITMENT_RELIABILITY_VALUE"].source == "ABSENT_COMMITMENT_CONTRACT"
    assert all(
        not terms[name].unavailable and terms[name].low == terms[name].high == 0
        for name in ("AVOIDED_PEAK_COST", "COMMITMENT_RELIABILITY_VALUE")
    )


def test_negative_simulated_price_keeps_its_label(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    request.devices[0].base_reserve_kwh = 4.0
    request.devices[0].travel_flex_reserve_kwh = 2.0
    request.devices[0].effective_reserve_kwh = 4.0
    price = request.forecast.regional_prices.add(load_zone="LZ_AEN")
    price.interval_begin_time.CopyFrom(request.intervals[0].begin_time)
    price.price_per_mwh.value = -50.0
    price.price_per_mwh.lower = -50.0
    price.price_per_mwh.upper = -50.0
    price.price_per_mwh.model_version = "day-ahead-v1"
    price.price_per_mwh.value_kind = "simulated_forward"
    price.price_per_mwh.provenance = device_pb2.DATA_PROVENANCE_SIMULATED

    response = serve(OptimizationServer()).Optimize(optimize_request)

    assert response.plan.margin_explanation.conservative_margin < 0
    assert response.plan.margin_explanation.terms[0].source == "FROZEN_SIMULATED_PRICE"
