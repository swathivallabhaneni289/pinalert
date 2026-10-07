---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 17
subsystem: ui
tags: [javascript, css, leaflet, popup-card, resize-observer, forced-colors, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-11 vote button style and phase3TrustGlyphIcons, 03-13 trust block and fadeStage, 03-16 severity label and shelter formatter, 03-18 too far hint on the vote block"
provides:
  - "buildPopupCard and updatePopupCard in map.js: one card per marker, updated in place on every poll"
  - "Neutral 340px popup card with 4px severity bar, 20px header glyph and a max height that scrolls inside itself"
  - "Close button moved first in the popup DOM and drawn as a 44px X (x.svg mask)"
  - "--shadow-popup token in main.css"
  - "popup_contract_test.go with the three pinned tests plus TestPopupCardSurfaceAndGeometry"
affects: [03-19, 03-20]
tech-stack:
  added: []
  patterns:
    - "Build once per marker, update text, classes and attributes in place"
    - "ResizeObserver plus requestAnimationFrame calling only setLatLng on the open popup"
    - "p17 media aware CSS helpers (p17MediaRules, p17MediaIndex) for at-rule blocks"
key-files:
  created:
    - web/popup_contract_test.go
    - web/static/icons/x.svg
  modified:
    - web/static/js/map.js
    - web/static/js/votes.js
    - web/static/css/trust.css
    - web/static/css/main.css
    - web/js_contract_test.go
    - web/votes_contract_test.go
    - web/css_contract_test.go
key-decisions:
  - "The 359px short viewport block repeats the base 16 and 16 max height value (the plan's 'same head with the 16 and 16 form'), declared after the narrow block."
  - "The card keeps a popup-card__meta-lines wrapper so the shelter line stacks under the first meta line beside the glyph."
  - "Close button state heads use base:not([hidden]):hover and :focus-visible, so they stay at (0,5,1) above Leaflet's (0,3,1) pair."
requirements-completed: [TRUST-05, TRUST-07, D-09, D-20]
status: complete
duration: 55min
completed: 2026-10-08
---

# Phase 3 Plan 17: Popup card Summary

Opening a pin now shows one neutral 340px card built once per marker and updated in place on every 30 second poll, with the close X first in the DOM, the severity and state labels above the title, a category and time meta line (plus the shelter line), the scored trust block and the vote controls.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Build the popup card once and update it in place | c9dbf8c |
| 2 | Style the neutral 340px card, 20px header glyph, scrolling max height | 79d7123 |
| 3 | Close button first and drawn as a 44px X | fdfdb1c |

## RED then GREEN

- Task 1 RED: `TestPopupCardBuiltOnceUpdatedInPlace` failed with `map.js: must not contain "buildPopupContent"`, `must not contain "setPopupContent("` and `could not locate "function buildPopupCard(report)" in the text`. GREEN: it passes together with `TestMapPopupCarriesReportStateClasses`, `TestBothSurfacesMountTheSameVisibilityTag`, `TestBothSurfacesMountTheSameVoteBlock`, `TestTrustBlockMountedOnBothSurfaces`, `TestFadeUsesCombinedStage`, `TestVoteBlockReturnsNoteAndConsumersAppendInOrder`, `TestVoteErrorGetsNoChildNodes` and `TestIconGlyphsAreClassDriven` (`ok  pinalert/web`).
- Task 2 RED: `TestPopupHeaderGlyphIs20px` failed with `no rule with head ".leaflet-container .popup-card .icon-badge .icon-glyph"`; `TestPopupCardSurfaceAndGeometry` failed with about 30 messages, among them `the combined wrapper and tip rule must declare exactly background var(--color-bg) ... found map[background:var(--severity-tint, var(--color-bg)) ...]` and `main.css declares --shadow-popup 0 times, want exactly 1`. GREEN: both pass with `TestMapPopupSurfaceIsThemeAware`, `TestMapPopupTintTracksFeedRowAcrossStatesAndThemes`, `TestTrustCSS`, `TestVoteBlockHiddenGuard`, `TestVisibilityCascadeOverridesAgeRamp`, `TestVoteButtonsMeetTouchTargetAndUseTokensOnly`, `TestPressedVoteStatesWinEveryCombination`, `TestCategoryGlyphMaskRulesCoverEveryCategory`, `TestBadgeGlyphContrastAcrossAgeStagesAndThemes`, `TestThemeOverrideBlocksExistForBothModes`, `TestNoPillShapedControls`.
- Task 3 RED: `TestPopupCloseButtonComesFirst` failed with `init: missing "map.on('popupopen', onPopupOpen)"` and `could not locate "function onPopupOpen(evt)"`. GREEN: passes, plus `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon` and `TestCategoryGlyphMaskRulesCoverEveryCategory` with x.svg.
- Final `go test ./web/ -count=1` printed `ok  pinalert/web`. A `-v` run gave 121 `--- PASS` and 0 `--- FAIL` or `--- SKIP` (DATABASE_URL unset; no web test uses a database). All ten named tests (the four new, the four edited, and the two x.svg glyph tests) show `--- PASS`.
- Other gates: `go vet ./web/` clean; `gofmt -l web/popup_contract_test.go web/css_contract_test.go web/votes_contract_test.go` prints nothing; `node --check` parses map.js and votes.js.
- Acceptance greps: `setPopupContent(` 0, `buildPopupContent` 0, `rebuilds its popup` 0, `PinalertTrust.createTrustBlock({ showScores: true })` 1, `actions.appendChild(block.note);` 1, `popupAnchor: [0, -22]` 1, `minWidth: 340, maxWidth: 340, keepInView: true` 1, `--shadow-popup:` 1 in main.css and 0 in trust.css, `max-width: 899.98px` 1 and `max-width: 899px` 0, `popup-card__footer` 0 in both files, `url("/static/icons/x.svg")` 2, `map.on('popupopen', onPopupOpen)` 1, `"static/icons/x.svg": true` 1. Added lines of every touched file contain 0 occurrences of U+2014 or U+2013.

## Notes for plan 03-19

- `onPopupOpen(evt)` is the one popupopen handler to extend (dialog role, `aria-labelledby` pointing at the `pinalert-popup-title-N` id on `.popup-card__title`, focus, padding recompute). It is registered with `map.on('popupopen', onPopupOpen)` in `init()`.
- `relayoutOpenPopup(marker, card)` is the ResizeObserver frame callback; 03-19 adds the reduced motion pan there.
- `cards[id]` holds each card (`{ el, glyph, severity, tag, title, metaGlyph, metaLine, shelterLine, trust, votes, observer, frame }`). `.popup-card` carries `tabindex="-1"`.
- The bindPopup options are `{ className: popupStateClasses(report), minWidth: 340, maxWidth: 340, keepInView: true }` with no autopan keys yet.
- `TestPopupCardBuiltOnceUpdatedInPlace` bans `update()` only inside `updatePopupCard` and `relayoutOpenPopup`. `TestPopupCloseButtonComesFirst` bans `.focus(`, `setAttribute('role'` and `update()` inside the `onPopupOpen` body; because `jsFunctionBody` runs to the next module level `function`, the comment above the following function counts as part of the previous body.
- `Pinalert.formatShelterLine(report)` returns the empty string `''` for any report whose category is not `shelter_open` (and for a missing report); for a shelter it joins a capitalised status word and an integer headcount with a middle dot, and may also return `''` when neither is valid. `updatePopupCard` hides the shelter line whenever the value is empty.

## Deviations from Plan

- **Command substitutions (rule 11).** The sandbox refused compound one-liners that mixed `git diff`, `grep` and `gofmt`, and a heredoc append to a Go test file. I ran each check as a separate plain command (`go test`, `go vet`, `gofmt -l`, `node --check`, `grep -c`, and `git diff -U0 HEAD -- <files>` redirected to a scratch file before counting added long dash lines), and appended the Go tests with the Edit tool. The plan's verify commands name no `env -u DATABASE_URL` prefix; `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset`.
- **Tests committed with their task.** RED runs were executed and recorded above, but each task's test and implementation went into one commit, as the plan defines one commit per task.
- **Header comment.** The map.js header comment was rewritten (new lines carry no dash) because it described the rebuild path this plan removes. The existing first line comment dash was left alone (A69).
- **Stale test comment.** The close glyph contrast sentence in `TestMapPopupTintTracksFeedRowAcrossStatesAndThemes` was folded into the new 4.5:1 matrix on `--color-bg` as the plan directs; the old 3:1 per tint matrix is gone.
- No other deviations. Only the files in files_modified changed (plus the new `x.svg`). STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. No database, server or port was used and p3_0317 was never created.

## Known Stubs

None.

## Threat Flags

None. T-03-06 is mitigated: the card is built with `createElement` only, every string including the description goes through `Pinalert.setText`, class names come from fixed literals and the validated `Pinalert.iconClass` and `popupStateClasses`, `TestPopupCardBuiltOnceUpdatedInPlace` fails on `innerHTML` in the update body, and `TestPopupCloseButtonComesFirst` fails if x.svg holds a script or a link.

## Human check (deferred)

The browser checks belong to the plan 03-20 V-42 owner script (PORT=8792 on 127.0.0.1 with a disposable database) and were not run here: open a pin, open the trust panel and the Mark resolved confirmation, wait past one 30 second poll and confirm both stay open and focus has not moved; confirm the neutral card, left severity bar, 20px header glyph and X close button in light and dark themes, at 320 and 360 widths, and in forced colours.

## Self-Check: PASSED

Created and modified files exist: web/popup_contract_test.go, web/static/icons/x.svg, web/static/js/map.js, web/static/js/votes.js, web/static/css/trust.css, web/static/css/main.css, web/js_contract_test.go, web/votes_contract_test.go, web/css_contract_test.go. Commits c9dbf8c, 79d7123 and fdfdb1c exist on the worktree branch.
