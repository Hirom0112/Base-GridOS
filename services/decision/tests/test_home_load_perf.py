from time import perf_counter

from google.protobuf.timestamp_pb2 import Timestamp
from gridos.server import _device_states
from gridos.v1 import optimization_pb2

DEVICES = 5000
INTERVALS = 12


def test_home_load_lookup_scales_to_the_full_fleet(
    optimize_request: optimization_pb2.OptimizeRequest,
) -> None:
    request = optimize_request.request
    template = optimization_pb2.DeviceState()
    template.CopyFrom(request.devices[0])
    del request.devices[:]
    del request.intervals[:]
    del request.forecast.site_loads[:]
    for index in range(INTERVALS):
        request.intervals.add(
            begin_time=Timestamp(seconds=1_800_000_000 + 300 * index),
            end_time=Timestamp(seconds=1_800_000_300 + 300 * index),
            target_kw=3.0,
        )
    for index in range(DEVICES):
        device = request.devices.add()
        device.CopyFrom(template)
        device.device_id = f"device-{index}"
        device.site_id = f"site-{index}"
        for interval in request.intervals:
            load = request.forecast.site_loads.add(site_id=device.site_id)
            load.interval_begin_time.CopyFrom(interval.begin_time)
            load.load_kwh.value = 0.1

    started = perf_counter()
    devices = _device_states(request)
    elapsed = perf_counter() - started

    assert len(devices) == DEVICES
    assert all(device.available and device.home_load_kw > 0.0 for device in devices)
    assert elapsed < 2.0
