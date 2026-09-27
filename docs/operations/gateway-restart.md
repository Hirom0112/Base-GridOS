# Gateway restart

Use this when the gateway process exits or telemetry and command acknowledgements stop. The gateway's SQLite file is durable state; restart with the same database path, gateway ID, fleet file, control address, and authorization setting. Never delete or replace the SQLite file as a restart step.

1. Stop only the gateway through its process supervisor and restart the same build and configuration. Check that it resumes its telemetry cadence and drains buffered observations.
2. Confirm duplicate command delivery receives the existing acknowledgement. Commands that expired while the gateway was down remain expired; a newer generation cannot be overwritten by an older command.
3. Review source-time progression. Repeated wall-clock slots are skipped, while a missed slot is represented as missing telemetry before the current physical sample. A per-device producer rejection is counted and logged without ending the fleet loop.

The isolated restart rehearsal keeps the event and audit trail intact:

```sh
go test ./tests/integration -run '^TestGatewayRestart$' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/tests/integration 110.617s`.
