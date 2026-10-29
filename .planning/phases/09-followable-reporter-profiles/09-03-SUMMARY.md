---
phase: 09-followable-reporter-profiles
plan: 03
subsystem: service
tags: [reporter-signal, seam, go]
requires: []
provides:
  - SignalSource, SignalOption, reporterSignalsWith, LoadReporterSignals, ReporterTagQuerier, ErrTooManyReporterAccounts
  - WithReportSignals, WithVotingSignals, WithAuthSignals, WithDispatchSignals
affects: [09-13, 09-14]
tech-stack:
  added: []
  patterns: [one loader for every reporter tag read, guard test on the single production caller]
key-files:
  created:
    - internal/service/reportersignals.go
    - internal/service/reportersignals_test.go
  modified:
    - internal/service/reporter.go
    - internal/service/report.go
    - internal/service/historyread.go
    - internal/service/trust.go
    - internal/service/auth.go
    - internal/service/pushdispatch.go
key-decisions:
  - "reporterIsReliable calls LoadReporterSignals alone (one history read) instead of reading history itself and then the loader, which would read it twice. A reporter with no history still reads as the none tag, so the answer is unchanged."
requirements-completed: [PROF-02, PROF-03]
status: complete
---

# Phase 9 Plan 03: One loader for every reporter tag Summary

Every production read of a reporter tag now goes through reporterSignalsWith or LoadReporterSignals, with an optional SignalSource per service (nil everywhere), so plan 09-13 can feed tapped ratings into every surface by supplying one source.

## What was built

- reporter.go: `SignalOption`, an empty `signalConfig`, and `ReporterSignals(..., now, opts ...SignalOption)`. The options are applied into the config before the loop; the loop is untouched and the 26 four argument test calls compile unchanged.
- reportersignals.go (new): `SignalSource`, `ReporterTagQuerier`, `ErrTooManyReporterAccounts`, `reporterSignalsWith` (nil source reads exactly as before, a source error fails the read wrapped with a nil map) and `LoadReporterSignals` (empty list gives an empty map without a query, more than MaxReporterAccountsPerRead is refused, otherwise history then votes over voteReadIDs(nil, history)).
- The five production calls moved behind the loader: Nearby (report.go), loadReadContext (historyread.go), CastVote (trust.go), Activity (auth.go), reporterIsReliable (pushdispatch.go). The four options sit beside each service's existing options. No option is wired in main.go or the router.
- Every behaviour added by Phases 5, 7.1 and 8 was kept (comment_count, comments_open, ViewerFollows with its fifth argument, the share area lookup, the official alerts lane); only the one call line in each file changed.

## Tests

- New in reportersignals_test.go: TestReporterSignalsOneProductionCaller, TestReporterSignalsWithNilSource, TestSignalSourceOptionsApplied, TestSignalSourceErrorFailsTheRead, TestLoadReporterSignalsBounds (extra, covers the 201 ids and empty list cases), TestLoadReporterSignalsMatchesFeed (database).
- Full run on scratch database p9_03_a: `go test ./internal/service/ ./internal/api/... -p 1 -count=1` passed (service, api, handlers). The only skip was TestScreeningFixture (allowed). The database was dropped afterwards. go build and go vet are clean.

## Deviations from Plan

**1. [Rule 1 - plan wording] reporterIsReliable reads history once.** The plan said to keep the early false for no history and call LoadReporterSignals after the history read, which would read the history twice. The loader does the whole read sequence, so reporterIsReliable is now one call. Behaviour is identical today: no history gives the none tag, which is not Reliable, and no votes query runs.

**2. Commit ordering.** The guard test lives in the task 1 file (reportersignals_test.go) as the plan places it, so at commit b9500315 TestReporterSignalsOneProductionCaller would fail; it passes from faa9f01c, when the five direct calls were removed.

**3. TestLoadReporterSignalsMatchesFeed uses `All: true`** in the Nearby query because a fresh unconfirmed low report is Provisional and the default feed does not list it.

## Commits

- b9500315 feat(09-03): add the reporter signal option, source and loader
- faa9f01c feat(09-03): read every reporter tag through the one loader

## Known Stubs

None. The signalConfig is intentionally empty until plan 09-13 adds the ratings field.

## Self-Check: PASSED
