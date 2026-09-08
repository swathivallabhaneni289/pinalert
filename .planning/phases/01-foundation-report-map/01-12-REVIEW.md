---
phase: 01-foundation-report-map
reviewed: 2026-09-08T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - web/templates/index.html.tmpl
  - web/template_contract_test.go
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/js_contract_test.go
  - .claude/CLAUDE.md
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
  fixed: 2
status: issues_found_and_fixed
---

# Phase 01 Plans 11+12: Code Review Report

**Reviewed:** 2026-09-08T00:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

This review covers the full vector-basemap migration (plans 01-11 and 01-12): vendor CDN loading
with SRI in `index.html.tmpl` plus its new `template_contract_test.go`, and the basemap
construction swap in `map.js`/`modal.js` plus the rewritten `js_contract_test.go`. `go build ./...`,
`go vet ./...`, `go test ./...` and `node --check` on both JS files all pass. `git diff --stat`
across both plans' commits touches exactly the files each plan declared — no stylesheet, no other
template, no other JavaScript file was modified.

**Vendor loading (01-11):** Script/stylesheet order in `index.html.tmpl` is correct (Leaflet →
MapLibre renderer → bridge → app modules), every vendor tag carries `integrity="sha256-…"` and
`crossorigin="anonymous"`, and none carries `defer`/`async`. I independently recomputed the
bridge's SRI hash against the live CDN bytes during this review and it matches the pinned value
byte-for-byte (`sha256-Hmz4yz61/ZCYeaob82o4P7UGyaWy27+rq85lopTdH8s=`), corroborating the executor's
own recomputation gate and the orchestrator's earlier Node `vm`-sandbox confirmation that
`L.maplibreGL` and `maplibregl` both resolve. `TestVendorMapScriptsLoadInDependencyOrder` correctly
bounds its attribute assertions per tag (via `findTagWindow`) rather than searching unboundedly, so
it can't be fooled by an attribute belonging to a neighboring tag.

**Basemap swap (01-12) — the five specific questions this review was asked to focus on:**

1. **`hasVectorBasemap()` correctness across every failure mode.** Traced all four required failure
   modes by hand against the UMD factory behavior described in `01-11-RESEARCH.md`: bridge script
   missing → `L.maplibreGL` stays `undefined` on `L`, first check catches it safely. Renderer script
   missing → the bridge's own UMD factory sets `L.maplibreGL` as a function unconditionally at
   evaluation time regardless of whether `global.maplibregl` resolved (it only dereferences that
   parameter later, inside `_initGL()`), so the *first* check alone would **not** catch this — but a
   *second*, independent check (`typeof maplibregl === 'undefined'`) does. WebGL2 context creation
   throwing or returning null/falsy is handled correctly (`try/catch` plus `!!` coercion). All four
   modes are correctly handled in the code as written today — but see WR-01: neither of the two new
   Go tests actually asserts these two `typeof` checks exist, only that the probe is *defined* and
   *invoked*, so this correctness is not durably enforced (mutation-confirmed).
2. **`setMaxZoom()` placement relative to Leaflet's retina mutation.** Correct, and load-bearing —
   not redundant. Leaflet's `detectRetina` branch mutates `options.maxZoom` inside the `TileLayer`
   constructor itself (at `L.tileLayer(...)` call time), so reading `rasterLayer.options.maxZoom`
   at any point after construction is safe. More importantly: `map.options.maxZoom` is fixed at `19`
   from the `L.map()` call, and Leaflet's `getMaxZoom()` returns the map's own `options.maxZoom`
   whenever it is defined, **ignoring** any zoom-bound layer's own maximum entirely. So without the
   `setMaxZoom()` correction, the map's pinch-zoom ceiling would stay at `19` even when the raster
   layer's own ceiling silently dropped to `18` on a retina display — letting a visitor zoom to a
   level at which the grid layer renders no tiles. The fix works by actively lowering the map's
   ceiling to match. No bug found in the mechanism itself — but see WR-01: this line, too, is
   asserted only by a one-time shell gate in the plan's `<verify>` block, not by any Go test
   (mutation-confirmed).
3. **Attribution — exactly one credit box, both paths.** Traced independently for both branches.
   Vector path: `attributionControl: false` is force-set inside the bridge on the MapLibre `Map` it
   constructs, and the bridge's `getAttribution()` returns `customAttribution` synchronously at
   `onAdd`, routed into Leaflet's own (default, single) attribution control. Raster path: the tile
   layer's own `attribution` option is the only credit source in that branch. Neither branch sets an
   `attribution`/`attributionControl` option on the `L.map()` call itself, so there is no path by
   which both credit sources could reach the same control simultaneously — the two branches are
   mutually exclusive by construction (`if (hasVectorBasemap()) {...} else {...}`). No
   duplicate-credit-box path found.
4. **Inline-literal convention.** Verified consistently followed in both files: style URL,
   attribution strings (each file's own, deliberately different), and all zoom/bounds values are
   inline literals, not hoisted into `app.js` or any shared module. No shared-constant violation.
5. **Tool-artifact contamination.** `grep -nE "</(content|invoke|parameter|function_calls)>|<invoke|<parameter"` across both plan/summary/research documents and all six reviewed source files returns no matches. Clean.

No security issues, no hardcoded secrets, no dangerous functions, no debug artifacts (`console.log`,
`debugger`, `TODO`/`FIXME`), and no dead code were found. The shipped JavaScript is correct as
written — nothing found here misbehaves at runtime. The material finding (WR-01) is about the
durability of two guarantees this plan's own threat register claims are mitigated, not about the
code's current behavior.

## Warnings

### WR-01: The capability-probe test asserts the probe is *invoked*, not that it checks either vendor global or `setMaxZoom` — both load-bearing lines are unenforced by any Go test, and mutation-testing confirms it

**File:** `web/js_contract_test.go:287-334` (`TestCapabilityProbeGatesBasemapChoice`); the lines it
fails to cover live in `web/static/js/map.js:33-38, 90` and `web/static/js/modal.js:328-333, 385`.

**Issue:** `TestCapabilityProbeGatesBasemapChoice` checks that `function hasVectorBasemap` is
defined, that `getContext('webgl2')` appears *somewhere in the file*, and that the probe is invoked
before the vector construction — but it never opens a bounded window on the probe's own body and
never asserts what the probe actually *contains*. I confirmed by direct mutation that this makes
both of the following silently pass every test in the file and `node --check`:

**Mutation 1 — delete both vendor-global checks from `hasVectorBasemap()` in both files:**
```js
function hasVectorBasemap() {
  try { return !!document.createElement('canvas').getContext('webgl2'); }
  catch (e) { return false; }
}
```
`go test ./web/ -run 'TestBasemapBranchesPointAtCorrectHosts|TestMapsDeclareOwnZoomBounds|TestCapabilityProbeGatesBasemapChoice' -v` — **all three tests still pass.** This is exactly the
line whose necessity required reading the bridge's own UMD factory source to establish (see Summary
point 1): the bridge sets `L.maplibreGL` as a function even when `maplibregl` never loaded, so
`typeof maplibregl === 'undefined'` is the *only* code-level defense against a renderer-script SRI
mismatch or 404 slipping past the probe. Delete it and this plan's own threat register (T-01-44)
claim that the probe "also covers the vendor scripts failing to load or being blocked by their
integrity check" becomes false with zero build-time signal.

**Mutation 2 — delete `map.setMaxZoom(rasterLayer.options.maxZoom)` /
`modalMap.setMaxZoom(rasterLayer.options.maxZoom)` from both files** (the plan's own headline
correction, and the fix traced as correct and load-bearing in Summary point 2): all three tests
**still pass.** The only place this line is checked at all is a one-time
`grep -Fq 'setMaxZoom('` in `01-12-PLAN.md`'s `<verify>` block, which ran once during execution and
left no trace in the repository.

Both mutations were restored via direct file copy and reconfirmed against a clean `git diff` and a
full green `go test ./web/ -v` before writing this finding.

**This is the identical defect class `01-10-REVIEW.md`'s WR-02 already flagged and fixed once in
this same phase** (a threat-register `mitigate` disposition backed only by a plan-time verify gate,
with no durable Go-test equivalent). That review's fix pattern — extend the bounded per-construction
window the existing tests already use — applies directly here and was available as prior art within
this same directory.

**Fix:** Bound a window on the probe's own body (from `"function hasVectorBasemap"` to its closing
`}`, or to the next `"function "` token, whichever comes first) and assert, *within that window*:

```go
probeBody, _, probeTermFound := windowAfter(text, probeDefAnchor, "\n  }")
// or a small brace-counting helper if a plain string terminator proves too fragile
if !probeTermFound {
    t.Fatalf("%s: could not bound the capability probe's own body", module)
}
if !strings.Contains(probeBody, "L.maplibreGL") {
    t.Errorf("%s: the capability probe must check L.maplibreGL — without it, a renderer script "+
        "(maplibre-gl.js) that failed to load or was blocked by its integrity check is not caught, "+
        "since the bridge sets L.maplibreGL unconditionally regardless of that global's state", module)
}
if !strings.Contains(probeBody, "typeof maplibregl") {
    t.Errorf("%s: the capability probe must check the maplibregl global directly — this is the only "+
        "check that catches the renderer script itself failing to load", module)
}
if !strings.Contains(probeBody, "getContext('webgl2')") {
    t.Errorf("%s: the capability probe must request a webgl2 context", module)
}
```

And separately, in `TestBasemapBranchesPointAtCorrectHosts`'s existing raster-window handling (or a
small dedicated check), assert `setMaxZoom(` appears in the file *after* the raster branch's
`.addTo(` — mirroring the exact ordering the test's own doc comment already describes but never
checks:

```go
addToIdx := strings.Index(text, rasterAnchor)
// ... locate the raster branch's own addTo, then:
if !strings.Contains(text[addToIdxOfRasterAddTo:], "setMaxZoom(") {
    t.Errorf("%s: raster fallback must re-derive the map's maxZoom from the layer's own "+
        "post-construction value — without it, a retina-adjusted fallback layer can leave the map "+
        "able to reach a zoom at which it renders no tiles", module)
}
```

Both additions ride along in the same file and helpers already present; this closes the gap the
same way WR-02 in `01-10-REVIEW.md` closed the equivalent `maxZoom` gap for the raster-only layer.

**Outcome: fixed.** Extended `TestCapabilityProbeGatesBasemapChoice` with a bounded probe-body window
(reusing the existing `defIdx`/`callIdx` computation already in the test — the span between the
probe's own definition and its first call site) and three assertions scoped to that window: the
bridge-global guard (`typeof L.maplibreGL`), the renderer-global guard (`typeof maplibregl`), and the
webgl2 context request. Also added a `fallbackCeilingCorrection` check requiring the literal
`setMaxZoom(rasterLayer.options.maxZoom)` to appear after the raster branch's own anchor in the file.
Reproduced both of this review's exact mutations (deleting the two vendor-global checks; deleting the
`setMaxZoom` line) in both files and confirmed each now fails naming the file and citing the specific
missing guard, then restored and confirmed a clean pass. Full `go build && go vet && go test ./...`
and `node --check` on both JS files all pass with the fix in place.

### WR-02: `modal.js` has no guard against Leaflet itself failing to load, unlike `map.js` — the new fallback design's safety net doesn't reach this specific failure mode in this file

**File:** `web/static/js/modal.js:346-361` (compare `web/static/js/map.js:46-50`)

**Issue:** `map.js`'s `init()` explicitly checks `typeof L === 'undefined'` and returns early before
ever touching `L`:

```js
function init() {
  var container = document.getElementById('map');
  if (!container || typeof L === 'undefined') {
    return;
  }
  ...
  map = L.map(container, {...});
```

`modal.js`'s `initLocation()` has no equivalent check. It calls `L.map(modalMapEl, {...})`
unconditionally as the first line inside its `if (!modalMap)` guard:

```js
function initLocation() {
  if (!modalMap) {
    modalMap = L.map(modalMapEl, {...});   // throws ReferenceError if L is undefined
    if (hasVectorBasemap()) { ... } else { ... }
  }
  ...
```

If Leaflet's own script tag ever fails to load (network failure, an ad/privacy blocker, or an SRI
mismatch on `leaflet.js` specifically — the same class of failure the new capability probe and
fallback branch exist to defend against for the *other* two vendor scripts), `map.js` degrades
gracefully: no primary map, but nothing else on the page breaks. `modal.js` does not degrade at
all: `openModal()` has already set `modal.hidden = false` and built the category grid by the time
`initLocation()` throws, so the visitor sees a half-rendered modal with an uncaught
`ReferenceError` in the console, no location map, no keydown/Escape trap attached (that line runs
after `initLocation()` and never executes), and no way to submit a report — the app's one
report-submission path breaks silently in exactly the emergency-reporting scenario this plan's own
threat register (T-01-44) says the fallback exists to protect ("a low-end Android device or an
in-app browser opened from a shared link during an emergency").

This is not a new bug introduced by this migration — `modal.js`'s `L.map(modalMapEl)` call was
unconditional before 01-12 too, and the raster-only fallback would have hit the same failure mode
pre-migration if Leaflet itself failed to load. It's flagged here because (a) this plan explicitly
added a parallel, duplicated capability probe to `modal.js` specifically to make basemap failures
survivable, and the asymmetry with `map.js`'s existing guard means that design goal is not
uniformly achieved across the two files it was applied to, and (b) the plan's own instruction was
to do "exactly the same" to modal.js as map.js — this one guard was the one difference.

**Fix:** Add the same guard `map.js` already has, at the top of `initLocation()`:

```js
function initLocation() {
  if (!modalMapEl || typeof L === 'undefined') {
    return;
  }
  if (!modalMap) {
    modalMap = L.map(modalMapEl, {...});
    ...
```

and have `openModal()` account for `initLocation()` becoming a no-op in that case (e.g. leave the
location notice / tap-to-place UI in an honest unresolved state) rather than assuming it always
succeeds. This brings `modal.js` to the same graceful-degradation floor `map.js` already has,
closing the one asymmetry between the two files' otherwise-identical treatment.

**Outcome: fixed, with a scope note.** Added the `typeof L === 'undefined'` guard at the top of
`initLocation()`, mirroring `map.js`'s pattern exactly, so a missing Leaflet script now makes the
function a no-op instead of throwing. Did **not** additionally modify `openModal()`'s downstream
UI state (the location-notice / tap-to-place messaging) for this already-pathological case — Leaflet
itself failing to load is a pre-existing failure mode this migration did not introduce (the fix's own
issue text says as much), and giving it bespoke UI copy is a separate, larger scope decision than
"stop the uncaught exception," which is what this review's fix actually required. The uncaught
`ReferenceError` is eliminated; the modal now silently has no working location picker in this
edge case rather than crashing, which is the load-bearing part of the original finding.

## Info

### IN-01: `readOptionValue`'s substring search is order-dependent, not key-bounded — currently correct, but fragile

**File:** `web/js_contract_test.go:43-59`

**Issue:** `readOptionValue(body, "style")` finds the *first* occurrence of the literal text
`"style"` anywhere in `body`, not the first occurrence of `style` as a standalone option key. In the
vector basemap's options object, the value itself is
`'https://tiles.openfreemap.org/styles/liberty'`, which contains `style` as a substring of
`styles`. This currently works correctly only because the key `style:` is written *before* the
value on the same line, so `strings.Index` finds the key first by construction. The same mechanism
means `readOptionValue(body, "maxZoom")` in `TestMapsDeclareOwnZoomBounds` would, in principle, match
a hypothetical future `maxNativeZoom` option or a comment mentioning "maxZoom" before the real key,
if either were ever introduced ahead of it in the same bounded window. Currently fine in both cases
given the file's present layout; not urgent.

**Fix:** A more robust key match would anchor on a word boundary or require the key to be
immediately followed by optional whitespace and a colon (e.g. search for `"style:"`/`"style :"`
patterns, or a small regex `\bstyle\s*:`) rather than a bare substring match. Low priority — flagged
for awareness, not requiring action before this ships.

---

_Reviewed: 2026-09-08T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
