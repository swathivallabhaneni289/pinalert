# Phase 01 (plan 11): MapLibre GL + OpenFreeMap vector basemap — Research

**Researched:** 2026-09-08
**Domain:** Browser map rendering — swapping Leaflet's raster `L.tileLayer` for a MapLibre GL vector layer via the `@maplibre/maplibre-gl-leaflet` bridge, in a no-build-step, plain-`<script>`-tag frontend.
**Confidence:** HIGH (the decisive findings come from the plugin's own published UMD source, the published package metadata, the CDN's own 200/404 responses, MapLibre's shipped `.d.ts`, and OpenFreeMap's live TileJSON — all fetched this session)

---

## ⚠️ Premise Correction (read this first)

The task brief states the target is `maplibre-gl@6.8.0` loaded via a plain `<script>` tag. **That combination is impossible.**

`maplibre-gl` **v6 ships no UMD / browser-global build at all** — its `dist/` contains only ES modules.

```
https://unpkg.com/maplibre-gl@6.8.0/dist/maplibre-gl.js  -> HTTP 404
https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js -> HTTP 200  (1,056,837 bytes)
```
[VERIFIED: unpkg HTTP status, fetched 2026-09-08]

`maplibre-gl@6.8.0/dist/` listing (complete, JS only): `maplibre-gl.mjs`, `maplibre-gl-dev.mjs`, `maplibre-gl-shared.mjs`, `maplibre-gl-shared-dev.mjs`, `maplibre-gl-worker.mjs`, `maplibre-gl-worker-dev.mjs` — every one an `.mjs`. [VERIFIED: `unpkg.com/maplibre-gl@6.8.0/dist/?meta`]

The bridge plugin ships **two mutually-exclusive examples**, and its own maintainers draw the line explicitly:

| Example file | Setup | MapLibre version | API surface |
|---|---|---|---|
| `examples/basic-v5.html` | classic `<script>` tags, browser globals | **v2–v5** | `L.maplibreGL({...})` |
| `examples/basic.html` | `<script type="module">` + `importmap` + Leaflet's **ESM** build | v6 | bare `maplibreGL({...})` |

`basic-v5.html` carries the comment: *"The UMD adapter supports the browser globals used by MapLibre GL JS v2-v5."* [VERIFIED: fetched from `unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/examples/basic-v5.html`]

### Recommendation: pin `maplibre-gl@5.24.0` (UMD), not 6.8.0

Three independent reasons, all grounded in this repo's locked constraints:

1. **No build step.** `index.html.tmpl` loads Leaflet as a classic script. The v6/ESM path requires an `importmap` **and** swapping Leaflet itself to `leaflet-src.esm.js` — which destroys the global `L` that `app.js`, `map.js`, `modal.js`, and `feed.js` all depend on. That is the full rewrite the decision block explicitly declined.
2. **SRI is unenforceable on the v6 path.** `maplibre-gl.mjs` transitively imports `./maplibre-gl-shared.mjs`. SRI on a `<script>` tag does not cover chunks the module graph pulls in afterwards. The v5 UMD is a single self-contained file (worker inlined) — one file, one hash, complete coverage. This repo puts `integrity=` on every third-party tag today; the v6 path would silently downgrade that posture.
3. **Latest ≠ required.** `maplibre-gl@6.8.0` was published **2026-09-07** — one day before this research. `5.24.0` (2026-04-23) is the mature terminal v5. The plugin's `peerDependencies` accept `^2.4.0 || ^3.3.1 || ^4.3.2 || ^5.0.0 || ^6.0.0`, so v5 is fully supported, not a downgrade hack. [VERIFIED: npm registry `@maplibre/maplibre-gl-leaflet@0.1.4`]

**If the planner emits a `maplibre-gl@6.x` `<script src=...>` tag, the page 404s the script and then throws `L.maplibreGL is not a function`** — which is verbatim [open issue #52](https://github.com/maplibre/maplibre-gl-leaflet/issues/52), filed by someone who hit exactly this. [CITED: github.com/maplibre/maplibre-gl-leaflet/issues/52]

*Everything else in the decision block is unaffected: OpenFreeMap, the `liberty` style, and the bridge-plugin-not-rewrite approach all stand.*

---

## Summary

The `@maplibre/maplibre-gl-leaflet` bridge is a **234-line `L.Layer` subclass**. It is small enough to have been read in full this session, and reading it settles most of the open questions definitively rather than by inference.

Three defaults in that source make this migration far lower-risk than a "swap the rendering engine" framing suggests. The layer defaults to `pane: "tilePane"` and `interactive: false`, so the WebGL canvas lands in exactly the DOM pane the raster tiles occupy today — beneath Leaflet's marker, popup, and control panes, with no z-index change anywhere. It force-sets `attributionControl: false` on the MapLibre `Map` it constructs and instead pushes the style's attribution string into **Leaflet's own** `attributionControl` — so there is exactly one attribution box, the same `.leaflet-control-attribution` element as today, and no duplicate. And MapLibre's `pixelRatio` "Defaults to `devicePixelRatio` if not specified," which makes Leaflet's `detectRetina` concept inapplicable rather than merely unused.

The real work is smaller than the real risks. Two risks dominate. First, **`maxZoom` silently disappears**: Leaflet derives `getMaxZoom()` from zoom-bound layers, only `GridLayer` registers as one, and the GL layer extends bare `L.Layer` — so deleting `L.tileLayer({maxZoom: 19})` leaves the map at `maxZoom === Infinity` unless `maxZoom` is set explicitly on the `L.map()` options. Second, **`modal.js` constructs its Leaflet map while `#report-modal` is still `hidden`** — a 0×0 container — and only calls `invalidateSize()` on a later `setTimeout(0)`. A raster tile layer tolerates that; a WebGL context initialised against a zero-size container is a documented failure shape in this plugin's issue tracker.

**Primary recommendation:** Pin `leaflet@1.9.4` (unchanged) + `maplibre-gl@5.24.0` (UMD) + `@maplibre/maplibre-gl-leaflet@0.1.4` (UMD, package root), loaded in that exact order as three classic `<script>` tags with SRI. Replace both `L.tileLayer(...)` calls with `L.maplibreGL({ style: 'https://tiles.openfreemap.org/styles/liberty', attributionControl: { customAttribution: '<OpenFreeMap credit>' } })`, add the maintainer-sanctioned `maxBounds`/`maxBoundsViscosity`/`minZoom`/`maxZoom` options to **both** `L.map()` calls, and defer the modal's GL layer until after the modal is visible.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| Basemap raster/vector rendering | Browser / Client (WebGL canvas) | CDN (OpenFreeMap tiles, Cloudflare-fronted) | MapLibre renders vector tiles client-side; the server is not in this path at all. |
| Map interaction (pan/zoom/click) | Browser / Client (Leaflet) | — | Leaflet remains the sole interaction owner; the GL layer is `interactive: false` and only mirrors Leaflet's view. |
| Marker/popup/pin rendering | Browser / Client (Leaflet DOM panes) | — | Unchanged. `L.divIcon`/`L.marker`/`bindPopup(domElement)` stay in Leaflet's `markerPane`/`popupPane`. |
| Attribution display | Browser / Client (Leaflet `attributionControl`) | — | The bridge routes MapLibre's attribution *into* Leaflet's control; MapLibre's own control is disabled. |
| Third-party asset integrity | Frontend Server (`index.html.tmpl`) | CDN (unpkg) | SRI hashes are authored server-side into the template; the browser enforces them. |
| Static-source regression gating | API / Backend (Go test over `embed.FS`) | — | `web/js_contract_test.go` inspects the embedded JS at build time — unchanged mechanism, new assertions. |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---|---|---|---|
| `leaflet` | **1.9.4** (unchanged) | Map framework, markers, popups, controls, panes | Already in use. Bridge peer-depends on `leaflet: ^1.9.3` — 1.9.4 satisfies it. [VERIFIED: npm registry peerDependencies] |
| `maplibre-gl` | **5.24.0** | WebGL vector-tile renderer | Terminal v5, published 2026-04-23. **The last version with a UMD browser-global build.** Bridge peer range includes `^5.0.0`. [VERIFIED: npm registry + unpkg 200] |
| `@maplibre/maplibre-gl-leaflet` | **0.1.4** | `L.Layer` subclass that drives a MapLibre `Map` from Leaflet's view state | Official MapLibre-org plugin. Published with SLSA provenance attestation and npm trusted-publisher OIDC (`github`). [VERIFIED: npm registry dist.attestations] |

### Supporting

| Asset | Purpose | When to Use |
|---|---|---|
| `maplibre-gl@5.24.0/dist/maplibre-gl.css` | Positions MapLibre's internal container/canvas | **Required, not optional.** `.maplibregl-map { position: relative; overflow: hidden }` and `.maplibregl-canvas { position: absolute; top: 0; left: 0 }` are load-bearing for the canvas to sit correctly inside the layer container. [VERIFIED: fetched and inspected] |
| OpenFreeMap `liberty` style | Style JSON (111 layers, v8 spec) | `https://tiles.openfreemap.org/styles/liberty` — HTTP 200, `access-control-allow-origin: *`, `cache-control: public, max-age=86400`. [VERIFIED: `curl -I`] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|---|---|---|
| `maplibre-gl@5.24.0` UMD | `maplibre-gl@6.8.0` ESM + importmap | Requires Leaflet's ESM build → loses global `L` → rewrites all four JS modules. Loses enforceable SRI (multi-chunk module graph). Explicitly out of scope. |
| `maplibre-gl.js` | `maplibre-gl-csp.js` (also in v5 dist) | Only if a CSP is later added that forbids `blob:` workers. Not needed now — this app sends **no** `Content-Security-Policy` header (`grep -rn "Content-Security-Policy" internal/ cmd/ web/` → zero hits). [VERIFIED: grep] |

**Installation:** none — no package manager in this project. Assets are CDN `<script>`/`<link>` tags only.

---

## Package Legitimacy Audit

| Package | Registry | Age | Publisher / Provenance | Source Repo | Verdict | Disposition |
|---|---|---|---|---|---|---|
| `maplibre-gl@5.24.0` | npm | published 2026-04-23 | maplibre org | github.com/maplibre/maplibre-gl-js | **OK** | Approved |
| `@maplibre/maplibre-gl-leaflet@0.1.4` | npm | current `latest` | npm **trusted publisher** (GitHub OIDC), **SLSA v1 provenance attestation** present | github.com/maplibre/maplibre-gl-leaflet | **OK** | Approved |
| `leaflet@1.9.4` | npm | already in use, unchanged | — | github.com/Leaflet/Leaflet | **OK** | No change |

[VERIFIED: `registry.npmjs.org/@maplibre/maplibre-gl-leaflet/0.1.4` → `dist.attestations.provenance.predicateType: "https://slsa.dev/provenance/v1"`, `_npmUser.trustedPublisher.id: "github"`]

Both new packages are scoped to / owned by the **maplibre** organisation, the same org that maintains the renderer. No third-party wrapper, no single-maintainer risk, no slopsquatting surface (the scoped name `@maplibre/…` cannot be squatted by a non-org publisher).

**Packages removed due to SLOP verdict:** none.
**Packages flagged SUS:** none.

### SRI hashes

The **method** used below was validated by reproducing the two hashes already in `index.html.tmpl` byte-for-byte:

```
$ curl -sL https://unpkg.com/leaflet@1.9.4/dist/leaflet.js  | openssl dgst -sha256 -binary | openssl base64
20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo=   # matches the tag in index.html.tmpl exactly
$ curl -sL https://unpkg.com/leaflet@1.9.4/dist/leaflet.css | openssl dgst -sha256 -binary | openssl base64
p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY=   # matches the tag in index.html.tmpl exactly
```
[VERIFIED: computed this session, both match]

Computed with that same validated command against the pinned URLs:

| URL | integrity |
|---|---|
| `https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css` | `sha256-qx5w1Z7EBGW65+cDDaLzzPKBM/1QLmK9WY7vut/XpzI=` |
| `https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js` | `sha256-RamwepGJzlYFTGIKlHzPQeKR5YyV6bYVM7dAqqZe5cs=` |
| `https://unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js` | `sha256-Hmz4yz61/ZCYeaob82o4P7UGyaWy27+rq85lopTdH8s=` |

**Executor must independently recompute these before committing**, using the exact command above against the exact pinned URLs. Do not copy a hash from unpkg's own `?meta` endpoint — that is the CDN vouching for itself, which defeats the point of SRI. Note the existing tags use **`sha256-`**, not `sha384-`; match that for consistency.

---

## Architecture Patterns

### Data flow

```
                        index.html.tmpl  (SRI-pinned classic <script> tags, strict order)
                                 │
        ┌────────────────────────┼────────────────────────┐
        ▼                        ▼                        ▼
   leaflet.js            maplibre-gl.js         leaflet-maplibre-gl.js
   (global L)          (global maplibregl)      reads BOTH globals,
        │                        │              assigns L.maplibreGL
        └────────────┬───────────┴──────────────────┬─────┘
                     ▼                              ▼
              map.js  init()                 modal.js  initLocation()
              L.map('#map', {...})           L.map('#modal-map', {...})
                     │                              │
                     ▼                              ▼
            L.maplibreGL({style}).addTo(map)  ── same call ──
                     │
                     ▼
        MaplibreGL.onAdd(map)
          ├─ _initContainer()  -> <div class="leaflet-gl-layer">
          ├─ map.getPane("tilePane").appendChild(container)   ◄── z-index 200
          └─ _initGL()
               ├─ new maplibregl.Map({ ...opts, attributionControl:FALSE })
               ├─ GET tiles.openfreemap.org/styles/liberty   (style JSON)
               │       └─ GET tiles.openfreemap.org/planet   (TileJSON)
               │             └─ GET .../{z}/{x}/{y}.pbf      (vector tiles)
               ├─ canvas.classList += leaflet-image-layer, leaflet-zoom-animated
               └─ on("load") -> Leaflet attributionControl.addAttribution(...)
                     │
   Leaflet view events (move/zoom/resize)  ──throttled 32ms──▶  glMap.jumpTo({zoom: leafletZoom - 1})

   UNCHANGED, all above the canvas in higher panes:
     markerPane  (600)  L.marker + L.divIcon badge pins
     popupPane   (700)  bindPopup(DOM element)
     .fab               z-index 1000  (position: fixed, outside Leaflet entirely)
     .modal-backdrop    z-index 1100
```

### Pattern 1: Script load order is a hard constraint

`leaflet-maplibre-gl.js`'s UMD factory reads `global.L` and `global.maplibregl` **at evaluation time**:

```js
// leaflet-maplibre-gl.js, line 8 — VERIFIED, read from the published file
(global = typeof globalThis !== "undefined" ? globalThis : global || self,
 factory(global.MaplibreGLLeaflet = {}, global.L, global.maplibregl));
```

Both globals must already exist. None of the three tags may carry `defer` or `async` in a way that reorders them relative to each other. The app's own four `defer`red modules run after all of them (classic non-`defer` scripts execute before any `defer`red script), so `L.maplibreGL` is guaranteed present by the time `map.js` runs. **Do not add `defer` to the three vendor tags** — a `defer`red `leaflet-maplibre-gl.js` would still work by ordering, but a mix of `defer`/non-`defer` among the three would not.

### Pattern 2: The exact tags

```html
<!-- in <head>, after the project's own stylesheets -->
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
      integrity="sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY="
      crossorigin="anonymous">
<link rel="stylesheet" href="https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css"
      integrity="sha256-qx5w1Z7EBGW65+cDDaLzzPKBM/1QLmK9WY7vut/XpzI="
      crossorigin="anonymous">

<!-- end of <body>, before the app's own defer'd modules -->
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"
        integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="
        crossorigin="anonymous"></script>
<script src="https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js"
        integrity="sha256-RamwepGJzlYFTGIKlHzPQeKR5YyV6bYVM7dAqqZe5cs="
        crossorigin="anonymous"></script>
<script src="https://unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js"
        integrity="sha256-Hmz4yz61/ZCYeaob82o4P7UGyaWy27+rq85lopTdH8s="
        crossorigin="anonymous"></script>
<script src="/static/js/app.js" defer></script>
<!-- ...map.js, modal.js, feed.js unchanged... -->
```

**Path gotcha:** the bridge's UMD build lives at the **package root** — `/leaflet-maplibre-gl.js` — not under `/dist/`. `/dist/leaflet-maplibre-gl.mjs` is the ESM build and will not work in a classic script tag. The package's `main` field confirms the root path: `"main": "leaflet-maplibre-gl.js"`. [VERIFIED: npm registry + `examples/basic-v5.html`]

### Pattern 3: `L.map()` options — the maintainer-sanctioned boilerplate

**Both** official examples set the same three options on the Leaflet map, with these comments verbatim:

```js
var map = L.map(container, {
  maxBounds: [[180, -Infinity], [-180, Infinity]], // restrict bounds to avoid max latitude issues with MapLibre GL
  maxBoundsViscosity: 1, // make the max bounds "solid" so users cannot pan past them
  minZoom: 1             // prevent sync issues at zoom 0
});
```

[VERIFIED: both `examples/basic.html` and `examples/basic-v5.html`]

This is not example decoration — the package ships a dedicated `debug/max-latitude-bug-reproduction.html` for the failure it prevents. **Copy the `maxBounds` literal verbatim, including its inverted-looking lat/lng ordering; do not "correct" it.** It must land in **both** `map.js` and `modal.js`.

The `minZoom: 1` floor exists because the bridge runs MapLibre one zoom level below Leaflet (`zoom: this._map.getZoom() - 1`, MapLibre uses 512px tiles). At Leaflet zoom 0 that computes MapLibre zoom −1, which is out of range. All existing zoom levels in this codebase (11, 13, 16) are comfortably above the floor.

### Pattern 4: `maxZoom` must move from the layer to the map

Leaflet computes `getMaxZoom()` as:

```js
// leaflet-src.js:3967 — VERIFIED
getMaxZoom: function () {
  return this.options.maxZoom === undefined
    ? (this._layersMaxZoom === undefined ? Infinity : this._layersMaxZoom)
    : this.options.maxZoom;
},
```

`_layersMaxZoom` is populated from `_zoomBoundLayers`, which is only ever written by `_addZoomLimit`, which is only ever called from **`GridLayer.beforeAdd`** (`leaflet-src.js:11268`). `MaplibreGL` extends bare `L.Layer` and defines no `beforeAdd`, so it registers no zoom limit — passing `maxZoom` to `L.maplibreGL()` would be inert. [VERIFIED: read from `leaflet-src.js`]

**Consequence:** deleting `L.tileLayer({ maxZoom: 19 })` leaves both maps at `maxZoom === Infinity` — unbounded pinch-zoom, a real UX regression on touch. Set it on the map:

```js
map = L.map(container, { maxBounds: ..., maxBoundsViscosity: 1, minZoom: 1, maxZoom: 19 });
```

**Overzoom note (expected, acceptable):** OpenFreeMap's vector source declares `"maxzoom": 14`. Because the bridge runs MapLibre at `leafletZoom - 1`, past roughly Leaflet zoom 15 the renderer is overzooming z14 tile data. Vector overzoom keeps geometry crisp (it's re-rasterised from vectors, not upscaled pixels) but adds no new label/POI detail. `flyTo(marker, 16)` and the modal's `setView(..., 16)` both sit in this band. This is normal for every OpenMapTiles-schema basemap and is not a defect — but the planner should expect "zoomed right in, streets are sharp but there aren't many more labels" and not treat it as a bug.

### Pattern 5: Attribution — pass it explicitly

The bridge resolves attribution like this:

```js
// leaflet-maplibre-gl.js:73-84 — VERIFIED
getAttribution: function() {
    if (this.options.attributionControl) return this.options.attributionControl.customAttribution;
    var map = this._glMap;
    if (map && this.options.attributionControl !== false) {
        var style = map.getStyle();
        if (style && style.sources) return Object.keys(style.sources).map(...source.attribution...).filter(Boolean).join(", ");
    }
    return "";
},
```

and forces MapLibre's own control off:

```js
// leaflet-maplibre-gl.js:123-140 — VERIFIED
var options = L.extend({}, this.options, { container: ..., center: ..., zoom: ..., attributionControl: false });
this._glMap = new maplibre_gl.Map(options);
this._glMap.on("load", function() {
    if (_map && _map.attributionControl) {
        _map.attributionControl.removeAttribution(_currentAttribution);
        _map.attributionControl.addAttribution(_getAttribution());
    }
});
```

Two paths are available, and **the explicit one is strictly better here**:

- **Auto (not recommended):** OpenFreeMap's TileJSON *does* carry the right string, so this would work — `https://tiles.openfreemap.org/planet` returns `"attribution": "<a href=\"https://openfreemap.org\" target=\"_blank\">OpenFreeMap</a> <a href=\"https://www.openmaptiles.org/\" target=\"_blank\">&copy; OpenMapTiles</a> Data from <a href=\"https://www.openstreetmap.org/copyright\" target=\"_blank\">OpenStreetMap</a>"`. [VERIFIED: fetched live]. But it only appears **after the GL `load` event fires** (a network round-trip later), it depends on MapLibre propagating TileJSON `attribution` onto `getSource().attribution` (unverified at runtime this session — `[ASSUMED]`), and it is not statically assertable by a Go test. Note also the `liberty` style's second source, `ne2_shaded`, declares no attribution and is dropped by `.filter(Boolean)` — so the joined result would be the single OpenFreeMap string either way.
- **Explicit (recommended):**

```js
L.maplibreGL({
  style: 'https://tiles.openfreemap.org/styles/liberty',
  attributionControl: {
    customAttribution: '<a href="https://openfreemap.org" target="_blank" rel="noopener">OpenFreeMap</a> ' +
      '<a href="https://www.openmaptiles.org/" target="_blank" rel="noopener">&copy; OpenMapTiles</a> ' +
      'Data from <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a>'
  }
}).addTo(map);
```

This returns synchronously from `getAttribution()` at `onAdd` (Leaflet's attribution control calls `getAttribution()` when a layer is added), so the credit renders immediately rather than after the style loads; it is deterministic; and it is a fixed literal a static Go test can assert. The `attributionControl` object is also spread into the MapLibre `Map` constructor — but line 129 unconditionally overwrites it with `false`, so it never reaches MapLibre. Confirmed by reading the source.

**Security note for reviewers:** Leaflet's attribution control inserts this string as **HTML**, not text — which looks like a violation of this project's "report-authored content is always DOM text, never markup" convention. It is not: this is a fixed, developer-authored literal containing zero user input, and it is the same trust level as the `'&copy; <a href="...">OpenStreetMap</a> contributors'` string already passed to `L.tileLayer` today. Adding `rel="noopener"` alongside `target="_blank"` is a small hardening improvement over OpenFreeMap's own published snippet.

**Must-delete:** both existing `attribution:` options inside the two `L.tileLayer(...)` calls go away with the calls themselves. If either is preserved on the `L.map()` options or re-added elsewhere, the control will show **two** credit strings.

### Pattern 6: Pane / z-index — nothing to do

```js
// leaflet-maplibre-gl.js:38-43 — VERIFIED
options: { updateInterval: 32, padding: .1, interactive: false, pane: "tilePane" },
// :50-51
var paneName = this.getPaneName();
map.getPane(paneName).appendChild(this._container);
```

The canvas container is appended to Leaflet's **`tilePane`** — z-index 200 in `leaflet.css`, the exact pane the raster tiles occupy today. Everything above it is untouched: `markerPane` (600) holds the `L.divIcon` badge pins, `popupPane` (700) holds the DOM-element popups, `controlPane` (800) holds the attribution box, and `.fab` (z-index 1000) / `.modal-backdrop` (1100) are `position: fixed` outside Leaflet's DOM entirely. `interactive: false` means the canvas never receives pointer events, so marker clicks and `map.on('click')` (used by the modal's tap-to-place fallback) behave identically.

`getPaneName()` also degrades safely: `return this._map.getPane(this.options.pane) ? this.options.pane : "tilePane"`.

**No CSS change is needed for layering. Do not add z-index rules.**

### Anti-Patterns to Avoid

- **Loading `maplibre-gl@6.x` in a classic `<script>` tag.** 404 → `L.maplibreGL is not a function`. See the Premise Correction.
- **Loading `@maplibre/maplibre-gl-leaflet` from `/dist/leaflet-maplibre-gl.mjs` in a classic tag.** That is the ESM build; the UMD is at the package root.
- **Adding a second attribution control**, or keeping the old OSM attribution string alongside the new one.
- **Adding z-index/pane CSS for the canvas.** The default is already correct; overriding it is how markers end up behind the basemap.
- **Keeping `detectRetina`.** It is a `L.TileLayer` option with no meaning on a GL layer. Remove it rather than porting it.
- **Calling `L.maplibreGL()` before the modal is visible.** See Pitfall 1.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| Syncing MapLibre's camera to Leaflet's view | Custom `map.on('move'/'zoom')` → `glMap.jumpTo()` glue | The bridge plugin | It already handles throttling (32ms), the −1 zoom offset for 512px tiles, `zoomanim` CSS-transform interpolation, pinch-zoom, `padding: 0.1` overdraw so edges don't tear during pan, and Leaflet's `_proxy` transition-end hook. ~230 lines of subtle affine-transform code. |
| Attribution wiring between the two libraries | Manually calling `attributionControl.addAttribution()` | The bridge's `getAttribution()` + `customAttribution` option | Already wired, already deduplicates on GL `load`, already suppresses MapLibre's competing control. |
| HiDPI / retina canvas sizing | Any manual `devicePixelRatio` math | MapLibre's default `pixelRatio` | See Pitfall 3 — it's automatic. |
| Basemap style authoring | A hand-written style JSON | OpenFreeMap `liberty` (111 layers, glyphs + sprites hosted) | Font stacks (`/fonts/{fontstack}/{range}.pbf`) and sprite sheets (`/sprites/ofm_f384/ofm`) are hosted alongside; a hand-rolled style would need both. |

**Key insight:** the bridge is small enough to read end-to-end (234 lines) and every behaviour this migration depends on is visible in it. That is exactly why it's the right tool: the "what if the plugin does something surprising" risk is bounded by a file you can read in five minutes, unlike a full renderer swap.

---

## Common Pitfalls

### Pitfall 1: MapLibre initialised against a zero-size hidden container (`modal.js`) — HIGHEST RISK

**What goes wrong:** `modal.js:316-324` builds `modalMap` inside `initLocation()`, which `openModal()` calls. The current sequence is `modal.hidden = false` → `buildCategoryGrid()` → `updateSeverityReadout()` → `initLocation()`. `#report-modal` is un-hidden first, but `#modal-map` sits inside a `display: flex` backdrop whose layout may not have been computed yet in the same task, and both existing geolocation branches still schedule `modalMap.invalidateSize()` on a `setTimeout(..., 0)` — which is the tell that the container's size was known to be unreliable at construction time.

The bridge's `onAdd` immediately runs `_initContainer()` (which reads `this._map.getSize()`) and `_initGL()` (which constructs `new maplibregl.Map({container})`). A raster tile layer tolerates a 0×0 container — it simply fetches no tiles and recovers on `invalidateSize()`. A WebGL context does not recover as gracefully.

**Why it happens:** WebGL canvas dimensions are baked at context creation; MapLibre sizes its drawing buffer to `container.clientWidth * pixelRatio`. Zero-size at init produces a 0×0 drawing buffer.

**How to avoid:** in `initLocation()`, un-hide first, then create the Leaflet map, call `modalMap.invalidateSize()`, and only then `.addTo()` the GL layer — ideally inside a `requestAnimationFrame` so layout has definitely flushed:

```js
if (!modalMap) {
  modalMap = L.map(modalMapEl, { maxBounds: [[180,-Infinity],[-180,Infinity]], maxBoundsViscosity: 1, minZoom: 1, maxZoom: 19 });
  window.requestAnimationFrame(function () {
    modalMap.invalidateSize();
    L.maplibreGL({ style: OFM_STYLE_URL, attributionControl: { customAttribution: OFM_ATTRIBUTION } }).addTo(modalMap);
  });
}
```

**Warning signs:** blank/black `#modal-map` on first open that fixes itself on second open; console `Cannot read properties of null (reading 'getZoom')` or `null is not an object (evaluating 'this._map.containerPointToLayerPoint')`.

**Related upstream evidence:**
- [Issue #58](https://github.com/maplibre/maplibre-gl-leaflet/issues/58) — `containerPointToLayerPoint` of null. **Already fixed in 0.1.4**: the requested `if (!this._map) return;` guard is present at line 166 of the published UMD. [VERIFIED: read the file]
- [Issue #67](https://github.com/maplibre/maplibre-gl-leaflet/issues/67) — `getZoom` of null, filed against 0.0.22, still open. `_update` is guarded in 0.1.4 but `_pinchZoom`, `_animateZoom`, `_transformGL`, and `_transitionEnd` are **not**. Relevant if the modal is closed mid-animation.
- The package ships **`debug/maps-concurrent-resizing.html`** (11,841 bytes) — a dedicated debug page for two simultaneously-resizing maps, which is precisely this app's `#map` + `#modal-map` situation. Worth opening during implementation.

**Note also:** `map.js` is safe here — `#map` has a resolved `height: 100dvh` from the prior UAT gap-closure fix (`main.css:409-412`, guarded by `TestPrimaryMapHasResolvedHeight`), and is visible at `DOMContentLoaded`.

### Pitfall 2: `maxZoom` silently becomes `Infinity`

Covered in full under Architecture Pattern 4. This is the single most likely thing to be dropped, because the option currently lives inside the call being deleted.

### Pitfall 3 (non-pitfall, resolved): retina / `detectRetina`

**Not a risk — the concept is inapplicable.** MapLibre's shipped type definitions state it directly:

> *"The canvas' `width` attribute will be `container.clientWidth * pixelRatio` and its `height` attribute will be `container.clientHeight * pixelRatio`. Defaults to `devicePixelRatio` if not specified."*
> — `maplibre-gl@5.24.0/dist/maplibre-gl.d.ts:11287-11289` [VERIFIED: read from the published `.d.ts`]

So the drawing buffer already matches the display's device pixel ratio with no configuration, and labels/geometry are re-rasterised from vectors at that ratio rather than upscaled from a fixed-resolution raster. **No MapLibre option needs setting.** `detectRetina` was a `L.TileLayer`-only workaround for `tile.openstreetmap.org` serving no `@2x` variant; it has no counterpart and should be deleted, not ported. (`setPixelRatio()` exists for manual override — e.g. deliberately rendering at 1x to save battery — but is not wanted here.)

### Pitfall 4: WebGL becomes a hard dependency with no fallback

**What goes wrong:** MapLibre GL JS v5 requires **WebGL2**. The bundle attempts `canvas.getContext("webgl2")`, falls back to `getContext("webgl")`, and if both return null throws `"Failed to initialize WebGL"`. [VERIFIED: string extracted from `maplibre-gl@5.24.0/dist/maplibre-gl.js`]. Even where the WebGL1 fallback context is obtained, v5's renderer targets WebGL2 and full rendering is not guaranteed `[ASSUMED]`.

**Why it matters here specifically:** this is an emergency-reporting app whose stated audience is mobile users in India. Low-end Android devices, in-app WebViews (WhatsApp/Facebook browsers — a very likely referral path for a "share this flood report" link), privacy-hardened browsers with WebGL disabled, remote-desktop/VM sessions, and blocklisted GPU drivers can all fail. Today a raster tile layer degrades to "slow map"; after this change it degrades to **"no map at all."** There is no automatic fallback in the bridge.

**How to avoid / mitigate:** at minimum, wrap the GL layer construction so a throw doesn't take out marker rendering with it, and consider a feature check before constructing:

```js
function hasWebGL() {
  try { return !!document.createElement('canvas').getContext('webgl2'); }
  catch (e) { return false; }
}
```
The planner should decide (or escalate to the user) whether to (a) accept the regression as documented, (b) fall back to the existing `L.tileLayer` when `hasWebGL()` is false, or (c) show a notice. Option (b) is cheap — the old call is being deleted anyway and could be retained as a `catch` branch — but it complicates the static Go test's "exactly one map-layer construction per file" assumption, so it needs a deliberate decision rather than an accident.

**Warning signs:** blank map with a `Failed to initialize WebGL` console error; works on desktop Chrome, blank in a WhatsApp in-app browser.

### Pitfall 5: payload weight on a mobile emergency user

| Asset | Uncompressed | Gzipped over the wire |
|---|---|---|
| `leaflet.js` (existing) | ~147 KB | **42.5 KB** |
| `maplibre-gl.js` (new) | 1,056,837 B (~1.01 MB) | **275.5 KB** |
| `maplibre-gl.css` (new) | 70,024 B | **10.1 KB** |
| `leaflet-maplibre-gl.js` (new) | small | **2.7 KB** |

[VERIFIED: `curl -H "Accept-Encoding: gzip" -o /dev/null -w %{size_download}`]

Net addition: **~288 KB gzipped**, roughly a 7× increase in map-library payload, all render-blocking-ish at the bottom of `<body>`. Plus new runtime fetches the raster path didn't make: the style JSON, the TileJSON, glyph `.pbf` ranges per font stack, and the sprite sheet. On a congested mobile network during an actual emergency this is a real trade. It is a legitimate one (vector tiles are far smaller per-tile than raster and cache better across zooms), but it should be a stated, accepted cost rather than an unnoticed one. Consider `<link rel="preconnect" href="https://tiles.openfreemap.org">`.

### Pitfall 6: CSP would need `worker-src blob:` (not blocking today)

MapLibre spawns its tile-decode worker from a Blob URL — `new Blob([workerBundleString], { type: 'text/javascript' })` + `URL.createObjectURL`. [VERIFIED: grepped the bundle]. This app currently sends **no** `Content-Security-Policy` header (`grep -rn "Content-Security-Policy" internal/ cmd/ web/` → zero hits; the only response headers set anywhere are `Cache-Control` at `internal/api/router.go:94` and two `Content-Type`s). So nothing breaks now — but any future CSP hardening plan must allow `worker-src blob:` (and `script-src https://unpkg.com`), or switch to the `maplibre-gl-csp.js` build shipped in the same dist. Worth a one-line note in whatever tracks future security work.

### Pitfall 7: pan smoothness on vector tiles (cosmetic, monitor only)

[Issue #63](https://github.com/maplibre/maplibre-gl-leaflet/issues/63) — *"Map panning is choppy/jittery when using MapLibre-GL-Leaflet with vector tiles"*, open since Dec 2024. Structural cause: Leaflet owns the pan gesture and the bridge repositions/`jumpTo`s the GL map on a 32 ms throttle, so the canvas trails the DOM by up to a frame. Adjustable via the `updateInterval` option (default 32) at a CPU cost. Related: [#26](https://github.com/maplibre/maplibre-gl-leaflet/issues/26) polyline drift on pan/zoom (this app draws no polylines). Not a blocker — flag for the UAT human-check ("does panning feel smooth?") rather than pre-emptively tuning.

---

## Runtime State Inventory

Not applicable — greenfield frontend change, no rename/refactor/migration.

| Category | Items Found | Action Required |
|---|---|---|
| Stored data | None — this change touches no database, no schema, no stored string. | none |
| Live service config | None — the tile provider is a public static-file CDN with no account, no key, no dashboard. | none |
| OS-registered state | None. | none |
| Secrets/env vars | None — OpenFreeMap requires no API key (verified: the style URL returns 200 with no auth header). | none |
| Build artifacts | None — no build step, no bundler, no generated assets. Go's `embed.FS` picks up edited JS/CSS/templates on the next `go build`. | none |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|---|---|---|---|---|
| `tiles.openfreemap.org` (style + TileJSON + tiles + fonts + sprites) | Basemap rendering | ✓ | style spec v8, TileJSON 3.0.0, planet build `20260830_080001_pt` | none (no basemap without it) |
| CORS on OpenFreeMap | Browser fetch of style/tiles | ✓ | `access-control-allow-origin: *` | — |
| `unpkg.com` | CDN for all three vendor assets | ✓ | already relied on for Leaflet | jsdelivr (would change all SRI URLs; not needed) |
| WebGL2 in the visitor's browser | MapLibre rendering | **varies by client** | — | **none built in** — see Pitfall 4 |
| `curl` + `openssl` | Computing SRI hashes at implementation time | ✓ (used this session) | — | — |
| No CSP header configured | MapLibre blob worker | ✓ (none present) | — | `maplibre-gl-csp.js` build |

**Missing dependencies with no fallback:** WebGL2 on the client. This is the one genuine availability regression introduced by this change.

---

## Existing Code Impact

### `web/static/js/map.js`

- **Line 24:** `map = L.map(container);` → add the options object (`maxBounds`, `maxBoundsViscosity`, `minZoom: 1`, `maxZoom: 19`).
- **Lines 25-29:** the `L.tileLayer(...)` call → replaced by `L.maplibreGL({...}).addTo(map)`.
- **Everything else is unaffected.** Full read of the file confirms no raster-tile-specific dependency: no `tileload`/`tileerror`/`tilelayer` event handlers, no `getPane('tilePane')` access, no z-index assumptions. `L.divIcon` (112), `L.marker` (126), `bindPopup(DOM element)` (127), `setPopupContent(DOM element)` (122), `map.removeLayer` (153), `map.flyTo` (166), `marker.openPopup()` (167), `marker.getElement()` (177) are all pure Leaflet marker/popup API, entirely orthogonal to which layer paints the basemap.
- **XSS convention preserved:** `bindPopup`/`setPopupContent` still receive DOM `Element`s built via `Pinalert.setText`. The bridge touches none of this. [VERIFIED: read both files in full]

### `web/static/js/modal.js`

- **Line 318:** `modalMap = L.map(modalMapEl);` → add the same options object.
- **Lines 319-323:** the `L.tileLayer(...)` call → replaced, **and deferred until the container has size** (Pitfall 1).
- `modalMap.invalidateSize()` at lines 339 and 356 stays — the bridge listens for Leaflet's `resize` event (`getEvents()` line 70 → `_resize` → `_transitionEnd`) and re-sizes/re-jumps the GL map accordingly, so `invalidateSize()` correctly propagates.
- `modalMap.on('click', ...)` (line 358, tap-to-place) is unaffected — `interactive: false` means the canvas swallows nothing.
- `L.marker(..., { draggable: true })` (line 302) and `dragend` (303) are marker-pane behaviour, unaffected.
- No tile-event handlers, no pane access.

### `web/js_contract_test.go`

The `L.tileLayer(` anchor disappears from both files. Full rewrite guidance in **Validation Architecture** below.

### `web/static/css/*.css`

**No changes required.** Verified:
- Zero `.leaflet-*` selectors exist anywhere in `main.css`, `modal.css`, or `feed.css` (`grep -rn "leaflet" web/static/css/` → no matches), so there is no override to update.
- `maplibre-gl.css` contains **62 distinct class selectors, all `.maplibregl-*` prefixed**, plus two non-class `.org`/`.w3` fragments from embedded namespace URLs. Zero collisions with `.leaflet-*`, zero with any project class, zero `mapboxgl` legacy names. [VERIFIED: parsed the stylesheet]
- Container sizing already satisfied: `#map { height: 100vh; height: 100dvh }` (`main.css:409`) and `#modal-map { height: 220px }` (`modal.css:22`). MapLibre needs a sized container plus `position: relative` and `overflow: hidden` on its own wrapper — and it supplies those itself via `.maplibregl-map` in its own stylesheet, applied to the div the bridge hands it. Nothing is asked of the project's CSS.

**Unrelated pre-existing gap, noted not fixed:** `map.js` applies `.map-pin--highlight` (line 181) and builds `.map-popup` / `.map-popup__meta` / `.map-popup__description` (92-108), but **no stylesheet defines any of these four classes**. That predates this plan and is orthogonal to it — flagging for the planner's awareness only; do not scope-creep into it here.

---

## Code Examples

### Shared constants (suggest a single definition, e.g. in `app.js`'s `Pinalert` namespace)

```js
// Source: https://openfreemap.org (attribution requirement) +
//         https://tiles.openfreemap.org/planet (TileJSON attribution field, verified)
var OFM_STYLE_URL = 'https://tiles.openfreemap.org/styles/liberty';
var OFM_ATTRIBUTION =
  '<a href="https://openfreemap.org" target="_blank" rel="noopener">OpenFreeMap</a> ' +
  '<a href="https://www.openmaptiles.org/" target="_blank" rel="noopener">&copy; OpenMapTiles</a> ' +
  'Data from <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a>';

// Maintainer-sanctioned boilerplate — see examples/basic-v5.html.
// maxZoom is NOT part of that boilerplate; it is here because the GL layer
// registers no zoom limit (unlike L.tileLayer), so without it getMaxZoom()
// returns Infinity.
var MAP_BASE_OPTIONS = {
  maxBounds: [[180, -Infinity], [-180, Infinity]], // avoid max-latitude issues with MapLibre GL
  maxBoundsViscosity: 1,                           // make the bounds "solid"
  minZoom: 1,                                      // prevent sync issues at zoom 0
  maxZoom: 19
};
```

### `map.js` — the replacement

```js
// Source: @maplibre/maplibre-gl-leaflet@0.1.4 examples/basic-v5.html (UMD, browser-global setup)
map = L.map(container, MAP_BASE_OPTIONS);

L.maplibreGL({
  style: OFM_STYLE_URL,
  attributionControl: { customAttribution: OFM_ATTRIBUTION }
}).addTo(map);
```

### `modal.js` — the replacement, with the hidden-container guard

```js
if (!modalMap) {
  modalMap = L.map(modalMapEl, MAP_BASE_OPTIONS);
  // The modal is un-hidden immediately before initLocation() runs, but the
  // GL context's drawing buffer is sized at construction from
  // container.clientWidth/Height — so defer to the next frame, after layout
  // has flushed, rather than initialising WebGL against a 0x0 box.
  window.requestAnimationFrame(function () {
    modalMap.invalidateSize();
    L.maplibreGL({
      style: OFM_STYLE_URL,
      attributionControl: { customAttribution: OFM_ATTRIBUTION }
    }).addTo(modalMap);
  });
}
```

### Verified upstream reference (the canonical no-build-step setup)

```html
<!-- Source: unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/examples/basic-v5.html -->
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<link rel="stylesheet" href="https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css" />
<script src="https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js"></script>
<!-- The UMD adapter supports the browser globals used by MapLibre GL JS v2-v5. -->
<script src="https://unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js"></script>
<script>
  var map = L.map('map', { maxBounds: [[180,-Infinity],[-180,Infinity]], maxBoundsViscosity: 1, minZoom: 1 })
             .setView([38.912753, -77.032194], 2);
  L.marker([38.912753, -77.032194]).bindPopup('...').addTo(map).openPopup();
  var gl = L.maplibreGL({ style: 'https://demotiles.maplibre.org/style.json' }).addTo(map);
</script>
```

---

## Validation Architecture

### Test Framework

| Property | Value |
|---|---|
| Framework | Go stdlib `testing` (package `web`, static source inspection over `embed.FS`) |
| Config file | none — `go test` |
| Quick run command | `go test ./web/ -run TestMap -v` |
| Full suite command | `go test ./...` |

### Answering open question 4: yes, an equivalent static test is straightforward

**The anchor string is `L.maplibreGL(`** — confirmed at the source level, not inferred:

```js
// leaflet-maplibre-gl.js:223-229 — VERIFIED
var maplibreGL = function(options) { return new MaplibreGL(options); };
if (Object.isExtensible(L)) {
    L.MaplibreGL = MaplibreGL;
    L.maplibreGL = maplibreGL;
}
```

The UMD build attaches to the `L` namespace (Leaflet plugin convention). The bare `maplibreGL(` form appears only on the ESM path, where `L` is a module namespace object and therefore not extensible. Since the recommendation is the UMD path, **`L.maplibreGL(` is the correct and only anchor.**

Every structural property the existing test relies on survives:
- Same package, same `embed.FS` walk over `static/js`.
- Same `stripCSSComments` reuse — and the same reasoning still holds verbatim: the new `style:` value is `https://tiles.openfreemap.org/...`, which contains `//`, so a line-comment stripper would still truncate it. Block-comments-only remains correct.
- Same `.addTo(` window bound (the codebase's `L.maplibreGL({...}).addTo(map)` shape matches `L.tileLayer({...}).addTo(map)` exactly).
- Same "more than one per file → `t.Fatalf`" discipline.
- Same trailing `seen[module]` loop over `map.js` + `modal.js` so the test cannot pass vacuously if a call is deleted.

### Phase Requirements → Test Map

| Req | Behavior | Test Type | Automated Command | File |
|---|---|---|---|---|
| R1 | Both JS modules construct a MapLibre GL layer via the bridge | unit (static) | `go test ./web/ -run TestMapLayersUseOpenFreeMapVectorStyle` | rewrite in `web/js_contract_test.go` |
| R2 | The style URL is exactly OpenFreeMap's liberty endpoint (catches a silent re-point to an unreviewed host) | unit (static) | same | same |
| R3 | Both `L.map()` calls set a positive `maxZoom` (replaces the lost `L.tileLayer` maxZoom) | unit (static) | `go test ./web/ -run TestMapsDeclareMaxZoom` | same file, **new** test |
| R4 | The template loads all three vendor scripts, in order, each with `integrity=` and `crossorigin=` | unit (static) | `go test ./web/ -run TestVendorScriptsArePinnedWithSRI` | `web/` template-contract test, **new** |
| R5 | `#map` still has a self-resolving height | unit (static) | existing `TestPrimaryMapHasResolvedHeight` | `web/css_contract_test.go` — unchanged, still passes |
| R6 | Basemap actually renders; pins/popups/FAB on top; panning smooth; attribution shows once | manual | — | UAT — WebGL rendering cannot be asserted statically |

### Test rewrite specifics

**Rename** `TestTileLayersRequestRetinaTiles` → `TestMapLayersUseOpenFreeMapVectorStyle`.

**Delete outright** (do not adapt): the `detectRetina` presence/value assertions, the `maxZoom > 0` assertion *inside the layer options*, and the long doc comment explaining Leaflet's retina branch. The rationale for deletion — not a silent drop — is that `pixelRatio` "Defaults to `devicePixelRatio`" (`maplibre-gl.d.ts:11287`), so the entire premise the old test guarded (a raster host with no `@2x` variant) no longer exists. Say this in the new doc comment.

**Add:**
- `const maplibreLayerAnchor = "L.maplibreGL("`.
- Within the `L.maplibreGL(` → `.addTo(` window, read the `style` option with the existing `readOptionValue` helper and assert it equals `'https://tiles.openfreemap.org/styles/liberty'` (quotes trimmed). This is the "someone silently repoints the tile host" gate the task asked for.
- Optionally assert `customAttribution` is present and non-empty in the same window — cheap, and it guards the legal-ish attribution requirement.
- **New `TestMapsDeclareMaxZoom`**: anchor on `L.map(`, bound the window to the closing `)`/`;`, assert a `maxZoom` option is present and parses to a positive int. This is the direct replacement for the assertion being lost, and it guards a real regression (unbounded pinch-zoom), not just a string.

**Preserve verbatim** the existing test's honest-limits paragraph pattern: static inspection proves the call is constructed with the right style URL; it cannot prove WebGL initialised, tiles fetched, or anything rendered. Those belong to UAT.

**New template test (R4)** is genuinely valuable here because the load *order* is a correctness constraint (Pattern 1), not a style preference: walk `TemplatesFS`, find `index.html.tmpl`, assert `leaflet.js` appears before `maplibre-gl.js` appears before `leaflet-maplibre-gl.js`, and that each of the three `<script>` tags carries `integrity="sha256-` and `crossorigin="anonymous"`. A reordering regression would otherwise only surface as a runtime `L.maplibreGL is not a function` in a browser nobody ran.

### Sampling Rate

- **Per task commit:** `go test ./web/ -run 'TestMap|TestVendor|TestPrimaryMap'`
- **Per wave merge:** `go test ./...`
- **Phase gate:** full suite green + manual UAT before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] None — `web/js_contract_test.go` and `web/css_contract_test.go` already exist with the `embed.FS` + `stripCSSComments` pattern; this plan edits and extends them rather than bootstrapping infrastructure.

---

## Security Domain

`security_enforcement: true`, `security_asvs_level: 1`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---|---|---|
| V2 Authentication | no | No auth in this change (anonymous access model unchanged). |
| V3 Session Management | no | Untouched. |
| V4 Access Control | no | Untouched. |
| V5 Input Validation / Output Encoding | **yes** | Report-authored content continues to reach the DOM only via `Pinalert.setText` and `bindPopup(Element)`. The one HTML-inserted string (`customAttribution`) is a fixed developer-authored literal with zero user input — same trust level as the `L.tileLayer` attribution string it replaces. |
| V6 Cryptography | no | No crypto. SRI hashing is integrity, covered under V14. |
| V14 Configuration / Supply Chain | **yes** | **The main security surface of this change.** Three new third-party CDN assets. Controls: exact version pins (no ranges, no `@latest`), `integrity="sha256-…"` on every tag, `crossorigin="anonymous"`, hashes computed independently of the CDN's own metadata, packages sourced from the `maplibre` org with SLSA provenance + npm trusted-publisher OIDC. |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation | Status in this plan |
|---|---|---|---|
| Compromised/substituted CDN asset | Tampering | SRI + exact version pin | Applied to all three tags |
| Slopsquatted / typosquatted package | Tampering | Scoped `@maplibre/*` name from the renderer's own org; provenance attestation verified | Verified this session |
| Silent re-point of the tile host to an untrusted server | Tampering, Information Disclosure | Static Go test asserting the exact style URL (R2) | New assertion in this plan |
| XSS via attribution HTML | Tampering | Fixed literal, no user input, `rel="noopener"` on `target="_blank"` links | Applied |
| Third-party visibility into visitor location | Information Disclosure | **Unchanged trust posture** — tile requests already leak approximate viewport to `tile.openstreetmap.org`; this swaps *which* third party (OpenFreeMap/Cloudflare) sees it, not *whether*. Worth one sentence in the plan; not a new class of exposure. |
| Denial of basemap on WebGL-less clients | Denial of Service (availability) | See Pitfall 4 — currently unmitigated; requires a planner decision |
| Future CSP breaking the blob worker | — | `worker-src blob:` or the `-csp` build | Documented, not blocking (no CSP today) |

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| Raster 256px PNG tiles from `{s}.tile.openstreetmap.org` + `detectRetina` upscaling workaround | Vector `.pbf` tiles rendered client-side by WebGL at native `devicePixelRatio` | Industry shift ~2015-2020; MapLibre forked from Mapbox GL JS in 2020 after the v2 license change | Crisp at any DPR, smaller per-tile payloads, restyleable; costs a WebGL dependency and ~275 KB gz of renderer |
| `mapbox-gl-leaflet` | `@maplibre/maplibre-gl-leaflet` | Post-2020 fork | The MapLibre plugin is the maintained descendant; the upstream `mapbox-gl-leaflet` source is still cited in this plugin's own issue threads (#58) |
| `maplibre-gl` UMD browser global | ESM-only distribution | **`maplibre-gl` v6.0.0** | The UMD build was dropped. This is precisely why v5.24.0 is the correct pin for a no-build-step project. |
| `{s}.` subdomain sharding in tile URLs (in the code being deleted) | Obsolete under HTTP/2 multiplexing; OSM's tile policy now discourages it | ~2016 onward | Moot — the URL disappears entirely |

**Deprecated / removed by this change:**
- `detectRetina` — a `L.TileLayer`-only option with no GL counterpart. Delete, don't port.
- `{s}` subdomain sharding.
- Direct dependence on `tile.openstreetmap.org` (whose usage policy this project was previously operating under).

---

## Project Constraints (from `.claude/CLAUDE.md`)

| Directive | Compliance |
|---|---|
| "Budget: Free tiers only — no paid map API" | ✓ OpenFreeMap: no signup, no API key, no rate limits, static-file hosting. Verified: style URL returns 200 with no auth. |
| "server-rendered HTML (`html/template`) + vanilla JS" | ✓ No build step, no npm, no bundler introduced. Three CDN script tags. |
| "GSD Workflow Enforcement — no direct repo edits outside a GSD workflow" | ✓ This is research only; no source file was modified. |
| "Conventions: not yet established" | — |
| Existing security posture: SRI + `crossorigin` on every third-party tag | ✓ Extended to all three new tags; hashes computed with the method validated against the existing Leaflet hashes. |
| XSS convention: report-authored content as DOM text/elements, never HTML strings | ✓ Preserved. `bindPopup(Element)` / `Pinalert.setText` unchanged. The one HTML string added contains no user input. |

### CLAUDE.md edits the planner must make (exact current wording, for a minimal diff)

**Three** locations reference the current stack, not one. Quoted verbatim from `.claude/CLAUDE.md`:

1. **Line 32** (inside `## Project` → `### Constraints` → the `- **Tech stack**:` bullet):
   > `computation, server-rendered HTML (\`html/template\`) + vanilla JS, Leaflet.js/OpenStreetMap for`
   > `  the map. Chosen for a clean API-first architecture that's a good portfolio signal and doesn't`

   The phrase to change is **`Leaflet.js/OpenStreetMap for the map`** (it wraps across lines 32-33).

2. **Line 73** (`## Recommended Stack` → `### Core Technologies` table, the Leaflet row) — currently:
   > `| Leaflet.js | **1.9.4** (verified via npm registry \`leaflet@latest\`, no 2.x shipped as of 2026-09) | Map rendering | Already chosen. No API key, no billing, pairs with free OSM tile servers (mind OSM's tile-usage policy at higher traffic — a portfolio demo is well within it). |`

   The clause **"pairs with free OSM tile servers (mind OSM's tile-usage policy at higher traffic — a portfolio demo is well within it)"** becomes inaccurate: the app no longer touches OSM tile servers. Two new rows are also needed for `maplibre-gl` 5.24.0 and `@maplibre/maplibre-gl-leaflet` 0.1.4.

3. **Line 163** (`## Sources`) — the npm-registry line for Leaflet. New source lines should be added for the two new packages and OpenFreeMap.

Also consider a `## What NOT to Use` row for **`maplibre-gl` v6 in a no-build-step project** (no UMD build) — that is exactly the class of trap that table exists to record, and it is the single most likely way this decision gets silently reversed later.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | MapLibre v5's WebGL1 fallback context does not reliably produce correct full rendering (v5 effectively requires WebGL2) | Pitfall 4 | If WebGL1 in fact renders fine, the availability regression is smaller than stated — the mitigation decision could be relaxed. Verified only that the code tries `webgl2` then `webgl` and throws if both fail. |
| A2 | MapLibre propagates a vector source's TileJSON `attribution` onto `getSource(id).attribution` | Pattern 5 (auto path) | Only affects the *rejected* auto path. The recommended explicit `customAttribution` makes this irrelevant. |
| A3 | `#report-modal`'s layout has not necessarily flushed by the time `initLocation()` runs in the same task | Pitfall 1 | If layout has in fact flushed, the `requestAnimationFrame` deferral is merely harmless belt-and-braces, not required. Cheap insurance either way — the existing `setTimeout(0)` + `invalidateSize()` calls in the current code are circumstantial evidence the concern is real. |
| A4 | Vector overzoom past OpenFreeMap's `maxzoom: 14` renders acceptably at Leaflet zoom 16 | Pattern 4 | If labels thin out unacceptably at zoom 16, `flyTo(..., 16)` and the modal's `setView(..., 16)` may want lowering to ~15. A UAT visual check settles it. |
| A5 | Marker `dragend`/`click` and popup positioning stay pixel-accurate relative to the GL canvas during zoom animation | Existing Code Impact | Issue #26 (polyline drift) suggests overlay/canvas registration can drift mid-animation, though markers are DOM-positioned by Leaflet independently of the canvas. UAT should include "open a popup, zoom, confirm it stays on its pin." |

---

## Open Questions

1. **WebGL-less fallback: accept the regression, or keep `L.tileLayer` as a fallback branch?**
   - What we know: MapLibre throws with no basemap if WebGL is unavailable; the audience is Indian mobile users, plausibly arriving via in-app WebViews.
   - What's unclear: how much of the realistic audience that is. No telemetry exists.
   - Recommendation: **escalate to the user during planning.** The cheapest option is retaining the existing `L.tileLayer` call inside a `hasWebGL()` false branch — but that puts two basemap constructions in each file, which the static test's "at most one per file" `Fatalf` currently forbids. The test's shape depends on this answer, so it must be decided *before* the test is rewritten, not after.

2. **Does the planner want `updateInterval` tuned below the 32 ms default?**
   - What we know: issue #63 reports choppy panning on vector tiles; `updateInterval` is the exposed knob.
   - Recommendation: ship the default, put "does panning feel smooth on a phone?" in UAT, tune only if it fails. Pre-emptive tuning costs CPU on the low-end devices this app targets.

3. **Should `<link rel="preconnect" href="https://tiles.openfreemap.org">` be added?**
   - Recommendation: yes, cheap and clearly beneficial given the new style/TileJSON/glyph/sprite round-trips — but it is a separate one-line concern the planner may prefer to fold into the same template task.

---

## Sources

### Primary (HIGH confidence — fetched and read this session)
- `unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js` — the full 234-line UMD source. Settled: pane default, `interactive: false`, `attributionControl: false` forcing, `getAttribution()` logic, `L.maplibreGL` global assignment, the `!this._map` guard, the −1 zoom offset.
- `unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/examples/basic-v5.html` and `examples/basic.html` — the two mutually-exclusive setups and the maintainers' "v2-v5" UMD statement.
- `registry.npmjs.org/@maplibre/maplibre-gl-leaflet/0.1.4` — peerDependencies, `main` field, SLSA provenance, trusted-publisher OIDC.
- `registry.npmjs.org/maplibre-gl` — dist-tags, 5.x version list, publish dates.
- unpkg HTTP status: `maplibre-gl@6.8.0/dist/maplibre-gl.js` → 404; `@5.24.0` → 200.
- `unpkg.com/maplibre-gl@6.8.0/dist/?meta` and `@5.24.0/dist/?meta` — complete file listings proving ESM-only vs UMD-present.
- `unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.d.ts` — `pixelRatio` "Defaults to `devicePixelRatio`" (line 11287).
- `unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css` — all 62 class selectors, `.maplibregl-map` / `.maplibregl-canvas` rules.
- `unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js` — WebGL2/WebGL context strings, blob-worker construction.
- `unpkg.com/leaflet@1.9.4/dist/leaflet-src.js` — `getMaxZoom` (3967), `_addZoomLimit` (7012), `GridLayer.beforeAdd` (11268).
- `https://tiles.openfreemap.org/styles/liberty` — style JSON, sources, glyphs, sprite, 111 layers; response headers (CORS, cache).
- `https://tiles.openfreemap.org/planet` — TileJSON with the exact `attribution` string, `maxzoom: 14`.
- Locally computed SRI hashes, method validated against the two existing Leaflet hashes in `index.html.tmpl`.
- Repo files read in full: `web/static/js/map.js`, `web/static/js/modal.js`, `web/templates/index.html.tmpl`, `web/js_contract_test.go`, `web/static/css/main.css`, `.claude/CLAUDE.md`; grepped `web/static/css/`, `internal/`, `cmd/`.

### Secondary (MEDIUM)
- `openfreemap.org` — attribution requirement wording ("OpenFreeMap © OpenMapTiles Data from OpenStreetMap"; the OpenFreeMap portion is optional but encouraged; "If using MapLibre, attribution is added automatically"). Corroborated by the live TileJSON's own `attribution` field, which matches.
- GitHub issue tracker `maplibre/maplibre-gl-leaflet` — 16 open issues enumerated; #52, #58, #63, #67 read in full.

### Tertiary (LOW)
- None relied upon. No claim in this document rests on WebSearch alone.

---

## Metadata

**Confidence breakdown:**
- Version/UMD decision: **HIGH** — CDN 404/200 plus the plugin's own two examples; not inference.
- Attribution, pane/z-index, `L.maplibreGL` anchor name: **HIGH** — read directly from the published plugin source.
- Retina/`pixelRatio`: **HIGH** — quoted from the shipped `.d.ts`.
- `maxZoom` regression: **HIGH** — traced through Leaflet 1.9.4 source (`getMaxZoom` → `_zoomBoundLayers` → `GridLayer.beforeAdd`).
- CSS non-collision, container sizing: **HIGH** — both stylesheets parsed.
- Modal hidden-container risk: **MEDIUM** — the mechanism is certain (WebGL buffer sized at construction); whether it manifests in this exact code path is A3, unverified without a browser. The mitigation is cheap and harmless either way.
- WebGL2 hard requirement severity: **MEDIUM** — the throw path is verified in the bundle; the WebGL1-fallback adequacy is A1.
- Pan-smoothness / overzoom aesthetics: **LOW** — UAT questions, not researchable statically.

**Research date:** 2026-09-08
**Valid until:** ~2026-10-08 (30 days). Re-check if `maplibre-gl` v6 ever restores a UMD build, or if `@maplibre/maplibre-gl-leaflet` publishes past 0.1.4 — both would reopen the version decision.
