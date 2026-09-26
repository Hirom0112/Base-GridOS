from dataclasses import replace
from datetime import UTC, datetime, timedelta

from gridos.forecasting.evaluate import evaluate_load, select_forecast
from gridos.forecasting.load import LoadForecast


def test_evaluate_records_realized_error_and_interval_coverage() -> None:
    issued_at = datetime(2025, 1, 7, tzinfo=UTC)
    forecast = LoadForecast(
        values_kwh=(1.0, 2.0),
        lower_kwh=(0.0, 1.0),
        upper_kwh=(2.0, 3.0),
        training_window=(issued_at - timedelta(days=2), issued_at - timedelta(days=1)),
        feature_version="profile-v1",
        model_version="baseline-v1",
        issued_at=issued_at,
        horizon=timedelta(minutes=30),
    )

    result = evaluate_load(forecast, (1.5, 4.0))

    assert result.model_version == "baseline-v1"
    assert result.issued_at == issued_at
    assert result.mean_absolute_error_kwh == 1.25
    assert result.interval_coverage == 0.5


def test_unhealthy_or_missing_model_uses_deterministic_baseline() -> None:
    issued_at = datetime(2025, 1, 7, tzinfo=UTC)
    baseline = LoadForecast(
        values_kwh=(1.0,),
        lower_kwh=(0.0,),
        upper_kwh=(2.0,),
        training_window=(issued_at - timedelta(days=2), issued_at - timedelta(days=1)),
        feature_version="profile-v1",
        model_version="baseline-v1",
        issued_at=issued_at,
        horizon=timedelta(minutes=15),
    )
    learned = replace(baseline, values_kwh=(1.5,), model_version="learned-v1")
    unhealthy = replace(learned, values_kwh=(float("nan"),))

    assert select_forecast(None, baseline) == baseline
    assert select_forecast(unhealthy, baseline) == baseline
    assert select_forecast(learned, baseline) == learned
