---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 09
subsystem: ui
tags: [javascript, vote-controls, accessibility, focus-management, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-04 getVoterLocation and castVote transport in votes.js"
provides:
  - "createVoteButton and setVoteButtonLabel: icon span plus label span on every vote button"
  - "Named resolve confirmation group (Cancel first, focus on Cancel) and the A52 focus steps"
  - "cancelResolveConfirm hook for plan 03-13's Escape listener"
  - "createVoteBlock returns { controls, note, error }; feed row and popup mount the hidden note"
  - "createReopenControl and setReopenLabel; Activity Reopen is the shared control"
affects: [03-11, 03-13, 03-15, 03-18, 03-19]
tech-stack:
  added: []
  patterns:
    - "p09 static contract-test helpers (p09ReadJS, p09FuncBody, p09Ordered) with a tight function-body scan"
key-files:
  created:
    - web/vote_controls_contract_test.go
  modified:
    - web/static/js/votes.js
    - web/static/js/feed.js
    - web/static/js/map.js
    - web/static/js/activity.js
key-decisions:
  - "Removal focus is confined to Mark resolved and reads 'left the feed' from the live DOM after the refetch, never from the response."
  - "The icon span carries no icon-glyph base class; plan 03-11's mask rule repeats the mask base."
requirements-completed: [TRUST-05]
status: complete
duration: 35min
completed: 2026-10-08
---

# Phase 3 Plan 09: Vote controls DOM contract Summary

One vote control builder in votes.js now serves the feed row, the map popup and the Activity page, with icon and label spans on every vote button, a named Mark resolved group that focuses Cancel, focus that always lands on a real control, and a hidden note slot both surfaces mount.

## What was built

- Task 1 (b6a836a): the resolve confirmation is `role="group"` labelled by its question (`pinalert-resolve-q-N`, from the `voteBlockSeq` counter). DOM order is question, Cancel, Yes. Opening focuses Cancel. New helpers `isDisplayed`, `canTakeFocus`, `focusIsLost`, `focusResolveButton`, `captureVoteOrigin`, `findLiveControls`, `findFeedRow`, `captureRemovalFocus`, `focusAfterRetract`, `focusAfterVote`, plus the exported `cancelResolveConfirm(container)`. `onControlsClick` captures origin and removal, and calls the focus helpers on the Cancel, refetch and error paths.
- Task 2 (5ddbd3e): `createVoteButton` and `setVoteButtonLabel`. Confirm, Dispute and Mark resolved are built through the helper. `createVoteBlock` returns `{ controls, note, error }` and the note is `p.vote-note#pinalert-vote-note-N`, created hidden and empty. `feed.js` and `map.js` append controls, note, error.
- Task 3 (ba5bd06): `createReopenControl(reportId)` returns `{ controls, button, error }`. `activity.js` lost `buildReopenControl`, calls the shared builder and writes its three remaining labels through `PinalertVotes.setReopenLabel`.

## Exports and DOM classes other plans consume

- `window.PinalertVotes`: `cancelResolveConfirm` (returns false while the block is busy or when nothing is open), `createReopenControl`, `setReopenLabel` (bound to `setVoteButtonLabel`).
- DOM classes: `vote-btn__icon` (no `icon-glyph` base class) with modifiers `vote-btn__icon--check`, `--ban`, `--shield-check`, `--rotate-ccw`; `vote-btn__label`; `vote-note` with counter id `pinalert-vote-note-N` (the id plan 03-18 points at).
- votes.js reads the template ids `view-toggle`, `feed-empty` and `disputed-empty` for the last resort focus. Plan 03-19's A67 feed.js helper may consolidate that.

## Interim look until plan 03-11 lands

The icon spans are empty and take no room. The confirmation keeps the Phase 2 column layout, with Cancel now above Yes.

## TDD record

- Task 1 RED: `go test ./web/ -run 'TestResolveConfirmIsANamedGroupWithCancelFirst|TestFocusStepsAfterMarkResolved|TestEscapeCancelHookIsExported'` gave `--- FAIL` for all three. GREEN afterwards.
- Task 2 RED: `TestVoteButtonsCarryIconAndLabelSpans` and `TestVoteBlockReturnsNoteAndConsumersAppendInOrder` failed. `TestVoteErrorGetsNoChildNodes` and `TestRetapResendsTheSameVote` passed already, because they pin absences that held before the change (regression guards, not RED).
- Task 3 RED: `TestReopenControlIsSharedFromVotes` failed. GREEN afterwards.
- Final `go test ./web/ -count=1 -v` with DATABASE_URL unset: `ok  	pinalert/web`, no `--- FAIL`, no `--- SKIP`. `--- PASS:` lines seen for TestResolveConfirmIsANamedGroupWithCancelFirst, TestFocusStepsAfterMarkResolved, TestEscapeCancelHookIsExported, TestVoteButtonsCarryIconAndLabelSpans, TestVoteBlockReturnsNoteAndConsumersAppendInOrder, TestVoteErrorGetsNoChildNodes, TestRetapResendsTheSameVote, TestReopenControlIsSharedFromVotes, and the existing resolve, activity and Reopen tests.
- Static counts: `votes.js` 23 long-dash lines (cap 27), `feed.js` 31 (cap 31), `map.js` 12 (cap 12), `activity.js` 8 (cap 9), new test file 0. `node --check` passes on all four JS files. `gofmt -l web/vote_controls_contract_test.go` is empty and `go vet ./web/` is clean.

## Deviations from Plan

### Command substitutions
- The sandbox refused several long compound verify one-liners and heredoc appends. I split each verify block into separate `grep -cF`, `node --check`, `gofmt`, `go vet` and `go test -v | grep` commands that check the same things, and appended Task 2 and Task 3 tests with the Edit tool instead of a shell heredoc. Commits were run as separate `git add` and `git commit` calls.

### Auto-fixed Issues
None. The plan executed as written. The note about `vote-btn__icon` comments, forbidden words and span placement was followed.

## Known Stubs
None. The note node is intentionally empty and hidden; plan 03-18 fills it.

## Threat Flags
None. No new network surface; ids are compared as dataset strings and never built into selectors or markup (T-03-06), and the note carries no text so nothing in the browser affects what the server counts (T-03-09).

## Human check (deferred)
Task 1's keyboard focus check runs in plan 03-20's V-42 session. No server or database was used by this plan.

## Self-Check: PASSED
Created file `web/vote_controls_contract_test.go` exists. Commits b6a836a, 5ddbd3e and ba5bd06 exist on the worktree branch.
