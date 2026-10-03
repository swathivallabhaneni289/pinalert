# Requirements: Pinalert

**Defined:** 2026-09-05
**Core Value:** A report showing "confirmed by N nearby" must be verifiably backed by N
independent nearby confirmations, resistant to trivial gaming.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Identity & Login

- [x] **IDENT-01**: A visitor cannot submit a report or cast a confirm/dispute vote until they've
      verified an email address by clicking a magic link sent to it (decided 2026-09-10 during
      Phase 1.1 discussion — a typed one-time passcode was the original assumption, superseded by
      a clickable link) — supersedes FOUND-01's anonymous/no-signup model, reversed 2026-09-10
      (see Key Decisions in PROJECT.md)

- [x] **IDENT-02**: Verification-email delivery uses a free-tier transactional email provider
      (e.g. Resend), within its free quota — no paid SMS/phone verification path exists in this
      phase (phone OTP has no free tier at any real volume and requires India DLT sender
      registration for the cheap rate; deferred, see PROJECT.md Key Decisions)

- [x] **IDENT-03**: A verified user can view their own profile page, listing their submitted
      reports and their confirm/dispute voting activity

- [x] **IDENT-04**: Verification-email requests are rate-limited per email address and per IP
      (30-60 second resend cooldown), so the email-sending endpoint can't be trivially abused for
      spam or cost inflation

- [x] **IDENT-05**: The Phase 1 session-cookie mechanism is retained as the underlying
      identity/session carrier — it is now gated behind a verified account rather than usable
      anonymously; Phase 2's votes/reports continue to key off `session_id` as designed

### Foundation

- [x] **FOUND-01**: An anonymous session identity is issued to a first-time visitor (cookie/
      localStorage), with no signup required — **superseded 2026-09-10 by IDENT-01** (mandatory
      email verification via magic link); shipped and verified as originally specified in Phase 1,
      but the access model was deliberately reversed afterward (see PROJECT.md Key Decisions)

- [x] **FOUND-02**: User can submit a location-tagged report (GPS or manual area) with a category
      (flood/earthquake/fire/storm-cyclone damage/road blocked/power outage/shelter open/
      rescue needed/other — 9 categories, expanded from an original flood/cyclone-only 5-category
      list during Phase 1 discussion), severity (low/medium/critical), and free-text description

- [x] **FOUND-03**: User can view a feed of reports filtered to those near their current location,
      served via an indexed bounding-box prefilter + Haversine query (not a full-table scan)

- [x] **FOUND-04**: User can view reports as pins on a Leaflet/OpenStreetMap map
- [x] **FOUND-05**: A report stops appearing in the live feed once its expiry time passes, checked
      as a read-time predicate on every query (not solely dependent on a background sweep job)

- [x] **FOUND-06**: A "shelter open" report can carry a capacity status
      (Available/Limited/Full/Closed) with an optional headcount

### Trust Engine

- [x] **TRUST-01**: User can confirm or dispute another user's report
- [x] **TRUST-02**: A report's visibility state (Hidden/Provisional/Live/Retracted) is computed by
      one shared resolver function and is identical everywhere it's shown — feed, map, triage
      view, and shareable cards

- [x] **TRUST-03**: A vote counts toward the independence threshold only if it comes from a
      distinct anonymous session AND a distinct geohash cell from every other counted vote on
      that report

- [x] **TRUST-04**: A newly submitted non-critical report displays as "provisional" until a
      second independent confirmation arrives; critical/rescue-needed reports publish at full
      visibility immediately, with no gate

- [ ] **TRUST-05**: The feed displays "confirmed by N nearby" computed from the count of distinct
      confirming geohash cells/sessions, not the raw number of votes

- [x] **TRUST-06**: A report's severity is used for triage sort order only, and cannot by itself
      bypass the provisional visibility gate (TRUST-04)

- [ ] **TRUST-07**: A report displays two distinct trust signals — a fast-decaying "is this still
      current" confidence score and a slow-decaying "is this source reliable" score

- [x] **TRUST-08**: User (reporter or a nearby confirmer) can mark a report resolved, removing it
      from the live feed

- [x] **TRUST-09**: Concurrent confirm/dispute votes submitted on the same report at the same time
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

- [ ] **ROBUST-06**: GDACS-sourced pins and SACHET alerts display a distinct authority badge, visually
      distinguishable from crowd reports and from demo-mode reports

- [ ] **ROBUST-07**: A first-time visitor sees a populated, working demo (seeded past-event
      timeline) and can trigger a simulated report to try the confirm/dispute flow themselves

- [ ] **ROBUST-08**: Every page displays a persistent disclaimer that Pinalert is unofficial, not
      affiliated with any government agency, and not a substitute for calling emergency services

### Report History

- [ ] **HIST-01**: User can switch the list and the map between Active, Resolved, Disputed and Past
      reports with one View selector in place of the disputed checkbox; Active is the default. The four
      views are a disjoint partition (Active: Live and Provisional; Disputed: Hidden; Resolved: closed
      by people; Past: expired), and voting stays available in Active and Disputed, so a Hidden report
      can still be confirmed back, but not in Resolved or Past (added 2026-10-03)

- [ ] **HIST-02**: The Resolved view lists reports that people closed within the same bounded window
      as Past, each labelled with when it was resolved and who closed it (the reporter alone or
      independent confirmers) and showing the Phase 3 counters, so self-resolving cannot hide a
      dispute; rescue-needed rows are shown at a coarse location

- [ ] **HIST-03**: The Past view lists reports that expired in the last 24 hours, 3 days or 7 days,
      each labelled with its outcome (confirmed by N places, expired unconfirmed, or disputed),
      using the same outcome rules as the reporter rating; critical and rescue-needed reports that
      expire unconfirmed get a neutral "Expired" label, never a negative one, and rescue-needed rows are
      shown at a coarse location

- [ ] **HIST-04**: When no active reports exist, the list says what happened recently using real
      counts only (for example "2 were resolved today") and links to the Resolved view

- [ ] **HIST-05**: User can search for a place on the main map and see reports for that area, not
      only reports near their own location

- [ ] **HIST-06**: User can filter the list and map by category and by severity

### Official Alerts

- [ ] **ALERT-01**: An Alerts view lists the official alerts that cover the reader's district,
      ingested server side from NDMA's SACHET feed (IMD, CWC and state authority alerts), read
      only and unaffected by crowd votes (added 2026-10-03)

- [ ] **ALERT-02**: Each official alert shows its issuer, when it was issued and when it expires,
      carries the authority badge, and disappears when it expires

- [ ] **ALERT-03**: A banner appears at the top of the list only when a Severe or Extreme official
      alert covers the reader's area

- [ ] **ALERT-04**: The app shows when each official feed was last checked and says plainly when a
      feed is unavailable, so a stale alert never looks current

### Updates & Following

- [ ] **NOTIF-01**: A signed-in user sees a bell with an unread count and an Updates list;
      tapping an update opens the report (added 2026-10-03)

- [ ] **NOTIF-02**: A user automatically follows a report they posted or voted on and can mute any
      report

- [ ] **NOTIF-03**: Updates about a followed report are sent for state changes only (the report
      reaches Live, is hidden by disputes, is resolved, or is within 1 hour of expiring), at most one
      per report per hour, decided by the single visibility resolver, worded anonymously from the
      same server trust object the popup uses, and never naming a voter, an account or a location;
      time based events are evaluated lazily when the bell refreshes

- [ ] **NOTIF-04**: User can filter the feed to Reliable reporters only; critical and rescue-needed
      reports always stay visible

- [ ] **NOTIF-05**: User can opt in to an update when a Reliable reporter posts within a chosen
      distance of where the reader is looking; non-critical reports notify only once Live,
      critical and rescue-needed reports immediately, with a cap on alerts per reporter per
      recipient per hour

- [ ] **PROF-01**: A user can opt in to a public reporter profile with a nickname (never the
      email); it is off by default and available only to Reliable reporters, and nicknames are unique,
      screened and exclude reserved words such as agency names

- [ ] **PROF-02**: A profile shows the reliability tag with its counts, member since, and recent
      finished reports at a coarse location, with no email and no follower counts

- [ ] **PROF-03**: A user can follow and unfollow opted-in reporters privately, mute or block,
      see their reports in a Following view and get updates when they post (non-critical only once
      Live, no coordinates or address in the text); switching the profile off, or losing the Reliable
      tag, ends all follows and pending alerts, and a reporter can remove all followers without
      seeing who they are

### Report Page & Comments

- [ ] **PAGE-01**: A report opens into its own page with the shared trust block and a history
      timeline built from the vote log (posted, confirmed from N places, disputed, resolved)

- [ ] **PAGE-02**: A signed-in user can read and add short comments on a report page, each screened
      by moderation and rate-limited per account; commenting makes the user follow that report

### Ops

- [x] **OPS-01**: The JSON API is documented via an OpenAPI/Swagger spec accessible at a stable
      URL

- [x] **OPS-02**: Every push runs automated tests, `go vet`, and a build check via GitHub Actions
      CI

### Presentation

- [x] **UX-01**: A user can choose Light, Dark, or follow-the-OS appearance from inside the app
      (added 2026-09-18 from Phase 2 UAT — the app already renders both palettes via
      `prefers-color-scheme`/`[data-theme]`, but nothing lets a user choose independently of the
      OS setting)

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

- **PWA-06**: Phase 3.2's updates and Severe or Extreme official alerts for the reader's district are
  delivered as Web Push to installed PWAs

- **PWA-07**: Opt-in email digests of updates (needs a DNS verified sending domain)

### History & Replay

- **HIST-07**: A time slider rebuilds the map as it looked at any past moment, from the vote log

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
| IDENT-01 | Phase 1.1 | Complete |
| IDENT-02 | Phase 1.1 | Complete |
| IDENT-03 | Phase 1.1 | Complete |
| IDENT-04 | Phase 1.1 | Complete |
| IDENT-05 | Phase 1.1 | Complete |
| FOUND-01 | Phase 1 | Complete (superseded by IDENT-01, Phase 1.1) |
| FOUND-02 | Phase 1 | Complete |
| FOUND-03 | Phase 1 | Complete |
| FOUND-04 | Phase 1 | Complete |
| FOUND-05 | Phase 1 | Complete |
| FOUND-06 | Phase 1 | Complete |
| OPS-01 | Phase 1 | Complete |
| OPS-02 | Phase 1 | Complete |
| TRUST-01 | Phase 2 | Complete |
| TRUST-02 | Phase 2 | Complete |
| TRUST-03 | Phase 2 | Complete |
| TRUST-04 | Phase 2 | Complete |
| TRUST-06 | Phase 2 | Complete |
| TRUST-08 | Phase 2 | Complete |
| TRUST-09 | Phase 2 | Complete |
| UX-01 | Phase 2 | Complete (gap-closure plan 02-10) |
| TRUST-05 | Phase 3 | Pending |
| TRUST-07 | Phase 3 | Pending |
| HIST-01 | Phase 3.1 | Pending |
| HIST-02 | Phase 3.1 | Pending |
| HIST-03 | Phase 3.1 | Pending |
| HIST-04 | Phase 3.1 | Pending |
| HIST-05 | Phase 3.1 | Pending |
| HIST-06 | Phase 3.1 | Pending |
| NOTIF-01 | Phase 3.2 | Pending |
| NOTIF-02 | Phase 3.2 | Pending |
| NOTIF-03 | Phase 3.2 | Pending |
| NOTIF-04 | Phase 3.2 | Pending |
| NOTIF-05 | Phase 3.2 | Pending |
| ROBUST-01 | Phase 4 | Pending |
| ROBUST-02 | Phase 4 | Pending |
| ROBUST-03 | Phase 4 | Pending |
| ROBUST-04 | Phase 4 | Pending |
| ROBUST-08 | Phase 4 | Pending |
| ROBUST-05 | Phase 5 | Pending |
| ROBUST-06 | Phase 5 | Pending |
| ROBUST-07 | Phase 5 | Pending |
| ALERT-01 | Phase 5 | Pending |
| ALERT-02 | Phase 5 | Pending |
| ALERT-03 | Phase 5 | Pending |
| ALERT-04 | Phase 5 | Pending |
| COORD-01 | Phase 6 | Pending |
| COORD-02 | Phase 6 | Pending |
| COORD-03 | Phase 6 | Pending |
| COORD-04 | Phase 6 | Pending |
| COORD-05 | Phase 6 | Pending |
| COORD-06 | Phase 6 | Pending |
| COORD-07 | Phase 6 | Pending |
| COORD-08 | Phase 6 | Pending |
| PAGE-01 | Phase 8 | Pending |
| PAGE-02 | Phase 8 | Pending |
| PROF-01 | Phase 9 | Pending |
| PROF-02 | Phase 9 | Pending |
| PROF-03 | Phase 9 | Pending |

**Coverage:**

- v1 requirements: 59 total (33 original + 5 new IDENT-01..05 added 2026-09-10 for the inserted
  Phase 1.1: Identity & Login + 1 new UX-01 added 2026-09-18 from Phase 2 UAT + 20 new added
  2026-10-03 for roadmap Phases 3.1, 3.2, 5, 8 and 9: HIST 6, ALERT 4, NOTIF 5, PAGE 2, PROF 3.
  By group: Foundation 6, Identity 5, Trust Engine 9, Coordination 8, Robustness 8, Ops 2,
  Presentation 1, Report History 6, Official Alerts 4, Updates & Following 8, Report Page 2)

- Mapped to phases: 59 (100%)
- Unmapped: 0

---
*Requirements defined: 2026-09-05*
*Last updated: 2026-10-03: added HIST-01..06, ALERT-01..04, NOTIF-01..05, PAGE-01..02 and PROF-01..03
for roadmap Phases 3.1, 3.2, 5, 8 and 9 (raised by the owner after the Phase 3 popup sketch), plus
v2 items PWA-06, PWA-07 and HIST-07. Earlier update, 2026-09-18: added UX-01 (in-app light/dark toggle) surfaced by Phase 2's UAT gap
closure round, linked to plan 02-10; added IDENT-01..05 for Phase 1.1 (mandatory email+OTP login,
inserted before Phase 2 after the user decided anonymous voting was too gameable); FOUND-01
marked superseded, not removed (it shipped and was verified as originally specified in Phase 1)*
