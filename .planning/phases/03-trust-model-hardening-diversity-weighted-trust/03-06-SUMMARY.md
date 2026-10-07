---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 06
subsystem: store
tags: [sqlc, postgres, reliability, history, index, lateral, explain, go]
requires:
  - phase: 03-01
    provides: "regenerated votes.sql.go baseline and the testVoteContext fixtures; no code from it is used here"
provides:
  - "sqlcgen.Queries.ReporterHistoryReports: bounded per account history read keyed by sessions.account_id"
  - "idx_reports_session_expires (migration 00005)"
  - "testutil.SeedReporterAccount, SeedHistoryReport, SeedReporterHistory and the HistorySeed size constants"
affects: [03-10, 03-12]
tech-stack:
  added: []
  patterns:
    - "per session LATERAL with LIMIT, then a window function per account, so the planner is forced onto the session index"
    - "three way drift guard: query source, hand copied EXPLAIN const and sqlc generated const compared after whitespace normalisation"
    - "EXPLAIN test asserts the index by name, not a generic index scan"
key-files:
  created:
    - internal/store/queries/reliability.sql
    - internal/store/sqlc/reliability.sql.go
    - internal/store/reliability_test.go
    - internal/testutil/history.go
    - internal/store/migrations/00005_add_reports_session_expires_index.sql
  modified:
    - internal/store/migrations_test.go
key-decisions:
  - "The cap counts the latest 30 expired reports inside the window, not 30 judged reports (RESOLVED Q4). Neutral classification of critical and rescue needed reports that expired unconfirmed stays in Go. Plan 03-10 records this in docs/TRUST-MODEL.md."
requirements-completed: [TRUST-07]
duration: about 1 hour
completed: 2026-10-08
status: complete
---

# Phase 3 Plan 06: Source reliable history read Summary

One generated query, `ReporterHistoryReports`, returns each requested reporter account's latest expired reports across all of its sessions (at most RowCap per account, only inside the window), and migration 00005 makes it index backed, proven by an EXPLAIN that names `idx_reports_session_expires` at 10 and 200 accounts over 48,000 reports.

## What was built

- **Task 1** (`c7abc65`): `queries/reliability.sql` with the per session LATERAL plus per account `row_number()` shape from RESEARCH, keyed through `sessions.account_id` and carrying no session id, voter cell, text or coordinates. `make sqlc` produced `sqlc/reliability.sql.go`. `reliability_test.go` holds `reporterHistorySQLForExplain`, the drift guard (V-26, with two negative controls), `TestReporterHistoryBounds` (V-25) and `TestReporterHistoryUsesAccountNotSession` (V-28 store part). `testutil/history.go` holds `SeedReporterAccount` and `SeedHistoryReport`.
- **Task 2** (`8611c2c`): migration `00005_add_reports_session_expires_index.sql`, `TestMigrationsEmbedded` extended (V-27, no database), `TestSessionExpiresIndexExists` (V-27), `TestReporterHistoryUsesSessionIndex` (V-24) and `SeedReporterHistory` with the three size constants. RED was observed first: `TestMigrationsEmbedded` failed with "expected migrations/00005... to be embedded" before the migration existed.

## Generated names and types (verbatim, for plans 03-10 and 03-12)

```go
type ReporterHistoryReportsParams struct {
	Now         time.Time
	WindowStart time.Time
	RowCap      int32
	AccountIds  []int64
}

type ReporterHistoryReportsRow struct {
	AccountID int64
	ReportID  int64
	Severity  string
	Category  string
	ExpiresAt time.Time
}

func (q *Queries) ReporterHistoryReports(ctx context.Context, arg ReporterHistoryReportsParams) ([]ReporterHistoryReportsRow, error)
```

## EXPLAIN evidence (V-24)

Plan lines naming the index, from `TestReporterHistoryUsesSessionIndex -v` on `p3_0306`:

- 10 accounts: `->  Index Scan using idx_reports_session_expires on reports r  (cost=0.41..226.00 rows=60 width=33) (actual time=0.104..0.151 rows=30 loops=30)`
- 200 accounts: `->  Index Scan using idx_reports_session_expires on reports r  (cost=0.41..226.00 rows=60 width=33) (actual time=0.043..0.057 rows=30 loops=600)`

Neither plan contains `Seq Scan on reports`. The real query over all 200 accounts returned exactly 6,000 rows, 30 per account, all inside the window.

## Verification

- `gofmt -l internal/store internal/testutil`: no output. `sqlc diff`: exit 0. `go vet ./...`: clean.
- `make sqlc` after the migration changed no generated file. Across both commits `internal/store/sqlc` gained only `reliability.sql.go`; `votes.sql.go`, `reports.sql.go`, `models.go` and `db.go` are unchanged against HEAD.
- Migrations directory holds 5 `.sql` files. 00005 non comment lines hold the CREATE INDEX statement once and the DROP INDEX statement once, with one `-- +goose Up` and one `-- +goose Down`.
- `go test ./... -short -count=1` with `DATABASE_URL` unset (checked with echo): every package `ok`.
- Full suite on a fresh `p3_0306` with `go test ./... -count=1 -p 1 -v`, filtered to result lines: every package `ok` (api, api/handlers, auth, geocode, mailer, ratelimit, service, session, store 7.411s, testutil, web), zero `--- FAIL`, zero `--- SKIP`. Store and testutil alone with `-v`: all tests PASS, including `TestReporterHistoryBounds`, `TestReporterHistoryUsesAccountNotSession`, `TestSessionExpiresIndexExists`, `TestReporterHistoryUsesSessionIndex`, `TestNearbyReportsUsesIndex`, `TestVotesQuerySourceHasNoUpsertOrLock`, `TestReportsByAccountSpansMultipleSessions`, `TestNewTestDB`.
- `p3_0306` was created fresh and dropped each time; `psql -l` lists no `p3_0306` afterwards.
- No dash characters (U+2014, U+2013) in any file this plan touched. No server was started. Ports 8080, 8090 and databases `pinalert_test`, `pinalert` were never touched. No image files were created.

## For plan 03-10 (docs)

The cap counts the latest 30 expired reports inside the 90 day window, not 30 judged reports (RESOLVED Q4). The two readings differ only for a reporter with many critical or rescue needed reports that expired unconfirmed. Record this in docs/TRUST-MODEL.md.

## For the owner

The owner's own dev database needs `make migrate` after this phase merges, run by the owner, to get `idx_reports_session_expires`. The queries work without it, only slower. The executor never ran `make migrate` and never touched that database.

## Deviations from Plan

1. **[Tooling] Verify commands split.** The sandbox refused the plan's single compound verify one-liners and the `env -u DATABASE_URL` form. I first ran `echo "DATABASE_URL=${DATABASE_URL:-unset}"`, which printed `unset`, then ran the same checks as separate plain commands (gofmt, `sqlc diff`, `go vet`, grep and count checks, `go test -short`, and create, test, drop of `p3_0306` as three commands). The same assertions were checked.
2. **[Scope, minor] EXPLAIN plan lines logged.** `TestReporterHistoryUsesSessionIndex` also calls `t.Logf` with each plan line that names the index, so the SUMMARY can quote them. No assertion changed.
3. **[Tooling] Seeder added by Edit.** A heredoc append to `history.go` was refused, so `SeedReporterHistory` was added with Edit. Content is as planned.

No Rule 1 to 3 fixes were needed. No auth gates.

## Known Stubs

None.

## Threat Flags

None. T-03-05 and T-03-10 are mitigated and tested as the plan's threat register describes.

## Self-Check: PASSED

- FOUND: internal/store/queries/reliability.sql, internal/store/sqlc/reliability.sql.go, internal/store/reliability_test.go, internal/testutil/history.go, internal/store/migrations/00005_add_reports_session_expires_index.sql
- FOUND commits: c7abc65, 8611c2c
