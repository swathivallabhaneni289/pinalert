# Phase 3: Trust-Model Hardening, Diversity-Weighted Trust - Context

**Gathered:** 2026-09-30
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase makes the trust numbers a reader sees on a report resistant to trivial gaming and easy
for a first time user to understand. It delivers TRUST-05 (a "confirmed by N people nearby" count
computed from distinct confirming places, not raw votes, with a deliberately chosen and documented
geohash precision) and TRUST-07 (two distinct trust signals shown side by side on every report: a
fast decaying "is this still current" signal, and a slow decaying "is this source reliable" signal,
which is a public rating of the reporter).

It also makes the word "nearby" true for the first time: today no code compares a voter's location
with the report's location, so a voter anywhere on earth counts as an independent nearby
confirmation. After this phase a vote from more than 1 km away is rejected.

The trust block is one shared piece, shown identically on the feed row, the map popup and a
reporter's own reports on the Activity page. Both signals are display only: they never change a
report's visibility, never hide or block anything, and never dim critical or rescue needed rows
beyond what Phase 1's time fade already does (Phase 2 D-06 and D-08 stand unchanged).

Not in this phase: comments and an X style report page (deferred, needs its own phase), triage
ordering by these signals (Phase 6), vote rate limiting (Phase 4), demo mode (Phase 5).

Planner note: the distance rule (D-01), its client hints (D-02, D-22) and the Activity page block
(D-20) are owner requested additions that support the two roadmap criteria. If the plan has to be
cut, cut D-22 first. The server side check in D-01 is what makes "nearby" true and is not optional.

</domain>

<decisions>
## Implementation Decisions

### Counting rule for N (TRUST-05)
- **D-01:** The server **rejects a vote cast from more than 1 km from the report**. It covers all
  four vote values (confirm, dispute, resolve, reopen) across both vote kinds. The reporter's own
  instant resolve and reopen are exempt, because those votes are uncounted and instant (Phase 2
  D-13, amended D-16). This mirrors the GPS denial block (Phase 2 D-18): a vote that cannot be
  location backed is refused, never accepted and silently ignored. 1 km was chosen over 2 km and
  over "no distance rule". The radius lives in one named server side constant. No schema change is
  needed, because the check runs at write time. Votes written before Phase 3 were never distance
  checked and cannot be re-checked (only the voter's cell is stored, not the coordinates), so they
  are grandfathered and keep counting. The docs must say so, and the too far tests should run on a
  fresh or reset database.
- **D-02:** Because 1 km is tight, the client refreshes the voter's location **silently when
  permission is already granted** and retries once with a fresh fix before showing the too far
  error. It **never prompts per vote** (Phase 2 D-17 stands). Where permission is not persistent,
  the cached fix is reused. Today's sessionStorage location cache has no expiry, so without this a
  voter who walked to the incident after caching a home position would be wrongly rejected. This was
  announced right after the owner chose 1 km and was not objected to; the mechanism is the planner's
  call.
- **D-03:** The voter cell precision **stays at 7** (cells are 153 m tall and about 125 to 151 m
  wide at India latitudes, because the width is 152.9 m times the cosine of the latitude: about 134 m
  at Delhi and about 149 m at Chennai). Votes stay written at precision 7. Phase 3 closes
  the interim status of that value (02-RESEARCH assumption A3): the value is now a deliberate,
  documented choice. Deliverables: a derivation table for precisions 5 to 8 in the docs, and a test
  that simulates witnesses in a 300 m circle against distinct cell count. The docs must state the
  limits honestly: two accounts a few metres apart across a cell edge count as two; one account
  holds one current vote so walking around cannot add cells; the real gaming cost is extra
  verified email accounts plus a claimed location; client coordinates cannot be verified, so a
  spoofed location defeats both the cell rule and the 1 km rule. The docs must not claim spoof
  resistance.
- **D-04:** **N equals the distinct place count the resolver already gates on** (`ConfirmCells`
  from `BuildVoteTally`). There is one precision constant and one counting rule for the gate, the
  display, the still current signal and the reliability classifier (Phase 2 D-14). There is no
  dampening curve. N counts distinct places among verified accounts, so it is a lower bound on
  people: several people in one place count once.

### Still current signal (TRUST-07, first half)
- **D-05:** The signal reads **recent only when at least 2 distinct confirming places** confirmed
  inside a window of **one quarter of the report's lifetime** (2 hours for 8 hour reports, 6 hours
  for 24 hour reports). It uses the same cell derivation as N: reporter excluded, one current row
  per account, content confirm votes only (resolve and reopen never refresh it). One account
  re-tapping Confirm cannot refresh it, because only distinct places count. Faking it costs 2
  verified accounts and 2 claimed locations, the same as forging Live.
- **D-06:** With fewer than 2 confirming places the state is **not yet corroborated**: no fade and
  no negative wording, and it never reads as stale, so a new critical report is not undercut. The
  threshold is the same one Provisional uses, so there is no gap at exactly 1 confirmation (exact
  definition: R-01).
- **D-07:** **Fade clock.** For non critical rows the fade stage is the worse of Phase 1's time
  based stage (`ageStage`) and the server authored still current stage, so a row never shows two
  contradictory freshness cues. **Critical and rescue needed rows keep the time based fade only**
  and show the still current word as plain text with no extra dimming, so a soft signal never
  mutes a possible emergency. `expires_at` stays the hard cutoff and expiry does not change. The
  browser renders the server band and never re-derives it (mapping in R-02).
- **D-08:** **Wire form for both signals:** a band, the real facts behind it, and an **integer 0
  to 100 score** (the user chose this over bands plus facts alone). The score comes from a
  documented formula over real votes, is recomputed on every read and is never stored (Phase 2
  D-08). Band thresholds are cut from the score so the two cannot disagree. Phase 6 may sort on the
  integers. The plan and the docs must record how the roadmap's word "score" is satisfied. Scores are null in
  the cases listed in R-01.
- **D-09:** The 0 to 100 number is shown on screen **in the map popup only** (small, no extra
  sentence). The feed row shows only the band word inside the D-18 block. The facts behind a band
  appear only in the hover reason (D-19) and in the wire object. A source with no rating has no
  number (R-01).

### Source reliable signal (TRUST-07, second half)
- **D-10:** Reliability is a **public, person level rating of the reporter**, keyed to the
  verified account through `sessions.account_id` (never `session_id`, because one account can hold
  several sessions). The rating, the tag and the counts behind it are visible to everyone on every
  report. The account id and the email are never shown. Linking one person's reports together is
  intended, not a privacy risk. Official GDACS pins (Phase 5) and accountless reports have no
  rating. Confirmer accuracy scoring stays deferred (TRUSTX-01). "Visible" means the tag on every
  block (D-18), the counts in the hover (D-19) and the number in the popup (D-09).
- **D-11:** **Outcomes.** Only **expired** reports are judged, because a vote on an expired report
  is rejected, so its tally is frozen and its outcome cannot change. Corroborated (2 or more
  independent confirming places, not out disputed) counts for. Contradicted (Hidden by dispute)
  counts against. **A report that expires never confirmed counts as a mild negative**, weighted
  less than a contradicted outcome (the user chose this over corroboration only). Reports still in
  flight, and the reporter's own resolve or reopen, are neutral, so self resolving cannot erase a
  bad record. Classification calls `Resolve` with neutralised metadata (non critical severity, non
  rescue category) and a tally with the resolution fields zeroed, then maps Live by confirmation to
  corroborated, Hidden by dispute to contradicted and Provisional to unconfirmed. It must never
  copy `Resolve`'s thresholds. What counts as judged is in R-05, and how an out disputed critical
  report is treated is in R-06.
- **D-12:** **Critical and rescue needed reports are exempt from the unconfirmed penalty.** Their
  expiry without confirmation is neutral, while corroborated or contradicted outcomes still count.
  This keeps the intent of Phase 2 D-06: nobody is marked down for reporting a possible emergency
  that no one nearby happened to confirm. R-06 covers critical reports that are out disputed.
- **D-13:** **Minimum evidence and decay.** Below **5 judged reports** (corroborated, contradicted
  or expired unconfirmed) the source is a **New reporter** with no rating and no number, never a
  default score. Each outcome's weight halves every **30 days**, only the last 90 days count, and
  the read is capped at the latest 30 judged reports per account (which also bounds the query). The
  minimum counts judged reports inside that window and cap without weighting, so decay changes
  weight, not eligibility (R-05). The 90 day and 30 report bounds were part of the option shown to the
  owner. The user accepted that on a small deployment most sources will show New reporter.
- **D-14:** **Display only, shown everywhere.** The tag appears on every report including critical
  and rescue needed ones, as the same plain text. It never hides, dims, blocks, reorders or filters
  anything in Phase 3. Phase 6 may use it as one soft server side triage input.
- **D-15:** **Tags, in everyday words:** **Reliable reporter**, **Mixed record**, **Unreliable
  reporter**, **New reporter**. They came from the user's rule that a first time user must
  understand a tag at once, and that it must be short. One word tags and full sentence tags were
  both rejected. The row shows the tag only. Thresholds are cut from the reliability score. **New
  reporter must never look like a good tag.** Known limits to document: a liar can start over with a
  fresh email to shed a bad tag, and two colluding accounts could dispute an honest reporter
  (brigading detection is deferred, TRUSTX-03).

### Trust block display
- **D-16:** **Count line:** `Confirmed by N people nearby` for any N of 1 or more (`Confirmed by 1
  person nearby` at one) and nothing at zero. The noun "people" was added after the user asked
  "confirmed by three what?" and matches the wording in the original project description. When
  someone disputed, the same line ends with `, M disputed` (only when M is 1 or more; M is the
  distinct dispute place count, R-04). Raw vote totals are never shown.
- **D-17:** The **Unconfirmed chip keeps its wording** (the user chose this over renaming it to
  "Needs 2 confirmations"). At exactly 1 confirmation the row shows the chip and `Confirmed by 1
  person nearby` together, and the user accepted that. Hovering the chip explains: "Needs 2
  confirmations before this counts as confirmed."
- **D-18:** **Layout and words.** Two short lines of plain text in the normal text color (color is
  reserved for severity): line 1 is the count line, line 2 is the still current word and the
  reporter tag side by side, stacking on very narrow phones. No chips, bars, colors or icons. Still
  current words: **Up to date**, **Getting old**, **Needs re-confirming**, and **Too early to
  tell**. "Too early to tell" appears only when no Unconfirmed chip is showing, that is on critical
  and rescue needed reports with fewer than 2 confirming places, and it is identical for every
  severity with no warning phrase. On a Provisional row the second line shows only the reporter
  tag, and so does a Hidden row in the Show disputed view (R-03). The user asked that the boxes not be filled with too many words, be simple and accessible,
  and put detail on hover without feeling clumsy.
- **D-19:** **Reasons on hover.** Hover on desktop, tap on touch and focus on keyboard (dismissible
  with Escape) each show one short sentence with real numbers: the count line ("3 people in 3
  different places confirmed this. Counts separate places, not votes."), the still current word ("2
  different places confirmed this in the last 2 hours."), the reporter tag ("5 of 7 earlier reports
  were confirmed by people nearby.") and the Unconfirmed chip (D-17). These four sentences are the
  approved wording. N is a lower bound on people (D-04), and the second half of the count sentence
  carries that caveat. Hover sentences for the other words and tags are left to Claude's Discretion.
  Most phones have no hover, so tap and focus are required, not optional.
- **D-20:** **One block, identical everywhere.** The same block appears on the feed row, the map
  popup and each of the reporter's own reports on the Activity page. There is no reporter only
  variant and no separate wordy rating line, because the tag on a reporter's own reports is how they
  see their own rating. The popup's only extra is the two small numbers (D-09). The Activity page
  refreshes the block after Reopen, its only vote action (Mark resolved exists only on the feed row
  and the popup), so it never goes stale; details in R-08. This replaces the earlier working
  assumption that the Activity page stays unchanged.
- **D-21:** **One server built trust object** is returned by both `GET /api/reports` (the
  `FeedReportResponse` element) and the vote response (`CastVoteResponse`), so the two cannot
  drift. The voter's cell and the account id never appear on the wire. The OpenAPI spec is
  regenerated and its drift guards are extended (see Claude's Discretion).
- **D-22:** **Voting from too far.** Once the voter's location is known, the vote buttons for a
  report more than 1 km away are **disabled with a hover reason**: "Too far to vote. You need to be
  within 1 km of this report." Before the location is known the buttons stay enabled, so the first
  tap still asks for GPS as today (Phase 2 D-17). It applies to Confirm, Dispute and Mark resolved on
  rows the viewer did not post. Non reporter Reopen has no UI surface today and is enforced by D-01
  alone. The reporter's own instant resolve and reopen stay enabled. The server remains the real
  gatekeeper (D-01), so the disabled state is only a hint. The option the owner chose carried a small
  distance check in the browser: the browser computes one great circle distance between the cached
  voter location and the report's coordinates, only to decide the hint, against the radius the server
  supplies through the page config (a new field sourced from the same constant as D-01). That is the
  only geometry allowed in JavaScript and it grants nothing. The 1 km figure in the sentence is built
  from that supplied radius, never typed into a template or script.

### Derived rules (worked out from the decisions above, not separately reviewed by the owner)
- **R-01:** **Not yet corroborated** means fewer than 2 distinct confirming places over the report's
  whole life (the same `ConfirmCells` the resolver gates on), not inside the window. In that state
  the band is `not_yet_corroborated` and the still current score is **null, never 0**. With 2 or more
  confirming places the score is computed, the words Up to date, Getting old and Needs re-confirming
  are cut from it, and Up to date requires at least 2 distinct confirming places inside the window.
  The reliability score is likewise null for a New reporter. No number is ever shown when a score is
  null, and Phase 6 treats null as unknown.
- **R-02:** **Fade mapping.** The server sends a fade stage (fresh, aging or stale) for non critical
  rows only: Up to date and not yet corroborated give fresh, Getting old gives aging, Needs
  re-confirming gives stale. Critical and rescue needed rows get none. The browser only takes the
  worse of that field and its own time based stage, in every place the fade is applied (including the
  60 second `refreshAgeAndTime` timer in `feed.js`), and holds no thresholds. Intended consequence of
  D-07: a non critical report that no 2 places have re-confirmed inside the window dims earlier than
  Phase 1's last quarter rule.
- **R-03:** On a **Hidden** row (Show disputed view only) line 1 is the count line with the dispute
  count and line 2 shows only the reporter tag, like a Provisional row. No still current word and no
  still current fade contribution appear there.
- **R-04:** In `, M disputed`, M is `DisputeCells` from the same `BuildVoteTally`, the distinct
  dispute places, so it obeys the same independence rule as N.
- **R-05:** **What counts as judged (D-11, D-13).** A report is judged once its `expires_at` is at or
  before now, whether or not it was Retracted. The reporter's own resolve and reopen votes are ignored
  when classifying, so a self resolved report is judged by its content votes only. Neutral outcomes
  (reports still in flight, and critical or rescue needed reports that expired unconfirmed, per D-12)
  are not judged and do not count toward the 5. The 5 report minimum counts judged reports inside the
  90 day window and the 30 report cap, unweighted. Age for the window and the half life is measured
  from `expires_at`. The reporter tag hover shows the same raw counts: corroborated reports out of
  judged reports.
- **R-06:** **Out disputed critical reports.** Neutralising the metadata in the classifier is
  deliberate: a critical or rescue needed report that independent places out dispute is judged
  contradicted for the reporter's rating only. It is still never Hidden, because the real `Resolve`
  call on the read path keeps its critical bypass, and the rating is display only (D-14). D-12's
  exemption covers only the unconfirmed penalty. This default was not confirmed separately with the
  owner. If they would rather not penalise, judge critical and rescue needed reports only when
  corroborated.
- **R-07:** **Popup numbers.** In the map popup each number sits directly after its own word on
  line 2 (for example "Up to date 82"), with no "/100", no bars and no icon, and the hover reason
  explains it. Nothing is shown when the score is null.
- **R-08:** **Activity page.** The block appears on reports the account posted; reports the account
  only voted on keep their current display. A Retracted or expired report there shows the count line
  and the reporter tag only, with no still current word. The only vote control on that page is the
  reporter's own Reopen, so the block updates from the trust object in `CastVoteResponse` after
  Reopen, and the page has no too far hint.

### Claude's Discretion
- The two 0 to 100 score formulas and the band and tag cutoffs. Leans from research: still current
  decays from the time the second most recent distinct confirming place confirmed, with the window as
  its scale; reliability is a smoothed (for example Wilson lower bound) share of decayed
  corroborated over decayed judged outcomes, with expired unconfirmed as a fractional failure. Whatever
  is chosen must be documented, table tested at the boundaries with an injected `now`, and free of
  invented inputs.
- What a standing independent dispute does to still current (lean: it cancels one confirming place
  inside the window, net floored at 0; a dispute never refreshes).
- Where the pure functions live (lean: beside `Resolve` in `internal/service`, for example a new
  `reliability.go`) and the input shape. `Resolve` keeps its signature and stays the only visibility
  authority. Reliability and currency must reuse `BuildVoteTally` and `independentCellCount`, with
  no second threshold constant and no second cell derivation.
- Wire field names and enums, and whether the count wording is server authored or built in the
  browser from integers. No threshold may live in JavaScript. Any server authored string needs a Go
  side test that forbids long dashes, because the existing dash guards scan templates and targeted
  JS only.
- The hover sentences not fixed in D-19 (suggested: Too early to tell, "Fewer than 2 places have
  confirmed this so far."; New reporter, "Fewer than 5 finished reports so far."; the Mixed record and
  Unreliable reporter reasons with their raw counts; Getting old and Needs re-confirming with when the
  last confirmations were), a neutral variant of the too far sentence for Mark resolved, and an
  accessible tooltip built for hover, tap and focus rather than the native `title` attribute alone.
  Every word and tag needs a hover sentence with its real numbers. No `aria-live` on the block,
  because the 30 second poll would re-announce it.
- Where the blocks sit in the DOM, a new `trust.js` mirroring `visibility.js` (create once, update
  per poll), and the CSS, kept to the existing tokens, four type sizes and two weights, in the
  normal text color (muted text fails AA contrast on light tints).
- How the Activity page gets the block (server rendered from the same source, or fetched).
- The weight of the mild unconfirmed negative, and a cap such as `99+` for very large counts.
- The mechanism for the silent location refresh, the status code and copy of the too far
  rejection (in the Copywriting Contract voice, no dashes), and the column shape of the reliability
  history read (one bounded batched query keyed by the page's distinct reporter account ids, then one
  `CurrentVotesForReports`, grouped in Go, with independence never aggregated in SQL).
- `ExpiryDuration` depends on severity only, so a low severity rescue needed report has an 8 hour
  lifetime although the critical bypass treats it as critical. The still current window inherits that
  lifetime; note it in the docs.
- The single schema change: an index on `reports (session_id)`, probably `(session_id,
  expires_at DESC)`, with an EXPLAIN based index test like the Phase 1 one.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project level and requirements
- `.planning/PROJECT.md` - Core Value, the trust-model hardening list, Key Decisions (DB hosting row
  is still Pending and is unaffected by this phase)
- `.planning/REQUIREMENTS.md` - TRUST-05 and TRUST-07 (TRUST-07 says "score"; the shipped form is
  D-08), TRUSTX-01 and TRUSTX-03 (deferred). TRUST-03 still says "distinct anonymous session"; the
  live rule is Phase 2 D-14 (distinct verified account and distinct cell)
- `.planning/ROADMAP.md` - "Phase 3" goal and success criteria, and "Phase 6" (triage consumes these
  signals, so they need machine readable values)
- `.planning/STATE.md` - blocker "Geohash cell-size precision has no benchmarked value" (closed by
  D-03; its wording that implies two constants is superseded by D-04)

### Locked decisions this phase must not contradict
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md` - D-06 (critical
  never gated or hidden by soft signals), D-08 (state is a pure function of the tally, nothing
  latches), D-13, D-14 (one independence rule), D-16 amended, D-17 and D-18 (GPS capture and the
  denied block)
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md` - assumptions
  A2 and A3 (precision 7 was interim)
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md` - approved
  design contract, Copywriting Contract, token limits, and the line 199 re-tap contract, which stays
- `.planning/phases/01-foundation-report-map/01-CONTEXT.md` - D-14, D-15, D-17 (expiry tiers and the
  time fade this phase combines with) and the neutral utility direction
- `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md` - verified
  accounts, and D-13 (the merged Activity section)
- `.planning/phases/01-foundation-report-map/assets/01-theme-reference.png` - approved visual
  language for "Confirmed by N nearby" and "Unconfirmed"
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-VERIFICATION.md` and
  `02-UAT.md` - Phase 2 status (see the sequencing note in Specific Ideas)

### Research
- `.planning/research/PITFALLS.md` - Pitfall 10 (precision against a 100 to 300 m incident radius,
  and the line 548 warning to sanity bound client geolocation) and Pitfall 4 (Sybil gaming)
- `.planning/research/ARCHITECTURE.md` - the VisibilityResolver contract, Pattern 3 (independence
  predicate then weighting), Anti-Patterns 1 to 3, and TrustScoringService as pure functions over
  votes. Pattern 3 Layer B proposes a dampened displayed N; D-04 declines it and it is deferred
- `PROJECT-NOTES.md` - origin of the confidence versus reliability split. Its stored score columns
  (line 237) and `.planning/research/SUMMARY.md` line 53 conflict with Phase 2 D-08 and must not be
  followed
- `.planning/research/FEATURES.md` - lines 33 and 69 key reliability to an anonymous session
  (stale: the live rule is the verified account, D-10)

### Vote and visibility code
- `internal/service/trust.go` - `CastVote`, `BuildVoteTally`, `independentCellCount`,
  `voterGeohashPrecision`: every counting and write time rule, and the missing distance check
- `internal/service/visibility.go` - `Resolve` (unused `now` parameter), `VoteTally`,
  `criticalBypass`, `IndependentAgreementThreshold`
- `internal/service/report.go` - `Nearby` (feed read path), `ExpiryDuration` (severity only: 24
  hours critical, 8 hours otherwise), `BoundingBox`, `geohashPrecision` (reports use precision 8)
- `internal/service/feed.go` - `ReportView`, `ListableInFeed`, `ViewerContentVote`
- `internal/service/auth.go` - `ActivityForAccount`, the third caller of `Resolve` and the existing
  per account history composer
- `internal/store/queries/votes.sql` - `CurrentVotesForReports` (latest row per account and kind,
  with `created_at`), `ReportVoteContext` (needs the report's latitude and longitude for D-01)
- `internal/store/queries/reports.sql` - `NearbyReports` (pinned byte for byte by a drift constant
  and an EXPLAIN test: do not modify), `ReportsByAccount`, `ReporterAccountsForReports`
- `internal/store/migrations/00001_create_reports.sql` - shows there is no index on
  `reports.session_id`
- `internal/store/migrations/00002_add_accounts_and_verification.sql` - `sessions.account_id`
  (nullable) and `idx_sessions_account_id`
- `internal/store/migrations/00004_create_votes.sql` - append only log, server default `created_at`,
  the never serialise the cell rule
- `internal/testutil/seed.go` - seeds accountless reports (a NULL reporter account)

### API surface
- `internal/api/handlers/votes.go` - `CastVoteRequest`, `CastVoteResponse`, error mapping, swagger
  annotations
- `internal/api/handlers/reports.go` - `FeedReportResponse`, `reportViewToResponse` (wire keys
  `visibility`, `visibility_reason`, `your_vote`, `is_own_report`)
- `internal/api/handlers/auth.go` - `profileReport` and `newProfileReport` for the server rendered
  Activity page
- `internal/api/handlers/swagger_test.go` - drift guard (`TestSwaggerSpecCoversRoutes`); it forbids
  `session_id` and `reporter_account_id`, and needs new field names plus a `geohash_cell` guard
- `docs/swagger.json` - regenerate after any wire change
- `cmd/server/main.go` - `defaultRadiusKm = 10.0` (also a constant in `handlers/reports.go`) and the
  place the page config is built

### Front end
- `web/static/js/votes.js` - `getVoterLocation` cache, the GPS denied block, the post vote
  `fetchReports` refetch, the re-tap behavior
- `web/static/js/visibility.js` - chip label table (Live shows none) and the render server fields
  only pattern to copy
- `web/static/js/feed.js` - `createRow` and `updateRow` (where the block mounts), and the `ageStage`
  callers `updateRow` and the 60 second `refreshAgeAndTime` timer
- `web/static/js/map.js` - `buildPopupContent`, rebuilt on every render, and the `ageStage` callers
- `web/static/js/app.js` - `ageStage` (fresh above 0.25 of lifetime left, aging from 0.125 to 0.25,
  stale below 0.125), `setText`
- `web/static/js/activity.js` - the reopen path, which does not refetch today
- `web/templates/index.html.tmpl` and `web/templates/profile.html.tmpl` - the feed page and the
  server rendered Activity page
- `web/static/css/trust.css`, `web/static/css/main.css`, `web/static/css/feed.css` - the chip and
  state cascade, design tokens, row geometry and muted text contrast
- `web/design_rules_contract_test.go` - the long dash and pill shape gates new copy and CSS must pass

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `BuildVoteTally` and `independentCellCount` in `internal/service/trust.go`: the only place raw
  vote rows become independent cells. Currency and reliability reuse them, so no second cell
  derivation exists.
- `Resolve` in `internal/service/visibility.go`: reused by the reliability classifier with
  neutralised metadata, so no second copy of its Hidden and Live predicates exists.
- `CurrentVotesForReports` and `ReporterAccountsForReports`: batched reads already used by the feed
  path. The reliability read follows the same batched shape.
- `ActivityForAccount` in `internal/service/auth.go`: the existing per account history composer.
  Generalise it, but bound it (window and row cap) and never reuse it unbounded on the feed path.
- `visibility.js` and `trust.css`: the create once, update per poll builder pattern and the state
  cascade that the new block extends.
- `BoundingBox` in `internal/service/report.go`: a usable prefilter for the distance rule.

### Established Patterns
- Server computed, client untrusted: the browser renders `visibility`, `visibility_reason` and the
  new trust fields and computes nothing. Every string reaches the DOM through `Pinalert.setText`,
  never a markup parsing sink.
- The vote log is append only and every state is recomputed on read. Nothing latches and no score is
  stored (Phase 2 D-08).
- Pure functions in `internal/service` that are table tested with an injected clock, and batched reads
  to avoid N+1.
- OpenAPI drift guards pin handlers to `docs/swagger.json`; migrations run as an explicit step, never
  at boot.
- There is **no Go Haversine helper**. The only Haversine is SQL inside the pinned `NearbyReports`
  (plus a copy in `internal/store/reports_test.go`), so the distance rule needs a new small Go
  function or a SQL expression added to `ReportVoteContext`.

### Integration Points
- `CastVote` (`internal/service/trust.go`) gains the distance check, after validation and before the
  insert. `ReportVoteContext` gains `r.latitude` and `r.longitude`.
- `FeedReportResponse` and `CastVoteResponse` gain the shared trust object. `Nearby`, `CastVote` and
  `ActivityForAccount` (the three callers of `Resolve`) all attach it, so `CastVote` also loads the
  reporter's history for that one report (one extra query).
- `feed.js createRow` and `map.js buildPopupContent` mount the block at the existing visibility chip
  insertion points. The popup is rebuilt on every render, and the feed row is created once and
  rewritten idempotently on each poll.
- The `ageStage` callers take the combined stage from D-07 and R-02: `feed.js` `updateRow` and the 60
  second `refreshAgeAndTime` timer (otherwise the timer reverts the fade to the time only stage), and
  the three call sites in `map.js`.
- `profile.html.tmpl` and `activity.js` render the same block on a reporter's own reports (D-20).
- The page config (`cmd/server/main.go`, `handlers.PageConfig`) supplies the 1 km value to the
  browser (D-22).

</code_context>

<specifics>
## Specific Ideas

- The user's own words on reliability: each person should have a rating based on how their reports
  turned out and on whether people confirmed the situation, visible to others, "and that's how the
  other people know that this person, what he posted was true and they can trust it rather than
  people just lying", with a tag so people know when a person is not reliable.
- The user's own words on wording: the tags must be "very clear for the new user" and not too long;
  the boxes "should not be filled with too many words, make it simple and accessible"; more detail
  belongs on hover; and it must not be clumsy. What readers see and what reporters see should be the
  same.
- "Confirmed by three what?" is the test every new string must pass: a first time user reads it
  without help.
- The user wants clicking a report to open something like a post on X, with a discussion thread of
  what people are saying about the situation. That is deferred (see below). The trust block is
  deliberately one reusable piece so it can drop into that future view unchanged.
- Known stale wording in older documents, not to be copied into Phase 3 plans: TRUST-03's "distinct
  anonymous session" (live rule: Phase 2 D-14); "reliability keyed to an anonymous session" in
  FEATURES.md (lines 33 and 69) and PROJECT-NOTES.md (line 240) (live rule: the verified account); "reports fade unless re-confirmed"
  (expiry is flat by severity and nothing extends it); and the STATE.md blocker's implication of two
  precision constants (D-04: one).
- Sequencing note: ROADMAP.md marks Phase 2 complete, but `02-VERIFICATION.md` (2026-09-18,
  human_needed, 8 of 13 must-haves) predates UAT round 3 and plans 02-11 to 02-14, and `02-UAT.md` is
  fixes_shipped_pending_retest (the provisional dimming, Mark resolved button hide and UI polish fixes
  await a live retest). Phase 3 builds directly on the resolver, so running `/gsd-verify-work 2` before
  executing Phase 3 is advisable. Phase 1.1's live email delivery check is a separate open item.
- Cross phase hand offs: Phase 5 demo mode must handle far away visitors, who cannot vote on
  seeded reports under D-01 (either simulated reports are placed near the visitor, or demo reports
  are exempt from the distance rule), and it is the place to seed clearly labelled demo accounts if
  a demo should show a rated reporter. Phase 6 sorts on the integer scores server side, must never use
  them to bury a critical or rescue needed report (D-14), and must never bake a count or rating into a
  shareable card image.

</specifics>

<deferred>
## Deferred Ideas

- **Report page with comments** that opens like a post on X, showing what people are saying about
  the situation: a new capability needing its own phase, including moderation and abuse scoping for
  free text tied to real accounts. The user wants it soon. Suggest adding it with `/gsd-phase`.
- Re-confirmation extending `expires_at` so "fades unless re-confirmed" is literally true (touches
  FOUND-05 and the pinned `NearbyReports` predicate). Flagged to the user during the still current
  discussion; not requested for this phase.
- Demoting stale non critical reports to Provisional, or dropping them from the feed before expiry
  (would make visibility depend on `now`, against Phase 2 D-08).
- A diversity dampening curve for the displayed N (ARCHITECTURE Layer B), only if a plain distinct
  place count proves insufficient.
- Sending browser GPS accuracy with the vote, per category or per severity radii, and detecting
  spoofed or mock location.
- Burst and brigading detection, including a cap on how much one confirmer pair can contribute to a
  reporter's record (TRUSTX-03), and confirmer accuracy scoring (TRUSTX-01).
- Vote rate limiting and re-cast cooldown messaging (Phase 4, ROBUST-04).
- Email alias normalisation (plus addressing, dot variants) and disposable email blocking to raise
  the cost of extra verified accounts (identity follow up on Phase 1.1).
- Push notifications when N crosses a threshold or a confirmed report goes quiet or is retracted.
- A public reporter profile page or badge tiers beyond the four tags.
- A stored or materialised reliability score refreshed by a job, only if measured recompute cost
  ever becomes a problem (conflicts with Phase 2 D-08 today).
- The database hosting choice (Supabase, Neon, Render or Railway): the PROJECT.md Key Decision is
  still Pending and this phase does not depend on it. External claim to re-check when the decision
  is made: Supabase's free plan pauses projects after 7 days of low activity
  (https://supabase.com/docs/guides/platform/free-project-pausing), which matters for keeping the demo
  live.

### Reviewed Todos (not folded)
- **Real 3D WebGL globe on login screen** (`.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`):
  matched on the keyword "real" and area "ui" only. It concerns the login page visual and has no
  bearing on trust signals, so it was not folded into this phase.

</deferred>

---

*Phase: 03-trust-model-hardening-diversity-weighted-trust*
*Context gathered: 2026-09-30*
