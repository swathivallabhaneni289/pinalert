---
phase: 09-followable-reporter-profiles
plan: 27
subsystem: docs
tags: [docs, trust-model, push, moderation, policy, legal]
requires:
  - phase: 09-26
    provides: the follow note on the phone, send time recheck and cleanup
  - phase: 09-29
    provides: the Includes ratings by N people line
provides:
  - TRUST-MODEL.md sections Reporter profiles, Stars and tapped ratings, Follows, mute, block and follow notes
  - fifteen new constants, eight identifiers and twelve cited tests guarded by TestTrustModelDocMatchesCode
  - PUSH.md Follow notes section, MODERATION.md Ratings and Names sections
  - Policy section Profiles, following and ratings and three new What Pinalert keeps lines
affects: [09-28]
tech-stack:
  added: []
  patterns:
    - "the doc guard test gets a helper per duration unit (p20Hours, p20Minutes) instead of printing 1 days"
key-files:
  created: []
  modified:
    - docs/TRUST-MODEL.md
    - docs/PUSH.md
    - docs/MODERATION.md
    - internal/service/trust_model_doc_test.go
    - web/templates/legal_body.html.tmpl
    - internal/api/handlers/legal_e2e_test.go
key-decisions:
  - "The Policy Last updated date and service.CurrentTermsVersion are NOT moved. They stay at 2026-10-29 until the first public deploy, when the owner moves both together (README step 9). The Policy text is done in full."
  - "Constants whose names live in package main (rateBurst, rateEvery) are described in prose and not tabled, because the service package test cannot read them."
  - "The dispatcher order in the docs is the code's: Critical, Follow, Reliable, then the change notes and the confirmed update."
requirements-completed: []
status: complete
completed: 2026-10-30
---

# Phase 9 Plan 27: Phase 9 documentation and the Policy text Summary

The trust model, push and moderation documents now tell the whole Phase 9 story as built (opt in profiles, tapped ratings with the up to 40 point limit and its answers, follows, mute, block and the follow note), and the Policy names the new public data, with the doc guard test holding the new constants, identifiers and tests to the code.

## What was built

- **docs/TRUST-MODEL.md** (commit 44fc6139):
  - `## Reporter profiles`: opt in, who can switch on, Name screening and its ASCII limit, the uniform not found, public lists split into named people and counts, Recent reports (live exact, finished at precision 5 with a date and no time and no link, D-20), the See this reporter row, evaluate on touch, deletion.
  - `## Stars and tapped ratings`: stars floor, the combined formula in integers, the half star blur and its honest limit (W5), who may rate (D-23), raters counted by distinct place (D-21), the 24 hour rule and the Rate bucket (10, then one every 30 seconds), the private min rule (D-22), the B4 paragraph in the required words with both worked examples (39 lifted to 70, 100 dropped to 60), what answers it (place rule, soft ending and restore, moderator removal, the trust panel line) and what does not (five places, T-09-103, TRUSTX-03).
  - `## Follows, mute, block and follow notes`: follow rules and the five end reasons, "at once" (D-24), block, mute scope (D-12), the Notify me bell, the follow note under the one rule, the shared first alert index, order and caps, the send time recheck and cleanup, what pending means (Q9).
  - Updated: the Display only passage, the Phone alerts kind list and cap sentence, the open gap paragraph (the 3.5 bar), the hand off lines, Honest limits (three bullets), Out of scope, an Upgrading line for migrations 00020 to 00024, and fifteen rows in the constants table.
- **internal/service/trust_model_doc_test.go**: `p20ConstFiles` and `p20Constants` gain `ProfileCoarsePrecision`, `MinRatersForStars`, `RatingPointsPerStar`, `RatingChangeGap`, `ModeratorRatingsCap`, `FollowingCap`, `PeopleListPageSize`, `BlockedListCap`, `MutedPeopleListCap`, `NotifyFollowsCap`, `FollowedPostsCap`, `FollowSweepInterval`, `ProfileReportsWindow`, `ProfileReportsCap`, `FollowFanoutCap`; new helpers `p20Hours` and `p20Minutes`; eight identifiers; twelve must cite tests.
- **docs/PUSH.md**: the Follow notes section (one row two channels, words, no username, distance and home area ignored, send time recheck, cleanup on every end path, order, limits, `FollowFanoutCap` described as it really behaves, mute scope), the lock screen line, the switch list (no Settings switch, D-11), the Counts limit, migrations 00020 to 00024 in owner check 0, and an owner check 7 for the follow note.
- **docs/MODERATION.md**: Names on a public profile, Ratings (the three moderator routes taken from `router.go`, the `removed_by` and `removed_at` audit stamp, the Remove all box, permanent rater removal, the follow restore with its conditions, the pending username moderator exception), three rows in the Limits table, and the Retention section.
- **web/templates/legal_body.html.tmpl** (commit 644606af): the section Profiles, following and ratings and three lines in What Pinalert keeps (follows, blocks and mutes; Notify me choices; ratings given).
- **internal/api/handlers/legal_e2e_test.go**: `TestPolicyNamesProfilesFollowingAndRatings` (new) and the h3 sequence pin amended (commit ddd3fda4).

## Verification

- `TestTrustModelDocMatchesCode` passes, all six subtests seen with `-v`.
- `TestTermsVersionFollowsPolicyDate`, `TestPolicyNamesProfilesFollowingAndRatings`, `TestLegalPagesRender`, `TestLegalPagesContactBlockOnlyWhenSet` pass. `TestUserVisibleCopyUsesPlainPunctuation`, `TestNoNoiseWordsInUserCopy` and the other legal tests in `./web/` pass.
- `make swag` leaves `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml` with no diff (review W16).
- `go build ./...` and `go vet ./...` exit 0.
- Acceptance greps: the three new headings print 3, `by up to 40 either way` prints 1, no non ASCII character and no long dash in any touched file.
- The owner wording check over the whole tracked tree prints nothing.
- Full `go test ./... -p 1 -count=1 -v` on the scratch database `p9_27_a`, run after the last code change: all 20 packages ok, 6588 PASS lines, 0 FAIL, and exactly three SKIP lines, the allowed `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture`. The first full run (before commit ddd3fda4) showed the two legal pin failures of deviation 5 and nothing else. `p9_27_a` was created by me (it did not exist) and dropped.

## Deviations from Plan

**1. [Owner instruction, overrides the plan] The Policy date and the terms version did not move.**
- **Plan said:** move `policyLastUpdated` and `CurrentTermsVersion` to the execution date together (D-17, T-09-93).
- **Done instead:** the Policy text was written in full, and both values stay at 2026-10-29. The standing rule of Phases 7.1 and 8, written in README step 9, keeps both fixed until the day of the first public deploy, when the owner moves them together.
- **Why this is safe:** `TestTermsVersionFollowsPolicyDate` still holds them equal. T-09-93 (people bound by terms they did not see) is met by the deploy day move, which covers the Phase 9 lines too, because nobody outside the owner can have agreed to any Policy text before that day.
- **Files not modified:** `internal/api/handlers/legal.go`, `internal/service/profile.go`. The new Policy test pins phrases and says so in a dated note; the date pin stays equality.

**2. [Rule 1 - Plan inconsistency] Dispatcher order.** The caller note and the plan summary text give "Critical, Follow, Changes, Reliable"; `pushdispatch.go` runs `criticalBranch`, `followBranch`, `reliableBranch`, `changesBranch`, `confirmedBranch`. The docs state what the code does: Critical, Follow, Reliable, then the change notes and the confirmed update.

**3. [Rule 3 - Blocking] `MutedListCap` is not the new constant.** The plan lists `MutedListCap`; `MutedListCap` (100, updates.go) already existed for muted reports. The person list cap is `MutedPeopleListCap` (500, people_mute.go, renamed by plan 09-18). The table and the guard use `MutedPeopleListCap`.

**4. [Rule 3 - Blocking] `rateBurst` and `rateEvery` are not in the constants table.** They live in `cmd/server/main.go` (package main), which the service package test cannot read. They are stated in prose (10, then one every 30 seconds) in the Stars section and in the Keeping this document true section, and in the MODERATION.md Limits table.

**5. [Rule 3 - Blocking] A second test pin broke.** `TestLegalPagesRender` and `TestLegalPagesContactBlockOnlyWhenSet` pin the exact h3 sequence of the Policy. Both failed in the first full run after the new section was added; the sequence list and the contact variant slice were amended with a dated note (commit ddd3fda4). No other assertion changed.

**6. [Minor] `FollowFanoutCap` wording.** Plans 09-25 and 09-26 found that the comment in `pushdispatch.go` says a follower past the cap loses only the phone note, while a follower whose own bell poll already wrote the row can still get a push. The docs describe the real behavior. The code comment was not touched (the file is not in this plan's list); see the notes below.

**7. [Minor] Constants table also holds `ModeratorRatingsCap`, `FollowedPostsCap`, `ProfileReportsWindow`, `PeopleListPageSize`, `BlockedListCap` and `MutedPeopleListCap`,** which the plan's interface list names or implies; `ProfileReportsWindow` renders as "30 days" through the existing `p20Days`, and `RatingChangeGap` and `FollowSweepInterval` through the two new helpers.

No auth gates. No stubs.

## Notes for 09-28

- Pushdispatch.go comment: reword the `FollowFanoutCap` comment (a follower past the cap whose own bell poll wrote the row can still get a phone note, because `EnqueueFollowFromUpdates` queues every followed_posted row of the report). The docs already say the true thing.
- Pruned first row case from 09-26: a confirmed update whose first follow row was pruned falls back to Critical or Reliable and is not rechecked against the follow facts. Not documented as a promise; mention it if the final gate looks at it.
- At the first public deploy (README step 9) the owner moves `policyLastUpdated` (internal/api/handlers/legal.go) and `service.CurrentTermsVersion` (internal/service/profile.go) together. `TestPolicyNamesProfilesFollowingAndRatings` and `TestTermsVersionFollowsPolicyDate` will keep passing when both move.
- The owner question left open by 09-15 stays open: a rater whose username is still pending is named on the moderator screen. MODERATION.md says so.
- Phase 9 added migrations 00020 to 00024; owner check 0 in PUSH.md and the Upgrading line in TRUST-MODEL.md list them.

## Threat model

T-09-93 mitigated by the deploy day move of README step 9 (see deviation 1). T-09-94 mitigated in TRUST-MODEL.md and the Policy (opt in only, finished reports coarse with the date only, lists split, lock screen without a username). T-09-100 mitigated by the B4 paragraph and its answers.

## Known Stubs

None.

## Commits

- 44fc6139 docs(09-27): write the trust model, push and moderation sections of Phase 9
- 644606af feat(09-27): add the Policy text for opt in profiles, ratings and public lists
- ddd3fda4 test(09-27): amend the Policy heading pins for the new section

## Self-Check: PASSED

All six modified files exist and carry the changes; commits 44fc6139, 644606af and ddd3fda4 are in `git log`; the scratch database is dropped; no background process or scratch file is left in the repo.
