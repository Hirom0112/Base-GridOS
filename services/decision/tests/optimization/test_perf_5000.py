import json
from pathlib import Path
from time import perf_counter

from gridos.fallback.planner import DeviceState, PlanningInterval
from gridos.optimization.model import optimize, optimize_with_budget


def test_perf_5000() -> None:
    fleet_path = Path(__file__).resolve().parents[4] / "testdata/fleets/austin-5000.jsonl"
    devices = []
    for line in fleet_path.read_text().splitlines():
        row = json.loads(line)
        devices.append(
            DeviceState(
                device_id=row["device_id"],
                usable_energy_kwh=row["usable_energy_kwh"],
                energy_kwh=0.8 * row["usable_energy_kwh"],
                reserve_percent=row["reserve_preference_percent"],
                hardware_floor_percent=10.0,
                dynamic_override_percent=0.0,
                max_discharge_kw=row["max_discharge_kw"],
                discharge_efficiency=row["discharge_efficiency"],
                home_load_kw=0.0,
            )
        )
    assert len(devices) == 5000
    intervals = [PlanningInterval(3000.0, 1.0 / 12.0) for _ in range(24)]
    started = perf_counter()
    plan = optimize(devices, intervals)
    assert perf_counter() - started < 10.0
    assert len(plan.shortfalls) == 24
    assert not plan.fallback
    timed_out = optimize_with_budget(devices[:50], intervals, 0.0)
    assert timed_out.fallback
    assert len(timed_out.shortfalls) == 24
