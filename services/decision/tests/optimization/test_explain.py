from math import isclose

from gridos.fallback.planner import DeviceState, PlanningInterval
from gridos.optimization.model import optimize


def test_explain_outputs_objective_margins_exclusions_and_fallback() -> None:
    devices = [
        DeviceState(
            device_id=f"device-{index}",
            usable_energy_kwh=4.0,
            energy_kwh=4.0,
            reserve_percent=25.0,
            hardware_floor_percent=10.0,
            dynamic_override_percent=0.0,
            max_discharge_kw=1.0,
            discharge_efficiency=1.0,
            home_load_kw=0.0,
            stale=index == 2,
        )
        for index in range(3)
    ]
    intervals = [PlanningInterval(1.5, 1.0)]
    plan = optimize(devices, intervals)
    assert [(item.device_id, item.reason) for item in plan.exclusions] == [
        ("device-2", "STALE_TELEMETRY")
    ]
    assert plan.objective.delivered_value > 0.0
    assert isclose(
        plan.objective.total_cost,
        plan.objective.shortfall_penalty
        + plan.objective.cycling_cost
        + plan.objective.uncertainty_cost,
    )
    assert plan.margins.reserve_kwh >= -1e-8
    assert plan.margins.power_kw >= -1e-8
    assert plan.feasible_fallback.fallback
    assert plan.feasible_fallback.shortfalls[0].shortfall_kw >= 0.0
