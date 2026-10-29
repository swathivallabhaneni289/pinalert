---
phase: 08-report-page-and-comments
plan: 01
subsystem: api
tags: [timeline, vote-log, trust, sqlc, chi, go]
requires:
  - phase: 02-trust-mechanic
    provides: BuildVoteTally, Resolve, ClosedByFor, closedText
  - phase: 03.1-history
    provides: ReportService.Focus, ViewOf
provides:
  - "TimelineVoteLog query (whole vote log of one report, id order, row cap)"
  - "service.BuildTimeline, TimelineInput, TimelineEvent, TimelineKind, TimelineMilestones, TimelineLogCap"
  - "service.TimelineService.Timeline and the closed timeline copy"
  - "GET /api/reports/{id}/timeline, handlers.ReportTimeline, TimelineResponse"
affects: [08-report-page, 08-19 docs regeneration]
tech-stack:
  added: []
  patterns: ["prefix replay of the append only vote log through BuildVoteTally and Resolve"]
key-files:
  created:
    - internal/service/timeline.go
    - internal/service/timelinecopy.go
    - internal/service/timelineread.go
    - internal/service/timeline_test.go
    - internal/service/timelinecopy_test.go
    - internal/api/handlers/timeline.go
    - internal/api/handlers/timeline_e2e_test.go
  modified:
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go
    - internal/api/router.go
key-decisions:
  - "The new query is TimelineVoteLog, not VoteLogForReport, because Phase 3.3 owns that name."
  - "A take back that lifts a retraction writes no reopen line; only a reopen vote does."
  - "No downward step is ever written; a milestone already reached is never written twice."
requirements-completed: [PAGE-01]
status: complete
duration: about 1 hour
completed: 2026-10-29
---

# Phase 8 Plan 01: The report history, "What happened so far" Summary

A gated JSON route serves a report's history, built by replaying the append only vote log through the same BuildVoteTally and Resolve the trust block uses, as closed copy sentences with no voter, account, place or cell on the wire.

## What landed

- Query `TimelineVoteLog(report_id, row_cap)` in `internal/store/queries/votes.sql`, generated into `internal/store/sqlc/votes.sql.go` (type `TimelineVoteLogRow`: id, account_id, kind, value, geohash_cell, created_at). Only that sqlc file changed.
- `internal/service/timeline.go`: `TimelineKind` (posted, confirmed, disputed, resolved, reopened, faded, yours), `TimelineEvent{Kind, Tone, Text, At, Yours}`, `TimelineMilestones` {1,2,3,4,5,10,25,50}, `TimelineLogCap` 2000, `TimelineInput`, pure `BuildTimeline`.
- `internal/service/timelinecopy.go`: every sentence, from constants and strconv.Itoa only.
- `internal/service/timelineread.go`: `TimelineQuerier` (ReportVoteContext, TimelineVoteLog), `NewTimelineService`, `WithTimelineClock`, `Timeline(ctx, reportID, viewerAccountID)`, `TimelineNotFoundMessage`.
- `internal/api/handlers/timeline.go`: `ReportTimeline`, `TimelineResponse{Events}`, `TimelineEventResponse{kind, tone, text, at, yours}`, swag block.
- `internal/api/router.go`: `timeline := service.NewTimelineService(deps.Sessions, deps.Reports)` and `r.Get("/api/reports/{id}/timeline", handlers.ReportTimeline(timeline))` flat in the gated group next to the flag route. No rate gate.

## Commits

- b976b981 feat(08-01): replay the vote log into the report history
- 50f40be0 feat(08-01): serve the report history at GET /api/reports/{id}/timeline

## Tests

- Service, no DB: `TestBuildTimeline` (16 table cases plus tone, fade, yours, determinism and input-untouched subtests), `TestTimelineMatchesTrust` (60 seeded random logs with take backs; each count equals BuildVoteTally at the writing row and never exceeds the highest reached), `TestTimelineWording`.
- Handlers e2e on p8_01_a: `TestTimelineForViewer`, `TestTimelineResponseOmitsIdentifiers`, `TestTimelineWithheldIs404` (pending, removed, too old and unknown id give byte equal 404 bodies), `TestTimelineRequiresSignIn` (extra, 401).
- Full run `go test ./... -p 1 -count=1` on p8_01_a: every package ok; the only SKIP lines are TestGeocodeLive, TestOfficialLive and TestScreeningFixture; zero FAIL. `go build ./...` and `go vet ./...` clean.
- Swag: `swag init -g internal/api/router.go` into a scratch directory outside the repository shows `/reports/{id}/timeline` with a `get` operation; docs/ untouched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Query renamed from VoteLogForReport to TimelineVoteLog**
- **Found during:** Task 1 (`make sqlc` failed with "duplicate query name")
- **Issue:** Phase 3.3 already has `VoteLogForReport` in push.sql (read by the push worker, no id column, no cap), and it is on the list of names other phases call.
- **Fix:** the new read is `TimelineVoteLog` with `TimelineVoteLogParams{ReportID, RowCap}`. push.sql is untouched. The plan's acceptance greps for `name: VoteLogForReport` and `func (q *Queries) VoteLogForReport` therefore match the old query only; the equivalent greps are `name: TimelineVoteLog` and `func (q *Queries) TimelineVoteLog`. `TimelineInput.Log` is `[]sqlcgen.TimelineVoteLogRow`.
- **Commit:** b976b981

**2. [Rule 3 - Blocking] TimelineQuerier reads ReportVoteContext, not ReportForFocus**
- **Found during:** Task 2
- **Issue:** ReportForFocus has no reporter account and no moderation state, both of which BuildTimeline needs.
- **Fix:** the existing ReportVoteContext (the read VotingService and FlagService make) supplies severity, category, moderation state, reporter, created and expiry. Focus is still the gate; a report that became withheld between the two reads is ErrReportNotFound. No new query.
- **Commit:** 50f40be0

**3. [Rule 1 - Bug] TimelineVoteLog moved above VotedReportsByAccount in votes.sql**
- **Found during:** full test run
- **Issue:** `TestVotedReportsByAccountUsesTheAccountIndex` reads votes.sql from VotedReportsByAccount to the end of the file as one statement; a query appended after it broke EXPLAIN.
- **Fix:** the query sits above VotedReportsByAccount, with a comment saying why. The older test is unchanged.
- **Commit:** 50f40be0

**4. [Rule 1 - Bug] BuildTimeline names the moderation state when it calls Resolve**
- **Found during:** full test run (`TestResolveCallersMentionModeration` scans every Resolve caller for a `Moderation` key in a ReportMeta literal)
- **Fix:** BuildTimeline rebuilds the meta field by field, Moderation included. Same behavior.
- **Commit:** 50f40be0

### Mechanism differences from the plan text (same behavior)

- The replay does not call ClosureOf at every step (it would be cubic over a 2000 row log, on a route polled every 30 seconds). It reads the retraction flip from Resolve's visibility (Retracted) at each step and builds the Closure for ClosedByFor from that step's tally (ReporterResolved, ResolveCells at the threshold), which is what ClosureOf itself does at the flip.
- Entering and leaving the Disputed view are read from the Resolve visibility (Hidden) at the row time, not from ViewOf, so a row at or after the expiry cannot lose a line. ViewOf is used for the faded decision.

## Decisions recorded

- Only upward steps and state changes are written. A milestone already reached is never written twice. After a take back the trust block may read lower than the last line; the block is the current truth (for the 08-19 TRUST-MODEL text).
- The step that first makes a report Live writes one line, "Confirmed from N spots. Shown to everyone.", in place of the milestone line. A critical report is Live from posting and never gets that line. It is written once per report.
- Entering the Disputed view writes the dispute sentence plus " Marked Disputed." on a milestone, or the count form "Disputed from N spots. Marked Disputed." off a milestone. Leaving writes "No longer marked Disputed." unless the report was resolved.
- A take back (unresolve) that lifts a retraction writes no reopen line (D-28: a take back is not a reopen), so after "Resolved by ..." the history may stay silent until the trust block shows the report active again. If the owner wants a take back line, it needs one more closed sentence.
- The reporter's own content rows get no "You confirmed it." line, matching the tally that excludes them.
- "You marked it resolved." is dropped only when the same row closes the report as "Resolved by you" or "Resolved by you and people nearby".
- Event order: time, then log id, then closure, confirm, dispute, leave, yours within one row.

## Notes for later plans

- Route shape: `GET /api/reports/{id}/timeline` answers `{"events":[{"kind","tone","text","at","yours"}]}`, `Cache-Control: no-store`, 404 body `{"error":{"field":"report","message":"This report is no longer available."}}` for every unavailable report, 400 for a malformed id.
- Phase 8 plans that touch votes.sql must keep VotedReportsByAccount last in the file.
- docs/ was not regenerated (plan 08-19 does it once).
- The swag scratch output stayed in the session scratchpad (a delete was refused); nothing is in the repository.

## Known Stubs

None.

## Threat Flags

None. The new route is in the plan's threat model (T-08-01 to T-08-04); the identifier scan, the Focus gate and the row cap are in place and tested.

## Self-Check: PASSED

- Files exist: timeline.go, timelinecopy.go, timelineread.go, both service tests, handlers/timeline.go, timeline_e2e_test.go.
- Commits b976b981 and 50f40be0 exist on the worktree branch.
