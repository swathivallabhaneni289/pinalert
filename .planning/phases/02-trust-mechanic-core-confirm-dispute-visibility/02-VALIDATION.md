---
phase: 2
slug: trust-mechanic-core-confirm-dispute-visibility
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-12
updated: 2026-09-15
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| **Config file** | none — `make test` / `make test-short` wrap plain `go test` invocations |
| **Quick run command** | `go test ./... -short` (skips every test needing `DATABASE_URL`) |
| **Full suite command** | `go test ./... -v -p 1` (requires `DATABASE_URL`; `-p 1` is required — tests share one Postgres and each calls `testutil.NewTestDB`'s `TRUNCATE ... RESTART IDENTITY`) |
| **Estimated runtime** | ~30-60 seconds (full suite, single worker against shared Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short`
- **After every plan wave:** Run `go test ./... -v -p 1` (requires `DATABASE_URL`)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-02 T1-3, 02-03a T1-3, 02-03b T1-3, 02-05 T1-3 | 02-02, 02-03a, 02-03b, 02-05 | 1, 2, 3, 5 | TRUST-01 | T-02-05 | Confirm/dispute cast (store→service→HTTP→client), stored, reflected in tally | unit + integration + e2e | `go test ./internal/store/... -run TestCastVoteConcurrent && go test ./internal/service/... -run TestCastVote && go test ./internal/api/handlers/... -run TestCastVote` | ❌ pending execution | ⬜ pending |
| 02-01 T1-3, 02-04 T1-3 | 02-01, 02-04 | 1, 4 | TRUST-02 | T-02-04 | `Resolve()` gives identical output regardless of caller; feed/map both call it, never re-derive | unit (table-driven) + e2e | `go test ./internal/service/ -run TestResolve && go test ./internal/api/handlers/... -run TestFeedVisibilityMatchesCastVoteResponse` | ❌ pending execution | ⬜ pending |
| 02-03a T1-3, 02-05 T1-3 | 02-03a, 02-05 | 2, 5 | TRUST-03 | T-02-01 | Only distinct-account + distinct-cell votes count; GPS-denial hard-blocks the vote client-side | unit + frontend contract | `go test ./internal/service/ -run TestIndependentCellCount && go test ./web/ -run TestVoteTransportHasNoLocationFallback` | ❌ pending execution | ⬜ pending |
| 02-01 T1-3, 02-04 T1-3, 02-06 T1-3 | 02-01, 02-04, 02-06 | 1, 4, 6 | TRUST-04 | — | Non-critical Provisional until 2nd independent confirm; critical instant; dimmed+labelled client-side | unit + e2e + frontend contract | `go test ./internal/service/ -run TestResolve_ProvisionalGate && go test ./internal/api/handlers/... -run TestShowDisputedRevealsHiddenReports && go test ./web/ -run TestVisibilityCascadeOverridesAgeRamp` | ❌ pending execution | ⬜ pending |
| 02-01 T1-3, 02-04 T1-3 | 02-01, 02-04 | 1, 4 | TRUST-06 | — | Severity affects triage sort only, never bypasses the gate for non-critical | unit + unit | `go test ./internal/service/ -run TestResolve_SeverityNeverBypassesGateAlone && go test ./internal/service/... -run TestNearbySeverityDoesNotChangeGatingAmongNonCritical` | ❌ pending execution | ⬜ pending |
| 02-03a T1-3, 02-03b T1-3, 02-07 T1-3 | 02-03a, 02-03b, 02-07 | 2, 3, 7 | TRUST-08 | T-02-03 | Reporter-instant-resolve AND reporter-instant-reopen (symmetric, D-13/D-16 amended 2026-09-15); a non-reporter's resolve or reopen still needs independent agreement; `ReporterReopened` is set only from a server-side identity comparison, never client input | unit + e2e + frontend contract | `go test ./internal/service/ -run TestCastVoteAllowsReporterResolutionVote && go test ./internal/service/ -run TestBuildVoteTallyOnlyReporterSetsReporterReopened && go test ./internal/api/handlers/... -run TestReporterCanResolveOwnReportInstantly && go test ./internal/api/handlers/... -run TestReopenRequiresIndependentAgreement && go test ./internal/api/handlers/... -run TestProfileReopenIsInstantForTheReporter` | ❌ pending execution | ⬜ pending |
| 02-02 T1-3 | 02-02 | 1 | TRUST-09 | T-02-02 | N concurrent votes on one report never silently drop one | integration/concurrency | `go test ./internal/store/... -run TestCastVoteConcurrent -p 1` | ❌ pending execution | ⬜ pending |

*Backfilled 2026-09-15 once all 8 PLAN.md files existed (02-03 split into 02-03a/02-03b during detailed planning, shifting waves 4-7 by one from this file's original draft). Each cell lists every plan that contributes to that requirement's proof — several requirements span the full store→service→HTTP→client chain, so no single plan owns them alone. "File Exists" reflects pre-execution state: every test file is created within its own plan's tasks (TDD-style RED→GREEN→Harden, per 02-01's pattern), not via a separate Wave 0 pass — there is nothing to scaffold ahead of time here. Threat Refs (T-02-01..05) correspond to the Security Domain threats below, in the same order listed there. Full per-plan test-name lists live in each PLAN.md's own "Artifacts this phase produces" section.*

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Superseded — no separate Wave 0 pass needed.* Each item below turned out to be created inside its
own plan's own tasks (TDD-style RED→GREEN→Harden), not a shared pre-planning scaffold:

- [x] `internal/service/visibility_test.go` — table-driven `Resolve()` tests covering: critical
      bypass over Hidden-by-dispute, critical bypass over Provisional, Hidden reversibility,
      Retracted precedence, reopen. → **02-01, Task 1** (written before Task 2's implementation).
- [x] `internal/service/trust_test.go` — `independentCellCount()` and `BuildVoteTally()` tests.
      → **02-03a** (package `service`, alongside the exported `VotingService`/`CastVote` symbols).
- [x] `internal/store/votes_test.go` — `TestCastVoteConcurrent`, `TestCurrentVoteIsLatestOnAccountChange`,
      `TestVotesArePerReportPerAccountKind`, plus two additional gates the plan added
      (`TestVotesHaveNoUniqueKeyBeyondPrimaryKey`, `TestVotesQuerySourceHasNoUpsertOrLock`).
      → **02-02**.
- [x] `internal/api/handlers/votes_e2e_test.go` — end-to-end for the 403 on reporter self-vote and
      the 401 on an unverified caller. → **02-03b** (plus `TestReopenRequiresIndependentAgreement`,
      added after a coverage gap was caught on review).

*Framework install: none — `go test` is already the only tool this repo uses.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| GPS-denial blocks the vote (D-18) end-to-end in a real browser | TRUST-03 (supporting UX) | Browser geolocation permission prompts cannot be driven by `go test`; the deny path is a client-side branch in vanilla JS, not server logic | Open the report feed in a browser, deny the location permission prompt when confirming/disputing, confirm the vote is blocked client-side with a clear message rather than silently submitted |
| "Show disputed" filter and dimmed+labeled Provisional styling render as designed (D-09/D-10/D-11) | TRUST-02, TRUST-04 | Visual/CSS rendering — not asserted by Go tests | Load the feed and map with a Hidden report present, toggle "Show disputed", confirm pins/rows appear grayed/outlined per D-11 and Provisional rows show both dimming and the text label per D-09 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies — confirmed by gsd-plan-checker
      (2026-09-15): 3 `<automated>` blocks per plan (11 for 02-07), zero missing
- [x] Sampling continuity: no 3 consecutive tasks without automated verify — confirmed, holds
- [x] Wave 0 covers all MISSING references — superseded: no separate Wave 0 needed, every test is
      created inside its owning plan's own tasks (see "Wave 0 Requirements" section above)
- [x] No watch-mode flags — confirmed, zero found across all 8 plans
- [x] Feedback latency < 60s — confirmed, full suite ~30-60s against shared Postgres
- [x] `nyquist_compliant: true` set in frontmatter — done

**Approval:** approved 2026-09-15 (gsd-plan-checker VERIFICATION, Dimension 8/Nyquist: PASS)
