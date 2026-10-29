---
phase: 09-followable-reporter-profiles
plan: 14
subsystem: api
tags: [ratings, stars, wiring, routes, rate-limit, swagger, go]
requires:
  - phase: 09-03
    provides: SignalSource, the With*Signals options, LoadReporterSignals
  - phase: 09-04
    provides: PeopleService, PersonProfile, profilecopy helpers
  - phase: 09-06
    provides: FollowSweep and WithFollowSweepSignals
  - phase: 09-07
    provides: the people routes, PeopleRead and PeopleAction buckets, writePersonNotFound
  - phase: 09-13
    provides: RatingService, ReporterSignal OutcomeScore, Raters, TappedHalves, the P9-C28 sentences
provides:
  - the RatingService wired into every production tag reader (report, voting, auth, dispatch, follow sweep, people)
  - profile fields stars_from_reports, stars_from_ratings, may_rate, your_rating
  - POST /api/people/{username}/rating and /rating/clear behind the per account Rate bucket
  - RateLimits.Rate in all four places
affects: [09-15, 09-16, 09-17, 09-26, 09-27, 09-29]
tech-stack:
  added: []
  patterns:
    - "one ratingService value handed to every tag reader, pinned by a text reading test"
    - "one 409 sentence per refusal class so a route is never an oracle"
key-files:
  created:
    - internal/api/handlers/ratings.go
    - internal/api/handlers/ratings_e2e_test.go
    - internal/api/handlers/rated_reporter_e2e_test.go
    - internal/api/ratelimits_rate_test.go
    - internal/service/ratings_wiring_test.go
  modified:
    - cmd/server/main.go
    - internal/api/router.go
    - internal/api/handlers/ratelimit_gate.go
    - internal/api/handlers/people.go
    - internal/service/people.go
    - internal/service/personprofile.go
    - internal/service/personprofile_test.go
    - internal/service/profilecopy.go
    - internal/service/profilecopy_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "A rating answer for every not allowed reason is one 409 on field stars with RatingNotAllowedMessage; a change inside a day is its own 409 sentence; an invisible person is the uniform 404."
  - "Clearing your own rating never needs the person to be visible: a clear of a private or unknown profile is a quiet 204."
  - "The rating 429 reuses the people 429 sentence with the field rate_rating."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 14: Ratings on every surface, profile star parts and rating routes Summary

Tapped ratings are now live on every surface at once: one `RatingService` is the signal source of the report, voting, auth, dispatch, follow sweep and people readers, the profile carries both stars parts and the reader's own rating state, and two limited, documented routes give and remove a rating with one fixed refusal sentence.

## What was built

- **Wiring (task 1).** `cmd/server/main.go` builds `ratingService := service.NewRatingService(queries)` once and passes it as `WithReportSignals`, `WithVotingSignals`, `WithAuthSignals`, `WithDispatchSignals` (inside the push branch, where the dispatcher exists) and `WithFollowSweepSignals`; `Deps.Ratings` is set. `router.go` adds `Deps.Ratings` and builds the people service with `WithPeopleSignals(deps.Ratings)` (and `WithPeopleRatings`, see Deviations) when it is set.
- **Profile parts (task 2).** `PersonProfile` gains `StarsFromReports`, `StarsFromRatings` (both empty below five counted raters), `MayRate` and `YourRating`. `RatingReader` and `WithPeopleRatings` are new. `StarsFromReportsText`, `StarsFromRatingsText` (the tapped part through `HalfStarsText`, so 22 stars from 5 raters reads 4.5, never 4.4) and `StarsGroupLabelParts` are new in `profilecopy.go`. The profile JSON carries `stars_from_reports`, `stars_from_ratings`, `may_rate` and `your_rating`. `may_rate` and `your_rating` are read for another person's visible profile only, so your own page and the Blocked by you face read false and 0.
- **Routes and bucket (task 3).** `handlers/ratings.go`: `RatePerson` (strict body `{"stars": n}`, 204) and `ClearPersonRating` (empty body, 204). Errors: the stars `ValidationError`, a missing, mistyped or extra key all answer 400 on field `stars` with `RatingStarsMessage`; `ErrRatingNotAllowed` and `ErrRatingTooSoon` answer 409 on field `stars` with their P9-C28 sentences; `ErrPersonNotFound` is the uniform 404. `RateLimits.Rate` with `RateMiddleware()` (per account, keyed `acct:<id>`, no IP family), `rateBurst = 10`, `rateEvery = 30 * time.Second`, in the gate, `RateLimitsDefault`, `resolveRateLimits`, the main.go constants block and the RateLimits literal. `p20Tweak` opens it. `make swag` regenerated the three docs files (additions only; `@license.name` still reads "Unlicensed (all rights reserved)").

## Tasks and commits

1. 1aac1f45 feat(09-14): plug the ratings source into every tag reader
2. 1615e557 feat(09-14): add the two stars parts and the reader's rating state to the profile
3. 56b044d7 feat(09-14): add the rating routes and the per account Rate bucket
4. 8a088468 test(09-14): name the Rate bucket case so the route test run picks it up
5. 109ca602 test(09-14): keep the ratings source in the harness hooks that rebuild the report service

## Tests

New, each seen with `-v` and none skipped: `TestRatingsSourceWiredEverywhere`, `TestRatedReporterSameEverywhere`, `TestProfileStarsParts` (7 subtests), `TestProfileCopy` (new `stars_parts` subtest), `TestRatingRoutes`, `TestRatingRoutesRateLimit`, `TestRateBucketWiredEverywhere`. Extended: `TestAccessGateBlocksPeopleRoutes` and `TestSwaggerSpecCoversRoutes` (both rating routes), `p20Tweak`.

- `TestRatedReporterSameEverywhere`: a reporter with outcome 60 (5 corroborated, 3 contradicted) and five five star raters in five distinct cells reads 80, Reliable reporter, with the same tag, label, score, score sentence and counts on the feed, Focus, Past, a sixth account's confirm answer, a rater's Activity page and the loader the dispatcher uses; the profile reads the same words and stars, "3.0 from reports" and "5.0 from ratings by 5 people". After `ModeratorRemove` of one rating every surface reads 60, Mixed record together and the parts are empty. The private leg (outcome 86, five one star raters, profile off) reads 53, Mixed record everywhere, and still 53 with "4.3 from reports" and "1.0 from ratings by 5 people" after the profile is switched on (D-22).
- Mutation check: removing `WithVotingSignals` from the e2e harness made `TestRatedReporterSameEverywhere` fail on the vote surface (it read 60 Mixed against 80 Reliable); restored.
- Full gate on scratch database `p9_14_a`: `go test ./... -p 1 -count=1` ok for every package. The only skips are the three known ones: `TestScreeningFixture`, `TestOfficialLive`, `TestGeocodeLive`. After the last two commits the touched packages (`./internal/api/...`, `./internal/service/`, `./cmd/...`) were rerun and are ok. `go build ./...` and `go vet ./...` are clean, `make swag` leaves no diff. The scratch database was dropped.

## Harnesses wired (task 1 grep)

Wired to the ratings source: `newE2EServerWith` (reports_e2e_test.go, builds `ratings` and `Deps.Ratings`), `newTrustE2EServer` (trust_e2e_test.go), `newAuthE2EServer` (auth_e2e_test.go), `pwSetup` (push_flow_e2e_test.go, report, vote and dispatcher through `d.Ratings`), and the seven tweak hooks that replace `d.Reports` (and in one case `d.Votes`): reliable_alert_e2e_test.go, submit_screening_e2e_test.go, submit_idempotent_e2e_test.go, report_ratelimit_e2e_test.go, flag_note_e2e_test.go, moderator_e2e_test.go, username_e2e_test.go.

Left without the source on purpose, because they test gating, logging, routing or the demo and never rate anyone: `internal/api` gate_test.go, request_log_test.go, ip_backstop_test.go, official_flow_e2e_test.go, demo_session_test.go, official_refresh_routes_test.go; `internal/api/handlers` votes_body_test.go, nearby_validation_test.go, swagger_test.go (nil stores); `internal/store/reports_test.go`; `internal/demo` (world.go and parity_db_test.go, the practice world has no ratings); and the service package tests, which call `ReporterSignals` and the loader directly with their own source.

## Deviations from Plan

**1. [Rule 3 - Blocking] Files outside a task's list.** Task 2 also changed `internal/service/people.go` (the `ratings` field of `PeopleService`) and `internal/api/router.go` (`WithPeopleRatings(deps.Ratings)`); the router line therefore landed in the task 2 commit, not task 1, so task 1 compiles without `WithPeopleRatings`. Task 1 also touched `auth_e2e_test.go` and `push_flow_e2e_test.go`, and a later test commit touched seven hook files, all for the harness wiring the task requires.

**2. [Rule 1 - Plan inconsistency] Test numbers.** `TestProfileStarsParts` uses the 78 point `SeedPublicReporter` fixture, not the plan's 72, so its 12 rater case reads "3.9 from reports" and its total 3.9; the 72 case ("3.6 from reports") is pinned in `TestProfileCopy`. Five five star raters were also added to show the total moving (4.4).

**3. [Rule 1 - Acceptance] Test name.** The plan puts the 429 (V6) case inside `TestRatingRoutes`; I first wrote it as `TestRatingRateLimit`, which the acceptance `-run` pattern does not match. Renamed to `TestRatingRoutesRateLimit` (commit 8a088468).

**4. Clear never checks visibility.** `RatingService.Clear` (09-13) succeeds quietly for an unknown or private person, so a rater can always withdraw their own rating. The route test pins a 204 for a clear of a private profile. A malformed username is the uniform 404.

**5. The rating 429 reuses `PeopleLimitMessage`.** UI-SPEC P9-C13 lists "Too many tries. Wait {N} minutes and try again." as a client action error; the server sentence is the people one ("You're doing that very quickly. You can try again in N seconds.") with the field `rate_rating`. The UI plans should build their own line from `retry_after_seconds`, or the sentence can be reworded once for all people buckets.

No auth gates. No stubs. No threat flags beyond the plan's register (T-09-52 to T-09-55 are mitigated and tested: wiring test plus parity test, one 409 body for every refusal reason, the Rate bucket plus the 24 hour rule, strict bodies with the SameSite Lax cookie).

## Notes for plan 09-15 and later

- The push worker still has no ratings source; plan 09-26 adds `WithPushWorkerSignals(ratingService)` and must add it to the `ratingsWiringCalls` slice in `internal/service/ratings_wiring_test.go` (it reads main.go as text).
- Plan 09-27 documentation: `rateBurst` and `rateEvery` (10, then one every 30 seconds, per account) are not yet in `docs/TRUST-MODEL.md` or the constants guard; plan 09-13 already listed `MinRatersForStars`, `RatingPointsPerStar` and `RatingChangeGap`.
- The moderator routes belong to a later wave. The `RatingService` moderator methods (`ModeratorList`, `ModeratorRemove`, `ModeratorRemoveRater`) take the moderator id from the caller and check no rights, so each moderator handler must check moderator rights itself before calling them.
- The profile HTML page (09-08) does not yet draw the two parts or the rating control; the JSON carries `stars_from_reports`, `stars_from_ratings`, `may_rate` and `your_rating` for it, and `StarsLabel` already holds the whole accessible sentence.
- The e2e harness now sets `Deps.Ratings` in `newE2EServerWith`, so every e2e profile read fills `may_rate` and `your_rating`; tests that compare the whole profile JSON should expect both keys.
- `PersonProfile` stays comparable (`==`) as before; the new fields are strings, a bool and an int.

## Known Stubs

None.

## Self-Check: PASSED

Created files exist: `internal/api/handlers/ratings.go`, `internal/api/handlers/ratings_e2e_test.go`, `internal/api/handlers/rated_reporter_e2e_test.go`, `internal/api/ratelimits_rate_test.go`, `internal/service/ratings_wiring_test.go`. Commits 1aac1f45, 1615e557, 56b044d7, 8a088468 and 109ca602 are in `git log`.
