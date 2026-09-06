---
phase: 01-foundation-report-map
plan: 04
subsystem: frontend-walking-skeleton
tags: [html-template, leaflet, vanilla-js, go-embed, dom-contract, xss-safety]
status: complete
requirements: [FOUND-02, FOUND-04, FOUND-03]

dependency-graph:
  requires:
    - phase: 01-foundation-report-map (plan 01-01)
      provides: Go module scaffold, migrations, testutil
    - phase: 01-foundation-report-map (plan 01-02)
      provides: web/static/css design-token layer, 9 committed Lucide icons
    - phase: 01-foundation-report-map (plan 01-03)
      provides: session.Manager, service.ReportService, api.NewRouter/Deps, POST/GET /api/reports
  provides:
    - "web/templates/index.html.tmpl — the DOM contract (app-shell, map, fab-report, modal skeleton, feed states, toast)"
    - "internal/api/handlers.Page/PageConfig/ParsePageTemplate"
    - "web/embed.go — package web bridging the go:embed directory boundary (TemplatesFS, StaticFS)"
    - "window.Pinalert — shared client store (app.js)"
    - "window.PinalertMap — Leaflet map (map.js)"
    - "web/static/js/modal.js — thin end-to-end submission modal"
    - "GET / and GET /static/* routes"
  affects:
    - "01-05 (report modal — replaces modal.js wholesale, must not edit index.html.tmpl or app.js/map.js)"
    - "01-06 (feed list — replaces feed.js, reads Pinalert.state via subscribe, must not edit index.html.tmpl)"
    - "01-07 (OpenAPI docs / router — router.go was touched by this plan; sequenced after this plan per orchestrator's overlap detection)"

tech-stack:
  added: []
  patterns:
    - "go:embed bridge package: web/embed.go exports TemplatesFS/StaticFS since handlers/router can't embed a sibling of the repo root directly — mirrors internal/store/migrations.go's MigrationsFS pattern"
    - "One GET /api/reports fetch owned entirely by app.js; map.js (and future feed.js) read only from Pinalert.state via subscribe — no per-panel duplicate polling"
    - "DOM safety: every report-authored string reaches the DOM via Pinalert.setText (textContent) or Leaflet's Element-based bindPopup/setPopupContent path (verified against Leaflet 1.9.4 source: passing an Element triggers appendChild, not innerHTML) — never a markup string"
    - "SRI-pinned third-party CDN: Leaflet 1.9.4 CSS/JS integrity hashes computed locally via openssl and cross-checked byte-for-byte against leafletjs.com/download.html's published hashes before use"

key-files:
  created:
    - web/embed.go
    - web/templates/index.html.tmpl
    - internal/api/handlers/page.go
    - internal/api/handlers/page_test.go
    - web/static/js/app.js
    - web/static/js/map.js
    - web/static/js/modal.js
    - web/static/js/feed.js
  modified:
    - internal/api/router.go
    - cmd/server/main.go

decisions:
  - "Added web/embed.go (not in the plan's files_modified list) because //go:embed cannot reference a path outside the embedding package's own directory — internal/api/handlers/page.go cannot embed ../../../web/templates directly. Mirrors the existing internal/store/migrations.go MigrationsFS pattern exactly. Rule 3 (blocking)."
  - "Updated cmd/server/main.go (not in the plan's files_modified list) to construct the parsed template and PageConfig and pass them into api.Deps. Without this, Deps.Template stays nil and the binary panics on GET / — but Task 3's own human-check requires `make run` to serve a working shell end to end, so this was a compile-and-run blocker, not scope creep. Rule 3 (blocking)."
  - "Leaflet 1.9.4 SRI hashes were not invented — fetched leaflet.js/leaflet.css from unpkg, computed sha256 digests locally with openssl, and cross-checked the result byte-for-byte against leafletjs.com/download.html's own published integrity attributes before writing them into index.html.tmpl. A wrong hash would make the browser silently refuse to load Leaflet at all, which no automated grep-based verify catches."
  - "ageStage boundary handling: fresh is remainingFraction > 0.25 (strict), aging is 0.125 <= remainingFraction <= 0.25 (inclusive at 0.125), stale is remainingFraction < 0.125 (strict) — chosen so the acceptance criteria's 'aging between 12.5% and 25%' and 'stale below 12.5%' are both true at the exact 0.125 boundary."
  - "CATEGORY_LABELS keys/values transcribed verbatim from 01-UI-SPEC.md's Icon Mapping table (e.g. 'Storm/Cyclone damage' for storm_cyclone), not invented independently."
  - "Popups and marker badges are passed to Leaflet as DOM Element objects (bindPopup(element), setPopupContent(element), L.divIcon({html: element})), not markup strings — verified directly against Leaflet 1.9.4's DivIcon.js and DivOverlay.js source (github.com/Leaflet/Leaflet@v1.9.4) that an Element triggers appendChild rather than innerHTML internally, so this path never re-introduces the XSS sink the project-level grep gate is designed to catch."

metrics:
  duration: "~70 min"
  completed: 2026-09-06
---

# Phase 1 Plan 04: Page Shell, Shared Client Store, Live Map, and Thin Submission Modal Summary

Closes the Walking Skeleton: `html/template` app shell embedded via a self-contained Go binary, a single shared `window.Pinalert` client store that owns the one `GET /api/reports` fetch, a live Leaflet/OpenStreetMap map rendering severity-coloured age-aware badge pins, and a thin-but-real submission modal — browser → API → Postgres → back to the map, with no signup, proven against a running server over real HTTP.

## What Was Built

**Task 1 — Page shell, static serving, and the DOM contract**
(`web/templates/index.html.tmpl`, `internal/api/handlers/page.go`, `page_test.go`, `internal/api/router.go`, `web/static/js/feed.js`, plus the additive `web/embed.go` and `cmd/server/main.go` deviations): the full DOM contract wave 4 builds against — `app-shell`, `map`, `fab-report`, the list pane's skeleton/empty/error states, the modal skeleton (category grid, severity slider, shelter fields, discard-confirm), and the toast — all with the exact ids and Copywriting Contract strings. Three stylesheets in declared order, Leaflet 1.9.4 pinned via CDN with SRI `integrity` hashes I computed locally and cross-checked against Leaflet's own published values, and four project scripts deferred with `app.js` loaded first. `handlers.Page` renders the embedded template with a server-configurable fallback centre (Bengaluru) and default radius. `GET /static/*` serves the embedded `web/static` tree with a 1-hour cache header. `TestPageShellServesDOMContract` asserts every contract id, all three stylesheets, and script load order.

**Task 2 — The shared client store and the live Leaflet map**
(`web/static/js/app.js`, `web/static/js/map.js`): `window.Pinalert` is the single shared store — `CATEGORIES`/`SEVERITIES`/`CAPACITY_STATUSES`/`SEVERITY_LABELS`/`CATEGORY_LABELS`, `config` (read from the shell's `data-*` attributes), `state{reports,status,error,selectedId,center}`, `subscribe`/`setCenter`/`fetchReports`/`startPolling`/`submitReport`/`select`/`onSelect`, and the formatting helpers `severityClass`/`iconPath`/`relativeTime`/`ageStage`/`setText`. `window.PinalertMap` initialises Leaflet on `#map`, centres on geolocation or the configured fallback, and reconciles `L.divIcon` markers by report id across polls (never rebuilding the layer, so an open popup survives a 30-second refresh). Badge markup and popups are built from fixed strings, the validated icon path, and DOM element construction only — verified against Leaflet's own source that passing an `Element` to `bindPopup`/`setPopupContent`/`L.divIcon({html})` triggers `appendChild`, not `innerHTML`.

**Task 3 — Minimal working submission modal**
(`web/static/js/modal.js`): the FAB opens a modal over the still-visible, dimmed map with focus moved in and Tab trapped; Escape/Cancel close it and restore focus. GPS pre-fills a draggable marker at zoom 16, `dragend` updates the coordinate readout live; denial/unavailability shows the exact inline notice and falls back to tap-to-place, never blocking submission. Category is a plain single-select tile list (the 3×3 grid is 01-05's job); severity wires the existing native range input, updating `aria-valuetext` and the readout on every `input`. Submit disables the button, shows "Posting…", calls `Pinalert.submitReport`, and on success closes the modal, shows the "Report posted." toast, and lets the store refresh redraw the map with no page reload; on failure the modal stays open with the server's `{field, message}` rendered into `#form-error`. The thin modal never sends shelter-capacity fields.

## End-to-End Proof

Ran the real binary locally (`DATABASE_URL` against the local Postgres, `ENV=development`):
- `GET /` → 200, contains `id="map"` exactly once, all contract ids present.
- `GET /static/css/main.css` → 200, `GET /static/icons/flood.svg` → 200 (both from the embedded filesystem).
- `POST /api/reports` (no cookie) → 201 with a full report body, `Set-Cookie` issued.
- `GET /api/reports?lat=12.9716&lon=77.5946&radius_km=10` (with the issued cookie) → 200, returns exactly the just-created report with `distance_km: 0`.

This proves the full stack — browser-shaped HTTP calls through the real router, real `ReportService`, real Postgres, and back — end to end, independent of the JS/Leaflet layer (which needs a real browser to execute and is covered by the plan's `<human-check>` below).

## Task Commits

1. **Task 1: Page shell, static serving, and the DOM contract**
   - `22588d7` (feat) — index.html.tmpl, page.go, page_test.go, router.go, web/embed.go, feed.js, main.go
2. **Task 2: The shared client store and the live Leaflet map**
   - `d987bc3` (feat) — app.js, map.js
3. **Task 3: Minimal working submission modal**
   - `9119f92` (feat) — modal.js

**Plan metadata:** committed as part of this SUMMARY (worktree mode — orchestrator handles the final docs commit after merge; STATE.md/ROADMAP.md are not touched by this agent per its instructions).

## Verification

- `go build ./...`, `go vet ./...` — clean.
- `go test ./... -v -p 1` (real Postgres, `DATABASE_URL` set) — all packages pass, including the new `TestPageShellServesDOMContract`.
- `node --check` passes on `app.js`, `map.js`, `modal.js`.
- Zero `innerHTML` occurrences (comment-stripped) across all three JS files.
- `map.js` contains zero occurrences of the reports-fetch endpoint string — the map reads only from the shared store.
- Every DOM contract id, all three stylesheet links, all four script tags (in order, `app.js` first), and the pinned-Leaflet `integrity`/`crossorigin` attributes are present in `index.html.tmpl`.
- Manual end-to-end HTTP proof above (submit → nearby round trip).

## Pending Human Verification

Task 3's `<human-check>` is not a `checkpoint:*` gate (plan is `autonomous: true`, `human_verify_mode: end-of-phase`), so execution did not stop for it — same handling 01-02 used for its icon-visual check. The following should be confirmed visually at end-of-phase, since it requires a real browser (not exercised by the automated HTTP proof above):
1. The map renders visible OSM tiles (not just an empty gray `#map` div).
2. Tapping "+" opens the modal over the dimmed map with visible focus movement.
3. The modal's embedded Leaflet map correctly sizes itself after becoming visible (the `invalidateSize()` call after a `setTimeout(0)` — Leaflet is known to mis-measure a container that was `display: none` at init time).
4. Denying the browser's location permission shows the inline notice and tap-to-place actually places a marker.
5. Choosing a category, moving the slider, typing a description, and submitting closes the modal, shows the toast, and the new pin visually appears on the map (color/badge correctness, not just an HTTP 201).
6. Switching the OS to dark mode repaints the page using the dark CSS variables from `main.css` (plan 01-02).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `web/embed.go` — `//go:embed` cannot cross a package-directory boundary**
- **Found during:** Task 1, before writing `page.go`
- **Issue:** The plan describes loading templates "via `//go:embed` from `web/templates`" inside `internal/api/handlers/page.go`, and serving `/static/*` "from an embedded `web/static` filesystem" in `internal/api/router.go`. Go's `//go:embed` directive can only reference paths inside the embedding file's own package directory — it cannot reach `../../../web/templates` from `internal/api/handlers`, so the plan as literally written does not compile.
- **Fix:** Added a small new package `web` at the repo root (`web/embed.go`) exporting `TemplatesFS` and `StaticFS`, mirroring the exact pattern `internal/store/migrations.go` already uses for goose migrations (`MigrationsFS`). `handlers.ParsePageTemplate()` and `api.staticFileServer()` both import `pinalert/web` and consume these exported `embed.FS` values instead.
- **Files added:** `web/embed.go`
- **Verification:** `go build ./...` clean; `GET /`, `GET /static/css/main.css`, `GET /static/icons/flood.svg` all return 200 against the embedded filesystem.
- **Committed in:** `22588d7` (Task 1)

**2. [Rule 3 - Blocking] Updated `cmd/server/main.go` to wire the parsed template and `PageConfig` into `Deps`**
- **Found during:** Task 1, after extending `api.Deps` with `Template`/`Page` fields
- **Issue:** `main.go` is not in the plan's `files_modified` list, but it constructs `api.Deps{...}` directly. Without updating it, `Deps.Template` stays `nil` and every `GET /` request panics inside `template.ExecuteTemplate`. Task 3's own `<human-check>` requires `make run` to serve a working app end to end, which is impossible without this change.
- **Fix:** `main.go` now calls `handlers.ParsePageTemplate()` (fail-fast via `log.Fatalf` on a parse error, consistent with the file's existing fail-fast style for `DATABASE_URL`/`SESSION_SECRET`) and passes a `PageConfig{FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10}` into `Deps`.
- **Files modified:** `cmd/server/main.go`
- **Verification:** manual end-to-end HTTP proof above; `go build ./...`/`go vet ./...` clean.
- **Committed in:** `22588d7` (Task 1)

No other deviations. All three tasks' automated verify commands pass exactly as specified in the plan (Task 1's ID/stylesheet/script grep gates, Task 2's contract-member/innerHTML/single-fetch-owner gates, Task 3's `innerHTML`/`aria-valuetext`/`draggable`/`submitReport` gates, and the full `go build && go vet && go test ./... -v -p 1` suite).

## Known Stubs

None. Every artifact this plan promises (`Pinalert`, `PinalertMap`, the DOM contract, `handlers.Page`) is fully wired to real data — the map fetches from the real `GET /api/reports` endpoint, the modal posts to the real `POST /api/reports` endpoint, and both were proven against a running server over real HTTP above. `modal.js` and `feed.js` are explicitly documented in-file as thin/placeholder and owned by plans 01-05/01-06 respectively — this is the plan's own stated scope, not an undocumented stub.

## Self-Check: PASSED

All 10 claimed files verified present on disk; all 3 commit hashes (`22588d7`, `d987bc3`, `9119f92`) verified present in `git log`.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*
