---
phase: 07-address-search-box-for-report-location
plan: 01
subsystem: api
tags: [go, geocoding, nominatim, rate-limiting, http-proxy]

# Dependency graph
requires: []
provides:
  - "internal/geocode.Client: rate-limited, identified Nominatim /search proxy client"
  - "internal/api/handlers.Geocode: GET /api/geocode handler, unrouted (route registration is plan 07-03)"
  - "internal/api/handlers.GeocodeSearcher interface: the seam plan 07-03's Deps.Geocode wires against"
affects: [07-02-address-search-box-for-report-location, 07-03-address-search-box-for-report-location, 07-04-address-search-box-for-report-location]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Standalone internal/geocode package with no dependency on internal/service or internal/api, mirroring internal/ratelimit's shape"
    - "rate.Limiter.Wait(ctx) with a bounded sub-timeout, never Allow(), for a shared global outbound budget"
    - "Handler-local response structs (GeocodeResult/GeocodeResponse) mapped field-by-field from a service-layer type, never re-marshalled, matching CastVoteResponse's precedent"
    - "Single-method interface (GeocodeSearcher) as the seam for a handler's first third-party-network dependency, enabling upstream-failure unit tests with no live service"

key-files:
  created:
    - internal/geocode/client.go
    - internal/geocode/client_test.go
    - internal/api/handlers/geocode.go
    - internal/api/handlers/geocode_test.go
  modified: []

key-decisions:
  - "userAgent() falls back to a hardcoded repo-URL identifier when contactEmail is empty, rather than emitting an empty parenthetical, so the feature can never be the reason the process fails to identify itself to Nominatim"
  - "Nil GeocodeSearcher degrades to the D-04 503 fallback rather than panicking, so a Deps{} literal that omits the field (e.g. an unrelated test) stays safe"

patterns-established:
  - "Outbound proxy to a third-party API: fixed base-URL constant, url.Values.Encode() only, process-wide rate.Limiter.Wait with a bounded sub-context, unexported response-shape struct that never reaches the caller"

requirements-completed: [D-02, D-04]

coverage:
  - id: D1
    description: "internal/geocode.Client.Search makes one rate-limited, identified GET to Nominatim's fixed /search URL and returns only {name, display_name, lat, lon} with numeric coordinates"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_SetsUserAgent"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_ForwardsOnlyQueryToFixedEndpoint"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_ParsesStringCoordinates"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_SkipsUnparseableRecords"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestNominatimSearchURLIsHTTPS"
        status: pass
    human_judgment: false
  - id: D2
    description: "A single process-wide limiter serializes concurrent Search calls to <=1/sec, and a wait exceeding 1500ms errors out rather than queueing without limit"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_SerializesConcurrentCalls"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_LimiterWaitTimeout"
        status: pass
    human_judgment: false
  - id: D3
    description: "Search returns a wrapped error (never panics) on upstream non-2xx status, malformed body, or an already-cancelled context"
    verification:
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_UpstreamNon200"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_MalformedBody"
        status: pass
      - kind: unit
        ref: "internal/geocode/client_test.go#TestClient_Search_ContextCancelled"
        status: pass
    human_judgment: false
  - id: D4
    description: "GET /api/geocode's handler refuses a q outside 3-200 UTF-8 runes before any outbound call, counting runes not bytes"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_ValidatesQueryLength"
        status: pass
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_CountsQueryLengthInRunes"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every failure path (upstream error, nil searcher dependency) maps to 503 plus D-04's exact fallback sentence, never a panic or a 500"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_UpstreamFailureReturnsFriendlyError"
        status: pass
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_NilSearcherReturnsFriendlyError"
        status: pass
    human_judgment: false
  - id: D6
    description: "A successful response allowlists to exactly {name, display_name, lat, lon} per result (never Nominatim's full record), serialises an empty result set as a non-null array, and sets a private Cache-Control"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_AllowlistsResponseFields"
        status: pass
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_EmptyResultsSerialiseAsArray"
        status: pass
      - kind: unit
        ref: "internal/api/handlers/geocode_test.go#TestGeocode_SetsPrivateCacheControl"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-29
status: complete
---

# Phase 07 Plan 01: Nominatim Proxy Client and Geocode Handler Summary

**A standalone `internal/geocode` package proxying Nominatim's `/search` under a process-wide 1 req/sec `rate.Limiter.Wait` ceiling with an identifying `Pinalert/` User-Agent, plus a thin `GET /api/geocode` handler that validates `q` in UTF-8 runes and allowlists the response to `{name, display_name, lat, lon}`.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-09-29
- **Tasks:** 2
- **Files modified:** 4 (all newly created)

## Accomplishments
- `internal/geocode.Client.Search(ctx, q)` — the package's single exported call site — waits on a process-wide `rate.Limiter` (never `Allow()`), bounded by a 1500ms sub-timeout, before issuing one GET to the fixed `https://nominatim.openstreetmap.org/search` constant with only `q`/`format`/`limit`/optional-`email` forwarded via `url.Values.Encode()`
- Every outbound request carries a `Pinalert/1.0 (+...)` User-Agent, with a hardcoded fallback identifier when no contact email is configured, so the process can never send Nominatim a stock header
- Nominatim's string-typed `lat`/`lon` are parsed to `float64`, with unparseable records skipped (never zero-filled) so a bad row can never become a 0,0 pin
- `handlers.Geocode(searcher GeocodeSearcher) http.HandlerFunc` validates `q` to 3-200 UTF-8 runes (`utf8.RuneCountInString`, not `len`) before any outbound call, maps every failure — including a nil `searcher` dependency — to 503 plus the exact D-04 sentence, and allowlists a successful response to `{name, display_name, lat, lon}` as a non-null array
- 17 new unit tests (10 in `internal/geocode`, 7 in `internal/api/handlers`) all green, none requiring Postgres or live network access

## Task Commits

1. **Task 1: internal/geocode package, the rate-limited Nominatim proxy client** - `e15f6a7` (feat)
2. **Task 2: GET /api/geocode handler, query validation and response allowlisting** - `d76e350` (feat)

_TDD note: both tasks carry `tdd="true"` in the plan, but each was committed as a single feat commit with its test file written and passing in the same commit — the plan's `<action>` block for each task specifies writing the client/handler and its full test file together as one unit, not a separate RED-then-GREEN commit sequence. All tests were green at commit time._

## Files Created/Modified
- `internal/geocode/client.go` - `Client`, `Result`, `nominatimRecord`, `NewClient`, `newClientWithBaseURL`, `Search`, `buildRequest`, `userAgent`, and the five package-level constants (`nominatimSearchURL`, `defaultLimiterEvery`, `defaultLimiterBurst`, `limiterWaitTimeout`, `httpClientTimeout`, `resultLimit`)
- `internal/geocode/client_test.go` - 10 tests against an `httptest.NewServer` fake upstream
- `internal/api/handlers/geocode.go` - `GeocodeSearcher`, `GeocodeResult`, `GeocodeResponse`, `Geocode`, `parseGeocodeQuery`, and the four package-level constants (`minGeocodeQueryRunes`, `maxGeocodeQueryRunes`, `geocodeUnavailableMessage`, `geocodeCacheControl`)
- `internal/api/handlers/geocode_test.go` - 7 tests, `package handlers`, driven by a local `stubSearcher`, no router and no database

## Decisions Made
- `userAgent()`'s fallback identifier is `"Pinalert/1.0 (+https://github.com/swathivallabhaneni289/pinalert)"` when `contactEmail` is empty — this exact string is a load-bearing detail for plan 07-03, which decides whether `NOMINATIM_CONTACT_EMAIL` is required or optional at deploy time
- A nil `GeocodeSearcher` is treated as a runtime-degradation case (503 + D-04 message), not a programming-error panic, unlike `CastVote`'s missing-account-context case — because a `Deps{}` literal omitting `Geocode` is a realistic test/deploy state, not solely a routing bug

## Deviations from Plan

None - plan executed exactly as written. All ten Task 1 tests and all seven Task 2 tests match the plan's `<behavior>` blocks by name and assertion; all listed acceptance-criteria greps pass, including the one `place_id`/`osm_id`/`place_rank`/`boundingbox` match in `client.go`, which is the plan-mandated documentation comment explaining why those fields are deliberately absent from `nominatimRecord`'s struct tags (not a struct tag itself — verified by inspection).

## Issues Encountered
- Running `go test ./... -short -count=1` with Go's default parallel-package scheduling produced two unrelated failures against the shared local `pinalert_test` Postgres instance: a `deadlock detected` transient in `internal/api/handlers` test setup, an index-plan-shape assertion in `internal/store` (`TestNearbyReportsUsesIndex`), and a vote-ordering assertion in `internal/store` (`TestCastVoteConcurrentSameAccountKeepsEveryRow`). Re-running with `-p 1` (serial package execution) produced a clean `ok` across all 11 packages, including `internal/geocode` and `internal/api/handlers` in isolation. This is pre-existing test-suite flakiness under concurrent DB access from parallel package test binaries sharing one database — not caused by this plan, which adds no route, modifies no existing file, and touches no Postgres table.

## Next Phase Readiness

**For plan 07-03 (route registration):** the exact wiring contract is:
```go
type GeocodeSearcher interface {
	Search(ctx context.Context, q string) ([]geocode.Result, error)
}
func Geocode(searcher GeocodeSearcher) http.HandlerFunc
```
`*geocode.Client` (via `geocode.NewClient(contactEmail string) *geocode.Client`) satisfies `GeocodeSearcher` unchanged — plan 07-03 wires `Deps.Geocode *geocode.Client` (or the interface type) into `handlers.Geocode(deps.Geocode)` and registers `GET /api/geocode` flat inside the existing gated `r.Group`.

**For plan 07-04 (client-side JS):** the exact response shape to parse is:
```json
{"results": [{"name": "Bengaluru", "display_name": "Bengaluru, Karnataka, India", "lat": 12.9767936, "lon": 77.5900820}]}
```
`results` is always present and is `[]` (never `null`) on a no-match search. `minGeocodeQueryRunes = 3` must match plan 07-04's `MIN_QUERY_RUNES` client-side constant exactly. `geocodeUnavailableMessage = "Search unavailable, try tapping the map instead."` must match plan 07-04's own client-side fallback sentence exactly, verbatim, so the visitor sees one wording regardless of whether the failure was server-side or in the browser's own fetch.

**RESEARCH.md assumptions A1-A4:** none contradicted in practice. A1 (debounce/UX tuning) and A3 (attribution) are out of this plan's scope (frontend, plans 07-02/07-04). A2 (optional `email` param reduces block risk) and A4 (`burst=1` is adequate) were both implemented exactly as the research recommended, with no observed reason to revisit either at unit-test time — real-traffic validation of A4 is out of scope for a unit-tested backend plan and would only surface during manual UAT in a later plan.

No blockers for plan 07-02 (frontend markup) or 07-03 (route registration); both can proceed independently against this plan's committed interface.

## Self-Check: PASSED

- FOUND: internal/geocode/client.go
- FOUND: internal/geocode/client_test.go
- FOUND: internal/api/handlers/geocode.go
- FOUND: internal/api/handlers/geocode_test.go
- FOUND: .planning/phases/07-address-search-box-for-report-location/07-01-SUMMARY.md
- FOUND commit: e15f6a7 (Task 1)
- FOUND commit: d76e350 (Task 2)
- FOUND commit: 45fe14c (SUMMARY.md)

---
*Phase: 07-address-search-box-for-report-location*
*Completed: 2026-09-29*
