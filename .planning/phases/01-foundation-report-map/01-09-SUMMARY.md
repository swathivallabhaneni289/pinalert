---
phase: 01-foundation-report-map
plan: 09
subsystem: ui
tags: [css, layout, leaflet, viewport-units, regression-test]

# Dependency graph
requires:
  - phase: 01-foundation-report-map (plan 01-04)
    provides: the .app-shell/.pane grid layout and the #map container div this plan gives a height
  - phase: 01-foundation-report-map (plan 01-08)
    provides: the guarded .modal-backdrop fix that made this defect visible again (it was previously masked by a full-viewport black scrim on every load)
provides:
  - "#map height rule (100vh legacy fallback, 100dvh effective) in main.css's layout-containers section"
  - ".app-shell min-height paired into the same 100vh/100dvh viewport-unit family"
  - "TestPrimaryMapHasResolvedHeight durable regression test in web/css_contract_test.go"
affects: [01-foundation-report-map (remaining UAT re-run, starting from Test 1)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Two-declaration viewport-unit pairing (100vh fallback line, then 100dvh on the next line) used identically on both .app-shell's min-height and #map's height, so a box that resolves via the dynamic viewport unit never drifts out of sync with a sibling box still on the legacy unit"
    - "CSS contract test matching a selector's LAST whitespace-separated field against a target id, rather than substring search — lets a descendant-qualified selector still match while excluding an unrelated id that merely contains the target string as a substring (#modal-map vs #map)"

key-files:
  created: []
  modified:
    - web/static/css/main.css
    - web/css_contract_test.go

key-decisions:
  - "Declared #map's height directly in a viewport unit rather than a percentage of .pane, so resolution is self-contained and does not depend on .app-shell's CSS grid stretch behavior or on a stretched grid item's size counting as definite for a percentage child (both hold today, but the fix doesn't need either to remain true)"
  - "Paired .app-shell's min-height into the same 100vh/100dvh family as #map's new height, so the shell floor and the map's height agree on which viewport metric they mean (no phantom scroll strip, no gap under mobile browser chrome)"
  - "Confirmed via Step 0 (see Accomplishments) that no invalidateSize() gap exists, so the fix stayed CSS-only as the plan required: the template hard-codes data-view=\"map\", so .pane--map is never display:none at Leaflet init time in either layout mode, and feed.js's switchView() already dispatches a window resize event on returning to map view, which Leaflet's default trackResize listener uses to call invalidateSize() itself"

patterns-established: []

requirements-completed: []

coverage:
  - id: D1
    description: "The primary map container has a self-resolving viewport-unit height, so Leaflet's L.map(container) measures a non-zero viewport instead of the 0px box that caused UAT Test 1's blocker"
    requirement: "FOUND-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestPrimaryMapHasResolvedHeight"
        status: pass
      - kind: unit
        ref: "go test ./internal/api/handlers/ -run TestPageShellServesDOMContract"
        status: pass
      - kind: unit
        ref: "go test ./... (full suite)"
        status: pass
    human_judgment: true
    rationale: "Static CSS inspection (the new test) can prove a height-establishing rule exists on the right selector, but cannot prove the resulting computed height is non-zero in a real browser's layout. The plan's own verify block requires a DevTools computed-height check at both >=900px and <900px, a narrow-screen List<->Map toggle round-trip, a check that the modal's own map is unaffected, and a mobile-chrome clipping check — none of which this executor performed, per this plan's explicit scope note (\"Test 1 is not closed by this plan\") and the project's human_verify_mode: end-of-phase setting. Re-enter via /gsd-verify-work 01."

duration: 14min
completed: 2026-09-06
status: complete
---

# Phase 01 Plan 09: Primary Map Container Height Fix + Regression Test Summary

**Gave the primary Leaflet map container a self-resolving `100dvh`/`100vh` height in `main.css` (with `.app-shell`'s `min-height` paired into the same viewport-unit family) and locked it with a new `TestPrimaryMapHasResolvedHeight` Go regression test — closing the UAT Test 1 retest blocker where the map rendered as a solid black 0px box.**

## Performance

- **Duration:** ~14 min
- **Started:** 2026-09-06 (worktree base commit `ef33eef`)
- **Completed:** 2026-09-06
- **Tasks:** 1
- **Files modified:** 2 (`web/static/css/main.css`, `web/css_contract_test.go`)
- **Files created:** 0

## Accomplishments

- **Step 0 confirmed (no JS change needed).** Verified both facts the plan required before committing to a CSS-only fix: (1) `web/templates/index.html.tmpl` hard-codes `data-view="map"` on `#app-shell`, and `main.css`'s narrow-screen rules only hide `.pane--map` when `data-view="list"` — so at page load, in both layout modes, `.pane--map` is `display: block` when `map.js`'s `init()` runs; the map never initializes inside a hidden container. (2) `feed.js`'s `switchView()` already dispatches `window.dispatchEvent(new Event('resize'))` when returning to map view, with an existing comment explaining that Leaflet's default `trackResize` option listens for that event and calls the map's own `invalidateSize()` in response. Both held, so the fix stayed CSS-only as required.
- **Fixed the height defect.** Added an `#map` rule in `main.css`'s "Layout containers" section (placed after `.pane`, before `.view-toggle`, in DOM order) declaring `height: 100vh` (legacy fallback) followed by `height: 100dvh` (effective value in engines that support dynamic viewport units — chosen over the static unit because on mobile the static `100vh` is the *large* viewport, taller than the visible area while the address bar shows, which would push the map's bottom edge behind browser chrome). Paired the same two-declaration treatment onto `.app-shell`'s existing `min-height: 100vh`, adding a `min-height: 100dvh` line after it, so the shell floor and the map's height agree on the same viewport-unit family (avoids a phantom scroll strip / gap under the map). No `width`, `position`, `min-height`, or `z-index` was added to `#map` — the container is a block-level div that already fills its pane's inline size, the pane is already `position: relative`, and the FAB already sits above Leaflet's panes at `z-index: 1000`.
- **Added the regression test.** `TestPrimaryMapHasResolvedHeight` in `web/css_contract_test.go` walks every `*.css` file under `static/css` in the embedded `StaticFS`, strips comments, splits each rule on the last `{` (so a rule nested in a media query parses correctly), and matches a rule as targeting the primary map container only when one of its comma-separated selectors' *last whitespace-separated field* equals `#map` exactly — this lets a descendant-qualified selector still match while excluding `#modal-map` (a different element inside the report modal that this test must not police, since a plain substring match would false-positive on it). It then collects every `height`/`min-height` declaration on matching rules and asserts (A) at least one matching rule exists in `main.css`, and (B) at least one collected value carries a viewport-height unit (`vh`, which also covers `dvh`/`svh`/`lvh` as substrings) or an absolute length unit — not a bare percentage or keyword. Both failure messages explain the UAT Test 1 blocker and, for (B), are explicit that this is locking in *this fix's chosen strategy*, not an absolute rule of CSS: a percentage height is legitimate when every ancestor has a definite height, and a developer who deliberately builds that chain should update the test alongside it.
- **Confirmed the test actually bites.** Temporarily removed the new `#map` rule from `main.css`, re-ran `go test ./web/ -run TestPrimaryMapHasResolvedHeight -v`, watched it fail with the expected "no rule targeting the primary map container" message, then restored the file from a backup and verified the restored file was byte-identical (`diff` returned no output) before re-running the full pass.

## Task Commits

Each task was committed atomically:

1. **Task 1: Give the primary map container a self-resolving viewport height, and lock it with a CSS contract test** - `45ab1bb` (fix)

**Plan metadata:** committed alongside this SUMMARY (worktree mode — orchestrator finalizes STATE.md/ROADMAP.md after merge)

## Files Created/Modified

- `web/static/css/main.css` - Added `#map { height: 100vh; height: 100dvh; }` in the layout-containers section (with an explanatory comment naming the new test); paired `.app-shell`'s `min-height` with a `100dvh` line after its existing `100vh`
- `web/css_contract_test.go` - Added `TestPrimaryMapHasResolvedHeight`, reusing the existing `stripCSSComments` helper and the same last-`{`-split rule-parsing technique `TestModalBackdropHiddenGuard` already established

## Decisions Made

- Declared `#map`'s height directly in a viewport unit rather than a percentage of `.pane`, so the resolution is self-contained and does not depend on `.app-shell`'s CSS grid "stretch auto tracks" step running, or on a stretched grid item's size counting as definite for a percentage-sized child. Both hold today in current engines, but the fix does not need either to remain true — this is the same reasoning `#modal-map`'s own fixed `220px` height already embodies for its own, unrelated box.
- Matched the test's selector check on the *last field* of each comma-separated selector rather than a substring search, specifically to avoid a false positive on `#modal-map` while still allowing a descendant-qualified form like `.pane--map #map` to match.
- Did not touch `modal.css` (`#modal-map`'s working `220px` height is unrelated and asserted byte-unchanged) or any JavaScript/template file, per the plan's CSS-only constraint — confirmed via `git diff --quiet HEAD` gates on both.

## Deviations from Plan

None — plan executed exactly as written. Step 0's two confirmation facts both held as expected, so no JavaScript change was needed and no scope expansion occurred.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - this plan is a CSS-only fix plus one Go test; no data source or UI component was stubbed.

## Threat Flags

None - no new network endpoint, auth path, file access pattern, or schema change was introduced. This plan's own `<threat_model>` (T-01-27 through T-01-30) is fully covered by the CSS rule, the paired viewport-unit declarations, and the `git diff --quiet` scope gates already verified above.

## Next Phase Readiness

- The code-level fix and its regression test are both complete and verified: `go build ./...`, `go vet ./...`, `go test ./web/ -run 'TestPrimaryMapHasResolvedHeight|TestModalBackdropHiddenGuard'`, `go test ./internal/api/handlers/ -run TestPageShellServesDOMContract`, and the full `go test ./...` suite all pass. The viewport-unit-count gate (`100dvh`/`100vh` each appearing at least twice in `main.css`'s effective source) and both `git diff --quiet HEAD` scope gates (modal.css/feed.css untouched; no JS/template touched) also pass. The upstream-contract grep gate (primary map id, its Leaflet lookup, the modal map's own height, `invalidateSize` in `modal.js`, and the resize-dispatch pattern in `feed.js`) passes.
- **Not closed by this plan, by the plan's own explicit design.** The plan's `<verification>` section states plainly: "Still open after this plan — do not mark these covered. UAT Tests 1, 2, 3, 4, 5, 6, 7, 9 and 10 remain unresolved or [pending]." The Task 1 human-check — a DevTools computed-height reading at both >=900px and <900px, live-tile confirmation, a narrow-screen List→Map toggle round-trip, the modal's own map still rendering correctly, and a mobile-browser-chrome clipping check — was **not** performed by this executor. This is intentional, not an oversight: the plan frames itself as surgical code-level repair ("The root cause is fully confirmed; no further debugging is required"), the project's `workflow.human_verify_mode` is `end-of-phase`, and the plan's own text says Test 1 "must be re-run from scratch." Re-enter via `/gsd-verify-work 01` starting from Test 1 to close this and the eight tests blocked behind it.
- **REQUIREMENTS.md left untouched.** `.planning/REQUIREMENTS.md` already lists FOUND-03 and FOUND-04 as `[x]` / "Complete" from earlier in Phase 1's execution. This plan does not add, remove, or re-verify that status — it is exactly the claim UAT Test 1's outstanding human-check exists to substantiate, so this SUMMARY does not assert those requirements are newly validated by this plan alone.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*

## Self-Check: PASSED

- FOUND: web/static/css/main.css
- FOUND: web/css_contract_test.go
- FOUND: commit 45ab1bb (Task 1)
