---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "06"
subsystem: ui
tags: [vanilla-js, css-cascade, trust-mechanic, visibility-resolver, contract-tests]

requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility
    provides: "02-05's shared vote-block builder (window.PinalertVotes), its trust.css conventions (comment-stripped contract tests, the mandatory :not([hidden]) guard, the custom-property-only/no-raw-colour rule), and votes_contract_test.go's shared helpers (stripCSSComments, parseCSSRules, declsOf, ruleBySelector, windowAfter, findTagWindow, jsFunctionBody)"
provides:
  - "window.PinalertVisibility: the display half of the trust mechanic — validated state/reason resolution, the visibility-tag chip builder/updater, and the vis-<state> class applier, computing nothing"
  - "trust.css's .vis-provisional / .vis-hidden cascade rules, the hidden-badge dual-selector rule, and the .visibility-tag chip rule"
  - "The 'Show disputed reports' toggle: Pinalert.state.showDisputed / Pinalert.setShowDisputed, one conditional show_disputed=true on the one report fetch, and the #disputed-empty empty state"
  - "Both feed.js and map.js mounting the same visibility tag from one builder, at the insertion point 02-05 reserved"
affects: [02-07]

tech-stack:
  added: []
  patterns:
    - "One-builder-two-surfaces (D-01 continued): window.PinalertVisibility serves both feed.js's createRow/updateRow and map.js's buildBadgeElement/buildPopupContent, the same shape 02-05 established for the vote block"
    - "Fail-closed validation (T-01-17 continued): visibilityState/visibilityReason validate every server-supplied field against a fixed allowlist before it reaches a class name or attribute, falling back to Provisional (never Live) on anything unrecognised"
    - "Shared-fetch-shared-array (D-11's mechanism): one boolean flag conditions the one GET /api/reports call app.js owns; both surfaces render from the one resulting state.reports array, so there is no second query where they could disagree"
    - "Two [hidden]-guard forms, applied by which rule owns the display value: :not([hidden]) when this plan's own rule sets display (the chip), the #id[hidden] override when a shared component rule (main.css's .empty-state) sets it"

key-files:
  created:
    - web/static/js/visibility.js
  modified:
    - web/static/js/feed.js
    - web/static/js/map.js
    - web/static/js/app.js
    - web/static/css/trust.css
    - web/templates/index.html.tmpl
    - web/template_contract_test.go
    - web/votes_contract_test.go

key-decisions:
  - "createVisibilityTag/updateVisibilityTag/applyVisibilityClass are the only three PinalertVisibility entries feed.js and map.js call — no class literal, response-field name or query-param name appears in either consumer, enforced by TestBothSurfacesMountTheSameVisibilityTag and TestShowDisputedUsesOneSharedQueryParam"
  - "The Provisional --severity-current value is asserted string-equal (at test time, read from main.css) to .age-aging's own value, not hardcoded twice, so the two cannot silently drift apart"
  - "TestVisibilityCascadeOverridesAgeRamp's custom-property allowlist (exactly --severity-current/--severity-tint/--badge-glyph-fg) supersedes 02-05's cruder grep -cE '^\\s*--[a-z-]+:' = 0 gate, which could not distinguish re-declaring an existing property from inventing a new token"
  - "Fixed a latent, pre-existing bug in feed.js (from Phase 1/01-06): a comment containing the literal substring \"/static/icons/*.svg\" caused this package's shared stripCSSComments test helper to treat it as an unterminated block comment and silently drop everything in the file after that line. No prior test happened to statically inspect code past that point via this helper; TestFeedExplainsAnEmptyDisputedResult (Task 3) was the first to do so and surfaced it. Fixed by rephrasing the comment to avoid the accidental /* sequence — no behavioural change, comment text only."

requirements-completed: [TRUST-02, TRUST-04]

coverage:
  - id: D1
    description: "A Provisional report reads as not-yet-trusted on both surfaces at once: the row/pin desaturate through Phase 1's exact expiry-fade mix, and an explicit \"Unconfirmed\" chip renders — both treatments, never one alone"
    requirement: "TRUST-04"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVisibilityCascadeOverridesAgeRamp"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestBothSurfacesMountTheSameVisibilityTag"
        status: pass
    human_judgment: true
    rationale: "Static inspection proves the CSS values are string-equal to main.css's own and that both surfaces call the same builder in the right order. It cannot prove the rendered cascade actually resolves to the desaturated pixels in a real browser, or that the chip is legible next to the row — Task 3's <human-check> (deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase) carries that claim."
  - id: D2
    description: "Checking \"Show disputed reports\" adds show_disputed=true to the one GET /api/reports call the shared store owns, so Hidden rows and Hidden map pins appear together from one fetch and one state.reports array; unchecking removes both through the existing reconcile-and-remove loops with no new removal code"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestShowDisputedUsesOneSharedQueryParam"
        status: pass
    human_judgment: true
    rationale: "Static inspection proves there is exactly one fetch, one parameter, and neither renderer builds its own query. It cannot prove the pins/rows actually appear and disappear together in a real browser, nor that a Hidden pin is visually findable against the basemap — both covered by Task 3's <human-check>."
  - id: D3
    description: "A Hidden report renders with the outline treatment on both surfaces — transparent badge fill with a neutral outline ring, muted glyph, neutral row left-border, no severity tint — reaching the badge through both the compound and descendant selector forms main.css's own comment says are each individually insufficient"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVisibilityCascadeOverridesAgeRamp"
        status: pass
    human_judgment: true
    rationale: "Static inspection proves both selector forms exist in one rule and that no orphan/glyph gate is tripped. It cannot prove the pin stays visually findable against the basemap in a real browser — Task 3's <human-check> step 6 covers that and asks for a screenshot if not."
  - id: D4
    description: "Turning the toggle on with nothing disputed nearby renders the Copywriting Contract's exact empty-state copy instead of a blank list, via the existing .empty-state component, with setEmptyState the sole owner of both empty-state elements"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestFeedExplainsAnEmptyDisputedResult"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestDisputedEmptyStateHiddenGuard"
        status: pass
    human_judgment: false
  - id: D5
    description: "An unrecognised, missing or non-string visibility value renders as Provisional, never as Live — the client never upgrades a report's apparent trust on a value it could not validate"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVisibilityTagCopyAndFallback"
        status: pass
    human_judgment: false
  - id: D6
    description: "The mandatory [hidden] guard on the chip and the id-and-attribute guard on the disputed empty state both exist, each with a vacuity check so the gate cannot pass if the guarded rule were deleted"
    verification:
      - kind: unit
        ref: "web/votes_contract_test.go#TestVisibilityTagHiddenGuard"
        status: pass
      - kind: unit
        ref: "web/votes_contract_test.go#TestDisputedEmptyStateHiddenGuard"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-15
status: complete
---

# Phase 2 Plan 06: Visibility Display — Confirm/Dispute Trust State Becomes Legible Summary

**`window.PinalertVisibility` chip/cascade module, a `.vis-provisional`/`.vis-hidden` CSS cascade that byte-matches Phase 1's expiry-fade mix, and a "Show disputed reports" toggle that adds one shared query parameter to the client's single report fetch.**

## Performance

- **Duration:** 55 min (estimated)
- **Tasks:** 3
- **Files:** 1 created, 7 modified

## Accomplishments

- `web/static/js/visibility.js`: a new module that reads the resolver's `visibility`/`visibility_reason` fields, validates each against a closed allowlist (T-01-17), and exposes a chip builder/updater plus a state-class applier. Computes nothing — no threshold, no tally, no derivation.
- `trust.css` gained the D-09 cascade: `.vis-provisional` reuses `.age-aging`'s exact desaturation mix (asserted string-equal at test time against main.css, not hardcoded), `.vis-hidden` carries the outline badge treatment through both the compound (`.icon-badge.vis-hidden`) and descendant (`.vis-hidden .icon-badge`) selector forms, and `.visibility-tag` ships the mandatory `:not([hidden])` guard.
- Both `feed.js` and `map.js` mount the identical chip from the one builder, at the insertion point 02-05's artifact table reserved (between the meta/description and the vote block) — proven structurally by `TestBothSurfacesMountTheSameVisibilityTag`.
- The "Show disputed reports" toggle: one boolean on the shared store (`Pinalert.state.showDisputed` / `Pinalert.setShowDisputed`), one conditional `&show_disputed=true` on the one `GET /api/reports` call, and a `#disputed-empty` empty state that replaces the general one when the filter returns nothing.
- Eight new contract tests in `web/votes_contract_test.go`, all passing, extending 02-05's file and reusing its helpers.
- Fixed a latent pre-existing bug (see Deviations) that would have silently broken any future contract test inspecting code near the end of `feed.js`.

## Task Commits

Each task was committed atomically:

1. **Task 1: One module that knows what a visibility state looks like** - `0e3a9ff` (feat)
2. **Task 2: Both surfaces wear the same treatment — Provisional becomes legible** - `23d0265` (feat)
3. **Task 3: One toggle, one query parameter, two surfaces** - `2770789` (feat)

_No TDD RED/GREEN/REFACTOR split: each task's contract tests were written first (confirmed failing against the absent module/rules/wiring), then made to pass, then committed as one atomic `feat` commit per task per this plan's own execution instructions._

## Files Created/Modified

- `web/static/js/visibility.js` — new. `window.PinalertVisibility`'s six public entries (see below), validated fallback, no markup-parsing sink.
- `web/static/js/feed.js` — `createRow` builds/appends the chip before the vote block, returns it under `tag`; `updateRow` applies the state class and updates the chip; `setEmptyState` is the sole owner of both empty-state elements; a `change` listener on the toggle sets the store flag and refetches; SCOPE comment gained TRUST-02; a latent comment bug fixed (see Deviations).
- `web/static/js/map.js` — `buildBadgeElement` applies the state class to the badge itself; `buildPopupContent` builds, updates and appends the chip before the vote block; SECURITY comment extended by one sentence.
- `web/static/js/app.js` — `state.showDisputed` (default `false`), `setShowDisputed` exported beside `setCenter`, one conditional `show_disputed=true` on `fetchReports`'s URL. `fetch(` count unchanged at 2.
- `web/static/css/trust.css` — `.vis-provisional`, `.vis-hidden`, `.icon-badge.vis-hidden, .vis-hidden .icon-badge`, `.visibility-tag:not([hidden])`, `#disputed-empty[hidden]`, `.filter-toggle`.
- `web/templates/index.html.tmpl` — one deferred `<script>` for `visibility.js` between `votes.js` and `map.js`; the `.filter-toggle` label above `#report-list`; the `#disputed-empty` element after `#feed-empty`.
- `web/template_contract_test.go` — `appModules` gained `"/static/js/visibility.js"`.
- `web/votes_contract_test.go` — eight new tests (listed below).

## Decisions Made

See `key-decisions` in frontmatter. The most consequential: the Provisional CSS value is asserted string-equal to main.css's own `.age-aging` value at test time (read live, not copy-pasted into the test as a second hardcoded literal), so the two genuinely cannot drift apart even if `.age-aging`'s value is ever tuned.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a latent stripCSSComments-truncation bug in a pre-existing `feed.js` comment**
- **Found during:** Task 3, while writing `TestFeedExplainsAnEmptyDisputedResult`
- **Issue:** A Phase-1-era comment in `feed.js` (`createToggleIcon`'s doc comment) contained the literal text `/static/icons/*.svg`, whose `icons/` + `*.svg` juxtaposition forms the two-character sequence `/*`. This package's shared `stripCSSComments` test helper (used by many contract tests across `web/*_test.go` to strip block comments before parsing JS/CSS text) treats any `/*` as a block-comment opener and, finding no matching `*/` later in the file, silently drops everything from that point to end-of-file. No prior test happened to statically assert on code appended after that line via `stripCSSComments`, so the bug was invisible until this task's new wiring (the toggle's `change` listener, appended near the end of `feed.js`) needed to be inspected the same way.
- **Fix:** Reworded the comment to avoid the accidental `/*` sequence (`"adding new SVG files under /static/icons (e.g. new-icon.svg)"`). No behavioural change — comment text only.
- **Files modified:** `web/static/js/feed.js`
- **Verification:** `TestFeedExplainsAnEmptyDisputedResult` passes; full `go test ./web/ -count=1 -v` green; `git diff` confirms the change is comment-only.
- **Committed in:** `2770789` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 pre-existing bug, Rule 1).
**Impact on plan:** No scope creep — this was a correctness fix to a shared test-infrastructure blind spot that this plan's own new tests were the first to trip. Worth flagging for future JS authors in this codebase: avoid writing `/*` (even accidentally, e.g. via a glob pattern in prose) inside a `//`-style line comment in any file this package's contract tests inspect with `stripCSSComments`.

## Issues Encountered

None beyond the deviation above. One self-caught authoring mistake during Task 1 (a new `trust.css` header comment contained `.sev-*/.age-*`, which likewise formed an accidental `/*` sequence and truncated `trust.css` from `TestVisibilityCascadeOverridesAgeRamp`'s point of view) was found and fixed before that task's commit, so it never shipped and is not listed as a deviation — flagging it here only because the same failure mode bit twice in one plan and future authors in this codebase should watch for it in both CSS and JS comments.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

**For 02-07, reading this file instead of the source per this plan's `<output>` instructions:**

### `window.PinalertVisibility` — final export list (as shipped)

| Entry | Signature | Notes |
|-------|-----------|-------|
| `VISIBILITY_STATES` | `['live','provisional','hidden','retracted']` | The allowlist every server-supplied `visibility` value is validated against |
| `VISIBILITY_TAG_LABELS` | object keyed by state | `live`→`''` (no chip), `provisional`→`'Unconfirmed'`, `hidden`→`'Disputed'`, `retracted`→`'Resolved'` — Copywriting Contract verbatim. **02-07 renders the `retracted` entry on the Activity page from this exact map — do not declare a second one.** |
| `visibilityState` | `(report) -> string` | Validated state slug; returns `'provisional'` for anything unrecognised/missing/null — **never** `'live'` |
| `applyVisibilityClass` | `(el, report) -> void` | Replaces any `vis-`-prefixed class on `el` with `vis-<state>`. Caller supplies the element (feed.js: the row; map.js: the badge itself) |
| `createVisibilityTag` | `() -> HTMLSpanElement` | `span.visibility-tag`, ships `hidden`. Applies no state — `updateVisibilityTag` is always called immediately after |
| `updateVisibilityTag` | `(tag, report) -> void` | Sets text via `Pinalert.setText`, the `visibility-tag--<state>` modifier class, `data-visibility-reason` (validated, omitted when unrecognised); clears `hidden`. For the live state (empty label) it clears all three and re-sets `hidden`. Idempotent — safe to call on every poll. Guards `tag` for null. |

Internal, **not** exported (free to change): `VISIBILITY_REASONS` (the five closed reason slugs: `resolved`, `critical_bypasses_gates`, `disputed`, `awaiting_second_independent_confirmation`, `confirmed`), `visibilityReason`, `replacePrefixedClass`.

### DOM shape as shipped

```html
<span class="visibility-tag visibility-tag--provisional" data-visibility-reason="…">Unconfirmed</span>
```

- **Live report:** the `<span class="visibility-tag">` element exists on the row/popup but carries `hidden`, empty text, no modifier class, no reason attribute. No "Confirmed" chip.
- **Modifier class:** `visibility-tag--provisional` / `visibility-tag--hidden` / `visibility-tag--retracted`. **Declared but unstyled by this plan** — `trust.css` styles the row/badge, not the chip's own modifier. 02-07 may add a rule targeting this class without touching the JS.
- **Reason hook:** `data-visibility-reason`, one of the five slugs above, omitted (not present) when unrecognised or empty. No prose is rendered from it anywhere in this plan.

### Insertion points relative to 02-05's block (`block.controls`, `block.error`)

- **Feed row** (`feed.js`'s `createRow`): the tag is appended to `.report-row__body` immediately after `.report-row__meta` and **before** `block.controls`. Held on the row handle under the key `tag`: `{ el, glyph, title, meta, votes, tag }` (`votes` is 02-05's `block` object; `tag` is this plan's chip element).
- **Map popup** (`map.js`'s `buildPopupContent`): the tag is built **and** updated in one call (unlike the feed row), then appended after `.map-popup__description` and before `block.controls` — because `upsertMarker` rebuilds popup content on every render rather than updating it in place.
- **State class placement:** `feed.js` puts `vis-<state>` on the **row** element (`row.el`), inheriting down to its badge. `map.js` puts it on the **badge element itself** (there being no row ancestor on that surface). This asymmetry is main.css's own documented age-ramp split, not an inconsistency — see main.css's `.icon-badge` comment.

### `trust.css` selector list as shipped

| Selector | Declares | [hidden] guard form |
|----------|----------|----------------------|
| `.vis-provisional` | `--severity-current` (= `.age-aging`'s value, byte-for-byte), `--badge-glyph-fg` | n/a |
| `.vis-hidden` | `--severity-current`, `--severity-tint: transparent`, `--badge-glyph-fg` | n/a |
| `.icon-badge.vis-hidden, .vis-hidden .icon-badge` | `background: transparent`, `outline: 1px solid var(--color-border)` | n/a |
| `.visibility-tag:not([hidden])` | chip layout/typography | **`:not([hidden])`** — this file owns the rule that sets `display` |
| `#disputed-empty[hidden]` | `display: none` | **`#id[hidden]` override** — `display: flex` on this element comes from main.css's shared `.empty-state` rule, which this file does not own, so `:not([hidden])` would not be in the cascade path |
| `.filter-toggle` | toggle layout | none — never `hidden`-toggled |

**No modifier-class rules** (`.visibility-tag--provisional` etc.) and **no `.vis-live` rule** are shipped — nothing to declare for either, per 02-05's convention of omitting empty rules.

**Custom-property allowlist:** exactly `--severity-current`, `--severity-tint`, `--badge-glyph-fg` — all three are main.css's own, re-declared (never invented). `TestVisibilityCascadeOverridesAgeRamp` enforces this and supersedes 02-05's cruder `grep -cE '^\s*--[a-z-]+:' = 0` gate.

**Reminder for 02-07 (02-05 flagged one gap, there are two):** `web/templates/profile.html.tmpl` loads **neither** `trust.css` **nor** `visibility.js`. 02-07's Activity page needs both — without the stylesheet its Reopen button and any chip ship unstyled, and without the script `window.PinalertVisibility` is undefined and the "Resolved" chip is never built. Add both `<link>`/`<script>` tags there, each with `?v={{.AssetVersion}}`.

### `Pinalert.setShowDisputed` / `state.showDisputed`

- `Pinalert.state.showDisputed`: boolean, default `false` (D-10's closed default).
- `Pinalert.setShowDisputed(value)`: coerces to boolean, assigns `state.showDisputed`. No notify, no fetch — mirrors `setCenter`'s shape exactly; the caller decides when to refetch.
- `Pinalert.fetchReports()` appends `&show_disputed=true` to its one URL only when the flag is `true`; omits it entirely otherwise (byte-identical to Phase 1's URL when unset).
- 02-07's Activity page reads no feed and has no reason to touch this flag, but it exists on the shared store if a future surface needs to reason about which reports the main feed is currently showing.

### Hidden pin findability in the human check

**Not yet run.** Task 3's `<human-check>` is deferred to end-of-phase UAT per `workflow.human_verify_mode: end-of-phase` — it runs during `/gsd-verify-work 2`, not during this plan's own execution, and this plan correctly stayed fully autonomous through all three tasks. Step 6 of that check specifically asks whether the Hidden pin (transparent fill + thin outline ring) stays findable against the basemap, with instructions to record a screenshot as a UI-SPEC follow-up (not a code defect) if it is not. **02-07 and the phase verifier should check `02-UAT.md` for this outcome before assuming the transparent-fill treatment is confirmed legible.**

### Deviations affecting the cascade's static-vs-runtime gap

None beyond what's listed above. All eight new contract tests' "honest limits" paragraphs are explicit that static inspection proves selector/value/call-order correctness but cannot prove the rendered cascade resolves as intended in a real browser — that class of defect is exactly what Task 3's `<human-check>` exists to catch, and it has not run yet.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Plan: 06*
*Completed: 2026-09-15*

## Self-Check: PASSED

- FOUND: `web/static/js/visibility.js` (created file exists on disk)
- FOUND: `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-06-SUMMARY.md`
- FOUND: `0e3a9ff` (Task 1 commit) in `git log`
- FOUND: `23d0265` (Task 2 commit) in `git log`
- FOUND: `2770789` (Task 3 commit) in `git log`
- Re-ran all plan-level `<verification>` commands: `gofmt -l web/` clean, `go build ./...` and `go vet ./...` exit 0, `go test ./web/ -count=1 -v` all green (26 tests including this plan's 8 new ones), `go test ./... -short` exits 0, `go test ./... -p 1` with `DATABASE_URL` set exits 0, `git status --porcelain` clean, `votes.js` and `js_contract_test.go` confirmed unmodified via `git diff --stat` against base.
