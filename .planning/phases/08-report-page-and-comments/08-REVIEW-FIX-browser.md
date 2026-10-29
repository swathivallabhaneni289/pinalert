---
phase: 08
part: browser
fixed_at: 2026-10-29T00:00:00Z
review_path: .planning/phases/08-report-page-and-comments/08-REVIEW-browser.md
base: 83f9f49e
iteration: 1
findings_in_scope: 12
fixed: 8
recorded: 4
skipped: 0
status: partial
---

# Phase 8: browser code review fix report

**Source review:** 08-REVIEW-browser.md, triaged in 08-REVIEW.md.
**Scope:** WR-B01 to WR-B05, the cheap IN-B items, IN-B01 recorded as built.

**Summary:**
- Findings in scope: 12 (5 warnings, 7 info).
- Fixed: 8 (all 5 warnings; IN-B02, IN-B06, IN-B07 in its first point).
- Recorded with a reason: 4 (IN-B01 as built; IN-B03, IN-B04, IN-B05 deferred).
- Every fix was test first: a new node test failed on the old code and passes after the fix.

Files changed:
- web/static/js/comments.js, web/static/js/map.js, web/static/js/report-page.js;
- web/comments_test.mjs, web/report_page_test.mjs, web/popup_footer_test.mjs;
- web/official_card_panel_test.mjs, which is new.

No CSS change was needed, and no web/*_test.go was amended. The Go contract pins still hold: `COMMENTS_TIMEOUT_MS = 10000` and `applyPopupPlacement(popup, card)` are kept.

## Fixed

### WR-B01: comment requests have no real time limit

**Commit:** 557eebdb
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:** `request()` now runs on `Pinalert.requestTimeout(COMMENTS_TIMEOUT_MS, 'comments')`:
- the fetch and the body read are both raced against `limit.expired`;
- `limit.signal` goes on the request;
- `clear()` runs only once the body has come;
- `abort` clears the limit and aborts the controller, with a guard for a null controller.

Post, Like, Delete and Show more all go through `request()`, so they come back at 10 s.

**Tests:** the sandbox now runs the real `requestTimeout`, cut from app.js. The four new tests cover:
- a post whose aborted request never settles (the WebKit case);
- a like whose aborted request never settles;
- a like whose body read never ends;
- Show more whose body read never ends.

Each test expects the control back on, the error line shown, the words kept, and a second press that sends again.

The history fetch in report-page.js keeps its own timer. It never clears that timer at the headers, and it heals at the next 30 s tick, so it was left as is.

### WR-B02: a GDACS card opened from the laptop panel lands in the detail pane

**Commit:** e3f54f3d
**Files:** web/static/js/map.js, web/official_card_panel_test.mjs (new), web/popup_footer_test.mjs

**Applied fix:**
- `openCard` places the card by `phoneDetail()` alone, so it opens in the popup pane with its pan on a laptop even while the panel is open. The card is the Phase 5 laptop bubble, so `replaceAllPlacements` not walking it no longer matters.
- `exitPanel` no longer returns focus to the old report's opener when the GDACS card is the open popup.
- The phone path is unchanged: the card still opens in the detail pane sheet with no pan.

**Tests:** official_card_panel_test.mjs cuts `applyPopupPlacement`, `detailInPane`, `exitPanel`, `panelNoteClose`, `panelNoteOpen`, `isOfficialPopup` and `openCard` out of the real map.js. It then opens a card while the panel is open and checks:
- the pane and the pan;
- that after the exit turn the panel is gone, nothing was sent to the old pin, and focus stays on the card;
- the focus guard on its own, with the card inside the host;
- the phone placement.

The popup_footer_test.mjs harness gained an `isOfficialPopup` stub and an `activePopup` value for its slice of `exitPanel`, with a dated note.

**CSS:** none needed, so nothing is handed to the templates fixer. Whether the bubble is drawn in the right spot is still to be checked in a browser.

### WR-B03: a successful post wipes words typed while it was in flight

**Commit:** 54e990a9
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:** `submit` remembers the sent words. On 201, `clearSent`:
- empties the box when it still holds exactly the sent words;
- when the box starts with the sent words, removes only them, plus the spaces that follow;
- leaves a box that was rewritten alone;
- clears the reply target only when it is still the one the post was sent to.

**Tests:**
- extra words typed during the post stay;
- a rewritten box is left alone;
- a reply target chosen during the post survives;
- a reply that was sent still clears its own target and the box.

**Status:** fixed: requires human verification. This is a logic change to the composer's clearing rule.

### WR-B04: new comments stop appearing after Show more, and your own post lands out of order

**Commit:** 16967c41
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:**
- Top level comments are placed by id (`insertRoot`). The server pages and orders them by id, oldest first, so a later page and your own post always land in order.
- `mergeRoots` now takes the page's `after` cursor and drops stale comments only inside (after, last id] of that page, so merging page 2 no longer touches page 1.
- Once Show more has been used, every poll also reads the last page Show more read (`loadTail`, chained after the first page). `loadTail` takes over `inflight`, so `stopLoad` ends it. The last page's answer sets `more` and `nextAfter`, so a last page that grew past 50 brings Show more back.
- `loadMore` writes its page through the same merge.
- A cleared list (skeleton or error) resets `loadedMore` and `lastPageAfter`, so it starts again from the first page.

`open()` does not reset the paging. The poll now reads the tail, so reopening the report shows new comments without dropping the pages already read.

**Tests:**
- after Show more, a 71st comment on page 2 appears at the next poll, last in the list;
- a last page that grew past 50 brings Show more back with the new cursor;
- your own post on a long report sits after page 2 once Show more has run.

**Status:** fixed: requires human verification. This changes the paging logic.

### WR-B05: an in-flight poll flips a Like back

**Commit:** 8ada4b90
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:** this uses the stamp approach, not stopLoad plus load, so it costs no extra request.
- Each Like answer stamps its comment (`answerStamp`, `rec.likeStamp`).
- Every list request records the stamp when it leaves: the first page, the tail and Show more.
- `freshData` keeps the comment's own `likes` and `viewer_liked` when the list answer left before that comment's latest Like answer. This holds for roots and replies alike.

A list answer that left before the Like, or while it was out, can no longer flip the thumb back. Show more is covered as well. An answer that left after the Like is written as it comes.

**Tests:** three orders:
- a poll sent before the like and answered after it;
- a poll sent while the like was out;
- a poll sent after the like answered, which is written as it comes.

**Status:** fixed: requires human verification. This is a logic change to the late-answer rule.

### IN-B02: a post with an unknown outcome invites a duplicate

**Commit:** 55a8ac98
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:** a post with no readable answer calls `load()` after the error line:
- a time out or network failure, in the catch;
- a 201 with an unreadable body, or a status the box does not word, in the fallthrough.

A comment the server did store then shows before a second press. The words stay in the box. There is still no idempotency key; that would be a server change.

**Tests:** a post that timed out asks for the list and shows the stored comment, and a stalled 201 body asks for the list too.

### IN-B06: history and comments do not refresh when the tab comes back

**Commit:** 39ec977f
**Files:** web/static/js/comments.js, web/static/js/report-page.js, web/comments_test.mjs, web/report_page_test.mjs

**Applied fix:** each script adds a `visibilitychange` listener when its poll starts and removes it when the poll stops. The listener runs the same `tick`, which loads only when the tab is visible and the page is showing. No listener is added per open beyond that one.

**Tests:** the fake documents gained `addEventListener` and `removeEventListener`. The new tests check:
- a visible tab asks again and a hidden one does not;
- close removes the listener;
- two opens keep one listener.

### IN-B07 (first point): the Enter that ends a Safari composition posts early

**Commit:** cbbb9c57
**Files:** web/static/js/comments.js, web/comments_test.mjs

**Applied fix:** Enter posts only when `evt.keyCode !== 229`, as well as the existing checks.

**Test:** an Enter with keyCode 229 and `isComposing` false sends nothing, and a plain Enter posts.

The other two IN-B07 points are left as they are:
- Escape in the composer is what UI-SPEC line 572 asks for.
- The one wasted request per Open report in `enterPanel` costs a request, not correctness.

## Recorded, not changed

### IN-B01: a 409 "comments closed" clears the typed words

**File:** web/static/js/comments.js (onPostAnswer, the 409 `comments` branch)
**Reason:** stays as built, per the triage in 08-REVIEW.md. UI-SPEC line 474 allows the 409 closed answer to clear the box.

### IN-B03: the live status lines may not announce a repeat

**File:** web/static/js/comments.js (speak, speakDeleted)
**Reason:** deferred. Writing the words a turn later changes when every status line is written, including the counter's spoken bands, so many existing status tests would have to change. The effect is also unverified in VoiceOver. It is better done with a VoiceOver check in the simulator.

### IN-B04: Flag or Delete on a comment a poll removes leaves focus on the body

**Files:** web/static/js/flag.js (returnFocus), web/static/js/comments.js (sendDelete)
**Reason:** deferred, and rare. A full fix touches the Flag dialog's comment-mode return path as well as the removal paths in comments.js. No node suite drives that combination yet.

### IN-B05: the moderation Comments list is rebuilt under an action in flight

**File:** web/static/js/moderate.js (applyComments, commentActionDone, loadLists)
**Reason:** deferred. No node suite drives the moderation Comments section, so a change could not be made test first within this round. The missing rejection handler after `.then(loadComments)` predates Phase 8.

## Checks

- `node --test web/*.mjs`: 1445 pass, 0 fail.
- `go test ./web/`: ok.
- Full Go suite, run once on the scratch database p8_fix_b: 0 FAIL, 20 packages ok, 6192 PASS lines, 3 SKIP. The database was created before the run and dropped after.
- Kept as before:
  - no markup sink was added;
  - `navigator.share` is not called;
  - the Delete box and the Share window are untouched;
  - the Phase 5 GDACS pins, banner and focus return are kept, now with the laptop panel case fixed;
  - the 7.1 installed ask is not touched.
- The added lines hold no en or em dash.

---

_Fixed: 2026-10-29_
_Fixer: Claude (gsd-code-fixer, browser part)_
_Iteration: 1_
