# Gate 5 report

**Run:** 2026-09-27 05:09Z to 07:30Z by the director on `main`, in a declared gate window with lane stacks, Bazel and load tests paused (light fixes by root allowed inside the window, each verified here).
**Result:** backend GREEN. Every Gate 5 condition below holds on the director's machine; the UI leg stays open on the console track.

## Gate 5 conditions

| Condition | Proof command | Result |
| --- | --- | --- |
| Load targets met and recorded | `grep -E "p95\|dropped" "claude docs/gate-reports/load.md"` | persisted=600000 dropped=0 sequence_gaps=0; read_p95=3.65 ms, sent_before_persisted=0 |
| Terraform validates | `terraform -chdir=infrastructure/aws init -backend=false && terraform -chdir=infrastructure/aws validate` (Terraform v1.16.4) | `Success! The configuration is valid.` |
| No secrets or long-lived keys | `grep -rn "private_key" infrastructure/aws`; `gitleaks detect --no-banner --redact` | 0 matches; `no leaks found` over 21.49 MB |
| Role matrix green | `go test ./services/control/internal/api/... -run RoleMatrix -count=1 -v` | TestRoleMatrixControl, ReadServices, TelemetryCredential, Context, Member all PASS |
| Scrubber green | `go test ./services/control/... -run Scrub -count=1 -v` | TestScrubbedSink, TestScrubExporters PASS |
| README verified by execution | 5F.3 clean clone at HEAD: `make plugins`, `make generate` (last line `sqlc generate`), `pnpm --dir apps/console install --frozen-lockfile` (`Done in 2.2s`), `go build ./services/control/cmd/control`; director checked every linked path and tool version; `make test-go` | ok; full suite exit 0, 38 packages ok, `tests/integration 2106.577s`, `tests/load 634.956s` |
| Runbooks verified by execution | 5F.1, director rerun 2026-09-27 04:47Z | six runbooks, every block executed on the demo |
| STUBS.md only PENDING-LIVE entries naming the live system | `grep -rn STUBBED services tools --include='*.go' --include='*.py' --include='*.ts'`; `grep -c STUBBED STUBS.md` | no matches; 0 (3a99409, condition amended 3e83521) |
| Bazel | `bazelisk test //services/... --nocache_test_results --test_env=GRIDOS_DATABASE_URL=... --test_output=errors`; `bazelisk test //apps/console:vitest --nocache_test_results`; `bazelisk build //services/control:image` | `Executed 31 out of 31 tests: 31 tests pass.`; `Executed 1 out of 1 test: 1 test passes.`; `Build completed successfully` (five host-only targets stay manual and run under `make test-go`) |
| Observability | `curl localhost:9464/metrics \| grep gridos_`; `curl localhost:33000/api/dashboards/uid/gridos-dispatch`; `curl -u <grafana admin> localhost:33000/api/v1/provisioning/alert-rules` | 38 gridos_ series; 200; four rules: Safety rejections, Fallback rate, Uncertain commands, Stale telemetry share |

## Required scenarios

Nineteen scenarios in five sequential groups on isolated stacks (`tests/integration`, `-count=1`, `GOFLAGS=-p=1`). Groups 1, 4 and 5 are from the first pass; groups 2 and 3 are the rerun after the next-command fault repair (0f7dbd8, c46d514), which root first proved with three consecutive isolated passes of each repaired scenario.

```text
group1: --- PASS: TestAuditChain (112.17s)
group1: --- PASS: TestHarness (102.13s)
group1: --- PASS: TestWorkerTermination (101.70s)
group1: --- PASS: TestGatewayRestart (101.87s)
group1: --- PASS: TestOutageReplay (101.72s)
group1: ok  	github.com/Hirom0112/Base-GridOS/tests/integration	520.049s
group4: --- PASS: TestZeroPercentReservePreservesHardwareFloor (111.32s)
group4: --- PASS: TestTravelFlexLifecycle (105.18s)
group4: --- PASS: TestWeatherStaleAlarmRaiseFloor (141.81s)
group4: ok  	github.com/Hirom0112/Base-GridOS/tests/integration	358.757s
group5: --- PASS: TestAnomalySignalLabel (112.27s)
group5: --- PASS: TestNegativeMarginNoFlexDispatch (101.89s)
group5: --- PASS: TestNoFeeMarketReward (101.51s)
group5: --- PASS: TestScenarioFitsLiveMeasurementWindow (0.00s)
group5: ok  	github.com/Hirom0112/Base-GridOS/tests/integration	316.181s
group2: --- PASS: TestUnderReservedExcluded (110.70s)
group2: --- PASS: TestHoustonTwentyPercentOffline (101.74s)
group2: --- PASS: TestLostAckStillExecuting (101.76s)
group2: --- PASS: TestOldExpiryNewerPending (101.64s)
group2: --- PASS: TestMeasurementGapUnknown (101.64s)
group2: ok  	github.com/Hirom0112/Base-GridOS/tests/integration	517.920s
group3: --- PASS: TestConsecutiveEventsUseNextDeviceGeneration (114.10s)
group3: --- PASS: TestHeatEventCanonical (109.68s)
group3: --- PASS: TestInfeasibleTargetShortfall (102.07s)
group3: --- PASS: TestOptimizerTimeoutFallback (101.69s)
group3: ok  	github.com/Hirom0112/Base-GridOS/tests/integration	428.105s
```

First pass failures, both fixed inside the gate: TestOldExpiryNewerPending (zero uncertain newer commands: the next-command fault skipped the end-of-event zero stops) and TestHeatEventCanonical (no UNCERTAIN exception: the next-command fault expired after one gateway tick).

## FULL_SPEC §10 acceptance list, item by item

| Acceptance item | Proof command | Result |
| --- | --- | --- |
| Ingest and normalize public market, load, weather, outage | `go test ./services/control/internal/context/` (in `make test-go`) | ok |
| Deterministic multi-thousand-device fleet, replayable | `uv run --project tools/generation pytest tools/generation` | 17 passed (Gate 4); replay `IDENTICAL` on the demo (5F.1) |
| Display fleet health, reserve, availability, context, event | console track | open on the UI track |
| Feasible rolling-horizon plan with per-device schedules | `TestHeatEventCanonical` | 109.68 s |
| Reject a plan breaching reserve, power, energy, freshness, window | `go test ./services/control/internal/safety/ -run Golden` (in `make test-go`) | ok |
| Explicit operator approval for the canonical event | `TestHeatEventCanonical`; console live proof `playwright test --config playwright.live.config.ts -g "live approval passes"` | 109.68 s; 1 passed (6.9s) |
| Resume an in-flight event after worker restart | `TestWorkerTermination`; worker restart runbook executed on the demo | 101.70 s; PID 36048 resumed |
| Repeated command delivery idempotent | `TestGatewayRestart`; emergency-stop runbook retry returned the same stop id | 101.87 s |
| Distinguish intent, acknowledgement, verified delivery | `TestHarness`, `TestMeasurementGapUnknown` | 102.13 s, 101.64 s |
| Inject offline devices, delayed telemetry, duplicates, optimizer timeout | `TestHoustonTwentyPercentOffline`, `TestLostAckStillExecuting`, `TestOptimizerTimeoutFallback` | 101.74 s, 101.76 s, 101.69 s |
| Report with provenance, input versions, plan version | report tests in `make test-go`; `GetEventReport` fixtures (3F.3) | ok |
| Visibly label every simulated or modeled result | value_kind and provenance on every response; console badge (U0.4) | backend ok, UI open |
| Only consented, effective-dated resilience and Travel Flex policies | `TestTravelFlexLifecycle` | 105.18 s |
| Travel Flex expires automatically; early return handled | `TestTravelFlexLifecycle` | 105.18 s |
| Severe weather, stale telemetry, alarms raise the floor | `TestWeatherStaleAlarmRaiseFloor` | 141.81 s |
| Decline dispatch when conservative margin is negative | `TestNegativeMarginNoFlexDispatch` | 101.89 s |
| Preserve pricing, reward, consent versions per decision | `TestNoFeeMarketReward` | 101.51 s |
| Away-mode alerts described as energy anomalies | `TestAnomalySignalLabel` | 112.27 s |
| Operator read p95 under 500 ms | `claude docs/gate-reports/load.md` | read_p95 3.65 ms |
| Command intent persisted before delivery | same | sent_before_persisted=0 |
| Safety validation under 2 s for an event cohort | Gate 3 bench `Validate5000x288` | 0.28 s |
| Planning under 10 s for the canonical 5,000-device scenario | Gate 3 `-k perf_5000` | 1.45 s |
| New telemetry in the live view within 5 s | telemetry lag on the demo after gateway restart | 11 s after one 15 s cadence, 9 s steady |
| No reserve violation in property tests or failure scenarios | `TestUnderReservedExcluded`, `TestZeroPercentReservePreservesHardwareFloor` | 110.70 s; 111.32 s |
| Same seed and versioned inputs, same outcome | replay runbook on the demo | `IDENTICAL` |

## UI leg

The console runs in local auth mode against the control service with the mock identity signer (Makefile 5d4cd49, 77c3bcf). Verified here: live step-up approval proof (1 passed), identity badge rename (18 vitest), demo-path spec with zero soft assertions. Steps still owed by the console track: context, forecast, member, live failure, recovery, reserve and expiry panels, and replay. Gate 5's UI condition stays open on the console track; 3F.5, 4F.5 and 5F.2 close together when the 17-step demo-path spec is green on the standing demo.

## Connectors and PII

`go test ./services/control/internal/connectors/... -count=1` printed `ok ... 1.230s`. The PII grep over `testdata` printed nothing.

## Full Go suite

`GRIDOS_DATABASE_URL=... make test-go` on the fixed tree: exit 0, 38 packages `ok`, none failed. The first run inside the gate found two red packages, both fixed by root with RED/GREEN and verified here before the rerun: the publisher deadline test assumed one publish batch would claim a later generation behind an in-flight predecessor (the 2B.9 deferral is intended, so the test now asserts the deferral and a second batch), and the contract canonical-JSON test compared bytes against unstable protojson whitespace (both documents are now compacted; the fixture is untouched).

## Defects found and fixed inside the gate

Four: the next-command gateway fault expired after one tick (0f7dbd8) and skipped zero-setpoint commands (c46d514), which hid the UNCERTAIN path in two scenarios; the publisher test assumption above (15cffbf); the contract test comparison above (8ac3bb9). Two host-side findings were fixed by lanes before the window: stale sqlc output with `make generate` not running sqlc (5D.7), and the demo console started without local auth mode or the control API URL (5d4cd49, 77c3bcf).

## Backend items opened by the console track during the gate

4D.8 region-correct context, 4C.6 plan evidence in the explanation, 4E.8 per-command truth, 4E.9 measured reserve compliance, 4B.9 member catalog read, 4C.7 historical H3 layer. They are root's queue after this window and do not gate Wave 5's backend conditions; the console labels each missing panel as missing evidence until they land.
