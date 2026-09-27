# GridOS handoff

Written 2026-09-27 at 15:20Z, the single handoff for this repo. It
replaces SESSION_HANDOFF.md and ASTRA_HANDOFF.md.

## Where things stand

Plan and status live in `claude docs/BUILD_ORDER.md`, every decision with
evidence in `claude docs/QUESTIONS_AND_DECISIONS.md`, and operator
runbooks in `docs/operations/` (demo walkthrough: `docs/operations/demo.md`;
Loom script: `VIDEO_SCRIPT.md`). Git history on GitHub was rewritten on
2026-09-27 to remove co-author trailers; older SHAs quoted in the
decisions log and mailbox refer to the pre-rewrite history (commit
subjects are unchanged, so search by subject).

251 of 254 backend plan items are done. Open: 3F.5, 4F.5 and 5F.2, which
close together on one green run of the integrated spec
`demo-path.spec.ts`.

## What this session fixed (all RED then GREEN, focused tests re-run)

- Integrated demo-path run defects: weather provenance panic, WEATHER
  override slices with gaps, risk override continuity, reconciliation
  partial interval end, ListEventCommands indexes (migration 0021),
  WatchEvent deadline surfacing as internal, approval and launch audit on
  the server clock, reserve compliance measured over dispatched devices,
  exact-name locators and a 60 s launch wait in the spec.
- Audit P0s: migrate down now rolls back (single `database/rollback/`),
  integration tests fail instead of skipping when the stack is down
  (`make test-load` holds the load suite), gateway rejects a reused
  idempotency key as CONFLICTING_REUSE, telemetry publish has a 10 s
  timeout, malformed audit rows surface as errors.
- Audit Tier 0: PII scrubber catches underscore IDs, fixture regeneration
  keeps LZ_AEN, real-time prices skip energy-weighted zone rows, the stale
  rapid counterexample was removed, build debris is ignored.
- 2A.13, 4E.10 and 5D.8 closed with live evidence. The 4E.10 report
  fixture is `testdata/fixtures/api/ReportService/GetEventReport.live.json`.

## Open, in priority order

1. **Household reserve breach (safety).** Under METER_NET_EXPORT the
   optimizer ignored home load (`services/decision/gridos/server.py:92`
   set `home_load_kw=0.0`), so about 150 dispatched devices ended 0.29 kWh
   below their protected reserve on live events. Fixed in the decision service (d463c21e RED, ba8c08f2 GREEN: frozen home
   load is budgeted into export dispatch; 19 decision tests pass), but not
   yet proven live. Follow-up: the Go safety energy balance still treats
   the setpoint as total discharge with no home-load term, so the
   independent check cannot catch this class of breach, and
   expected_energy_kwh is reported as export-only drain to stay
   consistent with it. Add a home-load term to the Go safety model, then
   restore the true value. A site without a covering load forecast is now
   excluded as UNAVAILABLE rather than treated as zero load.
   Demo-path step 14 stays red until this lands on a rebuilt demo.
2. **Integrated 17-step run (3F.5, 4F.5, 5F.2).** Steps 01 to 13 pass
   live; 14 needs item 1; 15 and 16 pass; 17 needed the server-clock audit
   fix (landed). The standing demo still runs the pre-fix binary
   because the user was recording the Loom on it. Rebuild on HEAD, wait for telemetry lag under 15 s, run
   the spec as the first launch.
3. **Audit P0 leftovers.** Typed CommandState to remove five `_ =`
   RecordCommand drops (plan in the decisions log); `requireDemo` in
   `tests/end-to-end/vertical_slice_test.go` should fail when
   GRIDOS_CONTROL_URL is set and unreachable (not committed because the
   hook runs that package against a live stack); the pre-commit hook runs
   `tests/end-to-end` with `-short` against any listening stack.
4. **Audit tiers 1 to 6** (`.local/audit-tiers.md`, local only): safety
   gate checks that compare values to themselves (`dispatcher.go:204`),
   unset maintenance lock, ramp and grid limits, no temperature check,
   SOC absent becomes 0.0, Travel Flex headroom never released because
   ConservativeMargin and MarginHurdle are never set, fleet data is random
   rather than grounded, no county so outage risk is always unavailable,
   solar and voltage never simulated, DATASETS.md drift, P1 dead code
   (control analytics and connectors, decision engines without callers).
5. **Price data gap.** The committed ERCOT fixtures cover 2025-01-01 to
   01-07 only, so live events have no LZ_AEN price and margin is reported
   unavailable by design.
6. The demo has two stray VALIDATED events created by an end-to-end test
   run at about 15:18Z; they were never approved or launched.


## Rules that matter

- Commit exact paths, RED and GREEN as separate commits, no `--no-verify`,
  no `git add -A`, no reset, stash or checkout in the shared tree.
- Never add Co-Authored-By or "Generated with" lines to commits or PRs.
- Rebuild before any live proof; the gateway arms its faults once per
  process, so a proof event must be the first launch after the rebuild.
- Wait for telemetry lag under 15 s before planning, or every device is
  excluded as STALE_TELEMETRY.
- Reset the gateway store and the control database together or not at all.

## Standing demo

Ports: control 28080, decision 25061, gateway 28081, console 3000, mock
identity signer 8080, metrics 9464 to 9467, Grafana 33000 under the
`observability` profile. It runs in live scenario mode with step-up enforced.

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

