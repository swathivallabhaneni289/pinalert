---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: 01
subsystem: trust-mechanic
tags: [go, tdd, pure-function, visibility, trust-model]

requires:
  - phase: 01-foundation-report-submission-nearby-feed
    provides: internal/service package conventions (Severity/Category enum shape, doc-comment-cites-decision-IDs style, prose-subtest t.Run convention)
provides:
  - "Resolve(meta, tally, now) (Visibility, ResolveReason) — the single exported authority for a report's visibility state, pure and DB-free"
  - "Visibility (four states) and ResolveReason (five closed slugs) types + constants"
  - "ReportMeta (Severity, Category) — the smallest subset Resolve needs"
  - "VoteTally (six fields, symmetric ReporterResolved/ReporterReopened pair) — the contract 02-03a's BuildVoteTally() must populate"
  - "IndependentAgreementThreshold = 2 — the one shared independence number for Hidden floor, Provisional gate, confirmer-resolve, and non-reporter reopen"
affects: [02-03a, 02-03b, 02-04, 02-06, 02-07]

tech-stack:
  added: []
  patterns:
    - "Pure resolver function: Resolve() takes only plain Go values (no context.Context, no I/O, no package-level mutable state) so every future read path can call it with zero coupling excuse"
    - "Symmetric reporter-instant boolean pair: ReporterResolved/ReporterReopened mirror each other field-for-field in isRetracted() — dropping either term is a visible, greppable asymmetry"
    - "TDD RED-GREEN-REFACTOR with an exhaustive matrix test as the totality/determinism proof, not just example-based table tests"

key-files:
  created:
    - internal/service/visibility.go
    - internal/service/visibility_test.go
  modified: []

key-decisions:
  - "Test file uses package service_test (external test package), not package service as the plan's prose literally stated — the plan's own behavior spec requires qualified references like service.IndependentAgreementThreshold throughout every row, which only compiles in an external test package; report_test.go (the plan's own cited convention target) is itself package service_test. Resolved as a Rule 1 bug-fix (self-contradictory plan text), not an architectural deviation — no production code or test semantics changed."

patterns-established:
  - "Decision ladder as an ordered if-cascade with each rung's doc comment citing its owning decision ID (D-05..D-16) — later plans reading visibility.go can trace every branch back to 02-CONTEXT.md"

requirements-completed: [TRUST-02, TRUST-04, TRUST-06]

coverage:
  - id: D1
    description: "Resolve() implements the five-rung decision ladder (Retracted > critical bypass > Hidden > Provisional gate > Live) as the sole authority for a report's visibility state"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve"
        status: pass
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve_RetractedAndReopen"
        status: pass
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve_HiddenIsReversible"
        status: pass
    human_judgment: false
  - id: D2
    description: "Provisional gate at IndependentAgreementThreshold confirm cells, with critical/rescue_needed bypassing it entirely"
    requirement: "TRUST-04"
    verification:
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve_ProvisionalGate"
        status: pass
    human_judgment: false
  - id: D3
    description: "Self-declared severity alone can never unlock full visibility (only the criticalBypass predicate can)"
    requirement: "TRUST-06"
    verification:
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve_SeverityNeverBypassesGateAlone"
        status: pass
    human_judgment: false
  - id: D4
    description: "Resolve is total, returns only closed-set reasons, is deterministic across repeat calls, and is time-invariant for Phase 2 — proven over an exhaustive ~27.6k-combination matrix, not just example rows"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "internal/service/visibility_test.go#TestResolve_IsTotalAndDeterministic"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-15
status: complete
---

# Phase 02 Plan 01: Pure Visibility Resolver Summary

**`Resolve(meta, tally, now) (Visibility, ResolveReason)` — a pure, five-rung decision ladder (Retracted > critical bypass > Hidden > Provisional > Live) proven correct over 13 named table-driven cases plus an exhaustive ~27.6k-combination totality/determinism matrix, with zero dependency beyond `time`.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-09-15 (worktree branch check)
- **Completed:** 2026-09-15T14:46:20Z
- **Tasks:** 3
- **Files modified:** 2 (both created)

## Accomplishments
- `internal/service/visibility.go`: the four-state `Visibility` type, five-slug closed `ResolveReason` set, `IndependentAgreementThreshold = 2` (the one shared independence number for four rungs), `ReportMeta`, the six-field `VoteTally` with its symmetric `ReporterResolved`/`ReporterReopened` pair, and `Resolve()` — importing nothing but the standard library `time` package.
- `internal/service/visibility_test.go`: `TestResolve`, `TestResolve_ProvisionalGate`, `TestResolve_SeverityNeverBypassesGateAlone`, `TestResolve_HiddenIsReversible`, `TestResolve_RetractedAndReopen` (13 named prose subtests total) plus `TestResolve_IsTotalAndDeterministic`'s exhaustive matrix — closing `02-VALIDATION.md`'s first Wave 0 gap.
- Full RED → GREEN → (no REFACTOR needed) TDD cycle: Task 1 landed a genuinely failing proof against a stub, Task 2 implemented the ladder and turned every case green without editing a single test, Task 3 added the totality/determinism proof as pure test additions with zero changes to `visibility.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Lock the visibility contract and land the failing table-driven proof (RED)** - `ae8836c` (test)
2. **Task 2: Implement the ordered decision ladder so every locked decision passes (GREEN)** - `d7a8a4c` (feat)
3. **Task 3: Prove the resolver is total, caller-independent and time-invariant (T-02-04)** - `f3f1cf4` (test)

_TDD plan: RED (test) → GREEN (feat) → additional test-only proof, no REFACTOR commit needed — the GREEN implementation matched the reference shape from 02-RESEARCH.md's Pattern 2 with no cleanup required._

## Files Created/Modified
- `internal/service/visibility.go` - The pure resolver: `Visibility`/`ResolveReason` types+constants, `IndependentAgreementThreshold`, `ReportMeta`, `VoteTally`, `criticalBypass()`, `isRetracted()`, `Resolve()`. Import set is exactly `time`.
- `internal/service/visibility_test.go` - Table-driven proof of every locked decision (13 named cases across 5 functions) plus the exhaustive totality/determinism/time-invariance matrix (`TestResolve_IsTotalAndDeterministic`, ~27.6k combinations).

## Final Resolve() Signature and Reason Slugs

```go
func Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, ResolveReason)
```

The five reason slugs (a frozen serialized contract — 02-04 puts these on the wire, 02-06 maps them to display copy):

| Constant | Slug |
|---|---|
| `ReasonResolved` | `"resolved"` |
| `ReasonCriticalBypass` | `"critical_bypasses_gates"` |
| `ReasonDisputed` | `"disputed"` |
| `ReasonAwaitingConfirmation` | `"awaiting_second_independent_confirmation"` |
| `ReasonConfirmed` | `"confirmed"` |

## VoteTally Six-Field Contract (for 02-03a's BuildVoteTally())

| Field | Type | Meaning |
|---|---|---|
| `ConfirmCells` | `int` | Standing independent confirm count (distinct geohash cells) |
| `DisputeCells` | `int` | Same, for disputes |
| `ResolveCells` | `int` | Standing independent resolve count, EXCLUDING the reporter's own vote |
| `ReopenCells` | `int` | Standing independent reopen count, EXCLUDING the reporter's own vote |
| `ReporterResolved` | `bool` | Reporter's own current resolution vote is resolve (D-13's instant path) |
| `ReporterReopened` | `bool` | Reporter's own current resolution vote is reopen (D-16-amended's instant path) |

**Security contract (T-02-03):** both `ReporterResolved` and `ReporterReopened` MUST be set by `BuildVoteTally()` only when the authenticated account ID equals the report's `reporter_account_id`, compared server-side — never from a client-asserted flag, request body field, or vote row's claimed identity. A client that can set `ReporterReopened` can un-retract any report in the system.

## Final isRetracted() Expression (verbatim, for 02-03a and 02-07 to assert against)

```go
resolved := t.ReporterResolved || t.ResolveCells >= IndependentAgreementThreshold
reopened := t.ReporterReopened || t.ReopenCells >= IndependentAgreementThreshold
return resolved && !reopened
```

## Decisions Made
- Used `package service_test` (external test package) for `visibility_test.go` rather than the literal `package service` the plan prose stated — see "Deviations from Plan" below.
- `02-VALIDATION.md`'s Per-Task Verification Map backfill (outline Open Question 3) is intentionally NOT touched by this plan — it's a phase-level step deferred to avoid colliding with 02-02 in the same wave (both plans run in Wave 1).
- **Flag for the phase-level `02-VALIDATION.md` backfill:** row 47 of that file's Per-Task Verification Map still describes the pre-amendment reading ("any reopen needs independent agreement, no reporter carve-out") and names a test `TestReopenRequiresIndependentAgreement` whose name now asserts only half the contract. That row belongs to 02-03a/02-03b/02-07 and was deliberately left unedited here (`02-VALIDATION.md` is outside this plan's `files_modified`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test file package changed from `service` to `service_test`**
- **Found during:** Task 1 (writing `visibility_test.go`)
- **Issue:** The plan's `<action>` prose for Task 1 literally states `visibility_test.go`: package `service` (internal test package, matching `report_test.go`'s prose-subtest convention). This is self-contradictory and would not compile: `report_test.go` — the file the plan cites as the convention to match — is itself `package service_test` (external test package, confirmed by reading it), and the plan's own `<behavior>` section requires qualified references like `service.IndependentAgreementThreshold`, `service.ReasonCriticalBypass`, `service.Severities`, etc. throughout literally every test row. A file in `package service` cannot reference its own package with a `service.` qualifier — that's a compile error, not a style choice.
- **Fix:** Wrote `internal/service/visibility_test.go` as `package service_test`, matching `report_test.go`'s actual package and satisfying every qualified reference the plan's `<behavior>` section specifies verbatim.
- **Files modified:** internal/service/visibility_test.go
- **Verification:** File compiles and `go vet ./internal/service/` exits 0; all `service.X` qualified references throughout resolve correctly.
- **Committed in:** ae8836c (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — self-contradictory plan text)
**Impact on plan:** Necessary for the file to compile at all; every locked decision, acceptance criterion, and verification command in the plan is otherwise satisfied exactly as written. No scope creep — package clause only.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. This plan is pure Go standard library only, no database, no HTTP, no new dependency.

## Next Phase Readiness
- `Resolve()`, `ReportMeta`, `VoteTally`, `Visibility`, `ResolveReason` are ready for 02-03a's `BuildVoteTally()` and `VotingService` to consume.
- The critical security note for 02-03a is recorded above and in the plan's threat model: `ReporterResolved`/`ReporterReopened` must be set only on a server-side reporter-identity match.
- No blockers. `go test ./... -short` is green; `internal/service/visibility_test.go` — the first Wave 0 gap in `02-VALIDATION.md` — is fully closed.
- Plan 02-02 (running in parallel, same wave) touches zero overlapping files with this plan (`internal/service/visibility.go`'s import set stayed exactly `time`, confirming the parallel-safety seam held).

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*

## Self-Check: PASSED

- FOUND: internal/service/visibility.go
- FOUND: internal/service/visibility_test.go
- FOUND: .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-01-SUMMARY.md
- FOUND commit: ae8836c (test — RED)
- FOUND commit: d7a8a4c (feat — GREEN)
- FOUND commit: f3f1cf4 (test — totality/determinism proof)
- FOUND commit: 08c126e (docs — plan metadata)
- Re-ran all `<acceptance_criteria>` and the plan-level `<verification>` block: gofmt clean, go vet clean, import set exactly `time`, all six TestResolve* functions green, field-name-set and symmetry greps pass, `go test ./... -short` green, files touched exactly the two declared, suite completes in well under 60s.
