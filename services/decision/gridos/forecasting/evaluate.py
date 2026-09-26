from collections.abc import Sequence
from dataclasses import dataclass
from datetime import datetime
from math import isfinite

from gridos.forecasting.load import LoadForecast


@dataclass(frozen=True)
class ForecastEvaluation:
    model_version: str
    issued_at: datetime
    mean_absolute_error_kwh: float
    interval_coverage: float


def evaluate_load(forecast: LoadForecast, actual: Sequence[float]) -> ForecastEvaluation:
    if len(actual) != len(forecast.values_kwh) or not actual:
        raise ValueError("actual values must match the forecast horizon")
    if not all(isfinite(value) for value in actual):
        raise ValueError("actual load must be finite")
    error = sum(
        abs(predicted - observed)
        for predicted, observed in zip(forecast.values_kwh, actual, strict=True)
    ) / len(actual)
    covered = sum(
        lower <= observed <= upper
        for lower, observed, upper in zip(
            forecast.lower_kwh, actual, forecast.upper_kwh, strict=True
        )
    ) / len(actual)
    return ForecastEvaluation(forecast.model_version, forecast.issued_at, error, covered)


def select_forecast(learned: LoadForecast | None, baseline: LoadForecast) -> LoadForecast:
    if learned is None:
        return baseline
    if learned.issued_at != baseline.issued_at or learned.horizon != baseline.horizon:
        return baseline
    if not learned.model_version or len(learned.values_kwh) != len(baseline.values_kwh):
        return baseline
    if len(learned.lower_kwh) != len(learned.values_kwh) or len(learned.upper_kwh) != len(
        learned.values_kwh
    ):
        return baseline
    if any(
        not all(isfinite(value) for value in (lower, predicted, upper))
        or not lower <= predicted <= upper
        for lower, predicted, upper in zip(
            learned.lower_kwh, learned.values_kwh, learned.upper_kwh, strict=True
        )
    ):
        return baseline
    return learned
