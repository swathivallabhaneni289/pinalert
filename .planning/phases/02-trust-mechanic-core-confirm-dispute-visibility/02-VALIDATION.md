---
phase: 2
slug: trust-mechanic-core-confirm-dispute-visibility
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-12
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
| 02-xx-xx | TBD | TBD | TRUST-01 | T-02-05 | Confirm/dispute cast, stored, reflected in tally | unit + integration | `go test ./internal/service/... ./internal/store/... -run TestCastVote` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-02 | T-02-04 | `Resolve()` gives identical output regardless of caller | unit (table-driven) | `go test ./internal/service/... -run TestResolve` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-03 | T-02-01 | Only distinct-account + distinct-cell votes count | unit | `go test ./internal/service/... -run TestIndependentCellCount` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-04 | — | Non-critical Provisional until 2nd independent confirm; critical instant | unit | `go test ./internal/service/... -run TestResolve_ProvisionalGate` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-06 | — | Severity affects triage sort only, never bypasses the gate for non-critical | unit | `go test ./internal/service/... -run TestResolve_SeverityNeverBypassesGateAlone` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-08 | T-02-03 | Reporter-instant-resolve; confirmer-resolve needs independent agreement | integration | `go test ./internal/store/... -run TestResolveReopen` | ❌ W0 | ⬜ pending |
| 02-xx-xx | TBD | TBD | TRUST-09 | T-02-02 | N concurrent votes on one report never silently drop one | integration/concurrency | `go test ./internal/store/... -run TestCastVoteConcurrent -p 1` | ❌ W0 | ⬜ pending |

*Task/Plan/Wave columns are placeholders — the planner fills in real IDs once PLAN.md files exist. Threat Refs (T-02-01..05) correspond to the Security Domain threats below, in the same order listed there.*

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/service/visibility_test.go` — table-driven `Resolve()` tests covering: critical
      bypass over Hidden-by-dispute, critical bypass over Provisional, Hidden reversibility
      (confirms later outweighing disputes flips back to Live/Provisional in the same test via two
      calls with different tallies), Retracted taking precedence over everything except reopen,
      reopen flipping Retracted back to the live pipeline.
- [ ] `internal/service/trust_test.go` — `independentCellCount()` unit tests (empty, all-same-cell,
      all-distinct, mixed) and `BuildVoteTally()` tests against fabricated `CurrentVotesForReports`
      rows (including the reporter's own resolution vote being excluded from `ResolveCells` and
      surfaced via `ReporterResolved` instead).
- [ ] `internal/store/votes_test.go` — mirrors `internal/store/cooldown_test.go`'s structure
      exactly: `TestCastVoteConcurrent` (N goroutines, N distinct accounts, one report, assert
      `COUNT(*) = N`), `TestCurrentVoteIsLatestOnAccountChange` (same account votes confirm then
      dispute, assert exactly one current row and it's the later one), `TestVotesArePerReportPerAccountKind`
      (a content vote and a resolution vote from the same account on the same report don't
      interfere with each other's "current" read).
- [ ] `internal/api/handlers/votes_e2e_test.go` — end-to-end for the 403 on reporter self-vote
      (D-03) and the 401 on an unverified caller (reusing the existing gated-route test pattern
      from `reports_e2e_test.go`).

*Framework install: none — `go test` is already the only tool this repo uses.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| GPS-denial blocks the vote (D-18) end-to-end in a real browser | TRUST-03 (supporting UX) | Browser geolocation permission prompts cannot be driven by `go test`; the deny path is a client-side branch in vanilla JS, not server logic | Open the report feed in a browser, deny the location permission prompt when confirming/disputing, confirm the vote is blocked client-side with a clear message rather than silently submitted |
| "Show disputed" filter and dimmed+labeled Provisional styling render as designed (D-09/D-10/D-11) | TRUST-02, TRUST-04 | Visual/CSS rendering — not asserted by Go tests | Load the feed and map with a Hidden report present, toggle "Show disputed", confirm pins/rows appear grayed/outlined per D-11 and Provisional rows show both dimming and the text label per D-09 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
