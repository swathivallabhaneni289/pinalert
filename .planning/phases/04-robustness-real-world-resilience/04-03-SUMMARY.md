---
phase: 04-robustness-real-world-resilience
plan: 03
subsystem: ratelimit
tags: [rate-limit, token-bucket, tdd, robust-04]
requires: []
provides:
  - "ratelimit.PerKey keyed token bucket (Config, Decision, NewPerKey, Reserve) with whole second waits, a real refund and a per bucket idle window"
affects: [04-09, 04-15, 04-20]
tech-stack:
  added: []
  patterns: ["wrap golang.org/x/time/rate", "injected clock", "refund at the reservation instant in sync.Once"]
key-files:
  created:
    - internal/ratelimit/perkey.go
    - internal/ratelimit/perkey_test.go
  modified: []
key-decisions:
  - "Refund cancels at the captured reservation instant inside sync.Once (verified correction 1)"
  - "Idle window is the larger of IdleWindow and Burst * Every (verified correction 3)"
  - "Store is in memory; a restart forgets buckets, stated in the PerKey doc comment"
metrics:
  duration: "about 25 minutes"
  completed: 2026-10-16
status: complete
---

# Phase 4 Plan 03: PerKey keyed limiter Summary

A keyed token bucket over golang.org/x/time/rate that admits a burst, tells the wait in whole seconds, refunds a token for real and never evicts a bucket before it would have refilled.

## Precondition check

`go.mod` pins `golang.org/x/time v0.16.0`, the version whose `CancelAt` the plan read. `internal/ratelimit/` held only `perip.go` and `perip_test.go`. Both facts matched, so the plan ran as written.

## RED and GREEN

**Task 1 (reserve, wait, refund)**
- RED: all seven tests written; `perkey.go` had the refund closure cancel at a later clock read. Quoted run: `--- FAIL: TestPerKeyRefundAtReservationInstant` with `perkey_test.go:176: reservation 1 of 1 for "k" was refused (RetryAfter 25s), want admitted` and `perkey_test.go:186: ... (RetryAfter 9s), want admitted`. The other six passed.
- GREEN: the closure captures `now` once and calls `r.CancelAt(now)` inside `sync.Once`, with a comment saying why. All seven pass.

**Task 2 (idle window, sweep, concurrency)**
- RED: added `TestPerKeyIdleWindowNeverEarly` and `TestPerKeyConcurrentBound`, plus `sweep` and `entryCount` with the window taken as configured. Quoted run: `--- FAIL: TestPerKeyIdleWindowNeverEarly` (for example `serious: idleWindow = 0s, want 25m0s`). `TestPerKeyConcurrentBound` already passed in RED, because it pins Task 1's `Reserve` under concurrency.
- GREEN: `effectiveIdleWindow`, `sweepLoop` (ticker on the effective window, evicting against the injected clock) and `go p.sweepLoop()` started after the two validation panics.

No REFACTOR commit.

## Commits

- ee7705c test(04-03): add failing PerKey reserve and refund tests
- 031f6ea feat(04-03): PerKey reserve with a refund at the reservation instant
- 3315dfa test(04-03): add failing PerKey idle window and concurrency tests
- 188903c feat(04-03): PerKey idle window per bucket and background sweep

Each commit touches only `internal/ratelimit/perkey.go` and/or `internal/ratelimit/perkey_test.go` and carries the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` as the orchestrator's rules file requires.

## V-08 evidence

`go test ./internal/ratelimit/ -race -count=1 -v`:

```
--- PASS: TestAllowsBurstThenRefuses
--- PASS: TestRefillAfterInterval
--- PASS: TestIndependentKeysHaveIndependentBudgets
--- PASS: TestConcurrentAllowDoesNotRace
--- PASS: TestIdleEntryIsEvicted
--- PASS: TestBackgroundSweepEventuallyEvicts
--- PASS: TestMiddlewareRefusesBeyondBurst
--- PASS: TestMiddlewareFallsBackToRawRemoteAddr
--- PASS: TestMiddlewareIgnoresForgedForwardedHeaders
--- PASS: TestPerKeyBurstThenRefuses
--- PASS: TestPerKeyRefillOnInjectedClock
--- PASS: TestPerKeyIndependentKeys
--- PASS: TestPerKeyRefundAtReservationInstant
--- PASS: TestPerKeyInterleavedRefundIsLost
--- PASS: TestNewPerKeyPanicsOnInvalidConfig
--- PASS: TestPerKeyNilNowUsesWallClock
--- PASS: TestPerKeyIdleWindowNeverEarly
--- PASS: TestPerKeyConcurrentBound
ok  	pinalert/internal/ratelimit
```

No `WARNING: DATA RACE`. The PerKey tests also passed `-race -count=30`. `go test ./... -count=1` passed for every package. DATABASE_URL was not set and no database was used, so there are 0 database tests and no SKIP accounting applies (the disposable database `p4_0403` was never created).

## Deviations from Plan

- The plan's verify commands use `env -u DATABASE_URL`; the owner rules say the sandbox refuses that wrapper, so plain `go test` with the variable unset was used. The scripted automated verify blocks were not run verbatim; their checks were done by hand instead: gofmt clean, `go vet` clean, `package ratelimit` first line, `CancelAt(now)` twice, `time.Now` once, no `const`, no U+2014 or U+2013, `perip.go`, `perip_test.go`, `go.mod`, `go.sum` untouched.
- The plan's commit examples name the trailer `Claude Sonnet 5.5`; the rules file says its trailer (`Claude Opus 5.5`) overrides, so that one was used.
- Test file adds two small helpers beyond the listed ones (`perKeyRefuse`, and `perKeyAdmit` returns the decisions). No behaviour change.

## Known Stubs

None.

## Threat Flags

None. `PerKey` reads no request, logs nothing and has no network address fallback.

## Self-Check: PASSED

Both files exist, all four commits exist on the worktree branch, and the full ratelimit package passes under `-race`.
