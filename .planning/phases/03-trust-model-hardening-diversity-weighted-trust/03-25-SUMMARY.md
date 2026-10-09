---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 25
subsystem: ui
tags: [votes-js, focus, leaflet-popup, too-far-hint, contract-tests, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: votes.js focus helpers (03-09) and the too far hint (03-18)
provides:
  - findLiveControls ignores a map popup that map.js has marked closed
  - the too far refusal line clears together with the too far hint
  - V-42 step 8(f) and UAT test 4 describe the silent refresh on a too far tap
affects: [03-UAT live checks T5 P4 and T4 S1.11b]
tech-stack:
  added: []
  patterns:
    - "Read the synchronous data-popup-open mark instead of timing Leaflet's 200 ms fade"
    - "Narrow flag (refusedTooFar, set from err.field) so only one message on a shared error line is cleared"
key-files:
  created: []
  modified:
    - web/static/js/votes.js
    - web/vote_controls_contract_test.go
    - web/vote_hint_contract_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-20-SUMMARY.md
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UAT.md
key-decisions:
  - "A52: a popup origin is live only while data-popup-open is true; row origins are unchanged"
  - "A8, D-22: no UI-SPEC row gives the refusal line a lifetime, so the fallback applies and it clears with the hint, only when it holds the server's too far refusal"
requirements-completed: []
status: complete
---

# Phase 3 Plan 25: Popup resolve focus and stale refusal line Summary

Closes the last two defects of the scripted browser run: Mark resolved from the map popup now ends with focus on the map container at default motion, and the earlier too far refusal line goes away with the hint. One stale line of the V-42 script was corrected.

## What changed

- **Task 1 (53fb069).** Added `isInOpenPopup(controls)` and gated both branches of `findLiveControls` on it for popup origins. Leaflet keeps a closing popup node attached for 200 ms, so the old `isConnected` check returned the dying controls, `focusAfterVote` focused a dying button and returned, and the node's removal dropped focus on the body. The `data-popup-open` mark is cleared synchronously by map.js inside the render, before the refetch resolves, so it is correct at the moment the focus step runs. No timer, animation event or style read is involved. `focusAfterVote`, `focusAfterRetract` and `captureRemovalFocus` are untouched.
- **Task 2 (a001850).** Added `block.refusedTooFar`, set in the `onControlsClick` catch branch from `err.field === 'location'` and reset in `clearVoteError`. New `clearRefusalWithHint(block, hintWasShowing)` calls `clearVoteError` only when the hint was showing before the render, is hidden after it, and the line holds the too far refusal. It runs after `applyDistanceHint` in `updateVoteBlock` and in `refreshHintThenRetry`. GPS denied (no `field`), generic failure and the saved-but-not-refreshed lines are never cleared by it.
- **Task 3 (299551c).** Step 8(f) in 03-20-SUMMARY.md now says a tap on a marked button sends no vote but takes one fresh fix silently (WR-03) and recomputes the mark. Test 4 in 03-UAT.md says the same in `expected:` and points to the corrected script in `note:`. Only those two lines of 03-UAT.md changed.

## Tests

Both new tests failed before the change (RED confirmed) and pass after it:

- `TestPopupResolveFocusIgnoresAFadingPopup` (web/vote_controls_contract_test.go)
- `TestTooFarRefusalClearsWithTheHint` (web/vote_hint_contract_test.go)

Existing tests run unedited and passing: `TestFocusStepsAfterMarkResolved`, `TestEscapeCancelHookIsExported`, `TestResolveConfirmIsANamedGroupWithCancelFirst`, `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`, `TestVoteHint`, `TestVoteHintSentenceMatchesServer`, `TestVoteRadiusReadWithoutConfig`, `TestVoteErrorGetsNoChildNodes`, `TestVoteTransportHasNoLocationFallback`. `go build ./...` and `go vet ./...` are clean.

## Deviations from Plan

None in the code. One scope note: plan verification step 2 asks for the whole `./web/` package. The repo rules for this run forbid running more than the named tests because the package can need a database, so the orchestrator runs the full suite on a scratch database after the merge. No database was used, no server was started.

## Live proof still owed (orchestrator, after merge)

- T5 P4: after Yes on Mark resolved in the map popup at default motion, `document.activeElement` is `.leaflet-container`. If it still fails, report the observed `document.activeElement` and do not widen the fix.
- T4 S1.11b: after the hint clears at the 120 second bound, the earlier refusal line is gone, and a GPS denied or generic failure line stays.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or storage. `refusedTooFar` is a client side boolean set from `err.field`, no message text is read, and every error write still goes through `Pinalert.setText`.

## Self-Check: PASSED

- web/static/js/votes.js, web/vote_controls_contract_test.go, web/vote_hint_contract_test.go, 03-20-SUMMARY.md and 03-UAT.md exist and carry the changes above.
- Commits 53fb069, a001850 and 299551c exist on the worktree branch.
- STATE.md, ROADMAP.md and REQUIREMENTS.md are untouched.
