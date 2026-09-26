from collections.abc import Callable

from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc


def test_solver_server_prefers_validated_highs_plan(
    serve: Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub],
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    response = serve(OptimizationServer()).Optimize(optimize_request)
    assert not response.plan.fallback_used
    assert response.plan.fallback_reason == ""
    assert response.plan.solver_version == "highs"
    objective = response.plan.objective_breakdown
    assert objective.grid_value == 0.0
    assert objective.commitment_tracking_value == 0.0
    assert objective.objective_value == -(
        objective.degradation_cost + objective.penalty_exposure + objective.reliability_risk_cost
    )
    assert response.plan.shortfalls[0].shortfall_kw == 0.0
