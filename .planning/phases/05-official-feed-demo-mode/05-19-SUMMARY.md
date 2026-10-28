---
phase: 05-official-feed-demo-mode
plan: 19
subsystem: demo-places
tags: [demo, official-alerts, banner, my-activity, visit, sample-alerts, phone, laptop]
status: complete

requires:
  - phase: 05-06
    provides: SampleAlerts (3 invented alerts), the world, World.Activity, PretendLat and PretendLon
  - phase: 05-08
    provides: official.BuildView, ViewInput.Demo, Row.Demo and officialViewToResponse
  - phase: 05-09
    provides: the source agnostic Official alerts renderer (official-alerts.js)
  - phase: 05-13
    provides: DemoRoutes, the Demo page, demoNavFor, the API base and fixed position seams
  - phase: 05-15
    provides: the Nearby banner script (official-banner.js)
  - phase: 05-17
    provides: demo.js and the practice flow
provides:
  - demo.SampleView, the sample alerts through the real view
  - GET /demo/alerts and GET /demo/api/v/{visit}/official/alerts
  - GET /demo/activity (four parts, this visit only)
  - the Demo tag on the Nearby banner (Banner.Demo, OfficialBannerResponse.Demo, #alert-banner-demo)
  - one visit across the three places (?v=<visit>, taken off the address on load)
  - World.VisitorVotes (the visitor's vote times)
affects: [05-20, 05-21, 05-22]

key-files:
  created:
    - internal/demo/alerts.go
    - internal/demo/alerts_test.go
    - internal/demo/visitor.go
    - internal/api/handlers/demo_official.go
    - internal/api/handlers/demo_activity.go
    - web/templates/demo_alerts.html.tmpl
    - web/templates/demo_activity.html.tmpl
  modified:
    - internal/official/view.go
    - internal/official/view_test.go
    - internal/api/handlers/official.go
    - internal/api/handlers/demo.go
    - internal/api/handlers/demo_page.go
    - internal/api/handlers/nav.go
    - internal/api/handlers/demo_e2e_test.go
    - internal/api/handlers/demo_page_e2e_test.go
    - internal/api/handlers/swagger_test.go
    - web/templates/demo.html.tmpl
    - web/static/js/official-banner.js
    - web/static/js/ended-nearby.js
    - web/static/js/demo.js
    - web/static/css/demo.css
    - web/demo_contract_test.go
    - web/official_banner_test.mjs
    - web/demo_practice_test.mjs
    - web/ended_nearby_test.mjs
    - web/chrome_contract_test.go
    - web/pwa_contract_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml

key-decisions:
  - "The sample alerts are stored-alert values placed on the pretend position's district and state, then run through the same pure BuildView as the real page with Demo set. Nothing is painted: the groups, the labels, the banner pick and the +1 come from the real rule."
  - "A Demo row's details panel no longer says Source: NDMA SACHET. buildRow gives Demo rows a line of their own (Made up for the Demo. Not from any issuer.), so a made up alert never names an issuer's channel (T-05-99)."
  - "The three Demo places share one visit through ?v=<visit>. Every page takes the v off its own address with history.replaceState as soon as it loads, so a reload or a copied address starts a fresh Demo (D-01). The bar links carry the visit from the server (place links) and from demo.js (wordmark and Report links, which the shared bar writes bare)."
  - "My activity builds the real service.ActivityPage from the world (Focus for each report, VisitorVotes for the vote words and times) and hands it to the real buildActivityPage, so every card is the real card. The Demo drops the Updates part and the reporter record; every card carries the Demo tag in its status cell."
  - "from=welcome travels with the visit on the bar links, so Exit demo goes to /welcome on all three places and not only on the one the visitor entered."

requirements-completed: []
duration: about 5 hours
completed: 2026-10-29
---

# Phase 5 Plan 19: The Demo's Official alerts, banner and My activity Summary

The Demo now has all three places. Official alerts lists the three made up alerts through the real renderer with the Demo tag, no Official badge and the line "These alerts are made up for the Demo.", Nearby shows the sample Extreme alert in the real banner with a Demo tag inside the same 44px row, and My activity lists the practice report and the votes of this visit in the real four part layout, all through one visit that writes nothing and sets no cookie.

## What was built

- `internal/demo/alerts.go`: `SampleView(w *World, now time.Time) official.View` maps `SampleAlerts` to stored alerts on the district and state of the pretend position (areas Locate), with times relative to now, and calls `BuildView` once with Demo true and a feed checked this moment (fresh, age 0). `internal/official/view.go`: `Banner.Demo` set from `ViewInput.Demo`, `AreaOfDistrict` (the exported twin of the service's district area), and the Demo source line.
- `internal/api/handlers/demo_official.go`: `DemoAlertsPage` (GET /demo/alerts) and `demoOfficialAlerts` (GET /demo/api/v/{visit}/official/alerts, answering `officialViewToResponse(demo.SampleView(...))`, ignoring the position in the query). `official.go`: `OfficialBannerResponse.Demo` (json demo). Both read no session or account.
- `internal/api/handlers/demo_activity.go`: `DemoActivityPage` (GET /demo/activity). It reads each report the visitor posted, confirmed or disputed with the real `Focus` read (so a resolved or faded report stays on the page), shapes it as a `service.ActivityReport`, and runs the real `buildActivityPage`. Part addresses stay in the Demo and in the visit. `internal/demo/visitor.go`: `World.VisitorVotes` reads the visitor's standing content vote and its time from the world's vote log.
- `internal/api/handlers/demo_page.go`, `demo.go`, `nav.go`: `demoVisitFor` (a valid, living `?v=` is reused, anything else starts a new world), `newDemoPlaceModel` and `serveDemoPlace` shared by the three pages, `demoPlaceHref`, `demoNavFor(current, visit, from, ...)`, and the two new routes registered inside `DemoRoutes` behind the visit limiter.
- `web/templates/demo_alerts.html.tmpl`, `demo_activity.html.tmpl`, `demo.html.tmpl`: the alerts shell (page root with the Demo API base and the pretend position, no account, no stored position script), the four part activity page (real classes, empty states and scripts, a Demo tag in every card, no Updates part, no queue strip), and the banner copy on Nearby with the `#alert-banner-demo` span after the icon.
- `web/static/js/official-banner.js` (optional Demo tag, shown only when the pick says `demo === true`, hidden with the banner), `ended-nearby.js` (reads the page's `data-fixed-lat` and `data-fixed-lon` before any stored position), `demo.js` (takes `?v=` off the address with `replaceState`, completes the wordmark and Report links from `data-visit`), `demo.css` (the tag is `flex: none` and has its own `[hidden]` rule).

## TDD record

RED (90b48f7f): `TestSampleView`, `TestSampleViewIsRelativeToNow`, `TestBuildViewDemoBanner`, `TestDemoAlertsPageMarkup`, `TestDemoOfficialAlertsAPI` failed (the stub `SampleView` and an unset `Banner.Demo` kept it compiling). GREEN: abc1e9b4.
RED (9ef3d856): 8 node cases (banner Demo tag, `?v=` removal, link rewrite, fixed position in ended-nearby.js) and `TestDemoBannerOnDemoPage`, `TestDemoActivityPageMarkup`, `TestDemoActivityPageParts`, `TestDemoPageLoadsNoForbiddenScripts` (amended list) failed. GREEN: 0990a3c4.
Task 3 (0857aa28): `TestDemoPlacesWriteNothing` and `TestDemoNavKeepsVisit` passed at once, as they pin what Tasks 1 and 2 built.

## Amended tests (every old case kept, dated notes in the files)

- `web/demo_contract_test.go` `TestDemoPageLoadsNoForbiddenScripts`: `official-banner.js` joins the expected list right after `app.js`.
- `web/chrome_contract_test.go`: the two new Demo templates get `demo+top` rows (the Demo strip, then the top bar). `web/pwa_contract_test.go`: the two new Demo templates join the named app head exemption, each pinned to carry none by its own markup test.
- `internal/api/handlers/demo_page_e2e_test.go` `TestDemoPageRendersWhatItPromises`: the Official alerts and My activity bar links are checked as `/demo/alerts?v=` and `/demo/activity?v=` (they carry the visit now).
- `internal/api/handlers/swagger_test.go` `TestSwaggerOfficialAlerts`: the property list of `handlers.OfficialBannerResponse` gains `demo`; the six keys written before are all still required. The first full run caught this one (see Verification); 65d25c8e fixed it.
- `web/official_banner_test.mjs`: `buildPage` takes `demoPage` (the Demo template's own banner markup); five new cases. `web/ended_nearby_test.mjs`: `load` takes `fixed`; two new cases. `web/demo_practice_test.mjs`: `load` takes a history object, a data-visit element and the bar links; four new cases. The cases written before are untouched.

## Verification

- Full suite on p5_19 (`go test ./... -p 1 -count=1 -v`, the second full run): 5192 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL`, 0 panics, 20 packages `ok`, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed). The named tests passed: TestSampleView, TestSampleViewIsRelativeToNow, TestBuildViewDemoBanner, TestDemoAlertsPageMarkup, TestDemoBannerOnDemoPage, TestDemoActivityPageMarkup, TestDemoActivityPageParts, TestDemoOfficialAlertsAPI, TestDemoPlacesWriteNothing, TestDemoNavKeepsVisit, plus TestDemoPageWritesNothing, TestDemoWritesNothing, TestDemoParityWithSQL and TestSwaggerOfficialAlerts. No known flake fired, so none was rerun. The first full run had one failure, TestSwaggerOfficialAlerts (a property list pin that did not know the new `demo` field of the banner); 65d25c8e fixed it and the second run is the one reported here.
- `node --test web/*.mjs`: 1043 tests, 1043 pass, 0 fail. `go vet ./...` clean. `gofmt -l .` printed `internal/api/handlers/place_test.go` and `internal/geocode/photon.go`, both listed before this plan; none of my files.
- Acceptance greps: `func SampleView(w \*World, now time.Time) official.View` 1, `BuildView(` in alerts.go 1, the Demo attribution sentence in the alerts template 1, `data-account-id` in both new templates 0, `alert-banner-demo` and `official-banner.js` in demo.html.tmpl 1 each, `func DemoActivityPage` 1, `data-fixed-lat` in ended-nearby.js 2, `Amended 2026-10-.. by plan 05-19` in official_banner_test.mjs 2, no account or session read in demo_official.go, no en or em dash in any file I wrote.
- p5_19 dropped and confirmed gone with `psql -lqt`; nothing listens on 8791.

## Browser walk (Playwright, 8 runs, a fresh context and no cookie each run)

Scratch server on 8791 (see Deviations, item 7), the real `api.NewRouter` over p5_19. The walk opens `/demo?from=welcome`, taps the banner, opens the first alert's details, goes back to Nearby, posts a practice report through the real window, confirms one near report and disputes another, opens My activity, and on the phone opens Posted by you and Ended nearby. A geolocation spy, a permissions spy and a storage spy ran in every run. Screenshots stayed in the scratch folder.

| Check | chromium 390 light | chromium 390 dark | chromium 1470 light | chromium 1470 dark | webkit 390 light | webkit 390 dark | webkit 1470 light | webkit 1470 dark |
|---|---|---|---|---|---|---|---|---|
| Nearby 200, no Set-Cookie, strip first (top 0), Exit 44px to /welcome, no notice strip | yes | yes | yes | yes | yes | yes | yes | yes |
| Sample banner shown: 44px, Extreme look, Demo tag shown (24px) after the icon and before the event words, tag inside the row, no wrap, no horizontal overflow; reads "Flood warning · Ntr district +1" | yes | yes | yes | yes | yes | yes | yes | yes |
| Banner address is /demo/alerts?v=<visit>&from=welcome; wordmark and both Report links carry the same visit and from | yes | yes | yes | yes | yes | yes | yes | yes |
| Tap on the banner lands on /demo/alerts, same visit, ?v= gone from the address, from=welcome kept | yes | yes | yes | yes | yes | yes | yes | yes |
| Alerts: status "Last checked just now", "In force now 3", groups "Where you are: Ntr district" and "Across Andhra Pradesh", 3 rows, every row has the Demo tag, no Official badge, attribution exactly "These alerts are made up for the Demo.", no NDMA, SACHET or GDACS word on the page, Official alerts current | yes | yes | yes | yes | yes | yes | yes | yes |
| Alert details panel Source line reads "Made up for the Demo. Not from any issuer." | yes | yes | yes | yes | yes | yes | yes | yes |
| Strip and Exit (to /welcome) on all three places | yes | yes | yes | yes | yes | yes | yes | yes |
| Back to Nearby by the bar keeps the same visit | yes | yes | yes | yes | yes | yes | yes | yes |
| Practice post 201, confirm 200, dispute 200, all on /demo/api/v/<visit>/ | yes | yes | yes | yes | yes | yes | yes | yes |
| My activity: Posted 1, You confirmed 1, You disputed 1, every card has the Demo tag, the status chip and trust block drawn by activity.js, "You confirmed just now", Distance "0.1 km" from the pretend position, no bell and no queue strip | yes | yes | yes | yes | yes | yes | yes | yes |
| Ended nearby loads from the visit's own route (laptop: column heads shown; phone: 1 faded row) with no location state and no error | yes | yes | yes | yes | yes | yes | yes | yes |
| Phone: hub list, Posted by you opens as its own screen with the part name as title and a Back arrow to /demo/activity?v=<visit>; only that part is shown | yes | yes | n/a | n/a | yes | yes | n/a | n/a |
| Requests: 0 to /api outside /demo/api/v/, the sample route called 3 times, 0 Set-Cookie on any /demo path, 0 browser cookies | yes | yes | yes | yes | yes | yes | yes | yes |
| Geolocation and permissions spy counts | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| sessionStorage, IndexedDB, document.cookie | empty | empty | empty | empty | empty | empty | empty | empty |
| localStorage | empty | empty | 1 key | 1 key | empty | empty | 1 key | 1 key |
| Page errors | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

The one localStorage key at laptop width is `pinalert_theme`, written by the shared theme.js at load (plan 05-13 reported the same). It is the theme choice, not Demo data.

Banner fit with the Demo tag (area text "Ntr district", row stays 44px and never wraps, both engines, light and dark):

| Width | Tag | Area width left | Area shown |
|-------|-----|-----------------|------------|
| 390 | 57px wide, 24px high | 69px | all 12 characters |
| 360 | same | 48px | clipped with an ellipsis |
| 320 | same | 8px | about one letter |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] A Demo row said its source was NDMA SACHET**
- **Found during:** Task 1 (T-05-99)
- **Issue:** `buildRow` cleared the badge for Demo rows but left `Details.SourceLine = "NDMA SACHET"`, so the details panel of a made up alert named an issuer's channel.
- **Fix:** Demo rows get `sourceLineDemo` ("Made up for the Demo. Not from any issuer."). Pinned in `TestBuildViewDemoBanner`, `TestSampleView` and the walk. The real source line is unchanged and pinned.
- **Files modified:** internal/official/view.go. **Commit:** abc1e9b4

**2. [Rule 3 - Blocking] nav.go and demo_page.go moved into Task 1**
- The alerts page needs the Demo bars with the visit on their links, so `demoNavFor(current, visit, from, ...)`, `demoVisitFor`, `newDemoPlaceModel` and `serveDemoPlace` were built with Task 1, not Task 2. `TestDemoPageRendersWhatItPromises` is amended for the new addresses (dated note).

**3. [Rule 3 - Blocking] Files outside the plan's list**
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`: regenerated with `swag init` for the new `demo` field of `OfficialBannerResponse` (the diff is that one property).
- `internal/demo/visitor.go` (new): the world's vote log was the only place the vote words and times of the visitor are kept, and My activity needs them ("You confirmed 3 min ago"). `World.Activity` and world.go are untouched.
- `web/ended_nearby_test.mjs`, `web/chrome_contract_test.go`, `web/pwa_contract_test.go`, `internal/api/handlers/demo_page_e2e_test.go`, `internal/api/handlers/swagger_test.go`: amended as listed above, with dated notes.
- `internal/official/view.go` gained the exported `AreaOfDistrict` so the Demo places the sample on the same area shape the real service builds (the state name needs `titleName`, which is unexported).

**4. [Interpretation] The wordmark and Report links are completed by demo.js**
- `app_bar.html.tmpl` writes the wordmark and both Report links with the bare Demo address, and the real pages' tests pin that href expression. Rather than change the shared bar and those pins, `demo.js` adds `?v=<visit>` (and `from=welcome`) to those links from the page's own `data-visit`. Without script they start a fresh Demo, as a reload does.

**5. [Interpretation] Only `?part=` picks a Demo part**
- The real page also accepts the old `tab=` values and `part=updates`. The Demo has no old bookmarks and no Updates part, so both read as the list (pinned in `TestDemoActivityPageParts`).

**6. [Interpretation] Sample times follow the clock of each read**
- Each read of the sample route is relative to the world clock, so "Issued 40 min ago" stays 40 minutes however long the page is open. The sample is the same for every visit, so `SampleView` accepts a nil world.

**7. [Scratch] Throwaway server**
- The sandbox refuses the server with ENV=development and I did not invent a mail key. A throwaway test file (`internal/api/handlers/zz_p5_19_walk_test.go`, literal throwaway session secret, log mailer, 127.0.0.1:8791, p5_19) served the real `api.NewRouter`; it was deleted before the commit that follows it (`git status` showed only my files), and the server was stopped.

## Known limits and findings for the orchestrator

- **Browser Back.** A page removes `?v=` from its own address (D-01), so a Back to the previous Demo place loads an address with no visit and starts a fresh Demo (or restores the page from the browser's page cache, when it has it). The bar links and the banner keep the visit; only the Back button and a reload start again, as the plan asks.
- **320px banner.** With the Demo tag the area text gets 8px at 320 and 48px at 360 (table above). The row stays one line at 44px and the accessible name holds the full words. This is the clipping plan 05-15 already flagged for its review round; the tag makes it a little worse on the Demo only.
- **Visit limiter.** The three pages share the per IP visit limit (burst 30, one more every 6 seconds), so a visitor who moves between places very fast could meet the plain "Too many visits" page. Reusing a visit costs no memory (T-05-101).

## Known Stubs

None. The Demo's sample alert text is invented on purpose and every row, group and the banner carry the Demo flag.

## Threat Flags

None beyond the plan's register: T-05-99 (Demo tag, no badge, the Demo attribution and source line: `TestSampleView`, `TestDemoAlertsPageMarkup`, the banner node cases and the walk), T-05-100 (`TestDemoPlacesWriteNothing` over every table, no account or session read), T-05-101 (pages behind the visit limiter, `?v=` reuses a world), T-05-102 (`ValidVisitID` before the lookup, `TestDemoNavKeepsVisit` with malformed values, and no echo of the value into the page).

## Commits

- 90b48f7f test(05-19): pin the Demo's sample alerts view, alerts API and alerts page
- abc1e9b4 feat(05-19): the Demo's Official alerts place from the sample alerts
- 9ef3d856 test(05-19): pin the sample banner, the Demo's My activity and the visit address
- 0990a3c4 feat(05-19): the sample banner on the Demo's Nearby and the Demo's My activity
- 0857aa28 test(05-19): the Demo's three places write nothing and keep one visit
- 65d25c8e test(05-19): the swagger pin for the banner gains the demo flag
- the metadata commit holding this file

## Self-Check: PASSED

- Created files exist: internal/demo/alerts.go, alerts_test.go, visitor.go, internal/api/handlers/demo_official.go, demo_activity.go, web/templates/demo_alerts.html.tmpl, demo_activity.html.tmpl.
- Commits 90b48f7f, abc1e9b4, 9ef3d856, 0990a3c4, 0857aa28 and 65d25c8e exist on the worktree branch.
- The throwaway walk test file is gone, p5_19 is dropped (`psql -lqt` count 0) and nothing listens on 8791.
