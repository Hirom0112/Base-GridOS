from collections.abc import Sequence
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from typing import Literal


@dataclass(frozen=True, slots=True)
class BacktestInterval:
    at: datetime
    profile_load_kwh: Decimal
    public_price_usd_per_mwh: Decimal
    profile_source: str
    price_source: str

    def __post_init__(self) -> None:
        if self.at.tzinfo is None or self.at.utcoffset() is None:
            raise ValueError("backtest time must have a timezone")
        if not self.profile_load_kwh.is_finite() or self.profile_load_kwh < 0:
            raise ValueError("profile load must be finite and nonnegative")
        if not self.public_price_usd_per_mwh.is_finite():
            raise ValueError("public price must be finite")
        if not self.profile_source or not self.price_source:
            raise ValueError("public source identifiers required")


@dataclass(frozen=True, slots=True)
class BillAssumptions:
    flexible_kwh: Decimal
    charging_kwh: Decimal
    retail_usd_per_kwh: Decimal
    reward_usd_per_kwh: Decimal
    degradation_usd_per_kwh: Decimal
    support_usd_per_interval: Decimal
    source: str

    def __post_init__(self) -> None:
        for amount in (
            self.flexible_kwh,
            self.charging_kwh,
            self.retail_usd_per_kwh,
            self.reward_usd_per_kwh,
            self.degradation_usd_per_kwh,
            self.support_usd_per_interval,
        ):
            if not amount.is_finite() or amount < 0:
                raise ValueError("bill assumptions must be finite and nonnegative")
        if not self.source:
            raise ValueError("assumption source required")


@dataclass(frozen=True, slots=True)
class SimulatedValue:
    amount: Decimal
    value_kind: Literal["SIMULATED"]
    sources: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class BillValueSimulation:
    member_savings_usd: SimulatedValue
    company_margin_usd: SimulatedValue


def simulate_bill_value(
    intervals: Sequence[BacktestInterval], assumptions: BillAssumptions
) -> BillValueSimulation:
    if not intervals:
        raise ValueError("backtest intervals required")
    member_savings = Decimal(0)
    company_margin = Decimal(0)
    sources = {assumptions.source}
    previous: datetime | None = None
    for interval in intervals:
        if previous is not None and interval.at <= previous:
            raise ValueError("backtest intervals must be chronological")
        previous = interval.at
        sources.update((interval.profile_source, interval.price_source))
        delivered = min(interval.profile_load_kwh, assumptions.flexible_kwh)
        reward = delivered * assumptions.reward_usd_per_kwh
        market_rate = interval.public_price_usd_per_mwh / Decimal(1000)
        member_savings += delivered * assumptions.retail_usd_per_kwh + reward
        company_margin += (
            delivered * market_rate
            - assumptions.charging_kwh * market_rate
            - reward
            - delivered * assumptions.degradation_usd_per_kwh
            - assumptions.support_usd_per_interval
        )
    lineage = tuple(sorted(sources))
    return BillValueSimulation(
        SimulatedValue(member_savings, "SIMULATED", lineage),
        SimulatedValue(company_margin, "SIMULATED", lineage),
    )
