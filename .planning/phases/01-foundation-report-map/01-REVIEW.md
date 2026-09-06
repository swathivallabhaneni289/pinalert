---
phase: 01-foundation-report-map
reviewed: 2026-09-06T00:00:00Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - .github/workflows/ci.yml
  - cmd/migrate/main.go
  - cmd/server/main.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - internal/api/handlers/page.go
  - internal/api/handlers/page_test.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/reports_e2e_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/router.go
  - internal/service/report.go
  - internal/service/report_test.go
  - internal/session/cookie.go
  - internal/session/cookie_test.go
  - internal/store/db.go
  - internal/store/migrations.go
  - internal/store/migrations/00001_create_reports.sql
  - internal/store/migrations_test.go
  - internal/store/queries/reports.sql
  - internal/store/queries/sessions.sql
  - internal/store/reports_test.go
  - internal/store/sqlc/db.go
  - internal/store/sqlc/models.go
  - internal/store/sqlc/reports.sql.go
  - internal/store/sqlc/sessions.sql.go
  - internal/testutil/db.go
  - internal/testutil/db_test.go
  - internal/testutil/seed.go
  - web/embed.go
  - web/static/js/app.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/static/js/feed.js
  - web/templates/index.html.tmpl
  - web/static/css/main.css
  - web/static/css/modal.css
  - web/static/css/feed.css
findings:
  critical: 1
  warning: 7
  info: 2
  total: 10
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-09-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 33 (plus icon assets inspected in support of CR-01)
**Status:** issues_found

## Summary

This phase is well-built: the session HMAC/timing-safe verification, the never-SELECT-*-on-report-reads discipline, the server-side-only validation, and the client-side textContent-only DOM discipline (all called out as mandatory in CLAUDE.md) are implemented correctly and are covered by targeted tests (`TestNearbyReportsExcludesSessionID`, `TestNearbyResponseOmitsSessionID`, the session tamper/cross-secret tests). The previously-fixed shared-test-DB race (`-p 1` in `ci.yml`) is present and correct.

The defects found are one confirmed visual-rendering bug in the map view (the app's primary feature) and a cluster of edge-case/test-fidelity gaps — no data-loss, injection, or session-integrity issues were found.

## Critical Issues

### CR-01: Map pin icon has no CSS sizing rule — the two sibling call sites both needed one and got it, this one didn't

**File:** `web/static/js/map.js:73-86` (badge construction), missing counterpart in `web/static/css/main.css`

**Issue:** `map.js`'s `buildBadgeElement` builds `<span class="icon-badge icon-badge--pin ...">` containing a plain `<img src="/static/icons/{category}.svg">`, with no width/height set on the `<img>` via attribute or class. The source SVGs (e.g. `web/static/icons/flood.svg`) declare `width="100%" height="100%"` — a percentage with no concrete size to resolve against — so the `<img>` has no usable intrinsic size.

This is not a theoretical concern: the same icon-via-`<img>` pattern is used in two other places, and both of those needed an explicit fix:
- `web/static/css/feed.css:58-62` — `.report-row .icon-badge img { width: 55%; height: 55%; display: block; }`, with a comment (`feed.css:51-57`) that spells out exactly why: *"main.css's `.icon-badge svg { width: 55%; height: 55%; }` only targets an inline `<svg>`, not an `<img>` referencing one — without an equivalent rule here the icon has no usable intrinsic size … and falls back to the browser's default replaced-element box."*
- `web/static/css/modal.css:57-60` — `#category-grid .category-tile img { width: 24px; height: 24px; }`

`main.css:235-238`'s `.icon-badge svg { width: 55%; height: 55%; }` rule is itself dead code — nothing in the codebase creates an inline `<svg>` inside `.icon-badge`; every call site (map.js, modal.js, feed.js) uses `<img>`. That rule was evidently the intended fix, but only feed.css and modal.css added the `<img>`-specific equivalent. Map markers — rendered via Leaflet's `L.divIcon` for every report pin on the primary map view — have no equivalent rule anywhere in `main.css`, `modal.css`, or `feed.css`.

**Fix:** Add the missing rule next to the other two (or generalize it in `main.css` so all three call sites share it):
```css
/* main.css, near .icon-badge--pin */
.icon-badge--pin img {
  width: 55%;
  height: 55%;
  display: block;
}
```

## Warnings

### WR-01: `radius_km`/`lat`/`lon` range validation is bypassed by the literal string `"NaN"` (GET /api/reports only)

**File:** `internal/api/handlers/reports.go:235`, `:277`; `internal/service/report.go:260-268` (`clamp`)

**Issue:** `strconv.ParseFloat` accepts the literal `"NaN"` and returns a valid (non-error) `math.NaN()`. Both range checks compare with `<`/`>`:
```go
// reports.go:235
if radiusKm < minRadiusKm || radiusKm > maxRadiusKm {
// reports.go:277 (parseCoordinate, used for both lat and lon)
if parsed < min || parsed > max {
```
Every comparison against NaN is `false`, so `?lat=NaN&lon=NaN&radius_km=NaN` passes all three checks and reaches `service.Nearby`. There, `clamp` (`report.go:260-268`) has the identical pattern (`v < min` / `v > max`, both false for NaN) and passes NaN straight through into the SQL bounding-box parameters. The query does not crash or leak data — Postgres float8 comparisons against NaN are false, so the query silently returns zero rows instead of the 400 the input actually deserves — but it is a real gap in the "nothing here trusts client input" server-side validation contract this codebase otherwise takes seriously.

Note this is **GET-only**: the identical bound-check pattern in `service.ValidateSubmitInput` (`report.go:199-204`) is not reachable via POST /api/reports, because `encoding/json` rejects a bare `NaN` token as invalid JSON syntax before validation ever runs.

**Fix:** Reject non-finite values explicitly, e.g. in `parseCoordinate` and the radius check:
```go
if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
    writeFieldError(w, http.StatusBadRequest, name, name+" must be a finite number.")
    return 0, false
}
```

### WR-02: Description length limit is enforced in bytes server-side but UTF-16 code units client-side — the two disagree for any non-ASCII script

**File:** `internal/service/report.go:191-197`; `web/static/js/modal.js:468-470`

**Issue:** The server checks `len(desc)` where `desc` is a Go `string` — `len()` on a string returns byte length, not character count. The client checks `description.length` (JS string length = UTF-16 code units, effectively character count for the BMP). For any multi-byte UTF-8 script — including Hindi/Devanagari or other Indic scripts, directly relevant given this project's stated India-first audience — the client will accept up to 1000 *characters* while the server (encoding each character as 2-3 UTF-8 bytes) rejects well before that, at roughly 330-500 characters. A visitor who writes a report near the client's own limit gets a confusing late server 400 the client-side check never warned about, contradicting `service.ValidateSubmitInput`'s doc comment that client messages should never be contradicted by a server rejection.

**Fix:** Count runes server-side (`len([]rune(desc))` or `utf8.RuneCountInString(desc)`) so the limit means the same "1000 characters" the client and the copy both promise.

### WR-03: The FOUND-03 "index is used" drift guard doesn't actually compare against the shipped SQL

**File:** `internal/store/reports_test.go:23-63`

**Issue:** `nearbyReportsSQLForExplain` is a hand-maintained copy of the `NearbyReports` query body, with a comment claiming it "must stay byte-for-byte in sync" with `queries/reports.sql` and that `TestNearbyReportsQuerySourceHasExpectedShape` is "a drift guard against the two diverging silently." That test only greps `queries/reports.sql` for the substrings `"expires_at > now()"` and `"BETWEEN"` — it never compares the constant to the file contents, so the constant can drift arbitrarily (extra join, changed predicate, different column order) and this test keeps passing. `TestNearbyReportsUsesIndex` then runs `EXPLAIN ANALYZE` against the *constant*, not the query the application actually executes (`sqlcgen.NearbyReports`, generated from the real file) — so the "no Seq Scan" guarantee this test exists to provide is validated against a copy, not the shipped code path.

**Fix:** Either (a) read the `NearbyReports` query body out of `queries/reports.sql` at test time (e.g., extract the `-- name: NearbyReports` block) and assert it equals `nearbyReportsSQLForExplain`, or (b) drop the copy and `EXPLAIN` by calling `sqlcgen.New(pool).NearbyReports` directly (via `pgx`'s query logging or a `EXPLAIN`-wrapped raw call using the same const sqlc emits, which is accessible from an external test only via this kind of duplication — option (a) is simpler).

### WR-04: Swagger spec doesn't match actual error response shapes

**File:** `docs/docs.go:59-73` (GET spec), `:110-115` (POST 500 spec); `internal/api/handlers/reports.go:187`, `:248`

**Issue:** Two mismatches between the documented contract and the real handlers:
- POST `/reports`'s 500 is documented (`docs.go:110-115`) as returning `handlers.ErrorResponse` (a JSON `{"error":{...}}` body), but `SubmitReport`'s actual 500 path (`reports.go:187`) calls `http.Error(w, "internal server error", http.StatusInternalServerError)`, which writes `text/plain` with a bare string body, not JSON, and not the `ErrorResponse` shape.
- GET `/reports` is documented (`docs.go:59-73`) with only 200 and 400 responses, but `NearbyReports` (`reports.go:248`) can also return the same undocumented, non-JSON 500.

`TestSwaggerSpecCoversRoutes` (`swagger_test.go:119-165`) only checks that the paths/methods/enum values exist in the committed spec — it never checks response shapes, so this drift is undetected.

**Fix:** Either make the handlers' 500 path actually emit `ErrorResponse` JSON (consistent with every other error path in this file, which already goes through `writeFieldError`/`writeJSON`), or correct the swag annotations to match what's really returned. The former is preferable — it also makes the 500 response consistent with the 400 shape for any client parsing errors generically.

### WR-05: No DB-level constraints on category/severity/capacity-status enums or the shelter-only-columns rule

**File:** `internal/store/migrations/00001_create_reports.sql:3-16`

**Issue:** `category`, `severity`, and `shelter_capacity_status` are bare `TEXT` columns and `shelter_headcount` is a bare `INTEGER`, with no `CHECK` constraints enforcing the enum values `service.Categories`/`Severities`/`CapacityStatuses` declare, and nothing tying `shelter_capacity_status`/`shelter_headcount` to `category = 'shelter_open'`. All of that enforcement lives only in `service.ValidateSubmitInput`, which is bypassed by anything that writes to the table directly (a future migration/backfill script, an admin tool, or — concretely, in this codebase today — `internal/testutil/seed.go:25-35`, whose `seedCategories` list contains `"storm_cyclone_damage"`, a value that does not match `service.CategoryStormCyclone` ("storm_cyclone") or any other canonical category, and `internal/store/reports_test.go:171-176`, which inserts a raw row bypassing the service entirely). Neither of those two locations is caught by any test, because nothing checks seeded/inserted rows against the canonical enum. This is low-risk today (seed data used only for volume/index tests) but becomes a real correctness risk once Phase 2/3 aggregate confirm/dispute counts per category over this table.

**Fix:** Add `CHECK (category IN (...))`, `CHECK (severity IN (...))`, `CHECK (shelter_capacity_status IS NULL OR shelter_capacity_status IN (...))`, and `CHECK ((category = 'shelter_open') OR (shelter_capacity_status IS NULL AND shelter_headcount IS NULL))` in a follow-up migration; fix the `"storm_cyclone_damage"` typo in `testutil/seed.go` to `"storm_cyclone"`.

### WR-06: Bounding box does not wrap across the antimeridian

**File:** `internal/service/report.go:242-258` (`BoundingBox`)

**Issue:** `BoundingBox` clamps `lonMin`/`lonMax` to `[-180, 180]` rather than wrapping. A query centered near longitude ±180 (e.g. Fiji, the Chukotka/Alaska border region) with a radius that would normally cross the dateline instead gets a bounding box truncated at the boundary, silently excluding reports that are geographically within range but numerically on the other side of the ±180 seam. Low real-world impact for an India-focused deployment (India's longitude range, ~68–97°E, is nowhere near the antimeridian), but it's a genuine correctness gap in the one query that both the map and the list depend on, and would surface immediately if this app were ever deployed in/near the Pacific.

**Fix:** Either document this as an accepted Phase 1 limitation (India-only launch geography), or handle the wraparound case by issuing two bounding boxes (`[lonMin, 180]` and `[-180, lonMax]`) when the computed range would cross ±180.

### WR-07: `TestExpiryReadTimePredicate` has a thin, clock-dependent margin

**File:** `internal/store/reports_test.go:113-145`

**Issue:** The test seeds a report with a 2-second TTL (`testutil.SeedExpiringReport(t, pool, 2)`) and then `time.Sleep(2500 * time.Millisecond)` before asserting expiry. The margin between "TTL elapsed" and "test wakes up and queries" is 500ms, shared against `now()` on a real (not mocked) Postgres clock and a potentially loaded CI runner. A slow CI host or GC pause could push the actual elapsed wall-clock time on either side close enough to make this test flake in either direction (querying before Postgres's `now()` has passed `expires_at`, or occasionally the reverse for the "before expiry" assertion if scheduling is delayed).

**Fix:** Widen the margin (e.g. 3s TTL / 4s sleep) or, more robustly, seed the row with `expires_at` computed from a captured `now()` and assert against `NOW() - INTERVAL` server-side rather than relying on wall-clock sleep timing in the test process.

## Info

### IN-01: The `-p 1` test-isolation fix is correct today but depends entirely on an unenforced convention

**File:** `.github/workflows/ci.yml:39-44`

**Issue:** The comment correctly explains that `-p 1` is required because every package's `testutil.NewTestDB` truncates a table shared across the whole `postgres:16` service container. This works only because no test in the current suite calls `t.Parallel()` within a package — `-p 1` serializes packages but does nothing to prevent within-package parallelism. Nothing enforces or lints against a future test adding `t.Parallel()`, which would silently reintroduce the exact race this fix closed. (Note: the corresponding `Makefile` target was not in this review's file list and could not be checked for the same flag.)

**Fix:** A one-line comment on `testutil.NewTestDB` warning against `t.Parallel()` in any test that calls it would be cheap insurance; a `grep -r 't.Parallel()' --include=*_test.go` check in CI would make the invariant enforced rather than just documented.

### IN-02: `defaultRadiusKm = 10.0` is declared twice, independently

**File:** `cmd/server/main.go:32`; `internal/api/handlers/reports.go:29`

**Issue:** The same 10km default appears as `defaultRadiusKm` in `cmd/server/main.go` (used only to populate the client's `data-default-radius-km` attribute, which the client always echoes back on every request) and separately as `defaultRadiusKm` in `internal/api/handlers/reports.go` (used when a caller of `GET /api/reports` omits `radius_km` entirely — reachable today only from a direct API client, not the shipped JS, which always sends the value explicitly). The two are not wired to the same source of truth; changing one without the other would make the displayed default and the server's actual fallback default silently diverge for direct API consumers.

**Fix:** Thread `PageConfig.DefaultRadiusKm` through to the handler (e.g. via `Deps`) so there is exactly one 10.0 in the codebase.

---

_Reviewed: 2026-09-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
