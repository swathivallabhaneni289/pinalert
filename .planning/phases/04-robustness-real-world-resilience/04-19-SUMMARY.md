---
phase: 04-robustness-real-world-resilience
plan: 19
subsystem: low-data-view
tags: [low-data, voting, rate-limit, report-form, page-lock, geolocation]
status: complete
requires: [04-10, 04-17, 04-18]
provides:
  - PinalertVotes.createVoteBlock(reportId, options) with the resolve option
  - the 429 wait memory inside castVote (voteWaitKind, rememberVoteWait, knownVoteWait)
  - Pinalert.formatWait, Pinalert.lockPage, Pinalert.unlockPage
  - the Low data report dialog, the map free location object and #low-report
key-files:
  created:
    - web/low_form_contract_test.go
  modified:
    - web/static/js/votes.js
    - web/static/js/app.js
    - web/static/js/report-form.js
    - web/static/js/low.js
    - web/static/js/trust.js
    - web/static/css/modal.css
    - web/static/css/low.css
    - web/templates/low.html.tmpl
    - web/votes_contract_test.go
    - web/vote_controls_contract_test.go
    - web/vote_hint_contract_test.go
    - web/takeback_contract_test.go
    - web/trust_contract_test.go
requirements: [ROBUST-01]
completed: 2026-10-16
---

# Phase 4 Plan 19: Post and vote from the Low data view Summary

A person on `/low` can now Confirm and Dispute nearby reports (never Mark resolved), post a report with the phone position or a typed place through the shared report form with no map, the page behind an open form is locked, and a rate limited vote shows the server's wait in the vote error and sends nothing until the wait has passed.

BASE: `21dc60626a862802596889b3f856c1ea9a12a21d`

## Commits

- 87eaa77 feat(04-19): Confirm and Dispute on the Low data view, and a rate limited vote shows its wait (Task 1)
- 7c1d1f6 feat(04-19): an open report form locks the page behind it (Task 2)
- 11430ea feat(04-19): post a report from the Low data view with the phone position or a typed place (Task 3)
- The SUMMARY commit follows.

## What was built

- **Votes** (`votes.js`, `low.js`, `app.js`): `createVoteBlock(reportId, options)` wraps the Mark resolved button and its confirmation in `if (withResolve)`, where `withResolve` is false only for `options.resolve === false`. `createLowRow` mounts the block with `{ resolve: false }` under the trust block (controls, the too far note, the vote error), `updateLowRow` calls `updateVoteBlock` after `updateTrustBlock`, and the row has a hidden `p.report-row__meta` Written line filled by `updateLowWrittenLine` (from `Pinalert.writtenLine`, also called by `refreshLowTimes`). A 429 answer is remembered per report and vote kind in the module's `voteWaits` object (memory only), `castVote` rejects with the stored server sentence (its wait phrase reworded for the time left through `Pinalert.formatWait`, kept unchanged when the phrase is not found) before any location read or request, and the check sits before the `unresolve` branch so a take back during a wait is refused too.
- **Page lock** (`app.js`, `report-form.js`, `modal.css`, `trust.js`): `lockPage(backdrop)` walks from the backdrop to the body and silences every sibling on the way with `inert` (or `aria-hidden` where inert is missing), skipping `#toast` and `#notice-live` and recursing into any container that holds one; it sets `data-modal-open` on `<html>`. `unlockPage()` restores exactly what it changed and the scroll position. The shared form calls the pair in `openForm` (right after the backdrop shows) and `closeForm` (right after it hides, before focus returns). `modal.css` has `html[data-modal-open] { overflow: hidden; }` and `overscroll-behavior: contain` on `.modal-panel`. `closeOneLevel` in `trust.js` returns false while the attribute is set.
- **Posting without a map** (`low.html.tmpl`, `low.js`, `report-form.js`, `low.css`): the Low page has its own `#report-modal` dialog shell (panel classed `report-form--low`), the `fieldset.low-location` with the status line, the four state controls and the typed place search block, then the shared partial; `createLowLocation()` implements the eight states with a 100 metre accuracy line and an 8 second position timeout, `accuracyPhrase` words the accuracy, and `lowSearchFailureMessage()` ignores the server text. `report-form.js` got `SEARCH_TIMEOUT_MS`, `searchTimeoutSignal()`, the `noteSearchOutcome` hook and the low variant failure path.

## Step 0 census (recorded before any edit)

1. `votes.js` line counts at HEAD (and after this plan, unchanged): `getVoterLocation(` 4 (the plan expected 3; Phase 3.1 added the call in the hint refresh path), `fetch(` 1, `navigator.geolocation.getCurrentPosition(` 1. The header was `function createVoteBlock(reportId) {`. Functions that query `.resolve-confirm` or `.vote-btn--resolve`: `applyTakeBackBlock` (null checked), `openResolveConfirm` and `closeResolveConfirm` (dereferenced `confirmBlock` with no null check), `focusResolveButton` (passes through `canTakeFocus`, which accepts null), `cancelResolveConfirm` (already returns false when nothing is open). Null guards added to `openResolveConfirm` (checks first, before hiding the primary buttons) and `closeResolveConfirm`.
2. The anchor `"createVoteBlock(reportId)"` appeared on 9 lines, all in the four listed files: `votes_contract_test.go` 2, `vote_controls_contract_test.go` 4, `vote_hint_contract_test.go` 1, `takeback_contract_test.go` 2. All re-pointed to `"createVoteBlock(reportId, options)"`; no assertion changed.
3. `trust.js`: `closeOneLevel()` guarded `'report-modal'` and `'account-menu'`, one `document.addEventListener(` line. `TestTrustDisclosure` passes both needle lists over the `closeOneLevel()` body through `p13Ordered`; both gained `'data-modal-open'` after `'account-menu'`.
4. 04-10: the `window.PinalertReportForm = { mount: mount, open: openForm, close: closeForm }` export, `openForm` shows the backdrop then calls `picker.start(`, `closeForm` hides it then refocuses, no other function shows or hides it; `runSearch` holds the one `fetch(` and hands failures to `picker.searchFailureMessage(`; `mount` stores `formVariant`; `resetForm` calls `picker.reset()`. `report-form.js` reads none of `modal-map`, `coord-readout`, `location-notice`. `modal.js` shows or hides only `#location-notice`, never `#report-modal`. `report_form.html.tmpl` is static. None of 04-10's tests reads `low.html.tmpl`.
5. `showToast(message, action, durationMs)`, `writtenLine`, and the Written line class `report-row__meta` are present. `app.js` `fetch(` 2 and `getElementById('toast')` 1 before and after.
6. `low.html.tmpl` had no `#low-report`, `#toast`, `report-form.js` tag or `modal.css` link; all four were added (plus the dialog shell and location fieldset). `createLowRow` returned `{ el, glyph, severity, title, meta, tag, trust }` (a `severity` field the plan did not list) and does not call `updateLowRow` itself (`renderLowRows` does right after), so no extra `updateVoteBlock` call was needed. `TestLowDataRowsNotInteractive`, `TestLowDataOwnFetchBoot` and `TestLowDataAssetPolicy` scope as the plan described; no low test pins the absence of the strings this plan adds.
7. `stripJSComments`, `TestNoMarkupSinksAnywhere`, `TestPhase4HiddenGuard`, `service.WaitPhrase`, `VoteLimitMessage`, `VoteChangeLimitMessage` and the 429 body present. `TestShowDisputedUsesOneSharedQueryParam` present and passing.

The element that carries `report-form--low` is the dialog panel `div.modal-panel` (the element passed as `form`). The Written line is not interactive and adds no listener to any row function.

## Deviations from Plan

**1. [Rule 2 - Missing critical functionality] The focus cycle skips hidden controls.** The Low panel's first control in document order is a hidden location button, so the existing `trapTab` (first and last control by query order) would never wrap Shift+Tab from the first visible control. `trapTab` in `report-form.js` now filters out controls inside a `[hidden]` ancestor. All 04-10 tests still pass.

**2. [Rule 1 - small fix, comments only] Re-indented lines in `createVoteBlock`.** Wrapping the resolve section moved three existing comment lines containing em dashes into the diff. I reworded those three comment lines without dashes so no added line carries a dash.

**3. [Spacing] `.low-page__head .btn` margin.** Changed from `margin: var(--space-sm) 0 0` to `var(--space-sm) var(--space-sm) 0 0` in `low.css` so the two head buttons (Open full version, Report an emergency) keep a gap and wrap cleanly at 320 to 390 px. Tokens only.

**4. [Planner note] `getVoterLocation(` count is 4, not 3.** Recorded in the census; the line counts before and after are equal.

Nothing outside `files_modified` changed; `report_form.html.tmpl`, `index.html.tmpl` and `modal.js` are untouched.

## Verification run

- `DATABASE_URL= go test ./web/ -count=1`: ok (all tests, including every named test in the three task verify lists, printed `--- PASS`).
- Database run on the disposable `p4_0419` (created by me, dropped after): `DATABASE_URL=postgres://localhost:5432/p4_0419?sslmode=disable go test ./internal/... ./web/ -count=1 -p 1 -v`: every package ok, 776 PASS lines, 1 SKIP line. The one SKIP is `TestScreeningFixture` (not database backed: the moderation fixture has no recorded scores until plan 04-27); database backed tests skipped 0. `TestLowDataRoutes`, `TestLowDataRoutesCookieRedirect`, `TestLowPageRenders`, `TestExistingPagesRenderWithChrome` pass.
- `node --check` on votes.js, app.js, low.js, report-form.js, trust.js passes; `go vet ./web/` and `gofmt -l` on the touched Go files print nothing.
- A throwaway node script against a stub (in the scratchpad, not in the repo) ran `castVote` twice for one report against a fake 429: the first call sent one request and rejected with the server sentence and `retryAfterSeconds` 30; the second sent no request and rejected with the same sentence.
- Added lines contain no U+2014 or U+2013; the added Phase 4 files contain none at all; the words light and lite appear in no added line.

## Known Stubs

None. Every control on the Low data view is wired. Rows carry no queue strip or Flag control yet (04-22 and 04-24 own those).

## Notes for the orchestrator

- The human checks in each task (V-24) run in 04-27 on a scratch server: the phone checks of the position states, the geocoder outage sentence, the page lock with VoiceOver, and the 429 wait message on the popup, the Low data row and the Activity Reopen. I did not start a server or a browser.
- The Low data row's vote block still contains the Phase 3.1 take back block (hidden for Active reports; it shows only for a resolved report of the viewer's own, which the Active list never holds). No Mark resolved or resolve confirmation exists on the row.
- A tap on a button marked "too far" runs the hint refresh before `castVote`, so that one path can read a position during a known wait; the wait itself still blocks the vote request.
- `takeback_contract_test.go`, `vote_hint_contract_test.go`, `vote_controls_contract_test.go`, `votes_contract_test.go` and `trust_contract_test.go` changed only by the anchor string and the `'data-modal-open'` needle and assertion.

## Threat Flags

None beyond the plan's register (T-04-01, T-04-09, T-04-13, T-04-16 are mitigated as written; the fix is held in memory for the open form only and written to no browser storage by this plan).

## Self-Check: PASSED

Created and modified files exist, commits 87eaa77, 7c1d1f6 and 11430ea exist on the worktree branch, and the test commands above passed.
