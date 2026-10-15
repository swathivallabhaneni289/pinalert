---
phase: 04-robustness-real-world-resilience
plan: 29
subsystem: trust-model
tags: [reporter-rating, moderation, sqlc, r-02, t-04-16]
requires:
  - phase: 04-robustness-real-world-resilience
    provides: "04-01 ReportMeta.Withheld/Serious and ModerationState; 04-05 reports.moderation_state column"
provides:
  - "ReporterHistoryReports returns moderation_state as its sixth column, never filtered"
  - "ClassifyOutcome is neutral first when ReportMeta.Withheld()"
  - "reporterHistoryMeta carries the stored moderation state"
affects: [04-07, 04-16]
tech-stack:
  added: []
  patterns: ["one Withheld predicate shared by the resolver and the rating"]
key-files:
  created:
    - internal/service/moderation_rating_test.go
  modified:
    - internal/store/queries/reliability.sql
    - internal/store/sqlc/reliability.sql.go
    - internal/store/reliability_test.go
    - internal/service/reliability.go
    - internal/service/reporter.go
key-decisions:
  - "R-02 classifier rule: neutral exactly when ReportMeta.Withheld(); a pending serious report is judged as before. It is one line in ClassifyOutcome that the owner can veto to make every pending report neutral."
requirements-completed: [ROBUST-03]
status: complete
duration: about 30 minutes
completed: 2026-10-16
---

# Phase 4 Plan 29: Reporter rating neutral for withheld reports Summary

A held (pending and not serious) or removed report is now neutral for its author's reporter rating whatever its votes, because the history read carries `moderation_state` and `ClassifyOutcome` returns neutral first when `ReportMeta.Withheld()`.

## Commits

- febc321 test(04-29): expect the reporter history read to carry moderation state (RED, store)
- 9d9aed8 feat(04-29): carry moderation state on the reporter history read (query, regenerated sqlc file, new store test)
- bfcd199 test(04-29): add failing rating neutrality tests for moderated reports (RED, service, 9 failing subtests, no build error)
- b9a4d97 feat(04-29): make held and removed reports neutral for the reporter rating

## Step 0 census

- All prerequisites found: 03-03 symbols and tests, 03-06 query, row struct, helpers and four `TestReporterHistory` tests, 03-10 `reporterHistoryMeta` and `ReporterSignals`, both WR-05 subtests (once each), 04-01 `Withheld`, `Serious`, `ModerationState`, `ModerationStates`, `ReportMeta.Moderation`, 04-05 migration `00007_add_moderation_and_idempotency.sql` and `ModerationState` in `votes.sql.go`.
- `sqlc version`: v1.31.1. `internal/store/sqlc/` was clean before starting.
- `reliability.sql` holds exactly one query.
- Positional `ReporterHistoryReportsRow` literal census: none. Every literal (`p10History` and the fallback row in `reporter_test.go`, `p12History` and `hist8` in `feed_test.go`, the loop in `trust_test.go`, the empty slice in `votes_body_test.go`) is keyed or empty. `reporter_test.go` did not change.
- Non test call sites: `ScoreReliability(` only in `reporter.go` (one site, line 145). `ClassifyOutcome(` in `reporter.go` (one site, line 138) and in `historyread.go` line 292 (Phase 3.1 Past card label, does not feed `ScoreReliability`).
- Baseline counts on `reliability.go` non comment lines: `Resolve(` 1, `meta.criticalBypass()` 1. Both are 1 after the plan. In `reporter.go` after the plan: one call site each of `ClassifyOutcome(`, `ScoreReliability(`, `BuildVoteTally(`; `ModerationState(row.ModerationState)` once.

## Generated row

`ReporterHistoryReportsRow` fields: AccountID int64, ReportID int64, Severity string, Category string, ExpiresAt time.Time, ModerationState string. `make sqlc` ran once; `sqlc diff` exits 0; only `reliability.sql.go` changed among generated files. `ReporterHistoryReportsParams` is unchanged.

## R-02 rule as applied

Neutral exactly when `ReportMeta.Withheld()` (removed, or pending and not serious). A pending serious report and every clear or zero value report are judged as before. The query never filters on moderation: with a read cap of 2, the removed and the pending report come back and the older clear one is cut (`TestReporterHistoryCarriesModerationState`). The scorer drops neutral rows before its cap of 30 judged reports (`TestModeratedReportsAreNeutralForRating`: 5 corroborated plus 30 newer held reports stays Reliable 86 with 5 judged, where the same rows clear give 30 judged and 0 corroborated).

## T-04-16 residual

The SQL read cap of 100 counts held and removed rows together with the other neutral rows, so 100 neutral reports in a row can still push an old record out of the read. Mitigating points: pending is usually transient (04-14 releases it), removed needs a moderator or a late reject, a push out only drops older rows and the rating reads New when fewer than 5 judged remain, and 04-15's allowances bound posting speed. Phase 3 finding WR-05-V1 stays open as of 2026-10-08; this plan does not fix it or change the read cap.

## Test runs

- Quick: `go test ./... -count=1` passes (all packages ok).
- Database run on disposable `p4_0429` (created, then dropped): `DATABASE_URL=postgres://localhost:5432/p4_0429?sslmode=disable go test ./... -count=1 -p 1 -v` all packages ok, 0 `--- SKIP`, 0 `--- FAIL`. The five `TestReporterHistory*` tests pass there, including `TestReporterHistoryUsesSessionIndex`.
- Service run: `TestModeratedReportsAreNeutralForRating`, `TestClassifyOutcome`, `TestClassifyOutcomeMatchesResolve`, `TestOutcomeSetIsClosed`, `TestReporterSignalsGroupByAccount`, `TestReporterSignalsNewAndNone`, `TestReliability*` all pass unedited.

## Deviations from Plan

- The verify one liners use `env -u DATABASE_URL`, which the sandbox refuses; I ran plain `go test` with `DATABASE_URL` unset in the shell (verified empty) instead. No effect on results.
- `.planning/STATE.md` showed as modified in the worktree during the run (a tool side effect, not an edit of mine). It was not staged or committed and was restored to HEAD before returning.

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

All four task commits exist, the five touched files and the new test file exist, the full suite passes with 0 skips on `p4_0429`, and no added line holds U+2014 or U+2013.
