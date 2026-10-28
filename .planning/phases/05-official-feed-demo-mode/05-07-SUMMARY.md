---
phase: 05-official-feed-demo-mode
plan: 07
subsystem: ui
tags: [maplibre-gl, webgl2, nasa-gibs, login, html-template, contract-tests]

requires:
  - phase: 05-official-feed-demo-mode
    provides: "05-04: the 85 same origin /globe/tiles/{z}/{y}/{x}.jpg routes and the session skip rule set"
provides:
  - "A real turning NASA Blue Marble Earth behind the sign in card, drawn by login-globe.js from our own tile route"
  - "The sign in quiet row Try the demo, Disclaimer, Policy as its own partial with a reserved explainer video slot"
  - "Referrer-Policy same-origin on GET /login"
  - "Seven web contract tests, a handler test and 29 node cases pinning all of it"
affects: [05-10 Demo link target /demo, 05-20 Policy wording for the sign in page, phase 08 sign in card replacement, phase 07.1]

tech-stack:
  added: []
  patterns:
    - "Three template defines in one partial file (css in the head, markup after the card, deferred script after auth.js) so ordering rules hold and a Low data device gets nothing"
    - "Opacity dim as a custom property on one allowed wrapper rule, fade in as a keyframe on the inner element"
    - "Loader reads pinned library URLs and SRI from data attributes on its own script tag; the page never writes a maplibre tag"

key-files:
  created:
    - web/static/js/login-globe.js
    - web/static/css/login-globe.css
    - web/templates/login_globe.html.tmpl
    - web/templates/login_ways_links.html.tmpl
    - web/login_globe_contract_test.go
    - web/login_globe_test.mjs
    - internal/api/handlers/login_referrer_test.go
  modified:
    - web/templates/login_gate.html.tmpl
    - web/templates/verify_outcome.html.tmpl
    - web/static/css/auth.css
    - internal/api/handlers/auth.go
    - web/layout_rules_contract_test.go
    - web/low_contract_test.go
    - web/low_budget_contract_test.go
    - web/ways_in_contract_test.go
    - web/reader_contract_test.go
    - internal/api/handlers/ways_in_e2e_test.go
    - docs/MODERATION.md
  deleted:
    - web/static/img/globe.svg

key-decisions:
  - "The globe partial holds three defines (login_globe_css in the head, login_globe after .login-column, login_globe_script after auth.js), because one include inside #login-gate would put a deferred script ahead of auth.js and a body stylesheet could shift the card"
  - "The credit line is written in the template, not built with createElement: it holds its space from the first paint and has no DOM sink at all (stricter for T-05-38)"
  - "A lost WebGL context gives the globe up (plan and UI-SPEC 5c) instead of resuming it as sketch item 7 did"
  - "No has-globe toggling from script: the class is written by the template only when Low data is off, so no layout change happens after first paint"

patterns-established:
  - "Guarded loader: load, then idle, then probe, then download, then build; every failure path calls one giveUp that keeps the space"
  - "Fake timer and fake animation frame node harness for a WebGL loader (web/login_globe_test.mjs)"

requirements-completed: [ROBUST-07]

duration: ~75 min
completed: 2026-10-29
status: complete
---

# Phase 5 Plan 07: The sign in globe and Try the demo row Summary

**A guarded maplibre-gl globe turns NASA Blue Marble pictures from Pinalert's own tile route behind the sign in card, and Try the demo leads the quiet row, both in partials Phase 8 can keep.**

## Performance

- **Tasks:** 3 of 3 (RED commit, build commit, row spacing commit)
- **Files:** 7 created, 11 modified, 1 deleted

## Accomplishments

- `login-globe.js` runs after window load and idle, asks for WebGL2 before any download, injects the pinned maplibre-gl 5.24.0 css and js (SRI, crossorigin anonymous, no-referrer), builds exactly one map (globe projection, one raster source capped at zoom 3 from `/globe/tiles/`, no interaction, no attribution control, India at 80, 18), and turns it 360 degrees per 120 seconds west to east at about 30 frames a second. Reduce motion draws one still frame and is followed live. A hidden tab pauses the loop and the 10 second give up count. A library error, a map error before the first frame, a lost context or nothing drawn in 10 seconds calls one `giveUp`, which removes the map and keeps the empty space and the hidden credit.
- Low data view writes no globe markup, stylesheet or script (`{{if not .LowDataDefault}}` on all three includes). Measured: zero requests to unpkg.com and to /globe/tiles, no `login-globe` request.
- The flat picture globe is gone: `globe.svg`, the mask, its keyframes and the markup in `login_gate` and `verify_outcome`. `auth.css` holds no "globe".
- `login_ways_links.html.tmpl`: Try the demo (`#login-try-demo`, `/demo`), Disclaimer, Policy, a template comment reserving the explainer video slot. Gap 8px both ways.
- `LoginGate` sends `Referrer-Policy: same-origin`.

## Task Commits

1. Task 1 RED: `8479e4fc` test(05-07) failing pins. Before the build the 7 web tests failed (7 `--- FAIL` lines) and the node file failed to load login-globe.js.
2. Task 2 build: `46e407af` feat(05-07) the globe, loader, stylesheet, both partials, the header and the amended pins (the row partial and the header landed here, not in Task 3, because the amended pins and the login template cannot build without them).
3. Task 3: `1ed0ef86` style(05-07) 8px row gaps, after the browser check.

## Amended pins (each carries a dated note, "Amended 2026-10-29 by plan 05-07")

| File | Test or place | Change |
|------|---------------|--------|
| web/layout_rules_contract_test.go | `opacityAllowed` / TestNoOpacityText | entry moved from auth.css `.globe-bg` (0.35 mask picture) to login-globe.css `.globe-bg` (0.5 light, 0.4 dark via `--globe-dim`), reason mentions NASA |
| web/low_contract_test.go | Low page url() walk | the `.globe-bg__mask` exemption removed (the mask no longer exists) |
| web/low_budget_contract_test.go | `p427CSSURLTargets`, TestP427HelpersSelfCheck | the inert exception and its fixture row removed |
| web/ways_in_contract_test.go | TestLoginLowDataPanelStyles | panel must include the partial; the links are read from the partial text; every word kept |
| web/reader_contract_test.go | TestReaderOpeners | login links read from login_gate plus the partial |
| internal/api/handlers/ways_in_e2e_test.go | p421OffText, p421OnText, link lists | gain Try the demo and /demo ahead of Disclaimer and Policy |
| docs/MODERATION.md | Low data size budget paragraph | the "one inert globe rule excluded" clause removed with a dated note |

No assertion was weakened: every earlier word, link and order is still asserted.

## Browser check (Playwright, Chromium with SwiftShader WebGL2 and WebKit with WebGL2, scratch server on 8791)

| Check | Result |
|-------|--------|
| Form usable before the globe | every run: heading, field and Send visible at first paint, Send enabled after typing |
| Card box first paint vs +12 s, 390x844 and 1470x830, light and dark, both engines | identical in all 8 runs; layout-shift sum 0 in Chromium and WebKit |
| Globe appears | `is-ready` on all 8 runs, one canvas, no page errors; dim 0.5 light, 0.4 dark |
| Row | Try the demo, Disclaimer, Policy, each 44px tall, text-decoration none; stacks below 360px, wraps at 390 |
| Low data (`/login?low=1`) | 0 requests to unpkg.com, 0 to /globe/tiles, 0 to login-globe; no globe, credit or script in the DOM; panel words as before; same at both sizes, both engines |
| No WebGL2 (probe forced null) | 0 unpkg and 0 tile requests, card unmoved, credit visibility hidden with its 44px space kept, never ready |
| Reduce motion | canvas pixels unchanged over 3 s in 4 of 4 runs after the tiles settle; the one early failure (Chromium 390, screenshot 0.8 s after the first frame) settled to a stable hash within 1.5 s |
| Turning | pixels change over 3 s in all 4 normal runs |
| Credit placement | corner at 1470x830, 1280x700, 1000x900, 1470x600; in flow under the Low data panel at 1000x700, 359x700, 320x640; no overlap, no horizontal overflow at 320 |

The orchestrator still owes the iPhone simulator and Mac Safari look. Sphere sizing note: the real page has no mockup strip, so the sketch formula gives a 828px sphere at 1440x900 and 776px at 390x844 (the plan's 768px at 1440x900 came from the sketch's 65px strip).

## Full suite

`go test ./... -p 1 -count=1 -v` on disposable `p5_07` (created, migrated to 00011, dropped): 0 FAIL lines, 3 SKIP lines (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed), 1631 PASS lines, TestCastVoteConcurrentSameAccountKeepsEveryRow did not flake. `go vet ./...` clean; `node --test web/*.mjs` 923 pass, 0 fail (29 of them the new file); `gofmt -l` lists only the pre-existing internal/api/handlers/place_test.go. The scratch server on 8791 was stopped and p5_07 dropped.

## Deviations from Plan

**1. [Rule 3 - Blocking] The globe is three template defines, not one.** The plan put the deferred tags in the markup partial; a script tag inside #login-gate would sit before auth.js and break its own rule that auth.js runs first, and a stylesheet in the body could let the card paint before the credit's space exists. `login_globe.html.tmpl` therefore defines `login_globe_css` (head), `login_globe` (after .login-column) and `login_globe_script` (after auth.js), all under `{{if not .LowDataDefault}}`. Acceptance greps still hold (one `template "login_globe"`, one tile address, one SRI each).

**2. [Rule 2 - hardening] The credit is written in the template, not built by script.** The plan said createElement and textContent; writing it with the page reserves its space from the first paint and leaves no DOM sink (T-05-38). The two NASA links are asserted by TestLoginGlobePartialOrder.

**3. [Rule 3] Two extra test files edited.** `web/reader_contract_test.go` and `internal/api/handlers/ways_in_e2e_test.go` pinned the two link row and needed dated amendments; a new `internal/api/handlers/login_referrer_test.go` serves the real page, because `package web` cannot import `handlers` (import cycle), so `TestLoginRefererPolicy` in the web file pins the source and the handler test checks the header, the Low data absence and one Try the demo on the served page.

**4. Ordering.** The row partial, `LoginGate` header and the ways in amendments were committed with the Task 2 build commit because the amended pins and the login template cannot build without them; Task 3 then held the row gap change and the browser check.

**5. Lost WebGL context gives the globe up** (plan and UI-SPEC 5c) rather than resuming it as sketch item 7 suggested.

**6. Give up count restarts rather than pauses.** A hidden tab skips the 10 second check and showing the tab starts a fresh 10 seconds; the plan's "time while hidden not counted" is met in the safe direction.

## Known Stubs

None. The explainer video slot is a template comment by the plan's design (not drawn); the Demo itself (`/demo`) is built by later plans of this phase, so the link 404s until 05-10 lands.

## Threat Flags

None beyond the plan's register. T-05-34 (SRI and version pins, TestLoginGlobeTags), T-05-35 (header, no-referrer, same origin tiles), T-05-36 (browser check: card never moves), T-05-37 (no location, no data route call), T-05-38 (no DOM sink) and T-05-39 (Low data, probe, give up) are mitigated and pinned.

## Self-Check: PASSED

Files found: login-globe.js, login-globe.css, login_globe.html.tmpl, login_ways_links.html.tmpl, login_globe_contract_test.go, login_globe_test.mjs, login_referrer_test.go; `web/static/img/globe.svg` absent. Commits found: 8479e4fc, 46e407af, 1ed0ef86.
