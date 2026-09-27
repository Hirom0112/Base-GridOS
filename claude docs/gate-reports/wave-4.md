# Gate 4 report

**Run:** 2026-09-27 by the director on `main`, in a declared gate window
with lane stacks, load tests, and Bazel runs paused.
**Result:** backend GREEN, every required scenario passing on the director's
machine after the reruns below; the UI leg stays open on the console track.

## Required scenarios (TECHSTACK, seventeen)

Run in five sequential groups on isolated stacks (`tests/integration`,
`-count=1`), each scenario inside a one-minute live window.

```text
GROUP ONE (durability)
--- PASS: TestAuditChain (111.67s)
--- PASS: TestHarness (101.85s)
--- PASS: TestWorkerTermination (102.02s)
--- PASS: TestGatewayRestart (102.33s)
--- PASS: TestOutageReplay (112.90s)          (rerun alone after the load sweep)
GROUP TWO (failure handling)
--- PASS: TestUnderReservedExcluded (112.41s)
--- PASS: TestHoustonTwentyPercentOffline (101.79s)
--- PASS: TestMeasurementGapUnknown (101.77s)
--- PASS: TestLostAckStillExecuting (111.96s)   rerun after the report-energy fix 7735ce3 (first pass: report energy must be finite and nonnegative)
--- PASS: TestOldExpiryNewerPending (101.72s)   rerun after the same fix
GROUP THREE (planning)
--- PASS: TestHeatEventCanonical (112.27s)   rerun after scope: scheduled and the scaled verification cadence; MISSING exception asserted
--- PASS: TestInfeasibleTargetShortfall (101.91s)
--- PASS: TestOptimizerTimeoutFallback (101.78s)
GROUP FOUR (flexibility, part one)
--- PASS: TestZeroPercentReservePreservesHardwareFloor (111.25s)
--- PASS: TestTravelFlexLifecycle (105.04s)   rerun after the trace identity fix (first pass failed the workflow on "invalid trace identity")
--- PASS: TestWeatherStaleAlarmRaiseFloor (141.78s)
GROUP FIVE (flexibility, part two)
--- PASS: TestAnomalySignalLabel (111.16s)
--- PASS: TestNegativeMarginNoFlexDispatch (101.68s)
--- PASS: TestNoFeeMarketReward (101.48s)
```

## FULL_SPEC §10 acceptance list, item by item

| Acceptance item | Proof command | Result |
| --- | --- | --- |
| Ingest and normalize public market, load, weather, outage | `go test ./services/control/internal/context/` | ok (4D.1) |
| Deterministic multi-thousand-device fleet, replayable | `uv run --project tools/generation pytest tools/generation` | 17 passed; replay `IDENTICAL` |
| Display fleet health, reserve, availability, context, event | console track (U1, U2, U4) | open on the UI track |
| Feasible rolling-horizon plan with per-device schedules | `-k optimizer_small` and `TestHeatEventCanonical` | 111 s, 149 HiGHS schedules on the demo; canonical 112 s |
| Reject a plan breaching reserve, power, energy, freshness, window | `go test ./services/control/internal/safety/ -run Golden` | 36 subtests ok |
| Explicit operator approval for the canonical event | `-run StepUp` and `TestHeatEventCanonical` | ok (5E.2) |
| Resume an in-flight event after worker restart | `TestWorkerTermination` | 102 s ok |
| Repeated command delivery idempotent | `make test-e2e` (duplicate delivery) | ok |
| Distinguish intent, acknowledgement, verified delivery | `TestHarness`, `TestMeasurementGapUnknown` | harness 102 s, measurement gap 102 s |
| Inject offline devices, delayed telemetry, duplicates, optimizer timeout | `TestHoustonTwentyPercentOffline`, `TestLostAckStillExecuting`, `TestOptimizerTimeoutFallback` | Houston 102 s, lost ack 112 s, timeout fallback 102 s |
| Report with provenance, input versions, plan version | `-run Immutable`, `-run LiveReport`, `GetEventReport` fixtures | ok (4E) |
| Visibly label every simulated or modeled result | value_kind and provenance on every response; console badge | backend ok, UI open |
| Only consented, effective-dated resilience and Travel Flex policies | `TestTravelFlexLifecycle` | 105 s |
| Travel Flex expires automatically; early return handled | `TestTravelFlexLifecycle` | 105 s |
| Severe weather, stale telemetry, alarms raise the floor | `TestWeatherStaleAlarmRaiseFloor`; demo COMMUNICATIONS override | 142 s; demo override at 02:30Z |
| Decline dispatch when conservative margin is negative | `TestNegativeMarginNoFlexDispatch` | 102 s |
| Preserve pricing, reward, consent versions per decision | `TestNoFeeMarketReward`, `-run "Offer|Ledger"` | 101 s; six policy tests |
| Away-mode alerts described as energy anomalies | `TestAnomalySignalLabel` | 111 s |
| Operator read p95 under 500 ms | `go test ./tests/load/ -run DispatchPath` | read_p95 3.05 ms |
| Command intent persisted before delivery | same | sent_before_persisted=0 |
| Safety validation under 2 s for an event cohort | `-bench Validate5000x288` | 0.28 s |
| Planning under 10 s for the canonical 5,000-device scenario | `-k perf_5000` | 1.45 s |
| New telemetry in the live view within 5 s | `go test ./tests/end-to-end/ -run IngestScale` | ok against the demo |
| No reserve violation in property tests or failure scenarios | `-k hypothesis_solver`, `TestUnderReservedExcluded`, `TestZeroPercentReservePreservesHardwareFloor` | under-reserved 112 s, zero-percent 111 s |
| Same seed and versioned inputs, same outcome | `go run ./services/control/cmd/replay --event <id>` | IDENTICAL |

## UI leg

`playwright test demo-path`: steps 2, 3, 9, 13, 16 pass on the last run;
steps 10 to 15 wait on the console asserting the typed exception entries
(2F.11) against a demo started in live scenario mode (2A.10); steps 1, 4 to
8, and 17 are console screens still owed. Gate 4's UI condition stays open
on the console track.

## Defects found and fixed inside the gate

Five real defects surfaced during the sweep and were fixed before the
scenarios went green: the publisher stopping after one batch and the
workflow reporting before the event window (earlier waves), and in this
gate a report source that rejected events with uncertain commands, an
activity that failed on a trace attribute, a geo drill-down that panicked
without a fleet snapshot, and a canonical scenario whose injections never
hit dispatched devices. Two further defects remain open as Gate 4 blockers
for the demo path rather than the scenario suite: per-device command
generations across events (2B.9, emergency stop rejected as obsolete on a
fleet that has run before) and automatic recovery inside the envelope
(2B.10). The gate closes on the suite; the report records both items as
open for the console leg.
