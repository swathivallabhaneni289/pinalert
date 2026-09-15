---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-02"
subsystem: database
tags: [postgres, sqlc, goose, concurrency, votes, trust-mechanic]

# Dependency graph
requires:
  - phase: 01-foundation-report-and-map
    provides: reports/accounts/sessions schema, sqlc/pgx/goose store-layer conventions
provides:
  - Append-only `votes` table (migration 00004) with no unique key beyond its own primary key
  - `InsertVote :exec` and `CurrentVotesForReports :many` sqlc-generated Go accessors
  - `sqlcgen.CurrentVotesForReportsRow` and `sqlcgen.InsertVoteParams` — the exact types 02-03's
    VotingQuerier interface is written against
  - Concurrency proof that N simultaneous vote casts never silently drop a row (TRUST-09)
  - Structural + source-level guards against the vote log ever gaining a contended row (T-02-02)
affects: [02-03a, 02-03b, 02-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Append-only vote log with DISTINCT ON + ORDER BY ... created_at DESC, id DESC as the
      'current vote' read, instead of a cached counter or upsert-keyed-on-(report,account,kind)
      row — no shared mutable row exists to serialise concurrent writers on"
    - "Source-level drift guard (forbiddenClauseIn) that strips SQL comments before scanning for
      forbidden clauses, self-proven against a hardcoded known-bad counter-example so the guard's
      ability to fail is demonstrated in-process, not trusted on the honor system"

key-files:
  created:
    - internal/store/migrations/00004_create_votes.sql
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go
    - internal/store/votes_test.go
  modified:
    - internal/store/sqlc/models.go
    - internal/testutil/db.go

key-decisions:
  - "No UNIQUE or PRIMARY KEY over (report_id, account_id, kind) — the vote log is a pure append,
    concurrency safety for TRUST-09 comes from Postgres handling concurrent plain INSERTs into an
    unconstrained table, not from a lock or upsert-conflict clause"
  - "id intentionally omitted from CurrentVotesForReports' selected columns so sqlc emits a
    dedicated CurrentVotesForReportsRow struct rather than collapsing the result onto the Vote
    model — verified empirically against sqlc v1.31.1"
  - "Schema and queries written before the test file (Task 1 before Task 2) — a deliberate,
    documented ordering, not a TDD inversion: a test referencing sqlcgen.InsertVoteParams before
    sqlc generate has emitted it would not compile, and RED is preserved instead via Task 3's
    negative-control guard test"

patterns-established:
  - "Pattern 1 (append-only votes, no cached counter) from 02-RESEARCH.md, now implemented exactly
    as pinned"

requirements-completed: [TRUST-01, TRUST-09]

coverage:
  - id: D1
    description: "votes table exists as an append-only log with no unique key beyond its own primary key, indexed for the current-vote read"
    requirement: "TRUST-09"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestVotesHaveNoUniqueKeyBeyondPrimaryKey"
        status: pass
      - kind: integration
        ref: "internal/store/votes_test.go#TestVotesQuerySourceHasNoUpsertOrLock"
        status: pass
    human_judgment: false
  - id: D2
    description: "A vote is durably appended and reads back through CurrentVotesForReports with report_id, account_id, kind, value, geohash_cell intact"
    requirement: "TRUST-01"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestCastVoteConcurrent"
        status: pass
    human_judgment: false
  - id: D3
    description: "N concurrent casts from N distinct accounts on one report produce exactly N rows with no dropped write"
    requirement: "TRUST-09"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestCastVoteConcurrent"
        status: pass
    human_judgment: false
  - id: D4
    description: "N concurrent casts from one account on one report/kind produce N raw rows and exactly 1 current row matching the MAX(id) row — the property distinguishing this design from the rejected upsert design"
    requirement: "TRUST-09"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestCastVoteConcurrentSameAccountKeepsEveryRow"
        status: pass
    human_judgment: false
  - id: D5
    description: "Changing a vote appends a row (D-02); the older row survives and the current-vote read returns the newer value, with the id DESC tiebreaker exercised by an identical-created_at case"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestCurrentVoteIsLatestOnAccountChange"
        status: pass
    human_judgment: false
  - id: D6
    description: "Content and resolution votes from one account on one report coexist without disturbing each other's current value (D-02, D-16)"
    verification:
      - kind: integration
        ref: "internal/store/votes_test.go#TestVotesArePerReportPerAccountKind"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-09-15
status: complete
---

# Phase 2 Plan 2: Append-Only Vote Log Summary

**Append-only `votes` table with a `DISTINCT ON` current-vote read, proven under real concurrency
against Postgres — no cached counter, no upsert, no row lock, structurally immune to the
lost-update class of bug (T-02-02).**

## Performance

- **Duration:** 35 min
- **Started:** 2026-09-15T14:10:00Z (approx.)
- **Completed:** 2026-09-15T14:45:25Z
- **Tasks:** 3
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- Migration `00004_create_votes.sql`: `votes` table with exactly one uniqueness-bearing
  constraint (its own `id` primary key) — no `UNIQUE (report_id, account_id, kind)`, so there is
  no contended row for concurrent writers to race on.
- `internal/store/queries/votes.sql`: `InsertVote :exec` (bare append, no conflict clause) and
  `CurrentVotesForReports :many` (`DISTINCT ON (report_id, account_id, kind) ... ORDER BY
  report_id, account_id, kind, created_at DESC, id DESC`), batched over an array of report ids.
- sqlc-regenerated Go layer: `sqlcgen.CurrentVotesForReportsRow` (a dedicated struct, `id`
  deliberately omitted from the select so sqlc does not collapse the result onto the `Vote`
  model) and `sqlcgen.InsertVoteParams`.
- `internal/store/votes_test.go`: 6 tests proving TRUST-09's concurrency guarantee, D-02's
  append-not-overwrite vote-change semantics (including the `id DESC` tiebreaker under an
  identical-`created_at` case), D-02/D-16's content-vs-resolution independence, and two guard
  tests (database-level + source-level) that fail if the contended-row design is ever
  reintroduced.
- `internal/testutil/db.go`: `Truncate` now names `votes` explicitly in its table list.

## Task Commits

Each task was committed atomically:

1. **Task 1: The append-only vote log exists, is indexed for the current-vote read, and has typed
   Go accessors** - `e5e1f3e` (feat)
2. **Task 2: Prove the log never loses a vote under concurrency and always reads back the latest
   one (TRUST-09, D-02)** - `4449c8b` (test)
3. **Task 3: Lock the structural absence of the race — no unique key, no upsert, no row lock
   (T-02-02)** - `cfe7779` (test)

## Files Created/Modified

- `internal/store/migrations/00004_create_votes.sql` - append-only `votes` table + covering index
- `internal/store/queries/votes.sql` - `InsertVote`, `CurrentVotesForReports`
- `internal/store/sqlc/votes.sql.go` - sqlc-generated Go accessors (new file)
- `internal/store/sqlc/models.go` - gained the `Vote` struct
- `internal/store/votes_test.go` - 6 tests (4 concurrency/behavior proofs + 2 structural guards)
- `internal/testutil/db.go` - `Truncate` now names `votes` explicitly

## Final Generated Signatures (for 02-03's reference, per this plan's `<output>` spec)

```go
type CurrentVotesForReportsRow struct {
	ReportID    int64
	AccountID   int64
	Kind        string
	Value       string
	GeohashCell string
	CreatedAt   time.Time
}

func (q *Queries) CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]CurrentVotesForReportsRow, error)

type InsertVoteParams struct {
	ReportID    int64
	AccountID   int64
	Kind        string
	Value       string
	GeohashCell string
}

func (q *Queries) InsertVote(ctx context.Context, arg InsertVoteParams) error
```

## Observed Row/Current-Row Counts (`TestCastVoteConcurrentSameAccountKeepsEveryRow`)

8 goroutines cast alternating confirm/dispute votes from one account on one report/kind:
- Raw row count for `(report_id, account_id)`: **8** — every concurrent cast persisted as its own
  row (the vote-change history Phase 3 needs survives).
- `CurrentVotesForReports` row count: **1** — the read collapses to exactly one current vote.
- That current row's `Value` was asserted equal to an independently-read
  `SELECT value FROM votes ... ORDER BY id DESC LIMIT 1` value (not a hardcoded literal), proving
  `DISTINCT ON` returns the genuinely latest row regardless of which goroutine won the race.

## Decisions Made

- No cached `confirm_count`/`dispute_count`/`status` column added to `reports` — visibility is
  recomputed live per D-08, out of scope for this store-layer plan.
- `geohash_cell` on `votes` holds the voter's own location at precision 7, written by 02-03's
  service layer — never client-supplied, never client-visible. This plan only puts the column in
  reach; the non-serialization control belongs to 02-04's response-shape work (recorded in the
  plan's threat model as a non-threat note for 02-04).
- Local verification used a pre-existing local Postgres 16 instance (`pinalert_test` database via
  `brew services`) since Docker was unavailable in this environment — no schema or behavior
  difference from CI's `postgres:16` service container.

## Deviations from Plan

None - plan executed exactly as written. The one apparent deviation — writing the schema/queries
(Task 1) before the test file (Task 2) — is explicitly specified by the plan itself in its
"Sequencing note (schema before tests, deliberately)" section, not an executor-introduced
deviation.

## Issues Encountered

None. `make sqlc` succeeded on the first run with no `id`-column ambiguity, `go build`/`go vet`
were clean throughout, and all four Task 2 tests plus both Task 3 guard tests passed on first
execution against a real Postgres instance.

## User Setup Required

None - no external service configuration required. Local verification used an existing local
Postgres 16 database; CI already provisions its own `postgres:16` service container per
`.github/workflows/ci.yml`.

## Next Phase Readiness

- `sqlcgen.CurrentVotesForReportsRow`, `sqlcgen.InsertVoteParams`, and the two `Queries` methods
  exist with the exact shapes `02-PLAN-OUTLINE.md` pins for 02-03's `VotingQuerier` interface —
  02-03a/02-03b can build `VotingService`/`CastVote` directly against them with no re-verification
  of the generated file needed (signatures recorded above).
- `ReporterAccountID` (the `reports` → `sessions` → `accounts` join backing D-03's reporter-
  self-vote block) is deliberately NOT added here — 02-03's own task, appended to
  `internal/store/queries/votes.sql` in a later wave.
- No blockers. `files_modified` for this plan has zero overlap with sibling wave-1 plan 02-01
  (`internal/service/visibility.go`), confirmed by `git log` — both wave-1 plans landed cleanly in
  parallel worktrees.

## Self-Check: PASSED

- `internal/store/migrations/00004_create_votes.sql` — FOUND
- `internal/store/queries/votes.sql` — FOUND
- `internal/store/sqlc/votes.sql.go` — FOUND
- `internal/store/votes_test.go` — FOUND
- Commit `e5e1f3e` — FOUND in `git log --oneline --all`
- Commit `4449c8b` — FOUND in `git log --oneline --all`
- Commit `cfe7779` — FOUND in `git log --oneline --all`
- All plan `<acceptance_criteria>` re-run and passing (see Task Commits + verification output
  above): `make sqlc` exits 0 idempotently, `go build`/`go vet` clean, `go test ./... -short`
  green, `go test ./... -v -p 1` (DATABASE_URL set) green with all 6 new tests PASS and none SKIP.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*
