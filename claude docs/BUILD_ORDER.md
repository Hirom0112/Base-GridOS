# GridOS build order

**Status:** execution contract for the director and six worker agents
**Written:** 2026-09-26
**Sources:** [`FULL_SPEC.md`](../FULL_SPEC.md) (product and safety authority),
[`TECHSTACK.md`](../TECHSTACK.md) (architecture, layout, implementation
sequence), [`DATASETS.md`](../DATASETS.md) (what data exists),
[`docs/domain/system-understanding.md`](../docs/domain/system-understanding.md)
(physics and worked numbers).

This document is the whole plan and the whole TODO in one place. Nothing in it
invents scope: every item traces to a section of the three root specs or to
`docs/domain/system-understanding.md`, which the specs name as the physics
reference. If an item and a spec disagree, the spec wins and the item is wrong.
Any numeric threshold in an item that no spec states (a parameter range, a
file-size cap, a resolution) is an assumption: the worker reports it in the
mailbox and the director logs it in `ASSUMPTIONS.md`.

---

## 0. How to read and run this document

### The shape

The build is six **waves**. Each wave has six **lanes** (A through F), one
lane per worker agent, so six agents are busy for the entire wave. A lane owns
a set of packages for the wave and no other agent touches those packages while
the wave runs (global rule: one agent per package per slice).

Each lane is a sequence of numbered items. Item IDs are `<wave><lane>.<n>`,
for example `1C.4` is wave 1, lane C, fourth item.

### The UI track

The operator console (`apps/console/`) is not built by these six lanes. It is
built by a separate UI agent following `UI_TRACK.md`, which owns that
directory completely. Lane F of each wave instead supplies what the UI agent
builds against: recorded API fixtures, a mock Connect server, the real demo
stack, streaming endpoints, and the integration tests. The director runs the
UI track's Playwright acceptance spec at every gate.

### Parallelism marks

- `[P]` fully parallel. The item depends on nothing outside its own lane. Start
  it the moment the lane is free.
- `[after X]` the item needs item `X` from another lane to be **merged to
  main** first. Until then the lane works on its other `[P]` items or writes
  the RED tests for this item (tests can always be written early).
- Every wave opens with all six lanes on `[P]` items so no agent is idle at
  wave start. Cross-lane waits are placed late in each lane on purpose.

### Gates

A wave ends at a **gate**. The gate is a list of commands the director runs on
main after merging every lane. A lane's items can be marked verified before the
gate, but the next wave does not start until the gate is green. If a gate
fails, the director opens follow-up items in the same wave and reassigns.

### Status marks

- `[ ]` open
- `[~]` dispatched to an agent
- `[x]` verified complete. Only the director writes `[x]`, only after running
  the item's verify command and seeing it pass.

### TDD scope

Items in STRICT scope (engines, validators, math, state machines, dedup,
storage transitions, generators) are written as `RED:` / `GREEN:` pairs. The
RED test is committed first on the lane branch, with its failure output pasted
into the mailbox report, then the GREEN implementation is its own commit. Main
only ever receives green merges, but the lane branch history must show red
then green or the director rejects the merge.

Items in ADAPTED scope (UI, external clients, infrastructure, docs) are
test-first in spirit: the Playwright spec, the recorded fixture, or the
`terraform validate` run is the acceptance test and is written before the
feature.

### Dispatch protocol for the orchestrator

1. The director (Claude) sends one wave's lane assignment to the orchestrator
   (the Codex session named worker). The orchestrator spawns six subagents, one
   per lane, each in its own git worktree on branch `lane/<wave><lane>` cut
   from main.
2. A subagent works its lane top to bottom. It never edits files outside its
   lane's `Owns:` list. Ownership is per wave: a directory one lane owned in
   Wave 1 may belong to a different lane in Wave 2. Dependency manifests
   (`services/control/go.mod`, `services/decision/pyproject.toml`,
   `apps/console/package.json`) have one owning lane per wave, named in that
   lane's `Owns:`; the owning lane's first item pre-declares every dependency
   the wave's other lanes will need so nobody waits. `go.sum`, `uv.lock`, and
   `pnpm-lock.yaml` conflicts are resolved by the director at merge with
   `go mod tidy`, `uv lock`, or `pnpm install`. Generated code
   (`buf generate` output) is never committed; every lane runs
   `make generate` locally.
3. When an item passes its verify command, the subagent appends one line to
   the mailbox in this form:
   `worker: DONE <item id> | <branch> <short sha> | <verify command> | <last line of output>`
   When blocked: `worker: BLOCKED <item id> | <reason> | <what it needs>`
4. The director independently runs the verify command on the branch, reads the
   diff, checks the red-then-green history, then marks `[x]` here.
5. At wave end the director merges lanes A through F into main in lane order,
   runs the gate, and posts the gate result to
   `claude docs/gate-reports/wave-<n>.md`.
6. `ASSUMPTIONS.md`, `STUBS.md`, and `BLOCKED.md` at repo root are written
   only by the director. A worker reports an assumption, a stub marker, or a
   blocker in its mailbox line (`worker: ASSUMPTION <item> | <text>`,
   `worker: STUB <item> | <marker> | <path>`, `worker: BLOCKED ...`) and the
   director records it at merge. Each assumption names the spec section it
   interprets.

### Standing items (every wave)

- `[ ]` Director updates `ASSUMPTIONS.md` at every merge that reported one.
- `[ ]` Director keeps `STUBS.md` equal to every `STUBBED` or `PENDING-LIVE` marker in the tree.
  Verify: `grep -rn "STUBBED\|PENDING-LIVE" --include=*.go --include=*.py --include=*.ts --include=*.tsx . | wc -l` equals the count of entries in `STUBS.md`.
- `[ ]` No `any` in TypeScript, no `Any` in Python, no `interface{}` used as an
  escape hatch in Go outside generated code.
  Verify: `grep -rn ": any\b\|as any\b" apps --include=*.ts --include=*.tsx | grep -v generated` prints nothing; `grep -rn "typing.Any\|: Any\b" services/decision | grep -v _pb2` prints nothing.
- `[ ]` No real PII in fixtures. Verify: `grep -rEn "[0-9]{3}-[0-9]{2}-[0-9]{4}|@gmail\.com|@yahoo\.com" testdata` prints nothing, and no fixture record carries a street address field.
- `[ ]` Every simulated fixture and UI view carries `provenance: SIMULATED`
  (FULL_SPEC §2).

---

## 1. Dependency map

```text
Wave 0  toolchain | contracts | truth model + fleet gen | database | data ingestion | UI contract + mock API
            \          |            |                       |            |               |
Wave 1  gateway sim | safety gate | decision fallback | storage+outbox | fleet+API | ingest + end to end
            \          |            |                       |            |               |
Wave 2  failure lab | Temporal wf | reconciliation | scenarios+integration | decision hardening | scale + live stream
            \          |            |                       |            |               |
Wave 3  forecasting | HiGHS optimizer | Go differential+perf | planning integration | BigQuery sink | replay + fixtures
            \          |            |                       |            |               |
Wave 4  member policy+DB | margin evaluator | map data (Go) | public context API | report+economics | flex scenarios + fixtures
            \          |            |                       |            |               |
Wave 5  connectors+contract tests | load tests | observability | Terraform/ECS | security+privacy | docs+runbooks

UI track (separate agent, UI_TRACK.md): apps/console across every wave, fed by lane F.
```

Hard cross-lane dependencies (everything else is `[P]`):

| Needs | Provided by | Why |
| --- | --- | --- |
| Generated Go/Python/TS types | `0B` contracts | Every service and the console import them |
| Fleet fixture files | `0C` fleet generator | Gateway, decision, storage seeds, console all load the same fleet |
| Migrations | `0D` database | Storage tests, API, workflows persist to these tables |
| Compose stack | `0A` toolchain | Integration tests need Postgres and Temporal running |
| Golden plan fixtures | `1C` decision | Go safety differential tests read them |
| Storage interfaces | `1D` storage | API, workflows, reconciliation call them |
| Gateway gRPC server | `1A` gateway | Outbox publisher and integration tests talk to it |

---

## 2. Wave 0 — foundation and truth model

Implements FULL_SPEC §12 Phase 0 and TECHSTACK implementation sequence step 1.
Every lane starts `[P]`.

### Lane 0A — toolchain, build graph, local stack

Owns: repo root files (`Makefile`, `AGENTS.md`, `.gitignore`),
`infrastructure/local/`, `tools/development/`.

- `[ ]` 0A.1 `[P]` Install the missing toolchain: Go 1.23+, `buf`, `bazelisk`
  (used in Wave 5), `temporal` CLI, `uv` with Python 3.12, `sqlc`. Record
  exact versions in `AGENTS.md`. Verify: `go version && buf --version && bazelisk version && temporal --version && uv python list | grep 3.12 && sqlc version` all print.
- `[ ]` 0A.2 `[P]` Write `AGENTS.md` at repo root: the short implementation
  rules from TECHSTACK "Navigation and ownership rules" and FULL_SPEC §14 data
  rules, the one-command local start, and the per-language test commands.
  Under 80 lines. Verify: `wc -l AGENTS.md` under 80 and every command it lists runs.
- `[ ]` 0A.3 `[P]` `infrastructure/local/compose.yaml`: PostgreSQL 16 with a
  disposable volume, Temporal dev server (`temporalio/auto-setup` or the
  `temporal server start-dev` image), and healthchecks for both. No BigQuery,
  no cloud credentials (TECHSTACK "Local and deployed topology").
  Verify: `docker compose -f infrastructure/local/compose.yaml up -d --wait && docker compose -f infrastructure/local/compose.yaml ps` shows both healthy.
- `[ ]` 0A.4 `[P]` Root `Makefile` with targets `up`, `down`, `generate`
  (runs `buf generate contracts`), `test-go`, `test-py`, `test-web`,
  `test-all`, `hooks`, `ui-mock` (runs 0F.1's mock server). No `demo`
  target yet; it arrives with the first runnable slice in 1F.3 (TECHSTACK:
  no placeholders). Add the ignored local
  working directory `.gridos/` (normalized data, quarantine, analytics sink)
  and every `buf generate` output directory to `.gitignore`.
  Verify: `make up` and `make down` succeed; `make -n test-all` lists the three test targets; `git check-ignore .gridos/x` prints the path.
- `[ ]` 0A.5 `[P]` Pre-commit and pre-push hooks (`tools/development/hooks/`)
  running `buf lint`, `gofmt`, `ruff`, `eslint`, and the standing `any`/PII
  greps from §0. Installed by `make hooks`. Never bypassed (this is the
  gate the global rules treat as the reviewer).
  Verify: a commit containing `: any` in a `.ts` file is rejected by the hook.

### Lane 0B — contracts

Owns: `contracts/`, `buf.yaml`, `buf.gen.yaml`.

- `[ ]` 0B.1 `[P]` `contracts/buf.yaml` with the `gridos.v1` module, `STANDARD`
  lint, and `FILE` breaking rules. `buf.gen.yaml` generating `connect-go` into
  `services/control/internal/gen`, Python `protobuf` + `grpcio` into
  `services/decision/gridos/gen`, and `connect-es` into
  `apps/console/src/api/gen`. All three output directories are gitignored
  (0A.4) and produced by `make generate`; no lane commits them.
  Verify: `buf lint contracts` passes on the empty module.
- `[ ]` 0B.2 `[P]` `contracts/gridos/v1/device.proto`: `Site`, `Device`,
  `Provenance` (`provenance` enum with the five FULL_SPEC §2 values,
  `source_id`, `source_uri`, `observed_at`, `ingested_at`, `schema_version`,
  optional `simulation_seed`), battery parameters with explicit units in field
  names (`usable_energy_kwh`, `max_charge_kw`, `max_discharge_kw`,
  `charge_efficiency`, `discharge_efficiency`), state of health, temperature,
  alarms, firmware, `last_seen_at`, `has_solar`, `has_automatic_backup`,
  gateway connection state, weather zone, load zone, H3 cell (never a street
  address, never coordinates). `Cohort` with `name`, `algorithm_name`, and
  `load_zone` so a partition is addressable the way market partitions are.
  Verify: `buf lint contracts` passes.
- `[ ]` 0B.3 `[P]` `telemetry.proto`: `TelemetryObservation` with
  `source_time`, `receive_time`, `observation_time`, units, sign convention
  enum (AC side, discharge positive), `sequence`, quality flags, measurement
  boundary enum (`BATTERY_TERMINAL`, `METER_NET_EXPORT`,
  `IMPORT_REDUCTION_VS_BASELINE`), and a `ValueState` enum with `MISSING`,
  `STALE`, `UNKNOWN`, `ZERO` as distinct states (TECHSTACK "Service
  contracts"). The observation body is a `PowerFlow` (`from_grid_kw`,
  `from_storage_kw`, `from_solar_kw`, `non_solar_to_home_kw`, `to_home_kw`),
  `state_of_energy_percent`, grid voltage, and an operating-state one-of:
  `OnGrid`, `OffGridOutage`, `OffGridNoHomePower`, `OffGridOvercurrent`
  (with `overcurrent_limit_kw`), `OffGridOvercurrentStandby`, or
  `TelemetryUnavailable`, each carrying `observed_at`. Every state except
  `TelemetryUnavailable` also carries
  `estimated_backup_hours_at_current_usage` and
  `estimated_backup_hours_at_750_watts` (the reference critical load, see
  0C.1). Verify: `buf lint contracts` passes.
- `[ ]` 0B.4 `[P]` `dispatch.proto`: `EventRequest`, `DispatchEvent` with the
  eleven-state lifecycle enum from TECHSTACK "Temporal workflows",
  `EligibilitySnapshot` with exclusion reason enum, `ReservePolicy`,
  `CommandIntent` (immutable `command_id`, `idempotency_key`, `device_id`,
  `event_id`, `plan_version`, `generation`, `setpoint_kw`, `issued_at`,
  `effective_at`, `expires_at`, `policy_version`, `correlation_id`),
  `CommandAcknowledgement` (an acceptance of receipt, never a result),
  `EmergencyStop`, and the member-facing `GridEvent` (`begin_time`,
  `end_time`, `event_type`) that explains why a battery did or did not
  participate. Verify: `buf lint contracts` passes.
- `[ ]` 0B.5 `[P]` `optimization.proto`: `OptimizationRequest`, `DispatchPlan`
  (objective breakdown, constraint margins, exclusions with reasons, fallback
  flag, solver and model versions), `DeviceSchedule`, `ShortfallReport` per
  interval. `verification.proto`: `DeliveryVerification`,
  `UncertaintyInterval` (signed feasible-power bounds with the derivation
  inputs listed in TECHSTACK "Safety and delivery semantics"), event report
  message with every field from FULL_SPEC §5.9. Verify: `buf lint contracts` passes.
- `[ ]` 0B.6 `[P]` `member_policy.proto` and `pricing.proto`: `ResiliencePlan`
  (effective-dated, consent text and version, market, reserve floor),
  `TravelFlexWindow` (start, end, timezone, temporary reserve, early-return
  action, credit type enum fixed daily / event / annual), `ReserveOverride`
  (reason enum: weather, outage risk, health, stale telemetry, alarm, early
  return), `PricingCatalogSnapshot` built from a `MemberPlan` that splits into an
  `EnergyPlan` (supply rate one-of: fixed, indexed with discount, subscription,
  or time-of-use windows; delivery one-of: passthrough, percentage credit,
  included; solar buyback one-of: fixed, realtime adder, match supply;
  `monthly_charge_cents`, `term_months`, early termination fee, renewable
  percentage) and a `BatteryPlan` (`BatteryEquipment` with `hardware_sku`,
  `description`, `energy_rate_offset_cents_per_kwh`; `monthly_payment_cents`,
  `term_months`; optional `PlanAddOn` with `rate_offset_cents` and
  `monthly_fee_cents`), plus the separate flexibility reward field, so no
  market inherits another market's fee structure. `FlexibilityOffer`,
  `MarginEstimate` with every term of the FULL_SPEC §5.11 formula,
  `HomeActivityAlert` whose description field is fixed text "energy anomaly
  signal, not a verified intrusion". Then run generation.
  Verify: `buf generate contracts` writes Go, Python, and TS output and `buf lint contracts` passes.
- `[ ]` 0B.7 `[P]` Breaking-change check against `main` using
  `buf breaking contracts --against '.git#branch=main'`, documented in
  `contracts/README.md` and added to the `Makefile` `generate` target by the
  director at merge. Verify: the command exits 0 on the lane branch and exits 1 when a field number is changed in a scratch copy.
### Lane 0C — truth model, fleet generator, scenario format

Owns: `docs/domain/truth-model.md`, `tools/generation/`, `testdata/fleets/`,
`testdata/scenarios/` (format only this wave).

- `[ ]` 0C.1 `[P]` `docs/domain/truth-model.md`: event measurement boundary
  (default `METER_NET_EXPORT` for the canonical event, `BATTERY_TERMINAL`
  selectable), interval `dt` = 5 minutes, sign convention, energy update
  equation and reserve inequality from `system-understanding.md` "The physical
  model", effective reserve rule `max(hardware_floor, plan_floor,
  dynamic_override)` (TECHSTACK "Safety and delivery semantics"), command
  semantics (idempotency, generation, expiry), the site operating-state
  machine (on grid, off-grid outage, off-grid no home power, off-grid
  overcurrent, overcurrent standby, telemetry unavailable) with the rule that
  an islanded site has zero grid-service capacity, the backup-duration
  convention (always report hours at current usage and hours at a 750 W
  reference critical load, which resolves the FULL_SPEC §15 critical-load
  question), and the event and per-command state machines as explicit
  transition tables. Each open decision from FULL_SPEC §15 that this doc
  resolves is reported as an assumption for the director to log.
  Verify: every state named in TECHSTACK "Temporal workflows" appears in the transition table; at least six `worker: ASSUMPTION 0C.1` lines reach the mailbox.
- `[ ]` 0C.2 `[P]` RED: `tools/generation/tests/test_fleet_determinism.py`
  pins that `generate_fleet(seed=20260926, size=5000)` produces a SHA-256 the
  test hard-codes, that two runs are byte-identical, and that `size=50` is a
  prefix-stable subset. Verify: `uv run pytest tools/generation -k determinism` fails with "module not found".
- `[ ]` 0C.3 `[P]` GREEN: `tools/generation/fleet/generate.py` producing
  `testdata/fleets/<name>.jsonl` with stable pseudonymous IDs, physically
  plausible battery parameters (usable energy 10 to 40 kWh, power 5 to 12 kW,
  one-way efficiencies 0.92 to 0.97), ERCOT weather-zone assignment across the
  eight zones, load-profile type assignment from the ERCOT residential profile
  classes in DATASETS §3, `has_solar` and `has_automatic_backup` flags, load
  zone, cohort membership, reliability traits, reserve preferences, and H3
  cells generated inside the zone polygon. No street addresses, no names
  (FULL_SPEC §8). Every record has `provenance: SIMULATED` and
  `simulation_seed`. Verify: `uv run pytest tools/generation` passes.
- `[ ]` 0C.4 `[P]` RED: `test_fleet_bounds.py` with Hypothesis: for any seed,
  every device satisfies the parameter ranges, reserve preference is within
  the plan bands, and no two devices share an ID. Verify: fails on the first
  property before GREEN tightens the generator.
- `[ ]` 0C.5 `[P]` GREEN: generator satisfies 0C.4. Produce the checked-in
  fleets `texas-5000.jsonl` and `texas-50.jsonl` (the 50-device fleet is the
  unit-test fleet). Verify: `uv run pytest tools/generation` passes and `wc -l testdata/fleets/texas-5000.jsonl` is 5000.
- `[ ]` 0C.6 `[P]` Scenario format: `testdata/scenarios/SCHEMA.md` and a
  pydantic model in `tools/generation/scenario/model.py` for a seeded clock,
  fleet reference, event definition (region, window, target MW, boundary),
  timed injections (the FULL_SPEC §5.8 list as an enum), and expected
  outcomes. RED test loads an invalid scenario and expects a validation error;
  GREEN writes the model. Verify: `uv run pytest tools/generation -k scenario` passes.
- `[ ]` 0C.7 `[P]` The canonical scenario `testdata/scenarios/heat-event-canonical.yaml`:
  5,000-site Texas fleet, severe-weather evening, one region, one event window,
  target MW, the injections named in FULL_SPEC §9 steps 11 and 12. Validates
  against 0C.6. Verify: `uv run python -m tools.generation.scenario validate testdata/scenarios/heat-event-canonical.yaml` prints OK.

### Lane 0D — database schema and queries

Owns: `database/`, `sqlc.yaml`.

- `[ ]` 0D.1 `[P]` Migration tool choice reported as an assumption
  (`golang-migrate` SQL files, applied by a Go entry point in Wave 1; this
  wave they are applied with `psql`). `database/migrations/0001_events.sql`:
  `dispatch_requests`, `dispatch_events` (state column constrained to the
  eleven states), `plan_versions` (immutable rows), `input_snapshots`,
  `eligibility_snapshots`. Verify: `make up && for f in database/migrations/*.sql; do psql "$GRIDOS_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"; done` succeeds twice (idempotent).
- `[ ]` 0D.2 `[P]` `0002_commands.sql`: `command_intents` (all fields from
  0B.4, unique `command_id`, unique `idempotency_key`), `command_outbox`
  (state, attempts, next_attempt_at, published_at), `command_states` with a
  CHECK on the per-command state list from `truth-model.md`,
  `command_acknowledgements`, `uncertainty_intervals`. Verify: same as 0D.1.
- `[ ]` 0D.3 `[P]` `0003_policy.sql`: `reserve_policies` (versioned),
  `resilience_plans` (effective-dated, consent text + version),
  `travel_flex_windows`, `reserve_overrides`, `pricing_catalog_snapshots`
  (energy plan and battery plan as separate JSONB documents matching 0B.6,
  each with its own term and monthly charge), `plan_add_ons`,
  `flexibility_offers`, `reward_ledger` (append-only). Verify: same as 0D.1.
- `[ ]` 0D.4 `[P]` `0004_audit.sql`: `operator_approvals`, `emergency_stops`,
  `verification_summaries`, `audit_journal` (append-only, trigger blocks
  UPDATE and DELETE), correlation ID columns on every table above.
  Verify: an `UPDATE audit_journal` in psql is rejected by the trigger.
- `[ ]` 0D.5 `[P]` `database/queries/*.sql` for sqlc: conditional event state
  transition (`UPDATE ... WHERE state = $expected RETURNING`), insert intent
  plus outbox in one statement set, claim outbox batch with `FOR UPDATE SKIP
  LOCKED`, upsert acknowledgement, append audit. `sqlc.yaml` targeting
  `services/control/internal/storage/gen`. Verify: `sqlc generate && sqlc vet` succeed.
- `[ ]` 0D.6 `[P]` `database/seeds/dev.sql` loading `testdata/fleets/texas-50.jsonl`
  into a `sites` reference table (structure from 0B.2) for local
  development only. Verify: after seeding, `psql -c "select count(*) from sites"` returns 50.

### Lane 0E — public-data ingestion and fixtures

Owns: `tools/data/`, `testdata/fixtures/` (except `contracts/`), `docs/data/`
additions.

- `[ ]` 0E.1 `[P]` `tools/data/manifest.py` and `tools/data/MANIFEST.json`:
  every file in DATASETS §1 to §9 with source URL, provenance, SHA-256, size,
  date range. Provenance values are mapped onto the FULL_SPEC §2 enum
  (`CONFIRMED_ORGANIZER_SANDBOX` becomes `CONFIRMED_SANDBOX`; anything
  labelled `INFERRED_NOT_VERIFIED` in the discovery log is refused, not
  ingested). RED: a test that checks each listed file exists in `data/` or
  `testdata/fixtures` and matches its checksum; GREEN: manifest generation.
  Verify: `uv run pytest tools/data -k manifest` passes against the current cache.
- `[ ]` 0E.2 `[P]` RED: `test_ercot_prices.py` pins that normalizing
  `dam-spp-2025.csv` yields rows with `provenance=CONFIRMED_PUBLIC`,
  `unit=USD_per_MWh`, UTC and local timestamps, and that the DST repeated-hour
  flag produces two distinct UTC rows for the fall-back hour. GREEN:
  `tools/data/normalize/ercot_prices.py` for DAM and RTM into Parquet under
  `.gridos/normalized/` (ignored; `data/` stays a raw research cache per
  FULL_SPEC §14). Verify: `uv run pytest tools/data -k ercot` passes.
- `[ ]` 0E.3 `[P]` RED then GREEN: `ercot_load_profiles.py` turning the
  103-column backcast rows into long-format 15-minute kWh with profile type,
  weather zone, and DST-aware UTC; rejects rows whose interval count is not
  96 or 100. Verify: `uv run pytest tools/data -k load_profiles` passes.
- `[ ]` 0E.4 `[P]` RED then GREEN: `outages.py` normalizing
  `texas_outage_event_data.csv` (754,216 events) with county, utility,
  customers out, duration, and computing a per-county monthly outage rate table.
  Verify: `uv run pytest tools/data -k outages` passes and the rate table has one row per county-month.
- `[ ]` 0E.5 `[P]` RED then GREEN: `nws.py` parsing the eight NWS GeoJSON
  snapshots into forecast periods and alerts with `issued_at`, `valid_from`,
  `valid_to`, and `value_kind=forecast`. Verify: `uv run pytest tools/data -k nws` passes.
- `[ ]` 0E.6 `[P]` Quarantine path: any record failing schema, range,
  sequence, or freshness validation is written to `.gridos/quarantine/` with
  the reason (FULL_SPEC §14). RED: a deliberately corrupt row lands in quarantine
  with reason `RANGE`. Verify: `uv run pytest tools/data -k quarantine` passes.
- `[ ]` 0E.7 `[P]` Cut small fixtures for tests and the offline demo into
  `testdata/fixtures/public/`: one week of DAM and RTM for `LZ_HOUSTON` and
  `HB_HOUSTON`, one week of all 32 residential profile types (four classes
  across eight weather zones, DATASETS §3), one month of Travis and Harris
  county outage rates, the eight Texas NWS files (forecast and alerts for four
  cities, DATASETS §5), and the ERCOT system load merged file. Each under 500 KB, each with a `PROVENANCE.md`
  sidecar. Verify: `du -sh testdata/fixtures` under 5 MB; `find testdata/fixtures -name PROVENANCE.md | wc -l` equals the number of fixture directories.
- `[ ]` 0E.8 `[P]` `tools/data/fetch.py` re-downloading every manifest entry
  from its source URL with checksum verification, so the cache is reproducible
  (FULL_SPEC §14). ADAPTED scope: recorded-response test plus one live smoke
  fetch of the smallest file. Verify: `uv run python -m tools.data.fetch --only ercot-system-load --verify` succeeds.

### Lane 0F — UI contract, mock API, contract tests

The console itself is built by the UI track (`UI_TRACK.md`), not by a backend
lane. This lane gives the UI track something to build against from day one.

Owns: `tools/development/mockapi/`, `testdata/fixtures/api/`, `tests/contract/`.

- `[ ]` 0F.1 `[after 0B.6]` `tools/development/mockapi`: a Go Connect server
  that serves every operator and member method in `gridos.v1` from JSON
  fixture files at `testdata/fixtures/api/<Service>/<Method>.json`, with
  `GRIDOS_AUTH_MODE=local` identities, the six roles, and the
  `site_location` permission, so the console runs with no backend.
  Verify: `go run ./tools/development/mockapi & curl -s -X POST -H 'content-type: application/json' localhost:8080/gridos.v1.FleetService/GetFleetSummary -d '{}'` returns the fixture.
- `[ ]` 0F.2 `[after 0C.5]` Hand-authored fixtures for the Wave 1 methods
  (`GetFleetSummary`, `ListSites`, `CreateEventRequest`, `GetEvent`,
  `ApproveEvent`) derived from the `texas-50` fleet, every aggregate carrying
  timestamp, provenance mix, and freshness, every record `SIMULATED`, plus
  `testdata/fixtures/api/INDEX.json` mapping each UI screen to its methods.
  RED: a test validates every fixture against its generated proto type and
  every method in `INDEX.json` against the proto descriptors.
  Verify: `go test ./tools/development/mockapi/ -run Fixtures` passes.
- `[ ]` 0F.3 `[after 0B.6]` `tests/contract/`: a Go test, a Python test, and
  a Vitest test that each round-trip the same `CommandIntent` JSON fixture
  (`testdata/fixtures/contracts/command_intent.json`) through their generated
  types and produce byte-identical canonical JSON.
  Verify: `go test ./tests/contract/... && uv run pytest tests/contract && pnpm --dir apps/console vitest run tests/contract`.
- `[ ]` 0F.4 `[after 0F.1]` Fixture recording harness `mockapi record`: given
  a running control service, calls every method in `INDEX.json` and writes
  the responses back into `testdata/fixtures/api/`, so later waves refresh
  fixtures with one command. Verify: `go test ./tools/development/mockapi/ -run Record` passes against a stub server.

### Gate 0

- `make up` brings PostgreSQL and Temporal healthy.
- `buf lint contracts && buf generate contracts` succeed; `tests/contract` round-trip (0F.3) passes in all three languages.
- `uv run pytest tools` passes (generation and data).
- All migrations apply twice; `sqlc generate` succeeds.
- `make ui-mock` serves every Wave 1 fixture (0F.1, 0F.2).
- UI track: `pnpm --dir apps/console build && lint && test` pass; `demo-path` spec shows 17 red steps (U0.6).
- `make hooks` installed and a deliberate `: any` commit is rejected.
- `claude docs/gate-reports/wave-0.md` written with the output of each command.

---

## 3. Wave 1 — vertical control slice

Implements FULL_SPEC §12 Phase 1 and TECHSTACK steps 2, 3 (fallback part), 4, 5.
Every lane starts with `[P]` items; the cross-lane waits are last.

### Lane 1A — gateway simulator

Owns: `services/gateway-simulator/`.

- `[ ]` 1A.1 `[P]` RED: `internal/battery/model_test.go` pins the worked
  example from `system-understanding.md`: 39.2 kWh usable, 74% SOC, 40%
  reserve, 10 kW inverter, 3.1 kW load, 0.95 discharge efficiency gives
  stored 29.008 kWh, reserve 15.680 kWh, above-reserve 13.328 kWh, AC
  available 12.6616 kWh, two-hour discharge 6.3308 kW, net export 3.2308 kW,
  backup 4.805 h. Verify: `go test ./services/gateway-simulator/internal/battery/` fails with "undefined".
- `[ ]` 1A.2 `[P]` GREEN: `internal/battery` implementing the energy update
  with one-way efficiencies, mode exclusion (no simultaneous charge and
  discharge), power and energy bounds, ramp limit, temperature derate hook,
  the operating-state machine from `truth-model.md` (on grid, off-grid
  outage, no home power, overcurrent with a configurable limit in kW,
  overcurrent standby), and backup-hours estimates at current usage and at
  750 W. An islanded state reports zero grid-service capacity.
  Verify: `go test ./services/gateway-simulator/internal/battery/` passes.
- `[ ]` 1A.3 `[P]` RED: `internal/gateway/store_test.go`: a command is
  persisted to SQLite before the acknowledgement is returned; a duplicate
  `command_id` is acknowledged idempotently without a second physical effect;
  a lower `generation` than the stored one is rejected with reason
  `OBSOLETE_GENERATION`; a command whose `expires_at` has passed is rejected.
  Verify: fails with "undefined".
- `[ ]` 1A.4 `[P]` GREEN: `internal/gateway` SQLite store (`modernc.org/sqlite`
  so the build stays pure Go) with `commands` and `telemetry_buffer` tables,
  write-then-ack ordering, dedup, generation check, effective and expiry
  enforcement (TECHSTACK "Gateway simulator and SQLite" items 1 to 4).
  Verify: `go test ./services/gateway-simulator/internal/gateway/` passes.
- `[ ]` 1A.5 `[P]` RED: `internal/telemetry/producer_test.go`: observations
  carry distinct `source_time`, `receive_time`, `observation_time`, a
  monotonic `sequence` per device, explicit units, the measurement
  boundary, the five-field `PowerFlow`, `state_of_energy_percent`, and the
  operating-state one-of; a gap yields `TelemetryUnavailable` and
  `ValueState=MISSING`, never a zero; the five power-flow fields balance
  (`to_home_kw` equals grid plus storage plus solar contributions within
  tolerance). Verify: fails.
- `[ ]` 1A.6 `[P]` GREEN: `internal/telemetry` producing observations from the
  battery model on the scenario clock, buffering to SQLite before publish,
  deleting only after confirmed cloud receipt, and replaying the buffer on
  reconnect (TECHSTACK items 6 to 8). Verify: `go test ./services/gateway-simulator/internal/telemetry/` passes.
- `[ ]` 1A.7 `[after 0B.6]` gRPC server in `cmd/gateway-simulator` implementing
  the command receipt and telemetry publish services from `gridos.v1`, loading
  a fleet file and scenario clock from flags. Real gRPC test in
  `services/gateway-simulator/tests/protocol_test.go` sending a `CommandIntent`
  and reading back an acknowledgement and telemetry. Verify: `go test ./services/gateway-simulator/...` passes.
- `[ ]` 1A.8 `[P]` RED then GREEN: restart test. Kill the process mid-event,
  restart, and assert retained commands still execute and buffered telemetry
  is delivered exactly once. Verify: `go test ./services/gateway-simulator/tests -run Restart` passes.

### Lane 1B — independent safety gate

Owns: `services/control/internal/safety/`.

- `[ ]` 1B.1 `[P]` RED: `safety_test.go` table tests, one case per check in
  TECHSTACK "Safety and delivery semantics": non-finite value, wrong vector
  length, charge bound, discharge bound, simultaneous charge and discharge,
  energy balance drift beyond tolerance, energy below effective reserve at
  interval k, availability false, maintenance lock, stale telemetry, meter
  export limit, interconnection limit, wrong measurement boundary, wrong
  generation, effective time in the past, expiry before effective, policy
  version mismatch, undeclared shortfall. Each case expects a specific
  machine-readable violation code. Verify: `go test ./services/control/internal/safety/` fails with "undefined".
- `[ ]` 1B.2 `[P]` GREEN: `Validate(plan, canonicalState) (Approval, []Violation)`
  reconstructing energy trajectories from canonical state, not from the
  plan's own claims. Verify: table tests pass.
- `[ ]` 1B.3 `[P]` RED then GREEN: effective reserve = max(hardware floor,
  plan floor, dynamic override). Cases: a `0%` Grid Flex plan still protects
  the hardware floor; a weather override raises the floor above the plan; a
  Travel Flex window before its start or after its end or after early return
  cannot lower the floor; an active override blocks Travel Flex.
  Verify: `go test ./services/control/internal/safety/ -run Reserve` passes.
- `[ ]` 1B.4 `[P]` RED then GREEN: fail closed. Missing state of charge,
  missing freshness, or contradictory inputs produce a rejection, never an
  approval (FULL_SPEC §4 invariant 8). Verify: `-run FailClosed` passes.
- `[ ]` 1B.5 `[P]` Property test with `pgregory.net/rapid`: for random plans
  and states, an approved plan never has energy below effective reserve at
  any interval. Verify: `go test ./services/control/internal/safety/ -run Property -rapid.checks=2000` passes.
- `[ ]` 1B.6 `[after 1C.6]` Differential test reading the golden fixtures in
  `testdata/golden/plans/`: every Python-approved plan is Go-approved and
  every Python-flagged plan is Go-rejected with the same violation family.
  Verify: `go test ./services/control/internal/safety/ -run Golden` passes.
- `[ ]` 1B.7 `[P]` Benchmark: validation of a 5,000-device, 24-interval plan
  under 2 seconds (FULL_SPEC §10). Verify: `go test ./services/control/internal/safety/ -bench Validate5000 -benchtime 3x` reports under 2 s per op.

### Lane 1C — decision service: energy math and deterministic fallback

Owns: `services/decision/`, `testdata/golden/`.

- `[ ]` 1C.1 `[P]` `uv` project at `services/decision` with Python 3.12,
  `numpy`, `polars`, `highspy` (installed now, used in Wave 3), `hypothesis`,
  `pytest`, `ruff`, `mypy --strict`. Verify: `uv run --directory services/decision mypy gridos` passes on the empty package.
- `[ ]` 1C.2 `[P]` RED: `tests/test_energy.py` pins the same worked example as
  1A.1 to four decimal places. Verify: fails with ImportError.
- `[ ]` 1C.3 `[P]` GREEN: `gridos/physics/energy.py` with the energy update,
  reserve inequality, per-interval energy-limited power, and backup-duration
  estimate given a critical-load forecast. Verify: `uv run pytest services/decision -k energy` passes.
- `[ ]` 1C.4 `[P]` RED: `tests/test_fallback.py`: given the 50-device fleet
  and a target that only 30 devices can meet, the fallback filters
  ineligible devices, ranks survivors, allocates conservatively, and reports a
  per-interval shortfall whose sum equals target minus allocated; an
  infeasible target never lowers any reserve. Verify: fails.
- `[ ]` 1C.5 `[P]` GREEN: `gridos/fallback/planner.py` implementing the
  `system-understanding.md` "Forecasts and optimization" fallback with the
  same hard constraints the solver will use. Verify: `uv run pytest services/decision -k fallback` passes.
- `[ ]` 1C.6 `[P]` Golden fixtures: `testdata/golden/plans/` with at least
  eight `OptimizationRequest` + expected `DispatchPlan` pairs (feasible,
  infeasible with shortfall, reserve-tight, stale device excluded, zero-percent
  plan with hardware floor, weather override, expired window, duplicate
  device). A pytest regenerates and diffs them. Verify: `uv run pytest services/decision -k golden` passes and the fixture directory is checked in.
- `[ ]` 1C.7 `[P]` Hypothesis invariants: for random fleets, the fallback
  plan never violates power, energy, or reserve bounds and declared shortfall
  is never negative. Verify: `uv run pytest services/decision -k hypothesis` passes with `--hypothesis-seed=0`.
- `[ ]` 1C.8 `[after 0B.6]` gRPC server `gridos/server.py` exposing
  `Optimize(OptimizationRequest) -> DispatchPlan` that runs the fallback (the
  solver arrives in Wave 3), with a hard timeout budget from the request and a
  `fallback=true` flag in the response. Verify: `uv run pytest services/decision -k server` passes using an in-process gRPC channel.
- `[ ]` 1C.9 `[P]` RED then GREEN: `validation/` module that independently
  checks any plan for finite values, vector lengths, and feasibility with
  explicit tolerances before it leaves the service. Verify: `uv run pytest services/decision -k validation` passes.

### Lane 1D — storage, event state, transactional outbox

Owns: `services/control/internal/storage/` (including
`internal/storage/publisher/`), `services/control/go.mod` (Wave 1 owner),
`services/control/cmd/migrate`.

- `[ ]` 1D.1 `[P]` `services/control/go.mod` pre-declaring every Go dependency
  Waves 1 and 2 need so no other lane edits it: `pgx/v5`, `golang-migrate`,
  `connect-go`, `grpc`, `pgregory.net/rapid`, `testcontainers-go`, the
  Temporal Go SDK, `modernc.org/sqlite` is not needed here (gateway is its
  own module). sqlc-generated code from 0D.5 committed under
  `internal/storage/gen`, and `cmd/migrate` applying `database/migrations`
  with `golang-migrate`. Verify: `go run ./services/control/cmd/migrate up` against `make up` succeeds and `go build ./...` succeeds.
- `[ ]` 1D.2 `[P]` Test harness: each test gets a disposable database via
  `CREATE DATABASE` on the compose PostgreSQL (or `testcontainers-go`), with
  migrations applied. Verify: `go test ./services/control/internal/storage/ -run Harness` passes.
- `[ ]` 1D.3 `[P]` RED: `events_test.go`: state transitions follow the
  transition table in `truth-model.md`; an illegal transition returns
  `ErrIllegalTransition`; a conditional update with a stale expected state
  affects zero rows; every transition appends an `audit_journal` row.
  Verify: fails.
- `[ ]` 1D.4 `[P]` GREEN: `internal/storage/events.go`. Verify: `go test ./services/control/internal/storage/ -run Event` passes.
- `[ ]` 1D.5 `[P]` RED: `outbox_test.go`: inserting a command intent and its
  outbox row is one transaction (a forced failure after the intent insert
  leaves neither row); claiming a batch uses `SKIP LOCKED` so two claimers
  never receive the same row; a re-claimed row after a crash keeps the same
  `command_id` and payload (FULL_SPEC §4 invariant 5). Verify: fails.
- `[ ]` 1D.6 `[P]` GREEN: `internal/storage/outbox.go` and the
  `OutboxPublisher` interface the control API and workflows will use.
  Verify: `-run Outbox` passes.
- `[ ]` 1D.7 `[P]` RED then GREEN: acknowledgements and uncertainty intervals.
  Recording an acknowledgement never changes the event state directly; a send
  with no acknowledgement inside its deadline is marked `UNCERTAIN` with a
  stored signed feasible-power interval. Verify: `-run Ack` passes.
- `[ ]` 1D.8 `[P]` Model-based test with `rapid`: random sequences of
  transitions, acknowledgements, and expiries against an in-memory reference
  model of the command state machine; the database agrees with the model at
  every step. Verify: `go test ./services/control/internal/storage/ -run Model -rapid.checks=500` passes.
- `[ ]` 1D.9 `[after 1A.7]` `internal/storage/publisher`: claims outbox rows
  and delivers them over gRPC to the gateway, records acknowledgements, marks
  uncertain on deadline, retries with the same `command_id` (TECHSTACK
  "Transactional command outbox"). Integration test against the real gateway
  simulator process. Verify: `go test ./services/control/internal/storage/publisher/` passes.

### Lane 1E — fleet twin, eligibility, control API

Owns: `services/control/internal/fleet/`, `services/control/internal/api/`,
`services/control/internal/report/`, `services/control/cmd/control`.

- `[ ]` 1E.1 `[P]` RED: `fleet/twin_test.go`: the twin holds the latest
  accepted state per site; a command being issued does not change the twin
  (FULL_SPEC §5.3); telemetry older than the freshness threshold marks the
  site `STALE`; a site whose operating state is any off-grid variant or
  `TelemetryUnavailable` contributes zero dispatchable capacity; the twin
  carries both backup-hours estimates; aggregate MW and MWh are sums of
  fresh on-grid sites only, and every aggregate carries a timestamp,
  provenance mix, and freshness. Verify: fails.
- `[ ]` 1E.2 `[P]` GREEN: `internal/fleet/twin.go`. Verify: `go test ./services/control/internal/fleet/ -run Twin` passes.
- `[ ]` 1E.3 `[P]` RED then GREEN: eligibility with exclusion reasons
  (offline, stale, islanded or off-grid, overcurrent, maintenance lock,
  under reserve, alarm, outside region, outside participation window). Under-reserved devices are excluded and
  reported (TECHSTACK e2e scenario 2). Verify: `-run Eligibility` passes.
- `[ ]` 1E.4 `[after 1D.6]` ConnectRPC server `cmd/control` with handlers:
  `GetFleetSummary`, `ListSites` (H3 aggregate by default, exact location
  only with the separately granted `site_location` permission),
  `CreateEventRequest`, `GetEvent`, `ApproveEvent`. Authorization enforced
  server-side for the six FULL_SPEC §11 roles; the approver role is required
  for `ApproveEvent`. Tests use the generated Connect client.
  Verify: `go test ./services/control/internal/api/` passes including a 403 for the wrong role.
- `[ ]` 1E.5 `[after 1B.2, 1C.8, 1D.9]` Phase 1 straight-line dispatcher (no
  Temporal yet, replaced in Wave 2): create event, freeze snapshot, call the
  decision service, run the safety gate, require approval, persist intents,
  hand to the publisher. Marked `REPLACED-IN-WAVE-2` in code and reported as
  a stub. Verify: `go test ./services/control/internal/api/ -run Dispatch` passes.
- `[ ]` 1E.6 `[P]` RED then GREEN: `internal/report` basic event report
  (requested, approved, commanded, acknowledged MW; devices excluded by
  reason; provenance and versions) assembled from storage.
  Verify: `go test ./services/control/internal/report/` passes.
### Lane 1F — telemetry ingest and vertical-slice end to end

Owns: `services/control/internal/ingest/`, `tests/end-to-end/`, the
`Makefile` `demo` target, `testdata/fixtures/api/` (recording).

- `[ ]` 1F.1 `[P]` RED: `ingest_test.go`: the control-side gRPC telemetry
  receive service acknowledges receipt only after a durable write, so the
  gateway may delete its buffer (TECHSTACK gateway item 7); out-of-order
  sequences are accepted and ordered by `sequence`; duplicates by
  (`device_id`, `sequence`) are dropped; `MISSING` and `STALE` are stored as
  states, never coerced to zero. Verify: fails with "undefined".
- `[ ]` 1F.2 `[after 1D.6, 1E.2]` GREEN: `internal/ingest` writing
  observations through storage and updating the fleet twin.
  Verify: `go test ./services/control/internal/ingest/` passes.
- `[ ]` 1F.3 `[after 1E.5, 1A.7, 1F.2]` `make demo` target (compose, migrate,
  seed `texas-50`, start gateway simulator, decision service, control
  service, console) and `tests/end-to-end/vertical_slice_test.go` driving
  the whole Phase 1 path through the API: create event, fallback plan,
  safety approved, operator approval recorded, intents persisted before any
  network send, gateway acknowledges, telemetry ingested, basic report
  contains provenance and versions. Verify: `make demo` comes up and `go test ./tests/end-to-end/ -run VerticalSlice` passes.
- `[ ]` 1F.4 `[after 1F.3]` Record real responses for every Wave 1 method
  from the running demo stack with `mockapi record`, replacing the
  hand-authored fixtures. Verify: `go test ./tools/development/mockapi/ -run Fixtures` passes on the recordings and every Wave 1 file in `testdata/fixtures/api/` changed.
- `[ ]` 1F.5 `[after 1F.3]` Duplicate-delivery end to end: publish the same
  `command_id` twice through the outbox; the gateway shows one physical
  effect and storage shows one acknowledgement. Verify: `go test ./tests/end-to-end/ -run DuplicateDelivery` passes.

### Gate 1

- `make demo` (1F.3) starts compose, migrates, seeds `texas-50`, starts
  gateway simulator, decision service, control service, and console.
- `tests/end-to-end/vertical_slice_test.go` (1F.3) and
  `DuplicateDelivery` (1F.5) pass.
- Recorded fixtures (1F.4) replace every hand-authored Wave 1 fixture.
- `go test ./...`, `uv run pytest`, `pnpm test` all green; 1B.7 under 2 s.
- UI track: Playwright demo-path steps 2, 3, 9, 16 green against `make demo`.
- `claude docs/gate-reports/wave-1.md` written.

---

## 4. Wave 2 — durable failure handling

Implements FULL_SPEC §12 Phase 2 and TECHSTACK step 6.

### Lane 2A — failure laboratory in the simulator

Owns: `services/gateway-simulator/internal/failures/`,
`services/gateway-simulator/cmd/`.

- `[ ]` 2A.1 `[P]` RED: `failures/inject_test.go` for each injection in
  FULL_SPEC §5.8: offline device, delayed telemetry, dropped message,
  duplicated message, gateway restart, partial region outage, bad forecast
  hook, hot battery, stale state, optimizer timeout (signalled to the decision
  service through the scenario). The remaining §5.8 injection, worker
  restart, is a control-plane fault and is exercised by 2B.6. Each injection
  is seeded and reproducible. Verify: fails.
- `[ ]` 2A.2 `[P]` GREEN: injection engine driven by the scenario file from
  0C.6, keyed off the scenario clock. Verify: `go test ./services/gateway-simulator/internal/failures/` passes.
- `[ ]` 2A.3 `[P]` RED then GREEN: "twenty percent of Houston devices
  disconnected" computes lost MW from the affected devices' schedules, not
  20% of fleet MW (`system-understanding.md` "Reliable execution"). Verify: `-run HoustonTwenty` passes.
- `[ ]` 2A.4 `[P]` RED then GREEN: network outage followed by SQLite replay
  delivers every buffered observation once, in sequence, with original
  `source_time`. Verify: `-run OutageReplay` passes.
- `[ ]` 2A.5 `[P]` Scenario runner flag `--scenario <file>` on the simulator
  binary; a run with the same seed twice produces identical telemetry hashes.
  Verify: `go test ./services/gateway-simulator/tests -run ScenarioDeterminism` passes.

### Lane 2B — Temporal dispatch workflow

Owns: `services/control/internal/dispatch/`, `services/control/cmd/worker`,
`services/control/internal/api/` root package only (to retire the Wave 1 dispatcher; `internal/api/events/` is 2F's),
`services/control/go.mod` (Wave 2 owner).

- `[ ]` 2B.1 `[P]` `cmd/worker`, task queue, and a workflow test suite using
  the Temporal SDK test environment with time skipping (the SDK dependency
  was pre-declared in 1D.1).
  Verify: `go test ./services/control/internal/dispatch/ -run Smoke` passes.
- `[ ]` 2B.2 `[P]` RED: `workflow_test.go` walking the eleven states in order
  with activities mocked; the workflow never skips `VALIDATED` or `APPROVED`;
  an `ApproveEvent` signal is required before `COMMANDS_PERSISTED`; an
  `EmergencyStop` signal from any state after `SENT` issues superseding
  zero-setpoint commands with a higher generation. Verify: fails.
- `[ ]` 2B.3 `[P]` GREEN: the workflow and activities (freeze inputs, request
  plan, validate, wait approval, persist intents, publish, track, verify,
  end, reconcile, report) as listed in FULL_SPEC §5.7. Verify: `-run Lifecycle` passes.
- `[ ]` 2B.4 `[P]` RED then GREEN: retry classes. Transient gateway errors
  retry with the same command ID; validation failures do not retry; command
  expiry is a durable timer; a device replaced within the approved envelope
  gets a new generation, never a reused one. Verify: `-run Retry` passes.
- `[ ]` 2B.5 `[P]` Replay test: recorded workflow history from 2B.3 replays
  without nondeterminism errors after the code change in 2B.4. Verify: `go test ./services/control/internal/dispatch/ -run Replay` passes.
- `[ ]` 2B.6 `[after 1D.9]` Worker-kill integration test: start an
  event against the compose Temporal, kill the worker process after `SENT`,
  restart it, and assert the event reaches `VERIFIED` with no duplicated
  physical intent (FULL_SPEC §10 "Resume an in-flight event"). Verify: `go test ./services/control/tests -run WorkerRestart` passes.
- `[ ]` 2B.7 `[after 2B.6]` Remove the Wave 1 straight-line dispatcher from
  `internal/api` and report the stub as retired. Verify: `grep -rn REPLACED-IN-WAVE-2 services` prints nothing.

### Lane 2C — reconciliation: acknowledgement versus delivery

Owns: `services/control/internal/reconciliation/`,
`services/control/internal/report/`.

- `[ ]` 2C.1 `[P]` RED: `uncertain_test.go`: a send whose acknowledgement is
  lost produces a signed feasible-power interval derived from the last
  confirmed command, the possibly accepted command, effective and expiry
  times, ramp behaviour, and fresh telemetry (TECHSTACK "Safety and delivery
  semantics"); the interval's upper bound is never below the possibly-executing
  setpoint. Verify: fails.
- `[ ]` 2C.2 `[P]` GREEN: `internal/reconciliation/uncertain.go`. Verify: `-run Uncertain` passes.
- `[ ]` 2C.3 `[P]` RED then GREEN: capacity is not reallocated over an
  uncertain device until its command expiry passes or fresh telemetry proves
  its state (TECHSTACK e2e scenario 4 and "Safety and delivery semantics":
  the system does not blindly replace capacity that may still be operating).
  Verify: `-run NoOvershoot` passes.
- `[ ]` 2C.4 `[P]` RED then GREEN: delivery verification integrates power over
  actual elapsed intervals; measurement gaps stay `UNKNOWN` and are reported
  as uncertain intervals, never as zero delivery (TECHSTACK e2e scenario 11).
  Verify: `-run Gaps` passes.
- `[ ]` 2C.5 `[P]` RED then GREEN: late and duplicated observations update
  reconciliation history by event time without erasing earlier knowledge.
  Verify: `-run Late` passes.
- `[ ]` 2C.6 `[P]` Property test: for random sequences of acks, telemetry,
  and expiries, reported delivered energy is never greater than the integral
  of the telemetry actually received. Verify: `-run Property -rapid.checks=1000` passes.
- `[ ]` 2C.7 `[after 2B.3]` Reconciliation activities registered with the
  workflow; `internal/report` from 1E.6 now includes delivered MW/MWh,
  tracking error, response latency, and uncertain intervals (FULL_SPEC §5.9).
  Verify: `go test ./services/control/internal/report/ -run Delivered` passes.

### Lane 2D — scenarios and cross-service integration harness

Owns: `testdata/scenarios/`, `tests/integration/`, `tests/end-to-end/`,
`tools/development/scenario-run.sh`.

- `[ ]` 2D.1 `[P]` Write every scenario file for the TECHSTACK "Required
  end-to-end scenarios" list that Waves 2 and 3 can exercise:
  `heat-event-canonical` (from 0C.7, run in 3D.5 once the solver exists),
  `under-reserved-excluded`,
  `houston-20pct-offline`, `lost-ack-still-executing`,
  `old-command-expiry-newer-pending`, `worker-termination`,
  `gateway-restart`, `network-outage-sqlite-replay`,
  `measurement-gap-unknown`. Each validates against 0C.6.
  Verify: `for f in testdata/scenarios/*.yaml; do uv run python -m tools.generation.scenario validate "$f"; done` prints OK for each.
- `[ ]` 2D.2 `[P]` `tests/integration/harness_test.go`: starts compose,
  gateway simulator with `--scenario`, decision service, control service and
  worker, then drives an event through the API and asserts the scenario's
  expected outcomes. Written now with `t.Skip` per scenario until the lane
  that provides the behaviour lands. Verify: `go test ./tests/integration/ -run Harness` passes the harness self-test.
- `[ ]` 2D.3 `[after 2A.2, 2B.3, 2C.2]` Unskip and pass: `houston-20pct-offline`,
  `lost-ack-still-executing`, `old-command-expiry-newer-pending`.
  Verify: `go test ./tests/integration/ -run "Houston|LostAck|OldExpiry"` passes.
- `[ ]` 2D.4 `[after 2A.4, 2B.6]` Unskip and pass: `worker-termination`,
  `gateway-restart`, `network-outage-sqlite-replay`. Verify: `-run "Worker|GatewayRestart|OutageReplay"` passes.
- `[ ]` 2D.5 `[after 2C.4]` Unskip and pass: `measurement-gap-unknown`,
  `under-reserved-excluded`. Verify: `-run "Gap|UnderReserved"` passes.
- `[ ]` 2D.6 `[P]` Audit-trail invariant: after every scenario, the
  `audit_journal` contains an unbroken chain of transitions for the event and
  every command (FULL_SPEC §10 "without losing the event audit trail").
  Verify: `go test ./tests/integration/ -run AuditChain` passes.

### Lane 2E — decision service hardening

Owns: `services/decision/`, `testdata/golden/`.

- `[ ]` 2E.1 `[P]` RED then GREEN: timeout. A request whose budget is exceeded
  returns the fallback plan with `fallback=true` and the timeout reason; the
  solver work runs in a subprocess that is killed on timeout
  (`system-understanding.md` "Reliable execution"). Verify: `uv run pytest services/decision -k timeout` passes.
- `[ ]` 2E.2 `[P]` RED then GREEN: no incumbent. A request with no prior plan
  and an unhealthy solver still returns a fallback, never an error.
  Verify: `-k no_incumbent` passes.
- `[ ]` 2E.3 `[P]` RED then GREEN: invalid vector. A solver result with NaN,
  wrong length, or bound violation is rejected by `validation/` and replaced
  by the fallback. Verify: `-k invalid_vector` passes.
- `[ ]` 2E.4 `[P]` RED then GREEN: cohort replacement. Given a list of devices
  that dropped out mid-event, the service proposes replacements only within
  the approved envelope and reports the shortfall if none exist.
  Verify: `-k replacement` passes.
- `[ ]` 2E.5 `[P]` Uncertainty margins: availability probability reduces
  counted capacity; `confidence × nameplate` is never presented as guaranteed
  (`system-understanding.md`). RED pins a numeric case. Verify: `-k margin` passes.
- `[ ]` 2E.6 `[P]` Extend `testdata/golden/plans/` with timeout, no-incumbent,
  invalid-vector, and replacement cases for the Wave 3 Go differential run.
  Verify: `uv run pytest services/decision -k golden` passes.

### Lane 2F — scale, live stream, fixtures

Owns: `services/gateway-simulator/tests/`, `services/control/internal/api/events/`,
`tests/end-to-end/`, `testdata/fixtures/api/`.

- `[ ]` 2F.1 `[P]` RED then GREEN: simulator scale. 5,000 simulated devices
  in one process produce telemetry every 5 seconds for 10 minutes with no
  sequence gaps (FULL_SPEC §8 "approximately 5,000 devices"; the cadence and
  resource budget are assumptions to report).
  Verify: `go test ./services/gateway-simulator/tests -run Scale5000 -timeout 20m` passes.
- `[ ]` 2F.2 `[after 1F.2]` RED then GREEN: ingest scale. The control plane
  sustains the 5,000-device stream and twin freshness stays under 5 seconds
  (FULL_SPEC §10). Verify: `go test ./tests/end-to-end/ -run IngestScale -timeout 20m` passes.
- `[ ]` 2F.3 `[after 2C.7]` Record live-event fixtures from a scenario run:
  `GetEvent` at several lifecycle states, the timeline, and an
  `EmergencyStop` response, into `testdata/fixtures/api/`.
  Verify: `go test ./tools/development/mockapi/ -run Fixtures` passes.
- `[ ]` 2F.4 `[after 2D.4]` Run the UI track's `demo-path` spec against
  `make demo` and record the result in the gate report; route failures to the
  UI track or the owning backend lane. Verify: `pnpm --dir apps/console playwright test demo-path` executed and output saved.
- `[ ]` 2F.5 `[after 2C.7]` `internal/api/events`: `WatchEvent`
  server-streaming Connect method emitting sent, acknowledged, delivered, and
  uncertain-interval updates as reconciliation produces them, registered in
  `cmd/control`. Verify: `go test ./services/control/internal/api/events/ -run Watch` passes and a client sees an update within 5 s of new telemetry.

### Gate 2

- All eight Wave 2 scenarios pass in `tests/integration` (the canonical
  5,000-site heat event runs in Wave 3 once the solver meets its budget).
- Worker restart resumes the event; gateway restart retains commands; network
  outage replays telemetry once.
- Idempotent redelivery proven at gateway and outbox.
- Audit chain unbroken in every scenario.
- Simulator and ingest sustain 5,000 devices; `WatchEvent` delivers within 5 s.
- UI track: demo-path steps 2, 3, 9 to 16 green (2F.4).
- `claude docs/gate-reports/wave-2.md` written.

---

## 5. Wave 3 — forecasting and optimization

Implements FULL_SPEC §12 Phase 3 and TECHSTACK steps 3 (solver), 4, 8.

### Lane 3A — forecasting

Owns: `services/decision/gridos/forecasting/`,
`services/decision/tests/forecasting/`. (3B owns `pyproject.toml` this wave
and pre-declares any forecasting dependency in 3B.1.)

- `[ ]` 3A.1 `[P]` RED: `test_load_baseline.py`: the similar-day baseline for
  a site with the RESHIWR COAST profile reproduces a held-out day within a
  pinned mean absolute error, and the output carries `training_window`,
  `feature_version`, `model_version`, `issued_at`, `horizon`, and a calibrated
  interval (FULL_SPEC §5.4). Verify: fails.
- `[ ]` 3A.2 `[P]` GREEN: `forecasting/load.py` similar-day baseline using the
  normalized ERCOT profiles from 0E.3 assigned by 0C.3. Verify: `uv run pytest services/decision -k load_baseline` passes.
- `[ ]` 3A.3 `[P]` RED then GREEN: regional load and price forecasts from
  normalized ERCOT system load and DAM/RTM prices (persistence plus
  day-ahead where available), with realized-error evaluation.
  Verify: `-k regional` passes.
- `[ ]` 3A.4 `[P]` RED then GREEN: outage risk per county-hour from the 0E.4
  rate table combined with active NWS alerts from 0E.5; output is labelled
  `value_kind=modeled_estimate`. Verify: `-k outage_risk` passes.
- `[ ]` 3A.5 `[P]` RED then GREEN: availability and failure probability per
  device from reliability traits, connectivity history, and freshness; SOC
  trajectory forecast whose interval widens with telemetry age.
  Verify: `-k availability` passes.
- `[ ]` 3A.6 `[P]` Evaluation harness `forecasting/evaluate.py` recording
  realized error per forecast, and a deterministic baseline that is used when
  a learned model is missing or unhealthy. Verify: `-k evaluate` passes.

### Lane 3B — HiGHS rolling-horizon optimizer

Owns: `services/decision/gridos/optimization/`,
`services/decision/tests/optimization/`, `services/decision/gridos/server.py`,
`services/decision/pyproject.toml` (Wave 3 owner), `testdata/golden/`.

- `[ ]` 3B.1 `[P]` RED: `test_optimizer_small.py`: on the 50-device fleet with
  a 2-hour window and a target only 30 devices can meet, the LP allocates the
  target at every interval, respects every reserve, produces zero shortfall,
  and solves under 1 second. Verify: fails.
- `[ ]` 3B.2 `[P]` GREEN: `optimization/model.py` building the HiGHS model
  with nonnegative `p_ch` and `p_dis`, one-way efficiencies, energy and power
  bounds, effective reserve at every interval, mode exclusion, soft shortfall
  variable, objective valuing delivered service and penalizing shortfall,
  cycling, and uncertainty (FULL_SPEC §5.5). Verify: `-k optimizer_small` passes.
- `[ ]` 3B.3 `[P]` RED then GREEN: infeasible target yields a visible
  per-interval shortfall and no reserve relaxation (TECHSTACK e2e scenario 10).
  Verify: `-k infeasible` passes.
- `[ ]` 3B.4 `[P]` RED then GREEN: cohort construction and disaggregation from
  cohort plan to per-device `DeviceSchedule`, with a reconstruction test that
  sums disaggregated schedules back to the cohort plan within tolerance.
  Verify: `-k disaggregate` passes.
- `[ ]` 3B.5 `[P]` Output includes objective breakdown, constraint margins,
  excluded devices with reasons, and a feasible fallback alongside
  (FULL_SPEC §5.5). Verify: `-k explain` passes.
- `[ ]` 3B.6 `[P]` Performance: canonical 5,000-device scenario plans under 10
  seconds, and a forced timeout returns the fallback (FULL_SPEC §10).
  Verify: `uv run pytest services/decision -k perf_5000 --durations=1` reports under 10 s.
- `[ ]` 3B.7 `[P]` Hypothesis invariants over the solver output identical to
  1C.7. Verify: `-k hypothesis_solver` passes.
- `[ ]` 3B.8 `[after 3B.5]` Server switches to solver-first, fallback on
  timeout or invalid result; golden fixtures regenerated and reviewed.
  Verify: `-k golden` passes and the diff is reviewed in the mailbox report.

### Lane 3C — Go differential validation and performance

Owns: `services/control/internal/safety/`, `services/control/tests/`,
`services/control/go.mod` (Wave 3 owner).

- `[ ]` 3C.1 `[after 3B.8]` Golden differential rerun over the solver-produced
  fixtures. Verify: `go test ./services/control/internal/safety/ -run Golden` passes.
- `[ ]` 3C.2 `[P]` RED then GREEN: aggregate commitment check and ramp-rate
  check added to the gate (FULL_SPEC §5.6). Verify: `-run "Aggregate|Ramp"` passes.
- `[ ]` 3C.3 `[P]` RED then GREEN: any material input change after approval
  invalidates the plan version and requires re-approval (FULL_SPEC §5.6).
  Verify: `-run Reapproval` passes.
- `[ ]` 3C.4 `[P]` Load-shaped benchmark: 5,000 devices, 288 intervals (24 h at
  5 min) under 2 seconds. Verify: `-bench Validate5000x288 -benchtime 3x` under 2 s.

### Lane 3D — planning integration in the control plane

Owns: `services/control/internal/dispatch/`, `services/control/internal/api/`,
`tests/integration/`, `testdata/scenarios/`.

- `[ ]` 3D.1 `[P]` RED then GREEN: input snapshot freezing. The workflow
  stores the exact forecast inputs, eligibility snapshot, policy versions, and
  model versions used, and the plan version references them (FULL_SPEC §4
  invariant 9). Verify: `go test ./services/control/internal/dispatch/ -run Snapshot` passes.
- `[ ]` 3D.2 `[after 3A.6, 3B.8]` Forecast and optimize activities call the
  decision service with a budget; a timeout is recorded as a decision in the
  timeline and the fallback plan proceeds to validation. Verify: `-run PlanningActivities` passes.
- `[ ]` 3D.3 `[P]` Explanation API: `GetPlanExplanation` returning objective
  breakdown, reserve held back, constraint margins, exclusions with reasons,
  and per-interval shortfall. Verify: `go test ./services/control/internal/api/ -run Explanation` passes.
- `[ ]` 3D.4 `[P]` Reject-then-approve path: the API can validate an
  intentionally unsafe alternative and return machine-readable violations
  without creating commands (FULL_SPEC §9 step 8). Verify: `-run UnsafeAlternative` passes.
- `[ ]` 3D.5 `[after 3D.2]` Scenarios `optimizer-timeout-fallback` and
  `infeasible-target-shortfall` added to `tests/integration`, and
  `heat-event-canonical` (TECHSTACK e2e scenario 1) unskipped now that the
  solver plans 5,000 sites inside its budget. All three passing.
  Verify: `go test ./tests/integration/ -run "Timeout|Infeasible|Canonical"` passes.

### Lane 3E — BigQuery optional sink and replay

Owns: `services/control/internal/analytics/`.

- `[ ]` 3E.1 `[P]` `analytics.Sink` interface with two implementations: a
  fixture-backed local sink writing newline JSON under `.gridos/analytics/`
  (default) and a BigQuery sink enabled only by `GRIDOS_ANALYTICS=bigquery`
  (TECHSTACK "Local and deployed topology"). RED tests use the local sink.
  Verify: `go test ./services/control/internal/analytics/` passes.
- `[ ]` 3E.2 `[P]` Raw telemetry, normalized telemetry, forecasts and actuals,
  dispatch and verification facts, and data-quality facts exported as
  append-only records with the provenance fields from FULL_SPEC §2.
  Verify: `-run Export` passes.
- `[ ]` 3E.3 `[P]` BigQuery sink integration test guarded by a build tag and a
  recorded-request fixture; one live smoke test documented but skipped without
  credentials. Verify: `go test -tags bigquery ./services/control/internal/analytics/` passes with the fixture.
- `[ ]` 3E.4 `[P]` Assert the safety gate and command path never import the
  analytics package (TECHSTACK "BigQuery is not queried by the safety gate").
  Verify: `go list -deps ./services/control/internal/safety ./services/control/internal/storage/publisher | grep analytics` prints nothing.

### Lane 3F — replay and explanation fixtures

Owns: `services/control/internal/replay/`, `services/control/cmd/replay`,
`services/control/internal/api/replay/`, `tests/end-to-end/`,
`testdata/fixtures/api/`.

- `[ ]` 3F.1 `[P]` RED then GREEN: replay manifest written per event: seed,
  fleet file hash, scenario hash, input snapshot IDs, solver and fallback
  versions, code version (FULL_SPEC §4 invariant 9).
  Verify: `go test ./services/control/internal/replay/ -run Manifest` passes.
- `[ ]` 3F.2 `[after 3D.2]` `cmd/replay` (a product capability, FULL_SPEC §3
  and §10, so it lives with the control plane) re-running an event from its
  manifest and diffing the outcome; identical apart from explicitly recorded
  nondeterminism. Verify: `go run ./services/control/cmd/replay --event <id>` prints `IDENTICAL`.
- `[ ]` 3F.3 `[after 3D.3, 3A.6]` Record `GetPlanExplanation`, forecast
  responses with intervals, a `fallback=true` case, and `ValidateAlternative`
  violations into `testdata/fixtures/api/`. Verify: `go test ./tools/development/mockapi/ -run Fixtures` passes.
- `[ ]` 3F.4 `[after 3F.2]` `internal/api/replay`: `ReplayEvent` Connect
  method returning the diff for the console. Verify: `go test ./services/control/internal/api/replay/` passes.
- `[ ]` 3F.5 `[after 3D.5]` Run the UI track's `demo-path` spec at the gate
  and record the result. Verify: output saved in the gate report.

### Gate 3

- Canonical 5,000-device scenario: plan under 10 s, validation under 2 s.
- Optimizer timeout → fallback; infeasible target → visible shortfall.
- Golden differential Go versus Python green.
- Replay of the canonical event prints `IDENTICAL` (3F.2).
- Safety and command path have no analytics dependency.
- UI track: demo-path green except steps 1 and 7 (3F.5).
- `claude docs/gate-reports/wave-3.md` written.

---

## 6. Wave 4 — data-rich operations and member flexibility

Implements FULL_SPEC §12 Phase 4 and TECHSTACK step 9.

### Lane 4A — member policy engine and storage

Owns: `services/control/internal/fleet/policy/`,
`services/control/internal/storage/` (policy queries only),
`services/control/internal/api/member/`, `services/control/cmd/control`
(handler registration), `database/migrations/` and `database/queries/`
(additions only), `services/control/go.mod` (Wave 4 owner).

- `[ ]` 4A.1 `[P]` RED: `policy_test.go`: only a consented, effective-dated
  plan applies; a plan change records consent text, version, effective time,
  and the explanation shown (FULL_SPEC §5.10); plan names, bands, prices, and
  rewards come from a versioned market catalog, never constants. Verify: fails.
- `[ ]` 4A.2 `[P]` GREEN: `fleet/policy` with catalog loading from
  `pricing_catalog_snapshots`. Verify: `go test ./services/control/internal/fleet/policy/` passes.
- `[ ]` 4A.3 `[P]` RED then GREEN: Travel Flex lifecycle. Window applies only
  between consented start and end in its local timezone, expires
  automatically, early return cancels it and restores the safer reserve; a
  fixed daily, event, or annual credit is recorded, never a per-kWh discount by
  default. Verify: `-run TravelFlex` passes.
- `[ ]` 4A.4 `[P]` RED then GREEN: dynamic overrides. Severe weather, outage
  risk, stale telemetry, device alarms, or loss of communications raise the
  effective reserve immediately and are recorded as `ReserveOverride` rows
  (FULL_SPEC §4 invariant 11). Verify: `-run Override` passes.
- `[ ]` 4A.5 `[P]` RED then GREEN: versioned offers and the reward ledger. A
  `FlexibilityOffer` row is written whenever a plan or Travel Flex option is
  shown, holding the catalog version, price, and consent text presented, and
  is immutable; the reward ledger is append-only; every decision stores the
  pricing, reward, and consent versions it used (FULL_SPEC §5.11, §10).
  Verify: `-run "Offer|Ledger"` passes.
- `[ ]` 4A.6 `[P]` RED then GREEN: away-mode anomaly alert compares load to a
  consented baseline only while the home is marked away (by an active Travel
  Flex window or an explicit away flag) and the member opted in; the alert
  text is the fixed "energy anomaly signal" wording (FULL_SPEC §5.10).
  Verify: `-run Anomaly` passes.
- `[ ]` 4A.7 `[after 4A.5]` Connect service `internal/api/member` with
  `SelectResiliencePlan`, `ScheduleTravelFlex`, `EndTravelFlexEarly`,
  `GetMemberStatus` (operating state, `state_of_energy_percent`, backup
  hours at current usage and at 750 W, the recent `GridEvent` list, current
  plan and reserve), the member role only seeing its own site, registered in
  `cmd/control`. Verify: `go test ./services/control/internal/api/member/` passes.

### Lane 4B — incremental margin evaluator

Owns: `services/decision/` (whole package this wave, including
`gridos/optimization/` for 4B.6 and `pyproject.toml`).

- `[ ]` 4B.1 `[P]` RED: `test_margin.py` pins the FULL_SPEC §5.11 formula on a
  worked case with every term, and that the conservative estimate uses the
  low end of value and the high end of every cost. Verify: fails.
- `[ ]` 4B.2 `[P]` GREEN: `economics/margin.py` producing `MarginEstimate`.
  Verify: `uv run pytest services/decision -k margin_formula` passes.
- `[ ]` 4B.3 `[P]` RED then GREEN: hurdle gate. Additional flexibility is used
  only when the conservative margin clears the configured hurdle; a negative
  margin produces no additional dispatch (TECHSTACK e2e scenario 15).
  Verify: `-k hurdle` passes.
- `[ ]` 4B.4 `[P]` RED then GREEN: market-specific rewards. A market with no
  membership fee produces a fixed reward, not a fee waiver (TECHSTACK e2e
  scenario 16). Verify: `-k no_fee_market` passes.
- `[ ]` 4B.5 `[P]` Bill-and-value simulator: expected member savings and
  company margin over a backtest window from normalized public prices and
  profiles, every output labelled `SIMULATED` or `DERIVED` (DATASETS §15).
  Verify: `-k bill_simulator` passes.
- `[ ]` 4B.6 `[after 4A.2]` Optimizer consumes Travel Flex capacity only when
  the policy engine reports it active and 4B.3 clears. Verify: `-k travel_flex_capacity` passes.

### Lane 4C — geographic and electrical map data

Owns: `services/control/internal/fleet/geo/`,
`services/control/internal/api/geo/`, `testdata/fixtures/geo/`.

- `[ ]` 4C.1 `[P]` RED then GREEN: server-side H3 aggregation of sites at
  resolutions 5 through 8 with counts, capacity, SOC bands, connectivity, and
  active dispatch per cell; cells with fewer than 5 sites are merged upward
  before leaving the server (FULL_SPEC §5.2 privacy; the resolutions and the
  merge threshold are assumptions to report). Verify: `go test ./services/control/internal/fleet/geo/` passes.
- `[ ]` 4C.2 `[after 4D.3]` Connect service `internal/api/geo` with
  `ListCells` and `Drilldown` through market → load zone → utility territory
  → substation → feeder → authorized site, substation and feeder from a
  clearly labelled synthetic or licensed public model (FULL_SPEC §5.2), exact
  site only with the `site_location` permission; registered in `cmd/control`
  after 4D.3's registration lands. Verify: `go test ./services/control/internal/api/geo/` passes including a 403 test.
- `[ ]` 4C.3 `[P]` Offline map assets: a MapLibre style JSON and small Texas
  boundary, weather-zone, and load-zone GeoJSON under `testdata/fixtures/geo/`
  served by the control service, so the map renders with no proprietary token
  and no network (TECHSTACK "Fleet map"). Verify: `curl localhost:8080/geo/style.json` returns the style and `du -sh testdata/fixtures/geo` under 5 MB.
- `[ ]` 4C.4 `[P]` RED then GREEN: no geo response ever carries a street
  address or a real member home; a test scans every geo response type for
  address-like fields. Verify: `go test ./services/control/internal/api/geo/ -run NoAddress` passes.

### Lane 4D — public context services

Owns: `services/control/internal/context/`, `services/control/internal/api/context/`.

- `[ ]` 4D.1 `[P]` RED then GREEN: reference loaders for normalized ERCOT
  prices, system load, outage rates, and NWS forecasts and alerts from
  `testdata/fixtures/public` (offline default) with provenance and freshness
  on every response. Verify: `go test ./services/control/internal/context/` passes.
- `[ ]` 4D.2 `[P]` RED then GREEN: dispatch-window identification. Ranks
  candidate windows by price, regional load, and outage risk and returns the
  forecast grid value with `value_kind=modeled_estimate` (FULL_SPEC §3 "Grid
  and market operations"). Verify: `-run Windows` passes.
- `[ ]` 4D.3 `[after 4A.7]` Connect service `internal/api/context` with
  `GetMarketContext`, `GetWeatherContext`, `GetOutageRisk`,
  `ListDispatchWindows`, registered in `cmd/control` after 4A.7's
  registration lands. Verify: `go test ./services/control/internal/api/context/` passes.

### Lane 4E — event report, comparison, modeled economics

Owns: `services/control/internal/report/`.

- `[ ]` 4E.1 `[P]` RED then GREEN: full event report per FULL_SPEC §5.9:
  requested, approved, commanded, acknowledged, delivered MW and MWh;
  baseline and measurement method; latency, tracking error, availability,
  confidence; reserve violations prevented and exclusions by reason; modeled
  gross value, degradation cost, penalty exposure, net value; data gaps,
  assumptions, provenance, model versions. Financial fields carry
  `value_kind=modeled_estimate`. Verify: `go test ./services/control/internal/report/` passes.
- `[ ]` 4E.2 `[P]` Report is immutable once the event is `REPORTED`; a second
  build returns the stored version. Verify: `-run Immutable` passes.
- `[ ]` 4E.3 `[P]` Event comparison: two reports diffed on every numeric field
  and on plan and policy versions. Verify: `-run Compare` passes.
- `[ ]` 4E.4 `[after 4A.5, 4B.2]` Member rewards and conservative incremental
  margin included in the report (FULL_SPEC §9 step 16). Verify: `-run Rewards` passes.
- `[ ]` 4E.5 `[P]` Partner view of the report: aggregates only, no site rows,
  no travel or away state (FULL_SPEC §11). Verify: `-run PartnerView` passes including a test that the JSON has no `site_id`.
### Lane 4F — flexibility scenarios and fixtures

Owns: `tests/integration/`, `testdata/scenarios/`, `tests/end-to-end/`,
`testdata/fixtures/api/`.

- `[ ]` 4F.1 `[after 4A.7, 4B.6]` Scenario files and `tests/integration`
  cases for TECHSTACK e2e scenarios 12 to 17: `travel-flex-lifecycle`
  (activation, automatic expiry, early-return cancellation),
  `weather-stale-alarm-raise-floor`, `zero-percent-reserve-hardware-floor`,
  `negative-margin-no-dispatch`, `no-fee-market-reward`,
  `anomaly-signal-label`. Each asserts its outcome through the API and the
  stored report. Verify: `go test ./tests/integration/ -run "TravelFlex|RaiseFloor|ZeroPercent|NegativeMargin|NoFee|Anomaly"` passes.
- `[ ]` 4F.2 `[after 4A.7]` Record member fixtures: `GetMemberStatus` in
  each operating state, `SelectResiliencePlan`, `ScheduleTravelFlex`,
  `EndTravelFlexEarly`, and a `HomeActivityAlert`. Verify: `go test ./tools/development/mockapi/ -run Fixtures` passes.
- `[ ]` 4F.3 `[after 4D.3, 4C.2]` Record context and geo fixtures: market,
  weather, outage risk, dispatch windows, H3 cells at each resolution, one
  drill-down path. Verify: fixtures test passes.
- `[ ]` 4F.4 `[after 4E.4]` Record report, comparison, and partner-view
  fixtures. Verify: fixtures test passes and the partner fixture has no `site_id`.
- `[ ]` 4F.5 `[after 4F.1]` Run the full 17-step `demo-path` spec from the UI
  track at the gate and assemble the FULL_SPEC §10 acceptance table with the
  command that proves each bullet. Verify: `playwright test demo-path` all 17 steps green; table saved in the gate report.

### Gate 4

- Every scenario in TECHSTACK "Required end-to-end scenarios" (all 17) passes
  in `tests/integration`, including Travel Flex, weather override, zero-percent
  reserve, negative margin, no-fee market, and anomaly labelling.
- UI track: demo-path steps 1 to 17 green (4F.5).
- FULL_SPEC §10 acceptance list walked item by item in the gate report with the
  command that proves each (4F.5).
- `claude docs/gate-reports/wave-4.md` written.

---

## 7. Wave 5 — integration readiness

Implements FULL_SPEC §12 Phase 5 and TECHSTACK step 10. Phase 6 (authorized
production pilot) is out of scope for this build because it requires
authorized operational data that DATASETS §14 shows does not exist here.

### Lane 5A — connector interfaces and contract tests

Owns: `services/control/internal/connectors/`, `tests/contract/`, `docs/data/`.

- `[ ]` 5A.1 `[P]` Connector interfaces for all eleven rows of the FULL_SPEC
  §8 table: market prices and regional load, weather and alerts, outage risk,
  household load, battery telemetry, grid topology, member policy, resilience
  and Travel Flex behaviour, commands, settlement, pricing and rewards. Each
  has the simulated implementation and a `PENDING-LIVE` slot reported for
  `STUBS.md`. Verify: `go test ./services/control/internal/connectors/` passes.
- `[ ]` 5A.2 `[P]` Contract tests asserting a simulated connector and a
  future live connector produce identical schema output for the same request.
  Verify: `go test ./tests/contract/ -run Connector` passes.
- `[ ]` 5A.3 `[P]` `docs/data/integration-notes.md` listing every field that
  FULL_SPEC §7 says is missing, which connector supplies it, and its
  authorization requirement. Verify: each §7 bullet appears in the table.

### Lane 5B — load tests

Owns: `tests/load/`.

- `[ ]` 5B.1 `[P]` Telemetry ingest load test at 5,000 devices reporting every
  5 seconds for 10 minutes; no dropped observations, sequence gaps reported.
  Verify: `go test ./tests/load/ -run TelemetryIngest -timeout 20m` passes.
- `[ ]` 5B.2 `[P]` Dispatch path load test: 5,000 command intents persisted
  before any network send, p95 read endpoint latency under 500 ms during the
  run (FULL_SPEC §10). Verify: `-run DispatchPath` prints p95 under 500 ms.
- `[ ]` 5B.3 `[after 5B.2]` Results written to `claude docs/gate-reports/load.md`
  with the raw output of both runs. Verify: `grep -E "p95|dropped" "claude docs/gate-reports/load.md"` shows the p95 figure and a zero dropped count.

### Lane 5C — observability

Owns: `infrastructure/observability/`, `infrastructure/local/`,
`services/control/internal/observability/`,
`services/decision/gridos/observability/`,
`services/gateway-simulator/internal/observability/`, every `cmd/` main for
the init call, `services/control/go.mod` (Wave 5 owner).

- `[ ]` 5C.1 `[after 5E.2]` OpenTelemetry traces in Go and Python with
  correlation and workflow IDs on every span, initialised from each `cmd/`
  main; Prometheus metrics for commands by state, acknowledgement latency,
  telemetry freshness, safety rejections, solver time, fallback rate. The
  observability packages are `[P]`; only the `cmd/` edits wait for 5E.2.
  Verify: `curl localhost:9464/metrics | grep gridos_` lists each metric.
- `[ ]` 5C.2 `[P]` Grafana dashboards provisioned in compose for fleet,
  dispatch, and failure recovery. Verify: `curl localhost:3000/api/dashboards/uid/gridos-dispatch` returns 200.
- `[ ]` 5C.3 `[P]` RED then GREEN: log, trace, and analytics scrubbers; a test
  emits a record containing a site ID, command credential, and travel window
  and asserts none reach the exporter (FULL_SPEC §11). Verify: `go test ./services/control/... -run Scrub` passes.
- `[ ]` 5C.4 `[P]` Grafana alert rules for safety rejections, fallback rate,
  uncertain commands, and stale-telemetry share, provisioned with the
  dashboards. Verify: `curl localhost:3000/api/v1/provisioning/alert-rules` lists the four rules.

### Lane 5D — deployment

Owns: `infrastructure/aws/`.

- `[ ]` 5D.1 `[P]` Terraform for ECS services (control, worker, decision,
  gateway simulator for staging), RDS PostgreSQL, Temporal (managed or
  self-hosted behind a variable), ALB, secrets in AWS Secrets Manager.
  Verify: `terraform -chdir=infrastructure/aws init -backend=false && terraform -chdir=infrastructure/aws validate`.
- `[ ]` 5D.2 `[P]` Workload Identity Federation for BigQuery with no stored
  long-lived key (TECHSTACK "Warehouse identity"). Verify: `grep -rn "private_key" infrastructure/aws` prints nothing.
- `[ ]` 5D.3 `[P]` Container images for each service built by Bazel or Docker
  with pinned base images. Verify: `docker build -f services/control/Dockerfile .` succeeds for each service.
- `[ ]` 5D.4 `[P]` `terraform plan` recorded against a mock provider; no
  `apply` in this build. Verify: plan output saved to `claude docs/gate-reports/terraform-plan.md`.
- `[ ]` 5D.5 `[P]` `MODULE.bazel` with Bzlmod (`rules_go`, `gazelle`,
  `rules_python`, `rules_proto`, `rules_buf`) now that real build targets
  exist (TECHSTACK: "added with the first build target"). Go and Python
  service tests run under Bazel. Verify: `bazelisk test //services/...` passes.
- `[ ]` 5D.6 `[after 5D.5]` Console wrapped as a Bazel `sh_test` running
  `pnpm --dir apps/console test`; container images from 5D.3 buildable
  through Bazel targets. Verify: `bazelisk test //apps/console:vitest` and `bazelisk build //services/control:image` succeed.

### Lane 5E — security and privacy

Owns: `docs/operations/security/` (threat model and retention),
`services/control/internal/api/` (all subpackages, for authorization tests),
`tools/development/hooks/`.

- `[ ]` 5E.1 `[P]` Role matrix test: every API handler × every role from
  FULL_SPEC §11, asserting allow or deny. Verify: `go test ./services/control/internal/api/ -run RoleMatrix` passes.
- `[ ]` 5E.2 `[P]` Step-up authorization and immutable audit entry required
  for dispatch approval and emergency stop. Verify: `-run StepUp` passes.
- `[ ]` 5E.3 `[P]` `docs/operations/security/threat-model.md` covering the
  browser, API, workflow, outbox, gateway, and analytics boundaries, and
  `docs/operations/security/retention.md` with retention and deletion rules
  for household data and travel state (FULL_SPEC §11 and §12 Phase 5).
  Verify: both docs exist and each FULL_SPEC §11 bullet maps to a section.
- `[ ]` 5E.4 `[P]` Secrets scan in the pre-push hook as the enforcement of
  FULL_SPEC §11 "never commit operational credentials".
  Verify: `gitleaks detect` passes and a scratch commit containing a fake AWS key is rejected.

### Lane 5F — docs, runbooks, demo

Owns: `README.md`, `docs/operations/` (except `security/`), `AGENTS.md`.

- `[ ]` 5F.1 `[P]` Runbooks in `docs/operations/`: worker restart, gateway
  restart, stuck outbox, uncertain command, emergency stop, replay an event.
  Each runbook's commands executed once and output pasted. Verify: every command block in each runbook runs.
- `[ ]` 5F.2 `[P]` Demo runbook `docs/operations/demo.md` walking FULL_SPEC §9
  steps 1 to 17 with the exact clicks and expected screens. Verify: the
  director follows the runbook against `make demo` and every step's expected
  screen appears; `playwright test demo-path` is green on the same build.
- `[ ]` 5F.3 `[after all lanes]` Docs alignment pass: every command in
  `README.md` and `AGENTS.md` executed; status section updated from
  "specification stage" to the shipped state. Verify: `docs-align` skill run
  green.
- `[ ]` 5F.4 `[after 5F.3]` Final gate report `claude docs/gate-reports/wave-5.md`
  with the full FULL_SPEC §10 acceptance table and STUBS.md reviewed.
  Verify: every bullet under FULL_SPEC §10 "The MVP is complete when it can"
  has a table row naming the command that proves it and its passing output.

### Gate 5

- Load targets met and recorded.
- Terraform validates; no secrets or long-lived keys in the tree.
- Role matrix and scrubber tests green.
- README and runbooks verified by execution.
- `STUBS.md` contains only `PENDING-LIVE` connector entries.

---

## 8. Judgment calls to confirm

1. **Bazel arrives in Wave 5, native tools verify throughout.** TECHSTACK
   names Bazel with Bzlmod but says `MODULE.bazel` is added with the first
   build target. Every item's verify command uses `go test`, `pytest`, and
   `pnpm`; Bazel wraps them in lane 5D once the tree is real. The console is
   not compiled by Bazel.
2. **Fleet generator and data tools are Python.** TECHSTACK leaves `tools/`
   language open. Python was chosen because Polars and the decision service
   already need it.
3. **Local auth mode is a marked stub.** Clerk is the identity provider, but
   the judged path must run offline, so `GRIDOS_AUTH_MODE=local` exists and is
   listed in `STUBS.md`.
4. **Canonical measurement boundary defaults to meter net export** with the
   battery-terminal boundary selectable. The spec leaves this open (§15); the
   choice lives in `ASSUMPTIONS.md` and every report names the boundary used.
5. **Phase 6 is excluded.** It needs authorized data the manifest says does
   not exist.
6. **Layout additions.** TECHSTACK's tree is the target and allows a new
   directory with a clear owner and use. This plan adds, each owned by one
   lane: `services/control/internal/api` (operator and service APIs, split
   into per-domain Connect services), `internal/report` (event accounting,
   FULL_SPEC §5.9), `internal/context` (public market, weather, and outage
   context), `internal/connectors` (Phase 5 connector interfaces),
   `internal/storage/publisher` (the outbox adapter's network side),
   `internal/observability`, `services/decision/gridos/physics` and
   `gridos/economics`, `apps/console/src/member`, `testdata/golden` (shared
   Python and Go golden fixtures, TECHSTACK "Verification strategy"),
   `tests/load`, `.gridos/` (ignored local working data so `data/` stays a
   raw cache), root `Makefile` and `sqlc.yaml`, and `claude docs/` for
   director reports. The director records this list in `ASSUMPTIONS.md` at
   Gate 0.
8. **The console is a separate track.** The user runs their own UI agent on
   `apps/console/` from `UI_TRACK.md`. Six backend lanes stay full because
   lane F became the fixture, mock, ingest, scale, and integration lane.
7. **Site location is a permission, not a role.** FULL_SPEC §11 lists six
   roles and makes exact location "a separately authorized capability", so
   the plan models it as a `site_location` permission that any role may be
   granted separately.

---

## 9. Item count

| Wave | A | B | C | D | E | F | Total |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0 | 5 | 7 | 7 | 6 | 8 | 4 | 37 |
| 1 | 8 | 7 | 9 | 9 | 6 | 5 | 44 |
| 2 | 5 | 7 | 7 | 6 | 6 | 5 | 36 |
| 3 | 6 | 8 | 4 | 5 | 4 | 5 | 32 |
| 4 | 7 | 6 | 4 | 3 | 5 | 5 | 30 |
| 5 | 3 | 3 | 4 | 6 | 4 | 4 | 24 |
| | | | | | | | **203** |

156 items are fully parallel and 47 wait on one other lane. Plus the five
standing items applied every wave. The UI track adds 40 items of its own in
`UI_TRACK.md`.
