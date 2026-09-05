# Feature Research

**Domain:** Crowd-verified local emergency information feed (floods/cyclones, India context)
**Researched:** 2026-09-05
**Confidence:** HIGH

This file sanity-checks the feature categorization already produced in `PROJECT.md` /
`PROJECT-NOTES.md` against known behavior of real analog products (Citizen, Waze, Ushahidi,
PulsePoint, Facebook Safety Check, X Community Notes, Wikipedia ClueBot NG/Pending Changes) and
flags dependency/complexity issues in the current Active/Stretch/V2 split. It does not re-derive
the feature list — that research is already done and cited in `PROJECT-NOTES.md`.

## Feature Landscape

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Location-tagged reports | Every analog (Citizen, Waze, Ushahidi) filters by place, not global feed | LOW-MEDIUM | Browser geolocation API + lat/lon columns; no PostGIS needed at this scale |
| Report categories | Users scan by type during a fast-moving event | LOW | Fixed enum, cheap |
| Severity level | Urgent items must surface first or the feed is unusable during a surge | LOW | Sort key on existing fields |
| Map view | Every analog product in this category leads with a map | MEDIUM | Leaflet + OSM tiles, pin clustering at scale |
| Auto-expiry | This product's stated Core Value is literally "the feed reflects what's true now" | LOW-MEDIUM | Background sweep job; do not relabel this as a differentiator — for this specific product it is non-negotiable table stakes, not optional polish |
| Resolved marking (reporter/nearby closes out a report) | **Currently mis-scoped as Stretch.** This is the reporter-driven complement to auto-expiry and is one cheap status transition (`active → resolved`) on data that already exists | LOW | **Recommend promoting to Active.** No new schema, no new infra — it directly strengthens the same Core Value claim auto-expiry exists to serve, at negligible cost |

### Differentiators (Competitive Advantage — align with Core Value)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Confirm/dispute voting ("confirmed by N nearby") | This *is* the Core Value — the trust mechanic other apps in this space don't do well | MEDIUM | Correctly the centerpiece; everything else should be judged by whether it strengthens or dilutes this |
| Provisional visibility gate | Converges with 3 independent real deployments (Ushahidi Kenya 2008, Banpani.org Assam, Wikipedia Pending Changes) on the same fix for single-source rumors | MEDIUM | Depends on categories + severity + confirm/dispute existing first (needs to know category to decide critical-bypass) |
| Diversity-weighted confirm count | Directly defeats vote-stuffing from one location — the cheapest realistic attack on the trust mechanic | MEDIUM-HIGH | **See "Open Design Question" below — this is under-specified in the current Active list and could change how the feature is built, not just when.** |
| Split confidence/reliability decay | Two independently-decaying scores is a real refinement over Waze's single confidence score | MEDIUM-HIGH | **Silently depends on a persistent anonymous session identity existing** (see Dependencies) — reliability score has nothing to accumulate against without it |
| Retraction push + live-resolving status link | Fixes a failure mode (corrections reach ~1/44th the audience of the original claim, per the Boston Marathon misinformation study) that the shareable-card feature itself introduces | Split — see Dependencies | **This is two features bundled as one Active bullet with very different complexity — see recommended split below** |
| "I'm safe" check-in | Facebook Safety Check proved demand for this exact primitive after a disaster | LOW | Cheap: same `reports` table, new category. Correctly cheap enough to be core-adjacent even though not universal table stakes for this app category |
| Needs ↔ offers matching | No major analog does this well; genuine differentiator for coordination, not just information | MEDIUM | Filtering logic only, reuses existing report data |
| Volunteer/rescue-team triage view | Directly modeled on Kerala 2018 volunteer workflow (keralarescue.in) | LOW-MEDIUM | Filtered query, no new data needed beyond severity/category already in scope |
| Shareable verified-report cards | Turns the trust mechanic into the re-share vector that competes directly with unverified WhatsApp forwards — the stated problem this product solves | MEDIUM | Reuses the same "public status page" primitive as the receipt code (see Dependencies) |
| Structured shelter capacity status | Cyclone Fani (~1.5M people, 9,177 shelters in ~24h) is direct evidence a binary "open" pin fails at scale | LOW | One extra field + enum, cheap |
| Anonymous reporter receipt code | Kerala 2018 study: ~25% of ~45,000 logged rescue requests were duplicates — this is the fix | LOW-MEDIUM | Architecturally identical to the "live-resolving status link" needed for shareable cards — build once, reuse for both |
| Responder claim + coverage-gap layer | Documented duplication-of-effort problem in Kerala 2018 and Assam | MEDIUM | Depends on session identity to attribute a claim and prevent one anonymous user unclaiming another's claim, and on the triage view existing |
| Human-contact verification tier | What the most effective real Kerala 2018 volunteers already did by hand; keralarescue.in has a field for this | LOW-MEDIUM | Depends conceptually on responder claim + triage view (same actor doing both) |
| Official open-data feed integration (GDACS) | Solves the empty-map-on-first-visit problem; verified GDACS exposes a free, unauthenticated GeoJSON API (`gdacs.org/gdacsapi/api/Events/geteventlist`) — this is real, not aspirational | MEDIUM | Confirmed via web search: no auth required, GeoJSON format, only requires source attribution. Good core inclusion |
| Demo/replay mode | Solves the "reviewer sees an empty feed" problem for a portfolio project specifically | LOW-MEDIUM | High ROI for the stated portfolio purpose; cheap relative to value |
| Authority badge (for GDACS-sourced pins only) | **Currently deferred as one unit — should be split.** The data model already carries `source: crowd/official` from the GDACS integration (Active), so badging official-sourced pins is nearly free once GDACS ships | LOW (for the GDACS-sourced half) | **Recommend splitting:** badge-for-existing-official-pins is cheap enough to ship alongside GDACS integration in Active; only the "verified authority *account*" flow (a real org submitting reports under its own identity) needs the deferral reasoning currently written |

### Anti-Features (Correctly Deferred or Worth Reconsidering)

| Feature | Why It Seems Good | Why Problematic Now | Current Categorization Assessment |
|---------|--------------------|----------------------|------------------------------------|
| Image-safety check as part of Active auto-moderation | Auto-moderation should obviously cover images, not just text | Photo attachment itself is Stretch — there are no images to check at core launch, so this half of the Active bullet does nothing until a Stretch feature ships | **Miscategorization: split the Active bullet.** Text toxicity/spam filter stays Active; image-safety check moves to Stretch, paired explicitly with photo attachment |
| Background Sync (service worker) as written | "Queue offline, send once connectivity returns" sounds like a solved PWA primitive | Verified via web search: the Background Sync API has **zero support in Safari (including iOS Safari) and zero support in Firefox** as of 2026 — only Chromium-based browsers support it. iOS Safari is a dominant mobile browser in the target (India, general public) context | **Recommend rescoping the requirement, not just noting risk.** Primary implementation should be an IndexedDB queue with automatic replay on next page load/foreground (works everywhere); true Background Sync API registration is a Chrome-only enhancement layered on top, not the baseline mechanism |
| Missing/found person registry | Extremely high real-world value (Google Person Finder precedent: Haiti 2010, Japan 2011, Kerala 2018 ~20,900 records) | Highest abuse-surface data type (third-party PII) on an anonymous, no-signup, auto-approved platform | Correctly V2 — reasoning is sound, don't reconsider |
| Trusted-verifier override | Speeds up verification during a surge | Requires inventing a "moderator" identity, which conflicts with the stated anonymous/no-signup access-model constraint | Correctly Stretch, but flag explicitly: this needs an access-model exception, not just new UI — it's a bigger lift than its list position suggests |

## Feature Dependencies

```
Report categories ──requires──> [nothing, foundational]
Severity level ──requires──> [nothing, foundational]
Confirm/dispute voting ──requires──> Report categories, Severity level

Provisional visibility gate ──requires──> Report categories + Severity + Confirm/dispute
Diversity-weighted confirm count ──requires──> Confirm/dispute voting
                                  ──requires──> Confirmer location capture (UNRESOLVED — see below)

Anonymous session identity (sessions table) ──requires──> [nothing, foundational, but NOT
                                              currently listed as its own Active item]
Split confidence/reliability decay ──requires──> Anonymous session identity
Responder claim + coverage-gap layer ──requires──> Anonymous session identity, Triage view
Human-contact verification tier ──requires──> Responder claim, Triage view
Rater trust score (Stretch) ──requires──> Anonymous session identity, a terminal
                              "verified true/false" outcome (Resolved marking) — this dependency
                              is NOT currently noted in PROJECT-NOTES.md

Anonymous reporter receipt code ──shares-implementation-with──> Shareable card's
                                  "live-resolving status link" (same public status-page primitive;
                                  build once)

Retraction push (proactive notify) ──requires──> Websockets OR Web Push (both Stretch)
                                    ──requires──> report_views table (NOT in current data model
                                    — needed to know which sessions saw a report while live)
Retraction "live-resolving link" (passive, on-visit) ──requires──> Receipt-code status-page
                                                        primitive only (no push infra needed)

Authority badge (GDACS pins) ──requires──> Official open-data feed integration (Active) ──only──
Authority badge (submitted-authority accounts) ──requires──> Verified-authority-account flow (V2)

Auto-moderation confidence-cascade (flag-for-review band) ──requires──> A 5th `status` value or
                                              separate `moderation_flag` column — NOT in current
                                              data model (`active/resolved/provisional/hidden`
                                              has no "flagged, still visible" state)

Photo attachment (Stretch) ──requires──> [nothing new]
Image-safety check ──requires──> Photo attachment (currently miscategorized as Active — see above)
Reshare/duplicate-image flag (Stretch) ──requires──> Photo attachment (already correctly noted)

Per-incident coordination channel (V2) ──requires──> Websockets (Stretch)
CAP-compliant alert feed (V2) ──requires──> [nothing new, but low value without gov't integration]
Digitally-signed authority reports (V2) ──requires──> CAP-compliant feed (V2) + authority accounts (V2)
```

### Dependency Notes — the ones that change scope, not just order

1. **Anonymous session identity is a load-bearing prerequisite that isn't its own Active line
   item.** Reliability score, responder-claim attribution, and (later) rater trust score all
   assume a persistent-but-anonymous session id exists and accumulates history. It's in the data
   model (`sessions` table) but appears in no Active requirement bullet. **Recommend surfacing
   "anonymous session identity (cookie/localStorage id, no signup)" as its own explicit early
   Active requirement**, sequenced before confidence/reliability split and responder claim.

2. **Open design question: how does diversity-weighted confirm count get the confirmer's
   location?** The data model has `geohash_cell` on `confirmations`, but nothing in the Active
   list specifies how a voter's location is captured. Two real options with materially different
   UX and signal quality:
   - Browser geolocation prompt on every confirm/dispute vote — accurate, but adds friction to
     the fastest interaction in the product and is refusable (what happens to a refused vote's
     weight?).
   - IP-derived coarse geohash — silent, no friction, but IP geolocation in India is frequently
     city-level or carrier-NAT-level, which can collapse the diversity signal toward
     near-zero (many "distinct" voters resolving to the same coarse cell).

   This isn't a sequencing question — it changes what "diversity-weighted" actually measures.
   **This should be resolved as a design decision in the same phase that builds confirm/dispute,
   not deferred to later**, since retrofitting a location-capture mechanism onto an existing vote
   flow is more disruptive than deciding it upfront.

3. **"Retraction push" is two features of very different cost bundled into one Active bullet.**
   The live-resolving status link (a shareable card's URL reflects current state when visited) is
   cheap and reuses the receipt-code page primitive — keep this in Active. Proactively pushing a
   correction to everyone who already saw the report requires (a) real-time delivery infra
   (websockets or Web Push, both currently Stretch) and (b) a `report_views` table tracking which
   sessions saw which report while live — neither exists yet. **Recommend: keep the passive
   live-resolving link in Active; move the proactive push notification to Stretch, explicitly
   chained after Web Push/websockets, with `report_views` noted as its schema prerequisite.**

4. **Auto-moderation's "flag for review, still visible" middle band has no home in the current
   status model.** `status` is `active/resolved/provisional/hidden`; there's no state for
   "visible but flagged." Needs either a 5th status value or a separate boolean/enum
   `moderation_flag` column. Also worth noting honestly: the "≤0.1% false-positive budget"
   framing implies empirical calibration against labelled data, which won't exist at portfolio
   scale before launch. The credible version is a documented, reasoned threshold choice with the
   calibration methodology written up as a design decision — that's still strong portfolio
   signal, an actually-calibrated-from-data number is not achievable pre-launch.

5. **Rater trust score (Stretch) depends on a terminal ground-truth state that doesn't fully
   exist yet.** Scoring whether a session's past votes matched "the eventual verified outcome"
   requires some report to reach a definitive true/false end-state. Auto-expiry alone doesn't
   produce that (expired ≠ confirmed false). This is another point in favor of promoting
   "Resolved marking" to Active — it also happens to be the ground-truth signal this Stretch
   feature will eventually need.

## MVP Definition

### Launch With (v1 / Active, as corrected)

- [ ] Location-tagged reports, categories, severity, map view — foundational, no dependencies
- [ ] Anonymous session identity — currently missing as an explicit item; add it, it's a
      prerequisite for several other Active items
- [ ] Confirm/dispute voting — the Core Value
- [ ] Auto-expiry + **Resolved marking (promote from Stretch)** — both cheap, both directly serve
      the "feed reflects what's true now" claim
- [ ] Provisional visibility gate, diversity-weighted confirm count (with the location-capture
      method decided explicitly), split confidence/reliability decay — trust-model hardening,
      correctly Active
- [ ] Retraction: live-resolving status link only (proactive push moved to Stretch — see above)
- [ ] "I'm safe" check-in, needs/offers matching, triage view, shareable cards — closing the loop
- [ ] Shelter capacity status, receipt code, responder claim, human-contact tier — coordination
- [ ] Low-bandwidth fallback view
- [ ] Offline queue via IndexedDB + replay-on-load (rescoped from "Background Sync API" — see
      above; treat true Background Sync registration as a Chrome-only enhancement, not the
      baseline)
- [ ] Auto-moderation: **text-only** toxicity/spam filter (image-safety check moved to Stretch
      with photo attachment); needs a "flagged for review" state added to the status model
- [ ] GDACS official feed integration, **plus authority badge for GDACS-sourced pins** (cheap
      once the feed exists — promote this half out of the deferred "authority badge" item)
- [ ] Demo/replay mode, disclaimer, OpenAPI docs, CI

### Add After Validation (Stretch, as corrected)

- [ ] Photo attachment, paired with its now-correctly-scoped image-safety check
- [ ] Live updates (websockets) and Web Push/alert radius — unlocks proactive retraction push,
      capability-matched alerts, targeted confirm-nudge, per-recipient response tracking (all
      already correctly chained after this in PROJECT-NOTES.md)
- [ ] Authority-account flow (the verified-authority-*account* half of the badge feature, distinct
      from GDACS-pin badging which is now Active)
- [ ] Water-depth field/overlay, duplicate clustering, reshare/duplicate-image flag, eyewitness
      tag, auto-suggestion, rater trust score, trusted-verifier override, anti-brigading
      detection, translation queue, multi-language toggle, swipe-triage, CAP labels, authority
      export, WhatsApp/SMS bot, routing hints — categorization here is sound, no changes
      recommended

### Future Consideration (V2, unchanged)

- [ ] Missing/found person registry, per-incident coordination channel, CAP-compliant alert feed,
      digitally-signed authority reports — reasoning in PROJECT-NOTES.md is sound, no changes
      recommended

## Sources

- Existing project research: `PROJECT-NOTES.md` (Citizen, Waze, Ushahidi, PulsePoint, Google
  Crisis Response, Facebook Safety Check; Kerala 2018/Chennai 2015/Cyclone Fani/Assam case
  studies; X Community Notes, Waze confidence/reliability, Wikipedia Pending Changes/ClueBot NG;
  OASIS CAP 1.2)
- Web search (2026-09-05): Background Sync API browser support — confirmed zero support in
  Safari (including iOS) and Firefox as of 2026; Chromium-only
  ([caniuse.com/background-sync](https://caniuse.com/background-sync),
  [testmuai.com background-sync-browser-support](https://www.testmuai.com/learning-hub/background-sync-browser-support/))
- Web search (2026-09-05): GDACS API confirmed free, unauthenticated, GeoJSON-format, source
  attribution only requirement
  ([gdacs.org API quickstart](https://www.gdacs.org/Documents/2025/GDACS_API_quickstart_v1.pdf))

---
*Feature research for: crowd-verified local emergency information feed*
*Researched: 2026-09-05*
