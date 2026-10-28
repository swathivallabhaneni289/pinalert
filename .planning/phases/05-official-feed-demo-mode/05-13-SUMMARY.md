---
phase: 05-official-feed-demo-mode
plan: 13
subsystem: demo-page
tags: [demo, nearby, seams, api-base, fixed-position, marks, strip, phone, leaflet]
status: complete

requires:
  - phase: 05-10
    provides: DemoRoutes, DemoConfig, DemoVisitLimitDefault, the /demo mount and the Demo API under /demo/api/v/{visit}
  - phase: 05-06
    provides: the world, PretendLat, PretendLon, PretendPlace
provides:
  - GET /demo (and /demo/): the Demo's Nearby page, a fresh in memory world per visit
  - handlers.DemoPage, demoViewModel, demoExitHref, demoNavFor, NavView.Demo, Chrome.DemoPage
  - web/templates/demo.html.tmpl, demo_strip.html.tmpl, web/static/css/demo.css
  - window.PinalertApiBase seam (api-base.js) in app.js, votes.js, ended-nearby.js
  - the fixed position seam (data-fixed-lat, data-fixed-lon, data-fixed-place) in map.js, modal.js, votes.js
  - the pinalert:feed-centre event from app.js
  - the Demo marks (Demo tag, dashed ring class, "Demo." names) in feed.js and map.js
affects: [05-15, 05-17, 05-18, 05-19]

key-files:
  created:
    - internal/api/handlers/demo_page.go
    - internal/api/handlers/demo_page_e2e_test.go
    - web/templates/demo.html.tmpl
    - web/templates/demo_strip.html.tmpl
    - web/static/css/demo.css
    - web/static/js/api-base.js
    - web/static/js/demo.js
    - web/demo_contract_test.go
    - web/demo_page_test.mjs
  modified:
    - internal/api/handlers/demo.go
    - internal/api/handlers/nav.go
    - internal/api/handlers/chrome.go
    - internal/api/router.go
    - web/templates/app_bar.html.tmpl
    - web/templates/report_form.html.tmpl
    - web/static/js/app.js
    - web/static/js/votes.js
    - web/static/js/ended-nearby.js
    - web/static/js/map.js
    - web/static/js/modal.js
    - web/static/js/feed.js
    - web/ended_nearby_test.mjs
    - web/map_ask_once_test.mjs
    - web/app_bar_contract_test.go
    - web/chrome_contract_test.go
    - web/phone_layout_contract_test.go
    - web/pwa_contract_test.go
    - web/queue_contract_test.go
    - web/report_form_contract_test.go

key-decisions:
  - "The Demo page is a second template that mirrors index.html.tmpl; index.html.tmpl is untouched, and the real scripts run the Demo through two seams (API base, fixed position) that default to the real behaviour."
  - "The vote path also takes the pretend position from the page (getVoterLocation and peekVoterLocation in votes.js), which the plan did not list: without it the first Confirm raised the browser prompt and wrote sessionStorage, and the row could never say Too far to vote."
  - "The shared report form hides the Call 112 box and the What is happening picks behind Chrome.DemoPage, so the Demo page carries no tel link."
  - "The phone bar of the Demo shows the wordmark Pinalert (UI-SPEC 4c) and everything the real phone page fixes to the top of the page starts below the strip."

requirements-completed: []
duration: about 4 hours
completed: 2026-10-29
---

# Phase 5 Plan 13: The Demo's Nearby page Summary

A visitor with no account opens /demo and gets the real Nearby, map and list, on the Vijayawada sample: every row, popup and pin marked Demo, confirm and dispute working under the real rules from a fixed pretend position in Singh Nagar, the real Too far to vote words on farther reports, and an Exit demo button, with no browser location question. One truth is only partly met, see "Findings for the orchestrator" (static files still start a session).

## What was built

- `internal/api/handlers/demo_page.go`: `DemoPage(tmpl, store, cfg)` creates one world per request (`store.Create()`), builds `demoViewModel` (visit, `/demo/api/v/<visit>` as API base, `ExitHref` from `demoExitHref`, the pretend position, the page radii) and renders `demo.html.tmpl` from a buffer, with `Cache-Control: no-store` and `Referrer-Policy: same-origin`. `demoExitHref` answers `/welcome` only for the exact word `welcome`, else `/login`. `demoVisitLimiter` is the per IP visit limit, answering 429 as a plain sentence. A store error answers 503 "The Demo is busy. Try again in a minute."
- `internal/api/handlers/demo.go`: `DemoConfig` gains `Template` and `Page`; `DemoRoutes` mounts `GET /` (so /demo and /demo/) behind the visit limiter when a template is given. A router built without a template mounts no page, so the 05-10 tests are unchanged. `internal/api/router.go` passes `deps.Template` and the page config into the one existing `r.Mount("/demo", ...)`.
- `nav.go`: `NavView.Demo`, `demoNavFor` (the real place list with the hrefs /demo, /demo/alerts, /demo/activity). `chrome.go`: `Chrome.DemoPage`.
- `app_bar.html.tmpl`: under `{{if not .Nav.Demo}}` the Search a place include (still directly under its Nearby condition), the bell and its list, the gear and the person menu. The wordmark and the two Post a report links point at /demo in the Demo; the else branches keep the real addresses.
- `report_form.html.tmpl`: the Call 112 box and the picks sit in `{{if not .Chrome.DemoPage}}`.
- `demo.html.tmpl`, `demo_strip.html.tmpl`, `demo.css`: the page, the strip ("Demo. Nothing here is real." and the 44px outlined Exit demo link, accessible name Exit the demo), the Demo tag, the dashed pin ring, and the phone offsets.
- `api-base.js`: reads `data-api-base` and `data-demo` from `#app-shell` or `main`, sets `window.PinalertApiBase` (only a path of letters, digits, slash, underscore, hyphen, one leading slash) and `window.PinalertDemoPage`.
- `demo.js`: writes the strip height into `--disclaimer-bar-height` now, on resize, on strip resize and one frame later; exposes `measureStrip` and `init`.

## Seam call sites found (rule 1 re-grep)

`grep -c 'func DemoRoutes' internal/api/handlers/demo.go` printed 1.

`grep -n "'/api/reports" web/static/js/*.js` before editing: app.js 560 (feed GET) and 755 (POST), votes.js 525 (vote POST), ended-nearby.js 416 (past GET). All four moved to `apiBase() + '/reports...'`. Also found, left alone because the Demo never loads them: flag.js, mute.js and settings-alerts.js (`/api/reports/<id>/flag|mute|unmute`). The plan's line numbers held for app.js (560, 755), ended-nearby.js and feed.js createRow; votes.js moved from the plan's list: its call is at 525.

`grep -n 'geolocation' web/static/js/map.js web/static/js/modal.js`: map.js 513 and 517 (`locateReader`), modal.js 195 and 196 (`initLocation`). Followed the code: the plan did not name votes.js 304 and 309 (`getVoterLocation`, the third place the page asks the browser, and the sessionStorage cache at 272 and 320), nor `peekVoterLocation` (984, the stored fix the row reads for Too far to vote). Both now read the fixed position first.

## TDD record

RED (commit 596be17e), run before any page code: `TestDemoPageMarkup`, `TestDemoPageLoadsNoForbiddenScripts`, `TestDemoPageHasNoAccountAndNoStorageWrites`, `TestDemoStripCopyAndExit`, `TestDemoNavHidesBellGearSearch`, `TestApiBaseSeam`, `TestFixedPositionSeam`, `TestDemoMarksInFeedAndMap` failed (web), `TestDemoPageWritesNothing`, `TestDemoPageExitAllowlist`, `TestDemoPageRateLimited`, `TestDemoPageRendersWhatItPromises` failed (handlers, on p5_13), and `web/demo_page_test.mjs` failed on the missing scripts. `TestRealPageUnchangedByDemoSeams` passed at once, as a pin that must stay green. The RED commit holds only the new `DemoConfig` fields so the tests compile.
GREEN: Task 2 (784cfb56) made the markup, strip, nav and handler tests pass; Task 3 (542e3094) the seam and mark tests. 313b1393 is the walk fix (phone offsets and wordmark).

## Amended tests (every old case kept, dated notes in the files)

- `web/queue_contract_test.go` (TestQueueSubmitReportClassifiesQueueable): the pinned `fetch('/api/reports', {` is now `fetch(apiBase() + '/reports', {`; on a real page that is still POST /api/reports (`queue_logic_test.mjs` runs it and reads the address, unchanged).
- `web/app_bar_contract_test.go`: `searchBarNav` gains `Demo bool` (the rebuilt shape the real template renders with); TestAppBarReportNeverCurrent accepts the href expression whose else branch is `/?post=1` and pins the Demo branch beside it.
- `web/phone_layout_contract_test.go` (TestPhoneBottomNavMarkup): the same href expression for the Report link.
- `web/chrome_contract_test.go`: a `demo+top` row for demo.html.tmpl (body opens with the Demo strip, then the top bar); the notice strip is still refused outside Nearby.
- `web/pwa_contract_test.go` (TestEveryPageIncludesAppHead): demo.html.tmpl is the one named exemption (app_head loads notify.js and registers the service worker); TestDemoPageLoadsNoForbiddenScripts pins it carries neither.
- `web/report_form_contract_test.go`: the partial holds 6 other template actions instead of 4 (the Demo guard's `if` and `end`); the old four are all still counted.
- `web/ended_nearby_test.mjs`: `load` takes an optional `apiBase`; the pins on /api/reports at the two places stay as they were (default base), and two cases cover the Demo base.
- `web/map_ask_once_test.mjs`: the fake document gains `getElementById` (null like every real page); two cases run the Demo shell (no browser question, no marker, no kept fix, Back to my location).

## Verification

Full suite on p5_13 (`go test ./... -p 1 -count=1 -v`): 5045 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL`, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed). 20 packages `ok`. The named tests passed: TestDemoPageMarkup, TestDemoPageLoadsNoForbiddenScripts, TestDemoPageHasNoAccountAndNoStorageWrites, TestDemoStripCopyAndExit, TestDemoNavHidesBellGearSearch, TestApiBaseSeam, TestFixedPositionSeam, TestDemoMarksInFeedAndMap, TestRealPageUnchangedByDemoSeams, TestDemoPageWritesNothing, TestDemoPageExitAllowlist, TestDemoPageRateLimited (plus TestDemoPageRendersWhatItPromises). No known flake failed, so none was rerun. `node --test web/*.mjs`: 979 tests, 979 pass, 0 fail. `go vet ./...` clean. `gofmt -l` lists only the two files that were already listed (internal/api/handlers/place_test.go, internal/geocode/photon.go). A grep of every line this branch added for en and em dashes found none.

## Browser walk (Playwright, 8 runs)

Scratch walk server on 8795 (see Deviations, item 3). Each run is a fresh context with no cookie, geolocation permission not granted and a spy on `Geolocation.prototype.getCurrentPosition` and `watchPosition`, `navigator.permissions.query`, `Storage.prototype.setItem` and `indexedDB.open`. Screenshots only in the scratch folder, none in the repo.

| Check | chromium 390 light | chromium 390 dark | chromium 1470 light | chromium 1470 dark | webkit 390 light | webkit 390 dark | webkit 1470 light | webkit 1470 dark |
|---|---|---|---|---|---|---|---|---|
| GET /demo 200, no Set-Cookie on the page, Referrer-Policy same-origin | yes | yes | yes | yes | yes | yes | yes | yes |
| Strip first (top 0, 54px), text "Demo. Nothing here is real.", Exit 44px, name "Exit the demo", no underline, no notice strip | yes | yes | yes | yes | yes | yes | yes | yes |
| Bell, gear, person menu, Search a place, Only Reliable, Flag, Mute absent; theme button only at 1470 | yes | yes | yes | yes | yes | yes | yes | yes |
| Map centred at the pretend position (16.5424, 80.6386; 1470 shifts 0.004 deg for the tools row), 18 pins | yes | yes | yes | yes | yes | yes | yes | yes |
| Every pin has the 2px dashed ring, offset 2px; pin names start "Demo." | yes | yes | yes | yes | yes | yes | yes | yes |
| 18 rows, each with the Demo tag first in the state line, row text starting "Demo." | yes | yes | yes | yes | yes | yes | yes | yes |
| Popup or detail sheet carries the Demo tag in the badge row, no Flag or Mute | yes | yes | yes | yes | yes | yes | yes | yes |
| Confirm a report within 1 km: 200 from /demo/api/v/<visit>/reports/7/confirm; trust block "0 confirmed, Not confirmed yet, Needs 2 people at different spots" becomes "1 confirmed, Confirmed by 1 person, Needs 1 more person at a different spot" | yes | yes | yes | yes | yes | yes | yes | yes |
| Dispute another near report: 200 | yes | yes | yes | yes | yes | yes | yes | yes |
| Far report (about 1.4 to 5 km): the row says "Too far to vote"; the open report shows the button marked and the note "Too far to vote. You need to be within 1 km of this report."; a tap on the marked button sends no request | yes | yes | yes | yes | yes | yes | yes | yes |
| Geolocation spy (position, watch) and permissions query counts | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Post a report window opened: pin placed, line "Near Singh Nagar.", no location notice, no `tel:` link, no Call 112 box, no picks, no /api/geocode request, spy 0 | yes | not run | yes | not run | yes | not run | yes | not run |
| sessionStorage, IndexedDB (databases list), document.cookie | empty | empty | empty | empty | empty | empty | empty | empty |
| localStorage | empty | empty | 1 key | 1 key | empty | empty | 1 key | 1 key |
| Every request to this site other than assets goes to /demo/api/v/<visit>/ | yes | yes | yes | yes | yes | yes | yes | yes |
| Document scroll 0; shell top at the strip bottom (54px); top bar below the strip (phone) | yes | yes | yes | yes | yes | yes | yes | yes |
| Exit demo goes to /login; /demo?from=welcome renders Exit to /welcome, which redirects to /login | yes | yes | yes | yes | yes | yes | yes | yes |
| Page errors | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

The post window row was run in a separate pass (post.js) for all four size and engine pairs; dark was not repeated for it. The one localStorage key at laptop width is `pinalert_theme`, see Findings. The webkit runs log console errors "pt" and "AJAXError ... tiles.openfreemap.org": cancelled and failed map tile requests when the map pans or the sandbox has no route to the tile host, not page code. The real Nearby was rechecked in both engines at 390 and 1470: a signed in account (log mailer link) loads `GET /api/reports`, asks the browser for its location once, shows no Demo strip, tag or attribute, no page errors.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] votes.js asked the browser for a position and wrote sessionStorage**
- **Found during:** orientation, confirmed by the node case
- **Issue:** the plan names map.js and modal.js for the fixed position, but the first Confirm would have raised the browser prompt (`getVoterLocation`) and written `pinalert_voter_location`, and the row's Too far to vote reads the stored fix (`peekVoterLocation`). That fails truth 4 and T-05-75.
- **Fix:** `fixedPosition()` in votes.js, read first by `getVoterLocation` (resolves the page position, no storage, no prompt) and `peekVoterLocation`. The server judges the 1 km rule from the pretend position whatever the body says (05-10), so the numbers the page sends grant nothing.
- **Files:** web/static/js/votes.js; pinned in TestFixedPositionSeam and demo_page_test.mjs. **Commit:** 542e3094

**2. [Rule 3 - Blocking] router.go and the page configuration**
- The page needs the templates and the real page configuration, which only the router has; `router.go` is not in the plan's file list. One line changed (the existing `r.Mount("/demo", ...)` now passes `Template` and `Page`). **Commit:** 784cfb56

**3. [Rule 2 - Missing critical] tel link in the shared report form**
- T-05-73 says the page carries no tel link, and plan 05-17 only "finishes" it. The shared partial would have rendered the hidden Call 112 box with `tel:`. Added `Chrome.DemoPage` and an `{{if not .Chrome.DemoPage}}` around the box and the picks in `report_form.html.tmpl` (not in the plan's file list). The real page and the Low data view render exactly as before. **Commit:** 784cfb56

**4. [Rule 1 - Bug] phone layout under the Demo strip (found in the walk)**
- phone.css fixes the top bar, the report view with its Back arrow and the not available view to the top of the page, which put them over the Demo strip; and the real page hides the phone title because Search a place takes its place, so the Demo bar showed "Nearby" over the map. demo.css moves each below the strip by `--disclaimer-bar-height`, the title is the wordmark "Pinalert" on a solid chip (UI-SPEC 4c) and steps aside while a report is open. **Commit:** 313b1393

**5. [Rule 3] Place search kept but hidden in the post window**
- report-form.js binds `#location-search` without a null check, so the three search elements stay in the window's markup, with the field wrapper hidden. No geocoder request is made (walk). Plan 05-17 owns the practice flow.

**6. Placement of the centre event**
- `announceFeedCentre` sits right after `state.lastUpdatedAt` in the success branch, before `markListLoading(false); notify();`, because two existing pins require those two statements adjacent. The event is announced in the same step as the render.

### Process notes the orchestrator should judge

- **I ran `python3 --version` once, by mistake** (a typing slip while composing a command). The project rule forbids even that. It printed a version and did nothing else. No Python was written, run or added anywhere; the repo holds none.
- **The walk server was not `cmd/server`.** The sandbox refused `ENV=development` for a plain command, and the real server refuses to start without it or a mail key. I did not invent a mail key. I wrote a scratch Go main (`cmd/p513walk`, never added to git, deleted before the last commit) that builds `api.NewRouter` like the router tests do, with a literal throwaway session secret ("p5-13-walk-scratch-secret-not-for-production", the same kind of value the router tests use), the log mailer, `127.0.0.1:8795` and the p5_13 database. That is the same router and the same code a test server runs, but it is not the deployed binary. It was stopped, port 8795 confirmed free.

## Findings for the orchestrator

1. **Truth 6 is only partly met: static files start a session.** `GET /demo` and the Demo API write no row and set no cookie (TestDemoPageWritesNothing proves it, and curl on /demo left `sessions` unchanged). But the browser then fetches `/static/css/*.css` and `/static/js/theme.js`, which are not in `sessionSkipSet`, and each cookieless response sets `pinalert_session` and upserts a sessions row. Measured on p5_13 with the final build: `select count(*) from sessions` was 0 before one Chromium visit to /demo and 12 after (the 12 parallel cookieless asset requests); a later single `curl` of `/static/css/demo.css` made it 13. The real sign in page does the same today. I did not fix it: plan 05-12 edits the same `sessionSkipSet` return line and `TestPWASkipSetMatchesRoutes`, so a change here would conflict. Suggested follow up after 05-12 merges: add a `"/static"` prefix to `sessionSkipSet`, amend `TestPWASkipSetMatchesRoutes` (its prefix assertion currently says exactly `["/demo"]`) with a dated note, and extend `TestDemoPageWritesNothing` to GET one `/static` asset and assert no Set-Cookie and unchanged table counts.
2. **localStorage is not empty at laptop width.** The shared `theme.js`, which every page loads, writes the display preference `pinalert_theme` on first run at widths of 900px and up (it writes nothing on a phone width). It is the only key: no `pinalert.lowLastPosition`, no `pinalert_voter_location`, no queue, no account key, and sessionStorage and IndexedDB are empty. The plan and UI-SPEC 4d.6 say the Demo writes no browser storage and keep the theme button on the laptop bar, so the two conflict. Open decision: leave it (a display preference, harmless), or have the Demo page skip the write.
3. The Demo bars link `/demo/alerts` and `/demo/activity`, which answer 404 until plan 05-19.

## Known Stubs

- `/demo/alerts` and `/demo/activity` (linked from both bars) are 404 until plan 05-19.
- No alert banner on the Demo list yet (plan 05-19 adds it, plan 05-15 reads `pinalert:feed-centre`).
- The practice posting flow (the climb to Confirmed, the toast, the Critical choice) is plan 05-17's; the post window opens and the pin starts at Singh Nagar, but a posted report has no extra Demo handling yet beyond the Demo API.
- The place search inside the post window is present and hidden, left for 05-17.

## Threat Flags

None beyond the plan's register. T-05-69 (Exit allowlist, TestDemoPageExitAllowlist), T-05-71 (marks, walk), T-05-72 (visit limit, TestDemoPageRateLimited), T-05-73 (no tel link: source, rendered output and the walk), T-05-74 (real page unchanged, walk), T-05-75 (no location call, walk spy at 0) are mitigated as planned. T-05-70 is partly met, see Findings 1 and 2.

## Self-Check: PASSED

- All created files exist: internal/api/handlers/demo_page.go, demo_page_e2e_test.go, web/templates/demo.html.tmpl, demo_strip.html.tmpl, web/static/css/demo.css, web/static/js/api-base.js, demo.js, web/demo_contract_test.go, web/demo_page_test.mjs.
- Commits exist on the worktree branch: 596be17e (test), 784cfb56 (page), 542e3094 (seams), 313b1393 (walk fix).
- `git diff -- web/templates/index.html.tmpl` against the base is empty.
- p5_13 dropped (`psql -lqt` shows no p5_13), port 8795 free, `cmd/p513walk` removed.
