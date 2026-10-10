---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 22
subsystem: ui
tags: [trust-block, sketch-003, variant-h, chevron, hover-fill, forced-colors, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: amendments A75 to A84 from plan 03-21
provides:
  - No underline in any state on the trust toggle, on the feed row, the map popup and the Activity row (UAT Gap 1)
  - A wrapping flex toggle with a face wrapper (flex 1 1 6.85em) and an aria-hidden chevron that turns on open
  - A soft hover fill, an unchanged press fill and inset ring, and a border cue under forced colours
affects: [03-23, 03-UAT Gap 1]
tech-stack:
  added: []
  patterns:
    - "Fixed class literals for new nodes in createTrustBlock, no payload value reaches a class"
    - "Forced colours replace fills with a Canvas rest border and a CanvasText hover and press border"
key-files:
  created:
    - web/trust_face_contract_test.go
    - web/static/icons/chevron-down.svg
  modified:
    - web/trust_contract_test.go
    - web/css_contract_test.go
    - web/static/js/trust.js
    - web/static/css/trust.css
key-decisions:
  - "The hover fill is not wrapped in a hover media query (A77), same as the vote buttons"
  - "UAT Gap 3 (text too long) stays open and deferred (A84); no wording changed"
requirements-completed: []
status: complete
---

# Phase 3 Plan 22: Trust toggle face Summary

The trust toggle is now a wrapping flex row holding a face (counts and signals) and an aria-hidden chevron, with a hover fill instead of the underline, so UAT Gap 1 is closed in code on all three surfaces at once.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 (RED) | ce9c3c8 | test(03-22): pin the trust toggle face, chevron, fills and underline ban |
| 2 (GREEN) | a6caee1 | feat(03-22): trust toggle face with a chevron and a hover fill, no underline |

Each commit lists only its planned paths (checked with `git show --stat --format= HEAD`).

## RED failure lines

The four listed tests failed for the stated reasons before any production change:

- TestPhase3TrustGlyphMasksReferenceEveryExemptIcon: `static/icons/chevron-down.svg: listed in phase3TrustGlyphIcons but missing from the embedded tree`.
- TestTrustCSS: toggle `display = "block", want "flex"` plus the missing flex, gap, width, margin-inline and padding-inline keys, and `.trust-block__toggle:hover background-color = "", want "color-mix(...)"`.
- TestTrustToggleFaceMarkup: `missing "'trust-block__face'"`, `missing "'trust-block__chevron'"`, `"face.appendChild(counts)" is missing`, and the two `toggle.appendChild(counts|signals)` forbids.
- TestTrustToggleFaceStyle: `no rule with head ".trust-block__face:not([hidden])" in media ""`.

The package compiled and vetted in the RED state.

## Green run

- `go vet ./web/`: pass.
- The nine named tests (the four above plus TestCategoryGlyphMaskRulesCoverEveryCategory, TestTrustBlockMountedOnBothSurfaces, TestTrustDisclosure, TestTrustModuleHasNoThresholdsOrSinks, TestVoteStyleSurvivesForcedColors): ok.
- `go test ./web/ -count=1`: ok.
- `gofmt -l web`: prints nothing.
- No added line in any file of the plan holds an em or en dash.
- No database, server, browser or port was used. `go test ./...` was not run.

## What changed

- **trust.js, createTrustBlock only.** A `face` span (`trust-block__face`) now holds `counts` and `signals`, a `chevron` span (`trust-block__chevron`, `aria-hidden="true"`, no text) follows it, and the hint stays last. The reporter signal stays inside `signals` (plan 03-23 moves it). Neither node is added to `instance`; no function, tabindex, title or live region was added.
- **chevron-down.svg.** A copy of clock.svg with class `lucide lucide-chevron-down` and the single path `m6 9 6 6 6-6`. First line equals circle-alert.svg's licence line.
- **trust.css.** The toggle is `display: flex` with wrap, 4px wider on each side, no underline offset. Hover declares only the half strength border colour fill. The face rule (6.85em basis), the chevron mask rule, the open turn (180deg), the reduced motion rule for the chevron and the forced colours rules (Canvas rest border, CanvasText hover and press border, CanvasText chevron) follow in the order the plan gave. The press rule and the inset focus ring are unchanged.

## Test changes

No existing assertion was weakened. The plan names these edits and they are the only ones:

| Test | Old | New |
| ---- | --- | --- |
| TestTrustCSS, row `.trust-block__toggle:not([hidden])` | display `block`, width `100%`, `text-underline-offset` `4px`, padding `var(--space-sm) var(--space-xs)`, plus the unchanged min-height, radius, background, colour, font, text-align and transition | display `flex`, flex-wrap `wrap`, justify-content `flex-end`, align-items `center`, align-content `center`, column-gap and row-gap `var(--space-xs)`, width `calc(100% + var(--space-sm))`, margin-inline `calc(-1 * var(--space-xs))`, padding-inline `var(--space-sm) var(--space-xs)`; the underline offset key is removed (A77); other values unchanged |
| TestTrustCSS, row `.trust-block__toggle:hover` | `text-decoration` `underline` | `background-color` `color-mix(in srgb, var(--color-border) 50%, transparent)` |
| css_contract_test.go `nonCategoryIcons` | no chevron entry | gains `static/icons/chevron-down.svg` |
| css_contract_test.go `phase3TrustGlyphIcons` | six names ending in `x.svg` | seven names ending in `chevron-down.svg`; the comment above it names plan 03-22 |

Test code of TestPhase3TrustGlyphMasksReferenceEveryExemptIcon and TestCategoryGlyphMaskRulesCoverEveryCategory is unchanged. New: `web/trust_face_contract_test.go` with TestTrustToggleFaceMarkup and TestTrustToggleFaceStyle.

## Deviations from Plan

None. The plan was executed as written. One note: gofmt realigned the map values in the edited TestTrustCSS row (no content change). Every anchor (`createTrustBlock(options)`, the three `toggle.appendChild` lines, the toggle and media heads in trust.css, `nonCategoryIcons`, `phase3TrustGlyphIcons`) was found where the plan said.

## UAT Gap 3

Stays open and deferred (A84). No sentence, no `internal/` file and no wording test was touched.

## Live proof still owed (orchestrator, after merge)

No underline in any state on the three surfaces, hover and press fills that differ, the forced colours border cue and a visible chevron in Chromium, and no avoidable count split at 200 percent text. This plan proves only the shipped text and the declared CSS.

## Known Stubs

None.

## Threat Flags

None. T-03-06, T-03-22-01 and T-03-22-02 are mitigated as planned: fixed class literals, a copied one path SVG with the mask test, and an aria-hidden empty chevron with no tabindex.

## Self-Check: PASSED

- web/trust_face_contract_test.go and web/static/icons/chevron-down.svg exist.
- Commits ce9c3c8 and a6caee1 exist on the worktree branch.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched.
