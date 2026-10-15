---
phase: 04-robustness-real-world-resilience
plan: 05
subsystem: database
tags: [postgres, sqlc, goose, moderation, idempotency, flags]
requires: []
provides:
  - "reports.moderation_state, request_id, request_account_id, client_written_at"
  - "tables moderation_checks, report_flags, moderation_actions"
  - "18 new sqlc queries plus r.moderation_state on ReportVoteContext"
affects: [04-07, 04-11, 04-14, 04-17, 04-20, 04-23]
tech-stack:
  added: []
  patterns:
    - "state on the row from INSERT, extras through one batched read"
    - "MATERIALIZED CTE + FOR UPDATE SKIP LOCKED claim"
    - "Keep and Remove as one statement with two data modifying CTEs"
key-files:
  created:
    - internal/store/migrations/00007_add_moderation_and_idempotency.sql
    - internal/store/queries/moderation.sql
    - internal/store/sqlc/moderation.sql.go
    - internal/store/moderation_test.go
  modified:
    - internal/store/queries/reports.sql
    - internal/store/queries/votes.sql
    - internal/store/sqlc/models.go
    - internal/store/sqlc/reports.sql.go
    - internal/store/sqlc/votes.sql.go
    - internal/testutil/db.go
    - internal/store/migrations_test.go
key-decisions:
  - "Migration number taken: 00007 (highest on base was 00006). Another session may add 00007 for Phase 3.2; renumber at merge time."
  - "KeepReport and RemoveReport qualify the main UPDATE with reports.id because sqlc found the bare id ambiguous against the CTE columns."
  - "FinishModerationCheck takes verdict as a plain string (always known when a check finishes); scores and model stay nullable."
requirements-completed: [ROBUST-02, ROBUST-03, ROBUST-09]
duration: about 40 min
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 05: Moderation and idempotency store layer Summary

One migration, 18 new sqlc queries and 17 real Postgres tests that pin every store level guarantee the later Phase 4 plans build on: state on the row from the insert, an account scoped idempotent insert and replay, a single claimant worker claim, uncapped queue counts, and one statement terminal Keep and Remove.

## Step 0 results (all passed, nothing improvised)

- Highest migration on base: `00006_add_votes_account_kind_index.sql`. Migration taken: **00007** (`00007_add_moderation_and_idempotency.sql`). If a Phase 3.2 session also claims 00007, the merge must renumber one of them; the tests find the file by suffix and `goose.NumericComponent`, so no test names a number.
- `votes.sql` line 57: select list ended `r.latitude, r.longitude, r.created_at` (found).
- `reliability.sql` has `ReporterHistoryReports` (found); `reliability.sql.go` stayed byte identical after `make sqlc`.
- `migrations_test.go` already named `00005_add_reports_session_expires_index.sql` (found); `00006` assertions kept too.
- `const nearbyReportsSQLForExplain` at `internal/store/reports_test.go:29` (found).
- `TRUNCATE` literal in `internal/testutil/db.go` found; `seedReportForVotes` and `seedAccountForVotes` found in `votes_test.go`.
- All `sqlcgen.ReportVoteContextRow{...}` literals are keyed (handlers, service tests); `go vet ./...` passes with the new field.
- `sqlc version`: `v1.31.1`.
- `git status --porcelain internal/store/sqlc/` before staging: ` M models.go`, ` M reports.sql.go`, ` M votes.sql.go`, `?? moderation.sql.go` (exactly the four expected files).

## Commits

- `79ad742` feat(04-05): migration, Truncate list, migration tests
- `3239030` feat(04-05): queries and regenerated sqlc
- `c5e554b` test(04-05): store proofs (moderation_test.go)

## Generated Go the later plans read (package `sqlcgen`)

Functions (all `func (q *Queries) ...(ctx, ...)`):

- reports.sql.go: `InsertReportIdempotent(ctx, InsertReportIdempotentParams) (InsertReportIdempotentRow, error)`, `ReportByRequest(ctx, ReportByRequestParams{AccountID int64, RequestID string}) (ReportByRequestRow, error)`, `ReportExtrasForReports(ctx, ReportExtrasForReportsParams{ViewerAccountID int64, ReportIds []int64}) ([]ReportExtrasForReportsRow, error)`, `ReportForModeration(ctx, reportID int64) (ReportForModerationRow, error)`
- moderation.sql.go: `InsertModerationCheck(ctx, InsertModerationCheckParams) error`, `ClaimDueModerationChecks(ctx, ClaimDueModerationChecksParams{LeaseSeconds int32, BatchLimit int32}) ([]ClaimDueModerationChecksRow, error)`, `FinishModerationCheck(ctx, FinishModerationCheckParams{Verdict string, Scores []byte, Model *string, NeedsReview bool, ReportID int64}) error`, `RescheduleModerationCheck(ctx, RescheduleModerationCheckParams{NextAttemptAt time.Time, LastError *string, ReportID int64}) error`, `SetReportModerationState(ctx, SetReportModerationStateParams{ModerationState string, ReportID int64}) error`, `ReviewQueue(ctx) ([]ReviewQueueRow, error)`, `FlagQueue(ctx) ([]FlagQueueRow, error)`, `PendingQueue(ctx) ([]PendingQueueRow, error)`, `ModerationQueueCounts(ctx) (ModerationQueueCountsRow, error)`, `ModerationStatusForReport(ctx, reportID int64) (ModerationStatusForReportRow, error)`, `KeepReport(ctx, reportID int64) error`, `RemoveReport(ctx, reportID int64) error`, `InsertModerationAction(ctx, InsertModerationActionParams{ReportID, ModeratorAccountID int64, Action string}) error`, `InsertReportFlag(ctx, InsertReportFlagParams{ReportID, AccountID int64, Reason string}) (int64, error)`
- votes.sql.go: `ReportVoteContextRow.ModerationState string` (last field).

Types of the fields the outline asked about:

| Field | Go type |
|-------|---------|
| `ClientWrittenAt` (params and rows) | `pgtype.Timestamptz` |
| `Scores` (params and rows) | `[]byte` |
| `FlagCounts` | `[]byte` (JSON object reason to count) |
| `RequestID` (Insert params) | `*string` |
| `RequestAccountID` (Insert params) | `*int64` |
| `CheckStatus` | `string` in `ReviewQueueRow` and `PendingQueueRow` (inner join), `*string` in `FlagQueueRow` and `ReportForModerationRow` (left join) |
| `OpenFlagTotal` | `int64` |
| `NeedsReview` | `bool` |
| `Verdict`, `Model` (rows, InsertModerationCheck) | `*string` |
| `InsertModerationCheckParams.NextAttemptAt` | `time.Time`; `CheckedAt` is `pgtype.Timestamptz` |
| `ClaimDueModerationChecksRow.Attempts` | `int32` |
| Counts row | `ReviewCount`, `FlaggedCount`, `PendingCount` all `int64` |
| Status row | `InReview`, `InFlagged`, `InPending` all `bool` |

Insert params: `ShelterCapacityStatus *string`, `ShelterHeadcount *int32`, `ModerationState string`.

## Truncate

`testutil.Truncate` gained `moderation_checks, report_flags, moderation_actions`. No earlier table was missing from the list.

## Verification results

- `TestMigrationsEmbedded`, `TestMigrationsUpDown`, `TestMigrationsTruncateListsEveryTable` against `p4_0405`: 3 PASS, 0 SKIP.
- The 13 test functions that the plan's `-race` command selects (14 `--- PASS` lines because `TestInsertReportIdempotent` matches two) against `p4_0405` with `-race -p 1 -count=1`: 14 PASS, 0 SKIP, no race reported. `TestNearbyReportsQueryUnchanged` and `TestModerationRowsCarryNoSessionID` also pass without a database.
- `moderation_test.go` holds 14 tests (`grep -c '^func Test'`).
- Wave sampling on `p4_0405`: `go test ./internal/... ./web/ -count=1 -p 1` all `ok`, nothing skipped for lack of a database.
- Without `DATABASE_URL`: `go test ./... -count=1` ok, `go vet ./...` ok, `sqlc diff` exit 0, `gofmt -l` empty.
- `p4_0405` was dropped afterwards. No other database was touched.

## Deviations from Plan

None in behavior. Three small implementation notes:

1. **[Rule 3 - Blocking] sqlc "column reference id is ambiguous"** in `KeepReport` and `RemoveReport` (the main `UPDATE reports` next to the data modifying CTEs). Fixed by writing `reports.id` and `reports.moderation_state` in their WHERE clauses. Files: `internal/store/queries/moderation.sql`. Fixed before the Task 2 commit.
2. `TestMigrationsEmbedded` assertions for the new file were added inside that test (as the plan says), and the helpers `phase4MigrationName` and `upSectionTables` sit in `migrations_test.go` (package `store`).
3. The plan text calls the migration test count "three" and the store test count "fourteen"; the final counts match (3 and 14).

## Known Stubs

None.

## Threat Flags

None. No endpoint, auth path or file access was added; the new tables hold no report text and no new row type exposes a session, email or account field (`TestModerationRowsCarryNoSessionID`).

## Notes for the orchestrator

- If Phase 3.2 also adds `00007_*`, renumber one file at merge. The only name based coupling is the suffix `_add_moderation_and_idempotency.sql`.
- No browser harness checks change; nothing user visible changed.
- Downstream plans must pass an empty (not nil) `ReportIds` only when they want zero rows; a nil slice also yields zero rows through `ANY(NULL)`, so both are safe.

## Self-Check: PASSED

Files verified present: migration 00007, `moderation.sql`, `moderation.sql.go`, `moderation_test.go`. Commits `79ad742`, `3239030`, `c5e554b` exist on `worktree-agent-a76de144311c0b780`. All plan verification commands ran and passed.
