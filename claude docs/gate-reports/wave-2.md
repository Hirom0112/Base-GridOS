# Gate 2 report

**Run:** 2026-09-26 and 2026-09-27 by the director on `main`.
**Result:** backend GREEN; the UI leg is open on the console track.
**Verified items:** 45 of 45 in Wave 2 (40 planned plus five opened during
the wave: 2A.7, 2A.8, 2D.7, 2F.9, 2F.10). Every item was re-run by the
director before being marked `[x]`.

## Integration scenarios, run by the director

Each scenario runs on an isolated PostgreSQL database, Temporal task queue,
and ports, in a real one-minute event window with the simulator on one shared
wall clock (`tests/integration`, `-count=1`). The whole package now exceeds
Go's default ten-minute timeout, so the director ran it in groups and the
`test-go` target carries `-timeout 40m` (2da9da5).

```text
--- PASS: TestAuditChain (105.37s)
--- PASS: TestHarness (101.62s)
--- PASS: TestWorkerTermination (101.62s)
ok  	github.com/Hirom0112/Base-GridOS/tests/integration	308.996s
--- PASS: TestGatewayRestart (109.27s)
--- PASS: TestHoustonTwentyPercentOffline (101.73s)
--- PASS: TestLostAckStillExecuting (101.53s)
--- PASS: TestUnderReservedExcluded (110.00s)
--- PASS: TestOldExpiryNewerPending (102.56s)
--- PASS: TestOutageReplay (101.58s)
--- PASS: TestMeasurementGapUnknown (101.77s)
ok  	github.com/Hirom0112/Base-GridOS/tests/integration	416.410s
```

Worker restart resumes the event (WorkerTermination), gateway restart retains
commands (GatewayRestart), a network outage replays buffered telemetry once
(OutageReplay), and the audit chain holds in every scenario (AuditChain and
the per-scenario invariant from 2D.6).

## End to end and idempotency

```text
$ make test-e2e
ok  	github.com/Hirom0112/Base-GridOS/tests/end-to-end	19.498s
$ go test ./services/control/... -run 'IngestScale|Idempot|Duplicate|Outbox|Watch|Scale'
ok  	github.com/Hirom0112/Base-GridOS/services/control/internal/storage	3.716s
ok  	github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation	1.453s
$ go test ./services/gateway-simulator/... -run 'Scale5000|Duplicate|Idempot'
ok  	github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway	1.355s
ok  	github.com/Hirom0112/Base-GridOS/services/gateway-simulator/tests	206.112s
```

The 5,000-device simulator scale test and the control ingest scale test both
pass; `WatchEvent` polling is covered by the events package tests.

## Standing demo, rebuilt on the current tree

```text
demo ready: control=:28080 decision=:25061 gateway=:28081
telemetry lag: 10 s at 00:28:24, 10 s at 00:29:09 (15 s cadence)
fresh 1 MW event: DISPATCH_EVENT_STATE_VALIDATED version 1 (149 HiGHS schedules)
manifest code_version d2b354f665e46c026cbfe14affebf87faa8aa0c2+modified
```

Three defects surfaced here and were fixed inside the wave: drifting source
time (2A.7), empty telemetry samples (2A.8), and a silent validation failure
on an empty plan (3C.5). The unbounded audit journal became 4A.9 and is now
landed as a partitioned telemetry table.

## Sweep (`.local/gate2.sh`)

```text
$ compose healthy
local-postgres-1 Up 3 hours (healthy)
local-temporal-1 Up 3 hours (healthy)
$ contracts lint + breaking + local generate
contracts ok
$ python suites
tools/generation: 16 passed in 2.44s
tools/data: 8 passed in 1.02s
services/decision: 71 passed in 3.80s
$ decision strict mypy
Success: no issues found in 23 source files
$ safety benchmark under 2 s
BenchmarkValidate5000-14        	       3	  34737778 ns/op
BenchmarkValidate5000x288-14    	       3	 272107000 ns/op
$ migrations twice + sqlc + seed
sqlc ok
50
$ hook installed and rejects a comment
tools/development/hooks
hook_reject_exit=1
$ suppression scan, files over 500 lines, comment lines, any: nothing
$ stub markers vs STUBS.md rows
markers: 2  rows: 2
$ prefixed subjects in last 120 commits / stashes
prefixed: 0  stashes: 0
```

Twenty-three Go packages passed in the sweep. Two packages failed there and
passed on re-run: `internal/replay` (another lane mid-edit) and
`tests/integration` (the ten-minute default timeout, since fixed). Migration
0005 failed its second apply in the sweep and was sent back to its author.

## UI leg

`pnpm exec playwright test demo-path` against the standing demo: steps 2, 3,
9, 13, and 16 pass; steps 10, 11, 12, 14, and 15 (fan-out, seeded failure,
retries, tracking, expiry) and 1, 4 to 8, and 17 fail with element not found.
The backend behind every step is exercised by the scenarios above; the
missing pieces are console screens on the UI track (U2 and U3). Gate 2's UI
condition (steps 2, 3, 9 to 16 green) therefore stays open on the console
track and closes when those screens land; it does not block Wave 3, whose
lanes touch nothing the console consumes.
