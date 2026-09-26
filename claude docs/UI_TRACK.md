# GridOS UI track

**Status:** brief and contract for the UI agent that owns the operator console
**Written:** 2026-09-26
**Runs alongside:** [`BUILD_ORDER.md`](BUILD_ORDER.md), the six backend lanes
**Design authority:** [`docs/design/visual-system.md`](../docs/design/visual-system.md), section
"Golden system for the GridOS web experience" (lines 13 to 345)
**Product authority:** [`FULL_SPEC.md`](../FULL_SPEC.md) §3, §5, §9, §10, §11

---

## 1. Mission

Build the GridOS operator console: a calm, high-trust control room in which an
operator sees the fleet, plans and approves a dispatch, watches it execute,
stops it if needed, and reads an honest report. A smaller member view lets a
household pick a resilience plan and schedule Travel Flex.

The console is precise, legible, auditable, and safe first. The Living Grid
spatial layer earns its place by communicating scale, flow, health,
uncertainty, and cause and effect. It never replaces the DOM for anything an
operator reads or clicks.

## 2. Boundary

The UI agent owns `apps/console/` completely. No backend lane edits anything
under it. The UI agent edits nothing outside it. Three things cross the line:

| Direction | What | Where |
| --- | --- | --- |
| Backend to UI | Protobuf contracts, the single source of truth for every type | `contracts/gridos/v1/*.proto` |
| Backend to UI | Generated Connect-ES client, produced locally by `make generate`, never committed | `apps/console/src/api/gen/` |
| Backend to UI | Recorded API responses for every method, one JSON file per method, refreshed each wave | `testdata/fixtures/api/<Service>/<Method>.json` and `INDEX.json` |
| Backend to UI | A mock Connect server that serves those fixtures so the console runs with no backend | `make ui-mock` (lane 0F) |
| Backend to UI | The real stack | `make demo` (lane 1F) |
| UI to backend | The Playwright acceptance spec for the 17-step demo story, run by the director at every gate | `apps/console/tests/demo-path.spec.ts` |
| UI to backend | Requests for a new field, method, or fixture | a mailbox line, see §8 |

Three ways to run the console, in the order they become available:

1. **Fixtures only.** Vitest and Storybook-style rendering against the JSON
   files in `testdata/fixtures/api/`. Available from Wave 0.
2. **Mock server.** `make ui-mock` serves the same fixtures over Connect with
   local identities and roles. Available at Gate 0.
3. **Real stack.** `make demo` brings up PostgreSQL, Temporal, the gateway
   simulator, the decision service, the control plane, and the console.
   Available at Gate 1.

## 3. Rules that are not negotiable

From `AGENTS.md` at the repo root, which is binding for the UI agent too:
least code, zero comments, gates are one-way, many small commits by exact
path in the shared tree. The pre-commit hook enforces it.

From the specs:

- Strict TypeScript, no `any`, ESLint `no-explicit-any` as an error, and the
  ESLint ceilings `complexity` 18, `max-depth` 4, `max-lines` 500,
  `max-lines-per-function` 150 as errors.
- Browser state is never authoritative. Every consequential action goes to
  the server, and the server enforces authorization (TECHSTACK "Operator
  console", FULL_SPEC §11).
- Every aggregate shows its timestamp, provenance mix, and freshness
  (FULL_SPEC §5.1). Every simulated or modeled value is visibly labelled
  (FULL_SPEC §2, §10). The five provenance classes are `CONFIRMED_PUBLIC`,
  `CONFIRMED_SANDBOX`, `AUTHORIZED_OPERATIONAL`, `DERIVED`, `SIMULATED`.
- No street address, ever. Default views are H3 cells or clusters; exact
  site location appears only when the server grants the `site_location`
  permission (FULL_SPEC §5.2, §11).
- Sent, acknowledged, and physically delivered are three different things and
  are drawn as three different things (FULL_SPEC §4 invariant 3, §9 step 13).
- A stop request is shown as requested, not as heard, until an
  acknowledgement or telemetry confirms it (`docs/domain/system-understanding.md`).
- A `0%` plan is explained as "no customer-designated reserve above protected
  limits", never as an empty battery (FULL_SPEC §5.10).
- The anomaly alert uses the fixed wording "energy anomaly signal" and no
  intrusion, burglary, or safety language (FULL_SPEC §5.10).
- New telemetry is visible within 5 seconds in the local demo (FULL_SPEC §10).
- Backup readiness is always shown two ways: hours at current usage and hours
  at a 750 W reference load. A site that is off grid, in overcurrent, or
  reporting `TelemetryUnavailable` is drawn as exactly that state, never as
  "online" with stale numbers. Home power is drawn as the five-field power
  flow (from grid, from storage, from solar, non-solar to home, to home).
- Roles: operator, approver, analyst, partner, service, member. The partner
  view shows aggregates only and never travel or away state (FULL_SPEC §11).

From the golden system in `docs/design/visual-system.md`:

- The 15 golden rules (lines 35 to 101). Read all of them before the first
  component.
- The visual language: design character, color tokens, typography, surface
  rules, composition (lines 103 to 186).
- Motion grammar and timing tokens (lines 188 to 210).
- The rendering boundary: CSS and DOM for everything an operator reads or
  clicks, Canvas or WebGL only for large instanced fleets, topology, and
  ambient depth (lines 228 to 252).
- Performance and accessibility budgets (lines 254 to 269). These are
  acceptance criteria.
- The anti-patterns list (lines 297 to 310) is an automatic rejection list.
- The review scorecard (lines 312 to 327): a release needs 18 of 20 and no
  zero in truth, safety, accessibility, or fallback.
- The copy-ready art-direction prompt (lines 329 to 345) is the prompt to
  give any image or motion tool.

## 4. Screens

Each screen names its route, the API methods it reads, the spec section that
defines it, and the FULL_SPEC §9 demo steps it must make pass.

| Screen | Route | API methods | Spec | Demo steps |
| --- | --- | --- | --- | --- |
| Shell: top bar, left nav, status strip, provenance badge | all | none | `visual-system.md` "Composition", FULL_SPEC §2 | all |
| Fleet command center | `/fleet` | `FleetService.GetFleetSummary`, `ListSites`, `ContextService.GetMarketContext`, `GetWeatherContext`, `GetOutageRisk`, `ListDispatchWindows` | FULL_SPEC §5.1 | 1 |
| Dispatch request | `/dispatch/new` | `DispatchService.CreateEventRequest` | FULL_SPEC §9 steps 2 and 3 | 2, 3 |
| Plan explanation and approval | `/dispatch/:eventId` | `GetEvent`, `GetPlanExplanation`, `ValidateAlternative`, `ApproveEvent` | FULL_SPEC §5.5, §5.6, §9 steps 4 to 9 | 4, 5, 6, 8, 9 |
| Live event | `/events/:eventId` | `EventsService.WatchEvent` (server stream), `GetEvent`, `EmergencyStop` | FULL_SPEC §5.7, §9 steps 10 to 15 | 10, 11, 12, 13, 14, 15 |
| Event report and comparison | `/events/:eventId/report`, `/events/compare` | `ReportService.GetEventReport`, `CompareEvents`, `ReplayEvent` | FULL_SPEC §5.9, §9 steps 16 and 17 | 16, 17 |
| Map | `/map` | `GeoService.ListCells`, `Drilldown`, plus the local style and basemap the control plane serves | FULL_SPEC §5.2 | 2 (region pick) |
| Member | `/member` | `MemberService.GetMemberStatus`, `SelectResiliencePlan`, `ScheduleTravelFlex`, `EndTravelFlexEarly` | FULL_SPEC §3 "Member", §5.10 | 7 |

Method names above are the contract the backend lanes build to. If a name in
`contracts/gridos/v1/` differs, the proto wins and this table is corrected.

## 5. Items

Status marks and RED/GREEN rules are the same as `BUILD_ORDER.md` §0. The
UI agent marks its own items `[x]` after the verify command passes; the
director re-runs the Playwright spec at each gate. Items in ADAPTED scope:
the Vitest test or the Playwright step is written before the view.

### U0 — shell and harness (runs during backend Wave 0)

- `[ ]` U0.1 `[P]` TanStack Start app with strict TypeScript
  (`noUncheckedIndexedAccess` on), Tailwind, ESLint with
  `no-explicit-any` and the four ceilings from §3 as errors, Vitest, Testing
  Library, Playwright. Only the root route exists. Verify: `pnpm --dir apps/console build && pnpm --dir apps/console lint && pnpm --dir apps/console test`.
- `[ ]` U0.2 `[P]` Design tokens from the golden color, typography, surface,
  and timing tokens; the layout shell (top bar, left nav, content, status
  strip). Static frame first (golden rule 12). Snapshot tests, light and dark.
  Verify: `pnpm --dir apps/console vitest run shell`.
- `[ ]` U0.3 `[P]` Provenance badge and freshness chip: renders one of the five
  classes, refuses to render an aggregate that lacks a timestamp and
  freshness. RED test first. Verify: `vitest run Provenance`.
- `[ ]` U0.4 `[P]` Auth boundary: Clerk provider from env; when
  `GRIDOS_AUTH_MODE=local` a local dev identity with a chosen role and the
  optional `site_location` permission, marked `STUBBED` in code and reported.
  Verify: `GRIDOS_AUTH_MODE=local pnpm --dir apps/console dev` serves the root route with no network call to Clerk.
- `[ ]` U0.5 `[after BUILD_ORDER 0B.6]` Generated Connect-ES client wired into
  `src/api/client.ts` with TanStack Query and typed error and loading states.
  Verify: `vitest run client`.
- `[ ]` U0.6 `[P]` Playwright spec `tests/demo-path.spec.ts`: one `test.step`
  per FULL_SPEC §9 story step, 17 steps, all failing. This is the red test for
  the whole console. Verify: `playwright test demo-path` reports 17 failing steps, none skipped.
- `[ ]` U0.7 `[after BUILD_ORDER 0F.2]` Fixture loader that feeds
  `testdata/fixtures/api/*.json` into Vitest and into a component gallery
  route available only in development. Verify: `vitest run fixtures` and the gallery renders every fixture without a runtime error.
- `[ ]` U0.8 `[P]` Reduced-motion, keyboard, and no-canvas fallback baseline:
  every route renders its full operational truth with WebGL disabled
  (golden rules 14 and 15). Playwright runs the suite once with
  `prefers-reduced-motion` and once with WebGL blocked. Verify: `playwright test --project=reduced-motion --project=no-webgl` passes for the routes that exist.

- `[ ]` U0.9 `[after U0.5]` TypeScript leg of the contract round-trip: a
  Vitest test reading `testdata/fixtures/contracts/command_intent.json`
  through the generated `CommandIntent` type and producing byte-identical
  canonical JSON, matching the Go and Python legs. Verify: `vitest run contract`.

### U1 — fleet and dispatch (runs during backend Wave 1)

- `[ ]` U1.1 `[after 0F.2]` Fleet command center: installed MW and MWh,
  dispatchable now and forecast, reserved for backup, device counts by
  operating state (on grid, off-grid outage, no home power, overcurrent,
  standby, telemetry unavailable) and by online, offline, degraded, stale,
  maintenance, plus communications and acknowledgement health. Each metric carries timestamp, provenance, and
  freshness. Vitest against the `GetFleetSummary` fixture. Verify: `vitest run fleet`.
- `[ ]` U1.2 `[P]` Dispatch request form: region, window, target MW, and the
  measurement boundary shown explicitly; Zod schema validates before submit.
  Verify: `vitest run dispatch-form`.
- `[ ]` U1.3 `[after BUILD_ORDER 1F.4]` Both views against recorded real
  responses through the mock server, then against `make demo`.
  Verify: `vitest run fleet dispatch` and a manual run against `make demo`.
- `[ ]` U1.4 `[after BUILD_ORDER 1E.5]` Approval screen: step-up
  confirmation, reason for every excluded device, plan version shown.
  Verify: `vitest run approval`.
- `[ ]` U1.5 `[after BUILD_ORDER 1F.3]` Demo-path steps 2, 3, 9, and 16
  (basic report) green against `make demo`. Verify: `playwright test demo-path` shows those steps passing.
- `[ ]` U1.6 `[P]` The `SIMULATED` badge on every simulated view; a Vitest
  test renders each existing page and asserts it. Verify: `vitest run simulated-badge`.
- `[ ]` U1.7 `[P]` The Living Grid ambient layer, version one: one persistent
  renderer for the fleet route, instanced cells from the H3 fixture, DPR
  capped at 1.5, paused when hidden, disposed on route exit, and the DOM
  fallback identical in meaning (golden "Default implementation
  constraints"). Verify: `playwright test living-grid` passes in normal, reduced-motion, and no-webgl projects and the route's critical JavaScript stays under the 250 KB gzip budget.

### U2 — live execution (runs during backend Wave 2)

- `[ ]` U2.1 `[after BUILD_ORDER 2F.3]` Live event page: sent, acknowledged,
  and delivered MW as three series with an uncertainty band, from the
  `WatchEvent` stream. Verify: `vitest run events-live`.
- `[ ]` U2.2 `[P]` Event timeline: every state transition, retry, and
  recovery decision with timestamp and reason. Verify: `vitest run events-timeline`.
- `[ ]` U2.3 `[P]` Emergency stop with step-up confirmation; "stop requested"
  until confirmation arrives. Verify: `vitest run emergency-stop`.
- `[ ]` U2.4 `[after BUILD_ORDER 2F.5]` Streaming wired through TanStack
  Query; new telemetry visible within 5 seconds; Playwright measures it.
  Verify: `playwright test telemetry-latency` reports under 5 s.
- `[ ]` U2.5 `[after BUILD_ORDER 2D.3]` Demo-path steps 10 to 15 green.
  Verify: `playwright test demo-path` shows 2, 3, 9 to 16 passing.
- `[ ]` U2.6 `[P]` Dispatch motion: the flow animation begins only after the
  approval response and shows "verified" only after telemetry, never before
  (golden anti-pattern list, last item). Verify: `playwright test dispatch-motion` asserts ordering against the fixture timeline.

### U3 — planning explanation (runs during backend Wave 3)

- `[ ]` U3.1 `[after BUILD_ORDER 3F.3]` Plan explanation panel: expected
  value, reserve held back, constraint margins, excluded devices with
  reasons, per-interval shortfall. Verify: `vitest run explanation`.
- `[ ]` U3.2 `[P]` Forecast charts with calibrated intervals for load, price,
  outage risk, availability, each labelled `forecast` or `modeled_estimate`
  with issue time. Red, amber, and green are never decorative. Verify: `vitest run forecast-charts`.
- `[ ]` U3.3 `[P]` Fallback banner naming the reason when `fallback=true`.
  Verify: `vitest run fallback-banner`.
- `[ ]` U3.4 `[after BUILD_ORDER 3D.4]` "Validate unsafe alternative" control
  showing returned violations. Verify: `vitest run unsafe-alternative`.
- `[ ]` U3.5 `[after BUILD_ORDER 3F.4]` Replay button and replay result view.
  Verify: `vitest run replay`.
- `[ ]` U3.6 `[after U3.4]` Demo-path steps 4 to 6, 8, and 17 green.
  Verify: `playwright test demo-path` shows all steps except 1 and 7 passing.

### U4 — map, member, report (runs during backend Wave 4)

- `[ ]` U4.1 `[after BUILD_ORDER 4C.3]` MapLibre map with the locally served
  style and basemap, H3 layer, and toggles for density, capacity, SOC bands,
  connectivity failures, outages, severe weather, active dispatch, price
  volatility, modeled constraints, candidate deployment regions. Drill-down
  market to feeder. Verify: `vitest run map && playwright test map`.
- `[ ]` U4.2 `[P]` Map privacy: the site popup has no address field and exact
  location appears only with `site_location`. Verify: `vitest run map-privacy`.
- `[ ]` U4.3 `[after BUILD_ORDER 4F.2]` Member status: operating state,
  state of energy, backup hours at current usage and at 750 W, the power-flow
  diagram, the recent grid-event list explaining why the battery did or did
  not participate, plan and reserve, savings and participation outcomes.
  One view per operating-state fixture. Verify: `vitest run member-status`.
- `[ ]` U4.4 `[P]` Plan selection with reserve, price, risk tradeoff, exact
  consent text and catalog version, and the `0%` explanation.
  Verify: `vitest run plan-select`.
- `[ ]` U4.5 `[P]` Travel Flex scheduling: start, end, timezone, credit shown,
  early-return button. Verify: `vitest run travel-flex`.
- `[ ]` U4.6 `[P]` Anomaly alert card with the fixed wording; a test asserts
  the strings. Verify: `vitest run anomaly-alert`.
- `[ ]` U4.7 `[after BUILD_ORDER 4F.4]` Event report and two-event comparison,
  provenance badges and `modeled` labels on every financial figure; partner
  variant with aggregates only. Verify: `vitest run report`.
- `[ ]` U4.8 `[after BUILD_ORDER 4F.3]` Command center context strip: weather
  alerts, outage risk, regional load, market prices, forecast grid value and
  verified event value, each with provenance and freshness. Demo step 1.
  Verify: `vitest run context-strip`.
- `[ ]` U4.9 `[after U4.5, U4.8]` Travel Flex Playwright flows and demo-path
  step 7; the full 17-step demo path green. Verify: `playwright test` all green.
- `[ ]` U4.10 `[after U4.9]` Golden review scorecard filled in with evidence
  (screenshots per viewport, reduced-motion and no-webgl runs, performance
  trace) and stored under `apps/console/tests/evidence/`. Verify: score at least 18 of 20 with no zero in truth, safety, accessibility, or fallback.

### U5 — polish (runs during backend Wave 5)

- `[ ]` U5.1 `[P]` Sentry and PostHog behind env flags; a Vitest test asserts
  analytics events contain no household identifiers, site IDs, or travel
  state (FULL_SPEC §11). Verify: `vitest run analytics-privacy`.
- `[ ]` U5.2 `[P]` Budgets re-verified on the final build: critical route
  JavaScript, frame time, and the adaptive quality tiers from the golden
  budgets table. Verify: `pnpm --dir apps/console build` size report and a Playwright performance trace saved to `tests/evidence/`.
- `[ ]` U5.3 `[after BUILD_ORDER 5F.3]` Console section of `README.md`
  commands executed and accurate.

## 6. Gates

The director runs `pnpm --dir apps/console playwright test demo-path` against
`make demo` at every backend gate from Gate 1 on. The steps expected green at
each gate:

| Gate | Steps green |
| --- | --- |
| 1 | 2, 3, 9, 16 |
| 2 | 2, 3, 9 to 16 |
| 3 | all except 1 and 7 |
| 4 | all 17 |

A Playwright failure at a gate is routed to whichever side owns the cause:
the UI track if the view is wrong, the backend lane if the response is wrong.

## 7. Working method

- Read the golden rules, the anti-patterns, and the scorecard before the
  first component. Design the static frame before any motion.
- Build against fixtures first, the mock server second, the real stack last.
  Do not wait for the backend to start a screen.
- Every screen ships with its Vitest test, its Playwright step, its
  reduced-motion run, and its no-webgl run.
- Never invent a field. If a screen needs data the fixture does not have,
  request it (§8) and render the honest gap meanwhile.

## 8. Communication

The UI agent appends one line to `.local/mailbox.log` (ignored by git) for
each event, in this form:

```text
ui: DONE <item> | <short sha> | <verify command> | <last line of output>
ui: REQUEST <item> | <method or field needed> | <why>
ui: BLOCKED <item> | <reason>
```

The director reads the mailbox, answers requests by opening a backend item,
and reports gate results back the same way with `director:` lines.
