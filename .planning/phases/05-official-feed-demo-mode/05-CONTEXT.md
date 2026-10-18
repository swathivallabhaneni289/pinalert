# Phase 5: Official Feed & Demo Mode - Context

**Gathered:** 2026-10-17 and 2026-10-18, from the owner's answers given one at a time by voice (discuss round run by session pinalert-60), plus the owner decisions already written in ROADMAP Phase 5 (2026-10-16 and 2026-10-17).
**Status:** Ready for research and planning. Everything tagged CLAUDE'S DISCRETION stands on the default named here unless the owner vetoes it at the viewing session. The owner judges layout by looking at a test copy, so do not reopen placement questions before then.

<domain>
## Phase Boundary

A first-time visitor sees a working product instead of an empty one: real government warnings (NDMA SACHET, which carries IMD, CWC and state authority alerts) fill the Official alerts place and, when serious, a one line banner on Nearby; GDACS hazard pins sit on the map; a Demo opens a practice copy of the 2024 Vijayawada floods without an account; and the sign in page shows a real turning Earth behind the card. Severe or Extreme alerts for a person's home area also reach their phone, silently, through Phase 3.3's plumbing.

Not in this phase: shading alert areas on the map, the Open-Meteo rain line, the replay slider (backlog 999.5), the explainer video (after the project is finished), the shared report status page (Phase 8).

Requirements: ROBUST-05, ROBUST-06, ROBUST-07, ALERT-01, ALERT-02, ALERT-03, ALERT-04, plus success criterion 5 (the globe).
</domain>

<decisions>
## Implementation Decisions

### How to read the status tags
- **OWNER DECIDED**: the owner said it in this round. Do not re-ask.
- **LOCKED EARLIER**: decided in the ROADMAP or an earlier phase and carried here. Do not re-ask.
- **CLAUDE'S DISCRETION**: the owner was not asked. Build the default named here unless the owner vetoes it at the viewing session.

### The Demo
- **D-01 [OWNER DECIDED, 2026-10-17]:** In the Demo a visitor can do everything a real user can, on pretend data: open reports, confirm or dispute them, and post a practice report. A few seconds after they post, pretend neighbours confirm it one by one, so they watch "confirmed by N nearby" climb to Confirmed. That number is the heart of Pinalert, so the Demo must show it working. Nothing they do is saved: leaving or reloading starts the Demo fresh. Chosen over "look only plus one simulate a report button".
- **D-02 [LOCKED EARLIER, ROADMAP owner decisions 2026-10-16]:** The Demo is a sandboxed practice copy with no login, so nothing touches real reports, votes, ratings or reporter tags. The sample is the 2024 Vijayawada floods along the Budameru, with real place names on the map and made-up reports, each visibly marked Demo, and no real people or real reports.
- **D-03 [OWNER DECIDED, 2026-10-17]:** Where the Demo is offered:
  - On the sign in screen, in the same quiet row as the legal links: **Try the demo · Disclaimer · Policy**. Not a big button.
  - In the first-time flow, on **step 1 Welcome**, as a secondary button **beside Continue** (owner, from a screenshot of step 1). No new step is added, so a person in a hurry never clicks through an extra screen.
  - The step 2 Disagree message already points to the Demo (03.4 D-27) and stays.
  - The explainer video link, when it exists, sits beside the sign in screen's Try the demo link (ROADMAP owner decisions 2026-10-16).
- **D-04 [OWNER DECIDED, 2026-10-17]:** An account is needed to use Pinalert. Only these open without one: the Demo, the Disclaimer and Policy pages, and the shared report status page (Phase 8). The Demo route joins the sign in gate's exempt list, and `internal/api/gate_test.go` is updated with a note, as ROADMAP scope note 6 foresaw (amends Phase 1.1 D-05).
- **D-05 [CLAUDE'S DISCRETION]:** How the Demo works:
  - It looks like the real app (Nearby, Official alerts, My activity, Post a report), with a slim strip on every Demo screen: "Demo. Nothing here is real." and an Exit button that returns to wherever the visitor came from: the sign in screen, or step 1 Welcome of the first-time flow for a signed in person. Every sample report and sample alert carries a Demo mark.
  - It opens on a moment during the flood, with times shown relative to that moment ("20 min ago"), centred on Vijayawada, and never asks for location.
  - About 15 to 25 sample reports across the eight categories and all three severities, a few already Confirmed, one Disputed, one Ended. A few sample official alerts (marked Demo) fill Official alerts and show the banner, so the visitor sees that part too.
  - The practice report and the visitor's votes run through the real `VisibilityResolver` and trust copy on in-memory or per-visit data, never the live tables. The pretend neighbours confirm from different places, one every few seconds, until the report reaches the real Confirmed threshold, so the Demo proves the actual mechanic and not a fake animation.
  - Settings, the bell and phone alerts are not live in the Demo. My activity shows the practice report and the votes cast during this visit.
  - The Demo route is rate limited per IP (it has no account) and writes nothing to the database.

### Official alerts content and the banner
- **D-06 [LOCKED EARLIER]:** Official alerts is a main place built by Phase 3.4 (03.4 D-10, D-22) and filled by this phase: one list, "In force now" with the count and the districts, and "Last checked N minutes ago" at the top. On a laptop, rows line up in columns (Level, Alert, Issued by, Issued, Until). Phone cards end with Issued and Until on one line. It replaces the "Alerts tab in the list panel" of ROADMAP scope note 4. The page must never say that no alert is in force when the feed is down or not connected (03.4 T-34-17).
- **D-07 [OWNER DECIDED, 2026-10-17, settles the headline strip todo]:** The owner asked whether the banner should exist at all; it stays, because people open Nearby and not Official alerts. It shows only when a Severe or Extreme alert covers the reader's area. Its shape, after the owner said two lines took too much space and asked for "neat, not too much text, not AI coded, not beginner level, clean":
  - **One slim line, no taller than a list row** (44px tap height), at the top of the Nearby list where Phase 3.4 put it (`#alert-banner` in `web/templates/index.html.tmpl`). Not over the map.
  - A level icon, then "Cyclone warning · Krishna district", then a small chevron on the right, like a row in phone Settings. The whole row is the tap target and opens Official alerts.
  - No underlined "tap for details" words and no long alert text. The issuer and the full text live on Official alerts.
  - Still, never a scrolling crawl.
  - The level shows by colour plus the icon (red for Extreme, orange for Severe), never colour alone.
  - The area name shortens to fit and never wraps.
- **D-08 [CLAUDE'S DISCRETION]:** When several Severe or Extreme alerts are in force, the line shows the newest most serious one and a small "+2" count before the chevron. When the feed is stale or unavailable, the banner never shows an old alert as current (ALERT-04): it hides, and Official alerts says plainly that the feed could not be checked and when it last worked.
- **D-09 [CLAUDE'S DISCRETION]:** Which area Official alerts covers: alerts covering where the reader is now (the same place Nearby uses, their location or the place they searched), plus the reader's home area in its own group when it is a different district. Phone notifications use the home area only (Phase 3.3). Alerts that can only be placed at state level are listed in an "Across <state>" group and never dropped, Extreme ones above all (ROADMAP scope note 2b).
- **D-10 [CLAUDE'S DISCRETION]:** Alerts that arrive in a regional language show the English headline where the feed carries one, and the original text in full on the alert's details.

### Phone notifications for official alerts
- **D-11 [LOCKED EARLIER, amended by OWNER DECIDED 2026-10-18]:** Severe or Extreme official alerts covering the person's home area reach the phone, on by default, with a Settings switch; the push plumbing comes from Phase 3.3 and this phase adds the row and turns it on (ROADMAP owner decisions 2026-10-16). **Amended 2026-10-18: no buzzes anywhere in the project.** The owner said "not gonna have any buzzes in this project. We need to like get rid of that" and chose option A: notifications still appear on the lock screen but silently, no sound and no vibration. This replaces the 2026-10-16 wording "a cyclone or something should be notified and should buzz". An exception that keeps sound for a serious danger warning was offered and not taken. Known limit, told to the owner: iPhone's own settings decide the sound; the site can only ask for silence.

### GDACS pins
- **D-12 [CLAUDE'S DISCRETION]:** GDACS events (earthquakes, tropical cyclones, floods and the other GDACS hazard types) show on the Nearby map as an official pin with its own shape and the authority badge, never mistakable for a crowd pin or a Demo pin (ROBUST-06). Tapping one opens a read only card with the issuer, the hazard, issued and until, and a link to the GDACS page. No confirm or dispute controls, never counted in "N reports" or any crowd number, no reliability tag (Phase 3 D-10). Where an event covers the reader's district it is also listed on Official alerts. GDACS stays the last item on the cut list (ROADMAP overview); the owner wants nothing cut, so plan it as the final wave.

### The sign in page globe
- **D-13 [LOCKED EARLIER, ROADMAP owner decisions 2026-10-17]:** Built in this phase from sketch 009: placement A, the globe behind the card; NASA Blue Marble pictures from NASA GIBS drawn by the maplibre-gl 5.24.0 the map page already loads (globe projection); one turn every 2 minutes with India in the middle at the start; still under reduce motion; nothing downloaded in Low data view or on a device that cannot draw it; a credit line. It overrides the Phase 1.1 UI-SPEC ban on WebGL and CDN libraries on the sign in page. The sketch README's "For the build" list (21 items) is the starting point. Whether the server may keep a copy of the NASA pictures is checked against NASA's terms at planning (item 21).
- **D-14 [LOCKED EARLIER]:** Phase 8 replaces the sign in card with Create account and Sign in (password, one emailed 6-digit code). The globe sits behind whichever card is there; whichever of Phases 5 and 8 is planned second builds on the other's `login_gate.html.tmpl`. The new "Try the demo · Disclaimer · Policy" row (D-03) must survive that change.

### The feed plumbing (from the ROADMAP scope notes, carried as the research gate)
- **D-15 [LOCKED EARLIER, ROADMAP scope notes 1 to 4]:** SACHET first, GDACS second. Official alerts get their own table, never the `reports` table (it has a required session id and no source column). The Go poller fetches server side (no CORS). Point in polygon is done in Go (no PostGIS). Read only, never mixed into crowd counts. The spike before planning writes down three things: the scheduler on a host that sleeps (a lazy refresh on request with a stored last-checked time, or an external cron calling a protected endpoint), the boundary data (polygon alerts, name-only alerts needing a district and mandal gazetteer, and the fallback for alerts that cannot be placed), and the GDACS storage decision. Also settle: does the RSS item carry severity, expiry, issuer and area or does each alert need its own CAP fetch; CAP Update and Cancel; a fallback lifetime when expires is missing. Confirm NDMA's terms before wide promotion.

### Claude's Discretion (summary)
D-05 (how the Demo works), D-08 (several alerts, stale feed), D-09 (which area), D-10 (regional language), D-12 (GDACS pins). The Open-Meteo rain line is not built (deferred).

### Folded Todos
- `.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`: the globe, now D-13 and D-14.
- `.planning/todos/pending/2026-10-16-official-alert-headline-strip-over-the-map.md`: settled by D-07 (one slim still line at the top of the Nearby list, not over the map).
- `.planning/todos/pending/2026-10-16-feed-tabs-active-choice-and-official-alerts.md`: already settled by Phase 3.4 (Nearby's Show: All, Confirmed, Disputed menu and Official alerts as a main place); nothing left for this phase.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope and earlier decisions
- `.planning/ROADMAP.md` Phase 5 section: goal, success criteria 1 to 5, scope notes 1 to 6, owner decisions 2026-10-16 and 2026-10-17.
- `.planning/REQUIREMENTS.md`: ROBUST-05, ROBUST-06, ROBUST-07, ALERT-01 to ALERT-04.
- `.planning/phases/03.4-app-layout/03.4-CONTEXT.md`: D-10 (main places), D-15 (banner placement), D-22 (Official alerts layout), D-25 (Settings), D-27 (first-time flow, Disagree points to the Demo), D-29 and D-30 (notice strip).
- `.planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md`: the push plumbing, the rows in the Settings section Notifications (named "Phone notifications" before 2026-10-18), and the 2026-10-18 amendment (silent notifications).
- `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md`: D-05, login required to view anything (amended by D-04 here).
- `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-CONTEXT.md`: D-10 (official pins carry no rating).

### The globe
- `.planning/sketches/009-login-globe/README.md`: placement A, NASA GIBS, download weight, "For the build" items 1 to 21.
- `.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`: history of the globe request.

### Feeds
- SACHET India RSS: `https://sachet.ndma.gov.in/cap_public_website/rss/rss_india.xml` (checked 2026-10-03, no key, public domain, CAP 1.2 links).
- GDACS open data feeds (research picks the exact endpoint).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/api/handlers/alerts.go` and `web/templates/alerts.html.tmpl`: the Official alerts place exists with an honest empty state ("Official alerts are not connected yet"); this phase replaces the empty state with the list and keeps the rule that it never claims no alert is in force.
- `web/templates/index.html.tmpl`: `#alert-banner` (a hidden link to `/alerts` with icon, title and line) already sits at the top of the Nearby list sheet; D-07 reshapes it into one line.
- `web/templates/welcome.html.tmpl`: step 1 has a single Continue (`#welcome-continue-1`); D-03 adds Try the demo beside it.
- `web/templates/login_gate.html.tmpl`, `web/static/css/auth.css`: the sign in page and today's flat CSS globe.
- `internal/service` resolver (`VisibilityResolver`, `confirmed.go`, `currency.go`, `trustcopy.go`): reused by the Demo so the practice report runs the real mechanic.

### Established Patterns
- Public routes are registered before the gated group in `internal/api/router.go` (`/login`, `/disclaimer`, `/policy` around lines 399 to 409); the pinned exempt list is tested in `internal/api/gate_test.go`.
- Scheduled work uses a plain `time.Ticker` goroutine (stack notes); the free host sleeps, so the spike decides lazy refresh or external cron.
- Every page is server rendered with `html/template` and vanilla JS, no build step; CDN scripts are pinned with SRI.

### Integration Points
- Settings, Notifications (the section's name since 2026-10-18): this phase adds the "Severe official alerts for my area" row and turns it on (03.3).
- Phase 8 replaces the sign in card; coordinate `login_gate.html.tmpl` (D-14).

</code_context>

<specifics>
## Specific Ideas

- The owner's words on the banner: "should be neat and not too much of text not too AI coded one or not like a beginner level one should be clean", and "if you make it bigger for like the cyclone thing, it's just gonna like take more space".
- Banner example the owner accepted: `▲ Cyclone warning · Krishna dist.  ›`
- The owner's words on buzzing: "not gonna have any buzzes in this project. We need to like get rid of that."

</specifics>

<deferred>
## Deferred Ideas

- Shading the affected alert areas on the map (ROADMAP scope note 4, later).
- Open-Meteo rain outlook line (ROADMAP scope note 5, optional, not in this phase).
- Replay the last 24 hours (backlog 999.5), which would extend the Demo.
- The explainer video, after the project is finished (ROADMAP owner decisions 2026-10-16).
- A sound exception for serious danger warnings: offered on 2026-10-18 and declined outright by the owner the same day ("we are not gonna have any errors or buzz buttons for this. It's only gonna be a notification thing"). No buzz, alarm or sound option anywhere, not even a switch. Do not offer it again.

</deferred>
