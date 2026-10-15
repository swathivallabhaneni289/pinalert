---
phase: 04-robustness-real-world-resilience
plan: 04
subsystem: moderation
tags: [openai-moderation, rate-limit, fixtures, recorder]
requires: []
provides:
  - "internal/moderation: Client, NewClient, Screen, CallError, Result, Screener, DefaultModel, LoadScreener, PassScreener, PassModel"
  - "cmd/modrecord: opt in recorder for the D-07 fixture scores"
affects: [04-11, 04-14, 04-27, 04-28]
tech-stack:
  added: []
  patterns: ["one attempt per call, classified *CallError", "two rate.Limiters reserved at one instant, refused pair returned with CancelAt(now)"]
key-files:
  created:
    - internal/moderation/client.go
    - internal/moderation/client_test.go
    - internal/moderation/loader.go
    - internal/moderation/loader_test.go
    - cmd/modrecord/main.go
    - cmd/modrecord/main_test.go
  modified: []
key-decisions:
  - "Fixture files are top level JSON arrays in the field order id, text, language, serious, expect, model, scores, two space indent, HTML escaping off, trailing newline, empty scores as {} and unrecorded model as the empty string"
  - "Screen always returns *CallError; callers use errors.As with a *moderation.CallError target"
requirements-completed: [ROBUST-03]
duration: about 40 minutes
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 04: Moderation provider client, loader and fixture recorder Summary

A standalone `internal/moderation` package calls the OpenAI Moderation endpoint with one attempt, a 3 second deadline, only the trimmed text, classified failures and a process wide budget, and a `cmd/modrecord` command records the fixture scores with no live call anywhere in the tests.

## Tasks and commits

| Task | Commit | What |
| ---- | ------ | ---- |
| 1 | 182faae | Provider client: request shape, status classification, Retry-After, budget |
| 2 | bd5516f | `LoadScreener` and `PassScreener` |
| 3 | e47595a | `cmd/modrecord` recorder |

Each task was commit with explicit paths (`git commit -- <paths>`).

## Tests first

Each task started with a compiling skeleton whose methods return placeholders, then the tests. Failures seen before the real code:

- Task 1: every `Screen` test failed on assertions with an empty error text (skeleton returned an empty `*CallError`), `TestParseRetryAfter` failed on the placeholder value.
- Task 2: all five `TestLoadScreener` subtests failed (`LoadScreener returned <nil>, want *Client`, nil error where an error was expected); `TestLoadScreenerModelOverride` panicked on the nil interface.
- Task 3: six of seven recorder tests failed on assertions (run returned 1 instead of 0 or the refusal text was missing). `TestRecorderRejectsUnknownField` passed trivially against the stub, which returns 1; it exercises real validation once the recorder reads files.

What made them pass: the full `client.go` (typed request and response, `CheckRedirect` returning `http.ErrUseLastResponse`, `ReserveN`/`DelayFrom`/`CancelAt(now)` budget, bounded body reads), the mailer shaped `loader.go`, and the recorder with strict decoding, atomic write, bounded retry and the quota stop.

## Verbatim test output

Command: `go test ./internal/moderation/ ./cmd/modrecord/ -count=1 -race -v`, top level result lines (97 RUN and result lines in total including subtests; zero SKIP lines):

```
--- PASS: TestClientRequestShape (0.00s)
--- PASS: TestClientSendsOnlyTrimmedText (0.00s)
--- PASS: TestClientRetryAfter (0.01s)
--- PASS: TestClientTransientAndPermanent (0.01s)
--- PASS: TestClientQuotaIsNotRetried (0.00s)
--- PASS: TestClientTimeout (0.05s)
--- PASS: TestClientBudgetRefusesWithoutNetwork (0.00s)
--- PASS: TestModerationURLIsHTTPS (0.00s)
--- PASS: TestClientErrorsCarryNoSecrets (0.06s)
--- PASS: TestParseRetryAfter (0.00s)
--- PASS: TestLoadScreener (0.00s)
--- PASS: TestLoadScreenerModelOverride (0.00s)
PASS
ok  	pinalert/internal/moderation	1.510s
--- PASS: TestRecorderRefusesWithoutLiveFlag (0.01s)
--- PASS: TestRecorderFillsModelAndScores (0.00s)
--- PASS: TestRecorderKeepsTextByteIdentical (0.01s)
--- PASS: TestRecorderAlwaysUsesPinnedModel (0.00s)
--- PASS: TestRecorderStopsOnQuota (0.00s)
--- PASS: TestRecorderRetriesTransientThenStops (0.00s)
--- PASS: TestRecorderRejectsUnknownField (0.00s)
PASS
ok  	pinalert/cmd/modrecord	1.688s
```

`go run ./cmd/modrecord` with no environment prints the `MODERATION_LIVE` refusal and exits 1. No database was used and no `DATABASE_URL` was set.

## Notes for later plans

- 04-11 and 04-14: `Screen` returns errors as `*moderation.CallError`; use `errors.As` with a `*moderation.CallError` target. `Transient` means pending and retry later (honour `RetryAfter`, at most one hour), `Quota` means never retry, neither means permanent. A `CallError` built outside the package (`&moderation.CallError{Status: 401}`) has a non empty `Error()` ("provider error").
- 04-11 must build one `Client` per process through `LoadScreener` and share it, which is what makes the 250 requests and 10,000 token a minute budget process wide.
- 04-28: the fixture files must be top level JSON arrays in the recorder's layout (field order id, text, language, serious, expect, model, scores; two space indent; HTML escaping off; trailing newline; empty `scores` as `{}`, unrecorded `model` as `""`). The recorder rejects unknown fields and trailing data and never creates a missing file.
- 04-27: run `MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord` from the repository root. A zero balance key that is refused prints a line naming `insufficient_quota`.

## Deviations from Plan

None. The plan was executed as written. Small implementation choices inside the plan's freedom: the budget refusal rounds the reservation delay up to whole seconds with a one second floor; `Screen` also returns a permanent `CallError` with the phrase "request not built" if request construction fails (unreachable with a constant URL); the recorder calls `d.wait` before every attempt including retries.

## Known Stubs

None.

## Threat Flags

None. The only outbound surface is the constant https provider URL the plan's threat model already covers.

## Self-Check: PASSED

All six files exist; commits 182faae, bd5516f and e47595a are present; the package tests pass under `-race` with zero SKIP; `go.mod` and `go.sum` are unchanged; no U+2013 or U+2014 in the six files.
