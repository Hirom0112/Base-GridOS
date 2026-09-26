from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval, plan_fallback


def replace_dropped(
    approved: FallbackPlan,
    devices: list[DeviceState],
    envelope: frozenset[str],
    dropped: frozenset[str],
    intervals: list[PlanningInterval],
) -> FallbackPlan:
    lost_kw = [0.0] * len(intervals)
    surviving: set[str] = set()
    for schedule in approved.schedules:
        if len(schedule.intervals) != len(intervals):
            raise ValueError("approved schedule length does not match the intervals")
        if schedule.device_id not in dropped:
            surviving.add(schedule.device_id)
            continue
        for index, planned in enumerate(schedule.intervals):
            lost_kw[index] += planned.grid_service_kw
    candidates = [
        device
        for device in devices
        if device.device_id in envelope
        and device.device_id not in dropped
        and device.device_id not in surviving
    ]
    replacement_intervals = [
        PlanningInterval(lost, interval.duration_hours)
        for lost, interval in zip(lost_kw, intervals, strict=True)
    ]
    return plan_fallback(candidates, replacement_intervals)
