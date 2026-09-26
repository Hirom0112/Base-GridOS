# 3D Web Logs

## Three-site animation teardown and reusable build prompts

Primary targets: [heyaristotle.com](https://www.heyaristotle.com/), [meermohsin.me](https://www.meermohsin.me/), and [3DCC Core](https://cocktailtheory.github.io/3DCC-core/)  
Inspected: September 26, 2026  
Purpose: explain three distinct approaches to dimensional web experiences and provide copy-ready prompts for building original work with the same classes of techniques.

> Important: reproduce engineering patterns, not any target’s branding, composition, copy, characters, models, dataset, interaction concept, or artwork. Use original art direction and licensed assets.

---

# Golden system for the GridOS web experience

## The pattern found across the research

The strongest dimensional sites do not win by maximizing 3D. They make five disciplined choices:

1. **One spatial metaphor** explains the experience.
2. **One normalized motion source** coordinates the camera, objects, and DOM.
3. **One visual focal point** dominates each viewport.
4. **Real DOM owns meaning and action; rendering owns atmosphere and spatial context.**
5. **A bounded palette, material system, and motion grammar** make every scene feel related.

The failures are equally consistent: unrelated effects, several competing canvases, text trapped in WebGL, unclear navigation, excessive glass and bloom, no reduced-motion state, and no performance ceiling.

For GridOS, the resulting concept is **The Living Grid**: a calm control-room interface in which thousands of batteries appear as an aggregated field of energy, flowing through the operating loop:

```text
Observe -> Forecast -> Optimize -> Approve -> Dispatch -> Verify -> Learn
```

The spatial layer communicates scale, flow, health, uncertainty, and cause-and-effect. The operator console remains precise, legible, auditable, and safe.

## The 15 golden rules

### 1. One metaphor: a living but governed grid

Represent the fleet as a field of cells, nodes, or regional clusters connected by restrained energy paths. Capacity can brighten or rise; uncertainty can soften boundaries; quarantined devices can detach or dim. Do not mix this with planets, generic particles, floating glass sculptures, and unrelated sci-fi motifs.

### 2. Truth before spectacle

Every dramatic visual must correspond to a real, derived, or explicitly simulated state. Preserve the repository’s data-truth classes in the UI. Never let animation imply that power was delivered when only a command or acknowledgement exists.

### 3. The DOM owns every consequential action

Approval, dispatch, pause, terminate, reserve policy, exception review, and audit navigation must be semantic HTML controls. WebGL may visualize the consequence and provide supplementary selection, but it is never the only path to an operation.

### 4. One canvas, one clock, one scroll truth

Use at most one persistent spatial canvas for a view. Drive it from one normalized progress/state source. Do not let GSAP, React state, a smooth-scroll library, and independent render loops compete to define position or time.

### 5. 3D earns its cost

Use 3D only for relationships that are materially clearer in space: fleet scale, regional aggregation, changing topology, dispatch propagation, or command-versus-response. Use ordinary charts, maps, tables, and forms for exact comparison and control.

### 6. One focal event per viewport

At any moment, only one element may perform a large transform, emit a strong glow, or command the highest contrast. Everything else supports it through lower amplitude, lower saturation, or slower movement.

### 7. Scroll tells causality, not decoration

For a public narrative page, map chapters to the operating loop. Scroll should explain how observation becomes a safe, verified event. It must scrub backward correctly, tolerate refresh at any position, and never require scroll for an operator workflow.

### 8. Motion is damped and interruptible

Input sets targets; the render loop interpolates toward them. Avoid direct pointer-to-transform mapping, uninterruptible entrance sequences, and cinematic delays on task-critical views. A user action always overrides ambient motion.

### 9. Depth has an information hierarchy

Use three consistent planes:

- **Near:** controls, decisions, alarms, and active selection.
- **Middle:** charts, map clusters, event state, and comparisons.
- **Far:** ambient grid topology, atmosphere, and context.

Near layers are sharp and high contrast. Far layers are quieter, lower contrast, slower, and never intercept input.

### 10. Light communicates state

Glow is not decoration. Mint indicates healthy/available energy, amber means attention or uncertainty, red means blocked/unsafe/failure, and blue indicates forecast or informational state. Decorative light must remain neutral and subordinate so status colors retain meaning.

### 11. Restraint creates the premium look

Prefer large areas of quiet near-black green, thin structural lines, crisp typography, a single luminous field, and infrequent high-energy moments. Avoid glass on every panel, rainbow gradients, thick neon outlines, constant particle motion, and excessive bloom.

### 12. Design the static frame first

Every state must communicate correctly as a still image before animation is added. Reduced motion uses these intentionally composed static states; it is not an animation frozen halfway through.

### 13. Performance is part of the art direction

Set budgets before asset production. Cap device-pixel ratio, compress geometry and textures, instance repeated objects, pause offscreen work, and provide quality tiers. A stable, quiet 45–60 fps composition is more convincing than a complex scene that stutters.

### 14. Degrade without losing the story

If WebGL fails, show the same information through the map, charts, summary metrics, and a poster or CSS field. If scroll timelines are unsupported, use a stable composition and normal document flow. No safety or operational capability depends on graphical enhancement.

### 15. Every scene has acceptance evidence

Define viewport screenshots, state fixtures, keyboard paths, reduced-motion captures, console-error checks, performance traces, and fallback captures before calling an experience complete.

## Golden visual language

### Design character

```text
calm infrastructure
high-trust control room
quietly cinematic
technical but humane
precise, not sterile
luminous, not neon
dense when operating; spacious when explaining
```

Avoid “generic cyberpunk dashboard.” The look should suggest dependable energy infrastructure observed at night: charcoal-green surfaces, soft phosphor light, exact geometry, and brief pulses that reveal energy movement.

### Color tokens

| Token | Value | Use |
|---|---:|---|
| `--ink-0` | `#050907` | Page and canvas void |
| `--ink-1` | `#09120E` | Primary application background |
| `--ink-2` | `#0E1B15` | Raised panels |
| `--ink-3` | `#16271F` | Borders, grid lines, inactive tracks |
| `--paper-0` | `#F0F7F2` | Primary text and decisive values |
| `--paper-1` | `#B9C9BF` | Secondary copy |
| `--paper-2` | `#7F9588` | Labels and inactive metadata |
| `--energy` | `#66F2A4` | Healthy capacity, available energy, primary accent |
| `--forecast` | `#6DB8FF` | Forecasts, projections, informational state |
| `--attention` | `#F2BC57` | Uncertainty, review, warning |
| `--critical` | `#FF6B68` | Unsafe, failed, blocked; never decorative |
| `--offline` | `#66736C` | Stale, unavailable, or disconnected |

Rules for using the palette:

- The interface is mostly ink and paper; status hues occupy a small fraction of the screen.
- Use `--energy` as the single brand accent outside status contexts.
- Never rely on hue alone; pair it with an icon, label, pattern, or shape.
- Reserve red exclusively for a condition requiring intervention or preventing action.
- Use gradients only to describe a real transition such as low-to-high capacity or forecast confidence.

### Typography

- **UI and display:** a precise grotesk such as Geist or Inter.
- **Telemetry and identifiers:** IBM Plex Mono or Geist Mono.
- Use no more than two font families and four weights.
- Headlines are compact and sentence case, not wide all-caps slogans.
- Numeric values use tabular figures.
- Uppercase is limited to short labels, with generous tracking.
- Operational copy says what happened, why it happened, and what the operator can do next.

Suggested scale:

| Role | Size | Line height | Notes |
|---|---:|---:|---|
| Narrative display | `clamp(3rem, 8vw, 8rem)` | `0.9–0.96` | Marketing/story pages only |
| Page title | `2rem–3rem` | `1.05` | One per view |
| Section title | `1.25rem–1.5rem` | `1.2` | Quiet, compact |
| Body/UI | `0.875rem–1rem` | `1.45–1.6` | Never below 14px for primary UI |
| Telemetry | `0.75rem–0.875rem` | `1.3` | Mono, tabular figures |

### Shape, border, and surface rules

- Use a restrained radius scale: `6px`, `10px`, and `16px`; no pill-shaped containers except compact status chips.
- Panels use a solid dark surface with one-pixel borders. Blur is optional and localized, never the default material.
- Use shadows sparingly; define elevation primarily through surface tone and edge light.
- Grid lines are low contrast and purposeful. They align content or encode scale.
- Icons use a consistent 1.5px stroke and simple geometric construction.
- Reserve irregular or organic forms for the energy field itself, creating contrast with the rational interface.

### Composition

The operator console uses a 12-column grid with a stable command rail, a large situational canvas/map, and an evidence panel. A recommended desktop composition is:

```text
┌──────────────┬──────────────────────────────────┬──────────────────┐
│ command rail │ fleet map / spatial state        │ event evidence   │
│ status       │                                   │ forecast         │
│ event steps  │ one selected region or event     │ exclusions       │
│ controls     │ carries the visual focus          │ audit trail      │
└──────────────┴──────────────────────────────────┴──────────────────┘
```

On smaller screens, collapse to normal vertical flow in this order: event truth, critical state, primary action, map/visualization, evidence, secondary controls. Never shrink the desktop control room into illegibility.

## Motion grammar

### Timing tokens

| Motion class | Duration | Easing | Use |
|---|---:|---|---|
| Immediate feedback | `80–120ms` | linear or ease-out | Press, toggle, focus acknowledgement |
| UI transition | `160–240ms` | `cubic-bezier(.2,.8,.2,1)` | Panel, tab, disclosure |
| State transition | `320–480ms` | damped ease | Selection, map regrouping, chart update |
| Spatial chapter | `700–1200ms` equivalent | spring/damped | Narrative camera or topology change |
| Ambient cycle | `6–16s` | sine-like | Very subtle breathing or flow |

### Motion rules

- Transform and opacity are the default animated properties.
- Camera motion and object motion do not peak at the same time.
- Scroll position controls primary progress; scroll velocity may control only small secondary deformation.
- Energy paths pulse only while power is scheduled or measured to flow.
- An acknowledgement may create a short node response; verified delivery creates the stronger response.
- Errors stop or break flow rather than merely changing its color.
- Hover movement stays within 2–6px or 1–3 degrees.
- Parallax amplitude decreases from foreground to background and is disabled on touch unless it serves a clear purpose.
- Reduced motion removes camera travel, morphing, velocity deformation, and ambient loops while retaining state changes.

## Spatial storytelling sequence

For a public or demo narrative, use a single pinned sequence with seven reversible chapters:

| Chapter | Spatial action | DOM message |
|---|---|---|
| Observe | Individual cells resolve into a regional field | Current fleet state and provenance |
| Forecast | A translucent future envelope expands around the field | Demand, risk, availability, uncertainty |
| Optimize | Eligible cells organize into a dispatch shape | Constraints and chosen cohort |
| Approve | Unsafe cells lock out; the plan pauses visibly | Independent safety gate and operator decision |
| Dispatch | A controlled wave travels only through approved nodes | Versioned commands and deadlines |
| Verify | Commanded and measured forms separate, then reconcile | Telemetry proves delivery; ACK does not |
| Learn | Residual differences become the next forecast signal | Replay, audit, and model improvement |

Keep the narrative copy in DOM sections beside or above the canvas. Each chapter must also make sense without animation.

## Rendering and implementation boundary

### Use CSS/DOM for

- Navigation, headings, body copy, buttons, forms, alerts, tooltips, tables, and audit logs.
- Simple layer parallax, reveals, sticky storytelling, and status transitions.
- Any view whose spatial content can be communicated through a map or chart.

### Use Canvas/WebGL for

- Large instanced fleets, topology, spatial aggregation, controlled flow, and transitions that require a camera.
- Ambient depth behind a semantic narrative.
- Supplementary direct manipulation when an equivalent DOM path exists.

### Default implementation constraints

- One persistent renderer per route.
- Device-pixel ratio capped at `1.5` by default and `2` only on a high-quality tier.
- Adaptive quality tiers for antialiasing, postprocessing, particle/node count, and shadows.
- Instancing for repeated batteries, homes, nodes, or cells.
- Draco or Meshopt for geometry; KTX2/Basis for substantial textures.
- No continuous React state updates from the render loop.
- Pause or throttle when offscreen, hidden, or in low-power/reduced-motion conditions.
- Dispose geometry, textures, materials, observers, and animation handles on route exit.
- A DOM/CSS fallback presents the same operational truth.

## Performance and accessibility budgets

These are starting acceptance budgets, not aspirations:

| Budget | Target |
|---|---:|
| Initial critical route JavaScript | `< 250 KB` gzip, excluding lazy 3D route |
| Initial narrative imagery | `< 1.5 MB` compressed |
| Initial 3D payload | `< 3 MB` compressed; stream/lazy-load the rest |
| Total decoded texture memory | `< 128 MB` desktop, `< 64 MB` mobile |
| Draw calls | `< 100` desktop, `< 60` mobile |
| Sustained frame rate | `>= 55 fps` target desktop; `>= 30 fps` minimum supported mobile |
| Main-thread long tasks | No repeated tasks over `50ms` during interaction |
| Layout shift | `CLS < 0.1` |
| Keyboard path | Every consequential task operable without canvas interaction |
| Contrast | WCAG AA for text and controls |

## Golden component styling

### Primary action

Solid `--energy` background, dark ink text, 8–10px radius, strong visible focus ring, and no glow until hover/focus. Destructive actions are never styled as the primary positive action and always require explicit confirmation language.

### Panels

Use `--ink-2`, a one-pixel `--ink-3` border, and 16–24px internal spacing. Panel titles state the object and time context. Avoid nesting more than two panel levels.

### Status chips

Use a small icon or pattern plus label, not color alone. Prefer direct words: `Available`, `Forecast`, `Needs review`, `Unsafe`, `Stale`, `Simulated`.

### Charts

Use direct labels where possible. Commanded power and measured delivery must be visually distinct by both line style and color. Confidence intervals are translucent regions, not extra lines. Tooltips include timestamp, units, provenance, and whether the value is observed, derived, or simulated.

### Map and spatial nodes

Aggregate before rendering. Never expose household coordinates. Node size represents one named measure only; color represents one named status only. Selection adds an outline and label rather than only increasing glow.

### Alerts

State the condition, operational consequence, evidence, and next safe action. Avoid vague messages such as “Something went wrong.” Critical alerts remain visible until resolved or acknowledged according to policy.

## Anti-patterns: automatic rejection

- Critical text or controls rendered only inside WebGL.
- More than one decorative canvas competing for GPU resources.
- Smooth scrolling added to the operator console.
- Red, amber, or green used decoratively.
- Continuous bloom, chromatic aberration, film grain, or camera shake.
- More than one large motion event at the same time.
- Fake live telemetry or unlabeled simulated values.
- Particle counts or texture sizes chosen without a device budget.
- A loading gate that blocks useful HTML while optional 3D assets download.
- Mobile treated as a scaled desktop scene.
- Reduced motion implemented only by increasing duration.
- A dispatch animation that begins before approval or appears verified before telemetry arrives.

## Golden review scorecard

Score each category from 0 to 2. A release must score at least 18/20 and may not score 0 in truth, safety, accessibility, or fallback.

| Category | 0 | 1 | 2 |
|---|---|---|---|
| Truth | Visual state can misrepresent data | Mostly accurate with ambiguous moments | Every state and provenance is explicit |
| Safety | Controls/visuals can imply unsafe action | Guarded but weakly explained | Safety boundary is visible and enforced |
| Focus | Several elements compete | Hierarchy exists but drifts | One clear focal event per viewport |
| Coherence | Effects feel unrelated | Mostly consistent | One metaphor, material, and motion language |
| Legibility | Visuals compromise reading | Readable with weak states | Clear across breakpoints, zoom, and themes |
| Motion | Decorative or hard to interrupt | Mostly purposeful | Causal, damped, reversible, interruptible |
| Performance | No budget or visible jank | Meets minimum on tested devices | Meets budgets with adaptive quality |
| Accessibility | Canvas is required | Fallback exists with gaps | Equivalent semantic and keyboard path |
| Fallback | Failure blocks meaning/action | Static fallback is partial | Full truth and core tasks survive |
| Evidence | Judged by impression | Some screenshots/tests | Fixture, viewport, trace, and fallback proof |

## Copy-ready golden art-direction prompt

```text
Design and implement the GridOS web experience as a calm, high-trust energy control room called “The Living Grid.” The interface coordinates a distributed fleet of residential batteries while protecting household backup reserve. It must feel precise, quietly cinematic, and operationally credible—not like a generic cyberpunk dashboard.

Use near-black charcoal-green backgrounds, solid dark panels, one-pixel structural borders, warm off-white typography, and a restrained phosphor-mint accent. Reserve blue for forecasts, amber for review/uncertainty, and red exclusively for unsafe or blocked states. Use a precise grotesk for UI and a mono face with tabular figures for telemetry. Avoid rainbow gradients, glass on every panel, thick neon edges, constant particles, excessive bloom, and decorative status colors.

Build around one spatial metaphor: an aggregated living grid of regional energy cells and restrained connection paths. The spatial layer may show fleet scale, capacity, uncertainty, topology, dispatch propagation, and commanded-versus-measured response. It must never be the sole representation of data or the sole path to an action.

Keep navigation, headings, copy, charts, tables, alerts, forms, approvals, dispatch controls, and audit evidence in semantic DOM. Use at most one WebGL canvas, one animation clock, and one normalized progress/state source. Critical operator workflows use native interaction and must never depend on scroll.

For the narrative sequence, tell seven reversible chapters: Observe, Forecast, Optimize, Approve, Dispatch, Verify, and Learn. Each chapter has one focal transformation and matching DOM explanation. Acknowledgement proves receipt; measured telemetry proves delivery. Never animate a plan as dispatched before approval or as verified before measurements arrive.

Provide desktop, tablet, short-screen, and mobile compositions; keyboard and screen-reader paths; reduced-motion static states; WebGL and asset-failure fallbacks; adaptive quality tiers; explicit payload/draw-call/texture budgets; and screenshots at named states. The page must remain useful before the optional 3D scene loads.
```

---

## Case study 1: Aristotle — native CSS 2.5D scroll storytelling

### Executive finding

The homepage’s main effect is **not a live Three.js/WebGL scene**. It is a carefully constructed **2.5D compositing system**:

1. A tall `255svh` sequence creates scroll distance.
2. A `100svh` stage remains `position: sticky; top: 0`.
3. A named native CSS View Progress Timeline (`--hero-sequence`) maps section visibility to animation progress.
4. Layered AVIF/WebP artwork, transparent figures, cloud images, masks, text panels, shadows, and a doorway frame are animated mostly with `transform`, `opacity`, and `visibility`.
5. The foreground doorway scales from `1` to about `5.5`, which creates the sensation that the camera travels through it.
6. A separate landscape layer only scales to about `1.12`, producing depth through differential motion.
7. Later sections use pointer parallax, perspective letter reveals, intersection-triggered entrances, marquees, and a multilayer footer.

This is a strong choice for an illustration-led marketing page: it looks cinematic, downloads static compressed images, stays compatible with semantic HTML, and avoids the rendering cost and complexity of a full 3D engine.

## Confidence key

- **Verified**: directly visible in the live response, response headers, HTML, CSS, or JavaScript bundle.
- **Inferred**: a conclusion drawn from verified implementation evidence.
- **Not detected**: no recognizable production signature appeared in the homepage assets inspected; this is not proof that the company never uses it elsewhere.

## Verified technology stack

| Layer | Finding | Evidence / confidence |
|---|---|---|
| Application framework | Next.js App Router-style React application | `/_next/` assets, React Server Component payload, Next router code; verified |
| Next.js version in shipped runtime | `16.1.3` | Runtime version string in the public JS bundle; verified for the inspected build |
| React version in shipped runtime | `19.3.0-canary-f93b9fd4-20251217` | Renderer version string in the public JS bundle; verified for the inspected build |
| Build system | Turbopack | Shipped `turbopack-*.js` runtime chunk; verified |
| Hosting / CDN | Vercel | `server: Vercel`, `x-vercel-*`, `x-nextjs-prerender`, and cache headers; verified |
| Rendering | Statically prerendered homepage with client hydration | `x-nextjs-prerender: 1`, RSC data, hydrated client components; verified |
| Core animation | Native CSS Scroll-driven Animations / View Timeline | `view-timeline-name`, `animation-timeline`, `animation-range`, CSS keyframes; verified |
| Supporting motion | Web Animations API, `requestAnimationFrame`, CSS transitions/keyframes | Bundle calls `Element.animate()`, rAF, observers; verified |
| Visibility triggers | `IntersectionObserver` | Reveal and parallax activation code; verified |
| Visual media | Layered PNG plus responsive AVIF/WebP raster assets, inline SVG, one MP4 | Public media paths and `<picture>` output; verified |
| Image delivery | Next.js image optimization plus hand-authored responsive `<picture>` sources | `/_next/image` and direct AVIF/WebP source sets; verified |
| Fonts | Self-hosted Bogue Regular/Medium and Mulish Variable Latin | Preloaded local WOFF2 files; verified |
| Product analytics | PostHog, Google tag/Ads, Meta Pixel, OpenAI Ads pixel | Runtime integrations in shipped bundle; verified |
| Monitoring | Sentry | Browser SDK and production ingest configuration in bundle; verified |
| Consent/privacy | Iubenda | Consent runtime and public widget/policy links; verified |
| Performance analytics | Vercel Web Analytics/Insights code is bundled | `/_vercel/insights/script.js` loader; verified as bundled |

### Tools not detected on this homepage

No recognizable homepage-production signature was found for:

- Three.js
- React Three Fiber / Drei
- WebGL renderer setup
- GSAP or ScrollTrigger
- Lenis
- Locomotive Scroll
- Spline
- Lottie
- Rive `.riv` assets/runtime initialization

The page preserves normal browser scrolling. That matters: the “smoothness” comes from compositor-friendly native scroll animation, not from hijacking the scroll position.

## How the hero works

### 1. Scroll distance and the pinned stage

The outer sequence is `height: 255svh`. Its child stage is `height: 100svh`, `position: sticky`, and pinned to `top: 0`. As the remaining `155svh` passes, the viewport appears to stay inside one scene while scroll progress drives the choreography.

Conceptual structure:

```html
<section class="hero-sequence">
  <div class="hero-stage">
    <div class="landscape-layer">...</div>
    <div class="doorway-mask">...</div>
    <div class="doorway-zoom-layer">...</div>
    <div class="people-layer">...</div>
    <div class="intro-copy">...</div>
    <div class="manifesto-panels">...</div>
  </div>
</section>
```

```css
.hero-sequence {
  height: 255svh;
  view-timeline-name: --hero-sequence;
  view-timeline-axis: block;
  position: relative;
  overflow: clip;
}

.hero-stage {
  height: 100svh;
  min-height: 42rem;
  position: sticky;
  top: 0;
  overflow: hidden;
  isolation: isolate;
}
```

### 2. Native scroll scrubbing

The outer section supplies a named view timeline. Descendants attach keyframes to it:

```css
@supports (animation-timeline: --hero-sequence) {
  .doorway-zoom-layer {
    animation: portal-zoom 1ms linear both;
    animation-timeline: --hero-sequence;
    animation-range: entry 100% exit 0%;
  }
}
```

The nominal `1ms` duration is not the playback time. Once `animation-timeline` is assigned, scroll progress becomes the clock. Scrolling backward also reverses the animation naturally.

### 3. The fake camera move

The visual “camera” move is differential scaling:

- Doorway/foreground: approximately `scale(1)` → `scale(1.08)` → `scale(5.5)`.
- Landscape/background: approximately `scale(1)` → `scale(1.12)`.
- People fade out early.
- Foreground UI disappears before the zoom finishes.
- A dark outside-the-door mask fades away as the portal fills the viewport.
- The scale origin is aligned with the perceived vanishing point/opening, not simply dead center.

That difference in scale rate is what sells depth. It is closer to multiplane animation than a real 3D camera.

### 4. Nine-slice artwork construction

The doorway is not one ordinary full-screen background. The layout uses a three-by-three grid and crops a large source image into independently sized regions. CSS custom properties represent each slice’s source coordinates. Small overlaps and gradient mask feathering hide seams.

Why use this technique:

- The central arch/opening can keep a controlled aspect ratio.
- Side walls can expand to cover very wide screens.
- The floor and top can resize independently.
- The opening stays aligned with figures, copy, and zoom origin.
- It avoids stretching the entire illustration uniformly.

### 5. Layer inventory

The observed hero is composed from assets and generated effects, not a model:

- Large painted doorway “nine-slice” image in AVIF/WebP.
- Separate full-bleed landscape image in multiple resolutions.
- Separate transparent figures in multiple resolutions.
- Multiple cloud sprites.
- CSS-generated cast shadow using the figures image as a mask.
- CSS contact shadow using a blurred gradient ellipse.
- Intro headline, CTA, and statistics as real HTML.
- Two later text panels as real HTML.
- Inline SVG sketch used during image loading.
- Full-screen color cover faded after artwork paints successfully.

### 6. Timeline choreography

Approximate stages extracted from the shipped keyframes:

| Scroll timeline | Visual action |
|---:|---|
| `0–10%` | Doorway begins a gentle push from scale `1` to `1.08` |
| `0–18%` | Figures remain present |
| `~13–28%` | Outside mask releases/fades |
| `~18–30%` | Figures depart |
| `~0–25%` | Intro foreground remains, then disappears |
| `10–48%` | Main portal push accelerates toward scale `5.5` |
| `36–65%` | First manifesto panel is visible |
| `~67%` | First panel moves upward and fades |
| `~71–100%` | Second manifesto panel becomes visible |
| `~75–81%` | Doorway layer hides after it has filled the view |
| `36–81%` | Background landscape slowly enlarges to about `1.12` |

The implementation intentionally completes major visual changes before the very end of the view timeline, so the user sees a stable final state before leaving the section.

## Supporting motion systems

### Pointer-based 2.5D parallax

Illustration layers are grouped as `back`, `mid`, and `front`. Mouse position is normalized from `-1` to `1` on both axes, then written to CSS variables:

| Plane | Horizontal travel | Vertical travel |
|---|---:|---:|
| Back | about `-10px` | about `-8px` |
| Mid | about `+7px` | about `+6px` |
| Front | about `+18px` | about `+14px` |

The direction reversal on the back layer creates counter-parallax. Updates are capped to one `requestAnimationFrame`. The effect only runs while its section intersects the viewport, only for a mouse pointer with no button pressed, and resets on pointer leave, blur, intersection exit, or reduced-motion changes.

### Character-by-character reveal

Large statements are split into words and characters while preserving word wrapping. Each character receives its own drift, lift, tilt, timeline, and range. The starting transform combines:

```css
opacity: 0;
transform: translate3d(var(--drift), calc(-1 * var(--lift)), 0)
  rotateX(72deg)
  rotateZ(var(--tilt))
  scale(.98);
```

The parent provides roughly `24rem` perspective. This is a CSS 3D transform on DOM text, not a 3D scene.

### Reveal-on-entry

General content reveals use `IntersectionObserver` with about a `0.15` threshold and a negative bottom root margin. Group children get roughly `100ms` incremental delays. The observer unobserves completed targets, avoiding permanent scroll work.

### Clouds

Cloud images have two motions layered together:

- Slow time-based horizontal drift with long durations and negative delays, so the sky looks already in motion on load.
- Scroll-linked vertical rise tied to the hero timeline, creating additional perceived depth during the push-in.

### Footer parallax and overscroll

The footer defines its own named view timeline. Distance hills, midground hills, the giant wordmark, and foreground hills use different entry ranges and small translate/scale differences. It also has a custom “elastic” end-of-page treatment: wheel/touch energy is damped in a `requestAnimationFrame` loop and applied as tiny vertical translations to layers with different depth factors. It activates only at the bottom of the document and caps displacement.

### Header behavior

The fixed header:

- Detects whether it overlaps a marked hero contrast zone.
- Changes visual treatment over the hero.
- Hides after roughly 12px of accumulated downward scroll and reappears on upward scroll.
- Batches geometry checks through `requestAnimationFrame`.

## Progressive enhancement and resilience

This site is unusually careful about failure modes:

- It feature-detects `ViewTimeline`, `view-timeline-name`, `animation-timeline`, and `animation-range`.
- Without support, it removes the long pinned sequence and shows a static hero.
- Under `prefers-reduced-motion: reduce`, the page removes scroll motion, pointer motion, and unnecessary cloud animation.
- Short viewports (`max-height: 40rem`) also get a static layout instead of a cramped sticky experience.
- It waits for hero images to decode/paint before removing a cover.
- Broken images are retried, then the section enters a degraded but readable state.
- The loading state uses inline SVG stroke animation rather than a blank area.
- Main copy and controls remain semantic HTML rather than text baked into the art.
- Most continuous movement is limited to `transform` and `opacity`.

## Why it feels like 3D

The illusion is the sum of several smaller cues:

1. **Occlusion:** foreground architecture overlaps the distant landscape.
2. **Differential scale:** near layers expand much faster than distant layers.
3. **Vanishing-point alignment:** transform origins converge on the doorway opening.
4. **Motion parallax:** pointer and scroll move depth planes by different amounts/directions.
5. **Shadows:** cast/contact shadows ground transparent figures.
6. **Atmospheric layers:** drifting clouds establish distance.
7. **Pinned camera:** the sticky viewport reads like a camera frame.
8. **Timed exits:** figures and interface disappear before the “camera” passes through them.
9. **Text depth:** perspective letter rotations add a subtle spatial cue outside the hero.

## When to use 2.5D versus real WebGL

Use this 2.5D method when:

- The experience is art-directed around one or a few fixed compositions.
- The camera mainly pushes, pans, fades, or reveals.
- SEO, accessible HTML, battery life, and rapid loading matter.
- You can export foreground/midground/background layers.
- Mobile should receive nearly the same composition.

Use Three.js/React Three Fiber when:

- Users must orbit or move freely.
- Lighting, reflections, particles, physics, or 3D model deformation are central.
- Objects must respond spatially from many angles.
- The scene cannot be convincingly represented as layered planes.

Do not add WebGL merely to imitate this homepage. The native CSS version is the closer technical match.

## Case study 2: Meer Mohsin — scroll-directed real-time 3D portfolio

Research target: [meermohsin.me](https://www.meermohsin.me/)

### Executive finding

This site does use real WebGL. It combines a conventional document full of HTML sections with several canvas experiences. GSAP ScrollTrigger pins long sections and converts scroll progress into camera/model transforms; Lenis smooths the input; Three.js renders imported glTF scenes; and a separate raw-WebGL fluid simulation powers a full-screen effect. This is the “creative developer portfolio” architecture that Aristotle deliberately avoids.

### Verified stack

| Layer | Finding | Evidence / confidence |
|---|---|---|
| Site architecture | Static hand-authored HTML plus one bundled JavaScript entry; no React/Vue/Svelte runtime detected | Public HTML and bundle; verified |
| Bundler | Vite-style hashed ESM asset output and module-preload runtime | Public asset layout and bootstrap; verified |
| Hosting | Vercel | Response headers; verified |
| 3D engine | Three.js r180 | Renderer’s shipped revision constant; verified |
| 3D loading | `GLTFLoader`, `DRACOLoader`, glTF/GLB models | Loader code and `model1-*.glb` / `floor-*.glb`; verified |
| Scroll choreography | GSAP 3.13.0 + ScrollTrigger 3.13.0 | Library banners and registered plugin; verified |
| Smooth scrolling | Lenis 1.3.25 | Runtime version and initialization; verified |
| Text splitting | SplitType | Bundled library/data markers and character/word usage; verified |
| Layout transition | GSAP Flip | `getState()` / `from()` usage; verified |
| Shader effect | Custom raw-WebGL fluid simulation | `#fluid2` canvas and simulation/shader configuration; verified |
| Media | AVIF/WebP/PNG, MP4/WebM, MP3, custom fonts | Public assets; verified |
| Form backend | Web3Forms | Form action in public HTML; verified |

### How its 3D scrolling works

The page uses at least two dedicated Three.js presentations:

1. **Roman statue scene.** A transparent WebGL renderer loads a compressed glTF statue. ScrollTrigger rotates the model multiple full turns, moves it forward/backward on the z-axis, raises it on y, and coordinates surrounding DOM transitions. Pointer movement adds smaller camera/model offsets. A very long ScrollTrigger range turns the page into a scrubbed camera track.
2. **Recognition/awards scene.** A second renderer loads a floor/environment model. A pinned timeline lasting roughly `4.5 × innerHeight` moves and rotates its camera through several framed viewpoints.

Representative architecture:

```js
const timeline = gsap.timeline({
  scrollTrigger: {
    trigger: section,
    start: "top top",
    end: "+=2500",
    scrub: 0.8,
    pin: true,
    invalidateOnRefresh: true,
  },
});

timeline
  .to(model.position, { z: -35, y: 12, ease: "none" })
  .to(model.position, { y: 20, ease: "none" });
```

The exact production code is more extensive, but this captures the pattern: ScrollTrigger owns progress; GSAP mutates Three.js object vectors; the render loop draws the latest state.

### Lenis–ScrollTrigger synchronization

The site creates a Lenis instance with a roughly `1.2s` duration, smooth wheel input, and touch synchronization. It then:

- calls `ScrollTrigger.update` when Lenis emits scroll updates;
- advances Lenis from the GSAP ticker;
- disables GSAP lag smoothing;
- manually restores scroll position on reload;
- excludes a case-study sidebar from smooth-scroll interception.

This produces a cinematic delayed “catch-up” feel. It is more complex than native scrolling and needs careful keyboard, nested-scroll, anchor, history, and reduced-motion testing.

### DOM motion language

The portfolio uses many GSAP timelines beyond WebGL:

- Multi-screen intro activated by a click, allowing audio and experience startup.
- Pinned service section extending for about `690%` of viewport travel.
- SVG path drawing through `strokeDashoffset`.
- Character/word splitting, staggered text, blur-to-sharp entrances, and `rotateX` reveals.
- Clip-path image wipes and parallax via `yPercent`.
- FLIP relocation of headings between layout containers.
- Scroll-scrubbed video/image/card sequences.
- Audio cues tied to section entry and WebGL motion.
- Custom cursor and menu/contact overlays.

### Raw-WebGL fluid effect

`#fluid2` is separate from Three.js. The bundle creates a lower-resolution velocity/dye simulation with configurable dissipation, pressure iterations, curl, and splat radius. This is a shader-based fluid-feedback effect: pointer or automated splats inject dye/velocity into textures, and fullscreen passes evolve the field each frame.

This is a useful example of choosing raw WebGL for a specialized GPU simulation while retaining Three.js for scene graphs and glTF assets.

### Strengths and tradeoffs

Strengths:

- True changing viewpoint, model rotation, and lighting justify WebGL.
- ScrollTrigger offers precise long-form choreography and pinning.
- glTF enables detailed authored models rather than layered flat art.
- DOM text stays indexable and can be choreographed beside the canvas.
- Different technologies are used for what each does well: Three.js for objects, raw WebGL for fluids, GSAP for orchestration.

Tradeoffs observed or implied by the shipped architecture:

- The main minified JS entry alone is about 835 KB before transfer compression, excluding models, images, video, and audio.
- Multiple renderers/effects can compete for GPU memory and animation-frame time.
- Long pinned regions and heavy motion increase the importance of mobile and keyboard testing.
- Smooth-scroll mediation adds another state system between physical input and scroll-linked timelines.
- A click-to-activate gate helps with audio autoplay but delays access to the content experience.
- The bundle is tightly coupled to global selectors, so teardown, route reuse, and incremental maintenance are harder than component-scoped architecture.

## Case study 3: 3DCC Core — procedural spatial interface

Research targets: [live 3DCC Core](https://cocktailtheory.github.io/3DCC-core/) and [public repository](https://github.com/cocktailtheory/3DCC-core)

### Executive finding

3DCC is not scroll storytelling. It is a full-viewport, direct-manipulation spatial data interface. The entire app is one HTML file hosted on GitHub Pages, with Three.js r128 loaded from cdnjs and no framework, build system, model files, or image textures. Geometry and textures are generated at runtime.

The conceptual model is three axes × three values = 27 cocktail zones, plus the observer at the origin. Dragging rotates the field, raycasting selects spheres, and selecting the origin moves the camera from an external overview to a viewpoint inside the coordinate system.

### Verified stack and scene graph

| Layer | Finding | Evidence / confidence |
|---|---|---|
| Hosting | GitHub Pages | Response headers and repository; verified |
| Architecture | One HTML file with inline CSS and JavaScript | Public source; verified |
| 3D engine | Three.js r128 from cdnjs | Script URL; verified |
| Models/textures | No external 3D models or scene textures | Public page assets; verified |
| Geometry | Spheres, toruses, cylinders, lines/arrows, groups | Public source; verified |
| Materials | Physical glass-like spheres, basic additive halos/vectors | Public source; verified |
| Labels | Runtime 2D canvas → `CanvasTexture` → Three.js sprites | Public source; verified |
| Interaction | Pointer drag, custom orbit math, Raycaster selection | Public source; verified |
| Audio | Web Audio API synthesis | Public source; verified |
| Animation | One `requestAnimationFrame` loop plus explicit state machines | Public source; verified |

### Rendering pipeline

- `WebGLRenderer` uses antialiasing, alpha, a transparent clear color, and device-pixel ratio capped at `2`.
- A perspective camera uses a 42-degree external field of view and a wider internal field of view.
- Exponential fog hides distant clutter and creates depth.
- Ambient and point lights illuminate clear-coated physical materials.
- A small equirectangular environment map is painted into a 2D canvas, converted to `CanvasTexture`, and prefiltered with `PMREMGenerator` for glossy reflections.
- Each data sphere uses `MeshPhysicalMaterial` with low roughness, clearcoat, and emissive color.
- `onBeforeCompile` injects a Fresnel/iridescent contribution into the material fragment shader.
- Halos, orbital rings, and selection paths use transparency and additive blending.

### Data-to-space mapping

The app defines three categorical axes:

- x: mouthfeel/body, from crisp/refresh to creamy;
- y: structure/strength, from light/soft to hard;
- z: sweetness, from dry to sweet.

Nested iteration creates all 27 coordinate combinations. Each coordinate receives a representative cocktail and a color derived from its x/y/z values. This is the core lesson: real 3D is functional here because position encodes three independent variables, not just decoration.

### Interaction model

1. Pointer down ends auto-rotation and begins dragging.
2. Pointer movement updates rotation angles; in internal mode it updates an orbital camera position instead.
3. A small movement threshold separates a click/tap from a drag.
4. A Three.js Raycaster tests sphere and origin intersections.
5. Hover raycasts change the cursor on mouse devices.
6. Selecting a zone raises its emissive intensity and runs a neon path from the observer to the target.
7. Selecting the origin starts a timed camera transition, uses a white flash to hide a viewpoint swap, then places the camera on a small orbit around the center.
8. “Take off” reverses the state machine and returns to external mode.

The neon path is built procedurally from cylinder segments along an x→y→z route. It grows, holds, and fades by changing segment length, position, and opacity over time. This makes the relationship between origin and selection visible rather than merely highlighting the destination.

### Audio design

The sound button initializes an `AudioContext` only after user interaction. Short oscillator/gain envelopes produce click, travel, close, enter, and exit cues. Frequencies and stereo pan can be derived from the selected coordinate. This avoids audio-file downloads and makes sound semantically responsive to data.

### Accessibility and fallback notes

Verified positives:

- The canvas has `role="img"` and a descriptive interaction label.
- Exit and sound controls are native buttons with accessible labels.
- A WebGL capability check presents a readable failure screen.
- Reduced motion disables the boot sequence and auto-rotation and shortens transitions.
- Pixel ratio is capped.
- Temporary neon geometry/materials are explicitly disposed.

Limitations to improve in a new implementation:

- The WebGL fallback explains failure but does not provide the 27 items as an HTML table/list.
- Canvas raycast targets are not individually keyboard focusable.
- The canvas accessible name cannot communicate current selection or the full spatial dataset.
- The render loop continues continuously even when the tab/scene could be idle.
- Three.js r128 is an intentionally simple CDN setup but substantially older than the engine in the Meer Mohsin example.
- The repository states all rights reserved and notes pending patent/design applications; learn from the technical pattern, but do not clone the coordinate concept, data mapping, visual identity, copy, or interaction sequence.

## Three-site comparison

| Dimension | Aristotle | Meer Mohsin | 3DCC Core |
|---|---|---|---|
| Visual category | Illustrated cinematic 2.5D | Hybrid editorial site + real-time 3D | Full-screen spatial data interface |
| Primary renderer | DOM/CSS + raster layers | DOM + multiple WebGL canvases | One WebGL canvas + small DOM HUD |
| Main motion driver | Native CSS view timeline | Lenis → GSAP ticker → ScrollTrigger | Pointer input + time/state machine |
| 3D engine | None detected | Three.js r180 + raw WebGL | Three.js r128 |
| Asset strategy | Painted AVIF/WebP/PNG layers | glTF/Draco + video/images/audio | Procedural geometry/textures/audio |
| Scroll model | Native reversible scroll scrub | Smoothed, pinned GSAP scrub | Page scrolling disabled |
| Best suited to | Marketing narrative | Creative portfolio/showreel | Exploratory multivariate interface |
| Main risk | Browser support for native timelines | Payload/GPU/motion complexity | Accessibility and discoverability |

### Selection rule

- Choose **Aristotle’s class of architecture** when art direction is fixed and the story can be expressed by layered depth.
- Choose **Meer Mohsin’s class** when authored 3D assets must rotate or the camera must travel through a model while the surrounding document tells a story.
- Choose **3DCC’s class** when spatial position itself is the interface and users must directly explore/select the scene.

---

# External discovery pass: comparable sites and implementation studies

This section is the discovery work that should have been included from the start. It is not a list of visual clones. Each reference contributes a distinct implementation pattern that can be combined with the three primary case studies.

### Evidence labels

- **Live/official** — the deployed experience or creator's own site was inspected.
- **Author case study** — an implementation article written by, or with direct input from, the creators.
- **Tutorial/demo** — a public teaching example with implementation guidance or code.
- **Secondary showcase** — useful visual reference, but technical claims should be independently verified before becoming requirements.

## Reference index

| Reference | Experience class | Reported or demonstrated stack | Reusable lesson | Evidence |
|---|---|---|---|---|
| [Bruno Simon portfolio](https://bruno-simon.com/) | Driveable 3D portfolio/world | Three.js; current site also exposes WebGPU and quality choices | Make navigation itself playful, but retain obvious controls, quality selection, and an HTML fallback | Live/official |
| [Joseph Santamaria portfolio case study](https://tympanus.net/codrops/2026/04/28/more-than-a-portfolio-building-a-scroll-driven-3d-world-with-something-to-say/) | Scroll-directed 3D narrative world | Three.js, GSAP, WebGL, Blender, Draco, KTX2/Basis, instancing | Treat compression, draw calls, mobile shaders, and narrative pacing as one system | Author case study |
| [ZERO case study](https://tympanus.net/codrops/2026/07/17/zero-the-engineering-behind-a-defiant-interactive-narrative/) | Gesture-led interactive narrative | Three.js, GSAP, Howler, Vite, custom shader work | Build around a repeatable gesture and hard performance budget instead of adding interaction after the visuals | Author case study |
| [The Spark case study](https://tympanus.net/codrops/2026/01/09/the-spark-engineering-an-immersive-story-first-web-experience/) | Story-first WebGL/Webflow hybrid | cables.gl, Webflow, shared scroll controller | A visual CMS layer and a GPU scene can coexist if one normalized scroll source coordinates both | Author case study |
| [Lusion WebGL Scroll Sync](https://webgl-scroll-sync.lusion.co/) | DOM/WebGL synchronization demo | WebGL scene synchronized with document scroll | Align DOM sections and 3D objects through one coordinate contract; do not tune two unrelated animations by eye | Tutorial/demo |
| [Reactive Depth](https://tympanus.net/codrops/2026/02/17/reactive-depth-building-a-scroll-driven-3d-image-tube-with-react-three-fiber/) | Scroll-reactive image tunnel | React Three Fiber, shaders, scroll inertia | Feed velocity as well as position into deformation so the scene feels physical rather than merely scrubbed | Author tutorial |
| [Camera fly-through with Theatre.js](https://tympanus.net/codrops/2023/02/14/animate-a-camera-fly-through-on-scroll-using-theatre-js-and-react-three-fiber/) | Authored camera path | React Three Fiber, Theatre.js, scroll mapping | Use a visual timeline for camera choreography, then map scroll into that authored timeline | Tutorial/demo |
| [Progressively enhanced WebGL lens](https://tympanus.net/codrops/2023/10/10/progressively-enhanced-webgl-lens-refraction/) | WebGL enhancement over semantic DOM | React Three Fiber, `r3f-scroll-rig`, Lenis, shaders | Keep real DOM content and let WebGL track it, producing a useful page before or without the canvas | Tutorial/demo |
| [Scroll, refraction, and shader effects](https://tympanus.net/codrops/2019/12/16/scroll-refraction-and-shader-effects-in-three-js-and-react/) | Scroll-linked gallery distortion | React, Three.js/R3F, instancing, shaders | Preserve a real scroll area and move high-frequency animation through refs rather than React state | Author tutorial |
| [Chang Liu portfolio V4](https://tympanus.net/codrops/2019/10/16/case-study-chang-liu-portfolio-v4/) | Distorted portfolio/gallery | Three.js and shader-based image deformation | A restrained 2D portfolio can gain depth from one coherent distortion language without becoming a 3D world | Author case study |
| [Apple-style image sequence lesson](https://francescocastronuovo.com/learn/lessons/apple-style-image-sequence-webflow/) | Rendered-product scroll sequence | Canvas, frame sequence, GSAP ScrollTrigger | Pre-rendered frames are often the right answer when photorealism matters more than free camera movement | Tutorial/demo |
| [Infinite Canvas](https://tympanus.net/codrops/2026/01/07/infinite-canvas-building-a-seamless-pan-anywhere-image-space/) | Free-pan spatial gallery | React Three Fiber and a recycled spatial layout | Virtualize or recycle items so an apparently unbounded world has bounded memory and draw cost | Author tutorial |

## What each reference adds to the architecture menu

### 1. A navigable world: Bruno Simon

Bruno Simon's portfolio replaces page navigation with a small driveable vehicle. The important pattern is not “put a car on the site”; it is **spatial affordance**. The visitor learns the world by moving through it, while keyboard, pointer, touch, and gamepad support broaden the input model. The current official page exposes quality controls and a non-WebGL HTML route, useful reminders that a playful primary interface still needs an escape hatch.

Use this class when exploration is part of the brand promise. Do not use it when visitors primarily need to compare services, scan prices, or complete a task quickly.

### 2. A scroll-authored world: Joseph Santamaria

This is the closest external reference to the requested “3D animations with scrolling” category. Its case study describes a production pipeline spanning Blender-authored scenes, Three.js, GSAP, compressed textures and geometry, instancing, mobile-specific shader decisions, and narrative transitions. The lesson is that scroll choreography is only the visible layer; asset budgets and render architecture determine whether it survives on normal hardware.

A useful implementation contract is:

```text
native scroll position
  -> normalized chapter progress
  -> camera/scene timeline
  -> current chapter state for semantic DOM
  -> velocity envelope for secondary motion
```

### 3. A single gesture as the narrative engine: ZERO

ZERO reportedly turns drawing a zero into the recurring interaction and visual motif. That is a stronger design system than a collection of unrelated hover effects. Its case study also documents aggressive reduction from large source assets to a compact delivery budget and targets ordinary Android hardware. The reusable move is to define one input primitive, one visual response family, and one measurable performance envelope early.

### 4. A hybrid authoring stack: The Spark

The Spark joins a Webflow-authored content layer to cables.gl scenes through a shared scroll controller. This is useful for teams that need visual editing or CMS ownership without asking the content system to render 3D. The integration boundary should exchange small stable values—chapter ID, normalized progress, velocity, direction—not direct knowledge of every object in both systems.

### 5. DOM/WebGL registration: Lusion and progressive enhancement

The Lusion scroll-sync demo and the progressive WebGL lens tutorial demonstrate a recurring architecture: semantic DOM owns layout and accessibility, while the canvas mirrors selected elements in screen space. On every relevant layout change, measure the DOM rectangles and update the 3D projection. During scroll, update a shared offset instead of repeatedly forcing layout reads.

This is often preferable to rebuilding an editorial page entirely inside WebGL. It keeps selection, links, headings, search indexing, responsive flow, and keyboard behavior in the browser's strongest layer.

### 6. Authored camera choreography: Theatre.js

For a camera that must hit exact compositions, a visual animation tool can create the path more reliably than dozens of hard-coded scroll ranges. Store the camera track as authored data, map normalized scroll progress into its playhead, and keep content chapter boundaries in a separate manifest. Designers can refine framing without restructuring application logic.

### 7. Pre-rendered pseudo-3D: image sequences

An image-sequence canvas can deliver ray-traced or simulated motion without shipping a real-time scene. The trade is a larger set of raster downloads and a fixed camera. It works well for a single product transformation, especially when the object does not need pointer interaction. Decode only a sliding window around the current frame, provide a poster image, and expose the same story in real DOM text.

## Cross-reference decision table

| Desired outcome | Start with | Avoid by default |
|---|---|---|
| Painterly cinematic reveal | Aristotle-style layered DOM/CSS | A real-time engine used only to move flat art |
| Product turntable with fixed camera | Image sequence or modest Three.js scene | A large open-world scene |
| Camera travel through authored models | Three.js/R3F + GSAP or Theatre.js | Independent per-section tweens with no shared timeline |
| Editorial layout with localized distortion | DOM + tracked WebGL overlays | Rendering all typography into the canvas |
| Playable brand world | Bruno-style navigable scene | Hiding essential business information inside exploration |
| Spatial data explorer | 3DCC-style raycast interface | Treating decorative particles as meaningful data |
| CMS-managed immersive story | Webflow/content layer + isolated GPU runtime | Letting the CMS and renderer maintain separate scroll truth |

---

# Public prompt and workflow research

## Provenance note

The entries in this section were found in public prompt articles or tutorials. They are **not the 25 prompts later in this document**. To respect the authors' work, the log records each prompt's intent, constraints, and useful pattern rather than reproducing whole prompts verbatim. “Tested,” “production-ready,” and similar descriptions below are source claims unless independently verified here.

## Source 1: AIReiter — six published Three.js prompt patterns

Source: [6 Best Prompts to Build Stunning 3D Websites with AI](https://aireiter.com/blog/3d-website-prompts)

The article publishes six complete prompts. Recorded in condensed form:

1. **Particle-galaxy hero** — ask for a single HTML file using Three.js from a CDN, a large amber/violet particle field, subtle pointer response, restrained orbit controls, a DOM headline/CTA above the canvas, correct pointer-event layering, and resize handling.
2. **Scroll-driven product showcase** — use a torus-knot stand-in, fixed full-viewport canvas, four content sections, a complete camera orbit over the page, per-section material changes, left-aligned semantic copy, lerped values, studio lighting, and a ground shadow.
3. **GLSL fluid-gradient landing page** — render a full-screen quad with time-driven noise and grain behind a glass-like DOM card, with no heavy postprocessing stack.
4. **3D portfolio fly-through** — place six project cards along a curve, move the camera with scroll, frame the active card, use fog for depth, damp scroll input, and keep the cards actionable. The article says its first attempt at this prompt did not fully succeed, a useful warning about asking a model to solve layout, camera direction, and interaction in one pass.
5. **Interactive product viewer** — request a simple geometric product, swatches, a material toggle, constrained orbit controls, studio lighting, contact shadow, and HTML controls rather than 3D text controls.
6. **One-page 3D business site** — retain normal hero/services/work/contact structure while one central 3D form changes scale and color between sections; keep the rest as DOM and define mobile behavior.

What is worth borrowing: real API nouns, explicit layering rules, a single-file boundary for prototypes, and concrete mobile/resize requirements. What still needs human work: original models, tuned shaders, art direction, asset rights, profiling, semantic structure, and device testing.

## Source 2: AgentOS Guide — ambitious “build proof” prompts

Source: [Built With Opus 5.5: Interactive 3D Portfolio](https://agentos.guide/opus-5-5-portfolio)

The page records several large prompts and the artifacts produced from them:

1. **Procedural sky-whale world** — generate an explorable fantasy scene with code-created assets, flight controls, time-of-day changes, and explicit phone-size screenshots as acceptance evidence.
2. **Japanese river valley** — build a procedural environment around a river corridor, bridges, vegetation, rocks, and a small settlement; treat composition and navigable landmarks as requirements.
3. **Clearwater scene** — concentrate the brief on water rendering, underwater visibility, shoreline composition, and environmental mood rather than a generic “realistic water” request.
4. **LUMEN scroll experience** — create a self-contained Three.js page in which a glass sculpture morphs through scroll chapters, paired with a material-study interface, dark art direction, negative style constraints, and a concrete “done means” checklist.

The best pattern here is the acceptance block. A generation prompt becomes more useful when it requires screenshots at named viewports, interaction verification, console-error checks, and a list of forbidden shortcuts. The visual ambitions are high, so these examples should be treated as prototypes requiring performance and accessibility hardening—not proof that one prompt reliably creates a production site.

## Source 3: AETUMI — prompt an existing production scaffold

Source: [How to Build a 3D Website with AI](https://aetumi.app/news/how-to-build-a-3d-website/)

This commercial guide recommends starting from a working template or scene and asking AI to modify it rather than regenerate the entire rendering architecture. Its reusable prompt requirements include:

- one canvas rather than a canvas per decorative object;
- a capped device-pixel ratio;
- a simple studio-light rig;
- subtle pointer tilt with damping;
- pausing or reducing work when the canvas is offscreen;
- `prefers-reduced-motion` behavior and a poster fallback;
- real DOM headings, copy, and calls to action.

That is a good production-oriented checklist. The source markets its own templates, so treat product comparisons and performance claims as vendor claims.

## Source 4: MotionSites — image-to-3D plus reference-video workflow

Source: [Build a 3D Scroll Animated Website with AI](https://motionsites.ai/lesson/build-3d-scroll-animated-website-with-ai)

The public lesson breaks the work into a prompt chain instead of one giant request:

1. Ask an image model for consistent front, rear, left, and right views of the subject.
2. Feed those views into an image-to-3D service and export a GLB, with a practical texture-resolution target.
3. Ask a coding model for a fixed/pinned page whose scroll progress transitions between scenes and scatters or reassembles lettering.
4. Supply the GLB and a motion-reference video, then ask for a Three.js implementation that matches timing and composition.
5. Request appropriate tone mapping and lighting adjustments as a separate finishing pass.

The valuable idea is decomposition: asset generation, geometry conversion, motion blocking, implementation, and color finishing are different jobs. The source is commercial and its rapid “award-winning” framing is promotional; generated geometry still needs topology, compression, material, licensing, and mobile checks.

## Source 5: VULK — prompt-first versus visual-builder framing

Source: [How to Build a 3D Website with AI: Complete Guide](https://vulk.dev/blog/how-to-build-a-3d-website-with-ai-complete-guide)

This guide presents a collection of prompt-led 3D website patterns and contrasts generated code with visual scene builders. Its useful research value is the decision itself: prompt-first workflows are strong for fast scaffolding and code ownership, while visual tools are stronger for direct spatial art direction. It is also a vendor-authored page; use it to identify categories and workflow questions, not as independent evidence that its own product is the best implementation route.

## Repeating anatomy of the stronger public prompts

Across these sources, the prompts that are most actionable specify:

```text
1. Runtime boundary
   framework, renderer, single-file prototype versus production app

2. Scene inventory
   exact objects, lights, fog, camera, materials, and asset formats

3. Input mapping
   scroll/pointer/touch input -> normalized value -> target property

4. DOM/canvas ownership
   which layer owns text, links, layout, controls, and hit testing

5. Motion character
   interpolation method, damping, duration, easing, and reverse behavior

6. Responsive policy
   camera framing, breakpoints, DPR, texture/model variants, touch behavior

7. Accessibility contract
   reduced-motion state, keyboard path, semantic duplicate/fallback, contrast

8. Performance budget
   asset bytes, draw calls, triangles, texture sizes, long-task and FPS targets

9. Failure behavior
   loading state, WebGL failure, missing model, slow network, offscreen pause

10. Acceptance evidence
    named viewport screenshots, console check, interaction test, performance trace
```

## What the public prompts commonly omit

- Asset rights and provenance.
- A hard compressed-transfer budget.
- GPU-memory implications of decoded textures.
- Context-loss handling and cleanup on route changes.
- Deep-linking to chapters and restoring the correct scroll/camera state.
- Browser zoom, short landscape phones, low-power mode, and background-tab behavior.
- Keyboard navigation when canvas objects are interactive.
- Analytics events that do not fire on every animation frame.
- A visual-regression plan for several scroll positions.
- A statement of what must remain useful if JavaScript or WebGL fails.

Those omissions are addressed in the original prompts below.

---

# Original copy-ready website-building prompts

The following 25 prompts are **original synthesis written for this research log**. They were not copied from a “25 prompts” article, GitHub repository, or any of the public sources above. The number 25 was simply the way I divided the three-site findings into implementation tasks, audits, and architecture choices.

## Prompt 1 — Master implementation prompt (closest technical match)

```text
Act as a senior creative front-end engineer. Build an original, production-ready, illustration-led landing page with cinematic scroll storytelling. Use Next.js App Router, React, TypeScript, CSS Modules, and native CSS Scroll-driven Animations. Do not use Three.js, React Three Fiber, GSAP, Lenis, Locomotive Scroll, Spline, or scroll hijacking unless a requirement truly cannot be met natively.

Creative direction:
- [BRAND / PRODUCT]
- [AUDIENCE]
- [ORIGINAL VISUAL CONCEPT]
- [PALETTE]
- [SERIF / SANS FONT DIRECTION]
- Mood: tactile, editorial, optimistic, painterly, dimensional.
- Do not copy Aristotle’s doorway, characters, landscape, wording, logo, palette, or exact composition.

Hero architecture:
- Create a 240–280svh outer sequence with `view-timeline-name: --hero-sequence` and `view-timeline-axis: block`.
- Place a 100svh sticky stage at top: 0 with overflow hidden and isolation isolate.
- Compose at least four depth planes from optimized AVIF/WebP assets: far atmosphere, landscape, subject, foreground frame.
- Keep all marketing copy and controls as semantic HTML.
- Align each transform origin to a designed focal point/vanishing point via CSS custom properties.
- Scrub transforms with `animation-timeline: --hero-sequence` and explicit `animation-range` values.
- Animate only transform, opacity, visibility, filter where restrained, and CSS custom properties that feed compositor-friendly transforms.
- Make foreground scale much faster than the background to simulate a camera push through an opening.
- Fade the subject and initial UI before the foreground fills the viewport.
- Transition to two sequential editorial text panels inside the same pinned scene.
- Ensure scrolling backward reverses every state cleanly.

Supporting interactions:
- Add pointer parallax to selected layered illustrations using normalized pointer coordinates, requestAnimationFrame batching, and CSS variables for back/mid/front planes.
- Run pointer parallax only for mouse pointers while the section is visible.
- Add intersection-triggered reveal groups with staggered children; unobserve after reveal.
- Add one scroll-linked character reveal using perspective and rotateX without harming accessible text.
- Add slow atmospheric drift with randomized negative animation delays.
- Add a layered footer whose depth planes use different view-timeline ranges.

Resilience and accessibility:
- Implement `@supports` feature detection for all scroll-timeline features.
- Provide a complete static fallback that removes the tall sequence and sticky pinning.
- Under `prefers-reduced-motion: reduce`, show final readable states, remove continuous drift/parallax, and preserve all content.
- Disable the pinned sequence on viewports shorter than 40rem.
- Never hide meaningful content until JavaScript has initialized the reveal behavior.
- Maintain keyboard order, visible focus, correct landmarks/headings, descriptive alt text, and adequate contrast.
- Do not transform the native scroll position.

Performance:
- Preload only the critical hero font files and the single critical hero image.
- Use responsive `<picture>` sources with explicit dimensions and AVIF/WebP fallbacks.
- Split transparent layers tightly and keep alpha images near their rendered size.
- Decode hero images before revealing the composition; provide a lightweight SVG loading sketch and readable degraded state.
- Pause observers and pointer work while offscreen.
- Avoid per-scroll React state updates and layout thrashing.
- Target no unexpected layout shift, responsive 60fps motion, and a useful static first render.

Deliverables:
1. Brief architecture explanation.
2. File tree.
3. Complete components and CSS Modules, not pseudocode.
4. Typed data structures for scenes/key moments.
5. Fallback and reduced-motion implementation.
6. Asset manifest with dimensions, formats, focal points, and alt text.
7. Playwright tests for timeline support, fallback, mobile, reduced motion, keyboard access, and reverse scrolling.
8. Performance checklist and manual QA instructions.

Before coding, state the original visual metaphor, identify the depth planes, and provide a 0–100% choreography table. Then implement it.
```

## Prompt 2 — Art direction and asset-production brief

```text
Act as an art director and technical illustrator preparing assets for a layered 2.5D scroll website. Design an original scene for [BRAND] around the metaphor “[METAPHOR].” It must not resemble Aristotle’s architecture, teacher/student figures, pastoral landscape, branding, or copy.

Create a production asset plan with:
- A far atmosphere/background plate that safely covers 16:9, 4:3, 3:2, ultrawide, and mobile crops.
- A midground environment with a clearly stated focal point at normalized coordinates [X, Y].
- One transparent subject layer with clean alpha edges.
- One foreground framing layer designed to scale past the viewport and reveal the background.
- Three to six transparent atmospheric sprites.
- Optional texture overlays that can repeat without visible seams.
- Separate shadow strategy: source silhouette mask, cast-shadow transform, and contact-shadow ellipse.

For every asset, specify filename, role, source dimensions, intended rendered size, crop-safe region, alpha requirement, export format, quality target, and responsive variants. Prefer AVIF plus WebP; use PNG only when alpha quality requires it. Keep text, logos, CTAs, numbers, and accessibility-critical information out of images.

End with a layer-stack diagram from farthest to nearest and a handoff checklist for the front-end engineer.
```

## Prompt 3 — Scroll choreography designer

```text
Design a scroll choreography for a 260svh section with a pinned 100svh stage. Return a table covering 0–100% progress. Use overlapping phases rather than evenly spaced steps.

Scene layers: [LIST LAYERS].
Narrative beats: [LIST BEATS].

For each layer define:
- active progress range;
- transform origin;
- translate/scale/rotation values;
- opacity and visibility thresholds;
- easing segment;
- z-index/occlusion relationship;
- what happens when scrolling backward;
- reduced-motion final state.

Use these principles:
- Hold the opening composition long enough to read.
- Begin with a subtle 1.00→1.06 push.
- Accelerate the near layer only after the CTA/copy is understood.
- Keep background scale restrained, normally below 1.15.
- Remove near subjects before the camera seems to intersect them.
- Complete the transition before 85% so the ending can settle.
- Avoid animating layout properties.

Return both the choreography table and corresponding named CSS keyframe skeletons.
```

## Prompt 4 — Native CSS timeline implementation

```text
Implement a reusable React + TypeScript `ScrollScene` component using native CSS named view timelines. It must accept:
- `timelineName`;
- sequence height in svh;
- minimum stage height;
- focal point x/y;
- typed layer definitions;
- typed key moments;
- optional static fallback content.

Requirements:
- Use a tall relative wrapper and a sticky 100svh stage.
- Expose focal point and dimensions through CSS custom properties.
- Attach child keyframes to the named timeline.
- Put `animation` before `animation-timeline` in each rule so the shorthand does not reset the custom timeline.
- Feature-detect both CSS properties and a usable `ViewTimeline` currentTime.
- Add a data attribute for fallback mode.
- In fallback/reduced-motion/short-height mode, remove sticky positioning, long sequence height, and hidden text.
- Avoid scroll event listeners for the main choreography.
- Include a minimal example with foreground scale, background scale, subject fade, and two text panels.
- Include comments explaining why each range exists.
```

## Prompt 5 — Nine-slice cinematic portal/frame

```text
Build a responsive nine-slice foreground compositor for one large original illustration. Use a 3×3 CSS Grid where the center column preserves a designed aspect ratio while side columns expand to cover the viewport. Crop one source image into all nine cells using CSS custom properties for source x, y, width, and height.

Requirements:
- No canvas and no WebGL.
- Preserve a configurable opening/focal area across mobile, tablet, desktop, and ultrawide screens.
- Add 1px overlap to prevent fractional-pixel seams.
- When `mask-composite: intersect` is supported, feather horizontal and vertical slice seams with small gradient masks.
- Keep the source image’s proportions exact; do not stretch individual pieces.
- Document how to derive each slice coordinate from a design file.
- Add a debug mode that outlines all nine cells and labels their source coordinates.
- Add screenshot tests at 390×844, 768×1024, 1440×900, and 2560×1440.
```

## Prompt 6 — Pointer depth parallax hook

```text
Create an accessible `usePointerDepthParallax` React hook and companion CSS.

Behavior:
- Observe the owning section with IntersectionObserver.
- Listen for pointermove only on that section.
- Respond only to `pointerType === "mouse"` and only when no buttons are pressed.
- Normalize pointer x/y within the section to the inclusive range -1…1.
- Batch style writes to one requestAnimationFrame.
- Write six variables: --depth-back-x/y, --depth-mid-x/y, --depth-front-x/y.
- Use back (-10px, -8px), mid (+7px, +6px), and front (+18px, +14px) as configurable defaults.
- Reset variables on pointerleave, viewport exit, window blur, and reduced-motion changes.
- Clean up every listener, observer, media-query listener, and pending frame.
- Do not update React state per pointer event.

Add unit tests for normalization/clamping/cleanup and an interaction test proving touch movement does nothing.
```

## Prompt 7 — Scroll-linked character unfurl

```text
Build an accessible character-by-character heading reveal without duplicating announced text.

Requirements:
- Preserve words as wrapping units but animate individual visual characters.
- Give assistive technology one unsplit string; mark decorative split characters aria-hidden.
- Parent perspective: approximately 24rem.
- Starting state per character: opacity 0, small randomized x drift, upward lift, rotateX around 65–75deg, small rotateZ tilt, scale around .98.
- Ending state: opacity 1 and identity transform.
- Drive each character from a native view timeline with slightly staggered animation-range values.
- Use deterministic pseudo-random values from the character index so SSR and hydration agree.
- Provide IntersectionObserver/time-based fallback.
- Under reduced motion, render the final text immediately.
- Preserve punctuation, nonbreaking spaces where necessary, and responsive line wrapping.
```

## Prompt 8 — Reveal system

```text
Create a small reveal primitive for React that supports a single element or a staggered group.

Use IntersectionObserver with configurable defaults: threshold .15 and rootMargin "0px 0px -15% 0px". Set a `data-reveal-ready` attribute only after the observer exists, then set `data-reveal-visible="true"` on entry and unobserve completed targets. For groups, set a CSS delay variable in 100ms increments.

Prevent these common bugs:
- SSR content remaining invisible if JavaScript fails.
- Focus moving into visually hidden content.
- Strict Mode leaking duplicate observers.
- reduced-motion users waiting through a stagger.
- repeatedly triggering finished reveals.

Return the hook, component, CSS Module, and tests.
```

## Prompt 9 — Atmospheric cloud system

```text
Implement a layered atmospheric sprite system using ordinary `<img>` elements and CSS, not canvas.

Each cloud has typed configuration for depth, top position, width clamp, opacity, horizontal drift duration, negative delay, and scroll-rise distance. Nest two wrappers so time-based horizontal drift and scroll-driven vertical rise use separate transform properties and cannot overwrite one another.

Requirements:
- Long 70–220 second linear drift loops.
- Negative delays so clouds begin at varied positions.
- Rear clouds smaller, slower, and more transparent.
- Attach vertical rise to the hero’s named timeline.
- Hide or freeze decorative motion for reduced motion.
- Mark all sprites decorative with empty alt text and aria-hidden.
- Avoid large transparent canvases around the source images.
```

## Prompt 10 — Layered footer parallax

```text
Build an original illustrated footer with four depth planes: distant terrain, midground terrain, giant brand wordmark, and foreground terrain. Use a named CSS view timeline on the footer landscape. Give each plane a different entry animation range so the foreground travels more than the background.

Use subtle values only: translation within roughly 0–18% of layer height and scale within 1.00–1.08. Keep footer navigation as semantic HTML above the landscape. Add slow decorative cloud drift. Provide a non-animated fallback and reduced-motion state. Ensure no animated layer blocks links or pointer input.

Optional: add a restrained bottom-of-page elastic response driven by wheel/touch energy. Only activate at the true document bottom; cap displacement around 44px; exponentially damp velocity; update transforms in requestAnimationFrame; never prevent scrolling unless the user is already at the boundary; and omit the effect under reduced motion.
```

## Prompt 11 — Loading, decode, and degraded-state controller

```text
Create a robust hero-artwork reveal controller for a layered responsive image scene.

Requirements:
- Collect all `[data-hero-artwork]` images.
- Wait for `load`/`decode()` where available.
- Require two animation frames after successful decode before removing the cover.
- Display an inline SVG sketch loader whose paths animate with stroke-dasharray/stroke-dashoffset.
- If an image fails, retry once with a cache-busting query and a safe fallback source strategy.
- If the scene still fails, set `data-hero-degraded`, stop the complex reveal, switch to a readable static color treatment, and keep the CTA usable.
- Report one deduplicated handled error to the app’s monitoring adapter without including personal data.
- Continue checking at a low frequency for late recovery, then clear the degraded state if every image becomes valid.
- Clean up timers and animations on unmount.
- With JavaScript disabled, hide the cover/sketch and show content immediately.
```

## Prompt 12 — Motion accessibility audit

```text
Audit this scroll-story implementation for WCAG-friendly motion, keyboard access, semantics, and failure safety. Return findings by severity and then patch the code.

Check specifically:
- prefers-reduced-motion removes scroll-scrubbed zoom, pointer parallax, drifting backgrounds, character rotation, and elastic overscroll;
- content is never accessible only at one scroll percentage;
- sticky scenes do not trap keyboard focus or hide focused controls;
- reverse scrolling does not create flashing or abrupt visibility toggles;
- all split text has a single accessible name;
- decorative layers are ignored by assistive technology;
- CTA focus styles remain visible over every background phase;
- color contrast remains valid before, during, and after the scene;
- short viewports and 200% zoom receive a normal-flow layout;
- no essential information exists only in an image or animation.

Provide an automated Playwright reduced-motion test and a manual screen-reader/keyboard checklist.
```

## Prompt 13 — Performance audit and optimization

```text
Profile this layered scroll page on mid-tier mobile hardware and a throttled network. Do not remove the core visual concept. Diagnose and fix:
- oversized responsive images;
- excessive alpha surface area;
- decode stalls;
- layout shifts;
- main-thread scroll handlers;
- forced synchronous layouts;
- overlapping requestAnimationFrame loops;
- too many `will-change` layers;
- filters that trigger large paint regions;
- animations running while offscreen;
- hydration work in static sections;
- duplicate analytics/observer initialization.

Prefer native CSS scroll timelines for the primary sequence. Keep transforms and opacity compositor-friendly. Produce before/after measurements for LCP, CLS, INP, transfer size, decoded image memory, long tasks, and animation frame consistency. State the device, browser, viewport, and throttling profile for every result.
```

## Prompt 14 — Cross-browser fallback and test matrix

```text
Create a compatibility layer and test suite for a native CSS scroll-driven landing page.

At runtime, verify:
1. `window.ViewTimeline` exists;
2. CSS.supports accepts `view-timeline-name`;
3. CSS.supports accepts `animation-timeline`;
4. CSS.supports accepts the required `animation-range` syntax;
5. a ViewTimeline constructed for the section yields a usable currentTime.

If any check fails, set a fallback data attribute before the user can encounter the pinned sequence. In fallback mode: make the sequence auto-height, make the stage relative, show the initial content, hide secondary scroll-only panels or place them in normal flow, and remove all timeline animations.

Write Playwright coverage for Chromium, WebKit, and Firefox; desktop and mobile sizes; reduced motion; short viewport; JavaScript disabled; images blocked; slow images; back/forward navigation; refresh at a mid-page scroll position; and scroll reversal. Use screenshots at key progress points with reasonable visual-diff tolerances.
```

## Prompt 15 — Visual-regression harness for scroll keyframes

```text
Build a Playwright visual-regression helper for a sticky scroll scene. Locate the sequence, calculate its scrollable travel as `sectionHeight - viewportHeight`, and capture deterministic screenshots at 0%, 10%, 20%, 30%, 40%, 50%, 65%, 75%, 85%, and 100% progress.

Before each capture:
- wait for fonts and hero images;
- disable unrelated time-based animations without disabling the scroll timeline;
- set an exact scroll position;
- wait two requestAnimationFrame ticks;
- verify the expected layer visibility and transform ranges in the DOM.

Add assertions for seam gaps, blank frames, accidental horizontal overflow, unreadable copy, early visibility:hidden, transform-origin drift, and differences between forward and reverse arrival at the same progress point.
```

## Prompt 16 — True WebGL alternative (only if real 3D is required)

```text
Reinterpret this layered scroll-story concept as an original React Three Fiber experience because the product genuinely requires live 3D [STATE WHY]. Use Next.js, React, TypeScript, `@react-three/fiber`, Drei, and glTF assets. Preserve native document scrolling and semantic HTML overlays; do not replace the page with a canvas-only interface.

Requirements:
- One fixed/sticky canvas confined to the story section.
- A normalized 0–1 progress source derived from the section’s real scroll travel.
- Camera position/target and object transforms sampled from a typed keyframe track.
- Damped interpolation that remains deterministic when scrolling backward.
- Compressed glTF with Draco or Meshopt, KTX2 textures, sensible DPR cap, and lazy loading.
- Explicit light count, draw-call, triangle, and texture-memory budgets.
- Pause rendering when offscreen; use frameloop demand when possible.
- HTML headline/CTA remain outside the canvas and synchronized by progress.
- Static poster fallback for WebGL failure, data saver, low-power mode, reduced motion, and crawlers.
- No scroll hijacking.
- Include disposal, context-loss recovery, loading progress, and mobile quality tiers.

Explain why WebGL adds user value that layered DOM/CSS could not provide. If it does not, recommend the 2.5D implementation instead.
```

## Prompt 17 — Reverse-engineering prompt for another site

```text
Perform an evidence-based technical teardown of [URL]. Do not guess from appearance alone.

Inspect:
- response and CDN headers;
- HTML/framework markers;
- JS and CSS asset names;
- runtime/version strings;
- network-loaded models, videos, image sequences, fonts, and analytics;
- canvas/WebGL contexts;
- signatures for Three.js, R3F, GSAP ScrollTrigger, Lenis, Locomotive, Spline, Rive, and Lottie;
- sticky section geometry;
- CSS `scroll-timeline`, `view-timeline`, `animation-timeline`, and `animation-range`;
- scroll listeners, rAF loops, observers, Web Animations API calls, and CSS custom-property updates;
- reduced-motion and unsupported-browser fallbacks.

Label every conclusion Verified, Inferred, or Not detected. Separate the visual illusion from the actual rendering technology. Produce a layer diagram, 0–100% timeline, asset inventory, performance risks, accessibility risks, and an original implementation plan that uses the same engineering class without copying the site’s protected design.
```

## Prompt 18 — Final implementation review

```text
Review the completed 2.5D scroll website as a staff front-end engineer. First run the project and inspect it at key scroll positions. Then report and fix only evidence-backed issues.

Acceptance criteria:
- The initial composition is complete before motion begins.
- The focal point does not drift between breakpoints.
- Foreground scaling never exposes an edge or seam.
- The background moves less than the foreground.
- Copy remains readable and interactive for long enough.
- Backward scroll precisely reverses the scene.
- No meaningful content is permanently hidden if JS, images, or timeline support fails.
- Reduced motion is a coherent static design, not a broken animation frozen midway.
- 200% zoom and short screens use normal flow.
- Pointer parallax causes no React rerenders.
- Offscreen observers/loops are stopped.
- No custom smooth-scroll layer interferes with native input.
- There is no unexpected horizontal overflow.
- Tests cover mobile, desktop, WebKit, Chromium, Firefox, reduced motion, blocked images, and refresh mid-scroll.

Return: prioritized findings, patches, tests run, remaining risks, and a concise ship/no-ship recommendation.
```

## Prompt 19 — Scroll-directed Three.js portfolio (Meer-style technical class)

```text
Act as a senior creative developer. Build an original editorial portfolio that combines semantic HTML sections with real-time Three.js scenes controlled by scroll. Use Vite, TypeScript, Three.js, GSAP, ScrollTrigger, and glTF. Use Lenis only if user testing shows that native scroll does not achieve the desired feel. Do not copy Meer Mohsin’s red/black identity, triple-ring loader, statue, page structure, wording, or animations.

Architecture:
- Keep the page and all copy as HTML; mount transparent canvases only where real 3D adds value.
- Load one optimized glTF hero object and one smaller secondary environment.
- Use GLTFLoader with Meshopt or Draco only if compression savings justify decoder cost.
- Create one renderer per simultaneously visible scene at most; prefer reusing a renderer when feasible.
- Cap DPR by device tier and configure explicit mobile/desktop quality profiles.
- Drive model/camera properties from GSAP timelines with ScrollTrigger `scrub` and `pin`.
- Never animate the pinned element itself; animate nested DOM or Three.js objects.
- Use `invalidateOnRefresh` and function-based end distances for responsive geometry.
- Render only while a canvas is visible or a tween/pointer interaction is active.
- Dispose scene resources and kill every ScrollTrigger on teardown.

Motion system:
- Create one long pinned 3D chapter with a clear beginning, three camera beats, and a settled end.
- Use linear easing for direct scroll mapping; use numeric scrub only where intentional catch-up is desired.
- Add subtle pointer offsets after scroll transforms, composing them without overwriting base camera state.
- Coordinate DOM word/line reveals with camera beats using labels on one master timeline.
- Include one SVG draw sequence and one clip-path media reveal.
- Keep non-scroll ambient motion separable and pause it offscreen.

Loading and activation:
- Show an honest progress UI based on asset loading, not a decorative fixed timer.
- Do not block the entire site behind a click unless audio or pointer lock truly requires it.
- Start audio only after explicit opt-in; provide persistent mute state.
- Provide a static poster and ordinary document flow while 3D loads or fails.

Performance budgets:
- Initial JS transfer [BUDGET].
- Initial texture transfer [BUDGET].
- Maximum triangles [BUDGET].
- Maximum draw calls [BUDGET].
- Maximum decoded texture memory [BUDGET].
- DPR cap [VALUE].
- Define low/medium/high quality tiers using hardware concurrency, memory hints, viewport, and measured frame time.

Accessibility:
- Reduced motion replaces pinned camera travel with static scene posters and normal-flow sections.
- Canvas content has an HTML equivalent and never contains the only project information.
- Anchor navigation, keyboard scroll, history restoration, zoom, and nested scroll areas work with or without Lenis.

Deliver complete source, asset pipeline commands, timeline diagram, cleanup code, fallback behavior, Playwright tests, and a performance report.
```

## Prompt 20 — Lenis + GSAP integration audit

```text
Review or implement Lenis with GSAP ScrollTrigger safely.

If smooth scrolling is retained:
- Initialize one Lenis instance only.
- Call `ScrollTrigger.update` from the Lenis scroll event.
- Advance `lenis.raf(time * 1000)` from `gsap.ticker`.
- Document any `gsap.ticker.lagSmoothing(0)` decision and its downside.
- Exclude modal, code, form, and nested-scroll regions with explicit prevent rules.
- Preserve anchor links, browser back/forward, scroll restoration, keyboard paging, find-in-page, and focus scrolling.
- Stop/destroy Lenis during route transitions and remove ticker callbacks.
- Disable smoothing for reduced motion and any environment where it worsens input latency.
- Refresh ScrollTrigger after fonts, models, and responsive layout settle.

First determine whether Lenis is actually needed. Provide an A/B implementation using native scroll, measure INP/frame consistency, and recommend the simpler option if the visual difference is marginal.
```

## Prompt 21 — Procedural spatial-data interface (3DCC-style technical class)

```text
Design and implement an original Three.js spatial interface for [DATASET]. Position must encode three meaningful independent dimensions:
- x = [DIMENSION X]
- y = [DIMENSION Y]
- z = [DIMENSION Z]

Do not copy 3DCC’s cocktail dataset, @-origin metaphor, 27-zone system, gold/black visual identity, text, coordinate labels, or enter/take-off sequence. Invent a distinct conceptual model and interaction language.

Technical constraints:
- TypeScript modules with no UI framework unless the surrounding product already uses one.
- Generate simple geometry and labels procedurally; do not load a model unless it communicates data.
- Use a perspective camera, bounded fog, deliberate lighting, and DPR capped at 2 or lower by device tier.
- Convert label canvases to textures only at the resolution needed; cache and dispose them.
- Use instanced meshes when many points share geometry/material.
- Map data values to position, size, color, and sound through documented pure functions.
- Use Raycaster for pointer selection and a spatial index or reduced raycast set if point count grows.
- Separate click/tap from drag with a movement threshold.
- Implement orbit controls directly or with a maintained control module; support pointer capture and cancel events.
- Use an explicit state machine for overview, transition, detail, and return states.
- Pause rendering when the document is hidden and switch to on-demand rendering when idle.
- Handle resize, context loss/restoration, and resource disposal.

Interface requirements:
- A synchronized HTML search/filter/list provides keyboard and screen-reader access to every data point.
- Selecting in HTML focuses/highlights the 3D point; selecting in 3D updates an `aria-live` detail panel without excessive announcements.
- Offer top/front/side reset views and a “reduce depth” 2D projection.
- Provide a non-WebGL table/scatterplot fallback with the same filters and data.
- Reduced motion disables auto-rotation, animated camera travel, pulsing, and long path growth.

Deliver a data schema, mapping rationale, scene graph, state diagram, interaction specification, full implementation, automated tests, and an accessibility test plan.
```

## Prompt 22 — Procedural glass/iridescent data points

```text
Create a performant Three.js material system for luminous data points.

Start with MeshPhysicalMaterial using controlled roughness, clearcoat, clearcoat roughness, emissive color, and environment intensity. Prefer the engine’s supported iridescence feature when available. If a custom Fresnel/rainbow treatment is still needed, inject the smallest possible fragment-shader modification with `onBeforeCompile` and define a stable `customProgramCacheKey`.

Generate a lightweight environment map at runtime from a 2D canvas with two or three broad radial light blobs, convert it to an equirectangular CanvasTexture, prefilter it once with PMREMGenerator, assign the result to `scene.environment`, then dispose the source texture and generator.

Provide:
- quality tiers for 20, 200, and 2,000 points;
- an instancing-compatible alternative;
- a cheaper MeshStandard/ShaderMaterial fallback;
- correct transparent sorting/depth strategy;
- shader compilation warm-up;
- color-management settings;
- memory/resource disposal;
- screenshots on dark and light backgrounds;
- GPU frame-time comparison for each tier.

Avoid transparency and MeshPhysicalMaterial on thousands of overlapping points unless measurements prove it is viable.
```

## Prompt 23 — Neon relational path animation

```text
Build an original animated 3D path that communicates a relationship from a source point to a selected target.

Requirements:
- Generate an orthogonal or domain-specific route from source to target as ordered 3D segments.
- Render a three-layer glow using a narrow bright core plus wider low-opacity additive shells, or provide a postprocessing bloom alternative and compare costs.
- Animate three phases: grow from source, hold, retract/fade toward target or source.
- Update segment scale and midpoint without recreating geometry every frame.
- Add joint caps so bends do not show gaps.
- Keep the path visible through complex backgrounds without disabling depth testing indiscriminately; document the occlusion choice.
- Derive optional Web Audio pitch/pan from direction or data values, only after user opt-in.
- Dispose geometries/materials when the selection changes.
- Reduced motion shows an immediate static path or simple highlight.
- Mirror the relationship in plain text for nonvisual users.

Return geometry math, reusable class/API, animation state machine, tests for all axis/direction combinations, and profiling results.
```

## Prompt 24 — GPU fluid background as a bounded enhancement

```text
Implement a responsive pointer-reactive fluid background as an optional enhancement using WebGL2 ping-pong framebuffers. Keep it visually separate from the content and never make content depend on it.

Pipeline:
- velocity advection;
- dye advection;
- curl/vorticity force;
- divergence;
- iterative pressure solve;
- gradient subtraction;
- final compositing.

Requirements:
- Scale simulation and dye resolution independently by quality tier.
- Use half-float render targets only after extension/capability checks.
- Cap splat rate and batch pointer input.
- Run at reduced resolution on mobile.
- Pause when offscreen, hidden, idle for [N] seconds, or under reduced motion/data saver.
- Recover cleanly from WebGL context loss.
- Provide a CSS gradient/noise fallback.
- Do not intercept pointer events needed by UI.
- Track average GPU/frame time and reduce quality dynamically after sustained slow frames.
- Dispose every framebuffer, texture, program, buffer, and listener.

Deliver annotated shader modules, framebuffer lifecycle code, adaptive-quality controller, fallback, tests, and a performance comparison against a pre-rendered video alternative.
```

## Prompt 25 — Architecture chooser for 3D web concepts

```text
Evaluate this concept before implementation: [DESCRIBE CONCEPT]. Choose exactly one primary architecture:
A. layered DOM/CSS 2.5D with native scroll timelines;
B. document + Three.js canvases orchestrated by GSAP ScrollTrigger;
C. full-viewport procedural Three.js spatial interface.

Score each option from 1–5 for narrative fit, interaction need, visual fidelity, accessibility, SEO, initial load, runtime performance, mobile viability, authoring cost, maintenance cost, and fallback quality. Explain every score with reference to the actual concept.

Decision rules:
- If users only watch a controlled push/pan through fixed art, prefer A.
- If real models/cameras must change during a document narrative, consider B.
- If spatial relationships and direct exploration are the product, consider C.
- Do not choose WebGL merely for prestige.
- Do not choose smooth-scroll mediation without a measured UX reason.

Return the decision, smallest viable prototype, asset plan, performance budgets, accessibility equivalent, failure fallback, and the evidence that would cause you to switch architectures.
```

---

## Recommended build sequence

1. Define the metaphor, focal point, and layer stack.
2. Create a static responsive composition first.
3. Export and validate responsive image assets.
4. Add the tall wrapper and sticky stage.
5. Add native view-timeline keyframes one layer at a time.
6. Add semantic copy and phase transitions.
7. Build unsupported/reduced-motion/short-screen fallbacks.
8. Add reveal, pointer, atmosphere, and footer systems.
9. Add decode/degraded-state handling.
10. Capture keyframe screenshots and profile performance.
11. Test reverse scroll, refresh mid-scene, keyboard, touch, and blocked assets.

## Common failure modes

- Calling every dimensional landing page “Three.js” without inspecting it.
- Scaling a single flattened image, which makes every depth plane move identically.
- Using center-center transform origins when the art has a different vanishing point.
- Baking copy into artwork.
- Hiding server-rendered content before JavaScript is known to work.
- Using `animation` shorthand after `animation-timeline`, which silently resets the custom timeline.
- Animating width, height, top, or left on every scroll frame.
- Combining horizontal drift and vertical parallax on one `transform`, causing one animation to overwrite the other.
- Applying `will-change` permanently to dozens of large layers.
- Leaving transparent padding around images, inflating decode memory and paint bounds.
- Keeping observers and rAF loops active offscreen.
- Treating reduced motion as “slower motion” rather than providing a stable composition.
- Forgetting short screens, browser zoom, refresh at an existing scroll position, and reverse scroll.
- Adding smooth-scroll libraries to a design that already works with native scroll.

## Sources and research notes

- [Aristotle homepage](https://www.heyaristotle.com/) — live UI, HTML, public CSS/JS assets, fonts, and media paths inspected directly.
- [Meer Mohsin portfolio](https://www.meermohsin.me/) — live UI, public HTML, CSS/JS bundle, model/media paths, and runtime signatures inspected directly.
- [3DCC Core live experience](https://cocktailtheory.github.io/3DCC-core/) — public single-file implementation inspected directly.
- [3DCC Core repository and concept notes](https://github.com/cocktailtheory/3DCC-core) — public implementation context, interaction rationale, status, and rights notice.
- [MDN: CSS scroll-driven animations](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Scroll-driven_animations) — overview of scroll and view progress timelines.
- [MDN: Scroll-driven animation timelines](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Scroll-driven_animations/Timelines) — named view timelines, ranges, and performance context.
- [W3C: Scroll-driven Animations specification](https://www.w3.org/TR/scroll-animations/) — normative model for scroll/view timelines.
- [WebKit: A guide to scroll-driven animations with CSS](https://webkit.org/blog/17101/a-guide-to-scroll-driven-animations-with-just-css/) — practical platform guidance.
- [Chrome for Developers: Scroll-driven animations](https://developer.chrome.com/docs/css-ui/scroll-driven-animations) — implementation concepts and examples.
- [GSAP ScrollTrigger documentation](https://gsap.com/docs/v3/Plugins/ScrollTrigger/) — official scrub, pin, range, refresh, and lifecycle behavior.
- [Three.js WebGLRenderer documentation](https://threejs.org/docs/pages/WebGLRenderer.html) — renderer setup, compilation, and performance controls.
- [Three.js Raycaster documentation](https://threejs.org/docs/pages/Raycaster.html) — pointer-to-scene intersection model.
- [Three.js Material documentation](https://threejs.org/docs/pages/Material.html) — `onBeforeCompile` behavior and shader customization guidance.
- [Three.js MeshPhysicalMaterial documentation](https://threejs.org/docs/pages/MeshPhysicalMaterial.html) — clearcoat, iridescence, and performance considerations.
- [Bruno Simon portfolio](https://bruno-simon.com/) — official live spatial portfolio, controls, rendering choices, and HTML fallback.
- [Joseph Santamaria portfolio case study](https://tympanus.net/codrops/2026/04/28/more-than-a-portfolio-building-a-scroll-driven-3d-world-with-something-to-say/) — creator-side discussion of Three.js/GSAP production, asset optimization, and mobile rendering.
- [ZERO engineering case study](https://tympanus.net/codrops/2026/07/17/zero-the-engineering-behind-a-defiant-interactive-narrative/) — creator-side account of interaction, stack, optimization, and performance targets.
- [The Spark engineering case study](https://tympanus.net/codrops/2026/01/09/the-spark-engineering-an-immersive-story-first-web-experience/) — creator-side description of a cables.gl/Webflow hybrid.
- [Lusion WebGL Scroll Sync](https://webgl-scroll-sync.lusion.co/) — public scroll/scene synchronization demo.
- [Reactive Depth tutorial](https://tympanus.net/codrops/2026/02/17/reactive-depth-building-a-scroll-driven-3d-image-tube-with-react-three-fiber/) — scroll inertia and shader deformation in React Three Fiber.
- [Theatre.js camera fly-through tutorial](https://tympanus.net/codrops/2023/02/14/animate-a-camera-fly-through-on-scroll-using-theatre-js-and-react-three-fiber/) — authored camera animation mapped to scroll.
- [Progressively enhanced WebGL lens](https://tympanus.net/codrops/2023/10/10/progressively-enhanced-webgl-lens-refraction/) — semantic DOM coordinated with WebGL via scroll-rig techniques.
- [Scroll, refraction, and shaders tutorial](https://tympanus.net/codrops/2019/12/16/scroll-refraction-and-shader-effects-in-three-js-and-react/) — real scroll layout, shader effects, and instancing.
- [Chang Liu portfolio V4 case study](https://tympanus.net/codrops/2019/10/16/case-study-chang-liu-portfolio-v4/) — shader-based portfolio distortion.
- [Apple-style image sequence lesson](https://francescocastronuovo.com/learn/lessons/apple-style-image-sequence-webflow/) — fixed-canvas frame sequence controlled with ScrollTrigger.
- [Infinite Canvas tutorial](https://tympanus.net/codrops/2026/01/07/infinite-canvas-building-a-seamless-pan-anywhere-image-space/) — recycled spatial gallery implementation.
- [AIReiter 3D website prompts](https://aireiter.com/blog/3d-website-prompts) — six public prompt examples and the author's test notes.
- [AgentOS interactive 3D portfolio prompt log](https://agentos.guide/opus-5-5-portfolio) — public large-prompt examples and build artifacts.
- [AETUMI AI 3D website guide](https://aetumi.app/news/how-to-build-a-3d-website/) — vendor-authored template-modification workflow and production checklist.
- [MotionSites 3D scroll lesson](https://motionsites.ai/lesson/build-3d-scroll-animated-website-with-ai) — vendor-authored multiview-to-GLB-to-code prompt chain.
- [VULK AI 3D website guide](https://vulk.dev/blog/how-to-build-a-3d-website-with-ai-complete-guide) — vendor-authored overview of prompt-led and visual-builder workflows.

### Scope caveat

The Aristotle and Meer Mohsin sections are black-box reviews of public deployed builds on the inspection date, not access to their private source repositories or design files. Exact internal package names and authoring tools cannot always be proven from minified output. 3DCC Core was additionally checked against its public repository. Public assets, hashes, versions, and implementations can change at any time. External examples mix official sites, creator-authored case studies, tutorials, and clearly labeled vendor material; inclusion is not an endorsement. The public-prompt catalog is a condensed research record, not a verbatim republication.
