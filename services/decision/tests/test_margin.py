from collections.abc import Callable
from dataclasses import replace

import pytest
from gridos.fallback.planner import DeviceState, PlanningInterval, plan_fallback
from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc
from gridos.validation.plan import validate_plan


def device(device_id: str, availability_probability: float) -> DeviceState:
    return DeviceState(
        device_id=device_id,
        usable_energy_kwh=10.0,
        energy_kwh=8.0,
        reserve_percent=20.0,
        hardware_floor_percent=10.0,
        dynamic_override_percent=0.0,
        max_discharge_kw=5.0,
        discharge_efficiency=1.0,
        home_load_kw=0.0,
        availability_probability=availability_probability,
    )


def test_margin_availability_reduces_counted_capacity() -> None:
    fleet = [device("a", 0.5), device("b", 0.5)]

    plan = plan_fallback(fleet, [PlanningInterval(target_kw=4.0, duration_hours=0.5)])

    setpoints = {s.device_id: s.intervals[0].grid_service_kw for s in plan.schedules}
    assert setpoints == {"a": 5.0, "b": 3.0}
    assert plan.shortfalls[0].requested_kw == 4.0
    assert plan.shortfalls[0].allocated_kw == 8.0
    assert plan.shortfalls[0].expected_kw == 4.0
    assert plan.shortfalls[0].shortfall_kw == 0.0


def test_margin_certain_devices_count_at_nameplate() -> None:
    fleet = [device("a", 1.0), device("b", 1.0)]

    plan = plan_fallback(fleet, [PlanningInterval(target_kw=4.0, duration_hours=0.5)])

    assert [s.intervals[0].grid_service_kw for s in plan.schedules] == [4.0]
    assert plan.shortfalls[0].allocated_kw == 4.0
    assert plan.shortfalls[0].expected_kw == 4.0


def test_margin_shortfall_is_measured_against_expected_delivery() -> None:
    fleet = [device("a", 0.5), device("b", 0.5)]

    plan = plan_fallback(fleet, [PlanningInterval(target_kw=6.0, duration_hours=0.5)])

    assert plan.shortfalls[0].allocated_kw == 10.0
    assert plan.shortfalls[0].expected_kw == 5.0
    assert plan.shortfalls[0].shortfall_kw == 1.0


def test_margin_zero_availability_is_excluded() -> None:
    plan = plan_fallback([device("a", 0.0)], [PlanningInterval(4.0, 0.5)])

    assert plan.schedules == ()
    assert [(e.device_id, e.reason) for e in plan.exclusions] == [("a", "UNAVAILABLE")]


def test_margin_rejects_probability_outside_unit_interval() -> None:
    with pytest.raises(ValueError, match="availability"):
        plan_fallback([device("a", 1.5)], [PlanningInterval(4.0, 0.5)])


def test_margin_validator_rejects_nameplate_presented_as_delivery() -> None:
    fleet = [device("a", 0.5), device("b", 0.5)]
    intervals = [PlanningInterval(target_kw=6.0, duration_hours=0.5)]
    plan = plan_fallback(fleet, intervals)
    assert validate_plan(plan, fleet, intervals) == ()
    nameplate = replace(plan.shortfalls[0], expected_kw=10.0, shortfall_kw=0.0)

    codes = {
        v.code for v in validate_plan(replace(plan, shortfalls=(nameplate,)), fleet, intervals)
    }

    assert "EXPECTED_MISMATCH" in codes
    assert "SHORTFALL_MISMATCH" in codes


def test_margin_response_reports_expected_not_nameplate(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    request.intervals[0].target_kw = 6.0
    request.devices[0].availability_probability = 0.5
    request.devices[0].effective_reserve_kwh = 2.0
    request.devices[0].discharge_efficiency = 1.0
    second = request.devices.add()
    second.CopyFrom(request.devices[0])
    second.device_id = "device-b"

    response = serve(OptimizationServer()).Optimize(optimize_request)

    assert [s.intervals[0].setpoint_kw for s in response.plan.device_schedules] == [5.0, 5.0]
    assert response.plan.shortfalls[0].requested_kw == 6.0
    assert response.plan.shortfalls[0].feasible_kw == 5.0
    assert response.plan.shortfalls[0].shortfall_kw == 1.0
    assert "INSUFFICIENT_FEASIBLE_CAPACITY" in response.plan.shortfalls[0].reasons
