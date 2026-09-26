import pytest
from gridos.physics.energy import (
    ac_energy_above_reserve,
    backup_duration_hours,
    backup_duration_with_forecast,
    energy_limited_discharge_kw,
    preserves_reserve,
    reserve_energy_kwh,
    stored_energy_kwh,
    update_energy_kwh,
)


def test_worked_physics_example() -> None:
    stored = stored_energy_kwh(39.2, 74.0)
    reserve = reserve_energy_kwh(39.2, 40.0)
    available = ac_energy_above_reserve(stored, reserve, 0.95)

    assert stored == pytest.approx(29.0080)
    assert reserve == pytest.approx(15.6800)
    assert stored - reserve == pytest.approx(13.3280)
    assert available == pytest.approx(12.6616)
    assert energy_limited_discharge_kw(10.0, available, 2.0) == pytest.approx(6.3308)
    assert 6.3308 - 3.1 == pytest.approx(3.2308)
    assert backup_duration_hours(reserve, 0.95, 3.1) == pytest.approx(4.8052)


def test_update_and_reserve_constraints() -> None:
    assert update_energy_kwh(10.0, 4.0, 0.0, 0.5, 0.9, 0.95) == pytest.approx(11.8)
    assert update_energy_kwh(10.0, 0.0, 3.8, 0.5, 0.9, 0.95) == pytest.approx(8.0)
    assert preserves_reserve(8.0, 8.0)
    assert not preserves_reserve(7.99, 8.0)


def test_energy_limited_power_and_forecast_duration() -> None:
    assert energy_limited_discharge_kw(10.0, 1.5, 0.25) == pytest.approx(6.0)
    assert backup_duration_with_forecast(2.0, 0.95, [1.0, 1.0, 3.0], 0.5) == pytest.approx(1.3)


@pytest.mark.parametrize(
    ("charge_kw", "discharge_kw"),
    [(-1.0, 0.0), (0.0, -1.0), (1.0, 1.0)],
)
def test_invalid_power_is_rejected(charge_kw: float, discharge_kw: float) -> None:
    with pytest.raises(ValueError):
        update_energy_kwh(10.0, charge_kw, discharge_kw, 0.5, 0.9, 0.95)
