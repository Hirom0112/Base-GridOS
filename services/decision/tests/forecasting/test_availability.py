from datetime import timedelta

from gridos.forecasting.availability import forecast_availability, forecast_soc


def test_availability_uses_trait_connectivity_and_freshness() -> None:
    healthy = forecast_availability(
        "HIGH", successful_contacts=95, total_contacts=100, age=timedelta(seconds=5)
    )
    unreliable = forecast_availability(
        "LOW", successful_contacts=30, total_contacts=100, age=timedelta(minutes=10)
    )

    assert 0 < unreliable.availability_probability < healthy.availability_probability < 1
    assert healthy.failure_probability == 1 - healthy.availability_probability
    assert unreliable.failure_probability == 1 - unreliable.availability_probability


def test_soc_interval_widens_with_telemetry_age() -> None:
    fresh = forecast_soc(20.0, 40.0, 4.0, 8.0, timedelta(seconds=5), steps=3)
    stale = forecast_soc(20.0, 40.0, 4.0, 8.0, timedelta(minutes=20), steps=3)

    assert [point.expected_percent for point in fresh] == [
        50.0,
        49.166666666666664,
        48.333333333333336,
    ]
    assert all(
        0 <= point.lower_percent <= point.expected_percent <= point.upper_percent <= 100
        for point in stale
    )
    assert (
        stale[0].upper_percent - stale[0].lower_percent
        > fresh[0].upper_percent - fresh[0].lower_percent
    )
    assert (
        stale[2].upper_percent - stale[2].lower_percent
        > stale[0].upper_percent - stale[0].lower_percent
    )
