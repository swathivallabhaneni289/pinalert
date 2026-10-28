---
phase: 05-official-feed-demo-mode
plan: 10
subsystem: demo-api
tags: [demo, sandbox, in-memory, session-skip, gate, rate-limit, strict-json]
status: complete

requires:
  - phase: 05-03
    provides: demo.Store, demo.World, the in memory querier, caps, ErrWorldFull
  - phase: 05-04
    provides: sessionSkip rule set (exact GET, exact POST, prefixes) and skipSessionFor
  - phase: 05-06
    provides: World.Seed/SeedSetup, PostPractice, Vote, Nearby, ApplyDue, PretendLat/PretendLon
  - phase: 05-08
    provides: the gated /api/official/alerts route the new gate rows keep gated
provides:
  - handlers.DemoRoutes, DemoConfig, DemoLimit, DemoVisitLimitDefault, DemoWriteLimitDefault
  - GET and POST /demo/api/v/{visit}/reports and POST .../reports/{id}/{confirm,dispute,resolve,reopen,unresolve}
  - Deps.Demo (nil builds a store seeded with demo.SeedSetup), the /demo mount in the exempt block, "/demo" in sessionSkipSet prefixes
  - shared helpers writeCastVoteError, castVoteResponse (votes.go), submitInputFrom, writeSubmitError, nearbyHandler (reports.go)
affects: [05-12, 05-13, 05-17, 05-19]

key-files:
  created:
    - internal/api/handlers/demo.go
    - internal/api/handlers/demo_test.go
    - internal/api/handlers/demo_e2e_test.go
    - internal/api/demo_session_test.go
  modified:
    - internal/api/handlers/votes.go
    - internal/api/handlers/reports.go
    - internal/api/router.go
    - internal/api/gate_test.go
    - internal/api/pwa_routes_test.go

key-decisions:
  - "Flag and Mute have no Demo routes and are hidden in the Demo; the five vote words are registered one by one, so any other word under a report is a plain 404 (pinned by TestDemoNoFlagOrMuteRoutes)."
  - "Every Demo vote is cast from the one pretend position; the body is decoded strictly but its coordinates are ignored, so the real 1 km rule applies to the pretend position."
  - "GET /demo/api/v/{visit}/reports runs the whole real GET /api/reports handler (nearbyHandler), so past, disputed, all, category, report= focus and the recent/cut objects all work in the Demo with the real sentences."
  - "Demo writes sit behind a per IP limiter (default burst 60, one per second); reads are not limited because a page polls while a practice report climbs."

requirements-completed: [ROBUST-07]

duration: ~75 min
completed: 2026-10-29
---

# Phase 5 Plan 10: The Demo API Summary

A visitor with no account can now drive a sandboxed copy of the report and vote API at `/demo/api/v/<visit>/...`: same JSON shapes and the same refusals as the real routes, run on the in memory world and the real services, writing nothing to any table and setting no cookie.

## What was built

- `internal/api/handlers/demo.go`: `DemoRoutes(DemoConfig)` returns a chi router for mounting at `/demo`. `demoWorld` validates the visit through `demo.Store.Get` and answers 404 on field `visit` with "This Demo has ended. Reload the page to start again." `demoNearby` applies due neighbour votes then runs the real feed handler; `demoSubmit` decodes `SubmitReportRequest` strictly (64 KiB cap), calls `World.PostPractice` and answers 201 (200 on a replay) with `reportToResponse`; `demoVote` decodes `CastVoteRequest` strictly (4 KiB cap), calls `World.Vote` from the pretend position and answers with `castVoteResponse` or `writeCastVoteError`. `demo.ErrWorldFull` answers 429 "This Demo is full. Reload the page to start again." No handler reads a session or an account. No swag annotation (same shapes as the documented routes, a sandbox).
- `votes.go` and `reports.go`: extractions only, no status, field or sentence changed. `writeCastVoteError(w, err)` and `castVoteResponse(res)` come out of `CastVote`. `submitInputFrom(req)` and `writeSubmitError(w, err)` come out of `SubmitReport`. `NearbyReports` is now `nearbyHandler(svc, viewer, nearby)` with the account lookup and `svc.Nearby` passed in; the Demo passes the visitor's synthetic account and `world.Nearby`.
- `router.go`: `Deps.Demo`; `r.Mount("/demo", ...)` in the exempt block (line 543, before `r.Use(requireVerifiedAccount`); `"/demo"` as the one entry of `sessionSkipSet`'s prefixes; a dated amendment paragraph in the `NewRouter` doc comment.

## Amended tests (dated notes in the files, every old case kept)

- `TestAccessGateExemptRoutes` (`internal/api/gate_test.go`): "Amended 2026-10-29 by plan 05-10 (D-04, owner 2026-10-17): the Demo opens without an account". Gained the row "demo feed" (GET /demo/api/v/<visit>/reports?lat=..., must answer 200 from a store created in the test) and a loop of negative rows with exact statuses: `/api/reports` 401, `/api/official/alerts` 401, `/alerts` 302.
- `TestPWASkipSetMatchesRoutes` (`internal/api/pwa_routes_test.go`): "Amended 2026-10-29 by plan 05-10": the combined "POST and prefixes are empty" check is split; the POST set is still asserted empty and the prefixes are asserted to be exactly `["/demo"]`. Every other assertion is unchanged.

## New tests

Handlers, no database: `TestDemoRoutesRejectBadVisit`, `TestDemoStrictJSON`, `TestDemoNoFlagOrMuteRoutes`, `TestDemoRateLimitPerIP`. Handlers e2e on a database: `TestDemoWritesNothing` (counts every table from `pg_tables`, asserts the status of each step, and asserts the practice report climbed to `confirmed_places >= IndependentAgreementThreshold` with `confirmed_by_people` after the store clock moved 20 s, so the in memory neighbours demonstrably ran), `TestDemoAPIShapesMatchReal` (key sets of the GET item, POST answer, confirm answer and list envelope against the real routes, fresh against fresh), `TestDemoRulesAreReal` (a seed report 1.1 to 4 km away gives 403 field `location` with `service.TooFarMessage()`; the visitor's own report gives 403 "You can't vote on your own report."; the third take back gives 409 "You have used both undos on this report."). `internal/api`: `TestDemoSessionSkip` (no cookie and no sessions row under /demo; `/demonstration`, `/demo/../api/reports`, `/Demo/api`, a double slash, HEAD and PUT each get a cookie and exactly one new row and none answers 200 with Demo data; driven in process so dot segments and double slashes arrive unnormalised).

## Reading of "confirm from 1.5 km"

The vote position is fixed at the pretend position, so "from 1.5 km" means a seed report that lies more than 1 km from it. The test picks a seed report with `distance_km` between 1.1 and 4 from the active feed. The Ended seed report is not used because the service checks expiry (409) before distance.

## TDD record

RED (commit 474ae59a): run on the disposable `p5_10` database before Task 2, `TestDemoSessionSkip`, `TestAccessGateExemptRoutes/demo_feed`, `TestPWASkipSetMatchesRoutes`, `TestDemoWritesNothing`, `TestDemoAPIShapesMatchReal`, `TestDemoRulesAreReal`, `TestDemoRoutesRejectBadVisit` (all 6 subtests), `TestDemoStrictJSON`, `TestDemoNoFlagOrMuteRoutes` and `TestDemoRateLimitPerIP` all failed. GREEN (commit 73babafa): all passed on the first run after the handlers landed. Task 3 needed no test adjustment.

## Verification

Full suite on `p5_10` (`go test ./... -p 1 -count=1 -v`): 4935 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL` lines, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed). Every package `ok`. The 12 named PASS lines for the new and amended tests are present. `go vet ./...` clean. No known flake (TestCastVoteConcurrentSameAccountKeepsEveryRow, TestVotedReportsByAccountUsesTheAccountIndex) failed, so no rerun was needed. Node tests were not run (no JavaScript changed). `p5_10` was dropped and `psql -lqt` confirms it is gone.

Acceptance greps: `r.Mount("/demo"` appears once at line 543 and `r.Use(requireVerifiedAccount` at line 602; `"/demo"` appears 3 times in router.go; `writeCastVoteError(` is defined once in votes.go and called from demo.go; the non comment session/account/flag/mute grep over demo.go prints 0; `@Router` prints 0; the lines this plan added contain no en or em dash (older lines in router.go have them and were left alone).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Stub types in the RED commit**
- **Found during:** Task 1
- **Issue:** The failing tests use `DemoRoutes`, `DemoConfig`, `DemoLimit` and `Deps.Demo`, which do not exist yet; `go vet ./internal/api/...` (the Task 1 verify) would not compile.
- **Fix:** The test commit carries a stub `DemoRoutes` returning an empty router, the config types and the unused `Deps.Demo` field, so the tests compile and fail at run time. Task 2 replaced the stub.
- **Files modified:** internal/api/handlers/demo.go, internal/api/router.go
- **Commit:** 474ae59a

**2. [Rule 3 - Design] nearbyHandler instead of parseNearbyQuery**
- **Found during:** Task 2
- **Issue:** The plan allows extracting `parseNearbyQuery` for the feed query. But the real feed handler also serves the past view (History), the report link (Focus) and the empty tab counts, which `World.Nearby` alone cannot answer. Returning active data labelled `past` would be dishonest.
- **Fix:** The whole handler body became `nearbyHandler(svc, viewer, nearby)`, shared by the real route and the Demo. The Demo passes `world.Reports`, the visitor's synthetic account and `world.Nearby`. Validation order, messages and the `Cache-Control: no-store` header are unchanged (nearby_validation_test.go passes). `submitInputFrom` returns only the `SubmitInput` (the plan's `*paramProblem` return had nothing to report; validation stays in `ValidateSubmitInput`).
- **Files modified:** internal/api/handlers/reports.go
- **Commit:** 73babafa

**3. [Rule 4 note, decided here] Vote body coordinates ignored**
- The plan says the pretend position is sent on every Demo vote (wave note 05-06). `demoVote` decodes the body strictly and then casts from `demo.PretendLat` and `demo.PretendLon` whatever the body says. The Demo page need not send coordinates, a stray body cannot place the visitor near a far report, and the real 1 km rule is still enforced by the real service.

### Not done

- **The optional curl walk on port 8793.** The sandbox refused to launch the server with `ENV=development`, and the server refuses to start without it unless a mail provider key is set (`RESEND_API_KEY must be set outside ENV=development`). I did not invent a key. The two checks the plan wanted (an unknown visit answers 404 with the ended sentence and no Set-Cookie; `/api/reports` without a session answers 401) are covered in process by `TestDemoRoutesRejectBadVisit`, `TestDemoSessionSkip` and the `TestAccessGateExemptRoutes` rows. No server was left running; the built binary was deleted.

## Notes for later plans

- `expected_account_id` in a Demo post body is accepted and ignored (the device queue is not live in the Demo).
- Plan 05-12 adds the POST entry to `sessionSkipSet`; `TestPWASkipSetMatchesRoutes` still asserts the POST set empty and will need its own dated amendment then.
- `chi` `Mount("/demo", ...)` claims `/demo`, `/demo/` and `/demo/*`. Plan 05-13's `GET /demo` page and the later page routes must be registered inside `DemoRoutes` (or the mount changed), not as a second top level `r.Get("/demo")`.
- `DemoConfig.VisitLimit` exists for plan 05-13's page; `DemoRoutes` does not read it.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-05-51 to T-05-56 are the mitigations built here).

## Self-Check: PASSED

- internal/api/handlers/demo.go, demo_test.go, demo_e2e_test.go and internal/api/demo_session_test.go exist.
- Commits 474ae59a (test) and 73babafa (feat) exist on the worktree branch.
