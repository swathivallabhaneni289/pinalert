---
phase: 05-official-feed-demo-mode
plan: 22
subsystem: official-feed
tags: [gdacs, map, pins, official-alerts, accessibility, leaflet]
status: complete
requirements: [ROBUST-05, ROBUST-06, ALERT-04]

requires:
  - phase: 05-official-feed-demo-mode (05-21, 05-09, 05-19)
    provides: GET /api/official/pins (gated, fresh only), hazards_state fields on the alerts answer, the Official alerts renderer, the Demo pins with a dashed ring
provides:
  - web/static/js/official-pins.js, the GDACS layer (own layer group, flat rectangular pin, read only card)
  - web/static/css/official-pins.css, the pin, the card and the 44px outlined link
  - PinalertMap.addOverlay(layer) and PinalertMap.openCard(latlng, node, name, opts) in map.js
  - the second muted status line on Official alerts (hazard events from GDACS)
  - docs/OFFICIAL-ALERTS.md section "GDACS hazard pins"
affects: [Phase 5 verification, the owner's viewing session, the orchestrator's iPhone simulator check]

key-files:
  created:
    - web/static/js/official-pins.js
    - web/static/css/official-pins.css
    - web/official_pins_contract_test.go
    - web/official_pins_test.mjs
  modified:
    - web/static/js/map.js
    - web/static/js/official-alerts.js
    - web/static/css/official.css
    - web/templates/index.html.tmpl
    - web/templates/alerts.html.tmpl
    - web/official_alerts_test.mjs
    - docs/OFFICIAL-ALERTS.md

key-decisions:
  - "The GDACS card opens through a new openCard seam in map.js that has its own open and close path. onPopupOpen and onPopupClose hand an official popup to it with one guard each, so no report address, report link, selection or list focus path ever sees it."
  - "The pin is two layers: a transparent 96 by 44 marker box (the touch target, passed to Leaflet because Leaflet writes the icon size inline) and a 28px rectangle inside it. Contract tests read the inner rule for shape and the box rule for size."
  - "A switch from an open crowd report to a pin ends the report's address in place (replace) instead of letting its queued step back in the browser run, because that step's popstate would close the new card."
  - "The hazard status line is optional in official-alerts.js, so the Demo's alerts page (no such element) runs unchanged and never draws a GDACS freshness notice."
  - "Keep in view and tools row clearance for the card run through onMapMoveEnd, which already does both for the open popup, so the existing tests that count panPopupIntoView and keepCardClearOfTools call sites hold without an amendment."

metrics:
  tasks: 3
  commits: 5 (3 tasks, 1 fix found in the browser, 1 summary)
  completed: 2026-10-29
---

# Phase 5 Plan 22: GDACS hazard pins Summary

Current GDACS hazards now show on the Nearby map as flat rectangular official pins (the word GDACS and the hazard glyph, no severity colour) that open a read only card in the same container as a crowd report, and Official alerts says when hazard events were last checked.

## What was built

**Task 1, failing tests first** (`f332a763`). `web/official_pins_contract_test.go` (TestOfficialPinDistinctFromCrowdAndDemo, TestOfficialPinCard, TestOfficialPinsNeverCounted, TestOfficialPinsScriptSinks, plus TestOfficialPinsAreLinked), `web/official_pins_test.mjs` (21 cases, 22 after the fix below) and 12 hazard line cases in `web/official_alerts_test.mjs`.

RED, run once before Task 2: Go, 5 failing tests (the four named and TestOfficialPinsAreLinked); `official_pins_test.mjs` could not load (no `official-pins.js`); `official_alerts_test.mjs` 48 tests, 38 pass, 10 fail (10 of the 12 new cases fail; two, the unknown hazard word and the markup case, pass trivially because nothing is drawn yet).

**Task 2, the layer** (`889f7fda`).
- `official-pins.js`: fetches only `<base>/official/pins` (one request on load, a timer every 5 minutes, and a `pinalert:feed-centre` event at most every 5 minutes, one at a time). Only a response whose state is exactly `fresh` draws; stale, unavailable, a failed request, an HTTP error and a wrongly shaped answer clear the layer. A pin of an unknown kind, level or place is skipped. Hazard word, glyph and level word come from tables in the script, never from feed text. Every string goes through `textContent`; `safeGdacsLink` keeps only `https://www.gdacs.org/` links of printable ASCII. A redraw keeps an unchanged marker (so focus is kept) and removes one that left.
- `official-pins.css`: `.official-pin` 28px high, 4px radius, `--color-text` fill with `--color-bg` glyph and word, no severity colour, no ring; `.official-pin-box` at least 44px; `.official-pin.is-chosen` scale(1.35) with no ring, no transition under reduced motion; the card header band, badge, level chip (extreme, severe, neutral tints), title, lines, honesty line, credit, and `.official-card__link` as a 44px outlined link with no underline. Forced colours handled.
- `map.js`: `addOverlay(layer)` and `openCard(latlng, node, name, opts)`. The card is an `L.popup` in the popup pane on a laptop and the `detailPane` on a phone, with the same close button, Escape (trust.js finds it by `data-popup-open`) and the pin's `data-popup-open` and focus return. `opts.opener` is the pin element, `opts.onClose` releases the chosen look.
- `official-alerts.js` and `alerts.html.tmpl`: `p.official-status.official-status--hazards#official-hazard-status`, drawn from `hazards_state`, `hazards_age_seconds` and `hazards_last_success_at` with the three sentences, ticking once a minute with the first line, cleared while loading and after a failed load. `official.css` gets one small padding rule for it.
- `index.html.tmpl`: the stylesheet after `phone.css`, the script after `map.js`. The Demo template loads neither (TestOfficialPinsAreLinked).

**Fix found in the browser** (`5d4d1992`). Leaflet makes a marker's element only when the map already has a view. On a slow first location fix the pins were drawn with no accessible name and no Enter or Space key (the first browser run showed three pins with no `aria-label`). The layer now names and keys the element on the marker's own `add` event as well, once per element. A node case pins it (`a marker whose element appears later still gets its name and its keys`).

**Task 3, docs and browser check** (`8bc1f2da`). `## GDACS hazard pins` in `docs/OFFICIAL-ALERTS.md`, the two "not built" lines replaced, and the live timeout of 2026-10-29 stated plainly together with the quick fix that has since landed on main (40 s per request, the two requests and the feeds run at the same time; seen in `git log main`, commit `4bc7448e`).

## Browser check

Scratch server: a throwaway Go test serving the real `api.NewRouter` on 127.0.0.1:8794 over p5_22 (literal throwaway session secret, an in memory fake mailer from the api package's gate tests, so no mail was sent and no mailer key was invented; sign in through the real link flow), with `http.DefaultTransport` replaced by a transport that refuses and counts every outbound request. Seeded with psql: fresh gdacs and sachet status, three hazards (a Red earthquake near Vijayawada, an Orange cyclone over the Bay of Bengal, a Green flood in Assam, the last with a non allowed `http://` link) and four crowd reports around the reader. Playwright, geolocation Vijayawada (16.5062, 80.6480), WebKit and Chromium, 390x844 and 1470x830, light and dark, plus a reduced motion run (light, both engines, both sizes). Screenshots are in the scratch folder only. No live GDACS or SACHET call was made: the blocked transport recorded one refused request to www.gdacs.org, started by the lazy kick after I set the feed to 2 hours ago; it never left the process.

All 8 combinations (engine x size x theme) gave the same result unless a column says otherwise.

| Check | Laptop 1470x830 (both engines, both themes) | Phone 390x844 (both engines, both themes) |
|-------|---------------------------------------------|-------------------------------------------|
| Pins drawn | 3 GDACS rectangles, 90.1 x 28 px in a 96 x 44 box, radius 4px, fill rgb(26,29,33) light and the text colour in dark, word GDACS | same |
| Crowd pins beside them | 4 round badges, 44 x 44, radius 50% | 4 round dots, 26 x 26, radius 50% |
| Nearby list and count with and without the layer | 4 rows, "4 reports, 10 km" both, identical | identical |
| Enter on a focused pin | card opens in the popup, focus on the card, `data-popup-open` on popup, page and pin | card opens in the detail sheet, heading Report focused |
| Card content | GDACS badge, glyph, chip Red, title, Hazard, Issued 3 hours ago, Until 10:16 am, Issued by GDACS, severity text, honesty line, credit, Open GDACS page | same |
| Link | `_blank`, `noopener noreferrer`, 44 px high, no underline | same |
| Vote, trust, Flag, Mute, buttons in card | 0 | 0 |
| Escape | closes, focus back on the pin, root mark and chosen look removed | closes, root mark and chosen look removed, focus on the page body (identical to a crowd report on a phone, checked by `focus.js`) |
| Close button | closes | closes (Back) |
| Space on the far cyclone | card opens, map panned so the card is in view | card opens in the sheet, in view |
| Green flood with a disallowed link | card with no button | same |
| Chosen pin | transform matrix(1.35,...), transition 0.2s, no outline | same |
| Reduced motion (light) | transition 0s, still scale 1.35 | same |
| Crowd report then pin | report closes, card opens, address `?report=` removed in place, card stays open after the queued settle | same |
| Pin then crowd report | card closes, report opens with `?report=2` | same |
| Page errors | none | none |

Demo (`/demo`, no cookie, 4 engine and size combinations): 18 round Demo pins, each with a `2px dashed` ring and radius 50%, 0 GDACS pins, no request to `/official/pins`, the pins script not even present. Official alerts, fresh: "Last checked 6 minutes ago" and under it "Hazard events from GDACS: last checked 7 minutes ago", both in the muted colour rgb(107,114,128), 4 combinations. Stale case (gdacs set to 2 hours ago): the pins route answers `state: stale` with 0 pins, Nearby draws 0 GDACS pins and still 4 crowd pins, and Official alerts reads "Hazard events from GDACS could not be checked since 3:21 am." with the SACHET line unchanged.

Observation, not a defect: in the seeded run Official alerts listed no GDACS row for the reader. The seeded earthquake lies in district 510, next to the reader's district 749 (checked with `areas.Locate`), so the 05-21 row rule correctly left it out.

## Verification

- `node --test web/*.mjs`: 1082 tests, 1082 pass, 0 fail (1048 at 05-21 plus 34 new).
- Full Go run on the disposable database p5_22 (`go test ./... -p 1 -count=1 -v`): 0 `--- FAIL`, 0 `^FAIL`, 0 `panic`, 5251 `--- PASS` (5246 at 05-21 plus the 5 new), 20 packages `ok`. Skips: TestGeocodeLive, TestOfficialLive, TestScreeningFixture only. No known flake fired, so no rerun. `go vet ./...` and `go build ./...` clean. The throwaway server test file and a one point probe test were deleted before the commit; `git status` was clean.
- p5_22 was dropped (`psql -lqt` shows none) and nothing listens on 8794.
- No em or en dash on any line I added (perl check on every file I touched, plus the added lines of map.js, official.css and official-alerts.js, which hold older dashes of their own).

## Deviations from Plan

### Auto-fixed

**1. [Rule 1 - Bug] Pins drawn before the map had a view had no name and no keys**
- **Found during:** Task 3 browser check (first run: `aria-label` missing on all three official markers).
- **Issue:** `marker.getElement()` is empty until the map has a view, so the layer's name and key handlers were never attached on a slow first fix.
- **Fix:** decorate on the marker's `add` event too, once per element; node case added.
- **Files:** `web/static/js/official-pins.js`, `web/official_pins_test.mjs`. **Commit:** `5d4d1992`.

**2. [Rule 3 - Blocking] map.js carries guards, which the plan did not expect ("change nothing else")**
- One `if (isOfficialPopup(...))` guard in each of `onPopupOpen` and `onPopupClose`, plus a module variable pair and the new functions (`addOverlay`, `isOfficialPopup`, `openCard`, `openOfficialPopup`, `centerOfficialInStrip`, `closeOfficialPopup`). Without them a GDACS popup would run `noteReportOpened(null)`, which writes `?report=null` into history. The guards leave the crowd path byte for byte as it was after the guard.
- The first draft of the open path called `panPopupIntoView` and `keepCardClearOfTools` directly and passed over `onReportPopState`; the existing tests that count those call sites (TestHarnessFindingP4_7OpenCardNeverSitsUnderTheTools, TestPopupReducedMotionPansWithoutAnimation) and the `report_view_test.mjs` harness, which extracts the report view section of map.js, failed. Rather than amend them, the card's open path calls `onMapMoveEnd()` (which does both for the open popup) and the switch from a report to a pin is handled in `openOfficialPopup` (see key decisions). No existing test was changed.

**3. [Scope] Files outside the plan's list**
- `web/static/css/official.css`: one rule (`.official-status.official-status--hazards:not([hidden])`, padding-top 4px) so the second line sits close under the first. Dated comment in the file.
- `web/official_alerts_test.mjs`: the `buildPage` helper gained an option `hazardLine` (default off) with a dated "Amended 2026-10-29 by plan 05-22" note; every older case runs on the same page as before.

### Choices inside the plan's freedom
- `openCard` takes a fourth argument `opts` ({opener, onClose}) so focus returns to the pin and the chosen look is released on close.
- The glyph classes are the layer's own (`official-glyph--earthquake`, `cyclone`, `flood`, `wildfire`, `volcano`), because `Pinalert.iconClass` would turn a volcano into "other".
- A fifth Go test, TestOfficialPinsAreLinked, pins the template wiring and that the Demo loads no GDACS layer.
- A card already open is not closed when a refresh finds the hazard gone; the next open replaces it. Documented in OFFICIAL-ALERTS.md.

## Known Stubs

None. The layer reads the real route and draws real stored hazards.

## Threat Flags

None beyond the plan's register. T-05-114 (textContent only, TestOfficialPinsScriptSinks and the markup case), T-05-115 (link allowlist, `noopener noreferrer`, 18 bad links in the node case), T-05-116 (shape, word, no colour, contract test and the browser table), T-05-117 (own layer group, one fetch, TestOfficialPinsNeverCounted and the unchanged list and count in the browser) and T-05-118 (nothing drawn unless fresh, node cases and the browser stale check) are mitigated and tested.

## Notes for the orchestrator

- Left for the orchestrator's iPhone simulator check: that the three pin kinds (crowd dot 26 px, Demo dot with dashed ring, GDACS rectangle with the word) are told apart on a real iPhone; Playwright WebKit has passed pages that the simulator later showed broken. The owner looks at a GDACS card at the viewing session.
- On a phone, closing the card leaves focus on the page body, exactly as closing a crowd report does there today (the detail sheet sits outside the map container, so the map's focus tracking never sees it). Not changed.
- `requirements` ROBUST-05, ROBUST-06 and ALERT-04 are not marked (project rule). STATE.md, ROADMAP.md and REQUIREMENTS.md are untouched.
- The live GDACS feed is still unproven: no live call was made here by rule. The timeout fix is on main (`4bc7448e`), not in this branch's base, so the docs describe main after the merge.

## Self-Check: PASSED

- Files present: official-pins.js, official-pins.css, official_pins_contract_test.go, official_pins_test.mjs, this summary.
- Commits present: f332a763, 889f7fda, 5d4d1992, 8bc1f2da.
- Acceptance greps: the four named Go tests 4, `Hazard events from GDACS` in official_alerts_test.mjs 7 lines (at least 3; 12 test cases), `addOverlay`/`openCard` in map.js and `PinalertMap.addOverlay(` in official-pins.js, `official-pins.js` and `official-pins.css` once each in index.html.tmpl, `noopener noreferrer` in official-pins.js, no reports route in official-pins.js code, `## GDACS hazard pins` once.
- Every project rule checked: path limited commits with the exact trailer, no push, no stash, no Python, no image in the repo, p5_22 only and dropped, port 8794 only and stopped.
