from dataclasses import dataclass
from decimal import Decimal
from typing import Literal


@dataclass(frozen=True, slots=True)
class MoneyRange:
    low: Decimal
    high: Decimal

    def __post_init__(self) -> None:
        if not self.low.is_finite() or not self.high.is_finite():
            raise ValueError("margin terms must be finite")
        if self.low > self.high:
            raise ValueError("margin bounds must be ordered")


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

    def __post_init__(self) -> None:
        for cost in (
            self.charging_energy,
            self.incremental_degradation,
            self.penalty_exposure,
            self.member_reward,
            self.support_and_risk_cost,
        ):
            if cost.low < 0:
                raise ValueError("margin cost must be nonnegative")


@dataclass(frozen=True, slots=True)
class MarginEstimate:
    conservative: Decimal
    optimistic: Decimal


@dataclass(frozen=True, slots=True)
class MemberReward:
    kind: Literal["FIXED_CREDIT", "FEE_WAIVER"]
    amount: Decimal


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


def eligible_additional_capacity(
    capacity_kw: Decimal, conservative_margin: Decimal, hurdle: Decimal
) -> Decimal:
    if not capacity_kw.is_finite() or capacity_kw < 0:
        raise ValueError("additional capacity must be finite and nonnegative")
    if not hurdle.is_finite() or hurdle < 0:
        raise ValueError("margin hurdle must be finite and nonnegative")
    if not conservative_margin.is_finite():
        raise ValueError("conservative margin must be finite")
    if conservative_margin <= hurdle:
        return Decimal(0)
    return capacity_kw


def market_reward(membership_fee: Decimal, fixed_credit: Decimal) -> MemberReward:
    if not membership_fee.is_finite() or membership_fee < 0:
        raise ValueError("membership fee must be finite and nonnegative")
    if not fixed_credit.is_finite() or fixed_credit < 0:
        raise ValueError("fixed credit must be finite and nonnegative")
    if membership_fee > 0:
        return MemberReward("FEE_WAIVER", membership_fee)
    if fixed_credit == 0:
        raise ValueError("no-fee market requires a fixed credit")
    return MemberReward("FIXED_CREDIT", fixed_credit)
