---
phase: 08
part: review fix round 2, skeptic
checked_commit: 54a26eac
base_commit: b3fc0213
reviewed: 2026-10-29
findings:
  blocker: 0
  warning: 1
  info: 3
  total: 4
status: issues_found
database: p8_skep2 (absent at start, created, dropped at the end)
---

# Phase 8: skeptic, review fix round 2

Scope: `git diff b3fc0213..54a26eac` only. Read only. No server was started.

Tests run on p8_skep2: `go test ./internal/service ./internal/api/... ./web -run 'Comment|Share|TrustModel|Policy|Reply|Replies' -count=1 -p 1` all ok. `node --test web/comments_test.mjs`: 59 pass, 0 fail.

## Findings

### SK2-01 (warning): a deleted reply in a read tail plus one new reply leaves the new reply unreachable, with no Show more replies button

**File:** `web/static/js/comments.js:925` (`coverReplies`), with `placeReplies` at about line 857 and the toggle count at line 815.

**Failure scenario:**
1. A thread has 150 replies. The person opens it and presses Show more replies. The page answer has `more` false, so `coveredTo` is 150 and `serverMore` is false.
2. Reply 120 is deleted (or removed by a moderator). Then someone else posts reply 151.
3. The next poll's list answer carries replies 1 to 100, with `reply_count` 150 (149 old plus 151), so it is capped. `placeReplies(…, 0, false)` drops only inside (0, 100], so stale reply 120 stays. `replyIds.length` is still 150.
4. `coverReplies` works out `150 > 150` as false and `last (100) >= coveredTo (150)` as false, so `serverMore` stays false.

The result: reply 151 cannot be reached and no button offers it. The thread still shows the gone reply 120. The toggle says "View 150 replies", and that matches only by coincidence. This lasts until the panel is reopened. The recorded known limit covers only a stale delete, removal or like count past the first 100. It does not say that a new reply can become unreachable. The reachability of every reply is what SK-01 was meant to fix.

**Fix:** reconcile the read tail rather than guess from counts. On a capped list answer for an open thread with `coveredTo > last`, let the tick (or `coverReplies`) read `threadPath(root, last)` and pass the answer through `placeReplies(rec, replies, sentStamp, last, !more)`. That drops stale tail replies and brings in new ones. A cheaper partial fix keeps the button honest: count the held replies with an id up to `last` (call it `head`), then set `serverMore` when `t.total - head !== heldTail`, where `heldTail` is the number of held replies after `last`. Then a mismatch either way shows the button.

### SK2-02 (info): the toggle undercounts by one while a poll that left before your own reply is still out, in a capped thread

**File:** `web/static/js/comments.js:1487` and `:922`

**Scenario:** A thread has 101 replies and 100 of them are read. You post reply 102, so `total` becomes 102 and `replyIds` holds 101. A list answer that left before the post comes back with `reply_count` 101. `coverReplies` sets `total` to 101, and the toggle shows `max(101, 101)`, which is "View 101 replies" when the real number is 102. The next poll corrects it.

**Fix:** in `coverReplies`, add one for each held reply whose `postStamp > sentStamp` that the answer did not carry. The answer's stamp is already passed to `placeReplies`.

### SK2-03 (info): ListReplies on a hidden report answers `open: rep.open`, while List answers `open: false`

**File:** `internal/service/comments.go:535-537`

**Scenario:** For a finished Rescue report (`rep.hidden`), `List` returns `CommentList{Comments: []}`, so `open` is false. `ListReplies` returns `Open: rep.open`. If `CommentsOpen` is true for that report, the two reads of the same report disagree on `open`. This is a report level fact the viewer already has, not a comment oracle, but the one-rule promise in the doc comment is not quite kept.

**Fix:** return `CommentList{Comments: []CommentView{}}` when `rep.hidden`, the same as List.

### SK2-04 (info): the new TRUST-MODEL parenthetical says the sentence names the confirm count, which is true only for some states

**File:** `docs/TRUST-MODEL.md:656`

**Scenario:** The bullet now says "the sentence names how many people nearby confirmed it". The share page's `Sentence` is the trust block line for the state. For an Ended report that line is `PublicEndedSentence` ("Faded out. No longer updating."). A Resolved report uses `publicResolvedSentence`, and an unconfirmed Active report uses a "Needs N people…" line. None of these names a confirm count, so a reader of the doc expects a number the page does not show.

**Fix:** reword it as "(for a confirmed report the sentence names how many people nearby confirmed it; the cells, the votes and the scores are never shown)".

## Clean

- SK-01 server: `ListReplies` uses `after_id = root - 1` with `root_cap 1`, which picks the first listable root with an id at or above the asked id. The answer is non-empty only when that root equals `rootID`. So these all answer the same empty list: a reply id, another report's comment, an unknown id, another person's held root, and a removed or deleted root with no shown reply. Held, removed and deleted replies, and the username shown only when cleared, all go through the unchanged `visible`/`shown` CTE and the `au` join. `reply_after` filters only the `replies` CTE, so `shown_replies` stays the whole thread's count. `More` comes from the cap + 1 extra row, and `NextAfter` is the last reply id. The handler gives 400 for a bad value, for `reply_after` without `replies_of`, and for `after` with `replies_of`.
- SK-01 browser:
  - No duplicates: ids are deduped through `seen` and `nodes`, then sorted.
  - Your own reply survives a list answer or a page answer that left before the post, through `postStamp`. It is still dropped correctly by a later answer that no longer carries it.
  - Show more replies does not loop. `serverMore` is taken from the server's `more`, and a capped poll sets it again only when the counts or ranges show unread replies.
  - A failed read keeps the button and shows the error line, built with `element`/`setText`, with no markup sink.
  - Adding to `answerStamp` on a post is harmless to `freshData`, because the like stamps compare only by order.
- SK-04: `loggedPath` checks both the escaped prefix and `path.Clean("/"+u.Path)`. That covers `//r/`, `/%72/`, `/./r/`, `/%2Fr/` and `/x/../r/`. The query string is never logged, and `/r/{token}` is the only route that carries a share token.
- SK-02: the page shows the What is happening pick for a Critical report, and the doc now lists it. That matches `share_status.html.tmpl`. The `TestSharePageShowsOnlyAllowedFields` citation is kept. SK2-04 is the only precision nit.
- SK-07: the Policy line matches `comments.go:351`, which keeps a result only when the outcome is not unfinished and the check gave a model or scores. When no screener is set up, no check runs, and nothing is kept. The diff does not touch `PolicyLastUpdated` or `service.CurrentTermsVersion`. The Policy date did not move.

_Reviewer: Claude (skeptic, round 2)_
