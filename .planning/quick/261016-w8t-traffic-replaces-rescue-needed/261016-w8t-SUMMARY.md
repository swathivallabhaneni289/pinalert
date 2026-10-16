---
phase: quick-261016-w8t
plan: 01
subsystem: reports, categories
tags: [category, traffic, rescue_needed, swagger, trust-model]
status: complete
requirements: [QUICK-261016-w8t]
completed: 2026-10-16
key-files:
  created:
    - internal/testutil/stored_report.go
    - internal/api/handlers/category_retire_e2e_test.go
    - web/category_retire_contract_test.go
    - web/static/icons/traffic.svg
  modified:
    - internal/service/report.go
    - internal/api/handlers/reports.go
    - internal/api/handlers/auth.go
    - web/static/js/app.js
    - web/static/css/main.css
    - docs/TRUST-MODEL.md
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
---

# Quick 261016-w8t: Traffic replaces Rescue needed for new reports

A new report can no longer be rescue_needed (400 on the field category with the plain words message, before replay, the rate gate and the screener), Traffic takes its slot in the form, and every report already stored as rescue_needed keeps all of its rules.

## What changed

- Go: `CategoryTraffic`, `Categories` (all ten known, read paths), `PostableCategories` (nine, form and POST enum), `Category.Postable()`, `RescueNeededRetiredMessage`. `ValidateSubmitInput` keeps the unknown slug branch first, then refuses a valid but not postable category.
- Browser: `CATEGORIES` is the nine postable (Traffic just before Other), `RETIRED_CATEGORIES` is `['rescue_needed']`, `KNOWN_CATEGORIES` is their union, used by `iconClass` and `setCategory` so old rows keep the glyph and the Rescue chip still filters. `CHIP_LABELS` untouched.
- Icon and label: Lucide car front in `traffic.svg`, `.icon-glyph--traffic` rule, `Traffic` in `CATEGORY_LABELS` and the server `categoryLabels`.
- Swagger: POST enum lists the nine postable, response enums and the `?category=` enum list all ten. Regenerated with swag v1.16.4; a second `make swag` leaves `docs/` clean. Churn was limited to the enums and descriptions.
- TRUST-MODEL.md: one dated bullet in Honest limits, no new heading.
- Tests: rescue fixtures that were POSTed now go straight into the database through `testutil.InsertStoredReport` and `storeReportAs`, every old assertion kept. Posting a rescue report is asserted as refused (service, SubmitChecked, e2e at three severities, screening, rate limit). The serious bucket stays covered through critical posts (Traffic critical). New: `TestCategoryLists`, `TestSubmitRefusesRetiredRescueCategory`, `TestTrafficCategoryPostsAndFilters`, `TestTrafficReplacesRescueInTheForm`, `TestCategoryLabelsCoverEveryKnownCategory`, `TestSwaggerCategoryEnumsSplitPostableFromStored`.

## Commits (all path limited, with the Opus 5.5 trailer)

- 64babe2 test(261016-w8t): store rescue fixtures straight into the database in the e2e tests
- 1e1daaa feat(261016-w8t): traffic replaces rescue needed for new reports, stored rescue rows keep their rules
- 3e3f437 feat(261016-w8t): Traffic tile, label and icon, stored rescue reports keep theirs
- ed09f51 docs(261016-w8t): Swagger lists postable and stored categories apart, trust model note
- 9008607 style(261016-w8t): drop a dash from a new test message

## Verification

Disposable database `p_w8t_exec` (dropped at the end), `DATABASE_URL='postgres://localhost/p_w8t_exec?sslmode=disable'`.

- `go build ./...` and `go vet ./...` clean, `gofmt -l` empty.
- `go test ./... -p 1 -count=1 -v`: 14 packages `ok`, 0 `FAIL`, 0 `panic`, 3038 `--- PASS` lines (subtests included).
- `--- SKIP` count is 2, neither is a database skip: `TestGeocodeLive` (needs `GEOCODE_LIVE=1`, calls real Photon and Nominatim) and `TestScreeningFixture` (needs recorded scores from `go run ./cmd/modrecord`, plan 04-27). Both are env gated and unchanged by this task. No database test skipped.
- `node --test web/*.mjs`: 139 pass, 0 fail.
- `visibility.go` and `coarsen.go` have no diff against the base. No CHIP_LABELS line changed.

## Deviations from Plan

- An extra small commit (9008607) removed a dash from a message string in a new test, found by a final scan for em and en dashes.
- The plan's `cd /Users/swathivallabhaneni/code/pinalert` in verify commands was not used: all work and tests ran inside the worktree.
- No other deviations. Tests ranging over `Categories` (visibility, moderation_rating, reliability, moderation_view, reporter) passed unchanged with ten entries, so no file outside the plan was edited.

## Open items for the orchestrator (not done here)

- `policy.html.tmpl` line 29 still says "A serious report, critical or Rescue needed, is published at once". That copy belongs to the owner and was left unchanged.
- The `CHIP_LABELS` comment in `web/static/js/app.js` still counts "the other five categories". The chip set is the 3.4 session's call.
- An offline queued report saved as rescue_needed before this change now comes back rejected with the new message, and reopening it leaves no tile selected.

## Self-Check: PASSED

Created files exist (stored_report.go, category_retire_e2e_test.go, category_retire_contract_test.go, traffic.svg) and all five commits are in `git log`.
