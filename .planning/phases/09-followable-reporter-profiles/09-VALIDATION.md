---
phase: 9
slug: followable-reporter-profiles
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-25
---

# Phase 9: Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Built from `09-RESEARCH.md` "Validation Architecture" and its common pitfalls.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (packages `service`, `handlers_test`, `web`, `api`) with a real Postgres for DB tests, `node:test` for `web/*.mjs`, manual iPhone simulator, Mac Safari and real phone checks |
| **Config file** | none. `Makefile` targets: `test`, `test-short`, `vet`. CI (`.github/workflows/ci.yml`) runs `go build ./...`, `go vet ./...`, `node --test web/*.mjs`, then `go test -v -p 1 ./...` against Postgres 16 |
| **Quick run command** | `env -u DATABASE_URL go test ./internal/service/ ./web/ ./internal/api/... -short -count=1 && node --test web/*.mjs` |
| **Full suite command** | `DATABASE_URL=postgres://localhost:5432/p9_<plan>_a?sslmode=disable go test ./... -p 1 -count=1`, then `node --test web/*.mjs`. The database is a disposable one, created for the run (refuse if the name exists) and dropped after |
| **Estimated runtime** | about 30 seconds quick, about 6 minutes full (an estimate at the size of the Phase 5 suite, recheck after Wave 0) |

Never run against `pinalert_test`, `pinalert`, `p31_4_uat` or `p34_uat`. Give every subagent its own database name and tell it so.

---

## Sampling Rate

- **After every task commit:** run the quick run command for the packages the task touched. A task that touches a DB backed package also runs that package's tests with `DATABASE_URL` set to the plan's own `p9_<plan>_a` database.
- **After every plan wave:** run `go vet ./...`, `node --test web/*.mjs`, and `go test ./... -p 1` on the plan's own scratch database (owner recipe: per plan databases, own merge gate).
- **Before `/gsd-verify-work`:** the full suite and `go vet ./...` must be green on a fresh scratch database. Then run the Linux CI check recipe from owner memory, then the owner's mockup viewing (phone in the iPhone simulator, laptop in Mac Safari only, no iPad) and the one real phone check.
- **Max feedback latency:** 60 seconds for the quick run.
- **Right after the 70 bar lands (plan 09-01):** run the full suite on a scratch database and record every fixture that flips from Mixed to Reliable in that plan's SUMMARY, because the fixtures at score 71 flip silently and `docs/TRUST-MODEL.md` pins 75 in five places.

---

## Per-Task Verification Map

Filled by the planner once the plans exist. Requirement level map from research. The Automated Command column holds a label; the commands are in the block below the table.

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| PROF-02 | `CombinePoints` table (r=4, 5, 6, all ones, all fives, rounding half up) and the `StarsText` walk from 0 to 100: the text is at least "3.5" exactly when the tag is Reliable (score 69 never reads 3.5) | unit | V1 | Wave 0 | pending |
| PROF-02 | Tag cut at 70 and Mixed at 35; fixture flips; 6C2X is exactly 70, 4C2U is 69 | unit | V2 | exists, amend | pending |
| PROF-02 | A New reporter stays New whatever the ratings | unit | V3 | Wave 0 | pending |
| PROF-02 | The doc constants table equals the code (70) | unit | V4 | exists, amend doc | pending |
| PROF-02 | Feed, vote response, history, Activity and the push dispatcher agree on a rated reporter; every production caller of `ReporterSignals(` carries the ratings option | integration (DB) | V5 | partly, amend | pending |
| PROF-02 | Rating eligibility (voted within the rule, not self, public profile, an unresolve does not count), upsert, clear, per account limit | integration (DB) | V6 | Wave 0 | pending |
| PROF-02 | A removed rating and a deleted rater stop counting; the moderator routes answer 404 to others | integration (DB) | V7 | Wave 0 | pending |
| PROF-01 | Opt in needs a username; the Name is screened and the switch fails closed on an unfinished check | integration (DB) | V8 | Wave 0 | pending |
| PROF-01 | Private, unknown, deleted and blocked give one identical 404 (same body and status) | integration (DB) | V9 | Wave 0 | pending |
| PROF-02 | Profile reports: 30 day window, Withheld and None never listed, finished reports coarse at precision 5, Rescue keeps 03.1 D-23, distance to the coarse point, finished rows are not links | integration (DB) | V10 | Wave 0 | pending |
| PROF-02 | Followers and Following split named versus "N others"; hidden entries never linked; keyset paging | integration (DB) | V11 | Wave 0 | pending |
| PROF-03 | Follow rules (Reliable now, public, no block, not self), unfollow, remove follower, block effects both ways | integration (DB) | V12 | Wave 0 | pending |
| PROF-03 | Switching off ends follows and queued follow pushes; tag loss ends them by sweep and at send time, with the clock moved so decay is tested and not only a rating change | integration (DB) | V13 | Wave 0 | pending |
| PROF-03 | Follow note: Live and Provisional only; one per account and report shared with the Reliable alert; "Not confirmed yet" then confirmed in place under the same tag with no second ping; a follower with no home area is still notified; hourly cap | integration (DB) | V14 | Wave 0 | pending |
| PROF-03 | `BuildPushText` follow case has no reporter text field, no username and no noise words | unit | V15 | exists, amend | pending |
| PROF-03 | Closed lists: `PushKinds` has six entries, `ReservedPushKinds` is empty, `UpdateKinds` | unit | V16 | exists, amend | pending |
| PROF-01, PROF-02, PROF-03 | Settings rows and their order, person menu, phone View my profile, size only names, 44px targets, registered icons, plain punctuation, noise words | contract | V17 | exists, amend | pending |
| PROF-01, PROF-02, PROF-03 | Star box after resolve (skippable), bell button, lists render with `textContent` | node | V18 | Wave 0 | pending |
| API docs | New routes documented | contract | V19 | exists, amend | pending |
| Gate | New routes answer 401 anonymously | integration (DB) | V20 | exists, amend | pending |

*Status: pending, green, red, flaky*

Commands for the table, one per row. They are written as plain text because a pipe inside a table cell needs escaping, and a copied backslash pipe is a literal pipe in a Go regular expression and matches no test. Commands marked DB need `DATABASE_URL` set to the plan's own scratch database; a skipped DB test is not a pass.

```
# V1
go test ./internal/service/ -run 'TestCombinePoints|TestStarsText' -short -count=1
# V2
go test ./internal/service/ -run 'TestReliabilityTable|TestReliabilityInvariants' -short -count=1
# V3
go test ./internal/service/ -run TestRatingsNeverRateANewReporter -short -count=1
# V4
go test ./internal/service/ -run TestTrustModelDocMatchesCode -short -count=1
# V5 (DB)
go test ./internal/service/ ./internal/api/handlers/ -run 'TestFeedSignalsMatchCastVoteSignals|TestRatedReporterSameEverywhere' -p 1 -count=1
# V6 (DB)
go test ./internal/api/handlers/ -run TestRatingRoutes -p 1 -count=1
# V7 (DB)
go test ./internal/api/handlers/ -run 'TestModeratorRatings|TestRatingsIgnoreRemoved' -p 1 -count=1
# V8 (DB)
go test ./internal/api/handlers/ -run TestProfileOptIn -p 1 -count=1
# V9 (DB)
go test ./internal/api/handlers/ -run TestProfileNotFoundIsUniform -p 1 -count=1
# V10 (DB)
go test ./internal/service/ ./internal/api/handlers/ -run 'TestProfileReports' -p 1 -count=1
# V11 (DB)
go test ./internal/api/handlers/ -run TestPeopleLists -p 1 -count=1
# V12 (DB)
go test ./internal/api/handlers/ -run 'TestFollow|TestBlock|TestMute' -p 1 -count=1
# V13 (DB)
go test ./internal/service/ -run 'TestFollowSweep|TestFollowEndsOnSwitchOff' -count=1
# V14 (DB)
go test ./internal/service/ -run 'TestFollowBell|TestFollowPush' -count=1
# V15
go test ./internal/service/ ./web/ -run 'TestBuildPushText|TestNoNoiseWordsInUserCopy' -short -count=1
# V16
go test ./internal/service/ -run 'TestPushKindsClosed|TestUpdateKinds' -short -count=1
# V17
go test ./web/ -short -count=1
# V18
node --test web/person_test.mjs web/votes_stars_test.mjs
# V19
go test ./internal/api/handlers/ -run TestSwaggerSpecCoversRoutes -short -count=1
# V20 (DB)
go test ./internal/api/ -run TestAccessGate -p 1 -count=1
```

**Executors treat "no tests to run" as a failure.** `go test -run` exits 0 when the pattern matches nothing, so a typo or a test not yet written passes silently. Run each Wave 0 command once before the code exists and see it fail for a missing test, then pass after.

---

## Wave 0 Requirements

- [ ] `internal/service/stars_test.go`: `CombinePoints` and the `StarsText` walk
- [ ] `internal/service/ratings_test.go`, `people_test.go`, `profilereports_test.go`, `followsweep_test.go`, `followbell_test.go`, `followpush_test.go`
- [ ] `internal/api/handlers/people_e2e_test.go`, `ratings_e2e_test.go`, `moderator_ratings_e2e_test.go`
- [ ] `web/person_contract_test.go`, `web/person_test.mjs`, `web/votes_stars_test.mjs`
- [ ] A seed helper in `internal/testutil` for a reporter with N judged reports plus raters (extends `SeedReliableReporter`)
- [ ] A seed for the owner's viewing session: at least one opted in Reliable reporter with 5 or more eligible raters, a username and a Name, built as a Go seed helper run with `go run` against a scratch copy only (extend `SeedReliableReporter`). It does not depend on Phase 5 demo accounts, which are still a draft
- [ ] Amend with dated owner notes, and do not delete the checks that still hold, the tests that pin "no profile yet": `TestNotificationsNoFollowOrOfficialRow` stays true, `TestPushKindsClosed` and `TestPushCapsAreNamed` flip, and the Settings and places pins (`places_contract_test.go`, `places_e2e_test.go`, `settings_rows_contract_test.go`)
- [ ] Phase 7.1 and Phase 8 outputs re-grepped at planning (username column and case rule, the reserved word function, the test helper names `newVerifiedClient` and `newE2EServer`, the `/?report=ID` link, the comments visible to viewer hook), and the plan fails if they are missing
- [ ] Framework install: none

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The owner's mockup viewing of the profile header in order (round picture, username, name, stars with both parts, tag with counts, member since, Followers and Following, Follow with the bell), plus the open choices: whether the home area shows (Q1), the optional disclosure that five raters move a score about 30 points (Q4), stars or the "Score N out of 100." line in popups (Q5), where mute is undone (Q6), the Policy text and a new terms version (Q11), and whether the 70 bar may ship alone (Q10) | PROF-01, PROF-02, D-05, D-13 | The owner judges by looking, and the UI step shows the D-13 mockup first | One batched session on a scratch test copy (own port, never 8080 or 8090, scratch database) loaded by the Wave 0 seed. Phone in the iPhone simulator, laptop in Mac Safari only, no iPad. Show it, do not ask in words |
| Profile page, Followers and Following windows, Settings rows in People and reports, and the star box after "Yes, mark resolved" at phone width | PROF-01, PROF-02, PROF-03, D-04, D-06 | Playwright WebKit passed layouts that iPhone Safari showed broken | iPhone simulator through `safaridriver` and `simctl` screenshots, plus Mac Safari |
| The follow flow by hand: follow, bell on, post from the reporter's account, the bell row reads "Not confirmed yet" then the confirmed sentence, unfollow, block, remove follower, switch the profile off | PROF-03, D-08, D-09 | End to end feel and timing across two accounts | Scratch test copy with the seeded reporter, two browsers (iPhone simulator and Mac Safari) |
| An installed phone gets one silent follow note, with no username on the lock screen, updated in place when the report is confirmed with no second ping, and a tap opens the app | PROF-03, D-09, D-10 | Needs a real installed phone | Owner's phone, after Phase 3.3 is live. No sound and no vibration |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency under 60 seconds
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
