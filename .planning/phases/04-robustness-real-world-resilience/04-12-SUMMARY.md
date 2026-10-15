---
phase: 04-robustness-real-world-resilience
plan: 12
subsystem: ui
tags: [go, html-template, chi, disclaimer, policy, legal-pages, glossary]
status: complete

requires:
  - phase: 04-robustness-real-world-resilience
    provides: Chrome, EmergencyNumbers, the bar partial (04-06); legal tokens and hidden guard (04-08); test harness (04-09)
provides:
  - public GET /disclaimer and GET /policy, no account data, Cache-Control private, max-age=300
  - service.HeldLabel, HeldSentence, RemovedLabel, RemovedSentence, RemovedPolicyPointer, DisclaimerResolvedEntry for 04-16
  - service.DisclaimerGlossary built from trustcopy.go identifiers
  - legal.css
affects: [04-16, 04-23, 04-27]

key-files:
  created:
    - internal/service/moderationcopy.go
    - internal/service/moderationcopy_test.go
    - internal/api/handlers/legal.go
    - internal/api/handlers/legal_e2e_test.go
    - web/templates/disclaimer.html.tmpl
    - web/templates/policy.html.tmpl
    - web/static/css/legal.css
    - web/legal_contract_test.go
  modified:
    - internal/api/router.go
    - internal/api/gate_test.go

key-decisions:
  - "policyLastUpdated is 16 October 2026, the real date the text was written"
  - "Test side corrections made while going green are committed with the GREEN commit of Task 3"

requirements-completed: [ROBUST-08, ROBUST-10]
completed: 2026-10-16
---

# Phase 4 Plan 12: Public Disclaimer and Policy pages Summary

A logged out visitor can read the Disclaimer and Policy pages: the bar, a numbers block from the one numbers setting with 112 as the only tap to call link, a nine entry glossary read from the app's own trust sentences, honest Policy text, and contact blocks that exist only when CONTACT_EMAIL is set.

## Task commits

| Task | Commit | What |
|------|--------|------|
| 1 RED | 38fab81 | declarations plus failing tests for the Disclaimer, numbers block, exempt route |
| 1 GREEN | 2493895 | OlderNumbersLabel, handlers.Legal, disclaimer template, /disclaimer route |
| 2 RED | f697a4d | failing copy, glossary and glossary render tests |
| 2 GREEN | 8be8d0e | Activity and Resolved strings, DisclaimerGlossary, Reports expire figures |
| 3 RED | 722ba73 | failing Policy, contact block, template source and CSS tests |
| 3 GREEN | 4491e0b | policy template, legal.css, /policy route, router doc comment |

## Step 0 prerequisite grep hits (base SHA 852f3049b3d15b98eb2562452d1427be79b75156)

- `internal/service/trustcopy.go:65: func provisionalLeadSentence() string`
- `trustcopy.go:90: const expiredLeadSentence`, `:97-100` the four currencyLabel constants, `:157 const needsReconfirmingSentence`, `:168 func tooEarlySentence() string`, `:188 func reporterLabel(t ReporterTag) string`
- `internal/service/reliability.go:129: ReliabilityMinJudged = 5`; `ReporterNew` present in service
- `internal/api/handlers/chrome.go:155: func (c Chrome) withDefaults() Chrome`; `TelHref()` returns `template.URL`
- `internal/api/router.go:314: auth.Chrome = deps.Chrome`
- `internal/service/servercopy_test.go:39: func assertServerCopy`
- `internal/api/handlers/reports_e2e_test.go:55 newE2EServer`, `:63 newE2EServerWith`
- Phase 3.1 view names: `report.view === 'past'` in web/static/js/votes.js and the `past` tab in feed.js

## Cookie and host greps behind the Policy text

Cookie writers (`document.cookie|http.SetCookie|Set-Cookie|analytics`, non test files): only `internal/session/cookie.go:112` and `:166` (the session cookie, MaxAge one year, cleared on logout). No analytics. The sentence "There are no advertising or tracking cookies." was therefore written. Note: on this base there is no Low data view cookie, queue or Low data position yet (later Phase 4 plans add them); the Policy list names them as the plan and UI-SPEC require, so the owner check after wave 8 must compare the list with the finished code (human check 4).

Host greps for the third party sentence:
- `web/templates/index.html.tmpl:15: https://unpkg.com/leaflet@1.9.4/dist/leaflet.css` (also :18, :169)
- `web/static/js/map.js:266: 'https://tiles.openfreemap.org/styles/liberty'`
- `web/static/js/map.js:275: L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'`
- `internal/geocode/client.go:39: https://nominatim.openstreetmap.org/search`

Votes are stored at geohash precision 7 (`voterGeohashPrecision = 7` in internal/service/trust.go), about 150 metres, which matches the Policy figure.

## policyLastUpdated

`16 October 2026`.

## Verification

Database tests ran only on the disposable `p4_0412` (created by me, dropped, confirmed gone).

- Task 1 command (service, api, handlers, web): all nine named tests PASS, 0 SKIP.
- Task 2 command (no database): TestServerCopyModeration, TestDisclaimerGlossaryShape, TestDisclaimerGlossaryReadsTrustCopy, TestOlderNumbersLabel, TestLegalPagesRender, TestLegalPagesRenderNoAccountData, TestLegalPagesNumbersBlock, TestLegalPagesGlossaryUsesAppSentences, TestDisclaimerBarOnEveryFullPage, TestUserVisibleCopyUsesPlainPunctuation all PASS.
- Task 3 command with `DATABASE_URL=postgres://localhost:5432/p4_0412?sslmode=disable go test ./web/ ./internal/api/ ./internal/api/handlers/ -run '...' -count=1 -p 1 -v`: PASS for TestAccessGateExemptRoutes, TestLegalPagesRender, TestLegalPagesRenderNoAccountData, TestLegalPagesContactBlockOnlyWhenSet, TestLegalPagesNumbersBlock, TestLegalPagesGlossaryUsesAppSentences, TestLegalPagesPublicPrivateCache, TestLegalPagesTemplateSource, TestLegalPagesCSS, TestDisclaimerBarOnEveryFullPage, TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage, TestUserVisibleCopyUsesPlainPunctuation, TestPhase4HiddenGuard. 0 SKIP lines.
- Whole suite: `DATABASE_URL=... go test ./... -count=1 -p 1` all packages ok. `go build ./...`, `go vet ./...` and gofmt clean. No U+2013 or U+2014 in any new file or added line.

## Deviations from Plan

1. **[Rule 1 - test bug] Tests corrected while going green (Task 3 GREEN commit, files outside the GREEN list but inside files_modified).**
   - `legalText` strips tags to an empty string (not a space) so "write to a@b.org." reads as a sentence.
   - `TestLegalPagesPublicPrivateCache` uses a fresh cookie jar per page for the logged out half, because the session cookie is only set on a first visit.
   - The "light and lite" word check in `TestLegalPagesTemplateSource` covers class attributes and comments only: the plan's own Policy copy must say "Your Light or Dark choice".
2. The plan verify commands use `env -u DATABASE_URL`; the sandbox refuses that wrapper, so no database runs left the variable unset in the shell.
3. In the policy template the "Limits" section and the contact block avoid any deadline or time word; the tests check for "within".

## Known Stubs

None.

## Threat Flags

None beyond T-04-12 and T-04-18, both mitigated and covered by tests.

## For the orchestrator

- The bar's Disclaimer link no longer 404s.
- Browser harness checks: the two new full pages have no map or Activity JS, only theme.js; judge at 320, 360 and 390 px (human check in 04-27).
- Owner still to do before promotion: check the numbers on live government pages and get a lawyer or mentor read of both pages.

## Self-Check: PASSED

All created files exist and the six task commits plus this SUMMARY are on the worktree branch.
