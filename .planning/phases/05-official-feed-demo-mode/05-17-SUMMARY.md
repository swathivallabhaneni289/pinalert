---
phase: 05-official-feed-demo-mode
plan: 17
subsystem: demo-practice-flow
tags: [demo, practice-report, trust, polling, report-form, toast, ended-visit]
status: complete

requires:
  - phase: 05-13
    provides: the Demo's Nearby page, the fixed pretend position seam, the API base seam, demo.js, the Demo post window
  - phase: 05-10
    provides: the Demo API (POST reports, vote words, the ended visit 404 with field visit)
  - phase: 05-06
    provides: the world, the pretend neighbours (NeighbourInterval 4 s) and ApplyDue
provides:
  - the practice flow in demo.js (onPosted, watchPractice, stopWatching, announceConfirmed, onEndedVisit)
  - the pinalert:report-posted event (app.js, REPORT_POSTED_EVENT) and the window.PinalertDemoEnded hook
  - a Demo flag on the page view models, read by the shared report form
  - a Demo post window without address search and location notice
affects: [05-19]

key-files:
  created:
    - internal/api/handlers/demo_practice_e2e_test.go
    - web/demo_practice_test.mjs
  modified:
    - web/static/js/demo.js
    - web/static/js/app.js
    - web/static/js/report-form.js
    - web/static/js/modal.js
    - web/templates/demo.html.tmpl
    - web/templates/report_form.html.tmpl
    - internal/api/handlers/page.go
    - internal/api/handlers/low.go
    - internal/api/handlers/demo_page.go
    - web/demo_contract_test.go
    - web/report_form_contract_test.go

key-decisions:
  - "The page types no trust sentence. The step words come from the report's trust object (lead_sentence while below Confirmed) and the announcement is the practice prefix plus trust.confirmed_sentence. Confirmed is read from confirmed_by_people, the server's own flag, not from visibility, because Critical is shown at once but not confirmed."
  - "The toast and the open wait one tick after the post event, so the post window has closed first. The toast text lives in demo.js; report-form.js skips its own Report posted toast on the Demo page."
  - "The poll gives up after 40 ticks (about two minutes) so a dropped visit or a filtered list cannot poll for ever."

requirements-completed: []
duration: about 2 hours
completed: 2026-10-29
---

# Phase 5 Plan 17: The Demo's practice report and the climb to Confirmed Summary

A Demo visitor posts a practice report with the real Post a report window, sees the toast and the report open on its trust block, and watches the server's own words climb from Needs 2 people to Confirmed while the page polls every 3 seconds; nothing is saved, and the Demo's form has no Call 112 box, no picks and no address search.

## What was built

- `web/static/js/demo.js`: `PRACTICE_POLL_MS = 3000`; `onPosted(report)` (after one tick: the toast "Practice report posted. Pretend neighbours will confirm it in a few seconds.", then `PinalertMap.openReport(id)` with `Pinalert.openReport(id)` as the fallback when the report has no pin) and starts `watchPractice(id)`; `watchPractice` subscribes to the feed state and calls `Pinalert.fetchReports()` every 3 seconds; `announceConfirmed(sentence)` writes "Practice report confirmed. " plus the server's `trust.confirmed_sentence` to `#feed-status` once, then `stopWatching()` clears the interval and unsubscribes; `onEndedVisit()` sends the page to `/demo`, keeping `?from=welcome` and no other value. All of it runs only when `window.PinalertDemoPage === true`; the existing strip code still loads alone with no `window.Pinalert`.
- `web/static/js/app.js`: `REPORT_POSTED_EVENT = 'pinalert:report-posted'`, raised on the document with `{report}` after the refetch that follows the write (so the pin exists); `noteDemoEnded` calls `window.PinalertDemoEnded` for a 404 whose error field is `visit`, in `submitReport` (body already parsed) and in `fetchReports` (body read for that on the Demo page only).
- `web/static/js/report-form.js`: stand-in detached elements when the address search elements are missing (so the search code needs no check at each use and draws nothing); no second "Report posted." toast on the Demo page.
- `web/static/js/modal.js`: the location notice is optional; the near line reads `data-pretend-place` first, then `data-fixed-place`.
- `web/templates/demo.html.tmpl`: the post window has no `#location-search` field, results, status, wrapper or `#location-notice`; `#app-shell` carries `data-pretend-place`.
- `web/templates/report_form.html.tmpl`: the Call 112 box and the picks sit under `{{if not .Demo}}`.
- `internal/api/handlers/page.go`, `low.go`: `Demo bool` (always false); `demo_page.go`: `Demo: true`.

## TDD record

RED (commit 261c4834), run before any page code: `web/demo_practice_test.mjs` 10 of 13 failed (3 passed: the real page and the refused post cases, which pin that nothing happens); `TestDemoPostWindowHasNoSearch`, `TestDemoPracticeFlowSource`, `TestDemoFormGuardSource` and `TestReportFormToleratesNoSearch` failed. `TestDemoPracticeClimbOverHTTP`, `TestDemoFormHasNoCallBoxOrPicks` and `TestRealFormsKeepCallBoxAndPicks` passed at once: they pin what 05-10 and 05-13 already built (the server climb, the Chrome.DemoPage guard, the unchanged real pages), as 05-13 did for its own pins. GREEN: commit f7155977.

## Verification

Full suite on p5_17 (`go test ./... -p 1 -count=1 -v`): 5091 `--- PASS`, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed), 1 `--- FAIL`: the known flake TestVotedReportsByAccountUsesTheAccountIndex (internal/store). Rerun by name once on the same database it failed again (it depends on planner statistics); p5_17 was then dropped and created fresh, the whole `internal/store` package rerun on it, and the test passed (`ok pinalert/internal/store`). Every other package `ok`. The named tests passed: TestDemoPracticeClimbOverHTTP, TestDemoFormHasNoCallBoxOrPicks, TestRealFormsKeepCallBoxAndPicks, TestDemoPostWindowHasNoSearch, TestDemoPracticeFlowSource, TestDemoFormGuardSource, TestReportFormToleratesNoSearch, and the 05-13 and 05-10 pins TestDemoPageWritesNothing and TestDemoWritesNothing. `node --test web/*.mjs`: 1003 tests, 1003 pass, 0 fail (13 are new). `go vet ./...` clean. `gofmt -l internal/api/handlers web` lists only internal/api/handlers/place_test.go (already listed before; I did not run it over internal/geocode). No new en or em dash in any added line. p5_17 dropped (`psql -lqt` shows none); port 8798 free.

## Browser walk (Playwright, 8 runs, fresh context and no cookie each run)

Scratch server on 8798 (see Process notes). The walk opens `/demo?from=welcome`, opens Post a report, checks the window, picks Critical (checking for a Call 112 box or picks) then Medium, Flood, types a description, posts, and samples the page every 250 ms for about 10 seconds. A geolocation spy counted 0 calls in every run; 0 requests to /api/geocode; 0 page errors. Times are from the click on Post report. After all eight runs `sessions` and `reports` in p5_17 both held 0 rows.

| Size, theme, engine | Near line | No Call 112, picks, search, notice, tel | Critical: no box, no picks, no tel | Toast and opens on trust block (Demo tag) | Trust: Needs 2 people | 1 confirmed: Needs 1 more person | Confirmed by 2 people, row live | #feed-status announcement | Reload |
|---|---|---|---|---|---|---|---|---|---|
| 390 light chromium | Near Singh Nagar. Drag the pin or tap the map to move it. | yes | yes | 316 ms | 316 ms | 6244 ms | 9290 ms, row Not confirmed yet to live at the same sample | once, 9053 ms | no practice report, 18 rows |
| 390 dark chromium | same | yes | yes | 326 ms | 326 ms | 6274 ms | 9064 ms, same sample | once, 9053 ms | fresh, 18 rows |
| 1470 light chromium | same | yes | yes | 300 ms | 300 ms | 6322 ms | 9128 ms, same sample | once, 9057 ms | fresh, 18 rows |
| 1470 dark chromium | same | yes | yes | 294 ms | 294 ms | 6197 ms | 9060 ms, same sample | once, 9053 ms | fresh, 18 rows |
| 390 light webkit | same | yes | yes | 324 ms | 324 ms | 6180 ms | 9236 ms, same sample | once, 9086 ms | fresh, 18 rows |
| 390 dark webkit | same | yes | yes | 298 ms | 298 ms | 6150 ms | 9203 ms, same sample | once, 9063 ms | fresh, 18 rows |
| 1470 light webkit | same | yes | yes | 329 ms | 329 ms | 6177 ms | 9231 ms, same sample | once, 9096 ms | fresh, 18 rows |
| 1470 dark webkit | same | yes | yes | 303 ms | 303 ms | 6148 ms | 9203 ms, same sample | once, 9075 ms | fresh, 18 rows |

The announcement in every run read "Practice report confirmed. " followed by the server's sentence, which the page showed as "Confirmed by 2 people at different spots." The window closed and the toast "Practice report posted. Pretend neighbours will confirm it in a few seconds." showed at the same sample as the report opening (a popup element on both sizes: the laptop popup, and the phone sheet that the same popup becomes). The step "Needs 1 more person at a different spot." and the Confirmed look were read from the page, not typed by the test.

Timing differs from the plan's "about 4 seconds" and "about 8 seconds": the pretend neighbours' votes fall due at 4 s and 8 s on the server (NeighbourInterval), and a 3 second poll sees them at the next tick, so the visitor sees the second step at about 6 s and Confirmed at about 9 s. The orchestrator may want the poll to be shorter, or the neighbour interval shorter; both are one constant.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The toast is in report-form.js, not app.js**
- **Found during:** orientation. The success toast `Pinalert.showToast('Report posted.')` is in `report-form.js` (and its EARLIER path), not in `submitReport`.
- **Fix:** demo.js shows the practice toast (so the node case can pin it) one tick after the post event, together with the open; `report-form.js` skips its own "Report posted." and "Your earlier report was posted." toasts when `window.PinalertDemoPage === true` and the report was not held. A held report keeps its sentence.
- **Commit:** f7155977

**2. [Rule 3 - Blocking] modal.js is outside the plan's file list**
- **Found during:** orientation. `initLocation` runs `locationNotice.hidden = true` before the fixed position branch, so removing `#location-notice` from the Demo would have thrown and left the window with no map pin. The fixed position and the "Near Singh Nagar." line were already built there by 05-13, not in report-form.js as the plan assumed.
- **Fix:** null guards on the notice in `initLocation` and `showLocationDenied`; the near line reads `data-pretend-place` first (the plan's attribute), `data-fixed-place` second. report-form.js got no second reader of the fixed position.
- **Commit:** f7155977

**3. [Rule 3] A Demo flag beside Chrome.DemoPage**
- 05-13 already guarded the Call 112 box and the picks with `{{if not .Chrome.DemoPage}}`. The plan asks for `{{if not .Demo}}` and a `Demo bool` on the view models, so I switched the guard (still one `if` and one `end`, so the count pinned in `report_form_contract_test.go` is unchanged; a dated note is in that file). `Chrome.DemoPage` is still set by `demo_page.go` but nothing reads it now. Left in place to keep the change small; a later clean up can remove it.

**4. [Rule 2 - rendered tests live in the handlers package]**
- `TestDemoFormHasNoCallBoxOrPicks`, `TestRealFormsKeepCallBoxAndPicks` and `TestDemoPostWindowHasNoSearch` render the real handlers, so they sit in `internal/api/handlers/demo_practice_e2e_test.go` (the `web` package cannot import handlers without a cycle). The `web` contract file holds three source tests (`TestDemoPracticeFlowSource`, `TestDemoFormGuardSource`, `TestReportFormToleratesNoSearch`). The Low data view never had a Call 112 box or picks (its branch of the partial lacks them), so `TestRealFormsKeepCallBoxAndPicks` pins both blocks on the main page and that the Low data view renders unchanged with no Demo flag.

**5. Event shape**
- `pinalert:report-posted` carries `detail = {report}` (not the bare report).

### Not covered

- A vote on an ended visit (`votes.js`) still shows the vote's own error; only the report post and the feed read route to `PinalertDemoEnded`, as the plan names app.js.

## Known Stubs

None for this plan's goal. `/demo/alerts` and `/demo/activity` stay 404 until plan 05-19 (05-13's stub).

## Threat Flags

None beyond the register. T-05-91 (no tel link, no Call 112 box: TestDemoFormHasNoCallBoxOrPicks and the walk), T-05-92 (no page typed trust sentence: negative source test and node cases that feed in made up server words), T-05-93 (all text through setText/textContent, the toast through showToast), T-05-94 (3 second poll only while below Confirmed, stops on Confirmed, on a new watch, after 40 ticks), T-05-95 (Demo false on the main page and Low view: TestRealFormsKeepCallBoxAndPicks, existing suites) are mitigated as planned.

## Process notes the orchestrator should know

- **The walk server was not `cmd/server`.** As in 05-13, the sandbox refuses `ENV=development` and I did not invent a mail key. I wrote a scratch Go main (`cmd/p517walk`) that builds `api.NewRouter` with a literal throwaway session secret, the Demo store with the real seed, and p5_17 on 127.0.0.1:8798. I built the binary into the scratch folder and deleted the source before any commit; it was never added to git. The server was stopped and 8798 confirmed free.
- **app.js merge risk.** Three small edits: a block after `announceFeedCentre`, one branch in `fetchReports`' `!res.ok`, and two lines in `submitReport`. Plans 05-15 and 05-18 ran in parallel; if either touches those spots the merge needs a look. phone.css was not touched.
- Playwright walk scripts live only in the scratch folder, no images were saved anywhere.

## Self-Check: PASSED

- Created files exist: internal/api/handlers/demo_practice_e2e_test.go, web/demo_practice_test.mjs.
- Commits exist on the worktree branch: 261c4834 (test), f7155977 (feat).
- p5_17 dropped, port 8798 free, cmd/p517walk removed.
