from datetime import UTC, datetime, timedelta
from decimal import Decimal

from gridos.economics.bill import BacktestInterval, BillAssumptions, simulate_bill_value


def test_bill_simulator_uses_public_prices_profiles_and_explicit_assumptions() -> None:
    start = datetime(2025, 1, 4, tzinfo=UTC)
    intervals = (
        BacktestInterval(start, Decimal("2"), Decimal("100"), "public-profile", "public-price"),
        BacktestInterval(
            start + timedelta(minutes=30),
            Decimal("1"),
            Decimal("200"),
            "public-profile",
            "public-price",
        ),
    )
    assumptions = BillAssumptions(
        flexible_kwh=Decimal("1.5"),
        charging_kwh=Decimal("0.5"),
        retail_usd_per_kwh=Decimal("0.20"),
        reward_usd_per_kwh=Decimal("0.10"),
        degradation_usd_per_kwh=Decimal("0.02"),
        support_usd_per_interval=Decimal("0.015"),
        source="scenario-a",
    )

    result = simulate_bill_value(intervals, assumptions)

    assert result.member_savings_usd.amount == Decimal("0.75")
    assert result.company_margin_usd.amount == Decimal("-0.13")
    assert result.member_savings_usd.value_kind == "SIMULATED"
    assert result.company_margin_usd.value_kind == "SIMULATED"
    assert result.member_savings_usd.sources == (
        "public-price",
        "public-profile",
        "scenario-a",
    )
    assert result.company_margin_usd.sources == result.member_savings_usd.sources
