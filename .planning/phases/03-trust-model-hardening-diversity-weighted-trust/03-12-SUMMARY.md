---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 12
subsystem: api
tags: [go, trust-model, reporter-reliability, batching, service-layer]

requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-05 BuildTrust and TrustInput.Reporter; 03-06 the ReporterHistoryReports query; 03-10 ReporterSignals and the none signal rule"
provides:
  - "Nearby, CastVote and ActivityForAccount each read the reporters' bounded history once, keyed by verified account, and set TrustInput.Reporter"
  - "Trust.Reporter is real and deeply equal across the three read paths for the same data"
  - "MaxReporterAccountsPerRead = 200 caps a feed read, nearest reports first; reports past the cap read none and stay listed"
affects: [03-14 wire mapping of Trust.Reporter, 03-13, Phase 3.1 history labels]

tech-stack:
  added: []
  patterns:
    - "One history read and one vote read per request; the history report ids join the single CurrentVotesForReports call through voteReadIDs"
    - "Signals looked up by account (reporterSignalRef), nil meaning the none signal"
    - "Votes grouped once per request (voteRowsByReport), per report work reads only its own rows"

key-files:
  created: []
  modified:
    - internal/service/report.go
    - internal/service/trust.go
    - internal/service/auth.go
    - internal/service/feed_test.go
    - internal/service/trust_test.go
    - internal/service/auth_test.go
    - internal/service/trustview_test.go
    - internal/service/trust_paths_test.go

key-decisions:
  - "CastVote reads history after every refusal and the cell computation and before InsertVote, so a failed read writes nothing and a refused vote reads no history (03-01 refusal call logs unchanged)"
  - "Nearby and ActivityForAccount capture now before the batched reads, because the history read takes it and ReporterSignals must see the same clock (R-05)"
  - "pageReporterAccounts stops at MaxReporterAccountsPerRead new accounts but an already collected account still maps to its signal on a farther report"
  - "The three fakes filter history by AccountIds like the real query and return an empty result when unconfigured"

patterns-established:
  - "reporterHistoryParams is the single place the 90 day window and the 30 row cap reach the query"
  - "A signal is attached through reporterSignalRef, never by building a Trust or ReporterSignal literal in a read path"

requirements-completed: [TRUST-07, D-10, D-13, D-14, R-05]

coverage:
  - id: D1
    description: "Every read path loads the reporter history once, keyed by verified account, and carries the real reporter signal"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestReporterSignalKeyedByAccount"
        status: pass
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteReadsReporterHistoryOnce"
        status: pass
      - kind: unit
        ref: "internal/service/auth_test.go#TestActivityForAccountReadsReporterHistoryOnce"
        status: pass
    human_judgment: false
  - id: D2
    description: "The three paths return deeply equal trust objects, including Reporter, for the same data"
    requirement: "TRUST-07"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestCastVoteTrustMatchesNearbyTrust"
        status: pass
      - kind: unit
        ref: "internal/service/trust_paths_test.go#TestTrustAttachedOnEveryReadPath"
        status: pass
    human_judgment: false
  - id: D3
    description: "A feed read makes one reporter, at most one history and one vote read, and rates at most 200 accounts nearest first"
    requirement: "TRUST-07"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyBatchesVoteReadsOnceWithReporters"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyReporterAccountCap"
        status: pass
    human_judgment: false
  - id: D5
    description: "History is requested with the 90 day window, the 30 row cap and the injected clock, and a report expiring exactly at now counts as judged"
    requirement: "R-05"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestReporterSignalKeyedByAccount"
        status: pass
      - kind: unit
        ref: "internal/service/trust_test.go#TestCastVoteReadsReporterHistoryOnce"
        status: pass
    human_judgment: false
  - id: D4
    description: "The rating is display only: changing it changes nothing but Trust.Reporter"
    requirement: "D-14"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyRatingNeverChangesVisibilityOrOrder"
        status: pass
    human_judgment: false

duration: about 45min (estimated)
completed: 2026-10-08
status: complete
---

# Phase 3 Plan 12: Reporter history on every read path Summary

**Nearby, CastVote and ActivityForAccount now read the reporters' bounded history once by verified account, fold its ids into the one vote read, and carry the real reporter signal, with a 200 account cap on feed reads.**

## Performance

- **Duration:** about 45 min
- **Completed:** 2026-10-08
- **Tasks:** 2 of 2
- **Files modified:** 8 (all under internal/service)

## Accomplishments

- `Querier`, `VotingQuerier` and `AuthQuerier` gained `ReporterHistoryReports`, and the three fakes gained it too (filtered by `AccountIds`, empty when unconfigured, so `p05VotingFake` gets it by promotion).
- `report.go` holds the shared helpers `reporterHistoryParams`, `voteReadIDs`, `pageReporterAccounts`, `reporterSignalRef` and `voteRowsByReport`. `Nearby` runs: page, ids, one `now`, reporters, accounts, history (only when there is an account), one vote read over `voteReadIDs`, one `ReporterSignals`, one grouping, then per report `BuildTrust` with `Reporter: reporterSignalRef(signals, reporterID)`.
- `CastVote` reads the reporter's history once after the cell computation and before `InsertVote`; `ActivityForAccount` reads the account's history once after capturing `now`.
- `MaxReporterAccountsPerRead = 200` bounds a feed read. The over cap report keeps its place and reads none; a farther report by an account inside the cap keeps that account's signal.
- The rating never feeds `Resolve`, `ListableInFeed`, append order or the `IncludeDisputed` partition (D-14), proven by swapping one reporter between New, Reliable (86), Mixed (64) and Unreliable (32) in both views.

## Task Commits

1. Task 1 RED: `ed94fdc` test(03-12): add failing tests for the reporter history read on every path
2. Task 1 GREEN: `804e3e6` feat(03-12): load reporter history on every read path and carry the reporter signal
3. Task 2 RED: `e87c737` test(03-12): add failing tests for the reporter account cap and the display only rating
4. Task 2 GREEN: `3b2989e` feat(03-12): cap a feed read at 200 reporter accounts, nearest first

## Files Created/Modified

- `internal/service/report.go` - interface method, five helpers, `MaxReporterAccountsPerRead`, rewritten `Nearby` and its numbered doc steps
- `internal/service/trust.go` - interface method, `CastVote` history read and `Reporter` on the trust object, doc step 7 extended
- `internal/service/auth.go` - interface method, `ActivityForAccount` history read, signals on every row
- `internal/service/feed_test.go` - `fakeFeedQuerier.ReporterHistoryReports`, `p12` fixtures, `TestNearbyBatchesVoteReadsOnceWithReporters`, `TestReporterSignalKeyedByAccount`, `TestCastVoteTrustMatchesNearbyTrust`, `TestNearbyReporterAccountCap`, `TestNearbyRatingNeverChangesVisibilityOrOrder`
- `internal/service/trust_test.go` - `fakeVotingQuerier.ReporterHistoryReports`, `currentVotesIDs`, `p12CastNow`, `p12CastHistory`, `TestCastVoteReadsReporterHistoryOnce`
- `internal/service/auth_test.go` - `fakeAuthQuerier.ReporterHistoryReports`, `votesArg`, `TestActivityForAccountReadsReporterHistoryOnce`
- `internal/service/trustview_test.go`, `internal/service/trust_paths_test.go` - expected values only (see below)

## Notes for later plans

- `trustview_test.go` and `trust_paths_test.go` were edited only to add the reporter signal to the expected values of `TestCastVoteResultCarriesTrust` and `TestTrustAttachedOnEveryReadPath`, under the outline's file ownership rule, because the read paths now carry account 7's New reporter signal where 03-10 left none. Neither test was renamed, moved or duplicated.
- For 03-14: `Trust.Reporter` is real on all three paths and only needs mapping to the wire. The vote path reads history one request after the feed, so a feed versus vote equality test must not seed a history report whose `expires_at` falls between the two reads (Pitfall 8).
- The `CastVote` history read sits after every refusal and before `InsertVote`, so a failed read writes nothing and 03-01's refusal call log `["ReportVoteContext"]` still holds (asserted again by `TestCastVoteReadsReporterHistoryOnce`).

## Verification

- `gofmt -l internal/service`: empty. `go vet ./...`: clean.
- `go test ./internal/service/ ./web/ -short -count=1`: `ok pinalert/internal/service 1.715s`, `ok pinalert/web 0.724s`.
- All nine named tests pass: `TestNearbyBatchesVoteReadsOnceWithReporters`, `TestReporterSignalKeyedByAccount`, `TestCastVoteTrustMatchesNearbyTrust`, `TestCastVoteReadsReporterHistoryOnce`, `TestActivityForAccountReadsReporterHistoryOnce`, `TestNearbyReporterAccountCap`, `TestNearbyRatingNeverChangesVisibilityOrOrder`, `TestTrustAttachedOnEveryReadPath`, `TestCastVoteResultCarriesTrust`.
- Structural checks from the plan's verify commands (run from a script file, see below): each read path body has exactly one of `s.q.ReporterHistoryReports(`, `reporterHistoryParams(`, `voteReadIDs(`, `ReporterSignals(`, `reporterSignalRef(`, `s.q.CurrentVotesForReports(`, `BuildTrust(TrustInput{` and `s.now().UTC()`; source order holds in all three files; 0 `Trust{` or `ReporterSignal{` literals and 0 reliability constants in the three service files; 0 dashed lines in the checked comments; `const MaxReporterAccountsPerRead = 200` declared once.
- Whole suite on a fresh `p3_0312` with `-v`: `rc=0`, `ok` for internal/api, internal/api/handlers, internal/auth, internal/geocode, internal/mailer, internal/ratelimit, internal/service, internal/session, internal/store, internal/testutil and web, 0 `--- SKIP` lines. `p3_0312` was dropped afterwards (not listed by `psql -l`).

## Deviations from Plan

### Auto-fixed Issues

None. No bug fixes or missing functionality were found.

### Minor departures

1. **[Doc wording]** The first prose paragraph of `Nearby`'s doc comment said the page's votes and reporters are loaded "in two batched calls", which would now be false. I reworded that one sentence to name the reporters, the history and the votes in batched calls. The first line still starts `// Nearby runs`, and the numbered steps were rewritten as the plan asks.
2. **[Verify command substitutions]** The sandbox refused compound one-liners, so the structural checks and the closing full suite run were executed from two small scripts in the session scratchpad instead of inline. The script runs the plan's own create, test, drop form (`createdb p3_0312`, `go test ./... -count=1 -p 1`, `rc=$?`, `dropdb`), with `-v` added so skipped tests could be counted. `DATABASE_URL` was unset in the shell, so `env -u DATABASE_URL` was dropped from the no database runs (`-short` was used instead where a database could be reached).
3. **[Extra test helpers]** Besides the helpers the plan names, `feed_test.go` also holds `p12VoteContext`, `p12ActivityRow`, `p12Clock` and `p12ReporterOf` (all `p12` prefixed). `auth_test.go` and `trust_test.go` gained `reflect` and `slices` imports.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or schema change; account ids are only passed to the store and used as map keys, and `Trust` and `ReporterSignal` gained no field.

## Self-Check: PASSED

- Files: all eight modified files exist, and this SUMMARY.md exists.
- Commits: `ed94fdc`, `804e3e6`, `e87c737`, `3b2989e` are all present on `worktree-agent-aae6b1939eb71533c`.
- `git diff --stat 472017c HEAD` lists exactly the eight `files_modified` entries; no STATE.md, ROADMAP.md or REQUIREMENTS.md change.
