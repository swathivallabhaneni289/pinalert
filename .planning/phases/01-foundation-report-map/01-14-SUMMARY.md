---
phase: 01-foundation-report-map
plan: 14
subsystem: ui
tags: [css, svg, mask, glyph, contrast, wcag, contract-test]

# Dependency graph
requires:
  - phase: 01-foundation-report-map
    provides: plan 01-13's CSS-masked category-glyph system (`.icon-glyph` base rule, nine `.icon-glyph--{category}` rules, class-driven renderers) and the `.planning/debug/category-glyph-legibility.md` diagnosis this plan's two mechanisms are traced to
provides:
  - Nine category icon SVGs re-authored with a heavier, uniform stroke-width (2 -> 3, identical across all nine, still ISC-licensed lucide-static line art on the 24-unit viewBox)
  - An age-aware badge glyph foreground (`--badge-glyph-fg` custom property, inherited from `.age-aging`/`.age-stale`) so a desaturated report's glyph stays readable against its own faded badge fill
  - A darkened light-mode `--color-severity-medium` hex (#C98A1A -> #C48419) clearing the one WCAG-failing fresh-severity pairing the re-derived 18-pairing matrix found
  - Two computed Go contract gates (`TestCategoryGlyphInkCoverageAcrossRenderContexts`, `TestBadgeGlyphContrastAcrossAgeStagesAndThemes`), each proven during this execution to fail on the defect it guards and pass once fixed
  - 01-REVIEW.md WR-01 closed: the dead `.icon-badge svg` selector removed, and the orphan-selector guard widened to catch that selector shape (it did not before this plan)
affects: [any future phase touching category icons, badge/age-ramp theming, or the report modal/map/feed rendering]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Age-aware badge foreground via inherited custom property: .icon-badge's color reads var(--override, var(--token)); the override is declared inside the SAME .age-* rules that already write --severity-current, so it reaches both DOM shapes the age class can land in (badge element itself for map pins, row ancestor for feed rows) without a descendant or compound selector"
    - "Contract-test resolvers that require an EXACT selector-head match (never a substring search) when the resolver's correctness depends on which of several DOM shapes a fix actually took — a substring match would let a wrongly-shaped fix compute a passing ratio it does not deserve"

key-files:
  created: []
  modified:
    - web/static/icons/flood.svg
    - web/static/icons/earthquake.svg
    - web/static/icons/fire.svg
    - web/static/icons/storm_cyclone.svg
    - web/static/icons/road_blocked.svg
    - web/static/icons/power_outage.svg
    - web/static/icons/shelter_open.svg
    - web/static/icons/rescue_needed.svg
    - web/static/icons/other.svg
    - web/static/css/main.css
    - web/css_contract_test.go
    - .planning/phases/01-foundation-report-map/01-UI-SPEC.md

key-decisions:
  - "Chose stroke-width 3.0 (uniform across all nine icons) — inside the plan's derived [2.5, 3.5] bound, roughly midway, giving a comfortable geometric margin against viewBox clipping (outer edge at 23.5 of 24 units) while still materially raising ink coverage in all three render contexts"
  - "For the one WCAG failure the worked option's ramp-only fix left open (light-mode fresh-medium, 2.94:1), took the 'darken the light hex' route over the 'per-theme severity override' route — keeps the override mechanism to exactly the .age-* shape the gate's resolver reads, at the cost of two hex edits (both light theme sources) plus one 01-UI-SPEC.md table+note edit"
  - "Split the single logical diff into two atomic task commits (icons+WR-01 first, then contrast fix) by temporarily reverting Task 2's main.css/UI-SPEC portions, verifying Task 1 in isolation, committing, then restoring and verifying Task 2 — preserves the plan's 'each task independently shippable' property in the actual commit history, not just in prose"

patterns-established:
  - "Badge foreground age-adaptation: any future age-ramp-driven visual property on .icon-badge should follow --badge-glyph-fg's pattern (declared in .age-* rules, consumed via var(--override, var(--token)) on the base rule), not a descendant/compound selector"

requirements-completed: [FOUND-02, FOUND-03, FOUND-04]

coverage:
  - id: D1
    description: "Nine category icons re-authored with uniform stroke-width 3 (was 2), raising physical glyph ink in all three render contexts (pin 3.025px, tile 3.000px, feed row 2.200px — all above the pre-fix 2.017/2.000/1.467 baseline), plus 01-REVIEW.md WR-01's dead .icon-badge svg selector removed and the orphan-selector guard widened to catch that shape"
    requirement: "FOUND-02, FOUND-03, FOUND-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestCategoryGlyphInkCoverageAcrossRenderContexts"
        status: pass
      - kind: unit
        ref: "web/css_contract_test.go#TestCategoryGlyphMaskRulesCoverEveryCategory"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./web/..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Badge glyph foreground now adapts per age stage via an inherited --badge-glyph-fg custom property (declared in .age-aging/.age-stale, consumed by .icon-badge's color), plus a darkened light-mode --color-severity-medium (#C98A1A -> #C48419); all 18 severity x age-stage x theme pairings clear WCAG 1.4.11's 3.0:1 floor"
    requirement: "FOUND-03, FOUND-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestBadgeGlyphContrastAcrossAgeStagesAndThemes"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "Both new gates proven to actually discriminate: five temporary edits (pre-fix stroke-width restored, a sizing literal shrunk, the stale override removed, the override re-expressed as a wrong-shape compound selector, WR-01's dead selector re-added), each confirmed to fail the relevant gate with the predicted message/ratio, then reverted; full suite confirmed clean afterward including 01-13's own TestIconGlyphsAreClassDriven and TestCategoryGlyphMaskRulesCoverEveryCategory, both still passing untouched"
    verification:
      - kind: other
        ref: "manual discrimination-check sequence (documented below) + go build ./... && go vet ./... && go test ./... -v -run 'TestCategoryGlyphInkCoverageAcrossRenderContexts|TestBadgeGlyphContrastAcrossAgeStagesAndThemes|TestCategoryGlyphMaskRulesCoverEveryCategory|TestIconGlyphsAreClassDriven'"
        status: pass
    human_judgment: false
  - id: D4
    description: "Real-browser confirmation that a person can NAME each category glyph at the 17.6px feed-row size and on map pins, all nine modal grid tiles nameable, the three merge-prone shapes (power_outage slash, earthquake zigzag, rescue_needed ring) still read as distinct under the heavier stroke, the stale-aged badge glyph is readable in both themes, and the age ramp still reads as fading rather than louder — Task 3's human-check procedure in 01-14-PLAN.md, both themes, both map pin and feed row and modal grid contexts"
    verification: []
    human_judgment: true
    rationale: "Every automated gate in this plan (ink coverage, WCAG point-contrast) proves a NECESSARY condition — more ink, sufficient point-contrast — but neither proves a human can actually distinguish one glyph's SHAPE from another's at a glance, which is the plan's actual goal and the literal complaint that triggered it ('I can't see which one it is'). This was NOT performed during this automated execution (no browser access in this worktree-isolated agent) and is recorded here, per config.json's human_verify_mode: end-of-phase, for routing to a human at phase-end verification. This item explicitly SUPERSEDES and SUBSUMES 01-13-SUMMARY.md's outstanding coverage item D3 (six context/theme combinations never confirmed in a real browser) — D4 here covers those six plus the age-stage readability check plus the naming test, so D3 closes as absorbed into this item rather than lingering as a separate ghost item."

duration: ~50min
completed: 2026-09-09
status: complete
---

# Phase 01 Plan 14: Category Glyph Legibility — Ink Coverage + Age-Aware Contrast Summary

**Raised all nine category icons' stroke-width 2->3 (uniform, within the plan's derived [2.5, 3.5] bound) and made `.icon-badge`'s glyph foreground age-aware via an inherited `--badge-glyph-fg` custom property, clearing every one of 18 severity x age-stage x theme WCAG 1.4.11 pairings — gated by two new computed Go contract tests, each proven to actually fail on the defect it guards.**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-09-09
- **Tasks:** 3 (Task 3 produced no permanent file changes — see below)
- **Files modified:** 12

## Accomplishments
- Closed Mechanism 1 (stroke-only source artwork capping ink at a 1.47-2.02px hairline in all three render contexts, both themes): raised all nine icon files' `stroke-width` from 2 to 3, identically, keeping the ISC license comment, `fill="none"`, the 24-unit viewBox, round caps/joins, and `currentColor`. Post-fix physical stroke widths: pin badge 3.025px (was 2.017), modal tile 3.000px (was 2.000), feed row 2.200px (was 1.467) — strictly greater in all three contexts.
- Folded in 01-REVIEW.md's WR-01: removed `main.css`'s dead `.icon-badge svg` selector (the badge-child sizing rule now lists only the glyph selector), and widened `assertGlyphSizingAndNoOrphanedImageSelectors`'s trailing-field check from `"img"` only to `"img"` or `"svg"`, so that exact dead-selector shape is now caught by the guard that documented it but never enforced it.
- Closed Mechanism 2 (badge glyph foreground never adapting to the age-ramp's fill rewrite, a genuine WCAG 1.4.11 failure as low as 1.60:1 light / 2.36:1 dark once stale): `.icon-badge`'s `color` is now `var(--badge-glyph-fg, var(--color-bg))`; `.age-aging` and `.age-stale` each declare `--badge-glyph-fg: var(--color-text)`, inherited through the exact same mechanism that already carries `--severity-current` — so it reaches both DOM shapes the age class can land in (badge element itself for map pins, row ancestor for feed rows) without a descendant or compound selector.
- Extended the fix's scope from the diagnosed stale-only failure to the full re-derived 18-pairing matrix (severity x age-stage x theme), which found light-mode aging also failed (down to 2.19:1). Fixed the one pairing the ramp-only fix left open — light-mode fresh-medium at 2.94:1 — by darkening light `--color-severity-medium` from `#C98A1A` to `#C48419` (both light theme sources; dark value untouched), clearing it to 3.15:1. `01-UI-SPEC.md`'s color table and a new explanatory note were updated in the same commit.
- Added `TestCategoryGlyphInkCoverageAcrossRenderContexts`: parses every icon's stroke-width/viewBox, asserts identical stroke-width within bound, resolves the feed-row and modal-tile render boxes from literals only (never a cascade resolver), guards the map-pin domination argument with two cheap literal checks instead of resolving `var(--touch-target-min)`, and asserts computed physical stroke width clears the floor in both asserted contexts.
- Added `TestBadgeGlyphContrastAcrossAgeStagesAndThemes`: resolves all four theme token sources (asserting pairwise equality within each theme, closing the "half-applied theme edit" trap), resolves badge fill from the shipped `.sev-*`/`.age-*` rules (supporting passthrough and `color-mix()` forms), resolves glyph foreground via an EXACT selector-head match on each `.age-{stage}` rule (never a substring search — this is what makes a wrongly-shaped fix self-detecting), and asserts all 18 pairings clear 3.0:1.
- Proved both new gates discriminate: five temporary edits (documented below), each confirmed to fail the relevant gate with the predicted message or ratio, then reverted. Confirmed the full suite is clean afterward, including 01-13's own `TestIconGlyphsAreClassDriven` and `TestCategoryGlyphMaskRulesCoverEveryCategory`, both still green and untouched.

## Task Commits

Each independently-shippable task was committed atomically. Task 3 produced no permanent code changes (all five discrimination edits were temporary and reverted before the working tree was left clean), so it has no commit of its own — its results are documented here and in the Deviations/Discrimination sections below.

1. **Task 1: Raise glyph ink coverage in all three render contexts, gate it on computed stroke width, and clear WR-01** - `42a0a5e` (feat)
2. **Task 2: Give every badge glyph a 3:1 foreground across all eighteen severity x age x theme pairings, gated on computed WCAG contrast** - `464799f` (feat)
3. **Task 3: Prove both gates discriminate, then confirm a human can name each category at the smallest size shipped** - no commit (verification-only; see below)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `web/static/icons/{flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other}.svg` - `stroke-width="2"` -> `stroke-width="3"`, single-attribute change each, everything else byte-identical
- `web/static/css/main.css` (Task 1 commit) - Removed the dead `.icon-badge svg,` selector half of the badge-child sizing rule (WR-01)
- `web/static/css/main.css` (Task 2 commit) - `.icon-badge`'s `color` changed to `var(--badge-glyph-fg, var(--color-bg))`; `.age-aging`/`.age-stale` each declare `--badge-glyph-fg: var(--color-text)`; `.age-fresh` deliberately left bare (falls through to the original foreground); light `--color-severity-medium` darkened `#C98A1A` -> `#C48419` in both `:root` and `:root[data-theme="light"]`; icon-badge and age-ramp comment blocks rewritten to document the new mechanism and its rationale
- `web/css_contract_test.go` - Widened `assertGlyphSizingAndNoOrphanedImageSelectors` (WR-01 guard) plus its doc comment; added `TestCategoryGlyphInkCoverageAcrossRenderContexts` and its parsing helpers (`extractAttr`, `findDeclValue`, `parsePx`, `parsePercent`); added `TestBadgeGlyphContrastAcrossAgeStagesAndThemes` and its resolver helpers (`bareVarName`, `parseColorVarShape`, `splitColorStop`, `parseColorMixStops`, `parseHexRGB`, `mixHexSRGB`, `resolveColorRefHex`, `resolveSeverityCurrentHex`, `resolveForegroundValueHex`, `srgbChannelToLinear`, `relativeLuminance`, `wcagContrastRatio`, `isDarkMediaRootHead`, `extractThemeTokens`, `cssRule`, `parseCSSRules`, `declsOf`, `ruleBySelector`)
- `.planning/phases/01-foundation-report-map/01-UI-SPEC.md` - Severity palette table's light-mode medium dot value updated to `#C48419`; added a one-paragraph note explaining the sub-perceptual darkening and the ratio it clears (2.94:1 -> 3.15:1)

## Decisions Made
- Stroke-width chosen as 3.0 (uniform across all nine icons) — see `key-decisions` in frontmatter for the geometric-margin rationale (outer edge lands at 23.5 of 24 units, well inside the 3.5 ceiling's 23.75).
- For light-mode fresh-medium's one remaining WCAG failure (2.94:1), took the hex-darkening route over a per-theme severity foreground override — see `key-decisions` for why (keeps the gate's resolver to exactly the shape it already reads).
- Split what was written as one logical set of edits into two atomic task commits by temporarily reverting Task 2's `main.css`/`01-UI-SPEC.md` portions (and the corresponding test imports), verifying Task 1 fully in isolation, committing, then restoring the full state and verifying Task 2 — see `key-decisions`.
- Did not rename `assertGlyphSizingAndNoOrphanedImageSelectors` despite it now also catching a bare-vector-element (`svg`) selector, not just `img` — the function is referenced only within this file, and the plan's own instruction was to update its doc comment to match its widened implementation, not to rename it.

## Deviations from Plan

None — plan executed exactly as written. The two implementation choices above (stroke-width value, hex-darken route) were explicitly left to Claude's discretion within the plan's stated bounds, not deviations from it.

## Discrimination Checks (Task 3)

All five ran as temporary edits, each confirmed to fail as predicted, each reverted immediately (`git status --short` and `git diff --stat` confirmed clean before proceeding to the next):

1. **Restored `flood.svg`'s stroke-width to 2** (pre-fix value). `TestCategoryGlyphInkCoverageAcrossRenderContexts` failed, naming `flood.svg` on the uniformity check, both physical-width floor checks, and logging its regressed 2.017/2.000/1.467 numbers. Reverted.
2. **Shrunk the feed-row glyph sizing literal** (`.report-row .icon-badge .icon-glyph` from 55% to 40% in `feed.css`). The same test failed on its monotonicity guard ("feed row glyph width 40% is below the current 55% baseline"). Reverted.
3. **Removed `.age-stale`'s `--badge-glyph-fg` override.** `TestBadgeGlyphContrastAcrossAgeStagesAndThemes` failed on all three stale pairings, computing exactly 1.60:1 (light) and 2.36:1 (dark) — the two numbers the diagnosis independently computed by hand, which is also a cross-check that the gate's own arithmetic is correct and not merely self-consistent. Reverted.
4. **Re-expressed the stale override as a wrong-shape compound selector** (`.icon-badge.age-stale { --badge-glyph-fg: ... }` instead of declaring it inside the bare `.age-stale` rule). The gate's exact-selector-head resolver found nothing in the bare `.age-stale` rule, fell through to the base token, and failed with the same 1.60:1/2.36:1 ratios as check 3 — proving the resolver cannot be fooled by a fix that would silently cover only the map pin (badge carries both classes) and miss the feed row (age class on the row ancestor). Reverted.
5. **Re-added `main.css`'s dead `.icon-badge svg,` selector.** The widened orphan-selector guard inside `TestCategoryGlyphMaskRulesCoverEveryCategory` failed, naming `.icon-badge svg` explicitly — confirming it did not catch this shape before Task 1's widening. Reverted.

After all five reverts, `go build ./...`, `go vet ./...`, and the full `go test ./...` ran clean, including `TestIconGlyphsAreClassDriven` and `TestCategoryGlyphMaskRulesCoverEveryCategory` (01-13's own gates, unmoved).

## Issues Encountered

None during automated execution. See "Known Stubs / Outstanding Verification" below for the one item this plan's own Task 3 `<human-check>` defers to a human — this is expected per `config.json`'s `human_verify_mode: "end-of-phase"`, not a gap in this execution.

## Known Stubs / Outstanding Verification

Not a stub in the shipped code — every icon file, CSS rule, and Go gate is fully wired, and `go test ./...` is green in full, including both new gates and both of 01-13's untouched gates. The one outstanding item is Task 3's own `<human-check>` step:

- **Not yet performed:** starting the server and, in Safari on the Mac (the browser the original UAT report came from), confirming a person can actually NAME each category glyph at the 17.6px feed-row size and on map pins, that all nine modal grid tiles are present and nameable, that the three stroke-merge-prone shapes (`power_outage`'s slash-vs-bolt, `earthquake`'s zigzag vertices, `rescue_needed`'s inner ring) still read as distinct under the heavier stroke rather than merging into a blob, that no glyph clips the 24-unit viewBox, that the stale-aged badge glyph is readable against its own faded fill in both themes, and that the age ramp still reads as fading rather than louder than fresh — per Task 3's twelve-item human-check procedure in `01-14-PLAN.md`. This is exactly the check the automated gates in this plan cannot perform: ink coverage and point-contrast are both NECESSARY conditions for a legible glyph, but neither proves a human can tell two SHAPES apart, which is the plan's actual goal and the literal complaint ("I can't see which one it is") that triggered it.
- **This is a worktree-isolated parallel executor with no browser access**, and `config.json` sets `human_verify_mode: "end-of-phase"` — this human check is intentionally deferred to phase-end verification (`/gsd-verify-work`), consistent with how 01-13's own equivalent human-check (its outstanding coverage item D3) was handled. Recorded as coverage item D4 (`human_judgment: true`) in this SUMMARY's frontmatter.
- **D4 explicitly supersedes and subsumes 01-13-SUMMARY.md's outstanding D3** (six context/theme combinations never confirmed in a real browser): D4's twelve-item check covers those six plus the age-stage readability check plus the naming test 01-13's D3 never asked. D3 closes as absorbed into D4, not as a separate lingering ghost item.
- **Per this plan's own `<success_criteria>`, this means the plan's success criteria are NOT fully closed by this automated execution alone.** Every computed gate passes and every discrimination check confirms the gates are honest, but the plan's literal goal — "a person glancing at a feed row at 17.6px can say which category it is without hovering" — is stated as a human-confirmed fact, and that confirmation has not yet happened. State this plainly rather than self-grading it as done: Task 3's `<done>` bullet "The human check confirms every category is NAMEABLE..." is outstanding, not met.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The UAT gaps tagged `test: [2, 6]` in `01-UAT.md` are closed at the code and gate level: both diagnosed mechanisms have shipped fixes and computed gates, and both gates were proven during this execution to actually discriminate (fail on the defect, pass once fixed).
- Before either gap can be marked fully resolved in `01-UAT.md`, Task 3's human-check (the naming test at 17.6px, plus the age-stage/shape-integrity checks) still needs to run in a real browser — see "Known Stubs / Outstanding Verification" above. This is the single remaining blocker on this plan's own success criteria, not on any other phase's work.
- No blockers for subsequent phases; the `--badge-glyph-fg` inherited-custom-property pattern is now the established convention for any future age-ramp-driven visual property on `.icon-badge`.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-09*
