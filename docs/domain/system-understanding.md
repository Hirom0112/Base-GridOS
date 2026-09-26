# GridOS domain and system understanding

Reviewed September 25, 2026. This document preserves domain reasoning,
calculations, and source research. [`TECHSTACK.md`](../../TECHSTACK.md) is the
authority for the current architecture, and
[`data-discovery-log.md`](../data/data-discovery-log.md) preserves the associated
source investigation. The repository had no application code at review time.

## What we are building

Base GridOS is a proposed control-room simulator for a fleet of residential batteries. It helps an operator decide how much a fleet can promise, select eligible batteries, execute a dispatch reliably, recover from failures, and measure what happened. Its central promise is that grid participation respects each home's backup requirement.

The minimum complete product is a closed loop: **observe → forecast → optimize → dispatch → verify → learn**. A map is the operator's entry point; the state machine, reserve constraints, and measured feedback are the substantive product. The learning step initially updates availability estimates and reports forecast error; it does not need autonomous model training.

The member-facing flexibility layer has two controls. A resilience plan sets a
standing customer reserve preference, while Travel Flex applies a temporary,
automatically expiring preference during an announced absence. A zero-percent
customer reserve means no member-selected buffer above the protected device and
dynamic risk floor, never a physically empty battery. Weather, outage risk,
stale telemetry, device alarms, or early return may raise the effective reserve
at any time.

The hackathon version controls simulated devices. Public observations inform the scenario, and synthetic household data fills the explicitly identified gaps. An authorized sandbox could later replace the device/data adapters without changing the decision model. Real hardware control requires a separately validated integration and operational permissions.

## What the independent research changes

| Finding | Design consequence |
| --- | --- |
| Base's current article exposes fleet and dispatch tables directly in the page, including the 205.5 MW August 2026 nameplate figure. It labels capacity and coverage caveats. [B1] | Prefer a versioned page-table snapshot where suitable; parsing a hashed JavaScript bundle is not the only path. This aggregate history is context, not the current fleet's dispatchable MW. |
| Base distinguishes device telemetry from premise-meter settlement and warns that its illustrated line-flow effect remains unexplained. [B1] | Keep operational tracking, settlement estimates, and causal grid-impact claims separate. |
| ERCOT's Public API documentation requires a subscription key and authentication token. [E1] | “Public data” does not mean every endpoint works anonymously. Use a cached approved report for the offline demo. |
| NWS documents forecast discovery via `/points` and active alerts, and requires a identifying User-Agent. [W1] | Store forecast issue time and valid time; cache the forecast endpoint and refresh its mapping periodically. |
| ERCOT maintains an ADER pilot page and governing-document links. [E2] | Do not turn a proposed program phase in an article into an implemented rule. Keep market eligibility/configuration versioned and verify the applicable document before production. |
| Neither reviewed Codex document supplies authorized per-device telemetry or a usable sandbox credential. | All household, topology, command, and response examples in this learning package are simulated. The review did not establish a private fleet integration. |

This research verifies selected central claims, not every endpoint in the discovery log. Address routing and bill-analysis endpoints were not exercised; they are not necessary for this architecture. The hackathon rules and participant terms were not independently reverified. Event track recommendations remain the supplied spec's planning context.

## System boundaries and ownership

1. **Provider adapters:** obtain public snapshots, synthetic fixtures, or approved sandbox records. Preserve origin and permissions; do not silently replace failed real data with synthetic data under the same label.
2. **Normalization and quality:** standardize UTC timestamps, interval lengths, units, sign conventions, device identities, and geographic identifiers. Deduplicate on source identity plus event identity. Mark late, stale, missing, and inferred values explicitly.
3. **Data stores:** immutable raw snapshots support replay; normalized time-series support analytics; an event journal records plans, commands, acknowledgements, measurements, and reasons. These can be local files and an embedded database for the MVP.
4. **Forecasts and twins:** forecast home net load, weather risk, and fleet availability. Each twin carries physical limits, current energy, policy reserve, uncertainty, and last-seen time. A twin is an estimate of a device, not proof that the device responded.
5. **Optimizer:** selects a time-indexed schedule within physical, member, geographic, and market constraints. Output includes requested/feasible capacity, spare headroom, rejected sites and reasons, assumptions, and input version identifiers.
6. **Independent safety gate:** rechecks freshness, reserve, temperature, maintenance, participation, export limits, and plan validity immediately before sending. A plan that was safe five minutes ago may no longer be safe.
7. **Orchestrator and device adapter:** coordinate commands, deadlines, acknowledgements, retries, leases, cancellation, and failures. The simulator uses the same command contract proposed for a future sandbox.
8. **Verifier and reporting:** compare actual measured outcomes with the intended service definition. Reconcile state, calculate energy and response metrics, annotate uncertainty, and feed failures back into the next optimization.

The operator UI submits requests through a policy-controlled API. The copilot reads evidence and may draft a request; the same authorization, safety gate, and approval policy govern that request. A stop request is a high-priority command, not a guarantee that a disconnected device heard it. Independent local device protection remains essential.

## The physical model: power, energy, and the meter boundary

Power is a rate, in kW or MW. Energy is an amount, in kWh or MWh. A 20 MW event lasting two hours requires 40 MWh at its defined measurement boundary. The original demo script specifies 20 MW but does not specify duration or measurement boundary; those must be explicit in implementation.

Use nonnegative charge power `p_ch` and discharge power `p_dis`, defined on the AC side. For interval length `dt` in hours:

```text
energy_next_kWh = energy_kWh
                  + charge_efficiency * p_ch_kW * dt
                  - p_dis_kW * dt / discharge_efficiency

reserve_kWh(t) <= energy_kWh(t) <= usable_capacity_kWh
0 <= p_ch <= permitted_charge_kW
0 <= p_dis <= permitted_discharge_kW
grid_import_kW = home_load_kW - solar_kW + p_ch_kW - p_dis_kW
```

Prohibit simultaneous charge and discharge with a mode constraint or a physically enforced scheduling policy. Do not use round-trip efficiency as both one-way efficiencies; their product is round-trip efficiency. Inverter, thermal, ramp, and export constraints can tighten these limits.

The reserve floor protects **grid-service dispatch while connected**. During an outage, the battery can consume that protected energy to serve critical loads. An islanded site has zero grid-service capacity and a distinct backup operating mode; it still respects its device minimum SOC. Never prohibit use of emergency reserves precisely when the household needs them.

There are three different service quantities: battery discharge; net export through the home's meter; and reduction in grid import relative to a baseline. They are not interchangeable. The optimization request and verifier must use the same boundary. A battery can help the grid by reducing imports even while its house remains a net consumer.

### A worked example using the supplied synthetic fixture

Assume 39.2 kWh usable capacity, 74% SOC, a 40% reserve, 10 kW inverter, no solar, constant 3.1 kW home load, and an illustrative 95% one-way discharge efficiency.

```text
Stored energy:                  39.2 × 0.74 = 29.008 kWh
Protected reserve:              39.2 × 0.40 = 15.680 kWh
Energy above reserve:                        13.328 kWh
AC energy available:            13.328 × .95 = 12.6616 kWh
Two-hour battery discharge:     min(10, 12.6616 / 2) = 6.3308 kW
Two-hour net export:             6.3308 - 3.1 = 3.2308 kW
Backup at constant 3.1 kW load:  15.680 × .95 / 3.1 ≈ 4.805 hours
```

At the same reserve and efficiency, 18 hours would require a critical-load average at or below about 0.828 kW. Therefore “40% reserve” cannot imply “18 hours backup” without a critical-load forecast. This example assumes the capacity/SOC energy convention above; a real adapter must confirm whether reported usable energy already accounts for conversion losses.

A risk-adjusted reserve can be the greater of a member minimum and forecast critical-load energy over an outage horizon, adjusted for losses and uncertainty. If the requirement exceeds capacity or current stored energy, report the protection deficit and prioritize charging where feasible. Do not clamp the reserve and claim the backup requirement is satisfied.

## Forecasts and optimization

Start with a repeatable recent-day or similar-day home-load baseline. Track out-of-sample forecast error before adding boosted trees. Weather alerts are observed inputs; an outage probability is a modeled estimate requiring validation. SOC uncertainty grows when telemetry is stale. Unknown electrical topology cannot support a claim of actual feeder relief.

The optimizer consumes the event window, target and service boundary, eligible devices, forecasts, reserves, economic assumptions, and any authorized topology. Hard constraints enforce protection and participation. A soft target-shortfall variable makes infeasibility visible rather than forcing unsafe output. The objective can penalize shortfall, cycling, and uncertainty while valuing delivered service.

Do not count `confidence × nameplate` as guaranteed capacity. Availability is a probability estimate; reserve margin or scenario constraints should account for correlated regional failures. Summed individual headroom may still be infeasible because of shared network constraints or event duration.

A deterministic fallback can filter ineligible devices, calculate per-interval energy-limited power, rank the survivors, allocate conservatively, and report residual shortfall. It must use the same hard constraints as the solver. Re-run on a 5–15 minute planning cadence and on material state changes; the dispatch/measurement loop must operate at the event's required faster cadence. These are separate clocks.

## Reliable execution and failure recovery

Track **event state** separately from **per-command state**. An event may partially succeed while some commands time out. The happy path is planned, validated, sent, acknowledged, executing, verified, then an explicitly labeled settlement estimate. An acknowledgement confirms receipt/acceptance; only measurements establish delivery.

A command should carry event ID, device ID, immutable command ID, plan version, sequence/generation, absolute setpoint, effective time, expiry, and expected policy version. Retries repeat the same command ID and payload. A changed schedule gets a new version. Receivers must durably deduplicate and reject obsolete generations. An in-memory simulator demonstrates this contract; it does not prove crash-safe production delivery.

For the recommended local journal, commit a command intent/outbox entry and state change in one SQLite transaction. A single state owner sends committed entries and uses conditional transitions; on restart it scans pending entries and reconciles ambiguous sends. Remote actions are not atomic with database commits. Isolate bounded solver work in a subprocess so it cannot block stop handling and expiry. Independently check finite values, energy bounds, and feasibility using explicit numerical tolerances before accepting a solver result.

The hardest case is **sent but acknowledgement lost**. The device may still be discharging. Mark delivery as uncertain, bound possible output, inspect fresh telemetry, and cancel or await enforced command expiry before reallocating overlapping capacity. Otherwise recovery can overshoot. A lease only works if the device enforces it using a trusted clock. Network silence alone does not prove zero power.

For the demo, use an explicitly simulated expiry behavior. Inject at least three failures: lost connectivity, reserve violation, and a worker/retry fault. Detect them, exclude or quarantine unsafe devices, reconcile commands, re-optimize, revalidate, and measure again. If safe spare capacity is insufficient, report a miss. “20% of Houston devices” is not “20% of fleet MW”; calculate the loss from affected device schedules.

## Verification, economics, and provenance

Integrate observed power over actual elapsed intervals to obtain energy; do not just sum kW samples. Report signed error, absolute error, response latency, data completeness, reserve violations, and uncertain intervals. Missing telemetry is neither zero delivery nor successful delivery. A delayed measurement uses event time and should update the reconciliation history without erasing earlier knowledge.

Baseline savings and market revenue are estimates until tariffs, service rules, metering, and settlement inputs are known. Revenue is not profit: include charging cost, losses, degradation, and penalties. Avoid double counting an exported kWh as both an avoided imported kWh and an energy sale.

Additional flexibility is worthwhile only when added dispatch or avoided-peak
value, plus reliability value, exceeds charging, degradation, penalties, the
member reward, and incremental support/risk cost under conservative estimates.
Use market-specific fixed daily, event, or annual rewards rather than assuming
every territory has a membership fee to waive. Away-period load may support an
unusual-energy-activity notification with consent, but it is not a verified
burglary, fire, or life-safety signal.

Extend the discovery log's source classification with independent fields:

```text
source_class: CONFIRMED_PUBLIC | CONFIRMED_ORGANIZER_SANDBOX |
              SIMULATED | INFERRED_NOT_VERIFIED
value_kind: observation | forecast | modeled_estimate
source_url / dataset_id / source_version
observed_at / issued_at / valid_from / valid_to / ingested_at
unit / interval_seconds / quality_flags / geographic_resolution
input_dataset_ids / model_version / plan_id / command_id
```

A forecast derived from public weather remains a forecast; source class alone is insufficient. Replay manifests pin snapshots, fixture seed, solver/fallback version, scenario events, and the clock. Keep exact addresses outside the optimizer and default public views to synthetic/aggregated locations.

## Practical MVP boundary

Build a modular application whose deployment boundaries follow the current
architecture in [`TECHSTACK.md`](../../TECHSTACK.md). Keep responsibilities
explicit without treating every diagram box as an independently deployed
service.

Implement in this order: contracts and reproducible fixtures; energy/reserve mathematics; solver or deterministic allocator; safety gate and command journal; simulator and failure injection; measured verification; operator UI and explanations. Utility/member portals, predictive maintenance, installation intelligence, and a broad copilot are later views or extensions.

Acceptance evidence should include duration-aware feasible and infeasible events, reserves and inverter limits at every timestep, lost-ACK reconciliation, idempotent retries, stale-command rejection, process restart/replay, three injected failures, measurement gaps, and truthful shortfall reporting. Cache every input needed for the final demo.

Open decisions before implementation: event duration and service boundary; exact public grid report; authorized dataset availability; critical-load definition; conservative freshness thresholds; simulated device lease semantics; command approval policy; economic assumptions; and ownership of the member-facing promise. A utility/zone value on a synthetic coordinate is a fixture assumption until an actual mapping verifies it.

## Sources consulted independently

- **B1:** [Base: ADER Phase IV and the capacity crunch](https://www.basepowercompany.com/blog/aggregated-ders-and-the-capacity-crunch), inspected September 25, 2026. Aggregate context and measurement distinctions only; not a household telemetry feed.
- **E1:** [ERCOT: Using the Public API](https://developer.ercot.com/applications/pubapi/user-guide/using-api/), inspected September 25, 2026. Authentication requirements.
- **E2:** [ERCOT: ADER pilot project](https://www.ercot.com/mktrules/pilots/ader), inspected September 25, 2026. Authoritative starting point for program documents; this review is not a rule-compliance certification.
- **W1:** [NWS API documentation](https://www.weather.gov/documentation/services-web-api), inspected September 25, 2026. Forecast and alert access.
- **A1:** [ElevenLabs: Create speech](https://elevenlabs.io/docs/api-reference/text-to-speech/convert) and [quickstart](https://elevenlabs.io/docs/eleven-api/quickstart), inspected September 25, 2026. Used only for the learning-video narrator integration.

The engineering model, proposed safety additions, calculations, and build order are this review's design recommendations, not claims that Base implements this architecture.
