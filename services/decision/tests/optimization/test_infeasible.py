from gridos.fallback.planner import DeviceState, PlanningInterval, effective_reserve_kwh
from gridos.optimization.model import optimize


def test_infeasible_target_preserves_reserve_and_reports_each_shortfall() -> None:
    device = DeviceState(
        device_id="tight",
        usable_energy_kwh=2.0,
        energy_kwh=1.5,
        reserve_percent=50.0,
        hardware_floor_percent=10.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=1.0,
        discharge_efficiency=1.0,
        home_load_kw=0.0,
    )
    intervals = [PlanningInterval(2.0, 1.0), PlanningInterval(2.0, 1.0)]
    plan = optimize([device], intervals)
    assert [item.shortfall_kw for item in plan.shortfalls] == [1.5, 2.0]
    assert [item.expected_kw for item in plan.shortfalls] == [0.5, 0.0]
    assert plan.schedules[0].intervals[-1].expected_energy_kwh >= effective_reserve_kwh(device)
