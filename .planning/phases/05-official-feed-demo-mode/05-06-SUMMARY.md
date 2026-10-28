---
phase: 05-official-feed-demo-mode
plan: 06
subsystem: demo
tags: [go, in-memory, seed, trust-mechanic, pretend-neighbours, sample-alerts]

requires:
  - phase: 05-official-feed-demo-mode
    provides: "05-03 Demo world: memQuerier, World, Store, SessionFor, SetClockOffset, WithWorldSetup"
provides:
  - "Seed(w), SeedSetup, PretendLat, PretendLon, PretendPlace and the 20 report Vijayawada flood sample"
  - "SampleAlert, SampleAlerts (3 invented official alerts for plan 05-19)"
  - "PlanNeighbours, PlannedVote, NeighbourInterval, offsetPoint"
  - "World.PostPractice, World.Vote, World.Nearby, World.ApplyDue, World.Activity"
affects: [05-10 Demo API, 05-13, 05-19 sample official alerts, 05-15 Demo page]

tech-stack:
  added: []
  patterns:
    - "State is derived, never painted: seeds are real SubmitChecked and CastVote calls on a world clock set into the past"
    - "Scripted actors as a pure function of elapsed time, applied before each read, no goroutine or timer"

key-files:
  created:
    - internal/demo/seed.go
    - internal/demo/seed_test.go
    - internal/demo/neighbours.go
    - internal/demo/neighbours_test.go
  modified:
    - internal/demo/world.go
    - internal/demo/world_test.go

key-decisions:
  - "The Budameru is named in report text only, because its Nominatim hit is a river centre near Kankipadu about 12 km east in another district."
  - "Bhupesh Nagar did not resolve, so no report sits there."
  - "ApplyDue never returns a vote refusal: a refused neighbour vote drops that report's plan, so a read cannot break because a pretend neighbour could not vote."
  - "World.Nearby forces the viewer to the visitor account and World.Vote forces the account to the visitor, so a caller cannot read or cast as anyone else. Vote does not override the position: plan 05-10 must send PretendLat, PretendLon."

patterns-established:
  - "Seed table holds reports and vote counts only; a test pins that the vote log equals the planned votes, so no state can be written another way"

requirements-completed: [ROBUST-07]

duration: 70min
completed: 2026-10-29
status: complete
---

# Phase 5 Plan 06: Vijayawada seed and pretend neighbours Summary

**Every Demo world opens on a 20 report moment of the 2024 Vijayawada floods whose Confirmed, Disputed and Ended states come from the real resolver, and a practice report climbs from "Needs 2 people" to "Confirmed by 2 people at different spots" as pretend neighbours vote through the real CastVote.**

## What was built

- `internal/demo/seed.go`: `PretendLat` 16.5424, `PretendLon` 80.6386, `PretendPlace = "Singh Nagar"`, the `seedReports` table (20 reports), `Seed`, `SeedSetup`, `SampleAlert`, `SampleAlerts`, `SampleKeyPrefix`.
- `internal/demo/neighbours.go`: `NeighbourInterval = 4 * time.Second`, `PlannedVote`, `PlanNeighbours`, `offsetPoint`.
- `internal/demo/world.go`: `PostPractice`, `Vote`, `Nearby`, `ApplyDue`, `Activity` and the type `Activity`, a mutex over the plans and the activity lists.
- Nothing in `internal/service` changed.

## The seed

20 reports: 11 within 1 km of the pretend position (one Disputed, one Ended among them) and 9 beyond 1 km. All nine postable categories appear; severities are 3 critical, 7 low and 10 medium. Votes: 23 confirms and 3 disputes from synthetic voters, each at its own precision 7 cell 300 to 750 m from its report. States seen through `World.Nearby` (All) and `Reports.History`: at least 3 (five by design) read as confirmed by people, the Disputed report is Hidden by 3 dispute cells against 0 confirm cells, and exactly one report is in the Past group. None is Retracted. The Ended report is a medium report posted 9 hours before the world start, with its votes cast at 9 hours before the start so CastVote does not refuse them as expired.

Near reports are spread around two resolved places by invented offsets of 80 to 400 m; far reports sit at four resolved places. The offsets are not claims about any street.

## Resolved coordinates

Resolved once on 2026-10-29 through https://nominatim.openstreetmap.org/search (format=json, limit=1, countrycodes=in, the place plus Vijayawada), User-Agent `Pinalert (https://github.com/swathivallabhaneni289/pinalert)`. 10 requests of the 12 allowed, run one at a time over several seconds, never at runtime.

| Name | Latitude | Longitude | What Nominatim returned |
|------|----------|-----------|-------------------------|
| Singh Nagar (pretend position) | 16.5424 | 80.6386 | A primary road (Vijayawada - Nuzvidu Road) in the area; no "Singh Nagar" neighbourhood node exists in OSM. A second query (Singh Nagar, NTR District) returned the Ajit Singh Nagar node. |
| Ajit Singh Nagar | 16.5395 | 80.6405 | neighbourhood node |
| Rajarajeswari Peta | 16.5316 | 80.6230 | neighbourhood node |
| Payakapuram | 16.5489 | 80.6496 | neighbourhood node |
| Krishnalanka | 16.5017 | 80.6271 | a health centre point inside the neighbourhood |
| Ibrahimpatnam | 16.5940 | 80.5225 | town node (the first query returned a school and was rejected) |
| Bhupesh Nagar | none | none | empty answer, no report placed there |
| Budameru | not used | not used | river relation centre near Kankipadu (16.5189, 80.7594), about 12 km east, another district; named in text only |

The pretend position falls in the district NTR (LGD code 749, Andhra Pradesh) by `areas.Default().Locate`; the sample alerts use that name.

## Trust sentences observed (TestNeighboursClimbToConfirmed)

Compared exactly against the server's words (all match trustcopy.go, so the test needed no change):

- t0 and t0 + 3 s: Provisional, `LeadSentence` "Needs 2 people at different spots to confirm it."
- t0 + 4 s: one neighbour has confirmed, "Needs 1 more person at a different spot to confirm it."
- t0 + 8 s: Live and confirmed by people, `ConfirmedSentence` "Confirmed by 2 people at different spots." (the lead sentence is empty once Live, so the confirmed sentence is the one compared there).
- At t0 + 30 s, and on a late read after 5 minutes, only 2 neighbour votes are in the log (the real threshold); the other two planned votes are dropped.
- A far seed report: `ErrVoterTooFar`, whose words are "Too far to vote. You need to be within 1 km of this report."

## Verification

- `DATABASE_URL= go test ./internal/demo/ -short -count=1` passes, also with `-race`; `go vet ./...` clean; gofmt clean.
- Acceptance greps: 'Budameru' 7, 'nominatim' 1, `PretendPlace = "Singh Nagar"` 1, SubmitChecked or CastVote calls in seed.go code 2, `NeighbourInterval = 4 * time.Second` 1, `IndependentAgreementThreshold` in world.go 2, `go func|time.NewTimer|time.AfterFunc` in non-comment lines of the package (tests included) 0, no en or em dash in the package.
- Full suite on disposable `p5_06` (migrated to version 11, dropped afterwards, `psql -lqt` count 0): 19 packages ok, the only skips TestGeocodeLive, TestOfficialLive and TestScreeningFixture, TestDemoParityWithSQL PASS (not skipped), TestCastVoteConcurrentSameAccountKeepsEveryRow PASS (no flake). One failure outside this plan, below.
- The shell had DATABASE_URL and all PG* variables empty before the run; DATABASE_URL was set inline only on the migrate, test and rerun commands for `p5_06`. pinalert_test was never touched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Bhupesh Nagar and the Budameru gave no usable point**
- **Found during:** coordinate lookup
- **Issue:** Bhupesh Nagar returned an empty list; the Budameru returned a river relation centre about 12 km east in another district; "Singh Nagar" resolved to a road and then to the Ajit Singh Nagar node.
- **Fix:** No report is placed at Bhupesh Nagar, the Budameru is named in text only and its bank reports sit at the resolved Ajit Singh Nagar point, and the pretend position is the Singh Nagar road point. Each is recorded in the seed.go header and the table above.
- **Files modified:** internal/demo/seed.go
- **Commit:** 00e847f3

**2. [Rule 2 - Missing detail] Mutex, plan cleanup and forced viewer**
- **Issue:** plan 05-10 will call these methods from concurrent requests, a refused neighbour vote must not break every later read, and the viewer must not be caller supplied.
- **Fix:** a mutex over the plans and the activity lists; `ApplyDue` drops a report's plan on a refusal instead of returning it (tested with an expired report); `Nearby` forces the viewer to the visitor account.
- **Files modified:** internal/demo/world.go
- **Commit:** 9599cf3b

**3. Added fields beyond the plan's artifact list**
- `SampleAlert` has an extra `Sample bool` (true on every entry) beside the `sample-` key prefix, so the "marked as a sample" rule is a value as well as a naming habit. Plan 05-19 builds from the `SampleAlerts` variable, so the extra field costs it nothing.
- `World.pendingNeighbourVotes` (unexported) exists for the plan-dropping test.

## Deferred Issues

`TestVotedReportsByAccountUsesTheAccountIndex` in `internal/store` fails on `p5_06` in the full run and on its own rerun: the planner chose `idx_votes_report_account_kind_created` instead of `idx_votes_account_kind_report_created` on the near empty tables. This plan cannot reach it: `git diff --stat 0c0ccfb3 HEAD` shows only `internal/demo/*` and this SUMMARY, `go list -deps ./internal/store/` holds no `pinalert/internal/demo`, and `git log ad7daf6..0c0ccfb3 -- internal/store` (from plan 05-03's last commit to the base) lists no commit, so nothing in the store, its migrations or `votes_test.go` changed after 05-03 reported a clean run. It reproduced on a second run of the test alone. The cause is not established (possibly planner statistics or the Postgres build on this machine). It is out of scope and not fixed, so the plan's "0 FAIL lines" criterion is not met by this one test, and the full suite is not green until it is looked at.

## Known Stubs

None. The sample alert text and the report text are intentionally invented, and every alert carries `Sample: true` and a `sample-` key. Plan 05-19 draws the Demo marks.

## Notes for the plans that use this

- Plan 05-10 must send `PretendLat, PretendLon` on every Demo vote. `World.Vote` forces the account but not the position, so the 1 km rule is the real one.
- Neighbour votes are stamped from the world clock at the moment they are applied (CastVote stamps its own time), so a late reader sees the votes land together; the sentences are unaffected.
- A confirm and a dispute by the visitor collapse to one row (the newer), as in SQL; `Activity` follows that.
- Seed reports and votes count against the 40 report and 400 vote ceilings: the sample uses 20 reports and 26 votes, leaving 20 practice reports and 374 votes.

## Commits

- 42c47c94 test(05-06): add failing tests for the Vijayawada seed and sample alerts
- 00e847f3 feat(05-06): Vijayawada flood seed through the real submit and vote services
- 77bcebe9 test(05-06): add failing tests for pretend neighbours and practice voting
- 9599cf3b feat(05-06): pretend neighbours confirm a practice report through the real vote code

## Self-Check: PASSED

seed.go, seed_test.go, neighbours.go, neighbours_test.go, world.go and world_test.go exist; the four commits exist on the branch; `p5_06` is dropped.
