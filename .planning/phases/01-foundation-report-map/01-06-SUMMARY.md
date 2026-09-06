---
phase: 01-foundation-report-map
plan: 06
subsystem: frontend-report-feed
tags: [vanilla-js, dom-reconciliation, accessibility, leaflet, xss-safety]
status: complete
requirements: [FOUND-03, FOUND-04, FOUND-05]

dependency-graph:
  requires:
    - phase: 01-foundation-report-map (plan 01-02)
      provides: web/static/css design tokens, .report-row/.sev-*/.age-*/.icon-badge/.skeleton-row/.empty-state/.error-state/.view-toggle
    - phase: 01-foundation-report-map (plan 01-03)
      provides: GET /api/reports (category, severity, shelter_capacity_status, shelter_headcount fields)
    - phase: 01-foundation-report-map (plan 01-04)
      provides: window.Pinalert shared store (subscribe/select/onSelect/severityClass/ageStage/iconPath/relativeTime/setText), window.PinalertMap.flyTo, index.html.tmpl DOM contract
  provides:
    - "web/static/js/feed.js — severity-first/newest-first report list, age-ramp desaturation, loading/empty/error states, bidirectional map/list linkage, narrow-screen view toggle"
    - "web/static/css/feed.css — list container, row internals, icon-badge img sizing, [hidden] specificity fix, narrow-screen padding"
  affects:
    - "Phase 2 (Trust Engine) — the meta line format ({Category} · {relative time}) and row structure this plan ships is exactly where a confirmation count will be added later; do not re-derive the row layout from scratch"

tech-stack:
  added: []
  patterns:
    - "DOM reconciliation by report id (insertBefore-based reorder, not clear-and-rebuild) so scroll position and open state survive a background poll"
    - "CSS custom-property inheritance: severity/age classes are applied once to the <li>.report-row, not duplicated onto the child .icon-badge — --severity-current cascades down automatically"
    - "Leaflet size invalidation without access to the map instance: dispatch a window 'resize' event, which Leaflet's default trackResize option already listens for internally"
    - "[hidden] attribute + a scoped #id[hidden]{display:none} override, needed because an author-origin display value elsewhere always beats the [hidden] user-agent rule regardless of source order"

key-files:
  created: []
  modified:
    - web/static/js/feed.js
    - web/static/css/feed.css

decisions:
  - "Sort a slice()-copied array for severity-first/newest-first ordering (D-08); the shared Pinalert.state.reports array is never mutated, since map.js renders from the same reference"
  - "Category icon-badge sev-*/age-* classes live on the <li>.report-row only, relying on CSS custom-property inheritance to reach the child badge, rather than duplicating classes on both elements as map.js's standalone pin badges do"
  - "View-toggle icon built via inline SVG DOM nodes (createElementNS) rather than a new icon asset file — UI-SPEC's Icon Mapping table only covers the 9 category icons, and adding a new /static/icons/*.svg file would fall outside this plan's two-file ownership"

metrics:
  duration: ~55 minutes
  completed: 2026-09-06
---

# Phase 01 Plan 06: Report Feed — List, Age Ramp, Map/List Linkage Summary

Renders the nearby-reports feed as a severity-ordered, age-desaturating list bound to the same shared store the map reads from, with full bidirectional map/list linkage and a narrow-screen view toggle — the "alongside other nearby reports" half of the Phase 1 goal.

## What Was Built

`web/static/js/feed.js` was rewritten wholesale (replacing plan 01-04's no-op placeholder) to:

- Subscribe to `Pinalert.subscribe` and render `state.reports` into `#report-list`, performing no fetch of its own — the list and the map read from the exact same fetched array, so they cannot drift apart (D-06).
- Sort a copy of the reports array severity-first (critical → medium → low), then newest-first within each band (D-08), never mutating the shared array.
- Reconcile rows by report id across polls (id-keyed `insertBefore` repositioning, create-on-miss, remove-on-gone) rather than clearing and rebuilding the list every 30 seconds, so scroll position and open state survive a background refresh.
- Render each row as a `.report-row` `<li>` carrying `Pinalert.severityClass(report)` and `age-{fresh,aging,stale}` from `Pinalert.ageStage(report)` (re-evaluated on a 1-minute interval, along with the relative-time text, since the stage is a function of elapsed time rather than a server push). The severity/age classes live only on the `<li>`; the child `.icon-badge` inherits `--severity-current` via normal CSS custom-property inheritance rather than needing the same classes applied twice.
- Build the meta line as `{Category} · {relative time}`, extended with capacity status and headcount for `shelter_open` reports, and route every report-authored string through `Pinalert.setText` (never a markup-parsing sink) per T-01-03.
- Drive four states — loading/idle, error, empty, and populated — from `state.status` alone, toggling the `hidden` attribute on `#feed-skeleton` / `#feed-empty` / `#feed-error` / `#report-list`. The exact empty-state and error-state copy already lives in `index.html.tmpl` (plan 01-04's markup); this plan only shows/hides it and wires `#feed-retry` to `Pinalert.fetchReports()`. A fetch failure never clears existing rows or (indirectly, since the map keeps its own pins) the map's markers.
- Wire bidirectional map/list linkage: activating a row (click or Enter/Space) calls `Pinalert.select(id, 'list')` and `PinalertMap.flyTo(id)`; a map-originated selection (`Pinalert.onSelect` with `source === 'map'`) highlights and scrolls the matching row into view. Each direction is guarded on `source` so neither echoes back on itself.
- Wire the narrow-screen view toggle: flips `#app-shell`'s `data-view` attribute, labels itself by destination ("List" while on the map, "Map" while on the list, per the Copywriting Contract) with a small inline-SVG icon built via `createElementNS` (no new icon asset — UI-SPEC's Icon Mapping only covers the 9 category icons), and dispatches a `window` `resize` event after swapping back to map view so Leaflet's own `trackResize` handling calls `invalidateSize()` internally (map.js's returned API is only `{flyTo, highlight}`, so this file cannot call it directly). Selecting a row while the narrow-screen list view is active switches back to the map view automatically. Focus moves to the newly revealed pane's first interactive element after every swap.

`web/static/css/feed.css` adds only what main.css doesn't already provide: the `#report-list` reset/layout, a scoped `.report-row .icon-badge img` sizing rule (main.css's existing `.icon-badge svg` rule targets inline `<svg>`, not the `<img>` this plan uses to load the shared icon files), the view-toggle icon's `flex-shrink`, a `[hidden]` specificity fix (see Deviations), and exactly one `@media (max-width: 899.98px)` block for bottom padding so the reparented toggle button doesn't obscure the last row.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `[hidden]` attribute was a no-op on the four feed state containers**

- **Found during:** Task 2, while verifying the loading/empty/error state machine actually toggled visibility.
- **Issue:** `main.css`'s `.empty-state, .error-state { display: flex; }` is an author-origin declaration. Author-origin `display` always wins over the user-agent stylesheet's `[hidden] { display: none; }`, regardless of selector specificity or source order (origin trumps specificity in the CSS cascade). Setting/removing the `hidden` attribute from `feed.js` therefore had zero visual effect on `#feed-empty` and `#feed-error` — both would already have been rendered on first paint despite the `hidden` attribute in the shipped markup, stacked under the skeleton. The same trap applies to `#report-list`/`#feed-skeleton` the moment either gets an author `display` value (which `#report-list` needed, for the flex-column row layout).
- **Fix:** Added a scoped override in `feed.css`: `#report-list[hidden], #feed-skeleton[hidden], #feed-empty[hidden], #feed-error[hidden] { display: none; }`. An `[id][hidden]` selector is specificity (1,1,0), which beats `.empty-state`/`.error-state` at (0,1,0) regardless of source order, and `feed.css` also loads last in `index.html.tmpl`. Entirely contained within this plan's owned CSS file.
- **Files modified:** `web/static/css/feed.css`
- **Commit:** 1a31ba9

**2. [Rule 1 - Bug] `#view-toggle` vanished entirely in narrow-screen map view**

- **Found during:** Task 3, while tracing the narrow-screen toggle's visibility rules.
- **Issue:** `index.html.tmpl` ships `#view-toggle` as a child of `.pane--list`. `main.css`'s narrow-screen rules set `.pane--list { display: none; }` while `data-view="map"`. A `display: none` ancestor removes its entire subtree from rendering — no CSS on a descendant (including the button's own `position: fixed`) can override that. The result: on narrow screens, once a visitor switched to Map view, the only control that could switch back to List view disappeared with it — a dead end on the exact interaction path D-07 exists to guarantee.
- **Fix:** `feed.js` reparents `#view-toggle` to `#app-shell` (a sibling of both panes) on load, before wiring its click handler. `.view-toggle`'s CSS rules are all element-scoped (never a descendant selector), so its appearance and fixed positioning are unaffected by which element contains it, and `main.css`'s breakpoint rules remain the sole, untouched authority — no override, no `!important`, no CSS added to this plan's one-media-query budget for the fix itself. Verified: the toggle now renders and is clickable at narrow widths in both map and list view, and stays `display: none` at ≥900px per `main.css`'s baseline rule.
- **Files modified:** `web/static/js/feed.js`
- **Commit:** 1a31ba9

### Auth Gates

None — this plan has no authentication surface.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources — the list renders live from `Pinalert.state.reports`.

## Threat Flags

None. All row content passes through `Pinalert.setText`; category/severity-derived classes and icon paths are produced by `Pinalert.severityClass`/`Pinalert.ageStage`/`Pinalert.iconPath`, which validate against fixed enums (T-01-03, T-01-17 — pre-existing mitigations this plan reuses, does not extend). No new network endpoint, auth path, or schema surface introduced.

## Observations (not deviations — outside this plan's ownership)

- `map.js`'s pin badges also render an `<img src="...svg">` inside `.icon-badge`, and `main.css`'s `.icon-badge svg { width: 55%; height: 55%; }` rule only targets an inline `<svg>` element, not an `<img>` referencing one. The same missing-intrinsic-size gap this plan fixed for its own rows (via `.report-row .icon-badge img` in `feed.css`, scoped so it cannot reach map.js's markers) likely also affects map pins. Not fixed here — `map.js`/`map.css` are outside this plan's `files_modified` and owned by plan 01-05's wave, not filed to a shared `deferred-items.md` to avoid a merge conflict with that sibling agent's concurrent work in the same wave.

## Verification

- `node --check web/static/js/feed.js` — passes.
- Task 1 grep gates (no innerHTML-equivalent sink, `Pinalert.subscribe`/`setText` present, zero `/api/reports` references, zero trust-signal copy, zero raw hex in `feed.css`) — all pass.
- Task 2 grep gates (`ageStage`/state-container ids present, zero JS-computed opacity, zero client-side expiry filter, all four state copy strings present in `feed.js` or `index.html.tmpl`) — all pass.
- Task 3 grep gates (`PinalertMap.flyTo`/`Pinalert.onSelect`/`Pinalert.select`/`data-view`/`report-row--selected` present, `invalidateSize` mechanism documented, `feed.css` has exactly one `@media` block and zero raw hex) — all pass.
- Upstream contract check (every DOM hook, `main.css` class, and `window.Pinalert`/`PinalertMap` member this plan depends on) — intact.
- `go build ./...`, `go vet ./...`, `go test ./... -v -p 1` (substituted `-p 1` for the plan's bare `-v` per the parallel-execution note on a known cross-package Postgres test-isolation race) — all packages pass; this plan touches no Go files.
- `git status --short` confirms only `web/static/js/feed.js` and `web/static/css/feed.css` were modified.
- The Task 3 `<human-check>` visual/interaction walkthrough (map/list side-by-side, click-to-fly, click-to-highlight, severity ordering, age desaturation, narrow-screen toggle, empty/error states, dark mode) is deferred to end-of-phase manual verification per `01-VALIDATION.md`/project config (`human_verify_mode: "end-of-phase"`) — not run by this executor.

## Self-Check: PASSED

- FOUND: `web/static/js/feed.js` exists — confirmed via `git show HEAD:web/static/js/feed.js` (staged and committed).
- FOUND: `web/static/css/feed.css` exists — confirmed via `git show HEAD:web/static/css/feed.css` (staged and committed).
- FOUND: commit `1a31ba9` — confirmed via `git log --oneline -1`.
