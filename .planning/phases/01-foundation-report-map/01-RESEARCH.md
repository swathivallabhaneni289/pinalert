# Phase 1: Foundation — Report & Map - Research

**Researched:** 2026-09-06
**Domain:** Greenfield Go web service (chi + pgx/sqlc + Postgres) with server-rendered HTML,
vanilla-JS Leaflet map, anonymous cookie sessions, and OpenAPI docs — first implementation phase
of Pinalert
**Confidence:** MEDIUM (stack/version claims verified directly against the Go module proxy and npm
registry; architectural/pattern claims inherit HIGH confidence from this project's own prior
ARCHITECTURE.md/PITFALLS.md research; a handful of implementation-detail claims — exact Go
session-cookie snippets, exact GitHub Actions YAML, ARIA slider specifics — are WebSearch-sourced
and tagged LOW/MEDIUM individually below)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Report categories (scope correction):**
- **D-01:** Expanded from 5 to **9 categories**: Flood, Earthquake, Fire, Storm/Cyclone damage,
  Road blocked, Power outage, Shelter open, Rescue needed, Other.

**Report submission flow:**
- **D-02:** Location is set by GPS pre-filling a draggable marker on the map; the visitor can drag
  to correct. Not tap-only, not GPS-locked.
- **D-03:** The submission form is a modal that opens over the map (not a separate page).
- **D-04:** Category selection is a 3×3 icon grid, single tap. "Other" gets a generic flag icon.
- **D-05:** Severity (low/medium/critical) is set via a **slider**, not buttons — smooth animated
  transition, each position shows both a number and a label ("1 · Low", "2 · Medium",
  "3 · Critical"). Accessibility non-negotiable: large touch target, high contrast,
  keyboard/screen-reader operable.

**Landing view & layout:**
- **D-06:** Split view — map and list side-by-side on wider screens.
- **D-07:** On narrow/mobile screens, defaults to map view with a one-tap toggle to reveal the
  list.
- **D-08:** List is sorted severity-first (critical always on top), then newest-first within each
  severity band.
- **D-09:** "Submit a new report" is a floating action button, persistent over the map.

**Visual tone:**
- **D-10:** Overall visual direction is **neutral utility** — grayscale/minimal base, color
  reserved almost entirely for severity signaling.
- **D-11:** Severity color mapping is traffic-light: green (low) / amber (medium) / red (critical).
- **D-12:** Category icons (grid + map pins) use simple, minimal glyphs (Lucide/Feather style):
  white glyph on a solid color-coded circular badge. Badge color = report's severity (resolved in
  UI-SPEC), not a fixed per-category hue.
- **D-13:** Dark mode ships from Phase 1, built with CSS variables from the start — a genuinely
  distinct palette, not an inverted light theme.
- **D-16:** Report list rows encode severity with a colored left-border accent + tinted row
  background.
- **D-17:** The expiry fade (D-15) is implemented as **desaturation toward gray** as a report ages
  (Fresh → Aging → Stale), not literal opacity reduction.

**Expiry behavior:**
- **D-14:** Default expiry is two-tiered by **severity**: Critical (any category) = 24 hours;
  Low/Medium = 8 hours. Flat default for Phase 1 only.
- **D-15:** As a report approaches expiry, it visually fades over roughly the last 25% of its
  lifetime, not disappearing abruptly.

### Claude's Discretion
- Exact geohash/session-identity storage mechanism (cookie vs. localStorage) — pick whichever is
  more robust for anonymous, no-signup persistence; not discussed as a user-facing decision.
- Exact CSS variable naming/theming implementation for dark mode — implementation detail.
- Exact icon set/library choice within "simple outlined line icons" — pick one, be consistent
  (UI-SPEC.md already resolved this to Lucide, self-hosted).
- Precise fade curve/easing for expiry and severity-slider transitions.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within Phase 1 scope. Per-category expiry tuning, trust-weighted expiry,
and confirm/dispute-driven fade behavior are correctly scoped to Phase 2/3.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| FOUND-01 | Anonymous session identity issued on first visit (cookie/localStorage), no signup | HMAC-signed cookie pattern below (Security Domain, Code Examples); session table schema in Architecture Patterns |
| FOUND-02 | Submit location-tagged report: 9 categories, severity (low/med/critical), free-text description | Schema + validation rules below; category/severity enum design in Architecture Patterns; UI-SPEC's modal/grid/slider already specify the client side |
| FOUND-03 | View feed filtered to nearby reports via indexed bounding-box + Haversine (not full-table scan) | Bounding-box+Haversine SQL pattern (Code Examples), Pitfall 1 mitigation, index design in Standard Stack/Architecture Patterns |
| FOUND-04 | View reports as pins on Leaflet/OpenStreetMap map | Leaflet wiring pattern (Code Examples), no-build-step static asset structure |
| FOUND-05 | Report stops appearing once expiry passes, checked as a read-time predicate on every read | `expires_at < now()` predicate pattern (Code Examples), Pitfall 5 mitigation (`timestamptz`), Validation Architecture test |
| FOUND-06 | Shelter-open report carries capacity status (Available/Limited/Full/Closed) + optional headcount | Schema decision below (nullable columns on `reports`, server-side conditional validation) |
| OPS-01 | JSON API documented via OpenAPI/Swagger spec at a stable URL | swaggo/swag + http-swagger/v2 wiring (Code Examples) |
| OPS-02 | Every push runs automated tests, `go vet`, and a build check via GitHub Actions CI | GitHub Actions workflow (Code Examples), Validation Architecture |
</phase_requirements>

## Summary

This phase is a from-scratch Go web service: no existing code, no repo structure beyond planning
docs. The project-level stack is already locked in `.planning/research/STACK.md` and
`.planning/research/ARCHITECTURE.md` — this document adds the concrete, phase-1-specific
implementation patterns those documents leave as "fill in at build time": exact project layout,
the anonymous-session mechanism, the exact schema for `reports` (including the shelter-capacity
fields FOUND-06 needs and where `expires_at` gets computed), the bounding-box+Haversine query
shape, how Leaflet/vanilla-JS wires up with zero build step, and how OpenAPI docs + CI get stood
up cheaply.

Two decisions this document resolves that weren't fully specified upstream: (1) **shelter capacity
schema** — nullable columns directly on `reports`, not a separate table or JSONB blob, because at
9 categories with exactly one needing extra fields, normalization is premature; and (2) **the API
contract** — a single `GET /api/reports?lat=&lon=&radius_km=` endpoint must serve both the map and
the list, because UI-SPEC's "always in sync" wide-screen requirement (D-06) breaks if map and list
pull from two independently-queried endpoints.

**Primary recommendation:** Build `internal/store` schema with `timestamptz` columns and a
bounding-box-indexed `reports` table from the first migration (goose, sequential numbering); wire
`sqlc` to `pgx/v5`; issue an HMAC-signed anonymous session cookie from a `SESSION_SECRET` env var
that fails fast if unset; serve one `GET /api/reports` endpoint for both map and list; generate
Swagger 2.0 docs with `swaggo/swag` + `swaggo/http-swagger/v2`; and stand up a GitHub Actions
workflow (`go build`, `go vet`, `go test`, with a `postgres:16` service container) before writing
any application code, since CI is itself a Phase 1 requirement (OPS-02), not an afterthought.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Anonymous session issuance/verification | API / Backend | Browser (cookie storage only) | Session identity must be tamper-resistant (HMAC-signed); the browser only stores an opaque cookie, all trust logic lives server-side — load-bearing for later reputation/rate-limiting phases |
| Report submission (validation, persistence) | API / Backend | Database / Storage | Category/severity/capacity enum validation and `expires_at` computation must happen server-side, never trusted from the client; Postgres persists the row |
| Nearby-reports query (bbox + Haversine) | Database / Storage | API / Backend | The two-stage filter is a SQL-level concern (indexed bbox prefilter, then Haversine); the API layer is a thin pass-through of lat/lon/radius params |
| Map rendering (Leaflet pins) | Browser / Client | — | Leaflet runs entirely client-side against the JSON API response; no server-side map rendering |
| List rendering (feed rows) | Browser / Client | Frontend Server (initial HTML shell) | Initial page load is server-rendered (`html/template`) for a working no-JS skeleton; subsequent polling/updates are client-side JS re-rendering from the same JSON endpoint |
| Severity slider + category grid (UI controls) | Browser / Client | — | Pure client-side interaction (vanilla JS), no server round-trip until submit |
| Expiry enforcement | Database / Storage | API / Backend | `expires_at < now()` is a SQL predicate in every read query (source of truth); no client-side expiry logic — an expired report simply isn't returned |
| OpenAPI/Swagger docs | API / Backend | — | Generated from Go doc-comments on chi handlers (`swag init`), served by the same Go binary at `/swagger/*` |
| CI (test/vet/build) | Build / CD (not a runtime tier) | — | GitHub Actions, external to the running application entirely |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.3.2 [VERIFIED: Go module proxy, released 2026-08-20] | HTTP router/middleware | Already locked in STACK.md; radix-tree routing, route-group middleware fits `/api/reports` + `/swagger/*` cleanly |
| `github.com/jackc/pgx/v5` | v5.10.0 [VERIFIED: Go module proxy, released 2026-06-03] | PostgreSQL driver | Already locked in STACK.md; supersedes `lib/pq`, built-in `pgxpool` |
| `sqlc` | v1.31.1 [VERIFIED: Go module proxy, released 2026-04-22] | SQL → type-safe Go generator | Already locked in STACK.md; keeps full control of the bbox+Haversine SQL while getting typed params/results |
| `github.com/pressly/goose/v3` | v3.28.0 [VERIFIED: Go module proxy, released 2026-09-02] | DB migrations | STACK.md offered goose-or-golang-migrate; **this document resolves it to goose**, prescriptively — see Architecture Patterns |
| PostgreSQL | 16 or 17 (local dev confirmed: 16.14 via Homebrew) | Primary datastore | Already locked; plain lat/lon, no PostGIS |
| Leaflet.js | 1.9.4 [VERIFIED: npm registry, no 2.x shipped] | Map rendering | Already locked in STACK.md; no API key, CDN `<script>` tag, no build step |
| `html/template` | Go stdlib | Server-rendered HTML shell | Already locked; auto-escapes by default (XSS-safe for server-rendered content — see Security Domain for the JS-side gap) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/mmcloughlin/geohash` | v0.10.0 [VERIFIED: Go module proxy; low churn expected for a stable encode/decode library, not abandonment] | Geohash encoding | Compute and store a `geohash` column on `reports` at insert time now, even though Phase 1 has no votes yet — this avoids a backfill migration when Phase 2's diversity-weighted confirm count needs geohash on the *voter's* location; cheap to add to the report row today for future coarse-filtering/consistency |
| `github.com/swaggo/swag` (CLI) | v1.16.6 [VERIFIED: Go module proxy, released 2025-07-28] | Generates Swagger 2.0 spec from Go doc-comments | Run `swag init` as a dev/CI step; annotate chi handlers with `// @Router`, `// @Param`, etc. |
| `github.com/swaggo/http-swagger/v2` | v2.0.2 [VERIFIED: Go module proxy, released 2023-08-30] | Serves Swagger UI + `doc.json` | **Use the `/v2` module path**, not plain `http-swagger` (v1.3.4) — the officially-fetched chi wiring example (see Code Examples) imports `/v2`; STACK.md's plain install line should be corrected to include `/v2` |
| `lucide-static` (npm) | v1.41.0 [ASSUMED — see Package Legitimacy Audit] | Source of the 9 category SVG icons | Not a runtime/npm dependency of the Go app — download 9 SVGs once (`npx lucide-static` or direct unpkg/jsdelivr fetch), commit under `/web/static/icons/*.svg`, no ongoing dependency. Icon names `droplet`, `activity`, `flame`, `wind`, `construction`, `zap-off`, `house`, `life-buoy`, `flag` **confirmed present** in package v1.41.0 [VERIFIED: npm pack + tar inspection, this session] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go-chi/chi` | Go 1.22+ stdlib `net/http.ServeMux` | Zero routing deps, but loses chi's middleware-grouping ergonomics — not worth it once `/api/*` and `/swagger/*` need different middleware stacks |
| `sqlc` + `pgx` | `sqlx` | Lighter, no codegen step, less type-safety — reasonable trade but sqlc's compile-time-checked bbox+Haversine query params are worth the codegen step here |
| `goose` | `golang-migrate` | Both fine for plain-SQL migrations; goose chosen because its CLI (`goose create`, `goose up`) is marginally simpler for a solo dev and Go-func migrations are available if demo-seed data ever needs programmatic backfill (later phase) |
| Bounding-box + Haversine | Postgres `cube`/`earthdistance` extensions | `earthdistance` gives GiST-indexed radius queries out of the box with no separate install; not needed at Phase 1 scale, but documented as the fallback if bbox+Haversine ever becomes a bottleneck |
| Nullable columns on `reports` for shelter capacity | Separate `shelter_details` table, or a JSONB `details` column | A side table or JSONB column is the right call once *multiple* categories need bespoke fields (e.g., v2's water-depth field for flood) — revisit if a 3rd category-specific field set appears; premature now for exactly one category |

**Installation:**
```bash
go get github.com/go-chi/chi/v5@v5.3.2
go get github.com/jackc/pgx/v5@v5.10.0
go get github.com/mmcloughlin/geohash@v0.10.0
go get github.com/swaggo/http-swagger/v2@v2.0.2

go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
go install github.com/swaggo/swag/cmd/swag@v1.16.6
```
No `npm install` for the frontend — Leaflet 1.9.4 loads via CDN `<script>` tag; the 9 Lucide SVGs
are downloaded once and committed as static files, not an npm runtime dependency.

**Version verification:** All Go module versions above were confirmed directly against
`proxy.golang.org/<module>/@latest` in this research session (not taken from search-result text —
this catches stale search snippets, e.g. one earlier pass found a dependabot PR title implying
`sqlc` was at v1.19 when the proxy shows v1.31.1). `lucide-static` was confirmed on the npm
registry (`npm view lucide-static version` → 1.41.0) and its icon contents inspected directly via
`npm pack` + `tar` in this session.

## Package Legitimacy Audit

The `package-legitimacy check` seam only supports `npm`/`pypi`/`crates` ecosystems; Go modules are
not covered. All Go module packages above were instead verified directly against the authoritative
Go module proxy (`proxy.golang.org`) in this session and in the prior STACK.md research pass —
treated as [VERIFIED: Go module proxy], the Go-ecosystem equivalent of the legitimacy gate.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/go-chi/chi/v5` | Go module proxy | active since 2018, released 2026-08-20 | high (de facto Go REST standard) | github.com/go-chi/chi | OK (proxy-verified) | Approved |
| `github.com/jackc/pgx/v5` | Go module proxy | active since 2013, released 2026-06-03 | high (standard Postgres driver) | github.com/jackc/pgx | OK (proxy-verified) | Approved |
| `github.com/pressly/goose/v3` | Go module proxy | active, released 2026-09-02 | high | github.com/pressly/goose | OK (proxy-verified) | Approved |
| `github.com/mmcloughlin/geohash` | Go module proxy | released 2020-04-03 (stable, low-churn API) | moderate | github.com/mmcloughlin/geohash | OK (proxy-verified) | Approved |
| `github.com/swaggo/swag` | Go module proxy | active, released 2025-07-28 | high | github.com/swaggo/swag | OK (proxy-verified) | Approved |
| `github.com/swaggo/http-swagger/v2` | Go module proxy | released 2023-08-30 | moderate-high | github.com/swaggo/http-swagger | OK (proxy-verified) | Approved |
| `lucide-static` | npm | latest version published 2026-09-04 (package itself has 642K weekly downloads, official `lucide-icons/lucide` GitHub org) | 642,098/week | github.com/lucide-icons/lucide | **SUS** (`too-new` — flags the recency of this specific published version, not the package's overall standing) | Flagged — see note below |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** `lucide-static` — flagged only because its most recent
version was published very recently; the package itself is the official Lucide icon distribution
(642K weekly downloads, verified GitHub org `lucide-icons`). **Important context for the planner:**
this is *not* an npm runtime dependency of the Go application — no `package.json` exists in this
project. The actual action is downloading 9 named SVG files once and committing them to
`/web/static/icons/`. The planner should add a `checkpoint:human-verify` task scoped to "confirm
the 9 committed SVG files render correctly and match the UI-SPEC icon-mapping table," not a
supply-chain review of an ongoing dependency.

*The icon names in UI-SPEC.md's mapping table (`droplet`, `activity`, `flame`, `wind`,
`construction`, `zap-off`, `house`, `life-buoy`, `flag`) were confirmed present in
`lucide-static@1.41.0` by this research session — no substitutions needed.*

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │              BROWSER (client)             │
                    │  html/template shell → boots vanilla JS   │
                    │  ┌─────────────┐        ┌──────────────┐  │
                    │  │  Leaflet map │◄──────►│  Report list  │  │
                    │  │  (pins)      │  same  │  (rows)       │  │
                    │  └──────┬───────┘  data  └──────┬────────┘  │
                    │         │         (D-06 sync)    │           │
                    │         └───────────┬───────────┘           │
                    │                     │ fetch()                │
                    └─────────────────────┼────────────────────────┘
                                          │ JSON over HTTP
                    ┌─────────────────────▼────────────────────────┐
                    │               API LAYER (chi)                 │
                    │  GET  /                → html/template shell  │
                    │  GET  /api/reports     → ONE endpoint, both   │
                    │                          map + list read here │
                    │  POST /api/reports     → submit               │
                    │  GET  /swagger/*       → Swagger UI + doc.json│
                    │       ↓ decode/encode only, no SQL here        │
                    └─────────────────────┬────────────────────────┘
                                          │
                    ┌─────────────────────▼────────────────────────┐
                    │            SERVICE LAYER (Go)                  │
                    │  SessionService: issue/verify HMAC cookie      │
                    │  ReportService: validate enum fields,          │
                    │    compute expires_at from severity (D-14),    │
                    │    compute geohash, persist                    │
                    │  FeedService: parse lat/lon/radius,             │
                    │    call store with bbox params                 │
                    └─────────────────────┬────────────────────────┘
                                          │ sqlc-generated Querier
                    ┌─────────────────────▼────────────────────────┐
                    │             STORE LAYER (pgx)                  │
                    │  reports table: lat/lon (indexed bbox cols),   │
                    │    category, severity, description,            │
                    │    shelter_capacity_status, shelter_headcount,  │
                    │    geohash, created_at, expires_at (timestamptz)│
                    │  sessions table: session_id (HMAC-signed),      │
                    │    created_at                                   │
                    └─────────────────────┬────────────────────────┘
                                          │
                    ┌─────────────────────▼────────────────────────┐
                    │              PostgreSQL 16/17                  │
                    │  Query path: bbox prefilter (indexed) →        │
                    │    Haversine on narrowed set →                 │
                    │    WHERE expires_at > now() (read-time gate)   │
                    └─────────────────────────────────────────────────┘
```

A reader can trace the primary use case (submit a report, see it on the map) top to bottom: the
browser POSTs to `/api/reports`, the service layer validates and computes `expires_at`/`geohash`,
the store layer inserts a row; on the next `GET /api/reports` (polled by both the map and the
list), the store applies the bbox+Haversine+expiry filter and both UI panels render from the same
JSON payload.

### Recommended Project Structure

This matches `.planning/research/ARCHITECTURE.md`'s locked project structure exactly — reproduced
here with Phase 1's concrete file list:

```
pinalert/
├── cmd/
│   └── server/
│       └── main.go              # config, DB pool, router wiring, session secret load
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── reports.go        # POST/GET /api/reports
│   │   │   └── page.go           # GET / (html shell)
│   │   └── router.go             # chi router + middleware + swagger mount
│   ├── service/
│   │   ├── report.go             # validation, expires_at computation, geohash
│   │   └── session.go            # HMAC issue/verify
│   ├── session/
│   │   └── cookie.go             # cookie get/set helpers
│   ├── store/
│   │   ├── db.go                 # pgxpool setup
│   │   ├── queries/               # .sql files consumed by sqlc
│   │   │   └── reports.sql
│   │   ├── migrations/            # goose SQL migrations, sequential numbering
│   │   │   └── 00001_create_reports.sql
│   │   └── sqlc/                  # sqlc-generated Go code (gitignored or committed — team choice)
│   └── testutil/
│       └── db.go                 # spin up/truncate test Postgres
├── web/
│   ├── templates/
│   │   └── index.html.tmpl
│   └── static/
│       ├── js/
│       │   ├── map.js            # Leaflet init, marker drag, popups
│       │   ├── modal.js          # report submission modal, severity slider
│       │   └── feed.js           # list rendering, polling
│       ├── css/
│       │   └── main.css          # CSS variables (light/dark), spacing/type scale
│       └── icons/                # 9 committed Lucide SVGs
├── docs/                          # swag-generated: docs.go, swagger.json, swagger.yaml
├── sqlc.yaml
├── .github/workflows/ci.yml
└── go.mod
```

### Pattern 1: `expires_at` is computed and materialized at write time, never derived at read time

**What:** `ReportService.Submit()` computes `expires_at = now() + duration(severity)` (24h for
critical, 8h for low/medium, per D-14) and writes it as a `timestamptz` column. Every read query
filters `WHERE expires_at > now()` — a plain indexed comparison, not a computed expression.
**When to use:** From the very first migration — this is the schema decision Pitfall 5 and
FOUND-05 both hinge on.
**Example:**
```go
// internal/service/report.go
func expiryDuration(severity Severity) time.Duration {
    if severity == SeverityCritical {
        return 24 * time.Hour
    }
    return 8 * time.Hour // low or medium
}

func (s *ReportService) Submit(ctx context.Context, in SubmitInput) (Report, error) {
    now := time.Now().UTC()
    expiresAt := now.Add(expiryDuration(in.Severity))
    gh := geohash.Encode(in.Lat, in.Lon) // mmcloughlin/geohash, stored for future phases
    return s.q.InsertReport(ctx, store.InsertReportParams{
        Category:    in.Category,
        Severity:    in.Severity,
        Description: in.Description,
        Latitude:    in.Lat,
        Longitude:   in.Lon,
        Geohash:     gh,
        CreatedAt:   now,
        ExpiresAt:   expiresAt,
        // ShelterCapacityStatus / ShelterHeadcount only set if Category == ShelterOpen
    })
}
```

### Pattern 2: Shelter capacity is a nullable, category-gated field pair on `reports`

**What:** `shelter_capacity_status` (nullable enum: `available`/`limited`/`full`/`closed`) and
`shelter_headcount` (nullable int) live directly on `reports`, not a side table. Server-side
validation rejects (400, not silent-ignore) a request where these fields are set but
`category != 'shelter_open'`, and requires `shelter_capacity_status` to be present and valid when
`category == 'shelter_open'`.
**When to use:** Now — this is the first instance of a "category-specific field" problem this
project will hit again (v2's flood water-depth field is the same shape). Nullable columns on the
base table is correct at "exactly one category needs extra fields"; revisit as a side table or
JSONB `details` column only once a second or third category-specific field set appears.
**Example (validation):**
```go
func ValidateSubmitInput(in SubmitInput) error {
    if in.Category == CategoryShelterOpen {
        if !in.ShelterCapacityStatus.Valid() {
            return ErrValidation("shelter_capacity_status", "required for shelter_open reports")
        }
    } else if in.ShelterCapacityStatus != "" || in.ShelterHeadcount != nil {
        return ErrValidation("shelter_capacity_status", "only valid for shelter_open reports")
    }
    return nil
}
```

### Pattern 3: One `GET /api/reports` endpoint feeds both the map and the list

**What:** The map (Leaflet pins) and the list (feed rows) both call the exact same
`GET /api/reports?lat=&lon=&radius_km=` endpoint and render from the same JSON array. There is no
separate `/api/map` vs `/api/feed`.
**When to use:** Always for this phase — UI-SPEC D-06's "always in sync" wide-screen requirement
(clicking a list row pans the map; clicking a pin highlights the list row) is trivial if both
panels share one fetched array in client-side JS, and a source of subtle desync bugs if they're
two independently-polled endpoints that can race or paginate differently.
**Trade-offs:** A single endpoint means the client, not the server, applies any list-specific
sort (severity-first per D-08) if that sort differs from the map's natural render order — cheap to
do client-side since the full nearby set is already in memory.

### Pattern 4: `timestamptz` everywhere, `now()` computed server-side only

**What:** Every time column (`created_at`, `expires_at`, and the sessions table's `created_at`) is
`timestamptz`, never bare `timestamp`. Expiry decisions are made by comparing against Postgres's
own `now()` (or Go's `time.Now().UTC()`, consistently), never by trusting a client-supplied
timestamp for "is this expired."
**When to use:** From the first migration — this is Pitfall 5's exact prevention.

### Anti-Patterns to Avoid

- **Two separate endpoints for map vs. list:** breaks D-06's sync guarantee; use one
  `GET /api/reports`.
- **Running goose migrations on app boot:** race condition risk if the platform ever runs >1
  instance (Render/Railway autoscale); run migrations as an explicit CLI/CI step (already flagged
  in STACK.md's "What NOT to Use").
- **Generating a random `SESSION_SECRET` per process when the env var is unset:** silently
  invalidates every existing session on every restart/cold-start — a real risk given Render free
  tier's frequent sleep/cold-start cycle (see `.planning/research/ARCHITECTURE.md` Anti-Pattern 2).
  Fail fast (`log.Fatal`) if `SESSION_SECRET` is unset outside an explicit dev mode.
- **Deriving `expires_at` at read time from a stored "TTL hours" column:** makes the read-time
  predicate a computed expression instead of a plain indexed comparison, and reintroduces the
  timezone-drift risk Pitfall 5 describes. Materialize `expires_at` at insert time instead.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Type-safe SQL access | Hand-written `database/sql` scanning boilerplate, or a full ORM | `sqlc` + `pgx/v5` | Already locked in STACK.md; keeps the bbox+Haversine SQL fully hand-controllable while eliminating manual struct-scanning bugs |
| DB schema versioning | A custom migration runner script | `goose` (CLI + `embed.FS` for the compiled binary) | Reinventing up/down migration tracking is a well-solved problem; goose's CLI is simple enough for a solo dev |
| Session cookie signing | A custom XOR/base64 "signature" scheme | stdlib `crypto/hmac` + `crypto/sha256` + `crypto/subtle.ConstantTimeCompare` | Hand-rolled signing schemes are a classic security anti-pattern (timing attacks, weak constructions); the stdlib primitives are correct and sufficient — no third-party session library needed |
| OpenAPI/Swagger spec authoring | Hand-writing `openapi.yaml` and keeping it in sync with handlers manually | `swaggo/swag` (`swag init` from doc-comments) | Already locked in STACK.md; spec drifts from code the moment it's hand-maintained separately |
| Accessible slider widget | A custom `<div>`-based slider with manual keyboard event handling | Native `<input type="range">` + `aria-valuetext` override via JS | The native element ships correct keyboard behavior (arrow keys, Home/End, Page Up/Down) and `role="slider"` semantics for free; only `aria-valuetext` needs manual wiring to announce the label text alongside the number |

**Key insight:** Every "don't hand-roll" item above already has a locked library choice from
STACK.md or a stdlib primitive — the risk in this phase isn't reaching for a custom solution where
none is needed, it's under-specifying *how* the chosen tool is wired (e.g., which swaggo module
path, which goose numbering scheme) and leaving that ambiguity for the planner to guess at.

## Common Pitfalls

*(Full catalogue in `.planning/research/PITFALLS.md`; the two below are the ones with direct
schema/architecture consequences for Phase 1's first migration.)*

### Pitfall 1: Naive full-table Haversine scan
**What goes wrong:** `SELECT *, haversine(...) AS d FROM reports WHERE d < :radius` forces Postgres
to compute Haversine for every row before it can filter — fine at demo scale, becomes the slowest
endpoint once the table grows.
**How to avoid:** Two-stage filter from the first migration: indexed bounding-box prefilter
(`latitude BETWEEN ... AND ... AND longitude BETWEEN ... AND ...`, backed by an index on
`(latitude, longitude)`), then exact Haversine only on the narrowed candidate set.
**Warning signs:** `EXPLAIN ANALYZE` on the feed query shows `Seq Scan`, not an index scan.

### Pitfall 5: Timezone/expiry bugs
**What goes wrong:** Bare `timestamp` columns silently shift meaning depending on the deploy
environment's local timezone (dev machine IST vs. hosting provider UTC); a report that should have
expired stays visible, or vice versa.
**How to avoid:** `timestamptz` for every time column, `now()` compared server-side only, and one
explicit automated test: create a report with `expires_at` a few seconds in the future, assert it
disappears from `GET /api/reports` after that instant — run in CI (typically UTC).

## Code Examples

### `sqlc.yaml` (pgx/v5)
```yaml
# Source: docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html [CITED]
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/store/queries"
    schema: "internal/store/migrations"
    gen:
      go:
        package: "sqlcgen"
        sql_package: "pgx/v5"
        out: "internal/store/sqlc"
```

### Bounding-box + Haversine query (`internal/store/queries/reports.sql`)
```sql
-- name: NearbyReports :many
-- Source: bounding-box-then-exact-distance pattern, .planning/research/ARCHITECTURE.md
--         + .planning/research/PITFALLS.md Pitfall 1 [CITED: project research]
SELECT *,
       ( 6371 * acos(
           cos(radians(sqlc.arg(lat)::float8)) * cos(radians(latitude))
           * cos(radians(longitude) - radians(sqlc.arg(lon)::float8))
           + sin(radians(sqlc.arg(lat)::float8)) * sin(radians(latitude))
         )
       ) AS distance_km
FROM reports
WHERE expires_at > now()
  AND latitude  BETWEEN sqlc.arg(lat_min)::float8 AND sqlc.arg(lat_max)::float8
  AND longitude BETWEEN sqlc.arg(lon_min)::float8 AND sqlc.arg(lon_max)::float8
HAVING ( 6371 * acos(
           cos(radians(sqlc.arg(lat)::float8)) * cos(radians(latitude))
           * cos(radians(longitude) - radians(sqlc.arg(lon)::float8))
           + sin(radians(sqlc.arg(lat)::float8)) * sin(radians(latitude))
         ) ) <= sqlc.arg(radius_km)::float8
ORDER BY distance_km ASC;
```
Index required (first migration): `CREATE INDEX idx_reports_lat_lon ON reports (latitude,
longitude);` plus `CREATE INDEX idx_reports_expires_at ON reports (expires_at);` so the read-time
expiry predicate (FOUND-05) is also indexed, not a sequential filter.

### goose migration (`internal/store/migrations/00001_create_reports.sql`)
```sql
-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
CREATE TABLE reports (
    id                       BIGSERIAL PRIMARY KEY,
    session_id               TEXT NOT NULL,
    category                 TEXT NOT NULL,
    severity                 TEXT NOT NULL,
    description              TEXT NOT NULL,
    latitude                 DOUBLE PRECISION NOT NULL,
    longitude                DOUBLE PRECISION NOT NULL,
    geohash                  TEXT NOT NULL,
    shelter_capacity_status  TEXT,
    shelter_headcount        INTEGER,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at               TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_reports_lat_lon ON reports (latitude, longitude);
CREATE INDEX idx_reports_expires_at ON reports (expires_at);
CREATE INDEX idx_reports_geohash ON reports (geohash);

CREATE TABLE sessions (
    session_id  TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE reports;
DROP TABLE sessions;
```
Run with sequential numbering: `goose -s create create_reports sql` generates the `00001_...`
filename; apply with `goose postgres "<dsn>" up` as an explicit deploy/CI step, never on app boot.

### Anonymous session cookie (HMAC-signed)
```go
// Source: pattern synthesized from Go stdlib crypto/hmac docs + community writeups
// (calhoun.io/securing-cookies-in-go, sohamkamani.com/golang/session-cookie-authentication)
// [CITED — pattern verified against stdlib API surface, this session]
const cookieName = "pinalert_session"

func NewSessionID(secret []byte) (id, signed string) {
    raw := make([]byte, 16)
    _, _ = rand.Read(raw) // crypto/rand, never math/rand
    id = base64.RawURLEncoding.EncodeToString(raw)
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(id))
    sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
    return id, id + "." + sig
}

func VerifySessionID(signed string, secret []byte) (id string, ok bool) {
    parts := strings.SplitN(signed, ".", 2)
    if len(parts) != 2 {
        return "", false
    }
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(parts[0]))
    expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
    if !hmac.Equal([]byte(expected), []byte(parts[1])) { // constant-time compare
        return "", false
    }
    return parts[0], true
}

// main.go — fail fast, never auto-generate a secret outside dev mode
secret := os.Getenv("SESSION_SECRET")
if secret == "" {
    if os.Getenv("ENV") != "development" {
        log.Fatal("SESSION_SECRET must be set outside development")
    }
    secret = "dev-only-insecure-secret"
}
```
Cookie flags: `HttpOnly: true`, `Secure: true` (in production), `SameSite: http.SameSiteLaxMode`,
long `MaxAge` (e.g., 1 year) since there's no login to re-establish identity.

### Leaflet draggable marker (GPS-prefilled, drag-to-correct — D-02)
```javascript
// Source: leafletjs.com marker API + verified dragend pattern [CITED]
const map = L.map('report-map').setView([defaultLat, defaultLon], 16);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
  attribution: '&copy; OpenStreetMap contributors'
}).addTo(map);

let marker;
navigator.geolocation.getCurrentPosition(
  (pos) => {
    const { latitude, longitude } = pos.coords;
    map.setView([latitude, longitude], 16);
    marker = L.marker([latitude, longitude], { draggable: true }).addTo(map);
    marker.on('dragend', () => {
      const { lat, lng } = marker.getLatLng();
      updateCoordinateReadout(lat, lng); // live readout per UI-SPEC step 2
    });
  },
  () => {
    // Geolocation denied/unavailable — UI-SPEC inline notice + tap-to-place fallback
    showLocationDeniedNotice();
    map.on('click', (e) => {
      if (marker) map.removeLayer(marker);
      marker = L.marker(e.latlng, { draggable: true }).addTo(map);
    });
  }
);
```

### Accessible severity slider (D-05)
```html
<!-- Source: W3C WAI-ARIA APG range-related-properties practices [CITED] -->
<input type="range" id="severity" min="1" max="3" step="1" value="1"
       aria-label="Severity" aria-valuetext="1 · Low">
```
```javascript
const labels = { 1: '1 · Low', 2: '2 · Medium', 3: '3 · Critical' };
const slider = document.getElementById('severity');
slider.addEventListener('input', () => {
  slider.setAttribute('aria-valuetext', labels[slider.value]);
  updateSliderVisual(slider.value); // color ramp + numeric readout, per UI-SPEC
});
```
Native `<input type="range">` provides arrow-key/Home/End/PageUp/PageDown keyboard support and
`role="slider"` semantics for free; `aria-valuenow` is set automatically from the element's value.
Only `aria-valuetext` needs manual JS wiring, because the numeric value alone ("2") doesn't convey
the label ("Medium") that UI-SPEC requires to be announced.

### chi + swaggo/http-swagger/v2 wiring
```go
// Source: github.com/swaggo/http-swagger/blob/master/example/go-chi/main.go [CITED — fetched this session]
import (
    "github.com/go-chi/chi/v5"
    httpSwagger "github.com/swaggo/http-swagger/v2"
    _ "pinalert/docs" // swag-generated
)

// @title Pinalert API
// @version 1.0
// @description Anonymous crowd-verified local emergency report API
// @BasePath /api

func NewRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Get("/swagger/*", httpSwagger.Handler(
        httpSwagger.URL("/swagger/doc.json"),
    ))
    r.Route("/api", func(r chi.Router) {
        r.Post("/reports", handlers.SubmitReport)
        r.Get("/reports", handlers.NearbyReports)
    })
    return r
}
```
`swag` generates a **Swagger 2.0** spec (not OpenAPI 3.x) — this satisfies OPS-01's wording
("OpenAPI/Swagger spec"), but note the version explicitly so nobody expects OAS3-specific tooling.

### GitHub Actions CI (`OPS-02`)
```yaml
# Source: docs.github.com/actions/automating-builds-and-tests/building-and-testing-go [CITED]
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  build:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: pinalert_test
        ports: ["5432:5432"]
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build -v ./...
      - run: go vet ./...
      - run: go test -v ./...
        env:
          DATABASE_URL: postgres://postgres:postgres@localhost:5432/pinalert_test?sslmode=disable
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `gorilla/mux` for routing | `go-chi/chi/v5` | Dec 2022 (mux archived) | Already reflected in STACK.md; no new information this pass |
| Bare `timestamp` columns for time-sensitive data | `timestamptz` always | Long-standing Postgres best practice, re-surfaced by this project's own Pitfall 5 | Prevents deploy-environment-dependent expiry bugs |
| Hand-rolled OpenAPI YAML | Doc-comment-generated spec (`swag init`) | Standard practice for Go+chi/gin/echo stacks | Spec can't drift from handler code silently |

**Deprecated/outdated:** none newly surfaced this pass beyond what STACK.md already documents
(`gorilla/mux`, `lib/pq`, Google Perspective API — the latter is Phase 4 scope, not Phase 1).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exact Go project subfolder names (`internal/service`, `internal/store`, etc.) beyond what ARCHITECTURE.md already locks | Recommended Project Structure | Low — cosmetic naming, easy to rename; ARCHITECTURE.md's higher-level structure is already locked and HIGH confidence |
| A2 | HMAC session-cookie code snippet's exact shape (field names, `.` separator format) | Code Examples | Low — the underlying primitives (`crypto/hmac`, `crypto/rand`, constant-time compare) are correct stdlib usage; exact wire format is an implementation detail the planner/executor can adjust |
| A3 | GitHub Actions YAML's exact structure (job names, `postgres:16` service block syntax) | Code Examples | Low — GitHub's own docs confirm the general pattern; exact YAML may need minor adjustment when actually run |
| A4 | Shelter-capacity schema decision (nullable columns vs. side table) is this document's own reasoning, not sourced from an external authority | Architecture Patterns, Pattern 2 | Medium — reasonable default for "1 category needs extra fields," but if the planner disagrees and prefers a side table now for cleaner future extension, that's a legitimate alternative; flag for discuss-phase if contested |
| A5 | CSRF is not treated as a hard requirement for `POST /api/reports` in Phase 1 (relying on `SameSite=Lax` cross-site POST blocking) | Security Domain | Medium — correct for "anonymous public report creation with no account-takeover risk," but if a reviewer wants an explicit CSRF token regardless, that's a cheap addition; not verified against an authoritative source this session |
| A6 | `sqlc` generates `pgtype.Timestamptz` (not `time.Time`) for `timestamptz` columns by default under `pgx/v5`, requiring either an override or `.Time` field access in handler code | Code Examples / Standard Stack | Low-Medium — this is sqlc's documented default behavior from training knowledge, not independently re-verified against the fetched docs page (the fetch returned a redirect stub, not full content); if wrong, it's a compile error the planner/executor will catch immediately, not a silent bug |

**If this table is empty:** N/A — see entries above; all are LOW-MEDIUM risk implementation
details, none blocks planning.

## Open Questions

1. **Should the `sessions` table row exist before or only after a report/vote is created?**
   - What we know: FOUND-01 requires session issuance on *first visit*, before any report exists.
   - What's unclear: Whether to write a `sessions` row on every first page load (even a visitor who
     never submits) or lazily on first `POST /api/reports`.
   - Recommendation: Write the `sessions` row eagerly on first visit (any `GET /` or `GET /api/*`
     without a valid cookie) — this is what "issued to a first-time visitor" in FOUND-01 literally
     says, and it's needed regardless once Phase 2's reputation tracking wants a session's full
     history, not just its report-submission history.

2. **Should `reports.geohash` use a fixed precision now, even though no vote/confirm logic reads it
   yet in Phase 1?**
   - What we know: PITFALLS.md Pitfall 10 says geohash cell-size is a load-bearing numeric choice
     that needs to be benchmarked against a realistic incident radius (~100-300m) — explicitly
     deferred to the Phase 3 trust-hardening plan in STATE.md's Blockers/Concerns.
   - What's unclear: Whether Phase 1 should pick *any* precision now (to avoid a later backfill) or
     leave the column out entirely until Phase 2/3 resolves the precision question.
   - Recommendation: Store geohash at a reasonably fine precision (e.g., 8 characters, ~19m
     precision) now — finer than needed is always safely truncatable later for the diversity-count
     grouping, while coarser-than-needed would require a full backfill. This avoids the backfill
     risk without pre-committing to the actual cell-size-for-independence-counting decision, which
     stays correctly deferred to Phase 3.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | entire backend | ✓ | 1.26.4 | — |
| PostgreSQL | schema/store layer, local dev + CI | ✓ (local) | 16.14 (Homebrew) | CI uses a `postgres:16` service container (see GitHub Actions example) |
| git | version control | ✓ | 2.55.0 | — |
| Docker | optional local Postgres container, not required if using local Homebrew Postgres | ✓ | 29.6.1 | Local Homebrew Postgres already running and sufficient |
| gh (GitHub CLI) | repo/CI setup convenience | ✓ | 2.96.0 | — |
| Go module proxy access | dependency verification | ✓ (network reachable during this research session) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — all required tools for Phase 1 local development
and CI are present.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `go test` (no third-party test runner — do not add `testify` or similar by reflex; stdlib table-driven tests + `net/http/httptest` are sufficient for this phase's scope) |
| Config file | none — see Wave 0 |
| Quick run command | `go test ./... -short` |
| Full suite command | `go test ./... -v` (requires `DATABASE_URL` pointing at a real Postgres for store-layer tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | First request with no cookie gets a `Set-Cookie`; subsequent request with the cookie reuses the same session id | unit/integration | `go test ./internal/session/... -run TestSessionIssuance -v` | ❌ Wave 0 |
| FOUND-02 | Rejects invalid category/severity enum values; accepts valid combinations | unit | `go test ./internal/service/... -run TestValidateSubmitInput -v` | ❌ Wave 0 |
| FOUND-03 | `EXPLAIN ANALYZE` on the nearby-reports query shows an index scan, not `Seq Scan`, against a seeded large dataset | integration (needs real Postgres) | `go test ./internal/store/... -run TestNearbyReportsUsesIndex -v` | ❌ Wave 0 |
| FOUND-04 | (No dedicated backend test — Leaflet rendering is client-side; covered by manual UAT per Nyquist sampling, not an automated Go test) | manual-only | — | — |
| FOUND-05 | Report with `expires_at` ~2s in the future disappears from `GET /api/reports` after that instant | integration (needs real Postgres, real clock) | `go test ./internal/store/... -run TestExpiryReadTimePredicate -v` | ❌ Wave 0 |
| FOUND-06 | `shelter_capacity_status` required and validated when category=shelter_open; rejected when category≠shelter_open | unit | `go test ./internal/service/... -run TestShelterCapacityValidation -v` | ❌ Wave 0 |
| OPS-01 | `GET /swagger/doc.json` returns 200 with valid JSON | smoke | `go test ./internal/api/... -run TestSwaggerDocServed -v` | ❌ Wave 0 |
| OPS-02 | CI itself running test/vet/build on every push is the verification of this requirement (meta — not a `go test` case) | n/a (CI config) | `.github/workflows/ci.yml` present and green | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -short`
- **Per wave merge:** `go test ./... -v` (full suite, real Postgres)
- **Phase gate:** Full suite green before `/gsd-verify-work`, plus manual UAT for FOUND-04
  (Leaflet map rendering, drag-marker behavior, D-06/D-07 sync/toggle) since map interaction isn't
  meaningfully unit-testable in Go.

### Wave 0 Gaps
- [ ] `go.mod` / module init — nothing exists yet
- [ ] `internal/testutil/db.go` — spin up/truncate a test Postgres connection, shared by all
      store-layer tests (FOUND-03, FOUND-05)
- [ ] `.github/workflows/ci.yml` — with the `postgres:16` service container from Code Examples
- [ ] `sqlc.yaml` + first goose migration — needed before any store-layer test can run
- [ ] Seed-data script/fixture for the FOUND-03 large-dataset index-scan test (a few thousand fake
      rows spread across a wide lat/lon range)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No — Phase 1 is anonymous-only by design, no passwords/accounts | N/A |
| V3 Session Management | Yes | HMAC-signed session cookie (`crypto/hmac`+`crypto/sha256`), `HttpOnly`+`Secure`+`SameSite=Lax`, constant-time verification (`hmac.Equal`), `SESSION_SECRET` from env, fail-fast if unset outside dev mode |
| V4 Access Control | No — no roles/permissions exist in Phase 1; all reads/writes are anonymous and public by design | N/A |
| V5 Input Validation | Yes | Server-side enum validation for category (9 values), severity (3 values), shelter_capacity_status (4 values, conditionally required); lat/lon range bounds (−90..90 / −180..180); description length floor (UI-SPEC: ≥10 chars) — hand-written Go validation functions, no third-party validation library needed at this scope |
| V6 Cryptography | Yes (narrow) | `crypto/hmac`+`crypto/sha256` for session signing, `crypto/rand` (never `math/rand`) for session ID generation — both stdlib, never hand-rolled |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via report fields | Tampering | `sqlc`-generated parameterized queries by construction — no string-concatenated SQL anywhere in the store layer |
| XSS via user-authored description rendered in JS-built DOM (map popups, list rows assembled client-side from the JSON API) | Tampering / Info Disclosure | `html/template` auto-escapes the *initial* server-rendered shell, but map popups and feed rows built by `map.js`/`feed.js` from fetched JSON must use `textContent`, never `innerHTML`, when inserting report description text |
| Session forgery/replay (client crafts an arbitrary session id to inherit an aged/reputable session) | Spoofing | HMAC-signed cookie value, verified server-side with `hmac.Equal` (constant-time); a client cannot construct a valid signature without `SESSION_SECRET` |
| CSRF on `POST /api/reports` | Tampering | `SameSite=Lax` blocks cross-site POST from being sent with the cookie on a cross-origin form submission; not adding an explicit CSRF token this phase since report creation has no account-takeover consequence — flagged as Assumption A5 for discuss-phase if a reviewer wants it hardened further |
| Session-secret rotation/loss on redeploy | Tampering / Availability | `SESSION_SECRET` sourced from a persistent env var (not regenerated per process); losing it invalidates all sessions — acceptable for Phase 1 (no reputation data yet to lose), but the pattern must not change once Phase 2 makes sessions load-bearing for trust scoring |

## Sources

### Primary (HIGH confidence)
- `.planning/research/STACK.md`, `.planning/research/ARCHITECTURE.md`, `.planning/research/PITFALLS.md` — this project's own prior ecosystem research, already locked as canonical references per 01-CONTEXT.md
- `proxy.golang.org/<module>/@latest` — direct Go module proxy queries for chi, pgx, goose, geohash, swag, http-swagger, http-swagger/v2, x/time (this session)
- npm registry (`npm view lucide-static version`) + direct `npm pack`/`tar` inspection of package contents (this session) — icon-name verification

### Secondary (MEDIUM confidence)
- [github.com/swaggo/http-swagger/blob/master/example/go-chi/main.go](https://github.com/swaggo/http-swagger/blob/master/example/go-chi/main.go) — fetched directly this session, chi+swagger wiring pattern
- [github.com/pressly/goose](https://github.com/pressly/goose) — fetched directly this session, CLI commands and migration file format
- [docs.sqlc.dev — Using Go and pgx](https://docs.sqlc.dev/en/v1.31.1/guides/using-go-and-pgx.html) — sqlc.yaml `sql_package: pgx/v5` config (fetch returned a partial/redirect page; core config confirmed, `timestamptz` override behavior not independently re-verified — see Assumption A6)
- [W3C WAI-ARIA APG — Range-related properties](https://www.w3.org/WAI/ARIA/apg/practices/range-related-properties/) — `aria-valuetext` pattern (via WebSearch synthesis)
- [docs.github.com — Building and testing Go](https://docs.github.com/actions/automating-builds-and-tests/building-and-testing-go) — GitHub Actions Go CI pattern (via WebSearch synthesis)
- Bounding-box + Haversine pattern — corroborates `.planning/research/PITFALLS.md` Pitfall 1, cross-checked via WebSearch against multiple independent write-ups (MySQL-authored, database-agnostic pattern)

### Tertiary (LOW confidence)
- Go project layout for chi+pgx+sqlc (WebSearch synthesis, no single authoritative source — superseded by this project's own locked `ARCHITECTURE.md` structure, used only for confirmation)
- Anonymous HMAC session cookie exact code shape (WebSearch synthesis of multiple community writeups — underlying stdlib primitives are correct, exact wire format is this document's own construction)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version verified directly against the Go module proxy or npm registry this session, inheriting STACK.md's prior verification
- Architecture: HIGH — inherits directly from this project's own locked ARCHITECTURE.md; the two new decisions (shelter-capacity schema, single feed endpoint) are reasoned extensions of that architecture, not external claims
- Pitfalls: HIGH — inherits directly from this project's own locked PITFALLS.md
- Implementation-detail code snippets (session cookie, GH Actions YAML, ARIA slider): MEDIUM-LOW — WebSearch-sourced patterns, correct in shape but not copy-pasted from a single authoritative example; flagged individually in the Assumptions Log

**Research date:** 2026-09-06
**Valid until:** 30 days (stable Go-ecosystem stack; re-verify package versions if planning is
delayed past early October 2026)

---
*Phase: 01-foundation-report-map*
*Research completed: 2026-09-06*
