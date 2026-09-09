---
phase: 01
slug: foundation-report-map
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-09
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

**Gate configuration:** `asvs_level: 1` · `block_on: high` (from `.planning/config.json` →
`workflow.security_asvs_level` / `workflow.security_block_on`). Severity order:
critical > high > medium > low. Only OPEN threats at `high` or above count toward `threats_open`.

**Scope:** 15 plans (`01-01` … `01-15`). Plans 01-01 through 01-14 carry an authored
`<threat_model>` block (`register_authored_at_plan_time: true`). Plan 01-15 was a live
UAT-correction round with no authored threat model; its register (`T-01-15-01` … `T-01-15-05`)
was built retroactively from the shipped diff and is marked accordingly.

---

## Register Construction Rules

The 14 authored registers reuse threat IDs in two different ways. Both are resolved explicitly
here so the totals below are reproducible.

**Rule 1 — genuine recurrence (one row, MAX severity, every cited sink verified).** The same
threat re-declared by a later plan that reuses the same control:

| ID | Declared in | Severity resolved to |
|----|-------------|----------------------|
| T-01-02 | 01-03 (high), 01-07 (high) | high |
| T-01-03 | 01-04 (high), 01-05 (high), 01-06 (high) | high |
| T-01-07 | 01-03 (high), 01-05 (medium) | **high** |
| T-01-17 | 01-04 (medium), 01-06 (medium) | medium |
| T-01-18 | 01-04 (medium), 01-05 (medium) | medium |
| T-01-20 | 01-04 (low), 01-06 (low) | low |
| T-01-SC | 01-01 (medium), 01-02 (medium) | medium |

**Rule 2 — ID collision (two distinct threats, separate rows, disambiguated by originating
plan).** Six IDs were re-issued by a later plan against a completely different component. Emitting
one row per bare ID would silently drop three `high` threats (`T-01-23` from 01-06, `T-01-27` from
01-09, `T-01-16` from 01-04), so each collision is carried as two rows:

| ID | First use | Second use |
|----|-----------|------------|
| T-01-16 | 01-04 · Leaflet CDN SRI · **high** | 01-07 · Swagger UI served from binary · medium |
| T-01-23 | 01-06 · Repudiation, no unbacked trust claim · **high** | 01-08 · DoS, modal backdrop guard · **high** |
| T-01-24 | 01-06 · Repudiation, no client expiry filter · medium | 01-08 · Tampering, severity restyle scope · medium |
| T-01-25 | 01-07 · InfoDisc, spec examples · medium | 01-08 · DoS, reduced-motion override · low |
| T-01-26 | 01-07 · Repudiation, spec drift · medium | 01-08 · InfoDisc, none introduced · low |
| T-01-27 | 01-07 · DoS, "Try it out" · low | 01-09 · DoS, map resolved height · **high** |

This collision is an artifact of plans 01-08 through 01-10 restarting numbering rather than
continuing the phase-wide sequence. It is a register-hygiene defect, recorded here rather than
resolved silently. **Future plans in this project must continue from `T-01-50` / the `T-01-NN-MM`
sub-numbering, never restart.**

**Total after both rules: 71 threats.**

---

## Trust Boundaries

Consolidated from all 15 plans.

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| browser → `POST /api/reports` | Fully untrusted, unauthenticated input | JSON body, headers, session cookie |
| browser → `GET /api/reports` | Untrusted query parameters control the scan window | `lat`, `lon`, `radius_km` |
| cookie jar → session middleware | Client holds and may attempt to modify its own identity token | HMAC-signed opaque session id |
| service layer → Postgres | All SQL is sqlc-generated and parameterised | Report rows, session rows |
| store row → JSON response | The point at which an internal column could leak publicly | `session_id` must never cross |
| API response → client DOM | Report descriptions authored by anonymous strangers, rendered in other visitors' browsers | Free-text description, category, severity |
| template data → rendered HTML | Server values interpolated by `html/template` | Fallback coords, radius, `AssetVersion` |
| third-party CDN → visitor browser | Three scripts + two stylesheets execute with full page privileges | unpkg.com vendor bundles |
| client browser → tile provider | Every basemap request reveals approximate viewer location | Viewport coordinates |
| browser geolocation → report coordinate | Visitor's own device supplies the location | Precise lat/lon |
| repository → public GitHub | Repo is public; anything committed is world-readable | Source, CI config, `go.sum` |
| developer machine → managed Postgres | `DATABASE_URL` carries a real credential to a third-party host | DSN |
| author stylesheet → user-agent default behaviour | An author-origin `display` can override native `[hidden]` semantics | CSS declarations |
| static asset tree → page CSS | Nine icon files referenced as `mask-image` sources | Same-origin SVG alpha channels |
| published spec → the internet | The document states exactly what the API accepts and returns | Swagger 2.0 schema |

---

## Threat Register

Evidence paths are relative to `/Users/swathivallabhaneni/code/pinalert/`.

### From 01-01 — repo, schema, CI

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-09 | Information Disclosure | `.github/workflows/ci.yml`, committed env files | high | mitigate | Ephemeral `postgres:16` DSN only; real creds in secrets; `.env` gitignored | closed | `.github/workflows/ci.yml:44` DSN is `postgres://postgres:postgres@localhost:5432/…`; `:8-9` warning comment; `.gitignore:8-9` (`.env`, `.env.local`); `git ls-files \| grep -i env` → zero committed env files |
| T-01-10 | Tampering | Schema migration application | medium | mitigate | Migrations run only from `cmd/migrate` / `internal/testutil`, never on app boot | closed | `goose.Up` appears at exactly two sites: `cmd/migrate/main.go:50` and `internal/testutil/db.go:48`; zero `goose`/`migrate` references in `cmd/server/main.go` (only the doc comment at `:2`) |
| T-01-11 | Repudiation | Time-sensitive `expires_at` semantics | high | mitigate | All time columns `TIMESTAMPTZ`, server-side `now()` defaults | closed | `internal/store/migrations/00001_create_reports.sql:14` (`created_at TIMESTAMPTZ NOT NULL DEFAULT now()`), `:15` (`expires_at TIMESTAMPTZ NOT NULL`), `:24` (`sessions.created_at TIMESTAMPTZ … DEFAULT now()`); zero bare `TIMESTAMP` columns. *Discrepancy: the plan says "all four time columns"; the shipped schema has three. The security property (no bare `TIMESTAMP`) holds for every column present. `expires_at` carries no SQL default by design — it is computed in Go from `time.Now().UTC()` at `internal/service/report.go:296,323`, which is T-01-07's control.* |
| T-01-12 | Denial of Service | Unbounded proximity scan | medium | mitigate | `idx_reports_lat_lon` and `idx_reports_expires_at` in first migration | closed | `internal/store/migrations/00001_create_reports.sql:18-19` (plus `idx_reports_geohash:20`); query uses the indexed bbox prefilter at `internal/store/queries/reports.sql:29-31`; `TestNearbyReportsUsesIndex` committed |
| T-01-SC | Tampering | Go module + `lucide-static` supply chain | medium | mitigate | Exact pinned versions, `go.sum` committed, no `@latest`, no `package.json`/`node_modules` | closed | `go.mod:6-11` all six direct deps pinned to exact versions; `go.sum` tracked in git; zero `@latest` in `*.mod`/`*.yml`; `find` for `package.json` and `node_modules` → zero results |

### From 01-02 — design system and icons

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-13 | Tampering | Inline SVG rendered into the DOM | low | accept | Glyphs are repo-committed first-party assets, never user-supplied | closed | Accepted risk **AR-01**. Verified: 9 SVGs under `web/static/icons/`, all referenced as static paths; zero `<script>`/`onload`/`href` in any of the nine |
| T-01-14 | Information Disclosure | Third-party runtime asset fetch for icons/fonts | low | mitigate | Icons self-hosted, font stack system-only | closed | Nine icons served from `/static/icons/*.svg` (same-origin); `web/static/css/main.css:31` font stack is `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, …` (no webfont); zero `@import`, `fonts.googleapis`, `fonts.gstatic`, or `url(http…)` in any stylesheet. *Scope note: this row's original blanket phrasing ("page rendering makes no runtime request to a third-party host") was superseded by 01-11/01-12, which added five unpkg.com vendor assets and an OpenFreeMap preconnect. Those exposures are separately registered as T-01-19, T-01-40 and T-01-48 — none is unregistered.* |

### From 01-03 — server slice

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-01 | Spoofing | `internal/session/cookie.go` — session forgery/replay | high | mitigate | `crypto/rand` id + HMAC-SHA256 verified with `hmac.Equal`; tampered cookie discarded and replaced | closed | `internal/session/cookie.go:58` (`crypto/rand.Read`), `:90` (`hmac.New(sha256.New, m.secret)`), `:83` (`hmac.Equal`), `:138-141` (invalid signature → fresh id, never trusted), `:46-48` (empty secret rejected at construction), `:117-119` (`HttpOnly`, `Secure`, `SameSite=Lax`). `TestSessionIssuance` committed |
| T-01-02 | Information Disclosure | `NearbyReports` query and the report JSON response | high | mitigate | Both projections enumerate columns; handler declares its own response struct field by field | closed | `internal/store/queries/reports.sql:15-16` and `:18-19` enumerate columns, no `session_id`; `:11-12` `InsertReport … RETURNING` likewise excludes it; `internal/api/handlers/reports.go:61-76` `ReportResponse` hand-declared, no session field; `internal/store/sqlc/reports.sql.go` carries `SessionID` only as an *insert parameter* (`:28,:57`). Also verified in the published spec: zero `session_id`/`sessionId` in `docs/docs.go` and `docs/swagger.json`. **Caveat:** `TestNearbyReportsExcludesSessionID` **SKIPPED** locally (no `DATABASE_URL`) — it is not passing evidence here; the static evidence above is what closes this row. `TestNearbyResponseOmitsSessionID` and `TestNearbyReportsQuerySourceHasExpectedShape` are also committed and run in CI against a live Postgres |
| T-01-04 | Tampering | SQL injection via report fields or query params | high | mitigate | Every query sqlc-generated and parameterised; no string concatenation in the store layer | closed | Zero `fmt.Sprintf`/concatenation in any SQL path across `internal/store/`, `internal/service/`, `internal/api/` (the single `fmt.Sprintf` at `internal/service/report.go:134` formats a `ValidationError` message, not SQL); zero raw `.Query(`/`.Exec(`/`.QueryRow(` outside `internal/store/sqlc/` and `internal/testutil/` (test-only); queries use `sqlc.arg()` placeholders throughout `internal/store/queries/reports.sql` |
| T-01-06 | Tampering / Availability | `SESSION_SECRET` handling | high | mitigate | Fatal exit when unset outside `ENV=development`; never auto-generates a secret | closed | `cmd/server/main.go:100-110` — `loadSessionSecret` returns the env value, else `log.Fatal("SESSION_SECRET must be set outside ENV=development")` at `:106`; the development fallback at `:109` is a fixed literal (`:25`), never randomly generated |
| T-01-07 | Tampering | Client-supplied `expires_at` / `created_at` / `geohash` / `id` | **high** | mitigate | Submit input carries only 7 client-settable fields; server computes the rest; unknown JSON fields rejected | closed | `internal/api/handlers/reports.go:40-55` `SubmitReportRequest` has exactly 7 fields (no id/geohash/created_at/expires_at/session_id); `:155` `dec.DisallowUnknownFields()`; `internal/service/report.go` `SubmitInput` mirrors the same 7; server computes `now` at `:296`, `geohash.EncodeWithPrecision` at `:298`, `expiresAt` at `:323` |
| T-01-08 | Denial of Service | Unbounded request body or scan window | medium | mitigate | 64 KB `MaxBytesReader`, description ≤ 1000 chars, `radius_km` ≤ 50 | closed | `internal/api/handlers/reports.go:23` (`maxSubmitBodyBytes = 64 * 1024`), `:153` (`http.MaxBytesReader`), `:30` (`maxRadiusKm = 50.0`), `:235-239` (range rejection); `internal/service/report.go:195-196` (1000-char cap) |
| T-01-05 | Tampering | CSRF on `POST /api/reports` | low | accept | `SameSite=Lax`; no token — anonymous posting has no account-takeover consequence | closed | Accepted risk **AR-02**. The stated control is verified present: `internal/session/cookie.go:119` `SameSite: http.SameSiteLaxMode` |
| T-01-15 | Information Disclosure | Error responses | medium | mitigate | Validation errors return field + safe message only; all else a generic 500 with detail logged server-side | closed | `internal/api/handlers/reports.go:284-286` (`writeFieldError` → `{field, message}` only); `:186-188` and `:247-249` (`log.Printf` server-side, `http.Error(w, "internal server error", 500)` to the client); `:116-119` `ErrorDetail` carries no internal detail |

### From 01-04 — browser slice and map

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-03 | Tampering | Report-authored strings rendered by `map.js`, `modal.js`, `feed.js` | high | mitigate | All report strings reach the DOM via `Pinalert.setText` (text node); popups built as DOM elements; grep gate on every JS file | **closed** (primary control verified; see **Shortfall SF-01** for the missing build gate) | `web/static/js/app.js:268-270` — `setText` assigns `el.textContent` only. **Zero** occurrences of `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, `eval(` or `new Function` across all four JS files (the only textual hit is a comment at `map.js:11`). All 25 report-string sinks go through `Pinalert.setText`: `map.js:165,169`; `feed.js:179,180,293,366`; `modal.js:87,102,214,286,294,445,450,515,558,588,596,597,601,606,609`. Popups: `map.js:192` `bindPopup(buildPopupContent(report))` where `buildPopupContent` (`:157-175`) uses `createElement` + `setText` and returns an Element. Every `setAttribute` call across the three renderers takes a fixed literal or an allowlisted value |
| T-01-16 (01-04) | Tampering | Leaflet loaded from a third-party CDN | high | mitigate | Version-pinned URL + SRI `integrity` + `crossorigin="anonymous"` | closed | `web/templates/index.html.tmpl:101-103` (`leaflet@1.9.4/dist/leaflet.js`, `integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="`, `crossorigin="anonymous"`) and `:10-12` for `leaflet.css`. `TestVendorMapScriptsLoadInDependencyOrder` **PASSES** and asserts `integrity="sha256-` + `crossorigin="anonymous"` on every vendor tag |
| T-01-17 | Tampering | Category value interpolated into a class name / icon path | medium | mitigate | Validated against the nine-slug allowlist, falls back to `other` | closed | `web/static/js/app.js:217-220` — `iconClass` does `CATEGORIES.indexOf(category) === -1 ? 'other' : category`; `CATEGORIES` defined at `:20`. Severity path: `:203-209` `severityClass` falls back to `low`; `:246-262` `ageStage` returns one of three fixed strings. `TestIconGlyphsAreClassDriven` **PASSES**. *Evolution note: 01-13 replaced `Pinalert.iconPath` (file path sink) with `Pinalert.iconClass` (CSS class sink); the allowlist check carried across unchanged — that transition is T-01-13-01* |
| T-01-18 | Information Disclosure | Precise geolocation | medium | accept | Core product function; visitor sees and confirms the coordinate before submit | closed | Accepted risk **AR-03**. Control verified: `web/static/js/modal.js:294` renders the coordinate to `#coord-readout` before submission; `web/templates/index.html.tmpl:57-60` provides the readout and the denial fallback notice |
| T-01-19 | Information Disclosure | OSM tile requests reveal viewport to a third party | low | accept | Unavoidable for a free, no-API-key map; per PROJECT.md budget constraint | closed | Accepted risk **AR-04** |
| T-01-20 | Denial of Service | Polling load / duplicate polling from a second panel | low | mitigate | One 30 s interval on one endpoint, shared store, no per-panel duplicate polling | closed | `web/static/js/app.js:142-151` — `startPolling` guards on `pollTimer !== null` (`:145`) so repeat calls are no-ops; single `setInterval` at `:148` on `config.pollIntervalMs` (30000, `:70`); `fetchReports` is "the one and only GET /api/reports call" (`:107-110`). `map.js:114` calls `startPolling()`; `feed.js` issues **no** fetch of its own except the explicit retry button (`:436`). `feed.js:452` `setInterval(refreshAgeAndTime, 60000)` is a local re-render timer with no network call |

### From 01-05 — submission modal

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-21 | Tampering | Client-side validation treated as the control | high | mitigate | Client mirrors rules for UX only; the server revalidates everything and is sole authority | closed | `internal/service/report.go:185-221` — server-side `ValidationError` returns for category (`:185`), severity (`:188`), description min 10 (`:193`) and max 1000 (`:196`), latitude (`:200`), longitude (`:203`), shelter capacity required/forbidden (`:208`,`:212`), headcount forbidden/unrealistic (`:215`,`:221`). Invoked from `Submit` before persistence; handler maps `ValidationError` → 400 at `internal/api/handlers/reports.go:181-185`. `TestValidateSubmitInput` and `TestShelterCapacityValidation` committed |
| T-01-22 | Information Disclosure | Shelter headcount on a public feed | low | accept | Optional, coarse, self-reported operational data about a shelter, not personal data | closed | Accepted risk **AR-05** |

### From 01-06 — feed list

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-23 (01-06) | Repudiation | Showing a trust claim the data does not support | high | mitigate | List renders no confirmation count, "Unconfirmed" badge or "verified" wording | **closed** (property verified; see **Shortfall SF-02**) | Grep for `confirm\|verified\|unconfirmed\|dispute\|trust` across `feed.js` and `map.js` returns **only** the scope comment at `feed.js:16-17` — zero rendered occurrences. No badge element, count, or trust wording is constructed anywhere in the row builder (`feed.js:120-185`) |
| T-01-24 (01-06) | Repudiation | Client-side expiry drifting from server expiry | medium | mitigate | List applies no expiry filter; server read-time predicate is the sole definition of "expired" | **closed** (property verified; see **Shortfall SF-02**) | Grep for `expires_at\|expired\|filter(` in `web/static/js/feed.js` returns **zero** matches — the list renders whatever the store holds. Sole expiry predicate is `internal/store/queries/reports.sql:29` (`WHERE expires_at > now()`). `TestExpiryReadTimePredicate` committed |

### From 01-07 — OpenAPI / Swagger docs

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-25 (01-07) | Information Disclosure | Spec examples and descriptions | medium | mitigate | Synthetic examples only; no real session id, DSN, secret or internal hostname; only the two public operations documented | closed | Zero matches for `neon.tech\|supabase\|localhost:5432\|postgres://\|SESSION_SECRET` in `docs/docs.go`; `paths` contains only `"/reports"` (`docs/docs.go:24`); examples are synthetic (`internal/api/handlers/reports.go:42-54`, e.g. `13.0827`/`80.2707`) |
| T-01-16 (01-07) | Tampering | Swagger UI asset delivery | medium | mitigate | `http-swagger/v2` serves UI assets from the compiled binary — no third-party CDN for the doc page | closed | `internal/api/router.go:81` `httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))`; `go.mod:10` `github.com/swaggo/http-swagger/v2 v2.0.2` with `github.com/swaggo/files/v2` as the embedded asset source; `TestSwaggerDocServed` committed |
| T-01-26 (01-07) | Repudiation | Spec drifting from actual handler behaviour | medium | mitigate | Spec generated from handler doc-comments; drift guard asserts route/enum coverage in CI | closed | `TestSwaggerSpecCoversRoutes` **PASSES** (`internal/api/handlers/swagger_test.go:119`); annotations live on the handlers at `internal/api/handlers/reports.go:132-144` and `:201-212`; `Makefile` `swag` target regenerates from `internal/api/router.go`. Verified without regenerating (read-only audit) |
| T-01-27 (01-07) | Denial of Service | "Try it out" issuing live writes | low | accept | API is anonymous and public by design; the doc page grants no capability a plain HTTP client lacks. Rate limiting is Phase 4 (ROBUST-04) | closed | Accepted risk **AR-06** |

### From 01-08 — modal backdrop and slider CSS

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-23 (01-08) | Denial of Service | `.modal-backdrop` full-viewport scrim | high | mitigate | `display` applied only through a `:not([hidden])`-guarded selector; contract test fails the build otherwise | closed | `web/static/css/main.css:781` — the **only** `.modal-backdrop` rule setting layout is `.modal-backdrop:not([hidden]) { … }`. `TestModalBackdropHiddenGuard` (`web/css_contract_test.go:46`) **PASSES** — a durable, committed, CI-run gate |
| T-01-24 (01-08) | Tampering | Severity control restyle | medium | mitigate | CSS-only; element stays a native `<input type="range">`; no ARIA slider role added | closed | `web/templates/index.html.tmpl:65-66` — `<input type="range" id="severity" min="1" max="3" step="1" … aria-label="Severity" aria-valuetext="1 · Low" class="severity-slider">`. Native element preserved; **no** `role="slider"` anywhere in the template or JS. *The `git diff --quiet` byte-unchanged gate was an execution-time check and is not retroactively re-runnable; the substantive property it protected is verified present above* |
| T-01-25 (01-08) | Denial of Service | `prefers-reduced-motion` override | low | mitigate | Per-selector rule blocks (not one grouped selector); count gate requires ≥ 4 surviving `transition: none` | closed | `web/static/css/modal.css` contains **5** `transition: none` declarations (gate requires ≥ 4), across two separate `@media (prefers-reduced-motion: reduce)` blocks at `:148` and `:270`; a third at `web/static/css/main.css:612` |
| T-01-26 (01-08) | Information Disclosure | None introduced (CSS-only plan) | low | accept | No data flow, endpoint, payload, dependency or storage change | closed | Accepted risk **AR-07** |

### From 01-09 — map viewport height

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-27 (01-09) | Denial of Service | Primary map container sizing | high | mitigate | Viewport-relative height that resolves without depending on any ancestor; contract test fails the build if removed or downgraded | closed | `web/static/css/main.css:529-531` — `#map { height: 100vh; height: 100dvh; }`, self-contained per the `:518-522` comment. `TestPrimaryMapHasResolvedHeight` (`web/css_contract_test.go:136`) **PASSES** — durable, committed, CI-run gate |
| T-01-28 (01-09) | Denial of Service | Mobile viewport sizing of `.app-shell` and the map | medium | mitigate | Dynamic viewport unit + paired legacy fallback on both boxes | closed | `web/static/css/main.css:499-500` (`.app-shell { min-height: 100vh; min-height: 100dvh; }`) and `:530-531` (`#map`) — both declare `vh` first as the legacy fallback and `dvh` second so it wins where supported |
| T-01-29 (01-09) | Tampering | Scope of the fix | low | mitigate | `git diff --quiet` gates assert other stylesheets, all JS and all templates byte-unchanged | closed | *Execution-time gate, not retroactively re-runnable.* The property it protected is independently verified: the XSS-safe DOM path (T-01-03), the session/submit flow (T-01-01/T-01-07) and the modal map (T-01-44) are all confirmed intact by this audit |
| T-01-30 (01-09) | Information Disclosure | None introduced (CSS-only plan) | low | accept | No data, endpoint, payload, dependency, credential or storage change | closed | Accepted risk **AR-08** |

### From 01-10 — retina tiles

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-31 | Information Disclosure | Tile URL template in `map.js` and `modal.js` | medium | mitigate | Gate asserts the exact OSM tile URL template byte-for-byte in both files | closed | `web/static/js/map.js:77` and `web/static/js/modal.js:381` both carry `'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'`. Durable gate: `web/js_contract_test.go:34` pins `rasterTileURLTemplate` to that exact literal; `TestBasemapBranchesPointAtCorrectHosts` **PASSES**, reading the value out of each construction call's own options window |
| T-01-32 | Denial of Service | 4× tile fetch volume on HiDPI against OSM's free community servers | low | accept | Inside OSM's usage policy at portfolio-demo scale; the alternative (commercial vendor) was explicitly declined | closed | Accepted risk **AR-09**. `detectRetina: true` confirmed at `map.js:79`, `modal.js:383` |
| T-01-33 | Tampering | The load-bearing max-zoom option both tile layers depend on | medium | mitigate | Gate asserts the option is present in both files (Leaflet's retina branch is conditional on it) | closed | `web/static/js/map.js:78` (`maxZoom: 19` on the raster layer) and `:63` (on the map); `modal.js:382` and `:367`. Durable gate: `TestMapsDeclareOwnZoomBounds` (`web/js_contract_test.go:224`) **PASSES**, asserting `maxZoom` is present and a positive integer in both modules |
| T-01-34 | Tampering | Scope of the change | low | mitigate | `git diff --quiet` gates plus a dirty-file count cap | closed | *Execution-time gate.* Protected properties independently verified by this audit (T-01-03, T-01-23/01-08, T-01-27/01-09 all confirmed intact) |
| T-01-35 | Denial of Service | Maximum reachable zoom on retina displays (19 → 18) | low | accept | Deliberate tradeoff; raising to 20 would request tiles OSM does not serve | closed | Accepted risk **AR-10** |
| T-01-36 | Elevation of Privilege / Spoofing / Repudiation | None introduced | low | accept | No endpoint, payload, dependency, credential, session, storage or schema change | closed | Accepted risk **AR-11** |

### From 01-11 — MapLibre vendor tags

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-37 | Tampering | The three vendor script tags | high | mitigate | Exact pinned versions + sha256 `integrity` + `crossorigin="anonymous"` on every tag; hashes independently recomputed, never copied from the CDN's own metadata | closed | **Hashes independently re-downloaded and recomputed during this audit — all five match the template byte-for-byte:** `leaflet@1.9.4/dist/leaflet.js` → `sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo=` (template `:102`) ✓; `maplibre-gl@5.24.0/dist/maplibre-gl.js` → `sha256-RamwepGJzlYFTGIKlHzPQeKR5YyV6bYVM7dAqqZe5cs=` (`:105`) ✓; `@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js` → `sha256-Hmz4yz61/ZCYeaob82o4P7UGyaWy27+rq85lopTdH8s=` (`:108`) ✓; `leaflet@1.9.4/dist/leaflet.css` → `sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY=` (`:11`) ✓; `maplibre-gl@5.24.0/dist/maplibre-gl.css` → `sha256-qx5w1Z7EBGW65+cDDaLzzPKBM/1QLmK9WY7vut/XpzI=` (`:14`) ✓. Method: `curl -sL <url> \| openssl dgst -sha256 -binary \| base64`. All five URLs pin an exact version — no range, no floating tag; all carry `crossorigin="anonymous"`. `TestVendorMapScriptsLoadInDependencyOrder` **PASSES** (see **Shortfall SF-04** — that gate checks the `sha256-` prefix, not the value) |
| T-01-38 | Tampering | Package provenance of the two new packages | medium | accept | Org-scoped packages, provenance attestation, CDN tags only — no package-manager install | closed | Accepted risk **AR-12**. Verified: zero `package.json` / `node_modules` anywhere in the repo |
| T-01-39 | Denial of Service | CDN and tile-host availability (SPOF) | medium | accept | Widens an existing dependency rather than creating a new class; markers, feed, submit and API unaffected | closed | Accepted risk **AR-13** |
| T-01-40 | Information Disclosure | Preconnect hint to the tile host | low | accept | Discloses nothing the tile fetches do not already disclose | closed | Accepted risk **AR-14**. `web/templates/index.html.tmpl:16` |
| T-01-41 | Tampering | Vendor script load order | medium | mitigate | Test fails the build on reordering, missing `integrity`/`crossorigin`, `defer`/`async` on a vendor tag, or an app module hoisted above the bridge | closed | `TestVendorMapScriptsLoadInDependencyOrder` (`web/template_contract_test.go:85`) **PASSES** — asserts exactly one occurrence per vendor src, strict positional ordering, no `defer`, no `async`, and every app module below the bridge. Template state matches: vendor tags at `:101`, `:104`, `:107` (undeferred, in dependency order); app modules at `:110-113` (all `defer`, all below the bridge) |
| T-01-42 | Repudiation / Spoofing / Elevation of Privilege | None introduced | low | accept | No endpoint, payload, credential, session, storage or schema change | closed | Accepted risk **AR-15** |

### From 01-12 — vector basemap swap

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-43 | Tampering / Information Disclosure | The vector style URL in `map.js` and `modal.js` | medium | mitigate | Shell gate + contract test assert the exact OpenFreeMap liberty endpoint in each file | closed | `web/static/js/map.js:68` and `web/static/js/modal.js:372` both carry `style: 'https://tiles.openfreemap.org/styles/liberty'`. Durable gate: `web/js_contract_test.go:33` pins `vectorStyleURL` to that exact literal; `TestBasemapBranchesPointAtCorrectHosts` **PASSES**, reading the value from the construction call's own bounded options window |
| T-01-44 | Denial of Service | Basemap availability on clients without WebGL2 | high | mitigate | Capability probe requiring both vendor globals **and** a WebGL2 context, falling back to 01-10's raster layer | closed | `web/static/js/map.js:33` (`typeof L.maplibreGL !== 'function'`), `:36` (`typeof maplibregl === 'undefined'`), `:40` (`!!document.createElement('canvas').getContext('webgl2')`) → raster fallback at `:77`. Mirrored in `modal.js:327`, `:330`, `:334` → `:381`. `TestCapabilityProbeGatesBasemapChoice` (`web/js_contract_test.go:297`) **PASSES** |
| T-01-45 | Denial of Service | Client passes the probe but the renderer still fails | low | accept | Broken/blocklisted GPU driver; documented in code rather than papered over with a try/catch that would not catch it | closed | Accepted risk **AR-16** |
| T-01-46 | Denial of Service | Maximum reachable zoom on the raster fallback path | medium | mitigate | Fallback branch re-derives the map's ceiling from the layer's own post-construction value | closed | `web/static/js/map.js:90` `map.setMaxZoom(rasterLayer.options.maxZoom);` and `web/static/js/modal.js:392` `modalMap.setMaxZoom(rasterLayer.options.maxZoom);` — present in both files, inside the fallback branch |
| T-01-47 | Tampering | Attribution string inserted as markup | low | accept | Fixed developer-authored literal containing zero user input; links carry `rel="noopener"` | closed | Accepted risk **AR-17**. Verified literal + `rel="noopener"` at `map.js:71`, `modal.js:375` |
| T-01-48 | Information Disclosure | Which third party receives viewport coordinates | low | accept | Swaps which third party sees it, not whether; provider needs no account, key or cookie | closed | Accepted risk **AR-18** |
| T-01-49 | Denial of Service | Payload weight on a mobile emergency user | medium | accept | Stated cost (~275 KB compressed); vector tiles are smaller per tile and cache across zoom levels | closed | Accepted risk **AR-19** |
| T-01-50 | Spoofing / Repudiation / Elevation of Privilege | None introduced | low | accept | No endpoint, payload, dependency, credential, session, storage or schema change | closed | Accepted risk **AR-20** |

### From 01-13 — glyph CSS mask recolor (round 2)

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-13-01 | Tampering | `Pinalert.iconClass` — category's new sink is a CSS class name | medium | mitigate | Existing allowlist check carried across unchanged; falls back to `other` | closed | `web/static/js/app.js:217-220` — `CATEGORIES.indexOf(category) === -1 ? 'other' : category`; `CATEGORIES` at `:20`. `TestIconGlyphsAreClassDriven` (`web/js_contract_test.go:387`) **PASSES**, bounding its allowlist check to the helper's own body |
| T-01-13-02 | Tampering | DOM construction in `map.js`, `modal.js`, `feed.js` | medium | mitigate | All three renderers keep `createElement` + fixed class strings; no markup-string assembly | closed | `createElement` used 6× in `map.js`, 8× in `modal.js`, 9× in `feed.js`; zero markup sinks anywhere. Class assignment is fixed literal + validated helper output: `map.js:144,147`; `modal.js:210`; `feed.js:130,132,176-177` (`replacePrefixedClass` with the allowlist-derived suffix). `TestIconGlyphsAreClassDriven` **PASSES** |
| T-01-13-03 | Information Disclosure | Mask source URLs in `main.css` | low | accept | All sources are same-origin paths into the already-public static tree | closed | Accepted risk **AR-21**. Verified: 18 `mask-image` declarations (9 categories × prefixed + standard) at `web/static/css/main.css:303-329+`, all `url("/static/icons/*.svg")`; zero `url(http…)` or `url(//…)` in any stylesheet |
| T-01-13-SC | Tampering | npm/pip/cargo installs | low | accept | No package installed from any package manager | closed | Accepted risk **AR-22**. Verified: zero `package.json`, zero `node_modules` |

### From 01-14 — glyph ink coverage and contrast (round 3)

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-14-01 | Tampering | Hand-edited SVG files under `web/static/icons/` | low | accept | One numeric attribute per file; consumed only as CSS `mask-image` sources, which are never scripted | closed | Accepted risk **AR-23**. Verified across all nine SVGs: single `stroke-width` attribute, `currentColor` present, **zero** `<script>`, `onload` or `href` in any file |
| T-01-14-02 | Tampering | DOM construction in `map.js`, `modal.js`, `feed.js` | medium | mitigate | No renderer reverted to a replaced-element icon; existing gate stays green | closed | `TestIconGlyphsAreClassDriven` **PASSES**. Renderers still use `createElement` + `<span class="icon-glyph …">` (`map.js:147`, `modal.js:210`, `feed.js:132,177`) — no `<img>`/`<object>` icon element anywhere |
| T-01-14-03 | Tampering | `Pinalert.iconClass`'s CATEGORIES allowlist | medium | mitigate | Untouched; still asserted by the same test | closed | `web/static/js/app.js:20` (`CATEGORIES`), `:218` (allowlist check). `TestIconGlyphsAreClassDriven` **PASSES** |
| T-01-14-04 | Denial of Service | The two new contract gates | low | accept | Pure static parses over an embedded `fs.FS`: no network, no subprocess, no filesystem write | closed | Accepted risk **AR-24**. Verified: zero `net/http`, `exec.Command`, `os.Create` or `WriteFile` in `web/css_contract_test.go`, `web/js_contract_test.go`, `web/template_contract_test.go` |
| T-01-14-SC | Tampering | npm/pip/cargo installs | low | accept | No package installed from any package manager | closed | Accepted risk **AR-25** |

### From 01-15 — live-debugging round 4 (retroactive STRIDE)

*No `<threat_model>` was authored for plan 01-15. This register was constructed from the shipped
diff: `main.css` / `feed.css` glyph colour + sizing rules, nine SVG `stroke-width` reverts,
`AssetVersion` cache-busting in `cmd/server/main.go`, `AssetVersion` threading in
`internal/api/handlers/page.go`, `?v=` query strings in `index.html.tmpl`, and the dev-mode
`Cache-Control` branch in `internal/api/router.go`. Severities assigned by impact × likelihood.*

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status | Evidence |
|-----------|----------|-----------|----------|-------------|------------|--------|----------|
| T-01-15-01 | Tampering | `AssetVersion` value provenance | medium | mitigate | Server-derived timestamp only; never reachable from request input | closed | `cmd/server/main.go:72` — `AssetVersion: strconv.FormatInt(time.Now().Unix(), 10)`, computed once inside `main()` before the router is built. `internal/api/handlers/page.go:56-63` — `Page` builds `pageViewModel` **only** from the closed-over `cfg`; the `*http.Request` parameter `r` is never read. Zero `r.URL` / `r.Header` / `r.FormValue` / `r.Cookie` anywhere in `page.go` |
| T-01-15-02 | Tampering / Elevation of Privilege | Dev-only `Cache-Control` relaxation | medium | mitigate | Gated on `ENV=="development"` read at process boot; not client-flippable | closed | `cmd/server/main.go:42` reads `env := os.Getenv("ENV")`; `:74` sets `Dev: env == "development"`. `internal/api/router.go:106-118` — `staticFileServer(dev bool)` resolves `cacheControl` **once at construction** (`:115-117`), captured in the returned closure; the per-request handler at `:119-122` only writes the already-fixed string. No request header, query param or cookie participates. The `Dev` zero value is `false` → production caching (`internal/api/router.go:33-42`) |
| T-01-15-03 | Tampering | `?v=` query string as a potential reflected-value sink | medium | mitigate | The only value templated into the `?v=` position is the server timestamp; `html/template` contextual auto-escaping applies | closed | `web/templates/index.html.tmpl` — `{{.AssetVersion}}` appears exactly 7× (`:7,8,9,110,111,112,113`), all in the query-string position of a same-origin `/static/…` path. `internal/api/handlers/page.go:36-41` — `pageViewModel` has exactly four fields (three server constants + `AssetVersion`); no user-controlled value can reach it. Template parsed via `template.ParseFS` (`html/template`, `:49-51`), so contextual escaping is active. `TestPageShellAppliesAssetVersionToLocalStaticAssets` committed |
| T-01-15-04 | Tampering | CDN vendor tags must stay unversioned so their SRI hashes still match | **high** | mitigate | `?v=` applied to local static assets only, never to a tag carrying `integrity` | closed | `web/templates/index.html.tmpl` — `?v=` appears on exactly the 7 local `/static/…` paths (`:7,8,9,110,111,112,113`) and on **zero** tags carrying `integrity=`. All five vendor URLs (`:10,13,101,104,107`) remain byte-unversioned with their `integrity` + `crossorigin` attributes intact. `TestVendorMapScriptsLoadInDependencyOrder` **PASSES** (asserts exactly one occurrence of each vendor src and re-checks `integrity`/`crossorigin`) — confirming 01-15 did not re-open T-01-37 or T-01-16 (01-04) |
| T-01-15-05 | Tampering | Nine SVG `stroke-width` reverts (3 → 2) | low | accept | Continuation of T-01-14-01: one numeric attribute per file, `mask-image` consumption only | closed | Accepted risk **AR-26**. Verified: all nine SVGs at `stroke-width="2"`, all `currentColor`, zero `<script>`/`onload`/`href` |

---

## Mitigation Shortfalls (WARNING — not blocking)

These are declared mitigations whose **substantive security property is verified present in the
shipped code**, but whose **declared build-gate half is absent from the repository**. They are
regression-durability gaps, not live vulnerabilities, so they do not count toward `threats_open`.
They are recorded here rather than glossed, because the plans state the gate as an enforced
property.

### SF-01 — the markup-sink grep gate does not exist as a committed gate (relates to T-01-03, high)

Plans 01-04, 01-05 and 01-06 each declare: *"a grep gate on every JS file fails the build on a
markup-parsing sink."*

- **What is verified present:** the security property itself. Zero `innerHTML`, `outerHTML`,
  `insertAdjacentHTML`, `document.write`, `eval(` or `new Function` across all four JS files; all
  25 report-string sinks route through `Pinalert.setText`'s `textContent` assignment; popups and
  badges are passed to Leaflet as DOM Elements.
- **What is absent:** any durable enforcement. The full committed test inventory (27 `func Test…`
  across the repo) contains no markup-sink assertion; `grep` for `innerHTML` in `web/*_test.go`,
  all `*_test.go`, `Makefile` and `.github/workflows/ci.yml` returns zero. The gate existed only as
  a `<verify><automated>` shell command inside the PLAN documents — run once at execution time,
  never in CI.
- **Consequence:** a future edit reintroducing `element.innerHTML = report.description` would ship
  a stored-XSS hole with a fully green build. This is the single highest-value regression gate in
  the phase and it is the one that is missing.
- **Suggested remediation (implementation change — NOT applied by this audit):** add a
  `TestNoMarkupParsingSinks` to `web/js_contract_test.go` walking `StaticFS` `static/js/*.js` and
  failing on any of the six sink tokens outside a comment. It would sit alongside the four contract
  tests that already run in CI.

### SF-02 — the 01-06 feed grep gates do not exist as committed gates (relates to T-01-23/01-06 high, T-01-24/01-06 medium, T-01-20 low)

Plan 01-06 declares "Enforced by a grep gate" on three rows. The properties are all verified
present today (see the register rows above), but no committed test or CI step asserts them. Same
class as SF-01: absent regression durability, not an absent control. Of these, T-01-23 (01-06) is
the most consequential — it protects the product's core integrity claim (never render a trust
badge the data cannot support).

### SF-03 — execution-time-only scope gates (relates to T-01-24/01-08, T-01-29, T-01-34)

Four rows cite `git diff --quiet` byte-unchanged gates. These are inherently execution-time checks
and cannot be re-run retroactively against a later tree. Recorded for completeness. Each row's
protected property was independently re-verified by this audit through direct inspection rather
than through the gate, and all hold.

### SF-04 — the committed SRI gate checks the hash *prefix*, not the hash *value* (relates to T-01-37, high)

`TestVendorMapScriptsLoadInDependencyOrder` asserts
`strings.Contains(window, "integrity=\"sha256-")`. A garbage, stale, or CDN-self-sourced hash
passes that assertion identically to a correct one. The one property 01-11's threat model called
out as load-bearing — *"a hash read from the CDN's own metadata endpoint is the CDN vouching for
itself and enforces nothing meaningful"* — is therefore not covered by any committed gate; it was
covered only by an execution-time verify step.

- **Status:** not a live gap. This audit independently re-downloaded all five vendor URLs and
  recomputed each sha256; **all five match**, so the pinned values are genuine and the browser's
  SRI enforcement is active. T-01-37 is closed on that recomputation, not on the prefix check.
- **Residual risk:** a future version bump that pastes a wrong hash ships a page whose map silently
  dies (SRI fails closed — the script is refused, not executed). Fails safe, but fails silently.
- **Suggested remediation (implementation change — NOT applied by this audit):** either pin the
  five expected hash *values* as constants in `web/template_contract_test.go` (offline, no network
  in CI), or add a periodic/manual `make verify-sri` target that re-downloads and recomputes.

---

## Unregistered Flags

**None.**

Twelve of the fifteen SUMMARY files (01-01, 01-02, 01-03, 01-04, 01-07, 01-08, 01-10, 01-11,
01-12, 01-13, 01-14, 01-15) contain no `## Threat Flags` section at all; the three that do (01-05,
01-06, 01-09) each declare "None" with reasoning. Because a missing section is not the same as a
declared "None", the absence of flags was **not** taken as evidence — every attack surface was
instead re-derived by direct inspection of the shipped code, and each maps to a registered threat:

| Surface found in code | Maps to |
|-----------------------|---------|
| Five unpkg.com vendor assets executing with page privileges | T-01-37, T-01-16 (01-04) |
| `tiles.openfreemap.org` preconnect + style/tile/glyph/sprite fetches | T-01-40, T-01-43, T-01-48 |
| `{s}.tile.openstreetmap.org` raster fallback fetches | T-01-19, T-01-31, T-01-32 |
| Public `/swagger/*` route (no auth) | T-01-16 (01-07), T-01-25 (01-07), T-01-27 (01-07) |
| Public `POST` + `GET /api/reports` (anonymous, unauthenticated) | T-01-05, T-01-07, T-01-08, T-01-21 |
| `?v=` cache-buster query string on 7 local static assets | T-01-15-03, T-01-15-04 |
| `Cache-Control` branch on `ENV` | T-01-15-02 |
| Nine same-origin `mask-image` SVG sources | T-01-13-03, T-01-14-01, T-01-15-05 |

**Note (informational, not a threat):** `.github/workflows/ci.yml:36` pins `go-version: '1.25'`
while `go.mod:3` requires `go 1.26.4`. With no `toolchain` directive in `go.mod` and no
`GOTOOLCHAIN` override in the workflow, Go's default `GOTOOLCHAIN=auto` will download and switch to
1.26.4, so CI does still run. It is nonetheless a fragility worth pinning explicitly, because every
test-enforced mitigation in this register depends on that `go test` step actually executing.

---

## Accepted Risks Log

Every `accept` disposition in the phase, with the rationale authored in its originating plan.

| Risk ID | Threat Ref | Plan | Severity | Rationale | Accepted By | Date |
|---------|------------|------|----------|-----------|-------------|------|
| AR-01 | T-01-13 | 01-02 | low | Glyphs are repo-committed authored assets, never user-supplied, referenced as static file paths rather than parsed from a response body. Re-evaluate if a later phase ever inlines user-influenced SVG | Solo developer | 2026-09-09 |
| AR-02 | T-01-05 | 01-03 | low | `SameSite=Lax` prevents the session cookie riding a cross-site form POST. No CSRF token added because anonymous report creation carries no account-takeover consequence and there is no authenticated action to ride (01-RESEARCH.md A5). **Revisit when Phase 2's voting makes a forged cross-site action consequential** | Solo developer | 2026-09-09 |
| AR-03 | T-01-18 | 01-04, 01-05 | medium | Submitting a precise coordinate is the product's core function; the visitor drags the marker and sees the exact value in the readout before submitting. Geolocation is requested only when the report modal opens, never on page load, and denial is a fully supported path | Solo developer | 2026-09-09 |
| AR-04 | T-01-19 | 01-04 | low | OSM tile requests reveal the viewport to a third party. Unavoidable for a free, no-API-key map, explicitly chosen under PROJECT.md's budget constraint; the tile server sees a viewport, not a report | Solo developer | 2026-09-09 |
| AR-05 | T-01-22 | 01-05 | low | Shelter headcount is optional, self-reported, coarse operational data about a shelter rather than personal data about an individual — it is the point of FOUND-06 | Solo developer | 2026-09-09 |
| AR-06 | T-01-27 (01-07) | 01-07 | low | Swagger "Try it out" issues live writes, but the API is anonymous and public by design and `POST /api/reports` is already reachable by any HTTP client. Phase 4's ROBUST-04 rate limiting is the real control and is correctly scoped there | Solo developer | 2026-09-09 |
| AR-07 | T-01-26 (01-08) | 01-08 | low | No data flow, endpoint, payload, dependency or storage change — the plan edits two stylesheets and adds one test that reads embedded assets | Solo developer | 2026-09-09 |
| AR-08 | T-01-30 | 01-09 | low | No data, endpoint, payload, dependency, credential or storage change — one stylesheet plus one test over already-embedded assets | Solo developer | 2026-09-09 |
| AR-09 | T-01-32 | 01-10 | low | Retina mode fetches four tiles where one was fetched before, on HiDPI clients only. Comfortably inside OSM's tile-usage policy at portfolio-demo traffic. Accepted rather than mitigated because the alternative — a commercial tile vendor with its own quota — was explicitly declined. **Revisit tile hosting as a whole if real traffic materialises** | Solo developer | 2026-09-09 |
| AR-10 | T-01-35 | 01-10 | low | Leaflet's retina branch decrements the layer max zoom 19→18; the map inherits that ceiling. One less zoom level in exchange for higher-detail tiles at every reachable level. Raising the option to 20 would request tiles OSM does not serve, replacing a sharp map with grey squares | Solo developer | 2026-09-09 |
| AR-11 | T-01-36 | 01-10 | low | No endpoint, payload, dependency, credential, session, storage or schema change — one boolean option inserted into two existing constructor calls | Solo developer | 2026-09-09 |
| AR-12 | T-01-38 | 01-11 | medium | Both packages are scoped to or owned by the renderer's own organisation, ruling out the typosquatting surface a bare unscoped name carries; the bridge ships a provenance attestation and trusted-publisher identity, verified during research. No package manager runs in this project at all — these are CDN tags, not installs | Solo developer | 2026-09-09 |
| AR-13 | T-01-39 | 01-11 | medium | CDN and tile-host availability are two single points of failure for the basemap. Accepted deliberately: the app already depended on this same CDN for Leaflet and on a third-party tile host for raster tiles, so this widens an existing dependency rather than creating a new class of one. Markers, feed, submit flow and API are unaffected by either outage | Solo developer | 2026-09-09 |
| AR-14 | T-01-40 | 01-11 | low | The preconnect opens a TLS connection to the tile host on page load even for a visitor who never triggers a tile fetch. The same host receives viewport coordinates moments later on any normal use, so this discloses nothing the tile fetches do not already disclose | Solo developer | 2026-09-09 |
| AR-15 | T-01-42 | 01-11 | low | No endpoint, payload, credential, session, storage or schema change — markup in one template, one Go test file, and documentation | Solo developer | 2026-09-09 |
| AR-16 | T-01-45 | 01-12 | low | A broken or blocklisted GPU driver can grant a WebGL2 context and still fail to render. Not catchable with a try/catch around construction, because Leaflet defers the layer's add hook until the map's first view is set, so the failure surfaces asynchronously. Accepted with the mechanism documented in code rather than papered over with a try/catch that would look like a safety net without being one | Solo developer | 2026-09-09 |
| AR-17 | T-01-47 | 01-12 | low | Leaflet's attribution control inserts its string as markup. This one is a fixed developer-authored literal containing zero user input — same trust level as the OSM credit string already passed to the tile layer — and each link carries `rel="noopener"`. The app's real XSS-relevant path is untouched | Solo developer | 2026-09-09 |
| AR-18 | T-01-48 | 01-12 | low | The trust posture is unchanged in kind: tile requests already leaked approximate viewport to a third-party host. This swaps which third party sees it, not whether one does. The provider requires no account, no API key, and sets no cookie | Solo developer | 2026-09-09 |
| AR-19 | T-01-49 | 01-12 | medium | The renderer adds roughly 275 KB compressed plus new runtime fetches for style, tile manifest, glyph ranges and sprite sheet — around a sevenfold increase in map-library payload, on a connection that may be congested during an actual emergency. Accepted as a stated cost: vector tiles are far smaller per tile and cache across zoom levels, and 01-11's preconnect removes one cold handshake. **Revisit if real-world load times prove bad** | Solo developer | 2026-09-09 |
| AR-20 | T-01-50 | 01-12 | low | No endpoint, payload, dependency, credential, session, storage or schema change — a client-side layer construction rewritten in two files plus one Go test over embedded assets | Solo developer | 2026-09-09 |
| AR-21 | T-01-13-03 | 01-13 | low | All nine mask sources are same-origin paths into the already-public, already-served static tree; a mask reference discloses nothing a direct request for the same file would not. No new origin, credential or header involved | Solo developer | 2026-09-09 |
| AR-22 | T-01-13-SC | 01-13 | low | No package installed from any package manager. Icons, CSS and JS are all already-committed first-party files; no new runtime or build dependency added | Solo developer | 2026-09-09 |
| AR-23 | T-01-14-01 | 01-14 | low | One numeric attribute changed per file, structural edits forbidden. These files are consumed only as CSS `mask-image` sources — a context in which a referenced SVG document is never scripted, never same-origin-scriptable, and contributes nothing but an alpha channel | Solo developer | 2026-09-09 |
| AR-24 | T-01-14-04 | 01-14 | low | Both new contract gates are pure static parses over an embedded `fs.FS` with no network, no subprocess and no filesystem write; worst case on a malformed input is a `t.Fatalf`, which is the intended behaviour | Solo developer | 2026-09-09 |
| AR-25 | T-01-14-SC | 01-14 | low | No package installed from any package manager. Every file touched is already-committed first-party content; no new runtime or build dependency added | Solo developer | 2026-09-09 |
| AR-26 | T-01-15-05 | 01-15 | low | Retroactively registered. The nine `stroke-width` 3→2 reverts are a direct continuation of AR-23's accepted risk: one numeric attribute per file, consumed only as `mask-image` alpha sources, never scripted | Solo developer | 2026-09-09 |

*Accepted risks do not resurface in future audit runs. Four carry an explicit revisit trigger:
AR-02 (Phase 2 voting), AR-09 (real traffic), AR-19 (real-world load times), AR-01 (user-influenced
inline SVG).*

---

## Verification Method

- **ASVS Level 1** — verify each declared mitigation is present in the file cited by its mitigation
  plan. Applied throughout; several rows were verified at greater depth where the control's
  location mattered (e.g. T-01-15-02's boot-time vs per-request resolution).
- **`mitigate`** — grep for the declared pattern in the cited implementation file, plus, where the
  mitigation names a test, confirm that test exists **and** passes **and** the guarded property is
  independently present in the implementation.
- **`accept`** — entry written into the Accepted Risks Log above with the plan-authored rationale;
  where the accept row also names a control (AR-02's `SameSite=Lax`, AR-03's coordinate readout,
  AR-17's `rel="noopener"`), that control was verified present rather than taken on trust.
- **`transfer`** — no threat in this phase carries a `transfer` disposition.
- **Test evidence:** `go test ./...` — all packages green. Named gates re-run individually with
  `-v`: `TestModalBackdropHiddenGuard` PASS, `TestPrimaryMapHasResolvedHeight` PASS,
  `TestVendorMapScriptsLoadInDependencyOrder` PASS, `TestIconGlyphsAreClassDriven` PASS,
  `TestSwaggerSpecCoversRoutes` PASS. **`TestNearbyReportsExcludesSessionID` SKIPPED** (no local
  `DATABASE_URL`) — recorded as a skip, not counted as passing evidence; T-01-02 is closed on
  static evidence instead.
- **Independent SRI recomputation:** all five vendor URLs were re-downloaded and re-hashed during
  this audit (`curl -sL <url> | openssl dgst -sha256 -binary | base64`) rather than trusting the
  template's values or the CDN's own metadata. All five match — see T-01-37 and SF-04.
- **Read-only discipline:** no implementation file was created, modified or deleted. `sqlc generate`
  and `swag init` were deliberately **not** run (they write into `internal/store/sqlc/` and
  `docs/`); the committed `docs/docs.go` and `docs/swagger.json` were grepped directly instead.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open (blocking) | Open (non-blocking) | Run By |
|------------|---------------|--------|-----------------|---------------------|--------|
| 2026-09-09 | 71 | 71 | 0 | 0 | gsd-security-auditor (ASVS L1, block_on: high) |

**Composition:** 45 `mitigate` (all verified present in the cited implementation) + 26 `accept`
(all logged, AR-01 … AR-26) = 71. Zero `transfer`. 4 WARNING-class mitigation shortfalls recorded
(SF-01, SF-02, SF-03, SF-04) — regression-durability gaps, not absent controls, and not counted
toward `threats_open`.

**Per-plan split (mitigate / accept):** 01-01 5/0 · 01-02 1/1 · 01-03 7/1 · 01-04 4/2 · 01-05 1/1 ·
01-06 2/0 · 01-07 3/1 · 01-08 3/1 · 01-09 3/1 · 01-10 3/3 · 01-11 2/4 · 01-12 3/5 · 01-13 2/2 ·
01-14 2/3 · 01-15 4/1.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (AR-01 … AR-26)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter
- [x] ID collisions surfaced explicitly rather than resolved silently (6 collisions, 3 recovered `high` threats)
- [x] Implementation files unmodified

**Approval:** verified 2026-09-09

**Carry-forward to Phase 2:**
1. Land SF-01's markup-sink test — the phase's highest-value missing regression gate.
2. Land SF-02's three feed-list gates, especially the T-01-23 (01-06) trust-claim gate.
3. Re-open AR-02 (CSRF) — Phase 2's confirm/dispute voting is exactly the "consequential
   cross-site action" whose absence the acceptance rests on.
4. Continue threat numbering from `T-01-50` / the `T-01-NN-MM` scheme; never restart a sub-range.
5. Pin CI's `go-version` to match `go.mod`, so the gates enforcing this register cannot silently
   stop running.
6. Strengthen SF-04's SRI gate to assert hash *values*, not the `sha256-` prefix — the current gate
   would pass a wrong hash, and a wrong hash kills the map silently.
7. Author a `<threat_model>` block for every future plan. 01-15 shipped server-side changes
   (`AssetVersion`, an `ENV`-gated `Cache-Control` branch, template query strings) with no threat
   model at all; its register here is retroactive.
