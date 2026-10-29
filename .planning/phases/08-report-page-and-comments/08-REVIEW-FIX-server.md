---
phase: 08
part: server
fixed_at: 2026-10-29T18:30:00Z
review_path: .planning/phases/08-report-page-and-comments/08-REVIEW-server.md
base_commit: 83f9f49e
iteration: 1
findings_in_scope: 13
fixed: 8
recorded_not_fixed: 5
skipped: 0
status: all_fixed
---

# Phase 8: server code review fix report

**Fixed at:** 2026-10-29
**Source review:** 08-REVIEW-server.md, plus WR-T02 (server half) and IN-T04 from 08-REVIEW-templates.md, triaged in 08-REVIEW.md
**Iteration:** 1
**Database:** p8_fix_s only (created, recreated for the final run, dropped at the end)

**Summary:**
- In scope: 13. These are the 3 server warnings, the WR-T02 server half, IN-T04, the 3 cheap IN-S items, and the 5 IN-S items to record.
- Fixed: 8, each test first (the new or amended test seen failing, then passing), one commit per finding. IN-S01 has a second commit for a test it broke.
- Recorded, not fixed: 5 (IN-S03, IN-S04, IN-S06, IN-S07, IN-S08), each with a reason below.
- Kept:
  - sessionSkipSet prefixes are still ["/demo", "/static", "/r"] and POST is still ["/internal/official/refresh"]. The only addition is the exact GET path /favicon.ico.
  - No migration was added. 00019 is still the last one.
  - The 3.3 names are unchanged.
  - No new enumeration oracle.

## Fixed

### WR-S01: The share token is written to the access log

**Commit:** 6f444dfb
**Files:** `internal/api/router.go`, `internal/api/share_routes_test.go`
**Applied fix:** `pathOnlyLogFormatter.NewLogEntry` now passes the path through `loggedPath`, which writes `/r/{token}` for any path under `/r/`. The Write line and the Panic line both read the redacted path.
**Test:** `TestShareTokenNeverInAccessLog` captures `Deps.RequestLog` and GETs `/r/<token>` and `/r/<token>/extra`. It asserts that the token is absent and that `GET /r/{token} ` appears twice. Before the fix the token was in both lines.

### WR-S02: The reply cap was one limit for the whole page

**Commit:** e19fdd7c
**Files:**
- `internal/store/queries/comments.sql`
- `internal/store/sqlc/comments.sql.go` (regenerated)
- `internal/service/comments.go`
- `internal/service/comments_test.go`
- `internal/store/comments_test.go`

**Applied fix:**
- The `replies` CTE of `CommentsForReport` caps each thread with `ROW_NUMBER() OVER (PARTITION BY parent_id ORDER BY id)`, oldest first.
- A new column `shown_replies` is the real, uncapped count of the replies the viewer may see on each root, placeholders included, under the same rule as a listed reply.
- `List` sets `ReplyCount` from that column instead of counting the returned rows.
- `CommentReplyCap` now means per thread and is 100. The worst case is 51 roots times 100 replies.
- The JSON field and the swagger docs are unchanged.

**Test:** `TestCommentReplyCapPerThread` seeds thread A with cap+1 replies, then thread B with 1 reply. B lists its reply with ReplyCount 1. A lists cap replies with ReplyCount cap+1. Before the fix, B had 0 replies and A's ReplyCount was 500.
**Amended:** in `internal/store/comments_test.go`, the `reply_cap 1` assertion has a dated note. It now pins at most 1 reply per thread (5 rows); it was 1 reply in total.

### WR-S03: A finished Rescue report showed a comment count

**Commit:** dd505a2c
**Files:** `internal/service/historyread.go`, `internal/service/comments_test.go`
**Applied fix:** `buildFinishedView` sets `CommentCount` to 0 whenever `CoarsenFinishedRescue` blurred the report (`v.Approximate`). Every finished view goes through it: Focus (the report page and the popup row) and the Past and Resolved lists. Nearby keeps only active and disputed views (`report.go`, the expiry gap guard), so it needs no change.
**Test:** `TestCommentCountHiddenOnFinishedRescue` covers a faded and a resolved Rescue report, each with 2 comments: Focus and List both say 0. A past Flood report keeps 1 in both. Before the fix, Focus said 2.

### WR-T02 (server half): /favicon.ico set a session cookie

**Commit:** 71f37cd2
**Files:**
- `internal/api/handlers/pwa.go`
- `internal/api/router.go`
- `internal/api/pwa_routes_test.go`
- `internal/api/share_routes_test.go`

**Applied fix:**
- `handlers.FaviconPath` is a new exact GET route that answers a plain 404 (`handlers.FaviconNotFound`).
- `sessionSkipSet` adds that one path to its GET set. The prefixes, the POST set and `PWAPublicPaths` (still the ten UI-SPEC paths) are unchanged.
- The templates fixer adds the icon link to the public page.

**Test:** `TestFaviconSetsNoCookie` sends a cookieless GET to /favicon.ico and asserts 404, no Set-Cookie and an unchanged sessions count. Before the fix the request set a cookie and wrote one sessions row.
**Amended:** `TestPWASkipSetMatchesRoutes` has a dated note. The GET set is now the PWA paths, the tiles and FaviconPath, and FaviconPath must be a registered GET route.

### IN-T04: Retry-After did not match "Wait a minute"

**Commit:** 538f4a3b
**Files:** `internal/api/handlers/public_status.go`, `internal/api/handlers/public_status_e2e_test.go`
**Applied fix:** `publicStatusRetryAfter` is `"60"`, matching `service.PublicTooManyLine`.
**Test:** the assertion in `TestSharePageRateLimited` now wants 60, with a dated note. Before the fix it saw 1.

### IN-S01: A tab in a pasted comment was refused

**Commits:** 9000bb5a and b4ba7215 (follow-up)
**Files:** `internal/service/commenttext.go`, `internal/service/commenttext_test.go`, `internal/service/comments_test.go`
**Applied fix:**
- `CleanCommentBody` turns each tab into a space, after the CRLF step and before the trim, so tabs at the ends still trim.
- Every other control character is still refused.
- No browser script pre-checks characters, so the browser needs no change.

**Test:** in `TestCleanCommentBody`, `"a\tb"` now gives `"a b"`, with a dated note, and two new cases cover tabs at the ends and only tabs. The follow-up commit amends `TestCommentScreeningOutcomes`, which posted a tab as its refused control character; it now uses the bell character, with a dated note.

### IN-S02: The 500 path of /r/{token} skipped the page headers

**Commit:** 1c373370
**Files:** `internal/api/handlers/public_status.go`, `internal/api/handlers/public_status_error_test.go` (new)
**Applied fix:**
- Two helpers: `setPublicStatusPrivacyHeaders` sets `no-store`, `noindex, nofollow` and `no-referrer`, and `writePublicStatusError` sets them before `http.Error`.
- Both 500 paths, a failed status read and a failed render, now use `writePublicStatusError`.
- Content-Type stays `text/plain`, which is correct for that body.

**Test:** `TestPublicStatusErrorHeaders` needs no database. It uses a fake `ShareQuerier` whose token read fails, and a template set with no `share_status`. It asserts 500, the three headers and no cookie for both paths. Before the fix all three headers were missing.

### IN-S05: A queued comment push was sent after the comment was removed or deleted

**Commit:** 0fe7b1cb
**Files:**
- `internal/store/queries/push.sql`
- `internal/store/sqlc/push.sql.go` (regenerated)
- `internal/service/pushworker.go`
- `internal/service/pushworker_test.go`
- `internal/service/pushcomments_test.go`

**Applied fix:**
- A new query, `CommentPushStillHolds`, asks whether the note still has a comment to be about.
- The worker calls it after the existing predicate, only for a changed row whose event is `comment_on_yours` or `reply_to_yours`. On false it skips the row as `not_eligible` and leaves the bell row alone.
- The query is true while some comment on the report passes all of these checks:
  - clear and not deleted;
  - not by the recipient;
  - top level for `comment_on_yours`, or answering one of the recipient's comments for `reply_to_yours`;
  - newer than the recipient's last read note of that kind on that report (created_at or checked_at, whichever is later).
- The update row holds no comment id, and later comments fold into the one unread row. A note is therefore dropped only when no qualifying comment is left, not when only the first one is gone.
- Known edge, accepted because the owner wants few notifications: a comment written before such a read and cleared only later by a moderator's Keep reads false. The note is not sent, and the bell row still shows.

**Tests:**
- `TestWorkerSkipsWhenNoLongerEligible` gains four cases: each kind with its comment still there (sent) and removed or deleted (not_eligible). The two removed cases failed before the wiring.
- `TestCommentPushStillHolds` runs the query on the database. It covers a removed, deleted or own comment, a second clear comment, a reply to someone else, and a comment before and after the last read note.

## Recorded, not fixed

### IN-S03: Comment ids can be probed through the 403 and 404 answers
**Reason:** changing the 403/404/field answers of Delete and actOnComment touches behaviour that existing handler and browser tests pin. The leak is small (ids are sequential, nothing about the content leaks), and no new oracle was added. Left for a later round.

### IN-S04: The timeline keeps the oldest 2000 vote rows
**Reason:** the fix needs either a "history shortened" note, which is new copy and template work, or a newest-rows read, which needs a standing row seed. Neither is cheap, and the trust block still shows the current truth.

### IN-S06: Account deletion can race a comment in flight
**Reason:** the window is milliseconds and needs the person's own two requests to overlap. Guarding `InsertComment` changes a query that the comment, worker and account delete tests all exercise, which is not cheap for this round.

### IN-S07: Every non-development start needs an https BASE_URL
**Reason:** this is intended and documented in the README. Making the UAT recipe set `ENV=development` is a docs and memory change outside the server files. Tightening the check with `url.Parse` changes startup behaviour, which was not asked for in this round.

### IN-S08: The moderator comment list scans every comment
**Reason:** fine at today's scale, and the route is moderator only. The UNION rewrite is a later performance item.

## Verification

- `go build ./...` and `go vet ./...`: clean.
- Full run: `go test ./... -p 1 -count=1 -v` on a freshly recreated p8_fix_s. Results: 20 packages ok, 0 FAIL, 2174 top level tests passed (6212 with subtests). The only 3 skips are the known ones: TestGeocodeLive, TestOfficialLive and TestScreeningFixture. p8_fix_s was dropped afterwards.
- `node --test web/*.mjs`: 1422 pass, 0 fail.
- Every commit is path limited, carries the trailer, and adds no non-ASCII character. No em or en dash, no Python, no image, and no sound or vibration wording.
- Every changed file is in the server fixer's list (internal/** and the router and handlers). No web, template, CSS, docs or README file changed. `docs/TRUST-MODEL.md` needed no change.

_Fixer: Claude (gsd-code-fixer), server part_
