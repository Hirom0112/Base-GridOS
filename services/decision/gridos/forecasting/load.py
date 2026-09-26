from collections.abc import Sequence
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from math import ceil, isfinite


@dataclass(frozen=True)
class LoadObservation:
    timestamp: datetime
    energy_kwh: float


@dataclass(frozen=True)
class LoadForecast:
    values_kwh: tuple[float, ...]
    lower_kwh: tuple[float, ...]
    upper_kwh: tuple[float, ...]
    training_window: tuple[datetime, datetime]
    feature_version: str
    model_version: str
    issued_at: datetime
    horizon: timedelta


def forecast_load(
    observations: Sequence[LoadObservation], issued_at: datetime, horizon: timedelta
) -> LoadForecast:
    if issued_at.tzinfo is None or issued_at.utcoffset() is None:
        raise ValueError("issued_at must be timezone-aware")
    if issued_at.hour or issued_at.minute or issued_at.second or issued_at.microsecond:
        raise ValueError("issued_at must start at local midnight")
    if horizon != timedelta(days=1):
        raise ValueError("horizon must be one day")

    days: dict[date, dict[int, float]] = {}
    for observation in observations:
        timestamp = observation.timestamp
        if timestamp.tzinfo is None or timestamp.utcoffset() is None:
            raise ValueError("observation timestamp must be timezone-aware")
        if timestamp >= issued_at:
            raise ValueError("training observations must precede issued_at")
        if timestamp.minute % 15 or timestamp.second or timestamp.microsecond:
            raise ValueError("observations must align to 15-minute intervals")
        if not isfinite(observation.energy_kwh) or observation.energy_kwh < 0:
            raise ValueError("load energy must be finite and nonnegative")
        slot = timestamp.hour * 4 + timestamp.minute // 15
        values = days.setdefault(timestamp.date(), {})
        if slot in values:
            raise ValueError("duplicate load interval")
        values[slot] = observation.energy_kwh

    same_class = sorted(
        day
        for day, values in days.items()
        if len(values) == 96 and (day.weekday() < 5) == (issued_at.weekday() < 5)
    )
    if len(same_class) < 2:
        raise ValueError("at least two complete similar days are required")

    values_kwh = tuple(days[same_class[-1]][slot] for slot in range(96))
    residuals = sorted(
        abs(days[current][slot] - days[previous][slot])
        for previous, current in zip(same_class, same_class[1:], strict=False)
        for slot in range(96)
    )
    radius = residuals[ceil(0.9 * len(residuals)) - 1]
    return LoadForecast(
        values_kwh=values_kwh,
        lower_kwh=tuple(max(0.0, value - radius) for value in values_kwh),
        upper_kwh=tuple(value + radius for value in values_kwh),
        training_window=(
            min(observation.timestamp for observation in observations),
            max(observation.timestamp for observation in observations),
        ),
        feature_version="similar-day-weekday-v1",
        model_version="load-baseline-v1",
        issued_at=issued_at,
        horizon=horizon,
    )
