---
phase: 09-followable-reporter-profiles
plan: 06
subsystem: service
tags: [follows, sweep, reliable-tag, go]
requires:
  - phase: 09-02
    provides: FolloweeIDsPage, FolloweeStanding, endFollowsOf, the two end reasons
  - phase: 09-03
    provides: LoadReporterSignals, SignalSource, ReporterTagQuerier
provides:
  - FollowSweep (NewFollowSweep, Run, SweepOnce, FollowSweepInterval of 5 minutes)
  - the sweep started with the server on the shared run context
affects: [09-14, 09-26]
tech-stack:
  added: []
  patterns: ["stdlib time.Ticker runner, first sweep at start", "every follow ending goes through endFollowsOf"]
key-files:
  created:
    - internal/service/followsweep.go
    - internal/service/followsweep_test.go
  modified:
    - cmd/server/main.go
key-decisions:
  - "FollowSweepQuerier embeds followEnder (EndFollowsOf and EndFollowPair) because endFollowsOf takes that interface."
patterns-established:
  - "A page failure is logged with page bounds and the sweep moves on; only a failed read of the next followee page stops it."
requirements-completed: [PROF-03]
status: complete
---

# Phase 9 Plan 06: Follow sweep Summary

A five minute stdlib ticker sweep ends the stored follows of anyone who stops being a Reliable reporter (reason tag_lost) or stops being followable (reason profile_off), started with the server whether or not phone alerts are configured.

## What was built

- `followsweep.go`: `FollowSweep.Run` sweeps once at start, then every `FollowSweepInterval`, and returns when its context is done (a cancelled context does no work). `SweepOnce` pages `FolloweeIDsPage` in batches of `MaxReporterAccountsPerRead` (200) by keyset, reads `FolloweeStanding` and one `LoadReporterSignals` per page, then ends follows only through `endFollowsOf`: not followable gives `profile_off`, followable but tag not Reliable gives `tag_lost`. A missing account row counts as not followable. Logs carry ids and counts only.
- `cmd/server/main.go`: `followSweep := service.NewFollowSweep(queries)`; `pushRunners` renamed `runners`, starts with `followSweep.Run`, the push runners are appended inside the pushOn branch. No SignalSource passed (plan 09-14 wires it).

## Tests

All five named tests pass and were seen with `-v`: TestFollowSweepKeepsReliable, TestFollowSweepEndsOnDecay (clock moved 120 days, 2 follows end tag_lost, rows remain, second sweep ends 0), TestFollowSweepEndsOnPrivate (private, pending username, deleted, each profile_off while still Reliable), TestFollowSweepPagesBatches (450 followees, pages starting 0, 200, 400, three tag reads), TestFollowSweepStopsOnCancel. Full run `go test ./internal/service/ ./cmd/... -p 1` on scratch database p9_06_a: all ok, 543 PASS lines, only TestScreeningFixture skipped. `go build ./...` and `go vet` clean. The database is dropped.

## Deviations from Plan

**1. [Rule 3 - blocking] FollowSweepQuerier embeds followEnder.** The plan listed `EndFollowsOf` alone, but `endFollowsOf` takes the `followEnder` interface, which also needs `EndFollowPair`, so the interface embeds `followEnder` (the fake querier in the test has a no-op `EndFollowPair`).

**2. Paging test uses a fake querier** (no database), as the plan says only the cancel test must; it always runs.

## Notes for later plans

- Plan 09-26 adds the queued note cleanup inside `endFollowsOf`; the sweep needs no change.
- Plan 09-14 passes `WithFollowSweepSignals(src)` once the ratings source exists.

## Known Stubs

None.

## Threat Flags

None. T-09-20, T-09-21, T-09-22 mitigated as planned.

## Commits

- 64a2adc7 feat(09-06): end follows when the followed person loses the tag
- 9396f727 feat(09-06): start the follow sweep with the server

## Self-Check: PASSED

followsweep.go, followsweep_test.go and both commits exist.
