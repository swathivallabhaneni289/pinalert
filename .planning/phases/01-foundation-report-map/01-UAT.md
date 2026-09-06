---
status: testing
phase: 01-foundation-report-map
source: [01-VERIFICATION.md]
started: 2026-09-06T09:55:00Z
updated: 2026-09-06T09:55:00Z
---

## Current Test

number: 1
name: Live map tile rendering in a real browser
expected: |
  Open the app (`make migrate && make run`) and confirm the Leaflet/OSM map renders visible
  tiles, not a blank gray #map div. A live OpenStreetMap tile layer is visible, centred on
  geolocation or the Bengaluru fallback.
awaiting: user response

## Tests

### 1. Live map tile rendering
expected: A live OpenStreetMap tile layer is visible, centred on geolocation or the Bengaluru fallback.
result: [pending]

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
result: [pending]

### 9. Interactive Swagger UI walkthrough
expected: Open `/swagger/index.html`, expand `POST /reports` and `GET /reports`, and use "Try it out" on `GET /reports` with real lat/lon values — the UI renders correctly, both operations show all enum values, and "Try it out" returns live data from the database.
result: [pending]

### 10. Sync REQUIREMENTS.md's OPS-01 tracking entry
expected: Update `.planning/REQUIREMENTS.md`'s OPS-01 checklist line from "[ ] Pending" to "[x]" and its coverage-table row from "Pending" to "Complete", now that the Swagger/OpenAPI deliverable has been verified live and working, matching the other seven Phase 1 requirement rows already marked Complete.
result: [pending]

## Summary

total: 10
passed: 0
issues: 0
pending: 10
skipped: 0
blocked: 0

## Gaps
