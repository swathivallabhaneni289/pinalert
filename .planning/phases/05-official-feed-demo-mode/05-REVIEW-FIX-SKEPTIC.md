---
phase: 05
part: review-fix-skeptic
base: e2a20bdd
head: f6ca25ba
checked_at: 2026-10-29
counts:
  closes: 40
  closes_with_new_problem: 3
  does_not_close: 1
  recorded_not_changed: 7
  not_reproduced: 1
  duplicate_rows: 1
  total_rows: 53
new_problems:
  warning: 3
  info: 1
---

# Phase 5 review fix round: skeptic pass

Read only. Scope is the fix commits between e2a20bdd and f6ca25ba (the three merged fixers). While this pass ran, main moved on to b71ff71b with a follow-up round (3bc67461 to 35875b59). That round changed no JavaScript, so the JavaScript line numbers below hold at both f6ca25ba and b71ff71b. Its one effect on this table is noted under WR-T07.

What I ran:
- `node --test` on official_alerts, official_pins, official_banner, welcome_flow, demo_page, ended_nearby, official_card_heading, demo_practice and login_globe: 337 of 337 pass.
- `DATABASE_URL= go test -short` on internal/official, internal/areas, internal/demo and internal/api/handlers: all ok. `go vet` on official and handlers: clean.
- No database, no browser, no Python.

Depth:
- Read in full and traced: every fix for CR-B01, CR-B02, WR-B01, WR-B02, WR-B05, WR-B09, CR-S01, WR-S01, WR-S02, WR-S05, WR-S06 and WR-T01.
- Rows marked "light" were checked only against the diff and the tests that exist for them. They were not traced through their callers.

## Verdicts

Lens 1: does the fix close the finding for the scenario the review wrote? Lens 2: does it add a new problem?

| Id | Lens 1 | Lens 2 | Evidence |
|----|--------|--------|----------|
| CR-S01 | closes | none found | store.go:103-110 runs MarkWithdrawnByStored after every non Cancel upsert and before the hooks (refresher.go:578-585). References are stored as bare identifiers: parseReferences (cap.go:266-289) keeps the identifier of each sender,identifier,sent triple, and store.go:123 joins them with spaces, so `= ANY(string_to_array(...))` matches. Covers Cancel first, Update first across runs, and a refetch under its own guid. |
| WR-S01 | closes | none found | refresher.go:336-340: a Partial run returns before success. The run is due by LastAttemptAt, so a partial run does not cause a tight rerun. A failed CAP is not partial. |
| WR-S02 | closes | none found (see note 1) | refresher.go:424-444: saves run on WithoutCancel plus 10 s, and the success mark on WithoutCancel plus 5 s. Each save is one transaction that upserts by guid, so a repeat stores no second row. |
| WR-S03 | closes (light) | none found | EndOfficialHazard flips only a stored current row. Covered by the new subtest of TestGdacsRefreshStores and by TestGdacsEndedA. |
| WR-S04 | closes (light) | none found | Count leaves ended rows out, and a zero date is sent as "". The amended view tests keep the old "3 listed" and "2 listed" checks through listedRows. |
| WR-S05 | closes | none found (see note 2) | pushenqueue.go:81-129. The step is half a precision 4 cell, and both box ends are sampled, so every prefix 4 cell that meets a padded district box is listed. A matching home cell (areas.go:226, five points inside the cell) has a prefix 4 parent that holds the matching point, so it is never filtered out. Prefixes come from `scoped`, the same placement matchDistricts uses. |
| WR-S06 | closes | new problem NP-3 (info) | demo_page.go:125-129 charges only before store.Create. A missing charge in the context means no limit. |
| WR-S07 | closes (light) | none found | names.go is applied in Load. TRUST-MODEL "Ntr" was fixed after head (8a64ce79). |
| WR-S08 | closes (light) | none found | The test drops the other index inside its own rolled back transaction. |
| IN-S01 | closes (light) | none found | The field is removed, and TestNoTemplateReadsChromeDemoPage covers it. |
| IN-S02 | closes (light) | none found | Comment only. |
| IN-S03 | closes (light) | none found | sachet.go:308-316: the recover sets that item's error. |
| IN-S04 | closes (light) | none found | refresher.go:283-297: waiters get errRunPanicked, and the panic is rethrown after close(c.done). |
| IN-S05 | closes (light) | none found | Server side, plus the browser rows of WR-S04-browser and IN-S05-browser. |
| IN-S06 | recorded, not changed | n/a | Per triage. |
| IN-S07 | closes (light) | none found | The amended TestTitleName keeps the J and K case, with new words. |
| IN-S08 | closes (light) | none found | Issuers now read "Sample ...". The orchestrator still decides between this and the triage default. |
| IN-S09 | recorded, not changed | n/a | Per triage. |
| IN-S10 | recorded, not changed | n/a | Per triage. |
| IN-S11 | closes (light) | none found | PushOff leaves out the hook. |
| CR-B01 | closes | new problems NP-1 and NP-4 (warning) | official-alerts.js:895-905: the tick calls load() once a fresh answer reaches 1800 s or a shown row ends, and every 5 min while stale or unavailable. :907-927 add visibilitychange and pageshow. load() calls showLoading() first (:781), so neither claim outlives the answer. |
| CR-B02 | closes | none found | Every script that builds an API URL and is loaded on a Demo page fails closed: app.js:125-136 and :706-709, :891; votes.js:294-306; ended-nearby.js:125-136; official-alerts.js:308-318 and :788; official-banner.js:196-206. report-form.js:952 and map-tools.js:341 call /api/geocode, but the Demo has no place search (app_bar.html.tmpl:33-37 under `not .Nav.Demo`, demo.html.tmpl:23-25), and activity.js, demo.js, show-menu.js, modal.js, map.js and feed.js have no fetch. The null base is checked before each fetch: votes.js:592-598, ended-nearby.js:467-473, app.js:706-709 and :891-898. official-pins.js, flag.js, mute.js and sw.js are not loaded on any Demo page. |
| WR-B01 | closes, partly on the alerts page | none found | Pins and banner: a real 10 s timer with abort, a sequence drop and requestOut released (official-pins.js:480-532, banner :211-260). Alerts page: the limit is checked on the minute tick (official-alerts.js:841-852), so the "10 s" is 60 to 70 s in practice. This is documented. During that time the page shows only the skeleton and makes no claim. |
| WR-B02 | closes | new problem NP-2 (warning) | official-pins.js:403-469 close the card on clear, replace and remove, through map.js:2323-2328. |
| WR-B03 | closes | none found | map.js:1081-1086: "Hazard" is set after the official check, and the reused heading is retitled (:2238-2243). |
| WR-B04 | closes | none found | official-pins.js:281-285 gives each card its own id. |
| WR-B05 | closes | none found (see note 3) | welcome.js:138 and :1343-1349. The loop is the DOM order of shown, enabled, non hidden controls. The template has no positive tabindex and no radios. The only CSS-hidden controls are in the side panel, and sideShown() handles it. Agree and Disagree are handled by UNREAD_ATTR. So focus() always lands, and Tab is never swallowed. The amended old cases now assert the next control as well, so none is weaker. |
| WR-B06 / WR-T05 | closes | none found | role was dropped in alerts.html.tmpl, and the script strips role and aria-live from the Demo notice. demo_alerts.html.tmpl was fixed after head (491da717). |
| WR-B08 | closes | see NP-1 | 404 with field visit hands over to PinalertDemoEnded (official-alerts.js:805-813, ended-nearby.js:489-497). |
| WR-B09 | closes | none found | official-alerts.js:245-297 sends a stored centre or fix only if under 30 min old. The banner keeps a centre only when own or place is true (official-banner.js:308-315, app.js:159-166, :783). |
| IN-B01 | closes (light) | none found (votes.js is not loaded on /demo/alerts) | votes.js:627-630. |
| IN-B02 | closes (light) | none found | 2000 ms. The limit is 60, so the poll still lasts about 2 min. |
| IN-B03 | closes (light) | none found | votes.js reads `main[data-...]`. |
| IN-B04 | closes | none found | SAFE_BASE is applied in alerts and banner. |
| IN-B05 | closes (light) | none found | login-globe.js:344-350. `.globe-bg` is `visibility: hidden` until `.is-ready` (login-globe.css:52-59, :76-78), so the normal path still loads. |
| IN-B06 | closes (light); name half not reproduced | none found | Comment fixed. The row name check was done by the fixer in both engines, not by me. |
| WR-T02 | closes (light) | none found | U+00A0 in both clockText functions. The amended asserts change only the space. |
| WR-S04-browser | closes (light) | none found | "Recently ended" never claims in force or none in force. |
| IN-S05-browser | closes (light) | none found | GDACS rows show under the unavailable notice, and no SACHET row does. |
| WR-T01 | closes | none found | phone.css:1388-1425, only under 359.98px. The test is additive: every old case is kept, the narrow block is pinned, and no other rule may wrap. The event has `flex: 1 1 0`, so the first line cannot wrap, and the area takes a full line. 360px and wider are untouched. The 46px height was measured by the fixer in a browser, not by me. |
| WR-T03 | closes (light) | none found | TestOfficialCardSpacingBeatsPopupReset. |
| WR-T04 | closes (light) | none found | Comment and test match the painted colour. |
| WR-T06 | closes (light) | none found | TestOfficialRowHoverKeepsMutedTextReadable. |
| WR-T07 | does not close in range | n/a | Blocked by the templates fixer. Fixed after head by 3bc67461: demo_strip.html.tmpl:12 now has no aria-label. That commit is outside this range and not reviewed here. |
| IN-T01 | see WR-B03 | n/a | Duplicate. |
| IN-T02 | closes (light) | none found | 16px label. |
| IN-T03 | recorded, not changed | n/a | Per triage. |
| IN-T04 | recorded, not changed | n/a | Per triage. |
| IN-T05 | closes (light) | none found | |
| IN-T06 | recorded, not changed | n/a | Per triage. |
| IN-T07 | recorded, not changed | n/a | Per triage. |
| IN-T08 | not reproduced | n/a | Nearby has no Past view, and the Demo NearbyReports returns only unexpired reports. Not rechecked by me. |
| IN-T09 | closes (light) | none found | |

Notes (not new problems):
1. WR-S02 and shutdown. `officialSvc.Run(context.Background())` (cmd/server/main.go:467) was never tied to the signal, and that is older than this round. A save can now run up to 10 s after a request or run is cancelled. At exit, `defer pool.Close()` (main.go:172) waits for it, so a save does not outlive the process. Once the pool closes, the saves left in the loop fail at Acquire. No double write is possible: each save is one transaction keyed by guid. A run lasts at most about 70 s, and the claim gap is 4 min.
2. WR-S05 residual. The row cap of 8000 now applies inside the padded district boxes, not nationwide. More than 8000 eligible accounts with lower ids inside a box but outside the district (for example a metro inside a neighbour district's box) would still push district accounts past the cap. This is far beyond today's scale and is within the alternative fix the review offered, so it is not a new problem.
3. WR-B05 title leak. On steps 1 and 3 in WebKit, the first Tab from the step title (tabindex -1, not in the loop) is left to Safari and leaves the flow. The next Tab is caught by `!root.contains(active)` and returns to the first control, so it is a one press leak, not a trap. It is older than this round and recorded by the fixer. Small fix if wanted: treat `active === title` like "before the first item" on a forward Tab and focus `items[0]`.

## New problems

### NP-1 (warning): CR-B01 ends every idle Demo Official alerts visit at about 31 minutes and moves the page to Demo Nearby by itself

**File:** `web/static/js/official-alerts.js:880-905` (answerTooOld, tick), with `:805-813` (WR-B08 handover), `web/static/js/demo.js:233-242`, `internal/demo/store.go:27, 125-139`

**Scenario (verified by reading; not run in a browser):**
1. The Demo answer is always fresh with age 0 (internal/demo/alerts.go:20, 34). So on /demo/alerts, `ageOf` (official-alerts.js:222-227, age_seconds plus elapsed) is just the time since the page drew it.
2. Nothing else on that page touches the world. app-bar.js and demo.js make no requests. So the world's lastUsed is the initial page load and the first API read.
3. The minute tick asks again at the first tick past 1800 s, which is about minute 31. `Store.Get` sweeps first (store.go:132), and sweepLocked drops a world idle for 30 min or more (store.go:149-154), so the world is already gone.
4. The server answers 404 with field visit. load() then calls PinalertDemoEnded, which runs `location.replace('/demo')`.

So a visitor reading Demo Official alerts with the tab in view is moved to a new Demo Nearby world after about 31 minutes. Nobody pressed anything, and the move spends a visit token.

A hidden tab may do the same if the browser keeps running the interval; I did not check that. Before this round the page sat still until Try again. On Demo Nearby the 30 s poll keeps the world alive, so this timing applies only to the alerts page. The CR-B01 limit (1800 s) and the world TTL (1800 s) are the same number, so the first automatic ask always lands on a dead visit.

**Smallest fix:** in official-alerts.js, hand over to PinalertDemoEnded only for a load the reader started (the first load and Try again). For example, pass `byReader` to load() and check it next to `mine === requestSeq`. An automatic ask that hits an ended visit then shows the load error, and the reader's Try again restarts the Demo as WR-B08 intends. Another option: on a page with `data-demo="1"`, have answerTooOld return false, because the Demo sample never ages.

**Test:** official_alerts_test.mjs. Load with the Demo shell (data-demo, data-api-base, fixed point) and a PinalertDemoEnded stub. Answer the first request fresh with age 0. Advance 31 min. Answer the tick's request with 404 and `{error:{field:'visit'}}`. Assert the stub was not called. Then press Try again, answer the same, and assert it was called once.

### NP-2 (warning): WR-B02 closes the card before its pin is removed, so keyboard focus falls to the page body

**File:** `web/static/js/official-pins.js:413-417` (clearAll), `:449-453` and `:460-463` (drawPins), with `web/static/js/map.js:2433-2442` (closeOfficialPopup)

**Scenario (verified by reading; not run in a browser):**
1. A keyboard or VoiceOver user opens a GDACS card with Enter, so focus is inside the card.
2. A 5 minute refresh answers stale, or changes that pin's level.
3. closeOpenCard() runs first. Leaflet's closePopup fires popupclose at once, and closeOfficialPopup sees focus inside the card. The opener is still connected, so it calls `opener.focus()`.
   This assumes Leaflet 1.9 fires popupclose (map.js:389, onPopupClose :1285-1293) inside closePopup. I did not read the Leaflet source. popupFocusInside comes from the focusin listener (map.js:392, :1231-1234).
4. Next, `layer.clearLayers()` or `group.removeLayer(existing.marker)` takes that marker out of the page. Focus is now on a removed node, and `document.activeElement` becomes `body`.

The reader loses their place on the map, and the next Tab starts from the top of the page. closeOfficialPopup already has the right fallback: when the opener is not connected, it focuses the map container (:2440-2441). It just never reaches it, because of the order.

**Smallest fix:** remove the marker first, then close the card:
- In clearAll: clear the layer, then closeOpenCard().
- In drawPins: call `group.removeLayer(...)` before `closeOpenCard()` in both places.

closeOfficialPopup then finds the opener disconnected and focuses the map container.

**Test:** official_pins_test.mjs. Give the fake map a closeCard that records `opener.isConnected` (or the order of the removeLayer and closeCard calls). Open a card, `drawPins({state:'stale'})`, and assert the opener was already disconnected when closeCard ran. A browser check: open the card by keyboard, force a stale refresh, and assert `document.activeElement` is the map container, not `body`.

### NP-3 (info): WR-S06 makes the visit limit fail open when the charge is missing from the context

**File:** `internal/api/handlers/demo_page.go:125-129`

**Issue (verified by reading; not reachable today):**
- `if charge, ok := ...; ok && !charge()` creates a new world without any limit whenever the context carries no charge function.
- All three Demo pages are wrapped today (demo.go:137-143), so there is no live exposure.
- The old code refused in the middleware and was closed by construction. Any future page that calls newDemoPlaceModel or demoVisitFor without `pages.With(...)` would mint worlds with no limit, and evict live visits (WR-S06 item 2).

**Smallest fix:** treat a missing charge as refused (`if !ok || !charge()`). Or pass the charge function as a parameter of demoVisitFor, so the compiler requires it.

**Test:** a handler test that calls DemoAlertsPage without the limiter middleware and with no `?v=`. It should assert 429, or no world created (`store.Len()` unchanged).

### NP-4 (warning): CR-B01's automatic asks blank and rebuild the list, so focus and an open row are lost every few minutes

**File:** `web/static/js/official-alerts.js:774-783` (load calls showLoading), `:339-344` (showLoading hides every block), `:666-667` (renderGroups empties the list), `:895-927` (tick, visibilitychange, pageshow)

**Scenario (verified by reading; not run with a screen reader):**
1. The feed is stale, so the rows show under the stale notice.
2. A keyboard or VoiceOver user is on a row button, or has opened a row (aria-expanded true, :606-607).
3. Every 5 minutes the tick calls load(). showLoading() hides the section, and render() empties `listEl` and builds new row buttons.
4. The focused button is removed, so focus falls to `body`, and the open detail closes.

The same happens:
- at the 30 minute fresh limit;
- each time the tab is shown again after a minute or more away.

The script has no focus() call and keeps no open row id (grep shows no `focus(` or `activeElement` in official-alerts.js). Before this round, only the reader's own Try again rebuilt the list.

**Smallest fix:**
- Keep the blanking for the fresh limit ask only. There the claim must go at once, which is the point of CR-B01.
- For the stale, unavailable and tab shown asks, send the request without showLoading().
- When the answer arrives, note the focused row's id and whether it was open before rendering. Afterwards, reopen that row and put focus back on it if it still exists. Otherwise put focus on the section title.

**Test:** official_alerts_test.mjs. Open with withRows(staleAt()). Focus the first row button and open it. Advance 5 min and answer the request with the same rows. Assert that document.activeElement is the row button with the same id and that its aria-expanded is still "true".

## Weakened test audit

Every removed assertion in the amended tests was checked against its replacement:
- **official_alerts_test.mjs:** the stale ageing case now answers the re-asks for 60 ticks and still asserts "3 hours ago". Clock strings changed only the space to U+00A0. Position cases gained savedAt.
- **official_pins_test.mjs:** the card regex now takes U+00A0.
- **official_banner_test.mjs:** the stored centre gained savedAt, and the feed-centre detail gained own and place.
- **welcome_flow_test.mjs:** the four "left to the browser" cases now assert prevented plus the exact next control.
- **view_test.go:** Count checks became listedRows checks plus Count 0.
- **demo_contract_test.go and demo_practice_test.mjs:** 3000 became 2000.
- **official_alerts_contract_test.go:** the role="status" requirement is inverted by decision, and the tag rule moved to demo.css.
- **TestBannerOneLineAndTarget:** additive only.

None of them dropped an old case without a replacement. official_push_test.go changed the call shape. It is a database test and was not run here.

---

_Checked: 2026-10-29. Skeptic, read only. No repo file changed except this one._
