from dataclasses import dataclass

from gridos.fallback.planner import (
    DeviceSchedule,
    DeviceState,
    PlanningInterval,
    ScheduleInterval,
    effective_reserve_kwh,
    exclusion_reason,
)


@dataclass(frozen=True, slots=True)
class Cohort:
    devices: tuple[DeviceState, ...]


def build_cohorts(devices: list[DeviceState]) -> tuple[Cohort, ...]:
    grouped: dict[tuple[float, float, float, float], list[DeviceState]] = {}
    for device in devices:
        if exclusion_reason(device) is not None:
            continue
        key = (
            device.max_discharge_kw,
            device.discharge_efficiency,
            device.home_load_kw,
            device.availability_probability,
        )
        grouped.setdefault(key, []).append(device)
    return tuple(Cohort(tuple(group)) for group in grouped.values())


def disaggregate(
    cohort: Cohort,
    service_kw: tuple[float, ...],
    intervals: list[PlanningInterval],
) -> tuple[DeviceSchedule, ...]:
    if len(service_kw) != len(intervals):
        raise ValueError("cohort plan length differs from horizon")
    energies = {device.device_id: device.energy_kwh for device in cohort.devices}
    schedules: dict[str, list[ScheduleInterval]] = {
        device.device_id: [] for device in cohort.devices
    }
    for target, interval in zip(service_kw, intervals, strict=True):
        if target < 0.0:
            raise ValueError("cohort service must be nonnegative")
        remaining = target
        for device in sorted(
            cohort.devices,
            key=lambda member: -(energies[member.device_id] - effective_reserve_kwh(member)),
        ):
            energy = energies[device.device_id]
            reserve = effective_reserve_kwh(device)
            max_service = max(0.0, device.max_discharge_kw - device.home_load_kw)
            ratio = device.max_discharge_kw / max_service if max_service else 0.0
            energy_limit = (
                (energy - reserve) * device.discharge_efficiency / interval.duration_hours
            )
            dispatch = min(remaining, max_service, energy_limit / ratio if ratio else 0.0)
            dispatch = max(0.0, dispatch)
            discharge = dispatch * ratio
            energies[device.device_id] = (
                energy - discharge * interval.duration_hours / device.discharge_efficiency
            )
            schedules[device.device_id].append(
                ScheduleInterval(dispatch, discharge, energies[device.device_id])
            )
            remaining -= dispatch
        if remaining > 1e-8:
            raise ValueError("cohort plan cannot be disaggregated safely")
    return tuple(
        DeviceSchedule(device.device_id, tuple(schedules[device.device_id]))
        for device in cohort.devices
    )
