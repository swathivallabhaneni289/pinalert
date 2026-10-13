---
phase: 03-trust-model-hardening-diversity-weighted-trust
verified: 2026-10-08T00:00:00Z
reverified: 2026-10-10T00:00:00Z
reverified_again: 2026-10-10T00:00:00Z
reverified_after_redesign: 2026-10-10T00:00:00Z
reverified_after_tick_marks: 2026-10-13T00:00:00Z
status: human_needed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 9/9
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  note: "Focused re-runs after gap closure plans 03-25 (merged in 584cef3) and 03-26 (merged in 2cc0158), then after the redesign and the shorter wording (plans 03-21 to 03-24, 03-27, 03-28 and 03-29, last merge 28d6f3d, HEAD fde18f2). See Re-verification after 03-25, Re-verification after 03-26 and Re-verification after the redesign. Fourth run on 2026-10-13 after plans 03-30 and 03-31 and quick task 261010-psc (HEAD 8b9c802): 9/9, no regressions, see Re-verification after plans 03-30 and 03-31."
gaps: []
deferred: []
human_verification:
  - test: "Trust counters and words (V-42 step 5). As A post a low severity report at R (12.9716, 77.5946), confirm as B then C (C from 12.9686, 77.5946). Post a critical report and confirm it once as B."
    expected: "Row and popup go 0, 1, 2 confirmed. The lead sentence disappears at 2 and Report up to date (A86, was Up to date) appears with the popup score. The critical report with one place reads Too early to tell with no extra dimming."
    why_human: "Needs three signed-in profiles, a live server and Chrome location overrides. Tests prove the server values, not the rendered browser result."
  - test: "Panel open and close (step 6). Open the trust panel by click, tap, Enter and Space, close by Escape, on a feed row and in a popup. Listen with VoiceOver."
    expected: "Panel opens and closes on all four inputs. VoiceOver reads the toggle name then the hint 'Shows what these mean.'"
    why_human: "Keyboard and screen reader behavior. The web contract tests are static source checks and do not run the browser JS."
  - test: "Escape by surface ownership (step 7, plan 03-19). Walk each focus position with a popup panel and a row panel open."
    expected: "First Escape closes the panel and focuses the toggle, the second closes the popup. Popup panel closes before a row panel when focus is in the popup."
    why_human: "Focus order across two surfaces is runtime browser behavior."
  - test: "Too far hint and refusal (steps 8 and 9, plans 03-04 and 03-18). Vote from 2 km, check the buttons, force a POST from the console, test the 0.9896 km and 1.0008 km boundary pair, wait out the 120 second bound, test silent refresh, Ask and Block location states."
    expected: "Refusal text 'Too far to vote. You need to be within 1 km of this report.' with HTTP 403 field location. Buttons show off within one poll, stay Tab reachable, do nothing on click. 12.9805 accepted, 12.9806 refused. Hint clears after the bound. Own Mark resolved stays enabled from far. Blocked location shows the locked sentence and sends no POST."
    why_human: "Needs Chrome Sensors, permission prompts and the Network panel. Server refusal is proven by TestCastVoteTooFar, but the browser hint and cache are not."
  - test: "Mark resolved confirmation and focus (step 10, plan 03-09). Keyboard only on a feed row, then as reporter in list view at phone width and from the map popup at desktop width."
    expected: "Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body."
    why_human: "Focus management and screen reader announcement."
  - test: "Reopen on Activity (step 11, plan 03-15). Resolve own report, open Activity, open the panel, press Reopen."
    expected: "Counters show 0 and 0 with no numbers, panel opens with server sentences, Reopen and the Resolved label go, panel stays open with refreshed sentences, focus on the trust toggle."
    why_human: "Live focus and refresh behavior."
  - test: "Width and wrapping (step 12, plan 03-16). At 320px and 200 percent text size, with a long unbroken title and URL."
    expected: "No horizontal scroll in the 236px feed body or the 188px Activity body. Severity word shows before the title and is read first by a screen reader. Labels stay readable and unclipped."
    why_human: "Layout and screen reader order."
  - test: "Vote layout and button look (steps 13 and 14, plan 03-11). 600px row, 900px two pane layout, 320px Activity Reopen, light and dark, forced colors, reduced motion."
    expected: "Confirm and Dispute each about half width, Mark resolved full width. Pressed Confirm is the inverted fill. Forced colors shows glyphs, Highlight fill, GrayText disabled, bordered Yes. No hover transition under reduced motion."
    why_human: "Visual and media feature rendering."
  - test: "Popup card, position, reduced motion, keyboard and focus (steps 15 to 18, plans 03-17 and 03-19)."
    expected: "Panel and confirmation survive a 30 second poll with focus unmoved. Popup tip centred and card inside the view at 320 and 360px, controls not covered after autopan, no animated pan under reduced motion. Pins have names and open on Enter and Space. Focus survives poll removal and reorder. Focus returns to the marker or row on close."
    why_human: "Map geometry, autopan, poll timing and focus are runtime browser behavior."
  - test: "Look and feel and plain language check (step 19). Check the toggle press fill on a Provisional row, forced colors, both themes. Ask someone who has not seen the app what the numbers mean. Judge whether the shorter wording (A85) reads short enough."
    expected: "The two signals read as separate things (still current versus reporter) and the person finds the panel without help."
    why_human: "Visual judgment and comprehension, which no automated check can give. This is the check that the goal of a reader understanding the signals is met in practice."
  - test: "Owner housekeeping (step 20 and reminders). Run migrate once on the owner dev database for migration 00005 after merge. Drop p3_uat when finished."
    expected: "Index idx_reports_session_expires exists on the dev database."
    why_human: "Touches the owner's own database, which the executors and this verifier are not allowed to use."
---

# Phase 3: Trust Model Hardening (Diversity-Weighted Trust) Verification Report

**Phase Goal (MVP):** As a person reading the feed during an emergency, I want to see how many different nearby people confirmed or disputed each report, and whether it is still current and its reporter reliable, so that a few accounts cannot make a weak report look well corroborated.
**Verified:** 2026-10-08
**Status:** human_needed
**Re-verification:** Initial verification 2026-10-08; focused re-verifications after plans 03-25 and 03-26 on 2026-10-10, and after the redesign and the shorter wording (plans 03-21 to 03-24, 03-27, 03-28 and 03-29) on 2026-10-10, and after plans 03-30 and 03-31 and quick task 261010-psc on 2026-10-13 (see the sections near the end)

Every automated check passes. The only open items are the browser, two profile, keyboard, screen reader and visual checks that plans 03-04, 03-09, 03-11, 03-13, 03-15 to 03-19 deliberately deferred to the owner's V-42 script (folded into 03-20-SUMMARY.md). None is a gap.

## User Flow Coverage

| Step of the story | Expected | Evidence in code | Status |
| --- | --- | --- | --- |
| Reader sees how many different nearby people confirmed | A place count N, not the raw vote count | `BuildTrust` sets `ConfirmedPlaces` and `DisputedPlaces` from `independentCellCount(placeVoteCells(...))` (`internal/service/trustview.go`). Browser prints `trust.confirmed_text` (`web/static/js/trust.js:407`). | VERIFIED |
| Reader sees whether the report is still current | Fast decaying currency band, score, sentence | `buildCurrency` in `internal/service/currency.go`; window is lifetime divided by 4, 25 points lost per window. Sent as `trust.currency`. | VERIFIED |
| Reader sees whether the reporter is reliable | Slow decaying reporter tag, score, sentence | `ScoreReliability`, 90 day window, 5 judged reports minimum, tags at 75 and 35. Sent as `trust.reporter`. | VERIFIED |
| A few accounts cannot fake corroboration | Same place votes count once, distant voters refused | Voter cell at precision 7 computed server side; 1 km rule in `CastVote` step 6; reporter and own report votes excluded in `filterVoteRows`. | VERIFIED (spoofed GPS is a documented limit) |

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC1: the feed shows "confirmed by N nearby" computed from distinct confirming geohash cells, not raw votes | VERIFIED | `BuildTrust` -> `independentCellCount` (set of cells). Called by the feed (`report.go:607`), the vote response (`trust.go:457`) and Activity (`auth.go:406`). The old raw count has no remaining use (`grep` for confirm_count, vote_count, votes_total, confirm_cells in `internal/api` and `web/static` found nothing). `TestTrustCountsDistinctPlacesNotVotes` passes in the gate log. |
| 2 | SC1: the geohash precision is deliberately chosen and documented | VERIFIED | `voterGeohashPrecision = 7` in `trust.go:35`, distinct from report precision 8 (`report.go:25`), cell derived server side only (`trust.go:418`, no client field). `docs/TRUST-MODEL.md` has the precision 5 to 8 derivation table, per city widths, a seeded 300 m simulation and a "Why 7" section. `TestGeohashPrecisionWitnesses` (8 subtests) and `TestTrustModelDocMatchesCode` pass. |
| 3 | SC2: a fast decaying "is this still current" score exists | VERIFIED | `currency.go`: `CurrencyWindowDivisor = 4`, `CurrencyPointsPerWindow = 25`, score reaches 0 after four windows of the report's lifetime. `TestCurrencyScoreBoundaries` and `TestFadeStageMapping` pass. |
| 4 | SC2: a slow decaying "is this source reliable" score exists and is a distinct signal | VERIFIED | `reliability.go`: 90 day window (`ReliabilityWindow`), age weighted outcomes (`outcomeWeight`, `weightShift`), minimum 5 judged reports, separate tags Reliable, Mixed, Unreliable, New, None. `TestReliabilityTable` and `TestReporterSignalFromSeededHistory` pass. |
| 5 | SC2: both signals are on the wire side by side on every surface | VERIFIED | `TrustResponse` carries `currency` and `reporter` together (`internal/api/handlers/trust.go`). `trustToResponse` is called by `reports.go:205`, `votes.go:199` and `auth.go:422`. `TestFeedTrustMatchesCastVoteTrust` passes: feed and vote response carry the same object. |
| 6 | SC2: the browser renders both signals from server values | VERIFIED | `trust.js` builds a `signals` container holding `createSignal` for currency and reporter (lines 199 to 204) and fills each from `trust.currency` and `trust.reporter` (469 to 475). `feed.js`, `map.js` and `activity.js` all use `PinalertTrust.createTrustBlock` and `updateTrustBlock`. Template loads `trust.js` (`index.html.tmpl:137`, `profile.html.tmpl:60`). Static contract tests in `web/` pass. Actual rendering is a human check. |
| 7 | Vote distance rule (D-01) enforced server side, fail closed | VERIFIED | `MaxVoteDistanceKm = 1.0`, `withinVoteRadius` uses `<=` so NaN and Inf are refused, `CastVote` refuses before any write; reporter's own resolution is the only exemption and is decided from the database identity. `TestCastVoteTooFar` (confirm, dispute, resolve, reopen, expired from far) passes. |
| 8 | Concurrent votes never lose an update or skew N | VERIFIED | `TestTrustCountUnderConcurrentVotes` passes, also under `-race` per 03-20 summary. |
| 9 | Trust object is identical for every viewer and leaks no identifiers | VERIFIED | `TrustInput` has no viewer field. `TestTrustResponseOmitsIdentifiers` passes. |

**Score:** 9/9 truths verified (0 present but behavior unverified at the server level). Browser behavior of truths 6 is covered by the human items below.

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/service/geo.go` | 1 km rule, refusal text | VERIFIED | 74 lines, substantive, used by `CastVote` and the votes handler. |
| `internal/service/trust.go` | precision constant, `independentCellCount`, `filterVoteRows`, `CastVote` | VERIFIED | Wired into feed, vote and Activity paths. |
| `internal/service/trustview.go` | `BuildTrust` | VERIFIED | Three production callers. |
| `internal/service/currency.go`, `reliability.go`, `reporter.go` | the two signals | VERIFIED | Called from `BuildTrust` and the three read paths. |
| `internal/api/handlers/trust.go` | wire shape | VERIFIED | Copies fields only, no arithmetic. |
| `web/static/js/trust.js` | shared browser block | VERIFIED | 524 lines, used by feed, map and Activity. |
| `docs/TRUST-MODEL.md` | documented model and precision | VERIFIED | 493 lines, kept true by `TestTrustModelDocMatchesCode`. |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| `report.go` feed read | `BuildTrust` | batched votes plus `ReporterSignals` | WIRED |
| `CastVote` | `BuildTrust` | post insert read of current votes | WIRED |
| `auth.go` Activity | `BuildTrust` | own report votes | WIRED |
| handlers | `trustToResponse` | feed, vote, Activity JSON | WIRED |
| `feed.js`, `map.js`, `activity.js` | `PinalertTrust.updateTrustBlock` | server trust object | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data variable | Source | Real data | Status |
| --- | --- | --- | --- | --- |
| trust block N and M | `trust.confirmed_text` | `CurrentVotesForReports` rows, distinct cells | Yes (database query, `TestTrustCountsDistinctPlacesNotVotes` seeds real rows) | FLOWING |
| currency signal | `trust.currency` | the same vote rows plus report times | Yes | FLOWING |
| reporter signal | `trust.reporter` | `ReporterHistoryReports` plus votes | Yes (`TestReporterSignalFromSeededHistory`) | FLOWING |

### Behavioral Spot-Checks

No server was started and no test was re-run by the verifier. The orchestrator ran the full suite on HEAD 560a883 on a fresh scratch database: build and vet clean, 367 top level tests passed, 0 failed, 0 skipped. The verifier confirmed in the saved log (`gate-w9.log`) that all named tests above print `--- PASS`, that there are 0 `--- FAIL` or `--- SKIP` lines, and that every package is `ok`.

### Probe Execution

SKIPPED (the phase declares no probe scripts).

### Requirements Coverage

| Requirement | Source plans | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| TRUST-05 | 03-01, 03-02, 03-04, 03-05, 03-07, 03-09, 03-11, 03-13, 03-15, 03-16, 03-17, 03-18, 03-19, 03-20 | "confirmed by N nearby" from distinct confirming cells, not raw votes | SATISFIED | Truths 1, 2, 7, 8. |
| TRUST-07 | 03-03, 03-06, 03-08, 03-10, 03-12, 03-13, 03-14, 03-15, 03-16, 03-17, 03-19, 03-20 | two distinct signals, fast decaying currency and slow decaying source reliability | SATISFIED | Truths 3, 4, 5, 6. |

Both IDs appear in plan frontmatter and are accounted for. No other REQUIREMENTS.md row maps to Phase 3, so there are no orphaned requirements. The other IDs in plan frontmatter (D-nn, R-nn) are phase decision and review references, not REQUIREMENTS.md rows.

Housekeeping: the TRUST-05 and TRUST-07 checkboxes (lines 72 and 78) and the traceability rows (lines 375 and 376) in `.planning/REQUIREMENTS.md` still read pending. Update them after the owner's V-42 pass.

### Anti-Patterns Found

None. `grep` for TBD, FIXME, XXX, TODO and HACK across `internal`, `web`, `docs` and `cmd` returned nothing. No stub rendering was found: `trustToResponse` copies real service values and the browser holds no trust rule of its own. Known Stubs in 03-20 is None.

Phase 2 regression check: `02-VERIFICATION.md` status is passed, and the full suite on the final tree includes the Phase 2 vote, visibility and e2e tests, all passing.

### Known limits (documented, not gaps)

- Spoofed browser location defeats both the 1 km rule and the cell rule. `docs/TRUST-MODEL.md` states this plainly and a test fails if the doc ever claims spoof resistance.
- GPS error, the 30 second poll and the advisory far hint are listed in Honest limits.

## Human Verification Required

Eleven items are in the frontmatter (the V-42 script steps 5 to 20 grouped by concern). They are the whole of what remains. Record any failure as a gap for `/gsd-plan-phase 3 --gaps`.

## Gaps Summary

No gaps. Both ROADMAP success criteria are met by shipped, wired, tested code. The status is human_needed only because the deliberately deferred browser, keyboard, screen reader and visual checks have not yet been run by the owner.

## Re-verification after 03-25

**Date:** 2026-10-10. **Scope:** focused re-run after gap closure plan 03-25 (code commits 53fb069 and a001850, merged in 584cef3). The first run above is kept as written. Plans in `held-redesign-drafts/` are drafts of a redesign on hold and were not read as part of this check.

**Result:** status stays `human_needed`, score 9/9, 0 gaps, 0 regressions.

### The nine truths against the current code

| # | Truth | Re-check | Status |
| --- | --- | --- | --- |
| 1 | Distinct place count N | `independentCellCount` still the single implementation, called from `currency.go:207`, `currency.go:251` and the three read paths. `TestTrustCountsDistinctPlacesNotVotes` PASS. | VERIFIED |
| 2 | Precision chosen and documented | `voterGeohashPrecision = 7` (`trust.go:35`). `TestTrustModelDocMatchesCode` PASS on the current tree. | VERIFIED |
| 3 | Fast decaying currency | `CurrencyWindowDivisor = 4`, `CurrencyPointsPerWindow = 25` (`currency.go:78,81`). `TestCurrencyScoreBoundaries` PASS. | VERIFIED |
| 4 | Slow decaying reliability | 90 day window, minimum 5 judged (`reliability.go:121,126`). Review fix WR-05 since the first run added `ReliabilityRowCap = 30` judged rows (line 131); the window, minimum and tags are unchanged. `TestReliabilityTable` PASS. | VERIFIED |
| 5 | Both signals on the wire together | `TestFeedTrustMatchesCastVoteTrust` PASS. | VERIFIED |
| 6 | Browser renders both from server values | `trust.js` untouched by 03-25. Browser rendering stays a human item. | VERIFIED (server side); browser in human items |
| 7 | 1 km rule server side, fail closed | `MaxVoteDistanceKm = 1.0`, `withinVoteRadius` uses `<=` (`geo.go:27,60`). `TestCastVoteTooFar` PASS. | VERIFIED |
| 8 | Concurrent votes never skew N | `TestTrustCountUnderConcurrentVotes` PASS. | VERIFIED |
| 9 | Identical for every viewer, no identifiers | `TestTrustResponseOmitsIdentifiers` PASS. | VERIFIED |

Between the first run (HEAD 560a883) and now, `internal/` and `docs/` changed only through review fixes WR-02 to WR-08 and the reliability row cap (14 files, mostly tests). None changes a must-have, and the doc test keeps `docs/TRUST-MODEL.md` true to the code.

### What 03-25 touched, read from the current code

- `web/static/js/votes.js`: `isInOpenPopup(controls)` (line 928) returns true only when the controls sit in a `.leaflet-popup` carrying `data-popup-open="true"`. `findLiveControls` (line 939) uses it in both branches for popup origins; row origins are unchanged. No timer, animation event or style read is involved. `focusAfterVote` (line 1029) is unchanged: when no live controls remain and a removal was captured it calls `focusAfterRetract`, which sends popup origins to the map container (line 995). `focusIsLost` treats focus inside the dying controls as lost, so the map container takes focus.
- `web/static/js/map.js`: `onPopupOpen` sets `data-popup-open` (line 439) and `onPopupClose` removes it synchronously (line 565), before Leaflet's 200 ms fade ends and before the refetch resolves. This is the signal `isInOpenPopup` reads.
- `refusedTooFar`: set in the `onControlsClick` catch branch from `err.field === 'location'` (line 1210), reset in `clearVoteError` (line 746). `clearRefusalWithHint` (line 753) clears the line only when the hint was showing before the render, is hidden after it and the line holds the too far refusal. It runs after `applyDistanceHint` in `updateVoteBlock` (line 606) and in `refreshHintThenRetry` (line 1115). GPS denied and generic failure lines carry no `location` field, so they are never cleared by it.
- New contract tests exist and PASS: `TestPopupResolveFocusIgnoresAFadingPopup` (`web/vote_controls_contract_test.go:429`) and `TestTooFarRefusalClearsWithTheHint` (`web/vote_hint_contract_test.go:346`). Both are static source checks. Against the code before the fix (`git show 53fb069^` and `a001850^`) `votes.js` has zero occurrences of `isInOpenPopup`, `refusedTooFar` or `clearRefusalWithHint`, so both tests would fail there, as the SUMMARY says. Being static, they do not run the browser; the live proof is the scripted run below.

### Evidence

- Verifier ran the full suite once on a fresh scratch database (`p3_ver_gate`, created and dropped by the gate script): build and vet clean, 373 top level tests PASS, 0 FAIL, 0 SKIP, every package `ok`. This matches the orchestrator's 373 of 373.
- Scripted browser run on the merged code, cited not re-run: 164 PASS, 0 FAIL, 8 NEEDS A PERSON (Chromium and WebKit). UAT test 4 check S1.11b and test 5 check P4 now pass in both engines.
- `gofmt -l` lists `internal/api/handlers/geocode_test.go`, `internal/mailer/resend.go` and `web/js_contract_test.go`. None is in the 03-25 diff; this is an Info item and not a Phase 3 gap.
- Debt markers (TBD, FIXME, XXX) in `votes.js`, `map.js`, `internal`, `docs` and `cmd`: none.

### Requirements cross-reference

Plan 03-25 declares `requirements-completed: []` and the first run's coverage stands. TRUST-05 and TRUST-07 are both present in `.planning/REQUIREMENTS.md` (lines 72 and 78, traceability rows 375 and 376), both mapped to Phase 3, both accounted for by the plans listed in Requirements Coverage above. No orphaned ID. Both still read Pending there; REQUIREMENTS.md was not edited by this verification.

### Items that remain for a person (from 03-UAT.md, nothing new)

Pending: test 2 (panel open and close, VoiceOver), 3 (Escape by surface ownership), 4 (Ask in Safari and the prompt UI; scripted part proven), 5 (VoiceOver announcement of the Mark resolved confirmation; scripted part proven), 7 (width and wrapping at 320px, 200 percent text), 8 (button look in Safari, real Windows High Contrast not available), 9 (popup card, position, reduced motion, keyboard and focus), 10 (look and feel, plain language check, plus the three cosmetic owner remarks logged under Gaps in 03-UAT.md that need explicit UI-SPEC amendments), 11 (owner housekeeping: migrate 00005 on the dev database, drop p3_uat). Passed: tests 1 and 6. The two scripted gaps in 03-UAT.md are resolved.

Owner decisions to raise: the three cosmetic remarks from test 1 (underline on hover, reporter tag position, text length) change locked UI decisions and belong with the on-hold redesign decision.

## Re-verification after 03-26

**Date:** 2026-10-10. **Scope:** second focused re-run after gap closure plan 03-26 (code commits a6d1b9a and 9531778, merged in 2cc0158, summary ae59bb4), with 03-SECURITY.md (d715a6c) now present. Plans in `held-redesign-drafts/` are drafts of a redesign on hold and were not read.

**Result:** status stays `human_needed`, score 9/9, 0 gaps, 0 regressions. No new human items.

### The nine truths against the current code

`internal/`, `docs/` and `cmd/` have no diff between 2f669c4 (the 03-25 re-run base) and HEAD, so truths 1 to 5 and 7 to 9 rest on unchanged server code. The gate log below shows each named test as `--- PASS`: `TestTrustCountsDistinctPlacesNotVotes`, `TestTrustModelDocMatchesCode`, `TestCurrencyScoreBoundaries`, `TestReliabilityTable`, `TestFeedTrustMatchesCastVoteTrust`, `TestCastVoteTooFar`, `TestTrustCountUnderConcurrentVotes`, `TestTrustResponseOmitsIdentifiers`. Truth 6 (browser renders both signals) is unaffected by 03-26 (`trust.js`, `trust.css`, templates, `votes.js` and `activity.js` untouched); the only web diff since 2f669c4 is `map.js`, `feed.js` and `popup_contract_test.go`. All nine stay VERIFIED, with browser rendering of truth 6 in the human items.

### What 03-26 touched, read from the current code

- `web/static/js/map.js`, `flyTo` (line 868): the open call is now `if (!marker.isPopupOpen()) { marker.openPopup(); }` (lines 881 to 883), after the pan or fly and before the unchanged opener claim `if (listOpenerId !== null && marker.isPopupOpen())` that sets `popupOpener = { id: listOpenerId, kind: 'list' }`. `openPopup(` appears once in the function. The `Pinalert.onSelect` subscriber (line 203, `source === 'list'` then `flyTo(id)`) is unchanged, and `onPopupOpen` still consumes `listOpenerId` and sets `data-popup-open` (line 439). Effect: a row activation reaches `flyTo` twice (the selection callback, then the A74 seam in `activateRow`, `feed.js:119 to 124`), and the second call no longer re-opens an open popup, so the card keeps the focus `popupopen` gave it and the list still claims the opener. A repeat activation on an already open popup keeps focus where it is (noted in the SUMMARY, not a defect of the contract).
- `web/static/js/feed.js`, `focusIsLost(active)` (line 255): true for null or the body, true when `active.closest('.leaflet-popup')` is not null and its `data-popup-open` is not `'true'`, else false. `focusAfterRemoval` (line 279) now returns early only when `!focusIsLost(active)`; the candidate loop (next or previous row, then `#map`, then the view toggle, then the empty states) is unchanged. `clearRows` shares it. The signal is the mark that `onPopupClose` clears synchronously (`map.js:565`), with no timer, animation event or style read, so it does not depend on Leaflet's 200 ms fade. Focus on a live open popup (mark `'true'`) or any other element is still left alone. This is the same class and the same signal as the 03-25 fix in `votes.js`.
- Debt markers (TBD, FIXME, XXX) in `feed.js`, `map.js` and `popup_contract_test.go`: none. No em or en dash was added to this report.

### New contract tests

- `TestListRowActivationOpensThePopupOnce` (`web/popup_contract_test.go:631`) and `TestFocusAfterRemovalIgnoresAFadingPopup` (line 738) exist and PASS in the gate log. The order assertion in `TestPopupDialogFocusAndOpener` (line 575) was amended to expect `isPopupOpen()` before `marker.openPopup()`; it also PASSES.
- Both new tests are static source checks like their neighbours (`p19Code`, `jsFunctionBody`, `p13Ordered`, `p13MustContain`, `p13MustMatch`, `p13MustNotContain` over `map.js` and `feed.js` text); no browser, server or database is used. Honest limit: being static they pin the decision and the call arrangement, not the runtime focus result; the scripted run is the live proof.
- Fail-before check against the pre-fix code: `git show "a6d1b9a^:web/static/js/map.js"` has `marker.openPopup()` unguarded (line 879, `isPopupOpen()` only after it at line 880), so the new `flyTo` pattern `if \(!marker\.isPopupOpen\(\)\) \{\s*marker\.openPopup\(\);\s*\}` and the amended order assertion cannot match. `git show "9531778^:web/static/js/feed.js"` has zero occurrences of `focusIsLost` and still holds `active !== document.body` (line 262), so `jsFunctionBody(..., "focusIsLost(active)")` fails and the `MustNotContain "active !== document.body"` check fails. Both tests also do not exist in the pre-fix test files (0 matches). So the tests would fail against the old code.

### Evidence

- Verifier ran the full suite once through the gate script on a fresh scratch database (`p3_ver2_gate`, created and dropped by the script): build and vet clean, 375 top level tests PASS, 0 FAIL, 0 SKIP, every package `ok`. This matches the expected 375. The gate's gofmt listing is the same three files as before (`internal/api/handlers/geocode_test.go`, `internal/mailer/resend.go`, `web/js_contract_test.go`), none in the 03-26 diff; Info only.
- Scripted browser run on the merged code, cited not re-run: tests 2 to 10 in one run, 309 PASS, 0 FAIL, 18 NEEDS A PERSON (Chromium and WebKit). T9.14 and T9.13 now pass in both engines. T9.16 passes in Chromium and is a NOTE in WebKit, the accepted browser difference recorded under Gaps in `03-UAT.md` (no code change, not a gap).
- `03-SECURITY.md` (d715a6c): `status: verified`, `threats_open: 0`. Its scope note covers the register of plans 03-01 to 03-20 and 03-25; plan 03-26 carries its own three accepted low threats (T-03-26-01 to T-03-26-03, no data, vote or permission touched), which this verification read and agrees with.

### Requirements cross-reference

TRUST-05 (`.planning/REQUIREMENTS.md:72`, traceability row 375) and TRUST-07 (line 78, row 376) are both mapped to Phase 3 and both declared in the 03-26 plan frontmatter, with earlier plans accounting for them as listed in Requirements Coverage. Plan 03-26 declares `requirements-completed: []`. No orphaned ID. Both still read Pending in REQUIREMENTS.md, which this verification did not edit; update them after the owner's human pass.

### Items that remain for a person (from 03-UAT.md, nothing new)

Pending: tests 2, 3, 4, 5, 7, 8, 9, 10 and 11. Passed: tests 1 and 6. The test 9 remainder is how the popup card looks (T9.17). The three cosmetic owner remarks under Gaps in `03-UAT.md` (underline on hover, reporter tag position, text length) still belong with the on-hold redesign decision. All scripted gaps in `03-UAT.md` are now resolved or accepted.

## Re-verification after the redesign (plans 03-21 to 03-24, 03-27, 03-28 and 03-29)

**Date:** 2026-10-10. **Scope:** third focused re-run, goal backward, against HEAD fde18f2. Plans covered: 03-27 (merge 745ebda), 03-21 (7bc0b59), 03-22 (444bff3), 03-23 (e3dea5a), 03-24 (69c25f8), 03-28 (eb21b76, follow up 25391c6) and 03-29 (28d6f3d, amendment A86). The base of this check is the 03-26 merge 2cc0158. `git diff 28d6f3d..HEAD` over `internal`, `web`, `docs` and `cmd` is empty, so the gate and the scripted run below were made on the same code as HEAD. The summaries were not taken as evidence; every claim below was read from the current files.

**Result:** status stays `human_needed`, score 9/9, 0 gaps, 0 regressions. No new person items beyond those already in `03-UAT.md`; two owner flags are noted at the end.

### The nine truths against the current code

Server code changed since 2cc0158 only in `internal/service/trustcopy.go` (wording), `internal/service/trust.go` (+16 lines, the 03-27 location rule), `internal/service/currency.go` (two comment lines), `internal/api/handlers/trust.go` (swagger example tags only), `votes.go` and `reports.go` (03-27 validation). `trustview.go`, `reliability.go`, `reporter.go`, `geo.go` and `report.go` are unchanged.

| # | Truth | Re-check | Status |
| --- | --- | --- | --- |
| 1 | Distinct place count N | `func independentCellCount` (`trust.go:120`) is still the one implementation. `TestTrustCountsDistinctPlacesNotVotes` `--- PASS` in `gate-w18.log`. Counter sentences now name places only (`trustcopy.go:35 to 59`), which states the same rule more exactly. | VERIFIED |
| 2 | Precision chosen and documented | `const voterGeohashPrecision = 7` (`trust.go:35`). `TestTrustModelDocMatchesCode` PASS in the verifier's own run of `./internal/service/` and in `gate-w18.log`. | VERIFIED |
| 3 | Fast decaying currency | `CurrencyWindowDivisor = 4`, `CurrencyPointsPerWindow = 25` (`currency.go:78,81`). `TestCurrencyScoreBoundaries` and `TestFadeStageMapping` PASS (verifier's run). | VERIFIED |
| 4 | Slow decaying reliability | `ReliabilityMinJudged = 5`, `ReliabilityWindow = 90 * 24 * time.Hour`, `ReliabilityRowCap = 30` (`reliability.go:121,126,131`), unchanged. `TestReliabilityTable` PASS (verifier's run), `TestReporterSignalFromSeededHistory` PASS (`gate-w18.log`). | VERIFIED |
| 5 | Both signals on the wire together | `TrustResponse` still carries `currency` and `reporter`; only example tags changed. `TestFeedTrustMatchesCastVoteTrust` PASS (`gate-w18.log`). | VERIFIED |
| 6 | Browser renders both from server values, both at once on every report | See the next section. The still current word is the one `createSignal` in the toggle face (`trust.js:230`); the reporter words are `createReporterWords` (`trust.js:133`), owned by the same instance (`reporter: reporter`, line 280), placed by `createStateLine` on each surface and filled by `updateTrustBlock` (`renderTrustSignal(block.reporter, ...)`, line 535). Neither needs the panel opened. Where a signal is absent this is the server's decision as before (Provisional rows get no currency word, R-03, D-18; a reporter tag of none draws nothing, A47). | VERIFIED (static); live rendering in the human items |
| 7 | 1 km rule server side, fail closed | `MaxVoteDistanceKm = 1.0`, `<= MaxVoteDistanceKm` (`geo.go:27,60`). 03-27 adds a stricter rule before it: a vote with no coordinates is refused (`if in.LocationMissing && !isReporterResolutionVote`, `trust.go:420`), so a missing location can no longer be read as the point 0,0. `TestCastVoteTooFar` PASS (`gate-w18.log`), `TestCastVoteRequiresALocationExceptForTheReportersOwnResolution` PASS (verifier's run). | VERIFIED |
| 8 | Concurrent votes never skew N | `TestTrustCountUnderConcurrentVotes` PASS (`gate-w18.log`). | VERIFIED |
| 9 | Identical for every viewer, no identifiers | `TestTrustResponseOmitsIdentifiers` PASS (`gate-w18.log`). The wording is still built only from constants and `strconv.Itoa` (`trustcopy.go`), with no viewer input. | VERIFIED |

TRUST-07 and ROADMAP Success Criterion 2: every report still shows both signals without opening anything, the still current word inside the trust block and the reporter words beside the state label. They are no longer in one signals row: on a row the state line sits above the block, and in the popup the reporter words are in the header while the still current word sits in the block under the title and meta. The roadmap wording "side by side" is therefore met as "both at once, separately labelled", which `docs/TRUST-MODEL.md` now states in those words (A75). This is recorded as an owner flag below, not a gap.

### What these plans touched, read from the current code

**D-20, one block, one builder, three surfaces (03-23).**
- `createTrustBlock(options)` (`trust.js:193`) builds the toggle, the face, the chevron and the panel, and also builds the reporter element through `createReporterWords(showScores)` (line 231) without mounting it (`TestTrustStateLineMarkup` bans `reporter.el` and `createStateLine(` inside `createTrustBlock`).
- `createStateLine(className, tag, block)` (`trust.js:308`) appends the state label, a space and `block.reporter.el`; it never sets `hidden` or text. Each consumer calls it exactly once with its own class literal: `feed.js:170` (`'report-row__stateline'`), `map.js:329` (`'popup-card__stateline'`), `activity.js:69` (`'report-row__stateline'`). The test counts one call per file.
- Consumers type no `trust-` class literal: grep of `'trust-` and `"trust-` in `feed.js`, `map.js`, `activity.js` and `votes.js` returns nothing, and `TestTrustBlockMountedOnBothSurfaces` bans it for feed and map.
- `updateTrustBlock` (`trust.js:519`): the invalid trust branch hides both `block.el` and `block.reporter.el` (line 527); a valid object sets the reporter through `renderTrustSignal`, then `block.signals.hidden = !state.currency` (line 538), so an empty signals row takes no space while the reporter words still show.
- Focus and announcements: `createReporterWords` makes a plain `span` with no `tabindex`, button or icon (`TestTrustStateLineMarkup` bans `tabindex`, `'button'`, `aria-hidden` and the icon classes in it), so the toggle stays the only focus stop of the block and the Tab order is unchanged. The chevron is `aria-hidden="true"` (`trust.js:250`). `git diff 2cc0158..HEAD -- web/static web/templates` adds no `aria-live`.

**No underline, hover fill, press, ring, forced colours, reduced motion (03-22).**
- `trust.css` declares `text-decoration` only as `none` (lines 892 and 1306) and no `text-underline-offset`. `TestTrustToggleFaceStyle` fails on any other `text-decoration` value or any of the underline longhands.
- `.trust-block__toggle:hover` is `background-color: color-mix(in srgb, var(--color-border) 50%, transparent)` (line 771); `.trust-block__toggle:active` follows it at line 777 with `var(--color-border)`, so press wins by order; `:focus-visible` has `outline-offset: -2px` (line 783), an inset ring.
- Forced colours (line 952): the toggle has a `Canvas` border at rest and `CanvasText` on hover and press, and the chevron opts out of adjustment with `CanvasText`. Reduced motion (line 937) removes the toggle transition and the chevron turn.
- The A83 empty line rule `:not([hidden]):not(:has(> :not([hidden])))` is present for both `.report-row__stateline` (line 1060) and `.popup-card__stateline` (line 1210).

**The 03-25, 03-26 and 03-27 fixes still hold.**
- 03-25: `isInOpenPopup` (`votes.js:928`) reads `data-popup-open === 'true'`, set at `map.js:449` and removed in `onPopupClose` at `map.js:588`.
- 03-26: `flyTo` guards the open with `if (!marker.isPopupOpen())` (`map.js:905`); `focusIsLost` (`feed.js:254`) is used by `focusAfterRemoval` (line 280).
- 03-27: `math.IsNaN(parsed)` refuses NaN lat, lon and radius_km (`reports.go:382`, `448`); a body with neither coordinate sets `in.LocationMissing = true` (`votes.go:157`) and one missing key is a 400 naming it; `categoryLabel` (`app.js:355`) returns the table entry for an own property, else `CATEGORY_LABELS.other`, and `map.js` and `feed.js` call it; `onMapPointerDown` (`map.js:543`) sets `closePressHadFocus`, which `onPopupClose` reads and clears (lines 582 to 585). The tests for each (listed under New contract tests and in the 03-25 and 03-26 sections) are present and PASS.

**The shorter wording and the labels with a subject (03-28, 03-29).**
- `internal/service/trustcopy.go` holds the Proposed column of `.planning/sketches/003-trust-block-redesign/README.md` "Wording proposals" for every row: the caveat `Counts places, not votes.` ends `confirmedSentence` only; `disputedSentence` carries none; the lead is `Needs 2 confirmations to count as confirmed.`; the expiry lead is `Expired, so it can no longer be confirmed.`; the still current, dispute, too early, score and New reporter sentences match the table word for word.
- The five identifiers named in the 03-28 plan (`countsCaveat`, `provisionalLeadSentence`, `expiredLeadSentence`, `needsReconfirmingSentence`, `tooEarlySentence`) all exist with their old names.
- `currencyLabel` returns `Report up to date`, `Report getting old`, `Report needs re-confirming` (`trustcopy.go:77 to 79`); `Too early to tell` is unchanged (line 80) and is used outside `currencyLabel`, as before. The vote error `This report has expired.` is unchanged (`votes.go:193`).
- The swagger examples (`docs/swagger.yaml:68` `Report up to date`, `:446`, `:477`) and `docs/TRUST-MODEL.md` (sentence table lines 103 to 105, label table line 136) carry the new text.
- No old fragment is left: grep of `Counts separate places`, `before this counts as confirmed`, `It can no longer be confirmed` and `Confirmed earlier, but` over `internal`, `web`, `docs` and `cmd` outside tests returns nothing. The only test hit is the ban list in `TestTrustModuleHasNoThresholdsOrSinks`, which forbids it in `trust.js`. The bare old labels do not appear as display text in `web/static/js`, `web/templates` or the swagger files.

**`docs/TRUST-MODEL.md` (03-24):** says that both signals show at once on every report without opening anything, the still current word in the trust block and the reporter tag beside the state label, and that Phase 8 places the words with `createStateLine`. `TestTrustModelDocMatchesCode` keeps it tied to the code.

**Phase boundary.** Every code commit since 2cc0158 is a 03-21 to 03-29 commit (`git log --no-merges 2cc0158..HEAD -- internal web docs cmd`). The three files `gofmt`'d by 03-27 (`geocode_test.go`, `mailer/resend.go`, `js_contract_test.go`) are whitespace only. Phase 4 and 3.x changes are planning text only: 04-12, 04-16, 04-18 and 04-UI-SPEC.md follow the new wording (08f5e61, 25391c6 for 03-28; 997aad2 for 03-29), 04-29-PLAN.md changes only a file name after the rename commit 9664064, and the sketch 004 and 03.2 reference edits are owner sketch commits. No Phase 3.1 or Phase 4 code exists in the diff.

### New contract tests

| Test | File | Pins | Result |
| --- | --- | --- | --- |
| `TestTrustToggleFaceMarkup` | `web/trust_face_contract_test.go:20` | face, chevron `aria-hidden`, hint (03-22) | PASS (verifier's run) |
| `TestTrustToggleFaceStyle` | `web/trust_face_contract_test.go:47` | hover fill, press after hover, inset ring, underline ban, forced colours, reduced motion (03-22) | PASS |
| `TestTrustStateLineMarkup` | `web/trust_topzone_contract_test.go:35` | `createReporterWords`, `createStateLine`, one call per surface, retired classes banned (03-23) | PASS |
| `TestTrustStateLineStyle` | `web/trust_topzone_contract_test.go:151` | state line layout, A83 empty line rule, spacing (03-23) | PASS |
| `TestPopupClosePressKeepsTheFocusAnswer` | `web/popup_contract_test.go:680` | `onMapPointerDown`, `closePressHadFocus` order, no timer (03-27) | PASS |
| `TestCategoryLabelNeverShowsRawText` | `web/popup_contract_test.go:733` | Other fallback, no raw category (03-27) | PASS |
| `TestCastVoteRequiresALocationExceptForTheReportersOwnResolution` | `internal/service/trust_test.go:1284` | location rule in `CastVote` (03-27) | PASS (verifier's run) |
| `TestVoteBodyWithoutCoordinates` | `internal/api/handlers/votes_body_test.go:100` | handler 400s (03-27) | PASS (`gate-w18.log`) |
| `TestNearbyRejectsNaNQueryValues` | `internal/api/handlers/nearby_validation_test.go:42` | NaN 400s (03-27) | PASS (`gate-w18.log`) |
| `TestTrustCopy*IsPlainText` (three) | `internal/service/trustcopy_test.go` | new sentences and labels, caveat once, character set (03-28, 03-29) | PASS (verifier's run) |

Fail-before, read with `git show <commit>^:<file>`: `a6caee1^` `trust.css` has `text-decoration: underline` (1, now 0) and `trust.js` has no `trust-block__chevron`; `e6c0724^` `trust.js` has no `createStateLine`; `072a3f4^` `trustcopy.go` has `Counts separate places`; `b2b1e1b^` has no `Report up to date`; `3e8ba86^` `reports.go` has no `IsNaN`; `d3bf109^` `trust.go` has no `LocationMissing`; `0c65f0d^` `app.js` has no `function categoryLabel`; `50092af^` `map.js` has no `closePressHadFocus`. Each new test would fail on the code before its fix.

Assertions removed or weakened (`git diff 2cc0158..HEAD` over all `*_test.go`, every removed non-comment line read):
- The `.trust-icon--user` and `.trust-signal--second::before` rows of `TestTrustCSS`, and `trust-icon trust-icon--user` and `trust-signal--second` in the `trust.js` must-contain list, went with the user glyph and second separator (A76). They are replaced by a ban: `p23RetiredClasses` must not appear in `trust.js` or the CSS.
- The underline expectations (`text-decoration: underline` on hover, `text-underline-offset: 4px`) were replaced by the fill and by an underline ban, so the pin is stronger.
- The old "both counter sentences end with the caveat" check became "the confirmed sentence ends with the caveat exactly once" plus "the disputed sentence contains no Counts", which pins A85 more tightly.
- The popup header `padding-right` moved to `.popup-card__pills` with an explicit "header must not declare padding-right" check.
- All other removed lines are expected strings replaced one for one with the new wording in `trust_e2e_test.go`, `activity_e2e_test.go`, `currency_test.go`, `trustview_test.go`, `reporter_test.go`, `feed_test.go`, `trust_paths_test.go` and `trustcopy_test.go`.
None of them leaves a must-have unpinned.

### Evidence

- Verifier's own runs on HEAD fde18f2, one command each: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./web/ -count=1 -v` ok, 141 top level PASS, 0 FAIL, 0 SKIP; `go test ./internal/service/ -count=1 -v` ok, 104 top level PASS, 0 FAIL, 0 SKIP (fakes only, no database); `gofmt -l internal cmd web docs` lists nothing (the three earlier Info files are now clean).
- Post merge gates on fresh scratch databases, cited not re-run: `gate-w13.out` 380 PASS, `gate-w14.out` 382, `gate-w15.out` 384, `gate-w16.out` 384, `gate-w17.out` and `gate-w18.out` 384 with 0 FAIL and 0 SKIP and every package `ok`. The verifier read `gate-w18.log` for the database tests named above (all `--- PASS`, 0 `--- FAIL` or `--- SKIP` lines).
- Scripted browser run on the final build, cited not re-run: `final-harness-2.log`, repo head 8210287 (same code as HEAD), tests 2 to 11, Chromium and WebKit, 442 PASS, 0 FAIL, 20 NEEDS A PERSON, 42 NOTE.
- Debt markers (TBD, FIXME, XXX) in `trust.js`, `trust.css`, `feed.js`, `map.js`, `activity.js`, `app.js`, `trustcopy.go`, `trust.go`, `currency.go`, `votes.go`, `reports.go`, `handlers/trust.go` and `docs/TRUST-MODEL.md`: none. No em or en dash was added to this report.

### Requirements cross-reference

All seven plans declare requirements in frontmatter: 03-21 `[TRUST-07, D-09, D-18, D-19, D-20]`, 03-22 `[TRUST-05, TRUST-07]`, 03-23 `[TRUST-07]`, 03-24 `[TRUST-07]`, 03-27 `[TRUST-05, TRUST-07]`, 03-28 `[TRUST-07]`, 03-29 `[TRUST-07]`; each summary has `requirements-completed: []`. TRUST-05 (`.planning/REQUIREMENTS.md:72`, row 375) is still met by truths 1, 2, 7 and 8; TRUST-07 (line 78, row 376) by truths 3 to 6, with truth 6 now resting on the state line plus the block. No orphaned ID; the D-nn IDs are decision references. Both still read Pending in REQUIREMENTS.md, which this verification did not edit.

### Items that remain for a person (from 03-UAT.md, nothing invented)

- VoiceOver: test 2 (toggle name and hint) and test 5 (announcement of the Mark resolved confirmation).
- Ask in Safari and the prompt UI: test 4.
- A real device at 200 percent text: test 7. The scripted NOTE rows show `Report needs re-confirming` taking 3 lines at 200 percent in the 320px popup and the 320px Activity row, with no clipping; whether that is acceptable is for the owner.
- Button style and real Windows High Contrast: test 8.
- How the popup card looks: test 9.
- Look and feel, the plain language check, and whether the shorter wording reads short enough: test 10, which also closes UAT Gap 3.
- `make migrate` for 00005 on the dev database and dropping `p3_uat`: test 11.

Test 3 (Escape by surface ownership) is fully proven by the script in both engines with 0 NEEDS A PERSON, but `03-UAT.md` still reads pending until the owner records it. Passed: tests 1 and 6.

Owner flags, not gaps:
- ROADMAP Success Criterion 2 says "side by side". The redesign shows both signals at once but in two places (the state line and the block). The roadmap wording could be brought in line with A75, or the owner can confirm it reads as meant.
- The step 5 frontmatter item now expects `Report up to date` (A86) where it used to say `Up to date`.

## Re-verification after plans 03-30 and 03-31 (and quick task 261010-psc)

**Date:** 2026-10-13. **Scope:** fourth focused re-run, goal backward, against HEAD 8b9c802. The base is fde18f2 (the redesign re-run above). Covered: quick task 261010-psc (merge 99ce2d8, commits 4cf8367 and 69470d6, A87), plan 03-30 (merge 4a6ae83, commits 6751799, 194c56b and ea1aee4, A88) and plan 03-31 (merge 8b9c802, commits 9d8b84f to 41ade6c, A89 to A96). `git diff 945ecea 8b9c802 -- internal web cmd docs` is empty, so the builder's final commit and HEAD hold the same code. The summaries were not taken as evidence; every claim below was read from the current files.

**Result:** status stays `human_needed`, score 9/9, 0 gaps, 0 regressions. No new person items beyond those already in `03-UAT.md`, plus three owner flags and one Info item at the end.

### The nine truths against the current code

`internal/service/trust.go`, `geo.go`, `currency.go`, `reliability.go`, `reporter.go` and `report.go` have no diff since fde18f2, so truths 1 to 4, 7 and 8 rest on unchanged server code. The server files that changed are `trustcopy.go` (words only), `trustview.go` (a lead sentence parameter and one added field), the new `trustfacts.go`, and `handlers/trust.go` (one added field, plus example tags).

| # | Truth | Re-check | Status |
| --- | --- | --- | --- |
| 1 | Distinct place count N | `func independentCellCount` (`trust.go:120`) is still the one implementation; its callers are `currency.go:207`, `currency.go:251` and `trust.go:229 to 232`. No new counting code: `trustfacts.go` reads `t.ConfirmedPlaces` and `t.DisputedPlaces` that `BuildTrust` already set. | VERIFIED |
| 2 | Precision chosen and documented | `trust.go` untouched. `docs/TRUST-MODEL.md` changed (98 lines) and `TestTrustModelDocMatchesCode` is in the cited full run, 0 FAIL. | VERIFIED |
| 3 | Fast decaying currency | `currency.go` untouched (constants at `:78` and `:81` as before). | VERIFIED |
| 4 | Slow decaying reliability | `reliability.go` and `reporter.go` untouched. | VERIFIED |
| 5 | Both signals on the wire together | `TrustResponse` still carries `currency` and `reporter` with every field of both unchanged in name and type (`handlers/trust.go`); `Facts []FactResponse` is a new last field (`json:"facts"`), always an array, never null (`factsToResponse`). `p07TrustWireKeys` gained `"facts"`, and `p14CurrencyWireKeys` and the reporter key set are not touched, so the old keys stay pinned. The three producers still go through the one `trustToResponse` (`reports.go:206`, `votes.go:222`, `auth.go:422`). `TestTrustCarriesFactsOnTheWire` compares vote response and feed whole object. | VERIFIED |
| 6 | Browser renders both from server values, both at once on every report | Feed row and Activity row: the toggle face (`trust.js:254 to 257`) still holds the counters and the still current word (`createSignal`, line 241, filled at `updateTrustBlock` 588, `signals.hidden = !state.currency` at 592); the reporter words are `createReporterWords` (line 161), placed beside the state label by `createStateLine` (line 312, one call each in `feed.js:170` and `activity.js:69`) and filled at line 590. Neither needs the panel opened. Popup: `createTrustCard` (line 614) has no toggle, no chevron and no panel, so the card is always showing; the state word is the currency label (`updateTrustCard` 674 to 678) and the currency and reporter rows are in the always open facts list (`renderFacts`, line 680). A Provisional or none case is still the server's decision (a `currency` fact exists only when the label is filled, `trustfacts.go:157`, and none gives no reporter row, `:193`). | VERIFIED (static); live rendering in the human items |
| 7 | 1 km rule server side, fail closed | `geo.go` and `trust.go` untouched. | VERIFIED |
| 8 | Concurrent votes never skew N | Code untouched; test in the cited run. | VERIFIED |
| 9 | Identical for every viewer, no identifiers | `TrustInput` has no viewer field (unchanged). `buildTrustFacts(t, vis, expiresAt, now)` (`trustfacts.go:205`) reads only the built `Trust`, the visibility and the clock. Every fact string is a constant, `strconv.Itoa` of a count, or the existing signal's own sentence (themselves from constants and counts). `TestTrustFactsAreDeterministic` (`trustfacts_test.go:424`) checks that the same input gives the same facts, regardless of vote row order. `TestTrustResponseOmitsIdentifiers` (`trust_e2e_test.go:493`) scans the whole raw feed and vote bodies, which now contain `facts`, for emails, `@`, the voter cell, `account_id`, `geohash_cell` and both session ids, and walks every key at any depth. | VERIFIED |

TRUST-07 and Success Criterion 2 still hold in the form noted in the last re-run ("both at once, separately labelled"): on a row the word and the reporter words are in the state line and the face, and in the popup both are rows of the always open card.

### What these plans touched, read from the current code

**Quick task 261010-psc (A87).** `feed.js`: `applySelectionHighlight(id)` has lost its scroll parameter and the `scrollIntoView` call; both call sites follow (`activateRow`, line 121, and the `onSelect` subscriber for `source === 'map'`). It marks the row selected and clears the others, nothing else. A list originated selection is unchanged (it still flies the map and opens the popup). No CSS, markup or `map.js` change.

**Plan 03-30 (A88).** Copy only. `trustcopy.go` sentences name people at different spots, the word place is in no sentence, `countsCaveat` is retired, and the Provisional lead follows the confirming count through `provisionalLeadSentenceFor` (`trustcopy.go:76`), with `trustview.go` passing `confirmed`. No JSON key or shape changed (`confirmed_places`, `disputed_places`, `recent_places` stay). `TestPanelWordingStaysFilled` (`trustwording_test.go:207`) pins that every line that was filled before is still filled, and `TestPanelWordingNamesNoPlace` (line 148) bans the word place.

**Plan 03-31 (A89 to A96).**
- `trustfacts.go` is the one list builder, called once at the end of `BuildTrust`. The status row is for an expired report that is not retracted; the confirmations and disputes rows are always there (`:213`); the currency row carries `c.Sentence` and `c.ScoreSentence` of the existing signal (`:160`); the reporter row carries `r.Label`, `r.Sentence` and `r.ScoreSentence` (`:196 to 199`). The only own wording for those two kinds is the title word and, for Gone quiet, a fixed detail that drops the words its title says (`:173`). The test grid compares each fact against the signal's own fields (`trustfacts_test.go:349 to 384`), so a drift fails.
- `trust.js` is 730 lines. It holds no sentence and no rule: `validFacts` (line 76) checks the closed lists, `renderFacts` (line 536) writes server titles and details with `Pinalert.setText`, and the tone glyph table (line 52) is the one region exempt from the numeric literal ban. `TestTrustModuleHasNoThresholdsOrSinks` (`trust_contract_test.go:315`) removes exactly that table by `p31GlyphTable` before scanning for any numeral from 2 up, so the exemption does not widen. The ban list on server wording in `trust.js` grew (`Counts places`, `people at different spots`, `of their last` and the rest) rather than shrank. `TestToneGlyphsAreFourShapes` pins the table's content.
- Distance is shown only when the feed was loaded around the reader's own position: `setCenter(lat, lon, own)` is called with `true` only from the geolocation success branch (`map.js:236`, the fallback branches at 239 and 244 pass `false`), and `map.js:390` asks `Pinalert.feedIsAroundReader()` before `formatDistance`. The fade line (`app.js:437`) says only "Fades in N hrs." or "Fades in N min." from `expires_at` and an expired or missing value reads nothing; it never says a confirmation extends the life, as its comment records (T-03-31-03).
- Row panels and Activity panels render the same list with `createFactsList(false)`, so no score detail line exists there (A96, D-09); the popup uses `createFactsList(true)` (A50).

### Behaviour changes worth saying plainly

- `trust.js` `validTrust` (line 111) now requires a valid `facts` array. A trust object without `facts` hides the whole block rather than inventing a zero. All three server producers send it, and `TestTrustCarriesFactsOnTheWire` and `TestTrustResponseOmitsIdentifiers` (through `p07AssertTrustKeys`) pin the key on the feed and vote responses. Info, not a gap.
- The map popup has no trust panel any more. The popup close path no longer calls `collapsePanelsIn` and its pin is now a ban (`TestPopupDialogFocusAndOpener`). The 03-25 and 03-26 signals are still in place and still pinned: `data-popup-open` is set (`map.js:472`) and removed (`map.js:611`) and `TestPopupDialogFocusAndOpener` requires the removal; `isInOpenPopup` (`votes.js:921`) and `focusIsLost` (`feed.js:254`) are not in this diff.

### New contract tests

| Test | File | Pins | Result |
| --- | --- | --- | --- |
| `TestTrustFactsEveryState`, `TestTrustFactsRulesOverTheGrid`, `TestTrustFactsPlurals`, `TestTrustFactsAreDeterministic` | `internal/service/trustfacts_test.go` | order, tones, plurals, closed character set, no place word, equality with the signals' own sentences, determinism (A90) | cited full run, all PASS |
| `TestTrustCarriesFactsOnTheWire` | `internal/api/handlers/trust_e2e_test.go` | five keys per fact, array never null, feed equals vote response, rows follow votes | cited full run, PASS |
| `TestPanelWordingNamesNoPlace`, `...StaysFilled`, `...EveryState`, `...SingularAndPlural`, `...PluralConstants` | `internal/service/trustwording_test.go` | A88 words, every filled line still filled | cited full run, PASS |
| `TestFactsListIsTheOnlySourceOfRows`, `TestToneGlyphsAreFourShapes`, `TestTrustCardIsAlwaysOpen`, `TestRowPanelHoldsTheFactsList`, `TestPopupBuildsTheCardAndTheFadeLine`, `TestSharedDistanceAndFadeFormatters`, `TestToneTokensClearTheContrastFloors`, `TestFactsCSS` | `web/trust_facts_contract_test.go` | one list builder, glyph shapes, no toggle in the card, shared distance and fade formatters, contrast floors 4.5:1 and 3:1 in all four theme sources, card and row look | verifier's own run of `./web/`, ok |
| `TestMapSelectionNeverScrollsThePage` | `web/popup_contract_test.go:689` (quick task 261010-psc) | A87 | verifier's own run of `./web/`, ok |

### Assertions removed or weakened

`git diff 89221f7..8b9c802 -- '*_test.go'` (plan 03-31) and `git diff fde18f2..89221f7 -- '*_test.go'` (plan 03-30), every removed non-comment line read. None leaves a must-have unpinned.
- 03-30: every removed line is an old expected sentence, replaced one for one by the new wording (`trust_e2e_test.go`, `currency_test.go`, `feed_test.go`, `reporter_test.go`, `trust_paths_test.go`, `trustcopy_test.go`, `trustview_test.go`). The pin "the confirmed sentence ends with the caveat exactly once" and "the disputed sentence carries no caveat" is retired on purpose because A88 retires the caveat; its replacement is the no place guard plus the plural and singular tests. The lead sentence table in `trustview_test.go` keeps the same cases (for example `retracted wins over expired`, line 323, and `provisional expiring exactly now`, line 324) with one extra input column.
- 03-31 `trust_e2e_test.go`: `"currency", "reporter"` became `"currency", "reporter", "facts"`, so the key set got stricter.
- 03-31 `reporter_test.go`: the "reporter signal changes nothing else" comparison now removes the reporter row of the facts together with the signal (`p31WithoutReporter`), because the row carries the signal; the same property is still pinned. The "Trust ends with Currency then Reporter" check became "ends with Currency, Reporter, Facts".
- 03-31 `feed_test.go`: `p12WithoutReporter` also drops the reporter fact. Same property.
- 03-31 `popup_contract_test.go`: `createTrustBlock({ showScores: true })` and `updateTrustBlock(` became `createTrustCard()` and `updateTrustCard(` (the same ordered positions, and `createTrustCard(` is banned in the update function). The A31 ban on `distance_km` in `updatePopupCard` was replaced by a must-contain of the gate `Pinalert.feedIsAroundReader()` and the formatter call, which the ban could not coexist with. `collapsePanelsIn` in the close path became a ban.
- 03-31 `trust_contract_test.go`: the `PANEL_LINES` list, the old panel line class and the sentence field reads in `renderTrustPanel` are replaced by the facts list pins (A89). The numeric literal scan still runs over all of `trust.js` except the glyph table. The old colour pin (`color` must equal `var(--color-text)` on panel lines, plus the grid gap of sentence lines) is replaced by `TestToneTokensClearTheContrastFloors` and `TestFactsCSS`, which compute contrast from the declared values.
- 03-31 `trust_topzone_contract_test.go` and `trust_face_contract_test.go`: `createReporterWords(showScores)` and its `, score` pieces are gone with the numbers on the signals (A92); the function is now pinned without options and the face test pins `createTrustBlock()` with no options. The reporter words are still plain text outside the button with no tab stop.

### Evidence

- Verifier's own runs on HEAD 8b9c802: `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l internal cmd web docs` lists nothing; `go test ./web/ -count=1` ok; `go test ./internal/service/ -count=1` ok.
- Whole Go suite on a fresh scratch database at the builder's final commit 945ecea (same code as HEAD), cited, not re-run: 403 top level PASS, 0 FAIL, 0 SKIP, every package ok, vet and gofmt clean (`gate31.txt`).
- Scripted browser run on merged main 8b9c802, Chromium and WebKit, tests 0 and 2 to 12, cited, not re-run: 508 PASS, 0 FAIL, 24 NEEDS A PERSON, 60 NOTE (`h31-main-final.txt`). The 24 are the same kinds as before plus T2.4 (VoiceOver on the popup card, once per engine) and T12.17 (the look against the picture).
- Debt markers (TBD, FIXME, XXX) across `internal`, `web`, `docs` and `cmd`: none.
- Phase boundary: the code diff since fde18f2 is 29 files, all trust copy, trust facts, the trust and popup surfaces, swagger and docs, and one comment line in `reports.go`. No Phase 3.1 or Phase 4 code. Phase 4 changes are planning text only (04-12, 04-16, 04-18 and 04-UI-SPEC.md, labelled).

### Requirements cross-reference

TRUST-05 (`.planning/REQUIREMENTS.md:72`, traceability row 375) is met by truths 1, 2, 7 and 8, which rest on unchanged code. TRUST-07 (line 78, row 376) is met by truths 3 to 6, with truth 6 now resting on the face and state line on rows and the always open card in the popup. No orphaned ID. Both still read Pending in REQUIREMENTS.md, which this verification did not edit.

### Items that remain for a person (from 03-UAT.md, nothing invented)

VoiceOver on the toggle and on the Mark resolved confirmation; VoiceOver on the popup card (T2.4); Ask in Safari and the prompt UI; a real device at 200 percent text; button style and real Windows High Contrast; how the card looks against the owner's picture (T12.17); the plain language check, including whether the shorter people wording reads short enough; `make migrate` for 00005 on the dev database and dropping `p3_uat`. Tests 1 and 6 passed earlier.

### Owner flags, not gaps

- The popup still shows two score lines ("Score S out of 100.", the `score_detail` rows) that the owner's chosen picture does not show. That is A50 and D-09 kept on purpose by decision, and TRUST-07 says "score", so the lines stay unless the owner decides otherwise.
- The popup has no panel any more, so the wording of human items 2 and 3 in the frontmatter ("popup panel closes before a row panel") no longer describes a popup behaviour; only the row panel and the popup close remain. Worth a one line edit of those items by the owner.
- The fade number is computed in the browser (`app.js:437`, `Math.floor` over the server's `expires_at` and the server clock), the same class as `relativeTime` and `ageStage`. No trust rule lives there, but the line is not authored by the server like the facts are.

### Info only

- `needsDetail` (`trustfacts.go:92`) repeats the threshold subtraction of `provisionalLeadSentenceFor` (`trustcopy.go:76`), both from `IndependentAgreementThreshold`, and `raisedBar` (`trustfacts.go:143`) reads a raised bar by comparing sentences. Both are pinned by the facts grid test, so a drift fails, but they are the two places a later change to the threshold wording has to touch together.
- Phase 4 candidate, not a Phase 3 gap: `POST /api/reports` accepts a null or absent latitude and longitude and files the report at 0,0. `SubmitReportRequest` declares `Latitude` and `Longitude` as plain `float64` (`internal/api/handlers/reports.go:50` and `:52`), `SubmitReport` copies them straight into `SubmitInput` (`:276` and `:277`), and the service only range checks them (`internal/service/report.go:207`), so 0 passes. The vote endpoint answers "latitude is required." through pointer fields (`internal/api/handlers/votes.go:158` to `:163`). A pointer field with the same 400 in the report handler would close it.

---

_Verified: 2026-10-08_
_Re-verified: 2026-10-10 after plan 03-25_
_Re-verified: 2026-10-10 after plan 03-26_
_Re-verified: 2026-10-10 after the redesign (plans 03-21 to 03-24, 03-27, 03-28 and 03-29)_
_Re-verified: 2026-10-13 after plans 03-30 and 03-31 and quick task 261010-psc_
_Verifier: Claude (gsd-verifier)_
