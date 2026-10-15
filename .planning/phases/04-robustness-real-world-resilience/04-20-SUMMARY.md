---
phase: 04-robustness-real-world-resilience
plan: 20
subsystem: api
tags: [flag, moderation, rate-limit, openapi, go]
requires:
  - phase: 04-01
    provides: ReportMeta.Withheld and Serious
  - phase: 04-05
    provides: report_flags table and InsertReportFlag
  - phase: 04-07
    provides: ReportView.ViewerFlagged filled from the batched extras read
  - phase: 04-09
    provides: RateGate, RateLimitsDefault.Flag, FlagLimitMessage
  - phase: 04-15
    provides: RateLimitResponse and p15Assert429
provides:
  - POST /api/reports/{id}/flag (service, handler, gate middleware, route)
  - viewer_flagged on every feed element
  - OpenAPI for the flag route with a drift guard
affects: [04-23, 04-24]
tech-stack:
  added: []
  patterns: [consumer defined two method querier, per account PerKey bucket never refunded]
key-files:
  created:
    - internal/service/flag.go
    - internal/service/flag_test.go
    - internal/api/handlers/flag.go
    - internal/api/handlers/flag_e2e_test.go
  modified:
    - internal/api/handlers/ratelimit_gate.go
    - internal/api/router.go
    - internal/api/handlers/reports.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "Flag bucket charged per admitted attempt, never refunded (like the vote bucket)"
  - "Expired reports can be flagged; 409 means only already flagged"
  - "Check order: reason, read, withheld, own report, insert"
  - "400 shapes: bad reason is field reason, bad body is field body, bad id is field id"
  - "Server copy reuses the dialog wording"
requirements-completed: []
duration: about 1 hour
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 20: Flag route and viewer marker Summary

**Signed in people can flag someone else's report once with one of five reasons through POST /api/reports/{id}/flag, limited per account, never hiding the report, and the feed tells each viewer which reports they already flagged.**

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | 1d5f432 | internal/service/flag.go and flag_test.go |
| 2 | 59aaaf3 | handlers/flag.go, FlagMiddleware, router registration, flag_e2e_test.go (five tests) |
| 3 | c02611d | viewer_flagged, regenerated docs, TestFlagViewerMarker and TestFlagSwaggerCoversRoute |

## Step 0 records

- `ReportVoteContextRow` has `ReporterAccountID *int64` and `ModerationState string`. `ReportMeta.Moderation`, `Withheld()`, `Serious()` present (visibility.go). `assertServerCopy` present. `InsertReportFlag(ctx, InsertReportFlagParams{ReportID, AccountID, Reason}) (int64, error)` present. `ReportView.ViewerFlagged` present and filled by 04-07.
- Gate: variable `rateGate` in `NewRouter`, built by `handlers.NewRateGate(limits, clock, clientIPHeader, chrome)`. Key helper `accountKey(id)` gives `acct:<id>`. Backstop helper `reserveIPBackstop(r)`, bucket name logged `ip_backstop`. Block log shape `handlers: rate limit: bucket=%s account=%d wait_seconds=%d` through `logBlock`. `writeRateLimited(w, field, message, wait, serious, number)`.
- `RateLimits.Flag`, `RateLimitsDefault.Flag = {2 min, 10}`, `LimiterClock`, `ClientIPHeader` present. `FlagLimitMessage` present. `flagBurst` and `flagEvery` wired in main.go (not touched).
- Harness: `newE2EServerWith(t, tweak) (*httptest.Server, *recordingMailer, *pgxpool.Pool)`.
- Feed query used by the tests: `/api/reports?lat=..&lon=..&radius_km=5`, default view (no `view` parameter; the server default is `active`).
- Swagger baselines on this base: 6 `RateLimitResponse` refs and 6 `Retry-After` headers (the plan's Task 1 text said 5, its Task 3 text said 6; Phase 3.1's unresolve route makes it 6). A fresh `swag init` matched the committed docs before and after this plan. After: 7 and 7 in swagger.json, 7 refs in docs.go.
- Names were free (`fakeFlagQuerier`, `FlagService`, `p20*`).

## Decisions this plan settled (owner can veto)

1. Gate shape: `r.With(rateGate.FlagMiddleware())`, backstop first, then the account bucket, nothing refunded, so a 404, 409 or 400 costs a token.
2. Expired reports can be flagged (Past rows included); 409 is reserved for already flagged.
3. Check order: reason, context read, withheld, own report, insert. An author's own withheld report answers 404.
4. 400 shapes: unknown or missing reason is field `reason`; bad JSON or unknown key is field `body`; bad id is `parseReportID`'s field `id`.
5. Copy: the four server messages reuse the dialog wording; 403 has no dialog counterpart.

## Gate additions

The flag PerKey field and the two constants (`flagBucketName = "flag"`, `flagRateField = "rate_flag"`) were not present and were added in `ratelimit_gate.go`; the bucket is built inside `NewRateGate` from `limits.Flag`.

## Byte comparison exclusions

None. No feed key depends on the wall clock at read time on these reads, so `TestFlagNeverChangesVisibility` and `TestFlagViewerMarker` compare whole raw feed objects.

## Regenerated swagger counts

`/reports/{id}/flag` once, `handlers.FlagRequest` once, `handlers.FlagResponse` once, `viewer_flagged` once, `#/definitions/handlers.RateLimitResponse` 7 in swagger.json and 7 in docs.go, `"Retry-After": {` 7. The `FlagRequest.reason` enum is five JSON strings in `FlagReasons` order (the `false` slug is a string).

## Verification run

- `go test ./... -count=1` with DATABASE_URL unset: all packages ok.
- Database run on the disposable `p4_0420` (created, dropped afterwards): `DATABASE_URL=postgres://localhost:5432/p4_0420?sslmode=disable go test ./internal/api/... -count=1 -p 1 -v` ok for both packages, 175 top level PASS lines, 0 `--- SKIP`.
- `-race` on `-run TestFlag` (handlers) on `p4_0420`: all seven TestFlag* tests pass. Task 2's run with `-race` also covered TestVoteRateLimit, TestIPBackstop, TestReportRateLimit, TestWithheldReportsInNoFeed and TestCastVoteOnMissingReportIs404.
- Service quick run: TestServerCopyFlag, TestFlagReasons, TestFlagServiceRules, TestFlagQuerierCannotWriteReports, TestResolveCallersMentionModeration, TestServerCopyRejections pass.
- No U+2014 or U+2013 in any added line; gofmt clean; no server started.

## Deviations from Plan

### Auto-fixed Issues

None to code. Notes:

- Each task is one commit (feat), not a separate RED and GREEN commit, per the orchestrator's one commit per task rule. The RED state (tests first) was observed locally: Task 1 failed to compile before `flag.go`, and `TestFlagSwaggerCoversRoute` failed before `make swag`.
- The plan's verify commands use `env -u DATABASE_URL`, which the sandbox refuses; DATABASE_URL was unset in the shell, so plain `go test` has the same effect.
- Task 1 step 0 wrote swagger baseline "5 and 5" while Task 3 said 6; the code has 6 (see above). Final counts 7 and 7 follow Task 3.

**Total deviations:** 0 behavior changes. **Impact:** none.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the flag route, which is in the plan's threat model (T-04-08, T-04-09, T-04-16, T-04-11).

## For the orchestrator

- Only the ten files in `files_modified` plus this SUMMARY changed. No STATE, ROADMAP or REQUIREMENTS edits.
- No browser harness check changes by design; the Swagger UI page now lists the flag route (the plan's human check for the end of phase UAT).

## Self-Check: PASSED

Files exist, commits 1d5f432, 59aaaf3 and c02611d are on the branch, all test commands above passed with 0 SKIP on the database run.
