from collections.abc import Sequence
from math import isfinite


def _nonnegative(name: str, value: float) -> None:
    if not isfinite(value) or value < 0.0:
        raise ValueError(f"{name} must be finite and nonnegative")


def _efficiency(value: float) -> None:
    if not isfinite(value) or not 0.0 < value <= 1.0:
        raise ValueError("efficiency must be finite and in (0, 1]")


def stored_energy_kwh(usable_energy_kwh: float, state_of_energy_percent: float) -> float:
    _nonnegative("usable_energy_kwh", usable_energy_kwh)
    if not isfinite(state_of_energy_percent) or not 0.0 <= state_of_energy_percent <= 100.0:
        raise ValueError("state_of_energy_percent must be in [0, 100]")
    return usable_energy_kwh * state_of_energy_percent / 100.0


def reserve_energy_kwh(usable_energy_kwh: float, reserve_percent: float) -> float:
    return stored_energy_kwh(usable_energy_kwh, reserve_percent)


def ac_energy_above_reserve(
    energy_kwh: float, reserve_kwh: float, discharge_efficiency: float
) -> float:
    _nonnegative("energy_kwh", energy_kwh)
    _nonnegative("reserve_kwh", reserve_kwh)
    _efficiency(discharge_efficiency)
    return max(0.0, energy_kwh - reserve_kwh) * discharge_efficiency


def energy_limited_discharge_kw(
    permitted_discharge_kw: float, available_ac_energy_kwh: float, interval_hours: float
) -> float:
    _nonnegative("permitted_discharge_kw", permitted_discharge_kw)
    _nonnegative("available_ac_energy_kwh", available_ac_energy_kwh)
    if not isfinite(interval_hours) or interval_hours <= 0.0:
        raise ValueError("interval_hours must be finite and positive")
    return min(permitted_discharge_kw, available_ac_energy_kwh / interval_hours)


def update_energy_kwh(
    energy_kwh: float,
    charge_kw: float,
    discharge_kw: float,
    interval_hours: float,
    charge_efficiency: float,
    discharge_efficiency: float,
) -> float:
    for name, value in (
        ("energy_kwh", energy_kwh),
        ("charge_kw", charge_kw),
        ("discharge_kw", discharge_kw),
    ):
        _nonnegative(name, value)
    if charge_kw > 0.0 and discharge_kw > 0.0:
        raise ValueError("charge and discharge are mutually exclusive")
    if not isfinite(interval_hours) or interval_hours <= 0.0:
        raise ValueError("interval_hours must be finite and positive")
    _efficiency(charge_efficiency)
    _efficiency(discharge_efficiency)
    return (
        energy_kwh
        + charge_efficiency * charge_kw * interval_hours
        - discharge_kw * interval_hours / discharge_efficiency
    )


def preserves_reserve(energy_kwh: float, reserve_kwh: float) -> bool:
    _nonnegative("energy_kwh", energy_kwh)
    _nonnegative("reserve_kwh", reserve_kwh)
    return energy_kwh >= reserve_kwh


def backup_duration_hours(
    available_dc_energy_kwh: float, discharge_efficiency: float, critical_load_kw: float
) -> float | None:
    _nonnegative("available_dc_energy_kwh", available_dc_energy_kwh)
    _efficiency(discharge_efficiency)
    _nonnegative("critical_load_kw", critical_load_kw)
    if critical_load_kw == 0.0:
        return None
    return available_dc_energy_kwh * discharge_efficiency / critical_load_kw


def backup_duration_with_forecast(
    available_dc_energy_kwh: float,
    discharge_efficiency: float,
    critical_load_kw: Sequence[float],
    interval_hours: float,
) -> float:
    _nonnegative("available_dc_energy_kwh", available_dc_energy_kwh)
    _efficiency(discharge_efficiency)
    if not isfinite(interval_hours) or interval_hours <= 0.0:
        raise ValueError("interval_hours must be finite and positive")
    remaining_ac_kwh = available_dc_energy_kwh * discharge_efficiency
    duration = 0.0
    for load_kw in critical_load_kw:
        _nonnegative("critical_load_kw", load_kw)
        interval_energy = load_kw * interval_hours
        if interval_energy > remaining_ac_kwh and load_kw > 0.0:
            return duration + remaining_ac_kwh / load_kw
        remaining_ac_kwh -= interval_energy
        duration += interval_hours
    return duration
