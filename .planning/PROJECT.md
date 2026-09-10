# Pinalert

## What This Is

A local, verified information feed for emergencies like floods or cyclones. People near an
affected area post short, location-tagged updates (a flooded road, a shelter that's open, a
power outage, someone needing rescue); other people nearby confirm or dispute each report, so a
report showing "confirmed by 12 people nearby" can be trusted while an unconfirmed one gets
flagged. Reports auto-fade unless re-confirmed, so the feed always reflects what's true right
now — a live, trustworthy alternative to unverified WhatsApp forwards during a disaster.

## Core Value

The confirm/dispute trust mechanic — the ability to tell a real-time reader "this specific
report is currently backed by N independent nearby confirmations" — must work correctly and be
resistant to trivial gaming. Everything else in the product exists to feed data into or surface
output from that mechanic.

## Requirements

### Validated

**Shipped in Phase 1 (Foundation — Report & Map, completed 2026-09-10):**
- ✓ Anonymous session identity — HMAC-signed cookie, issued on first visit, no signup — Phase 1
  (**access model superseded 2026-09-10** — see Phase 1.1 in Active and the mandatory-login Key
  Decision below; the cookie mechanism itself is retained, just now gated behind a verified
  account rather than usable anonymously)
- ✓ Location-tagged reports — indexed bounding-box prefilter + Haversine, confirmed index scan
  (not a full-table scan) via `TestNearbyReportsUsesIndex` against real Postgres — Phase 1
- ✓ Report categories — all 9 categories (flood, earthquake, fire, storm/cyclone damage, road
  blocked, power outage, shelter open, rescue needed, other) shipped with committed glyphs — Phase 1
- ✓ Severity level (low/medium/critical) — accessible slider, drives expiry tier and future triage
  ordering — Phase 1
- ✓ Auto-expiry — read-time predicate (`expires_at < now()`) confirmed live on every read via
  `TestExpiryReadTimePredicate`, no background-sweep dependency — Phase 1
- ✓ Map view — Leaflet with MapLibre GL vector basemap (OpenFreeMap liberty style), WebGL-gated
  fallback to OpenStreetMap raster tiles for clients without WebGL2 — Phase 1 (upgraded from the
  original plain-Leaflet/OSM plan mid-phase; see Key Decisions)
- ✓ OpenAPI/Swagger docs for the JSON API — browsable spec at a stable URL with a drift guard
  against handler behavior — Phase 1
- ✓ GitHub Actions CI (`go test`, `go vet`, build) — confirmed green against final shipped code — Phase 1

### Active

**Identity & Login (Phase 1.1 — inserted 2026-09-10, must land before Phase 2):**
- [ ] Mandatory email + one-time-passcode (OTP) verification before a visitor can submit a report
      or cast a confirm/dispute vote — replaces the anonymous no-signup model shipped in Phase 1
- [ ] Free-tier transactional email provider (e.g. Resend) for OTP delivery — no paid SMS/phone
      verification path; phone OTP has no free tier at any real volume and the cheap route
      additionally requires India DLT sender registration (see Key Decisions)
- [ ] User profile page — a verified user's own submitted reports and voting/confirm-dispute
      activity history
- [ ] OTP request rate limiting, per email address and per IP

**Base reporting loop (remaining):**
- [ ] Confirm/dispute voting on each report, surfaced as "confirmed by N nearby" — append-only
      vote log is the source of truth, with an atomic cache update in the same transaction as
      each vote (never read-then-write), and a concurrency test as a completion criterion, not a
      later hardening pass. Votes are now cast by verified accounts (Phase 1.1), not anonymous
      sessions — the independence predicate below still checks distinct session + distinct
      geohash cell, but "session" now implies "verified account."
- [ ] Resolved marking — reporter or nearby users close out a report once it's no longer true
      (promoted from Stretch: cheap, complements auto-expiry, directly serves Core Value)

**Trust-model hardening (research-backed, protects Core Value directly):**
- [ ] A single `VisibilityResolver` function is the sole authority for report visibility
      (Hidden/Provisional/Live/Retracted), called by every read path — feed, map, triage,
      shareable cards. This is the architectural anchor of the whole trust model: every pitfall
      research found traces back to visibility logic drifting out of sync across paths.
- [ ] Independence predicate on votes (distinct session + distinct geohash cell) — must exist
      before any diversity-weighting curve; a gate built on raw vote counts is trivially beaten
      by two votes from one device, which is worse than no gate at all
- [ ] Provisional visibility gate — new non-critical reports render dimmed until a second
      independent confirmation; critical/rescue-needed always publishes instantly, no gate
- [ ] Diversity-weighted confirm count — "confirmed by N" computed from distinct geohash
      cells/sessions, not raw vote count, so one location can't inflate its own tally. How the
      confirmer's own location is captured (GPS prompt vs. IP-derived coarse geohash) is an open
      design decision — see Key Decisions.
- [ ] Split confidence/reliability decay — separate fast-decaying "is this incident still real"
      score from slow-decaying "is this source trustworthy" score
- [ ] Live-resolving status link on shareable cards — the card carries only a status URL, never a
      baked-in confirmation count or unqualified "verified" claim in the image itself, so a
      report later disputed into hiding shows "since retracted" instead of a frozen stale count
      in a forwarded screenshot (a static baked-in count would defeat the point of this feature)

**Closing the information loop:**
- [ ] "I'm safe" check-in — anyone can post/search a safety check-in for themselves or an area
- [ ] Needs ↔ offers matching — need (water/medicine/transport) and offer reports, filterable
      together
- [ ] Volunteer/rescue-team triage view — filtered to severity=critical + rescue-needed
- [ ] Shareable verified-report cards — a summary card resharable back into WhatsApp groups

**Data quality / real-world coordination:**
- [ ] Structured shelter capacity status (Available/Limited/Full/Closed + optional headcount)
- [ ] Anonymous reporter receipt code — check a submitted report's status without re-posting
- [ ] Responder claim + coverage-gap layer — mark a report "responding" so teams don't duplicate
      effort, and surface genuinely unclaimed active reports. Must auto-expire and never silently
      remove a report from the coverage-gap view — an unaccountable override on exactly the
      reports where being wrong matters most is a documented abuse vector.
- [ ] Human-contact verification tier — a volunteer who phoned the person can mark
      "contacted at HH:MM" with a field-verified priority that outranks passive voting, subject to
      the same expiry/reversibility requirement as the responder claim above
- [ ] Authority badge for GDACS-sourced official pins — cheap, ships alongside the official feed
      integration below (distinct from a verified-authority-*account* flow for municipal/NDRF
      accounts, which stays deferred — see Out of Scope)

**Robustness:**
- [ ] Low-bandwidth / text-first fallback view (no map, no photos) for degraded networks
- [ ] Offline report queue — IndexedDB + retry-on-`online`-event is the baseline (the Background
      Sync API has zero Safari/iOS/Firefox support as of 2026, so it can only ever be a
      Chromium-only progressive enhancement layered on top, never the mechanism itself)
- [ ] Auto-moderation on submit — text toxicity/spam filter only for v1, run as a
      confidence-cascade calibrated to an explicit false-positive budget, not a flat cutoff
      (image-safety check moves to Stretch, paired with photo attachment which doesn't exist yet)
- [ ] Session-first, IP-secondary rate limiting — pure IP-based limiting is counterproductive in
      India specifically: carrier-grade NAT means hundreds of real phones share one public IP, so
      a tight per-IP limit throttles legitimate nearby reporters during the exact surge the
      product exists for. Rate-limit per anonymous session first, with IP as a looser backstop.
- [ ] Official open-data feed integration (GDACS) — idempotent upsert on `(source, external_id)`,
      bypassing the trust/vote pipeline entirely (crowd disputes must not be able to hide an
      official pin) — background "official" pins so the map is never empty. IMD/CWC bulletins are
      a separate follow-up spike (feed format unverified), not assumed available alongside GDACS.
- [ ] Demo/replay mode — seeded past-event timeline + "simulate a report" button, so a first-time
      visitor sees the product working immediately
- [ ] Explicit disclaimer — not affiliated with any government agency, not a substitute for
      calling emergency services
- [ ] OpenAPI/Swagger docs for the JSON API
- [ ] GitHub Actions CI (`go test`, `go vet`, build)

### Out of Scope

**Stretch (build if core is solid and time remains):**
- Photo attachment on a report — deferred, core loop works without it
- Live updates without refresh (websockets) — deferred, polling is acceptable for v1
- Image-safety check on photo uploads — deferred alongside photo attachment itself; nothing to
  check against until photos exist
- Verified-authority-*account* flow (municipal corp, NDRF posting as a badged account) — deferred;
  the badge for GDACS-sourced official pins is Active, this is the separate account/login flow
- Proactive retraction push notification (vs. the passive live-resolving status link, which is
  Active) — deferred, needs Web Push/websockets infra plus a new `report_views` table
- Alert radius via Web Push (opt-in notification near a saved location) — deferred, needs
  service-worker/VAPID setup beyond the core PWA shell
- Structured water-depth field + graduated flood-extent overlay — deferred, valuable but a
  moderate lift beyond the discrete-pin map view
- Duplicate-report clustering via text embeddings — deferred, needs an external embeddings call
- Reshare/duplicate-image flag (perceptual hashing) — deferred, only matters once photo
  attachment ships
- Self-declared eyewitness tag — deferred, nice-to-have signal, not core to the trust mechanic
- Severity/category auto-suggestion from free text — deferred, manual selection is fine for v1
- Rater trust score for confirmers (separate from reporter reputation) — deferred, diversity
  weighting covers the most urgent gaming vector first
- Trusted-verifier override for designated moderators — deferred until there's a moderator role
- Anti-brigading burst detection — deferred, real but significant effort; diversity-weighting
  and provisional-gating cover the cheaper attacks first
- Capability-matched dispatch alerts (boat/medical/generator tags) — deferred, needs the alert
  radius/push feature first
- Targeted confirm-nudge to already-engaged sessions — deferred, needs push infra first
- Crowd-translation queue for report free text — deferred past v1
- Multi-language UI toggle (Hindi + one regional language) — deferred past v1
- Rapid swipe-triage queue — deferred, the filtered triage list view covers v1 needs
- CAP-standard action/certainty labels — deferred, real interoperability value but not needed
  until the core trust mechanic is proven
- Authority handoff export (packaged report / area digest for officials) — deferred past v1
- Per-recipient response tracking with auto-escalation on alerts — deferred, needs push infra
- WhatsApp/SMS reporting bot — deferred; the highest real-world-reach feature, but its own
  integration project (Twilio/WhatsApp Cloud API), not a core-build add
- Route-around-hazard hints on the map — deferred, needs routing logic out of scope for v1

**V2 / future (real value, deliberately not building now):**
- Missing/found person registry — highest abuse-surface data type (third-party PII) on an
  anonymous no-signup platform; needs a moderation-gated design before it's safe to ship
- Per-incident real-time coordination channel (chat) for responders — materially more work than
  the rest of the list; revisit only if the core product gets real usage
- Full CAP-compliant alert feed with Update/Cancel lifecycle threading — real interoperability
  value is speculative without an actual government integration path
- Digitally-signed authority reports (XML-Signature on CAP alerts) — only meaningful once the
  CAP feed exists and there are real authority accounts to hold keys

## Context

Picked from a shortlist of 5 portfolio project ideas (blood-donor network, industrial safety
tracker, this local-info feed, a shared-fund tracker, a school early-warning tracker) — this one
won on having a genuine differentiator (crowd trust verification) rather than being CRUD-over-a-
database.

Extensive feature research was done before this build (full findings kept in
`PROJECT-NOTES.md` at the repo root):
- **Competitor/analog apps:** Citizen, Waze, Ushahidi, PulsePoint, Google Crisis Response,
  Facebook Safety Check
- **Academic crisis-informatics research:** disaster social-media studies (Hurricane Sandy,
  Haiti 2010, Chennai 2015 flood-inundation mapping)
- **Real India case studies:** Kerala floods 2018 (including the actual keralarescue.in
  volunteer codebase and a published study of ~45,000 logged rescue requests), Chennai floods
  2015, Cyclone Fani 2019 Odisha evacuation, Assam floods (Banpani.org, 2026)
- **Trust-mechanism analogs:** X/Twitter Community Notes, Waze's confidence/reliability scoring,
  Wikipedia Pending Changes/ClueBot NG
- **Alerting standards:** OASIS Common Alerting Protocol (CAP) 1.2, used by IPAWS/NWS/Google
  Public Alerts

That research is the source for most of the "trust-model hardening" and "data quality" Active
requirements above — they're not speculative additions, they're documented fixes for failure
modes real deployments hit.

A second, domain-ecosystem research pass (stack/features/architecture/pitfalls, full findings in
`.planning/research/`) validated the stack and surfaced further corrections now folded into the
Active/Out-of-Scope lists above — notably the `VisibilityResolver` architectural pattern, the
independence-predicate-before-diversity-weighting sequencing, and the CGNAT rate-limiting issue.
It also found that "free" Postgres hosting on Railway/Render isn't actually persistent (Render's
free tier expires after 30 days; Railway dropped its indefinite free tier) — resolved as a Key
Decision below.

## Constraints

- **Purpose**: Coding portfolio for job applications in India, not a startup — scope decisions
  favor demonstrable engineering signal (architecture, testing, documented trust-model
  reasoning) over business metrics or growth features.
- **Timeline**: Core build targeted at 2-4 weeks solo; trust-model hardening and stretch items
  extend beyond that as an ongoing project, not a hard deadline.
- **Team**: Solo developer.
- **Tech stack**: Go 1.25+, PostgreSQL 16/17 with plain lat/lon columns (no PostGIS — unnecessary
  at this scale), `go-chi/chi/v5` for routing (not `gorilla/mux` — unmaintained since Dec 2022),
  `jackc/pgx/v5` + `sqlc` for type-safe SQL, `mmcloughlin/geohash` for diversity-weighting cell
  computation, server-rendered HTML (`html/template`) + vanilla JS, Leaflet.js as the map framework
  with MapLibre GL rendering OpenFreeMap vector tiles as the basemap (upgraded mid-Phase-1 from the
  original plain-OpenStreetMap-raster plan; OSM raster retained as the WebGL-less fallback).
  Chosen for a clean API-first architecture that's a good portfolio signal and doesn't
  need paid infrastructure.
- **Budget**: Free tiers only for the core build — no paid map API, no paid SMS/WhatsApp gateway
  (confirmed 2026-09-10: no phone-OTP provider — Firebase, MSG91, Twilio Verify, AWS SNS — has a
  free tier at any real send volume; the cheapest real option, AWS SNS at ~$0.003/SMS, additionally
  requires India DLT sender registration, a business-entity paperwork requirement, not just an API
  key — this is why login uses email OTP only, via a free-tier transactional email provider like
  Resend, not phone).
  Database hosting uses Neon or Supabase (genuinely persistent free Postgres, unlike Railway/
  Render's now-limited free tiers) rather than a paid plan. Auto-moderation uses OpenAI's
  Moderation API (`omni-moderation-latest`, free) rather than Google's Perspective API (shutting
  down Dec 31, 2026).
- **Access model** (revised 2026-09-10 — see Key Decisions): Mandatory email + OTP verification
  before a visitor can submit a report or vote — reverses Phase 1's original anonymous/no-signup
  model. Decided during Phase 2 discussion after the user raised a real vote-stuffing concern
  (multiple devices/sessions from one person); weighed against the independence predicate
  (distinct session + distinct geohash cell) already planned for Phase 2/3, but the user judged
  the accountability gap worth closing directly rather than relying on location-diversity alone.
  Session-first, IP-secondary rate limiting (originally reconsidered after research flagged CGNAT
  — shared carrier IPs — as a real India-specific collateral-damage risk) still applies underneath
  the login gate.
- **Legal**: This is a public, unofficial emergency-information app — India's IT Rules 2021
  intermediary-safe-harbor conditions apply the moment it's public, regardless of real traffic.
  Research confidence on specific applicability is LOW; get an actual legal/mentor review before
  any wide public promotion (not required before initial deploy, which is a portfolio artifact,
  not a promoted public service).

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Verified local emergency feed over 4 other portfolio ideas | Confirm/dispute trust mechanic is a real differentiator, not just CRUD | — Pending |
| API-first Go backend, website as first client | Enables PWA/native app later with no backend rewrite | ✓ Good — Phase 1 shipped a browsable OpenAPI/Swagger spec against the same handlers the web client consumes |
| Anonymous posting, no signup, session-first/IP-secondary rate limiting | Signup friction is unacceptable for emergency reporting; pure IP limiting was reconsidered after research flagged CGNAT collateral damage in India | Shipped and verified in Phase 1 as originally specified — **then reversed 2026-09-10** (see the mandatory-login decision below); session-first/IP-secondary rate limiting itself is Phase 4 (ROBUST) scope, not yet built, and now applies underneath the login gate rather than to anonymous sessions |
| Mandatory email + OTP login before reporting or voting, phone OTP explicitly rejected | User raised a real concern during Phase 2 discussion: fully anonymous voting is gameable via multiple devices/sessions from one person. Weighed against the independence predicate (distinct session + distinct geohash cell) already planned for Phase 2/3 — that predicate stops multi-device stuffing FROM ONE LOCATION, but not a person deliberately spoofing/visiting multiple locations. User judged the accountability gap worth closing directly. Researched current OTP pricing before deciding: no phone-OTP provider (Firebase, MSG91, Twilio Verify, AWS SNS) has a free tier at real volume, and the cheapest real option (AWS SNS, ~$0.003/SMS) requires India DLT sender registration (a business-entity paperwork requirement) to get that rate — conflicts with this project's free-tier-only budget and solo-developer/portfolio context. Email OTP (via Resend's free tier: 3,000/month, 100/day) closes the same accountability gap at zero cost. | Decided 2026-09-10, inserted as new Phase 1.1 (before Phase 2) — Pending implementation. Reverses Phase 1's already-shipped FOUND-01 (see REQUIREMENTS.md) |
| Plain lat/lon + Go/SQL distance calc instead of PostGIS | Sufficient accuracy at this scale, avoids infra overkill | ✓ Good — Phase 1's bounding-box + Haversine query confirmed using an index scan (`TestNearbyReportsUsesIndex`), not a full-table scan, against real Postgres |
| Demo/replay mode + official open-data feed (GDACS) promoted to core, not stretch | Solves the empty-map-on-first-visit problem that would otherwise kill the demo for reviewers | — Pending |
| Trust-model hardening (provisional gating, diversity-weighted confirms, confidence/reliability split, retraction propagation) promoted to core | These are documented fixes for real failure modes (single-source rumors, vote-stuffing, stale-info spreading via shared cards), not speculative extras — the trust mechanic is the Core Value, so it needs to hold up | — Pending |
| Single `VisibilityResolver` function as sole authority for report visibility, called by every read path | Architecture research found every documented trust-model pitfall traces back to visibility logic drifting out of sync across feed/map/triage/share-card paths | — Pending |
| Independence predicate (distinct session + geohash cell) built before diversity-weighting curve | A weighting scheme built on raw vote counts is trivially beaten by multiple votes from one device — the predicate must exist first, not as a later refinement | — Pending |
| DB hosting on Neon or Supabase instead of Railway/Render free tiers | Research found Render's free Postgres expires after 30 days and Railway dropped its indefinite free tier — neither is "free and persistent," which a portfolio project needs; Neon/Supabase are | — Pending |
| `go-chi/chi/v5` + `pgx/v5` + `sqlc`, `mmcloughlin/geohash`, OpenAI Moderation API | Chi replaces the originally-considered gorilla/mux (unmaintained since Dec 2022); pgx+sqlc gives type-safe SQL without an ORM fighting the custom geo/trust queries; OpenAI's Moderation API is free and covers text+image in one call, unlike Google's Perspective API (shutting down) | chi/pgx/sqlc validated in Phase 1 (routing, store layer, migrations); `mmcloughlin/geohash` is an added dependency with no diversity-weighting usage yet (Phase 3 scope); OpenAI Moderation API not yet integrated (Phase 4 scope) — Partially Pending |
| Vector basemap migration: Leaflet + MapLibre GL rendering OpenFreeMap tiles, OSM raster as WebGL-less fallback (`01-11-RESEARCH.md`) | User-chosen mid-Phase-1 quality upgrade over the original plain-Leaflet/OSM-raster plan; sharper rendering with no paid API key, `@maplibre/maplibre-gl-leaflet` bridge keeps all existing Leaflet marker/popup code unchanged | ✓ Good — Phase 1, capability-probed WebGL2 fallback verified working |
| Confirmer location-capture method for diversity weighting (GPS prompt vs. IP-derived geohash) | Materially different UX/signal-quality tradeoff — flagged by research as needing an explicit decision, not left implicit | ⚠️ Unresolved — decide in Phase 2/3 planning |
| Repo public on GitHub (`swathivallabhaneni289/pinalert`) | Portfolio project — needs to be visible to recruiters/interviewers | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-09-10 — mandatory email+OTP login decision (Phase 1.1 inserted before Phase 2)*
