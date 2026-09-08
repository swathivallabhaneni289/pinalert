---
phase: 01-foundation-report-map
plan: 12
subsystem: web-frontend
tags: [maplibre-gl, leaflet, vector-basemap, webgl-fallback, contract-test]

requires:
  - phase: 01-foundation-report-map
    provides: web/templates/index.html.tmpl loading MapLibre GL 5.24.0 and the Leaflet<->MapLibre bridge 0.1.4 as pinned, SRI-checked vendor <script> tags in dependency order (plan 01-11), so L.maplibreGL exists by the time map.js/modal.js run
provides:
  - "Both Leaflet map instances (the primary map and the report modal's own separate instance) render the OpenFreeMap liberty vector basemap through MapLibre GL, gated by a private hasVectorBasemap() capability probe per file"
  - "A byte-identical fallback to the OpenStreetMap raster tile layer plan 01-10 shipped when either vendor global is missing or no WebGL2 context can be obtained"
  - "Each map declares its own maxZoom/minZoom/maxBounds so the vector layer's lack of a registered zoom limit cannot leave pinch-zoom unbounded"
  - "A fallback-path maxZoom re-derivation (map.setMaxZoom(rasterLayer.options.maxZoom)) so a retina-adjusted raster fallback never reaches a zoom at which it renders no tiles"
  - "web/js_contract_test.go rewritten around the two-branch shape: TestBasemapBranchesPointAtCorrectHosts, TestMapsDeclareOwnZoomBounds, TestCapabilityProbeGatesBasemapChoice, with shared readOptionValue/windowAfter package-level helpers"
affects: []

tech-stack:
  added: []
  patterns:
    - "Per-file capability probe (hasVectorBasemap) gating a vector-vs-raster branch, duplicated per module rather than shared, matching this codebase's established inline-literal convention so the static Go contract test can read option values back out of each file's own construction call"
    - "Fallback-layer maxZoom re-derivation: after adding a byte-identical legacy layer inside the else branch, read its own post-construction maxZoom back onto the map object rather than hardcoding a second value"

key-files:
  created: []
  modified:
    - web/static/js/map.js
    - web/static/js/modal.js
    - web/js_contract_test.go

key-decisions:
  - "Followed the plan's explicit instruction to keep every option value (style URL, attribution strings, zoom numbers, map options object) as an inline literal duplicated across map.js and modal.js rather than factoring into shared constants — this is load-bearing for the rewritten contract test, which reads values back out of each construction call's own text."
  - "Placed the fallback's map.setMaxZoom(rasterLayer.options.maxZoom) re-derivation line immediately after rasterLayer.addTo(map/modalMap) but outside the raster construction's own '.addTo(' search window, matching the plan's requirement that the contract test's raster window remain unambiguously scoped to the layer's own options."
  - "Deferred the Task 2 human-check (live-browser seven-step verification, including the WebGL-disabled fallback run) — this execution runs as an autonomous worktree agent with no browser or display access and no provisioned DATABASE_URL to start cmd/server. This mirrors the established precedent in 01-11-SUMMARY.md for the same project config (workflow.human_verify_mode: end-of-phase)."

patterns-established:
  - "Mutation-then-restore verification discipline for a rewritten contract test: before committing, temporarily break the code in each of the ways the test claims to catch, confirm the specific test fails naming the specific file, then restore via `git checkout --` and re-confirm a clean diff before moving on. Recorded here as a durable practice for any future rewrite of a static-inspection Go test in this repo."

requirements-completed: []

coverage:
  - id: T1-vector-and-fallback-construction
    description: "Both map.js and modal.js construct the vector basemap via L.maplibreGL when hasVectorBasemap() is true, and the byte-identical raster fallback via L.tileLayer otherwise"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestBasemapBranchesPointAtCorrectHosts"
        status: pass
      - kind: other
        ref: "shell gates: per-file token counts (L.maplibreGL(, L.tileLayer(, hasVectorBasemap(, getContext('webgl2'), customAttribution, detectRetina), style/raster URL presence, retina/maxZoom/bounds content gates — all in web/static/js/map.js and modal.js"
        status: pass
    human_judgment: false
  - id: T1-zoom-bounds-on-map
    description: "Each L.map() call declares maxBounds/maxBoundsViscosity/minZoom/maxZoom on its own options, and the fallback branch re-derives the map's maxZoom from the raster layer's post-construction value"
    verification:
      - kind: unit
        ref: "web/js_contract_test.go#TestMapsDeclareOwnZoomBounds"
        status: pass
      - kind: other
        ref: "shell gates: maxBoundsViscosity/minZoom:1/maxZoom:19/setMaxZoom( presence checks in both files"
        status: pass
    human_judgment: false
  - id: T1-attribution-single-and-correct
    description: "Attribution credits OpenFreeMap, OpenMapTiles and OpenStreetMap on the vector path with rel=noopener, the fallback keeps its own byte-identical attribution string per file, and exactly one plain attribution option exists per file (no doubled credit box)"
    verification:
      - kind: other
        ref: "shell gates: contributors=1 and openstreetmap.org/copyright=2 in map.js; copy; OpenStreetMap contributors present and openstreetmap.org/copyright=1 in modal.js; openfreemap.org/openmaptiles.org/rel=noopener presence; exactly one plain attribution: option per file"
        status: pass
    human_judgment: false
  - id: T1-no-stylesheet-or-out-of-scope-file-touched
    description: "No stylesheet, template, or other JavaScript/Go source file changed by this plan"
    verification:
      - kind: other
        ref: "shell gates: git diff --quiet HEAD -- web/static/css; git diff --quiet HEAD -- web/templates web/static/js/app.js web/static/js/feed.js web/css_contract_test.go web/template_contract_test.go; git status --porcelain -- web/static/js file count <= 2"
        status: pass
    human_judgment: false
  - id: T2-three-tests-rewritten
    description: "web/js_contract_test.go holds exactly three test functions covering the two branches, the maps' own zoom bounds, and the capability-probe gate, with shared helpers and stripCSSComments/StaticFS reuse"
    verification:
      - kind: unit
        ref: "go test ./web/ -v (TestBasemapBranchesPointAtCorrectHosts, TestMapsDeclareOwnZoomBounds, TestCapabilityProbeGatesBasemapChoice all pass, alongside TestModalBackdropHiddenGuard, TestPrimaryMapHasResolvedHeight, TestVendorMapScriptsLoadInDependencyOrder)"
        status: pass
      - kind: other
        ref: "shell gates: grep for stripCSSComments/StaticFS presence, exactly 3 func Test, task-scoped git diff isolation"
        status: pass
    human_judgment: false
  - id: T2-four-bite-directions
    description: "The rewritten tests were proven to bite in all four directions before committing: pass on correct code, and a named failure each for a re-pointed vector style URL, a deleted raster fallback branch, and a removed map maxZoom"
    verification:
      - kind: other
        ref: "Manually run: (1) go test ./web/ -v on correct code — all pass. (2) modal.js style URL mutated to https://evil.example.com/styles/liberty — TestBasemapBranchesPointAtCorrectHosts fails naming static/js/modal.js. (3) map.js raster fallback branch deleted (unconditional vector construction) — TestBasemapBranchesPointAtCorrectHosts fails naming static/js/map.js with 'expected a raster fallback construction... found none'. (4) modal.js map's maxZoom option removed — TestMapsDeclareOwnZoomBounds fails naming static/js/modal.js. Each mutation restored via git checkout -- and confirmed clean (empty git diff --stat) before the next mutation."
        status: pass
    human_judgment: false
  - id: T2-human-check
    description: "Live-browser seven-step verification of the vector path, attribution, layering, panning feel, the modal map, the narrow-screen toggle, and the WebGL-disabled fallback path"
    verification: []
    human_judgment: true
    rationale: "Requires a live browser/display and a running server with a provisioned DATABASE_URL, neither available to this autonomous worktree agent. Per workflow.human_verify_mode: end-of-phase, deferred to /gsd-verify-work 01, matching the identical precedent in 01-11-SUMMARY.md. All automated verification for this plan is complete; only the visual/runtime judgment remains outstanding."

duration: ~13min
completed: 2026-09-08
status: complete
---

# Phase 01 Plan 12: MapLibre GL Vector Basemap Swap Summary

**Both Leaflet map instances now render the OpenFreeMap liberty vector basemap through a private per-file WebGL2 capability probe, falling back to the byte-identical OpenStreetMap raster layer plan 01-10 shipped — with the maximum zoom moved onto each map's own options and re-derived from the fallback layer's post-construction value so a retina display can never reach a blank zoom.**

## Performance

- **Duration:** ~13 min
- **Completed:** 2026-09-08
- **Tasks:** 2 (both completed)
- **Files modified:** 3 (0 created, 3 modified)

## Accomplishments

- `web/static/js/map.js` and `web/static/js/modal.js` each gained a private `hasVectorBasemap()` capability probe (requiring `L.maplibreGL` as a function, the `maplibregl` global to be defined, and a WebGL2 canvas context) that branches the basemap construction between `L.maplibreGL({ style: ..., attributionControl: { customAttribution: ... } })` and the exact `L.tileLayer(...)` raster call each file shipped before this plan.
- Both `L.map()` calls now carry `maxBounds`/`maxBoundsViscosity`/`minZoom: 1`/`maxZoom: 19` on their own options — the maintainers' bridge boilerplate plus this project's own `maxZoom` relocation, since the vector layer registers no zoom limit of its own.
- The raster fallback branch in both files re-derives the map's `maxZoom` from the layer's own post-construction value (`map.setMaxZoom(rasterLayer.options.maxZoom)`), correcting the case where Leaflet's retina branch decrements the layer's own ceiling on a high-density display.
- `web/js_contract_test.go` was rewritten from one test (`TestTileLayersRequestRetinaTiles`) into three: `TestBasemapBranchesPointAtCorrectHosts`, `TestMapsDeclareOwnZoomBounds`, and `TestCapabilityProbeGatesBasemapChoice`, sharing two new package-level helpers (`readOptionValue`, `windowAfter`) and still reusing `stripCSSComments`.
- Every option value (style URL, attribution strings, zoom numbers) stays an inline literal duplicated per file, per the plan's explicit load-bearing requirement — no shared constants were introduced.

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace both raster basemaps with a capability-gated vector basemap that falls back to the existing raster layer** — `418d429` (feat)
2. **Task 2: Rewrite the JavaScript contract test for a two-branch basemap, and prove it bites in both directions** — `adc8ddf` (test)

_No plan metadata commit was made from within this worktree — the orchestrator commits this SUMMARY.md, along with STATE.md/ROADMAP.md/REQUIREMENTS.md if they change, after merging this worktree's branch._

## Files Created/Modified

- `web/static/js/map.js` (197 -> 259 lines) — added `hasVectorBasemap()`, the four-option map construction, and the vector/raster branch inside `PinalertMap.init()`. Every other function (`centerOnVisitor`, `buildBadgeElement`, `buildPopupContent`, `upsertMarker`, `render`, `flyTo`, `highlight`) is untouched.
- `web/static/js/modal.js` (576 -> 638 lines) — added the modal's own private `hasVectorBasemap()` and the same branch inside `initLocation()`'s `if (!modalMap)` guard. `placeMarker`, the draggable marker/dragend handler, the geolocation success/failure paths, `showLocationDenied`, tap-to-place, both `invalidateSize()` calls and their timing, `resetForm`, `openModal`, and the entire form/validation/shelter/discard logic are untouched.
- `web/js_contract_test.go` (176 -> 334 lines) — full rewrite per Task 2, described above.

## Decisions Made

- **Inline literals preserved per file, not factored into shared constants** — followed exactly as the plan required. The rewritten contract test's `readOptionValue`/`windowAfter` helpers depend on reading option values back out of each file's own construction-call text; a shared constant would have made both new tests fail against otherwise-correct code.
- **`setMaxZoom` re-derivation placed outside the raster window** — the line `map.setMaxZoom(rasterLayer.options.maxZoom)` sits after `rasterLayer.addTo(map)` (the raster branch's own `.addTo(` terminator), so `TestBasemapBranchesPointAtCorrectHosts`'s raster window reads unambiguously the layer's own `maxZoom`, not the map's post-correction value. Documented as a non-obvious ordering dependency in both the code comment and the test's doc comment.
- **Task 2 human-check deferred, not skipped or faked.** The plan's Task 2 `<human-check>` step asks for a seven-step live-browser confirmation, including disabling WebGL to observe the fallback path. This worktree agent has no browser/display access and no provisioned `DATABASE_URL` to start `cmd/server`. Per `.planning/config.json`'s `workflow.human_verify_mode: "end-of-phase"`, this is deliberately batched rather than performed per-plan, mirroring the identical precedent in `01-11-SUMMARY.md`. All automated verification is complete; only the visual/runtime judgment remains outstanding.

## Deviations from Plan

None — plan executed exactly as written. Every automated verify gate in both tasks passed on the first implementation attempt; no auto-fix (Rules 1-3) or architectural escalation (Rule 4) was needed.

## Bite-Direction Verification (Task 2, all four run and recorded)

1. **Correct code passes.** `go test ./web/ -v` — all six tests in the package pass, including the three new/rewritten ones.
2. **Re-pointed vector style URL.** Temporarily changed `modal.js`'s vector `style` option to `https://evil.example.com/styles/liberty`. `TestBasemapBranchesPointAtCorrectHosts` failed: `static/js/modal.js: vector basemap style must be exactly "https://tiles.openfreemap.org/styles/liberty", found "https://evil.example.com/styles/liberty"`. Restored via `git checkout -- web/static/js/modal.js`; confirmed clean.
3. **Deleted raster fallback branch.** Temporarily collapsed `map.js`'s `init()` to always construct the vector layer unconditionally (removing the `if/else` and the entire raster branch). `TestBasemapBranchesPointAtCorrectHosts` failed: `static/js/map.js: expected a raster fallback construction in this file but found none`. Restored via `git checkout -- web/static/js/map.js`; confirmed clean.
4. **Removed map maxZoom.** Temporarily deleted the `maxZoom: 19` line from `modal.js`'s `L.map()` options object. `TestMapsDeclareOwnZoomBounds` failed: `static/js/modal.js: the map's own construction is missing maxZoom — the vector basemap layer registers no zoom limit of its own, so without this option pinch-zoom becomes unbounded`. Restored via `git checkout -- web/static/js/modal.js`; confirmed clean.

Each restore was verified with an empty `git diff --stat` and empty `git status --porcelain` before proceeding to the next mutation, and the full suite was re-run clean (`go test ./web/ -v -count=1`) after the last restore.

## Issues Encountered

None. All shell-gate verification commands ran as discrete, unrolled single-purpose commands (this sandbox rejects multi-command loops/chains as "too complex to verify... stays inside the worktree"), matching the same adaptation documented in `01-11-SUMMARY.md`; no gate was skipped, only the shell syntax was adapted.

## Outstanding: Human Visual/Runtime Verification (not blocking, not performed by this agent)

The plan's Task 2 `<human-check>` is a seven-step live-browser walkthrough that this autonomous worktree agent cannot perform (no browser/display, no provisioned `DATABASE_URL`). Concrete steps for whoever runs `/gsd-verify-work 01` next:

1. Ensure `DATABASE_URL`, `SESSION_SECRET` (or `ENV=development`) are set, then `make run` (or `go run ./cmd/server`).
2. **Vector path:** hard-reload with DevTools open, cache disabled. Confirm the OpenFreeMap liberty style renders (visibly different from the old OSM raster look), console shows no errors, and Network tab shows requests to `tiles.openfreemap.org`.
3. **Attribution:** confirm the credit box shows OpenFreeMap, OpenMapTiles and OpenStreetMap as three links, exactly once — no doubled credit box.
4. **Layering:** confirm pins/popups/FAB still draw above the basemap, and a popup stays anchored to its pin through a couple of zoom notches.
5. **Panning feel:** drag the map; slight lag against a known upstream bridge issue is expected/acceptable, obvious stutter is worth reporting.
6. **Modal map:** open the report modal (`+`), confirm its own map draws the vector basemap correctly on first and second open, and drag the marker.
7. **Narrow-screen toggle:** shrink below 900px, toggle list <-> map, confirm the basemap is still drawn (not blank).
8. **WebGL-disabled fallback (the other half of this plan):** disable WebGL via DevTools "Show Rendering" -> "Disable WebGL" (or a browser with WebGL off), hard-reload. Confirm the original OpenStreetMap raster map renders — plain, working, with its own OSM credit and no OpenFreeMap credit — and tiles are still drawn at the deepest reachable zoom (this specifically exercises the `setMaxZoom` correction). Re-enable WebGL, reload, confirm the vector basemap returns.

This plan closes no UAT test. Tests 1 and 8 in `01-UAT.md` already passed; Tests 2, 3, 4, 5, 6, 7, 9 and 10 remain `[pending]` and are re-entered via `/gsd-verify-work 01`. Tests 2 and 5 are map-behavioural and should specifically be re-run against this new basemap. `01-UAT.md` was deliberately left untouched by this plan.

## User Setup Required

None — no external service configuration required. OpenFreeMap needs no signup, no API key, no billing (unchanged from plan 01-11).

## Requirements

FOUND-02 and FOUND-04 are **not** newly marked complete by this plan — both were already `Complete` in `.planning/REQUIREMENTS.md` (Phase 1). This plan is a rendering-quality upgrade to an already-passing capability, closing no new requirement or UAT test. No `requirements.mark-complete` call was made, matching the identical precedent in `01-11-SUMMARY.md`.

## Next Phase Readiness

- The basemap migration described across plans 01-11 and 01-12 is now fully landed: vendor assets pinned and integrity-checked (01-11), basemap construction swapped with a required WebGL fallback (01-12).
- The next actionable step for this phase is the human verification batch via `/gsd-verify-work 01`, which should now include: (a) 01-11's still-outstanding browser confirmation of the vendor globals, (b) 01-12's seven/eight-step human-check above, and (c) re-running UAT Tests 2 and 5 against the new basemap.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-08*

## Self-Check: PASSED

- FOUND: `web/static/js/map.js`
- FOUND: `web/static/js/modal.js`
- FOUND: `web/js_contract_test.go`
- FOUND: commit `418d429` (Task 1)
- FOUND: commit `adc8ddf` (Task 2)
