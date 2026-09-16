---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
verified: 2026-09-16T20:30:00Z
status: human_needed
score: 5/5
behavior_unverified: 0
overrides_applied: 0
mode_guard:
  phase_mode: mvp
  goal_is_user_story: false
  note: >
    ROADMAP.md marks this phase Mode: mvp, but the phase's Goal field ("A user can confirm or
    dispute a report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed
    by one shared, concurrency-safe resolver everywhere it's shown.") does not match the required
    "As a <role>, I want to <capability>, so that <outcome>." format the MVP Mode Verification
    section's user-story guard requires. Per that guard, full MVP-mode verification (the User Flow
    Coverage table) is refused and NOT produced. This report instead runs the standard
    goal-backward methodology directly against ROADMAP's 5 explicit numbered Success Criteria,
    which are unambiguous and fully testable on their own. This is an escalation, not a silent
    substitution: a human should decide whether to fix the goal wording (e.g. re-run
    `/gsd mvp-phase 02` or hand-edit ROADMAP.md's Goal line) or clear the `Mode: mvp` flag for this
    phase, so future verification runs don't hit the same guard.
deferred:
  - truth: "A report's visibility state is identical on the triage view and the shareable card, in addition to feed/map/Activity"
    addressed_in: "Phase 6"
    evidence: "REQUIREMENTS.md Traceability maps COORD-04 (triage list) and COORD-08 (shareable card) to Phase 6; neither surface exists yet in the codebase, so SC4 is verified for every surface that currently exists (feed, map, Activity page) and the resolver's one-function architecture structurally extends to any future surface without change."
human_verification:
  - test: "D-18 GPS-denial hard block, live in a real browser (harvested from 02-05-PLAN.md Task 3's deferred <human-check>, 02-VALIDATION.md Manual-Only row 1)"
    expected: "Denying the browser's location prompt on Confirm/Dispute greys the buttons out and immediately re-enables them, shows the exact GPS-denial sentence in the row's .vote-error element, and sends NO network request to /api/reports/{id}/confirm|dispute (verified in DevTools Network panel — a request sent and then rejected is a failure even if the visible outcome looks similar). A granted prompt is cached in sessionStorage and not re-requested for a second vote in the same tab, but is re-requested in a new tab. Both the feed row and the map popup behave identically, and no state changes before the server responds (D-04)."
    why_human: "This is live browser permission-prompt behavior (grant/deny/timeout dialogs, DevTools Network panel inspection, sessionStorage inspection, throttled-network waiting-state observation) that cannot be exercised by a Go test or a static contract test. The automated contract test (TestVoteTransportHasNoLocationFallback, which ran and passed) proves no fallback CODE PATH exists in the source — it does not prove a real denied prompt in a real browser actually produces the correct on-screen error, re-enables the button, and sends zero requests."
  - test: "D-09/D-10/D-11 Provisional dimming+label, Hidden outline treatment, and the 'Show disputed reports' toggle, live in a real browser and both themes (harvested from 02-06-PLAN.md Task 3's deferred <human-check>, 02-VALIDATION.md Manual-Only row 2)"
    expected: "A fresh non-critical report shows BOTH a desaturated badge/border AND an explicit 'Unconfirmed' chip, on both the feed row and the map pin popup, and the dimming visibly wins over the report's actual age stage. A critical report shows neither treatment. With 'Show disputed reports' unchecked, a disputed report is absent from both list and map; checking it reveals the disputed report with the outline treatment (transparent badge, neutral ring, muted glyph, no severity tint) on both surfaces simultaneously, findable against the basemap; unchecking removes both together. The checkbox does not persist checked across a reload. An empty disputed result shows the specific 'No disputed reports nearby' copy, not a blank list. All of the above holds in dark mode too."
    why_human: "Visual rendering, cross-surface consistency, and reload-state behavior in a live browser. The project's own contract tests (TestVisibilityCascadeOverridesAgeRamp, TestShowDisputedUsesOneSharedQueryParam, etc., all of which ran and passed) prove the CSS selector specificity and source order WOULD win and that the client wiring is structurally correct — they do not prove the rendered page actually reads as visually distinct to a person looking at it, which is exactly the point of this decision (D-09's own wording: 'trust state exists; it is not legible' until a human confirms it reads that way)."
  - test: "TRUST-08 Mark Resolved / confirmation-gate / Reopen flow, live in a real browser (harvested from 02-07-PLAN.md's deferred <human-check>, third Manual-Only-equivalent row named directly in that plan)"
    expected: "Tapping 'Mark resolved' on someone else's report replaces the button set in place with an inline confirmation and sends NO request until the affirm button is tapped; Cancel restores the row with no request. Affirming as a non-reporter shows the 'recorded, awaiting agreement' toast and the report stays in the feed; affirming as the reporter shows the 'resolved' toast and the report disappears from both list and map immediately. The map popup offers the same three buttons, wrapping rather than causing a horizontal scrollbar. The Activity page lists the resolved report with the outline/'Resolved' treatment and a Reopen control; tapping Reopen shows the reopen-succeeded toast (never the pending one) and the row updates with no page reload, and the feed shows the report live again. Denying GPS on affirm hard-blocks with the GPS-denial copy and sends no request."
    why_human: "Live browser flow spanning two pages (feed and Activity/profile), toast timing/copy correctness, in-place DOM replacement without a reload, and a live GPS-denial interaction on the resolve path specifically. Automated e2e/contract tests already prove the server-side visibility transitions and the absence of client-side identity/threshold logic (TestReporterCanResolveOwnReportInstantly, TestReopenHasNoClientSideIdentityOrThreshold, etc., all of which ran and passed) — they do not exercise the actual click-through UX a person experiences."
---

# Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility Verification Report

**Phase Goal:** A user can confirm or dispute a report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed by one shared, concurrency-safe resolver everywhere it's shown.
**Verified:** 2026-09-16
**Status:** human_needed
**Re-verification:** No — initial verification

## Escalation: Mode/Goal-Format Mismatch (read before the rest of this report)

ROADMAP.md tags this phase `Mode: mvp`. The MVP Mode Verification methodology requires the
phase's Goal to be a literal User Story (`As a <role>, I want to <capability>, so that <outcome>.`)
before it will produce the narrowed User Flow Coverage table. Phase 2's actual Goal field is not
in that format (it reads as a declarative technical statement, not a first-person user story —
contrast Phase 1's and Phase 1.1's Goal fields, which both do start with "As a..."). Per the
verifier's own guard, this means: **do not silently apply MVP narrowing, and do not silently
ignore the mode flag either — surface it.** This is that surfacing. It does not block this
report's findings below, which instead verify directly against ROADMAP's 5 explicit, unambiguous,
individually-numbered Success Criteria (a superset of what a User Flow Coverage table would have
checked). A human should decide whether to correct the Goal field's wording or clear the `mode:
mvp` flag for Phase 2 so a future verification pass doesn't hit the same guard. See the
`mode_guard` frontmatter block above for the machine-readable form.

## Method

This is initial verification (no prior `02-VERIFICATION.md` existed). Must-haves were assembled
from ROADMAP.md's 5 Success Criteria plus the `must_haves` frontmatter (`truths`, `artifacts`,
`key_links`, and `prohibitions`) of all 8 phase plans (02-01, 02-02, 02-03a, 02-03b, 02-04, 02-05,
02-06, 02-07). Verification went beyond static reading: the project was built (`go build ./...`,
`go vet ./...`), and the **full test suite was executed once against a real local Postgres 16
instance** (a scratch `pinalert_verify_test` database, migrated via the project's own embedded
goose migrations and dropped afterward) — specifically so TRUST-09's concurrency claim and the
resolver's exhaustive test carry genuine, independently-observed behavioral evidence rather than
trusting SUMMARY.md's narration of a CI run this verifier did not itself watch run.

```
go test -p 1 ./...
ok  	pinalert/internal/api            0.667s
ok  	pinalert/internal/api/handlers   1.133s
ok  	pinalert/internal/auth           (cached)
ok  	pinalert/internal/mailer         (cached)
ok  	pinalert/internal/ratelimit      (cached)
ok  	pinalert/internal/service        0.204s
ok  	pinalert/internal/session        (cached)
ok  	pinalert/internal/store          3.485s
ok  	pinalert/internal/testutil       0.539s
ok  	pinalert/web                     0.441s
```

All packages passed with zero failures. `go vet ./...` reported nothing.

**Every 02-0N-PLAN.md was also scanned for planner-deferred `<human-check>` blocks** (per
`workflow.human_verify_mode: end-of-phase`, #3309). Three were found — in 02-05 (Task 3), 02-06
(Task 3), and 02-07 — and **no `02-UAT.md` exists yet**, meaning none of them have been run. These
are harvested into the `human_verification` list above and are the reason this report's status is
`human_needed` rather than `passed`, even though every automatable truth below verified clean.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A user can confirm or dispute another user's report, and concurrent votes on the same report never silently lose an update, verified by an automated concurrency test | ✓ VERIFIED | `internal/service/trust.go`'s `CastVote` + four HTTP routes (`internal/api/handlers/votes.go`) implement confirm/dispute. `internal/store/migrations/00004_create_votes.sql` is a deliberately append-only table with **no** unique constraint beyond the `id` primary key (`TestVotesHaveNoUniqueKeyBeyondPrimaryKey`, ran and passed). `TestCastVoteConcurrent` (8 goroutines, 8 distinct accounts, one report) and the discriminating `TestCastVoteConcurrentSameAccountKeepsEveryRow` (8 concurrent casts from **one** account, asserting the raw row count is 8, not 1 — proving no upsert/lock design) were **run against a real Postgres by this verifier** and both **PASS**. |
| 2 | A newly submitted non-critical report displays as "provisional" until a second independent confirmation arrives; a critical/rescue-needed report publishes at full visibility immediately, with no gate | ✓ VERIFIED | `internal/service/visibility.go`'s `Resolve()`: `criticalBypass()` (severity==critical OR category==rescue_needed) returns `VisibilityLive` unconditionally, checked *before* the Hidden and Provisional rungs. Non-critical reports gate on `ConfirmCells < IndependentAgreementThreshold` (=2). `TestResolve_ProvisionalGate`, `TestResolve`, and the e2e `TestIndependentConfirmsFlipProvisionalToLive` / `TestConfirmsFromOneCellStayProvisional` all ran and passed. (The rendered *legibility* of Provisional — dimming + chip, live in a browser — is a separate harvested human-verification item above, D-09.) |
| 3 | Only a vote from a distinct verified account AND a distinct geohash cell counts toward that independent confirmation; severity affects triage sort order only and can never by itself unlock full visibility | ✓ VERIFIED | `internal/service/trust.go`'s `independentCellCount` counts distinct geohash cells over an already-account-deduped slice (`CurrentVotesForReports`'s `DISTINCT ON (report_id, account_id, kind)`). `voterGeohashPrecision = 7` is computed **server-side** from raw lat/lon — `CastVoteInput`/`CastVoteRequest` carry `Latitude`/`Longitude` only (confirmed by reading `trust.go` and `votes.go` directly, and by `TestCastVoteRejectsUnknownBodyField`, which proves a client-supplied geohash field is rejected 400 by `DisallowUnknownFields`). `TestResolve_SeverityNeverBypassesGateAlone` and e2e `TestConfirmsFromOneCellStayProvisional` (two accounts, same cell, stays provisional) both ran and passed. Feed ordering (`internal/service/report.go`'s `Nearby`) stays distance-ascending regardless of severity; severity is applied client-side only for triage sort, never as a gate input. |
| 4 | A report's visibility state is identical everywhere it's shown — feed, map, triage view, shareable card — because one shared resolver function computes it | ✓ VERIFIED (for surfaces that exist; see `deferred`) | `grep -rn "Resolve("` across `internal/service` finds **exactly 3 call sites** for the package-level `Resolve` function: `internal/service/report.go:450` (feed/map, single `GET /api/reports` route serves both per `router.go`), `internal/service/auth.go:378` (Activity/profile page), `internal/service/trust.go:361` (the vote-cast response itself). `internal/store/queries/reports.sql` and `votes.sql` carry **no** visibility/vote predicate (`grep` confirms — `NearbyReports` is byte-for-byte what Phase 1 shipped). No JavaScript file computes visibility; `web/static/js/visibility.js` only renders the server's `visibility`/`visibility_reason` fields, with an explicit "unrecognised value renders as Provisional, never Live" fail-closed default. `TestFeedVisibilityMatchesCastVoteResponse` (e2e, asserts the cast response and the subsequent feed read agree) ran and passed. Triage view and shareable card (COORD-04/COORD-08) do not exist yet — correctly deferred to Phase 6, see `deferred` in frontmatter — so the verifiable claim for *this* phase covers every surface that currently exists. |
| 5 | A user (reporter or nearby confirmer) can mark a report resolved, removing it from the live feed | ✓ VERIFIED | `VoteKindResolution`/`VoteResolve` path: reporter's own resolve vote sets `ReporterResolved=true`, retracting instantly with no threshold (D-13); a non-reporter's resolve needs `IndependentAgreementThreshold` (2) distinct cells (D-14). `ListableInFeed` (`internal/service/feed.go`) returns `false` for `VisibilityRetracted` under **both** the default and `?show_disputed=true` views. `TestFeedNeverReturnsRetractedReports`, `TestReporterCanResolveOwnReportInstantly`, `TestReopenRequiresIndependentAgreement` (e2e) all ran and passed; UI wiring (Mark Resolved button inside `.vote-controls` on both feed row and map popup, `TestResolveButtonMountsInsideTheVoteControlsContainer`) was inspected and its contract test ran and passed. (The live click-through UX is a separate harvested human-verification item above, TRUST-08.) |

**Score:** 5/5 truths verified (all programmatically verifiable behavior; 0 left
present-but-behavior-unverified).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/service/visibility.go` | Pure `Resolve()` resolver, D-05..D-16 ladder | ✓ VERIFIED | Reviewed in full; import set is exactly `time`, no I/O, no mutable state, matches doc comments and plan must-haves exactly. |
| `internal/service/visibility_test.go` | Table-driven + exhaustive totality proof | ✓ VERIFIED | `TestResolve_IsTotalAndDeterministic` (27+ severity/category subtests) ran and passed. |
| `internal/store/migrations/00004_create_votes.sql` | Append-only votes table, no unique key beyond PK | ✓ VERIFIED | Read directly; `TestVotesHaveNoUniqueKeyBeyondPrimaryKey` ran and passed. |
| `internal/store/queries/votes.sql` | No upsert/lock clause | ✓ VERIFIED | `TestVotesQuerySourceHasNoUpsertOrLock` ran and passed. |
| `internal/store/votes_test.go` | Concurrency proof | ✓ VERIFIED | Ran against a real local Postgres; both concurrency tests pass. |
| `internal/service/trust.go` | Independence predicate, `VotingService.CastVote` | ✓ VERIFIED | Reviewed in full; matches D-03/D-13/D-14/D-16/D-17 exactly; no `net/http`/chi import. |
| `internal/api/handlers/votes.go` | Four vote routes, one factory | ✓ VERIFIED | Referenced by router.go; e2e tests pass; `CastVoteRequest` carries only latitude/longitude. |
| `internal/service/feed.go` | `ListableInFeed`, `ViewerContentVote` | ✓ VERIFIED | Reviewed; no SQL-side visibility logic. |
| `web/static/js/votes.js` | Shared vote-block builder, GPS hard-block (D-17/D-18) | ✓ VERIFIED (structurally) | Reviewed; `getVoterLocation` rejects (never falls back) on denial; contract tests (`TestVoteTransportHasNoLocationFallback`, `TestVoterLocationIsCachedPerSession`) ran and passed. Live-browser behavior is a harvested human-verification item. |
| `web/static/js/visibility.js` | Shared visibility-tag builder | ✓ VERIFIED | Reviewed; fail-closed unrecognised-value handling confirmed. |
| `web/static/js/activity.js` | Reopen affordance on Activity page | ✓ VERIFIED | Reviewed; no client-side identity/threshold logic (`TestReopenHasNoClientSideIdentityOrThreshold` passed). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `report.go` (feed/map) | `visibility.go` `Resolve()` | direct call | ✓ WIRED | Confirmed by grep + read. |
| `auth.go` (Activity) | `visibility.go` `Resolve()` | direct call | ✓ WIRED | Confirmed by grep + read. |
| `trust.go` (`CastVote`) | `visibility.go` `Resolve()` | direct call | ✓ WIRED | Confirmed by grep + read; `TestFeedVisibilityMatchesCastVoteResponse` proves the two paths agree. |
| `votes.js` | `POST /api/reports/{id}/{confirm,dispute,resolve,reopen}` | `fetch` | ✓ WIRED | `TestVoteButtonsMeetTouchTargetAndUseTokensOnly`, `TestBothSurfacesMountTheSameVoteBlock` confirm one shared builder drives both feed and map. |
| `router.go` | `requireVerifiedAccount` gate | route group | ✓ WIRED | `TestCastVoteRequiresVerifiedAccount` (e2e) passed. |

### Prohibitions (must_haves.prohibitions — the must-NOT sibling to truths)

Every plan's `prohibitions:` block was enumerated and checked directly against the codebase
(grep/read for structural claims, the already-run test suite for test-backed claims). All items
below resolved with concrete enforcement evidence — none is left unverified/flagged.

| Plan | Prohibition (paraphrased) | Verification | Evidence |
|------|---------------------------|---------------|----------|
| 02-01 | `visibility.go` imports only `time`; no I/O, no `context.Context`, no package state | test+read | Read in full: `import "time"` is the only import; `Resolve`/`isRetracted`/`criticalBypass` are pure functions with no I/O or package vars. |
| 02-01 | No second independence number for non-reporter reopen; only reporter-instant paths are boolean | read | `isRetracted()` reads `IndependentAgreementThreshold` for both `ResolveCells` and `ReopenCells`; `ReporterResolved`/`ReporterReopened` are plain booleans with no threshold arithmetic. |
| 02-01 | No reporter carve-out on Hidden/Provisional rungs | read | The Hidden rung (`DisputeCells >= threshold`) and Provisional rung (`ConfirmCells < threshold`) reference no reporter-identity field at all. |
| 02-01 | No cached `confirm_count`/status column, no dampening curve | read | `ReportMeta`/`VoteTally` have no such fields; grep for `confirm_count` across `internal/` finds no such column anywhere in this phase's migrations. |
| 02-02 | No UNIQUE/PK over (report_id, account_id, kind) | test | `TestVotesHaveNoUniqueKeyBeyondPrimaryKey` — ran, passed. |
| 02-02 | No upsert-conflict/row-lock clause in votes.sql | test | `TestVotesQuerySourceHasNoUpsertOrLock` — ran, passed. |
| 02-02 | No cached confirm/dispute/status column on `reports` | read | Migration 00004 only adds `votes`; no `ALTER TABLE reports` anywhere in this phase. |
| 02-02 | No `internal/service` import from any file this plan touches | grep | `grep "pinalert/internal/service"` over all 02-02 `files_modified` — zero matches. |
| 02-03a | No second independence predicate | read | `independentCellCount` is the only distinctness function in `internal/service`; grep confirms no duplicate. |
| 02-03a | No geohash string accepted from caller | read | `CastVoteInput` has `Latitude`/`Longitude` fields only — no `GeohashCell` field. |
| 02-03a | No `net/http`, no chi, no handler type in `trust.go` | grep | `grep -n "net/http\|go-chi\|http\."  internal/service/trust.go` — zero matches. |
| 02-03b | No auth check inside `votes.go` (identity from context only) | grep | `votes.go` reads `account.FromContext(r.Context())`; no password/JWT/credential-check code present. |
| 02-03b | No visibility/tally/independence logic in the handler | grep | `grep "Resolve(\|BuildVoteTally(\|independentCellCount(" internal/api/handlers/votes.go` — zero matches; the handler only calls `service.VotingService.CastVote`. |
| 02-03b | No `r.Route`/`r.Mount` subrouter for vote paths | grep | `grep "r.Route\|r.Mount" internal/api/router.go` — zero matches (router's own comment confirms flat registration). |
| 02-03b | No geohash/visibility/kind/value field on `CastVoteRequest` | read | `CastVoteRequest` struct has exactly two fields: `Latitude`, `Longitude`. |
| 02-04 | No visibility/vote predicate in `reports.sql` | read | `NearbyReports` query read in full — bounding-box + Haversine only, no votes-table reference. |
| 02-04 | No `Resolve`/`BuildVoteTally`/`VoteTally` call from `internal/api/handlers` | grep | `grep "service.Resolve(\|service.BuildVoteTally(\|service.VoteTally{" internal/api/handlers/reports.go` — zero matches. |
| 02-04 | No reporter account id/email in feed response | read | `FeedReportResponse` struct read in full — no account/email field; explicit doc comment states `is_own_report` is boolean-only (T-01-02). |
| 02-04 | No second report read route | grep | Exactly one `r.Get("/api/reports", ...)` in `router.go`. |
| 02-04 | No cached visibility/status/confirm_count column on `reports` | read | Confirmed — no such migration exists. |
| 02-05 | No fallback location / default centre in `votes.js` | test | `TestVoteTransportHasNoLocationFallback` — ran, passed. |
| 02-05 | No optimistic update (no state change before server response) | read+test | `setBlockBusy` disables before fetch, `updateVoteBlock` only called after response; `TestVoteButtonsMeetTouchTargetAndUseTokensOnly` family passed. |
| 02-05 | No client-computed geohash / extra POST field | read | `castVote` sends `{latitude, longitude}` only (confirmed reading `votes.js`). |
| 02-05 | No Mark resolved / resolve UI in this plan's scope | superseded within-phase | `git log --oneline -- web/static/js/votes.js` shows the file created in 02-05 (`81179bb`, `41d2573`) with no resolve UI, then extended in 02-07 (`427af3f feat(02-07): implement Mark Resolved with inline confirmation`) — Mark Resolved was correctly and deliberately added to the same file one wave later, by design, not a violation of this prohibition's scope. |
| 02-05 | No `innerHTML`/`insertAdjacentHTML`/`outerHTML`/`document.write` | grep | Zero matches across `votes.js`, `visibility.js`, `activity.js`, `feed.js`, `map.js`. |
| 02-06 | No vote-casting change (`votes.js` untouched by 02-06's diff) | test (git) | `git log --oneline --all --grep="02-06"` lists five 02-06 commits; none touches `web/static/js/votes.js` (`git log --oneline -- web/static/js/votes.js` shows only 02-05 and 02-07 commits). Confirmed, not inferred. |
| 02-06 | No "confirmed by N nearby" count/number string | grep | `grep -i "confirmed by" web/static/js/*.js web/templates/*.tmpl` — zero live matches (one is an explanatory code comment, not rendered copy). |
| 02-06 | No client-side visibility computation | read | `visibility.js` only reads `report.visibility`/`report.visibility_reason` fields — no derivation. |
| 02-06 | No new CSS custom property / raw colour literal in `trust.css` | grep | `grep -oE "^[[:space:]]*--[a-z-]+:" web/static/css/trust.css` returns exactly the three properties the prohibition names as pre-existing (`--badge-glyph-fg`, `--severity-current`, `--severity-tint`) and nothing else — checked directly, not deferred. |
| 02-07 | No client-side identity/threshold mechanism in the reopen path | grep+test | `grep "is_own_report\|threshold" web/static/js/activity.js` — only explanatory comments, no logic; `TestReopenHasNoClientSideIdentityOrThreshold` ran and passed. |
| 02-07 | No new SQL query / no `internal/store/` change / no swag regen | test (git) | `git log --all --oneline --name-only --grep="02-07"` shows zero commits touching any `internal/store/` or `docs/swagger*` path. Confirmed, not inferred. |
| 02-07 | No retraction/expiry/vote predicate in SQL | read | Confirmed — `reports.sql`/`votes.sql` carry no such predicate (same evidence as 02-04's equivalent prohibition). |
| 02-07 | No new `VOTE_ACTIONS` entry / no transport change | read | `VOTE_ACTIONS = ['confirm', 'dispute', 'resolve', 'reopen']` — exactly the four values named since 02-05. |
| 02-07 | No general voting-history list on the Activity page | read | `activity.js`'s own comment and `ActivityForAccount`'s doc comment both confirm it lists only the account's own submitted reports. |
| 02-07 | No second toast implementation | grep | `showToast`/`#toast` defined once, in `app.js`; `modal.js` delegates to it (confirmed by grep). |

None of these prohibitions required an `unverified-prohibition` flag — every one has direct
enforcement evidence: a passing named test, unambiguous source reading, a grep with zero matches,
or (for the "no diff in this plan's commits" claims) a `git log`/`git show` check against the
actual phase commit history rather than an inference from current file contents alone.

### Behavioral Spot-Checks (run by this verifier, not sourced from SUMMARY.md)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TRUST-09 concurrency (N distinct accounts) | `go test ./internal/store/... -run TestCastVoteConcurrent` against real local Postgres | PASS, 8/8 rows persisted | ✓ PASS |
| TRUST-09 discriminating proof (same account, no upsert) | `go test ./internal/store/... -run TestCastVoteConcurrentSameAccountKeepsEveryRow` | PASS, raw count = 8 (would be 1 under upsert) | ✓ PASS |
| No unique key beyond PK on `votes` | `go test ./internal/store/... -run TestVotesHaveNoUniqueKeyBeyondPrimaryKey` | PASS | ✓ PASS |
| Resolver totality/determinism | `go test ./internal/service/... -run TestResolve_IsTotalAndDeterministic` | PASS (27 subtests) | ✓ PASS |
| Full workspace suite (single run) | `go test -p 1 ./...` against real local Postgres | All packages PASS, 0 failures | ✓ PASS |
| Build / static check | `go build ./...`, `go vet ./...` | Clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| TRUST-01 | 02-02, 02-03a, 02-03b, 02-05 | User can confirm or dispute another user's report | ✓ SATISFIED | Four vote routes, shared vote-block UI, e2e-proven end to end. |
| TRUST-02 | 02-01, 02-04, 02-06 | One shared resolver, identical everywhere shown | ✓ SATISFIED | Exactly 3 `Resolve()` call sites; no SQL/JS re-derivation. |
| TRUST-03 | 02-03a, 02-03b, 02-05 | Distinct account AND distinct geohash cell for independence | ✓ SATISFIED | `independentCellCount` over server-computed geohash cells; e2e-proven. |
| TRUST-04 | 02-01, 02-04, 02-06 | Provisional gate; critical bypass with no gate | ✓ SATISFIED | `criticalBypass()`, exhaustive resolver tests, e2e provisional/live transitions. |
| TRUST-06 | 02-01, 02-04 | Severity is triage-sort-only, never a gate | ✓ SATISFIED | `TestResolve_SeverityNeverBypassesGateAlone`; feed ordering unaffected by severity. |
| TRUST-08 | 02-03a, 02-03b, 02-04, 02-07 | Reporter or nearby confirmer can mark resolved | ✓ SATISFIED | D-13/D-14/D-16 implemented and tested; Activity page Reopen wired. Live click-through UX is a harvested human-verification item. |
| TRUST-09 | 02-02, 02-03a | Concurrent votes never silently lost, automated concurrency test | ✓ SATISFIED | Concurrency tests run directly by this verifier against a real Postgres; both pass. **Note:** `.planning/REQUIREMENTS.md`'s Traceability table still lists TRUST-09 as "Pending" (a stale bookkeeping row — every other Phase 2 TRUST-* row reads "Complete"). This is a documentation-tracking gap, not a code gap; recommend updating that one row as trivial housekeeping, not a phase blocker. |

No orphaned requirements: all IDs REQUIREMENTS.md maps to Phase 2 (TRUST-01, 02, 03, 04, 06, 08, 09) appear in at least one plan's `requirements:` frontmatter field.

### Anti-Patterns Found

None. Scanned all 39 files listed in `02-REVIEW.md`'s `files_reviewed_list` for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches. `go vet ./...` is clean.

### Advisory Item Carried Forward from Code Review (not a phase must-have)

`02-REVIEW.md` (completed same day, 0 BLOCKER / 1 WARNING / 2 INFO) flags **WR-01**: the four vote-casting routes carry no rate or velocity limit, which is a real gap against the Core Value's "resistant to trivial gaming" framing (an attacker who registers several verified accounts could supply arbitrary distinct coordinates to defeat the independence predicate without physically being anywhere). This is correctly scoped as advisory rather than a plan must-have — no plan's `must_haves.artifacts` or `truths` names rate limiting, and ROBUST-04 (session-based rate limiting) is explicitly Phase 4 scope. Flagged here for visibility, not as a gap blocking this phase.

### Human Verification Required

Three items, harvested from planner-deferred `<human-check>` blocks in 02-05, 02-06, and 02-07
(see frontmatter `human_verification` for the full detail each item needs — reproduced in summary
here):

1. **D-18 GPS-denial hard block** (feed + map, both surfaces) — a denied location prompt must send
   zero network requests and show the exact denial copy; a granted prompt must be cached per
   session, not per vote.
2. **D-09/D-10/D-11 Provisional dimming+label and Hidden outline treatment, plus the "Show
   disputed reports" toggle** — must render identically on feed and map, in both light and dark
   themes, and the toggle's checked state must not survive a reload.
3. **TRUST-08 Mark Resolved / confirmation gate / Reopen click-through flow** — the inline
   confirm/cancel exchange, the outcome toast copy (reporter-instant vs. confirmer-pending), and
   the Activity-page Reopen control's single-outcome behavior, all live in a browser.

No `02-UAT.md` currently exists recording these as run. Recommend running them via
`/gsd-verify-work 2` per `workflow.human_verify_mode: end-of-phase`, or manually against the setup
steps each plan documents, before treating Phase 2 as fully closed.

### Gaps Summary

No code gaps found. All 5 ROADMAP success criteria are verified against the actual codebase — not
merely claimed in SUMMARY.md — through direct code inspection, static analysis (`go vet`), and by
independently executing the phase's test suite (including the concurrency-critical `internal/store`
package) against a real local Postgres 16 database created for this verification and dropped
afterward. Every `must_haves.prohibitions` item across all 8 plans was checked and holds. The
report's status is `human_needed` rather than `passed` solely because three planner-deferred
`<human-check>` items (GPS-denial UX, Provisional/Hidden visual legibility, and the Mark
Resolved/Reopen click-through flow) have not yet been run in a real browser — this is expected,
intentional process per `workflow.human_verify_mode: end-of-phase`, not a defect. Two additional
non-blocking notes are carried forward: a one-row documentation staleness in REQUIREMENTS.md's
Traceability table (TRUST-09 marked "Pending"), and the pre-existing WARNING from 02-REVIEW.md
about vote-route rate limiting.

---

_Verified: 2026-09-16_
_Verifier: Claude (gsd-verifier)_
