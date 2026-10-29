---
phase: 09-followable-reporter-profiles
plan: 26
subsystem: service
tags: [follow-notes, push, push-worker, send-time-recheck, cleanup, ratings-wiring]
requires:
  - phase: 09-25
    provides: the follow kind in the outbox, the follow branch of the dispatcher, the two delete queries
  - phase: 03.3
    provides: deviceSkipsRepeatPush, heldByHourlyCap, anyAppleDevice, AccountPushSentAboutReport
  - phase: 09-14
    provides: the rating service as SignalSource and TestRatingsSourceWiredEverywhere
provides:
  - the follow case of the push worker (followHolds, stillHolds, kindSwitchOn, confirmedBaseKind)
  - WithPushWorkerSignals, wired in main.go and in the wiring test
  - queued follow note cleanup inside endFollowsOf and endFollowPair, and on Mute and bell off
affects: [09-27, 09-28]
tech-stack:
  patterns:
    - "send time recheck: the worker reads the follow facts again before the predicate, nothing waits for a sweep"
    - "one place for the queued note cleanup: the two follow ending helpers"
key-files:
  created:
    - internal/service/pushworker_follow_test.go
    - internal/service/followpush_cleanup_test.go
  modified:
    - internal/service/pushworker.go
    - cmd/server/main.go
    - internal/service/ratings_wiring_test.go
    - internal/service/followend.go
    - internal/service/followend_test.go
    - internal/service/followsweep_test.go
    - internal/service/people.go
    - internal/service/people_mute.go
key-decisions:
  - "No new skip reason and no migration: every follow skip at send time is not_eligible (push_outbox_skip_reason_valid is unchanged)."
  - "The queued follow notes are deleted by the two helpers on every call, also when no follow was active (one cheap delete)."
  - "A confirmed update whose base is follow goes through the same send time follow check, because the cleanup deletes kind follow rows only."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-30
---

# Phase 9 Plan 26: Follow note, phone channel finished Summary

The push worker now sends a follow note only if everything it stands on still holds at send time (follow, bell, standing, no block or mute, Reliable reporter tag with the ratings), ignores home area and distance for it, replaces it in place when the report is confirmed, counts it toward the hourly cap, and every way a follow or a bell ends deletes the notes still waiting in the outbox.

## What was built

- **pushworker.go**
  - `PushWorkerQuerier` gained `ActiveFollowNotify`, `FolloweeStanding`, `PersonRelation`, `PersonMuted` and `ReporterHistoryReports` (with `CurrentVotesForReports` from `PushFactsQuerier` the worker is a `ReporterTagQuerier`).
  - `WithPushWorkerSignals(src SignalSource)` and a `signals` field.
  - `pushRowRun.base` (the row's kind, or `confirmedBaseKind` for a confirmed update) and `pushRowRun.followHolds`.
  - `sendRow` reads the follow facts once, before step 3, when the base is follow, through `(*PushWorker).followHolds`: reporter present and not the reader, `ActiveFollowNotify`, `FolloweeStanding` followable, `PersonRelation` with no block either way, not `PersonMuted`, and `LoadReporterSignals` with the worker's source says `ReporterReliable` now. It stops at the first false, cheapest first.
  - `stillHolds`: `case PushKindFollow: MayNotifyAboutReport(UpdateFollowedPosted, ...) && run.followHolds`; the confirmed update case also needs `followHolds` when its base is follow.
  - Step 5 (home area and radius) is skipped for base follow, with a comment naming Pitfall 4 and UI-SPEC Defaults 19. The distance is still computed, so a follower with a home area gets the distance words; with none the words leave them out (`BuildPushText` from 09-25).
  - `kindSwitchOn` returns true for follow (the bell on the follow is the switch, read by `followHolds`); `confirmedBaseKind` maps a first row of kind follow to follow.
  - The follow kind goes through the 3.3 gap fix's `deviceSkipsRepeatPush` (via the existing first push check, which covers every kind but the confirmed update) and `heldByHourlyCap`; no second copy. The `heldByHourlyCap` comment no longer says "Phase 9 adds follow"; the file header has a follow paragraph.
- **cmd/server/main.go**: `service.WithPushWorkerSignals(ratingService)` on the 3.3 worker. `ratingService` was already built before it.
- **ratings_wiring_test.go**: `"WithPushWorkerSignals(ratingService)"` in the main.go list, dated note (review W8).
- **followend.go**: `followEnder` gained `DeleteQueuedFollowPushesForFollowee` and `DeleteQueuedFollowPush`; `endFollowsOf` deletes the followee's queued follow notes right after the end, `endFollowPair` the pair's. Doc comments name SC5, Q9, D-24 and review W1. Block ends its two follows through `endFollowPair`, so it is covered.
- **people.go**: `PeopleQuerier` gained the two delete methods (it lists the end methods by hand, it does not embed `followEnder`); `SetNotify` off deletes the pair's queued notes after the write. `FollowSweepQuerier` and `RatingQuerier` embed `followEnder`, so they carry the methods without an edit.
- **people_mute.go**: `Mute` deletes the pair's queued notes after turning the bell off.

## Tests (database tests on the scratch database p9_26_a, zero skips in these)

- pushworker_follow_test.go: TestFollowPushNoHomeAreaStillSent (no home area, and a home area about 140 km away, both sent; body without and with the distance), TestFollowPushSendTimeRecheckDecay (the history sits at the edge of the 90 day window, the clock moves 20 minutes, the row is skipped not_eligible while the follow is still active and no sweep ran), TestFollowPushSkipsMutedOrBlocked (muted, blocked either way, bell off, follow ended, person went private; one control follower is sent), TestFollowPushReadsRatedTag (with the source the note is skipped, without it the same note is sent: the W8 drift), TestFollowPushConfirmedInPlace (same tag, normal urgency, not counted, the follow note is; also the send time check on the confirmed update), TestFollowPushHourlyCap, TestFollowPushAppleFirstOnly, TestFollowPushNeverHoldsBackCritical.
- followend_test.go: TestFollowEndHelpersDeleteQueuedNotes (pair and whole delete, a note about another person's report, a note a worker holds and the bell rows stay).
- followpush_cleanup_test.go: TestFollowEndsOnSwitchOff (V13: real dispatcher rows, switch off, outbox empty, worker sends nothing), TestFollowCleanupOnEveryEndPath (Unfollow, Remove follower, Remove all followers, switching the profile off, evaluate on touch, the sweep, a rating write, a moderator removal), TestFollowCleanupOnBlockMuteBellOff (Block both ways, Mute, bell off, and bell back on brings nothing back).
- followsweep_test.go: the fake sweep querier gained the two methods and TestFollowSweepPagesBatches now asserts one cleanup per ended followee. TestFollowSweepStopsOnCancel still compiles and passes.
- Mutations tried, each caught: step 5 without the follow exemption (7 tests fail), the confirmed update without `followHolds` (TestFollowPushConfirmedInPlace), the follow case without `followHolds` (decay, skips, rated tag), the delete removed from `endFollowsOf` (switch off, helper, five end paths), from `endFollowPair` (Unfollow, Remove follower, both Block cases) and from `Mute` (Mute case).
- `go build ./...` and `go vet ./...` exit 0. Full `go test ./... -p 1 -count=1 -v` on p9_26_a after the last code change: every package ok; the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. p9_26_a was created fresh (it did not exist) and dropped.

## Interfaces widened (fakes and implementers)

- `PushWorkerQuerier` (five methods): implementers `*sqlcgen.Queries` (the `var _` line) and `pwQuerier` in internal/service/pushworker_test.go. The five stubs live in pushworker_follow_test.go (same package), so pushworker_test.go is untouched. `OfficialPushWorkerQuerier` is a separate interface and was not changed.
- `followEnder` (two methods): implementers `*sqlcgen.Queries`, `fakeSweepQuerier` (followsweep_test.go), and through their interfaces `PeopleQuerier` (people.go, edited by hand), `FollowSweepQuerier` and `RatingQuerier` (embed it). No other type implements them; `go vet ./...` is clean.

## Deviations from Plan

**1. [Rule 3 - Blocking, test files] TestFollowPushAppleFirstOnly uses a sent changed row, not a Critical or Reliable one, as the earlier push.** The first alert index (`uq_push_outbox_first`, 00024) allows one critical, reliable or follow row per person and report, so an earlier Critical or Reliable push cannot sit beside a follow row for the same pair. The per device check counts a sent push of any kind (`AccountPushSentAboutReport`), so a sent changed row (for example an earlier comment note) exercises the same code. The behavior asserted is the plan's: the Google device gets the note about R under report-{id}, the Apple device gets nothing, the bell row stays, and a report with no earlier push reaches both.

**2. Extra tests beyond the named ones.** TestFollowPushReadsRatedTag (the W8 proof that the worker's source matters, T-09-99) and the sweep fake's cleanup assertion.

**3. No file outside the plan's list was edited.** `SetNotify` lives in people.go, as the plan expected.

**4. Test command form.** `env -i` forms are refused by the harness, so the tests ran as `export DATABASE_URL=postgres://localhost:5432/p9_26_a?sslmode=disable; go test ...` in one call, after confirming DATABASE_URL and PGDATABASE were empty.

## Notes

- **FollowFanoutCap comment (pushdispatch.go) not reworded.** The plan does not let this plan touch that file. The comment still says a follower past the cap loses only the phone note; a follower past the cap whose own bell poll wrote the row can still get a push, because `EnqueueFollowFromUpdates` queues for every followed_posted row of the report. Reword it when pushdispatch.go is next edited.
- **Pruned first row.** When the first push row was pruned, `confirmedBaseKind` falls back to Critical or Reliable for a confirmed update, so a confirmed update whose follow row is gone is not rechecked against the follow facts and the distance rule applies to it. This only concerns a confirmed update still queued after its first row was pruned; not changed here. Mention it to 09-28.
- **Delete runs on every helper call.** `endFollowsOf` also runs on every view of a person whose tag is gone (evaluate on touch), so that view now issues one cheap delete. No behavior change beyond that.
- **Real payload path checked.** internal/push has no closed list of payload kinds and sw.js reads none, so the follow kind needs no change there.
- The Policy date stays 2026-10-29. This plan stores no new personal data.

## Notes for 09-27 (docs)

- Say: a follow note is checked again at send time, so it stops the moment the follow, the bell, the profile, the Reliable reporter tag, a block or a mute goes; and a note still waiting is deleted at once when a follow or the bell ends.
- Say: a follow note ignores distance and the home area; a follower with no home area gets it without the distance words.
- Say: one notification per report; an iPhone or Mac Safari endpoint gets only the first push about a report; a confirmed update replaces the note in place under report-{id} on other browsers and is not counted toward the hourly limit; follow notes count, Critical is never held but counts.
- Every phone note is silent; keep the docs free of wording about sound or buzzing.

## Notes for 09-28

- Pruned first row case above; the skip reason is always not_eligible for a follow note that no longer holds.
- `WithPushWorkerSignals(ratingService)` is in main.go; TestRatingsSourceWiredEverywhere requires it.

## Commits

- aeb2d86b: feat(09-26): recheck follow notes at send time in the push worker
- 824feff6: feat(09-26): delete queued follow notes when a follow or a bell ends

## Known Stubs

None.

## Threat Flags

None. T-09-89 (send time follow facts read plus deletion on every end path, tests with a moved clock), T-09-90 (follow skips the distance rule, TestFollowPushNoHomeAreaStillSent), T-09-91 (follow counts toward the hourly cap, one note per report, in place update) and T-09-99 (WithPushWorkerSignals, wiring test, TestFollowPushReadsRatedTag) are mitigated as planned.

## Self-Check: PASSED

Files exist: internal/service/pushworker_follow_test.go, internal/service/followpush_cleanup_test.go. Commits aeb2d86b and 824feff6 exist.
