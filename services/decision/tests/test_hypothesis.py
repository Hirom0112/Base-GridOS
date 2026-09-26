from gridos.fallback.planner import (
    DeviceState,
    PlanningInterval,
    effective_reserve_kwh,
    plan_fallback,
)
from hypothesis import given
from hypothesis import strategies as st


@st.composite
def fleets(draw: st.DrawFn) -> tuple[list[DeviceState], list[PlanningInterval]]:
    count = draw(st.integers(min_value=1, max_value=20))
    devices: list[DeviceState] = []
    for index in range(count):
        capacity = draw(st.floats(min_value=1.0, max_value=50.0, allow_nan=False))
        reserve_percent = draw(st.floats(min_value=0.0, max_value=90.0, allow_nan=False))
        reserve_kwh = capacity * reserve_percent / 100.0
        energy = draw(st.floats(min_value=reserve_kwh, max_value=capacity, allow_nan=False))
        devices.append(
            DeviceState(
                device_id=f"device-{index}",
                usable_energy_kwh=capacity,
                energy_kwh=energy,
                reserve_percent=reserve_percent,
                hardware_floor_percent=0.0,
                dynamic_override_percent=0.0,
                max_discharge_kw=draw(st.floats(min_value=0.0, max_value=20.0, allow_nan=False)),
                discharge_efficiency=draw(st.floats(min_value=0.8, max_value=1.0, allow_nan=False)),
                home_load_kw=draw(st.floats(min_value=0.0, max_value=3.0, allow_nan=False)),
                availability_probability=draw(
                    st.floats(min_value=0.0, max_value=1.0, allow_nan=False)
                ),
            )
        )
    intervals = [
        PlanningInterval(
            target_kw=draw(st.floats(min_value=0.0, max_value=200.0, allow_nan=False)),
            duration_hours=draw(st.floats(min_value=1 / 60, max_value=2.0, allow_nan=False)),
        )
        for _ in range(draw(st.integers(min_value=1, max_value=6)))
    ]
    return devices, intervals


@given(fleets())
def test_hypothesis_fallback_preserves_hard_bounds(
    case: tuple[list[DeviceState], list[PlanningInterval]],
) -> None:
    devices, intervals = case
    by_id = {device.device_id: device for device in devices}
    plan = plan_fallback(devices, intervals)

    assert all(shortfall.shortfall_kw >= 0.0 for shortfall in plan.shortfalls)
    assert all(
        shortfall.expected_kw <= shortfall.allocated_kw + 1e-9 for shortfall in plan.shortfalls
    )
    for schedule in plan.schedules:
        device = by_id[schedule.device_id]
        previous_energy = device.energy_kwh
        for planned, _interval in zip(schedule.intervals, intervals, strict=True):
            assert 0.0 <= planned.discharge_kw <= device.max_discharge_kw
            assert planned.grid_service_kw >= 0.0
            assert planned.expected_energy_kwh <= previous_energy
            assert planned.expected_energy_kwh + 1e-9 >= effective_reserve_kwh(device)
            previous_energy = planned.expected_energy_kwh
