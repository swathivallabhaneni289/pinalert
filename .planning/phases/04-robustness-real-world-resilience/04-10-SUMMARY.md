---
phase: 04-robustness-real-world-resilience
plan: 10
subsystem: web-frontend
tags: [report-form, refactor, location-interface, leaflet, css-split, description-hint]
status: complete
requires:
  - 04-06 (top chrome bar edit in index.html.tmpl, untouched here)
provides:
  - window.PinalertReportForm (mount, open, close) in web/static/js/report-form.js
  - the location interface (current, choose, isSet, missingMessage, searchFailureMessage, optional start and reset)
  - web/templates/report_form.html.tmpl shared field partial
  - web/static/css/modal-map.css
affects:
  - 04-19 (Low data posting mounts the same form and partial)
  - 04-22 (queue paths build on the open and mount signatures)
tech-stack:
  added: []
  patterns:
    - shared form script behind a small location object, map implementation in a separate script
    - template partial invoked by file name, static markup only
key-files:
  created:
    - web/static/js/report-form.js
    - web/templates/report_form.html.tmpl
    - web/static/css/modal-map.css
    - web/report_form_contract_test.go
  modified:
    - web/static/js/modal.js
    - web/static/css/modal.css
    - web/templates/index.html.tmpl
    - web/js_contract_test.go
    - web/votes_contract_test.go
decisions:
  - "Search failure Error text uses a non user visible literal ('Search request failed'); the catch reads fieldMessage only, so the main form still shows the server's message or its own constant exactly as before (U8)"
  - "mount and openForm return quietly when not mounted or when an option is missing, so a failed script load never throws on the page"
metrics:
  tasks: 2
  files: 9
  completed: 2026-10-16
---

# Phase 4 Plan 10: Shared report form behind a location interface Summary

The report form logic now lives in one map free script, `report-form.js`, behind a five method location interface that `modal.js` implements with Leaflet. The main page posts exactly as before. The shared field markup is one partial, the three map rules sit in `modal-map.css`, the secondary button reads "Cancel report", and the Description box carries the owner's hint line.

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | 25c91d4 | Shared partial with Cancel report and the description hint, modal-map.css split, disabled look keeps only its cursor, five new contract tests |
| 2 | 83272d5 | report-form.js, Leaflet picker rewrite of modal.js, script tag, three new contract tests, three re-pointed tests |

## Task 1 prerequisite greps (Step 0, all passed)

- trust.js deferred versioned tag in index.html.tmpl: 1. `'report-modal'` in trust.js: 1.
- `TestTrustModuleLoadsBeforeItsConsumers` (trust_contract_test.go:101) and `TestTrustDisclosure` (:329) both defined.
- `class="top-chrome"`: 1. `chrome.css?v={{.AssetVersion}}`: 1.
- The report modal block held exactly the 21 pinned ids in the pinned order.
- modal.js: 35 top level functions, 0 references to PinalertReportForm.
- The three map rules were rules only in modal.css; the one main.css hit is a comment.
- `gofmt -l web/votes_contract_test.go web/js_contract_test.go` printed nothing at the start (both clean) and prints nothing now.
- The three map rule declarations in modal.css matched the pinned values.

## RED failures recorded

- Task 1: `report_form.html.tmpl` and `modal-map.css` did not exist (four tests failed on file reads), and `TestReportFormSubmitDisabledLook` failed because `#submit-report:disabled` still declared opacity.
- Task 2: `report-form.js` did not exist, so `TestReportFormLocationInterface`, `TestReportFormOwnsTheFormLogic`, `TestReportFormNewFilesPlainText`, `TestIconGlyphsAreClassDriven`, `TestLocationSearchUsesTextSinksAndExistingPinPlacement` and `TestToastHasExactlyOneImplementation` failed on the read, and `TestReportFormScriptOrder` failed with zero script tags.

## Final function counts

- `modal.js`: 13 top level functions (5 kept Leaflet functions plus 8 interface functions).
- `report-form.js`: 31 top level functions (30 moved plus `mount`; `openModal` and `closeModal` became `openForm` and `closeForm`).

## Verification run

- `node --check` on both scripts passes; `go vet ./web/` passes.
- The 20 named tests of Task 2's first command all print `--- PASS`.
- `go test ./... -count=1` passes for every package (DATABASE_URL unset).
- On the disposable database `p4_0410` (created by this run, dropped after): `go test ./internal/api/handlers/ ./web/ -run 'TestPageShell|TestExistingPagesRenderWithChrome|TestReportForm|TestModalMapRulesMoved' -count=1 -p 1 -v` passed, 14 tests, 0 SKIP lines.
- A throwaway node smoke script in the scratchpad (not in the repo) loaded both scripts against a stub DOM: the fab click opened the form and an empty submit showed "Choose a category to continue.", so no name left on the wrong side of the split throws at load or open.
- `gofmt -l` prints nothing for report_form_contract_test.go, votes_contract_test.go and js_contract_test.go. No gofmt -w was run.
- Files the plan forbids editing (app.js, votes.js, feed.js, map.js, trust.js, visibility.js, account-menu.js, Go sources, page_test.go, css_contract_test.go, template_contract_test.go, trust_contract_test.go) are untouched.

## Deviations from Plan

None. The plan was executed as written.

Two small notes on tooling, not on the plan: the file write tool turned a Go escape sequence for the two dash characters into the literal characters, so `TestReportFormNewFilesPlainText` builds them with `string(rune(0x2014))` and `string(rune(0x2013))` instead, which keeps the test file dash free. The ellipsis in the in flight label is the literal character in `report-form.js` and the test builds it with `string(rune(0x2026))`.

## Known Stubs

None. `formVariant` is stored by `mount` but nothing reads it yet, by design: the low variant behaviour belongs to 04-19.

## Threat Flags

None. No new route, input, storage or server code. T-04-01 is covered: the re-pointed `TestLocationSearchUsesTextSinksAndExistingPinPlacement` counts zero markup sink properties in both `report-form.js` and `modal.js` and requires at least two `Pinalert.setText(` calls in `renderResultRow`.

## For the orchestrator

- The human check (V-24, in 04-27) covers the real browser behaviour: pin from phone position, typed place, map tap, "Cancel report" label, disabled Post report look, the hint line under Description at 320, 360 and 390 px, and the discard flow.
- The Post report disabled look and the Cancel report label change by design; any browser harness expectation of the old "Cancel" text or the 0.6 opacity must be updated.

## Self-Check: PASSED

Created files exist (report-form.js, report_form.html.tmpl, modal-map.css, report_form_contract_test.go, this SUMMARY), commits 25c91d4 and 83272d5 exist on the worktree branch, and every verification command above passed.
