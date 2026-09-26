from dataclasses import replace

import pytest
from gridos.fallback.planner import DeviceState, PlanningInterval, plan_fallback
from gridos.fallback.replacement import replace_dropped
from gridos.validation.plan import validate_plan


def device(device_id: str, max_discharge_kw: float) -> DeviceState:
    return DeviceState(
        device_id=device_id,
        usable_energy_kwh=10.0,
        energy_kwh=8.0,
        reserve_percent=20.0,
        hardware_floor_percent=10.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=max_discharge_kw,
        discharge_efficiency=1.0,
        home_load_kw=0.0,
    )


FLEET = [device("a", 5.0), device("b", 5.0), device("c", 2.0), device("d", 10.0)]
ENVELOPE = frozenset({"a", "b", "c"})
INTERVALS = [PlanningInterval(target_kw=8.0, duration_hours=0.5)]


def approved_plan() -> tuple[list[DeviceState], object]:
    in_envelope = [item for item in FLEET if item.device_id in ENVELOPE]
    plan = plan_fallback(in_envelope, INTERVALS)
    assert [schedule.device_id for schedule in plan.schedules] == ["a", "b"]
    assert plan.schedules[0].intervals[0].grid_service_kw == 5.0
    return in_envelope, plan


def test_replacement_stays_inside_the_approved_envelope() -> None:
    _, approved = approved_plan()

    proposal = replace_dropped(approved, FLEET, ENVELOPE, frozenset({"a"}), INTERVALS)

    assert [schedule.device_id for schedule in proposal.schedules] == ["c"]
    assert proposal.schedules[0].intervals[0].grid_service_kw == 2.0
    assert proposal.schedules[0].intervals[0].expected_energy_kwh == 7.0
    assert proposal.shortfalls[0].requested_kw == 5.0
    assert proposal.shortfalls[0].allocated_kw == 2.0
    assert proposal.shortfalls[0].shortfall_kw == 3.0
    assert validate_plan(proposal, FLEET, [PlanningInterval(5.0, 0.5)]) == ()


def test_replacement_reports_full_shortfall_when_no_candidate_exists() -> None:
    _, approved = approved_plan()
    envelope = frozenset({"a", "b"})

    proposal = replace_dropped(approved, FLEET, envelope, frozenset({"a"}), INTERVALS)

    assert proposal.schedules == ()
    assert proposal.shortfalls[0].requested_kw == 5.0
    assert proposal.shortfalls[0].shortfall_kw == 5.0


def test_replacement_excludes_unhealthy_candidates_with_reason() -> None:
    _, approved = approved_plan()
    fleet = [replace(item, stale=True) if item.device_id == "c" else item for item in FLEET]

    proposal = replace_dropped(approved, fleet, ENVELOPE, frozenset({"a"}), INTERVALS)

    assert proposal.schedules == ()
    assert [(item.device_id, item.reason) for item in proposal.exclusions] == [
        ("c", "STALE_TELEMETRY")
    ]
    assert proposal.shortfalls[0].shortfall_kw == 5.0


def test_replacement_of_an_unscheduled_dropout_loses_nothing() -> None:
    _, approved = approved_plan()

    proposal = replace_dropped(approved, FLEET, ENVELOPE, frozenset({"c"}), INTERVALS)

    assert proposal.schedules == ()
    assert proposal.shortfalls[0].requested_kw == 0.0
    assert proposal.shortfalls[0].shortfall_kw == 0.0


def test_replacement_rejects_an_approved_plan_of_the_wrong_length() -> None:
    _, approved = approved_plan()

    with pytest.raises(ValueError, match="length"):
        replace_dropped(approved, FLEET, ENVELOPE, frozenset({"a"}), INTERVALS * 2)
