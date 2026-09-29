---
phase: 07-address-search-box-for-report-location
plan: 04
subsystem: ui
tags: [javascript, dom, geocoding, debounce, sink-discipline, leaflet]

# Dependency graph
requires:
  - phase: 07-address-search-box-for-report-location
    provides: "07-02's location-search/location-search-results/location-search-status DOM hooks and CSS classes"
  - phase: 07-address-search-box-for-report-location
    provides: "07-03's live GET /api/geocode route, gated and rate-limited"
provides:
  - "web/static/js/modal.js: the location search module (debounced dropdown, suggestion tap, inline status messages, resetSearch teardown)"
  - "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement: locks the sink discipline, single pin-placement path, top-level listener wiring, and six-piece teardown contract"
  - ".planning/phases/07-address-search-box-for-report-location/07-VALIDATION.md: backfilled Per-Task Verification Map, status flipped out of draft"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pause-based debounce (clearTimeout + setTimeout per keystroke) with a monotonically increasing sequence counter to discard out-of-order fetch responses, mirroring votes.js's castVote fetch/error idiom"
    - "Session-scoped Map cache keyed on lowercased query text, cleared by the same teardown path that clears every other piece of transient UI state"
    - "New DOM-reading behavior gets a Go contract test that reads the embedded static asset (StaticFS) rather than a pasted copy, in the style of this package's existing basemap/icon/theme contract tests"

key-files:
  created: []
  modified:
    - web/static/js/modal.js
    - web/js_contract_test.go
    - .planning/phases/07-address-search-box-for-report-location/07-VALIDATION.md

key-decisions:
  - "resetSearch is a dedicated function called from resetForm rather than inlined, matching the plan's explicit instruction that resetForm stays the single teardown path with no parallel teardown function introduced"
  - "The sequence guard (mySeq !== searchSeq) is checked on both the success and the failure path of runSearch, not only success, so a stale request's failure cannot repaint a status message for a query the visitor already moved past"
  - "Threat Ref column values for the backfilled validation map were sourced directly from each plan's own <threat_model> table (07-01/07-02/07-03 PLAN.md files), not guessed, since those files were still on disk and readable"

requirements-completed: [D-01, D-03, D-04]

coverage:
  - id: D1
    description: "Every place name and full address returned by the geocoding proxy reaches the DOM only through Pinalert.setText, never a markup-parsing sink (T-07-03)"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement"
        status: pass
    human_judgment: false
  - id: D2
    description: "A suggestion tap reuses the file's single existing pin-placement path (L.marker( appears exactly once) rather than constructing a second marker"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement"
        status: pass
    human_judgment: false
  - id: D3
    description: "The search input's listener is registered once in the top-level wiring block, never inside initLocation (which runs on every openModal call)"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement"
        status: pass
    human_judgment: false
  - id: D4
    description: "resetSearch clears all six pieces of search state (timer, sequence counter, input value, dropdown children, status text, query cache) and is wired into resetForm, the single teardown path"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement"
        status: pass
    human_judgment: false
  - id: D5
    description: "The submit-time location validation message names searching as one of the three ways to set a location"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestLocationSearchUsesTextSinksAndExistingPinPlacement"
        status: pass
    human_judgment: false
  - id: D6
    description: "Typing at least three code points and pausing shows a suggestion list; exactly one request per pause, none per keystroke, and a repeated query is served from the in-session cache with no request at all"
    requirement: "D-01"
    verification: []
    human_judgment: true
    rationale: "No JS test framework exists in this repo; debounce timing and network-panel request counting are live-browser behaviors, deferred to /gsd-verify-work 7 per workflow.human_verify_mode: end-of-phase."
  - id: D7
    description: "Tapping a suggestion places and centres the pin at zoom 16 and the pin stays draggable afterward with no extra confirmation step"
    requirement: "D-03"
    verification: []
    human_judgment: true
    rationale: "Visual map interaction, not asserted by Go tests; deferred to /gsd-verify-work 7."
  - id: D8
    description: "A no-match search shows 'No matches found.' and a failed/unavailable search shows 'Search unavailable, try tapping the map instead.', both inline, with GPS/tap/drag/submit unaffected"
    requirement: "D-04"
    verification: []
    human_judgment: true
    rationale: "Requires simulating an empty-result query and a network failure in a live browser; deferred to /gsd-verify-work 7."
  - id: D9
    description: "A slow response for an earlier query is discarded rather than overwriting the suggestions for a newer query (out-of-order response guard)"
    requirement: "D-04"
    verification: []
    human_judgment: true
    rationale: "The mySeq !== searchSeq structural guard count was confirmed by source assertion during task execution, but simulating an actual out-of-order network race requires a live browser; deferred to /gsd-verify-work 7."

duration: ~20min (task commits 12:47-12:55 IST; total session including reads and verification longer)
completed: 2026-09-29
status: complete
---

# Phase 07 Plan 04: Address Search Box Behavior Summary

**Debounced live-suggestion dropdown, suggestion-tap pin placement through the existing `placeMarker`/`modalMap.setView` path, and D-04's no-match/unavailable inline messages, all wired into `web/static/js/modal.js`, locked in by a new Go contract test reading the embedded file, with `07-VALIDATION.md`'s Per-Task Verification Map fully backfilled and its frontmatter flipped out of draft.**

## Performance

- **Duration:** ~8 min across three task commits (12:47:08 to 12:55:13 IST); full session including reads, greps and verification runs was longer
- **Completed:** 2026-09-29
- **Tasks:** 3
- **Files modified:** 3 (`web/static/js/modal.js`, `web/js_contract_test.go`, `07-VALIDATION.md`)

## Accomplishments

- `modal.js` now owns the address search box end to end: `showSearchStatus`/`clearSearchStatus` for the inline status line, `hideDropdown`/`renderResultRow`/`renderDropdown` for the suggestion list (every string inserted through `Pinalert.setText`, never a markup sink), `onSearchInput`/`runSearch` for the pause-based debounce with a same-query cache and an out-of-order response guard, and `resetSearch` for the six-piece teardown wired into the existing `resetForm` single teardown path
- Shipped constants: `SEARCH_DEBOUNCE_MS = 600`, `MIN_QUERY_RUNES = 3` (matching the server's `minGeocodeQueryRunes`), `SEARCH_RESULT_ZOOM = 16` (matching GPS auto-fill's own zoom), `SEARCH_NO_MATCH_MESSAGE = 'No matches found.'`, and `SEARCH_UNAVAILABLE_MESSAGE = 'Search unavailable, try tapping the map instead.'` (byte identical to the server's `geocodeUnavailableMessage`)
- A suggestion tap calls the file's one existing `placeMarker`/`modalMap.setView` pair (`L.marker(` still appears exactly once in the file), so the pin lands, centres, and stays draggable through the same code path GPS auto-fill uses, with no second confirmation step
- The submit-time location validation message now reads "Set a location by searching, dragging the pin, or allowing location access." (previously named only two of the three ways)
- New `TestLocationSearchUsesTextSinksAndExistingPinPlacement` in `web/js_contract_test.go` reads the embedded `modal.js` bytes and asserts: the three element ids are referenced, exactly one `L.marker(` construction exists, `renderResultRow`'s own body calls `Pinalert.setText(` at least twice plus `placeMarker(` and `modalMap.setView(`, a whole-file zero count on `innerHTML`/`outerHTML`/`insertAdjacentHTML`/`document.write`, the input listener's byte offset is after `function resetForm` (proving top-level wiring, not `initLocation`-scoped), `resetSearch`'s own body (bounded to the next top-level function) contains all five required teardown calls, and the new three-way location message is present exactly once while the old two-way one is gone
- `07-VALIDATION.md`'s Per-Task Verification Map backfilled: every `TBD` replaced with real Task/Plan/Wave values, two placeholder test names from the pre-planning draft corrected to the real names 07-01 actually shipped, 13 automated rows each proven by running its cited command by name with `-v` and `DATABASE_URL` exported and confirming a `--- PASS: <name>` line, and frontmatter flipped to `status: complete`, `wave_0_complete: true`, `nyquist_compliant: true`

## Task Commits

Each task was committed atomically:

1. **Task 1: render the suggestion dropdown, handle suggestion selection, and show the inline messages** - `57baf0c` (feat)
2. **Task 2: debounced input wiring, same-query cache, stale-response guard, and the resetForm teardown** - `323a2a5` (feat)
3. **Task 3: JS contract test for the sink and pin-placement properties, and backfill the validation map** - `bdfe33c` (test)

_Note: no plan-metadata commit yet in this worktree — the orchestrator's merge step performs the final shared-state commit after all wave agents complete. STATE.md and ROADMAP.md are intentionally untouched by this plan's execution._

## Files Created/Modified

- `web/static/js/modal.js` - added the location search element lookups, header note on the third-party sink discipline, `SEARCH_DEBOUNCE_MS`/`MIN_QUERY_RUNES`/`SEARCH_RESULT_ZOOM`/`SEARCH_NO_MATCH_MESSAGE`/`SEARCH_UNAVAILABLE_MESSAGE` constants, `searchDebounceTimer`/`searchSeq`/`searchCache` mutable state, `showSearchStatus`/`clearSearchStatus`/`hideDropdown`/`renderResultRow`/`renderDropdown`/`onSearchInput`/`runSearch`/`resetSearch` functions, the `resetSearch()` call inside `resetForm`, the top-level `locationSearchInput.addEventListener('input', onSearchInput)` wiring, and the updated `validate()` location message
- `web/js_contract_test.go` - added `TestLocationSearchUsesTextSinksAndExistingPinPlacement`
- `.planning/phases/07-address-search-box-for-report-location/07-VALIDATION.md` - backfilled Per-Task Verification Map (16 rows: 13 automated, 3 manual), frontmatter flipped out of draft

## Decisions Made

- Built `renderResultRow`'s primary/secondary line split exactly as `07-RESEARCH.md`'s reference implementation and `07-02-SUMMARY.md`'s shipped class names specify: primary line falls back to `display_name` when Nominatim's `name` field is absent, secondary line (the `.location-search-result-secondary` class) only renders when `name` was present
- Kept `runSearch`'s fetch/error handling byte-for-byte in the same shape as `votes.js`'s `castVote` (`res.json().catch(() => ({}))` then `!res.ok` then throw with `fieldMessage`), so this codebase has one error idiom for its two client-side fetches rather than two
- Used the project's `windowAfter` test helper (already defined in `web/js_contract_test.go`) to bound both the `renderResultRow` and `resetSearch` regions in the new contract test, rather than adding a second parsing helper

## Deviations from Plan

None - plan executed exactly as written. All three tasks' actions, acceptance criteria greps, and the contract test's seven numbered assertions match the plan's `<action>` blocks precisely. The two placeholder test names the plan's own text flagged as wrong in the pre-planning `07-VALIDATION.md` draft (`TestClient_Search_RateLimited`, `TestGeocode_UpstreamTimeout`) were corrected to the real names 07-01 shipped (`TestClient_Search_SerializesConcurrentCalls`, `TestGeocode_UpstreamFailureReturnsFriendlyError`), exactly as the plan's action instructed.

## Issues Encountered

None. `node --check web/static/js/modal.js`, `go test ./web/ -count=1`, `go test ./... -short -count=1`, and `make test` all passed cleanly after every task, with no flakiness observed in this session (the pre-existing shared-Postgres flakiness logged in `deferred-items.md` by earlier plans in this phase did not reproduce during this plan's verification runs).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 07's four plans are all committed. The three D-01/D-03/D-04 manual browser UAT items (live debounced suggestions with cache reuse, suggestion-tap pin placement and drag, and the no-match/unavailable inline messages with GPS/tap/drag/submit unaffected) were **not run in this session** (no live browser available to this agent) and remain deferred to `/gsd-verify-work 7`, exactly as `07-VALIDATION.md`'s Manual-Only Verifications section and this plan's `<human-check>` block specify. `07-VALIDATION.md`'s `nyquist_compliant: true` flag reflects that every task across all four plans carried a passing `<automated>` verify, not that the manual UAT items were completed; those three rows in the Per-Task Verification Map remain `⬜ pending` on purpose.

No blockers for the phase's end-of-phase UAT pass.

## Self-Check: PASSED

- FOUND: web/static/js/modal.js (modified)
- FOUND: web/js_contract_test.go (modified)
- FOUND: .planning/phases/07-address-search-box-for-report-location/07-VALIDATION.md (modified)
- FOUND commit: 57baf0c (Task 1)
- FOUND commit: 323a2a5 (Task 2)
- FOUND commit: bdfe33c (Task 3)

---
*Phase: 07-address-search-box-for-report-location*
*Completed: 2026-09-29*
