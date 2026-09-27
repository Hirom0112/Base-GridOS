# Uncertain command

Use this when a send timed out or the gateway receipt is missing. `UNCERTAIN`
means the command may have executed. Do not treat it as rejected or send an
opposite setpoint until reconciliation establishes a safe interval.

**Confirm.** Compare the durable latest states with the process metric:

```sh
PGOPTIONS='-c statement_timeout=10s' psql 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable' -Atqc "SELECT COUNT(*) FROM (SELECT DISTINCT ON (command_id) state FROM command_states ORDER BY command_id, recorded_at DESC) latest WHERE state='UNCERTAIN'"
curl -fsS --max-time 5 http://127.0.0.1:9464/metrics | rg '^gridos_uncertain_commands'
```

Observed output on the standing demo: `0`, then
`gridos_uncertain_commands 0`. For a nonzero result, inspect the command ID,
acknowledgement deadline, and its `uncertainty_intervals` lower and upper kW
bounds before acting.

**Act.** Check [Gateway restart](gateway-restart.md) if the gateway is down;
otherwise allow its durable acknowledgement to arrive or the reconciliation
workflow to bound the unresolved interval. A late `ACCEPTED` receipt resolves
the same command ID. Do not create a new generation to force a response.

**Recover.** Re-run both checks and confirm the latest state is no longer
`UNCERTAIN`. Check the appended acknowledgement or reconciliation decision
and the physical telemetry before calling the command delivered. If the
interval remains unresolved beyond the configured window, escalate with the
event ID, command ID, and feasible bounds; keep the reserve and audit history
intact.

The isolated timeout and late-acceptance proof remains:
`go test ./services/control/internal/storage -run 'TestAckDeadlineMarksUncertainWithInterval|TestLateAcceptedAcknowledgementResolvesUncertainCommand' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/services/control/internal/storage 0.836s`.
