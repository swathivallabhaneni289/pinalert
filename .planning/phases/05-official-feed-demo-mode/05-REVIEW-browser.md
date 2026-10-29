---
phase: 05
part: browser
reviewed_at: 2026-10-29T00:00:00Z
depth: standard
base: 1f44177d
status: issues_found
findings:
  critical: 1
  warning: 9
  info: 6
  total: 16
files_reviewed_list:
  - web/static/js/api-base.js
  - web/static/js/app.js
  - web/static/js/demo.js
  - web/static/js/ended-nearby.js
  - web/static/js/feed.js
  - web/static/js/login-globe.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/static/js/official-alerts.js
  - web/static/js/official-banner.js
  - web/static/js/official-pins.js
  - web/static/js/report-form.js
  - web/static/js/settings-alerts.js
  - web/static/js/sw.js
  - web/static/js/votes.js
  - web/static/js/welcome.js (item a only)
---

# Phase 5 code review: the browser scripts

Scope: the Phase 5 changes (commits tagged `(05-` and quick 261029-47j) in the 15 scripts under `web/static/js`, plus `welcome.js` for item (a), and how they work with the code around them. I read the templates, CSS and Go handlers only where a script depends on them. Review only: no repo file was changed apart from this one.

The node suites I ran all pass: official_alerts 48, official_banner 31, official_pins 22, demo_practice 17, demo_page 13, login_globe 34, pwa_sw 22 and welcome_flow 70. None of the findings below is covered by those tests.

Each finding is marked "Verified" (traced in the code) or "Unverified" (what still needs a browser).

## Critical Issues

### CR-B01: Official alerts keeps saying "No official alerts in force" and "In force now" after the answer has gone stale

**File:** `web/static/js/official-alerts.js:613-657, 694-698, 708`

**Issue:** The page asks the server once, on load (708), and again only when someone presses Try again (693). The once-a-minute timer (694-698) calls only `renderStatus`. That updates the status line and the two age sentences, but it never decides the page's state again.

- A fresh and empty answer therefore keeps "No official alerts in force for your area" on screen for as long as the tab is open.
- A fresh answer with rows keeps its "In force now" heading and its rows, including rows whose Until time has already passed.

No `visibilitychange` or `pageshow` handler exists either (grep confirms), so a phone tab restored hours later shows the same claim.

UI-SPEC 1b allows the claim only while the last success is under 30 minutes old. 03.4 T-34-17 says the page must never claim no alert is in force when the feed cannot vouch for it.

**Scenario:**
1. At 9:00 pm the reader opens Official alerts and gets fresh and empty: "No official alerts in force for your area", "Last checked 4 minutes ago".
2. The phone sleeps with the tab open.
3. At 3:00 am a cyclone warning is issued for their district.
4. At 6:00 am they look again. The status line reads "Last checked 9 hours ago", the heading still says no alert is in force, and the warning is never fetched.

The same happens to a stale answer, which never moves on to unavailable, and to "In force now" rows that have expired. Verified by reading.

**Fix (smallest):**
- In the minute tick, once `ageOf(current, elapsed)` reaches the 30 minute fresh limit (or once an hour has passed since `renderedAt`), call `load()`. Do the same when a shown row's `expires_at` has passed.
- Also call `load()` on `visibilitychange` to visible and on `pageshow` with `persisted`, when the answer is older than a minute.
- At the very least, the tick must hide `official-empty` and the "In force now" heading once the age passes 30 minutes. Never leave them up waiting for the next request.

**Test:**
- In `official_alerts_test.mjs`, open with `view()` (fresh, empty, age 240 s), call `page.advance(60000)` 27 times, then assert that either a second request was sent or the visible text no longer matches `/no official alerts in force for your area/i`.
- Add the same check for `withRows()` and "In force now".

## Warnings

### WR-B01: No time limit on the three official requests; a hung pins request freezes the GDACS layer

**File:** `web/static/js/official-pins.js:436-457`, `web/static/js/official-banner.js:200-223`, `web/static/js/official-alerts.js:661-691`

**Issue:** None of the three scripts uses `AbortController` or a timer (grep confirms). `app.js` adds `requestTimeout` to every list read because, as its own comment notes, WebKit can leave a request's promise unsettled. What each script does while its request hangs:

- **`official-pins.js`:** sets `requestOut = true` before the fetch and clears it only when the fetch settles. While it hangs, the 5 minute timer and the feed-centre cue do nothing (437), and the last fresh pins stay drawn as current.
- **`official-banner.js`:** keeps showing the last alert. After a place change it shows the alert for the old place until the hung request ends, because `onCentre` never hides the banner when the place changes (264-283).
- **`official-alerts.js`:** keeps the skeleton and "Checking for official alerts…" on screen.

**Scenario:** On an iPhone, the reader switches apps while a pins refresh is in flight. The request never settles, `requestOut` stays true, and the GDACS hazard pins drawn before keep showing for the life of the page, even after GDACS has ended the event.

Verified: no timeout exists, and the `requestOut` gate blocks every later load. Unverified: whether a plain (not aborted) fetch can stay unsettled forever in today's WebKit, or only until the browser's own network timeout of a minute or more. Either way the layer is frozen for that time.

**Fix:**
- Reuse the `app.js` pattern: an `AbortController` plus a 10 second timer that rejects.
- On that rejection, pins call `clearAll()` and reset `requestOut`, the banner calls `hideQuiet()`, and the page calls `renderError()`.
- In `official-banner.js` `onCentre`, call `hideQuiet()` as soon as `wanted` differs from `fetched`, so an alert never sits over a place it was not fetched for.

**Test:**
- Node: answer no request for 10 s of fake time. Assert that pins clear and the next `loadPins()` sends a new request, that the banner is hidden, and that the page shows the load error.
- Banner: send a feed-centre event for a new place and assert the banner hides before the new answer arrives.

### WR-B02: An open GDACS card stays open after its pin is removed or the feed goes stale

**File:** `web/static/js/official-pins.js:383-388, 393-432, 453-456`

**Issue:** `clearAll()` (used for a stale or unavailable answer, a failed request or a bad shape) and the per-pin removal in `drawPins` remove markers only. The card is a separate `L.popup` opened by `PinalertMap.openCard`, and nothing closes it.

**Scenario:** A reader opens the card for "Cyclone in the Bay of Bengal, Red". The next refresh answers `state: "stale"` and the layer empties, but the card stays open with its Issued and Until lines, as if the hazard were current. The same happens when a refresh changes the pin's key (for example Orange to Red): the old marker is replaced, and the open card still says Orange, with an opener element that is no longer in the page.

Verified by reading.

**Fix:**
- Keep the id of the open pin in `openFor`.
- In `clearAll()`, and wherever a redraw removes or replaces that id, close the card through the map: add a small `PinalertMap.closeCard()` seam that calls `map.closePopup(officialPopup)`.

**Test:** In `official_pins_test.mjs`, open a card, then `drawPins({state: 'stale'})`. Assert the map's close was called (or `openCard`'s `onClose` ran) and no popup remains.

### WR-B03: On a phone the GDACS card is announced as "Report"

**File:** `web/static/js/map.js:1077, 1081, 2369`, and `web/static/css/phone.css:977`

**Issue:** `onPopupOpen` calls `ensureReportHeading(container, close)`, which inserts an `h2` reading "Report", before the official branch at 1081. `openOfficialPopup` then calls `focusReportHeading(container)` on a phone (2369). The heading is clipped visually but kept for screen readers, so VoiceOver first says "Report, heading level 2" for a GDACS hazard. UI-SPEC 3b says the card is "plainly a different thing", and ROBUST-06 says official items must never be mistakable for crowd reports.

**Scenario:** A VoiceOver user taps the GDACS rectangle. Focus lands on "Report", and only after that do they reach "Earthquake in Vanuatu Islands". Verified by reading. Not checked with a real screen reader.

**Fix:** Let `ensureReportHeading` take the heading words. Call it after the official check, with "Hazard" (or "Official hazard") for an official popup and "Report" otherwise.

**Test:** Contract or node case: open a card through `openCard` with a phone detail stub, then assert that the `.report-view__title` text in the official popup is not "Report".

### WR-B04: Every GDACS card uses the same title id, so a quick switch can give the new card the old name

**File:** `web/static/js/official-pins.js:271-272`, `web/static/js/map.js:2329`

**Issue:** `buildCard` always uses `id="official-card-title"`, and `openOfficialPopup` points `aria-labelledby` at it. In Leaflet 1.9, with `fadeAnimation` on (map.js 339 turns it on unless reduce motion is set), a closed popup stays in the page for 200 ms. The `phone.css` comment near 990 says the same.

**Scenario:** With card A open, the reader taps pin B. For 200 ms both cards are in the page, and A's title comes first in DOM order. B's dialog name resolves to A's title just as focus moves into B, so a screen reader can announce the wrong hazard. Crowd cards avoid this with a counter (`pinalert-popup-title-` plus `popupTitleSeq`, map.js 849).

Verified by reading. The 200 ms removal is Leaflet 1.9 `DivOverlay.onRemove` behaviour and was not observed in a browser.

**Fix:** Give each card its own id, `'official-card-title-' + (++seq)`.

**Test:** Node: build two cards and assert their title ids differ.

### WR-B05: (a) The welcome focus trap leaves out links, so the Disagree box's Try the demo cannot be reached by Tab

**File:** `web/static/js/welcome.js:136, 273-288, 1319-1340`; `web/templates/welcome.html.tmpl:89, 117`

**Issue:** `FOCUSABLE_SELECTOR` is `'button, input, [tabindex]:not([tabindex="-1"])'`, so neither `#welcome-try-demo-1` nor `#welcome-try-demo-2` is in the trap's list. This is confirmed, and it is wider than the 05-18 summary says. That summary claims the links "are reachable by Tab in document order", which is not quite what happens:

- **Step 2, Disagree box, both sizes.** The trap's last item is Log out. Tab from Log out wraps to the first item (`active === last`, 1336), so forward Tab never reaches Try the demo, which comes after Log out. Shift+Tab from the first item jumps to Log out, skipping the link again.
- **If the link gets focus some other way** (a mouse, or a screen reader cursor): the link is not `last`, so the browser's own Tab runs. Steps 3 and 4 are hidden and the alerts `<dialog>` is closed, so focus leaves the page for the browser toolbar, out of an `aria-modal` dialog.
- **Step 1 on a phone** (no side panel): the list holds only Continue. Tab and Shift+Tab from Continue both come back to Continue, so Try the demo is reachable only once, by the first forward Tab from the title.

Verified by reading the trap and the template order: the `aside.welcome__side` comes before the steps, and the closed `dialog#welcome-alerts-ask` follows the root.

**Fix:**
- Change the selector to `'a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])'`.
- Note that the fake `matches()` in `welcome_flow_test.mjs` (around line 88) treats `a[href]` as the tag name `A[HREF]` and never matches it. The fake needs one line for `a[href]`, and `buildPage` needs to create the two links.

**Test:**
- Step 2: open the Disagree box, focus Log out, press Tab, and expect focus on `welcome-try-demo-2`. Press Tab again and expect a wrap to the first item.
- Phone step 1: focus Continue, press Shift+Tab, and expect focus on `welcome-try-demo-1`.

### WR-B06: The stale notice is a live region that changes every minute, so screen readers hear it every minute

**File:** `web/templates/alerts.html.tmpl:31` and `web/templates/demo_alerts.html.tmpl:56` (`role="status"` on `#official-notice`); `web/static/js/official-alerts.js:353-369, 694-698`

**Issue:** The minute tick rewrites `noticeTextEl` every time. For a stale answer between 30 and 60 minutes old, the words change every minute ("since 2:05 pm (45 minutes ago)", then 46, and so on), so a polite live region announces once a minute while the reader is on the page. UI-SPEC "Interaction and Accessibility" says the status line is not live and asks for no chatter. The file's own header says "with no new request and no live region".

Verified by reading. Not heard with VoiceOver.

**Fix (pick one):**
- Drop `role="status"` from the notice. The page already shows it on load, and nothing needs to be announced later.
- Or leave the age out of the live sentence and keep the ticking age in the plain status line only.

**Test:** Contract test: `#official-notice` has no `role="status"`. Or a node case: after `advance(60000)` on a stale answer, the live element's text is unchanged.

### WR-B07: If api-base.js fails, the Demo talks to the real API while still using the pretend position

**File:** `web/static/js/app.js:96-99`, `votes.js:253-272, 291-299`, `modal.js:133-147`, `map.js:516-541`, `api-base.js:39-44`

**Issue:** The two Demo seams can fail separately.

- The API base comes only from `api-base.js`. If that script fails to load, or the server-written base fails `SAFE_BASE`, `window.PinalertApiBase` is never set, and `app.js`, `votes.js` and `ended-nearby.js` fall back to `'/api'`, the real routes.
- The fixed position is read straight from `data-fixed-lat` and `data-fixed-lon` on `#app-shell` by `map.js`, `votes.js` and `modal.js`, which do not check whether the base was set.
- `window.PinalertDemoPage` is also set only by `api-base.js`, so the Demo tags disappear too.

**Scenario:** A signed in, onboarded person opens `/demo` (allowed by D-03) on a flaky phone connection, and the deferred `api-base.js` request fails while the others load.

1. The page under "Demo. Nothing here is real." shows real reports around Singh Nagar with no Demo tags.
2. Confirm sends a real vote to `/api/reports/<id>/confirm` with the pretend Singh Nagar coordinates (`getVoterLocation` returns the fixed point).
3. Post a report creates a real report at the pretend pin.

That is a real vote cast from a made-up position, against the trust mechanic itself.

Verified client side. Unverified: whether the real gate accepts a session that is partway through the welcome flow. For a fully onboarded account it does.

**Fix (fail closed):**
- Have `apiBase()` in `app.js`, `votes.js` and `ended-nearby.js` return no base, and send nothing, when the page carries `data-demo="1"` (or a fixed position) but `PinalertApiBase` is unset.
- Alternatively, make each `fixedPosition()` return null unless `window.PinalertApiBase` is set, and make `app.js` refuse to fetch on a `data-demo` page with no base. One shared check in `app.js` that the others call is enough.

**Test:** Node or contract case: load `app.js` and `votes.js` with a shell that has `data-demo="1"` and fixed coordinates but no `PinalertApiBase`. Assert that no request goes to a URL starting with `/api/`.

### WR-B08: When a Demo visit ends, Demo Official alerts and Demo My activity hit a dead end that blames the connection

**File:** `web/static/js/official-alerts.js:297-305, 678-690`; `web/static/js/ended-nearby.js` (the read at about 452); `web/static/js/activity.js:238-242`

**Issue:** Only `app.js` sends an ended visit to `PinalertDemoEnded`, from the feed read and the post. On `/demo/alerts`, the server's 404 with field `visit` becomes "Couldn't load official alerts. Check your connection and try again." Try again asks the same dead visit every time (the visit id is inside the page's `data-api-base`), so it can never succeed.

**Scenario:** The visitor opens Demo Official alerts, which makes one request and then no more. They read for 30 minutes, the world is dropped (`WorldIdleTTL` 30 min), or the server restarts or wakes from the free tier's sleep, which empties the in-memory store. They press Try again and keep seeing a connection error on a working connection. The same thing happens on Demo My activity for the Ended nearby read, Reopen and take back.

Verified by reading. `demo.js` is loaded on both pages but nothing there calls it.

**Fix:**
- In `official-alerts.js`, read the error body on a 404, and when `error.field === 'visit'` and `window.PinalertDemoEnded` exists, call it.
- `demo.js` already sets `PinalertDemoEnded` only on Demo pages, so the real page cannot be affected.
- Do the same in `ended-nearby.js`, and in `votes.js` `postVote` (see IN-B01).

**Test:** Node: load `official-alerts.js` with `PinalertDemoEnded` stubbed. Answer 404 with `{error: {field: 'visit'}}` and assert the stub was called.

### WR-B09: "Where you are" can be a place the reader is not at

**File:** `web/static/js/official-alerts.js:217-258`; `web/static/js/app.js:704` (via `official-banner.js:230-240`); `web/static/js/position.js:67-102`

**Issue:** Two ways the page can send a place that is not the reader's:

1. **An old stored fix.** With no Nearby centre for the tab (for example a new window opened by tapping an official push), `areaSource()` falls back to `PinalertPosition.read()`, which has no age limit even though `savedAt` is available. The server labels any point it is sent "Where you are: <district>".
2. **A fallback centre.** `app.js` announces the feed centre for every list it applies, including the boot fallback centre that is used when location is denied or slow (`map.js` 445, 467; `state.center.own` false). That is the home area cell centre, or the configured city for an account with no home area. The banner stores it as the tab's Nearby centre for this account.

**Scenario:** A reader whose last Nearby fix was in Vijayawada three days ago is now in Chennai. A push for their home area opens `/alerts` in a new window. The page sends the three-day-old Vijayawada point and says "Nothing is in force for where you are or for your home area", although Chennai was never checked.

Verified client side. The "Where you are" label comes from `internal/official/view.go:248`. This is partly by design: UI-SPEC 1 says "the position Nearby last used".

**Fix:**
- Ignore a stored fix older than a bound (for example 1 hour, using `savedAt`), so the server falls back to the home area alone.
- Add `own` to the feed-centre event detail and let the banner store a centre only when `own` is true or the centre is a searched place.

**Test:** Node: give `PinalertPosition.read()` a `savedAt` older than the bound and assert the request has no `lat` or `lon`.

## Info

### IN-B01: (b) A vote on an ended Demo visit shows the server's sentence and does not restart the Demo

**File:** `web/static/js/votes.js:556-605`

**Issue:** Confirmed, and already disclosed in the 05-17 summary (line 115). `postVote` turns the 404 into an error that carries the server's own words, "This Demo has ended. Reload the page to start again.", shown under the controls. It never calls `PinalertDemoEnded`.

On Demo Nearby this is a narrow case. The 30 second list poll keeps the world alive while the page is open, so the visit ends under an open page only after a long background suspend or a server restart. The next poll then restarts the page anyway, possibly while the person is reading the message. The wording is honest and tells them what to do, so this is not a Warning on its own.

**Fix:** In `postVote`, when `res.status === 404` and `fieldError.field === 'visit'`, call `window.PinalertDemoEnded` if it exists. Return the same rejection so the controls stay consistent.

**Test:** Node: with `PinalertDemoPage` true and the stub set, a 404 visit answer to `castVote` calls the stub once.

### IN-B02: (c) The practice climb shows at about 6 s and 9 s, not 4 s and 8 s

**File:** `web/static/js/demo.js:76, 170-182`; `internal/demo/neighbours.go:16`

**Issue:** Confirmed, and disclosed in the 05-17 summary (line 88). The neighbours' votes fall due at the post time plus 4 s and plus 8 s, applied by `ApplyDue` on each read. The 3 second poll starts when `pinalert:report-posted` fires, which is after the post and its refetch, so the reads land at about 3, 6 and 9 s. "One every few seconds" (UI-SPEC 4d) still holds, and the words shown are the server's, so this is a timing gap, not a defect.

**Fix:** Set `PRACTICE_POLL_MS` to 2000, so ticks land shortly after each due time and the climb shows at about 4 s and 8 s. Alternatively, schedule one read just after each due time.

**Test:** `demo_practice_test.mjs`: assert `PRACTICE_POLL_MS` is no more than `NeighbourInterval / 2`. Alternatively, a Go and node parity pin.

### IN-B03: On Demo My activity, votes.js does not see the pretend position and reads the real voter cache

**File:** `web/static/js/votes.js:253-264, 426-432, 1016-1028`; `web/templates/demo_activity.html.tmpl:50-55`

**Issue:** `fixedPosition()` reads only `#app-shell`, but `demo_activity` puts the pretend point on `main.profile-page`. Reopen (`ownReport`) calls `ownReportLocation()`, then `peekVoterLocation()`, which then reads the real sessionStorage voter fix. If the same tab voted on the real Nearby within the fresh window, that real position is sent to the Demo sandbox. Otherwise it sends 0,0. Nothing is written, so the "stores nothing" rule holds, but the seam is inconsistent with `ended-nearby.js`, which reads `.profile-page`.

**Fix:** In `votes.js`, read `#app-shell, .profile-page[data-demo="1"]`.

**Test:** Node: with only the `.profile-page` carrier, `peekVoterLocation()` returns the fixed point and touches no storage.

### IN-B04: official-banner.js and official-alerts.js use the raw data-api-base without SAFE_BASE

**File:** `web/static/js/official-banner.js:186-195`; `web/static/js/official-alerts.js:260-269`

**Issue:** When `PinalertApiBase` is unset, both fall back to the unchecked `data-api-base` attribute. That skips the T-05-74 check `api-base.js` exists for. The value is server written today, so this is defence in depth only.

**Fix:** Run the same `SAFE_BASE` test before using the attribute, or drop the fallback and use `'/api'`.

**Test:** Node: `data-api-base="//evil.example"` with no `PinalertApiBase` sends the request to `/api/official/alerts`.

### IN-B05: The globe's "box of 0" guard does not cover a missing stylesheet, which its comment says it does

**File:** `web/static/js/login-globe.js:100-111, 340`

**Issue:** If `login-globe.css` fails to load, `.globe-bg` has no rules. As a flex item of `.login-gate` (a flex row in `auth.css:13-19`), it is as wide as the credit's text, not 0. So `wrapper.clientWidth > 0`, and on a phone the about 800 KB library and the tiles still download into a map host with no height. The credit is also visible from the first paint. When the stylesheet does apply, `--g` is clamped to at least 140px, so the guard never fires then either.

Verified by reading the CSS. Not run in a browser.

**Fix:** Have the guard check a value only the stylesheet sets, for example `getComputedStyle(wrapper).visibility === 'hidden'` before `is-ready`, or a `--globe-ready-box` custom property.

**Test:** `login_globe_test.mjs`: with no stylesheet rule (visibility not hidden), `startGlobe` injects no script.

### IN-B06: Small leftovers

**File:** `web/static/js/settings-alerts.js:794` and `web/static/js/official-alerts.js:466-547`

**Issue:**
- **Stale comment:** the comment above `openHomeFrom` still says "one of the two no home rows". There are three now.
- **Row button name, unverified:** in `official-alerts.js`, the row button's name is built from adjacent spans with no space text between some of them: the chip word, the Demo tag, the badge word and the issuer name. Depending on the browser's name rules it may read "OfficialIMD Chennai". The banner avoids this with hidden ". " separators. This needs a check of the accessibility tree in a browser.

## Checked and clean

- **(d) Duplicate labels on a phone: clean.**
  - The column head row is `aria-hidden="true"` (`official-alerts.js:576`), so the clipped copy on a phone is never read.
  - The `.official-prefix` labels ("Issued by", "Issued", "Until") are clipped only at 900px and wider (`official.css:500`), where the column head is aria-hidden. They are visible on a phone. Either way each label is read once, so there are no duplicates on either size.
- **sw.js safeTarget:** accepts exactly `/alerts` and `/?report=` followed by 1 to 18 digits. Everything else, including `/alerts/`, `/alerts?x`, `//alerts` and non-strings, becomes `/` (`sw.js:38-40`). Notifications are silent with no vibrate. Clean.
- **DOM safety:** every feed string is written with `textContent` or `Pinalert.setText`: official-alerts rows and details, the banner, the GDACS card and pin name, Demo tags, the toasts and the status line. `lang` is set only from a closed map or `/^[a-z]{2,3}$/`. The GDACS link requires the `https://www.gdacs.org/` prefix plus printable ASCII with no quote, angle bracket, backslash or backtick, and opens with `noopener noreferrer`. No `innerHTML` of server text. `L.divIcon` gets an element, not a string.
- **Banner honesty:** it draws only when `state === 'fresh'` and the pick is Severe or Extreme with words. Any failure or bad shape hides it, and stale answers from older requests are dropped by `requestSeq`. It announces once, and again only when the level rises (subject to WR-B01).
- **GDACS pins:** drawn only when `state === 'fresh'` (subject to WR-B01 and WR-B02). They are on their own layer from `/official/pins` and never touch the store, the list or the counts, and the script is not loaded on any Demo page.
- **Pin keyboard handling:** Enter and Space call `preventDefault` on keydown, so Leaflet's keypress-to-click does not open the card twice, and `stopPropagation` keeps the map's Space handler out.
- **Re-rendering:** the late-element decoration runs once per element, so a re-render adds no second listener.
- **Escape and focus:** Escape closes the card through `trust.js` (`isPopupMarker` sees `data-popup-open` on the opener), and focus returns to the pin.
- **Reduced motion:** the pin growth and the row chevron are instant under reduced motion. Pin boxes are 96 by 44.
- **Demo storage:** the scripts on the three Demo pages write only `theme.js`'s preference key. `position.js` and `queue.js` are not loaded. `official-banner.js` stores a centre only when `data-account-id` is set, and the Demo shells have none. On the Demo page `votes.js` returns the fixed point before any sessionStorage read or write. `map-tools.js` only removes its own focus key. IN-B03 is the one gap, a read on Demo My activity.
- **No real data in the Demo when the scripts load normally:** Demo pages load no `official-pins.js`, `flag.js`, `mute.js`, `queue.js` or `position.js`. Every report URL goes through `apiBase()`. Mute and Flag are suppressed when `PinalertDemoPage` is set. The exception is WR-B07.
- **demo.js:** the visit pattern is checked before it is written into a link. `from` is allowlisted to `welcome`. `v` is dropped from the address with `replaceState`. The practice watch stops on Confirmed, on a new watch and after 40 ticks. The announcement is the server's sentence behind a fixed prefix.
- **app.js poll races:** `fetchReports` drops stale answers (sequence, write floor, view, chip and centre), and the Demo's 3 s poll goes through it. The ended-visit 404 is read only on the Demo page.
- **Login globe:**
  - The `PHONE_QUERY` matchMedia check matches the stylesheet's `(max-width: 899.98px)`.
  - On a phone the box is the CSS-reserved `--g` (140 to 300px).
  - No library download happens before the WebGL2 probe and the size check (see IN-B05 for the missing-stylesheet case).
  - A Low data device gets no globe markup, CSS or script (server gate in `login_gate.html.tmpl:11, 47, 51`).
  - The globe stands still under reduced motion, follows changes to that setting live, and pauses in a hidden tab.
  - Any failure removes the map and keeps the reserved space. The credit stays `visibility: hidden` until ready.
  - Tiles come only from the same-origin `/globe/tiles/`, and the libraries are pinned with SRI and `no-referrer`.
- **Settings switch (05-14):** the official row follows the Critical pattern: the no-home variant, `saveKind` limited to `official_push`, put back on failure, the home note and focus return.
- **report-form.js and modal.js Demo changes:** detached stand-ins for the missing search elements, null guards on the location notice, the fixed pin with no geolocation call, and no duplicate toast. Clean.

---

_Reviewed: 2026-10-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
