---
phase: 08-report-page-and-comments
plan: 07
subsystem: web
tags: [report-page, laptop-panel, popup, leaflet, vanilla-js, css]
requires:
  - phase: 08-05
    provides: PinalertReportPage.create(report) with setHosted, open, close
  - phase: 08-06
    provides: window.PinalertShare.open, a direct child of .app-shell at z-index 1100
provides:
  - the laptop report panel section of map.js (D-18 default A)
  - the Open report row of the laptop bubble, with the server's comment_count only
  - report-panel.css, the 520px panel over the Nearby list column at 900px and up
affects: [08-08, 08-16, 08-17, 08-20]
tech-stack:
  added: []
  patterns: [host switch flag so a close and reopen is not a close, settle a close a turn later like the plan 39 address code, a MutationObserver clears Leaflet's inline left and transform instead of the important flag]
key-files:
  created:
    - web/static/css/report-panel.css
    - web/popup_footer_test.mjs
    - web/report_panel_contract_test.go
  modified:
    - web/static/js/map.js
    - web/static/css/trust.css
    - web/templates/index.html.tmpl
    - web/popup_contract_test.go
    - web/layout_rules_contract_test.go
key-decisions:
  - "No important flag: the panel popup states top and height (Leaflet's inline bottom is then ignored), the content states min-width and max-width 100% (beats the inline width), and map.js clears the inline left and transform as Leaflet writes them (holdPanelPosition)."
  - "The grid column of the open panel is 520px (the list is display none), so the map takes the rest and no pin lies under the panel."
  - "The panel uses the phone order (arrangeCard with detailInPane()), so Flag and Mute are body children and report-panel.css mirrors the phone's two calm rows."
  - "Exit is settled a turn later (panelNoteClose and panelNoteOpen), the same rule the plan 39 address code uses, so a pin tap is a switch and keeps the panel."
requirements-completed: []
metrics:
  tasks: 2
  commits: 3
  files: 8
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 07: The laptop report panel and the Open report row Summary

On a laptop the bubble keeps its look and gains one Open report row (real comment count only); pressing it moves the same card into a 520px panel that replaces the Nearby list, with Back to the list, pin switching, and the address untouched.

## What was built

### The panel section of map.js

Between `// ---- The laptop report panel (Phase 8 plan 07)` and `// ---- End of the laptop report panel (Phase 8 plan 07)`:

- State: `panelOpen`, `hostSwitching`, `panelExitTimer`, `panelOpener`, `panelPositionWatch`.
- `detailInPane()` (phoneDetail() or panelOpen), `enterPanel(id)`, `exitPanel()`, `replaceAllPlacements()`.
- `openReportWords(count)`: '' unless count is a non negative whole number; "No comments", "1 comment", "{N} comments". `openReportName(count)`: "Open report" or "Open report, " plus the words.
- `buildOpenReportRow(reportId)` and `writeOpenReportRow(card, report)`: the row is `button.popup-card__open-report` with `.popup-card__open-icon`, `.popup-card__open-label`, `.popup-card__open-count`, `.popup-card__open-chevron`. It is built only on a card that has a report page (so not on the Demo), inserted before the page node (the page stays the last child), hidden whenever the card is in the detail pane, and rewritten by updatePopupCard from `report.comment_count` only.
- `panelNoteClose(opener, popupEl)` and `panelNoteOpen(popupEl)`: the same turn rule as the address code. `holdPanelPosition` and `releasePanelPosition` keep Leaflet's inline left and transform off the open panel.

Wiring outside the section: `applyPopupPlacement` uses `detailInPane()` and hides the row in the pane; `onPopupOpen` names the anchor "Back to the list" in the panel, skips `noteReportOpened` during the host switch and calls `panelNoteOpen` and `holdPanelPosition`; `onPopupClose` clears `hadFocus`, the report link end and `noteReportClosed` during the host switch and calls `releasePanelPosition` and `panelNoteClose`; `keepCardClearOfTools` and `panPopupIntoView` leave the panel alone.

A Nearby row tap is untouched (default A, review W10): neither the onSelect callback nor flyTo mention the panel; a node test pins that, and that `enterPanel(` has exactly its definition and the row's click.

### CSS

- `trust.css`: the row rules (UI-SPEC Surface 8): full width under a 1px border hairline, `min-height: var(--touch-target-min)`, hover and focus fill `--color-surface` with an inset ring, 20px masks (message-square, chevron-right), count Label 14/400 with tabular numbers, every display rule `:not([hidden])`, a forced colours block.
- `report-panel.css` (linked after report-page.css): one `@media (min-width: 900px)` block, everything under `html[data-report-panel]`: shell columns `520px minmax(0, 1fr)`, list hidden, popup fixed under the top bar at 520px with a right hairline, the wrapper as the only scroll box (padding 16px 24px 0, below the 44px Back row), the close anchor drawn as arrow-left plus the word Back, Flag and Mute rows for the phone order.

## Verification

- `node --test web/*.mjs`: 1378 tests, 1378 pass. `web/popup_footer_test.mjs`: 12 tests, including "count words", "row name", "enter and exit", "switch keeps the panel", "list row keeps the bubble on a laptop", "phone never enters the panel", "the section writes no address and no history" and "a closing panel popup is hidden at once and shown again if it reopens".
- `DATABASE_URL= go test ./web/ -count=1` and `-short`: ok. The named run printed PASS for TestPopupFooterOpenReportRow, TestReportPanelGeometry, TestReportPanelHasNoPhoneRules, TestPopupCardSurfaceAndGeometry, TestSizeOnlyElementsAreNamed, TestNewControlsMeetTouchTarget and TestNoUnderlineAtRest.
- `go vet ./...` and `gofmt -l web` print nothing. `git diff` of votes.js, trust.js, feed.js and phone.css is empty. No em or en dash and no non ASCII character was added. No database, server or port was used.
- NOT checked: how the panel and bubble look and behave in a real browser (Mac Safari at 1280 and 1440: pin tap, Open report, switch, Back, Escape, focus return, typing). That is the owner's viewing, plan 08-20.

## Contract tests amended (dated 2026-10-29, "by Phase 8 plan 07")

- `popup_contract_test.go` TestPopupCardSurfaceAndGeometry: the negative footer checks kept exactly; added positive checks that map.js builds `popup-card__open-report` and reads `comment_count` and that trust.css has the row rule.
- `layout_rules_contract_test.go`: `sizeOnlyNamed` gains `.popup-card__open-report`, the list hide `html[data-report-panel] .app-shell > .pane--list` and the panel's tip hide; a note says the entries quoting the owner's 2026-10-19 sentence stay true. `touchControls` gains `.popup-card__open-report`.
- `template_contract_test.go` and `report_view_contract_test.go`: not changed; nothing they pin differs.

## Deviations from Plan

### Auto-fixed or adapted

**1. [Rule 3 - Blocking] Existing map.js contract tests pin literal lines**
- Seven tests (harness P4_7, phone layout, popup resize, report view, review fix A and A2, popup close press) pin literal lines of keepCardClearOfTools, panPopupIntoView, onMapResize, onPopupOpen and onPopupClose. Instead of amending them, the map.js changes were shaped to keep every pinned line: `|| phoneDetail() || panelOpen` in keepCardClearOfTools, a second `if (panelOpen)` in panPopupIntoView, onMapResize keeps its own loop (replaceAllPlacements is a second copy of it), the Back to the list label is a second `setAttribute` after the pinned one, `hadFocus` is cleared on the next line, and the link end is wrapped in `if (!hostSwitching)`. Also official_card_heading_test.mjs runs the head of onPopupOpen in a sandbox, so the panel label code sits after that slice.
- Cost: replaceAllPlacements duplicates onMapResize's loop. A later plan may fold them and amend the pins.

**2. [Rule 1/2 - Correctness] A closing panel popup is hidden at once**
- The popup fades out for 200ms after a close; once the panel attribute is removed it would lose the panel rules and drift to the page corner. panelNoteClose sets `visibility: hidden` on its container and panelNoteOpen clears it on any reopen. Third commit.

**3. [Deviation] The important flag**
- The plan's CSS note asked for no !important, and phone.css's flagged left, bottom, transform and width were not copied. See key-decisions: stated top and height, min and max width, and a MutationObserver clearing left and transform. TestReportPanelHasNoPhoneRules pins no `!important` in the file.

**4. [Deviation] Grid column**
- UI-SPEC says the panel "takes the list column's place" and "leaves the map 380px at 900px", so the shell columns become 520px while the panel is open; the map re-measures through the existing resize watcher (watchMapBox). The map's centre is kept, so a pin near the map's left edge may leave the view; to be seen at the viewing.

**5. [Note] Focus return after an empty map tap**
- exitPanel returns focus to the opener only when focus is in the panel, on the body, or lost. After a tap on empty map focus is on the map container, so it stays there (the same as the bubble today). Back, Escape and the browser's back return focus to the row or the pin. Owner to judge.

**6. [Process] Test expectations**
- `TestPopupFooterOpenReportRow` forbids a digit next to the word comment in map.js code, so the plural is built as `'comment' + (count === 1 ? '' : 's')`.

## Notes for later plans

- 08-08 (comment_count): the row already reads `report.comment_count`; nothing to change in map.js. Until the server sends it the row shows only "Open report".
- 08-16 and 08-17: comments mount into `div.report-page__comments` as before; in the panel the wrapper (`.leaflet-popup-content-wrapper`) is the only scroll box, so a sticky composer is `position: sticky; bottom: 0` inside the card body. The panel's wrapper padding is 16px 24px 0 and the body's left pad is 16px; adjust at the viewing.
- Option B: delete the Open report row builder, openReportWords, openReportName and the A11 amendment, call enterPanel from marker click and from onSelect with source list; report-panel.css stays.
- report-page.css times use `--color-text` (08-05 deviation 5); the panel is on `--color-bg`, so they could return to muted. Not done here.

## Known Stubs

None. The row shows no count until the server sends `comment_count` (by design, T-08-30).

## Threat Flags

None. No new endpoint, auth path or storage; the only number on the row is the server's field.

## Commits

- 3fd89beb feat(08-07): laptop panel section and the Open report row
- a9f19b88 feat(08-07): the 520px laptop report panel and its contract tests
- db547dd9 fix(08-07): hide a closing panel popup at once

## Self-Check: PASSED

Files found: report-panel.css, popup_footer_test.mjs, report_panel_contract_test.go. Commits 3fd89beb, a9f19b88 and db547dd9 found. No file was deleted.
