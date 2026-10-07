---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 01
subsystem: trust
tags: [voting, geo, haversine, distance-rule, sqlc, go]
requires: []
provides:
  - "MaxVoteDistanceKm, haversineKm, withinVoteRadius, ErrVoterTooFar, TooFarMessage in internal/service/geo.go"
  - "CastVote seven step order with reporter resolution exemption and D-01 distance refusal"
  - "ReportVoteContextRow.Latitude, Longitude, CreatedAt"
  - "testVoteContext, testReportLat, testReportLon fixtures in internal/service/trust_test.go"
affects: [03-02, 03-04, 03-08, 03-12, 03-18]
tech-stack:
  added: []
  patterns:
    - "fail closed radius test written as an inclusive less than or equal comparison"
    - "reporter exemption decided only from database reporter identity versus authenticated account"
key-files:
  created:
    - internal/service/geo.go
    - internal/service/geo_test.go
  modified:
    - internal/service/trust.go
    - internal/service/trust_test.go
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go
    - internal/store/votes_test.go
    - internal/api/handlers/votes_e2e_test.go
    - internal/api/handlers/feed_visibility_e2e_test.go
key-decisions:
  - "One too far sentence serves all four vote values; TooFarMessage carries no computed distance."
  - "voterGeohashPrecision stays 7, recorded as the deliberate D-03 choice, closing 02-RESEARCH assumption A3."
requirements-completed: [TRUST-05]
duration: about 45 minutes
completed: 2026-10-07
status: complete
---

# Phase 3 Plan 01: Nearby is real Summary

CastVote now refuses a vote cast from more than 1 km from the report, for all four vote values, before the voter cell is computed and before any row is written. The reporter's own resolve and reopen stay exempt, and NaN or Inf coordinates fail closed.

## What was built

- **Task 1** (`103b685`): `ReportVoteContext` selects `r.latitude, r.longitude, r.created_at` in the same single query that already reads the reporter, severity, category and expiry. `make sqlc` regenerated `votes.sql.go` (`Latitude float64`, `Longitude float64`, `CreatedAt time.Time`). `TestReportVoteContextReturnsCoordinates` reads all three back from real Postgres and checks the missing report case returns `pgx.ErrNoRows`. Voter C moved to (12.9686, 77.5946) at the five e2e call sites.
- **Task 2** (`9c1cb1f`): new `geo.go` and the seven step `CastVote`. Step 4 computes `isReporterResolutionVote` from `rc.ReporterAccountID` against the authenticated `in.AccountID`, for the resolution kind only. Step 5 expiry is never skipped by the exemption. Step 6 refuses with `ErrVoterTooFar` unless exempt. The `voterGeohashPrecision` comment records D-03 and A3. `testVoteContext` replaced the nine fixture literals. Added `TestCastVoteRejectsTooFar`, `TestCastVoteCheckOrder`, `TestCastVoteReporterExemption`.
- **Task 3** (`b8c92b7`): `geo_test.go` with `TestHaversine`, `TestWithinVoteRadius`, `TestWithinVoteRadiusBoundary`, `TestHaversineInsideBoundingBox`, `TestTooFarMessage`. All five passed on the first run against `geo.go` with the measured expectations, no tolerance loosened.

## Measured values

- Chennai to Bengaluru: 290.1720 km. Report to (12.9746, 77.5946): 0.3336 km.
- Boundary: (12.9805, 77.5946) is 0.9896 km (accepted); (12.9806, 77.5946) is 1.0008 km (refused). (13.05, 77.65) is 10.58 km (refused).
- 0.01 degrees of longitude: 1.0835 km at 13N, 0.9763 km at 28.6N, 0.9219 km at 34N.
- Voter C at (12.9686, 77.5946) is 0.3336 km south of the report, in precision 7 cell tdr1v8y, distinct from voter B's tdr1v9y (`assertDistinctCells` still guards every site).
- `TestHaversineInsideBoundingBox`: 2000 seeded samples per centre at 8, 13, 28.6, 34 and 37 degrees north, all accepted points inside the box, accepted count above the 500 floor.

## Verification

- `sqlc diff`: exit 0. `go vet ./...`: clean. `gofmt -l` over the eight Go files this plan touches: no output.
- No dash characters (U+2014, U+2013) in `geo.go`, `geo_test.go`, the rewritten `voterGeohashPrecision` comment or the `CastVote` declaration.
- Full suite on a fresh `p3_0301`, run with `go test ./... -count=1 -p 1 -v`, filtered to result lines: every package `ok` (api 2.210s, api/handlers 10.538s, auth, geocode, mailer, ratelimit, service 0.445s, session, store 9.796s, testutil, web), zero `--- FAIL`, zero `--- SKIP`. `p3_0301` was created fresh and dropped in the same command each time.
- No server was started. Ports 8080 and 8090 and the databases `pinalert_test` and `pinalert` were never touched.

## Behaviour to know about

A far vote over HTTP returns 500 until plan 03-04 maps `ErrVoterTooFar` to 403 with field `location` and `TooFarMessage()`. `votes.go` and its swag annotations were deliberately not changed here, and `make swag` was not run.

## Deviations from Plan

None. The plan executed as written. Tooling notes only: the sandbox rejected `env -u DATABASE_URL`, so the no-database runs used the shell's already empty `DATABASE_URL` (checked with `echo`), which is equivalent. Pre-existing dash characters elsewhere in `trust.go` (outside the two regions the plan names) were left alone as out of scope.

## Known Stubs

None.

## Threat Flags

None. T-03-01 and T-03-02 are mitigated and tested; T-03-03 (spoofed location) is accepted and no comment claims spoof resistance.

## Self-Check: PASSED

- FOUND: internal/service/geo.go, internal/service/geo_test.go
- FOUND commits: 103b685, 9c1cb1f, b8c92b7
