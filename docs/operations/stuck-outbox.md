# Stuck outbox

Use this when an approved event has durable command intents but no new gateway receipt. Check the worker, gateway, and database first. `PERSISTED` means the intent exists; `SENT` means a delivery attempt occurred; neither proves physical delivery.

1. Confirm the worker is polling, the gateway is reachable, and the outbox lease has expired before expecting another claim. A process crash leaves the row for a later claim.
2. Restart the worker only if it is stopped. Allow the publisher to reclaim and resend the same command ID and idempotency key. Never insert a replacement row or hand-edit an outbox lease to accelerate a retry.
3. If the gateway outcome is unknown, follow the uncertain-command runbook and preserve the feasible power interval until reconciliation.

The isolated storage check exercises a crash and lease reclaim:

```sh
go test ./services/control/internal/storage -run '^TestOutboxCrashReclaimPreservesCommand$' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/services/control/internal/storage 0.814s`.
