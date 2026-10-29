---
phase: 09-followable-reporter-profiles
plan: 22
subsystem: service
tags: [follow-notes, updates, bell, notify-me, refresh]
requires:
  - phase: 09-21
    provides: followed_posted kind, InsertFollowedPostedAlert, NotifyFollowsFor, shared first alert index, FollowedPostedLine and FollowedPostedDetail
  - phase: 09-18
    provides: MutedPeopleIDs
  - phase: 09-05
    provides: the shared profile read pipeline pattern (narrow candidate reader on the querier)
provides:
  - FollowedLiveCandidates and ActiveFollowsAmong queries
  - ReportService.FollowedLiveReports (shared pipeline)
  - UpdatesService.refreshFollowedPosts and FollowedPostsCap
  - the follow note's words in UpdatesService.List
affects: [09-23, 09-25, 09-26]
tech-stack:
  patterns:
    - "a second producer step shares the first alert slot by returning the ids it wrote, and the next step skips exactly those"
    - "a narrow reader interface asserted on the reports dependency, so NearbyReader, the demo World and the alert fakes stay unchanged"
key-files:
  created:
    - internal/service/followedreports.go
    - internal/service/updatesfollow.go
    - internal/service/updatesfollow_test.go
  modified:
    - internal/store/queries/profile_reports.sql
    - internal/store/sqlc/profile_reports.sql.go
    - internal/service/updatesservice.go
    - internal/service/updatesalerts.go
    - internal/service/updatesservice_test.go
    - internal/service/updatesalerts_mute_test.go
key-decisions:
  - "The reports side is reached by a type assertion (followedLiveReader) on UpdatesService.reports, not by widening NearbyReader, so the demo World and every Nearby fake stay as they are; a reader without it ends the step quietly."
  - "UpdatesQuerier is widened by five methods (NotifyFollowsFor, FolloweeStanding, InsertFollowedPostedAlert, PublicProfilePathsForViewer, ActiveFollowsAmong); the compiler checks every implementer."
  - "A failure in the follow step does not stop the Reliable step; Refresh runs both and returns the follow error first."
  - "The list uses ActiveFollowsAmong (active follow, bell on or off), not NotifyFollowsFor, so turning the bell off while still following leaves old rows readable."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 22: Follow note producer and list words Summary

Refresh now writes one bell note per report a Notify me person posts after the bell went on, under the one predicate and every send time recheck, and the list words the note at read time and ends the name when the report finishes or the follow or profile is gone.

## What was built

- **profile_reports.sql**: `FollowedLiveCandidates` (accounts, now, since, row limit; ProfileReportCandidates columns plus the reporter account id; newest first; `now` is a parameter, never the database clock) and `ActiveFollowsAmong` (which of these people the reader still follows). Only profile_reports.sql.go changed in the generated code; models.go did not change; `sqlc generate` leaves no further diff.
- **followedreports.go**: `FollowedLiveQuery` and `ReportService.FollowedLiveReports`. It reads through a small `followedCandidateReader` asserted on the querier (the 09-05 pattern), keeps a row only when it is newer than its own reporter's Since (fail closed for an unknown reporter), runs `loadReadContext` with signals, `classify` and `baseView`, keeps only Active and Disputed views, and returns them oldest first from the newest `FollowedPostsCap` rows.
- **updatesfollow.go**: `FollowedPostsCap = 200` and `refreshFollowedPosts(ctx, accountID, now) (map[int64]struct{}, error)`: NotifyFollowsFor (cap `NotifyFollowsCap`), drop muted people, drop people `FolloweeStanding` says are not followable, `FollowedLiveReports`, then per view skip own reports, skip unless `MayNotifyAboutReport(UpdateFollowedPosted, ...)`, skip unless the trust object's reporter tag is Reliable now, then `InsertFollowedPostedAlert` with the Reliable insert's ceiling arguments; `pgx.ErrNoRows` is the normal answer. It returns the ids it actually wrote.
- **updatesservice.go**: Refresh calls it before `refreshReliableAlerts` and passes the written set. `List` builds line 1 with `FollowedPostedLine(username)`, line 2 with `FollowedPostedDetail(CategoryWord, SeverityWord)` and line 3 from the live trust object (reusing the 09-21 trust case), so the row updates in place. The extra reads (public usernames, active follows) run only when the page holds a follow note.
- **updatesalerts.go**: `refreshReliableAlerts` takes `followNoted` and skips exactly those report ids (review W3).

## Decision recorded (research Q9)

"Pending alerts" in SC5 means notes not yet written and phone notes still queued. A bell row already written stays as history and reads "This report is no longer available" (Available false, no username, no report id, no category, no trust line) as soon as any of these is true: the report is Resolved or expired (D-20), the follow has ended, the person went private, was deleted or blocked the reader.

## Tests (all on the scratch database p9_22_a, zero skips in these)

- Database: TestFollowBellWritesNote (Provisional and Live, none before the bell), TestFollowBellListableOnly (Withheld, removed, Hidden, Resolved, expired give none; a held report gets its note when it becomes listable), TestFollowBellOnePerReport (four refreshes, one row; a reader with the Reliable alert on also holds one first alert, the follow note), TestFollowBellSkipsMutedAndNotReliable (muted, rated below Reliable by five one star raters, private, ended follow), TestFollowBellOnlyAfterNotifyOn (a report posted while the bell was off gets none after it goes on again), TestFollowBellBounded (250 candidates read as 200, the newest, oldest first; one refresh writes exactly the hourly ceiling of 10), TestFollowBellRowWords, TestFollowBellEndedFollowReadsUnavailable (unfollowed, private, deleted, blocker; bell off with follow kept stays readable), TestFollowBellFinishedReportDropsName (resolved and expired).
- Fake: TestFollowBellFollowWinsButCeilingDoesNotCost (W3), TestFollowBellSkipsOwnAndLowerTagsAndMuted, TestFollowBellFakeBounds (60 bells read as NotifyFollowsCap), TestFollowBellNoReaderEndsQuietly, TestFollowBellErrorKeepsReliableStep, TestFollowBellListReadsOnlyForFollowNotes.
- Two mutations were tried to prove the tests bite: removing the Reliable tag check fails the rated-below test; making the list's live check always true fails the finished report test.
- Full `go test ./... -p 1 -count=1` on p9_22_a after the last code change: every package ok; the only skips are TestScreeningFixture, TestOfficialLive and TestGeocodeLive. `go build ./...` and `go vet ./...` exit 0. p9_22_a was created fresh and dropped.

## Interfaces widened (fakes and implementers)

- `UpdatesQuerier` gained five methods. Implementers: `*sqlcgen.Queries` (the `var _` line), `fakeUpdatesQuerier` in internal/service/updatesservice_test.go (five zero-value, non-recording methods, dated amendment note), which `alertFake` and `mutedErrFake` embed. No other type implements it.
- `refreshReliableAlerts` gained a parameter; its one direct test caller in internal/service/updatesalerts_mute_test.go now passes nil.
- `NearbyReader` is unchanged. `ReportService` gained `FollowedLiveReports`; the demo World and the Nearby fakes need nothing.

## Deviations from Plan

**1. [Rule 3 - Blocking] Test files outside the plan's list were touched.** `updatesservice_test.go` (fake gained the five methods) and `updatesalerts_mute_test.go` (one call site got a nil argument) had to change to compile after the signature and interface changes the plan itself calls for. Commit e36931c0.

**2. [Rule 3 - Executor slip] python3 was run once.** While orienting for Task 2 the command `python3 --version` was run by mistake, against the plan's rule 2. It only printed a version; no Python file was read, written or run, and nothing in the repo is Python.

**3. Design choice, not a deviation: `ActiveFollowsAmong` was added** to profile_reports.sql (the only SQL file the plan allows to change) because no batched read of the reader's active follows existed, and NotifyFollowsFor would have blanked old rows when the bell is turned off but the follow stays.

## Commits

- e36931c0: feat(09-22): write the follow note in Refresh for the people a reader has Notify me on for
- c5a4b6fb: feat(09-22): read the follow note's words at list time and end the name with the follow

## Notes for later plans

- **09-25 and 09-26 (phone)**: `refreshFollowedPosts` is the only writer of followed_posted rows in Refresh; the phone path reads those rows (push.sql still selects `kind = 'reliable_nearby'` only). The send time check is `ActiveFollowNotify(follower, followee)`. Follow wins the first alert slot per account and report, so a person who follows a Reliable reporter gets the follow note and never both; the dispatcher order is Critical, Follow, Changes, Reliable. Finished reports and ended follows must not push (the bell row already reads as gone then).
- **Handlers and page script (not touched here)**: `updateRowLabel` in internal/api/handlers/updates.go adds "{Category}, {Severity}." whenever `item.Category` is set, and the follow row's `ReportLine` is already that same sentence, so the accessible name of a follow row would say it twice. Pick one when the row is wired (for example skip the extra part for kind followed_posted). The wire already carries `Category`, `Severity` and `SeverityLevel` for the row's dot.
- **09-23**: nothing in this plan changes people.go or personprofile.go. A mute turns the bell off through TurnOffFollowNotify; the producer also reads the mute itself, so a mute that lands before the next poll still stops the note.
- **Policy plan**: no new stored personal data; follow notes store only the kind and the report id, as 09-21 recorded. The Policy date stays 2026-10-29.

## Known Stubs

None.

## Threat Flags

None. T-09-76 (the one predicate, Not confirmed yet wording, classify for moderation state), T-09-77 (the name is read at list time from public, clear, still followed people only), T-09-78 (NotifyFollowsCap, FollowedPostsCap, the hourly ceiling, one batched read per step) and T-09-79 (send time rechecks for followable, Reliable now, not muted, still followed) are mitigated as planned.

## Self-Check: PASSED

Files exist: internal/service/followedreports.go, internal/service/updatesfollow.go, internal/service/updatesfollow_test.go. Commits e36931c0 and c5a4b6fb exist.
