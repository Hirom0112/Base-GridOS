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

