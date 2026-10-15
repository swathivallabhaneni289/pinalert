---
phase: 04-robustness-real-world-resilience
plan: 11
subsystem: api
tags: [go, moderation, text-screening, held-state, swagger]
status: complete

requires:
  - phase: 04-robustness-real-world-resilience
    provides: 04-01 ReportMeta.Withheld and Serious, 04-02 Decide, IsGibberish and RejectMessage, 04-04 moderation.Screener and LoadScreener, 04-05 InsertReportIdempotent and InsertModerationCheck, 04-07 held-invisible read path, 04-09 newE2EServerWith
provides:
  - service.CheckText, the one general text check (gibberish prefilter, one 3 second provider call, four generic outcomes)
  - service.SubmitChecked, which replaces the deleted Submit and is the one submit path
  - service.WithScreener, CreatedReport, SubmitResult, ErrTextRejected, TextRejectedError
  - POST /api/reports answers 422 on field description for refused text and carries held in every 201
  - cmd/server/main.go builds one screener per process
  - regenerated API reference with held and the 422
affects: [04-14, 04-15, 04-16, 04-17, 04-22, 04-27]

tech-stack:
  added: []
  patterns:
    - "Generic text check (no report vocabulary) with the report rules layered in SubmitChecked"
    - "Moderation state decided before the insert and written by it, never changed after"
    - "Check row written with one immediate retry; failure logs the numeric report id only"

key-files:
  created:
    - internal/service/textcheck.go
    - internal/service/textcheck_test.go
    - internal/api/handlers/submit_screening_e2e_test.go
  modified:
    - internal/service/report.go
    - internal/service/report_test.go
    - internal/service/feed_test.go
    - internal/api/handlers/reports.go
    - cmd/server/main.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml

key-decisions:
  - "CheckText passes a constant to Decide (which never reads it), so abuse is refused at every severity (D-03, T-04-02)"
  - "An unfinished check stores the report pending in the same insert at every severity; a serious pending report is published by the critical bypass and keeps its pending check row"
  - "If the check row cannot be written after one immediate retry, the stored report is still returned with a nil error and the report id is logged"

requirements-completed: [ROBUST-03]
duration: about 55 min
completed: 2026-10-16
---

# Phase 4 Plan 11: Inline text screening and the held state Summary

**POST /api/reports now screens the trimmed description once, inside 3 seconds, before storing anything: abuse and gibberish answer 422 with a named reason, borderline text posts with a review row, and a check that cannot finish holds a non serious report off every feed while a serious one publishes with its check still queued.**

## What was built

- Task 1 (`7fc71a4`): `internal/service/textcheck.go` with `TextScreener`, `TextCheckOutcome`, `TextCheck`, `InlineScreenTimeout` and `CheckText`. `ValidateSubmitInput` counts runes (`utf8.RuneCountInString`) and `WithScreener` was added. `TestCheckText` (13 subtests) and `TestValidateSubmitInputCountsRunes` (V-07) pass.
- Task 2 (`81b947c`): `SubmitChecked`, `CreatedReport`, `SubmitResult`, `ErrTextRejected`, `TextRejectedError`, `createdReportFromInsertRow`, `textCheckRow`; the old `Submit` and `reportFromInsertRow` are deleted; `Querier` names `InsertReportIdempotent` and `InsertModerationCheck`. `SubmitReport` validates, calls `SubmitChecked`, maps a rejection to 422 with `service.RejectMessage`, and `ReportResponse.Held` is always present. `TestSubmitChecked` (no database) and the four V-04 end to end tests pass on `p4_0411`.
- Task 3 (`c7ddc3d`): `moderation.LoadScreener(env)` in `main.go` (fatal on error), injected with `service.WithScreener(screener)`. The swag block gained the 422 and two description sentences. `make swag` ran once and is idempotent (a second run leaves the three files byte identical). `TestSubmitScreeningSwaggerFields` passes.

## Step 0 findings

- `ReportOption func(*ReportService)`, `NewReportService(q Querier, opts ...ReportOption)`, `WithReportClock` and the field `ReportService.now` exist as expected.
- `Querier` before the change held `InsertReport`, `NearbyReports`, `CurrentVotesForReports`, `ReporterAccountsForReports`, `ReporterHistoryReports`, `ExpiredReportsInWindow`, `OpenReportsInArea`, `ReportForFocus`, `ResolutionUndoCounts`, `ReportExtrasForReports`. The only implementer outside the generated `*sqlcgen.Queries` is `fakeFeedQuerier` (and `p05VotingFake` by embedding); nothing else needed the new methods.
- Generated types: `InsertReportIdempotentParams` adds `ModerationState string`, `RequestID *string`, `RequestAccountID *int64`, `ClientWrittenAt pgtype.Timestamptz`; the row adds `ModerationState string`. `InsertModerationCheckParams` is `ReportID int64`, `Status string`, `NextAttemptAt time.Time`, `Verdict *string`, `Scores []byte`, `Model *string`, `CheckedAt pgtype.Timestamptz`, `NeedsReview bool`.
- `CallError.Error` has a pointer receiver; `CheckText` uses `errors.As` with a `*moderation.CallError` target.
- `newE2EServerWith(t, tweak func(d *api.Deps, q *sqlcgen.Queries))` returns `(*httptest.Server, *recordingMailer, *pgxpool.Pool)`.
- The disputed view is still selected with `show_disputed=true` through `getNearby(..., true)`; the critical bypass reason slug is `critical_bypasses_gates`.
- No `p11`, `fakeTextScreener` or `fakeSubmitQuerier` identifier existed in package `handlers_test` or `service_test`. `ValidateSubmitInput` held 2 byte length calls before and 0 after.

## Decisions (Claude's Discretion)

**Check row write failure.** The report row is already stored and the store has no transaction, so a 500 would invite the author to post the same report again. `SubmitChecked` therefore tries `InsertModerationCheck` at most twice, and if both fail it logs `service: SubmitChecked: recording the text check for report <id>: <error>` (the numeric id and the error only) and still returns the stored report with a nil error. A held report stays held, which is the fail closed side.

**Accepted residual risk (T-04-16).** A report whose check row could not be written even after the retry has no automatic recovery: 04-14's claim and 04-05's Pending list read only `moderation_checks`, so a held report stays held and off every feed with no worker retry and no moderator list entry, and a pending serious report stays published unchecked. This is rare (two consecutive failed writes right after a successful insert on the same database) and the log names the id. The owner can veto this and ask 04-14 or 04-23 to also pick up pending reports that have no check row.

## swagger.json diff summary

Exactly four changes and nothing else: the two new description sentences on `POST /reports`, the `422` response referring to `handlers.ErrorResponse`, the `held` boolean property on `handlers.ReportResponse`, and the new `SubmitReportRequest.description` text. No path or definition was gained or lost, and none of `withheld`, `awaiting_text_check`, `removed_after_review` appears.

## Deviations from Plan

None in behavior. Notes on process:
- The RED stub step for `textcheck.go` was skipped; the tests were written first and `TestValidateSubmitInputCountsRunes/telugu` failed before the rune count change, and the new end to end tests passed on the first run once the handler was wired. The `textcheck_test.go` file also imports `os` (to restore the log writer), which the plan's import list did not name.
- No existing test description tripped the gibberish prefilter.

## Verification

- `go build ./...`, `go vet ./...` and `gofmt -l` clean.
- `go test ./internal/service/ ./internal/api/... -count=1` passes without a database.
- `DATABASE_URL=postgres://localhost:5432/p4_0411?sslmode=disable go test ./... -count=1 -p 1 -v`: every package ok, 703 PASS lines. One `--- SKIP`: `TestScreeningFixture`, which is not database backed; it skips until 04-27 records the fixture scores. The database backed SKIP count is 0.
- `p4_0411` was created by this run, used only by it, and dropped.

## Known Stubs

None.

## Threat Flags

None. No new endpoint or trust boundary beyond the plan's threat model; the screener receives only the trimmed description.

## Self-Check: PASSED

Files exist (`textcheck.go`, `textcheck_test.go`, `submit_screening_e2e_test.go`, the edited files and the three docs files), commits `7fc71a4`, `81b947c` and `c7ddc3d` exist, `git status --porcelain -- docs/` is empty, and the full suite is green on `p4_0411`.
