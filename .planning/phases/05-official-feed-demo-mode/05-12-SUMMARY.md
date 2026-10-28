---
phase: 05-official-feed-demo-mode
plan: 12
subsystem: official-alerts
tags: [cron, refresh-endpoint, bearer-token, ticker, session-skip, github-actions, d-15]
status: complete

requires:
  - phase: 05-05
    provides: official.Refresher and RefreshIfDue (the one idempotent refresh, minimum gap, atomic claim)
  - phase: 05-08
    provides: official.Service, Kick, Refresher()
  - phase: 05-10
    provides: sessionSkipSet prefix rule set with an empty POST set
  - phase: 05-11
    provides: the push enqueue version hook wired in official.NewService
provides:
  - POST /internal/official/refresh, a bearer protected door for an external cron (handlers.OfficialRefresh)
  - official.Service.RefreshAll, Service.Run and TickerInterval (the 5 minute ticker while the host is awake)
  - official.Refresher.Feeds (registered feeds in a fixed order)
  - the first POST entry of sessionSkipSet, so cron calls write no sessions row
  - handlers.ValidateRefreshToken and main.go startup wiring for OFFICIAL_REFRESH_TOKEN
  - .github/workflows/official-refresh.yml (every 10 minutes plus manual dispatch)
affects: [05-14, 05-16, 05-20]

key-files:
  created:
    - internal/api/handlers/official_refresh.go
    - internal/api/handlers/official_refresh_test.go
    - internal/api/official_refresh_routes_test.go
    - .github/workflows/official-refresh.yml
  modified:
    - internal/official/service.go
    - internal/official/service_test.go
    - internal/official/refresher.go
    - internal/api/router.go
    - internal/api/gate_test.go
    - internal/api/pwa_routes_test.go
    - cmd/server/main.go

key-decisions:
  - "The per IP limiter wraps only a POST to a configured route. An unset token or a GET/HEAD/PUT/DELETE goes straight to the handler's 404, so a flood can never turn the hidden route into a 429 that proves it exists (T-05-64). A wrong token spends the budget, which slows guessing."
  - "Both sides of the token compare are SHA-256 hashed first, then compared with subtle.ConstantTimeCompare, so the token's length does not show in timing."
  - "The route is always registered (even with no token) so the route table and the POST skip set agree and TestPWASkipSetMatchesRoutes can walk it."
  - "A right token with no refresher behind it (deps.Official that does not implement RefreshAll) answers 503, never a panic."
  - "Any error word outside timeout, status, parse, anomaly, network is reported as network, so no feed or error text can reach the summary."

requirements-completed: [ALERT-04]

duration: about 1h 15m
completed: 2026-10-29
---

# Phase 5 Plan 12: The official feed refresh door Summary

**A bearer protected POST /internal/official/refresh for a GitHub Actions cron, a 5 minute ticker for the awake host, and the 10 minute workflow, all funnelling into the one RefreshIfDue, with no session row per cron call.**

## What was built

- **Endpoint** (`internal/api/handlers/official_refresh.go`): 404 byte equal to an unknown route when `OFFICIAL_REFRESH_TOKEN` is unset and for every method but POST (registered with `r.Handle`, so never a 405 or an Allow header). A POST needs `Authorization: Bearer <token>`; wrong, missing, other scheme, lowercase scheme, and near prefix tokens are 401. It reads no request data, runs `RefreshAll` under `RefreshDeadline = 55s`, and answers `{"feeds":[{feed,skipped,items,stored,error}]}` with closed words only and `Cache-Control: no-store`. No swag annotation (not part of the public API; the file says so).
- **Ticker** (`internal/official/service.go`): `RefreshAll` ranges over `Refresher.Feeds()` (SACHET, then GDACS, then others by name), each through `RefreshIfDue`; `Run` sweeps once at boot and then every `TickerInterval = 5 * time.Minute`, each sweep bounded by 55 seconds with a panic guard, ending when its context ends. The ticker factory is an unexported field so the test drives ticks through a channel.
- **Router** (`internal/api/router.go`): `Deps.OfficialRefreshToken`, `OfficialRefreshLimit = BucketLimit{Every: 10s, Burst: 6}`, the route outside the gate and before `requireVerifiedAccount`, `"/internal/official/refresh"` as the first entry of the POST skip set, and a dated doc paragraph ("Amended 2026-10-29 by Phase 5 plan 05-12 (D-15)").
- **Startup** (`cmd/server/main.go`): reads and trims `OFFICIAL_REFRESH_TOKEN`; under 32 characters is `log.Fatalf` with "OFFICIAL_REFRESH_TOKEN must be at least 32 characters" (the value is never printed); unset logs the warning; `go officialSvc.Run(context.Background())` starts beside the moderation worker.
- **Workflow** (`.github/workflows/official-refresh.yml`): `*/10 * * * *` plus manual dispatch, `permissions: {}`, runs only when `vars.PINALERT_BASE_URL` is set, one curl step (`--fail`, 3 retries 30 seconds apart, `--max-time 120`) reading the token and URL through a step `env:` block. No checkout step. The comments say it is best effort, the 60 day rule, the 30 minute alternative, and that the secret and the variable are the owner's to add.

## Live run record (one real SACHET refresh)

The sandbox refuses to start the server with `ENV=development` (see Deviations), so the live run went through the real router in process with the real `official.NewService` over the plan's own database p5_12 and a throwaway token generated inside the test process (never written anywhere). One call of the endpoint, as planned: one RSS fetch and 40 CAP fetches.

- Call 1, `POST /internal/official/refresh` with the bearer token: **200 in 2.03 s**, no Set-Cookie, body `{"feeds":[{"feed":"sachet","skipped":false,"items":100,"stored":40,"error":""}]}`.
- `official_feed_status` row for `sachet` after call 1: last_attempt_at set, last_success_at set, last_error null, consecutive_failures 0, items_seen 100. `official_alerts` held 40 rows.
- Sessions count: 0 before, 0 after call 1, 0 after call 2. The cron call wrote no sessions row.
- Call 2, immediately after: 200, `{"feeds":[{"feed":"sachet","skipped":true,"items":0,"stored":0,"error":""}]}`. The 4 minute minimum gap held; NDMA was not asked again.
- The push hook ran inside the stored versions (two "push alert queued ... recipients=0" lines), as wired by plan 05-11.

The scratch test file used for this was deleted before any commit.

## Tests

New, no database (handlers): `TestRefreshEndpointUnsetIs404`, `TestRefreshEndpointMethods`, `TestRefreshEndpointToken` (ten bad forms, then the right one), `TestRefreshEndpointSummaryWords`, `TestRefreshEndpointDeadline` (injected 50 ms deadline), plus `TestRefreshEndpointNoRefresher`, `TestRefreshEndpointEmptySummary`, `TestValidateRefreshToken`.

New, no database (official): `TestServiceRunTicks`, `TestRefreshAllRespectsGap`, `TestRefreshAllReportsFailures`.

New, database (api): `TestRefreshEndpointNoSession`, `TestRefreshEndpointRateLimit` (6 OK, 7th 429 with the standard message and no refresh, a GET from the same address still 404, another address 200, wrong tokens spend the budget), `TestRefreshEndpointUnsetStaysHidden` (20 POSTs, all 404, never 429), `TestRefreshEndpointIsRegistered`. A compile time `var _ handlers.OfficialRefresher = (*official.Service)(nil)` pins that the real service can serve the route, because every route test uses a fake.

### Amended tests (every old assertion kept)

- **TestPWASkipSetMatchesRoutes** (`internal/api/pwa_routes_test.go`), "Amended 2026-10-29 by plan 05-12 (D-15)": the POST set was asserted empty; it is now asserted to be exactly `["/internal/official/refresh"]` and that path must be a registered route. The prefixes stay exactly `["/demo"]` and every GET assertion is unchanged.
- **TestAccessGateExemptRoutes** (`internal/api/gate_test.go`), "Amended 2026-10-29 by plan 05-12 (D-15)": the test server now sets a fake refresher and a token; four rows were added at the end: POST with the token is 200 with no session, POST without it is the endpoint's own 401, GET is 404, and a server with no token answers 404. Every older row and loop is unchanged.
- TestSkipSessionFor was run unchanged (it uses its own local rule set) and passes.

### Full suite on p5_12

`go test ./... -p 1 -count=1 -v` with `DATABASE_URL` pointing at p5_12 only: **5060 `--- PASS` lines, 0 `--- FAIL`, 0 `^FAIL`, 3 `--- SKIP`** (TestGeocodeLive, TestOfficialLive, TestScreeningFixture; all allowed). Every package `ok`. No known flake needed a rerun. The Node tests were not run (this plan touches no JavaScript). `go build ./...` and `go vet` on the touched packages are clean; `gofmt -l` lists only the pre-existing `internal/api/handlers/place_test.go`.

p5_12 was created, migrated with goose to version 13, and dropped; `psql -lqt` no longer lists it. Nothing listens on 8794 (no server was ever started).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `Refresher.Feeds()` added in internal/official/refresher.go**
- **Found during:** Task 1
- **Issue:** `fetchers` is private to the refresher, and `RefreshAll` (in service.go) needs the registered feeds in a fixed order. refresher.go is not in the plan's `files_modified`.
- **Fix:** A 15 line `Feeds()` accessor (SACHET, GDACS, then others by name).
- **Files modified:** internal/official/refresher.go
- **Commit:** 10bc4b54

**2. [Rule 2 - Missing critical] Failure words and a nil refresher**
- **Found during:** Task 1
- **Issue:** `RefreshIfDue` can return an error with an empty `ErrorClass` (a status read or claim failure, an unknown feed), which the summary would show as a clean run; a right token with no refresher behind it would panic.
- **Fix:** `RefreshAll` fills `ErrorClass` from the error when it is empty; the handler maps any word outside the closed set to `network`; a nil refresher answers 503. Tests cover both.
- **Commit:** 10bc4b54

### Plan variations (not bugs)

- **Live run in process, not on port 8794.** The sandbox refuses a command that sets `ENV=development`, as an earlier executor found. As the project rules say, I invented no secret or mail key and covered the same checks in process with the real router and the real service over p5_12 (call, summary JSON, status row, sessions count, the gap). The one real SACHET refresh is exactly as allowed in the plan.
- **Startup rule checked through the helper, not the binary.** The same sandbox rule stopped a run of the built server with a 31 character token, and without `ENV=development` the server stops earlier on `SESSION_SECRET`. The rule lives in `handlers.ValidateRefreshToken` (tested: empty allowed, 31 characters refused with the exact message, message never holds the value) and main.go calls it with `log.Fatalf("%v", err)`. The first run of the real binary will exercise the fatal path.
- **Token compare hashes first.** Both sides are SHA-256 hashed and then compared with `subtle.ConstantTimeCompare`, so token length does not leak in timing. `grep -c 'subtle.ConstantTimeCompare'` still prints 1.
- **`OfficialRefreshLimit` is a `BucketLimit` value** (`{Every: 10s, Burst: 6}`), reusing the existing type rather than a new one.
- **Commit trailer.** The plan and project rules pin `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` and every commit uses it exactly, although the session's own attribution reminder names a different model.

## Auth gates

None. The owner-side setup (GitHub secret `OFFICIAL_REFRESH_TOKEN`, repository variable `PINALERT_BASE_URL`, the same token as a Render environment variable, and the 10 versus 30 minute choice) is documented in the plan's `user_setup` and in the workflow header; I set none of it.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-05-63 to T-05-67 are mitigated as specified and tested (token compare and 401, hidden 404, minimum gap plus limiter plus deadline, exact POST skip, token never logged or written; `git grep` for the throwaway value finds nothing and the workflow reads it from secrets). T-05-68 (cron disabled) stays accepted: the feed state ages on its own.

## Commits

- b84e3b66 test(05-12): add failing tests for the official refresh endpoint and ticker
- 10bc4b54 feat(05-12): protected official refresh endpoint and a 5 minute ticker
- ef231a4a test(05-12): add failing route tests for the official refresh door
- 2c365e64 feat(05-12): wire the official refresh door outside the gate, sessionless
- 0571f53c chore(05-12): add the 10 minute cron workflow for the official feed

TDD note: each test commit does not compile on its own (it names the code that the next commit adds), which is the RED gate; the `test(...)` then `feat(...)` order is in the log for both tasks.

## Self-Check: PASSED

- internal/api/handlers/official_refresh.go, official_refresh_test.go, internal/api/official_refresh_routes_test.go, .github/workflows/official-refresh.yml: present.
- The five commits above are in `git log`.
- Acceptance greps: ConstantTimeCompare 1, request body reads 0, RefreshTokenMinLen 1, RefreshDeadline 1, TickerInterval 1, route string in router.go 2, route line (637) before `requireVerifiedAccount` (648), the Amended paragraph in router.go 1, `Run(context.Background())` in main.go 2, workflow greps all as specified (secret 1, variable guard 1, dispatch 1, checkout 0, cron 1, permissions 1).
- No em or en dash in any added line; no image, no Python; STATE.md, ROADMAP.md and REQUIREMENTS.md untouched.
