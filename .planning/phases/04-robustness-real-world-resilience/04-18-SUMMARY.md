---
phase: 04-robustness-real-world-resilience
plan: 18
subsystem: low-data-view
tags: [low-data, gate, cookie, compression, robustness]
requires: [04-06, 04-08, 04-13, 04-15]
provides:
  - GET /low behind the access gate, handlers.LowPage, LowDataPath, LowDataCookie
  - the pinalert_low_data cookie, the bare / redirect to /low and the /?full=1 clear
  - low.html.tmpl, low.js, low.css (Active list from the stored position, stale line)
  - router wide gzip for HTML, CSS, JavaScript, JSON and SVG
key-files:
  created:
    - internal/api/handlers/low.go
    - internal/api/low_routes_test.go
    - web/templates/low.html.tmpl
    - web/static/js/low.js
    - web/static/css/low.css
    - web/low_contract_test.go
  modified:
    - internal/api/handlers/page.go
    - internal/api/gate.go
    - internal/api/router.go
    - web/js_contract_test.go
    - web/votes_contract_test.go
key-decisions:
  - "The Low data page passes the own flag to Pinalert.setCenter (true for the stored and the fresh position, false for the default area), because app.js now takes a third argument"
  - "No bell exists, so the title row clears only the account trigger: padding-right calc(var(--touch-target-min) + var(--space-md))"
duration: about 1.5 hours
completed: 2026-10-16
status: complete
requirements: [ROBUST-01]
---

# Phase 4 Plan 18: Read the Low data view Summary

A signed in person can open `/low`, a text only page with no map library, no image and no web font that lists Active reports at once from the last known position stored on the device, remembers the choice per device with a cookie, and every HTML, CSS, JavaScript, JSON and SVG response now travels gzip compressed.

BASE: `3d7f1f80216c89c093e6be47b44fdd6b8f18f6a7`

## Commits

- 1bf48ba feat(04-18): serve the Low data view behind the gate with a remembered choice (Task 1)
- 0287c54 feat(04-18): list Active reports on the Low data view from the stored position (Task 2)
- ff2678c feat(04-18): compress HTML, CSS, JavaScript, JSON and SVG responses (Task 3)
- The SUMMARY commit follows.

## What was built

- **Route and memory** (`low.go`, `page.go`, `gate.go`, `router.go`): `LowPage` copies the shape of `Page` (account guard, then 500 before any cookie), sets `Chrome.LowDataPage = true` on its local copy, sets the cookie, answers `no-store`. A logged out `/low` goes to `/login?low=1` (constant `loginLowDataTarget`), every other path to exactly `/login`. `Page` checks `full=1` first (clear and 302 to `/`), then a bare `/` (empty `RawQuery`) with cookie value `1` (302 to `/low`). Any other query string is never redirected. All targets are constants (T-04-13).
- **Cookie**: `pinalert_low_data=1`, `Path=/`, one year, `SameSite=Lax`, `Secure` when `r.TLS != nil` or `X-Forwarded-Proto` is https (case insensitive), not HttpOnly. Clearing uses `MaxAge: -1`.
- **Page** (`low.html.tmpl`): a full page with the bar and account header inside `.top-chrome`, `theme.js` in the head, `chrome.css` before `low.css`, the `#notice-live` announcer, `main.low-page#app-shell` with the six data attributes, the head block, the basis line, the stale block, the list, skeleton, empty and error panels. Scripts: app, votes, visibility, trust, low, account-menu.
- **Client** (`low.js`): stored position first (`localStorage`, guarded by try and catch), then the fresh one after an 8 second wait; the five basis sentences; rows from `createLowRow` (severity label, title, meta, state line, trust block from `createTrustBlock`), no tabindex and no listener; `updateLowRow`; `renderLowRows` with focus handling; `renderStale` with one announcement on entering and one on leaving; own `listChain` so a slow stored position answer can never land after the fresh one. It sends no request itself.
- **Styles** (`low.css`): the 640px column with the P4-A22 bottom padding, head, basis, stale line, list reset, `cursor: default` on the row, and the four id hidden override on one line. Tokens only.
- **Compression** (`router.go`): `compressionLevel = 5`, `compressibleContentTypes` (text/plain deliberately absent), `middleware.Compress` directly after the request logger and before the session middleware. No new module.

## Step 0 findings (re-grep of the built code)

- Every Phase 3 and Phase 4 symbol listed in the plan was present: `PinalertTrust` with `createTrustBlock()`, `updateTrustBlock`, `fadeStage`, `focusToggle`, `createSeverityLabel`, `updateSeverityLabel`, `createStateLine`; `formatShelterLine`, `SEVERITY_WORDS` and `updatedAgo` in `app.js`; `hasLoaded`, `lastUpdatedAt`; the four stale literals in `feed.js`; `#notice-live` in `index.html.tmpl`; `page := deps.Page` with `page.Chrome = deps.Chrome` in `router.go`; `.btn[hidden]` and `.inline-note` in `chrome.css`; `stripJSComments`; `TestPhase4HiddenGuard`, `TestTopChromeGeometry`, `TestDisclaimerBarOnEveryFullPage`; the gate test helpers.
- **Built row shape mirrored**: the main list `createRow(id)` is the compact 3.1 row (state line, meta, away, far, title, trust line, cue; tabindex, click, keydown). `createLowRow` does not copy it. It follows the Phase 3 shape: badge with `icon-glyph`, `report-row__body` holding the severity label, title, meta, the state line (visibility tag plus the reporter words) and the trust block, mounted as the Activity page mounts it. `updateLowRow` mirrors `updateRow` minus the away and far facts and the selection highlight.
- **Removal focus rules mirrored**: when a poll removes the row that held focus, focus goes to the trust toggle of the row now at that index, else of the row before, else `#low-report` when present, else `#low-full`. The feed's map and tab link fallbacks do not apply (no map, no tab strip). Moves use `moveBefore` where the browser has it and the focused element is focused again afterwards, as in `renderRows`.
- **`focusToggle` signature**: `focusToggle(blockOrRow)`, accepting a block instance or an element holding one, returning a boolean. `row.trust` and the row element both work.
- **Active view**: `app.js` starts with `state.view = 'active'`, adds `&view=` only for disputed and past, and never reads the address. No setter call was needed in `low.js`.
- **`setCenter` changed shape**: it now takes a third `own` argument (feed distance is shown only for the reader's own position). `showListFor(lat, lon, own)` passes `true` for the stored and fresh positions and `false` for the default area. The plan's literal `Pinalert.setCenter(lat, lon)` was therefore written as `Pinalert.setCenter(lat, lon, own)` and the contract test pins that form.
- **Bell**: none exists (the `bell` grep hits are `aria-labelledby` and a comment in `activity.css`). The account header partial reads only `.Email`. So `lowViewModel` needed no field beyond the plan's list, and `.low-page__head h1` gets `padding-right: calc(var(--touch-target-min) + var(--space-md))` (clearing the account trigger alone). The test constant `p418TitlePaddingRight` records this provenance in a comment.
- **Floors in `web/low_contract_test.go`** (so no test passes vacuously): at least 6 linked stylesheets and 6 body scripts (this plan's own lists), at least one counted `url(` target in the linked stylesheets (chrome.css, trust.css and others carry icon masks), at least 10 parsed rules in `low.css` (the file has 13).
- **`node --check web/static/js/low.js`**: ran, exit 0. I also ran low.js once against hand written stubs (not committed) to watch the boot, the first load, the stale enter and leave, the empty state and the fresh position fix: all behaved as specified.

## Deviations from Plan

**1. [Blocking item for the orchestrator, no file outside scope edited] `TestDisclaimerBarOnEveryFullPage` fails on `low.html.tmpl`.**
- `web/chrome_contract_test.go` (04-06, not in this plan's `files_modified`) sets `hasAccount` only for `index.html.tmpl` and `profile.html.tmpl`; any other full page that calls `account_header.html.tmpl` fails with "this page has no account and must not call the account header". The Low data page must call the header (D-21, it shows the email and the account menu), and the plan's verify lists this test.
- Following the rules file I did not edit that file. I confirmed the exact fix once by a temporary edit and then restored the file with `git checkout -- web/chrome_contract_test.go`: changing the line to `hasAccount := strings.HasSuffix(path, "index.html.tmpl") || strings.HasSuffix(path, "profile.html.tmpl") || strings.HasSuffix(path, "low.html.tmpl")` makes the test pass, and nothing else in that test changes.
- **Orchestrator action**: apply that one line after merging this branch (or give it to a plan that owns the file). Until then the only failing test in the whole tree is `TestDisclaimerBarOnEveryFullPage/templates/low.html.tmpl`.

**2. [Step 0 finding] `setCenter` gets the third argument** (see Step 0 findings). The test pins `Pinalert.setCenter(lat, lon, own)` in `showListFor`.

**3. [Leniency] Later script order.** `TestLowDataAssetPolicy` requires queue.js, flag.js and report-form.js, if present, to sit between trust.js and low.js, but does not force an order among those three (the plan said "in that order" without naming one the later plans could rely on).

Tests I wrote in `low_routes_test.go` and `low_contract_test.go` use rune numbers (0x2014 and so on) instead of the dash characters, so no dash is in any file I added.

## Interim states (wave 5 to wave 6, as the plan lists)

- The empty state copy says "use Report an emergency above"; the button arrives with 04-19.
- `/login?low=1` shows the ordinary login page until 04-21 makes it set the cookie and show the "is on" note; the cookie is already set when `/low` is served.
- Rows carry no vote block, Written line, queue strip or Flag control until 04-19, 04-22 and 04-24.
- The account menu on the Low data view still reads Activity and Log out only; 04-21 adds the Low data and legal items and reads `Chrome.LowDataPage`.
- The Low data page is not linked from anywhere yet except the slow notice on the main page, which already points at `/low`.
- No byte budget figure was set and there is no `TestLowDataByteBudget` (04-27 owns it).

## Verification run

- `DATABASE_URL= go test ./... -count=1`: every package ok except `pinalert/web`, whose only failure is deviation 1.
- Database run on `p4_0418` (created, used, dropped): `DATABASE_URL=postgres://localhost:5432/p4_0418?sslmode=disable go test ./internal/api/... ./web/ -count=1 -p 1 -v`: `internal/api` ok, `internal/api/handlers` ok, `web` only deviation 1; 437 top level PASS lines, 0 SKIP lines.
- Task 1 database tests (`TestLowDataRoutes`, `TestLowDataRoutesCookieRedirect`, `TestAccessGate*`) all pass with 0 skips.
- Task 2 set (`TestLowData*`, `TestBothSurfacesMountTheSameVisibilityTag`, `TestIconGlyphsAreClassDriven`, `TestNoMarkupSinksAnywhere`, `TestPhase4HiddenGuard`, `TestVoteBlockHiddenGuard`, `TestTopChromeGeometry`, `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`, `TestUserVisibleCopyUsesPlainPunctuation`): all pass.
- Task 3: `TestCompressionMiddleware` and `TestLowPageRenders` pass. `go.mod` and `go.sum` untouched. `gofmt -l` is clean.
- `git diff --name-only BASE..HEAD` lists exactly the 11 `files_modified` of the plan.

## Known Stubs

None. Every control on the page is wired; the missing report button, vote block, queue strip and Flag control are the interim states above, owned by named later plans.

## Threat Flags

None beyond the plan's register (T-04-01 and T-04-13 are mitigated as written).

## Self-Check: FAILED

One plan verification item does not pass on this branch alone: `TestDisclaimerBarOnEveryFullPage` fails for `templates/low.html.tmpl` because `web/chrome_contract_test.go` (outside this plan's `files_modified`) hardcodes which pages may call the account header; the one line fix is in Deviations item 1 and was proven by a temporary edit that I reverted. Everything else checks out: the three task commits exist (1bf48ba, 0287c54, ff2678c), all created files exist, and the other tests in the plan's verify lists pass with 0 skips.
