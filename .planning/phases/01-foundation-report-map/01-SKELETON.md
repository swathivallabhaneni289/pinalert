# Walking Skeleton — Pinalert

**Phase:** 1
**Generated:** 2026-09-06

## Capability Proven End-to-End

An anonymous visitor opens the app, taps the floating "+" button, drags a GPS-prefilled marker to
the right spot, submits a location-tagged emergency report, and immediately sees that report as a
severity-coloured pin on the live Leaflet map — with no signup, backed by a real Postgres row,
returned by a real bounding-box + Haversine query, and with the report disappearing on its own once
`expires_at` passes.

That single sentence exercises every tier: browser JS → chi HTTP handler → validation/service layer
→ sqlc/pgx → Postgres → back out through the same read path the map and the list both consume.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language / runtime | Go 1.25+ (`go build ./...`, stdlib `testing`) | Locked in PROJECT.md constraints; stdlib-first keeps the portfolio signal legible and avoids a JS build step entirely |
| HTTP router | `github.com/go-chi/chi/v5` v5.3.2 | Locked in STACK.md. `gorilla/mux` archived Dec 2022. Route-group middleware needed once `/api/*`, `/static/*` and `/swagger/*` want different stacks |
| Database | PostgreSQL 16/17, plain `latitude`/`longitude` `DOUBLE PRECISION` columns, **no PostGIS** | Locked in PROJECT.md. Proximity is a two-stage indexed bbox prefilter + Haversine, per PITFALLS.md Pitfall 1 |
| DB driver / data access | `jackc/pgx/v5` v5.10.0 under `sqlc` v1.31.1 (`sql_package: "pgx/v5"`) | Type-safe params/results while keeping the geo SQL fully hand-written. No ORM — GORM would fight the Haversine/diversity queries |
| Generated code policy | **`internal/store/sqlc/` output is committed to git** | CI then needs no `sqlc` binary; `go build ./...` is the single source of truth for whether the tree compiles |
| Migrations | `pressly/goose/v3` v3.28.0, sequential numbering, embedded via `//go:embed`, run by `cmd/migrate` | STACK.md offered goose-or-golang-migrate; goose resolved prescriptively by 01-RESEARCH.md. Embedding + a dedicated binary means **migrations never run on app boot** (race risk with >1 instance) and CI needs no goose CLI |
| Time handling | `TIMESTAMPTZ` for every time column; `expires_at` materialised at write time; read paths filter `expires_at > now()` | PITFALLS.md Pitfall 5. A stored TTL-hours column would make the read-time gate a computed expression instead of an indexed comparison |
| Identity / auth | Anonymous only. HMAC-SHA256-signed opaque session cookie (`pinalert_session`), `crypto/rand` id, `hmac.Equal` constant-time verify, secret from `SESSION_SECRET`, fail-fast if unset outside `ENV=development` | PROJECT.md access model: no signup, session-first identity. No password/account tier exists in v1 at all, so ASVS V2 does not apply; V3 does |
| Frontend | Server-rendered `html/template` shell + vanilla JS + hand-written CSS. **No npm, no bundler, no build step.** Leaflet 1.9.4 from CDN; 9 Lucide SVGs downloaded once and committed | UI-SPEC Design System. Keeps the deploy artifact a single Go binary + static files, and keeps the later low-bandwidth mode (ROBUST-01) cheap |
| Client data flow | One `GET /api/reports?lat=&lon=&radius_km=` endpoint feeds **both** the map and the list, through a single shared client store (`window.Pinalert`) | 01-RESEARCH.md Pattern 3. Two independently-polled endpoints would race and silently desync the D-06 side-by-side view |
| Theming | CSS custom properties in `web/static/css/main.css`, light + dark declared from day one via `prefers-color-scheme` | D-13 — dark mode is a distinct palette, not an inverted light theme, and retrofitting variables later is expensive |
| Directory layout | `cmd/` (server, migrate) · `internal/{api,service,session,store,testutil}` · `web/{templates,static}` · `docs/` (swag output) | Matches `.planning/research/ARCHITECTURE.md`'s locked structure verbatim |
| Deployment target | Postgres on **Neon or Supabase** (genuinely persistent free tier); Go service on a free web tier. Phase 1 ships a documented full-stack local run (`make migrate && make run`) rather than a live deploy | PROJECT.md Key Decision — Render's free Postgres expires after 30 days and Railway dropped its indefinite free tier; neither survives "portfolio piece that must stay live" |

## Stack Touched in Phase 1

- [ ] **Project scaffold** — `go.mod`, `sqlc.yaml`, `Makefile`, `.github/workflows/ci.yml` (build + vet + test on every push), stdlib `testing` (plan 01-01)
- [ ] **Routing** — chi router serving `GET /`, `GET /static/*`, `POST /api/reports`, `GET /api/reports`, `GET /swagger/*` (plans 01-03, 01-04, 01-07)
- [ ] **Database** — real write (`InsertReport`) and real read (`NearbyReports`, indexed bbox prefilter + Haversine + `expires_at > now()`) against a real Postgres, exercised in CI via a `postgres:16` service container (plans 01-01, 01-03)
- [ ] **UI** — Leaflet map rendering pins from the live API, plus a submission modal with a draggable GPS-prefilled marker that POSTs and makes a new pin appear (plans 01-04, 01-05, 01-06)
- [ ] **Deployment** — documented full-stack local run: `make migrate && make run` against `DATABASE_URL`; hosting target recorded above, actual deploy is not a Phase 1 gate (plan 01-01)

## Out of Scope (Deferred to Later Slices)

Explicitly **not** in the skeleton. This list exists so later phases do not re-litigate Phase 1's
minimalism:

- Confirm/dispute voting, the `VisibilityResolver`, Hidden/Provisional/Live/Retracted states, and
  "mark resolved" — **Phase 2** (TRUST-01..04, 06, 08, 09). Phase 1 stores no votes and shows no
  confirmation counts. The schema must merely not *preclude* them (`timestamptz` columns, a stable
  `session_id` per report, a `geohash` column already populated).
- "Confirmed by N nearby", diversity-weighted counting, geohash cell-size selection, and the
  confidence/reliability signal split — **Phase 3** (TRUST-05, TRUST-07). Phase 1 stores `geohash`
  at precision 8 purely to avoid a later backfill; it commits to no cell-size semantics.
- Offline IndexedDB queue, low-bandwidth text-only mode, toxicity/spam moderation, session/IP rate
  limiting, and the unofficial-service disclaimer — **Phase 4** (ROBUST-01..04, 08).
- GDACS official-feed ingestion, authority badges, and seeded demo mode — **Phase 5**. Phase 1's
  empty map on first visit is a real, expected state with real empty-state copy, not a bug.
- Safety check-ins, needs/offers, triage list, responder claims, receipt codes, shareable cards —
  **Phase 6**.
- Photo attachment, Web Push/VAPID, websockets, embeddings, perceptual hashing — v2, per
  REQUIREMENTS.md.
- Per-category or trust-weighted expiry tuning. Phase 1 ships the flat two-tier severity default
  only (24h critical / 8h low-medium, D-14).
- Background expiry sweep job. Expiry is enforced purely as a read-time SQL predicate (FOUND-05);
  a sweep is a storage-reclamation optimisation, not a correctness mechanism, and is not needed yet.
- A JS test framework. Client-side map/slider behaviour is covered by the manual UAT items in
  01-VALIDATION.md, not by an automated browser harness.
- Live deploy to Neon/Supabase + a hosted web tier. The target is decided and recorded above; the
  Phase 1 gate is a working documented local full-stack run.

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton **without altering the
architectural decisions above** — same router, same store layer, same session mechanism, same
single-read-endpoint pattern, same CSS variable system.

- **Phase 2 — Trust Mechanic Core:** adds a `votes` table and a shared `VisibilityResolver`
  consumed by every read path. Reuses the existing session cookie as the voter identity and the
  existing `reports.geohash` column. Extends `NearbyReports`, does not fork a second read endpoint.
- **Phase 3 — Diversity-Weighted Trust:** groups votes by distinct `geohash` cell (truncating the
  precision-8 value stored from Phase 1) and adds the confidence/reliability signal pair to the
  same list-row and map-pin components built in plans 01-04/01-06.
- **Phase 4 — Robustness:** layers an IndexedDB submit queue over the existing `POST /api/reports`
  contract, a text-only render mode over the existing shared client store, session-first rate-limit
  middleware into the existing chi stack, and the disclaimer into the existing page shell.
- **Phase 5 — Official Feed & Demo Mode:** ingests GDACS rows into the same `reports` read path
  with an `authority` flag, badged using the icon-badge component from plan 01-02.
- **Phase 6 — Coordination:** adds check-ins, needs/offers, and responder claims as new tables and
  new routes under the same `/api` group, ordered by the Phase 3 trust signals.

---

*Phase: 01-foundation-report-map*
*Walking Skeleton recorded: 2026-09-06*
