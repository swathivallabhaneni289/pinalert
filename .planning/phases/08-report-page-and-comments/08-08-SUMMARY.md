---
phase: 08-report-page-and-comments
plan: 08
subsystem: database
tags: [comments, postgres, sqlc, migration, visibility, account-deletion]
requires:
  - phase: 08-02
    provides: report page data and the share token
  - phase: 08-07
    provides: the Open report row that reads report.comment_count from the reports payload
  - phase: 07.1
    provides: accounts.username, username_screen, deleted_at and the AccountDeleteQuerier steps
provides:
  - migration 00018 with comments, comment_likes, comment_flags, comment_moderation_actions
  - CommentsForReport, the one query that decides what a viewer sees of a report's comments
  - comment_count and viewer_commented in ReportExtrasForReports
  - testutil seed helpers for comments, likes, comment flags and usernames
  - account deletion steps for comments, likes and comment flags
affects: [08-09, 08-10, 08-11, 08-12, 08-13, 08-14, 08-15, 09-10]
tech-stack:
  added: []
  patterns: [visibility written once in a CTE, conditional LEFT JOIN for nullable names so sqlc types them as *string, placeholders zeroed in SQL]
key-files:
  created:
    - internal/store/migrations/00018_add_comments.sql
    - internal/store/queries/comments.sql
    - internal/store/sqlc/comments.sql.go
    - internal/store/comments_test.go
    - internal/testutil/comments.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/queries/reports.sql
    - internal/store/sqlc/reports.sql.go
    - internal/store/sqlc/models.go
    - internal/store/moderation_test.go
    - internal/testutil/db.go
    - internal/service/accountdelete.go
    - internal/service/accountdelete_test.go
key-decisions:
  - "Migration number is 00018 (00017 was the newest). report_flag_reasons_distinct from 00010 is reused, no second function."
  - "reply_cap is a total bound on reply rows for the returned roots, not a per thread page size."
  - "A placeholder row returns account_id 0 as well as empty body and name, so nothing identifies its author."
  - "The three comment deletion steps sit right after the Phase 7.1 steps (before DeleteAccountUpdates), as the plan words it."
  - "No edit to internal/demo/memquerier.go: the new extras fields default to 0 and false, and TestDemoParityWithSQL passes."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 08: Comments data layer Summary

Comments, likes, comment flags and comment moderation actions now have their own tables, with one visibility query for a viewer, a real comment count in the batched extras read, seed helpers, and account deletion that blanks a leaver's words inside the existing transaction.

## What was built

- **Migration `00018_add_comments.sql`** (Down drops the four tables in reverse; `report_flag_reasons_distinct` stays).
  - `comments`: id, report_id, account_id, parent_id (thread root), reply_to_id (comment actually answered), body, created_at, deleted_at, moderation_state, needs_review, and the held-check columns check_attempts, next_attempt_at, lease_until, last_error, verdict, scores, model, checked_at.
  - Constraints: `comments_state_valid`, `comments_body_shape` (live 1 to 280 characters, deleted exactly empty), `comments_thread_shape`, `comments_parent_not_self`, `comments_attempts_valid`, `comments_pending_due`, `comments_verdict_valid`.
  - Indexes: `idx_comments_report (report_id, id)`, `idx_comments_parent`, `idx_comments_due`, `idx_comments_review`, `idx_comments_account`.
  - `comment_likes` (primary key comment_id, account_id; `idx_comment_likes_account`), `comment_flags` (`comment_flags_one_per_account`, `comment_flags_reasons_valid`, `comment_flags_note_length`, `idx_comment_flags_open`, `idx_comment_flags_account`), `comment_moderation_actions` (action keep or remove).
  - No ON DELETE, no CONCURRENTLY. Truncate lists the four tables last; `TestMigrationsTruncateListsEveryTable` passes.

## Queries (internal/store/queries/comments.sql)

`InsertComment` (:one, returns `id, created_at`; params report_id, account_id, parent_id, reply_to_id, body, moderation_state, needs_review, next_attempt_at, verdict, scores, model, checked_at), `CommentTarget` (:one), `CommentsForReport` (:many), `MarkCommentDeleted` (:execrows, takes `now`), `LikeComment` (:execrows), `UnlikeComment` (:execrows), `CommentLikeState` (:one, `likes int32, viewer_liked`), `InsertCommentFlag` (:one, no row on conflict means pgx.ErrNoRows), `BlankAccountComments` (:execrows, takes `now`), `DeleteAccountCommentLikes` (:exec), `DeleteAccountCommentFlags` (:exec).

`CommentTargetRow`: `ID, ReportID, AccountID, ParentID *int64, ReplyToID *int64, Deleted bool, ModerationState string, AuthorUsername *string, AuthorUsernameScreen *string, AuthorDeleted bool`.

`CommentsForReportParams`: `ViewerAccountID, ReportID, AfterID int64; RootCap, ReplyCap int32`.

`CommentsForReportRow` columns, in order:

| Column | Go type | Meaning |
|--------|---------|---------|
| `id` | int64 | comment id |
| `parent_id` | *int64 | thread root for a reply, NULL for a root |
| `reply_to_id` | *int64 | comment answered, NULL for a root |
| `account_id` | int64 | service internal, 0 on a placeholder |
| `body` | string | empty on a placeholder |
| `created_at` | time.Time | |
| `gone` | string | '' shown, 'deleted' or 'removed' placeholder |
| `held` | bool | the viewer's own pending comment |
| `author_username` | *string | NULL unless the author is the viewer, or screen is clear and account not deleted |
| `author_kind` | string | 'user', 'new_member', 'deleted_account', '' for a placeholder |
| `reply_to_author` | *string | reply only, and only under the review W3 rule, else NULL |
| `is_reporter` | bool | false on a placeholder |
| `likes` | int32 | 0 on a placeholder |
| `viewer_liked` | bool | false on a placeholder |
| `viewer_flagged` | bool | false on a placeholder |

Rules, written once in the query: shown = not deleted and (clear, or pending and the viewer's own); a root is listed when shown, or when deleted or removed with at least one shown reply (placeholder); a reply is listed when shown and its root is listed. Rows come back in thread order (root, then its replies, id order). `ReportExtrasForReportsRow` gained `CommentCount int32` (clear, not deleted, every level, never pending or removed, same for every viewer) and `ViewerCommented bool` (viewer has a clear, not deleted comment).

## Account deletion

`AccountDeleteQuerier` gained `BlankAccountComments(ctx, BlankAccountCommentsParams{Now, AccountID}) (int64, error)`, `DeleteAccountCommentLikes(ctx, accountID)` and `DeleteAccountCommentFlags(ctx, accountID)`. `DeleteAccount` calls them in that order right after the Phase 7.1 steps (after `ClearAuthThrottle` when keyed), before `DeleteAccountUpdates`, inside the one transaction. `wantDeleteOrder` is now 15 steps and `wantDeleteOrderKeyed` 16, with a dated note.

## Tests

- `TestMigration00018` (markers, no CONCURRENTLY or ON DELETE, every table check, Down then Up).
- `TestCommentsForReportVisibility` (13 sub-cases: clear, held, placeholders, review W4 leak check, removed placeholder, shown replies only, review W3 reply_to_author cases, New member, deleted account, is_reporter, likes and own flags, caps and paging).
- `TestCommentCountMatchesList`, `TestCommentCountExtras`, `TestCommentLikeOnePerAccount`, `TestCommentFlagOnePerAccount`, `TestCommentWriteQueries`.
- `TestReportExtrasForReports` amended with dated assertions for the two new columns.
- `TestDeleteAccountComments` (words blanked, likes and flags removed, other people's rows kept, reader view, rollback on a failed scrub), `TestDeleteAccountOrder` and the fake querier amended with dated notes.
- Full run on fresh database `p8_08_a`: `go test ./... -p 1 -count=1 -v` exit 0, 2100 PASS lines, only `TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture` skipped. `go vet ./...` clean, `make sqlc` leaves no further diff (only comments.sql.go, reports.sql.go, models.go changed), `-short` run clean. The database was dropped at the end.

## Deviations from Plan

None that change behavior. Small choices the plan left open are listed under key-decisions (reply_cap meaning, account_id 0 on placeholders, deletion step placement). The two nullable names (`author_username`, `reply_to_author`) are conditional LEFT JOINs rather than CASE expressions, because sqlc types a CASE with a NULL branch as `interface{}`.

The new `TestMigration00018` registers `db.Close` with `t.Cleanup` instead of `defer`, so its row cleanup runs on an open handle (the older migration tests close first, which is why they leave rows behind).

## Notes for later plans

- 08-04's `TestSharePageShowsOnlyAllowedFields` skipped its "a comment" marker because no comments table existed. It can now be added (a comment body and a username must not appear on the share page). Not edited here.
- 08-09: use `CommentLikeState` for the answer to a like or unlike; `InsertComment` takes `scores` as `[]byte` JSON and `NextAttemptAt` must be set for `pending`; call `CommentTarget` after a zero-row `MarkCommentDeleted` to tell not yours from already gone. `CommentReplyCap` is a total bound.
- 08-14 (worker) and 08-13 (moderator screen): the claim, settle and review queries are not written here. `MarkCommentDeleted` and `BlankAccountComments` leave `needs_review`, `next_attempt_at` and `lease_until` alone, so the worker and the review list must skip rows with `deleted_at` set.
- The count equals the non placeholder list rows only while a reply never sits under a root that is neither shown nor a placeholder (a root held by someone else). The service must refuse replying to a non clear target, which 08-09 already does (ErrCommentGone).
- Phase 9 plan 09-10 adds its block filter to `CommentsForReport` and to `comment_count` and should null `reply_to_author` when the answered author is blocked (noted in the query header).
- `internal/demo/memquerier.go` was not edited; the Demo has no comments and answers 0 and false by zero value.

## Self-Check: PASSED

Files present: 00018_add_comments.sql, comments.sql, comments.sql.go, comments_test.go (store), comments.go (testutil), and the modified files above. Commits present: 9fe1173f (migration), ca782452 (queries, count, helpers), cb1f7060 (account deletion).
