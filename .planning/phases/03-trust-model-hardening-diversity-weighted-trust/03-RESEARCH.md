# Phase 3: Trust-Model Hardening, Diversity-Weighted Trust - Research

**Researched:** 2026-09-30
**Domain:** Go service layer, sqlc/goose/Postgres, swag wire docs, vanilla JS trust block, contract-test-heavy repo
**Confidence:** HIGH on repository facts (every claim below was checked against live code this session), MEDIUM on score constants (design choices, flagged), MEDIUM on cross-browser behaviour (two items tagged ASSUMED)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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

### Deferred Ideas (OUT OF SCOPE)
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
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TRUST-05 | The feed displays "confirmed by N nearby" computed from the count of distinct confirming geohash cells/sessions, not the raw number of votes. Shipped form: D-01 (distance rule that makes "nearby" true), D-03 (documented precision 7), D-04 (N is `ConfirmCells`), D-16/D-17 (count line). | Sections "Distance rule", "Change map", "Score formulas" (N needs no formula), "Wire and docs", "Front end", "Validation Architecture" rows V-01 to V-12 |
| TRUST-07 | A report displays two distinct trust signals side by side: a fast-decaying "is this still current" confidence score and a slow-decaying "is this source reliable" score. Shipped form: D-08 (band, facts, integer 0 to 100), D-05 to D-07, D-10 to D-15, R-01 to R-08. | Sections "Score formulas" (concrete formulas, worked examples, boundary tables), "Reliability history read", "Front end", "Validation Architecture" rows V-13 to V-40 |
</phase_requirements>

## Read this first (things that change how the planner should behave)

1. **Incident report from the research session.** Early in this session the researcher ran the full test suite once against the `pinalert_test` database to record a baseline, before reading `02-14-PLAN.md`, which says `pinalert_test` holds the owner's dev server manual testing data and must never be targeted. `testutil.NewTestDB` truncates `accounts, magic_link_tokens, email_cooldowns, reports, sessions, votes`. That data is gone (afterwards `pinalert_test` held 0 accounts, 0 votes, 1 seeded report; the separate `pinalert` database was not touched). The owner will need to log in again on their dev server and any hand made test reports there are lost. Everything after that ran against throwaway databases, which have been dropped. **Every plan in this phase must state that DATABASE_URL points at a freshly created disposable database (for example `createdb p3_verify`), never `pinalert_test`.** See "Validation Architecture".
2. **No new Go or npm packages are needed.** Package Legitimacy Audit: none.
3. **Formula constants below are design choices, not facts.** Three of them are surfaced as owner questions (Open Questions Q1, Q3, Q4) with the numbers attached. Everything else the planner can build as written.
4. **Existing tests will fail the moment D-01 lands.** Five e2e call sites vote from 10.58 km away and ten unit fixtures carry no report coordinates. Exact list in "Tests".
5. **The repo's JS contract tests constrain how D-02 and D-22 may be implemented** (single `getCurrentPosition(`, single `fetch(`, no `Pinalert.config` in `votes.js`, no `disabled` word inside `updateVoteBlock`, `activity.js` may hold exactly one `addEventListener`). Exact list in "Common Pitfalls" and "Front end".
6. Running `/gsd-ui-phase 3` before planning is advisable: `config.json` has `workflow.ui_phase: true`, the phase has a UI hint, and the hover/tap/focus tooltip is a new component with no UI-SPEC yet.

## Summary

Phase 3 is mostly pure computation plus one bounded read plus one shared UI block. The core is a set of pure functions in `internal/service` (great circle distance, currency, outcome classification, reliability, trust builder) fed by the rows the three read paths (`Nearby`, `CastVote`, `ActivityForAccount`) already load. The only schema change is one index. The only new query is a bounded "latest expired reports per account" read (about 1 ms at 20k rows with the index versus 20 ms without, measured; the first query form I tried took 1.5 s at 200k rows without the index). The wire change is one `trust` object shared by `FeedReportResponse` and `CastVoteResponse`. The front end is one new module `trust.js` (block builder, accessible tooltip, fade helper) mounted from `feed.js`, `map.js` and `activity.js`, plus a small hint added inside `votes.js`.

The decisions are unusually complete, so research effort went into (a) verifying the CONTEXT claims against live code (all verified, three imprecisions recorded in the Assumptions Log), (b) choosing formulas whose band and tag boundaries are provably consistent with the score (integer arithmetic for currency, invariants for reliability), (c) finding every existing test and contract that D-01, D-02, D-22 and the new block will collide with, and (d) measuring the reliability read so the index and the query shape are evidence based.

**Primary recommendation:** Build in this order: pure functions and docs first (no DB), then migration plus new query plus `ReportVoteContext` coordinates, then wire the three read paths and the distance rule, then the handlers and swagger regeneration, then `trust.js` and CSS, then the `votes.js` hint. Keep the single shared `Trust` shape fixed from the first slice so later slices only fill fields.

## Project Constraints (from CLAUDE.md and owner memory)

- Stack is fixed: Go 1.25+ (repo `go.mod` says 1.26.4, local toolchain 1.26.4), `go-chi/chi/v5`, `jackc/pgx/v5` + `sqlc` v1.31.1, `html/template` + vanilla JS, `mmcloughlin/geohash` v0.10.0. No PostGIS, no new dependencies, no build step for JS. [VERIFIED: go.mod, `sqlc version`, `swag --version`]
- Generated code is committed: `internal/store/sqlc/*` (from `make sqlc`) and `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` (from `make swag`). CI has neither `sqlc` nor `swag` binaries, so forgetting to regenerate is not caught by CI except through the substring asserts in `swagger_test.go`. [VERIFIED: `.github/workflows/ci.yml`, `sqlc.yaml` header comment]
- Migrations are run as an explicit step (`make migrate`), never on boot. [VERIFIED: `cmd/migrate/main.go`]
- Start any file changing work through a GSD command (this research file is that workflow's own output).
- Owner memory rules that bind every string and mockup: no em dashes or en dashes anywhere the user can read (page titles, labels, error messages, helper copy, docs prose); no fake or invented metrics; no emoji as icons; no pill shaped buttons; no hero text; restrained transitions only; **never suggest port 8080** for the local dev server (use `PORT=8090` in any run instructions). Structurally required hyphens (`re-confirming`, CSS class names, file names) are fine.
- Commit messages end with the attribution line given in the session reminder.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Distance rule (1 km) | API / Backend (`service.CastVote`) | Browser (advisory hint only) | Server is the only gatekeeper; coordinates are client asserted, so the browser check grants nothing (D-22) |
| Counting N and M | API / Backend (`BuildVoteTally`) | none | One independence rule (D-04); browser never counts |
| Still current band, score, fade stage | API / Backend (pure fn) | Browser combines fade stage with time stage | D-07 and R-02: browser takes worse of two, holds no thresholds |
| Reliability history read | Database (bounded query) | API (classification, weighting) | Independence is never aggregated in SQL; SQL only bounds the rows |
| Trust copy (labels, reasons) | API / Backend (Go strings) | Browser renders via `setText` | One place for wording, Go side dash test, identical on all three surfaces |
| Trust block DOM and tooltip | Browser (`trust.js`) | Server renders JSON into the Activity page | One builder for feed, popup and Activity (D-20) |
| Radius value for the hint | Frontend server (page config, data attribute) | Browser reads attribute | Single constant `service.MaxVoteDistanceKm` feeds both (D-22) |
| Index on `reports(session_id, expires_at)` | Database | none | Only schema change |

## Standard Stack

No new libraries. Everything below is already in `go.mod` or the repo. [VERIFIED: go.mod, repo grep]

| Component | Version | Purpose in this phase |
|-----------|---------|-----------------------|
| Go | 1.26.4 (local and go.mod) | Pure functions, `math`, `time`; integer arithmetic for currency |
| `github.com/mmcloughlin/geohash` | v0.10.0 | Cell width derivation test (D-03); `EncodeWithPrecision`, `BoundingBox(hash)` |
| `github.com/jackc/pgx/v5` + `sqlc` | v5.10.0, v1.31.1 | New query `ReporterHistoryReports`; `ReportVoteContext` gains coordinates |
| `github.com/pressly/goose/v3` | v3.28.0 | Migration `00005` |
| `github.com/swaggo/swag` | v1.16.4 in go.mod and local CLI | Regenerates docs; output verified byte stable against the committed files |
| `html/template` | stdlib | `data-vote-radius-km` attribute, `data-trust` JSON attribute on Activity rows |
| Vanilla JS, Leaflet 1.9.4 | existing | `trust.js`, tooltip, hint |

**Alternatives considered:** Wilson lower bound for reliability (the CONTEXT lean). Rejected as the primary formula because an additive or lower-bound penalty makes a spotless but aging record drift toward "Mixed record" or "Unreliable reporter" (see "Score formulas", invariants I1 and I2). It is kept as a documented comparison in `docs/TRUST-MODEL.md`.

**Installation:** none. **Version verification:** `sqlc version` = v1.31.1 and `swag --version` = v1.16.4 locally; regenerating with them reproduced the committed `internal/store/sqlc/*.go` and `docs/swagger.json`/`swagger.yaml` byte for byte (only the Go package name in `docs.go` differed because I wrote to a scratch directory). [VERIFIED: scratch regeneration this session]

## Package Legitimacy Audit

No external packages are installed by this phase. Audit not applicable.

**Packages removed due to SLOP verdict:** none
**Packages flagged SUS:** none

## Architecture Patterns

### System Architecture Diagram

```
                        POST /api/reports/{id}/{confirm|dispute|resolve|reopen}
 browser votes.js  ---------------------------------------------------------------+
  (silent refresh,                                                                 v
   retry once)                                             handlers.CastVote (decode, map errors)
                                                                     |
                                                                     v
                                                     service.VotingService.CastVote
   1 validate input ----> 2 ReportVoteContext (now with lat/lon) ----> 3 self vote block (content only)
   4 reporter exemption known ----> 5 expired? (409) ----> 6 distance check (403 "location")
   7 InsertVote ----> 8 CurrentVotesForReports(report id + history ids) ----> 9 BuildTrust + Resolve
                                                                     |
        200 {visibility, reason, trust}  <---------------------------+

 GET /api/reports?lat&lon&radius_km  (poll every 30 s)
   handlers.NearbyReports -> service.ReportService.Nearby
     NearbyReports (unchanged, pinned) -> page ids
     ReporterAccountsForReports(page ids) -> distinct reporter account ids (capped)
     ReporterHistoryReports(account ids, now, window_start, cap)   <- new, bounded, uses new index
     CurrentVotesForReports(page ids UNION history ids)            <- still ONE call
     per report: BuildVoteTally + Resolve (unchanged authority) + BuildTrust
     per account: classify history outcomes -> reliability (one per distinct account)
   -> 200 {reports:[{..., trust:{...}}]}

 GET /profile (Activity)  service.AuthService.ActivityForAccount
   ReportsByAccount -> ids ; ReporterHistoryReports([account]) ; CurrentVotesForReports(ids UNION history ids)
   -> profileReport.TrustJSON -> <li data-trust="{...}">  -> trust.js builds the same block

 Browser render path (all three surfaces):  trust object -> PinalertTrust.createTrustBlock/updateTrustBlock
   -> setText only; body-level tooltip; PinalertTrust.fadeStage(report) = worse(Pinalert.ageStage, server fade_stage)
```

### Recommended Project Structure (new and changed files)

```
internal/service/
  geo.go               NEW  MaxVoteDistanceKm, haversineKm, withinVoteRadius, ErrVoterTooFar
  currency.go          NEW  still current band, score, fade stage (pure)
  reliability.go       NEW  outcome classification, weighting, tag (pure)
  trustview.go         NEW  Trust, BuildTrust, ReportView/CastVoteResult/ActivityReport carry Trust
  trustcopy.go         NEW  every server authored label and reason sentence (one file, one dash test)
  trust.go             MOD  extract shared row filter; CastVote gains distance step + Trust
  report.go, auth.go   MOD  Nearby and ActivityForAccount attach Trust; interfaces gain the new query
internal/store/
  migrations/00005_add_reports_session_expires_index.sql   NEW
  queries/reliability.sql                                   NEW  (keeps votes.sql and reports.sql guards untouched)
  queries/votes.sql                                         MOD  ReportVoteContext selects r.latitude, r.longitude
  sqlc/*.go                                                 REGEN (committed)
internal/api/handlers/
  trust.go             NEW  TrustResponse types + trustToResponse (shared by feed, vote, profile JSON)
  reports.go, votes.go, auth.go, page.go   MOD
web/static/js/trust.js         NEW   web/static/css/trust.css  MOD (append)
web/static/js/votes.js, feed.js, map.js, activity.js   MOD
web/templates/index.html.tmpl, profile.html.tmpl       MOD
docs/TRUST-MODEL.md            NEW  (derivation table, formulas, limits) + regenerated swagger files
```

### Pattern 1: one row filter, two consumers (no second cell derivation)
**What:** Extract the reporter exclusion and kind and value routing from `BuildVoteTally` into one unexported helper that returns the four buckets with `(cell, createdAt)` per vote. `BuildVoteTally` becomes counts of those buckets (its output and signature unchanged, so all three callers and every existing test still pass). Currency reads the same buckets for per cell latest confirm time.
**Why:** Currency needs timestamps that `BuildVoteTally` currently drops, and a second copy of the reporter exclusion filter is exactly the drift D-04 forbids.
**Guard:** a property test asserting `BuildTrust(...).ConfirmedPlaces == BuildVoteTally(...).ConfirmCells` over random row sets.

### Pattern 2: pure functions with an injected `now`
`Resolve(meta, tally, now)` already takes `now` (unused). All new functions take `now time.Time` and never call `time.Now()`. The three services call `time.Now().UTC()` once per request. Add functional options for a clock on `ReportService` and `VotingService` (names must differ from the existing `WithClock` which returns `AuthOption`, for example `WithReportClock`, `WithVotingClock`); constructors stay call compatible because the option parameter is variadic (`NewReportService(nil)` in `swagger_test.go` keeps compiling). `ActivityForAccount` currently calls `time.Now().UTC()` directly (auth.go line 361) although `AuthService.now` exists; switch it to `s.now()`.

### Pattern 3: server authored strings, browser renders text only
All labels and reasons come from `trustcopy.go`. The browser never builds a sentence from integers, so no threshold or grammar rule lives in JS. Every string goes through `Pinalert.setText`.

### Anti-Patterns to Avoid
- **Second visibility authority:** never re-derive Live, Hidden or Provisional. Classification calls `Resolve` with neutral metadata (D-11).
- **Storing any score:** recompute on every read (Phase 2 D-08). `PROJECT-NOTES.md` line 237 and research `SUMMARY.md` line 53 (stored score columns) must not be followed.
- **Modifying `NearbyReports`:** do not touch it. (Note: the "byte for byte" pin is weaker than CONTEXT says, see Assumptions A1, but the rule stands.)
- **Aggregating independence in SQL.** SQL only bounds rows.
- **Using `disabled` for the too far hint** (see Pitfalls).

## Change map (ask 1)

### By decision

| ID | Create | Modify | Notes |
|----|--------|--------|-------|
| D-01 | `internal/service/geo.go` (`MaxVoteDistanceKm`, `haversineKm`, `withinVoteRadius`, `ErrVoterTooFar`); `internal/service/geo_test.go` | `internal/store/queries/votes.sql` (`ReportVoteContext` adds `r.latitude, r.longitude`) then `make sqlc`; `internal/service/trust.go` `CastVote`; `internal/api/handlers/votes.go` (map sentinel to 403, field `location`, swagger text) | Order and exemption rule in "Distance rule" |
| D-02 | none | `web/static/js/votes.js` (`getVoterLocation`, `castVote`); `web/votes_contract_test.go` (two anchors) | Mechanism in "Front end" |
| D-03 | `docs/TRUST-MODEL.md`; `internal/service/geohash_precision_test.go` (in package `service` so it can read the unexported constant) | `internal/service/trust.go` comment on `voterGeohashPrecision` (closes A3); orchestrator closes the STATE.md blocker | Derivation table numbers in "Distance rule" |
| D-04 | `Trust.ConfirmedPlaces` set from `tally.ConfirmCells` | `internal/service/trust.go` (row filter extraction) | No new constant |
| D-05, D-06, R-01 | `internal/service/currency.go` | none | Formula in "Score formulas" |
| D-07, R-02, R-03 | `fade_stage` in `Trust.Currency`; `PinalertTrust.fadeStage` | `feed.js` (2 call sites), `map.js` (3 call sites) | Sites: feed.js lines 205 and 355; map.js lines 168, 242, 284 |
| D-08 | `Currency.Score`, `Reporter.Score` (`*int`, explicit null) | `docs/TRUST-MODEL.md` records how "score" is satisfied | |
| D-09, R-07 | popup only option in `trust.js` | `map.js` `buildPopupContent` passes `{showScores: true}` | Feed and Activity omit the option |
| D-10 to D-15, R-05, R-06 | `internal/service/reliability.go`; `internal/store/queries/reliability.sql`; migration `00005`; store test for index and bounds | `Querier`, `VotingQuerier`, `AuthQuerier` interfaces (each gains `ReporterHistoryReports`); three fakes | Query and index in "Reliability history read" |
| D-16, D-17, D-18, D-19, R-03, R-04 | `internal/service/trustcopy.go`; `web/static/js/trust.js`; CSS block in `trust.css` | `feed.js createRow/updateRow`, `map.js buildPopupContent` | Copy table under "Wire and docs" |
| D-20, R-08 | none | `internal/service/auth.go` (`ActivityReport.Trust`), `internal/api/handlers/auth.go` (`profileReport.TrustJSON`), `web/templates/profile.html.tmpl` (`data-trust`, script tag), `web/static/js/activity.js` | Reopen updates the block from `CastVoteResponse.trust` |
| D-21 | `internal/api/handlers/trust.go` | `FeedReportResponse.Trust`, `CastVoteResponse.Trust`, `reportViewToResponse`, `CastVote` handler; `docs/*` regenerated; `swagger_test.go` extended | |
| D-22 | none | `internal/api/handlers/page.go` (`PageConfig.VoteRadiusKm`, `pageViewModel.VoteRadiusKm`), `cmd/server/main.go`, `web/templates/index.html.tmpl` (`data-vote-radius-km`), `web/static/js/votes.js` (`applyDistanceHint`), `web/static/css/trust.css` | Cut first if the plan must shrink |

### Dependency order

1. **W1 pure, no DB:** row filter extraction, `geo.go`, `currency.go`, `reliability.go`, `trustcopy.go`, `BuildTrust`, geohash precision test, docs draft. Table tests with injected `now`.
2. **W2 store:** migration 00005, `reliability.sql`, `ReportVoteContext` coordinates, `make sqlc`, EXPLAIN and bounds tests. Needs a disposable DB.
3. **W3 service wiring:** interfaces plus the three fakes, `CastVote` distance step and `Trust`, `Nearby` and `ActivityForAccount` attach `Trust`, clock options, fix existing fixtures. Needs W1 and W2.
4. **W4 wire:** `TrustResponse`, both response types, error mapping, `PageConfig`, profile `TrustJSON`, `make swag`, swagger and e2e tests. Needs W3.
5. **W5 front end:** `trust.js` and CSS, templates and script order, `feed.js`/`map.js`/`activity.js` integration, `votes.js` hint. Needs W4 for real payloads (can start against a fixture object after W1 fixes the shape).
6. **W6 docs and closure:** `docs/TRUST-MODEL.md`, README pointer, human UAT script.

**Vertical slice option (phase mode is mvp):** S1 "nearby is real" (D-01, D-02 silent refresh, fixture fixes, D-03 test and doc table); S2 "count line" (full `Trust` shape frozen with counts filled, wire, `trust.js` block with count line on feed, popup, Activity; TRUST-05 demonstrable); S3 "still current" (currency, fade, popup number); S4 "source reliable" (migration, query, reliability, tag); S5 "reasons and hint" (tooltip reasons, D-22 hint, docs). Freeze the whole `Trust` struct in S2 so swagger regenerates without renames later. If cut is needed: drop D-22 first (CONTEXT planner note).

## Reliability history read (ask 2)

### Recommended query (`internal/store/queries/reliability.sql`, new file)

Placed in its own file so `TestVotesQuerySourceHasNoUpsertOrLock` (reads all of `votes.sql`) and `TestNearbyReportsQuerySourceHasExpectedShape` (reads all of `reports.sql`) are not disturbed. sqlc generation of this exact text succeeded in a scratch copy and produced `ReporterHistoryReportsParams{Now, WindowStart time.Time; RowCap int32; AccountIds []int64}` and `ReporterHistoryReportsRow{AccountID, ReportID int64; Severity, Category string; ExpiresAt time.Time}`. [VERIFIED: scratch `sqlc generate`]

```sql
-- name: ReporterHistoryReports :many
SELECT t.account_id::bigint AS account_id, t.report_id, t.severity, t.category, t.expires_at
FROM (
    SELECT x.account_id, rr.id AS report_id, rr.severity, rr.category, rr.expires_at,
           row_number() OVER (PARTITION BY x.account_id ORDER BY rr.expires_at DESC, rr.id DESC) AS rn
    FROM sessions x
    CROSS JOIN LATERAL (
        SELECT r.id, r.severity, r.category, r.expires_at
        FROM reports r
        WHERE r.session_id = x.session_id
          AND r.expires_at <= sqlc.arg(now)::timestamptz
          AND r.expires_at > sqlc.arg(window_start)::timestamptz
        ORDER BY r.expires_at DESC, r.id DESC
        LIMIT sqlc.arg(row_cap)::int
    ) rr
    WHERE x.account_id = ANY(sqlc.arg(account_ids)::bigint[])
) t
WHERE t.rn <= sqlc.arg(row_cap)::int
ORDER BY t.account_id, t.expires_at DESC, t.report_id DESC;
```

Why this shape (measured, local Postgres 16.14, scratch data):

| Shape and data | Without index | With `(session_id, expires_at DESC, id DESC)` |
|---|---|---|
| Lateral over `unnest(accounts)` joining sessions inside, 200k reports, 2,000 accounts, 50 accounts requested | 1,508 ms, 964k buffer hits, plan drives from `idx_reports_expires_at` | 9 ms, 5.6k buffer hits |
| Recommended per session top N plus window function, 20k reports, 10 accounts | 20 ms, plan uses `Index Scan Backward using idx_reports_expires_at` | 1 ms, `Index Scan using idx_reports_session_expires on reports r` |

The first shape sometimes chose the wrong index at 20k rows even with the new index present. The recommended per session lateral forces the equality on `session_id` and is robust at both sizes. Params come from Go constants (`now` captured once per request and reused for decay, `window_start = now - 90 days`, `row_cap = 30`), so SQL has no magic numbers and the injected clock reaches the query.

**Semantics to record in docs:** the cap is applied to the latest 30 *expired reports in the window*, not to 30 *judged* reports, because SQL cannot know which are neutral without vote data. The only neutral expired reports are critical or rescue needed reports that expired unconfirmed (D-12), so the two readings differ only for a reporter with many such reports. This is a small deviation from D-13's literal wording and is listed as Open Question Q4.

### Index and migration

`internal/store/migrations/00005_add_reports_session_expires_index.sql` (next number is 00005, four exist). Conventions observed in 00001 to 00004: header line `-- Source: github.com/pressly/goose README [CITED]`, then `-- +goose Up`, statements, `-- +goose Down`; files are picked up by `//go:embed migrations/*.sql` automatically, applied by `cmd/migrate` (`make migrate`) and by `testutil.NewTestDB` (`goose.Up`). No `NO TRANSACTION` is needed at this scale (plain `CREATE INDEX` inside goose's transaction); note in a comment that a large production table would want `CONCURRENTLY`.

```sql
-- +goose Up
CREATE INDEX idx_reports_session_expires ON reports (session_id, expires_at DESC, id DESC);
-- +goose Down
DROP INDEX idx_reports_session_expires;
```

The `id DESC` tail matches the query's tie break so the per session scan needs no extra sort. `Truncate` in `testutil` needs no change (no new table). **The owner's dev database needs `make migrate` before the new code is exercised there** (queries work without the index, only slowly).

### EXPLAIN test (how the existing one works, and the trap)

`TestNearbyReportsUsesIndex` (`internal/store/reports_test.go` line 66): `testutil.NewTestDB`, `testutil.SeedReports(50_000)`, `ANALYZE reports`, then `pool.Query(ctx, "EXPLAIN (ANALYZE, FORMAT TEXT) "+nearbyReportsSQLForExplain, params...)` where the SQL is a hand copied const with positional params (sqlc emits an unexported const, unreachable from package `store_test`), and it asserts the plan text contains `Index Scan`, `Index Only Scan` or `Bitmap Index Scan` and does not contain `Seq Scan on reports`.

**Trap:** a generic "some index scan, no Seq Scan" assertion would pass for the history query even with no new index, because without it the planner uses `idx_reports_expires_at`. The equivalent test must **assert the index name** `idx_reports_session_expires` in the plan text (and still assert no `Seq Scan on reports`). `SeedReports` cannot be reused: it inserts session ids (`seed-session-low` and similar) that have no `sessions` row and only three distinct values. Write a dedicated seeder: for example 200 accounts, 3 sessions each bound via `sessions.account_id`, about 80 reports per session with `expires_at` spread over the last 90 days (use `pool.CopyFrom` as `SeedReports` does), then `ANALYZE reports` and `ANALYZE sessions`. Follow the repo convention of a hand copied `reporterHistorySQLForExplain` const plus a source shape test that reads `queries/reliability.sql`; make the drift guard stronger than the existing one by normalising whitespace and replacing `sqlc.arg(name)` with positional parameters before comparing to the const (the existing guard only checks two substrings, see A1).

Add a bounds test on real rows: for N accounts the result has at most `N * 30` rows, only expired rows inside the window, per account descending, accounts with several sessions aggregate together, an accountless session contributes nothing, `expires_at == now` is included, `expires_at == window_start` is excluded.

### `sqlc generate` output handling

`make sqlc` runs `sqlc generate` (config `sqlc.yaml`, out `internal/store/sqlc`, package `sqlcgen`, `sql_package: pgx/v5`). Generated files are committed (CI needs no sqlc). Regeneration with the installed v1.31.1 reproduced every committed file byte for byte. Commit `reliability.sql.go` (new) and the changed `votes.sql.go`.

### Batching and cost

Feed path: `ReporterAccountsForReports(page ids)` gives distinct non nil reporter account ids; one `ReporterHistoryReports` call; then **one** `CurrentVotesForReports` over `page ids UNION history report ids` (deduplicate ids) so the existing invariant "one batched vote read per path" and `TestNearbyBatchesVoteReadsOnce` stay valid (they use nil reporters, so no history runs there; add a new test with reporters asserting exactly one vote read). Reorder `Nearby` slightly: reporters first, then history, then votes. `CastVote`: the reporter's history for the single report is one extra query; union the report id into the same vote read. `ActivityForAccount`: use the same query with a one element account list; its ids are a subset of the account's own report ids.

Worst case measured: 300 distinct reporters times 30 history reports (9,000 report ids, 71,849 current vote rows) took 68 ms server side for the vote read, per poll. See Security and Risks for the account id cap.

## Distance rule (ask 3)

### Helper

There is no Go Haversine today; the only ones are SQL inside the pinned `NearbyReports` (law of cosines form, radius 6371) and a copy in `internal/store/reports_test.go`. Add `haversineKm` in `geo.go` using R = 6371 so it agrees with the feed's `distance_km` and with the browser hint. Use the haversine form (better than acos at small distances) and clamp the intermediate to [0, 1]. **Fail closed on non finite input:** write the check as `if !(d <= MaxVoteDistanceKm)` rather than `d > MaxVoteDistanceKm`, so NaN or Inf is refused. (JSON bodies cannot carry NaN, but the existing range check `< -90 || > 90` also lets NaN through in any non JSON caller, and `strconv.ParseFloat("NaN")` is accepted by the feed's query parser.)

```go
// Source: standard haversine, R matches NearbyReports (6371).
const MaxVoteDistanceKm = 1.0
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
    const earthRadiusKm = 6371.0
    p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
    dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
    h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
    return 2 * earthRadiusKm * math.Asin(math.Sqrt(math.Min(1, math.Max(0, h))))
}
func withinVoteRadius(rLat, rLon, vLat, vLon float64) bool {
    return haversineKm(rLat, rLon, vLat, vLon) <= MaxVoteDistanceKm // NaN compares false, so NaN is refused
}
```

### BoundingBox as a prefilter

Not needed: the check is one pair, so a single Haversine is a handful of trig calls. Use `BoundingBox` only as a test oracle: for random voters with `haversineKm <= R`, the voter must lie inside `BoundingBox(reportLat, reportLon, R)`. This holds because `BoundingBox` uses 111.045 km per degree (smaller than 111.195 at R = 6371), so its box is slightly larger than the true circle. It cross checks both helpers.

### `ReportVoteContext`

Add `r.latitude, r.longitude` to the select list (append after `s.account_id AS reporter_account_id`, or before; sqlc field order follows the select list, tests use named fields). Generated `ReportVoteContextRow` gains `Latitude, Longitude float64`. The `LEFT JOIN sessions` stays. Update the query's long comment (it says "Four points"; it now feeds five things).

### Step order inside `CastVote` (name it and test it)

1. validate kind, value, coordinate ranges (`ValidationError`, unchanged)
2. `ReportVoteContext`
3. self vote block: content vote by the reporter gives `ErrCannotVoteOwnReport` (403 "account")
4. compute `isReporterResolutionVote := in.Kind == VoteKindResolution && rc.ReporterAccountID != nil && *rc.ReporterAccountID == in.AccountID`
5. expired gives `ErrReportExpired` (409)
6. distance check **unless** `isReporterResolutionVote`: refuse with `ErrVoterTooFar`
7. compute cell, `InsertVote`, read votes, build tally, `Resolve`, `BuildTrust`

Consequences to test: an expired report and too far gives 409 (expiry wins); the reporter's own content vote from far away gives 403 own report (self vote block first); the reporter's resolve or reopen from anywhere succeeds; a non reporter's resolve or reopen from more than 1 km is refused; a Retracted but unexpired report still enforces distance on non reporters; an accountless report (`ReporterAccountID == nil`) exempts nobody. The exemption reads only the server side reporter identity, never anything from the request.

### Error mapping

Use a **dedicated sentinel**, not `ValidationError`. A `ValidationError{Field:"latitude"}` (as the GPS denied path uses) would be indistinguishable to the client from a malformed coordinate, and D-02's retry once must fire only on too far. Recommended: `service.ErrVoterTooFar` mapped in `handlers.CastVote` (add a branch in the `errors.Is` group above the `errors.As` branch) to **403** with `writeFieldError(w, 403, "location", message)`. The client keys on `error.field === "location"`. Alternative with zero handler change: `ValidationError{Field:"location"}` giving 400; rejected because 400 already means malformed here. Copy (no dashes; the content sentence equals D-22's hover sentence, the radius built from the constant): content votes `Too far to vote. You need to be within 1 km of this report.`; resolve and reopen `Too far to do this. You need to be within 1 km of this report.` The response must not include the computed distance. Update the swagger `@Description` (403 now covers two causes); the existing single `@Failure 403` line, shared by the four routes, stays.

### Test cases at India latitudes (numbers computed this session with R = 6371)

| Case | Result |
|---|---|
| Chennai (13.0827, 80.2707) to Bengaluru (12.9716, 77.5946) | 290.17 km (reject) |
| Report (12.9716, 77.5946), voter 0.0089 degrees north | 0.9896 km (accept) |
| same, voter 0.0090 degrees north | 1.0008 km (reject) |
| same, voter (12.9746, 77.5946) | 0.3336 km (accept) |
| same, voter (12.9686, 77.5946) | 0.3336 km (accept) |
| same, voter (13.0500, 77.6500) (the old test fixture) | 10.58 km (reject) |
| 0.01 degrees of longitude at 13N | 1083.5 m (reject) |
| 0.01 degrees of longitude at 28.6N | 976.3 m (accept) |
| 0.01 degrees of longitude at 34N | 921.8 m (accept) |
| 1 km east in degrees of longitude at 8, 13, 28.6, 34, 37 N | 0.009082, 0.009230, 0.010243, 0.010848, 0.011261 (all round trip to 1.0000 km) |
| identical points | 0 |
| NaN, +Inf in any argument | refused |

Also test symmetry, and an optional DB parity test against `NearbyReports`' `distance_km` within 1 metre (the SQL law of cosines form loses roughly 0.1 m at short range).

### Derivation table for D-03 (numbers from the geohash library, Chennai unless stated)

| Precision | Bits (lon, lat) | Cell height | Cell width (Chennai 13.08N) |
|---|---|---|---|
| 5 | 13, 12 | 4,886 m | 4,760 m |
| 6 | 15, 15 | 611 m | 1,190 m |
| 7 (chosen) | 18, 17 | 152.7 m | 148.7 m |
| 8 | 20, 20 | 19.1 m | 37.2 m |

Precision 7 width by city: Chennai 148.7 m, Bengaluru 148.8 m, Kolkata 141.0 m, Delhi 134.1 m, Srinagar 126.5 m. Witness simulation (500 trials, seed 42, uniform points in a 300 m radius circle, precision 7): 5 witnesses give 2 to 5 distinct cells (mean 4.4); 50 witnesses 12 to 21 (mean about 16.5); 200 witnesses 17 to 22 (mean about 20). So even a very large crowd in a 300 m circle can only ever show about 22 distinct places, which is the honest statement of "N is a lower bound on people". Assert generous bounds in the test (for example at most 24 distinct cells at every one of the five latitudes, and a fixed seed) rather than exact means.

## Score formulas (ask 4)

All constants live in one place per file, are named, and are documented in `docs/TRUST-MODEL.md`. Nothing here invents an input: every quantity is a vote row, a `created_at`, an `expires_at`, or a named constant.

### Still current score (0 to 100, null per R-01)

Definitions for one report at time `now`:
- `L = expires_at - created_at`, `W = L / 4` (2 h for 8 h reports, 6 h for 24 h reports; a low severity rescue needed report is 8 h because `ExpiryDuration` is severity only, so `W` is 2 h there, document it).
- Confirming places: from the shared row filter (reporter excluded, content, confirm, one current row per account), group by cell; a place's time is the latest `created_at` among its rows. `N = ConfirmCells`.
- `D_W` = distinct dispute cells among current dispute rows with `created_at >= now - W` (a standing independent dispute inside the window; a dispute never refreshes).
- `k = IndependentAgreementThreshold + D_W`. Sort places by time descending; `A = now - T_k` where `T_k` is the k-th most recent place time; clamp `A` to `[0, 4W]` (clock skew between the database `now()` default and Go's clock can make a vote look slightly in the future).

Rules:
1. If `N < 2`: band `not_yet_corroborated`, score `null` (never 0).
2. Else if fewer than `k` places exist (contested): score `0`, band `needs_reconfirming`.
3. Else integer score `100 - ceil(25 * A / W)`, computed in integer nanoseconds (no float rounding), floored at 0.
4. Bands cut from the integer: `>= 75` `up_to_date`, `50 to 74` `getting_old`, `<= 49` `needs_reconfirming`.

Why it is consistent by construction: `A <= W` is exactly `score >= 75`, `A <= 2W` is exactly `score >= 50`, so the words match the integer with no side guard, and with `D_W = 0` "Up to date" is exactly "at least 2 distinct places inside the window" (R-01). Each dispute place inside the window raises the bar by one place, which is "cancels one confirming place inside the window, net floored at 0" expressed on the k-th most recent place. The score loses one point per started 1/25 of a window (4.8 minutes on an 8 hour report). A single account re-tapping Confirm refreshes only its own place's time, so it cannot move the second most recent place unless a genuinely different account is also recent (D-05).

```go
// lost = ceil(25*age/window), age clamped to [0, 4*window] first so 25*age cannot overflow.
func currencyScore(age, window time.Duration) int {
    if window <= 0 { return 0 }
    if age < 0 { age = 0 }
    if age > 4*window { age = 4 * window }
    lost := (25*int64(age) + int64(window) - 1) / int64(window)
    return 100 - int(lost)
}
```

Fade stage (R-02), non critical rows only, and only when the row is not Hidden, Retracted or expired: `up_to_date` and `not_yet_corroborated` give `fresh`, `getting_old` gives `aging`, `needs_reconfirming` gives `stale`. Critical and rescue needed rows: `null`. Band `not_applicable` (Hidden, Retracted or `now >= expires_at`, the last only reachable on the Activity page): score `null`, no label (R-03, R-08).

Boundary table (identical for 8 h and 24 h reports; injected `now`; `A` is age of the k-th place):

| A | score | band |
|---|---|---|
| 0 | 100 | up_to_date |
| 1 ns | 99 | up_to_date |
| W/25 | 99 | up_to_date |
| W/25 + 1 ns | 98 | up_to_date |
| W - 1 ns | 75 | up_to_date |
| W | 75 | up_to_date |
| W + 1 ns | 74 | getting_old |
| 1.5 W | 62 | getting_old |
| 2 W | 50 | getting_old |
| 2 W + 1 ns | 49 | needs_reconfirming |
| 3 W | 25 | needs_reconfirming |
| 4 W (= L) | 0 | needs_reconfirming |
| negative (skew, -5 s) | 100 | up_to_date |

Worked example (8 h report created 10:00, W = 2 h; three distinct places confirm at 10:30, 11:00 and 13:00):

| now | N | score | band |
|---|---|---|---|
| 10:45 | 1 | null | not_yet_corroborated |
| 11:00 | 2 | 93 | up_to_date |
| 11:30 | 2 | 87 | up_to_date |
| 13:00 | 3 | 75 | up_to_date (T_2 is 11:00, A = 2 h) |
| 13:30 | 3 | 68 | getting_old |
| 15:00 | 3 | 50 | getting_old |
| 16:00 | 3 | 37 | needs_reconfirming |
| 17:59 | 3 | 12 | needs_reconfirming |

Contested: two confirms (10:30, 11:00) plus one dispute at 11:30 give `k = 3`, only 2 places exist, so score 0 and `needs_reconfirming` from 11:30 until the dispute leaves the window; a third confirm at 13:00 with the same dispute gives `k = 3`, `T_3 = 10:30`, `A = 2.5 h`, score 68, `getting_old`. The table test must include these and the property "advancing `now` never raises the score; adding a place never lowers it; adding a dispute never raises it".

### Reliability score (0 to 100, null for New reporter)

Inputs per account: the judged outcomes inside the window and cap (R-05), each with `expires_at`.

- Outcome credit: corroborated `1.0`, contradicted `0.0`, expired unconfirmed `c` (recommended `c = 0.4`; owner question Q1). Neutral outcomes (in flight, critical or rescue needed unconfirmed) are not in the list.
- Weight `w_i = 2^(-(now - expires_at_i) / 30 days)` (half life 30 days, age from `expires_at`, D-13, R-05).
- `n` = number of judged outcomes (unweighted, after window and cap). If `n < 5`: tag `new`, score `null`.
- `p = sum(w_i * credit_i) / sum(w_i)`, a recency weighted average in `[0, 1]`.
- `score = round(100 * (n * p + 1) / (n + 2))`.

Plain language: "estimated confirmed reports out of finished reports, with one imaginary confirmed and one imaginary unconfirmed report added so a short record is never shown as 0 or 100." Tags cut from the rounded integer: `>= 75` Reliable reporter, `35 to 74` Mixed record, `<= 34` Unreliable reporter.

Why this formula rather than the CONTEXT lean (Wilson lower bound on decayed counts): with an additive prior or a lower bound that shrinks as weights decay, a spotless record drifts down into "Mixed record" (5 corroborated reports 45 days old scored 73 under the additive form I first tried) and an honest reporter with only unconfirmed reports drops to "Unreliable reporter" (5 unconfirmed scored 31), which a first time reader takes as an accusation. Here decay only changes the *relative* weight of outcomes (a uniform ageing changes nothing), the 90 day window then removes old outcomes and a record that falls under 5 judged becomes "New reporter". Invariants guaranteed and to be property tested:
- **I1:** a history with no contradicted and no unconfirmed outcome (5 to 30 judged, any ages) always scores at least `100 * 6/7 = 85.7` and is Reliable, never Mixed or Unreliable.
- **I2:** with `c` in `[0.35, 0.74)` a history of only unconfirmed outcomes scores 40.6 to 42.9 for `n` from 5 to 30 (with `c = 0.4`), so it is Mixed, and only contradicted outcomes can push a reporter to Unreliable.
- **I3:** replacing an outcome by a better class never lowers the score; order of outcomes never matters.
- **I4:** the score always lies in `[100/(n+2), 100(n+1)/(n+2)]`, so 0 and 100 are never reached.

Numeric table (all outcomes fresh unless noted, `c = 0.4`):

| History (n) | p | score | tag |
|---|---|---|---|
| 5 corroborated, fresh, 45 days old or 89 days old | 1.00 | 86 | Reliable |
| 30 corroborated | 1.00 | 97 | Reliable |
| 4 corroborated + 1 unconfirmed (5) | 0.88 | 77 | Reliable |
| 4 corroborated + 1 contradicted (5) | 0.80 | 71 | Mixed |
| 3 corroborated + 2 unconfirmed (5) | 0.76 | 69 | Mixed |
| 3 corroborated + 2 contradicted (5) | 0.60 | 57 | Mixed |
| 5 corroborated + 5 unconfirmed (10) | 0.70 | 67 | Mixed |
| 2 corroborated + 3 contradicted (5) | 0.40 | 43 | Mixed |
| 5 unconfirmed only (5) | 0.40 | 43 | Mixed |
| 30 unconfirmed only (30) | 0.40 | 41 | Mixed |
| 1 contradicted + 4 unconfirmed (5) | 0.32 | 37 | Mixed |
| 2 contradicted + 3 unconfirmed (5) | 0.24 | 31 | Unreliable |
| 1 corroborated + 4 contradicted (5) | 0.20 | 29 | Unreliable |
| 5 contradicted (5) | 0.00 | 14 | Unreliable |
| 8 corroborated + 2 contradicted (10), exact boundary | 0.80 | 75.00 | Reliable |
| 6 corroborated + 12 contradicted (18), exact boundary | 0.333 | 35.00 | Mixed |
| mixed ages: corroborated at 5, 20, 50 days, contradicted at 10 days, unconfirmed at 40 days (5) | 0.6591 | 61 | Mixed |
| recency: 5 corroborated at 80 days + 2 contradicted at 1 day (7) | 0.287 | 33 | Unreliable |
| recency: 5 contradicted at 80 days + 2 corroborated at 1 day (7) | 0.713 | 67 | Mixed |

Boundary cases the table test must cover with an injected `now`: `n = 4` versus `5` judged (New reporter versus rated, with neutral outcomes not counting), age exactly 90 days (window is `expires_at > now - 90 d`, so excluded) versus 89 d 23 h 59 m, `expires_at == now` (judged, weight 1), `expires_at` in the future (not returned by SQL), the 30 report cap (31st oldest dropped), exact score 75 and 35 (integer exact examples above, computed as `100 * (n*p+1) / (n+2)` with fresh weights `2^0 = 1` exactly), and rounding at `x.5`.

### Outcome classification (D-11, D-12, R-05, R-06)

```go
// neutral metadata and a tally with the resolution fields zeroed, then Resolve decides.
neutral := ReportMeta{Severity: SeverityLow, Category: CategoryOther}
t := tally; t.ResolveCells, t.ReopenCells, t.ReporterResolved, t.ReporterReopened = 0, 0, false, false
vis, _ := Resolve(neutral, t, now)      // Live -> corroborated, Hidden -> contradicted, Provisional -> unconfirmed
// D-12: if meta.criticalBypass() and outcome is unconfirmed -> neutral (not judged)
```
Retracted cannot occur after zeroing, so a self resolved report is judged by content votes only. Never copy `Resolve`'s thresholds. A property test compares the classifier to `Resolve(neutral, zeroed tally)` over random tallies. Reporter identity for each history report is the account being scored, so `BuildVoteTally(rows, reportID, &accountID)` excludes their own content votes.

### Options the owner may want to change (kept as data in one table)
`c` (unconfirmed credit), tag cutoffs 75 and 35, band cutoffs 75 and 50, window fraction (quarter). See Open Questions.

## Wire and docs (ask 5)

### Trust object (proposal, names avoid the word `cell`)

```go
type TrustResponse struct {
    ConfirmedPlaces   int    `json:"confirmed_places"`   // N, exact integer (label caps at 99+)
    DisputedPlaces    int    `json:"disputed_places"`    // M
    CountLabel        string `json:"count_label"`        // "" at zero
    CountReason       string `json:"count_reason"`
    UnconfirmedReason string `json:"unconfirmed_reason"` // "" unless the row shows the Unconfirmed chip
    Currency          CurrencyResponse `json:"currency"`
    Reporter          ReporterResponse `json:"reporter"`
}
type CurrencyResponse struct {
    Band         string  `json:"band" enums:"up_to_date,getting_old,needs_reconfirming,not_yet_corroborated,not_applicable"`
    Score        *int    `json:"score"`       // explicit null, no omitempty (your_vote precedent)
    Label        string  `json:"label"`       // "" when no word is shown (Provisional with chip, Hidden, Retracted, expired)
    Reason       string  `json:"reason"`
    RecentPlaces int     `json:"recent_places"`
    WindowMinutes int    `json:"window_minutes"`
    FadeStage    *string `json:"fade_stage" enums:"fresh,aging,stale"` // null for critical, rescue needed, Hidden, not_applicable
}
type ReporterResponse struct {
    Tag                 string `json:"tag" enums:"reliable,mixed,unreliable,new,none"`
    Score               *int   `json:"score"`
    Label               string `json:"label"`  // "" for none
    Reason              string `json:"reason"`
    JudgedReports       int    `json:"judged_reports"`
    CorroboratedReports int    `json:"corroborated_reports"`
}
```
The server decides which words show (`Label` empty means show nothing), so the browser holds no rule. `none` covers accountless reports and any reporter past the per read account cap. Declared once in `internal/api/handlers/trust.go`, built by `trustToResponse(service.Trust)`, used by `reportViewToResponse`, the `CastVote` handler and the profile JSON. `FeedReportResponse` gets a field `Trust` of type `TrustResponse` with JSON key `trust` (always present, no omitempty); `CastVoteResponse` gets the same field. Update the `CastVoteResponse` doc comment (it currently says it "deliberately carries no vote counts"). No account id, email, cell, session id or raw vote total appears anywhere.

### Copy (server authored, in `trustcopy.go`; drafts for UI-SPEC review, only the four D-19 sentences are approved)

| Element | Text |
|---|---|
| Count line | N=0 and M=0: empty. N=1: `Confirmed by 1 person nearby`. N of 2 or more: `Confirmed by N people nearby` (`99+` above 99). Append `, M disputed` for M of 1 or more. **N=0 and M of 1 or more: `M disputed`** (D-16 is silent, Open Question Q3) |
| Count reason | N of 1 or more: `N people in N different places confirmed this. Counts separate places, not votes.` (N=1: `1 person in 1 place confirmed this. Counts separate places, not votes.`). N=0, M of 1 or more: `M different places disputed this.` |
| Up to date | label `Up to date`; reason `R different places confirmed this in the last 2 hours.` (R is `recent_places`, hours from the window) |
| Getting old, Needs re-confirming | reason drafts: `Fewer than 2 places confirmed this in the last 2 hours.` and, when a dispute raised the bar, `Someone nearby disputed this, so it needs K confirmations in the last 2 hours.` Avoid second level relative times so a feed response and a vote response computed a moment apart stay equal |
| Too early to tell | `Fewer than 2 places have confirmed this so far.` (the 2 is `IndependentAgreementThreshold`) |
| Reporter, rated | `C of J earlier reports were confirmed by people nearby.` (C corroborated, J judged; same template for all three rated tags) |
| Reporter, new | `Only J finished reports so far. A rating needs 5.` |
| Unconfirmed chip | `Needs 2 confirmations before this counts as confirmed.` (empty unless the row is Provisional) |

A Go test enumerates a grid of inputs, collects every produced string and asserts none contains U+2014 or U+2013 (existing dash guards scan templates and selected JS only), and that all strings match a closed character class (digits, ASCII letters, space, `,.'%` and the hyphen in `re-confirming`). No user authored text (report description) may ever enter these strings, which is the stored XSS control.

### swag, docs, drift tests

- `make swag` (`swag init -g internal/api/router.go -o docs`) regenerates `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`. Verified deterministic against the committed files. Commit all three.
- Nested struct references and `enums:"..."` tags are already used by `FeedReportResponse`. For explicit nullability on the two `*int` scores, swagger 2.0 has no null; document in the field comment and optionally add `extensions:"x-nullable=true"` (not run this session, tagged ASSUMED).
- `swagger_test.go` `TestSwaggerSpecCoversRoutes` only checks substrings in the committed JSON, it does not compare against handlers. So add asserts for the definition `TrustResponse`, `CurrencyResponse`, `ReporterResponse` and every new field name, plus the four `band`, `tag`, `fade_stage` enum values. Add the **name guard**: parse `definitions`, walk every property key, and fail on any key matching `(?i)cell|account|session|email|geohash_cell` (the report's own `geohash` field is allowed, so do not ban the bare word geohash). Keep the existing `session_id` and `reporter_account_id` substring guards. Add a runtime analogue of `TestNearbyResponseOmitsSessionID` asserting neither a feed nor a vote response body contains `account_id`, `geohash_cell`, `voter` or the reporter's email.
- The `Deps{}` literal in that test uses `service.NewReportService(nil)`; keep the constructor variadic so it compiles.
- New doc `docs/TRUST-MODEL.md` (owned by this phase): precision derivation table, the 1 km rule and grandfathering, formulas and cutoffs, worked examples, and the honest limits (client coordinates unverified so spoofing defeats both rules; cell edge pairs count as two; N is a lower bound on people; window inherits an 8 hour lifetime for low severity rescue needed reports; a liar can start over with a fresh email; colluding accounts can dispute an honest reporter; pre Phase 3 votes were never distance checked and may inflate old reliability). It must record how the roadmap word "score" is satisfied (D-08) and must not claim spoof resistance. README gets a one line pointer.

## Front end (ask 6)

### Mount points (verified line anchors)

- `feed.js createRow` (line 147): meta appended, then `PinalertVisibility.createVisibilityTag()`, then `PinalertVotes.createVoteBlock(id)`. Insert `PinalertTrust.createTrustBlock()` between the chip and the vote block (chip, then count line, then line 2, then controls). `updateRow` (line 203) calls `PinalertTrust.updateTrustBlock(row.trust, report)` next to `updateVisibilityTag`. `row` object gains `trust`.
- `feed.js refreshAgeAndTime` (line 346) and `updateRow` (line 205): replace `Pinalert.ageStage(report)` with `PinalertTrust.fadeStage(report)`. Otherwise the 60 second timer reverts the fade to time only.
- `map.js`: `buildBadgeElement` (line 166, call at 168), `popupStateClasses` (line 241, call at 242), `syncPopupState` (line 271, call at 284): same replacement. `buildPopupContent` (line 191) mounts the block between chip and vote block and passes `{ showScores: true }` (D-09, R-07).
- `activity.js` builds `report` from data attributes today; add `trust: JSON.parse(row.dataset.trust)` inside try/catch and mount through the same builder. After Reopen, `PinalertTrust.updateTrustBlock(block, result.trust)` in the existing `.then` (R-08). No new `addEventListener` in this file (contract allows exactly one).
- `profile.html.tmpl`: add `data-trust="{{.TrustJSON}}"` on the `<li>` and `<script src="/static/js/trust.js?v={{.AssetVersion}}" defer>` between `visibility.js` and `activity.js` (existing order test only requires app, votes, visibility, activity strictly increasing, so an insert between keeps it valid). `html/template` escapes the JSON in an attribute context. `profileReport` gains `TrustJSON string`, produced from the same `trustToResponse` plus `json.Marshal`, so the Activity block cannot drift from the wire shape. R-08's clause "reports the account only voted on keep their current display" is moot: `ReportsByAccount` lists only reports the account posted (see A2).
- `index.html.tmpl`: script tag for `trust.js` after `visibility.js`, before `map.js` and `feed.js`; add `data-vote-radius-km="{{.VoteRadiusKm}}"` on `#app-shell` next to `data-default-radius-km`.

### Page config and the radius

`handlers.PageConfig` (built once in `cmd/server/main.go`) gains `VoteRadiusKm float64`, set from `service.MaxVoteDistanceKm`; `pageViewModel` copies it; the template renders it (Go renders `1.0` as `1`, as `DefaultRadiusKm: 10` renders `10`). `page_test.go` asserts the rendered attribute equals the constant. **`votes.js` must not read `Pinalert.config`** (contract forbids that literal, comments included). Read the attribute directly: `document.getElementById('app-shell')` then `getAttribute('data-vote-radius-km')`, returning null when absent (Activity page: no hint). No numeric fallback in JS (D-22: "never typed into a script"): absent or unparsable means no hint.

### `votes.js`: silent refresh, retry once, disabled hint

Current facts: `getVoterLocation()` (line 98) reads `sessionStorage['pinalert_voter_location']` as `{latitude, longitude}` with no timestamp and calls `getCurrentPosition(..., { timeout: 8000 })`; `castVote` (line 155) calls `getVoterLocation().then(fetch(...))`; `onControlsClick` refetches the feed after every successful vote. Design:

1. Cache entry becomes `{latitude, longitude, at}` (ms epoch). Entries without `at` count as stale.
2. `getVoterLocation(opts)` with `opts.fresh`: cache hit newer than a small freshness bound (for example 60 s, a client timer, not a trust threshold) is used. Otherwise, if permission is already granted (`navigator.permissions.query({name:'geolocation'})` state `granted`), call the single existing `getCurrentPosition` with `maximumAge` 0 silently; if it fails, fall back to the stale cached fix rather than blocking. If permission is `prompt` or the Permissions API is missing, reuse the cached fix and never prompt; only when there is no cache at all does the first vote prompt, exactly as today (D-17). `denied` still rejects with `GPS_DENIED_MESSAGE` (D-18).
3. Retry once inside `castVote`: on a rejection with `err.field === 'location'`, and only if permission is granted, call `getVoterLocation({fresh:true})` and post again through the same `fetch(` call site (implement as a local `attempt(fresh)` function in `castVote`'s own body that recurses once). If permission is not granted, do not retry (the same coordinates would fail again); show the server message.
4. Hint: `applyDistanceHint(block, report)` as a **separate function** called from `updateVoteBlock`. It reads the cache synchronously through a new `peekVoterLocation()` (no prompt, no promise). It applies only when the cached fix is younger than a hint bound (for example 2 minutes) so a stale fix can never lock a voter out; it skips reporter owned rows for resolve and reopen; it computes one great circle distance with a small JS haversine and, if farther than the radius, sets `aria-disabled="true"` plus a `data-too-far` marker on Confirm, Dispute and Mark resolved, and stores the hover sentence `Too far to vote. You need to be within R km of this report.` with R taken from the attribute. `onControlsClick` ignores clicks on `aria-disabled` buttons (the hover reason is shown by the trust tooltip module through a shared `PinalertTrust.bindReason`). The hint is removed when the fix is fresher and near, when the cache ages out, or when no fix exists.

### Accessible tooltip (WCAG 2.1 SC 1.4.13, Level AA: dismissible without moving pointer or focus, hoverable, persistent) [CITED: https://www.w3.org/WAI/WCAG21/Understanding/content-on-hover-or-focus.html]

- One body level tooltip element (created lazily by `trust.js`, `hidden` until used, `position: fixed`, high z-index above Leaflet panes and the toast, `pointer-events` on so the pointer can move onto it). Body level placement is required because Leaflet popups clip absolutely positioned children.
- Triggers are native `<button type="button">` elements (the three text items: count line, still current word, reporter tag), so keyboard focus, Enter and Space work with no extra role. Each trigger has `aria-describedby` pointing at a visually hidden span (class `.visually-hidden` exists in `main.css` line 707) holding the same reason sentence, so screen readers get the reason without hover. Ids must be unique per created block (module counter, not report id, because the same report appears in the list and the popup). The visible tooltip copies that span's text; `aria-hidden="true"` on the visible tooltip so nothing is announced twice. **No `aria-live` anywhere** (D-19).
- Show on `mouseenter` and `focus`; hide on `mouseleave` after a short delay unless the pointer entered the tooltip, on `blur`, on `Escape` (document level keydown while open, which satisfies "dismissible"), and on a click or tap outside. A click or tap on a trigger toggles it (touch has no hover; iOS Safari does not reliably focus a button on tap, ASSUMED A6, so a click handler is required and must not depend on focus).
- Triggers stop click and keydown propagation the way the vote block does, so a tap never activates the feed row (`activateRow`) or Leaflet's popup handlers.
- Chip: `.visibility-tag` is a span owned by `visibility.js` and guarded by existing CSS tests. Do not change its element type. `PinalertTrust.bindReason(el, sentence)` sets `tabindex="0"`, `role="button"`, `aria-describedby` and the same listeners on it (used for the chip and for the disabled vote buttons). The trust block's own items use real buttons.
- Popup rebuild: `map.js` rebuilds popup content on every 30 second poll, replacing trigger nodes. `trust.js` subscribes once to `Pinalert.subscribe` and, in a microtask after renders, hides the tooltip if its trigger is no longer in the document. Focus inside a rebuilt popup is lost, the same accepted limitation `votes.js` documents.
- Tab stops: up to three triggers per row add to two or three vote buttons; accepted, listed in Risks.

### CSS and layout

Append to `trust.css` only tokens (no hex, no `rgb(`, `rgba(`, `hsl(`: `TestVoteButtonsMeetTouchTargetAndUseTokensOnly` scans the whole file), four sizes (`--font-size-label` and `--font-size-body` suffice) and two weights, normal `--color-text` (muted text fails AA on light tints, and `.report-row__meta` already uses muted). Every rule with `display` needs the `:not([hidden])` guard (this repo's fifth encounter with the trap). Tooltip surface can copy `#toast` (inverted `--color-text` on `--color-bg`, 8px radius, no pill). Line 2 uses `display: flex; flex-wrap: wrap; gap: var(--space-sm)` so it stacks on narrow phones (about 236 px of row body at 320 px viewport by my arithmetic, tagged ASSUMED, not measured). The disabled hint styles `.vote-btn[aria-disabled="true"]` like the existing `.vote-btn:disabled` rule (muted text, border, surface, `cursor: not-allowed`) but keeps it focusable.

### Fade helper

`PinalertTrust.fadeStage(report)`: read `report.trust.currency.fade_stage`; if it is one of the three allowed strings, return the worse of it and `Pinalert.ageStage(report)` using the fixed order `fresh < aging < stale`; else return `Pinalert.ageStage(report)`. Holds no threshold. After the change no consumer may call `Pinalert.ageStage(` directly (add a contract test that `feed.js` and `map.js` contain zero occurrences). The band can lag the browser by up to one poll (30 s); document as accepted.

## Tests (ask 7)

### Conventions verified

- Real Postgres through `testutil.NewTestDB(t)` (applies embedded goose migrations, `TRUNCATE accounts, magic_link_tokens, email_cooldowns, reports, sessions, votes RESTART IDENTITY CASCADE`, skips when `DATABASE_URL` is unset). `make test` is `go test ./... -v -p 1` (packages share one database). `make test-short` is `go test ./... -short`, which in practice skips DB tests because `DATABASE_URL` is unset.
- Service tests: `trust_test.go` is in package `service` (can reach unexported symbols); `feed_test.go`, `auth_test.go`, `report_test.go` are `service_test`. Fakes are hand written recording fakes: `fakeVotingQuerier` (trust_test.go line 360), `fakeFeedQuerier` (feed_test.go line 16), `fakeAuthQuerier` (auth_test.go line 22). Each consumer interface (`Querier`, `VotingQuerier`, `AuthQuerier`) names shared generated methods separately by design, so each gets `ReporterHistoryReports` and each fake gets a matching method.
- Handler e2e tests: `newE2EServer`, `newVerifiedClient`, `postVote`, `decodeVoteResponse`, `floodReportBody`, `getNearby`, `decodeFeedResponse`.
- Web contract tests (`web/*_contract_test.go`) read embedded JS, CSS and templates as text. Helper `stripCSSComments` strips only `/* */` blocks, **not** `//` line comments, so a forbidden literal inside a `//` comment still fails. `web/design_rules_contract_test.go` whole file scans templates for U+2014 and U+2013, and gates pill radius (`999px`) in `main.css`.

### Existing tests that will break or need edits when this phase lands

| File | Reason | Fix |
|---|---|---|
| `internal/service/trust_test.go` (10 `ReportVoteContextRow{}` literals, 20 `CastVoteInput{}` uses) | zero value coordinates are 0,0 so every non reporter vote would now be too far | give the fixtures `Latitude: 12.9716, Longitude: 77.5946` through one shared helper; extend `fakeVotingQuerier` with `ReporterHistoryReports` |
| `internal/service/feed_test.go`, `auth_test.go` | new interface method | add to `fakeFeedQuerier`, `fakeAuthQuerier` |
| `internal/api/handlers/votes_e2e_test.go` lines 282 and 355 | voter C at `13.0500, 77.6500` is 10.58 km from the report | use **C = (12.9686, 77.5946)**: 0.3336 km south, precision 7 cell `tdr1v8y`, distinct from B `(12.9746, 77.5946)` cell `tdr1v9y` and from the report cell `tdr1v9q` |
| `internal/api/handlers/feed_visibility_e2e_test.go` lines 87, 132, 288 | same voter C | same replacement; keep `assertDistinctCells` |
| `web/votes_contract_test.go` `TestVoterLocationIsCachedPerSession` | anchors `function getVoterLocation()` with empty parens | change anchor to the new signature (for example `getVoterLocation(opts)`), keep the intent: one `sessionStorage.getItem(` and one `setItem(` inside, get before prompt |
| `web/votes_contract_test.go` `TestVoteTransportHasNoLocationFallback` | exactly one `navigator.geolocation.getCurrentPosition(`, exactly one `fetch(`, `castVote(reportId, action)` signature anchor, body order (`getVoterLocation(` before `fetch(`), no `Pinalert.config`, no `setView(` | keep all of these true: reuse the single prompt call and the single fetch call site; keep the `castVote(reportId, action)` signature; do not write those literals in any `//` comment in `votes.js` |
| `web/votes_contract_test.go` `TestOwnReportRuleRemovesControlsRatherThanDisablingThem` | forbids the word `disabled` in `applyOwnReportRule` and `updateVoteBlock` bodies; `setBlockBusy` must own `disabled` | hint lives in a separate function and uses `aria-disabled`; add a test that `applyDistanceHint` references `aria-disabled` and never `.disabled =` |
| `web/votes_contract_test.go` `TestBothSurfacesMountTheSameVoteBlock` and `TestBothSurfacesMountTheSameVisibilityTag` | `feed.js` and `map.js` may not contain `vote-btn`, `vote-controls`, `vote-error`, `your_vote`, `is_own_report`, `visibility-tag`, `vis-provisional`, `vis-hidden`, `visibility_reason`, `report.visibility`; createRow order `body.appendChild(meta)` before `createVisibilityTag(` before `createVoteBlock(`; same for `buildPopupContent` | the trust block and the hint are applied inside `trust.js` and `votes.js`; consumers call only `PinalertTrust.*`; add a parallel test banning trust class names and `report.trust` in consumers |
| `web/votes_contract_test.go` activity tests | `activity.js`: no `fetch(`, no `navigator.geolocation`, exactly one `addEventListener`, exactly one `Pinalert.showToast(`, no `Threshold`, `accountID`, `is_own_report`, `REOPEN_PENDING_TOAST`, `resolutionOutcomeMessage` | tooltip listeners live in `trust.js`; activity only calls `PinalertTrust.*` |
| `web/js_contract_test.go` `TestMapPopupCarriesReportStateClasses` | `bindPopup(` className option must be exactly `popupStateClasses(report)`; `setPopupContent(` must remain; `unbindPopup(` forbidden | keep; only change what `popupStateClasses` calls |
| `internal/api/handlers/swagger_test.go` | drift guard is substring only | extend as listed under "Wire and docs" |

Unchanged and still valid: `TestNearbyReportsUsesIndex`, `TestExpiryReadTimePredicate` (do not touch `NearbyReports`), the design rules tests, `TestProfilePageLoadsTrustAssetsInDependencyOrder` (an inserted `trust.js` keeps the four listed scripts strictly ordered).

### New test inventory (also the Validation Architecture map, see below)

Pure, no DB: haversine and radius boundaries (table above), `BoundingBox` containment property, `CastVote` distance table (four values, reporter versus non reporter, zero `InsertVote` calls on refusal), step order test, exemption tests, row filter property tests (permutation invariant, reporter excluded, one place per cell, one current row per account, confirm then dispute counts once), currency table and properties, fade mapping, classifier versus `Resolve` property, reliability table and invariants I1 to I4, copy dash and charset tests, geohash precision witness test, `Nearby` and `ActivityForAccount` batching (one vote read even with history, history skipped with no reporters, account id cap), `CastVote` trust equals feed trust.
DB: history index by name, history bounds, history window edge, `ReportVoteContext` returns coordinates, migration embedded and index present in `pg_indexes`, e2e too far (403, field `location`), e2e `trust` present and equal on feed and vote response, e2e Activity `data-trust`, e2e response bodies contain no `account_id` or `geohash_cell`, concurrency test (24 goroutines voting from 12 distinct in radius cells: final `N = 12`, no error, run with `-race`).
Contract (web): `trust.js` load order in both templates, both surfaces and Activity mount the same block, no consumer calls `Pinalert.ageStage(`, `trust.js` has no markup sink and no numeric threshold identifiers and no long dash anywhere (new file, whole file scan is fine), no `aria-live` in `trust.js`, tooltip handles Escape, focus, hover, click (static presence), `votes.js` keeps one prompt and one fetch, hint uses `aria-disabled`, `data-vote-radius-km` attribute present and read without `Pinalert.config`, trust CSS tokens only and `:not([hidden])` guards and no `999px`.

### Property tests worth planning

- **Dedupe:** N unchanged by adding accounts in an already counted place, by row order, by the reporter's own rows; a confirm then dispute by one account counts once as a dispute.
- **Monotonicity:** a new place never lowers N, recent places or currency score at fixed `now`; a dispute never raises currency; advancing `now` never raises currency; reliability improves with better outcome class; window edge is the only discontinuity (drop to New reporter at exactly 90 days is expected, pin it).
- **Concurrency:** many concurrent votes then one read equals the distinct places of the committed rows; distance check is stateless.
- **Determinism:** same inputs give the same `Trust` (no map iteration order leaks into strings or times).

## Don't Hand-Roll

| Problem | Don't build | Use instead | Why |
|---|---|---|---|
| Independence counting | a second cell dedupe | `independentCellCount` via the shared row filter | D-04 one rule |
| Visibility decisions for history | copies of Live and Hidden predicates | `Resolve` with neutral metadata | D-11, TRUST-02 |
| Distance in SQL for the vote check | SQL expression in `ReportVoteContext` | one Go `haversineKm` | testable, NaN safe, one place |
| Per account top N | client side loops or N queries | the single lateral query above | measured, index backed |
| Tooltip | `title` attribute alone | body level tooltip plus button triggers | `title` fails on touch and keyboard |
| Schema drift protection | trusting CI | `make sqlc`, `make swag`, then `git diff --exit-code` in the phase gate | CI has neither tool |
| Dash policing of server strings | eyeballing | one Go test over a grid of outputs | existing guards do not see Go strings |

## Common Pitfalls

### Pitfall 1: `getVoterLocation` refactor trips five contract assertions
`votes.js` may contain exactly one `navigator.geolocation.getCurrentPosition(`, exactly one `fetch(`, no `Pinalert.config`, no `setView(`, and `castVote(reportId, action)` must keep `getVoterLocation(` before `fetch(` in its own body. `getVoterLocation` must keep one `getItem` before the prompt and one `setItem`. Implement refresh and retry by reusing those call sites (a fresh flag; a local recursive attempt inside `castVote`). Comments count: do not write `Pinalert.config` or `fetch(` in any `//` comment.

### Pitfall 2: a natively disabled button cannot show its reason
Browsers do not deliver hover events to disabled buttons and do not focus them, so the D-22 hover reason and keyboard focus reason would never appear. Use `aria-disabled="true"` plus a click guard, which also satisfies the contract that `setBlockBusy` is the only owner of `disabled`.

### Pitfall 3: the generic index assertion passes without the new index
See "EXPLAIN test". Assert `idx_reports_session_expires` by name.

### Pitfall 4: popup content is rebuilt every poll
An open tooltip or focused trigger inside a Leaflet popup is destroyed by `setPopupContent`. Hide the tooltip when its trigger detaches; accept lost focus.

### Pitfall 5: clocks
`votes.created_at` comes from the database `now()`; band math uses Go's clock. Clamp negative ages. Tests must set `created_at` explicitly by SQL and inject `now`.

### Pitfall 6: two `WithClock`s
`service.WithClock` already returns `AuthOption`. New options need distinct names.

### Pitfall 7: current votes contain resolution rows too
`CurrentVotesForReports` is per (report, account, kind). Currency and classification must read content rows only, and must exclude the reporter, exactly as `BuildVoteTally` does.

### Pitfall 8: the feed and vote responses can differ by a poll tick
Equality tests between them must not include second granularity text. Prefer integers and hour granularity words.

### Pitfall 9: `NearbyReports` has no row limit and radius reaches 50 km
History work multiplies with distinct reporters on the page. See Security (account id cap).

## Runtime State Inventory

Not a rename or migration phase. Stored data: one new index only, no data migration. Live service config, OS registered state, secrets and build artifacts: none. **Grandfathered data:** votes written before this phase were never distance checked and cannot be re-checked (only cells are stored); they keep counting for N and for old reliability, and the docs say so. The owner's dev database must have `make migrate` run once.

## Environment Availability

| Dependency | Required by | Available | Version | Fallback |
|---|---|---|---|---|
| Go | build and tests | yes | 1.26.4 | none |
| `sqlc` | regenerate store code | yes | v1.31.1 | none, output is committed |
| `swag` | regenerate docs | yes | v1.16.4 | none |
| `goose` CLI | not needed (embedded `goose.Up` and `cmd/migrate`) | yes | n/a | n/a |
| PostgreSQL | DB tests, EXPLAIN test | yes, local, accepting connections on the unix socket in `/tmp:5432` | 16.14 (Homebrew) | none for DB tests |
| Docker | not needed | no daemon running | n/a | use local Postgres |
| Browsers | human UAT of tooltip, hint, touch | owner's machine | n/a | manual step |

**Missing dependencies with no fallback:** none. **Do not use `pinalert_test`** (owner dev data, see the incident note above); create `createdb p3_verify` (or similar) and drop it after.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (go 1.26.4); web contract tests are ordinary Go tests reading embedded assets |
| Config file | none (`Makefile` targets `test`, `test-short`, `vet`, `sqlc`, `swag`) |
| Quick run command | `go test ./internal/service/ ./web/ -count=1` (no DATABASE_URL, pure tests and contract tests, about 2 s) |
| Full suite command | `DATABASE_URL="postgres://localhost:5432/<disposable>?sslmode=disable" go test ./... -p 1 -count=1` (baseline before this phase: 11 test packages, all `ok`, about 15 s wall including builds; that baseline was taken on the wrong database, see incident, and is only informational) |
| Disposable database | `createdb p3_verify` before, `dropdb p3_verify` after. **Never `pinalert_test`** (02-14-PLAN.md line 173 already sets this rule; the owner's dev server on port 8090 uses it) |

### Phase Requirements to Test Map

| Req / Decision | Behavior | Test type | Automated command | File exists? |
|---|---|---|---|---|
| V-01 D-01 | Haversine correctness, symmetry, NaN and Inf refused, India latitude table | unit | `go test ./internal/service/ -run 'TestHaversine|TestWithinVoteRadius' -count=1` | Wave 0 (geo_test.go) |
| V-02 D-01 | Voter inside 0.9896 km accepted, 1.0008 km refused (report 12.9716,77.5946) | unit | same package `-run TestWithinVoteRadiusBoundary` | Wave 0 |
| V-03 D-01 | `BoundingBox` contains every point within R (oracle for helper) | property | `-run TestHaversineInsideBoundingBox` | Wave 0 |
| V-04 D-01 | `CastVote` refuses all four values from far, zero inserts | unit | `go test ./internal/service/ -run TestCastVoteRejectsTooFar -count=1` | Wave 0 (trust_test.go) |
| V-05 D-01 | Order: own report before expiry before distance | unit | `-run TestCastVoteCheckOrder` | Wave 0 |
| V-06 D-01 | Reporter resolve or reopen exempt from anywhere, non reporter not, accountless exempts nobody | unit | `-run TestCastVoteReporterExemption` | Wave 0 |
| V-07 D-01 | Handler returns 403 `field: location`, fixed copy, no distance in body | e2e (DB) | `go test ./internal/api/handlers/ -run TestCastVoteTooFar -count=1` | Wave 0 |
| V-08 D-01 | Existing vote e2e still pass with C = 12.9686,77.5946 | e2e (DB) | `go test ./internal/api/handlers/ -count=1` | edit existing |
| V-09 D-01 | `ReportVoteContext` returns report coordinates | store (DB) | `go test ./internal/store/ -run TestReportVoteContextReturnsCoordinates` | Wave 0 |
| V-10 D-03 | Cell width and witness test (at most 24 distinct cells from 200 witnesses in 300 m at five latitudes) | unit | `go test ./internal/service/ -run TestGeohashPrecisionWitnesses` | Wave 0 |
| V-11 D-04 TRUST-05 | `ConfirmedPlaces == ConfirmCells`, dedupe, permutation, reporter excluded | property | `-run TestBuildTrustCountsMatchTally` | Wave 0 |
| V-12 D-16 R-04 TRUST-05 | Count line grammar (0, 1, 2, 99, 100, disputes, N=0 with M) | unit | `-run TestTrustCountLabel` | Wave 0 |
| V-13 D-05 R-01 TRUST-07 | Currency boundary table (W-1ns, W, W+1ns, 2W, 2W+1ns, 4W, skew) for 8 h and 24 h | unit | `-run TestCurrencyScoreBoundaries` | Wave 0 |
| V-14 R-01 | N below 2 gives null score and `not_yet_corroborated`, also for critical | unit | `-run TestCurrencyNotYetCorroborated` | Wave 0 |
| V-15 D-05 | One account re-tapping cannot refresh; two places can | unit | `-run TestCurrencyRetapCannotRefresh` | Wave 0 |
| V-16 lean | Dispute raises the bar by one place, never refreshes; contested gives 0 | unit | `-run TestCurrencyDisputes` | Wave 0 |
| V-17 D-05 | Window is a quarter of lifetime (8 h and 24 h, rescue needed low severity is 8 h) | unit | `-run TestCurrencyWindowFollowsLifetime` | Wave 0 |
| V-18 | Currency monotone in time and places | property | `-run TestCurrencyMonotone` | Wave 0 |
| V-19 D-07 R-02 R-03 | Fade stage mapping, none for critical, rescue needed, Hidden, not applicable | unit | `-run TestFadeStageMapping` | Wave 0 |
| V-20 D-11 D-12 R-05 R-06 | Outcome classification table, equals `Resolve` on neutral inputs | unit and property | `-run TestClassifyOutcome` | Wave 0 |
| V-21 D-13 | Reliability worked table, boundaries (n 4 vs 5, 30 cap, exact 75 and 35) | unit | `-run TestReliabilityTable` | Wave 0 |
| V-22 D-13 D-15 | Invariants I1 to I4 (spotless never Mixed, unconfirmed only Mixed, monotone, bounded) | property | `-run TestReliabilityInvariants` | Wave 0 |
| V-23 D-13 | Half life weights and 90 day window edge | unit | `-run TestReliabilityDecayAndWindow` | Wave 0 |
| V-24 D-13 | Index named `idx_reports_session_expires` used, no Seq Scan on reports | store (DB) | `go test ./internal/store/ -run TestReporterHistoryUsesSessionIndex` | Wave 0 |
| V-25 D-13 | History bounds: window, cap 30, expired only, multi session, accountless | store (DB) | `-run TestReporterHistoryBounds` | Wave 0 |
| V-26 D-13 | Query source drift guard | store | `-run TestReporterHistoryQuerySourceHasExpectedShape` | Wave 0 |
| V-27 D-13 | Migration 00005 embedded and index exists | store (DB) | `go test ./internal/store/ -run 'TestMigrations|TestSessionExpiresIndexExists'` | edit existing |
| V-28 D-10 | Reliability keyed by account across sessions | store and service | `-run TestReporterHistoryUsesAccountNotSession` | Wave 0 |
| V-29 D-21 | Feed and vote response carry equal `trust`; batching keeps one vote read; account cap | service and e2e | `-run 'TestNearbyBatches|TestFeedVisibilityMatchesCastVoteResponse'` | edit existing |
| V-30 D-21 | Swagger has the new definitions, enums, no `cell`, `account`, `session`, `email` property names | unit | `go test ./internal/api/handlers/ -run TestSwaggerSpecCoversRoutes` | edit existing |
| V-31 D-21 | Response bodies contain no `account_id`, `geohash_cell`, email | e2e (DB) | `-run TestTrustResponseOmitsIdentifiers` | Wave 0 |
| V-32 D-16 to D-19 | Every server string is dash free and from the closed charset | unit | `go test ./internal/service/ -run TestTrustCopy` | Wave 0 |
| V-33 D-20 R-08 | Activity rows carry `data-trust`; block updates from `CastVoteResponse.trust` after Reopen | e2e and contract | `go test ./internal/api/handlers/ ./web/ -run 'TestProfile|TestActivity'` | Wave 0 |
| V-34 D-22 | `data-vote-radius-km` equals `service.MaxVoteDistanceKm`; read without `Pinalert.config` | unit and contract | `go test ./internal/api/handlers/ ./web/ -run 'TestPageShell|TestVoteRadius'` | Wave 0 |
| V-35 D-02 | `votes.js` one prompt, one fetch, cache with timestamp, retry once path present | contract | `go test ./web/ -run TestVoteTransport -count=1` | edit existing |
| V-36 D-22 | Hint uses `aria-disabled`, never `disabled` in `updateVoteBlock` | contract | `go test ./web/ -run TestVoteHint` | Wave 0 |
| V-37 D-07 R-02 | No `Pinalert.ageStage(` in `feed.js` or `map.js`; both use `PinalertTrust.fadeStage` | contract | `go test ./web/ -run TestFadeUsesCombinedStage` | Wave 0 |
| V-38 D-18 D-19 | Same block mounted on both surfaces and Activity; no banned literals in consumers; load order | contract | `go test ./web/ -run TestTrust` | Wave 0 |
| V-39 D-19 | Tooltip: Escape, focus, hover, click present; no `aria-live`; no markup sink | contract | `go test ./web/ -run TestTrustTooltip` | Wave 0 |
| V-40 D-18 | trust CSS tokens only, `:not([hidden])` guards, no `999px`, no long dash | contract | `go test ./web/ -run 'TestTrustCSS|TestNoPillShapedControls|TestUserVisibleCopy'` | Wave 0 |
| V-41 TRUST-09 spirit | Concurrent votes then N equals distinct places | concurrency (DB) | `go test ./internal/api/handlers/ ./internal/store/ -race -run TestTrustCountUnderConcurrentVotes` | Wave 0 |
| V-42 all | Hover, tap, keyboard focus on a touch device and desktop; screen reader reads reasons; too far hint with DevTools Sensors location override; Reopen refresh on Activity; 320 px width wrapping | manual UAT | script in the phase UAT file | manual |

### Sampling Rate
- **Per task commit:** `go test ./internal/service/ ./web/ -count=1` (no database, about 2 s), plus the package the task touched.
- **Per wave merge:** full suite against the disposable database, `-p 1 -count=1`.
- **Phase gate:** full suite green; `go vet ./...`; `sqlc generate` then `git diff --exit-code internal/store/sqlc`; `swag init -g internal/api/router.go -o docs` then `git diff --exit-code docs`; manual UAT items V-42.

### Wave 0 Gaps
- [ ] Disposable database procedure written into the plans (and never `pinalert_test`).
- [ ] `voteRowAt` helper (vote row with explicit `created_at`) in the service tests; helper to build many places quickly.
- [ ] History fixture seeder (accounts, bound sessions, reports with explicit `expires_at`, votes with explicit `created_at`), likely in `internal/testutil`.
- [ ] Clock options on `ReportService` and `VotingService` (deterministic e2e), `ActivityForAccount` uses `s.now()`.
- [ ] Three fakes gain `ReporterHistoryReports`; `ReportVoteContextRow` fixtures gain coordinates.
- [ ] e2e helpers `decodeTrust` and a swagger property name walker.
- [ ] New contract test files for trust (`web/trust_contract_test.go`) and the updated `votes.js` anchors.

## Security Domain

ASVS level 1 (`config.json`: `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high`).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no change | existing verified account gate (`requireVerifiedAccount`) |
| V3 Session Management | no change | existing session cookie |
| V4 Access Control | yes | reporter exemption keyed on server side reporter identity only; distance check for everyone else; reliability keyed on `sessions.account_id` |
| V5 Input Validation | yes | coordinate range check plus NaN and Inf fail closed; body decoder keeps `DisallowUnknownFields` (no client cell, no client distance) |
| V6 Cryptography | no | none introduced |
| V8 Data Protection | yes | no account id, email, voter cell or raw vote total on the wire or in swagger; name guard test |
| V11 Business Logic | yes | distance rule, one account one current vote, min 5 judged, window and cap |
| V14 Configuration | yes | radius from one constant; migrations explicit |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Bypass the distance check by claiming to be the reporter | Elevation | exemption computed from `rc.ReporterAccountID` (database) versus authenticated `in.AccountID`, resolution kind only; unit tests V-06 |
| NaN or Inf coordinate slips past `< -90 \|\| > 90` | Tampering | `!(d <= radius)` comparison; test V-01 |
| Spoofed GPS defeats cell rule and 1 km rule | Spoofing | not preventable client side; documented honestly, cost is verified accounts plus a claimed location (D-03); accuracy and mock detection deferred |
| Leak voter cell or account id | Information disclosure | trust object carries counts, labels, integers only; swagger name guard; runtime body test V-31; the too far error carries no distance |
| History read DoS: `NearbyReports` has no row limit and radius up to 50 km, history is 30 reports per distinct reporter, then a vote read over that many ids, on every 30 second poll per client | Denial of service | bounded per account by `LIMIT 30`; **add a Go side cap on distinct account ids per read** (recommended constant 200, nearest reports first because rows arrive nearest first; accounts beyond the cap get tag `none`); index; measured 68 ms for 300 accounts, so roughly 45 ms at the cap; test that the cap holds; optional in process short TTL cache is deferred (conflicts with recompute on every read) |
| Stored XSS through server authored strings | Tampering | strings built only from constants and integers, never from report text; browser writes with `setText`; `data-trust` JSON is attribute escaped by `html/template` and parsed with `JSON.parse` inside try/catch; enum values used in class names validated against allowlists in `trust.js` |
| Long dash or unexpected characters in server copy | Tampering (integrity of copy) | Go test over a grid of outputs |
| Grandfathered far away votes inflating old records | Repudiation | documented in TRUST-MODEL.md; tests run on a fresh database |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CONTEXT says `NearbyReports` is "pinned byte for byte by a drift constant and an EXPLAIN test". Verified: `nearbyReportsSQLForExplain` is a hand copy and `TestNearbyReportsQuerySourceHasExpectedShape` only checks two substrings (`expires_at > now()` and `BETWEEN`). The instruction not to modify it still stands. | Anti-patterns, EXPLAIN test | Low: a stronger drift guard was assumed to exist; the new guard should be stronger |
| A2 | R-08 says reports the account only voted on keep their display. Verified `ReportsByAccount` returns only reports the account posted, so no such rows exist. | Front end | None |
| A3 | Score constants `c = 0.4`, tag cutoffs 75 and 35, band cutoffs 75 and 50, 25 points per window, prior of one confirmed and one unconfirmed report: design choices by the researcher within the CONTEXT discretion grant, not facts | Score formulas | Medium: labels real people, owner may want different values (Q1, Q2) |
| A4 | Performance numbers (1.5 s to 9 ms, 20 ms to 1 ms, 68 ms) come from a local Postgres 16.14 scratch database with synthetic data, not from the hosting target | Reliability history read | Low: order of magnitude and plan shape hold, absolute times will differ on Neon or Supabase |
| A5 | `x-nullable` swag extension tag on the two `*int` fields works with swag 1.16.4 (not run) | Wire and docs | Low: skip the tag and rely on the field comment |
| A6 | iOS Safari does not reliably focus a button on tap, so the tooltip needs a click handler rather than focus alone | Front end | Low: the click handler is harmless where focus does work |
| A7 | `navigator.permissions.query({name:'geolocation'})` is usable in the target Safari versions. MDN says the Permissions API is Baseline since September 2022 with "some parts" varying and lists geolocation as permission aware, but did not confirm Safari support of that specific name this session. Design already falls back to reuse the cached fix when the API is missing or throws | Front end | Low: worst case D-02's silent refresh does not run on that browser and the voter sees the too far message with the cached fix |
| A8 | Row body width of about 236 px at a 320 px viewport is arithmetic (viewport minus row padding, badge, gap, border), not measured | Front end CSS | Low |
| A9 | Neon or Supabase pooler behaviour with pgx statement caching was not checked (out of scope, DB host still Pending) | Environment | Low for this phase |

## Open Questions

1. **Q1 (owner): how should an unconfirmed only history read?** With the recommended formula and `c = 0.4`, a reporter whose reports all expired unconfirmed scores 43 (n = 5) to 41 (n = 30) and is tagged **Mixed record**, and only contradicted outcomes can make anyone **Unreliable reporter**. Alternatives: `c = 0.25` gives 32 (n = 5) and 27 (n = 30), so five unconfirmed reports read Unreliable; `c = 0.5` gives exactly 50 at any n, so unconfirmed is neutral, not negative, which contradicts D-11 "mild negative". Recommendation: `c = 0.4`, because D-12's spirit is that nobody is marked down for a quiet area, and "Unreliable reporter" reads as an accusation. The property test for I2 pins whatever the owner decides.
2. **Q2 (planner may decide, owner may veto): cutoffs.** Reliable at 75 or more, Mixed 35 to 74, Unreliable 34 or less; Up to date 75 or more, Getting old 50 to 74, Needs re-confirming 49 or less. Table above shows where typical histories land (4 of 5 corroborated with one contradicted is 71, Mixed; 8 corroborated and 2 contradicted is exactly 75, Reliable).
3. **Q3 (owner): count line when N is 0 and M is 1 or more.** D-16 says nothing at zero and appends `, M disputed`, so the joined sentence is undefined. Recommendation: show `M disputed` alone. Can occur on a Provisional row with one dispute.
4. **Q4 (owner, low stakes): D-13 says the read is capped at the latest 30 judged reports.** SQL can only cap the latest 30 expired reports (judging needs votes). They differ only for reporters with many critical or rescue needed reports that expired unconfirmed (neutral by D-12). Exact alternative: fetch 60, take the first 30 judged in Go, at double the read cost. Recommendation: cap 30 expired reports and document.
5. **Q5 (planner): score for Hidden, Retracted and expired rows.** Recommended `null` with band `not_applicable` and no word (R-03 and R-08 only speak about display). If Phase 6 wants a score on Hidden rows, revisit.
6. **Q6 (planner): status code.** Recommended 403 with field `location`; 400 via `ValidationError` is the zero handler alternative.
7. **Q7 (planner): account cap constant (200) and behaviour past it.** Recommended tag `none` beyond the cap.

## Risks a planner must schedule around

1. **Phase 2 verification is still pending.** `02-VERIFICATION.md` (2026-09-18) has `status: human_needed`, `score: 8/13`, `behavior_unverified: 5`; `02-UAT.md` has `status: fixes_shipped_pending_retest` (GPS denial network check, Provisional dimming and the Show disputed empty state, Mark resolved row replacement, Safari Back after Reopen, theme legibility in light mode). ROADMAP marks Phase 2 complete. This phase builds directly on the resolver and edits `votes.js`, `feed.js` and `map.js`, so run `/gsd-verify-work 2` first if possible. Phase 1.1's live email delivery check is a separate open item.
2. **Owner dev data was lost** (see incident). Plan the human UAT with a fresh database and fresh accounts; the manual dev server needs `make migrate` after this phase merges.
3. **Grandfathered votes.** Pre phase votes are uncounted for distance and still count for N and old reliability; too far tests must run on a fresh or reset database; docs must say so.
4. **Phase 5 demo mode** cannot vote from far away under D-01 (CONTEXT handoff). Not solved here.
5. **Poll lag:** bands and fades update per 30 second poll; the 60 second timer only re-combines the stored server stage with the time stage.
6. **Keyboard tab stops:** up to three new focus stops per row.
7. **Real phone GPS error** (tens of metres, worse indoors) can push a voter standing near 1 km over the line; accuracy is not sent (deferred), the retry with a fresh fix mitigates stale fixes only.
8. **Copy overflow:** the longest count line (`Confirmed by 99+ people nearby, 99+ disputed`) wraps on narrow phones; CSS must allow wrap.
9. **Regeneration discipline:** CI cannot detect a forgotten `make sqlc` or `make swag`; the phase gate commands above do.

## State of the Art

| Old approach | Current approach | Impact |
|---|---|---|
| Vote from anywhere counts as nearby | Server refuses votes over 1 km (D-01) | "nearby" becomes true for new votes |
| Raw or unchecked confirmation display | N from distinct places, lower bound on people | matches Core Value |
| Time only fade (`ageStage`) | Worse of time stage and server currency stage for non critical rows | contradictory cues removed |
| Stored reputation scores (research notes) | Recomputed on read, integer 0 to 100, never stored | matches Phase 2 D-08 |

**Deprecated or stale wording not to copy:** TRUST-03 "distinct anonymous session" (live rule is verified account plus cell), "reports fade unless re-confirmed" (expiry is flat by severity, nothing extends it), reliability keyed to an anonymous session (FEATURES.md lines 33 and 69, PROJECT-NOTES.md line 240), the STATE.md blocker's implication of two precision constants.

## Sources

### Primary (HIGH confidence, read or executed this session)
- Live repository: `internal/service/{trust,visibility,report,feed,auth}.go`, `internal/store/queries/{votes,reports}.sql`, `internal/store/migrations/00001` to `00004`, `internal/store/{reports_test,votes_test}.go`, `internal/api/handlers/{votes,reports,auth,page,swagger_test}.go`, `cmd/server/main.go`, `web/static/js/{votes,feed,map,app,visibility,activity}.js`, `web/static/css/{main,trust,feed}.css`, `web/templates/{index,profile}.html.tmpl`, `web/*_contract_test.go`, `Makefile`, `sqlc.yaml`, `.github/workflows/ci.yml`
- Scratch experiments (databases and directories deleted afterwards): sqlc generation of the new query and byte stability check; swag byte stability check; EXPLAIN ANALYZE runs on 200k and 20k report tables; vote read timing at 72k rows; geohash cell and 300 m witness simulation; formula tables
- [W3C Understanding SC 1.4.13 Content on Hover or Focus](https://www.w3.org/WAI/WCAG21/Understanding/content-on-hover-or-focus.html) (Level AA; dismissible, hoverable, persistent)
- [MDN Geolocation.getCurrentPosition](https://developer.mozilla.org/en-US/docs/Web/API/Geolocation/getCurrentPosition) (`maximumAge` default 0, `timeout` default Infinity, prompt only if permission not yet granted)
- [MDN Permissions API](https://developer.mozilla.org/en-US/docs/Web/API/Permissions_API) (querying does not prompt; geolocation is permission aware; Baseline since September 2022)

### Secondary (MEDIUM)
- Prior discussion briefs and reconciler in the session scratchpad (`brief-*.json`, `plan.json`); every fact relied on here was re-verified against the live code above.

### Tertiary (LOW)
- none relied on

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new packages, versions read from `go.mod` and CLIs
- Architecture and change map: HIGH, every symbol and line anchor read this session
- Formulas: MEDIUM, arithmetic verified by script and boundary tables, constants are design choices flagged for the owner
- Front end: MEDIUM, contract constraints verified in test source, cross browser behaviour partly ASSUMED (A6, A7)
- Pitfalls: HIGH for repository specific ones

**Research date:** 2026-09-30
**Valid until:** 2026-10-30 (stable stack; re-check only if `votes.js` or the contract tests change before planning)
