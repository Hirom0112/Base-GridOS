from datetime import UTC, datetime, timedelta
from math import exp

import pytest
from gridos.forecasting.outage import OutageAlert, forecast_outage_risk


def test_outage_risk_uses_county_rate_and_only_active_alerts() -> None:
    at = datetime(2025, 1, 7, 12, tzinfo=UTC)
    active = OutageAlert("Travis", at - timedelta(hours=1), at + timedelta(hours=1))
    other_county = OutageAlert("Harris", at - timedelta(hours=1), at + timedelta(hours=1))
    expired = OutageAlert("Travis", at - timedelta(hours=2), at)

    baseline = forecast_outage_risk("Travis", 0.037664286849066676, at, [])
    alerted = forecast_outage_risk(
        "Travis", 0.037664286849066676, at, [active, other_county, expired]
    )

    assert baseline.value_kind == alerted.value_kind == "modeled_estimate"
    assert baseline.hourly_probability == pytest.approx(1 - exp(-0.037664286849066676 / 744))
    assert alerted.hourly_probability == pytest.approx(1 - exp(-2 * 0.037664286849066676 / 744))
    assert alerted.active_alert_count == 1
