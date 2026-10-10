---
quick_id: 261010-psc
type: quick
description: Stop the page scrolling when a pin is clicked on a wide screen
created: 2026-10-10
autonomous: true
files_modified:
  - web/static/js/feed.js
  - web/popup_contract_test.go
  - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
must_haves:
  truths:
    - "Clicking a pin on a wide screen opens its popup and highlights the matching row, and the page does not scroll"
    - "A selection that starts from a list row behaves exactly as it did"
    - "No CSS, markup, map.js, focus, keyboard, vote or trust code changes"
  artifacts:
    - path: "web/static/js/feed.js"
    - path: "web/popup_contract_test.go"
    - path: ".planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md"
---

# Quick task 261010-psc: stop the page scrolling when a pin is clicked

## Objective

Make a pin click on a wide screen leave the page where it is. The owner said it plainly during the final UAT:
when I click a pin on the map the page should just stay on the same screen, and if I want the list I will go
there myself.

## Why

At 900px and wider the map and the list sit side by side, and neither pane is a scroll container. The list
grows the document, `#map` is 100dvh tall at the top, and so the DOCUMENT scrolls. `feed.js` handled a
map-originated selection with `applySelectionHighlight(id, true)`, which called `scrollIntoView` on the
matching row, so the whole page scrolled down and the map left the viewport. Below 900px the list is hidden
in map view, so the call did nothing there. The behaviour came from Phase 1 ("map and list are always in
sync", `01-UI-SPEC.md` wide screens, `01-06-SUMMARY.md`).

## Tasks

1. Red first: a contract test `TestMapSelectionNeverScrollsThePage` in `web/popup_contract_test.go`, next to
   `TestListRowActivationOpensThePopupOnce`. It fails while `feed.js` holds the scroll call.
   - files: `web/popup_contract_test.go`
   - verify: the test fails before the change and names every reason.
2. Fix `feed.js`: remove the `scroll` parameter and the `scrollIntoView` call from `applySelectionHighlight`,
   keep the highlight and the `source === 'map'` guard, update the two doc comments, and update the call in
   `activateRow`.
   - files: `web/static/js/feed.js`
   - verify: the contract test passes, the rest of the suite is unchanged.
3. Scripted browser check `T9.18` in the durable harness (`~/code/pinalert-browser-checks`, outside the repo),
   in Chromium and WebKit: 1200x700, 7 reports so the list is taller than the window, a real mouse click on
   the pin of a report whose row starts below the fold. It asserts the popup is open, `window.scrollY` is
   unchanged and that row has `report-row--selected`. A precondition assertion (document taller than the
   window) stops it passing vacuously. Run it on the unfixed server first, then on the fix.
4. Docs: amendment A87 in `03-UI-SPEC.md` (the scroll clause of Phase 1 is superseded for map originated
   selections, the layout is unchanged), and this quick task folder.

## Non goals

- No CSS or layout change: no sticky map pane, no new scroll container.
- No change to focus handling, keyboard behaviour, the A74 flyTo seam, popup autopan, the vote code or the
  trust code. `map.js` is not touched.

## How it runs

- Own git worktree, own scratch database, disposable database for the harness (name ends in `_auto`), port 8801.
- Path limited commits with the Co-Authored-By trailer. No `gsd-tools commit`, no push.
- `STATE.md`, `ROADMAP.md`, `REQUIREMENTS.md`, `03-UAT.md`, `03-VERIFICATION.md` and other phases' files are
  not edited.
