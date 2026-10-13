---
quick_id: 261013-ubs
type: quick
description: Fix the red GitHub CI cross by truncating the cooldown test time to microseconds
created: 2026-10-13
autonomous: true
files_modified:
  - internal/store/cooldown_test.go
must_haves:
  truths:
    - "TestClaimEmailCooldownAllowsAfterWindow passes on Linux, where time.Now() has nanosecond precision, as well as on macOS"
    - "No application code changes, only the test"
    - "The whole suite is also run with TZ=UTC and, when Docker is available, on Linux against Postgres 16 like the CI job, before main is pushed"
  artifacts:
    - path: "internal/store/cooldown_test.go"
---

# Quick task 261013-ubs: fix the red GitHub CI cross

## Objective

GitHub Actions has failed on every push to `main` since 2026-09-12. The owner saw the red cross on the repository page on
2026-10-13 and asked for it to be fixed and for `main` to be pushed.

## Why it fails

The only failing test is `TestClaimEmailCooldownAllowsAfterWindow` in `internal/store/cooldown_test.go` (plan 01.1-08,
commit a3fd449). It stores `time.Now()` and compares the value Postgres returns, which has microsecond precision, with the
value it sent. On the owner's Mac the clock has microsecond precision, so it passes. On GitHub's Linux runners the clock has
nanoseconds, so `got.Equal(secondRequestedAt)` is false. Build and vet pass and so does every other test.

## Change

One line, test only: `firstRequestedAt := time.Now().Truncate(time.Microsecond)`, the same pattern `identity_test.go`
already uses. `secondRequestedAt` is derived from it by adding a minute, so it is truncated too.

## Proof

1. The test passes on a fresh scratch database (never pinalert_test, pinalert or p3_view).
2. The whole suite passes on a fresh scratch database with `TZ=UTC`, to catch other tests that depend on the Mac's time zone.
3. When Docker is available, the whole suite passes on Linux with `postgres:16`, the same service and flags as `ci.yml`.
4. `go vet ./...` and `gofmt -l` are clean.
5. After the push, the next GitHub run on `main` is green.
