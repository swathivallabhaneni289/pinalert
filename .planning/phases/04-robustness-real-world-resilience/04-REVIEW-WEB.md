---
phase: 04-robustness-real-world-resilience
reviewed: 2026-10-16T00:00:00Z
depth: deep
files_reviewed: 38
files_reviewed_list:
  - web/static/css/auth.css
  - web/static/css/chrome.css
  - web/static/css/feed.css
  - web/static/css/legal.css
  - web/static/css/low.css
  - web/static/css/main.css
  - web/static/css/modal-map.css
  - web/static/css/modal.css
  - web/static/css/moderate.css
  - web/static/css/phone.css
  - web/static/css/trust.css
  - web/static/js/account-menu.js
  - web/static/js/activity.js
  - web/static/js/app.js
  - web/static/js/feed.js
  - web/static/js/flag.js
  - web/static/js/low.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/static/js/moderate.js
  - web/static/js/queue.js
  - web/static/js/report-form.js
  - web/static/js/slow-notice.js
  - web/static/js/trust.js
  - web/static/js/visibility.js
  - web/static/js/votes.js
  - web/templates/account_header.html.tmpl
  - web/templates/disclaimer.html.tmpl
  - web/templates/disclaimer_bar.html.tmpl
  - web/templates/flag_dialog.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/login_gate.html.tmpl
  - web/templates/low.html.tmpl
  - web/templates/moderate.html.tmpl
  - web/templates/policy.html.tmpl
  - web/templates/profile.html.tmpl
  - web/templates/report_form.html.tmpl
  - web/templates/verify_outcome.html.tmpl
findings:
  critical: 0
  warning: 9
  info: 13
  total: 22
status: issues_found
---

# Phase 4: Code Review Report (web half)

**Reviewed:** 2026-10-16
**Depth:** deep
**Files Reviewed:** 38
**Status:** issues_found

## Summary

Scope: the vanilla JavaScript, CSS and html/template files changed since 2a765a4 (the web half of Phase 4). Read every listed file, traced the calls into app.js, votes.js, trust.js and map-tools.js, and read the server handlers where a JSON shape, a status code or a stored field crosses the boundary (internal/api/handlers/reports.go, internal/service/report.go, internal/service/modworker.go, internal/store/queries/moderation.sql). `node --test web/queue_logic_test.mjs` passes (4 of 4).

No script injection was found. Every markup sink grep (innerHTML, insertAdjacentHTML, outerHTML, document.write) is empty across the changed JavaScript; report text, server sentences, geocoder text, flag labels and score keys all reach the page through `Pinalert.setText` or `setAttribute`; the only href built from data is a numeric id (moderate.js), a closed path pair (activity.js, checked again by `safeResendHref`) or `tel:` plus a number. The templates put data only in text and quoted attribute contexts. Every localStorage and sessionStorage access is inside try/catch; one IndexedDB feature test sits outside its try (IN-W04). No em dash or en dash was added to any listed file.

The weightiest problems are layout and robustness, not injection:

- The 390 px suspicion is settled from the CSS. The unofficial bar itself does not cover the place search, because the shell starts below the sticky bar. What covers the search is the account trigger: the phone rule that hid it stopped matching when Phase 4 moved the header into `.top-chrome`. The trigger now sits over the Close button of the open search row at 320, 360 and 390 px wide, and the phone theme toggle sits invisibly under the bar (WR-W01).
- The notice card treats the list as hidden on every phone, so its rows cover the visible list (WR-W02).
- A missing flag.js (one of ten deferred requests on a weak link) blanks the main list and the whole Low data list, because the store's notify does not isolate listeners (WR-W04).
- The offline queue has three edges a person can hit: a read failure looks like an empty queue, a discard question is torn down and the report sent anyway, and a saved report can be posted under a different account (WR-W06, WR-W07, WR-W09).

## Warnings

### WR-W01: The phone hide rule for the account trigger no longer matches. The trigger covers Close on the open place search, and the phone theme toggle sits under the bar

**File:** `web/static/css/phone.css:71`, `web/static/css/phone.css:77-81`, `web/static/css/phone.css:149-151`, with `web/templates/index.html.tmpl:26-30`, `web/static/css/auth.css:324-328`, `web/static/css/chrome.css:12-17`

**Issue:** At base 2a765a4, `account_header.html.tmpl` and `theme_toggle.html.tmpl` were direct children of `<body>` (old index.html.tmpl lines 23 and 24). Phase 4 wrapped the header in `<div class="top-chrome">`, but phone.css still hides it with `body > .app-header { display: none; }`, which now matches nothing. The same applies on the Activity page, which also loads phone.css. Geometry below 900 px, with W the viewport width and B the bar's bottom edge (67.8 px at default text):

- `.app-header` is `position: absolute; top: calc(100% + 16px); right: 16px` inside `.top-chrome` (sticky, z-index 1050). The 44 px trigger therefore spans x W-60 to W-16 and y B+16 to B+60, above everything in `#app-shell`.
- The tools row sits at y B+8 to B+52. Collapsed, it ends at `right: calc(16px + 44px + 8px)`, so x W-68, and is clear of the trigger.
- Open (`[data-search-open="true"]`), it takes `right: var(--space-md)`. Its last control, `.map-search__close`, then sits at x W-60 to W-16, the same 44 px band as the trigger, and 36 of its 44 px overlap the trigger vertically.

Scenario: a phone at 320, 360 or 390 px opens the place search and taps the cross. The tap lands on the account trigger and opens the account menu. The search row cannot be closed by tap (a phone has no Escape key), so the category chips stay hidden behind the open row.

Separately, `body > .theme-toggle { top: 16px; right: 16px }` (fixed, z-index 1000) now sits entirely inside the bar (y 16 to 60 against a bar of at least 67.8 px at z-index 1050). It cannot be seen or tapped, but it is still in the tab order, so a keyboard user's focus disappears under the bar (WCAG 2.4.11). The map tools still leave a 68 px slot on the right for a toggle that is no longer there.

**Fix:** Point the hide rule at the new parent, and settle the toggle the way the owner decided (phones follow the device theme):

```css
@media (max-width: 899.98px) {
  .top-chrome > .app-header { display: none; }
  body > .theme-toggle { display: none; } /* owner: phones follow the system theme */
}
```

If the trigger is meant to stay on phones (P4-A4 assumes it does), give the open search row `right: calc(var(--space-md) + var(--touch-target-min) + var(--space-sm))` instead, and move the toggle below the bar with `top: calc(var(--disclaimer-bar-height) + var(--space-md))`. Add a contract test that the phone hide rule's selector matches the header's real parent in index.html.tmpl.

### WR-W02: On every phone the notice card treats the list as hidden, so its Queue and Stale rows cover the visible list

**File:** `web/static/js/feed.js:967-970` (`listPaneHidden`), used by `syncNoticeCard` at `web/static/js/feed.js:997-1013`

**Issue:** `listPaneHidden()` returns true when `#app-shell[data-view="map"]` and the viewport is narrow. That was main.css's model. Phase 3.1's phone.css (lines 46-58) shows both panes "whatever data-view says", and the view toggle is `display: none` on phones, so `data-view` stays at its template value `map` for the whole visit.

Scenario: a phone at 390 px has one saved report, or a poll fails after the first load.
- `syncNoticeCard` shows the fixed notice card (bottom 84 px, left 16 px, right 84 px) with "1 waiting to send." and Show waiting reports, or "Last updated N minutes ago."
- The same facts are already visible in the list pane, as the queue strip and the stale line.
- The card covers the lower list rows. The list's bottom padding (`calc(44px * 2 + 8px + 32px)`, 128 px) is shorter than the card's top edge (84 px plus about 100 px of card), so the last row cannot be scrolled clear.
- Pressing Show waiting reports sets `data-view="list"`, which on a phone changes nothing visible.

**Fix:** Ask the layout instead of the attribute:

```js
function listPaneHidden() {
  var pane = document.querySelector('.pane--list');
  return !!pane && pane.getClientRects().length === 0;
}
```

Also call `syncNoticeCard` from a `resize` listener, since the answer now depends on layout.

### WR-W03: A post still in flight is applied to a form that was closed or reopened, and can wipe a new draft

**File:** `web/static/js/report-form.js:849-876` (`handleSubmit`), with `requestClose` at `:230`, `closeForm` at `:712`, `resetForm` at `:561`

**Issue:** While a post is out (up to 10 s, `SUBMIT_TIMEOUT_MS`), only the submit button is disabled. Cancel report, Escape and a tap on the backdrop still work. The promise handlers are not tied to the submission they belong to.

Scenario on a weak link:
1. The person taps Post report. The request hangs.
2. After a few seconds they press Escape, then Discard, and the form closes (`resetForm` sets `currentRequestId = null`).
3. They tap + again and start typing a new report.
4. The first request now answers 201. The handler calls `closeForm()`, which hides the new form and runs `resetForm()`, so the new description is gone without any discard question. The toast says "Report posted.", about the report they discarded.

If the first request instead fails as queueable:
- `saveToQueue` reads the new form's `currentRequestId` (null), and `PinalertQueue.save` rejects "not valid".
- The new form shows "Something went wrong. Try again." for a report the person has not sent.

On a 400 or 422, the old server sentence is written into the new form, and `focusField` moves focus there.

**Fix:** Bind each outcome to its submission and drop outcomes for a form that has since closed. Alternatively, block closing while posting.

```js
var submissionSeq = 0;
function handleSubmit() {
  ...
  var mySeq = ++submissionSeq;
  var myRequestId = currentRequestId;
  Pinalert.submitReport(payload).then(function (report) {
    if (mySeq !== submissionSeq || backdropEl.hidden) { /* still discard the queue row, still toast */ return; }
    ...
  }, function (err) {
    if (mySeq !== submissionSeq || backdropEl.hidden) { return; }
    ...
  });
}
// closeForm(): submissionSeq += 1;
```

Pass `myRequestId` to `saveToQueue`, rather than reading the shared variable.

### WR-W04: If flag.js fails to load, the main list and the whole Low data list stop rendering, because notify does not isolate listeners

**File:** `web/static/js/low.js:305`, `web/static/js/map.js:574`, `web/static/js/app.js:214-218`

**Issue:** `createLowRow` and `buildPopupCard` call `PinalertFlag.createFlagControl(report)` with no guard. Each of the ten deferred scripts is a separate request, and the Low data view exists for links where single requests time out. If `flag.js` is the one that fails, `PinalertFlag` is undefined:

- **Low data view:** `renderLow`, the only subscriber, throws in the first `createLowRow`. The throw escapes `notify()`, which loops over listeners with no try, and lands in `fetchReports`' catch. That marks the store `error` with `hasLoaded` already true and notifies again, which throws again. The page shows an empty list under "Last updated just now. Trying again.", and every 60 s poll repeats it. Nothing can be read, voted on or flagged.
- **Main page:** `upsertMarker` adds the first marker to the map, then throws in `buildPopupCard`. Because map.js subscribed before feed.js, the feed's `render` never runs. The list stays on the skeleton and only one pin, with no popup, is drawn.

The same unguarded pattern already exists for `PinalertTrust`, `PinalertVotes` and `PinalertVisibility`. Phase 4 adds one more single point of failure to it, on the page meant to survive weak links.

**Fix:** Isolate listeners, and make the Flag control optional:

```js
function notify() {
  for (var i = 0; i < listeners.length; i++) {
    try { listeners[i](state); } catch (e) { if (window.console) { console.error(e); } }
  }
}
// low.js and map.js
var flagControl = window.PinalertFlag ? PinalertFlag.createFlagControl(report) : null;
if (flagControl) { body.appendChild(flagControl.el); }
// updateLowRow / updatePopupCard: if (row.flag) { PinalertFlag.updateFlagControl(row.flag, report); }
```

### WR-W05: At enlarged text the sticky bar outgrows `--disclaimer-bar-height`. The page then scrolls and the bar covers the map tools, the place search and focused controls

**File:** `web/static/css/main.css:62-67` and `:192-205` (the token), `web/static/css/chrome.css:8-10` and `:12-34`, `web/static/css/phone.css:37-42`

**Issue:** The bar is sticky with `min-height: var(--disclaimer-bar-height)`, so larger text grows it, as intended. The token is a fixed px value per width tier (67.8 px below 480 px). The page shells subtract the token rather than the real bar: `body > .app-shell { height: calc(100dvh - var(--disclaimer-bar-height)) }`, and likewise `#map`, `.login-gate`, the notice card and the popup max heights. No rule sets `overflow` on `html` or `body`, so the excess scrolls the document.

Arithmetic at 390 by 844 with Android Chrome text scaling at 200 percent. Desktop zoom does not trigger this, because the tiers follow CSS width.
- The bar text is 28 px with line-height 1.4, so 39.2 px per line.
- The text is about 112 characters, roughly 1500 px wide against 374 px of content. That gives 4 to 5 lines, or about 165 to 205 px of bar.
- Against the 67.8 px token, the document is about 100 to 137 px taller than the viewport.
- When the person scrolls the page, or the browser scrolls a focused control into view, the sticky bar overlays the top of the shell. That covers the 52 px tools row (search button and chips) and most of the 176 px map strip.
- `html { scroll-padding-top: calc(var(--disclaimer-bar-height) + 8px) }` also uses the token, so a control scrolled into view lands under the taller bar.

**Fix:** Measure the real bar once and on resize, and drive the token from it, as account-menu.js already does for its panel:

```js
// in a small shared script or app.js
var bar = document.querySelector('.disclaimer-bar');
if (bar && typeof ResizeObserver === 'function') {
  new ResizeObserver(function () {
    document.documentElement.style.setProperty('--disclaimer-bar-height', bar.offsetHeight + 'px');
  }).observe(bar);
}
```

The CSS tiers stay as the first paint value. Alternatively, drop `position: sticky` below 900 px, where the shells are full height anyway.

### WR-W06: The queue treats any IndexedDB read failure as "no saved reports", so the reports leave the strip and stop sending until the page is reloaded

**File:** `web/static/js/queue.js:151-170` (`readStored`), `:174-199` (`loadRows`), `:131-137` (no `onclose`)

**Issue:** `readStored` resolves `[]` on `request.onerror` and on any throw from `db.transaction(...)`. `loadRows` then replaces `rows` with the memory rows alone. Every caller reads that as the truth:

- `renderStrip` removes the stored rows from the strip, and hides the strip when none are left.
- `runFlush` finds nothing due.
- `sendRow` returns 'skip'.

The connection is never reopened: there is no `database.onclose` handler, and `db` keeps pointing at the dead connection, so every later `transaction()` throws again. Trigger (unverified on a device): WebKit can close a page's IndexedDB connection after the tab is backgrounded ("Connection to Indexed Database server lost"), and a storage-pressure eviction or a "clear site data" also closes it.

Scenario: a person saves a report with no signal, switches apps, and returns once the signal is back. The strip has vanished and nothing sends, though the report is still on disk. The help line promises "Reports send by themselves when you have a signal and this page is open." A new save in this state drops to memory only and shows the memory toast.

**Fix:** Keep the last known rows on a failed read, and reopen once:

```js
function readStored() {
  return new Promise(function (resolve) {
    ...
    request.onerror = function () { resolve(null); };   // null = unknown, not empty
    ...
  }).then(function (list) {
    if (list !== null) { return list; }
    db = null;
    return openStore().then(function (database) { db = database; return db ? readStoredOnce() : []; });
  });
}
// loadRows: when the read is unknown, keep the previous stored rows instead of dropping them.
// openStore: database.onclose = function () { db = null; };
```

### WR-W07: A periodic flush tears down an open Discard question. Focus drops to the page and the report is sent while the person is confirming the discard

**File:** `web/static/js/queue.js:797-806` (`updateRow` removes `.queue-strip__confirm` and hides the actions), `:872-926` (`openDiscardConfirm`), `:405-449` (`runFlush` picks every waiting row)

**Issue:** The Discard question is a group appended to the row, outside `actionsEl`. Every flush (the 30 s interval, the `online` event, a wake timer) leases each due row. The lease changes the row's state signature, so `updateRow` removes the question and hides the actions.

Scenario:
1. A Waiting row (no signal). The person taps Discard report, and focus is on Keep saved report.
2. The signal returns: `online` fires, and `sendRow` writes `sending_until`.
3. `renderStrip` rebuilds the row. The question is removed while it holds focus, and focus falls to `<body>`, with no fallback because the row itself stays.
4. The report the person was about to discard is posted.

**Fix:** Skip a row whose question is open when choosing due rows, and keep the question across renders:

```js
// runFlush, in the due filter
var openEl = rowEls[record.request_id];
if (openEl && openEl.querySelector('.queue-strip__confirm')) { return false; }
```

Alternatively, have `updateRow` leave an open question in place when the only change is the lease.

### WR-W08: The Low data form asks only for a high accuracy fix, with no default accuracy retry, so indoors it times out where the main form succeeds

**File:** `web/static/js/low.js:735-751` (`requestFix`), `:853-862` (`onRetry`)

**Issue:** `requestFix` always uses `{ enableHighAccuracy: true, timeout: 8000, maximumAge: 0 }`. A cold GPS start or a GPS only provider indoors often cannot answer that in 8 s. Every Retry position repeats the same request. votes.js learned exactly this (WR-07 in its own comment): after a high accuracy timeout or "position unavailable", it tries once more at default accuracy. The main form's picker (modal.js:162-184) uses the default accuracy. The UI-SPEC sets no position options for this block.

Scenario: a phone indoors on the Low data view.
- The location block goes Asking, then Timed out.
- The place search needs the weak network and fails, which moves the block to Both failed.
- The only way forward left is Open the full version, which is the heavy page the person came here to avoid.
- The same phone would have given a coarse network fix in under a second.

**Fix:** Mirror votes.js:

```js
function attempt(high) {
  navigator.geolocation.getCurrentPosition(function (pos) { onFix(seq, pos); }, function (err) {
    if (high && !(err && err.code === err.PERMISSION_DENIED)) { attempt(false); return; }
    onFail(seq, err);
  }, { enableHighAccuracy: high, timeout: LOCATION_TIMEOUT_MS, maximumAge: high ? 0 : 60000 });
}
attempt(true);
```

The weak state (over 100 m) already asks the person to accept a coarse fix, so nothing is placed silently.

### WR-W09: A saved report can be posted under a different account than the one it was saved for

**File:** `web/static/js/queue.js:147-149` (`belongsToAccount`), `:312-325` (BroadcastChannel, only at mount), `:474-507` (`sendRow`); server side `internal/api/handlers/reports.go:447` (`DisallowUnknownFields`) and `:490` (account from the session)

**Issue:** The queue binds rows to the account id rendered when the page loaded (T-04-14). The server attributes a post to whatever session cookie is sent at send time. The only guard against a changed account is one BroadcastChannel message that a new page posts when it mounts. Browsers without BroadcastChannel (Safari before 15.4) skip the guard entirely, and there is a window before the other page mounts.

Scenario on a shared phone with two tabs:
1. Tab 1 is rendered for account A and holds a Blocked row (a 429 wait of several minutes).
2. In tab 2, A logs out and B signs in through the magic link. The verify page waits 2 s, then loads `/`.
3. Before B's `/` has mounted its queue (or never, on an older Safari), tab 1's wake timer fires.
4. Tab 1 sends A's report with B's cookie, and the server stores it as B's report.

Nothing in the request lets the server notice the mismatch.

**Fix (crosses the boundary):** send the expected account with each queued post and have the server refuse a mismatch. A 409 would let the queue mark the row signed out. Because the decoder uses `DisallowUnknownFields`, the field must be added to `SubmitReportRequest` in internal/api/handlers/reports.go first. A client-only stopgap: before each pass, compare `accountId` with a cheap authenticated read (for example the `account_id` echoed by any gated JSON answer) and set `sessionLapsed` on a mismatch.

## Info

### IN-W01: Policy page copy versus what the code stores (item 9 of the brief; the scores line is already owner item 5)

**File:** `web/templates/policy.html.tmpl:43`, `:28`, `:33`; code in `internal/service/report.go:537-563` and `internal/service/modworker.go:246-289`

**Issue:** Confirmed against the code. `textCheckRow` writes scores inline only for a borderline result. When the background worker finishes a check, `FinishModerationCheck` stores verdict, scores and model for every verdict (pass, borderline, reject). On a late reject, the worker also sets the report to `removed`, with the report row and its text kept. Three sentences therefore state less than the code does:

- Line 43, quoted exactly: "The scores of the automatic check for borderline reports, without the report text." Proposed: "The scores of the automatic check for borderline reports and for any report whose check finished late, without the report text."
- Line 28: "If the text is clearly hateful, harassing or sexual, it is rejected and nothing is stored." This is true only for a check that finishes in time. Proposed addition after it: "If the check finished late, the report is removed instead: it stays stored but shows to no one."
- Line 33 ties "Removed after review" to moderators. A late automatic reject shows the author the same words with no person involved. Either say "after review or after the automatic check" or give the worker's removal its own label.

**Fix:** Edit the three sentences, and include them in the pending legal read.

### IN-W02: Disabling a focused Retry button drops focus, which makes the "move focus to the first row" code dead (browser dependent)

**File:** `web/static/js/low.js:932`, `:938`, `:480-487`; `web/static/js/feed.js:1447`, `:1060-1066`

**Issue:** Retry now sets `disabled = true` on the button that has focus. Under the HTML focus fixup rule (Firefox, and Chromium in current releases; flag.js:383 already notes "A button that was disabled while sending can drop focus"), focus moves to `<body>`. On success, `document.activeElement === staleRetryBtn` is therefore false, and focus is not moved to the first row. On failure, focus stays on the body. Unverified per engine.

**Fix:** Use `aria-disabled="true"` with a click guard (as moderate.js and queue.js do), or move focus before disabling.

### IN-W03: With no stored position, the Low data list waits as long as the location prompt is unanswered

**File:** `web/static/js/low.js:204-212`, `:921-928`

**Issue:** The spec's row "No stored position, fix pending: skeleton rows" is followed. However, the geolocation `timeout` does not count time spent waiting for permission, so an ignored or quiet permission prompt (Chrome's quiet UI) leaves the skeleton and "Finding your position." on screen with no request ever sent. map.js:345 has the same pattern; the Low data page inherits it.

**Fix:** Start a separate 8 s timer that falls back to the default area list (basis "default") while the prompt is still open, and replace that list when a fix arrives.

### IN-W04: The IndexedDB feature test runs outside its try (trigger unverified)

**File:** `web/static/js/queue.js:113`

**Issue:** `typeof indexedDB` reads the global getter before the `try` at line 118. In contexts where that getter throws (for example a sandboxed frame without same origin), `openStore` throws synchronously inside the Promise executor. The promise rejects instead of resolving null, and the `storeReady.then(...)` chain in `mount` has no rejection handler, so the strip never renders. This is a literal miss of "every storage access wrapped".

**Fix:** Move the check inside the try: `try { if (!window.indexedDB) { resolve(null); return; } request = indexedDB.open(...); } catch (e) { resolve(null); }`.

### IN-W05: `.mod-row__state` uses a 12 px corner

**File:** `web/static/css/moderate.css:174`

**Issue:** The house rule is corners up to 8 px. The comment explains it restates the built `.visibility-tag` chip (trust.css:267, also 12 px), so it is consistent with an existing exception, but on a short chip it reads as a pill.

**Fix:** Either record the 12 px chip as an accepted exception in the UI-SPEC, or bring both chips to 8 px.

### IN-W06: The fallback timeout stops the timer when headers arrive, but the comment says it also covers the body

**File:** `web/static/js/app.js:526-544` (`requestTimeout`), `:561`; `web/static/js/flag.js:307-309` and `web/static/js/moderate.js:153-155` (same shape)

**Issue:** On browsers without `AbortSignal.timeout`, `timeout.clear()` runs as soon as the response headers arrive, so a stalled body read has no limit, contrary to "The signal covers the wait for the answer and for its body".

**Fix:** Clear the timer after `res.json()` settles.

### IN-W07: A 429 with a wait of 0 seconds makes the queue resend every second (unverified that the server can send 0)

**File:** `web/static/js/app.js:576-580`, `web/static/js/queue.js:519-525`, `:573`

**Issue:** `retryAfterSeconds` accepts 0. `classify` then sets `next_try_at = now`, and `scheduleWake` wakes after 1 s, which gives a 1 s loop for as long as the server keeps answering 429 with 0.

**Fix:** Clamp to at least 1 s, and apply a floor such as 5 s for queued resends.

### IN-W08: Send now pressed while an automatic pass is starting does not resend Not sent rows

**File:** `web/static/js/queue.js:405-411`

**Issue:** `runFlush(true)` returns the running `flushPromise` when one exists, which drops `manual`. The button is aria-disabled only once `passRunning` is set (after `loadRows`), so in that window the press is accepted but rejected and signed out rows are skipped silently.

**Fix:** Remember a pending manual request and run one more manual pass after the current one settles.

### IN-W09: The moderator Find result region announces the whole row

**File:** `web/templates/moderate.html.tmpl:34`, `web/static/js/moderate.js:713-717`

**Issue:** `#mod-find-result` is `aria-live="polite"` and receives the status line plus the full row (text, scores, buttons, confirmation question), so a screen reader reads everything at once.

**Fix:** Put the live region on a separate status paragraph that holds only "Found report N." or the error sentence.

### IN-W10: The tap to place handler is added again on every open where location is off (carried over)

**File:** `web/static/js/modal.js:190-200`

**Issue:** `showLocationDenied` calls `modalMap.on('click', ...)` on each open. The base file documented this as a known pre-existing bug. After a denied open, a later granted open still places the pin on a map tap, and each tap runs `placeMarker` once per stacked handler.

**Fix:** Register the handler once in `initLocation` when `modalMap` is created, gated on a flag that `showLocationDenied` sets and `clearMarker` clears.

### IN-W11: The queue lease is shorter than the worst case send

**File:** `web/static/js/queue.js:35`, `:490-502`; `web/static/js/app.js:597-603`

**Issue:** `submitReport` resolves only after the POST (up to 10 s) and the post write refetch (up to 10 s). The 15 s lease can expire first. In a browser without Web Locks, a second tab can then resend the same row. The server's request id makes that harmless (200 replay), but both tabs show "Report sent."

**Fix:** Set the lease to at least 25 s, or remove the row before the refetch.

### IN-W12: Double announcements on save and send

**File:** `web/static/js/queue.js:358-362`, `:493-497`; `web/static/js/report-form.js:1010`

**Issue:** Saving announces "N reports waiting to send." through `#notice-live`, and the form shows the save toast in `#toast`, which is also a polite status region. Sending likewise writes "Report sent." to both. Screen readers hear each event twice.

**Fix:** Announce through one region only, and skip `announce` when a toast with the same meaning is shown.

### IN-W13: With scripts blocked or failing, the Low data view shows only its heading and two inert buttons

**File:** `web/templates/low.html.tmpl:30-60`

**Issue:** The UI-SPEC records that the pages need JavaScript, so this is not a defect against the record. Still, the page built for the worst connections gives no sign of why it is empty (the list is `aria-busy="true"` forever, and Report an emergency does nothing).

**Fix:** Add a short `<noscript>` line, or a server rendered sentence that low.js hides at boot, naming the emergency number and saying the list needs scripts.

---

_Reviewed: 2026-10-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
