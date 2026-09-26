from dataclasses import dataclass
from math import isfinite


@dataclass(frozen=True, slots=True)
class DeviceState:
    device_id: str
    usable_energy_kwh: float
    energy_kwh: float
    reserve_percent: float
    hardware_floor_percent: float
    dynamic_override_percent: float
    max_discharge_kw: float
    discharge_efficiency: float
    home_load_kw: float
    available: bool = True
    stale: bool = False
    maintenance: bool = False
    operating_state: str = "ON_GRID"


@dataclass(frozen=True, slots=True)
class PlanningInterval:
    target_kw: float
    duration_hours: float


@dataclass(frozen=True, slots=True)
class ScheduleInterval:
    grid_service_kw: float
    discharge_kw: float
    expected_energy_kwh: float


@dataclass(frozen=True, slots=True)
class DeviceSchedule:
    device_id: str
    intervals: tuple[ScheduleInterval, ...]


@dataclass(frozen=True, slots=True)
class Exclusion:
    device_id: str
    reason: str


@dataclass(frozen=True, slots=True)
class Shortfall:
    requested_kw: float
    allocated_kw: float
    shortfall_kw: float


@dataclass(frozen=True, slots=True)
class FallbackPlan:
    schedules: tuple[DeviceSchedule, ...]
    exclusions: tuple[Exclusion, ...]
    shortfalls: tuple[Shortfall, ...]
    fallback: bool = True


def effective_reserve_kwh(device: DeviceState) -> float:
    reserve_percent = max(
        device.hardware_floor_percent,
        device.reserve_percent,
        device.dynamic_override_percent,
    )
    return device.usable_energy_kwh * reserve_percent / 100.0


def _validate_device(device: DeviceState) -> None:
    values = (
        device.usable_energy_kwh,
        device.energy_kwh,
        device.reserve_percent,
        device.hardware_floor_percent,
        device.dynamic_override_percent,
        device.max_discharge_kw,
        device.discharge_efficiency,
        device.home_load_kw,
    )
    if not device.device_id or not all(isfinite(value) for value in values):
        raise ValueError("device values must be identified and finite")
    if device.usable_energy_kwh < 0.0 or not 0.0 <= device.energy_kwh <= device.usable_energy_kwh:
        raise ValueError("device energy is outside capacity")
    if not all(0.0 <= value <= 100.0 for value in values[2:5]):
        raise ValueError("reserve percentages must be in [0, 100]")
    if device.max_discharge_kw < 0.0 or device.home_load_kw < 0.0:
        raise ValueError("device power must be nonnegative")
    if not 0.0 < device.discharge_efficiency <= 1.0:
        raise ValueError("discharge efficiency must be in (0, 1]")


def _exclusion_reason(device: DeviceState) -> str | None:
    if device.stale:
        return "STALE_TELEMETRY"
    if device.maintenance:
        return "MAINTENANCE_LOCK"
    if not device.available:
        return "UNAVAILABLE"
    if device.operating_state != "ON_GRID":
        return "UNAVAILABLE"
    if device.energy_kwh <= effective_reserve_kwh(device):
        return "RESERVE"
    return None


def _interval_capacity_kw(device: DeviceState, energy_kwh: float, duration_hours: float) -> float:
    reserve_kwh = effective_reserve_kwh(device)
    available_ac_kwh = max(0.0, energy_kwh - reserve_kwh) * device.discharge_efficiency
    discharge_kw = min(device.max_discharge_kw, available_ac_kwh / duration_hours)
    return max(0.0, discharge_kw - device.home_load_kw)


def plan_fallback(devices: list[DeviceState], intervals: list[PlanningInterval]) -> FallbackPlan:
    for interval in intervals:
        if not isfinite(interval.target_kw) or interval.target_kw < 0.0:
            raise ValueError("target must be finite and nonnegative")
        if not isfinite(interval.duration_hours) or interval.duration_hours <= 0.0:
            raise ValueError("duration must be finite and positive")
    eligible: list[DeviceState] = []
    exclusions: list[Exclusion] = []
    seen: set[str] = set()
    for device in devices:
        _validate_device(device)
        if device.device_id in seen:
            exclusions.append(Exclusion(device.device_id, "DUPLICATE_DEVICE"))
            continue
        seen.add(device.device_id)
        reason = _exclusion_reason(device)
        if reason is None:
            eligible.append(device)
        else:
            exclusions.append(Exclusion(device.device_id, reason))
    energies = {device.device_id: device.energy_kwh for device in eligible}
    scheduled = {device.device_id: [] for device in eligible}
    shortfalls: list[Shortfall] = []
    for interval in intervals:
        ranked = sorted(
            eligible,
            key=lambda device: (
                -_interval_capacity_kw(device, energies[device.device_id], interval.duration_hours),
                device.device_id,
            ),
        )
        remaining_kw = interval.target_kw
        for device in ranked:
            capacity_kw = _interval_capacity_kw(
                device, energies[device.device_id], interval.duration_hours
            )
            grid_service_kw = min(remaining_kw, capacity_kw)
            discharge_kw = grid_service_kw + device.home_load_kw if grid_service_kw > 0.0 else 0.0
            next_energy = energies[device.device_id] - (
                discharge_kw * interval.duration_hours / device.discharge_efficiency
            )
            energies[device.device_id] = next_energy
            scheduled[device.device_id].append(
                ScheduleInterval(grid_service_kw, discharge_kw, next_energy)
            )
            remaining_kw -= grid_service_kw
        allocated_kw = interval.target_kw - remaining_kw
        shortfalls.append(Shortfall(interval.target_kw, allocated_kw, remaining_kw))
    schedules = tuple(
        DeviceSchedule(device.device_id, tuple(scheduled[device.device_id]))
        for device in eligible
        if any(item.grid_service_kw > 0.0 for item in scheduled[device.device_id])
    )
    schedules = tuple(
        sorted(
            schedules,
            key=lambda schedule: (-schedule.intervals[0].grid_service_kw, schedule.device_id),
        )
    )
    return FallbackPlan(schedules, tuple(exclusions), tuple(shortfalls))
