---
phase: 07-address-search-box-for-report-location
plan: 03
subsystem: api
tags: [go, chi, rate-limiting, swagger, geocoding]

# Dependency graph
requires:
  - "internal/geocode.Client (plan 07-01)"
  - "internal/api/handlers.Geocode, handlers.GeocodeSearcher (plan 07-01)"
provides:
  - "GET /api/geocode: live, gated, rate-limited route reachable by the frontend (plans 07-02, 07-04)"
  - "api.Deps.Geocode, api.Deps.GeocodeRateLimit, api.GeocodeRateLimit, api.GeocodeRateLimitDefault"
  - "docs/swagger.json /geocode: published API contract for the route"
affects: [07-02-address-search-box-for-report-location, 07-04-address-search-box-for-report-location]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-route r.With(limiter.Middleware()) scoping for a secondary anti-abuse budget, distinct from a global process-wide limiter living inside the third-party client package itself"
    - "Additive Deps struct fields with a zero-value-safe fallback constant, so every existing Deps{} literal keeps compiling and keeps its prior behavior"
    - "Warn-and-continue startup contract for secondary/convenience env vars, deliberately distinct from the fail-fast contract used for SESSION_SECRET/RESEND_API_KEY"

key-files:
  created: []
  modified:
    - internal/api/router.go
    - internal/api/gate_test.go
    - cmd/server/main.go
    - README.md
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - internal/api/handlers/swagger_test.go

key-decisions:
  - "GeocodeRateLimitDefault (burst 3, one token per 2s) is a fresh named constant, not a reuse of RequestLinkRateLimitDefault (burst 5, one token per 60s) — the two traffic shapes differ (bursty-within-one-submission vs. rare-across-a-session), per 07-RESEARCH.md Open Question 2"
  - "NOMINATIM_CONTACT_EMAIL follows a warn-and-continue startup contract, not the SESSION_SECRET/RESEND_API_KEY fail-fast pattern — address search is a secondary convenience whose whole design contract (D-04) is that its failure never blocks reporting or boot"
  - "geocode.NewClient(...) is constructed exactly once, in cmd/server/main.go, which is what makes the process-wide limiter inside internal/geocode genuinely process wide rather than per-request"

requirements-completed: [D-02, D-04]

coverage:
  - id: T1
    description: "GET /api/geocode is reachable only by a verified session; an unverified caller gets 401 with error.field == \"auth\" and no results key in the body"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/gate_test.go#TestAccessGateBlocksUnverifiedGeocode"
        status: pass
      - kind: manual
        ref: "curl smoke test, unverified request"
        status: pass
    human_judgment: false
  - id: T2
    description: "GET /api/geocode is registered as a flat literal path inside the gated r.Group, never under an r.Route wildcard mount"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "grep -c 'r.Route(' with comment lines excluded == 0; byte-offset source assertion"
        status: pass
    human_judgment: false
  - id: T3
    description: "The route carries its own per-IP token-bucket limiter (burst 3, one token per 2s), distinct from the request-link budget"
    requirement: "D-02"
    verification:
      - kind: manual
        ref: "curl smoke test: 6 rapid verified requests yielded 200,200,200,429,429,429"
        status: pass
    human_judgment: false
  - id: T4
    description: "A Deps literal that omits Geocode still compiles and still serves every other route"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/handlers/swagger_test.go newSwaggerTestRouter's Deps{} literal (no Geocode field set) compiles and TestSwaggerDocServed passes"
        status: pass
    human_judgment: false
  - id: T5
    description: "The server logs a warning and keeps running when NOMINATIM_CONTACT_EMAIL is unset; it never fails to boot"
    requirement: "D-04"
    verification:
      - kind: manual
        ref: "behavioral startup check with NOMINATIM_CONTACT_EMAIL= : warning printed, server reached \"pinalert listening on :8123\", did not exit"
        status: pass
    human_judgment: false
  - id: T6
    description: "docs/swagger.json documents GET /geocode with its 401/503 responses and GeocodeResponse, and documents no Nominatim-internal record field"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/handlers/swagger_test.go#TestSwaggerSpecCoversRoutes"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-09-29
status: complete
---

# Phase 07 Plan 03: Route Registration, Client Wiring, and Published Spec Summary

**`GET /api/geocode` is now live inside the verified-account gate at burst-3/2s per-IP throttling, backed by a single process-wide `geocode.Client` constructed from `NOMINATIM_CONTACT_EMAIL` in `cmd/server/main.go`, and published in `docs/swagger.json` with its own OPS-01 drift guard.**

## Performance

- **Duration:** ~30 min
- **Completed:** 2026-09-29
- **Tasks:** 3
- **Files modified:** 8 (all modifications to existing files; no new files created)

## Route Path and Budget

- Final route: `GET /api/geocode`, registered as a flat literal path inside the existing gated
  `r.Group` in `internal/api/router.go`, between the four vote routes and `GET /profile`.
- Per-IP budget that shipped: `GeocodeRateLimitDefault = GeocodeRateLimit{Burst: 3, Every: 2 *
  time.Second}` — a fresh named constant, distinct from `RequestLinkRateLimitDefault` (burst 5,
  every 60s). `cmd/server/main.go` wires this same shape via `geocodeRateLimitBurst = 3` /
  `geocodeRateLimitEvery = 2 * time.Second`.

## Accomplishments

- Added `Deps.Geocode handlers.GeocodeSearcher` and `Deps.GeocodeRateLimit GeocodeRateLimit`
  additive fields to `internal/api/router.go`'s `Deps` struct, each zero-value-safe exactly as
  `Votes` and `RequestLinkRateLimit` already were
- Declared `GeocodeRateLimit` struct and `GeocodeRateLimitDefault` var immediately after the
  existing `RequestLinkRateLimit`/`RequestLinkRateLimitDefault` pair, with a comment recording why
  the budget is a fresh shape rather than a reuse
- Registered `GET /api/geocode` as a flat literal path, gated, with `r.With(geocodeLimiter.Middleware())`
  scoping the per-IP limiter to this one route only — proven by a byte-offset source assertion
  (registration falls between the gate's `r.Use` line and `r.Post("/auth/logout"`) and by zero
  actual `r.Route(` calls in the file's code (two pre-existing matches are doc-comment prose
  quoting the rejected pattern for explanatory purposes — see Deviations)
- Added `TestAccessGateBlocksUnverifiedGeocode` to `internal/api/gate_test.go`, run against real
  Postgres: PASS (not SKIP) — 401, `error.field == "auth"`, no `"results"` substring in the body
- Constructed the real `geocode.Client` exactly once, in `cmd/server/main.go`, from
  `os.Getenv("NOMINATIM_CONTACT_EMAIL")`; the missing-variable path logs a warning and continues
  rather than `log.Fatal`, deliberately breaking from the `SESSION_SECRET`/`RESEND_API_KEY`
  fail-fast pattern because address search is a secondary convenience whose D-04 contract requires
  it to never block boot or reporting
- Documented `NOMINATIM_CONTACT_EMAIL` in `README.md`: what it's for, that it's optional, the
  warn-not-fail contract, and that there is no signup or API key for Nominatim
- Regenerated `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` via `make swag` (see
  Generator Used below), publishing `/geocode` with its `q` parameter and 200/400/401/429/503
  responses plus the `handlers.GeocodeResponse`/`handlers.GeocodeResult` definitions
- Extended `TestSwaggerSpecCoversRoutes` in `internal/api/handlers/swagger_test.go` with this
  plan's OPS-01 drift guard: asserts `/geocode`'s `get` operation carries `"401"`, `"503"`, and
  `"q"`, asserts `GeocodeResponse`/`display_name`/`lat`/`lon` are documented, and `t.Fatalf`s if
  `place_id`, `osm_id`, `place_rank`, or `boundingbox` ever appear anywhere in the published spec

## Task Commits

1. **Task 1: register GET /api/geocode inside the gate with its own per-IP limiter** - `6bfec1c` (feat)
2. **Task 2: construct the geocode client in cmd/server/main.go and document its environment variable** - `03e6702` (feat)
3. **Task 3: regenerate the published API spec and extend its drift guard** - `943fbfd` (docs)

## Files Modified

- `internal/api/router.go` - `Deps.Geocode`, `Deps.GeocodeRateLimit`, `GeocodeRateLimit` type,
  `GeocodeRateLimitDefault` var, the `GET /api/geocode` registration inside the gated group
- `internal/api/gate_test.go` - `TestAccessGateBlocksUnverifiedGeocode`
- `cmd/server/main.go` - `geocodeRateLimitBurst`/`geocodeRateLimitEvery` consts, the
  `NOMINATIM_CONTACT_EMAIL` read with warn-not-fail handling, the `Deps.Geocode`/
  `Deps.GeocodeRateLimit` wiring, the `pinalert/internal/geocode` import
- `README.md` - the `NOMINATIM_CONTACT_EMAIL` export line and its explanation paragraph
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` - regenerated by `make swag`
- `internal/api/handlers/swagger_test.go` - the `/geocode` drift-guard block in
  `TestSwaggerSpecCoversRoutes`

## Decisions Made

- Kept `GeocodeRateLimit`'s zero-value fallback shape identical to `RequestLinkRateLimit`'s
  established pattern (compare-to-zero-struct then substitute the default), rather than
  introducing a different idiom, so a future reader sees one consistent convention across both
  per-route limiters in this file
- Did not touch the two pre-existing `router.go` doc-comment lines that mention `r.Route(` as
  prose (explaining why that pattern is rejected at this prefix) even though a naive `grep -c
  'r.Route(' router.go` returns 2, not the plan's literal acceptance-criteria expectation of 0 —
  see Deviations for the substantiated proof that zero actual `r.Route(` calls exist in code

## Deviations from Plan

### Acceptance-criteria grep false positive (not a code deviation)

The plan's Task 1 acceptance criteria list `grep -c 'r.Route(' internal/api/router.go` equals 0
as a checkable claim. The literal grep returns 2, both pre-existing doc-comment lines (present
before this plan started, at what are now lines ~175 and ~212) that quote `r.Route("/api", ...)`
and `r.Route("/api/reports/{id}", ...)` in prose while explaining why this router never uses that
pattern — the same lines this plan's own `<read_first>` section (lines 139-159, 168-191 in the
pre-plan file) directed the executor to read for context. `git diff` confirms my edits never
touched those lines. The substantiated check —
`grep -vE '^\s*//' internal/api/router.go | grep -c 'r\.Route('` (comment lines excluded) — returns
0, and the byte-offset assertion the plan itself specifies as the authoritative proof (registration
falls between the gate's `r.Use` and `r.Post("/auth/logout"`) also passes. No code change was
needed; this is documented here so the discrepancy between the literal grep and the plan's intent
is traceable rather than silently glossed over.

No other deviations — plan executed exactly as written. All acceptance-criteria checks pass,
including every grep, the byte-offset assertion, the behavioral startup check, and the curl smoke
test.

## Generator Used

`swag` was found on `PATH` (`/Users/swathivallabhaneni/go/bin/swag`) and its version was
confirmed as `v1.16.4` — an exact match to `go.mod`'s pinned `github.com/swaggo/swag v1.16.4` —
before running `make swag` (`swag init -g internal/api/router.go -o docs`). The pinned `go run
github.com/swaggo/swag/cmd/swag@v1.16.4` fallback was not needed.

## Verification Results

- `go build ./...` and `go vet ./...`: exit 0.
- `go test ./... -short -count=1`: exit 0, all 11 testable packages `ok`.
- `DATABASE_URL=... go test ./internal/api/ -run 'TestAccessGate' -v -count=1`: exit 0,
  `TestAccessGateBlocksUnverifiedGeocode` reported **PASS**, not SKIP — `DATABASE_URL` was
  exported for the run, so the V4 access-gate proof is not vacuous.
- `go test ./internal/api/handlers/ -run 'TestSwagger' -v -count=1`: exit 0, both
  `TestSwaggerDocServed` and `TestSwaggerSpecCoversRoutes` PASS.
- `make test` (full suite against real Postgres, `-p 1`): exit 0, every package `ok`.
- Behavioral startup check: `NOMINATIM_CONTACT_EMAIL=` (empty) plus a real `DATABASE_URL`,
  `SESSION_SECRET`, `ENV=development`, `PORT=8123` — printed `WARNING: NOMINATIM_CONTACT_EMAIL
  unset; outbound geocoding User-Agent will fall back to the project repository URL` then
  `pinalert listening on :8123`, and did not exit. Server was stopped afterward; confirmed no
  lingering process.
- Curl smoke test (manual, per the plan's `<verification>` step 6), run against a local server on
  port 8123 (never 8080) with a real verified session obtained through the actual
  request-link-then-verify flow:
  - Verified session, `GET /api/geocode?q=bengaluru` → **200**, body
    `{"results":[{"name":"Bengaluru","display_name":"Bengaluru, Bangalore North, Bengaluru Urban, Karnataka, India","lat":12.9767936,"lon":77.590082}]}`
    — one result carrying exactly `name`, `display_name`, `lat`, `lon`.
  - No session cookie, same query → **401**, body
    `{"error":{"field":"auth","message":"Verify your email to continue."}}`.
  - Six rapid verified requests → `200, 200, 200, 429, 429, 429` — burst of 3 consumed
    immediately, then throttled, exactly matching `GeocodeRateLimitDefault`.

## Next Phase Readiness

Plan 07-04 (client-side JS) can now call the live `GET /api/geocode` endpoint end to end: a
verified session gets a real 200 with the documented `{results: [{name, display_name, lat, lon}]}`
shape (empty array, never null, on no match), an unverified session gets 401, and a burst beyond 3
requests within 2 seconds from one IP gets 429 with the standard `{error:{field,message}}`
envelope. No blockers for 07-04.

## Self-Check: PASSED

- FOUND: internal/api/router.go (modified)
- FOUND: internal/api/gate_test.go (modified)
- FOUND: cmd/server/main.go (modified)
- FOUND: README.md (modified)
- FOUND: docs/docs.go (modified)
- FOUND: docs/swagger.json (modified)
- FOUND: docs/swagger.yaml (modified)
- FOUND: internal/api/handlers/swagger_test.go (modified)
- FOUND commit: 6bfec1c (Task 1)
- FOUND commit: 03e6702 (Task 2)
- FOUND commit: 943fbfd (Task 3)

---
*Phase: 07-address-search-box-for-report-location*
*Completed: 2026-09-29*
