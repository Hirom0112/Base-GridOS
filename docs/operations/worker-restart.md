# Worker restart

Use this when the Temporal worker stops or stops polling the configured task queue. Keep the control API and gateway running. Record the event ID, task queue, last command state, and correlation ID before intervening.

1. Stop only the worker through its process supervisor. Start the same worker build with the same `GRIDOS_TASK_QUEUE`, database, gateway, decision, and Temporal settings. A deployment must retain its persistent PostgreSQL and Temporal state across this restart.
2. Confirm the worker polls the queue again and the event continues from its durable workflow history. A restarted worker must not create a second command intent for the same event and generation.
3. If the event remains stalled, inspect the outbox and uncertain-command runbooks. Do not mark an unacknowledged command delivered or relax reserve to force completion.

The isolated restart rehearsal terminates and restarts the worker during a live event:

```sh
go test ./tests/integration -run '^TestWorkerTermination$' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/tests/integration 111.437s`.
