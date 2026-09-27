from bisect import bisect_right
from math import inf, isfinite

from gridos.v1 import optimization_pb2, telemetry_pb2


def home_loads_kw(
    request: optimization_pb2.OptimizationRequest,
) -> dict[str, float | None]:
    site_ids = {device.site_id for device in request.devices}
    if request.measurement_boundary != telemetry_pb2.MEASUREMENT_BOUNDARY_METER_NET_EXPORT:
        return dict.fromkeys(site_ids, 0.0)
    forecasts: dict[str, list[tuple[float, float]]] = {}
    for load in request.forecast.site_loads:
        kwh = max(load.load_kwh.value, load.load_kwh.upper)
        if not isfinite(kwh) or kwh < 0.0 or load.load_kwh.lower < 0.0:
            raise ValueError("invalid site load forecast")
        begin = load.interval_begin_time
        forecasts.setdefault(load.site_id, []).append(
            (begin.seconds + begin.nanos / 1_000_000_000, kwh)
        )
    for series in forecasts.values():
        series.sort()
    spans = [
        (
            interval.begin_time.seconds + interval.begin_time.nanos / 1_000_000_000,
            interval.end_time.seconds + interval.end_time.nanos / 1_000_000_000,
        )
        for interval in request.intervals
    ]
    loads: dict[str, float | None] = {}
    for site_id in site_ids:
        series = forecasts.get(site_id, []) if site_id else []
        peak_kw: float | None = 0.0
        for begin, end in spans:
            position = bisect_right(series, (begin, inf))
            if position == 0:
                peak_kw = None
                break
            peak_kw = max(peak_kw or 0.0, series[position - 1][1] / ((end - begin) / 3600.0))
        loads[site_id] = peak_kw
    return loads
