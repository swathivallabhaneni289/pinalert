---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "07"
subsystem: trust-mechanic-ui
tags: [javascript, css, go, html-template, resolve, reopen, trust-mechanic]

requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility
    provides: "02-01 Resolve()/BuildVoteTally, 02-03a VotingService.CastVote (reporter-instant reopen amendment), 02-05 votes.js/trust.css/VOTE_ACTIONS, 02-06 visibility.js/VISIBILITY_TAG_LABELS"
provides:
  - "Mark resolved button + inline destructive confirmation on feed row and map popup"
  - "Shared Pinalert.showToast, the single #toast owner across the app shell and profile page"
  - "service.ActivityForAccount — the Activity page's batched, resolver-backed trust state"
  - "Reopen control on the Activity page, instant for the reporter (D-16 amended)"
affects: [profile-page, feed-page, map-page, votes-transport, visibility-display]

tech-stack:
  added: []
  patterns:
    - "Fixed-copy exception, licensed structurally: a client-side toast can be hardcoded to one outcome only when the page's own listing scope (viewer's own reports) plus a server-side guarantee together make that outcome the only reachable one — the row's chip/state/control removal still read the response"
    - "Validate category/severity once, in the service layer, so the same validated struct feeds both the resolver's ReportMeta and the rendered class name"

key-files:
  created:
    - web/static/js/activity.js
    - internal/api/handlers/activity_e2e_test.go
  modified:
    - web/static/js/votes.js
    - web/static/js/app.js
    - web/static/js/modal.js
    - web/static/css/trust.css
    - web/static/css/main.css
    - web/static/css/modal.css
    - web/templates/profile.html.tmpl
    - web/votes_contract_test.go
    - web/account_menu_contract_test.go
    - internal/service/auth.go
    - internal/service/auth_test.go
    - internal/api/handlers/auth.go

key-decisions:
  - "The inline resolve confirmation nests INSIDE .vote-controls (one level deeper than 02-UI-SPEC.md's sketch) to reuse 02-05's single delegated listener rather than needing a second one"
  - "No new store query for the Activity page's retracted reports — ReportsByAccount already carries no expiry predicate; encoding retraction in SQL would be ARCHITECTURE.md Anti-Pattern 2"
  - "Category/severity validation moved from the handler into service.ActivityForAccount, so the resolver's inputs and the rendered class name can never disagree"
  - "Reopen's toast is hardcoded to the succeeded string — the one deliberate exception to 'read the server's answer' in this phase, licensed by the Activity page listing only the viewer's own reports plus 02-03a's server-guaranteed instant reporter reopen (D-16 amended 2026-09-15)"

patterns-established:
  - "One delegated click listener per new interactive block, dispatching on data-action — no per-button listeners anywhere in this plan's additions"
  - "CSS state rules that must match an existing rule property-for-property are checked with a test that reads both rules' declsOf() and asserts equality, rather than trusted by inspection (TestRetractedRowTreatmentMatchesHidden)"

requirements-completed: [TRUST-08]

coverage:
  - id: D1
    description: "Mark resolved renders inside .vote-controls on the feed row and map popup, for the reporter and non-reporter alike; a single tap opens an inline confirmation and sends nothing until the affirm button is tapped; the outcome toast is chosen from the server's returned visibility"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveButtonMountsInsideTheVoteControlsContainer"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveIsGatedByAnInlineConfirmation"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveOutcomeCopyIsChosenFromTheServerResponse"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveActionAllowlistIsUnchanged"
        status: pass
    human_judgment: true
    rationale: "Static text-inspection tests prove the DOM/handler structure and the no-network-until-affirm ordering; the actual tap-to-confirm interaction, real network timing, and visual rendering on both surfaces (including a Leaflet popup's wrap-not-scroll behavior) require a real browser — covered by this plan's end-of-phase <human-check> steps 1-4 and 7."
  - id: D2
    description: "One shared toast implementation: app.js's Pinalert.showToast is the single #toast owner; modal.js's submit toast and both new resolve/reopen outcomes route through it; its CSS moved from modal.css to main.css so the profile page (no modal.css) can show toasts too"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestToastHasExactlyOneImplementation"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestToastStylingLivesWhereBothPagesLoadIt"
        status: pass
    human_judgment: false
  - id: D3
    description: "The Activity page computes visibility through service.Resolve over service.BuildVoteTally, with one batched CurrentVotesForReports call and no new store query; category/severity are validated exactly once, in the service; every report the account filed is still listed, resolved or not; the false voting-history placeholder sentence is removed"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "internal/service/auth_test.go#TestActivityForAccountResolvesVisibility"
        status: pass
      - kind: unit
        ref: "internal/service/auth_test.go#TestActivityForAccountBatchesVoteReadsOnce"
        status: pass
      - kind: unit
        ref: "internal/service/auth_test.go#TestActivityForAccountValidatesCategoryAndSeverityOnce"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileShowsResolvedStateForAResolvedReport"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileShowsLiveReportsWithoutARetractedMarker"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileStillListsEveryReportTheAccountFiled"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileNoLongerPromisesVotingHistory"
        status: pass
    human_judgment: true
    rationale: "The chip's rendered text/label ('Resolved', 'Unconfirmed') is built client-side by activity.js/visibility.js at page load. The e2e tests prove the server-rendered hooks (data-visibility, the vis-* class) are correct; the actual rendered label requires a real browser — covered by <human-check> step 5."
  - id: D4
    description: "Reopen renders only on retracted, unexpired rows; posts through the shared PinalertVotes.castVote transport with no confirmation step; its toast is always the reopen-succeeded string with no branch between two outcomes; the chip, state class and control removal all still come from the response's visibility; no ownership/account/threshold logic exists in the client"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestReopenButtonPostsThroughTheSharedTransport"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestReopenSuccessCopyIsAlwaysTheReopenedString"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestReopenHasNoClientSideIdentityOrThreshold"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestReopenAppliesTheServerAnswerRatherThanGuessing"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestRetractedRowTreatmentMatchesHidden"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileReopenIsInstantForTheReporter"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileOffersReopenOnARetractedReport"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileWithholdsReopenOnAnExpiredRetractedReport"
        status: pass
      - kind: integration
        ref: "internal/api/handlers/activity_e2e_test.go#TestProfileWithholdsReopenOnANonRetractedReport"
        status: pass
    human_judgment: true
    rationale: "The reopen tap-and-toast flow, and the row updating out of the resolved treatment live without a page reload, needs a real browser — covered by <human-check> step 6."

duration: 15min (commit-to-commit; total session including codebase orientation was longer)
completed: 2026-09-15
status: complete
---

# Phase 2 Plan 07: Mark Resolved, Reopen, and the Activity Page's Real Trust State Summary

**Mark resolved (with an inline destructive confirmation, server-driven outcome copy) and Reopen (instant for the reporter, D-16 amended) now have a surface — the feed, the map popup, and the Activity page — closing TRUST-08 and Phase 2 itself.**

## Performance

- **Duration:** ~15 min of commit-to-commit implementation (six commits, test→feat per task); total session time including full codebase/context orientation was longer, appropriate for a 14-file, phase-closing plan.
- **Completed:** 2026-09-15
- **Tasks:** 3 (Mark Resolved; the Activity page's real trust state; Reopen)
- **Files modified:** 14 (2 created, 12 modified) — exactly the plan's declared `files_modified` list, verified via `git diff --stat` against the plan's base commit.

## Accomplishments

- **Mark resolved** renders inside `.vote-controls` on both the feed row and the map popup, for the reporter and non-reporter alike (D-13, D-15). A tap opens an inline two-button confirmation and sends nothing until the affirm button is tapped; the outcome toast is chosen by the new `PinalertVotes.resolutionOutcomeMessage(action, result)` from the server's own `visibility` field, never from identity or a threshold.
- **One shared toast**: `modal.js`'s private `showToast` implementation was promoted into `app.js` as `Pinalert.showToast`, the single `#toast` owner. Its CSS (the rule, `@keyframes toast-in`, and the reduced-motion override) moved from `modal.css` to `main.css`, folded into the existing reduced-motion block rather than a second one — the profile page loads `main.css` but not `modal.css`.
- **The Activity page's real trust state**: `service.ActivityForAccount` calls the existing `ReportsForAccount`, then exactly one `CurrentVotesForReports` over the whole page, then `BuildVoteTally` + `Resolve` per report — the same batched shape 02-04's `Nearby` uses. No new store query was created; `profile.html.tmpl` now loads `trust.css` and four client modules (`app.js`, `votes.js`, `visibility.js`, `activity.js`) in dependency order, and every row carries `data-report-id`/`data-visibility`/`data-visibility-reason`/`vis-*` hooks. The now-false "once voting launches" placeholder sentence is removed.
- **Reopen** renders only on rows that are both retracted and not yet expired (`data-can-reopen`). It posts through the unchanged `PinalertVotes.castVote('reopen', …)` transport — no new geolocation code, no confirmation step. Its toast is always the reopen-succeeded string (D-16 amended 2026-09-15), but the chip, state class, and control removal still read the response's visibility, never a guess.

## Task Commits

Each task followed a RED (`test`) then GREEN (`feat`) commit pair:

1. **Task 1: Mark Resolved**
   - `test(02-07): add failing tests for Mark Resolved (Task 1)` — `e6b7e54`
   - `feat(02-07): implement Mark Resolved with inline confirmation (Task 1)` — `427af3f`
2. **Task 2: the Activity page's real trust state**
   - `test(02-07): add failing tests for the Activity page's real trust state (Task 2)` — `4c33a21`
   - `feat(02-07): give the Activity page its real trust state (Task 2)` — `38c454b`
3. **Task 3: Reopen**
   - `test(02-07): add failing tests for Reopen (Task 3)` — `5cf8dca`
   - `feat(02-07): implement Reopen on the Activity page (Task 3)` — `a353c24`

## Files Created/Modified

- `web/static/js/activity.js` — **created.** The Activity page's whole client half: the chip render (via `visibility.js`'s existing builders) and Reopen's build/dispatch/server-answer application. Computes nothing, fetches nothing.
- `internal/api/handlers/activity_e2e_test.go` — **created.** `package handlers_test`, real router + real Postgres, reusing `reports_e2e_test.go`'s `newE2EServer` (already wires `deps.Votes`) and `votes_e2e_test.go`'s vote/session helpers rather than a second harness.
- `web/static/js/votes.js` — resolve button, `.resolve-confirm` block, `openResolveConfirm`/`closeResolveConfirm`, `resolutionOutcomeMessage`, ten new exported copy constants.
- `web/static/js/app.js` — `showToast` (lazily resolves `#toast`, no-ops if absent), exported.
- `web/static/js/modal.js` — private toast implementation removed; its one call site now calls `Pinalert.showToast`.
- `web/static/css/trust.css` — five resolve/confirm/cancel/reopen selectors, the confirmation block's layout, and `.vis-retracted` (identical to `.vis-hidden`, both selector forms).
- `web/static/css/main.css` — the moved `#toast` rule + `@keyframes toast-in`, folded into the existing reduced-motion block.
- `web/static/css/modal.css` — the toast rule/keyframes/reduced-motion override removed (moved to `main.css`).
- `web/templates/profile.html.tmpl` — `trust.css`, `#toast`, four deferred script tags, and per-row trust hooks; placeholder paragraph removed.
- `web/votes_contract_test.go` — 17 new tests across the three tasks, plus one rewritten assertion in `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`.
- `web/account_menu_contract_test.go` — doc comment corrected (assertion unchanged).
- `internal/service/auth.go` — `AuthQuerier.CurrentVotesForReports`, `ActivityReport`, `ActivityForAccount`.
- `internal/service/auth_test.go` — `fakeAuthQuerier.CurrentVotesForReports` + three new tests.
- `internal/api/handlers/auth.go` — `profileReport` gains five fields; `newProfileReport` takes `service.ActivityReport` and deletes its own category/severity fallback; `Profile` calls `ActivityForAccount`.

## Decisions Made

- **The inline resolve confirmation nests inside `.vote-controls`** (one level deeper than `02-UI-SPEC.md`'s sketch) rather than beside it, to reuse 02-05's single delegated click listener instead of needing a second one. Same markup, classes, and copy either way.
- **No new store query for the Activity page.** `02-PLAN-OUTLINE.md` and `02-04-PLAN.md` both anticipated one would be needed; it wasn't, because `ReportsByAccount` already carries no expiry predicate and returns every report the account has ever filed. A bespoke "retracted reports" query would have encoded retraction in SQL — `ARCHITECTURE.md` Anti-Pattern 2, T-02-04.
- **Category/severity validation moved from the handler into the service** (`ActivityForAccount`), so the exact values the resolver saw and the values the template renders can never disagree — two copies could otherwise drift and render a report in one category's colour against another category's trust state.
- **Reopen's toast is the one deliberate fixed-copy exception in this phase.** The Activity page lists only the viewing account's own submitted reports, and 02-03a's amended `CastVote` makes a reporter's reopen instant and threshold-free (D-16 amended 2026-09-15) however the retraction arose. Together those two facts mean a 200 from this button always means the report is reopened, so the pending-outcome toast is structurally unreachable through it. The exception stops at the toast: the chip, the state class, and the conditional removal of the control block all still come from the response's visibility. `TestProfileReopenIsInstantForTheReporter` is the test that fails first if the server-side guarantee is ever weakened — at which point the fixed copy must move back to `resolutionOutcomeMessage`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestProfileShowsResolvedStateForAResolvedReport`'s own first draft asserted on client-rendered text**
- **Found during:** Task 2's GREEN run against real Postgres.
- **Issue:** The test's first draft checked the raw `GET /profile` HTML body for the literal string `"Resolved"` — but that text is built by `activity.js`/`visibility.js` client-side, never by the Go template. A Go HTTP test cannot execute JavaScript, so this assertion could never pass regardless of implementation correctness.
- **Fix:** Replaced the assertion with checks on the server-rendered hooks the client actually needs (`data-visibility="retracted"`, `data-visibility-reason="resolved"`), and added a comment stating the honest limit of this test's claim — the rendered chip text itself is the end-of-phase `<human-check>`'s job (step 5).
- **Files modified:** `internal/api/handlers/activity_e2e_test.go`
- **Verification:** `go test ./internal/api/handlers/ -run TestProfileShowsResolvedStateForAResolvedReport -p 1 -v` passes against real Postgres.
- **Committed in:** `38c454b` (Task 2 GREEN commit)

**2. [Rule 1 - Bug] `TestReopenHasNoClientSideIdentityOrThreshold`'s own forbidden-string list false-positived on its own explanatory comments**
- **Found during:** Task 3's GREEN run.
- **Issue:** The test's first draft banned the bare lowercase word `"threshold"` anywhere in `activity.js`'s embedded text. `stripCSSComments` only strips `/* */` blocks, not `//` line comments, so `activity.js`'s own doc comments explaining *why* no threshold is computed (e.g. "threshold-free") tripped the ban — the test was catching its own documentation, not a real regression.
- **Fix:** Narrowed the forbidden token to the capitalized identifier shape `"Threshold"` (matching `service.IndependentAgreementThreshold`), which still catches any real threshold-arithmetic regression while allowing the English word in prose.
- **Files modified:** `web/votes_contract_test.go`
- **Verification:** `go test ./web/ -run TestReopenHasNoClientSideIdentityOrThreshold -v` passes.
- **Committed in:** `a353c24` (Task 3 GREEN commit)

**3. [Rule 2 - Missing Critical] `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`'s plan-described "container-is-hidden assertion" did not exist in the shipped codebase**
- **Found during:** Task 1's RED-test authoring.
- **Issue:** The plan states this existing test's "container-is-hidden assertion is rewritten" to account for Task 1's supersession of 02-05's "the emptied `.vote-controls` container is hidden" truth. No such assertion existed in the test as shipped by 02-05/02-06 — the test never checked `block.controls.hidden` at all, only `applyOwnReportRule`'s `.remove()`/`disabled` behaviour.
- **Fix:** Added a new assertion (rather than rewriting a nonexistent one) confirming `applyOwnReportRule`'s body still computes `hasAnyButton` before deciding the container's `hidden` state — documenting that the mechanism is unchanged even though, after Task 1, it never actually hides the container on a real row (the resolve button always exists).
- **Files modified:** `web/votes_contract_test.go`
- **Verification:** `go test ./web/ -run TestOwnReportRuleRemovesControlsRatherThanDisablingThem -v` passes.
- **Committed in:** `e6b7e54` (Task 1 RED commit)

---

**Total deviations:** 3 auto-fixed (2 self-authored test bugs, 1 plan/codebase mismatch). **Impact on plan:** None affect production code correctness — all three are test-authoring corrections discovered and fixed during the same task's RED/GREEN cycle, before the corresponding GREEN commit. No scope creep.

### Scope notes (not deviations, recorded for the phase-level review)

- **`newProfileReport`'s `CanReopen` computation and `profile.html.tmpl`'s `data-can-reopen` attribute were implemented in Task 2's commit**, one function edit ahead of Task 3's own scope, since `newProfileReport` is a single function and splitting the retracted-visibility computation from the expiry-gated `CanReopen` computation across two commits would have meant editing the same function twice for no benefit. Task 3's own commit is therefore purely client-side (`activity.js`'s reopen button/click wiring, `trust.css`'s `.vis-retracted` rule) plus its own tests. All of Task 3's e2e tests (`TestProfileOffersReopenOnARetractedReport`, `TestProfileWithholdsReopenOnAnExpiredRetractedReport`, `TestProfileWithholdsReopenOnANonRetractedReport`, `TestProfileReopenIsInstantForTheReporter`) were written and passed against real Postgres as part of Task 2's own e2e file, since they share one test harness and don't conflict with Task 2's own claims.
- **`newActivityE2EServer` was never built.** The advisor flagged this as a likely blocker (the auth e2e harness historically had no `Votes` service wired), but `reports_e2e_test.go`'s existing `newE2EServer` already wires `deps.Votes: service.NewVotingService(queries)` — so `activity_e2e_test.go` reuses it directly, plus `votes_e2e_test.go`'s `newVerifiedClient`/`postVote`/`decodeVoteResponse`/`submitReportGetID`/`floodReportBody` and `auth_e2e_test.go`'s `submitReport`, rather than standing up a fourth harness.

## Two supersessions (named per the plan's `<output>` instruction)

1. **02-05's "the emptied `.vote-controls` container is hidden" truth is superseded.** `Mark resolved` now renders for the reporter too (D-13, D-15), so the container is never emptied in practice. `applyOwnReportRule`'s own logic (computing `hasAnyButton` before setting `hidden`) is unchanged — 02-05's own instruction predicted exactly this outcome. See deviation 3 above for the test-side note.
2. **`modal.css` owning the toast rules is superseded by `main.css`.** The profile page loads `main.css` and `app.js` but not `modal.css`/`modal.js`, and a page with no modal must not link the modal stylesheet to get one shared component.

## The one UI-SPEC divergence

The inline resolve confirmation nests **inside** `.vote-controls`, one level deeper than `02-UI-SPEC.md`'s sketch (which drew it as a sibling with no `data-action` attributes). A sibling would sit outside 02-05's one delegated listener and force a second one; nesting it preserves that contract. Same markup, classes, and copy either way — only the nesting differs.

## The one open UI-SPEC gap

A Retracted report past its own `expires_at` shows the "Resolved" chip and **no** Reopen control, with nothing on the row explaining why. This is correct behaviour — 02-03a's expiry rejection (step 4) is not scoped by vote kind the way its self-vote block is, and `NearbyReports` filters on expiry regardless of visibility, so a reopen there would 409 and, even if it somehow succeeded, restore nothing to the feed — but `02-UI-SPEC.md`'s Reopen section specifies the button unconditionally on Retracted rows and supplies no copy for the withheld case. The Copywriting Contract's "This report has expired." string is specified as a vote *error* (shown after a failed action), and repurposing it as a static row note would invent an unsanctioned string, so none was added. **Flagging this for the UI checker, not treating it as a defect.**

## The declined store query

**No "the account's retracted reports" SQL query was created.** `02-PLAN-OUTLINE.md` and `02-04-PLAN.md` both anticipated this plan would need one, on the reasoning that 02-04 excludes Retracted reports from the public feed by design. That reasoning doesn't transfer: `ReportsByAccount` is a different query that never excluded anything (its own SQL comment records "a person's own history does not vanish from their own profile when a report ages out of the public feed"). The visibility this page shows is computed by `service.Resolve` over one `CurrentVotesForReports` read, exactly like every other read path. `internal/store/` and `docs/` are untouched — verified via `git diff --stat`.

## The third missing asset

`profile.html.tmpl` was missing `app.js` as well as the two `trust.css`/`visibility.js` 02-06 flagged — `visibility.js` writes its chip text through `Pinalert.setText`, so without `app.js` the chip is never built at all. `votes.js` is a fourth, needed for the Reopen transport. All four now load, in dependency order (`app.js` → `votes.js` → `visibility.js` → `activity.js`), verified by `TestProfilePageLoadsTrustAssetsInDependencyOrder`.

## Issues Encountered

None beyond the three self-authored test-bug deviations documented above, all resolved within the same task's RED/GREEN cycle.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **Phase 2 is now complete.** All seven plans (02-01 through 02-07) have shipped; `02-VALIDATION.md`'s Per-Task Verification Map is now unblocked and its `TBD` Task/Plan/Wave columns can be backfilled as a phase-level step (deliberately outside this plan's own `files_modified`, per the same reasoning every sibling plan gave — several executors making scoped edits to one shared table would clobber each other).
- **Deferred to `STATE.md`'s Blockers/Concerns (carry this forward verbatim enough to act on):** 01.1's D-13 promised the merged Activity section would eventually list the account's confirm/dispute voting history alongside its submitted reports. This plan delivers the submitted-reports half with real trust state and defers the voting-history half to **IDENT-03** — it needs a second store query over votes cast on *other* accounts' reports, which is new scope, not a gap in TRUST-08.
- **Also carry forward:** the open UI-SPEC gap (expired-retracted rows show no Reopen control and no explanation) — a real gap for the UI checker to rule on, not a planner's silent call.
- All `go build ./...`, `go vet ./...`, `go test ./... -short`, and `go test ./... -p 1` (real Postgres) are green. Every test 02-05 and 02-06 shipped still passes, with only the one named assertion (`TestOwnReportRuleRemovesControlsRatherThanDisablingThem`) extended, none weakened.
- End-of-phase `<human-check>` (7 steps, `02-VALIDATION.md`'s third Manual-Only row) is the remaining verification: the actual tap-to-confirm flow, the map popup's wrap-not-scroll behaviour, the rendered "Resolved"/"Unconfirmed" chip text, and the GPS-denial hard block, all against a real browser and a running server.

## Self-Check: PASSED

- Verified all created/modified files exist on disk (`web/static/js/activity.js`, `internal/api/handlers/activity_e2e_test.go`, `web/static/js/votes.js`, `web/templates/profile.html.tmpl`, `web/static/css/trust.css`, `internal/service/auth.go`, `internal/api/handlers/auth.go`) via `[ -f ]`.
- Verified all six commits exist in `git log --oneline` for this plan (`e6b7e54`, `427af3f`, `4c33a21`, `38c454b`, `5cf8dca`, `a353c24`).
- Re-ran every task's `<acceptance_criteria>` and named `<verify>` commands; all pass.
- Re-ran the full `package web`, `internal/service`, and `internal/api/handlers` suites, plus `go test ./... -short` and `go test ./... -p 1` against real Postgres — all green.
- Confirmed via `git diff --stat` against the plan's base commit that exactly the plan's declared `files_modified` list changed — no forbidden surface (`internal/store/`, `docs/`, `feed.js`, `map.js`, `visibility.js`) was touched.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*
