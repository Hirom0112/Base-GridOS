# Gate 0 report

**Run:** 2026-09-26 by the director on `main` after all six lanes reported.
**Result:** GREEN pending one item (0B.6, TypeScript message-type generation).
**Verified items:** 38 of 39 in Wave 0. Every item was independently
re-run by the director before being marked `[x]`.

## Checks and evidence

Output of `.local/gate0.sh`, run once on the shared tree:

```text

$ compose up and healthy
local-postgres-1 Up 17 minutes (healthy)
local-temporal-1 Up 17 minutes (healthy)

$ buf lint + generate + breaking
contracts ok

$ make test-go
ok  	github.com/Hirom0112/Base-GridOS/tests/contract	(cached)
ok  	github.com/Hirom0112/Base-GridOS/tools/development/mockapi	0.436s

$ make test-py
============================== 8 passed in 0.76s ===============================
============================== 8 passed in 1.21s ===============================

$ migrations twice + sqlc
applied twice
sqlc ok

$ seed
50

$ mock api serves every Wave 1 method
FleetService/GetFleetSummary 200
FleetService/ListSites 200
DispatchService/GetEvent 200
DispatchService/CreateEventRequest 200
DispatchService/ApproveEvent 403

$ hook installed and rejects a comment
tools/development/hooks
hook_reject_exit=1

$ suppression scan (expect nothing)
AGENTS.md
CLAUDE.md
tools/development/hooks/pre-commit
done

$ files over 500 lines (expect nothing)
done

$ comment scan (expect nothing)
done

$ stub markers vs STUBS.md rows
markers: 1  rows: 1

$ prefixed subjects in last 80 commits / stashes
prefixed: 0  stashes: 0

$ TypeScript generated files
apps/console/src/api/gen/gridos/v1/api_connect.ts
apps/console/src/api/gen/gridos/v1/dispatch_connect.ts
```

## Readings

- Compose stack healthy; PostgreSQL 16 and Temporal both `(healthy)`.
- Contracts lint, generate, and pass the breaking check against `main`.
- Go workspace tests green (contract round-trip, mock API fixtures and recorder).
- Python tests green in both projects (8 generation, 8 data).
- Migrations apply twice without error; sqlc generate and vet pass; seed loads 50 sites.
- Mock API answers every Wave 1 method; `ApproveEvent` returns 403 with no identity, which is the approver-role check working.
- Hook installed and still rejects a comment line.
- No suppressions, no file over 500 lines, no comment lines in code. The three suppression-scan hits are `AGENTS.md`, its `CLAUDE.md` symlink, and the hook itself naming the forbidden markers.
- One `STUBBED` marker in the tree, one row in `STUBS.md`.
- No prefixed commit subjects in the last 80 commits, no stashes.
- **Pending:** TypeScript generation emits only `*_connect.ts`; the `bufbuild/es` plugin swap (0B.6) is outstanding. Re-run of the last check closes the gate.
- **UI track:** not started; the 17-step Playwright spec is not yet available. Recorded as "UI track pending" per the decisions log.

## Corrections made during the wave

Nine plan defects surfaced and were fixed in flight, each logged in
`QUESTIONS_AND_DECISIONS.md`: RED commits versus the green hook, per-package
Python projects, missing API service contracts (0B.8), generated Go in an
`internal` path (0B.9), the contract test split across runtimes, the root
test targets (0A.5), the sqlc directive exemption, the module resolution
under `go.work`, and lane F's fixture-directory ownership.
