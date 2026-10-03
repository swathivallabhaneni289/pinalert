# Roadmap: Pinalert

## Overview

Pinalert started as six vertical slices (extended since, see the last paragraph below), each one something a real visitor can try end-to-end.
Phase 1 gets the base reporting loop live (submit, view, expire) with the anonymous session
identity, API docs, and CI in place from day one, since retrofitting the schema/session model
later is expensive. Phase 2 builds the Core Value itself — confirm/dispute voting resolved by a
single, concurrency-safe `VisibilityResolver` — as its own complete slice, gated by an
independence predicate before any weighting refinement is layered on. Phase 3 hardens that same
trust display against trivial gaming (diversity-weighted counts, split confidence/reliability
signals) as a dedicated slice, because the geohash-precision and confirmer-location-capture
decisions it resolves are genuinely load-bearing, not incidental polish. Phase 4 makes the app
survive real conditions — degraded networks, abuse pressure, legal exposure. Phase 5 solves the
empty-map-on-first-visit problem with official alerts (NDMA SACHET, in an Alerts view and banner), GDACS pins on the map and a working demo. Phase 6 closes the
information loop with the coordination features (safety check-ins, needs/offers, triage,
responder claims, shareable cards) that depend on the trust signals built in Phases 2-3.

Added 2026-10-03, after the Phase 3 popup sketch: Phase 3.1 (report history views) and Phase 3.2
(in-app updates and trusted-reporter alerts) sit right after Phase 3 because they consume its trust
object, outcome labels and reliability tag. Phase 5 now leads with NDMA SACHET alerts. Phase 8
(report page and comments) runs before Phase 6 because Phase 6's receipts and shareable cards link to its
page, and Phase 9 (followable reporter profiles) comes last. Suggested cut order if time runs short, from the
2026-10-03 independent reviews: Phase 9 first, then the Reliable-reporter filter and alerts (NOTIF-04 and
NOTIF-05) from Phase 3.2, then the comments half of Phase 8, then the GDACS pins in Phase 5. Email digests
stay in the backlog.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation — Report & Map** - Anonymous visitors can submit and view location-tagged reports on a live map, with read-time auto-expiry, OpenAPI docs, and CI in place. (original 10 plans completed 2026-09-06; reopened for the vector basemap migration, plans 01-11/01-12; gap-closure round 2 (01-13) fixed the solid-black-icon defect; gap-closure round 3 (01-14) landed 2026-09-09 — raised glyph ink coverage and fixed badge contrast across all 18 severity/age/theme pairings; gap-closure round 4 (01-15) landed 2026-09-09 — live-tested fix for dark-mode category-tile glyph color, the actual root cause 01-14's stroke-width diagnosis missed, plus two caching-bug fixes; see `01-UAT.md`) (all 15 plans complete; UAT passed 10/10 with 0 open issues; verification passed including MVP-mode User Flow Coverage, security SECURED with 0 open threats, Nyquist validation compliant) (completed 2026-09-10) — **note:** this phase's anonymous/no-signup access model (FOUND-01) was superseded 2026-09-10 by Phase 1.1's mandatory login decision; the shipped code is unaffected, but the product's access model changes starting Phase 1.1
- [x] **Phase 1.1: Identity & Login — Mandatory Email Verification** *(INSERTED 2026-09-10 — urgent insertion, decided during Phase 2 discussion)* - A visitor must verify an email address via a magic link before they can submit a report or cast a confirm/dispute vote; supersedes Phase 1's anonymous-access model. (completed 2026-09-12)
- [x] **Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility** - Users can confirm/dispute reports through one shared, concurrency-safe visibility resolver with a provisional gate and resolved marking. (original 8 plans completed 2026-09-15; **reopened 2026-09-17** for gap-closure round 1 — a human UAT walkthrough found 6 gaps, 3 major and 3 minor, all diagnosed in `02-UAT.md`: the inline resolve confirmation never replaced the button row, its prompt was illegible in the map popup, a Safari Back navigation showed a stale feed after Reopen, the Activity page had no way back to the map, plus two user-requested scope changes — a persisting "Show disputed" filter and an in-app light/dark toggle. Plans 02-08, 02-09 and 02-10, all wave 8, mutually parallel, executed and merged 2026-09-18) — **all 11 plans complete, but re-verification (02-VERIFICATION.md) returned `human_needed`: 8 rendered-browser behaviors still require a live UAT pass (in progress as a second `02-UAT.md` round, tests 4-11) before this phase can be marked complete** (completed 2026-09-23)
- [ ] **Phase 3: Trust-Model Hardening — Diversity-Weighted Trust** - "Confirmed by N nearby" and the reliability/currency signals reflect distinct nearby corroboration, resistant to trivial gaming.
- [ ] **Phase 3.1: Report history views** *(INSERTED 2026-10-03)* - A reader can switch the list and map between Active, Resolved, Disputed and Past reports, search another area, and filter by category and severity.
- [ ] **Phase 3.2: In-app updates and trusted-reporter alerts** *(INSERTED 2026-10-03)* - A signed-in person is told inside the app when a report they care about changes, and can ask to hear when a Reliable reporter posts nearby.
- [ ] **Phase 4: Robustness — Real-World Resilience** - The app stays usable on degraded networks, under abuse/moderation pressure, and with clear legal footing.
- [ ] **Phase 5: Official Feed & Demo Mode** - First-time visitors see official alerts (NDMA SACHET, including IMD and CWC) in an Alerts view and banner, GDACS pins on the map, and a working demo immediately, instead of an empty product.
- [ ] **Phase 6: Coordination — Safety, Needs, and Response** - People can check in safe, exchange needs/offers, and responders can triage, claim, and verify reports without duplicating effort.
- [x] **Phase 7: Address search box for report location** - A reporter can type an address to place the report pin, alongside the draggable pin. (4 of 4 plans complete, UAT 6 of 6 passed, security review closed) (completed 2026-09-29)
- [ ] **Phase 8: Report page and comments** - A reader can open any report into its own page with the trust block and a history timeline, and later read and add moderated comments.
- [ ] **Phase 9: Followable reporter profiles** - Reporters who opt in and have earned the Reliable tag can be followed privately, and followers are told when they post. First to cut if time runs short.

## Phase Details

### Phase 1: Foundation — Report & Map

**Goal**: As a visitor, I want to submit a location-tagged emergency report and see it alongside other nearby reports on a live map, so that I can report and track emergencies without needing to create an account.
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

### Phase 1.1: Identity & Login — Mandatory Email Verification

*(INSERTED 2026-09-10.)* Decided mid-way through discussing Phase 2: the user raised a real
concern that fully anonymous voting is gameable (multiple devices/sessions from one person), and
after weighing it against the independence predicate (distinct session + distinct geohash cell)
already planned for Phase 2, decided the accountability gap was worth closing with mandatory
login rather than relying on location-diversity alone. Phone OTP was ruled out on cost grounds
(no free tier at any real volume; the cheap route additionally requires India DLT sender
registration) — see Key Decisions in PROJECT.md for the full tradeoff discussion. This phase must
land before Phase 2, because Phase 2's vote/independence design is now built against verified
accounts, not anonymous sessions.

**Goal**: As a visitor, I want to verify my email with a magic link before I can report or vote, so that every report and vote is tied to an accountable, verified identity.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: IDENT-01, IDENT-02, IDENT-03, IDENT-04, IDENT-05
**Success Criteria** (what must be TRUE):

  1. A visitor cannot submit a report or cast a confirm/dispute vote without first verifying an
     email address by clicking a magic link sent to it.

  2. Verification-email delivery works through a free-tier transactional email provider (e.g.
     Resend), within its free quota — no paid SMS or phone verification path exists in this phase.

  3. A verified user can view a profile page listing their own submitted reports and their
     voting/confirm-dispute activity.

  4. Verification-email requests are rate-limited per email address and per IP (30-60 second
     resend cooldown), so the email-sending endpoint can't be trivially abused for spam or cost
     inflation.

  5. The Phase 1 session-cookie mechanism continues to carry identity underneath the new
     verified-account gate — Phase 2's votes/reports still key off `session_id` as designed, just
     now backed by a verified account rather than an anonymous one.

**Decided in `01.1-CONTEXT.md`** (discussed 2026-09-10): magic link over a typed code; link
expires after 5 minutes; login is required to view anything, not just to act; the landing screen
shows a continuous rotating-globe background with the login form already interactive on top;
sessions are long-lived (reuses Phase 1's existing 1-year cookie), support simultaneous
multi-device login, and have an explicit logout; no account-recovery mechanism exists for v1 —
losing email access means verifying a new one and starting fresh.

**Plans**: 8/8 plans complete
Plans:
**Wave 1**

- [x] 01.1-01-PLAN.md — Request a magic link: identity schema, token primitives, mailer seam, login gate screen (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01.1-02-PLAN.md — Click the link and get in: atomic single-use consumption, account/session binding, five outcome screens (wave 2)
- [x] 01.1-03-PLAN.md — Real Resend delivery, fail-fast mailer loader, and the pre-launch domain-verification note (wave 2)

**Wave 3** *(blocked on 01.1-02)*

- [x] 01.1-04-PLAN.md — The gate closes: requireVerifiedAccount middleware, route regrouping, API reference realigned (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01.1-05-PLAN.md — Abuse resistance: per-email cooldown, per-IP token bucket, live resend countdown (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01.1-06-PLAN.md — Shared header partial rendered on every gated page: account menu with verified address, Activity item, immediate logout (D-12) (wave 5)

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01.1-07-PLAN.md — Profile page under that shared header, with one merged Activity section listing the account's reports across devices, plus the logout endpoint (wave 6)

**Wave 7** *(gap closure — blocked on Wave 6 completion)*

- [x] 01.1-08-PLAN.md — Gap closure for SC4/IDENT-04: un-forgeable per-IP limiter key (drops chi's deprecated header-trusting middleware) and an atomic `email_cooldowns` claim replacing the TOCTOU-racy per-address cooldown, both proven by tests that fail against the pre-fix tree (wave 7)

**UI hint**: yes

### Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility

**Goal**: A user can confirm or dispute a report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed by one shared, concurrency-safe resolver everywhere it's shown.
**Mode:** mvp
**Depends on**: Phase 1.1
**Requirements**: TRUST-01, TRUST-02, TRUST-03, TRUST-04, TRUST-06, TRUST-08, TRUST-09
**Success Criteria** (what must be TRUE):

  1. A user can confirm or dispute another user's report, and concurrent votes submitted on the same report at the same time never silently lose an update, verified by an automated concurrency test.
  2. A newly submitted non-critical report displays as "provisional" until a second independent confirmation arrives; a critical/rescue-needed report publishes at full visibility immediately, with no gate.
  3. Only a vote from a distinct verified account AND a distinct geohash cell counts toward that independent confirmation — a report's self-declared severity affects triage sort order only and can never by itself unlock full visibility.
  4. A report's visibility state (Hidden/Provisional/Live/Retracted) is identical everywhere it's shown — feed, map, triage view, and shareable card — because one shared resolver function computes it.
  5. A user (the reporter or a nearby confirmer) can mark a report resolved, removing it from the live feed.

**Plans**: 15/15 plans complete

**Wave 1** *(parallel — zero file overlap, neither depends on the other)*

- [x] 02-01-PLAN.md — The pure `Resolve()` resolver: Hidden/Provisional/Live/Retracted decided in exactly one place, critical bypass over both gates, Hidden fully reversible from live tallies, retract/reopen never latching (D-05..D-09, D-14, D-16) (wave 1)
- [x] 02-02-PLAN.md — Append-only vote log with no shared mutable counter to race on: N concurrent casts from N accounts all land, latest row per (report, account, kind) wins, content and resolution votes never interfere (TRUST-09, D-02) (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-03a-PLAN.md — `VotingService.CastVote`: voter geohash cell computed server-side from raw lat/lon, self-vote block scoped to content votes only, expired-report rejection, one shared independence helper (D-03, D-13, D-17) (wave 2)

**Wave 3** *(blocked on 02-03a)*

- [x] 02-03b-PLAN.md — The four vote routes mounted inside the existing verified-account gate from one `handlers.CastVote` factory, with the OpenAPI spec regenerated so OPS-01 stays truthful (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-04-PLAN.md — Feed and map return the identical resolver-computed visibility: Retracted never appears, Hidden only under `?show_disputed=true`, plus `your_vote` and `is_own_report` for the client (D-03, D-08, D-10, D-11, D-12) (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 02-05-PLAN.md — First user-clickable increment: Confirm/Dispute on the feed row and the map pin popup from one shared builder, GPS captured once per session with a hard block on denial, nothing rendered until the server answers (D-01, D-02, D-04, D-17, D-18) (wave 5)

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 02-06-PLAN.md — Trust state made legible: Provisional rows and pins dimmed *and* labelled "Unconfirmed", one "Show disputed reports" toggle revealing outlined rows *and* pins from one shared query param, and an empty result that explains itself (D-09, D-10, D-11) (wave 6)

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 02-07-PLAN.md — A report can be taken out of the live feed and put back: "Mark resolved" with an inline destructive confirmation on both surfaces and outcome copy driven by the server's own answer, plus the Activity page's real trust state and "Reopen · not actually resolved" (D-12, D-13, D-15, D-16) (wave 7)

**Gap closure (round 1)** *(from `02-UAT.md`'s six `status: failed` gaps, diagnosed 2026-09-17; the three plans share zero files and run in parallel. Run via `/gsd-execute-phase 02 --gaps-only`)*

- [x] 02-08-PLAN.md — The inline resolve confirmation actually replaces the button row on both surfaces (the base vote-button rule was the one `display` rule in `trust.css` missing its `:not([hidden])` guard), and the Leaflet popup gets a theme-aware surface so its prompt is legible — the popup had been a white box in dark mode since Phase 1 (wave 8)
- [x] 02-09-PLAN.md — The feed refetches when Safari restores it from the back-forward cache, `GET /api/reports` declares itself uncacheable, and the "Show disputed reports" filter is carried in the page URL so a reload keeps the view (a user-requested reversal of the spec's deliberate no-persistence scoping, amended on `02-UI-SPEC.md`) (wave 8)
- [x] 02-10-PLAN.md — The Activity page gets a "Back to map" link, and a three-state System/Light/Dark control in the account menu writes the root `data-theme` attribute Phase 1's `main.css` was already built to read, applied before first paint on every full page including the logged-out ones (wave 8)

**Gap closure (round 2)** *(from `02-UAT.md`'s six `status: failed` gaps, diagnosed live 2026-09-22; the four plans share zero files and run in parallel. Run via `/gsd-execute-phase 02 --gaps-only`)*

- [x] 02-11-PLAN.md — Both map surfaces (the primary map and the report modal's pin-drop map) resolve their OpenFreeMap basemap style from the live theme and re-style it on a toggle via a root-attribute observer, leaving `theme.js` byte-identical; the WebGL-less raster fallback stays light in every theme as an explicitly accepted limitation (wave 9)
- [x] 02-12-PLAN.md — "Show disputed reports" becomes a true partition rather than an additive reveal: `ListableInFeed` returns Live and Provisional only when the flag is off and Hidden only when it is on, with the two test suites, the research diagram and the regenerated OpenAPI spec brought onto the exclusive contract (**resolves backlog Phase 999.2**) (wave 9)
- [x] 02-13-PLAN.md — Provisional dimming strengthened from the intermediate age-ramp mix to 80 percent toward stale with a neutral row wash, behind a new perceptual-distance gate written to fail first against the shipped value, plus the defensive hidden-attribute guard on the Mark Resolved rule (cause still unconfirmed) (wave 9)
- [x] 02-14-PLAN.md — The two mechanical `site-design-rules.md` violations: ten user-visible long dashes rewritten in plain punctuation across nine files (including a third copy of the rate-limit message the debug sweep missed), and the two pill-shaped controls de-pilled, behind a standing gate. The broad polish items are deferred to `/gsd-ui-phase` (wave 9)

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

### Phase 03.1: Report history views (INSERTED)

**Goal:** A reader can switch the list and the map between Active, Resolved, Disputed and Past reports, so resolved and expired reports stay visible as clearly labelled history instead of vanishing, and can check another area or narrow the list by category and severity. Raised by the owner on 2026-10-03 while looking at the main screen, which today offers only the live list and a "Show disputed reports" checkbox. Inserted right after Phase 3 because it consumes Phase 3's outcome labels and trust object, not because it is urgent.
**Requirements**: HIST-01, HIST-02, HIST-03, HIST-04, HIST-05, HIST-06
**Depends on:** Phase 3, Phase 7
**UI hint**: yes
**Success Criteria** (what must be TRUE):

  1. The list panel has one View selector (Active, Resolved, Disputed, Past) in place of the disputed checkbox, the map shows the same set as the list, and Active is the default.
  2. The four views are a disjoint partition: Active (Live and Provisional), Disputed (Hidden), Resolved (closed by people, within the same bounded window as Past) and Past (expired, in the last 24 hours, 3 days or 7 days). A report closed and then expired stays in Resolved, and a hidden report that expires moves to Past. Resolved rows say who closed them (the reporter alone or independent confirmers) and show the Phase 3 counters, so self-resolving cannot hide a dispute. Past rows carry an outcome label, with a neutral "Expired" for critical and rescue-needed reports that expire unconfirmed. Voting stays in Active and Disputed, so a Hidden report can still be confirmed back, and is off in Resolved and Past.
  3. When no active reports exist, the panel says what happened recently using real counts only, and links to the Resolved view.
  4. A reader can search for a place on the main map and see reports for that area, and can filter the list by category and by severity.
  5. Every view takes its states from the single visibility resolver, and no history view ever changes an Active count.

**Scope notes** (starting points for discuss-phase, not decisions):

  1. The View selector sits at the top of the list panel, where the checkbox is now. The `show_disputed` flag becomes a single `view` setting, and the map follows the list.
  2. New read queries sit beside the pinned `NearbyReports` query, which must not change. Outcome labels reuse Phase 3's outcome rules, so history cannot disagree with the reporter rating.
  3. "Check another area" reuses Phase 7's address search on the main map. The category and severity filters are plain dropdowns, separate from Phase 6's volunteer triage view.
  4. Care points: past rescue-needed reports show a coarse location, past reports are never mixed into Active or its counts, keep the window short (7 days), and show no invented numbers.
  5. D-12 amendment to record in discuss-phase: Retracted reports now appear in the separate Resolved view (never in Active or its counts), where D-12 had them leave the live feed and map entirely. Rescue-needed rows, and ideally all history rows, are coarsened server side before they leave the API.
  6. One panel state model: this phase owns the list panel's state (view, tab and filters kept in the URL) so Phases 3.2, 5 and 9 plug their tabs and filters into it, and deep links such as /?view=resolved&report=ID work. The default tab is called Nearby once other tabs exist. Phase 3's outcome classifier must be exposed as a reusable pure function that can return the neutral outcome. HIST-05 reuses Phase 7's geocoder, which has a process-wide limit of 1 Nominatim request per second, so reader searches need their own budget (or a cache) so they cannot starve reporters' address search.

**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 03.1 to break down)

### Phase 03.2: In-app updates and trusted-reporter alerts (INSERTED)

**Goal:** A signed-in person is told inside the app when something changes on a report they care about, and can ask to hear when a Reliable reporter posts nearby, so they know the current situation without reopening the app and hunting for it. Raised by the owner on 2026-10-03, and earlier in Phase 1.1 as "notify a reporter when someone confirms or disputes their report". Inserted right after Phase 3 because the updates are worded from Phase 3's trust object and the alerts use Phase 3's public reliability tag.
**Requirements**: NOTIF-01, NOTIF-02, NOTIF-03, NOTIF-04, NOTIF-05
**Depends on:** Phase 3, Phase 3.1
**UI hint**: yes
**Success Criteria** (what must be TRUE):

  1. A bell next to the account icon shows an unread count that rides on the existing 30 second refresh, and opens an Updates list where tapping an update opens that report in the view it now belongs to (a deep link such as /?view=resolved&report=ID, using Phase 3.1's views).
  2. A user follows a report automatically when they posted it or voted on it, and can mute any report.
  3. Updates about a followed report are sent for state changes only (the report reaches Live, is hidden by disputes, is resolved, or is within 1 hour of expiring), at most one per report per hour, in anonymous wording built from the same server trust object as the popup, never naming a voter, an account or a location. Every decision calls the single visibility resolver.
  4. A reader can filter the feed to Reliable reporters only (critical and rescue-needed reports always stay visible), and can opt in to an update when a Reliable reporter posts within a chosen distance of where they are looking. Non-critical reports notify only once they are Live, and critical and rescue-needed reports notify immediately.

**Scope notes** (starting points for discuss-phase, not decisions):

  1. Comment events plug in when Phase 8 ships (commenting will make the user follow the report). Email digests and Web Push are separate later work (see the backlog), and Web Push needs a PWA phase first.
  2. Care points: notification volume can be used to harass a reporter (dispute spam), so batch and cap per recipient. A notification must never bypass the provisional gate, which is why non-critical reports notify only once Live. The Reliable tag now grants reach and is farmable (Phase 3 D-03 prices gaming at extra verified accounts plus a claimed location), so cap how many alerts one reporter can trigger per recipient per hour, and treat burst detection (TRUSTX-03) as a dependency before the alert rule is widened beyond the in-app bell.
  3. Anchors and clocks: the server stores no user location, so alert distance is measured from the reader's current map view or GPS fix when the bell refreshes, and alerts arrive only while the app is open (a saved location is deferred with PWA-03). The host sleeps, so time based events ("within 1 hour of expiring") are evaluated lazily on the bell refresh, not by a scheduler. Events come from the append-only vote log and report timestamps, so nothing latches (Phase 2 D-08).
  4. Data: an updates table with read state, a mutes table, and hourly cap bookkeeping. Implicit followers are derived from existing tables (the reporter through sessions, voters through votes). On a small deployment most reporters show New reporter (D-13), so the Reliable-only filter and alerts may run on an empty set until Phase 5's demo accounts exist. If the phase runs long, split NOTIF-04 and NOTIF-05 into a later phase.
  5. Settings live in the account menu: in-app always on, email and push optional later. The Reliable-reporter alerts use the public tag only and expose no identity. Phase 3 D-14 keeps the tag display only inside Phase 3; here the filter and the alert rule are opt-in user choices and must never hide or reorder a report by default.

**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 03.2 to break down)

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

**Goal**: A first-time visitor lands on a populated, working map immediately, with real official alerts and hazard pins plus a working demo, instead of an empty product.
**Mode:** mvp
**Depends on**: Phase 1, Phase 2, Phase 3, Phase 3.1
**Requirements**: ROBUST-05, ROBUST-06, ROBUST-07, ALERT-01, ALERT-02, ALERT-03, ALERT-04
**Success Criteria** (what must be TRUE):

  1. The map view shows official pins ingested from the GDACS open-data feed alongside crowd reports, and an Alerts view lists the official alerts that cover the reader's district, ingested from NDMA's SACHET feed (IMD, CWC and state authority alerts). Official items are unaffected by crowd confirm/dispute votes.
  2. Official items carry a visibly distinct authority badge, so they're never confused with crowd reports or demo-mode reports, and each shows its issuer, when it was issued and when it expires. Expired alerts disappear.
  3. A banner appears at the top of the list only when a Severe or Extreme official alert covers the reader's area, and the app shows when each official feed was last checked and says plainly when one is unavailable, so a stale alert never looks current.
  4. A first-time visitor sees a seeded past-event timeline populating the feed and map immediately, and can press "simulate a report" to try the confirm/dispute flow themselves.

**Scope notes** (added 2026-10-03 after a feasibility check, starting points for discuss-phase):

  1. SACHET first, GDACS second. Checked 2026-10-03: SACHET's India-wide RSS (`sachet.ndma.gov.in/cap_public_website/rss/rss_india.xml`) answered without a key, declares itself public domain, links each item to a CAP 1.2 message, and carried 99 alerts that day. IMD's own APIs need a key or an allow-listed IP, and SACHET already carries IMD and CWC alerts, so the IMD APIs are not used.
  2. The spike is a gate with three written outputs before planning: (a) the scheduler, because the free host sleeps and cold-starts (a lazy refresh on request with a stored last-checked time, or an external cron calling a protected endpoint), (b) the boundary data: some alerts carry a polygon and some only area names (for example mandals), so name-only alerts need a district and mandal gazetteer, with a fallback for alerts that cannot be placed (list the reader's state-level alerts, never drop them silently, especially Extreme ones), and (c) the GDACS storage decision (the reports table has a required session id and no source column). Also settle the fetch model (does the RSS item carry severity, expiry, issuer and area, or does each alert need its own CAP fetch), CAP Update and Cancel handling, and a fallback lifetime when expires is missing. Alerts can arrive in regional languages with an English headline. Confirm NDMA's terms before wide promotion.
  3. An alert covers an area and a validity window, not a point, so it gets its own table and not the `reports` table. The feed sends no CORS headers, so the Go poller fetches it server side. Point in polygon is done in Go (no PostGIS). The poller must make failure visible through the "last checked" line.
  4. Read only: no confirm or dispute controls, never mixed into crowd counts. The Alerts tab and the banner sit in the list panel beside Nearby. Shading the affected areas on the map comes later.
  5. Optional later: an Open-Meteo rain outlook line (free for non-commercial use, credit required). The "replay the last 24 hours" time slider is in the backlog and would extend this phase's demo.
  6. Demo mode must be decided here: login is required to view anything (Phase 1.1 D-05), so a first-time visitor and a recruiter see the login screen, not the demo. A likely answer is a separately entered, read-only or sandboxed /demo route exempt from the login gate (an amendment to D-05 plus an update to the pinned exempt-route test), with fixtures kept out of the live tables and exempt from the 1 km and self-vote rules by construction, so seeded rows never leak into real counts or reporter tags. Resend domain verification is a prerequisite of the first public share.

**Plans**: TBD
**UI hint**: yes

### Phase 6: Coordination — Safety, Needs, and Response

**Goal**: Beyond raw reporting, people can find help, offer help, and responders can act on the most urgent, best-corroborated reports without duplicating effort or losing accountability.
**Mode:** mvp
**Depends on**: Phase 1, Phase 2, Phase 3, Phase 8 (receipts and shareable cards link to the report page)
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
Phases execute in numeric order: 1 → 1.1 → 2 → 3 → 3.1 → 3.2 → 4 → 5 → 8 → 6 → 9 (Phase 7 shipped early; Phase 8 runs before Phase 6 because Phase 6's receipts and shareable cards link to the report page; Phase 9 is the first to cut)

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation — Report & Map | 15/15 | Complete    | 2026-09-10 |
| 1.1 Identity & Login — Mandatory Email Verification | 8/8 | Complete   | 2026-09-12 |
| 2. Trust Mechanic Core — Confirm/Dispute & Visibility | 15/15 | Complete   | 2026-09-23 |
| 3. Trust-Model Hardening — Diversity-Weighted Trust | 0/TBD | Not started | - |
| 3.1 Report history views | 0/TBD | Not started | - |
| 3.2 In-app updates and trusted-reporter alerts | 0/TBD | Not started | - |
| 4. Robustness — Real-World Resilience | 0/TBD | Not started | - |
| 5. Official Feed & Demo Mode | 0/TBD | Not started | - |
| 6. Coordination — Safety, Needs, and Response | 0/TBD | Not started | - |
| 7. Address search box for report location | 4/4 | Complete | 2026-09-29 |
| 8. Report page and comments | 0/TBD | Not started | - |
| 9. Followable reporter profiles | 0/TBD | Not started | - |

## Backlog

### Phase 999.1: Address search box for report location (PROMOTED to Phase 7, 2026-09-23)

**Status:** Promoted to Phase 7 after the user called it "one of the most important things" and
asked for it to be planned during Phase 2 UAT round 3 continuation. Kept here rather than deleted
so a reader of the backlog's history sees where it went.

**Goal:** [Captured for future planning] Add a text/address search box for setting a report's
location, using OSM Nominatim geocoding (free, no API key) to jump the map/pin to the typed
address — kept alongside the existing draggable pin for fine-tuning, not replacing it. Surfaced
during Phase 2 UAT re-verification (2026-09-18): the current submission flow is 100% map-interaction
(GPS auto-fill, drag, tap-to-place), with no way to type an address.
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.2: "Show disputed reports" as an exclusive filter (RESOLVED, promoted 2026-09-22)

**Status:** Promoted into Phase 2 gap-closure round 2 as plan `02-12-PLAN.md`, after the user raised
the same objection a second time during UAT round 3 (2026-09-21/22). Do not promote again. Kept here
rather than deleted so a reader of the backlog's history sees where it went.

**Goal:** [Captured for future planning] Change "Show disputed reports" from an additive reveal
(adds Hidden reports on top of the normal feed, per D-10) to an exclusive filter that shows ONLY
actually-disputed reports when checked. Surfaced during Phase 2 UAT re-verification (2026-09-18):
user found it confusing that a fresh, zero-vote Provisional report (which is visible in the
default feed regardless of the toggle) also appears when the toggle is checked, since nothing
distinguishes "here because you asked for disputed" from "here anyway." Not a bug — current
behavior matches documented D-10 — but a real UX objection to that design.
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.3: PWA shell and Web Push for updates and official alerts (BACKLOG)

**Goal:** [Captured for future planning] Make the app installable (service worker, manifest, install to Home Screen) and deliver Phase 3.2's updates and Severe official alerts for the reader's district as Web Push (VAPID keys, `webpush-go`, stored subscriptions that are removed when an endpoint answers 410 Gone). Ask for permission right after a user posts a report, not on first visit. iPhones only receive web push in an installed app (iOS 16.4 and later), so the install step comes first. Raised by the owner on 2026-10-03; relates to the deferred PWA-03, PWA-04 and PWA-06. Severe or Extreme alerts only. Push text is built from the trust object at send time and links to the live report, so it never carries a count that can go stale.
**Requirements:** PWA-06 (v2)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.4: Email digests for updates (BACKLOG)

**Goal:** [Captured for future planning] Opt-in digests of Phase 3.2's updates (for example "3 updates on your reports") sent through Resend. Blocked until a sending domain is DNS verified, because the sandbox sender reaches only the account owner, and the free tier is about 100 sends a day, so batch rather than send one email per event. Magic links already spend that quota, so a digest run must never starve logins. Digest text is built from the trust object at send time and links to the live report. Raised by the owner on 2026-10-03.
**Requirements:** PWA-07 (v2)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.5: Replay the last 24 hours (BACKLOG)

**Goal:** [Captured for future planning] A time slider that rebuilds the map as it looked at any past moment. A report's state is a pure function of its votes (Phase 2 D-08) and the vote log keeps every vote with its timestamp, so no new storage is needed. It strengthens Phase 5's demo mode and needs the history queries from Phase 3.1. Raised by the owner's discussion on 2026-10-03.
**Requirements:** HIST-07 (v2)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 7: Address search box for report location

**Goal:** Add a text/address search box for setting a report's location, using OSM Nominatim
geocoding (free, no API key) to jump the map/pin to the typed address, kept alongside the
existing draggable pin for fine-tuning, not replacing it. Surfaced during Phase 2 UAT
re-verification (2026-09-18) and repeated by the user during Phase 2 UAT round 3 continuation
(2026-09-23, "we don't know if it's where we're going... it's like randomly picking up a place
depending on the location where I'm in, but we need to have that description for the location
also mentioned properly when the person is making a report"): the current submission flow is
100% map-interaction (GPS auto-fill, drag, tap-to-place), with no way to type an address.
**Requirements**: TBD (promoted backlog item, not tied to a REQUIREMENTS.md id; the traceable unit
is 07-CONTEXT.md's decisions D-01 through D-04, carried in each plan's `requirements` field)
**Depends on:** Phase 1 (the report-submission modal and its map already exist there; this phase
only adds a search input alongside them, it does not need Phase 6's coordination features)
**Plans:** 4/4 plans complete

Plans:
**Wave 1**

- [x] 07-01-PLAN.md — internal/geocode rate-limited Nominatim proxy client plus the GET /api/geocode handler (wave 1)
- [x] 07-02-PLAN.md — search box markup and styling in the report modal, with hidden-guard and stacking contract tests (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-03-PLAN.md — route registration inside the verified-account gate, server wiring, and the regenerated API spec (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 07-04-PLAN.md — debounced client-side search wiring, suggestion-to-pin placement, and the teardown and sink contract tests (wave 3)

### Phase 8: Report page and comments

**Goal:** A reader can open any report from the map popup or the feed into its own page, see the same trust block, and see how the report's trust changed over time. Once Phase 4's moderation and rate limiting exist, they can also read and add short comments, so the discussion about a situation stays inside the verified feed. Raised by the owner during Phase 1.1 and again after the Phase 3 popup sketch, and was deferred in PROJECT.md until moderation and free text abuse were scoped, so that scoping is the first job of this phase's discussion. The page ships first with a history timeline built from the vote log (posted, confirmed from N places, disputed, resolved), which has no free text. Comments follow.
**Requirements**: PAGE-01, PAGE-02
**Depends on:** Phase 3 (page and timeline), Phase 4 (comments only)
**UI hint**: yes
**Success Criteria** (what must be TRUE):

  1. Tapping a report in the feed, or the popup's footer, opens that report's own page showing the same trust block as the popup, with a way back to the map.
  2. The page shows a timeline of how the report's trust changed (posted, confirmed from N places, disputed, resolved), built from the vote log, with no names or locations of voters.
  3. A signed-in user can read and add short comments on a report page. Every comment passes the moderation check and a per account rate limit, and comment text never reaches the page through a markup parsing sink.
  4. The popup footer shows a real comment count only, never a count the backend cannot supply.

**Scope notes** (starting points for discuss-phase, not decisions):

  1. Design starting points: sketch 002 (`.planning/sketches/002-full-report-view-and-thread/`) and decisions 4 to 6 in `.planning/sketches/REVIEWS.md` (page or side panel, trust numbers on the page, composer or read only).
  2. Phase 3 supplies the trust block and trust object the page reuses, and Phase 4 supplies the moderation and rate limiting free text needs. If comments are wanted before Phase 4, pull that work forward in discuss-phase.
  3. The d4 popup's comments footer needs a defined state until this phase ships (hidden, or shown without a made up count). Comment events then plug into Phase 3.2's updates.
  4. Phase 4 as written screens report text with a category based pending rule and rate limits report submission. Comments and nicknames (Phase 9) need a reusable text moderation hook with a held state that is not tied to report categories, plus a per account limiter, so either Phase 4 gains that criterion or this phase builds it.

**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 8 to break down)
### Phase 9: Followable reporter profiles

**Goal:** People who choose to be public, and have earned the Reliable reporter tag, can be followed, and a follower is told when they post, without exposing anyone who did not opt in. Raised by the owner on 2026-10-03, it takes up the "public reporter profile page" that Phase 3 deferred. First to cut if time runs short, because Phase 3.2's Reliable-reporter alerts already deliver most of the value without profiles.
**Requirements**: PROF-01, PROF-02, PROF-03
**Depends on:** Phase 3, Phase 3.2, Phase 4, Phase 5
**UI hint**: yes
**Success Criteria** (what must be TRUE):

  1. A user can switch on "Let others follow me" (off by default), and only a user with the Reliable reporter tag can be followed. They choose a nickname, never the email; nicknames are unique, screened and exclude reserved words such as agency names.
  2. An opted-in reporter's profile shows the nickname, the reliability tag with its counts ("7 of 9 earlier reports were confirmed by people nearby"), member since, and recent finished reports at a coarse location, with no email and no follower count anywhere.
  3. A user can follow and unfollow privately, mute or block, and see reports from people they follow in a Following view next to Nearby and Alerts, with their distance.
  4. A follower is told when a followed reporter posts: critical and rescue-needed reports immediately, other reports only once they are Live, with no coordinates or address in the text, through Phase 3.2's pipeline.
  5. Switching off "Let others follow me", or the Reliable tag falling away, ends all follows and pending alerts at once, and a reporter can remove all followers without seeing who they are.

**Scope notes** (starting points for discuss-phase, not decisions):

  1. Care points: a nickname makes one person's reports linkable, and Phase 3 D-10's "not a privacy risk" holds only while no nickname is shown, so opt-in is required and history shows coarse locations only. A notification must respect the provisional gate so it cannot amplify an unconfirmed report. No follower counts and no likes, so reporting does not become a popularity game.
  2. The "See this reporter" link sits inside the trust block's explanation panel and appears only for opted-in reporters.
  3. Data: a nickname and a public-profile flag on accounts, a `follows` table and a block list. Following is private.

**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 9 to break down)
