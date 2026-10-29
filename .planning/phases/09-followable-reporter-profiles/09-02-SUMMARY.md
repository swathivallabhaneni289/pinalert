---
phase: 09-followable-reporter-profiles
plan: 02
subsystem: database
tags: [postgres, sqlc, goose, follows, blocks, profiles, account-deletion]
requires:
  - phase: 07.1
    provides: accounts.username, username_screen, GetAccountIDByUsername
  - phase: 08
    provides: the comment tables and the 15 step account deletion order
provides:
  - migration 00020 (profile_public, account_follows with soft ending, person_blocks)
  - people.sql, 23 typed queries with ended rows filtered
  - endFollowsOf and endFollowPair, the only way a service ends follows
  - testutil SeedPerson, SeedPublicReporter, SeedFollow, SeedBlock
  - account deletion that clears the person's follows and blocks
affects: [09-04, 09-13, 09-26]
tech-stack:
  added: []
  patterns:
    - "soft ending rows (ended_at plus end_reason) with a text scan test over every query file"
key-files:
  created:
    - internal/store/migrations/00020_add_people_profiles.sql
    - internal/store/queries/people.sql
    - internal/store/sqlc/people.sql.go
    - internal/store/people_test.go
    - internal/service/followend.go
    - internal/service/followend_test.go
    - internal/testutil/people.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/queries/accounts.sql
    - internal/store/sqlc/accounts.sql.go
    - internal/store/sqlc/models.go
    - internal/testutil/db.go
    - internal/service/accountdelete.go
    - internal/service/accountdelete_test.go
key-decisions:
  - "Migration number taken: 00020 (newest on main was 00019_add_comment_update_kinds.sql)."
  - "DeleteAccountFollows and DeleteAccountBlocks run right before RevokeAccountSessions, after DeleteAccountPushSubscriptions, not directly after DeleteAccountCommentFlags (see Deviations)."
  - "BlockedPeople also returns username_screen so the caller shows a name only when it is clear."
patterns-established:
  - "Follows end only through endFollowsOf and endFollowPair; plan 09-26 adds the queued phone note cleanup inside them."
  - "TestFollowReadsFilterEnded fails on any query block that touches account_follows without naming ended_at in code (comments do not count)."
requirements-completed: [PROF-01, PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 02: People profiles, follows and blocks data Summary

Migration 00020 adds the opt in profile flag and person scoped follows (soft ending, five reasons) and blocks, with 23 typed queries that filter ended follows everywhere, two follow ending helpers, seed helpers and account deletion hooks.

## What was built

- **Migration 00020** (`00020_add_people_profiles.sql`): `accounts.profile_public` (default false) and `profile_public_at` with `accounts_profile_public_at_set`; `account_follows` (composite key, `account_follows_not_self`, `account_follows_end_shape`, `account_follows_end_reason_valid`, three partial indexes); `person_blocks` (`person_blocks_not_self`, `idx_person_blocks_blocked`). Down drops it all. `TestMigration00020` proves Up, every refusal (23514 and 23505), Down and Up again.
- **people.sql**: all 23 queries from the plan. `FollowAccount` checks opt in, live account, clear username and both block directions in the one INSERT ... SELECT, and reactivates an ended row with a fresh `created_at`. Named lists and counts use the same filter, so everyone else is only counted (D-03).
- **followend.go**: `FollowEndReason` with five constants, `followEnder`, `endFollowsOf`, `endFollowPair`.
- **testutil/people.go**: `SeedPerson`, `SeedPublicReporter`, `SeedFollow`, `SeedBlock`; `Truncate` names both new tables.
- **Account deletion**: `AccountDeleteQuerier` gains `DeleteAccountFollows` and `DeleteAccountBlocks`, called in the one transaction; `ScrubDeletedAccount` also sets `profile_public = false`.

## Task commits

1. Migration, migration test, truncate list: de118101
2. Queries, sqlc output, helpers, seeds, store and service tests: 1898b759
3. Account deletion hooks and amended tests: 720aaf15

## Tests

Full run on a fresh scratch database `p9_02_a` (`go test ./internal/store/ ./internal/service/ ./internal/testutil/ -p 1`): all three packages ok, 650 passing tests, the only skip is TestScreeningFixture. New tests all seen with `-v`: TestMigration00020, TestFollowAccountGuards, TestFollowReactivatesEndedRow, TestBlockAccountInsertsBlockOnly, TestEndedFollowsAreInvisible, TestFollowReadsFilterEnded, TestFollowersPageSplit, TestFollowersPageKeyset, TestPublicProfilePathsForViewer, TestFolloweeStanding, TestFollowEndHelpers, TestDeleteAccountEndsFollowsAndBlocks. `go build ./...`, `go vet ./...` clean and `sqlc generate` leaves no diff. The scratch database is dropped.

## Deviations from Plan

**1. [Rule 1 - Plan inconsistency] Deletion step position.** Task 3 says to call the two deletes "immediately before RevokeAccountSessions" and also that `wantDeleteOrder` ends with `DeleteAccountCommentFlags, DeleteAccountFollows, DeleteAccountBlocks, RevokeAccountSessions, ScrubDeletedAccount`. On main `DeleteAccountCommentFlags` is an early step with updates, cursors, mutes and the push steps after it, so both cannot hold without reordering Phase 8 and 3.3 steps. I kept every old step and its order and put the two new steps right after `DeleteAccountPushSubscriptions` (immediately before `RevokeAccountSessions`, after every Phase 5, 7.1 and 8 step). `wantDeleteOrder` is now 17 steps and `wantDeleteOrderKeyed` 18, with a dated note. The literal "ends in exactly CommentFlags, Follows, Blocks" acceptance line is therefore not met; the intent (after every earlier step, in the one transaction, before the scrub) is.

**2. [Rule 2 - Missing safety] BlockedPeople returns username_screen.** The plan listed username and created_at. Because a username shows only when its screen is clear, the query also returns `username_screen` so plan 09-04 can decide what to show for a pending name without losing the ability to unblock that person.

**3. [Minor] Query comment scan.** `TestFollowReadsFilterEnded` strips comment lines before checking, so a comment naming `ended_at` cannot satisfy the guard; it has a negative control and requires at least 10 queries on the table to be seen.

No auth gates. No other deviations.

## Notes for later plans

- `FollowAccount` returns 0 rows for refused and already following; the service tells them apart with `PersonRelation`.
- A self follow or self block fails with SQLSTATE 23514 (follow only when the profile is public, since the insert is not attempted otherwise).
- Account deletion hard deletes follows and blocks in both directions, and `accountdelete` is covered here; no other personal table was added by this plan.
- `go vet`/gofmt: pre-existing gofmt drift in `internal/store/comments_test.go`, `internal/api/handlers/place_test.go` and `internal/geocode/photon.go` is not from this plan and was left alone.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-09-04 to T-09-07, T-09-95, T-09-96), all mitigated and tested.

## Self-Check: PASSED

All created files exist; commits de118101, 1898b759, 720aaf15 are on the branch.
