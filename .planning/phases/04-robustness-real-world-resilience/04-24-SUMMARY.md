---
phase: 04-robustness-real-world-resilience
plan: 24
subsystem: web-frontend
tags: [flag, moderation, dialog, focus-return, popup-card, low-data]
status: complete
requires:
  - 04-19 (Pinalert.formatWait, lockPage, unlockPage, the trust.js data-modal-open guard, the Low data vote block)
  - 04-20 (POST /api/reports/{id}/flag, viewer_flagged, FlagReasons, FlagReasonLabel)
  - 04-22 (queue.js script tag position, chrome.css and low.js edits it made)
provides:
  - window.PinalertFlag (createFlagControl, updateFlagControl) in web/static/js/flag.js
  - the one Flag dialog partial web/templates/flag_dialog.html.tmpl on the main page and the Low data view
  - PinalertMap.focusMarker(id)
  - the 04-24 section of chrome.css (control and dialog rules)
key-files:
  created:
    - web/static/js/flag.js
    - web/templates/flag_dialog.html.tmpl
    - web/flag_contract_test.go
  modified:
    - web/static/js/map.js
    - web/static/js/low.js
    - web/static/css/chrome.css
    - web/templates/index.html.tmpl
    - web/templates/low.html.tmpl
requirements: [ROBUST-09]
completed: 2026-10-16
---

# Phase 4 Plan 24: Flag this report on every reading surface Summary

A signed in reader can now tap "Flag this report" at the end of an opened report (the map popup on a laptop, the full screen sheet on a phone, any tab) or at the end of a Low data row, choose one of the server's five reasons in a locked dialog, and see the control read "Flagged", with the 201, 409, 429, 404, 410 and offline answers each handled and nothing saved for later.

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | f13c4f1 | flag.js (control, dialog controller, 201 and 409), the dialog partial, the popup card mount, main page template edits, the first six pinned tests |
| 2 | 370bdb5 | Low data row mount, low.html.tmpl, PinalertMap.focusMarker, the full focus return chain, TestFlagScriptLoadOrder |
| 3 | 3dac5fd | 429 wait memory, gone, offline and Sending answers, the chrome.css section, the two geometry tests |

## Step 0 results

- Every prerequisite row and in-phase input matched on the base, except the one the orchestrator already approved (the Low row shape, below). `feed.js` holds no vote block (`createVoteBlock(` count 0), `createRow`, `createPastCard` and `activateRow` exist, `showToast(message, action, durationMs)` exists, `formatWait`, `lockPage` and `unlockPage` are exported, `trust.js` has the `data-modal-open` guard, `FlagReasons`, `FlagFalse` and `FlagReasonLabel` exist, `viewer_flagged` is served, `/flag` is routed behind the flag gate.
- Import cycle check: `go list -deps ./internal/service | grep -cx 'pinalert/web'` printed 0, so the reason test imports `internal/service` safely.
- No `p424` helper or `TestFlag` name existed before.
- The open check: no test in compact_row, activity_trust, popup, map_follow, vote_hint or vote_controls pins the vote error as the last child of the popup actions block, an exact key list of the object `buildPopupCard` returns, or a child count of `.popup-card__actions`. They pin the order controls, note, error (still true) and "no nested function before the vote block call" (still true). Nothing was stopped or edited there.
- Escape handlers: `map.js` has no document or window level keydown listener (only `onMapKeydown` on the map container, which does not contain the dialog), so nothing was changed in it. `trust.js` owns the capture phase Escape listener and already stands down on `data-modal-open` (04-19). `report-form.js` adds its document keydown listener in `openForm` and removes it in `closeForm`, so it is not live while the Flag dialog is open. `map-tools.js` listens on its search form, `account-menu.js` on its own panel. Nothing to report.
- No existing marker focus export in `map.js` (the earlier `focusOpenPopup` and `takeRemovedPopupFocus` do other jobs), so `focusMarker` is new and was not reused from anywhere.

## createLowRow row object shape (recorded for 04-26 and 04-27)

`createLowRow(report)` builds locals (`li`, `badge`, `glyph`, `body`, `severityLabel`, `title`, `meta`, `written`, `tag`, `trust`, `voteBlock`) and returns one object literal `{ el, glyph, severity, title, meta, written, tag, trust, voteBlock }`; nothing is attached by assignment after creation. It does not call `updateLowRow`; `renderLowRows` calls `createLowRow` for a new id and then `updateLowRow(row, report)` for every row in the same loop pass. The Flag control is built after `body.appendChild(voteBlock.error)`, appended to `body` (so it is the last element of the row), and returned as `flag: flagControl`.

## Placement

- `index.html.tmpl`: `{{template "flag_dialog.html.tmpl" .}}` on its own line directly before `<div id="toast"`, after the closing tag of the `#report-modal` backdrop, inside `#app-shell`. `<script src="/static/js/flag.js?v={{.AssetVersion}}" defer></script>` directly after `queue.js` and before `report-form.js`.
- `low.html.tmpl`: the same call directly before `<div id="toast"`, after the Low report dialog closes, at body level. The same script tag between `queue.js` and `report-form.js`.

## chrome.css 04-24 section (rule list)

`.flag-control:not([hidden])`, `.flag-control:not([aria-disabled="true"]):hover`, `.flag-control:not([aria-disabled="true"]):active`, `.flag-control[aria-disabled="true"]`, `.modal-panel .flag-dialog__title`, `.flag-dialog__reasons`, `.flag-dialog__reason:not([hidden])`, `.flag-dialog__reason input`, `.flag-dialog__hint, .flag-dialog__reassurance`, `.flag-dialog__actions:not([hidden])`, then one `prefers-reduced-motion: reduce` block (`.flag-control:not([hidden])`) and one `forced-colors: active` block (`.flag-control:not([hidden])`, `.flag-control[aria-disabled="true"]`, `.flag-dialog__panel`). Tokens only, no new custom property.

## Deviations from Plan

### Approved amendment

**1. [Orchestrator approved] Low data row part joins the returned literal, not by assignment**
- **Found during:** Task 1 step 0 (by a previous executor, confirmed here)
- **Issue:** The plan expected `row.voteBlock = voteBlock` inside `createLowRow`; on this base `createLowRow` returns an object literal that already holds `voteBlock: voteBlock`.
- **Fix:** Built the control after `body.appendChild(voteBlock.error)`, appended `flagControl.el` to `body`, added `flag: flagControl` to the returned literal; the test needle is `flag: flagControl` and there is no `row.flag = flagControl` assignment. `updateLowRow` calls `PinalertFlag.updateFlagControl(row.flag, report)` right after the vote block update. No extra update call is needed in `createLowRow` because `renderLowRows` runs `updateLowRow` on every new row.
- **Files modified:** web/static/js/low.js, web/flag_contract_test.go
- **Commit:** 370bdb5

### Auto-fixed Issues

**2. [Rule 1 - Bug] Reduced motion rule targets `.flag-control:not([hidden])`**
- **Found during:** Task 3
- **Issue:** The plan names the selector `.flag-control` for the reduced motion rule. The transition is declared on `.flag-control:not([hidden])`, which has higher specificity, so a rule on bare `.flag-control` would never win and motion would stay on.
- **Fix:** The rule and the test use `.flag-control:not([hidden])`.
- **Files modified:** web/static/css/chrome.css, web/flag_contract_test.go
- **Commit:** 3dac5fd

### Minor form differences (no behaviour change)

- The reduced motion and forced colors rules are in two new blocks at the end of the 04-24 section instead of inside the existing 04-08 blocks, to keep the edit append only and away from sibling plans that touch `chrome.css`. `TestChromeCSSTokensOnly` still passes.
- The Task 3 extensions to `TestFlagSendOutcomes` live in a helper (`p424CheckAnswersAndWaits`) called from that test, so the pinned name still carries every assertion.
- The test file builds its dash characters from rune values (`string(rune(0x2014))`), because the write tool turned the unicode escapes into literal characters on the first write; the file is now dash free by the perl scan.
- Commands in the verify blocks were run as plain `go test` (DATABASE_URL is unset in this shell and the sandbox refuses `env -u`).

## Verification run

- `go test ./web/ -count=1` ok; `go test ./... -count=1` ok (all packages).
- By name, all PASS: TestFlagControlClickDoesNotActivateItsRow, TestFlagControlAbsentOnOwnReport, TestFlagControlBuiltOnceUpdatedInPlace, TestFlagDialogCopy, TestFlagDialogMatchesServerReasons, TestFlagDialogLocksPage, TestFlagControlGeometry, TestFlagDialogGeometry, TestFlagScriptLoadOrder, TestFlagSendOutcomes, plus the stay green set (TestPopupCardBuiltOnceUpdatedInPlace, TestVoteBlockReturnsNoteAndConsumersAppendInOrder, TestBothSurfacesMountTheSameVisibilityTag, TestVoteClickDoesNotActivateItsRow, the TestLowData* set, TestPhase4HiddenGuard, TestPhase4ContrastPairs, TestNoMarkupSinksAnywhere, TestChromeCSSTokensOnly, TestVendorMapScriptsLoadInDependencyOrder, TestUserVisibleCopyUsesPlainPunctuation, TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage, TestRowHasNoControls).
- V-20: on the disposable database `p4_0424` (created fresh, dropped afterwards) `DATABASE_URL=postgres://localhost:5432/p4_0424?sslmode=disable go test ./internal/api/handlers/ -run TestFlag -count=1 -p 1 -v` passed 7 tests (TestFlagOnePerAccount, TestFlagOwnReport, TestFlagWithheldReport, TestFlagRateLimit with six subtests, TestFlagNeverChangesVisibility, TestFlagViewerMarker, TestFlagSwaggerCoversRoute), 0 SKIP, 0 FAIL.
- `go test ./internal/service/ -run TestNearbyBatchesVoteReadsOnce` ok. `go vet ./web/` clean, `gofmt -l` clean, `node --check` clean on flag.js, map.js and low.js, dash scans clean, `feed.js` untouched.

## Known Stubs

None. `flagWait` is filled only by a 429, as designed.

## Threat Flags

None. The one network call is the existing 04-20 route; no new endpoint, storage or markup sink.

## Hand off

- 04-26 (moderator screen): flags sent from this control now reach `POST /api/reports/{id}/flag`; the dialog sends only `{"reason": slug}`.
- 04-27: the Low data byte budget must include `flag.js` and the dialog partial's markup (about 2.2 KB of markup and about 16 KB of script, before compression). Run the V-24 Flag steps from Task 3's human check (keyboard only flow on the opened report, the Low data scroll lock, 320 px dialog, the 11th flag wait, offline send with no queue row). The popup card grows by one 44px control plus 4px margin (P4-A6); measure it in V-24. Not run here: no server, no browser (owner rules).
- Phase 8 reuses this dialog on the report page; it expects `#flag-dialog` and `window.PinalertFlag` on the page and a `.flag-control` with `data-flag-report-id`.
- Not verified at runtime here: real focus return, the page lock on touch, and 320/360/390 px layout of the dialog. These are static-checked only; the V-24 steps cover them.

## Self-Check: PASSED

Created files exist (flag.js, flag_dialog.html.tmpl, flag_contract_test.go); commits f13c4f1, 370bdb5 and 3dac5fd exist on the branch; the full test suite and the V-20 database run passed with 0 SKIP.
