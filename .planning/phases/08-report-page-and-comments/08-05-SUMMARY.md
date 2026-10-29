---
phase: 08-report-page-and-comments
plan: 05
subsystem: web
tags: [report-page, timeline, share, phone, vanilla-js, css]
requires:
  - phase: 08-01
    provides: GET /api/reports/{id}/timeline (gated, no-store, events with kind, tone, text, at, yours)
provides:
  - window.PinalertReportPage.create(report), the mount agnostic page body builder
  - the phone report page: What happened so far under the shipped card, a Share button slot, an empty comments mount
  - arrangeCard order fix (Stop updates and Flag above the page node)
affects: [08-06, 08-07, 08-16, 08-17]
tech-stack:
  added: []
  patterns: [build once update in place, rows kept as records, request sequence number drops stale answers]
key-files:
  created:
    - web/static/js/report-page.js
    - web/static/css/report-page.css
    - web/static/icons/share-2.svg
    - web/report_page_test.mjs
    - web/report_page_contract_test.go
  modified:
    - web/static/js/map.js
    - web/templates/index.html.tmpl
    - web/css_contract_test.go
    - web/labels_contract_test.go
    - web/layout_rules_contract_test.go
    - web/template_contract_test.go
    - web/mute_contract_test.go
key-decisions:
  - "The page is hosted on the phone only (setHosted(phoneDetail()) in applyPopupPlacement); the laptop bubble is unchanged until 08-07."
  - "No page is built on the Demo (window.PinalertDemoPage), because the timeline route is not under /demo."
  - "Small text on the page takes --color-text, not muted, because the phone sheet carries the severity tint (P8-R8)."
metrics:
  tasks: 2
  commits: 2
  files: 12
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 05: The report page body on the phone Summary

One mount agnostic builder (report-page.js) puts "What happened so far", a Share slot and an empty Comments mount under the phone's shipped card, refreshed in place and kept below Stop updates and Flag across any resize.

## What was built

### The page object (for 08-06, 08-07, 08-16, 08-17)

`PinalertReportPage.create(report)` returns `{ el, share, update(report), setHosted(on), open(), close(), refresh() }`. `PinalertReportPage.formatTimelineTime(iso, now, yours)` is exported too (`now` is a Date or milliseconds, `yours` optional).

- `el`: `section.report-page`, born `hidden`. Children: `section.report-page__timeline` and `div.report-page__comments`.
  - Timeline section: `h2.report-page__heading` (id `report-page-history-N`, text "What happened so far"), `ol.report-timeline` (`aria-labelledby` the heading, `aria-busy` true while it loads), `div.report-timeline__error` holding `p.report-timeline__error-title`, `p.report-timeline__error-line` and `button.report-page__retry` ("Try again").
  - Row: `li.report-timeline__row` holding `time.report-timeline__time` (with `datetime`), `span.report-timeline__dot` plus `report-timeline__dot--good|neutral|bad` (aria-hidden) and `p.report-timeline__text`. Loading rows are `li.report-timeline__row.report-timeline__row--skeleton` with `span.report-timeline__bar` plus `--time`, `--dot`, `--text`.
  - Comments mount: `div.report-page__comments` with `data-report-id`, empty. It is never replaced by a refresh. Plans 08-16 and 08-17 mount into it; the page gives it a 24px top margin only once it is not empty.
- `share`: `button.vote-btn.report-page__share`, text "Share", glyph by `::before` mask of `/static/icons/share-2.svg`. Hidden unless the page is hosted AND `window.PinalertShare.open` exists (re-checked on every `update` and `setHosted`, so a late loading Share script works). Click calls `PinalertShare.open({ reportId, category, opener })` and stops propagation. It is not inside `el`; map.js puts it in the actions group.
- `setHosted(on)` shows or hides `el` and Share; `open()` marks the report opened and, if hosted, fetches and starts the one 30 second interval; `close()` stops both and drops any answer in flight; `refresh()` refetches now if showing.
- Refetch triggers: open, host change while open, a changed signature of `trust.confirmed_places`, `trust.disputed_places`, `visibility`, `view`, `your_vote`, the 30 second tick while `document.visibilityState === 'visible'`, and Try again.

### map.js wiring

`card.page` is built in `buildPopupCard` right after the vote block (not on the Demo, and only when report-page.js loaded). `updatePopupCard` calls `card.page.update(report)` last. `applyPopupPlacement` calls `card.page.setHosted(phone)` after `arrangeCard`. `onPopupOpen` calls `cards[id].page.open()` and `onPopupClose` calls `cards[id].page.close()` (after the GDACS early returns), and the card removal branch closes it too. `arrangeCard` puts Stop updates, its error line and Flag before the page node through a local `place(node)`. The plan 39 markers are untouched (1 each).

### Other

- `index.html.tmpl`: report-page.css after trust.css, report-page.js after trust.js and before map.js. All Phase 5, 3.3 and 7.1 pieces in the file are untouched.
- `report-page.css` has no media query except `prefers-reduced-motion` and `forced-colors`; hosts place it.

## Verification

- `node --test web/*.mjs`: 1352 tests, 1352 PASS, 0 FAIL, 0 skipped. `node --test web/report_page_test.mjs`: 9 PASS (build once, update keeps nodes, timeline states, when it fetches, a late answer after close is dropped, time words, share hidden without the window script, the page script has no markup sink, child order across a resize).
- `DATABASE_URL= go test ./web/ -count=1`: ok (also with `-short`). The named run `-run 'TestReportPage|TestPopupCardSurfaceAndGeometry|TestNoMarkupSinksAnywhere|TestVendorMapScriptsLoadInDependencyOrder|TestNewIconsAreRegisteredAndUsed|TestNewControlsMeetTouchTarget|TestNewScriptsUsePlainPunctuation'` printed 10 PASS, 0 FAIL, 0 SKIP, including TestReportPageReusesTrustBlock, TestReportPageNoThresholds, TestReportPageNodeStaysLast and TestReportPageIsLinkedInOrder.
- `go vet ./...` and `gofmt -l web` print nothing.
- `git diff` of votes.js, trust.js and popup_contract_test.go is empty.
- No database, server or port was used. The phone check (Safari in the iPhone simulator, rotating across 900px) is for the owner's viewing, plan 08-20.

## Deviations from Plan

### Auto-fixed or adapted

**1. [Rule 3 - Blocking] Extra test files amended with dated notes**
- `web/mute_contract_test.go` (TestMuteMountedBeforeFlag pinned the literal `lastParent.appendChild(mute.el);` strings that the Pitfall 5b fix necessarily replaces by `place(mute.el)`, `place(mute.error)`, `place(flag)`) and `web/labels_contract_test.go` (the 12px timeline dot and its loading twin are discs with `border-radius: 50%`, UI-SPEC "Radii": 50% for discs; two heads exempted with a note). Neither file was in the plan's list. No assertion was weakened: Mute, its error line and Flag keep their order, and the mute test also pins `place` itself.
- `css_contract_test.go` and `labels_contract_test.go` went into the Task 1 commit (not Task 2) so every commit is green.

**2. [Rule 2 - Correctness] No page on the Demo**
- The Demo runs map.js on a sandbox API under /demo, which has no /timeline route; the real route would answer for the wrong world. `window.PinalertDemoPage` skips building the page, as it skips Mute and Flag.

**3. [Deviation] Number constants**
- `TestReportPageNoThresholds` could not literally allow only the poll, timeout and 60 second numbers: the clock needs 12, padding needs 10, the skeleton needs 3, the gone status is 404. They are named constants (`HALF_DAY_HOURS`, `TWO_DIGITS`, `SKELETON_ROWS`, `STATUS_GONE`) listed in the test; no other number above 1 may appear, and no comparison on a place count.

**4. [Deviation] your_vote in the signature**
- The refetch signature also carries `your_vote`, so the reader's own line ("You confirmed it.", shown as "Now") appears after their vote even when the place counts do not change.

**5. [Deviation] Time column colour**
- UI-SPEC Surface 4 says the time column is muted. The phone sheet is the severity tint and P8-R8 puts muted text on the page colour only, so the times and the error line take `--color-text`. The neutral dot stays `--color-text-muted`. When 08-07 hosts the page on `--color-bg`, the times may return to muted.

**6. [Deviation] Page created before Mute, not after Flag**
- The plan says create the page after the flag control and insert the Share button after the vote block's error line. Creating it right after `block.error` gives the same DOM order in one step; TestMuteMountedBeforeFlag still passes unchanged on that part.

**7. [Note] Retry moves focus**
- Try again focuses the section heading (tabindex -1) because the button leaves the page; that is the reader's own press. Refreshes never call focus (tested).

**8. [Process slip]** One stray `python3 --version` was run by mistake while editing; it changed nothing. No Python was used for any work.

## TDD Gate Compliance

Plan 05 is not `type: tdd`. The tests were written first and seen red: `node --test web/report_page_test.mjs` failed with ENOENT before report-page.js existed, and "child order across a resize" failed (page node ahead of Stop updates and Flag) before the arrangeCard change. Per the plan's one commit per task, the tests and code landed in the same commits, so there are no separate `test(...)` commits.

## Known Stubs

- `div.report-page__comments` is intentionally empty until plans 08-16 and 08-17.
- Share does nothing visible until the Share window script (08-06) sets `window.PinalertShare`; the button stays hidden until then, by design.

## Threat Flags

None. The new network read is the 08-01 endpoint behind the existing session gate; all server text is set with `Pinalert.setText`.

## Commits

- 1469cf5f feat(08-05): report page builder, its CSS and the share-2 glyph
- 8fc2c5e0 feat(08-05): mount the report page in the phone card and keep its order

## Self-Check: PASSED

Files found: report-page.js, report-page.css, share-2.svg, report_page_test.mjs, report_page_contract_test.go. Commits 1469cf5f and 8fc2c5e0 found. No file was deleted.
