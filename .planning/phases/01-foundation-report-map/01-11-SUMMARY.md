---
phase: 01-foundation-report-map
plan: 11
subsystem: web-frontend
tags: [maplibre-gl, leaflet, vendor-assets, sri, contract-test, stack-docs]

requires:
  - phase: 01-foundation-report-map
    provides: web/templates/index.html.tmpl with a pinned, SRI-checked Leaflet script tag and web/js_contract_test.go / web/css_contract_test.go as the embed.FS-based contract-test pattern this plan extends
provides:
  - "MapLibre GL 5.24.0 and the Leaflet<->MapLibre bridge 0.1.4 loaded as pinned, integrity-checked classic <script> tags in strict dependency order (Leaflet -> renderer -> bridge -> app modules)"
  - "MapLibre stylesheet link and a crossorigin preconnect hint for tiles.openfreemap.org"
  - "TestVendorMapScriptsLoadInDependencyOrder regression test locking vendor tag order, pinning, integrity, crossorigin, and absence of defer/async on the three vendor tags"
  - "CLAUDE.md stack documentation updated at all three locations that described a raster-only OpenStreetMap map"
affects: [01-12-vector-basemap-swap]

tech-stack:
  added:
    - "maplibre-gl@5.24.0 (CDN script tag, UMD/browser-global build — no package manager involved)"
    - "@maplibre/maplibre-gl-leaflet@0.1.4 (CDN script tag, Leaflet<->MapLibre bridge)"
  patterns:
    - "Vendor script load order as a Go-test-enforced correctness constraint, not a comment: the bridge reads two other globals at script-evaluation time, so tag order is asserted via string-position comparison over the embedded template"
    - "Tag-window extraction (last '<' before a src, first '>' after it) to scope per-tag attribute assertions without cross-contaminating a neighboring tag's attributes"

key-files:
  created:
    - web/template_contract_test.go
  modified:
    - web/templates/index.html.tmpl
    - .claude/CLAUDE.md

key-decisions:
  - "Recomputed all five vendor integrity hashes (two pre-existing Leaflet, three new) from the pinned URLs' own bytes at implementation time, cross-checked byte-for-byte against 01-11-RESEARCH.md's cross-check values and against the template's pre-existing Leaflet hashes — no hash was read from unpkg's own metadata endpoint or transcribed from memory."
  - "Deferred the plan's <human-check> browser verification step (confirming typeof L.maplibreGL === 'function' and typeof maplibregl === 'object' in a live browser) — this execution runs as an autonomous worktree agent with no browser or physical display access, and cmd/server requires a live DATABASE_URL this worktree does not have provisioned. This mirrors the established precedent in 01-10-SUMMARY.md for the same project config (workflow.human_verify_mode: end-of-phase)."

patterns-established:
  - "Vendor CDN assets are order-locked and integrity-locked by a dedicated template_contract_test.go, separate from the CSS/JS contract tests, following the same TemplatesFS/StaticFS embed.FS-read pattern"

requirements-completed: []

coverage:
  - id: D1
    description: "Template loads maplibre-gl@5.24.0 and @maplibre/maplibre-gl-leaflet@0.1.4 as pinned, SRI-checked, non-deferred script tags in strict dependency order after Leaflet and before the app's four deferred modules"
    verification:
      - kind: unit
        ref: "web/template_contract_test.go#TestVendorMapScriptsLoadInDependencyOrder"
        status: pass
      - kind: other
        ref: "shell gate: line-number comparison of leaflet.js < maplibre-gl.js < leaflet-maplibre-gl.js < app.js in web/templates/index.html.tmpl"
        status: pass
    human_judgment: false
  - id: D2
    description: "All five vendor integrity hashes (two pre-existing Leaflet, three new) independently recomputed from the pinned URLs' own bytes and matched against the template"
    verification:
      - kind: other
        ref: "shell gate: curl + openssl dgst -sha256 recomputation against each of the five pinned URLs, grep-matched into web/templates/index.html.tmpl"
        status: pass
    human_judgment: false
  - id: D3
    description: "MapLibre stylesheet link and crossorigin preconnect hint for tiles.openfreemap.org present in <head>"
    verification:
      - kind: unit
        ref: "web/template_contract_test.go#TestVendorMapScriptsLoadInDependencyOrder"
        status: pass
    human_judgment: false
  - id: D4
    description: "No JavaScript file or stylesheet changed; both maps still render OpenStreetMap raster tiles exactly as before this plan"
    verification:
      - kind: other
        ref: "shell gate: git diff --quiet HEAD -- web/static (task-scoped, confirmed clean)"
        status: pass
    human_judgment: false
  - id: D5
    description: "CLAUDE.md accurately describes the map stack at all three identified locations (tech-stack bullet, Core Technologies table, What-NOT-to-Use table + Sources)"
    verification:
      - kind: other
        ref: "shell gates: grep for maplibre/5.24.0/0.1.4/OpenFreeMap presence, absence of stale 'pairs with free OSM tile servers' clause, What-NOT-to-Use row presence, Sources entry presence — all in .claude/CLAUDE.md"
        status: pass
    human_judgment: false
  - id: D6
    description: "Browser runtime confirmation that both vendor globals (L.maplibreGL, maplibregl) are defined with a clean console and a visually unchanged map"
    verification: []
    human_judgment: true
    rationale: "Requires a live browser and a running server with a provisioned DATABASE_URL, neither available to this autonomous worktree agent (no display, no DB). Per workflow.human_verify_mode: end-of-phase, deferred to /gsd-verify-work 01, matching the precedent set in 01-10-SUMMARY.md's identical circumstance."

duration: ~20min
completed: 2026-09-08
status: complete
---

# Phase 01 Plan 11: MapLibre GL Vendor Assets Summary

**Loaded MapLibre GL 5.24.0 and the Leaflet-MapLibre bridge 0.1.4 as pinned, SRI-checked, order-locked vendor `<script>` tags — no JavaScript or CSS changed, both maps still render OpenStreetMap raster tiles unchanged until plan 01-12 consumes the new globals.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-09-08
- **Tasks:** 2 (both completed)
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- `web/templates/index.html.tmpl` now loads `maplibre-gl@5.24.0` and `@maplibre/maplibre-gl-leaflet@0.1.4` as classic `<script>` tags, placed after the existing Leaflet script and before the app's four deferred modules, each carrying a sha256 `integrity` attribute recomputed from the pinned URL's own bytes and `crossorigin="anonymous"`, with no `defer`/`async` on any of the three vendor tags.
- Added the pinned MapLibre stylesheet link and a `crossorigin` preconnect hint for `tiles.openfreemap.org`, both in `<head>`.
- New `web/template_contract_test.go` with `TestVendorMapScriptsLoadInDependencyOrder`, which reads the shipped template out of the embedded `TemplatesFS` and fails the build on any vendor-tag reordering, missing integrity/crossorigin attribute, a `defer`/`async` attribute on a vendor tag, or an app module hoisted above the bridge.
- `.claude/CLAUDE.md` updated at all three locations the research identified: the tech-stack constraint bullet, the Core Technologies table (Leaflet row rewritten, two new rows added), and the What-NOT-to-Use table plus Sources list.

## Task Commits

Each task was committed atomically:

1. **Task 1: Load the MapLibre renderer and the Leaflet bridge as pinned, integrity-checked vendor tags in dependency order, locked by a template contract test** — `403260c` (feat)
2. **Task 2: Update the project stack documentation at the three locations that still describe a raster-only OpenStreetMap map** — `e809a29` (docs)

_No plan metadata commit was made from within this worktree — the orchestrator commits this SUMMARY.md, along with REQUIREMENTS.md if it changes, after merging this worktree's branch._

## Files Created/Modified

- `web/templates/index.html.tmpl` — added the MapLibre stylesheet link, the `tiles.openfreemap.org` preconnect hint, and the two new vendor `<script>` tags between the existing Leaflet script and `/static/js/app.js`. Both pre-existing Leaflet integrity hashes left byte-identical.
- `web/template_contract_test.go` (new, 159 lines vs. the plan's `min_lines: 85` estimate) — `TestVendorMapScriptsLoadInDependencyOrder`.
- `.claude/CLAUDE.md` — tech-stack bullet, Core Technologies table (Leaflet row + two new rows), What-NOT-to-Use table row, Sources list entries.

## Decisions Made

- **Hash provenance discipline followed exactly as specified:** all five vendor integrity hashes (the two pre-existing Leaflet ones as a sanity check on the method, plus the three new ones) were recomputed with `curl -sL "<url>" | openssl dgst -sha256 -binary | openssl base64` at implementation time. The Leaflet JS hash reproduced `20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo=` byte-for-byte before any new hash was trusted. All three new hashes matched `01-11-RESEARCH.md`'s cross-check values exactly — no mismatch to report, no hash was read from unpkg's `?meta` endpoint or invented.
- **Human-check deferred, not skipped or faked.** The plan's Task 1 `<human-check>` step asks for a live-browser confirmation (`typeof L.maplibreGL === 'function'`, `typeof maplibregl === 'object'`, clean console, visually unchanged map) with the server running. This worktree agent has no browser/display access and no provisioned `DATABASE_URL` to start `cmd/server` against. Per `.planning/config.json`'s `workflow.human_verify_mode: "end-of-phase"`, human verification is deliberately batched rather than performed per-plan, and this plan carries `autonomous: true` with no `checkpoint:*` task type — so this is an expected, non-blocking gap, mirroring the identical precedent documented in `01-10-SUMMARY.md`. All automated verification for this step is complete; only the visual/runtime judgment remains outstanding.

## Deviations from Plan

None — plan executed exactly as written. Both new integrity hashes matched the research doc's cross-check values on the first computation; no mismatch, no re-derivation needed.

## Issues Encountered

None. The sandbox's Bash tool rejected a small number of multi-command shell one-liners (loops, semicolon chains) as "too complex to verify... stays inside the worktree" — these were the verify-gate loop constructs from the plan's `<verify>` block. Each was manually unrolled into discrete single-purpose commands (one `grep`/`curl` per vendor asset) with identical net verification coverage; no gate was skipped, only the shell syntax was adapted to the sandbox's constraints.

## Outstanding: Human Visual/Runtime Verification (not blocking, not performed by this agent)

**Post-merge update (orchestrator, same session):** After merging this plan, the decisive part of the
human-check (`typeof L.maplibreGL` / `typeof maplibregl`) was independently verified — not in a real
browser (none was available in this session either), but via actual code execution: a Node.js `vm`
sandbox with browser-like globals stubbed in (document/navigator/screen/TextDecoder/etc.) fetched and
executed the exact three pinned vendor script URLs, live, from unpkg, in the template's exact dependency
order. Result:

```json
{ "typeof L": "object", "typeof L.maplibreGL": "function", "typeof maplibregl": "object" }
```

This confirms the bridge's browser build correctly attaches its entry point to Leaflet's namespace and
that both vendor globals resolve — the specific claim step 6 below could not otherwise confirm without a
real browser. This does NOT substitute for confirming the page's console is clean and the map is visually
unchanged in a real browser with a real DOM/WebGL context — those two items remain genuinely outstanding
and are unchanged from the list below.

Concrete steps for whoever runs `/gsd-verify-work 01` next:

1. Ensure `DATABASE_URL`, `SESSION_SECRET` (or `ENV=development`) are set, then `make run` (or `go run ./cmd/server`).
2. Hard-reload the page with DevTools open and the cache disabled.
3. In the Console, evaluate `typeof L.maplibreGL` — expect exactly `function`. If `undefined`, check the Console for an integrity-hash mismatch message and the Network tab for a 404 on one of the three vendor scripts. *(Independently confirmed via sandboxed execution above — this browser confirmation is now a formality, not a live risk.)*
4. Evaluate `typeof maplibregl` — expect `object`. *(Same — independently confirmed above.)*
5. Confirm the Console shows no errors at all on load.
6. Confirm the map looks and behaves exactly as before this plan: same OpenStreetMap raster tiles, same pins, same attribution box, the `+` button still opens the modal with its own small map. Nothing about the rendered map should have changed yet — the new vendor globals are loaded but unused until plan 01-12 lands.

This plan closes no UAT test — Tests 1 and 8 in `01-UAT.md` already passed; Tests 2, 3, 4, 5, 6, 7, 9 and 10 remain `[pending]` and are re-entered via `/gsd-verify-work 01` after plan 01-12 lands. `01-UAT.md` was deliberately left untouched by this plan.

## User Setup Required

None — no external service configuration required. OpenFreeMap needs no signup, no API key, no billing.

## Requirements

FOUND-02 and FOUND-04 are **not** newly marked complete by this plan — both were already `Complete` in `.planning/REQUIREMENTS.md` before this plan ran (Phase 1, per the traceability table), and the plan's own objective states it "closes no UAT test" and prepares the rendering layer without changing behavior. No `requirements.mark-complete` call was made; this is intentional, matching the identical precedent in `01-10-SUMMARY.md`.

## Next Phase Readiness

- The vendor layer plan 01-12 needs is now in place, pinned, integrity-checked, and order-locked by a regression test — 01-12 can proceed as a pure JavaScript change (`L.tileLayer(...)` -> `L.maplibreGL(...)` in `map.js` and `modal.js`) with no remaining infrastructure risk.
- Outstanding: the live-browser human-check above should be run before or alongside 01-12's own verification, since it is the cheapest point at which a wrong hash or a 404'd CDN path would surface (`typeof L.maplibreGL` would still read `undefined` today, since nothing calls it yet — the check here is that the globals exist, not that they're used).

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-08*

## Self-Check: PASSED

- FOUND: `web/templates/index.html.tmpl`
- FOUND: `web/template_contract_test.go`
- FOUND: `.claude/CLAUDE.md`
- FOUND: commit `403260c` (Task 1)
- FOUND: commit `e809a29` (Task 2)
- FOUND: commit `e9dba13` (this SUMMARY.md)
