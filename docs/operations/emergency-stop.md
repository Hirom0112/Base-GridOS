# Emergency stop

Use this when an active dispatch must end before expiry. The request requires
an operator or approver and an idempotency key. A requested stop is not a
confirmed gateway action.

**Confirm.** Record the event ID and verify commands have been sent. This
rehearsal created and launched a fresh `runbook-stop-*` event on the standing
demo; do not stop another operator's event:

```sh
curl -fsS --max-time 10 -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' --data '{"eventId":"runbook-stop-20260927T043935Z"}' http://127.0.0.1:28080/gridos.v1.DispatchService/GetEvent | jq -r '.event.state'
```

Observed output before the stop: `DISPATCH_EVENT_STATE_ACKNOWLEDGED_OR_UNCERTAIN`.

**Act.** Keep the same idempotency key and request body on every retry.
Replace the event ID only after checking the event and its operator. This
local demo currently has no step-up key set; its request was accepted without
an assertion:

```sh
event_id=runbook-stop-20260927T043935Z
requested_at=2026-09-27T04:41:35.607690Z
body=$(jq -nc --arg id "$event_id" --arg at "$requested_at" '{eventId:$id,idempotencyKey:("stop-"+$id),requestedBy:"runbook-operator",reason:"Operator runbook rehearsal",requestedAt:$at,correlationId:$id}')
curl -fsS --max-time 20 -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' --data "$body" http://127.0.0.1:28080/gridos.v1.EventsService/EmergencyStop | jq -c '{stopRequested, emergencyStopId:.emergencyStop.emergencyStopId}'
```

Observed output: `{"stopRequested":true,"emergencyStopId":"runbook-stop-20260927T043935Z:stop-runbook-stop-20260927T043935Z"}`.
When step-up is configured, provide a fresh `EMERGENCY_STOP` assertion bound to
this event and add
`-H "X-GridOS-Step-Up: $STEP_UP_ASSERTION"` to the RPC. An absent or stale
assertion is rejected before the stop is recorded.

**Recover.** Confirm the audit request and zero-setpoint intent, then watch
the event and gateway receipts until the physical response is safe:

```sh
psql 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable' -P pager=off -c "SET statement_timeout = '10s'; SELECT stop.event_id, stop.reason, COUNT(intent.command_id) FILTER (WHERE intent.setpoint_kw = 0) AS zero_setpoint_intents FROM emergency_stops stop LEFT JOIN command_intents intent ON intent.event_id=stop.event_id WHERE stop.event_id='runbook-stop-20260927T043935Z' GROUP BY stop.event_id, stop.reason"
```

Observed output: one stop with reason `Operator runbook rehearsal` and one
zero-setpoint intent. The audit journal also contained
`EMERGENCY_STOP_REQUESTED`. Inspect the latest state and gateway receipt for
every command before calling the stop complete:

```sh
PGOPTIONS='-c statement_timeout=10s' psql 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable' -P pager=off -c "SELECT i.command_id, i.generation, i.setpoint_kw, s.state AS latest_state, a.receipt_status, a.rejection_reason FROM command_intents i JOIN LATERAL (SELECT state FROM command_states WHERE command_id=i.command_id ORDER BY recorded_at DESC LIMIT 1) s ON true LEFT JOIN command_acknowledgements a ON a.command_id=i.command_id WHERE i.event_id='runbook-stop-20260927T043935Z' ORDER BY i.generation"
```

Observed output: the nonzero generation-5 command and zero-setpoint
generation-6 command were both `ACKNOWLEDGED` with `ACCEPTED` receipts. The
stop had one zero intent, one accepted receipt, no expired state, and no
pending outbox row 11.1 seconds after the request. Gateway SQLite retained
generation 6 with setpoint 0. Fresh telemetry at 04:42:30Z reported
`fromStorageKw=1.22`, `toHomeKw=1.22`, and `fromGridKw=0`: net export at the
meter boundary was zero. The director separately verified 166 accepted
zero commands out of 166 in the full-fleet stop rehearsal. If the gateway
is unavailable, use the gateway and uncertain-command runbooks; retain the
reserve floor. Require accepted zero-setpoint receipts and fresh physical
telemetry before declaring a stop complete.

The isolated API test proves the response does not falsely claim confirmation:
`go test ./services/control/internal/api/events -run '^TestEmergencyStopReportsRequestedWithoutConfirmation$' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/services/control/internal/api/events 0.382s`.
