---
phase: 01-foundation-report-map
plan: 15
subsystem: ui
tags: [css, svg, contrast, cache-busting, live-uat, contract-test]

# Dependency graph
requires:
  - phase: 01-foundation-report-map
    provides: plan 01-14's badge-contrast mechanism (`--badge-glyph-fg`, Mechanism 2, kept) and its stroke-width mechanism (Mechanism 1, reverted here after live retest found it did not fix the reported defect)
provides:
  - The actual fix for the modal category grid's reported illegibility — `.category-tile .icon-glyph { color: var(--color-text) }` (plus a selected-state counter-rule) — found by inspecting served CSS live against the human tester's real-time feedback, not by another diagnose/plan/execute round
  - 01-14's stroke-width change (2 -> 3) fully reverted across all nine icons, since it was an unrequested side effect on map-pin/feed-row badges the tester never complained about, and was not what fixed the modal
  - A process-start-time `AssetVersion` query string on every local static asset URL, so a server restart during live UAT can never again be masked by browser or Cache-Control caching
  - `.icon-badge`/`.report-row .icon-badge` glyph sizing reduced 55% -> 48% on direct human request after the color fix made the icons look "big" for the first time (previously too faint to have an opinion on)
  - `TestCategoryGlyphInkCoverageAcrossRenderContexts` removed (asserted a stroke-width floor the icons no longer clear, deliberately) — `TestBadgeGlyphContrastAcrossAgeStagesAndThemes` untouched and still passing
affects: [any future phase touching category icons, badge/age-ramp theming, static-asset caching, or the report modal/map/feed rendering]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cache-busting local static assets via a process-start-time version query string threaded from main.go through a PageConfig field into the template, leaving pinned/SRI-hashed CDN tags untouched — the minimum viable fix for '//go:embed bakes assets at compile time, and the file server also sends a 1h Cache-Control, so a restart alone cannot force a browser to see new bytes.'"
    - "When a UAT retest fails after a mechanically-verified fix, check the serving channel (running process age, browser cache) before re-diagnosing the fix itself — verified live via `ps`/`lsof`/`curl` against the actual running server, not assumed from source or from a previous test's passing gate."

key-files:
  created: []
  modified:
    - web/static/css/main.css
    - web/static/css/feed.css
    - web/static/icons/flood.svg
    - web/static/icons/earthquake.svg
    - web/static/icons/fire.svg
    - web/static/icons/storm_cyclone.svg
    - web/static/icons/road_blocked.svg
    - web/static/icons/power_outage.svg
    - web/static/icons/shelter_open.svg
    - web/static/icons/rescue_needed.svg
    - web/static/icons/other.svg
    - web/css_contract_test.go
    - internal/api/handlers/page.go
    - cmd/server/main.go
    - web/templates/index.html.tmpl

key-decisions:
  - "Consulted the advisor rather than running a third diagnose->plan->check->execute loop after 01-14's mechanically-passing fix was reported as still broken live — the loop was not converging (two rounds, same complaint), so the bottleneck was judged to be the fix strategy (chasing a computed floor) rather than diagnosis quality"
  - "Fixed the modal glyph color directly (no subagent, no new gap-closure round through /gsd-execute-phase) since the change was small, immediately verifiable against served bytes, and the whole point was confirming what a specific human on a specific browser sees right now — an isolated worktree agent cannot do that"
  - "Reverted 01-14's stroke-width change wholesale rather than forking a second, thicker-stroke icon set scoped to only the modal tile context — CSS mask-image cannot vary a referenced SVG's stroke-width per instance, so a per-context split would have required maintaining two parallel icon sets for a mechanism that turned out not to matter for the actual complaint"
  - "Removed TestCategoryGlyphInkCoverageAcrossRenderContexts rather than lower its floor to match the reverted stroke-width — a test asserting 'stroke-width >= 2.5' immediately below a deliberate revert to 2 would be actively misleading, not just loose; 01-REVIEW.md's WR-05 had already found its per-context assertions added little independent value beyond a single uniform check anyway"
  - "Chose a Unix timestamp captured once at process start (not a content hash, not a build-time value) for AssetVersion — simplest mechanism that guarantees monotonic uniqueness per restart, at the cost of not being deterministic across identical rebuilds; acceptable for a project with no CDN and no need for long-lived immutable caching yet"

patterns-established:
  - "Cache-busting: any future local static asset must be linked with ?v={{.AssetVersion}}, not a bare path, or it silently re-introduces the exact caching bug this plan closed"

requirements-completed: [FOUND-02, FOUND-03]

coverage:
  - id: D1
    description: "Modal category grid glyph color raised from --color-text-muted to --color-text (with a selected-state counter-rule), confirmed by the human tester in dark mode by moving on to a different topic without repeating the complaint, and later explicitly confirmed ('looks good') after switching the OS to light mode"
    requirement: "FOUND-02, FOUND-03"
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./... (all pass); curl against the live server's exact served CSS URL, confirmed byte-identical to source"
        status: pass
    human_judgment: true
    rationale: "The literal defect (a human cannot identify a category glyph at a glance) is a perceptual claim no automated gate can certify — confirmed live, in-conversation, by the actual person who reported it, in both themes, which this project treats as satisfying the outstanding human-check item (D4 in 01-14-SUMMARY.md, which itself subsumed 01-13-SUMMARY.md's D3)."
  - id: D2
    description: "01-14's stroke-width change (2 -> 3) reverted to 2 across all nine icons; TestCategoryGlyphInkCoverageAcrossRenderContexts removed with it; TestBadgeGlyphContrastAcrossAgeStagesAndThemes (Mechanism 2, unrelated) confirmed still passing untouched"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestBadgeGlyphContrastAcrossAgeStagesAndThemes"
        status: pass
      - kind: unit
        ref: "web/css_contract_test.go#TestCategoryGlyphMaskRulesCoverEveryCategory"
        status: pass
      - kind: unit
        ref: "web/js_contract_test.go#TestIconGlyphsAreClassDriven"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "Icon-to-badge glyph sizing reduced from 55% to 48% (main.css .icon-badge .icon-glyph and feed.css .report-row .icon-badge .icon-glyph, kept in sync) on direct human request after the color fix made the icons visually prominent enough to have a size opinion on for the first time"
    requirement: "FOUND-02"
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && go test ./... (all pass, no test hardcodes 55%); curl against the live server's served CSS"
        status: pass
    human_judgment: true
    rationale: "'Feels big' is a subjective sizing preference, not a measurable defect — 48% was chosen as a moderate first reduction and confirmed acceptable by the human tester in the same pass as the color and theme checks ('looks good')."
  - id: D4
    description: "A server restart during live UAT can no longer be masked by a stale running process or by browser HTTP caching — AssetVersion (process-start Unix timestamp) is appended to every local static asset URL"
    requirement: "FOUND-02"
    verification:
      - kind: other
        ref: "curl http://localhost:8080/ shows versioned URLs; curl against the exact versioned URL returns current content; go build/vet/test all pass"
        status: pass
    human_judgment: false
duration: ~90min (live, conversational, interleaved with human testing — not comparable to a single-pass executor duration)
completed: 2026-09-09
status: complete
---

# Phase 01 Plan 15: Live UAT Correction — Modal Glyph Color, Stroke Revert, Cache-Busting Summary

**Found and fixed the real cause of the modal category grid's reported illegibility (glyph color, not stroke geometry — 01-14's fix), reverted 01-14's stroke-width side effect on the map/feed badges, fixed two independent server/browser caching bugs that were blocking the human tester from ever seeing a fix land, and reduced badge glyph sizing on direct request — all confirmed live, by the human tester, in both dark and light mode.**

## Why this plan exists

01-13 fixed category icons rendering solid-black in dark mode (an `<img>`-isolation bug). The
retest found a second, distinct defect: icons rendered but were too low-visual-weight to
identify. 01-14 diagnosed and fixed this as two mechanisms — stroke-only icon geometry
(Mechanism 1, fixed via thicker stroke) and badge age-stage contrast (Mechanism 2, fixed via an
age-aware foreground) — both gated by computed Go tests, both passing.

The human tester then reported the modal grid was **still** unreadable. Two more rounds of "did
you actually restart the server" and "is your browser caching it" followed before the real
picture emerged: two compounding infrastructure problems (five stale/orphaned server processes;
an unversioned static-asset URL under a 1-hour `Cache-Control`) meant the tester had, for a
while, genuinely never seen any of the day's fixes at all. Once that was resolved and the tester
really was looking at 01-14's shipped code, the modal grid was *still* reported illegible — which
meant Mechanism 1's own premise (stroke-thinness was the dominant legibility factor) was wrong,
or at least insufficient on its own. The actual fix was the tile's glyph *color*
(`--color-text-muted`, never touched by 01-14), not its stroke width.

## What shipped

1. **`.category-tile .icon-glyph { color: var(--color-text) }`** (+ a `.category-tile--selected
   .icon-glyph { color: var(--color-bg) }` counter-rule so the selected state doesn't invert into
   an invisible same-tone pairing). Dark-mode contrast against `--color-surface` went from ~7:1
   to ~17.7:1. This is the fix that actually resolved the original complaint.
2. **Stroke-width reverted 3 -> 2** on all nine `web/static/icons/*.svg` files. 01-14's stroke
   thickening was applied uniformly (mask-image cannot vary stroke-width per render context from
   one shared SVG), so it also visibly thickened map-pin and feed-row badge icons — a part of the
   UI the tester had explicitly said was already fine, back when this whole thread started.
   `TestCategoryGlyphInkCoverageAcrossRenderContexts` (which asserted a >= 2.5 floor) was removed
   rather than weakened, since keeping it would assert something now deliberately false.
3. **`AssetVersion` cache-busting.** `cmd/server/main.go` now computes
   `strconv.FormatInt(time.Now().Unix(), 10)` once at startup and passes it through
   `handlers.PageConfig` into `index.html.tmpl`, which appends it as `?v=...` to every local
   `<link>`/`<script>` tag (not the third-party CDN tags, which are already SRI-pinned).
4. **Badge glyph sizing 55% -> 48%** (`main.css` and `feed.css`, kept in sync as the existing
   code comment already required), on direct request after the color fix made the previously
   near-invisible icons look "big" for the first time.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` all green after every change in this plan.
- Every CSS change was verified against the **live server's served bytes** via `curl` with a
  cache-busting query of its own, not just against source on disk — this plan's whole premise is
  that source-level correctness had already been achieved twice (01-13, 01-14) and still wasn't
  visible to the person who needed to see it.
- Final confirmation: the human tester reported "looks good" after switching the OS to light
  mode and re-checking, following an explicit dark-mode pass earlier in the same session. This
  satisfies 01-14-SUMMARY.md's outstanding coverage item D4 (itself a supersede of 01-13's D3) —
  the real-browser, both-themes, at-a-glance identifiability check neither prior plan's automated
  gates could perform.

## Deviation from standard process

No `gsd-executor` or `gsd-debugger` subagent was spawned for this plan. Every fix here was
authored and verified directly by the orchestrating session, interleaved in real time with the
human tester's own screenshots and reports, because the defect being chased was specifically
"what does a real person, on a real device, actually perceive right now" — a question a
worktree-isolated agent with no browser cannot answer, and a question that had already survived
two correctly-executed automated rounds unresolved. See `01-15-PLAN.md`'s `<process_deviation>`
for the full reasoning.

## Self-Check: PASSED
