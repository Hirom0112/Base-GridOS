from gridos.fallback.planner import DeviceState, PlanningInterval
from gridos.optimization.model import optimize
from gridos.validation.plan import validate_plan
from hypothesis import given, settings
from hypothesis import strategies as st


@given(
    home_load=st.floats(min_value=0.1, max_value=0.8, allow_nan=False),
    target=st.floats(min_value=0.05, max_value=0.8, allow_nan=False),
    reserve=st.floats(min_value=0.0, max_value=50.0, allow_nan=False),
)
@settings(max_examples=25, deadline=None)
def test_hypothesis_solver_preserves_physical_bounds(
    home_load: float, target: float, reserve: float
) -> None:
    device = DeviceState(
        device_id="site-1",
        usable_energy_kwh=4.0,
        energy_kwh=4.0,
        reserve_percent=reserve,
        hardware_floor_percent=0.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=1.0,
        discharge_efficiency=0.9,
        home_load_kw=home_load,
    )
    intervals = [PlanningInterval(target, 1.0 / 12.0) for _ in range(3)]
    plan = optimize([device], intervals)
    assert plan.shortfalls[0].allocated_kw > 0.0
    assert validate_plan(plan, [device], intervals) == ()
