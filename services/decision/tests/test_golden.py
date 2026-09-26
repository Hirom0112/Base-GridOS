import json
from pathlib import Path

from gridos.golden.fixtures import render_fixtures


def load_checked_in() -> dict[str, dict[str, object]]:
    fixture_directory = Path(__file__).parents[3] / "testdata" / "golden" / "plans"
    return {
        path.stem: json.loads(path.read_text()) for path in sorted(fixture_directory.glob("*.json"))
    }


def test_golden_plans_match_regenerated_fixtures() -> None:
    checked_in = load_checked_in()

    assert set(checked_in) == {
        "duplicate-device",
        "expired-window",
        "feasible",
        "infeasible-shortfall",
        "invalid-vector",
        "no-incumbent",
        "replacement",
        "reserve-tight",
        "stale-device-excluded",
        "timeout",
        "weather-override",
        "zero-percent-hardware-floor",
    }
    assert checked_in == render_fixtures()


def test_golden_hardening_cases_carry_their_fallback_reason() -> None:
    fixtures = load_checked_in()
    reasons = {
        name: fixtures[name]["expected_dispatch_plan"]["fallback_reason"]
        for name in ("feasible", "timeout", "no-incumbent", "invalid-vector", "replacement")
    }

    assert reasons == {
        "feasible": "",
        "timeout": "TIMEOUT",
        "no-incumbent": "SOLVER_FAILED",
        "invalid-vector": "INVALID_VECTOR",
        "replacement": "DETERMINISTIC_FALLBACK",
    }
    assert fixtures["timeout"]["solver_outcome"] == "TIMEOUT"
    assert fixtures["no-incumbent"]["solver_outcome"] == "SOLVER_FAILED"
    assert all(fixtures[name]["expected_validation"]["approved"] for name in reasons)


def test_golden_invalid_vector_records_the_rejected_solver_plan() -> None:
    fixture = load_checked_in()["invalid-vector"]
    rejected = fixture["rejected_solver_plan"]

    assert rejected["schedules"][0]["intervals"][0]["discharge_kw"] == 6.0
    assert "POWER_BOUND" in fixture["rejected_violation_families"]
    assert fixture["expected_dispatch_plan"]["schedules"][0]["intervals"][0]["discharge_kw"] == 4.0


def test_golden_replacement_records_envelope_and_dropout() -> None:
    fixture = load_checked_in()["replacement"]

    assert fixture["replacement"]["dropped"] == ["device-a"]
    assert fixture["replacement"]["envelope"] == ["device-a", "device-b", "device-c"]
    assert fixture["optimization_request"]["intervals"][0]["target_kw"] == 3.0
    assert [s["device_id"] for s in fixture["expected_dispatch_plan"]["schedules"]] == ["device-c"]
    assert fixture["expected_dispatch_plan"]["shortfalls"][0]["shortfall_kw"] == 1.0
