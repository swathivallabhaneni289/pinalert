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
  warning: 8
  info: 2
  total: 10
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-09T00:00:00Z
**Depth:** standard
**Files Reviewed:** 53
**Status:** issues_found

## Summary

This review supersedes the prior 01-REVIEW.md (0 Critical / 5 Warning / 2 Info) for the same 53
files, following plan 01-15 — a live, human-in-the-loop correction round run outside the normal
executor pipeline. 01-15 changed: `web/static/css/main.css` (new `.category-tile .icon-glyph` /
`.category-tile--selected .icon-glyph` color override; badge glyph sizing 55%→48%),
`web/static/css/feed.css` (kept its scoped glyph-sizing rule in sync at 48%),
`web/css_contract_test.go` (removed `TestCategoryGlyphInkCoverageAcrossRenderContexts` and its
exclusive helpers), all nine icon SVGs (`stroke-width` reverted 3→2, undoing 01-14), and three new
files — `cmd/server/main.go`, `internal/api/handlers/page.go`, `web/templates/index.html.tmpl` —
adding a process-start-time `AssetVersion` cache-busting query string (`?v=`) on every local
CSS/JS asset link, fixing a real bug where a 1-hour `Cache-Control: max-age=3600` plus orphaned
`go run` server processes meant a human tester could not see fixes land even after correct
restarts.

**Scrutiny tiering:** the 01-15 file set got a full line-by-line read plus independent
verification — `go build ./...`, `go vet ./...`, and `go test ./web/... ./internal/api/handlers/...
./cmd/...` all pass; `git show` was used to confirm the exact before/after diff for each changed
file rather than relying on the stated change summary. The remaining files (unchanged since the
prior pass) were re-confirmed still-current by re-reading the exact lines each carried-forward
finding below cites.

**CSS specificity claim verified correct.** `.category-tile .icon-glyph` and
`.category-tile--selected .icon-glyph` are both exactly two chained class selectors — identical
specificity (0,2,0) — so the winner is decided by source order, and
`.category-tile--selected .icon-glyph` (main.css line 482) is declared after
`.category-tile .icon-glyph` (line 472), so it correctly wins for a tile carrying both classes.
Confirmed against actual DOM construction in `modal.js:203`/`271` (`tile.className =
'category-tile'`, then `classList.toggle('category-tile--selected', ...)` — both classes really do
co-exist on the same element, not swapped). Also checked `modal.css` for a competing
`#category-grid .category-tile .icon-glyph` declaration that could out-specificity this pair from
a different file loaded later — it only sets `width`/`height` there (24px), never `color`, so no
conflict. This mechanism is sound.

**Test-removal audit: clean.** `TestCategoryGlyphInkCoverageAcrossRenderContexts` and its exclusive
helpers (`extractAttr`, `findDeclValue`, `parsePx`, `parsePercent`, the ink-floor/ceiling
constants) are fully gone from `web/css_contract_test.go` — `grep` for each name returns only the
doc-comment's prose mentions, `go build`/`go vet` are clean, and `TestBadgeGlyphContrastAcrossAgeStagesAndThemes`
is byte-for-byte untouched. No dangling reference, no orphaned helper, no unused import. This
removal does not leave a regression risk unguarded that WR-05 (below, now closed) hadn't already
flagged as testing nothing independent.

**AssetVersion wiring: correctly threaded, no injection risk, but incompletely covers the caching
surface it was built to fix.** `PageConfig.AssetVersion` → `pageViewModel.AssetVersion` →
`{{.AssetVersion}}` in every local `<link>`/`<script>` tag is wired end-to-end correctly, and
`strconv.FormatInt(time.Now().Unix(), 10)` is fully server-controlled numeric output with no
attacker-reachable input, so there is no XSS/injection risk regardless of `html/template`'s
contextual escaping. However, three residual caching gaps are now findings below (WR-06, WR-07,
WR-08) — the general shape of this plan's own root-cause bug (byte-identical cached response,
nothing forcing a refetch) turns out not to be fully closed by this fix, just narrowed to CSS/JS
document `<link>`/`<script>` tags specifically.

## Warnings

### WR-01 (closed): dead `.icon-badge svg` selector

Fixed by 01-14. Kept as a closed entry so this document's numbering stays stable across review
passes.

### WR-02: Per-category `-webkit-mask-image` declarations are still asymmetrically unguarded

**File:** `web/static/css/main.css:302-345`, `web/css_contract_test.go` (`collectGlyphMaskRules`)
**Issue:** Unaffected by 01-15 — confirmed unchanged (`grep -n "webkit-mask-image"
web/css_contract_test.go` still matches only a comment, never an assertion).
`assertBaseGlyphRule` enforces both prefixed and unprefixed mask-size/repeat/position on the
shared `.icon-glyph` base rule, but the nine per-category rules that resolve the actual icon path
are only checked in unprefixed form. Deleting all nine `-webkit-mask-image` lines from `main.css`
would leave every test in the package green while older WebKit-based engines render all nine
category glyphs invisible.
**Fix:** Extend `collectGlyphMaskRules` (or add a companion assertion) to also require a
`-webkit-mask-image` declaration resolving to the same path as the unprefixed `mask-image` for
every category rule.

### WR-03: `feed.css`'s scoped glyph-sizing rule is a same-value duplicate, and its explanatory comment is now factually wrong

**File:** `web/static/css/feed.css:51-59`
**Issue:** 01-15 changed both `main.css`'s `.icon-badge .icon-glyph` and `feed.css`'s
`.report-row .icon-badge .icon-glyph` from 55% to 48% (confirmed via `git show 1f87510` and `git
show 5da5d1f`), so the two rules remain byte-identical in effect — the scoped rule still changes
nothing at runtime, same underlying no-op-duplicate issue the prior review flagged. What's new:
the comment directly above the rule was **not** updated and now reads:

```css
/* Scoped to .report-row so this does not reach map.js's pin badges, which
   this plan does not own. main.css's shared `.icon-badge .icon-glyph`
   rule already sizes the glyph at 55%; this scoped rule exists so the
   feed row's own smaller badge keeps its explicit sizing without that
   shared rule needing to know about this specific badge size. */
.report-row .icon-badge .icon-glyph {
  width: 48%;
  height: 48%;
}
```

The comment claims main.css sizes the glyph at "55%" — main.css is also 48% now. A future editor
reading only this comment would believe the two files still diverge by design; they don't.
**Fix:** This rule cannot simply be deleted — `TestCategoryGlyphMaskRulesCoverEveryCategory`'s
`assertGlyphSizingAndNoOrphanedImageSelectors` requires at least one rule in `feed.css` whose
selector contains `icon-glyph` and whose body contains `width` (a presence check, not a value
check), so removing it outright fails that test. Either (a) keep the rule, correct the comment's
"55%" to "48%," and state plainly that it is currently a byte-identical duplicate of main.css's
cascade value kept only to satisfy that test's per-file requirement; or (b) if `feed.css` is meant
to diverge from `main.css` going forward, do so now and drop the stale "already sizes... 55%"
framing entirely.

### WR-04: `TestIconGlyphsAreClassDriven`'s regression guard is still brittle to quote style

**File:** `web/js_contract_test.go:387-408`
**Issue:** Unchanged since the prior review — confirmed by re-reading current source. The guard
against the exact regression that caused UAT Tests 2/6 is a literal substring match
(`const imageNodeCreation = "createElement('img')"`). A revert written as
`createElement("img")` or built via a template literal passes this check cleanly while
reintroducing the defect.
**Fix:** Loosen the match to be whitespace/quote-tolerant, or assert the absence of any
`<img`-tagged element ever appended to a badge/tile container instead of matching the constructor
call textually.

### WR-05 (closed): `TestCategoryGlyphInkCoverageAcrossRenderContexts`'s tautological per-context assertions

Test deleted outright by 01-15 (see summary's test-removal audit above), which is a valid
resolution of this finding — the prior review had already established the per-context comparisons
were algebraically identical to the single uniform-stroke-width check that ran alongside them, so
no independently-verifying assertion is lost by the removal. Kept as a closed entry so this
document's numbering stays stable.

### WR-06 (fixed post-review): The `AssetVersion` cache-busting mechanism has no test coverage

**File:** `internal/api/handlers/page_test.go`, `internal/api/handlers/page.go`
**Issue:** `TestPageShellServesDOMContract` constructs `PageConfig{}` without setting
`AssetVersion` (so it renders empty: `/static/css/main.css?v=`) and never asserts that
`AssetVersion` is threaded from `PageConfig` through `pageViewModel` into the rendered `href`/`src`
attributes. If a future edit accidentally dropped the `AssetVersion: cfg.AssetVersion,` line in
`Page()` (page.go:62) — silently reverting to the exact stale-cache bug this plan exists to fix —
no test in this package would fail.
**Fix:** Add an assertion (either in `TestPageShellServesDOMContract` or a new test) that
constructs `PageConfig{AssetVersion: "some-distinct-value"}` and asserts every local CSS/JS tag's
`href`/`src` contains `?v=some-distinct-value`.
**Resolution:** Fixed same-day, post-review: added `TestPageShellAppliesAssetVersionToLocalStaticAssets`
(page_test.go) asserting exactly this — every local CSS/JS URL contains `?v=<AssetVersion>` and
the third-party CDN script does not.

### WR-07 (fixed post-review): Icon SVGs referenced via CSS `url()` are not covered by the `AssetVersion` mechanism

**File:** `web/static/css/main.css:302-345` (nine `mask-image: url("/static/icons/*.svg")`
declarations), `internal/api/router.go:94`
**Issue:** `AssetVersion` only versions the CSS/JS `<link>`/`<script>` tags the template emits
directly; every `mask-image: url("/static/icons/{category}.svg")` in `main.css` is a static,
unversioned path. `staticFileServer` (router.go:94) applies the same
`Cache-Control: public, max-age=3600` to every path under `/static/*`, icons included. This plan's
own diff reverted all nine icons' `stroke-width` from 3 back to 2 — a real content change to those
exact files — but a browser that already cached the old (stroke-width=3) SVG bytes from within the
last hour has no reason to refetch them even after the versioned CSS forces a fresh CSS fetch,
because the icon `url()` path embedded in that fresh CSS is unchanged. This is a narrower,
time-bounded (≤1 hour) recurrence of the same failure mode — a cached response with nothing
forcing a refetch — that this plan was built to eliminate for CSS/JS.
**Fix:** Either version the icon `url()` paths too (e.g. a build/deploy step appending
`?v={{.AssetVersion}}` isn't available inside a static CSS file without templating CSS itself, so
a content-hash-based filename or a shorter `max-age` specifically for `/static/icons/*` is more
practical), or reduce `Cache-Control` max-age for the icons subtree so this window shrinks from an
hour to something operationally acceptable.
**Resolution:** Fixed same-day, post-review, via a different route than either suggested fix:
rather than version or shorten-cache just the icons subtree, added `api.Deps.Dev` (router.go),
threaded from `cmd/server/main.go`'s existing `ENV=development` check, which makes
`staticFileServer` send `Cache-Control: no-store` for the ENTIRE `/static/*` tree (not just
icons) whenever `ENV=development` — eliminating this failure class for every current and future
static asset in dev mode, not just the ones this review happened to name. Production behavior
(`public, max-age=3600`) is unchanged, since the zero-value `Dev: false` is what every existing
`Deps{}` literal without the field already gets.

### WR-08 (new): The HTML document response itself (which carries the `?v=` token) sets no `Cache-Control` or validator headers

**File:** `internal/api/handlers/page.go:56-70`
**Issue:** `Page()` sets only `Content-Type`; it never sets `Cache-Control`, `Last-Modified`, or
`ETag` on the `/` response. The entire `AssetVersion` mechanism depends on a fresh HTML document
being served on every request/restart — if the document itself is ever served stale by an
intermediary (a CDN, a reverse proxy fronting a future deploy, or a browser's own heuristic
freshness caching in the absence of any explicit directive, per RFC 7234 §4.2.2), the new `?v=`
token would never reach the client at all and every static asset link would keep resolving to
whatever version was baked into the last-cached HTML. In practice, most browsers revalidate a
plain navigation request with no validator headers, so this is unlikely to bite today behind no
intermediary — but it is the mechanism's own foundation and is currently unguarded by any explicit
directive.
**Fix:** Set an explicit `Cache-Control: no-cache` (or `no-store`) header on the `/` response in
`Page()`, so the document is never cached by an intermediary regardless of heuristic freshness
rules, making the `AssetVersion` mechanism's correctness independent of client/proxy defaults.

## Info

### IN-01: Icon SVGs still carry vestigial `fill="none" stroke="currentColor"` attributes

**File:** `web/static/icons/*.svg` (all nine files, e.g. `flood.svg:8-9`)
**Issue:** Unchanged since the prior review. Every icon still declares `fill="none"`/
`stroke="currentColor"` from its pre-mask-consumption history. Since every icon is consumed
exclusively as a `mask-image` source, only the rasterized alpha channel matters; `stroke=
"currentColor"` never resolves against any real page color. Functionally harmless.
**Fix:** Optional — strip `fill`/`stroke` down to whatever minimally produces the intended
silhouette, or add a one-line comment noting these attributes are vestigial under mask
consumption.

### IN-02 (closed): `road_blocked.svg` stroke-merge risk at small render context

Moot as of 01-15 — the finding's entire premise was 01-14's `stroke-width="3"`, which 01-15
reverted back to `stroke-width="2"` (confirmed via direct inspection of all nine SVGs). The
specific interior-stroke-merge risk at the feed row's smallest render context no longer applies at
the current, thinner stroke weight. Kept as a closed entry so this document's numbering stays
stable.

---

_Reviewed: 2026-09-09T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
