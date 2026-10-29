---
phase: 08-report-page-and-comments
plan: 11
subsystem: service
tags: [comments, updates, bell, notifications, migration, sqlc]
requires:
  - phase: 08-09
    provides: CommentService, its stored change notifier, Post's clear outcome
  - phase: 08-08
    provides: the comments tables
provides:
  - migration 00019 with the two comment update kinds and uq_updates_comment_unread
  - UpdateCommentOnYours and UpdateReplyToYours with event lines and trust line cases
  - CommentForNotify and InsertCommentUpdate (the one comment notification insert)
  - CommentService.NotifyCommentVisible and the CommentVisibleNotifier interface
  - the commented arm of FollowedOpenReports
affects: [08-13, 08-14, 08-15, 08-19, 09-10, 09-21]
tech-stack:
  added: []
  patterns: [write time insert that skips in SQL, partial unique index as the only cap, ceiling reads that skip a kind]
key-files:
  created:
    - internal/store/migrations/00019_add_comment_update_kinds.sql
    - internal/service/commentupdates_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/queries/updates.sql
    - internal/store/queries/comments.sql
    - internal/store/sqlc/updates.sql.go
    - internal/store/sqlc/comments.sql.go
    - internal/service/updates.go
    - internal/service/updatescopy.go
    - internal/service/comments.go
    - internal/service/updates_test.go
    - internal/service/updatescopy_test.go
    - internal/service/moderation_view_test.go
    - internal/service/comments_test.go
    - internal/api/handlers/updates.go
    - internal/api/handlers/activity_page_test.go
key-decisions:
  - "Comment rows never roll up into more and do not count toward the hourly ceiling: the three ceiling reads (AdvanceCursorAndInsertUpdate, MarkFadingAndInsertUpdate, InsertReliableAlert) now skip the two comment kinds, and the unread unique index is the only cap."
  - "A top level comment tells the report's reporter (comment_on_yours); a reply tells only the author of the comment it answered (reply_to_yours), never the reporter."
  - "A clear comment counts for the follow arm only while it is undeleted (review W8)."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 11: Comment bell rows Summary

A new comment on your report and a reply to your comment now reach your bell as one unread row per kind and report, carrying a kind and a report id only, and commenting makes the account follow the report.

## Migration

`internal/store/migrations/00019_add_comment_update_kinds.sql` (next free number after 00018).

The exact list written:

```
CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours'))
```

It also creates `uq_updates_comment_unread ON updates (account_id, report_id, kind) WHERE read_at IS NULL AND kind IN ('comment_on_yours', 'reply_to_yours')`. Down drops the index, deletes rows of the two kinds, and restores the old six kind list exactly. 00009 was the only earlier migration that touched `updates_kind_valid` (Phases 5 and 7.1 did not change it), so the old list is 00009's. Phase 9 plan 09-21 copies this list and appends `followed_posted`.

## What was built

- Go: `UpdateCommentOnYours`, `UpdateReplyToYours`; `UpdateEventKinds` now seven (after `UpdateReliableNearby`), `UpdateKinds` eight (before `UpdateMore`); `EventLine` words "New comment on your report" and "New reply to your comment" (no word about sound or vibration); `UpdateTrustLine` answers both the same line as `UpdateReachedLive`.
- Queries: `CommentForNotify` (:one) and `InsertCommentUpdate` (:execrows) in comments.sql. The insert reads from `accounts`, so an unknown recipient, the commenter, an account with `deleted_at` and a recipient with a `report_mutes` row write nothing; `ON CONFLICT (account_id, report_id, kind) WHERE read_at IS NULL AND kind IN (...) DO NOTHING`.
- `FollowedOpenReports` gained `UNION SELECT c.report_id FROM comments c WHERE c.account_id = ... AND c.deleted_at IS NULL AND c.moderation_state = 'clear'`.
- `CommentService.NotifyCommentVisible(ctx, commentID) error` and `CommentVisibleNotifier`. It returns nil for an unknown, deleted, removed or held comment, picks the recipient, writes via `InsertCommentUpdate`, and pokes the stored notifier with `ReportChanged(reportID, commenterAccountID)` only when a row was written. `Post` calls it for a comment stored clear (borderline included) and logs a failure with the comment id only; a held comment is not announced. `CommentQuerier` gained the two new methods.
- handlers/updates.go swagger enums: `reached_live,hidden,resolved,fading,reliable_nearby,comment_on_yours,reply_to_yours,more`. docs/ is left for plan 08-19.

## Tests

- `TestMigration00019` (store): both kinds insert, an unknown kind fails 23514, a second unread row of the same account, report and kind fails 23505, a read row does not conflict, other kinds are not capped, Down removes comment rows and restores the old list, Up again works.
- `commentupdates_test.go`: `TestCommentUpdatesRecipients` (nine subtests: comment, reply, reply to reply, own report, muted, deleted account, no reporter account, thirty comments as one row and the next after reading, comment rows outside the hourly ceiling proved through `InsertReliableAlert`), `TestCommentUpdatesHeldWaits`, `TestCommentUpdateCarriesNoText`, `TestLikesNotify`, `TestCommenterFollowsReportInUpdates`.
- Amended with dated notes: `TestUpdateKindsClosed`, `TestUpdatesCopy`, `TestUpdateTrustLineChoices`, the UpdateTrustLine row of the moderation view test, `TestFillUpdatesPart` (new subtest), and in comments_test.go the "posting calls no notifier yet" subtest.
- Full run on a fresh `p8_11_a`: `go test ./... -p 1 -count=1 -v` exit 0, every package ok, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped, every new test shows `--- PASS`. `go build`, `go vet` and a `-short` run clean. `make sqlc` leaves no further diff. The database was dropped at the end.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Amended a 08-09 test not in the plan's file list**
- **Found during:** Task 2
- **Issue:** `TestCommentScreeningOutcomes/posting calls no notifier yet` in `internal/service/comments_test.go` asserted zero notifier calls after posts, which is exactly what this plan wires.
- **Fix:** renamed to "posting pokes the notifier once per bell row written" and now asserts calls equal the comment update rows written, with a dated note. The likes and flag assertions of zero calls are untouched and still pass.
- **Files modified:** internal/service/comments_test.go
- **Commit:** d6b2c1a3

### Plan wording kept

- The trust line for a comment row uses `ConfirmedSentence` exactly like `reached_live`, as the plan says. On a disputed report that sentence is the confirmed sentence of the trust object, not the disputed one; a later copy review may want comment rows to follow the disputed line.

## Notes for later plans

- 08-13 and 08-14: call `CommentVisibleNotifier.NotifyCommentVisible(ctx, commentID)` after a held comment is set clear. Calling it twice is safe (the unread index makes the second a no-op, and the notifier is poked only for a written row).
- 08-15 (phone): `EnqueueChangedFromUpdates` lists its kinds explicitly (`reached_live`, `hidden`, `resolved`, `fading`), so comment rows are not pushed by it; the poke through `ReportChanged` is the hook.
- 08-10: the feed's viewer follows filter should match the new follow arm (clear and undeleted comment).
- Phase 9 plan 09-10: add the block skip inside `InsertCommentUpdate` (noted in its header comment). Plan 09-21: copy the kind list from 00019; the comment unread index is scoped to the two comment kinds, so it will not collide.
- 08-19: regenerate docs for the new enums.

## Known Stubs

None.

## Threat Flags

None. The only new write is `InsertCommentUpdate`, covered by T-08-49 to T-08-52.

## Self-Check: PASSED

Files present: 00019 migration, commentupdates_test.go, and the modified files above. Commits present: c2b44fdc (kinds), d6b2c1a3 (insert, notify, follow arm). No non ASCII byte, so no dash, in any changed line.
