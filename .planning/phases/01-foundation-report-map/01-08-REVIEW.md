---
phase: 01-foundation-report-map
reviewed: 2026-09-06T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - web/static/css/main.css
  - web/static/css/modal.css
  - web/css_contract_test.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
  fixed: 1
status: issues_found_and_fixed
---

# Phase 01 Plan 08: Code Review Report

**Reviewed:** 2026-09-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

This is a narrow, well-scoped gap-closure plan and it lands correctly on its two stated goals.
The `.modal-backdrop:not([hidden])` guard is present, matches the established pattern already
used elsewhere in this codebase (`modal.css:199`, `modal.css:282`), and I confirmed
`TestModalBackdropHiddenGuard` actually bites: temporarily stripping the guard from `main.css`
and re-running the test produces the expected two failures, then restoring the file leaves the
tree clean (`git status`/`git diff` verified empty afterward). The comment-stripping /
last-`{`-split parser in `web/css_contract_test.go` is correct for every rule shape present in
this codebase's three stylesheets, including rules nested inside the `@media
(prefers-reduced-motion: reduce)` block. `go build`, `go vet`, and the full `go test ./...` suite
all pass. The severity-slider fill math (the `--touch-target-min / 2 + ratio * (100% -
--touch-target-min)` expression) is dimensionally valid CSS `calc()` and I hand-verified it
produces the correct thumb-aligned edge at all three ratios (0, 0.5, 1). The
`prefers-reduced-motion` fix genuinely resolves the invalid-selector-list defect described in the
plan: each vendor-prefixed pseudo-element now gets its own rule, so the override is live CSS in
both engines instead of dead code.

One real, provable defect survived every automated gate because the plan itself specified the
value that causes it (see WR-01) — automated gates that check "is `--severity-fill-ratio` present
at all" cannot catch "is its *fallback value* correct." No security, data-loss, or accessibility
regressions were found; the modal's native-range-input contract, the byte-identical
`modal.js`/`index.html.tmpl` constraint, and the no-raw-hex rule in `modal.css` are all honored.

## Warnings

### WR-01: `--severity-fill-ratio` fallback (0.5) is inconsistent with the adjacent `--severity-current` fallback (sev-low) in the same declaration

**File:** `web/static/css/main.css:543-552` and `web/static/css/main.css:559-570`

**Issue:** Both track rules compute the fill's colour and the fill's width from the same two
custom properties, but give them fallbacks that describe two different severity values:

```css
background-image: linear-gradient(
  to right,
  var(--severity-current, var(--color-severity-low)),   /* falls back to LOW (value 1) */
  var(--severity-current, var(--color-severity-low))
);
...
background-size:
  calc(
    var(--touch-target-min) / 2 + var(--severity-fill-ratio, 0.5) *   /* falls back to MEDIUM (value 2) */
      (100% - var(--touch-target-min))
  )
  100%;
```

`--severity-current` falls back to `--color-severity-low` (severity 1), matching the template's
actual default (`web/templates/index.html.tmpl:61`, `value="1"`, and `modal.js` calls
`buildSeverityControl()` + `updateSeverityReadout()` synchronously at init, which sets `sev-low`
on the wrapper before the modal is ever shown — so on the happy path this fallback never actually
renders). But `--severity-fill-ratio` falls back to `0.5`, which is the *medium* (value 2)
position, not low. If `--severity-fill-ratio` is ever unset while `--severity-current` still
resolves to low-severity green (e.g. JS throws before `buildSeverityControl()` completes, or a
future refactor adds another consumer of `.severity-slider` without the `.severity-control.sev-*`
wrapper), the rendered control would show a green (low) coloured fill sitting at the 50% (medium)
position — reproducing exactly the thumb/fill misalignment bug this whole `calc()` expression was
written to prevent. This is a plan-origin defect: Task 2 item 3 of `01-08-PLAN.md` specifies the
`0.5` fallback verbatim, so no automated gate (which only checks for the *presence* of
`--severity-fill-ratio`, not its fallback value) could have caught it.

**Fix:** Change the fallback in both `background-size` declarations from `0.5` to `0`, so it
matches `--severity-current`'s low-severity fallback:

```css
background-size:
  calc(
    var(--touch-target-min) / 2 + var(--severity-fill-ratio, 0) *
      (100% - var(--touch-target-min))
  )
  100%;
```

**Outcome: fixed.** Applied verbatim in both `background-size` declarations
(`main.css` commit `607f6fd`), verified with `go build && go vet && go test ./...`
(full suite green) after the change.

### WR-02: WebKit thumb vertical-centering assumption is unverified in a live browser

**File:** `web/static/css/main.css:573-590`

**Issue:** `margin-top: calc((var(--space-sm) - var(--touch-target-min)) / 2)` (i.e. `-18px`)
assumes WebKit vertically centres the 8px `::-webkit-slider-runnable-track` inside the input's
~44px content box, so shifting the 44px thumb up by 18px re-centres it on the track. This is the
canonical fix pattern for this exact WebKit behaviour and is very likely correct, but it depends
on the host `<input>`'s *rendered* height actually resolving to 44px (via `min-height`, since no
explicit `height` is set) and on WebKit's specific centring behaviour for the runnable-track,
neither of which is exercised by any automated test in this plan. The plan's own `01-08-SUMMARY.md`
(line 128) already flags this as an open item for the `/gsd-verify-work 01` Test 3 re-run — this
entry is not a new finding, it's flagging that the risk is real and still open, not yet closed by
a live-browser check.

**Fix:** No code change proposed here; confirm in Chrome/Edge (and any other WebKit-family target)
during the Test 3 re-run that the thumb sits centred on the track rather than riding above or
below it, per the plan's own human-check item.

## Info

### IN-01: `background-color` transition on the track pseudo-elements never actually fires

**File:** `web/static/css/modal.css:113-123`

**Issue:** Both `::-webkit-slider-runnable-track` and `::-moz-range-track` list `background-color
150ms ease` as a transitioned property, but `background-color` on these rules is always
`var(--color-border)` (main.css:540, :558) — it is not a severity-dependent value and never
changes as a result of any user interaction the control supports. The only thing that changes
`--color-border` is a light/dark theme switch via `prefers-color-scheme`/`data-theme`, which isn't
gated behind a user gesture this transition would meaningfully soften. The declaration is
harmless but doesn't describe anything that actually animates in practice.

**Fix:** Either remove `background-color` from the transitioned-properties list (only
`background-size` is doing real work here), or leave it as defensive/future-proofing if a
theme-toggle animation is anticipated later — but note the "which is what makes the fill genuinely
slide" comment in `modal.css:105-108` should then clarify that `background-color` is not part of
that story.

### IN-02: `TestModalBackdropHiddenGuard`'s guard match is an exact substring, not a parsed selector

**File:** `web/css_contract_test.go:87`

**Issue:** `strings.Contains(selectorHead, guard)` requires the literal text `:not([hidden])`
(no internal whitespace) to appear anywhere in the selector head. Two brittleness points, both
currently harmless given the codebase's actual style but worth knowing about:
1. A cosmetic reformat such as `:not( [hidden] )` (extra whitespace, functionally identical CSS)
   would make this test fail even though the guard is semantically present.
2. Because the check is selector-head-scoped rather than per-comma-separated-selector, a
   hypothetical `.something:not([hidden]), .modal-backdrop { display: flex; }` would be treated as
   "guarded" even though the guard only qualifies `.something`, not `.modal-backdrop`. No such
   selector list exists in the codebase today.

**Fix:** No change required now — this matches the algorithm the plan specified verbatim, and
generalizing the parser was explicitly out of scope for this task. Flagging for awareness if this
test is ever extended to cover more selectors.

---

_Reviewed: 2026-09-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
