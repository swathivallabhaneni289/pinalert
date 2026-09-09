---
status: diagnosed
phase: 01-foundation-report-map
source: [01-VERIFICATION.md]
started: 2026-09-06T09:55:00Z
updated: 2026-09-09T09:55:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

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
reported: "i cant see the symbols properly" (original report, dark mode, solid-black glyphs — fixed by plan 01-13, see retest note below)
severity: minor
retest_after_01-13: |
  Retested after plan 01-13's CSS-mask fix. Original defect (solid-black-on-black,
  img-isolation) is confirmed fixed — screenshot shows all nine glyphs rendering as thin
  light/white outline shapes on the unselected tiles, matching the intended muted-text
  colour, not solid black.
  NEW distinct issue surfaced on retest: "It's not that clear. I think the black background
  is mixing up with the outlines, and I think we should make it more significant so
  everybody knows because... people use it in a emergency situations, and I don't want them
  to... search up. Oh, I can't see which one it [is]." User confirmed on follow-up this is a
  legibility/visual-weight complaint (thin, low-contrast strokes hard to distinguish at a
  glance), not tiles being literally blank. Severity set to minor (icons are present and
  distinguishable on close inspection, not missing/broken) but flagged as user-elevated given
  the emergency-app context — fast glyph recognition is a real usability requirement here,
  not pure cosmetic polish.

### 3. Severity slider accessibility (screen reader, keyboard, reduced motion)
expected: Tab to the slider; arrow keys/Home/End work; visible focus ring; a screen reader announces "1 · Low" / "2 · Medium" / "3 · Critical" (not just the bare number); colour animation disables under OS "Reduce motion" while the control stays usable.
result: pass

### 4. Full shelter/validation/discard interactive walkthrough
expected: "Shelter open" reveals a required capacity field that hides/clears on category switch; submitting with no category/no description/short description shows exact contract copy and moves focus to the offending field; typing then pressing Escape shows a discard-confirm prompt ("Keep editing" returns to the form, "Discard" closes it); opening and immediately escaping an untouched modal shows no prompt.
result: pass

### 5. Map/list linkage, ordering, age-desaturation, narrow-screen toggle
expected: With 3+ reports of differing severity/age — desktop: map+list side-by-side, click-to-fly and click-to-highlight both directions, critical rows sort above medium/low regardless of age, older reports visibly desaturate toward gray. Narrow (<900px): map-default view, toggle swaps with no gray-tile flash, selecting a list row returns to map view with the pin highlighted. Stopped server shows an error state with working Retry; an empty area shows empty-state copy. (Also exercises the two behavior_unverified_items below under real data: critical-first sort invariant, and `Pinalert.ageStage` boundary semantics.)
result: pass
notes: |
  Confirmed with live seeded data (2 critical, 1 medium, 2 low severity reports, one backdated
  into "aging" and one into "stale" age-stage): desktop map+list side-by-side layout; critical
  rows sorted above medium above low regardless of age; a stale low-severity report visibly
  desaturated from bright green to dull gray vs. a fresh one; clicking a list row flies the map
  to and highlights that pin; clicking a map pin highlights the corresponding list row; stopping
  the server produces a clean in-app "Couldn't load reports / Check your connection and try
  again." error state with a working Retry button (map tiles still render since they come from
  the third-party OpenFreeMap CDN, independent of the local server — expected).
  NOT independently confirmed by the user: narrow-screen (<900px) toggle behavior (user did not
  see a toggle appear when eyeballing a narrower window without a precise width, and explicitly
  said "think it's fine, let's move on" rather than reporting a firm defect) and the empty-area
  empty-state copy. Re-checked the toggle at the code level post-session and it IS correctly
  wired: `.view-toggle { display: none }` (main.css:414) becomes `display: inline-flex; position:
  fixed; ...` as a bottom-left pill button inside `@media (max-width: 899.98px)` (main.css:
  447-463), feed.js:43/380-388 wires the click handler to flip `.app-shell`'s `data-view`
  attribute, and main.css:430-445 shows/hides the two panes off that same attribute. Most likely
  explanation: the user's manual window-resize never actually crossed the 899.98px threshold, or
  the floating pill button was easy to miss. Recommend a quick recheck with the browser's
  device/responsive mode at a precise width, but not logged as a Gap — no defect found in the
  code path itself.

### 6. Dark-mode visual repaint
expected: Switching the OS to dark mode with the app open repaints using a genuinely distinct dark palette (brighter/more-saturated severity dots, darker tint backgrounds) per D-13 and 01-UI-SPEC.md's dark hex values — not an inverted light theme.
result: issue
reported: "All looks fine except for that one part [category icon dimness in the report modal]." (original report — fixed by plan 01-13, see retest note below)
severity: minor
note: "Same root cause as Test 2's gap (currentColor doesn't inherit through <img>-loaded SVGs) — not a new/separate defect, no duplicate Gap entry added. Everything else — background tints, severity-colored dots/pins, overall dark palette — confirmed as a genuinely distinct dark theme, not an inverted light one. (The earlier-reported 'black void' below the list was investigated and is NOT a bug — #0B0D10 is the exact spec'd dark --color-bg value per 01-UI-SPEC.md:95; see Gaps section.)"
retest_after_01-13: |
  Same retest and same new finding as Test 2 (shared root cause, shared fix): the
  solid-black-on-black defect is fixed, but the resulting thin/muted-grey glyph strokes are
  hard to distinguish at a glance in dark mode — see Test 2's retest_after_01-13 note and the
  Gaps section for the full report and severity rationale.

### 7. Category icon semantic/visual correctness
expected: All nine category SVGs are semantically correct when rendered (power_outage is a slashed bolt, earthquake is a seismograph zigzag, other is a plain flag) and none render as a broken/empty box.
result: pass
notes: "User confirmed all nine glyph shapes are semantically correct and none render broken/empty. Dimness/low-contrast in dark mode is a separate, already-tracked concern (same root cause as Test 2/6's gap), not a shape-correctness defect."

### 8. Live GitHub Actions CI run
expected: Push this branch (currently 48 commits ahead of origin/main, unpushed) or open a PR, and confirm `.github/workflows/ci.yml`'s Actions run is green (build, vet, test all pass against the postgres:16 service container) on GitHub's own infrastructure, not just local reproduction.
result: pass

### 9. Interactive Swagger UI walkthrough
expected: Open `/swagger/index.html`, expand `POST /reports` and `GET /reports`, and use "Try it out" on `GET /reports` with real lat/lon values — the UI renders correctly, both operations show all enum values, and "Try it out" returns live data from the database.
result: pass
notes: |
  Verified directly (Chrome extension unavailable this session, so not literally clicked through
  in a rendered browser, but every functional piece confirmed): GET /swagger/index.html -> 200
  text/html; GET /swagger/doc.json -> 200, valid Swagger 2.0 JSON documenting both POST /reports
  and GET /reports with all enum values present on both request/response schemas (9 categories,
  3 severities, 4 shelter_capacity_status values); zero session_id occurrences in the spec; a
  live GET /api/reports?lat=12.9716&lon=77.5946&radius_km=15 call (equivalent to what "Try it
  out" issues) returned real data — all 5 seeded reports, correctly distance-sorted.

### 10. Sync REQUIREMENTS.md's OPS-01 tracking entry
expected: Update `.planning/REQUIREMENTS.md`'s OPS-01 checklist line from "[ ] Pending" to "[x]" and its coverage-table row from "Pending" to "Complete", now that the Swagger/OpenAPI deliverable has been verified live and working, matching the other seven Phase 1 requirement rows already marked Complete.
result: pass
notes: "Done directly (mechanical tracking-file edit, not a visual UAT check) — REQUIREMENTS.md:114 checklist line and :219 traceability-table row both updated to Complete, matching Test 9's live-verified Swagger/OpenAPI functionality."

## Summary

total: 10
passed: 8
issues: 2
pending: 0
skipped: 0
blocked: 0
notes: 2 (cosmetic feedback on Test 3's severity slider styling, captured pre-emptively; and a Test-6-adjacent "black void" report that was investigated and found to be correct dark-mode styling per spec, not a bug — see Gaps. Tests 2 and 6 were retested 2026-09-09 after plan 01-13's fix: the original solid-black-icon defect is confirmed resolved, but both tests still show `result: issue` because retesting surfaced a new, distinct glyph-legibility issue — see Gaps.)

## Gaps

- truth: "A correctly-sized, non-overflowing category glyph appears on a colour-coded pin ... immediately after submit" (also affects the report modal's own category grid; confirmed also impacting Test 6/dark-mode via retest — same root cause, not a separate defect; Test 7/icon-correctness still pending)
  status: resolved
  reason: "User reported: i cant see the symbols properly (dark mode) — all 9 category icons render as solid black, invisible against the dark UI. Reconfirmed on Test 6 retest: 'all looks fine except for that one part' (icon dimness in the report modal)."
  severity: major
  test: [2, 6]
  resolution_confirmed: |
    User retested after plan 01-13 landed (screenshot of the report modal in dark mode) and
    confirmed the solid-black-on-black defect is gone — all nine glyphs render as visible
    light-coloured outline shapes, not black boxes. This specific truth (glyphs are visible,
    not solid black) is resolved. A DISTINCT follow-on issue — the resulting glyphs being
    thin/low-visual-weight and hard to distinguish at a glance — was raised in the same retest
    and is tracked as its own new Gap entry below (not this one), since it has a different
    root cause (icon stroke weight / colour choice, not img-isolation).
  artifacts: [web/static/css/main.css, web/static/css/feed.css, web/static/css/modal.css, web/static/js/app.js, web/static/js/map.js, web/static/js/modal.js, web/static/js/feed.js, web/css_contract_test.go, web/js_contract_test.go]
  missing: []
  fix_applied: |
    Plan 01-13 replaced option (a)/(c) below with option (b): a CSS mask-based glyph system.
    All three renderers (map.js, modal.js, feed.js) now build `aria-hidden` `<span
    class="icon-glyph icon-glyph--{category}">` nodes via a new `Pinalert.iconClass(category)`
    allowlist-validated helper, instead of `<img>` nodes pointed at an icon path. main.css gained
    an `.icon-glyph` base rule (`background-color: currentColor` + prefixed/unprefixed
    `mask-image` longhands) plus nine `.icon-glyph--{category}` rules, so glyph color now
    resolves through the ordinary CSS cascade (page background color on filled badges, muted
    text color on unselected tiles, inverted on selected tiles) in both light and dark mode —
    the `<img>` sealed-document isolation this bug depended on no longer applies. Two new Go
    contract tests (web/css_contract_test.go: TestCategoryGlyphMaskRulesCoverEveryCategory,
    web/js_contract_test.go: TestIconGlyphsAreClassDriven) fail the build if any category loses
    its glyph rule or a renderer reverts to `<img>`-based icons. `go build`, `go vet`, and
    `go test ./...` all pass post-merge.
    NOT yet confirmed by a human: the plan's own Task 3 visual check (all 6 context×theme
    combinations — badge/unselected-tile/selected-tile x light/dark — legible in Safari) was
    intentionally left uncertified by the executor per this project's human-check discipline.
    That visual confirmation is the one remaining item before this gap can be marked resolved;
    route it through /gsd-verify-work 01.
    Also noted by code review (01-REVIEW.md WR-01, non-blocking): `.icon-badge svg` in
    main.css:239-243 is dead CSS left over from the pre-mask approach, and the new contract
    test's own doc comment describes this exact failure mode but its implementation doesn't
    check for a stray `svg` selector — only a trailing `img` one. Cosmetic/coverage gap, not a
    functional regression.
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

- truth: "Category glyphs (report-modal tiles, map pin badges, feed row badges) are quickly identifiable at a glance in both light and dark mode, not just technically non-black" (new — found on the 01-13 retest, same UI area as the resolved Gap above but a different defect; scope widened after diagnosis found this affects all 3 render contexts, not just the modal tile the user directly screenshotted)
  status: diagnosed
  reason: "User reported (retest of Tests 2/6 after plan 01-13 landed, screenshot of the dark-mode report modal): \"It's not that clear. I think the black background is mixing up with the outlines, and I think we should make it more significant so everybody knows because... people use it in a emergency situations, and I don't want them to... search up. Oh, I can't see which one it [is].\" Confirmed on follow-up: the glyphs ARE rendering (not blank/solid-black), but their thin strokes are hard to distinguish from the tile background at a glance."
  severity: minor
  test: [2, 6]
  root_cause: |
    All 9 category SVGs (web/static/icons/*.svg, Lucide v1.41.0 icon set) are pure stroke/line
    art — `fill="none" stroke="currentColor" stroke-width="2"` on a 24x24 viewBox, zero filled
    shapes. CSS `mask-image` alpha-masks the rendered SVG, so only the ~2px stroke path is ever
    unmasked (rendered stroke width ≈1.47-2.02px across the 3 contexts, thinnest in the 17.6px
    feed-row badge) — an inherent hairline regardless of color token. This is NOT a hex-contrast
    defect: the unselected-tile pairing this session flagged (--color-text-muted #9CA3AF on
    --color-surface #16181C) computes to ≈7:1, passing WCAG AAA — cross-checked against light
    mode's ≈4.3:1 for the same pairing (objectively worse contrast, yet drew no complaint),
    confirming color isn't the discriminating factor. The bottleneck is the source artwork's
    stroke-only geometry capping ink coverage regardless of color choice.
  artifacts:
    - path: "web/static/icons/*.svg"
      issue: "All 9 files are stroke-only outline art (fill=\"none\"), so the CSS mask's opaque area is only a ~2px stroke line, not a filled silhouette"
    - path: "web/static/css/main.css:265-274"
      issue: ".icon-glyph base rule — mask mechanism is correctly implemented, not the defect; included for context"
    - path: "web/static/css/modal.css:57-60"
      issue: "24px modal-tile glyph sizing (thinnest-affected of the 3 contexts is feed.css, not this one)"
    - path: "web/static/css/feed.css:56-59"
      issue: "17.6px feed-row glyph sizing — smallest render, ≈1.47px stroke, most vulnerable to sub-pixel thinning"
  missing:
    - "Heavier/filled icon artwork, or a thicker stroke-width, so the CSS mask has more ink coverage regardless of color token"
  debug_session: .planning/debug/category-glyph-legibility.md

- truth: "A map-pin or feed-row badge's category glyph has sufficient WCAG point-contrast against its badge background, at every age stage" (new — found during diagnosis of the entry above, not directly reported by the user; scoped to badge contexts only, distinct root cause from the entry above)
  status: diagnosed
  reason: "Not directly reported by the human tester (who only exercised the modal category grid, which has no age-ramp involvement) — surfaced as a second, additive mechanism while diagnosing the glyph-legibility gap above, per this project's fail-safe discipline of not dropping a diagnosed defect just because it wasn't the literal complaint."
  severity: minor
  test: [2, 6]
  root_cause: |
    `.icon-badge` paints its glyph with `color: var(--color-bg)` against a `--severity-current`
    background; `.age-stale` (main.css:211-215) overrides `--severity-current` to a flat
    `--color-age-stale` regardless of the report's original severity. Computed contrast: dark
    mode glyph #0B0D10 on badge #4B4F55 ≈2.36:1 (fails WCAG 1.4.11's 3:1 graphical-object floor
    and AA's 4.5:1 text floor); light mode glyph #FFFFFF on badge #C9CDD1 ≈1.6:1 (worse still).
    Severity-independent (age-stale forces the same neutral gray regardless of original
    severity) — every stale-aged report's map-pin/feed-row glyph has genuinely insufficient
    contrast in both themes. `.age-aging` (50% color-mix toward stale) is borderline
    (~3.7:1 for critical/dark — above the 3:1 graphical floor, below the 4.5:1 text floor) but
    not a clear failure like the stale stage. This is additive to, not a duplicate of, the
    stroke-thinness mechanism above — it's a genuine, quantifiable point-contrast defect on top
    of the inherent hairline-stroke problem, scoped only to badges (not the modal grid, which
    has no age-ramp) once a report desaturates to "stale."
  artifacts:
    - path: "web/static/css/main.css:211-215"
      issue: ".age-stale flattens badge background to a fixed --color-age-stale regardless of severity, without adjusting the glyph's --color-bg foreground to compensate"
    - path: "web/static/css/main.css:236"
      issue: ".icon-badge glyph color: var(--color-bg) — the foreground half of the failing pairing"
  missing:
    - "A stale-stage badge glyph/background pairing that clears WCAG 1.4.11's 3:1 graphical-object floor in both themes (e.g. a different foreground token for aged badges, or adjusting --color-age-stale's lightness)"
  debug_session: .planning/debug/category-glyph-legibility.md

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

- truth: "Switching the OS to dark mode with the app open repaints using a genuinely distinct dark palette (brighter/more-saturated severity dots, darker tint backgrounds) per D-13 and 01-UI-SPEC.md's dark hex values — not an inverted light theme." (Test 6 — investigated, NOT a bug, see resolution below)
  status: not_a_bug
  reason: "User reported (unprompted, via two screenshots): with dark mode active and 1-3 reports in the feed, the list panel below the last report card renders as a large solid pitch-black area, initially assumed to be missing background styling."
  severity: n/a
  test: 6
  root_cause_confirmed: |
    FALSE ALARM — verified directly against the code before handing to fix-planning, and it's
    correct as shipped. `body { background: var(--color-bg) }` (main.css:161) is the only
    background rule needed here since `.pane`/`.pane--list`/`.app-shell` are all intentionally
    transparent over it (main.css:371-388). `--color-bg` in dark mode is `#0B0D10`
    (main.css:76,103) — and 01-UI-SPEC.md:95 documents `#0B0D10` as the exact spec'd dark value
    for "Page background, modal background, map container background" (60% dominant color). A
    near-black hex value reads as "solid black" in a screenshot/to the eye, but it IS the
    intended dark palette per spec, not an unstyled void. No fix needed; this entry stays only
    as a record that the report was investigated, not dropped silently.

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
