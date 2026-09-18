---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-08"
subsystem: ui
tags: [css, trust.css, leaflet, hidden-attribute, specificity, wcag-contrast, gap-closure]

# Dependency graph
requires:
  - phase: 02-07
    provides: "Mark Resolved inline destructive confirmation (openResolveConfirm/closeResolveConfirm in votes.js), rendered on both the feed row and the map pin popup"
provides:
  - "trust.css's base vote-button rule guarded against the user-agent [hidden] specificity trap, so the confirm/dispute/resolve buttons actually disappear when openResolveConfirm hides them"
  - "A theme-aware Leaflet popup surface override (.leaflet-container .leaflet-popup-content-wrapper, .leaflet-container .leaflet-popup-tip), so popup content is no longer a white box in dark mode"
  - "Three new/extended regression gates: TestVoteBlockHiddenGuard's widened targets, TestMapPopupSurfaceIsThemeAware, TestResolveConfirmPaintsItsOwnSurface"
affects: [02-verify-work, 02-UAT, any-future-phase-that-styles-a-leaflet-popup]

tech-stack:
  added: []
  patterns:
    - "Every `display`-setting rule in trust.css (and feed.css/main.css/auth.css before it) must carry the `:not([hidden])` guard against the browser's user-agent-origin `[hidden] { display: none }` rule — this is the file's now-actually-complete discipline, checked by TestVoteBlockHiddenGuard's target list rather than asserted in prose alone."
    - "Overriding a vendor (Leaflet) rule that loads AFTER the app's own stylesheets requires winning on specificity, not source order — add an ancestor field (`.leaflet-container `) rather than relying on link order, and never redeclare a vendor property (e.g. box-shadow) that this file's own hex/functional-notation contract tests forbid."

key-files:
  created: []
  modified:
    - web/static/css/trust.css
    - web/votes_contract_test.go

key-decisions:
  - "Diagnosed the map-popup illegibility gap entirely from source (fetched vendor stylesheet + grep for zero app leaflet-popup rules), closing what 02-UAT.md had recorded as needing live DevTools inspection — see 'Diagnosis Retrospective' below."
  - "Did not touch ruleBySelector's exact-head matching semantics; moved the two callers to the guarded selector head instead of relaxing the helper to a substring search, per its own doc comment."
  - "Left the Leaflet popup close button's vendor rule untouched: its selector already out-specifies a two-class-plus-ancestor override, and its measured contrast (~4.2:1 dark, ~4.6:1 light) already clears the 3:1 graphical-object floor in both themes."

patterns-established:
  - "A vendor stylesheet linked after the app's own sheets needs a specificity-winning override, not a source-order-winning one — document the exact selector-head reason in the CSS comment so a future edit cannot 'simplify' the ancestor field away with a green build."

requirements-completed: [TRUST-08, TRUST-02]

coverage:
  - id: D1
    description: "Tapping 'Mark resolved' now REPLACES the Confirm/Dispute/Mark-resolved button row with the inline Yes/Cancel confirmation (buttons actually hidden, not just visually beside the prompt) on both the feed row and the map pin popup"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteBlockHiddenGuard"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVoteButtonsMeetTouchTargetAndUseTokensOnly"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveControlsMeetTouchTargetAndUseTokensOnly"
        status: pass
    human_judgment: true
    rationale: "Static CSS inspection proves the guard exists and the rule body is unguarded no longer; it cannot prove a real browser actually stops rendering the three buttons the instant the hidden attribute is set. Covered by this plan's own human-check step 1/2 in 02-UAT.md re-verification."
  - id: D2
    description: "The map popup's Mark Resolved confirmation heading is legible in dark mode (popup surface is now theme-aware instead of a hard-coded white Leaflet box)"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestMapPopupSurfaceIsThemeAware"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveConfirmPaintsItsOwnSurface"
        status: pass
    human_judgment: true
    rationale: "Static inspection proves the override's declarations exist, resolve through the correct tokens, and the token pair clears 4.5:1 WCAG AA contrast — it cannot prove a real browser composited the popup as expected, or falsify the light-mode-does-not-reproduce prediction this plan records. Covered by 02-UAT.md's human-check steps 2-4."

duration: ~35min
completed: 2026-09-18
status: complete
---

# Phase 2 Plan 8: Gap closure — hidden-attribute specificity guard and Leaflet popup theming Summary

**Guarded trust.css's base vote-button rule against the `[hidden]` specificity trap and gave the Leaflet map popup a theme-aware surface, closing both `severity: major` gaps from 02-UAT.md Test 3.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-09-18T08:34:44Z
- **Tasks:** 2/2
- **Files modified:** 2 (`web/static/css/trust.css`, `web/votes_contract_test.go`)

## Accomplishments

- **Gap 1 closed (D-15, TRUST-08):** `.vote-btn`'s selector head is now `.vote-btn:not([hidden])`. Before this change the base vote-button rule was the one omission in trust.css's own stated `:not([hidden])` discipline — `openResolveConfirm` correctly set `hidden = true` on the three primary buttons, but the browser's user-agent `[hidden] { display: none }` rule lost to the unguarded author-origin `display: inline-flex` at equal specificity, so the buttons kept rendering beside the Yes/Cancel confirmation on both the feed row and the map popup.
- **Gap 2 closed (TRUST-02):** Added `.leaflet-container .leaflet-popup-content-wrapper, .leaflet-container .leaflet-popup-tip { background: var(--color-bg); color: var(--color-text); }`. Leaflet 1.9.4's own stylesheet paints both a white background and a `#333` foreground, and this app had never overridden either since Phase 1 — so any Leaflet popup was a white box in every theme including dark, and the Mark Resolved confirmation's own dark-mode `--color-text` heading (near-white) landed on that white surface, invisible.
- `.resolve-confirm:not([hidden])` now paints its own `background: var(--color-bg)` as belt-and-braces, matching the base vote-button rule and the visibility-tag chip's existing self-painted-surface pattern.
- Three regression gates added/extended: `TestVoteBlockHiddenGuard`'s `targets` slice now includes `.vote-btn`; `TestMapPopupSurfaceIsThemeAware` (new) asserts the ancestor field is present on every popup-surface selector head, the combined wrapper+tip selector exists with exactly the two expected declarations, `leaflet@1.9.4/dist/leaflet.css` is still linked, and `--color-text`/`--color-bg` clear 4.5:1 WCAG AA contrast in all four theme sources; `TestResolveConfirmPaintsItsOwnSurface` (new) asserts the belt-and-braces background/color declarations.

## Task Commits

1. **Task 1: Complete trust.css's hidden-guard discipline on the base vote-button rule, and realign the two exact-head lookups that read it** - `2705743` (fix)
2. **Task 2: Give the Leaflet popup a theme-aware surface, so the confirmation heading has something legible to sit on** - `bdfa5a3` (fix)

**Plan metadata:** committed alongside this SUMMARY in worktree mode (STATE.md/ROADMAP.md updates deferred to the orchestrator).

## Files Created/Modified

- `web/static/css/trust.css` - Guarded `.vote-btn`'s selector head with `:not([hidden])` (body untouched); added the Leaflet popup surface override section; added `background: var(--color-bg)` to `.resolve-confirm:not([hidden])`.
- `web/votes_contract_test.go` - Widened `TestVoteBlockHiddenGuard`'s targets to include `.vote-btn`; realigned the two `ruleBySelector(rules, ".vote-btn")` lookups in `TestVoteButtonsMeetTouchTargetAndUseTokensOnly` and `TestResolveControlsMeetTouchTargetAndUseTokensOnly` to the guarded head; added `TestMapPopupSurfaceIsThemeAware` and `TestResolveConfirmPaintsItsOwnSurface`.

## Decisions Made

- **`ruleBySelector` itself was not touched.** Its doc comment states plainly that a future reader must not simplify it into a `strings.Contains` call, for a glyph-contrast reason specific to how it's used elsewhere in the package. Both call sites that read the now-guarded `.vote-btn` rule were moved to look up the guarded head instead.
- **The Leaflet popup close button was deliberately left alone.** Its vendor rule (`.leaflet-container a.leaflet-popup-close-button`) carries a type field plus two classes, already out-specifying a two-class-plus-ancestor override — matching it would tie and lose on source order, requiring an even longer selector for no benefit. Its `#757575` glyph measures ~4.2:1 against this app's dark `--color-bg` and ~4.6:1 against the light one, both clear of the 3:1 graphical-object floor, so it needed no override.
- **Leaflet's own popup box-shadow was not redeclared.** It uses a functional colour notation (`rgba(...)`) that two existing tests in `trust.css`'s test suite forbid outright, is theme-neutral, and needs no override — deliberately left to cascade from the vendor rule.

## Diagnosis Retrospective (recorded per this plan's `<output>` instructions)

**02-UAT.md's gap 2 was resolvable from source after all — it did not need live DevTools inspection.** The UAT investigation audited only `color` declarations and never asked what *background* the text sat on. The five-step chain that closed it, with its two primary sources:

1. Leaflet 1.9.4 ships `.leaflet-popup-content-wrapper, .leaflet-popup-tip { background: white; color: #333; ... }` — confirmed by fetching the pinned stylesheet the template actually links (`unpkg.com/leaflet@1.9.4/dist/leaflet.css`).
2. This repository had **zero** rules matching `leaflet-popup` or `map-popup` in any of its stylesheets before this plan — confirmed by grep across `web/static/css/`.
3. Therefore the map popup was a white box in dark mode, for the whole of Phases 1 and 2.
4. `.resolve-confirm p` sets `color: var(--color-text)`, which in dark mode is `#F3F4F6` — near-white text on Leaflet's white wrapper. Invisible.
5. This explains why the sibling vote buttons rendered at full contrast despite the same popup context: the base vote-button rule paints its own `background: var(--color-bg)` and therefore its own dark surface, while `.resolve-confirm` (before this plan) declared only a border. Phase 1's own popup text (`.map-popup__meta`, `.map-popup__description`) carried no app `color` rule at all and simply inherited Leaflet's dark-grey `#333`, legible on white by accident — which is why Phase 1's UAT passed and nobody noticed the white box.

**This is a latent-defect class, not a one-off bug.** Any future element mounted into a Leaflet popup that sets a theme-aware foreground without a background would have hit the same failure. The override removes the class of bug, not just this one instance.

**The falsifiable prediction and its outcome:** the diagnosis predicted this bug does not reproduce in light mode, because `--color-text` (`#1A1D21`) on Leaflet's white is roughly 16:1 — legible by coincidence. This plan's own automated gates (`TestMapPopupSurfaceIsThemeAware`'s WCAG assertions) confirm the light-theme token pair clears 4.5:1 as expected. Whether a live browser confirms the pre-fix bug genuinely did not reproduce in light mode is recorded by 02-UAT.md's re-verification pass (human-check, not run by this executor) — if it *did* reproduce in light mode, this diagnosis is wrong and must be reopened per the plan's own instruction.

**This is the repository's fifth encounter with the user-agent-`[hidden]` specificity trap** (`main.css`'s modal backdrop, `feed.css`'s empty state, `auth.css`'s account menu, `trust.css`'s visibility chip, and now the base vote button). The file's own header comment asserted the discipline was complete and it was not — `TestVoteBlockHiddenGuard`'s widened target list is what now makes that claim checkable rather than aspirational. The same question is worth asking of any stylesheet this project adds next.

## Deviations from Plan

None - plan executed exactly as written. Both tasks' acceptance criteria, verification commands, and prohibitions were followed without needing a Rule 1-4 deviation. One informal acceptance-criteria grep check (`grep -c '!important'` returning 0) does not literally hold against the raw file because of two PRE-EXISTING comment-text occurrences of the word "!important"/"rgb(" from prior plans (02-06/02-07), unrelated to anything this plan touched — the actual automated gate (`go test`, which operates on comment-stripped text) is unaffected and green, and no CSS declaration anywhere in the file uses `!important` or a raw functional colour notation. Not logged as a Rule 1-4 fix since nothing this plan wrote needed fixing; noted here only so a reviewer comparing raw grep output against the plan's literal acceptance-criteria text isn't confused by pre-existing prose.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both `severity: major` gaps in `02-UAT.md` Test 3 have an automated fix and a passing regression gate; the remaining step is the human-check re-verification (`/gsd-verify-work 2`) against a real browser in both themes, per `workflow.human_verify_mode: end-of-phase`.
- No blockers introduced for `02-09` or `02-10` (this plan touches zero files in common with either sibling gap-closure plan in this wave).

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-18*
