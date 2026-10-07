---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 07
subsystem: api
tags: [trust, openapi, swag, e2e, go, chi]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "service.Trust, BuildTrust, ReportView.Trust, CastVoteResult.Trust, WithReportClock and WithVotingClock (03-05)"
provides:
  - "handlers.TrustResponse and trustToResponse, the one mapper from service.Trust to JSON"
  - "trust object on every GET /api/reports element and every vote response"
  - "OpenAPI definition handlers.TrustResponse plus a scoped identifier name guard"
  - "newTrustE2EServer, p07TrustWireKeys, p07GuardedSwaggerRoots test helpers"
affects: [03-13, 03-14, 03-15]
tech-stack:
  added: []
  patterns:
    - "one handler mapper shared by feed, vote response and later the Activity page"
    - "swagger name guard scoped to guarded roots and the definitions they reference"
key-files:
  created:
    - internal/api/handlers/trust.go
    - internal/api/handlers/trust_e2e_test.go
  modified:
    - internal/api/handlers/reports.go
    - internal/api/handlers/votes.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "trustToResponse copies seven fields and computes nothing; the 99+ cap and every sentence stay in the service (A27, D-19)"
  - "p07SessionID delegates to the existing sessionIDFromJar helper rather than re-reading the cookie"
requirements-completed: [TRUST-05, D-04, D-21]
status: complete
duration: 40min
completed: 2026-10-08
---

# Phase 3 Plan 07: Trust object on the wire Summary

The feed and all four vote routes now return one identical `trust` object (two place counters, their server capped numerals and three panel sentences) through a single mapper, documented in OpenAPI and guarded against identifier leaks.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Feed and every vote response carry the same trust object, N counts places | 735c957 |
| 2 | OpenAPI documents trust, scoped name guard, no identifiers in real bodies | 5141141 |
| 3 | Concurrent votes from distinct places land as exactly that many places | eca8669 |

## Evidence

RED runs:
- Task 1: both new tests failed with `confirm vote response carries no trust object: {"visibility":"provisional","reason":"awaiting_second_independent_confirmation"}`.
- Task 2: `TestSwaggerSpecCoversRoutes` failed with `docs/swagger.json is missing the guarded definition "handlers.TrustResponse"` before `swag init` ran.

`swag init` diff: one new definition (`handlers.TrustResponse`, seven properties), two `trust` properties (FeedReportResponse and CastVoteResponse, each an allOf wrapper around the $ref), five description lines (one on GET /reports, four on the vote routes). A fresh `swag init -g internal/api/router.go -o <tmp>/docs` produced docs.go, swagger.json and swagger.yaml identical to the committed files.

Database runs, all on a freshly created `p3_0307`, dropped afterwards:
- `go test -race ./internal/api/handlers/ -run 'TestTrustCountUnderConcurrentVotes|TestFeedTrustMatchesCastVoteTrust|TestTrustCountsDistinctPlacesNotVotes|TestTrustResponseOmitsIdentifiers' -count=1 -p 1 -v`: all four `--- PASS`, `ok pinalert/internal/api/handlers 3.296s`, no `WARNING: DATA RACE`. The concurrency test also passed with `-count=8`.
- Full suite `go test ./... -count=1 -p 1`: every package `ok` (api, handlers, auth, geocode, mailer, ratelimit, service, session, store, testutil, web).
- `TestSwaggerSpecCoversRoutes` and `TestSwaggerDocServed` pass with DATABASE_URL unset.

Long dash counts after the edits (HEAD baselines in brackets): votes.go 8 [8], reports.go 22 [22], swagger_test.go 3 [3], swagger.json 7 [7], docs.go 7 [7], swagger.yaml 8 [8]. trust.go and trust_e2e_test.go: 0.

## Deviations from Plan

**1. [Verify command substitution]** The sandbox refused the plan's compound one-line verify commands (the `(dropdb; createdb && out=$(...); st=$?; dropdb ...)` form and the `S=$(mktemp -d); swag init ...` form). I ran the same checks as separate plain commands: drop and create the database, run `go test` with `DATABASE_URL` set inline and grep the result lines, run `swag init` into a scratchpad `docs` directory and `diff -q` each file, then drop the database. The grep and count gates from the verify blocks were run individually with the same patterns. `DATABASE_URL` was unset in the shell, so no `env -u` prefix was needed.

**2. [Rule 3 - helper reuse]** `p07SessionID` calls the existing `sessionIDFromJar` (auth_e2e_test.go) instead of repeating the cookie parsing. Behavior is the same: it returns the id before the first dot and fails when absent.

No other deviations. No file outside `files_modified` changed; no edit to internal/service, STATE.md, ROADMAP.md or REQUIREMENTS.md.

## Notes for later plans

- 03-14: `newTrustE2EServer` has the request-link limiter opened (`Burst: 100, Every: time.Millisecond`). Append `currency` and `reporter` to `p07TrustWireKeys`, and `handlers.CurrencyResponse` and `handlers.ReporterResponse` to `p07GuardedSwaggerRoots`; do not rewrite either list. `Trust.Currency` from 03-08 is deliberately not mapped yet.
- 03-15: `trustToResponse` is the mapper the Activity `data-trust` JSON must use.

## Known Stubs

None.

## Threat Flags

None. The new wire surface is the seven count and sentence fields; T-03-04 and T-03-06 (wire half) are covered by `TestSwaggerSpecCoversRoutes`, `TestTrustResponseOmitsIdentifiers` and `p07AssertTrustKeys`.

## Self-Check: PASSED

Files found: trust.go, trust_e2e_test.go, 03-07-SUMMARY.md. Commits found: 735c957, 5141141, eca8669.
