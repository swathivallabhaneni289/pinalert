---
phase: 04-robustness-real-world-resilience
plan: 01
subsystem: service
tags: [visibility, moderation, tdd, resolver]
requires: []
provides:
  - ModerationState, ModerationClear, ModerationPending, ModerationRemoved, ModerationStates
  - VisibilityWithheld (fifth value of Visibilities)
  - ReasonRemovedAfterReview, ReasonAwaitingTextCheck (last two of ResolveReasons)
  - ReportMeta.Moderation, ReportMeta.Serious(), ReportMeta.Withheld()
affects: [04-07, 04-11, 04-14, 04-15, 04-16, 04-22, 04-23, 04-29]
tech-stack:
  added: []
  patterns: [closed enum with exact Valid(), one shared predicate read by two rungs]
key-files:
  created:
    - internal/service/moderation_view_test.go
  modified:
    - internal/service/visibility.go
    - internal/service/visibility_test.go
    - internal/service/history_test.go
    - internal/service/trustwording_test.go
decisions:
  - "Serious() is a one line wrapper over criticalBypass(); Withheld() is removed, or pending and not Serious(); Resolve's first statement reads Withheld()."
  - "No swagger enums tag changes: withheld and the two new slugs reach no documented JSON response."
requirements: [ROBUST-03]
status: complete
completed: 2026-10-16
---

# Phase 4 Plan 01: Moderation rung of the visibility resolver Summary

`Resolve` now withholds a removed report above every other rung (even critical) and a pending non serious report next, while a pending serious report falls through to the existing ladder; the zero value and `clear` behave exactly as before, and `withheld` is valid but listable in no view.

## Step A prerequisite results (base 2a765a4)

- A1: `Resolve` and `criticalBypass` each matched one line; five rungs in the Phase 2 order; `ReportMeta` had only Severity and Category. Every `ReportMeta{` literal in internal and cmd is keyed, so the new field broke none.
- A2: `func (v Visibility) ListableInFeed(includeDisputed bool) bool`, feed.go line 81; the switch `default` returns false.
- A3: closed view type `FeedView` (history.go, list `FeedViews`). The only view predicate is `ViewOf(v Visibility, expiresAt, now time.Time) FeedView` (pure). `trustLeadSentence` and `buildTrustFacts` only format text; `FeedView.Valid` and `BuildHistoryFacts` are skipped by name in the source guard.
- A4: reliability.go has exactly 1 non comment neutral `Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryOther}, ` call.
- A5 enum loop scan: hits were trustview_test.go:176, reporter_test.go:530, history_test.go:70, trustwording_test.go:85, trustcopy_test.go:136, 291, 422, visibility_test.go:383 (plus review_fix_b_test.go, which uses a local copy). Verdict: all stay green except two, which failed with `withheld` in `Visibilities`: `TestViewOf/total_and_disjoint` (history_test.go) and `TestPanelWordingStaysFilled` (trustwording_test.go, via `p30Grid`). The orchestrator approved Option A (edit both tests); see Deviations.

## RED

Commit `07aa08c`. Stubs compile; all five pinned tests print `--- FAIL` on assertions with no build or setup failure: TestReportMetaSeriousMatchesBypass, TestWithheldInNoView, TestResolveModeration, TestResolveModerationZeroValueNeutral, TestReportMetaWithheld.

## GREEN

Commit `e6d9ef3`. `ModerationState.Valid()` (exact loop), `withheld` appended to `Visibilities`, the two slugs appended to `ResolveReasons`, `Serious()` and `Withheld()` bodies, and the `if meta.Withheld() {` block as the first statement of `Resolve`. Doc comments updated (header, Visibility, VisibilityWithheld, ResolveReason, ModerationState, Moderation field, Serious, Withheld, Resolve purity paragraph and the seven item rung list). No existing long dash comment line was edited.

All of the following pass with `DATABASE_URL` unset: the five pinned tests, `TestResolve`, `TestResolve_IsTotalAndDeterministic`, `TestListableInFeed`, `TestViewOf`, `TestPanelWordingStaysFilled`. The quick run `go test ./... -count=1` tail:

```
ok  	pinalert/internal/api	0.842s
ok  	pinalert/internal/api/handlers	3.092s
ok  	pinalert/internal/auth	4.123s
ok  	pinalert/internal/geocode	1.799s
ok  	pinalert/internal/mailer	2.253s
ok  	pinalert/internal/ratelimit	1.860s
ok  	pinalert/internal/service	4.953s
ok  	pinalert/internal/session	3.644s
ok  	pinalert/internal/store	5.594s
ok  	pinalert/internal/testutil	5.010s
ok  	pinalert/web	5.566s
```

Database backed tests skip by design with `DATABASE_URL` unset; this plan needs no database and none was created or touched.

## REFACTOR

None.

## Deviations from Plan

### Scope addition approved by the orchestrator (Rule 3, blocking)

Adding `withheld` to `Visibilities` failed two existing tests in files outside `files_modified`. The orchestrator approved editing `internal/service/history_test.go` and `internal/service/trustwording_test.go` (no other wave 1 or 2 plan lists them). Both edits are in the GREEN commit `e6d9ef3`.

1. `history_test.go`, subtest "total and disjoint" of `TestViewOf`. Old: the loop asserted every visibility in `Visibilities` gets a valid `FeedView` (never `FeedViewNone`). New: inside the same loop, before that assertion:
   ```go
   // A withheld report lands in no view (R-01).
   if v == VisibilityWithheld {
       if got != FeedViewNone {
           t.Errorf("ViewOf(%q, %v) = %q, want none", v, exp, got)
       }
       continue
   }
   ```
   The original assertions stay in force for every other visibility.
2. `trustwording_test.go`, `p30Grid`. Old: `for _, vis := range Visibilities {` went straight into `for mname, meta := range metas {`. New: between them,
   ```go
   // A withheld report never reaches BuildTrust (R-01).
   if vis == VisibilityWithheld {
       continue
   }
   ```

### Other

- Commit trailer: the rules file and the orchestrator's decision say `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the plan text says `Sonnet 5.5`. The rules file wins; all three code commits use the Opus trailer, so the plan's step 8 `grep -c ... Sonnet 5.5` check would print 0 by design.
- The plan's verify commands use `env -u DATABASE_URL`, which the sandbox refuses. `DATABASE_URL` is unset in this environment, so plain `go` commands were used.
- A first, blocked version of this SUMMARY was committed earlier as `df1fcc3`; this file replaces it.

## Commits

- 07aa08c test(04-01): add failing moderation rung tests
- e6d9ef3 feat(04-01): add the moderation rung to Resolve
- df1fcc3 docs(04-01): blocked report (superseded)
- the final SUMMARY commit (docs(04-01), see the report)

## Known Stubs

None.

## Threat Flags

None. T-04-16 is mitigated for the resolver itself (removed and held rungs above Retracted and the bypass, read through one predicate; `TestWithheldInNoView` plus its source guard).

## Self-Check: PASSED

visibility.go, visibility_test.go, moderation_view_test.go, history_test.go and trustwording_test.go exist with the stated changes; commits 07aa08c and e6d9ef3 exist; the five pinned tests and the full quick run pass.
