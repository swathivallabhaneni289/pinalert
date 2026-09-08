---
status: partial
phase: 01-foundation-report-map
source: [01-VERIFICATION.md]
started: 2026-09-06T09:55:00Z
updated: 2026-09-08T15:45:00Z
---

## Current Test

[session paused — Test 2 found a major issue: category icons render solid black in dark mode
(currentColor doesn't inherit through <img>-loaded SVGs). Root cause fully confirmed, fix not yet
planned/applied. Resume via /gsd-verify-work 01 at Test 2 once a fix lands, or continue the gap-
closure pipeline (diagnose -> plan -> execute -> review) in a fresh session given context budget.]

<!-- Basemap migration note (post-Test-1, pre-Test-2): plans 01-11/01-12 replaced the OSM raster
basemap with OpenFreeMap's "liberty" vector style via MapLibre GL, with an automatic WebGL2
capability fallback to the byte-identical prior raster layer. On this Mac's Safari the map is
currently rendering via the RASTER FALLBACK path (confirmed by the attribution text reading
"Leaflet | © OpenStreetMap contributors" rather than the vector path's three-link OpenFreeMap/
OpenMapTiles/OpenStreetMap credit) — CORS headers and a sandboxed execution of the actual vendor
script bytes both confirm the vendor code itself is correct, so this is very likely this browser
session's WebGL2 availability, not a code defect. Either basemap renders correctly and every
Leaflet-based marker/popup/badge/drag/fly-to/highlight behavior is identical on both paths per
01-12-PLAN.md's design, so this does not block resuming UAT. Tests 2 and 5 exercise map-behavioral
code paths and should be evaluated against whichever basemap the tester's browser actually shows. -->

## Tests

### 1. Live map tile rendering
expected: A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback.
result: pass
notes: "Two blockers found and fixed in sequence before this passed: (1) modal-backdrop [hidden] guard (01-08), (2) #map had no CSS height at all (01-09). User confirmed the map now renders fully with correct centering and real tiles. Separately noted: tile text looks blurry on a Retina display — tracked as a cosmetic gap below (not blocking this test's literal pass criteria)."

### 2. New-pin visual rendering after submit (exercises the CR-01 icon-sizing fix)
expected: Submit a report via the "+" button; a correctly-sized (55% of badge), non-overflowing category glyph appears on a colour-coded pin at the submitted coordinate immediately after submit, with no page reload.
result: issue
reported: "i cant see the symbols properly" (screenshot: report modal in dark mode — all 9 category icons render as solid black glyphs, essentially invisible against the dark modal background)
severity: major

### 3. Severity slider accessibility (screen reader, keyboard, reduced motion)
expected: Tab to the slider; arrow keys/Home/End work; visible focus ring; a screen reader announces "1 · Low" / "2 · Medium" / "3 · Critical" (not just the bare number); colour animation disables under OS "Reduce motion" while the control stays usable.
result: [pending]

### 4. Full shelter/validation/discard interactive walkthrough
expected: "Shelter open" reveals a required capacity field that hides/clears on category switch; submitting with no category/no description/short description shows exact contract copy and moves focus to the offending field; typing then pressing Escape shows a discard-confirm prompt ("Keep editing" returns to the form, "Discard" closes it); opening and immediately escaping an untouched modal shows no prompt.
result: [pending]

### 5. Map/list linkage, ordering, age-desaturation, narrow-screen toggle
expected: With 3+ reports of differing severity/age — desktop: map+list side-by-side, click-to-fly and click-to-highlight both directions, critical rows sort above medium/low regardless of age, older reports visibly desaturate toward gray. Narrow (<900px): map-default view, toggle swaps with no gray-tile flash, selecting a list row returns to map view with the pin highlighted. Stopped server shows an error state with working Retry; an empty area shows empty-state copy. (Also exercises the two behavior_unverified_items below under real data: critical-first sort invariant, and `Pinalert.ageStage` boundary semantics.)
result: [pending]

### 6. Dark-mode visual repaint
expected: Switching the OS to dark mode with the app open repaints using a genuinely distinct dark palette (brighter/more-saturated severity dots, darker tint backgrounds) per D-13 and 01-UI-SPEC.md's dark hex values — not an inverted light theme.
result: [pending]

### 7. Category icon semantic/visual correctness
expected: All nine category SVGs are semantically correct when rendered (power_outage is a slashed bolt, earthquake is a seismograph zigzag, other is a plain flag) and none render as a broken/empty box.
result: [pending]

### 8. Live GitHub Actions CI run
expected: Push this branch (currently 48 commits ahead of origin/main, unpushed) or open a PR, and confirm `.github/workflows/ci.yml`'s Actions run is green (build, vet, test all pass against the postgres:16 service container) on GitHub's own infrastructure, not just local reproduction.
result: pass

### 9. Interactive Swagger UI walkthrough
expected: Open `/swagger/index.html`, expand `POST /reports` and `GET /reports`, and use "Try it out" on `GET /reports` with real lat/lon values — the UI renders correctly, both operations show all enum values, and "Try it out" returns live data from the database.
result: [pending]

### 10. Sync REQUIREMENTS.md's OPS-01 tracking entry
expected: Update `.planning/REQUIREMENTS.md`'s OPS-01 checklist line from "[ ] Pending" to "[x]" and its coverage-table row from "Pending" to "Complete", now that the Swagger/OpenAPI deliverable has been verified live and working, matching the other seven Phase 1 requirement rows already marked Complete.
result: [pending]

## Summary

total: 10
passed: 2
issues: 1
pending: 7
skipped: 0
blocked: 0
notes: 1 (cosmetic feedback on Test 3's severity slider styling, captured pre-emptively; Test 3 itself remains pending for its full accessibility checklist)

## Gaps

- truth: "A correctly-sized, non-overflowing category glyph appears on a colour-coded pin ... immediately after submit" (also affects the report modal's own category grid, and by extension Test 6/dark-mode and Test 7/icon-correctness, both still pending)
  status: failed
  reason: "User reported: i cant see the symbols properly (dark mode) — all 9 category icons render as solid black, invisible against the dark UI"
  severity: major
  test: 2
  artifacts: [web/static/js/map.js, web/static/js/modal.js, web/static/icons/*.svg]
  missing: ["a way for currentColor-based SVGs to actually inherit page CSS color when dark mode is active"]
  root_cause_confirmed: |
    Every category SVG (web/static/icons/*.svg, e.g. flood.svg) uses `stroke="currentColor"` —
    a technique that only works when an SVG is inlined directly into the page DOM, so
    `currentColor` resolves against the surrounding element's CSS `color` (which main.css
    already sets differently for light/dark mode).
    But both icon call sites — map.js:143-144 (map pin badges) and modal.js:209-210 (category
    grid tiles) — load these SVGs via `document.createElement('img'); img.src =
    Pinalert.iconPath(category);`. An <img>-referenced SVG is rendered as a fully sealed,
    separate document: the browser does NOT let the parent page's CSS (including `color`)
    reach inside it. `currentColor` inside an <img>-loaded SVG therefore always resolves to
    that isolated document's own default text color (effectively black), regardless of the
    page's light/dark mode. In light mode this was an invisible bug (black-on-white/light
    background reads fine); in dark mode it produces black-on-near-black icons that are
    unreadable — exactly what the user's screenshot shows.
    Fix requires one of: (a) fetch each SVG's text and inline it as a real <svg> DOM element
    (via innerHTML on a sanitized/trusted string, or DOMParser) instead of an <img> src, so
    currentColor can inherit normally; or (b) keep <img> but recolor via a CSS mask-image
    technique (background-color + mask-image: url(icon.svg), which DOES respect page CSS
    since the color comes from background-color, not the SVG's own currentColor); or (c) ship
    two static icon variants (light/dark) and swap img.src based on the active theme. Touches
    both map.js's badge-building code and modal.js's category-grid-building code identically,
    since both use the same <img>-based pattern. Not caused by the 01-11/01-12 basemap
    migration — pre-existing since the icon system shipped in plan 01-02, only now surfaced by
    a human actually testing in dark mode.

- truth: "A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback."
  status: fix_landed
  reason: "User reported: it looks like this and i dont see any map (screenshot shows the report-submission modal already open on a fresh page load, covering the whole viewport in a near-black backdrop)"
  severity: blocker
  test: 1
  artifacts: [web/static/css/main.css:549]
  missing: []
  root_cause_confirmed: |
    `.modal-backdrop { display: flex; }` in main.css:549 has no `:not([hidden])` guard.
    `#report-modal` (class="modal-backdrop") carries the `hidden` attribute by default
    (web/templates/index.html.tmpl:48) and modal.js only clears it on the FAB click
    (openModal(), web/static/js/modal.js) — but the browser's native `[hidden] { display:
    none }` rule is user-agent-origin, so it loses to this author-origin `.modal-backdrop`
    display rule at equal specificity regardless of hidden actually being present. The
    modal therefore renders open (and its rgb(0 0 0 / 55%) full-viewport scrim) on every
    page load, before the FAB is ever clicked — obscuring the map/list pane underneath it.
    This exact bug class was already identified and fixed elsewhere in this same codebase
    for #shelter-fields and #discard-confirm (modal.css:139,222) and for
    #report-list/#feed-skeleton/#feed-empty/#feed-error (feed.css:13-26, with an explicit
    comment explaining the specificity fix) — `.modal-backdrop` itself was missed.
    Fix: add `.modal-backdrop:not([hidden])` (matching the existing pattern) or move the
    `display: flex` off the bare class selector.

- truth: "A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback." (retest, new root cause)
  status: fix_landed
  reason: "Retest after 01-08 landed: modal-backdrop fix confirmed working (modal only opens on '+' click, and the modal's own #modal-map renders real tiles correctly). But the PRIMARY #map (behind the modal, in .pane--map) is still solid black on a fresh page load — screenshot shows only the list panel, no map at all."
  severity: blocker
  test: 1
  artifacts: [web/static/css/main.css]
  missing: ["a height rule for #map"]
  root_cause_confirmed: |
    `#map` (web/templates/index.html.tmpl:22) has NO CSS rule anywhere in main.css or
    feed.css setting its height — confirmed via grep across both files (only `#modal-map`,
    the SEPARATE Leaflet instance inside the report modal, has an explicit `height: 220px`
    in modal.css:23-27, which is exactly why that map renders fine while the primary one
    doesn't). `#map`'s only ancestor is `.pane--map` inside `.app-shell` (display: grid);
    `.pane { min-height: 0; overflow: hidden; position: relative; }` sets no height either.
    The FAB button (`.fab`) is `position: fixed` (main.css:292), so it's removed from normal
    flow and contributes nothing to `.pane--map`'s content height. With no explicit height
    anywhere in the ancestor chain and no in-flow sibling content, `#map` collapses to 0
    height, so `L.map(container)` in map.js's `init()` initializes against a 0-height
    viewport — Leaflet has no visible area to paint tiles into. This is a pre-existing
    defect from plan 01-04 (walking skeleton) that has been masked the entire time by the
    01-08-fixed modal-backdrop bug: the modal's full-viewport black scrim covered the
    entire screen on every load, so nobody could ever see that the map behind it was
    already broken. Fix: give `#map` (and/or `.pane--map`) an explicit height — e.g.
    `#map { height: 100%; }` plus ensuring `.pane--map` and its `.app-shell` grid row
    actually resolve to a non-zero height (grid rows need `align-items: stretch` — the
    grid default — AND a definite height on some ancestor up to the viewport; `.app-shell`
    already has `min-height: 100vh` but that's a MIN on the shell itself, not a stretch
    target down through `.pane` to `#map` — needs verification against actual grid
    row-sizing behavior when the only sized ancestor is `min-height` on the grid container,
    not `height`).

- truth: "Severity slider accessibility (screen reader, keyboard, reduced motion)" (Test 3, not yet formally run)
  status: fix_landed
  reason: "User reported (unprompted, from the screenshot): the severity slider look is 'too basic level' and asked for the interaction/visuals to feel smoother/more polished. This is a visual-polish note, not a functional break — full accessibility checklist for Test 3 (screen reader announcement, keyboard/Home/End, reduced-motion) is still untested and should be re-verified once restyled."
  severity: cosmetic
  test: 3
  artifacts: [web/static/css/modal.css, web/static/css/main.css]
  missing: []

- truth: "A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback." (tile quality, Test 1 passed but flagged)
  status: fix_landed
  reason: "User reported: map now renders correctly (Test 1 passed) but tile text/labels look blurry on a Retina display."
  severity: cosmetic
  test: 1
  artifacts: [web/static/js/map.js, web/static/js/modal.js]
  missing: ["detectRetina: true on both L.tileLayer(...) calls"]
  root_cause_confirmed: |
    tile.openstreetmap.org serves only standard-resolution (1x, 256px) raster tiles — it has
    no {r}/@2x retina variant. Neither map.js's primary L.tileLayer (line ~25) nor modal.js's
    modal-map L.tileLayer (line ~319) sets `detectRetina`, so on a Retina/HiDPI display the
    browser upscales the 1x tile images by the device pixel ratio, which is what makes text
    look soft. Confirmed via Leaflet's own official docs (leafletjs.com/reference.html) that
    `detectRetina: true` alone — no manual `tileSize`/`zoomOffset` needed — makes Leaflet
    automatically request four tiles from one zoom level higher and composite them into the
    same space on a detected retina display, yielding genuinely higher-detail rendering (not
    just an upscale) with zero new vendor, API key, or cost. User evaluated and explicitly
    rejected switching to Google Maps (would require a Google Cloud billing account/card even
    for the free tier, contradicting this project's "no paid map API" constraint) and MapTiler/
    Stadia Maps (better quality but require a new vendor + free API key signup) in favor of
    this zero-dependency fix. Fix: add `detectRetina: true` to both existing L.tileLayer
    options objects.
