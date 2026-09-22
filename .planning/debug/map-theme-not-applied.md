---
status: diagnosed
trigger: "map-theme-not-applied: The map's basemap does not visibly change when the user switches the app's Light/Dark theme; it always renders the same light-colored style."
created: 2026-09-22T07:46:53Z
updated: 2026-09-22T08:05:00Z
---

## Current Focus

hypothesis: CONFIRMED — map.js hardcodes a single light MapLibre vector style URL at init time and never reads document.documentElement's data-theme attribute, and theme.js (by explicit, test-locked design) never notifies any other module of a mode change — so the map basemap cannot react to the theme toggle at all, regardless of mode.
test: Read map.js and theme.js in full; grepped for any CustomEvent/MutationObserver/matchMedia wiring between the two modules; read theme_contract_test.go to confirm theme.js's no-app-shell-dependency contract is test-enforced, not incidental; confirmed OpenFreeMap's dark style endpoint and the maplibre-gl-leaflet setStyle API via web search.
expecting: n/a — root cause confirmed, goal is find_root_cause_only.
next_action: none — return ROOT CAUSE FOUND to caller.

## Symptoms

expected: The map basemap visibly matches the app's current Light/Dark theme (not just the surrounding chrome).
actual: User reported (2026-09-21/22): "even if i change it to the dark mode the map doesn't change much and same goes to the um when i change it to the light mode."
errors: None reported.
reproduction: Test 10 in UAT (.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UAT.md). Toggle the theme control (person icon > Theme) between Light and Dark on the main map page and observe the basemap.
started: Discovered during Phase 2 UAT round 3, 2026-09-22.

## Eliminated

(none — root cause found on first pass, matching the orchestrator's preliminary diagnosis)

## Evidence

- timestamp: 2026-09-22T07:50:00Z
  checked: web/static/js/map.js (full file, current worktree state)
  found: init() (line 52-107) constructs the basemap unconditionally. hasVectorBasemap() (line 38-50) gates vector vs raster capability only, not theme. Vector path (line 72-81): `L.maplibreGL({ style: 'https://tiles.openfreemap.org/styles/liberty', ... })` — hardcoded literal string, no variable, no read of data-theme, no matchMedia call anywhere in the file. Raster fallback (line 83-87): standard OSM raster tile server, also hardcoded, no dark variant referenced. The constructed layer is never assigned to a module-level variable — `L.maplibreGL({...}).addTo(map)` is a one-off expression, so there is currently no reference held anywhere in the module that a later theme-change handler could call `.setStyle()` on.
  implication: Confirms orchestrator's preliminary diagnosis exactly. No theme-awareness exists in this file at all, on init or afterward.

- timestamp: 2026-09-22T07:50:30Z
  checked: web/static/js/theme.js (full file, current worktree state)
  found: applyMode() (line 64-70) is the only place a value reaches the DOM: one setAttribute('data-theme', mode) call, one removeAttribute('data-theme') call, both on document.documentElement. No CustomEvent is dispatched anywhere in the file. wireControl()'s click handler (line 113-119) calls applyMode/persistMode/re-labels the control text and does nothing else — no call into any other module, no window/document event fired.
  implication: theme.js is provably a dead end for any listener — there is no signal of any kind (event, callback, global function) that map.js (or anything else) could currently observe to learn a mode change happened.

- timestamp: 2026-09-22T07:51:00Z
  checked: web/theme_contract_test.go, specifically TestThemeModuleHasNoAppShellDependency (line 115-127)
  found: This test asserts `!strings.Contains(text, "Pinalert")` against the raw theme.js source — i.e. theme.js is deliberately, contractually forbidden from referencing the app shell/global client store, because it also loads unmodified on the login gate and verify-outcome pages where no other app script exists (confirmed those two templates also load only theme.js, not map.js/app.js — see web/templates/login_gate.html.tmpl:9, web/templates/verify_outcome.html.tmpl:10, web/templates/profile.html.tmpl:11).
  implication: This is not an oversight to "just fix" by having theme.js call into Pinalert.* or dispatch a `Pinalert`-referencing event — that would fail a build-gating test. Any signal theme.js emits toward map.js must be a generic, unnamespaced-to-"Pinalert" mechanism (a lowercase-named CustomEvent, or no change to theme.js at all — e.g. map.js observing the DOM directly via MutationObserver). This materially narrows the fix's integration options and must inform planning.

- timestamp: 2026-09-22T07:53:00Z
  checked: web/templates/index.html.tmpl (script load order)
  found: theme.js loads synchronously in <head> (line 19, no defer/async — confirmed required by TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage) and therefore applies data-theme (or leaves it absent for system mode) before body parsing even starts. map.js loads at the end of body with `defer` (line 125), so by the time map.js's init() runs, document.documentElement's data-theme attribute (or its absence) already reflects the fully resolved, persisted mode from localStorage. There is no race between the two scripts on initial load — the data is available and stable by the time map.js could read it.
  implication: The *initial-load* half of the fix is straightforward: map.js's init() can synchronously read document.documentElement.getAttribute('data-theme') at the moment it builds the style URL, with zero timing hazard. The *dynamic toggle-after-load* half is the part requiring a new signal path (see above), since nothing currently notifies map.js of a later click.

- timestamp: 2026-09-22T07:54:00Z
  checked: web/static/css/main.css lines 11-134 (theme selector structure) and grep for matchMedia across web/static/js/
  found: The system-preference case (data-theme attribute absent, the default/'system' mode) is currently resolved entirely in CSS via `@media (prefers-color-scheme: dark)`, with no JS involvement anywhere in the codebase (`grep -rn matchMedia web/static/js/` returns zero matches).
  implication: There is no existing JS precedent to reuse for resolving "what does the map's theme currently mean when data-theme is absent" — map.js's fix must add its own `window.matchMedia('(prefers-color-scheme: dark)').matches` read for the absent-attribute case, mirroring what main.css already does declaratively. This is a new capability, not a refactor of an existing one.

- timestamp: 2026-09-22T07:56:00Z
  checked: OpenFreeMap dark style endpoint and maplibre-gl-leaflet's runtime API (web search, cross-referenced against GitHub README/npm-published API surface)
  found: `layer.getMaplibreMap()` returns the underlying `maplibregl.Map` instance once the layer has been added and initialized; `layer.getMaplibreMap().setStyle(url)` is the documented, standard way to swap styles on a live map without a full layer/map re-init. `https://tiles.openfreemap.org/styles/dark` (confirmed live, HTTP 200, per orchestrator's prior curl check) is a same-vendor, same-auth-model (free, no key) counterpart to the already-in-use `liberty` style.
  implication: A full re-init of the Leaflet map or the vector layer is NOT required for the toggle-after-load case on the vector (WebGL) path — `setStyle()` on the held maplibregl.Map reference is the clean, minimal integration point, provided a reference to the layer is retained (see next finding for a timing hazard on this).

- timestamp: 2026-09-22T07:58:00Z
  checked: map.js lines 32-37 (existing code comment) cross-referenced against centerOnVisitor() (lines 109-136) and init() (lines 52-107)
  found: map.js's own comment documents that "Leaflet defers a layer's add hook until the map's first view is set" — i.e. `L.maplibreGL({...}).addTo(map)` in init() does NOT synchronously construct the underlying maplibregl.Map; that only happens once `map.setView(...)` is first called. `setView` is only called inside `centerOnVisitor()`'s `boot()` callback (line 116-121), which itself only runs after the async `navigator.geolocation.getCurrentPosition` call resolves (success or the denial/timeout error callback) — not synchronously during init().
  implication: A real hazard for the fix: if the theme toggle is clicked before geolocation has resolved (plausible — the permission prompt can sit unanswered, and the timeout is 8000ms), the underlying maplibregl.Map may not exist yet, so `layer.getMaplibreMap()` could return undefined/throw if called naively. Any dynamic re-theming code must either (a) guard for this (check the layer/map exists before calling setStyle, and no-op or queue-until-ready otherwise — since the *next* init will already pick up the correct data-theme value once setView does fire), or (b) simply rely on the fact that the *initial* style choice at construction time will already be correct (per the evidence above, data-theme is stable before map.js even runs), and only worry about re-applying on a toggle that happens strictly after the map has already rendered once.

- timestamp: 2026-09-22T07:59:00Z
  checked: 02-UAT.md lines 230-238 (pre-existing gap entry for this exact issue)
  found: The UAT file already contains a root_cause entry and a `missing` list matching this investigation's findings almost verbatim, including the OpenFreeMap dark-style research and the open question about the raster fallback having no free dark equivalent.
  implication: This investigation independently re-derives and confirms the same diagnosis from the live code, rather than trusting the UAT file's claim at face value — corroborating, not circular.

- timestamp: 2026-09-22T08:10:00Z
  checked: web/static/js/modal.js (grepped for maplibreGL/tileLayer/openfreemap/liberty/L.map, then read lines 315-414 in full) — this is the app's SECOND map surface, not previously checked. index.html.tmpl:68 renders `<div id="modal-map"></div>` inside the "Report an emergency" modal (the pin-drop location picker), and modal.js is loaded on the same page as map.js.
  found: modal.js's initLocation() (line 343-424) independently builds its own Leaflet map (`modalMap`, line 361) the first time the report modal is opened (lazy-constructed, guarded by `if (!modalMap)`, not on page load like the primary map). It carries an exact duplicate of the same bug: `hasVectorBasemap()` (line 324-336, byte-for-byte the same probe as map.js's) gates vector vs. raster capability only; the vector branch (line 368-377) hardcodes the identical literal `'https://tiles.openfreemap.org/styles/liberty'` string with no data-theme read; the raster fallback (line 379-384) is the same hardcoded light-only OSM tile server. A code comment at line 315-316 explicitly documents this is deliberate duplication ("this codebase's established convention of duplicating these small pieces per file rather than sharing them"), so this is not a stray copy-paste bug — it is the same architectural gap, replicated on purpose in a second file.
  implication: The root cause is not confined to map.js. Any fix must cover BOTH map.js's primary map and modal.js's modal-map location picker, or the report-submission pin-drop map will still silently stay light-only in dark mode after a fix that only touches map.js — a second, easily-missed UAT failure on the same underlying symptom. Given the file's own stated duplication convention, the fix will most likely need to duplicate whatever theme-resolution/re-apply logic is added to map.js into modal.js as well (matching the existing pattern), rather than assuming a single shared helper will be reused across the two files without an explicit decision to break that convention.

## Resolution

root_cause: |
  Both of the app's Leaflet map surfaces hardcode a single light-only basemap with
  zero theme-awareness, and there is no signal path by which either could learn of
  a theme change even if they wanted to:

  1. web/static/js/map.js's init() (the primary map, built on page load) hardcodes
     the MapLibre vector style 'https://tiles.openfreemap.org/styles/liberty' and a
     light-only OSM raster fallback, with no read of document.documentElement's
     data-theme attribute (or the system color-scheme preference) at construction
     time, and no mechanism to react to a later theme change.
  2. web/static/js/modal.js's initLocation() (the report-submission modal's
     pin-drop location picker, lazily built the first time the modal opens)
     independently duplicates the identical hardcoded style URL and raster
     fallback — confirmed deliberate duplication per the file's own comment
     (line 315-316), not an oversight distinct from map.js's.

  This is compounded by theme.js being contractually self-contained
  (TestThemeModuleHasNoAppShellDependency in web/theme_contract_test.go greps the
  raw source and fails the build if it ever references "Pinalert" — because
  theme.js also runs unmodified on the login-gate/verify-outcome/profile pages
  where neither map.js nor modal.js exists). theme.js's toggle click handler only
  ever sets/removes the data-theme attribute on <html> and re-labels the control;
  it dispatches no event and calls no shared function. The modules are fully
  decoupled by design, so nothing currently connects a theme change to either map
  surface's basemap, on first load or on toggle.

  Confirmed NOT a race condition: theme.js loads synchronously and un-deferred in
  <head> on every full page (build-gated by
  TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage) and applies data-theme
  before body parsing starts; map.js loads deferred at the end of body, so by the
  time either map surface's init code runs, data-theme is already stable and
  synchronously readable. The gap is pure absence of integration code, not timing.

suggested_fix_direction: |
  Not implemented (find_root_cause_only mode). Concrete integration points and
  hazards for the planner, derived from this investigation:

  - Style-resolution helper: given data-theme may be absent (system/default
    mode), resolve the *effective* mode as
    `document.documentElement.getAttribute('data-theme') ||
    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')`
    — mirroring what main.css already does declaratively via its
    `@media (prefers-color-scheme: dark)` block. No existing JS precedent for this
    exists yet (`grep -rn matchMedia web/static/js/` is currently empty); this is
    new code, not a refactor. Must be re-evaluated on every resolution, not cached
    at first load, since toggling dark -> system REMOVES the attribute rather than
    setting a value.
  - Initial load (both map.js and modal.js): straightforward — swap the hardcoded
    'liberty' literal for a resolved-mode lookup ('liberty' vs OpenFreeMap's
    confirmed-live 'https://tiles.openfreemap.org/styles/dark') at the point each
    file currently constructs its L.maplibreGL({style: ...}) call. No timing
    hazard here (see Evidence above).
  - Toggle-after-load (the actual reported symptom): theme.js emits no signal, and
    per the app-shell-dependency contract test it should not be modified to add
    one lightly. Two viable approaches:
      (a) RECOMMENDED — a MutationObserver in map.js/modal.js on
          document.documentElement watching {attributes: true,
          attributeFilter: ['data-theme']}. Requires zero change to theme.js,
          which sidesteps the self-containment contract entirely rather than
          threading a needle through it.
      (b) Alternative — a lowercase-named CustomEvent (e.g.
          'pinalert:theme-change', matching modal.js's existing
          'category-change' CustomEvent pattern) dispatched from theme.js's
          applyMode(). Viable because TestThemeModuleHasNoAppShellDependency
          greps only for the literal "Pinalert" (capital P) — but it adds an
          outbound dependency to a module three other pages load standalone, for
          no clear gain over (a).
  - Mechanical prerequisite for re-applying without a full re-init (vector path
    only): both files currently discard the L.maplibreGL(...) return value
    (`.addTo(map)` / `.addTo(modalMap)` as bare expressions) — a live style swap
    needs `layer.getMaplibreMap().setStyle(url)`, so the layer reference must be
    retained in a module variable first.
  - Timing hazard on that setStyle call: per map.js's own documented comment
    (and mirrored in modal.js), Leaflet defers a layer's onAdd hook — and
    therefore the underlying maplibregl.Map's construction — until the map's
    first setView() call, which itself only fires inside the async
    navigator.geolocation.getCurrentPosition callback (success or
    denial/timeout, up to an 8000ms timeout). If the user opens the account menu
    and clicks the theme toggle before geolocation resolves,
    layer.getMaplibreMap() may not yet exist. Any dynamic re-theme handler must
    guard for this (no-op/skip if the layer isn't ready — the initial
    construction will already have picked the correct style once setView does
    fire, per the initial-load fix above) rather than assume the maplibre map is
    always present.
  - modal.js's map is lazily constructed on first modal open, not on page load —
    a MutationObserver-based handler added there must itself guard for
    `modalMap` being null/undefined if the toggle is clicked before the report
    modal has ever been opened once.
  - Open decision for the planner, not resolved here: the WebGL-less OSM raster
    fallback has no known free dark-tile equivalent (confirmed live,
    'https://tiles.openfreemap.org/styles/dark' only serves the vector/MapLibre
    path). The fix needs an explicit choice — accept a light-only raster fallback
    even in dark mode (with that limitation documented), or find/evaluate an
    alternative free dark raster source — rather than this being silently
    dropped.
fix: (not implemented — find_root_cause_only mode; see suggested_fix_direction above)
verification: (not applicable — find_root_cause_only mode)
files_changed: []
