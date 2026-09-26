from dataclasses import replace

from gridos.fallback.planner import DeviceState, PlanningInterval, plan_fallback
from gridos.validation.plan import validate_plan


def decision_inputs() -> tuple[list[DeviceState], list[PlanningInterval]]:
    return (
        [
            DeviceState(
                device_id="device-a",
                usable_energy_kwh=10.0,
                energy_kwh=8.0,
                reserve_percent=40.0,
                hardware_floor_percent=10.0,
                dynamic_override_percent=0.0,
                max_discharge_kw=5.0,
                discharge_efficiency=0.95,
                home_load_kw=1.0,
            )
        ],
        [PlanningInterval(target_kw=3.0, duration_hours=0.25)],
    )


def test_validation_accepts_a_feasible_plan() -> None:
    devices, intervals = decision_inputs()
    plan = plan_fallback(devices, intervals)

    assert validate_plan(plan, devices, intervals) == ()


def test_validation_rejects_nonfinite_values_and_wrong_vector_length() -> None:
    devices, intervals = decision_inputs()
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    invalid_interval = replace(schedule.intervals[0], discharge_kw=float("nan"))
    invalid_plan = replace(
        plan,
        schedules=(replace(schedule, intervals=(invalid_interval, invalid_interval)),),
    )

    codes = {violation.code for violation in validate_plan(invalid_plan, devices, intervals)}

    assert "NONFINITE_VALUE" in codes
    assert "VECTOR_LENGTH" in codes


def test_validation_reconstructs_energy_and_reserve() -> None:
    devices, intervals = decision_inputs()
    plan = plan_fallback(devices, intervals)
    schedule = plan.schedules[0]
    unsafe = replace(schedule.intervals[0], discharge_kw=5.0, expected_energy_kwh=3.0)
    invalid_plan = replace(plan, schedules=(replace(schedule, intervals=(unsafe,)),))

    codes = {violation.code for violation in validate_plan(invalid_plan, devices, intervals)}

    assert "ENERGY_BALANCE" in codes
    assert "RESERVE" in codes
