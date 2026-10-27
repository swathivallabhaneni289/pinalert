# Phase 9: Followable reporter profiles - Research

**Researched:** 2026-10-25
**Domain:** Go 1.26 / chi / pgx + sqlc / goose, server rendered html/template + vanilla JS (no build step). Trust model change, public profiles, a social graph (follow, mute, block), tapped star ratings, and a new notification kind on the Phase 3.2 bell and Phase 3.3 phone channel.
**Confidence:** HIGH on what the code does today (every pin below was read from the working tree on main at commit 631de22). MEDIUM on the shape of Phases 7.1 and 8, which have no plans yet (see "What this phase needs from 7.1 and 8"). The design recommendations are marked ASSUMED where they are mine, and are collected in the Assumptions Log.

<user_constraints>
## User Constraints (from CONTEXT.md)

Copied from `09-CONTEXT.md`. The planner must honor these. Nothing in this file reopens them.

### Locked Decisions

**How to read the status tags**
- **OWNER DECIDED**: the owner said it in this round. Do not re-ask.
- **LOCKED EARLIER**: decided in the ROADMAP Phase 9 section or an earlier phase. Do not re-ask.
- **CLAUDE'S DISCRETION**: the owner was not asked. Build the default named here unless the owner vetoes it.

**Carried from the ROADMAP (locked)**
- **D-01 [LOCKED EARLIER]:** Profiles are opt in ("Let others follow me", off by default), and only a person with the Reliable reporter tag can be followed. People appear by their Phase 8 username (unique, public); the Name is a profile only detail; the email is never shown.
- **D-02 [LOCKED EARLIER]:** Stars: below 5 raters they come from report outcomes only (the Phase 3 score); from 5 raters on, half outcomes and half the tapped average, both parts shown beside the total. Readers tap one to five stars on a reporter's profile: verified account only, one rating per account per reporter and changeable, never your own, only from accounts that voted on at least one of that reporter's reports, shown only with 5 or more raters. A skippable star box appears right after "Yes, mark resolved". The plan says how a tapped rating is removed or ignored when abuse is found.
- **D-03 [LOCKED EARLIER]:** Followers and Following lists are public (owner chose this over the advice to keep them private). A person without their own opted in profile appears only as a number ("and 3 others"), never by name, and cannot be tapped.
- **D-04 [LOCKED EARLIER]:** A reporter's profile opens from their name in the report's trust block, shown only for opted in reporters. My profile sits in the laptop person menu and on the phone in Settings, About you. Let others follow me, Following, Followers and Blocked people are rows in Settings, People and reports.

**The Reliable reporter tag**
- **D-05 [OWNER DECIDED, 2026-10-17, chosen over the advice of 4 stars]:** The Reliable reporter tag starts at **3.5 stars**, on the combined stars of D-02. Stars are the Phase 3 reliability score (0 to 100) divided by 20, shown with one decimal. This loosens today's bar (`ReliableMinScore = 75` in `internal/service/reliability.go`, about 3.75 stars) to 70; the owner was told a few reporters near the line will gain the tag. The other tags keep their meaning: Mixed record from today's `MixedMinScore = 35` (1.75 stars) mapped the same way, and New reporter below 5 judged reports, never shown as a good tag (Phase 3 D-13, D-15).

**Where you see the people you follow**
- **D-06 [OWNER DECIDED, 2026-10-17]:** The Instagram way, on profile pages, not on Nearby and not in My activity:
  - Every profile shows **Followers** and **Following** with their counts at the top; tapping either opens the list.
  - Tapping a name in a list opens that person's page with the reports they posted recently. It is the same kind of page as your own profile ("just like how you'll have your profile page and all your posts").
  - Each report on a profile shows its distance from you (SC3).
  This replaces ROADMAP SC3's suggestion of a choice in the Nearby Show menu.
- **D-07 [CLAUDE'S DISCRETION]:** "Recently" covers the last 30 days, newest first: live reports appear as they do on the map; finished ones show at a coarse location (finished Rescue reports keep 03.1 D-23). Tapping a report opens it on the report page (Phase 8).

**Being told when someone you follow posts**
- **D-08 [OWNER DECIDED, 2026-10-18]:** A small **bell** sits on a followed person's profile, beside the Following button, as on Instagram. It appears as soon as you follow someone. It is **one on/off: Notify me**, off by default ("It's just one note"). There is no separate buzz option.
- **D-09 [OWNER DECIDED, 2026-10-18]:** With the bell on, you get a quiet note when they post, marked "Not confirmed yet" until people nearby confirm it. This uses the same rule as the Reliable reporter alert, which Phase 3.2 builds as one named predicate (03.2 D-29, owner amendment of 2026-10-18): the note can go as soon as the report is listable (Live or Provisional), never while it is Hidden, Withheld or Retracted, and one note per account and report. On the phone it is one silent notification, updated in place when the report is confirmed (03.3 amendment of 2026-10-18). This replaces ROADMAP SC4's "critical immediately, other reports only once Live".
- **D-10 [OWNER DECIDED, 2026-10-18]:** No buzzes anywhere in the project: every phone notification is silent, no sound and no vibration (iPhone's own settings decide the sound; the site can only ask for silence).
- **D-11 [OWNER ACCEPTED, 2026-10-18]:** The per person bell replaces the general "Someone I follow posts" switch that Phase 3.3 planned for Settings, so the choice lives in one place. Settings keeps the device's on/off switch, "On this device" in the Notifications section, which still governs every phone notification.

### Claude's Discretion
- **D-12:** Mute and block. Mute stops every note from that person; their reports still show everywhere, because hiding emergency reports is never safe. Block also stops them following you and removes you from their Following, hides your profile from them, and hides their comments from you (Phase 8); their reports still show in the feed. Each person's profile has a Remove follower action for your own followers, and switching off "Let others follow me" or losing the Reliable tag ends all follows and pending notes at once (SC5). The old "remove all followers without seeing who they are" wording no longer applies, because the lists are public (D-03).
- **D-13:** The profile header in order: round picture, username, name, stars with both parts beside the total (D-02), the reporter tag with its counts ("7 of 9 earlier reports were confirmed by people nearby"), member since, Followers and Following, then Follow (or Following) with the bell beside it. Shown to the owner as a mockup at the UI step; the owner judges by looking.
- **D-14:** Rating abuse: a moderator can remove a tapped rating; ratings from an account later removed for abuse stop counting; burst detection (TRUSTX-03) stays a named dependency, recorded in `docs/TRUST-MODEL.md` with the lower 3.5 bar (D-05) as a reason it matters more.

### Deferred Ideas (OUT OF SCOPE)
None new in this round.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PROF-01 | Opt in public profile ("Let others follow me"), off by default, Reliable reporters only (amended: username replaces nickname, Name and Home area from step 4; profile still opt in) | Data model (accounts flag, account_follows), opt-in preconditions and Name screening (Part 3 and Part 8), "What this phase needs from 7.1 and 8" |
| PROF-02 | Profile shows tag with counts, member since, recent finished reports at a coarse location; amended: tapped stars with guard rails, public Followers and Following lists | Part 1 (stars arithmetic and every pin), Part 2 (ratings), Part 4 (profile read and coarsening) |
| PROF-03 | Follow, unfollow, mute, block; get a note when they post; switching off or losing the tag ends all follows and pending alerts | Part 3 (follows, mute, block), Part 5 (notes, sweep, send time recheck), Part 6 (Settings rows) |

REQUIREMENTS.md text for PROF-01 to PROF-03 is stale in several places (nickname, "no follower counts", "non-critical only once Live", "privately"). The amendments are in the ROADMAP and in 09-CONTEXT. The plans' `must_haves` must come from 09-CONTEXT, and the orchestrator should refresh REQUIREMENTS.md at planning. [VERIFIED: .planning/REQUIREMENTS.md lines 257 to 276]
</phase_requirements>

## Project Constraints (from CLAUDE.md and memory)

- Stack is fixed: Go, `go-chi/chi/v5`, `jackc/pgx/v5` + `sqlc` (committed generated code, `sqlc generate`), goose migrations run as an explicit step, `html/template` + vanilla JS, no build step, no PostGIS. [CITED: .claude/CLAUDE.md]
- GSD workflow: all file changes happen inside a GSD command.
- Owner rules from project memory that bind this phase: never use ports 8080 or 8090; never test against `pinalert_test` or `pinalert` (the suite truncates it), use a disposable database and tell every subagent; no dashes in prose, copy or comments (em and en dashes; a standing test scans templates); no images in the repo; Go is the main language and no Python anywhere; no buzzes, every phone notification silent; no underlined text links (actions are real buttons); phone and laptop carry the same features, only placement differs; reference pictures are reproduced as drawn; do not rewrite or force push history; migrations take the next free number at execution time and plans never hardcode one.
- The sketch skill `sketch-findings-pinalert` holds the validated visual direction for popup, feed rows and the trust block. The UI step for the profile page (D-13 mockup) should load it.

## Summary

Phase 9 changes the project's core trust mechanic and adds a social layer on top of it. Three things make it larger than the CONTEXT suggests.

First, the Reliable bar is computed, not stored, so moving it to 70 and letting tapped stars feed it is a change to one function plus every place that calls it. `ReporterSignals` has five production callers (feed, vote response, history, Activity, push dispatcher) and all five must read the same ratings or the feed, the popup, the Only Reliable tick, the bell and push will disagree. The fix is one optional input on `ReporterSignals` and one shared loader, not a second rule. At least four existing test fixtures at score 71 flip from Mixed to Reliable, and `docs/TRUST-MODEL.md` pins 75 in five places. The star display must truncate (score 69 is 3.45 stars and must not read 3.5 beside a Mixed tag).

Second, the follow notification is not a Settings switch plus a query. It is a new bell kind (`updates_kind_valid`), a new push kind (`push_outbox_kind_valid`, `uq_push_outbox_first`, the worker's distance check, the hourly cap list, the confirmed update join), a new line of lock screen copy, and a sweep that ends follows when the tag falls away, because the tag is never stored and decays with time. Phase 3.3 already reserved the name `follow` and left the Settings page without the row, so the removal D-11 asks for is already done in code; what is left is to widen the closed lists Phase 3.3 and Phase 5 shaped.

Third, naming and routing collide with things that already exist: `/profile` is My activity (`profile.html.tmpl` is the Activity page, not a profile), "follow" already means following a report (`ViewerFollows`, `FollowedOpenReports`), `report_mutes` is about reports, and the person menu test pins exactly two items. New names must be person scoped.

**Primary recommendation:** Build in this order: (1) a pure `stars` core plus the 70 bar and fixture/doc updates as a standalone first plan; (2) one schema migration and sqlc queries for opt in, follows, mutes, blocks and ratings; (3) ratings wired into `ReporterSignals` through one option and one loader used by all five callers; (4) profile read, `/u/{username}` page, lists and Settings rows; (5) the "See this reporter" link and the star box after resolve; (6) follow notes (bell kind, push kind, worker, sweep) last, because they are the first thing to cut.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Reliable tag cut at 70 on combined stars | API / Backend (`internal/service`, pure functions) | none | The tag is server authored and never recomputed in the browser (TRUST-02). [VERIFIED: internal/service/filter comment reliablefilter.go lines 1 to 20, web/static/js has no score threshold, grep] |
| Star rating write and eligibility | API / Backend | Database (EXISTS over votes, reports, sessions) | Eligibility is a trust rule and must not be a browser hint. |
| Rating aggregate into the tag | API / Backend (`ReporterSignals`) | Database (one batched read) | One code path for feed, vote response, history, Activity and push. |
| Follow, mute, block state | Database / Storage | API / Backend | Plain tables with composite primary keys. |
| Public profile page and lists | Frontend Server (html/template render) | API / Backend (JSON for lists and rows) | Same pattern as Settings and Activity: server renders the shell, vanilla JS fills lists. |
| Recent reports on a profile with coarse location | API / Backend (shared read path) | Database (account keyed candidate read) | Visibility and coarsening live in the one shared read (`classify`, `buildFinishedView`), never in SQL or JS. |
| "Not confirmed yet" note text | API / Backend (copy built at read time) | none | Built from the current trust object so it updates in place (03.2 D-15). |
| Phone notification, same tag, silent | API / Backend (dispatcher, worker) | Service worker (existing `sw.js`) | Tag is `report-<id>`; the worker already asks for silence. |
| Ending follows when the tag falls away | API / Backend (ticker sweep + send time recheck) | Database | The tag is computed, so a job must notice it changing. |
| Star tap UI, bell button, Follow button | Browser / Client (vanilla JS) | none | Text via `textContent` only; no markup sinks. |

## Standard Stack

### Core
No new libraries. Everything needed is already in `go.mod` or the standard library. [VERIFIED: go.mod, internal/service, web/static/js]

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-chi/chi/v5 | in go.mod | routes in the gated group | Project standard, flat literal routes. |
| jackc/pgx/v5 + sqlc | sqlc 1.31.1 local, matches CLAUDE.md | typed queries | `make sqlc`, output committed. |
| pressly/goose/v3 | v3.28.0 in go.mod | migrations | Run as an explicit step. |
| swaggo/swag | v1.16.4 in go.mod, same as the local binary | OpenAPI for the new JSON routes | `make swag`, drift guarded by `TestSwaggerSpecCoversRoutes`. [VERIFIED: go.mod line 13, `swag --version`] |
| `internal/service` text check (`CheckText`, `ScreenAbuse`) | in repo | screen Name before it becomes public | Reuse, do not call the provider elsewhere. [VERIFIED: internal/service/textcheck.go] |
| `internal/ratelimit` + `handlers.RateGate` | in repo | per account buckets for follow, rate, list reads | Keyed by account, never by IP (CGNAT). [VERIFIED: router.go geocode and push limiters] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| One shared `push_outbox` with a new `follow` kind | A sibling outbox table (Phase 5 chose this for `official`) | Follow notes need the confirmed update join, the shared first alert unique index (one note per account and report) and the worker's hourly cap. A sibling table would copy all of that. Recommend widening `push_outbox`. [ASSUMED] |
| Counting followers with `COUNT(*)` per request | Denormalised counters | Not needed at this scale; an index on `(followee_id)` is enough. |

**Installation:** none. **Version verification:** `sqlc version` v1.31.1, `swag --version` v1.16.4, `goose` present, Go 1.26.4, Node v26.4.0. [VERIFIED: local probes 2026-10-25]

## Package Legitimacy Audit

No external packages are installed in this phase. The audit table is empty by design.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| none | | | | | | |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

A star icon is needed (no `star.svg` exists in `web/static/icons/`); draw it as a new SVG in the repo, do not add an icon package. [VERIFIED: ls web/static/icons]

## What this phase needs from 7.1 and 8 (the dependency contract)

Neither phase has plans yet. The planner must name these by grepping the merged code at planning time and failing the plan if they are missing. Everything below is an assumption about outputs, [ASSUMED] unless a CONTEXT line is cited.

**From Phase 7.1 (builds before this phase):**
1. `accounts.username`: unique public handle, compared case insensitively, with a charset that excludes `@` and `/` (the trust parity test bans `@` in feed values, and a path segment must be safe). Null only for accounts that have not yet passed the one time fill in. [CITED: 07.1-CONTEXT D-06, D-07; column name and case rule ASSUMED]
2. Name and Home area are required, so `display_name` and `home_area_name` are set for every account that reached the app. [CITED: 07.1-CONTEXT D-06] The columns exist now (migration 00008). [VERIFIED]
3. A reusable reserved word check (police, ndma, imd, govt and similar) as a function, not only inside the username route. This phase needs it for the Name, see gap G1. [ASSUMED, flag to the 7.1 planner]
4. A username lookup helper (by name, case insensitive) and the live check route, which this phase's routes can share. [ASSUMED]
5. The test helpers `newVerifiedClient` and `newE2EServer` will be rewritten when the magic link is replaced by a password and typed code. Phase 9 tests must call the helpers by name and not copy the old sign in steps. [VERIFIED: current helpers in internal/api/handlers/*_e2e_test.go; rewrite is 07.1 D-12 scope]

**From Phase 8 (builds before this phase):**
1. The report page and its link form `/?report=ID` (03.1 D-11) which this phase's profile rows open. [CITED: 08-CONTEXT D-01]
2. Comments with usernames, and a comments read and count query that can take a `viewer_account_id`. Block (D-12) hides a blocked person's comments from the blocker, and the popup's real comment count must exclude them for that viewer. Phase 8 should leave a hook now: a single query fragment or Go function for "comments visible to viewer", used by both the list and the count. If Phase 8 does not, this phase has to edit Phase 8's queries. [CITED: 08-CONTEXT D-03, D-12 here; hook ASSUMED]
3. The trust block's explanation panel as shipped, where "See this reporter" goes (ROADMAP Phase 9 scope note 2). [CITED: 08-CONTEXT D-01]

**Gaps: needed here, built by neither 7.1 nor 8 (flagged for the planner):**
- **G1 Name screening and reserved words.** `ProfileUpdate` says names are not screened "until Phase 9, when a name is first shown to anyone else". 7.1 D-07 screens only the Username. So Phase 9 owns screening the Name (abuse screen through `ScreenAbuse`, reserved word match on word boundaries so "NDMA Control Room" is refused), on opt in and on every later Name edit while public. Fail closed: if the check is `unfinished` the switch does not turn on. [VERIFIED: internal/service/profile.go lines 78 to 85 comment, internal/service/textcheck.go]
- **G2 An "account removed for abuse" state.** Only `deleted_at` exists and it means the person deleted themselves. D-14's "ratings from an account later removed for abuse stop counting" needs either a new account state or a moderator action. Recommend the smaller one: a moderator action "Remove this person's ratings" that stamps `removed_at` on every rating row by that rater, plus the join on `accounts.deleted_at IS NULL` so a deleted account's ratings stop counting by themselves. A full account ban is out of scope. [VERIFIED: grep for banned or suspended finds nothing; ASSUMED recommendation]
- **G3 A moderator route for ratings.** The moderator group (`requireModerator`, answers a byte equal 404 to everyone else) exists with only report Keep and Remove. Ratings need list and remove routes inside that group. [VERIFIED: router.go moderator group]
- **G4 Person mute.** `report_mutes` mutes reports only. D-12 needs a person level mute that both the 3.2 Reliable alert and the new follow note respect. New table, new name.
- **G5 A picture.** No avatar storage exists and the no images rule applies. Use a letter avatar: the first letter of the username on a tinted circle drawn in CSS. [ASSUMED]
- **G6 Account deletion hooks.** `AccountDeletionService` and `ScrubDeletedAccount` must also clear the opt in flag, the person's follows both ways, mutes, blocks, notify flags and queued follow pushes. Decide the ratings the account gave: recommend delete them (they are a personal opinion row like the account's other personal rows), and state in `docs/TRUST-MODEL.md` that this is the one place deleting an account can move another person's stars, which differs from the "no effect on others" rule written on `DeleteAccount` for votes and reports. [VERIFIED: internal/service/accountdelete.go lines 80 to 100]

## Part 1. The trust core change (D-05) and combined stars (D-02)

### 1.1 What the tag is today
`ReporterTag` is cut inside `ScoreReliability` from a rounded integer score by `reporterTagForScore` (`ReliableMinScore` 75, `MixedMinScore` 35). It is computed on every read from the last 90 days of expired reports and is never stored. There is no SQL that pins the numbers. [VERIFIED: internal/service/reliability.go lines 148 to 151, 202 to 210, 234 to 238; internal/store/queries/reliability.sql has no threshold]

### 1.2 Every place that pins the numbers (file and line)

**Go code**
| Where | What |
|-------|------|
| `internal/service/reliability.go:148-151` | `ReliableMinScore = 75`, `MixedMinScore = 35` with comments |
| `internal/service/reliability.go:202-210` | `reporterTagForScore` |
| `internal/service/reliability.go:234-235, 238` | doc comment "75 or more is Reliable reporter, 35 to 74 Mixed record, 34 or less" and "boundaries 75, 35 and 25 are exact" |
| `internal/service/reliability.go` doc invariants (I1 to I4 block) | I1 says an all corroborated history is Reliable. It still holds for the outcomes score but not for the combined tag. Reword. |

**Tests that fail or change meaning when the constant moves to 70** (scores 70 to 74 change tag):
| Where | Why it changes |
|-------|----------------|
| `internal/service/reliability_test.go:259` ("4 corroborated and 1 contradicted", score 71, expects Mixed) | becomes Reliable |
| `internal/service/reliability_test.go:448-450` (`expectedTag` hard codes `>= 75` and `>= 35`) | must read the constants or 70 |
| `internal/service/reliability_test.go:258, 270, 341, 343` ("sit exactly on 75") | still pass, stop being boundary rows; add the new 70 rows |
| `internal/service/reporter_test.go:212` (account 102, score 71, expects Mixed) and `:246` (`Tag != ReporterMixed`) | becomes Reliable |
| `internal/service/trustfacts_test.go:255` (fact "Mixed record", tone Caution) | becomes "Reliable reporter", tone Good |
| `internal/service/trustwording_test.go:435` ("Mixed record", "Score 71 out of 100.") | becomes "Reliable reporter" |
| `internal/service/trust_model_doc_test.go:71-72, 232-233` | compares the doc constants table with `ReliableMinScore` and `MixedMinScore`; passes once `docs/TRUST-MODEL.md` is edited |

Not pinned and left alone: `feed_test.go:1443` (score 64, Mixed), `profile_record_test.go:36` (hand built signal), `trustfacts_test.go:127` and `trustwording_test.go:337` ("Score 72" is the still current score, a different scale with its own 75 and 50 cuts, `UpToDateMinScore`; do not touch it). [VERIFIED: grep of tests]

**The authoritative list comes from running the whole suite on a scratch database right after the constant change.** DB seeded end to end tests (`reliable_filter_e2e_test.go`, `reliable_alert_e2e_test.go`, `pushdispatch_db_test.go`, `testutil/history.go`) may seed a 70 to 74 history as a "not Reliable" control; grep cannot prove there are none. The executor of the first plan runs the full suite and records every flipped fixture in the SUMMARY.

**docs/TRUST-MODEL.md**
| Line | What |
|------|------|
| 303 | "the boundaries 75, 35 and 25 are exact (4 corroborated and 1 unconfirmed is exactly 5.25 / 7, which is 75)" |
| 307 | "75 or more (`ReliableMinScore`) is Reliable reporter, 35 to 74 ... is Mixed record" |
| 329 | anchor row "75 (exact boundary, 5.25 / 7) Reliable" stays true but is no longer a boundary |
| 330 | anchor row "4 corroborated, 1 contradicted: 71, Mixed" becomes Reliable |
| 341 | "8 corroborated, 2 contradicted: 75 (exact)" no longer exact boundary |
| 342 | "6 corroborated, 12 contradicted: 35 (exact) Mixed" unchanged (Mixed floor stays 35, see A2) |
| 577-578 | constants table `ReliableMinScore` 75 -> 70 (guarded by `TestTrustModelDocMatchesCode`) |
| 374, 529, 612 | TRUSTX-03 mentions; add the stars arithmetic and the lower bar as the reason it matters more (D-14) |
| 358-361 | "Display only": the tag now also gates following and notes; say so |
| 555, 619 | hand off line about Phase 9 profiles and the out of scope bullet "A public reporter profile page" must change |

New anchor rows (computed with the same integer arithmetic, equal ages, verified with a throwaway Go program): 6 corroborated + 2 contradicted (n=8) = **70 exact**; 9 corroborated + 2 contradicted + 2 unconfirmed (n=13) = **70 exact**; 4 corroborated + 2 unconfirmed (n=6) = **69**, the near miss that must read Mixed; 4 corroborated + 1 contradicted = 71 Reliable. [VERIFIED: scratch calculation, same formula as reliability.go]

**UI copy and API text**
- `internal/service/trustcopy.go:174` and `:230`: "Score N out of 100." (currency and reporter score lines). Keep the sentence. The score number a reader sees must be the combined points (see 1.4), so the word and the number cannot disagree (TRUST-MODEL line about "the band or tag is cut from the same integer"). [VERIFIED]
- `internal/api/handlers/trust.go:49, 63, 83`: swagger examples ("Score 86", "Score 92") unaffected. `ReporterResponse` gains fields (1.4), so `make swag` regenerates `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.
- `web/static/js` and `web/*.mjs` hold no threshold (grep for 75, 35, 70 near score or tag finds nothing). The browser draws server strings. Keep it that way: `TestTrustModuleHasNoThresholdsOrSinks` pins it. [VERIFIED]
- Swagger enum text for tag stays the four tags.

**No stored data holds the tag**, so there is no data migration for D-05. A stored follow is the only persisted consequence of a tag (Part 5). [VERIFIED: no tag column anywhere]

### 1.3 The Phase 3 rules that stay intact (03-CONTEXT D-10 to D-15)
- Keyed to the account through `sessions.account_id`, never `session_id`. Reports reach their account through sessions. [VERIFIED: reliability.sql point (a)]
- Only expired reports are judged. Credits 1, 0, 0.25. Half life 30 days, window 90 days, cap 30 judged, neutral rows dropped first. All unchanged; `ScoreReliability` is not edited except the constant and comments.
- **Below 5 judged reports a reporter is New with no score, whatever the ratings say** (D-13). Combined stars are computed only when `Score != nil`. A New reporter with 40 five star taps is still New. [VERIFIED: ScoreReliability returns ReporterNew with nil Score under ReliabilityMinJudged]
- Display only on default views (D-14): Resolve never reads the tag. Phase 9 makes the tag matter to follow and notes, which are opt in. The doc must say so.
- Held, removed and withheld reports stay neutral (Phase 4 R-02). Unchanged.

### 1.4 How combined stars feed the tag (recommended design)

Work in integer points on the 0 to 100 scale, the same unit as the tag, and cut the tag from the rounded integer exactly as Phase 3 does.

```
outcome points S        = ScoreReliability(...).Score        (0 to 100, nil for New)
raters r, tap sum T      = from non removed ratings of non deleted raters
r < 5                    -> points = S                         (D-02: outcomes only)
r >= 5                   -> points = round_half_up( (S*r + 20*T) / (2*r) )
tag                      = Reliable if points >= 70, Mixed if >= 35, else Unreliable
stars text               = floor(points / 2) tenths, e.g. 70 -> "3.5", 69 -> "3.4", 35 -> "1.7"
```

Integer form for exact boundaries: `num = S*r + 20*T`, `den = 2*r`, `points = (2*num + den) / (2*den)`. Tested against `r = 5` all fives, all ones, mixed.

**Display rounding must truncate.** Score 69 is 3.45 stars. Round half up, `%.1f` on a float, or any common rounding shows "3.5" beside a Mixed record tag, and "3.5" is the Reliable bar. Show floor to one decimal, computed as `points / 2` tenths in integers. Score 70 reads 3.5, 69 reads 3.4, 35 reads 1.7. Apply the same helper to the tapped average (`T*10/r` tenths, floor) and the outcomes part. One helper, one test that walks 0 to 100. [VERIFIED: arithmetic; recommendation ASSUMED]

**Where it plugs in** (so all surfaces agree):
- Keep `ScoreReliability` pure and unchanged apart from the constant. Add a pure `stars.go` (name is mine) with `CombinePoints`, `StarsTenths`, `StarsText`, constants `MinRatersForStars = 5` and `TappedHalfWeight`.
- Add the new constants (`MinRatersForStars`, the tapped weight, the follow caps) to `p20ConstFiles` and `p20Constants` in `trust_model_doc_test.go` and to the constants table in `docs/TRUST-MODEL.md`, so `TestTrustModelDocMatchesCode` guards them like `ReliableMinScore`. [VERIFIED: trust_model_doc_test.go lines 59 to 78, 224 to 243]
- `ReporterSignals(accountIDs, history, votes, now)` is called with four arguments in 26 test sites. Widen with a variadic option, `ReporterSignals(..., opts ...SignalOption)` and `WithRatings(map[int64]RatingTally)`, so tests keep compiling, and so there is still one rule. Inside, after `ScoreReliability`, apply `CombinePoints` only when `Score != nil`, then recompute the tag from the combined integer. `Reliability.Score` stays the outcomes part; `ReporterSignal.Score` becomes the combined points. New fields on `ReporterSignal`: `OutcomeStars`, `TappedStars` (nil below 5 raters), `Raters` (0 below 5), `Stars` text. The tapped part and rater count are exposed only at 5 or more.
- **The five production callers each need the ratings read.** [VERIFIED: grep]
  1. `internal/service/report.go:1018` (Nearby feed)
  2. `internal/service/trust.go:591` (vote response)
  3. `internal/service/historyread.go:214` (Past, Focus, counts)
  4. `internal/service/auth.go:622` (Activity page)
  5. `internal/service/pushdispatch.go:640` via `reporterIsReliable` at `:628` (push dispatcher; if it is missed, a phone note says Reliable while the feed does not)
  Add one helper next to `reporterHistoryParams` that does a single batched read, `ReporterExtrasForAccounts(account_ids)`, returning per account: the rating count and sum (non removed ratings, non deleted raters, only while the profile is public, see A8), the public flag and the username. Each caller makes that one extra read. The batch is bounded by `MaxReporterAccountsPerRead = 200`.
- `TestFeedSignalsMatchCastVoteSignals` and `TestFeedTrustMatchesCastVoteTrust` are the guards that the feed and the vote response stay equal. Extend them with a seeded rated reporter.
- The profile link ("See this reporter") is viewer dependent (a blocked viewer must not get it), so it must **not** go inside the `Trust` object, which is built once and compared for equality across surfaces. Put `reporter_profile_path` on the report row beside `viewer_vote` and `is_own_report` (set in `baseView`, the viewer field filler). [VERIFIED: baseView in historyread.go lines 296 to 325; parity tests]
- Feed JSON key names must not match `cell|account|session|email|voter` and values must not contain `@`; use `reporter_profile_path` with the value `/u/<username>`. [VERIFIED: trust_e2e_test.go lines 459 to 461, 540]

### 1.5 What the lower bar and the tapped half let people do (document it, do not hide it)
Numbers computed with the integer formula above, five raters:
- A reporter whose outcome score is **39 or more** reaches 70 (Reliable) when five raters all give five stars. (S=39: (195+500)/10 = 69.5 -> 70.) S=40 with five fives is exactly 70.
- A reporter at outcome score 86 (five corroborated) drops to **53** (Mixed record) when five raters all give one star. That also ends every follow they have (SC5) and, through the Only Reliable tick, alerts and push, changes who hears them.
- So five eligible accounts swing a score by about plus or minus 30 points, and from the fifth rater on the tapped half carries 50 percent however many raters there are. The eligibility rule (a verified account that voted within 1 km of one of that reporter's reports) is the price per account. [VERIFIED: arithmetic; MaxVoteDistanceKm is a published rule, TRUST-MODEL "Who can vote"]
- This is not a reason to reopen D-02 or D-05. It is the reason TRUSTX-03 stays a named dependency (D-14), reason enough to put it in TRUST-MODEL "Known limits" and "Honest limits", and the reason moderator removal (Part 2) and the per account rate limits are not optional. See Open Question Q4 for the one optional mitigation (a ramp) that the owner may want to hear about.

## Part 2. Star ratings (D-02, D-14)

**Who may rate** (all checked server side in one transaction; the browser only shows or hides the control):
1. A verified account (the gate). 2. Not the reporter: `rater_id <> reporter_id`. 3. One row per (rater, reporter), changeable by upsert. 4. The reporter has a public profile (A8). 5. Eligibility: the rater has cast at least one vote of value `confirm`, `dispute`, `resolve` or `reopen` on a report posted from any session of the reporter's account. Exclude `unresolve` (the take back needs no location, so it proves nothing about being nearby). These four values were location checked within `MaxVoteDistanceKm`. [VERIFIED: votes table, trust.go VoteValue constants, vote handler doc]

```sql
-- name: RaterMayRate :one
SELECT EXISTS (
  SELECT 1
  FROM votes v
  JOIN reports  r ON r.id = v.report_id
  JOIN sessions s ON s.session_id = r.session_id
  WHERE v.account_id = sqlc.arg(rater_id)::bigint
    AND s.account_id = sqlc.arg(reporter_id)::bigint
    AND v.value IN ('confirm','dispute','resolve','reopen')
) AS eligible;
```
`votes` has `idx_votes_account_kind_report_created (account_id, kind, report_id, ...)` (migration 00006) so the plan is an index scan on the rater side. [VERIFIED: migration 00006]

**Votes grandfathered before the 1 km rule** may satisfy eligibility without ever having been near. That is the same limit TRUST-MODEL already documents; note it, do not fix it.

**Routes** (house style: flat literal paths in the gated group, POST for actions, strict JSON, SameSite Lax cookie so no CSRF token):
- `POST /api/people/{username}/rating` body `{"stars":1..5}` upsert; `POST /api/people/{username}/rating/clear` withdraws the caller's own rating. Per account bucket through `RateGate` (add a `Rate` field to `RateLimits`; a change allowed at most once per reporter per day is enough). Errors never reveal why someone is ineligible beyond one fixed sentence, so the response is not an oracle about who voted on whom.
- The star box after "Yes, mark resolved": the resolve response (`CastVoteResponse`) gains an optional `rate_prompt` (`{profile_path, current_stars}`) present only when the caller is eligible, not the reporter, and the reporter is public. `votes.js` shows the box after the success message and Undo; Skip dismisses. The browser decides nothing about eligibility. [VERIFIED: votes.js lines 115, 1839 to 1850; CastVoteResponse]

**Removal and ignoring (D-14):**
- Per row `removed_at` and `removed_by` (moderator account id); aggregates ignore rows where `removed_at IS NOT NULL`.
- Aggregates also join `accounts` and ignore raters with `deleted_at IS NOT NULL`, so ratings of removed accounts stop counting without a second job. Account deletion deletes the ratings the account gave (G6) which gives the same effect immediately.
- Moderator routes in the existing moderator group (404 to everyone else): list ratings of a reporter by username (rater username, stars, time), remove one, remove all by one rater. Reuse `requireModerator`, register with `r.Handle`. Screen text follows `moderate.html.tmpl` and `moderate.js` patterns. [VERIFIED: router.go moderator group]
- A block never deletes or hides ratings (see Part 3). Otherwise blocking critics would let a reporter erase bad ratings.

**Brigade and farming controls that fit inside the locked decisions:** per account rate bucket; shown only at 5 or more raters; aggregate only, never who rated; moderator removal; and the TRUSTX-03 note in the docs with the arithmetic from 1.5. A rater also cannot rate a reporter whose profile is private, which removes rating as a quiet way to move someone's tag.

## Part 3. Data model

New names are person scoped. Do not call anything `follows`, `mutes` or `blocks` alone: `follow.go` (`ViewerFollows`), `FollowedOpenReports`, `ViewerFollows` on report rows and `report_mutes` already mean report level concepts. [VERIFIED: internal/service/follow.go, mutes.go, migration 00009]

Two migrations are cleaner than one, both numbered at execution time ("the next free migration number at execution time"): one for profiles, follows, mutes, blocks and ratings (plans 2 and 3), one for the notification kinds (the follow plan). Phase 5 and Phase 3.3 also take numbers; every plan re-reads the migrations folder before naming a file. Goose files carry both Up and Down.

```sql
-- schema migration (shape only; number is next free at execution time)
ALTER TABLE accounts
    ADD COLUMN profile_public    BOOLEAN NOT NULL DEFAULT false,   -- "Let others follow me"
    ADD COLUMN profile_public_at TIMESTAMPTZ;

CREATE TABLE account_follows (
    follower_id       BIGINT NOT NULL REFERENCES accounts(id),
    followee_id       BIGINT NOT NULL REFERENCES accounts(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    notify            BOOLEAN NOT NULL DEFAULT false,              -- the bell, D-08
    notify_since      TIMESTAMPTZ,                                 -- only reports posted after this note
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT account_follows_not_self CHECK (follower_id <> followee_id),
    CONSTRAINT account_follows_notify_since CHECK (notify = (notify_since IS NOT NULL))
);
CREATE INDEX idx_account_follows_followee
    ON account_follows (followee_id, created_at DESC, follower_id DESC);   -- Followers list, keyset

CREATE TABLE person_mutes  (muter_id   BIGINT NOT NULL REFERENCES accounts(id),
                            muted_id   BIGINT NOT NULL REFERENCES accounts(id),
                            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                            PRIMARY KEY (muter_id, muted_id), CHECK (muter_id <> muted_id));
CREATE TABLE person_blocks (blocker_id BIGINT NOT NULL REFERENCES accounts(id),
                            blocked_id BIGINT NOT NULL REFERENCES accounts(id),
                            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                            PRIMARY KEY (blocker_id, blocked_id), CHECK (blocker_id <> blocked_id));
CREATE INDEX idx_person_blocks_blocked ON person_blocks (blocked_id);      -- "who blocked me" checks

CREATE TABLE profile_ratings (
    rater_id    BIGINT NOT NULL REFERENCES accounts(id),
    reporter_id BIGINT NOT NULL REFERENCES accounts(id),
    stars       SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at  TIMESTAMPTZ,
    removed_by  BIGINT REFERENCES accounts(id),
    PRIMARY KEY (rater_id, reporter_id),
    CHECK (rater_id <> reporter_id)
);
CREATE INDEX idx_profile_ratings_reporter ON profile_ratings (reporter_id) WHERE removed_at IS NULL;
```
Source for the table style: the existing 00009 and 00011 migrations (named CHECKs, partial indexes, composite keys). [VERIFIED: migrations 00009, 00011]

**Opt in preconditions** (`PATCH /api/me/visibility` or similar, own handler and swag block, not folded into `UpdateProfile`): username present, Name present, Name passes G1. Turning off is one transaction: set `profile_public = false`, delete all rows where `followee_id = me`, delete queued follow pushes for this reporter's reports (Part 5). Do not delete ratings (A8).

**Follow preconditions** (checked server side every time): follower is not the followee, followee `profile_public` is true, followee tag is Reliable now (computed through `ReporterSignals` with ratings), no block either way between the two, followee account not deleted. Following is idempotent. Unfollow always works.

**Block effects** (one transaction): insert block; delete follows in both directions; delete mutes between them is optional; the blocked person gets the same "not found" for the blocker's profile and gets no `reporter_profile_path` for the blocker's reports; the blocker's Followers and Following lists drop the blocked person; Phase 8's comment reads hide the blocked person's comments from the blocker (dependency above). The blocked person's reports keep showing in every feed (hiding emergency reports is never safe, D-12). Blocking does not touch ratings.

**Public lists (D-03).** Followers of X = rows with `followee_id = X`. Split in SQL: named entries are accounts with `profile_public AND deleted_at IS NULL AND username IS NOT NULL`, ordered newest first with keyset paging (`created_at, follower_id`), 50 per page; everyone else is returned only as a count ("and 3 others"). The counts at the top of a profile are the total including the hidden ones. Following of X is the same by `follower_id`; by construction a followee is always public when followed, so a Following list is normally fully named, but still apply the split because a followee may have left. Hide blocked-either-way pairs from the viewer's own view of the lists and count them in "others" only. A hidden entry is never a link. [ASSUMED design; D-03 is the rule]

**Mute (D-12, G4).** `person_mutes` is read by the 3.2 Reliable alert step and by the new follow note step. It never changes what any feed shows. Where to undo it is Open Question Q6.

**Account deletion (G6)** adds queries to `AccountDeleteQuerier`: delete follows where the account is follower or followee, delete mutes and blocks where the account is the actor, delete ratings given, delete queued follow pushes. `ScrubDeletedAccount` also sets `profile_public = false`. `internal/service/accountdelete_test.go` pins the interface and the order, so it is amended. [VERIFIED: accountdelete.go]

## Part 4. Profiles (D-04, D-06, D-07, D-13)

### 4.1 Routes and files (new names, do not touch the Activity page)
- **`/profile` is My activity, not a profile.** `web/templates/profile.html.tmpl` has `<title>Pinalert: My activity</title>`, `handlers.Profile` renders it, and `profile_nav_contract_test.go` pins it. The CONTEXT's "current My profile page from Phase 3.4" is the Activity page. Do not edit it into a profile. [VERIFIED: profile.html.tmpl line 7, router.go `/profile`]
- New page route `GET /u/{username}` (own or anyone's), new template (for example `person.html.tmpl`), and `GET /me` that answers 303 to `/u/<my username>`. The person menu then links to a constant `/me`, so `account_header_menu` (rendered on every page with many different data structs) needs no per page username field. The Settings About you row (phone) links the same `/me`. [ASSUMED]
- JSON: `GET /api/people/{username}` (header data), `/followers`, `/following` (keyset, `?after=`), `/reports` (`?lat=&lon=` optional). Actions as POST verb paths: `follow`, `unfollow`, `notify` (body `{"on":bool}`), `remove-follower`, `mute`, `unmute`, `block`, `unblock`, `rating`, `rating/clear`. All inside the gated group; the exempt list in `gate_test.go` is not touched. Add the new paths to an anonymous 401 table test.
- Every JSON handler carries `@Tags`, `@Router` and plain punctuation in the annotations (backlog 999.8: long dashes in comments leak into the public `/swagger`). Run `make swag` and commit the three generated files. [VERIFIED: ROADMAP 999.8; swag go.mod pin]
- Unknown username, private profile, deleted account, blocked viewer: the same 404 body and status (no enumeration of who opted in). A private person is still reachable only through their own pages. Per account bucket on the people routes, because the lists are public and scrapable by design.

### 4.2 Header (D-13) and what is server authored
Order: round letter picture, username, Name, stars with the outcomes part and the tapped part beside the total (tapped part only at 5 or more raters), the tag with its counts, member since (`accounts.created_at`, month and year), Followers and Following with counts that open the lists, then Follow or Following and the bell. All words come from the server (extend `trustcopy.go` with profile sentences; do not build them in JS). The counts sentence in D-13 ("7 of 9 earlier reports were confirmed by people nearby") differs from today's panel line ("7 of their last 9 reports were confirmed."); pick one in the UI spec. Home area on the profile is Open Question Q1. The page shows the same stars to everyone; the owner of the page sees a quiet link to Settings. A person who has not opted in sees their own page with the same layout and a plain line pointing at Settings.

A star control for readers (1 to 5 buttons, 44px targets via `--touch-target-min`, current value marked with `aria-pressed`, a Clear button). A chosen item must not carry a dark border (`TestNoDarkBorderOnChosenItems`); the bell uses the existing `bell.svg` and `bell-off.svg`.

### 4.3 Recent reports (30 days, D-07) through the one shared read
- Add `ReportService.ProfileReports(ctx, ProfileQuery)`. Candidate read: all reports whose session belongs to the account, `created_at > now - 30 days`, newest first, capped (for example 100 plus one to detect truncation). Use the same lateral per session shape and `idx_reports_session_expires` as `ReporterHistoryReports`. [VERIFIED: reliability.sql]
- Then reuse `loadReadContext(withSignals true)`, `classify`, `baseView`, and `buildFinishedView`. Never write a second visibility rule or SQL predicate. `classify` already fails closed when the moderation extras are missing. Any new direct call to `Resolve` must carry the stored moderation state, which `TestResolveCallersMentionModeration` enforces; going through `classify` satisfies it. A `FeedViewNone` report is listed nowhere. [VERIFIED: historyread.go classify and tests]
- Which views to list: Active (Live and Provisional, with the trust block saying "Not confirmed yet"), Disputed (listed, labelled), Resolved, Past. Omitting Disputed would flatter a reporter. [ASSUMED]
- Live reports show exactly as on the map (exact point), as D-07 says: the same report is already public on the map, the profile only adds the link to a username. Finished reports show **coarse**: replace the point with the centre of a geohash cell, drop the description, and measure the distance to that coarse point. Today's `CoarsenFinishedRescue` only blurs Rescue (precision 6, about 1 km, text removed); PROF-02 asks for a coarse location for finished reports generally, so add a profile wide coarsener beside it (for example `CoarsenFinishedForProfile`) at precision 5 (about 5 km, the same grid as the stored home area) and keep Rescue on its precision 6 with no text (03.1 D-23). Put it in `buildFinishedView` behind a flag on `viewInput` so every finished view still leaves through the one place. [VERIFIED: coarsen.go; recommendation ASSUMED, precision 5 is A4]
- Distance from the reader: JS sends `lat` and `lon` (the same centre rule as Nearby: device position, searched place, or home area through `page.HomeCentre`); the server computes it with `haversineKm` and returns a number rounded to whole kilometres. No distance when no centre.
- **Finished rows are never links; only live rows open the report page.** `Focus` goes through `buildFinishedView`, which coarsens only Rescue reports, so every other finished report opens at its exact point on `/?report=ID`. If finished profile rows linked there, a profile that shows only coarse cells would be one tap from an exact location tied to a username for the last 7 days, which defeats ROADMAP scope note 1 (history shows coarse locations only). `Focus` also answers `too_old` after `HistoryWindow` (7 days), and reports expire in 8 hours (24 for critical), so almost every row on a 30 day profile is finished anyway. Recommended (D-07 is Claude's discretion): live rows (Active, and Disputed while still live) open `/?report=ID` as the map does; finished rows (Resolved, Past) are plain non interactive rows ("ended 12 days ago, Flood, Medium, near X"). That also removes the 7 day cutoff to explain. If the owner wants finished rows to open, the report page itself must first coarsen a finished report for any viewer who is not the poster, which is a Phase 8 change. See Open Question Q3. [VERIFIED: history.go lines 49 to 52, report.go lines 34 to 39, historyread.go Focus and buildFinishedView]
- Do not show finished descriptions on a profile (a description can hold a name or phone number that the reporter typed). Show category, severity, time and coarse area. [ASSUMED]

### 4.4 Entry points
- Laptop person menu gains My profile as a third item first. `TestPersonMenuItems` (web/places_contract_test.go:17-27) requires exactly 2 `role="menuitem"` and bans "My profile" and `href="/profile"`; `TestSettingsBackAndNoGear` (places_e2e_test.go ~line 224) expects the menu text "Settings Log out". Amend both with the owner's dated note. `ways_in_contract_test.go:215` pins `account-menu__item` on menu links.
- Phone: a "View my profile" row in Settings About you (phone only per 03.4 D-25). A phone only element must be named in `sizeOnlyNamed` (`TestSizeOnlyElementsAreNamed`) with the reason that the laptop reaches the same page from the person menu. `places_e2e_test.go:185, 256` and `waitingSettingsLabels` (places_contract_test.go ~lines 185 to 190) ban "View my profile", "My profile", "Let others follow me", "Following", "Followers", "Blocked people" today and must have these removed from the bans. [VERIFIED]
- "See this reporter": a link inside the trust block's explanation panel, shown only when the row carries `reporter_profile_path`. Built in `trust.js` with `textContent` and a server given path; no markup sink, no threshold.

## Part 5. Notifications (D-08 to D-11)

### 5.1 What Phase 3.3 actually built for "Someone I follow posts"
- Nothing user visible. Settings has no such row and `TestNotificationsNoFollowOrOfficialRow` (web/settings_rows_contract_test.go:692) pins its absence; keep that test true. The kind name is reserved only: `PushKindFollow = "follow"` in `ReservedPushKinds`; `PushKind.Valid` refuses it; `BuildPushText` returns `ok=false` for it. No migration, no switch. [VERIFIED: pushkinds.go lines 12 to 56, pushcopy.go BuildPushText doc, settings.html.tmpl]
- 03.3 plan 14 (Settings rows) is merged. Plan 15 (docs and e2e: "Phone alerts" section in TRUST-MODEL, `docs/PUSH.md`) is not; if it lands before this phase, extend both with the follow kind; if not, plan 15 owns creating them. [VERIFIED: git log 2afcf13; 03.3-15-PLAN.md]
- Phase 5 plan 05-11 (uncommitted draft, read only) moves `PushKindOfficial` into `PushKinds` and leaves `ReservedPushKinds = [follow]`, with `TestPushKindsClosed` pinning "follow is not Valid". Phase 9 builds on the version that is on main, then flips that test again: `PushKinds` gains `follow`, `ReservedPushKinds` becomes empty. Phase 5 chose a sibling outbox for official alerts and leaves `push_outbox` itself alone. [VERIFIED: 05-11-PLAN.md lines 125, 177, 188]

### 5.2 Design (one rule, two channels, same as the Reliable alert)
**In app bell.** New update kind `followed_posted` (name is mine). Producer: a new step in `UpdatesService.Refresh`, next to `refreshReliableAlerts`, that for the polling account reads follow rows with `notify = true`, finds the followee's reports posted after `notify_since`, applies `MayNotifyAboutReport(UpdateFollowedPosted, ...)` and writes one row per account and report. Add the kind to the single predicate's existing allowance branch (`kind == UpdateReliableNearby || kind == UpdateFollowedPosted`), never a second copy: Live and Provisional, never Hidden, Withheld, Retracted or expired. Row words are built at read time from the current trust object ("Not confirmed yet." then the confirmed sentence), exactly like the Reliable note (`UpdateTrustLine`, `EventLine`). Touch points: `updates.go` (kind lists, `UpdateKinds`, `IsEvent`), `updatescopy.go:44, 65`, `handlers/updates.go:42` (swagger enum), `handlers/activity.go:195`, `updates_kind_valid` CHECK, and the tests that pin the closed list. [VERIFIED: grep of reliable_nearby above]
- One note per account and report across both producers: the Reliable alert step skips a report whose reporter the reader follows with the bell on; and add a guard so a race still cannot write both (widen the unique index `uq_updates_reliable_once` to cover `followed_posted` or add a sibling index and make both inserts check the other). `InsertReliableAlert`'s `ON CONFLICT ... WHERE kind = 'reliable_nearby'` must match the index predicate it infers; if the index is widened, change the clause too. [VERIFIED: updates.sql lines 154 to 200]
- Skip the reporter's own reports, muted people (`person_mutes`) and blocked pairs. Respect the hourly ceiling (`UpdatesHourlyCeiling`) like every kind.
- **Bounds, because this step runs inside `Refresh` on every 30 second bell poll.** Cap how many people one account can follow (a named constant, for example `FollowingCap`, number set at planning) and how many of those can have the bell on (`NotifyFollowsCap`). The producer reads only follows with `notify = true`, only the followees' reports still active (`expires_at > now`) and posted after `notify_since`, with a row cap like `FollowedReportsCap` (200) so a heavy account loses the oldest first. Put the constants in the docs constants table. [VERIFIED: updates.go FollowedReportsCap and Refresh; cap numbers ASSUMED]
- Display availability check at read time: if the reporter is no longer public or no longer followed, the row reads "This report is no longer available", the same as a report that no longer shows. Already written rows are not deleted when a follow ends; that is the definition of "pending" recommended in Open Question Q9.

**Phone.** Reuse the Reliable pattern: `reliableBranch` in `EvaluateReport` asks the updates service to Refresh each candidate account and then queues a push for exactly the `reliable_nearby` rows that step wrote ("one row, two channels", `EnqueueReliableFromUpdates`). Add a `followBranch` that runs before `reliableBranch` (so follow wins the shared first alert slot), Refreshes each follower of the reporter who has the bell on, and queues a `follow` outbox row for each `followed_posted` row (a new `EnqueueFollowFromUpdates` modelled on `EnqueueReliableFromUpdates`), keyed by `(account_id, report_id)`. The rule lives once, in the bell step; push only follows it. Required changes, all of them, because each is a closed list today: [VERIFIED: push.sql, pushworker.go, pushdispatch.go]
1. Migration: `push_outbox_kind_valid` adds `'follow'`; `uq_push_outbox_first` becomes `WHERE kind IN ('critical','reliable','follow')`; every `ON CONFLICT (account_id, report_id) WHERE kind IN (...)` clause (`EnqueueInitialPush`, `EnqueueReliableFromUpdates`) is updated to the new predicate; `push_report_progress` gets `follow_state` and `follow_at` if a per reporter fan out cap is kept (recommended small cap, a stand in for TRUSTX-03 like the others; `CountReporterFanouts` extends).
2. `EnqueueConfirmedUpdates`, the `first_kind` LEFT JOIN in `ClaimDuePushes` and `EnqueueChangedFromUpdates`' NOT EXISTS list all say `kind IN ('critical','reliable')`; add `'follow'`. This is what makes the in place "confirmed" update replace the follow note under the same tag `report-<id>` with no second ping.
3. Worker: `stillHolds` gets a `PushKindFollow` case (`MayNotifyAboutReport(UpdateFollowedPosted ...)` **and** the follow row still exists with `notify` on **and** the reporter is still public **and** still Reliable now (`reporterIsReliable`, which must include ratings) **and** no mute or block). `confirmedBaseKind`, `kindSwitchOn` (there is no per kind account switch for follow; the check is the follow row) and the hourly cap list in `CountAccountPushesInWindow` gain follow. [VERIFIED: pushworker.go 458 to 531, push.sql CountAccountPushesInWindow]
4. **Distance exception.** Worker step 5 skips every kind except `changed` when the person has no home area (`no_home_area`) or the report is beyond the alert radius (`out_of_range`). A follow note is not about distance. Make step 5 conditional on the base kind being critical or reliable (a confirmed update of a follow note also skips it). Leaving this out means followers without a home area silently get nothing. [VERIFIED: pushworker.go step 5]
5. Copy: `BuildPushText` gains a `follow` case and a `confirmed_update` built from a follow base. **Recommend no username on the lock screen.** `PushTextInput` deliberately has no name field (`TestBuildPushTextHasNoReporterTextField`), a lock screen is read by bystanders, and a username there discloses who someone follows. Title like "A person you follow reported {category} near {area}", body "Not confirmed yet." then "Confirmed by people nearby."; optional distance only when a home area exists. No reporter text, no coordinates, no word about sound (`TestNoNoiseWordsInUserCopy` bans a closed list of noise words). Silence is the service worker's job and already done. [VERIFIED: pushcopy.go; noise_words_contract_test.go]
6. `ReservedPushKinds` empty, `PushKinds` has six entries, `TestPushKindsClosed` and `TestPushCapsAreNamed` amended. The cap `PushPerAccountPerHour = 4` already limits a follower; the per recipient per reporter cap that 3.2 uses for Reliable (one per hour) is not recommended here because D-09 says one note per report and a flood updater should reach followers.
7. Dedupe with Reliable and Critical for the same report: critical wins (it is enqueued first), follow beats reliable. Say it in the dispatcher doc comment.

### 5.3 SC5: the tag falls away, the switch goes off
- Switch off: synchronous in the request that turns it off (Part 3).
- Tag falls away is computed, never an event, and it can change by time alone (decay, a report expiring) or by a rating write. So: (1) **send time recheck** in the worker (above) and in the bell producer (skip a followee whose tag is not Reliable now); (2) **evaluate on touch**: any profile read of X that computes a non Reliable tag deletes X's follows then; a rating write re-evaluates that reporter immediately; (3) **a ticker sweep** (PushChangesSweepInterval is 5 minutes; the same style) that reads distinct followees in batches of 200, computes their signals with ratings, and deletes the follows and queued follow pushes of those no longer Reliable or no longer public. Wire it in `cmd/server/main.go` where the dispatcher and worker are started. [VERIFIED: pushdispatch.go Run and intervals; pushworker; main.go wiring is the 3.3 plan 11 pattern]
- Flapping: a reporter near 70 can lose every follower to decay or a rating swing, and followers do not come back when the tag returns (SC5 says ends). Say so in the profile copy for the reporter ("Followers are removed when the Reliable tag is lost").
- "Pending alerts": recommended meaning is queued phone notes plus notes not yet written; bell rows already written stay as history (Open Question Q9).
- A reporter sees who follows them (lists are public) and can Remove follower one at a time; there is no bulk remove other than switching the profile off.

## Part 6. Settings rows (03.4 D-25) in People and reports

Today the section holds only Muted reports and its line reads "Reports you have turned updates off for." [VERIFIED: settings.html.tmpl lines 150 to 163] Add four rows. The order is fixed by 03.4 D-25: Let others follow me, Following, Followers, Muted reports (already built), Blocked people. So the three follow rows go before Muted reports and Blocked people goes after it. The rows: **Let others follow me** (switch, `role="switch"`, saved at once like the alert switches; turning on can fail with the Name or username message), **Following** (count, opens a window), **Followers** (count, opens a window), **Blocked people** (count, opens a window with Unblock; Mute list under Q6). Reuse `app-window` and the Muted reports window code (`settings.js`). Rewrite the section line. Tests to amend: `places_contract_test.go` (chevron row count around line 226, `waitingSettingsLabels`), `places_e2e_test.go` ("Settings button rows are ..." around line 150 and the bans at 185 and 256), `settings_rows_contract_test.go`. New scripts join `phaseScripts` (plain punctuation) and `noiseCheckedFiles`; new controls join `touchControls` (44px); a new star icon is registered in `laterIcons` (`TestNewIconsAreRegisteredAndUsed`, which also checks the glyph file against the reference). [VERIFIED: layout_rules_contract_test.go, css_contract_test.go:811 to 830]

## Part 7. Privacy (scope note 1): everywhere the profile can leak

Phase 3 D-10 ("linking one person's reports is intended, not a privacy risk") held because no username was shown. A username, a Name, a 30 day list and live points together can form a movement trace. List for the planner and the threat model:

1. **Exact live coordinates linked to a username.** Same point as the map, new link to a person. Accept per D-07, limited to currently live reports (8 hours, 24 for critical).
2. **Finished reports over 30 days.** Exact points would trace a routine. Coarse cell at precision 5, no text, distance to the coarse point (4.3). Even coarse, 30 days of cells can reveal a neighbourhood; this is why the list is capped and why the profile only exists after opt in.
2b. **A finished row that links to the report page.** `Focus` does not coarsen non Rescue finished reports, so a link from a finished profile row hands back the exact point (4.3). Finished rows on a profile are never links.
3. **Distance to an exact point.** Never compute a distance to the exact point of a finished report (the same leak `CoarsenFinishedRescue` documents). [VERIFIED: coarsen.go comment]
4. **Home area and Name.** The ROADMAP owner decision says both show on an opted in profile; D-13's header lists Name but no home area. A home area is where someone lives, so Q1. Never expose `home_area_cell`.
5. **Voter identity.** Ratings: aggregate only, count only at 5 or more, never a list of raters to anyone but moderators. The stars appear the moment the fifth rating lands, which lets a reporter who watches a count guess the fifth rater; accept, document. The ineligible response must be one fixed sentence.
6. **Social graph.** Named entries exist only for opted in people; hidden ones are numbers; hidden entries are never links; block does not reveal itself (same 404, no profile link).
7. **Enumeration.** Same 404 for unknown, private, deleted, blocked. Per account bucket.
8. **Lock screen.** No username, no text, no coordinates, area name only as 3.3 already does.
9. **Referrer and URLs.** Profile paths carry a username, not an email; no query values with locations except the optional `lat` and `lon` on a JSON call that the request logger already strips (path only logging). Keep the page `Cache-Control: no-store`. [VERIFIED: requestLogger writes the path only]
10. **Feed JSON.** `reporter_profile_path` appears only for opted in reporters and not for a viewer blocked by them; no account id, no `@`.
11. **Display strings.** Username and Name are user text: `html/template` escaping server side and `textContent` in JS only (`TestNoMarkupSinksAnywhere`). Treat bidi and invisible characters with `cleanText`. [VERIFIED: js_sinks_contract_test.go; profile.go cleanText]
12. **Withheld reports.** The profile read must never list a Withheld or `FeedViewNone` report; it goes through `classify`.
13. **Account deletion.** Clears the opt in flag and the graph (G6).
14. **Policy page.** The Policy text (and `CurrentTermsVersion`, which must equal the Policy's "Last updated" date, `TestTermsVersionFollowsPolicyDate`) should say that an opted in profile shows username, name, stars, recent coarse reports and follow lists. Changing the Policy changes the terms version and asks everyone to agree again at their next sign in. Raise this with the owner; it is a product consequence of editing the Policy. [VERIFIED: profile.go lines 36 to 41]

## Part 8. API docs and the web contract tests

- Every new JSON handler: `@Summary`, `@Description`, `@Tags people`, `@Param`, `@Success`, `@Failure` (401, 403, 404, 409, 429), `@Router` under `@BasePath /api`, plain punctuation. `make swag` then `TestSwaggerSpecCoversRoutes` (reads the committed `docs/swagger.json`) passes. Add the new routes to that test's list. HTML routes (`/u/{username}`, `/me`) carry no swag block, like `/settings`. [VERIFIED: router.go comments, swagger_test.go]
- `web/*_contract_test.go` rules a new template or script must satisfy:
  - No markup parsing sinks in any script under `static/js` (`TestNoMarkupSinksAnywhere`): no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`.
  - No long dashes in any template (`TestUserVisibleCopyUsesPlainPunctuation`), and in the "phase scripts" list (`phaseScripts`, `TestNewScriptsUsePlainPunctuation`); comments count.
  - No pill shaped controls; the round avatar is not a button, but any button radius must pass `TestNoPillShapedControls`.
  - No underline at rest (`TestNoUnderlineAtRest`); actions are real buttons.
  - No noise words in listed files (`noiseCheckedFiles`).
  - Touch targets at least 44px via `--touch-target-min` for the new controls (`touchControls`).
  - Anything hidden on one size is named in `sizeOnlyNamed` with a reason; phone and laptop keep the same features.
  - No `opacity` on text, no dark border on chosen items (`TestNoOpacityText`, `TestNoDarkBorderOnChosenItems`).
  - Design tokens are declared once (`TestPhase34DesignTokensDeclaredOnce`); type sizes use the rem tokens, never px literals.
  - Trust JS has no thresholds or sinks (`TestTrustModuleHasNoThresholdsOrSinks`).
  - Feed JSON key names avoid `cell|account|session|email|voter`; no `@` in values (`TestTrustResponseOmitsIdentifiers`).
  - New CSS files must be added to every template that links stylesheets and to the asset version scheme (`{{.AssetVersion}}`).
- Tests that need updating are listed per part above; the final plan runs the full web and Go suites and records every pin that moved.

## Architecture Patterns

### System Architecture Diagram

```
Reader (phone or laptop browser)
  |
  |-- GET /u/{username}  ---------------------> [gated group, onboarding gate]
  |       html/template shell (person.html.tmpl)         |
  |-- fetch /api/people/{u}, /followers, /following -----+--> PeopleService
  |-- fetch /api/people/{u}/reports?lat&lon -------------+      |  profile_public? blocked? deleted?  -> same 404
  |-- POST follow | notify | mute | block | rating ------+      |
  |                                                             v
  |                                              ReporterSignals(+WithRatings) <--- ReporterExtrasForAccounts (1 batched read)
  |                                                             |   ScoreReliability (unchanged, outcomes S)
  |                                                             |   CombinePoints(S, r, T) -> tag cut at 70
  |                                                             v
  |                                              ProfileReports -> loadReadContext -> classify (Resolve+ViewOf) -> baseView
  |                                                             |   finished -> buildFinishedView -> CoarsenFinishedForProfile / Rescue
  |                                                             v
  |                                                    JSON rows (server written words, distance, coarse points)
  |
Report posted ---> PushDispatcher.EvaluateReport --> followBranch (before reliableBranch)
Bell poll (30 s) -> UpdatesService.Refresh ------> refreshFollowedPosts (MayNotifyAboutReport)
                                                       |  follow row + notify + tag Reliable now + not muted/blocked
                                                       v
                                         updates row 'followed_posted'   and   push_outbox 'follow' (shared first-alert slot)
                                                                                      |
                                       PushWorker.sendRow: stillHolds(follow) -> no distance check -> silent push, tag report-<id>
                                                                                      |
Ticker sweep (5 min) -> distinct followees -> ReporterSignals(+ratings) -> delete follows (+ queued pushes) if tag lost / profile private
Moderator (404 to all others) -> list ratings -> remove one / remove all by a rater -> next read ignores them
```

### Recommended Project Structure
```
internal/service/
  stars.go            pure: CombinePoints, StarsTenths, StarsText, MinRatersForStars
  ratings.go          RatingService: eligibility, upsert, clear, tallies; moderator remove
  people.go           PeopleService: opt in, follow, notify, mute, block, remove follower, lists, sweep
  profilereports.go   ProfileReports (candidate read + shared pipeline) and CoarsenFinishedForProfile call
  profilecopy.go      every profile sentence (kept out of JS)
  followsweep.go      ticker sweep, send time helper
internal/api/handlers/
  people.go           profile, lists, actions (swag annotated)
  ratings.go          rating + moderator rating routes
internal/store/queries/
  people.sql, ratings.sql   (sqlc; regenerate and commit)
internal/store/migrations/
  <next free>_..._profiles_and_ratings.sql, <next free>_..._follow_notes.sql
web/templates/person.html.tmpl, web/static/js/person.js, web/static/css/person.css,
web/static/icons/star.svg, web/static/js/{votes.js,trust.js,settings.js} edits
docs/TRUST-MODEL.md, docs/MODERATION.md (ratings), docs/PUSH.md (if present)
```

### Pattern 1: one rule, many callers (the signal option)
**What:** extend the existing function with an optional input instead of adding a second rule; one loader for all five callers.
**Example:**
```go
// Source: this repo, internal/service/reporter.go (shape), recommended change [ASSUMED]
type SignalOption func(*signalConfig)

func WithRatings(t map[int64]RatingTally) SignalOption {
	return func(c *signalConfig) { c.ratings = t }
}

// inside ReporterSignals, after rel := ScoreReliability(outcomes[id], now)
// if rel.Score != nil { rel = withCombinedStars(rel, cfg.ratings[id]) } // New stays New (D-13)
```

### Pattern 2: pure stars core
```go
// stars.go (pure, table tested; recommended) [ASSUMED]
const MinRatersForStars = 5

func CombinePoints(outcome, raters, tapSum int) int {
	if raters < MinRatersForStars {
		return outcome
	}
	num := int64(outcome)*int64(raters) + 20*int64(tapSum)
	den := int64(2 * raters)
	return int((2*num + den) / (2 * den)) // round half up
}

func StarsTenths(points int) int { return points / 2 } // floor: 70 -> 35, 69 -> 34

func StarsText(points int) string {
	t := StarsTenths(points)
	return fmt.Sprintf("%d.%d", t/10, t%10)
}
```

### Pattern 3: the profile read is the Past read with a different candidate source
Candidate rows come from reports by this account's sessions in the last 30 days; everything after that is `loadReadContext`, `classify`, `baseView`, `buildFinishedView`. The only new branch is the coarsener flag.

### Anti-Patterns to Avoid
- **A second place that cuts the tag.** Anything that compares a score to 70 outside `reporterTagForScore` is a bug.
- **Putting a viewer dependent field inside `Trust`.** It breaks feed and vote response parity.
- **Editing `profile.html.tmpl`** for the new profile. It is My activity.
- **Naming a table `follows` or a field `viewer_follows`** for the person graph.
- **A stored tag or stored combined score.** Both are computed on read, by design.
- **JS computing stars or eligibility.** Strings and booleans come from the server.
- **Deleting ratings on block.** It lets a reporter launder critics away.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Is this report visible | a profile specific filter | `classify` + `ViewOf` via `loadReadContext` | One visibility authority, `TestResolveCallersMentionModeration` and `TestWithheldInNoView` guard it |
| Coarse location | rounding lat/lon | `CoarsePoint` (geohash cell centre) | Idempotent, one grid, no half leaked fields |
| Text abuse check | a new moderation call | `CheckText` / `ScreenAbuse` | Bounded, fail closed behaviour already tested |
| Per account limits | a new limiter | `ratelimit.NewPerIP` as keyed bucket and `RateGate` | CGNAT safe by account key |
| Phone send, caps, leases | a second worker | the existing `PushWorker`, `push_outbox`, `ClaimDuePushes` | Leases, backoff, silent flags, tag replacement already built |
| Eligibility to notify | a second predicate | `MayNotifyAboutReport` | 03.2 D-29: a second copy is a bug |
| Settings windows | new modal code | `app-window` markup and `settings.js` pattern | Contract tests already cover it |
| Hidden characters in names | a regex | `cleanText` in `profile.go` | Handles ZWJ rules for Indic and Arabic scripts |

**Key insight:** this phase adds almost no new mechanism. It widens closed lists and shares existing pipelines; the risk is in the many places those lists are repeated.

## Common Pitfalls

### Pitfall 1: displayed stars contradict the tag
**What goes wrong:** score 69 shows "3.5" next to Mixed record. **Why:** rounding 3.45 up. **Avoid:** floor to a tenth with integers (1.4). **Warning sign:** any `fmt.Sprintf("%.1f")` of points / 20. Test: walk 0 to 100 and assert the text is at least "3.5" exactly when the tag is Reliable.

### Pitfall 2: one of five callers misses the ratings
**What goes wrong:** the feed says Reliable (with taps) and a phone note, the Activity page or the vote response says Mixed. **Avoid:** one loader helper, the parity tests extended with a rated reporter, and a test that greps for `ReporterSignals(` and fails when a production caller lacks the option. `reporterIsReliable` in the dispatcher is the easy one to forget.

### Pitfall 3: fixtures flip silently
**What goes wrong:** the 71 fixtures keep "Mixed" in comments and tables. **Avoid:** run the full suite on a scratch database right after the constant change and record every flip in the first plan's SUMMARY (1.2).

### Pitfall 4: the follow note never reaches followers without a home area
Worker step 5 (no home area, out of range) applies to every kind but `changed`. Make follow explicitly exempt, test with a follower with no home area.

### Pitfall 5: two first alerts for one person and report
Reliable alert and follow note both written, or two outbox rows. **Avoid:** follow branch before reliable, shared unique index, Reliable bell step skips followed reporters, `ON CONFLICT` predicates updated to match the widened index (a mismatched inference predicate is a runtime SQL error, not a compile error).

### Pitfall 6: tag loss never ends the follows
The tag is not stored, so nothing "fires". **Avoid:** send time recheck + evaluate on touch + sweep (5.3). Test the decay case by moving the clock, not only the rating case.

### Pitfall 7: name collisions
`/profile` is Activity; "follow" is report level in `follow.go`; `report_mutes`; `Profile` handler name. New code uses `/u`, `/me`, `account_follows`, `person_mutes`, `person_blocks`, `profile_ratings`, `People*`.

### Pitfall 8: a finished profile row that opens the report page
`Focus` returns `too_old` after 7 days and, more importantly, does not coarsen a finished non Rescue report, so a finished profile row that links to it undoes the coarse location. Finished rows are not links (4.3, Q3).

### Pitfall 9: profile 404 tells you who opted in
Use one body and status for unknown, private, deleted and blocked; do not branch the message.

### Pitfall 10: block used to erase critics
Block never touches ratings; blocked people cannot open the profile, so they cannot change a rating, and that is accepted.

### Pitfall 11: Option A deletion moves other people's stars
Ratings given by a deleted account are removed; state it in TRUST-MODEL (G6).

### Pitfall 12: the person menu fan out
`account_header_menu` is included by many page structs. Link to constant `/me`, do not add a username to every page model.

### Pitfall 13: tests that pin "no profile yet"
List in Parts 4 and 6; amend each with a dated owner note, do not delete the checks that still hold (for example `TestNotificationsNoFollowOrOfficialRow`).

### Pitfall 14: 7.1 rewrites the test sign in
Use helper names only; re-grep at planning.

## Code Examples

### Eligibility and aggregate (sqlc shapes) [ASSUMED]
```sql
-- name: RatingTalliesForReporters :many
SELECT pr.reporter_id,
       COUNT(*)::int      AS raters,
       SUM(pr.stars)::int AS tap_sum
FROM profile_ratings pr
JOIN accounts rater    ON rater.id = pr.rater_id AND rater.deleted_at IS NULL
JOIN accounts reporter ON reporter.id = pr.reporter_id AND reporter.profile_public
WHERE pr.reporter_id = ANY(sqlc.arg(account_ids)::bigint[])
  AND pr.removed_at IS NULL
GROUP BY pr.reporter_id;
```

### The one shared worker guard for the follow kind [ASSUMED]
```go
case PushKindFollow:
	return MayNotifyAboutReport(UpdateFollowedPosted, f.Vis, f.ExpiresAt, run.now) &&
		w.followStillHolds(ctx, run) // row exists, notify on, reporter public, Reliable now, no mute or block
```

### Migration widening (shape only, number at execution time) [ASSUMED]
```sql
ALTER TABLE push_outbox DROP CONSTRAINT push_outbox_kind_valid;
ALTER TABLE push_outbox ADD CONSTRAINT push_outbox_kind_valid
    CHECK (kind IN ('critical','reliable','changed','confirmed_update','follow'));
DROP INDEX uq_push_outbox_first;
CREATE UNIQUE INDEX uq_push_outbox_first ON push_outbox (account_id, report_id)
    WHERE kind IN ('critical','reliable','follow');
ALTER TABLE updates DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates ADD CONSTRAINT updates_kind_valid CHECK (kind IN
    ('reached_live','hidden','resolved','fading','reliable_nearby','followed_posted','more'));
```
Read the current constraint text in the folder before writing it: Phase 5 or another phase may have widened the same lists first. The Down block restores them exactly.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Reliable at score 75 (about 3.75 stars), outcomes only | 70 (3.5 stars), combined with tapped stars from 5 raters | D-05, 2026-10-17 | Fixtures at 71 flip, docs pinned, five callers read ratings |
| "Someone I follow posts" Settings switch | per person Notify me bell | 2026-10-18 | Settings row never built; kind name reserved in 3.3 |
| Non critical follower notes only once Live (ROADMAP SC4) | note from posting while Live or Provisional, "Not confirmed yet" | D-09 | Same predicate as the Reliable alert |
| Private follows (PROF-03 text) | public Followers and Following lists | D-03 | Lists need the named versus count split |

**Deprecated or outdated:** REQUIREMENTS.md PROF text (nickname, no follower counts, privately); ROADMAP SC3 placement and SC4 timing (replaced by D-06, D-09); the "remove all followers without seeing who they are" wording (D-12).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 7.1 adds `accounts.username`, unique case insensitive, charset without `@` and `/`, and exports a reusable reserved word check | Dependency contract | Routes and Name screening need rework; planner must grep merged 7.1 |
| A2 | `MixedMinScore` stays 35 points (1.75 stars, shown 1.7) since D-05 says "mapped the same way" | Part 1 | If the owner meant to move the Mixed floor too, one constant and the anchor rows change |
| A3 | Opt in ("Let others follow me") is available to anyone; only following requires the Reliable tag; a public non Reliable profile shows no Follow button and a quiet line | Part 3, Q2 | If opt in is Reliable only, the switch hides and the profile of a non Reliable person never exists |
| A4 | Finished non rescue reports on a profile use a precision 5 cell (about 5 km), no description; Rescue keeps precision 6 and no text | Part 4 | Too coarse or too fine for the owner's taste; one constant |
| A5 | Action routes are POST verb paths, not PUT and DELETE | Part 4 | Style only |
| A6 | No username on the follow note lock screen | Part 5 | Owner may want the name; costs a new copy field and a test change |
| A7 | Letter avatar, no picture upload | G5 | Owner may expect uploads (violates no images and adds moderation) |
| A8 | Ratings count toward the tag only while the reporter's profile is public; they are kept (not deleted) when it is switched off | Part 1, Part 2 | If they always count, the tag depends on hidden data; if deleted, opt out wipes critics |
| A9 | Profile lists Disputed reports too | Part 4 | Flattering omission if dropped |
| A10 | Two migrations (schema; notification kinds) rather than one | Part 3 | Ordering only |
| A11 | Widen `push_outbox` for the follow kind rather than a sibling table | Standard Stack | A sibling table is possible but duplicates the confirmed update machinery |
| A12 | A rating change allowed at most once per reporter per day; limits otherwise per account | Part 2 | Numbers only |
| A13 | Phase 3.3 plan 15 may land before this phase and then owns `docs/PUSH.md` | Part 5 | Docs plan placement |

## Open Questions

1. **Does the home area show on an opted in profile (Q1)?**
   - Known: ROADMAP owner decision of 2026-10-16 says name and home area both show on an opted in profile. D-13's header lists Name but not home area. Settings says the home area is "Used for alerts, never your exact spot".
   - Unclear: which wins.
   - Recommendation: show the Name; show the home area name only if the UI mockup session says so; never the cell. It is the one field that says where a person lives.
2. **Can a person who is not Reliable switch the profile on (Q2)?** D-01 says only a Reliable person can be followed; SC5 implies the flag can outlive the tag. Recommendation A3: yes, with no Follow button for them until they hold the tag.
3. **Do finished rows open the report page (Q3)?** Recommendation: no. Only live rows are links; finished rows are plain, so the coarse location holds. Do not widen `HistoryWindow` for this phase. Changing this needs the report page to coarsen finished reports for non posters (Phase 8).
4. **Ramp the tapped half (Q4).** Not reopening D-02. Optional disclosure to the owner: five eligible raters move a score about 30 points and carry half the weight at 5 as at 500. A ramp (for example full half weight only from 15 raters) would change D-02's wording. Default: build D-02 exactly, document the arithmetic.
5. **The popup score line (Q5).** "Score N out of 100." stays (recommended, N is the combined points). Whether popups and rows should show stars instead is a UI step decision; D-05 only fixes the cutoff in stars.
6. **Where mute is undone (Q6).** D-04 lists Blocked people but no Muted people row. Recommendation: Mute and Unmute in the profile menu, and a second heading "Muted" inside the Blocked people window.
7. **Who screens the Name (Q7).** Recommendation G1: this phase, unless the 7.1 plans take it; the planner greps 7.1 first.
8. **Username on a lock screen (Q8).** Recommendation A6: no.
9. **What "pending alerts" means in SC5 (Q9).** Recommendation: queued phone notes and unwritten notes end at once; bell rows already written stay as history and read "no longer available" if the follow is gone.
10. **If Phase 9 is cut, does the 70 bar stand alone (Q10)?** The constant change and the fixture and doc edits are independent of everything else and could ship as a small first plan or a quick task. That is the owner's call because it changes who is tagged Reliable.
11. **Policy text and terms version (Q11).** A public profile changes what the Policy should say, and editing it forces a new agreement for everyone. Owner decision.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build, tests | yes | 1.26.4 | none needed |
| Node | `node --test web/*.mjs` | yes | v26.4.0 | none needed |
| sqlc | regenerate queries | yes | v1.31.1 | none needed |
| swag | regenerate docs | yes | v1.16.4 (matches go.mod) | none needed |
| goose | migrations | yes | binary present, library v3.28.0 | `go run ./cmd/migrate` |
| psql, postgres, pg_ctl | scratch database for the DB tests | yes (Homebrew) | not probed further | docker is also present |
| Docker | Linux CI parity check (memory: how to run CI steps on Linux) | yes | not probed | skip |

[VERIFIED: command -v and version probes, 2026-10-25]. No database was connected to or created during this research. Missing dependencies with no fallback: none.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (package `service`, `handlers_test`, `web`, `api`) plus Node `node:test` for `web/*.mjs` |
| Config file | none; `Makefile` targets `test`, `test-short`, `vet`; CI in `.github/workflows/ci.yml` |
| Quick run command | `env -u DATABASE_URL go test ./internal/service/ ./web/ ./internal/api/... -short -count=1` then `node --test web/*.mjs` |
| Full suite command | `go test ./... -p 1` against a disposable scratch database, then `node --test web/*.mjs` |

**Database rule for every executor and subagent:** the suite's `testutil.NewTestDB` truncates the database `DATABASE_URL` points at. Never point it at `pinalert_test` or `pinalert` (they are the owner's). Create a uniquely named scratch database, refuse if the name exists, run `DATABASE_URL=postgres://localhost/<scratch>?sslmode=disable go test ./... -p 1`, drop it afterwards, and never use ports 8080 or 8090 for anything that listens. [CITED: project memory, Makefile]

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| PROF-02 | `CombinePoints` table (r=4,5,6, all ones, all fives, rounding half up) and `StarsText` walk 0 to 100 matches tag | unit | V1 | no, Wave 0 |
| PROF-02 | Tag cut at 70 and Mixed at 35; fixture flips; 6C2X = 70 exact, 4C2U = 69 | unit | V2 | yes, amend |
| PROF-02 | New reporter stays New whatever the ratings | unit | V3 | no, Wave 0 |
| PROF-02 | Doc constants table equals code (70) | unit | V4 | yes, amend doc |
| PROF-02 | Feed, vote response, history, Activity, dispatcher agree on a rated reporter | integration (DB) | V5 | partly, amend |
| PROF-02 | Rating eligibility (voted within rule, not self, public profile, unresolve does not count), upsert, clear, per account limit | integration (DB) | V6 | no, Wave 0 |
| PROF-02 | Removed rating and deleted rater stop counting; moderator routes 404 to others | integration (DB) | V7 | no, Wave 0 |
| PROF-01 | Opt in needs username, Name screened, fails closed on unfinished check | integration | V8 | no, Wave 0 |
| PROF-01 | Private, unknown, deleted, blocked give one identical 404 | integration | V9 | no, Wave 0 |
| PROF-02 | Profile reports: 30 day window, Withheld/None never listed, finished coarse at precision 5, Rescue keeps 03.1 D-23, distance to coarse point | integration | V10 | no, Wave 0 |
| PROF-02 | Followers and Following split named versus "N others", hidden never linked, keyset paging | integration | V11 | no, Wave 0 |
| PROF-03 | Follow rules (Reliable now, public, no block, not self), unfollow, remove follower, block effects both ways | integration | V12 | no, Wave 0 |
| PROF-03 | Switching off ends follows and queued follow pushes; tag loss ends them by sweep and at send time (clock moved) | integration | V13 | no, Wave 0 |
| PROF-03 | Follow note: Live and Provisional only, one per account and report with Reliable alert, "Not confirmed yet" then confirmed in place, same tag, no second ping, follower with no home area still notified, hourly cap | integration | V14 | no, Wave 0 |
| PROF-03 | `BuildPushText` follow case, no reporter text field, no noise words | unit | V15 | yes, amend |
| PROF-03 | Closed lists: `PushKinds` six, `ReservedPushKinds` empty, `UpdateKinds` | unit | V16 | yes, amend |
| PROF-01..03 | Settings rows, person menu, phone View my profile, size only names, touch targets, icons, plain punctuation | contract | V17 | yes, amend |
| PROF-01..03 | Star box after resolve, bell button, lists render with `textContent` | node | V18 | no, Wave 0 |
| API docs | New routes documented | contract | V19 | yes, amend |
| Gate | New routes answer 401 anonymously | integration | V20 | yes, amend |

Commands for the table, one per row. They are written as plain text because a pipe inside a table cell needs escaping, and a copied backslash pipe is a literal pipe in a Go regular expression and matches no test.

```
# V1
go test ./internal/service/ -run 'TestCombinePoints|TestStarsText' -short
# V2
go test ./internal/service/ -run 'TestReliabilityTable|TestReliabilityInvariants' -short
# V3
go test ./internal/service/ -run TestRatingsNeverRateANewReporter -short
# V4
go test ./internal/service/ -run TestTrustModelDocMatchesCode -short
# V5
go test ./internal/service/ ./internal/api/handlers/ -run 'TestFeedSignalsMatchCastVoteSignals|TestRatedReporterSameEverywhere'
# V6
go test ./internal/api/handlers/ -run TestRatingRoutes
# V7
go test ./internal/api/handlers/ -run 'TestModeratorRatings|TestRatingsIgnoreRemoved'
# V8
go test ./internal/api/handlers/ -run TestProfileOptIn
# V9
go test ./internal/api/handlers/ -run TestProfileNotFoundIsUniform
# V10
go test ./internal/service/ ./internal/api/handlers/ -run 'TestProfileReports'
# V11
go test ./internal/api/handlers/ -run TestPeopleLists
# V12
go test ./internal/api/handlers/ -run 'TestFollow|TestBlock|TestMute'
# V13
go test ./internal/service/ -run 'TestFollowSweep|TestFollowEndsOnSwitchOff'
# V14
go test ./internal/service/ -run 'TestFollowBell|TestFollowPush'
# V15
go test ./internal/service/ ./web/ -run 'TestBuildPushText|TestNoNoiseWordsInUserCopy' -short
# V16
go test ./internal/service/ -run 'TestPushKindsClosed|TestUpdateKinds' -short
# V17
go test ./web/ -short
# V18
node --test web/person_test.mjs web/votes_stars_test.mjs
# V19
go test ./internal/api/handlers/ -run TestSwaggerSpecCoversRoutes -short
# V20
go test ./internal/api/ -run TestAccessGate
```

**Executors treat "no tests to run" as a failure.** `go test -run` exits 0 when the pattern matches nothing, so a typo or a test not yet written passes silently. Run each Wave 0 command once before the code exists and see it fail for a missing test, then pass after.

### Sampling Rate
- **Per task commit:** the quick run command for the touched package.
- **Per wave merge:** the full suite on a scratch database plus `node --test web/*.mjs`.
- **Phase gate:** full suite green and `go vet ./...` before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/service/stars_test.go` (`CombinePoints`, `StarsText` walk)
- [ ] `internal/service/ratings_test.go`, `people_test.go`, `profilereports_test.go`, `followsweep_test.go`, `followbell_test.go`, `followpush_test.go`
- [ ] `internal/api/handlers/people_e2e_test.go`, `ratings_e2e_test.go`, `moderator_ratings_e2e_test.go`
- [ ] `web/person_contract_test.go`, `web/person_test.mjs`, `web/votes_stars_test.mjs`
- [ ] A seed helper in `internal/testutil` for a reporter with N judged reports plus raters (extends `SeedReliableReporter`)
- [ ] Framework install: none
- [ ] A seed for the owner's viewing session. The D-13 mockup viewing and the hands on test need at least one opted in Reliable reporter with 5 or more raters, or nothing on a profile can be followed. Build it as a Go seed helper run with `go run` against a scratch copy only (extend `SeedReliableReporter` in `internal/testutil`, plus accounts, a username, a Name and five eligible raters). Do not depend on Phase 5 demo accounts: 03.2 D-28 expected the Reliable set to stay empty until they exist, and Phase 5 is still a draft.

## Security Domain

`security_enforcement` is enabled (absent from config means on; it is `true`), ASVS level 1, block on high. [VERIFIED: .planning/config.json]

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no change | Existing gate; 7.1 replaces the sign in |
| V3 Session Management | no change | Existing session cookie, SameSite Lax |
| V4 Access Control | yes | Account from the gate only; follow rows keyed by caller; block both ways; rater eligibility server side; moderator group 404 to others |
| V5 Input Validation | yes | Strict JSON bodies with unknown fields refused, stars 1 to 5, username path validated against 7.1 charset before any query, Name through `cleanText` and `CheckText` |
| V6 Cryptography | no | none added |
| V7 Error handling and logging | yes | Logs hold ids and counts only; request log writes path only |
| V8 Data protection | yes | Part 7 privacy list; coarse locations; no `home_area_cell` on the wire |
| V11 Business logic | yes | Rating arithmetic and caps, follow caps, sweep, TRUSTX-03 dependency |
| V13 API | yes | swag documented, per account rate limits, uniform 404 |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| IDOR on remove follower, notify, unblock | Tampering | Every query keyed by the caller's account id from the gate; never from the body |
| Username enumeration of who opted in | Information disclosure | One 404 body and status; per account bucket |
| Stored XSS through username or Name | Tampering | `html/template` escaping, `textContent`, `TestNoMarkupSinksAnywhere` |
| Impersonating an agency in the Name | Spoofing | Reserved word match on word boundaries plus abuse screen (G1) |
| Brigading or farming stars | Tampering | Eligibility, caps, moderator removal, aggregate only, documented plus or minus 30 point swing, TRUSTX-03 named dependency |
| Block used to erase critics | Tampering | Block does not touch ratings |
| Scraping the public social graph | Information disclosure | Accepted by D-03; paged, bucketed, named only if opted in |
| Notification spam to followers | Denial of service | Account hourly cap, per report single note, optional per reporter fan out cap |
| Lock screen disclosure | Information disclosure | No username, no text, no coordinates in the push |
| CSRF on action routes | Tampering | POST with strict JSON and SameSite Lax cookie (DEC-L) |
| Referrer leak of identifiers | Information disclosure | Username in the path only; no email anywhere |

## Suggested split into plans and waves

Eleven plans in eight waves. This repo merges parallel worktrees, so two plans in one wave must not touch the same file. The files that several plans would touch are named here and each has one owner per wave:

- `internal/api/router.go` (route registration), the three generated docs files `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, and the route list in `internal/api/handlers/swagger_test.go`: only one plan per wave edits or regenerates them.
- The generated sqlc files (`internal/store/sqlc/*.go`, and `models.go` changes whenever a migration adds columns): only 09-02 and 09-08 (the two migrations) regenerate them; queries written by other plans are added to the `.sql` files before one of those regenerations, or the plan that adds a query regenerates alone in its wave.
- The `RateLimits` struct (`handlers/ratelimit_gate.go`), its defaults in `router.go` and the constants block in `cmd/server/main.go`: 09-03 adds every bucket this phase needs (rate a reporter, people reads, people actions) in one edit, so later plans do not touch them.
- `historyread.go`: 09-05 edits `buildFinishedView`, 09-07 edits `baseView`. They are in different waves.

**Wave 1 (foundation, no UI, disjoint files)**
- **09-01 Trust core.** `stars.go` (pure) and its constants, `ReliableMinScore = 70`, comments, every fixture flip, `docs/TRUST-MODEL.md` edits (1.2) and new anchor rows, the new constants added to `p20ConstFiles` and `p20Constants` and to the doc's constants table. Standalone and shippable. Files: `reliability.go`, `stars.go`, the four test files named in 1.2, `trust_model_doc_test.go`, `docs/TRUST-MODEL.md`.
- **09-02 Schema and queries.** Migration 1 (accounts flag, `account_follows`, `person_mutes`, `person_blocks`, `profile_ratings`), `people.sql`, `ratings.sql`, sqlc regenerate, account deletion hooks and their tests. Files: `migrations/`, `queries/`, `sqlc/`, `accountdelete.go`, `accountdelete_test.go`.

**Wave 2**
- **09-03 Ratings into the tag.** `RatingService`, `ReporterExtrasForAccounts`, `WithRatings`, the helper in all five callers, the parity tests, the rating routes, the `RateLimits` buckets, swag. Alone in its wave because it touches the five callers (`reporter.go`, `report.go`, `trust.go`, `historyread.go`, `auth.go`, `pushdispatch.go`), `router.go` and the docs output.

**Wave 3**
- **09-04 People service.** Opt in (with G1), follow, unfollow, remove follower, mute, block, lists, uniform 404, per account follow cap, swag. Its follow check ("Reliable now") needs 09-03's `WithRatings`, which is why it is a wave later. Files: `people.go`, `handlers/people.go`, `router.go`, docs output.

**Wave 4 (surfaces, disjoint files)**
- **09-05 Profile read and page.** `ProfileReports`, `CoarsenFinishedForProfile`, `buildFinishedView` flag, `person.html.tmpl`, `person.js`, `person.css`, star icon, `/me`, `/u/{username}`, `router.go` (the two HTML routes), the UI spec mockup viewing first.
- **09-06 Entry points and Settings rows.** Person menu item, phone View my profile row, People and reports rows and windows, test amendments. Files: `account_header.html.tmpl`, `settings.html.tmpl`, `settings.js`, the contract tests named in Parts 4 and 6. No `router.go`, no docs output.

**Wave 5**
- **09-07 Trust block link and the star box.** `reporter_profile_path` on report rows (`baseView`), `trust.js` link, `rate_prompt` on the resolve response, `votes.js` box with Skip, `handlers/votes.go`, docs output.

**Wave 6 (notifications, first to cut)**
- **09-08 Follow notes.** Migration 2, `followed_posted` bell kind, `follow` push kind, dispatcher `followBranch`, worker changes (distance exception, hourly cap list, confirmed update), copy, tests, `ReservedPushKinds` empty, the bell producer bounds (5.2). Files: `updates.go`, `updatesalerts.go`, `updatescopy.go`, `pushkinds.go`, `pushcopy.go`, `pushdispatch.go`, `pushworker.go`, `push.sql`, `updates.sql`, `handlers/updates.go`, sqlc and docs output.

**Wave 7 (disjoint files)**
- **09-09 Sweep and tag loss.** `followsweep.go`, wiring in `cmd/server/main.go`, send time recheck, the flapping copy. Depends on 09-04 and 09-08.
- **09-10 Moderator ratings.** List and remove routes in the moderator group (`router.go`), screen section, `docs/MODERATION.md`, docs output if those routes carry swag blocks. Depends on 09-03.

**Wave 8**
- **09-11 Docs, swagger, end to end and privacy sweep.** `docs/TRUST-MODEL.md` (stars, ratings, follows, notes, TRUSTX-03 arithmetic, honest limits), `docs/PUSH.md` if present, `make swag`, the privacy tests of Part 7, the full suite on a scratch database, and a REQUIREMENTS.md refresh request to the orchestrator.

**First to cut, in this order if time runs short** (the ROADMAP already says Phase 9 is the first phase to cut):
1. The phone push for follow notes (keep the bell note), then the whole follow note (09-08, 09-09 reduce to the end-of-follow on switch off and a send time check).
2. Person mute (G4) and the Muted list.
3. The star box after resolve (the stars can still be tapped on the profile).
4. The star ratings entirely, with moderator removal (the rating half of 09-03, 09-07, 09-10), leaving outcomes-only stars at the 70 bar.
5. Lists windows beyond counts.
Keep 09-01 even then only if the owner wants the 70 bar (Q10), and always keep the uniform 404 and the Name screening if profiles ship at all.

## Sources

### Primary (HIGH confidence, read from the working tree on main, 2026-10-25)
- `internal/service/reliability.go`, `reporter.go`, `trustcopy.go`, `trust_model_doc_test.go`, `reliability_test.go`, `reporter_test.go`, `trustfacts_test.go`, `trustwording_test.go`
- `internal/service/historyread.go`, `coarsen.go`, `history.go`, `profile.go`, `accountdelete.go`, `textcheck.go`, `follow.go`, `mutes.go`, `updates.go`, `updatesalerts.go`, `updatesservice.go`, `updatescopy.go`
- `internal/service/pushkinds.go`, `pushcopy.go`, `pushdispatch.go`, `pushworker.go`; `internal/store/queries/push.sql`, `updates.sql`, `reliability.sql`, `votes.sql`, `accounts.sql`; migrations 00002, 00004, 00006 to 00011
- `internal/api/router.go`, `gate_test.go`, `handlers/votes.go`, `handlers/trust.go`, `handlers/trust_e2e_test.go`, `handlers/swagger_test.go`, `handlers/places_e2e_test.go`
- `web/templates/settings.html.tmpl`, `account_header.html.tmpl`, `profile.html.tmpl`; `web/places_contract_test.go`, `settings_rows_contract_test.go`, `layout_rules_contract_test.go`, `js_sinks_contract_test.go`, `design_rules_contract_test.go`, `noise_words_contract_test.go`, `css_contract_test.go`; `web/static/js/votes.js`, `trust.js`
- `docs/TRUST-MODEL.md`, `docs/MODERATION.md`, `Makefile`, `sqlc.yaml`, `.github/workflows/ci.yml`, `.planning/config.json`
- `.planning/phases/09-followable-reporter-profiles/09-CONTEXT.md`, `07.1-CONTEXT.md`, `08-CONTEXT.md`, `03-CONTEXT.md` (D-10 to D-15), `03.2-CONTEXT.md` (D-29 and the 2026-10-18 amendment), `03.3-CONTEXT.md`, `03.3-03/12/14/15-PLAN.md`, `03.4-CONTEXT.md` (D-24b to D-27), `.planning/ROADMAP.md` (Phase 9, 7.1, 8, 999.8), `.planning/REQUIREMENTS.md`
- `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/seams.md`

### Secondary (MEDIUM confidence)
- `.planning/phases/05-official-feed-demo-mode/05-11-PLAN.md`, `05-PATTERNS.md`, `05-RESEARCH.md` (uncommitted drafts, read only; they may still change)

### Tertiary (LOW confidence)
- None. No web search was needed: the phase uses only libraries already in the repo. The stars arithmetic was checked with a throwaway Go program outside the repo using the same integer formula.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new libraries.
- Architecture: HIGH for the reuse points (every named function and line was read); MEDIUM for 7.1 and 8 outputs, which are assumptions until they have plans.
- Pitfalls: HIGH for those found in code or tests; the gaming arithmetic is exact for the stated formula.
- Open privacy and product choices: collected in Open Questions, not decided here.

**Research date:** 2026-10-25
**Valid until:** about 14 days, or until Phase 5, 7.1 or 8 plans land (they change `pushkinds.go`, the login test helpers and the comment queries this phase touches).
