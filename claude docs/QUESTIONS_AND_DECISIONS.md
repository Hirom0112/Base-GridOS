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
- **Reversed: frozen inputs as an activity result.** At 5,000 devices the
  FreezeInputs result is about 4 MB, above Temporal's payload limit, so
  workflows failed and events stayed REQUESTED (found by lane F at 2F.8).
  Correct design: FreezeInputs writes the input and eligibility snapshots
  to PostgreSQL through the existing snapshot tables and returns only their
  identifiers; RequestPlan and later activities load by identifier. Lane F
  owns `internal/dispatch/activities.go` and `internal/api` runtime for this
  fix since lane B is complete. The 5,000-device demo fleet is not reduced.
- **Early start of Wave 3 lanes A and B (2026-09-26):** forecasting (3A) and
  the HiGHS optimizer (3B) live in the decision service, whose Wave 2 items
  are verified, and touch nothing the open Wave 2 lanes edit. They start now
  on Hirom's new orchestrator terminal so it is not idle; lanes 3C to 3F
  wait for Gate 2. Gate 3 still requires Gate 2 to have closed first.
- **2F.5 (lane F), accepted:** `WatchEvent` polls durable command and
  reconciliation truth every 250 ms; sent MW excludes PERSISTED and appears
  only once a command state advances; acknowledged MW requires a durable
  ACCEPTED acknowledgement; fleet delivery uses the latest reconciliation
  audit; per-cell delivery reruns the per-device integration at read time;
  cells without measured telemetry carry a MISSING delivered state and
  uncertainty gaps, never a claimed zero.
- **Single orchestrator (2026-09-26, Hirom's order):** the session in
  Hirom's labeled terminal, thread `01a0dfbe`, is the sole orchestrator.
  The original headless session is stopped. The director no longer staffs
  lanes itself; the orchestrator runs three subagents plus its own lane.

- **3A.1, 3A.2 (lane 3A), verified:** the RED test fails with the missing
  `gridos.forecasting` module, which is the named reason for a new package;
  the similar-day baseline uses the latest complete same-class day and a
  90th-percentile absolute-residual radius as the calibrated interval, and
  carries training window, feature and model versions, issue time, horizon.
- **3B.2 scope (lane 3B), accepted:** `optimization/model.py` is a
  discharge-only LP over the event window: nonnegative service per device
  and interval, power bound net of home load, one-way discharge efficiency,
  whole-window energy bound above the effective reserve (which, with no
  charging, holds at every interval), a soft shortfall column priced at
  1000 per kW, and availability as expected delivery. The per-kW unit cost
  on service is the cycling term. Charging variables and charge/discharge
  mode exclusion are deferred: no interval carries a price or a pre-event
  horizon yet, so there is no concrete need (AGENTS.md "Before adding
  code" rule 1). Revisit when the rolling horizon spans priced hours.
- **3B.3 green on first run, accepted:** the soft-shortfall LP already
  yields a visible per-interval shortfall without touching any reserve; the
  test has a positive control (the shortfall is nonzero and equals target
  minus expected), so no artificial RED was manufactured.
- **3B.4 (lane 3B), verified:** cohorts group eligible devices by power,
  efficiency, home load, and availability; disaggregation is greedy by
  headroom above reserve and raises when the cohort plan cannot be met,
  never relaxing a reserve. Reconstruction sums back within tolerance.
- **UI request, geographic context (2026-09-26):** the console's sparse
  rectangular cell field is the real fixture: `_austin_cells` samples
  uniformly inside a lat/lon bounding box. That is a generation flaw, not a
  rendering one, so a new item 2D.7 gives the Austin fixture a
  density-weighted urban silhouette and regenerates the fleet and the
  recorded `ListSites` fixture. No terrain or basemap tiles will ever be
  served; 4C.3 (Wave 4) delivers a locally served MapLibre style plus small
  Texas boundary, weather-zone, and load-zone GeoJSON. U1.7 renders cells
  and boundary lines only; U4.1 waits for 4C.3. The UI agent is right not to
  fabricate locations or density to match a generated concept image.
- **2F.8 and 2B.7 closed (2026-09-26):** director re-ran `make test-e2e`
  (ok, 6.8 s) and rebuilt the standing demo with the worker; a fresh event
  moved REQUESTED to PLANNED through Temporal in four seconds. The Wave 1
  straight-line lifecycle and both retired STUBS rows are gone from the
  tree; `internal/api/dispatcher.go` now holds only the domain steps the
  activities call.
- **3A.3 to 3A.6, 3B.5 to 3B.8, 2D.7, 2F.3 verified** by re-running every
  reported selector on the director's machine (22 decision tests, mypy
  clean, 6 Austin generator tests, mockapi fixtures). Accepted
  assumptions: outage hazard is the county monthly rate over calendar-month
  hours times one plus the active alert count, labelled modeled_estimate;
  device reliability priors 99:1, 19:1, 9:1 with freshness decay over 300 s;
  objective weights are dimensionless planning weights (shortfall 1000,
  cycling and uncertainty unit) until intervals carry prices; the 3A.5
  GREEN commit loosened a binary-float equality in its own RED test to
  approx without changing any physical value; density for 2D.7 means sites
  per occupied cell falling across outward H3 bands (42.6, 16.1, 4.8).
- **3B.8 golden diff, reviewed:** eight baseline plans switch from
  fallback to the HiGHS result with empty fallback reason; four fixtures
  only normalize numeric zeros; timeout, invalid-vector, no-incumbent, and
  replacement fixtures are unchanged, so the fallback path stays live.
- **c30de15 accepted:** the worker-termination scenario drops its
  `WORKER_RESTART` injection because the integration harness itself
  restarts the worker process; the simulator has no such failure kind.
  The scenario's intent is unchanged.
- **Two control defects found by lane D, assigned to lane F as 2F.9 and
  2F.10:** the publisher sends only the first batch of 100 commands, and
  the workflow reports before the event window opens. The director's own
  integration runs (Harness and AuditChain stuck at SENT after 90 s)
  confirm the first. 2D.2 and 2D.6 stay open until they pass on the
  director's machine after 2F.9. Lifecycle timing is fixed with durable
  Temporal timers on the real event window; integration tests shorten the
  window to about a minute and raise the simulator cadence. No accelerated
  clock exists in the control service.
- **Early start of Wave 3 lanes C and E (2026-09-26):** lanes 3A and 3B are
  complete, freeing two orchestrator slots. 3C (safety differential and
  benchmark) and 3E (analytics sink) own paths disjoint from the open Wave 2
  work, so they start now. 3D and 3F wait for Gate 2 because they own
  `internal/dispatch/`, `tests/integration/`, and `tests/end-to-end/`.
- **Integration scenario retiming (lane D, 2D.2 follow-up), accepted:**
  the harness rewrites each scenario at run time: the event begins ten
  seconds after stack setup and lasts one minute, injections keep their
  fraction of the original window, and the simulator clock starts at the
  same wall-clock instant with a five-second interval at a five-second
  cadence. Telemetry `source_time` and event time therefore share one real
  clock; the control service and worker see no altered time. The checked-in
  scenario files keep their two-hour product windows. Commits f9d57f7 (RED)
  and 1118d77 (GREEN); director re-run ok in 0.4 s.
- **3C.1 rejected then accepted (lane 3C):** the first differential test
  (35c2ece) had no rejected fixture, so it could not catch a gate that
  approves everything. 7b37e30 adds a control per fixture: a dynamic
  reserve raised above stored energy must yield ENERGY_BELOW_RESERVE and a
  discharge one kW above the device maximum must yield DISCHARGE_BOUND.
  Director re-run: 36 subtests pass.
- **3C.2 (lane 3C), accepted:** the aggregate commitment check already
  existed in the gate (declared shortfall must cover target minus actual)
  and gains a test; the ramp check is new with a device
  `MaxRampKWPerMinute` where zero means no ramp limit beyond the power
  bounds, since FULL_SPEC §5.6 carries no ramp metadata.
- **3C.3 (lane 3C), accepted with a binding condition:** approval carries a
  SHA-256 digest of the proposed plan and the canonical state with the
  clock zeroed; revalidation with a different digest yields
  REAPPROVAL_REQUIRED. Because device telemetry times sit inside the
  canonical state, the digest is only stable over the frozen snapshot from
  FreezeInputs, never over live state. 3D.1 must compute and compare the
  digest against the frozen snapshot; any caller passing live state would
  demand re-approval on every telemetry tick.
- **2D.3 finding, scoped injections (2026-09-26):** the failure engine picks
  its target by hashing the seed, injection time, and kind across all 5,000
  devices, so a lost-ack or old-expiry fault usually misses the roughly 113
  scheduled devices. Decision: scenario injections gain an optional
  `scope: scheduled` field; with it the engine chooses deterministically
  among devices that have received a command for the active event (hash of
  seed and the sorted commanded device ids). No harness back door that
  sends commands. Lane D owns `services/gateway-simulator/internal/failures/`
  and `testdata/scenarios/SCHEMA.md` additively for this, lane A being
  complete.
- **3C.4 verified, lane 3C complete:** the full-day gate over 5,000 devices
  and 288 intervals runs in 0.28 s per validation on the director's
  machine, well under the 2 s ceiling.
- **Early start of 3F.1 (2026-09-26):** lane 3C's slot is free. 3F.1 (replay
  manifest, new package `internal/replay/`) touches nothing open, so it
  starts now. 3D waits: lane F is mid-edit in `internal/dispatch/` and
  `internal/reconciliation/` for 2F.10, and lane D holds
  `tests/integration/`. 3D.3 and 3D.4 may start once 2F.10 is verified.
- **3E.1 (lane 3E), verified:** local newline-JSON sink under
  `.local/analytics/` by default; the BigQuery sink is a plain HTTPS POST to
  the insertAll endpoint with the record id as insertId, Application
  Default Credentials through `golang.org/x/oauth2/google` (one go.mod
  line), a 10 s client timeout, and three bounded retries on 429 and 5xx.
  Row-level insert errors fail the write. Director re-run ok.
- **3F.1 (lane 3F), verified:** one JSON manifest per event under a local
  directory, written to a temp file, fsynced, and hard-linked into place so
  it can never be overwritten; a second create with identical content is
  idempotent and different content is an error. Fleet and scenario bytes
  are SHA-256 hashed at creation. RED c58c988, GREEN 5c6b038, director
  re-run ok.
- **3E.2 (lane 3E), verified:** seven append-only record kinds share one
  envelope whose provenance carries class, source id and URI, observed and
  ingested times, schema version, and an optional simulation seed; every
  field is validated at the sink boundary and id collisions are rejected.
  RED 29ac47c, GREEN 35eb238, director re-run ok.
- **3E.3 (lane 3E), verified:** behind the `bigquery` build tag, a recorded
  insertAll body fixture, an idempotent retry with the same insertId, and a
  row-error rejection all pass; the live smoke is skipped unless
  `GRIDOS_BIGQUERY_LIVE=1` with ADC and a table. Director re-run: three
  pass, one skip.
- **2F.9 and 2F.10 (lane F), verified; lane F complete:** the publisher
  drains batches until the event has no PERSISTED intent (progress-checked,
  Temporal retries cover a concurrent event filling a batch); zero commands
  at EndEvent are idempotent across worker retries and carry a valid
  delivery window. The workflow is versioned under `event-window`: durable
  timer to begin_time, EXECUTING, VerifyDelivery at every reporting
  interval end with emergency stop and replacement signals live during
  every wait, VERIFIED at end_time, EndEvent, a 30 s late-message grace,
  ReconcileLateMessages, ProduceReport, then expiry. Old histories keep the
  pre-window path. Director runs: dispatch unit tests ok; Harness,
  AuditChain, and WorkerTermination pass in about 100 s each on the
  director's machine, where all three previously stalled at SENT.
- **2D.2 and 2D.6 verified** on the same run. 2D.4 still needs
  gateway-restart and outage-replay; 2D.3 and 2D.5 remain with lane D.
- **3E.4 rejected then accepted:** the import isolation is now a Go test
  that shells out to `go list -deps` with a positive control, not a
  one-off command. Lane 3E complete.
- **Lane 3D opens in two agents (2026-09-26):** 3D-dispatch takes 3D.1,
  3D.2, and 3D.6 in `internal/dispatch/`; 3D-api takes 3D.3 and 3D.4 in
  `internal/api/`. Neither touches `tests/integration/` or
  `testdata/scenarios/` while lane D holds them; 3D.5 waits for lane D.
  The 3C.3 condition applies to 3D.1: the approval digest is computed over
  the frozen snapshot.
- **3D.3 (lane 3D-api), verified:** `GetPlanExplanation` is an additive
  RPC returning the stored plan's objective breakdown, constraint margins,
  exclusions, shortfalls, and the summed effective reserve held back; it
  is authorized for operator, approver, analyst, and service roles and
  refuses a plan version that does not match the event. Director:
  regenerated contracts, buf lint clean, control builds, test ok. One
  refactor requested as its own commit: `LoadPlan` becomes part of the
  store interface instead of a runtime type assertion.
- **3D.1 (lane 3D-dispatch), verified:** FreezeInputs reloads the persisted
  snapshot by id and carries its SHA-256; a retry whose snapshot differs
  fails; approval binds a digest of the frozen request and stored plan, and
  PersistIntents refuses a mismatched digest. Both digests are over
  persisted data, never live state (3C.3 condition met). Two RED and two
  GREEN commits; director re-run ok.
- **Forecast boundary for 3D.2 (2026-09-26):** the decision service gains
  one additive RPC, `Forecast`, on `OptimizationService` in
  `optimization.proto`: request carries the event window, sites, and the
  frozen telemetry window; response carries per-interval site load, regional
  price, outage risk, and per-device availability with feature and model
  versions and value_kind. Dispatch calls it with a budget, freezes the
  response bytes next to the input snapshot, and passes the forecast into
  `OptimizationRequest` through additive fields. Lane 3D-dispatch owns
  `optimization.proto` additively (already granted) and
  `services/decision/gridos/server.py` plus its tests additively, lane 3B
  being complete. A forecast timeout is a timeline decision and the
  deterministic baseline from 3A.6 is used.
- **3D.2 history source and persistence seam (2026-09-26):** no telemetry
  history travels over the wire and no new snapshot table is added. The
  `ForecastRequest` carries identifiers only: per site the
  `load_profile_type`, `load_zone`, `weather_zone`, and county, plus the
  frozen device telemetry already in the snapshot. The decision service
  reads the public fixtures it already owns under
  `testdata/fixtures/public/` (load profiles, ERCOT prices, system load,
  outages, weather) by those identifiers, with CONFIRMED_PUBLIC provenance;
  a missing source yields an unavailable forecast, never a synthesized
  number. The forecast is an optimizer input, so FreezeInputs calls
  `Forecast` with its budget before persisting the input snapshot and the
  forecast lands inside that snapshot; a timeout records a timeline decision
  and freezes the 3A.6 deterministic baseline instead. `lifecycle.go` is
  untouched.
- **3D.4 (lane 3D-api), verified; lane 3D-api complete:**
  `ValidateUnsafeAlternative` runs the independent safety gate over an
  operator-supplied alternative against the frozen input for a VALIDATED
  event and returns machine-readable violations with no event transition
  and no commands; the positive control is a safe alternative that
  approves. The store interface now requires `LoadPlan` (dfc42fa).
  Director: regenerated, buf lint clean, api package ok.
- **Early start of Wave 4 lane C (2026-09-26):** the slot freed by 3D-api
  takes lane 4C (geo aggregation, offline map assets, address scan), whose
  paths `internal/fleet/geo/`, `internal/api/geo/`, and
  `testdata/fixtures/geo/` touch nothing open and unblock the console's map
  work (U4.1). 4C.2 waits for 4D.3. Gate 3 still requires Gate 2 first.
  Lane 3F may record the `GetPlanExplanation` half of 3F.3 now and hold the
  forecast half for 3D.2.
- **3D.2 metadata carry-through:** `Site` in `device.proto` gains additive
  `load_profile_type`, `reliability_trait`, and optional `county`; the
  fleet loader keeps them so the frozen snapshot can hand identifiers to
  `Forecast`. County has no source yet, so outage risk is reported
  unavailable until lane 4C's Texas boundary data can assign counties.
- **3D.2 API boundary (lane 3D-api), accepted:** `Forecast` on
  `OptimizationService` takes the frozen `OptimizationRequest` (which now
  carries `ForecastSite` identifiers) and returns typed site load, regional
  price, outage risk, and device availability values, each with bounds,
  feature and model versions, value_kind, and provenance, plus an explicit
  `unavailable_sources` list. `Site` carries load profile type, reliability
  trait, and optional county. Director: regenerated, buf lint clean, api
  and fleet packages ok. 3D.2 marks when the dispatch activities and the
  decision server side pass `-run PlanningActivities`.
- **4C.3 (lane 4C), verified:** the control service serves a MapLibre style
  with only local GeoJSON sources under `/geo/`: a Texas outline from the
  public Census TIGERweb generalized state boundary (CONFIRMED_PUBLIC) and
  weather-zone and load-zone extents that are SIMULATED rectangles taken
  from the fleet generator, not operational boundaries. Assets total 516 KB.
  Director: geo tests ok; standing demo rebuilt so the route answers on
  28080.
- **3D.2 timeout semantics, accepted:** the decision server computes the
  deterministic baseline before the learned model and returns it on an
  internal model timeout; a transport timeout in dispatch freezes an
  explicit unavailable forecast, audits the decision, and the optimizer's
  deterministic fallback proceeds through validation. Go never fabricates a
  baseline it did not receive.
- **4C.1 resolutions (2026-09-26):** sites carry resolution-7 H3 cells
  only, and a finer cell cannot be derived from a coarser one, so
  aggregation covers resolutions 5 through 7 by walking to parents;
  resolution 8 is reported unavailable rather than fabricated. Lane 4C owns
  `services/control/go.mod` and `go.sum` additively for `h3-go/v4`, added
  in the same commit as its first use, no `go mod tidy`.
- **3D.2 (lane 3D-dispatch), verified:** FreezeInputs calls `Forecast`
  with its budget and freezes the typed response inside the input
  snapshot; a forecast transport timeout freezes an explicit unavailable
  forecast, audits FORECAST_TIMEOUT, and forces the deterministic fallback;
  an internal solver timeout returns the validated fallback with
  PLAN_FALLBACK_SELECTED; an Optimize transport timeout after bounded
  retries is a decision-service outage that fails the event visibly. The
  decision server computes the baseline before the learned model. Five
  RED commits precede the GREEN ones. Director: dispatch, fleet, api, and
  decision suites all ok (63 Python tests, mypy clean); 5,000-site forecast
  measured at 0.04 s and 618 KB.
- **4C.1 and 4C.4 (lane 4C), verified:** privacy-safe aggregation at
  resolutions 5 to 7 with a five-site merge threshold, SOC bands (low under
  30 percent, high at or above 70, unknown without telemetry), connectivity
  and active-dispatch counts; the address scan covers every served geo
  asset and the cell response with escaped-key positive controls and
  refuses unsafe assets at startup.
- **Standing demo findings (2026-09-26, director):** three defects surfaced
  while trying to record a VALIDATED event. 1) The simulator derives source
  time from the tick count, so a 5,000-device tick slower than its cadence
  drifts behind the clock without bound; the demo drifted 90 s in five
  minutes and every device was excluded as stale. Item 2A.7. Mitigation
  now: the demo gateway runs at a 15 s cadence (Makefile), which holds the
  lag near 20 s against the 30 s freshness limit. 2) The telemetry stream
  never uses the battery model: observations carry no state of energy and
  no power flow, so the planner sees zero energy, produces no schedules,
  and the gate rejects "device schedules required". Item 2A.8. The
  end-to-end suite passes because it pushes its own telemetry. 3) A day of
  five-second telemetry had grown `audit_journal` to 4.3 million rows and
  3.4 GB, so the latest-per-device load took four seconds. Item 4A.9. The
  director dropped and recreated the demo database and the gateway store;
  resetting the gateway store alone had made every new observation collide
  with old sequence numbers and be deduplicated away. Lesson recorded: the
  gateway's sequence store and the control database are reset together or
  not at all. 4) A non-retryable validation failure leaves the event in
  PLANNED with no operator-visible reason; an empty plan with full declared
  shortfall must validate as a quantified shortfall. Item 3C.5.
- **3D.2 forecast metadata follow-up, granted:** `ForecastValue` gains
  additive `training_window_begin`, `training_window_end`, `issued_at`,
  and `horizon`; realized error is produced by the 3A.6 evaluation after the
  event and recorded against the frozen forecast, not carried on the value.
- **2D.3, 2D.4, 2D.5 verified; lane D complete (2026-09-26):** on the
  director's machine every Wave 2 integration scenario passes: harness,
  audit chain, worker termination, gateway restart, Houston partial outage,
  lost acknowledgement, old-command expiry, network outage replay,
  measurement gap, and under-reserved exclusion, each in 100 to 110 s
  inside the one-minute live window. Two earlier failures (old-expiry and
  outage-replay stuck at REQUESTED) happened while a dozen commits from
  other lanes were landing in the shared tree and did not reproduce.
- **2A.8 (lane 2A), verified on the standing demo:** each simulated device
  now runs a battery model seeded from the fleet file with home load from
  its public load profile; observations carry state of energy and power
  flow; the gateway store writes each tick's sequences and buffer in one
  transaction; the scale test honors short mode. On a fresh database and
  gateway store, a fresh 1 MW event reached VALIDATED in two seconds with
  149 HiGHS device schedules at plan version 1. Assumption accepted: a
  10 percent hardware floor stands in for the missing protected operating
  floor, and initial energy derives deterministically from the seed.
- **Demo reset lesson, repeated:** resetting the gateway sequence store
  alone makes every new observation id collide with the control database's
  history and be deduplicated away; the two reset together or not at all.
- **Gate 2 sweep (director):** contracts lint, breaking, and generation
  clean; 23 Go packages ok; Python 95 tests green across three projects;
  decision mypy clean; safety benchmarks 0.03 s and 0.27 s; sqlc clean;
  seed loads 50 sites; hook installed and rejects a comment; no
  suppressions, no comments, no `any`, no file over 500 lines; two stub
  markers match two STUBS.md rows; no prefixed subjects, no stashes. Two
  findings: `tests/integration` now exceeds Go's default ten-minute package
  timeout when run whole (thirteen scenarios at about 100 s each), so the
  `test-go` target needs an explicit timeout for that package and the
  director runs the scenarios in groups; migration 0005_replacement.sql is
  not re-runnable (plain ADD COLUMN), unlike the earlier migrations, and
  goes back to its author.
- **Early start of Wave 4 lane E (2026-09-27):** the agent freed by lane
  3F takes 4E (event report, comparison, modeled economics) in
  `internal/report/`, which nothing open touches.
- **3D.6 (lane 3D-dispatch), verified:** a replacement loads current device
  state through the fleet snapshotter, calls the decision service's
  `Replace` RPC with the explicit dropped list and envelope, runs the
  independent safety gate on the replacement plan, persists a replacement
  snapshot and a new plan version that keeps the event's frozen input, and
  issues the command with the next generation; a rejected replacement stays
  a quantified shortfall. Director: dispatch, decision (71 tests), and
  contract checks ok.
- **3F.4 (lane 3F), verified:** `ReplayService.ReplayEvent` is its own
  service, authorized, returning manifest provenance, the ordered timeline,
  and the plan diff. Director: api/replay and cmd/replay ok.
- **3F.2 rejected pending two fixes:** the recorded manifest carries
  `code_version` "(devel)" (module version, not the commit) and an empty
  string for an absent scenario. The manifest must record the VCS revision
  from build settings (with an explicit `GRIDOS_CODE_VERSION` for builds
  without VCS metadata, and a hard error when neither exists) and omit the
  scenario fields when no scenario ran. The worker hook, seed rule, and
  end-to-end assertion are otherwise accepted.
- **2A.7 (lane 2A), verified on the standing demo:** live source time is
  the wall clock truncated to the cadence; a skipped slot yields one MISSING
  observation per device at the last skipped slot, then the current sample.
  On the rebuilt demo the lag held at 10 s across two measurements 45 s
  apart at a 15 s cadence, with telemetry landing in the new
  `telemetry_observations` table.
- **3F.2 (lane 3F), verified:** the worker records the VCS revision (with
  a modified marker when the tree is dirty) or an explicit
  `GRIDOS_CODE_VERSION`, and refuses to start without either; a manifest
  omits scenario fields when no scenario ran. A fresh demo event's manifest
  carries the full commit hash and no scenario keys.
- **3C.5 (root), verified:** the proto-to-safety conversion accepts an
  empty plan whose contiguous per-interval shortfalls fully quantify the
  target, so a fleet with no capacity validates as a quantified shortfall
  instead of spinning in PLANNED; reserve validation is unchanged.
- **4E.1 (lane 4E), verified:** the report builder computes requested,
  approved, commanded, acknowledged, and delivered energy, baseline and
  measurement, latency and tracking, availability and confidence, reserve
  prevention, exclusions, modeled economics marked modeled_estimate, data
  gaps, assumptions, provenance, and versions with finite-bounds checks.
  The live PostgreSQL source and immutability are 4E.2, with the
  `event_reports` design recorded in the mailbox.
- **Director practice, recorded:** when the shared tree does not build,
  the standing demo is rebuilt from `git archive HEAD` in the scratchpad
  with the generated contracts copied in; the demo database and gateway
  store are reset together only, and the worker is started with an
  explicit `GRIDOS_CODE_VERSION` when built outside VCS metadata.
- **4E.3 and 4E.5 (lane 4E), verified:** report comparison emits sorted
  changes for every numeric field, uncertainty bounds, exclusion counts,
  and plan, policy, solver, and model versions; the partner view exposes
  aggregate power, delivered energy, and modeled net value only, with a
  positive control proving private site, device, travel, and away strings
  planted in the source never reach the JSON. Director: report package ok,
  seven tests.
- **3D.5 (lane 3D), verified; lane 3D complete:** on the director's
  machine the canonical 5,000-device heat event passes in 111 s,
  infeasible-target-shortfall in 110 s, and optimizer-timeout-fallback in
  102 s, the last through the decision server's startup solver budget knob
  so HiGHS deterministically times out and the validated fallback is served
  with PLAN_FALLBACK_SELECTED. Two earlier failures came from an
  uncommitted partition migration in the shared tree, since replaced by a
  declarative migration.
- **4B.1 and 4B.2 (lane 4B), verified:** `economics/margin.py` models
  every FULL_SPEC §5.11 term as a validated nonnegative Decimal range and
  the conservative estimate takes the low end of each value and the high
  end of each cost; the RED failed on the missing module. Director: 8
  margin tests pass, mypy clean over 25 files.
- **4B.3 (lane 4B), verified:** additional flexibility is eligible only
  when the conservative margin strictly clears the configured hurdle; a
  margin at or below the hurdle, including a negative one, yields zero
  additional capacity. Director re-run: hurdle and formula tests pass.
- **4B.4 (lane 4B), verified:** the member reward is a closed union of
  fee waiver and fixed credit; a market with no membership fee yields a
  fixed credit and a no-fee market without a configured credit is an
  error at the edge. Director re-run: 11 economics tests pass.
- **4E.2 (lane 4E), verified:** `event_reports` stores the report bytes
  and SHA-256 once, in the same transaction as the RECONCILED to REPORTED
  transition and its audit row; a retry returns the stored version and the
  PostgreSQL report source reads that row without recomputation. Director:
  report and storage tests pass; migration 0007 applies twice cleanly.
- **4A.9 (lane 4A), verified:** observations now live in a partitioned
  `telemetry_observations` table with a latest-per-device index; a
  singleton hourly Temporal maintenance workflow creates today's and
  tomorrow's partitions and drops those older than seven days; an
  observation past the window is rejected with an audit row and a
  CodeInvalidArgument so the gateway does not retry it; every non-test
  reader of the old audit rows is gone; retention is relative to the
  store's injected clock. Director runs: storage, ingest, reconciliation
  selectors ok; IngestScale ok against the rebuilt demo; Houston, outage
  replay, and measurement gap scenarios pass (314 s).
- **4D.1 (lane 4D), verified:** offline loaders for ERCOT day-ahead and
  real-time prices, system load, derived county outage rates, and NWS
  forecasts and alerts stamp every record with its provenance class
  (CONFIRMED_PUBLIC from the file, DERIVED from the outage row itself),
  source as-of time, and age, and reject missing or future-dated sources.
  Director: context package tests pass.
- **4B.5 (lane 4B), verified:** the bill-and-value simulator takes
  normalized public prices and profiles plus explicit cost assumptions over
  a backtest window and returns member savings and company margin as
  SIMULATED values with source lineage; no measured savings are claimed.
  Director: test passes, mypy clean over 27 files.
- **4A.1 and 4A.2 (lane 4A), verified:** a plan selection applies only
  with consent text, version, effective time, and the explanation shown;
  names, bands, prices, and rewards come from an immutable versioned
  catalog row referenced by foreign key; the idempotency key rejects a
  replay with different consent or plan; an inactive catalog is refused.
  Migration 0008 applies twice and survives rollback and re-apply.
  Director: policy package tests pass.
- **4D.2 (lane 4D), verified:** candidate dispatch windows rank by
  equal-weight min-max normalized price, regional load, and outage risk
  with earlier start as the tie-break; gross grid value is price times
  feasible MW times hours, labelled modeled_estimate; non-finite inputs are
  rejected. Weights are a recorded assumption to revisit when a market
  contract exists. Director: context package tests pass.
- **4B.6 (lane 4B), verified:** `OptimizationRequest` gains additive
  per-device `base_reserve_kwh` and optional `travel_flex_reserve_kwh`
  plus request-level `conservative_margin` and `margin_hurdle`; the
  optimizer uses the reserve increment only when a verified flex window is
  present and the conservative margin clears the hurdle, otherwise it plans
  against the base reserve. Director: contracts lint and breaking clean,
  77 decision tests, mypy clean, control builds against the regenerated
  contracts. Control-side population of the fields is 4A.3.
- **4E.6 (lane 4E), verified at the source:** the PostgreSQL report source
  loads the exact frozen snapshot, sums unique site-interval load into the
  baseline, averages device availability, carries model versions with
  FROZEN_FORECAST provenance, reads delivered energy from durable
  verification, and lists baseline_confidence_unavailable,
  delivery_method_unavailable, and the economics gap explicitly. The demo
  curl in the item's verify line is not yet possible because `GetEvent`
  exposes only the basic report; that is new item 4E.7.
- **Forecast interval coverage (lane 4B), accepted:** `ForecastValue`
  gains optional `interval_coverage`; the load baseline emits 0.90 and
  other forecasts omit it. Director: contracts breaking-clean, decision
  suite green, control builds.
- **4C.2 hierarchy (2026-09-27):** the offline fleet has no utility,
  substation, or feeder, so the drill-down uses ERCOT as market, the site
  load zone, and deterministic synthetic utility, substation, and feeder
  ids derived from H3 and site metadata, every node labelled SIMULATED, no
  street or home coordinates. `GeoService` is additive in `api.proto` with
  one registration line.
- **4D.3 (lane 4D), verified:** `ContextService` serves filtered ERCOT
  market and system load, NWS forecasts and alerts, and derived county
  outage rates with a per-record source stamp, and ranks caller-supplied
  modeled candidate windows without joining non-overlapping histories.
  Director: contracts lint and breaking clean, context packages ok,
  control builds; the demo probe waits for lane 4A's missing migration.
- **Contract file rule (2026-09-27):** `api.proto` reached the 500-line
  ceiling, so every new service lives in its own proto file (`geo.proto`,
  `report.proto`) importing shared types.
- **Broken HEAD (2026-09-27):** commits 6a875d0 and 8e8ea33 reference a
  `member_sites` table that no tracked migration creates, so a fresh
  database cannot start the worker and the director could not rebuild the
  demo from HEAD. Lane 4A was ordered to commit 0010_member_sites before
  anything else. Rule recorded: a commit that references a table its
  migrations do not create is red even when the hook passes.
- **4E.7 (lane 4E), verified:** `ReportService.GetEventReport` returns
  the stored report for a REPORTED event and the live view otherwise,
  applies the partner redaction on request, and denies partner callers the
  full view and unauthorized roles any view; the transport test proves
  private device data appears only in the full view. It stays in
  `api.proto` (479 lines) because buf breaking forbids moving committed
  symbols between files. Director: report package ok, control builds; the
  demo curl follows the demo rebuild after migration 0010.
- **4A.3 (lane 4A), verified:** Travel Flex windows apply only between a
  consented local start and end, expire automatically, record a fixed
  credit, and an early return cancels the window with an audited
  idempotent override to the catalog maximum until the original end; the
  simulated fleet gets a durable one-member-per-site binding through
  migration 0010; the frozen request carries base and consented flex
  reserves while the safety state stays at base. Director: policy and api
  selectors pass, 0010 re-applies, the rebuilt demo seeds 5,000 bindings.
- **4B.7 (lane 4B), verified:** the decision service derives a
  conservative margin from frozen public price lower bounds only, caps a
  public-price-only positive margin at zero, and preserves the base
  reserve; unavailable terms are explicit. Director: decision suite green,
  mypy clean.
- **Flex reserve roundtrip (2026-09-27):** both lanes found that a
  positive margin can never consume flex under the current binding. New
  item 4B.8 makes the reserve selection a first-class value through the
  safety gate rather than relaxing the equality check.
- **4A.4 approved:** migration 0011 adds the COMMUNICATIONS override
  reason with rollback; the override is a typed idempotent command with a
  bounded interval and an audit row.
- **Demo rebuilt on the current tree:** ContextService and ReportService
  answer on 28080; a fresh event created twelve seconds after start
  validated as a fully quantified shortfall (all devices still stale at
  that instant), which is the 3C.5 behaviour working as intended, and the
  live report shows the stale exclusions, FROZEN_FORECAST provenance, and
  the economics version gap.
- **4C.2 (lane 4C), verified; lane 4C complete:** `GeoService` in its own
  `geo.proto` serves privacy-merged cells and a market to feeder
  drill-down over a synthetic hierarchy labelled SIMULATED at every node,
  with the address scan covering its responses. Director: contracts lint
  and breaking clean, geo packages ok, control builds.
- **4A.4 (lane 4A), verified:** typed risk overrides (weather, outage
  risk, health, stale telemetry, alarm, communications) need evidence and
  a higher floor, apply at their effective time, expire, survive plan
  changes, reject a changed idempotent payload, and write one audit row;
  migration 0011 adds the communications reason. Director: override tests
  pass, 0011 re-applies. The automatic caller is new item 4A.10.
