---
phase: 08-report-page-and-comments
plan: 04
subsystem: share
tags: [share, public-page, open-graph, session-skip, rate-limit, gate]
requires:
  - phase: 08-02
    provides: ShareService.PublicStatus, ValidShareToken, ShareFixedLine, PublicAgo, the Public* words
  - phase: 08-03
    provides: handlers.PublicShareURL, Deps.PublicBaseURL, Deps.Shares
  - phase: 05
    provides: the sessionSkip rule set (get, post, prefixes)
provides:
  - GET /r/{token}, the one Phase 8 route that opens without an account
  - handlers.PublicStatusPage and publicStatusViewModel
  - web/templates/share_status.html.tmpl (defines share_status) and web/static/css/public.css
  - "/r" in sessionSkipSet's prefixes
  - api.SharePageRateLimit, SharePageRateLimitDefault, Deps.SharePageRateLimit; main.go sharePageBurst and sharePageEvery
affects: [08-06, 08-19]
tech-stack:
  added: []
  patterns: [server rendered public page with no script and no cookie, limiter built only when CLIENT_IP_HEADER is set]
key-files:
  created:
    - internal/api/handlers/public_status.go
    - internal/api/handlers/public_status_e2e_test.go
    - web/templates/share_status.html.tmpl
    - web/static/css/public.css
    - web/public_page_contract_test.go
    - internal/service/share_timeline_test.go
    - internal/api/share_routes_test.go
  modified:
    - internal/api/router.go
    - cmd/server/main.go
    - internal/api/gate_test.go
    - internal/api/pwa_routes_test.go
    - web/chrome_contract_test.go
    - web/labels_contract_test.go
    - web/pwa_contract_test.go
    - web/theme_contract_test.go
key-decisions:
  - "The page defines its own template name share_status and is executed by that name, like the Demo pages are by file name; it links main.css, chrome.css (the button state cascade) and public.css."
  - "Status codes: 200 active, disputed, resolved, ended; 410 removed; 404 held, unknown, malformed, too old; 429 with Retry-After: 1 for a refusal."
  - "A phone sign in button is 48px (UI-SPEC spacing table), 44px on the laptop; the shared rule keeps every button at the 44px token."
requirements-completed: [COORD-08]
status: complete
duration: about 1 hour
completed: 2026-10-29
---

# Phase 8 Plan 04: The public status page Summary

A forwarded link /r/{token} opens a small server rendered page with no account, no script, no cookie and no sessions row, showing only the category and rough area, a neutral status word, the server's own sentence, severity and when, with Open Graph tags that carry no count and no claim.

## Commits

- 589d0d76 feat(08-04): add the public status page handler, template and styles
- 50868560 feat(08-04): serve GET /r/{token} outside the gate with a per IP limit and no session

## What was built

- `handlers.PublicStatusPage(tmpl, cfg, svc, base, allow)`: a refused request (allow false) renders the Too many requests page with 429 and `Retry-After: 1` before anything else; otherwise the chi `token` param goes to `ShareService.PublicStatus` (a service error logs "public status failed" with no token and answers a plain 500). The view model has no field for a description, coordinates, a distance, a username, comments or the trust facts. CanonicalURL is `PublicShareURL(base, token)` only for a token that passed `ValidShareToken`, otherwise base plus "/". Never reads r.Host. Every answer carries `text/html; charset=utf-8`, `Cache-Control: no-store`, `X-Robots-Tag: noindex, nofollow`, `Referrer-Policy: no-referrer` and no Set-Cookie.
- Template: lang en, viewport, robots noindex, title "h1 · Pinalert", og:title (the h1 text), og:description (`ShareFixedLine`), og:url, og:type website, og:image (base plus /icons/app-512.png, 8455 bytes). No script and no app_head. Wordmark row, h1, neutral chip, the detail box as a `dl` (Severity with the 8px dot, What is happening for a Critical report with a pick, Posted, Resolved or Ended), "Sign in to see more" as `btn btn--primary` to /login, the legal line with `Chrome.Numbers.Primary`, Disclaimer and Policy as outlined button links.
- `public.css`: top row 60px, laptop grid 260px and 1fr with a 32px gap from 900px, phone one column with 16px padding, 12px radius box, 24px chip with a 4px radius, tokens only, every display rule guarded with `:not([hidden])`.
- Router: `r.Get("/r/{token}", ...)` in the exempt block after /policy; `"/r"` added to `sessionSkipSet()` prefixes (the POST entry `/internal/official/refresh` kept); `SharePageRateLimit` (default burst 120, one more a second); the limiter is built only when `strings.TrimSpace(deps.ClientIPHeader) != ""` and keys on `middleware.GetClientIP`, admitting an unknown address; with no header every request is admitted. Dated paragraph added to the NewRouter doc comment. main.go: constants `sharePageBurst`, `sharePageEvery` and `Deps.SharePageRateLimit`.

## The exempt list as it now stands

GET /login, POST /login, GET /auth/verify (303 redirect), GET /disclaimer, GET /policy, GET /r/{token}, everything under /demo, the ten app asset paths, the 85 globe tile paths, /static/*, /swagger/*, the six POST /api/auth/ routes (sign-up, sign-up/code, code/resend, sign-in, reset, reset/code), POST /internal/official/refresh (own bearer token). Session skip set: GET paths (ten PWA plus 85 tiles), POST path /internal/official/refresh, prefixes /demo, /static, /r.

Phase 5's sessionSkip rule set existed and was used (no fallback needed).

## Tests

All run on the scratch database p8_04_a (dropped at the end).

- `go test ./... -p 1 -count=1 -v`: exit 0, 5873 `--- PASS`, 0 `--- FAIL`, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture only).
- `DATABASE_URL= go test ./... -short -count=1`: exit 0. `go build ./...` and `go vet ./...` clean.
- Each named test seen with `--- PASS`: TestSharePageShowsOnlyAllowedFields, TestSharePageStates (12 subtests), TestShareOpenGraph, TestSharePageRateLimited, TestSharePageNoLimitWithoutClientIPHeader, TestSharePageSetsNoCookie (3 agents), TestSharePageNearMisses, TestAccessGateExemptRoutes (new row and eight near misses), TestPWASkipSetMatchesRoutes, TestSkipSessionFor, TestPWAAssetsSetNoCookie, TestPublicPageHasNoScript, TestPublicPageControlsMeetTouchTarget, TestPublicEndedSentenceMatchesTimeline.
- Mutation check: removing "/r" from the prefixes made TestSharePageSetsNoCookie fail with a Set-Cookie and a new sessions row for all three agents; restored.
- HEAD /r/{token} as observed (T-08-85, accepted): 405, one Set-Cookie, one new sessions row. The test fails only if a HEAD returns the page body.

## Deviations from Plan

**1. [Rule 3 - Blocking] Four shared web contract tests amended with dated notes**
- **Found during:** Task 1, first run of `go test ./web/`.
- **Issue:** the plan says the plan edits no shared web contract list, but every file containing `<html` is held by shared tests to carry the theme script (TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage), the app head (TestEveryPageIncludesAppHead), an entry in the bar table (TestDisclaimerBarOnEveryFullPage) and, for round shapes, TestReadOnlyLabelsRoundedControlsNot. The plan and UI-SPEC require this page to carry no script and no app head (an app head would register the service worker for a reader with no account), so the page cannot satisfy them.
- **Fix:** the same pattern Phase 5 used for the Demo pages: one named file exempted in theme_contract_test.go and pwa_contract_test.go, a `"none"` row in chrome_contract_test.go, and the exact head `.public-dot:not([hidden])` exempted in labels_contract_test.go (the 8px severity dot is a drawing, like report-page.css's timeline dots). Each carries "Amended 2026-10-29 by Phase 8 plan 04". TestPublicPageHasNoScript pins that the page holds no script, no app head and no theme script.
- **Files:** web/theme_contract_test.go, web/pwa_contract_test.go, web/chrome_contract_test.go, web/labels_contract_test.go. **Commit:** 589d0d76.
- **Note for 08-06:** it also edits web tests; these four files hold small localized hunks that should merge cleanly.

**2. [Rule 3 - Blocking] chrome.css is linked as well as main.css and public.css**
- chrome_css_contract_test requires chrome.css on every full page (after main.css) and it holds the button state cascade `btn--primary` relies on.

**3. TestPWASkipSetMatchesRoutes** also keeps the Phase 7.1 POST entry assertion as written (the plan's own text predated it), as the orchestrator instructed, and adds that every prefix must cover a registered GET route.

**4. gofmt realigned the Deps literal in cmd/server/main.go** around the new field; no other formatting change.

**5. Housekeeping slip, no effect on the repo:** one stray empty file `scratch_edit.txt` was created in the main checkout root by a mistaken redirect and removed at once.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-08-14 to T-08-22 and T-08-85 are handled as written: token only (numeric and near miss paths tested non 200), /r prefix in the session skip with a counting test, html/template escaping with no script, og:url and og:image from configuration (test sends Host evil.example), no digit in og:title or og:description, no private fields (markers seeded and absent), limiter only with CLIENT_IP_HEADER, noindex and no-store, only removed named (410).

## Notes for later plans

- 08-06 and 08-19: the Share window's link already points at /r/{token}; the page is live. `docs/` is untouched (an HTML page, not in the API spec).
- A report with an unknown area reads `{Category} report`; the area appears only when `reports.area_name` was stored by a Share tap or a phone alert.

## Self-Check: PASSED

- Created files exist: public_status.go, public_status_e2e_test.go, share_status.html.tmpl, public.css, public_page_contract_test.go, share_timeline_test.go, share_routes_test.go.
- Commits 589d0d76 and 50868560 exist on the worktree branch.
- No U+2013 or U+2014 in added lines; the only non-ASCII added is the middle dot in the title.
