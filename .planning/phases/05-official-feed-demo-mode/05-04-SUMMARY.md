---
phase: 05-official-feed-demo-mode
plan: 04
subsystem: api
tags: [go, chi, session, nasa-gibs, globe, memory-cache, ssrf]

requires:
  - phase: 03.3-installable-app-and-phone-alerts
    provides: skipSessionFor, pwaSkipSet and handlers.PWAPublicPaths (the session bypass this plan generalises)
provides:
  - handlers.GlobeTilePaths, GlobeTilesConfig, NewGlobeTiles and GlobeTiles.Handler (85 Earth tiles served from memory)
  - sessionSkip rule set (exact GET paths, exact POST paths, prefixes) and sessionSkipSet in router.go
  - 85 exact GET routes /globe/tiles/{z}/{y}/{x}.jpg in the exempt block, outside the login gate
  - Deps.GlobeUpstream and Deps.GlobeUserAgent
affects: [05-07 globe page, 05-10 Demo prefix, 05-12 refresh endpoint, 05-20 Policy page GIBS sentence]

tech-stack:
  added: []
  patterns:
    - "One sessionless rule set (sessionSkip) that every public route of Phase 5 joins instead of a second bypass"
    - "In package single flight (mutex plus map of in flight calls) in front of a sync.Map memory copy"

key-files:
  created:
    - internal/api/handlers/globe.go
    - internal/api/handlers/globe_test.go
    - internal/api/globe_routes_test.go
  modified:
    - internal/api/router.go
    - internal/api/pwa_routes_test.go
    - internal/api/gate_test.go
    - cmd/server/main.go

key-decisions:
  - "Kept the server copy of the NASA tiles (research G6 recommendation): NASA data policy is CC0 unless marked, reproduction without further permission, NASA acknowledged; the GIBS no-store header is a cache directive, not a licence term. Memory only, never on disk, never in the repo."
  - "Redirects are refused whatever client is passed in (CheckRedirect is forced), so a supplied client cannot reopen an SSRF hole."
  - "A failed tile answers 404 with Cache-Control: no-store, so a browser asks again once the upstream is back instead of caching the miss."
  - "main.go reads the raw BASE_URL variable for the User-Agent, not the baseURL variable, because that one defaults to http://localhost:PORT when unset; this follows 'whichever parts are set'."

patterns-established:
  - "sessionSkip.matches is the single strict matcher: empty RawPath, no //, no /./, no /../, no trailing /. or /.., GET or POST only"

requirements-completed: [ROBUST-07]

duration: 35min
completed: 2026-10-28
status: complete
---

# Phase 5 Plan 04: Globe tiles from memory and one session skip rule set Summary

**The sign in globe's 85 NASA Blue Marble tiles are served from Pinalert's memory after one fetch each from the fixed GIBS host, with no session rows, and the 3.3 session bypass is now one strict rule set (exact GET, exact POST, prefixes) ready for the Demo and the refresh endpoint.**

## Accomplishments

- `internal/api/handlers/globe.go`: `GlobeTilePaths()` (85 paths, zoom 0 to 3, row then column), `NewGlobeTiles`, `Handler(path)`. Fail closed: constant https upstream host, redirects refused, 10 s timeout, 1 MB body limit, `image/jpeg` check, empty body refused, one fetch per tile at a time, failures remembered 60 s, 404 on failure, `Cache-Control: public, max-age=2592000`, `nosniff`, no cookie. Bodies live in a `sync.Map`; nothing is written to disk.
- `internal/api/router.go`: `sessionSkip{get, post, prefixes}` with `matches`, `sessionSkipSet()` (replaces `pwaSkipSet`; GET set is PWAPublicPaths plus GlobeTilePaths, POST set and prefixes empty for now), `skipSessionFor(sessionSkip, ...)`, the 85 exact GET routes in the exempt block from one `NewGlobeTiles` built in `NewRouter`, `Deps.GlobeUpstream` (tests only) and `Deps.GlobeUserAgent`, and the dated amendment paragraph in the `NewRouter` doc comment.
- `cmd/server/main.go`: only the User-Agent value is added, `Pinalert/1.0 (+<BASE_URL>; contact <CONTACT_EMAIL>)` with whichever parts are set, else the repository URL fallback the geocoder uses.
- Hand off for later plans: 05-10 adds the Demo prefix and 05-12 the refresh endpoint to `sessionSkipSet()`; `TestPWASkipSetMatchesRoutes` currently asserts the POST set and prefixes are empty and must be amended by whichever plan adds the first entry.

## Task Commits

1. Task 1 RED: `33d4dd2` test(05-04): add failing tests for the globe tile handler
2. Task 1 GREEN: `07a53fb` feat(05-04): serve the 85 globe tiles from a memory copy of NASA GIBS
3. Task 2 RED: `a014af0` test(05-04): add failing tests for the one session skip rule set and the 85 tile routes
4. Task 2 GREEN: `90d691d` feat(05-04): one session skip rule set and the 85 globe tile routes

## Amended tests (each carries an "Amended 2026-10-28 by plan 05-04" note)

- **TestSkipSessionFor** (`internal/api/pwa_routes_test.go`): now built with a `sessionSkip{get: /a, post: /q, prefixes: /p}`. All 7 original rows kept unchanged (exact GET skips, trailing slash, map suffix, other path, POST, HEAD, encoded spelling). 18 rows added: POST of an exact post path skips, GET of a post only path, POST of a get only path, prefix itself GET and POST, under prefix GET and POST, deeper under prefix, `/px`, `/p/../a`, `/p/./x`, `/p//x`, `/p/.`, `/p/..`, `/P/x`, HEAD under prefix, PUT under prefix, encoded spelling under prefix.
- **TestPWASkipSetMatchesRoutes** (`internal/api/pwa_routes_test.go`): now over `sessionSkipSet()`. Every old assertion about the ten PWA paths is kept; the GET set must equal PWAPublicPaths plus the 85 GlobeTilePaths, each tile is a registered GET route, and the POST set and prefixes are asserted empty.
- **TestAccessGateExemptRoutes** (`internal/api/gate_test.go`): server now built through `newGateTestServerWith` with the fake upstream; new row "globe tile" (GET /globe/tiles/0/0/0.jpg, must be 200); new near misses `/globe/tiles/9/0/0.jpg` and `/globe/tiles` (must not be 200). The doc comment's "seventeen routes" count is now eighteen in effect; the count word was left as written and the amendment note says what was added.
- TestPWAAssetsSetNoCookie is unchanged and passes.

## New tests

- Handlers (`globe_test.go`): TestGlobeTilePaths, TestGlobeTileFetchedOnce, TestGlobeTileRefusesBadUpstream (redirect, text/html, oversize, 500 and, beyond the plan, an empty body), TestGlobeTileFailureRemembered, TestGlobeTileHeaders, plus two beyond the plan: TestGlobeTileOversizeAtTheLimitIsKept (exactly 1 MB is accepted) and TestGlobeTileUpstreamIsTheNASAConstant.
- Router (`globe_routes_test.go`): TestGlobeTilePathsAreRoutes (no database) and TestGlobeTilesSetNoCookie (database; all 85 tiles leave the sessions count unchanged, and eight near misses each get a cookie and exactly one new row).

## Verification

- `go build ./...` and `go vet ./...` clean. gofmt reports only `internal/api/handlers/place_test.go` and `internal/geocode/photon.go`, both unformatted before this plan and left alone.
- Handler tests pass without a database (also with `-race`).
- On the disposable database p5_04 (created, migrated to version 11 with `go run ./cmd/migrate`, dropped at the end): TestGlobeTilesSetNoCookie, TestAccessGateExemptRoutes and TestPWAAssetsSetNoCookie pass with zero `--- SKIP` lines.
- Full suite on p5_04: `go test ./... -p 1 -count=1 -v` gave `ok` for all 16 packages with tests, 0 `--- FAIL` lines, 4686 PASS lines and exactly 2 `--- SKIP` lines, TestGeocodeLive and TestScreeningFixture (both allowed). TestCastVoteConcurrentSameAccountKeepsEveryRow did not flake, so no rerun was needed. p5_04 was dropped afterwards and confirmed gone.

## Deviations from Plan

None. The plan was executed as written. Two small additions inside the plan's intent: an empty upstream body is refused, and the 404 answer carries `Cache-Control: no-store`.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-05-17 to T-05-21 are mitigated as listed; T-05-22 is accepted as listed.

## Notes

- Not run by this plan: no server was started and NASA was never called; every test uses a generated fake upstream.
- An early shell command touched and removed a throwaway `/tmp/dummy` file outside the worktree; no repo or other data was affected.
