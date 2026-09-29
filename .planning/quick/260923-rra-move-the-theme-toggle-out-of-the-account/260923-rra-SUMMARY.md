---
phase: quick-260923-rra
plan: 01
subsystem: ui
tags: [css, templates, theme-toggle, floating-control, contract-tests]

requires:
  - phase: quick-260923-qwi
    provides: the two-state sun/moon theme.js module, the theme-toggle and theme-toggle-icon ids, and the two icon assets it previously rendered inside the account menu
provides:
  - a standalone theme_toggle.html.tmpl partial holding the control's markup
  - the control as a fixed 44px floating circle in the bottom-right corner, occupying the lower of two stacked slots on the map page, with the report button raised into the slot above it
  - the same control at the ordinary floating offset on the Activity page
  - a token-derived calc on the report button's bottom, replacing its previous hardcoded single-token offset
  - two rewritten/added contract tests (TestThemeToggleRendersAsAFloatingControl, TestThemeToggleFloatsClearOfTheReportButton) that lock the new arrangement against regression
affects: [ui-polish, account-menu, map-page-layout, activity-page-layout]

tech-stack:
  added: []
  patterns:
    - "Shared template partial included by body-level {{template}} action on two independent full pages, registered automatically by html/template.ParseFS over templates/*.tmpl with no Go-side wiring"
    - "Computed CSS geometry lock in Go tests: parse :root design tokens into a table, resolve calc()/var() expressions through a no-magic-number gate (reject px literals and non-addition operators), then assert on the resulting integers rather than on raw declaration text"

key-files:
  created:
    - web/templates/theme_toggle.html.tmpl
  modified:
    - web/theme_contract_test.go
    - web/templates/account_header.html.tmpl
    - web/templates/index.html.tmpl
    - web/templates/profile.html.tmpl
    - web/static/css/auth.css
    - web/static/css/main.css

key-decisions:
  - "D-A (recorded in PLAN.md, not made here): the floating control takes the LOWER of two stacked slots and the report button moves up into the slot above it — a direct user choice between two named alternatives, not an inference"
  - "D-B: the new partial is named theme_toggle.html.tmpl, not theme_toggle_fab.html.tmpl, because the control keeps its own .theme-toggle class and visual treatment distinct from .fab"
  - "D-C: one CSS rule, one bottom value, both pages — no page-scoped override for the Activity page, since the control now sits at the ordinary floating offset there"
  - "D-D: theme.js is untouched; getElementById is document-scoped and indifferent to where the ids live"
  - "D-E: the removal from account_header.html.tmpl is a plain deletion, no placeholder or empty row left behind"

patterns-established:
  - "When extracting a control from a dropdown menu into a standalone floating element, the extracted rule must explicitly declare every property it used to free-ride on from its former parent rule (display, align-items, justify-content, cursor) — losing this silently centers nothing and ships a green build with a visibly broken layout"

requirements-completed:
  - "Phase 2 UAT round 3 follow-up: move the sun/moon theme toggle out of the account menu into an always-visible floating control stacked with the report button"

coverage:
  - id: D1
    description: "theme_toggle.html.tmpl exists as the single shared partial, included exactly once by index.html.tmpl and profile.html.tmpl, and zero times by login_gate.html.tmpl, check_inbox.html.tmpl and verify_outcome.html.tmpl"
    requirement: "Phase 2 UAT round 3 follow-up: move the sun/moon theme toggle out of the account menu into an always-visible floating control stacked with the report button"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeToggleRendersAsAFloatingControl"
        status: pass
    human_judgment: false
  - id: D2
    description: "the account menu no longer renders the control at all (critical negative assertion)"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeToggleRendersAsAFloatingControl"
        status: pass
    human_judgment: false
  - id: D3
    description: "the control's vertical band and the report button's raised vertical band cannot overlap, the control is provably the lower of the two, and the report button's offset is a token-derived calc rather than a magic number"
    verification:
      - kind: unit
        ref: "web/theme_contract_test.go#TestThemeToggleFloatsClearOfTheReportButton"
        status: pass
    human_judgment: false
  - id: D4
    description: "the report button, now 52px higher than the shipped build, still reads as the obvious primary action and remains thumb-reachable; the pair reads as one deliberate stack; the toggle's glyph is visually centered; nothing collides at 375px width (Leaflet zoom control, account button, mobile view-toggle); a toast does not look wrong against the raised button; the control sits behind the report modal backdrop; the Activity page's lone floating circle reads normally"
    verification: []
    human_judgment: true
    rationale: "These are visual/interaction judgments about a live rendered page (thumb reachability, whether two circles 'read as a pair', glyph centering at a glance, narrow-viewport collisions) that no headless Go test can evaluate. The plan's own human-check step calls these out explicitly as the one real open question in this change."

duration: 20min
completed: 2026-09-29
status: complete
---

# Quick Task 260923-rra: Move the theme toggle out of the account menu Summary

**Extracted the sun/moon theme toggle from the account-menu dropdown into its own always-visible fixed-position floating circle, stacked in the bottom-right corner directly below the report button, which itself moved up into the slot the toggle vacated — governed end to end by a computed CSS geometry lock rather than eyeballed pixel values.**

## Performance

- **Duration:** ~20 min (commit span 10:18:03 to 10:20:29 IST, 2026-09-29)
- **Tasks:** 3/3 completed
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments

- Rewrote `web/theme_contract_test.go`: replaced `TestAccountHeaderRendersThemeControl` with `TestThemeToggleRendersAsAFloatingControl` (existence, exactly-once counts, accessible-name attributes, the critical negative assertion that the account header no longer contains the control id, exactly-once/zero-times include counts across five pages, and the theme.js cross-file drift guard), and added `TestThemeToggleFloatsClearOfTheReportButton` (a computed geometry lock resolving `.fab` and `.theme-toggle` offsets from design tokens, checking non-overlap, gap tightness, the stacking-order decision, and the cascade-orphan guard). Committed this as a deliberately RED state (both new tests failing against the unmodified templates/CSS), per the plan.
- Created `web/templates/theme_toggle.html.tmpl` holding exactly the control's button element, with the `menuitem` role and `account-menu__item` class dropped and everything else (id, aria-label, title, glyph span) byte-identical to what shipped inside the account menu.
- Deleted the control from `web/templates/account_header.html.tmpl` as a plain deletion — no placeholder, no empty row. The menu is back to its pre-qwi shape: email, Activity, Log out.
- Included the new partial at body level in `web/templates/index.html.tmpl` and `web/templates/profile.html.tmpl`, immediately after each page's existing account-header include, with byte-identical include lines on both pages.
- Rewrote the `.theme-toggle` rule in `web/static/css/auth.css` into a self-sufficient fixed-position floating circle: `position: fixed`, `right`/`z-index` shared with `.fab`, `bottom: var(--space-lg)` (the report button's former offset, taken over unchanged, no calc), and newly declared `display`, `align-items`, `justify-content`, `cursor` and full-contrast `color` so it no longer depends on the account-menu-item rule it used to free-ride on. Rewrote the comment block above it to describe the final arrangement and the D-A decision instead of the now-dead source-order argument.
- Raised `.fab`'s `bottom` in `web/static/css/main.css` by exactly one declaration: `calc(var(--space-lg) + var(--touch-target-min) + var(--space-sm))`, moving the report button into the slot above the toggle with an 8px gap. Nothing else in that rule or file changed. Added a short companion comment above `.fab` pointing at `.theme-toggle`'s rationale and the geometry test.
- Whole suite green with no database at any point: `go build ./...`, `go vet ./...`, `go test ./web/ -short`, and `go test ./... -short` all pass.

## Computationally verified (headless Go tests, no browser)

- The markup lives in exactly one file (`theme_toggle.html.tmpl`) and is included exactly once by each of `index.html.tmpl` and `profile.html.tmpl`, and zero times by `login_gate.html.tmpl`, `check_inbox.html.tmpl` and `verify_outcome.html.tmpl`.
- The control id (`theme-toggle`) is entirely absent from `account_header.html.tmpl` — confirmed both by the test's critical negative assertion and by a direct grep of the shipped file.
- The report button's raised offset is a token-derived `calc()` referencing `--space-lg`, `--touch-target-min` and `--space-sm` by name, with no raw pixel literal; the control's own offset is a single plain token reference with no `calc` at all.
- The two controls' vertical bands provably cannot intersect: resolved bands are 24px-68px (control, lower) and 76px-120px (report button, upper), with an 8px gap — logged directly by the test (`t.Logf`) on every run.
- The control is provably the lower of the two (a separate assertion from the band check, so a future well-meaning "fix" that swaps them back cannot ship green).
- Both stylesheets were provably edited together: a half-done edit to only one file would leave both bands claiming the identical offset and fully intersecting, which the band check would catch as a red build.
- The control's `z-index` (1000) is at least the report button's (1000).
- The `.theme-toggle` rule declares its own `display`, `align-items`, `justify-content` and `cursor` rather than relying on the account-menu-item rule it no longer lives inside (the cascade-orphan guard).
- The circle geometry still matches `.account-trigger` (unchanged by this task — `TestThemeToggleIconAssetsExist` was not touched and still passes).
- `web/static/css/main.css` changed by exactly one declaration (`.fab`'s `bottom`) plus five comment lines above it — confirmed by a zero-context `git diff` inspection, not just by claim.
- No em dash or en dash appears in any added line in either stylesheet (checked via a zero-context diff, added lines only, since both files carry pre-existing em dashes elsewhere that are out of this task's scope). `TestUserVisibleCopyUsesPlainPunctuation` also passes over the new template partial.
- The whole suite (`go build`, `go vet`, `go test ./... -short`) is green; `DATABASE_URL` was unset throughout and `pinalert_test` was never touched.

Also recorded explicitly per the plan's instruction: the stack's bottom-most pixel is unchanged from the previously shipped build. Whichever control holds the lower slot sits at the same `--space-lg` (24px) offset the report button used to occupy, so clearance over Leaflet's default bottom-right attribution strip needs no fresh verification — this was true before this task and remains true after it, by construction of the token reuse.

## Not verified by any headless test — needs a live look

In priority order, per the plan's explicit instruction that this is the one real open question:

1. **Whether the report button, having moved up 52px, still reads as the obvious primary action and remains comfortably thumb-reachable.** This is the single real cost the user accepted when choosing this stacking order (D-A) — not up for revisiting, but worth confirming it feels right in the hand.
2. Whether the toggle's glyph is actually centered in its circle (the cascade-orphan failure mode this task's CSS changes specifically address).
3. Whether the pair reads as one deliberate stack, with the report button clearly the dominant control, rather than as two unrelated floating circles.
4. Whether anything collides at 375px phone width — specifically Leaflet's top-left zoom control, the top-right account button, and the mobile-only bottom-left view-toggle, which now shares the toggle's height.
5. Whether a shown toast looks acceptable against the report button's new higher position (the toast is z-index 1200 and deliberately paints over both controls — pre-existing behavior, not a defect to look for).
6. Whether the toggle correctly sits behind the report modal's backdrop when the modal opens.
7. On the Activity page, whether the lone floating circle reads as a normal floating control now that it sits at the ordinary offset — a sanity check per D-C, not an open design question.
8. That one tap still flips the whole app cleanly in both places (theme.js itself was not touched, so this is a low-risk regression check, not a new behavior).

## IMPORTANT: dev server restart required

The CSS, JS and templates in this project are embedded into the Go binary at build time (`go:embed`). None of the changes in this task — the new partial, the removed account-menu row, or the repositioned CSS — will be visible in a running dev server until the binary is rebuilt and restarted. The user's dev server on port 8090 must be stopped and restarted from a fresh build before any of the live-look items above can be checked.

## Deviations from Plan

None — plan executed exactly as written. All three tasks completed with the exact file scope specified (theme_contract_test.go; the new partial plus account_header/index/profile templates; auth.css plus exactly one declaration in main.css), and Task 1's deliberate RED state was produced and committed as instructed before Tasks 2 and 3 turned it green.

## Known Stubs

None. No empty/placeholder data paths were introduced; this task only moved and repositioned existing, already-wired markup and CSS.

## Threat Flags

None. This task touched no network endpoint, no auth path, no file access pattern and no schema — it is a pure client-side markup/CSS relocation. The plan's own threat model (T-RRA-01 duplicate-control-id, T-RRA-02 report-button-overlap) is fully covered by the two contract tests above; no new surface was introduced beyond what the plan's threat register already accounts for.

## Self-Check

Created files exist:
- FOUND: web/templates/theme_toggle.html.tmpl

Modified files carry the expected changes:
- FOUND: `.theme-toggle` in web/static/css/auth.css declares `position: fixed`
- FOUND: `.fab` in web/static/css/main.css has the raised `calc()` bottom
- FOUND: no `theme-toggle` reference remains in web/templates/account_header.html.tmpl

Commits exist in git history:
- FOUND: d1d8435 (test: rewrite theme contract tests)
- FOUND: e597c3f (feat: extract theme toggle into a shared partial)
- FOUND: 8af3daa (feat: reposition theme toggle into the lower floating slot)

## Self-Check: PASSED
