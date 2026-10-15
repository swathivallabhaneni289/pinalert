---
phase: 04
slug: robustness-real-world-resilience
mode: mvp
granularity: standard
plans: 29
waves: 9
created: 2026-10-06
---

# Phase 4: Plan Outline

> Chunked run. One single plan agent writes each row below from this outline.
>
> **Precedence when sources disagree (highest first):** (1) `04-CONTEXT.md` D-01 to D-24 and R-01 to R-10
> (locked) and the owner choices of 2026-10-05 recorded in `04-UI-SPEC.md` "Decisions carried forward";
> (2) `04-UI-SPEC.md` (approved) for everything visual or interactive, including amendments P4-A1 to P4-A28;
> (3) `04-RESEARCH.md` for engineering approach, with its Open Questions resolved by the planning brief's
> orchestrator decisions 1 to 8; (4) `04-PATTERNS.md` for which analog to copy. The brief's verified
> corrections 1 to 13 and the "UI contract code facts" U1 to U11 at the top of RESEARCH override any older
> sentence in RESEARCH, PATTERNS, VALIDATION or UI-SPEC. This outline's pins override all of them for names.
> A pin that restates a locked CONTEXT decision must agree with it (a disagreement is a defect, not an override).
> The "Validation rows" command overrides supersede 04-VALIDATION.md (no plan edits VALIDATION, RESEARCH or PATTERNS).
>
> **Precondition risks (not plans).** Phases 3.1 and 3.2 are NOT built, and Phase 3 is only partly built (measured 2026-10-08: `internal/service/reliability.go`, `reporter.go`, `trustview.go`, `trustcopy.go` and their tests exist, and `app.js` already has `formatShelterLine`; every plan still re-greps the built code in its first task). The 20 Phase 3 plan files are
> final (closed at commit b2d71bb); Phases 3.1 and 3.2 have no plans, only ROADMAP and UI-SPEC "Cross phase
> hand offs" notes. Phase 4 executes after all three land (ROADMAP order 3, 3.1, 3.2, 4). Every plan that
> touches a symbol, file or test those phases create carries `## Cross-phase prerequisites` and a first task
> that re-greps the built code and stops with a report if anything is absent or renamed. Migration numbers
> are never written: "the next free number at execution time".

**Phase Goal (every plan restates it under `## Phase Goal`):**

**As a** person reading or reporting during an emergency, often on a weak phone connection, **I want to**
keep reading, posting and confirming reports when the signal is bad, knowing abusive posts are screened out
and that the site says plainly it is unofficial, **so that** the reporting loop keeps working and I am not
misled about what Pinalert is.

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| 04-01 | `Resolve` gives a removed report `withheld` above every other rung (even critical), gives a pending non serious report `withheld`, lets a pending serious report fall through to the critical bypass, keeps the zero value neutral, and `withheld` is listable in no view; `ReportMeta.Serious()` and `ReportMeta.Withheld()` are the predicates (R-01, R-02, R-03, R-04, D-02) | 1 | none | ROBUST-03 |
| 04-02 | One reusable text policy decides pass, borderline or reject from provider scores (only hate, harassment and sexual reject; violence, self harm and illicit never do), rejects gibberish counted in runes, and names the problem in four plain messages (D-03, D-04, D-05, D-06, R-06) | 1 | none | ROBUST-03 |
| 04-03 | A keyed token bucket with reserve, wait time and a refund that really refunds (cancel at the reservation instant), an injected clock and an idle window per bucket proves burst, refill, key independence, refund and the concurrent bound under `-race` (D-10, D-11, correction 1, correction 3) | 1 | none | ROBUST-04 |
| 04-04 | A provider client sends only the trimmed text to the pinned snapshot with a 3 second deadline, classifies 429, 5xx and timeouts as transient and quota and other 4xx as permanent, honours `Retry-After`, a loader picks the real client, a development pass screener or a startup error, and an opt in recorder can capture fixture scores (D-01, D-07, Claude's Discretion provider client) | 1 | none | ROBUST-03 |
| 04-05 | The database can hold moderation state, a display only written time and a request id: a retried insert with the same account and request id makes one row while another account gets its own, the account scoped replay finds it, flags are unique per account, the worker claim hands a check to one claimant, the extras read is one batched call, and seeded rows are `clear` with no check row (R-01, R-05, R-10, D-04, D-08, D-09, D-19; store parts of D-01, D-02, R-03) | 1 | none | ROBUST-02, ROBUST-03, ROBUST-09 |
| 04-06 | Every existing full page (login, verify outcome, map, Activity) shows the thin unofficial bar first in `<body>` inside `.top-chrome`, with the exact D-22 sentence composed from one numbers setting and a link to the Disclaimer, and every full page renders with the shared `Chrome` data (D-21, D-22, D-23 bar link, D-24 setting, R-08, P4-A1 markup). Accepted interim look until 04-08 (MVP framing): 04-06 does not edit main.css or auth.css to hide it, and no UAT, deploy or human check runs between wave 1 and wave 2 | 1 | none | ROBUST-08 |
| 04-07 | A held or removed report is in no feed, no map, no history view and no vote: `Nearby`, `CastVote` and `ActivityForAccount` pass moderation into `Resolve` through one batched extras read, a vote on it answers 404, the Activity read carries its state and display only written time, and a source guard keeps every future `Resolve` caller honest (R-01, R-02, R-05, D-02, D-19, corrections 5 and 6) | 2 | 04-01, 04-05 | ROBUST-03 |
| 04-08 | The pages fit under the bar: the account trigger sits 16px under the bar at any height, the map, shell and login page subtract the bar token, the popup reserves read against the shorter map, focus never lands under the bar, the shared inline note and error looks, plain button states and destructive fills exist, and one scan proves no JS file uses a markup sink (P4-A1 to P4-A4, P4-A16, P4-A17, P4-A20, P4-A25, P4-A28, D-05, correction 8). Its `<human-check>` (run in 04-27) confirms at 320x568 and 1280x800, on the map and Activity pages, that the account trigger sits fully below the bar with a 16px gap and that the map and login pages do not scroll at default text size (removes the 04-06 interim look, MVP framing) | 2 | 04-06 | ROBUST-08 |
| 04-09 | Votes are throttled per verified account (20 quickly then 1 every 15 s, a 30 s re-cast cooldown refunded on any non 2xx) with a 429, a `Retry-After` header and a specific wait message, the network backstop applies only with a configured header, and every limit value lives in one named place (D-10, D-11, D-12, WR-01, corrections 1 and 3) | 2 | 04-02, 04-03, 04-06 | ROBUST-04 |
| 04-10 | The report form logic moves into a shared `report-form.js` behind a small location interface that `modal.js` implements with Leaflet, with no change to the main page's behaviour beyond the Cancel report label and the owner's description hint (2026-10-08, D-05), and every pinned test re-pointed, merged green before the Low data view uses it (D-15 prerequisite, P4-A23, RESEARCH "The form extraction is the riskiest refactor") | 2 | 04-06 | ROBUST-01 |
| 04-11 | Posting runs the text check inline: abusive text is refused with a named reason and nothing stored at every severity, borderline text posts and is queued for review, an unfinished check holds a non serious report off every feed (the 201 says `held`) and publishes a serious one, and the description length counts characters (D-01, D-02, D-03, D-04, D-06, R-03, R-06) | 3 | 04-02, 04-04, 04-05, 04-07, 04-09 | ROBUST-03 |
| 04-12 | A logged out visitor can read the public Disclaimer and Policy pages: no account data, a private short cache, the numbers block from the one setting with 112 as a tap to call link, the label glossary from the app's own sentences, and the contact blocks only when an address is configured (D-22, D-23, D-24, R-07, P4-A18) | 3 | 04-06, 04-08, 04-09 | ROBUST-08, ROBUST-10 |
| 04-13 | On a weak connection the main page keeps its list and says "Last updated N minutes ago" instead of hiding it, a stalled poll times out after 10 seconds, and when the list has not loaded in about 6 seconds a notice says "Taking long? Open the low data view." even when unpkg hangs (D-17, D-20, P4-A12, P4-A19, P4-A24) | 3 | 04-06, 04-08, 04-10 | ROBUST-01, ROBUST-02 |
| 04-14 | A report held because the check could not finish is retried in the background with backoff, jitter and `Retry-After`, released on a late pass, released as clear and listed for review (`needs_review`) on a late borderline, withdrawn as removed on a late reject (serious included) and never retried once expired (D-01 background retry, D-02, D-04, R-03) | 4 | 04-11 | ROBUST-03 |
| 04-15 | Report posting has two separate allowances keyed on the account (serious 5 then 1 every 5 minutes, other 8 then 1 every 15 minutes) chosen by `Serious()`, a rejected, invalid or failed post costs nothing and the submit attempt bucket is wired here (a replay never reaches the allowance, 04-17), and the 429 tells how long to wait, carries `serious` and the emergency number, and every 429 is documented in OpenAPI (D-10, D-11, D-12, R-04, orchestrator decision 1) | 4 | 04-09, 04-11, 04-12 | ROBUST-04 |
| 04-16 | An author sees their own held report as "Waiting for a text check" and a removed one as "Removed after review" on the Activity page, with no trust block, counters or vote controls, and a held row past expiry shows Expired (D-02, D-08, R-01, R-02, P4-A9, P4-A10, P4-A8 Activity line) | 4 | 04-07, 04-08, 04-12 | ROBUST-03, ROBUST-09 |
| 04-17 | A report retried after a lost answer is never stored twice (200 with the stored report on a repeat, checked before any allowance is reserved, so a replay costs nothing), the server stamps every time, and a report sent more than 30 minutes after it was written shows "Written 4 hours ago" on the feed row and the popup (D-19, R-05, P4-A8) | 5 | 04-07, 04-13, 04-15 | ROBUST-02 |
| 04-18 | A signed in person can open `/low`, a text only page with no map that lists Active reports at once from the last known position using the same trust block and rows, remembers the choice per device and redirects a bare `/`, under the asset policy (no map library, raster image or web font) with response compression (D-13, D-14, D-16 reading, D-17 cookie, R-08, R-09, P4-A14, P4-A22, orchestrator decision 2) | 5 | 04-06, 04-08, 04-13, 04-15 | ROBUST-01 |
| 04-19 | On the Low data view a person can post a report with their phone position or a typed place and Confirm or Dispute (never Mark resolved), forms lock the page behind them, and a rate limited vote shows the wait in the vote error everywhere (D-15, D-16, D-12 votes, P4-A15, P4-A26, P4-A27, U8, U10) | 6 | 04-10, 04-17, 04-18 | ROBUST-01 |
| 04-20 | A signed in person can flag someone else's report once with one of five reasons through `POST /api/reports/{id}/flag`, limited per account, never hiding it, with own report 403, withheld or missing 404, repeat 409, and the feed tells each viewer which reports they already flagged (D-09, D-10, D-11 flag) | 6 | 04-05, 04-07, 04-09, 04-17, 04-18 | ROBUST-09, ROBUST-04 |
| 04-21 | The Low data view is reachable from the login page, the account menu (which now scrolls inside itself and adds Disclaimer and Policy) and the remembered choice; the Activity back link follows the device default (D-17, D-23 links, P4-A11, P4-A18 login, P4-A21) | 6 | 04-16, 04-18 | ROBUST-01, ROBUST-10 |
| 04-22 | A report posted with no signal, a timeout or a 502, 503 or 504 is saved on the device bound to its account, shown in a "Waiting to send" strip on the main page and the Low data view, retried on `online`, load, a timer and Send now, never expires, and a 429 offers "Send after the wait"; a created report with `held` true (201 or 200 replay, form or queue flush) shows the 6 second held toast instead of "Report posted." or "Report sent."; and `?resend=<request_id>` on the main page or Low data view opens that page's form prefilled ("Not sent. {REASON}") only for a saved row of the same account (D-02, D-18, D-19, D-20, D-12 serious line, P4-A12, P4-A13, T-04-14) | 7 | 04-13, 04-15, 04-17, 04-19 | ROBUST-02 |
| 04-23 | A moderator on the server list can read the three queues, find any report by id, Keep or Remove it with an audit row, while everyone else gets a 404 byte equal to an unknown route on every method (D-08, R-03 Remove authority, correction 2) | 7 | 04-05, 04-07, 04-14, 04-20 | ROBUST-09 |
| 04-24 | A reader can tap "Flag this report" in the opened report (the popup, in every Phase 3.1 tab) and on a Low data row, choose a reason in a locked dialog and see "Flagged", without the tap ever reaching a row or the map (a main page list row holds no control since Phase 3.1, P4-A7 amended), and Escape closes the dialog first (D-09, P4-A5, P4-A6, P4-A7) | 8 | 04-19, 04-20, 04-22 | ROBUST-09 |
| 04-25 | The Activity page shows the author's own waiting reports with Discard and an Edit and resend link to the page that has a form (`/low?resend=ID` when this device's default is the Low data view, else `/?resend=ID`), and an optional Node test proves flush order, never discard and wrong account never sent; it only builds the link, never the `resend` landing (D-19, D-20, orchestrator decision 6) | 8 | 04-21, 04-22 | ROBUST-02 |
| 04-26 | A moderator works the queues on a phone: three lists with real counts, scores and reasons in words, Find a report, Keep, a confirmed Remove, Refresh lists, all text through `setText` (D-08, P4-A17 remove, open item 12) | 8 | 04-23 | ROBUST-09 |
| 04-27 | The moderation design, launch checklist and earlier contracts are documented, generated code matches its sources, Phase 4 files pass the bare label and banned word gates, the finished Low data view is proven under its measured byte budget, the owner's recorded fixture scores are committed, and the owner walks the V-24 manual script and the four human checks on a fresh disposable database (D-07 docs, D-14 byte budget, D-22 human check, D-24 checklist, Docs to update) | 9 | 04-01 to 04-26, 04-28, 04-29 | ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-08, ROBUST-09, ROBUST-10 |
| 04-28 | A five language fixture of 150 to 300 realistic genuine reports (en, hi, te, ta, hinglish; urgent wording, all capitals, addresses, phone numbers) plus abusive examples runs offline through `Decide` and the gibberish rule, and once scores are recorded proves no genuine report rejected, no serious report withheld and the abusive set rejected (D-07) | 2 | 04-02 | ROBUST-03 |
| 04-29 | A held or removed report never counts for or against its author's reporter rating: the Phase 3 history read carries `moderation_state`, `ClassifyOutcome` is neutral when `ReportMeta.Withheld()`, and the scorer drops such rows as neutral before it caps at 30 judged reports (R-02, R-01; the rating side of D-02 held and R-03 removed) | 2 | 04-01, 04-05 | ROBUST-03 |

## MVP framing

The phase is seven vertical slices. Each plan is one step through a slice and ends at something a real
person or a real end to end test can do or prove that it could not before. Pure plans (04-01, 04-02,
04-03, 04-04) are scheduled first because nothing blocks them, and each ends at a provable guarantee (the
Phase 3 precedent of 03-03 and 03-06). 04-05 is a store plan, not a bare schema plan: like 03-06 it ends at
store level guarantees a test proves (one row per retried request, account scoped replay, single claimant,
unique flag). 04-10 is a pure refactor plan by design (RESEARCH: the riskiest refactor merges green before
Low data uses it).

| Slice | Plans in order | What becomes possible after the slice |
|-------|----------------|---------------------------------------|
| S1 "says it is unofficial" | 04-06, 04-08, 04-12 | The bar on every page (04-06), pages that fit under it (04-08), public Disclaimer and Policy pages (04-12) |
| S2 "abuse screened, held state" | 04-01, 04-02, 04-28, 04-04, 04-05, 04-07, 04-29, 04-11, 04-14, 04-16 | Held and removed never listable (04-01, 04-07) and neutral for the rating (04-29), the fixture proves the policy (04-28), the text check at submit (04-11), late verdicts (04-14), the author sees why (04-16) |
| S3 "limits" | 04-03, 04-09, 04-15 | Votes throttled (04-09, WR-01 closed), then posting with two allowances (04-15) |
| S4 "weak network main page" | 04-13, 04-17 | The list survives a failed poll and the slow notice points to Low data (04-13); retries never double insert and late reports say when they were written (04-17) |
| S5 "Low data view" | 04-10, 04-18, 04-19, 04-21 | Shared form (04-10), the page reads (04-18), posts and votes (04-19), and is reachable from every way in (04-21) |
| S6 "offline queue" | 04-22, 04-25 | Saved reports on the main page and Low data view (04-22), and on Activity (04-25) |
| S7 "flag and moderate" | 04-20, 04-23, 04-24, 04-26 | Flag API (04-20), moderator API (04-23), the Flag control (04-24), the moderator screen (04-26) |
| Closure | 04-27 | Docs, regeneration, final gates, owner UAT |

**First user visible increment:** 04-06 plus 04-08 (end of wave 2). Every page shows "Unofficial, not a
government service. Emergency? Call 112 (older numbers 100, 101, 108 may also work). Disclaimer". The bar
ships in 04-06 (wave 1) and fits from 04-08 (wave 2); until then the fixed account trigger (auth.css
`.app-header`) overlaps the bar on the map and Activity pages, and the map, Activity, login and verify pages
(100dvh) overflow by the bar height. The 04-06 human check covers only the bar text, link and place; the 04-08
human check (run in 04-27) confirms at 320x568 and 1280x800 that the trigger sits 16px below the bar and the map
and login pages do not scroll at default text size. **First blocker closed:** 04-09
(wave 2) closes STATE.md WR-01 (vote routes had no throttle).

**Why the RESEARCH build order was regrouped.** RESEARCH orders by layer; MVP mode wants every plan to end at
something observable, so the foundation is split along its guarantees (04-01 rung, 04-05 store, 04-07 "held is
invisible" end to end), and the extraction (04-10) merges two waves before Low data posting (04-19). The hot files
(`router.go`, `main.go`, `handlers/reports.go`, `app.js`, `index.html.tmpl`) allow one editor per wave, which sets
the 9 waves. **Increment per plan:** each Objective states what can be done or proven after the plan; plan
authors copy it into `## Phase Goal` as the plan's own step.

## Sizing

Thresholds: target 2 to 3 tasks and 5 to 8 files. 4 tasks or 10 files is a warning; 5 tasks or 15 files is a
blocker. Generated files (sqlc, swag: 3 files per swag run) and small SVG icons count. No plan reaches a
blocker, so no phase split is recommended.

| Plan | Tasks | Files | Context weight | Check |
|------|-------|-------|----------------|-------|
| 04-01 | 2 | 3 | light, TDD (`type: tdd`) | ok |
| 04-02 | 3 | 3 | medium, TDD for `Decide` | ok |
| 04-03 | 2 | 2 | medium logic, TDD (`type: tdd`) | ok |
| 04-04 | 3 | 6 | medium | ok |
| 04-05 | 3 | 11 | medium heavy (4 generated files) | warning, accepted: one plan owns all Phase 4 SQL; the Phase 3 history query edit sits in 04-29 (w2) so this warning does not grow |
| 04-06 | 3 | 13 | medium (12 one line template and wiring edits) | warning, accepted: the four page templates and their two view model files (page.go, auth.go) must land together or a page 500s (Pitfall 11) |
| 04-07 | 3 | 9 | medium | ok |
| 04-08 | 3 | 9 | medium | ok |
| 04-09 | 2 | 10 | medium | warning, accepted: Task 1 touches 8 files because the MVP slice needs the copy (`service.VoteLimitMessage`, `VoteChangeLimitMessage`) to land with the throttle and no message literal may live under `internal/api`; the tenth file is one `RateLimits` line in the `trust_e2e_test.go` harness |
| 04-10 | 2 | 9 | heavy (848 line move) | ok |
| 04-11 | 3 | 11 | heavy | warning, accepted: 3 of the 11 are generated swagger files, and `feed_test.go` gains two interface only stubs because the `Querier` method set changes |
| 04-12 | 3 | 10 | medium (copy drafting) | warning, accepted: the render and e2e tests need their own file in package `handlers_test` |
| 04-13 | 3 | 8 | medium | ok |
| 04-14 | 2 | 3 | medium | ok |
| 04-15 | 3 | 9 | medium (3 generated) | ok |
| 04-16 | 2 | 8 | medium | ok |
| 04-17 | 2 | 12 | medium (3 generated) | warning, accepted: 3 of the 12 are generated swagger files, the client Written line is three small edits, the `request_id` copy test is its own small `package service_test` file, and `feed_test.go` gains one interface only stub (`ReportByRequest`) |
| 04-18 | 3 | 11 | heavy | warning, accepted: the route, the page and its contract tests cannot be split without a page that has no test |
| 04-19 | 3 | 13 | heavy | warning, accepted: the P4-A5 guard in Phase 3's `trust.js` and its test, and a one line anchor re-point in two Phase 3 vote tests that must ship with the signature change (the form partial `report_form.html.tmpl` stays as 04-10 left it) |
| 04-20 | 3 | 10 | medium (3 generated) | warning, accepted: 3 of the 10 are generated swagger files |
| 04-21 | 3 | 9 | medium | ok |
| 04-22 | 3 | 12 | heavy | warning, accepted: the main page and Low data mounts are two lines each, and `modal.js` gains one guard line |
| 04-23 | 3 | 9 | heavy | ok |
| 04-24 | 3 | 8 | medium heavy | ok (feed.js left the plan when Phase 3.1 took controls off the list row) |
| 04-25 | 3 | 7 | medium (third task optional: the Node test) | ok (the seventh file is the new `activity_queue_e2e_test.go`) |
| 04-26 | 2 | 4 | medium | ok |
| 04-27 | 3 | 12 | light, byte budget and owner UAT | warning, accepted: the three fixture files change only when the owner records, the budget must follow every wave 8 edit, and `docs/TRUST-MODEL.md` gains two R-02 sentences |
| 04-28 | 3 | 3 | heavy authoring; task 1 genuine en and hinglish, task 2 genuine hi, te, ta, task 3 `abusive.json` and the test | ok |
| 04-29 | 2 | 7 | medium (1 generated file) | ok |

Total 29 plans in 9 waves, above the brief's 15 to 20: five added scope groups (vote throttle, moderator screen,
Flag control, legal pages, Low data posting and voting) and one editor per wave on the hot files; merging would
breach the 15 file blocker. Nothing is cut: the CONTEXT cut order is only a suggestion and is not applied.

## Sources per plan

Cite by heading, never by line number. **Every plan** reads this outline's "Cross-plan contracts" and
"Coverage map", `./.claude/CLAUDE.md`, the brief, RESEARCH and PATTERNS "Corrections after the code check"
(with "UI contract code facts"), PATTERNS "Shared Patterns" and "Landmines found while mapping", and
VALIDATION "Test Infrastructure". **Every UI plan** (04-06, 04-08, 04-10, 04-12, 04-13, 04-16, 04-18, 04-19,
04-21, 04-22, 04-24, 04-25, 04-26) also loads `Skill("sketch-findings-pinalert")` and reads UI-SPEC "Rules
this contract must obey", "Vocabulary", "Typography", "Color" and "Hand Off Notes" ("CSS rules to keep",
"Script order"). R = RESEARCH, P = PATTERNS, U = UI-SPEC.

| Plan | CONTEXT | V rows | RESEARCH, PATTERNS, UI-SPEC sections |
|------|---------|--------|--------------------------------------|
| 04-01 | D-02, R-01 to R-04 | V-06 | R "Moderation state model" items 3 to 5, collisions rows 5, 6; P "`internal/service/visibility.go` (edit: moderation rung)", "Closed enum plus drift guard"; Phase 3 03-03 `ClassifyOutcome` |
| 04-02 | D-03 to D-06 (with the D-05 owner note of 2026-10-08: the gibberish message), R-06, Discretion (thresholds, snapshot), OpenAI facts | V-01, V-22 | R "Area A" ("Provider facts", "Landmines" 5, 6, 9), Assumptions A3, A9; P "`internal/service/screening.go`"; U "Rejected text messages" |
| 04-03 | D-10, D-11, Discretion (limiter store) | V-08 | R "Area C" "Design" items 1 and 4, "Landmines", "Code Examples" "Reserve and refund", Pitfalls 1, 2, A4, A5; P "`internal/ratelimit/perkey.go`" |
| 04-04 | D-01, D-07, Discretion (provider client, account setup risk) | V-03, V-02 | R "Area A" "Integration points", "Provider facts", "Landmines" 1, 2, 4, A1, A2; P "`internal/moderation/loader.go`", "`internal/moderation/client.go`"; the fixture entry shape in the identifiers row (recorder tests use a temp file in that shape) |
| 04-05 | R-01, R-05, R-10, D-04, D-08, D-09, D-19 | V-23, V-12 | R "Moderation state model" items 1, 2, 6, 7, "Area A" "Landmines" 7, 8, "Area B" "Queue queries", "Area E" "Server contract", "Code Examples" idempotent insert, collisions 1, 2, 18; P "Migrations and queries", "Test safety"; Phase 3 03-01, 03-06 |
| 04-06 | D-21, D-22, D-23 (bar link), D-24 (setting), R-07, R-08 | V-17 | R "Area F" (bar, one template set, numbers), Pitfall 11; P "Templates"; U "Surface: Disclaimer Bar" whole, P4-A1 (markup), "Element table" bar rows |
| 04-07 | D-02, D-19, R-01, R-02, R-05 | V-06, V-20, V-04 | R "Moderation state model" items 6 to 8, collisions 3, 4, 5, 7, Pitfalls 3, 6; P "`internal/service/report.go` (edit)", "`internal/service/trust.go` (edit ...)", "Go handler and e2e test patterns"; Phase 3 03-01, 03-03, 03-05, 03-12 |
| 04-08 | D-05, D-21 | V-22, V-24 | R "Area B" "Landmines" (`stripJSComments`), "Area F" Geometry row; P "CSS", text sink rule; U P4-A1 to A4, A16, A17, A20, A25, A28, "Spacing Scale", "Component Geometry", "Color", "Motion"; Phase 3 03-17, 03-19 |
| 04-09 | D-10 to D-12, Discretion (store, backstop), STATE WR-01 | V-10, V-11, V-22 | R "Area C" whole, Pitfalls 2, 10, "Security Domain"; P "`ratelimit_gate.go`", "`router.go` (edit)", "`cmd/server/main.go` (edit)"; U "Rate limit messages", "Wait format" |
| 04-10 | D-15, D-05 owner note of 2026-10-08 (the hint), code_context landmines | V-21 | R "Area D" "The form extraction is the riskiest refactor"; P "`modal.js` and `report-form.js`"; U "Posting a report without a map" (shared form paragraph), P4-A23, "Tests that change" form items |
| 04-11 | D-01 to D-04, D-06, R-03, R-06 | V-04, V-07 | R "Area A" "Integration points", "Landmines" 1, 3, 4, 7, "Code Examples" handler order, Pitfalls 4, 7; P "`handlers/reports.go` (edit: `SubmitReport`)"; U Submit Outcomes 1, 2 (server side: the `held` field only), 3, 8 |
| 04-12 | D-09 (police plain text), D-20 (Policy storage item), D-21, D-22 to D-24, R-07, R-08, 1.1 D-05 | V-17, V-18 | R "Area F", Open Question 5, "Security Domain"; P "Templates", router exempt block; U "Surface: Disclaimer Page and Policy Page" whole, "Copywriting Contract" legal, P4-A18, "The 112 link"; Phase 3 `trustcopy.go`, 3.1 Past window |
| 04-13 | D-17, D-20 | V-16, V-24 | R "Area D" "Other landmines", collisions 17; P "`app.js` (edit)", feed.js and slow-notice.js bullets; U P4-A12, A19, A24, "3. The slow main page message", "The stale list on the main page", "The notice card", "Announcements", open items 9, 10, 26 |
| 04-14 | D-01 (background retry), D-02, D-04 (late borderline), R-03, Discretion (retries) | V-05 | R "Area A" "Landmines" 7, 8; P "`internal/service/moderation.go`, `modworker.go`" |
| 04-15 | D-10 to D-12, R-04 | V-09, V-11 | R "Area C" "Design" items 2 to 5, handler order, Pitfall 1; P "`ratelimit_gate.go`", "`handlers/reports.go`"; U outcome 4, "Rate limit messages", "The 112 link", "Where the serious sentence comes from" |
| 04-16 | D-02, D-08, R-01, R-02, R-03 (author told) | V-06, V-24 | R "Moderation state model" items 3, 8, collisions 7, 12, 15; P visibility.js and activity.js bullets; U "Activity page: held and removed rows", P4-A8 to A10; Phase 3 03-15, 03-16 |
| 04-17 | D-19, R-05 | V-12, V-22 (`request_id` copy) | R "Area E" "Server contract", idempotent insert example, "Code Examples" handler order, "Area C" "Design" item 4, Pitfall 9, A10; P "`handlers/reports.go`"; U P4-A8, "Written 4 hours ago", open item 17 |
| 04-18 | D-13, D-14, D-16, D-17, D-21, R-08, R-09 | V-15, V-16 | R "Area D" "Integration points", "Size budget (D-14)", "Other landmines", Open Question 2, Pitfall 15; P gate.go paragraph, "Templates", low.js bullet; U "Surface: Low Data View" (budget, structure, list, position, widths, focus), "4. The remembered choice", P4-A14, P4-A22, open items 1, 4, 25 |
| 04-19 | D-12, D-15, D-16 | V-21 | R "Area D" "Other landmines", U8, U10; P votes.js bullet; U "Posting a report without a map", "The location block", "Queue, offline and the form", P4-A5, A15, A26, A27, "Interaction Contract" 5, 6, 10 |
| 04-20 | D-09 to D-11 | V-20 | R "Area B" (Flag, Viewer marker, CSRF); P "`handlers/votes.go` (edit ...)"; U "The dialog", "Dialog states" |
| 04-21 | D-17, D-23 | V-15, V-24 | R "Area D" Redirects row, U4; P "Templates"; U "Surface: Ways In" sections 1, 2, 4, P4-A11, P4-A21, "Copywriting Contract" ways in |
| 04-22 | D-12, D-18 to D-20, Discretion (IndexedDB) | V-13 | R "Area E" "Client", Pitfall 14; P "`queue.js`", "`app.js` (edit)"; U "Surface: Offline Queue" (saved, strip, rows incl. Edit and resend, announcements, wait format), Submit Outcomes 2, 4 to 6 and the form line markup, Copywriting Held toast, P4-A12, P4-A13, open items 6, 16, 19 |
| 04-23 | D-08, D-21, R-03, R-08 (bar on `moderate.html.tmpl`) | V-19 | R "Area B" whole, "Moderator gate with its own lookup", Pitfall 5, "Security Domain"; P "`internal/api/moderator.go`", "`internal/service/moderation.go`"; U "The three lists", "Find any report", "The moderation row" |
| 04-24 | D-09 | V-20, V-24 | R collisions 11; P flag.js bullet; U "Surface: Flag Control and Dialog" whole, P4-A5 to A7, `.flag-control` and dialog geometry, "Cross phase hand offs" 3.1 bullet; sketch `references/popup-structure.md` |
| 04-25 | D-19, D-20 | V-13, V-14 | R "Area E" "Client", Open Question 6; U "The strip" (Activity), "Queued rows" (Edit and resend), open item 7 |
| 04-26 | D-08 | V-19, V-22, V-24 | U "Surface: Moderator Screen" whole, "Copywriting Contract" moderator, P4-A17, open items 12, 20 |
| 04-27 | D-07, D-14 (byte budget test, documented figure), D-22 to D-24, Specifics "Checks the owner will expect" | V-24, V-16, V-02 (recording), Manual-Only table | R collisions 19, "Environment Availability", "Size budget (D-14)"; U "Manual checks (V-42 additions)", "Docs to update", "New tests and checks", "Size budget as a UI constraint", P4-A14 |
| 04-28 | D-07 | V-02 | R "Area A" "D-07 fixture", Assumptions A3, A9; P "`internal/service/screening.go`" (fixture paragraph); this outline's fixture identifiers row |
| 04-29 | R-02, R-01 | V-06 | R "Moderation state model" item 4, collisions row 5 (superseded here); Phase 3 03-03 `ClassifyOutcome`, 03-06 `ReporterHistoryReports` and its store tests, 03-10 `reporterHistoryMeta`, `ReporterSignals` |

## Cross-plan contracts (pinned here so per-plan calls do not drift)

### Standing rules for every plan

- [ ] **Disposable database, one per plan:** `p4_04NN` (for 04-07 that is `p4_0407`). Canonical form: `dropdb --if-exists p4_04NN; createdb p4_04NN && DATABASE_URL="postgres://localhost:5432/p4_04NN?sslmode=disable" go test <pkgs> -run '<regex>' -count=1 -p 1; rc=$?; dropdb --if-exists p4_04NN; exit $rc`. Keep the exit code in `rc` (zsh makes `status` read only; the Phase 3 review found the `status` form aborts before `dropdb`). Add `-race` for limiter and idempotency tests. NEVER `pinalert_test` or `pinalert`.
- [ ] **Quick run:** `env -u DATABASE_URL go test ./... -count=1` (or a package list). The `env -u` prefix is mandatory.
- [ ] **`-run` hygiene (exact form, works in zsh and bash):** join names with a plain `|`, add `-v`, capture and check: `out=$(go test <pkgs> -run '<regex>' -count=1 -v 2>&1); rc=$?; printf '%s\n' "$out"; printf '%s\n' "$out" | grep -q '^--- PASS' || rc=1; exit $rc`. In the database form this goes between `createdb ... &&` and `dropdb`: `dropdb --if-exists p4_04NN; createdb p4_04NN && out=$(DATABASE_URL="postgres://localhost:5432/p4_04NN?sslmode=disable" go test <pkgs> -run '<regex>' -count=1 -p 1 -v 2>&1); rc=$?; printf '%s\n' "$out"; printf '%s\n' "$out" | grep -q '^--- PASS' || rc=1; dropdb --if-exists p4_04NN; exit $rc`. Never pipe `go test` itself into `grep` (the pipeline status hides its exit code).
- [ ] **Running and UAT:** `PORT=8792`, `BASE_URL=http://127.0.0.1:8792`, `ENV=development`, a disposable database. Never ports 8080 or 8090. No server is started by any automated verify.
- [ ] **No dashes** (U+2014, U+2013) in prose, docs, Go strings, templates, and every NEW JS or CSS file including comments. Existing files keep their existing comment dashes.
- [ ] **No new Go module, no npm package, no JS build step, no image or screenshot saved in the repo.**
- [ ] **Regeneration:** `make sqlc` runs only in 04-05 (wave 1, every Phase 4 query) and 04-29 (wave 2, the Phase 3 reporter history query only). `make swag` runs once per wave: 04-11 (w3), 04-15 (w4), 04-17 (w5), 04-20 (w6). Each commits `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`. Never edit generated files by hand. CI has neither tool.
- [ ] **Verify hygiene:** no `|| true` or `2>/dev/null || echo` feeding a comparison; every grep count measured first with its provenance stated; negative greps strip comments (`grep -v '^\s*//'`) or are region scoped; no RESEARCH or UI-SPEC line numbers in a verify command; gofmt gates name only the plan's own files.
- [ ] **Same wave RED windows (every wave):** plans keep their RED commits. A RED step adds compiling stubs first, so its package always builds and the RED state is failing assertions only (for example 04-01, 04-04, 04-06, 04-07, 04-09, 04-11 and 04-12); the one exception is 04-20, whose RED leaves the `internal/service` test package uncompiled until its GREEN, which follows at once. A plan whose verify runs a whole package (no `-run`) may meet a same wave sibling's new tests failing between that sibling's RED and GREEN commits (wave 1: 04-01 and 04-02 in `internal/service`, plus 04-03, 04-04 and 04-06 in their own packages; wave 2: 04-07, 04-09, 04-28 and 04-29; wave 3: 04-11 and 04-12; wave 6: 04-19, 04-20 and 04-21; wave 8: 04-24, 04-25 and 04-26 in package `web`). A failure that names only the sibling's own new tests, or a build failure confined to a package only that sibling edits, is not this plan's defect: wait for the sibling's GREEN commit and rerun once; never edit the sibling's files. RED window tests are named in the plans' wave notes (04-01, 04-02, 04-07, 04-09, 04-12, 04-19, 04-20, 04-21; wave 8 in 04-24, 04-25 and 04-26), and 04-05, 04-06, 04-10, 04-13 and 04-14 carry the wait and rerun rule; 04-28 keeps its RED uncommitted behind a build constraint. In a shared working tree every commit names its paths (`git commit -m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- <paths>`), never a bare commit.
- [ ] **Human verify mode is end of phase:** no `checkpoint:human-verify` tasks; manual steps go in `<verify><human-check>`; the full script runs in 04-27.
- [ ] **Threat model:** every plan carries `<threat_model>` at ASVS level 1, blocking on high, reusing the ids below. No package installs, so no supply chain row.
- [ ] **Naming:** the words "light" and "lite" never appear in a Low data name, file, route, class, string or comment. UI-SPEC Vocabulary words are used verbatim.
- [ ] **Cross-phase prerequisites:** first task re-greps each Phase 3, 3.1 or 3.2 symbol the plan lists and stops with a report if one is missing or renamed. Never invent it.
- [ ] **Decision citations:** every D, R and P4-A id the plan implements is cited at the end of a `must_haves.truths` entry, e.g. "(D-02, R-01)".

### Go identifiers shared across plans

| Identifier | File | Created by | Used by |
|-----------|------|-----------|---------|
| `type ModerationState string`; `ModerationClear = "clear"`, `ModerationPending = "pending"`, `ModerationRemoved = "removed"`; `ModerationStates`; `Valid()` (zero value `""` is treated as clear by `Resolve`) | `internal/service/visibility.go` | 04-01 | 04-07, 04-11, 04-14, 04-16, 04-23 |
| `VisibilityWithheld Visibility = "withheld"`, appended to `Visibilities` so `Valid()` accepts it | visibility.go | 04-01 | 04-07, 04-16 |
| `ReasonRemovedAfterReview ResolveReason = "removed_after_review"`, `ReasonAwaitingTextCheck ResolveReason = "awaiting_text_check"`, appended to `ResolveReasons` | visibility.go | 04-01 | 04-07, 04-16, 04-23 |
| `ReportMeta.Moderation ModerationState` (`Resolve` treats the zero value as clear; R-02's rating neutrality is `ClassifyOutcome`'s job in 04-29, superseding RESEARCH "Moderation state model" item 4, collisions row 5 and PATTERNS where they call the zero value alone R-02) | visibility.go | 04-01 | all `Resolve` callers |
| `func (m ReportMeta) Serious() bool { return m.criticalBypass() }` (`criticalBypass` keeps its name); `func (m ReportMeta) Withheld() bool` (true exactly when `removed`, or `pending` and not `Serious()`; `""` and `clear` give false), used by `Resolve`'s two top rungs | visibility.go | 04-01 | 04-11, 04-15, 04-22 (server side flag); `Withheld` 04-29 |
| `ClassifyOutcome` (03-03) gains a first step `if meta.Withheld() { return OutcomeNeutral }`; its one neutral meta `Resolve(` line stays byte for byte (03-03's one call and one `meta.criticalBypass()` guards hold); `reporterHistoryMeta` (03-10) copies `row.ModerationState`; `ReporterSignals` keeps one `ClassifyOutcome(` call | `internal/service/reliability.go`, `reporter.go` (Phase 3) | 04-29 | |
| `Resolve` rung order: removed gives (`withheld`, `removed_after_review`); then pending and not `Serious()` gives (`withheld`, `awaiting_text_check`); then the existing Retracted, bypass, Hidden, Provisional, Live. Pending and serious falls through to the bypass (Live) | visibility.go | 04-01 | |
| `type Verdict string`: `VerdictPass = "pass"`, `VerdictBorderline = "borderline"`, `VerdictReject = "reject"` | `internal/service/screening.go` | 04-02 | 04-11, 04-14 |
| `type RejectReason string`: `RejectHateful = "hateful"`, `RejectHarassing = "harassing"`, `RejectSexual = "sexual"`, `RejectGibberish = "gibberish"`; `RejectReasons` | screening.go | 04-02 | 04-11 |
| `type ScreenDecision struct{ Verdict Verdict; Reason RejectReason }`; `func Decide(scores map[string]float64, serious bool) ScreenDecision` (pure; `serious` never changes the result, D-03; the provider `flagged` boolean is never read) | screening.go | 04-02 | 04-11, 04-14 |
| `func IsGibberish(text string) bool` (runes; a letter is `unicode.IsLetter(r) \|\| unicode.IsMark(r)`; run of one letter rune (only letters form a run, so repeated punctuation, emoji, digits and spaces never do; see "Open questions resolved during planning") at least `GibberishRunLength = 10`, or fewer than two tokens of two or more letters) | screening.go | 04-02 | 04-11 |
| Threshold constants `RejectScoreThreshold = 0.85`, `BorderlineScoreThreshold = 0.40` (starting values, tuned only on the recorded fixture); the reject category map: `hate`, `hate/threatening` to hateful; `harassment`, `harassment/threatening` to harassing; `sexual`, `sexual/minors` to sexual | screening.go | 04-02 | |
| `func RejectMessage(r RejectReason) string` (the four exact messages; the gibberish one is the owner's wording of 2026-10-08, see "Open questions resolved during planning") | screening.go | 04-02 | 04-11, 04-22 (as `{REASON}`) |
| Fixture files `internal/service/testdata/screening/genuine.json` and `abusive.json`; entry `{ "id", "text", "language" (one of `en`, `hi`, `te`, `ta`, `hinglish`), "serious" bool, "expect" (`publish` or `reject`), "model" string, "scores" object }`; empty `scores` means not recorded. File layout (the 04-04 recorder writes it, 04-28 authors it): each of genuine.json and abusive.json is a top level JSON array of entries; keys appear in the order id, text, language, serious, expect, model, scores, with no other keys; two space indentation; a trailing newline; an unrecorded entry is written as `"model": ""` and `"scores": {}`; the characters ampersand, less than and greater than and ordinary non ASCII text are literal UTF-8 (`SetEscapeHTML(false)`), while U+2028, U+2029 and control characters stay `\u` escapes because Go encoding/json always escapes them. `TestScreeningFixture`: none recorded is `t.Skip` naming the count and `MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord` (quick run stays green); some recorded is `t.Fatalf` listing the missing ids; all recorded asserts every `model` is `omni-moderation-2024-09-26`, then the V-02 checks. `TestScreeningFixtureShape` always runs (150 to 300 genuine, five languages, valid `expect`, unique ids) | testdata | 04-28 | 04-04 recorder writes `model` and `scores`; 04-27 commits them |
| `moderation.Result{ Model string; Scores map[string]float64 }`; `moderation.CallError{ Status int; Transient bool; Quota bool; RetryAfter time.Duration }` with `Error()`; `type moderation.Screener interface{ Screen(ctx context.Context, text string) (Result, error) }` | `internal/moderation/client.go` | 04-04 | 04-11, 04-14 |
| `moderation.NewClient(apiKey, model string) *Client`, unexported `newClientWithBaseURL` test seam; constants `moderationURL = "https://api.openai.com/v1/moderations"`, `DefaultModel = "omni-moderation-2024-09-26"`, `inputRuneCap = 1000`; process wide budget of 250 requests a minute and 10,000 estimated tokens a minute (rune count) that refuses with `CallError{Transient: true}` without a network call | client.go | 04-04 | |
| `moderation.LoadScreener(env string) (Screener, error)`: `OPENAI_API_KEY` set gives the client (model from `MODERATION_MODEL`, default `DefaultModel`); unset in `development` gives `PassScreener` with a WARNING log; otherwise an error naming the variable | `internal/moderation/loader.go` | 04-04 | 04-11 (`main.go`) |
| Recorder `cmd/modrecord/main.go`: refuses unless `MODERATION_LIVE=1` and `OPENAI_API_KEY` are set; fills `model` and `scores` in both fixture files; throttled under 10,000 tokens a minute; `TestRecorderRefusesWithoutLiveFlag` in `cmd/modrecord/main_test.go` | cmd | 04-04 | owner (04-27 human check) |
| `type TextScreener interface{ Screen(ctx context.Context, text string) (moderation.Result, error) }` | `internal/service/textcheck.go` | 04-11 | 04-14 |
| `type TextCheckOutcome string`: `TextClear = "clear"`, `TextBorderline = "borderline"`, `TextRejected = "rejected"`, `TextUnfinished = "unfinished"`; `type TextCheck struct{ Outcome TextCheckOutcome; Reason RejectReason; Scores map[string]float64; Model string; RetryAfter time.Duration }`; `func CheckText(ctx context.Context, s TextScreener, text string) TextCheck` (prefilter gibberish first, then ONE provider call inside `InlineScreenTimeout = 3 * time.Second`; a nil screener is clear with no network; no report vocabulary, D-06) | textcheck.go | 04-11 | 04-14 (worker uses `Decide` the same way) |
| `func WithScreener(s TextScreener) ReportOption` (Phase 3's `ReportOption` pattern) | `internal/service/report.go` | 04-11 | `main.go`, e2e tweaks |
| `ValidateSubmitInput` counts the description with `utf8.RuneCountInString` (10 to 1000) | report.go | 04-11 | |
| `func (s *ReportService) SubmitChecked(ctx context.Context, sessionID string, accountID int64, in SubmitInput) (SubmitResult, error)`; `type CreatedReport struct{ ID int64; Category Category; Severity Severity; Description string; Latitude, Longitude float64; Geohash string; ShelterCapacityStatus, ShelterHeadcount (types as in `ReportView`); CreatedAt, ExpiresAt time.Time; Moderation ModerationState }` (the `InsertReportIdempotent` row and the `ReportByRequest` row both map into it); `type SubmitResult struct{ Report CreatedReport; Held bool; Replayed bool; Check TextCheck }`; sentinel `var ErrTextRejected` wrapped in `type TextRejectedError struct{ Reason RejectReason }`. `Submit` is deleted (one caller, correction 4) and the handler calls `SubmitChecked` | report.go | 04-11 (screening), 04-17 (request id, written at, replay) | handlers |
| `SubmitInput.RequestID string`, `SubmitInput.WrittenAt *time.Time`; `ValidateSubmitInput` checks a non empty `RequestID` against `^[A-Za-z0-9-]{16,64}$` (`""` is absent) and returns `ValidationError` field `request_id` with the exported constant `RequestIDInvalidMessage` (report.go; a plain sentence in the closed character set, see "Server authored copy"), so the 400 comes before the replay lookup; `func (s *ReportService) FindReplay(ctx context.Context, accountID int64, requestID string) (SubmitResult, bool, error)` (account scoped, called by the handler before any reservation). Handler (04-17): `SubmitReportRequest.RequestID *string` and `WrittenAt *string` (json `...,omitempty`; swag `format:"date-time"`); the handler only copies RequestID into `SubmitInput.RequestID` (nil gives `""`) and never checks it itself; nil or `""` WrittenAt is absent, else `time.Parse(time.RFC3339, s)` (accepts JS `toISOString`), a parse error gives nil, never an error; then `service.ClampWrittenAt(parsed, now)` | report.go, handlers/reports.go | 04-17 | handlers |
| Constants `WrittenLabelThreshold = 30 * time.Minute`, `WrittenAtMaxAge = 14 * 24 * time.Hour`; `func ClampWrittenAt(w *time.Time, now time.Time) *time.Time` (nil gives nil; future clamps to now; older than `WrittenAtMaxAge` clamps to now minus it, so a parseable value is never dropped; parsing happens in the handler); the clamped pointer enters `InsertReportIdempotentParams.ClientWrittenAt` through the unexported `writtenAtParam(t *time.Time) pgtype.Timestamptz` (04-17, report.go), the inverse of 04-07's `writtenAtPtr(t pgtype.Timestamptz) *time.Time`: sqlc with pgx/v5 generates every nullable timestamptz, parameter or column, as `pgtype.Timestamptz` (never `*time.Time`; sqlc.yaml overrides only the non null case), so nil is the zero value (NULL); `func WrittenAtForDisplay(createdAt time.Time, written *time.Time) *time.Time` (nil unless the gap exceeds the threshold) | `internal/service/feed.go` (`WrittenLabelThreshold`, `WrittenAtForDisplay`) and `internal/service/report.go` (`WrittenAtMaxAge`, `ClampWrittenAt`) | 04-07 (feed.go part), 04-17 (report.go part) | 04-07 read paths, 04-17 |
| `ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ReportIds []int64; ViewerAccountID int64})` on `Querier` (report.go) and `AuthQuerier` (auth.go) and their fakes `fakeFeedQuerier`, `fakeAuthQuerier` only; called once per non empty page after the existing batched reads | service | 04-07 | |
| `ReportView.WrittenAt *time.Time`, `ReportView.ViewerFlagged bool`; `ActivityReport.Moderation ModerationState`, `ActivityReport.WrittenAt *time.Time`; `ActivityForAccount` skips `BuildTrust` exactly when `ReportMeta.Withheld()` (removed, or pending and not serious; a pending serious report was public and keeps its trust block, D-02, P4-A10) | `feed.go`, `auth.go` | 04-07 | 04-16, 04-17, 04-20 |
| `CastVote` new step right after the context read: `rc.ModerationState` removed, or pending and not serious, gives `ErrReportNotFound`; `ReportMeta.Moderation` passed into `Resolve` | `trust.go` | 04-07 | |
| `ratelimit.Config{ Every time.Duration; Burst int; IdleWindow time.Duration; Now func() time.Time }`; `func NewPerKey(c Config) *PerKey`; `type Decision struct{ OK bool; RetryAfter time.Duration; Cancel func() }`; `func (p *PerKey) Reserve(key string) Decision` (one captured `now`; `CancelAt(now)` in `sync.Once`; `RetryAfter` rounded up to whole seconds); a zero `IdleWindow` defaults to `Burst * Every`, never less | `internal/ratelimit/perkey.go` | 04-03 | 04-09, 04-15, 04-20 |
| `api.BucketLimit{ Every time.Duration; Burst int }` and `api.RateLimits{ ReportSerious, ReportOther, Vote, VoteChange, Flag, SubmitAttempt, IPBackstop BucketLimit }` with `RateLimitsDefault`; zero field means the default; `Deps.RateLimits RateLimits`, `Deps.ClientIPHeader string`, `Deps.LimiterClock func() time.Time` (nil means `time.Now`) | `internal/api/router.go` | 04-09 | 04-15, 04-20 |
| Budget constants in one block in `cmd/server/main.go`: `reportSeriousBurst = 5`, `reportSeriousEvery = 5 * time.Minute`, `reportOtherBurst = 8`, `reportOtherEvery = 15 * time.Minute`, `voteBurst = 20`, `voteEvery = 15 * time.Second`, `voteChangeEvery = 30 * time.Second` (burst 1), `flagBurst = 10`, `flagEvery = 2 * time.Minute`, `submitAttemptBurst = 20`, `submitAttemptEvery = 6 * time.Second`, `ipBackstopBurst = 60`, `ipBackstopEvery = 20 * time.Second`; env `CLIENT_IP_HEADER` | main.go | 04-09 | |
| Bucket keys: `acct:<id>` for every account bucket; `acct:<id>:rpt:<reportID>:<kind>` for the re-cast cooldown, `<kind>` being `string(service.VoteKind)` (`content` for confirm and dispute, `resolution` for resolve and reopen, never the route name), so confirm then dispute on one report within 30 s answers 429 `rate_vote_change` (D-11); the client IP from `middleware.GetClientIP` for the backstop, skipped when empty or when `CLIENT_IP_HEADER` is unset (never a shared bucket) | handlers/ratelimit_gate.go | 04-09 | 04-15, 04-20 |
| `handlers.RateGate` (holds the PerKey buckets including the IP backstop, the clock, and a `Chrome` copy that router.go passes from `deps.Chrome` when it builds the gate in 04-09, so 04-15 never changes the constructor; the serious 429 reads `Chrome.withDefaults().Numbers.Primary` inside package handlers, so a zero Deps still answers 112; the IP backstop is checked inside each route's own gate step: `VoteMiddleware` (04-09), report handler step (4) (04-15), the flag gate (04-20); the backstop bucket check is never router wide middleware, while the client IP extraction middleware (`ClientIPFromRemoteAddr`, or `ClientIPFromHeader` when `Deps.ClientIPHeader` is set) stays router wide as today, because it fills `GetClientIP`); `func (g *RateGate) VoteMiddleware(kind service.VoteKind) func(http.Handler) http.Handler` (router.go wraps each vote route with `r.With(gate.VoteMiddleware(K))`, K the same `service.VoteKind` it passes to `handlers.CastVote` there; vote bucket never refunded; cooldown refunded on any non 2xx through `middleware.NewWrapResponseWriter`); `func writeRateLimited(w, field, message string, wait time.Duration, serious bool, number string)`; every block logged with bucket, numeric account id and wait only | `internal/api/handlers/ratelimit_gate.go` | 04-09 | 04-15 (report gate), 04-20 (flag gate) |
| `func WaitPhrase(d time.Duration) string` (rounded up: `{N} seconds`, `{N} minutes`, `{N} hours`, singular forms); `ReportLimitMessage(serious bool, wait time.Duration)`, `VoteLimitMessage(wait)`, `VoteChangeLimitMessage(wait)`, `FlagLimitMessage(wait)` | `internal/service/ratecopy.go` | 04-09 | 04-15, 04-20 |
| `handlers.EmergencyNumbers{ Primary string; Older []string; StateHelpline, DistrictHelpline string }` with methods `OlderList() string` ("100, 101, 108"), `HasOlder() bool`, `HasStateLines() bool`, `TelHref() template.URL` (`html/template`'s `template.URL` of "tel:" plus Primary: html/template rewrites a plain string with the tel scheme in an href to `#ZgotmplZ`, so the typed value is what lets a template write `href="{{...TelHref}}"`; safe because the scheme is a fixed literal and `isEmergencyNumber` admits only 2 to 15 ASCII digits; 04-06 pins `DefaultEmergencyNumbers().TelHref()` equal to `template.URL("tel:112")` and a template execute rendering `href="tel:112"`, 04-12 sets `CallHref` from it and asserts the rendered `href="tel:"` plus the primary number); `DefaultEmergencyNumbers()` (112; 100, 101, 108; 1070; 1077); `LoadEmergencyNumbers(lookup func(string) (string, bool)) EmergencyNumbers` reading `EMERGENCY_PRIMARY`, `EMERGENCY_OLDER` (comma list; set but empty means none), `EMERGENCY_STATE_LINES` ("1070,1077"; set but empty drops the row) | `internal/api/handlers/chrome.go` | 04-06 | 04-12, 04-15, 04-22 (through the 429 body) |
| `handlers.Chrome{ Numbers EmergencyNumbers; ContactEmail string; DisclaimerCurrent bool; LowDataPage bool }` embedded as field `Chrome` in `pageViewModel`, `loginGateViewModel`, `verifyOutcomeViewModel`, `profileViewModel` and every new page model; `Deps.Chrome Chrome`; env `CONTACT_EMAIL` (empty means no contact block anywhere). `LowDataPage` is set true only by `LowPage` (04-18) and read by the account header partial (04-21) to show "Open full version" instead of "Low data view"; `DisclaimerCurrent` is set only by the Disclaimer page (04-12). No later plan edits `chrome.go`. Chrome has no AssetVersion (each view model takes it from its `PageConfig` or `AuthConfig`). `Page`, `LoginGate`, `Verify`, `Profile` keep their signatures (`page_test.go` untouched); Chrome rides on new fields `PageConfig.Chrome` and `AuthConfig.Chrome`, which NewRouter fills from `deps.Chrome` on local copies; each handler uses `cfg.Chrome.withDefaults()` (unexported: `Numbers = DefaultEmergencyNumbers()` when `Numbers.Primary == ""`, the `RequestLinkRateLimit` zero value pattern), so any Deps literal or direct call renders the exact D-22 sentence; main.go sets `Deps.Chrome` from `LoadEmergencyNumbers(os.LookupEnv)` and `CONTACT_EMAIL` | chrome.go | 04-06 | every page plan; 04-09 and 04-15 (`RateGate`, the serious 429 number) |
| `handlers.Legal(tmpl *template.Template, cfg AuthConfig, page string)` for `disclaimer` and `policy`, public like `/login`, wired with the router's local `auth` copy; its view model carries `cfg.Chrome.withDefaults()` and `cfg.AssetVersion` (theme.js and legal.css use `?v={{.AssetVersion}}`, so `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` stays green); `const policyLastUpdated` (a real date edited with the text) | `internal/api/handlers/legal.go` | 04-12 | |
| `service.HeldLabel`, `HeldSentence`, `RemovedLabel`, `RemovedSentence`, `RemovedPolicyPointer`, `DisclaimerResolvedEntry` (exact strings below) | `internal/service/moderationcopy.go` | 04-12 | 04-16 |
| `handlers.LowPage(tmpl *template.Template, cfg PageConfig)` (reads `cfg.Chrome.withDefaults()`, sets `LowDataPage = true` on its own copy, and `cfg.AssetVersion`); `const LowDataCookie = "pinalert_low_data"`; `func setLowDataCookie(w, r)`, `func clearLowDataCookie(w, r)` (one year, `SameSite=Lax`, `Secure` on https, `Path=/`, not HttpOnly) | `internal/api/handlers/low.go` | 04-18 | 04-21 (`LoginGate` `low=1`, `low=0`) |
| `type FlagReason string`: `FlagSpam = "spam"`, `FlagAbusive = "abusive"`, `FlagFalse = "false"`, `FlagDangerous = "dangerous"`, `FlagOther = "other"`; `FlagReasons`; `Valid()`; `func FlagReasonLabel(r FlagReason) string` (the one label mapping: Spam or gibberish, Abusive or hateful, False or a hoax, Dangerous or illegal, Something else) | `internal/service/flag.go` | 04-20 | 04-23 (JSON labels), 04-24 (dialog partial) |
| `service.FlagService`, `NewFlagService(q FlagQuerier)`, `Flag(ctx, FlagInput{ReportID, AccountID int64; Reason FlagReason}) error`; sentinels `ErrFlagOwnReport`, `ErrAlreadyFlagged`, existing `ErrReportNotFound` (missing or withheld); messages `FlagReasonMessage` (400), `FlagOwnReportMessage` (403), `FlagNotFoundMessage` (404), `FlagAlreadyMessage` (409). Built once in `api.NewRouter` as `service.NewFlagService(deps.Sessions)`: no `Deps` field, no main.go or harness edit, so every harness gets a non nil service | flag.go | 04-20 | |
| `service.ModerationService`, `NewModerationService(q ModerationQuerier, opts...)` (built in `api.NewRouter` from `deps.Sessions`), `Queues(ctx) (ModerationQueues, error)` with `ModerationQueues{ Review, Flagged, Pending []ModerationRow; Counts struct{ Review, Flagged, Pending int64 } }` (counts from `ModerationQueueCounts`, on `ModerationQuerier` and its fake), `Find(ctx, id) (ModerationRow, []string, error)`, `Keep(ctx, id, moderatorID int64) (ModerationRow, error)`, `Remove(ctx, id, moderatorID int64) (ModerationRow, error)`; sentinels `ErrAlreadyHandled`, `ErrReportNotFound`; messages `ModAlreadyHandledMessage` (409), `ModNotFoundMessage` (404); `func ParseModeratorEmails(raw string) (map[string]struct{}, error)` (split on commas, trim, skip empty, `NormalizeEmail`, dedupe; an error names the 1 based position, never the value) | `internal/service/moderation.go` | 04-23 | `main.go` |
| `func NormalizeEmail(raw string) (string, error)` (exported wrapper returning exactly what `normalizeEmail` returns, same `ValidationError`) | `internal/service/auth.go` | 04-23 | `ParseModeratorEmails` |
| `api.requireModerator(q, allowed map[string]struct{}) func(http.Handler) http.Handler` (own `GetAccountBySessionID` lookup; any failure is `http.NotFound`); `Deps.ModeratorEmails map[string]struct{}` built by main.go with `service.ParseModeratorEmails(os.Getenv("MODERATOR_EMAILS"))` (an error is `log.Fatalf` naming the variable and position; unset or empty logs one WARNING and gives an empty set). The moderator routes live in their own `r.Group` on the root router, a sibling of the access gated group, outside `requireVerifiedAccount`, with `r.Use(requireModerator(q, allowed))` and every path on `r.Handle`; RESEARCH Code Examples' "inside the gated group" means this group | `internal/api/moderator.go` | 04-23 | |
| `handlers.ModeratePage(tmpl *template.Template, cfg PageConfig)`; view model carries `cfg.Chrome.withDefaults()` and `cfg.AssetVersion` (every local asset tag uses `?v={{.AssetVersion}}`); `TestModeratorPageRenders` uses the real view model | `internal/api/handlers/moderation.go` | 04-23 | 04-26 |
| `service.ModerationWorker`, `NewModerationWorker(q WorkerQuerier, s TextScreener, opts...)`, `Run(ctx context.Context)` (one sweep at start, then every `ModerationSweepInterval = 30 * time.Second`), `SweepOnce(ctx) error`; backoff `min(30s * 2^attempts, 1h)` times jitter 0.5 to 1.5, at least `Retry-After`; claim lease 60 s; at most 5 rows per sweep | `internal/service/modworker.go` | 04-14 | `main.go` |
| e2e harness `newE2EServerWith(t, tweak func(d *api.Deps, q *sqlcgen.Queries))` (nil tweak equals today's `newE2EServer`) and `newGateTestServerWith(t, tweak)`; defaults set every `RateLimits` bucket generous so existing suites never hit a limit; later plans pass a tweak from their OWN test file and never edit the harness files | `reports_e2e_test.go`, `gate_test.go` | 04-09 | 04-11, 04-15, 04-17, 04-20, 04-23 |

**sqlc (all Phase 4 SQL created by 04-05; later plans read the committed generated files, never trust these lines).**
Paths follow sqlc.yaml: migrations in `internal/store/migrations/`, queries in `internal/store/queries/`, generated
Go in `internal/store/sqlc/` (package `sqlcgen`); never create a repo root `queries/` or `sqlc/` directory.
Migration (next free number, name by purpose `internal/store/migrations/..._add_moderation_and_idempotency.sql`): `reports.moderation_state TEXT NOT NULL DEFAULT 'clear' CHECK (moderation_state IN ('clear','pending','removed'))`, `reports.request_id TEXT`, `reports.request_account_id BIGINT REFERENCES accounts(id)`, `reports.client_written_at TIMESTAMPTZ`, partial unique index `UNIQUE (request_account_id, request_id) WHERE request_id IS NOT NULL`; tables `moderation_checks` (report_id PK FK, status `pending` or `done`, attempts, next_attempt_at, lease_until, last_error, verdict, scores JSONB, model, needs_review BOOL, checked_at, created_at; partial indexes on pending and on needs_review), `report_flags` (id, report_id FK, account_id FK, reason CHECK in the five slugs, created_at, dismissed_at, `UNIQUE (report_id, account_id)`), `moderation_actions` (id, report_id, moderator_account_id, action CHECK in `keep`, `remove`, created_at; no report text). Queries: `internal/store/queries/reports.sql` gains `InsertReportIdempotent :one`, `ReportByRequest :one`, `ReportExtrasForReports :many` (report_id, moderation_state, client_written_at, viewer_flagged), `ReportForModeration :one`; `internal/store/queries/votes.sql` `ReportVoteContext` gains `r.moderation_state`; new `internal/store/queries/moderation.sql`: `InsertModerationCheck :exec`, `ClaimDueModerationChecks :many` (`FOR UPDATE SKIP LOCKED`, lease, `expires_at > now()`, joined to report text and meta), `FinishModerationCheck :exec` (sets status `done` plus verdict, scores, model, checked_at and `needs_review`, the last passed as a parameter, so 04-14 needs no new query), `RescheduleModerationCheck :exec`, `SetReportModerationState :exec`, `ReviewQueue :many`, `FlagQueue :many` (open flags grouped with per reason counts), `PendingQueue :many` (unexpired, oldest first), `ModerationQueueCounts :one` (bigint `review_count`, `flagged_count`, `pending_count`, each an unlimited COUNT over its list's WHERE; `flagged_count` is `COUNT(DISTINCT report_id)` over open flags), `ModerationStatusForReport :one`, `KeepReport :exec` (one statement: clear `needs_review`, dismiss open flags, release a pending report and close its check), `RemoveReport :exec`, `InsertModerationAction :exec`, `InsertReportFlag :one` (`ON CONFLICT DO NOTHING RETURNING id`). Lists cap at 100; heading counts come only from `ModerationQueueCounts`, never from a list's length. `NearbyReports` is not touched.
**04-29 (w2, the only other `make sqlc` run):** the Phase 3 history query (built: `ReporterHistoryReports` in `internal/store/queries/reliability.sql`, 03-06; re-grep) selects `r.moderation_state` through the lateral subquery as a sixth column `t.moderation_state`, never filtered in WHERE (held and removed rows count under the read cap `ReliabilityHistoryReadCap` like any expired report, and the scorer drops them as neutral), and commits the regenerated `reliability.sql.go`.

### Wire contracts

| Route | Method | Gate | Contract |
|-------|--------|------|----------|
| `/disclaimer`, `/policy` | GET | exempt (public) | 200 HTML, `Cache-Control: private, max-age=300`, no account data (04-12) |
| `/low` | GET | gated; logged out gets 302 to `/login?low=1` with `Cache-Control: no-store` | 200 HTML, `Cache-Control: no-store`, sets `pinalert_low_data=1` (04-18) |
| `/` | GET | gated | bare `/` with the cookie and no query string gets 302 to `/low` (`Cache-Control: no-store`); `/?full=1` clears the cookie and 302s to `/`; any other query string is never redirected (04-18) |
| `/login` | GET | exempt | `?low=1` sets the cookie and shows the "is on" note; `?low=0` clears it (04-21) |
| `/api/reports` | POST | gated | request adds `request_id` (optional, `^[A-Za-z0-9-]{16,64}$`, else 400 field `request_id`) and `written_at` (optional RFC 3339 string; a malformed string is stored as NULL, a future value is clamped to the server time, a value older than 14 days is clamped to 14 days before the server time; never rejected; a non string JSON value is a 400 body error like any other type mismatch). 201 `{report: {..., held}}`; replay 200 with the stored report (`held` from its state); 400 validation as today; 422 rejection `{"error":{"field":"description","message": RejectMessage}}`; 429 rate body (below); 500 generic |
| `/api/reports` | GET | gated | each `FeedReportResponse` adds `written_at` (RFC 3339, omitempty, only past the 30 minute gap; 04-17) and `viewer_flagged` (bool; 04-20). `visibility` and `visibility_reason` enums are UNCHANGED: withheld never reaches the feed; 04-01 and 04-07 do not add `withheld` or the two new slugs to any `enums:` tag (see New decisions, R-01) |
| `/api/reports/{id}/{confirm,dispute,resolve,reopen}` | POST | gated plus `RateGate.VoteMiddleware(kind)` | 404 for a withheld report (04-07); 429 rate body with field `rate_vote` or `rate_vote_change`; `CastVoteResponse` enums UNCHANGED |
| `/api/reports/{id}/flag` | POST | gated plus flag bucket | body `{"reason": slug}` with `DisallowUnknownFields`; 201 `{"flagged": true}`; 400 field `reason`; 403 field `report` (own report); 404 field `report` (missing or withheld); 409 field `report`; 429 field `rate_flag` (04-20) |
| `/moderate` | all methods via `r.Handle` | `requireModerator` | moderator GET 200 HTML (`no-store`, `<meta name="robots" content="noindex">`); moderator other method 405 from the handler; everyone else `http.NotFound` byte equal to an unknown path (04-23) |
| `/api/moderation/queue` | all via `r.Handle` | `requireModerator` | GET `{"review":[ModRow],"flagged":[ModRow],"pending":[ModRow],"counts":{"review":N,"flagged":N,"pending":N}}`; headings render `counts`, never an array length; a Keep or Remove decrements every list that held the id (floor 0) or refetches |
| `/api/moderation/reports/{id}` | all via `r.Handle` | `requireModerator` | GET `{"report": ModRow, "lists": ["review"\|"flagged"\|"pending"...]}`; 404 `{"error":{"field":"id","message":...}}` for an unknown id |
| `/api/moderation/reports/{id}/keep`, `/remove` | all via `r.Handle` | `requireModerator` | POST 200 `{"report": ModRow}`; 404 unknown; 409 already handled (keep on a report in no list, remove on a removed report); each success writes one `moderation_actions` row |

**POST /api/reports step order (RESEARCH Code Examples; never reordered).** 04-11 builds steps 1, 2, 6, 7; 04-15
inserts 4, 5 and the refund on reject and error; 04-17 inserts 3, the `request_id` check inside step 2 and the refund
for a replay found at insert. (1) Decode with `DisallowUnknownFields`. (2) `ValidateSubmitInput`; a 400 (malformed
`request_id` included, 04-17) touches no bucket. (3) `FindReplay` answers 200 with the stored report, no bucket, no
provider. (4) The IP backstop (keyed on the client IP, skipped when it is unknown), then the submit attempt bucket,
both inside the handler after decode, never as middleware on this route, and neither ever refunded (step 7's `Cancel`
covers only the step 5 allowance); a 429 from either uses field `rate_report` and `ReportLimitMessage(serious, wait)`,
where `serious` is computed from the decoded body (`meta.Serious()`) and `emergency_number` is the configured primary
number (`Chrome.withDefaults().Numbers.Primary`, set only when `serious`), so a serious post blocked by either carries
the serious form (D-12). (5) Allowance `Reserve` (`ReportSerious` when `meta.Serious()`, else
`ReportOther`). (6) `SubmitChecked`. (7) 201 with `held`. `Decision.Cancel` runs exactly once on every outcome after
step 5 except a stored new report: a 422 text or prefilter reject, any non 2xx, and `Replayed` true (insert returned
no row, 200 from `ReportByRequest`). A 201 held report keeps its cost.

**ModRow (moderator JSON, not in swagger):** `id`, `category`, `severity`, `description`, `created_at`, `expires_at`, `written_at` (omitempty), `serious` (bool), `moderation` (`clear`, `pending`, `removed`), `view` (`active`, `disputed`, `resolved`, `past` or `none`; from the Phase 3.1 view partition, `none` for withheld or past every window), `visibility_word` (omitempty; empty exactly when `ReportMeta.Withheld()`, at any expiry; else Showing to everyone (Live or Unconfirmed in the Active view), Disputed, Resolved, Expired, or No longer in any view; never a moderation word, which the client derives from `moderation` plus `serious`, UI-SPEC moderation row item 1), `scores` and `model` (on every ModRow of a report whose check is `done` with `needs_review`: queue, Find, Keep and Remove alike; omitted otherwise), `flags` (`{"total": N, "reasons": [{"reason": slug, "label": FlagReasonLabel, "count": n}]}`, only reasons with count at least 1). No flagger identity, no email, no session.

**429 body (every Phase 4 limiter):** header `Retry-After: <integer seconds>`; body `{"error":{"field": F,"message": M},"retry_after_seconds": N,"serious": bool,"emergency_number": "112"}` with `emergency_number` omitempty and set only when `serious` is true (report route, serious post). Fields `rate_report`, `rate_vote`, `rate_vote_change`, `rate_flag`. The submit attempt bucket and the IP backstop answer with the route's own field and message: on `POST /api/reports` both run inside the handler after decode (step (4) above), so a serious post blocked by either carries `serious` true and the number (D-12); on the vote and flag routes the backstop runs inside that route's own gate. Swagger: one `handlers.RateLimitResponse` definition and `@Failure 429` on `POST /reports`, the five vote routes (four plus the Phase 3.1 take back `unresolve`) and the flag route (04-15 adds all but the flag route's, which 04-20 adds). Only 04-15 edits `swagger_test.go`; later guards read `docs/swagger.json` as it does, in the plan's own file: 04-17 `TestSubmitIdempotentSwaggerFields` (`request_id`, `written_at` on `SubmitReportRequest`, `held`, `written_at` on `FeedReportResponse`) and 04-20 `TestFlagSwaggerCoversRoute` (flag post with 201, 400, 401, 403, 404, 409, 429 referring to `RateLimitResponse`; `FlagRequest` reason enum equal to `FlagReasons` in order; `FlagResponse.flagged`; `viewer_flagged`). The existing `perip.go` body and `Too many requests. Try again in a minute.` stay untouched.

**Swagger definitions Phase 4 adds:** `RateLimitResponse`; fields `request_id`, `written_at` on `SubmitReportRequest`; `held` on `ReportResponse`; `written_at`, `viewer_flagged` on `FeedReportResponse`; `FlagRequest` (`reason` with `enums:"spam,abusive,false,dangerous,other"`), `FlagResponse`. The name guard `(?i)cell|account|session|email` and the `session_id`, `reporter_account_id` substring bans stay green. No moderator API annotations.

**Cookies and query names:** `pinalert_low_data` (value `1`); queries `full=1`, `low=1`, `low=0`, `resend=<request_id>` (read only by `queue.js` `mount` on the main page and the Low data view, 04-22, which opens the form prefilled from that saved row; the Activity link is built by 04-25). Phase 4 never reads `view` or `report` server side; `moderate.js` only builds `/?report=ID` or `/?view=<view>&report=ID` strings.

### Server authored copy

Closed character set for every Phase 4 server string: ASCII letters, digits, space and `, . ' ? : ( ) =`. No
U+2014, U+2013, curly quotes or ellipsis. Shared helper `assertServerCopy(t, s string)` in
`internal/service/servercopy_test.go` (04-02). The test family `TestServerCopy...` (matched by
`-run TestServerCopy`) lives in one file per owner. Every Phase 4 server message is an exported constant or func in
package `service`, never a literal in `internal/api` or its handlers, which map sentinels to these names for
`writeFieldError` (not the votes.go inline literal pattern); the gate's `http.NotFound` body is not a message
(T-04-06). The new test files `servercopy_test.go`, `screening_test.go`, `screening_fixture_test.go`,
`ratecopy_test.go`, `moderationcopy_test.go`, `requestid_copy_test.go`, `flag_test.go`, `moderation_test.go` and
`moderation_rating_test.go` are `package service_test` (like auth_test.go), never use the fakes in trust_test.go (a
`package service` file), and a test needing an unexported symbol goes in its own `package service` file without
`assertServerCopy`. Fakes in either package are named after the interface they fake: `fakeTextScreener` (04-11,
`textcheck_test.go`), `fakeWorkerQuerier` (04-14, `modworker_test.go`), `fakeFlagQuerier` (04-20, `flag_test.go`),
`fakeModerationQuerier` (04-23, `moderation_test.go`); 04-29's `moderation_rating_test.go` calls the pure
`ClassifyOutcome` and `ReporterSignals` with row literals and declares no fake. Before declaring a fake, a plan greps
`^type fake` over `internal/service/*_test.go`: a name already declared in the same package (for example the existing
`fakeAuthQuerier`, `fakeMailer`, `fakeFeedQuerier` in `package service_test` or `fakeVotingQuerier` in `package
service`) is reused, never declared again.

| Message set | File | Test | Owner |
|-------------|------|------|-------|
| Four rejections: "This text looks hateful, so it can't be posted. Reword it and try again." / "...looks like harassment..." / "...looks sexual..." (UI-SPEC exact) / gibberish: "Say what is happening so people nearby can help, for example: flood on Main Road, three people on a roof." (the owner's wording of 2026-10-08, replacing the UI-SPEC gibberish row; see "Open questions resolved during planning") | `internal/service/screening.go` | `TestServerCopyRejections`, `TestRejectMessages` (`servercopy_test.go`) | 04-02 |
| Five server composed limit messages: report ("You've posted several reports in a short time. You can post again in {WAIT}. Your text is still here."), serious report ("You've posted several serious reports in a short time. You can post again in {WAIT}. Your text is still here."), vote, vote change, flag (UI-SPEC exact). The serious sentence "In an emergency, call {PRIMARY} now." is always appended by the CLIENT | `internal/service/ratecopy.go` | `TestServerCopyRateLimits`, `TestWaitPhrase` (`ratecopy_test.go`) | 04-09 |
| Two queued row limit messages, CLIENT authored in the `queue.js` constant table (listed so V-22's seven has one owner): "Waiting to send. It will send by itself in {WAIT}." and the serious form appending "In an emergency, call {NUMBER} now." only when the 429 has `serious` true (`tel:` link built by DOM calls) | `web/static/js/queue.js` | `TestQueueStatusCopy` (`web/queue_contract_test.go`: plain characters, no dashes, the 112 sentence only on the serious form) | 04-22 |
| Held label and sentence, removed label, sentence and Policy pointer, the Disclaimer Resolved entry | `internal/service/moderationcopy.go` | `TestServerCopyModeration` (`moderationcopy_test.go`) | 04-12 |
| Flag 400, 403, 404 and 409 messages (`FlagReasonMessage`, `FlagOwnReportMessage`, `FlagNotFoundMessage`, `FlagAlreadyMessage`); `FlagReasonLabel` strings | `internal/service/flag.go` | `TestServerCopyFlag` (`internal/service/flag_test.go`) | 04-20 |
| Moderator API error messages (`ModAlreadyHandledMessage`, `ModNotFoundMessage`) | `internal/service/moderation.go` | `TestServerCopyModeratorAPI` (`internal/service/moderation_test.go`) | 04-23 |
| `request_id` 400 message (`RequestIDInvalidMessage`, returned by `ValidateSubmitInput`) | `internal/service/report.go` | `TestServerCopyRequestID` (`internal/service/requestid_copy_test.go`, new, package `service_test`) | 04-17 |
| Bar sentence and legal page text | templates | `TestDisclaimerBarSentenceIsExact` (04-06), `TestLegalPages...` (04-12), `TestUserVisibleCopyUsesPlainPunctuation` (existing) | 04-06, 04-12 |

V-22's seven rate limit messages are five Go server strings (`TestServerCopyRateLimits`, 04-09) plus two client
queued row strings (`TestQueueStatusCopy`, 04-22); see the V-22 command override in "Validation rows".

Exact Activity strings (04-12 defines, 04-16 renders): `HeldLabel` "Waiting for a text check"; `HeldSentence` "It isn't shown in any feed until a text check finishes. Reload the page later to see where it stands."; `RemovedLabel` "Removed after review"; `RemovedSentence` "This report was removed after review, so it no longer shows to anyone."; `RemovedPolicyPointer` "If you have a question, see the Policy page." (rendered only when `CONTACT_EMAIL` is set, "Policy page" a link).

### JavaScript exports shared across plans

Rows whose function already exists in `app.js` (`showToast`, `submitReport`, `fetchReports`) are edits in place,
never new definitions; the plan's first task greps for the existing definition and stops if it is missing.

| Export | Module | Created or extended by | Used by |
|--------|--------|-----------|---------|
| `Pinalert.showToast(message, action, durationMs)` (settled 2026-10-16: `action` is the Phase 3.1 short Undo object or `null`; `durationMs` is optional and sets a plain toast's length, default the built `TOAST_MS` of 3000, while an action toast keeps 8 seconds; callers with a duration and no action write `showToast(msg, null, MS)`; sets `data-toast-open="true"` on `<html>` while shown and clears it at every hide; still one `function showToast`, one `getElementById('toast')` and the `showToast: showToast` export; 04-13 moves the Phase 3.1 pins `TestToastAction` (header, body read, timer line) and `TestReviewFixAToastPauseMouseOnly` (body read) to this header and records old and new literals) | `app.js` | 04-13 (extends the built `showToast(message, action)`) | 04-19, 04-22, 04-24, 04-26 |
| `fetchReports(options)` rules (the Phase 3 signature, `options.afterWrite` stays): `state.hasLoaded` widens the Phase 3 background line, so every call after the first success is a background refresh; `AbortSignal.timeout(10000)` with an `AbortController` fallback, carried in `url = new Request(url, { signal: signal });` so the request call stays `pending = fetch(url)`; a `TimeoutError` or `AbortError` that is not stale is a failed poll and moves neither the applied marker (`lastAppliedSeq`) nor the write floor, a stale response (success or failure) changes nothing, and `hasLoaded` and `lastUpdatedAt` are written only in the success write; sets `data-feed-requested` on `<html>` when the first request is sent; still exactly two `fetch(`, zero `show_disputed` and one `'&view='` in app.js (the Phase 3.1 pins of `TestShowDisputedUsesOneSharedQueryParam`; the place search's one `fetch(` is in `map-tools.js`); the 3.1 stale test and tab drop (`sentView`, `sentCategory`, `sentLat`, `sentLon`, `rowsFor`, `rowsMatchStore()`) stay as built | app.js | 04-13 | 04-18 |
| `Pinalert.writtenLine(report)` returns `""` or `"Written " + Pinalert.relativeTime(report.written_at)` | app.js | 04-17 | 04-19 (Low data rows), 04-26 (moderator rows) |
| `Pinalert.formatWait(seconds)` (rounded up, same words as `WaitPhrase`) | app.js | 04-19 | 04-22, 04-24 |
| `Pinalert.lockPage(backdrop)`, `Pinalert.unlockPage()` (P4-A26 walk from the backdrop to `<body>`, `inert` plus `aria-hidden` fallback, never on `#toast` or `#notice-live`, `data-modal-open` on `<html>`) | app.js | 04-19 | `report-form.js` (04-19), `flag.js` (04-24) |
| `Pinalert.submitReport(payload)` resolves to the created report including `held`; rejects with an error carrying `status`, `field`, `fieldMessage`, `queueable` (no HTTP answer, timeout, 502, 503, 504), `retryAfterSeconds`, `serious`, `emergencyNumber`; a 200 replay resolves like a 201; still one `function submitReport`, exactly two `fetch(` in app.js and the `submitReport: submitReport` export | app.js | 04-22 (extends the existing `submitReport(payload)`) | `report-form.js`, `queue.js` |
| `window.PinalertReportForm`: `mount({ backdrop, form, location, variant })` where `variant` is `"main"` or `"low"`; `open({ opener, prefill, note, requestId })`; `close()`. Location interface object: `current()` returns `{ latitude, longitude }` or `null`; `choose(result)` with `{ lat, lon, displayName }`; `isSet()`; `missingMessage()`; `searchFailureMessage(serverMessage)` (main returns the server message or its constant; low ignores the server text) | `report-form.js` | 04-10 (mount, location interface), 04-19 (variant low, search timeout `SEARCH_TIMEOUT_MS = 8000`), 04-22 (`prefill`, `note`, `requestId`, queue paths, Send after the wait) | `modal.js`, `low.js` |
| `PinalertVotes.createVoteBlock(reportId, options)` with `options.resolve === false` skipping Mark resolved inside the same function body; 429 handled inside the existing `castVote` fetch, the wait remembered per report and vote kind (content: confirm and dispute; resolution: resolve and reopen; as the server cooldown key), so neither button of the pair sends while it runs | `votes.js` | 04-19 | `low.js` |
| `window.PinalertQueue`: IndexedDB database `pinalert-queue` version 1, store `reports`, key path `request_id`, index `account_id`; record `{ request_id, account_id, payload, written_at, status, attempts, next_try_at, last_error, serious, emergency_number, sending_until }`; `mount({ strip, accountId, headingLevel, onEdit, focusFallback })` (`focusFallback`, added by 04-22, is a function returning the element to focus when the strip empties), `save(record)`, `flush()`, `discard(requestId)`, `get(requestId)`; Web Locks name `pinalert-queue-flush` with a `sending_until` lease fallback; in memory fallback; `navigator.storage.persist()`; flush on `online`, page load, every `QUEUE_FLUSH_MS = 30000` and Send now; never discards for age. Given `onEdit`, `mount` reads `resend` once after the store loads; for a row with that `request_id` and `account_id` equal to `accountId` it calls `onEdit(record, null, note)` (`onEdit` is always called as `onEdit(record, opener, note)`: the opener is the strip's Edit and resend button, or `null` on this `resend` landing, and `note` is the ready "Not sent. {REASON}" note that `queue.js` builds once for both pages; the strip's Edit and resend path, `PinalertReportForm.open({ prefill: record.payload, note, requestId: record.request_id })`), then drops `resend` with `history.replaceState`; any other case does nothing, silently. Mount calls run after the form mount: `feed.js` (after `modal.js`) on the main page, form before queue inside `low.js`. 04-25 adds a `resendHref(record)` option making Edit and resend a link (`/low?resend=ID` when the 04-21 Activity back link points at `/low`, else `/?resend=ID`); Activity passes it and no `onEdit` | `queue.js` | 04-22 (store, mount, `resend` landing, the `focusFallback` option, the `onEdit(record, opener, note)` arguments), 04-25 (`resendHref` option) | 04-25 (`activity.js` one mount call, passing `focusFallback`) |
| `window.PinalertFlag`: `createFlagControl(report)` returns `{ el }`; `updateFlagControl(control, report)`; one dialog per page `#flag-dialog`; two `stopPropagation` listeners (click, Enter and Space keydown) without `preventDefault` | `flag.js` | 04-24 | `feed.js`, `map.js`, `low.js` |
| `low.js` row function `createLowRow(report)` using `PinalertVisibility.createVisibilityTag`, `PinalertTrust.createTrustBlock`, `PinalertVotes.createVoteBlock(id, { resolve: false })`, `Pinalert.iconClass`; no `tabindex`, no row listener; `Pinalert.subscribe` and `Pinalert.setCenter` before the first fetch; poll from `data-poll-interval-ms` (60000); stored position key `pinalert.lowLastPosition` in `localStorage`. 04-18's `createLowRow` has no vote block and no `PinalertVotes` call; 04-19 adds the vote block after the trust block with the votes.js `options` change (D-15, P4-A15) | `low.js` | 04-18 (rows, boot), 04-19 (form, votes, Written line), 04-22 (strip mount), 04-24 (flag control) | |
| `PinalertVisibility.displayState(report)` precedence: removed, held (only while `report.expired !== true`, in exactly that form), retracted, expired, vote state; `VISIBILITY_STATES` stays four values; `'retracted'` still precedes `report.expired === true`, so 03-15's tests stay green unedited | `visibility.js` | 04-16 | `activity.js` |

**Script order (deferred unless stated).** Main page head: `slow-notice.js` first (not deferred, before every `<link>`), stylesheets (the two unpkg sheets with `media="print"` and `onload="this.media='all'"`), `theme.js`. Body end after the three unpkg tags: `app.js`, `votes.js`, `visibility.js`, `trust.js`, `queue.js`, `flag.js`, `report-form.js`, `map.js`, `modal.js`, `feed.js`, `account-menu.js`. Low data view: `theme.js` (head); `app.js`, `votes.js`, `visibility.js`, `trust.js`, `queue.js`, `flag.js`, `report-form.js`, `low.js`, `account-menu.js`. Activity: `app.js`, `votes.js`, `visibility.js`, `trust.js`, `queue.js`, `activity.js`, `account-menu.js`. Moderator: `theme.js` (head), `app.js`, `trust.js`, `moderate.js`, `account-menu.js` (04-26 adds `trust.js` for the shared read only severity label `PinalertTrust.createSeverityLabel` that UI-SPEC "The moderation row" item 1 asks for; a departure from the UI-SPEC Hand Off "Script order", recorded under "Open questions resolved during planning"). Login, verify outcome, Disclaimer, Policy: `theme.js` only (login also `auth.js`). Each plan adds only the tags of scripts that exist after it; the final order above holds after 04-24.

### CSS, DOM and data attribute contracts

- **Tokens:** `--disclaimer-bar-height` declared once in `main.css` `:root` plus two media tiers (480px, 900px) (04-08); `--account-menu-max-height` set at runtime by `account-menu.js` (04-21). No other new custom property.
- **Top chrome markup (04-06):** `div.top-chrome` > `aside.disclaimer-bar[aria-label="Pinalert notice"]` > one `p` with the sentence and `a[href="/disclaimer"]` (with `aria-current="page"` on the Disclaimer page), then the account header partial on account pages only. First element in `<body>` on every full page. The partial file never contains the text `<html`.
- **chrome.css ownership:** 04-06 the bar box (fill, border, padding, type), `.top-chrome`, `.disclaimer-bar p { margin: 0 }`; 04-08 the bar `min-height: var(--disclaimer-bar-height)` and `scroll-padding-top` (the token is declared by 04-08 in `main.css`), `.inline-error`, `.inline-note`, `.btn[hidden] { display: none; }`, plain, primary and destructive button states, `.btn--destructive`; 04-13 `.notice-card*`; 04-22 `.queue-strip*`, queued rows; 04-24 `.flag-control*`, `.flag-dialog*`.
- **Other sheets:** `legal.css` (04-12), `low.css` (04-18, 04-19, `report-form--low` tile rule), `moderate.css` (04-26), `modal-map.css` (04-10, the `#modal-map`, `#coord-readout`, `#location-notice` rules moved out of `modal.css`).
- **ids:** `#notice-card`, `#notice-live` (04-13, main page; 04-18 adds it to Low data, 04-25 to Activity), `#feed-stale` and `#feed-stale-retry` (04-13), `#queue-strip` (04-22 main and Low data; 04-25 Activity), `#send-after-wait`, `#form-note` (04-22), `#form-error` (existing), `#description-hint` (04-10, in the shared form partial, so on the main page and the Low data view; `#description` carries `aria-describedby="description-hint"`), `#low-list`, `#low-skeleton`, `#low-empty`, `#low-error`, `#low-basis`, `#low-stale`, `#low-report`, `#low-retry-position` (04-18, 04-19), `#flag-dialog` (04-24), `#mod-refresh`, `#mod-refresh-error`, `#mod-find-form`, `#mod-find`, `#mod-find-help`, `#mod-find-result`, `#mod-h-auto`, `#mod-h-people`, `#mod-h-pending` (04-23 template, 04-26 behaviour).
- **Data attributes:** `data-account-id` (numeric account id) on `#app-shell` of the main page and Low data view (04-22 main, 04-18 Low data) and on `.profile-page` (04-25), only on the owner's own HTML; `data-moderation="held"\|"removed"` on Activity `<li>` (04-16); `data-feed-requested`, `data-feed-state="ok"`, `data-toast-open`, `data-popup-open`, `data-modal-open` on `<html>` (04-13, 04-08 for `data-popup-open`, 04-19 for `data-modal-open`).
- **Classes:** `vis-held`, `vis-removed` (04-16, same treatment as `vis-expired` but in their own `trust.css` rules, never grouped with the `.vis-expired` rules, whose exact selectors 03-16's `TestExpiredRowTreatmentMatchesRetracted` reads; each with one compound and descendant badge rule); `report-row--low`, `report-form--low`, `low-*`, `mod-*`, `legal-*`, `notice-card*`, `queue-strip*`, `flag-control*`, `flag-dialog*`. Consumers never type another module's classes.
- **Hidden guard:** every new `display` rule on a hidden capable element carries `:not([hidden])` in its head; the Low data blocks get the id plus `[hidden]` override in `low.css`. `TestPhase4HiddenGuard` (04-08) walks every CSS file over the Phase 4 class prefixes so later rules are covered by construction.
- **CSS rules:** tokens only, no hex, `rgb(`, `rgba(`, `hsl(`, no `!important`, no `999px`, four font sizes and two weights, system colour keywords only inside forced colors blocks.

### Threat IDs (reuse, do not mint per plan)

| ID | STRIDE | Threat | Severity | Disposition | Plans |
|----|--------|--------|----------|-------------|-------|
| T-04-01 | T | Stored XSS through report text, server messages or flag labels on any Phase 4 surface (moderator rows, queued rows, Low data rows, the shared report form and its location results, toasts and the notice card, Activity held and removed rows) | high | mitigate: `Pinalert.setText` and DOM calls only; `TestNoMarkupSinksAnywhere` over every `static/js` file; links in text stay inert (D-05) | 04-08, 04-10, 04-13, 04-16, 04-18, 04-19, 04-22, 04-24, 04-26 |
| T-04-02 | T | Skip screening by marking a post serious | high | mitigate: abuse rejected at every severity (D-03); the fixture proves serious wording is never withheld and abuse is rejected regardless (04-28); late reject withdraws a published serious report (R-03) | 04-02, 04-11, 04-14, 04-28 |
| T-04-03 | D | Provider quota exhaustion or cost by one account | medium | mitigate: submit attempt bucket, process wide request and token budget, pending degradation, no inline retry | 04-04, 04-11, 04-15 |
| T-04-04 | I | Account, session, IP, coordinates or report text leave to the provider or into logs and the audit table | high | mitigate: only `strings.TrimSpace(description)` is sent (recording fake asserts the body); logs carry bucket, numeric id and wait only; `moderation_actions` has no text; the key is never logged | 04-04, 04-05 (store part: no text column in `moderation_checks` or `moderation_actions`), 04-09, 04-11, 04-23 |
| T-04-05 | T | Double insert, or another account's report returned, through `request_id` | high | mitigate: partial unique index on (`request_account_id`, `request_id`), one statement insert, replay scoped to the gate's account | 04-05, 04-17 |
| T-04-06 | I | Moderator route discovery through a 302, 401, 405 or `Allow` header | medium | mitigate: own gate answering `http.NotFound`, routes registered with `r.Handle`, byte equality table including wrong methods | 04-23 |
| T-04-07 | E | A non moderator keeps or removes a report | high | mitigate: the list is normalised once at startup with `NormalizeEmail` (`ParseModeratorEmails`, a bad entry stops startup); on each request the gate looks up the verified account's stored email (already normalised at sign in) in that set; verified account required | 04-23 |
| T-04-08 | T | Flag brigading hides a real emergency | medium | mitigate: a flag never changes `Resolve` input; one flag per account and report; flag bucket; Remove only by a moderator | 04-05 (store part: `UNIQUE (report_id, account_id)`, a flag writes only `report_flags`), 04-20, 04-24 |
| T-04-09 | D | Report, vote or flag flooding | medium | mitigate: PerKey buckets keyed on the verified account; refunds bounded by the burst; memory store reset on restart accepted (Remove is the backstop) | 04-03, 04-09, 04-15, 04-20 |
| T-04-10 | S | Forged client IP header puts many people in one bucket or lets one person rotate | medium | mitigate: header named only by `CLIENT_IP_HEADER`; unset or empty address skips the backstop, never a shared bucket; spoof test stays green | 04-09 |
| T-04-11 | T | CSRF on Flag, Keep, Remove or report posting | low | accept: SameSite=Lax session cookie (DEC-L); cross site POSTs carry no cookie | 04-20, 04-23 |
| T-04-12 | I | Legal pages leak account data or are stored in a shared cache with a cookie | medium | mitigate: no account header or email (R-07), `Cache-Control: private, max-age=300` | 04-12 |
| T-04-13 | T | Open redirect through the Low data cookie redirect or `full=1` | low | mitigate: constant targets `/low` and `/` only | 04-18, 04-21 |
| T-04-14 | I | A saved report is sent or shown under another account on a shared device | medium | mitigate: rows bound to `data-account-id`, never drawn, counted or sent for another account; the server takes the account from the session | 04-22, 04-25 |
| T-04-15 | R | A moderator action cannot be traced | low | mitigate: one `moderation_actions` row per Keep and Remove | 04-05, 04-23 |
| T-04-16 | E | A held or removed report reachable through a vote, a flag or a read path that forgot moderation | high | mitigate: the `Resolve` rung, 404 on vote and flag, `TestResolveCallersMentionModeration`, `TestWithheldInNoView`; Activity returns a withheld row only to its author, with no trust block, counters or votes (`TestActivityModerationRowsOmitTrust`); the rating treats it as neutral (`TestModeratedReportsAreNeutralForRating`) | 04-01, 04-05 (store part: the state lives on the row from insert), 04-07, 04-16, 04-20, 04-29 |
| T-04-17 | T | A client supplied `written_at` changes expiry, order or fade | medium | mitigate: display only, clamped, never feeds `created_at`, `expires_at`, sort or stage | 04-05 (store part: its own nullable column that no query filters, orders or computes expiry from), 04-17 |
| T-04-18 | T | Misleading emergency numbers (a number shown as national or as always working) | medium | mitigate: exact sentence test, numbers from one setting, empty lists drop their text; human check on live government pages before promotion | 04-06, 04-12, 04-27 |

### File ownership (creating plan, then later editors, by wave)

Same wave plans never share a file. New tests go in new files owned by their plan; existing test files are
edited only for an amendment in "Tests that change", each by exactly one plan. `web/design_rules_contract_test.go`
derives its template set from `TemplatesFS`, so no plan edits it (04-06 and 04-12 run it). A file is new only
when it is marked (new) or sits in a row marked (all new); every unmarked file already exists (or is created by
Phase 3) and is edited, never rewritten.

| File | Created or first edited by | Later editors (wave) |
|------|---------------------------|----------------------|
| `internal/service/visibility.go`, `visibility_test.go`, `moderation_view_test.go` (new) | 04-01 (w1) | none |
| `internal/service/screening.go`, `screening_test.go`, `servercopy_test.go` (all new) | 04-02 (w1) | `screening.go`: 04-27 (w9), only the two threshold constants |
| `internal/service/screening_fixture_test.go`, `internal/service/testdata/screening/genuine.json`, `abusive.json` (all new) | 04-28 (w2) | JSON: 04-27 (w9) commits the recorded `model` and `scores`, never the text |
| `internal/ratelimit/perkey.go`, `perkey_test.go` (all new) | 04-03 (w1) | none |
| `internal/moderation/client.go`, `loader.go`, `client_test.go`, `loader_test.go`, `cmd/modrecord/main.go`, `cmd/modrecord/main_test.go` (all new) | 04-04 (w1) | none |
| `internal/store/migrations/<next free number>_add_moderation_and_idempotency.sql` (new), `internal/store/queries/reports.sql`, `internal/store/queries/votes.sql`, `internal/store/queries/moderation.sql` (new), `internal/store/sqlc/models.go`, `internal/store/sqlc/reports.sql.go`, `internal/store/sqlc/votes.sql.go`, `internal/store/sqlc/moderation.sql.go` (new; generated by `make sqlc`, package `sqlcgen`), `internal/testutil/db.go`, `internal/store/migrations_test.go`, `internal/store/moderation_test.go` (new) | 04-05 (w1) | none |
| `internal/store/queries/reliability.sql`, `internal/store/sqlc/reliability.sql.go` (generated), `internal/store/reliability_test.go` (all Phase 3 03-06) | 04-29 (w2) | none |
| `internal/service/reliability.go` (03-03), `reporter.go`, `reporter_test.go` (03-10; the test only if a row literal is positional), `internal/service/moderation_rating_test.go` (new) | 04-29 (w2) | none |
| `web/templates/disclaimer_bar.html.tmpl` (new), `verify_outcome.html.tmpl`, `internal/api/handlers/chrome.go` (new), `internal/api/handlers/chrome_render_test.go` (new), `web/chrome_contract_test.go` (new) | 04-06 (w1) | none |
| `web/templates/index.html.tmpl` | 04-06 (w1) | 04-10 (w2), 04-13 (w3), 04-22 (w7), 04-24 (w8) |
| `web/templates/login_gate.html.tmpl` | 04-06 (w1) | 04-21 (w6) |
| `web/templates/profile.html.tmpl` | 04-06 (w1) | 04-16 (w4), 04-21 (w6), 04-25 (w8) |
| `internal/api/handlers/page.go` | 04-06 (w1) | 04-18 (w5), 04-22 (w7) |
| `internal/api/handlers/auth.go` | 04-06 (w1) | 04-16 (w4), 04-21 (w6), 04-25 (w8) |
| `internal/api/router.go` | 04-06 (w1) | 04-09 (w2), 04-12 (w3), 04-15 (w4), 04-18 (w5), 04-20 (w6), 04-23 (w7) |
| `cmd/server/main.go` | 04-06 (w1) | 04-09 (w2), 04-11 (w3), 04-14 (w4), 04-23 (w7) |
| `web/static/css/chrome.css` (new) | 04-06 (w1) | 04-08 (w2), 04-13 (w3), 04-22 (w7), 04-24 (w8) |
| `internal/service/report.go` | 04-07 (w2) | 04-11 (w3), 04-17 (w5) |
| `internal/service/auth.go` | 04-07 (w2) | 04-23 (w7) |
| `internal/service/trust.go`, `trust_test.go`, `feed.go`, `feed_test.go`, `auth_test.go`, `resolve_callers_test.go` (new), `internal/api/handlers/withheld_e2e_test.go` (new), and since Phase 3.1 `historyread.go` (the `classify` Resolve caller of History, Focus and RecentHistoryCounts) and `activity_page_test.go` (whole call logs of `ActivityPageForAccount`) | 04-07 (w2) | `feed_test.go` only: 04-11 (w3, two interface only `fakeFeedQuerier` stubs for the new insert queries), 04-17 (w5, one interface only `ReportByRequest` stub) |
| `web/static/css/main.css` | 04-08 (w2) | 04-13 (w3) |
| `web/static/css/auth.css` | 04-08 (w2) | 04-21 (w6) |
| `web/static/css/trust.css` | 04-08 (w2) | 04-16 (w4) |
| `web/static/js/map.js` | 04-08 (w2) | 04-17 (w5), 04-24 (w8) |
| `web/profile_nav_contract_test.go` | 04-08 (w2, P4-A1 lock 1) | 04-21 (w6, P4-A21 back link) |
| `web/popup_contract_test.go` (Phase 3 03-17, 03-19), `web/chrome_css_contract_test.go` (new), `web/js_sinks_contract_test.go` (new) | 04-08 (w2) | none |
| `internal/api/handlers/ratelimit_gate.go` (new) | 04-09 (w2) | 04-15 (w4), 04-20 (w6) |
| `internal/service/ratecopy.go` (new), `ratecopy_test.go` (new), `internal/api/handlers/reports_e2e_test.go` (harness only), `internal/api/handlers/votes_ratelimit_e2e_test.go` (new), `internal/api/ip_backstop_test.go` (new) | 04-09 (w2) | none |
| `internal/api/handlers/trust_e2e_test.go` (existing, also edited by Phase 3 plans; harness only: one `RateLimits` line in `newTrustE2EServer`) | 04-09 (w2) | none (04-15 reads it) |
| `internal/api/gate_test.go` | 04-09 (w2, harness) | 04-12 (w3, `TestAccessGateExemptRoutes`) |
| `web/static/js/report-form.js` (new), `web/templates/report_form.html.tmpl` (new) | 04-10 (w2) | 04-19 (w6, `report-form.js` only; the static partial is not edited), 04-22 (w7, both) |
| `web/static/js/modal.js`, `web/static/css/modal-map.css` (new), `web/report_form_contract_test.go` (new) | 04-10 (w2) | `modal.js` only: 04-22 (w7, one guard line) |
| `web/static/css/modal.css` | 04-10 (w2) | 04-19 (w6) |
| `web/js_contract_test.go` | 04-10 (w2) | 04-18 (w5, `TestIconGlyphsAreClassDriven` adds `low.js`) |
| `web/votes_contract_test.go` | 04-10 (w2, toast delegation) | 04-16 (w4, `vis-held` and `vis-removed` in the `TestBothSurfacesMountTheSameVisibilityTag` literal list), 04-18 (w5, fourth surface), 04-19 (w6, `createVoteBlock` anchor) |
| `internal/service/textcheck.go`, `textcheck_test.go`, `internal/api/handlers/submit_screening_e2e_test.go` (all new) | 04-11 (w3) | none |
| `internal/service/report_test.go` (existing; adds one test) | 04-11 (w3) | none |
| `internal/api/handlers/reports.go` | 04-11 (w3) | 04-15 (w4), 04-17 (w5), 04-20 (w6) |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | 04-11 (w3) | 04-15 (w4), 04-17 (w5), 04-20 (w6) |
| `web/templates/disclaimer.html.tmpl`, `policy.html.tmpl`, `web/static/css/legal.css`, `internal/api/handlers/legal.go`, `internal/api/handlers/legal_e2e_test.go` (package `handlers_test`: the render tests and the e2e), `web/legal_contract_test.go`, `internal/service/moderationcopy.go`, `moderationcopy_test.go` (all new) | 04-12 (w3) | none |
| `web/static/js/app.js` | 04-13 (w3) | 04-17 (w5), 04-19 (w6), 04-22 (w7) |
| `web/static/js/feed.js` | 04-13 (w3) | 04-17 (w5), 04-22 (w7) (04-24 no longer edits it: Phase 3.1 took every control off the list row) |
| `web/static/js/slow-notice.js`, `web/feed_stale_contract_test.go`, `web/slow_notice_contract_test.go` (all new) | 04-13 (w3) | none |
| `internal/service/modworker.go`, `modworker_test.go` (all new) | 04-14 (w4) | none |
| `internal/api/handlers/votes.go`, `internal/api/handlers/swagger_test.go` (edit, existing) | 04-15 (w4) | none |
| `internal/api/handlers/report_ratelimit_e2e_test.go` (new) | 04-15 (w4) | none |
| `web/static/js/visibility.js`, `internal/api/handlers/activity_moderation_e2e_test.go` (new), `web/activity_moderation_contract_test.go` (new) | 04-16 (w4) | none |
| `web/static/js/activity.js` | 04-16 (w4) | 04-25 (w8) |
| `internal/api/handlers/submit_idempotent_e2e_test.go`, `web/written_line_contract_test.go`, `internal/service/requestid_copy_test.go` (all new) | 04-17 (w5) | none |
| `web/templates/low.html.tmpl`, `web/static/js/low.js`, `web/static/css/low.css` (all new) | 04-18 (w5) | 04-19 (w6), 04-22 (w7; `low.css` not edited), 04-24 (w8; `low.css` not edited) |
| `internal/api/handlers/low.go` (new), `internal/api/low_routes_test.go` (new), `web/low_contract_test.go` (new), `internal/api/gate.go` | 04-18 (w5) | none |
| `web/static/js/votes.js`, `web/low_form_contract_test.go` (new) | 04-19 (w6) | none |
| `web/vote_controls_contract_test.go` (Phase 3 03-09), `web/vote_hint_contract_test.go` (Phase 3 03-18), `web/takeback_contract_test.go` (Phase 3.1 plan 14) | 04-19 (w6, `createVoteBlock` anchor re-pointed to `createVoteBlock(reportId, options)`) | none |
| `internal/service/flag.go`, `flag_test.go`, `internal/api/handlers/flag.go`, `flag_e2e_test.go` (all new) | 04-20 (w6) | none |
| `web/templates/account_header.html.tmpl` (edit, existing), `web/static/js/account-menu.js` (edit, existing), `web/ways_in_contract_test.go` (new), `internal/api/handlers/ways_in_e2e_test.go` (new) | 04-21 (w6) | none |
| `web/static/js/queue.js` (new) | 04-22 (w7) | 04-25 (w8) |
| `web/queue_contract_test.go` (new) | 04-22 (w7) | none |
| `internal/api/moderator.go`, `internal/api/handlers/moderation.go`, `internal/service/moderation.go`, `moderation_test.go`, `internal/api/handlers/moderator_e2e_test.go` (all new) | 04-23 (w7) | none |
| `web/templates/moderate.html.tmpl` (new) | 04-23 (w7) | 04-26 (w8) |
| `web/static/js/flag.js`, `web/templates/flag_dialog.html.tmpl`, `web/flag_contract_test.go` (all new) | 04-24 (w8) | none |
| `web/queue_activity_contract_test.go`, `internal/api/handlers/activity_queue_e2e_test.go`, `web/queue_logic_test.mjs` (all new; the `.mjs` optional) | 04-25 (w8) | none |
| `web/static/js/moderate.js`, `web/static/css/moderate.css`, `web/moderate_contract_test.go` (all new) | 04-26 (w8) | none |
| `README.md`, `docs/MODERATION.md` (new), `web/phase4_gates_contract_test.go` (new), `web/low_budget_contract_test.go` (new), `.planning/phases/01-foundation-report-map/01-UI-SPEC.md`, `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md`, `.claude/skills/sketch-findings-pinalert/references/popup-structure.md`, `.claude/skills/sketch-findings-pinalert/sources/REVIEWS.md`, `docs/TRUST-MODEL.md` (existing; its `### Outcomes` and `### Window, half life and the 30 report cap` sections come from Phase 3 plan 03-10) | 04-27 (w9) | none |

**P4-A5 Escape guards** (`web/static/js/trust.js`, `web/trust_contract_test.go`, both Phase 3 files): 04-19 owns
them, as one generic rule: the `trust.js` listener does nothing while `html[data-modal-open]` is set, which
`Pinalert.lockPage` sets for the Low data form (04-19) and later for the Flag dialog (04-24). Both files are in
04-19's count (14 with the Phase 3.1 `takeback_contract_test.go` anchors: warning, accepted; the two edits are one line each). 04-24 only asserts the
behaviour in its own test file.

If a plan author finds by grep that an anchor it changes is asserted by an existing test in a file not
listed for that plan, the plan adds that file only when no same wave plan lists it (checked against this
table); otherwise it raises the conflict instead of editing.

### Pinned test names (plan authors may add tests, not rename these)

- 04-01: `TestResolveModeration`, `TestResolveModerationZeroValueNeutral`, `TestReportMetaSeriousMatchesBypass`, `TestReportMetaWithheld` (removed, pending non serious, pending serious, clear and `""`), `TestWithheldInNoView` (loops every `Visibility` over `ListableInFeed` and every Phase 3.1 view predicate).
- 04-02: `TestDecide`, `TestDecideIgnoresSeverity`, `TestGibberish`, `TestGibberishCountsRunes`, `TestRejectMessages`, `TestServerCopyRejections`.
- 04-03: `TestPerKeyBurstThenRefuses`, `TestPerKeyRefillOnInjectedClock`, `TestPerKeyIndependentKeys`, `TestPerKeyRefundAtReservationInstant`, `TestPerKeyInterleavedRefundIsLost`, `TestPerKeyIdleWindowNeverEarly`, `TestPerKeyConcurrentBound` (whole package runs with `-race`).
- 04-04: `TestClientRequestShape`, `TestClientSendsOnlyTrimmedText`, `TestClientRetryAfter`, `TestClientTransientAndPermanent`, `TestClientQuotaIsNotRetried`, `TestClientTimeout`, `TestClientBudgetRefusesWithoutNetwork`, `TestModerationURLIsHTTPS`, `TestLoadScreener` (five case matrix), `TestRecorderRefusesWithoutLiveFlag` (in `cmd/modrecord/main_test.go`).
- 04-05: edited `TestMigrationsEmbedded`; `TestMigrationsUpDown`, `TestMigrationsTruncateListsEveryTable` (in `internal/store/migrations_test.go`), `TestInsertReportIdempotentOneRow`, `TestInsertReportIdempotentConcurrent` (`-race`), `TestReportByRequestScopedToAccount`, `TestReportExtrasForReports`, `TestClaimModerationChecksSingleClaimant`, `TestReportFlagsUniquePerAccount`, `TestSeededReportIsClear`, `TestNearbyReportsQueryUnchanged`, `TestModerationQueueCounts` (101 review rows list 100 and count 101; a report with two flag reasons counts once in `flagged_count`).
- 04-06: `TestDisclaimerBarOnEveryFullPage`, `TestDisclaimerBarSentenceIsExact`, `TestDisclaimerBarDropsEmptyNumbers`, `TestDisclaimerBarHasNoTelLink`, `TestLoadEmergencyNumbers`, and `TestExistingPagesRenderWithChrome` in `internal/api/handlers/chrome_render_test.go` (the four existing pages, plus a zero `Chrome` that still renders the default numbers; each later page plan pins its own render test: 04-12 `TestLegalPagesRender`, 04-18 `TestLowPageRenders`, 04-23 `TestModeratorPageRenders`); `TestChromeWithDefaults` (also `DefaultEmergencyNumbers().TelHref()` equal to `template.URL("tel:112")` and a stub `html/template` execute of an href from `TelHref` rendering `href="tel:112"`, never `#ZgotmplZ`) and `TestChromeReachesViewModels` (both in `chrome_render_test.go`); `TestDisclaimerBarStylesheet` (`web/chrome_contract_test.go`); runs `TestUserVisibleCopyUsesPlainPunctuation`.
- 04-07: edited `TestNearbyBatchesVoteReadsOnce` (adds `ReportExtrasForReports`: once on a page, zero on empty), edited `TestCastVoteCheckOrder`; edited `TestNearbyBatchesVoteReadsOnceWithReporters` (`feed_test.go`) and `TestActivityForAccountReadsReporterHistoryOnce` (`auth_test.go`), each gaining only the trailing `ReportExtrasForReports` call log entry; `TestCastVoteRefusesWithheld`, `TestActivityCarriesModeration`, `TestResolveCallersMentionModeration`, `TestWrittenAtForDisplay`, `TestWithheldReportsInNoFeed` (e2e).
- 04-08: `TestDisclaimerBarToken`, `TestBarTokenReadByPageWrappers`, `TestTopChromeGeometry`, `TestPhase4HiddenGuard`, `TestPhase4ContrastPairs`, `TestPlainButtonStates`, `TestNoMarkupSinksAnywhere`, `TestStripJSComments`; edited `TestActivityPageLinksBackToTheMap` (lock 1), `TestPopupPaddingTokensAgree` (bar token).
- 04-09: `TestVoteRateLimit` (includes: confirm then dispute on one report within 30 s gives 429 `rate_vote_change`; confirm then resolve does not), `TestVoteRateLimitChangeCooldown`, `TestVoteRateLimitChangeCooldownRefundedOnRefusal`, `TestVoteRateLimitAllFiveRoutes`, `TestIPBackstop`, `TestServerCopyRateLimits`, `TestWaitPhrase`; stays green `TestRequestLinkLimiterIgnoresSpoofedProxyHeaders`.
- 04-10: `TestReportFormLocationInterface`, `TestReportFormOwnsTheFormLogic`, `TestModalMapRulesMoved`, `TestReportFormDescriptionHint` (in `web/report_form_contract_test.go`, the owner's hint of 2026-10-08); edited `TestIconGlyphsAreClassDriven`, `TestLocationSearchUsesTextSinksAndExistingPinPlacement`, `TestToastHasExactlyOneImplementation`; stays green `TestLocationSearchHiddenGuards`, `TestLocationSearchDropdownStacking`.
- 04-11: `TestSubmitScreening`, `TestSubmitScreeningSendsOnlyText`, `TestSubmitScreeningHeldNotInFeed`, `TestSubmitScreeningPendingSeriousPublished` (all in `submit_screening_e2e_test.go`), `TestCheckText`, `TestValidateSubmitInputCountsRunes` (added to existing `report_test.go`).
- 04-12: `TestLegalPagesRender`, `TestLegalPagesRenderNoAccountData`, `TestLegalPagesContactBlockOnlyWhenSet`, `TestLegalPagesNumbersBlock`, `TestLegalPagesGlossaryUsesAppSentences`, `TestLegalPagesPublicPrivateCache` (e2e), all in `internal/api/handlers/legal_e2e_test.go`, render tests executing both templates on the real view model; `TestServerCopyModeration`; edited `TestAccessGateExemptRoutes`; runs `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`.
- 04-13: `TestFeedRenderKeepsRowsOnError`, `TestFeedRenderLoadingKeepsRows`, `TestFetchReportsBackgroundRefreshAndTimeout`, `TestSlowNoticeLoadsFirst`, `TestVendorStylesheetsDoNotBlockPaint`, `TestShowToastDuration`, `TestNoticeCardHidesForToastAndPopup`; stay green `TestVendorMapScriptsLoadInDependencyOrder`, `TestMapPopupSurfaceIsThemeAware`, `TestNoPillShapedControls`, `TestFeedExplainsAnEmptyDisputedResult`, `TestShowDisputedUsesOneSharedQueryParam`, `TestFeedErrorStateKeepsStaleRows`, `TestFetchReportsDropsOnlyResponsesOlderThanTheLastApplied`, `TestAgeAndFadeUseTheServerClock` (Phase 3 pins on `fetchReports(options)` and `render(state)`, run unedited).
- 04-14: `TestModerationWorker`, `TestModerationWorkerBackoff`, `TestModerationWorkerLateRejectRemovesSerious`, `TestModerationWorkerSkipsExpired`, `TestModerationWorkerLateBorderlineNeedsReview` (a held non serious and a published serious pending report each end `clear` with a `done` check, verdict `borderline`, `needs_review`, and appear in `ReviewQueue`).
- 04-15: `TestReportRateLimit`, `TestReportRateLimitSeparateBuckets`, `TestReportRateLimitRefunds` (a 400, a text reject, a prefilter reject and an injected insert error each cost nothing; clock advanced between Reserve and Cancel), `TestReportRateLimitBody` (includes a serious post blocked by the IP backstop, `Deps.ClientIPHeader` set through the harness tweak: field `rate_report`, `serious` true and the number); edited `TestSwaggerSpecCoversRoutes` (requires the 429s).
- 04-16: `TestActivityHeldAndRemovedRows` (e2e), `TestProfileReportAcceptsWithheld`, `TestDisplayStateModeration`, `TestActivityModerationRowsOmitTrust` (also: activity.js types no `vis-held` or `vis-removed` and reads `moderation: row.dataset.moderation`), `TestHeldRemovedRowTreatmentMatchesExpired` (`web/activity_moderation_contract_test.go`, mirroring 03-16's `TestExpiredRowTreatmentMatchesRetracted`: same declarations as `.vis-expired`, one badge rule in both forms, no shared rule head), `TestRemovedPolicyPointerNeedsContact`; edited the `TestBothSurfacesMountTheSameVisibilityTag` literal list. Unchanged: the 03-15 and 03-16 expired tests, `css_contract_test.go`, `activity_e2e_test.go`.
- 04-17: `TestSubmitIdempotentReplayCostsNoAllowance`, `TestSubmitIdempotent`, `TestSubmitIdempotentConcurrent` (`-race`; after N concurrent same `request_id` posts the next distinct report within the burst is admitted), `TestSubmitIdempotentPerAccount`, `TestSubmitIdempotentSwaggerFields`, `TestWrittenAt` ("not-a-date" and "" give 201 with NULL `client_written_at`; a 20 day old value is stored as `created_at` minus 14 days and returned as `written_at`), `TestWrittenAtNeverFeedsExpiry`, `TestWrittenLineFormatter`, `TestServerCopyRequestID` (`internal/service/requestid_copy_test.go`).
- 04-18: `TestLowDataRoutes`, `TestLowDataRoutesCookieRedirect`, `TestLowPageRenders` (both in `internal/api/low_routes_test.go`), `TestLowDataAssetPolicy`, `TestLowDataNoBannedWords`, `TestLowDataRowsNotInteractive`, `TestLowDataOwnFetchBoot`, `TestLowDataHiddenOverrides`, `TestCompressionMiddleware`; edited `TestBothSurfacesMountTheSameVisibilityTag` (`low.js` joins the module list for the three calls and banned literals only; no order anchor), `TestIconGlyphsAreClassDriven` (`low.js`).
- 04-19: `TestLowDataVoteBlockOmitsResolve` (first to assert `low.js` passes `{ resolve: false }`), `TestLowDataLocationStates`, `TestLowDataSearchIgnoresServerText`, `TestLowDataFormCopy`, `TestVoteRateLimitClient`, `TestPageLockPairs`; edited (anchor re-pointed to `function createVoteBlock(reportId, options)`, assertions unchanged) `TestVoteClickDoesNotActivateItsRow`, `TestResolveButtonMountsInsideTheVoteControlsContainer`, the four 03-09 tests and 03-18's `TestVoteHint` listed in "Tests that change"; `TestTrustDisclosure` (P4-A5 guard).
- 04-20: `TestFlagOnePerAccount`, `TestFlagOwnReport`, `TestFlagWithheldReport`, `TestFlagRateLimit`, `TestFlagNeverChangesVisibility`, `TestFlagViewerMarker`, `TestFlagSwaggerCoversRoute` (in `flag_e2e_test.go`), `TestServerCopyFlag`.
- 04-21: `TestLoginLowDataPanel`, `TestLoginLowParamSetsCookie` (e2e), `TestAccountMenuItems`, `TestAccountMenuMeasuresHeight`; edited `TestActivityPageLinksBackToTheMap` (P4-A21); stay green `TestAccountMenuHiddenGuard`, `TestAccountMenuJSHasNoAppShellDependency`, `TestAccountMenuJSOutsideClickListenerIsTornDown`.
- 04-22 (all in `web/queue_contract_test.go`): `TestQueueNoMarkupSinksAndNoAgeDiscard`, `TestQueueBoundToAccount`, `TestQueueSendsThroughSubmitReport`, `TestQueueTimeoutFallback`, `TestQueueFlushTriggers`, `TestQueueStatusCopy`, `TestQueueSendAfterWaitOnlyAfter429`, `TestQueueSubmitReportClassifiesQueueable`, `TestQueueResendParamOpensFormForSameAccountOnly` (the `resend` read in `mount`, the account check before `onEdit`, the `replaceState`, form mount before queue mount), `TestQueueHeldSubmitToast` (the held string at 6000 in `report-form.js` and on the `queue.js` Sent path, chosen on `held`; "Report posted." and "Report sent." stay the non held toasts; all through `Pinalert.showToast(`).
- 04-23: `TestModeratorPageRenders`, `TestModeratorRoutesIndistinguishableFrom404` (table incl. wrong methods), `TestModeratorQueues` (exact `counts`; a withheld ModRow has empty `visibility_word`; a Find result for a review list report carries `scores`), `TestModeratorKeep`, `TestModeratorRemove`, `TestModeratorRemoveById`, `TestModeratorAuditRow`, `TestServerCopyModeratorAPI`, `TestParseModeratorEmails` (`internal/service/moderation_test.go`: valid, mixed case, padded, empty entries, a display name entry fails, unset).
- 04-24: `TestFlagControlClickDoesNotActivateItsRow`, `TestFlagControlBuiltOnceUpdatedInPlace`, `TestFlagControlAbsentOnOwnReport`, `TestFlagDialogCopy`, `TestFlagDialogMatchesServerReasons`, `TestFlagDialogLocksPage`.
- 04-25: `TestQueueActivityStrip`, `TestQueueEditAndResendLink`; optional `web/queue_logic_test.mjs` (`node --test`, not wired into CI).
- 04-26: `TestModerateScreenCopy`, `TestModerateUsesSetTextOnly`, `TestModerateRemoveIsConfirmed`, `TestModerateFindParsesReportParam`, `TestModerateHeadingCountsFromServer` (the heading text reads `counts`, never `.length`).
- 04-27: `TestPhase4NoBareLabels`, `TestPhase4NoBannedViewWords` (whole words and identifier parts, theme toggle files excluded), `TestLowDataByteBudget` (`web/low_budget_contract_test.go`, before the UAT task): raw bytes of the rendered `/low` HTML plus every local CSS, JS and icon file it links (CSS referenced icons included), measured after the wave 8 edits (04-24, 04-25), plus about 15 percent, rounded up to the next KB; figure and budget in the test comment and `docs/MODERATION.md`, which also records any icon dropped to meet the budget (P4-A14; one is dropped only if the measured budget requires it). Human check after recording: `env -u DATABASE_URL go test ./internal/service/ -run TestScreeningFixture -count=1 -v` reports PASS, not SKIP; a rejected genuine entry means adjusting only the two thresholds, noted in `docs/MODERATION.md`.
- 04-28: `TestScreeningFixture`, `TestScreeningFixtureShape` (`internal/service/screening_fixture_test.go`).
- 04-29: `TestModeratedReportsAreNeutralForRating` (`internal/service/moderation_rating_test.go`: expired pending low flood with no votes is neutral, not unconfirmed; removed with 2 confirm places or with disputes is neutral; pending serious and zero value metas judge as before, so 03-03's tests are unchanged; 5 expired held reports leave a New reporter, and 30 newer held reports push no older judged record out of the cap of 30 judged reports); edited `TestReporterHistoryQuerySourceHasExpectedShape` (six column prefix, hand const) and `TestReporterHistoryUsesAccountNotSession` (gains `ModerationState` as the sixth field in its reflection check; verify with the V-06 db command `db ./internal/store/ -run TestReporterHistory`, because the test may skip without a database); `TestReporterHistoryUsesSessionIndex` still finds its index.

## Coverage map

### Decisions and derived rules

| ID | Plan(s) | Note |
|----|---------|------|
| D-01 | 04-04, 04-05, 04-11, 04-14 | one inline attempt, 3 s, text only; background retry (04-14) on the check table and its single claimant claim (04-05, store part) |
| D-02 | 04-01, 04-05, 04-07, 04-11, 04-14, 04-16, 04-22, 04-29 | held off every feed, author label, background release; held toast (04-22); the `moderation_state` column (04-05, store part); a held report is neutral for the reporter rating (04-29) |
| D-03 | 04-02, 04-11 | reject at every severity, form stays open, nothing stored |
| D-04 | 04-02, 04-05, 04-11, 04-14, 04-23 | borderline posts, scores without text, review list (late borderline: 04-14) |
| D-05 | 04-02, 04-08, 04-10 | gibberish only (the gibberish message in the owner's wording of 2026-10-08, 04-02); links inert through the sink scan; the description hint (04-10, owner note of 2026-10-08) |
| D-06 | 04-02, 04-11 | `Decide`, `CheckText` and the messages carry no report vocabulary |
| D-07 | 04-28, 04-04, 04-27 | fixture, recorder, docs sentence, owner records scores |
| D-08 | 04-05, 04-23, 04-26, 04-16 | moderator screen, Keep, Remove, removed label |
| D-09 | 04-05, 04-12, 04-20, 04-24 | flag reasons, never hides, online only; police guidance as plain text on the legal pages (04-12) |
| D-10 | 04-03, 04-09, 04-15, 04-20 | account keyed limits; IP only as a configured backstop |
| D-11 | 04-03, 04-09, 04-15, 04-20 | the numbers, one constants block |
| D-12 | 04-09, 04-15, 04-19, 04-22 | exact wait, `Retry-After`, 112 line for serious, logged blocks |
| D-13 | 04-18, 04-27 | name and line; banned words gate |
| D-14 | 04-18, 04-27 | own page, asset policy and compression (04-18); measured byte budget after the last Low data asset, figure documented in `docs/MODERATION.md` (04-27) |
| D-15 | 04-10, 04-19 | form extraction, post and vote |
| D-16 | 04-18, 04-19 | stored position first; location states |
| D-17 | 04-13, 04-18, 04-21 | slow notice, cookie, ways in |
| D-18 | 04-22 | IndexedDB, real outcomes, votes never queued |
| D-19 | 04-05, 04-07, 04-17, 04-22, 04-25 | never expires client side, server stamps, Written line |
| D-20 | 04-12, 04-13, 04-22, 04-25 | strip, stale list, account binding; honest storage limit on the Policy page (04-12) |
| D-21 | 04-06, 04-08, 04-12, 04-18, 04-23 | bar on every full page, geometry; bar on each new page |
| D-22 | 04-06, 04-12, 04-15, 04-27 | one numbers setting, exact sentence, human check (it names the setting as `EMERGENCY_PRIMARY`, `EMERGENCY_OLDER`, `EMERGENCY_STATE_LINES`, superseding VALIDATION's Manual-Only wording) |
| D-23 | 04-06, 04-12, 04-21 | public pages; bar link (04-06); login and menu links (04-21) |
| D-24 | 04-06, 04-12, 04-16, 04-27 | contact setting, blocks only when set, README checklist |
| R-01 | 04-01, 04-05, 04-07, 04-16, 04-29 | rung, columns, batched read, Activity only; swagger enums deliberately unchanged (New decisions) |
| R-02 | 04-01, 04-07, 04-27, 04-29 | `Resolve` zero value is clear; `ClassifyOutcome` is neutral when `meta.Withheld()`, the history read carries `moderation_state`, the scorer drops neutral rows before its cap of 30 judged reports (04-29); `docs/TRUST-MODEL.md` says so (04-27) |
| R-03 | 04-01, 04-05, 04-11, 04-14, 04-16, 04-23, 04-29 | late reject withdraws; author told (04-16); Remove authority; `removed` is terminal in the store and the claim skips it (04-05, store part); a removed report is neutral for the reporter rating (04-29) |
| R-04 | 04-01, 04-15 | `Serious()` picks the bucket |
| R-05 | 04-05, 04-07, 04-17 | server stamps, request id plus unique index, display only `written_at` |
| R-06 | 04-02, 04-11 | runes in the policy and in `ValidateSubmitInput` |
| R-07 | 04-06, 04-12 | no account data on legal pages |
| R-08 | 04-06, 04-12, 04-18, 04-23 | bar on every full page including the four new ones |
| R-09 | 04-18, 04-19 | fourth trust block surface, Active only |
| R-10 | 04-05, 04-27 | seeded rows clear with no check; docs note for Phase 5 |

### Requirements

| ID | Plans |
|----|-------|
| ROBUST-01 | 04-10, 04-13, 04-18, 04-19, 04-21, 04-27 |
| ROBUST-02 | 04-05, 04-13, 04-17, 04-22, 04-25, 04-27 |
| ROBUST-03 | 04-01, 04-02, 04-04, 04-05, 04-07, 04-11, 04-14, 04-16, 04-27, 04-28, 04-29 |
| ROBUST-04 | 04-03, 04-09, 04-15, 04-20, 04-27 |
| ROBUST-08 | 04-06, 04-08, 04-12, 04-27 |
| ROBUST-09 | 04-05, 04-16, 04-20, 04-23, 04-24, 04-26, 04-27 |
| ROBUST-10 | 04-12, 04-21, 04-27 |

### Validation rows

| Row | Plan(s) | Row | Plan(s) |
|-----|---------|-----|---------|
| V-01 | 04-02 | V-13 | 04-22, 04-25 |
| V-02 | 04-28 (test), 04-04 (recorder), 04-27 (recording) | V-14 | 04-25 (optional) |
| V-03 | 04-04 | V-15 | 04-18, 04-21 |
| V-04 | 04-11 (04-07 for the no feed half) | V-16 | 04-18, 04-13 (`TestFeedRenderKeepsRowsOnError`), 04-27 (`TestLowDataByteBudget`) |
| V-05 | 04-14 (includes a late borderline: released `clear` with `needs_review`) | V-17 | 04-06, 04-12 |
| V-06 | 04-01, 04-07, 04-29 | V-18 | 04-12 |
| V-07 | 04-11 | V-19 | 04-23, 04-26 |
| V-08 | 04-03 | V-20 | 04-20, 04-07 (`TestNearbyBatchesVoteReadsOnce`), 04-24 |
| V-09 | 04-15, 04-17 (replay; that name exists only from wave 5) | V-21 | 04-10, 04-19 |
| V-10 | 04-09 | V-22 | 04-02, 04-09, 04-12, 04-17 (`request_id` copy), 04-20, 04-22 (queued row copy), 04-23 (copy); 04-08 (sink scan) |
| V-11 | 04-09, 04-15 (swagger) | V-23 | 04-05 |
| V-12 | 04-05 (store), 04-17 | V-24 | 04-27 (script), each UI plan lists its steps in `<human-check>` |

**Command overrides (supersede 04-VALIDATION.md for these rows; `quick` and `db` as there):** V-03 `quick
./internal/moderation/ ./cmd/modrecord/`. V-06 adds `|TestModeratedReportsAreNeutralForRating` and `db
./internal/store/ -run TestReporterHistory` (04-29). V-09 `db ./internal/api/handlers/ -run
'TestReportRateLimit|TestSubmitIdempotentReplayCostsNoAllowance'`. V-15 adds `db ./internal/api/handlers/ -run
TestLoginLowParamSetsCookie`. V-17 `quick ./web/ ./internal/api/handlers/ -run
'TestDisclaimerBar|TestLegalPages|TestUserVisibleCopyUsesPlainPunctuation|TestExistingPagesRenderWithChrome'`. V-18
adds `db ./internal/api/handlers/ -run TestLegalPagesPublicPrivateCache`. V-19 adds `quick ./web/ -run TestModerate`
(04-26). V-22 `quick -run 'TestServerCopy|TestQueueStatusCopy|TestNoMarkupSinksAnywhere'`. V-23 `db ./internal/store/ -run
'TestMigrationsEmbedded|TestMigrationsUpDown|TestMigrationsTruncateListsEveryTable'` (04-05). Rows not listed keep their
command; a pinned name outside its row's regex is still run by the per-commit quick run (a database test only by its plan's own verify) and by its plan's own verify.

### UI-SPEC amendments

| ID | Plan(s) | ID | Plan(s) | ID | Plan(s) |
|----|---------|----|---------|----|---------|
| P4-A1 | 04-06 (markup), 04-08 (geometry) | P4-A11 | 04-21 | P4-A21 | 04-21 |
| P4-A2 | 04-08 | P4-A12 | 04-13 (duration, width, toast flag), 04-22 (held toast and the two No signal save toasts at 6000) | P4-A22 | 04-18 (`.low-page`), 04-26 (`.mod-page`) |
| P4-A3 | 04-08 | P4-A13 | 04-15 (body), 04-22 (form and queue) | P4-A23 | 04-10 |
| P4-A4 | 04-08 | P4-A14 | 04-18 (asset policy), 04-27 (icons counted in the measured budget; any dropped icon recorded in `docs/MODERATION.md`) | P4-A24 | 04-13 |
| P4-A5 | 04-19 (guard), 04-24 (dialog Escape) | P4-A15 | 04-19 | P4-A25 | 04-08 |
| P4-A6 | 04-24 | P4-A16 | 04-08 | P4-A26 | 04-19 (lock pair, form), 04-24 (dialog) |
| P4-A7 | 04-24 | P4-A17 | 04-08 (CSS), 04-22 (discard), 04-26 (remove) | P4-A27 | 04-19 |
| P4-A8 | 04-17 (feed, popup, formatter), 04-16 (Activity), 04-19 (Low row), 04-26 (moderator row) | P4-A18 | 04-12, 04-21 (login `low=1`) | P4-A28 | 04-08 (states), 04-10 (`#submit-report:disabled` opacity removed in `modal.css`) |
| P4-A9 | 04-16 | P4-A19 | 04-13 | | |
| P4-A10 | 04-16 | P4-A20 | 04-08 (asserted unchanged) | | |

### Tests that change (each amendment edited by exactly one plan; rows marked run only list every plan that runs the test)

| Test | Plan |
|------|------|
| `TestActivityPageLinksBackToTheMap` lock 1 (P4-A1) | 04-08 |
| `TestActivityPageLinksBackToTheMap` back link address and name (P4-A21) | 04-21 |
| Phase 3 popup max height token agreement `TestPopupPaddingTokensAgree` (P4-A4) | 04-08 |
| `TestTrustDisclosure` Escape guards (P4-A5) | 04-19 |
| Phase 3 contrast test (new pairs) | 04-08, as its own `TestPhase4ContrastPairs` (Phase 3 files untouched) |
| `TestAccessGateExemptRoutes` and the router doc comment (P4-A18) | 04-12 |
| Router doc comment, third part: the exempt set (rewritten in w3), the access gated group, and the moderator group outside the access gate answering 404 to everyone not listed | 04-23 |
| `TestBothSurfacesMountTheSameVisibilityTag` banned literal list (`web/votes_contract_test.go`): adds `"vis-held"` and `"vis-removed"` (P4-A9); module list and anchors stay for 04-18 | 04-16 |
| `TestUserVisibleCopyUsesPlainPunctuation` (derived set, run only) | 04-06, 04-12 |
| `TestIconGlyphsAreClassDriven` (`report-form.js` in, `modal.js` out) | 04-10 |
| `TestIconGlyphsAreClassDriven` (`low.js` added) | 04-18 |
| `TestLocationSearchUsesTextSinksAndExistingPinPlacement` | 04-10 |
| `TestLocationSearchHiddenGuards`, `TestLocationSearchDropdownStacking` (stay green, run only) | 04-10 |
| Anchor re-pointed to `createVoteBlock(reportId, options)`, assertions unchanged: `TestVoteClickDoesNotActivateItsRow`, `TestResolveButtonMountsInsideTheVoteControlsContainer` (`votes_contract_test.go`); `TestResolveConfirmIsANamedGroupWithCancelFirst`, `TestEscapeCancelHookIsExported`, `TestVoteButtonsCarryIconAndLabelSpans`, `TestVoteBlockReturnsNoteAndConsumersAppendInOrder` (`vote_controls_contract_test.go`, 03-09); `TestVoteHint` (`vote_hint_contract_test.go`, 03-18); `TestTakeBackFinishedReportIsReadOnly`, `TestTakeBackLimitSentences` (`takeback_contract_test.go`, Phase 3.1 plan 14) | 04-19 |
| `TestToastHasExactlyOneImplementation` (delegation re-pointed to `report-form.js`) | 04-10 |
| `TestBothSurfacesMountTheSameVisibilityTag` (fourth surface) | 04-18 |
| `TestVoteBlockHiddenGuard`, `TestAccountMenuHiddenGuard` (stay green, run only) | 04-08 and 04-21 run them |
| `TestVendorMapScriptsLoadInDependencyOrder`, `TestMapPopupSurfaceIsThemeAware` (stay green, run only) | 04-13 |
| `#toast` rule readers (`TestNoPillShapedControls`, vote contract tests; stay green, run only) | 04-13 |
| `TestFeedErrorStateKeepsStaleRows`, `TestFetchReportsDropsOnlyResponsesOlderThanTheLastApplied`, `TestAgeAndFadeUseTheServerClock` (Phase 3 pins on `fetchReports(options)` and `render(state)` in `web/votes_contract_test.go`; stay green, run only) | 04-13 |
| `TestCastVoteCheckOrder` (withheld step) | 04-07 |
| `TestNearbyBatchesVoteReadsOnce` (extras call) | 04-07 |
| `TestNearbyBatchesVoteReadsOnceWithReporters` (`feed_test.go`) and `TestActivityForAccountReadsReporterHistoryOnce` (`auth_test.go`), both created by Phase 3 plan 03-12, and the whole call logs of `activity_page_test.go` (quick task 261013-wm5, extended by Phase 3.1 plan 07): each exact call log on a success path gains only the trailing `ReportExtrasForReports` entry | 04-07 |
| `TestSwaggerSpecCoversRoutes` (429s on `POST /reports` and the five vote routes, the Phase 3.1 take back `unresolve` included, `RateLimitResponse`, `held`; the only `swagger_test.go` edit) | 04-15 |
| `TestValidateSubmitInputCountsRunes` added to existing `report_test.go`; `TestValidateSubmitInput` length cases stay green | 04-11 |
| `TestMigrationsEmbedded` | 04-05 |
| `TestReporterHistoryQuerySourceHasExpectedShape`, `TestReporterHistoryUsesAccountNotSession` (`internal/store/reliability_test.go`, 03-06) | 04-29 |
| `TestPrimaryMapHasResolvedHeight` (stays green with the calc) | 04-08 |

### Docs to update

| Item | Plan |
|------|------|
| `03-UI-SPEC.md` accent list (P4-A16 item 6 extension, P4-A17 item 5) and A60, A65 and A68 pointers to P4-A4 (A68 because the P4-A4 row and the UI-SPEC "Earlier contracts" row both list it: its `panPopupIntoView` padded view rectangle uses the top reserve P4-A4 re-derives) | 04-27 |
| `docs/TRUST-MODEL.md` (sections from Phase 3 plan 03-10): under `### Outcomes` one bullet saying a held report (pending, not serious) and a removed report are neutral whatever their votes (Phase 4 R-02); in `### Window, half life and the 30 report cap` the sentence about where the two cap readings differ also names held and removed reports | 04-27 |
| `01-UI-SPEC.md` report modal "Cancel" to "Cancel report" | 04-27 |
| Sketch findings `references/popup-structure.md` "Actions" row and the d4 build checklist (`sources/REVIEWS.md`) | 04-27 |
| `ROADMAP.md`, `REQUIREMENTS.md` scope edits | done (commit e82fde6, owner approved 2026-10-06); no plan edits them |
| `README.md` launch checklist (U2: add to the numbered list "Before sharing a deployed instance with real visitors", or create a Launch checklist section): "set the contact email (`CONTACT_EMAIL`; empty means no contact block)" and "check the emergency numbers on the live government pages and change only `EMERGENCY_PRIMARY` (default 112), `EMERGENCY_OLDER` (default 100,101,108) and `EMERGENCY_STATE_LINES` (default 1070,1077; set but empty drops the text) if they differ", plus `CLIENT_IP_HEADER` canary, `MODERATOR_EMAILS`, `OPENAI_API_KEY`, zero balance key check. The "Run locally" export block gains one commented line each, in its style, with default and fail fast rule: `OPENAI_API_KEY` (optional in development, where checks pass with a WARNING; required otherwise, like `RESEND_API_KEY`), `MODERATION_MODEL`, `CONTACT_EMAIL`, the three `EMERGENCY_*`, `CLIENT_IP_HEADER` (unset skips the IP backstop), `MODERATOR_EMAILS` (unset: no moderator; a bad entry stops startup) | 04-27 |
| New `docs/MODERATION.md` (screening cascade, thresholds and snapshot, the "zero of N, under about 3 in N" sentence, held and removed states, worker, moderator screen, Flag, limits and the memory store consequence, retention statement, R-10 for Phase 5, the Low data byte budget figure and any icon dropped to meet it (P4-A14), the recorder: `MODERATION_LIVE=1` plus `OPENAI_API_KEY` and its token throttle) | 04-27 |

### Verified corrections in the brief

| # | Correction | Plan |
|---|------------|------|
| 1 | Refund with `CancelAt(reservation instant)` in `sync.Once`; tests advance the clock | 04-03 (and 04-09, 04-15 refund tests) |
| 2 | Moderator routes with `r.Handle`; wrong method table | 04-23 |
| 3 | Idle window per bucket, at least burst times refill | 04-03, 04-09 |
| 4 | `Submit` has one caller; `SubmitChecked` added, `Submit` deleted | 04-11 |
| 5 | `ReportExtrasForReports` on `Querier` and `AuthQuerier` and their fakes only; `CastVote` via `ReportVoteContext` | 04-05, 04-07 |
| 6 | `Resolve` caller guard strips comments, allowlists `reliability.go` with one call | 04-07 |
| 7 | `low.js` subscribes and calls `setCenter` before any vote | 04-18 |
| 8 | `stripJSComments` and the `map.js` comment reword | 04-08 |
| 9 | `.top-chrome` is Phase 4's wrapper; the 3.2 bell sits next to the account trigger | 04-06, 04-08 |
| 10 | Own walker test over Phase 4 selectors | 04-08 (`TestPhase4HiddenGuard`) |
| 11 | Four existing tests are amended, not created | 04-06 and 04-12 (copy test, run only), 04-15, 04-07, 04-05 |
| 12 | Phase 3 plans are final; plan files win over the collisions table | every plan's prerequisites section |
| 13 | UI contract code facts U1 to U11 | U1 04-15; U2 04-27; U3 04-08, 04-19; U4 04-21; U5 04-17; U6 04-10; U7 04-18; U8 04-19; U9 none (untouched); U10 04-19; U11 04-07 |

## Open questions resolved during planning

Orchestrator decisions from the brief (defaults; the owner can veto each one separately):

1. A 400, a text or prefilter reject, any non 2xx after the reservation, an early replay (never reserves) and a concurrent replay found at insert (reserves, then cancels) do not consume the report allowance (04-15 for the reject, the 400 and errors; 04-17 for both replays); see "POST /api/reports step order".
2. Compression: chi `middleware.Compress` (no new dependency) in its own task (04-18); the budget is tested in raw bytes (04-27).
3. The D-07 fixture runs on scores recorded once by the owner against `omni-moderation-2024-09-26`; the recorder and the offline test are built (04-04, 04-28); recording is an end of phase human check (04-27).
4. No link to `/moderate` anywhere; the owner bookmarks it (04-26).
5. No retention job; the Policy page states the real retention (nothing is deleted today) and offers removal through the contact line (04-12).
6. One optional `node --test` file for `queue.js` with a hand written fake IndexedDB, in its own optional task (04-25). CI (`.github/workflows/ci.yml`) runs only Go (`go build`, `go vet`, `go test -v -p 1 ./...`), so it is not wired into CI.
7. Migration numbers are never written; the store plan names the file by purpose.
8. UI-SPEC "Open items for the planner" defaults apply as written (paths, cookie name, bar tiers, timing values, viewer flagged from the batched read, `resend` link, 6 s toasts, `data-popup-open`, backdrop over the bar, no flagger identity and no polling on the moderator screen, no Save-Data hint, no Mark resolved on Low data, strip heading levels, one 30 minute constant, client rendered Low data rows, no non sticky bar, the first request timer for the slow notice); the concrete values are pinned in the contracts above.

New decisions made in this outline (stated with the reason; the owner can veto each):

- **29 plans in 9 waves** rather than about 15 to 20: the added scope and the hot files (`router.go` has seven Phase 4 editors, `main.go` five) force one editor per wave, and merging plans would breach the 15 file blocker. No scope is cut. The revision added 04-28 (fixture split from 04-02) and 04-29 (R-02 rating neutrality, so 04-05 and 04-07 do not grow).
- **R-02 classifier rule:** neutral exactly when `ReportMeta.Withheld()` (removed, or pending and not serious), `Resolve`'s own predicate. A pending serious report was public and its votes are real, so it is judged as before; a zero value meta is clear, so 03-03's tests hold. The owner can veto and make every pending report neutral.
- **R-01's swagger enum clause is not applied.** `withheld` and the two new slugs never reach a documented JSON response (the feed filters through `ListableInFeed`, vote and flag answer 404, Activity is server rendered, ModRow is not in swagger), so the `enums:` tags in reports.go and votes.go and swagger_test.go's four value list stay unchanged. Guards: `TestWithheldInNoView` (04-01), `TestWithheldReportsInNoFeed`, `TestCastVoteRefusesWithheld` (04-07), `TestFlagWithheldReport` (04-20), and the existing `ResolveReasons` loop in `TestResolve_IsTotalAndDeterministic`.
- **One Chrome carrier** (`PageConfig.Chrome`, `AuthConfig.Chrome`, filled by NewRouter, `withDefaults()` for a zero value); no handler takes a separate Chrome parameter; existing signatures unchanged.
- **FlagService and ModerationService are built in `NewRouter`** from `deps.Sessions`, not via `Deps` fields set in main.go, which would be nil in every untweaked test suite.
- **The `resend` landing lives in `queue.js` `mount`** (04-22), the strip's own Edit and resend path; 04-25 only builds the link.
- **Swagger guards after 04-15 live in each plan's own test file**, so 04-17 and 04-20 do not grow.
- **Heading counts come from `ModerationQueueCounts`**, never a capped list's length.
- **`MODERATOR_EMAILS` is parsed once at startup**; a bad entry stops startup naming its position, never the address.
- **`Submit` is deleted** and the handler calls `SubmitChecked` (correction 4: one caller; a wrapper with no caller is dead code).
- **`/?full=1` clears the cookie and redirects to `/`** (UI-SPEC "The way back" wins over RESEARCH Pitfall 15's "returns 200"); any other query string is never redirected.
- **A malformed `written_at` is stored as NULL and an out of bound one is clamped** (future to now, older to now minus `WrittenAtMaxAge`), never a 400, so a queued report is never refused and an old one never looks fresh (R-05, D-19; supersedes RESEARCH Area E's "out of bound is NULL" and A10); a malformed `request_id` is a 400 field `request_id` from `ValidateSubmitInput` with `RequestIDInvalidMessage`.
- **One 429 schema for every limiter**, with `serious` false and `emergency_number` omitted except for a serious report block.
- **On `POST /api/reports` the IP backstop runs inside the handler after decode** (step (4), before the submit attempt bucket, never refunded), not as middleware, so a serious post blocked by the backstop still gets the serious form and the emergency number (D-12: every blocked serious post adds the 112 line; RESEARCH Area C "Gate in the handler, not middleware, for reports"). The middleware alternative would answer every backstop block as non serious. The owner can veto.
- **Fixture location** is `internal/service/testdata/screening/` (the test that reads it lives in `internal/service`), and the recorder is `cmd/modrecord`.
- **The screener is injected with `service.WithScreener`**, not an `api.Deps` field, so the screening plan does not edit `router.go`.
- **The e2e harness gains a tweak hook once (04-09)**; later plans configure Deps from their own test files.
- **Server copy tests are one family, one file per owner** (`-run TestServerCopy`), sharing `assertServerCopy` from 04-02; every such file is `package service_test` and every message lives in package `service` (see "Server authored copy").
- **The flag reason labels live once in Go** (`FlagReasonLabel`); the dialog partial is static markup guarded by `TestFlagDialogMatchesServerReasons`, and the moderator JSON carries the labels.
- **P4-A5 is one generic guard** keyed on `html[data-modal-open]` (set by `lockPage`), owned by 04-19, so 04-24 does not edit Phase 3's `trust.js`.
- **The emergency numbers are one value loaded from env with defaults** (`EMERGENCY_PRIMARY`, `EMERGENCY_OLDER`, `EMERGENCY_STATE_LINES`), so a change needs no code change (D-22 Specifics); set but empty drops the text.
- **The Low data byte budget** is set by 04-27 (wave 9), after 04-24 and 04-25 (both w8) last edit Low data assets; the procedure is pinned under 04-27 in "Pinned test names".

Decisions made in the reconcile run after the 29 plans were written (each stated with the reason; the owner can veto each one separately):

- **OWNER DECISION NEEDED before 04-02 executes: what counts as a "long run of one character" in the gibberish rule (D-05).** The plans are written for option A; nobody has signed it off yet. Record the owner's yes or no as a dated line under D-05 in `04-CONTEXT.md` or in the 04-02 SUMMARY gate.
  - **Option A, letters only (what the plans say now; 04-02 Reading 1, the `IsGibberish` row, 04-11's prefilter truth, 04-28's fixture):** only ten or more of the same letter in a row counts as a run. Consequence: ten exclamation marks, ten dots or ten identical emoji next to two real words are accepted; text made only of such marks is still refused by the second rule (fewer than two real words). Reason: D-05 says the rule never looks at phone numbers and D-07 forbids rejecting a genuine report, and repeated marks are common in real urgent posts. Cost of keeping it: none, the plans already say this.
  - **Option B, any character except digits and spaces (the plan checker's alternative, closer to D-05's and the UI-SPEC's literal "one character"):** ten identical characters of any kind count as a run, except digits and spaces, so phone numbers and long space runs stay safe. Consequence: a post such as "Help!!!!!!!!!! water rising" is refused with the gibberish message, so some genuine urgent wording is refused. Cost: 04-02 Task 2's rule and its `TestGibberish` rows for the exclamation, dot and emoji cases (they become true), and the text in 04-11 and 04-28 that repeats the rule, all change before 04-02 runs.
- **`TelHref` returns `template.URL`** (04-06 builds it, 04-12 uses it for the Disclaimer's one tap to call link). Reason: html/template rewrites an untyped tel URL in an href to `#ZgotmplZ`; the typed value renders as written and stays safe because the scheme is a fixed literal and the primary number is digits only. 04-06 pins the typed value and a template execute that renders `href="tel:112"`; 04-12 asserts the rendered `href="tel:"` plus the primary. The other way (TelHref for Go only, the template writing the literal tel prefix before the number) was not taken.
- **Nullable timestamps from sqlc are `pgtype.Timestamptz`.** sqlc with pgx/v5 generates every nullable timestamptz, parameter or column, as `pgtype.Timestamptz` (the committed `UsedAt` field shows it; `sqlc.yaml` overrides only the non null case to `time.Time`). 04-07's `writtenAtPtr` reads it, 04-17's `writtenAtParam` writes it, and 04-11, 04-14 and 04-23 name that type. This is a fact, not a preference; listed so the owner sees why the plans changed.
- **Same wave RED windows** (standing rule above): plans keep their RED commits (TDD plans need them), and a whole package run that fails only on a same wave sibling's not yet green tests waits for that sibling's GREEN commit and reruns. Veto: run 04-01 and 04-02 (and the wave 2 `internal/service` plans) in isolated working trees instead.
- **The Coverage map credits store and rating parts.** 04-05 is listed for D-01, D-02 and R-03 (store parts) and for threats T-04-04, T-04-08, T-04-16 and T-04-17, and 04-29 for D-02 and R-03 (the rating side), because those plans cite the ids for work they really do; the plans keep their citations. Veto: drop those citations from 04-05 and 04-29 instead, so each matches its original row.
- **The moderator page loads `trust.js`** between `app.js` and `moderate.js` (Script order above), because UI-SPEC "The moderation row" item 1 asks for the shared read only severity label from `PinalertTrust`. The approved UI-SPEC is not edited (the brief forbids it); this entry, 04-26's decisions and its SUMMARY record the departure from the UI-SPEC Hand Off "Script order". Veto: duplicate the severity label in `moderate.js` and drop the tag.
- **04-27 also points 03-UI-SPEC A68 to P4-A4**, because the P4-A4 amendment row and the UI-SPEC "Earlier contracts" row both list A68, even though the UI-SPEC "Docs to update" paragraph names only A60 and A65.
- **04-27 updates `docs/TRUST-MODEL.md`** (one `### Outcomes` bullet and the cap readings sentence) so the Phase 3 trust model document stays true once held and removed reports are neutral for the rating (R-02). Veto: leave the document to a later docs pass.
- **04-23's fixture uses three distinct unfinished texts** listed in one `modUnfinishedTexts` set, so the stub keeps its exact text match and every description stays distinct.
- **Recorded, no change:** 04-06 keeps its three task grouping (the Sizing row's Pitfall 11 reason; `TestChromeReachesViewModels` proves the view models carry Chrome, and the router's local copies stay checked by grep gates and by the render tests of later page plans).

Decisions made in the revision after plan check round 1 (each stated with the reason; the owner can veto each one separately):

- **A failed check row write is retried once, and the rare double failure is an accepted residual risk** (04-11, T-04-16). `SubmitChecked` retries `InsertModerationCheck` once immediately; if both writes fail the report is stored with no check row, so 04-14's worker and 04-05's Pending list never see it (a held report stays held and off every feed, a pending serious report stays published unchecked) and only the log names its id. Reason: two consecutive failed writes right after a successful insert are rare, and fixing it fully needs a new query outside wave 3. Veto: have 04-14 or 04-23 also pick up pending reports that have no check row.
- **The Policy cookie list names the session cookie as set on a first visit, before you sign in** (04-12), instead of UI-SPEC row 6's "the sign in cookie", because the session middleware writes the cookie for a logged out reader too; the "no advertising or tracking cookies" sentence is written only after a grep of every cookie writer, quoted in the SUMMARY. Veto: keep the UI-SPEC wording.
- **The moderation claim fixes its row set in a `MATERIALIZED` CTE** (04-05) instead of `IN (subquery)`, so `LIMIT` with `SKIP LOCKED` cannot claim more rows than the batch limit. Engineering fact, listed so the owner sees why 04-05 changed.
- **Low data rows carry 03-16's shared severity label and shelter line** (04-18: `PinalertTrust.createSeverityLabel()`, `Pinalert.formatShelterLine(report)`, no `CAPACITY_LABELS` copy), because 03-16 replaced those parts of the feed row that 04-18 mirrors.
- **04-25 mounts the Activity strip in Task 1** (the `resendHref` and focus options join the same call in Task 2), so the first task ends with something a person can see; 04-22 keeps its task split and gains a console human check for Task 1 instead.
- **A moderator Keep racing an in flight worker call is closed, not accepted** (04-14): 04-05's `KeepReport` and its pending check guards already stop a late reject, so the old "accept" row was wrong and is now "mitigate".
- **04-09's in flight vote tests are excused by name** (wave note) rather than hidden behind a build constraint; 04-14's scope gate is limited to `cmd/`, `internal/service/`, `internal/store/` and `internal/moderation/`, which no wave 4 sibling touches.
- **04-20's RED is the one RED that does not compile** (its Task 1 leaves the `internal/service` test package unbuilt until its GREEN, which follows at once); wave 6 siblings excuse a failure confined to `internal/service` in that window by their wave notes rather than 04-20 gaining compiling stubs. Veto: give 04-20 compiling stubs like 04-07 and 04-11.

Owner decision of 2026-10-08 (recorded in 04-CONTEXT under D-05; not a veto item, the owner said yes to this exact proposal):

- **People are told to describe what is happening, in two approved strings used verbatim.** (1) The description box of the shared report form partial `report_form.html.tmpl` (so both the main page form and the Low data view form) shows the hint "Describe what is happening, for example: water is knee deep on Main Road and three people need help." as a visible 14px helper line `p#description-hint` between the Description label and the textarea, tied with `aria-describedby="description-hint"`; a visible line was chosen over an input hint attribute inside the box, which disappears while typing and has weak contrast (04-10, `TestReportFormDescriptionHint`; tokens only, the Label size and weight, the muted colour on the `--color-bg` panel). (2) The gibberish refusal returned by `RejectMessage(RejectGibberish)` becomes "Say what is happening so people nearby can help, for example: flood on Main Road, three people on a roof." (04-02, `TestRejectMessages`, `TestServerCopyRejections`). Both strings fit the closed character set as written (letters, spaces, comma, colon, period; no digit), so no wording was adapted. Quotes of the old message were updated in 04-11, 04-22, 04-25 and 04-27.
- **Departure from the approved UI-SPEC copy contract, recorded here because 04-UI-SPEC.md is not edited:** the gibberish row of "Rejected text messages" is replaced (that section lets the planner refine wording, but the owner's sentence leaves the "Reword it and try again." pattern the other three keep), and the description hint is a new string the UI-SPEC copy contract does not list. The hateful, harassing and sexual messages stay UI-SPEC exact. Consequence to note, not blocking: the new gibberish message carries no report word (the D-06 vocabulary guard still passes) but asks the author to say what is happening, which fits a post or a Phase 8 comment; a later phase that screens other text, such as Phase 9 nicknames, may want its own gibberish wording.
- **The D-05 run reading stays "letters only" as planned.** A post of only one word is already rejected as no real words (fewer than two words of two or more letters), so the owner's goal needs no new rule. The owner did not object to the letters only reading; the veto in the "OWNER DECISION NEEDED" entry above remains open until the owner signs it off.

## OUTLINE COMPLETE
