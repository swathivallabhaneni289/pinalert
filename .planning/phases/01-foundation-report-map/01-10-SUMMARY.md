---
phase: 01-foundation-report-map
plan: 10
subsystem: web-frontend
tags: [leaflet, tiles, retina, contract-test, gap-closure]
dependency-graph:
  requires: []
  provides: [retina-tile-detection, js-contract-test-package]
  affects: [web/static/js/map.js, web/static/js/modal.js]
tech-stack:
  added: []
  patterns:
    - "Leaflet detectRetina option on both tile layer constructions"
    - "Per-file anchor-count + colon/value string scan for JS contract testing, reusing the CSS contract test's block-comment stripper"
key-files:
  created:
    - web/js_contract_test.go
  modified:
    - web/static/js/map.js
    - web/static/js/modal.js
decisions:
  - "Reused stripCSSComments from web/css_contract_test.go rather than duplicating a JS comment stripper — block comment delimiters are identical between CSS and JS, and duplicating would create two sources of truth for the same parsing concern."
  - "Per-file occurrence counting instead of brace-span parsing for the tile layer options object, because the URL template's {s}/{z}/{x}/{y} placeholders put the first closing brace inside the URL string, not around the options object."
metrics:
  duration: "~25 minutes"
  completed: 2026-09-06
status: complete
---

# Phase 01 Plan 10: Retina Tile Detection Gap Closure Summary

Enabled Leaflet's `detectRetina` option on both the primary map's and the report modal's own
separate `L.tileLayer(...)` calls as a pure single-line insertion in each file, and added a new
`TestTileLayersRequestRetinaTiles` regression test that fails the build (naming the offending
file) if either construction ever loses the option or the option's boolean-true value.

## What Was Built

**`web/static/js/map.js`** — inserted `detectRetina: true,` between the existing `maxZoom: 19`
and `attribution` lines inside `PinalertMap.init()`'s tile layer construction (line 26 of the
updated file). No other line touched; the tile URL template, max-zoom value, and attribution
string are byte-identical to before.

**`web/static/js/modal.js`** — the same insertion inside `initLocation()`'s tile layer
construction for the modal's own separate Leaflet instance (line 320 of the updated file),
indented to match its one-block-deeper nesting (8 spaces). Same byte-identical guarantee on URL,
max-zoom, and attribution.

**`web/js_contract_test.go`** (new file, 132 lines — the plan's `min_lines: 90` was an estimate;
the honest implementation came in above it, so no adjustment was needed in either direction) —
`TestTileLayersRequestRetinaTiles` walks every `*.js` file under `static/js` in the embedded
`StaticFS`, strips block comments by reusing `stripCSSComments` from
`web/css_contract_test.go` (JS and CSS share identical `/* */` delimiters — duplicating the
stripper would create a second source of truth for the same parsing rule), and for each file
containing exactly one `L.tileLayer(` anchor, locates `detectRetina` and asserts its value —
read as the text between the first colon after the identifier and the next comma or closing
brace — equals the literal `true`. It fails loudly (naming the file) if: a file has more than
one tile layer construction (assumption violated), the option is missing, the option appears
more than once, or the option's value isn't exactly `true`. It also tracks which of
`static/js/map.js` and `static/js/modal.js` were seen with an anchor and fails if either is
absent, so a deleted or renamed call can't pass vacuously. The test's doc comment states
plainly that static inspection can prove the option is present and enabled but cannot prove
Leaflet actually fetched higher-density tiles at runtime — that's the human-check's job — and
records the load-bearing `maxZoom > 0` dependency from Leaflet 1.9.4's retina branch.

Per-file occurrence counting was used instead of brace-span parsing because the URL template's
`{s}`/`{z}`/`{x}`/`{y}` placeholders put the tile layer call's first closing brace inside the URL
string itself, not around the options object — a span-based approach would never reach the
option and would fail on correct code.

No inline comment was added at either call site (a hard requirement — a verify gate counts
non-comment occurrences of `detectRetina` per file and expects exactly one).

## Deviations from Plan

None — plan executed exactly as written.

## Verification Performed

1. `node --check` passed on both edited JS files before and after the change (syntax intact,
   trailing comma correct).
2. `go build ./...` and `go vet ./...` both succeeded.
3. `go test ./web/ -run 'TestTileLayersRequestRetinaTiles|TestPrimaryMapHasResolvedHeight|TestModalBackdropHiddenGuard' -v`
   — all three PASS, confirming the new test coexists cleanly with the two pre-existing CSS
   contract tests.
4. **Bidirectional bite-check (Step 4, both directions confirmed):**
   - With the option present in both files: `TestTileLayersRequestRetinaTiles` **PASSES**.
   - With the option temporarily removed from `web/static/js/modal.js` only: the test **FAILS**,
     naming `static/js/modal.js` explicitly in the failure message
     (`js_contract_test.go:85: static/js/modal.js: tile layer construction is missing the
     "detectRetina" option entirely...`).
   - The line was restored and `diff` confirmed against a pre-edit backup copy that the restored
     file is byte-identical to its intended post-fix state; the test was re-run and passed.
5. All five content gates from the plan's `<verify>` block passed on both `map.js` and
   `modal.js`: `detectRetina` appears exactly once in non-comment source in each file; it is set
   to `true` (not merely present) in each; the tile URL template
   (`https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png`) is byte-identical in each; `maxZoom: 19`
   is still present in each (the load-bearing retina-branch guard); `attribution` is still present
   in each.
6. Scope gates: `git diff --quiet HEAD -- web/static/css web/templates` — clean (no stylesheet or
   template touched). `git diff --quiet HEAD -- web/static/js/app.js web/static/js/feed.js
   web/css_contract_test.go` — clean (none of those three touched). `git status --porcelain --
   web/static/js` showed exactly 2 dirty files (`map.js`, `modal.js`).
7. `git status --porcelain` after staging showed exactly the three intended files: `A
   web/js_contract_test.go`, `M web/static/js/map.js`, `M web/static/js/modal.js`.
8. `go test ./...` — full suite passed across all packages (`internal/api/handlers`,
   `internal/service`, `internal/session`, `internal/store`, `internal/testutil`, `web`).

## Outstanding: Human Visual Verification (not blocking, not performed by this agent)

This execution ran as an autonomous worktree agent with no browser or physical display access,
so the plan's subjective visual-quality `<human-check>` step could not be performed here.
Per project config, `workflow.human_verify_mode` is `"end-of-phase"` — human verification is
deliberately batched rather than performed per-plan — and this plan carries `autonomous: true`
with no `checkpoint:*` task type, so this is an expected, non-blocking gap in this plan's
execution, not a deviation. **Automated verification is complete; the visual judgement itself is
the only outstanding item.** Concrete steps for whoever runs `/gsd-verify-work 01` next:

1. Start the server, open the app on a Retina/HiDPI display (a non-retina external monitor shows
   no change at all, since the option is a no-op there).
2. Open DevTools, disable cache, hard-reload — Leaflet reads `devicePixelRatio` once at page load,
   so the window must already be on the Retina display *before* reloading.
3. In the Network tab, filter for `tile.openstreetmap.org` and read the zoom segment of the
   requested tile paths: expect **one level higher** than the map's own zoom (map boots at 13 with
   geolocation granted / 11 on the Bengaluru fallback → expect tile requests at z14 / z12
   respectively).
4. Inspect any one tile `<img>`: natural size should read 256, displayed CSS size should read 128.
5. **The decisive question:** do street names and place labels on the primary map now look
   visibly sharper than before — not merely "no regression"? Record the verdict honestly either
   way.
6. Click the `+` FAB, repeat the sharpness judgement on the modal's own small map, then drag its
   marker and close the modal to confirm nothing else about the submit flow changed.
7. Expected-and-not-a-bug: on a Retina display the map's maximum reachable zoom is now one level
   lower (18 instead of 19) — Leaflet's retina branch trades one zoom level for four-times-denser
   tiles; do not "fix" this by raising `maxZoom` (OSM serves no z20 tiles).

This plan closes no UAT test — `01-UAT.md`'s Test 1 already passed on its literal criteria before
this plan ran, and the cosmetic tile-quality gap this plan addresses remains `pending_fix` in
that file until the human-check above is run and the gap is re-verified via `/gsd-verify-work 01`.
`01-UAT.md` itself was deliberately left untouched by this plan (its status transition is owned by
the verify-work flow, and it was already dirty in the parent worktree at session start —
editing it here would risk a merge conflict on the orchestrator's merge).

## Requirements

FOUND-02 and FOUND-04 are **not** newly marked complete by this plan — both were already Complete
before this gap-closure round, per the plan's own objective ("This plan closes no UAT test" and
does not unblock either requirement). No `requirements.mark-complete` call was made; this is
intentional, not an omission.

## Self-Check: PASSED

- FOUND: `web/js_contract_test.go` (132 lines)
- FOUND: `web/static/js/map.js` (197 lines, ≥ plan's 196-line floor)
- FOUND: `web/static/js/modal.js` (575 lines, ≥ plan's 574-line floor)
- FOUND: commit `2b17e1a` (`fix(01-10): enable retina tile fetching on both Leaflet tile layers`)
  present in `git log --oneline`
