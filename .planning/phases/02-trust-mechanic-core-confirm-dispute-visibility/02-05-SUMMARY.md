---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "05"
subsystem: ui
tags: [javascript, dom, geolocation, sessionstorage, trust-mechanic, accessibility]

# Dependency graph
requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility (plan 02-04)
    provides: >
      GET /api/reports carrying your_vote (confirm/dispute/null, key always
      present) and is_own_report per report, and the four POST
      /api/reports/{id}/{action} vote routes (02-03a/02-03b) this plan's
      transport layer calls.
provides:
  - "window.PinalertVotes: a client-side module (getVoterLocation, castVote, createVoteBlock, updateVoteBlock, setBlockBusy, showVoteError, plus GPS_DENIED_MESSAGE/GENERIC_FAILURE_MESSAGE) that is the entire client half of the confirm/dispute mechanic"
  - "A single .vote-controls/.vote-error DOM block mounted identically on both the feed row (feed.js) and the map popup (map.js), so a visitor can tap Confirm or Dispute on either surface — the phase's first user-clickable increment"
  - "web/static/css/trust.css, token-only styling for the vote controls, linked between feed.css and auth.css"
  - "web/votes_contract_test.go (package web), 8 new contract tests plus the shared jsFunctionBody helper, reusing the package's existing stripCSSComments/windowAfter/findTagWindow/parseCSSRules/declsOf/ruleBySelector"
affects: [02-06 (visibility tags, show-disputed toggle), 02-07 (Mark Resolved / Reopen, Activity page)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Container-level event delegation for vote buttons (one click + one keydown listener per .vote-controls, not per button) so a future third button needs no new wiring"
    - "aria-pressed (not a CSS class) as the sole active-state mechanism, so the button label never has to change and one attribute serves both the a11y tree and the CSS selector"
    - "setBlockBusy as the single owner of the disabled attribute; updateVoteBlock owns aria-pressed and the own-report removal only — two non-overlapping responsibilities so a background poll landing mid-vote can neither re-enable an in-flight button nor leave a stale pressed state"
    - "D-04's no-optimistic-update rule implemented as: disable on tap -> POST -> keep busy across the mandatory Pinalert.fetchReports() refetch -> release busy only once the re-render has applied the server's own your_vote"
    - "sessionStorage-backed per-session geolocation cache, consulted before ever raising the native prompt, with a hard REJECT (no fallback centre) on denial/unsupported/timeout — the deliberate structural opposite of modal.js's initLocation()/showLocationDenied() and map.js's centerOnVisitor()"

key-files:
  created:
    - web/static/js/votes.js
    - web/static/css/trust.css
    - web/votes_contract_test.go
  modified:
    - web/templates/index.html.tmpl
    - web/template_contract_test.go
    - web/static/js/feed.js
    - web/static/js/map.js

key-decisions:
  - "window.PinalertVotes = { ... } is assigned as a plain IIFE's internal last statement (not the window.X = (function(){ return {...}; }()) pattern map.js/app.js use) — this is the only shape that produces the literal substring `window.PinalertVotes = {` the plan's own acceptance gate greps for exactly once; noted as a minor structural deviation from the plan's 'same style as map.js' prose, resolved in the direction the machine-checked acceptance criterion required"
  - "template_contract_test.go's appModules gained votes.js immediately after app.js (its actual template position) rather than first in the slice as the plan's action literally said — the plan's own two instructions ('place first' vs 'matching the order it will appear in the template') contradicted each other since votes.js loads after app.js; the loop only asserts each module's position against the vendor bridge, never relative order among app modules, so this has no test-behavior consequence and matches the template's real order"
  - "Chose two specific literal fingerprints (`Pinalert.config`, `setView(`) rather than bare substrings like `config` for TestVoteTransportHasNoLocationFallback's fallback-centre detector, so the plan-mandated prose describing the submission path's 'configured default position' by concept does not collide with the gate that is supposed to catch the identifiers themselves"
  - "Every comment naming the forbidden literal 'disabled' was kept out of the doc comments directly preceding updateVoteBlock and setBlockBusy, because jsFunctionBody's window for a function X spans from X's own declaration to the NEXT function's declaration line — so a doc comment written directly above function Y lands inside the PRECEDING function's checked window, not Y's own. Comments describing the busy-state ownership rule use paraphrase ('interactive attribute', 're-enable') instead of the literal word where that would have landed in applyOwnReportRule's or updateVoteBlock's own (must-not-contain-disabled) windows"

requirements-completed: [TRUST-01, TRUST-03]

coverage:
  - id: D1
    description: "Vote transport (getVoterLocation + castVote): sessionStorage cache consulted before the native geolocation prompt, hard rejection (no fallback centre) on denial/unsupported/timeout, and a POST built only after location resolves"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteTransportHasNoLocationFallback"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoterLocationIsCachedPerSession"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteModuleLoadsBeforeItsConsumers"
        status: pass
    human_judgment: false
  - id: D2
    description: "Vote block DOM layer: one builder produces the .vote-controls/.vote-error markup, the reporter's own controls are removed (not disabled) and the emptied container hidden, aria-pressed is rewritten on both buttons on every update, and setBlockBusy is the sole owner of the disabled attribute"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteBlockHiddenGuard"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteButtonsMeetTouchTargetAndUseTokensOnly"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestOwnReportRuleRemovesControlsRatherThanDisablingThem"
        status: pass
    human_judgment: false
  - id: D3
    description: "The same block is mounted identically on the feed row (feed.js) and the map popup (map.js), driven by one builder, with a vote tap never activating the row it sits inside"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestBothSurfacesMountTheSameVoteBlock"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteClickDoesNotActivateItsRow"
        status: pass
    human_judgment: false
  - id: D4
    description: "End-to-end runtime UX: GPS-denial hard stop with the exact Copywriting Contract sentence and no network request sent, once-per-session prompting, the waiting/busy state visibly held until the server answers, identical behavior on both surfaces, vote changing, own-report rendering, and row-activation isolation on a narrow viewport"
    human_judgment: true
    rationale: "These are real-browser claims (an actual geolocation permission dialog, actual network timing, actual visual states) that static contract tests structurally cannot execute. Carried by Task 3's <human-check>, deferred to /gsd-verify-work 2 per workflow.human_verify_mode: end-of-phase — not a checkpoint:human-verify task, so this plan stays autonomous."

# Metrics
duration: 15min
completed: 2026-09-15
status: complete
---

# Phase 2 Plan 05: Confirm/Dispute Vote Controls Summary

**window.PinalertVotes ships a session-cached, fallback-free geolocation-backed vote transport plus one DOM builder mounted identically on the feed row and the map popup — the phase's first user-clickable increment of the confirm/dispute trust mechanic.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-15T15:52:00Z (approx.)
- **Completed:** 2026-09-15T16:02:35Z
- **Tasks:** 3
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments

- A visitor can tap **Confirm** or **Dispute** on a feed row and on a map pin popup, both mounted by one `PinalertVotes.createVoteBlock`/`updateVoteBlock` pair, with identical markup and classes on both surfaces (D-01, TRUST-01).
- The voter's location is read from a `sessionStorage` cache before any prompt is raised, and the native prompt fires at most once per browser session (D-17) — proven structurally by `TestVoterLocationIsCachedPerSession`'s ordering assertion.
- A denied or unsupported prompt is a hard stop: `getVoterLocation` **rejects** rather than substituting a position, `castVote`'s single `fetch(` call is structurally downstream of that rejection, and the Copywriting Contract's exact GPS-denial sentence renders in the row's own `.vote-error` (D-18, T-02-01) — the plan's single named copy-paste risk, closed by `TestVoteTransportHasNoLocationFallback` rather than a reviewer.
- No optimistic update anywhere: `setBlockBusy` disables every button in the block on tap and stays busy across the mandatory `Pinalert.fetchReports()` refetch; the active state is applied only from the server's own `your_vote` on the next render (D-04, D-02).
- `applyOwnReportRule` removes (never disables) the reporter's own Confirm/Dispute buttons and hides the emptied container — a UI courtesy layered on 02-03b's enforced 403, never a replacement for it (D-03).
- Every JS-toggled `display` rule in `trust.css` carries the `:not([hidden])` specificity guard — this repo's third encounter with the trap that shipped the Phase 1 modal-backdrop UAT blocker, caught here by `TestVoteBlockHiddenGuard` before it could ship.
- `trust.css` introduces no new custom property and no raw color literal; `.vote-btn` meets the 44×44 touch-target floor through `var(--touch-target-min)`.

## Task Commits

Each task followed a RED → GREEN split (Tasks 1–2 are `tdd="true"`; Task 3 follows the same discipline as a matter of consistency even though it isn't gated):

1. **Task 1: The vote transport** —
   `03b20a5` test(02-05): add failing tests for vote transport module,
   `81179bb` feat(02-05): implement vote transport module
2. **Task 2: One vote block, built once and updated on every poll** —
   `c807ab5` test(02-05): add failing tests for the vote block DOM layer,
   `41d2573` feat(02-05): implement the vote block DOM layer
3. **Task 3: Mount the same block on both surfaces** —
   `5206db1` test(02-05): add failing tests for mounting the vote block on both surfaces,
   `6e5a62e` feat(02-05): mount the vote block on the feed row and the map popup

TDD gate compliance verified: every task's `test(...)` commit precedes its `feat(...)` commit, and each RED-phase run was confirmed failing before implementation (see git log; no `git log --grep` gaps).

**Plan metadata:** this commit (docs(02-05): complete plan)

## Files Created/Modified

- `web/static/js/votes.js` — the whole client half of the vote mechanic: `getVoterLocation`, `castVote`, `createVoteBlock`, `applyOwnReportRule`, `updateVoteBlock`, `setBlockBusy`, `showVoteError`, `clearVoteError`, `onControlsClick`, exported as `window.PinalertVotes`
- `web/static/css/trust.css` — token-only styling for `.vote-controls`/`.vote-btn`/`.vote-error` and their active/disabled states
- `web/votes_contract_test.go` — 8 new contract tests (`TestVoteTransportHasNoLocationFallback`, `TestVoterLocationIsCachedPerSession`, `TestVoteModuleLoadsBeforeItsConsumers`, `TestVoteBlockHiddenGuard`, `TestVoteButtonsMeetTouchTargetAndUseTokensOnly`, `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`, `TestBothSurfacesMountTheSameVoteBlock`, `TestVoteClickDoesNotActivateItsRow`) plus the shared `jsFunctionBody` helper
- `web/templates/index.html.tmpl` — one deferred `<script>` for `votes.js` between `app.js` and `map.js`; one `<link>` for `trust.css` between `feed.css` and `auth.css`
- `web/template_contract_test.go` — `appModules` gained `"/static/js/votes.js"`
- `web/static/js/feed.js` — `createRow` mounts and returns the block under the handle's `votes` key; `updateRow` calls `updateVoteBlock`; header SCOPE comment narrowed
- `web/static/js/map.js` — `buildPopupContent` mounts and updates the block after the description; header SECURITY comment extended by one sentence

## Decisions Made

See `key-decisions` in the frontmatter above for the full rationale on each. In short: the export-object literal shape, the `appModules` insertion point, the two fallback-fingerprint literals chosen for the headline no-fallback gate, and the comment-placement convention required to keep `jsFunctionBody`'s window-bounding from swallowing a neighboring function's mandated prose — all four were resolved in the direction the plan's own machine-checked acceptance criteria required, and are documented here rather than left implicit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Resolved a self-contradiction in Task 1's `appModules` instruction**
- **Found during:** Task 1
- **Issue:** The plan's `<action>` said to place `"/static/js/votes.js"` "first in that slice, matching the order it will appear in the template" — but the template's actual order is `app.js` then `votes.js`, so "first" and "matching template order" cannot both be true.
- **Fix:** Placed `votes.js` immediately after `app.js` in `appModules`, matching the template's real load order. `TestVendorMapScriptsLoadInDependencyOrder`'s loop only checks each module's position against the vendor bridge, never relative order among app modules, so this has no test-behavior consequence.
- **Files modified:** `web/template_contract_test.go`
- **Verification:** `TestVendorMapScriptsLoadInDependencyOrder` passes; full `-short` suite green.
- **Committed in:** `03b20a5`

**2. [Rule 1 - Bug] `window.PinalertVotes = {}` assignment shape adjusted to satisfy the plan's own literal grep gate**
- **Found during:** Task 1
- **Issue:** The plan's prose said to write the module "in the same style as `map.js`'s `window.PinalertMap`" (the `window.X = (function(){ ... return {...}; }());` pattern), but the plan's own acceptance criterion greps for the literal substring `window.PinalertVotes = {` appearing exactly once — a pattern that `window.PinalertVotes = (function () {` does not produce (there's an intervening `(function () {`).
- **Fix:** Wrote the module as a bare `(function () { ... }());` that assigns `window.PinalertVotes = { ... };` as its own last statement, rather than wrapping the whole IIFE in `window.PinalertVotes = (...)`. This is the only shape that satisfies the literal acceptance gate while still exporting the same public surface.
- **Files modified:** `web/static/js/votes.js`
- **Verification:** `grep -c 'window.PinalertVotes = {' web/static/js/votes.js` is `1`; all contract tests pass.
- **Committed in:** `81179bb`

---

**Total deviations:** 2 auto-fixed (both Rule 1 — resolving internal contradictions in the plan's own instructions in favor of its machine-checked acceptance criteria). **Impact on plan:** Both fixes were necessary for the plan's own `<verify>` block to pass as written; no scope creep, no behavior change beyond what the plan specified.

## Issues Encountered

None beyond the two documented deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

**For 02-06 (visibility tags, "Show disputed" toggle):**
- `.visibility-tag` insertion point on both surfaces: between the meta/description line and `block.controls` — i.e., insert it before `block.controls` is appended, on both the feed row and the map popup. This plan appended `block.controls` then `block.error` immediately after the meta/description line specifically so 02-06 has one documented insertion point rather than a restructuring.
- `trust.css` loads after `feed.css` and before `auth.css`, specifically so 02-06's `.vis-provisional`/`.vis-hidden` rules can be appended to `trust.css` (or a sibling file loaded in the same slot) and win over the existing `.age-*` ramp with no specificity trick.
- Any `display` rule 02-06 adds for a JS-toggled element **must** carry the same `:not([hidden])` guard `TestVoteBlockHiddenGuard` enforces here — this is now the third time this repo has needed that guard.

**For 02-07 (Mark Resolved / Reopen, Activity page):**
- `VOTE_ACTIONS` already lists `'resolve'` and `'reopen'`, and `castVote(reportId, action)` is transport-only and reusable unchanged for both.
- `.vote-controls`' single container-level `click`/`keydown` listener means 02-07's third button needs **no new event wiring** — it is picked up automatically by the existing delegation in `createVoteBlock`.
- `applyOwnReportRule`'s hide-when-empty rule (`block.controls.hidden = !hasAnyButton`) clears itself automatically once 02-07 appends a resolve button to a reporter's own row — no special case needed there.
- **`profile.html.tmpl` does not yet link `trust.css`.** 02-07 must add that `<link>` itself, or its Reopen button on the Activity page ships unstyled.
- The reporter's own-report empty-state hide behavior (`applyOwnReportRule` hiding `.vote-controls` when it has no children) was **not exercised in a live browser** in this plan — it is proven only by the static contract test `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`, which bounds the function body and asserts the right calls exist. The actual visual baseline for "what a reporter's own row looks like today" is established by Task 3's deferred `<human-check>` (item 6, "Own report"), run at `/gsd-verify-work 2` — 02-07 should read that UAT result rather than assume the row's current appearance.
- The one accepted limitation documented in `onControlsClick`'s own comment (a background poll's popup rebuild racing an in-flight map-popup vote can leave a completion handler holding a detached node) is unchanged by 02-07's future resolve/reopen buttons — the same append-only-log absorption applies to a duplicate resolve/reopen vote exactly as it does to a duplicate confirm/dispute vote.

**Blockers:** None. All three tasks' automated verification passes (`go test ./web/ -count=1`, `go test ./... -short`, and `go test ./... -p 1` with `DATABASE_URL` set). The one remaining verification surface — Task 3's 7-step browser walkthrough (GPS denial, once-per-session prompting, the waiting state, both surfaces, vote changing, own-report rendering, row activation) — is deliberately deferred to `/gsd-verify-work 2` per `workflow.human_verify_mode: end-of-phase`, and is recorded above as coverage item D4 with `human_judgment: true`.

## Self-Check: PASSED

All three created files (`web/static/js/votes.js`, `web/static/css/trust.css`,
`web/votes_contract_test.go`) found on disk. All six per-task commit hashes
(`03b20a5`, `81179bb`, `c807ab5`, `41d2573`, `5206db1`, `6e5a62e`) found in
`git log --oneline --all`. All `<acceptance_criteria>` re-verified via grep
per task (see task commits). Plan-level `<verification>` steps 1–6 re-run:
`gofmt -l web/` empty, `go build ./...` and `go vet ./...` exit 0, `go test
./web/ -count=1 -v` all pass (8 new tests + full Phase 1/1.1 suite), `go test
./... -short` exits 0, `go test ./... -p 1` exits 0 with `DATABASE_URL` set,
`git status --short` shows no file modified outside `web/`, `app.js` diff is
empty.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Plan: 05*
*Completed: 2026-09-15*
