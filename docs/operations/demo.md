# Severe-weather demo

The canonical demo uses the simulated Austin fleet and the
`heat-event-canonical` scenario. The scenario clock starts with the first
accepted nonzero command for the event. Its offline-device and delayed-gateway
faults are scheduled within the following ten minutes. The gateway publishes
ordinary physical telemetry while the presenter prepares the event.

Start an isolated demo with unused ports and its own database and directory.
The director runs this command for acceptance; do not run it against a standing
demo:

```sh
GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml make demo
```

Open the console URL printed by the demo process. Use the local demo identity.
Keep the event identifier visible as you move through **Plan & approval**,
**Execution**, and **Report**. The fleet and scenario are marked **SIMULATED**;
the presentation must not describe modeled values as observed market or
household data.

1. **Observe context.** Open **Observe** or `/fleet`. The Austin fleet view
   shows installed power, usable energy, reserved backup, dispatchable power,
   operating state, and provenance with freshness. The regional context panel
   must show forecast load, prices, weather, and outage risk. Regional context
   presentation is pending the console demo-path hard assertion.
2. **Choose the commitment.** Select the **operator** demo role, then click
   **Plan a dispatch**. Choose **Greater Austin · LZ_AEN**, enter a UTC start a
   few minutes ahead and an end at least ten minutes later, set **Target power
   (MW)** to `0.001`, and choose **Meter net export**. Click **Create dispatch
   plan**. The **Review the plan** page opens under a new event ID.
3. **Inspect the frozen version.** On **Plan & approval**, check **Plan v1** and
   the event ID. Wait for **Safety validated**. The frozen input snapshot IDs
   and versions must be visible with the plan; this screen is pending the
   console demo-path hard assertion.
4. **Inspect the forecast.** Open **Forecast intervals** on the plan view.
   Consumption, risk, and available capacity must be shown by interval with
   their source and freshness. This screen is pending the console demo-path
   hard assertion.
5. **Inspect the optimized plan.** Open **Optimization explanation**. Check
   proposed power against held household reserve and target shortfall. This
   screen is pending the console demo-path hard assertion.
6. **Review exclusions and value.** On **Plan & approval**, inspect
   **Exclusions by reason**, then **Constraint margins** and the expected-value
   and reserve breakdown. Exclusions are available; the explanation panels
   are pending the console demo-path hard assertion.
7. **Show Travel Flex and weather reserve.** Open **Travel Flex eligibility**
   and the weather risk detail. A consented scheduled window should show its
   extra eligible capacity and stated credit; weather risk should show the
   raised effective reserve. This screen is pending the console demo-path
   hard assertion.
8. **Prove the safety gate.** Click **Validate unsafe alternative** and inspect
   the rejection reason, then return to the validated plan. The unsafe-plan
   control is pending the console demo-path hard assertion.
9. **Approve and launch.** Switch to the **approver** demo role. Click
   **Review approval**, type the displayed plan version in the confirmation
   dialog, and click **Confirm approval**. Check **Approved**. Click **Review
   launch**, enter the same plan version, and click **Confirm launch**. Approval
   and launch produce separate records; launch moves the event toward **Sent**.
10. **Inspect command fan-out.** Click **Execution**. Open **Command fan-out**
    and distinguish persisted commands, sent attempts, receipts, and uncertain
    outcomes. This panel is pending the console demo-path hard assertion.
11. **Observe seeded faults.** Keep **Execution** open through the live scenario
    window. **Scenario failures** must show offline devices and delayed gateway
    evidence from `WatchEvent`. The gateway runtime now schedules both faults;
    the director's isolated demo verification and console hard assertion are
    pending.
12. **Observe recovery.** Open **Recovery decisions**. Verify bounded retries,
    stale-capacity removal, and any replacement plan against the approved
    envelope. This panel and its hard assertion are pending.
13. **Separate intent, receipt, and delivery.** In **Measured event response**,
    compare **Sent intent**, **Acknowledged receipt**, and **Measured delivery**.
    Unknown delivery remains unknown. The three quantities are present; the
    full scenario hard assertion is pending.
14. **Check the protected floor.** Open **Reserve protection evidence**. Verify
    the effective household reserve and Grid Flex floor while the response is
    tracked against the request. This panel and its hard assertion are pending.
15. **Observe safe return.** Wait for explicit event expiry, then open **Safe
    return evidence**. Confirm commands return to zero without lowering
    protected reserve. This panel and its hard assertion are pending.
16. **Review the report.** Click **Report**. The **Event evidence** screen must
    distinguish requested, approved, commanded, acknowledged, delivered, and
    shortfall. Inspect latency, reserve protection, rewards, conservative
    incremental margin, modeled economics, provenance, and assumptions. Basic
    accounting is available; the full report screen and assertion are pending.
17. **Replay.** Click **Replay event** from the report. Compare the replay
    manifest, timeline, and plan difference with the original event ID and
    frozen input versions. The replay control and assertion are pending.

The acceptance run ends only when the director observes every expected screen
on the scenario-backed demo and `playwright test demo-path` passes with hard
assertions on the same build. A soft assertion or a visible placeholder does
not complete a step.
