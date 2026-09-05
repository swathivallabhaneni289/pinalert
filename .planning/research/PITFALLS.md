# Pitfalls Research

**Domain:** Crowdsourced/crisis-informatics local emergency feed — anonymous geo-tagged reporting
with a confirm/dispute trust mechanic, deployed as a solo-dev public portfolio project (India,
floods/cyclones)
**Researched:** 2026-09-05
**Confidence:** MEDIUM-HIGH (technical pitfalls verified against PostgreSQL docs, CGNAT/rate-limit
industry writeups, and academic crisis-informatics literature; feature-interaction pitfalls derived
directly from this project's own PROJECT.md/PROJECT-NOTES.md scope, not external sources; India-
specific legal pitfalls verified against IT Rules 2021 primary-source summaries but are not a
substitute for an actual lawyer's opinion — flagged LOW confidence where noted)

## Critical Pitfalls

### Pitfall 1: Naive lat/lon "nearby" query that full-scans the table

**What goes wrong:**
The project deliberately skips PostGIS in favor of plain `latitude`/`longitude` columns and a
Haversine calculation in Go or SQL. Done naively — `SELECT *, haversine(lat, lon, :lat, :lon) AS
d FROM reports WHERE d < :radius` — Postgres computes the Haversine expression for every row
before it can filter, because there's no index it can use on a computed expression. This is fine
at 500 rows and demo-mode seed data; it silently becomes the slowest endpoint in the app as soon
as the reports table has a few tens of thousands of rows (which auto-expiry won't prevent, since
expired-but-not-deleted rows still count unless you actually purge them).

**Why it happens:**
"No PostGIS needed at this scale" (a correct call for a 2-4 week solo build) gets read as
"no indexing strategy needed at all," which is a different and wrong conclusion. The bug is
invisible in dev/demo because the dataset is small by construction.

**How to avoid:**
Two-stage filter, not a single Haversine pass: first cheap indexed bounding-box filter (plain
B-tree indexes on `latitude` and `longitude`, or a composite index, `WHERE latitude BETWEEN
:latMin AND :latMax AND longitude BETWEEN :lonMin AND :lonMax`), then Haversine only on the
already-narrowed candidate set to get exact distance and correct ordering. A bounding box alone
over-includes (corners of the box are farther than the radius), so the Haversine pass is still
needed as the final filter/sort — just on far fewer rows. This is a well-documented pattern for
exactly this scale (thousands to low millions of rows) that doesn't require PostGIS. (The clearest
public write-ups of this pattern happen to be MySQL-authored, but the bounding-box-then-exact-
distance technique is database-agnostic — it applies identically in Postgres with B-tree indexes.)

**Warning signs:**
`EXPLAIN ANALYZE` on the feed/map query shows `Seq Scan` on `reports` with the row estimate
growing linearly with table size; feed load time visibly increases after seeding a large demo
dataset or after auto-expiry accumulates months of rows without a purge job.

**Phase to address:**
Base reporting loop / data model phase — get the bounding-box-then-Haversine pattern and the
lat/lon indexes into the schema from the first migration, not retrofitted later.

---

### Pitfall 2: Race condition in the confirm/dispute tally — the Core Value silently breaks under concurrency

**What goes wrong:**
"Confirmed by N nearby" is the single feature this whole project exists to make trustworthy. The
naive implementation — read current count, add one, write it back, either as two round trips or
as an `UPDATE reports SET confirm_count = confirm_count + 1` racing against a separately-computed
diversity-weighted aggregate — loses updates under concurrent voting. During an actual surge (the
exact moment the feature matters most — many people near one incident voting within seconds of
each other) two confirms arriving concurrently can result in a stored count of 1 instead of 2, or
a diversity-weighted recompute reading a stale confirmations table. This is worse than a generic
counter bug here: an undercounted or inconsistent trust score is a correctness failure of the
product's one differentiator, not a cosmetic bug.

**Why it happens:**
Vote/hit counters are the textbook example of a lost-update race, and Postgres's default READ
COMMITTED isolation does not prevent it by itself for read-modify-write patterns split across
application code. It's easy to test single-user and never notice because the race only manifests
under true concurrency.

**How to avoid:**
One rule, not two competing ones: the append-only `confirmations` table (one row per voter per
report, already in the planned data model) is always the single source of truth. Any denormalized
`confirm_count`/diversity-weighted cache column exists purely as a read-performance optimization
and must be updated atomically *inside the same transaction* as the vote insert (either a
single-statement `UPDATE ... SET x = x + delta WHERE ...`, or a `SELECT ... FOR UPDATE` on the
report row before recomputing), never via a separate read-then-write round trip from application
code. Because the cache is always rebuildable from the source table, a periodic reconciliation job
(or a one-off backfill if drift is ever suspected) is a safety net, not the primary mechanism.

**Warning signs:**
Confirm count doesn't match `COUNT(*)` from the confirmations table under a concurrent load test
(even a simple script firing 20 parallel confirms at one report and asserting the final count is
exactly 20); manual QA won't catch this — it requires an explicit concurrency test.

**Phase to address:**
Trust mechanic / confirm-dispute phase — write a concurrency test for the tally as part of that
phase's UAT criteria, not deferred to a "hardening" pass later.

---

### Pitfall 3: IP-based rate limiting mistakes shared carrier IPs for single abusers (and vice versa)

**What goes wrong:**
The access model is anonymous + IP rate-limited by design (correct call for emergency use — no
signup friction). But in India, mobile carriers overwhelmingly use Carrier-Grade NAT (CGNAT),
meaning hundreds to thousands of real phones share one public IP. A per-IP rate limit tuned to
stop one abusive actor either (a) is so loose it does nothing against a determined abuser rotating
within the same NAT pool, or (b) is tight enough to matter and then blocks/throttles dozens of
genuine nearby reporters during exactly the high-report-volume moment (a flood event) when many
real people in the same neighborhood — behind the same cell tower's NAT — are trying to report
simultaneously. This is the opposite of the intended effect: the anti-abuse mechanism suppresses
legitimate signal during the exact surge the product exists for.

**Why it happens:**
IP-based abuse control assumes one-IP-to-one-user, which is a poor assumption for mobile traffic
specifically (as opposed to home broadband), and India has very high mobile-first usage for this
kind of app.

**How to avoid:**
Rate-limit per anonymous session (the already-planned cookie/localStorage session id) as the
primary key, with per-IP as a secondary, looser backstop only for unauthenticated bulk/scripted
abuse (e.g., no-cookie requests). Set per-IP limits noticeably higher than "reports per person per
hour" would suggest, specifically because it's known that many legitimate users share an IP.
Combine with the planned confirm/dispute diversity weighting (already keyed on session, not IP)
as the real anti-gaming layer, rather than leaning on IP throttling to do that job.

**Warning signs:**
During load-testing or after any real traffic spike, a cluster of genuine-looking reports from
plausible nearby coordinates all fail with 429s; conversely, if abuse testing shows one script can
still flood the feed by simply not sending cookies, IP limiting alone isn't sufficient.

**Phase to address:**
Robustness/anti-abuse phase (rate limiting + auto-moderation) — decide the session-first,
IP-secondary limiting model explicitly rather than defaulting to "just rate-limit by IP" as the
whole strategy.

---

### Pitfall 4: Trust score gamed via trivially-resettable anonymous sessions (Sybil attack on the Core Value)

**What goes wrong:**
The confirm/dispute mechanic, diversity weighting, and reliability score all key off an anonymous
session id stored client-side (cookie/localStorage). Any of these is one "clear cookies" or
incognito tab away from a fresh, zero-history session with a new geohash "identity." Combined
with the fact that a phone can trivially change its browser-reported location (dev tools geolocation
override, or simply moving the phone/spoofing GPS), a single motivated person can stack multiple
"independent" confirmations on their own report, defeating exactly the provisional-visibility-gate
and diversity-weighting mechanisms designed to prevent this.

**Why it happens:**
No-signup anonymous posting is a correct and deliberate product decision (friction is unacceptable
during an emergency) but it means there is close to zero cost to generating a new "independent"
identity, which is the precondition every fraud-resistant trust system (Community Notes, Waze,
Wikipedia Pending Changes) actually depends on holding at least weakly true.

**How to avoid:**
Don't aim for unbeatable — aim for "costs more effort than the value of gaming it," which is the
realistic bar for a portfolio project's core mechanic. Concretely: rate-limit *report+confirm
combos from the same IP within a short time window* even across different session ids (raises the
cost of the cookie-clear attack); treat "confirmation from a session created in the last N minutes"
as lower-weight in the diversity calculation than an aged session (a same-day-created burst of
"independent" confirmers is itself a signal); log and expose (even just in your own README/demo
narrative) that this is a known, documented limitation with a stated mitigation — for a portfolio
project, showing you understood and reasoned about the Sybil problem is worth more than pretending
it's solved. Do not claim in copy/UI that "confirmed by N nearby" is fraud-proof.

**Warning signs:**
None will show up in normal manual testing — this requires deliberately red-teaming your own
mechanic (open 5 incognito windows, submit a fake report, confirm it from all 5 with slightly
different mock coordinates) before considering the trust-hardening phase done.

**Phase to address:**
Trust-model hardening phase — this is precisely the phase whose stated purpose is protecting the
Core Value; add "red-team your own confirm mechanic" as an explicit phase verification step, not
just unit tests of the happy path.

---

### Pitfall 5: Timezone and expiry-window bugs silently show stale reports as current (or hide live ones)

**What goes wrong:**
Auto-expiry ("reports fade/archive after N hours unless re-confirmed") depends entirely on
correct timestamp handling. Common concrete bugs: storing `created_at`/`expires_at` without a
timezone in Postgres (`timestamp` instead of `timestamptz`) so the app server's local time zone
silently shifts the meaning of "N hours old" depending on deploy environment (dev machine on IST,
Railway/Render container on UTC); computing "time ago" display strings in JS using the browser's
local timezone against a server timestamp with a mismatched assumed zone, so "3 min ago" reads
wrong; and off-by-one-day errors around midnight IST if any date-only (no time) comparisons creep
in for daily digest/report-age logic.

**Why it happens:**
Solo dev building fast, testing from one machine/timezone (IST) against a database that may be
deployed with a different default timezone (Railway/Render Postgres often defaults to UTC); the
bug is invisible until deploy, or until a specific hour-of-day boundary is crossed during testing.

**How to avoid:**
Use `timestamptz` (not bare `timestamp`) for every time column from the first migration; always
compute "is this expired" server-side using the database's `now()` compared against a stored UTC
instant, never by shipping a naive local time to the client and having the client decide; write
one explicit test that creates a report with `expires_at` a few seconds in the future and asserts
it disappears from the feed after that time, run in CI (which typically runs in UTC) — this alone
would catch most instances of this class of bug.

**Warning signs:**
A report that should have expired is still showing (or vice versa) specifically after a deploy or
specifically depending on what time of day (IST) testing happens; CI passes but manual local
testing shows different expiry behavior than staging/production.

**Phase to address:**
Base reporting loop phase (schema decision — `timestamptz` from day one) and auto-expiry
implementation phase — add the "expires in N seconds, then check it's gone" test as an explicit
UAT/verification criterion for that phase.

---

### Pitfall 6: Anonymous public posting becomes an unmoderated vector for panic, harassment, or illegal content — and "confidence-cascade moderation" is under-specified until it's tuned

**What goes wrong:**
Anonymous, no-signup, geo-tagged public posting during a real (or perceived) disaster is a
specifically attractive target for: deliberate false "rescue needed" reports (cruelty/prank,
documented in real disaster social media), targeted harassment using a real address as a fake
"rescue needed" or "flooding" location, and — the most severe case — CSAM or other illegal content
uploaded via the (currently deferred, but planned) photo attachment feature. The plan already
calls for a "confidence-cascade" auto-moderation calibrated to a false-positive budget, which is
the right idea, but it is not a plug-and-play feature: a text-toxicity API tuned for social-media
harassment detection is not tuned for "is this a plausible false disaster report," and getting the
false-positive/false-negative tradeoff wrong in either direction either lets real harassment
through or auto-hides genuine urgent reports (the worst possible failure mode for an emergency
app). There is no such thing as a well-calibrated moderation cascade on day one — it needs real
(or realistically simulated) abuse traffic to tune against, which a fresh public deploy won't have.

**Why it happens:**
Moderation APIs (Perspective API, cloud vision safety APIs, etc.) are trained on general web/social
content, not disaster-report-specific false positives (e.g., the word "help," "urgent," ALL CAPS,
addresses, and phone numbers are extremely common and legitimate in genuine emergency reports, but
also common patterns in spam/scam detection heuristics).

**How to avoid:**
Ship a human-reviewable moderation queue (even a minimal internal-only page) from day one alongside
the automated cascade — don't rely on the auto-moderation threshold alone before it's been tuned
against real traffic. Explicitly test the cascade against a small hand-written set of realistic
genuine emergency reports ("HELP flooding rising fast need boat at [address]") to confirm they are
NOT auto-hidden, before testing that it catches abuse. Treat photo upload (already a deferred
stretch feature) as materially higher-risk than text — if/when it ships, it needs a
image-safety API call in the request path before storage, not after, and a documented takedown
path, given planned public deployment.

**Warning signs:**
Any report containing urgent/all-caps/address-like text getting auto-hidden in testing is a sign
the cascade threshold is miscalibrated in the dangerous direction (suppressing real urgency, not
just failing to catch spam).

**Phase to address:**
Robustness/auto-moderation phase — explicitly budget time for calibration testing against
hand-written realistic-report fixtures, not just "call the API and pick a threshold."

---

### Pitfall 7: "Public live demo, zero real users" makes the empty-map problem worse than expected, and demo/seed data can be mistaken for real reports

**What goes wrong:**
A crowdsourced app with zero real traffic shows an empty map to every visitor, which for a
portfolio project reviewed by a recruiter/interviewer reads as "broken" or "doesn't work," not
"correctly empty." The project already (correctly) promotes demo/replay mode and the GDACS
official-feed poller to core scope specifically to solve this — but a second, related pitfall
often gets missed: if seeded historical/demo data and real official-feed/crowd data are not
*visually and structurally* distinguishable at all times (not just at load, but persistently),
a real visitor during an actual flood could mistake three-year-old demo-mode Chennai-flood seed
pins for live current reports, or a reviewer could mistake official GDACS pins for the app's own
crowd-verification working when it's actually just relaying an external feed untouched by the
trust mechanic at all.

**Why it happens:**
Demo data and real data share the same `reports` table and rendering path by design (that's the
point — the app should "just work" the same in both modes), which makes it easy to forget to keep
a persistent, unmissable visual distinction between crowd-verified, official-feed, and
demo/seed sources.

**How to avoid:**
Make `source` (crowd/official/demo) a first-class, always-visible UI distinction — not just a
database column — e.g., distinct pin styling per source, an explicit "DEMO DATA — for
illustration" banner/mode toggle rather than silently mixing demo pins into what looks like a live
feed, and a visible timestamp on every pin so staleness is self-evident. Keep demo/replay mode as
an explicit, separately-entered mode (a toggle or a `/demo` route) rather than auto-seeded rows
sitting permanently in the same feed a real user would see live, if at all avoidable — the
project's own requirement list already implies this ("simulate a report" button, seeded timeline)
but the roadmap should make explicit that demo mode is *switchable*, not permanently blended.

**Warning signs:**
Screenshot the live production feed months after seeding, with zero real traffic — if it's
impossible to tell at a glance which pins are demo, which are official-feed, and which (if any)
are real crowd reports, the distinction has degraded.

**Phase to address:**
Demo/replay mode phase and official-feed integration phase — treat "source is always visually
distinguishable" as an explicit UAT criterion for both, not just "data displays on the map."

---

### Pitfall 8: Unofficial emergency app legal exposure — implied authority, and India-specific intermediary-liability gaps

**What goes wrong:**
Two distinct legal risk surfaces, both real for a publicly deployed, real-looking emergency app
with a real domain and polished UI: (1) **implied-authority risk** — a well-designed map with an
"official" source badge, severity levels, and rescue-needed categories can be mistaken by a
stressed user for an authoritative/government service, especially if found via search during an
actual event; a user who acts (or fails to act, e.g. doesn't call real emergency services) based on
a false or stale crowd report and comes to harm is a scenario the project needs to have explicitly
designed against, not just disclaimed after the fact. (2) **intermediary/UGC liability in India**
— anonymous, unmoderated-until-reviewed user-generated content posted publicly triggers India's
IT Act 2000 / IT Rules 2021 intermediary framework; safe-harbor protection is conditional on due
diligence (a grievance-redressal mechanism, and content takedown expeditiously — currently 36 hours
— upon actual knowledge via a court/government order), which is a materially higher bar than "we
have a disclaimer in the footer." *(This point is legal-domain and specific-jurisdiction —
confidence LOW; treat as "get this checked with an actual lawyer or knowledgeable mentor before
wide public promotion," not as legal advice.)*

**Why it happens:**
"It's just a portfolio project, no real users yet" creates a false sense that legal/authority-
implication concerns don't apply yet — but the app is deployed live and publicly indexed, meaning
it is discoverable during a real event regardless of intended audience, and intermediary
obligations attach to what the platform does (host public UGC), not to how many users it currently
has.

**How to avoid:**
Ship the already-planned disclaimer, but make it structurally load-bearing, not just a footer
line: a persistent, hard-to-miss banner (not just a linked page) stating unofficial status and "call
[emergency number] first," shown on first visit and re-shown periodically; a real, working
grievance/report-abuse contact path (even a monitored email) rather than none, since that is the
actual due-diligence bar, not merely a disclaimer's existence; avoid any visual design choice that
mimics official government/NDRF branding, seals, or color schemes; keep the "official" badge/source
distinction (Pitfall 7) doing double duty here — a report that says `source: official (GDACS)`
should never be visually confusable with a moderator-endorsed or authority-verified crowd report,
since the project has explicitly deferred a real verified-authority-account flow.

**Warning signs:**
None will surface through normal testing — this is a "check before wide publicity/launch, not
after" category. A concrete trigger: before sharing the live URL on LinkedIn/portfolio/socials
where it could get real unexpected traffic during an actual flood/cyclone news cycle.

**Phase to address:**
Robustness/legal-disclaimer phase — but treat the *content and placement* of the disclaimer as a
design decision needing real thought (not a checkbox), and revisit before any public-launch/
promotion milestone (distinct from "first git push to a public repo," which is lower stakes).

---

### Pitfall 9: Self-declared "critical" severity is an unaccountable bypass of the only visibility gate

**What goes wrong:**
The provisional-visibility gate is the project's stated primary defense against single-source
rumors, but "critical/rescue-needed always publishes instantly, no gate" — and severity is a field
the anonymous poster picks themselves, with zero accountability for picking it wrongly. This
creates two separate failure modes, not one: an abuse path (mark any report "critical" to skip the
gate entirely — the exact mechanism designed to slow down unverified single-source claims is
opt-out by checkbox), and an ordinary incentive-driven failure mode with no malice required (anyone
who wants their report seen immediately learns that "critical" gets instant full visibility while
other categories sit dimmed, so severity inflation happens organically). Both converge on the same
outcome: the volunteer/rescue-team triage view — explicitly filtered to severity=critical +
rescue-needed to help responders prioritize — fills with noise and stops being a usable priority
signal, which is the exact failure this project's own research (Kerala 2018 duplicate/noise
problem) was trying to design around.

**Why it happens:**
The gate-bypass for critical reports is the right call for genuine emergencies (an actual
rescue-needed report can't wait for a second confirmation), but that correct exception is defined
on a field with no verification cost attached to selecting it.

**How to avoid:**
Decouple *visibility* from *triage priority*. Critical reports should still publish instantly and
full-visibility (don't gate genuine rescue calls) — but the triage view's ordering must not trust
self-declared severity alone: rank by diversity-weighted confirms and the human-contact-verification
tier (already planned) ahead of raw self-declared severity, so an uncorroborated "critical" report
sinks in priority relative to a corroborated one within the same view, rather than all
self-declared-critical reports appearing equally urgent. Track the ratio of critical-declared to
total reports as an operational signal — a ratio that climbs well above what real incident rates
would suggest (e.g., >~30% of all reports marked critical) indicates the label is being gamed or
over-used, and is worth surfacing even just as an internal metric during the demo period.

**Warning signs:**
The triage view fills with reports that were never independently corroborated; the proportion of
"critical" reports climbs disproportionately relative to other categories without a real event
driving it.

**Phase to address:**
Trust mechanic / provisional-gate phase — this needs to be designed alongside the gate itself, not
patched in after the triage view is built on the assumption severity is trustworthy.

---

### Pitfall 10: Diversity-weighted confirm geohash precision can suppress genuine local consensus instead of just filtering fake consensus

**What goes wrong:**
"Diversity-weighted confirm count" is defined as distinct confirming geohash cells, not raw votes
— but the geohash cell-size parameter is a load-bearing numeric choice, not a detail. If cells are
too large relative to the real spatial scale of a typical incident (a flooded road segment is
realistically ~100-300m), several genuinely independent nearby confirmers (neighbors on the same
street, people at the same bus stop) can fall inside one cell and get counted as a single
"independent" confirmation — meaning a real report with several real witnesses can stay stuck
in provisional/dimmed status indefinitely, which is the exact inverse of what the gate is supposed
to do (let real, witnessed reports through faster than single-source rumors). This failure mode is
invisible in testing unless someone explicitly checks confirms-per-cell against the chosen cell
size for a realistic incident radius.

**Why it happens:**
"Distinct geohash cell" reads as a simple technical definition of independence, but geohash
precision is an arbitrary choice that has to be matched against the real physical scale of what
you're trying to detect (many-eyewitnesses vs. one-person-stuffing-votes) — it's easy to pick a
precision level (e.g., copy a default from a geohash library example) without checking it against
this project's actual incident scale.

**How to avoid:**
Choose the geohash precision deliberately against realistic incident radius (a flooded road/blocked
road event, not a city-block-sized flood zone) and document the rationale, rather than defaulting
to whatever a geohash library example uses. Define an explicit fallback for gate-lifting when cell
diversity is structurally unavailable (e.g., confirms all cluster in one small area because the
incident genuinely only has nearby witnesses) — such as distinct sessions plus distinct network
origin as a secondary independence signal, so a real report isn't trapped in provisional status
purely because everyone who witnessed it lives on the same street.

**Warning signs:**
A report accumulates several confirms from what look like genuinely different individuals (checked
manually during testing/demo) but the diversity-weighted count/gate status doesn't move — a strong
signal the cell size is set too coarse for the incident types this app actually handles.

**Phase to address:**
Trust-model hardening phase — the geohash precision needs to be a stated, justified numeric
decision in that phase's plan, not left as a library default.

---

### Pitfall 11: Anonymous responder-claim and human-contact-verification are unaccountable overrides on exactly the reports where being wrong matters most

**What goes wrong:**
Both "responder claim" (mark a report "responding" so it drops out of the unclaimed-coverage-gap
view) and "human-contact verification" (mark "contacted at HH:MM" with a priority that outranks
passive voting) are planned as actions any anonymous session can take, with no accountability
mechanism, on precisely the critical/rescue-needed reports where a wrong override causes the most
harm. Unlike the misinformation-style pitfalls elsewhere in this document, the harm mode here isn't
"someone sees a false claim" — it's "a real rescue-needed report silently disappears from the
unclaimed-gaps view because anyone (including someone testing the UI, or a bad actor) marked it
'responding' and never actually went, and nobody follows up because the coverage-gap layer now
shows it as covered." This is the single highest-severity abuse vector across the whole feature
list, and it is not mentioned anywhere else in this project's own trust-model-hardening research
despite that hardening explicitly targeting misinformation-style gaming.

**Why it happens:**
Responder-claim and human-contact tier were designed to solve a real, documented problem (duplicate
effort in Kerala 2018) by adding a fast, frictionless "someone's on it" signal — but frictionless
and unaccountable are the same design choice here, and the feature was scoped for coordination
value without a matching abuse-model pass.

**How to avoid:**
Claims must expire on a short fixed window (e.g., 30-60 minutes) and automatically return to
"unclaimed" in the coverage-gap view unless actively refreshed — never a permanent, silent removal
from that view. A claim should annotate a critical report, never remove it from the critical/
rescue-needed triage list itself — the triage view should always show "responding (claimed Xm ago
by an unverified session)" rather than hiding it. The human-contact-verification tier — which is
explicitly meant to outrank passive voting — should require more than a fresh anonymous session to
invoke (e.g., an aged session with some minimum prior track record, plus a strict per-session rate
cap on how many "contacted" marks can be made in a given window), and any claim or contact-mark
should be reversible by a subsequent session (e.g., someone else confirms the report is still
active), always defaulting back to unclaimed/unverified rather than requiring the original claimant
to retract it.

**Warning signs:**
None will surface in isolated feature testing — this requires explicitly testing the
"responder claim it, never follow up, does the report silently vanish from the coverage-gap view"
scenario, and the "one session immediately marks a fresh critical report 'contacted,' does that
outrank real confirms" scenario, as dedicated abuse-model test cases.

**Phase to address:**
Whichever phase implements responder claim and the human-contact-verification tier — flag it
explicitly as needing its own abuse-model design pass (expiry, non-hiding annotation, reversibility)
before being marked done, not just a CRUD feature.

---

### Pitfall 12: A shareable card that bakes the confirm count into a static image defeats the retraction mechanism it's paired with

**What goes wrong:**
The project's own retraction-push design explicitly exists to fix a documented real failure mode
(the Boston Marathon bombing misinformation study — corrections reach a small fraction of the
audience of the original false claim) by giving shareable cards "a status URL that resolves to
current state... instead of a frozen stale count." But the shareable-card feature itself is
specified as "a simple image/card (✅ Road X flooded — confirmed by 12 nearby, 10 min ago)"
intended for exactly the channel (WhatsApp forwards) where the retraction fix matters most. A
forwarded image's pixels cannot be retracted, live-updated, or resolved to current state — once
"confirmed by 12 nearby" is burned into a JPEG/PNG and forwarded, no live-resolving link fixes it,
because nothing about a static image is live. As specified, the two features directly contradict
each other: the retraction mechanism's stated purpose is undone by the delivery format it's
supposed to protect.

**Why it happens:**
"Shareable card" and "retraction resolves live via a status link" were designed as complementary
features addressing related problems, but the image-vs-link tension wasn't resolved — a card
optimized for WhatsApp-forward believability (a self-contained, good-looking image with the count
visible) is structurally the wrong artifact for a mechanism that depends on the viewer re-fetching
current state.

**How to avoid:**
The card image must not carry any numeric confirm count or an unqualified "✅ verified" claim
baked into the pixels — it should carry only the incident description, category, and a short
status URL (plus maybe a QR code or short-link for WhatsApp-friendliness), with the actual count
and confirm/retracted state living behind that link, fetched live whenever anyone opens it. This
preserves the "believable, shareable card" value for spreading the URL, while keeping the
retraction/live-resolution property intact — because the property only works on data fetched at
view time, never on data burned into distributed image bytes.

**Warning signs:**
None will surface until specifically asked "if this report is disputed/hidden five minutes after
someone forwards the card image, does the forwarded copy still say 'confirmed by 12'?" — the
answer for a naive image-with-baked-in-count implementation is always yes, which is the bug.

**Phase to address:**
Shareable-cards phase — this needs to be resolved as a design decision (link-carries-state,
image-carries-no-state) before implementation starts, not discovered after the image template is
already built.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Plain lat/lon columns, no PostGIS | No extra infra, simpler local dev | Must hand-roll bounding-box + Haversine correctly (Pitfall 1); no built-in spatial index (GiST) if data grows past low-hundreds-of-thousands of rows | Acceptable for this project's stated scale — just don't skip the bounding-box index |
| Denormalized `confirm_count` column for read speed | Fast feed queries, no aggregate join per page load | Becomes the race-condition surface (Pitfall 2) if updated via blind increment instead of atomic/recomputed | Acceptable only if updates go through a single atomic path (transaction or single `UPDATE ... SET x = x+1`), never split read-then-write across app code |
| IP-only rate limiting (skip session-based limiting) | One line of middleware, no session infrastructure needed | Blocks legitimate CGNAT-shared users during surges, doesn't stop cookie-clearing abuse (Pitfall 3, 4) | Never acceptable as the sole layer once anonymous sessions already exist for reputation tracking — the session infra is already planned, so there's no cost saved by skipping it here |
| Single flat auto-moderation threshold instead of the planned confidence-cascade | Faster to ship, one API call, one if-statement | Either lets real abuse through or auto-hides genuine urgent reports (Pitfall 6) — the exact failure the cascade was designed to avoid | Acceptable only as an interim MVP with a human review queue as the real safety net, never as the sole gate before public launch |
| Skipping a purge/archive job for expired reports | Simpler v1 — expired rows just get filtered out of queries | Table grows unbounded, degrading query performance and defeating the bounding-box index's benefit over time (Pitfall 1) | Acceptable for the first weeks of a low-traffic demo; add a scheduled archive/delete job before any real promotion push |
| Trusting self-declared severity for triage ordering | No extra field/logic needed, simplest possible triage view | Triage view fills with unverified "critical" noise (Pitfall 9), defeating its purpose | Never acceptable once diversity-weighted confirms or human-contact verification exist — always rank by those first |
| Frictionless, permanent responder-claim with no expiry | Simple UI, immediate coordination value | A stale/false claim silently hides a real gap indefinitely (Pitfall 11) | Never acceptable for critical/rescue-needed reports — always require an expiry+refresh model |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| GDACS / IMD / CWC background poller (goroutine) | Treating the external feed as always available/well-formed; a poller crash or malformed-response panic can silently kill the goroutine and the map quietly stops updating with no error surfaced anywhere | Wrap the poller loop in recover()/retry-with-backoff, log poll failures distinctly from "poll succeeded, zero new events," and add a simple health signal (e.g., `last_successful_poll_at` exposed on a status endpoint) so a stalled poller is detectable, not silent |
| Browser Geolocation API | Assuming geolocation always succeeds and is precise; on many Android/older browsers it can return a stale cached position, a large accuracy radius, or fail outright (permission denied), especially indoors during a power-outage scenario (the exact scenario this app targets) | Always read and store/display the `accuracy` value returned alongside coordinates; have an explicit manual-pin-drop fallback on the map when geolocation fails/denies, not just a blank error |
| Toxicity/moderation API (hosted) | Sending raw free text straight to a general-purpose toxicity API and trusting its score as ground truth for a disaster-report domain it wasn't tuned for (see Pitfall 6) | Calibrate against a hand-built fixture set of realistic genuine vs. abusive disaster reports before trusting any threshold in production |
| Railway/Render Postgres deploy | Assuming the deployed Postgres instance's default timezone matches local dev (Pitfall 5) | Explicitly set/verify session timezone handling, use `timestamptz` everywhere, test expiry logic against the actual deployed DB's `now()` |
| Background Sync (service worker, offline queue) | Recording `created_at`/`expires_at` at server-receipt time for a report that was actually observed hours earlier while offline — a stale observation enters the feed looking fresh, and its expiry window is measured from the wrong start | Capture the client-side observation timestamp at submit time and send it with the queued report; use that timestamp (validated against a sane bound) for both display "time ago" and expiry-window calculations, not server-receipt time |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Full-table Haversine scan for "nearby" feed | Feed/map endpoint latency grows linearly with total report count, not with local report density | Bounding-box index first, Haversine only on the narrowed set (Pitfall 1) | Noticeable past a few thousand rows; painful past tens of thousands |
| Diversity-weighted confirm count recomputed via full join/aggregate on every page view | Feed page slows down specifically on popular/highly-confirmed reports as their confirmation count grows | Cache the computed diversity-weighted score, recomputed incrementally on each new confirm/dispute inside the same transaction that inserts the vote, not recomputed from scratch on every read | Breaks down once any single report accumulates hundreds of confirmations (plausible during a real surge event) |
| Unbounded `reports`/`confirmations` table growth (no archive job) | Slow queries appear gradually over weeks/months of the demo being live, not attributable to any single code change | Scheduled archive/delete or partition of expired rows | Matters most for a long-lived public demo, less for a short-lived one — but "long-lived public portfolio demo" is exactly this project's shape |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Treating the anonymous session id (cookie/localStorage) as a trust anchor without any anti-forgery/tamper check | A client can forge or replay a session id to impersonate an aged, high-reputation session | Sign/HMAC the session id server-side (don't accept a client-supplied arbitrary id as-is for anything reputation-relevant), and never trust client-reported geolocation for confirm/dispute diversity weighting without at least sanity-bounding it against plausible report-area radius |
| Storing raw IP addresses indefinitely alongside anonymous "no signup" reports | Undermines the "anonymous" positioning if the DB is ever exposed/leaked — raw IP + timestamp + precise GPS is realistically de-anonymizing | Hash IPs (with a server-side secret salt, not a fast unsalted hash) if stored for rate-limiting/abuse purposes, and consider a retention window rather than indefinite storage |
| No takedown/report-abuse path for photo or text content once public | Combined with the intermediary-liability gap (Pitfall 8), leaves no operational mechanism to act on a genuine illegal-content report even if legally aware of the obligation | Ship a minimal "report this post" action wired to a monitored inbox/queue before any wide public promotion, even if full auto-moderation isn't complete |
| "I'm safe" check-in posted on behalf of a third party with no verification | The same third-party-PII abuse surface that got the missing-person registry deliberately deferred to V2 (per PROJECT.md) is present here anyway — a false "X is safe" check-in can cause a real search/family to stand down incorrectly | Frame check-ins as self-reported by default ("I am safe") rather than easily third-party-postable; if third-party check-ins are kept, treat them with the same caution the project already applied to the missing-person registry, not as a lower-risk feature just because it's framed positively |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Empty map on first visit with zero copy explaining why | Reads as "broken app," especially damaging for a portfolio reviewer who won't wait around for real traffic | Demo/replay mode + official feed as core scope (already planned) — but make sure the empty/low-data state has explicit copy ("No active reports near you right now — here's a past event replay") rather than a bare empty map |
| "Confirmed by N nearby" shown without any indication of *when* or *how stale* | User can't tell if a confirmed-safe shelter status is 10 minutes or 10 hours old at a glance | Always pair confirm-count with a relative timestamp and visually fade/de-emphasize as a report approaches its expiry window, not just a binary shown/hidden state |
| Provisional-gate dimmed pins with no explanation of why they're dimmed | A user assumes a real report simply "isn't loading correctly" rather than understanding it's awaiting a second confirmation | Explicit micro-copy or icon distinguishing "provisional — awaiting confirmation" from a rendering bug, given this is a genuinely unusual UI state most users won't have seen elsewhere |
| Rate-limit/429 shown as a generic error during a real surge | A legitimate user behind a busy CGNAT IP during an actual event sees a confusing failure at the worst possible moment | A specific, human copy message ("too many reports from your network right now, try again in Xs") rather than a raw error, and err toward looser limits given Pitfall 3 |
| Responder claim shown as a simple binary "claimed/unclaimed" with no age or verification level | A team looking at the coverage-gap view can't tell a 5-minute-old claim from a 5-hour-old stale one, or an unverified self-claim from a human-contact-verified one | Always show claim age and claimant verification tier inline, and let the coverage-gap view surface stale claims (past the expiry window) as effectively unclaimed again (ties directly to Pitfall 11's expiry mechanism) |

## "Looks Done But Isn't" Checklist

- [ ] **Confirm/dispute tally:** Often missing a concurrency test — verify the count is exactly
      correct after firing many simultaneous confirms at one report, not just after sequential
      manual clicks.
- [ ] **Auto-expiry:** Often missing timezone correctness — verify a report actually disappears
      at its exact `expires_at` instant on the deployed (not just local dev) environment.
- [ ] **Nearby/feed query:** Often missing an index — verify with `EXPLAIN ANALYZE` that the query
      plan uses an index scan, not a sequential scan, once the table has more than a trivial
      number of rows (seed a large fake dataset to check, not just the demo dataset).
- [ ] **Auto-moderation cascade:** Often missing calibration against real-shaped input — verify a
      set of realistic urgent genuine reports pass through untouched before trusting it against
      abuse.
- [ ] **Background GDACS/official-feed poller:** Often missing failure visibility — verify there's
      a way to tell the poller has silently stopped working (a "last successful poll" signal),
      not just that it worked once during development.
- [ ] **Demo/replay mode:** Often missing a persistent visual distinction from live data — verify
      a screenshot of the feed at any point makes it obvious what's demo vs. official vs. real
      crowd data.
- [ ] **Legal disclaimer:** Often present but inert — verify it's an unmissable banner/first-visit
      element with a real contact/report-abuse path, not just linked text in a footer.
- [ ] **Anonymous session/reputation:** Often missing tamper-resistance — verify a client can't
      trivially forge or replay another session's id to inherit its reputation score.
- [ ] **Triage view:** Often missing corroboration-aware ordering — verify it ranks by
      diversity-weighted confirms/verification tier, not raw self-declared severity, before
      trusting it under a flood of self-marked "critical" reports.
- [ ] **Responder claim / human-contact tier:** Often missing expiry and reversibility — verify a
      claim automatically lapses back to "unclaimed" after its window, and that a critical report
      never fully disappears from the coverage-gap view just because it was claimed once.
- [ ] **Shareable card:** Often missing the link/image separation — verify the distributed image
      itself carries no confirm count or unqualified "verified" claim, and that opening the card's
      link always reflects current, live status.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|-----------------|
| Naive lat/lon full-scan query shipped | LOW | Add the bounding-box pre-filter and composite index; no data model change needed, purely a query rewrite |
| Race condition in confirm tally already shipped and possibly under-counting in production | MEDIUM | Recompute all `confirm_count`/diversity-weighted scores from the `confirmations` table (source of truth) in a one-off migration/backfill, then fix the write path to be atomic going forward |
| Timezone bug already causing wrong expiry in production | LOW-MEDIUM | Backfill/correct any bare `timestamp` columns to `timestamptz` with an explicit UTC-offset migration, audit all expiry-comparison code paths for one consistent time source |
| Moderation cascade shipped mis-calibrated (too strict, hiding real reports) | MEDIUM | Immediately widen the threshold or move the mid-band to "flag not hide" until a human-reviewed fixture set is built and calibration redone; communicate/apologize if any real reports were wrongly hidden during the affected window |
| Legal/disclaimer gaps discovered after some public traffic already happened | LOW | Add the banner and grievance/report-abuse path immediately — this is cheap to retrofit; the real cost is reputational/legal exposure during the gap, so prioritize this fix over feature work if ever raised as a concern |
| Triage view already trusting raw self-declared severity | LOW | Re-order the existing query by diversity-weighted confirms/verification tier as a secondary sort key; no schema change needed since the underlying data already exists |
| Responder-claim shipped without expiry, stale claims already hiding gaps | LOW-MEDIUM | Add an `expires_at`/last-refreshed column to claims with a migration defaulting existing claims to already-expired (forcing a re-triage pass), then enforce the expiry going forward |
| Shareable card image already shipped with count baked in | MEDIUM | Regenerate the card template to drop the numeric count/verified claim from the image and push the live count behind the status link only; old already-forwarded images can't be recalled, so treat this as "stop the bleeding" not "fix the past" |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| Naive full-scan nearby query | Base reporting loop / data model | `EXPLAIN ANALYZE` shows index scan, not seq scan, on a seeded large dataset |
| Race condition in confirm tally | Trust mechanic (confirm/dispute) | Concurrency test: N parallel confirms produce exactly N in the tally |
| IP rate limiting penalizing CGNAT users | Robustness / anti-abuse & rate limiting | Load test simulating many distinct sessions from one IP is not blocked; single session flooding across many IPs is blocked |
| Sybil-gamed trust score | Trust-model hardening | Explicit red-team pass: self-confirm from multiple fresh sessions, verify diversity weighting/provisional gate resists it as designed |
| Timezone/expiry bugs | Base reporting loop (schema) + auto-expiry implementation | Automated test: report with near-future `expires_at` disappears from feed at the correct instant on the deployed environment |
| Unmoderated abuse / miscalibrated cascade | Robustness / auto-moderation | Fixture test set of realistic genuine vs. abusive reports validated against the cascade before launch |
| Empty-map / demo-vs-real ambiguity | Demo/replay mode + official-feed integration | Screenshot review: source is visually unambiguous at any point in time |
| Legal/authority-implication exposure | Robustness / legal disclaimer | Manual review of banner visibility, contact path, and branding-similarity before any public-promotion milestone |
| Self-declared-critical bypasses provisional gate | Trust mechanic / provisional-gate | Triage view ordering test: uncorroborated self-declared-critical reports rank below corroborated ones |
| Geohash precision suppresses genuine consensus | Trust-model hardening | Manual test: several distinct real confirmers near a realistic incident radius move the diversity-weighted gate as expected |
| Unaccountable responder-claim / contact-tier override | Whichever phase ships responder claim + human-contact tier | Abuse-model test: unfollowed claim expires back to unclaimed; critical report never fully vanishes from coverage-gap view |
| Shareable card image bakes in a count the retraction mechanism can't reach | Shareable-cards phase | Manual test: dispute/hide a reported incident after generating its card, confirm the card's link (not just the app UI) reflects the retracted state |

## Sources

- PostgreSQL concurrency documentation and vote/hit-counter race-condition pattern —
  [oneuptime.com PostgreSQL race conditions](https://oneuptime.com/blog/post/2026-01-25-postgresql-race-conditions/view),
  [sqlfordevs.com transaction locking](https://sqlfordevs.com/transaction-locking-prevent-race-condition) (HIGH confidence — vendor/technical documentation)
- Bounding-box + Haversine two-stage geospatial query pattern —
  [plumislandmedia.net Haversine MySQL](https://www.plumislandmedia.net/mysql/haversine-mysql-nearest-loc/),
  [mysql.rjweb.org find nearest](https://mysql.rjweb.org/doc.php/find_nearest_in_mysql) (HIGH confidence for the pattern itself — these sources are MySQL-authored, but the bounding-box-then-exact-distance technique is database-agnostic and applies identically with Postgres B-tree indexes)
- CGNAT and IP-based rate-limiting collateral damage —
  [Cloudflare: One IP address, many users](https://blog.cloudflare.com/detecting-cgn-to-reduce-collateral-damage/),
  [SOAX: mobile proxies and CGNAT](https://soax.com/blog/mobile-proxies-cgnat) (HIGH confidence — primary vendor engineering writeups)
- Crowdsourced crisis-mapping misinformation/verification challenges (Ushahidi) —
  [ScienceDirect: Lessons learned from deploying crowdsourced technology](https://www.sciencedirect.com/science/article/pii/S1877050920313077/pdf) (MEDIUM-HIGH confidence — peer-reviewed)
- India IT Rules 2021 intermediary safe-harbor/grievance-officer/takedown framework —
  [PRS India: IT Intermediary Guidelines Rules 2021 billtrack](https://prsindia.org/billtrack/the-information-technology-intermediary-guidelines-and-digital-media-ethics-code-rules-2021),
  [Cyril Amarchand Mangaldas: From Harbour to Hardships](https://corporate.cyrilamarchandblogs.com/2021/05/from-harbour-to-hardships-understanding-the-information-technology-intermediary-guidelines-and-digital-media-ethics-code-rules-2021-part-ii/) (MEDIUM confidence for the general framework; LOW confidence for how it applies specifically to a solo hobby/portfolio deployment — recommend independent legal check before wide public promotion)
- Existing project research already cited in PROJECT-NOTES.md (Ushahidi 2008 Kenya, Waze
  confidence/reliability scoring, X/Twitter Community Notes, Wikipedia Pending Changes/ClueBot NG,
  Kerala 2018 rescue-request duplication study, Boston Marathon misinformation-reach study) —
  treated as HIGH confidence per the project's own prior research pass, reused here for the
  Sybil/gaming and moderation-calibration pitfalls specifically.
- Pitfalls 9-12 (severity self-declaration bypass, geohash precision, responder-claim/contact-tier
  abuse, shareable-card/retraction contradiction) are derived directly from internal
  feature-interaction analysis of this project's own PROJECT.md and PROJECT-NOTES.md scope, not an
  external source — confidence HIGH that the interaction/contradiction exists as specified, since
  it follows directly from the project's own stated design; MEDIUM on the specific numeric
  mitigations suggested (expiry windows, cell-size guidance), which are reasonable defaults, not
  independently benchmarked values.

---
*Pitfalls research for: Crowd-verified local emergency information feed (Pinalert)*
*Researched: 2026-09-05*
