---
phase: 01-foundation-report-map
reviewed: 2026-09-09T00:00:00Z
depth: standard
files_reviewed: 53
files_reviewed_list:
  - .claude/CLAUDE.md
  - .github/workflows/ci.yml
  - .planning/phases/01-foundation-report-map/01-UI-SPEC.md
  - .planning/phases/01-foundation-report-map/deferred-items.md
  - cmd/migrate/main.go
  - cmd/server/main.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - internal/api/handlers/page.go
  - internal/api/handlers/page_test.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/reports_e2e_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/router.go
  - internal/service/report.go
  - internal/service/report_test.go
  - internal/session/cookie.go
  - internal/session/cookie_test.go
  - internal/store/db.go
  - internal/store/migrations.go
  - internal/store/migrations/00001_create_reports.sql
  - internal/store/migrations_test.go
  - internal/store/queries/reports.sql
  - internal/store/queries/sessions.sql
  - internal/store/reports_test.go
  - internal/store/sqlc/db.go
  - internal/store/sqlc/models.go
  - internal/store/sqlc/reports.sql.go
  - internal/store/sqlc/sessions.sql.go
  - internal/testutil/db.go
  - internal/testutil/db_test.go
  - internal/testutil/seed.go
  - web/css_contract_test.go
  - web/embed.go
  - web/js_contract_test.go
  - web/static/css/feed.css
  - web/static/css/main.css
  - web/static/css/modal.css
  - web/static/icons/earthquake.svg
  - web/static/icons/fire.svg
  - web/static/icons/flood.svg
  - web/static/icons/other.svg
  - web/static/icons/power_outage.svg
  - web/static/icons/rescue_needed.svg
  - web/static/icons/road_blocked.svg
  - web/static/icons/shelter_open.svg
  - web/static/icons/storm_cyclone.svg
  - web/static/js/app.js
  - web/static/js/feed.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/template_contract_test.go
  - web/templates/index.html.tmpl
findings:
  critical: 0
  warning: 5
  info: 2
  total: 7
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-09T00:00:00Z
**Depth:** standard
**Files Reviewed:** 53
**Status:** issues_found

## Summary

This review supersedes the prior 01-REVIEW.md (2026-09-09, 0 Critical / 4 Warning / 2 Info) for
the same 53 files, following plan 01-14's second gap-closure round: a uniform `stroke-width`
bump (2→3) across all nine category SVGs, a new `--badge-glyph-fg` age-aware foreground custom
property in `main.css`, removal of the dead `.icon-badge svg` selector (closing the prior
review's WR-01), a darkened light-mode `--color-severity-medium` hex, two new computed Go
contract tests (`TestCategoryGlyphInkCoverageAcrossRenderContexts`,
`TestBadgeGlyphContrastAcrossAgeStagesAndThemes`), and the `01-UI-SPEC.md` color-table sync.

**Scrutiny tiering:** the 01-14 file set (both changed CSS/test files, the 9 SVGs, and — since
their DOM-shape claims about where `.age-*`/`.sev-*` classes land are load-bearing for the new
CSS mechanism — `map.js`/`feed.js`) got a full line-by-line read plus independent verification:
the two new Go tests were run (`go test ./web/...`, all pass), their math was hand-traced against
the shipped tokens/SVGs, and a counter-scenario (pre-01-14 stroke-width=2, pre-01-14 medium hex)
was computed by hand to confirm both new tests would genuinely have failed against the old
values — they are not vacuous. The remaining 44 files (unchanged since the prior pass) got a
lighter pass: full reads on the security-sensitive Go path (`reports.go`, `report.go`,
`cookie.go`, `router.go`, `page.go`, `main.go`, `db.go`), a project-wide grep sweep for the
standard anti-pattern set (secrets, `eval`/`innerHTML`, debug artifacts, empty catches), and
`go build ./... && go vet ./... && go test ./...` (all clean) — consistent with the prior
review's clean assessment of that set. Per the prior review's own note, `internal/testutil/seed.go`'s
`storm_cyclone_damage` vs. canonical `storm_cyclone` mismatch remains tracked in
`deferred-items.md` and is not repeated here.

**WR-01 verified fixed.** `grep -n "svg\|img" web/static/css/*.css` (excluding `.svg"` mask-image
URLs) now returns only a prose comment mentioning `<img>`, and
`assertGlyphSizingAndNoOrphanedImageSelectors` now includes the trailing-`svg` check the prior
review's fix suggested. No orphaned `.icon-badge svg` / `.category-tile svg` selector survives.

**`--badge-glyph-fg` wiring verified correct across both DOM shapes.** `map.js:144` puts the age
class on the badge element itself (compound match — `.age-*` wins over `.sev-*` for
`--severity-current`/`--badge-glyph-fg` at equal specificity because it appears later in
`main.css`'s source order); `feed.js:168-169` puts both the `sev-*` and `age-*` classes on the
`<li class="report-row">` ancestor, and the custom properties reach the child `.icon-badge` via
ordinary CSS custom-property inheritance — the same pre-existing mechanism that already made
`--severity-current` (badge fill) work in both contexts before 01-14. `ruleBySelector`'s
exact-selector-head match (not a substring search) means a wrongly-shaped fix (a compound
`.icon-badge.age-stale` or descendant `.report-row.age-stale .icon-badge` selector) would leave
the bare `.age-stale` rule undeclared and the test would catch it. This mechanism is sound.

**No security regression found** in the SVGs or the new CSS: no `<script>`, no `javascript:`
URLs, no new markup-string assembly in `map.js`/`feed.js` (both still build the badge via
`document.createElement` + `.className`, never `innerHTML`), and every mask-image `url()` in
`main.css` is a static literal pointing at the existing nine `/static/icons/*.svg` paths — no
user input reaches any of this.

**Carried forward from the prior review:** WR-02, WR-03 (updated), WR-04, and IN-01 below concern
files/mechanisms plan 01-14 did not touch and remain valid against current source, verified by
re-reading the exact lines cited. They are restated here (not just referenced) because this
review overwrites the prior artifact and a future `--auto` re-review pass reads only this file.

## Warnings

### WR-01 (closed): dead `.icon-badge svg` selector

Fixed by 01-14. No longer present; the new orphan-selector check in
`assertGlyphSizingAndNoOrphanedImageSelectors` now catches this shape if it recurs. Kept as a
closed entry (not renumbered) so this document's WR-02 through WR-04 numbering stays stable
across review passes.

### WR-02: Per-category `-webkit-mask-image` declarations are still asymmetrically unguarded

**File:** `web/static/css/main.css:302-345`, `web/css_contract_test.go:363-395` (`collectGlyphMaskRules`)
**Issue:** `assertBaseGlyphRule` enforces that the shared `.icon-glyph` base rule declares
`mask-size`/`mask-repeat`/`mask-position` in **both** prefixed and unprefixed form. But
`collectGlyphMaskRules`, which walks the nine per-category `.icon-glyph--{category}` rules (the
ones that resolve the actual icon path), still only inspects the unprefixed spelling:

```go
if strings.TrimSpace(name) == "mask-image" {
    maskSource = extractMaskURLPath(strings.TrimSpace(value))
}
```

Confirmed unaddressed by 01-14: `grep -n "webkit-mask-image" web/css_contract_test.go` matches
only a comment, never an assertion. Deleting all nine `-webkit-mask-image: url(...)` lines from
`main.css` would leave every test in the package green while older WebKit-based engines that only
honor the prefixed form render all nine category glyphs invisible — the one declaration that
actually resolves the icon path is the one place the codebase's "assert both forms" discipline
was not extended to.
**Fix:** Extend `collectGlyphMaskRules` (or add a companion assertion) to also require a
`-webkit-mask-image` declaration resolving to the same path as the unprefixed `mask-image` for
every category rule, mirroring `assertBaseGlyphRule`'s both-forms enforcement.

### WR-03: `feed.css`'s scoped glyph-sizing rule is still a same-value duplicate — now also a test literal

**File:** `web/static/css/feed.css:56-59`
**Issue:**

```css
.report-row .icon-badge .icon-glyph {
  width: 55%;
  height: 55%;
}
```

remains byte-identical in effect to `main.css`'s cascading `.icon-badge .icon-glyph { width: 55%;
height: 55%; }` — 55% of `.icon-badge--sm`'s own box resolves the same whether declared here or
inherited from main.css, so the rule still changes nothing at runtime; the prior review's finding
stands unchanged.

**Update since the prior review:** this is no longer purely cosmetic risk.
`TestCategoryGlyphInkCoverageAcrossRenderContexts` (new in 01-14) now reads this exact selector's
`width` value as a literal via `findDeclValue(t, "static/css/feed.css", ".report-row .icon-badge
.icon-glyph", "width")` and uses it as one leg of its stroke-width-floor computation. The prior
review's suggested fix ("delete the rule; main.css's shared rule already covers it") would now
also break this new test, which `t.Fatalf`s if the selector is missing. The underlying
no-op-duplicate observation is still correct, but any fix now needs to account for two test
dependents, not the original one.
**Fix:** Either (a) leave the rule in place with a comment stating plainly that it is currently a
byte-identical duplicate of main.css's cascade value, kept only because two contract tests read it
as a literal, so a future editor does not "clean it up" and silently break both, or (b) if this
selector is meant to diverge from main.css's 55% in the future, do so now and update both tests'
literals to match.

### WR-04: `TestIconGlyphsAreClassDriven`'s regression guard is still brittle to quote style

**File:** `web/js_contract_test.go:387-408`
**Issue:** Unchanged since the prior review — confirmed by re-reading the cited lines against
current source. The guard against the exact regression that caused UAT Tests 2/6 (glyphs
rendering solid black via a sealed `<img>` sub-document) is a literal substring match:

```go
const imageNodeCreation = "createElement('img')"
...
if strings.Contains(text, imageNodeCreation) {
```

A revert written as `createElement("img")`, `createElement( 'img' )`, or built via
`document.createElement.bind(document, 'img')` / a template literal passes this check cleanly
while reintroducing the exact defect the test exists to catch. No lint/format step enforces
single-quote consistency across this codebase's JS files that would otherwise make this moot.
**Fix:** Loosen the match to be whitespace/quote-tolerant (e.g. a small regex
`createElement\(\s*['"]img['"]\s*\)`), or assert the absence of any `<img`-tagged element ever
appended to a badge/tile container instead of matching the constructor call textually.

### WR-05 (new): `TestCategoryGlyphInkCoverageAcrossRenderContexts`'s per-context "physical stroke width" assertions are tautological

**File:** `web/css_contract_test.go:940-960`
**Issue:** The test's doc comment (lines 800-833) sells this as measuring "physical rendered
stroke width per context" against a floor — implying an assertion that could fail independently
per render-box context. Tracing the arithmetic shows it cannot:

```go
floorFeed := inkFloorStrokeWidth * feedRenderBoxPx / glyphViewBoxUnits
...
feedPhysical := sw * feedRenderBoxPx / glyphViewBoxUnits
...
if feedPhysical < floorFeed {
```

`feedRenderBoxPx` and `glyphViewBoxUnits` are identical, non-zero constants on both sides of the
comparison — they cancel algebraically, so `feedPhysical < floorFeed` reduces exactly to `sw <
inkFloorStrokeWidth`, the same condition already asserted directly and unconditionally at line
897 (`if uniform < inkFloorStrokeWidth || uniform > inkCeilingStrokeWidth`). The same cancellation
holds for `modalPhysical`/`floorModal`. Verified by hand-computation, not just inspection: for any
`sw`, `feedRenderBoxPx`, `modalRenderBoxPx` combination, `feedPhysical < floorFeed` and
`modalPhysical < floorModal` are true if and only if `sw < 2.5`, regardless of the render-box
values — the per-context loop can never fail independently of the single uniform-stroke-width
check that already runs once, before it. The render-box literals (`feedPct`, `smPx`, `modalPx`)
are real and are genuinely guarded — but only by their own three separate monotonicity checks
(`feedPct < 55.0`, `smPx < 32.0`, `modalPx < 24.0`), not by the "physical stroke width per
context" comparisons that the doc comment presents as the test's core claim.
**Fix:** Either drop the per-icon `feedPhysical`/`modalPhysical` `t.Errorf` checks (keep the
`t.Logf` for visibility) and adjust the doc comment to state plainly that render-box literals are
guarded by monotonicity only, not by an independent per-context floor; or make the comparison
genuinely context-dependent (e.g. derive `inkFloorStrokeWidth` as a floor on physical pixels
rather than on the SVG's `stroke-width` units, so a context with a smaller render box has a
tighter — not merely proportionally identical — bound).

## Info

### IN-01: Icon SVGs still carry vestigial `fill="none" stroke="currentColor"` attributes

**File:** `web/static/icons/*.svg` (all nine files, e.g. `flood.svg:8-9`)
**Issue:** Unchanged since the prior review (01-14 only edited `stroke-width`, from `2` to `3`).
Every icon still declares `fill="none"`/`stroke="currentColor"` from its pre-mask-consumption
history. Since every icon is consumed exclusively as a `mask-image` source, only the rasterized
alpha channel matters; `stroke="currentColor"` never resolves against any real page color.
Functionally harmless, but a future editor skimming these files could reasonably believe
`currentColor` is still doing something.
**Fix:** Optional — strip `fill`/`stroke` down to whatever minimally produces the intended
silhouette, or add a one-line comment noting these attributes are vestigial under mask
consumption.

### IN-02 (new): `road_blocked.svg` is the shape most at risk of stroke-merge at the smallest render context — flag for the human-check step

**File:** `web/static/icons/road_blocked.svg`, `.planning/phases/01-foundation-report-map/01-UI-SPEC.md`
**Issue:** 01-14's fix raises ink coverage but both new contract tests explicitly and correctly
disclaim proving shape identifiability (see each test's own "Honest limit" doc comment) — that is
deferred to 01-14 Task 3's human visual check. Of the nine icons, `road_blocked.svg` (a `rect`
20×8 plus seven interior/diagonal paths, all now at `stroke-width="3"`) is the most likely
candidate for the interior strokes to visually merge at the feed row's smallest render context
(17.6px box, ~0.73 px per SVG unit — the rect's 8-unit height renders at ~5.9px with a ~2.2px
physical stroke top and bottom, leaving roughly 1.5px of interior height for three diagonal
strokes at the same physical weight). This is not a code defect the test suite can catch (both
tests correctly scope themselves to ink-coverage and contrast, not silhouette legibility) — it's
a specific, named risk the human-check step should verify was actually looked at, rather than a
generic "eyeball the 3x3 grid" pass that could plausibly skim past this one icon at the smallest
size.
**Fix:** None required in code. When 01-14 Task 3's human-check is performed, explicitly confirm
`road_blocked` reads as distinguishable from the other eight icons in the feed-row context (not
just the modal's 3x3 grid, which renders at the larger 24px size), not just that all nine icons
are visible.

---

_Reviewed: 2026-09-09T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
