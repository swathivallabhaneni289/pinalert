---
phase: 01-foundation-report-map
plan: 02
subsystem: frontend-design-system
tags: [css, design-tokens, dark-mode, icons, accessibility]
status: complete
requirements: [FOUND-02, FOUND-04]
dependency-graph:
  requires: []
  provides:
    - "web/static/css/main.css (complete token set + shared component/layout classes)"
    - "web/static/css/modal.css (placeholder, owned by 01-05)"
    - "web/static/css/feed.css (placeholder, owned by 01-06)"
    - "web/static/icons/{flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other}.svg"
  affects:
    - "01-04 (page shell — links all three stylesheets, consumes .app-shell/.pane/.fab/.btn*/.modal-*/.severity-slider/.category-tile*)"
    - "01-05 (report modal — consumes .modal-backdrop/.modal-panel/.severity-slider/.category-tile*/.btn--destructive)"
    - "01-06 (feed list — consumes .report-row*/.sev-*/.age-*/.icon-badge*/.empty-state/.error-state/.skeleton-row)"
tech-stack:
  added: []
  patterns:
    - "CSS custom-property token layer with prefers-color-scheme + [data-theme] override, no filter/invert dark mode"
    - "Two-tier severity contract: .sev-* sets --severity-base/--severity-tint and a --severity-current default; .age-* overrides --severity-current at equal specificity via source order"
    - "Age-ramp desaturation via color-mix(), never opacity"
key-files:
  created:
    - web/static/css/main.css
    - web/static/css/modal.css
    - web/static/css/feed.css
    - web/static/icons/flood.svg
    - web/static/icons/earthquake.svg
    - web/static/icons/fire.svg
    - web/static/icons/storm_cyclone.svg
    - web/static/icons/road_blocked.svg
    - web/static/icons/power_outage.svg
    - web/static/icons/shelter_open.svg
    - web/static/icons/rescue_needed.svg
    - web/static/icons/other.svg
  modified: []
decisions:
  - "Added a --severity-current default inside each .sev-* rule (not only inside .age-*) so a bare .icon-badge.sev-critical or .report-row.sev-medium with no age class still renders its severity color, rather than resolving --severity-current to nothing. .age-* rules are placed after .sev-* in source order to override it at equal specificity."
  - "Skeleton-row pulse animates background-color between --color-surface and --color-border instead of opacity; modal-backdrop dim uses rgb(0 0 0 / 55%) instead of the opacity property — both to satisfy the plan's 'no fractional opacity anywhere' constraint (D-17's opacity/desaturation distinction) without changing the intended visual effect."
  - "Added a base .pane class (in addition to .pane--map/.pane--list) after finding plan 01-04 references `class=\"pane pane--map\"` / `class=\"pane pane--list\"` — the plan's own action text only named the two modifier classes, but the shell markup needs a shared base class too. Verified against 01-04/01-05/01-06's actual markup and JS references before adding, per the plan's stated goal that those three plans build their surface using only this contract."
  - "Used :root[data-theme=\"dark\"] / :root[data-theme=\"light\"] (attribute selector on :root, specificity 0,2,0) rather than a bare [data-theme] selector, so an explicit theme choice always beats the prefers-color-scheme media query regardless of source order."
metrics:
  duration: "~35 min"
  completed: 2026-09-06
---

# Phase 1 Plan 02: Design System Token Layer & Category Icons Summary

Complete `main.css` design-token layer (spacing, typography, light/dark surfaces, traffic-light
severity palette, age-desaturation ramp) plus every shared component/layout class the three later
frontend plans (01-04, 01-05, 01-06) consume, two placeholder stylesheets, and nine self-hosted
Lucide category glyphs — the full visual contract for wave 4.

## What Was Built

**Task 1 — Token layer** (`web/static/css/main.css`): all 23 tokens from 01-UI-SPEC.md's Spacing,
Typography, and Color sections, declared on `:root` with dark overrides inside
`@media (prefers-color-scheme: dark)` plus an explicit `:root[data-theme="dark"|"light"]` manual
override for a future toggle. Dark severity/surface hex values are transcribed directly from
UI-SPEC (not computed via filter/invert) — dots get brighter/more saturated, tint backgrounds get
darker, per D-13.

**Task 2 — Shared components & layout** (extends `main.css`; creates `modal.css`, `feed.css`
placeholders): severity context (`.sev-low/medium/critical`), age ramp (`.age-fresh/aging/stale`
via `color-mix()`), `.icon-badge` (+ `--sm`/`--pin` variants), `.report-row` (+ `--selected`),
neutral `.fab`/`.btn`/`.btn--primary`/`.btn--destructive`/`.category-tile` (+ `--selected`),
`.app-shell` split-view layout with a 900px breakpoint and `data-view` attribute swap for narrow
screens, `.skeleton-row`, `.empty-state`, `.error-state`, `.visually-hidden`, `.severity-slider`,
`.modal-backdrop`/`.modal-panel`, and a global `:focus-visible` rule.

**Task 3 — Category glyphs** (`web/static/icons/*.svg`): nine Lucide outline icons fetched from
the pinned `lucide-static@1.41.0` distribution via unpkg, renamed to category slugs, normalized
(fixed `width`/`height="24"` replaced with `100%`/`100%`, `viewBox` retained; `aria-hidden="true"`
and `focusable="false"` added). `stroke="currentColor"` / `fill="none"` preserved as-is (outline
style, no fill conversion). No icon-name substitutions were needed — all nine names confirmed
present exactly as mapped in 01-UI-SPEC.md and the research pass.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - missing critical functionality] `--severity-current` undefined without an `.age-*` class**
- **Found during:** Task 2, before implementation (caught via review of the plan's own success
  criteria against its class-contract description)
- **Issue:** As literally specified, only `.age-*` classes set `--severity-current`; `.sev-*`
  classes set only `--severity-base`/`--severity-tint`. The plan's own success criterion
  ("A `.icon-badge.sev-critical`... renders a white life-buoy glyph on a solid red circle") uses
  no `.age-*` class, so `--severity-current` would resolve to nothing and the badge would render
  transparent.
- **Fix:** Each `.sev-*` rule also declares `--severity-current: var(--severity-base)` as a
  default. The three `.age-*` rules are placed after `.sev-*` in file order so they override
  `--severity-current` at equal specificity (one class each) when both are applied together.
  Components additionally read `var(--severity-current, var(--severity-base))` as a fallback.
- **Files modified:** `web/static/css/main.css`
- **Commit:** 6b10829

**2. [Rule 1 - bug] Task 2's own third verify forbids fractional `opacity:`, which the natural implementation of the skeleton pulse and modal dim would trip**
- **Found during:** Task 2, before implementation
- **Issue:** A `@keyframes` skeleton pulse naturally uses `opacity: 0.5`; a modal backdrop dim
  naturally uses `opacity: 0.6`. Both would fail the plan's own automated verify
  (`grep -cE '^\s*opacity:\s*0?\.[0-9]+'` must be 0) and, more importantly, would violate D-17's
  distinction that a faded element reads as "disabled" while age-desaturation must read as
  "going stale" — the same reasoning extends to not using opacity for other UI dimming either.
- **Fix:** Skeleton pulse animates `background-color` between `--color-surface` and
  `--color-border`. Modal backdrop dim uses `background: rgb(0 0 0 / 55%)` (a color-channel
  alpha, not the `opacity` property).
- **Files modified:** `web/static/css/main.css`
- **Commit:** 6b10829

**3. [Rule 2 - missing critical functionality] Added a base `.pane` class**
- **Found during:** Task 2, cross-checked against 01-04/01-05/01-06's actual markup before
  writing, per the plan's stated goal that those plans build their surface using only this
  file's contract with no need to edit `main.css`.
- **Issue:** The plan's Task 2 action text names only `.pane--map` and `.pane--list` as layout
  classes. Plan 01-04's markup uses `class="pane pane--map"` / `class="pane pane--list"` — a
  shared base class plus a modifier — which `main.css` as literally specified would leave
  undeclared (harmless but incomplete: 01-04 would apply a class with no rule).
- **Fix:** Added `.pane { min-height: 0; overflow: hidden; position: relative; }` alongside the
  existing `.pane--map`/`.pane--list` breakpoint rules.
- **Files modified:** `web/static/css/main.css`
- **Commit:** 6b10829

No other deviations. All three tasks' automated verify commands pass as specified in the plan.

## Pending Human Verification

Task 3 carries a `<human-check>` (not a `checkpoint:*` gate — plan is `autonomous: true`,
`human_verify_mode` is `end-of-phase`): open each of the nine SVGs and confirm the glyph matches
its category semantically, in particular that `power_outage` shows a slashed bolt (not a plain
bolt), `earthquake` shows a seismograph zigzag, and `other` shows a plain flag, and that none
render as a broken/empty box. As a proxy check during this execution, the path data was inspected
directly:
- `power_outage.svg` (zap-off) contains a diagonal slash path `m2 2 20 20` cutting across the bolt
  shape — matches "slashed bolt, not a plain bolt".
- `earthquake.svg` (activity) is a single jagged multi-segment path — matches "seismograph-style
  zigzag".
- `other.svg` (flag) is a simple flag-on-pole outline.
- All nine files are well-formed `<svg>` elements with non-empty `<path>`/`<circle>`/`<rect>`
  children (visually inspectable, not literally rendered in a browser during this automated pass).

This item should be confirmed visually at end-of-phase per project config.

## Self-Check: PASSED

Verified all claimed files exist and all claimed commits are present in git history (see below).
