---
phase: 01-foundation-report-map
reviewed: 2026-09-06T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - web/static/css/main.css
  - web/css_contract_test.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
  fixed: 1
  false_positive: 1
status: issues_found_and_fixed
---

# Phase 01 Plan 09: Code Review Report

**Reviewed:** 2026-09-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

This is a narrow, correctly-scoped CSS fix and it lands on its stated goal: `#map` now has a
self-resolving `height: 100vh; height: 100dvh;` rule, `.app-shell`'s `min-height` is paired into
the same unit family, and `TestPrimaryMapHasResolvedHeight` exists and passes (`go test ./web/...
-run 'TestPrimaryMapHasResolvedHeight|TestModalBackdropHiddenGuard' -v` — both green). `go build
./...` and `go vet ./...` are clean. `git show 45ab1bb` confirms the diff touches exactly the two
files the plan authorized (31 lines added to `main.css`, 141 to `css_contract_test.go`); `modal.css`,
`feed.css`, every JS file and the template are untouched, matching the SUMMARY's claim.

I traced the three specific risk areas this review was asked to focus on:

1. **`100vh`/`100dvh` declaration-order correctness.** Correct on both `.app-shell` and `#map`.
   An engine that doesn't parse the `dvh` unit treats `height: 100dvh` as an invalid value and
   drops that whole declaration during parsing (per CSS's error-recovery rule for unsupported
   values), leaving the preceding `100vh` declaration as the computed value; an engine that does
   support `dvh` applies the later declaration per normal same-specificity cascade order. Both
   boxes use the identical two-line pattern, so neither can drift out of sync with the other.

2. **Whether full-bleed `#map` breaks the `>=900px` split view.** It does not, and the DOM
   confirms the reasoning the plan itself lays out: `index.html.tmpl:21-26` places `#map` and the
   fixed-position `.fab` as the only children of `.pane--map`, and `.pane--map` is an implicit-row
   grid item of `.app-shell` (`display: grid`, no `grid-template-rows`, so the row uses the
   default `auto` track-sizing algorithm). Because `#map`'s height is now a definite, self-
   resolving value (not `auto`), it is what gives `.pane--map`'s own auto-computed content height
   a non-zero value in the first place — which is exactly what makes that pane's contribution to
   the grid row's `auto` track-sizing algorithm non-zero, and by extension what makes the row (and
   `.pane--map`, stretched by the grid's default `align-items: stretch`) resolve to ~`100dvh` at
   `>=900px`, matching `#map`'s own height with no clipping against `.pane`'s `overflow: hidden`.
   The only visible side effect — `.pane--list` being stretched taller than its own content when
   the list is short, leaving empty space below the last row — is cosmetically new (the map was
   previously an invisible 0px box, so the row was never forced this tall before), but it is the
   *intended* D-06 outcome ("map fills its half"), not a regression, and it's exactly what the
   plan's own human-check step 1 exists to visually confirm. I did not find a case where `#map`'s
   content is clipped or where the split view breaks.

3. **The new test's selector-matching logic.** Traced by hand against all three shipped
   stylesheets: `#map` matches (`fields[-1] == "#map"`), `#modal-map` correctly does not (the
   stated goal), and a hypothetical descendant form like `.pane--map #map` would still match. No
   false positive or false negative exists against the CSS actually shipped today. Two structural
   gaps in the test's robustness are noted below as WR-01 and IN-01 — neither is exploitable
   against the current file set, but both are worth fixing or at least tracking given this test's
   stated job is to be a durable regression gate.

No security issues, no hardcoded secrets, no dangerous functions, no debug artifacts, and no dead
code were found in either file.

## Warnings

### WR-01: `TestPrimaryMapHasResolvedHeight`'s self-resolving-value assertion is not scoped to the same file as its rule-existence assertion

**File:** `web/css_contract_test.go:135-136, 188-221, 239-249`

**Issue:** The test declares two accumulators before the `fs.WalkDir` loop:

```go
var matchingRulesInMainCSS int
var selfResolvingValueFound bool
```

`matchingRulesInMainCSS` is correctly incremented only `if path == "static/css/main.css"`, so
assertion (A) — "at least one matching rule was found in `main.css`" — is properly scoped.
`selfResolvingValueFound`, however, is set from **any** matching rule in **any** walked `.css`
file, with no `path == "static/css/main.css"` guard:

```go
for _, decl := range strings.Split(declBody, ";") {
    ...
    if strings.Contains(value, "vh") || strings.HasSuffix(value, "px") || ... {
        selfResolvingValueFound = true   // not scoped to main.css
    }
}
```

Today this is harmless only because `#map` (matched by the last-field selector check) happens to
appear in exactly one shipped stylesheet (`main.css`) — confirmed by grep and by this file's own
`fs.WalkDir` over `static/css/*.css` (`main.css`, `modal.css`, `feed.css`). But the test's own
stated job (per its doc comment) is to be "a durable regression gate" against `main.css`
specifically regressing to a non-self-resolving value (e.g. someone changes `#map`'s height to a
bare percentage while leaving the comment and the rule shape intact). As written, assertion (B)
would still pass in that scenario if *any other* shipped stylesheet ever gained an unrelated rule
whose last-field selector happens to equal `#map` with a `vh`/px value (e.g. a copy-paste error
introducing a second `#map` block in a new stylesheet) — the regression in `main.css` would go
undetected because the global accumulator was already satisfied by the other file's rule.

**Fix:** Scope `selfResolvingValueFound` the same way `matchingRulesInMainCSS` already is —
either track it per-file and only honor `main.css`'s value, or (simplest) just gate the inner
`if` on `path == "static/css/main.css"` the same way the counter increment already is:

```go
if path == "static/css/main.css" {
    matchingRulesInMainCSS++
    for _, decl := range strings.Split(declBody, ";") {
        ...
        if strings.Contains(value, "vh") || strings.HasSuffix(value, "px") || ... {
            selfResolvingValueFound = true
        }
    }
}
```

**Outcome: fixed.** Applied as a `continue`-guarded scope check (equivalent to the
`if`-wrap above) rather than re-indenting the whole block. Confirmed the test still
bites: temporarily removed `#map`'s rule from `main.css`, watched
`TestPrimaryMapHasResolvedHeight` fail with the expected message, restored the file
(byte-identical via `git diff`), then reran green. Full `go build && go vet && go test
./...` passes.

## Info

### IN-01: Last-whitespace-field selector matching is fragile to compound selectors and whitespace-less combinators

**File:** `web/css_contract_test.go:173-183`

**Issue:** The match condition is exact string equality against the selector's last
whitespace-separated field:

```go
fields := strings.Fields(sel)
if fields[len(fields)-1] == primaryMapIDSelector {   // primaryMapIDSelector == "#map"
```

This correctly handles the two forms the plan calls out (`#map` alone, and a descendant form like
`.pane--map #map`), and correctly excludes `#modal-map`. It does not recognize:
- A compound selector with no separating space, e.g. `div#map` or `#map.some-class` — the whole
  token (`"div#map"`, `"#map.some-class"`) fails the `== "#map"` equality check.
- A combinator with no surrounding whitespace, e.g. `.pane--map>#map` or `.pane--map+#map` —
  `strings.Fields` only splits on whitespace, so this stays one token.

Neither form exists in the codebase today, so there is no live false negative. The direction of
risk is fail-safe rather than fail-open: if a future refactor happened to write the rule in one of
these forms while otherwise preserving a correct height value, this test would report "no rule
targeting the primary map container" and break the build on a false alarm, not silently pass a
real regression. Separately, the unit check (`strings.Contains(value, "vh")`) is
case-sensitive — a value like `100VH` (unusual but legal CSS) would not be recognized as
self-resolving. Both are minor robustness gaps in a test whose parsing technique intentionally
mirrors `TestModalBackdropHiddenGuard`'s existing (and similarly noted) substring-based approach.

**Fix:** No change required now — this matches the matching strategy the plan specified verbatim,
and a fuller CSS selector/value parser is explicitly out of scope for this test file. Worth
tracking if this test is ever extended to cover additional selectors or if the codebase starts
using compound/combinator forms for `#map`.

### IN-02 (retracted — false positive): claimed stray duplicate closing tag in `01-09-PLAN.md`

**File:** `.planning/phases/01-foundation-report-map/01-09-PLAN.md`

This finding claimed a duplicate `</output>` at a line 345. Verified directly against
both the working tree and the committed version (`git show HEAD:...01-09-PLAN.md`):
the file is 344 lines (`awk 'END{print NR}'`), ends with exactly one `<output>`/`</output>`
pair, and a hex dump of the trailing bytes confirms a single `</output>` followed by one
trailing newline — no line 345 exists, duplicate or otherwise. The two-tag contamination
(`</content>`/`</invoke>`) this file *did* have was fully removed in commit `ef33eef`,
before the plan-checker or executor ever saw it. No action taken; this entry is kept
(rather than deleted) so the false positive is on record instead of silently vanishing.

---

_Reviewed: 2026-09-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
