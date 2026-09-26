import json

from hypothesis import given
from hypothesis import strategies as st

from tools.generation.fleet.generate import PLAN_RESERVES, generate_fleet


def _records(seed: int) -> list[dict[str, object]]:
    return [json.loads(line) for line in generate_fleet(seed=seed, size=50).splitlines()]


@given(st.integers())
def test_device_parameter_bounds(seed: int) -> None:
    for device in _records(seed):
        energy = float(device["usable_energy_kwh"])
        charge_power = float(device["max_charge_kw"])
        discharge_power = float(device["max_discharge_kw"])
        assert 10 <= energy <= 40
        assert 5 <= charge_power <= 12
        assert 5 <= discharge_power <= 12
        assert charge_power <= energy
        assert discharge_power <= energy
        assert 0.92 <= float(device["charge_efficiency"]) <= 0.97
        assert 0.92 <= float(device["discharge_efficiency"]) <= 0.97


@given(st.integers())
def test_reserve_preference_is_in_plan_band(seed: int) -> None:
    for device in _records(seed):
        plan = str(device["resilience_plan"])
        reserve = int(device["reserve_preference_percent"])
        assert reserve in PLAN_RESERVES[plan]


@given(st.integers())
def test_device_ids_are_unique(seed: int) -> None:
    identifiers = [device["device_id"] for device in _records(seed)]
    assert len(identifiers) == len(set(identifiers))
