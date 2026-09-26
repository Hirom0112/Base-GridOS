import pytest
from gridos.physics.energy import (
    ac_energy_above_reserve,
    backup_duration_hours,
    energy_limited_discharge_kw,
    reserve_energy_kwh,
    stored_energy_kwh,
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
