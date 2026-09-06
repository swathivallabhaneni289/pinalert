---
phase: 01-foundation-report-map
plan: 05
subsystem: ui
tags: [vanilla-js, accessibility, aria, leaflet, dom-contract, xss-safety]

# Dependency graph
requires:
  - phase: 01-foundation-report-map (plan 01-02)
    provides: web/static/css design-token layer (--space-*, --font-*, --color-*, --severity-current, --touch-target-min), .category-tile/.modal-panel/.severity-slider shared classes, 9 committed Lucide icons
  - phase: 01-foundation-report-map (plan 01-03)
    provides: internal/service.ReportService (Category/Severity/CapacityStatus enums, ValidateSubmitInput, exact server validation messages), POST/GET /api/reports contract
  - phase: 01-foundation-report-map (plan 01-04)
    provides: web/templates/index.html.tmpl DOM contract (modal skeleton, shelter fields, discard-confirm, toast), window.Pinalert shared client store, the thin modal.js this plan replaces
provides:
  - "web/static/js/modal.js — production submission modal: 3x3 accessible category radio-grid, GPS-prefilled draggable marker with tap-to-place fallback, native accessible severity slider with animated traffic-light context, conditional shelter-capacity fieldset, full inline-validation/submit/discard state machine"
  - "web/static/css/modal.css — modal layout, category grid, severity slider transition/position-labels, shelter fieldset, inline error, toast, discard overlay — all consuming main.css tokens"
  - "category-change DOM event on #category-grid (detail: {category}) — the seam between category selection and shelter-field visibility"
affects:
  - "01-06 (feed list) — shares window.Pinalert but does not read modal.js; no coupling"
  - "any future phase touching the report submission flow must preserve the DOM ids (#category-grid, #severity, #shelter-fields, #discard-confirm, etc.) this plan wires against"

tech-stack:
  added: []
  patterns:
    - "Roving-tabindex radio group: role=radio/aria-checked per tile, exactly one tabIndex=0 at a time, arrow keys (+/-1 for left/right, +/-3 for up/down, wrapping) move both focus and selection in one step"
    - "CSS custom-property inheritance for state-driven color: a severity-control wrapper toggles sev-low/sev-medium/sev-critical (defined once in main.css) so --severity-current cascades to the native range input, its pseudo-element thumb, and the readout without any of them owning their own color logic"
    - "Native form controls kept native: the severity slider is the browser's own <input type=range> (no custom slider role anywhere in the file) and the shelter-capacity control is a native <select>, both because the browser's built-in keyboard/screen-reader semantics are correct by construction and a hand-built substitute would have to reimplement them"
    - "Client-side validation strictly mirrors internal/service.ValidateSubmitInput's rules and messages for usability only; the server remains the sole authority (T-01-21) — verified by matching the shelter-capacity message to the server's exact ValidationError string"
    - ":not([hidden]) compound selectors for any ID-selector rule that sets `display` on a native [hidden] element (#shelter-fields, #discard-confirm) — an ID selector alone would out-specify the browser's [hidden] { display: none } and defeat the JS hidden toggle"
    - "formTouched tracked only on genuine user interaction (click/keydown/drag/change/input), never on programmatic GPS auto-fill or resetForm's own assignments — so an untouched modal never gets the discard-confirmation prompt"

key-files:
  created: []
  modified:
    - web/static/js/modal.js
    - web/static/css/modal.css

key-decisions:
  - "Populated #shelter-capacity-status from Pinalert.CAPACITY_STATUSES at init (clearing the template's static <option> list) rather than leaving the four options hardcoded in index.html.tmpl, so the capacity enum has exactly one source of truth, matching how the category grid is built from Pinalert.CATEGORIES."
  - "Wrapped the existing native <label>/<input type=range>/<output> in a JS-created .severity-control div (once, at script init, by reparenting the existing DOM nodes) rather than editing index.html.tmpl (which this plan may not touch) — this gives one class-toggle point for --severity-current to cascade through CSS inheritance to the slider, its thumb, and the readout simultaneously."
  - "Client-side 'Choose a shelter capacity status.' message matches internal/service.report.go's ValidationError string verbatim, so the client and server rejection wording never drifts even though the client check is unreachable in practice (a native <select> always has a value)."
  - "Discard-confirmation Escape handling: if #discard-confirm is already open, Escape acts as 'Keep editing' (cancels the confirm) rather than force-discarding or closing the whole modal — matches the common modal-on-modal convention that Escape backs out one layer at a time."
  - "formTouched is set only on click/keydown/drag/change/input handlers a user actually triggers — GPS auto-placing the marker on open, and resetForm's own field assignments, never set it — so an untouched modal (even one with a GPS-derived default pin) closes without the discard prompt, matching the acceptance criteria's 'closing an untouched (empty) modal does not' ask before discarding."

requirements-completed: [FOUND-02, FOUND-06]

coverage:
  - id: D1
    description: "3x3 category icon grid behaves as a proper radio group: role=radio/aria-checked, roving tabindex, arrow-key navigation, category-change event fired on selection"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "node --check web/static/js/modal.js; grep gates for role/aria-checked/CATEGORIES/category-change"
        status: pass
    human_judgment: false
  - id: D2
    description: "Severity slider is the native range input with aria-valuetext + #severity-readout announcing the number-plus-word form on every change, animated traffic-light color via --severity-current, 44px thumb, visible focus ring, prefers-reduced-motion guard"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "grep gates for aria-valuetext/SEVERITY_LABELS/no-custom-slider-role in modal.js; prefers-reduced-motion/transition/touch-target-min/thumb-selectors in modal.css"
        status: pass
      - kind: manual_procedural
        ref: "plan 01-05 Task 2 <human-check> — keyboard/VoiceOver walkthrough"
        status: unknown
    human_judgment: true
    rationale: "Screen-reader announcement correctness and visual color-ramp animation can only be confirmed by a human with a real assistive-tech stack and a phone-width viewport; deferred to end-of-phase per this plan's autonomous/human_verify_mode:end-of-phase setting, same handling plan 01-04 used."
  - id: D3
    description: "shelter_open reveals a required capacity status (from Pinalert.CAPACITY_STATUSES) and optional headcount; every other category hides and clears both, and the submit payload omits both keys entirely"
    requirement: "FOUND-06"
    verification:
      - kind: unit
        ref: "grep gates for shelter_open/CAPACITY_STATUSES in modal.js; go test ./internal/service -run TestShelterCapacityValidation (server-side contract this client mirrors)"
        status: pass
    human_judgment: false
  - id: D4
    description: "All four Copywriting Contract validation messages, the two submit-button labels, the success toast, and both discard-prompt strings appear verbatim; a server {field, message} error renders as text and focuses the matching control"
    requirement: "FOUND-02"
    verification:
      - kind: unit
        ref: "verify_copy.sh grep gate (9 exact strings) against modal.js; innerHTML-absence grep"
        status: pass
    human_judgment: false
  - id: D5
    description: "Discard confirmation appears only when a field has been touched (Cancel, Escape, or backdrop click); an untouched modal closes immediately with no prompt"
    requirement: "FOUND-02"
    verification:
      - kind: manual_procedural
        ref: "plan 01-05 Task 3 <human-check> steps 4-5"
        status: unknown
    human_judgment: true
    rationale: "Whether a prompt does or doesn't appear at the right moment is a live-interaction UX check better confirmed by a human clicking through the real browser than inferred from source grep."
  - id: D6
    description: "go build/go vet/go test ./... -p 1 remain green after all three tasks; only web/static/js/modal.js and web/static/css/modal.css were modified"
    requirement: null
    verification:
      - kind: integration
        ref: "go build ./... && go vet ./... && go test ./... -p 1 (all packages ok)"
        status: pass
      - kind: unit
        ref: "git status --short shows only the two owned files modified"
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-09-06
status: complete
---

# Phase 1 Plan 05: Report Submission Modal (Full Design Contract) Summary

**Replaced plan 01-04's thin stand-in with the full UI-SPEC submission modal — an accessible 3x3 category radio-grid, a native severity slider with an animated traffic-light readout, GPS-prefilled draggable location, a conditional shelter-capacity fieldset, and the complete inline-validation/submit/discard state machine, all wired against the DOM contract and design tokens plans 01-02/01-04 already committed.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-09-06
- **Tasks:** 3 (each a single `feat` commit — no RED/GREEN cycle, plan is `autonomous: true`, not `type: tdd`)
- **Files modified:** 2 (`web/static/js/modal.js`, `web/static/css/modal.css`)

## Accomplishments

- **3x3 category radio-grid (D-01, D-04):** `#category-grid` renders exactly nine `.category-tile` buttons in `Pinalert.CATEGORIES` order with `role="radio"`/`aria-checked`, a roving tabindex, and arrow-key navigation (wrapping Left/Right by one, Up/Down by a row of three). Selecting a tile fires a `category-change` custom event that Task 3's shelter-fieldset logic listens for — the two features don't know about each other beyond that event contract.
- **Accessible animated severity slider (D-05, D-11):** kept the native `<input type="range">` — no custom slider role anywhere in the file. A JS-created `.severity-control` wrapper reparents the existing label/input/output and toggles a `sev-low`/`sev-medium`/`sev-critical` class; those classes (already defined in `main.css`) set `--severity-current`, which then cascades via CSS custom-property inheritance to the track fill, the thumb border, and the readout text color simultaneously — no component owns its own color logic. `aria-valuetext` and `#severity-readout` both update to the number-plus-word form ("1 · Low"/"2 · Medium"/"3 · Critical") on every `input` event. Three `aria-hidden` static position labels render beneath the track. All color transitions are wrapped in a `prefers-reduced-motion: reduce` guard.
- **Shelter capacity fields (FOUND-06):** `#shelter-capacity-status` is populated from `Pinalert.CAPACITY_STATUSES` (not the template's static options) so the enum has one source of truth. Selecting `shelter_open` reveals the fieldset and marks the status required; any other category hides it, clears both fields, and the submit payload omits both keys entirely — matching the server's rejection of either key on a non-shelter report.
- **Full validation/submit/discard state machine:** all four Copywriting Contract messages plus a server-matching "Choose a shelter capacity status." message, submit-time focus of the first invalid control, blur-validation on the description field, a disabling "Posting…" submit state, a "Report posted." toast on success, intact-input-on-failure with the server's `{field, message}` rendered as text and focused, and a discard-confirmation prompt (Cancel/Escape/backdrop-click) that appears only when `formTouched` is true.
- **Location picker refined (D-02):** GPS pre-fills a draggable marker at zoom 16 with a live coordinate readout on drag; denial/timeout/insecure-context shows the exact inline notice and falls back to tap-to-place — submission is never blocked on geolocation.

## Task Commits

1. **Task 1: 3x3 category radio-grid and refined location picker** — `bc1765a` (feat)
2. **Task 2: Accessible animated severity slider** — `3e6d5be` (feat)
3. **Task 3: Shelter capacity fields, inline validation copy, submit/discard states** — `7240d39` (feat)

**Plan metadata:** committed as part of this SUMMARY (worktree mode — orchestrator handles STATE.md/ROADMAP.md after merge, per this agent's instructions).

## Files Created/Modified

- `web/static/js/modal.js` — full rewrite of plan 01-04's thin stand-in: category radio-grid, severity control wrapper, shelter-capacity gating, validation/submit/discard state machine
- `web/static/css/modal.css` — full rewrite of the plan 01-04 placeholder: modal layout, category grid, severity slider transitions/position-labels, shelter fieldset, inline error, toast, discard overlay

## Decisions Made

See `key-decisions` in frontmatter. The two with the widest blast radius: reparenting the existing severity DOM nodes into a JS-created wrapper (rather than editing `index.html.tmpl`, which this plan cannot touch) so `--severity-current` cascades through CSS inheritance to every severity-aware element from a single class toggle; and populating the shelter-capacity `<select>` from `Pinalert.CAPACITY_STATUSES` at runtime so the four-status enum has exactly one source of truth across the whole client.

## Deviations from Plan

None — plan executed exactly as written. Both `<action>` blocks per task were followed as specified; no Rule 1/2/3 auto-fixes were needed and no architectural questions (Rule 4) arose. The one thing worth calling out is not a deviation but a fix made during self-verification, documented below.

### Note: CSS correctness fix caught during self-review

While reviewing the finished `modal.css`, `#form-error`'s `min-height` was initially set to `var(--line-height-label)` (a unitless number, `1.4`) — an invalid value for the `min-height` property, which the browser would silently ignore. Fixed to `calc(var(--font-size-label) * var(--line-height-label))`, a valid computed length, before the Task 3 commit. This was caught and corrected within the same task/commit boundary, not a post-hoc fix, so it is not logged as a separate Rule 1 deviation — no incorrect code was ever committed.

## Verification

- `node --check web/static/js/modal.js` passes after every task.
- Zero `innerHTML` occurrences (comment-stripped) in `modal.js`.
- All 9 exact Copywriting Contract strings present verbatim in `modal.js` (validation messages, button labels, toast, discard-prompt copy).
- `modal.css` contains zero raw hex colors (`grep -cE '#[0-9a-fA-F]{6}'` = 0) and consumes `main.css` tokens throughout.
- Upstream DOM/CSS contract re-verified intact: every id this plan depends on (`report-modal`, `modal-map`, `coord-readout`, `location-notice`, `category-grid`, `severity-readout`, `description`, `shelter-fields`, `form-error`, `submit-report`, `cancel-report`, `discard-confirm`, `toast`) still present in `index.html.tmpl`; every `main.css` class this plan depends on (`category-tile`, `modal-panel`, `modal-backdrop`, `severity-slider`, `touch-target-min`) still present.
- No custom `role="slider"` declared anywhere in `modal.js` — the native range input is the sole severity control.
- `go build ./...`, `go vet ./...`, `go test ./... -p 1` all green against the real local Postgres (`internal/api/handlers`, `internal/service`, `internal/session`, `internal/store`, `internal/testutil` all pass).
- `git status --short` after the final commit shows only `web/static/js/modal.js` and `web/static/css/modal.css` were ever modified — no cross-plan file touched.

## Pending Human Verification

This plan is `autonomous: true` with the phase's `human_verify_mode: end-of-phase` handling (same pattern 01-04 used) — the two `<human-check>` blocks below were not executed live in this session and should be confirmed at end-of-phase in a real browser:

1. **Task 2 (severity slider):** Tab to the slider and confirm a visible focus ring on the thumb; Left/Right/Up/Down arrow keys and Home/End change the value; the readout updates to "1 · Low"/"2 · Medium"/"3 · Critical" with the track animating green → amber → red; a screen reader (VoiceOver or equivalent) announces the full label, not just the number; the thumb is comfortably tappable on a phone-width viewport; enabling "Reduce motion" removes the color animation while the control stays fully usable.
2. **Task 3 (shelter/validation/discard flow):** selecting "Shelter open" reveals the capacity control and it's required; switching away hides it and submission still succeeds; submitting with no category / no description / a too-short description shows the exact contract copy and focuses the offending field in turn; a valid shelter report with capacity "Limited" and headcount 40 is accepted and appears on the map; typing a description then pressing Escape shows the discard prompt, "Keep editing" returns to the filled form, "Discard" closes it; opening and immediately pressing Escape on an untouched modal shows no prompt.

## Known Stubs

None. Every DOM hook and behavior this plan promises is wired to the real `window.Pinalert` client store and the real server contract from plan 01-03 — no placeholder data, no hardcoded empty state feeding the UI, no "coming soon" copy anywhere in the file.

## Threat Flags

None. This plan operates entirely within the trust boundaries already declared in `01-05-PLAN.md`'s `<threat_model>` (server error response → modal DOM; visitor input → `POST /api/reports`; browser geolocation → report coordinate) and introduces no new network endpoint, auth path, file-access pattern, or schema change. The shelter-capacity `<select>` population from `Pinalert.CAPACITY_STATUSES` and the category-grid population from `Pinalert.CATEGORIES` are both closed, hardcoded enums already covered by T-01-07 (payload field injection) in the plan's threat register — no new surface.

## Self-Check: PASSED

- `web/static/js/modal.js` — FOUND on disk.
- `web/static/css/modal.css` — FOUND on disk.
- Commit `bc1765a` — FOUND in `git log`.
- Commit `3e6d5be` — FOUND in `git log`.
- Commit `7240d39` — FOUND in `git log`.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*
