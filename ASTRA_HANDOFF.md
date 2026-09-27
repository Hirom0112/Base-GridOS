# GridOS handoff for Astra

## Current state

The build runs in one shared tree on `main`. The director coordinates through `.local/mailbox.log`; read its last 40 `director:` lines before each item and append progress there. `AGENTS.md`, `claude docs/BUILD_ORDER.md`, and `claude docs/QUESTIONS_AND_DECISIONS.md` govern the work. Do not rewrite the mailbox or restart the demo without the director's release.

The latest director instruction grants the worker ownership of a top-priority weather RPC fix in `services/control/internal/api/context/`. The integrated 17-step UI run stopped at step 1 before creating or launching an event: `GetWeatherContext` for Austin returned an empty upstream reply because `contextSource` panics on the valid `SIMULATED` provenance introduced by the demo weather scenario. The gateway had not been used for a launch. The director will rebuild and rearm it after the fix.

## Next action: weather RPC

1. Add a failing `GetWeatherContext` test using `testdata/scenarios/weather` that expects the Austin alert source to carry `DATA_PROVENANCE_SIMULATED`. Commit the RED test alone and record the exact failing command and last line in the mailbox.
2. In `services/control/internal/api/context/convert.go`, map `SIMULATED` explicitly. Change `contextSource` to return an error for an unsupported value and propagate it through the context API instead of panicking on data. Keep RED and GREEN in separate commits; run the focused context API test and the fast hook.
3. Post `worker: DONE` with both SHAs, command, and last output line. The director owns the demo rebuild and gateway rearm. The UI agent then reruns the integrated 17-step spec as the first launch.

The scenario public root is assembled by `make demo` from `testdata/fixtures/public` with `weather/` replaced by `testdata/scenarios/weather/`. `LoadPublic` needs price, load, outage, and weather files; the test should use a complete temporary root or the assembly check. The protobuf enum already defines `DATA_PROVENANCE_SIMULATED` in `contracts/gridos/v1/device.proto`.

## Remaining proof and fixture

The integrated run must complete before the 4E.10 report fixture can be recorded. After the director opens a short fixture window, record `ReportService/GetEventReport` from that completed event in `testdata/fixtures/api/ReportService/GetEventReport.json` and update its `INDEX.json` request ID. The existing RED test is commit `4643a56`; it requires both `planned_shortfall` and `delivery_shortfall`, including a positive-coverage `MEASURED` interval. Run `GOFLAGS=-p=1 go test ./tools/development/mockapi -run 'MeasuredReportShortfalls|Fixtures' -count=1` and commit the fixture GREEN. The old completed event `live-proof-1790502698` predates migration 0020, so its measured-delivered values are NULL and cannot prove a measured delivery shortfall. Existing fixture tests also assert the old report event ID and economics values; update those assertions only as required by the newly recorded response, and coordinate any UI fixture expectations with the UI agent.

The 4C.10 manifest fixture is already recorded and verified: commit `69d905e`, event `event-4c10-1790504164`, Travel Flex window `seed-4c10-1790504134-window`. That event was validated but never launched. The director's armed release for the failed run was on HEAD carrying `6b937e2`; the weather fix requires another rebuild before retry.

## Work already accepted

- 2A.13 persistent live faults and deterministic retry seed: `8c6d97e` / `d7038af` follow-up, three isolated canonical passes.
- 5F.5 demo weather assembly: `715078c` / `59d2c03`; the director verified two active WEATHER overrides at a 60% floor.
- 2B.11 per-command publish failure isolation: `1f0026a` / `94fa18d`; three isolated canonical passes, director verified.
- 4C.10 frozen manifest API and fixture: `ce546c4` / `628351b`, fixture `69d905e`; director verified.
- 4E.10 report API and approved-plan correction: `16ad5bf`, `7383f53`, `68aba6a`, then `97d05ab` / `6b937e2`. The report fixture is still open.
- The director's last recorded UI gate before the new shortfall UI work was green: 78 browser tests and 180 unit tests. UI requested a final recorded gate; the director owns that gate and the live demo.

## Shared-tree cautions

Commit exact owned paths only. Do not stage unrelated `go.work.sum`, untracked `control` or `gateway-simulator` binaries, `services/control/internal/storage/testdata/`, or Python egg-info. Do not run the full suite or `make demo` as a worker. The director runs wave gates and rebuilds. Preserve the gateway's first-launch ordering for the UI integrated run.
