# Uncertain command

Use this when a send timed out or the gateway receipt is missing. `UNCERTAIN` is a real state: the command may have executed. Do not treat it as rejected or send an opposite setpoint until reconciliation establishes a safe interval.

1. Read the event's command ID, last state, acknowledgement deadline, and feasible power interval. The `gridos_uncertain_commands` gauge counts commands whose latest persisted state is `UNCERTAIN`.
2. Let the gateway resend its durable acknowledgement or let reconciliation bound the unresolved interval. A late `ACCEPTED` acknowledgement resolves the same command; it does not create a new one.
3. Escalate only if the interval remains unresolved after the configured reconciliation window. Keep the household reserve and event audit history intact.

The isolated checks cover the timeout transition and late acceptance:

```sh
go test ./services/control/internal/storage -run 'TestAckDeadlineMarksUncertainWithInterval|TestLateAcceptedAcknowledgementResolvesUncertainCommand' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/services/control/internal/storage 0.836s`.
