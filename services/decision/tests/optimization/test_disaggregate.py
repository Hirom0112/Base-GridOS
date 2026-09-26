from math import isclose

from gridos.fallback.planner import DeviceState, PlanningInterval, effective_reserve_kwh
from gridos.optimization.cohort import build_cohorts, disaggregate


def test_disaggregate_reconstructs_cohort_plan() -> None:
    devices = [
        DeviceState(
            device_id=f"site-{index}",
            usable_energy_kwh=3.0,
            energy_kwh=3.0 if index < 2 else 1.6,
            reserve_percent=50.0,
            hardware_floor_percent=10.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=1.0,
            discharge_efficiency=1.0,
            home_load_kw=0.0,
        )
        for index in range(3)
    ]
    intervals = [PlanningInterval(1.5, 1.0), PlanningInterval(1.5, 1.0)]
    cohorts = build_cohorts(devices)
    assert len(cohorts) == 1
    schedules = disaggregate(cohorts[0], (1.5, 1.5), intervals)
    assert len(schedules) == len(devices)
    for interval_index in range(len(intervals)):
        assert isclose(
            sum(schedule.intervals[interval_index].grid_service_kw for schedule in schedules),
            1.5,
            abs_tol=1e-9,
        )
    for schedule in schedules:
        device = next(item for item in devices if item.device_id == schedule.device_id)
        assert all(
            item.expected_energy_kwh >= effective_reserve_kwh(device) - 1e-9
            for item in schedule.intervals
        )
