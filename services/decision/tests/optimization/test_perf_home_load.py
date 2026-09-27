import json
from pathlib import Path
from time import perf_counter

from gridos.fallback.planner import DeviceState, PlanningInterval
from gridos.optimization.model import optimize
from gridos.validation.plan import validate_plan


def test_full_fleet_with_home_load_plans_quickly_and_validly() -> None:
    fleet_path = Path(__file__).resolve().parents[4] / "testdata/fleets/austin-5000.jsonl"
    devices = [
        DeviceState(
            device_id=row["device_id"],
            usable_energy_kwh=row["usable_energy_kwh"],
            energy_kwh=0.8 * row["usable_energy_kwh"],
            reserve_percent=row["reserve_preference_percent"],
            hardware_floor_percent=10.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=row["max_discharge_kw"],
            discharge_efficiency=row["discharge_efficiency"],
            home_load_kw=1.2,
        )
        for row in map(json.loads, fleet_path.read_text().splitlines())
    ]
    intervals = [PlanningInterval(3000.0, 1.0 / 12.0) for _ in range(12)]

    started = perf_counter()
    plan = optimize(devices, intervals)
    elapsed = perf_counter() - started

    assert elapsed < 10.0
    assert plan.schedules
    assert not validate_plan(plan, devices, intervals)
