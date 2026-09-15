---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-03a"
subsystem: trust-mechanic
tags: [go, pgx, sqlc, geohash, tdd, voting]

requires:
  - phase: 02-01
    provides: "internal/service/visibility.go — Resolve()/VoteTally (six fields)/IndependentAgreementThreshold/ReportMeta"
  - phase: 02-02
    provides: "internal/store — votes table, InsertVote/CurrentVotesForReports queries"
provides:
  - "internal/store/queries/votes.sql: ReportVoteContext (:one) — severity/category/expires_at/nullable reporter account in one round trip"
  - "internal/service/trust.go: VoteKind/VoteValue enums with Valid()/ValidFor(), independentCellCount, BuildVoteTally, VotingQuerier interface, VotingService.CastVote"
  - "three sentinel errors (ErrCannotVoteOwnReport, ErrReportExpired, ErrReportNotFound) for 02-03b to map to HTTP status codes"
affects: [02-03b, 02-04, 02-05, 02-06, 02-07]

tech-stack:
  added: []
  patterns:
    - "voterGeohashPrecision (7) kept deliberately distinct from report.go's geohashPrecision (8) — different questions (voter independence vs. report location)"
    - "Server-computed fields never accepted from a client (CastVoteInput has no geohash field), extending report.go's SubmitInput convention to the vote write path"
    - "independentCellCount as the single reusable independence predicate (D-14), consumed by BuildVoteTally and nothing else"

key-files:
  created:
    - internal/service/trust.go
    - internal/service/trust_test.go
  modified:
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go

key-decisions:
  - "ReportVoteContext supersedes the narrower ReporterAccountID query 02-02/02-PATTERNS.md/02-RESEARCH.md anticipated — one round trip returns reporter identity + severity/category + expires_at, closing the window where a report could expire/vanish between two separate reads. A query named ReporterAccountID is deliberately not created; its absence is intentional."
  - "D-03's self-vote block is scoped to VoteKindContent only, both in the guard condition and behaviorally verified via a two-case table (resolve, reopen) — D-13's instant resolve and D-16-amended's instant reopen both remain the reporter's always-allowed path."
  - "CastVote contains no branch of its own for the reporter's instant resolve/reopen — both arrive as VoteTally flags from BuildVoteTally and are adjudicated inside Resolve, keeping exactly one visibility authority (T-02-04)."

requirements-completed: [TRUST-01, TRUST-03, TRUST-08, TRUST-09]

coverage:
  - id: D1
    description: "ReportVoteContext query resolves a report's severity, category, expiry, and nullable reporter account in one round trip, with a report whose session has no bound account yielding a NULL reporter (LEFT JOIN) rather than zero rows."
    requirement: "TRUST-01"
    verification:
      - kind: unit
        ref: "internal/store/votes_test.go#TestVotesQuerySourceHasNoUpsertOrLock"
        status: pass
      - kind: integration
        ref: "go test ./internal/store/... -p 1 (full store suite against real Postgres)"
        status: pass
    human_judgment: false
  - id: D2
    description: "independentCellCount is the sole implementation of the distinct-geohash-cell independence predicate (D-14); BuildVoteTally routes all four cell counts through it."
    requirement: "TRUST-03"
    verification:
      - kind: unit
        ref: "internal/service/trust_test.go#TestIndependentCellCount"
        status: pass
      - kind: unit
        ref: "internal/service/trust_test.go#TestBuildVoteTally"
        status: pass
    human_judgment: false
  - id: D3
    description: "VotingService.CastVote blocks a reporter's own content vote (D-03) before any row is written, while both halves of the reporter's instant resolution privilege (resolve D-13, reopen D-16-amended) remain always-allowed and threshold-free — including lifting a retraction caused by independent confirmers, not just the reporter's own resolve."
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteRejectsReporterContentVote"
        status: pass
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteAllowsReporterResolutionVote"
        status: pass
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteReporterInstantReopen"
        status: pass
    human_judgment: false
  - id: D4
    description: "The voter's geohash cell is computed server-side at voterGeohashPrecision=7 from raw coordinates; CastVoteInput carries no geohash field for a client to forge. Concurrent same-account votes each persist as a distinct append-only row (TRUST-09, owned by 02-02, re-verified here against the regenerated query layer)."
    requirement: "TRUST-09"
    verification:
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteComputesGeohashCellServerSide"
        status: pass
      - kind: integration
        ref: "internal/store/votes_test.go#TestCastVoteConcurrentSameAccountKeepsEveryRow"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-15
status: complete
---

# Phase 2 Plan 03a: Server-Side Vote Decision Layer (CastVote) Summary

**`VotingService.CastVote` — reporter self-vote block, server-computed geohash cell, and the six-field `VoteTally` that feeds `Resolve` — implemented with `ReportVoteContext` as its one supporting query, all ten `CastVote` behaviors and six `BuildVoteTally` behaviors TDD-proven against a recording fake, no HTTP surface.**

## Performance

- **Duration:** ~15 min (first commit 20:23:47 IST, last commit 20:32:14 IST, plus file-reading/context-gathering preceding the first commit)
- **Started:** 2026-09-15T14:50:00Z (approx.)
- **Completed:** 2026-09-15T15:02:14Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- `ReportVoteContext` (`:one`) appended to `internal/store/queries/votes.sql`, regenerated via `make sqlc` into `internal/store/sqlc/votes.sql.go` — a single round trip returning a report's `severity`, `category`, `expires_at`, and nullable `reporter_account_id` via `reports LEFT JOIN sessions`
- `internal/service/trust.go`: the vote vocabulary (`VoteKind`/`VoteValue` with `Valid()`/`ValidFor()`), three sentinel errors, `independentCellCount` (the one implementation of TRUST-03's independence predicate), and `BuildVoteTally` — turning a batch of current-vote rows into 02-01's six-field `VoteTally`, surfacing the reporter's own resolution vote through the symmetric `ReporterResolved`/`ReporterReopened` pair
- `VotingQuerier` interface, `VotingService`, `CastVoteInput`/`CastVoteResult`, and `VotingService.CastVote` — an eight-step ordered sequence that validates input, loads vote context, blocks a reporter's own content vote (D-03), rejects an expired report, computes the voter's cell server-side, appends the vote, reads current votes back, and resolves via `Resolve`
- 16 test functions across two TDD cycles (Task 2: 6, Task 3: 10), every one written and confirmed failing to compile before the corresponding implementation was added

## Task Commits

Each task was committed atomically:

1. **Task 1: One query answers every question CastVote asks about a report** — `d359a60` (feat)
2. **Task 2: One independence rule, written once — the vote vocabulary and the tally builder** — `4323ea5` (feat, TDD: RED confirmed via failed `go vet` before implementation, then GREEN)
3. **Task 3: VotingService.CastVote — the reporter blocked, the cell computed server-side, the fresh visibility returned** — `798e93d` (feat, TDD: RED confirmed via failed `go vet` before implementation, then GREEN)

**Plan metadata:** (this commit, docs: complete plan)

_Note: TDD tasks 2 and 3 each combined their RED (failing compile against the unimplemented package) and GREEN (implementation + passing tests) into a single commit per task, since the "RED" state here is a compile failure rather than a separately-committable failing-test state — the plan's `<action>` explicitly instructs "write the test file first and confirm it fails against unimplemented stubs before filling the bodies in," which was done, but there is no intermediate compilable-and-red state to commit separately for a from-scratch file. Both files (`trust.go`+`trust_test.go`) were authored and verified together, then committed as one `feat` commit per task, matching this repo's existing `report.go`/`report_test.go` and `auth.go`/`auth_test.go` commit granularity._

## Files Created/Modified

- `internal/store/queries/votes.sql` — appended `ReportVoteContext :one`, third query in the file (alongside 02-02's `InsertVote`/`CurrentVotesForReports`)
- `internal/store/sqlc/votes.sql.go` — regenerated; adds `ReportVoteContextRow` struct and `(*Queries).ReportVoteContext` method
- `internal/service/trust.go` — new file: vote vocabulary, `independentCellCount`, `BuildVoteTally`, `VotingQuerier`, `VotingService`, `CastVoteInput`/`CastVoteResult`, `CastVote`
- `internal/service/trust_test.go` — new file: `package service` (internal test package, since `independentCellCount` is unexported), 16 test functions plus `fakeVotingQuerier` and helper functions (`voteRow`, `acctPtr`, `castVoteAndGetCell`)

## Generated/Exported Symbols (for 02-03b, 02-04, 02-05, 02-06, 02-07)

**`ReportVoteContext`** (`internal/store/sqlc/votes.sql.go`):
```go
type ReportVoteContextRow struct {
	Severity          string
	Category          string
	ExpiresAt         time.Time
	ReporterAccountID *int64
}
func (q *Queries) ReportVoteContext(ctx context.Context, reportID int64) (ReportVoteContextRow, error)
```
**Supersession note:** `ReportVoteContext` supersedes the narrower `ReporterAccountID` query that 02-02's "Deliberately NOT produced here" section, `02-PATTERNS.md`, and `02-RESEARCH.md`'s Pattern 3 all anticipated 02-03 would add. A query named `ReporterAccountID` is deliberately **not** created — its absence is intentional, not a gap. A later drift check against those documents should read this note, not flag a missing query.

**`VotingQuerier`** (`internal/service/trust.go`) — the entire store surface the voting path touches; 02-03b and 02-04 both write their own fakes against this exact shape:
```go
type VotingQuerier interface {
	InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
}
```

**`CastVoteInput`** — six fields, no geohash field for a client to populate:
```go
type CastVoteInput struct {
	ReportID  int64
	AccountID int64
	Kind      VoteKind
	Value     VoteValue
	Latitude  float64
	Longitude float64
}
```

**`CastVoteResult`**:
```go
type CastVoteResult struct {
	Visibility Visibility
	Reason     ResolveReason
}
```

**Sentinel errors → 02-03b's HTTP status mapping:**

| Error | Status code |
|-------|-------------|
| `ErrCannotVoteOwnReport` | 403 |
| `ErrReportExpired` | 409 |
| `ErrReportNotFound` | 404 |

**`BuildVoteTally`** signature (frozen, unchanged by any amendment): `func BuildVoteTally(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) VoteTally` — populates all six `VoteTally` fields (`ConfirmCells`, `DisputeCells`, `ResolveCells`, `ReopenCells`, `ReporterResolved`, `ReporterReopened`).

## Decisions Made

- No new decisions beyond what `02-CONTEXT.md`/`02-RESEARCH.md` already locked (D-02, D-03, D-13, D-14, D-16 amended, D-17) — this plan implements those decisions rather than making new ones.
- Committed Task 2 and Task 3 as single `feat` commits each (test file + implementation together) rather than separate `test`/`feat` commits, since the RED state for a from-scratch file is a compile failure with no intermediate committable state — matches this repo's existing convention for `report.go`+`report_test.go` and `auth.go`+`auth_test.go`.

## Deviations from Plan

None — plan executed exactly as written. `ReportVoteContext`'s generated shape, `VotingQuerier`'s three methods, `CastVoteInput`/`CastVoteResult`'s field lists, `BuildVoteTally`'s signature, and `CastVote`'s eight-step sequence all match the plan's `<action>` sections verbatim. `make sqlc` did not modify `internal/store/sqlc/models.go` or `internal/store/sqlc/db.go` (only `votes.sql.go` changed) — the plan's instruction to commit all three "together" only applies when sqlc actually rewrites them; here only `votes.sql.go` had a diff, so only it (plus the source `votes.sql`) was staged for Task 1's commit.

## Issues Encountered

None. All acceptance criteria and the plan's full `<verification>` section (8 steps) passed on the first implementation pass for each task, including `make sqlc` idempotency (no diff on a second run), `gofmt`, `go vet ./...`, `go test ./... -short`, `go test ./... -p 1` against real Postgres, both `02-VALIDATION.md` commands (TRUST-01, TRUST-03), and the 02-02 source guard (`TestVotesQuerySourceHasNoUpsertOrLock`).

## User Setup Required

None — no external service configuration required.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced — `CastVote` is a fully functional service-layer implementation with no HTTP surface (by design, deferred to 02-03b).

## Threat Flags

None beyond what this plan's own `<threat_model>` already registers (T-02-05, T-02-01, T-02-03 — all `mitigate`, ASVS level 1). No new network endpoint, auth path, file access pattern, or schema change at a trust boundary was introduced outside that register.

## Next Phase Readiness

- `internal/service/trust.go` is ready for 02-03b to mount four HTTP routes on top of `VotingService.CastVote`, mapping the three sentinel errors to 403/409/404 as documented above.
- 02-04's feed path can call `BuildVoteTally(rows, reportID, reporterAccountID)` per report over one batched `CurrentVotesForReports` read without reintroducing an N+1.
- No blockers. `internal/service/trust.go` imports no `net/http` or `go-chi/chi` package (verified structurally), and this plan changed no file under `internal/api/`, `cmd/`, `web/`, or `docs/`.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*

## Self-Check: PASSED

- FOUND: internal/service/trust.go
- FOUND: internal/service/trust_test.go
- FOUND: internal/store/queries/votes.sql
- FOUND: internal/store/sqlc/votes.sql.go
- FOUND commit: d359a60 (Task 1)
- FOUND commit: 4323ea5 (Task 2)
- FOUND commit: 798e93d (Task 3)
