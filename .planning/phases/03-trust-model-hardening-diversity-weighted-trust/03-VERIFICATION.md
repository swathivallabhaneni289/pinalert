---
phase: 03-trust-model-hardening-diversity-weighted-trust
verified: 2026-10-08T00:00:00Z
reverified: 2026-10-10T00:00:00Z
reverified_again: 2026-10-10T00:00:00Z
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
  note: "Focused re-runs after gap closure plans 03-25 (merged in 584cef3) and 03-26 (merged in 2cc0158). See Re-verification after 03-25 and Re-verification after 03-26."
gaps: []
deferred: []
human_verification:
  - test: "Trust counters and words (V-42 step 5). As A post a low severity report at R (12.9716, 77.5946), confirm as B then C (C from 12.9686, 77.5946). Post a critical report and confirm it once as B."
    expected: "Row and popup go 0, 1, 2 confirmed. The lead sentence disappears at 2 and Up to date appears with the popup score. The critical report with one place reads Too early to tell with no extra dimming."
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
  - test: "Look and feel and plain language check (step 19). Check the toggle press fill on a Provisional row, forced colors, both themes. Ask someone who has not seen the app what the numbers mean."
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
**Re-verification:** Initial verification 2026-10-08; focused re-verifications after plans 03-25 and 03-26 on 2026-10-10 (see the sections near the end)

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

---

_Verified: 2026-10-08_
_Re-verified: 2026-10-10 after plan 03-25_
_Re-verified: 2026-10-10 after plan 03-26_
_Verifier: Claude (gsd-verifier)_
