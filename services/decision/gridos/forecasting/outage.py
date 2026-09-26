from calendar import monthrange
from collections.abc import Sequence
from dataclasses import dataclass
from datetime import datetime
from math import exp, isfinite
from typing import Literal


@dataclass(frozen=True)
class OutageAlert:
    county: str
    starts_at: datetime
    ends_at: datetime


@dataclass(frozen=True)
class OutageRisk:
    county: str
    hour: datetime
    hourly_probability: float
    active_alert_count: int
    value_kind: Literal["modeled_estimate"] = "modeled_estimate"


def forecast_outage_risk(
    county: str,
    monthly_outage_rate: float,
    hour: datetime,
    alerts: Sequence[OutageAlert],
) -> OutageRisk:
    if not county:
        raise ValueError("county is required")
    if hour.tzinfo is None or hour.utcoffset() is None:
        raise ValueError("hour must be timezone-aware")
    if hour.minute or hour.second or hour.microsecond:
        raise ValueError("hour must align to an hour")
    if not isfinite(monthly_outage_rate) or monthly_outage_rate < 0:
        raise ValueError("monthly outage rate must be finite and nonnegative")

    active_count = 0
    for alert in alerts:
        if alert.starts_at.tzinfo is None or alert.ends_at.tzinfo is None:
            raise ValueError("alert times must be timezone-aware")
        if alert.ends_at <= alert.starts_at:
            raise ValueError("alert must have a positive duration")
        if alert.county == county and alert.starts_at <= hour < alert.ends_at:
            active_count += 1

    hours = monthrange(hour.year, hour.month)[1] * 24
    hourly_probability = 1 - exp(-monthly_outage_rate * (1 + active_count) / hours)
    return OutageRisk(county, hour, hourly_probability, active_count)
