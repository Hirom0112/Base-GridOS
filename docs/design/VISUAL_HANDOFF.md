# GridOS visual implementation handoff

## Objective

Build the GridOS operator console as one connected control-room experience.
Proposal B, the Cinematic Living Grid, is the selected direction. Proposal A,
the Evidence-first Control Room, remains its static frame, reduced-motion
composition, no-WebGL fallback, and low-quality tier.

There is no separate marketing site. The operator console is the product. The
shell, event thread, and Greater Austin Living Grid connect every route through
the operating loop:

```text
Observe → Forecast → Optimize → Approve → Dispatch → Verify → Learn
```

Do not build disconnected dashboard pages. Do not begin with animation. Build
the complete static frame, then progressively add spatial behavior only when a
named server fact supports it.

## Required reading order

Read these files completely before creating or changing a component:

1. [`AGENTS.md`](../../AGENTS.md)
2. [`docs/design/visual-system.md`](visual-system.md)
3. [`docs/design/operator-console-proposals.md`](operator-console-proposals.md)
4. [`claude docs/UI_TRACK.md`](../../claude%20docs/UI_TRACK.md)
5. [`claude docs/ISSUES.md`](../../claude%20docs/ISSUES.md)
6. [`claude docs/BUILD_ORDER.md`](../../claude%20docs/BUILD_ORDER.md)
7. [`FULL_SPEC.md`](../../FULL_SPEC.md)
8. [`TECHSTACK.md`](../../TECHSTACK.md)
9. [`docs/domain/system-understanding.md`](../domain/system-understanding.md)

Use `FULL_SPEC.md` sections 2, 3, 4, 5, 9, 10, and 11 as the product truth.
Use `UI_TRACK.md` as the UI work order and ownership boundary. Use
`BUILD_ORDER.md` to confirm backend dependencies before consuming a contract or
fixture.

## Preserved concept visuals

- [Proposal A static control room](assets/operator-console-static.png)
- [Proposal B cinematic Living Grid](assets/operator-console-cinematic.png)
- [Side-by-side local viewer](operator-console-proposals.html)

The generated images are art-direction references, not UI specifications.
Generated labels and values do not override protobuf contracts, recorded
fixtures, or product wording.

## Selected visual architecture

The authenticated console shell owns one persistent renderer. Route components
provide focal state but do not create canvases, animation clocks, or render
loops.

The shell consists of:

- A thin top bar for product, scope, scenario time, role, and system state.
- A left operating-loop rail rather than page-oriented navigation.
- A central situational field containing the Living Grid and route evidence.
- A right evidence rail for provenance, freshness, exclusions, assumptions,
  and audit detail.
- A persistent bottom truth strip for fleet and active-event state.

The twelve-column desktop composition is:

```text
2 columns operating loop
7 columns situational field
3 columns evidence
```

The depth planes are:

- Near: decisions, approvals, stop controls, alerts, and active selection.
- Middle: charts, map state, forecast intervals, event comparisons, and labels.
- Far: the Living Grid, topology, and restrained atmosphere.

On small screens, order content as event truth, critical state, primary action,
evidence, visualization, then secondary controls.

## Proposal B interaction sequence

The same Greater Austin field changes state across routes. It is not replaced
by a new hero object on each page.

| Stage | Route focus | One spatial event | Required truth source |
| --- | --- | --- | --- |
| Observe | `/fleet`, `/map` | Neutral H3 capacity field resolves into the selected region | `GetFleetSummary`, `ListSites`, later geographic methods |
| Forecast | `/dispatch/:eventId` | One blue uncertainty envelope surrounds candidate capacity | Forecast issue, interval, model version, provenance |
| Optimize | `/dispatch/:eventId` | Eligible cells form one cohort; excluded cells separate | Versioned plan explanation and named exclusion reasons |
| Approve | `/dispatch/:eventId` | Unsafe cells lock out; the field holds completely still | Safety result and approved plan version |
| Dispatch | `/events/:eventId` | One white command trace enters approved cells | `WatchEvent` reporting `SENT` |
| Verify | `/events/:eventId` | Blue acknowledgement perimeters and mint measured delivery separate | Per-H3 acknowledgement and telemetry values |
| Learn | report and comparison routes | Commanded and measured contours retain their residual difference | Report, replay seed, versions, ordered updates, one replay timestamp |

Never animate dispatch from approval or launch responses. Approval and launch
are separate actions. Command motion begins only when the server reports
`SENT`. Never show verified delivery before telemetry.

## Visual language

Use the tokens and timing values from `visual-system.md` without substitution:

- `#050907` page and canvas void.
- `#09120E` application background.
- `#0E1B15` raised panels.
- `#16271F` borders and inactive geometry.
- `#F0F7F2` decisive text.
- `#B9C9BF` secondary text.
- `#7F9588` metadata.
- `#66F2A4` healthy available or measured energy.
- `#6DB8FF` forecasts, acknowledgements, and informational state.
- `#F2BC57` review, uncertainty, and warning.
- `#FF6B68` unsafe, failed, or blocked state only.
- `#66736C` stale, unavailable, or disconnected state.

Use a precise grotesk for interface text and a mono face with tabular figures
for telemetry. Use solid dark surfaces, one-pixel borders, and restrained
6 px, 10 px, and 16 px radii. Avoid default glass, thick neon, decorative
status colors, rainbow gradients, bloom, film grain, particles, and camera
shake.

## Data-to-visual rules

Every visual property has one meaning:

- Position represents an H3 aggregate location.
- Cell footprint represents site count.
- Cell elevation represents installed or nameplate capacity from the contract.
- Mint interior fill represents measured available or delivered energy only.
- Blue envelope represents a forecast interval.
- Amber softness or boundary represents uncertainty or an explicit review
  state.
- Red lock marks represent unsafe, failed, or blocked states only.
- Gray desaturation represents explicit offline or unavailable state.
- Physical separation represents exclusion or quarantine with a named reason.
- White directional paths represent command intent reported as sent.
- Blue perimeters represent acknowledgement.
- Mint fill or rise represents telemetry-verified delivery.
- A selection outline represents operator focus and is not an operational
  status.

If a required field is absent, omit the encoding and render the honest gap.
Never infer cell-level state from a fleet-wide total.

## Current data readiness

The deterministic Greater Austin fleet exists at
`testdata/fleets/austin-5000.jsonl`:

- 5,000 simulated devices.
- 320 H3 cells.
- Load zone `LZ_AEN`.
- Weather zone `SCENT`.
- No street addresses.

The generated fleet currently totals approximately 42.630 MW nameplate
discharge, 126.275 MWh usable energy, and 50.518 MWh customer-designated
reserve. These calculations are useful for validating the generator, but the
console must use recorded API responses rather than recalculating operational
aggregates in the browser.

Check the current contract and fixture state before implementation. At the
time of this handoff:

- Recorded Austin Wave 1 API fixtures are scheduled through BUILD_ORDER 1F.4.
- `LaunchEvent` and the server-owned launch record are scheduled in Wave 1.
- Per-H3 dispatchable capacity and availability are scheduled through 2F.6.
- Per-H3 sent, acknowledged, delivered, and uncertain updates are scheduled
  through `WatchEvent` in 2F.5.
- Replay seed, versions, and ordered updates are scheduled later through
  `ReplayEvent`.

Do not invent temporary versions of these fields. Use the mailbox request
format from `UI_TRACK.md` when a required contract or fixture is absent.

## What to build first

`apps/console/` was empty when this handoff was written. Begin at U0.1 and
follow `UI_TRACK.md` in order.

### U0.1 harness

Create the smallest current TanStack Start application with:

- Strict TypeScript and `noUncheckedIndexedAccess`.
- Tailwind.
- ESLint errors for `no-explicit-any`, complexity 18, maximum depth 4,
  500 lines per file, and 150 lines per function.
- Vitest and Testing Library.
- Playwright.
- Only the root route.

Verify with:

```text
pnpm --dir apps/console build
pnpm --dir apps/console lint
pnpm --dir apps/console test
```

Commit U0.1 separately.

### U0.2 static shell

Write the shell test before the component. Build the static frame with:

- The golden design tokens.
- Top bar, operating-loop rail, content field, evidence rail, and status strip.
- A complete light and dark snapshot even though dark is the primary art
  direction.
- Semantic landmarks, keyboard order, visible focus, and at least 14 px primary
  UI text.
- No canvas, route transition, ambient loop, or speculative data.

Verify with:

```text
pnpm --dir apps/console vitest run shell
```

Commit the RED test and GREEN implementation separately.

### Subsequent work

Continue with provenance and freshness enforcement, authentication, typed API
client, fixtures, and fallback baselines. U1.7 adds the first static Living
Grid renderer only after the recorded Austin `ListSites` fixture is available.
Do not skip ahead to dispatch motion; U2.6 depends on recorded per-H3 stream
data.

## Renderer constraints

- One renderer mounted by the authenticated shell.
- One normalized scene state.
- One animation clock.
- One focal transformation per viewport.
- Device-pixel ratio capped at 1.5.
- Instancing for repeated H3 cells.
- No continuous React state updates from the render loop.
- Pause when hidden.
- Park and release the Living Grid WebGL context on `/map`; MapLibre is the
  only active canvas there.
- Dispose only when the shell exits.
- Lazy-load spatial code after useful HTML.
- Keep critical route JavaScript below 250 KB gzip before the lazy spatial
  chunk.

## Static and failure equivalents

Proposal A is not optional. It defines the information hierarchy for every
state and is the required fallback when motion or WebGL is unavailable.

The no-WebGL view must provide:

- An SVG or structured-DOM H3 overview.
- A sortable regional aggregate table.
- Direct forecast interval labels.
- Exclusion lists grouped by reason.
- A semantic event timeline.
- Three distinct sent, acknowledged, and delivered series.
- A native replay control and timestamped state table.

Reduced motion removes camera travel, morphing, propagation waves, velocity
deformation, and ambient loops. It presents intentionally composed still
states, not animations frozen midway. All 17 demo steps remain operable.

## Non-negotiable product wording and boundaries

- Mark every simulated view `SIMULATED`.
- Show timestamp, freshness, and provenance for every aggregate.
- Never show a street address.
- Never claim acknowledgement is delivery.
- Show stop as `STOP REQUESTED` until confirmation arrives.
- Explain a 0% plan as “no customer-designated reserve above protected
  limits.”
- Use “energy anomaly signal” and no intrusion or life-safety language.
- Show backup readiness at current usage and at a 750 W reference load.
- Show all five home power-flow fields.
- Never expose member travel or away state to a partner role.
- DOM owns approval, launch, stop, forms, alerts, audit evidence, and every
  other consequential action.

## Working rules

- Edit only `apps/console/` while implementing the UI track.
- Preserve unrelated worktree changes.
- Use exact-path commits; never stage the repository broadly.
- Write zero comments and zero docstrings in code.
- Use no TypeScript `any` or suppression directives.
- Build against fixtures first, the mock server second, and the real stack
  last.
- Do not run the full repository suite. Run the item verification and allow the
  director to run wave gates.
- Report results in `.local/mailbox.log` using the `ui:` format in
  `UI_TRACK.md`.

## Definition of the intended result

The operator should feel that every route is a different lens on the same
fleet and the same event. At step 1 the field is quiet and trustworthy. At
approval it becomes governed. At step 13 the difference between command,
acknowledgement, and physical response is visible without reading a manual. At
step 17 the same system proves its result again through deterministic replay.

The wow comes from causal truth becoming spatially legible, never from an
unrelated effect.
