# Session handoff

Written 2026-09-27 at about 10:25Z by the director session (Claude) at the
user's request. Commit at writing: `c4595c1`, 1,480 commits on `main`.

## Roles

- **Director (this role).** Reads the plan, dispatches, independently
  re-runs every verify command, checks that RED and GREEN are separate
  commits, marks items `[x]` in `claude docs/BUILD_ORDER.md`, logs every
  decision in `claude docs/QUESTIONS_AND_DECISIONS.md`, owns the standing
  demo and all gate runs, and writes `claude docs/gate-reports/`. It does
  not write feature code.
- **Codex orchestrator ("worker", `worker:` lines).** The only backend
  implementer. Runs up to three subagents. Its root agent currently owns
  the remaining backend items.
- **Console agent (`ui:` lines).** Owns `apps/console/`.
- **Channel.** `.local/mailbox.log`, append only. The director writes
  `director:` lines. Watch it with a Monitor that prints new `worker:` and
  `ui:` lines; it expires every 30 minutes and must be re-armed.

## Where things stand

Gate 5 backend closed with `claude docs/gate-reports/wave-5.md`. Since
then the console track's evidence needs opened a series of backend items,
all verified except the ones below.

| Wave | Done | Open |
| --- | --- | --- |
| 0 | 39 | 0 |
| 1 | 50 | 0 |
| 2 | 53 | 1 |
| 3 | 33 | 1 |
| 4 | 46 | 2 |
| 5 | 26 | 2 |

Open items:

- **2A.13** `[~]` Live faults persist. Code verified, including a
  director isolated canonical pass. Closes when one launched event on the
  standing demo shows MISSING_TELEMETRY, UNCERTAIN_COMMAND, COMMAND_RETRY,
  STALE_CAPACITY_REMOVED and REBALANCED_COMMAND. The last demo proof showed
  four of them; UNCERTAIN was blocked by the defect fixed in 2B.11.
- **4E.10** Shortfall in the report. API verified (16ad5bf, 7383f53,
  68aba6a, correction 97d05ab and 6b937e2). The mockapi package carries an
  intentional RED (4643a56) until root records the report fixture from a
  completed demo event.
- **5D.8** `[~]` Compose sets `max_locks_per_transaction=256` (8fdd302).
  Closes with the live `show max_locks_per_transaction` line at the next
  `make up`, which must not happen under the standing demo.
- **3F.5, 4F.5, 5F.2** The console's integrated 17-step demo-path run on
  the standing demo. They close together.

## What is in flight right now

1. **Weather context crash.** The console's integrated run failed at step
   1 before any launch: `GetWeatherContext` for Austin drops the
   connection because `services/control/internal/api/context/convert.go`
   `contextSource` panics on `DATA_PROVENANCE_SIMULATED`, which the 5F.5
   demo alert introduced. Dispatched to root as the top item: RED on the
   scenario weather RPC, GREEN with an explicit SIMULATED mapping and an
   error instead of the default panic.
2. **After that fix lands:** rebuild the demo (this re-arms the gateway),
   post `DEMO READY (ARMED)`, and let the console run
   `pnpm --dir apps/console playwright test demo-path.spec.ts --workers 1`.
   That run must be the first launch on the fresh gateway, because the
   gateway arms its live faults once per process. Then verify the five
   exception kinds from the durable rows of that event, close 2A.13 and the
   three demo-path items, and open a short window for root to record the
   4E.10 report fixture from the completed event.

## Standing demo

Ports: control 28080, decision 25061, gateway 28081, console 3000, mock
identity signer 8080, metrics 9464 to 9467, Grafana 33000 under the
`observability` profile. It runs on `53dd8be` in live scenario mode with
step-up enforced.

Rebuild command (the step-up key lives in the ignored
`.local/demo/step-up.key`; `make demo` now starts the signer itself when
the key is set):

```sh
KEY=$(cat .local/demo/step-up.key)
nohup env GRIDOS_STEP_UP_KEY="$KEY" GRIDOS_CODE_VERSION=$(git rev-parse --short HEAD) \
  GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml \
  GRIDOS_CONTROL_METRICS_ADDRESS=127.0.0.1:9464 GRIDOS_WORKER_METRICS_ADDRESS=127.0.0.1:9465 \
  GRIDOS_GATEWAY_METRICS_ADDRESS=127.0.0.1:9466 GRIDOS_DECISION_METRICS_ADDRESS=127.0.0.1:9467 \
  make demo > .local/demo-launch.log 2>&1 &
```

Readiness checks after every rebuild: all listeners up, telemetry lag at
most one 15 s cadence, `gridos_gateway_publish_failures_total 0`,
`gridos_gateway_buffered_rows 0`, EmergencyStop without an assertion
returns `permission_denied`, the signer answers `POST /local/step-up`, two
active WEATHER overrides, and no `ERROR` lines in `.local/demo/worker.log`.

Seeded data currently on the demo: VALIDATED event
`event-4c10-1790504164` (not launched), Travel Flex window
`seed-4c10-1790504134-window` active through 13:15Z, two WEATHER overrides
from the simulated Austin alert.

## Rules learned the hard way

- Reset the gateway store and the control database together or not at
  all.
- Sweep orphan processes by PID or port, never by a command substring; a
  substring sweep once killed the demo's own launcher.
- Recorded browser gates run in a declared window with no `make generate`;
  binding regeneration reloads the Vite dev server mid-test.
- Seed demo fixtures only through public APIs and the live risk bridge,
  never by database inserts.
- Heavy runs (isolated stacks, Bazel, load tests) one at a time; the
  director declares gate windows.
- Every rebuild re-arms the gateway's live faults; any proof event must be
  the first launch after it.

## Where to read more

- Plan and status: `claude docs/BUILD_ORDER.md`.
- Every decision with evidence: `claude docs/QUESTIONS_AND_DECISIONS.md`.
- Gate reports: `claude docs/gate-reports/wave-2.md` through `wave-5.md`.
- Operator runbooks: `docs/operations/`.
- Mailbox history: `.local/mailbox.log`.
