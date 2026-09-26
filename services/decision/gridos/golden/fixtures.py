import json
from dataclasses import asdict, replace
from pathlib import Path

from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback


def _device(device_id: str) -> DeviceState:
    return DeviceState(
        device_id=device_id,
        usable_energy_kwh=10.0,
        energy_kwh=8.0,
        reserve_percent=20.0,
        hardware_floor_percent=10.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=5.0,
        discharge_efficiency=0.95,
        home_load_kw=1.0,
    )


def _pair(
    name: str,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    *,
    policy_context: str,
) -> dict[str, object]:
    plan = plan_fallback(devices, intervals)
    return {
        "name": name,
        "optimization_request": {
            "request_id": f"request-{name}",
            "event_id": f"event-{name}",
            "plan_version": 1,
            "intervals": [asdict(interval) for interval in intervals],
            "policy_context": policy_context,
        },
        "canonical_state": {"devices": [asdict(device) for device in devices]},
        "expected_dispatch_plan": _plan_dict(plan),
        "expected_validation": {"approved": True, "violation_families": []},
    }


def _plan_dict(plan: FallbackPlan) -> dict[str, object]:
    return {
        "schedules": [
            {
                "device_id": schedule.device_id,
                "intervals": [asdict(interval) for interval in schedule.intervals],
            }
            for schedule in plan.schedules
        ],
        "exclusions": [asdict(exclusion) for exclusion in plan.exclusions],
        "shortfalls": [asdict(shortfall) for shortfall in plan.shortfalls],
        "fallback": plan.fallback,
    }


def render_fixtures() -> dict[str, dict[str, object]]:
    interval = [PlanningInterval(3.0, 0.25)]
    return {
        "feasible": _pair(
            "feasible", [_device("device-a"), _device("device-b")], interval, policy_context="BASE"
        ),
        "infeasible-shortfall": _pair(
            "infeasible-shortfall",
            [replace(_device("device-a"), energy_kwh=3.0)],
            [PlanningInterval(10.0, 1.0)],
            policy_context="BASE",
        ),
        "reserve-tight": _pair(
            "reserve-tight",
            [replace(_device("device-a"), energy_kwh=4.0, reserve_percent=40.0)],
            interval,
            policy_context="BASE",
        ),
        "stale-device-excluded": _pair(
            "stale-device-excluded",
            [replace(_device("device-a"), stale=True), _device("device-b")],
            interval,
            policy_context="BASE",
        ),
        "zero-percent-hardware-floor": _pair(
            "zero-percent-hardware-floor",
            [replace(_device("device-a"), energy_kwh=1.0, reserve_percent=0.0)],
            interval,
            policy_context="GRID_FLEX",
        ),
        "weather-override": _pair(
            "weather-override",
            [replace(_device("device-a"), energy_kwh=6.0, dynamic_override_percent=60.0)],
            interval,
            policy_context="WEATHER_OVERRIDE",
        ),
        "expired-window": _pair(
            "expired-window",
            [replace(_device("device-a"), energy_kwh=4.0, reserve_percent=40.0)],
            interval,
            policy_context="TRAVEL_FLEX_EXPIRED",
        ),
        "duplicate-device": _pair(
            "duplicate-device",
            [_device("device-a"), _device("device-a")],
            interval,
            policy_context="BASE",
        ),
    }


def write_fixtures(directory: Path) -> None:
    directory.mkdir(parents=True, exist_ok=True)
    for name, fixture in render_fixtures().items():
        path = directory / f"{name}.json"
        path.write_text(json.dumps(fixture, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    write_fixtures(Path(__file__).parents[4] / "testdata" / "golden" / "plans")
