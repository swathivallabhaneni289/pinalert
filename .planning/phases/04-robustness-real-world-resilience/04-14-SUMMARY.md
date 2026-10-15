---
phase: 04-robustness-real-world-resilience
plan: 14
subsystem: api
tags: [go, moderation, background-worker, backoff, late-verdicts]
status: complete

requires:
  - phase: 04-robustness-real-world-resilience
    provides: 04-01 ReportMeta.Withheld and Serious, 04-02 Decide, 04-04 moderation.CallError, 04-05 claim and write statements, 04-11 TextScreener and the pending check rows
provides:
  - service.ModerationWorker, the background retry of text checks that could not finish at submit
  - service.ModerationBackoff, the pure backoff ladder with jitter and Retry-After
  - the worker started with the server process from cmd/server/main.go
affects: [04-15, 04-16, 04-22, 04-27]

tech-stack:
  added: []
  patterns:
    - "Consumer defined querier interface satisfied by *sqlcgen.Queries, checked with a var _ assertion"
    - "Report state written first, check closed second, so a crash re-screens instead of stranding a report"
    - "Waiting only through next_attempt_at, never a sleep or retry loop inside a sweep"

key-files:
  created:
    - internal/service/modworker.go
    - internal/service/modworker_test.go
  modified:
    - cmd/server/main.go

key-decisions:
  - "No attempt count cap for transient errors; the report's expiry (enforced by the claim) is the bound"
  - "Quota and permanent errors are rescheduled to the report's expires_at, which the claim never reaches, so the check stays pending and the report stays in the moderator Pending list"
  - "Shutdown (parent context cancelled during a provider call) writes nothing; the lease lapses"

requirements-completed: [ROBUST-03]
duration: about 35 min
completed: 2026-10-16
---

# Phase 4 Plan 14: Background moderation worker Summary

**A held report whose text check could not finish at submit is now retried in the background every 30 s (and once at boot): a late pass releases it, a late borderline releases it and lists it for review, a late reject removes it at every severity, and an expired report is never retried.**

## What was built

- Task 1 (`836e7c2`): `internal/service/modworker.go` with `WorkerQuerier`, `ModerationWorker`, `NewModerationWorker`, the options `WithWorkerClock`, `WithWorkerJitter`, `WithSweepInterval`, `ModerationBackoff`, `Run`, `SweepOnce` and the ten constants. `internal/service/modworker_test.go` with `fakeWorkerQuerier` and `TestModerationWorkerBackoff`, `TestModerationWorkerWriteOrder`, `TestModerationWorkerRunSweepsAtStartAndStops`.
- Task 2 (`cf95c9a`): the four database tests `TestModerationWorker` (late pass, retry then release, two concurrent workers, seeded row), `TestModerationWorkerLateRejectRemovesSerious`, `TestModerationWorkerSkipsExpired`, `TestModerationWorkerLateBorderlineNeedsReview`, the helper `insertPendingReport`, and the two lines in `cmd/server/main.go` (`moderationWorker := service.NewModerationWorker(queries, screener)` and `go moderationWorker.Run(context.Background())`, before `srv.ListenAndServe()`).

## Step 0 findings

- `BASE` = `1ebcc6570188224f758619a3c142d7d762ad8814`.
- Every symbol the plan names exists as expected: `ModerationState` and its three constants, `ReportMeta.Moderation`, `Serious()`, `Withheld()`, `Decide`, the three `Verdict` constants, `moderation.Result`, `moderation.CallError` (`Status int`, `Transient bool`, `Quota bool`, `RetryAfter time.Duration`, pointer receiver `Error`), `TextScreener`, and the generated statements.
- Generated types: `ClaimDueModerationChecksParams{LeaseSeconds int32; BatchLimit int32}`; `ClaimDueModerationChecksRow{ReportID int64; Attempts int32; Description string; Severity string; Category string; ModerationState string; ExpiresAt time.Time}`; `FinishModerationCheckParams{Verdict string; Scores []byte; Model *string; NeedsReview bool; ReportID int64}`; `RescheduleModerationCheckParams{NextAttemptAt time.Time; LastError *string; ReportID int64}`; `SetReportModerationStateParams{ModerationState string; ReportID int64}`; `InsertReportIdempotentParams{SessionID, Category, Severity, Description string; Latitude, Longitude float64; Geohash string; ShelterCapacityStatus *string; ShelterHeadcount *int32; CreatedAt, ExpiresAt time.Time; ModerationState string; RequestID *string; RequestAccountID *int64; ClientWrittenAt pgtype.Timestamptz}`.
- `fakeTextScreener` (package `service_test`, `textcheck_test.go`) has `mu sync.Mutex`, `result moderation.Result`, `err error`, `block bool`, `calls int`, `texts []string`, `deadlines []time.Duration`, `hadDeadline []bool`, plus `callCount()`; the helper `screenerWith(scores)` also exists and is reused.
- The screener variable in `cmd/server/main.go` is `screener` (type `moderation.Screener`, which satisfies `TextScreener`); `queries := sqlcgen.New(pool)` is the querier.
- `fakeWorkerQuerier` and `insertPendingReport` did not exist in the package before.

## RED then GREEN

RED: with only the test file present, `go vet ./internal/service/` failed with `undefined: service.WorkerQuerier`. GREEN: after `modworker.go`, the three unit tests passed under `-race` with no database.

## Database run on p4_0414

`p4_0414` was created by this run (it did not exist), used only by it, and dropped afterwards. Command: `DATABASE_URL=postgres://localhost:5432/p4_0414?sslmode=disable go test ./internal/service/ -run TestModerationWorker -count=1 -p 1 -race -v`. Result: ok. Seven top level `--- PASS` lines (`TestModerationWorker`, `TestModerationWorkerBackoff`, `TestModerationWorkerLateRejectRemovesSerious`, `TestModerationWorkerSkipsExpired`, `TestModerationWorkerLateBorderlineNeedsReview`, `TestModerationWorkerWriteOrder`, `TestModerationWorkerRunSweepsAtStartAndStops`) and 0 `--- SKIP`. The quick run `go test ./... -count=1` with `DATABASE_URL` unset is green for every package. `go build ./...`, `go vet` and `gofmt -l` are clean.

## Decisions to veto (Claude's Discretion)

1. **No attempt count cap for transient errors.** This departs from RESEARCH Area A Landmine 1, which gives "at most 2 retries" to the background worker. Reason: the outline pins the formula `min(30 s * 2^attempts, 1 h)`, whose 1 h cap two retries could never reach, and D-02 needs a held report released after an outage of any length. The report's expiry, enforced by the claim on the database clock, is the bound.
2. **Quota and permanent errors are never retried.** With no new query, the stop is a reschedule to the report's `expires_at`, which the claim never reaches, so the check stays `pending` and the report stays in the moderator Pending list for Keep or Remove (D-08 c). They are logged at error level with the class word and status number only.

Not a veto item: a moderator Keep that races an in flight provider call cannot be undone by a late reject, because 04-05's `KeepReport` closes the check in the same statement and `SetReportModerationState`, `FinishModerationCheck` and `RescheduleModerationCheck` each write only while the check is still `pending`. The worker issues no UPDATE of its own.

## 04-11 residual (T-04-16)

Still true. This plan does not touch the claim or retry path beyond reading `moderation_checks`: if `SubmitChecked` could not write the check row after two tries, there is no `moderation_checks` row for that report, so the worker never claims it. A non serious report stays held and a serious one stays published unchecked, with no automatic recovery and no moderator list entry. The log line from 04-11 names the report id. The owner can still ask 04-23 (or a later plan) to also pick up pending reports that have no check row.

## Deviations from Plan

None in behavior. Notes:
- When a screener call fails because the parent context was cancelled, `SweepOnce` returns nil (not the context error), so shutdown is quiet; `Run` also suppresses the sweep error log once the context is done.
- A nil `Scores` map is stored as `{}` and an empty model as SQL NULL.
- A write failure is logged without the error text (the report id and attempt only) and the error is returned joined from `SweepOnce`, which `Run` logs.

## Verification gates

- `git diff --name-only <BASE>..HEAD` lists exactly `cmd/server/main.go`, `internal/service/modworker.go`, `internal/service/modworker_test.go`.
- No U+2013 or U+2014 in the new files or the added `main.go` lines; no `log.Print` line mentions description, scores, email, session or coordinates; no HTTP or chi import in `modworker.go`.
- Browser harness: nothing changes by design (no route, template, CSS or JSON field).

## Known Stubs

None.

## Threat Flags

None. The worker sends only the trimmed description to the existing screener and adds no endpoint.

## Self-Check: PASSED

`internal/service/modworker.go`, `internal/service/modworker_test.go` and `cmd/server/main.go` exist with the changes; commits `836e7c2` and `cf95c9a` exist; the seven worker tests pass on `p4_0414` with 0 skips and the quick run is green.
