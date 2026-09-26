from pydantic import ValidationError
from pytest import raises

from tools.generation.scenario.model import Scenario

VALID_SCENARIO = {
    "name": "validation-control",
    "provenance": "SIMULATED",
    "clock": {"seed": 7, "start_at": "2026-08-12T16:00:00-05:00", "interval_seconds": 300},
    "fleet": {"path": "testdata/fleets/texas-50.jsonl", "size": 50},
    "event": {
        "region": "LZ_HOUSTON",
        "start_at": "2026-08-12T18:00:00-05:00",
        "end_at": "2026-08-12T20:00:00-05:00",
        "target_mw": 0.2,
        "boundary": "METER_NET_EXPORT",
    },
    "injections": [{"at": "2026-08-12T18:30:00-05:00", "kind": "OFFLINE_DEVICES"}],
    "expected": {
        "final_event_state": "REPORTED",
        "reserve_violations": 0,
        "allows_shortfall": True,
        "required_recovery_actions": ["REMOVE_STALE_CAPACITY", "REBALANCE"],
    },
}


def test_valid_scenario_is_accepted() -> None:
    scenario = Scenario.model_validate(VALID_SCENARIO)
    assert scenario.name == "validation-control"


def test_invalid_scenario_is_rejected() -> None:
    invalid = {**VALID_SCENARIO, "event": {**VALID_SCENARIO["event"], "target_mw": -1}}
    with raises(ValidationError):
        Scenario.model_validate(invalid)


def test_scheduled_injection_scope_is_validated() -> None:
    scheduled = {
        **VALID_SCENARIO,
        "injections": [{**VALID_SCENARIO["injections"][0], "scope": "scheduled"}],
    }
    assert Scenario.model_validate(scheduled).injections[0].scope == "scheduled"
    misspelled = {
        **VALID_SCENARIO,
        "injections": [{**scheduled["injections"][0], "scpoe": "scheduled"}],
    }
    with raises(ValidationError):
        Scenario.model_validate(misspelled)
