---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 11
subsystem: ui
tags: [css, vote-controls, forced-colors, reduced-motion, cascade-contract-tests, svg-masks]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-09 vote button DOM: icon span with vote-btn__icon plus modifier, label span, named confirmation, vote-note node"
provides:
  - "One outlined vote button style on the feed row, popup and Activity page, in a two track grid"
  - "Four Lucide glyph files masked by literal url rules and exempt from the category walk"
  - "Cascade resolver test helpers (p11ParseRules, p11Resolve and friends) that prove every state combination from the shipped stylesheet"
  - "Pressed, in flight, too far, disabled Mark resolved and destructive Yes looks; error glyph; note slot; forced colours; reduced motion"
affects: [03-13, 03-16, 03-17, 03-18, 03-20]
tech-stack:
  added: []
  patterns:
    - "Media aware CSS parser plus specificity and source order resolver in a Go contract test"
key-files:
  created:
    - web/static/icons/check.svg
    - web/static/icons/ban.svg
    - web/static/icons/shield-check.svg
    - web/static/icons/rotate-ccw.svg
    - web/vote_style_contract_test.go
  modified:
    - web/static/css/trust.css
    - web/css_contract_test.go
    - web/votes_contract_test.go
key-decisions:
  - "Yes uses min-width 144px (the outline literal) where the UI-SPEC says about 140px."
  - "The pressed too far rule is declared after the pressed in flight rule so aria-disabled decides the cursor when both are set."
requirements-completed: [TRUST-05]
status: complete
duration: 55min
completed: 2026-10-08
---

# Phase 3 Plan 11: Vote button style Summary

All vote buttons now share one outlined style with 20px glyphs in a two track grid, a pressed vote keeps its inverted fill in all 64 state combinations, the destructive Yes stays red and wins the cascade, and a Go cascade resolver proves it from the embedded stylesheet.

## What was built

- Task 1 (7dd4745): four Lucide SVG files (check, ban, shield-check, rotate-ccw) with the exact path data from the plan. `.vote-controls` is a two track grid, `.vote-btn` is the outlined 8px button with 16px semibold word, `.vote-btn__icon` carries the full 20px mask base, four modifier rules use literal `url("/static/icons/NAME.svg")` pairs, `.vote-btn__label` wraps. Mark resolved and Reopen span both tracks. `nonCategoryIcons` gained four entries, `phase3TrustGlyphIcons` was added, plus `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon` and `TestVoteButtonsShareOneOutlinedStyle`.
- Task 2 (6754880): inverted neutral pressed fill, hover and press heads at (0,7,0), pressed in flight and pressed too far heads at (0,4,0), too far look, destructive Yes heads (0,3,0), (0,4,0) busy and (0,5,0) hover mix, two disabled Mark resolved heads, the wrapping confirmation row with Cancel (flex 1, 96px) and Yes (flex 2, 144px). New tests `TestVoteStyleResolverCatchesPhase2Precedence`, `TestPressedVoteStatesWinEveryCombination` (64 pressed cases), `TestDestructiveYesHeadWins`, `TestResolveConfirmWrapsCancelBeforeYes`; the two tests in `votes_contract_test.go` now demand the A13 and A14 looks.
- Task 3 (8815b64): `.vote-error` reads in the normal text colour with a 4px critical left border and a `::before` mask of `circle-alert.svg`; `.vote-note` is 14px plain text; `.vote-btn` colour transitions with one reduced motion rule; an eight rule forced colours block last in the file. New tests `TestVoteErrorAlertGlyph`, `TestVoteErrorContrastAcrossThemes`, `TestVoteNoteReadsAsPlainText`, `TestVoteAccentStaysOnTheClosedList`, `TestVoteStyleSurvivesForcedColors`, `TestVoteButtonMotionRespectsReducedMotion`.

## TDD record (RED then GREEN)

- Task 1 RED (after tests, SVGs present, no CSS yet): `--- FAIL: TestPhase3TrustGlyphMasksReferenceEveryExemptIcon`, `--- FAIL: TestVoteButtonsShareOneOutlinedStyle`. GREEN after the CSS.
- Task 2 RED: `--- FAIL` for TestPressedVoteStatesWinEveryCombination, TestDestructiveYesHeadWins, TestResolveConfirmWrapsCancelBeforeYes and the edited TestVoteButtonsMeetTouchTargetAndUseTokensOnly; the negative control `TestVoteStyleResolverCatchesPhase2Precedence` passed from the start, as intended. `TestResolveControlsMeetTouchTargetAndUseTokensOnly` already passed in the RED run because Task 1's `.vote-btn--resolve` edit had already made Mark resolved outlined (the plan expected it to fail on the Task 1 stylesheet, which does not hold).
- Task 3 RED: all six new tests `--- FAIL` on the Task 2 stylesheet. GREEN after the CSS.
- Final `go test ./web/ -count=1 -v` with DATABASE_URL unset: `ok  	pinalert/web`, no `--- FAIL`, no `--- SKIP`. `--- PASS:` seen for all five pinned 03-11 tests (TestPhase3TrustGlyphMasksReferenceEveryExemptIcon, TestPressedVoteStatesWinEveryCombination, TestVoteErrorAlertGlyph, TestVoteErrorContrastAcrossThemes, TestDestructiveYesHeadWins), the two edited ones, the seven added ones, and the guards TestVoteBlockHiddenGuard, TestVisibilityCascadeOverridesAgeRamp, TestMapPopupTintTracksFeedRowAcrossStatesAndThemes, TestResolveConfirmPaintsItsOwnSurface, TestVoteErrorGetsNoChildNodes, TestNoPillShapedControls, TestCategoryGlyphMaskRulesCoverEveryCategory.
- Static counts: each glyph url literal on exactly 2 lines of trust.css, `circle-alert.svg` on 2; every kept and new head is exactly 1 whole line; `@media (forced-colors: active) {` and `@media (prefers-reduced-motion: reduce) {` once each. Long dash lines: trust.css 35 (cap 37), css_contract_test.go 69 (cap 69), votes_contract_test.go 140 (cap 142), new test file 0, four SVGs 0. `gofmt -l` on the three Go files is empty and `go vet ./web/` is clean.

## Notes for later plans

- `p11Matches` supports: type, class, id, `[attr]` and `[attr="v"]`, pseudo-classes hover, active, focus, focus-visible, disabled, `:not(compound)`, `::` pseudo-elements (never match an element), and the descendant combinator only. Anything else, and `>`, `+`, `~`, calls `t.Fatalf` naming the head.
- `p11Resolve` skips any comma part whose last compound has no `.vote-btn*` class token before parsing, so plans 03-13, 03-16 and 03-17 can append unrelated rules without touching this test. Heads are whitespace normalised, so a multi line comma head compares equal to its one line form.
- Plans 03-13 and 03-17: append `clock.svg` and `x.svg` to `phase3TrustGlyphIcons` together with their `nonCategoryIcons` entries; 03-13 updates the `user.svg` comment. `p11ParseRules` is available for media aware checks such as "system colour keywords only inside forced colours blocks".
- Plan 03-17: inside the popup its paragraph reset at (0,2,1) beats the `.vote-note:not([hidden])` and `.vote-error:not([hidden])` margins at (0,2,0), so its popup spacing heads own those margins there. This plan adds no `.leaflet-container` rule.
- Plan 03-18: the too far look keys only on `aria-disabled` with the value `true`; nothing styles `data-too-far`.

## Deviations from Plan

### Command substitutions (rule 11)
- The sandbox refused long compound verify one-liners, a shell heredoc append to trust.css and `git diff | grep` pipelines. I split each verify block into separate `grep -cxF`, `grep -cF`, dash count, `gofmt -l`, `go vet` and `go test -v > file` plus per-test `grep` commands that check the same things, appended CSS with the Edit tool, and wrote git diffs to a scratch file before counting added dash lines (0 in every task, except one gofmt reflow below).

### Auto-fixed Issues
None. The plan executed as written.

### Notes
- `gofmt -w web/votes_contract_test.go` reflowed one existing line (the `.vis-provisional` message, whitespace only). That line already held a long dash before my change and was not rewritten (A69).
- Task 2 RED: `TestResolveControlsMeetTouchTargetAndUseTokensOnly` was already green, see the TDD record.
- Yes uses `min-width: 144px` (outline literal) where the UI-SPEC says about 140px.

## Known Stubs
None. The note rule styles the slot that plan 03-18 fills.

## Threat Flags
None. The four SVG files hold only the listed Lucide path and circle elements, and `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon` fails on `<script` or `href` (T-03-06). The too far and disabled looks set colours and the cursor only, with no input blocking and no opacity (T-03-09).

## Human check (deferred)
The Task 3 browser check (half width Confirm and Dispute at 600px and in the two pane layout, Reopen wrap at 320px, pressed look in light and dark, forced colours emulation, reduced motion emulation) runs in plan 03-20's V-42 session. No database, server or port was used by this plan.

## Self-Check: PASSED
Created files exist: the four SVGs and `web/vote_style_contract_test.go`. Commits 7dd4745, 6754880 and 8815b64 exist on the worktree branch. STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched.
