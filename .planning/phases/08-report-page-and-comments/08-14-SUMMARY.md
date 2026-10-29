---
phase: 08-report-page-and-comments
plan: 14
subsystem: service
tags: [comments, moderation, worker, background, sqlc]
requires:
  - phase: 08-08
    provides: the comments table and its check columns
  - phase: 08-11
    provides: CommentVisibleNotifier and NotifyCommentVisible
  - phase: 08-13
    provides: Keep and Remove, whose row lock and state change the settle step must respect
provides:
  - ClaimDueCommentChecks, FinishCommentCheck and RescheduleCommentCheck
  - CommentWorker with Run and SweepOnce, started from main.go
affects: [08-18, 08-19]
tech-stack:
  added: []
  patterns: [sibling worker over the comments table, single statement settle guarded by moderation_state = pending, notify only on the row this worker changed]
key-files:
  created:
    - internal/service/commentworker.go
    - internal/service/commentworker_test.go
  modified:
    - internal/store/queries/comments.sql
    - internal/store/sqlc/comments.sql.go
    - cmd/server/main.go
key-decisions:
  - "The settle is one UPDATE guarded by moderation_state = 'pending' and deleted_at IS NULL. It takes the row lock itself; a moderator's Keep or Remove holds the same lock, so under READ COMMITTED the settle waits, re-reads, affects 0 rows and writes nothing. No separate lock statement is needed."
  - "NotifyCommentVisible runs only when FinishCommentCheck affected exactly one row and the new state is clear."
  - "The claim takes its clock from the worker (now and lease_until params, as the plan lists), so a frozen test clock decides what is due; the report expiry test uses the same clock."
  - "The provider call is bounded by ModerationWorkerCallTimeout (8 s), as the plan and PATTERNS say. See the note under Deviations."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 14: Comment check worker Summary

A held comment is now rechecked in the background every 30 seconds (and once at boot) until it passes, is flagged for moderators, or is refused, with the same claim, lease, backoff and error classes as the report worker.

## What was built

### Queries (comments.sql, only comments.sql.go regenerated)

- `ClaimDueCommentChecks` (:many; params `now`, `lease_until`, `batch_limit`): a MATERIALIZED CTE over `comments` with `moderation_state = 'pending'`, `deleted_at IS NULL`, `next_attempt_at <= now`, a lease that is NULL or past, joined to `reports` with `expires_at > now` and `moderation_state <> 'removed'`, `ORDER BY next_attempt_at, id`, `LIMIT batch`, `FOR UPDATE OF c SKIP LOCKED`. The update sets `lease_until` and `check_attempts + 1` and returns `id, body, check_attempts, expires_at`. `expires_at` is an addition to the plan's column list, needed to park quota and permanent errors where the claim never reaches.
- `FinishCommentCheck` (:execrows): sets state clear or removed, `needs_review`, `verdict`, `scores`, `model`, `checked_at`, clears lease, due time and `last_error`; only `WHERE moderation_state = 'pending' AND deleted_at IS NULL`.
- `RescheduleCommentCheck` (:execrows): sets `next_attempt_at` and `last_error`, clears the lease; only while pending.

### CommentWorker (internal/service/commentworker.go)

`CommentWorkerQuerier` (asserted against `*sqlcgen.Queries`), `CommentWorker`, `NewCommentWorker(q, screener, notifier, opts...)`, `Run(ctx)`, `SweepOnce(ctx)`. `WorkerOption` is a function of `*ModerationWorker`, so it could not be reused; the twins are `CommentWorkerOption`, `WithCommentWorkerClock`, `WithCommentWorkerJitter` and `WithCommentSweepInterval`. The constants, the three error class words and `ModerationBackoff` are reused by name, not copied.

Outcomes: pass gives clear; borderline gives clear with `needs_review`; reject gives removed and no notification; a transient error (including an error that is not a `CallError`) reschedules by `ModerationBackoff` with retry-after as a floor; quota and permanent errors are rescheduled to the report's `expires_at`; a shutdown during the call writes nothing and the lease lapses. The notifier is called only when this worker's own write changed one row to clear. A notify failure is logged with the comment id and does not fail the sweep (the comment is already visible). Log lines carry the comment id, attempt, verdict or class word, status and wait only.

### main.go

One `commentService` is now built right after the vote service and used both for `Deps.Comments` and as the worker's notifier (it was built inline in `Deps` before, so the worker needed the shared instance). `commentWorker := service.NewCommentWorker(queries, screener, commentService)` and `go commentWorker.Run(context.Background())` follow the moderation worker, with a comment in that block's style.

## Tests (internal/service/commentworker_test.go)

All nine named tests plus `TestCommentWorkerRunSweepsAtStartAndStops`, each seen with `-v` as `--- PASS`: `TestCommentWorkerReleasesClear` (also checks the author sees it before and others after, trimmed body sent, and no comment text in the log), `TestCommentWorkerBorderline` (also listed by `CommentModerationQueue`), `TestCommentWorkerRejectRemoves`, `TestCommentWorkerBacksOff` (five cases: transient, retry-after floor, non-CallError, quota, permanent; not due again at the same clock; comes back after its wait), `TestCommentWorkerStopsAtExpiry` (expired, removed report, ended by the clock), `TestCommentWorkerNoDoubleClaim` (two workers, 12 comments, each screened and notified once, also run under `-race`), `TestCommentWorkerShutdownWritesNothing`, `TestCommentWorkerSkipsDeleted` (deleted while held, and deleted during the call), `TestCommentWorkerNoDoubleNotify` (four subtests: worker alone; a moderator Keep during the check is not announced a second time; a late refusal cannot undo a Keep; a late pass cannot undo a Remove).

Full run on a fresh `p8_14_a`: `go test ./... -p 1 -count=1 -v` exit 0, every package ok, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped. `go build ./...`, `go vet ./...` and `DATABASE_URL= go test ./... -short -count=1` are clean, `sqlc generate` leaves no further diff (only comments.sql.go changed). The database was dropped at the end.

## Deviations from Plan

### Auto-fixed Issues

None.

### Points the owner or caller may want to check

- **Call timeout.** The orchestrator note said the provider call is bounded at 3 s; the plan text, PATTERNS and the report worker use `ModerationWorkerCallTimeout` (8 s), while 3 s is `InlineScreenTimeout` (the inline check at post time). I followed the plan: a retry after a 3 s inline timeout would otherwise time out again at the same bound. Five rows at 8 s each (40 s) fit inside the 60 s lease. Switching is a one line change in `screenRow` if 3 s was meant.
- **Extra column.** The claim also returns `expires_at` (see above).
- **Extra settle guard.** `FinishCommentCheck` also requires `deleted_at IS NULL`, so a comment its author deleted during the call is neither released nor notified (tested).
- **Extra test.** `TestCommentWorkerRunSweepsAtStartAndStops` was added beside the nine named tests.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the worker's three statements on the comments table, covered by T-08-59 to T-08-62.

## Self-Check: PASSED

Files present: commentworker.go, commentworker_test.go, the additions to comments.sql and comments.sql.go, the edit to main.go. Commits present: afd8735c (queries, worker, tests), 096ccc4b (main.go). Acceptance greps: `SKIP LOCKED` 3 in comments.sql, `ModerationBackoff` 3 in commentworker.go, `NewCommentWorker` 1 and `go commentWorker.Run` 1 in main.go. No non ASCII byte in any changed line, so no em or en dash.
