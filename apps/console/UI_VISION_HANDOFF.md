# GridOS UI vision and implementation handoff

Updated: September 26, 2026, America/Chicago. Latest UI implementation commit: `2bf2527`.

This file was explicitly requested by the user. It records direction and the handoff snapshot; code, contracts, and fresh checks take precedence over its status statements.

## The assignment

Finish a polished, connected operator console in the visual direction of **Proposal B: Cinematic Living Grid**. The user wants the finished product, not another proposal or a collection of disconnected dashboard screens. They have repeatedly pointed out that the current implementation does not resemble the Proposal B images closely enough.

Do not describe the current UI as finished, fully polished, or matching B. It has working foundations and several verified slices, but the spatial narrative and complete operating loop remain unfinished.

Personally inspect browser screenshots before inviting the user to open a preview. Automated assertions alone are not visual QA. Keep working through defects found in screenshots; do not simply capture them and declare success.

## Visual north star

[Proposal B concept image](../../docs/design/assets/operator-console-cinematic.png) is the art-direction target. [Proposal A](../../docs/design/assets/operator-console-static.png) is the static truth frame and fallback, not the eventual substitute for B.

The experience should feel like one quiet energy control room: a large geographic field, a narrow operating-loop rail, restrained evidence on the right, and a persistent truth strip. Every route is another lens on the same fleet and event.

- Give the geographic field visual priority. Avoid burying it under stacks of metric cards.
- Use near-black green, precise typography, fine borders, and generous quiet space around the focal field.
- Keep supporting evidence flat and legible. Avoid nested cards, decorative neon, particles, glass, or ornamental status colors.
- Keep selection distinct from operational state. White selection does not mean healthy, approved, or delivering.
- Preserve the same visual identity across desktop, mobile, light theme, reduced motion, and WebGL failure.
- Use only contract-backed geography and state. Do not manufacture a contiguous Austin silhouette, terrain, roads, feeder lines, or city labels to imitate the generated picture.

### The same field tells the whole story

| Stage    | Intended spatial treatment                               | Required evidence                                 |
| -------- | -------------------------------------------------------- | ------------------------------------------------- |
| Observe  | Neutral geographic capacity field                        | H3 location, site count, installed capacity       |
| Forecast | Blue interval envelope                                   | Forecast issue time, model, interval, provenance  |
| Optimize | Eligible cohort and named exclusions                     | Versioned plan explanation and exclusion reasons  |
| Approve  | Governed, still field; unsafe regions locked out         | Safety result and approved version                |
| Dispatch | White command intent traces                              | `WatchEvent` reporting `SENT`                     |
| Verify   | Distinct blue acknowledgement and mint measured response | Per-H3 receipt and telemetry evidence             |
| Learn    | Commanded/measured residuals and synchronized replay     | Published report, seed, versions, ordered updates |

Approval and launch never trigger delivery animation. Acknowledgement is not delivery. Missing measurements remain unknown, not zero.

## Read before resuming

Follow the full reading order in [VISUAL_HANDOFF.md](../../docs/design/VISUAL_HANDOFF.md), especially:

1. [AGENTS.md](../../AGENTS.md): ownership, tests, commits, and enforced ceilings.
2. [Visual system](../../docs/design/visual-system.md): golden system, tokens, composition, rendering boundaries, and scorecard.
3. [Proposal descriptions](../../docs/design/operator-console-proposals.md) and both concept images.
4. [UI_TRACK.md](../../claude%20docs/UI_TRACK.md): UI items and acceptance requirements.
5. [ISSUES.md](../../claude%20docs/ISSUES.md) and [BUILD_ORDER.md](../../claude%20docs/BUILD_ORDER.md): dependencies and current backend ownership.
6. [FULL_SPEC.md](../../FULL_SPEC.md), [TECHSTACK.md](../../TECHSTACK.md), and [system understanding](../../docs/domain/system-understanding.md).

Before each item, read the latest 40 `director:` lines in `.local/mailbox.log`, addressing messages for the UI lane first. Older blocker reports may have been resolved.

## What exists today

| Area        | Implemented                                                                                                            | Important limits                                                                                            |
| ----------- | ---------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Shell       | Operating-loop rail, evidence rail, truth strip, light/dark themes, responsive composition                             | Several stages still have no complete screen                                                                |
| Fleet       | Timestamped capacity/health evidence, refresh, aggregate selection                                                     | Context strip and full geographic state encodings remain                                                    |
| Living Grid | Shell-owned lazy Three.js renderer, instanced H3 capacity, actual H3 boundaries, SVG/table fallback                    | Static capacity field, not the full B narrative; no terrain or dispatch propagation                         |
| Dispatch    | Request form, event review, separate approval and launch confirmations                                                 | Rich explanation, forecast intervals, unsafe-alternative view, and new step-up assertion integration remain |
| Live event  | Sent/acknowledged/measured series, semantic values, unknown gaps, audit timeline, emergency-stop request               | Per-H3 motion, fan-out, failure/recovery, reserve, and safe-return evidence remain                          |
| Map         | `/map`, local MapLibre style, fleet H3 polygons, installed-MW/site-count controls, cell inspection, SVG/table fallback | Uses `FleetService.ListSites`, not the complete GeoService layers or hierarchy yet                          |
| Report      | Basic server event accounting and provenance                                                                           | Published reports, comparison, economics, and replay remain                                                 |
| Member      | Role boundary                                                                                                          | Full household, plan, Travel Flex, and anomaly views remain                                                 |

The map parks the Living Grid component, disposes/releases its WebGL context, and restores its renderer on return. The browser check verifies the previous context is lost and only one canvas remains. Event context stays in the shell.

Live-response history retains a bounded session window, not the complete event history. Device uncertainty bounds are explicitly not presented as a fleet confidence interval.

## Current backend integration facts

- At this handoff check, the UI at `http://127.0.0.1:3000` returned HTTP 307 and the control basemap at `http://127.0.0.1:28080/geo/style.json` returned HTTP 200. These are availability checks, not a new visual or end-to-end pass.
- Shared-stack rebuilds have caused temporary outages. Do not restart the director-owned stack or report an old outage as a permanent blocker.
- The local basemap contains a public Census Texas outline and simulated zone rectangles. It has no terrain or street tiles. The map currently hides the simulated zone boundaries and explains why.
- The live fleet uses privacy-merged H3 cells of different resolutions. Do not assume resolution 7, 320 cells, equal footprints, or the concept image's silhouette. An earlier live map inspection saw 206 regions; that count is not an invariant.
- Geo contract item 4C.5 has added `as_of` and aggregate metadata to cells, hierarchy nodes, and responses in `contracts/gridos/v1/geo.proto`. Recorded fixture refresh was still being corrected to include real telemetry freshness at this handoff. Confirm its DONE/VERIFIED message before consuming the new fixtures.
- Context, explanation, unsafe-alternative, report, replay, and member contracts/fixtures have advanced substantially beyond the UI. Inspect current protobufs and fixtures rather than inventing temporary data models or treating those screens as generally backend-blocked.

### Step-up is the immediate safety integration

The director's latest decision is **server-side signing only**:

1. In local auth mode, call the mock identity service at `POST /local/step-up` with `X-GridOS-Role` and JSON `{action, event_id, plan_version}`.
2. Read the response `{assertion}`.
3. Send that value as `X-GridOS-Step-Up` on `ApproveEvent` or `EmergencyStop`.

The endpoint is implemented in `tools/development/mockapi/main.go`. It is not automatically served by control on port 28080. Determine and configure the identity-service origin/proxy explicitly; the console currently proxies only `/rpc` and `/geo` to control.

Never put `GRIDOS_STEP_UP_KEY` in browser code, a Vite public environment variable, or a bundle. Production uses identity-provider assertions, not the local stub. Preserve idempotency and unknown-outcome handling when adding assertions. Notify the director when the UI integration is verified so demo enforcement can be enabled. The director is deliberately holding that rollout for the console.

## Recommended next work

1. Integrate server-issued step-up assertions with focused RED/GREEN tests for approval and stop, including denied/failed assertion acquisition. This closes a safety gap before further live approval testing.
2. Complete plan explanation, forecast evidence, constraint margins, and unsafe-alternative validation from current recorded contracts. Give these a coherent layout around the persistent field.
3. Complete command fan-out, scenario failures, recovery decisions, reserve protection, and expiry/safe-return evidence. Add per-H3 sent/acknowledged/measured encodings only from validated stream facts.
4. Upgrade the map to timestamped GeoService data, resolution-aware aggregation, hierarchy drilldown, and evidenced operational/context layers after the refreshed fixtures land.
5. Build published report/comparison and replay with one shared replay timestamp. Finish member status, plans, Travel Flex, and exact anomaly wording.
6. Perform the full Proposal B composition/motion pass across the completed operating loop. The map route alone is not fulfillment of the cinematic vision.

Do not satisfy demo assertions with empty regions or decorative headings. Each section must expose the actual supporting facts, provenance, timestamps, failures, and usable controls.

## Demo acceptance snapshot

Latest recorded director demo-path result: **2, 3, 9, 13, 16 green** at their existing scope. No fresh full demo-path run was made for this document.

| Red steps | Missing UI work                                                  |
| --------- | ---------------------------------------------------------------- |
| 1         | Regional context strip                                           |
| 4–6       | Forecast intervals, optimization explanation, constraint margins |
| 7         | Travel Flex eligibility/member experience                        |
| 8         | Unsafe-alternative validation                                    |
| 10–12     | Fan-out, scenario failures, recovery decisions                   |
| 14–15     | Reserve protection and safe return                               |
| 17        | Deterministic replay                                             |

Backend item 2A.10 is also adding launch-triggered live scenario timing. Recheck its state before claiming the seeded failure story in steps 11–15 is end-to-end ready.

## Verification already performed

These are historical focused results, not a full repository or release gate:

| Command                                                                                                 | Recorded result                                                                                                                         |
| ------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `pnpm --dir apps/console vitest run map-data`                                                           | 2 tests passed; last line `Duration 1.15s` with timing breakdown                                                                        |
| `pnpm --dir apps/console vitest run living-grid shell`                                                  | 4 tests passed; last line `Duration 2.24s` with timing breakdown                                                                        |
| `pnpm --dir apps/console playwright test map.spec.ts`                                                   | Latest full map run: 3 passed, 1 failed on failure-state ARIA; corrected afterward                                                      |
| `pnpm --dir apps/console playwright test map.spec.ts -g 'light theme'`                                  | Corrected failure case: last line `1 passed (13.4s)`                                                                                    |
| `pnpm --dir apps/console build`                                                                         | Exit 0; last build line `✓ built in 279ms`; subsequent change only removed an invalid host ARIA label                                   |
| Exact-path GREEN commit `2bf2527`                                                                       | Hook: 16 related test files, 45 tests passed; last test line `Duration 2.68s` with timing breakdown; secret scan and lint/format passed |
| `pnpm --dir apps/console playwright test --config playwright.live.config.ts -g 'receives event stream'` | Earlier real-demo check: last line `1 passed (7.0s)`; created a simulated plan, launched no commands                                    |

Map screenshots were personally inspected at 390 and 1440 px, with dark/light, no-WebGL, and basemap-failure states. Review found and fixed a blank rectangle caused by a caption overlay, poor mobile framing, inaccurate map evidence copy, and invalid ARIA after map teardown.

Screenshots under `test-results/` are temporary: subsequent Playwright runs replace them. Do not cite a deleted capture as an available artifact. At handoff, `test-results/map-light-basemap-failure.png` exists. Capture fresh evidence for the next slice.

The build emits size warnings for lazy spatial chunks. MapLibre and its worker are lazy-loaded; the full critical-JavaScript and final performance budgets still require explicit release verification. Do not silence warnings or relax ceilings.

## Local workflow and ownership

Work only in `apps/console/`, plus append-only UI coordination in `.local/mailbox.log`. Preserve the shared tree's unrelated changes. Do not edit backend, contracts, root docs, hooks, or infrastructure without the director's path grant. This handoff lives inside UI ownership.

If no console dev server is running, start it with:

```sh
GRIDOS_AUTH_MODE=local GRIDOS_API_URL=http://127.0.0.1:28080 pnpm --dir apps/console dev
```

Check the existing listener first; do not create duplicate preview servers. Standard Playwright tests start their own server on 3100. The live config targets the standing preview on 3000.

Useful entry points:

- [Console and persistent state](src/console.tsx), [shell](src/shell.tsx), [design tokens](src/tokens.css).
- [Living Grid](src/fleet/living-grid.tsx), [renderer](src/fleet/renderer.ts), [projection validation](src/fleet/scene.ts).
- [Map view](src/map/map.tsx), [map renderer](src/map/map-renderer.ts), [map tests](tests/map.spec.ts).
- [API client](src/api/client.ts), [auth](src/api/auth.tsx), [evidence validation](src/api/Provenance.tsx).
- [Live event](src/events/events-live.tsx), [audit timeline](src/events/audit-timeline.tsx), [stop control](src/events/emergency-stop.tsx).
- [Approval](src/dispatch/approval.tsx), [canonical demo acceptance](tests/demo-path.spec.ts).

Use exact-path commits, separate RED/GREEN commits where required, zero code comments/docstrings, no TypeScript `any`, and no hook bypasses. An index lock may belong to another agent; never delete it. Run focused checks for changed work; the director owns full wave gates.

## What “finished” must mean

All 17 demo steps work with meaningful evidence, including reduced motion and no WebGL. The complete experience visibly follows Proposal B's hierarchy and causal continuity. There is one active renderer, no unsupported state encoding, no private-location leakage, and no safety control dependent on canvas interaction. Desktop, tablet, short-screen, mobile, theme, failure, and keyboard states have fresh screenshot evidence. The golden review scores at least 18/20 with no zero in truth, safety, accessibility, or fallback.

Do not open the user's preview as a finished-product reveal until those claims are actually supported.
