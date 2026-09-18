---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-10"
subsystem: ui
tags: [html-template, css, vanilla-js, localstorage, accessibility, theme, navigation]

# Dependency graph
requires:
  - phase: 02-07
    provides: The shared account header partial and the Activity/profile page this plan extends
provides:
  - "A back-to-map link on the Activity page, the first thing in its <main>, closing UAT gap 4"
  - "A three-state (System/Light/Dark) theme toggle in the account menu, applied before first paint on every full page, closing UAT gap 6"
  - "color-scheme declared on all four theme sources in main.css, so native form controls and scrollbars follow the chosen theme too"
affects: [ui-review, 02-UAT-closure]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Theme applied at classic (non-deferred) head-script evaluation time, before DOMContentLoaded, to avoid a flash of the wrong palette"
    - "localStorage for a persisted preference (theme) vs. a URL parameter for shareable view state (02-09's disputed filter) — the same rule stated from both sides"
    - "Fixed-list validation before reflecting a stored value into a DOM attribute (T-01-17), mirroring app.js's iconClass discipline"

key-files:
  created:
    - web/static/js/theme.js
    - web/profile_nav_contract_test.go
    - web/theme_contract_test.go
  modified:
    - web/templates/profile.html.tmpl
    - web/templates/account_header.html.tmpl
    - web/templates/index.html.tmpl
    - web/templates/login_gate.html.tmpl
    - web/templates/verify_outcome.html.tmpl
    - web/static/css/auth.css
    - web/static/css/main.css

key-decisions:
  - "Three theme states (System -> Light -> Dark -> System), not two, so a reader can always route back to following the OS rather than being permanently stranded on an explicit choice."
  - "Theme preference persists via localStorage (survives across visits, never shareable); the disputed-filter view state (plan 02-09) persists via a URL parameter (shareable, does not survive across visits) — deliberately opposite calls for the opposite reason, stated here so they read as one consistent rule."
  - "theme.js is a classic, non-deferred script placed last in the head, after the stylesheet links, so it runs while <head> is still parsing and applies the stored mode before first paint. Deferring it or moving it to the body reintroduces the flash of the wrong palette the feature exists to remove."
  - "The login gate and verify-outcome pages load the theme module but render no control — they have no account header — because they still load main.css and therefore have a palette; a reader who chose Light and then logs out must not land on a dark screen."

requirements-completed: [IDENT-03, UX-01]

coverage:
  - id: D1
    description: "Activity page carries a 'Back to map' link as the first content element in <main>, meeting the 44px touch floor"
    requirement: "IDENT-03"
    verification:
      - kind: unit
        ref: "web/profile_nav_contract_test.go#TestActivityPageLinksBackToTheMap"
        status: pass
    human_judgment: false
  - id: D2
    description: "theme.js applies a validated, persisted System/Light/Dark mode before first paint, with no app-shell dependency and no markup-parsing sink"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleHasNoAppShellDependency"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleUsesNoMarkupParsingSink"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleValidatesStoredModeBeforeReflectingIt"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeModuleGuardsStorageAccess"
        status: pass
    human_judgment: false
  - id: D3
    description: "Theme control renders in the account menu between Activity and Log out, reusing the existing menu-item class/role/focus-trap"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestAccountHeaderRendersThemeControl"
        status: pass
      - kind: unit
        ref: "web/theme_contract_test.go#TestAccountMenuHiddenGuard (pre-existing, re-run to confirm no regression)"
        status: pass
    human_judgment: false
  - id: D4
    description: "All four full-page templates load theme.js in the head, asset-versioned, with neither defer nor async; the set is derived from the embedded filesystem so a future fifth page fails the build without it"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage"
        status: pass
    human_judgment: false
  - id: D5
    description: "main.css declares color-scheme on all four theme sources with no token value changed"
    requirement: "UX-01"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeOverrideBlocksExistForBothModes"
        status: pass
      - kind: unit
        ref: "web/css_contract_test.go#TestBadgeGlyphContrastAcrossAgeStagesAndThemes (18-pairing WCAG matrix, re-run to confirm no token drift)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Real-browser verification: back-link click-through, theme cycling and repaint, native-control theming, no-flash-on-reload, post-logout light-mode persistence, and the first light-mode walkthrough of all of Phase 2's UI"
    verification: []
    human_judgment: true
    rationale: "Requires a real browser against a running server per workflow.human_verify_mode: end-of-phase (plan's own <human-check> steps 1-7); static/unit tests cannot prove rendered tap-target size, absence of a paint flash, or visual legibility."

# Metrics
duration: ~55min
completed: 2026-09-18
status: complete
---

# Phase 2 Plan 10: Activity back-link and in-app theme toggle Summary

**A "Back to map" link on the Activity page and a System/Light/Dark toggle in the account menu, applied before first paint via a non-deferred head script and gated on every full page by a filesystem-derived test.**

## Performance

- **Duration:** ~55 min
- **Tasks:** 3
- **Files modified:** 10 (7 modified, 3 created)

## Accomplishments

- Closed UAT gap 4: the Activity page previously had no way back to the map/feed except the browser's own Back button. It now carries a "Back to map" link as the first thing inside its `<main>`, styled from existing `auth.css`/`main.css` tokens, meeting the 44px touch floor, in the page body rather than the shared header (DEC-S: the header carries no left-slot content of any kind).
- Closed UAT gap 6: added `theme.js`, a self-contained module with the identical no-app-shell-dependency and no-markup-parsing-sink contract `account-menu.js` already carries. It reads and validates a stored mode against a fixed three-element list (T-01-17), applies it via exactly one `setAttribute`/`removeAttribute` pair on the root `data-theme` attribute, and persists it via `localStorage`, all guarded in `try`/`catch` per `votes.js`'s privacy-mode precedent.
- Added the Theme control to the shared account header between Activity and Log out, reusing the existing `.account-menu__item` class and `menuitem` role — no change to `account-menu.js` or `auth.css`.
- Loaded `theme.js` in the `<head>` of all four full-page templates (`index`, `profile`, `login_gate`, `verify_outcome`) as a classic, non-deferred, asset-versioned script — including the two pages that render no header and therefore no control (see Key Decisions).
- Added `color-scheme` to all four theme sources in `main.css` (base `:root`, the dark media block, and both `data-theme` override blocks) so native form controls and scrollbars follow the chosen theme, not just the app's own surfaces. Confirmed no token value changed by re-running the existing 18-pairing WCAG contrast matrix.
- Added two new test files (`web/profile_nav_contract_test.go`, `web/theme_contract_test.go`, 7 tests) that derive the full-page template set from the embedded filesystem rather than hardcoding it, so a future fifth full page fails the build unless it loads the theme module too.

## Task Commits

Each task was committed atomically:

1. **Task 1: Give the Activity page a way back to the map** — `3e5257c` (feat)
2. **Task 2: The theme module and its control** — `1fe424b` (feat)
3. **Task 3: Load the theme module on every full page** (TDD RED/GREEN pair):
   - `b3dee5b` (test) — added `theme_contract_test.go`'s 7 tests; 6 passed immediately against Task 2's already-shipped `theme.js`/header control, and `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` failed on all four templates (genuine RED — no script tag existed yet)
   - `bae7fcc` (feat) — added the script tag to all four full-page templates; all 7 tests now pass (GREEN)

_Note: Task 3's RED phase is partial by design — the plan structures Task 2 (module + control) and Task 3 (test file + template wiring) as separate tasks, so 6 of the 7 tests in Task 3's new file assert properties Task 2 already satisfied. Only the template-wiring assertion was genuinely red before Task 3's second commit. This is documented rather than glossed over per the TDD Gate Compliance note below._

## Files Created/Modified

- `web/static/js/theme.js` — the theme module: reads/validates/applies the stored mode before first paint, wires the toggle control on `DOMContentLoaded`
- `web/profile_nav_contract_test.go` — `TestActivityPageLinksBackToTheMap`
- `web/theme_contract_test.go` — 7 tests gating the theme module and its template wiring
- `web/templates/profile.html.tmpl` — back-link (Task 1) + theme script tag (Task 3)
- `web/templates/account_header.html.tmpl` — the Theme menu item (Task 2)
- `web/templates/index.html.tmpl` — theme script tag (Task 3)
- `web/templates/login_gate.html.tmpl` — theme script tag (Task 3)
- `web/templates/verify_outcome.html.tmpl` — theme script tag (Task 3)
- `web/static/css/auth.css` — `.profile-back-link` rule (Task 1)
- `web/static/css/main.css` — `color-scheme` on all four theme sources (Task 2)

## Decisions Made

- **The theme toggle's requirement traceability, flagged and resolved.** This plan's own text (written at plan-authoring time) states plainly that the theme toggle had no backing requirement ID in `.planning/REQUIREMENTS.md` at the time the plan was written. By the time this plan executed, that gap had already been closed: `REQUIREMENTS.md` shows `UX-01` ("A user can choose Light, Dark, or follow-the-OS appearance from inside the app") was added 2026-09-18, explicitly linked to this plan (`| UX-01 | Phase 2 | Pending (gap-closure plan 02-10) |`). This plan's frontmatter `requirements` field already listed `UX-01` alongside `IDENT-03`. Both are now marked complete via `gsd-tools query requirements.mark-complete IDENT-03 UX-01` — `IDENT-03` was already complete (unrelated prior work), `UX-01` was newly marked. No further developer action needed on this item.
- **Phase 1's prediction held.** `main.css`'s header comment (written in Phase 1) claimed the two `data-theme` override blocks meant "a future theme toggle needs no restructuring." That held exactly: no token moved, no override block was edited, and the only change to that file across this plan was one non-token `color-scheme` declaration added to each of its four theme sources. Worth recording as a case where build-it-for-later paid off — most such comments don't get to be checked against a real future feature this cleanly.
- **The `color-scheme` addition, and why all four sources got it.** A forced theme that leaves native checkboxes and scrollbars on the OS palette is visibly half-applied. All four theme sources (the base `:root`, the `prefers-color-scheme: dark` media block, and both `data-theme` override blocks) were edited together for the exact reason `extractThemeTokens`'s own doc comment (`web/css_contract_test.go`) gives: editing one member of a pair and not the other ships a fix that works for OS-preference users and not explicit-toggle users, or vice versa.
- **The storage decision, paired with its sibling.** Theme is a *preference*: `localStorage`, persists across visits, never shareable. The disputed-report filter (plan `02-09`) is *view state*: a URL parameter, shareable, does not persist across visits. Stated here so the two decisions read as one consistent rule rather than an inconsistency between sibling plans.
- **The three-state choice.** System → Light → Dark → System, not a two-state toggle, so a reader always has a route back to following the OS rather than being permanently stranded on an explicit choice. The default (System) state stores nothing at all — a reader who never touches the control leaves no trace in `localStorage`.
- **The two pages that get the module but no control, and why.** The login gate and verify-outcome pages load `theme.js` in their head (so the stored preference still applies and paints correctly) but render no account header and therefore no Theme control — a reader who chose Light and then logs out must not land on a dark login screen. The module's own header-lookup guard (mirroring `account-menu.js`'s own guard) makes it inert there: it applies the stored theme and finds no control to wire, without throwing.
- **A grep-based acceptance criterion in the plan text needed a corrected check, documented rather than silently worked around.** The plan states `grep -c 'color-scheme:' web/static/css/main.css` should return 4. It actually returns 7, because the file already contained three pre-existing occurrences of the substring `color-scheme:` inside `prefers-color-scheme: dark` (one live `@media` rule plus two explanatory comments referencing it) — text this plan did not touch. Verified the plan's real intent with an anchored check instead: `grep -cE '^\s*color-scheme:' web/static/css/main.css` returns exactly 4, matching the four new property declarations this plan added, one per theme source. `git diff --stat web/static/css/main.css` confirms additions-only (16 insertions, 0 deletions), and the 18-pairing WCAG matrix (`TestBadgeGlyphContrastAcrossAgeStagesAndThemes`) still passes unchanged, confirming no token value was disturbed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Documentation correction, not a code defect] Corrected the literal `color-scheme:` grep acceptance criterion**
- **Found during:** Task 2 verification
- **Issue:** The plan's acceptance criterion counts raw substring occurrences of `color-scheme:`, which coincidentally also matches `prefers-color-scheme:` — text already present in the file before this plan touched it (one `@media` rule, two comments).
- **Fix:** No code change needed. Verified the real intent (4 new property declarations, one per theme source, no token changed) with an anchored regex and the existing WCAG contrast matrix instead of the plan's literal grep.
- **Files affected:** None (verification-only correction)
- **Verification:** `grep -cE '^\s*color-scheme:' web/static/css/main.css` → 4; `git diff --stat web/static/css/main.css` → additions-only; `TestBadgeGlyphContrastAcrossAgeStagesAndThemes` → PASS
- **Committed in:** N/A (no code change; documented here for the record)

---

**Total deviations:** 0 code changes; 1 documentation/verification correction (a plan-authored acceptance-criterion imprecision, not an implementation bug).
**Impact on plan:** None on scope or correctness. The plan's real requirement (4 new declarations, no token drift) is fully met and independently confirmed by the existing WCAG regression test.

## Issues Encountered

None beyond the acceptance-criterion note above.

## TDD Gate Compliance

Task 3 is tagged `tdd="true"`. Gate sequence found in git log:
1. `test(02-10): gate every full page on loading the theme module` (`b3dee5b`) — RED. 6 of 7 tests passed immediately (they assert properties of `theme.js`/the header control, already shipped in Task 2's commit); `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` genuinely failed on all four templates.
2. `feat(02-10): load the theme module on every full page` (`bae7fcc`) — GREEN. All 7 tests pass.

No REFACTOR commit was needed (no cleanup beyond the two commits above). This partial-RED shape is a direct consequence of the plan's own task boundary (Task 2 builds the module and control; Task 3 builds both the test file and the remaining template wiring) — recorded here rather than silently treated as a full RED cycle.

## User Setup Required

None — no external service configuration required. No package-manager install occurred in this plan (confirmed by the plan's own threat model: `localStorage` and `DOMContentLoaded` are platform APIs, no new dependency).

## Next Phase Readiness

- Both UAT gaps this plan targeted (Activity-page dead end, no theme control) are closed and independently verified via `go build`, `go vet`, `go test ./... -short`, and `go test ./... -p 1` against real Postgres — all green.
- `git status --porcelain` after all three tasks lists exactly the 10 paths declared in this plan's `files_modified` frontmatter field — no collision with sibling gap-closure plans `02-08`/`02-09`, which share zero files with this plan by design.
- **Not yet closed:** the plan's own `<human-check>` block (7 steps) requires a real browser against a running server, per `workflow.human_verify_mode: end-of-phase`. This includes step 6 — the first light-mode walkthrough of all of Phase 2's UI, since every screenshot from the original UAT session was dark mode. **This walkthrough has not been performed as part of this plan's execution** — it is explicitly deferred to the end-of-phase human verification pass, consistent with the plan's own instruction that "we looked and it was fine" and "we never looked" are different states, and only one of them is evidence. Run `/gsd-verify-work 2` (or the phase's designated end-of-phase UAT step) to perform the full `<human-check>` block, including this walkthrough, on a machine whose OS appearance is set to Light.

## Self-Check: PASSED

All 10 created/modified files under `files_modified` confirmed present on disk. All 5 commits
(`3e5257c`, `1fe424b`, `b3dee5b`, `bae7fcc`, `738c850`) confirmed present in `git log`.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-18*
