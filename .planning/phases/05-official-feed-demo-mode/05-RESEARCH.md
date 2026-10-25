# Phase 5: Official Feed & Demo Mode - Research

**Researched:** 2026-10-25
**Domain:** Server side ingestion of two public alert feeds (NDMA SACHET CAP over RSS, GDACS GeoJSON), point in polygon placement without PostGIS, a sandboxed in memory Demo that runs the real trust code, silent phone alerts through Phase 3.3's plumbing, and a MapLibre globe on the sign in page.
**Confidence:** MEDIUM overall. HIGH on the feed shapes, the boundary data and the Demo design (all probed or read in code this session). MEDIUM on the scheduler (host and cron behaviour is cited, not run). LOW on NDMA's written terms (no readable policy text was reachable).

All live calls of this session were made with the User-Agent `Pinalert-research/0.1 (portfolio project; contact address)`. Fourteen feed calls in total: 10 to SACHET (the RSS, four CAP messages, two refused polygon requests, the home page and two policy pages), 3 to GDACS (one a 400 for a missing parameter), 1 to NASA GIBS. The boundary release was downloaded once from GitHub. Raw probe outputs and the Go probe programs are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/3bfdaabd-2ee0-416f-851f-d0a5ce547954/scratchpad/p5research/`, never in the repo.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

How to read the status tags
- **OWNER DECIDED**: the owner said it in this round. Do not re-ask.
- **LOCKED EARLIER**: decided in the ROADMAP or an earlier phase and carried here. Do not re-ask.
- **CLAUDE'S DISCRETION**: the owner was not asked. Build the default named here unless the owner vetoes it at the viewing session.

The Demo
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

Official alerts content and the banner
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

Phone notifications for official alerts
- **D-11 [LOCKED EARLIER, amended by OWNER DECIDED 2026-10-18]:** Severe or Extreme official alerts covering the person's home area reach the phone, on by default, with a Settings switch; the push plumbing comes from Phase 3.3 and this phase adds the row and turns it on (ROADMAP owner decisions 2026-10-16). **Amended 2026-10-18: no buzzes anywhere in the project.** The owner said "not gonna have any buzzes in this project. We need to like get rid of that" and chose option A: notifications still appear on the lock screen but silently, no sound and no vibration. This replaces the 2026-10-16 wording "a cyclone or something should be notified and should buzz". An exception that keeps sound for a serious danger warning was offered and not taken. Known limit, told to the owner: iPhone's own settings decide the sound; the site can only ask for silence.

GDACS pins
- **D-12 [CLAUDE'S DISCRETION]:** GDACS events (earthquakes, tropical cyclones, floods and the other GDACS hazard types) show on the Nearby map as an official pin with its own shape and the authority badge, never mistakable for a crowd pin or a Demo pin (ROBUST-06). Tapping one opens a read only card with the issuer, the hazard, issued and until, and a link to the GDACS page. No confirm or dispute controls, never counted in "N reports" or any crowd number, no reliability tag (Phase 3 D-10). Where an event covers the reader's district it is also listed on Official alerts. GDACS stays the last item on the cut list (ROADMAP overview); the owner wants nothing cut, so plan it as the final wave.

The sign in page globe
- **D-13 [LOCKED EARLIER, ROADMAP owner decisions 2026-10-17]:** Built in this phase from sketch 009: placement A, the globe behind the card; NASA Blue Marble pictures from NASA GIBS drawn by the maplibre-gl 5.24.0 the map page already loads (globe projection); one turn every 2 minutes with India in the middle at the start; still under reduce motion; nothing downloaded in Low data view or on a device that cannot draw it; a credit line. It overrides the Phase 1.1 UI-SPEC ban on WebGL and CDN libraries on the sign in page. The sketch README's "For the build" list (21 items) is the starting point. Whether the server may keep a copy of the NASA pictures is checked against NASA's terms at planning (item 21).
- **D-14 [LOCKED EARLIER]:** Phase 8 replaces the sign in card with Create account and Sign in (password, one emailed 6-digit code). The globe sits behind whichever card is there; whichever of Phases 5 and 8 is planned second builds on the other's `login_gate.html.tmpl`. The new "Try the demo · Disclaimer · Policy" row (D-03) must survive that change.

The feed plumbing (from the ROADMAP scope notes, carried as the research gate)
- **D-15 [LOCKED EARLIER, ROADMAP scope notes 1 to 4]:** SACHET first, GDACS second. Official alerts get their own table, never the `reports` table (it has a required session id and no source column). The Go poller fetches server side (no CORS). Point in polygon is done in Go (no PostGIS). Read only, never mixed into crowd counts. The spike before planning writes down three things: the scheduler on a host that sleeps (a lazy refresh on request with a stored last-checked time, or an external cron calling a protected endpoint), the boundary data (polygon alerts, name-only alerts needing a district and mandal gazetteer, and the fallback for alerts that cannot be placed), and the GDACS storage decision. Also settle: does the RSS item carry severity, expiry, issuer and area or does each alert need its own CAP fetch; CAP Update and Cancel; a fallback lifetime when expires is missing. Confirm NDMA's terms before wide promotion.

### Claude's Discretion
D-05 (how the Demo works), D-08 (several alerts, stale feed), D-09 (which area), D-10 (regional language), D-12 (GDACS pins). The Open-Meteo rain line is not built (deferred).

### Deferred Ideas (OUT OF SCOPE)
- Shading the affected alert areas on the map (ROADMAP scope note 4, later).
- Open-Meteo rain outlook line (ROADMAP scope note 5, optional, not in this phase).
- Replay the last 24 hours (backlog 999.5), which would extend the Demo.
- The explainer video, after the project is finished (ROADMAP owner decisions 2026-10-16).
- A sound exception for serious danger warnings: offered on 2026-10-18 and declined outright by the owner the same day ("we are not gonna have any errors or buzz buttons for this. It's only gonna be a notification thing"). No buzz, alarm or sound option anywhere, not even a switch. Do not offer it again.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ROBUST-05 | Map shows official GDACS pins alongside crowd reports, unaffected by votes | G4: SEARCH endpoint, own table `official_hazards`, separate pin layer and JSON route, no vote controls |
| ROBUST-06 | GDACS pins and SACHET alerts carry a distinct authority badge, distinguishable from crowd and Demo reports | G4 pin design, G5 Demo mark, Pitfall 12, Validation rows for the badge |
| ROBUST-07 | First time visitor sees a populated working demo and can try confirm/dispute | G5: in memory per visit world running the real `CastVote`, `Nearby`, `Resolve`, `BuildTrust`; neighbour script; exempt routes; Vijayawada sample |
| ALERT-01 | Alerts view lists official alerts covering the reader's district from SACHET | G2 two step fetch, G3 LGD district placement, reader area rules |
| ALERT-02 | Each alert shows issuer, issued, expires, authority badge, disappears on expiry | G2 field map, expiry fallback, query filters `expires_at > now()` and `cancelled_at IS NULL` |
| ALERT-03 | Banner only for Severe or Extreme covering the reader's area | G1 honesty states, D-08 selection, Open Question 1 (state only alerts) |
| ALERT-04 | Show last checked and say plainly when a feed is unavailable | G1 `official_feed_status`, freshness state machine, page and banner rules |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Read from `/Users/swathivallabhaneni/code/pinalert/.claude/CLAUDE.md` and the owner's memory notes. The planner verifies every plan against these.

- Stack is fixed: Go 1.25+ (repo `go.mod` says 1.26.4), chi v5, pgx v5 with sqlc, goose migrations, `html/template` plus vanilla JS with no build step, Leaflet with the MapLibre basemap, PostgreSQL 16/17 with plain lat/lon columns and **no PostGIS**. Free tiers only, no paid API, SMS or WhatsApp.
- Migrations run as an explicit step, never on app boot. Scheduled work uses a plain `time.Ticker` goroutine (stack notes).
- CDN scripts are pinned with SRI. Phase 5 must not add a second map library: the globe uses the maplibre-gl 5.24.0 the map page already loads. `maplibre-gl` 6.x has no browser global build and must not be used.
- Legal: public unofficial emergency app, India IT Rules safe harbour applies once public; wide promotion needs a legal or mentor review. NDMA terms (item G9) are part of that.
- GSD workflow: all edits go through a GSD command. Research here edits nothing but this file.
- Owner memory rules that bind plans: no dashes (em or en) in anything written; no Python anywhere (Go or shell for side work); no image files in the repo; never use ports 8080, 8090, 8776, 8778; never test against `pinalert_test`, `pinalert`, `p31_4_uat` or `p34_uat`, use a disposable database per plan; no buzzes or sound anywhere (notifications are silent); phone first, the same features on phone and laptop; owner judges layout by looking, so do not ask placement questions; actions are real buttons and never underlined links; commit path limited and never push; never stage STATE.md.
- Project skill: `sketch-findings-pinalert` (map pin popup, feed rows, trust block). Load it for the official pin card, the banner row and the Official alerts rows.

## Summary

Four findings change the plan more than any other, and all four come from live data, not from reading.

1. **The SACHET RSS item is a pointer, not an alert.** Each of the 100 items carries only a title (66 of 100 in a regional script), category (always `Met`), a CAP link, the issuer in `author`, a guid and a pubDate; `description` is empty. Severity, expiry, area, geocodes and languages exist only in the CAP message, so every item needs one CAP fetch (1.6 to 4.2 KB each). The feed is a sliding window of exactly 100 items covering exactly 24 hours, so an alert missing from the feed is not cancelled, and a host that sleeps through a day loses alerts it never saw.
2. **Districts are placeable, mandals are not.** IMD alerts carry `LGD District Code` geocodes, and all 26 codes I checked resolve in a CC0 district file (Tamil Nadu 23 of 23, Maharashtra 3 of 3). The most common alert in the feed (45 of 100 items, Andhra Pradesh SDMA lightning) says only `areaDesc` "6 Mandals" with no names and no codes, and its polygon URL answered 403 to a non browser client. A mandal gazetteer therefore cannot place the commonest case; the working path is the LGD code join plus a state taken from `sender`.
3. **The scheduler is decided by push, not by the page.** A phone alert (D-11) must go out when an alert appears, with nobody on the site. A sleeping host wakes only on inbound traffic, so lazy refresh alone cannot deliver D-11. Use an external cron that calls a protected "refresh if due" endpoint, a ticker while awake, and a lazy kick on read as the third layer.
4. **The Demo can run the real trust code unchanged.** `Resolve`, `BuildVoteTally`, `BuildTrust`, `buildCurrency` and `ConfirmedByPeople` are pure. `ReportService` and `VotingService` take small interfaces (`service.Querier`, `service.VotingQuerier`). An in memory implementation of those two interfaces, one per visit, makes the real `CastVote` (including the 1 km rule and the self vote block), the real `Nearby` and the real trust copy run with zero database access.

**Primary recommendation:** Build a new `internal/official` package (poller, CAP parser, placement, freshness, push enqueue) and a new `internal/demo` package (in memory `Querier` and `VotingQuerier`, per visit world, Vijayawada seed, deterministic neighbour script); embed a simplified LGD district file (about 0.36 MB gzipped); run the poller from an external cron plus a ticker plus a lazy kick behind one idempotent function; and add Phase 5's push as a sibling outbox table that reuses 3.3's `push.Sender` and subscription reads.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Fetch SACHET RSS and CAP, GDACS JSON | API / Backend (poller goroutine and refresh endpoint) | Database | Feed sends no CORS headers (D-15); only the server can hold secrets, a stored last checked time and a cold start safe schedule |
| Parse CAP, choose language, expiry, Update and Cancel | API / Backend | none | Untrusted XML must be bounded and validated server side before it is stored |
| Place an alert (district, state, unplaced) | API / Backend (embedded boundary data, Go point in polygon) | Database (stores district codes per alert) | No PostGIS; boundary data is a read only in memory structure |
| Decide what the reader sees (area match, banner pick, freshness state) | API / Backend | Browser (renders only what it is told) | Same rule as TRUST-02: the server authors the state, the browser never recomputes it. The server sends `age_seconds`, never a client clock comparison |
| "Last checked N minutes ago" ticking | Browser (re-renders from a server sent age) | API | Display only; the stored fact is the server's |
| Banner row, Official alerts page, official pin card | Frontend Server (SSR shell) and Browser (vanilla JS fetch of the JSON routes) | API | The page needs the reader's position, which only the browser holds (the server keeps no reader location) |
| GDACS pin drawing | Browser (Leaflet layer) | API (`/api/official/pins`) | Pins are map furniture; data comes from the server, never from `reports` |
| Silent phone alert for official alerts | API / Backend (enqueue, send) | Browser (service worker shows it) | Recipient choice stays on the server (3.3 rule); the worker only shows what it is given |
| Demo world, votes, neighbour script | API / Backend (in memory per visit) | Browser (fixed pretend position, polling) | The real rules live in Go; JS must not hold a second copy (TRUST-02) |
| Globe | Browser (WebGL via maplibre-gl) | API (optional NASA tile cache route) | Pure presentation; the server only keeps a copy of 85 tiles if D-13's item 21 option is taken |
| Boundary file build | Developer tool (`go run`) | none | Runs offline against the upstream CC0 file, writes one embedded artifact |

## Research Gate Decisions

Each item below states the decision first, then the evidence, then what keeps it honest or what can go wrong.

### G1. The scheduler on a host that sleeps

**Decision.** Three layers around one idempotent function `Refresher.RefreshIfDue(ctx, feed)`:

1. **External cron (primary).** A GitHub Actions workflow on `schedule: '*/10 * * * *'` sends `POST /internal/official/refresh` with `Authorization: Bearer <secret>`. The request itself wakes a sleeping instance. The endpoint is registered outside the sign in gate, answers 404 when `OFFICIAL_REFRESH_TOKEN` is unset, compares with `crypto/subtle.ConstantTimeCompare`, runs RefreshIfDue for both feeds with a 55 second deadline, and returns a small JSON summary.
2. **Ticker while awake.** `go officialPoller.Run(ctx)` started in `cmd/server/main.go` beside `moderationWorker.Run` (the existing pattern, `main.go` line 346) calls RefreshIfDue every 5 minutes.
3. **Lazy kick (fallback).** Any read of `/api/official/*` or `/alerts` starts a non blocking, singleflight RefreshIfDue when `last_attempt_at` is older than 10 minutes. It never delays the response.

RefreshIfDue skips a feed whose `last_attempt_at` is under 4 minutes old, so the three layers never stampede the upstream.

**The cron call must not create a `sessions` row.** The session middleware wraps every route and `persistSession` runs `UpsertSession` for any request without a valid cookie. A `curl` from GitHub Actions carries no cookie, so each `POST /internal/official/refresh` would write one `sessions` row: about 4,300 junk rows a month at a 10 minute cadence. Phase 3.3 plan 04's `skipSessionFor(paths, next)` on main bypasses the middleware only for GET and an exact path. Phase 5 extends that one function (a method set plus an exact path set plus the `/demo` prefix) so it also covers `POST /internal/official/refresh`. Pin it with an e2e row: a refresh call leaves the `sessions` count unchanged.

**Evidence.**
- D-11 pushes to a closed phone when an alert appears. An official alert comes from outside; no visitor request will wake a sleeping host. Lazy refresh alone therefore cannot deliver D-11. [VERIFIED: reasoning from D-11 and the host rule below]
- Render spins a Free web service down after "15 minutes without receiving any inbound traffic"; spin up "takes about one minute"; each workspace gets 750 Free instance hours a month and a spun down service does not consume them. Outbound traffic is not defined as activity. [CITED: render.com/docs/free]
- A cron every 10 minutes keeps the instance awake around the clock: 31 x 24 = 744 of 750 free hours, leaving 6 hours of headroom for anything else on that workspace. [CITED: render.com/docs/free; arithmetic mine] This is a deliberate cost: "always on" is exactly what push needs.
- The feed window is 100 items covering exactly 24 hours (oldest pubDate 2026-10-24 09:15:05Z, newest 2026-10-25 09:15:09Z). A host asleep for more than a day loses alerts permanently, and during a cyclone the 100 item cap shrinks the covered time. [VERIFIED: live fetch 2026-10-25]
- GitHub Actions: "The shortest interval you can run scheduled workflows is once every 5 minutes"; the event "can be delayed during periods of high loads"; in a public repository scheduled workflows "are automatically disabled when no repository activity has occurred in 60 days"; they run only on the default branch. [CITED: docs.github.com events-that-trigger-workflows#schedule] So the cron is best effort, which is why the ticker and the lazy kick exist, and why honesty must not depend on the cron.
- The Render page also says a Free service that "initiates unusually high traffic" may be suspended. One RSS call per 5 to 10 minutes plus a handful of CAP calls is far below that. [CITED: render.com/docs/free]

**How "Last checked N minutes ago" and the stale state stay honest (ALERT-04, D-08).** Independent of which layer ran:

- Table `official_feed_status`, one row per feed (`sachet`, `gdacs`): `last_attempt_at`, `last_success_at`, `last_error` (a closed word: `timeout`, `status`, `parse`, `anomaly`, `network`), `consecutive_failures`, `last_etag`, `last_modified`, `items_seen`.
- "Last checked" always prints `last_success_at`, never `last_attempt_at`. A fetch that fails does not move it.
- A success is: HTTP 200, the body parsed, and not an anomaly. Anomaly = a body that parses to zero items when the previous success saw 10 or more (a feed that suddenly empties is treated as a failure, not as "no alerts").
- Constants (tunable, named in one const block, pinned by a test): `OfficialFreshWindow = 30 * time.Minute`, `OfficialStaleLimit = 6 * time.Hour`.
- State machine, computed on the server and sent as `state` plus `age_seconds`:

| State | When | Official alerts page | Banner | Push |
|-------|------|----------------------|--------|------|
| `fresh` | success within 30 min | list, "Last checked N minutes ago". The words "No alerts in force" appear only here and only for an empty list | shown if a Severe or Extreme alert covers the reader | normal |
| `stale` | last success 30 min to 6 h ago | "We could not check for alerts since 14:05 (2 hours ago)." Rows still listed under "Last known alerts, as of 14:05", every row's expiry still applied, no "In force now" heading | hidden (D-08) | enqueue only alerts first seen before the gap; no new ones can arrive anyway |
| `unavailable` | never succeeded, or last success over 6 h ago | "Official alerts could not be checked." with the last good time or "never". No list. Points to the state disaster management authority | hidden | none |

- The page never says that no alert is in force unless state is `fresh` and the list is empty (03.4 T-34-17). A test pins that sentence against every state.
- Alerts are removed by their own `expires_at` and `cancelled_at`, not by absence from the RSS (absence means only that the 24 hour window moved on).

**Alternative the owner can choose.** Cron every 30 minutes lets the host sleep between runs. It saves instance hours but delays a push by up to 30 minutes plus a cold start. Default stays 10 minutes. This is a cost and latency choice, not a correctness one.

### G2. The fetch model

**Decision.** Two step fetch, always: the RSS list, then one CAP message per new or changed item. Conditional GET on the RSS. Key alerts by the RSS guid.

**What the RSS item carries (verified on all 100 items, 2026-10-25).**

| Field | In the RSS item? | Notes |
|-------|------------------|-------|
| title | yes | In the issuer's language: 66 of 100 non Latin script (Telugu, Malayalam, Marathi, Hindi), one title repeated 45 times |
| description | present but empty on all 100 | Never rely on it |
| category | yes, always `Met` | Carries no hazard type; CWC flood items also say `Met` |
| link | yes | `https://sachet.ndma.gov.in/cap_public_website/FetchXMLFile?identifier=<guid>` |
| author | yes | `controlroom@ndma.gov.in (IMD Chennai)` style. Issuer label is the text in brackets. 13 distinct issuers seen: Andhra Pradesh SDMA 45, IMD Chennai 24, IMD Thiruvananthapuram 9, IMD Chandigarh 5, IMD Mumbai 4, and others including CWC 1 |
| guid | yes | 16 digit string, `isPermaLink="false"`; equals the link's identifier and the base of the CAP identifier |
| pubDate | yes | The sent time of the latest version. Use it to detect a changed item |
| severity, expires, area, polygon, geocodes, language | **no** | All in the CAP message |
| channel pubDate | yes but stale | Said 16 Oct while items were 25 Oct. Ignore it |

The RSS response is 79 KB, `Cache-Control: no-store`, but it sends `ETag: W/"79131-1792919710087"` and `Last-Modified` that matched the newest item. [VERIFIED: response headers] Whether it answers `If-None-Match` with 304 was not tested (call cap). Send the conditional headers and treat a 304 as a success that moves `last_success_at`; if the server ignores them nothing breaks. [ASSUMED: 304 behaviour]

**What the CAP message carries (4 real messages read).**

| CAP field | Seen | Use |
|-----------|------|-----|
| `identifier` | `IN-1792919196941008_8` | Full value kept as `cap_identifier`. Base before `_` equals the RSS guid |
| `sender` | `IMD-Chennai`, `Andhra-Pradesh-SDMA`, `Maharashtra-SDMA`, `CWC` | State fallback (see G3) |
| `sent`, `status`, `msgType`, `scope` | `Actual`, `Alert` or `Update`, `Public` | Store only `status = Actual` and `scope = Public`; ignore Exercise, Test, System, Draft, Private |
| `references` | `IMD-Mumbai,IN-1792859773702029_46,2026-10-24T22:05:47+05:30` on an Update | Full referenced identifier. Note its suffix (`_46`) differs from the message's own (`_29`): match on the full value, never on the base alone |
| `info` (repeated) | 2 blocks on 2 of 4 messages | One block per language. Observed `language` values: `en-IN`, `TL` (Telugu, not a standard code), `MR`. Treat the code as an opaque label |
| `info.event`, `urgency`, `severity`, `certainty` | `Lightning`, `Expected`, `Severe`, `Possible` | Severity values seen: Severe, Moderate. CAP's closed set is Extreme, Severe, Moderate, Minor, Unknown |
| `info.effective`, `onset`, `expires` | all present on all 4 | `expires` present on every message seen; still needs a fallback |
| `info.headline` | English text in the `en-IN` block, original text in the other | D-10 source |
| `info.description`, `instruction` | `description` empty on 3 of 4; CWC flood has full water level text; `instruction` "Please follow SDMA guidelines." | Details view |
| `area.areaDesc` | `6 Mandals`; `Chengalpattu, Chennai, ... districts of Tamil Nadu`; `Gandak, Dumariaghat, Gopalganj, Bihar` | Display only; never parsed for placement except a trailing state |
| `area.geocode` `LGD District Code` | 23 and 6 repeated entries on the IMD and Maharashtra messages; none on the AP SDMA and CWC ones | The placement key (G3) |
| `area.altitude`, `ceiling` | `26.35` and `84.76` on the CWC message; `0` elsewhere | Probably the station's latitude and longitude (see G3). Not a documented field meaning. Do not use |
| `info.parameter` `Polygon URL` | on all 4 | `FetchPolygonXMLFile?identifier=<guid>`. **Answered 403 to a non browser client twice (default and with a Referer); not worked around.** No inline `cap:polygon` or `cap:circle` appeared in any message |

CAP messages are 1.6 to 4.2 KB, `Content-Type: text/xml;charset=UTF-8`, ETag present.

**Languages (D-10).** Pick the headline from the first `info` whose language starts with `en`; keep the first non English `info` as the original. Show the English headline in rows and the banner source; show the original text in full in the details. If no English block exists, show the original with the label "Original language". Store `headline_lang_original` as the raw string.

**Update and Cancel.** One Update was seen (`msgType Update`, `references` present, same base id as the RSS guid, sent by `Maharashtra-SDMA` relaying an `IMD-Mumbai` alert). No Cancel appeared in the 4 messages read, and testing all 100 was outside the call budget. Handling follows CAP 1.2:
- `Alert`: insert.
- `Update`: upsert by guid (the RSS guid is the series key; the URL serves the latest version). Also mark any stored alert whose `cap_identifier` equals a full identifier in `references` as `superseded_at = now()`.
- `Cancel`: set `cancelled_at = now()`, hide immediately, keep the row for the page's history and for push dedupe.
- `Ack`, `Error`: ignore.
- NDMA's real Cancel behaviour is unverified. [ASSUMED: follows the CAP 1.2 standard]. Refetch the CAP only when the guid is new or its pubDate differs from the stored `source_pubdate`.

**Fallback lifetime when `expires` is missing or invalid.** `expires_at = sent + 6 hours`, shown as "Until about 20:35" with `expires_inferred = true`, and the details say "No end time was given." An `expires` earlier than `sent`, or later than `sent + 7 days`, counts as invalid and takes the fallback. [ASSUMED: 6 hours and 7 days are my choices; the planner or owner may change the two constants, pinned by tests]. IMD nowcasts here expire in about 3 hours, CWC flood in 24 hours.

**Bounds and politeness.**
- Fetch CAP for at most 40 new or changed items per run, newest first, 3 at a time, 8 second timeout, one retry. A cold start with 100 unseen items therefore takes 3 runs; nothing is lost because the window is 24 hours.
- Build the CAP URL from the guid after checking `^[0-9]{10,20}$`; never follow the `link` or any URL from the feed. Fixed host `sachet.ndma.gov.in`, `https` only, redirects refused.
- Limits: RSS body 1 MB, CAP body 256 KB (`io.LimitReader`). `encoding/xml` does not resolve external entities.
- User-Agent built like `internal/geocode`'s (contact from `CONTACT_EMAIL`, the base URL), for example `Pinalert/1.0 (+https://<host>; contact <email>)`. Never log a URL with query or the response body.
- Stored text is untrusted: strip control characters, cap lengths (headline 400, description 4000, instruction 1000, area 600 runes), render only through `html/template` or `textContent`.

**Dedupe for display.** 45 of the 100 items are the same Telugu sentence for different mandal sets, repeated every 4 to 10 minutes. Group rows by (`sender`, `event`, `severity`, `headline_en`) and show the newest with a count ("Lightning, Andhra Pradesh, 45 alerts today"). Without this the Official alerts page is one long list of the same line.

### G3. The boundary data

**Decision.** Districts only, joined by LGD code, from the CC0 `LGD_Districts` layer, simplified and embedded in the binary. No mandal or tehsil layer. Point in polygon is hand written Go (about 25 lines plus tests).

**Why not mandals.** The commonest alert (AP SDMA) names no mandals at all ("6 Mandals"), carries no geocodes and its polygon URL is refused. A mandal gazetteer would have nothing to match. The LGD subdistrict layer exists (6,471 polygons, CC0, 62.9 MB parquet) and is the later option if NDMA starts sending named mandals. [CITED: bharatlas.com]

**The data.** Source: ramSeraph `indian_admin_boundaries`, release tag `districts`, asset `LGD_Districts.geojsonl.7z` (22.2 MB; published 2023-12-11; description "Indian District Boundary data mapped to LGD codes", source LGD / Bharatmaps). [VERIFIED: gh api] A second listing, bharatlas.com, shows the same layer as "Districts (2024), LGD: 785 polygons", CC0 or CC BY 4.0, snapshot LGD 2024, 21.2 MB parquet. [CITED: bharatlas.com]

Probe results on the downloaded file (Go, this session):

| Check | Result |
|-------|--------|
| Features | 785 polygons, 784 distinct `dist_lgd` codes (one code appears twice: merge polygons per code) |
| Properties | `dist_lgd` (LGD district code), `dtname`, `state_lgd`, `stname`, plus census code `dtcode11` (do **not** key on `dtcode11`; it is the 2011 census code and does not match the CAP) |
| CAP code resolution | Tamil Nadu alert: 23 of 23 codes resolve (574 Kanchipuram, 568 Chennai, 730 Chengalpattu, 733 Tenkasi, 731 Ranipet, ...). Maharashtra: 492 Ratnagiri, 480 Kolhapur, 495 Sindhudurg, exactly the alert's text. The new post 2019 districts resolve too |
| Andhra Pradesh | 26 districts, so the 2022 reorganisation is in the file |
| Point in polygon, original | Vijayawada 16.5062,80.6480 falls in Ntr (lgd 749); Chennai 568; Kolhapur 480; Mumbai 483; Guwahati 618; Puri 369 |
| Simplified 0.01 degree (about 1.1 km), 3 decimals | 87,460 points, 1.39 MB JSON, **0.36 MB gzip**; same answers on the three test points |
| Simplified 0.005 degree (about 0.6 km) | 139,835 points, 2.22 MB JSON, 0.60 MB gzip |
| Lookup cost | 1,000 full scans of all 785 districts: 130 ms (0.13 ms per scan) before any bounding box prefilter |
| Original file | 86.4 MB uncompressed geojsonl; too large to embed |

The probe used an iterative Douglas-Peucker pass and rounded coordinates; the builder reproduces that. [VERIFIED: Go probe, `distprobe2`]

**Embedding.** A developer tool `cmd/buildareas` (Go, stdlib only) reads the upstream file (downloaded and unpacked by hand; macOS `bsdtar -xf` unpacks the `.7z`, no `7z` binary needed), simplifies at 0.01 degree and 3 decimals, merges duplicate codes, and writes `internal/areas/data/districts.json.gz` (about 0.36 MB, a data file, not an image). It is embedded with `go:embed`, decoded once at startup into `[]District{Code, Name, State, StateCode, BBox, Polys}`. A repo test checks the file's size band, that all 785 polygons parse, and that the three anchor points (Vijayawada, Chennai, Kolhapur) resolve. Accuracy limit: a point within about a kilometre of a district edge can land in the neighbour. This is acceptable at district scale and is said in `docs/` plainly.

**Licence.** The compiler dedicates the data under CC0 1.0 and asks for attribution to the Datameet community and "the original government sources wherever possible". The original is LGD / Bharatmaps government data. [CITED: github.com/ramSeraph/indianopenmaps DATA_LICENSE.md] The compiler's own tile path calls the set "not-so-open", and the government layer itself has no explicit open licence statement I could read, so confidence on the original is MEDIUM. Mitigations: the file is used only for an internal point in polygon test and is never drawn (no borders are shown, so no map depiction question arises); the credit goes in `docs/` and on the Policy page ("District areas: Local Government Directory via Datameet and ramSeraph, CC0"); the national geospatial guidelines (2021) and policy (2022) the compiler links favour reuse. This joins the pre promotion legal review already required by CLAUDE.md. [ASSUMED: that this reasoning satisfies the original publisher]

**Placement tiers (one function, `Place(alert) Placement`).**

| Tier | Input | Result | Seen in the feed |
|------|-------|--------|------------------|
| 1 `district` | one or more `LGD District Code` geocodes, any `info` block | `district_codes[]` plus `state_codes[]` from the file | IMD (Chennai, Mumbai relay), 2 of 4 messages |
| 2 `polygon` | inline `cap:polygon` or `cap:circle` | Optional. Reduce to the districts the shape touches | Never seen. Implement only if time allows; do not depend on it |
| 3 `state` | no codes: map `sender` of the form `<State>-SDMA` to a state; else a trailing state name in `areaDesc` after the last comma or "of" | `state_codes[]` only | AP SDMA "6 Mandals", CWC "Gandak, Dumariaghat, Gopalganj, Bihar" |
| 4 `unplaced` | neither | no area | not seen, must still be stored |

The sender to state table has one entry per state or UT SDMA (the portal says it takes alerts from "all 36 states/UTs Disaster Management Authorities" [CITED: sachet.ndma.gov.in/CapFeed]) and is tested against the `stname` set of the boundary file so a typo cannot silently place nothing.

**Fallback for alerts that cannot be placed at district level (D-09).** They are never dropped. They appear in an "Across <state>" group, Extreme first, then Severe, then by newest. Fully unplaced alerts appear in one last group "Could not be placed on a map", visible to everyone on Official alerts only, never in the banner or a push.

**The CWC station coordinate guess.** `altitude 26.35` and `ceiling 84.76` look like the gauge's latitude and longitude. Point in polygon on (26.35, 84.76) lands in Purbi Champaran, Bihar (lgd 213), while the alert says Gopalganj: right state, the neighbouring district across the river Gandak. Record as LOW confidence; do not use it for placement. State from the trailing "Bihar" is enough. [VERIFIED: probe; meaning of the fields ASSUMED]

**How the reader's area is known (D-09).**
- The server keeps no reader location (`position.js` header comment). So Nearby's JS sends its current position (device fix or the searched place, whichever Nearby is using) as `lat` and `lon` on `GET /api/official/alerts`. `requestLogger` logs only the path (`EscapedPath`), so the query string is not logged.
- The home area is stored as a place name plus a 5 character geohash cell (`accounts.home_area_name`, `accounts.home_area_cell`, migration 00008; no district, no coordinates). The server decodes the cell centre with `HomeAreaAnchor` (`internal/service/profile.go` and `alertsettings.go`). A 5 character cell is about 4.9 km square, so test the centre and the four corners and match if any of the five points lies in an alert district. This biases toward telling the person, never away. [ASSUMED: this is the right bias]
- The reader's state comes from the district their point falls in, which selects the "Across <state>" group. If the home district differs from the reader's, the home group is its own section above the state group.

### G4. GDACS

**Decision.** Use the documented SEARCH API, two small requests per refresh, own tables, own pin layer.

**Endpoint and fields.** `https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH` with `eventlist` (EQ, TC, FL, VO, DR, WF; semicolon separated), `fromdate`, `todate` (YYYY-MM-DD), `alertlevel` (red, orange, green; semicolon separated), and `pagenumber` and `pagesize` (at most 100 records per call). GDACS asks clients to save the collection and compare the `datetime` property before calling again. [CITED: GDACS API quick start, March 2025] The call without parameters answers 400 `{"message":"Eventtype is required."}`. [VERIFIED]

Live results: `events4app` answered 134 KB, 100 features in 3.1 s (only EQ and WF, all Green, none in India); a SEARCH for `TC;FL;EQ;VO;DR;WF`, 30 days, `orange;red` answered 25.6 KB, 15 features in 2.1 s (12 Orange, 3 Red). The combined `orange;red` filter worked here. [VERIFIED: live fetch] A third party client reports the combined alert filter as unreliable; if a test shows a missing level, query each level and dedupe on `eventid`. [CITED: ocha-lens docs, secondary]

Per feature, all points: `geometry.type` is `Point` on all 115 features seen (a polygon is behind `properties.url.geometry`; do not fetch it). `properties`: `eventtype`, `eventid`, `episodeid`, `name` ("Earthquake in Vanuatu Islands"), `description`, `alertlevel` (Green, Orange, Red), `alertscore`, `country`, `iso3`, `fromdate`, `todate`, `datemodified`, `iscurrent` (the string `"true"` or `"false"`), `severitydata{severity,severitytext,severityunit}`, and `url{report,details,geometry}`.

**Plan.**
1. Request A: `eventlist=TC;FL;EQ;VO;WF`, last 14 days, `alertlevel=orange;red`, keep `iscurrent == "true"`.
2. Request B: `eventlist=EQ;TC;FL`, last 3 days, all levels, then keep in Go only points inside the South Asia box (latitude 5 to 38, longitude 60 to 100), so a green M4.8 earthquake near the reader still shows.
3. Drop DR: a drought is one point for a multi country area, which would draw a misleading pin.
4. Run through the same RefreshIfDue every 15 minutes (own row in `official_feed_status`). Upsert by (`eventtype`, `eventid`); a pin leaves the map when `iscurrent` is false or `todate` is older than 2 days.

**India is usually absent.** The 100 current events held none in India; the 30 day orange and red search held one India flood and it was not current. [VERIFIED: live fetch] So GDACS will often draw nothing near an Indian reader. The Demo, not GDACS, is what makes the first visit look populated. This is why the cut list keeps GDACS last (D-12).

**Terms.** "We only request to acknowledge the source as 'Global Disaster Alert and Coordination System, GDACS'". Disclaimers: notifications for earthquakes, tsunamis and tropical cyclones are automatic and not reviewed by humans; "GDACS services are not meant to substitute nor to override any official information or alert message from local or national disaster management authorities"; no guarantee of completeness or timeliness. [CITED: GDACS API quick start; GDACS Terms of use March 2025] Both support the read only authority badge and the card text "Automatic estimate. Follow your state disaster management authority."

**Storage (D-15).** New table `official_hazards` (`event_type`, `event_id`, `episode_id`, `name`, `alert_level`, `country`, `latitude`, `longitude`, `from_date`, `to_date`, `is_current`, `severity_text`, `report_url`, `date_modified`, `first_seen_at`, `last_seen_at`), unique on (`event_type`, `event_id`). Never `reports`: that table needs a `session_id`, has no source column and feeds every vote path. A test asserts the `reports` row count is unchanged by a GDACS refresh.

**How the pin is drawn (D-12, ROBUST-06).** Crowd pins are `L.divIcon` category badges (`.icon-badge--pin`, `map.js` line 1596). The official pin gets its own `divIcon` class `official-pin`: a square (not round) badge with the hazard glyph and a fixed "GDACS" authority tag, in a neutral authority colour, on its own Leaflet layer group added from `/api/official/pins`. Rules: no vote controls, no popup that looks like the report popup, never counted in any "N reports" or the feed count, no reliability tag. The Demo mark (a visible "Demo" ribbon on crowd style pins) must be visually different from both. Card text comes from `textContent` only; the "Open GDACS page" link is allowed only if it starts with `https://www.gdacs.org/`.

### G5. The Demo

**Decision.** An in memory, per visit "world" that implements `service.Querier` and `service.VotingQuerier`, driven through the real `ReportService` and `VotingService`. No scratch schema, no duplicated JavaScript rules.

**Is the mechanic SQL bound?** No, and that decides everything.
- Pure, no I/O: `Resolve` (`visibility.go`), `BuildVoteTally`, `filterVoteRows`, `independentCellCount` (`trust.go`), `ConfirmedByPeople` (`confirmed.go`), `buildCurrency` (`currency.go`), `BuildTrust`, `withinVoteRadius` (`geo.go`). Their inputs are `[]sqlcgen.CurrentVotesForReportsRow`, a plain struct that can be built in memory. [VERIFIED: code read]
- Interface bound: `ReportService` takes `service.Querier` (12 methods: `InsertReportIdempotent`, `ReportByRequest`, `InsertModerationCheck`, `NearbyReports`, `CurrentVotesForReports`, `ReporterAccountsForReports`, `ReporterHistoryReports`, `ExpiredReportsInWindow`, `OpenReportsInArea`, `ReportForFocus`, `ResolutionUndoCounts`, `ReportExtrasForReports`) and `VotingService` takes `VotingQuerier` (`InsertVote`, `CurrentVotesForReports`, `ReportVoteContext`, `ReporterHistoryReports`, `ResolutionUndoCounts`). The `VotingQuerier` comment even says it is "small enough to fake without a real Postgres". [VERIFIED: `trust.go`, `report.go`]
- The only SQL semantics to reimplement: bounding box plus haversine and `expires_at > now()` for `NearbyReports`; `DISTINCT ON (report_id, account_id, kind)` newest row wins for `CurrentVotesForReports`; the expired window reads. That is a few hundred lines of Go.

**Options weighed.**

| Option | Real rule proven | Writes to DB | Cost | Verdict |
|--------|------------------|--------------|------|---------|
| A. In memory `Querier` and `VotingQuerier` per visit, real services on top | Yes: `CastVote` with the 1 km rule, self vote block, `Resolve`, `BuildTrust`, currency, trust copy | None | New package of about 500 lines; drift risk between the Go twin and the SQL | **Recommended**, with a differential test (below) |
| B. Per visit scratch schema in Postgres | Yes | Yes (DDL and rows per visitor); breaks D-05 "writes nothing to the database"; free tier connection and DDL limits | Medium | Reject |
| C. JavaScript simulation of the rules | No: a second authority, exactly what TRUST-02 forbids | None | Low | Reject |
| D. Extract a shared pure core | Already true: the core is pure today | None | Nothing to extract | Not needed |

**The drift guard.** Run one scripted scenario (seed, a post, six votes including a dispute and a resolve) through both the in memory world and a real scratch database, and compare the `Nearby` and `CastVote` outputs field by field (a Go e2e test on `p5_<plan>_a`). This keeps the Go twin of the SQL honest.

**Per visit state.** `GET /demo` creates a world (random 128 bit id from `crypto/rand`) in a `DemoStore` (`map[id]*World`, 30 minute idle TTL, at most 100 worlds, least recently used evicted; each world about 150 KB so the ceiling is about 15 MB). The page carries the id in `data-demo-visit`. Two ways to send it back, one line of tradeoff: a header `X-Demo-Visit` needs a header added at every real `fetch` call site (6 or more files), while putting the id inside the base path (`/demo/api/v/<id>/reports`, `api-base.js` building the prefix) changes only `api-base.js`. The id is not a secret (it guards only a sandbox with no data in it), so the **path form is recommended**. Never a cookie, so nothing persists. Reload means a new `GET /demo` and so a new world: "nothing is saved" holds by construction. Bound memory as well as request rate: per world at most 40 practice reports and 400 votes.

**Real services, demo wiring.**
- `ReportService` built with `NewReportService(world.Querier)` and **no screener** (`WithScreener` omitted), so the practice text is checked by the local gibberish prefilter only and goes to no vendor.
- Synthetic accounts: the visitor is account id `1_000_000_001`; pretend neighbours are `1_000_000_101` and up; seed voters use other ids. Ids never collide with real accounts and never reach a table.
- `SubmitChecked` stamps `created_at` from the wall clock and `InsertVote` carries no timestamp, so the in memory querier stamps votes from its own clock. Seeds are created relative to world creation ("20 min ago" is `now - 20 min`) by setting that clock to the past during seeding, then back. Report lifetimes are 24 hours for critical and 8 hours for the rest (`ExpiryDuration`), so a 30 minute Demo never sees a seed expire; the one Ended report is seeded with `expires_at` in the past inside the Past window.
- Handlers: write thin demo handlers in package `handlers` (`internal/api/handlers/demo.go`) so they can reuse the unexported mappers `reportViewToResponse`, `reportToResponse` and the vote response builder. `SubmitReport` and `CastVote` read the account and session from the request context, so reuse would need synthetic context values; calling the service methods directly is smaller. The JSON shapes must stay identical so the real JS works.
- The real map, feed, vote and trust JS can run on the Demo because the report endpoints are a handful of hard coded paths: `app.js` (2), `ended-nearby.js`, `flag.js`, `mute.js`, `votes.js`. Add one tiny `api-base.js` exposing `window.PinalertApiBase` (default `/api`, `/demo/api` on the Demo page) and change those call sites; update the contract tests that pin the literal strings. Build a new `demo.html.tmpl` that includes the same shared partials rather than editing `index.html.tmpl`, so the many tests that pin the real page stay as they are.

**What the Demo must switch off or fake (gotchas).**
1. **The 1 km rule (D-01 vs the rule).** Voting needs a position within `MaxVoteDistanceKm = 1` of the report, and the Demo never asks for location. Give the Demo one fixed pretend position (a point in Singh Nagar or Ajit Singh Nagar) that every vote sends. Seed about 10 reports within 1 km of it so the visitor can confirm and dispute, and about 10 farther away: voting on those returns the real "Too far to vote" message, which teaches the rule honestly. Do not exempt the Demo from the rule.
2. **Distinct cells for the neighbours.** The independence count is `COUNT(DISTINCT cell)` at geohash precision 7 (`voterGeohashPrecision = 7`: cells about 153 m tall and 125 to 151 m wide). Place neighbours at least about 250 m apart and from the practice report, all within 1 km, for example 300, 450, 600 and 750 m on four bearings. They need account ids other than the visitor's and other than each other.
3. **The climb to Confirmed.** `IndependentAgreementThreshold` is 2, so Provisional becomes Live and "confirmed by 2 nearby" after two neighbours; stop when `ConfirmCells >= IndependentAgreementThreshold` (read the constant, never write 2). A practice report is Provisional until then unless the visitor picks Critical, which the real `Resolve` publishes at once (the visitor still sees "Not confirmed yet" until people confirm, via `ConfirmedByPeople`). The demo script should use a non critical practice report to show the climb.
4. **Deterministic neighbours, no goroutines.** The script is a pure function of elapsed time: neighbour k votes at `postedAt + k x 4 s`. Every demo read first applies the votes that are due, through the real `CastVote`. The client polls `/demo/api/reports` every 3 seconds while a practice report is below Confirmed. This needs no timers per world, and it tests with a fake clock.
5. **No database at all, including `sessions`.** The router applies `deps.Session.Middleware(deps.persistSession)` to every route, and `persistSession` runs `UpsertSession`, so a cookieless Demo visitor would create a `sessions` row. Phase 3.3 plan 04 has already added `skipSessionFor(paths, next)` in `router.go` (read on main 2026-10-25): it skips the session middleware only for GET and an exact path in a set. The Demo needs POST and a path prefix, so Phase 5 must extend that function (or add a sibling `skipSessionForPrefix("/demo")`) and extend `TestSkipSessionFor`. The same function already serves the PWA assets, so reuse its test shape.
6. **Browser storage.** The Demo page carries no `data-account-id`, so `position.js` neither reads nor writes the stored fix (its documented rule). Verify the Demo never writes `pinalert.lowLastPosition` (a visitor who later signs in would otherwise start in Vijayawada), never starts `queue.js`'s IndexedDB queue, never calls `/api/geocode` (the search box is replaced by tap to place), and never registers the service worker prompt, the bell poll or Settings.
7. **Rate limit per IP (it has no account).** Reuse `ratelimit.NewPerIP(every, burst)` with `.Middleware()` on `GET /demo` (world creation, for example burst 30, one per 6 s) and a second one on `/demo/api/*` writes. Carrier NAT shares addresses, so keep these generous: the real defence against memory abuse is the world and per world caps, not the rate.
8. **Marking.** Every sample report, sample alert and practice report carries a visible Demo mark; the strip "Demo. Nothing here is real." plus Exit is in the demo template. Exit returns to `/login` or to `/welcome` step 1 (the visitor arrives with `?from=welcome`, validated against a two value allowlist, never an arbitrary URL).

**Routes and the gate (D-04).** Register in the exempt block of `router.go` beside `/login`, `/disclaimer`, `/policy`: `GET /demo`, `GET /demo/alerts`, `GET /demo/activity`, `GET /demo/api/reports`, `POST /demo/api/reports`, `POST /demo/api/reports/{id}/{confirm|dispute|resolve|reopen|unresolve}`. Update the doc comment above `NewRouter` (it lists the exempt set) and `TestAccessGateExemptRoutes` (its comment says "seven routes"; add the demo rows and a dated note "Amended by Phase 5 (D-04)"). Add a negative row: `/api/reports` and `/alerts` stay gated. The reused `flag.js` and `mute.js` call `/api/reports/{id}/flag`, `/mute` and `/unmute`; the exempt list above has no such Demo routes, so the Demo either hides the Flag and Mute controls (default, since Settings, the bell and phone alerts are not live in the Demo, D-05) or answers them with a no-op `204`. Pick one and pin it in a contract test, or they will 404.

**The 2024 Vijayawada sample.** Real place names, made up reports (D-02). Place names with a source behind them: Ajit Singh Nagar (worst hit, "16 wards were affected"), Krishnalanka, Bhupesh Nagar, Rajarajeswari Peta (chest deep water seen), Payakapuram, Singh Nagar, Ibrahimpatnam, and the Budameru breaches; the event is 31 August to 1 September 2024. [CITED: en.wikipedia.org/wiki/2024_Vijayawada_floods; theweek.in PTI 2024-09-01; thesouthfirst.com] Coordinates for these places were not verified this session. [ASSUMED] The plan must resolve each name once through the existing geocoder (Photon then Nominatim), store the verified coordinates in `internal/demo/seed.go`, and say so in the file header. Spread reports across the eight postable categories and all three severities as D-05 asks (15 to 25 reports, a few already Confirmed, one Disputed, one Ended). Texts are invented and carry no real person, no phone number and no street address. Sample official alerts for the Demo (2 to 3, marked Demo) are in the same Go file and flow through the same template as real ones.

### G6. The globe

**Decision.** Port the working prototype in `.planning/sketches/009-login-globe/index.html` (script about lines 560 to 758) into a login only `web/static/js/login-globe.js` and `web/static/css/login-globe.css`, load them only when Low data is off, and serve the NASA tiles through a small server side cache.

**Tiles.** `https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/BlueMarble_NextGeneration/default/GoogleMapsCompatible_Level8/{z}/{y}/{x}.jpeg`. Checked today: tile 0/0/0 answers 200, `image/jpeg`, 14,560 bytes, 256 x 256, `access-control-allow-origin: *`, an `ETag`, and `cache-control: max-age=0, no-store, no-cache, must-revalidate` with `expires: Thu, 1 Jan 1970`. The layer has no time dimension (`layer-time-actual` empty), so it is a static picture. [VERIFIED: live fetch]

**Style.** One raster source, `tileSize 256`, `maxzoom 3`, one raster layer, `projection: { type: 'globe' }`, `canvasContextAttributes: { contextType: 'webgl2', antialias: true, powerPreference: 'low-power' }`, `pixelRatio` capped at 2, `attributionControl: false`, `interactive: false`. Zoom 3 means at most 85 tiles (746 KB). The prototype ran in Chromium and WebKit with no errors and turned at 3.00 degrees a second. [VERIFIED: sketch 009 checks run 2026-10-17; not re run today]

**Weight (measured in the sketch, cold cache).** maplibre-gl js and css 285.6 KB compressed (shared with the map page, kept a year); the globe's own code about 6 KB; NASA pictures 101 to 249 KB in the first 9 seconds and about 378 KB for a full turn on a laptop; 746 KB is the most one visit can fetch. First visit about 540 KB on a laptop, against 31.9 KB for today's sign in page. Low data view and no WebGL2: 0 bytes, because the script and stylesheet tags are not written and the WebGL2 probe runs before any download.

**NASA's terms and a server copy (sketch item 21).** "Unless the content is marked with a use restriction or license, data provided from a NASA-led mission are licensed as Creative Commons Zero (CC0)"; "NASA material may be reproduced and distributed without further permission from NASA"; "NASA should be acknowledged as the source"; "NASA material may not be used to suggest or imply endorsement by NASA". [CITED: earthdata.nasa.gov data use policy] The GIBS docs ask clients to include: "We acknowledge the use of imagery provided by services from NASA's Global Imagery Browse Services (GIBS), part of NASA's Earth Science Data and Information System (ESDIS)". [CITED: nasa-gibs.github.io/gibs-api-docs] The GIBS pages say nothing on rate limits, caching or redistribution of tiles. The `no-store` header is a cache directive, not a licence term. So keeping a copy is permitted by the published policy. [CITED for the policy; ASSUMED for its application to this layer, since the layer page itself was not readable.]

**Recommendation: keep a server copy.**
- A route `GET /globe/tiles/{z}/{y}/{x}.jpg`, outside the gate (the page is pre login), accepting only the 85 valid indices (z 0 to 3, x and y in range), fetching each upstream once (fixed host, singleflight, 10 s timeout, 1 MB limit, `image/jpeg` check), keeping it in a `sync.Map` and answering `Cache-Control: public, max-age=2592000`.
- **No `sessions` rows from tiles.** A visitor without a cookie fetching up to 85 tiles would create up to 85 `sessions` rows through the global session middleware. The 85 paths are a closed set, so they go into the exact path set of the extended `skipSessionFor` (GET only). Add an e2e row: a tile fetch leaves the `sessions` count unchanged. (The `/login` page itself still creates its one session row today; that is existing behaviour and not Phase 5's to change.)
- Gains: repeat visits download nothing, visitors' addresses never reach NASA, a NASA outage after the first fetch does not matter, and the Low data promise gets cheaper. Costs: one public route and about 750 KB of memory; a cold start refetches.
- No image files in the repo: tiles live only in memory. If the upstream fetch fails the route answers 404 and the globe gives up quietly (the same path as any tile failure).
- The simpler alternative, drawing straight from NASA, costs about 100 to 380 KB per visit and needs no route. Take it only if the plan wants fewer moving parts.
- Credit line: "Earth pictures: NASA Blue Marble via NASA GIBS" linking to NASA's pages, plus the GIBS sentence in the Policy page.

**The UI-SPEC override.** Phase 1.1 UI-SPEC (`01.1-UI-SPEC.md` lines 143 to 171) banned WebGL, three.js and CDN libraries on the sign in page and asked for a self hosted SVG under 10 KB. The owner overrode it on 2026-10-17 (D-13). Record the override in the plan and in the amended pins; keep its reasons as rules: never block or move the form, nothing for Low data, a plan for devices that cannot draw it (here: no globe).

**What the build must follow from the sketch's "For the build" list** (line numbers there are from 2026-10-17 and are stale; re-grep):
- Load order: same unpkg URLs and SRI as `index.html.tmpl` (`maplibre-gl@5.24.0` js `sha256-RamwepGJzlYFTGIKlHzPQeKR5YyV6bYVM7dAqqZe5cs=`, css `sha256-qx5w1Z7EBGW65+cDDaLzzPKBM/1QLmK9WY7vut/XpzI=`), `crossorigin="anonymous"`, `referrerpolicy="no-referrer"`, injected after `load` plus idle (`requestIdleCallback` guarded, Safari has none), never in the head, never ahead of `auth.js`. Do not use the Leaflet bridge.
- Not in `theme.js` or any script the Low page links, and not in `auth.css` (Low links it and counts its bytes).
- Failure paths: probe `getContext('webgl2')` first; `try/catch` around the map; `error` before `load` turns it off; nothing drawn 10 seconds after download start gives up (the count waits while the tab is hidden); stop on `webglcontextlost`.
- Reduce motion read in script and followed live; hidden tab stops the loop; one turn per 2 minutes, India (longitude 80, latitude 18) centred at the start; opacity only on the `.globe-bg` wrapper (the one place `TestNoOpacityText` allows) with the value changed from 0.35 to 0.5 light and 0.4 dark and the test's reason rewritten.
- Removing the flat globe (`globe.svg`, `.globe-bg__mask`, `login_gate.html.tmpl` and `verify_outcome.html.tmpl` markup) touches `layout_rules_contract_test.go`, `low_contract_test.go`, `low_budget_contract_test.go` and `docs/MODERATION.md`.
- Policy text: `legal_body.html.tmpl` says the map uses unpkg and OpenFreeMap; the sign in page now contacts unpkg and the picture host (our own route if the cache is used, NASA otherwise) before anyone has agreed to anything. The owner picks the words.
- No geolocation and no `/api` call on the page; Referrer: a new screen that puts an email in its address would leak it to tile hosts (matters for Phase 8's code screen).

**What must survive Phase 8's replacement of the sign in card (D-14).** The ids `login-gate`, `login-form-panel` (or its successor) and the order "heading, field, button, Low data panel, links"; the globe markup placed inside `#login-gate` after `.login-column` so keyboards meet the form first; the quiet row `Try the demo · Disclaimer · Policy` in `.login-ways__links` (today `<p class="login-ways__links"><a href="/disclaimer">Disclaimer</a> <a href="/policy">Policy</a></p>`); the Low data panel words; the loader gating on `LowDataDefault`; the credit line outside the faded wrapper; the pinned tests for the globe tags, the WebGL probe, and the Demo link. Put the globe in its own template partial included by `login_gate.html.tmpl` so Phase 8 can replace the card without touching it.

### G7. The push seam (what 3.3 leaves, what Phase 5 adds)

Read on main 2026-10-25, while Phase 3.3 is landing. **3.3 is being built right now; every name below must be re-grepped at planning time.**

**What 3.3 leaves in place.**
- `internal/service/pushkinds.go`: `PushKindOfficial = "official"` and `PushKindFollow` as reserved names in `ReservedPushKinds`; `PushKind.Valid` refuses both "until that phase widens the list and its test". Caps constants (`PushPerAccountPerHour = 4`, `PushOutboxRetention`, `PushDevicesPerAccount`, ...).
- `internal/push`: `Sender` interface and `WebPushSender`, `Message{Payload, TTL, Urgency, Topic}`, `Payload{V, Kind, Tag, Title, Body, URL}` (`MaxPayloadBytes 1024`), `ValidTopic` (1 to 32 base64url characters), `DefaultEndpointPolicy`, `NewGuardedClient`, `LoadConfig` for `VAPID_*`.
- Migration `00011_add_push.sql`: `push_subscriptions` (unique endpoint, account, session), `push_outbox`, `push_report_progress`, `reports.area_name`, three `account_alert_settings` columns, `idx_accounts_home_area_cell`. The migration comment says "Phase 5 adds official and Phase 9 adds follow with their own migrations".
- `web/static/js/sw.js`: every push is shown with `silent: true`, no vibration, no sound field, `renotify: false`; a payload `tag` replaces an earlier notification of the same tag silently.
- Settings plumbing planned in 3.3 plans 12 and 14 and the first-time step 5 in plan 13, all with the slot for "Severe official alerts for my area" left undrawn, and the label kept in `waitingSettingsLabels` in `web/places_contract_test.go`.
- Planned but not yet on main (plan files only): `pushworker.go` (`PushWorker`, lease and backoff constants), `pushdispatch.go`, `pushrecipients.go`, `pushsubs.go`, `handlers/push.go`, `pushkinds`-based `BuildPushText` in `pushcopy.go` (it returns `ok false` for `official`).

**What Phase 5 must add.**
1. **Storage.** `push_outbox.report_id` is `NOT NULL REFERENCES reports(id)`, the kind CHECK is `('critical','reliable','changed','confirmed_update')`, and `push_outbox_update_shape` ties `changed` to `update_id`. An official push has no report, so it cannot use that table as is. Recommendation: a **sibling table `official_push_outbox`** (`account_id`, `alert_id` references `official_alerts(id)`, `cap_identifier`, `state`, `skip_reason`, `attempts`, `next_attempt_at`, `lease_until`, `created_at`, `expires_at`, `sent_at`) with a unique index on (`account_id`, `alert_id`). Reasons: no change to the type of `report_id` in generated sqlc code (3.3's worker and dispatcher read it as `int64`), no amendment of 3.3's constraints or tests, and an official lane that can have its own cap. The alternative (alter `push_outbox`: nullable `report_id`, new `official_alert_id`, `official` added to the CHECK, shape CHECK, new unique index) keeps one worker but ripples into every 3.3 query that selects `report_id`; take it only if the 3.3 worker is found to dispatch on kind through a single seam.
2. **Setting.** `account_alert_settings.official_push_on BOOLEAN NOT NULL DEFAULT true`, with `COALESCE(s.official_push_on, true)` in every read, a Go constant `DefaultOfficialPushOn = true` and a test pinning both (3.3's own pattern for the Critical default). The migration comment in 00011 already names this. PATCH `/api/me/alerts` gains `official_push`; Settings gains the row "Severe official alerts for my area" in the Notifications section (needs a home area, with the same "Set your home area to get these" state as the Critical row); first-time step 5 gains the second identical card "Severe official alerts for your area." (3.3 UI-SPEC line 144). Remove "Severe official alerts for my area" from `waitingSettingsLabels` and replace `TestNotificationsNoFollowOrOfficialRow` with its inverse.
3. **Kind.** Widen `PushKinds` with `PushKindOfficial` and move it out of `ReservedPushKinds`; amend `TestPushKindsClosed`. Add an `official` branch to `BuildPushText` with its own input struct. The words come from closed tables, never from feed text: severity word (Severe, Extreme), an event word from a closed map of CAP `event` strings ("Cyclone", "Heavy rain", "Flood", "Lightning", "Thunderstorm", "Heat wave", "Cold wave", "Tsunami", "Earthquake", anything else "Weather alert"), the district name from the boundary file (our data, not the feed's), and the issuer label from a closed table (IMD, CWC, state authority names). The headline and instruction stay on Official alerts. No noise words (3.3's `TestPushCopyHasNoNoiseWords` grid must include the new kind).
4. **Recipients.** Not 3.3's `PushHomeCandidates` (a distance filter around a report point). Select accounts with `home_area_cell IS NOT NULL`, a live subscription, and `official_push_on` true (`report_mutes` is per report and cannot apply to an alert, so there is no mute test); decode each distinct home cell once (cache `cell -> district codes` using the centre and four corners rule from G3); keep accounts whose district set meets the alert's `district_codes`. Row cap analogous to `PushRecipientsPerReport` (2000).
5. **What qualifies.** `status Actual`, `scope Public`, severity Severe or Extreme, placed at district level (see Open Question 1 for state only alerts), `sent` within the last hour, not expired and not within 10 minutes of expiry, feed state `fresh`. An Update triggers a new push only if the severity rose to Severe or Extreme or the district set grew to include the person's district.
6. **Replacement, not stacking.** Use a per alert Topic and tag: a stable 22 character base64url of the first 16 bytes of `sha256("official:" + guid)`, valid for `push.ValidTopic` and `payload.Tag`. An Update sends the same Topic, so a push service that honours Topic replaces an undelivered message, and the service worker replaces the shown notification (`renotify false`) without alerting again. **Apple caveat:** Phase 3.3 already found in place replacement unreliable on Apple endpoints: its plans name `IsAppleHost` (plan 02) and `TestWorkerConfirmedUpdateSkipsApple` (plan 11), and its confirmed update skips Apple hosts. Official Updates follow that same rule (re-grep it). Default: on an Apple endpoint an official Update that only changes wording is **not sent**; an Update that raises severity to Extreme or widens the district set to the person's district is sent as a second silent notification, because a missed escalation costs more than a duplicate. The owner may prefer "never send a second one"; the planner records the choice.
7. **Caps.** Its own: `OfficialPushPerAccountPer6Hours = 3` and a global `OfficialPushRecipientsPerAlert = 2000`. A push dropped by a cap shows nothing extra (3.3 decision 6). TTL for official: 2 hours, urgency `high` for Extreme, `normal` for Severe.
8. **Service worker.** `safeTarget` only allows `/?report=<digits>` or `/`. An official push must open Official alerts, so widen `safeTarget` to also allow the exact string `/alerts` (and extend `web/pwa_sw_test.mjs`). Payload `kind` `official`, `url` `/alerts`.
9. **The sender.** Reuse `push.Sender`, `PushSubscriptionsForAccount`, the 410 and failure handling and `PushBackoff` by calling them from a second small worker lane (`OfficialPushWorker`) that shares the existing interfaces. If 3.3's `PushWorker` exposes a per row send function, call it instead of copying.
10. **Enqueue point.** At the end of a successful SACHET refresh, in the same transaction that upserts the alert version (so a crash cannot lose a push or send it twice; the unique index is the second guard).

### G8. Migration numbering

Migrations on main today: 00001 to 00011. `00011_add_push.sql` (Phase 3.3) is present. 00009 is Phase 3.2, 00010 is the Phase 4 retest. Phase 5 must run `ls internal/store/migrations` at execution time and take the next free numbers; it expects 00012 and 00013 but must not hard code them. Phase 8 and Phase 9 will also add migrations and are later in the build order. Split into two files so the feed part can land and be verified before the push part, which depends on 3.3 being merged:

- First file (feed): `official_alerts`, `official_alert_areas` (alert id, `district_lgd` or `state_code`), `official_feed_status`, `official_hazards`.
- Second file (push): `official_push_outbox`, `account_alert_settings.official_push_on`.

Each file needs its own `TestMigration000NN` in `internal/store/migrations_test.go`, a goose Down in reverse order, no `CONCURRENTLY`, no `ON DELETE` on foreign keys (repo convention), and the table names added to `internal/testutil/db.go` `Truncate`. The rule 10 of the 3.3 plans (re-check the number first) applies.

### G9. NDMA's terms before wide promotion

- The RSS channel declares `<copyright>public domain</copyright>`. [VERIFIED: live fetch] That is the only machine readable statement I could find.
- The SACHET site has "RSS FEED" and "WebsitePolicy" pages (`/CapFeed`, `/WebsitePolicy`), but they are client rendered, so a plain fetch returns an empty shell and I could not read the policy text. A web search found nothing about SACHET terms. The site states its purpose as publishing "official warnings ... for the common people". [VERIFIED: fetched HTML] The page also says the portal "disseminates area-specific alerts to Indian Citizens from various Alert generating agencies", which supports redistribution but is not a licence. [CITED: sachet.ndma.gov.in/CapFeed]
- No written permission, rate limit or attribution rule was found. **Confidence LOW.** Required before wide promotion (a human step for the owner): open `https://sachet.ndma.gov.in/WebsitePolicy` in a browser, read it, and email NDMA's SACHET contact to ask in writing whether a free, unofficial, attributed relay is welcome and what rate is acceptable. Until then the product states plainly: "Alerts are copied from NDMA SACHET. Pinalert is not an official channel."
- Behave as a good client regardless: identify in the User-Agent with a contact, at most one RSS call per 5 minutes, conditional GET, CAP fetched once per version, no retry storm (exponential backoff with a 30 minute cap after failures), never send the feed text anywhere but our own pages.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library (`encoding/xml`, `encoding/json`, `net/http`, `compress/gzip`, `embed`, `crypto/subtle`, `crypto/rand`, `sync`) | go 1.26.4 per `go.mod` | RSS and CAP parsing, GDACS JSON, boundary file embed, refresh endpoint auth, per visit ids | The project already prefers stdlib for scheduled work and small tools. No new module is needed for this phase |
| `github.com/go-chi/chi/v5`, `jackc/pgx/v5`, sqlc, goose | as in `go.mod` | Routes, store, generated queries, migrations | Fixed by CLAUDE.md |
| `github.com/mmcloughlin/geohash` | as in `go.mod` | Decode the stored 5 character home cell to a centre and corners | Already used for the home area (`HomeAreaAnchor`) |
| `internal/push` (Phase 3.3) | as landed | `Sender`, `Message`, `ValidTopic`, guarded client | Phase 5 reuses it; do not build a second sender |
| `internal/ratelimit` (`NewPerIP`) | in repo | Per IP limits for the Demo routes | Already used for `/api/auth/request-link` |
| maplibre-gl | 5.24.0 (CDN, SRI pinned, already on the map page) | Globe projection on the sign in page | D-13; v6 has no browser global build |
| Leaflet | 1.9.4 (CDN, already loaded) | The GDACS pin layer on the Nearby map | Existing map framework |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `cmd/buildareas` (new, Go, stdlib) | n/a | Builds `internal/areas/data/districts.json.gz` from the upstream CC0 file | Run by hand when LGD changes; never at build or boot |
| `bsdtar` | macOS system | Unpack the upstream `.7z` | One time data preparation on the developer machine |
| GitHub Actions `schedule` | n/a | The external cron of G1 | Primary trigger; best effort |
| `node --test` | Node 22 or 24 in CI (local 26.4.0) | Browser module tests in `web/*.mjs` | Banner, alerts client, globe loader, demo client, service worker |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand written ray casting point in polygon | `github.com/paulmach/orb` (planar `Contains`, geojson) | A dependency for 25 lines; adds a module and a legitimacy check. Take it only if the planner prefers a tested library over our table tests |
| Embedded district file | A Postgres table of polygons | Needs a loader command, a migration and a query per lookup; embedded in memory is simpler and faster |
| In memory Demo world | Per visit scratch schema | See G5; rejected |
| GitHub Actions cron | cron-job.org or UptimeRobot (free) | A second vendor; no repo activity rule. Document as the backup if Actions is disabled after 60 quiet days |
| Server side NASA tile copy | Direct NASA tiles | See G6; direct costs 100 to 380 KB per visit and a NASA dependency per page view |

**Installation:** none. No new Go module and no new npm package. Data preparation:

```bash
# one time, on the developer machine, in a scratch directory (untrusted download)
gh release download districts -R ramSeraph/indian_admin_boundaries -p 'LGD_Districts.geojsonl.7z' -D "$SCRATCH/lgd"
bsdtar -xf "$SCRATCH/lgd/LGD_Districts.geojsonl.7z" -C "$SCRATCH/lgd/ex"
go run ./cmd/buildareas -in "$SCRATCH/lgd/ex/LGD_Districts.geojsonl" -out internal/areas/data/districts.json.gz
```

**Version verification.** `go version` here is go1.26.4 and `go.mod` says `go 1.26.4`; the boundary release was published 2023-12-11 and the file was read on 2026-10-25. maplibre-gl 5.24.0 is the version pinned in `index.html.tmpl` today.

## Package Legitimacy Audit

No external package is added by this phase, so there is nothing to run `package-legitimacy check` against: the Go work uses the standard library and modules already in `go.mod`, and the browser work uses the maplibre-gl 5.24.0 and Leaflet 1.9.4 tags already pinned with SRI. The one third party artifact is a **data file**, audited here by source:

| Artifact | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|----------|----------|-----|-----------|-------------|---------|-------------|
| `LGD_Districts.geojsonl.7z` (release `districts`) | GitHub release | published 2023-12-11, layer refreshed on bharatlas 2026-05 | n/a | github.com/ramSeraph/indian_admin_boundaries | OK for use as data | Approved: CC0 by the compiler, attribute Datameet and the government source; processed offline into a derived embedded file; the planner adds a `checkpoint:human-verify` on the licence wording before the first public share |
| NASA GIBS BlueMarble_NextGeneration tiles | gibs.earthdata.nasa.gov | long standing NASA service | n/a | NASA | OK | Approved: NASA policy CC0; keep the acknowledgement |
| GDACS events API | gdacs.org | long standing UN and EC service | n/a | n/a | OK | Approved: credit "Global Disaster Alert and Coordination System, GDACS" |

**Packages removed due to SLOP:** none. **Packages flagged SUS:** none. `paulmach/orb` is named only as an alternative; if chosen it must go through `package-legitimacy check` first and carry `[ASSUMED]` until then.

## Architecture Patterns

### System Architecture Diagram

```
                              (every 10 min, best effort)
 GitHub Actions cron ---POST /internal/official/refresh (Bearer)---+
                                                                   v
 time.Ticker (5 min, while awake) ----------------------> Refresher.RefreshIfDue(feed)
 lazy kick on /api/official/* or /alerts (non blocking) -->   |  skip if last_attempt < 4 min
                                                              |  singleflight per feed
          +---------------------------------------------------+------------------------------+
          v                                                                                  v
  SACHET: GET rss_india.xml (If-None-Match)                                  GDACS: SEARCH request A (orange;red)
   parse 100 items, keep new or changed guid/pubDate                         and request B (South Asia, any level)
   for <= 40 items, 3 at a time: GET CAP by guid (fixed host)                 upsert official_hazards
   parse CAP (status, scope, msgType, info per language)                      update official_feed_status('gdacs')
   choose English headline, expiry or 6 h fallback
   Place(): LGD codes -> districts | sender -> state | unplaced
   Update/Cancel/references handling
   upsert official_alerts + official_alert_areas  --(same tx)--> enqueue official_push_outbox
   update official_feed_status('sachet')                                      (Severe/Extreme, district level, fresh)
          |                                                                                  |
          v                                                                                  v
  Postgres: official_alerts, official_alert_areas, official_feed_status, official_hazards, official_push_outbox

 Reader (signed in) on Nearby:  JS sends lat,lon  ->  GET /api/official/alerts?lat&lon  (also drives #alert-banner)
      server: PIP(reader) -> district+state ; home cell -> district (centre + 4 corners)
              query alerts where expires_at > now and not cancelled and area matches
              attach feed state + age_seconds ; group, rank Extreme first ; build banner pick and "+N"
 Map: GET /api/official/pins -> Leaflet layer "official-pin" (never from /api/reports)

 Push: OfficialPushWorker claims rows (lease) -> BuildPushText(official) -> push.Sender (silent, Topic per alert) -> service worker -> /alerts

 Visitor (no account) -> GET /demo (rate limited per IP) -> new World in DemoStore (in memory, TTL, caps)
      -> demo.html.tmpl + same JS with PinalertApiBase=/demo/api, fixed pretend position, no storage, no geocoder
      -> /demo/api/reports  -> ReportService(memQuerier).Nearby   (applies due neighbour votes first)
      -> POST vote          -> VotingService(memQuerier).CastVote (real 1 km rule, Resolve, BuildTrust)
      -> POST /demo/api/reports -> SubmitChecked (no screener) -> neighbour script plans 300..750 m votes every 4 s
      nothing touches Postgres, not even sessions (skipSessionFor extended to /demo)

 Sign in page: HTML form first -> after load and idle, if WebGL2 and not Low data:
      load maplibre 5.24.0 (SRI) + login-globe.js -> style: raster source /globe/tiles/{z}/{y}/{x}.jpg (server cache of NASA GIBS)
```

### Recommended Project Structure

```
internal/
  official/            # Phase 5 feed engine
    refresher.go       # RefreshIfDue, singleflight, constants (FreshWindow, StaleLimit, caps)
    sachet.go          # RSS and CAP fetch, parse, normalise
    cap.go             # CAP structs, language pick, expiry rule, msgType rules
    place.go           # Place(): LGD codes, sender to state, tiers
    gdacs.go           # SEARCH requests A and B, mapping
    freshness.go       # State(feedStatus, now) -> fresh | stale | unavailable
    view.go            # reader view: area match, grouping, banner pick, "+N"
    pushenqueue.go     # recipients by district, enqueue, Topic derivation
    testdata/          # trimmed copies of the real CAP and RSS and GDACS samples (text only)
  areas/
    areas.go           # embed, decode, bbox prefilter, PIP
    data/districts.json.gz
  demo/
    store.go           # DemoStore with TTL, LRU, caps
    world.go           # World: clock, memQuerier, services
    memquerier.go      # service.Querier and service.VotingQuerier in memory
    seed.go            # Vijayawada sample, sample alerts, coordinates (verified once)
    neighbours.go      # deterministic neighbour script
  api/handlers/
    official.go        # GET /api/official/alerts, /pins ; POST /internal/official/refresh
    demo.go            # /demo pages and /demo/api handlers (reuse unexported mappers)
    globe.go           # GET /globe/tiles/{z}/{y}/{x}.jpg
cmd/buildareas/        # developer tool
web/templates/
  demo.html.tmpl, demo_strip partial, globe partial (included by login_gate), alerts.html.tmpl (filled)
web/static/js/
  api-base.js, official-banner.js, official-alerts.js, official-pins.js, login-globe.js, demo.js
web/static/css/
  login-globe.css, official.css
.github/workflows/official-refresh.yml
```

### Pattern 1: One idempotent refresh function behind three triggers

**What:** `RefreshIfDue(ctx, feed)` reads `official_feed_status`, returns at once if `last_attempt_at` is under the minimum gap, otherwise records an attempt, runs the fetch, and records success or a closed error word.
**When to use:** every trigger (cron endpoint, ticker, lazy kick) calls only this.

```go
// Sketch, not final code.
func (r *Refresher) RefreshIfDue(ctx context.Context, feed string) (Result, error) {
	v, err, _ := r.group.Do(feed, func() (any, error) { // singleflight: one run per feed at a time
		st, err := r.q.GetFeedStatus(ctx, feed)
		if err != nil {
			return nil, err
		}
		if r.now().Sub(st.LastAttemptAt) < MinRefreshGap {
			return Result{Skipped: true}, nil
		}
		if err := r.q.MarkAttempt(ctx, feed, r.now()); err != nil {
			return nil, err
		}
		res, ferr := r.fetchers[feed].Fetch(ctx) // returns a closed error word on failure
		if ferr != nil {
			_ = r.q.MarkFailure(ctx, feed, errorWord(ferr))
			return res, ferr
		}
		return res, r.q.MarkSuccess(ctx, feed, r.now(), res.ItemsSeen, res.ETag)
	})
	if err != nil {
		return Result{}, err
	}
	return v.(Result), nil
}
```

### Pattern 2: The server authors the state, the browser renders it

**What:** `GET /api/official/alerts` returns `state`, `age_seconds`, `last_success_at`, `in_area[]`, `home[]`, `state_groups[]`, `unplaced[]`, `banner{}`; the JS draws it and re-renders only the "N minutes ago" text from `age_seconds` plus its own elapsed time. Mirrors TRUST-02 and the existing feed rule.

### Pattern 3: Demo world on the real services

```go
// Sketch of the in memory seam, not final code.
type memQuerier struct {
	mu      sync.Mutex
	now     func() time.Time // world clock, set to the past while seeding
	reports []memReport
	votes   []sqlcgen.CurrentVotesForReportsRow // append only log, newest wins on read
}

// CurrentVotesForReports is DISTINCT ON (report_id, account_id, kind), latest created_at then id.
func (m *memQuerier) CurrentVotesForReports(_ context.Context, ids []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	// keep the newest row per (report, account, kind); order like the SQL so BuildTrust is identical
	...
}

var _ service.Querier = (*memQuerier)(nil)
var _ service.VotingQuerier = (*memQuerier)(nil)
```

### Pattern 4: Placement as one pure function with tiers

`Place(Alert, Areas) Placement{Tier, DistrictCodes, StateCodes}`; the reader match is a second pure function `Covers(placement, readerDistrict, readerState)`. Both are table tested with the real fixtures.

### Anti-Patterns to Avoid

- **Treating RSS absence as cancellation.** The window is 24 hours and 100 items; absence means the window moved. Retention follows CAP `expires` and `Cancel`.
- **Trusting `areaDesc` text for placement.** "6 Mandals" has no names; only codes and the trailing state are used.
- **Keying the boundary join on `dtcode11`.** It is the 2011 census code; use `dist_lgd`.
- **Putting official rows in `reports`** or counting them in any "N reports" figure (D-15, ROBUST-05).
- **Saying "no alerts" when the feed is stale or unavailable** (03.4 T-34-17).
- **Pushing feed text.** Push words come from closed tables; the headline lives on the page.
- **A Demo that special cases the rules.** No exemption from the 1 km rule, no fake counter: the neighbours vote through `CastVote`.
- **Putting the globe into `auth.css` or `theme.js`.** The Low page links both and counts their bytes.
- **Editing `index.html.tmpl` for the Demo.** Build `demo.html.tmpl`; the real page has dozens of pinned contract tests.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Web Push sending, VAPID, endpoint allowlist, payload cap | A second sender | `internal/push` from Phase 3.3 | Guarded client, endpoint policy and TTL rules already tested |
| Trust and visibility rules for the Demo | Any JS or Go copy of `Resolve`, threshold, 1 km rule | The real `VotingService.CastVote` and `ReportService.Nearby` on an in memory `Querier` | TRUST-02: one authority |
| Rate limiting the Demo | A new limiter | `ratelimit.NewPerIP` | Used and tested for `/api/auth/request-link` |
| Constant time secret compare | `==` on the token | `crypto/subtle.ConstantTimeCompare` | Timing safe |
| Per alert replacement of a shown notification | A new "dismiss old" call | Web Push `Topic` plus notification `tag` | Already supported by `push.Message.Topic` and `sw.js` |
| Map globe | three.js or a canvas globe | maplibre-gl 5.24.0 `projection: { type: 'globe' }` | Already loaded, SRI pinned, prototype proven |
| XML parsing | A hand scanner | `encoding/xml` with namespace free tags | Handles the `cap:` prefix and large inputs safely |
| Simplifying polygons | Per request geometry maths | Offline Douglas-Peucker in `cmd/buildareas` | Measured: 0.36 MB, 0.13 ms per full scan |

**Key insight:** the two pieces that look hardest, "run the real trust mechanic in a sandbox" and "place alerts without PostGIS", both reduce to small pure functions because earlier phases kept the rules pure and the SACHET team publishes LGD codes. The risk is not the algorithms but the seams: sessions, the gate, the 3.3 outbox shape, and honesty states.

## Common Pitfalls

### Pitfall 1: A sleeping host loses alerts and cannot push
**What goes wrong:** lazy refresh only runs for visitors; the 24 hour window moves on; a closed phone never hears about a cyclone.
**Why it happens:** Render spins down after 15 minutes without inbound traffic.
**How to avoid:** the external cron plus ticker plus lazy kick of G1; honesty states independent of the cron.
**Warning signs:** `last_success_at` gaps over 30 minutes in the status table; GitHub schedule disabled after 60 quiet days.

### Pitfall 2: The Demo, the cron call and the globe tiles create `sessions` rows
**What goes wrong:** the global session middleware upserts a session for every cookieless request, so "nothing is written" is false for the Demo, the GitHub cron `POST /internal/official/refresh` writes about 4,300 junk rows a month, and a cookieless visitor fetching globe tiles writes up to 85.
**How to avoid:** extend `skipSessionFor` once to cover the `/demo` prefix (GET and POST), `POST /internal/official/refresh`, and the 85 exact `/globe/tiles/...` paths (GET); a test counts rows in every table before and after a full Demo run, a refresh call and a tile fetch.

### Pitfall 3: Wrong code joins the district file
**What goes wrong:** keying on `dtcode11` matches nothing or the wrong district.
**How to avoid:** key on `dist_lgd`; a test resolves the real Tamil Nadu (23) and Maharashtra (3) code lists to the named districts.

### Pitfall 4: 45 near identical alerts bury the page
**What goes wrong:** AP SDMA lightning repeats every few minutes with the same sentence.
**How to avoid:** group by (sender, event, severity, English headline) and show a count.

### Pitfall 5: A state only Severe alert makes the banner permanent
**What goes wrong:** in Andhra Pradesh the lightning alert is Severe, state only, and nearly always in force.
**How to avoid:** Open Question 1 default: banner and push only for district level alerts, plus state level Extreme.

### Pitfall 6: Language codes are not standard
**What goes wrong:** `TL` for Telugu, `MR`, `en-IN`; a strict BCP 47 parse rejects them.
**How to avoid:** match `en` by prefix, treat everything else as an opaque label.

### Pitfall 7: The polygon URL is refused
**What goes wrong:** `FetchPolygonXMLFile` answered 403 twice.
**How to avoid:** not worked around and not required; placement uses codes and sender. Do not add browser header spoofing.

### Pitfall 8: Cold start fetch burst
**What goes wrong:** 100 unseen items means 100 CAP calls at once.
**How to avoid:** newest 40 per run, 3 concurrent, backoff; first run completes over 3 runs.

### Pitfall 9: Cron secret leakage or open refresh endpoint
**What goes wrong:** an unauthenticated refresh route lets anyone hammer NDMA through us.
**How to avoid:** 404 when unset, constant time compare, POST only, minimum gap, no body, log path only.

### Pitfall 10: The Demo leaks into real browser storage
**What goes wrong:** the pretend Vijayawada fix is written to `pinalert.lowLastPosition`, or the offline queue starts, so a later real sign in starts in the wrong place.
**How to avoid:** no `data-account-id` on the Demo page; contract test that `demo.html.tmpl` and its scripts never write that key or load `queue.js`.

### Pitfall 11: Service worker refuses the tap target
**What goes wrong:** `safeTarget` turns `/alerts` into `/`, so the official push opens Nearby.
**How to avoid:** widen it to the exact string `/alerts` and test it.

### Pitfall 12: The three kinds of pin look alike
**What goes wrong:** a GDACS pin, a Demo pin and a crowd pin share a silhouette; a visitor reads an automatic estimate as a crowd report.
**How to avoid:** distinct shape, "GDACS" tag, own layer, own card; a browser check at phone width in the iPhone simulator (Playwright WebKit passed things Safari showed broken, per the owner's note).

### Pitfall 13: The globe moves the card
**What goes wrong:** the globe arrives and shifts the form.
**How to avoid:** space and credit written with the page, `visibility: hidden` until ready, size from the sign in area, never a bare `100vh` (`chrome_css_contract_test.go`).

### Pitfall 14: `home_area_cell` is a 5 km square
**What goes wrong:** a centre point test misses an alert for the neighbouring district.
**How to avoid:** centre plus four corners, any match counts.

### Pitfall 15: Copy and house rules
**What goes wrong:** em or en dashes, noise words, emoji icons, or an underlined link slip into new templates, JS comments or push words.
**How to avoid:** the existing design and copy contract tests; add the new files to their lists.

## Code Examples

### CAP parsing with namespace free tags

```go
// Source: CAP 1.2 element names, verified against the four real messages read on 2026-10-25.
type capAlert struct {
	XMLName    xml.Name  `xml:"alert"`
	Identifier string    `xml:"identifier"`
	Sender     string    `xml:"sender"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	Scope      string    `xml:"scope"`
	References string    `xml:"references"`
	Infos      []capInfo `xml:"info"`
}
type capInfo struct {
	Language  string    `xml:"language"`
	Event     string    `xml:"event"`
	Urgency   string    `xml:"urgency"`
	Severity  string    `xml:"severity"`
	Certainty string    `xml:"certainty"`
	Effective string    `xml:"effective"`
	Onset     string    `xml:"onset"`
	Expires   string    `xml:"expires"`
	Headline  string    `xml:"headline"`
	Desc      string    `xml:"description"`
	Instr     string    `xml:"instruction"`
	Areas     []capArea `xml:"area"`
}
type capArea struct {
	Desc     string       `xml:"areaDesc"`
	Geocodes []capGeocode `xml:"geocode"`
}
type capGeocode struct {
	Name  string `xml:"valueName"`
	Value string `xml:"value"`
}
// Tags without a namespace match the cap: prefix in encoding/xml.
```

### Point in polygon (ray casting with holes)

```go
// Source: standard even odd rule; table test it with holes, multipolygons and edge points.
func inRing(x, y float64, r [][2]float64) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		xi, yi, xj, yj := r[i][0], r[i][1], r[j][0], r[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}
```

### Freshness

```go
func State(st FeedStatus, now time.Time) (state string, age time.Duration) {
	if st.LastSuccessAt.IsZero() {
		return "unavailable", 0
	}
	age = now.Sub(st.LastSuccessAt)
	switch {
	case age <= OfficialFreshWindow:
		return "fresh", age
	case age <= OfficialStaleLimit:
		return "stale", age
	default:
		return "unavailable", age
	}
}
```

### The cron workflow (sketch)

```yaml
# .github/workflows/official-refresh.yml
on:
  schedule: [{ cron: '*/10 * * * *' }]
  workflow_dispatch: {}
jobs:
  refresh:
    runs-on: ubuntu-latest
    steps:
      - run: |
          curl --fail --silent --show-error --retry 3 --retry-delay 30 --max-time 120 \
            -X POST -H "Authorization: Bearer ${{ secrets.OFFICIAL_REFRESH_TOKEN }}" \
            "${{ vars.PINALERT_BASE_URL }}/internal/official/refresh"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Flat spinning SVG globe on the sign in page | Real WebGL globe from NASA pictures in maplibre-gl 5 | maplibre-gl 5.0 introduced the globe projection | Owner approved 2026-10-17 |
| Mandal level gazetteer for AP alerts | LGD district codes in the CAP plus state from the sender | observed in the feed 2026-10-25 | A mandal list is not needed |
| Parsing area names from `areaDesc` | Using `geocode` `LGD District Code` | observed | Exact and language independent |
| Polling RSS only | RSS plus one CAP per item, RSS item has no severity | observed | Two step fetch |

**Deprecated or outdated:** the roadmap's "Alerts tab in the list panel" (replaced by the Official alerts place, D-06); the buzz wording of 2026-10-16 (replaced by silent notifications, D-11); sketch 009 line numbers (re-grep).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Fallback `expires_at = sent + 6 hours`, invalid beyond 7 days | G2 | An alert disappears early or lingers; both constants are tunable and pinned by tests |
| A2 | NDMA's Cancel messages follow the CAP 1.2 standard (`msgType Cancel`, `references`) | G2 | A cancelled alert may stay until it expires; no Cancel was observed in the 4 messages read |
| A3 | SACHET answers conditional GET with 304 | G2 | Only a wasted 79 KB download; nothing breaks |
| A4 | A free, attributed, unofficial relay is acceptable to NDMA | G9 | Legal and reputational; needs the owner's written enquiry before wide promotion |
| A5 | The compiler's CC0 dedication plus the national geospatial guidelines are enough for an internal, simplified, never drawn derivative of LGD boundaries | G3 | Licence challenge; mitigated by credit and the pre promotion legal review |
| A6 | Applying NASA's CC0 data policy to the BlueMarble_NextGeneration tiles permits a server copy | G6 | NASA could object; fall back to direct tiles (small code change) |
| A7 | GitHub Actions cron every 10 minutes is reliable enough; delays are tolerated by the 30 minute fresh window | G1 | Flapping stale state; widen the window or add cron-job.org as a second trigger |
| A8 | The 750 hour Render allowance stays at 750 and one always on service fits | G1 | Host suspends at month end; drop the cron to 30 minutes |
| A9 | Coordinates of the Vijayawada neighbourhoods are unverified | G5 | Wrong pins in the Demo; the plan verifies each through the geocoder once |
| A10 | Centre plus four corners is the right bias for home cells straddling districts | G3 | A few extra or missed pushes at district edges |
| A11 | The CWC `altitude` and `ceiling` fields are a latitude and longitude | G3 | None, because they are not used |
| A12 | SACHET's severity strings are only the CAP closed set (Extreme, Severe, Moderate, Minor, Unknown) | G2 | Only Severe and Moderate were observed; unknown values map to Unknown and never trigger the banner |
| A13 | A feed that returns 200 with zero items after a success with 10 or more is a failure | G1 | A real quiet day could look like an outage; the 10 item floor makes this unlikely given 100 items per 24 hours |

## Open Questions

1. **Do state only Severe alerts trigger the banner and the push?**
   - What we know: 45 of 100 current items are AP SDMA lightning alerts, severity Severe, with no district codes. If they count as "covering the reader's area", the banner is on for almost every Andhra Pradesh reader almost all the time, and D-11 pushes would be constant.
   - What's unclear: the owner's intent for D-07 and D-11 on state only alerts. D-09 only says they are listed under "Across <state>".
   - Recommendation (default until the viewing session): banner and push only for **district level** Severe or Extreme alerts, plus **state level Extreme**; state only Severe alerts appear in the Across group on Official alerts, Extreme first. Needs the owner's nod, in plain words, at the viewing session.
   - **ANSWERED 2026-10-25 (owner, "yeah do what you suggested"): the recommendation stands. Recorded as 05-CONTEXT D-16.**
2. **Can the feed show a cancelled alert?** What we know: one Update seen, no Cancel. What's unclear: real NDMA Cancel behaviour. Recommendation: build per CAP 1.2, add a live smoke test that logs unknown `msgType` values, review after a week of real traffic.
3. **Is the owner content with an always awake host?** The 10 minute cron uses 744 of 750 free hours. Recommendation: yes, because push needs it; offer the 30 minute cron as the cheap alternative.
4. **Server copy of the NASA tiles, or direct?** Recommendation: server copy (G6); the plan may choose direct if it wants less code.
5. **Does Phase 3.3's worker expose a per row send function Phase 5 can call?** Not on main yet. Re-grep `internal/service/pushworker.go` at planning; if it has a single dispatch seam, extending `push_outbox` becomes reasonable (G7).
6. **Where does Phase 5's "Try the demo" go for a signed in person?** D-03 puts it on step 1 of `/welcome`; confirm that `welcome.html.tmpl` has no change pending from Phases 3.3 and 8 when planned (3.3 plan 13 makes the flow five steps).
7. **Demo "My activity" depth.** Reusing the real Activity template with a demo view model is the expensive route; the minimal honest version lists the practice report and votes from the world. The planner picks the size; D-05 only requires those two facts.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | everything | yes | 1.26.4 (darwin arm64) | none needed |
| Node | `web/*.mjs` tests | yes | 26.4.0 locally (CI uses 22 or 24) | none needed |
| PostgreSQL client tools (`psql`, `createdb`) | e2e on a scratch database | yes | psql 16.14 | none needed |
| `sqlc`, `goose` | generated code, migrations | yes | in `~/go/bin` | none needed |
| `gh` | download of the boundary release | yes | `~/.local/bin/gh` | `curl -L` of the release URL (worked) |
| `bsdtar` | unpack `.7z` for `cmd/buildareas` | yes | `/usr/bin/bsdtar` unpacked the file | none needed (no `7z` binary installed) |
| SACHET, GDACS, NASA GIBS (network) | live smoke tests, builder | yes | all answered 200 today | recorded fixtures for CI |
| Docker | Linux CI check recipe | not probed | n/a | follow the repo's existing Linux CI recipe in the owner's notes |
| iPhone simulator and Mac Safari | phone checks | not probed | n/a | owner's UAT; Playwright WebKit is not enough on its own |

**Missing dependencies with no fallback:** none found.
**Missing dependencies with fallback:** NDMA's written terms (human step, G9).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (packages `internal/...`, `web` contract tests), `node:test` for `web/*.mjs`, manual and Playwright browser checks |
| Config file | none; CI runs `go build ./...`, `go vet ./...`, `node --test web/*.mjs`, then `go test -v -p 1 ./...` against Postgres 16 |
| Quick run command | `env -u DATABASE_URL go test ./internal/official/... ./internal/areas/... ./internal/demo/... ./web/ -short -count=1 && node --test web/*.mjs` |
| Full suite command | `go test ./... -p 1 -count=1` with `DATABASE_URL` pointing at a disposable `p5_<plan>_a` database only |

Never run against `pinalert_test`, `pinalert`, `p31_4_uat` or `p34_uat`; give every subagent its own database name and tell it so.

### Test kinds and what each must prove

| Kind | Where | Must prove |
|------|-------|-----------|
| Go unit, no database | `internal/official`, `internal/areas`, `internal/demo`, `internal/service` | CAP and RSS parsing against trimmed real fixtures (4 CAP messages, the RSS head); language pick (`en-IN` first, `TL` and `MR` as original); `Alert`, `Update` (with full identifier match), `Cancel`, `Ack` handling; expiry rule and 6 h and 7 day bounds; severity mapping; `status` and `scope` filters; placement tiers incl. the CWC and AP cases; sender to state table covers the file's `stname` set; ray casting with holes, multipolygon, edge points; reader coverage with centre plus four corners; grouping of the 45 repeats; banner pick, "+N", state hide; freshness state machine on a fake clock incl. the anomaly rule (zero items after a success with 10 or more); push words (closed tables, no feed text, no noise words, no dashes, within `PushTitleMaxRunes` and `PushBodyMaxRunes`); official Topic derivation passes `push.ValidTopic`; in memory querier semantics (`DISTINCT ON`, haversine, expiry, Past window); neighbour planner (distinct precision 7 cells, within 1 km, ids not equal to the visitor's, stops at `IndependentAgreementThreshold`); world and store caps and TTL |
| Go end to end on a scratch database | `internal/store`, `internal/api`, `internal/api/handlers` (own database per plan) | migrations up and down (`TestMigration000NN`); poller against an `httptest` fake SACHET and GDACS (ETag, 500, timeout, garbage XML, empty 200 anomaly, oversize body, off host link, bad guid); idempotent upsert and no duplicate rows over repeated runs; Update replaces, Cancel hides, expiry excludes; `reports` and `votes` row counts unchanged by any refresh; refresh endpoint 404 unset, 401 wrong token, 200 right token, minimum gap respected; `/api/official/*` gated (401) and the `state` and `age_seconds` honest across fresh, stale, unavailable; the sentence "no alerts in force" absent unless fresh and empty; push enqueue by district membership, `official_push_on` default true and opt out, no row for state only Severe, dedupe on unique index, Update replaces with the same Topic, per account cap; worker lane with a fake `push.Sender` (410 removes device, 429 backoff); **Demo writes nothing**: row counts of every table equal before and after a full Demo run, `sessions` included; the same `sessions` count is unchanged by a `POST /internal/official/refresh` call and by a fetch of a globe tile; Demo gate rows in `TestAccessGateExemptRoutes` plus negative rows; **differential test** of the in memory world against real SQL for one scripted scenario |
| Web contract tests | `web/*_test.go` | `#alert-banner` is one 44 px row with level icon, text, chevron, no wrap, whole row a link to `/alerts`, no underlined text link, colour plus icon; Official alerts template has the four states and the Last checked line; `login_gate.html.tmpl` has the row Try the demo, Disclaimer, Policy in that order and the globe partial after `.login-column`; welcome step 1 has a secondary Try the demo beside `#welcome-continue-1`; globe tags: URL, SRI, `crossorigin`, `referrerpolicy`, non blocking, not in the head, not before `auth.js`, absent when Low data; `auth.css`, `theme.js` and the Low page carry no globe; `.globe-bg` the only opacity; the demo template has the strip and Exit on every page, a Demo mark on every sample, no `data-account-id`, no `queue.js`, no write to `pinalert.lowLastPosition`, no `/api/geocode`; official pin class differs from `.icon-badge--pin`; `api-base.js` default `/api`; no em or en dash in any new file (`design_rules_contract_test.go` lists); policy page wording for unpkg, NASA, GDACS and LGD credits |
| Node tests | `web/*.mjs` | banner module formatting, "+N", hide on stale or unavailable; alerts client renders states and ticks "N minutes ago" from `age_seconds`; position source (Nearby's fix or searched place) sent as `lat`, `lon`; globe loader: WebGL2 probe first, nothing downloaded for Low data or no WebGL2, give up after 10 s, pause on hidden tab, still under reduce motion, never creates a second map; demo client poll cadence and neighbour climb; `sw.js` `safeTarget` allows `/alerts` and still rejects everything else, `optionsFor` still `silent: true` for the official payload |
| Browser checks (human, simulator and Mac Safari) | manual UAT, Playwright as a first pass only | the globe draws and turns, India first, no layout shift, still under reduce motion, zero requests in Low data view; banner and Official alerts at phone width in the iPhone simulator through `safaridriver` and `simctl` screenshots (Playwright WebKit passed layouts Safari showed broken); the Demo flow from the sign in link and from step 1: post, watch the count climb to Confirmed, dispute, too far message, Exit; GDACS pin distinct from crowd and Demo pins; a real installed phone receives a silent official notification and the tap opens `/alerts` |
| Live smoke (opt in) | `internal/official/live_test.go`, skipped unless an env flag is set, like `internal/geocode/live_test.go` | one real RSS and one real CAP parse; one GDACS SEARCH parse; logs unseen `msgType`, `severity` and `language` values |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ROBUST-05 | GDACS pins from own table, unaffected by votes, `reports` untouched | e2e + unit | `go test ./internal/official/ -run Gdacs -count=1` | Wave 0 |
| ROBUST-06 | Authority badge and pin distinct from crowd and Demo | web contract + browser | `go test ./web/ -run OfficialPin -count=1` | Wave 0 |
| ROBUST-07 | Demo populated, confirm and dispute work, neighbours reach Confirmed, writes nothing | unit + e2e + browser | `go test ./internal/demo/ ./internal/api/ -run Demo -count=1` | Wave 0 |
| ALERT-01 | District coverage from SACHET | unit + e2e | `go test ./internal/official/ -run 'Place|Cover' -count=1` | Wave 0 |
| ALERT-02 | Issuer, issued, until, expiry removal | unit + web contract | `go test ./internal/official/ ./web/ -run 'Expiry|AlertsPage' -count=1` | Wave 0 |
| ALERT-03 | Banner only Severe or Extreme covering the reader | unit + node | `go test ./internal/official/ -run Banner -count=1 && node --test web/official_banner_test.mjs` | Wave 0 |
| ALERT-04 | Last checked, stale and unavailable honesty | unit + e2e | `go test ./internal/official/ ./internal/api/ -run 'Fresh|Honest' -count=1` | Wave 0 |

### Sampling Rate

- **Per task commit:** the quick run command for the packages the task touched.
- **Per wave merge:** `go vet ./...`, `node --test web/*.mjs`, and the e2e set on the plan's own scratch database.
- **Phase gate:** full suite green on a scratch database, then the browser checks and the one real phone check before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/official/testdata/` trimmed real CAP samples (AP SDMA lightning, IMD Chennai with 23 codes, Maharashtra Update with `references`, CWC flood), an RSS head of 5 items, a GDACS SEARCH sample (text files only)
- [ ] Fake SACHET and GDACS servers (`httptest`) with ETag, failure, timeout and oversize modes
- [ ] `internal/areas` fixture: a tiny three district file plus a test that loads the real embedded file once
- [ ] `internal/demo` parity harness: scripted scenario runnable against both the in memory world and a real scratch database
- [ ] `internal/testutil`: add the new tables to `Truncate`, helpers to seed a feed status, an alert and an account with a home cell
- [ ] `web/` fixtures for the banner, alerts client, globe loader and demo client node tests
- [ ] The 3.3 push tests that pin the closed kind list and the settings rows must be amended with dated notes, not rewritten

## Security Domain

`security_enforcement` is on (ASVS level 1, block on high).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (the refresh endpoint secret) | Bearer secret from env, 32 or more random bytes, `ConstantTimeCompare`, 404 when unset, POST only |
| V3 Session Management | yes (the Demo, the cron refresh and the globe tiles must not create sessions) | `skipSessionFor` extended to `/demo`, `POST /internal/official/refresh` and the 85 tile paths; no cookie issued on those routes; e2e row asserts the `sessions` count is unchanged |
| V4 Access Control | yes | Demo routes exempt on purpose and pinned in `gate_test.go`; `/api/official/*` and `/alerts` stay behind the gate; the refresh route is outside the gate but behind its own secret |
| V5 Input Validation | yes | Fixed upstream hosts, guid pattern, body size limits, `encoding/xml`, length caps, control character strip, allowlists for Exit target and tile indices, strict JSON with `DisallowUnknownFields` on Demo POSTs |
| V6 Cryptography | yes | `crypto/rand` for world ids and the secret; never `math/rand`; no hand rolled hashing (Topic uses `crypto/sha256`) |
| V8 Data Protection | yes | The server keeps no reader location; `lat` and `lon` ride a GET query that the request logger does not record; no feed text in push payloads |
| V13 API | yes | New JSON routes carry swag annotations and join `swagger_test.go`; `application/json` only; errors in the existing envelope |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SSRF through the RSS `link` or a CAP `Polygon URL` | Tampering, Information disclosure | Build URLs from a validated guid; fixed host; https only; redirects refused; ignore every URL in a feed |
| XML bombs or huge bodies | Denial of service | `io.LimitReader` (1 MB, 256 KB), `encoding/xml` (no external entities), per run item cap |
| Stored XSS from alert, hazard or sample text | Tampering | `html/template` auto escape, `textContent` in JS, closed tables for push words, link allowlists (`https://www.gdacs.org/` only) |
| Alert spoofing by a poisoned feed or MITM | Spoofing | TLS to fixed hosts, `status Actual` and `scope Public` only; the page says "copied from NDMA SACHET, not an official channel" |
| Abuse of the refresh endpoint to hammer NDMA | Denial of service | Secret, minimum gap, singleflight, backoff |
| Demo memory exhaustion | Denial of service | World count and size caps, TTL, LRU, per IP limit, no goroutine per world |
| Demo confused with real data | Spoofing | Demo mark on every item, strip on every screen, no account data, separate API prefix, no writes |
| Push storm | Denial of service, annoyance | Per account 6 hour cap, per alert recipient cap, dedupe unique index, `fresh` only |
| Tile route abuse | Denial of service | 85 valid indices only, fixed upstream, singleflight, memory only |
| Referrer or address leak from the sign in page | Information disclosure | `referrerpolicy="no-referrer"` on the tags; our own tile route keeps visitors off NASA's logs |

## Sources

### Primary (HIGH confidence)
- Live fetches, 2026-10-25: `https://sachet.ndma.gov.in/cap_public_website/rss/rss_india.xml` (100 items, headers, fields); CAP messages for identifiers 1792919196941008, 1792915039196007, 1792897745415016, 1792859773702029; `FetchPolygonXMLFile` (403, twice); `https://www.gdacs.org/gdacsapi/api/events/geteventlist/events4app` and `/SEARCH?...`; `https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/BlueMarble_NextGeneration/default/GoogleMapsCompatible_Level8/0/0/0.jpeg`; `sachet.ndma.gov.in/` and `/CapFeed`, `/WebsitePolicy` (client rendered shells).
- Probe programs (Go) in the scratchpad: `rssstat`, `gdstat`, `distprobe2` (LGD join, point in polygon, simplification sizes, lookup timing).
- Repository code read this session: `internal/service/visibility.go`, `trust.go`, `confirmed.go`, `currency.go`, `geo.go`, `report.go`, `feed.go`; `internal/api/router.go`, `gate.go`, `gate_test.go`; `internal/session/cookie.go`; `internal/api/handlers/alerts.go`, `reports.go`, `votes.go`; `web/templates/index.html.tmpl`, `login_gate.html.tmpl`, `alerts.html.tmpl`, `welcome.html.tmpl`; `web/static/js/position.js`, `sw.js`; `internal/service/pushkinds.go`; `internal/push/payload.go`, `sender.go`; `internal/store/migrations/00008`, `00011`; `cmd/server/main.go`; `.github/workflows/ci.yml`.
- `.planning/` files: 05-CONTEXT, ROADMAP Phase 5, REQUIREMENTS, sketch 009 README and prototype, 03.3 CONTEXT, RESEARCH, UI-SPEC and plans 01 to 15.

### Secondary (MEDIUM confidence)
- GDACS API quick start (March 2025) and GDACS Terms of use (March 2025): `https://www.gdacs.org/documents/2025/GDACS_API_quickstart_v1.pdf`, `https://www.gdacs.org/documents/2025/GDACS_Terms_of_use_Mar_25.pdf`.
- NASA Earthdata data use policy `https://www.earthdata.nasa.gov/earth-observation-data/data-use-policy` and GIBS API docs `https://nasa-gibs.github.io/gibs-api-docs/`.
- Render free tier `https://render.com/docs/free`; GitHub Actions schedule `https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#schedule`.
- Boundary data: `https://github.com/ramSeraph/indian_admin_boundaries` (release `districts`, via `gh api`), `https://github.com/ramSeraph/indianopenmaps` DATA_LICENSE.md, `https://bharatlas.com/` (district and subdistrict layers, licences, sizes), `https://github.com/yashveeeeeeer/india-geodata`.
- Vijayawada place names: `https://en.wikipedia.org/wiki/2024_Vijayawada_floods`, `https://www.theweek.in/wire-updates/national/2024/09/01/mds19-ap-cm-ld-rains.html`, `https://thesouthfirst.com/news/wages-of-greed-choked-budameru-pays-back-vijayawada-with-rare-deluge-destruction-and-death`.

### Tertiary (LOW confidence)
- ocha-lens docs note that the combined GDACS alert level filter is unreliable (secondary, not reproduced).
- RainViewer page naming Payakapuram (marketing page, not used for anything binding).
- NDMA terms: nothing readable; see G9.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new dependencies, existing pins read in code.
- Feed shapes (SACHET, GDACS, GIBS): HIGH, fetched live and parsed this session; Cancel behaviour and 304 are ASSUMED.
- Boundary data: HIGH for join and size (probe); MEDIUM for the original government licence.
- Scheduler: MEDIUM, host and cron rules cited, not run end to end.
- Demo design: HIGH that the real services run on in memory queriers (interfaces read); MEDIUM on effort (about 500 lines plus a differential test).
- Push seam: MEDIUM, 3.3 is landing as this was written; names must be re-grepped.
- NDMA terms: LOW.

**Research date:** 2026-10-25
**Valid until:** 2026-11-08 for 3.3 names (fast moving); 30 days for the feed shapes and the boundary data.
