from enum import StrEnum
from pathlib import Path
from typing import Literal, Self

import yaml
from pydantic import (
    AwareDatetime,
    BaseModel,
    Field,
    NonNegativeInt,
    PositiveFloat,
    PositiveInt,
    model_validator,
)


class MeasurementBoundary(StrEnum):
    BATTERY_TERMINAL = "BATTERY_TERMINAL"
    METER_NET_EXPORT = "METER_NET_EXPORT"
    IMPORT_REDUCTION_VS_BASELINE = "IMPORT_REDUCTION_VS_BASELINE"


class InjectionType(StrEnum):
    OFFLINE_DEVICES = "OFFLINE_DEVICES"
    DELAYED_TELEMETRY = "DELAYED_TELEMETRY"
    DROPPED_MESSAGES = "DROPPED_MESSAGES"
    DUPLICATED_MESSAGES = "DUPLICATED_MESSAGES"
    GATEWAY_RESTART = "GATEWAY_RESTART"
    WORKER_RESTART = "WORKER_RESTART"
    PARTIAL_REGION_OUTAGE = "PARTIAL_REGION_OUTAGE"
    BAD_FORECASTS = "BAD_FORECASTS"
    HOT_BATTERIES = "HOT_BATTERIES"
    STALE_STATE = "STALE_STATE"
    OPTIMIZER_TIMEOUT = "OPTIMIZER_TIMEOUT"


class RecoveryAction(StrEnum):
    RETRY = "RETRY"
    REMOVE_STALE_CAPACITY = "REMOVE_STALE_CAPACITY"
    REBALANCE = "REBALANCE"


class SeededClock(BaseModel):
    seed: int
    start_at: AwareDatetime
    interval_seconds: PositiveInt


class FleetReference(BaseModel):
    path: Path
    size: PositiveInt


class EventDefinition(BaseModel):
    region: str = Field(min_length=1)
    start_at: AwareDatetime
    end_at: AwareDatetime
    target_mw: PositiveFloat
    boundary: MeasurementBoundary

    @model_validator(mode="after")
    def require_forward_window(self) -> Self:
        if self.end_at <= self.start_at:
            raise ValueError("event end must follow start")
        return self


class TimedInjection(BaseModel):
    at: AwareDatetime
    kind: InjectionType


class ExpectedOutcomes(BaseModel):
    final_event_state: Literal["VERIFIED", "RECONCILED", "REPORTED"]
    reserve_violations: NonNegativeInt
    allows_shortfall: bool
    required_recovery_actions: list[RecoveryAction]


class Scenario(BaseModel):
    name: str = Field(min_length=1)
    provenance: Literal["SIMULATED"]
    clock: SeededClock
    fleet: FleetReference
    event: EventDefinition
    injections: list[TimedInjection]
    expected: ExpectedOutcomes

    @model_validator(mode="after")
    def require_ordered_timeline(self) -> Self:
        if self.clock.start_at > self.event.start_at:
            raise ValueError("clock must start no later than event")
        if any(injection.at < self.clock.start_at for injection in self.injections):
            raise ValueError("injection cannot precede clock")
        if any(injection.at > self.event.end_at for injection in self.injections):
            raise ValueError("injection cannot follow event")
        return self

    @classmethod
    def from_yaml(cls, path: Path) -> Self:
        payload: object = yaml.safe_load(path.read_text())
        return cls.model_validate(payload)
