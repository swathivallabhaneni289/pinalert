---
phase: 08
part: server
reviewed_at: 2026-10-29T12:00:00Z
reviewed_commit: 67370b92
base_commit: 88cbe8ad
depth: standard
status: issues_found
files_reviewed_list:
  - cmd/server/main.go
  - internal/api/router.go
  - internal/api/handlers/comments.go
  - internal/api/handlers/moderation.go
  - internal/api/handlers/public_status.go
  - internal/api/handlers/ratelimit_gate.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/share.go
  - internal/api/handlers/timeline.go
  - internal/api/handlers/updates.go
  - internal/service/accountdelete.go
  - internal/service/areaname.go
  - internal/service/commentmoderation.go
  - internal/service/comments.go
  - internal/service/commenttext.go
  - internal/service/commentworker.go
  - internal/service/feed.go
  - internal/service/follow.go
  - internal/service/historyread.go
  - internal/service/pushcopy.go
  - internal/service/pushdispatch.go
  - internal/service/pushworker.go
  - internal/service/report.go
  - internal/service/share.go
  - internal/service/timeline.go
  - internal/service/timelineread.go
  - internal/service/updates.go
  - internal/service/updatescopy.go
  - internal/store/migrations/00017_add_report_share_token.sql
  - internal/store/migrations/00018_add_comments.sql
  - internal/store/migrations/00019_add_comment_update_kinds.sql
  - internal/store/queries/comment_moderation.sql
  - internal/store/queries/comments.sql
  - internal/store/queries/push.sql
  - internal/store/queries/reports.sql
  - internal/store/queries/share.sql
  - internal/store/queries/updates.sql
  - internal/store/queries/votes.sql
findings:
  critical: 0
  warning: 3
  info: 8
  total: 11
---

# Phase 8 server review: report page, timeline, Share, the public page and comments

Scope: the server side of the Phase 8 commits in `88cbe8ad..67370b92` (`git log --grep='(08-'`, paths under `internal` and `cmd`). The 7.1 round 3 and 4 fixes in the same range (credentials, `auth.go`) are not part of this review. I read all of `comments.go`, `commentmoderation.go`, `commentworker.go`, `commenttext.go`, `share.go` (service and handler), `public_status.go`, `timeline.go`, `timelineread.go`, `areaname.go` and the timeline and comment handlers. I also read migrations 00017 to 00019, `comments.sql`, `comment_moderation.sql` and `share.sql`. For `router.go`, `ratelimit_gate.go`, `main.go`, `accountdelete.go`, `pushdispatch.go`, `pushworker.go`, `pushcopy.go`, `updates.go`, `updatescopy.go`, `follow.go`, `feed.go`, `historyread.go`, `report.go`, `reports.go`, `updates.go` (handler) and the query files `push.sql`, `reports.sql`, `updates.sql` and `votes.sql`, I read the Phase 8 diff and followed each change into the code around it: the request logger, `sessionSkip.matches`, `requireModerator`, `MayNotifyAboutReport`, `changesBranch`, `changesFor`, the bell `List` and the push cap and per report queries. I read 08-CONTEXT in full. The 20 PLAN and SUMMARY files and 08-RESEARCH were checked by grep for the points below, not read in full.

`go test -p 1 -count=1` of `internal/store/...`, `internal/service/...`, `internal/api/...` and `cmd/...` passes against the scratch database `p8_rev_s`, which I created for this review and dropped afterwards. This review writes no code, so no finding below was reproduced by a new test. Each finding says whether it was verified in code (traced line by line) and how sure I am.

There are no Critical findings. `git grep "DELETE FROM reports"` outside tests finds nothing, so the new `comments.report_id` foreign key without ON DELETE breaks no cleanup.

## Checked and held

- **Share token.** It is 16 bytes from `crypto/rand` as unpadded base64url (22 characters). `ValidShareToken` matches the column CHECK, and a bad shape runs no query. The first writer wins under `WHERE share_token IS NULL`, and a losing request reads the token again. Only the request that minted the token may look up the area, at most once per report, and the attempt is recorded first. The public route never calls the lookup.
- **Public page.** It never reads `r.Host` or a forwarded header: `og:url` and `og:image` come from `BASE_URL`, and outside `ENV=development` startup now requires an https `BASE_URL`. Every answer the template renders sends `no-store`, `X-Robots-Tag: noindex, nofollow` and `Referrer-Policy: no-referrer`. The `/r` prefix skips the session middleware, so the page sets no cookie and writes no sessions row. The match is `/r` or `/r/...`, never `/reports` or `/remove`. A held report, an unknown token and a report past the history window all read unavailable (404), and a moderator removal reads 410. The view model has no field that could hold the words, a coordinate, a name or a comment.
- **Comment visibility** (`CommentsForReport`):
  - A held comment is shown to its author only.
  - A deleted or removed root is listed only when it has a shown reply. It is then a placeholder with no words, no name, no likes and no viewer flags, and `commentViewFromRow` blanks those again.
  - The author's name is shown only when the username screen is clear and the account is not deleted, or when the viewer is the author.
  - `reply_to_author` follows the same rule, and also needs the answered comment to be clear and not deleted. `Post` applies the same rule to its own `ReplyToAuthor`.
  - A reply target must be on the same report, clear and not deleted. The new reply joins the target's root, so threads stay one level deep.
- **Count against list.** For viewer 0, `comment_count` (clear and not deleted, at every level) equals the non placeholder rows of `CommentsForReport`, apart from the two cases in WR-S02 and WR-S03.
- **Likes and flags never feed trust.** `comment_likes` and `comment_flags` are read only by `comments.sql` and `comment_moderation.sql`. No vote, tally, visibility or reliability query touches them.
- **Moderation.**
  - Keep and Remove lock the comment row first (`FOR UPDATE`). The change, the flag dismissal and the audit row then commit in one `pgx.BeginFunc` transaction.
  - `FinishCommentCheck` and `RescheduleCommentCheck` change a row only while it is `pending`. A worker answer that arrives after a Keep or Remove therefore waits on the lock, reads the row again and writes 0 rows. The worker notifies only on its own 1 row write, and Keep notifies only when the row was held.
  - `requireModerator` is reused unchanged on the three new routes.
  - The moderator list names no author.
- **Comment notifications.**
  - `InsertCommentUpdate` skips the commenter, a recipient who muted the report and a deleted account. The partial unique index `uq_updates_comment_unread`, whose ON CONFLICT predicate matches the index, keeps it to one unread row per account, report and kind.
  - Comment rows are left out of the three bell ceiling reads and never roll up into "more".
  - On the phone side:
    - A comment push is a `changed` row, so it counts toward `CountAccountPushesInWindow`.
    - It is sent only while the report is active and Live (`eventStillHolds`).
    - It carries the per report `Tag` and `Topic`, and it is subject to the Apple one per report rule.
    - It goes only to followers with Changes switched on.
    - The poke's actor (the commenter) is skipped.
    - The note's text holds no comment words, no username and no count.
- **Rate limits.**
  - The account buckets: comment 6 then 1 per 20 s, like, unlike and delete 30 then 1 per 3 s, share 10 then 1 per 30 s. A comment flag shares the report flag bucket.
  - Each family has its own network backstop, active only with `CLIENT_IP_HEADER`. With the header, the public page has a per IP limit (120 then 1 per second). Without the header every request is admitted, a choice recorded as review B1 that I accept.
- **Migrations 00017 to 00019.** Up and Down are symmetric, and `migrations_test.go` runs each Down. The kind list in 00019 copies 00009's list with the two new kinds added. Its Down deletes the comment rows before restoring the old CHECK, and `push_outbox.update_id` has no foreign key, so that delete cannot fail. Every new hot path read has an index: `idx_comments_report`, `idx_comments_account`, `idx_comments_due` and the primary key of `comment_likes`.
- **SQL injection.** Every new query is sqlc with bound parameters, and there is no string built SQL.
- **The 10 s WriteTimeout.** The slowest new request is `PostComment`: two report reads, the target read, the screener (bounded at 3 s), the insert and the read back. Share adds at most one 2 s area lookup. Both fit well inside 10 s.
- **Personal data in logs.** The new log lines carry ids, class words and status numbers only. Nothing logs comment words or usernames, and the pg errors that get wrapped carry no row values. The one gap is the access log line in WR-S01.
- **Phase 5 and 7.1.** All callers of the `ViewerFollows` signature change were updated, and the Demo memquerier compiles against the wider `ReportExtrasForReportsRow`. The 3.2 ceiling reads only lose the two new kinds. `publicBaseURL` is the one behaviour change for an existing deploy; see IN-S07.

## Warnings

### WR-S01: The share token is written to the access log on every public page request

**File:** `internal/api/router.go:230-249` (`pathOnlyLogFormatter.NewLogEntry` stores `r.URL.EscapedPath()`, and `pathOnlyLogEntry.Write` prints it), mounted router wide at `router.go:612` (`r.Use(requestLogger(deps.RequestLog))`). The page is registered at `router.go:765`.

**Issue:** the access log strips the query string but keeps the path. For `GET /r/{token}`, the path is the capability itself. `public_status.go:114` says "The token is a capability: it is never written to a log", and the service error path keeps that promise. But the request logger, which runs before the handler, prints every token in full, including those from WhatsApp's crawler and every reader. Tokens never change and no code can revoke them (00017 comment). Anyone with access to the host's log store or a log drain can therefore collect every shared link and open each report's live public page. They also learn when each link was opened and how often, which a link holder alone could not learn.

The data behind the link is the same public status a link holder sees, so the harm is limited. It is still a capability leak that breaks the plan's own promise (T-08-19 and the handler comment).

Failure scenario (verified in code, not run): a reader opens `https://.../r/Zk3pQ9xV1mTq7nBw5cLd2A`. stdout gets `[req] GET /r/Zk3pQ9xV1mTq7nBw5cLd2A 200 ...`, and the Render log keeps that line.

Sure: high.

**Fix:** redact the token in the formatter, so every route and every future logger path is covered:

```go
func (f *pathOnlyLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	path := r.URL.EscapedPath()
	if strings.HasPrefix(path, "/r/") {
		path = "/r/{token}"
	}
	return &pathOnlyLogEntry{logger: f.logger, reqID: middleware.GetReqID(r.Context()), method: r.Method, path: path}
}
```

**Test that proves it:** in `share_routes_test.go`, build the router with `Deps.RequestLog = log.New(&buf, "", 0)`, seed a report with a known token, GET `/r/<token>`, and assert that `buf` does not contain the token but does contain `/r/{token}`. Today the token is in the buffer.

### WR-S02: The reply cap is one limit for the whole page, so one busy thread hides every later thread's replies for good

**File:** `internal/store/queries/comments.sql` (`CommentsForReport`, the `replies` CTE: `ORDER BY v.id LIMIT sqlc.arg(reply_cap)` over the replies of all listed roots), `internal/service/comments.go:60` (`CommentReplyCap = 500`), `comments.go:503-515` (`ReplyCount` is counted from the rows that came back).

**Issue:** `reply_cap` is applied once, across the replies of all 51 roots, in global id order. It is not applied per thread. The roots CTE takes `CommentPageSize + 1` roots, so the 51st root, which `List` then drops, also uses up part of the budget. Paging moves by root id only (`after`), so a reply cut from page 1 is never served on any page. The client is not told: `ReplyCount` is the number of replies returned, `More` is about roots only, and `comment_count` still counts the missing replies. The list and the count then disagree.

Failure scenario (verified in code, not run): during a flood, a "Road X open?" root gets 500 replies, spread over many accounts by the rate limit. A root posted later, "Shelter at school Y is full", gets 3 replies. All 503 replies have ids above the first root's. If the 500 replies of the first root come first in id order, the second root shows `reply_count: 0` and no replies to anyone. If the replies are interleaved, the newest replies anywhere on the page are the ones cut. Either way the heading says, for example, 520 comments while about 20 show.

Sure: high for the mechanism. How likely 500 replies are depends on scale, but it is exactly the busy emergency the product is for.

**Fix:** cap per root and return the real per root count, so the client can show "N more replies", or page within one thread:

```sql
replies AS (
    SELECT * FROM (
        SELECT v.*, ROW_NUMBER() OVER (PARTITION BY v.parent_id ORDER BY v.id) AS rn
        FROM visible v JOIN roots ON v.parent_id = roots.id
        WHERE v.shown
    ) x
    WHERE x.rn <= sqlc.arg(per_thread_cap)::int
)
```

Add a `shown_replies` count per root, from the same `visible` filter, as a column of the root row. `List` then sets `ReplyCount` from it instead of `len(Replies)`. Keep a total bound as well if wanted, but apply it after the per thread cap.

**Test that proves it:** in `comments_test.go`, seed root A with `CommentReplyCap` clear replies, then root B with one clear reply. Call `List` as viewer 0 and assert that B's reply is present and B's `ReplyCount` is 1. Today B has none.

### WR-S03: A finished Rescue report shows a comment count, but its comment list is always empty

**File:** `internal/service/comments.go:485-487` (`List` returns `CommentList{Comments: []CommentView{}}` with `Count` 0 when `rep.hidden`), `internal/service/historyread.go:254` and `:327` (`CommentCount: c.commentCount`, with no finished Rescue check), `internal/service/report.go:1107` (the same for Nearby).

**Issue:** `reportFor` sets `hidden` for a finished Rescue report (03.1 D-23, `CoarsenFinishedRescue`), and `List` then lists nothing and answers `comment_count: 0`. The report views behind the Past and Resolved lists and Focus (the report page and the popup row) still carry the real `comment_count` from `ReportExtrasForReports`. D-19 puts that number in the Comments heading and the popup row. A reader of a finished Rescue report sees "Comments (7)", opens them, and the list call answers an empty list and 0. That is the "count matches the list" promise (D-03, `TestCommentCountMatchesList`) broken for one category. The count also tells strangers how much discussion a rescue had after its text was removed.

Failure scenario (verified in code, not run): a Rescue report with 7 clear comments is resolved. In the Resolved tab its row view has `comment_count: 7`, while `GET /api/reports/{id}/comments` answers `{"comments":[],"comment_count":0,...}`.

Sure: medium to high. I traced the code, but I did not run the client to see whether it hides the heading for Rescue.

**Fix:** zero the count wherever the finished Rescue blur applies, at the one place `baseView` already knows `finished` and the category:

```go
// historyread.go baseView, and the same in Focus if it builds its own view
count := c.commentCount
if _, blurred := CoarsenFinishedRescue(r, finished, nil); blurred {
	count = 0
}
... CommentCount: count,
```

Alternatively, have `List` return the same number, but hiding both is the D-23 intent.

**Test that proves it:** in `comment_count_e2e_test.go`, seed a Rescue report with 2 clear comments, resolve it, and read it through Focus and through `List`. Assert that both counts are equal (0). Today Focus says 2.

## Info

### IN-S01: `CleanCommentBody` rejects a tab

**File:** `internal/service/commenttext.go` (the `unicode.IsControl(r)` loop lets only `\n` through)

**Issue:** a comment pasted from a notes app or WhatsApp that contains a tab gets "has characters that cannot be posted". **Fix:** map `\t` to a space before the control check.

### IN-S02: The public page's 500 path skips the page headers

**File:** `internal/api/handlers/public_status.go:115-116` and `:153`

**Issue:** a `PublicStatus` error or a template failure answers with `http.Error`, so the response has no `no-store`, no `noindex` and no `no-referrer`. A crawler could cache or index the error. **Fix:** set the same four headers before `http.Error`, or route the 500 through one helper.

### IN-S03: Comment ids can be probed through the different answers for missing and existing comments

**File:** `internal/service/comments.go` (`actOnComment` at line 619, `Delete` at line 584)

**Issue:**
- A missing comment answers 404 with field `comment`. A comment on a withheld or too old report answers 404 with field `report`.
- `Delete` answers 403 for someone else's comment, held ones included, and 404 for a missing one, with no report check at all.

A caller can therefore tell which ids exist and that their report is withheld. Ids are sequential, so the leak is small. **Fix:** in `Delete`, answer `ErrCommentGone` for a comment that is not yours on a report `reportFor` refuses. In `actOnComment`, return `ErrCommentGone` when `reportFor` fails.

### IN-S04: The timeline keeps the oldest 2000 vote rows, so a busy report's history stops early

**File:** `internal/store/queries/votes.sql` (`TimelineVoteLog`: `ORDER BY id LIMIT row_cap`), `internal/service/timeline.go:62` (`TimelineLogCap = 2000`)

**Issue:** past 2000 rows, the newest votes are the ones dropped. The resolved, reopened or faded lines then come from a stale state, while the trust block shows the current truth. **Fix:** detect the cap (`len(log) == TimelineLogCap`) and add an "earlier history shortened" note, or read the newest rows. The second option needs a standing row seed, so the first is simpler.

### IN-S05: A comment push already queued is still sent after the comment is removed or deleted

**File:** `internal/service/pushworker.go:645-651` (`eventStillHolds` checks the report only)

**Issue:** a borderline comment is posted clear, the reporter's push is queued, and a moderator removes it (or the author deletes it) a few seconds later. The silent note "has a new comment" still goes out, and the bell row stays. No words leak. **Fix:** optional: for the two comment kinds, also require that the report still has a clear comment newer than the recipient's last read row.

### IN-S06: Account deletion can race a comment in flight

**File:** `internal/service/accountdelete.go` (`BlankAccountComments`), `internal/store/queries/comments.sql` (`InsertComment` does not check `accounts.deleted_at`)

**Issue:** a `PostComment` that passed the gate before the delete commits can insert after `BlankAccountComments`. Its words then stay visible under "Deleted account". The window is milliseconds and needs the person's own two requests to overlap. **Fix:** add `AND EXISTS (SELECT 1 FROM accounts WHERE id = account_id AND deleted_at IS NULL)` through an `INSERT ... SELECT`, or lock the account row `FOR SHARE` in `Post`.

### IN-S07: Every non development start now needs an https `BASE_URL`

**File:** `cmd/server/main.go:783-796` (`publicBaseURL`)

**Issue:** with `ENV` unset, the server used to start with a localhost base, and now `log.Fatal`s unless `BASE_URL` is https. This is intended and documented in the README. Any local or scratch server started without `ENV=development` (the UAT scratch server recipe runs `env -i`) now fails at boot. Also, any string that starts with `https://` passes, including one with a path or a query. **Fix:** make sure the UAT recipe sets `ENV=development`, and consider `url.Parse` to require a host and no path, query or fragment.

### IN-S08: The moderator comment list scans every comment

**File:** `internal/store/queries/comment_moderation.sql` (`CommentModerationQueue`), `00018_add_comments.sql` (no index on `comment_moderation_actions.comment_id`)

**Issue:** the OR of `needs_review`, `pending` and an EXISTS on open flags cannot use one index, so each moderator page load reads the whole `comments` table. The audit table has no index on `comment_id`. Both are fine at today's scale and on a route only moderators can reach. **Fix:** later, a UNION of the three indexed arms (`idx_comments_review`, `idx_comments_due`, `idx_comment_flags_open`).
