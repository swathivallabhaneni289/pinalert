---
phase: 04-robustness-real-world-resilience
plan: 26
subsystem: web
tags: [moderation, moderator-screen, javascript, css]
requires:
  - phase: 04-23
    provides: the moderator page shell, the four moderator JSON routes and the ModRow JSON
  - phase: 04-17
    provides: Pinalert.writtenLine
  - phase: 04-13
    provides: Pinalert.showToast(message, action, durationMs)
provides:
  - web/static/js/moderate.js (queues, rows, real heading counts, Find, Keep, confirmed Remove, Refresh lists)
  - web/static/css/moderate.css (640px phone first column, neutral rows, action tracks)
  - the final moderate.html.tmpl structure (count spans, list containers, empty texts, script and style tags)
  - web/moderate_contract_test.go (ten contract tests)
affects: [04-27]
tech-stack:
  added: []
  patterns:
    - "Heading counts come only from the server counts object and are lowered by one dropReport rule"
    - "Busy state uses aria-disabled so the focused button keeps focus"
key-files:
  created:
    - web/static/js/moderate.js
    - web/static/css/moderate.css
    - web/moderate_contract_test.go
  modified:
    - web/templates/moderate.html.tmpl
key-decisions:
  - "trust.js loads on the moderator page so the severity label has one builder (script order theme.js, app.js, trust.js, moderate.js, account-menu.js)"
  - "Counts drop by one with a floor of 0 after Keep or Remove, no refetch, so rows never move under a thumb"
status: complete
completed: 2026-10-16
---

# Phase 4 Plan 26: Moderator screen client Summary

**A listed moderator can now read the three lists with real counts, look a report up by id or pasted link, Keep it at once, Remove it after a named confirmation and refresh on demand, with every string written through Pinalert.setText.**

## Commits

- `68d8517` Task 1: moderate.js, the template's list containers and script tags, eight contract tests
- `2a5c70f` Task 2: moderate.css, the stylesheet link, TestModerateStylesheet and TestModerateNotLinkedAnywhere
- the SUMMARY commit follows this file

BASE: `12b848f4ccd7cdf0c944e57ffeb48cc91d9de8ba`.

## Step 0 re-grep results

- `trust.js`: `createSeverityLabel` and `updateSeverityLabel` exist and are exported in `window.PinalertTrust`. Its only top level statements that touch other modules are the `document.addEventListener('keydown', onDocumentEscape, true)` call and the export; `PinalertVotes` is read only behind `window.PinalertVotes && typeof ...`. `PinalertVisibility` is not referenced. `onDocumentEscape` acts (and stops propagation) only when focus sits in a `.report-row` or `.leaflet-popup`, or when a popup is open and focus is on the body or its pin; focus in a `.mod-row` never qualifies, so `moderate.js` sees Escape on the Remove confirmation.
- `app.js`: `SEVERITY_WORDS`, `CATEGORY_LABELS`, `writtenLine`, `relativeTime`, `setText` and `function showToast(message, action, durationMs)` all present and exported.
- `trust.css`: `.severity-label` rules and `.visibility-tag:not([hidden])` present (radius 12px).
- ModRow: struct `modRowJSON` in `internal/api/handlers/moderation.go`, tags `id`, `category`, `severity`, `description`, `created_at`, `expires_at`, `written_at`, `serious`, `moderation`, `view`, `visibility_word`, `scores`, `model`, `flags` (`total`, `reasons` with `reason`, `label`, `count`). Queue keys `review`, `flagged`, `pending`, `counts`; find keys `report`, `lists`. View values `active`, `disputed`, `resolved`, `past`, `none` (`ModView*` in `internal/service/moderation.go`); find list names `review`, `flagged`, `pending`.
- Main page names: `feed.js` has `VIEW_PARAM = 'view'` and `REPORT_PARAM = 'report'`; the view names are `active`, `disputed`, `resolved`, `past`.
- Routes registered with `r.Handle` in `internal/api/router.go` for `/moderate` and the four `/api/moderation/...` paths.
- Test helpers `stripJSComments`, `jsFunctionBody`, `windowAfter`, `findTagWindow`, `stripCSSComments`, `parseCSSRules`, `ruleBySelector`, `declsOf` exist. No `p426` or `TestModerate` name existed before.
- `TestModeratorPageRenders` (04-23) pins `<h2 id="mod-h-...">` opening tags, the heading and line texts once, and the banned substrings; none of that changed.
- 04-23 had already placed: the head with `main.css`, `trust.css`, `auth.css`, `chrome.css`, `theme.js`; the bar and account header; the h1 and line; `#mod-refresh`, `#mod-refresh-error`, the find form with its input, help and button; `#mod-find-result`; the three `mod-section` sections with their lines; `#toast`; the `app.js` and `account-menu.js` tags. This plan added the count spans, the list ids with `role="list"` and `aria-labelledby`, the three empty texts, the `trust.js` and `moderate.js` tags and the `moderate.css` link.

## Recorded departure: trust.js on the moderator page

The outline and UI-SPEC script order for the moderator page is `theme.js`, `app.js`, `moderate.js`, `account-menu.js`. This plan loads `trust.js` as well, so the final order is `theme.js` (head), `app.js`, `trust.js`, `moderate.js`, `account-menu.js`. Reason: UI-SPEC row item 1 asks for the shared read only severity label and Phase 3 plan 03-16 makes `PinalertTrust.createSeverityLabel` and `updateSeverityLabel` its one builder; building the label in `moderate.js` would create a second builder. `votes.js` and `visibility.js` are not loaded and `trust.js` is inert on this page.

For 04-27's `docs/MODERATION.md` and the orchestrator: the outline "Script order" Moderator entry and its "Open questions resolved during planning" note were NOT edited here because `04-PLAN-OUTLINE.md` is outside this plan's `files_modified` and the rules file forbids touching other files. The orchestrator or 04-27 should record the departure in the outline.

## Measured figures

- `Pinalert.setText(` occurs 28 times in `moderate.js` (test floor 20).
- `TestModerateNotLinkedAnywhere` scanned 13 templates and 18 scripts (floors 8 and 10).
- `.mod-row__state:not([hidden])` restates the built `.visibility-tag:not([hidden])` radius, 12px (read from `trust.css` by the test). Declaration not restated: `min-height: 24px` and the top margin, because the plan keeps lengths in `moderate.css` to tokens and the row's flex gap already spaces the chip. The chip is 19.6px tall from its line height, so it reads slightly shorter than the shared severity label beside it; the owner can judge this in the V-24 phone check.

## Honest limit of the decrement rule

After a Keep or Remove the lists that held the id drop by one with a floor of 0 and nothing is refetched. A report beyond a list's first 100 rows that is acted on from another list leaves that list's count one high until Refresh lists.

## Verification

- `node --check web/static/js/moderate.js`, `go vet ./web/`, `gofmt -l` clean; no U+2014, U+2013 or U+2026 in the four files.
- All ten new tests pass, plus `TestNoMarkupSinksAnywhere`, `TestStripJSComments`, `TestPhase4HiddenGuard`, `TestPhase4ContrastPairs`, `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`, `TestDisclaimerBarOnEveryFullPage`, `TestUserVisibleCopyUsesPlainPunctuation`, `TestNoPillShapedControls`.
- `go test ./... -count=1` with `DATABASE_URL` unset: every package ok (no sibling RED test was met).
- Database run on a fresh `p4_0426` (created and dropped by this run): `DATABASE_URL=postgres://localhost:5432/p4_0426?sslmode=disable go test ./internal/api/ ./internal/api/handlers/ -run TestModerator -count=1 -p 1 -v` gave PASS for TestModeratorPageRenders, TestModeratorRoutesIndistinguishableFrom404, TestModeratorQueues, TestModeratorKeep, TestModeratorRemove, TestModeratorRemoveById and TestModeratorAuditRow, 0 SKIP.
- Wave sample `go test ./internal/... ./web/ -count=1 -p 1 -v` on `p4_0426`: all packages ok, 835 top level PASS, 1 SKIP. The one SKIP is `TestScreeningFixture` (the fixture has no recorded scores until 04-27 runs `cmd/modrecord`); it is not database backed, so the database SKIP count is 0.

## Deviations from Plan

1. **[Test wording] `.toFixed(2)` is checked in `scoreLine(item)`, not in `buildScores`.** The plan puts `.toFixed(2)` in `scoreLine` but lists it as a `buildScores` assertion. `TestModerateRowStructure` asserts `buildScores` calls `scoreLine(` and `scoreLine` holds `.toFixed(2)`. Same guarantee, consistent with the function split the plan describes.
2. **Added small helpers** beyond the plan's function list: `listByKey`, `makeEl`, `makeButton`, `scoreRank`, `entryOf`, `actionDone`, `actionFailed`, `finishFind`. `buildWhy` takes a third `lists` argument for a found report. All stay at the two space module level.
3. **The sandbox refuses `env -u` and heredocs and long one liners**, so quick runs used `go test` with `DATABASE_URL` unset (it was empty in the shell), and the verify one liners were run as separate commands. `createdb p4_0426` was run alone (no `dropdb` first) and the database was dropped at the end.
4. **The outline "Script order" note was not written** (see the recorded departure above).
5. A context usage hook repeatedly reported 90 percent used while the harness token counter showed about 14.8 million tokens left; it did not affect the work.

## Known Stubs

None. Every list, count and result is filled from the 04-23 JSON.

## Threat Flags

None beyond the plan's register. The only href is built by `mainPageHref` from the closed view set and a numeric id; no flagger identity is read; no server message text is displayed.

## Notes for the orchestrator

- Browser harness and the V-24 human check (phone widths, Telugu and Hindi text, offline Refresh) are for the owner in 04-27; no server was started.
- `STATE.md`, `ROADMAP.md` and `REQUIREMENTS.md` were not touched.

## Self-Check: PASSED

Files exist: `web/static/js/moderate.js`, `web/static/css/moderate.css`, `web/moderate_contract_test.go`, `web/templates/moderate.html.tmpl`. Commits `68d8517` and `2a5c70f` are on the branch. All plan verification commands ran and passed, and `p4_0426` was dropped.
