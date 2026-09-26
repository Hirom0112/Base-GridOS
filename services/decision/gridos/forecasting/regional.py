from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime, timedelta
from math import isfinite


@dataclass(frozen=True)
class RegionalObservation:
    timestamp: datetime
    value: float


@dataclass(frozen=True)
class RegionalForecast:
    values: tuple[float, ...]
    sources: tuple[str, ...]
    issued_at: datetime


def forecast_regional(
    observations: Sequence[RegionalObservation],
    issued_at: datetime,
    periods: int,
    day_ahead: Mapping[datetime, float] | None = None,
) -> RegionalForecast:
    if issued_at.tzinfo is None or issued_at.utcoffset() is None:
        raise ValueError("issued_at must be timezone-aware")
    if issued_at.minute or issued_at.second or issued_at.microsecond:
        raise ValueError("issued_at must align to an hour")
    if periods < 1:
        raise ValueError("periods must be positive")

    history: dict[datetime, float] = {}
    for observation in observations:
        if observation.timestamp.tzinfo is None or observation.timestamp.utcoffset() is None:
            raise ValueError("observation timestamp must be timezone-aware")
        if observation.timestamp >= issued_at:
            raise ValueError("observations must precede issued_at")
        if not isfinite(observation.value):
            raise ValueError("regional value must be finite")
        if observation.timestamp in history:
            raise ValueError("duplicate regional observation")
        history[observation.timestamp] = observation.value
    if not history:
        raise ValueError("at least one observation is required")

    available = day_ahead or {}
    values = []
    sources = []
    latest = history[max(history)]
    for index in range(periods):
        target = issued_at + timedelta(hours=index)
        if target in available:
            value = available[target]
            source = "day_ahead"
        elif target - timedelta(days=1) in history:
            value = history[target - timedelta(days=1)]
            source = "seasonal_persistence"
        else:
            value = latest
            source = "latest_persistence"
        if not isfinite(value):
            raise ValueError("day-ahead value must be finite")
        values.append(value)
        sources.append(source)
    return RegionalForecast(tuple(values), tuple(sources), issued_at)


def realized_mae(forecast: RegionalForecast, actual: Sequence[float]) -> float:
    if len(actual) != len(forecast.values) or not actual:
        raise ValueError("actual values must match the forecast horizon")
    if not all(isfinite(value) for value in actual):
        raise ValueError("actual values must be finite")
    return sum(
        abs(predicted - observed)
        for predicted, observed in zip(forecast.values, actual, strict=True)
    ) / len(actual)
