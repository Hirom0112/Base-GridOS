# GridOS planning issues

These issues must be resolved in `BUILD_ORDER.md`, `UI_TRACK.md`, the service
contracts, and the fixture schedule before the connected operator-console
experience can be implemented truthfully.

## 1. Add an explicit launch boundary

The protobuf exposes `ApproveEvent` but no `LaunchEvent` or equivalent command.
Approval must not implicitly mean dispatch.

The revised plan must include:

- A `LaunchEvent` RPC carrying event ID, approved plan version, idempotency key,
  actor, and timestamp.
- A server-confirmed launch state.
- A launch fixture and demo-path coverage for step 9.
- Command-flow animation beginning only when the event stream reports `SENT`,
  never from the approval response.

## 2. Prioritize the data needed for the approval-to-energy beat

The strongest demonstration moment cannot ship truthfully with the currently
scheduled contracts and fixtures.

The revised plan must deliver these dependencies before Living Grid dispatch
motion:

- Per-H3 dispatchable capacity and explicit availability state.
- Aggregate plan exclusions with reasons.
- `WatchEvent` updates containing sent, acknowledged, and delivered values by
  H3 aggregate.
- A launch-confirmed event state.
- Timestamp, freshness, and provenance metadata on every aggregate.

U1.7 and U2.6 must not depend on invented browser state while these contracts
are pending.

## 3. Replace the sparse Houston demo fixture with a Greater Austin fleet

The current UI fixture contains 50 devices across four H3 aggregates and the
event request selects `LZ_HOUSTON`. Four cells cannot demonstrate fleet scale,
and the selected region conflicts with the intended Austin experience.

The revised plan must include:

- A deterministic Greater Austin demonstration fleet.
- Approximately 5,000 devices distributed across a few hundred H3 cells.
- `LZ_SOUTH`, or the verified correct Austin operating region, used
  consistently throughout the canonical demo.
- An event target supported by the generated fleet's actual capacity.
- Updated fleet summary, H3, dispatch, event, weather, and scenario fixtures.
- The 50-device fleet retained for unit tests only.
- Tests for determinism, geographic scope, density, aggregate consistency,
  capacity consistency, and absence of addresses.

## 4. Move Living Grid ownership to the console shell

U1.7 currently describes a fleet-route renderer that is disposed when the
route exits. That prevents the spatial layer from connecting the operating
loop across routes.

The revised plan must define:

- One renderer owned by the authenticated console shell.
- Persistence across fleet, dispatch, live-event, report, comparison, map, and
  member routes.
- One normalized scene state and one animation clock.
- Route components that provide focal state without creating render loops.
- Pausing when hidden and disposal only when the console shell exits.
- Equivalent DOM or SVG operational truth on every route.

## 5. Define the MapLibre and Living Grid renderer policy

U4.1 requires MapLibre while the shell owns a persistent WebGL renderer.
Running both simultaneously would violate the one-canvas principle and consume
unnecessary GPU resources.

The revised plan must choose and document one approach:

- Keep MapLibre for the local basemap and electrical drill-down, pause and park
  the Living Grid while `/map` is active, and resume it without losing event
  context.
- Replace MapLibre only if the Living Grid can satisfy every U4.1 basemap,
  layer, privacy, and drill-down requirement.

Only one renderer may be active at a time.

## 6. Preserve every dispatch lifecycle boundary

The plan must not collapse approval, launch, delivery intent, acknowledgement,
and physical response.

The required sequence is:

```text
approved
→ launch requested
→ commands persisted
→ sent
→ acknowledged or uncertain
→ executing
→ telemetry verified
→ reconciled
→ reported
```

Every state needs a named server source, fixture, audit transition, and distinct
UI representation. A stop remains `STOP REQUESTED` until acknowledgement or
telemetry confirms its effect.

## 7. Give replay one authoritative clock

The report chart, audit timeline, and Living Grid must not maintain independent
playback positions.

The revised plan must include:

- One normalized replay-time source.
- Replay driven by the recorded seed and versioned inputs.
- Charts and spatial state projected from the same timestamp.
- No deterministic replay animation until `ReplayEvent` supplies the required
  seed, versions, and event updates.

## 8. Reorder the build around UI-critical contracts

The connected experience cannot wait for late-wave contracts after the shell
and Living Grid have already been implemented against incomplete data.

The dependency order should be:

```text
LaunchEvent contract
→ Greater Austin fleet and H3 fixtures
→ per-H3 planning fixture
→ WatchEvent aggregate fixture
→ shell-scoped renderer
→ approval and dispatch motion
→ replay synchronization
```

The revision must update `BUILD_ORDER.md`, `UI_TRACK.md`, API fixture indexes,
method ownership, and gate expectations together.

## 9. Define FULL_SPEC step 9 unambiguously

FULL_SPEC says the operator launches the event while the current API only
approves it.

The revised plan must decide whether:

- Approval and launch are separate operator actions.
- Approval causes a server-owned transition into launch.

A separate launch action is preferred because it preserves the independent
safety gate and creates an explicit, auditable command boundary. In either
case, the browser must never infer launch from navigation or local state.

## 10. Require truthful reduced-motion and no-WebGL operation

The dense Living Grid must remain an enhancement rather than an operational
dependency.

The revised plan must require:

- An SVG or structured DOM H3 fallback.
- Intentionally composed reduced-motion still states.
- Exact sent, acknowledged, and delivered values in semantic DOM.
- All 17 demonstration steps remaining passable without WebGL.
- No loss of provenance, freshness, exclusions, event state, or authorization
  boundaries in fallback mode.
