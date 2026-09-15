---
phase: 2
plan: "02-03a"
type: execute
wave: 2
depends_on: ["02-01", "02-02"]
files_modified:
  - internal/store/queries/votes.sql
  - internal/store/sqlc/votes.sql.go
  - internal/store/sqlc/models.go
  - internal/store/sqlc/db.go
  - internal/service/trust.go
  - internal/service/trust_test.go
autonomous: true
requirements: [TRUST-01, TRUST-03, TRUST-08, TRUST-09]

must_haves:
  truths:
    - "A content vote (confirm/dispute) whose caller is the report's own reporter is rejected with ErrCannotVoteOwnReport before any row is written — D-03 is enforced inside VotingService.CastVote, never only by a hidden client button (T-02-05)."
    - "A resolution vote (resolve/reopen) from that same reporter is NOT rejected: the block is scoped to VoteKindContent alone, because D-13 grants the reporter an instant resolve on their own report (TRUST-08)."
    - "The voter's geohash_cell is computed server-side as geohash.EncodeWithPrecision(latitude, longitude, voterGeohashPrecision) with voterGeohashPrecision = 7, from the raw coordinates D-17's GPS capture supplies — CastVoteInput carries no geohash field at all, so there is nothing for a client to forge."
    - "A vote on a report whose expires_at is not after the server's own now is rejected with ErrReportExpired and writes no row; a vote on a report id that does not exist is rejected with ErrReportNotFound and writes no row."
    - "Two confirm votes from two DISTINCT accounts standing in the SAME geohash cell produce ConfirmCells == 1, so the report stays Provisional — independence is measured in distinct cells, never in raw vote count (TRUST-03)."
    - "independentCellCount is the only implementation of 'independent agreement' in the tree: BuildVoteTally routes all four cell counts through it and no gate re-derives distinctness itself (D-14)."
    - "BuildVoteTally surfaces the reporter's own current resolution vote as ReporterResolved and excludes that account from ResolveCells/ReopenCells, exactly as 02-01's VoteTally doc comment requires."
    - "CastVote returns the visibility service.Resolve computes over the tally read back AFTER the insert, so the caller receives the post-vote state in the same response (D-04) and never re-derives it locally (T-02-04)."
  artifacts:
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go
    - internal/service/trust.go
    - internal/service/trust_test.go
  key_links:
    - "VotingQuerier's three methods — InsertVote, CurrentVotesForReports, ReportVoteContext — are the entire store surface VotingService touches; 02-04 writes its own fake against this exact interface."
    - "BuildVoteTally(rows, reportID, reporterAccountID) filters rows by reportID because 02-04 passes a multi-report batch from ONE CurrentVotesForReports call — the reportID parameter is what keeps the feed path from becoming an N+1."
    - "CastVote calls service.Resolve (02-01) instead of deciding visibility itself, so the cast response and the feed read path can never disagree (TRUST-02 / T-02-04)."
    - "ReportVoteContext LEFT JOINs sessions, so a pre-Phase-1.1 report whose session has no bound account yields a NULL reporter_account_id rather than zero rows — that report stays votable by everyone and the self-vote block correctly does not fire."
  prohibitions:
    - "No cached confirm_count / dispute_count / status column on reports, and no upsert-conflict or row-lock clause in any query this plan adds (D-08, T-02-02 — 02-02's TestVotesQuerySourceHasNoUpsertOrLock scans the file this plan appends to)."
    - "No second independence predicate anywhere: distinctness is computed by independentCellCount and nothing else (D-14)."
    - "No geohash string accepted from a caller — CastVoteInput has Latitude and Longitude only."
    - "No net/http, no chi, no HTTP status code and no handler type in internal/service/trust.go — the HTTP layer is 02-03b's."
---

## Phase Goal

**As a** person near an ongoing incident, **I want to** confirm or dispute another user's report **so that** reports lacking independent nearby corroboration lose visibility while well-corroborated ones stay trusted — and as a reporter or nearby confirmer, I want to mark a report resolved once it's no longer true, so the feed reflects what's actually happening right now.

<objective>
**This plan is one slice of that story: the server-side decision layer that turns a tap into a
counted, independence-checked vote.**

Given a caller, a report id, a vote kind/value and the caller's raw coordinates, this plan
decides whether the vote is allowed, computes the voter's own geohash cell server-side, appends
the vote, rebuilds the tally from the current-vote read, and hands back the freshly resolved
visibility. It is the half of plan 02-03 that has no HTTP surface; `02-03b` mounts it on four
routes.

**Why this plan was split out of 02-03.** The original 02-03 covered store query + service +
handlers + routes + `main.go` wiring + e2e tests + swagger regeneration — around ten files across
three architectural tiers. Split at the tier boundary, each half stays inside the phase's context
budget and each ends at an independently provable guarantee. The seam is clean: nothing in this
plan imports `net/http`, and nothing in 02-03b re-implements a rule decided here.

The decisions this plan implements, each cited by ID:

- **D-03** — the reporter cannot vote on their own report. Enforced in
  `VotingService.CastVote` by comparing the caller's account against the reporter account
  `ReportVoteContext` resolves through `reports` → `sessions`, before any row is written. The
  hidden button 02-05 renders is a courtesy; this check is the control (T-02-05).
- **D-13** — the reporter *can* mark their own report resolved, instantly and with no threshold.
  That is why D-03's block is scoped to `VoteKindContent` and never to `VoteKindResolution`. Get
  this scoping wrong in either direction and one of the two decisions breaks silently.
- **D-17** — the voter's location arrives as raw `{latitude, longitude}` captured by the browser's
  GPS prompt (once per session, cached). The server encodes the cell itself at
  `voterGeohashPrecision = 7`; there is no geohash field on `CastVoteInput` for a client to
  supply, extending `report.go`'s existing "server decides `ExpiresAt`/`Geohash`/`CreatedAt`"
  convention to the vote write path.
- **D-14** — one independence rule everywhere. `independentCellCount` is written once and reached
  through `BuildVoteTally` by every count `Resolve` consumes, so the Hidden trigger, the
  Provisional gate, confirmer-driven resolving and reopening all move together.
- **D-02** — content votes and resolution votes are separate `kind` values that never disturb each
  other's current-vote read, which is what keeps "is this report still true" and "is this
  resolved" independently tracked.

Purpose: TRUST-03 is the Core Value's load-bearing claim — a confirmation counts only if it comes
from a distinct verified account **and** a distinct geohash cell. This plan is where that stops
being a design statement and becomes a function with a test that fails if you delete it.

Output: the `ReportVoteContext` query plus its regenerated Go accessor, and
`internal/service/trust.go` / `internal/service/trust_test.go`.

**Scope boundary.** No HTTP handler, no route, no `Deps` field, no `main.go` change, no swagger
annotation, no JavaScript. Every one of those is 02-03b, which depends on this plan.

**Locked from RESEARCH Open Question 2 / Assumption A4:** a vote on an already-expired report is
rejected, not accepted-and-ignored. `02-UI-SPEC.md`'s Copywriting Contract already ships the
copy — "This report has expired." — and 02-03b maps the sentinel error this plan defines onto the
status code that carries it.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-PATTERNS.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-VALIDATION.md
@internal/service/report.go
@internal/service/auth.go
@internal/store/queries/reports.sql
@internal/store/queries/votes.sql
@sqlc.yaml
</context>

<tasks>

<task type="auto">
  <name>Task 1: One query answers every question CastVote asks about a report</name>

  <read_first>
    - `internal/store/queries/votes.sql` as written by 02-02 — the `-- name: X :verb` plus
      rationale-comment convention, and the two queries already there (`InsertVote`,
      `CurrentVotesForReports`). You are APPENDING to this file, not rewriting it.
    - `internal/store/queries/reports.sql`'s `ReportsByAccount` — the existing
      `reports` → `sessions` join this repo already uses to resolve a report's owning account, and
      its comment explaining why an INNER JOIN was acceptable there. Read that comment; this task
      deliberately reaches the opposite conclusion for a different reason, stated in `<action>`.
    - `internal/store/migrations/00001_create_reports.sql` — `reports.session_id` is plain `TEXT`
      with NO foreign key, and `sessions.session_id` is the `TEXT PRIMARY KEY`.
    - `internal/store/migrations/00002_add_accounts_and_verification.sql` — `sessions.account_id`
      is `BIGINT REFERENCES accounts(id)` and is **nullable on purpose** (its own comment explains
      why: a valid signed cookie can exist with no sessions row at all).
    - `sqlc.yaml` — package `sqlcgen`, `sql_package: pgx/v5`, `emit_pointers_for_null_types: true`
      (this is what turns the nullable `account_id` into `*int64`), and the non-null `timestamptz`
      override to `time.Time`.
    - `Makefile` — the `sqlc` target; generated output is committed to git on purpose.
    - `internal/store/votes_test.go` as written by 02-02 — specifically
      `TestVotesQuerySourceHasNoUpsertOrLock`, which reads `queries/votes.sql` from disk and scans
      its non-comment body. It will run against the file you are appending to.
  </read_first>

  <files>internal/store/queries/votes.sql, internal/store/sqlc/votes.sql.go, internal/store/sqlc/models.go, internal/store/sqlc/db.go</files>

  <action>
Append exactly one new named query to `internal/store/queries/votes.sql`, below the two 02-02
already wrote. Do not modify either existing query.

The query is `-- name: ReportVoteContext :one`. It selects four things: `r.severity`,
`r.category`, `r.expires_at`, and `s.account_id` aliased as `reporter_account_id`. It reads
`FROM reports r LEFT JOIN sessions s ON r.session_id = s.session_id` and filters
`WHERE r.id = sqlc.arg(report_id)`.

Four points the rationale comment above it must make, because each one is a place a later reader
would otherwise "correct" it.

(a) **Why one query instead of two.** `02-PATTERNS.md` and `02-RESEARCH.md` both anticipated a
narrower `ReporterAccountID` lookup plus a separate read of the report's severity/category.
`CastVote` needs all four values on every single call, and reading them in one statement removes
a window in which a report could expire or be deleted between the reporter lookup and the meta
read. This query supersedes `ReporterAccountID`; that narrower query is deliberately never
created.

(b) **Why the join stops at `sessions`.** `sessions.account_id` IS `accounts.id` — it is a foreign
key onto that column. A third join through `accounts` could only re-confirm the existence of a row
the foreign key already guarantees, at the cost of an extra join, so it is omitted on purpose, not
by oversight.

(c) **Why `LEFT JOIN` and not `INNER JOIN`.** `reports.session_id` carries no foreign key, and
`sessions.account_id` is nullable — both deliberate, both documented in migration `00002`. A
report submitted before Phase 1.1 made login mandatory therefore has no account-bound session. An
INNER JOIN would return zero rows for such a report, which `CastVote` cannot distinguish from
"no such report," and the report would become unvotable. With the LEFT JOIN it returns a NULL
`reporter_account_id` instead: nobody matches it, the D-03 self-vote block correctly does not
fire, and the report stays votable. Zero rows then means exactly one thing — the report id does
not exist.

(d) **What this query is for.** It is the single read `VotingService.CastVote` makes before
deciding: the reporter identity for D-03's self-vote block, the severity/category pair
`service.Resolve` needs as `ReportMeta`, and `expires_at` for the expired-report rejection.

Then run `make sqlc` to regenerate. Commit `internal/store/sqlc/votes.sql.go`,
`internal/store/sqlc/models.go` and `internal/store/sqlc/db.go` together — `sqlc generate` rewrites
all three and this repo commits generated output on purpose (`sqlc.yaml`'s header and
`.github/workflows/ci.yml` both record why: CI has no sqlc binary). If the `sqlc` binary is
missing, install the pinned version with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`,
never an unpinned `@latest`.

The generated shape you must end up with, and which Task 2 is written against:
a struct `ReportVoteContextRow` with fields `Severity string`, `Category string`,
`ExpiresAt time.Time` and `ReporterAccountID *int64`; and a method
`func (q *Queries) ReportVoteContext(ctx context.Context, reportID int64) (ReportVoteContextRow, error)`.
The pointer on `ReporterAccountID` comes from `emit_pointers_for_null_types: true` meeting the
nullable `sessions.account_id` column — if it generates as a bare `int64`, the column reference is
wrong and the NULL case above is silently unrepresentable, so treat that as a failure rather than
adapting Task 2 to it.

Add nothing else to this file. No upsert-conflict clause, no row-lock clause, no new index, no
column on `reports`.
  </action>

  <acceptance_criteria>
    - `internal/store/queries/votes.sql` contains exactly three `-- name:` lines: `InsertVote`,
      `CurrentVotesForReports`, `ReportVoteContext`.
    - `make sqlc` exits 0. This is a real gate — sqlc type-checks the query against the embedded
      migration schema, so a wrong table, column or alias fails here rather than at runtime.
    - `grep -c 'type ReportVoteContextRow struct' internal/store/sqlc/votes.sql.go` is exactly `1`.
    - `grep -c 'func (q \*Queries) ReportVoteContext(ctx context.Context, reportID int64) (ReportVoteContextRow, error)' internal/store/sqlc/votes.sql.go` is exactly `1`.
    - `grep -c 'ReporterAccountID \*int64' internal/store/sqlc/votes.sql.go` is exactly `1` — the
      nullable reporter account survived code generation as a pointer.
    - 02-02's guards still pass against the appended file:
      `go test ./internal/store/... -short -run TestVotesQuerySourceHasNoUpsertOrLock` reports PASS,
      not SKIP.
    - `go build ./...` and `go vet ./...` both exit 0.
    - With `DATABASE_URL` set, `go test ./internal/store/... -p 1` passes — every 02-02 vote test
      and every Phase 1/1.1 store test still green.
  </acceptance_criteria>

  <verify>
    <automated>make sqlc && go build ./... && go vet ./... && [ "$(grep -c -- '-- name:' internal/store/queries/votes.sql)" = "3" ] && [ "$(grep -c 'type ReportVoteContextRow struct' internal/store/sqlc/votes.sql.go)" = "1" ] && [ "$(grep -c 'ReporterAccountID \*int64' internal/store/sqlc/votes.sql.go)" = "1" ] && go test ./internal/store/... -short && go test ./internal/store/... -p 1</automated>
  </verify>

  <done>
    `ReportVoteContext` is a typed Go method on `sqlcgen.Queries` returning the report's severity,
    category, expiry and (possibly NULL) reporter account in one round trip; the vote-query file
    still carries no upsert or lock clause; the tree builds and every existing store test passes
    against the regenerated layer.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: One independence rule, written once — the vote vocabulary and the tally builder</name>

  <read_first>
    - `internal/service/visibility.go` as completed by 02-01 — the exact `VoteTally` field list
      (`ConfirmCells`, `DisputeCells`, `ResolveCells`, `ReopenCells`, `ReporterResolved`), its
      struct doc comment (which explicitly assigns the job of populating it to this file), and
      `IndependentAgreementThreshold`. Do not redeclare or shadow anything declared there.
    - `internal/service/report.go` lines 39-123 — the `Category` / `Severity` / `CapacityStatus`
      enum shape to mirror exactly: a string type, grouped constants, an exported ordered
      `Xs` slice, and a `Valid()` method that range-loops that slice with exact comparison and no
      case coercion. Also line 25's `geohashPrecision = 8` constant and its comment — the value
      this file must deliberately NOT reuse.
    - `internal/service/report.go` lines 125-135 — the `ValidationError{Field, Message}` type and
      its doc comment's rule that a server rejection must never contradict the client's own inline
      copy.
    - `internal/store/sqlc/votes.sql.go` — the exact `CurrentVotesForReportsRow` field names
      (`ReportID`, `AccountID`, `Kind`, `Value`, `GeohashCell`, `CreatedAt`).
    - `internal/service/report_test.go` and `internal/service/auth_test.go` headers — both declare
      `package service_test` (the EXTERNAL test package). This task's test file deliberately does
      not; see `<action>`.
    - `02-RESEARCH.md` "Pattern 3" — the reference implementation for `independentCellCount` and
      the `voterGeohashPrecision = 7` comment, written against this repo's conventions.
    - `02-VALIDATION.md` "Wave 0 Requirements", second bullet — it names
      `internal/service/trust_test.go`, `independentCellCount()` and `BuildVoteTally()` directly,
      and names `TestIndependentCellCount` in the Per-Task Verification Map. Those names are
      pinned; do not rename them.
  </read_first>

  <files>internal/service/trust.go, internal/service/trust_test.go</files>

  <behavior>
    Written before the implementation and failing against it, then made to pass without weakening
    an assertion. Every case uses fabricated `sqlcgen.CurrentVotesForReportsRow` values — no
    database, no fixtures.

    `TestVoteKindAndValueValidation`:
    - `VoteKindContent.Valid()` and `VoteKindResolution.Valid()` are true; `VoteKind("")` and
      `VoteKind("Content")` are false (exact comparison, no case coercion).
    - `VoteConfirm.ValidFor(VoteKindContent)` and `VoteDispute.ValidFor(VoteKindContent)` are true;
      `VoteResolve.ValidFor(VoteKindContent)` and `VoteReopen.ValidFor(VoteKindContent)` are false.
    - `VoteResolve.ValidFor(VoteKindResolution)` and `VoteReopen.ValidFor(VoteKindResolution)` are
      true; `VoteConfirm.ValidFor(VoteKindResolution)` and
      `VoteDispute.ValidFor(VoteKindResolution)` are false.
    - `VoteValue("confirm ")` and `VoteValue("CONFIRM")` are valid for no kind.

    `TestIndependentCellCount` — the name pinned by `02-VALIDATION.md`:
    - nil slice and empty slice → 0.
    - three entries all equal to `"tdr1qg0"` → 1. This is the TRUST-03 headline: three accounts
      standing together count once.
    - three distinct cells → 3.
    - mixed (`tdr1qg0`, `tdr1qg1`, `tdr1qg0`, `tdr1qg1`, `tdr1qg2`) → 3.
    - two empty strings → 1, with an in-test comment that an empty cell is impossible in practice
      because `votes.geohash_cell` is `NOT NULL` and `CastVote` always computes a 7-character
      value; the case is asserted so the helper's behaviour is defined rather than accidental.

    `TestBuildVoteTally` — every case passes `reportID = 1` unless stated:
    - two `content`/`confirm` rows, distinct accounts, distinct cells → `ConfirmCells` 2.
    - two `content`/`confirm` rows, distinct accounts, IDENTICAL cell → `ConfirmCells` 1. This is
      the discriminating case: a tally built by counting rows instead of cells would read 2, and
      the report would wrongly clear the Provisional gate. TRUST-03's entire claim is this one
      assertion.
    - two `content`/`dispute` rows in distinct cells → `DisputeCells` 2, `ConfirmCells` 0.
    - rows whose `ReportID` is 2 are ignored entirely when `reportID` is 1, including when they
      would otherwise dominate the counts. This proves the filter 02-04's batched read depends on.
    - `reporterAccountID` non-nil and a `resolution`/`resolve` row from that account →
      `ReporterResolved` true, `ResolveCells` 0 (the reporter's own vote is surfaced through the
      flag and excluded from the independent count, per 02-01's `VoteTally` doc comment).
    - `reporterAccountID` non-nil and a `resolution`/`reopen` row from that account →
      `ReporterResolved` false, `ReopenCells` 0. The reporter has no reopen channel (D-16's locked
      resolution); the row is dropped rather than counted.
    - two `resolution`/`resolve` rows from two NON-reporter accounts in distinct cells →
      `ResolveCells` 2, `ReporterResolved` false.
    - `reporterAccountID` nil → no row is treated as the reporter's; a `resolution`/`resolve` row
      counts toward `ResolveCells` normally.
    - rows with an unrecognised `Kind` (`"telemetry"`) or an unrecognised `Value` (`"maybe"`) are
      ignored and change no count — the vote vocabulary can grow in Phase 3 without a migration
      (02-02's schema left `kind`/`value` unconstrained for exactly this reason).

    `TestBuildVoteTallyExcludesReporterOwnContentVote`:
    - a `content`/`confirm` row from the reporter's own account, plus one from another account in a
      different cell → `ConfirmCells` 1, not 2. D-03 stops these being written in the first place;
      this is the read-side second layer that also covers any row predating the check (T-02-05).

    `TestBuildVoteTallyFeedsResolveEndToEnd`:
    - Build a tally from two confirm rows in distinct cells and pass it straight into
      `Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, tally, time.Now())`,
      asserting `VisibilityLive` / `ReasonConfirmed`. Repeat with the same two rows in ONE cell,
      asserting `VisibilityProvisional` / `ReasonAwaitingConfirmation`. This is the seam between
      02-01 and this plan, asserted rather than assumed.
  </behavior>

  <action>
Create `internal/service/trust.go`. It belongs to the existing `package service`; do NOT write a
second package doc comment — `internal/service/report.go` already owns it. Open the file with a
plain file-level comment (a blank line between it and the `package service` line, so it cannot be
mistaken for a package doc) stating that this file holds the vote vocabulary, the one independence
predicate, and the voting service, and that unlike `visibility.go` it deliberately DOES consume
generated store types.

Imports: `context`, `errors`, `time`, `github.com/mmcloughlin/geohash`,
`github.com/jackc/pgx/v5`, and `sqlcgen "pinalert/internal/store/sqlc"`. Nothing from `net/http`,
nothing from `chi`. Follow `internal/service/auth.go`'s existing import grouping (stdlib, blank
line, third-party, blank line, local).

Declare, in this order:

1. `const voterGeohashPrecision = 7`. Its doc comment must say explicitly that this is
   deliberately DIFFERENT from `report.go`'s `geohashPrecision = 8`, that the two answer different
   questions (where a report happened vs. how far apart two voters must stand to count as
   independent), that 7 is roughly a 153m cell chosen against `PITFALLS.md`'s ~100-300m realistic
   incident radius, that it is an interim documented choice logged as assumption A3 in
   `02-RESEARCH.md` which Phase 3 revisits with a benchmarked value (TRUST-05), and that it is
   explicitly not a library default. Reusing precision 8 here is the mistake the comment exists to
   prevent.

2. `type VoteKind string` with `VoteKindContent` = `"content"` and `VoteKindResolution` =
   `"resolution"`, `var VoteKinds = []VoteKind{VoteKindContent, VoteKindResolution}`, and
   `func (k VoteKind) Valid() bool` range-looping `VoteKinds`. Mirror `report.go`'s `Severity`
   shape exactly, including the exact-comparison-no-case-coercion note. The doc comment must state
   that the two kinds exist so "is this report still true" and "is this resolved" stay separately
   tracked signals rather than one overloaded vote (D-02, D-16), and that these string values are
   written into `votes.kind`, which 02-02 left as unconstrained `TEXT` precisely so this enum is
   the single validator.

3. `type VoteValue string` with `VoteConfirm` = `"confirm"`, `VoteDispute` = `"dispute"`,
   `VoteResolve` = `"resolve"`, `VoteReopen` = `"reopen"`, and
   `var VoteValues = []VoteValue{VoteConfirm, VoteDispute, VoteResolve, VoteReopen}`.

4. `func (v VoteValue) ValidFor(k VoteKind) bool` — returns true only for the confirm/dispute pair
   under `VoteKindContent` and only for the resolve/reopen pair under `VoteKindResolution`; false
   for every other combination including an unknown kind. Write it as a switch on `k` so adding a
   third kind later fails to compile silently rather than defaulting to permissive. Its doc comment
   should note this is the V5 Input Validation control: the routes 02-03b mounts pair kind and
   value at compile time, so a mismatch is unreachable through the HTTP surface today — this method
   is the defence that survives a future caller that does not have that guarantee.

5. The three sentinel errors, each with a doc comment naming the decision it enforces and the
   response 02-03b maps it to. Follow `auth.go`'s `ErrRateLimited = errors.New("service: rate limited")`
   convention for the message prefix:
   - `ErrCannotVoteOwnReport` — D-03, mapped to 403.
   - `ErrReportExpired` — RESEARCH Open Question 2's locked answer, mapped to 409.
   - `ErrReportNotFound` — mapped to 404; returned when `ReportVoteContext` yields `pgx.ErrNoRows`.

6. `func independentCellCount(cells []string) int` — build a `map[string]struct{}` sized
   `len(cells)`, insert every entry, return `len(seen)`. Its doc comment must state that this is
   the ONE implementation TRUST-03's independence predicate reduces to, that D-14 requires it be
   reused by the Hidden trigger, the Provisional gate, confirmer-driven resolving and reopening
   rather than reimplemented per gate, and that the distinct-ACCOUNT half of the predicate is
   already guaranteed by `CurrentVotesForReports`'s `DISTINCT ON` — callers must pass an
   already-per-account-deduped slice, and this function does not deduplicate by account itself.
   It stays unexported so nothing outside this package can build a competing count.

7. `func BuildVoteTally(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) VoteTally`.
   Collect four `[]string` slices of geohash cells, then return a `VoteTally` whose four count
   fields are each `independentCellCount` of one slice, plus the `ReporterResolved` flag.

   Per row: skip it unless `row.ReportID == reportID`. Determine whether the row belongs to the
   reporter — true only when `reporterAccountID != nil && row.AccountID == *reporterAccountID`.
   Then switch on `VoteKind(row.Kind)`:
   - `VoteKindContent`: if the row is the reporter's, skip it entirely (defence in depth for
     T-02-05 — D-03 stops these at write time, and this covers any row that predates the check);
     otherwise append `row.GeohashCell` to the confirm slice for `VoteConfirm`, the dispute slice
     for `VoteDispute`, and ignore any other value.
   - `VoteKindResolution`: if the row is the reporter's, set `ReporterResolved` to
     `VoteValue(row.Value) == VoteResolve` and append the cell to nothing at all — D-13 routes the
     reporter through the flag, and D-16's locked resolution gives the reporter no reopen channel,
     so a reporter `reopen` row sets the flag false and is otherwise dropped. Otherwise append the
     cell to the resolve slice for `VoteResolve` or the reopen slice for `VoteReopen`, ignoring any
     other value.
   - any other kind: ignore.

   The doc comment must state four things: that the `reportID` parameter exists because 02-04
   loads votes for a whole page of reports in ONE `CurrentVotesForReports` call and then calls this
   per report — removing the parameter would push the feed path into an N+1; that unrecognised
   kinds and values are ignored rather than erroring, so Phase 3 can extend the vocabulary without
   a migration; that the reporter's own resolution vote is surfaced via `ReporterResolved` and
   excluded from `ResolveCells`/`ReopenCells` exactly as `visibility.go`'s `VoteTally` doc comment
   specifies; and that this function performs no I/O and takes no `context.Context`, so it is as
   testable as `Resolve` itself.

Create `internal/service/trust_test.go` declaring **`package service`** — the INTERNAL test
package, not `service_test`. Put a comment at the top of the file explaining why: `independentCellCount`
is unexported and `02-VALIDATION.md` names `TestIndependentCellCount` as a required Wave 0 test, so
the test must live inside the package; Go permits a `service` and a `service_test` test package
side by side in one directory, and `report_test.go` / `auth_test.go` remain external. Because this
file is internal, reference every symbol unqualified (`VoteConfirm`, not `service.VoteConfirm`).

Cover every case in `<behavior>`. Structure each function as a slice of anonymous-struct cases with
a `name` field holding a prose sentence, iterated with `t.Run(tc.name, ...)`, matching
`report_test.go`'s prose-subtest convention. Write a tiny unexported helper that builds a
`sqlcgen.CurrentVotesForReportsRow` from (reportID, accountID, kind, value, cell) so the tables stay
readable, and a second helper returning `*int64` from an `int64` for the `reporterAccountID`
argument.

Write the test file first and confirm it fails against unimplemented stubs before filling the
bodies in. Do not add `CastVote` or `VotingService` in this task — they are Task 3.
  </action>

  <acceptance_criteria>
    - `gofmt -l internal/service/` prints nothing and `go vet ./internal/service/` exits 0.
    - `internal/service/trust_test.go` declares `package service` (the internal test package) and
      `internal/service/report_test.go` still declares `package service_test` — both packages
      coexist in the directory.
    - All five test functions exist with exactly these names: `TestVoteKindAndValueValidation`,
      `TestIndependentCellCount`, `TestBuildVoteTally`,
      `TestBuildVoteTallyExcludesReporterOwnContentVote`, `TestBuildVoteTallyFeedsResolveEndToEnd`.
    - `go test ./internal/service/ -run 'TestVoteKindAndValueValidation|TestIndependentCellCount|TestBuildVoteTally' -count=1` passes.
    - `02-VALIDATION.md`'s TRUST-03 command runs green:
      `go test ./internal/service/... -run TestIndependentCellCount`.
    - `grep -c 'func independentCellCount(cells \[\]string) int' internal/service/trust.go` is
      exactly `1` — one implementation, not one per gate (D-14).
    - `grep -c 'voterGeohashPrecision = 7' internal/service/trust.go` is exactly `1`.
    - `grep -c 'func BuildVoteTally(rows \[\]sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID \*int64) VoteTally' internal/service/trust.go` is exactly `1` — the batched-read
      signature 02-04 depends on.
    - `internal/service/visibility.go` is not edited by this task: it still declares exactly one
      `func Resolve(` and its import set is still exactly `time`.
    - `go test ./... -short` passes — no Phase 1 or Phase 1.1 test regressed.
  </acceptance_criteria>

  <verify>
    <automated>[ -z "$(gofmt -l internal/service/)" ] && go vet ./internal/service/ && [ "$(grep -c 'func independentCellCount(cells \[\]string) int' internal/service/trust.go)" = "1" ] && [ "$(grep -c 'voterGeohashPrecision = 7' internal/service/trust.go)" = "1" ] && [ "$(grep -c 'func BuildVoteTally(rows \[\]sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID \*int64) VoteTally' internal/service/trust.go)" = "1" ] && [ "$(grep -c 'func Resolve(' internal/service/visibility.go)" = "1" ] && grep -q '^package service$' internal/service/trust_test.go && for f in TestVoteKindAndValueValidation TestIndependentCellCount TestBuildVoteTally TestBuildVoteTallyExcludesReporterOwnContentVote TestBuildVoteTallyFeedsResolveEndToEnd; do grep -q "^func ${f}(t \*testing.T)" internal/service/trust_test.go || { echo "missing ${f}"; exit 1; }; done && go test ./internal/service/... -run TestIndependentCellCount -count=1 && go test ./internal/service/ -count=1 && go test ./... -short</automated>
  </verify>

  <done>
    The vote vocabulary validates itself, `independentCellCount` exists exactly once, and
    `BuildVoteTally` turns a batch of current-vote rows into the `VoteTally` 02-01's `Resolve`
    consumes — with two accounts in one cell proven to count as one, the reporter's own resolution
    vote proven to route through `ReporterResolved`, and the tally proven to drive `Resolve` to the
    right answer end to end.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: VotingService.CastVote — the reporter blocked, the cell computed server-side, the fresh visibility returned</name>

  <read_first>
    - `internal/service/trust.go` as written in Task 2 — the enums, sentinel errors,
      `independentCellCount` and `BuildVoteTally` you are about to build on.
    - `internal/service/report.go` lines 270-330 — the `Querier` interface /
      `ReportService` struct / `NewReportService` constructor idiom to mirror exactly, and
      `Submit`'s server-computed-field block (`now := time.Now().UTC()`, then `expiresAt`, then
      `geohash.EncodeWithPrecision(...)`) — the precedent `CastVote` follows for computing the
      voter's cell.
    - `internal/service/auth.go` lines 51-68 and 99-110 — the `AuthQuerier` interface shape and
      `NewAuthService` constructor. Note that `WithClock`/`AuthOption` already exist in this
      package under those exact names; do NOT add a second clock seam or a colliding option type.
      `CastVote` reads `time.Now().UTC()` directly and tests exercise expiry by programming the
      fake querier's `ExpiresAt`, which needs no seam at all.
    - `internal/service/auth_test.go` lines 22-135 — `fakeAuthQuerier`: a struct with one
      programmable return value and one recorded argument per method. This is the exact shape the
      fake `VotingQuerier` in this task copies.
    - `internal/store/sqlc/votes.sql.go` — `InsertVoteParams`' exact field names (`ReportID`,
      `AccountID`, `Kind`, `Value`, `GeohashCell`) and the `ReportVoteContextRow` shape from Task 1.
    - `internal/service/visibility.go` — `Resolve`'s signature, `ReportMeta`,
      `IndependentAgreementThreshold`, and the reason constants.
    - `02-RESEARCH.md` "Pattern 3"'s `CastVote` example — the reference shape. Note that the
      example loads the reporter id with a narrower query and leaves `meta` "omitted for brevity";
      Task 1's `ReportVoteContext` supplies both, so this implementation is the completed form of
      that sketch, not a departure from it.
  </read_first>

  <files>internal/service/trust.go, internal/service/trust_test.go</files>

  <behavior>
    Written before the implementation and failing against it. Every test drives a fake
    `VotingQuerier` that records the `InsertVoteParams` it was handed and returns a programmed
    `ReportVoteContextRow` and a programmed `[]sqlcgen.CurrentVotesForReportsRow`. No database.

    `TestCastVoteRejectsReporterContentVote` (D-03, T-02-05):
    - reporter account 7, caller account 7, kind `content`, value `confirm` →
      `errors.Is(err, ErrCannotVoteOwnReport)` is true, and the fake records ZERO `InsertVote`
      calls. Assert both: rejecting after writing the row would still fail the requirement.
    - Same with value `dispute` → identical outcome.

    `TestCastVoteAllowsReporterResolutionVote` (D-13, TRUST-08):
    - reporter account 7, caller account 7, kind `resolution`, value `resolve` → no error, exactly
      one `InsertVote` call recorded. The doc comment on this test must say plainly that a block
      applied to both kinds would silently break D-13's reporter-instant resolve, which is why this
      test exists next to the one above.
    - With the current-vote rows programmed to include that reporter `resolution`/`resolve` row,
      the returned result is `VisibilityRetracted` / `ReasonResolved` — the reporter's instant path
      end to end.

    `TestCastVoteAllowsNonReporter`:
    - reporter account 7, caller account 9, kind `content`, value `confirm` → no error, one
      `InsertVote` recorded with `ReportID`, `AccountID`, `Kind` and `Value` matching the input.

    `TestCastVoteTreatsNullReporterAsNobody`:
    - `ReporterAccountID` nil (a pre-Phase-1.1 report with no account-bound session), caller
      account 7, kind `content` → no error and the vote is inserted. A nil reporter must not be
      compared equal to any caller.

    `TestCastVoteRejectsExpiredReport`:
    - `ExpiresAt` one hour in the past → `errors.Is(err, ErrReportExpired)` and zero `InsertVote`
      calls.
    - `ExpiresAt` exactly the value `time.Now()` would have to beat is not asserted (it is a
      racing boundary); instead assert that `ExpiresAt` one hour in the FUTURE is accepted, so the
      predicate's direction is pinned.

    `TestCastVoteRejectsMissingReport`:
    - the fake's `ReportVoteContext` returns `pgx.ErrNoRows` → `errors.Is(err, ErrReportNotFound)`
      is true, `errors.Is(err, pgx.ErrNoRows)` is NOT required to hold, and zero `InsertVote` calls
      are recorded.

    `TestCastVoteComputesGeohashCellServerSide` (D-17, T-02-01):
    - For coordinates 12.9716 / 77.5946, the recorded `InsertVoteParams.GeohashCell` equals
      `geohash.EncodeWithPrecision(12.9716, 77.5946, 7)` computed independently in the test, and
      its length is exactly 7.
    - Two callers 40 metres apart (same coordinates with a ~0.0004 degree latitude offset) produce
      the SAME cell; two callers several hundred metres apart produce DIFFERENT cells. Compute both
      expected values in the test with `geohash.EncodeWithPrecision` rather than hardcoding
      strings, so the assertion tracks `voterGeohashPrecision` if it ever changes.
    - `CastVoteInput` has no geohash field to set — asserted structurally by the acceptance
      criteria, not here.

    `TestCastVoteValidatesInput`:
    - kind `VoteKind("bogus")` → a `ValidationError` with `Field` `"kind"`.
    - kind `content` with value `resolve` → a `ValidationError` with `Field` `"value"`.
    - latitude 91 → `ValidationError` with `Field` `"latitude"`; longitude -181 →
      `ValidationError` with `Field` `"longitude"`.
    - In every one of these cases the fake records ZERO calls of ANY method — validation runs
      before the store is touched at all.

    `TestCastVoteReturnsFreshlyResolvedVisibility` (D-04, TRUST-01, TRUST-03):
    - Non-critical report, current-vote rows programmed as one `content`/`confirm` from one
      account → `VisibilityProvisional` / `ReasonAwaitingConfirmation`.
    - Same report, rows programmed as two `content`/`confirm` from two accounts in DISTINCT cells →
      `VisibilityLive` / `ReasonConfirmed`.
    - Same report, rows programmed as two `content`/`confirm` from two accounts in the SAME cell →
      `VisibilityProvisional`. This is TRUST-03 proven through the full service path rather than
      only at the helper.
    - The fake asserts that `CurrentVotesForReports` was called AFTER `InsertVote` (record an
      ordered call log and assert the sequence), so the response can never reflect a pre-vote
      tally.
  </behavior>

  <action>
Append to `internal/service/trust.go`, below Task 2's declarations. Change nothing Task 2 wrote.

Declare `type VotingQuerier interface` with exactly three methods, matching the generated
signatures verbatim: `InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error`;
`CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)`;
and `ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)`.
Its doc comment must say this is the entire store surface the voting path touches — small enough to
fake without a real Postgres, mirroring `report.go`'s `Querier` — and that 02-04 will write its own
fake against this exact shape. Note that the parameter is named `reportIDs` here while sqlc emits
`reportIds`; Go matches interfaces structurally, so the names need not agree, but the types must.

Declare `type VotingService struct { q VotingQuerier }` and
`func NewVotingService(q VotingQuerier) *VotingService`, mirroring `NewReportService` exactly. No
options, no clock seam — see `<read_first>`.

Declare `type CastVoteInput struct` with exactly six fields: `ReportID int64`, `AccountID int64`,
`Kind VoteKind`, `Value VoteValue`, `Latitude float64`, `Longitude float64`. Its doc comment must
state, in `SubmitInput`'s own idiom, that `GeohashCell` and `CreatedAt` are deliberately absent
because the server computes them, and that `AccountID` comes from the verified-account gate's
request context (02-03b), never from a request body — a client cannot assert whose vote this is.

Declare `type CastVoteResult struct { Visibility Visibility; Reason ResolveReason }` — the two
values 02-03b serialises and 02-05/02-06 render. Declaring a named result rather than returning a
bare pair leaves room for 02-04's response work to grow the shape without changing every caller's
arity.

Implement `func (s *VotingService) CastVote(ctx context.Context, in CastVoteInput) (CastVoteResult, error)`
as an ordered sequence. The order is part of the specification and the doc comment must say so,
with the reason for each position:

1. **Validate the input, before touching the store.** If `!in.Kind.Valid()` return a
   `ValidationError{"kind", ...}`. If `!in.Value.ValidFor(in.Kind)` return
   `ValidationError{"value", ...}`. Range-check latitude to -90..90 and longitude to -180..180,
   returning `ValidationError{"latitude", ...}` / `ValidationError{"longitude", ...}`. For the two
   coordinate messages reuse `02-UI-SPEC.md`'s GPS-denial copy verbatim — "Voting needs your
   location, so nearby confirmations can be verified as independent. Turn on location access for
   this site and try again." — because a missing or nonsense coordinate on the wire means exactly
   the situation that copy describes, and `report.go`'s `ValidationError` doc comment requires a
   server rejection never to contradict the client's own inline copy. The kind/value messages are
   developer-facing (the routes pair them at compile time, so a user cannot trigger them); keep
   them short and factual. Validating first means garbage input costs zero database round trips.
2. **Load the report's vote context.** Call `s.q.ReportVoteContext(ctx, in.ReportID)`. If the error
   satisfies `errors.Is(err, pgx.ErrNoRows)`, return `ErrReportNotFound`; any other error is
   returned as-is for the handler to log and map to 500.
3. **Apply D-03's self-vote block.** If `in.Kind == VoteKindContent` and
   `rc.ReporterAccountID != nil` and `*rc.ReporterAccountID == in.AccountID`, return
   `ErrCannotVoteOwnReport`. Comment that the `VoteKindContent` guard is load-bearing in both
   directions: dropping it would break D-13's reporter-instant resolve, and widening it past
   content votes would silently disable the one action D-13 exists to grant. Comment that this sits
   above the expiry check because it is the authorisation decision and should not depend on a
   state check that might later move. The nil check is not defensive noise — a pre-Phase-1.1
   report legitimately has no reporter account, and a nil pointer must never compare equal to a
   caller.
4. **Reject an expired report.** Take `now := time.Now().UTC()` once, here, as the authoritative
   clock — never a client-supplied timestamp, matching `Submit`'s own convention — and if
   `!rc.ExpiresAt.After(now)` return `ErrReportExpired`. Comment that this is RESEARCH Open
   Question 2's locked answer (reject, do not accept-and-ignore) and that `02-UI-SPEC.md` ships the
   user-facing copy.
5. **Compute the voter's cell server-side.** `cell := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, voterGeohashPrecision)`.
   Comment that this is the T-02-01 control: the client sends raw coordinates only, and any
   geohash string a client might invent has nowhere to enter — `CastVoteInput` has no such field.
6. **Append the vote.** `s.q.InsertVote(ctx, sqlcgen.InsertVoteParams{...})` with `Kind` and
   `Value` converted via `string(...)`. Return any error unwrapped.
7. **Read the current votes back.** `s.q.CurrentVotesForReports(ctx, []int64{in.ReportID})`. This
   must come after the insert — the response's whole purpose (D-04: the client waits for the server
   rather than updating optimistically) is to carry the POST-vote state.
8. **Build the tally and resolve.** `tally := BuildVoteTally(rows, in.ReportID, rc.ReporterAccountID)`,
   then `meta := ReportMeta{Severity: Severity(rc.Severity), Category: Category(rc.Category)}`, then
   `vis, reason := Resolve(meta, tally, now)` reusing the same `now` from step 4. Return
   `CastVoteResult{Visibility: vis, Reason: reason}`.

The function doc comment must additionally record that `CastVote` calls `Resolve` rather than
deciding visibility itself, so the vote-cast response and 02-04's feed read give byte-identical
answers for the same data (TRUST-02 / T-02-04), and that no transaction, row lock or conflict
clause appears anywhere in this sequence because 02-02's append-only schema has no contended row
to serialise (T-02-02).

Append the tests from `<behavior>` to `internal/service/trust_test.go`, still `package service`.
Write one `fakeVotingQuerier` struct at the top of the new section carrying: a programmable
`ReportVoteContextRow` and error; a programmable `[]sqlcgen.CurrentVotesForReportsRow` and error; a
programmable `InsertVote` error; a recorded slice of every `InsertVoteParams` it received; and an
ordered `[]string` call log appended to by each of the three methods (`"ReportVoteContext"`,
`"InsertVote"`, `"CurrentVotesForReports"`). The call log is what makes
`TestCastVoteReturnsFreshlyResolvedVisibility`'s ordering assertion and every "zero calls
recorded" assertion mechanical rather than inferred.

Do not create any HTTP type, route or handler in this task.
  </action>

  <acceptance_criteria>
    - `gofmt -l internal/service/` prints nothing and `go vet ./internal/service/` exits 0.
    - All nine test functions exist with exactly these names: `TestCastVoteRejectsReporterContentVote`,
      `TestCastVoteAllowsReporterResolutionVote`, `TestCastVoteAllowsNonReporter`,
      `TestCastVoteTreatsNullReporterAsNobody`, `TestCastVoteRejectsExpiredReport`,
      `TestCastVoteRejectsMissingReport`, `TestCastVoteComputesGeohashCellServerSide`,
      `TestCastVoteValidatesInput`, `TestCastVoteReturnsFreshlyResolvedVisibility`.
    - `go test ./internal/service/ -run TestCastVote -count=1` passes with zero failures.
    - `02-VALIDATION.md`'s TRUST-01 command runs green:
      `go test ./internal/service/... ./internal/store/... -run TestCastVote -p 1`.
    - `grep -c 'func (s \*VotingService) CastVote(ctx context.Context, in CastVoteInput) (CastVoteResult, error)' internal/service/trust.go` is exactly `1`.
    - `grep -c 'geohash.EncodeWithPrecision' internal/service/trust.go` is exactly `1` — the cell is
      computed in one place, on the server.
    - `CastVoteInput` declares exactly six fields and none of them is a geohash: the struct body in
      `internal/service/trust.go` between `type CastVoteInput struct {` and its closing brace
      contains exactly the field names `ReportID`, `AccountID`, `Kind`, `Value`, `Latitude`,
      `Longitude`.
    - `internal/service/trust.go` imports no HTTP or routing package. Scoped to the import block
      only, so a doc comment elsewhere in the file that mentions the HTTP layer cannot invalidate
      its own gate:
      `awk '/^import \(/{b=1;next} b&&/^\)/{b=0} b' internal/service/trust.go | grep -qE 'net/http|go-chi/chi'`
      exits non-zero (no match).
    - `go test ./... -short` passes, and with `DATABASE_URL` set `go test ./... -p 1` passes.
  </acceptance_criteria>

  <verify>
    <automated>[ -z "$(gofmt -l internal/service/)" ] && go vet ./... && [ "$(grep -c 'func (s \*VotingService) CastVote(ctx context.Context, in CastVoteInput) (CastVoteResult, error)' internal/service/trust.go)" = "1" ] && [ "$(grep -c 'geohash.EncodeWithPrecision' internal/service/trust.go)" = "1" ] && ! awk '/^import \(/{b=1;next} b&&/^\)/{b=0} b' internal/service/trust.go | grep -qE 'net/http|go-chi/chi' && for f in TestCastVoteRejectsReporterContentVote TestCastVoteAllowsReporterResolutionVote TestCastVoteAllowsNonReporter TestCastVoteTreatsNullReporterAsNobody TestCastVoteRejectsExpiredReport TestCastVoteRejectsMissingReport TestCastVoteComputesGeohashCellServerSide TestCastVoteValidatesInput TestCastVoteReturnsFreshlyResolvedVisibility; do grep -q "^func ${f}(t \*testing.T)" internal/service/trust_test.go || { echo "missing ${f}"; exit 1; }; done && go test ./internal/service/ -count=1 && go test ./internal/service/... ./internal/store/... -run TestCastVote -p 1 && go test ./... -short && go test ./... -p 1</automated>
  </verify>

  <done>
    `VotingService.CastVote` refuses the reporter's own content vote before writing anything, still
    honours the reporter's instant resolve, rejects expired and missing reports, computes the
    voter's geohash cell from raw coordinates on the server, appends the vote, and returns the
    visibility `Resolve` computes over the tally read back afterwards — every branch proven against
    a recording fake with no database.
  </done>
</task>

</tasks>

## Artifacts this phase produces

Every symbol and file plan 02-03a creates or changes. 02-03b, 02-04, 02-05, 02-06 and 02-07 consume
these by these exact names; anything not listed here does not exist yet and must not be assumed.

**SQL query** (`internal/store/queries/votes.sql`, appended)

| Query | sqlc verb | Purpose |
|-------|-----------|---------|
| `ReportVoteContext` | `:one` | severity + category + expires_at + (nullable) reporter account for one report, in one round trip |

**Generated Go symbols** (`internal/store/sqlc/votes.sql.go`)

| Symbol | Shape |
|--------|-------|
| `sqlcgen.ReportVoteContextRow` | `Severity string; Category string; ExpiresAt time.Time; ReporterAccountID *int64` |
| `func (q *Queries) ReportVoteContext(ctx context.Context, reportID int64) (ReportVoteContextRow, error)` | single scalar arg, not a Params struct |

**Supersession (read this before checking any artifact table against 02-02 or 02-PATTERNS.md).**
`ReportVoteContext` **supersedes** the `ReporterAccountID` query that 02-02's "Deliberately NOT
produced here" section, `02-PATTERNS.md` and `02-RESEARCH.md`'s Pattern 3 all anticipated 02-03
would add. It returns the same `reporter_account_id` plus the `severity`, `category` and
`expires_at` that `CastVote` needs on the same call — one round trip instead of two, and no window
in which a report can expire or vanish between the reporter lookup and the meta read. A query named
`ReporterAccountID` is deliberately **not** created; its absence is intentional, not a gap. The
join stops at `sessions` because `sessions.account_id` *is* `accounts.id` by foreign key, so a
third join through `accounts` could only re-prove what the constraint already guarantees.

**New exported symbols in package `service`** (`internal/service/trust.go`)

| Symbol | Kind | Signature / value |
|--------|------|-------------------|
| `VoteKind` | type | `string` |
| `VoteKindContent` | const | `VoteKind` = `"content"` |
| `VoteKindResolution` | const | `VoteKind` = `"resolution"` |
| `VoteKinds` | var | `[]VoteKind` — ordered: content, resolution |
| `(VoteKind) Valid` | method | `func (k VoteKind) Valid() bool` |
| `VoteValue` | type | `string` |
| `VoteConfirm` | const | `VoteValue` = `"confirm"` |
| `VoteDispute` | const | `VoteValue` = `"dispute"` |
| `VoteResolve` | const | `VoteValue` = `"resolve"` |
| `VoteReopen` | const | `VoteValue` = `"reopen"` |
| `VoteValues` | var | `[]VoteValue` — all four, in the order above |
| `(VoteValue) ValidFor` | method | `func (v VoteValue) ValidFor(k VoteKind) bool` |
| `ErrCannotVoteOwnReport` | var | `error` — D-03; 02-03b maps it to 403 |
| `ErrReportExpired` | var | `error` — 02-03b maps it to 409 |
| `ErrReportNotFound` | var | `error` — 02-03b maps it to 404 |
| `BuildVoteTally` | func | `func BuildVoteTally(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) VoteTally` |
| `VotingQuerier` | interface | `InsertVote`, `CurrentVotesForReports`, `ReportVoteContext` — exactly three methods |
| `VotingService` | struct | unexported field `q VotingQuerier` |
| `NewVotingService` | func | `func NewVotingService(q VotingQuerier) *VotingService` |
| `CastVoteInput` | struct | `ReportID int64; AccountID int64; Kind VoteKind; Value VoteValue; Latitude float64; Longitude float64` |
| `CastVoteResult` | struct | `Visibility Visibility; Reason ResolveReason` |
| `(*VotingService) CastVote` | method | `func (s *VotingService) CastVote(ctx context.Context, in CastVoteInput) (CastVoteResult, error)` |

`BuildVoteTally`'s `reportID` parameter is not incidental: 02-04 loads votes for a whole page of
reports with ONE `CurrentVotesForReports` call and then calls `BuildVoteTally` once per report over
that shared slice. Removing the parameter would force a per-report query and reintroduce the N+1
the batched query exists to avoid.

**New unexported symbols in package `service`**

| Symbol | Signature / value |
|--------|-------------------|
| `voterGeohashPrecision` | untyped int `7` — deliberately distinct from `report.go`'s `geohashPrecision = 8` |
| `independentCellCount` | `func independentCellCount(cells []string) int` |

**New tests** (`internal/service/trust_test.go`, **`package service`** — the internal test package,
because `independentCellCount` is unexported and `02-VALIDATION.md` names `TestIndependentCellCount`
directly; `report_test.go` and `auth_test.go` remain `package service_test` and both packages
coexist in the directory)

`TestVoteKindAndValueValidation` · `TestIndependentCellCount` · `TestBuildVoteTally` ·
`TestBuildVoteTallyExcludesReporterOwnContentVote` · `TestBuildVoteTallyFeedsResolveEndToEnd` ·
`TestCastVoteRejectsReporterContentVote` · `TestCastVoteAllowsReporterResolutionVote` ·
`TestCastVoteAllowsNonReporter` · `TestCastVoteTreatsNullReporterAsNobody` ·
`TestCastVoteRejectsExpiredReport` · `TestCastVoteRejectsMissingReport` ·
`TestCastVoteComputesGeohashCellServerSide` · `TestCastVoteValidatesInput` ·
`TestCastVoteReturnsFreshlyResolvedVisibility`

**Deliberately NOT produced here** (so a drift check does not flag these as missing)

- `internal/api/handlers/votes.go`, `CastVoteRequest`, `CastVoteResponse`, `handlers.CastVote`, the
  four routes, `Deps.Votes`, `cmd/server/main.go` wiring, `votes_e2e_test.go`, and the
  `swag init` regeneration — all 02-03b (wave 3).
- `ReporterAccountID` as a standalone query — superseded, see above.
- Any change to `reports`, any cached counter or status column, any `web/static/js` file, any
  template, any `?show_disputed=true` handling — 02-04 and later.

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| verified caller → vote decision | The caller supplies a report id and raw coordinates. Both influence whether a possibly-real emergency stays visible. This plan is where those values stop being claims and become validated, server-derived facts. |
| service layer → store layer | `VotingService` is the only writer of `votes` rows in the tree. The store performs no validation of its own (02-02 says so explicitly), so every closed-enum check and every identity check has to happen here or nowhere. |
| vote data → resolver | The tally this plan builds is the sole input `Resolve` sees. A miscounted cell is indistinguishable, downstream, from a real corroboration. |

No client/untrusted input crosses an HTTP boundary inside this plan — there is no handler here.
No package-manager install occurs (`02-RESEARCH.md`'s Package Legitimacy Audit records that this
phase introduces no new external package), so no `T-02-SC` supply-chain row applies.

## STRIDE Threat Register

Threat IDs are reused verbatim from `02-VALIDATION.md` / `02-RESEARCH.md`'s Security Domain — this
plan mints no new IDs. ASVS level 1, block-on: high.

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-05 | Tampering (of a report's own trust score) | `VotingService.CastVote` step 3 — reporter self-confirming their own report | high | mitigate | Two layers, both server-side. (a) `CastVote` compares the caller's `AccountID` against the `reporter_account_id` that `ReportVoteContext` resolves through `reports` → `sessions`, and returns `ErrCannotVoteOwnReport` **before** `InsertVote` runs — `TestCastVoteRejectsReporterContentVote` asserts both the sentinel error and that zero insert calls were recorded, so rejecting-after-writing cannot pass. The hidden button 02-05 renders is a courtesy, never the control. (b) `BuildVoteTally` additionally drops any content row belonging to the reporter account at read time, covering rows that predate the check; `TestBuildVoteTallyExcludesReporterOwnContentVote` asserts it. The block is deliberately scoped to `VoteKindContent` so D-13's reporter-instant resolve keeps working, and `TestCastVoteAllowsReporterResolutionVote` fails if that scoping is widened. A NULL reporter account (a pre-Phase-1.1 report) never compares equal to a caller — `TestCastVoteTreatsNullReporterAsNobody`. |
| T-02-01 | Spoofing | `voterGeohashPrecision` / `independentCellCount` — Sybil or multi-account vote stuffing from one physical location | high | mitigate | The independence predicate is server-computed end to end and stacks on Phase 1.1's mandatory email verification. The cell is derived by `geohash.EncodeWithPrecision(lat, lon, 7)` inside `CastVote`; `CastVoteInput` has no geohash field, so there is no channel for a forged cell (structurally asserted by Task 3's six-field criterion, not only by comment). `independentCellCount` counts DISTINCT cells over an already-per-account-deduped read, so N accounts standing in one place count once — `TestBuildVoteTally`'s identical-cell case and `TestCastVoteReturnsFreshlyResolvedVisibility`'s same-cell case both fail if counting reverts to raw rows. **Residual risk, accepted and documented:** GPS spoofing via devtools override or a mock-location app is not preventable at this project's budget (`PITFALLS.md` Pitfall 4). Mitigated, not solved — no UI copy in this phase may claim the mechanic is fraud-proof. |
| T-02-03 | Elevation of Privilege | `CastVote` + `BuildVoteTally` — unauthorised resolve/reopen by a single non-reporter account, **write/tally half** | high | mitigate | A non-reporter's `resolution` vote is recorded as one cell among many and never short-circuits anything: `BuildVoteTally` routes it into `ResolveCells`/`ReopenCells`, which `Resolve` gates on `IndependentAgreementThreshold` (D-14). Only the report's own reporter account sets `ReporterResolved`, and the reporter is identified by the same `ReportVoteContext` join the D-03 block uses — never by a client assertion. The reporter gets no reopen channel at all (D-16's locked resolution): a reporter `resolution`/`reopen` row sets the flag false and is dropped rather than counted. **Scope note:** 02-01 mitigates the decision half (`isRetracted`'s two admitted paths) and this plan mitigates the write/tally half; the two together complete T-02-03. |

**Threats owned by other plans** (listed so the gap is explicit rather than silent): T-02-02
(concurrency race on the tally) → 02-02's append-only, no-shared-counter schema and its
`TestCastVoteConcurrentSameAccountKeepsEveryRow` proof; this plan adds no transaction, lock or
conflict clause that could reintroduce it. T-02-04 (resolver bypass from a read path) → 02-01's
pure `Resolve` plus 02-04's feed work; this plan's contribution is negative and load-bearing —
`CastVote` calls `Resolve` rather than deciding visibility itself, so the cast response cannot
drift from the feed.

`security_asvs_level: 1`, `security_block_on: high` — all three threats above are dispositioned
`mitigate`, never `accept`.
</threat_model>

<verification>
Run in order, from the repository root.

1. `make sqlc` — exits 0, and regeneration is idempotent (running it twice leaves `git status`
   clean the second time).
2. `gofmt -l internal/service/ internal/store/` prints nothing; `go build ./... && go vet ./...`
   both exit 0.
3. `go test ./... -short` — exits 0. Every test this plan adds is a pure unit test over fabricated
   values and runs here; nothing in this plan needs `DATABASE_URL`.
4. With `DATABASE_URL` set: `go test ./... -v -p 1` — the full suite is green, including every
   02-02 vote test and every Phase 1 / 1.1 test against a database carrying the regenerated query
   layer. `-p 1` is mandatory (shared database, per-test `TRUNCATE`).
5. `02-VALIDATION.md`'s own commands for this plan's requirements run green:
   - TRUST-01: `go test ./internal/service/... ./internal/store/... -run TestCastVote -p 1`
   - TRUST-03: `go test ./internal/service/... -run TestIndependentCellCount`
6. Confirm `git status` shows `internal/store/sqlc/votes.sql.go`, `internal/store/sqlc/models.go`
   and `internal/store/sqlc/db.go` staged — generated output is committed on purpose in this repo
   and CI has no sqlc binary.
7. Confirm 02-02's source guard still fires on the appended query file:
   `go test ./internal/store/... -short -run TestVotesQuerySourceHasNoUpsertOrLock -v` reports PASS,
   not SKIP.
8. Feedback latency — the whole `./internal/service/` package completes well under
   `02-VALIDATION.md`'s 60-second maximum.

Note, not a step for this executor: `02-VALIDATION.md`'s Per-Task Verification Map rows map to this
plan's tasks (`02-03a-01` … `02-03a-03`, wave 2). That table is backfilled phase-wide once all
PLAN.md files exist (`02-PLAN-OUTLINE.md` Open Question 3), not by this plan — several executors
making scoped edits to one shared table would clobber each other. `02-VALIDATION.md` is
deliberately absent from `files_modified`.
</verification>

<success_criteria>
- `ReportVoteContext` returns a report's severity, category, expiry and nullable reporter account in
  one round trip, and a report whose session has no bound account yields a NULL reporter rather
  than zero rows.
- A content vote from the report's own reporter is rejected with `ErrCannotVoteOwnReport` and no
  row is written (D-03, T-02-05).
- A resolution vote from that same reporter is accepted and, with the reporter's resolve row in the
  current-vote read, resolves the report to `VisibilityRetracted` (D-13, TRUST-08).
- The voter's `geohash_cell` is computed server-side at `voterGeohashPrecision = 7` from raw
  coordinates, and `CastVoteInput` has no geohash field for a client to populate (D-17, T-02-01).
- A vote on an expired report is rejected with `ErrReportExpired`, and a vote on a nonexistent
  report with `ErrReportNotFound`; neither writes a row.
- Two confirm votes from two distinct accounts in the same geohash cell leave the report
  `VisibilityProvisional`; moving one voter to a distinct cell flips it to `VisibilityLive`
  (TRUST-03, TRUST-04).
- `independentCellCount` exists exactly once and is reached by all four cell counts through
  `BuildVoteTally` (D-14).
- `BuildVoteTally(rows, reportID, reporterAccountID)` filters by report id, so 02-04 can call it
  per report over one batched read without an N+1.
- `CastVote` returns the visibility `Resolve` computes over the tally read back after the insert,
  never a locally derived one (D-04, TRUST-02, T-02-04).
- `internal/service/trust.go` imports no HTTP or routing package, and this plan changes no file
  under `internal/api/`, `cmd/`, `web/` or `docs/`.
- `go test ./... -short` is green, and `go test ./... -p 1` is green with `DATABASE_URL` set.
</success_criteria>

<output>
Create `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-03a-SUMMARY.md` when
done.

Record in it: the generated `ReportVoteContext` signature and `ReportVoteContextRow` field list; the
final `VotingQuerier` interface exactly as declared (02-03b and 02-04 both write fakes against it);
`CastVoteInput`/`CastVoteResult`'s field lists; the three sentinel errors paired with the status
codes 02-03b must map them to (403 / 409 / 404); and an explicit note that `ReportVoteContext`
supersedes the `ReporterAccountID` query 02-02's artifact table anticipated, so a later drift check
does not read its absence as a gap.
</output>
