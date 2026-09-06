---
phase: 01-foundation-report-map
plan: 08
subsystem: ui
tags: [css, accessibility, modal, range-input, reduced-motion]

# Dependency graph
requires:
  - phase: 01-foundation-report-map (plans 01-04, 01-05)
    provides: modal shell markup (#report-modal), the severity slider markup, and the sev-*/--severity-current design-token contract this plan restyles
provides:
  - guarded .modal-backdrop:not([hidden]) selector so the hidden attribute is authoritative again
  - TestModalBackdropHiddenGuard durable regression test in web/css_contract_test.go
  - thin-track/value-aligned-fill severity slider restyle with a preserved 44px thumb
  - a working prefers-reduced-motion override for the slider (previously dead code in every browser)
affects: [01-foundation-report-map (remaining UAT re-run)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CSS regression test reading embedded assets via web.StaticFS + fs.WalkDir, stripping comments then splitting on the last '{' before a '}' to isolate selector heads from declaration bodies"
    - "Range-input track/thumb split: geometry moved off the host onto ::-webkit-slider-runnable-track/::-moz-range-track pseudo-elements, keeping the thumb at the accessibility floor independently of track thickness"
    - "Per-vendor-prefix pseudo-element rules, never grouped in one selector list (an unrecognised pseudo-element in a selector list invalidates the whole rule in both engines)"

key-files:
  created:
    - web/css_contract_test.go
  modified:
    - web/static/css/main.css
    - web/static/css/modal.css

key-decisions:
  - "Guarded .modal-backdrop with :not([hidden]) rather than touching modal.js, matching the existing pattern at modal.css:139/222 and feed.css — keeps the fix to CSS only"
  - "Thinned the track via pseudo-elements instead of shrinking the thumb, preserving the 44px WCAG 2.5.5 floor per 01-UI-SPEC.md line 66"
  - "Used .severity-slider:active::-vendor-thumb (host-state-before-pseudo-element form) instead of ::-vendor-thumb:active for the optional active-state shadow, since a pseudo-class appended directly after a UA shadow pseudo-element is not reliably valid across engines"

patterns-established:
  - "CSS contract tests live in package web alongside embed.go so they can read StaticFS directly instead of walking relative paths from an internal/ package"

requirements-completed: [FOUND-02, FOUND-03]

coverage:
  - id: D1
    description: "Modal backdrop's display rule is guarded by :not([hidden]) so the map/list are visible on a fresh page load and the modal only renders when explicitly opened"
    requirement: "FOUND-03"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestModalBackdropHiddenGuard"
        status: pass
      - kind: unit
        ref: "internal/api/handlers -run TestPageShellServesDOMContract"
        status: pass
    human_judgment: true
    rationale: "Automated tests confirm the CSS rule and DOM contract, but the actual on-load visual behavior (map/list visible, no scrim, FAB/Cancel/Escape/backdrop-click still working) requires a browser check per the plan's own <verification> section, which defers this to /gsd-verify-work 01"
  - id: D2
    description: "Severity slider renders as a thin rounded track with a distinct unfilled trough and a thumb-aligned severity-coloured fill, 44px thumb preserved, reduced-motion override actually valid CSS across engines"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "go test ./web/... && go test ./internal/api/handlers/... (full suite, plus all plan-specified grep/awk gates)"
        status: pass
    human_judgment: true
    rationale: "Visual alignment of the fill with the thumb at all three values, dark-mode rendering, screen-reader announcement, and prefers-reduced-motion behavior are explicitly called out in the plan's human-check and deferred to the formal Test 3 re-run via /gsd-verify-work 01, not closed by this plan"

duration: 22min
completed: 2026-09-06
status: complete
---

# Phase 01 Plan 08: Modal Backdrop Guard + Severity Slider Restyle Summary

**Guarded `.modal-backdrop`'s display rule with `:not([hidden])` (closing the UAT Test 1 blocker that hid the map/list on load) and restyled the severity slider into a thin trough-and-fill track with a value-aligned fill, keeping the 44px thumb and fixing a previously dead-code `prefers-reduced-motion` override.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-06T17:02:38+05:30 (worktree base commit)
- **Completed:** 2026-09-06T17:24:05+05:30
- **Tasks:** 2
- **Files modified:** 2 (`web/static/css/main.css`, `web/static/css/modal.css`)
- **Files created:** 1 (`web/css_contract_test.go`)

## Accomplishments

- Fixed the UAT Test 1 blocker: `.modal-backdrop` now only applies its fixed/scrim/flex declarations through a `:not([hidden])`-guarded selector, so the native `hidden` attribute `modal.js` toggles is authoritative again and the map/report list are visible on a fresh page load.
- Added `TestModalBackdropHiddenGuard`, a durable Go regression test (in the `web` package, reading the embedded `StaticFS` directly) that walks every shipped CSS file and fails the build if any rule ever sets `display` on the modal backdrop selector without the guard. Confirmed the test actually bites by temporarily removing the guard and watching it fail, then restoring it.
- Restyled the severity slider: geometry moved off the host onto `::-webkit-slider-runnable-track`/`::-moz-range-track` pseudo-elements (an unfilled `var(--color-border)` trough plus a severity-coloured fill sized via a new `--severity-fill-ratio` custom property), while the 44px thumb (WCAG 2.5.5 floor) is unchanged in size, with a refined border/shadow.
- Fixed a latent accessibility bug found while implementing the restyle: the `prefers-reduced-motion` block in `modal.css` previously grouped a `-webkit-` and a `-moz-` pseudo-element in one selector list, which is invalid CSS as a whole — both Chrome and Firefox silently dropped the entire rule, including the standard `#severity-readout` selector caught in the same list. Split into five separate rules, each with its own body, so the override now actually removes every transition in both engines.

## Task Commits

Each task was committed atomically:

1. **Task 1: Guard the modal backdrop's display rule and lock it with a regression test** - `67c3e8f` (fix)
2. **Task 2: Restyle the severity slider — thin track, refined thumb, valid reduced-motion override** - `ab45072` (feat)

**Plan metadata:** committed alongside this SUMMARY (worktree mode — orchestrator finalizes STATE.md/ROADMAP.md after merge)

## Files Created/Modified

- `web/css_contract_test.go` - New `TestModalBackdropHiddenGuard` regression test guarding the backdrop-hidden-guard defect class
- `web/static/css/main.css` - Guarded `.modal-backdrop` selector; restyled `.severity-slider` host/track/thumb rules (thin track pseudo-elements, value-aligned fill, preserved 44px thumb, added active-state shadow)
- `web/static/css/modal.css` - Added `--severity-fill-ratio` per `sev-*` class on `.severity-control`; moved transitions onto per-vendor-prefix track/thumb pseudo-elements; rewrote the `prefers-reduced-motion` block as five separate valid rules

## Decisions Made

- Kept the fix CSS-only per the plan's hard constraints: `web/static/js/modal.js` and `web/templates/index.html.tmpl` are byte-identical to before this plan (verified via `git diff --quiet HEAD`), and the control remains a native `<input type="range">` with no ARIA slider role or custom keydown handling added.
- Used `.severity-slider:active::-webkit-slider-thumb` / `.severity-slider:active::-moz-range-thumb` (host `:active` state before the shadow pseudo-element) for the optional tactile-feedback shadow, rather than `::-vendor-thumb:active`, since appending a pseudo-class directly after a UA shadow pseudo-element is not a reliably valid selector form across engines — this keeps the same one-rule-per-vendor-prefix structure the plan requires while avoiding a form more likely to be silently dropped.
- Kept the two section comment lead-ins (`Severity slider (D-05)` and `Modal shells (D-02, D-03)`) verbatim as required, since the plan's own verify gate (an `awk` range extraction) depends on them as brackets.

## Deviations from Plan

None — plan executed as written. The `prefers-reduced-motion` dead-code fix was already specified by the plan itself (Task 2, constraint 10), not a Rule-1 discovery made independently during execution. The only implementation choice not spelled out verbatim by the plan was the `:active` selector form (see Decisions Made above), which stays within Task 2's "optionally add an `:active` treatment... per-pseudo-element" instruction.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both UAT gaps this plan targeted (Gap 1 blocker, Gap 2 cosmetic) are closed at the code/test level: `go build`, `go vet`, and the full `go test ./...` suite all pass, and every automated verify-gate specified in the plan (grep/awk checks for guard presence, track pseudo-elements, fill ratio, reduced-motion rule count, no-raw-hex, upstream-contract preservation) passes.
- **Not yet closed by this plan, by the plan's own design:** UAT Test 3's full accessibility checklist (screen-reader announcement, keyboard Home/End, dark-mode rendering, reduced-motion visual behavior) has never been formally run — only a pre-emptive visual note existed in `01-UAT.md`. Tests 2 through 10 (excluding 8) remain `[pending]`, blocked behind Gap 1 until now. Re-run `/gsd-verify-work 01` from Test 2 onward, running Test 3 in full against the restyled control.
- **One item worth explicit attention in that re-run:** the WebKit thumb's `margin-top: calc((var(--space-sm) - var(--touch-target-min)) / 2)` centering assumes WebKit vertically centers the 8px runnable track inside the input's 44px content box. If that assumption doesn't hold in a given WebKit build, the thumb would render offset (partially above the track) rather than centered on it. This is a plausible visual failure mode that only a live browser check can catch — confirm the thumb is vertically centered on the track in Chrome, not riding above or below it, during the Test 3 re-run.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*
