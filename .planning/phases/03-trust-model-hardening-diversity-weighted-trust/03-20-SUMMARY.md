---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 20
subsystem: docs
tags: [trust-model, documentation, self-checking-doc, phase-gate, uat-script]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "plans 03-01 to 03-19: the 1 km rule, counters, trust object, still current and reporter signals, hint, popup and focus work"
provides:
  - "docs/TRUST-MODEL.md covering every rule from vote to screen, with a 15 row constants table"
  - "TestTrustModelDocMatchesCode, which fails the build when the document and the code drift"
  - "README summary of the trust model and a throwaway database warning"
  - "A40 and A32 pointers from the Phase 1 and 2 UI contracts and the sketch reviews to 03-UI-SPEC.md"
  - "A green phase gate on p3_0320 and the owner V-42 script for a fresh p3_uat database"
affects: []
tech-stack:
  added: []
  patterns:
    - "Document checked against code by a package service test (p20 helpers): headings, constants table, identifiers, refusal mapping, named tests, plain text"
key-files:
  created:
    - internal/service/trust_model_doc_test.go
  modified:
    - docs/TRUST-MODEL.md
    - README.md
    - .planning/phases/01-foundation-report-map/01-UI-SPEC.md
    - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md
    - .claude/skills/sketch-findings-pinalert/sources/REVIEWS.md
key-decisions:
  - "The document names every test by its full func name (no -run prefixes), so the named tests check can resolve each token to a real func."
  - "The new test walks the repository skipping dot directories, so an execute-phase worktree under .claude cannot hide a missing test."
  - "No spoof resistance wording anywhere; the test lower cases the whole document and refuses ten listed phrases (T-03-03 stays accepted and documented)."
requirements-completed: [TRUST-05, TRUST-07, D-03, D-08]
status: complete
duration: 40min
completed: 2026-10-08
---

# Phase 3 Plan 20: Trust model document, phase gate and V-42 script Summary

docs/TRUST-MODEL.md now explains the trust model end to end and a Go test (`TestTrustModelDocMatchesCode`) proves its constants, names, refusal sentence and cited tests equal the code; the earlier UI contracts point at the Phase 3 contract, generated code matches its sources, and the full suite is green on a disposable database.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 (RED) | Failing test that keeps the trust model document true | 00ceeed |
| 1 (GREEN) | Document the trust model end to end, README summary | 1584624 |
| 2 | A40 and A32 pointers in the Phase 1 and 2 contracts and REVIEWS.md | bb1e211 |
| 3 | Phase gate (regeneration, vet, full suite, race run) and the V-42 script | no commit (no regeneration diff; the script is below) |

## What was built

- `internal/service/trust_model_doc_test.go` (package service, standard library only, every helper starts with `p20`): subtests "headings in order", "constants table", "named identifiers", "too far refusal", "named tests exist" and "plain text". The dash check builds the two code points with `rune(0x2013)` and `rune(0x2014)` so the file itself holds none.
- `docs/TRUST-MODEL.md`: six new second level sections (`## The two counters`, `## One trust object on every surface`, `## Browser hints, server decisions`, `## Where the roadmap words are met`, `## Hand offs to later phases`, `## Keeping this document true`) in the required order, a 15 row constants table, three new Honest limits bullets (GPS error, 30 second poll, advisory hint) and seven new Out of scope items. No existing heading was renamed, removed or reordered.
- `README.md`: three summary bullets directly under the 03-02 pointer line and one throwaway database bullet under Running tests.
- 01-UI-SPEC.md (one A40 line), 02-UI-SPEC.md (A40, A13/A18/A72 and A12/A16 lines) and REVIEWS.md (Status: historical line and the Build checklist pointer). Inserted lines only.

## Verification results

- RED: before the doc edits `go test ./internal/service/ -run TestTrustModelDocMatchesCode -count=1 -v` failed on the six missing headings, the missing constants section, six unnamed identifiers, the missing `TooFarMessage()` text and the prefix tokens `TestCurrency`, `TestReliability` and `TestReporterSignals`.
- GREEN: `--- PASS: TestTrustModelDocMatchesCode (0.05s)` with all six subtests passing; `go test ./internal/service/ -count=1` printed `ok  	pinalert/internal/service`.
- Dash counts after the edits (`LC_ALL=C grep -o`): README.md 21 em and 0 en, 01-UI-SPEC.md 46 and 2, 02-UI-SPEC.md 84 and 0, REVIEWS.md 0 and 0, docs/TRUST-MODEL.md 0 and 0, the new test file 0 and 0. These equal the measured baselines taken before any edit (they also equal the plan's numbers). `git diff --numstat` showed 0 deleted lines for the three contract files (4, 2 and 6 added).
- `gofmt -l internal/service/trust_model_doc_test.go` printed nothing.
- Regeneration: `make sqlc` and `make swag` ran; `git status --porcelain -- internal/store/sqlc docs/docs.go docs/swagger.json docs/swagger.yaml` printed nothing, and `docs/TRUST-MODEL.md` exists. No deviation commit was needed.
- `go vet ./...` printed nothing (pass).
- Quick suite: `ok  	pinalert/internal/service`, `ok  	pinalert/web`.
- Full suite on the freshly created p3_0320 with `-v -p 1`: `full-suite rc=0`; 0 lines matching `^(--- FAIL|FAIL)`, 0 lines matching `--- SKIP`, 1228 `--- PASS` lines, every package `ok` (api, handlers, auth, geocode, mailer, ratelimit, service, session, store, testutil, web). `TestTrustCountUnderConcurrentVotes`, `TestReporterSignalFromSeededHistory`, `TestCastVoteTooFar` and `TestTrustModelDocMatchesCode` each printed `--- PASS`.
- Race run on a second fresh p3_0320: `--- PASS: TestTrustCountUnderConcurrentVotes (0.35s)` and `ok  	pinalert/internal/api/handlers 2.444s` under `-race` (V-41 phase gate).
- `psql -l` count for p3_0320 afterwards: 0. Only p3_0320 was used, no other database was touched, no server was started, no port was used.

## Deviations from Plan

1. [Rule 3 - Blocked command] The sandbox refused one compound git status command and would refuse the plan's long verify one-liners, so every verify was split into plain commands (go test, gofmt, grep and perl count gates, make, vet, the create/test/drop wrapper). The same checks were run; nothing was skipped. `DATABASE_URL` printed `unset`, so `go test` ran without the `env -u` prefix.
2. [Tooling] The race run was piped through `tail`, so its printed `rc` belongs to `tail`. The test output itself shows `PASS` and `ok` under `-race`, and the full suite (not piped) printed `rc=0`.
3. [Plan correction by the orchestrator, rule 12a] The plan's owner reminder said Phase 2 verification was pending. It is closed (see the reminders below), so the script says so.
4. [Plan correction by the orchestrator, rule 12b] The V-42 script folds in the deferred checks of plans 03-04, 03-09, 03-11, 03-13, 03-15, 03-16, 03-17, 03-18 and 03-19, each tagged with its source plan, and uses the single database name p3_uat on PORT=8792.
5. [Rule 1 - doc must match the test] Four small doc additions beyond the plan's list so the named identifiers and tests checks could pass: a sentence naming `TooFarMessage` and where it is written, `TestHaversine` added to the 1 km test list, `TestReporterSignalFromSeededHistory` added to the reporter history sentence, and the two `-run` regexes rewritten to full test names (plan step 9).
6. The 01-UI-SPEC.md A40 line ends with "and wins over the list above" (a few extra words in the inserted line only).

No constant or identifier was found in a different file than the plan listed; all 15 constants, `ErrVoterTooFar` and the ten functions are declared where the plan said. The votes.go branch matches the plan's `writeFieldError(w, http.StatusForbidden, "location", service.TooFarMessage())` shape. The freshness values in votes.js are 60000 ms (`VOTER_LOCATION_FRESH_MS`) and 120000 ms (`VOTER_LOCATION_HINT_MS`), written into the doc as 60 and 120 seconds.

## Known Stubs

None.

## Threat Flags

None. T-03-03 stays accepted: the doc adds the GPS error, 30 second poll and advisory hint limits, and the test fails if the doc ever gains a spoof resistance phrase.

## TDD Gate Compliance

The RED commit (00ceeed, test only, failing) precedes the GREEN commit (1584624, document and README). No refactor commit.

## For the owner: V-42 script

Run this yourself at the end of the phase. It uses a fresh disposable database and fresh accounts. Never use the dev server database, port 8080 or port 8090. The executor never created p3_uat, never started the server and never ran make migrate.

### Setup

1. Create the database: `dropdb --if-exists p3_uat; createdb p3_uat`. In that terminal set `DATABASE_URL` to `postgres://localhost:5432/p3_uat?sslmode=disable`, then run `make migrate`.
2. In the same terminal start the server with `ENV=development`, `PORT=8792` and `BASE_URL=http://127.0.0.1:8792`, with `RESEND_API_KEY` unset so the development mailer prints magic links to the log, then `make run`. Open http://127.0.0.1:8792. The server listens on every interface at port 8792 (it binds ":" plus the port), so use it on a trusted network only.
3. Use three separate Chrome profiles, one per account: A (reporter), B and C (voters). Sign each in with a fresh email; the magic link is printed in the server log.
4. Locations come from Chrome DevTools, Sensors, Location override. Report point R is 12.9716, 77.5946. Voter B votes from R itself; voter C from 12.9686, 77.5946 (0.3336 km south, a different place). Boundary pair: 12.9805, 77.5946 (0.9896 km north) is accepted and 12.9806, 77.5946 (1.0008 km north) is refused. 12.9900, 77.5946 (about 2 km) is clearly far. For the 03-04 walk, 12.9896, 77.5946 is also about 2 km and 12.9746, 77.5946 is 330 m.

### What you can see live and what tests prove instead

Seen live on a fresh database: the two counters at 0, 1 and 2; the Unconfirmed lead sentence; Up to date with its popup score after 2 places confirm; Too early to tell on a critical or rescue needed report with fewer than 2 places; New reporter with "None of their reports has had a clear result yet."; the panel; the hint and the refusal; Reopen on the Activity page. (wording updated by plan 03-28: the New reporter line now reads "A rating needs 5 reports with a clear result. They have none so far." and the Unconfirmed lead sentence reads "Needs 2 confirmations to count as confirmed.") (labels updated by plan 03-29: the still current word now reads "Report up to date"; "Too early to tell" is unchanged.)

Proven by named tests because they need hours or history: Getting old and Needs re-confirming (`TestCurrencyScoreBoundaries`, `TestFadeStageMapping`), Reliable reporter, Mixed record and Unreliable reporter (`TestReporterSignalFromSeededHistory`, `TestReliabilityTable`), Expired Activity rows (`TestProfileMarksExpiredReports`). (labels updated by plan 03-29: these two now read "Report getting old" and "Report needs re-confirming".)

### Checks

5. Trust counters and words. As A, post a low severity report at R. As B, then C, confirm it. Watch the row and popup go from 0 confirmed to 1 to 2, the lead sentence disappear at 2, and Up to date appear with the popup score. Post a critical report as A and confirm it once as B: it reads Too early to tell, with no extra dimming. (label updated by plan 03-29: the word now reads "Report up to date", followed by the popup score.)
6. Panel (A5; also 03-13). It opens by click, tap, Enter and Space, and closes by Escape, on a feed row and in a popup. A screen reader (VoiceOver) reads the toggle name followed by the hint ". Shows what these mean.".
7. Escape by surface ownership (A70, 03-19). Focus on Confirm with the popup panel open: the first Escape closes the panel and focuses the toggle, the second closes the popup. With the panel open, repeat with focus on `.popup-card`, on the popup close button and on the marker. With a row panel open, focus on Confirm or on the row. With a popup panel and a row panel open at once and focus in the popup, Escape closes the popup panel first.
8. Too far (A8, D-01; the hint walk from 03-18). Only a vote writes the location cache (the map and the post form take fixes but never write it), and the hint only reads that cache, so the buttons never show as off before a vote from far. A later vote reuses the cached fix, so clear the site's sessionStorage for 127.0.0.1:8792 (DevTools, Application) and reload after every Sensors change. Post a fresh report as A at R for this step.
   - (a) As C, set Sensors to 12.9900, 77.5946, clear sessionStorage and reload.
   - (b) Press Confirm once: the vote error shows "Too far to vote. You need to be within 1 km of this report." and the far fix is now cached.
   - (c) Within one 30 second poll, Confirm, Dispute and Mark resolved on that report show as off, stay reachable by Tab, do nothing on click, Enter or Space, and show that sentence under them.
   - (d) Force a vote from the DevTools console of C's profile: POST to `/api/reports/{id}/confirm` with a JSON body of latitude 12.9900 and longitude 77.5946. Expect HTTP 403 with field `location` and the same sentence.
   - (e) Boundary pair on that report, clearing sessionStorage and reloading before each vote: from 12.9806 the vote is refused, from 12.9805 it goes through. As A, the own Mark resolved stays enabled from far.
   - (f) From 03-18: wait more than 120 seconds plus one poll and the sentence, the earlier vote error line and the marks clear. Known limit: a far hint can hold for up to the 120 second bound plus one 30 second poll. A tap on a marked button sends no vote but takes one fresh fix silently (WR-03) and recomputes the mark, so once the location is near the report one tap clears the hint and the vote goes through with that fix. While the location is still far, the tap changes nothing on screen. Then set the location near the report and vote: it goes through and no hint returns. The reporter's own row never shows the sentence.
9. Silent refresh walk (from 03-04). Profile A: Sensors at R, allow location, sign in, post a report. Profile B: Sensors at R, allow location ("Allow on every visit"), sign in. Change B's Sensors to 12.9896, 77.5946 (2 km) and tap Confirm on A's report. Expect no prompt, two POSTs in the Network panel (both 403), and the too far sentence under the buttons. Within 60 seconds change Sensors to 12.9746, 77.5946 (330 m) and tap Confirm again: no prompt, one 403 (the cached far fix) then one 200 (the silent fresh fix), and Confirm shown pressed after the refetch. In B's site settings set Location to Ask, keep the tab, wait over 60 seconds and tap Dispute: no prompt, the cached fix is reused. Set Location to Block and tap Confirm: the locked sentence "Voting needs your location, so nearby confirmations can be verified as independent. Turn on location access for this site and try again." shows and no POST goes out.
10. Mark resolved confirmation and focus (from 03-09). Feed row, keyboard only: Tab to a report you did not post, Tab to Mark resolved, press Enter. Focus lands on Cancel, and a screen reader announces the group "Mark this report resolved?" before Cancel. Press Enter on Cancel: focus is back on Mark resolved. Open the confirmation again and press Yes from a voter whose resolve stays pending: after the toast and refetch, focus is on that row's Mark resolved, not the body (DevTools console: `document.activeElement`). As the reporter, resolve your own report from a feed row in list view at phone width: the row leaves, and focus is on the row that took its place (or the previous row, or the view toggle when it was the last row). Repeat from the map popup at desktop width: focus lands on the map container.
11. Reopen (R-08; from 03-15). One profile is enough. Sign in, post a report, then use Mark resolved on it from the feed row. Open Activity: the report shows its severity word before the title, the Resolved label, the two counters (0 confirmed, 0 disputed) and the reporter tag only when the server sends one, and no numbers. Press the block's button and the panel opens with the server sentences. With the panel open, press Reopen: the Reopen control disappears, the Resolved label goes, the panel stays open with refreshed sentences without a reload, and keyboard focus sits on the trust toggle (not the page body).
12. Width (A22, A55; 03-16). At 320px width, the counters, the signals and a report with a long unbroken title or a long unbroken URL in the description wrap inside the 236px feed body and the 188px Activity body with no horizontal scroll. Every row shows Critical, Medium or Low before the title, and a screen reader reads the severity word before the title. At 200 percent text size the read only labels grow and stay readable and unclipped.
13. Vote layout (A17, A28; 03-11). At a 600px row and in the 900px two pane layout, Confirm and Dispute are each about half the width and Mark resolved is full width on its own line. At 320px the Activity page Reopen is full width with its glyph, and its word wraps to two lines inside the button rather than clipping.
14. Vote button look (A13, A46, RR-7; 03-11). A pressed Confirm reads as the inverted fill in light and dark, and stays so while a vote is in flight. With DevTools Rendering set to emulate forced-colors active: every vote glyph is visible, the pressed vote shows the Highlight fill, a disabled button reads GrayText, Yes has a visible border, and a forced vote error shows its alert glyph. With reduced motion emulated, hovering a vote button changes colour with no transition.
15. Popup card (03-17). Open a pin, open the trust panel and the Mark resolved confirmation, wait past one 30 second poll, and confirm both stay open and focus has not moved. Confirm the card is neutral with the left severity bar, the 20px header glyph and the X close button in the top right, in light and dark themes, at 320 and 360px widths, and in forced colours.
16. Popup position (A59, A60, A65; 03-19). In list view at 320px and 360px, tap a row whose pin is near the left or right edge; after switching to the map the popup tip is centred on the pin and the card is fully inside the view. At 320x568 the close button and the trust toggle are never covered by the account trigger, the FAB, the theme toggle or the view toggle after autopan. At 320x256 the card scrolls inside itself and close and Escape still work.
17. Reduced motion (A68; 03-19). With prefers-reduced-motion set in DevTools Rendering, at 320px and 360px open the confirmation or the panel on a pin near the top edge: no animated pan, controls clear. Rotate across 900px with a popup open: the popup stays placed.
18. Keyboard and focus (03-19). Pins have names a screen reader reads (A45) and open on Enter and on Space, and Space also works on the popup close button (A49). From a list row at phone width, opening the pin lands focus on the popup card (A51). A keyboard only Mark resolved on a feed row and in list view on a phone never leaves focus on the body (A52). While B has focus on a later row's trust toggle, have A mark one of A's own earlier reports resolved (instant for the reporter), and have A post a new report nearer the map centre, so the next polls remove and reorder rows: focus on the later row's trust toggle survives both (A67, A71). A poll removal while focus is inside the popup leaves focus off the body (A67). At 900px and above, a row opened by Enter and closed by one Escape leaves focus on that row, and a popup opened from its pin returns focus to the marker on close (A74). Closing a popup returns focus to the row that opened it, or to the marker when opened from the pin.
19. Look and feel. The trust toggle press fill shows on a Provisional row (A48). Ask someone who has not seen the app to find out what the numbers mean; record whether they find the panel (open item 2). Check forced colours (RR-7) and both light and dark themes across the page.
20. Finish: stop the server and run `dropdb p3_uat`.

### Reminders for the owner

- Your own dev database needs `make migrate` once after this phase merges, for migration 00005, which creates `idx_reports_session_expires`. The executor never touched that database. The queries work without the index, only slower.
- Phase 2 is closed and needs no action (02-VERIFICATION.md passed on 2026-10-07, 02-SECURITY.md verified with threats_open 0, 02-UAT.md 12 passed and 0 issues).
- Record any failure from this script as a gap for `/gsd-plan-phase 3 --gaps`.

## Self-Check: PASSED

Files exist: internal/service/trust_model_doc_test.go, docs/TRUST-MODEL.md, README.md, the two UI-SPEC files, REVIEWS.md. Commits 00ceeed, 1584624 and bb1e211 exist on the worktree branch. STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. p3_0320 was dropped.
