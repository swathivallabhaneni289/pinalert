---
phase: 01-foundation-report-map
plan: 13
subsystem: ui
tags: [css, mask, glyph, dark-mode, accessibility, currentColor, contract-test]

# Dependency graph
requires:
  - phase: 01-foundation-report-map
    provides: the shipped map/modal/feed UI (plans 01-04 through 01-12), the nine committed category icon SVGs, and the 01-UAT.md record that identified Tests 2/6 as the one remaining major gap
provides:
  - A CSS-masked category-glyph system (`.icon-glyph` base rule + nine `.icon-glyph--{category}` rules) that paints glyph color from the ordinary CSS cascade instead of a sealed replaced-element sub-document
  - Three renderers (map.js, modal.js, feed.js) switched from `<img>` icon nodes to class-driven `<span>` glyphs
  - Two durable Go contract tests (`TestCategoryGlyphMaskRulesCoverEveryCategory`, `TestIconGlyphsAreClassDriven`) that fail the build on a missing/mistyped glyph rule or a reverted renderer
affects: [any future phase touching category icons, dark-mode theming, or the report modal/map/feed rendering]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CSS mask-based icon coloring: background-color: currentColor + mask-image (prefixed and unprefixed longhands, never the mask shorthand) instead of embedding icons as replaced-element <img> nodes, so glyph color resolves through the normal CSS cascade in every context/theme"

key-files:
  created: []
  modified:
    - web/static/css/main.css
    - web/static/css/feed.css
    - web/static/css/modal.css
    - web/static/js/app.js
    - web/static/js/map.js
    - web/static/js/modal.js
    - web/static/js/feed.js
    - web/css_contract_test.go
    - web/js_contract_test.go

key-decisions:
  - "Recolor glyphs with a CSS mask (currentColor background + mask-image alpha shape) rather than fetching/inlining SVG markup (would break the FOUND-04 no-markup-string invariant) or shipping light/dark icon variants (two variants cannot cover the real 3-context x 2-theme = 6 combination matrix)"
  - "Declared every mask longhand twice (prefixed and unprefixed) using longhands only, never the mask shorthand, because the prefixed and unprefixed shorthands reset different sub-property sets and mixing them is engine-dependent"
  - "feed.js's poll-driven row update swaps only the trailing category-specific class via replacePrefixedClass('icon-glyph--', ...), not the full class pair Pinalert.iconClass returns, so the base icon-glyph class set once in createRow is never re-pushed and cannot accumulate duplicates across refreshes"
  - "A fourth 'selected tile tinted by chosen severity' state described in 01-UI-SPEC.md line 129 was confirmed NOT built in the shipped DOM during planning; left out of scope here as a separate UI-SPEC conformance observation, not this defect"

patterns-established:
  - "Icon rendering: every category icon is now a <span class=\"icon-glyph icon-glyph--{category}\"> built via Pinalert.iconClass(category), never an <img> pointed at an icon file — any future icon addition should follow this same class-driven mask pattern"

requirements-completed: [FOUND-02, FOUND-03, FOUND-04]

coverage:
  - id: D1
    description: "CSS-masked glyph system: .icon-glyph base rule (currentColor background, prefixed+unprefixed mask longhands) plus nine .icon-glyph--{category} rules resolving to the nine committed icon files; all three icon-sizing selectors (main.css badge child, feed.css report-row, modal.css category-grid) extended to size the glyph element with byte-identical 55%/24px values, replacing (not appending to) the old image selector"
    requirement: "FOUND-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestCategoryGlyphMaskRulesCoverEveryCategory"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "map.js buildBadgeElement, modal.js buildCategoryGrid, and feed.js createRow/updateRow all build a class-driven <span> glyph via Pinalert.iconClass instead of a replaced-element <img>; app.js exports iconClass (replacing iconPath) carrying forward the T-01-17 CATEGORIES allowlist validation"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestIconGlyphsAreClassDriven"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./web/..."
        status: pass
    human_judgment: false
  - id: D3
    description: "Real-browser legibility of all nine category glyphs across all six context/theme combinations (category grid, selected tile, badge/feed pin, both light and dark mode) in Safari, plus the CR-01 55% sizing and no-tiling regression checks from the plan's human-check step"
    requirement: "FOUND-02"
    verification: []
    human_judgment: true
    rationale: "Static inspection (the two new Go contract tests) proves every glyph rule resolves to a real, correctly-named icon file and that every renderer is class-driven, but it cannot execute CSS mask compositing or prove a real WebKit engine paints a legible, correctly-sized, non-tiled glyph in each of the six combinations. This is exactly the gap the plan's own human-check step names as complementary, not redundant, to the automated tests — it was NOT performed during this execution and is recorded here as the outstanding verification item."

duration: ~25min
completed: 2026-09-09
status: complete
---

# Phase 01 Plan 13: CSS-Masked Category Glyphs Summary

**Recolored all nine category glyphs with a CSS mask (currentColor background + mask-image) instead of replaced-element `<img>` icons, fixing the dark-mode "solid black glyph" defect UAT Tests 2/6 reported, and locked the fix with two Go contract tests.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-09-09
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- Added a CSS-masked glyph system to `main.css` (`.icon-glyph` base rule + nine `.icon-glyph--{category}` rules) and extended all three icon-sizing selectors (`main.css`, `feed.css`, `modal.css`) to size the new glyph element, replacing the old image selectors outright
- Switched `map.js`, `modal.js`, and `feed.js` from building replaced-element `<img>` icon nodes to building class-driven `<span>` glyphs via a new `Pinalert.iconClass` helper (replacing `iconPath`), carrying forward the T-01-17 category allowlist validation byte-for-byte
- Added `TestCategoryGlyphMaskRulesCoverEveryCategory` (Go, `web/css_contract_test.go`) and `TestIconGlyphsAreClassDriven` (Go, `web/js_contract_test.go`), both confirmed during this execution to fail on a mistyped mask path, a deleted glyph rule, and a reverted renderer, and to pass again once restored

## Task Commits

Each task was committed atomically:

1. **Task 1: Add the CSS-masked glyph system and extend all three icon-sizing selectors** - `aa37d25` (feat)
2. **Task 2: Switch all three renderers from image nodes to class-driven glyph spans** - `7c70752` (feat)
3. **Task 3: Lock the fix with two durable Go contract tests** - `2655a0c` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `web/static/css/main.css` - Added `.icon-glyph` base rule (currentColor background, prefixed+unprefixed mask longhands) and nine `.icon-glyph--{category}` rules; rewrote the icon-badge section's stale comment; extended the badge-child sizing selector to the glyph element in place of the image selector
- `web/static/css/feed.css` - Extended the report-row badge sizing rule to the glyph element; rewrote the explanatory comment to describe the shared main.css sizing rule instead of the removed `<img>` intrinsic-size problem
- `web/static/css/modal.css` - Extended the category-grid tile sizing rule to the glyph element in place of the image selector
- `web/static/js/app.js` - Replaced `iconPath` with `iconClass`, an allowlist-validated category-to-class-name helper exported on the public `Pinalert` API object
- `web/static/js/map.js` - `buildBadgeElement` now builds an `aria-hidden` glyph `<span>` via `Pinalert.iconClass` instead of an `<img>`; updated the file-level security comment
- `web/static/js/modal.js` - `buildCategoryGrid` now builds an `aria-hidden` glyph `<span>` via `Pinalert.iconClass` instead of an `<img>`
- `web/static/js/feed.js` - `createRow` builds a glyph `<span>` (returned as `row.glyph`, replacing `row.img`); `updateRow` swaps only the trailing category-specific class via `replacePrefixedClass('icon-glyph--', ...)` to stay idempotent across polls
- `web/css_contract_test.go` - Added `TestCategoryGlyphMaskRulesCoverEveryCategory` plus its parsing helpers (`extractCategoryList`, `takeIdentRun`, `extractMaskURLPath`, `collectGlyphMaskRules`, `assertBaseGlyphRule`, `assertGlyphSizingAndNoOrphanedImageSelectors`)
- `web/js_contract_test.go` - Added `TestIconGlyphsAreClassDriven`

## Decisions Made
- CSS mask recoloring chosen over inline-SVG injection (would violate the verified FOUND-04 no-markup-string invariant) and over light/dark icon variant swapping (two variants cannot cover the real 3-context x 2-theme = 6-combination matrix); see `key-decisions` in frontmatter for the full rationale carried over from planning
- feed.js's `updateRow` passes only the trailing category-specific class field to `replacePrefixedClass`, not the full `Pinalert.iconClass` return value, to stay idempotent across the 30s poll (verified by hand: base class survives, category class swaps cleanly on repeated calls)
- No change to `01-UI-SPEC.md` line 129's "selected tile tinted by severity" gap — confirmed during planning as not built in the shipped DOM and explicitly out of scope for this closure; the chosen fix is forward-compatible with it since glyph color already flows from the ordinary `currentColor` cascade

## Deviations from Plan

None - plan executed exactly as written. All three tasks' automated `<verify>` commands passed on the first attempt with no auto-fixes required.

## Issues Encountered

None during automated execution. See "Known Stubs / Outstanding Verification" below for the one item this plan's own `<human-check>` step defers to a human.

## Known Stubs / Outstanding Verification

This is not a stub in the shipped code — every renderer and every CSS rule is fully wired, and `go test ./web/...` is green in full. The one outstanding item is the plan's own Task 3 `<human-check>`:

- **Not yet performed:** starting the server and visually confirming all six context/theme glyph-legibility combinations (category grid, selected tile, badge/feed pin — each in both light and dark mode) in Safari, per Task 3's nine-step human-check procedure in `01-13-PLAN.md`. Static inspection (the two new Go contract tests) proves every glyph rule resolves to a real, correctly-named icon file and every renderer is class-driven, but cannot prove a real WebKit engine paints a legible, correctly-sized, non-tiled glyph — this is exactly the gap the plan names as complementary to, not redundant with, the automated tests. Recorded as coverage item D3 (`human_judgment: true`) in this SUMMARY's frontmatter for `/gsd-verify-work` to route to a human.
- **Known residual risk** (not covered by any static test): `mask-size: contain` is applied against icon files that declare `width="100%" height="100%"` with `viewBox="0 0 24 24"` — no intrinsic pixel size, the same property `feed.css` used to document as a problem for the old `<img>` path. `contain` should resolve correctly via the SVG's viewBox aspect ratio, but if the human check above reports a blank or mis-scaled glyph, the fallback is `mask-size: 100% 100%` on the `.icon-glyph` base rule.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The UAT gap tagged `test: [2, 6]` is closed at the code level: all three renderers and all three sizing stylesheets ship the CSS-masked glyph system, and two Go contract tests guard against regression.
- Before this gap can be marked fully resolved in `01-UAT.md`, the Task 3 human-check (six real context/theme combinations in Safari) still needs to run — see "Known Stubs / Outstanding Verification" above.
- No blockers for subsequent phases; the `Pinalert.iconClass` pattern is now the established convention for any future icon addition.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-09*

## Self-Check: PASSED

All 9 modified/created source files confirmed present on disk. All 3 task commit hashes (`aa37d25`, `7c70752`, `2655a0c`) confirmed present in `git log --oneline --all`.
