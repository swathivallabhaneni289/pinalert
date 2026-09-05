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

(None yet — ship to validate)

### Active

**Base reporting loop:**
- [ ] Location-tagged reports — pinned to a place (GPS/area), feed filtered by proximity
- [ ] Report categories — flooding, road blocked, power outage, shelter open, rescue needed
- [ ] Confirm/dispute voting on each report, surfaced as "confirmed by N nearby"
- [ ] Severity level (low/medium/critical) so urgent reports surface first
- [ ] Auto-expiry — reports fade/archive after N hours unless re-confirmed
- [ ] Map view — pins on a Leaflet/OpenStreetMap map for a quick visual scan

**Trust-model hardening (research-backed, protects Core Value directly):**
- [ ] Provisional visibility gate — new non-critical reports render dimmed until a second
      independent confirmation; critical/rescue-needed always publishes instantly, no gate
- [ ] Diversity-weighted confirm count — "confirmed by N" computed from distinct geohash
      cells/sessions, not raw vote count, so one location can't inflate its own tally
- [ ] Split confidence/reliability decay — separate fast-decaying "is this incident still real"
      score from slow-decaying "is this source trustworthy" score
- [ ] Retraction push + live-resolving status link on shareable cards — a report later disputed
      into hiding pushes a correction to everyone who saw it, and shared cards resolve live
      instead of freezing a stale confirmation count

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
      effort, and surface genuinely unclaimed active reports
- [ ] Human-contact verification tier — a volunteer who phoned the person can mark
      "contacted at HH:MM" with a field-verified priority that outranks passive voting

**Robustness:**
- [ ] Low-bandwidth / text-first fallback view (no map, no photos) for degraded networks
- [ ] Background Sync (service worker) — queue a report offline, send once connectivity returns
- [ ] Auto-moderation on submit — toxicity/spam text filter + image-safety check, run as a
      confidence-cascade calibrated to an explicit false-positive budget, not a flat cutoff
- [ ] Official open-data feed integration (GDACS / IMD / CWC) — background "official" pins so
      the map is never empty, and gives the authority badge real data to badge
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
- Resolved marking (reporter/nearby users close out a report) — deferred, auto-expiry covers the
  core "feed reflects what's true now" need for v1
- Authority badge for official sources (municipal corp, NDRF) — deferred until there's a real
  verified-authority-account flow to attach it to
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

## Constraints

- **Purpose**: Coding portfolio for job applications in India, not a startup — scope decisions
  favor demonstrable engineering signal (architecture, testing, documented trust-model
  reasoning) over business metrics or growth features.
- **Timeline**: Core build targeted at 2-4 weeks solo; trust-model hardening and stretch items
  extend beyond that as an ongoing project, not a hard deadline.
- **Team**: Solo developer.
- **Tech stack**: Go backend, PostgreSQL with plain lat/lon columns (no PostGIS — unnecessary at
  this scale), server-rendered HTML (`html/template`) + vanilla JS, Leaflet.js/OpenStreetMap for
  the map, deployed to Railway or Render. Chosen for a clean API-first architecture that's a good
  portfolio signal and doesn't need paid infrastructure.
- **Budget**: Free/cheap tiers only — no paid map API, no paid SMS/WhatsApp gateway in the core
  build.
- **Access model**: Anonymous posting with IP rate-limiting, no signup required — a deliberate
  choice, not a shortcut: requiring signup adds friction exactly when someone needs to report
  something fast during an emergency.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Verified local emergency feed over 4 other portfolio ideas | Confirm/dispute trust mechanic is a real differentiator, not just CRUD | — Pending |
| API-first Go backend, website as first client | Enables PWA/native app later with no backend rewrite | — Pending |
| Anonymous posting, no signup, IP rate-limited | Signup friction is unacceptable for emergency reporting | — Pending |
| Plain lat/lon + Go/SQL distance calc instead of PostGIS | Sufficient accuracy at this scale, avoids infra overkill | — Pending |
| Demo/replay mode + official open-data feed (GDACS) promoted to core, not stretch | Solves the empty-map-on-first-visit problem that would otherwise kill the demo for reviewers | — Pending |
| Trust-model hardening (provisional gating, diversity-weighted confirms, confidence/reliability split, retraction propagation) promoted to core | These are documented fixes for real failure modes (single-source rumors, vote-stuffing, stale-info spreading via shared cards), not speculative extras — the trust mechanic is the Core Value, so it needs to hold up | — Pending |
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
*Last updated: 2026-09-05 after initialization*
