---
phase: 03-trust-model-hardening-diversity-weighted-trust
verified: 2026-10-08T00:00:00Z
status: human_needed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: false
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
**Re-verification:** No, initial verification

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

---

_Verified: 2026-10-08_
_Verifier: Claude (gsd-verifier)_
