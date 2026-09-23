---
phase: quick-260923-qwi
plan: 01
subsystem: ui
tags: [css, javascript, html-template, localstorage, matchmedia, accessibility]

requires:
  - phase: 01.1-identity-login-mandatory-email-verification
    provides: the shared account header partial (account_header.html.tmpl) and the .account-menu__item / .auth-icon-- CSS conventions this task builds on
provides:
  - A two-state (light/dark) icon-only theme toggle replacing the three-way System/Light/Dark text control
  - Two new committed Lucide icon assets (sun.svg, moon.svg) under web/static/icons
  - A reusable geometry-lock CSS-contract-test pattern (.theme-toggle height == .account-trigger height) alongside the existing .profile-back-link precedent
affects: [future account-menu or auth-surface UI work, any future icon addition to web/static/icons]

tech-stack:
  added: []
  patterns:
    - "Icon-only interactive control: single glyph span with aria-label/title carrying the whole accessible name, keyed by the TARGET state so the label always describes the next action, not the current state"
    - "One-time OS-preference read persisted immediately at module-evaluation time, so a later load never re-consults window.matchMedia"

key-files:
  created:
    - web/static/icons/sun.svg
    - web/static/icons/moon.svg
  modified:
    - web/theme_contract_test.go
    - web/css_contract_test.go
    - web/static/css/auth.css
    - web/static/js/theme.js
    - web/templates/account_header.html.tmpl

key-decisions:
  - "The control stays inside #account-menu (D-A): styled into a circle via a sibling modifier class, not moved beside .account-trigger in the header, per explicit user confirmation before execution."
  - "Icon shows current state, accessible name describes the action (D-B): sun glyph when currently light, but aria-label/title read the switch-to-dark sentence, and vice versa."
  - "The one-time OS read is persisted immediately during module evaluation (D-C), a deliberate behavior change from the old code, which left no storage trace for a reader who never touched the control."
  - "No negative-count test gate replaces the removed removeAttribute assertion (D-D): the assertion is dropped outright since no code path clears the attribute any more, rather than flipped to an unhelpful zero-count gate."

requirements-completed:
  - "Phase 2 UAT round 3 follow-up: replace the wordy Theme text label with a two-state icon-only toggle"

coverage:
  - id: D1
    description: "Two new sun/moon icon files exist, are embedded, and their .auth-icon--sun/.auth-icon--moon CSS mask rules resolve to those exact files; the .theme-toggle control shares .account-trigger's height and is a true circle (border-radius 50%)"
    requirement: "Phase 2 UAT round 3 follow-up: replace the wordy Theme text label with a two-state icon-only toggle"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeToggleIconAssetsExist"
        status: pass
      - kind: unit
        ref: "web/css_contract_test.go#TestCategoryGlyphMaskRulesCoverEveryCategory"
        status: pass
    human_judgment: false
  - id: D2
    description: "theme.js is a validated two-element (light/dark) state machine: one setAttribute write path, matchMedia consulted only via a guarded read that runs once during module evaluation, storage read and write both try/catch guarded, no markup-parsing sink, no app-shell dependency"
    requirement: "Phase 2 UAT round 3 follow-up: replace the wordy Theme text label with a two-state icon-only toggle"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleValidatesStoredModeBeforeReflectingIt"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleGuardsStorageAccess"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleUsesNoMarkupParsingSink"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleHasNoAppShellDependency"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage"
        status: pass
    human_judgment: false
  - id: D3
    description: "The account menu control is icon-only markup (no visible text) carrying aria-label and title, cross-checked against theme.js and auth.css so a rename on any one side fails the build instead of shipping an empty circle"
    requirement: "Phase 2 UAT round 3 follow-up: replace the wordy Theme text label with a two-state icon-only toggle"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestAccountHeaderRendersThemeControl"
        status: pass
    human_judgment: false
  - id: D4
    description: "The glyphs read as a sun and a moon at a glance, the moon's single stroked crescent is not noticeably lighter than the sun's circle-and-rays under the CSS mask, one tap flips the app with no flash, contrast holds against the menu surface in both themes, and an unlabelled circle in a text-row menu looks deliberate rather than stranded"
    verification: []
    human_judgment: true
    rationale: "No headless Go test can render a mask-image glyph or judge visual weight/contrast/placement; this is exactly the plan's own human-check verify step (Task 3), and it needs a rebuilt binary and a real browser, not a static assertion."

duration: 8min
completed: 2026-09-23
status: complete
---

# Quick Task 260923-qwi: Icon-only two-state theme toggle Summary

**Replaced the three-way "Theme: Dark/Light/System" text control with a 44px circular icon-only toggle (sun/moon glyphs) driven by a rewritten two-state theme.js state machine.**

## Performance

- **Duration:** ~8 min (commit span 19:46 to 19:54 IST)
- **Tasks:** 3 completed (RED test rewrite, icon assets + CSS, theme.js + template rewrite)
- **Files modified:** 5 modified, 2 created

## Accomplishments

- The account menu's theme control is now an icon-only 44px circle: a sun glyph in light mode, a moon glyph in dark mode, with no text label at all.
- One tap flips the app straight between exactly two states. The System (follow-OS) mode is gone from the shipped control, the fixed mode list, and every code path.
- aria-label and title both describe the action the tap performs ("Switch to light mode" / "Switch to dark mode") and both update on every flip, keyed by the TARGET state per D-B.
- The operating system preference still seeds a first-ever visitor's starting mode exactly once, via a guarded `window.matchMedia` check, and that derived value is written to storage immediately during module evaluation (D-C), so no later load ever consults the operating system again.
- A corrupt or unrecognised stored value now falls back to the one-time derived value (never reflected into the root attribute), and both the storage read and write stay try/catch guarded.
- theme.js still contains no app-shell reference and no capitalised product name, so it keeps loading unchanged on the login gate and verify-outcome pages.

## Task Commits

Each task was committed atomically:

1. **Task 1: Rewrite the theme contract tests to the two-mode icon-only shape** - `1e68077` (test, RED)
2. **Task 2: Add the sun and moon icons, their mask rules, and the circular control rule** - `fae1059` (feat, GREEN for icon/CSS tests)
3. **Task 3: Rewrite theme.js as a two-state toggle and replace the header markup** - `d087466` (feat, GREEN for remaining tests)

**Plan metadata:** commit deferred to the orchestrator per this task's constraints (SUMMARY.md, STATE.md, PLAN.md not committed by this executor).

_TDD gate compliance: Task 1's commit is the RED gate (three named tests fail with the expected missing-matchMedia / missing-glyph-span-id / missing-icon-file messages, zero compile errors, every other test in the package still green). Tasks 2 and 3 together are the GREEN gate (the full web suite, then the full project suite, both pass with no database). No REFACTOR-gate commit was needed; nothing required cleanup after GREEN._

## Files Created/Modified

- `web/static/icons/sun.svg` - New Lucide-style stroke icon (circle + 8 rays), byte-for-byte matching arrow-left.svg's attribute shape
- `web/static/icons/moon.svg` - New Lucide-style stroke icon (single crescent path)
- `web/static/css/auth.css` - Two new `.auth-icon--sun` / `.auth-icon--moon` mask rules; new `.theme-toggle` circular control rule (placed after `.account-menu__item` for source-order precedence) plus hover, active and reduced-motion companions modelled on `.profile-back-link`
- `web/static/js/theme.js` - Rewritten from a three-mode (system/light/dark) state machine to a two-mode (light/dark) one: `readStoredMode` now signals `null` instead of a follow-the-OS default, a new `initialMode` function is the sole `matchMedia` consumer, `applyMode`/`persistMode` each collapse to one write path, and a new `render` function drives the glyph class and the target-keyed aria-label/title pair
- `web/templates/account_header.html.tmpl` - The theme control's text node and label span are gone; the button gains the `theme-toggle` class, `aria-label`/`title`, and a single glyph span (`id="theme-toggle-icon"`)
- `web/theme_contract_test.go` - `TestThemeModuleValidatesStoredModeBeforeReflectingIt` drops the removeAttribute count gate, adds a matchMedia presence gate; `TestAccountHeaderRendersThemeControl` swaps its label-span-id check for a glyph-span-id check and adds aria-label/title window assertions and a four-way (id/id/class/class) cross-file drift loop; new `TestThemeToggleIconAssetsExist` locks the two icon files, their mask rules, and the `.theme-toggle`/`.account-trigger` height geometry match
- `web/css_contract_test.go` - `nonCategoryIcons` map gains `static/icons/sun.svg` and `static/icons/moon.svg` entries, required so `TestCategoryGlyphMaskRulesCoverEveryCategory` does not fail on the two new non-category icon files

## Decisions Made

See `key-decisions` in frontmatter (D-A through D-D), all recorded in the plan and carried through execution unchanged. No new decisions were made during execution; the plan's decisions record was followed as written, including the pre-confirmed placement (D-A: control stays inside the account menu, not moved beside the account button).

## Deviations from Plan

None - plan executed exactly as written. All four `must_haves.truths`, all eight listed artifacts, and both `key_links` items (the `nonCategoryIcons` entries and the `.theme-toggle` / `.account-menu__item` source-order placement) are present in the final diff.

## Issues Encountered

None. The RED phase (Task 1) failed for exactly the three expected reasons (missing matchMedia reference, missing glyph-span id, missing icon files/CSS rules) with zero compile errors and no other test regressions; each subsequent task turned the expected subset of tests green without needing any additional fix-up commit.

## User Setup Required

None - no external service configuration required. This is a purely static-asset/client-script change: two SVG files, one stylesheet, one script, one template partial, two test files. No new dependency, no package install, no network call, no server-side change.

## What was verified computationally versus what still needs a live look

**Verified computationally (go build, go vet, `go test ./web/ -short`, `go test ./... -short`, all green, no `DATABASE_URL` set at any point):**
- Both icon files exist and are embedded in the compiled binary.
- The `.auth-icon--sun` / `.auth-icon--moon` CSS mask rules resolve to those exact committed files.
- The `.theme-toggle` control's declared height exactly matches `.account-trigger`'s declared height, and `.theme-toggle` declares `border-radius: 50%` (a true circle).
- theme.js, auth.css and the template all address the same ids (`theme-toggle`, `theme-toggle-icon`) and class names (`auth-icon--sun`, `auth-icon--moon`) — a rename on any one side would fail the build, not ship an empty circle.
- theme.js's mode list is a validated two-element array with exactly one `setAttribute('data-theme', ...)` write path and a `matchMedia` reference guarded by an existence check.
- Both the storage read and the storage write stay wrapped in `try`/`catch` (module still passes the "at least two `try {` blocks" gate).
- theme.js contains no markup-parsing sink (`innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`) and no reference to the app shell or the capitalised product name.
- Zero em dash or en dash characters in theme.js (manual grep) or in any embedded template (`TestUserVisibleCopyUsesPlainPunctuation`, whole-file scan).
- The whole existing test suite (`web` package plus every other package, short mode) stays green with no collateral damage.

**Genuinely NOT verified by any headless Go test, needing a real browser:**
- Whether the sun and moon glyphs read as a sun and a moon at a glance.
- Whether the moon's single stroked crescent is noticeably lighter in visual weight than the sun's circle-and-rays once both are rendered through the CSS mask.
- Whether one tap flips the whole app cleanly with no flash.
- Whether the circle's contrast is fine against the menu surface in BOTH themes (light and dark).
- Whether an unlabelled circle sitting in a menu of text rows looks deliberate or looks stranded. (If it looks stranded, the pre-scoped follow-up per D-A is moving it beside the account button in the header, deliberately left out of this task's scope.)

**The dev server needs a restart before any of this is visible.** The CSS, JS and templates are embedded into the Go binary via `go:embed` at build time, so the running dev server on port 8090 is still serving the old three-way text control's bytes until it is rebuilt and restarted.

## Next Phase Readiness

The code-verifiable half of this task is done and committed. What remains is a single human-check pass after a dev-server restart, per the plan's own Task 3 `<human-check>` step: open the account menu, confirm the glyph legibility, flip feel, both-theme contrast, and placement questions above. No further planning or code work is expected unless that human check surfaces a real problem, in which case the documented fallback is the D-A follow-up (moving the control beside the account button), which is explicitly out of scope here.

---
*Phase: quick-260923-qwi*
*Completed: 2026-09-23*

## Self-Check: PASSED

All 7 created/modified files confirmed present on disk (web/static/icons/sun.svg,
web/static/icons/moon.svg, web/static/css/auth.css, web/static/js/theme.js,
web/templates/account_header.html.tmpl, web/theme_contract_test.go,
web/css_contract_test.go). All 3 task commits confirmed present in git log
(1e68077, fae1059, d087466).
