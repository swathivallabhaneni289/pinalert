---
phase: 05-official-feed-demo-mode
plan: 03
subsystem: demo
tags: [go, in-memory, sandbox, trust-mechanic, differential-test, postgres]

requires:
  - phase: 03.3-installable-app-and-phone-alerts
    provides: the final service.Querier, service.VotingQuerier and the report and voting services the world runs on
provides:
  - internal/demo memQuerier implementing service.Querier and service.VotingQuerier on the world clock
  - World (real ReportService and VotingService, no screener, no notifier), SetClockOffset, SessionFor
  - Store of worlds (100 worlds, 30 minute idle drop, LRU eviction, 22 character crypto ids, ValidVisitID)
  - TestDemoParityWithSQL, the drift guard between the Go twin and the SQL
affects: [05-06 seed and neighbours, 05-10 Demo API]

tech-stack:
  added: []
  patterns:
    - "In memory twin of the SQL behind a consumer-defined interface, kept honest by a differential test on a real scratch database"
    - "Missing row that satisfies errors.Is(err, pgx.ErrNoRows) with no pgx import, by an Is method matching the sentinel message"
    - "Lazy idle expiry and LRU eviction under one lock, no goroutine or timer"

key-files:
  created:
    - internal/demo/memquerier.go
    - internal/demo/world.go
    - internal/demo/store.go
    - internal/demo/memquerier_test.go
    - internal/demo/world_test.go
    - internal/demo/store_test.go
    - internal/demo/parity_db_test.go
  modified: []

key-decisions:
  - "The twin's NearbyReports distance is the SQL's law of cosines expression (named haversineKm as the plan lists), not the service's true haversine, so DistanceKm agrees with the database."
  - "Missing rows answer pgx.ErrNoRows through an unexported error type with an Is method that matches the sentinel message while unwrapping the target, because the package must not import pgx (D-02). Tests pin it against the real sentinel and through CastVote, FindReplay and ReportForFocus."
  - "The whole InsertReportIdempotentParams is kept per report, so no pgtype import is needed for ClientWrittenAt."
  - "A sessions map (session id to account id) mirrors the LEFT JOIN on sessions, exposed as World.SessionFor(accountID), which returns the session id demo-<id> and binds it. Plans 05-06 and 05-10 must post and vote through it."
  - "The request id conflict is checked before the report ceiling, so a replay still works in a full world; a refused insert changes nothing and advances no id."

patterns-established:
  - "World clock = wall clock plus an atomic offset; the querier reads it where the SQL reads now() and uses the parameter where the SQL takes the instant as a parameter"

requirements-completed: [ROBUST-07]

duration: 55min
completed: 2026-10-28
status: complete
---

# Phase 5 Plan 03: The Demo world (in memory querier, world, store) Summary

**The real ReportService and VotingService now run unchanged on a per visit in memory world: bounded at 40 reports, 400 votes and 100 worlds, with no database import, and proven equal to the SQL by a differential test on a scratch database.**

## What was built

- `internal/demo/memquerier.go`: `memQuerier`, `ErrWorldFull`, `MaxReportsPerWorld = 40`, `MaxVotesPerWorld = 400`, helpers `haversineKm`, `newestPerReportAccountKind`, `inBox`, `capRows`.
- `internal/demo/world.go`: `World`, `NewWorld`, `Now`, `SetClockOffset`, `SessionFor`, the account id constants (`VisitorAccountID` 1_000_000_001, `NeighbourAccountBase` 1_000_000_100, `SeedReporterAccountBase` 1_000_000_200, `SeedVoterAccountBase` 1_000_000_300) and `VisitorSessionPrefix = "demo-"`.
- `internal/demo/store.go`: `Store`, `StoreOption`, `NewStore`, `WithStoreClock`, `WithWorldSetup`, `ValidVisitID`, `Create`, `Get`, `Len`, `MaxWorlds = 100`, `WorldIdleTTL = 30 * time.Minute`, `VisitIDBytes = 16`.
- Nothing in `internal/service` changed.

## Each memQuerier method and the query it mirrors

| Method | Mirrors | Twin behaviour |
|--------|---------|----------------|
| InsertReportIdempotent | reports.sql InsertReportIdempotent | Conflict on (request_account_id, request_id) returns a no rows error; then the 40 report ceiling; created_at restamped from the world clock, expires_at kept at the same lifetime; microsecond stamps |
| ReportByRequest | reports.sql ReportByRequest | Scoped to the caller's account and request id (T-04-05); no rows error otherwise |
| InsertModerationCheck | moderation.sql InsertModerationCheck | Stores nothing: the Demo has no screener, so no pending or borderline row is asked for, and no read needs the table |
| NearbyReports | reports.sql NearbyReports | expires_at after the world clock, inclusive box, SQL distance expression, distance ascending then id |
| ExpiredReportsInWindow | history.sql ExpiredReportsInWindow | expires_at at or before Now, after WindowStart (parameters), box, radius, expiry descending then id descending, row cap |
| OpenReportsInArea | history.sql OpenReportsInArea | expires_at after Now (strict), box, radius, distance ascending then id ascending, row cap |
| ReportForFocus | history.sql ReportForFocus | By id, no expiry or area predicate, distance from the passed point, no rows error when missing |
| ReporterAccountsForReports | reports.sql ReporterAccountsForReports | One row per existing requested report, nil account for an unbound session (LEFT JOIN) |
| ReportExtrasForReports | reports.sql ReportExtrasForReports | One row per existing requested report: moderation state, written time, viewer flag and mute always false (the Demo has neither) |
| ReporterHistoryReports | reliability.sql ReporterHistoryReports | Per requested account, expired within the window by the passed instants, newest first, capped per account, ordered by account |
| InsertVote | votes.sql InsertVote | Append with a rising id, created_at from the world clock, foreign key kept (missing report is an error), 400 vote ceiling |
| CurrentVotesForReports | votes.sql CurrentVotesForReports | DISTINCT ON (report, account, kind), newest created_at then highest id wins, ordered by report, account, kind |
| ReportVoteContext | votes.sql ReportVoteContext | One report's severity, category, expiry, reporter (nil for unbound), position, created_at, moderation state; no rows error when missing |
| ResolutionUndoCounts | undos.sql ResolutionUndoCounts | Counts every row of the account and kind per report (not only the newest), no row for a report with no match, ordered by report |

## Verification

- `DATABASE_URL= go test ./internal/demo/ -short -count=1` passes (23 tests); `go vet ./...` clean; `gofmt -l internal/demo` empty.
- Named tests all pass: TestMemQuerierImplementsInterfaces, TestNearbyReportsSemantics, TestCurrentVotesNewestWins, TestPastWindowReads, TestWorldCaps, TestCastVoteRunsRealRules, TestWorldClockOffset, TestStoreCreateGet, TestStoreTTLAndLRU, TestValidVisitID, TestDemoPackageHasNoDatabase, TestDemoParityWithSQL, plus extras (bounding box edges, missing rows, undo counts and history, request id replay, replay and no rows through the real services, world isolation, constants).
- `TestDemoParityWithSQL` on the disposable `p5_03`: PASS, zero `--- SKIP` lines in the package. A mutation check (inverting the newest vote rule in the twin) made it fail with a field by field diff, so it does detect drift.
- Full suite on `p5_03` (`go test ./... -p 1 -count=1 -v`): 0 FAIL lines, 17 packages ok, the only skips TestGeocodeLive and TestScreeningFixture. TestCastVoteConcurrentSameAccountKeepsEveryRow did not flake. `p5_03` dropped afterwards (`psql -lqt` count is 0). The shell had DATABASE_URL and all PG* variables empty; pinalert_test was never touched.
- No dash characters in the package, no `math/rand`, `crypto/rand` named once in store.go, no `go func`, `time.NewTicker` or `time.AfterFunc` in store.go, no WithScreener or ChangeNotifier in world.go outside comments.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Plan imprecision] Confirm and dispute by one account are one row, not two**
- **Found during:** Task 1 (TestCurrentVotesNewestWins)
- **Issue:** The plan's behaviour text says a confirm and a dispute by the same account are two rows because they are different kinds. In the real SQL both are `kind = content` (confirm and dispute are values), so DISTINCT ON (report_id, account_id, kind) keeps one row, the newer. Only a resolution row by the same account is a second row.
- **Fix:** The test and the twin follow the SQL; the parity scenario (A confirms then disputes) proves the real database agrees.
- **Files modified:** internal/demo/memquerier_test.go
- **Commit:** 65199c4, 599c38f

**2. [Rule 3 - Blocking] The package cannot import pgx but the services test errors.Is(err, pgx.ErrNoRows)**
- **Found during:** Task 1 design
- **Issue:** CastVote (ReportVoteContext), FindReplay (ReportByRequest), the insert conflict path and the focus read all treat pgx.ErrNoRows as "missing", while TestDemoPackageHasNoDatabase forbids importing pgx.
- **Fix:** An unexported error type whose `Is` matches the sentinel's message (and its wrapped `sql: no rows in result set`) while unwrapping the target. Tests assert `errors.Is(err, pgx.ErrNoRows)` against the real sentinel, and an unknown report through CastVote gives ErrReportNotFound. If pgx ever changes the message, those tests fail loudly.
- **Files modified:** internal/demo/memquerier.go
- **Commit:** 599c38f

**3. [Rule 2 - Missing detail] Session to account binding**
- **Issue:** The reporter's identity reaches the querier only through the session id (SubmitChecked passes the account only with a request id), so the self vote block and the history read need a session map.
- **Fix:** `memQuerier.BindSession` and `World.SessionFor(accountID)`. This is an addition to the plan's artifact list; plans 05-06 and 05-10 must call SessionFor when posting or voting as an account.

Plan acceptance greps on `MaxReportsPerWorld = 40` needed each constant on its own declaration (gofmt alignment would otherwise add spaces), so they are two separate `const` lines.

## Known Stubs

None. `InsertModerationCheck` is an intentional no-op (documented): the Demo services have no screener so no check row is ever requested.

## Threat Flags

None. The package adds no network endpoint, no file access and no table access. T-05-11 to T-05-16 are covered by TestDemoPackageHasNoDatabase, TestWorldCaps, TestStoreTTLAndLRU, TestValidVisitID, TestCastVoteRunsRealRules, the unset screener and notifier (grep checked) and TestDemoParityWithSQL.

## Commits

- 65199c4 test(05-03): add failing tests for the in memory querier and the world
- 599c38f feat(05-03): in memory querier and world behind the real report and vote services
- 7aacedd test(05-03): add tests for the world store and the no database guard
- ed771db feat(05-03): world store with random ids, ceilings, idle drop and eviction
- ad7daf6 test(05-03): differential test of the in memory world against the database

## Self-Check: PASSED

All seven files exist, all five commits exist on the branch, `p5_03` is dropped.
