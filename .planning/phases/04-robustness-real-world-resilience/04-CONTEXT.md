# Phase 4: Robustness, Real-World Resilience - Context

**Gathered:** 2026-10-04
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase keeps the reporting loop usable on weak networks, under abuse, and with an honest "unofficial"
statement. It delivers ROBUST-01 (a Low data view, the roadmap's text-only feed), ROBUST-02 (an offline report
queue), ROBUST-03 (automatic screening of report text), ROBUST-04 (rate limits keyed on the verified account) and
ROBUST-08 (a persistent disclaimer).

Five things were added by the owner during this discussion and are in scope:
1. Vote throttling and a re-cast cooldown. Phase 3 deferred it here (`03-CONTEXT.md` deferred list) and the Phase 2
   review found the four vote routes have no throttle at all (`.planning/STATE.md` blocker WR-01).
2. A moderator screen that only the owner (and anyone listed) can open.
3. A Report button on every post, like Instagram's Report, so an admin can act.
4. Separate public Disclaimer and Policy pages, linked from a short bar on every page.
5. The Low data view can post and vote, not only read.

Not in this phase: comments and the report page (Phase 8, the owner's Instagram idea is captured under Deferred),
official feeds and demo mode (Phase 5), an installable app, service worker or Web Push (backlog 999.3, the owner
said: keep it a website), the author deleting their own report, image screening.

Phases 3, 3.1 and 3.2 run before this phase (`ROADMAP.md` execution order: 1, 1.1, 2, 3, 3.1, 3.2, 4, 5, 8, 6, 9), but are
not built yet. Plan against their designs (`03-CONTEXT.md`, `03-UI-SPEC.md`, the Phase 3.1 view partition), not only
against today's code.

Planner notes:
- **Cut order if the plan has to be cut** (Claude's suggestion. Claude told the owner it would note this order unless
  they said otherwise, and the owner moved on without objecting. It is not a confirmed decision, so ask the owner before
  cutting anything they chose explicitly): first the Confirm and Dispute buttons on the Low data view (D-15, vote
  part), then the Report button (D-09), keeping the moderator screen's automatic list and Remove. Then trim the Policy
  page to essentials. The text check, the rate limits, the disclaimer bar and the offline queue are not optional.
- **Roadmap and requirements need an edit before planning.** The Phase 4 entry in `ROADMAP.md` and `REQUIREMENTS.md`
  do not list the added scope (vote throttling, moderator screen, Report button, Disclaimer and Policy pages, held
  state, posting and voting on the Low data view). The plan checker and verifier read those documents. Use
  `/gsd-phase` to edit Phase 4: extend ROBUST-03 and ROBUST-04, add requirement ids for the moderator screen with the
  Report button and for the legal pages, amend success criterion 3 (see D-02) and the ROBUST-03 text ("fail closed
  (provisional)"), reword criterion 4 and ROBUST-04 ("same anonymous session" becomes the verified account), and update
  "Depends on" (it lists only Phases 1 and 2; add Phase 3 and 3.1, whose trust block and view partition this phase uses).
- **`Mode: mvp` guard.** Phase 4 carries `Mode: mvp` but its Goal is not written as a user story, the same guard that
  stopped verification in Phases 2 and 3 (`.planning/STATE.md`). Decide before planning: rewrite the Goal as a user
  story or clear the flag.

</domain>

<decisions>
## Implementation Decisions

Provenance: decisions marked "owner" were chosen or stated by the owner. "Default" means Claude stated it, the owner
did not object, and it can be changed. Rules in the "Derived rules" list were worked out from these decisions and not
separately reviewed by the owner.

### Text screening (ROBUST-03)
- **D-01:** The check runs inline during `POST /api/reports`, while the person waits, with a short time limit (about
  3 seconds). Normally it finishes and there is no pending state. Only the cleaned description text is sent to the
  provider: never the account, session, IP address or coordinates. Critical and rescue-needed reports are never held
  by the check (ROBUST-03): they publish at once, and if the check could not finish it is retried in the background.
  The earlier architecture research preferred always-background moderation (`.planning/research/ARCHITECTURE.md`
  Pattern 4). Inline was chosen so a rejection reaches the person while the form is still open, and an outage still
  cannot block posting, because after the time limit the report becomes pending. Default (stated with the first
  question, not objected to).
- **D-02:** Pending (the check could not finish): a non-critical report is held off every feed and map until the check
  finishes. The author sees "Waiting for a text check" on their Activity page. A background retry (backoff with
  jitter, honouring `Retry-After`) releases it, after which the normal rules apply (Provisional until two independent
  places confirm). This amends roadmap success criterion 3 and the text of ROBUST-03 in `REQUIREMENTS.md` ("fail closed
  (provisional)"), which said other categories "stay provisional": Provisional is visible to everyone today (dimmed,
  labelled Unconfirmed), which would show unscreened text. The held
  state needs its own representation so it never appears in Show disputed or the Phase 3.1 Disputed view (Hidden),
  nor in Resolved or Past (see R-01). Owner chose "Hold off every feed".
- **D-03:** Clearly abusive text (hate, harassment, sexual content, at a high score) is rejected at submit for every
  severity, including critical and rescue-needed. The form stays open so the person can reword, nothing is stored, and
  the message names the problem. The violence and self-harm categories alone never reject, because genuine disaster
  reports use those words. Reason: severity is self-declared, so failing open on it would let anyone skip screening by
  choosing critical. Phase 2 D-06 covered soft trust signals, not content abuse. Owner chose "Reject all, reword".
- **D-04:** Borderline text (below the reject line) posts normally; a non-critical one stays Provisional until two
  places confirm. The verdict and per-category scores are stored without the report text, so thresholds can be tuned
  from real data, and the post appears on the moderator screen (D-08). This is the middle band of the confidence
  cascade (`PROJECT.md`: a cascade, not a flat cutoff). Owner chose "Post, add review page" over the smaller "post and
  log" option, knowing it adds scope.
- **D-05:** Spam means only gibberish: long runs of one character, or text with no real words. It does not look at
  capital letters, links, repeated posts or phone numbers (owner chose only the gibberish rule). Consequences: report
  text can contain links, so every place that shows text keeps inserting it as plain text (no auto linking and no
  markup parsing sink, which the project already enforces through `Pinalert.setText`), and rapid repeat posting is
  handled by the rate limits, not a duplicate rule. Research note: ALL CAPS, addresses and phone numbers are common in
  genuine emergency reports (`.planning/research/PITFALLS.md` Pitfall 6).
  Owner note (2026-10-08): people must be told to describe their problem or situation, so every report says what is
  happening instead of just "help". The owner approved two strings, used verbatim: the hint for the description box of
  the shared report form (main page and Low data view) "Describe what is happening, for example: water is knee deep on
  Main Road and three people need help." and the gibberish rejection message "Say what is happening so people nearby
  can help, for example: flood on Main Road, three people on a roof." A post of only one word is already rejected as
  no real words (fewer than two words of two or more letters), so no new rule is added. The plans keep the letters only
  run reading; the owner has not signed it off, and the veto recorded in the outline's "Open questions resolved during
  planning" stays open.
- **D-06:** One general text check, with the report rules (severity, pending, hold, spam) layered on top and nothing in
  it tied to report categories, so Phase 8 comments and Phase 9 nicknames reuse it. Phase 8 scope note 4 in
  `ROADMAP.md` asked Phase 4 to decide this. Owner chose "build it once".
- **D-07:** How accuracy is stated (default; the owner chose to clarify twice and then moved on): a written fixture of
  150 to 300 realistic genuine reports (English, Hindi, Telugu, Tamil and Hinglish, including urgent wording, all
  capitals, addresses and phone numbers) plus abusive examples, kept in the repo. The test fails if any genuine fixture
  is rejected or any critical or rescue fixture is withheld. The docs say plainly that zero of N shows the true rate is
  probably under about 3 in N, not 0.1%, and that real calibration needs real traffic and the moderator screen's
  decisions. Reason: OpenAI names Telugu, Bengali and Marathi in its language claims but not Hindi, Tamil or Hinglish
  (verified facts below). The earlier "at most 0.1%" figure (`PROJECT-NOTES.md`) is a design target, not a result.

### Moderator screen and Report button
- **D-08:** A small screen only moderators can open. Moderators are accounts whose verified email is on a list in server
  configuration (default; no schema change); everyone else gets a plain 404. It lists: (a) automatically flagged
  borderline posts with their scores, (b) user reports with reasons and a count (D-09), and (c) posts whose text check
  has not finished, including serious posts already published while the check was pending, so the owner can act if the
  provider is down for long. Actions: **Keep** (the flag is cleared and the post stays; on a post whose check has not
  finished, Keep accepts it as is and releases it into the normal rules, so there is no separate Release action) and
  **Remove** (the post is withheld from every feed and the author is told "Removed after review"). Remove also works on
  any post, not only listed ones, because India's takedown windows are reportedly short (unconfirmed, see the IT Rules
  note in the code context; build no fixed deadline into code or copy before a lawyer or mentor has read the Gazette
  text). A moderator finds such a post by typing its id or pasting a link that carries it: until Phase 8 gives each report
  its own page, that is a deep link carrying `report=ID` (Phase 3.1 plans links such as `/?view=resolved&report=ID`), and
  the id is shown to moderators in every list. It must be usable on a phone. Owner chose to have this screen.
- **D-09:** A Report button: a quiet secondary control (not a fourth main button) on the map popup, the list row and
  the Low data view, and later the Phase 8 report page. Reasons: spam, abusive or hateful, false or a hoax (with a hint
  that Dispute is the control for disagreeing with the facts), dangerous or illegal, other. One report per account per
  post, limited per account (D-10, D-11), and the reporter is never shown to the author. A report never hides a post by
  itself, so a group of people cannot hide a real emergency; it only queues the post for the moderator screen. It is
  online only (not saved offline). There is no police message and no police feature inside the flow: the owner said
  police guidance is plain text on the Disclaimer and Policy pages only. Owner reasoning: it works like Instagram's
  Report, so that the admin can take action. Details beyond the reasons list are defaults.

### Rate limits (ROBUST-04)
- **D-10:** Limits apply to report submission, the four vote routes (confirm, dispute, resolve, reopen) and a cooldown on
  re-casting a vote on the same report, plus the Report button (D-09). The primary key is the verified account, never
  the session: one account can hold several sessions (Phase 1.1 D-10, Phase 3 D-10), and "anonymous session" in
  ROBUST-04 is stale wording after Phase 1.1, the same correction Phase 3 recorded for TRUST-03. The network address is
  only a very loose secondary backstop, applied when the real visitor address is known (ROBUST-04 "IP secondary"; see
  Claude's Discretion). Settled from earlier phases; not asked.
  (Note 2026-10-16, Phase 3.1 built: a fifth vote route, the take back `POST /api/reports/{id}/unresolve`, sits
  beside the four in the verified account group. The Phase 4 plans read D-10's "the vote routes" as all five and
  throttle the take back like reopen, sharing the `resolution` cooldown key; RECOMMENDED DEFAULT, owner veto at UAT, listed under "Phase 3.1 alignment defaults" below.)
- **D-11:** Allowances (owner decision, stated in the owner's own words below), as a burst then a refill:
  - **Serious** reports (severity critical in any category, plus category `rescue_needed` always, even if marked low):
    about 5 quickly, then 1 more every 5 minutes.
  - **Everything else** (low and medium): about 8 quickly, then 1 more every 15 minutes (the owner wanted low and medium
    to have a bigger room too, with a longer wait).
  - **Votes:** about 20 quickly, then 1 every 15 seconds. **Changing a vote** on the same report: once every 30 seconds.
  - **Report button:** not discussed with the owner; Claude's discretion, a roomy default such as 10 quickly then 1 every
    2 minutes (a report cannot hide anything, so the limit only stops flooding the moderator screen).
  - The two report allowances are separate, so using one never blocks the other. All values live in one named place
    (the `cmd/server/main.go` constants pattern) and can be changed with one setting.
  Reasoning recorded for the planner: critical and rescue-needed reports can never be hidden by disputes (Phase 2 D-06),
  so the limit is the only automatic brake on fake serious posts, and the moderator screen's Remove is the backstop,
  including against one person using many email accounts. We cannot tell a real emergency from a fake one at the moment
  of posting, and a "different situation an hour or two later" must never be blocked, so seriousness is judged by the
  mark the reporter sets, the refill is time based, and the serious allowance is deliberately roomy. Owner's words: the
  other situations or events might also be as serious as the rescue thing, so they should get the allowance too; low
  and medium should also have it, but with a longer time.
- **D-12:** When a limit blocks a post: the message says exactly how long to wait, the typed text is kept (and the
  offline queue retries after the wait), a `Retry-After` header is sent, and for a serious post the message adds
  "In an emergency, call 112 now" (drawn from the numbers setting, D-22). Every block is logged so the owner can see
  whether it ever hits a real person. The existing limiter's 429 body hardcodes the field name `email` and the message
  "Too many requests. Try again in a minute." and sets no `Retry-After` (`internal/ratelimit/perip.go`), so it must not
  be reused unchanged on report, vote or Report routes. Copy follows the research: a specific human message, erring
  toward looser limits (`.planning/research/PITFALLS.md` UX table).

### Low data view (ROBUST-01)
- **D-13:** Name: **Low data view**, with the line "Text only, no map" under it. The words "light" and "lite" are never
  used for this view, because the Light and Dark theme toggle owns them (the owner raised this). Owner approved.
- **D-14:** It is its own page with its own bookmarkable address, not a switch inside the main page: the main page loads
  Leaflet and MapLibre (classic blocking scripts from unpkg) before the feed renders, so a switch inside it saves
  nothing. How it is built (server rendered rows, or the shared feed and vote scripts without the map libraries) is
  left to research, with a measured size budget that is documented and tested. It sits behind login like the main page.
  Stated to the owner, not objected to.
- **D-15:** People can read the list, post a report, and Confirm or Dispute (owner chose option 3 of read only, read and
  post, or read, post and vote). Claude had leaned to option 2 for phase size; the owner chose 3 after asking how the
  location prompt works. Posting needs a map free location path because the report modal hard requires a Leaflet
  marker today (see code context).
- **D-16:** Location: it uses the phone's position and the owner confirmed the exact position is wanted. Posting shows
  the accuracy ("accurate to about 30 metres" is the example wording) and offers a typed place as a secondary way
  (needs a little data). Voting uses the position for the 1 km check (Phase 3 D-01); only the precision 7 square is
  kept server side. No new browser question appears if the person already allowed location. The list shows at once from
  the last known position stored on the device, then updates when the fresh position arrives (today the main page waits
  for the position, up to 8 seconds, before it loads any reports). If both the phone position and the typed place
  fail, the person cannot post from the Low data view and is told to use the main page.
- **D-17:** Finding it (owner: ok): a link called "Low data view" on the login page and in the account menu, plus a
  bookmarkable address; on the main page, if the list has not loaded after about 6 seconds, a message "Taking long?
  Open the low data view." that works in every browser; the choice is remembered per device so the next visit opens
  there, with a clear "Open full version" link back. A Save-Data or slow connection hint is Chromium only (verified:
  absent in Safari, iOS browsers and Firefox), so it can only be an extra, never the way in.

### Offline queue (ROBUST-02)
- **D-18:** It stays a website: no service worker, no installable app, no Web Push (owner: "I just wanted to keep it as a
  website, so we're not going to talk about that"). The queue works while the page is open, or the next time it is
  opened with a signal. IndexedDB plus a retry on the `online` event is the baseline, decided by real request outcomes
  with a timeout, not by `navigator.onLine`. Background Sync stays v2 (PWA-05). Votes are not saved offline: a vote needs
  a live position for the 1 km check and the report may have changed (Phase 2 D-04: no optimistic vote updates).
- **D-19:** A saved report never expires and is never discarded for age (owner: "It should not expire. No matter what").
  It keeps trying until it sends or the person discards it. The expiry timer starts when it is uploaded, exactly like
  any other report (8 hours, or 24 for critical), and the server stamps `created_at` and `expires_at` on arrival; no
  client time is used for expiry or ordering. The row also says "Written 4 hours ago" when it was sent more than 30
  minutes after it was written, so readers can judge freshness (owner confirmed in their own words: the timer starts
  from upload and the row says written four hours ago). Owner chose option B over Claude's suggested option A.
  Research had recommended the opposite (`.planning/research/PITFALLS.md` Integration Gotchas, Background Sync row:
  expire from the observed time); the label is the mitigation that keeps the owner's choice.
- **D-20:** Defaults (stated, not objected to): a "Waiting to send" strip above the list and on the Activity page with
  Send now and Discard, the typed text safe until sent; a server rejection (for example the text check) shows why and
  lets the person edit and resend; when the signal drops the list stays on screen with "Last updated N minutes ago"
  instead of being replaced by an error (today `feed.js` hides the whole list after one failed poll); a saved report is
  bound to the account that wrote it, so a different person logging in on a shared phone never sends it. Honest limit to
  document: browsers can still delete saved data (private browsing closing, Safari removing a site's storage after about
  7 days of Safari use without interaction unless it is on the Home Screen, very low storage), so ask the browser to keep
  the storage and keep the unsent strip visible.

### Disclaimer and legal pages (ROBUST-08)
- **D-21:** A thin bar at the top of every full page, always visible, not closable: login, verify outcome, the map page,
  Activity, the Low data view and both legal pages. Short text and a link to the Disclaimer page. Owner chose the top.
  The map page has four fixed controls (list toggle, report button, theme toggle, toast) with a geometry contract test
  (`web/theme_contract_test.go` `TestThemeToggleFloatsClearOfTheReportButton`) and Leaflet controls at the top left;
  exact geometry is for the UI contract.
- **D-22:** Emergency numbers wording (owner challenged 112; two independent checks, see verified facts). Bar:
  "Unofficial, not a government service. Emergency? Call 112 (older numbers 100, 101, 108 may also work). Disclaimer",
  where Disclaimer links to the page. Disclaimer page: 112 is India's national emergency number for police, fire and
  ambulance; the older numbers 100 (police), 101 (fire) and 108 (ambulance) are still listed by many state governments
  and may still work in your state, but some states have retired them, so try 112 first; for floods and cyclones the
  state disaster helpline is often 1070 and the district control room often 1077, which vary by state, so check your
  state disaster management authority's website. This is draft wording: the evidence shows many states still list the
  older numbers, not that they work everywhere, and no source proves they forward to 112, so never say or imply that
  they always work. All numbers live in one setting. A person must check them on the live government pages before any
  promotion (112.gov.in could not be opened in either check). Never present 1070 or 1077 as national numbers. The owner
  saw the first wording "(or 100, 101, 108)" and moved on without changes; the independent review of this file found
  that overstated, so Claude softened it to "may also work" and will tell the owner.
- **D-23:** Two separate pages, public (owner request). **Disclaimer page:** unofficial and not a government service, not a
  substitute for emergency services, the numbers above, how to read Pinalert's labels (Unconfirmed, "confirmed by N
  places"), and the contact line. **Policy page** (rules, privacy and terms): what can be posted and what is screened out,
  how the automatic check works (the text goes to OpenAI's moderation service, which documents no retention for
  `/v1/moderations` and no training use), what is kept (email, reports with location, votes as a rough precision 7
  square), cookies, how to complain or ask for removal, and that the police should be contacted for anything serious
  (plain text only). Both must be readable without logging in, a small exception to Phase 1.1 D-05 ("login to view
  anything") limited to these two pages; the exempt route list in the `internal/api/router.go` doc comment and the pinned
  exempt route test are updated explicitly. Linked from the bar, the login page and the account menu. The text is
  written in plain words from what the app really does and still needs a lawyer or mentor to read it before promotion.
- **D-24:** The contact line (an email for reporting abuse or asking for removal) shows only when an address is set in
  server settings; nothing is public until then. Add "set the contact email" to the launch checklist in `README.md`,
  next to the Resend domain step. The owner has not yet chosen an address; do not invent one.

### Derived rules (worked out from the decisions above, not separately reviewed by the owner)
- **R-01:** Held and withheld states. Moderation state reaches `Resolve` (`internal/service/visibility.go`) through
  `ReportMeta`, which stays pure. It needs a new top rung above Retracted, because an admin Remove must hide even a
  critical report that the critical bypass otherwise keeps Live, and a pending rung for non-critical reports. The outcome
  is not Hidden and not Retracted. New `ResolveReason` slugs are a wire contract (swagger enums in `docs/swagger.json`
  and their drift guards). Reports in these states appear in no Phase 3.1 view (Active, Resolved, Disputed, Past); only
  the author (Activity) and moderators see them. The pinned `NearbyReports` query must not change: moderation state
  arrives through a batched read like `CurrentVotesForReports` and `ReporterAccountsForReports` (keep the constant round
  trip test). Phase 2 D-06 forbids hiding a critical report by votes; a moderation Remove is a separate authority (see
  R-03) and does not contradict it.
- **R-02:** The reporter rating (Phase 3 D-11, D-23) never judges a report because of a moderation outcome. Rejected
  reports are not stored; held, withheld and removed-after-review reports are neutral. Phase 3's classifier calls `Resolve`
  with neutralised metadata and must neutralise moderation state too.
- **R-03:** A critical or rescue report published while the check was pending whose late verdict is a reject is withdrawn the
  same way as a Remove (author told), consistent with D-03. This is an explicit, scoped amendment to Phase 2 D-06 and D-08:
  those decisions forbid hiding a critical report by disputes and make state a pure function of the vote tally, and this
  rule adds a separate authority (moderation) that can withhold it. The owner has not reviewed it separately.
- **R-04:** "Serious" in D-11 is exactly `ReportMeta.criticalBypass()` (`Severity == critical || Category == rescue_needed`),
  so one predicate serves the bypass and the allowance and they cannot drift.
- **R-05:** `created_at` and `expires_at` stay server stamped at insert (`ReportService.Submit`). `SubmitReportRequest` decodes with
  `DisallowUnknownFields`, so the new request id and the display only written-at time must be added explicitly. The
  written-at value is validated and clamped (not in the future, within a sane bound) and is never used for expiry or
  sorting. The request id plus a unique index makes a retry after a lost response unable to insert twice. This relaxes the
  Phase 1 rule "client cannot set any time" (T-01-07) in one bounded, display only place.
- **R-06:** `ValidateSubmitInput` counts description length in bytes with `len` (minimum 10, maximum 1000) while the browser
  counts UTF-16 units. Telugu, Hindi and Tamil characters are 3 bytes in UTF-8, so the limits mean fewer characters than
  the messages say. Screening and the gibberish rule must count characters (runes). Fix the validation only where the new
  screening touches it.
- **R-07:** Public legal pages carry no account data. The account header partial renders `{{.Email}}`, and the main page is
  `Cache-Control: no-store` for that reason (DEC-Q in `internal/api/handlers/page.go`), so the legal pages must not render
  the account header with an email, and being static they may be cacheable.
- **R-08:** Every full page template carries `theme.js` (existing test) and now the bar. Derive the set of full pages from the
  embedded filesystem the way `findFullPageTemplates` does, so a future page without the bar fails the build. New full
  pages: Low data view, Disclaimer, Policy, and the moderator screen. The existing guard of at least 4 full pages stays valid.
- **R-09:** The Low data view is a fourth place for the Phase 3 trust block (feed row, map popup, Activity page, Low data
  view), so it shows the same block built from the same server trust object (`03-CONTEXT.md` D-20, D-21). It shows the
  Active view only; the Phase 3.1 history views and filters stay on the main page.
  (Note 2026-10-16, Phase 3.1 built: the main page list row is now a compact row with a plain text trust line and no
  block, and the popup shows the always open trust card; the Low data row keeps the Phase 3 trust block reading `facts`,
  and shows no tab strip, chips or place search.)
- **R-10:** Demo and seeded data (Phase 5) must not call the moderation provider and must be exempt from the limits by
  construction, as they are exempt from the 1 km rule (`03-CONTEXT.md` Cross phase hand offs).

### Phase 3.1 alignment defaults (2026-10-16, for the Phase 4 UAT list)

Taken when the plans were brought in line with the merged Phase 3.1 code. Each stands unless the owner says otherwise at UAT.

- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): Flag this report stays as the last control of every Low data row, because a Low data row is the whole report there and has no popup to open (04-24).
- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): D-10's vote limits cover all five vote routes, the Phase 3.1 take back included, which shares the resolve and reopen cooldown (04-09, 04-15).
- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): a held or removed report that someone else voted on drops off that voter's My votes list; only its author and the moderators still see it (04-07).
- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): on the Past tab and on a report link the Flag control does not know you already flagged a report, so it reads Flag this report until you press it, and then the server's "already flagged" answer turns it to Flagged (04-07, 04-20).
- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): on the moderator screen a report that faded or was resolved more than 7 days ago still reads Expired or Resolved, not "No longer in any view", and its main page link shows the too old message (04-23).
- RECOMMENDED DEFAULT, owner veto at UAT (2026-10-16): on the compact list row "Written 4 hours ago" sits on the same line after the category and time, not on its own line; the opened report keeps it as a second line (04-17).

### Claude's Discretion
- Reject and borderline thresholds per category, tuned on the D-07 fixture. The alias `omni-moderation-latest` can change
  behaviour without notice, so pin the dated snapshot `omni-moderation-2024-09-26` for reproducible tests and keep a
  regression set. Treat Indian language support as best effort until the fixture proves otherwise.
- Provider client: plain `net/http` and JSON, following `internal/mailer/loader.go` (`LoadMailer`: the real client when a key
  is set, a log only fallback in development, fail fast otherwise; tests never make live calls) and `internal/geocode` (a
  process wide limiter, and the `handlers.GeocodeSearcher` interface with a test stub). Timeout about
  3 seconds; at most 2 retries on 429, 500, 503 or timeout, honouring `Retry-After`, backoff with jitter; never retry credit
  or quota errors; if an SDK is ever used, turn off its automatic retries. Free tier limits are 250 requests a minute, 5,000 a
  day and 10,000 tokens a minute, so a local pre-filter (length, gibberish) cuts calls and the cap falls into the pending
  path. Send one string per request, capped at 1000 characters; no input size limit is documented.
- Account setup risk: the endpoint is free in price, but community threads report `429 insufficient_quota` for accounts with no
  billing. Test a fresh zero balance key early, before relying on "free tiers only" (`PROJECT.md` Budget).
- How the held and withheld states are modelled (a fifth visibility value, or a reason plus a flag), as long as R-01 holds.
- Limiter store: in memory (extend `internal/ratelimit` with a per-account key, its own 429 body and `Retry-After`) or the
  database (the `email_cooldowns` upsert pattern). The free host sleeps and cold-starts, losing memory state; state the choice
  and the consequence. The network address backstop (ROBUST-04 "IP secondary") is very loose and applied only when the real
  visitor address is known: on Render, Railway and Fly.io `r.RemoteAddr` is the platform proxy, so
  `middleware.ClientIPFromRemoteAddr` (what the router uses today) would put every visitor in one bucket. Use
  `middleware.ClientIPFromHeader` with a header named in a deploy setting (CF-Connecting-IP on Render, Fly-Client-IP on Fly.io,
  X-Real-IP on Railway, to be confirmed with a canary request), an empty setting meaning the remote address for local
  development. Both chi variants fail closed (empty address), so define the fallback: skip the network limit, never a shared
  bucket. `PerIP.Middleware` falls back to the remote address host, so it must not be reused unchanged.
- IndexedDB without libraries: wrap open and put in try/catch with an in memory fallback; call `navigator.storage.persist()`;
  flush on the `online` event, page load and a timer; use `AbortSignal.timeout()` and accept both `TimeoutError` and
  `AbortError` (Chrome and Edge 103 to 123 abort with `AbortError`); `crypto.randomUUID()` needs a secure context, so write a
  fallback id generator for plain http testing on a phone over a LAN address.
- Low data view build approach and the byte budget number (document it and test it); polling less often than the main page.
- Exact copy of every message in the Copywriting Contract voice with no dashes, and a Go side test for server authored
  strings (the existing dash guards scan templates and targeted JS only, `03-CONTEXT.md` Claude's Discretion).
- Exact geometry of the bar and the Report control: the UI contract (`/gsd-ui-phase 4`), which must record the amendments to
  the d4 popup design (a quiet Report control inside the 340px card) and to `03-UI-SPEC.md`.
- Disclaimer and Policy text drafts. The link target for state disaster authorities: the SACHET Help Desk list
  (`sachet.ndma.gov.in/HelpDesk`) may be used only as an index of state authority sites, because its numbers are unverified.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project level and requirements
- `.planning/PROJECT.md` - Core Value; Constraints (Budget: free tiers, Legal: IT Rules, LOW confidence, legal review before
  wide promotion); Key Decisions (mandatory login, the OpenAI Moderation API choice); Active "Robustness" list
- `.planning/REQUIREMENTS.md` - ROBUST-01 to ROBUST-04 and ROBUST-08 (the "anonymous session" wording in ROBUST-04 is stale:
  the live rule is the verified account), IDENT-01 to IDENT-05, the v2 PWA-05 (Background Sync)
- `.planning/ROADMAP.md` - "Phase 4" goal and success criteria (criterion 3 is amended by D-02); Phase 8 scope note 4 (reusable
  text moderation hook with a held state and a per account limiter); Phase 3.1 and 3.2 sections; Phase 9 care points
- `.planning/STATE.md` - blocker WR-01 (vote routes have no rate limit); the `Mode: mvp` guard note; the IT Rules LOW confidence
  note; open items (Phase 2 live retests, Phase 1.1 Resend check)

### Locked decisions this phase must not contradict
- `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md` - D-05 (login to view anything; D-23 here
  makes a limited exception), D-08 to D-10 (one year cookie, logout, several sessions per account)
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md` - D-04 (no optimistic vote updates), D-06
  (critical and rescue never hidden by disputes or gated), D-08 (state is a pure function of the tally, nothing latches), D-17 and
  D-18 (GPS capture, denied means blocked)
- `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-CONTEXT.md` - D-01 (1 km vote rule), D-03 (precision 7
  cell), D-10 to D-14 and D-23 (reporter rating, judged reports, display only), D-20 and D-21 (one trust block, one server trust
  object), deferred list (vote rate limiting is this phase), cross phase hand offs
- `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md` - the approved Phase 3 UI contract (it was
  untracked in git when this discussion started and was committed as 065684f while it ran; it was not reviewed during this
  discussion, so read it before the Phase 4 UI contract). Also `03-DESIGN-BRIEF.md`
- `.planning/phases/07-address-search-box-for-report-location/07-CONTEXT.md` - the geocoder proxy and its process wide one request
  per second limit, which the typed place on the Low data view shares

### Research (read the cited sections)
- `.planning/research/PITFALLS.md` - Pitfall 3 (CGNAT and IP limits), Pitfall 6 (moderation calibration, urgent wording and phone
  numbers are legitimate), Pitfall 8 (legal exposure, disclaimer, grievance path, 36 hour figure now stale), Integration Gotchas
  rows for the moderation API and offline queue timestamps, UX table (429 copy)
- `.planning/research/ARCHITECTURE.md` - the `VisibilityResolver` contract with a moderation input, Pattern 4 (async moderation,
  which D-01 deliberately changes), Anti-Pattern 2 (no read path bypasses the resolver)
- `.planning/research/FEATURES.md` - the confidence cascade needs a "flagged, still visible" state; the honest note on a 0.1%
  budget; Background Sync rescoped to IndexedDB plus replay
- `.planning/research/STACK.md` - OpenAI Moderation API row, `golang.org/x/time/rate`, iOS and Background Sync notes
- `PROJECT-NOTES.md` lines 84 to 102 - origin of the cascade, the low bandwidth view, the disclaimer wording
- `.claude/skills/sketch-findings-pinalert/SKILL.md` and its `references/popup-structure.md`, `references/trust-block.md` - the d4
  popup design and the 340px neutral card the Report control must fit; `.planning/sketches/REVIEWS.md` build checklist

### Code to extend (read before planning)
- `internal/service/visibility.go` - `Resolve`, `ReportMeta`, `criticalBypass`, `ResolveReason` (closed wire contract)
- `internal/service/report.go` - `Submit`, `ValidateSubmitInput`, `ExpiryDuration`, `Nearby` (batched reads, ordering is part of
  the specification)
- `internal/api/handlers/reports.go` - `SubmitReportRequest` (`DisallowUnknownFields`), `SubmitReport`, `NearbyReports`
- `internal/api/handlers/votes.go` and `internal/service/trust.go` - the vote routes (four when this was written, five since Phase 3.1 added the take back `unresolve`) and `CastVote` the limits wrap
- `internal/api/router.go` - the exempt route list in the doc comment, the gated group, flat route registration, `r.With(limiter)`
  usage, `ClientIPFromRemoteAddr`
- `internal/api/gate.go` - `requireVerifiedAccount` (401 for `/api/`, 302 to `/login` otherwise)
- `internal/ratelimit/perip.go` - the reusable token bucket (`NewPerIP`, `Allow(key)`, `Middleware`), its 429 body
- `cmd/server/main.go` - named budget constants, environment handling, `Deps` wiring
- `internal/mailer/loader.go`, `internal/geocode/client.go` - the pattern for an outbound client with a development fallback and a
  stub for tests
- `web/templates/` - full pages `index.html.tmpl`, `login_gate.html.tmpl`, `verify_outcome.html.tmpl`, `profile.html.tmpl`;
  partials `account_header.html.tmpl`, `theme_toggle.html.tmpl`, `check_inbox.html.tmpl`
- `web/theme_contract_test.go` (`findFullPageTemplates`), `web/design_rules_contract_test.go` (dash and pill gates),
  `web/css_contract_test.go`, `web/js_contract_test.go`
- `web/static/js/app.js` (`Pinalert.submitReport`, `fetchReports`, `setText`), `modal.js`, `feed.js`, `votes.js`, `map.js`;
  `web/static/css/main.css` (layout, the fixed controls)
- `internal/store/migrations/` (next migration is 00005) and `internal/store/queries/`; `internal/testutil/db.go`
- `.github/workflows/ci.yml` and `Makefile` (tests run with `-p 1` against one Postgres)

### Outside sources behind D-07, D-08, D-18, D-22 and D-23 (checked 2026-10-04)
- OpenAI moderation guide, model page and data controls: `https://developers.openai.com/api/docs/guides/moderation`,
  `https://developers.openai.com/api/docs/models/omni-moderation-latest`, `https://developers.openai.com/api/docs/guides/your-data`
- India 112: `https://www.mha.gov.in/en/commoncontent/nirbhayaerss-section`, Lok Sabha Q.5252 of 24 March 2026
  (`https://www.mha.gov.in/MHA1/Par2017/pdfs/par2026-pdfs/LS24032026/5252.pdf`), Supreme Court order of 26 May 2026
  (`https://indiankanoon.org/doc/79241526/`)
- Browser support: MDN browser-compat-data and caniuse for Network Information, IndexedDB private mode, WebKit tracking prevention
  (`https://webkit.org/tracking-prevention/`)
- chi client IP middleware: the module source `github.com/go-chi/chi/v5@v5.3.2/middleware/client_ip.go`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ratelimit/perip.go` `PerIP`: a self pruning token bucket map around `golang.org/x/time/rate` with a generic `Allow(key)`.
  Reuse the bucket logic with an account key; the 429 body and the IP key are the parts that cannot be reused unchanged.
- `ReportMeta.criticalBypass()`: the one predicate for "serious" (R-04).
- `mailer.LoadMailer` (loader, development fallback, fail fast outside development) and `geocode.NewClient` with the
  `handlers.GeocodeSearcher` interface (process wide limiter, test stub): the patterns for the moderation client.
- `findFullPageTemplates` in `web/theme_contract_test.go`: derives every full page from the embedded templates, the model for the
  "bar on every page" test.
- `Pinalert.setText` and the no markup parsing sink contract tests: the guard that keeps links in report text inert (D-05).
- `votes.js` `getVoterLocation`: per tab `sessionStorage` cache before any browser prompt; `GPS_DENIED_MESSAGE` already says
  "Turn on location access for this site and try again."
- Phase 7's `/api/geocode` proxy and the report modal's search box: the typed place path for the Low data view.

### Established Patterns
- Server computed, client untrusted (Phase 1 T-01-07, Phase 1.1): the server stamps times and decides visibility; R-05 relaxes this
  only for a display only written-at value.
- `Resolve` is the sole visibility authority and `NearbyReports` is pinned byte for byte; visibility is filtered in Go over batched
  reads, never in SQL (R-01).
- Named budget constants in `cmd/server/main.go` with a `Deps` struct; routes registered flat; route scoped middleware through
  `r.With(...)`.
- Contract tests over embedded templates and static JS and CSS; copy rules enforced by tests (`TestUserVisibleCopyUsesPlainPunctuation`,
  `TestNoPillShapedControls`).
- Outbound services: the mailer shows the full pattern (an interface, a real client, a development fallback that fails fast
  outside `ENV=development`, no live calls in tests); the geocoder shows the interface, stub and process wide limiter without
  a development fallback.
- Migrations run as an explicit step, never on boot.

### Integration Points and Landmines
- `POST /api/reports` (`internal/api/router.go` gated group, `handlers.SubmitReport`) gains the screening call, the limiter and the new
  request fields. The vote routes (five since Phase 3.1) gain the vote limiter.
- The report modal cannot run without Leaflet: `initLocation` returns early when `L` is undefined, `validate()` requires `modalMarker`,
  and `buildPayload` reads `modalMarker.getLatLng()` (`web/static/js/modal.js`). The Low data view needs a map free location path;
  extract the form logic so only the location picker differs.
- `web/templates/index.html.tmpl` loads `leaflet.js`, `maplibre-gl.js` and `leaflet-maplibre-gl.js` as classic scripts from unpkg
  before the deferred app scripts, so the feed waits on the map libraries on a weak connection. `map.js` `centerOnVisitor` calls
  `Pinalert.fetchReports()` only after the position resolves, fails or times out (8 seconds).
- `feed.js` `render` hides the list on any failed poll (`state.status === 'error'`) although it keeps the rows (D-20 changes this).
- The router sets the client address with `middleware.ClientIPFromRemoteAddr`; the existing request link and geocode limiters therefore
  already see one shared address behind any host proxy. Hosting is still undecided (`PROJECT.md` Key Decisions).
- `internal/testutil.NewTestDB` truncates the identity and report tables when a test run starts. Never run tests against `pinalert_test` or
  `pinalert` (see Standing rules).
- Phase 3 will add an index on `reports (session_id)` and a trust object on `FeedReportResponse` and `CastVoteResponse`; Phase 3.1 adds a View
  selector and URL state to the list panel; Phase 3.2 adds a bell and Updates. The held and withheld states must not break any of them.

### Verified outside facts that constrain this phase (checked 2026-10-04 by two independent passes each)
- **OpenAI Moderation:** free to call; Free tier 250 requests a minute, 5,000 a day, 10,000 tokens a minute (Tier 1: 500, 10,000, 10,000);
  13 categories (harassment, harassment/threatening, hate, hate/threatening, illicit, illicit/violent, self-harm, self-harm/intent,
  self-harm/instructions, sexual, sexual/minors, violence, violence/graphic) and no spam, scam or advertising category; `category_scores`
  are calibrated 0 to 1; no documented input size limit; for `/v1/moderations` the data controls table shows no training use and no
  abuse monitoring retention; the launch post names Telugu (6.4x), Bengali (5.6x) and Marathi (4.6x) improvements, never Hindi, Tamil
  or Hinglish, and its low resource score is modest (weighted AUPRC 0.359 against 0.653 for all languages).
- **India emergency numbers:** 112 is the official single national number (MHA ERSS page, Lok Sabha answer of 24 March 2026, C-DAC) and the
  Supreme Court (SaveLIFE Foundation v Union of India, 26 May 2026) ordered every State and UT to integrate 100, 101, 102, 108, 1033
  and 1091 into 112 within three months; no compliance report was found. No survey measures what people actually dial, so the owner's
  view that people still use 100 and 108 is plausible and untested. Old numbers are being retired state by state (Nagaland discontinued
  100 on 2026-06-28; Telangana launched a next generation 112 on 2026-08-05 and reports conflict on the old numbers; Gujarat folded fire
  into 112 in six cities on 2026-09-23; Kerala Police's undated page says only the police is integrated). No reliable source proves
  old numbers forward to 112 everywhere. The 112.gov.in FAQ text is garbled about routing; do not quote it. State disaster authorities
  list 112 beside their own lines, often 1070 (state) and 1077 (district), which vary (Andhra Pradesh uses 1070 for both, Kerala also lists
  1079, Maharashtra's FAQ is inconsistent). No dated 2025 or 2026 NDMA advisory naming public numbers was found; NDMA's control room
  is 011-26701728.
- **IT Rules 2021, reported amendment (law firm summaries, medium confidence for the baseline, low for current numbers):** G.S.R. 120(E),
  notified 10 February 2026 and in force 20 February 2026, reportedly cut the takedown window after a court or authorised government
  order from 36 hours to 3 hours, grievance resolution from 15 days to 7, and takedown after a user grievance from 72 hours to 36. The
  baseline expects published rules and a privacy policy, a grievance officer with published contact, 180 day retention of removed content
  and 72 hour information requests. The research files that cite 36 hours and 15 days are stale. Read the Gazette text with a lawyer or
  mentor before promotion.
- **Browsers:** `navigator.connection` (saveData, effectiveType) is Chromium only; IndexedDB works in private modes but is wiped when the
  private session ends; Safari deletes script writable storage after about 7 days of Safari use without interaction unless the site is a
  Home Screen web app; `navigator.onLine` is unreliable, use real request outcomes.
- **Client address behind hosts:** `r.RemoteAddr` is the platform proxy on Render, Railway and Fly.io; chi's own doc example of
  `ClientIPFromXFFTrustedProxies(1)` conflicts with the evidence on Fly.io and Render; Railway staff guidance changed twice in 2026.

</code_context>

<specifics>
## Specific Ideas

- The owner's Instagram idea: "you can comment, you can react to it. If one person tries to open another person's report page, it should
  work like that." And: "Instagram or any other social media works for this type of issue." The owner also described the Report button as
  "exactly what Instagram does", with the admin able to delete a post ("the person who's running the page, someone like me").
- The owner on rescue limits: "How would we know if we are blocking a real emergency? Maybe we should let them do it anyway." Resolved as D-11
  and D-12 (a separate roomy allowance, refilling over time, with the emergency line and a log of blocks).
- The owner on the number: "I'm in India. I don't think we use 112 here. We should use something accordingly." Resolved as D-22 after two
  independent checks. The numbers must live in one setting so a change needs no code change.
- The owner on naming: the Low data view must not be confused with the Light and Dark theme.
- **Interaction note for later phases:** the owner repeatedly set aside pick-a-button prompts (six times in this discussion) and answered well in
  plain typed sentences, often through an Instagram comparison. Ask product questions in everyday words, one at a time, with a recommendation.
- **Standing owner rules that apply to this phase's work** (from the owner's saved notes): never run the Go test suite against `pinalert_test`
  or `pinalert` (it truncates the owner's dev data; use a disposable database, and tell every subagent: read only, never run tests); never use
  ports 8080 or 8090 for anything you start; save no images in the repo; no em dashes or en dashes in any user visible copy or planning prose; the
  site must not look "vibe coded" (no purple gradients, pill buttons, fake metrics, hero text, emoji icons, scroll animations).
- Checks the owner will expect: the fixture test for the text check; limiter tests for separate buckets, concurrency and the 429 body; a test that
  every full page carries the bar; an offline queue test that a retry never inserts twice; the exempt route test updated for the two legal pages; a
  test that a held or withheld report appears in no view; the Low data view size budget test; the moderator screen returns 404 to everyone else.
  Human checks: a zero balance OpenAI key, the numbers on live government pages, and a lawyer or mentor read of the two legal pages.
- Cross phase hand offs: Phase 8 reuses the text check, the per account limiter and the Report button for comments, and its per report page gives
  each report a stable link (takedown notices must name an exact address). Phase 9 reuses the text check for nicknames. Phase 3.2 may show
  "Removed after review" as an update. Phase 5's demo accounts must follow R-10.

</specifics>

<deferred>
## Deferred Ideas

- **Instagram style report page with comments and reactions** (Phase 8, Report page and comments): open any report as its own page, comment, react.
  Reactions on a report already exist as Confirm and Dispute, which are location checked. Open choice for the Phase 8 discussion: whether to also
  allow plain likes, for example on comments. The Phase 9 care points say "no likes so reporting does not become a popularity game", and sketch 002
  deliberately avoids a social feed look.
- The author deleting their own report (Instagram allows deleting your own post). Today an author can mark a report resolved (Phase 2).
- State aware helplines: store the person's state at report time and show that state's own disaster management authority link instead of one static
  string, so a wrong number cannot ship when a state changes its line.
- An installable app, a service worker and Web Push (backlog 999.3). The owner asked to keep it a website.
- Image safety screening (stretch, tied to photo attachment).
- Account level sanctions for repeat offenders (strikes, suspension): not discussed; needs its own scoping.
- Email alias normalisation and disposable email blocking to raise the cost of extra accounts (already on the Phase 3 deferred list).

### Reviewed Todos (not folded)
- **Real 3D WebGL globe on login screen** (`.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`): matched on the keyword "real"
  and area "ui" only, as in Phase 3. It concerns the login page visual, and a WebGL globe would make the login page heavier, the opposite of this
  phase, so it was not folded.

</deferred>

---

*Phase: 04-Robustness, Real-World Resilience*
*Context gathered: 2026-10-04*
