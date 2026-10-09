---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 26
subsystem: ui
tags: [map-js, feed-js, focus, leaflet-popup, contract-tests, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the list opener seam (A74) and the poll focus recovery (A67), plus the data-popup-open mark read in 03-25
provides:
  - flyTo opens the popup only when it is closed, so a row activation leaves focus on the popup card
  - focusIsLost treats focus inside a popup that map.js marked closed as lost, so a poll removal recovers focus
affects: [03-UAT live checks T9.13 and T9.14]
tech-stack:
  added: []
  patterns:
    - "Read the synchronous data-popup-open mark instead of timing Leaflet's 200 ms fade"
    - "Make a seam call idempotent when the first caller already did the work"
key-files:
  created: []
  modified:
    - web/static/js/map.js
    - web/static/js/feed.js
    - web/popup_contract_test.go
key-decisions:
  - "A74: keep the activateRow seam and the onSelect subscriber, guard the open call in flyTo instead"
  - "A67, A52: decide lost focus from the data-popup-open mark, with no timer, animation event or style read"
requirements-completed: []
status: complete
---

# Phase 3 Plan 26: Row activation opens the popup once and poll removal frees a fading popup Summary

Closes the last two defects of the scripted browser run for test 9: a row activation now leaves focus on the popup card, and a poll that removes the focused popup's report no longer strands focus on the page body.

## What changed

- **Task 1 (a6d1b9a).** `flyTo` in map.js wraps `marker.openPopup()` in `if (!marker.isPopupOpen()) {`. A row activation calls `Pinalert.select(id, 'list')`, whose `onSelect` subscriber already calls `flyTo(id)` and opens the popup, and then `activateRow` calls `PinalertMap.flyTo(id, { opener: 'list' })`. The second open call on an open popup made Leaflet remove and append the card node again, which dropped the focus `popupopen` had given it and fired no second `popupopen`. The opener claim after the guard is unchanged, so Escape still returns to the row. The `activateRow` seam, the subscriber, `onPopupOpen` and the pin keydown open call are untouched.
- **Task 2 (9531778).** feed.js gains `focusIsLost(active)`: true for null or the body, true for a node inside a `.leaflet-popup` whose `data-popup-open` is not `'true'`, false otherwise. `focusAfterRemoval` now returns early on `!focusIsLost(active)`, so the existing order (neighbouring row, map, view toggle, empty state) runs when focus sits in a fading popup. `clearRows` shares `focusAfterRemoval`, so it is covered too. map.js is unchanged for this defect: `onPopupClose` already clears the mark synchronously.

## Tests

- `TestListRowActivationOpensThePopupOnce` is new. The `flyTo` order assertion in `TestPopupDialogFocusAndOpener` was amended to expect the guard before the open call. Both failed on the old map.js and pass now.
- `TestFocusAfterRemovalIgnoresAFadingPopup` is new and failed on the old feed.js. It pins the decision, not the fade timing.
- `TestRenderRowsRemovesBeforePlacing`, `TestPinOpensOnSpace`, `TestPopupReducedMotionPansWithoutAnimation`, `TestTrustBlockMountedOnBothSurfaces`, `TestFadeUsesCombinedStage` and `TestIconGlyphsAreClassDriven` pass unedited.
- `go test ./web/ -count=1 -v`: exit 0, 0 FAIL, 0 SKIP. `go vet ./web/` is clean. `gofmt -l web` lists only `web/js_contract_test.go`, which this plan did not touch.

## Notes for the orchestrator

- A row activated while its popup is already open keeps focus where it is. The scripted check T9.14 verified live only the first activation, not a repeat on an already open popup.
- `votes.js` `focusAfterRetract` and the map.js `openedWhileHidden` flag are unchanged and out of this plan's reach.
- T9.16 (WebKit close button click returns focus to the map container) is the recorded accepted browser difference and was left alone.

## Live proof still owed (orchestrator, after merge)

- T9.14: at 1000px, Enter or a click on a feed row opens the popup once, focus ends on `.popup-card`, one Escape closes it with focus back on the row.
- T9.13: a poll that removes the focused popup's report moves focus to the neighbouring row, else the map, in real time. If either still fails, report the observed `document.activeElement` and do not widen the fix.

## Deviations from Plan

None. The plan was executed as written. No database, server, browser or port was used.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or storage. The two changes only skip a redundant popup open and choose a focus target.

## Self-Check: PASSED

- web/static/js/map.js, web/static/js/feed.js and web/popup_contract_test.go carry the changes above.
- Commits a6d1b9a and 9531778 exist on the worktree branch, each listing only its own paths.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched.
