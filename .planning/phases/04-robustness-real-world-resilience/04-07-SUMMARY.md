---
phase: 04-robustness-real-world-resilience
plan: 07
subsystem: service
tags: [moderation, trust, resolver, feed, activity, source-guard, e2e]
requires:
  - phase: 04-01
    provides: ModerationState, ReportMeta.Moderation, Withheld, Serious, the withheld Resolve rungs
  - phase: 04-05
    provides: ReportExtrasForReports query, ReportVoteContextRow.ModerationState, the moderation columns
provides:
  - CastVote refuses a withheld report as not found right after the context read
  - Nearby, ActivityPageForAccount and the shared history context each make one ReportExtrasForReports read and pass Moderation into Resolve
  - WrittenLabelThreshold, WrittenAtForDisplay, ReportView.WrittenAt and ViewerFlagged, ActivityReport.Moderation and WrittenAt
  - TestResolveCallersMentionModeration, an AST source guard on every Resolve caller
  - TestWithheldReportsInNoFeed, an end to end proof over the real router and Postgres
affects: [04-11, 04-16, 04-17, 04-20, 04-29]
tech-stack:
  added: []
  patterns:
    - one batched extras read per non empty page, indexed by report id, with a fail closed skip
    - source guard scored per function declaration with go/parser
key-files:
  created:
    - internal/service/resolve_callers_test.go
    - internal/api/handlers/withheld_e2e_test.go
  modified:
    - internal/service/trust.go
    - internal/service/feed.go
    - internal/service/report.go
    - internal/service/auth.go
    - internal/service/historyread.go
    - internal/service/trust_test.go
    - internal/service/feed_test.go
    - internal/service/auth_test.go
    - internal/service/activity_page_test.go
key-decisions:
  - "Activity skips BuildTrust exactly when ReportMeta.Withheld(), not whenever moderation is not clear: a pending serious report is public (D-02, P4-A10) and keeps its trust object"
  - "A report with no extras row is not shown on any read path (fail closed, T-04-16)"
  - "A voted report (another person's post) that resolves to withheld is dropped from the votes list; only its author's own Activity page lists it (R-01). Default recorded for the owner to veto"
  - "The history paths read extras with ViewerAccountID 0 because they need only the moderation state"
status: complete
completed: 2026-10-16
---

# Phase 4 Plan 07: Moderation reaches every read and vote path Summary

**Moderation state now reaches every Resolve call: a removed or held report is in no feed, no map pin, no history view and takes no vote, a pending serious report stays published, and a source guard fails the build for any future Resolve caller that forgets it.**

## Accomplishments
- `CastVote` builds its one `ReportMeta` (now with `Moderation` from `ReportVoteContextRow.ModerationState`) right after the context read and returns `ErrReportNotFound` when `meta.Withheld()`. The refusal sits before the own report block, the expiry check and the distance check, makes exactly one store call and writes no row. The doc comment gained a step 3 and the later steps were renumbered.
- `Nearby` makes one `ReportExtrasForReports` call per non empty page (page ids, the viewer's account id), after the vote read and before `attachUndos`. Moderation reaches `Resolve`, a report with no extras row is skipped, and `ReportView.WrittenAt` / `ViewerFlagged` are filled after `Resolve`.
- `ActivityPageForAccount` (which `ActivityForAccount` wraps) makes one extras read over the own post ids then the voted ids, carries `Moderation` and `WrittenAt` on each row, skips `BuildTrust` for a withheld row, and drops a withheld voted report.
- Phase 3.1's `loadReadContext` makes one extras read over the candidate ids and `classify` feeds `Moderation` into `Resolve`, so `History`, `RecentHistoryCounts` and `Focus` drop a withheld report (none view, `unavailable` for a link). A candidate with no extras row gets the none view.
- `WrittenAtForDisplay` and `writtenAtPtr` (NULL column to nil, never a pointer to the zero time).
- `TestResolveCallersMentionModeration` scans the package source with `go/parser`, per function declaration, and allowlists only `reliability.go` (asserting it still makes exactly one call).
- `TestWithheldReportsInNoFeed` proves the rule on a real database.

## Task Commits
1. Task 1 RED `956224f` test: failing tests for refusing votes on withheld reports
2. Task 1 GREEN `1d881f8` feat: refuse votes on withheld reports as not found
3. Task 2 RED `280345b` test: failing tests for moderation extras on the read paths (compiling stubs, fakes, tests)
4. Task 2 GREEN `39dcac1` feat: carry moderation and written time on the feed, activity and history reads
5. Task 3 `57a0382` test: guard every Resolve caller and prove withheld reports are in no feed (the production code already existed, so there is one commit)

## Step 0 census (Task 1)
- BASE: `70a642bdec8812d3d2144784328f799e0bd7ed8b`.
- Every Phase 3, Phase 3.1 and Phase 4 prerequisite named by the plan was present (04-05 generated structs, 04-01 predicates, `testVoteContext`, `farLat`, `ClassifyOutcome`, `BuildTrust`, `p05VotingFake`, `ReporterHistoryReports` on all three interfaces, the 03-12 tests, every e2e helper including `getNearby`, `h10URL`, `h10Get`, `h10MovePast`, `h10Find`).
- Committed type of `ReportExtrasForReportsRow.ClientWrittenAt`: `pgtype.Timestamptz`.
- `Resolve` callers found: `Nearby` (`report.go`), `CastVote` (`trust.go`), `ActivityPageForAccount` (`auth.go`, two calls), `classify` (`historyread.go`), `ClassifyOutcome` (`reliability.go`, exactly one). `feed.go` only mentions it in a comment. No caller outside the expected files.
- Baseline counts, before and after: `return CastVoteResult{}, ErrReportNotFound` in `CastVote` 1 then 2; `ReportMeta{` in `CastVote` 1 then 1; `Trust{` across `trust.go`, `report.go`, `auth.go` 0 then 0; `BuildTrust(TrustInput{` in `auth.go` 2 then 2.
- Feed view helpers the e2e used: `getNearby` (sends the `show_disputed=true` read alias, serves the default and disputed views), `h10URL` / `h10Get` for `view=disputed`, `view=past` and `report=ID`. A Past view existed, so the Past and report link checks ran.

## Existing tests whose call log gained the one `ReportExtrasForReports` entry
Only these three, each as the last entry on a success path that reaches the extras read:
- `TestNearbyBatchesVoteReadsOnceWithReporters` (`feed_test.go`, first subtest; its empty page subtest now also counts the new name as zero)
- `TestActivityForAccountReadsReporterHistoryOnce` (`auth_test.go`, first subtest)
- `TestActivityPageReadsPostsAndVotesInOnePass` (`activity_page_test.go`)

`TestNearbyBatchesVoteReadsOnce` gained the extras assertions the plan lists (one call, 25 ids in order, viewer id; zero on an empty page), and its Nearby query now carries `ViewerAccountID: 30`. This deviates from the outline's "Tests that change" list for the first two, as the plan states; all are in files this plan owns. No other existing test needed an edit (the unconfigured fakes answer a clear row per id).

## Decisions Made
- "Skips BuildTrust when moderation is not clear" is read as "skips exactly when `ReportMeta.Withheld()`": a pending serious report is public and takes real votes, so its Activity row keeps a full trust object (D-02, P4-A10). Only held and removed rows carry the zero `Trust`.
- Fail closed: a report the extras read did not return is not listed on `Nearby`, not returned by `ActivityPageForAccount`, and gets the none view in `classify`. With the real store every requested id has a row, so this only guards a store inconsistency.
- A withheld voted report is dropped from the Activity votes list (default recorded for the owner to veto).

## Deviations from Plan
None in behavior. Two small notes:
1. The plan's verify commands use `env -u DATABASE_URL`, which the sandbox refuses, so the quick runs used `go test` with `DATABASE_URL` unset in the shell (it was empty). This is the same thing.
2. A context-usage hook rewrote `.planning/STATE.md` in the worktree twice while the plan ran; I restored it with a path limited `git checkout` and never committed it. No STATE.md, ROADMAP.md or REQUIREMENTS.md change is part of this plan's commits.

## Issues Encountered
None. The e2e stop rule (Activity page unable to render `withheld`) did not trigger: `/profile` answered 200 and listed all six descriptions with no handler or template change.

## Verification
- `go build ./...` and `go vet ./...` clean; `gofmt -l` clean on all touched files.
- Quick run with no database: `go test ./internal/service/ -count=1` ok; `go test ./internal/api/... ./internal/store/ -count=1` ok (database tests skip by design).
- Named tests PASS: `TestCastVoteRefusesWithheld`, `TestCastVoteCheckOrder`, `TestCastVoteReadsReporterHistoryOnce`, `TestCastVoteRejectsTooFar`, `TestCastVoteReporterExemption`, `TestWrittenAtForDisplay`, `TestNearbyBatchesVoteReadsOnce`, `TestNearbyBatchesVoteReadsOnceWithReporters`, `TestNearbyCarriesModerationExtras`, `TestActivityCarriesModeration`, `TestHistoryReadsCarryModeration`, `TestActivityPageReadsPostsAndVotesInOnePass`, `TestActivityForAccountReadsReporterHistoryOnce`, `TestTrustAttachedOnEveryReadPath`, `TestCastVoteTrustMatchesNearbyTrust`, `TestResolveModeration`, `TestWithheldInNoView`, `TestResolveCallersMentionModeration`.
- `DATABASE_URL=postgres://localhost:5432/p4_0407?sslmode=disable go test ./internal/api/handlers/ -run TestWithheldReportsInNoFeed -count=1 -p 1 -v`: PASS (4 subtests, 0.13s).
- Full suite on a fresh `p4_0407`: `go test ./... -count=1 -p 1 -v` all packages ok, 0 FAIL, 0 SKIP, total under 20 seconds of test time. `p4_0407` was dropped afterwards.
- No U+2014 or U+2013 on any added line. No `.sql`, migration, generated file, handler, template, `visibility.go`, `reliability.go`, `reporter.go` or swagger enum changed.

## Known Stubs
None.

## Threat Flags
None. No new endpoint, auth path or schema change; T-04-16 and T-04-17 are mitigated as planned.

## Notes for the orchestrator
- `withheld` still reaches `BuildTrust` nowhere on the read paths: `Nearby` never lists a withheld report, `CastVote` refuses before building trust, and Activity skips `BuildTrust` for a withheld row.
- Between 04-07's RED and GREEN commits a sibling's whole package run could see the Task 1 and Task 2 failing tests named in the plan; both GREEN commits are in.
- `baseView` in `historyread.go` still builds `TrustInput.Meta` without `Moderation`; it is only reached for non withheld reports, so nothing changes. 04-29 or a later plan can pass it through if `BuildTrust` ever reads moderation.

## Self-Check: PASSED
Created files exist (`resolve_callers_test.go`, `withheld_e2e_test.go`, this SUMMARY); commits 956224f, 1d881f8, 280345b, 39dcac1 and 57a0382 are in the log; the full suite passed on a fresh `p4_0407` with 0 SKIP.
