# Questions and decisions

Running log kept by the director. Every decision made without asking Hirom is
written here the moment it is made, with the reason. Questions that genuinely
need Hirom appear under **Open** and move down once answered. This file
replaces the `ASSUMPTIONS.md` the build order originally named; spec-gap
assumptions reported by workers land here too.

## Open

(none)

## 2026-09-26

- **Q:** Who orchestrates the build? **D:** The Codex session named worker
  orchestrates and spawns up to six subagents; Claude directs, verifies every
  item independently, and marks `[x]`. Channel: `codex queue` in, `.local/mailbox.log` out.
- **Q:** Branches and worktrees, or one tree? **D:** One shared tree, direct
  to `main`, commits by exact path. Lane ownership already keeps paths
  disjoint, so branches would only add a merge step. Hirom preferred simpler.
- **Q:** How much gating per commit? **D:** Only the fast pre-commit hook on
  staged files plus the one verify command for the item in hand. The full
  suite runs once per wave by the director. Hirom does not want the build
  bogged down in repeated gates.
- **Q:** Where do the console screens get built? **D:** In a separate UI
  track (`UI_TRACK.md`) run by Hirom's own agent, which owns `apps/console`.
  Backend lane F supplies fixtures, a mock server, ingest, streaming, and
  integration tests instead.
- **Q:** Do backend wave gates block on the UI track's Playwright spec?
  **D:** No. If the UI track has not reached that gate's steps yet, the gate
  report records "UI track pending" and the next backend wave starts. The
  Playwright results are attached when they arrive.
- **Q:** Wave 0 sequencing when the toolchain is missing? **D:** Lane A's
  item 0A.1 (install Go, buf, uv with Python 3.12, sqlc, temporal, bazelisk)
  runs first as a single agent for a few minutes. The other five lanes start
  as soon as it reports done. Lanes may write their RED tests before that but
  cannot verify without the tools.
- **Q:** Where do assumptions and decisions live? **D:** Here, in one file,
  instead of a root `ASSUMPTIONS.md`. Fewer root files. `STUBS.md` stays at
  the root because the global rules name it.
- **Q:** What happens to the 57 MB `presentations/` folder? **D:** Kept on
  disk, ignored by git. The research doc moved to `docs/design/visual-system.md`.
- **Q:** Commit subject style? **D:** Plain imperative, no `type(scope):`
  prefix, enforced by the commit-msg hook. Matches the umath reference and
  the global rule that an unenforced convention is drift.
- **Q:** Measurement boundary for the canonical event? **D:** Meter net
  export by default, battery terminal selectable, because FULL_SPEC §15
  leaves it open and the worked physics example uses net export.
- **Q:** Critical-load definition for backup duration? **D:** Always report
  hours at current usage and hours at a 750 W reference load. Settles the
  FULL_SPEC §15 question for the first build.
- **Q:** Bazel now or later? **D:** Wave 5. Native `go test`, `pytest`, and
  `pnpm` verify everything until the tree is real.
- **Q:** Local auth for the offline demo? **D:** `GRIDOS_AUTH_MODE=local`
  with a dev identity, marked STUBBED, because the judged path may not call
  Clerk over the network.

### Reported by workers during Wave 0

- **0C.1 (FULL_SPEC §15 gaps, all accepted):** canonical events measure at
  `METER_NET_EXPORT` with `BATTERY_TERMINAL` selectable; planning and
  reporting use five-minute intervals; reports keep signed error, absolute
  error, completeness, and uncertainty instead of a binary tolerance; backup
  duration is reported at current usage and at 750 W; command expiry enforces
  a zero grid-service setpoint; expired commands are never revived after a
  reconnect; cancellation is a new command with a new ID and a higher
  generation.
- **Pre-commit hook edit by lane A:** the hook now prepends the Go bin
  directory to PATH so `golangci-lint` installed with `go install` is found.
  Accepted: it adds tool discovery and removes no check. Any hook edit that
  removes or loosens a check would be rejected.
- **RED commits versus the green hook (raised by lane C on 0C.2):** a commit
  that stages only test files is a RED commit; the hook still formats, lints,
  and checks comments and size but skips test execution, vet, and typed lint.
  Any commit that stages implementation runs the tests for its packages and
  must be green. Implementation can never land with a failing test, and RED
  commits stay honest because they contain nothing but the test.
- **Python projects under `tools/`:** one `tools/pyproject.toml` with a
  `uv.lock`, owned by lane C in Wave 0, holding the dependencies of both
  `tools/generation` and `tools/data` (polars, pydantic, hypothesis, pytest,
  pyyaml, pyarrow, h3). Lane E asks lane C for additions. The hook runs
  pytest through the nearest `pyproject.toml` above each test file.
- **AGENTS.md wording:** "RED and GREEN are separate commits" now also says
  a RED commit contains only test files.
- **0D.1 (lane D):** migrations are golang-migrate-compatible ordered SQL
  files, applied with psql in Wave 0 and by a Go entry point from Wave 1.
  Accepted; it is what the item already said.
- **Command state storage (lane D, 0D.2):** `command_states` is an
  append-only transition log with a trigger blocking UPDATE and DELETE; the
  current state of a command is its latest row. Accepted: it gives every
  transition an audit row for free. Wave 1 lane D implements the conditional
  transition as an insert guarded by the latest row, not an UPDATE.
  `dispatch_events` stays a mutable row with conditional updates.
- **sqlc annotations versus zero comments (lane D, 0D.5):** the hook now
  exempts lines of the exact form `-- name: Identifier :verb` because sqlc
  reads them as directives; any other SQL comment is still rejected. Same
  category as `//go:` and `#!`.
- **0C.3 fleet generator (lane C), all accepted for simulated fixtures:**
  reserve bands RESILIENT 60/70/80 %, BALANCED 30/40/50 %, GRID_FLEX
  0/10/20 % (FULL_SPEC §15 leaves exact bands to the market catalog);
  weather-zone geography uses conservative rectangular interiors of the
  eight ERCOT zones until a licensed polygon source is chosen; H3 resolution
  7 for fleet cells; automatic backup on two of every three devices with no
  source distribution claimed.
- **Python projects, revised:** lane C created `tools/generation/pyproject.toml`
  with its own lock before the shared-project note arrived. One project per
  Python package is cleaner than a shared one, so that stands: lane E creates
  `tools/data/pyproject.toml`, and `services/decision` gets its own in 1C.1.
  Item 0A.5's `test-py` runs pytest through each project it finds.
- **0C.4 (lane C):** simulated batteries are limited to a one-hour C-rate,
  so charge and discharge power in kW never exceed usable energy in kWh.
  Accepted as a plausibility bound for fixtures.
- **Rollback files (lane D, unrequested by any item):** `database/rollbacks/`
  with one file per migration. Accepted, not scope invention: `AGENTS.md`
  lists migrations with a rollback path under what simplicity never cuts.
  Verified: forward, rollback to zero tables, forward again, all clean.
- **0C.7 canonical scenario (lane C), accepted:** the 20 MW event runs two
  hours, 18:00 to 20:00 Central, in `LZ_HOUSTON` as the one selected region;
  devices go offline 15 minutes after start and the gateway delay begins 5
  minutes after that; the synthetic evening is 2026-08-12 and claims no
  historical event.
- **Missing API services (found by lane F at 0F.1):** no build-order item
  defined the Connect services; 0B.2 to 0B.6 defined messages only. Opened
  0B.8 for lane B: `api.proto` with `FleetService`, `DispatchService`, and
  `TelemetryService` for Wave 1. Later waves add `EventsService`,
  `ReportService`, `GeoService`, `MemberService`, and `ContextService` in
  their own lanes, as additive changes the breaking check allows.
- **Generated Go location (found by lane F at 0F.3):** Go `internal`
  packages are only importable inside their parent tree, so
  `services/control/internal/gen` could never serve the gateway simulator,
  the mock server, or cross-service tests. Generated Go now lives in its own
  module `contracts/gen/go` with a committed `go.mod`; generated files stay
  ignored and `make generate` produces them. Opened 0B.9. The root `go.work`
  from 0A.5 wires it in.
- **Contract round-trip test split:** the Go leg stays in `tests/contract`
  (0F.3); the Python leg moves into the decision service's project (1C.1);
  the TypeScript leg moves to the UI track (U0.9). Each reads the same fixture.
- **Implementation limits reported as assumptions (accepted, no spec
  bearing):** outage normalization writes Parquet in 10,000-event batches
  (0E.4); public-data downloads use three attempts, a 20-second timeout, and
  exponential delay (0E.8); the local mock API caps requests at 1 MiB with
  5-second header, 10-second read and write, and 30-second idle timeouts
  (0F.1); fixture recording uses a 10-second timeout and rejects responses
  over 1 MiB (0F.4); the gateway simulator's HTTP server uses 5-second
  header, 10-second read and write, 30-second idle, and 5-second shutdown
  timeouts (1A.7); the control service defaults to a 30-second telemetry
  freshness threshold and the same header, write, idle, and shutdown
  timeouts (1E.4); the publisher integration harness allows 10 seconds for
  the gateway to start and a 2-second acknowledgement deadline (1D.9); control
  startup allows 5 seconds to reach PostgreSQL (1E.4); the real-process
  lifecycle test allows 10 seconds per process to listen, probing every
  25 ms with a 100 ms dial timeout (1E.7).
- **Remote Buf plugins rate-limited at Gate 0:** the Buf Schema Registry
  returned `resource_exhausted` after the day's regenerations. Gate 0 closes
  on a retried generate; Wave 1 item 1F.6 moves every plugin to a local
  binary installed by `make plugins`, so generation never depends on
  buf.build being reachable or generous.
- **Numerical tolerance (lane C, 1C.7 and 1C.9):** property checks and the
  independent plan validator use an absolute tolerance of 1e-9 kWh at the
  reserve and energy-balance boundaries; the Go safety gate (1B.2) uses the
  same 1e-9 for energy balance and declared shortfall. FULL_SPEC calls for explicit
  tolerances without naming values. Accepted.
- **Missing optimization RPC (found by lane C at 1C.8):** the contract had
  the request and plan messages but no service, and the request carried no
  device state or time budget. Opened 1C.10; lane C owns
  `optimization.proto` additively for Wave 1. Pattern for later waves: the
  lane that consumes a contract may add to it, additively, with the breaking
  check as the gate, rather than routing through a contracts lane that no
  longer exists.
- **UI track issues 1 to 10 (`claude docs/ISSUES.md`), all accepted:**
  approval and launch are separate actions with a `LaunchEvent` RPC and an
  `EventLaunch` record but no new lifecycle state; per-H3 capacity moves to
  Wave 2 and `WatchEvent` reports per-aggregate values; the demo fleet is a
  Greater Austin `austin-5000` in `LZ_AEN` (1F.7) replacing the Houston
  choice from 0C.7, with `texas-5000` kept for the fleet-wide scenario; one
  shell-owned renderer with one clock; MapLibre stays for `/map` with the
  Living Grid parked; one replay clock fed by `ReplayEvent`; all 17 demo
  steps must pass without WebGL. The lane that consumes a contract may add to
  it additively (lane E owns `api.proto` this wave).
- **Greater Austin box (lane F, 1F.7):** latitude 29.95 to 30.90, longitude
  -98.30 to -97.00 as a conservative Travis, Williamson, and Hays box until a
  licensed polygon source is chosen. Accepted for simulated fixtures. The
  generated fleet lands in 320 cells at resolution 7 with 42.6 MW nameplate.
- **1C.8 (lane C):** the Wave 1 fallback plans battery setpoints with a 0 kW
  site load because `DeviceState` carries no load forecast yet; the requested
  measurement boundary is retained. Accepted for Wave 1; Wave 3 forecasting
  supplies the load. Also: generated Python gets `.pyi` stubs from the
  protoc `pyi` builtin so strict mypy can see the contract types without
  suppressions.
- **Binaries were not wired (found by lane F at 1F.3):** each Wave 1 piece
  was built and tested in isolation but `cmd/control` mounted only the API
  over an empty site list, and the decision server had no entrypoint. Opened
  1E.7 (mount ingest, dispatcher, publisher, report; load the fleet) and
  1C.11 (runnable decision server). `make demo` starts the console only if
  the UI track has created it; the vertical-slice test is API-driven.
- **Phase 1 runtime limits (lane E, 1E.7), accepted:** 5-second decision
  budget, 100-command publisher batches, 5-second leases and acknowledgement
  deadlines, 10-second internal RPC client timeout; `make decision` defaults
  to port 50061 (1C.11). All caller-configurable; none is a spec value.
- **API lifecycle must be reachable (found by lane F at 1F.3):** wiring a
  dispatcher that no handler calls is not a control plane. Phase 1 plans and
  validates synchronously inside `CreateEventRequest`, approval moves
  VALIDATED to APPROVED, and launch persists and publishes. Temporal takes
  this over in Wave 2 without changing the states.
- **Demo stack (lane F, 1F.3), accepted:** `make demo` binds control to
  28080, gateway to 28081, and decision to 25061 so nothing collides with
  the mock API on 8080; each service gets up to 30 seconds to listen; the
  console starts only if `apps/console/package.json` exists.
- **Gate 1 finding:** `make test-go` runs the end-to-end tests, which need
  the demo stack, so it fails when nothing is running. Opened 2F.7: those
  tests skip cleanly without the stack, `test-go` excludes them, and
  `make test-e2e` runs them with the stack up.
- **Wave gates run the real binaries.** From Gate 1 on, a wave gate is not
  green until the director has run the demo stack and the end-to-end tests
  against the real processes, not only the unit suites. Twice in Wave 1 the
  integration gap only appeared when lane F tried to run the whole thing.
- **AGENTS.md edit by the worker (7b3a8c6):** adds the "Director mailbox"
  section requiring every subagent to read the last 20 `director:` lines
  before each item. It codifies an instruction I had already given and
  loosens nothing. Accepted. Any AGENTS.md edit that weakens a rule would be
  reverted by the director.

### Reported by workers during Wave 2

- **2A.1 (lane A):** a device-scoped injection with no explicit scope selects
  exactly one seeded device until the scenario names a scope. Accepted.
- **2A.3 (lane A):** "twenty percent of devices" rounds up to at least one
  device so a nonempty region never yields an empty outage. **2B.3 (lane B):**
  workflow activities use a one-minute start-to-close timeout until
  production latency budgets exist. Both accepted.
- **Staffing (found at Wave 2 start):** the Codex runtime allows three
  subagents plus the orchestrator, so it can run at most four lanes. To keep
  six lanes moving as Hirom asked, the director staffs the remaining lanes
  with its own subagents under the same AGENTS.md, hooks, ownership, and
  mailbox rules. In Wave 2: Codex runs A, B, F; the director's agents run C,
  D, E. Lane ownership keeps them disjoint; the mailbox is shared.
- **Standing demo is sacred.** One `make demo` stays up on 28080/25061/28081
  for the UI track. Test targets and director gate runs use their own
  directory, ports, and database (`gridos_e2e`), and never run `make down`
  (which removes the PostgreSQL volume). 2F.7 was rejected for violating
  this after it wiped the standing stack and its database.
- **2A.5 (lane A):** the scenario determinism proof lives in
  `cmd/scenario` rather than the simulator's `tests/` directory. Accepted;
  the proof is the point, not the path.
- **2C.2 (lane C), accepted:** the uncertain window runs from the
  acknowledgement deadline to the possibly accepted expiry; bounds are the
  hull of the possibly accepted setpoint, fresh telemetry, and the last
  confirmed setpoint (plus zero if it expires inside the window); ramp reach
  from fresh telemetry clips the last confirmed and zero candidates but never
  the possibly accepted setpoint; a zero max ramp means unknown and clips
  nothing. Conservative in the direction TECHSTACK requires.
- **2D.1 scenario mappings (lane D), accepted:** the Houston scenario uses
  `texas-5000` in `LZ_HOUSTON` because the Austin fleet has no Houston
  devices; worker termination is a harness action, not a simulator
  injection; network outage is expressed as `DELAYED_TELEMETRY` and the
  measurement gap as `DROPPED_MESSAGES` because the schema has no dedicated
  kinds; shortfall is disallowed for worker termination, gateway restart, and
  the outage replay because those faults must not lose delivery.
- **2C.3 (lane C), accepted:** telemetry proves execution when an
  observation at or after the possibly accepted effective time is within
  0.05 kW of its setpoint; telemetry never proves rejection (only durable
  receiver state does); a possibly operating device counts at its interval
  upper bound and an expired one at zero.
- **2C.4 (lane C), accepted:** delivered energy uses sample-and-hold
  integration; a span longer than the caller's `MaxGap`, the span before the
  first observation, and the span after the last are unknown gaps that add
  nothing to delivered energy or measured time. The GREEN commit corrected
  one expected value in the RED test (the last five-minute hold), disclosed.
- **2F.1 (lane F), accepted:** the ten-minute scale window at a five-second
  cadence emits 120 samples per device from start through start plus
  9m55s; the 20-minute test timeout is the only resource ceiling because no
  memory limit is specified.
- **2D.1 amended (lane D), accepted:** the eight failure scenarios target
  0.5 MW (about eighty devices through the greedy fallback, one publisher
  batch) because they prove failure handling rather than scale; the canonical
  heat event keeps 20 MW for the Wave 3 solver.
- **2C.5 (lane C), accepted:** observations are keyed by device and
  observation time; a second observation at the same time is a duplicate
  and the first value is kept even if the values differ.
- **2E.1 (lane E), accepted:** solver work runs in a forkserver child (fork
  is unsafe under the threaded gRPC server); the deterministic fallback is
  computed in-process before the isolated solver runs, so a slow or unhealthy
  solver can never become an error; the solver slot defaults to the
  deterministic planner until 3B supplies HiGHS.
- **2E.2 (lane E), accepted:** the service holds no incumbent plan (stateless
  per request); a solver that raises, exits, or cannot be shipped to the
  child yields `fallback_reason` SOLVER_FAILED with the failure logged.
- **2C.6 (lane C), accepted:** the reporting interval is five minutes; the
  property draws nonnegative grid-service power so the bound against a hold
  integral is strict; the setpoint in force at any instant is the most
  recently issued command whose effective time has passed and whose expiry
  has not; response latency is the longest time any command took to be seen
  within 0.05 kW of its setpoint.
- **2E.3 (lane E), accepted:** the validator additionally rejects a
  grid-service vector above max(0, discharge minus home load) or below zero
  (EXPORT_BOUND) and any schedule for a stale, unavailable, or
  maintenance-locked device, using the same family names as the Go gate.
- **2E.4 (lane E), accepted, with a follow-up:** replacement logic lives in
  `gridos/fallback/replacement.py` but the contract has no RPC or fields to
  carry the dropped list and approved plan. Opened 3D.6 to add a `Replace`
  RPC and route the workflow's `IssueReplacement` through it.
- **2D.2 (lane D), accepted:** the harness never touches `make demo`; each
  run creates `gridos_integration_<nanos>`, migrates it, starts decision,
  gateway with `--scenario`, control, and worker on ephemeral ports, drops
  the database and stops the processes in cleanup, and skips when Docker,
  PostgreSQL, or Temporal is unreachable; the event window is the scenario
  duration shifted to real time because the gate rejects plans in the past.
- **2B.6 (lane B), accepted:** the recovery test allows 25 seconds for
  Temporal sticky-queue reassignment after a worker kill and holds VERIFIED
  for three seconds so the durable state is observable.
- **2E.5 (lane E), accepted:** setpoints stay physical; a device counts
  availability probability times setpoint toward the target; shortfall is
  target minus expected; `feasible_kw` carries the expected value, never the
  nameplate sum; probability zero is an UNAVAILABLE exclusion.
- **2C.7 (lane C), accepted:** delivery is measured at `METER_NET_EXPORT` as
  minus `from_grid_kw` and at `BATTERY_TERMINAL` as `from_storage_kw`;
  `IMPORT_REDUCTION_VS_BASELINE` is rejected until a baseline exists; only
  PRESENT observations with a power flow count; the window ends at
  min(event end, now); baseline method recorded as MEASURED_AT_BOUNDARY.
- **2E.6 (lane E), accepted:** the four new golden fixtures are rendered by
  the real decision path; the eight existing fixtures each gained exactly
  one `fallback_reason` line, disclosed and consistent.
- **Hook false positive fixed by the director:** a Python continuation line
  beginning with `* ` was read as a block-comment line. The star rule now
  applies only to Go, TypeScript, JavaScript, and proto; comment rules for
  `#`, `//`, `/*`, and `--` are unchanged. Tested: formatted Python
  continuation accepted, Go block comment rejected, SQL comment rejected.
- **Routed to lane B with 2B.7:** register `reconciliation.Activities` in
  `cmd/worker` and delete the STUBBED `VerifyDelivery` and
  `ReconcileLateMessages` (workflow resolves activities by name); surface
  `report.Delivered` through `pgreport.go` and additive report proto fields;
  fix `insertZeroCommand`, which writes command rows under device ID
  "event" and so appears to reconciliation as an unmeasured pseudo-device.
- **2F.2 (lane F):** bulk telemetry write with `CopyFrom` in one transaction,
  receipt only after commit; 5,000 devices stay under 5 seconds fresh.
- **Audit chain hole (found by lane D at 2D.6):** the launch transition's
  audit row records the launch record but no state key, so the event chain
  breaks at APPROVED to COMMANDS_PERSISTED. Routed to lane B with 2B.7:
  every EVENT_STATE_TRANSITIONED row carries previous and new state.
- **Integration defects found by lane D (2D.3, 2D.4, 2D.5):** (1) the
  publisher stops a batch at the first `DeadlineExceeded` even though the
  command was correctly marked UNCERTAIN, so the workflow retries and
  republishes; it must treat a recorded UNCERTAIN outcome as handled and
  continue (lane B, with 2B.7). (2) A late ACCEPTED acknowledgement for an
  UNCERTAIN command was rejected because the per-command state machine had
  no UNCERTAIN to ACKNOWLEDGED row; the truth model now has it (durable
  receiver state proving acceptance) and storage follows (lane B). (3) The
  simulator never applied scenario injections at runtime and exited on a
  failed publish; opened 2A.6 for lane A. The measurement-gap and Houston
  outcomes wait on 2C.7's wiring, already in 2B.7.
- **Terminal kill during Wave 2 (2026-09-26):** Hirom's terminals were
  closed, taking the Codex TUI, the director's lane agents, and the mailbox
  watcher. The Codex app-server daemon kept the worker thread alive (it
  accepts queued messages), the compose stack and standing demo survived,
  and every commit is intact; lanes B and F had uncommitted edits on disk,
  which stay theirs. The director restaffed lane D from its surviving
  uncommitted work. Recovery rule: sessions live in the daemon; reattach with
  `codex resume <thread id>` rather than starting new sessions.
- **2A.6 (lane A):** scenario injections now act at runtime on exactly the
  seeded devices, failed publishes buffer to SQLite and replay once, and
  `--cadence` speeds integration runs; lane A complete again.

