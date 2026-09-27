from collections.abc import Callable, Iterator
from concurrent.futures import ThreadPoolExecutor

import grpc
import pytest
from google.protobuf.duration_pb2 import Duration
from google.protobuf.timestamp_pb2 import Timestamp
from gridos.server import OptimizationServer
from gridos.v1 import optimization_pb2, optimization_pb2_grpc, telemetry_pb2


@pytest.fixture
def serve() -> Iterator[
    Callable[[OptimizationServer], optimization_pb2_grpc.OptimizationServiceStub]
]:
    started: list[tuple[grpc.Server, grpc.Channel]] = []

    def start(servicer: OptimizationServer) -> optimization_pb2_grpc.OptimizationServiceStub:
        server = grpc.server(ThreadPoolExecutor(max_workers=1))
        optimization_pb2_grpc.add_OptimizationServiceServicer_to_server(servicer, server)
        port = server.add_insecure_port("127.0.0.1:0")
        server.start()
        channel = grpc.insecure_channel(f"127.0.0.1:{port}")
        started.append((server, channel))
        return optimization_pb2_grpc.OptimizationServiceStub(channel)

    yield start
    for server, channel in started:
        channel.close()
        server.stop(0).wait()


@pytest.fixture
def optimize_request() -> optimization_pb2.OptimizeRequest:
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
                    site_id="site-a",
                )
            ],
            forecast=optimization_pb2.ForecastResponse(
                site_loads=[
                    optimization_pb2.ForecastSiteLoad(
                        site_id="site-a",
                        interval_begin_time=begin,
                        load_kwh=optimization_pb2.ForecastValue(value=0.0),
                    )
                ]
            ),
        )
    )
