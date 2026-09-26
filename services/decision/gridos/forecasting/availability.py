from dataclasses import dataclass
from datetime import timedelta
from math import exp, isfinite
from typing import Literal

ReliabilityTrait = Literal["HIGH", "MEDIUM", "LOW"]


@dataclass(frozen=True)
class AvailabilityForecast:
    availability_probability: float
    failure_probability: float


@dataclass(frozen=True)
class SocForecast:
    expected_percent: float
    lower_percent: float
    upper_percent: float


def forecast_availability(
    trait: ReliabilityTrait,
    successful_contacts: int,
    total_contacts: int,
    age: timedelta,
) -> AvailabilityForecast:
    if successful_contacts < 0 or total_contacts < successful_contacts:
        raise ValueError("contact counts are inconsistent")
    if age < timedelta(0):
        raise ValueError("telemetry age cannot be negative")
    priors = {"HIGH": (99, 1), "MEDIUM": (19, 1), "LOW": (9, 1)}
    if trait not in priors:
        raise ValueError("unknown reliability trait")
    successes, failures = priors[trait]
    connectivity = (successes + successful_contacts) / (successes + failures + total_contacts)
    availability = connectivity * exp(-age.total_seconds() / 300)
    return AvailabilityForecast(availability, 1 - availability)


def forecast_soc(
    energy_kwh: float,
    capacity_kwh: float,
    net_discharge_kw: float,
    max_power_kw: float,
    age: timedelta,
    steps: int,
) -> tuple[SocForecast, ...]:
    if not all(
        isfinite(value) for value in (energy_kwh, capacity_kwh, net_discharge_kw, max_power_kw)
    ):
        raise ValueError("energy and power must be finite")
    if capacity_kwh <= 0 or not 0 <= energy_kwh <= capacity_kwh:
        raise ValueError("energy must be within usable capacity")
    if max_power_kw < 0 or abs(net_discharge_kw) > max_power_kw:
        raise ValueError("net discharge exceeds power bound")
    if age < timedelta(0) or steps < 1:
        raise ValueError("age must be nonnegative and steps positive")

    result = []
    for index in range(steps):
        elapsed_hours = index / 12
        expected_kwh = min(capacity_kwh, max(0.0, energy_kwh - net_discharge_kw * elapsed_hours))
        expected = 100 * expected_kwh / capacity_kwh
        radius = 100 * max_power_kw * (age.total_seconds() / 3600 + elapsed_hours) / capacity_kwh
        result.append(
            SocForecast(expected, max(0.0, expected - radius), min(100.0, expected + radius))
        )
    return tuple(result)
