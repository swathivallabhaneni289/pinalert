# Requirements: Pinalert

**Defined:** 2026-09-05
**Core Value:** A report showing "confirmed by N nearby" must be verifiably backed by N
independent nearby confirmations, resistant to trivial gaming.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Foundation

- [ ] **FOUND-01**: An anonymous session identity is issued to a first-time visitor (cookie/
      localStorage), with no signup required
- [ ] **FOUND-02**: User can submit a location-tagged report (GPS or manual area) with a category
      (flooding/road blocked/power outage/shelter open/rescue needed), severity
      (low/medium/critical), and free-text description
- [ ] **FOUND-03**: User can view a feed of reports filtered to those near their current location,
      served via an indexed bounding-box prefilter + Haversine query (not a full-table scan)
- [ ] **FOUND-04**: User can view reports as pins on a Leaflet/OpenStreetMap map
- [ ] **FOUND-05**: A report stops appearing in the live feed once its expiry time passes, checked
      as a read-time predicate on every query (not solely dependent on a background sweep job)
- [ ] **FOUND-06**: A "shelter open" report can carry a capacity status
      (Available/Limited/Full/Closed) with an optional headcount

### Trust Engine

- [ ] **TRUST-01**: User can confirm or dispute another user's report
- [ ] **TRUST-02**: A report's visibility state (Hidden/Provisional/Live/Retracted) is computed by
      one shared resolver function and is identical everywhere it's shown — feed, map, triage
      view, and shareable cards
- [ ] **TRUST-03**: A vote counts toward the independence threshold only if it comes from a
      distinct anonymous session AND a distinct geohash cell from every other counted vote on
      that report
- [ ] **TRUST-04**: A newly submitted non-critical report displays as "provisional" until a
      second independent confirmation arrives; critical/rescue-needed reports publish at full
      visibility immediately, with no gate
- [ ] **TRUST-05**: The feed displays "confirmed by N nearby" computed from the count of distinct
      confirming geohash cells/sessions, not the raw number of votes
- [ ] **TRUST-06**: A report's severity is used for triage sort order only, and cannot by itself
      bypass the provisional visibility gate (TRUST-04)
- [ ] **TRUST-07**: A report displays two distinct trust signals — a fast-decaying "is this still
      current" confidence score and a slow-decaying "is this source reliable" score
- [ ] **TRUST-08**: User (reporter or a nearby confirmer) can mark a report resolved, removing it
      from the live feed
- [ ] **TRUST-09**: Concurrent confirm/dispute votes submitted on the same report at the same time
      never silently lose an update (verified by an automated concurrency test)

### Coordination

- [ ] **COORD-01**: User can post an "I'm safe" check-in for themselves, tagged to a location
- [ ] **COORD-02**: User can search "I'm safe" check-ins by name or area
- [ ] **COORD-03**: User can post a "need" report (water/medicine/transport) or an "offer" report,
      and view needs and offers near each other
- [ ] **COORD-04**: User can view a triage list filtered to severity=critical or
      category=rescue-needed reports
- [ ] **COORD-05**: User can mark an active report "responding" with an optional ETA; the claim
      auto-expires and never permanently hides the report from the coverage-gap view
- [ ] **COORD-06**: User can mark a report "contacted at HH:MM" after phoning the reporter
      directly, with a field-verified priority that outranks passive vote count in the triage view
- [ ] **COORD-07**: User can retrieve their own submitted report's current status via a receipt
      code, without re-posting to check
- [ ] **COORD-08**: User can generate a shareable card for a report containing only a
      live-resolving status link — never a baked-in confirmation count or "verified" claim baked
      into the image itself

### Robustness

- [ ] **ROBUST-01**: User can view a low-bandwidth, text-only fallback feed (no map, no photos) on
      a degraded connection
- [ ] **ROBUST-02**: A report submitted while offline is queued locally (IndexedDB) and
      automatically sent once connectivity returns
- [ ] **ROBUST-03**: A submitted report's text is screened by an automated toxicity/spam check
      before publishing, calibrated to a documented false-positive target; critical/rescue-needed
      reports fail open (visible while the check is pending), other categories fail closed
      (provisional)
- [ ] **ROBUST-04**: Report submission is rate-limited primarily per anonymous session, with IP
      as a secondary, looser limit — not primary IP-based limiting
- [ ] **ROBUST-05**: The map displays official pins ingested from the GDACS open-data feed
      alongside crowd reports, unaffected by crowd confirm/dispute votes
- [ ] **ROBUST-06**: GDACS-sourced pins display a distinct authority badge, visually
      distinguishable from crowd reports and from demo-mode reports
- [ ] **ROBUST-07**: A first-time visitor sees a populated, working demo (seeded past-event
      timeline) and can trigger a simulated report to try the confirm/dispute flow themselves
- [ ] **ROBUST-08**: Every page displays a persistent disclaimer that Pinalert is unofficial, not
      affiliated with any government agency, and not a substitute for calling emergency services

### Ops

- [ ] **OPS-01**: The JSON API is documented via an OpenAPI/Swagger spec accessible at a stable
      URL
- [ ] **OPS-02**: Every push runs automated tests, `go vet`, and a build check via GitHub Actions
      CI

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### PWA & Real-Time

- **PWA-01**: User can attach a photo to a report
- **PWA-02**: Feed updates live without a manual refresh (websockets)
- **PWA-03**: User can opt in to a push notification when a critical report appears near a saved
  location (Web Push/VAPID)
- **PWA-04**: A retraction pushes a proactive notification to every session that viewed/confirmed
  the report while it was live (needs a `report_views` table)
- **PWA-05**: Background Sync (Chromium-only enhancement layered on the IndexedDB baseline from
  ROBUST-02)

### Data Richness

- **DATA-01**: Flooding reports carry an optional water-depth field
  (ankle/knee/waist/vehicle-submerged), rendered as a graduated map overlay
- **DATA-02**: Near-identical reports are clustered into one pin via text-embedding similarity
- **DATA-03**: Uploaded photos are perceptual-hash-checked against known viral hoax images and
  flag a match
- **DATA-04**: User can self-declare a report as "I saw this myself" vs. "I heard about this"
- **DATA-05**: Severity/category is auto-suggested from the free-text description

### Trust Extensions

- **TRUSTX-01**: A confirmer's votes on other users' reports are scored for historical accuracy
  and weighted accordingly (separate from reporter reputation)
- **TRUSTX-02**: A designated moderator can immediately mark a report verified, bypassing the
  confirm-threshold wait
- **TRUSTX-03**: A disproportionate burst of votes from a small session/subnet cluster is detected
  and excluded from the public tally

### Reach

- **REACH-01**: Critical/rescue-needed alerts specifically ping opted-in users who tagged a
  relevant capability (boat, medical training, generator)
- **REACH-02**: A "can you confirm this?" prompt is pushed to sessions already engaged in that
  area recently
- **REACH-03**: User can flag a report's free text as needing translation and submit an inline
  translation
- **REACH-04**: UI and category labels are available in Hindi and one regional language

### Triage

- **TRIAGE-01**: User can rapidly confirm/dispute/skip a queue of unconfirmed reports one at a
  time (swipe interface)

### Standards & Interop

- **STD-01**: Report category/severity maps to CAP `responseType`/certainty vocabulary, displayed
  as a plain-language badge
- **STD-02**: User can export a packaged report or area digest formatted for handoff to municipal/
  NDRF contacts
- **STD-03**: Alert-radius push delivery is tracked per-recipient (Confirmed/Acknowledged/
  No-response) with auto-escalation on timeout
- **STD-04**: User can report or query nearby status via WhatsApp/SMS (Twilio or WhatsApp Cloud
  API)
- **STD-05**: The map shows a suggested route avoiding a reported blocked road
- **STD-06**: A verified-authority account (municipal corp, NDRF) can post/confirm reports under
  a badged identity

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Missing/found person registry | Highest abuse-surface data type (third-party PII) on an anonymous, no-signup platform — needs a moderation-gated design that doesn't exist yet, not a quick CRUD add |
| Per-incident real-time coordination channel (chat) | Materially more work than the rest of the feature set (real-time chat infra); revisit only if the core product gets real usage |
| Full CAP-compliant alert feed with Update/Cancel lifecycle threading | Real interoperability value is speculative without an actual government integration path |
| Digitally-signed authority reports (XML-Signature on CAP alerts) | Only meaningful once the CAP feed exists and there are real authority accounts to hold keys |

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| — | — | Pending roadmap |

**Coverage:**
- v1 requirements: 27 total
- Mapped to phases: 0 (pending roadmap creation)
- Unmapped: 27 ⚠️

---
*Requirements defined: 2026-09-05*
*Last updated: 2026-09-05 after initial definition*
