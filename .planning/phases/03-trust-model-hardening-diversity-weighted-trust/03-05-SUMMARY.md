---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 05
subsystem: trust
tags: [go, trust, counters, panel-copy, injectable-clock]
requires:
  - phase: 03-01
    provides: vote distance check, ReportVoteContextRow with coordinates and CreatedAt, TooFarMessage
provides:
  - BuildTrust and the Trust object (N, M, capped numerals, lead, confirmed and disputed sentences)
  - filterVoteRows, the one routing of vote rows shared by BuildVoteTally and BuildTrust
  - Trust on ReportView, CastVoteResult and ActivityReport
  - WithReportClock, WithVotingClock, and ActivityForAccount reading s.now()
affects: [03-07, 03-08, 03-10, 03-12, 03-13, 03-15]
tech-stack:
  added: []
  patterns:
    - one row filter with two consumers
    - server authored copy built from constants and integers only
    - injectable per service clocks named apart from WithClock
key-files:
  created:
    - internal/service/trustview.go
    - internal/service/trustcopy.go
    - internal/service/trustview_test.go
    - internal/service/trustcopy_test.go
    - internal/service/trust_paths_test.go
  modified:
    - internal/service/trust.go
    - internal/service/feed.go
    - internal/service/report.go
    - internal/service/auth.go
key-decisions:
  - "BuildTrust reads filterVoteRows, so N and M equal the resolver's ConfirmCells and DisputeCells by construction (D-04, R-04)."
  - "Submit keeps the wall clock: report times stay on the clock votes and the expiry filter use."
  - "Trust has no viewer field, so the object is identical for every viewer (D-21)."
requirements-completed: [TRUST-05, D-04, D-16, D-17, D-19, D-21, R-04]
status: complete
duration: about 1 hour
completed: 2026-10-08
---

# Phase 3 Plan 05: Server built trust object Summary

One pure `BuildTrust` turns a report's batched vote rows into N, M, capped numerals and the exact panel sentences, and `Nearby`, `CastVote` and `ActivityForAccount` each attach it with a single call under an injectable clock.

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 | Vote rows to counters and panel sentences from the resolver's row filter | e674e04 |
| 2 | CastVote returns the trust object, injected clock decides expiry | b69a8c0 |
| 3 | Feed and Activity carry the same trust object for every viewer | 89ba032 |

## Accomplishments

- `filterVoteRows`, `placeVote`, `voteBuckets` and `placeVoteCells` now hold the only routing of vote rows. `BuildVoteTally` delegates to them with unchanged output; the existing `TestBuildVoteTally*` and `TestIndependentCellCount` tests pass untouched.
- `trustcopy.go` is the single home of trust copy: `99+` cap, the counts caveat, the confirmed, disputed, Provisional and expiry sentences.
- `TestBuildTrustCountsMatchTally` (V-11, 500 seeded iterations plus the routing subtest), `TestTrustCountLabel` (V-12), `TestTrustCopyIsPlainText` (V-32), `TestTrustLeadSentence`, `TestTrustCounterSentences`, `TestTrustCarriesNoIdentifiers`, `TestCastVoteResultCarriesTrust` and `TestTrustAttachedOnEveryReadPath` all pass.
- `assertPlainCopy(t, s)` is in `trustcopy_test.go` for 03-08 and 03-10 to reuse.

## Baseline counts the gates relied on

- `switch VoteKind(row.Kind)`: 1 in `trust.go` before and after (moved into `filterVoteRows`).
- Non comment wall clock calls: 1 in `trust.go` (now 0), 2 in `report.go` (1 left, in `Submit`), 1 in `auth.go` in `ActivityForAccount` (now 0).
- `BuildTrust(TrustInput{` appears once each in `trust.go`, `report.go` and `auth.go`; no hand built `Trust{` literal anywhere.

## Verification

- `gofmt -l internal/service` printed nothing; `go vet ./...` and `go build ./...` passed.
- `go test ./internal/service/ ./web/ -count=1`: `ok pinalert/internal/service`, `ok pinalert/web` (DATABASE_URL was unset).
- Full suite on the disposable `p3_0305` (created fresh, dropped after): `go test ./... -count=1 -p 1 -v` gave `ok` for api, api/handlers, auth, geocode, mailer, ratelimit, service, session, store, testutil and web; 786 `--- PASS`, 0 `--- SKIP`, no FAIL. `dropdb` confirmed the database is gone.
- Dash gates (U+2014, U+2013) are clean over the five new files, the new region and `BuildVoteTally` body in `trust.go`, `CastVote`, and every line naming Trust, Clock, Option or `s.now` in `trust.go`, `report.go`, `auth.go`, `feed.go`.
- `git diff --stat` against the base shows exactly the nine `files_modified`.

## Deviations from Plan

1. [Rule 3 - Blocked command] The sandbox refused the plan's closing gate (`(dropdb ...; createdb ... && DATABASE_URL=... go test ...; rc=$?; ...)`) as too complex to verify. I ran the same steps as separate plain commands: `dropdb --if-exists p3_0305`, `createdb p3_0305`, the `go test` with DATABASE_URL set to `p3_0305` only, then `dropdb --if-exists p3_0305`.
2. The `env -u DATABASE_URL go test` prefix in the plan's verify lines was replaced by a plain `go test`, after `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset` (standing rule 11).
3. The long Task 1 to 3 verify one-liners were split into smaller grep, awk and perl checks covering the same assertions.
4. The Task 2 test was added to `trustview_test.go` after the Task 1 commit so each commit compiles and passes on its own.

No other deviations. No STATE.md, ROADMAP.md, REQUIREMENTS.md, handler, docs, web, SQL or migration file was touched.

## Notes for plan 03-12

`p05VotingFake` embeds `*fakeFeedQuerier`, so a generated method that 03-12 adds to `VotingQuerier` reaches it by promotion. That method must return an empty result when unconfigured, on both `fakeFeedQuerier` and `fakeAuthQuerier`, never an error or a panic.

## Known Stubs

None.

## Threat Flags

None. `Trust` holds integers and server authored strings only (T-03-04, T-03-06, T-03-07 mitigations are asserted by tests).

## Self-Check: PASSED

All nine files exist; commits e674e04, b69a8c0 and 89ba032 exist on `worktree-agent-a4c0a126c46954f60`.
