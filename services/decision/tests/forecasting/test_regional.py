from datetime import UTC, datetime, timedelta

from gridos.forecasting.regional import RegionalObservation, forecast_regional, realized_mae


def test_regional_load_uses_previous_day_without_future_values() -> None:
    start = datetime(2025, 1, 1, tzinfo=UTC)
    history = [
        RegionalObservation(start + timedelta(hours=hour), 100.0 + hour) for hour in range(24)
    ]
    forecast = forecast_regional(history, start + timedelta(days=1), periods=2)

    assert forecast.values == (100.0, 101.0)
    assert forecast.sources == ("seasonal_persistence", "seasonal_persistence")
    assert forecast.issued_at == start + timedelta(days=1)
    assert realized_mae(forecast, (102.0, 103.0)) == 2.0


def test_regional_price_prefers_available_day_ahead_and_falls_back() -> None:
    start = datetime(2025, 1, 1, tzinfo=UTC)
    history = [
        RegionalObservation(start + timedelta(hours=hour), float(hour)) for hour in range(24)
    ]
    issued_at = start + timedelta(days=1)
    forecast = forecast_regional(
        history,
        issued_at,
        periods=2,
        day_ahead={issued_at: 30.0},
    )

    assert forecast.values == (30.0, 1.0)
    assert forecast.sources == ("day_ahead", "seasonal_persistence")
    assert realized_mae(forecast, (31.0, 3.0)) == 1.5
