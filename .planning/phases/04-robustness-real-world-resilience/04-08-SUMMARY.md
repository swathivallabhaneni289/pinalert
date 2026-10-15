---
phase: 04-robustness-real-world-resilience
plan: 08
subsystem: ui
tags: [css, chrome, popup, contract-tests, wcag, javascript]
status: complete

requires:
  - phase: 04-robustness-real-world-resilience
    provides: 04-06 sticky .top-chrome wrapper, .disclaimer-bar and chrome.css linked on every full page
provides:
  - --disclaimer-bar-height token (67.8px, 48.2px from 480px, 28.6px from 900px) declared once in main.css
  - shell, map and login page heights that subtract the bar; account trigger anchored 16px under the bar
  - popup max heights and narrow autopan top reserve that read against the shorter map
  - data-popup-open="true" on the root element while a map popup is open
  - .inline-error, .inline-note, .btn[hidden], plain, primary and destructive button states
  - one all files markup sink scan with a comment aware stripper, and the Phase 4 hidden guard
affects: [04-13, 04-16, 04-22, 04-24, 04-26, 04-27]

key-files:
  created:
    - web/chrome_css_contract_test.go
    - web/js_sinks_contract_test.go
  modified:
    - web/static/css/main.css
    - web/static/css/auth.css
    - web/static/css/chrome.css
    - web/static/css/trust.css
    - web/static/js/map.js
    - web/profile_nav_contract_test.go
    - web/popup_contract_test.go

key-decisions:
  - "The popup max height count in trust.css is four, not three: Phase 3.1 plan 11 added a laptop block after the plan text was written"
  - "phone.css is not touched; its phone shell height still ignores the bar (see Follow ups)"

requirements-completed: []

duration: about 55 min
completed: 2026-10-16
---

# Phase 4 Plan 08: Pages fit under the bar Summary

The account trigger now hangs 16px under the bar from inside `.top-chrome`, the shell, map and login page subtract `--disclaimer-bar-height`, the popup reserves read against the shorter map, and one scan proves no script under `static/js` uses a markup parsing sink.

## Task commits

| Task | Commit | What |
|------|--------|------|
| 1 | 0f10f09 | bar token, wrappers that subtract it, `.app-header` absolute under the bar, `html` scroll padding, geometry tests |
| 2 | a345828 | four popup max heights, measured narrow top autopan reserve, root popup flag, `stripJSComments` and the sink scan |
| 3 | 2c40a9b | `.inline-error`, `.inline-note`, button states, forced colors and reduced motion, contrast, token and hidden guard tests |

## Values the plan asked to record

- JS file count N used by `TestNoMarkupSinksAnywhere`: 13 (not 10: Phase 3 and 3.1 added `trust.js`, `map-tools.js`, `activity-tabs.js`, `visibility.js`; `report-form.js` is not on this base). The constant is `minJSFilesScanned`.
- `data-popup-open` counts: no `strings.Count` over that name exists in any test, so none was amended.
- 04-06 declarations: `.top-chrome` already had sticky, top 0 and z-index 1050, and `.disclaimer-bar` already had flex, align-items and border-box, so only `min-height` was added to the bar rule. 04-06 already left a forced colors rule for the bar, so none was re-added.
- Phase 3.2 bell: none exists in templates, CSS or JS, nothing moved.
- Extra row backgrounds in `TestPhase4ContrastPairs`: none. main.css maps the three severities to their tint tokens, and trust.css only paints `--color-surface` or transparent, both already in the pair list.
- `TestPopupCardSurfaceAndGeometry` (03-17) was amended as the plan predicted: its three pinned max height strings now end with `- var(--disclaimer-bar-height)`.

## Verification

- `go test ./web/ -count=1` and `go test ./... -count=1` (no DATABASE_URL): all packages ok.
- Database wave sample on disposable `p4_0408` (created by me, dropped afterwards, confirmed gone): `DATABASE_URL=postgres://localhost:5432/p4_0408?sslmode=disable go test ./internal/... ./web/ -count=1 -p 1 -v`, all 12 packages ok, 0 `--- FAIL` and 0 `--- SKIP` lines.
- Named tests PASS: `TestDisclaimerBarToken`, `TestBarTokenReadByPageWrappers`, `TestTopChromeGeometry`, `TestPopupMaxHeightsReadTheBarToken`, `TestPopupOpenFlagOnRoot`, `TestStripJSComments`, `TestNoMarkupSinksAnywhere`, `TestInlineMessageLooks`, `TestPlainButtonStates`, `TestPhase4HiddenGuard`, `TestPhase4ContrastPairs`, `TestChromeCSSTokensOnly`, plus amended `TestActivityPageLinksBackToTheMap`, `TestPopupPaddingTokensAgree`, `TestPopupCardSurfaceAndGeometry`, and unedited `TestPrimaryMapHasResolvedHeight`, `TestThemeToggleFloatsClearOfTheReportButton`, `TestThemeOverrideBlocksExistForBothModes`, `TestAccountMenuHiddenGuard`, `TestVoteBlockHiddenGuard`, `TestNoPillShapedControls`, `TestThemeModuleUsesNoMarkupParsingSink`, `TestPopupDialogFocusAndOpener`.
- `go vet ./web/`, `gofmt -l web/` and `node --check web/static/js/map.js` clean. No dash character on any line added.

## Deviations from Plan

1. **[Plan text out of date] Popup max height count is four.** The plan's acceptance line says `grep -c 'var(--disclaimer-bar-height)' trust.css` prints 3 and the test text says "exactly 3". Since Phase 3.1 plan 11 there are four `100dvh` max height values (base, laptop, narrow, short), and the plan's own artifacts list also names four. All four were changed, and the check asserts exactly 4 with that provenance.
2. **[Test placement] The trust.css clause is its own test.** The plan says to extend `TestBarTokenReadByPageWrappers`; the clause is `TestPopupMaxHeightsReadTheBarToken` in the same file so the two failures read separately. `TestBarTokenReadByPageWrappers` itself covers the wrappers.
3. **[Comment wording] map.js line 11.** No comment in `map.js` names a markup sink on this base (the `appendChild()` wording is already gone), so no comment was reworded.
4. **[Tool] `env -u DATABASE_URL` was not used** (the sandbox refuses it); DATABASE_URL was simply unset for the no database runs.
5. **Task 1 test TestDisclaimerBarToken** counts `:root` rules inside the 900px block rather than requiring that block to hold only one rule, because main.css has other `@media (min-width: 900px)` blocks.

## Known Stubs

None.

## Follow ups for the orchestrator

- **phone.css still ignores the bar on phones (not in this plan's files).** `web/static/css/phone.css` lines 32 to 38 give `body > .app-shell` `height: 100vh/100dvh` and `.pane--map #map` `40dvh`, inside `max-width: 899.98px`. These beat the new main.css calcs at phone width, so on a phone the shell is still one bar height taller than the viewport and the page scrolls by about 68px at 320px. No Phase 4 plan lists `phone.css`. Fix: `height: calc(100dvh - var(--disclaimer-bar-height))` (with the `100vh` fallback first) on `body > .app-shell`. phone.css already reads the token with a `0px` fallback in the map search rules, so the file is ready for it. Decide which plan owns this edit.
- Browser harness checks that change by design: the account trigger now sits under the bar on laptop widths (it is hidden on phones by phone.css); the main.css and auth.css pages are shorter than the viewport by the bar; the popup max height is smaller by the bar height.
- Human checks in Task 1's `<verify>` are deferred to 04-27.

## Threat Flags

None. T-04-01 is mitigated: `TestNoMarkupSinksAnywhere` scans all 13 scripts (fails below 13), and `TestStripJSComments` proves the stripper keeps `https://`, quoted slashes and template text while dropping real comments. The new CSS adds only an empty `content: ""` mask pseudo element.

## Self-Check: PASSED

Files exist (`web/chrome_css_contract_test.go`, `web/js_sinks_contract_test.go`) and commits 0f10f09, a345828, 2c40a9b are on the worktree branch.
