# Stuck outbox

Use this when an approved event has durable command intents but no new gateway
receipt. `PERSISTED` means an intent exists; `SENT` means a delivery attempt
occurred. Neither proves physical delivery.

**Confirm.** Inspect each outbox row beside the latest append-only command
state. A `PUBLISHING` row with an `ACKNOWLEDGED` command is not an unresolved
send, even if the outbox row has not advanced:

```sh
PGOPTIONS='-c statement_timeout=10s' psql 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable' -P pager=off -c "SELECT o.command_id, o.state AS outbox_state, o.attempts, s.state AS command_state FROM command_outbox o JOIN LATERAL (SELECT state FROM command_states WHERE command_id=o.command_id ORDER BY recorded_at DESC LIMIT 1) s ON true ORDER BY o.next_attempt_at DESC LIMIT 5"
```

Observed output on the standing demo after the stop rehearsal: five
`PUBLISHED` rows, including two `REJECTED` command states. A published row
does not imply acceptance. Check the worker log and gateway availability
before acting.

**Act.** If the worker has stopped, follow [Worker restart](worker-restart.md)
with the same task queue and database. Let the publisher reclaim the due row
and resend the same command ID and idempotency key. Never insert a replacement
row or hand-edit a lease. If the latest state is `UNCERTAIN`, follow
[Uncertain command](uncertain-command.md) before sending an opposite setpoint.

**Recover.** Re-run the query and verify that the latest command state has
advanced to `ACKNOWLEDGED`, `EXECUTING`, or a terminal state, with a receipt
record for accepted commands. A `PUBLISHED` outbox row alone is insufficient.

The isolated crash and lease-reclaim proof remains:
`go test ./services/control/internal/storage -run '^TestOutboxCrashReclaimPreservesCommand$' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/services/control/internal/storage 0.814s`.
