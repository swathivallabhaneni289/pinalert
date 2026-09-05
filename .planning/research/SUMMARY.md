# Project Research Summary

**Project:** Pinalert
**Domain:** Crowd-verified local emergency information feed (floods/cyclones, India context) — anonymous geo-tagged reporting with a confirm/dispute trust mechanic, solo-dev public portfolio build
**Researched:** 2026-09-05
**Confidence:** MEDIUM-HIGH

## Executive Summary

Pinalert is a location-based, crowd-verification web app in the same product family as Citizen, Waze, Ushahidi, and Facebook Safety Check — anonymous users submit geo-tagged emergency reports (floods, cyclones, rescue needs) that gain or lose visibility through a confirm/dispute voting mechanic, with severity-based fast-tracking for genuine emergencies. The pre-selected stack (Go, PostgreSQL with plain lat/lon, `html/template` + vanilla JS, Leaflet/OSM, Railway/Render) is validated as sound for this scope — no PostGIS needed, `go-chi/chi` + `pgx/v5` + `sqlc` for the backend, OpenAI's Moderation API (free, multimodal) covering both text and image safety in one integration. The single architectural decision that determines whether the whole product works is the **VisibilityResolver**: one pure function, called by every read path (feed, map, triage, share-card), that turns moderation state + vote log + severity + timestamp into `{Hidden, Provisional, Live, Retracted}`. Every anti-pattern in the research traces back to *not* centralizing this logic.

The recommended approach sequences work as: (1) foundational data model + reporting loop with correct indexing/timezone handling from day one, since retrofitting the append-only vote log or `timestamptz` columns later is expensive; (2) the confirm/dispute trust mechanic itself, built directly on an independence predicate (distinct session + distinct geohash cell) *before* any diversity-weighting curve, with a mandatory concurrency test; (3) trust-model hardening (provisional gate, diversity weighting, Sybil red-teaming) as its own explicit phase, not a follow-on hardening pass; (4) robustness (rate limiting keyed on session not IP, auto-moderation with a human-reviewable queue, legal disclaimer) before any public promotion; (5) coordination features (triage, responder claim, shareable cards) each carrying their own abuse-model requirement, not just CRUD.

Key risks: a full-table Haversine scan that silently degrades as the table grows (fix: bounding-box index + Haversine on the narrowed set, from the first migration); a lost-update race on the confirm tally under real concurrent voting (fix: append-only vote log is truth, atomic cache update, concurrency test as UAT); IP-based rate limiting that blocks legitimate CGNAT-shared Indian mobile users while doing little against a determined abuser (fix: session-first, IP-secondary); trivially-resettable anonymous sessions gaming the trust score (fix: don't aim for unbeatable, raise the cost, document the limitation); and two features that as specified directly contradict each other — a shareable card with the confirm count baked into a static image defeats the retraction mechanism explicitly built to fix stale-forward misinformation (fix: card carries only a status link, never a baked-in count). Two feature-list corrections also matter for phase planning: "Resolved marking" and the split of "authority badge" and "retraction push" each need to move between Active/Stretch as detailed in FEATURES.md.

## Key Findings

### Recommended Stack

Go 1.25+, PostgreSQL 16/17 (no PostGIS), `go-chi/chi/v5` for routing, `jackc/pgx/v5` + `sqlc` for type-safe SQL, Leaflet 1.9.4 + OSM tiles via CDN (no npm build step), `html/template` for server rendering. Hosting: pick one deployment now (Render web free tier + Render **paid** Postgres, or Railway Hobby ~$5/mo) — Render's free Postgres expires after 30 days, which is fatal for a portfolio project meant to stay live indefinitely.

**Core technologies:**
- `go-chi/chi/v5` — HTTP router/middleware; `gorilla/mux` is unmaintained since Dec 2022, do not use
- `jackc/pgx/v5` + `sqlc` — type-safe SQL without an ORM; keeps full control of the custom geo/trust-scoring SQL that GORM would fight against
- `mmcloughlin/geohash` — diversity-weighted confirm counting (`COUNT(DISTINCT geohash_cell)`), not the primary distance mechanism
- Bounding-box + Haversine (plain B-tree indexes on lat/lon) — the primary "how far is nearby" query; `cube`/`earthdistance` contrib extensions as a fallback if it ever becomes a bottleneck
- `SherClockHolmes/webpush-go` — Web Push/VAPID for the PWA alert-radius feature (Stretch); note iOS Safari requires Home-Screen install before push works at all
- OpenAI Moderation API (`omni-moderation-latest`) — free, multimodal, covers both text toxicity and image safety in one vendor/API key; do not build on Google's Perspective API (shutting down Dec 31, 2026)
- IndexedDB queue + `online`-event retry as the offline-report baseline — Background Sync API has zero Safari/iOS/Firefox support as of 2026, so it can only be a Chromium-only progressive enhancement, never the baseline mechanism

### Expected Features

**Must have (table stakes, largely already in Active scope):** location-tagged reports, categories, severity, map view, auto-expiry, and **Resolved marking** (recommend promoting from Stretch to Active — it's the reporter-driven complement to auto-expiry, one cheap status transition, and directly serves the "feed reflects what's true now" Core Value at negligible cost).

**Should have (differentiators, the Core Value and its support structure):** confirm/dispute voting (the centerpiece), provisional visibility gate, diversity-weighted confirm count, split confidence/reliability decay, "I'm safe" check-in, needs↔offers matching, volunteer/rescue-team triage view, shareable verified-report cards, structured shelter capacity, anonymous reporter receipt code, responder claim + coverage-gap layer, human-contact verification tier, GDACS official-feed integration with authority badging for GDACS-sourced pins, demo/replay mode.

**Corrections to the existing Active/Stretch split (apply before roadmap phasing):**
- Promote "Resolved marking" Stretch → Active
- Add "anonymous session identity" as its own explicit early Active requirement (currently load-bearing but unlisted — reliability score, responder-claim attribution, and rater trust score all assume it exists)
- Split "auto-moderation" — text toxicity/spam filter stays Active; image-safety check moves to Stretch (paired with photo attachment, which doesn't exist yet)
- Split "retraction push" — the passive live-resolving status link stays Active (cheap, reuses the receipt-code primitive); proactive push notification moves to Stretch, chained after Web Push/websockets, with a `report_views` table noted as its schema prerequisite
- Split "authority badge" — badge for GDACS-sourced pins is cheap and can ship in Active alongside GDACS integration; the verified-authority-*account* flow stays deferred to V2/Stretch
- Resolve as an explicit design decision (not deferred): how does diversity-weighted confirm count capture the confirmer's location? (geolocation prompt vs. IP-derived coarse geohash — materially different UX/signal-quality tradeoff)

**Defer (v2+):** missing/found person registry, per-incident coordination channel, CAP-compliant alert feed, digitally-signed authority reports — reasoning already sound in PROJECT-NOTES.md.

### Architecture Approach

Layered Go service: thin API handlers (JSON + HTML side by side) → service layer (all business logic) → store layer (SQL only, no rules), with background jobs (official-feed poller, expiry sweep) calling the same service methods as the request path rather than duplicating insert/validation logic. The single most important component is the **VisibilityResolver** — a pure function with no DB/HTTP calls that takes report state, vote log, moderation state, and current time, and returns exactly one of `{Hidden, Provisional, Live, Retracted}` plus a reason. Every read path (feed, map, triage, share-card) must call this same resolver (directly, or via its cached output column) — this is what prevents the exact anti-pattern this project's own retraction feature exists to fix (share-card path drifting out of sync with the live feed path).

**Major components:**
1. **VisibilityResolver** — the single source of truth for what's visible, called synchronously on every vote/moderation-state change and by every read path
2. **TrustScoringService** — append-only vote log (`confirmations`) as truth; diversity-weighted count and confidence/reliability decay are always *computed*, with a denormalized cache column updated atomically in the same transaction as the vote insert, never via read-then-write
3. **ModerationService** — async confidence-cascade after submit (never inline/blocking); fail-open for critical/rescue-needed reports, fail-closed (Provisional) for everything else
4. **Official feed poller** — idempotent upsert on `(source, external_id)`, bypasses the trust pipeline entirely (crowd votes cannot hide an official GDACS pin — deliberate abuse-surface decision)
5. **FeedService** — bounding-box SQL prefilter + Haversine on the narrowed set, filtered by resolver-computed status, never re-deriving visibility itself

Key patterns: independence predicate (distinct session + distinct geohash cell) must exist before any diversity-weighting curve is built on top of it; expiry is a read-time predicate (`expires_at < now()` in every query), never solely dependent on a sweep job, because Render/Railway free-tier instances sleep after inactivity and an in-process ticker simply won't fire during that window.

### Critical Pitfalls

1. **Naive full-table Haversine scan** — silently fine at demo scale, becomes the slowest endpoint as the table grows. Fix: indexed bounding-box prefilter before Haversine, from the first migration.
2. **Race condition in the confirm/dispute tally** — the Core Value's correctness breaks under real concurrent voting (lost updates). Fix: append-only vote log as truth, atomic cache update in the same transaction, explicit concurrency test as phase UAT (not a later hardening pass).
3. **IP rate limiting mistakes CGNAT-shared carrier IPs for single abusers** — in India, hundreds of real phones share one public IP; a tight per-IP limit throttles legitimate reporters during exactly the surge the product exists for. Fix: rate-limit per anonymous session first, IP as a looser secondary backstop.
4. **Sybil attack on the trust score via trivially-resettable anonymous sessions** — a "clear cookies" attack stacks fake independent confirmations. Fix: don't aim for unbeatable; raise the cost (cross-session-but-same-IP rate limiting, down-weight freshly-created sessions), document the known limitation rather than claiming fraud-proof.
5. **Two features as specified directly contradict each other**: a shareable card with the confirm count baked into a static image structurally defeats the live-resolving retraction link paired with it (a forwarded JPEG cannot be retracted). Fix: card carries only a status URL, never a numeric count or unqualified "verified" claim in the pixels — resolve this as a design decision before the card template is built.

(Also flagged as high-severity but not in the "top 5": self-declared "critical" severity as an unaccountable bypass of the only visibility gate — decouple visibility from triage *priority* ordering; and unaccountable responder-claim/human-contact overrides on exactly the reports where being wrong matters most — require expiry+reversibility, never silent permanent removal from the coverage-gap view.)

## Implications for Roadmap

Based on combined research, suggested phase structure:

### Phase 1: Foundation — data model, reporting loop, map view
**Rationale:** Everything else depends on the schema being right from day one — `timestamptz` (not bare `timestamp`), lat/lon B-tree indexes + bounding-box query pattern, the append-only `confirmations` table shape, and anonymous session issuance are all far more expensive to retrofit than to get right up front.
**Delivers:** Submit a report (location, category, severity), see it on a Leaflet map, anonymous session identity issued (cookie/localStorage), auto-expiry as a read-time predicate.
**Addresses:** Location-tagged reports, categories, severity, map view, auto-expiry, anonymous session identity (surfaced explicitly per FEATURES.md correction).
**Avoids:** Pitfall 1 (naive full-scan query), Pitfall 5 (timezone/expiry bugs) — both are schema-time decisions, not later fixes.

### Phase 2: Trust mechanic — confirm/dispute, provisional gate, resolved marking
**Rationale:** This is the Core Value; it must be built on the independence predicate before any weighting refinement, and needs its own concurrency test as a phase-completion criterion, not a follow-on hardening task.
**Delivers:** Confirm/dispute voting with independence predicate (distinct session + geohash cell), VisibilityResolver as a pure, unit-tested function, provisional visibility gate, critical/rescue-needed fail-open bypass, Resolved marking.
**Uses:** `VisibilityResolver` pattern, append-only vote log + atomic cache update pattern from ARCHITECTURE.md.
**Avoids:** Pitfall 2 (tally race condition — explicit concurrency test required), Pitfall 9 (self-declared critical bypassing the gate — decouple visibility from triage priority now, even if the triage view itself ships later).

### Phase 3: Trust-model hardening — diversity weighting, decay scores, Sybil resistance
**Rationale:** Diversity weighting and the confidence/reliability decay split are refinements on Phase 2's foundation; the geohash precision and Sybil-mitigation choices are load-bearing numeric/design decisions that need dedicated attention, not a library default.
**Delivers:** Diversity-weighted confirm count with a deliberately chosen geohash precision, split confidence/reliability decay, red-teamed Sybil mitigation, explicit decision on how confirmer location is captured.
**Addresses:** Diversity-weighted confirm count, split confidence/reliability decay.
**Avoids:** Pitfall 4 (Sybil gaming — explicit red-team test required), Pitfall 10 (geohash precision suppressing genuine consensus).

### Phase 4: Robustness — rate limiting, auto-moderation, legal disclaimer
**Rationale:** Public-facing abuse surfaces (anonymous posting, no signup) must be closed before any real promotion; auto-moderation and rate limiting both need calibration work that a fresh deploy can't skip.
**Delivers:** Session-first/IP-secondary rate limiting, text-only auto-moderation confidence-cascade with a human-reviewable queue and calibration fixture set, persistent legal disclaimer banner with a working grievance-report path.
**Addresses:** Text toxicity/spam filter (Active per FEATURES.md correction — image-safety check deferred to Stretch with photo attachment).
**Avoids:** Pitfall 3 (CGNAT collateral damage), Pitfall 6 (miscalibrated moderation), Pitfall 8 (legal/implied-authority exposure — LOW confidence, recommend a real legal check before wide promotion).

### Phase 5: Official feed + demo mode
**Rationale:** Solves the empty-map-on-first-visit problem that would otherwise make a zero-traffic portfolio demo look broken; GDACS integration is independent of the trust pipeline and can be built in parallel with Phase 3/4 work.
**Delivers:** GDACS poller (idempotent upsert, bypasses trust pipeline), authority badge for GDACS-sourced pins, demo/replay mode with a persistent, unmissable visual distinction between crowd/official/demo sources.
**Addresses:** Official open-data feed integration, authority badge (GDACS half), demo/replay mode.
**Avoids:** Pitfall 7 (demo data mistaken for real reports), the poller-silent-failure integration gotcha (needs a `last_successful_poll_at` health signal).

### Phase 6: Coordination features — triage, responder claim, shareable cards
**Rationale:** These build on Phase 1-3's data (severity, confirms, verification tier) and each carries its own abuse-model requirement that must be designed in, not retrofitted.
**Delivers:** "I'm safe" check-in, needs/offers matching, volunteer triage view ordered by corroboration (not raw self-declared severity), responder claim with mandatory expiry, human-contact verification tier, shareable cards with link-only state (no baked-in counts), anonymous reporter receipt code.
**Addresses:** The bulk of the remaining Active differentiator list.
**Avoids:** Pitfall 11 (unaccountable claim/contact overrides — expiry + non-hiding annotation required), Pitfall 12 (shareable card defeating its own retraction mechanism — resolve link-vs-image design before building the template).

### Phase 7 (Stretch, later milestone): PWA, photo attachment, live push
**Rationale:** Correctly deferred — depends on Web Push/websockets infra and introduces the image-safety/dedup surface only once photo attachment exists.
**Delivers:** IndexedDB offline queue (Background Sync as Chromium-only enhancement layered on top, never the baseline), Web Push/alert radius (gated behind "add to home screen" on iOS), photo attachment + image-safety check + reshare/duplicate-image flag, proactive retraction push (needs `report_views` table).

### Phase Ordering Rationale

- Data model and the VisibilityResolver come first because retrofitting `timestamptz`, the append-only vote log, or a scattered-visibility-logic anti-pattern is far more expensive than building them correctly from phase 1.
- Trust mechanic hardening is split into its own phase (3) rather than bundled with the base confirm/dispute build (2), because the geohash precision and Sybil-mitigation decisions are genuinely load-bearing design choices research flagged as needing dedicated reasoning, not incidental polish.
- Robustness (4) is sequenced before official-feed/demo (5) and coordination (6) because rate limiting and moderation are abuse-surface closures that should exist before the product looks "done enough" to attract real traffic or promotion.
- GDACS/demo mode (5) is architecturally independent of the trust pipeline (bypasses it entirely) and could run in parallel with phase 3/4 if executing with more than one workstream.
- PWA/photo/push (7) is correctly last — it's Stretch scope with real cross-browser (iOS Safari) limitations that don't block core launch.

### Research Flags

Needs research during planning:
- **Phase 3 (trust-model hardening):** geohash cell-size choice and Sybil-mitigation thresholds are numeric design decisions without empirical calibration data available pre-launch — needs a documented reasoning pass, flagged in PITFALLS.md as not independently benchmarked.
- **Phase 4 (auto-moderation):** confidence-cascade threshold calibration against a hand-built realistic-report fixture set; also the India IT Rules 2021 intermediary-liability framework is flagged LOW confidence in PITFALLS.md and needs an actual legal check before wide public promotion.
- **Phase 5 (GDACS/official feed):** IMD/CWC bulletin integration (beyond GDACS) is explicitly unverified in ARCHITECTURE.md — GDACS is the safer first integration; treat IMD/CWC as a follow-up spike.

Standard patterns (skip research-phase):
- **Phase 1 (foundation):** bounding-box + Haversine, `timestamptz`, and the Go project structure are well-documented, HIGH-confidence patterns already fully specified in ARCHITECTURE.md and STACK.md.
- **Phase 2 (trust mechanic core):** the VisibilityResolver pattern and independence predicate are directly specified with working code sketches in ARCHITECTURE.md.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | Versions verified directly against Go module proxy/npm registry (not search snippets); library choices (chi, pgx, sqlc) are well-established but "MEDIUM" reflects that broader ecosystem judgment calls, not just version numbers, underlie the recommendations |
| Features | HIGH | Sanity-checked against real analog products (Citizen, Waze, Ushahidi, PulsePoint, Facebook Safety Check, X Community Notes, Wikipedia) and documented real-world disaster case studies (Kerala 2018, Cyclone Fani, Boston Marathon study) |
| Architecture | HIGH (MEDIUM on 2 externally-verified specifics) | Component boundaries derived directly from the project's own stated requirements; GDACS event-ID stability and Render/Railway free-tier sleep behavior are the two externally-verified specifics, corroborated via web search |
| Pitfalls | MEDIUM-HIGH | Technical pitfalls verified against PostgreSQL docs, CGNAT/rate-limit industry writeups, and peer-reviewed crisis-informatics literature; feature-interaction pitfalls (9-12) derived directly from this project's own scope (HIGH confidence the interaction exists, MEDIUM on suggested numeric mitigations); India-specific legal pitfall explicitly flagged LOW confidence, not a substitute for legal advice |

**Overall confidence:** MEDIUM-HIGH

### Gaps to Address

- **Confirmer location-capture method** (geolocation prompt vs. IP-derived coarse geohash) is explicitly unresolved in FEATURES.md and materially changes what "diversity-weighted" measures — must be decided as a design decision in Phase 2/3, not deferred.
- **Geohash cell-size precision** has no independently benchmarked value — needs a documented, reasoned choice against realistic incident radius (~100-300m for a flooded road segment) in Phase 3.
- **India IT Rules 2021 legal exposure** is LOW confidence — get an actual legal/mentor review before any wide public promotion, not just before initial deploy.
- **IMD/CWC feed integration** (beyond GDACS) has no confirmed stable feed format yet — treat as a follow-up spike, not assumed available for Phase 5.
- **Auto-moderation calibration** has no real abuse-traffic to tune against pre-launch — the credible v1 deliverable is a documented, reasoned threshold with a human-review queue as the real safety net, not an empirically-calibrated number.

## Sources

### Primary (HIGH confidence)
- Go module proxy (`proxy.golang.org`) and npm registry — direct version verification for chi, pgx, sqlc, goose, migrate, swag, geohash, goimagehash, webpush-go, Leaflet
- go.dev official blog/docs — Go 1.22+ ServeMux routing changes
- PostgreSQL official docs — `earthdistance`/`cube` extension behavior and accuracy
- OpenAI official docs/announcements — Moderation API (`omni-moderation-latest`) and Embeddings API capabilities/pricing
- caniuse.com — Background Sync API browser support (zero Safari/iOS/Firefox support confirmed)
- GDACS official API quickstart docs (v1/v2) — `gdacs:eventid` stability, GeoJSON `geteventlist` endpoint
- PostgreSQL concurrency documentation — vote/hit-counter race-condition pattern

### Secondary (MEDIUM confidence)
- Render/Railway free-tier behavior (community discussions, independent write-ups) — 30-day Postgres expiry, ~15-min inactivity spin-down
- Cloudflare/SOAX engineering writeups — CGNAT and IP-based rate-limiting collateral damage
- ScienceDirect peer-reviewed article — Ushahidi crowdsourced crisis-mapping verification challenges
- iOS Safari Web Push requirements (Notificare, Pushpad, cross-checked across 3+ sources) — Home-Screen-install requirement, iOS 16.4+
- Bounding-box + Haversine query pattern (MySQL-authored sources, database-agnostic technique)

### Tertiary (LOW confidence)
- India IT Rules 2021 intermediary-liability application to a solo portfolio deployment (PRS India, Cyril Amarchand Mangaldas) — general framework is MEDIUM, specific applicability is LOW; needs independent legal check
- Feature-interaction pitfalls 9-12 (severity bypass, geohash precision, responder-claim abuse, shareable-card/retraction contradiction) — HIGH confidence the interaction exists (derived directly from the project's own stated scope), MEDIUM confidence on the specific numeric mitigations suggested (not independently benchmarked)

---
*Research completed: 2026-09-05*
*Ready for roadmap: yes*
