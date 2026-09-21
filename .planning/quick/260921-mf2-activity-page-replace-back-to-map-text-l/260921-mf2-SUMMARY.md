---
phase: quick-260921-mf2
plan: 01
status: complete
subsystem: web-ui
tags: [css, template, icon, contract-test]
key-files:
  created:
    - web/static/icons/arrow-left.svg
  modified:
    - web/static/css/auth.css
    - web/templates/profile.html.tmpl
    - web/css_contract_test.go
    - web/profile_nav_contract_test.go
commits:
  - ded9441 feat: icon back control
  - c38f234 test: geometry lock
---

# Quick 260921-mf2: Activity page icon back control

The Activity page's underlined "Back to map" text link is now a 44px circular icon-only control (Lucide arrow-left, v1.41.0) matching the account button, on the same line, at the left edge of the content column.

## What changed

- `arrow-left.svg` fetched from `lucide-static@1.41.0`, given the same three local edits as the other icons (100% size, aria-hidden, focusable false). Read before committing: only `svg` and `path` elements.
- `auth.css`: new `.auth-icon--arrow-left` mask rule; `.profile-page` padding shorthand replaced by four longhands with `padding-top: var(--space-md)`; `.profile-back-link` rewritten (44px circle, border, background, shadow like `.account-trigger`, 150ms hover/press transition, reduced-motion block).
- `profile.html.tmpl`: icon-only anchor with `aria-label` and `title` "Back to map". The `<title>` dash was left alone (deferred polish item).
- `css_contract_test.go`: `arrow-left.svg` added to `nonCategoryIcons`.
- `profile_nav_contract_test.go`: `TestActivityPageLinksBackToTheMap` now asserts icon-only markup, the mask file resolves, sizing, transition band 120 to 180ms, both geometry locks, focus-ring dependency, reduced-motion rule.

## Geometry (computed, then measured)

Computed: both controls span y 16 to 60, center y 38. At 1440px the control spans x 424 to 468, the account button x 1380 to 1424. At 375px, x 24 to 68 and x 315 to 359. Email row top moves from y 128 to y 84.

Measured: a headless Chromium (Playwright, found in the local npx cache) loaded a static HTML page that copies the template's markup, styled by the real shipped CSS files, at 1440 and 375 wide in light and dark. Bounding boxes matched the computed values exactly (back y 16, h 44, cy 38; account button y 16, h 44, cy 38; email y 84; back left edge equals email and h2 left edge: 424 at 1440, 24 at 375). Icon glyph 24px, centered in the circle. Text decoration none. A 375px screenshot was looked at and looked right.

NOT confirmed: this was not the real `/profile` page. It was a static copy of the markup, without the server, database, login, or the page's JS. Hover, press and keyboard focus ring visuals were not exercised. Safari (the user's browser) was not tested. No browser render of the running app was done. To check by eye: rebuild and restart the dev server (CSS is compiled in via `//go:embed static`, and the asset version is the server start time, so a restart also busts the cache), then open `http://localhost:8090/profile`.

## Geometry lock proven live

Temporarily set `.profile-page` padding-top to `var(--space-lg)`: the test failed on the top-offset lock. Temporarily set `.profile-back-link` height to `var(--space-lg)`: the test failed on both the sizing check and the height-pair lock. Both reverted with `git checkout -- web/static/css/auth.css`.

## Tradeoffs and deliberate choices

- The control is in normal flow, so the two controls share a line at scroll top only. Once scrolled, the account button (fixed) stays and the back control scrolls away.
- The back control has hover and press states and `.account-trigger` does not. Requested asymmetry, not an oversight.

## Deviations from Plan

None, plan executed as written. One note: the existing `.profile-page` comment still says its top padding "clears the fixed-position header"; the plan said leave pre-existing comments untouched, so it is now slightly stale.

## Verification

`go vet ./...` clean. `go test ./... -short` passes, no failures. `DATABASE_URL` was unset; no database was touched and `pinalert_test` was never targeted. No em or en dashes in any added line. ROADMAP.md untouched.

## Self-Check: PASSED

Files exist, commits ded9441 and c38f234 exist, working tree clean after commits (SUMMARY is uncommitted by instruction).
