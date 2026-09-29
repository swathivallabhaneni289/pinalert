---
phase: 07-address-search-box-for-report-location
plan: 02
subsystem: ui
tags: [html-template, css, design-tokens, accessibility, dom-contract]

# Dependency graph
requires:
  - phase: 07-address-search-box-for-report-location
    provides: "07-01's internal/geocode client and internal/api/handlers geocode handler (backend search endpoint plan 07-04 will call)"
provides:
  - "Report modal location search markup: labelled #location-search input, #location-search-results listbox, #location-search-status inline status line"
  - "TestPageShellServesDOMContract now asserts all three new ids"
  - ".location-search-field / #location-search / .location-search-result / .location-search-result-secondary CSS classes, styled from design tokens only"
  - "TestLocationSearchHiddenGuards and TestLocationSearchDropdownStacking contract tests"
affects: ["07-04 (location search behavior/JS)"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "position: relative wrapper (.location-search-field) anchors an absolutely-positioned dropdown to the input rather than to .modal-panel, avoiding Leaflet pane clipping"
    - ":not([hidden]) guard required on every display-setting rule for a hidden-by-default element, per the standing Phase 1 modal-backdrop UAT lesson"

key-files:
  created: []
  modified:
    - web/templates/index.html.tmpl
    - internal/api/handlers/page_test.go
    - web/static/css/modal.css
    - web/css_contract_test.go

key-decisions:
  - "Search input placed above the modal map in DOM order (title, search, map, coord readout, ...) per CONTEXT.md's reading-order framing and the plan's explicit discretion grant"
  - "Discard confirmation overlay z-index raised from 10 to 1200 so it always paints above the dropdown's 1100, since .modal-panel is not its own stacking context and the two compete directly"

patterns-established:
  - "New hidden-by-default DOM elements in this codebase get a matching TestXHiddenGuards contract test modelled on TestModalBackdropHiddenGuard/TestAccountMenuHiddenGuard, reusing stripCSSComments"
  - "z-index stacking order between competing overlays gets a dedicated contract test (TestLocationSearchDropdownStacking) reusing parseCSSRules/declsOf/ruleBySelector rather than a one-off assertion"

requirements-completed: [D-01, D-04]

coverage:
  - id: D1
    description: "Report modal renders a labelled search input above the modal map, plus an empty suggestion container and an empty inline status line, all three addressable by id"
    requirement: "D-01"
    verification:
      - kind: unit
        ref: "internal/api/handlers/page_test.go#TestPageShellServesDOMContract"
        status: pass
    human_judgment: false
  - id: D2
    description: "Suggestion dropdown and inline status line stay invisible on first paint (hidden by default) until JS clears the attribute"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestLocationSearchHiddenGuards"
        status: pass
    human_judgment: false
  - id: D3
    description: "Suggestion dropdown anchors to the search input's own wrapper and paints above every Leaflet pane and below the discard confirmation overlay"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestLocationSearchDropdownStacking"
        status: pass
    human_judgment: false
  - id: D4
    description: "No em dash/en dash in new copy; no fully-rounded control introduced"
    verification:
      - kind: unit
        ref: "web/design_rules_contract_test.go#TestUserVisibleCopyUsesPlainPunctuation"
        status: pass
      - kind: unit
        ref: "web/design_rules_contract_test.go#TestNoPillShapedControls"
        status: pass
    human_judgment: false
  - id: D5
    description: "Visual verification: input styling matches shelter-capacity control conventions, dropdown/status line invisible on load, modal map and pin-drop unaffected, in both light and dark theme"
    verification: []
    human_judgment: true
    rationale: "Requires a real browser render (light/dark theme, exact corner rounding/height/border-colour match, absence of a stray empty box). Deferred to end-of-phase UAT per workflow.human_verify_mode: end-of-phase, recorded as a human-check in 07-02-PLAN.md's <verification> block."

duration: 32min
completed: 2026-09-29
status: complete
---

# Phase 07 Plan 02: Address Search Box Markup and Styling Summary

**Labelled address search input, suggestion dropdown, and inline status line added to the report modal above the map, styled from design tokens with a `:not([hidden])`-guarded stacking order that keeps the dropdown below the discard-confirm overlay and above Leaflet's panes.**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-29T06:28:00Z
- **Completed:** 2026-09-29T07:00:58Z
- **Tasks:** 2
- **Files modified:** 4 (plus one new deferred-items.md log)

## Accomplishments
- Report modal now renders three new addressable elements above `#modal-map`: `#location-search` (labelled text input), `#location-search-results` (empty hidden listbox), `#location-search-status` (empty hidden inline status line), wrapped so the dropdown anchors to the input rather than the whole panel
- `TestPageShellServesDOMContract` extended with all three new ids, locking the hook plan 07-04 depends on
- `#location-search`, `.location-search-result`, `.location-search-result-secondary` styled entirely from `main.css` design tokens (no raw hex), matching the shelter-capacity control's border/radius/height conventions
- Both hidden-by-default selectors carry a `:not([hidden])` guard on every `display`-setting rule, closing the exact gap that caused Phase 1's modal-backdrop UAT blocker
- Dropdown z-index (1100) clears Leaflet's max pane z-index (1000); discard-confirm overlay raised to 1200 so it always paints above an open dropdown
- Two new contract tests, `TestLocationSearchHiddenGuards` and `TestLocationSearchDropdownStacking`, reuse the package's existing `stripCSSComments`/`parseCSSRules`/`declsOf`/`ruleBySelector` helpers rather than adding a second parser

## Task Commits

Each task was committed atomically:

1. **Task 1: search box markup in the report modal, plus the DOM contract assertion** - `75a47e7` (feat)
2. **Task 2: search box and dropdown styling, with stacking and hidden-attribute contract tests** - `d1b6d1d` (feat)

**Deferred-items log:** `43908bb` (docs: log pre-existing full-suite test flakiness)

_Note: no plan-metadata commit yet — this plan runs inside a worktree; the orchestrator's merge step performs the final shared-state commit after all wave agents complete._

## Files Created/Modified
- `web/templates/index.html.tmpl` - inserted the location search markup block (label, `.location-search-field` wrapper with `#location-search` and `#location-search-results`, `#location-search-status`) between `#modal-title` and `#modal-map`
- `internal/api/handlers/page_test.go` - added `id="location-search"`, `id="location-search-results"`, `id="location-search-status"` to `TestPageShellServesDOMContract`'s `wantIDs`
- `web/static/css/modal.css` - added the `.location-search-field` / `#location-search` / `#location-search-results:not([hidden])` / `.location-search-result` / `.location-search-result-secondary` / `#location-search-status:not([hidden])` rule block; raised `#discard-confirm:not([hidden])`'s z-index from 10 to 1200
- `web/css_contract_test.go` - added `TestLocationSearchHiddenGuards` and `TestLocationSearchDropdownStacking`
- `.planning/phases/07-address-search-box-for-report-location/deferred-items.md` - new; logs pre-existing full-suite test flakiness discovered during final verification (see Issues Encountered)

## Decisions Made
- Search input placed above the map in DOM order (not below it), per CONTEXT.md's own framing that a person types a place, then sees the resulting pin on the map below — placement was explicitly left to Claude's discretion by the plan
- `#discard-confirm`'s z-index raised to 1200 rather than coupling `requestClose()` to close the dropdown in JavaScript: the discard overlay is a confirmation that must occlude everything in the panel, so raising its stacking order is the correct, simpler fix

## Deviations from Plan

None - plan executed exactly as written. Both tasks' markup, CSS, and test additions match the plan's `<action>` blocks precisely (element order, attribute list, selector rules, z-index values, comment content).

## Issues Encountered

**Full-suite test flakiness, unrelated to this plan's changes.** During Task 2's final verification pass, `go test ./... -short -count=1` (both with default parallelism and with `-p 1` for serial execution) surfaced a different set of failing tests on each of three runs, spread across `internal/api`, `internal/api/handlers`, `internal/store`, and `internal/testutil` — packages this plan does not touch (its two tasks are template/CSS/CSS-test-only). Every failing test passed when re-run in isolation. This is consistent with shared-state or timing contention against the local `pinalert_test` Postgres database, not a regression introduced here. Logged to `deferred-items.md` per the executor's Scope Boundary rule rather than fixed, since it is out of scope for a markup/CSS-only plan. This plan's own scoped verification — `go test ./web/...`, `go test ./internal/api/handlers/ -run TestPageShellServesDOMContract`, and `go test ./internal/api/handlers/ -short` — passed cleanly and deterministically on every run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

The three DOM hooks (`location-search`, `location-search-results`, `location-search-status`) and the exact class names (`.location-search-field`, `.location-search-result`, `.location-search-result-secondary`) plan 07-04 needs to build its dropdown are now locked into a standing contract test and ready to consume. Final z-index values shipped: dropdown `1100`, discard overlay `1200` (unchanged Leaflet ceiling `1000`). Label copy: "Search for a place or address"; placeholder copy: "Type a place name or address" — both plain-punctuation, no em/en dash, no icon glyph. All three new elements are inert (no JavaScript wired yet, by design) until plan 07-04 lands.

The pre-existing full-suite test flakiness noted above is not a blocker for this plan but is worth a look before the phase's overall post-wave regression gate — see `deferred-items.md`.

---
*Phase: 07-address-search-box-for-report-location*
*Completed: 2026-09-29*

## Self-Check: PASSED

All 6 claimed files found on disk (index.html.tmpl, page_test.go, modal.css, css_contract_test.go,
deferred-items.md, this SUMMARY.md). All 4 claimed commit hashes (75a47e7, d1b6d1d, 43908bb,
7e67dc3) found in `git log --oneline`.
