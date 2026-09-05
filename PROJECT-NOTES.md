# Pinalert — verified local-info feed for emergencies

Portfolio project (job applications, India, not a startup). Go, PostgreSQL, Leaflet.js.
2-4 week core build, deployed live as a mobile-responsive website first, PWA/app later.

Name: **Pinalert** (pin = location, alert = the report) — checked against real apps/companies, no conflict found.

## What it is
A local, verified information feed for emergencies like floods or cyclones — a live, trustworthy
map of what's actually happening on the ground, instead of unverified rumors and WhatsApp forwards.

## What it does during an emergency
People near the affected area post short updates — a flooded road, a shelter that's open, a power
outage, someone needing rescue — each tagged to a location. Other people nearby can confirm or
dispute that update, so a report showing "confirmed by 12 people nearby" can be trusted, while an
unconfirmed one gets flagged. Reports are marked by urgency, and they auto-fade unless
re-confirmed, so the feed always reflects what's true right now, not what was true three hours ago.

---

## Core features (build first)

**Base loop:**
- Location-tagged reports — pinned to a place (GPS/area), feed filtered by proximity, not global.
- Report categories — flooding, road blocked, power outage, shelter open, rescue needed.
- Confirm/dispute on each report — nearby users vote true/false; post shows "confirmed by N
  nearby." This is the differentiating feature.
- Severity level — low/medium/critical, so urgent reports surface first.
- Auto-expiry — reports fade/archive after some hours unless re-confirmed, since conditions
  change fast.
- Map view — pins on a map for a quick visual scan.

**Trust-model hardening (research-backed, strengthens the core differentiator):**
- **Provisional visibility gate** — a new report in a non-critical category renders as a dimmed
  "provisional" pin until a second independent confirmation arrives; critical/rescue-needed
  reports always publish at full visibility instantly, no gate. *Source: Ushahidi's 2008 Kenya
  deployment, Banpani.org's Assam flood map, Wikipedia Pending Changes — three independent
  deployments converged on this same fix for single-source rumors spreading unchecked.*
- **Diversity-weighted confirm count** — "confirmed by N nearby" is computed from distinct
  confirming geohash cells / session fingerprints, not raw vote count, so 10 confirms from one
  building read as ~1-2 while 4 confirms from 4 directions read as 4. Same per-vote data already
  logged, aggregated differently. *Source: X/Twitter Community Notes bridging-based ranking.*
- **Split confidence/reliability decay** — two scores instead of one fading trust percentage:
  confidence (is this specific incident still real right now, decays fast without
  reconfirmation) and reliability (how trustworthy is the submitting session's history, decays
  slowly). *Source: Waze's per-incident confidence/reliability scoring.*
- **Retraction push + live-resolving status link on shareable cards** — if a report is later
  disputed into hiding or marked false after being widely seen, push a correction to every
  session that viewed/confirmed it while live, and give each shareable card a status URL that
  resolves to current state ("since retracted") instead of a frozen stale count. *Source: Boston
  Marathon bombing misinformation study — corrections reached ~1/44th the audience of the
  original false claim. This directly fixes a failure mode our own shareable-card feature would
  otherwise introduce.*

**Closing the information loop (from earlier discussion):**
- **"I'm safe" check-in** — anyone can post "I'm safe at [location]"; others can search a
  name/area to see check-ins. Same `reports` table, new category.
- **Needs ↔ offers matching** — "need" reports (water, medicine, transport) and "offer" reports
  (surplus water, can drive people out), filterable so needs and offers near each other surface
  together.
- **Volunteer/rescue-team triage view** — filtered to severity=critical + rescue-needed, oldest
  first. Same data, different query/page.
- **Shareable verified-report cards** — a simple image/card ("✅ Road X flooded — confirmed by
  12 nearby, 10 min ago") to reshare back into the WhatsApp groups the rumors came from.

**Data quality / real-world coordination (research-backed):**
- **Structured shelter capacity status** — extend "shelter open" with a status field
  (Available/Limited/Full/Closed) plus optional headcount. *Source: Cyclone Fani, ~1.5M people
  into 9,177 shelters in ~24 hours — a binary "open" pin can't represent that.*
- **Anonymous reporter receipt code** — after submitting, a short code/link lets the anonymous
  poster check their own report's live status instead of re-posting elsewhere to check if it
  went through. *Source: Kerala 2018 rescue-request study — ~25% of ~45,000 logged requests were
  duplicates, 5,000+ hand-collapsed by volunteers.*
- **Responder claim + coverage-gap layer** — a volunteer/team marks an active report
  "responding" with optional ETA, visible to others, so two teams don't converge on one spot; a
  separate layer surfaces unclaimed active reports. *Source: documented duplication of effort in
  Kerala 2018 (Banpani.org "Gaps" view, Assam floods, does the same thing).*
- **Human-contact verification tier** — a volunteer who actually phoned the person can mark
  "contacted at HH:MM" with a field-verified priority that outranks passive confirm/dispute
  count in the triage view. *Source: what the most effective real Kerala 2018 volunteers did by
  hand; `keralarescue.in`'s actual codebase has a field built for exactly this.*

**Robustness:**
- **Low-bandwidth / text-first fallback view** — no map, no photos, just a fast list (category,
  severity, "3 min ago, confirmed by 8") for degraded networks.
- **Background Sync (service worker)** — queue a report offline (IndexedDB) if the network call
  fails; retry automatically once connectivity returns.
- **Auto-moderation on submit** — toxicity/spam text filter and image-safety check on photos,
  run as a **confidence-cascade with an explicit false-positive budget**: above a high-confidence
  threshold, auto-hide instantly; middle band, leave visible but flag for review; below that, no
  action. Threshold chosen against a target false-positive rate (e.g. ≤0.1% of genuine reports
  auto-hidden), not an arbitrary score. *Source: Wikipedia's ClueBot NG anti-vandalism bot.*
- **Official open-data feed integration** — poll GDACS (Global Disaster Alert and Coordination
  System, free GeoJSON API) or IMD/CWC bulletins and insert as `source: official` pins on the map
  alongside crowd reports. Solves the empty-map-on-first-visit problem and gives the "authority
  badge" feature real data to badge.
- **Demo/replay mode** — seed the database with a real past event's timeline (reports,
  confirmations, expiries) so a first-time visitor sees the product working immediately, plus a
  "simulate a new report" button to try the confirm/dispute flow live.
- Legal disclaimer — explicit "unofficial, not affiliated with any government agency, not a
  substitute for calling emergency services," especially given the authority badge feature.
- OpenAPI/Swagger docs for the JSON API; GitHub Actions CI (`go test`, `go vet`, build).

---

## Stretch features

**Original:**
- Photo attachment on a report.
- Live updates without refresh (websockets/goroutines).
- Resolved marking — reporter or nearby users close out a report once it's no longer true.
- Authority badge — official source (municipal corp, NDRF) outranks crowd reports visually.
- Alert radius — opt-in notification when a critical report appears near a saved location, via
  **Web Push API** (works even when the site/tab is closed, not just live websockets), through
  the PWA/service-worker path already planned.

**Data richness:**
- **Structured water-depth field + graduated overlay** — for flooding reports, an optional depth
  selector (ankle/knee/waist/vehicle-submerged) alongside severity, rendered as a spatially
  interpolated flood-extent overlay in addition to discrete pins. *Source: validated against
  real field-survey data in a 2021 study of the 2015 Chennai flood using social-media reports.*
- **Duplicate-report clustering** — embed report descriptions and merge near-identical reports
  into one pin with a count instead of 40 separate pins for the same flooded road.
- **Reshare/duplicate-image flag** — perceptual-hash uploaded photos against other Pinalert
  photos and a small seeded corpus of known viral disaster-hoax images; flag matches instead of
  treating them as fresh evidence. *Source: "Faking Sandy" study — 86% of tweets spreading fake
  Hurricane Sandy images were retweets of a small set of images, not fresh captures.*
- **Self-declared eyewitness tag** — reporter can tag "I saw this myself" vs. "I heard about
  this," a visible badge distinct from severity/confirm-count.
- **Severity/category auto-suggestion** from the free-text description.

**Trust-model extensions:**
- **Rater trust score for confirmers** — separate from reporter reputation, track whether a
  session's *votes* on other people's reports historically matched the eventual verified
  outcome, and weight that session's future votes accordingly. *Source: X Community Notes'
  "Rating Impact" metric.*
- **Trusted-verifier override** — a small set of designated moderators (or a session with a
  strong track record) can immediately mark a report verified, bypassing the confirm-threshold
  wait during a surge right after an event. *Source: Ushahidi's verification workflow.*
- **Anti-brigading burst detection** — detect a disproportionate burst of votes from a small
  cluster of sessions/subnets and silently exclude them from the public tally (without telling
  the voting sessions), freezing borderline cases into a moderation queue rather than letting the
  burst instantly flip status. *Source: Reddit's 2016 anti-vote-manipulation overhaul; research
  showing a coordinated 5-20% minority of raters can manipulate Community Notes.*

**Reaching the right people:**
- **Capability-matched dispatch alerts** — in addition to radius-based push, specifically ping
  opted-in users who tagged a relevant capability (has a boat, medically trained, has a
  generator). *Source: PulsePoint Respond's targeted AED-needed alerts.*
- **Targeted confirm-nudge** — push "can you confirm this?" specifically to sessions already
  engaged in that area recently, not a cold broadcast. *Source: PetaJakarta.org/CogniCity.*
- **Crowd-translation queue** — let any viewer flag a report's free text as needing translation
  and submit an inline translation, distinct from the fixed-UI-string multi-language toggle.
  *Source: Mission 4636 (2010 Haiti earthquake), 40,000+ SMS messages translated by a volunteer
  corps.*
- Multi-language toggle (Hindi + one regional language) for UI/category strings.

**Faster triage:**
- **Rapid swipe-triage queue** — a one-item-at-a-time swipe interface (confirm/dispute/skip) to
  clear a backlog of unconfirmed reports fast during a surge, alongside the filtered list view.
  *Source: Humanitarian OpenStreetMap Team's "Picture Pile" micro-tasking tool.*

**Standards interop (impressive, moderate effort):**
- **CAP-standard action & certainty labels** — map category/severity to CAP's standardized
  `responseType` codes (rescue-needed+critical → "Execute", flooding+high → "Evacuate") and
  translate confirm/dispute tally into CAP's certainty scale (Observed/Likely/Possible/Unlikely).
  Small lookup table on data already collected. *Source: OASIS CAP 1.2 spec.*
- **Authority handoff export** — package a verified report (coords, category, photo, confirm
  count, timestamps) into a formatted summary for forwarding to a municipal control room/NDRF
  contact, plus an on-demand area digest (counts by category/severity, open critical list,
  shelter capacity) exportable as JSON/printable HTML. *Source: the documented Chennai
  2015/Kerala 2018 ad hoc WhatsApp-group-relay pipeline to officials.*
- **Per-recipient response tracking with auto-escalation** — track Confirmed/Acknowledged/
  No-response per alert-radius subscriber instead of fire-and-forget, auto-resend or surface
  non-responder count after a timeout. *Source: Everbridge's closed-loop confirmation.*
- WhatsApp/SMS reporting bot (Twilio or WhatsApp Cloud API) — meets non-smartphone users where
  they are; the strongest real-world version of this product, but its own integration project.
- Route-around-hazard hints on the map.

---

## V2 / future (don't build now — real value, but too much scope for the initial build)

- **Missing/found person registry** — a structurally separate record (name, photo, last-known
  location/time, status, free text) searchable by name/area. *Source: Google Person Finder
  (PFIF), used after Haiti 2010, Japan 2011, Nepal 2015, and Kerala 2018 (~20,900 records).*
  Highest-abuse-surface data type on an anonymous platform (third-party PII) — needs
  moderation-gating, not an open auto-approved table, so it's scoped as v2 rather than core.
- **Per-incident coordination channel** — a lightweight real-time text thread attached to a
  specific critical report/cluster for responders to coordinate tactical detail. *Source: Zello
  push-to-talk channels used by the volunteer "Cajun Navy" during Hurricane Harvey.* Real value,
  but real-time chat is materially more work than the rest of this list.
- **CAP-compliant alert feed with lifecycle threading** — a full `/alerts.cap` XML endpoint with
  proper Update/Cancel lifecycle threading via `<references>`. Real interoperability value is
  speculative without an actual government integration path, but it's a concrete, checkable
  claim most hobby implementations get wrong.
- **Digitally-signed authority reports** — enveloped XML-Signature on CAP `<alert>` elements for
  verified authority accounts, matching how FEMA's IPAWS-OPEN authenticates alerting authorities.
  Only meaningful once the CAP feed exists and there are real authority accounts to hold keys.

---

## Tech plan (Go-centered)
- **Backend:** Go, using a router (`chi` or `gorilla/mux`), built as a JSON API first
  (`/api/reports`, `/api/confirm`, etc.) — the website is the first client of this API, not logic
  baked into page handlers. This means a mobile app later can call the same backend without a
  rewrite.
- **Database:** PostgreSQL with plain latitude/longitude columns — a basic distance calculation
  in Go or SQL is enough to filter "nearby" reports; no need for PostGIS at this scale.
- **Frontend:** Server-rendered HTML from Go (`html/template`) plus plain JavaScript for the
  interactive parts (map, confirm/dispute buttons), calling the JSON API underneath.
- **Map:** Leaflet.js with OpenStreetMap tiles — free, no API key or billing.
- **Location:** browser's built-in geolocation API, to tag a report and to center the
  feed/map on the viewer.
- **Real-time (stretch):** `gorilla/websocket` to push new reports live without a refresh;
  Web Push (VAPID) for background alert-radius notifications.
- **Background jobs:** a Go goroutine poller for the GDACS/official-data feed; auto-expiry sweep.
- **Moderation:** a hosted text-moderation/toxicity API call and an image-safety API call on
  submit (calibrated to a target false-positive rate); perceptual image hashing
  (`github.com/corona10/goimagehash` or similar) for the reshare/duplicate-image flag.
- **Dedup:** a text-embedding API call for near-duplicate report clustering.
- **Hosting:** Railway or Render — cheap/free Go deployment, gives one live URL.

## Website → app path
1. Ship the website first (API-backed, as above).
2. Turn it into a PWA (manifest + service worker) — installable on a phone home screen, works
   offline-ish with Background Sync, same codebase. This is the realistic "app" step for a solo
   2-4 week project.
3. If a true native app is wanted later, the API-first backend means only a thin client needs to
   be built — no backend rewrite.

## Data model (rough, updated)
- `reports`: id, category, severity, latitude, longitude, description, photo_url (optional),
  status (active/resolved/provisional/hidden), source (crowd/official), depth_level (optional,
  flooding only), eyewitness (self-declared bool), receipt_code, responder_session_id (optional,
  claim), responder_eta (optional), contacted_at (optional, human-verification tier),
  contacted_priority (optional), confidence_score, reliability_score, created_at, expires_at
- `confirmations`: report_id, voter identifier (IP-hash or anonymous session id), vote
  (confirm/dispute), geohash_cell (for diversity weighting), created_at
- `sessions` (anonymous, cookie/localStorage-based, no signup): id, reporter_reputation_score,
  rater_trust_score, capability_tags (optional: boat/medical/generator, opt-in)
- `shelters` (or a shelter-status extension on `reports`): capacity_status
  (available/limited/full/closed), headcount (optional)

## Access model
Anonymous posting with basic rate-limiting by IP — requiring signup adds friction exactly when
someone needs to report something fast during an emergency. Lightweight anonymous session
tracking (no signup) is used only for reputation/trust scoring and receipt codes, not identity.
