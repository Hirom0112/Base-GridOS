# GridOS demo video script

Target length is 3 to 4 minutes. The runbook behind every click is
`docs/operations/demo.md`; the labels below match `apps/console/tests/demo-path.spec.ts`.
Every fleet, telemetry, and economic value in this demo is SIMULATED or modeled,
and the narration says so.

## The timing problem, and how this script solves it

The event window is ten minutes long and the seeded faults fire inside it, so
the demo cannot run in real time inside four minutes. Launch the event early,
narrate the plan and safety evidence while the workflow runs, then pause the
Loom (or stop and start a second recording) and resume once the faults and the
measured delivery have landed. Stitch the two recordings if you record
separately. Nothing on screen is faked by the cut; say "I paused the
recording while the ten-minute event ran" when you resume.

## Pre-flight checklist

1. Confirm the standing stack is up: open `http://127.0.0.1:3000/fleet` and see
   the command center. The director reported it ARMED with telemetry lag under
   15 s. Do not launch any other event before recording; the first launch on
   this gateway is the one that carries the live faults.
2. Browser: one window, every other tab closed, notifications off, zoom at 90
   percent so tables fit, window sized to your Loom capture area.
3. **Demo role** selector (top of the page) is on `operator`.
4. Have a terminal ready off-camera for the optional reserve and Travel Flex
   lookup in segment 2 (command below).
5. Know your times: the plan start must be the next UTC minute at least one
   minute ahead, the end ten minutes later. Check `date -u` just before you
   record.

## Segment 1, 0:00 to 0:20, hook and problem

**Clicks.** None. Start on `/fleet`.

**On screen.** The command center: **Austin conditions**, the fleet panel with
**Availability**.

**Say.** "On a severe-weather evening in Texas, the grid needs capacity, and
thousands of home batteries could provide it. The hard part is doing it
without draining the backup power a family is counting on. GridOS coordinates
five thousand simulated Austin batteries as dependable capacity while protecting
every household's reserve."

## Segment 2, 0:20 to 1:20, plan and safety

**Clicks.**
1. Expand **Inspect Austin markets · LZ_AEN and SOUTH_C** and **Inspect weather
   and historical outage evidence** for one beat each.
2. Click **Plan a dispatch**. Set **Operating region** to **Greater Austin ·
   LZ_AEN**, **Start time** to the next UTC minute at least one minute ahead,
   **End time** ten minutes later, **Target power (MW)** to `20`,
   **Measurement boundary** to **Meter net export**. Click **Create dispatch
   plan**.
3. Wait for **Review the plan**, **Plan v1**, and **Safety validated** (up to
   15 s). Scroll past **Frozen plan inputs**, **Forecast intervals**
   (`5000 sites`), and **Optimization explanation** with **Reserve held back**.
4. Show **Exclusions by reason** and **Constraint margins** (point at the
   `RESERVE` row). Expand **Modeled objective · inspect the value and costs**.
5. Optional, only if you fetched the IDs: type a device into **Find reserve
   device** to show **Household reserve basis** with `WEATHER`, and a window
   into **Find Travel Flex window** to show the credit. Off-camera lookup,
   with `EVENT_ID` copied from the page:

   ```sh
   curl -s -X POST http://127.0.0.1:3000/rpc/gridos.v1.DispatchService/GetPlanExplanation \
     -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' \
     -d "{\"eventId\":\"$EVENT_ID\",\"planVersion\":\"1\"}" \
     | jq '{reserve: [.evidence.reserveBases[] | select((.overrideReason|tostring)|test("WEATHER")) | .deviceId][0], travel: .evidence.travelFlexBindings[0].windowId}'
   ```

6. Click **Validate unsafe alternative**. **Alternative rejected** appears while
   **Safety validated** stays on the plan.

**Say.** "I ask for 20 megawatts over the next ten minutes. GridOS freezes a
versioned snapshot of prices, weather, outage risk and fleet state, forecasts
each home, and builds a plan that holds back reserve energy. Here is what it
held back, why devices were excluded, and the modeled value. Now I hand the
independent safety gate a deliberately unsafe plan, and it rejects it while the
real plan stays validated."

## Segment 3, 1:20 to 1:45, approve and launch

**Clicks.** Switch **Demo role** to `approver`. Click **Review approval**, type
`1` in **Type plan version 1**, click **Confirm approval**, see **Approved**.
Click **Review launch**, type `1`, click **Confirm launch**. Within 60 s the
heading moves to **Sent** or later. Click **Execution**.

**On screen.** **Command fan-out** with states such as `SENT` and
`ACKNOWLEDGED`; **Command records** filling with rows.

**Say.** "A separate approver signs off on this exact plan version, with a
step-up check, and launches it. Commands fan out to five thousand batteries
through a durable workflow and the gateway."

**Pause the Loom here** (or stop recording one). Keep the **Execution** page
open. Wait until **Recorded failures** shows `UNCERTAIN_COMMAND` (up to about
6 minutes) before resuming.

## Segment 4, 1:45 to 2:20, live faults and recovery

**Clicks.** Resume on **Execution**. Scroll to **Recorded failures**, then
**Recovery decisions**.

**On screen.** **Recorded failures** lists `MISSING_TELEMETRY` and
`UNCERTAIN_COMMAND`. **Recovery decisions** lists `RETRY`,
`REBALANCED_COMMAND`, and `STALE_CAPACITY_REMOVED`.

**Say.** "I paused while the event ran. During it, a seeded fault took devices
offline and delayed a gateway. GridOS recorded missing telemetry and an
uncertain command, retried safely, removed the stale capacity from its count,
and rebalanced the remaining homes, all inside the envelope the approver
signed."

## Segment 5, 2:20 to 2:45, measured delivery versus acknowledgement

**Clicks.** Scroll to **Measured event response**. If still mid-event, pause
again until the end time passes, then show **Safe return evidence**.

**On screen.** Nonzero **Sent intent** and **Acknowledged receipt**, a MW value
for **Measured delivery**, and the `SIMULATED` label. After the end time,
**Safe return evidence** shows a count of `zero-setpoint intents`.

**Say.** "A battery saying yes is not power on the grid. GridOS keeps three
numbers apart: what we sent, what devices acknowledged, and what the meters
actually delivered. When measurement is missing it says delivery unknown
instead of inventing a number. At expiry it sends zero setpoints and says
plainly that those do not prove the fleet stopped."

## Segment 6, 2:45 to 3:05, household reserve protection

**Clicks.** Scroll to **Reserve protection evidence**. Keep this short.

**On screen.** `N of M devices observed · MEASURED`, a minimum margin, and
**Unobserved devices remain unknown**.

**Known gap, be honest.** The measured reserve margin can currently show a
below-reserve value on some devices; that is a known defect being fixed. Do not
linger. If a negative minimum margin or an alert is visible, say the line
below instead of the default one.

**Say (default).** "Every observed home stayed at or above its effective
reserve, and homes we could not observe are reported as unknown, not assumed
safe."

**Say (if a breach shows).** "Here the system surfaces a device reading below
its reserve instead of hiding it. That is exactly the evidence an operator
needs, and it is a defect we are fixing in how that margin is measured."

## Segment 7, 3:05 to 3:45, report and replay

**Clicks.** Click **Report**. **Event evidence** opens. Scroll through **Event
report** (**Requested power**, **Acknowledged power**, **Delivered power**,
**Tracking error**, **Response latency**, **Member rewards**, **Modeled
margin**, **Provenance and versions**, **Assumptions**) and **Delivery
shortfall**. Click **Replay event**. **Event replay** shows `IDENTICAL`. Drag
**Replay position** to the end, then back to the start.

**Say.** "The report shows what was requested, acknowledged and delivered, the
shortfall, latency, member rewards and a conservative modeled margin, labeled
simulated and not settled revenue. Then I replay the whole event from its seed
and frozen inputs, and it comes out identical, down to the fleet hash."

## Segment 8, 3:45 to 4:00, close with impact

**Clicks.** None, stay on the replay.

**Say.** "GridOS turns home batteries into capacity a grid operator can trust,
because every megawatt is measured, every failure is handled on the record, and
no family's backup power is traded away to hit a target."

## If something goes wrong on camera

**Safety validated** is slow: wait the full 15 s; if it never appears, the
telemetry was stale when you planned. Stop, wait for lag under 15 s, and plan
again.

The heading stays below **Sent** after launch: SENT waits for serial publishing
to finish. Pause the recording and give it the full 60 s.

**Recorded failures** is empty: faults only fire on the first launch on a fresh
gateway and within the ten-minute window. Wait up to 6 minutes. If this was not
the first launch, the director must restart the stack; narrate recovery from
what is shown rather than claiming faults that did not occur.

**Measured delivery** reads **Delivery unknown**: that is the honest state
without measurements. Say so and move on to the report.

Anything else: pause the Loom, fix it off camera, and resume. Never narrate a
number that is not on screen.

## Submission fields

**Project title.** GridOS: Home Batteries as Grid Capacity Without Spending the
Family's Backup

**Write-up.**

When a Texas heat wave or winter storm pushes the grid to the edge, the power
it needs is often already sitting in thousands of home batteries. Using that
power is hard: a grid operator needs capacity it can count on, and a household
needs to know its backup reserve will be there if the lights go out. Today
those goals pull against each other, and a battery that says it responded is
not proof that power reached the grid.

GridOS helps the operators who run residential battery fleets, the utilities
and market operators who buy that capacity, and the members whose homes hold
the batteries. It freezes a versioned snapshot of open market, weather and
outage data, forecasts each home, and builds a dispatch plan that holds back
every household's effective reserve, raising it when severe weather is coming.
An independent safety gate must approve the plan, and a separate approver must
sign off before launch. A durable workflow fans commands out to five thousand
simulated Austin batteries, survives offline devices and delayed gateways by
retrying, removing stale capacity and rebalancing within the approved envelope,
and reports sent, acknowledged and physically measured power as three separate
numbers. Members can opt into Travel Flex windows for a clearly stated credit.

The result is capacity a grid can trust and a household can live with. Every
event ends in a report of delivery, shortfall, latency, reserve protection,
rewards and conservative modeled margin, and replays identically from its
inputs. All fleet and economic values in the demo are simulated and labeled as
such; the market, weather and outage inputs come from public data.

**Team roster.** Fill in before submitting.

| Name | Role | Email | GitHub |
| --- | --- | --- | --- |
| | | | |
| | | | |
| | | | |
