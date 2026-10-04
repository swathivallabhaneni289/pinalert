---
phase: 03
slug: trust-model-hardening-diversity-weighted-trust
mode: mvp
granularity: standard
plans: 20
waves: 9
created: 2026-10-04
---

# Phase 3: Plan Outline

> Chunked run. One single plan agent writes each row below from this outline. Precedence when sources
> disagree (highest first): the owner decisions of 2026-10-04 recorded in the planning brief (A55 kept,
> `Mode: mvp` kept, chunked run); `03-CONTEXT.md` (D-23 sets `c = 0.25`; the replaced D-16 sets two
> counters that are always shown); `03-UI-SPEC.md` amendments A1 to A74 and its "Owner decisions
> (2026-10-03)"; then `03-RESEARCH.md`. RESEARCH's reliability worked table, its invariant I2, its
> `TrustResponse` proposal, its count line copy and its hover tooltip design are stale and are replaced
> by the pins in this file. Where a `03-VALIDATION.md` row conflicts with an amendment, the amendment
> wins and the plan updates the test expectation (V-39 becomes `TestTrustDisclosure`, V-40 is narrowed
> to controls, V-12 tests the two counters).
>
> **Precondition risk (not a plan):** Phase 2 verification is still pending (`02-VERIFICATION.md` is
> human_needed and `02-UAT.md` is fixes_shipped_pending_retest). This phase builds on the resolver and
> edits `votes.js`, `feed.js` and `map.js`, so running `/gsd-verify-work 2` before execution is advised.

**Phase Goal (every plan restates it under `## Phase Goal`):**

**As a** person reading the feed during an emergency, **I want to** see how many different nearby
people confirmed or disputed each report, and whether it is still current and its reporter reliable,
**so that** a few accounts cannot make a weak report look well corroborated.

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| 03-01 | A vote cast from more than 1 km from the report is refused by `CastVote` before anything is written, for all four vote values, with the reporter's own resolve and reopen exempt, NaN and Inf refused, the report's coordinates and creation time read in the same context query, and the existing e2e voters moved inside the radius so the suite stays green (D-01, D-03 comment) | 1 | none | TRUST-05 |
| 03-02 | Geohash precision 7 is a tested, documented choice: a seeded witness simulation bounds the distinct cells a 300 m crowd can produce at five Indian latitudes, and `docs/TRUST-MODEL.md` records the derivation table, the 1 km rule, grandfathered votes and the honest limits without claiming spoof resistance (D-03, D-01 docs) | 1 | none | TRUST-05 |
| 03-03 | Every judged outcome and every reporter rating is decided by pure functions: the classifier reuses `Resolve` with neutralised metadata and zeroed resolution fields, the score uses credits 1, 0 and 0.25 with a 30 day half life inside a 90 day window and needs 5 judged reports, critical and rescue needed reports that expire unconfirmed are neutral, and invariants I1 to I4 hold with I2 restated for `c = 0.25` (D-11, D-12, D-13, D-15, D-23, R-05, R-06) | 1 | none | TRUST-07 |
| 03-04 | A voter more than 1 km away gets HTTP 403 with field `location` and the fixed too far sentence, and the browser silently refreshes an already granted location and retries once before showing that sentence, never prompting per vote: the first user visible increment (D-01, D-02) | 2 | 03-01 | TRUST-05 |
| 03-05 | One server built `Trust` object carries the distinct confirming and disputing place counts N and M, their capped numerals and the panel sentences, built from the same row filter `BuildVoteTally` uses and attached identically by `Nearby`, `CastVote` and `ActivityForAccount` with injectable clocks (D-04, D-16, D-17, D-19, D-21, R-04, A6, A7, A27, A56) | 2 | 03-01 | TRUST-05 |
| 03-06 | The latest expired reports of a batch of reporter accounts are read in one bounded, index backed query keyed by the verified account across all its sessions: at most 30 per account, only inside the 90 day window, proven by an EXPLAIN that names `idx_reports_session_expires` (D-10, D-13, R-05) | 2 | 03-01 | TRUST-07 |
| 03-07 | `GET /api/reports` and every vote response carry the same `trust` object with both counters and their sentences, the OpenAPI spec documents it with no identifier leaking, and concurrent votes from distinct places land as exactly that many places (D-04, D-21, TRUST-05 criterion 1) | 3 | 03-04, 03-05 | TRUST-05 |
| 03-08 | Every trust object carries the still current band, a 0 to 100 score that stays null until 2 distinct places have confirmed, the facts behind it and a fade stage for non critical rows only, all from one pure function over vote times with an injected clock (D-05, D-06, D-07, D-08, R-01, R-02, R-03) | 3 | 03-02, 03-05 | TRUST-07 |
| 03-09 | One vote control builder serves the feed row, the popup and the Activity page: icon and label spans on every vote button, the resolve confirmation ordered question, Cancel, Yes as a named group with explicit focus steps, a shared Reopen builder, a confirmation cancel hook for Escape, and a hidden note node returned with the block (A12, A16, A34, A36, A52, A63, A64, A66) | 3 | 03-04 | TRUST-05 |
| 03-10 | A reporter's rating is composed from their bounded history rows and current votes into the reporter signal (tag, score, everyday label and sentences), every requested account gets a signal, a report with no rated account reads `none`, and the trust object carries it (D-10, D-13, D-14, D-15, D-23, A47, A50) | 4 | 03-03, 03-06, 03-08 | TRUST-07 |
| 03-11 | Vote buttons share one outlined style on all three surfaces with 20px glyphs, an inverted pressed fill, a destructive Yes that wins the cascade, visibly disabled and too far states, an alert glyph on errors and a two track grid (A12, A13, A14, A15, A16, A17, A18, A28, A35, A41, A43, A46, A53, A72, A73) | 4 | 03-09 | TRUST-05 |
| 03-12 | Every read path loads the reporter history once per request, so the feed, the vote response and the Activity page carry the real reporter signal with one batched vote read, at most 200 reporter accounts per feed read and no effect on visibility or order (D-10, D-13, D-14, D-21, R-05) | 5 | 03-05, 03-10 | TRUST-07 |
| 03-13 | A reader sees the trust block on every feed row and map popup: two counters always shown, the still current word and reporter tag whenever the server sends them, popup only scores, and one toggle that opens a panel of server sentences, with the row fade taking the worse of the time stage and the server stage (D-07, D-09, D-16, D-17, D-18, D-19 as amended by A5, D-20, R-02, R-03, R-07) | 5 | 03-07, 03-09, 03-11 | TRUST-05, TRUST-07 |
| 03-14 | The feed and every vote response expose the still current and reporter signals in the pinned wire shape, equal between the two and documented in OpenAPI, so the block shows both signals side by side on real data (D-08, D-21, TRUST-07 criterion 2) | 6 | 03-07, 03-08, 03-12 | TRUST-07 |
| 03-15 | A reporter sees the same trust block on each of their own reports on the Activity page, rendered from the server's trust JSON, with a visible severity word, an Expired state for expired reports and a refresh from the vote response after Reopen (D-20, R-08, A26, A29, A44, A57, A58) | 6 | 03-07, 03-09, 03-13 | TRUST-05, TRUST-07 |
| 03-16 | Every feed row and Activity row names its severity in a visible rounded label read before the title, read only labels share one rounded shape that no control uses, row meta and titles stay legible and wrap, and expired Activity rows get their own treatment (A1, A24, A26, A37, A38, A42, A44, A45, A55) | 6 | 03-13 | TRUST-05, TRUST-07 |
| 03-17 | Opening a pin shows one neutral popup card, built once and updated in place on every poll, with the close button first, the severity and state labels above the title, a category and time meta line with the shelter line, the trust block with scores and the vote controls (D-09, D-20, A2, A9, A10, A11, A19, A20, A21, A24, A25, A30, A31, A32, A54, A62, A65) | 7 | 03-11, 03-13, 03-16, 03-18 | TRUST-05, TRUST-07 |
| 03-18 | Once the viewer's location is known, Confirm, Dispute and Mark resolved on a report farther than the server's radius are marked too far with `aria-disabled` and an always visible sentence built from the page's radius attribute, cleared when the fix is stale or near, while the reporter's own Mark resolved stays enabled and the server stays the gatekeeper (D-22, A8, RR-3) | 6 | 03-09, 03-11, 03-13 | TRUST-05 |
| 03-19 | A keyboard or screen reader user can open a pin with Enter or Space, land in a named popup dialog, close the panel and then the popup with one Escape per level by surface ownership, return to the row or marker that opened it, and keep focus through polls that remove or reorder reports, while autopan clears the fixed controls and never animates under reduced motion (A23, A45, A49, A51, A59, A60, A65, A67, A68, A70, A71, A74) | 8 | 03-17 | TRUST-05, TRUST-07 |
| 03-20 | The trust model is documented end to end, the earlier UI contracts point at this one, regenerated code matches its sources, and the owner walks the full V-42 script on a fresh disposable database with fresh accounts (D-03, D-08, A32, A40, V-42) | 9 | 03-01, 03-02, 03-03, 03-04, 03-05, 03-06, 03-07, 03-08, 03-09, 03-10, 03-11, 03-12, 03-13, 03-14, 03-15, 03-16, 03-17, 03-18, 03-19 | TRUST-05, TRUST-07 |

## MVP framing

The phase is five vertical slices from RESEARCH ("Change map (ask 1)", "Vertical slice option"),
refined where a slice was too big for one plan, plus three presentation slices the UI-SPEC added
(the popup card, the one vote button style and the row labels). Each plan is a step through one
slice and ends at something a real user or a real end to end test can observe. Plans that are pure
computation (03-03, 03-06) are scheduled early because nothing blocks them, and each ends at a
provable guarantee, which is the Phase 2 precedent set by 02-01 and 02-02.

| Slice | Plans in order | What becomes possible after the slice |
|-------|----------------|---------------------------------------|
| S1 "nearby is real" | 03-01, 03-02, 03-04 | A far vote is refused (service test after 03-01, HTTP 403 after 03-04), and a voter who walked to the incident is re-located silently instead of being refused |
| S2 "counters" | 03-05, 03-07, 03-13, 03-15 | `N confirmed` and `M disputed` from distinct places appear on the wire (03-07), then on the feed row and popup (03-13), then on the Activity page (03-15) |
| S3 "still current" | 03-08, then 03-14 | Band, score and fade stage exist on every trust object (03-08) and reach the wire (03-14) |
| S4 "source reliable" | 03-03, 03-06, 03-10, 03-12, then 03-14 | Outcomes and ratings are decided (03-03), the bounded history is readable (03-06), composed into the reporter signal (03-10), loaded on every read path (03-12), and exposed (03-14) |
| S5 "reasons and hint" | panel in 03-13, 03-18 | Every word explains itself in the panel; a too far viewer sees why the buttons are off before tapping |
| Vote style | 03-09, 03-11 | One vote button style on all three surfaces with icons, focus steps and the inline confirmation |
| Rows | 03-16 | A visible severity word on every row, rounded labels, legible meta, wrapping titles |
| Popup card | 03-17, 03-19 | The neutral popup card, then its keyboard, focus and autopan behaviour |
| Closure | 03-20 | Docs, contract pointers, regeneration gate, owner UAT |

S3 and S4 converge in one wire plan (03-14) because TRUST-07 is one requirement, "two signals side
by side", and the signals row is one UI element. The browser block built in 03-13 renders each signal
the server sends and nothing for an absent one, which is the UI-SPEC rule in "Which signals show", so
after 03-14 both signals appear on real data with no further UI change.

**First user visible increment:** 03-04 (wave 2). A real person voting from more than 1 km away sees
"Too far to vote. You need to be within 1 km of this report." in the vote error, and a person whose
cached fix is stale is re-located silently and their vote goes through.

**Increment check, plan by plan** (after plan N, what can a real user or a real e2e test do that it
could not after the plans before it):

- 03-01: a service test casts each of the four vote values from 1.0008 km and sees `ErrVoterTooFar` with zero inserts; the reporter's own resolve from anywhere still succeeds.
- 03-02: a reader of `docs/TRUST-MODEL.md` can see why precision 7 was chosen and what it cannot prevent; a test proves the 300 m crowd bound.
- 03-03: a test can feed any history and get the outcome and rating the decisions define.
- 03-04: a real user from far away gets the clear 403 sentence; a user who moved gets a silent retry.
- 03-05: a service caller of any of the three read paths gets N, M and the panel sentences.
- 03-06: a store test reads a reporter's capped, windowed history across all of their sessions.
- 03-07: an e2e test reads `trust.confirmed_places` on the feed and on a vote response and they agree.
- 03-08: every trust object carries a band, score and fade stage that a test can pin at every boundary.
- 03-09: a keyboard user opening Mark resolved lands on Cancel inside a named group; Reopen on the Activity page is the shared control.
- 03-10: a test turns real history rows into Reliable reporter, Mixed record, Unreliable reporter, New reporter or none.
- 03-11: every surface shows one vote button style with glyphs and a visible disabled look.
- 03-12: the service read paths carry the real reporter signal with one vote read.
- 03-13: a reader sees the two counters and opens the panel on a feed row and a popup.
- 03-14: a reader sees the still current word and the reporter tag side by side on real data.
- 03-15: a reporter sees the block on their own reports and it refreshes after Reopen.
- 03-16: every row names its severity in words before the title.
- 03-17: a pin opens the neutral card that keeps its state across polls.
- 03-18: a far viewer sees the buttons marked too far, with the reason as visible text.
- 03-19: a keyboard user drives pins, popups and panels without losing focus.
- 03-20: the owner signs off the whole phase on a fresh database.

## Sizing

Thresholds: target 2 to 3 tasks and 5 to 8 files. 4 tasks or 10 files is a warning; 5 tasks or 15
files is a blocker. Generated files (sqlc, swag) and the small SVG icons are counted.

| Plan | Tasks | Files | Context weight | Check |
|------|-------|-------|----------------|-------|
| 03-01 | 3 | 9 | medium | ok (9 because the five e2e voter fixes must land with the rule) |
| 03-02 | 2 | 3 | light | ok |
| 03-03 | 2 | 2 | heavy logic, TDD (`type: tdd`) | ok |
| 03-04 | 2 | 7 | medium | ok |
| 03-05 | 3 | 9 | medium | ok |
| 03-06 | 2 | 6 | medium | ok |
| 03-07 | 3 | 8 | medium | ok |
| 03-08 | 3 | 7 | medium heavy | ok |
| 03-09 | 3 | 5 | medium | ok |
| 03-10 | 3 | 6 | medium | ok |
| 03-11 | 3 | 8 | medium | ok |
| 03-12 | 2 | 6 | medium | ok |
| 03-13 | 3 | 8 | heavy | ok |
| 03-14 | 2 | 6 | light medium | ok |
| 03-15 | 3 | 6 | medium | ok |
| 03-16 | 3 | 6 | medium | ok |
| 03-17 | 3 | 9 | heavy | ok |
| 03-18 | 2 | 6 | medium | ok |
| 03-19 | 3 | 5 | heavy | ok |
| 03-20 | 3 | 5 | light, includes the owner UAT | ok |

Total 20 plans in 9 waves, above the 10 to 14 estimate. About 8 of the 20 (03-09, 03-11, 03-13,
03-15, 03-16, 03-17, 03-18, 03-19) carry the UI-SPEC's popup card, vote style, row labels and focus
work; merging them would overload plans. Nothing is cut: D-22 (03-18) is planned in full, and the
CONTEXT planner note to cut D-22 first applies only if a split is ordered later.

## Sources per plan

Line ranges are current as of 2026-10-04. If RESEARCH.md shifts (the orchestrator appends the
resolutions below to its Open Questions), find the heading with `grep -n '^## \|^### '` and read that
section. Never copy these line numbers into a verify command. Every plan also reads this outline's
"Cross-plan contracts", `./.claude/CLAUDE.md`, and the PATTERNS.md "Shared Patterns" section (lines
261 to 280). Every UI plan (03-09, 03-11, 03-13, 03-15, 03-16, 03-17, 03-18, 03-19) also loads
`Skill("sketch-findings-pinalert")` and reads UI-SPEC "Source of Truth and Scope" (17 to 39), "Owner
decisions (2026-10-03)" (40 to 73) and "Hand Off Notes" (789 to 832).

**03-01** distance rule in the service
- CONTEXT: D-01, D-03 (the comment on `voterGeohashPrecision`), code_context "Integration Points" (lines 428 to 442).
- V rows: V-01, V-02, V-03, V-04, V-05, V-06, V-08, V-09.
- RESEARCH: "Distance rule (ask 3)" 533 to 607 (Helper 535 to 553, BoundingBox 554 to 557, ReportVoteContext 558 to 561, Step order 562 to 573, Error mapping 574 to 577, Test cases 578 to 596); "Tests (ask 7)" table "Existing tests that will break" 840 to 857; "Common Pitfalls" 5 and 7 (897 to 905); "Security Domain" 1002 to 1031.
- PATTERNS: "Pure service files" 43 to 74; "`internal/service/trust.go` (modify)" 77 to 120; "votes.sql and reliability.sql" 123 to 144; "Tests to update" 250 to 260.
- Note: `ReportVoteContext` also selects `r.created_at` (this outline's decision, so `CastVote` can compute the currency window in 03-08 without a second query).

**03-02** precision documented
- CONTEXT: D-03, D-01 (grandfathering text), deferred list (lines 484 to 512) so the docs name what is out of scope.
- V rows: V-10.
- RESEARCH: "Derivation table for D-03" 597 to 607; "Distance rule" Helper and Test cases 535 to 596; "Wire and docs" paragraph on `docs/TRUST-MODEL.md` (last bullet of 781 to 788); "Runtime State Inventory" 912 to 915; "Security Domain" Known Threat Patterns 1019 to 1031.
- PATTERNS: "No Analog Found" 281 to 289 (`docs/TRUST-MODEL.md`).

**03-03** reliability pure (TDD)
- CONTEXT: D-11, D-12, D-13, D-15, D-23, R-05, R-06, Claude's Discretion first bullet (lines 257 to 262).
- V rows: V-20, V-21, V-22, V-23.
- RESEARCH: "Reliability score" 674 to 717 (formula only; its numeric table and I2 are stale, use this outline's recomputed anchors); "Outcome classification" 718 to 728; "Options the owner may want to change" 729 to 731; "Property tests worth planning" 864 to 870.
- PATTERNS: "Pure service files" 43 to 74.

**03-04** HTTP refusal and silent refresh
- CONTEXT: D-01, D-02, Phase 2 D-17 and D-18 references (lines 48 to 54).
- A rows: none. V rows: V-07, V-35.
- RESEARCH: "Error mapping" 574 to 577; "`votes.js`: silent refresh, retry once" 804 to 812 (items 1 to 3; item 4 belongs to 03-18); "Common Pitfalls" 1 (885 to 887); "Tests" rows for `votes_contract_test.go` 848 to 850; "Assumptions Log" A7 (1042).
- UI-SPEC: "Copywriting Contract" error rows (677 to 690); "Interaction Contract" item 2 (660 to 672).
- PATTERNS: "votes.go (modify)" 155 to 179; "`web/static/js/votes.js` (modify)" 218 to 229.

**03-05** trust counts in the service
- CONTEXT: D-04, D-16, D-17, D-19, D-21, R-04, Claude's Discretion bullets 3 and 4 (lines 263 to 272).
- A rows: A5 (panel replaces hover), A6, A7, A27, A56. V rows: V-11, V-12, V-32.
- RESEARCH: "Pattern 1" 414 to 418; "Pattern 2" 419 to 421; "Pattern 3" 422 to 424; "Batching and cost" 527 to 532 (read only, history is 03-12); "Copy" 766 to 780 (charset rule only; its count line copy is stale); "Common Pitfalls" 6 and 8 (900 to 908).
- UI-SPEC: "Trust Block" 327 to 410 (What the block shows 331 to 336, The panel 355 to 370); "Copywriting Contract" 673 to 738 (Panel sentence templates 709 to 736).
- PATTERNS: "trust.go (modify)" 77 to 120; "report.go Nearby and auth.go ActivityForAccount" 147 to 152.

**03-06** bounded history read
- CONTEXT: D-10, D-13, R-05, Claude's Discretion last bullet (index) and the history read bullet (lines 285 to 293).
- V rows: V-24, V-25, V-26, V-27, V-28 (store part).
- RESEARCH: "Reliability history read (ask 2)" 463 to 532 (all subsections); "Common Pitfalls" 3 (891 to 893); "Open Questions" Q4 (1051).
- PATTERNS: "votes.sql and reliability.sql" 123 to 144 (migration and store test analogs).

**03-07** counts on the wire
- CONTEXT: D-21, D-04.
- A rows: A27. V rows: V-29, V-30, V-31, V-41.
- RESEARCH: "swag, docs, drift tests" 781 to 788 (name guard must be scoped, see contracts); "Tests" "Conventions verified" 833 to 839; "New test inventory" 858 to 863 (concurrency); "Security Domain" 1002 to 1031.
- PATTERNS: "votes.go (modify) and trust.go (new)" 155 to 179; "Tests to update" 250 to 260 (e2e helpers).

**03-08** still current signal
- CONTEXT: D-05, D-06, D-07, D-08, R-01, R-02, R-03, Claude's Discretion bullets 1, 2 and the `ExpiryDuration` bullet (lines 257 to 291).
- A rows: A50 (still current score sentence). V rows: V-13, V-14, V-15, V-16, V-17, V-18, V-19, V-32 (extension).
- RESEARCH: "Still current score" 612 to 673 (its tables do not depend on `c` and stand as written); "Common Pitfalls" 5, 7 and 8 (897 to 908).
- UI-SPEC: "Copywriting Contract" still current rows and score row (709 to 736); "Which signals show" 337 to 354.
- PATTERNS: "Pure service files" 43 to 74.

**03-09** vote control builder
- CONTEXT: D-20 (Reopen is the only Activity vote control), Phase 2 re-tap contract (canonical refs line 321).
- A rows: A12 (spans), A16 (DOM order and focus), A34, A36, A52, A63, A64, A66, A73 (JS side). V rows: none directly; new contract tests.
- RESEARCH: "Tests" rows for `votes_contract_test.go` 848 to 853; "Common Pitfalls" 2 (888 to 890).
- UI-SPEC: "Vote Controls and Too Far" 581 to 629 (Focus after Mark resolved 604 to 617); "Surface: Feed Row" "Vote controls on the row" 444 to 451 and "Focus order inside a row" 452 to 455; "Interaction Contract" 660 to 672.
- PATTERNS: "`web/static/js/votes.js` (modify)" 218 to 229; "feed.js, map.js, activity.js" 232 to 239.

**03-10** reporter signal composition
- CONTEXT: D-10, D-13, D-14, D-15, D-23, R-05, Claude's Discretion hover sentence bullet (lines 273 to 279).
- A rows: A47, A50 (reporter score sentence). V rows: V-28 (service part), V-32 (extension).
- RESEARCH: "Outcome classification" 718 to 728; "Batching and cost" 527 to 532; "Wire and docs" last bullet of 781 to 788 (limits to document); "Open Questions" Q4 and Q7 (1051, 1054).
- UI-SPEC: "Copywriting Contract" reporter rows and score row (709 to 736); "Which signals show" 337 to 354.

**03-11** vote button style and glyphs
- A rows: A12 (SVG and masks), A13, A14, A15, A16 (CSS layout), A17, A18, A28, A35, A41, A43, A46, A53, A72, A73.
- UI-SPEC: "Component Geometry" 208 to 251; "Typography" 252 to 270; "Color" 271 to 326 (Vote button states 297 to 316); "Vote Controls and Too Far" 581 to 629; "Motion" 739 to 744; "Accessibility" 745 to 776 (Forced colors 756 to 776); "Hand Off Notes" exact heads table 807 to 820.
- RESEARCH: "CSS and layout" 823 to 826.
- PATTERNS: "CSS: trust.css, main.css" 242 to 247 (icon analog `circle-alert.svg`).

**03-12** history on every read path
- CONTEXT: D-10, D-13, D-14, D-21, R-05.
- V rows: V-28 (service part), V-29 (batching part).
- RESEARCH: "Batching and cost" 527 to 532; "System Architecture Diagram" 355 to 388; "Common Pitfalls" 9 (909 to 911); "Security Domain" DoS row (1027).
- PATTERNS: "Interface growth pattern" inside 77 to 120; "report.go Nearby and auth.go ActivityForAccount" 147 to 152; "Tests to update" 250 to 260.

**03-13** trust block on feed row and popup
- CONTEXT: D-07, D-09, D-16, D-17, D-18, D-19, D-20, R-02, R-03, R-07.
- A rows: A3, A4, A5, A6, A7, A22, A23 (listener, guards, row branch), A33, A39 (clock), A47, A48, A50 (popup render), A61, A69. V rows: V-37, V-38, V-39 (renamed `TestTrustDisclosure`), V-40.
- UI-SPEC: "Trust Block" 327 to 410 (all subsections); "Surface: Feed Row" 411 to 465; "Surface: Map Popup" "Popup numbers" 539 to 542; "State Matrix" 630 to 659; "Copywriting Contract" "Labels, words and counters" 694 to 708; "Accessibility" 745 to 776; "Motion" 739 to 744; "Typography" 252 to 270; "Spacing Scale" 188 to 207.
- RESEARCH: "Mount points" 791 to 799; "Fade helper" 827 to 830; "CSS and layout" 823 to 826; "Common Pitfalls" 4 (894 to 896). The "Accessible tooltip" section 813 to 822 is superseded by A5.
- PATTERNS: "`web/static/js/trust.js` (new)" 188 to 215; "feed.js, map.js, activity.js" 232 to 239; "CSS" 242 to 247.

**03-14** signals on the wire
- CONTEXT: D-08, D-21.
- V rows: V-29 (signals equality), V-30 (extension), V-31 (re-asserted).
- RESEARCH: "swag, docs, drift tests" 781 to 788 (A5 `x-nullable`); "Assumptions Log" A5 (1040).
- PATTERNS: "votes.go (modify) and trust.go (new)" 155 to 179.

**03-15** Activity page block
- CONTEXT: D-20, R-08.
- A rows: A26 (Activity row label), A29, A44 (server and template), A57, A58, A64 (`focusToggle` use). V rows: V-33, V-38 (Activity part).
- UI-SPEC: "Surface: Activity Page" 558 to 580; "State Matrix" 630 to 659 (rows 10a and 10b); "Copywriting Contract" Expired row (689, 733).
- RESEARCH: "Mount points" activity and profile bullets (796 to 797); "Assumptions Log" A2 (1037).
- PATTERNS: "votes.go and trust.go handlers" Profile paragraph (178); "feed.js, map.js, activity.js" 232 to 239.

**03-16** row labels and row legibility
- A rows: A1, A24 (formatter), A26 (feed row), A37, A38, A42, A44 (CSS), A45 (rows), A55. Open items 5 and 10.
- UI-SPEC: "Color" "Severity label by level" 317 to 326; "Component Geometry" 208 to 251; "Surface: Feed Row" "Order inside `.report-row__body`" 417 to 431 and "Labels, fades and the block together" 456 to 465; "Accessibility" 745 to 776; "Open items" 833 to 853 (item 10 bullets).
- PATTERNS: "CSS" 242 to 247; "feed.js, map.js, activity.js" 232 to 239.

**03-17** popup card
- CONTEXT: D-09, D-20.
- A rows: A2, A9, A10, A11, A19, A20, A21, A24 (popup meta), A25, A30, A31, A32, A39 (x), A54, A62, A65 (CSS). V rows: none renamed; existing popup tests edited.
- UI-SPEC: "Surface: Map Popup" 466 to 557 (Structure 472 to 501, Build once 502 to 523, Leaflet overrides 524 to 538); "Component Geometry" 208 to 251; "Color" 271 to 326; "Accessibility" forced colors 756 to 776.
- RESEARCH: "Mount points" map.js bullet (795); "Common Pitfalls" 4 (894 to 896).
- PATTERNS: "feed.js, map.js, activity.js" 232 to 239 (popup anchors); "CSS" 242 to 247.

**03-18** too far hint
- CONTEXT: D-22, D-01 (server stays gatekeeper), D-02 (fix freshness).
- A rows: A8. RR-3. V rows: V-34, V-36.
- RESEARCH: "Page config and the radius" 800 to 803; "`votes.js`" item 4 (811); "Common Pitfalls" 1 and 2 (885 to 890).
- UI-SPEC: "Vote Controls and Too Far" "Too far to vote" 618 to 629; "Copywriting Contract" too far row (683).
- PATTERNS: "page.go, cmd/server/main.go, index.html.tmpl (D-22)" 182 to 185; "votes.js" 218 to 229.

**03-19** popup keyboard, focus and autopan
- A rows: A23 (popup branch), A45 (pin name), A49, A51, A59, A60, A65 (JS and token test), A67, A68, A70, A71, A74.
- UI-SPEC: "Surface: Map Popup" "Keyboard and screen reader" 543 to 557 and "Build once" 502 to 523; "Trust Block" "Behaviour" 400 to 410; "Surface: Feed Row" "Panel on the row" 440 to 443 and "Focus order" 452 to 455; "Motion" 739 to 744; "Copywriting Contract" pin name and close button rows (706 to 707).
- PATTERNS: "No Analog Found" 281 to 289 (disclosure and Escape).

**03-20** closure
- CONTEXT: D-03, D-08, specifics on cross phase handoffs (lines 475 to 480), deferred list (484 to 512).
- A rows: A32 (REVIEWS.md historical), A40 (accent list pointers). V rows: V-42.
- RESEARCH: "Validation Architecture" 930 to 1001 (phase gate); "Risks a planner must schedule around" 1056 to 1067; "Runtime State Inventory" 912 to 915.
- UI-SPEC: "Hand Off Notes" "Docs to update" (829) and the three "Tests changed or added" paragraphs (823 to 827) for the V-42 items.
- VALIDATION: "Manual-Only Verifications" 107 to 113 (wording superseded by A5 and A8).

## Cross-plan contracts (pinned here so per-plan calls do not drift)

### Standing rules for every plan

- **Disposable database, one per plan.** Any verify command that touches Postgres uses the plan's own database `p3_03NN` (for 03-07 that is `p3_0307`), never `pinalert_test` and never `pinalert`. Canonical form: `dropdb --if-exists p3_03NN; createdb p3_03NN && DATABASE_URL="postgres://localhost:5432/p3_03NN?sslmode=disable" go test <packages> -run '<regex>' -count=1 -p 1; status=$?; dropdb --if-exists p3_03NN; exit $status`. `testutil.NewTestDB` applies the embedded migrations itself. Parallel plans in one wave therefore never share or drop each other's database.
- **Quick run** (no database): `go test ./internal/service/ ./web/ -count=1`.
- **Ports and running.** Any run or UAT step uses `PORT=8792`, `BASE_URL=http://127.0.0.1:8792`, opens `http://127.0.0.1:8792`, `ENV=development` (log only mailer, magic links are copied from the server log), and a disposable database. Never 8080 or 8090. RESEARCH line 312's `PORT=8090` is stale.
- **No dashes** (U+2014, U+2013) in prose, docs, server strings, templates or `trust.js` (whole file, comments included). Edited files `trust.css` and existing JS keep their existing comment dashes untouched (A69).
- **No new Go or npm packages, no JS build step, no images or screenshots saved in the repo.**
- **Regeneration.** A plan that edits a `.sql` query runs `make sqlc` and commits the regenerated file; a plan that edits a swag annotation or a response struct runs `make swag` and commits `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`. CI has neither tool. Only one plan per wave runs `make sqlc` (03-01 in wave 1, 03-06 in wave 2).
- **Verify hygiene.** No `|| true` or `2>/dev/null || echo` feeding a comparison. Any count in a grep gate is measured first and its provenance stated in the plan. No RESEARCH line numbers in verify commands; anchor on function names and strings.
- **Human verification mode is end of phase.** No `checkpoint:human-verify` tasks. Manual checks go in `<verify><human-check>`; the full V-42 script lives in 03-20.
- **Threat model.** Every plan carries `<threat_model>` at ASVS level 1, blocking on high, reusing the IDs below. No package installs, so no `T-03-SC` row.

### Go identifiers shared across plans

| Identifier | File | Created by | Used by |
|-----------|------|-----------|---------|
| `const MaxVoteDistanceKm = 1.0` | `internal/service/geo.go` | 03-01 | 03-02 docs, 03-04 swagger text, 03-18 `PageConfig.VoteRadiusKm` |
| `haversineKm(lat1, lon1, lat2, lon2 float64) float64` (R = 6371, intermediate clamped to [0, 1]) | geo.go | 03-01 | 03-01 only |
| `withinVoteRadius(rLat, rLon, vLat, vLon float64) bool` written as `!(d <= MaxVoteDistanceKm)` refusal so NaN and Inf fail closed | geo.go | 03-01 | `CastVote` |
| `var ErrVoterTooFar = errors.New("service: voter is too far from the report")` | geo.go | 03-01 | 03-04 handler |
| `func TooFarMessage() string` returns `Too far to vote. You need to be within 1 km of this report.` with the number from `strconv.FormatFloat(MaxVoteDistanceKm, 'f', -1, 64)` | geo.go | 03-01 | 03-04 handler, 03-05 copy test grid |
| `ReportVoteContext` select list gains `r.latitude, r.longitude, r.created_at`; `sqlcgen.ReportVoteContextRow` gains `Latitude, Longitude float64` and `CreatedAt time.Time` | `internal/store/queries/votes.sql` | 03-01 | `CastVote` (03-01, 03-05) |
| `CastVote` step order: 1 validate, 2 `ReportVoteContext`, 3 self vote block (content only), 4 compute reporter resolution exemption from `rc.ReporterAccountID` versus `in.AccountID` (resolution kind only), 5 expired gives `ErrReportExpired`, 6 distance check unless exempt gives `ErrVoterTooFar`, 7 cell, insert, read votes, tally, `Resolve`, trust | `internal/service/trust.go` | 03-01 (steps 1 to 7), 03-05 (trust), 03-12 (history) | |
| `type placeVote struct{ Cell string; CreatedAt time.Time }`, `type voteBuckets struct{ Confirms, Disputes, Resolves, Reopens []placeVote; ReporterResolved, ReporterReopened bool }`, `func filterVoteRows(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) voteBuckets` (reporter content rows skipped, one current row per account as read) | trust.go | 03-05 | `BuildVoteTally` (output and signature unchanged), `BuildTrust`, 03-08 currency |
| `type Trust struct{ ConfirmedPlaces, DisputedPlaces int; ConfirmedText, DisputedText, LeadSentence, ConfirmedSentence, DisputedSentence string }` | `internal/service/trustview.go` | 03-05 | 03-07, 03-15 |
| `Trust.Currency CurrencySignal` | trustview.go | 03-08 | 03-14 |
| `Trust.Reporter ReporterSignal` | trustview.go | 03-10 | 03-12, 03-14 |
| `type TrustInput struct{ Rows []sqlcgen.CurrentVotesForReportsRow; ReportID int64; ReporterAccountID *int64; Meta ReportMeta; Visibility Visibility; CreatedAt, ExpiresAt, Now time.Time }` | trustview.go | 03-05 | three read paths |
| `TrustInput.Reporter *ReporterSignal` (nil yields the `none` signal, which is the real rule for accountless reports and reports past the account cap) | trustview.go | 03-10 | 03-12 sets it |
| `func BuildTrust(in TrustInput) Trust` | trustview.go | 03-05 | 03-08 and 03-10 extend its body |
| `ReportView.Trust Trust` | `internal/service/feed.go` | 03-05 | 03-07 |
| `CastVoteResult.Trust Trust` | trust.go | 03-05 | 03-07 |
| `ActivityReport.Trust Trust` | `internal/service/auth.go` | 03-05 | 03-15 |
| `type ReportOption func(*ReportService)`, `func WithReportClock(fn func() time.Time) ReportOption`, `NewReportService(q Querier, opts ...ReportOption)` | `internal/service/report.go` | 03-05 | 03-07, 03-14 e2e |
| `type VotingOption func(*VotingService)`, `func WithVotingClock(fn func() time.Time) VotingOption`, `NewVotingService(q VotingQuerier, opts ...VotingOption)` | trust.go | 03-05 | 03-07, 03-14 e2e |
| `ActivityForAccount` uses `s.now()` instead of `time.Now()` | auth.go | 03-05 | |
| `type CurrencyBand string`: `CurrencyUpToDate = "up_to_date"`, `CurrencyGettingOld = "getting_old"`, `CurrencyNeedsReconfirming = "needs_reconfirming"`, `CurrencyNotYetCorroborated = "not_yet_corroborated"`, `CurrencyNotApplicable = "not_applicable"`, ordered `CurrencyBands`, `Valid()` | `internal/service/currency.go` | 03-08 | 03-14 |
| `type FadeStage string`: `FadeFresh = "fresh"`, `FadeAging = "aging"`, `FadeStale = "stale"`, `FadeStages`, `Valid()` | currency.go | 03-08 | 03-14 |
| `type CurrencySignal struct{ Band CurrencyBand; Score *int; Label, Sentence, ScoreSentence string; RecentPlaces, WindowMinutes int; FadeStage *FadeStage }` | currency.go | 03-08 | 03-14 |
| Currency constants `CurrencyWindowDivisor = 4`, `UpToDateMinScore = 75`, `GettingOldMinScore = 50`, 25 points per window; k is `IndependentAgreementThreshold` plus distinct dispute places inside the window | currency.go | 03-08 | |
| `type Outcome string`: `OutcomeCorroborated`, `OutcomeContradicted`, `OutcomeUnconfirmed`, `OutcomeNeutral`; `type JudgedOutcome struct{ Outcome Outcome; ExpiresAt time.Time }` | `internal/service/reliability.go` | 03-03 | 03-10 |
| `func ClassifyOutcome(meta ReportMeta, tally VoteTally, expiresAt, now time.Time) Outcome` (neutral when `expiresAt.After(now)`; calls `Resolve` with `ReportMeta{Severity: SeverityLow, Category: CategoryOther}` and a tally whose `ResolveCells`, `ReopenCells`, `ReporterResolved`, `ReporterReopened` are zeroed; Live maps to corroborated, Hidden to contradicted, Provisional to unconfirmed; unconfirmed on a report whose real meta has the critical bypass is neutral). Reusable by Phase 3.1. | reliability.go | 03-03 | 03-10 |
| `type ReporterTag string`: `ReporterReliable = "reliable"`, `ReporterMixed = "mixed"`, `ReporterUnreliable = "unreliable"`, `ReporterNew = "new"`, `ReporterNone = "none"`, `ReporterTags`, `Valid()` | reliability.go | 03-03 | 03-10, 03-14 |
| `type Reliability struct{ Tag ReporterTag; Score *int; Judged, Corroborated int }`, `func ScoreReliability(outcomes []JudgedOutcome, now time.Time) Reliability` | reliability.go | 03-03 | 03-10 |
| Reliability constants `ReliabilityMinJudged = 5`, `ReliabilityHalfLife = 30 * 24 * time.Hour`, `ReliabilityWindow = 90 * 24 * time.Hour`, `ReliabilityRowCap = 30`, `UnconfirmedCredit = 0.25`, `ReliableMinScore = 75`, `MixedMinScore = 35` | reliability.go | 03-03 | 03-10, 03-12 (query params) |
| `type ReporterSignal struct{ Tag ReporterTag; Score *int; Label, Sentence, ScoreSentence string; JudgedReports, CorroboratedReports int }` | `internal/service/reporter.go` | 03-10 | 03-12, 03-14 |
| `func ReporterSignals(accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time) map[int64]ReporterSignal` (every requested account gets an entry; no rows gives New reporter with 0 judged; reporter identity per history report is the account being scored) | reporter.go | 03-10 | 03-12 |
| `const MaxReporterAccountsPerRead = 200` (nearest reports first, the order `NearbyReports` returns) | report.go | 03-12 | |
| `ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)` added to `Querier` (report.go), `VotingQuerier` (trust.go), `AuthQuerier` (auth.go) and to `fakeFeedQuerier` (`feed_test.go`), `fakeVotingQuerier` (`trust_test.go`), `fakeAuthQuerier` (`auth_test.go`) | three services and three fakes | 03-12 | |
| Expected sqlc output: `ReporterHistoryReportsParams{ Now, WindowStart time.Time; RowCap int32; AccountIds []int64 }`, `ReporterHistoryReportsRow{ AccountID, ReportID int64; Severity, Category string; ExpiresAt time.Time }`. Plans 03-10 and 03-12 read the committed generated file rather than trusting this line. | `internal/store/sqlc/reliability.sql.go` | 03-06 | 03-10, 03-12 |
| Index `idx_reports_session_expires ON reports (session_id, expires_at DESC, id DESC)` in `internal/store/migrations/00005_add_reports_session_expires_index.sql` | migration | 03-06 | |
| Handler: `type TrustResponse`, `func trustToResponse(t service.Trust) TrustResponse` | `internal/api/handlers/trust.go` | 03-07 | 03-14, 03-15 |
| Handler: `type CurrencyResponse`, `type ReporterResponse` and the two fields on `TrustResponse` | handlers/trust.go | 03-14 | 03-15 (automatically through the mapper) |
| `FeedReportResponse.Trust TrustResponse` (`json:"trust"`), `CastVoteResponse.Trust TrustResponse` (`json:"trust"`), no omitempty | reports.go, votes.go | 03-07 | |
| 403 mapping: `errors.Is(err, service.ErrVoterTooFar)` branch in the sentinel group before `errors.As`, `writeFieldError(w, http.StatusForbidden, "location", service.TooFarMessage())`; no distance in the body | votes.go | 03-04 | |
| `profileReport` gains `TrustJSON string`, `SeverityWord string`, `SeverityLevel string`, `Expired bool`; `func severityWord(s service.Severity) string` (Critical, Medium, Low); `newProfileReport` sets `Expired = !row.ExpiresAt.After(now)` and the `vis-expired` class when expired and not retracted | `internal/api/handlers/auth.go` | 03-15 | |
| `PageConfig.VoteRadiusKm float64`, `pageViewModel.VoteRadiusKm`, set in `cmd/server/main.go` from `service.MaxVoteDistanceKm` | page.go, main.go | 03-18 | |

### The trust wire object (JSON key `trust`, always present, never omitted)

Every key below was checked against the name guard `(?i)cell|account|session|email`: none matches.
Integers are exact counts; the `*_text` fields are server capped numerals for the surface. Scores are
`*int` serialised as JSON `null` (no omitempty); swag documents them in the field comment and with
`extensions:"x-nullable=true"` if swag 1.16.4 accepts it (RESEARCH A5), otherwise by comment only.

| Key | Type | Rule | Plan |
|-----|------|------|------|
| `confirmed_places` | int | N, `ConfirmCells` (D-04) | 03-05 / 03-07 |
| `disputed_places` | int | M, `DisputeCells` (R-04) | 03-05 / 03-07 |
| `confirmed_text` | string | `"0"` to `"99"`, `"99+"` above 99 (A27) | 03-05 / 03-07 |
| `disputed_text` | string | same rule | 03-05 / 03-07 |
| `lead_sentence` | string | retracted: `""`; else expired (`now >= expires_at`): the expiry line; else provisional: the D-17 sentence; else `""` | 03-05 / 03-07 |
| `confirmed_sentence` | string | always set | 03-05 / 03-07 |
| `disputed_sentence` | string | always set | 03-05 / 03-07 |
| `currency.band` | enum `up_to_date`, `getting_old`, `needs_reconfirming`, `not_yet_corroborated`, `not_applicable` | `not_applicable` for Hidden, Retracted and expired; `not_yet_corroborated` when N < 2 over the whole life (R-01) | 03-08 / 03-14 |
| `currency.score` | int or null | null for `not_yet_corroborated` and `not_applicable`; 0 when contested | 03-08 / 03-14 |
| `currency.label` | string | `""` when no word shows (provisional, `not_applicable`); `Too early to tell` for critical or rescue needed with N < 2; else the band word | 03-08 / 03-14 |
| `currency.sentence` | string | `""` exactly when `label` is `""` | 03-08 / 03-14 |
| `currency.score_sentence` | string | `""` exactly when `score` is null (A50) | 03-08 / 03-14 |
| `currency.recent_places` | int | distinct confirming places inside the window | 03-08 / 03-14 |
| `currency.window_minutes` | int | quarter of `expires_at - created_at` in whole minutes | 03-08 / 03-14 |
| `currency.fade_stage` | `fresh`, `aging`, `stale` or null | null for critical or rescue needed (criticalBypass), Hidden, Retracted, `not_applicable`; else `up_to_date` and `not_yet_corroborated` give fresh, `getting_old` aging, `needs_reconfirming` stale (R-02) | 03-08 / 03-14 |
| `reporter.tag` | enum `reliable`, `mixed`, `unreliable`, `new`, `none` | `none` for accountless reports and reports past the account cap | 03-10 / 03-14 |
| `reporter.score` | int or null | null for `new` and `none` | 03-10 / 03-14 |
| `reporter.label` | string | `""` for `none` | 03-10 / 03-14 |
| `reporter.sentence` | string | `""` for `none` | 03-10 / 03-14 |
| `reporter.score_sentence` | string | `""` when score is null | 03-10 / 03-14 |
| `reporter.judged_reports` | int | J | 03-10 / 03-14 |
| `reporter.corroborated_reports` | int | C | 03-10 / 03-14 |

Feed, vote and Activity carry the same object from the same `trustToResponse`. The Activity JSON is
`json.Marshal(trustToResponse(ar.Trust))`, so it gains `currency` and `reporter` the moment 03-14
lands; 03-15's test compares against the mapper output, not a fixed key list. No account id, email,
voter cell, session id or raw vote total appears anywhere on the wire.

**Swagger name guard (03-07, extended by 03-14):** walk the properties of `handlers.TrustResponse`,
`handlers.CurrencyResponse`, `handlers.ReporterResponse`, `handlers.FeedReportResponse` and
`handlers.CastVoteResponse` and fail on `(?i)cell|account|session|email`. It must not walk every
definition: `handlers.RequestLinkRequest` and `handlers.RequestLinkResponse` legitimately carry
`email`. Keep the existing `session_id` and `reporter_account_id` substring guards.

### Server authored copy (all in `internal/service/trustcopy.go` except `TooFarMessage`)

Closed character set for every server string: ASCII letters, digits, space, `,` `.` `'` `+`, and the
hyphen only inside `re-confirming`. No report text ever enters a trust string. One test family
`TestTrustCopy...` with a shared helper `assertPlainCopy(t, s string)` enumerates a grid of inputs and
checks every produced string (created by 03-05, extended by 03-08 and 03-10). Template slots below:
T is `IndependentAgreementThreshold`, MIN is `ReliabilityMinJudged`, WINDOW is built from
`window_minutes` as `{h} hours` (or `1 hour`) when it divides by 60, else `{m} minutes` (or `1 minute`).

| Line | Case | Exact text |
|------|------|-----------|
| Lead | provisional | `Needs {T} confirmations before this counts as confirmed.` |
| Lead | expired, not retracted (Activity) | `This report has expired. It can no longer be confirmed.` |
| Confirmed | N = 0 | `No one has confirmed this yet. Counts separate places, not votes.` |
| Confirmed | N = 1 | `1 person in 1 place confirmed this. Counts separate places, not votes.` |
| Confirmed | N >= 2 (real number above 99) | `{N} people in {N} different places confirmed this. Counts separate places, not votes.` |
| Disputed | M = 0 | `No one nearby has disputed this. Counts separate places, not votes.` |
| Disputed | M = 1 | `1 person in 1 place disputed this. Counts separate places, not votes.` |
| Disputed | M >= 2 | `{M} people in {M} different places disputed this. Counts separate places, not votes.` |
| Still current | Up to date | `{R} different places confirmed this in the last {WINDOW}.` |
| Still current | Getting old, no dispute in window | `Confirmed earlier, but fewer than {T} places confirmed it in the last {WINDOW}.` |
| Still current | Needs re-confirming, no dispute in window | `Confirmed earlier, but it has gone quiet. People nearby can confirm it again.` |
| Still current | Getting old or Needs re-confirming with a standing dispute in the window | `Someone nearby disputed this, so it needs {K} places to confirm it in the last {WINDOW}.` |
| Still current | Too early to tell | `Fewer than {T} places have confirmed this so far.` |
| Score (popup) | currency score not null | `Score {SCORE} out of 100, based on how recently different places confirmed this.` |
| Reporter | Reliable, Mixed, Unreliable | `{C} of {J} earlier reports were confirmed by people nearby.` |
| Reporter | New, J >= 2 | `Only {J} of their reports have had a clear result so far. A rating needs {MIN} reports with a clear result.` |
| Reporter | New, J = 1 | `Only 1 of their reports has had a clear result so far. A rating needs {MIN} reports with a clear result.` |
| Reporter | New, J = 0 | `None of their reports has had a clear result yet. A rating needs {MIN} reports with a clear result.` |
| Score (popup) | reporter score not null | `Score {SCORE} out of 100, based on how often their reports with a clear result were confirmed.` |
| Too far | 403 for all four vote values, and the 03-18 hint | `Too far to vote. You need to be within {RADIUS} km of this report.` |

Labels: `Up to date`, `Getting old`, `Needs re-confirming`, `Too early to tell`; `Reliable reporter`,
`Mixed record`, `Unreliable reporter`, `New reporter`. Fixed literals that live in `trust.js` (not
rules): the counter words `confirmed` and `disputed`, the visually hidden prefix `, score`, and the
visually hidden hint `. Shows what these mean.`.

### Formula anchors recomputed for `c = 0.25` (03-03 table test; 03-10 docs)

`score = round(100 * (n * p + 1) / (n + 2))`, `p` the recency weighted mean credit with weight
`2^(-(now - expires_at) / 30 days)`; tags cut from the rounded integer: 75 or more Reliable reporter,
35 to 74 Mixed record, 34 or less Unreliable reporter; fewer than 5 judged is New reporter with a null
score. Fresh weights unless noted. The plan author re-derives every row with a short script and
records the script output in the SUMMARY.

| History (n) | p | Score | Tag |
|-------------|---|-------|-----|
| 5 corroborated (any ages inside the window) | 1.00 | 86 | Reliable |
| 30 corroborated | 1.00 | 97 | Reliable |
| 4 corroborated, 1 unconfirmed (5) | 0.85 | 75 (exact boundary, 5.25 / 7) | Reliable |
| 4 corroborated, 1 contradicted (5) | 0.80 | 71 | Mixed |
| 3 corroborated, 2 unconfirmed (5) | 0.70 | 64 | Mixed |
| 3 corroborated, 2 contradicted (5) | 0.60 | 57 | Mixed |
| 5 corroborated, 5 unconfirmed (10) | 0.625 | 60 | Mixed |
| 2 corroborated, 3 contradicted (5) | 0.40 | 43 | Mixed |
| 5 unconfirmed only (5) | 0.25 | 32 | Unreliable |
| 30 unconfirmed only (30) | 0.25 | 27 | Unreliable |
| 1 contradicted, 4 unconfirmed (5) | 0.20 | 29 | Unreliable |
| 2 contradicted, 3 unconfirmed (5) | 0.15 | 25 | Unreliable |
| 1 corroborated, 4 contradicted (5) | 0.20 | 29 | Unreliable |
| 5 contradicted (5) | 0.00 | 14 | Unreliable |
| 8 corroborated, 2 contradicted (10) | 0.80 | 75 (exact) | Reliable |
| 6 corroborated, 12 contradicted (18) | 0.333 | 35 (exact) | Mixed |
| corroborated at 5, 20, 50 days, contradicted at 10 days, unconfirmed at 40 days (5) | 0.6394 | 60 | Mixed |
| 5 corroborated at 80 days, 2 contradicted at 1 day (7) | 0.287 | 33 | Unreliable |
| 5 contradicted at 80 days, 2 corroborated at 1 day (7) | 0.713 | 67 | Mixed |

Invariants: **I1** a history with no contradicted and no unconfirmed outcome (5 to 30 judged, any
ages) scores at least 85.7 and is Reliable. **I2 (restated per D-23)** a history of only unconfirmed
outcomes with at least 5 judged always reads Unreliable reporter (its score `100 * (0.25 n + 1) /
(n + 2)` is 32.14 at n = 5 and falls toward 25, independent of weights). **I3** replacing an outcome
by a better class never lowers the score, and order never matters. **I4** the score lies in
`[100 / (n + 2), 100 (n + 1) / (n + 2)]`. The currency boundary and worked tables in RESEARCH
"Still current score" do not depend on `c` and stand as written.

### JavaScript exports shared across plans

| Export | Module | Created by | Used by |
|--------|--------|-----------|---------|
| `PinalertVotes.getVoterLocation(opts)` with `opts.fresh`; cache entry `{latitude, longitude, at}` in the existing sessionStorage key | `votes.js` | 03-04 | 03-18 reads the cache through `peekVoterLocation()` |
| `PinalertVotes.createVoteBlock(reportId)` returns at least `{ controls, note, error }`; consumers append `controls`, then `note`, then `error` | votes.js | 03-09 | `feed.js`, `map.js` |
| `PinalertVotes.createReopenControl(reportId)` returns `{ controls, button, error }` (no note); `PinalertVotes.setReopenLabel(button, text)` writes only the label span | votes.js | 03-09 | `activity.js` |
| `PinalertVotes.cancelResolveConfirm(container)` closes an open `.resolve-confirm` inside `container` by the Cancel path (sends nothing, focuses that block's Mark resolved) and returns true when it acted | votes.js | 03-09 | `trust.js` Escape listener |
| `applyDistanceHint(block, report)` (module private, declared after `setBlockBusy`, called from `updateVoteBlock`), `peekVoterLocation()` (private, synchronous, never prompts) | votes.js | 03-18 | |
| `PinalertTrust.createTrustBlock(options)` with `options.showScores` (popup only) returns `{ el, toggle, panel }`; `el` is `div.trust-block` and starts `hidden` | `trust.js` | 03-13 | `feed.js`, `map.js`, `activity.js` |
| `PinalertTrust.updateTrustBlock(block, trust)` hides the block when `trust` is missing or fails validation; never resets the open state | trust.js | 03-13 | same |
| `PinalertTrust.fadeStage(report)` returns the worse of `Pinalert.ageStage(report)` and `report.trust.currency.fade_stage` in the order fresh, aging, stale; returns the time stage when the server stage is absent or not in the allowlist | trust.js | 03-13 | `feed.js` (`updateRow`, `refreshAgeAndTime`), `map.js` (badge, popup classes, popup state) |
| `PinalertTrust.focusToggle(blockOrRow)` | trust.js | 03-13 | 03-15 (`activity.js` when the Reopen control is removed) |
| `PinalertTrust.createSeverityLabel()`, `PinalertTrust.updateSeverityLabel(label, report)` | trust.js | 03-16 | `feed.js` (03-16), `map.js` popup header (03-17) |
| One capture phase document `keydown` listener in `trust.js`: (a) modal or account menu open does nothing, (b) open confirmation goes through `PinalertVotes.cancelResolveConfirm`, (c) row ownership closes that row's panel and focuses its toggle | trust.js | 03-13 | 03-19 extends the same listener with popup ownership (c) and the popup close step (d); no second listener |
| `Pinalert.SEVERITY_WORDS` (`low` Low, `medium` Medium, `critical` Critical), `Pinalert.formatShelterLine(report)` (`{Status} · {N} people`, `{Status}`, `{N} person` or `{N} people`, never the word Capacity) | `app.js` | 03-16 | `feed.js` `buildMetaText` (03-16), `map.js` popup meta (03-17) |
| `PinalertVisibility.displayState(report)` (retracted, else expired when `report.expired === true`, else `visibilityState`), `VISIBILITY_TAG_LABELS.expired = 'Expired'`, `PinalertVisibility.stateWord(report)`; `VISIBILITY_STATES` stays at four values | `visibility.js` | 03-15 | `activity.js` (03-15), pin names in `map.js` (03-19) |
| `buildPopupCard(report)` (replaces `buildPopupContent(report)`, no nested function declaration before the label, title and vote block anchors) plus an in place update function | `map.js` | 03-17 | |
| `PinalertMap.focusOpenPopup()`, `PinalertMap.takeRemovedPopupFocus()`, `PinalertMap.flyTo(id, { opener: 'list' })` | map.js | 03-19 | `feed.js` (03-19) |

### CSS, DOM and data attribute contracts

- **Trust block markup** (UI-SPEC "Markup" 371 to 399): `div.trust-block` > `button.trust-block__toggle[type=button][aria-expanded][aria-controls]` > `span.trust-block__counts` (two `span.trust-block__count`, the second also `--second`, each holding `span.trust-num` plus the word), `span.trust-block__signals` (two `span.trust-signal`, second `--second`, each `span.trust-icon.trust-icon--clock` or `--user` with `aria-hidden="true"` plus `span.trust-signal__text`), hidden hint span; sibling `div.trust-block__panel[hidden]` with `p.trust-block__line` children. Panel ids `trust-panel-N` from a module counter, never the report id. Popup numbers sit in `span.trust-num` after a `span.visually-hidden` holding `, score`.
- **Vote controls:** `div.vote-controls` (two track grid), `button.vote-btn` with `span.vote-btn__icon` (mask modifiers `vote-btn__icon--check`, `--ban`, `--shield-check`, `--rotate-ccw`) and `span.vote-btn__label`; `.resolve-confirm[role=group][aria-labelledby]` with ids `pinalert-resolve-q-N`; `p.vote-note[hidden]` with id `pinalert-vote-note-N` from the same votes.js counter; `p.vote-error[role=alert]` gets no child nodes (alert glyph is `::before`). Too far: `aria-disabled="true"`, `data-too-far`, `aria-describedby` pointing at the note, only on Confirm, Dispute and Mark resolved of rows the viewer did not post.
- **Labels:** `span.severity-label.severity-label--{low|medium|critical}` (dot is `::before` on medium and low, no DOM node), `span.visibility-tag` (rounded, `visibility-tag--expired` modifier), row class `vis-expired` written only by `applyVisibilityClass` through `displayState` on the client and by the template for first paint.
- **Popup card:** `div.popup-card[tabindex=-1]` with the classes the UI-SPEC "Structure" names (`popup-card*`); dialog role and name on `popup.getElement()`; title id `pinalert-popup-title-N` from a map.js counter; `bindPopup(` keeps `className: popupStateClasses(report)`; `unbindPopup(` stays forbidden; Leaflet options `minWidth: 340, maxWidth: 340`, `popupAnchor: [0, -22]`.
- **Data attributes:** `data-trust` (JSON, attribute escaped by `html/template`, parsed with `JSON.parse` in try and catch), `data-expired` (`true` or `false`) on Activity `<li>`; `data-vote-radius-km` on `#app-shell` (feed page only; its absence means no hint); `data-too-far` on vote buttons.
- **trust.css rules:** every new rule that declares `display` carries `:not([hidden])` in its head; no hex, `rgb(`, `rgba(`, `hsl(`; no `!important`; no `999px`; no new custom property (only `--severity-current`, `--severity-tint`, `--badge-glyph-fg` may be declared there); spacing as `var(--space-*)`, 44px as `var(--touch-target-min)`; icon masks as literal `url("/static/icons/NAME.svg")` with both `-webkit-` and unprefixed mask properties; no selector head contains `.icon-glyph--`; every Leaflet popup rule keeps the `.leaflet-container` ancestor; system colour keywords only inside `@media (forced-colors: active)` blocks; narrow layout query is `(max-width: 899.98px)`; new rules go after the Phase 2 cascade block. `--shadow-popup` is declared once in `main.css` `:root` (03-17).
- **trust.css ownership by plan:** 03-11 vote buttons, `.resolve-confirm` layout, `.vote-note:not([hidden])`, the A18 and A72 `aria-disabled` and disabled looks, `.vote-error` restyle and `::before`, the four vote glyph masks; 03-13 trust block, signals, panel, toggle press and inset focus, `trust-icon` base and the clock and user masks; 03-16 severity and visibility labels, Expired modifier, `vis-expired` row treatment; 03-17 popup card, Leaflet overrides, close button and its x mask, the 20px header glyph, the popup max height calc. 03-18 and 03-19 do not edit trust.css.
- **Allowed dimension literals** (no token exists): 24px label min height, 28px and 32px row min heights, 20px and 16px icon sizes, the 4px severity bar, 340px card width, separator widths, 1px and 2px hairline and ring widths, radii, the 120px max height floor, the 20px Leaflet margin, the 359px max height breakpoint, Cancel `min-width: 96px`, Yes `min-width: 144px`. Counts separator offset `calc(-0.5 * var(--space-lg))`.
- **Script order:** feed page `app.js, votes.js, visibility.js, trust.js, map.js, modal.js, feed.js, account-menu.js` (03-13); Activity `app.js, votes.js, visibility.js, trust.js, activity.js, account-menu.js` (03-15). Each tag uses `?v={{.AssetVersion}}` and `defer`.
- **Consumer bans kept:** `feed.js` and `map.js` never type `vote-btn`, `vote-controls`, `vote-error`, `your_vote`, `is_own_report`, `visibility-tag`, `vis-provisional`, `vis-hidden`, `visibility_reason`, `report.visibility`, any `trust-` class, or `Pinalert.ageStage(`. `activity.js` keeps no `fetch(`, no `navigator.geolocation`, exactly one `addEventListener`, exactly one `Pinalert.showToast(`, no `vis-expired` literal and no date comparison. `votes.js` keeps exactly one `navigator.geolocation.getCurrentPosition(`, exactly one `fetch(`, the `castVote(reportId, action)` and `createVoteBlock(reportId)` signatures, no `Pinalert.config`, no `setView(`, none of these literals in `//` comments, and no `disabled` substring inside `applyOwnReportRule` or `updateVoteBlock` bodies.

### Threat IDs (reuse, do not mint per plan)

| ID | STRIDE | Threat | Severity | Disposition | Plans |
|----|--------|--------|----------|-------------|-------|
| T-03-01 | E | Bypass the distance check by claiming to be the reporter | high | mitigate: exemption from `rc.ReporterAccountID` versus authenticated `in.AccountID`, resolution kind only | 03-01, 03-04 |
| T-03-02 | T | NaN or Inf coordinate slips past the range check | medium | mitigate: `!(d <= radius)` comparison, tests | 03-01 |
| T-03-03 | S | Spoofed GPS defeats the cell rule and the 1 km rule | medium | accept: documented honestly, cost is extra verified accounts plus a claimed location | 03-02, 03-20 |
| T-03-04 | I | Voter cell, account id, email or raw totals leak on the wire, in swagger or in `data-trust` | high | mitigate: counts and server strings only, scoped name guard, runtime body test, same mapper for Activity | 03-07, 03-14, 03-15 |
| T-03-05 | D | History read amplified by a 50 km feed with many reporters every 30 s | medium | mitigate: `LIMIT 30` per account, index, 200 account cap, one vote read | 03-06, 03-12 |
| T-03-06 | T | Stored XSS through server strings, `data-trust` or popup content | high | mitigate: strings from constants and integers only, `Pinalert.setText` only, attribute escaping, `JSON.parse` in try and catch, enum allowlists in `trust.js` | 03-05, 03-13, 03-15, 03-17 |
| T-03-07 | T | Long dash or unexpected characters in server copy | low | mitigate: `TestTrustCopy...` grid with `assertPlainCopy` | 03-05, 03-08, 03-10 |
| T-03-08 | R | Grandfathered far away votes from before this phase inflate old records | low | accept: documented, tests on fresh databases | 03-02 |
| T-03-09 | E | The browser hint treated as an authority | medium | mitigate: hint only toggles `aria-disabled`; server refuses regardless (D-01) | 03-18 |
| T-03-10 | S | Reliability keyed to a session so one person splits or merges identity | high | mitigate: keyed on `sessions.account_id` across sessions, tests | 03-06, 03-12 |
| T-03-11 | T | Self resolving erases a bad record | medium | mitigate: resolution fields zeroed before classification, self resolve neutral | 03-03 |
| T-03-12 | I | A public rating links one person's reports | low | accept: intended by D-10; no account id or email exposed | 03-10 |

### File ownership (creating plan, then later editors, by wave)

Same wave plans never share a file. New test files are plan owned so existing contract test files are
edited only where an existing expectation must change.

| File | Created or first edited by | Later editors |
|------|---------------------------|---------------|
| `internal/service/geo.go`, `geo_test.go` | 03-01 (new) | none |
| `internal/store/queries/votes.sql`, `internal/store/sqlc/votes.sql.go`, `internal/store/votes_test.go` | 03-01 | none |
| `internal/service/trust.go` | 03-01 | 03-05 (w2), 03-12 (w5) |
| `internal/service/trust_test.go` | 03-01 | 03-12 |
| `internal/api/handlers/votes_e2e_test.go` | 03-01 | 03-04 |
| `internal/api/handlers/feed_visibility_e2e_test.go` | 03-01 | none |
| `internal/service/geohash_precision_test.go` | 03-02 (new) | none |
| `docs/TRUST-MODEL.md` | 03-02 (new) | 03-08 (w3), 03-10 (w4), 03-20 (w9) |
| `README.md` | 03-02 | 03-20 |
| `internal/service/reliability.go`, `reliability_test.go` | 03-03 (new) | none |
| `internal/api/handlers/votes.go` | 03-04 | 03-07 |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | 03-04 | 03-07, 03-14 |
| `web/static/js/votes.js` | 03-04 | 03-09 (w3), 03-18 (w6), 03-17 (w7, deletes the accepted limitation comment per A19) |
| `web/votes_contract_test.go` | 03-04 | 03-11 (A13, A14), 03-17 (A10, A20, A21) |
| `internal/service/trustview.go`, `trustcopy.go`, `trustcopy_test.go` | 03-05 (new) | 03-08, 03-10 |
| `internal/service/trustview_test.go` | 03-05 (new) | 03-08 |
| `internal/service/trust_paths_test.go` (package `service_test`) | 03-05 (new) | none |
| `internal/service/feed.go` | 03-05 | none |
| `internal/service/report.go`, `internal/service/auth.go` | 03-05 | 03-12 |
| `internal/store/migrations/00005_add_reports_session_expires_index.sql`, `internal/store/queries/reliability.sql`, `internal/store/sqlc/reliability.sql.go`, `internal/store/reliability_test.go`, `internal/testutil/history.go` | 03-06 (new) | none |
| `internal/store/migrations_test.go` | 03-06 | none |
| `internal/api/handlers/trust.go`, `trust_e2e_test.go` | 03-07 (new) | 03-14 |
| `internal/api/handlers/reports.go` | 03-07 | none |
| `internal/api/handlers/swagger_test.go` | 03-07 | 03-14 |
| `internal/service/currency.go`, `currency_test.go` | 03-08 (new) | none |
| `web/static/js/activity.js` | 03-09 | 03-15 |
| `web/static/js/feed.js` | 03-09 | 03-13 (w5), 03-16 (w6), 03-19 (w8) |
| `web/static/js/map.js` | 03-09 | 03-13 (w5), 03-17 (w7), 03-19 (w8) |
| `web/vote_controls_contract_test.go` | 03-09 (new) | none |
| `internal/service/reporter.go`, `reporter_test.go` | 03-10 (new) | none |
| `web/static/css/trust.css` | 03-11 | 03-13 (w5), 03-16 (w6), 03-17 (w7) |
| `web/static/icons/check.svg`, `ban.svg`, `shield-check.svg`, `rotate-ccw.svg` | 03-11 (new) | none |
| `web/css_contract_test.go` (`nonCategoryIcons`, `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon`) | 03-11 | 03-13 (clock), 03-17 (x) |
| `web/vote_style_contract_test.go` | 03-11 (new) | none |
| `internal/service/feed_test.go`, `auth_test.go` | 03-12 | none |
| `web/static/js/trust.js` | 03-13 (new) | 03-16 (w6), 03-19 (w8) |
| `web/templates/index.html.tmpl` | 03-13 | 03-18 |
| `web/static/icons/clock.svg` | 03-13 (new) | none |
| `web/trust_contract_test.go` | 03-13 (new) | 03-19 |
| `internal/api/handlers/auth.go`, `web/templates/profile.html.tmpl`, `web/static/js/visibility.js`, `internal/api/handlers/activity_e2e_test.go` | 03-15 | none |
| `web/activity_trust_contract_test.go` | 03-15 (new) | none |
| `web/static/js/app.js` | 03-16 | none |
| `web/static/css/main.css` | 03-16 (A42, A55) | 03-17 (`--shadow-popup`) |
| `web/labels_contract_test.go` | 03-16 (new) | none |
| `web/static/icons/x.svg` | 03-17 (new) | none |
| `web/js_contract_test.go` | 03-17 (A19) | none |
| `web/popup_contract_test.go` | 03-17 (new) | 03-19 |
| `internal/api/handlers/page.go`, `page_test.go`, `cmd/server/main.go` | 03-18 | none |
| `web/vote_hint_contract_test.go` | 03-18 (new) | none |
| `.planning/phases/01-foundation-report-map/01-UI-SPEC.md`, `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md`, `.claude/skills/sketch-findings-pinalert/sources/REVIEWS.md` | 03-20 | none |

If a plan author finds by grep that an anchor it changes is asserted by an existing test in a file
not listed for that plan, the plan adds that file to `files_modified` only when no same wave plan
lists it (checked against this table), otherwise it raises the conflict instead of editing.

### Pinned test names (plan authors may add tests, not rename these)

- 03-01: `TestHaversine`, `TestWithinVoteRadius`, `TestWithinVoteRadiusBoundary`, `TestHaversineInsideBoundingBox`, `TestTooFarMessage`, `TestCastVoteRejectsTooFar`, `TestCastVoteCheckOrder`, `TestCastVoteReporterExemption`, `TestReportVoteContextReturnsCoordinates`.
- 03-02: `TestGeohashPrecisionWitnesses`.
- 03-03: `TestClassifyOutcome`, `TestClassifyOutcomeMatchesResolve`, `TestReliabilityTable`, `TestReliabilityInvariants`, `TestReliabilityDecayAndWindow`.
- 03-04: `TestCastVoteTooFar`, `TestVoteTransportRefreshesAndRetriesOnce`; edited `TestVoterLocationIsCachedPerSession`, `TestVoteTransportHasNoLocationFallback` stays green.
- 03-05: `TestBuildTrustCountsMatchTally`, `TestTrustCountLabel`, `TestTrustCopyIsPlainText`, `TestTrustLeadSentence`, `TestCastVoteResultCarriesTrust`, `TestTrustAttachedOnEveryReadPath`.
- 03-06: `TestReporterHistoryQuerySourceHasExpectedShape`, `TestSessionExpiresIndexExists`, `TestReporterHistoryUsesSessionIndex`, `TestReporterHistoryBounds`, `TestReporterHistoryUsesAccountNotSession`; edited `TestMigrationsEmbedded`.
- 03-07: `TestFeedTrustMatchesCastVoteTrust`, `TestTrustCountsDistinctPlacesNotVotes`, `TestTrustResponseOmitsIdentifiers`, `TestTrustCountUnderConcurrentVotes` (run with `-race`); edited `TestSwaggerSpecCoversRoutes`; helper `newTrustE2EServer(t, clock func() time.Time)` in `trust_e2e_test.go`.
- 03-08: `TestCurrencyScoreBoundaries`, `TestCurrencyNotYetCorroborated`, `TestCurrencyRetapCannotRefresh`, `TestCurrencyDisputes`, `TestCurrencyWindowFollowsLifetime`, `TestCurrencyMonotone`, `TestFadeStageMapping`, `TestTrustCopyCurrencyIsPlainText`.
- 03-09: `TestVoteButtonsCarryIconAndLabelSpans`, `TestResolveConfirmIsANamedGroupWithCancelFirst`, `TestReopenControlIsSharedFromVotes`, `TestVoteBlockReturnsNoteAndConsumersAppendInOrder`, `TestVoteErrorGetsNoChildNodes`, `TestEscapeCancelHookIsExported`.
- 03-10: `TestReporterSignalsGroupByAccount`, `TestReporterSignalsNewAndNone`, `TestTrustCopyReporterIsPlainText`.
- 03-11: `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon`, `TestPressedVoteStatesWinEveryCombination`, `TestVoteErrorAlertGlyph`, `TestVoteErrorContrastAcrossThemes`, `TestDestructiveYesHeadWins`; edited `TestVoteButtonsMeetTouchTargetAndUseTokensOnly`, `TestResolveControlsMeetTouchTargetAndUseTokensOnly`, `TestCategoryGlyphMaskRulesCoverEveryCategory` (via `nonCategoryIcons`).
- 03-12: `TestNearbyBatchesVoteReadsOnceWithReporters`, `TestNearbyReporterAccountCap`, `TestNearbyRatingNeverChangesVisibilityOrOrder`, `TestCastVoteTrustMatchesNearbyTrust`, `TestReporterSignalKeyedByAccount`.
- 03-13: `TestTrustModuleLoadsBeforeItsConsumers`, `TestTrustBlockMountedOnBothSurfaces`, `TestFadeUsesCombinedStage`, `TestTrustDisclosure`, `TestTrustCSS`, `TestTrustModuleHasNoThresholdsOrSinks`.
- 03-14: `TestFeedSignalsMatchCastVoteSignals`, `TestReporterSignalFromSeededHistory`.
- 03-15: `TestProfileRowsCarryTrustJSON`, `TestProfileMarksExpiredReports`, `TestProfileRendersSeverityLabel`, `TestActivityMountsTheTrustBlock`, `TestActivityReadsExpiredFromTheServer`, `TestDisplayStateDecidesExpired`, `TestProfileTemplateUsesServerSeverityAndTrust`.
- 03-16: `TestReadOnlyLabelsRoundedControlsNot`, `TestSeverityLabelHasNoFixedHeight`, `TestLabelAndMetaContrastAcrossThemes`, `TestExpiredRowTreatmentMatchesRetracted`, `TestRowTitleWrapsLongWords`, `TestRowSeverityLabelMountsFirst`.
- 03-17: `TestPopupCardBuiltOnceUpdatedInPlace`, `TestPopupHeaderGlyphIs20px`, `TestPopupCloseButtonComesFirst`; edited `TestMapPopupCarriesReportStateClasses`, `TestBothSurfacesMountTheSameVisibilityTag`, `TestMapPopupSurfaceIsThemeAware`, `TestMapPopupTintTracksFeedRowAcrossStatesAndThemes`.
- 03-18: `TestPageShellCarriesVoteRadius`, `TestVoteHint`, `TestVoteRadiusReadWithoutConfig`.
- 03-19: `TestPopupDialogFocusAndOpener`, `TestPinOpensOnSpace`, `TestPopupResizeUpdatesOnlyWithoutFocus`, `TestPopupPaddingTokensAgree`, `TestRenderRowsRemovesBeforePlacing`; extended `TestTrustDisclosure`.

## Coverage map

### Decisions and derived rules

| ID | Plan(s) | Note |
|----|---------|------|
| D-01 | 03-01, 03-04, 03-02 | service refusal, 403 mapping, grandfathering in docs |
| D-02 | 03-04 | silent refresh, retry once, never prompts per vote |
| D-03 | 03-02, 03-01 | witness test and docs; constant comment closes A3 |
| D-04 | 03-05, 03-07 | N is `ConfirmCells` via the shared row filter |
| D-05 | 03-08 | 2 distinct places in a quarter of the lifetime |
| D-06 | 03-08 | not yet corroborated, no fade, no negative word |
| D-07 | 03-08, 03-13 | server fade stage for non critical rows; browser worse of two |
| D-08 | 03-08, 03-10, 03-14, 03-20 | band, facts, integer score; docs record how "score" is met |
| D-09 | 03-13, 03-17 | numbers in the popup only |
| D-10 | 03-06, 03-12, 03-10 | keyed to `sessions.account_id`; public tag |
| D-11 | 03-03 | expired only, classifier through `Resolve` |
| D-12 | 03-03 | critical and rescue needed unconfirmed is neutral |
| D-13 | 03-03, 03-06, 03-12 | min 5, half life 30 days, 90 days, cap 30 |
| D-14 | 03-12, 03-13 | display only, never hides, dims, reorders |
| D-15 | 03-03, 03-10 | four tags in everyday words, cutoffs |
| D-16 | 03-05, 03-13 | two counters always shown (replaced D-16) |
| D-17 | 03-05, 03-13 | Unconfirmed chip wording kept; its sentence is panel line 1 |
| D-18 | 03-13, 03-08 | two lines, words; icons per A4 |
| D-19 | 03-05, 03-08, 03-10, 03-13 | sentences on the server; delivered through the one panel per A5 |
| D-20 | 03-13, 03-15, 03-17 | one block on feed row, popup, Activity |
| D-21 | 03-05, 03-07, 03-14 | one builder, one wire object |
| D-22 | 03-18 | hint with visible sentence per A8 |
| D-23 | 03-03, 03-10 | `c = 0.25`, I2 restated, anchors in docs |
| R-01 | 03-08 | null score below 2 places over the whole life |
| R-02 | 03-08, 03-13 | fade mapping, browser holds no thresholds |
| R-03 | 03-08, 03-13 | Hidden rows show reporter tag only |
| R-04 | 03-05 | M is `DisputeCells` |
| R-05 | 03-03, 03-06, 03-12 | judged once `expires_at <= now`, unweighted minimum |
| R-06 | 03-03 | out disputed critical judged contradicted for rating only |
| R-07 | 03-13 | number after its word, no "/100", no icon for the number |
| R-08 | 03-15, 03-08 | Activity block, Reopen refresh, `not_applicable` for expired and retracted |

### Requirements

| ID | Plan(s) |
|----|---------|
| TRUST-05 | 03-01, 03-02, 03-04, 03-05, 03-07, 03-09, 03-11, 03-13, 03-15, 03-16, 03-17, 03-18, 03-19, 03-20 |
| TRUST-07 | 03-03, 03-06, 03-08, 03-10, 03-12, 03-13, 03-14, 03-15, 03-16, 03-17, 03-19, 03-20 |

### Validation rows

| Row | Plan | Note |
|-----|------|------|
| V-01, V-02, V-03 | 03-01 | |
| V-04, V-05, V-06 | 03-01 | |
| V-07 | 03-04 | |
| V-08 | 03-01 | voter C moved to (12.9686, 77.5946) at the five call sites |
| V-09 | 03-01 | also asserts `created_at` |
| V-10 | 03-02 | |
| V-11, V-12 | 03-05 | V-12 now tests the two counters (D-16) |
| V-13 to V-19 | 03-08 | |
| V-20 to V-23 | 03-03 | V-22 uses the restated I2 |
| V-24 to V-27 | 03-06 | |
| V-28 | 03-06 (store), 03-12 (service) | |
| V-29 | 03-07 (counts), 03-12 (batching), 03-14 (signals) | |
| V-30 | 03-07, 03-14 | name guard scoped per contracts |
| V-31 | 03-07, re-asserted in 03-14 | |
| V-32 | 03-05, extended by 03-08 and 03-10 | charset includes `+` (A27) |
| V-33 | 03-15 | |
| V-34 | 03-18 | |
| V-35 | 03-04 | |
| V-36 | 03-18 | includes the A8 clearing assertions |
| V-37 | 03-13 | |
| V-38 | 03-13 (feed and popup), 03-15 (Activity) | |
| V-39 | 03-13, extended by 03-19 | renamed `TestTrustDisclosure` (A5) |
| V-40 | 03-13 (`TestTrustCSS`), 03-16 (narrowed radius test, A1, A38) | |
| V-41 | 03-07 | |
| V-42 | 03-20 | manual UAT, `<verify><human-check>` in an auto task |

### UI-SPEC amendments

| ID | Plan(s) | ID | Plan(s) | ID | Plan(s) |
|----|---------|----|---------|----|---------|
| A1 | 03-16 | A26 | 03-16 (feed), 03-15 (Activity) | A51 | 03-19 |
| A2 | 03-17 | A27 | 03-05 | A52 | 03-09 |
| A3 | 03-13 | A28 | 03-11, 03-09 | A53 | 03-11, 03-13, 03-16, 03-17 |
| A4 | 03-13 | A29 | 03-15 | A54 | 03-17 |
| A5 | 03-13, 03-05 | A30 | 03-17 | A55 | 03-16 (owner kept it) |
| A6 | 03-05, 03-13 | A31 | 03-17 | A56 | 03-05 |
| A7 | 03-05 | A32 | 03-17, 03-20 | A57 | 03-15 |
| A8 | 03-18, 03-09 (note node) | A33 | 03-13 | A58 | 03-15 |
| A9 | 03-17 | A34 | 03-09 | A59 | 03-19 |
| A10 | 03-17 | A35 | 03-11 | A60 | 03-19 |
| A11 | 03-17 | A36 | 03-09 | A61 | 03-13 |
| A12 | 03-09 (spans), 03-11 (glyphs) | A37 | 03-16 | A62 | 03-17 |
| A13 | 03-11 | A38 | 03-16 | A63 | 03-09 |
| A14 | 03-11 | A39 | 03-11, 03-13, 03-17 | A64 | 03-09, 03-15 |
| A15 | 03-11 | A40 | 03-20 (pointers), colours in 03-11, 03-16, 03-17 | A65 | 03-17 (CSS), 03-19 (JS, token test) |
| A16 | 03-09 (DOM, focus), 03-11 (layout) | A41 | 03-11 | A66 | 03-09 |
| A17 | 03-11 | A42 | 03-16 | A67 | 03-19 |
| A18 | 03-11 | A43 | 03-11 | A68 | 03-19 |
| A19 | 03-17 | A44 | 03-15 (server, template), 03-16 (CSS) | A69 | 03-13 |
| A20 | 03-17 | A45 | 03-16 (rows), 03-19 (pin name exception) | A70 | 03-19 |
| A21 | 03-17 | A46 | 03-11 | A71 | 03-19 |
| A22 | 03-13 | A47 | 03-13, 03-10 | A72 | 03-11 |
| A23 | 03-13 (listener, rows), 03-19 (popup) | A48 | 03-13 | A73 | 03-11 (CSS), 03-09 (no child nodes) |
| A24 | 03-16 (formatter), 03-17 (popup meta) | A49 | 03-19 | A74 | 03-19 |
| A25 | 03-17 | A50 | 03-08, 03-10, 03-13 | | |

Resolved by rule: RR-1 03-05; RR-2 03-05 and 03-13; RR-3 03-18; RR-4 03-17; RR-5 03-17 (popup
title) with A55 in 03-16; RR-6 03-17; RR-7 03-11, 03-13, 03-16, 03-17 (each adds forced colours rules
for what it styles); RR-8 03-09 and 03-11.

**V-42 manual script (03-20, on `p3_uat` at `PORT=8792` with fresh accounts):** panel opens by click,
tap, Enter and Space and closes by Escape on a row and in a popup (A5); screen reader reads the toggle
name with the `. Shows what these mean.` hint; every A70 Escape case; too far hint with a DevTools
Sensors location more than 1 km away, then a forced vote refused with the same sentence (A8, D-01);
Reopen on the Activity page refreshes the block (R-08); 320px width wrapping of counters, signals and
a long unbroken title at the 236px feed body and 188px Activity body (A22, A55); 200 percent text on
the labels; popup centring at 320px and 360px in list view (A59); close button and toggle never
covered after autopan at 320x568 and 320x256 (A60, A65); reduced motion with no animated pan and the
rotation across 900px (A68); `focusOpenPopup` from a list row at phone width (A51); keyboard only Mark
resolved never leaves focus on the body (A52); poll removal and poll reorder keep focus (A67, A71);
opener return to the row or the marker (A74); 600px row and 900px two pane grid widths (A17); the
toggle press fill on a Provisional row (A48); panel discoverability (open item 2); forced colours
(RR-7); light and dark themes. The owner's own dev database needs `make migrate` after this phase
merges; the executor never touches it.

## Open questions resolved during planning

RESEARCH.md Q1 and Q3 were already resolved by the owner (D-23 and the replaced D-16).

- RESOLVED Q2: cutoffs as the researcher recommended. Reporter tags from the rounded score: 75 or more Reliable reporter, 35 to 74 Mixed record, 34 or less Unreliable reporter. Still current bands: 75 or more Up to date, 50 to 74 Getting old, 49 or less Needs re-confirming. With `c = 0.25` the anchors table in this outline replaces RESEARCH's numeric table.
- RESOLVED Q4: SQL caps the latest 30 expired reports per account inside the 90 day window, not 30 judged reports; the deviation from D-13's literal wording is documented in `docs/TRUST-MODEL.md` by 03-10. The two readings differ only for reporters with many critical or rescue needed reports that expired unconfirmed.
- RESOLVED Q5: Hidden, Retracted and expired reports get currency band `not_applicable`, score null, empty label and sentence, and fade stage null.
- RESOLVED Q6: a dedicated sentinel `ErrVoterTooFar` mapped to HTTP 403 with field `location` and `TooFarMessage()`; the client retries once only on `field === "location"`.
- RESOLVED Q7: `MaxReporterAccountsPerRead = 200`, counted over the nearest reports first in the order `NearbyReports` returns them; reports by accounts past the cap get reporter tag `none` and show no tag.

UI-SPEC open items, defaults applied:

1. No distance in the popup meta (A31).
2. No chevron on the panel toggle; discoverability is a V-42 check, and a cue below the signals is added only if that check fails.
3. The server authors the capped `99+` numerals (`confirmed_text`, `disputed_text`); no cap lives in JS.
4. Still current wording is final as pinned in "Server authored copy" (the UI-SPEC drafts verbatim, with WINDOW built from `window_minutes`).
5. Resolving contrast CSS tests are added: labels and row meta in 03-16, `.vote-error` in 03-11, all four theme sources.
6. Location freshness: a cached fix younger than 60 seconds is reused without asking; otherwise a granted permission triggers a silent fresh fix, and the stale fix is reused when the refresh fails; the too far hint applies only with a fix younger than 120 seconds, so a stale fix never disables a button.
7. The two popup score sentences are kept (A50).
8. Popup autopan bottom padding stays 84px (A60).
9. A55 is kept (owner, 2026-10-04); no known gap is recorded.
10. Corrections applied: the 8px severity dot is a `::before` on `.severity-label--medium` and `.severity-label--low` (03-16); the narrow query is `(max-width: 899.98px)` and map.js tests it with `matchMedia` on `popupopen` and in the resize handler (03-17, 03-19); the extra allowed dimension literals are listed in the CSS contracts (120px floor, 20px Leaflet margin, 359px breakpoint, Cancel 96px, Yes 144px, `popupAnchor` -22 as half the 44px pin); R-07's "no icon" applies to the number itself (03-13); system colour keywords only inside forced colours blocks (`TestTrustCSS`, 03-13); `PinalertVisibility.stateWord(report)` is the one accessor for the pin name state word (03-15, used by 03-19); the counts separator offset is `calc(-0.5 * var(--space-lg))` (03-13); the focal point, Q2 scope note and A68 pointer corrections are wording in the UI-SPEC and need no code.

Other planning decisions recorded here:

- `ReportVoteContext` also selects `r.created_at`, so `CastVote` builds the same currency window as the feed with no second query.
- One too far sentence serves all four vote values, because the UI-SPEC Copywriting Contract fixes a single too far error state; RESEARCH's neutral variant for resolve and reopen is not used.
- The swagger name guard walks only the trust, feed and vote response definitions (the auth definitions carry `email` by design).
- `TooFarMessage` lives in `geo.go` next to the constant it formats, and the copy test grid includes it.
- Clock options are named `WithReportClock` and `WithVotingClock` so they do not collide with the existing `WithClock`.

## OUTLINE COMPLETE
