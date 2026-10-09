---
quick_id: 261008-t9o
type: quick
status: complete
description: Revise Phase 4 plans 04-13 and 04-29 to match the built Phase 3 code
completed: 2026-10-09
commit: ef3e01f
---

# Quick task 261008-t9o summary

## Done

- Plan 04-13 now fits the built `fetchReports(options)`, the write floor and the stale response guard. Rules C1 to C9 say how the 10 second timeout, the background refresh and the stale line compose with them. The signal travels in `new Request(url, { signal })`, so the call text stays `pending = fetch(url)` and every Phase 3 test runs unedited.
- Plan 04-29 and the outline state the rule review fix WR-05 built: neutral rows (held and removed included) are dropped before the cap of 30 judged reports, with a SQL read cap of 100. No locked decision was changed: R-02 says nothing about the cap.
- The Phase 3 rows of both Cross-phase prerequisites tables were re-checked against the built code. 7 outline edits were applied, with no new dash lines.
- Commit ef3e01f holds the three changed files (04-13-PLAN.md, 04-29-PLAN.md, 04-PLAN-OUTLINE.md).

## How it was checked

- Per plan, three independent passes: code truth, integrity and downstream consumers, and the standard plan check, then one repair round and a re-check.
- After the repair round, two items remained in scope and both were fixed by hand on the checkers' exact suggestions (not re-run through a checker): 04-13 wrongly said `TestUpdatedAgoPhrases` guards the comment placement rule (it reads with all comments stripped, so only `TestAgeAndFadeUseTheServerClock` does); 04-29's Step 0 census recipe would also match the two `func` declarations and could trigger a false stop.

## Needs the owner

1. **Empty list when the signal drops (04-13).** Revised plan: a failed poll with no rows shows only the stale line and hides "No reports near you yet". The earlier plan kept that text above the stale line. Not forced by any pin, and it makes the main page differ from the Low data view (04-18 keeps it). Recommendation: keep "No reports near you yet" and add a small loaded-empty signal so only the Show disputed drop case stays silent (closer to D-20).
2. **Extend the same fix to the other stale plans.** Out of this task's file list, so not done: 04-22 (its submitReport chain drops `{ afterWrite: true }` and would turn a Phase 3 test red), 04-27 (pins a sentence, `The two readings differ only for`, that no longer exists in docs/TRUST-MODEL.md after WR-05), 04-05 (a stale not built yet row), 04-18 (empty block wording), and four documents that still describe pre-Phase-3 code. Recommended as one follow-up quick task.
3. **TRUST-MODEL wording (from the 04-29 reviser).** The sentence that the Phase 4 rate limits are what bound 100 neutral reports may be too strong: the allowances bound a rate, not a count. Decide whether to soften it or to close the residual in SQL.

## Other files that would become false (from the two revisions, not edited)

From the 04-13 revision:
- 04-22-PLAN.md: GREEN app.js step for submitReport refetches with fetchReports().then(function () { return report; }, function () { return report; }). The built form and a Phase 3 pin require { afterWrite: true }: TestFetchReportsDropsOnlyResponsesOlderThanTheLastApplied checks the text fetchReports({ afterWrite: ...
- 04-18-PLAN.md: GREEN renderLow(state) step says an error status after a loaded empty list keeps the empty block and adds the stale line. The revised 04-13 hides the empty state while the latest poll failed (rule C4), so the Low data view and the main page would differ. Inconsistent, not false. One line fix if ...
- 04-24-PLAN.md: The inputs list and the sendFlag step say 'the 10 second AbortSignal.timeout pattern in fetchReports()' and 'build the signal as fetchReports does'. Still true for building the signal (native signal, or AbortController with a timer cleared on settle). Only the carrier differs: 04-13 passes the ...
- 04-19-PLAN.md: Checked, nothing becomes false: it adds new functions to app.js and edits no existing one, and the remark that Phase 3.1 rewrites fetchReports is unaffected. It still reads the data-toast-open and #notice-live names 04-13 keeps.
- 04-RESEARCH.md: Area D 'Other landmines' says render hides the list on a failed poll and that no existing test reads the body of render. Both are stale: since Phase 3 review rounds 1 and 2 render keeps the rows on error and TestFeedErrorStateKeepsStaleRows reads its body. The revised plan carries a correction note ...
- 04-PATTERNS.md: The feed.js bullet says that on error it hides the list although rows are kept. The built error block hides the list only when there are no rows (var hasRows = state.reports.length > 0).
- 04-CONTEXT.md: 'Integration Points and Landmines' bullet says feed.js render hides the list on any failed poll although it keeps the rows (D-20 changes this). No longer true since Phase 3 review round 1; the decisions D-17 and D-20 themselves are met as written.
- 04-UI-SPEC.md: The source cell of amendment P4-A19 quotes isBackgroundRefresh = state.status === 'ok' with no timeout and render hiding #report-list on error. The built code already has the status-or-rows rule and keeps rows on error. The amendment's decisions are met as written (hasLoaded stays set once loaded, ...

From the 04-29 revision:
- 04-27-PLAN.md: Several statements become false or unrunnable. (1) The must_have truth says docs/TRUST-MODEL.md 'names them where the two readings of the 30 report cap differ', and an artifact provides line says 'in the cap readings sentence'. The built doc has no two readings: WR-05 rewrote that section into one ...
- 04-PLAN-OUTLINE.md: The 04-27 row of the 'Docs to update' table (the docs/TRUST-MODEL.md entry) says the sentence about where the two cap readings differ also names held and removed reports. That sentence no longer exists in the built doc; the row needs the same re-anchoring as 04-27. Not part of the four 04-29 edits ...
- 04-26-PLAN.md: Nothing becomes false. Its 'row cap' wording (a list past its 100 row cap, in the Output paragraph) is the moderator list cap and has no link to the reporter history cap or to R-02. It never repeats the old neutral rows rule.
- 04-07-PLAN.md: Nothing false. 04-07 edits TestNearbyBatchesVoteReadsOnceWithReporters and TestActivityForAccountReadsReporterHistoryOnce to add the trailing ReportExtrasForReports call log entry. 04-29 relies on the RowCap: ReliabilityHistoryReadCap expectations in those two tests (and in ...
- 04-05-PLAN.md: Nothing false about the cap. Its Cross-phase row for the 03-06 query still says 'not built yet', which is stale because 03-06 is built, but its claim that make sqlc must leave reliability.sql.go byte identical in 04-05 stays true (04-29 owns that query). Not caused by this revision.
- 04-01-PLAN.md, 04-10-PLAN.md, 04-16-PLAN.md, 04-28-PLAN.md: They cite 04-29 only by test name (TestModeratedReportsAreNeutralForRating), by the single neutral Resolve line, or by file prefix p429. All of those are unchanged, so nothing becomes false.
- .continue-here.md and 03-FIX-VERIFICATION-1.md: The cross phase note ('Plan 04-29 pins the old reporter cap rule') and finding WR-05-V2 are resolved by this revision of the plan. I did not edit them; their owner can drop the note.

## Deviations from stock quick mode, on purpose

- No worktree executor (documentation only; avoids a merge commit and a base race with the other session).
- Path limited commits with the trailer. STATE.md carries this task's row but is not committed, because it holds an uncommitted edit from another session.
- Raw job results were kept in the session transcript only; the key findings are in this file.
