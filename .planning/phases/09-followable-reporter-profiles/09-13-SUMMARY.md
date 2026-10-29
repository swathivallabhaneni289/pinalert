---
phase: 09-followable-reporter-profiles
plan: 13
subsystem: trust
tags: [ratings, stars, independence, postgres, sqlc, go]
requires:
  - phase: 09-01
    provides: ReliableMinScore 70, StarsTenths, StarsText
  - phase: 09-02
    provides: migration 00020, people.sql, endFollowsOf, SeedPublicReporter, SeedFollow, SeedBlock
  - phase: 09-03
    provides: SignalSource, SignalOption, LoadReporterSignals
provides:
  - migration 00021 (profile_ratings, accounts.rater_removed_at and rater_removed_by)
  - ratings.sql with 11 queries, raters counted by distinct voter cell
  - CombinePoints, TappedHalfStars, HalfStarsText, MinRatersForStars, RatingPointsPerStar
  - WithRatings, RatingTally, ReporterSignal OutcomeScore, Raters, TappedHalves
  - RatingService (the SignalSource for tapped ratings, eligibility, Rate, Clear, moderator removal with follow restore)
  - account deletion that removes the ratings given and received
affects: [09-14, 09-15, 09-17, 09-27, 09-29]
tech-stack:
  added: []
  patterns:
    - "one rating per distinct voter cell, the most recently written, read from the rater's latest qualifying vote"
    - "integer only combined points with a below five raters guard, cut by the one reporterTagForScore"
key-files:
  created:
    - internal/store/migrations/00021_add_profile_ratings.sql
    - internal/store/queries/ratings.sql
    - internal/store/sqlc/ratings.sql.go
    - internal/service/ratings.go
    - internal/service/ratings_test.go
    - internal/service/ratings_db_test.go
  modified:
    - internal/store/sqlc/models.go
    - internal/store/migrations_test.go
    - internal/testutil/db.go
    - internal/testutil/people.go
    - internal/service/stars.go
    - internal/service/stars_test.go
    - internal/service/reporter.go
    - internal/service/reporter_test.go
    - internal/service/accountdelete.go
    - internal/service/accountdelete_test.go
key-decisions:
  - "Migration number taken: 00021 (newest on main was 00020_add_people_profiles.sql)."
  - "OutcomeScore and TappedHalves are nil, and Raters is 0, whenever ratings did not apply (see Deviations 2)."
  - "A repeat tap of the same stars keeps updated_at, so it can neither reset the daily lock nor take a cell slot from another rater (see Deviations 3)."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 13: Tapped star ratings Summary

Tapped ratings now feed the reporter tag under the same independence rule as confirmations: one counted rating per distinct voter cell, integer arithmetic with a below five raters guard, the tag still cut only in `reporterTagForScore`, and `RatingService` is ready to be the `SignalSource` for every surface. Wiring it into the surfaces, the HTTP routes and the screens are the next plans.

## What was built

- **Migration 00021** (`00021_add_profile_ratings.sql`): `profile_ratings` with `profile_ratings_not_self`, `profile_ratings_stars_range`, `profile_ratings_removed_shape`, the two partial and plain indexes, and `accounts.rater_removed_at` and `rater_removed_by` with `accounts_rater_removed_shape`. Exact Down. `TestMigration00021` covers the file, every refusal (23514, 23505), Down and Up again. `profile_ratings` is in `testutil.Truncate`.
- **ratings.sql** (11 queries): `RaterMayRate`, `GetRating`, `UpsertRating`, `ClearRating`, `RatingTalliesForReporters`, `ModeratorRatingsForReporter`, `RemoveRating`, `RemoveRatingsByRater`, `MarkRaterRemoved`, `RestoreTagLostFollows`, `DeleteAccountRatings`. The tally runs three steps: live ratings (not removed, not cleared, rater not deleted and not removed), each given the cell of the rater's most recent qualifying vote (confirm, dispute, resolve, reopen, from any cell including the report's own), then one rating per (reporter, cell), the most recently written. It returns the reporter's `profile_public` flag whether the profile is on or off. Blocks are never read.
- **stars.go**: `MinRatersForStars = 5`, `RatingPointsPerStar = 20`, `CombinePoints` (below five raters returns the outcome as its first statement, then `(2 num + den) / (2 den)` clamped to 0 to 100), `TappedHalfStars`, `HalfStarsText`.
- **reporter.go**: `RatingTally`, `WithRatings`, `applyRatings`. A New reporter (no score) is never rated. A private profile can only be lowered (points are min(outcome, combined)). The tag comes from `reporterTagForScore` of the combined points. `reporterSignalFor` deep copies the two new pointers.
- **ratings.go**: `RatingService` with `SignalOptions`, `MayRate`, `YourRating`, `Rate`, `Clear`, `ModeratorList`, `ModeratorRemove`, `ModeratorRemoveRater`, the errors `ErrRatingNotAllowed` and `ErrRatingTooSoon`, the three P9-C28 sentences, `RatingChangeGap` (24 hours). A rating write re-reads the tag through `LoadReporterSignals` with the service itself as the source and ends follows with `endFollowsOf(..., FollowEndTagLost, ...)` when it is not Reliable. A moderator removal re-reads the tag and either restores the `tag_lost` follows ended since the removed rating was written (`RestoreTagLostFollows`, capped at `FollowingCap`) or ends them. `ModeratorRemoveRater` marks the rater first, so their ratings stop counting even if a later step fails.
- **Account deletion**: `DeleteAccountRatings` joins `AccountDeleteQuerier` and runs right after `DeleteAccountBlocks`.
- **Seeds**: `SeedRaters` and `SeedRatersInOneCell` in `testutil/people.go` (unique e-mails from a counter, cells from 0.003 degree latitude steps, the helper fails the test if two cells collide).

## Tasks and commits

1. 22bca257 feat(09-13): add the profile ratings table and cell counted queries
2. c5345e32 feat(09-13): combine tapped ratings into the points with the private min rule
3. 109efc16 feat(09-13): add RatingService, moderator removal with follow restore, and rating deletion

Migration number taken: **00021**.

## Tests

New, each seen with `-v`: `TestMigration00021`, `TestCombinePoints`, `TestTappedHalfStars`, `TestRatingsNeverRateANewReporter`, `TestCombinedTagCut`, `TestBelowFiveRatersOutcomesOnly`, `TestPrivateProfileRatingsOnlyLower`, `TestReporterSignalForCopiesRatingFields`, `TestRatingEligibility`, `TestWitnessAtTheSceneCanRate`, `TestRatersCountedByDistinctCell`, `TestRepeatTapDoesNotStealTheCell`, `TestRatingUpsertAndDailyLimit`, `TestRatingClear`, `TestRatingsIgnoreRemoved`, `TestRatingsIgnoreDeletedRater`, `TestSwitchingOffNeverRaisesTag`, `TestRatingWriteEndsFollowsSoftly`, `TestModeratorRemovalRestoresFollows`, `TestRemovedRaterCannotRateAgain`, `TestBlockKeepsRatings`, `TestDeleteAccountRemovesRatings`. That is 22 new test functions (the plan named 20; two extras).

Full gate on scratch database `p9_13_a`: `go test ./internal/store/ ./internal/service/ ./internal/testutil/ ./internal/api/... -p 1 -count=1` all ok (store, service, testutil, api, handlers). The only skip is `TestScreeningFixture` (one of the three allowed). `go build ./...` and `go vet ./...` clean, `sqlc generate` leaves no diff. A mutation check (changing the per cell grouping so every rater counts) made `TestRatersCountedByDistinctCell` fail on five assertions, then it was reverted. The scratch database was dropped.

TDD note: the three tasks were written test and code together inside one task each, not as separate red and green commits, so there is no separate red commit to cite.

## Deviations from Plan

**1. [Rule 1 - Plan inconsistency] Deletion step position and count.** As in plan 09-02, the plan says `wantDeleteOrder` ends with `DeleteAccountCommentFlags, DeleteAccountFollows, DeleteAccountBlocks, DeleteAccountRatings, RevokeAccountSessions, ScrubDeletedAccount`, but `DeleteAccountCommentFlags` is an early Phase 8 step with the updates, cursors, mutes and push steps after it. I kept every old step and put `DeleteAccountRatings` right after `DeleteAccountBlocks`. `wantDeleteOrder` is now 18 steps and `wantDeleteOrderKeyed` 19, with a dated note. The tail is Follows, Blocks, Ratings, RevokeAccountSessions, ScrubDeletedAccount, which is the intent (after every earlier step, in the one transaction, before the scrub). The only fake of `AccountDeleteQuerier` is in `accountdelete_test.go`; it gained the method.

**2. [Rule 1 - Plan inconsistency] OutcomeScore stays nil when ratings did not apply.** Task 2 says "when the ratings did not apply, OutcomeScore equals Score", and also that without `WithRatings` every signal equals what plan 09-01's suite expects. Those cannot both hold: dozens of existing tests compare whole `ReporterSignal` values by `reflect.DeepEqual` against literals with no `OutcomeScore`. I kept the stronger rule: `OutcomeScore` and `TappedHalves` are nil and `Raters` is 0 when ratings did not apply, and `Score` is then the outcome points. The later plans (09-14 and 09-29) read these fields only when the ratings applied (`Raters >= MinRatersForStars` and `TappedHalves != nil`), so they are unaffected. Only the "field shape" subtest of `TestReporterSignalsNewAndNone` needed an amendment (three new fields, dated note in `reporter_test.go`).

**3. [Rule 1 - Trust rule hole] A repeat tap must not reset `updated_at`.** As specified, `UpsertRating` would bump `updated_at` when the same stars are tapped again. That would reset the rater's own 24 hour lock and, worse, make that rating the "most recently written" one of its cell, so a rater could take the cell's slot from another rater just by tapping again. In the same stars, not cleared branch the statement now keeps `updated_at` (and succeeds). `TestRepeatTapDoesNotStealTheCell` and the repeat check in `TestRatingUpsertAndDailyLimit` pin it.

**4. [Rule 2 - Missing check] `MayRate` and `Rate` do more than `RaterMayRate`.** The SQL covers the removed rater, self and vote checks. The service also checks that the reporter is visible (`publicFace`) and that no block exists in either direction. Answers: an invisible reporter (private, pending, deleted, or one who blocked the rater) is `ErrPersonNotFound`, as for Profile; a reporter the rater blocked is `ErrRatingNotAllowed`. The tally query reads no blocks (`TestBlockKeepsRatings`).

**5. [Minor] `ModeratorRatingsForReporter` takes a row limit.** The query has `reporter_id` and `row_limit` (the service passes `ModeratorRatingsCap = 200`), so one read is bounded. It returns live raters with a username only, since the moderator acts by username. `Clear` follows the Unfollow pattern for names: a well formed unknown username succeeds and changes nothing, a malformed one is `ErrPersonNotFound`.

**6. [Minor] No separate pinned latest version.** The plan said `migrations_test.go` pins the latest migration; no such pin exists on main (older migration tests derive the version from the file name, and `TestMigration00020` still passes through `DownTo` across 00021's Down). Only `TestMigration00021` and the truncate list were added.

No auth gates. No threat flags beyond the plan's register.

## Notes for plan 09-14 and later

- Wiring: `RatingService` satisfies `SignalSource` (`var _ SignalSource = (*RatingService)(nil)`). Pass it to every `With*Signals` option, including the follow sweep and the push worker. Until then every surface still shows outcomes only.
- `ReporterSignal.OutcomeScore`, `Raters` and `TappedHalves` are set only when ratings applied. A private profile whose ratings only lowered the points also has them set (Raters at least 5), which is the "includes ratings by N people" case.
- Moderator routes (plan 09-15 or later) call `ModeratorList`, `ModeratorRemove` and `ModeratorRemoveRater` with the moderator's account id; they do not check moderator rights themselves.
- Handlers should map `ErrRatingNotAllowed`, `ErrRatingTooSoon` and the stars `ValidationError` to the three P9-C28 sentences; `ErrPersonNotFound` stays the uniform 404.
- `RestoreTagLostFollows` restores follows ended with `tag_lost` at or after the earliest removed rating's write time; that can include a follow lost through time decay in the same window. It is accepted (only `tag_lost` rows, only when Reliable again).
- Plan 09-27 documentation to write: the new constants (`MinRatersForStars`, `RatingPointsPerStar`, `RatingChangeGap`) are not yet in `docs/TRUST-MODEL.md` or the `p20Constants` guard; the 40 point limit (review B4), that only five different places are needed and nothing stops someone who controls accounts in five places (T-09-103), the half star blur limit (W5), and G6 deletion (deleting an account removes the ratings it gave, which can move another reporter's stars).
- Policy: the ratings are new personal data (who rated whom, with stars). The Policy text and its date (2026-10-29) are not changed here, because this plan does not say to. The Policy plan must name ratings, who can see them (moderators only, by name) and that deleting an account removes the ratings given and received.
- Interface widening: only `AccountDeleteQuerier` was widened; the single satisfier besides `*sqlcgen.Queries` is `fakeDeleteQuerier` in `internal/service/accountdelete_test.go`.

## Known Stubs

None.

## Self-Check: PASSED

All created files exist (`00021_add_profile_ratings.sql`, `ratings.sql`, `ratings.sql.go`, `ratings.go`, `ratings_test.go`, `ratings_db_test.go`); commits 22bca257, c5345e32 and 109efc16 are in `git log`.
