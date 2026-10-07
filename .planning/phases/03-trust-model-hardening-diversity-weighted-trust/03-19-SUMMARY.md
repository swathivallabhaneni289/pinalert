---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 19
subsystem: ui
tags: [javascript, leaflet, accessibility, focus, keyboard, reduced-motion, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-13 trust.js with the one Escape listener, 03-15 PinalertVisibility.stateWord, 03-16 Pinalert.SEVERITY_WORDS, 03-17 popup card built once with onPopupOpen and relayoutOpenPopup"
provides:
  - "Pins named in words and opened with Enter or Space; the open popup is a named non modal dialog that takes focus"
  - "Escape resolved one level per press by surface ownership, with focus returned to the row or pin that opened the popup"
  - "A closed popup collapses its trust panel (PinalertTrust.collapsePanelsIn)"
  - "Focus never stranded on the page body by a poll that removes or reorders reports"
  - "Token derived autopan paddings, no animation under prefers-reduced-motion, focus safe popup re-measure"
affects: [03-20]
tech-stack:
  added: []
  patterns:
    - "data-popup-open DOM hook written by map.js and read by the one trust.js Escape listener"
    - "Remove departed rows before placing, moveBefore where present, refocus a moved row"
    - "p19 static contract helpers (p19Code, p19NoSpace, p19Condition)"
key-files:
  created: []
  modified:
    - web/static/js/map.js
    - web/static/js/feed.js
    - web/static/js/trust.js
    - web/popup_contract_test.go
    - web/trust_contract_test.go
key-decisions:
  - "data-popup-open is the DOM hook shared by map.js (writes it on the popup container and the pin) and trust.js (reads it), so trust.js needs no reference to the map module."
  - "Space is one delegated keydown handler on the map container (close button and pin), because Enter is already Leaflet's own and 03-17 added no key handler."
  - "Focus moves inside the Leaflet container use preventScroll: true, so the overflow hidden map container is not scrolled by the browser."
  - "popupHoldsFocus confirms the live document.activeElement and its client rects, so a focus already lost before the refetch (the hidden Mark resolved Yes) is not claimed and 03-09's focusAfterRetract keeps its map container path."
  - "panPopupIntoView clamps the offset per axis with the top left edge winning, so two calls in a row cannot pan back and forth."
  - "PinalertTrust.collapsePanelsIn is exported (a decision beyond the outline); it finds the panel beside the toggle because a closing popup may already be detached."
  - "Padding updates are applied to every bound popup on resize, so a popup first opened after a breakpoint change pans with current values."
requirements-completed: [TRUST-05, TRUST-07]
status: complete
duration: 50min
completed: 2026-10-08
---

# Phase 3 Plan 19: Popup keyboard, focus and motion Summary

A keyboard or screen reader user can open a pin with Enter or Space, land in a named popup dialog, close the trust panel and then the popup with one Escape per level by surface ownership, return to the row or pin that opened it, and keep focus through polls that remove or reorder reports, with autopan that clears the fixed controls and never animates under reduced motion.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Pins open on Enter and Space, popup dialog, Escape ownership, opener return | 2cd7cf8 |
| 2 | Remove departed rows before placing, focus never falls to the body | ca1d92e |
| 3 | Token derived autopan, reduced motion pan, focus safe re-measure | 54a9956 |

## Precondition gate (Task 1, before RED)

All five clauses held before any edit: `grep -cF 'src="/static/js/trust.js?v={{.AssetVersion}}" defer'` printed 1 in `web/templates/index.html.tmpl` and 1 in `web/templates/profile.html.tmpl`; `TestTrustModuleLoadsBeforeItsConsumers` printed `--- PASS`; `stateWord: stateWord` is present once in visibility.js and `SEVERITY_WORDS: SEVERITY_WORDS` once in app.js. This plan edits no template, visibility.js or app.js.

## RED then GREEN

- Task 1 RED: `TestPopupDialogFocusAndOpener` failed with `onPopupOpen: missing "var popup = evt.popup;"` (and ten more needles) and `could not locate "function onPopupClose(evt)"`; `TestPinOpensOnSpace` failed with `could not locate "function onMapKeydown(evt)"`; the extended `TestTrustDisclosure` failed with `closeOneLevel popup branch: "openPopupEl()" is missing` and `could not locate "function closePopupLevel(popupEl, mayClose)"`. GREEN: all three pass.
- Task 2 RED: `TestRenderRowsRemovesBeforePlacing` failed with `renderRows: needs .remove(), moveBefore( and insertBefore( (found 719, -1, 488)` and `missing "takeMapPopupFocus()"`, `missing "focusAfterRemoval("`. GREEN: passes, and `TestPopupDialogFocusAndOpener` still passes.
- Task 3 RED: `TestPopupResizeUpdatesOnlyWithoutFocus` failed with `could not locate "function onMapResize()"`; `TestPopupPaddingTokensAgree` failed with `map.js does not quote the CSS media condition "(max-width: 899.98px)"` and `could not locate "function refreshPanPadding()"`; `TestPopupReducedMotionPansWithoutAnimation` failed with `missing "fadeAnimation: !reducedMotion"` and `could not locate "function panPopupIntoView(popup)"`. GREEN: all three pass.
- Final `go test ./web/ -count=1` printed `ok  pinalert/web`. A `-v` run printed 0 `--- SKIP` and 0 `--- FAIL` lines (DATABASE_URL was unset; no web test uses a database). The seven named tests (`TestPopupResizeUpdatesOnlyWithoutFocus`, `TestPopupPaddingTokensAgree`, `TestPopupReducedMotionPansWithoutAnimation`, `TestPopupDialogFocusAndOpener`, `TestPinOpensOnSpace`, `TestRenderRowsRemovesBeforePlacing`, `TestTrustDisclosure`) gave a `--- PASS` count of 7.
- Other gates: `go vet ./web/` clean; `gofmt -l web/popup_contract_test.go web/trust_contract_test.go` prints nothing; `node --check` parses map.js, trust.js and feed.js. Acceptance greps: `focusOpenPopup: focusOpenPopup` 1 in map.js, `takeRemovedPopupFocus: takeRemovedPopupFocus` 1, `collapsePanelsIn: collapsePanelsIn` 1 in trust.js, `PinalertMap.flyTo(id, { opener: 'list' })` 1 in feed.js, `function focusAfterRemoval(candidates)` 1, `function panPopupIntoView(popup)` 1, `var NARROW_QUERY = '(max-width: 899.98px)'` 1. The `·` escape in map.js is still an escape, and the added lines of every diff held 0 occurrences of U+2014 or U+2013.

## Two 03-17 assertions relaxed (intended edits)

- `TestPopupCloseButtonComesFirst`: removed the `.focus(` and `setAttribute('role'` absence checks on `onPopupOpen(evt)` (A23 adds them). The `update()` absence check, the close button move anchors and the missing container guard stay.
- `TestPopupCardBuiltOnceUpdatedInPlace`: removed the `autoPan` absence check on the bindPopup window (A60). The `className` read, `card.el`, `minWidth: 340`, `maxWidth: 340`, `keepInView: true` and the absence of `closeOnClick` and `maxHeight` stay.

`onPopupOpen(evt)` and `relayoutOpenPopup(marker, card)` were extended in place; there is still one `map.on('popupopen'` registration and one `function onPopupOpen(`. The observer's frame callback still calls only `setLatLng(` and then `panPopupIntoView(`, never the popup content refresh.

## Decisions beyond the outline

The `data-popup-open` DOM hook shared by map.js and trust.js; the delegated Space handler on the map container; `preventScroll` on focus moves inside the Leaflet container; the live `activeElement` confirmation in `popupHoldsFocus`; the edge clamped pan offset; the `collapsePanelsIn` export; padding updates applied to every bound popup on resize.

## Deviations from Plan

- **Command substitutions (rule 11).** The sandbox refused a compound git plus echo one-liner at the start and a heredoc driven script edit of trust.js. I ran each check as a separate plain command and made the trust.js edit with the Edit tool. The plan's long verify one-liners were split into separate commands that check the same things: the two template greps, the `TestTrustModuleLoadsBeforeItsConsumers` `-v` PASS line, the two export greps, `go test ./web/ -count=1`, the `-v` PASS count over each named-test regex, `node --check` per file, `go vet ./web/`, and `gofmt -l`. `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset`, so `go test` ran without an env prefix.
- **Tests committed with their task.** RED runs were executed and recorded above, but each task's tests and implementation went into one commit, as the plan defines one commit per task.
- **Comment above the init map options.** The existing comment block above the map construction is untouched; `refreshPanPadding();` was placed above it so the comment still sits directly over the map options it describes.
- No other deviations. Only the five files in files_modified changed. STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. No database, server or port was used and p3_0319 was never created.

## Known Stubs

None.

## Threat Flags

None new. T-03-06 and T-03-04 are mitigated: the pin name is built from the fixed category, severity and state tables plus `relativeTime` and written with `setAttribute('aria-label', ...)`; `data-popup-open` carries only the literal `true`; the dialog name points at the existing card title id; the opener row is found by comparing `dataset.reportId`, never by a selector built from report data; `TestTrustModuleHasNoThresholdsOrSinks` still passes on trust.js.

## Human checks (deferred to 03-20 V-42, not run here)

No server was started; these belong to the owner script at PORT=8792 on 127.0.0.1 with a disposable database p3_uat:

- Task 1: every A70 Escape case (focus on Confirm with the popup panel open: first Escape closes the panel and focuses the toggle, second closes the popup; focus on `.popup-card`, on the close button and on the marker with the panel open; a row panel with focus in the row; a popup panel and a row panel open together with focus in the popup closes only the popup panel); A74 (at 900px and above a row opened by Enter and closed by one Escape leaves focus on that row; a popup opened from its pin returns focus to the marker); A49 Space on a pin and on the close button; A45 pin names read by a screen reader; A51 a list row tapped at phone width lands focus on the card.
- Task 2: with focus on a later row's trust toggle, a poll that drops an earlier report and a separate poll that reorders rows by a severity change each leave `document.activeElement` on the same toggle (A71); a poll removal while focus is inside the popup leaves focus off the body (A67).
- Task 3: at 320px and 360px in list view a row whose pin is near the left or right edge opens a popup whose tip is centred on the pin after switching to the map (A59); the close button and the trust toggle are never covered by the account trigger, FAB, theme toggle or view toggle after autopan at 320x568, while at 320x256 the card scrolls inside itself and close and Escape still work (A60, A65); under reduced motion at 320px and 360px a pin near the top edge with the confirmation or the panel open shows no animated pan, and a rotation across 900px with a popup open re-measures it (A68).

## Self-Check: PASSED

Modified files exist: web/static/js/map.js, web/static/js/feed.js, web/static/js/trust.js, web/popup_contract_test.go, web/trust_contract_test.go. Commits 2cd7cf8, ca1d92e and 54a9956 exist on the worktree branch.
