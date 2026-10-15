---
phase: 04-robustness-real-world-resilience
plan: 21
subsystem: web-ui
tags: [login, account-menu, low-data-view, legal-links, a11y]
requires: [04-16, 04-18]
provides:
  - login page panel (Low data view, Disclaimer, Policy) in both login states
  - remembered choice set by /login?low=1 and cleared by /login?low=0
  - account menu items Low data view or Open full version, Disclaimer, Policy
  - account menu panel with a measured max height that scrolls inside itself
  - Activity back link that follows the device default (href exactly /low or /)
affects: [04-25 reads the rendered back link href, 04-27 manual checks]
tech-stack:
  added: []
  patterns: [server side panel state chosen by a bool, runtime CSS custom property with token fallback]
key-files:
  created:
    - web/ways_in_contract_test.go
    - internal/api/handlers/ways_in_e2e_test.go
  modified:
    - internal/api/handlers/auth.go
    - web/templates/login_gate.html.tmpl
    - web/templates/account_header.html.tmpl
    - web/templates/profile.html.tmpl
    - web/static/css/auth.css
    - web/static/js/account-menu.js
    - web/profile_nav_contract_test.go
key-decisions:
  - "Followed the plan's six recorded decisions unchanged (note follows the cookie, no-store on GET /login, links stack below 360px, flex-shrink 0 on menu children, aria-label for the item name, one bool keyed conditionals in the back link)"
requirements-completed: []
status: complete
---

# Phase 4 Plan 21: Ways in to the Low data view Summary

The Low data view is now reachable from the login page panel, the account menu and a remembered choice set by `/login?low=1`; the legal pages are linked from the login page and the menu; the open menu scrolls inside a measured height; and the Activity back link follows the device default.

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | 0a585bb | login panel, `low=1` and `low=0`, no-store, `lowDataChosen`, panel CSS, tests |
| 2 | 3bb3e82 | menu items, `measurePanel`, `onPanelFocusIn`, scrolling panel CSS, tests |
| 3 | 1424b96 | back link follows the device default, amended `TestActivityPageLinksBackToTheMap`, e2e test |

## Step 0 results (Task 1)

- (a) `low.go` has `LowDataCookie`, `setLowDataCookie(w, r)`, `clearLowDataCookie(w, r)` and `LowPage` setting `LowDataPage = true` on its own Chrome copy. `Secure` comes from `requestIsHTTPS`, which is true only for `r.TLS != nil` or an `X-Forwarded-Proto: https` header; the plain http test server sends neither, so the cookie jar returns the cookie and `GET /` gives 302. No reader of the cookie existed in package `handlers` (`page.go` reads it inline), and `lowDataChosen` was free, so I declared `func lowDataChosen(r *http.Request) bool` in `auth.go`.
- (b) `gate.go` sends a logged out `/low` to `/login?low=1` (`loginLowDataTarget`); `Page` answers a bare `/` with the cookie with 302 to `/low`.
- (c) `Chrome Chrome` fields, `Chrome: cfg.Chrome.withDefaults()` and `LowDataPage` all present; `login_gate.html.tmpl` opens with `.top-chrome` and the bar.
- (d) `.app-header`, `#account-menu:not([hidden])` (display flex, column, absolute), `--disclaimer-bar-height` in `main.css`, and the helpers `stripJSComments`, `parseCSSRules`, `ruleBySelector`, `declsOf`, `stripCSSComments`, `jsFunctionBody` all exist in package `web`.
- (e) `/disclaimer` and `/policy` are registered public routes in `router.go`.
- (f) `profileViewModel`, `Profile`, `newProfileReport` and the single back link exist.
- (g) The account menu held: email row, Activity, Log out form. No Phase 3.2 bell or menu item exists on this base.
- (h) `TestReadOnlyLabelsRoundedControlsNot` exists and stays green.
- (i) Census: tests matching the new markup strings were checked (`low_routes_test.go`, `legal_e2e_test.go`, `page_test.go`, `chrome_render_test.go`, `activity_moderation_e2e_test.go`, `activity_page_contract_test.go`, `slow_notice_contract_test.go`, `theme_contract_test.go`, `low_contract_test.go`); none breaks. **The census missed one test**: it grepped link strings and structure, not template action order. See Deviations.
- (j) No `p421` helper existed before this plan.

## Menu order as built

Email row, Activity, Low data view (stacked, second line "Text only, no map", accessible name "Low data view, Text only, no map", `href="/low"`) or on the Low data view Open full version (`href="/?full=1"`), Disclaimer, Policy, Log out last. No link to the moderator screen.

## Back link for plan 04-25

The rendered `href` is exactly `/low` when the device default is the Low data view and exactly `/` otherwise, with `aria-label` and `title` Back to Low data view or Back to map. `profile-back-link` appears once, icon only, before the email row.

## Login page scroll at 320x568

Not measured: no browser may be run in this execution. The panel sits below the card by design, so a scroll caused by it is expected. Left for the owner in 04-27 (V-24), as the plan's human check says.

## Deviations from Plan

**1. [Plan conflict, not fixed, needs the orchestrator] TestPhoneActivityPageMarkup now fails**
- **Found during:** Task 3, full `./web/` run.
- **Issue:** `web/phone_layout_contract_test.go` (not one of this plan's nine files) calls `p13Ordered`, which compares the position of the FIRST occurrence of each needle. It expects `{{end}}` after `{{.Counts}}`. The back link the plan mandates (decision 6, acceptance grep `href="{{if .LowDataDefault}}/low{{else}}/{{end}}"`) puts the first `{{end}}` in `profile.html.tmpl` at line 24, before `{{with .Record}}`. Any conditional in that anchor ends in `{{end}}`, so no form of the plan's markup avoids it. The failure message is `profile.html.tmpl: "{{end}}" must come after "{{.Counts}}"`.
- **Action taken:** none beyond the plan. Per the executor rules I did not edit the file outside `files_modified`, and I did not rework the back link to dodge a test.
- **Fix for the orchestrator:** in `TestPhoneActivityPageMarkup`, change the bare `{{end}}` needle in the `p13Ordered(t, profile, "profile.html.tmpl", ...)` call to one only the record block has, for example `"</section>\n      {{end}}"` (line 41 `      </section>` then line 42 `      {{end}}` in `profile.html.tmpl`; check the exact indentation before quoting it).
- **Commit with the effect:** 1424b96.

**2. [Process] One commit per task instead of separate RED and GREEN commits**
- The owner rule says one path limited commit per task, which overrides the plan's two commit TDD wording. RED was run and seen failing before each GREEN edit (login: panel and cookie assertions failed; menu: items, JS and CSS assertions failed; back link: href and name assertions failed). Tests and implementation are in the same commit per task.

**3. [Process] `env -u DATABASE_URL` wrapper not used**
- The sandbox refuses it. DATABASE_URL was unset in the shell, so no database test ran by accident; database runs set it inline on `p4_0421`.

## Verification

- `go vet ./...` and `gofmt -l` clean on the touched files; no en or em dash in any added line or the two test files.
- `go test ./... -count=1` (DATABASE_URL unset): every package ok except `pinalert/web`, whose only failure is `TestPhoneActivityPageMarkup` (deviation 1). Every test this plan names passes: `TestLoginLowDataPanel`, `TestLoginLowDataPanelStyles`, `TestAccountMenuItems`, `TestAccountMenuMeasuresHeight`, `TestAccountMenuRendersPerPage`, `TestActivityPageLinksBackToTheMap`, the six `TestAccountMenuJS*` guards, `TestAccountMenuHiddenGuard`, `TestExistingPagesRenderWithChrome`, `TestDisclaimerBarOnEveryFullPage`, `TestUserVisibleCopyUsesPlainPunctuation`, `TestNoMarkupSinksAnywhere`, `TestPhase4HiddenGuard`, `TestReadOnlyLabelsRoundedControlsNot`, `TestNoPillShapedControls`.
- `DATABASE_URL=postgres://localhost:5432/p4_0421?sslmode=disable go test ./internal/api/... ./web/ -p 1 -count=1 -v`: `internal/api` ok, `internal/api/handlers` ok (includes `TestLoginLowParamSetsCookie`, `TestActivityBackLinkFollowsDeviceDefault`, `TestLowPageRenders`, `TestLowDataRoutes`, `TestLoginGateServesInteractiveForm`), 0 `--- SKIP` lines; `web` fails only on deviation 1. Database `p4_0421` was created by me and dropped afterwards.

## Known Stubs

None.

## Threat Flags

None. The login panel and the cookie carry no account data (T-04-12), only the exact values 1 and 0 of `low` act and the value is never echoed (T-04-13), and `account-menu.js` writes only a CSS custom property built from a number with no markup sink (T-04-01).

## Self-Check: FAILED

Reason: all of this plan's own tests pass and all files and commits exist (0a585bb, 3bb3e82, 1424b96), but the full `./web/` package has one red test outside this plan's files, `TestPhoneActivityPageMarkup`, caused by the markup the plan mandates (deviation 1). The one line fix is given above and needs the orchestrator.
