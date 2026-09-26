from collections.abc import Callable
from datetime import UTC, datetime, timedelta

import grpc
import pytest
from google.protobuf.timestamp_pb2 import Timestamp
from gridos.fallback.planner import plan_fallback
from gridos.server import OptimizationServer, _device_states
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def test_server_returns_validated_fallback(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    response = serve(OptimizationServer(solver=plan_fallback)).Optimize(optimize_request)

    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "DETERMINISTIC_FALLBACK"
    assert response.plan.event_id == "event-server"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert response.plan.shortfalls[0].shortfall_kw == 0.0


def test_server_rejects_missing_budget(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    optimize_request.request.budget.Clear()

    with pytest.raises(grpc.RpcError) as error:
        serve(OptimizationServer()).Optimize(optimize_request)

    assert error.value.code() == grpc.StatusCode.INVALID_ARGUMENT


def test_server_forecast_uses_public_profile_and_marks_missing_source(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    begin = datetime(2025, 1, 6, 6, tzinfo=UTC)
    end = begin + timedelta(minutes=15)
    request = optimize_request.request
    request.requested_at.CopyFrom(Timestamp(seconds=int(begin.timestamp())))
    request.intervals[0].begin_time.CopyFrom(Timestamp(seconds=int(begin.timestamp())))
    request.intervals[0].end_time.CopyFrom(Timestamp(seconds=int(end.timestamp())))
    request.sites.add(site_id="known", load_profile_type="RESHIDG_COAST", load_zone="LZ_AEN")
    request.sites.add(site_id="missing", load_profile_type="UNKNOWN", load_zone="LZ_AEN")
    request.devices[0].site_id = "known"
    request.devices[0].reliability_trait = "HIGH"
    request.devices[0].telemetry_observed_at.CopyFrom(Timestamp(seconds=int(begin.timestamp())))

    response = serve(OptimizationServer()).Forecast(
        optimization_pb2.ForecastRequest(request=request)
    )

    assert len(response.site_loads) == 1
    assert response.site_loads[0].site_id == "known"
    assert response.site_loads[0].load_kwh.value > 0
    assert response.site_loads[0].load_kwh.model_version == "load-baseline-v1"
    assert len(response.device_availability) == 1
    assert response.device_availability[0].probability.value > 0
    assert "site_load:missing" in response.unavailable_sources


def test_server_optimizer_uses_frozen_availability_forecast(
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    assert _device_states(request)[0].availability_probability == 1.0
    prediction = request.forecast.device_availability.add(device_id="device-a")
    prediction.probability.value = 0.25
    prediction.probability.model_version = "availability-baseline-v1"
    prediction.probability.feature_version = "reliability-freshness-v1"
    prediction.probability.value_kind = "modeled_estimate"
    assert _device_states(request)[0].availability_probability == 0.25


def test_server_forecast_transport_timeout_forces_safe_fallback(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    optimize_request.request.forecast.unavailable_sources.append("forecast_transport_timeout")
    response = serve(OptimizationServer()).Optimize(optimize_request)
    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "FORECAST_TIMEOUT"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
