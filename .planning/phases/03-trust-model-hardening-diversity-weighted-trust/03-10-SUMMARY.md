---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 10
subsystem: trust
tags: [go, trust-model, reporter-reliability, copy, docs]

requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-03 ClassifyOutcome and ScoreReliability; 03-06 ReporterHistoryReportsRow; 03-08 Trust.Currency and the currency copy style"
provides:
  - "ReporterSignal and ReporterSignals(accountIDs, history, votes, now): pure composition of history rows and current votes into the public reporter signal"
  - "Trust.Reporter and TrustInput.Reporter, set by BuildTrust through reporterSignalFor (nil, none and invalid tags give the canonical none signal)"
  - "Reporter labels, panel sentences and score sentence in trustcopy.go (D-15, D-19, A50)"
  - "docs/TRUST-MODEL.md section Reporter reliability, including the RESOLVED Q4 cap decision"
affects: [03-12, 03-13, 03-14, 03-17]

tech-stack:
  added: []
  patterns:
    - "Composition over rules: one ClassifyOutcome call per history row, one ScoreReliability call per account, no rule constant named in reporter.go"
    - "Votes grouped by report once, so cost is history rows plus vote rows"

key-files:
  created:
    - internal/service/reporter.go
    - internal/service/reporter_test.go
  modified:
    - internal/service/trustcopy.go
    - internal/service/trustcopy_test.go
    - internal/service/trustview.go
    - docs/TRUST-MODEL.md

key-decisions:
  - "The scored account is the reporter of every one of its history reports, so its own content votes never count and its own resolve cannot erase a record"
  - "Neutral rows are passed to ScoreReliability so the window and the cap of 30 give the same answer on raw rows as on the SQL capped read"
  - "reporterSignalFor gives every trust object its own Score pointer, so a caller cannot change a built trust object through its input"

requirements-completed: [TRUST-07, D-08, D-10, D-13, D-14, D-15, D-19, D-23, R-05]

duration: 40min
completed: 2026-10-08
status: complete
---

# Phase 03 Plan 10: Reporter signal composition Summary

**`ReporterSignals` turns bounded history rows plus one batch of current votes into Reliable reporter, Mixed record, Unreliable reporter or New reporter for every requested account, and `BuildTrust` carries it as the display only `Trust.Reporter`.**

## Performance

- **Tasks:** 3 of 3
- **Files modified:** 6 (2 created)

## Accomplishments

- `reporter.go`: `ReporterSignal` (Tag, Score, Label, Sentence, ScoreSentence, JudgedReports, CorroboratedReports), `ReporterSignals`, `reporterHistoryMeta` (unknown category falls back to other, unknown severity to low), `newReporterSignal`, `noneReporterSignal`, `reporterSignalFor`. Exactly one call site each of `ClassifyOutcome(`, `ScoreReliability(` and `BuildVoteTally(`; imports are only `time` and the generated store package.
- `trustcopy.go`: four labels, `reporterLabel`, `reporterSentence` (rated form plus the three New reporter variants, the minimum taken from `ReliabilityMinJudged`), `reporterScoreSentence`. Still the single line `import "strconv"`.
- `trustview.go`: `Trust.Reporter` after `Currency`, `TrustInput.Reporter *ReporterSignal`, one `reporterSignalFor(in.Reporter)` call in `BuildTrust`. Still one `filterVoteRows(` and one `buildCurrency(`.
- `docs/TRUST-MODEL.md`: `## Reporter reliability` with the eleven pinned subsections between `## Still current signal` and `## Honest limits`.

## Commits

- 7cc3dd3 test(03-10): add failing tests for the reporter signal composition (Task 1 RED)
- a0a38f7 feat(03-10): compose reporter signals from history rows and current votes (Task 1 GREEN)
- 4a73b8f test(03-10): add failing tests for the none signal and the trust object field (Task 2 RED)
- 3667f77 feat(03-10): carry the reporter signal on every trust object (Task 2 GREEN)
- 184f3f6 docs(03-10): document reporter reliability in the trust model (Task 3)

The RED commits do not compile on purpose (the symbols they test did not exist yet).

## Test evidence

- `TestReporterSignalsGroupByAccount`: PASS, 8 subtests (six signals 86, 71, 32, 75, New 4 of 5, New 0; own confirm does not count; own resolve keeps the contradicted outcome; unrequested key 107 absent; exactly 6 keys; stray votes change nothing; shuffled input equal; 200 iteration reference property with at least one rated reporter).
- `TestReporterSignalsNewAndNone`: PASS, 13 subtests (J of 1 and 2 sentences, in flight, at-now boundary 86, D-12 and R-06, window edge, both Q4 cap cases, fallback meta, empty request, canonical none, pass through with fresh Score pointer, display only on every visibility, field shape).
- `TestTrustCopyReporterIsPlainText`: PASS, 4 subtests (exact copy, builder grid, composition grid over 300 random histories, trust object grid).
- `TestReliabilityTable` PASS (27 rows) before the anchor table was copied from the outline; the doc table is the outline's 19 rows verbatim (21 pipe lines).
- Unchanged neighbours green: `TestTrustCarriesNoIdentifiers`, `TestBuildTrust*`, `TestTrustCopyIsPlainText`, `TestCastVoteResultCarriesTrust`, `TestTrustAttachedOnEveryReadPath`, `TestClassifyOutcome`, `TestReliability*`.
- Full suite on a fresh `p3_0310`: `go test ./... -count=1 -p 1` returned rc=0, every package `ok`. A second `-v` run of `./internal/store/ ./internal/service/` on a fresh `p3_0310` showed no `--- SKIP` and `TestReporterHistoryBounds`, `TestReporterHistoryUsesAccountNotSession`, `TestReporterHistoryUsesSessionIndex` passing. `p3_0310` was dropped each time (checked with `psql -l`). `go vet ./...` clean; `gofmt -l internal/service` empty; no U+2013 or U+2014 in any touched file.
- The doc gate (headings once, order, key phrases, 21 anchor pipe lines, no spoof claims) was run as a script and printed `DOC_OK`.

## Notes for later plans

- The three read paths (`Nearby`, `CastVote`, `ActivityForAccount`) carry the none signal until 03-12 sets `TrustInput.Reporter`. `reporterSignalFor` gives every trust object its own Score pointer.
- For 03-12: `ReporterSignals` expects the vote slice to include the history report ids (one batched `CurrentVotesForReports` over page ids and history ids together). It groups votes by report itself, so the caller need not.
- Q4 cap, stated precisely: the 03-06 history read caps at the latest 30 EXPIRED reports inside the window per account, which is not the same as 30 judged reports (RESOLVED Q4). Neutral rows (critical or rescue needed reports that expired unconfirmed) occupy cap slots. This is recorded in `docs/TRUST-MODEL.md` and pinned by the two Q4 cases in `TestReporterSignalsNewAndNone` (30 neutral push 5 corroborated out; 29 neutral leave 1 corroborated, J of 1).

## Deviations from Plan

Auto-fixed: none.

Substitutions:

- The long Task 1, 2 and 3 `<automated>` one-liners were not run verbatim (the sandbox refuses long compound commands). I ran the same checks as separate commands: `gofmt -l`, `go vet`, the named `go test -run` sets, the quick `go test ./internal/service/ ./web/`, the call-site and forbidden-constant greps, the dash scan with perl, and a scratch script for the doc gate. The final database run used the plan's own `dropdb`, `createdb`, `go test`, `rc=$?`, `dropdb` form.
- Quick runs used plain `go test` after `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset`, instead of `env -u DATABASE_URL go test`.
- One test helper outside the plan's listed set, `p10Deref`, `p10Int`, `p10Part`, `p10Batch` and `p10Join`, was added to `reporter_test.go` (all `p10` prefixed, same file).

## Known Stubs

None.

## Threat Flags

None. `ReporterSignal` carries only Tag, Score, Label, Sentence, ScoreSentence and the two counts, pinned by a reflection test.

## Self-Check: PASSED

- Files exist: reporter.go, reporter_test.go, trustcopy.go, trustcopy_test.go, trustview.go, docs/TRUST-MODEL.md.
- Commits exist: 7cc3dd3, a0a38f7, 4a73b8f, 3667f77, 184f3f6.
- `git diff --name-only` over the plan's commits lists only the six files in `files_modified`.
