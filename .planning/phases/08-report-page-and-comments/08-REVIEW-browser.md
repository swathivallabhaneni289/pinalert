---
phase: 08
part: browser
reviewed_at: 2026-10-29T00:00:00Z
reviewed_commit: 67370b92
base: 88cbe8ad
depth: standard
status: issues_found
findings:
  critical: 0
  warning: 5
  info: 7
  total: 12
files_reviewed_list:
  - web/static/js/report-page.js
  - web/static/js/share.js
  - web/static/js/comments.js
  - web/static/js/map.js (the 08-05 and 08-07 parts)
  - web/static/js/flag.js (the 08-18 comment mode)
  - web/static/js/moderate.js (the 08-18 Comments section)
  - web/templates/share_window.html.tmpl
  - web/templates/share_status.html.tmpl
  - web/templates/comment_delete_confirm.html.tmpl
  - web/templates/index.html.tmpl (script order and the two new includes)
  - web/templates/moderate.html.tmpl (ids only)
---

# Phase 8 code review: the browser scripts

Scope: the Phase 8 commits from 88cbe8ad to 67370b92. I read `report-page.js`, `share.js` and `comments.js` in full. I read the Phase 8 diff of `map.js`, `flag.js` and `moderate.js` in full, with the code around it that it calls: the popup open and close paths, `openCard` and `onMapResize` in `map.js`, `requestTimeout` and `lockPage` in `app.js`, the Escape handling in `trust.js` and the Log out box in `account-menu.js`. I also read the templates these scripts drive. This was a review only: no repo file was changed apart from this one. I created no database and drove no browser.

All node suites pass: 1422 of 1422 tests, including the five Phase 8 suites (86 tests). None of them covers WR-B01 to WR-B05.

Each finding is marked "Verified" (traced in the code) or "Unverified" (still needs a browser or the iPhone simulator), with how sure I am.

No finding reaches Critical. There is no markup sink and no call to `navigator.share`, and every double submit is guarded (see "Checked and clean").

## Warnings

### WR-B01: The comment requests have no real time limit, so a stuck post leaves the box on "Posting…" until a reload

**File:** `web/static/js/comments.js:985-1015` (request), `1308-1336` (submit), `639-668` (onLike), `685-708` (sendDelete), `1072-1115` (loadMore)

**Issue:**
- `request()` builds its own AbortController and 10 s timer. It clears the timer as soon as the headers arrive (`1001`), so `res.json()` has no bound at all.
- The timer itself only calls `controller.abort()`, and the request ends only if that abort settles the fetch promise. `app.js:820-840` records that WebKit can leave an aborted request's promise unsettled for good. That is why `Pinalert.requestTimeout` exists as "the one time limit of every request in the client". `share.js`, `flag.js` and `moderate.js` race against its `expired` promise; `comments.js` does not use it.
- When the promise never settles, `posting` stays true. Post keeps "Posting…" and `aria-disabled`, and Enter is refused by `if (posting ...) return;` (`1309`). The words are kept, but nothing will send them.
- The same happens to `likeBusy` (the Like button stays off), `deleteBusy` and `moreBusy` (Show more comments does nothing). The two list polls heal at the next tick, because `stopLoad` moves the sequence on. The history fetch in `report-page.js:331-367` has the same shape, but it heals at the next 30 s tick.

This is the same class as 07.1 WR-B01.

**Scenario:**
1. On an iPhone, in a flood, the person types "Water at knee height near the bus stand" and taps Post.
2. They switch to WhatsApp while the post is in flight, and iOS suspends the tab.
3. They come back: Post still reads "Posting…", and tapping it or pressing Enter does nothing.
4. Nothing says the comment did not go, so they cannot tell whether it was posted. Only a reload gets them out, and a reload loses the words.

**Fix:** build `request()` on `Pinalert.requestTimeout(COMMENTS_TIMEOUT_MS, 'comments')`:
- race the fetch against `limit.expired`, and race `res.json()` against it too;
- pass `limit.signal`;
- call `limit.clear()` only after the body;
- keep `abort` as `limit.controller.abort()` for `stopLoad`.

**Test:** in `comments_test.mjs`:
- Give it a fetch that never settles, press Post, then advance the fake clock 10 s.
- Expect Post back on with the word Post, POST_ERROR in the composer error and the words still in the box.
- Expect a second press to send a second POST.
- Repeat the same with Like.

**Status:** Verified in code. That WebKit leaves the promise unsettled is Unverified here; the repo's own comment records it. Sure: medium high.

### WR-B02: A GDACS card opened while the laptop report panel is open lands in the detail pane, and the panel's exit then takes focus off it

**File:** `web/static/js/map.js:2613-2642` (openCard), `205-225` (applyPopupPlacement), `309-313` (detailInPane), `421-435` (exitPanel), `441-456` (panelNoteClose), and the `isOfficialPopup` early return in `onPopupOpen` (about `1362-1366`)

**Issue:**
- `openCard` makes a new popup and calls `applyPopupPlacement(popup, null)` before `map.openPopup`. At that moment the report is still open in the panel, so `detailInPane()` is true. The GDACS card gets `pane = 'detailPane'` and `autoPan = false`.
- `map.openPopup` then closes the report, and `onPopupClose` runs `panelNoteClose`, which starts the exit timer.
- `onPopupOpen` returns at `isOfficialPopup` before `panelNoteOpen`, so nothing cancels the timer.
- One turn later `exitPanel` removes `data-report-panel`. Every rule in `report-panel.css` is scoped to that attribute, so the GDACS card is left in the body-level `.report-view-host` with no laptop layout. `replaceAllPlacements` only walks report markers, so the GDACS popup is never placed again.
- Second effect: `exitPanel` finds focus inside `.report-view-host`, because `openOfficialPopup` focused the GDACS card there. It therefore calls `returnFocusToOpener` with the old report's opener. Focus jumps off the card the person just opened and goes to the old report's pin.

This breaks the Phase 5 GDACS card on a laptop.

**Scenario:**
1. On a laptop, the person opens a report's bubble and presses Open report, so the panel takes the list's place.
2. They click the GDACS cyclone pin on the map.
3. The panel closes and the list comes back. The hazard card is drawn in the page-level host at Leaflet's layer offsets, away from its pin, and it does not move with the map.
4. Focus is on the old report's pin, not on the card. A keyboard user's Escape or Tab acts on the wrong thing.

**Fix:** place GDACS popups by `phoneDetail()` alone. For example, give `applyPopupPlacement` a third argument, or have `openCard` set the pane from `phoneDetail()` after the call. Alternatively, have `openCard` run `exitPanel()` synchronously when `panelOpen`, before it places the card. Either way, `exitPanel` should skip the focus return when `activePopup` is the official popup.

**Test:** in `official_card_close_test.mjs`, or a new map host test with the fake map:
1. Call `enterPanel` for a report, then `openCard` for a GDACS pin.
2. Expect `popup.options.pane === 'popupPane'`.
3. After one turn, expect `data-report-panel` gone and focus still on the official card.

**Status:** Verified in code. The visual result is Unverified (no browser run). Sure: high on the pane choice and the focus jump; medium on exactly where the card is drawn.

### WR-B03: A successful post wipes any words typed while it was in flight

**File:** `web/static/js/comments.js:1308-1336` (submit), `1276-1287` (onPostAnswer, 201 branch)

**Issue:**
- `submit` sends `input.value` as it was at the press. The textarea stays editable while `posting` is true.
- On 201, `onPostAnswer` runs `input.value = ''` and `clearReplyTarget(false)`. That drops everything typed during the request, and a reply target chosen during the request too.

The phase's own rule elsewhere is "Your words are still here".

**Scenario:**
1. On a slow connection the person posts "Road closed at the flyover" with Enter.
2. While "Posting…" shows, they type a second line, "Use the service road instead", to send next.
3. The 201 arrives: the first comment appears and the box is empty. The second line is gone with no message.

The same happens if they press Reply on another comment while the first post is out: the reply target and its @username are cleared.

**Fix:** remember the sent text (`var sent = input.value;` in `submit`) and pass it to `onPostAnswer`. On 201:
- If `input.value === sent`, clear the box as now.
- Otherwise remove only the sent prefix (`if (input.value.indexOf(sent) === 0) input.value = input.value.slice(sent.length).replace(/^\s+/, '')`), or leave the box alone.
- Clear the reply target only if it is still the one that was sent (`replyTarget === target`).

The other way is to set `input.readOnly = true` while posting.

**Test:** in `comments_test.mjs`:
- Press Post with a held fetch, set `input.value` to the sent words plus " more", resolve with 201, and expect " more" (trimmed) to stay in the box.
- Choose a new reply target before resolving, and expect it to survive.

**Status:** Verified in code. Sure: high that it happens; medium on how often (it needs a slow post).

### WR-B04: On a report with more than 50 comments, new comments from others never appear once Show more has been used, and your own post lands out of order before it

**File:** `web/static/js/comments.js:853-882` (mergeRoots), `966-982` (render, `if (!loadedMore)`), `1072-1115` (loadMore), `1245-1273` (addPosted)

**Issue:** the server sends top-level comments oldest first, 50 a page (`internal/api/handlers/comments.go:184`). The poll (`load`) only ever reads the first page.

- Before Show more is pressed, `addPosted` appends your own new comment at the end of the list. When you then press Show more, comments 51 and up (all older than yours) are appended after it. Your comment is left in the middle of the list, out of order.
- After Show more, `loadedMore` freezes `more` and `nextAfter` (`975`). Once the last page has come, `more` is false and the button is gone. Every later comment by someone else is on a page the poll never reads, so it never appears.
  - The heading count still goes up, so the count and the list disagree.
  - The card persists across close and reopen (`loadedMore` is never reset), so reopening the report does not help either. Only a reload does.

That defeats the "live" promise of the page on exactly the busiest reports.

**Scenario:**
1. A shelter report has 70 comments. The person presses Show more comments and reads to the end.
2. Over the next ten minutes, five people comment that the shelter is full.
3. The count reads 75, but none of the five comments ever appears, even after the report is closed and opened again.

**Fix:**
- After `loadedMore`, have the poll also read from the last known page start. Simplest: keep `lastPageAfter` (the `after` value of the last page read) and have `tick` request it too, merged with covered `'range'` from that cursor. Or read every page up to the known end on a poll.
- In `addPosted`, when `more` is true, do not append. Set a flag so the next Show more brings it in order, or just call `load()`.
- Reset `loadedMore`, `more` and `nextAfter` in `open()` so a reopen starts from a clean first page.

**Test:** in `comments_test.mjs`:
- Page 1 answers 50 with `more: true` and page 2 answers 20 with `more: false`. Press Show more.
- A later answer for page 2 holds 21 comments. Advance the poll and expect the 21st in the list.
- Also: post with `more` true and expect the new node after the page 2 nodes once Show more has run.

**Status:** Verified in code against the server's documented order. Sure: high on the mechanism; medium on reach (needs more than 50 top-level comments).

### WR-B05: A poll answer that was in flight when a Like answered flips the thumb back to its old state

**File:** `web/static/js/comments.js:639-668` (onLike), `1029-1069` (load), `614-637` (writeComment and writeActions)

**Issue:**
- `onLike` writes the server's answer into `rec.data` but does not move `loadSeq`.
- A list poll sent before the like, and answered after it, carries the old `likes` and `viewer_liked`. `mergeRoots` then runs `writeComment` with that data, and the pressed state and count go back.
- `afterDelete` avoids this by calling `load()` (which bumps the sequence). The like path does not.
- The next tap reads the stale `viewer_liked` and sends the verb the person did not mean. Unlike shows as a fresh like, for example. The display then stays wrong until the next poll, up to 30 s later.

The brief asks for "the count and the pressed state written only from the server's answer". This writes an older server answer over a newer one.

**Scenario:**
1. A poll leaves at t=0 on a slow link.
2. At t=0.2 s the person taps Like; the like answers at t=0.5 s and the thumb fills, showing 4.
3. The poll answers at t=1 s with the snapshot it read before the like: the thumb empties and shows 3.
4. The person thinks the tap failed and taps again. The client sends `like` again, and the server answers that it is liked (or, on a second round of the same race, sends `unlike`).

**Fix:** in `onLike` (and `sendDelete`), call `stopLoad()` before sending, so an older list answer is dropped, and call `load()` after the answer. Or stamp `rec.likeAt = ++likeSeq` and skip the like fields in `writeActions` for an answer whose request left before the stamp.

**Test:** in `comments_test.mjs`, start a poll and hold it, tap Like and resolve it as liked, then resolve the held poll with `viewer_liked: false`. Expect `aria-pressed="true"` and the new count.

**Status:** Verified in code. Sure: high on the mechanism; low to medium on impact (it self-corrects on the next poll).

## Info

### IN-B01: A 409 "comments closed" drops the typed words, as the UI-SPEC says, but against the brief's "keeps its text on every non-201 answer"

**File:** `web/static/js/comments.js:1288-1295`

On 409 with field `comments` the box runs `input.value = ''`. UI-SPEC line 474 sanctions this ("your typed words stay in the box until it is replaced, then are dropped"). The review brief says the composer keeps its text on every non-201 answer.

There is a consequence: a mistaken Mark resolved can be taken back (Phase 3.1), and comments then reopen, but the words are already gone. The fix is one line: do not clear `input.value`, because the hidden form keeps it until comments reopen. For the owner to choose; it does not block.

Verified. Sure: high.

### IN-B02: A post that is accepted but answered late, or answered with an unreadable body, invites a duplicate

**File:** `web/static/js/comments.js:985-1015`, `1276-1305`

- If the 10 s limit fires after the server has stored the comment, the box shows "Couldn't post your comment. Try again." with the words kept, and the next press posts it twice.
- A 201 whose body fails to parse (`body: null`) falls through to POST_ERROR in the same way.

There is no idempotency key. A refresh after an unknown outcome (call `load()` in the catch) would at least show the first copy before the person presses again. Verified.

### IN-B03: The live status lines may not announce a repeat

**File:** `web/static/js/comments.js:309-312` (speakDeleted), `1147-1150` (speak)

Both clear the status line and write it again in the same turn. VoiceOver often merges that into no change, so a second "Comment posted." or "Comment deleted." in a row can go unspoken. Write the empty string, then the words in a `setTimeout(..., 0)` or a `requestAnimationFrame`. Unverified in VoiceOver.

### IN-B04: Flag or Delete on a comment that a poll removes while its box is open leaves focus on the page body

**File:** `web/static/js/flag.js:613-621` (returnFocus, the comment early return); `web/static/js/comments.js:685-708` (sendDelete focuses `rec.del`)

If a moderator removes the comment while the Flag dialog or the Delete box is open, the opener is detached. `returnFocus` then returns early for comment mode. The Delete path's `rec.del.focus()` after a 404 hits a detached node, so focus falls to `body`. In both cases, falling back to the Comments heading would keep the reader in place. Verified, rare.

### IN-B05: The moderation Comments list is rebuilt under an action in flight, and a throw in the chain locks Refresh

**File:** `web/static/js/moderate.js:1118-1131` (applyComments), `1201-1212` (commentActionDone), `744-766` (loadLists)

- `applyComments` clears and rebuilds every row on each refresh. A Keep or Remove still in flight then finishes on a detached `entry`: focus goes to the heading and `commentTotal` drops one more time, so the count can read one low until the next refresh.
- `loadLists` has no rejection handler after `.then(loadComments)`. Any throw in `applyQueue` or `applyComments` therefore skips the last step, so `refreshing` stays true and Refresh stays off. This shape is older than Phase 8, but 08-18 extended it.
- No node suite drives the Comments section.

Verified.

### IN-B06: The history and the comments do not refresh when the tab comes back

**File:** `web/static/js/report-page.js:379-384`; `web/static/js/comments.js:1117-1122`

Both polls skip while hidden, and nothing listens for `visibilitychange`. A person who switches back from WhatsApp sees history and comments up to 30 s old. One `visibilitychange` listener per open page calling `refresh()` (removed on close) would fix it. Verified.

### IN-B07: Small leftovers

- `comments.js:1346`: Safari sends the Enter that ends a composition with `isComposing` false and `keyCode` 229 for some input methods, so that Enter can post early. Add `evt.keyCode !== 229`. Unverified for the keyboards Pinalert's readers use.
- `comments.js:1340-1360`: Escape in the composer with no reply target does nothing, and focus has no way out to close the report by key without a Tab first. This is what UI-SPEC line 572 asks for; noted only.
- `report-page.js:449-455` with `467-478`: `enterPanel` runs `setHosted(true)` (one history and comments load), then closes and reopens the popup (a second load). The first is aborted at once, so the cost is one wasted request per Open report.

## Checked and clean

- **Markup sinks:** none in the six scripts. There is no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write` or `eval`.
  - Every server and user string goes through `Pinalert.setText`, `createTextNode` or `setAttribute`.
  - The `@username` of a reply is a `strong` made with `createElement`, and only when the words start with exactly `@` and `reply_to_author` and a space.
  - The WhatsApp `href` is the fixed `https://wa.me/?text=` plus `encodeURIComponent(title + ' ' + url)`.
  - The public page is `html/template` with no script.
- **Share sheet:** `navigator.share` appears nowhere (grep), and the suite asserts it. WhatsApp and Copy link act only from their own taps, and Copy link calls `clipboard.writeText` inside the tap.
- **Share window:**
  - It opens in the tap's turn, with focus on the title (`tabindex="-1"`), so no button is outlined.
  - It closes only on the X and Escape; there is no backdrop listener.
  - It locks the page with `Pinalert.lockPage` and unlocks before focus goes back to Share.
  - The Tab trap moves one stop at a time for Safari.
  - A late answer is dropped by `sequence`. A 429 keeps Try again away for the server's wait.
- **Delete box:**
  - It is the `.logout-confirm` dialog, with focus on the title and its listeners bound once (`boundDeleteBox`).
  - Only Delete sends anything. Escape closes it the same way the Log out box does (`account-menu.js`).
  - The Escape also reaches `trust.js`, but `closeOneLevel` finds no owner for focus in a body-level dialog, so the report stays open.
- **Escape order:** the Share window and the Flag dialog stop Escape at their own backdrop, and `trust.js` stands aside on `data-modal-open`. In the composer, Escape is stopped at the input (cancelling a reply first), so typed words are never lost by a stray Escape.
- **Double submit:** guarded by `posting`, `likeBusy`, `deleteBusy`, `moreBusy`, the share `sequence`, the flag `sending` and the moderation `entry.busy`.
- **Late answers:** dropped by `seq` (history), `loadSeq` (comments list), `sequence` (Share) and `mine` (Copy link), apart from WR-B05.
- **Composer keeps its words:** kept on 400, 422, 429, the other 409s, 404, offline and a network failure, apart from IN-B01 and WR-B03.
- **Host switch and pin switching (laptop panel):**
  - `hostSwitching` stops the address, the report link and the focus return during `enterPanel`.
  - A pin tap in the panel closes and opens in the same turn, and `panelNoteOpen` clears the exit timer.
  - `exitPanel` returns focus only when it was in the panel or lost.
  - The MutationObserver is released on every close.
  - A resize across 900 px goes through the existing close path.
- **Listeners per open:** none are added per open. The Share window, Flag dialog and Delete box bind once. Each card's page and comments listeners live on that card's own nodes, which go with the card. Removing a card calls `page.close()`, which stops both polls.
- **Phone order:** `arrangeCard` puts Stop updates and Flag before the page node, so the page stays last across resizes.
- **Script order:** `report-page.js`, `share.js` and `comments.js` load before `map.js`, and the page builder looks up `PinalertComments` at card build time, so a failed load only drops that section.
- **Phase 5 banner and 7.1 installed ask:** not touched by these scripts. The new Delete box is a `dialog`, which `installed-ask.js`'s `pageLocked` already sees.
- **Tests:** 1422 of 1422 node tests pass (`node --test web/*.mjs`).
