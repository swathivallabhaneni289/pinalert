---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-11"
subsystem: ui
tags: [maplibre, leaflet, openfreemap, theme, mutationobserver, contract-test]

# Dependency graph
requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility
    provides: "plan 02-10's System/Light/Dark theme control, which writes data-theme on document.documentElement"
provides:
  - "map.js and modal.js resolve their MapLibre basemap style from the live app theme instead of a hardcoded literal"
  - "Both map surfaces re-style their retained vector layer on a later theme switch via an attribute-filtered MutationObserver"
  - "A rewritten host allowlist test that pins a closed two-URL set rather than one literal, and blocks an inline URL in the construction call"
  - "A new gate (TestMapBasemapFollowsTheAppTheme) that fails the build if either map module loses its theme wiring, or if theme.js grows a dependency on the maps"
affects: [02-UAT, future-map-work]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Theme-reactive basemap: a fixed key->URL lookup validated against a DOM attribute (T-01-17 discipline), re-resolved on every call rather than cached, feeding both initial construction and a later re-style"
    - "DOM-observation instead of event dispatch for one-way, opt-in cross-module signaling — theme.js stays unaware the map modules exist"

key-files:
  created: []
  modified:
    - web/static/js/map.js
    - web/static/js/modal.js
    - web/js_contract_test.go

key-decisions:
  - "Observe document.documentElement via MutationObserver rather than have theme.js dispatch a CustomEvent, so theme.js needed zero changes and stays usable on pages with no map"
  - "Re-resolve the theme mode fresh on every call, never cache it, because System mode is expressed by the data-theme attribute's absence rather than a value"
  - "Accept that the WebGL-less OpenStreetMap raster fallback stays light-only in dark mode — no free dark raster tileset exists, and adding a second raster vendor for one degraded path would add an unreviewed host receiving visitor coordinates"
  - "Deliberately kept the mode resolver, style lookup and observer duplicated per-file (map.js and modal.js) rather than extracted into a shared module, matching this codebase's stated convention"

patterns-established: []

requirements-completed: [UX-01]

coverage:
  - id: D1
    description: "map.js resolves the primary map's basemap from the live theme at construction, retains the vector layer, and re-styles it on a later Light/Dark/System switch without throwing when the renderer isn't built yet"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestMapBasemapFollowsTheAppTheme"
        status: pass
      - kind: unit
        ref: "web/js_contract_test.go#TestBasemapBranchesPointAtCorrectHosts"
        status: pass
    human_judgment: true
    rationale: "Static inspection proves the mode read, the observer registration and the style-swap call are wired and reachable — it cannot prove a real browser actually repaints the tiles when the theme changes. That visual confirmation is this plan's own end-of-phase human check (workflow.human_verify_mode: end-of-phase), deliberately deferred, not skipped."
  - id: D2
    description: "The report-submission modal's own pin-drop map (modal.js) follows the theme too, on first open and a later switch, so it does not stay light while the rest of the app goes dark"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestMapBasemapFollowsTheAppTheme"
        status: pass
      - kind: unit
        ref: "web/js_contract_test.go#TestBasemapBranchesPointAtCorrectHosts"
        status: pass
    human_judgment: true
    rationale: "Same static-inspection limit as D1: this surface's own renderer readiness is even harder to reason about (the map is built lazily on first modal open), so the actual repaint needs the end-of-phase human check, which explicitly covers reopening the modal after a theme switch."
  - id: D3
    description: "The host allowlist test is rewritten to a closed two-URL set that still fails on an unreviewed host and additionally fails on an inline URL in the construction call; theme.js stays byte-identical"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestBasemapBranchesPointAtCorrectHosts"
        status: pass
      - kind: unit
        ref: "web/js_contract_test.go#TestMapBasemapFollowsTheAppTheme"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-23
status: complete
---

# Phase 02 Plan 11: Map basemap follows the app theme Summary

**Both map.js and modal.js now resolve their MapLibre style from a validated data-theme read (falling back to prefers-color-scheme), retain their vector layer, and re-style it live via an attribute-filtered MutationObserver — closing UAT gap Test 10 on both map surfaces, not just the primary one.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-09-23T09:30:13Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `map.js` and `modal.js` each carry a fixed two-entry `BASEMAP_STYLES` lookup (light → OpenFreeMap `liberty`, dark → OpenFreeMap `dark`) and a `currentBasemapMode()` resolver that validates `document.documentElement.getAttribute('data-theme')` against that lookup's own keys before it selects a URL (T-01-17), falling back to `window.matchMedia('(prefers-color-scheme: dark)')` when the attribute is absent (System mode), never memoised.
- Both modules retain their MapLibre-Leaflet bridge layer (`vectorLayer` / `modalVectorLayer`) and register one `MutationObserver` on `document.documentElement` with `attributeFilter: ['data-theme']` that re-resolves the mode and calls `.setStyle(...)` on the retained layer's renderer — guarded three ways (layer set, `getMaplibreMap` present, renderer truthy) so a switch arriving before the renderer exists is a safe no-op instead of a thrown exception.
- `web/js_contract_test.go`'s `TestBasemapBranchesPointAtCorrectHosts` is rewritten: the vector half now asserts the construction call's `style` option does not inline a URL (`http` substring check) and separately pins the host by requiring each of the two reviewed URLs to appear exactly once per module with no other `https://tiles.` occurrence in the file.
- A new gate, `TestMapBasemapFollowsTheAppTheme`, fails the build if either module loses its mode resolution, its system-preference fallback, its attribute-filtered observer, its retained-layer accessor, or a style-swap call reachable from the observer registration — and fails if `theme.js` ever references `MutationObserver` or `PinalertMap`.
- `theme.js` was not touched — `git diff web/static/js/theme.js` is empty throughout, verified after every task.

## Task Commits

Each task was committed atomically:

1. **Task 1: Both map modules resolve their basemap from the live theme, and re-style when it changes** - `288a5d2` (feat)
2. **Task 2: Rewrite the host allowlist for two styles, and gate the theme wiring itself** - `fb88e64` (test)

_Note: Task 2 rewrites an existing test and adds a new one in the same file/commit, per the plan's `tdd="true"` framing — the RED state this closed was Task 1's own commit leaving `TestBasemapBranchesPointAtCorrectHosts` failing by design (confirmed via `go test -run TestBasemapBranchesPointAtCorrectHosts` immediately after Task 1, before Task 2's edits)._

## Files Created/Modified
- `web/static/js/map.js` — primary map: theme-aware style lookup, mode resolver, retained vector layer, attribute-filtered re-style observer
- `web/static/js/modal.js` — report modal's pin-drop map: the same four changes, duplicated per this file's own stated convention
- `web/js_contract_test.go` — host allowlist rewritten for two URLs instead of one; new `TestMapBasemapFollowsTheAppTheme` gate

## Decisions Made

- **Two files, not one.** `modal.js`'s pin-drop map duplicates `map.js`'s basemap construction on purpose (that file's own comment records the duplication as this codebase's deliberate convention, not a copy-paste slip). A fix scoped to `map.js` alone would have passed every existing gate and still failed the same UAT test one screen deeper, on the report-submission modal. This was flagged in the plan's diagnosis and confirmed while reading `modal.js` lines 308–424 before writing any code — both files needed the identical four-part change, written out separately rather than extracted into a shared helper.

- **The accepted raster limitation, stated plainly.** OpenFreeMap publishes no free dark raster tileset — the dark style is vector-only — so the WebGL-less OpenStreetMap raster fallback stays light in every theme, in both modules. This is a decision with a reason, recorded in three places: a code comment next to each raster branch (naming `TestBasemapBranchesPointAtCorrectHosts` so a future editor finds the reasoning before "improving" it), the rewritten test file's own top-of-file comment, and here. Introducing a second raster vendor to cover this one degraded path would add a new, unevaluated host receiving visitor viewport coordinates — exactly the risk that test exists to prevent. The UAT human-check step 6 explicitly calls this out as known-and-accepted, not a bug to re-report.

- **Why `theme.js` was not modified.** The signal path is deliberately one-way through the DOM: the map modules watch `document.documentElement`'s `data-theme` attribute via `MutationObserver`; `theme.js` dispatches no event and calls no shared function, and never learns the map modules exist. `theme.js` runs unmodified on the login gate, the verify-outcome page and the Activity page, where no map exists, and `TestThemeModuleHasNoAppShellDependency` fails the build if its source ever names the app shell — observing the DOM sidesteps that contract entirely rather than threading a needle through it. Verified via `git diff web/static/js/theme.js` (empty) after both tasks, and via the new test's assertion that `theme.js`'s source contains neither `MutationObserver` nor `PinalertMap`.

- **The mode is never cached.** Both `currentBasemapMode()` implementations read `document.documentElement.getAttribute('data-theme')` fresh on every call — at construction and again inside every observer callback — because System mode is expressed by the attribute's *absence* rather than by a value. A mode captured once at construction time would go stale the instant a reader switched back to following the OS.

- **The contract-test change, framed as a deliberate severing.** The previous `TestBasemapBranchesPointAtCorrectHosts` required the vector construction's `style` option to be one exact string literal (`vectorStyleURL`, now retired). That form necessarily broke the moment the style became a runtime lookup — the whole point of Task 1. Task 2 replaces it with two checks that together preserve (and arguably strengthen) the same security property: (1) the construction call's `style` value must not contain `http`, proving it selects rather than inlines a URL; (2) each of the two reviewed URLs (`vectorStyleURLLight`, `vectorStyleURLDark`) must appear in each module's own source exactly once, via `strings.Count`, with the `https://tiles.` prefix appearing nowhere else in the file — closing the "third style quietly added in a branch" case the old single-literal equality check would have missed entirely if a duplicate or leftover URL had been introduced. This was a rewrite requested by the plan, not a relaxation invented to make a failing test pass.

- **The duplication this plan deliberately did not remove.** `BASEMAP_STYLES`, `currentBasemapMode()`, and the `MutationObserver` registration/callback now exist twice — once in `map.js`, once in `modal.js`, near-identically. This follows the codebase's stated convention (both files already carried a byte-identical `hasVectorBasemap()` probe with a comment recording the duplication as deliberate). If a future phase decides to break that convention with a shared client-side helper module, these three pieces are among the first candidates to revisit.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' acceptance criteria (grep counts on `tiles.openfreemap.org/styles/dark`, `.../liberty`, `attributeFilter`, `getMaplibreMap`, `matchMedia`; `git diff` emptiness on `theme.js`; `git status --porcelain` scoped to exactly the plan's `files_modified`) were verified explicitly after each task, before committing, and all matched the plan's specified values on the first attempt.

## Issues Encountered

None. The one intentionally RED test the plan called out (`TestBasemapBranchesPointAtCorrectHosts`, expected to fail after Task 1 and before Task 2) failed with exactly the message the plan predicted, and went green immediately after Task 2's rewrite — no unexpected red/green states anywhere else in the two-task sequence.

## User Setup Required

None — no external service configuration required. No database is involved anywhere in this plan.

## Next Phase Readiness

- Both map surfaces are theme-aware in code and gated by two automated tests; the remaining verification is the human-check block in this plan (steps 1–6), deferred to the phase's end-of-phase UAT round per `workflow.human_verify_mode: end-of-phase`, and shared coverage-noted as `human_judgment: true` above for that reason.
- No blockers for the other three plans in this gap-closure wave (`02-12`, `02-13`, `02-14`) — this plan shares zero files with them, confirmed by the final `git diff --stat` against the wave's shared base commit showing exactly `web/static/js/map.js`, `web/static/js/modal.js`, and `web/js_contract_test.go`.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Plan: 02-11*
*Completed: 2026-09-23*
