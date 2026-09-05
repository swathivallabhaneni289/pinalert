# Stack Research

**Domain:** Location-based, crowd-verification web app (Go + PostgreSQL + Leaflet), solo portfolio build
**Researched:** 2026-09-05
**Confidence:** MEDIUM (web-search-verified across multiple corroborating sources incl. official docs/GitHub; version numbers verified directly against the Go module proxy and npm registry, not inferred from search snippets)

## Validation of Existing Choices

The team's pre-selected stack (Go, PostgreSQL with plain lat/lon, `html/template` + vanilla JS, Leaflet/OSM, Railway/Render) is **validated as sound for this project's scope and constraints**. No changes recommended to the core stack. This document fills in the specific libraries/versions/patterns needed to execute it, plus the five external-API/library gaps called out in the milestone context (geohash, moderation, image safety, embeddings, image hashing, push).

**Versioning note:** every version below was checked directly against `proxy.golang.org` (Go module proxy, authoritative for Go modules) or the npm registry on 2026-09-05, not taken from search-result text — several web-search snippets referenced stale versions (e.g. one dependabot PR title implied `sqlc` was at v1.19; the proxy shows v1.31.1 is current).

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25+ (latest stable is 1.27.1, released Aug 2026 per go.dev/VERSION) | Backend language | Already chosen. Build on whatever the current stable release is at build time — Go 1.22 introduced method+wildcard routing in `net/http.ServeMux`, which is relevant to the router decision below. |
| PostgreSQL | 16 or 17 | Primary datastore | Already chosen (no PostGIS). Both Railway and Render offer managed Postgres 16/17; either is fine — no feature in this project needs anything Postgres-version-specific. |
| Render (web service) + a paid always-on Postgres add-on, or Railway Hobby (~$5/mo all-in) | current platform tiers as of 2026-09 | Hosting | **Verdict: pick one now, don't default to "whichever free tier."** Render's free managed Postgres **expires after 30 days** — fatal for a portfolio project meant to stay live for recruiters/interviewers indefinitely. Railway no longer offers an indefinite free tier at all (one-time trial credit, then ~$5/mo minimum). **Recommendation:** deploy the Go web service on Render's free tier (750 free hours/mo covers a single always-on instance, just accept the 15-min-idle spin-down / cold-start on the first request after inactivity) paired with Render's **paid** Postgres tier (cheapest paid plan, a few dollars/month) so the database never expires — or, if $0 total is a hard requirement, use Railway's Hobby plan (~$5/mo, includes both web + Postgres) instead of stitching together two vendors' free tiers that don't compose (Render's free DB conflicts directly with "stay live indefinitely"). Either way, treat "the demo must never go dark" as a real requirement, not a footnote. |
| `github.com/go-chi/chi/v5` | **v5.3.2** (verified via Go module proxy, released 2026-08-20) | HTTP router / middleware | **Confidence: MEDIUM.** `gorilla/mux` was effectively discontinued in Dec 2022 (archived, no more releases) — do not start a new project on it. Chi is the de facto successor for Go REST APIs: radix-tree routing, a mature middleware ecosystem (`chi/middleware` for logging, recoverer, request-ID, CORS, rate-limiting), and route-grouping that maps cleanly onto an API-first design (`/api/reports`, `/api/confirm`, etc. behind versioned/prefixed groups). Actively maintained (last release within the past month), Go 1.20+ support. |
| `github.com/jackc/pgx/v5` | **v5.10.0** (verified via Go module proxy, released 2026-06-03) | PostgreSQL driver | **Confidence: MEDIUM.** `pgx` has superseded `lib/pq` (now in maintenance-only mode) as the standard Go Postgres driver — better performance, native support for Postgres types, and a connection-pool (`pgxpool`) built in. Use it either directly or as the driver underneath `sqlc` (see below). |
| `sqlc` | **v1.31.1** (verified via Go module proxy, released 2026-04-22) | SQL → type-safe Go code generator | **Confidence: MEDIUM.** Write plain SQL queries in `.sql` files; `sqlc generate` produces typed Go structs/functions. This is the modern recommended alternative to hand-rolling `database/sql` boilerplate or pulling in a heavyweight ORM like GORM — you keep full control of the SQL (needed for the Haversine/geohash queries below) while getting compile-time-checked param/result types. Pairs directly with `pgx` (`sqlc.yaml` → `sql_package: "pgx/v5"`). |
| Leaflet.js | **1.9.4** (verified via npm registry `leaflet@latest`, no 2.x shipped as of 2026-09) | Map rendering | Already chosen. No API key, no billing, pairs with free OSM tile servers (mind OSM's tile-usage policy at higher traffic — a portfolio demo is well within it). |
| `html/template` | stdlib | Server-rendered HTML | Already chosen. Auto-escapes by default (XSS-safe), no added dependency. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/mmcloughlin/geohash` | **v0.10.0** (verified via Go module proxy; last released 2020 — the API is small and stable, low churn is expected/fine for a geohash-encoding library, not a sign of abandonment) | Geohash encode/decode, bounding boxes | Encode each report's lat/lon to a geohash string on write (store as an indexed column). Use `geohash.Neighbors()`/`BoundingBox()` for proximity pre-filtering, and use the geohash-cell value directly for **diversity-weighted confirm counting** (`COUNT(DISTINCT geohash_cell)` per report instead of raw vote count — this is a named Active requirement). Note: geohash cells are a fixed grid, so a variable-radius "how far is nearby" query is better served by the indexed lat/lon bounding box (see Geo-Distance section) — use geohash primarily for the diversity-weighting count and coarse pre-filtering, not as the sole distance mechanism. |
| `github.com/pressly/goose/v3` **or** `github.com/golang-migrate/migrate/v4` | goose **v3.28.0** / migrate **v4.19.1** (both verified via Go module proxy) | DB schema migrations | Pick one, don't build your own. `golang-migrate` is the more widely adopted, purely-SQL, language-agnostic option. `goose` additionally supports Go-func migrations (useful only if you need to backfill/transform data programmatically, e.g. seeding demo/replay data). For this project's plain-SQL schema, either works; **goose** has a slightly gentler CLI for a solo dev. Run migrations as an explicit deploy step/CLI invocation, never on app boot (avoids race conditions if you ever run >1 instance). |
| `github.com/SherClockHolmes/webpush-go` | **v1.4.0** (verified via Go module proxy, released 2025-01-02 — note the module path is case-sensitive and must be imported with the exact `SherClockHolmes` capitalization) | Web Push + VAPID (server side) | For the PWA alert-radius push feature. RFC 8291 (payload encryption) + RFC 8292 (VAPID auth) compliant; includes `GenerateVAPIDKeys()`. This is the most widely used Go web-push library. Store the subscription (`endpoint`, `p256dh`, `auth` keys) per opted-in session on the browser's `PushSubscription`, POST to your API, and send via this library from a Go goroutine when a matching-radius critical report is created. |
| `github.com/corona10/goimagehash` | **v1.1.0** (verified via Go module proxy, released 2022-05-26) | Perceptual image hashing | For the reshare/duplicate-image flag stretch feature. Use `PerceptionHash` (phash) as primary (most robust to compression/resizing) with `Distance()` (Hamming distance) against a threshold (~10 bits on a 64-bit hash is a common starting point) to flag near-duplicate/reshared photos against other Pinalert uploads and a small seeded corpus of known viral hoax images. Only needed once photo attachment ships (currently stretch). |
| plain `time.Ticker` in a goroutine | stdlib | Scheduled jobs | For the GDACS/official-feed poller and the auto-expiry sweep. A `time.Ticker`-driven goroutine is sufficient at this scale (single instance, no distributed-cron need) and avoids an extra dependency entirely. **Not recommending `robfig/cron`** despite being the standard cron-expression library historically — its latest tagged release (v3.0.1) is from January 2020, six years stale with no further activity; for a project this small, a stdlib `time.Ticker` needs no cron-expression parsing anyway. |
| `golang.org/x/time/rate` | latest `golang.org/x/time` | IP-based rate limiting | For the anonymous-posting-with-IP-rate-limiting requirement. Standard token-bucket limiter; wrap as chi middleware keyed by IP (or IP-hash, to match the "IP-hash voter identifier" data model already planned). |
| `github.com/swaggo/swag` + `github.com/swaggo/http-swagger` | swag **v1.16.6** (verified via Go module proxy, released 2025-07-28) | OpenAPI/Swagger docs generation | For the "OpenAPI/Swagger docs for the JSON API" requirement — generates an OpenAPI spec + Swagger UI from Go doc-comment annotations on your chi handlers, no separate spec-writing by hand. |

### External APIs (moderation / embeddings / dedup)

| Service | Purpose | When to Use | Notes |
|---------|---------|-------------|-------|
| **OpenAI Moderation API** (`omni-moderation-latest`) | Text toxicity/spam filter **and** image-safety check | Auto-moderation on submit (both the text-moderation and image-safety Active requirements) | **Confidence: MEDIUM** (pricing/capability confirmed against OpenAI's own docs pages, surfaced via web search — treat as HIGH-reliability content, MEDIUM tier reflects the retrieval method). Free to call. Released Sept 2024, GPT-4o-based, multimodal — accepts image input (up to 20 MB) alongside text in the same call, covering both required checks with **one vendor, one API key, one integration**, which materially simplifies the confidence-cascade logic you're already planning (single set of category scores to threshold against your false-positive budget). Categories: hate, harassment, self-harm, sexual, violence (image support currently covers violence/self-harm/sexual; text covers the full category set). **What NOT to use:** Google's **Perspective API** — despite being free and widely used historically (Reddit, NYT, Wikipedia), Google has announced it will shut down Dec 31, 2026 with no migration tooling. Do not build a new dependency on it for a project that will still be a live portfolio piece past that date. |
| **OpenAI Embeddings API** (`text-embedding-3-small`) | Near-duplicate report-text clustering | Duplicate-report clustering (stretch feature) | **Confidence: MEDIUM.** $0.02 per 1M tokens ($0.01/1M via the Batch API) — trivially cheap at this project's volume. 1536 dimensions, 8192-token context (report descriptions are short, well within limits). OpenAI's embeddings are pre-normalized to unit length, so cosine similarity reduces to a plain dot product — cheap to compute in Go without a vector-math library. Store embeddings in a plain array/JSON column; compare new-report embeddings against recent same-area reports; **no pgvector/PostGIS-style extension needed** at this project's scale — a linear scan over the last N hours' reports in a given geohash cell is fine. |

## Geo-Distance Approach (No PostGIS)

Two viable options, both avoiding PostGIS as intended:

1. **Raw Haversine in SQL** — implement the Haversine formula as a plain SQL expression (or a small `CREATE FUNCTION`) directly on `latitude`/`longitude` columns. Pre-filter with an indexed **bounding-box** range query (`latitude BETWEEN ... AND ... AND longitude BETWEEN ... AND ...`, backed by a plain B-tree index on each column, or a composite index) before computing exact Haversine distance on the much smaller candidate set — this avoids a full-table distance computation on every "nearby" query. **Confidence: MEDIUM.**
2. **`cube` + `earthdistance` contrib extensions** — these ship in Postgres's standard `contrib` module (already present on Railway/Render managed Postgres, just `CREATE EXTENSION cube; CREATE EXTENSION earthdistance;` — no separate install, unlike PostGIS). Gives `ll_to_earth()`, `earth_distance()`, and `earth_box()` with **GiST-indexed** proximity search out of the box. Accuracy: assumes a perfectly spherical Earth, ~3 km error over a 1,400 km distance (negligible for city/district-scale flood/cyclone reporting). **Confidence: MEDIUM (source: official PostgreSQL docs, HIGH-reliability content).**

**Recommendation:** Use the **indexed lat/lon bounding box + exact Haversine (option 1) as the primary "how far is nearby" proximity query** — it supports an arbitrary/variable radius cleanly, which geohash's fixed grid cells don't. Use **geohash (`mmcloughlin/geohash`) specifically for the diversity-weighted confirm count** (`COUNT(DISTINCT geohash_cell)`), which is what it's actually needed for per the Active requirements, and optionally as a coarse first-pass filter (matching/neighboring cells) before the bounding-box+Haversine pass on large tables. If bounding-box-plus-Haversine performance ever becomes a real bottleneck at higher report volume, `earthdistance` (option 2) is the fallback with proper indexing — not PostGIS.

## Web Push (VAPID) + Background Sync — PWA Specifics

- **Web Push:** `SherClockHolmes/webpush-go` on the server; standard `PushManager.subscribe()` + VAPID public key on the client service worker. Well-supported on Chrome/Edge/Firefox desktop and Android. **iOS/Safari is materially more restrictive** (confirmed via web search against multiple 2025-2026 sources): push notifications require **iOS 16.4+** and **only work for web apps added to the Home Screen** — a regular Safari tab cannot receive push at all. Safari 18.4 (2025) added "Declarative Web Push," a simpler mechanism that skips the service-worker round-trip for basic notifications, but the Home-Screen-install requirement still applies. **Implication for this project:** the alert-radius push feature is already correctly scoped to the PWA phase (after the install-to-home-screen step exists) — just don't expect push to work for iOS users who only ever visit via a browser tab, and message this clearly in-product (e.g. "install to your home screen to enable alerts" prompt on iOS Safari).
- **Background Sync:** **Confidence: MEDIUM — this is a real gap to plan around.** The Background Sync API (`ServiceWorkerRegistration.sync`) has **no Safari support on macOS, iOS, or iPadOS as of 2026** (confirmed via caniuse.com, a HIGH-reliability compatibility-tracking source, plus corroborating articles), and Apple has stated no public plan to add it. Only Chromium-based browsers (Chrome, Edge, Samsung Internet) support it — roughly 76% global browser share, but a meaningful gap for iOS users specifically. **Recommendation:** implement the offline-report queue with an **IndexedDB queue + `online` event listener as the fallback retry mechanism**, and layer real Background Sync on top as a progressive enhancement where available (`if ('sync' in registration) {...}`). Don't build the offline-queue feature assuming Background Sync alone will cover all users.

## Installation

```bash
# Core
go get github.com/go-chi/chi/v5@v5.3.2
go get github.com/jackc/pgx/v5@v5.10.0

# Code generation (installed as a dev tool, not a module dependency)
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

# Migrations (pick one)
go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
# or: go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

# Supporting libraries
go get github.com/mmcloughlin/geohash@v0.10.0
go get github.com/SherClockHolmes/webpush-go@v1.4.0
go get github.com/corona10/goimagehash@v1.1.0
go get golang.org/x/time/rate

# OpenAPI docs
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go get github.com/swaggo/http-swagger
```

No `npm install` needed for the frontend — Leaflet 1.9.4 and vanilla JS are loaded via CDN `<script>`/`<link>` tags in the `html/template` layout, consistent with the "no build step" simplicity of this stack.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| `go-chi/chi` | Go 1.22+ stdlib `net/http.ServeMux` | If you want **zero routing dependencies**: Go 1.22 added method (`GET /path`) and wildcard (`/{id}`) pattern matching to the stdlib mux, closing most of the historical gap with third-party routers. Reasonable for this project's route count, but you lose chi's middleware-grouping ergonomics (route-scoped rate-limit/logging middleware), which is convenient given the moderation/rate-limiting needs here. Chi is still the better fit once you have >~15 routes or need per-group middleware. |
| `sqlc` + `pgx` | `sqlx` | If you'd rather write query-building helpers by hand instead of a code-gen step; `sqlx` is a lighter wrapper over `database/sql` with struct-scanning. Less type-safety than `sqlc`, but avoids the `sqlc generate` build step — reasonable trade for a very small schema. |
| `sqlc` + `pgx` | GORM | Avoid for this project — GORM's ORM abstraction fights against writing the custom geo/diversity-weighting SQL this app's trust mechanic depends on, and its query-building adds overhead you don't need at this scale. |
| Bounding-box + Haversine | `cube`/`earthdistance` contrib extensions | If hand-written Haversine SQL becomes a maintenance burden or you want GiST-indexed radius queries out of the box, `earthdistance` gives cleaner radius-based queries without installing PostGIS. |
| OpenAI Moderation API | Standalone third-party moderation vendors (Moderation API, Sightengine, Hive) | If you need moderation categories beyond OpenAI's list, non-English-language coverage OpenAI doesn't handle well, or want a vendor independent of your LLM provider. Adds a second vendor/API key for a portfolio project where OpenAI's free single-call multimodal option is simpler to justify in a design write-up. |
| Render (web) + Render or Railway (Postgres) | Fly.io | Fly.io is a reasonable third option for Go apps (good Docker-native deploy story) but wasn't in the original shortlist and adds no clear benefit here; not evaluated in depth. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| `gorilla/mux` | Repo has been archived/unmaintained since Dec 2022; no further releases or security fixes. | `go-chi/chi/v5` |
| `lib/pq` | In maintenance-only mode; no new features, and slower than pgx in modern benchmarks. | `jackc/pgx/v5` |
| PostGIS | Explicitly out of scope per project constraints — real infra overkill at this project's scale (single-city/region proximity queries, no polygon/routing geometry needed). | Plain lat/lon bounding-box + Haversine (with geohash for diversity-weighting), or `cube`/`earthdistance` contrib extension if needed |
| Google Perspective API | Announced shutdown Dec 31, 2026, no migration tooling provided — do not start a new dependency on a sunsetting API for a project meant to stay live/demoable past that date. | OpenAI Moderation API (`omni-moderation-latest`), free |
| Relying solely on Background Sync for the offline-report queue | No Safari/iOS support at all (any version) as of 2026 — a meaningful fraction of mobile users on this exact use case (emergency reporting from a phone) would silently get no offline queueing. | IndexedDB queue + manual `online`-event retry as the baseline, Background Sync as progressive enhancement only |
| Assuming Web Push works from a regular iOS Safari tab | It doesn't — iOS requires the site be installed to the Home Screen (iOS 16.4+) before push works at all. | Gate the push opt-in prompt behind an "add to home screen" step on iOS, and treat push as PWA-only across all platforms for consistency |
| Running DB migrations automatically on app startup | Race conditions if the platform ever spins up >1 instance (e.g. Render/Railway autoscale); also makes rollback harder to reason about. | Run `goose`/`migrate` as an explicit step in the deploy script/CI pipeline |

## Stack Patterns by Variant

**If photo attachment (stretch) ships:**
- Add `corona10/goimagehash` to the submit pipeline immediately after the OpenAI image-safety check (safety gate first, hash-dedup second — no reason to hash an image you're about to reject).
- Store the phash alongside the report row; compare new uploads against a rolling window (not the whole historical table) to keep the dedup check cheap.

**If duplicate-report text clustering (stretch) ships:**
- Batch embedding calls where possible (submit-time is fine at this project's volume — no need for the Batch API's 24h turnaround).
- Scope the similarity comparison to reports in the same/neighboring geohash cells and same category within a recent time window — don't compare every new report against the entire table.

**If you outgrow a single Go instance (unlikely at portfolio scale, but noted for completeness):**
- The background-job goroutines (GDACS poller, expiry sweep) would need to move to a single-leader pattern (e.g. Postgres advisory lock) to avoid duplicate work across instances — not needed at v1 scale, flagged only so it isn't a silent landmine if traffic ever justifies scaling out.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `sqlc` v1.31.1 | `pgx/v5` v5.10.0 | Set `sql_package: "pgx/v5"` in `sqlc.yaml`; sqlc has first-class pgx/v5 support (not just database/sql compatibility mode). |
| `go-chi/chi/v5` v5.2.1+ | Go 1.20+ | As of chi v5.2.1, the library dropped support for Go versions older than the four most recent majors — fine given the recommendation to build on Go 1.25+. |
| `SherClockHolmes/webpush-go` v1.4.0 | Go 1.18+ | No unusual constraints; standard VAPID/RFC 8291 implementation. Note the case-sensitive import path. |
| Postgres `cube`/`earthdistance` | Postgres 9.1+ (bundled in `contrib` since early versions) | Both extensions ship with standard Postgres distributions on Railway/Render managed Postgres — no separate package install needed, unlike PostGIS which typically requires a different managed-Postgres image/add-on. |

## Sources

- [go-chi/chi releases](https://github.com/go-chi/chi/releases) + Go module proxy (`proxy.golang.org/github.com/go-chi/chi/v5/@latest`) — version verified directly (HIGH — authoritative proxy)
- [Calhoun.io: Go's 1.22+ ServeMux vs Chi Router](https://www.calhoun.io/go-servemux-vs-chi/) — stdlib-vs-chi tradeoff (MEDIUM)
- [go.dev/blog/routing-enhancements](https://go.dev/blog/routing-enhancements) — official Go blog on 1.22 ServeMux changes (HIGH — official source)
- Go module proxy (`proxy.golang.org`) — direct version lookups for `pgx/v5`, `sqlc`, `goose/v3`, `migrate/v4`, `swag`, `cron/v3`, `geohash`, `goimagehash`, `webpush-go` (HIGH — authoritative for Go module versions, queried 2026-09-05)
- `go.dev/VERSION?m=text` — current Go release, go1.27.1 (HIGH — authoritative)
- npm registry (`registry.npmjs.org/leaflet/latest`) — Leaflet 1.9.4 confirmed as latest, no 2.x (HIGH — authoritative)
- [sqlc docs: Using Go and pgx](https://docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html) — sqlc+pgx integration pattern (MEDIUM, official project docs, version references superseded by direct proxy lookup above)
- [brandur.org/sqlc](https://brandur.org/sqlc) — real-world sqlc/pgx adoption rationale (MEDIUM)
- [PostgreSQL official docs: F.14 earthdistance](https://www.postgresql.org/docs/current/earthdistance.html) — earthdistance/cube functions and accuracy (HIGH — official Postgres documentation)
- [Hashrocket: Comparing PostGIS and PostgreSQL's earthdistance](https://hashrocket.com/blog/posts/juxtaposing-earthdistance-and-postgis) — accuracy comparison numbers (MEDIUM)
- [mmcloughlin/geohash GitHub](https://github.com/mmcloughlin/geohash) — API surface (MEDIUM)
- [Lasso: Perspective API sunset guide](https://www.lassomoderation.com/blog/perspective-api/) and [moderationapi.com/perspective-api-alternative](https://moderationapi.com/perspective-api-alternative) — Dec 2026 Perspective API shutdown (MEDIUM, cross-checked across multiple moderation-vendor sources)
- [OpenAI: Upgrading the Moderation API with our new multimodal moderation model](https://openai.com/index/upgrading-the-moderation-api-with-our-new-multimodal-moderation-model/) — omni-moderation-latest capabilities (HIGH — official OpenAI announcement, surfaced via web search)
- [OpenAI API docs: Moderation guide](https://developers.openai.com/api/docs/guides/moderation) — free pricing, 20MB image limit (HIGH — official docs)
- [OpenAI API docs: embeddings guide](https://developers.openai.com/api/docs/guides/embeddings) and [text-embedding-3-small model page](https://developers.openai.com/api/docs/models/text-embedding-3-small) — pricing/dims/context window (HIGH — official docs)
- [corona10/goimagehash GitHub](https://github.com/corona10/goimagehash) — phash/dhash/ahash API (MEDIUM)
- [SherClockHolmes/webpush-go GitHub](https://github.com/SherClockHolmes/webpush-go) — VAPID library (MEDIUM)
- [caniuse.com/background-sync](https://caniuse.com/background-sync) — Background Sync browser support table (HIGH — authoritative compatibility-tracking source)
- [testmuai.com: Background Sync Browser Support](https://www.testmuai.com/learning-hub/background-sync-browser-support/) — Safari/iOS gap detail (MEDIUM, corroborates caniuse)
- [Notificare: Web Push in iOS: Add to Home Screen](https://notificare.com/blog/2024/09/16/web-push-in-ios-add-to-home-screen/) and [Pushpad: iOS special requirements for web push notifications](https://pushpad.xyz/blog/ios-special-requirements-for-web-push-notifications) — iOS 16.4+/Home-Screen-install requirement, Safari 18.4 Declarative Web Push (MEDIUM, cross-checked across 3+ sources)
- [Encore: Render vs Railway 2026](https://encore.dev/articles/render-vs-railway) and [justinmckelvey.com: Is Render Free?](https://justinmckelvey.com/blog/is-render-free) — free-tier limits, 30-day Postgres expiry, sleep behavior (MEDIUM, cross-checked across 2+ sources)
- [dev.to: Best Database Migration Tools for Golang](https://dev.to/shrsv/best-database-migration-tools-for-golang-ajf) — golang-migrate vs goose comparison (MEDIUM)

---
*Stack research for: Location-based crowd-verification emergency-info web app (Go/PostgreSQL/Leaflet)*
*Researched: 2026-09-05*
