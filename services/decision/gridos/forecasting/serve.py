import csv
from datetime import UTC, datetime, timedelta
from math import isfinite
from pathlib import Path
from zoneinfo import ZoneInfo

import grpc

from gridos.forecasting.availability import ReliabilityTrait, forecast_availability
from gridos.forecasting.load import LoadObservation, forecast_load
from gridos.v1 import device_pb2, optimization_pb2

_PUBLIC = Path(__file__).resolve().parents[4] / "testdata/fixtures/public"
_CENTRAL = ZoneInfo("America/Chicago")
_TRAITS: dict[str, ReliabilityTrait] = {"HIGH": "HIGH", "MEDIUM": "MEDIUM", "LOW": "LOW"}


def _site_load_history(profile: str) -> list[LoadObservation]:
    path = _PUBLIC / "load-profiles/residential-week.csv"
    if not path.exists():
        return []
    observations: list[LoadObservation] = []
    with path.open(newline="") as source:
        for row in csv.DictReader(source):
            if row["PType_WZ"] != profile:
                continue
            day = datetime.strptime(row["Date"], "%Y-%m-%d %H:%M:%S").replace(tzinfo=_CENTRAL)
            for slot in range(96):
                value = row[f"int_kWh{slot + 1}"]
                if value:
                    observations.append(
                        LoadObservation(day + timedelta(minutes=15 * slot), float(value))
                    )
    return observations


def _site_load_at(
    observations: list[LoadObservation], begin: datetime, end: datetime
) -> tuple[float, float, float]:
    total = lower = upper = 0.0
    current = begin.astimezone(_CENTRAL)
    local_end = end.astimezone(_CENTRAL)
    while current < local_end:
        midnight = current.replace(hour=0, minute=0, second=0, microsecond=0)
        daily = forecast_load(
            [item for item in observations if item.timestamp < midnight],
            midnight,
            timedelta(days=1),
        )
        slot = current.hour * 4 + current.minute // 15
        next_slot = midnight + timedelta(minutes=15 * (slot + 1))
        segment_end = min(next_slot, local_end)
        fraction = (segment_end - current).total_seconds() / 900
        total += daily.values_kwh[slot] * fraction
        lower += daily.lower_kwh[slot] * fraction
        upper += daily.upper_kwh[slot] * fraction
        current = segment_end
    return total, lower, upper


def _day_ahead_prices() -> dict[tuple[str, datetime], float]:
    path = _PUBLIC / "ercot-prices/dam-spp-week.csv"
    if not path.exists():
        return {}
    prices: dict[tuple[str, datetime], float] = {}
    with path.open(newline="") as source:
        for row in csv.DictReader(source):
            day = datetime.strptime(row["Delivery Date"], "%m/%d/%Y").replace(tzinfo=_CENTRAL)
            hour = int(row["Hour Ending"].split(":", 1)[0]) - 1
            prices[row["Settlement Point"], day + timedelta(hours=hour)] = float(
                row["Settlement Point Price"]
            )
    return prices


def _forecast_value(
    bounds: tuple[float, float, float],
    feature: str,
    model: str,
    kind: str,
    provenance: device_pb2.DataProvenance,
) -> optimization_pb2.ForecastValue:
    return optimization_pb2.ForecastValue(
        value=bounds[0],
        lower=bounds[1],
        upper=bounds[2],
        feature_version=feature,
        model_version=model,
        value_kind=kind,
        provenance=provenance,
    )


def _stamp_forecast(
    value: optimization_pb2.ForecastValue,
    issued_at: datetime,
    begin: datetime,
    training_window: tuple[datetime, datetime] | None = None,
) -> None:
    value.issued_at.FromDatetime(issued_at)
    value.horizon.FromTimedelta(begin - issued_at)
    if training_window is not None:
        value.training_window_begin.FromDatetime(training_window[0])
        value.training_window_end.FromDatetime(training_window[1])


def forecast_response(
    wrapper: optimization_pb2.ForecastRequest,
    context: grpc.ServicerContext,
) -> optimization_pb2.ForecastResponse:
    if not wrapper.HasField("request"):
        context.abort(grpc.StatusCode.INVALID_ARGUMENT, "request is required")
    request = wrapper.request
    if not request.HasField("requested_at") or not request.HasField("budget"):
        context.abort(grpc.StatusCode.INVALID_ARGUMENT, "forecast time and budget are required")
    budget = request.budget.ToTimedelta().total_seconds()
    if not isfinite(budget) or budget <= 0.0:
        context.abort(grpc.StatusCode.INVALID_ARGUMENT, "budget must be positive")
    issued_at = request.requested_at.ToDatetime(tzinfo=UTC)
    response = optimization_pb2.ForecastResponse()
    histories: dict[str, list[LoadObservation]] = {}
    loads: dict[tuple[str, datetime, datetime], tuple[float, float, float] | None] = {}
    training: dict[tuple[str, datetime], tuple[datetime, datetime] | None] = {}
    prices = _day_ahead_prices()
    for interval in request.intervals:
        if not interval.HasField("begin_time") or not interval.HasField("end_time"):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "forecast interval is required")
        begin = interval.begin_time.ToDatetime(tzinfo=UTC)
        end = interval.end_time.ToDatetime(tzinfo=UTC)
        if end <= begin:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "forecast interval must be positive")
        seen_zones: set[str] = set()
        for site in request.sites:
            if site.load_profile_type not in histories:
                histories[site.load_profile_type] = _site_load_history(site.load_profile_type)
            history = histories[site.load_profile_type]
            key = (site.load_profile_type, begin, end)
            training_key = (site.load_profile_type, begin)
            if training_key not in training:
                midnight = begin.astimezone(_CENTRAL).replace(
                    hour=0, minute=0, second=0, microsecond=0
                )
                used = [item.timestamp for item in history if item.timestamp < midnight]
                training[training_key] = (min(used), max(used)) if used else None
            if key not in loads:
                try:
                    loads[key] = _site_load_at(history, begin, end) if history else None
                except ValueError:
                    loads[key] = None
            predicted_load = loads[key]
            if predicted_load is None:
                response.unavailable_sources.append(f"site_load:{site.site_id}")
            else:
                load_output = response.site_loads.add(site_id=site.site_id)
                load_output.interval_begin_time.CopyFrom(interval.begin_time)
                load_output.load_kwh.CopyFrom(
                    _forecast_value(
                        predicted_load,
                        "similar-day-weekday-v1",
                        "load-baseline-v1",
                        "modeled_estimate",
                        device_pb2.DATA_PROVENANCE_DERIVED,
                    )
                )
                load_output.load_kwh.interval_coverage = 0.90
                _stamp_forecast(load_output.load_kwh, issued_at, begin, training[training_key])
            if site.load_zone in seen_zones:
                continue
            seen_zones.add(site.load_zone)
            price = prices.get(
                (
                    site.load_zone,
                    begin.astimezone(_CENTRAL).replace(minute=0, second=0, microsecond=0),
                )
            )
            if price is None:
                response.unavailable_sources.append(f"regional_price:{site.load_zone}")
            else:
                price_output = response.regional_prices.add(load_zone=site.load_zone)
                price_output.interval_begin_time.CopyFrom(interval.begin_time)
                price_output.price_per_mwh.CopyFrom(
                    _forecast_value(
                        (price, price, price),
                        "ercot-dam-spp-v1",
                        "day-ahead-v1",
                        "confirmed_public_forward",
                        device_pb2.DATA_PROVENANCE_CONFIRMED_PUBLIC,
                    )
                )
                _stamp_forecast(price_output.price_per_mwh, issued_at, begin)
            response.unavailable_sources.append(f"outage_risk:{site.county or site.site_id}")
        for device in request.devices:
            trait = _TRAITS.get(device.reliability_trait)
            if trait is None or not device.HasField("telemetry_observed_at"):
                response.unavailable_sources.append(f"device_availability:{device.device_id}")
                continue
            age = issued_at - device.telemetry_observed_at.ToDatetime(tzinfo=UTC)
            if age < timedelta(0):
                response.unavailable_sources.append(f"device_availability:{device.device_id}")
                continue
            prediction = forecast_availability(trait, 0, 0, age)
            availability_output = response.device_availability.add(device_id=device.device_id)
            availability_output.interval_begin_time.CopyFrom(interval.begin_time)
            availability_output.probability.CopyFrom(
                _forecast_value(
                    (
                        prediction.availability_probability,
                        prediction.availability_probability,
                        prediction.availability_probability,
                    ),
                    "reliability-freshness-v1",
                    "availability-baseline-v1",
                    "modeled_estimate",
                    device_pb2.DATA_PROVENANCE_DERIVED,
                )
            )
            _stamp_forecast(availability_output.probability, issued_at, begin)
    return response
