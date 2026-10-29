---
phase: 09-followable-reporter-profiles
plan: 01
subsystem: trust
tags: [reliability, stars, trust-model, go]
requires: []
provides:
  - ReliableMinScore = 70 (3.5 stars, D-05)
  - service.StarsTenths and service.StarsText (pure integer floor)
  - TRUST-MODEL.md 70 bar, anchor rows, floor rule
affects: [09-13, 09-14, 09-17]
tech-stack:
  added: []
  patterns: [integer floor for display numbers, one function cuts the tag]
key-files:
  created:
    - internal/service/stars.go
    - internal/service/stars_test.go
  modified:
    - internal/service/reliability.go
    - internal/service/reliability_test.go
    - internal/service/reporter_test.go
    - internal/service/trustfacts_test.go
    - internal/service/trustwording_test.go
    - docs/TRUST-MODEL.md
decisions:
  - "Bar is 70 points (3.5 stars) by owner decision D-05 of 2026-10-17; tag still cut only in reporterTagForScore."
  - "Stars text floors from integer points, never a float format, so 69 reads 3.4 beside Mixed record."
metrics:
  tasks: 2
  files: 8
status: complete
---

# Phase 9 Plan 01: Reliable bar at 70 and integer star text Summary

The Reliable reporter bar moved from 75 to 70 points (3.5 stars, D-05) in the one place that cuts the tag, and a pure integer star helper (`StarsTenths`, `StarsText`) was added so a shown number can never contradict the tag.

## What was built

- `internal/service/reliability.go`: `ReliableMinScore = 70` with a comment naming 3.5 stars, D-05 and the owner decision of 2026-10-17; `MixedMinScore = 35` unchanged (1.75 stars, shown 1.7, assumption A2). The `ScoreReliability` doc comment and invariant I1 are reworded for 70. `reporterTagForScore` is still the only comparison.
- `internal/service/stars.go` (new, pure): `StarsTenths(points)` is points / 2 with integer division, clamped 0 to 50. `StarsText(points)` builds "3.5" from tenths with `strconv` on integers.
- `internal/service/stars_test.go` (new): `TestStarsText` (table including negative and over 100 clamping) and `TestStarsTextWalkMatchesTag` (every score 0 to 100: stars at least 3.5 exactly when the tag is Reliable).
- `TestReliabilityTable`: new anchor rows 6C+2X = 70 Reliable, 9C+2X+2U = 70 Reliable, 4C+2U = 69 Mixed; the 4C+1X row (71) now expects Reliable; two uniform ageing checks added for 70 and 69. `expectedTag` reads the two constants.
- `docs/TRUST-MODEL.md`: boundary sentence, tag cut sentence, dated change note, a "Stars are the score divided by 20" paragraph (floor rule, 34 and 35 both read 1.7), the anchor rows, constants table row 70.

## Fixtures flipped by the 70 bar

Unit fixtures (each amended in place with a dated note "Amended 2026-10-29 by Phase 9 plan 01 (D-05)"):

| File | Test | Old | New |
|------|------|-----|-----|
| internal/service/reliability_test.go | TestReliabilityTable, "4 corroborated and 1 contradicted" (71) | Mixed | Reliable |
| internal/service/reliability_test.go | TestReliabilityTable, "6 corroborated, 1 unconfirmed, 1 contradicted" (73) | Mixed | Reliable |
| internal/service/reliability_test.go | TestReliabilityDecayAndWindow, uniform ageing, score 73 check | Mixed | Reliable |
| internal/service/reliability_test.go | TestReliabilityInvariants `expectedTag` | literals 75 and 35 | the two constants |
| internal/service/reporter_test.go | TestReporterSignalsGroupByAccount, account 102 (71) and "own resolve keeps the contradicted outcome" | Mixed record | Reliable reporter |
| internal/service/trustfacts_test.go | TestTrustFactsEveryState, reporter row, account 102 | Mixed record, caution tone | Reliable reporter, good tone |
| internal/service/trustwording_test.go | TestPanelWordingEveryState, reporter tags, account 102 | Mixed record | Reliable reporter |

Two table rows that scored exactly 75 were renamed from "sit exactly on 75" to "score 75, still Reliable (no longer a boundary)"; their expectations did not change.

DB seeded fixtures: none flipped. The full suite on scratch database p9_01_a was green right after the change. `internal/testutil/history.go` `SeedReliableReporter` scores 78 (unchanged, still Reliable), and `reliable_filter_e2e_test.go`, `reliable_alert_e2e_test.go` and `pushdispatch_db_test.go` needed no edit, so they are not touched by this plan.

Left alone as the plan says: `feed_test.go` (score 64), `profile_record_test.go`, `trustfacts_test.go` near 127 and `trustwording_test.go` near 337 ("Score 72" current score scale).

Coverage note for the reviewer: the shared fixture `p10GroupFixture` no longer produces a Mixed record reporter in `TestTrustFactsEveryState` and `TestPanelWordingEveryState` (account 102 was the only one). The Mixed wording is still pinned by `trustcopy_test.go` (the tag to label map), `moderationcopy_test.go`, the score 64 feed pin and the 69 row of `TestReliabilityTable`.

## Verification

- `go build ./...` and `go vet ./...` exit 0.
- `go test ./... -p 1 -count=1 -v` on p9_01_a: all packages ok, 0 FAIL, 6229 PASS lines, only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped. After the final gofmt only, `./internal/service/` and `./internal/testutil/` were rerun green.
- Acceptance greps: `ReliableMinScore = 70` once, `MixedMinScore = 35` once, `StarsText` and `StarsTenths` signatures once each, no `>= 70` literal in reliability.go, reporter.go or stars.go, "3.5 stars" in TRUST-MODEL.md, dated notes in three test files.
- `p9_01_a` created (it did not exist) and dropped. `p9_02_a` belongs to the parallel plan and was not touched.

## Deviations from Plan

None in behavior. One process note: the first Task 1 commit left a gofmt issue (a missing space in a test row); it was fixed with `gofmt -w` in the Task 2 commit, which is why `reliability_test.go` appears in both commits.

## Assumptions about Phase 8 and seams from Phase 7.1

This plan builds none of them. A-P8-1 to A-P8-10 and the 7.1 seams are recorded in the plan for later plans to grep. No assumption was found false here because none was consumed. `docs/TRUST-MODEL.md` was edited from the merged tree (it already has the Phase 7.1 "Signing in" section); only the reliability sections and the constants table row were touched.

## Threat model

T-09-01 mitigated (single cut in `reporterTagForScore`, walk test ties stars to the tag), T-09-03 mitigated (integer floor), T-09-02 accepted (owner decision D-05, anchor rows documented). No new threat surface.

## Known Stubs

None.

## Commits

- c955f876 feat(09-01): move the Reliable reporter bar to 70 and add integer star text
- d510993d test(09-01): amend fixtures and TRUST-MODEL.md for the 70 bar

## Self-Check: PASSED

stars.go, stars_test.go and this file exist; both commits are in `git log`.
