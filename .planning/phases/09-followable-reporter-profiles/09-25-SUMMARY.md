---
phase: 09-followable-reporter-profiles
plan: 25
subsystem: store, service
tags: [follow-notes, push, migration, dispatcher, lock-screen]
requires:
  - phase: 09-22
    provides: refreshFollowedPosts (the only writer of followed_posted rows) and the shared first alert slot
  - phase: 09-21
    provides: followed_posted kind, NotifyFollowersOf, UpdateFollowedPosted in the one predicate
  - phase: 03.3
    provides: the push outbox, the dispatcher branches, BuildPushText
provides:
  - migration 00024_add_follow_push.sql
  - the follow kind in every closed list of push.sql and in PushKinds
  - EnqueueFollowFromUpdates, MarkFollowFanout, DeleteQueuedFollowPushesForFollowee, DeleteQueuedFollowPush
  - the follow lock screen words (BuildPushText follow case)
  - PushDispatcher.followBranch between Critical and Reliable, FollowFanoutCap
affects: [09-26]
tech-stack:
  patterns:
    - "one bell row, two channels: the push follows exactly the rows the bell step wrote"
    - "one partial unique index covers all three first alert kinds and every ON CONFLICT predicate repeats it word for word"
key-files:
  created:
    - internal/store/migrations/00024_add_follow_push.sql
    - internal/service/followpush_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/queries/push.sql
    - internal/store/sqlc/models.go
    - internal/store/sqlc/push.sql.go
    - internal/store/push_test.go
    - internal/service/pushkinds.go
    - internal/service/pushkinds_test.go
    - internal/service/pushcopy.go
    - internal/service/pushcopy_test.go
    - internal/service/pushdispatch.go
    - internal/service/pushdispatch_test.go
key-decisions:
  - "Migration number taken: 00024 (00023 was the newest, the follow notes)."
  - "The follow pass is recorded as fanned after one full pass whether or not any row was queued (review W2). A report that fails the predicate (Hidden, Withheld) or the initial window is not marked."
  - "A follower needs no home area: BuildPushText builds the follow note without a distance, and the distance part is left out of the body."
  - "The two delete queries leave a row a worker holds (lease in the future, database clock) alone; the send time check of 09-26 covers that row."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-30
---

# Phase 9 Plan 25: Follow note on the phone Summary

The follow note now reaches the outbox: migration 00024 and every closed list of the push pipeline know the follow kind, the lock screen words name nobody, and a new dispatcher branch refreshes the reporter's bell followers and queues one follow push per bell row, after Critical and before Reliable.

## Migration number

00024_add_follow_push.sql (the newest before it was 00023_add_follow_notes.sql). Up: `push_outbox_kind_valid` re-created with the four kinds of 00011 plus `follow` (00013 and later migrations never touched this list; official pushes live in their own table), `uq_push_outbox_first` re-created `WHERE kind IN ('critical', 'reliable', 'follow')`, `push_report_progress.follow_state` and `follow_at`, and `push_report_progress_state_valid` re-created with the third clause. Down deletes the follow rows first, drops the columns and restores the CHECK, the index and the kind list text exactly.

## What was built

- **push.sql**: the two existing first alert `ON CONFLICT` predicates (EnqueueInitialPush, EnqueueReliableFromUpdates) are widened word for word; `follow` joins the EnqueueChangedFromUpdates NOT EXISTS list (the update kind list that 08-15 set is untouched, one match), EnqueueConfirmedUpdates, the ClaimDuePushes `first_kind` join and CountAccountPushesInWindow. GetPushReportProgress returns follow_state and follow_at (selected last, so the generated row type is still PushReportProgress). New: `EnqueueFollowFromUpdates :execrows`, `MarkFollowFanout :exec` (sticky, like the other two), `DeleteQueuedFollowPushesForFollowee :execrows` (arg: followee id) and `DeleteQueuedFollowPush :execrows` (args: follower id and followee id), both limited to queued rows not held by a worker.
- **pushkinds.go**: PushKindFollow is in PushKinds (six entries, last); ReservedPushKinds is empty.
- **pushcopy.go**: `FromSomeoneYouFollowWords = "From someone you follow."`; the follow case of BuildPushText (title: the pick, else the category, then " near " and the area; body "From someone you follow. {Distance} from your home area. Not confirmed yet.", or without the distance part with no home area; confirmed form "Confirmed by people nearby. {Distance} from your home area." or "Confirmed by people nearby."). No username, report words or coordinates.
- **pushdispatch.go**: `followBranch` (skips with no reporter account, follow_state already set, a false MayNotifyAboutReport(UpdateFollowedPosted, ...), outside PushInitialWindow; reads NotifyFollowersOf up to `FollowFanoutCap = 500`, Refreshes each follower, calls EnqueueFollowFromUpdates, then MarkFollowFanout fanned). It runs after criticalBranch and before reliableBranch. The doc comments state the order Critical, Follow, Reliable, Changes, confirmed update and that Follow wins over Reliable.

## Tests (database tests on the scratch database p9_25_a, zero skips in these)

- Store: TestMigration00024 (kind CHECK, one index for all three kinds, follow_state CHECK, Up, Down, Up), TestFirstPushIndexIncludesFollow (both orders and EnqueueInitialPush with all three kinds), TestEnqueueFollowFromUpdates, TestMarkFollowFanoutIsSticky, TestFollowRowsInEveryClosedList (changed skipped beside a follow row, a sent follow push counts in the hourly window, first_kind of the confirmed update is follow), TestDeleteQueuedFollowPushes.
- Service: TestBuildPushTextFollow, TestFollowPushQueuedFromBellRows, TestFollowPushOrderCriticalFollowReliable, TestFollowPushFannedAfterOnePass. Amended with dated notes (2026-10-30): TestPushKindsClosed (six kinds, reserved list empty), TestPushCapsAreNamed (FollowFanoutCap 500; a follow push is held at the hourly cap and not below it), TestBuildPushTextRefuses (the case "follow is reserved" replaced by two follow refusal cases). TestPushTextBudgets gained follow checks. TestBuildPushTextHasNoReporterTextField is unchanged and true.
- Mutation tried: running followBranch before criticalBranch fails TestFollowPushOrderCriticalFollowReliable. Swapping followBranch and reliableBranch does not fail it, and cannot: the bell step writes a followed_posted row first and the updates table lets one first alert row exist per account and report, so Reliable has no row to queue for a follower whichever branch runs first. The outbox index is the second guard (TestFirstPushIndexIncludesFollow).
- Full `go test ./... -p 1 -count=1 -v` on p9_25_a after the last code change: every package ok; the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. `go build ./...` and `go vet ./...` exit 0; `sqlc generate` leaves no diff. p9_25_a was created fresh and dropped.

## Interfaces widened (fakes and implementers)

- `PushDispatchQuerier` gained `MarkFollowFanout`, `EnqueueFollowFromUpdates` and `NotifyFollowersOf`. Implementers: `*sqlcgen.Queries` (the `var _` line) and `dispatchFake` in internal/service/pushdispatch_test.go (three methods, a new `bellFollowers` field, dated note). No other type implements it.

## Deviations from Plan

**1. [Rule 3 - Blocking] Test file outside the plan's list was touched.** `internal/service/pushdispatch_test.go` (dispatchFake gained the three methods) had to change to compile after the interface widening the plan itself calls for.

**2. Extra tests, same files.** push_test.go carries four tests beyond the two named (TestEnqueueFollowFromUpdates, TestMarkFollowFanoutIsSticky, TestFollowRowsInEveryClosedList, TestDeleteQueuedFollowPushes) so every new query and every widened list is exercised.

**3. Design choice, not a deviation: delete queries and the lease.** Both delete queries skip a row whose lease is in the future (database `now()`, like ClaimDuePushes), so a row mid-send is never deleted under its worker. They take no time argument.

## Notes for 09-26

- **The worker skips follow rows today.** `stillHolds` returns false for an unknown kind, so a queued follow row is skipped with `not_eligible` until 09-26 adds the `PushKindFollow` case. Nothing is sent by this plan.
- The 09-26 list stands: `stillHolds` follow case (MayNotifyAboutReport(UpdateFollowedPosted, ...), ActiveFollowNotify for the follower and the report's reporter, FolloweeStanding followable, no block, reporter still Reliable now through `reporterIsReliable` with ratings), `kindSwitchOn` (it returns false for any kind it does not list, so follow needs a case that is true: there is no account switch for follow, the bell on the follow is the switch), `confirmedBaseKind` (the switch gains follow), `buildText` (a follow case that passes the distance only when there is a home area), and step 5 (the home area and radius skips must not apply to follow or to a confirmed update whose base is follow).
- `heldByHourlyCap` already holds follow at `PushPerAccountPerHour` (default branch) and CountAccountPushesInWindow counts follow; TestPushCapsAreNamed pins both. The comment above `heldByHourlyCap` still says "Phase 9 adds follow"; reword it when editing that file.
- `ratingsWiringCalls` (internal/service/ratings_wiring_test.go): 09-26 owns the `WithPushWorkerSignals(ratingService)` entry (its comment says so). The dispatcher already takes `WithDispatchSignals`, which is already in the list.
- Cleanup queries to call from followend.go: `DeleteQueuedFollowPushesForFollowee(ctx, followeeID)` and `DeleteQueuedFollowPush(ctx, sqlcgen.DeleteQueuedFollowPushParams{FollowerID, FolloweeID})`; both return rows deleted. Add them to PeopleQuerier, FollowSweepQuerier and RatingQuerier and their fakes.
- The confirmed update (EnqueueConfirmedUpdates) now also fires for a sent follow row that said Not confirmed yet, and its `first_kind` is `follow`; the worker must treat that base like Reliable for the words (`followBody` has the confirmed form already).
- Skip reasons: push_outbox_skip_reason_valid is unchanged. If 09-26 needs a new reason for an ended follow, it needs its own migration (00025).
- The Policy date stays 2026-10-29. This plan stores no new personal data: the follow row holds only ids, kind and state.

## Commits

- 701a47da: feat(09-25): widen the push outbox lists for the follow kind
- aeb7401a: feat(09-25): add the follow kind and its lock screen words
- adf5ae21: feat(09-25): add the follow branch to the push dispatcher

## Known Stubs

None.

## Threat Flags

None. T-09-85 (lock screen names nobody; TestBuildPushTextHasNoReporterTextField unchanged), T-09-86 (shared first alert index, matching ON CONFLICT predicates, dispatcher order), T-09-87 (follow_state fanned once, FollowFanoutCap, the initial window, the hourly cap counting follow) and T-09-88 (pushes queued only from followed_posted rows the bell step wrote) are mitigated as planned.

## Self-Check: PASSED

Files exist: internal/store/migrations/00024_add_follow_push.sql, internal/service/followpush_test.go. Commits 701a47da, aeb7401a and adf5ae21 exist.
