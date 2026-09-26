from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from enum import Enum
from logging import getLogger
from multiprocessing import get_context
from multiprocessing.connection import Connection

from gridos.fallback.planner import DeviceState, FallbackPlan, PlanningInterval
from gridos.validation.plan import validate_plan

Planner = Callable[[list[DeviceState], list[PlanningInterval]], FallbackPlan]

logger = getLogger(__name__)


class SolverFault(Enum):
    TIMEOUT = "TIMEOUT"
    SOLVER_FAILED = "SOLVER_FAILED"


SolverOutcome = FallbackPlan | SolverFault


@dataclass(frozen=True, slots=True)
class Decision:
    plan: FallbackPlan
    fallback_reason: str


def _deliver(
    planner: Planner,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    connection: Connection[FallbackPlan, object],
) -> None:
    with connection:
        connection.send(planner(devices, intervals))


def solve_within_budget(
    planner: Planner,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
    budget_seconds: float,
) -> SolverOutcome:
    context = get_context("forkserver")
    receiver, sender = context.Pipe(duplex=False)
    process = context.Process(target=_deliver, args=(planner, devices, intervals, sender))
    with receiver, sender:
        try:
            process.start()
        except Exception:
            logger.exception("solver could not be launched")
            return SolverFault.SOLVER_FAILED
        sender.close()
        try:
            if not receiver.poll(budget_seconds):
                return SolverFault.TIMEOUT
            received: object = receiver.recv()
        except EOFError:
            return SolverFault.SOLVER_FAILED
        finally:
            process.kill()
            process.join()
    if isinstance(received, FallbackPlan):
        return received
    return SolverFault.SOLVER_FAILED


def resolve(
    outcome: SolverOutcome,
    fallback: FallbackPlan,
    devices: list[DeviceState],
    intervals: list[PlanningInterval],
) -> Decision:
    if isinstance(outcome, SolverFault):
        return Decision(fallback, outcome.value)
    if validate_plan(outcome, devices, intervals):
        return Decision(fallback, "INVALID_VECTOR")
    return Decision(outcome, "DETERMINISTIC_FALLBACK")
