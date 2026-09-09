# Roadmap: Pinalert

## Overview

Pinalert ships as six vertical slices, each one something a real visitor can try end-to-end.
Phase 1 gets the base reporting loop live (submit, view, expire) with the anonymous session
identity, API docs, and CI in place from day one, since retrofitting the schema/session model
later is expensive. Phase 2 builds the Core Value itself — confirm/dispute voting resolved by a
single, concurrency-safe `VisibilityResolver` — as its own complete slice, gated by an
independence predicate before any weighting refinement is layered on. Phase 3 hardens that same
trust display against trivial gaming (diversity-weighted counts, split confidence/reliability
signals) as a dedicated slice, because the geohash-precision and confirmer-location-capture
decisions it resolves are genuinely load-bearing, not incidental polish. Phase 4 makes the app
survive real conditions — degraded networks, abuse pressure, legal exposure. Phase 5 solves the
empty-map-on-first-visit problem with official GDACS pins and a working demo. Phase 6 closes the
information loop with the coordination features (safety check-ins, needs/offers, triage,
responder claims, shareable cards) that depend on the trust signals built in Phases 2-3.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation — Report & Map** - Anonymous visitors can submit and view location-tagged reports on a live map, with read-time auto-expiry, OpenAPI docs, and CI in place. (original 10 plans completed 2026-09-06; reopened for the vector basemap migration, plans 01-11/01-12; gap-closure round 2 (01-13) fixed the solid-black-icon defect; gap-closure round 3 (01-14) landed 2026-09-09 — raised glyph ink coverage and fixed badge contrast across all 18 severity/age/theme pairings; gap-closure round 4 (01-15) landed 2026-09-09 — live-tested fix for dark-mode category-tile glyph color, the actual root cause 01-14's stroke-width diagnosis missed, plus two caching-bug fixes; see `01-UAT.md`) (all 15 plans complete; UAT passed 10/10 with 0 open issues; canonical verification/security/Nyquist gates pending re-run)
- [ ] **Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility** - Users can confirm/dispute reports through one shared, concurrency-safe visibility resolver with a provisional gate and resolved marking.
- [ ] **Phase 3: Trust-Model Hardening — Diversity-Weighted Trust** - "Confirmed by N nearby" and the reliability/currency signals reflect distinct nearby corroboration, resistant to trivial gaming.
- [ ] **Phase 4: Robustness — Real-World Resilience** - The app stays usable on degraded networks, under abuse/moderation pressure, and with clear legal footing.
- [ ] **Phase 5: Official Feed & Demo Mode** - First-time visitors see a populated map with official GDACS pins and a working demo immediately, instead of an empty product.
- [ ] **Phase 6: Coordination — Safety, Needs, and Response** - People can check in safe, exchange needs/offers, and responders can triage, claim, and verify reports without duplicating effort.

## Phase Details

### Phase 1: Foundation — Report & Map

**Goal**: A visitor can submit a location-tagged emergency report and see it alongside other nearby reports on a live map, without creating an account.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05, FOUND-06, OPS-01, OPS-02
**Success Criteria** (what must be TRUE):

  1. A first-time visitor is automatically issued an anonymous session (cookie/localStorage, no signup) and can immediately submit a report.
  2. A visitor can submit a report with location, category (flood/earthquake/fire/storm-cyclone damage/road blocked/power outage/shelter open/rescue needed/other — 9 categories), severity, and description; a shelter-open report additionally records a capacity status (Available/Limited/Full/Closed) with an optional headcount.
  3. A visitor can view a feed of reports filtered to those near their current location (bounding-box + Haversine, not a full-table scan) and see the same reports as pins on a Leaflet/OpenStreetMap map.
  4. A report stops appearing in the feed once its expiry time passes, checked live on every read — not solely dependent on a background sweep job.
  5. The JSON API is documented via a browsable OpenAPI/Swagger spec at a stable URL, and every push runs automated tests, `go vet`, and a build check via CI.

**Plans**: 15/15 plans complete
Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Repo scaffold, schema migration, test-DB infrastructure, GitHub Actions CI (wave 1)
- [x] 01-02-PLAN.md — Design system: complete token layer, dark mode, 9 committed category glyphs (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-03-PLAN.md — Server slice: anonymous HMAC session, submit, nearby feed, proven end to end (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-04-PLAN.md — Browser slice: page shell, shared client store, live Leaflet map, minimal submit — walking skeleton closes (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-05-PLAN.md — Full submission modal: 3×3 category grid, accessible severity slider, shelter capacity, validation (wave 4)
- [x] 01-06-PLAN.md — Feed list: severity-first ordering, age-ramp desaturation, map/list sync, view toggle, states (wave 4)
- [x] 01-07-PLAN.md — OpenAPI/Swagger docs at a stable URL with a drift guard (wave 4)

**Gap closure** *(from `01-UAT.md`; run via `/gsd-execute-phase 01 --gaps-only`)*

- [x] 01-08-PLAN.md — Guard the modal backdrop's `[hidden]` display rule (UAT Test 1 blocker) and restyle the severity slider track/thumb (wave 1)
- [x] 01-09-PLAN.md — Give the primary map container a self-resolving viewport height so Leaflet has an area to paint tiles into (UAT Test 1 retest blocker), locked by a CSS contract test (wave 1)
- [x] 01-10-PLAN.md — Enable retina tile fetching on both Leaflet tile layers so map labels are sharp on HiDPI displays (UAT Test 1 cosmetic follow-up), locked by a JS contract test (wave 1)

**Vector basemap migration** *(deliberate quality upgrade chosen by the user, researched in `01-11-RESEARCH.md`; NOT gap closure — run via `/gsd-execute-phase 01`)*

- [x] 01-11-PLAN.md — Load the MapLibre GL renderer and the Leaflet bridge as SRI-pinned vendor tags in dependency order, locked by a template contract test, and update the stack documentation (wave 1)
- [x] 01-12-PLAN.md — Swap both Leaflet basemaps to the OpenFreeMap liberty vector style via the bridge, with a WebGL-gated fallback to the existing OpenStreetMap raster layer, locked by a rewritten JS contract test (wave 2)

**Gap closure (round 2)** *(from `01-UAT.md` Tests 2 and 6; run via `/gsd-execute-phase 01 --gaps-only`)*

- [x] 01-13-PLAN.md — Recolour the nine category glyphs through a CSS mask so page CSS reaches them, fixing solid-black icons in dark mode across map pins, feed rows and the modal grid, locked by two contract tests (wave 1)

**Gap closure (round 3)** *(from `01-UAT.md`'s two `status: diagnosed` gaps on Tests 2 and 6, diagnosed in `.planning/debug/category-glyph-legibility.md`; run via `/gsd-execute-phase 01 --gaps-only`)*

- [x] 01-14-PLAN.md — Give the nine glyphs enough ink to be identifiable at the 17.6px feed-row size and give every badge glyph a 3:1 foreground across all 18 severity/age/theme pairings, locked by two computed gates (stroke width per render context, WCAG contrast matrix); folds in 01-REVIEW.md WR-01 (wave 1)

**Gap closure (round 4)** *(live-tested fixes from a real-browser UAT walkthrough on 2026-09-09; 01-14's stroke-width diagnosis proved insufficient in practice — the actual fix was category-tile glyph color, not thickness; documented retroactively per the project's process-deviation convention, see `01-15-PLAN.md`)*

- [x] 01-15-PLAN.md — Fix category-tile glyph color in dark mode (`.category-tile .icon-glyph` / `--selected` counter-rule), revert the unrequested map/feed stroke-width thickening, reduce oversized badge glyphs (55%→48%), and fix two independent caching bugs (stale server processes, unversioned static asset URLs) via an `AssetVersion` cache-busting mechanism and dev-mode `Cache-Control: no-store` (wave 1)

**UI hint**: yes

### Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility

**Goal**: A user can confirm or dispute a report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed by one shared, concurrency-safe resolver everywhere it's shown.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: TRUST-01, TRUST-02, TRUST-03, TRUST-04, TRUST-06, TRUST-08, TRUST-09
**Success Criteria** (what must be TRUE):

  1. A user can confirm or dispute another user's report, and concurrent votes submitted on the same report at the same time never silently lose an update, verified by an automated concurrency test.
  2. A newly submitted non-critical report displays as "provisional" until a second independent confirmation arrives; a critical/rescue-needed report publishes at full visibility immediately, with no gate.
  3. Only a vote from a distinct anonymous session AND a distinct geohash cell counts toward that independent confirmation — a report's self-declared severity affects triage sort order only and can never by itself unlock full visibility.
  4. A report's visibility state (Hidden/Provisional/Live/Retracted) is identical everywhere it's shown — feed, map, triage view, and shareable card — because one shared resolver function computes it.
  5. A user (the reporter or a nearby confirmer) can mark a report resolved, removing it from the live feed.

**Plans**: TBD
**UI hint**: yes

### Phase 3: Trust-Model Hardening — Diversity-Weighted Trust

**Goal**: The "confirmed by N nearby" number and trust signals shown to users are resistant to trivial gaming — a real reflection of distinct nearby corroboration, not raw vote count.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: TRUST-05, TRUST-07
**Success Criteria** (what must be TRUE):

  1. The feed displays "confirmed by N nearby" computed from the count of distinct confirming geohash cells/sessions — using a deliberately chosen, documented geohash precision — not the raw number of votes cast.
  2. A report displays two distinct trust signals side by side: a fast-decaying "is this still current" confidence score and a slow-decaying "is this source reliable" score.

**Plans**: TBD
**UI hint**: yes

### Phase 4: Robustness — Real-World Resilience

**Goal**: The reporting loop keeps working for real users on shaky networks, under abuse pressure, and with clear legal footing.
**Mode:** mvp
**Depends on**: Phase 1, Phase 2
**Requirements**: ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-08
**Success Criteria** (what must be TRUE):

  1. A user on a degraded connection can switch to a low-bandwidth, text-only feed view with no map or photos.
  2. A report submitted while offline is queued on-device (IndexedDB) and automatically sent once connectivity returns.
  3. A submitted report's text is automatically screened for toxicity/spam before publishing; critical/rescue-needed reports stay visible while the check is pending, other categories stay provisional until it clears.
  4. Rapid repeated submissions from the same anonymous session are throttled before a shared IP is penalized, so many legitimate phones behind one carrier-grade NAT IP aren't collectively blocked.
  5. Every page displays a persistent disclaimer that Pinalert is unofficial, not affiliated with any government agency, and not a substitute for calling emergency services.

**Plans**: TBD
**UI hint**: yes

### Phase 5: Official Feed & Demo Mode

**Goal**: A first-time visitor lands on a populated, working map immediately — real official hazard pins plus a working demo — instead of an empty product.
**Mode:** mvp
**Depends on**: Phase 1, Phase 2
**Requirements**: ROBUST-05, ROBUST-06, ROBUST-07
**Success Criteria** (what must be TRUE):

  1. The map view shows official pins ingested from the GDACS open-data feed alongside crowd reports, and those official pins are unaffected by crowd confirm/dispute votes.
  2. GDACS-sourced pins carry a visibly distinct authority badge, so they're never confused with crowd reports or demo-mode reports.
  3. A first-time visitor sees a seeded past-event timeline populating the feed and map immediately, and can press "simulate a report" to try the confirm/dispute flow themselves.

**Plans**: TBD
**UI hint**: yes

### Phase 6: Coordination — Safety, Needs, and Response

**Goal**: Beyond raw reporting, people can find help, offer help, and responders can act on the most urgent, best-corroborated reports without duplicating effort or losing accountability.
**Mode:** mvp
**Depends on**: Phase 1, Phase 2, Phase 3
**Requirements**: COORD-01, COORD-02, COORD-03, COORD-04, COORD-05, COORD-06, COORD-07, COORD-08
**Success Criteria** (what must be TRUE):

  1. Anyone can post an "I'm safe" check-in tagged to a location, and search check-ins by name or area.
  2. A user can post a "need" (water/medicine/transport) or an "offer" report and view needs and offers displayed near each other.
  3. A volunteer/rescue team can view a triage list filtered to severity=critical or category=rescue-needed reports, ordered using the diversity-weighted trust signals from Phase 3, not raw self-declared severity alone.
  4. A responder can mark an active report "responding" with an optional ETA that auto-expires and never permanently hides the report from the coverage-gap view; a volunteer who phoned the reporter can mark it "contacted at HH:MM," which outranks passive vote count in the triage view.
  5. A reporter can check their own report's current status via a receipt code without re-posting, and anyone can generate a shareable card for a report containing only a live-resolving status link — never a baked-in confirmation count or "verified" claim in the image itself.

**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation — Report & Map | 14/14 | Awaiting UAT | - |
| 2. Trust Mechanic Core — Confirm/Dispute & Visibility | 0/TBD | Not started | - |
| 3. Trust-Model Hardening — Diversity-Weighted Trust | 0/TBD | Not started | - |
| 4. Robustness — Real-World Resilience | 0/TBD | Not started | - |
| 5. Official Feed & Demo Mode | 0/TBD | Not started | - |
| 6. Coordination — Safety, Needs, and Response | 0/TBD | Not started | - |
