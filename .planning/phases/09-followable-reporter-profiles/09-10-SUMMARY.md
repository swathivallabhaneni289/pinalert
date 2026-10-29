---
phase: 09-followable-reporter-profiles
plan: 10
subsystem: comments
tags: [block, comments, notifications, sql, placeholder]
requires:
  - phase: 09-02
    provides: person_blocks table
  - phase: 09-08
    provides: Block and Unblock on the profile page
  - phase: 08-08
    provides: CommentsForReport shown CTE and comment_count in ReportExtrasForReports
  - phase: 08-11
    provides: InsertCommentUpdate and its self, mute and deleted skips
provides:
  - the block filter in the comment list, the comment count and the comment notification insert
  - the Comment hidden placeholder through the query, the service, the JSON and comments.js
affects: [09-11]
tech-stack:
  patterns:
    - "one blocked flag computed in the visible CTE next to shown, so the list, the reply rule, shown_replies and the placeholder rule all read the same fact"
key-files:
  created:
    - internal/service/comments_block_test.go
  modified:
    - internal/store/queries/comments.sql
    - internal/store/sqlc/comments.sql.go
    - internal/store/queries/reports.sql
    - internal/store/sqlc/reports.sql.go
    - internal/service/comments.go
    - internal/api/handlers/comments.go
    - internal/api/handlers/comments_e2e_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/comments.js
    - web/comments_test.mjs
key-decisions:
  - "A blocked author's comment is a hidden placeholder only when it is clear and not deleted. A deleted or removed one keeps its own word (Comment deleted, Comment removed), which says nothing about the block; a held one is never listed to anyone but its author."
  - "The block is computed once as a column of the visible CTE; shown excludes it, and the root rule lists a blocked clear root only when a shown reply exists."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 10: Block on the comment surfaces Summary

Someone you blocked no longer shows in your comments: their comments and replies leave your list and your comment count, a thread they started that others answered reads Comment hidden, no mention shows their name, and their comments never reach your bell or your phone.

## What was built

- **CommentsForReport (comments.sql)**: the visible CTE gains `blocked` (a person_blocks row from the viewer to the comment author) and `shown` now excludes it, so a blocked reply drops out through the existing reply rule and `shown_replies` leaves it out of the count of replies. The root rule lists a blocked, clear root as a placeholder when at least one reply is shown, with `gone = 'hidden'` and every Phase 8 zero (the zeros all hang on `l.shown`, so no word, name, kind, Reporter mark, like or viewer flag can appear). A blocked root with no shown reply is not listed. `reply_to_author` is NULL when the viewer blocked the author answered (the rn join gained the block test).
- **ReportExtrasForReports (reports.sql)**: comment_count skips comments whose author the viewer blocked. Viewer 0 blocks nobody, so its count is unchanged and still equals the list.
- **InsertCommentUpdate (comments.sql)**: one more NOT EXISTS beside the self, mute and deleted skips, from the recipient to the commenter, using the query's own argument casts. NotifyCommentVisible needed no change: zero rows is already "nothing written", and the phone note is queued only from rows this insert wrote.
- **CommentView / CommentResponse**: `Hidden` and `hidden` (swag field comment "true when you blocked the author and others replied"); the list description now mentions it. `make swag` was run and the three docs files are committed together.
- **comments.js**: `HIDDEN_LINE = 'Comment hidden'` drawn in the same branch and look as Comment deleted (empty disc, no head, no words, no meta row), and the like, delete and reply handlers refuse a hidden comment like a deleted one.

## Tests

Database tests ran on the scratch database `p9_10_a` (created, dropped at the end), every new test seen with `-v`:

- `TestBlockHidesCommentsFromBlocker` (3 subtests: list shape and placeholder zeros including a like and a flag made before the block, the M3 mention for the blocker versus everyone else, a blocked reporter's thread with no Reporter mark), `TestBlockedCommentCount` (blocker 3, others and viewer 0 six, list count equals real rows), `TestBlockIsOneWayForComments` (the blocked person still sees the blocker with the name, no row deleted or changed), `TestBlockedCommentDoesNotNotify` (3 subtests: a comment, a reply, and the one way case; checks updates, push_outbox, the changed rows and the dispatcher pokes). Written first, the notify test failed before the InsertCommentUpdate change and passed after.
- `web/comments_test.mjs`: new "hidden placeholder" case; 63 tests in the file pass.
- `TestCommentResponseOmitsIdentifiers` (comments_e2e_test.go) was amended with a dated note, see Deviations.
- Full `go test ./internal/... ./web/ -p 1 -count=1 -v` on `p9_10_a`: every package ok, 2241 passing tests, the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. `go build ./...` and `go vet ./internal/...` clean. `node --test web/*.mjs`: 1475 pass, 0 fail. `make sqlc` leaves models.go unchanged and `make swag` leaves no diff after the commits.

## Commits

- 58787b0f feat(09-10): hide a blocked person's comments, count and mention, and add Comment hidden
- 41283251 feat(09-10): skip a blocked person's comment in InsertCommentUpdate

## Deviations from Plan

**1. [Rule 3 - Blocking] comments_e2e_test.go was edited.** It is not in the plan's file list. `TestCommentResponseOmitsIdentifiers` pins the exact member set of a comment and failed on the new `hidden` member, so `hidden` was added to its expected set with a dated note ("Amended 2026-10-29 by Phase 9 plan 10 (D-12)"). Every other assertion stays.

**2. Test package.** `comments_block_test.go` is in package `service` (internal), not `service_test`, so it can reuse the `pcRig` of pushcomments_test.go, which is the only rig wired to the push dispatcher; the notify test needs it to check push_outbox for real rather than trivially.

No Phase 8 name differed from the plan.

## Notes for later

- `CommentPushStillHolds` (the send time recheck of a queued comment note) does not look at person_blocks. A note queued before a block and sent after it could still go out once. The plan names only the three seams above, so it was left alone; it would be a small NOT EXISTS if wanted.
- A stray one line file `placeholder.txt` was written by mistake under a mistyped scratch folder name in /private/tmp/claude-501 (outside the repo); the sandbox refused to delete it. It holds the letter x.

## Self-Check: PASSED

Files found: internal/service/comments_block_test.go, the two commits above (58787b0f, 41283251) in `git log`, 09-10-SUMMARY.md. No non-ASCII character or dash in any changed file.
