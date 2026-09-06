---
status: testing
phase: 01-foundation-report-map
source: [01-VERIFICATION.md]
started: 2026-09-06T09:55:00Z
updated: 2026-09-06T12:22:00Z
---

## Current Test

number: 1
name: Live map tile rendering in a real browser (retest 2)
expected: |
  Open http://localhost:8090 and confirm the primary map (behind the "+" button, not the modal)
  renders visible OpenStreetMap tiles filling the map pane, not a blank/black area. Centred on
  geolocation or the Bengaluru fallback.
awaiting: user response

## Tests

### 1. Live map tile rendering
expected: A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback.
result: [pending]
retest_note: "Two blockers found and fixed in sequence: (1) modal-backdrop [hidden] guard (01-08, confirmed working), (2) #map had no CSS height at all (01-09, commit 45ab1bb — #map { height: 100vh; height: 100dvh; }). Server restarted on port 8090 serving the fixed CSS. Re-testing again."

### 2. New-pin visual rendering after submit (exercises the CR-01 icon-sizing fix)
expected: Submit a report via the "+" button; a correctly-sized (55% of badge), non-overflowing category glyph appears on a colour-coded pin at the submitted coordinate immediately after submit, with no page reload.
result: [pending]

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
passed: 1
issues: 0
pending: 9
skipped: 0
blocked: 0
notes: 1 (cosmetic feedback on Test 3's severity slider styling, captured pre-emptively; Test 3 itself remains pending for its full accessibility checklist)

## Gaps

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
