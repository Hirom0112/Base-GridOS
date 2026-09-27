from decimal import Decimal
from math import isfinite

from gridos.v1 import device_pb2, optimization_pb2


def conservative_public_margin(
    request: optimization_pb2.OptimizationRequest,
) -> tuple[Decimal, str]:
    intervals = {(item.begin_time.seconds, item.begin_time.nanos) for item in request.intervals}
    prices: dict[str, tuple[Decimal, str]] = {}
    for forecast in request.forecast.regional_prices:
        value = forecast.price_per_mwh
        begin = (forecast.interval_begin_time.seconds, forecast.interval_begin_time.nanos)
        if (
            begin not in intervals
            or not forecast.load_zone
            or not value.model_version
            or (value.value_kind, value.provenance)
            not in {
                ("confirmed_public_forward", device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC),
                ("simulated_forward", device_pb2.DATA_PROVENANCE_SIMULATED),
            }
        ):
            continue
        if not all(isfinite(number) for number in (value.lower, value.value, value.upper)):
            raise ValueError("public price forecast must be finite")
        if not value.lower <= value.value <= value.upper:
            raise ValueError("public price forecast bounds must be ordered")
        lower = Decimal(str(value.lower))
        source = (
            "FROZEN_SIMULATED_PRICE"
            if value.provenance == device_pb2.DATA_PROVENANCE_SIMULATED
            else "FROZEN_PUBLIC_PRICE"
        )
        previous = prices.get(forecast.load_zone)
        if previous is None or lower < previous[0]:
            prices[forecast.load_zone] = lower, source
    margin = Decimal(0)
    margin_source = "UNAVAILABLE"
    for device in request.devices:
        if not device.HasField("travel_flex_reserve_kwh") or not device.HasField(
            "base_reserve_kwh"
        ):
            continue
        priced = prices.get(device.load_zone)
        if priced is None:
            continue
        zone_lower, source = priced
        if zone_lower < 0:
            incremental_kwh = Decimal(str(device.base_reserve_kwh)) - Decimal(
                str(device.travel_flex_reserve_kwh)
            )
            margin += (
                zone_lower * incremental_kwh * Decimal(str(device.discharge_efficiency)) / 1000
            )
            if incremental_kwh > 0:
                if source == "FROZEN_SIMULATED_PRICE" or margin_source == "UNAVAILABLE":
                    margin_source = source
    return margin, margin_source
