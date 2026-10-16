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

- [x] **TRUST-05**: The feed displays "confirmed by N nearby" computed from the count of distinct
      confirming geohash cells/sessions, not the raw number of votes

- [x] **TRUST-06**: A report's severity is used for triage sort order only, and cannot by itself
      bypass the provisional visibility gate (TRUST-04)

- [x] **TRUST-07**: A report displays two distinct trust signals — a fast-decaying "is this still
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
      a degraded connection, and can post a report and confirm or dispute from it

- [ ] **ROBUST-02**: A report submitted while offline is queued locally (IndexedDB) and
      automatically sent once connectivity returns

- [ ] **ROBUST-03**: A submitted report's text is screened by an automated toxicity/spam check
      before publishing, calibrated to a documented false-positive target; clearly abusive text is
      rejected at submit for every severity and borderline text is listed for moderators;
      critical/rescue-needed reports fail open (visible while the check is pending), other
      categories fail closed (held off every feed until the check finishes)

- [ ] **ROBUST-04**: Report submission, votes, vote changes and flags are rate-limited primarily
      per verified account, with the network address only as a very loose secondary limit, never
      primary IP-based limiting

- [ ] **ROBUST-05**: The map displays official pins ingested from the GDACS open-data feed
      alongside crowd reports, unaffected by crowd confirm/dispute votes

- [ ] **ROBUST-06**: GDACS-sourced pins and SACHET alerts display a distinct authority badge, visually
      distinguishable from crowd reports and from demo-mode reports

- [ ] **ROBUST-07**: A first-time visitor sees a populated, working demo (seeded past-event
      timeline) and can trigger a simulated report to try the confirm/dispute flow themselves
      (Amended 2026-10-16, owner decision: a Demo button on the login screen opens a sandboxed
      practice copy with no login; the sample is the 2024 Vijayawada floods along the Budameru, with
      real place names and made-up reports, each marked Demo)

- [ ] **ROBUST-08**: Every page displays a persistent disclaimer that Pinalert is unofficial, not
      affiliated with any government agency, and not a substitute for calling emergency services
      (Amended 2026-10-16, owner decision: met on the laptop; on the phone the owner chose no strip,
      so the disclaimer is read and agreed in the first-time flow and is one tap away in Settings,
      About Pinalert; the legal review risk stays, see 03.4-CONTEXT D-30)

- [ ] **ROBUST-09**: A moderator (the owner, or anyone on a server-side list) can open a screen no
      one else can open, see automatically flagged posts, user flags and posts whose text check
      has not finished, and Keep or Remove any post; any signed-in reader can flag a post with a
      reason, and a flag never hides a post by itself

- [ ] **ROBUST-10**: Public Disclaimer and Policy pages, readable without logging in and carrying
      no account data, state what Pinalert is not, the emergency numbers, what is screened and
      kept, and how to complain or ask for removal

### Report History

- [ ] **HIST-01**: User can switch the list and the map between Active, Disputed and Past reports
      with one tab strip at the top of the list, in place of the disputed checkbox, on the laptop and
      on the phone; Active is the default. The three tabs are a disjoint partition (Active: Live and
      Provisional; Disputed: Hidden; Past: closed by people or expired, shown as a Resolved group and
      an Expired group), and voting stays available in Active and Disputed, so a Hidden report can
      still be confirmed back, but not in Past, except that the person who marked a report resolved
      can take that mark back and the poster can reopen (added 2026-10-03, reworded 2026-10-14)
      (Amended 2026-10-16, owner decision: Phase 3.4 replaces this tab strip with the Show menu on
      Nearby (All, Confirmed, Disputed) and the Ended nearby part of My activity, which takes the
      Past content)

- [ ] **HIST-02**: The Resolved group of the Past tab lists reports that people closed in the last
      7 days, each labelled with when it was resolved and who closed it (the reporter alone or
      independent confirmers) and showing the Phase 3 counters, so self-resolving cannot hide a
      dispute; a finished rescue-needed row shows an approximate location (about 1 km) and no
      description text (reworded 2026-10-14)

- [ ] **HIST-03**: The Expired group of the Past tab lists reports that expired in the last 7 days,
      each labelled with its outcome (confirmed by N people nearby, expired unconfirmed, or
      disputed), using the same outcome rules as the reporter rating; critical and rescue-needed
      reports that expire unconfirmed get a neutral "Expired" label, never a negative one, and a
      finished rescue-needed row shows an approximate location (about 1 km) and no description text
      (reworded 2026-10-14)

- [ ] **HIST-04**: When no active reports exist, the list says what happened recently using real
      counts only (for example "2 were resolved in the last 24 hours") and links to the Past tab
      (reworded 2026-10-14)

- [ ] **HIST-05**: User can search for a place on the main map and see reports for that area, not
      only reports near their own location

- [ ] **HIST-06**: User can narrow the list and map by category with chips (All, Flood, Rescue,
      Shelter, Power) that hide nothing unless tapped; a severity filter is built only if the owner
      asks (reworded 2026-10-14)

- [ ] **HIST-08**: The report rows in the list are compact on every screen size: the laptop list
      uses the phone card, so at least five rows fit on an 800px tall screen, and Confirm, Dispute
      and Mark resolved are in the opened report, not on the row (added 2026-10-14, owner picked
      size A of sketch 006)

- [ ] **HIST-09**: A person who marked a report resolved by mistake can take that mark back from
      the opened report, at most twice on one report (the poster's Reopen counts against the same
      limit), and a short Undo follows the Yes; a faded report cannot be changed, and a take back
      never changes the poster's reporter rating (added 2026-10-14)

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
      (Amended 2026-10-16, owner decision: the bell sits top right on the phone and beside the
      person menu on the laptop and opens the latest 10 updates; See all updates opens My activity,
      Updates)

- [ ] **NOTIF-02**: A user automatically follows a report they posted or voted on and can mute any
      report

- [ ] **NOTIF-03**: Updates about a followed report are sent for state changes only (the report
      reaches Live, is hidden by disputes, is resolved, or is within 1 hour of expiring), at most one
      per report per hour, decided by the single visibility resolver, worded anonymously from the
      same server trust object the popup uses, and never naming a voter, an account or a location;
      time based events are evaluated lazily when the bell refreshes

- [ ] **NOTIF-04**: User can filter the feed to Reliable reporters only; critical and rescue-needed
      reports always stay visible
      (Amended 2026-10-16, owner decision: the Only Reliable reporters tick lives in the Show menu
      on Nearby)

- [ ] **NOTIF-05**: User can opt in to an update when a Reliable reporter posts within a chosen
      distance of where the reader is looking; non-critical reports notify only once Live,
      critical and rescue-needed reports immediately, with a cap on alerts per reporter per
      recipient per hour
      (Amended 2026-10-16, owner decision: the distance is measured from the person's home area,
      1, 2, 5 or 10 km, set in Settings as How far from my home area)

- [ ] **PROF-01**: A user can opt in to a public reporter profile with a nickname (never the
      email); it is off by default and available only to Reliable reporters, and nicknames are unique,
      screened and exclude reserved words such as agency names
      (Amended 2026-10-16, owner decision: the name given in the first-time flow replaces the
      nickname and keeps the reserved word screening; the profile is still opt in)

- [ ] **PROF-02**: A profile shows the reliability tag with its counts, member since, and recent
      finished reports at a coarse location, with no email and no follower counts
      (Amended 2026-10-16, owner decision: readers also tap one to five stars, with guard rails,
      and from 5 raters on the stars are half report outcomes and half the tapped average, see
      ROADMAP Phase 9; the public Followers and Following lists in PROF-03's note replace "no
      follower counts")

- [ ] **PROF-03**: A user can follow and unfollow opted-in reporters privately, mute or block,
      see their reports in a Following view and get updates when they post (non-critical only once
      Live, no coordinates or address in the text); switching the profile off, or losing the Reliable
      tag, ends all follows and pending alerts, and a reporter can remove all followers without
      seeing who they are
      (Amended 2026-10-16, owner decision: the Followers and Following lists are public on the
      profile, and anyone without their own opted in profile shows only as a number, never by name,
      so "remove all followers without seeing who they are" no longer holds, see ROADMAP Phase 9)

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

- [x] **UX-01**: A user can switch between Light and Dark appearance from inside the app with one
      round icon button. The operating system setting only picks the very first value, after that
      the choice is remembered. (Changed on 2026-09-23 from "Light, Dark, or follow-the-OS" by the
      owner's decision, quick task 260923-qwi.)
      (added 2026-09-18 from Phase 2 UAT — the app already renders both palettes via
      `prefers-color-scheme`/`[data-theme]`, but nothing lets a user choose independently of the
      OS setting)
      (Amended 2026-10-16, owner decision of 2026-10-15: the in-app button stays on the laptop; the
      phone has no button and follows the device's light or dark setting, see 03.4-CONTEXT D-08)

### App Layout

- [ ] **LAYOUT-01**: The app has three main places on the phone and the laptop alike, Nearby (opens
      first), Official alerts and My activity: the phone shows them in a bottom bar with a fourth
      Report item and keeps the bell and the Settings gear at the top right of every screen, and the
      laptop shows them in a top bar with Post a report, Search a place, the bell, the light and dark
      button and a person menu (My profile, Settings, Log out). Both sizes reach the same features
      (added 2026-10-16, Phase 3.4)

- [ ] **LAYOUT-02**: Nearby shows "N reports, 5 km" and one Show menu (All by default, Confirmed,
      Disputed, and NOTIF-04's Only Reliable reporters tick) instead of a tab strip; in All, disputed
      reports sit at the bottom under "Disputed: may not be true" with a grey icon and a Disputed
      tag, never faded, as presentation only, so the single visibility resolver and its Hidden
      outcome do not change. The official alert banner sits at the top of the list only when a
      Severe or Extreme alert covers the area, and opens Official alerts (added 2026-10-16,
      Phase 3.4)

- [ ] **LAYOUT-03**: On the phone the Nearby list is a sheet over the map with three positions
      (short, half by default, full): scrolling up first grows it, a grey handle can be dragged or
      tapped, and the map resizes so no pin hides under it. On both sizes, tapping a report shows
      its pin and tapping a pin shows its report, and the chosen card gets a soft grey fill (added
      2026-10-16, Phase 3.4)

- [ ] **LAYOUT-04**: My activity holds five parts, Updates, Posted by you, You confirmed, You
      disputed and Ended nearby (reports near the reader resolved or faded out in the last 7 days,
      the old Past content): the phone opens on a short list whose rows open each part on its own
      screen with a back arrow, and the laptop stacks the parts with their rows lined up in columns.
      Every feature of today's Activity page stays, including the trust counts and panel, the state
      word, Reopen and take back, the reporter record and the empty states (added 2026-10-16,
      Phase 3.4)

- [ ] **LAYOUT-05**: Post a report opens full screen from the phone's Report item and as a wide two
      pane window from the laptop's Post a report button (map with the address search and the
      draggable pin on the left, form on the right), and closes only with the X. The chosen
      severity is filled with its own soft colour, and a "Someone in danger right now? Call 112
      first, then post here." box with a Call 112 button and the What is happening? picks appears
      only for a Critical or Rescue needed report (added 2026-10-16, Phase 3.4)

- [ ] **LAYOUT-06**: One Settings page, opened from the gear on the phone and from the person menu
      on the laptop, stacks About you, Phone notifications, People and reports, About Pinalert
      (Disclaimer, Policy, Low data view) and Account (Log out of all devices, Delete my account).
      A row appears only once its feature exists, and later phases add their rows here, never a
      second settings place (added 2026-10-16, Phase 3.4)

- [ ] **LAYOUT-07**: After the first sign in, a five step first-time flow runs (full screen with a
      progress bar on the phone, a wide window with the step list on the laptop): Welcome,
      Disclaimer and policy read in a scroll box and agreed (only Agree moves on, and Disagree
      points to the Demo), Your location, About you (name and home area, both optional) and Alerts,
      which arrives with Phase 3.3 (added 2026-10-16, Phase 3.4)

- [ ] **LAYOUT-08**: The laptop shows a dark strip at the top of every page (unofficial, not a
      government service, cannot send help, reports can be wrong, in danger call 112) with a Read
      the disclaimer link; by owner decision the phone shows no strip on its main screens and
      carries the disclaimer in the first-time flow and in Settings, About Pinalert (see ROBUST-08).
      Disclaimer and Policy open in one reader that closes with the X; whether both sizes should
      match here is still open, see 03.4-CONTEXT O-1 (added 2026-10-16, Phase 3.4)

- [ ] **LAYOUT-09**: Every screen keeps the owner's rules: the phone and the laptop offer the same
      features, only arranged differently; nothing is repeated and each window has one way to
      close, the X; nothing a user needs sits off screen or under something floating; a chosen
      option is never outlined in black; no text is faded; laptop pages use the full width; and the
      phone follows the device's light or dark setting (added 2026-10-16, Phase 3.4)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### PWA & Real-Time

- **PWA-01**: User can attach a photo to a report
- **PWA-02**: Feed updates live without a manual refresh (websockets)
- **PWA-03**: User can opt in to a push notification when a critical report appears near a saved
  location (Web Push/VAPID)
  (Amended 2026-10-16, owner decision: Phase 3.3 builds it; the saved location is the person's
  home area, and rescue and critical reports near it are on by default once phone notifications
  are on; severe official alerts also buzz the phone, see PWA-06)

- **PWA-04**: A retraction pushes a proactive notification to every session that viewed/confirmed
  the report while it was live (needs a `report_views` table)

- **PWA-05**: Background Sync (Chromium-only enhancement layered on the IndexedDB baseline from
  ROBUST-02)

- **PWA-06**: Phase 3.2's updates and Severe or Extreme official alerts for the reader's district are
  delivered as Web Push to installed PWAs
  (Amended 2026-10-16, owner decision: Phase 3.3 builds the updates part; Severe or Extreme
  official alerts covering the home area buzz the phone, on by default with a Settings switch,
  using 3.3's push plumbing and turned on in Phase 5)

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
| TRUST-05 | Phase 3 | Complete |
| TRUST-07 | Phase 3 | Complete |
| HIST-01 | Phase 3.1 | Pending |
| HIST-02 | Phase 3.1 | Pending |
| HIST-03 | Phase 3.1 | Pending |
| HIST-04 | Phase 3.1 | Pending |
| HIST-05 | Phase 3.1 | Pending |
| HIST-06 | Phase 3.1 | Pending |
| HIST-08 | Phase 3.1 | Pending |
| HIST-09 | Phase 3.1 | Pending |
| NOTIF-01 | Phase 3.2 | Pending |
| NOTIF-02 | Phase 3.2 | Pending |
| NOTIF-03 | Phase 3.2 | Pending |
| NOTIF-04 | Phase 3.2 | Pending |
| NOTIF-05 | Phase 3.2 | Pending |
| LAYOUT-01 | Phase 3.4 | Pending |
| LAYOUT-02 | Phase 3.4 | Pending |
| LAYOUT-03 | Phase 3.4 | Pending |
| LAYOUT-04 | Phase 3.4 | Pending |
| LAYOUT-05 | Phase 3.4 | Pending |
| LAYOUT-06 | Phase 3.4 | Pending |
| LAYOUT-07 | Phase 3.4 | Pending |
| LAYOUT-08 | Phase 3.4 | Pending |
| LAYOUT-09 | Phase 3.4 | Pending |
| ROBUST-01 | Phase 4 | Pending |
| ROBUST-02 | Phase 4 | Pending |
| ROBUST-03 | Phase 4 | Pending |
| ROBUST-04 | Phase 4 | Pending |
| ROBUST-08 | Phase 4 | Pending |
| ROBUST-09 | Phase 4 | Pending |
| ROBUST-10 | Phase 4 | Pending |
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

- v1 requirements: 72 total (33 original + 5 new IDENT-01..05 added 2026-09-10 for the inserted
  Phase 1.1: Identity & Login + 1 new UX-01 added 2026-09-18 from Phase 2 UAT + 20 new added
  2026-10-03 for roadmap Phases 3.1, 3.2, 5, 8 and 9: HIST 6, ALERT 4, NOTIF 5, PAGE 2, PROF 3
  + 2 new ROBUST-09 and ROBUST-10 added 2026-10-06 for the Phase 4 scope the owner added in discussion
  + 2 new HIST-08 and HIST-09 added 2026-10-14 from the Phase 3.1 discussion, the compact rows and
  the take back of a mistaken Mark resolved + 9 new LAYOUT-01 to LAYOUT-09 added 2026-10-16 for the
  inserted Phase 3.4: App layout, from the owner's approved mockup (sketch 007).
  HIST-07 is a v2 item and is not counted.
  By group: Foundation 6, Identity 5, Trust Engine 9, Coordination 8, Robustness 10, Ops 2,
  Presentation 1, Report History 8, Official Alerts 4, Updates & Following 8, Report Page 2,
  App Layout 9)

- Mapped to phases: 72 (100%)
- Unmapped: 0

---
*Requirements defined: 2026-09-05*
*Last updated: 2026-10-16: added LAYOUT-01 to LAYOUT-09 for the inserted Phase 3.4 (App layout, from the owner's approved mockup, sketch 007) and recorded the owner's decisions of 2026-10-15 and 2026-10-16 as dated notes on ROBUST-07, ROBUST-08, UX-01, HIST-01, NOTIF-01, NOTIF-04, NOTIF-05, PROF-01 to PROF-03, PWA-03 and PWA-06. Earlier update, 2026-10-14: HIST-01 to HIST-06 reworded to the tab strip Active, Disputed and Past, the fixed 7 day window, category chips and the approximate location for finished rescue reports, and HIST-08 and HIST-09 added (compact rows, take back of a mistaken Mark resolved), all from the Phase 3.1 discussion (03.1-CONTEXT.md). Earlier update, 2026-10-13: TRUST-05 and TRUST-07 marked complete when Phase 3 closed (UAT 11 of 11, verification passed). Earlier update, 2026-10-06: added ROBUST-09 and ROBUST-10 for the Phase 4 scope the owner added in discussion (the moderator screen with the Flag control, and the public Disclaimer and Policy pages) and reworded ROBUST-01, ROBUST-03 and ROBUST-04 to match. Earlier update, 2026-10-03: added HIST-01..06, ALERT-01..04, NOTIF-01..05, PAGE-01..02 and PROF-01..03
for roadmap Phases 3.1, 3.2, 5, 8 and 9 (raised by the owner after the Phase 3 popup sketch), plus
v2 items PWA-06, PWA-07 and HIST-07. Earlier update, 2026-09-18: added UX-01 (in-app light/dark toggle) surfaced by Phase 2's UAT gap
closure round, linked to plan 02-10; added IDENT-01..05 for Phase 1.1 (mandatory email+OTP login,
inserted before Phase 2 after the user decided anonymous voting was too gameable); FOUND-01
marked superseded, not removed (it shipped and was verified as originally specified in Phase 1)*
