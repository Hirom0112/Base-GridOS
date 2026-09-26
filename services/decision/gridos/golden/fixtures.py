import json
from dataclasses import asdict, replace
from pathlib import Path

from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback
from gridos.fallback.replacement import replace_dropped
from gridos.optimization.model import optimize
from gridos.solver.bounded import Decision, SolverFault, SolverOutcome, resolve
from gridos.validation.plan import validate_plan


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


def _fixture(
    name: str,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    decision: Decision,
    *,
    policy_context: str,
) -> dict[str, object]:
    violations = validate_plan(decision.plan, devices, intervals)
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
        "expected_dispatch_plan": {
            **_plan_dict(decision.plan),
            "fallback_reason": decision.fallback_reason,
        },
        "expected_validation": {
            "approved": not violations,
            "violation_families": sorted({violation.code for violation in violations}),
        },
    }


def _pair(
    name: str,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    *,
    policy_context: str,
) -> dict[str, object]:
    decision = Decision(optimize(devices, intervals), "")
    return _fixture(name, devices, intervals, decision, policy_context=policy_context)


def _resolved(
    name: str,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    outcome: SolverOutcome,
) -> dict[str, object]:
    fallback = plan_fallback(devices, intervals)
    decision = resolve(outcome, fallback, devices, intervals)
    return _fixture(name, devices, intervals, decision, policy_context="BASE")


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


def _invalid_vector(
    devices: list[DeviceState], intervals: list[PlanningInterval]
) -> dict[str, object]:
    solved = plan_fallback(devices, intervals)
    schedule = solved.schedules[0]
    device = devices[0]
    overreach = replace(
        schedule.intervals[0],
        discharge_kw=6.0,
        expected_energy_kwh=device.energy_kwh
        - 6.0 * intervals[0].duration_hours / device.discharge_efficiency,
    )
    rejected = replace(
        solved,
        schedules=(replace(schedule, intervals=(overreach,)), *solved.schedules[1:]),
    )
    families = sorted({violation.code for violation in validate_plan(rejected, devices, intervals)})
    return {
        **_resolved("invalid-vector", devices, intervals, rejected),
        "rejected_solver_plan": _plan_dict(rejected),
        "rejected_violation_families": families,
    }


def _replacement() -> dict[str, object]:
    envelope = frozenset({"device-a", "device-b", "device-c"})
    dropped = frozenset({"device-a"})
    approved_devices = [
        replace(_device("device-a"), max_discharge_kw=4.0),
        _device("device-b"),
        replace(_device("device-c"), max_discharge_kw=3.0),
    ]
    approved = plan_fallback(approved_devices, [PlanningInterval(7.0, 0.25)])
    current = [replace(device, stale=device.device_id in dropped) for device in approved_devices]
    lost = [
        PlanningInterval(
            sum(
                schedule.intervals[0].grid_service_kw
                for schedule in approved.schedules
                if schedule.device_id in dropped
            ),
            0.25,
        )
    ]
    proposal = replace_dropped(approved, current, envelope, dropped, lost)
    return {
        **_fixture(
            "replacement",
            current,
            lost,
            Decision(proposal, "DETERMINISTIC_FALLBACK"),
            policy_context="BASE",
        ),
        "replacement": {
            "approved_plan": _plan_dict(approved),
            "envelope": sorted(envelope),
            "dropped": sorted(dropped),
        },
    }


def _hardening_fixtures() -> dict[str, dict[str, object]]:
    interval = [PlanningInterval(3.0, 0.25)]
    devices = [_device("device-a"), _device("device-b")]
    return {
        "timeout": {
            **_resolved("timeout", devices, interval, SolverFault.TIMEOUT),
            "solver_outcome": SolverFault.TIMEOUT.value,
        },
        "no-incumbent": {
            **_resolved("no-incumbent", devices, interval, SolverFault.SOLVER_FAILED),
            "solver_outcome": SolverFault.SOLVER_FAILED.value,
        },
        "invalid-vector": _invalid_vector(devices, interval),
        "replacement": _replacement(),
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
        **_hardening_fixtures(),
    }


def write_fixtures(directory: Path) -> None:
    directory.mkdir(parents=True, exist_ok=True)
    for name, fixture in render_fixtures().items():
        path = directory / f"{name}.json"
        path.write_text(json.dumps(fixture, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    write_fixtures(Path(__file__).parents[4] / "testdata" / "golden" / "plans")
