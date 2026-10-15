---
phase: 04-robustness-real-world-resilience
plan: 01
status: blocked
subsystem: service
tags: [visibility, moderation, tdd]
requirements: [ROBUST-03]
key-files:
  created: []
  modified: []
metrics:
  tasks_completed: 0
  tasks_total: 2
  completed: 2026-10-16
---

# Phase 4 Plan 01: Moderation rung of the visibility resolver Summary

Blocked at the Step A5 precondition scan of Task 1. No source file was edited and no RED or GREEN commit exists. The plan's own rule is to stop and report before editing anything when a fifth visibility would make an existing test fail, and that is what happened.

## Step A results (base 2a765a4)

- A1: `Resolve` and `criticalBypass` each print one line in visibility.go; the five rungs are in the Phase 2 order; `ReportMeta` has fields Severity and Category only. Every `ReportMeta{` literal in internal and cmd is keyed (no positional literal), so a third field breaks none.
- A2: `func (v Visibility) ListableInFeed(includeDisputed bool) bool` in feed.go line 81; the switch has a `default` returning false.
- A3: closed view type `FeedView` (history.go, list `FeedViews`); the only view predicate is `ViewOf(v Visibility, expiresAt, now time.Time) FeedView` (pure, `default` returns `FeedViewNone`). `trustLeadSentence` and `buildTrustFacts` only format text. No I/O needed.
- A4: the neutral `Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryOther}, ` call count in reliability.go is 1.
- A5: enum loop scan found the loops below. A trial run (appending `withheld` to `Visibilities` in a scratch edit, since reverted with `git checkout -- internal/service/visibility.go`) shows exactly two failing tests; every other loop stays green.

## Stop condition fired

Adding `VisibilityWithheld` to `Visibilities` (Task 2 step 2, and the closed set the RED tests pin) fails two tests that belong to other plans and are outside this plan's `files_modified`:

1. `TestViewOf/total_and_disjoint` in `internal/service/history_test.go` (the subtest at lines 69 to 81, Phase 3.1). It ranges over `Visibilities` and demands `ViewOf(v, ...)` is never `FeedViewNone` and always a valid `FeedView`. `ViewOf(withheld, ...)` is `FeedViewNone` by design (fail closed default), and R-01 requires exactly that. Output: `ViewOf("withheld", ...) = none, want one of the four views` for all three expiry values.
2. `TestPanelWordingStaysFilled` in `internal/service/trustwording_test.go` (Phase 3, grid from `p30Grid`, which ranges over `Visibilities`, lines 85 onward; assertions at lines 223 to 233). For `withheld` the real `BuildTrust` fills the currency label "Report needs re-confirming" and a currency score, because `buildCurrency` treats `withheld` like live; the test's expectations (`shown := in.Visibility == VisibilityLive && !expired`, `scored` for Live or Provisional only) say no. Every failure line starts with `withheld/`.

The other loops over `Visibilities` or `ResolveReasons` (`trustview_test.go:176`, `reporter_test.go:530`, `trustcopy_test.go:136, 291, 422`, `visibility_test.go:383`, and `review_fix_b_test.go` which uses a local copy) stay green.

## Needed from the orchestrator

Pick one, then re-run 04-01 (the plan text is otherwise sound):

- Option A (smallest, test only): allow 04-01 to edit two test files. In `history_test.go` make the "total and disjoint" loop skip `VisibilityWithheld` (or assert `FeedViewNone` for it), and in `trustwording_test.go` make `p30Grid` skip `VisibilityWithheld` (a withheld report never reaches `BuildTrust`; 04-07 and 04-16 decide what a trust panel for it is).
- Option B: leave those two tests to the plan that wires `withheld` into read paths (04-07) and keep `withheld` out of `Visibilities` here. This contradicts the plan's pinned closed set (`Visibilities` ends in `VisibilityWithheld`), so it needs the plan text changed.

Option A matches R-01 and the plan's intent ("withheld is valid but listable in no view"). Both fixes are two to four line test edits.

## Deviations from Plan

None in code. The plan could not start Task 1 because the A5 stop condition fired; nothing was improvised.

## Commits

None for tasks. This SUMMARY is the only commit of this run.

## Self-Check: FAILED

Plan not executed: Step A5 stop condition (two existing tests, `TestViewOf/total_and_disjoint` and `TestPanelWordingStaysFilled`, fail once `withheld` joins `Visibilities`; both files are outside this plan's scope). No source files changed.
