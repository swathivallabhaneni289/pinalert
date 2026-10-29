---
phase: 09-followable-reporter-profiles
plan: 05
subsystem: service
tags: [profiles, coarsening, privacy, reads]
requires: ["09-03"]
provides:
  - "ReportService.ProfileReports: a person's last 30 days of reports, newest first, at most 100"
  - "CoarsePointAt, ProfileCoarsePrecision, CoarsenFinishedForProfile"
  - "viewInput.ProfileCoarse and ProfileCentre on buildFinishedView"
affects: [09-07, 09-08]
tech-stack:
  patterns: ["one shared read pipeline (loadReadContext, classify, baseView, buildFinishedView)", "narrow consumer interface instead of widening Querier"]
key-files:
  created:
    - internal/store/queries/profile_reports.sql
    - internal/store/sqlc/profile_reports.sql.go
    - internal/service/profilereports.go
    - internal/service/profilereports_test.go
  modified:
    - internal/service/coarsen.go
    - internal/service/coarsen_test.go
    - internal/service/historyread.go
key-decisions:
  - "Disputed reports are listed and labelled (A9)."
  - "Finished rows carry no report id, no description and no time; EndedOn is the date only, in a fixed IST offset."
  - "ProfileReportCandidates is reached through a small profileCandidateReader interface asserted on the service querier, not added to Querier."
requirements-completed: [PROF-02]
duration: 40 min
completed: 2026-10-29
status: complete
---

# Phase 9 Plan 05: Profile reports read Summary

The server read behind a profile's Recent reports: a candidate query keyed by account, the existing shared read pipeline, and a profile coarsener so a public username never leads to an exact place for a finished report.

## Tasks

| Task | Commit | What |
|------|--------|------|
| 1 | d864254e | CoarsePointAt, ProfileCoarsePrecision = 5, CoarsenFinishedForProfile (Rescue keeps precision 6, no text); buildFinishedView picks it when ProfileCoarse is set |
| 2 | 7f17800e | ProfileReportCandidates, ProfileReports, ProfileReportRow and page types, seven database tests |

## What was built

- Live rows (Active, Disputed while live) go through baseView: exact point, words and id as on the map. Status is ongoing, not_confirmed or disputed; DistanceText is DistanceWords of the distance to the exact point, empty with no centre.
- Finished rows (Resolved, Expired) go through buildFinishedView with ProfileCoarse. Afterwards the row loses its id, CreatedAt, ExpiresAt, WrittenAt, History.At and History.LifetimeText. The end date is computed first and kept as EndedOn (date only). The distance is measured to the coarse point ("About N km", at least 1), never to the exact point.
- The candidate query has no moderation, vote or view predicate and computes no distance; classify decides membership and fails closed, so a withheld report is never listed (T-09-18). LIMIT 101 detects the cut at 100.

## Tests

`TestCoarsePointAt`, `TestCoarsenFinishedForProfile` (no database), and `TestProfileReportsWindowAndOrder`, `NeverListsWithheld`, `FinishedAreCoarse`, `RescueKeepsD23`, `Distance`, `Cut`, plus an extra `StatusWords` (ongoing, not confirmed, disputed words and the disputed line). All pass on p9_05_a with zero skips in these tests. Full `./internal/service/` and `./internal/store/...` run on p9_05_a: pass, one skip (TestScreeningFixture, not touched by this plan). `TestResolveCallersMentionModeration` passes. `go build ./...` and `go vet ./...` clean. models.go did not change. p9_05_a dropped at the end.

## Deviations from Plan

**1. [Rule 3 - Blocking] ProfileReportCandidates is not on the Querier interface.** Adding it broke `internal/demo/memquerier.go` and several test fakes outside this plan's files. It is declared as a small `profileCandidateReader` interface in profilereports.go and asserted on `s.q`; the real `*sqlcgen.Queries` satisfies it, and any other querier gets `ErrProfileReadUnavailable`. Impact: a later plan that builds a profile service on a fake querier must implement that one method.

**2. [Rule 2 - Missing critical functionality] Extra clearing of times on finished rows.** Beyond the id, the plan's "times stay on the server" is enforced in the row itself (CreatedAt, ExpiresAt, History.At, History.LifetimeText cleared), so no mapper can send one.

**3. Added `TestProfileReportsStatusWords`** (not in the plan's list) to cover the status words and the disputed line.

**4. The candidate query has no `distance_km` column** (the plan said to mirror ReportForFocus). The service sets the distance itself so an exact distance to a finished point cannot exist.

## Decisions

- End dates use a fixed IST offset (UTC+05:30), since the repo has no displayed-date zone convention and no tz database dependency.
- `DisputedMayNotBeTrueLine` did not exist in Go (only in JS, "Disputed: may not be true"), so it is defined in profilereports.go with the same words.
- Cut is true when the candidate read returned more than 100 rows, even if a withheld row means fewer than 100 are shown.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-09-16, T-09-18 and T-09-19 are mitigated as planned; T-09-17 accepted.

## Self-Check: PASSED

Files exist: profilereports.go, profilereports_test.go, profile_reports.sql, profile_reports.sql.go, coarsen.go changes. Commits d864254e and 7f17800e exist.
