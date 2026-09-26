from collections.abc import Callable

import grpc
import pytest
from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def test_server_returns_validated_fallback(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    response = serve(OptimizationServer()).Optimize(optimize_request)

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
