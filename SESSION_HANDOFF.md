# Session handoff

Updated 2026-09-27 at about 15:08Z by the director session (Claude).

## Roles

- **Director (this role).** Reads the plan, dispatches, independently
  re-runs every verify command, checks that RED and GREEN are separate
  commits, marks items `[x]` in `claude docs/BUILD_ORDER.md`, logs every
  decision in `claude docs/QUESTIONS_AND_DECISIONS.md`, owns the standing
  demo and all gate runs. It does not write feature code; this session
  dispatched its own subagents because the Codex worker went silent.
- **Console agent (`ui:` lines).** Owns `apps/console/`. A separate visual
  overhaul session holds uncommitted edits under `apps/console/src`; do not
  stage them.
- **Channel.** `.local/mailbox.log`, append only.

## Where things stand

251 of 254 backend plan items are done. Closed this session with live
evidence: 2A.13 (all five fault kinds on c94b11df), 4E.10 (live report
fixture 1f19b53 from fixture-4e10-1790520318), 5D.8 (locks 256). The final
recorded UI gate is green: vitest 188, Playwright 80.

Open: **3F.5, 4F.5, 5F.2**, which close together on one green integrated
run of `demo-path.spec.ts`. The last run (event 4b222277) passed steps 01
to 12 live, including the fault steps, and failed step 13: the measured
delivery value stays "Delivery unknown" although DELIVERY_VERIFIED rows
carry 18.1 MW. That stream defect was being root-caused at writing.

Defects found and fixed by the integrated runs, each RED/GREEN and
verified: weather provenance panic (f8ee28b/3e5ef01), WEATHER override
slices with gaps (3e9f54f/1aa4a8e), non-WEATHER override continuity
(93beec8/9f525cf), reconciliation partial interval end (d05949f/694787a),
ListEventCommands 5 s deadline from missing indexes (7a90689/f5fbfa9,
migration 0021), and two demo-path spec defects (baed119, 46fc98c).

## Running the integrated proof

Rebuild on HEAD (re-arms the gateway), wait until telemetry lag is under
15 s (planning earlier excludes every device as STALE_TELEMETRY), then:
`PLAYWRIGHT_LIST_PRINT_STEPS=1 pnpm --dir apps/console playwright test demo-path.spec.ts --workers 1 --reporter list`.
It must be the first launch on the fresh gateway and takes about 13
minutes because it waits out its own event window.

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

## Commands the next director needs

Mailbox watch (run as a Monitor, re-arm on its 30-minute expiry):

```sh
f=.local/mailbox.log; seen=$(wc -l < "$f" | tr -d ' ')
while true; do now=$(wc -l < "$f" | tr -d ' ')
  if [ "$now" -gt "$seen" ]; then sed -n "$((seen+1)),${now}p" "$f" | grep -E '^(worker|ui):'; seen=$now
  elif [ "$now" -lt "$seen" ]; then seen=$now; fi; sleep 2; done
```

Launch a proof event on the standing demo (create, approve with a signed
step-up assertion, launch; the window starts 30 s after creation):

```sh
C=http://127.0.0.1:28080; E="live-proof-$(date -u +%s)"; NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
BEGIN=$(date -u -v+30S +%Y-%m-%dT%H:%M:%SZ); END=$(date -u -v+62M +%Y-%m-%dT%H:%M:%SZ)
curl -fsS -X POST -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' \
  --data "{\"eventRequest\":{\"requestId\":\"$E\",\"eventType\":\"GRID_SERVICE\",\"beginTime\":\"$BEGIN\",\"endTime\":\"$END\",\"targetKw\":20000,\"measurementBoundary\":\"MEASUREMENT_BOUNDARY_METER_NET_EXPORT\",\"loadZones\":[\"LZ_AEN\"],\"correlationId\":\"$E\"},\"idempotencyKey\":\"create-$E\"}" \
  $C/gridos.v1.DispatchService/CreateEventRequest
# wait until GetEvent reports DISPATCH_EVENT_STATE_VALIDATED, then:
A=$(curl -fsS -X POST -H 'Content-Type: application/json' -H 'X-GridOS-Role: approver' \
  --data "{\"action\":\"APPROVE_EVENT\",\"event_id\":\"$E\",\"plan_version\":1}" http://127.0.0.1:8080/local/step-up | jq -r .assertion)
curl -fsS -X POST -H 'Content-Type: application/json' -H 'X-GridOS-Role: approver' -H "X-GridOS-Step-Up: $A" \
  --data "{\"eventId\":\"$E\",\"planVersion\":1,\"idempotencyKey\":\"approve-$E\",\"approvedBy\":\"director\",\"approvedAt\":\"$NOW\"}" \
  $C/gridos.v1.DispatchService/ApproveEvent
curl -fsS -X POST -H 'Content-Type: application/json' -H 'X-GridOS-Role: approver' \
  --data "{\"eventId\":\"$E\",\"planVersion\":1,\"idempotencyKey\":\"launch-$E\",\"requestedBy\":\"director\",\"requestedAt\":\"$NOW\"}" \
  $C/gridos.v1.DispatchService/LaunchEvent
```

Verify the five exception kinds for an event (expect MISSING_TELEMETRY,
UNCERTAIN_COMMAND, COMMAND_RETRY, STALE_CAPACITY_REMOVED and
REBALANCED_COMMAND within about fifteen minutes of the window start):

```sh
curl -s -X POST -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' \
  --data "{\"eventId\":\"$E\"}" http://127.0.0.1:28080/gridos.v1.EventsService/GetEventTimeline \
  | jq -r '[.exceptions[]?.kind] | unique | join(",")'
```

If a kind is missing, look at `command_outbox` attempts and states for the
event's intents and at `ERROR` lines in `.local/demo/worker.log`; both
earlier gaps were found that way.

## Standing working agreements

- The user's standing instruction is to keep going without stopping and
  without asking unless something is truly the user's call.
- Many small commits, by exact path, subjects under 72 characters with no
  prefix, RED and GREEN as separate commits. No worktrees, no `git add -A`,
  no reset, stash or checkout in the shared tree, never `--no-verify`.
- Only fast checks run in the hook; the full suite runs only at gates, by
  the director.
- Timestamps in the decisions log are real UTC from `date -u`.

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
