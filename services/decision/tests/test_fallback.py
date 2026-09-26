import json
from pathlib import Path

import pytest
from gridos.fallback.planner import DeviceState, PlanningInterval, plan_fallback


def fleet_devices() -> list[DeviceState]:
    fleet_path = Path(__file__).parents[3] / "testdata" / "fleets" / "texas-50.jsonl"
    records = [json.loads(line) for line in fleet_path.read_text().splitlines()]
    return [
        DeviceState(
            device_id=record["device_id"],
            usable_energy_kwh=record["usable_energy_kwh"],
            energy_kwh=record["usable_energy_kwh"] * 0.95,
            reserve_percent=record["reserve_preference_percent"],
            hardware_floor_percent=5.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=record["max_discharge_kw"],
            discharge_efficiency=record["discharge_efficiency"],
            home_load_kw=1.0,
            stale=index >= 30,
        )
        for index, record in enumerate(records)
    ]


def test_fallback_filters_ranks_allocates_and_reports_shortfall() -> None:
    devices = fleet_devices()
    intervals = [PlanningInterval(target_kw=200.0, duration_hours=1 / 12)] * 2
    plan = plan_fallback(devices, intervals)

    assert len(plan.schedules) == 30
    assert len(plan.exclusions) == 20
    assert all(item.reason == "STALE_TELEMETRY" for item in plan.exclusions)
    for index, interval in enumerate(intervals):
        allocated = sum(schedule.intervals[index].grid_service_kw for schedule in plan.schedules)
        assert plan.shortfalls[index].shortfall_kw == pytest.approx(interval.target_kw - allocated)
        powers = [schedule.intervals[index].grid_service_kw for schedule in plan.schedules]
        assert powers == sorted(powers, reverse=True)


def test_infeasible_target_never_lowers_reserve() -> None:
    device = DeviceState(
        device_id="reserve-tight",
        usable_energy_kwh=10.0,
        energy_kwh=4.0,
        reserve_percent=40.0,
        hardware_floor_percent=5.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=10.0,
        discharge_efficiency=0.95,
        home_load_kw=0.0,
    )

    plan = plan_fallback([device], [PlanningInterval(target_kw=10.0, duration_hours=1.0)])

    assert plan.schedules == ()
    assert plan.shortfalls[0].shortfall_kw == 10.0
