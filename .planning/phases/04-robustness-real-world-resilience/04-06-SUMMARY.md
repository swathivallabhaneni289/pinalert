---
phase: 04-robustness-real-world-resilience
plan: 06
subsystem: ui
tags: [go, html-template, chi, disclaimer, chrome, emergency-numbers]
status: complete

requires:
  - phase: 03.1-report-history-views
    provides: the four full page templates and their handlers as built
provides:
  - one emergency numbers setting (EMERGENCY_PRIMARY, EMERGENCY_OLDER, EMERGENCY_STATE_LINES) read once at startup
  - handlers.Chrome carrier on Deps, two configs and four view models
  - disclaimer_bar.html.tmpl partial and chrome.css, bar first inside div.top-chrome on every existing full page
  - CONTACT_EMAIL read and validated at startup
affects: [04-08, 04-09, 04-12, 04-15, 04-18, 04-23, 04-27]

tech-stack:
  added: []
  patterns:
    - "Zero value default via Chrome.withDefaults, mirroring the RequestLinkRateLimit default"
    - "Full page set derived from embedded templates (findFullPageTemplates) for the bar contract test"

key-files:
  created:
    - internal/api/handlers/chrome.go
    - internal/api/handlers/chrome_render_test.go
    - web/templates/disclaimer_bar.html.tmpl
    - web/static/css/chrome.css
    - web/chrome_contract_test.go
  modified:
    - internal/api/handlers/page.go
    - internal/api/handlers/auth.go
    - internal/api/router.go
    - cmd/server/main.go
    - web/templates/index.html.tmpl
    - web/templates/login_gate.html.tmpl
    - web/templates/verify_outcome.html.tmpl
    - web/templates/profile.html.tmpl

key-decisions:
  - "The bar partial reads only .Chrome fields, so 04-12 can reuse it on public pages unchanged"
  - "TelHref returns template.URL so html/template keeps the tel scheme; the bar never calls it"
  - "main.go imports net/mail as netmail because a local variable named mail (the mailer) shadows the package"

requirements-completed: [ROBUST-08]
duration: about 35 min
completed: 2026-10-16
---

# Phase 4 Plan 06: Unofficial bar on every full page Summary

The thin unofficial bar now opens the body of the login, verify outcome, map and Activity pages inside a sticky `div.top-chrome`, with the exact D-22 sentence composed from one numbers setting and a single link to `/disclaimer`.

## Task commits

| Task | Commit | What |
|------|--------|------|
| 1 | 5d7c5e8 | `chrome.go`: EmergencyNumbers, loader, Chrome, withDefaults; unit tests |
| 2 | 178c24d | Chrome on both configs and four view models, router local copies, main.go env wiring; `TestChromeReachesViewModels` |
| 3 | da69d4b | bar partial, `chrome.css`, `.top-chrome` in four templates, render tests and the derived contract tests |

## Task 1 prerequisite grep results

- (a) `trust.js` script tag present at `web/templates/index.html.tmpl` line 208, untouched.
- (b) `VoteRadiusKm` present in `page.go` (config, view model, literal) and `data-vote-radius-km` on the shell in `index.html.tmpl`.
- (c) `TrustJSON`, `SeverityWord`, `Expired` present in `auth.go` (`profileReport`, `newProfileReport`).
- (d) Full page set from the `<html` rule is exactly `templates/index.html.tmpl`, `login_gate.html.tmpl`, `verify_outcome.html.tmpl`, `profile.html.tmpl`. `check_inbox`, `account_header`, `theme_toggle` are partials.
- (e) No bell or Updates markup anywhere in `web/templates/` (the only `updates` hit is an unrelated modal `aria-labelledby`), so nothing was moved inside `.top-chrome` beyond the account header.
- (f) Helpers present: `findFullPageTemplates`, `parseCSSRules`, `ruleBySelector`, `declsOf`, `stripCSSComments`, `newPageTestRequest`, and `pageTestAccount` (a `var`, not a func).

## Verification run

- `go build ./...` and `go vet ./internal/api/... ./cmd/server/ ./web/` clean; `gofmt -l` clean.
- Without a database: `go test ./web/ ./internal/api/... ./cmd/... -count=1` all ok.
- Named tests, all PASS: `TestLoadEmergencyNumbers`, `TestChromeWithDefaults`, `TestChromeReachesViewModels`, `TestDisclaimerBarSentenceIsExact`, `TestDisclaimerBarDropsEmptyNumbers`, `TestDisclaimerBarHasNoTelLink`, `TestExistingPagesRenderWithChrome`, `TestDisclaimerBarOnEveryFullPage`, `TestDisclaimerBarStylesheet`, plus the unedited `TestUserVisibleCopyUsesPlainPunctuation` and `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`.
- With a database, on disposable `p4_0406` (created by me, dropped afterwards, confirmed gone): `DATABASE_URL=postgres://localhost:5432/p4_0406?sslmode=disable go test ./internal/api/... ./web/ -count=1 -p 1 -v` ok for all three packages, 508 PASS lines, 0 SKIP lines.
- `main.css` and `auth.css` untouched. No dash characters on any line added.

## Deviations from Plan

None to the contract. Two notes:

1. The plan's verify commands use `env -u DATABASE_URL`; the owner rules say the sandbox refuses that wrapper, so I ran the no database form with DATABASE_URL simply unset in the shell.
2. Task 3 was not a strict RED first: I wrote the partial, templates and stylesheet before the contract and render tests, so those tests passed on their first run. They are real checks against the final markup (exact sentence, one link, no tel, bar first in body), but they were not seen failing. Task 1 did have a seen RED (assertion failures against a compiling stub).

## Known Stubs

None. `/disclaimer` answers 404 until 04-12 serves it (accepted interim per the plan).

## Interim look (accepted by the plan)

Until 04-08, the fixed account trigger overlaps the bar on the map and Activity pages and the full height pages overflow by the bar height, because `main.css` and `auth.css` are not edited here.

## Follow ups for the orchestrator

- Browser harness checks that change by design: the account trigger now sits on top of the bar (04-08 fixes it); any layout or pixel check of the map, Activity, login or verify pages at phone widths will see the extra bar height.
- The human check in the plan (login, verify, map, Activity at 320x568 and 1280x800, `EMERGENCY_OLDER=` drops the parenthetical) is deferred to 04-27.
- main.go now stops startup if `CONTACT_EMAIL` is set to something that is not a bare email address; 04-27 should document the setting.

## Threat Flags

None beyond T-04-18, which is mitigated: sentence pinned by test, numbers validated to 2 to 15 digits, empty older list drops the parenthetical, bar never shows 1070 or 1077, main.go logs the loaded numbers.

## Self-Check: PASSED

Files exist (chrome.go, chrome_render_test.go, disclaimer_bar.html.tmpl, chrome.css, chrome_contract_test.go) and commits 5d7c5e8, 178c24d, da69d4b are on the worktree branch.
