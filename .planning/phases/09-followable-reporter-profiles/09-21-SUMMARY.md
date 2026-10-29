---
phase: 09-followable-reporter-profiles
plan: 21
subsystem: store, service
tags: [follow-notes, migration, updates, notify-me, predicate]
requires:
  - phase: 09-02
    provides: account_follows, people.sql, the ended_at rule for every follow read
  - phase: 09-18
    provides: person mutes (the service of 09-23 and 09-22 read them; nothing here does)
  - phase: 08-11
    provides: the comment update kinds, migration 00019, the way a new kind joins the closed lists
provides:
  - migration 00023_add_follow_notes.sql (Notify me columns, followed_posted kind, uq_updates_first_alert)
  - InsertFollowedPostedAlert and the six Notify me queries
  - UpdateFollowedPosted in the closed lists and in MayNotifyAboutReport
  - FollowedPostedLine, FollowedPostedDetail, NotifyFollowsCap
affects: [09-22, 09-23, 09-25, 09-26]
tech-stack:
  patterns:
    - "a new update kind joins the one predicate by widening the existing allowance condition, never a second branch"
    - "one partial unique index covers both first alert kinds and every ON CONFLICT predicate repeats it word for word"
key-files:
  created:
    - internal/store/migrations/00023_add_follow_notes.sql
    - internal/store/follownotify_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/queries/updates.sql
    - internal/store/queries/people.sql
    - internal/store/sqlc/models.go
    - internal/store/sqlc/updates.sql.go
    - internal/store/sqlc/people.sql.go
    - internal/store/updates_test.go
    - internal/service/updates.go
    - internal/service/updates_test.go
    - internal/service/updatescopy.go
    - internal/service/updatescopy_test.go
    - internal/api/handlers/updates.go
    - internal/api/handlers/activity.go
    - internal/api/handlers/activity_page_test.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "Migration number taken: 00023 (00022 was the newest, person mutes)."
  - "TurnOffFollowNotify and SetFollowNotify both work on one pair (follower, followee): 09-23's Mute turns off one pair, and the Settings and profile switch sets one pair on or off."
  - "SetFollowNotify keeps an existing notify_since when turned on again, so a repeat never moves the time that bounds which reports get a note."
  - "FollowAccount's reactivation also resets notify and notify_since, as a second guard on top of the ending statements, so a reactivated row can never come back with the bell on."
  - "EventLine returns an empty string for followed_posted; the list builder (09-22) writes line 1 with FollowedPostedLine and a username that is shown only when its screen is clear."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 21: Follow note data and closed lists Summary

A follow now carries a Notify me flag with its start time, a new update kind followed_posted sits in the closed lists and in the one notify predicate with the Reliable reporter alert's allowance, one index lets only one first note exist per account and report across both kinds, and the row words and the API enum are ready for the producer of the next plan.

## Migration number

00023_add_follow_notes.sql (the newest before it was 00022_add_person_mutes.sql). Up: `account_follows.notify` (default false) and `notify_since` with the CHECK `notify = (notify_since IS NOT NULL)`, two partial indexes on notify rows, `updates_kind_valid` re-created with the eight kinds of 00019 plus followed_posted, `uq_updates_reliable_once` dropped and `uq_updates_first_alert` created over `kind IN ('reliable_nearby', 'followed_posted')`. Down deletes the followed_posted rows first, restores the old index and the eight kind list word for word, and drops the columns. Phase 8's unread comment index is neither named nor touched.

## What was built

- **updates.sql**: InsertReliableAlert's ON CONFLICT now names the widened predicate; `InsertFollowedPostedAlert :one` is its sibling (same arguments, same hourly ceiling, same cursor seed, kind followed_posted).
- **people.sql**: `SetFollowNotify`, `TurnOffFollowNotify`, `CountNotifyFollows`, `NotifyFollowsFor` (followee_id, notify_since), `NotifyFollowersOf` (follower ids), `ActiveFollowNotify`; each filters `ended_at IS NULL`. `EndFollowPair` and `EndFollowsOf` clear notify and notify_since while ending, so an ended follow, and one restored later, has the bell off. `FollowAccount` reactivation clears them too.
- **service/updates.go**: `NotifyFollowsCap = 50`, `UpdateFollowedPosted` (events: right after UpdateReplyToYours, eight events; kinds: right before UpdateMore, nine kinds), and the allowance branch now reads `kind == UpdateReliableNearby || kind == UpdateFollowedPosted` (one condition).
- **service/updatescopy.go**: `FollowedPostedLine`, `FollowedPostedDetail`; EventLine returns "" for the new kind; UpdateTrustLine treats it in the same case as reliable_nearby.
- **handlers/updates.go, activity.go, docs/**: the kind enum gained followed_posted (swag regenerated; the license name still reads "Unlicensed (all rights reserved)"); the hub summary treats the follow note like the Reliable note.

## Tests

- New: TestMigration00023, TestFirstAlertIndexSharedByBothKinds (both orders, the ceiling, the cursor seed), TestFollowNotifyQueries (default off, repeat keeps the time, active only reads, both ending statements, followed again starts off), TestMayNotifyFollowedPosted (including a source guard that the allowance is one condition), TestFollowedPostedCopy, a hub summary subtest in TestFillUpdatesPart.
- Amended with dated notes ("Amended 2026-10-29 by Phase 9 plan 21 (D-09)"): TestUpdateKindsClosed (now 9 kinds, 8 events, with order checks), TestMayNotifyMatchesResolve (the follow note is the second kind on the alert side), TestUpdatesCopy (EventLine of the new kind is empty and the "every event has a line" loop skips it), and TestSwaggerSpecCoversRoutes in handlers/swagger_test.go (the enum list).
- TestFollowReadsFilterEnded and TestEndedFollowsAreInvisible still pass; TestNotificationsNoFollowOrOfficialRow and TestNoNoiseWordsInUserCopy untouched and passing.
- Full `go test ./... -p 1 -count=1` on the scratch database p9_21_a: every package ok (internal/api/handlers was rerun alone after the swagger pin fix and passed); about 2330 top level tests passed; the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. `go build ./...` and `go vet ./...` exit 0; `make sqlc` and `make swag` leave no further diff.

## Deviations from Plan

**1. [Rule 1 - Bug] The swagger enum pin in handlers/swagger_test.go**
- **Found during:** the full suite after Task 2.
- **Issue:** TestSwaggerSpecCoversRoutes pinned the old kind enum, so adding followed_posted to the enum made it fail. The file is not in the plan's file list.
- **Fix:** added followed_posted before more with a dated amendment note.
- **Files modified:** internal/api/handlers/swagger_test.go
- **Commit:** b342bbc5

**2. [Rule 2 - Missing critical] FollowAccount reactivation resets the bell**
- **Issue:** the plan clears notify in the two ending statements only. A path that left an ended row with notify on (none exists now) would restore with the bell on.
- **Fix:** FollowAccount's DO UPDATE also sets notify false and notify_since NULL. It is harmless today, and guards D-21 against a future ending path.
- **Files modified:** internal/store/queries/people.sql (and the generated people.sql.go)
- **Commit:** 6a026827

Other additions beyond the plan's list: internal/store/follownotify_test.go (the store test of the Notify me queries), internal/api/handlers/activity_page_test.go (hub summary subtest). Neither changes behaviour.

## Commits

- 6a026827: feat(09-21): add the Notify me flag, followed_posted kind data and shared first alert index
- 7866a569: feat(09-21): add followed_posted to the closed lists, the one predicate and the row words
- b342bbc5: test(09-21): amend the swagger kind enum pin for followed_posted

## Notes for later plans

- **09-22 (producer)**: call `MayNotifyAboutReport(UpdateFollowedPosted, ...)` first, then `InsertFollowedPostedAlert`; a `pgx.ErrNoRows` return means nothing was written (already had a first alert for the pair, or over the hourly ceiling). Only reports posted after `notify_since` (from NotifyFollowsFor) get a note. The followed_posted row counts toward UpdatesHourlyCeiling (it is not in the NOT IN list of the ceiling count), unlike the comment kinds. EventLine is empty for the kind, so the list builder must write line 1 with FollowedPostedLine and the report's category and severity words with FollowedPostedDetail, and show a username only when Screen is "clear".
- **09-23 (Notify me service and switch)**: SetFollowNotify takes (follower_id, followee_id, notify, now) and returns rows changed (0 means no active follow); it does not check the cap, the mute or the relation, the service does (CountNotifyFollows against NotifyFollowsCap). TurnOffFollowNotify takes a pair and returns 0 when the bell was already off. Profile can read the viewer's own bell with ActiveFollowNotify (no new query needed).
- **09-25 and 09-26**: NotifyFollowersOf(followee_id, row_limit) returns follower ids with the bell on, active follows only; ActiveFollowNotify(follower_id, followee_id) is the send time check. push.sql still selects only `kind = 'reliable_nearby'` for the phone channel; the follow note's phone path is theirs to add. AlertsFromReporterInWindow (the per reporter cap) still counts reliable_nearby rows only.
- **Policy plan**: new personal data stored: `account_follows.notify` and `notify_since` (a person's choice to be told about another person's posts, and when they chose it) and `updates` rows of kind followed_posted (the kind and the report id only, as for every update, no words or username stored). Both go away with the account's follows (DeleteAccountFollows) and with update pruning. The Policy date stays 2026-10-29.
- No Someone I follow posts switch exists in Settings (D-11); nothing here adds one.

## Known Stubs

None.

## Threat Flags

None. T-09-73 (the one predicate allows Live and Provisional only, Not confirmed yet until confirmed), T-09-74 (one index over both kinds, TestFirstAlertIndexSharedByBothKinds) and T-09-75 (the ON CONFLICT predicates are the index predicate word for word, exercised by the database test) are mitigated as planned.

## Self-Check: PASSED

Files and commits verified below.
