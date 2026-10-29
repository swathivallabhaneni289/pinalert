---
phase: 08
checked_commit: 27f00c8a
diff_range: 83f9f49e..27f00c8a
reviewed: 2026-10-29
findings:
  blocker: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 8 review fixes: skeptic pass

Read only. I read the server, browser and templates fixes in `83f9f49e..27f00c8a` against the three REVIEW and three REVIEW-FIX files. Tests ran on a database I made for this, `p8_skep`, and dropped afterwards. `go test -count=1 -p 1 ./internal/service/ ./internal/store/... ./internal/api/... ./web/` passed. `node --test` on comments, official_card_panel, report_page and popup_footer passed 82 of 82. I started no server.

## Findings

### SK-01 (warning): the per thread reply cap hides the newest replies, and the client never reads the real count

**Files:** `internal/store/queries/comments.sql:120-130`, `internal/service/comments.go:63`, `web/static/js/comments.js:784` and `:818-851`

WR-S02 caps every thread at `CommentReplyCap = 100`, keeping the oldest replies first. It puts the real count in `reply_count`. But `comments.js` never reads `reply_count`: `writeThread` labels the toggle from `t.replyIds.length`, and `placeReplies` makes the thread "hold exactly the replies of data".

What goes wrong: a busy thread on a cyclone report reaches 101 replies. The toggle says "View 100 replies". The newest replies, which are the ones that matter in an emergency, can never be reached. A reader then posts reply 102. `addPosted` adds it, and the next poll or visibility tick drops it, because the server's capped list does not include it. To the poster, their own reply vanishes about one poll after they post it. The old total cap of 500 had the same shape. The fix lowers the point where it bites for one thread from 500 to 100, and the "real count" it adds has no effect on screen.

**Fix:** pick one and pin it with a test.
(a) Keep the newest replies: `ROW_NUMBER() OVER (PARTITION BY v.parent_id ORDER BY v.id DESC)`, then order the outer result ascending. In `writeThread`, label the toggle from `rec.data.reply_count` and show a line such as "Older replies not shown" when `reply_count > replyIds.length`.
(b) Add reply paging with an after cursor for each thread. Also make `placeReplies` keep any reply id above the highest id the answer returned when `reply_count > replies.length`, the way `mergeRoots` limits removal to the covered range.

### SK-02 (warning): TRUST-MODEL Sharing bullet 3 now contradicts the README and the Policy

**File:** `docs/TRUST-MODEL.md:656` (Sharing, bullet 3)

The fixers changed the README (`README.md:201-203`) and the Policy (`legal_body.html.tmpl:135`) to say the public page shows the severity, "what is happening for a Critical report" and how many people confirmed it. The code agrees: `share.go:283` sets `Happening`, and `share.go:292` uses `view.Trust.ConfirmedSentence`, which `public_status.go:135` renders. Bullet 3 still lists only category, coarse area, status word, sentence, severity and times. It leaves out the What is happening pick, and it says the page "has no field for ... the trust facts", while the sentence carries the confirmation count. This document is the project's written trust model, so a reader comparing it with the README finds two different answers to "what does a stranger with the link see".

**Fix:** add "the What is happening pick for a Critical report" to the list. Reword the trust facts clause to: "no field for ... a comment, or the trust details beyond the one sentence (the sentence names how many people nearby confirmed it; the spread, cells and votes are never shown)". Then check that `TestSharePageShowsOnlyAllowedFields` still matches the wording.

### SK-03 (info): the send time comment check can drop a push in a narrow race at commit time

**File:** `internal/store/queries/push.sql:615-623` (`CommentPushStillHolds`)

The check compares `GREATEST(c.created_at, c.checked_at)` with the recipient's latest `read_at` of that kind on the report. `created_at` defaults to `now()`, which is the start time of the insert transaction. Here is the race. Comment Y's transaction starts at t1. The recipient marks the old bell row read at t2 > t1. Y commits after t2, so `NotifyCommentVisible` writes a new unread row and queues a push. At send time `t1 > t2` is false, so a correct note is skipped. This goes against the owner's rule toward silence, not toward noise, so it is low risk. Everything else I traced is right: no push when the only comment is gone, one push per report per device is untouched, nothing makes a sound, and `reply_to_id` is NULL exactly for top level comments (`comments.go:326`, `:445`), which matches the CASE.

**Fix:** compare against the unread bell row itself. The comment must have been seen at or before the claimed row's `updates.created_at`, and after the read before that row. Or record the comment id on the update row and compare ids.

### SK-04 (info): the access log redaction matches only the literal `/r/` prefix

**File:** `internal/api/router.go:244-249` (`loggedPath`)

`EscapedPath()` returns `RawPath` when one is set. So `GET //r/<token>` or `GET /%72/<token>` is not redacted. chi answers 404 because it routes on the raw form, but the token still lands in the log line. Only someone who already has the token can send this, so it is not a leak to strangers. It is still a way to plant a live capability in the logs. Everything that really routes under `/r/` is covered, the Panic line included, and `public_status.go:117` logs no token.

**Fix:** redact when `path.Clean(r.URL.Path)` starts with `/r/`, as well as when the escaped path does, or redact every 404 line.

### SK-05 (info): middle pages are never refreshed after Show more is used twice

**File:** `web/static/js/comments.js:1143-1164` (`loadTail`)

The poll now reads the first page and the last page read (`lastPageAfter`). Pages in between are never read again. A deletion, a removal, like counts or a new reply on a thread there stays stale until the panel is reopened. Before the fix the poll covered only page one, so this is better than before, but WR-B04's report does not mention it.

**Fix:** keep the list of page cursors and re-read each page in turn, one page per tick, or note the limit in the fix report.

### SK-06 (info): TestPhoneSheetHasNoMutedText matches overrides by last compound selector only

**File:** `web/review08_templates_fix_contract_test.go:194-233`

Overrides are keyed by `r08tLastCompound`. The test does not compare specificity, so a phone rule such as `.report-page .comments__count { color: var(--color-text) }` would count as an override even though the base rule `.leaflet-container .report-page .comments .comments__count` beats it in the cascade. Today's overrides in `phone.css:1358-1398` use the full chain, so the page is right. The test just would not catch a weaker override added later. It also skips muted rules inside any media block (`r.Media != ""`), a dark scheme block included.

**Fix:** require the override head to contain the base head, or compare specificity.

### SK-07 (info): the Policy says every comment keeps a check result, which is not always true

**File:** `web/templates/legal_body.html.tmpl:132`; `internal/service/comments.go:351`

The Policy now says: "For every comment the check's result, its scores and which model gave them are kept". A comment whose check gave no model and no scores (no screener configured), or one that stayed `pending` and was settled by a moderator, stores no verdict, model or scores (`comments.go:351` writes them only when `check.Model != "" || len(check.Scores) > 0`). This overclaims slightly in the safe direction.

**Fix:** "For every comment the check finished on, ...".

## Checked and found clean

- **Reply counts (WR-S02):** `shown_replies` counts the same `visible` set under the same `shown` rule as listed replies, uncapped, and is 0 on replies. Placeholder roots still count their shown replies. The service sets `ReplyCount` from it and no longer adds to it. Server paging of roots (`after_id`, `root_cap`) is unchanged. The only defect is on the client side (SK-01).
- **Access log (WR-S01):** the formatter covers both the Write and Panic lines. Query strings are never logged. The `Recoverer` stack print has no URL in it. The handler's error log leaves the token out.
- **/favicon.ico (WR-T02):** one exact GET route that answers a plain 404. It is in `sessionSkipSet().get`, and `matches` still refuses raw paths, `//` and dot segments. HEAD falls to the default branch and does not skip, which is the same as every other exact path. No cookie and no sessions row.
- **Push worker (IN-S05):** the check runs only for `PushKindChanged` with a comment update kind. An error goes to `rowErr`, not to a send. A false result skips the row and leaves the bell row alone. Mute, switches, one push per report and the hourly ceiling all run after it as before. There is no path that sends a push it should not.
- **WR-S03:** a blurred finished Rescue shows count 0 in every view built by `buildFinishedView`.
- **comments.js under Pinalert.requestTimeout (WR-B01):** comments.js loads only on `index.html.tmpl`, after `app.js`, and `requestTimeout` is exported at `app.js:1477`. Both the request and the body read race `limit.expired`. A body that times out lands in the onFail branch, giving `{body:null}` and then the IN-B02 refetch. `abort` clears the timer, then aborts. An aborted promise that never settles in WebKit is harmless, because `loadSeq` guards every list read. The expired promise has its own catch, so no unhandled rejection.
- **WR-B03 keep typed words:** only an exact match or a prefix of the sent words is cleared. The reply target is cleared only if it is still the one the post went to. The input is never disabled during a post (`writePostState` sets only `aria-disabled` on the button), so this fix is real.
- **WR-B04 order and last page:** `insertRoot` puts roots in id order for poll, Show more and the reader's own post. Removal in `mergeRoots` is limited to `(after, lastId]`, or to the end when no more pages follow. A `loadTail` answer for an old cursor is dropped (`after !== lastPageAfter`). Show more and the tail read never share `inflight`, and a new `load` aborts the tail. `clearList` resets `loadedMore` and `lastPageAfter`.
- **WR-B05 like stamp:** the stamp only goes up, and `sentStamp` is captured when each request leaves (load, tail, Show more). An answer to a list read sent after the Like answer is accepted in full. No loop.
- **IN-B02 refetch after an unknown outcome:** the form shows only in `rows` mode, so `load()` there never shows the skeleton or wipes the list. It returns at once when offline.
- **WR-B02 GDACS card:** placement now comes from `phoneDetail()` alone. `openOfficialPopup` sets `activePopup` (`map.js:2673`), so the early return in `exitPanel` fires and focus stays on the card. `replaceAllPlacements` touches only report markers.
- **IN-B06 tab return:** the listener is added only while polling and removed in `stopPolling`. `tick` returns early unless `visibilityState === 'visible'`, so going hidden sends nothing. A tick aborts the request in flight before it starts another, so there are no stacked requests.
- **IN-B07 keyCode 229:** this only blocks an Enter that ends a composition. A plain Enter is keyCode 13 on desktop Safari and Chrome. Shift+Enter is unchanged.
- **WR-T01 fixed composer:** the rule is inside `@media (min-width: 900px)`. `data-report-panel` is set only by `enterPanel`, which returns on a phone. The panel popup is itself `position: fixed`, top under the bars, left 0, 520px wide and full height, and map.js clears Leaflet's transform. Even during a zoom animation, where `will-change: transform` could make the popup the containing block, its left and bottom are the panel's, so the box lands in the same place. The `padding-bottom` on `.leaflet-popup-content`, computed as 1 + 12 + 2 + 24 + 24 + 16 = 79px with `--line-height-body: 1.5` unitless, keeps the last comment clear. The Back button is at the top and is not covered.
- **Templates and docs:** the README and Policy sharing lines match `share.go`. The services line now names comment checks. The `share_status` icon link is fine. The Share buttons padding is fine. The Comments heading focus ring is removed only on `:focus` of a `tabindex=-1` heading.
