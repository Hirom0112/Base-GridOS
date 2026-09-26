from concurrent.futures import ThreadPoolExecutor

import grpc
import pytest
from google.protobuf.duration_pb2 import Duration
from google.protobuf.timestamp_pb2 import Timestamp
from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc, telemetry_pb2


def request() -> optimization_pb2.OptimizeRequest:
    begin = Timestamp(seconds=1_800_000_000)
    end = Timestamp(seconds=1_800_000_300)
    return optimization_pb2.OptimizeRequest(
        request=optimization_pb2.OptimizationRequest(
            request_id="request-server",
            event_id="event-server",
            plan_version=1,
            intervals=[
                optimization_pb2.OptimizationInterval(begin_time=begin, end_time=end, target_kw=3.0)
            ],
            budget=Duration(seconds=1),
            measurement_boundary=telemetry_pb2.MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
            devices=[
                optimization_pb2.DeviceState(
                    device_id="device-a",
                    usable_energy_kwh=10.0,
                    energy_kwh=8.0,
                    hardware_floor_kwh=1.0,
                    effective_reserve_kwh=4.0,
                    max_charge_kw=5.0,
                    max_discharge_kw=5.0,
                    charge_efficiency=0.95,
                    discharge_efficiency=0.95,
                    availability_probability=1.0,
                    load_zone="LZ_AEN",
                )
            ],
        )
    )


@pytest.fixture
def optimization_stub() -> optimization_pb2_grpc.OptimizationServiceStub:
    server = grpc.server(ThreadPoolExecutor(max_workers=1))
    optimization_pb2_grpc.add_OptimizationServiceServicer_to_server(OptimizationServer(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    channel = grpc.insecure_channel(f"127.0.0.1:{port}")
    try:
        yield optimization_pb2_grpc.OptimizationServiceStub(channel)
    finally:
        channel.close()
        server.stop(0).wait()


def test_server_returns_validated_fallback(
    optimization_stub: optimization_pb2_grpc.OptimizationServiceStub,
) -> None:
    response = optimization_stub.Optimize(request())

    assert response.plan.fallback_used
    assert response.plan.fallback_reason == "DETERMINISTIC_FALLBACK"
    assert response.plan.event_id == "event-server"
    assert response.plan.device_schedules[0].intervals[0].setpoint_kw == 3.0
    assert response.plan.shortfalls[0].shortfall_kw == 0.0


def test_server_rejects_missing_budget(
    optimization_stub: optimization_pb2_grpc.OptimizationServiceStub,
) -> None:
    invalid = request()
    invalid.request.budget.Clear()

    with pytest.raises(grpc.RpcError) as error:
        optimization_stub.Optimize(invalid)

    assert error.value.code() == grpc.StatusCode.INVALID_ARGUMENT
