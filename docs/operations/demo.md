# Severe-weather demo

The canonical demo uses the simulated Austin fleet and the
`heat-event-canonical` scenario. The scenario clock starts with the first
accepted nonzero command for the event; its offline-device and
delayed-gateway faults fire within the following ten minutes. Every fleet,
telemetry, and economic value is **SIMULATED** or modeled; never present
it as observed market or household data.

## Start

The director starts the demo. Approval and launch require a step-up
assertion, so `GRIDOS_STEP_UP_KEY` must be set; `make demo` then also
starts the local identity signer on `127.0.0.1:8080`. The step-up key lives
in the ignored `.local/demo/step-up.key`.

```sh
GRIDOS_STEP_UP_KEY="$(cat .local/demo/step-up.key)" \
  GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml make demo
```

Wait for `demo ready: control=:28080 decision=:25061 gateway=:28081`, then
open the console at `http://127.0.0.1:3000/fleet`. Before planning, wait
until telemetry lag is under 15 s (one gateway cadence); planning earlier
excludes every device as `STALE_TELEMETRY`. The event must be the first
launch on a freshly started gateway so the seeded faults fire. The WEATHER
reserve overrides from the simulated Austin alert stay active until the
alert expires.

`make test-go` also runs the five host-only Go checks omitted from the Bazel
sandbox run: analytics isolation, gateway restart, command publisher,
API lifecycle, and replacement server. These checks need the host workspace
or process behavior and remain part of the Go gate.

## Steps

Keep the event ID visible; the **Plan & approval**, **Execution**, and
**Report** links under it switch views. The automated walk of the same
steps is `PLAYWRIGHT_LIST_PRINT_STEPS=1 pnpm --dir apps/console playwright
test demo-path.spec.ts --workers 1 --reporter list` (about 13 minutes,
because it waits out its own event window).

1. **Command center.** On `/fleet`, leave **Demo role** on `operator`. In
   **Austin conditions**, expand **Inspect Austin markets · LZ_AEN and
   SOUTH_C**: **Day-ahead reference prices** lists `LZ_AEN` and **Reference
   system load** lists `SOUTH_C`. Expand **Inspect weather and historical
   outage evidence**: **Austin weather forecasts** has rows and **Travis
   County outage history** appears. The fleet panel shows **Availability**.
2. **Select region, window, target.** Click **Plan a dispatch**. Set
   **Operating region** to **Greater Austin · LZ_AEN**, **Start time** to the
   next UTC minute at least one minute ahead, **End time** ten minutes later,
   **Target power (MW)** to `20`, and **Measurement boundary** to **Meter net
   export**. Click **Create dispatch plan**. **Review the plan** opens under a
   new event ID.
3. **Frozen input snapshot.** The identifiers show **Plan v1** and the
   heading **Safety validated** appears (up to 15 s). **Frozen plan inputs**
   lists **Input snapshot** and **Eligibility snapshot** IDs with nothing
   pending or unavailable.
4. **Forecast.** **Forecast intervals** covers `5000 sites`; **Frozen device
   availability** shows `5000`; **Frozen outage risk** and **Unavailable
   forecast sources** are shown.
5. **Reserve-preserving plan.** **Optimization explanation** shows **Reserve
   held back** in kWh, and the **Interval feasibility** table has rows.
6. **Value, constraints, exclusions.** **Exclusions by reason** and
   **Constraint margins** (with a `RESERVE` row) are shown. Expand **Modeled
   objective · inspect the value and costs** to reveal **Net objective**.
7. **Travel Flex and WEATHER reserve.** Enter a WEATHER-overridden device in
   **Find reserve device**: **Household reserve basis** shows `WEATHER`, the
   alert source ID, and an effective reserve at or above the override floor.
   Enter the seeded window in **Find Travel Flex window**: **Travel Flex
   eligibility** shows the credit in cents and the consent and policy
   versions.
8. **Safety gate.** Click **Validate unsafe alternative**. **Alternative
   rejected** appears while **Safety validated** stays on the plan.
9. **Approve and launch.** Switch **Demo role** to `approver`. Click **Review
   approval**, type `1` in **Type plan version 1**, click **Confirm
   approval**, and see **Approved**. Click **Review launch**, type `1` again,
   click **Confirm launch**. Within 60 s the heading moves to **Sent** (or a
   later state); SENT waits for serial command publishing to finish. Click
   **Execution**.
10. **Command fan-out.** **Command fan-out** shows `SENT`, `ACKNOWLEDGED`,
    `UNCERTAIN`, `EXECUTING`, or `COMPLETED`, and **Command records** has rows.
11. **Seeded failure.** Stay on **Execution**. **Recorded failures** shows
    `MISSING_TELEMETRY` within 3 minutes and `UNCERTAIN_COMMAND` within 6.
12. **Recovery.** **Recovery decisions** shows `RETRY`, `REBALANCED_COMMAND`,
    and `STALE_CAPACITY_REMOVED`.
13. **Sent, acknowledged, delivered.** **Measured event response** shows
    nonzero **Sent intent** and **Acknowledged receipt**, **Measured
    delivery** as a MW value (never zero-filled; **Delivery unknown** without
    measurements), and `SIMULATED`.
14. **Protected reserve.** **Reserve protection evidence** shows `N of M
    devices observed · MEASURED`, a minimum margin of at least 0, no alert,
    and **Unobserved devices remain unknown**.
15. **Expiry and safe return.** After the end time, **Safe return evidence**
    shows a nonzero count of `zero-setpoint intents` and states that they do
    not confirm the fleet has stopped.
16. **Report.** Click **Report**. **Event evidence** opens; **Event report**
    carries the event ID and **Requested power**, **Acknowledged power**,
    **Delivered power**, **Tracking error**, **Response latency**, **Member
    rewards**, **Modeled margin**, **Provenance and versions**, and
    **Assumptions**, labeled `SIMULATED` and `not settled revenue`.
    **Planned shortfall** has rows; **Delivery shortfall** shows `MEASURED`
    kWh.
17. **Replay.** Click **Replay event**. **Event replay** shows `IDENTICAL`
    with **Input snapshot**, **Eligibility snapshot**, and **Fleet SHA-256**.
    Drag **Replay position** to the end: **Historical geography clock**
    matches **Replay time**; drag back to the start and the time changes.

The acceptance run ends only when the director observes every expected
screen on the scenario-backed demo and `demo-path.spec.ts` passes on the
same build.
