---
phase: 08-report-page-and-comments
plan: 13
subsystem: api
tags: [comments, moderation, routes, sqlc, audit]
requires:
  - phase: 08-08
    provides: comments, comment_flags and comment_moderation_actions (migration 00018)
  - phase: 08-11
    provides: CommentVisibleNotifier and NotifyCommentVisible
  - phase: 08-12
    provides: the comment service built in main.go and resolved in the router
provides:
  - comment_moderation.sql with the queue, lock, keep, remove, dismiss and audit queries
  - CommentModerationService with Queue, Keep and Remove
  - three moderator routes for comments inside the requireModerator group
  - Deps.CommentModerationTx and its main.go wiring
affects: [08-14, 08-18, 08-19]
tech-stack:
  added: []
  patterns: [sibling service of the report moderation service, row lock on the comment, audit row in the same transaction, notify after commit]
key-files:
  created:
    - internal/store/queries/comment_moderation.sql
    - internal/store/sqlc/comment_moderation.sql.go
    - internal/service/commentmoderation.go
    - internal/service/commentmoderation_test.go
    - internal/api/handlers/moderate_comments_e2e_test.go
  modified:
    - internal/api/handlers/moderation.go
    - internal/api/router.go
    - cmd/server/main.go
key-decisions:
  - "Keep of a comment that is clear, not marked for review and not flagged answers ErrAlreadyHandled (409), so a double tap writes one audit row. This reuses the report side's error."
  - "Keep or Remove of an unknown, deleted or removed comment answers ErrCommentGone (404); a repeated Remove is therefore a 404, not an idempotent 200."
  - "The list also carries flag_reasons (the dialog labels of the distinct reasons people gave), an addition beyond the plan, because the query already returns them and a moderator decides faster with them."
  - "The three handlers carry no swag blocks, because the report moderation handlers have none on purpose (internal surface); plan rule 10 says to follow their style."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 13: Comment moderation Summary

Moderators can now list the comments that need a decision and Keep or Remove them, each act changing the comment, dismissing its open flags and writing its audit row in one transaction, behind the same plain 404 gate as the report moderation routes.

## What was built

### Queries (commit c76cf824)

`internal/store/queries/comment_moderation.sql`, one new generated file `comment_moderation.sql.go` (`git diff --stat` against the base lists nothing else under internal/store/sqlc):

- `CommentModerationQueue` (:many, `row_limit`): undeleted, not removed comments that have an open flag, `needs_review`, or `moderation_state = 'pending'`; newest first. Columns `id, report_id, category, body, created_at, moderation_state, needs_review, open_flags, flag_reasons, pending`. It selects no account id, email or username.
- `LockCommentForModeration` (:one, `FOR UPDATE OF c`): `id, moderation_state, deleted, needs_review, open_flags`.
- `KeepComment` (:execrows): clears `needs_review` and `lease_until`; a pending comment becomes clear and its `next_attempt_at` is cleared; never touches a removed or deleted comment.
- `RemoveComment` (:execrows): `removed`, `needs_review` false, `next_attempt_at` and `lease_until` cleared; never touches a removed or deleted comment.
- `DismissCommentFlags` (:execrows, `dismissed_at = now()`), `InsertCommentModerationAction` (:exec).

### Service (commit c76cf824)

`internal/service/commentmoderation.go`: `CommentModerationQuerier` (six queries, asserted against `*sqlcgen.Queries`), `CommentModerationTxFunc`, `PgxCommentModerationTx`, `NewCommentModerationService(q, tx, notifier)`, `CommentModerationItem`, the reason words `CommentReasonFlagged` ("Flagged by people"), `CommentReasonNearLine` ("Near the line"), `CommentReasonCheckPending` ("Check not finished"), `ModCommentAlreadyHandledMessage`, `ModCommentNotFoundMessage` and `CommentModerationQueueLimit = 200`.

Keep and Remove run in one transaction: lock, decide, change, dismiss flags, audit row. After a successful commit, Keep of a comment that was pending calls `NotifyCommentVisible` (an error is logged with the comment id only). With a nil tx func the same steps run on the plain querier (tests only).

### Routes and wiring (commit e3d7c404)

Registered with `r.Handle` inside the requireModerator group, so a wrong method reaches the gate and a non moderator gets the byte equal 404:

| Route | Handler |
| --- | --- |
| `GET /api/moderation/comments` | `ModerationComments` |
| `POST /api/moderation/comments/{id}/keep` | `ModerationCommentKeep` |
| `POST /api/moderation/comments/{id}/remove` | `ModerationCommentRemove` |

`Deps.CommentModerationTx` is set in main.go from `service.PgxCommentModerationTx(pool)`. The router builds `service.NewCommentModerationService(deps.Sessions, deps.CommentModerationTx, comments)` with the resolved comment service as the notifier.

## Moderator JSON shape, for plan 08-18

`GET /api/moderation/comments` answers 200, `Cache-Control: no-store`:

```
{ "comments": [ {
    "id": 12, "report_id": 3, "category": "flood",
    "words": "the comment as stored",
    "at": "2026-10-29T10:15:00Z",
    "reasons": ["Flagged by people", "Near the line", "Check not finished"],
    "flag_reasons": ["Spam or gibberish", "False or a hoax"],
    "held": false, "needs_review": false, "flags": 2
} ] }
```

- `reasons` holds only the words that apply, in the order flagged, near the line, check not finished. Never null. `flag_reasons` is the dialog labels in dialog order, an empty array when nobody flagged. `comments` is `[]` when empty (the client shows P8-C19 "No comments to review.").
- No key or value names an author, flagger, email or account (`TestModerateComments` walks every key and searches the body for usernames and emails).
- `POST .../keep` and `POST .../remove` answer 200 `{ "id": 12, "action": "keep" }` (or `"remove"`).
- Errors use the standard field error body: 404 field `id` with "No comment found with that id. Check the id and try again." (unknown, deleted, removed, or a repeated Remove); 409 field `id` with "This comment was already handled. Refresh the lists." (Keep of a comment that needs no decision); 400 field `id` with "Comment id must be a number."; 405 for a moderator using the wrong method; the plain 404 for everyone who is not a moderator.

## Tests

Disposable database `p8_13_a` (created, dropped at the end). Service (database): `TestCommentModerationQueueReasons`, `TestCommentKeepReleasesHeld` (five subtests: held comment released with one notify and the lease cleared, the real notifier writing the reporter's bell row, a flagged clear comment kept without a notify, a comment needing no decision, unknown/deleted/removed), `TestCommentRemove` (removed placeholder when a reply shows, held comment gone, repeated Remove), `TestCommentModerationAuditInTx` (audit row content, and a real rolled back transaction when the audit insert fails for both Keep and Remove), plus `TestServerCopyCommentModeration`. Handlers e2e: `TestModerateComments` (six subtests) and `TestModerateCommentsNonModerator404` (anonymous and ordinary account, three paths, six methods, byte equal to the report moderation route's 404).

Every new test shows `--- PASS` at top level when run with `-v`. Full run `go test ./... -p 1 -count=1 -v` on `p8_13_a`, in the foreground with the output filtered: every package `ok`, zero `--- FAIL`, and only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped. `go build ./...`, `go vet ./...` and `DATABASE_URL= go test ./... -short -count=1` are clean, and `make sqlc` leaves no diff.

## Deviations from Plan

### Auto-fixed Issues

None.

### Choices the plan left open

- No swag blocks on the three handlers: the report moderation handlers have none by design (their file header says the surface is internal), and rule 10 says to follow their style.
- Extra list field `flag_reasons`, extra copy constants, extra test `TestServerCopyCommentModeration`, and the `ErrAlreadyHandled` reuse listed under key-decisions.
- `CommentModerationQueueLimit = 200` was chosen here; the plan named a limit but no value.

### Process slip

- During the first full run I redirected the test output to `/tmp/none` and removed that file straight away. It held only test output and nothing from the repository; no other file was written outside the worktree.

## Notes for later plans

- 08-14 (worker): Keep sets a pending comment clear and clears `next_attempt_at` and `lease_until`; Remove sets `removed`. The worker's settle step must lock the comment row first (the same row `LockCommentForModeration` locks) and guard its update with `moderation_state = 'pending'`, so an in-flight check cannot turn a kept or removed comment back, and it should call `NotifyCommentVisible` only when it made the pending to clear transition. It must keep skipping rows with `deleted_at` set.
- 08-18 (moderator screen): use the JSON shape above. Remove reads to readers as "Comment removed" where replies keep a thread; the screen should refresh the list after an act and treat 404 and 409 as "already handled".
- 08-19: the three routes have no swag annotations on purpose, so docs/ needs no change for them.
- Rule 9 kept: the report moderation queries, service and routes are untouched.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the three routes inside the existing moderator gate (T-08-63 to T-08-66).

## Self-Check: PASSED

Files present: comment_moderation.sql, comment_moderation.sql.go, commentmoderation.go, commentmoderation_test.go, moderate_comments_e2e_test.go, and the edits to moderation.go, router.go and main.go. Commits present: c76cf824 (queries and service), e3d7c404 (routes, wiring, e2e). Acceptance greps: `"/api/moderation/comments` appears 3 times in router.go and `PgxCommentModerationTx` once in main.go. No non ASCII byte in any new or added line, so no em or en dash.
