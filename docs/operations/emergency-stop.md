# Emergency stop

Use this when the operator must end an active dispatch before its scheduled expiry. The command requires an authorized operator or approver and, when step-up is enabled, a fresh identity-provider assertion. The request is audited with an idempotency key.

1. Open the event's execution view, choose **Emergency stop**, review the event ID and reason, then confirm. Do not repeat the action with a new key merely because the UI has not yet received an acknowledgement.
2. Watch for the stop request and zero-setpoint intents for the current generation. A requested stop is not a confirmed gateway action; keep the event under observation until acknowledgement and physical response are visible.
3. If the gateway is unavailable, apply the gateway and uncertain-command runbooks. Preserve the existing command generation and reserve floor.

The isolated API check proves that a stop is reported as requested rather than falsely confirmed:

```sh
go test ./services/control/internal/api/events -run '^TestEmergencyStopReportsRequestedWithoutConfirmation$' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/services/control/internal/api/events 0.382s`.
