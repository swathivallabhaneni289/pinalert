---
phase: 04-robustness-real-world-resilience
plan: 09
subsystem: api
tags: [go, chi, rate-limit, votes, trust, perkey]
status: complete

requires:
  - phase: 04-robustness-real-world-resilience
    provides: 04-02 assertServerCopy, 04-03 ratelimit.PerKey, 04-06 handlers.Chrome
provides:
  - service.WaitPhrase and the five server composed rate limit messages
  - handlers.RateGate with the vote, vote change and network backstop buckets, VoteMiddleware on all five vote routes
  - one 429 shape (writeRateLimited, RateLimitResponse) and one block log line (logBlock)
  - api.RateLimits, api.RateLimitsDefault, Deps.RateLimits, Deps.ClientIPHeader, Deps.LimiterClock
  - the D-11 constants block in cmd/server/main.go and the CLIENT_IP_HEADER setting
  - newE2EServerWith and newGateTestServerWith tweak hooks with every bucket open by default
affects: [04-15, 04-19, 04-20, 04-27]

tech-stack:
  added: []
  patterns:
    - "Defaults resolved once in router.go (resolveRateLimits); the gate receives fully resolved limits"
    - "Cooldown reserved before the handler and refunded at the reservation instant on any non 2xx status"
    - "Backstop skipped, never shared and never the peer address, when the client IP is unknown"

key-files:
  created:
    - internal/service/ratecopy.go
    - internal/service/ratecopy_test.go
    - internal/api/handlers/ratelimit_gate.go
    - internal/api/handlers/votes_ratelimit_e2e_test.go
    - internal/api/ip_backstop_test.go
  modified:
    - internal/api/router.go
    - cmd/server/main.go
    - internal/api/handlers/reports_e2e_test.go
    - internal/api/handlers/trust_e2e_test.go
    - internal/api/gate_test.go

key-decisions:
  - "Limit types are declared in handlers and aliased in api (type BucketLimit = handlers.BucketLimit), because api imports handlers"
  - "VoteMiddleware order is backstop, vote bucket, cooldown; a vote bucket refusal never creates a cooldown entry and a cooldown refusal still spends its vote token"
  - "CLIENT_IP_HEADER is validated at startup: letters, digits and hyphens only, else log.Fatalf"

requirements-completed: [ROBUST-04]
duration: about 40 min
completed: 2026-10-16
---

# Phase 4 Plan 09: Rate limit gate on the five vote routes Summary

Per account vote throttle (20 quickly, then 1 every 15 seconds) plus a 30 second same-kind cooldown per report, with an exact 429 that names the wait, built on `ratelimit.PerKey`, from one constants block; the network backstop is off unless `CLIENT_IP_HEADER` names a header.

## What was built

- `internal/service/ratecopy.go`: `WaitPhrase` (seconds below 60, minutes rounded up below 3600 seconds, hours after, 59.5 seconds is "1 minute", 3599 seconds is "60 minutes") and `ReportLimitMessage`, `VoteLimitMessage`, `VoteChangeLimitMessage`, `FlagLimitMessage`. All sentences are the UI-SPEC sentences in the closed character set; the serious report message carries no emergency sentence.
- `internal/api/handlers/ratelimit_gate.go`: `BucketLimit`, `RateLimits`, `RateLimitResponse`, `RateGate`, `NewRateGate`, `VoteMiddleware`, `reserveIPBackstop`, `writeRateLimited`, `logBlock`.
- `internal/api/router.go`: aliases, `RateLimitsDefault`, `resolveRateLimits`, three new `Deps` fields, the client IP middleware switch, one `rateGate`, and the five vote routes wrapped (confirm, dispute: content kind; resolve, reopen, unresolve: resolution kind).
- `cmd/server/main.go`: the fourteen D-11 constants, `RateLimits` and `ClientIPHeader` in the `deps` literal, `CLIENT_IP_HEADER` read and validated.

## Settled points as implemented

1. Types declared in `handlers`, aliased in `api` (router.go).
2. `resolveRateLimits` fills every zero field from `RateLimitsDefault` once in `NewRouter`.
3. Constructor: `func NewRateGate(limits RateLimits, clock func() time.Time, clientIPHeader string, chrome Chrome) *RateGate`; built once in `NewRouter` as `rateGate`. Bucket fields: `vote`, `voteChange`, `ipBackstop` (all `*ratelimit.PerKey`), plus `backstopOn bool` and `chrome Chrome`. 04-15 adds the report and submit attempt buckets and 04-20 the flag bucket inside the constructor.
4. Order in `VoteMiddleware`: backstop, vote bucket, cooldown.
5. Cooldown key: `acct:<id>:rpt:<reportID>:<kind>` with the id parsed (`strconv.ParseInt`, base 10, 64 bit, above 0) and printed again; an unparseable id skips the cooldown and the handler answers 400.
6. The cooldown `Cancel` runs in a deferred function when the wrapped status (`middleware.NewWrapResponseWriter`) is outside 200 to 299, status 0 and panics included.
7. `WaitPhrase` rounds to whole seconds first (minimum 1).
8. `CLIENT_IP_HEADER` trimmed; any character outside ASCII letters, digits and hyphen stops startup with `log.Fatalf`; unset logs one WARNING.
9. Missing account in `VoteMiddleware` answers `http.Error(w, "internal server error", 500)`.
10. Both harnesses open all seven buckets with `BucketLimit{Every: time.Millisecond, Burst: 1000}`; the request link limiter is untouched in both.

Block log format string (04-15 step 0 reads this): `handlers: rate limit: bucket=%s account=%d wait_seconds=%d` (constant `rateBlockLogFormat`). Backstop helper: `(g *RateGate) reserveIPBackstop(r *http.Request) ratelimit.Decision`. Constants: `voteBucket`, `voteChangeBucket`, `ipBackstopBucket`, `voteRateField` (`rate_vote`), `voteChangeRateField` (`rate_vote_change`).

## Step 0 results

- All cross-phase and same-phase prerequisites found: `ErrVoterTooFar` (geo.go, mapped to 403 field `location` in votes.go), `newTrustE2EServer`, `NewVotingService`, `newVerifiedClient`, `submitReportGetID`, `floodReportBody`, `sessionIDFromJar`, `newGateJarClient`, `gateVerifySession`, `gateSessionIDFromJar`, `assertServerCopy`, `NewPerKey`/`Decision`/`Reserve`, `Chrome`, and the `page`/`auth` copies in the router. All name collision greps printed nothing.
- Router builders (test files calling `NewRouter(`): `internal/api/request_log_test.go`, `internal/api/gate_test.go`, `handlers/auth_e2e_test.go`, `handlers/trust_e2e_test.go`, `handlers/reports_e2e_test.go`, `handlers/votes_body_test.go` (builds its own chi router around `CastVote`, no gate), `handlers/swagger_test.go`. Of these only `trust_e2e_test.go` casts repeated or switched votes from one account; `request_log_test.go` sets no `Votes` and casts none. No fallback builder was needed.
- Baselines: lines containing `"/api/reports/{id}/` in router.go: 5 (the plan text said 4 when written, the fifth is the Phase 3.1 unresolve route); non comment `r.With(` lines: 2 before, 7 after; vote limit sentence in non test code under `internal/api`: 0 before and after.
- Swag baseline: I did not run it before editing. After the change a fresh `swag init` into the scratchpad gave `swagger.json` and `swagger.yaml` byte identical to `docs/`, and `docs.go` identical apart from the Go package name taken from the output directory (`p09docs` versus `docs`), so the published docs are unchanged until 04-15.

## Verification

- Copy: `go test ./internal/service/ -run 'TestWaitPhrase|TestServerCopy' -count=1 -v` passes `TestWaitPhrase`, `TestServerCopyRateLimits`, `TestServerCopyCheckerRejectsOutsideCharacters` and `TestServerCopyRejections`.
- Database `p4_0409` (created fresh, dropped after): `go test ./internal/api/handlers/ -race -run TestVoteRateLimit -count=1 -p 1 -v` passes all four tests with no data race. `go test ./internal/api/ -race -run 'TestIPBackstop|TestRequestLinkLimiterIgnoresSpoofedProxyHeaders' -count=1 -p 1 -v` passes `TestIPBackstop` with its four subtests and the spoof test.
- Whole tree without a database: `go test ./... -count=1` ok. Whole database form: `go test ./internal/... ./web/ -count=1 -p 1 -v` with `DATABASE_URL` set to `p4_0409`: every package ok, 0 FAIL lines, 0 `--- SKIP` lines.
- Mutation check: with the cooldown refund condition disabled for a moment, `TestVoteRateLimitChangeCooldownRefundedOnRefusal` failed ("far confirm = 429, want 403"); the file was restored from a copy before the commit.
- No U+2013 or U+2014 in any new file or added line; `perip.go`, `votes.go`, `swagger_test.go`, `docs/`, `go.mod` and `go.sum` untouched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Process] RED state for the vote tests was not observed**
- **Found during:** Task 1 Step 2
- **Issue:** I wrote the gate and router wiring (Step 3) before writing the vote tests (Step 2), so the four vote tests were never seen failing for the "no limit yet" reason. The commit boundary was unaffected (the plan commits the vote slice once).
- **Fix:** replaced with the mutation check above, which shows the refund test does catch a missing refund.

**2. [Rule 3 - Blocking] Verify commands adapted to the sandbox**
- **Issue:** the sandbox refuses `env -u DATABASE_URL`, `mktemp` subshell forms and long compound lines, and the executor rules say to set `DATABASE_URL` inline. `DATABASE_URL` is empty in this shell, so the no database runs simply omit it. The swag comparison was run into a scratchpad directory in separate commands.
- **Files modified:** none.

No fallback builder was opened and no file outside the plan's `files_modified` was edited.

## Known Stubs

None.

## Threat Flags

None. The only new trust boundary is the one the plan's threat model lists (T-04-09, T-04-10, T-04-04): `CLIENT_IP_HEADER` is validated at startup, the backstop skips unknown addresses, and block lines carry only a bucket name, a numeric account id and a wait.

## For the orchestrator

- Browser harness checks that change by design: none from this plan (no route, template, CSS or JS change). A vote that is throttled now answers 429 with a JSON body, which 04-19 will show in `.vote-error`; until then the vote buttons show whatever the existing generic error path shows.
- The `CLIENT_IP_HEADER` canary check is a human check for the 04-27 launch checklist; I did not run it.
- Commits: 185ff1a (copy RED), c04d6f9 (copy GREEN), 469b4ba (vote slice), 454312b (network backstop). The SUMMARY commit follows.

## Self-Check: PASSED

All five new files and five modified files exist, commits 185ff1a, c04d6f9, 469b4ba and 454312b are in the log, the final database run had 0 FAIL and 0 SKIP lines.
