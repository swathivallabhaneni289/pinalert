# Official alerts and the Demo

This document is for whoever runs Pinalert, reviews it or reads the code. It says where the official alerts come from, how they get into the database, what the app is allowed to tell a reader about them, who is told on their phone, how the Demo works and why it proves the real trust rule, and what is still open. It describes what is built. The trust rules for crowd reports are in [TRUST-MODEL.md](./TRUST-MODEL.md); running phone notifications is in [PUSH.md](./PUSH.md).

Official items are read only. They are never counted in any crowd number, never carry a reporter tag, and never share a table with reports. Pinalert is an unofficial relay: alerts are copied from NDMA SACHET, and nothing here says NDMA has agreed to that (see "Limits and open questions").

## Sources and credits

| Source | What Pinalert takes | Terms as far as they could be read |
|--------|---------------------|-------------------------------------|
| NDMA SACHET, `sachet.ndma.gov.in` | The India RSS list and one CAP message per alert. It carries IMD, CWC and state disaster management authority alerts. | The RSS channel says `public domain`. No written terms for relaying could be read: the policy page is drawn by script and a plain fetch returns an empty page, and a web search found nothing. Confidence is low. This is the owner's first open step. |
| Local Government Directory district boundaries, compiled by Datameet and ramSeraph | 783 district shapes, keyed by the LGD district code that SACHET messages carry. Used only to test which district a point is in. Never drawn. | The compiler dedicates the file under CC0 and asks for credit to Datameet and the original government sources. The original publisher's licence could not be read, so confidence is medium. |
| NASA Blue Marble through NASA GIBS | 85 small Earth pictures for the sign in page. | NASA data is CC0 unless marked, may be reproduced without further permission, and NASA is to be acknowledged. The server keeps a copy of the pictures in memory only. |
| GDACS | Current hazard events (earthquake, cyclone, flood, volcano, wildfire) from its SEARCH API, two small requests per refresh. Drawn as official pins on Nearby and listed on Official alerts. See "GDACS hazard pins". | GDACS asks to be acknowledged as `Global Disaster Alert and Coordination System, GDACS`, and says its automatic estimates do not replace local or national authorities. |

Where the credits appear: the Policy page names SACHET, GDACS and the district file, and carries NASA's acknowledgement sentence; the sign in page carries a credit line under the Earth. The Policy page also says, before anyone agrees to anything, which hosts the sign in page contacts: unpkg.com for the map library, and Pinalert itself for the Earth pictures. The Policy wording is a draft the owner picks the final words of.

The District file is built offline by `cmd/buildareas` from the compiler's download (which is never kept in the repo) into `internal/areas/data/districts.json.gz`, about 345 KB. The package header of `internal/areas` repeats the source, the licence and the build recipe.

## How alerts are fetched

Two steps, always. The RSS list is a pointer only: each item gives a guid, a title in the issuer's language, the issuer's name, a link and a publication date, and nothing else. Severity, end time, area, codes and language live in the CAP message the guid points to, so every new or changed item costs one more request. The list is a sliding window of 100 items covering about 24 hours, so an alert missing from the list has not been cancelled; the window only moved on.

Bounds on what the server asks and accepts:

- The CAP address is built from the guid only after it matches 10 to 20 digits, on one fixed https host. The link in the feed is never followed. Redirects are refused.
- The RSS body is limited to 1 MB and a CAP body to 256 KB. Timeouts are 15 and 8 seconds, with one retry. At most 3 CAP requests run at once, and at most 40 per run, newest first. A cold start with 100 unseen items takes three runs and loses nothing, because the window is a day. A run that leaves items for the next one does not move "Last checked", so the list never reads as fresh while an unseen item may still be in force.
- A feed is asked at most once every 4 minutes whoever calls (`MinRefreshGap`), one call at a time, with conditional requests (ETag and Last-Modified). The stored validators move only after a run that handled every item it needed, so a 304 can never hide an unfetched item.
- Only a message that is `Actual`, `Public` and an Alert, Update or Cancel is kept. Exercise, Test, System, Draft and Private messages are dropped and remembered in memory so they are not fetched again every run.
- The request names Pinalert, the deployment's `BASE_URL` and `CONTACT_EMAIL`, so NDMA can reach someone.
- Stored text is untrusted: control and direction override characters are removed, lengths are capped, and the browser draws it with `textContent` only.
- A failed run records one closed word: timeout, status, parse, anomaly or network. Anomaly means the list parsed to zero items after a success that saw 10 or more; that is treated as a failure, not as a quiet day. A failure never moves the last good check.

Three layers ask for a refresh. All of them end in the same guarded function, so they cannot stampede NDMA:

1. An outside cron, the main one. `.github/workflows/official-refresh.yml` calls `POST /internal/official/refresh` every 10 minutes with `Authorization: Bearer <token>`. The call wakes a sleeping host. The route answers 404 when `OFFICIAL_REFRESH_TOKEN` is unset (and for every method but POST), 401 for a wrong token, and 429 past a per address budget. The token must be at least 32 characters or the server stops at startup. The call reads no input, has a 55 second deadline, answers a small summary of closed words, and writes no session row.
2. A ticker inside the server, every 5 minutes while the process is awake, and once at start.
3. A lazy kick: a read of the alerts starts a background refresh when the last attempt is more than 10 minutes old. It never delays the answer.

The limits of those layers, plainly:

- A free host that sleeps refreshes only when a request wakes it. The 10 minute cron keeps one instance awake around the clock, which costs about 744 of the 750 free instance hours in a month (31 days of 24 hours). A cron every 30 minutes lets the host sleep between runs and saves hours, at the price of a push that can be up to 30 minutes late plus a cold start. That is a cost and delay choice for the owner, not a correctness one.
- GitHub scheduled workflows are best effort. They can be delayed under load, they run only on the default branch, and in a public repository they are disabled after 60 days without repository activity. So the honesty rules below never depend on the cron having run.
- The workflow does nothing while the repository variable `PINALERT_BASE_URL` is unset. The owner adds the GitHub secret `OFFICIAL_REFRESH_TOKEN` and the variable, and sets the same token on the host.

Tests: `TestRefreshStoresAndPlaces`, `TestRefreshIsIdempotent`, `TestRefreshColdStartOverRuns`, `TestRefreshAnomaly`, `TestRefreshFailuresKeepLastSuccess`, `TestClientBuildsCAPURLFromGUIDOnly` and `TestClientRefusesRedirect` for the fetch; `TestRefreshEndpointToken`, `TestRefreshEndpointNoSession`, `TestRefreshEndpointRateLimit` and `TestRefreshEndpointUnsetStaysHidden` for the door.

## How alerts stay honest

The server decides how far the stored alerts can be trusted and sends the answer; the browser only draws it. The decision uses one fact: the time of the last good check (`last_success_at`), never the time of the last attempt.

| State | When | Official alerts page | Nearby banner | Phone |
|-------|------|----------------------|---------------|-------|
| fresh | last good check at most 30 minutes ago | the list, and Last checked N minutes ago | shown if the rule below says so | normal |
| stale | 30 minutes to 6 hours ago | a notice that the feed could not be checked since the last good time, then the last known alerts that have not ended, under Last known alerts | hidden | nothing new can arrive; queued notes are dropped unless the alert itself was seen in the last 30 minutes |
| unavailable | never checked, or more than 6 hours ago | a notice that official alerts could not be checked, with the last good time or never, and no list | hidden | none |

The two windows are named in one constant block, `FreshWindow` and `StaleLimit`, and a test pins them.

The rule that matters most: the page may say that no official alert is in force only when the state is fresh, the reader's area is known, and the list is empty. Every other case says something else, or says nothing reassuring. That sentence is written in one place in the script, never in the template, and a grid test runs it against every state. A stale view that still carried a banner is hidden by the script as well as by the server.

Other honesty rules:

- Time words come from the server's clock (the last good check plus the age the server sent), so a phone with a wrong clock still reads right. Only the time since the answer arrived is the phone's own.
- An alert leaves the page by its own end time or by a Cancel, not by missing from the feed.
- A body that is not shaped like the route's answer is a load error, never an empty list.
- Near identical alerts are grouped: the Andhra Pradesh authority sends the same Severe lightning sentence for different mandal sets every few minutes (45 of the 100 items on the day it was checked). They show as one row with "N alerts like this" and count once in "In force now".
- A made up Demo alert never wears the Official badge and never names SACHET as its source.

Tests: `TestFreshConstants`, `TestBuildViewStaleAndUnavailable`, `TestOfficialAlertsHonest`, `TestOfficialAlertsNoClaimInTemplate`, `TestAlertsNeverSaysNoAlerts`, `TestBuildViewGroupsNearIdentical`, and the whole path in `TestPhase5OfficialJourney`, which ages the feed to stale (banner gone, rows kept) and to unavailable (no rows) and checks that no report, vote or session row changed.

## Where an alert applies

District placement uses the codes SACHET messages carry (`LGD District Code`), joined to the embedded district file by code. Point in polygon is hand written Go, with a bounding box filter first.

| Tier | Input | Result |
|------|-------|--------|
| district | one or more district codes in any language block of the message | the districts, and their states |
| state | no codes, but a sender named `<State>-SDMA`, or a state name after the last comma or the later of the last " of " in the area text | the state only |
| unplaced | neither | no area; still stored and listed, in the last group only |

Mandals and tehsils are not placed. The commonest alert in the feed names no mandals at all ("6 Mandals") and has no codes, so a mandal gazetteer would have nothing to match. A polygon tier is not built; no message seen carried a shape inline.

Accuracy limit: the file is simplified at about 0.01 degree (roughly 1.1 km). A point within about a kilometre of a district edge can land in the neighbour. That is acceptable at district scale and is said here instead of hidden. District names are shown as the boundary file spells them, except for a small display table in internal/areas/names.go that fixes a few that read badly (the file's "Ntr" is shown as "NTR", a misspelt "Visakhapatanam" as "Visakhapatnam", and Delhi's bare compass words as "North Delhi" and so on).

How the reader's area is known: the browser sends the point Nearby is using as `lat` and `lon` on the request. The server uses it for that one answer and neither stores nor logs it. The home area is a stored 5 character geohash cell (about 4.9 km square) with no district and no coordinates. The server tests the cell's centre and its four corners and matches if any of the five points lies in an alert's district. That leans toward telling a person who may be covered, never away.

Groups on Official alerts, in order: Where you are (the reader's district), Your home area (when it is a different district), Across the reader's state, Across the home state when that differs, and Could not be placed on a map. An alert sits in the first group that fits and never twice. A state level alert is never dropped.

Tests: `TestPlaceTiers`, `TestBuildViewGroupsAndOrder`, `TestBuildViewHomeGroup`, `TestHomeAreaCorners`, `TestOfficialAlertsNeverLogsPosition`.

## The banner and the phone

The rule (D-16, decided by the owner on 2026-10-25): the Nearby banner and the phone note fire only for a Severe or Extreme alert that names the reader's district, or for an Extreme alert that covers the reader's whole state. A state wide Severe alert is listed under Across the state and never raises the banner or a note. Without that, a reader in Andhra Pradesh would see the banner nearly all the time and stop noticing it.

The banner is one slim line at the top of the Nearby list, 44 pixels tall, with a level icon, the event ("Cyclone warning"), the area, a small "+N" when more qualify, and a chevron. Level shows by colour, shape and word. It shows the newest most serious alert, only when the state is fresh, and the whole row opens Official alerts. It never scrolls and never wraps.

The phone note:

- Severe and Extreme alerts for the person's stored home area. On by default, with a switch in Settings, Notifications, "Severe official alerts for my area". It needs a home area, and the first-time flow does not ask about it.
- A district placed alert matches by district only; a state placed alert matches only when Extreme. An alert that is not Actual and Public, was sent more than an hour ago, has ended, or ends within 10 minutes never queues. An Update queues again only when it raised the level or widened the district set to include the person.
- The note is silent. The site asks for no sound and no vibration and sets no vibration pattern. It cannot force that: an iPhone follows its own notification settings.
- The words come from closed tables, never from feed text: for example "Cyclone warning for NTR district" and "Extreme. Issued by IMD." The longer headline and the instruction stay on Official alerts. A tap opens exactly `/alerts`.
- Its own lane: the table `official_push_outbox` and a worker next to the Phase 3.3 one, using 3.3's guarded sender and answer rules. Official notes never count toward 3.3's hourly limit. Their own limit is 3 per account in 6 hours, at most 2000 people per alert, and a life of at most 2 hours. Urgency is high for Extreme and normal for Severe.
- Replacement and the Apple choice (changed 2026-10-28 and 2026-10-29). Notes work per alert series. SACHET sends an Update under a new guid that names the earlier message in its references, so the series follows those references to its first alert. The Topic and Tag come from that first alert's guid, so on Android and desktop an escalation replaces the earlier note in place. An Apple endpoint ignores the tag, so it gets only the first push about a series, and the rest stays on Official alerts. The rule is per account and series, there is no per device send record, and it only ever means fewer notes.
- Everything is checked again at send time: the alert is still in force and not within 10 minutes of its end, the alert was seen or the feed was good within the last 30 minutes, the switch is on, the home area still matches, and the 6 hour limit has room. Nothing is sent from a feed that is not fresh.

Tests: `TestBuildViewBannerRule`, `TestOfficialQualifies`, `TestOfficialPushEnqueue`, `TestOfficialPushEnqueueUpdates`, `TestOfficialWorkerRechecksAtSendTime`, `TestOfficialWorkerAreaMatch`, `TestOfficialWorkerCapSixHours`, `TestOfficialWorkerSeriesAcrossGuids`, `TestOfficialPushSendsAfterRealRefresh`, and the whole route in `TestPhase5OfficialJourney`.

## Update, Cancel and expiry

- Alert: stored.
- Update: if it keeps its guid, the row is overwritten. If it arrives under a new guid, which is how SACHET sends it, it is stored as a new row and the alert its references name is marked superseded, so the page shows only the newer one. A wording only Update queues no phone note, because it is judged against the alerts its references name.
- Cancel: the referenced alert is marked cancelled and leaves every list at once. A Cancel under the same guid overwrites the row. The row is kept for the record. A withdrawn alert is never pushed, even when the Cancel is stored before the alert it withdraws (the refresher stores the newest message first).
- Ack and Error messages are ignored.
- Expiry: a valid `expires` (not before the sent time and not more than 7 days after it) is used. Anything else ends the alert 6 hours after it was sent, and the row says the end was inferred, so the page reads "Until about" and not a plain time. Both numbers are constants (`FallbackLifetime`, `MaxLifetime`) pinned by tests.

Real Cancel messages from NDMA have not been observed. Cancel handling follows the CAP 1.2 standard and is proved against a synthetic fixture (see "Limits and open questions").

Tests: `TestRefreshUpdateAndCancel`, `TestParseCAPKeep`, `TestOfficialPushEnqueueUpdates`, `TestOfficialSeriesQueries`.

## The Demo

A visitor with no account can try Pinalert on a practice copy of the 2024 Vijayawada floods. It is offered in a quiet row on the sign in page (Try the demo, Disclaimer, Policy), beside Continue on step 1 of the first-time flow, and in the box that appears when someone does not agree to the terms. The Exit button goes back to where the visitor came from.

What it is:

- The real Nearby, Official alerts and My activity, on invented data. Every sample report, pin, sample alert and practice report carries a Demo mark. Place names are real (resolved once through OpenStreetMap, stored in `internal/demo/seed.go`); the reports are made up and name no real person.
- The real services on an in memory twin of the database queries. `ReportService` and `VotingService` take small interfaces, so one world per visit implements them in memory and the real `CastVote`, `Nearby`, `Resolve` and trust wording run on top. No code is copied into JavaScript. The page asks the server for every sentence.
- A differential test keeps the twin honest: `TestDemoParityWithSQL` runs one scripted scenario through the in memory world and a real database and compares the answers field by field.
- Nothing is saved. A visit is a random id in a store of at most 100 worlds, each dropped after 30 minutes idle, with at most 40 practice reports and 400 votes. The Demo API lives at `/demo/api/v/<visit>/`, behind per address limits. It sets no cookie and writes no database row (`TestDemoWritesNothing`, `TestDemoPlacesWriteNothing`; the shared theme script still keeps one display preference, `pinalert_theme`, on a laptop width page, and nothing else). Leaving or reloading starts a fresh Demo.
- The 1 km rule is not switched off. The visitor has one fixed pretend position in Singh Nagar and no location question is asked. About half the seed reports are within 1 km; a vote on a farther one is refused with the real "Too far to vote" words.
- Pretend neighbours climb a practice report to Confirmed through the real rule. They vote every 4 seconds from places 300 to 750 m apart, each in its own precision 7 cell, so the distinct place count rises exactly as it would for real people, until the real threshold is met. The page polls every 3 seconds, so a visitor sees "Needs 2 people" at the start, the middle step at about 6 seconds and Confirmed at about 9.
- Sample official alerts are three made up alerts run through the same `BuildView` as real ones, then marked Demo. Their details say "Made up for the Demo. Not from any issuer." Their issuers are made up names that start with "Sample", never a real office.

Why this proves the real rule and not a picture of it: the numbers and the words the visitor sees are produced by the same functions that run on the live site, and a test fails if the twin and the SQL disagree.

Also on the sign in page: a turning Earth drawn from NASA Blue Marble pictures. The server fetches each of the 85 pictures once from NASA GIBS and serves them from memory at `/globe/tiles/`, so a visit does not reach NASA. Nothing is drawn or downloaded in the Low data view or on a device without WebGL2, and the form never waits for it. On phones the whole Earth sits above the card, which is frosted glass while the Earth is on and solid when the browser cannot blur.

Tests: `TestDemoParityWithSQL`, `TestNeighboursClimbToConfirmed`, `TestDemoRulesAreReal`, `TestDemoPracticeClimbOverHTTP`, `TestSampleView`, `TestDemoWritesNothing`, `TestGlobeTilesSetNoCookie`.

## GDACS hazard pins

GDACS is the Global Disaster Alert and Coordination System. It publishes automatic estimates of hazard events anywhere in the world, and says they are not reviewed by people and do not replace local or national authorities. Pinalert shows the current ones as official pins on the Nearby map, as read only items, and says so on every one.

What is fetched. Two small requests to the SEARCH API of `www.gdacs.org`, the only host the client ever calls. Request A takes Orange and Red events of the last 14 days (cyclone, flood, earthquake, volcano, wildfire) and keeps the ones GDACS marks current. Request B takes any level of earthquake, cyclone and flood of the last 3 days and keeps the points inside latitude 5 to 38 and longitude 60 to 100, so a Green earthquake near the reader still shows. Droughts are dropped, because a drought is one point for a whole multi country area and would draw a misleading pin. An event in both answers is stored once. If either request fails the whole run fails and nothing is stored, so the last good check never claims a check that half the run did not make. The two requests run at the same time, so a refresh takes as long as the slower one, and the first failure cancels the other. Redirects are refused, each answer is capped at 2 MB, each request has a 40 second limit, and a report link is kept only when it starts with `https://www.gdacs.org/`.

Storage and the gap. Hazards live in their own table (`official_hazards`) and the feed has its own status row (`gdacs` in `official_feed_status`). They never touch the reports, votes or alert tables. The feed is asked at most once every 15 minutes whoever calls (`GdacsMinGap`), because GDACS asks clients to save the collection and compare its date before calling again. The same cron, ticker and lazy kick that refresh SACHET refresh it, each feed behind its own gate, and the feeds are refreshed at the same time, so each has the whole 55 second budget of a call; the results come back in the fixed order SACHET then GDACS, and a panic in one feed is reported as the word network for that feed and never stops the other. The cron door sets its own write deadline (its refresh deadline plus 5 seconds) once the token is checked, so the cron gets its answer even when a refresh outlives the server's 10 second write limit, which still holds for every other route. A hazard with no end date counts as ended 48 hours after it was last seen.

Why the pin is a flat rectangle with a word and no colour. A reader on a phone can see three kinds of pin: a crowd report, a Demo report and a GDACS hazard. They must never be mistaken for one another, and not by colour alone, because red, orange and green already mean crowd severity. A crowd pin is a round badge on a severity fill. A Demo pin is drawn exactly like a crowd pin. The Demo is marked instead by the Demo strip on every Demo screen, the Demo tag on every row and card, and a pin name that starts with Demo. A GDACS pin is a rectangle 28 pixels high with a 4 pixel radius, never round, in the text colour with the page colour for its hazard glyph and the word GDACS, centred in a transparent 44 pixel marker box that is the touch target. It is the same size on a phone and a laptop. The chosen pin grows by a third and carries no ring. The level (Red, Orange or Green) is a word on the card, never a pin colour.

The card. Tapping a pin, or pressing Enter or Space on it, opens a read only card in the place a crowd report opens: the popup on a laptop, the full screen detail sheet on a phone, with the same close button, the same Escape and the same return of focus to the pin. It shows the GDACS badge, the hazard glyph and the level chip, the event name, Hazard, Issued, Until, Issued by GDACS, GDACS's own severity text when it sent one, the line "Automatic estimate. Follow your state disaster management authority.", the credit "Global Disaster Alert and Coordination System, GDACS", and one outlined 44 pixel button, Open GDACS page, only when the link starts with `https://www.gdacs.org/`. The button opens in a new tab with `noopener noreferrer`. The card has no vote control, no trust block, no reliability tag, no Flag and no Mute (Phase 3 D-10). The pin layer is its own Leaflet layer group, fed only by `GET /api/official/pins`, never by `/api/reports`; it is never in the Nearby list, never in "N reports" and never in any crowd number. All feed text enters the page through `textContent`.

Never the banner, never a push. GDACS rows are listed on Official alerts for a reader whose district they fall in, with the badge GDACS, but a GDACS item never raises the Nearby banner and never queues a phone alert. The fetcher has no path to the alert store or the push hook, and the banner pick skips every hazard, so this is structural and not a setting.

The stale rule. The pins route answers no pins unless the GDACS feed is fresh (the last good check within 30 minutes; stale up to 6 hours; unavailable after that or never). The script checks the state again, and a stale or unavailable answer, a failed request and an answer of the wrong shape all clear the layer, so an old hazard is never shown as current. The layer asks again every 5 minutes and when the reader moves the map to a new place, but never more often than every 5 minutes. A card already open is not closed by a refresh.

The separate status line. Official alerts keeps its own line for SACHET ("Last checked 4 minutes ago") and adds a second, muted line under it for GDACS: "Hazard events from GDACS: last checked 6 minutes ago", or "Hazard events from GDACS could not be checked since 2:05 pm.", or "Hazard events from GDACS have not been checked yet." The minutes move once a minute with no new request. The Demo page has no such line, because the Demo has no hazards.

Near an Indian reader GDACS often draws nothing, because current events rarely include India. That is correct. The Demo, not GDACS, is what makes a first visit look populated.

Tests: `TestOfficialPinDistinctFromCrowdAndDemo`, `TestOfficialPinCard`, `TestOfficialPinsNeverCounted`, `TestOfficialPinsScriptSinks` and `TestOfficialPinsAreLinked` (web contract); `web/official_pins_test.mjs` (the layer, the keys, the link allowlist, the stale clear); the hazard line cases in `web/official_alerts_test.mjs`; on the server side `TestOfficialPinsRoute`, `TestOfficialPinsHiddenWhenStale` and `TestOfficialPinsGated`.

## Limits and open questions

Open, and the owner's to close:

1. NDMA's terms. No written terms for relaying SACHET could be read, and nothing here says NDMA has agreed to it. Before wide promotion: open `https://sachet.ndma.gov.in/WebsitePolicy` in a browser, read it, and email NDMA's SACHET contact to ask in writing whether a free, unofficial, attributed relay is welcome and what request rate is acceptable. Until then the product says plainly that alerts are copied from NDMA SACHET and that Pinalert is not an official channel.
2. The district boundary licence. The compiler's CC0 dedication is clear; the original government licence could not be read. The file is used only for an internal test and never drawn. Confirm the credit wording and include it in the legal or mentor review that CLAUDE.md asks for.
3. The Policy words. The three added sentences (sign in page hosts, NASA's acknowledgement, the source credits) are drafts for the owner to rewrite. Changing the Policy text means moving the "Last updated" date and the terms version together, which asks everyone to agree again.
4. Real Cancel behaviour. It is unverified; the live smoke test (`OFFICIAL_LIVE=1`) can log unknown message types, and a week of real traffic would settle it.
5. An always awake host, or a 30 minute cron. See "How alerts are fetched".
6. Migrations 00012 and 00013 have not been applied to the owner's development database. That needs `make migrate`, after the owner says yes.

Limits that stand:

- Alerts that name only mandals, or only a state, cannot be placed more finely than the state. Their level shows on Official alerts under Across the state.
- A point within about a kilometre of a district edge can land in the neighbouring district.
- A sleeping host refreshes only when something wakes it, and GitHub's cron can be late or switched off after 60 days of no repository activity. The page says so plainly (stale, unavailable) instead of guessing.
- GDACS often draws nothing near an Indian reader, because the current events rarely include India. The Demo, not GDACS, is what makes a first visit look populated.
- The live GDACS feed was slow on 2026-10-29: the first live run timed out at the 15 second request limit it had then, and GDACS answered in 20 to 60 seconds that day. A quick fix the same day (quick 261029-6wb) raised the limit to 40 seconds and made the two requests, and the feeds, run at the same time. Its one live run the same day, on an empty database through the real cron door, answered in 8.5 seconds with GDACS read 104 features, stored 9 hazards and no error, beside a clean SACHET run. A probe that day also showed that sending the semicolons of the query encoded (`%3B`) or raw gives GDACS's identical answer, so the encoded form stays. GDACS can still be slow on a bad day; while the feed cannot be read, its state is unavailable and the map draws no GDACS pin, which is the correct and honest result. The pins and the hazard line themselves were checked in a browser only against seeded data.
- An iPhone follows its own sound settings, and an Apple device gets only the first push about an alert series.
- In the Demo, the browser's Back button to a previous Demo place starts a fresh Demo (a page takes its visit id off its own address on load). On a 320 pixel wide screen the banner's area text can be clipped to a few letters; the row stays one line and the full words are in its accessible name.
- Nothing here is a claim that the trust model is beyond abuse. A person who fakes a location, or runs several verified accounts, can still move a crowd count; that is written up in [TRUST-MODEL.md](./TRUST-MODEL.md).
