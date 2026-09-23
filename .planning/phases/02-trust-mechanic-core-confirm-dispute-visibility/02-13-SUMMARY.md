---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "02-13"
subsystem: ui
tags: [css, color-mix, wcag-contrast, trust-mechanic, visibility-states, gap-closure]

# Dependency graph
requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility
    provides: "plan 02-06's visibility-state cascade (.vis-provisional/.vis-hidden in trust.css) and plan 02-07's Mark Resolved block, both of which this plan edits in place"
provides:
  - "A Provisional badge/border/row-wash treatment strong enough to perceive, gated by a new automated perceptual-distance test"
  - "TestProvisionalDimmingIsPerceptiblyDistinctFromLive: a permanent regression gate on Provisional's colour strength, resolved from the shipped stylesheet rather than hardcoded"
  - ".vote-btn--resolve carrying the same [hidden] specificity guard every sibling rule in its block carries"
affects: [02-verification, 02-UAT]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Chroma-ratio + WCAG-luminance dual assertion for perceptual-distance gates on color-mix() states, following the existing resolveSeverityCurrentHex/wcagContrastRatio helper layer in web/css_contract_test.go"

key-files:
  created: []
  modified:
    - web/static/css/trust.css
    - web/css_contract_test.go
    - web/votes_contract_test.go

key-decisions:
  - "Provisional's color-mix() ratio changed from 50/50 (severity/stale) to 20/80, chosen and verified against the shipped token tables before this plan was written — not tuned empirically during execution"
  - "Colourfulness (chroma), not luminance, is the primary perceptual assertion, because light mode's near-white stale token makes a 3:1 luminance floor mathematically unreachable there even at a 100% mix"
  - "The row's --severity-tint changes to var(--color-surface) (neutral), not transparent — transparent is exactly .vis-hidden's treatment, and collapsing the two states was explicitly rejected"
  - ".vote-btn--resolve gets the [hidden] guard as a consistency fix only; the live cause of UAT Test 6 (button staying visible) remains unidentified after exhaustive two-engine headless investigation, and the SUMMARY does not claim otherwise"
  - "The three sibling colour-only rules in the Mark Resolved block (.vote-btn--confirm-resolve, .vote-btn--cancel-resolve, .vote-btn--reopen) are deliberately left unguarded — the debug session's evidence pointed only at .vote-btn--resolve's colour signature, and broadening the change would add unproven surface"

patterns-established:
  - "A perceptual-distance CSS contract test asserts BOTH a theme-symmetric chroma ratio ceiling AND per-theme honest luminance floors, rather than a single number, when a palette's dark/light halves aren't luminance-symmetric"

requirements-completed: [TRUST-04, TRUST-08]

coverage:
  - id: D1
    description: "Provisional badge/border/row-wash now reads as visibly drained of colour on both the feed row and the map pin, in both themes, guarded by an automated perceptual-distance gate"
    requirement: "TRUST-04"
    verification:
      - kind: unit
        ref: "web/css_contract_test.go#TestProvisionalDimmingIsPerceptiblyDistinctFromLive"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestVisibilityCascadeOverridesAgeRamp"
        status: pass
    human_judgment: true
    rationale: "Static colour resolution proves the declared colours are far enough apart by two independent measures (chroma ratio, WCAG luminance) but cannot prove a person perceives the difference on their own screen — this is exactly the gap UAT Test 5 exposed even though every prior automated gate was green. The plan's own human-check (both themes, feed row and map pin, Provisional vs Disputed distinguishability) is the decisive confirmation."
  - id: D2
    description: ".vote-btn--resolve carries the [hidden] specificity guard every sibling display-bearing rule in trust.css carries, closing the one documented divergence from this file's own guard discipline"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestResolveControlsMeetTouchTargetAndUseTokensOnly"
        status: pass
    human_judgment: true
    rationale: "This is a defensive consistency fix, not a confirmed repair — exhaustive two-engine (Chromium, WebKit) headless reproduction against both an isolated block and the real feed-row DOM never reproduced UAT Test 6's reported symptom, so the live cause remains unidentified per .planning/debug/resolve-button-not-hidden.md. Only a live DevTools capture at the moment the symptom recurs (the plan's human-check step) can settle whether this fix addresses the real mechanism."

# Metrics
duration: 55min
completed: 2026-09-23
status: complete
---

# Phase 02 Plan 13: Provisional Dimming Perceptibility & Mark Resolved Guard Summary

**Doubled the Provisional visibility-state's color-mix() dimming strength (50/50 → 20/80 severity/stale), added a chroma+luminance perceptual-distance regression gate that did not exist before, neutralised the feed row's competing background wash, and restored a missing `[hidden]` guard on the Mark Resolved button.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-09-23
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Added `TestProvisionalDimmingIsPerceptiblyDistinctFromLive` to `web/css_contract_test.go` — a new automated gate resolving `.vis-provisional`'s and `.age-fresh`'s colours from the shipped stylesheet and asserting a chroma-ratio ceiling, per-theme luminance floors, glyph legibility, and Provisional/Hidden distinguishability. Confirmed RED against the unmodified 50/50 mix before any CSS changed.
- Strengthened `.vis-provisional`'s `--severity-current` from a 50/50 to a 20/80 severity/stale `color-mix()`, and added `--severity-tint: var(--color-surface)` so the feed row's background wash goes neutral instead of staying a full-strength severity tint.
- Restored the `[hidden]` specificity guard on `.vote-btn--resolve` for consistency with every other display-bearing rule in trust.css's Mark Resolved block — a defensive fix, not a confirmed repair (see Gap 5 section below).
- Rewrote the two existing contract tests Task 2 deliberately broke (`TestVisibilityCascadeOverridesAgeRamp`, `TestResolveControlsMeetTouchTargetAndUseTokensOnly`) onto the new contract, with the reasoning for each change recorded in their own doc comments.

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Write the perceptual-distance gate and watch it fail** — `8c5f391` (test)
2. **Task 2 (GREEN): Strengthen the Provisional treatment, neutralise the row wash, restore the Mark Resolved guard** — `9e5a97b` (feat)
3. **Task 3: Update the two contract tests this change deliberately broke** — `79cd442` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP update, applied by the orchestrator after wave merge — this is a worktree-isolated wave-9 plan, so STATE.md/ROADMAP.md are not touched here per the executor's worktree-mode contract)

## Files Created/Modified

- `web/css_contract_test.go` — Added `TestProvisionalDimmingIsPerceptiblyDistinctFromLive` and its `chromaOf` helper (331 lines added)
- `web/static/css/trust.css` — `.vis-provisional`'s color-mix ratio and `--severity-tint`, rationale comment rewrite, `.vote-btn--resolve` guard + comment (68 lines changed)
- `web/votes_contract_test.go` — `TestVisibilityCascadeOverridesAgeRamp` rewritten off string-equality onto a strictly-stronger-than-Aging invariant; `TestResolveControlsMeetTouchTargetAndUseTokensOnly`'s resolve-rule lookup updated to the guarded selector head (88 lines changed)

## The Before-and-After Numbers

Measured by the new test itself (`TestProvisionalDimmingIsPerceptiblyDistinctFromLive`), both measures, both themes, low and medium severity (the only severities that can ever be Provisional — a critical or rescue-needed report is never Provisional per D-06's bypass rung in `internal/service/visibility.go`'s `Resolve()`):

| Severity | Theme | Chroma ratio before (50/50) | Chroma ratio after (20/80) | Luminance before | Luminance after | Floor |
|---|---|---|---|---|---|---|
| low | dark | 0.51 | **0.22** | 1.97:1 | **3.16:1** | 2.80:1 |
| low | light | 0.52 | **0.23** | 1.46:1 | **1.83:1** | 1.65:1 |
| medium | dark | 0.47 | **0.15** | 1.90:1 | **2.93:1** | 2.80:1 |
| medium | light | 0.48 | **0.16** | 1.41:1 | **1.71:1** | 1.65:1 |

(Chroma ceiling for the new value is 0.30; all four severities including critical — computed mathematically even though not a renderable state — land between 0.15 and 0.23 after the change, well under the ceiling.)

## Decisions Made

- **The code was never broken.** `applyVisibilityClass` reached the right element on both surfaces (`feed.js` on the row ancestor, `map.js` on the pin's badge) with the right cascade order the whole time, confirmed by `.planning/debug/provisional-dimming-not-perceptible.md`'s diagnosis before this plan started. The defect was design strength, not wiring — worth stating plainly because it's a more useful lesson than a wiring bug would have been, and it's exactly why the new gate exists: nothing in the prior test suite could have caught a "technically-applied-but-too-subtle" defect of this shape.
- **Why the light-mode luminance floor (1.65:1) is lower than dark mode's (2.80:1), and this is a palette constraint, not a compromise.** Light mode's `--color-age-stale` token is a near-white grey, so mixing toward it *raises* luminance while the severity colour is mid-dark — even a 100% mix only reaches roughly 2:1 there. A 3:1 floor is mathematically unreachable in light mode with the tokens this app ships, so the floor was set just below what the chosen 20/80 ratio actually achieves. Colourfulness (chroma), not luminance, is what carries the perceptual claim in light mode — this is why Assertion A (chroma) has no theme-dependent floor while Assertion B (luminance) does.
- **The reuse that was severed, and `02-06` was not wrong to establish it.** Reusing `.age-aging`'s 50/50 value for Provisional was a reasonable consistency decision — "not yet trusted" dimmed exactly as hard as "about a quarter of its lifetime left". What was missing was any test asserting either value was far enough from the undimmed baseline to actually see. Both facts are true at once.
- **The tint decision and its two rejected alternatives.** `transparent` was rejected because it is exactly `.vis-hidden`'s own treatment, and collapsing Provisional onto Hidden's look would trade one closed UAT gap (Test 5) for a reopened one (Test 2, which confirmed Hidden's outline treatment correct). A new custom property was rejected because `TestVisibilityCascadeOverridesAgeRamp`'s custom-property allowlist forbids inventing a fourth token, and the existing `--color-surface` neutral token does the job.
- **Gap 5 (Mark Resolved), stated with its real confidence.** The `[hidden]` guard was added for consistency with every sibling rule in its block, not because a live defect was confirmed. Causality was never established across two rendering engines (Chromium, WebKit), isolated-block and full-feed-row-DOM reproductions, and a simulated background-poll-mid-confirmation race. If the symptom recurs, the next step is a live DevTools capture on the actual device (Elements panel: does the element carry `hidden=""`? Computed panel: what wins for `display`?) — not another static pass. The map popup surface (`map.js` rebuilds its popup content fresh on every render) was never covered by the original investigation and has its own separate human-check step in the plan.
- **The test breakage the planner found that the debug session did not.** `TestResolveControlsMeetTouchTargetAndUseTokensOnly` looks the resolve rule up by exact selector head, so it broke the moment the `[hidden]` guard was added to `.vote-btn--resolve`. Worth recording as a reminder that this codebase's exact-head matchers (`ruleBySelector`, by design — see its own doc comment) make a guard change a two-file edit, not a one-file edit.

## Deviations from Plan

None — plan executed exactly as written, including the recourse line the orchestrator added after plan-check review (measured chroma ratios stayed under the 0.30 ceiling on the first 20/80 attempt; the 15/85-then-10/90 escalation path was never needed).

## Issues Encountered

None. The plan's own Task 1 RED numbers matched the diagnosis document's pre-computed predictions closely (chroma ratios ~0.46-0.52 vs. diagnosis's "roughly 0.47 to 0.52"; dark luminance ~1.90-1.97:1 vs. diagnosis's "1.89-1.97:1"), and Task 2's GREEN numbers landed comfortably under/over the stated ceilings/floors on the first attempt with the exact 20/80 ratio specified in the plan.

## User Setup Required

None — no external service configuration required.

## Whether the Light-Mode Walkthrough Found Anything

Not applicable to this plan — the light-mode visual walkthrough is a `<human-check>` step deferred to end-of-phase verification (`workflow.human_verify_mode: end-of-phase`), not something this executor performs. This plan's own contribution to that walkthrough is that the automated gate now proves light mode's numbers moved in the same direction as dark mode's (chroma ratio 0.52→0.23 and 0.48→0.16 for low/medium respectively), closing the "light mode has never once been looked at" gap the diagnosis flagged — but the actual human look-and-confirm step is recorded separately in `02-UAT.md` / the phase's end-of-phase human-check, not here.

## Next Phase Readiness

- `web` package's full test suite is green (`go test ./web/ -count=1`), `go test ./... -short` is green, `go build ./...` and `go vet ./...` are clean.
- `git status --porcelain` after all three task commits lists exactly the three `files_modified` paths declared in this plan's frontmatter — no collision with a sibling wave-9 plan (02-11, 02-12, 02-14) and no scope creep.
- This plan is one of four mutually parallel wave-9 gap-closure plans sharing zero files; ready for the orchestrator's post-wave merge and shared-state update (STATE.md, ROADMAP.md, REQUIREMENTS.md), followed by the phase's end-of-phase human-check covering both this plan's Gap 4/Gap 5 items and the other three plans' items together.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Plan: 02-13*
*Completed: 2026-09-23*
