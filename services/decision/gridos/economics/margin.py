from dataclasses import dataclass
from decimal import Decimal


@dataclass(frozen=True, slots=True)
class MoneyRange:
    low: Decimal
    high: Decimal

    def __post_init__(self) -> None:
        if not self.low.is_finite() or not self.high.is_finite():
            raise ValueError("margin terms must be finite")
        if self.low < 0 or self.low > self.high:
            raise ValueError("margin bounds must be nonnegative and ordered")


@dataclass(frozen=True, slots=True)
class MarginComponents:
    dispatch_value: MoneyRange
    avoided_peak_cost: MoneyRange
    commitment_reliability_value: MoneyRange
    charging_energy: MoneyRange
    incremental_degradation: MoneyRange
    penalty_exposure: MoneyRange
    member_reward: MoneyRange
    support_and_risk_cost: MoneyRange


@dataclass(frozen=True, slots=True)
class MarginEstimate:
    conservative: Decimal
    optimistic: Decimal


def estimate_margin(components: MarginComponents) -> MarginEstimate:
    values = (
        components.dispatch_value,
        components.avoided_peak_cost,
        components.commitment_reliability_value,
    )
    costs = (
        components.charging_energy,
        components.incremental_degradation,
        components.penalty_exposure,
        components.member_reward,
        components.support_and_risk_cost,
    )
    return MarginEstimate(
        conservative=sum((term.low for term in values), Decimal(0))
        - sum((term.high for term in costs), Decimal(0)),
        optimistic=sum((term.high for term in values), Decimal(0))
        - sum((term.low for term in costs), Decimal(0)),
    )
