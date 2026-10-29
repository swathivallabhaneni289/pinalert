---
phase: 09-followable-reporter-profiles
plan: 28
subsystem: tooling
tags: [seed, final-gate, viewing, go]
requires:
  - phase: 09-27
    provides: the docs and Policy text the gate runs over
provides:
  - cmd/seedprofiles, the viewing seed for a scratch database
  - the Phase 9 final gate results and the owner's human check list
affects: []
tech-stack:
  added: []
  patterns:
    - "a seed that writes raw rows only where the services cannot (history in the past, votes) and uses the real services for the profile switch, ratings and follows, then reads the profiles back and fails if they do not read as described"
key-files:
  created:
    - cmd/seedprofiles/main.go
    - cmd/seedprofiles/main_test.go
  modified: []
key-decisions:
  - "The refusal reads the database name the way the driver does (pgx.ParseConfig), so a path, a dbname query value, the keyword form and the PGDATABASE fallback are all caught before any connection."
  - "The seed also refuses a server that is not on this machine (stricter than the plan)."
  - "The database test runs on the shared migrated test database through seed(), so it works on pinalert_test in CI, where run() would rightly refuse the name."
requirements-completed: [PROF-01, PROF-02, PROF-03]
status: complete
completed: 2026-10-30
---

# Phase 9 Plan 28: Viewing seed and final gate Summary

A `go run ./cmd/seedprofiles` seed fills a scratch database with made up people near Benz Circle for the owner's one viewing session, and the whole phase is green on a fresh scratch database (build, vet, 1604 node tests, 6577 Go passes with only the three allowed skips, the same again under TZ=UTC, and every V1 to V20 command).

## What was built

- `cmd/seedprofiles/main.go`, flags `-db` (required) and `-viewer-email` (optional), exit codes 0 done, 1 refused or failed, 2 bad usage (the shape of cmd/modrecord).
  - Refuses before connecting: pinalert, pinalert_test, p31_4_uat, p34_uat (any letter case, any spelling), any name not starting with `p9_`, an empty name, and any server that is not this machine. After connecting it checks `current_database()` is the name it was asked for, then checks the schema is migrated.
  - People (all made up, `<username>@example.com`, one shared password printed at the end, every account with a password, the current terms version, first run done, a clear username, a Name and the home area Benz Circle):
    - `ravi.k` public, Reliable from six corroborated reports (four older than 30 days inside the 90 day window, two finished ones the profile lists), one live report, six raters in six different precision 7 cells (4, 4, 3, 5, 4, 4 stars).
    - `anita.r` public, Reliable from six older reports, one live report, two raters (voter.a and voter.b), so no ratings part shows.
    - `new.reporter` public, New, one live report.
    - `private.one` profile off, one live report.
    - `voter.a`, `voter.b`, `rater.one` to `rater.six` only vote or rate.
  - Real services: `SetProfilePublic` (the real username and Name checks), `RatingService.Rate` (the real eligibility rule and cell counting), `PeopleService.Follow` (five follows between them). Raw SQL only for accounts, sessions, reports in the past and confirm votes.
  - Result on a fresh database: ravi.k reads Reliable reporter 4.2 stars with ratings by 6 people, anita.r Reliable reporter 4.4 stars, new.reporter New reporter, private.one not found. The seed reads all four back through `PeopleService.Profile` and exits 1 if any differs.
  - Second run: leaves the base data alone and repeats the real service steps safely. With `-viewer-email` (must end in `@example.com`, never echoed or printed, "sign up first" if no such account) it adds the viewer's confirm vote on ravi.k's live report from a seventh cell no rater uses, and a follow of anita.r with the bell off.
- `cmd/seedprofiles/main_test.go`: `TestSeedRefusesProtectedDatabases`, `TestSeedRefusesNonScratchNames` (neither needs a database; a connect function that fails the test proves nothing connected), `TestSeedBuildsViewingData`.

## Tasks and commits

1. 3d9d0138 feat(09-28): add the viewing seed for a scratch database
2. (Task 2 is the gate and the human checks; no file changed, as the plan allows.)
3. b999f0a2 docs(09-28): say when the seed's live reports end (live reports end 3 to 5 hours after the first run)

## Final gate (fresh scratch database p9_28_a, created and dropped; p9_28_seed likewise)

- `go build ./...` exit 0. `go vet ./...` exit 0.
- `node --test web/*.mjs`: 1604 tests, 1604 pass, 0 fail, 0 skipped.
- `go test ./... -p 1 -count=1 -v` on p9_28_a: exit 0, 6577 PASS lines, 0 FAIL, every package ok. The SKIP lines are exactly TestGeocodeLive, TestOfficialLive and TestScreeningFixture.
- The same run with `TZ=UTC` (the machine's own zone is IST, so the clock differs by 5 hours 30 minutes): exit 0, 6577 PASS, 0 FAIL, the same three skips. This stands in for the Linux check.
- Linux CI recipe: Docker was not tried (it crash-loops from an agent shell). The real Linux check runs on GitHub when the owner pushes.
- V1 to V20 (commands of 09-VALIDATION.md, each run once with -v, DB ones on p9_28_a, zero skips in all):

| V | Result | Tests seen |
|---|--------|-----------|
| V1 | ok, 3 pass | TestStarsText, TestStarsTextWalkMatchesTag, TestCombinePoints |
| V2 | ok, 32 | TestReliabilityTable, TestReliabilityInvariants |
| V3 | ok, 1 | TestRatingsNeverRateANewReporter |
| V4 | ok, 7 | TestTrustModelDocMatchesCode |
| V5 | ok, 2 | TestRatedReporterSameEverywhere, TestFeedSignalsMatchCastVoteSignals |
| V6 | ok, 2 | TestRatingRoutes, TestRatingRoutesRateLimit |
| V7 | ok, 54 | TestModeratorRatings, TestModeratorRatingsHiddenFromOthers, TestRatingsIgnoreRemoved |
| V8 | ok, 1 | TestProfileOptIn |
| V9 | ok, 1 | TestProfileNotFoundIsUniform |
| V10 | ok, 8 | the eight TestProfileReports tests of 09-05 and 09-07 |
| V11 | ok, 1 | TestPeopleLists |
| V12 | ok, 5 | TestFollowRoutes, TestBlockRoutes, TestMuteKeepsReportsVisible, TestMuteRoutes, TestMutedReportsRoute |
| V13 | ok, 9 | TestFollowEndsOnSwitchOff and the five TestFollowSweep tests |
| V14 | ok, 28 | the TestFollowPush and TestFollowBell tests |
| V15 | ok, 46 | TestBuildPushText (four of them), TestNoNoiseWordsInUserCopy |
| V16 | ok, 2 | TestPushKindsClosed, TestUpdateKindsClosed |
| V17 | ok, 689 | the whole web package |
| V18 | ok, 50 node tests | person_test.mjs and votes_stars_test.mjs |
| V19 | ok, 1 | TestSwaggerSpecCoversRoutes |
| V20 | ok, 52 | the TestAccessGate tests, including TestAccessGateBlocksPeopleRoutes |

- No flake seen in any run.
- The owner wording grep from the executor rules: empty. No em or en dash in the two new files. No Python, no image, no server started.

## Deviations from Plan

1. **Database name.** The plan's `p9_28_b` became `p9_28_seed` (the executor instructions override the plan). The database test does not need its own database at all after the final design: it runs `seed()` on the shared migrated test database from `testutil.NewTestDB` (p9_28_a in the full run, pinalert_test on GitHub CI, where `run()` would refuse that name). `run()` itself is also driven end to end in that test through a fake connect.
2. **Stricter than the plan:** the seed also refuses a server that is not this machine, and a `-viewer-email` that does not end in `@example.com` (owner rule of 2026-10-29: no real person's details on this Mac).
3. **Task 1 acceptance line** uses `env -i`, which the sandbox refuses; the tests ran as `export DATABASE_URL=...; go test ...` after confirming DATABASE_URL and PGDATABASE were empty. The no database run is `go test ./cmd/seedprofiles/ -short -count=1 -v` with DATABASE_URL empty (TestSeedBuildsViewingData then skips, the usual testutil rule; it ran with zero skips on the database).
4. **ravi.k has six finished reports, not two.** A Reliable record needs at least five judged reports, so four older ones (beyond the profile's 30 day window) build the record and two recent ones are what the profile lists, as the plan asks (two finished, one live).
5. Task 2 changed no file. The Linux step is the TZ=UTC run described above.

## Human checks for the owner (one viewing session)

Phone in the iPhone simulator (safaridriver and simctl screenshots), laptop in Mac Safari only, no iPad. Nothing here was run by this plan.

How to set up (scratch test copy, never ports 8080, 8090, 8776, 8778, 8781 or 8797; 8782 is free):

1. `createdb p9_view`, then `DATABASE_URL=postgres://localhost:5432/p9_view?sslmode=disable go run ./cmd/migrate`, then `go run ./cmd/seedprofiles -db postgres://localhost:5432/p9_view?sslmode=disable`. Do it just before the viewing: the live reports end 3 to 5 hours later.
2. Start the test copy on port 8782 with the scratch server recipe (own database, log mailer, development mode).
3. Sign up on the test copy with a made up address such as `viewer@example.com`, never your real email. Then run the seed again with `-viewer-email viewer@example.com`.
4. To post as a reporter, sign in as `ravi.k@example.com` with the shared password the seed printed.
5. Tear down afterwards: stop the server, `dropdb p9_view`, delete the server log.

**Check 1, the plan's first check.** The profile header order (round picture, username, name, home area, stars with both parts, tag with counts, member since, Followers and Following, Follow and the bell), finished report rows showing the date only, the "Includes ratings by N people." line in the trust panel, and every default of UI-SPEC "Defaults to confirm at the viewing session" 1 to 20, including the home area on a profile (Q1), the 40 point note about five raters in TRUST-MODEL (B4), stars or the Score line in popups (Q5), Muted people as its own row (Q6) and the new Policy text with the second agreement (Q11).

**Check 2, the plan's second check.** The follow flow by hand across two accounts (iPhone simulator and Mac Safari): opt in, follow, bell on, post from the reporter's account, the bell row reads Not confirmed yet and then the confirmed sentence, unfollow, block, remove follower, switch the profile off; and the star box after Yes, mark resolved and Skip.

**Check 3, the plan's third check.** On the owner's installed phone, after Phase 3.3 is live: one follow note per post with no username on the lock screen, updated in place when confirmed with no second alert, no sound and no vibration, and a tap opens the report.

**Open questions found while building (listed for the owner and the phase review, none fixed here):**

- **09-15, pending usernames.** Moderators see a rater's username on the Ratings screen even while that username is still being checked (pending), because the moderator acts by name. Keep it, or hide it and give moderators another way to remove a rating? Default taken: keep it. MODERATION.md says so.
- **09-17, no star box when the resolve closes the report.** The person whose "Yes, mark resolved" is the one that closes the report sees no star box, because the card goes away. They can still rate from the reporter's profile (See this reporter). Showing a box after the card is gone needs a host outside map.js.
- **09-26, pruned first follow row.** A confirmed update whose first follow row was pruned falls back to the Critical or Reliable base and skips the follow recheck (and the distance rule applies to it). Only reachable for a confirmed update still queued after its first row was pruned.
- **09-26 and 09-27, stale comment.** The `FollowFanoutCap` comment in `internal/service/pushdispatch.go` says a follower past the cap loses only the phone note; a follower whose own bell poll already wrote the row can still get a phone note. The docs say the true thing; the comment was not touched.
- **09-22, doubled category in a follow row's name.** `updateRowLabel` (internal/api/handlers/updates.go) adds "{Category}, {Severity}." whenever the row has a category, and a follow row's report line already says that, so its accessible name may say the category twice. Pick one.
- **09-10, one comment note after a block.** `CommentPushStillHolds` ignores `person_blocks`, so one comment note queued just before a block can still go out once (a small NOT EXISTS would close it).

Other reminders from the plan summaries, for the deploy day and not for the viewing: at the first public deploy the owner moves `policyLastUpdated` (internal/api/handlers/legal.go) and `service.CurrentTermsVersion` (internal/service/profile.go) together (README step 9).

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-09-92 is mitigated and tested without a database (four protected names in four spellings, the non p9_ names, the PGDATABASE fallback, a remote host, all refused before any connection). T-09-101 is mitigated: the seed prints usernames and paths only, refuses a non example.com viewer address, and never echoes an address or a connection string.

## Self-Check: PASSED

- cmd/seedprofiles/main.go and cmd/seedprofiles/main_test.go exist; commits 3d9d0138 and b999f0a2 are in `git log`.
- Databases p9_28_a and p9_28_seed were created fresh and dropped; no other database was touched, no server was started, nothing was pushed.
