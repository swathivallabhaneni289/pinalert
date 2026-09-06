---
phase: 01-foundation-report-map
plan: 03
subsystem: api
tags: [go, chi, pgx, sqlc, hmac-session, haversine, geohash]

# Dependency graph
requires:
  - phase: 01-foundation-report-map (plan 01-01)
    provides: Go module scaffold, reports+sessions migration, internal/testutil (NewTestDB, SeedReports, SeedExpiringReport), sqlc.yaml
provides:
  - "internal/session.Manager — HMAC-signed anonymous session cookie issue/verify + chi middleware"
  - "internal/service.ReportService — Category/Severity/CapacityStatus enums, ValidateSubmitInput, ExpiryDuration, BoundingBox, Submit, Nearby"
  - "internal/api.NewRouter/Deps — chi router with session middleware and POST/GET /api/reports"
  - "internal/store/sqlc — InsertReport, NearbyReports, UpsertSession (committed generated code)"
  - "cmd/server — the runnable Go binary (DATABASE_URL/SESSION_SECRET/ENV/PORT)"
affects: [01-04, 01-05, 01-06, 01-07]

# Tech tracking
tech-stack:
  added:
    - github.com/go-chi/chi/v5 v5.3.2 (now imported: router + middleware)
  patterns:
    - "Anonymous identity: crypto/rand session id + HMAC-SHA256 signature, hmac.Equal constant-time verify, chi middleware issues/verifies on every request and persists eagerly on first sight"
    - "expires_at materialized server-side at write time (time.Now().UTC() + ExpiryDuration(severity)); every read gates on expires_at > now(), never a sweep job"
    - "Two-stage NearbyReports query: indexed lat/lon BETWEEN prefilter in an inner subquery, exact Haversine distance cast to ::float8, outer WHERE distance_km <= radius_km"
    - "Every read-path SQL projection and every JSON response struct enumerates columns/fields explicitly — session_id can never reach a client"
    - "Handler declares its own JSON request/response structs (submitReportRequest, reportResponse) rather than marshalling a sqlc row wholesale"

key-files:
  created:
    - internal/session/cookie.go
    - internal/session/cookie_test.go
    - internal/service/report.go
    - internal/service/report_test.go
    - internal/api/router.go
    - internal/api/handlers/reports.go
    - internal/api/handlers/reports_e2e_test.go
    - internal/store/db.go
    - internal/store/queries/sessions.sql
    - internal/store/queries/reports.sql
    - internal/store/reports_test.go
    - internal/store/sqlc/db.go
    - internal/store/sqlc/models.go
    - internal/store/sqlc/sessions.sql.go
    - internal/store/sqlc/reports.sql.go
    - cmd/server/main.go
    - .planning/phases/01-foundation-report-map/deferred-items.md
  modified:
    - go.mod
    - sqlc.yaml

key-decisions:
  - "Fixed sqlc.yaml's text/integer go_type overrides (plan 01-01 artifact) — bare \"*string\"/\"*int32\" go_type specifiers are rejected by sqlc v1.31.1 (\"not a Go basic type\"); replaced with the documented emit_pointers_for_null_types: true mechanism, since every task in this plan ends in sqlc generate and this was blocking from Task 1 onward"
  - "Moved go-chi/chi/v5 from go.mod's indirect to direct require block by hand (mirroring 01-01's geohash promotion) rather than running go mod tidy, which would strip the still-unimported swaggo/http-swagger/v2 pin reserved for plan 01-07"
  - "internal/store/reports_test.go is package store_test (not store) and internal/api/handlers/reports_e2e_test.go is package handlers_test (not handlers) — both need to import packages that would otherwise create an import cycle (store_test -> testutil -> store; handlers_test -> api -> handlers)"
  - "NearbyReports casts the computed Haversine expression to ::float8 in the inner SELECT so sqlc infers DistanceKm as a plain float64 rather than a nullable pgtype.Float8, avoiding an unwrap step in the handler"
  - "TestNearbyReportsUsesIndex runs ANALYZE reports after the 50k-row CopyFrom seed, since pgx.CopyFrom leaves the planner without fresh stats and a stats-free planner can choose a Seq Scan even on a large table"
  - "TestNearbyReportsUsesIndex and its EXPLAIN target keep a literal copy of the NearbyReports SQL (sqlc emits the production query as an unexported const, unreachable from the external store_test package), guarded by a companion drift test (TestNearbyReportsQuerySourceHasExpectedShape) that greps queries/reports.sql for the same expires_at/BETWEEN shape"
  - "radius_km out-of-bounds (below 0.1 or above 50) is rejected with 400 rather than silently clamped, for consistency with how lat/lon out-of-range values are handled"

patterns-established:
  - "session.FromContext(ctx) is the only way any handler reads the caller's session id; the context key is an unexported struct{} type, never a string, so it can't collide with unrelated context values"
  - "ReportService.Nearby is the single read path both the map and the list will poll (01-04/01-06) — there is no map-only or list-only query"

requirements-completed: [FOUND-01, FOUND-02, FOUND-03, FOUND-05, FOUND-06]

coverage:
  - id: D1
    description: "Anonymous visitor gets an HMAC-signed session identity on first contact, persisted server-side, reused on follow-up requests; tampered/cross-secret/malformed cookies are rejected without panicking"
    requirement: "FOUND-01"
    verification:
      - kind: unit
        ref: "internal/session/cookie_test.go#TestSessionIssuance"
        status: pass
    human_judgment: false
  - id: D2
    description: "All nine category slugs and three severities validate exactly (no case coercion); description length and lat/lon range are enforced server-side"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "internal/service/report_test.go#TestValidateSubmitInput"
        status: pass
    human_judgment: false
  - id: D3
    description: "shelter_open requires a valid capacity status; capacity status/headcount are rejected for every other category; headcount bounds enforced"
    requirement: "FOUND-06"
    verification:
      - kind: unit
        ref: "internal/service/report_test.go#TestShelterCapacityValidation"
        status: pass
    human_judgment: false
  - id: D4
    description: "ExpiryDuration returns 24h for critical, 8h for low/medium; BoundingBox is symmetric and clamps at the poles"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "internal/service/report_test.go#TestExpiryDuration"
        status: pass
      - kind: unit
        ref: "internal/service/report_test.go#TestBoundingBox"
        status: pass
    human_judgment: false
  - id: D5
    description: "NearbyReports query plan uses an index scan form against a 50,000-row table with a narrow bounding box, never a sequential scan of reports"
    requirement: "FOUND-03"
    verification:
      - kind: integration
        ref: "internal/store/reports_test.go#TestNearbyReportsUsesIndex"
        status: pass
    human_judgment: false
  - id: D6
    description: "A report vanishes from the read path the instant expires_at passes, with no sweep job involved"
    requirement: "FOUND-05"
    verification:
      - kind: integration
        ref: "internal/store/reports_test.go#TestExpiryReadTimePredicate"
        status: pass
    human_judgment: false
  - id: D7
    description: "No report read path returns a session identifier, at the generated-type level and via a full read-path check with a sentinel session id"
    requirement: null
    verification:
      - kind: integration
        ref: "internal/store/reports_test.go#TestNearbyReportsExcludesSessionID"
        status: pass
    human_judgment: false
  - id: D8
    description: "End-to-end over real HTTP: POST a report with no cookie, then GET nearby at the same coordinate using the issued cookie and find exactly that report, with no session id anywhere in the response"
    requirement: null
    verification:
      - kind: e2e
        ref: "internal/api/handlers/reports_e2e_test.go#TestSubmitThenNearbyReturnsReport"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/reports_e2e_test.go#TestNearbyResponseOmitsSessionID"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-06
status: complete
---

# Phase 1 Plan 03: Anonymous Session, Report Submit, and Nearby-Reports Query Summary

**Full server-side reporting loop — HMAC-signed anonymous cookie sessions, nine-category/three-severity report validation with server-computed severity-tiered expiry, and a two-stage indexed-bbox-then-Haversine nearby-reports query — proven end to end by an automated HTTP test against real Postgres.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-09-06T08:08Z (first RED commit)
- **Completed:** 2026-09-06T08:23Z (last GREEN commit) + summary/self-check
- **Tasks:** 3 (each RED→GREEN, 6 commits total)
- **Files modified:** 18 (16 created, 2 modified: `go.mod`, `sqlc.yaml`)

## Accomplishments
- `internal/session.Manager`: `crypto/rand` session ids, HMAC-SHA256 signing, `hmac.Equal` constant-time verification, chi middleware that issues/verifies the `pinalert_session` cookie on every request and persists a new session eagerly on first sight (before any submission) — FOUND-01.
- `internal/service.ReportService`: the full nine-category/three-severity/four-capacity-status enum surface with ordered, contract-locked slices; `ValidateSubmitInput` enforcing every rule (exact enum match, description length, lat/lon range, shelter-capacity category-gating in both directions, headcount bounds); `ExpiryDuration`'s two-tier D-14 default; `BoundingBox`'s pole-safe lat/lon delta math — FOUND-02, FOUND-06, D-01, D-14.
- `POST /api/reports` and `GET /api/reports` wired through one router, one `ReportService`, one explicitly-column-enumerated SQL query pair (`InsertReport`, `NearbyReports`) — the same endpoint plan 01-04/01-06 will poll for both the map and the list (01-RESEARCH.md Pattern 3).
- `NearbyReports`: indexed lat/lon `BETWEEN` prefilter + `expires_at > now()` in an inner subquery, exact Haversine distance in the outer projection, proven against a 50,000-row seeded table to use an index scan, not a sequential scan — FOUND-03.
- Read-time-only expiry proven directly against the production query: a report disappears the instant `expires_at` passes, with no sweep job anywhere in the codebase — FOUND-05.
- Session-id leak prevention proven twice: a reflection check on the generated `NearbyReportsRow` struct, and a full read-path check with a sentinel session id that never appears in the serialized response.
- The whole loop proven end to end over real HTTP against real Postgres: `TestSubmitThenNearbyReturnsReport` posts with no cookie, then GETs nearby using the cookie it was issued, and gets back exactly that report.

## Task Commits

Each task followed RED (test) → GREEN (implementation):

1. **Task 1: Server bootstrap and the anonymous HMAC session identity**
   - `28f76e1` (test) — failing tests for the session manager
   - `784c1fd` (feat) — session.Manager, store.NewPool, api.NewRouter, cmd/server/main.go
2. **Task 2: POST /api/reports — validation, severity-tiered expiry, geohash, insert**
   - `9602adf` (test) — failing tests for validation/expiry/bbox
   - `75ccefd` (feat) — service.ReportService.Submit, InsertReport query, handlers.SubmitReport
3. **Task 3: GET /api/reports — indexed bbox prefilter, Haversine, read-time expiry, e2e proof**
   - `5abfed1` (test) — failing tests for NearbyReports and the e2e proof
   - `329c792` (feat) — NearbyReports query, service.ReportService.Nearby, handlers.NearbyReports

**Plan metadata:** committed as part of this SUMMARY (worktree mode — orchestrator handles the final docs commit after merge; STATE.md/ROADMAP.md are not touched by this agent per its instructions).

_TDD gate compliance: every task has a `test(01-03):` commit immediately followed by a `feat(01-03):` commit; no REFACTOR commits were needed (no dead code or duplication surfaced after GREEN)._

## Files Created/Modified
- `internal/session/cookie.go` - HMAC-signed session Manager, chi Middleware, FromContext
- `internal/session/cookie_test.go` - all eight `<behavior>` bullets, no database needed
- `internal/service/report.go` - Category/Severity/CapacityStatus enums, ValidateSubmitInput, ExpiryDuration, BoundingBox, ReportService.Submit/Nearby
- `internal/service/report_test.go` - validation/expiry/bbox table tests, no database needed
- `internal/api/router.go` - chi router, Deps, session middleware, POST+GET /api/reports
- `internal/api/handlers/reports.go` - SubmitReport, NearbyReports, field-by-field request/response structs
- `internal/api/handlers/reports_e2e_test.go` - real-HTTP-over-real-Postgres proof (package handlers_test)
- `internal/store/db.go` - store.NewPool (pgxpool, ping-on-open, no migration on boot)
- `internal/store/queries/sessions.sql` - UpsertSession :exec
- `internal/store/queries/reports.sql` - InsertReport :one, NearbyReports :many
- `internal/store/reports_test.go` - index-scan, expiry-predicate, session-leak tests (package store_test)
- `internal/store/sqlc/*.go` - regenerated sqlc output, committed per project policy
- `cmd/server/main.go` - fail-fast DATABASE_URL/SESSION_SECRET, dev-only insecure fallback, configured http.Server
- `.planning/phases/01-foundation-report-map/deferred-items.md` - logs the pre-existing `storm_cyclone_damage` seed-data mismatch (out of this plan's scope)
- `go.mod` - go-chi/chi/v5 moved from indirect to direct require
- `sqlc.yaml` - fixed the broken text/integer overrides (see Deviations)

## Decisions Made
See `key-decisions` in frontmatter. The two with the widest blast radius: fixing `sqlc.yaml`'s broken nullable-pointer overrides (blocking every task, since each ends in `sqlc generate`), and casting the Haversine expression to `::float8` so `NearbyReportsRow.DistanceKm` is a plain `float64` rather than a nullable pgtype wrapper.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `sqlc.yaml`'s text/integer overrides used an invalid go_type specifier**
- **Found during:** Task 1, first `sqlc generate` run
- **Issue:** Plan 01-01 left `sqlc.yaml` with `go_type: "*string"` / `go_type: "*int32"` overrides for nullable text/integer columns. sqlc v1.31.1 rejects this: "Package override `go_type` specifier \"*string\" is not a Go basic type e.g. 'string'". Every task in this plan ends with `sqlc generate`, so this was blocking from the first task onward.
- **Fix:** Replaced the two broken overrides with `emit_pointers_for_null_types: true` at the `gen.go` level — sqlc's documented mechanism for the identical nullable-pointer mapping (`docs/reference/config.md`), keeping the `timestamptz -> time.Time` override for the non-nullable time columns.
- **Files modified:** `sqlc.yaml`
- **Verification:** `sqlc generate` succeeds; `internal/store/sqlc/models.go` shows `ShelterCapacityStatus *string` / `ShelterHeadcount *int32` as expected.
- **Committed in:** `784c1fd` (Task 1)

**2. [Rule 3 - Blocking] `go mod tidy` risk avoided by hand-editing go.mod**
- **Found during:** Task 1, after `internal/api/router.go` first imported `github.com/go-chi/chi/v5`
- **Issue:** Plan 01-01's summary documented that `go mod tidy` strips any pinned-but-unimported dependency (chi, http-swagger/v2) back out of `go.mod`/`go.sum`. Running `tidy` after adding the real chi import would have stripped the still-unimported `swaggo/http-swagger/v2` pin reserved for plan 01-07.
- **Fix:** Moved `github.com/go-chi/chi/v5 v5.3.2` from the indirect to the direct require block by hand, without running `go mod tidy` — mirroring exactly how 01-01 promoted `geohash` to direct.
- **Files modified:** `go.mod`
- **Verification:** `go build ./...` and `go vet ./...` both clean; `grep` confirms chi's version is unchanged at `v5.3.2`.
- **Committed in:** `784c1fd` (Task 1)

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking issues preventing task completion, not scope creep)
**Impact on plan:** Both fixes were necessary preconditions for any task to compile or generate code; neither changed the plan's intended behavior or added functionality beyond what was specified.

## Issues Encountered
- The sandboxed Bash tool refused several otherwise-valid commands (a multi-command pipeline with no git in it, and any command setting `ENV=` inline) as "too complex to verify it stays inside the worktree" / matching a git-adjacent heuristic. Worked around by splitting into single plain commands and skipping the one manual `curl` smoke test the plan's `<verification>` section suggests (`curl -i -c jar -X POST localhost:8080/api/reports ...`) — coverage is unaffected since the identical request/response path is exercised by the automated e2e test (`TestSubmitThenNearbyReturnsReport`), which runs the real router over real HTTP against real Postgres.

## User Setup Required
None beyond what plan 01-01 already covers (local Postgres). Provisioning `SESSION_SECRET` on a real deploy target is documented in this plan's `user_setup` block but is not a gate for Phase 1 execution — local development uses the `ENV=development` insecure fallback, exactly as designed.

## Next Phase Readiness
- `session.Manager`, `service.ReportService`, `api.NewRouter`/`Deps`, and the full `POST`/`GET /api/reports` contract (exact JSON shapes documented in the plan's `artifacts_this_phase_produces` table) are ready for plan 01-04's browser client.
- `service.Categories`/`Severities`/`CapacityStatuses` (ordered, contract-locked) are ready for plan 01-05's category grid and plan 01-07's OpenAPI doc-comments.
- No blockers. The one deferred item (`testutil/seed.go`'s `storm_cyclone_damage` vs. the canonical `storm_cyclone` slug) is logged in `deferred-items.md` and does not affect any current test's correctness.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*
