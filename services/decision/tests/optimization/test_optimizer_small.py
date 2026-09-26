from time import perf_counter

from gridos.fallback.planner import DeviceState, PlanningInterval, effective_reserve_kwh
from gridos.optimization.model import optimize


def test_optimizer_small() -> None:
    devices = [
        DeviceState(
            device_id=f"device-{index:02d}",
            usable_energy_kwh=4.0,
            energy_kwh=4.0 if index < 30 else 1.0,
            reserve_percent=25.0,
            hardware_floor_percent=10.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=1.0,
            discharge_efficiency=1.0,
            home_load_kw=0.0,
        )
        for index in range(50)
    ]
    intervals = [PlanningInterval(30.0, 1.0 / 12.0) for _ in range(24)]
    started = perf_counter()
    plan = optimize(devices, intervals)
    assert perf_counter() - started < 1.0
    assert len(plan.shortfalls) == len(intervals)
    assert all(abs(item.shortfall_kw) < 1e-6 for item in plan.shortfalls)
    assert all(abs(item.expected_kw - 30.0) < 1e-6 for item in plan.shortfalls)
    assert len(plan.schedules) == 30
    for schedule in plan.schedules:
        device = next(item for item in devices if item.device_id == schedule.device_id)
        assert len(schedule.intervals) == len(intervals)
        assert all(
            item.expected_energy_kwh >= effective_reserve_kwh(device) - 1e-6
            for item in schedule.intervals
        )
        assert all(
            0.0 <= item.discharge_kw <= device.max_discharge_kw + 1e-6
            for item in schedule.intervals
        )
