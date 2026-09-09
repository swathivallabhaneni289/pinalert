---
phase: 01-foundation-report-map
reviewed: 2026-09-09T00:00:00Z
depth: standard
files_reviewed: 53
files_reviewed_list:
  - .claude/CLAUDE.md
  - .github/workflows/ci.yml
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
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-09T00:00:00Z
**Depth:** standard
**Files Reviewed:** 53
**Status:** issues_found

## Summary

This review supersedes the 2026-09-06 REVIEW.md for this phase. Per the workflow's scoping
instructions, the 9 files touched by plan 01-13 (gap closure — switching category-icon rendering
from `<img>`-loaded SVGs to CSS `mask`-based glyph spans) got proportionally deeper scrutiny:
`web/static/css/main.css`, `web/static/js/app.js`, `web/static/js/map.js`,
`web/static/js/modal.js`, `web/static/js/feed.js`, `web/css_contract_test.go`,
`web/js_contract_test.go`, and the two icon-consuming stylesheets `web/static/css/feed.css` /
`web/static/css/modal.css`. The remaining 44 files were re-read at standard depth; none produced
a new finding — they remain consistent with the prior review's clean assessment. The one
previously-logged item in that unchanged set (`internal/testutil/seed.go`'s
`storm_cyclone_damage` vs. the canonical `storm_cyclone` slug) is already tracked in
`.planning/phases/01-foundation-report-map/deferred-items.md` and is not repeated here as a new
finding.

**On the core question this gap-closure fix exists to answer — is the mask-based glyph rendering
XSS-safe:** yes. `Pinalert.iconClass` (app.js) validates `category` against the fixed
`CATEGORIES` array with `indexOf` (strict equality, no coercion) before building a class name,
falls back to `'other'` for anything unrecognised, and every one of the three renderers
(map.js/modal.js/feed.js) assigns the result only through `.className` / `el.className = ...` —
never through a markup-parsing sink. This is a **strict improvement** over the `<img src="...">`
approach it replaces: no path string is constructed from any category value at all, closing off
even a would-be path-traversal/host-confusion class of bug that an unchecked `src` build could
have opened. No injection, XSS, authz, or data-loss issue was found anywhere in this file set.

What follows are quality/robustness gaps in the mask migration itself and in the two new
CSS/JS contract tests that were written to guard it — several of them are literal instances of
the failure mode those tests' own doc comments warn against, which is why they're flagged as
Warnings rather than Info.

## Warnings

### WR-01: `.icon-badge svg` is dead CSS, and the new orphan-selector guard doesn't catch it

**File:** `web/static/css/main.css:239-243`
**Issue:** The sizing rule still carries both selectors:

```css
.icon-badge svg,
.icon-badge .icon-glyph {
  width: 55%;
  height: 55%;
}
```

Nothing in the current codebase places a bare `<svg>` element inside `.icon-badge` any more:
`map.js`'s `buildBadgeElement` and `feed.js`'s `createRow` both append a `span.icon-glyph`
child, and `feed.js`'s `createToggleIcon` builds an `<svg>` but appends it under
`#view-toggle`/`.view-toggle__icon`, unrelated to `.icon-badge`. `.icon-badge svg` is therefore
dead — exactly the pattern `web/css_contract_test.go`'s
`assertGlyphSizingAndNoOrphanedImageSelectors` doc comment calls out as the failure mode it
exists to catch ("an executor who appends the glyph selector to the existing image selector,
instead of replacing it, would still pass (a) while leaving dead CSS behind"). The guard's own
implementation only flags a trailing selector field of `img`:

```go
if len(fields) < 2 || fields[len(fields)-1] != "img" {
    continue
}
```

— so the `svg` variant walks straight through undetected.
**Fix:** Remove `.icon-badge svg,` from main.css (keep only `.icon-badge .icon-glyph`), and widen
the test's orphan check to also flag a trailing `svg` selector field alongside `img`, so a future
regression of the same shape is actually caught:

```go
if len(fields) < 2 || (fields[len(fields)-1] != "img" && fields[len(fields)-1] != "svg") {
    continue
}
```

### WR-02: Per-category `-webkit-mask-image` declarations are asymmetrically unguarded

**File:** `web/static/css/main.css:276-319`, `web/css_contract_test.go:340-402`
**Issue:** `assertBaseGlyphRule` (css_contract_test.go) enforces that the shared `.icon-glyph`
base rule declares `mask-size`, `mask-repeat`, and `mask-position` in **both** prefixed and
unprefixed form — the comment explains this cross-engine correctness requirement in detail.
But `collectGlyphMaskRules`, which walks the nine per-category `.icon-glyph--{category}` rules
(the ones that actually point at an icon file), only looks for the unprefixed spelling:

```go
if strings.TrimSpace(name) == "mask-image" {
    maskSource = extractMaskURLPath(strings.TrimSpace(value))
}
```

Deleting all nine `-webkit-mask-image: url(...)` lines from main.css leaves every test in the
package green, while older WebKit-based engines that only honour the prefixed form would render
all nine category glyphs invisible. This is the one declaration that actually resolves the icon
path, in the one place the test suite's "assert both forms" discipline — deliberately applied
to the base rule's three sibling mask properties — was not extended to.
**Fix:** Extend `collectGlyphMaskRules` (or add a companion assertion) to also require a
`-webkit-mask-image` declaration resolving to the same path as the unprefixed `mask-image` for
every category rule, mirroring `assertBaseGlyphRule`'s both-forms enforcement.

### WR-03: `feed.css`'s scoped glyph-sizing rule is a no-op duplicate

**File:** `web/static/css/feed.css:51-59`
**Issue:**

```css
.report-row .icon-badge .icon-glyph {
  width: 55%;
  height: 55%;
}
```

is byte-identical in effect to the already-cascading `.icon-badge .icon-glyph { width: 55%;
height: 55%; }` in `main.css:240-243` — 55% of the feed row's own smaller `.icon-badge--sm` box
resolves to the same value whether the declaration lives in main.css or is repeated here at
equal specificity. The comment above it ("the feed row's own smaller badge keeps its explicit
sizing without that shared rule needing to know about this specific badge size") describes a
distinction the rule doesn't actually make — both values are 55%, so this rule changes nothing.
Its only functional effect is satisfying `assertGlyphSizingAndNoOrphanedImageSelectors`'s
per-file requirement that `feed.css` contain *some* glyph-sizing rule with a `width` declaration.
**Fix:** Either delete the rule (main.css's shared rule already covers `.icon-badge--sm` since
percentage sizing is relative to each badge's own box) and relax the test's per-file requirement
to only apply where a file genuinely needs its own override, or — if a future badge size is
expected to diverge from 55% — leave a `/* intentionally matches main.css's 55% today */`
comment that says so honestly instead of implying a distinction that isn't there.

### WR-04: `TestIconGlyphsAreClassDriven`'s regression guard is brittle to quote style

**File:** `web/js_contract_test.go:387-408`
**Issue:**

```go
const imageNodeCreation = "createElement('img')"
...
if strings.Contains(text, imageNodeCreation) {
    t.Errorf(...)
}
```

This is the test guarding against exactly the regression that caused UAT Tests 2/6 (glyphs
rendering solid black in dark mode via a sealed `<img>` sub-document) — a real, security/UX
relevant guard. But the match is a literal substring against one specific quoting/spacing style.
A revert written as `createElement("img")`, `createElement( 'img' )`, or via
`document.createElement.bind(document, 'img')` / a template literal passes this check cleanly
while reintroducing the exact defect the test exists to catch. There is no linter/formatter step
enforcing single-quote consistency across this codebase's JS files that would otherwise make this
concern moot.
**Fix:** Loosen the match to be whitespace/quote-tolerant (e.g. a small regex
`createElement\(\s*['"]img['"]\s*\)`), or — more robustly — assert the *absence* of any
`<img` element ever appended to a badge/tile container by checking for `appendChild` calls whose
argument traces back to an `img`-tagged element, accepting that a determined revert can still
evade a purely textual check either way.

## Info

### IN-01: Icon SVGs still carry vestigial `fill="none" stroke="currentColor"` attributes that are now inert

**File:** `web/static/icons/*.svg` (all nine files, e.g. `flood.svg:8-9`)
**Issue:** Every icon file still declares `fill="none"` and `stroke="currentColor"` from its
prior life as a directly-embedded `<img>`/inline SVG. Now that every icon is consumed exclusively
as a `mask-image` source, the SVG is rasterized into a mask and only its **alpha channel**
matters — CSS Masking Level 1's `mask-mode: match-source` resolves to `alpha` for a `url()` image
reference, so `stroke="currentColor"` never resolves against any real page color and has no
visible effect; the mask boundary is defined purely by which pixels the 2px stroke touches.
Functionally harmless (this is why the mask technique works at all — thin opaque strokes on a
transparent background are a perfectly valid alpha mask), but the attribute is misleading
provenance: a future editor skimming these files could reasonably believe `currentColor` is still
doing something, when today it is dead weight carried over from the pre-01-13 rendering approach.
**Fix:** Optional cleanup — either strip `fill`/`stroke`/`stroke-width` down to whatever
minimally produces the intended silhouette (since only alpha matters now), or add a one-line
comment at the top of the icon directory (or in main.css's existing `.icon-glyph` comment block)
noting that these attributes are vestigial under mask consumption, so nobody "fixes" a
non-existent color bug here later.

### IN-02: 44 previously-reviewed files re-read with no new findings

**Files:** every file in `files_reviewed_list` above other than the 9 touched by plan 01-13.
**Issue:** N/A — re-reviewed at standard depth per this workflow's instructions; all remain
consistent with the prior (2026-09-06) clean assessment. The one open item on this file set —
`internal/testutil/seed.go`'s `seedCategories` using `storm_cyclone_damage` instead of the
canonical `storm_cyclone` slug — is already tracked in
`.planning/phases/01-foundation-report-map/deferred-items.md` and is referenced here only as a
pointer, not restated as a new finding.
**Fix:** N/A (tracked elsewhere).

---

_Reviewed: 2026-09-09T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
