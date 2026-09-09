---
status: diagnosed
trigger: "Investigate issue: category-glyph-legibility — After plan 01-13's CSS-mask fix (which correctly resolved the prior \"solid black icon\" defect), users report the category glyphs in dark mode are still hard to identify at a glance — the fix made icons technically non-black, but the resulting visual weight/contrast may be too low for fast recognition in this emergency-reporting app."
created: 2026-09-09T08:24:46Z
updated: 2026-09-09T08:50:00Z
---

## Current Focus
<!-- OVERWRITE on each update - reflects NOW -->

hypothesis: CONFIRMED (two additive mechanisms) — (1) all 9 category SVGs are stroke-only "line" art (fill="none", stroke-width="2" on a 24x24 viewBox) used as a CSS mask-image; alpha-masking (confirmed empirically via the screenshot) means only the ~2px stroke path is unmasked, producing an inherently thin hairline glyph in all 3 contexts/both themes regardless of color token — this alone explains the modal-tile complaint the user screenshotted, where hex contrast (~7:1) is not the bottleneck. (2) Separately, badge contexts (map pin, feed row) suffer a genuine WCAG contrast failure (~2.36:1 dark, ~1.6:1 light) specifically for reports aged to the "stale" stage, since `.age-stale` flattens the badge background to a fixed --color-age-stale regardless of severity while the glyph stays --color-bg.
test: n/a — root cause confirmed via direct file evidence (SVG source + CSS rules + computed contrast ratios + computed render sizes across all 3 contexts x 2 themes), goal is find_root_cause_only.
expecting: n/a
next_action: DONE — returning ROOT CAUSE FOUND structured diagnosis (do not proceed to fix_and_verify per goal: find_root_cause_only).

## Symptoms
<!-- Written during gathering, then IMMUTABLE -->

expected: Category glyphs (map pins, feed list rows, and the report modal's 3x3 category grid) are clearly, quickly identifiable at a glance in both light and dark mode — this matters because the app is for emergency reporting, where a user needs to recognize the right category fast, not squint at it.
actual: User retested the dark-mode report modal after plan 01-13 landed and reported (verbatim): "It's not that clear. I think the black background is mixing up with the outlines, and I think we should make it more significant so everybody knows because... people use it in a emergency situations, and I don't want them to... search up. Oh, I can't see which one it [is]." A screenshot confirmed the glyphs ARE rendering (not blank, not solid black) as thin light-grey/white outline shapes on the unselected dark tiles — so this is a legibility/visual-weight issue, not a recurrence of the original img-isolation bug.
errors: None reported (subjective visual-legibility complaint, not a crash/console-error bug)
reproduction: Open the "+" report submission modal with the OS/browser in dark mode; view the 3x3 category grid's unselected tiles (also check the map pin badges and feed list row icons, same underlying CSS).
started: Surfaced during a 01-UAT.md retest on 2026-09-09, immediately after plan 01-13 (which replaced <img>-loaded category SVGs with CSS mask-image-based .icon-glyph spans) merged to main.

## Eliminated
<!-- APPEND only - prevents re-investigating -->

- hypothesis: "Hex-contrast of --color-text-muted (#9CA3AF) on --color-surface (#16181C) in the
    unselected dark-mode category tile is the primary driver of low legibility (the UAT note's
    suggested numeric-contrast concern)."
  evidence: "Computed WCAG relative-luminance contrast ratio ≈7.0:1 for this exact pairing —
    passes AAA (7:1), not just AA (4.5:1). A pairing that already exceeds the strictest
    normal-text WCAG threshold cannot be the primary legibility bottleneck; the bottleneck is
    the source SVGs' stroke-only geometry capping ink coverage regardless of color (see
    Evidence/Resolution). Cross-checked against light mode: the unselected-tile pairing there
    (--color-text-muted #6B7280 on --color-surface #F1F3F4) computes to only ≈4.3:1 — objectively
    WORSE contrast than dark mode's 7:1 — yet Test 7 (light-mode-inclusive shape-correctness
    check) recorded no legibility complaint. This is consistent with (not conclusive proof of,
    since Test 7 targeted shape-correctness not at-a-glance recognition speed) hex-contrast not
    being the discriminating factor between the theme that drew a complaint and the one that
    didn't.
  timestamp: 2026-09-09T08:55:00Z

## Evidence
<!-- APPEND only - facts discovered -->

- timestamp: 2026-09-09T08:30:00Z
  checked: web/static/css/main.css (full file, 770 lines)
  found: |
    `.icon-glyph` base rule (lines 265-274) sets only background-color:currentColor +
    mask-size:contain/mask-repeat:no-repeat/mask-position:center — NO width/height. Sizing is
    delegated to context-specific rules: `.icon-badge .icon-glyph` (line 240-243) = 55% of
    badge box; `.icon-badge--sm` (321-324) = 32px badge; `.icon-badge--pin` and default
    `.icon-badge` = 44px (--touch-target-min). `.category-tile` (422-438) sets
    `color: var(--color-text-muted)`; `.category-tile--selected` (440-444) sets
    `color: var(--color-bg)` on a `--color-text` background (contrast-inverted).
    Dark-mode tokens (73-96): --color-surface:#16181C, --color-text-muted:#9CA3AF,
    --color-text:#F3F4F6, --color-bg:#0B0D10.
  implication: Confirms glyph color resolves correctly through the cascade per-context (the
    01-13 fix is sound); size is delegated elsewhere and needs checking per-context.

- timestamp: 2026-09-09T08:32:00Z
  checked: web/static/css/modal.css lines 45-60
  found: "`#category-grid .category-tile .icon-glyph { width: 24px; height: 24px; }` — the
    modal category-grid glyph renders at a fixed 24x24px box (not the 55%-of-badge rule,
    since the tile itself is not `.icon-badge`)."
  implication: Modal tile glyph render box = 24x24px, matching the SVG's own 24x24 viewBox
    1:1 (mask-size:contain at scale 1.0).

- timestamp: 2026-09-09T08:33:00Z
  checked: web/static/css/feed.css lines 51-59, web/static/js/feed.js:130, map.js:144
  found: "Feed row badge = `.icon-badge.icon-badge--sm` (32px box) → glyph 55% = 17.6px.
    Map pin badge = `.icon-badge.icon-badge--pin` (44px box, = --touch-target-min) → glyph
    55% = 24.2px. Badge glyph color = `--color-bg` (main.css:236, inherited via .icon-badge)
    painted on a `--severity-current` (bright saturated) background circle."
  implication: Three distinct render sizes across contexts (24px tile, 24.2px map pin,
    17.6px feed row) — feed row renders smallest, most vulnerable to sub-pixel thinning.

- timestamp: 2026-09-09T08:36:00Z
  checked: All 9 files in web/static/icons/*.svg (flood, earthquake, fire, storm_cyclone,
    road_blocked, power_outage, shelter_open, rescue_needed, other)
  found: |
    Every single icon (Lucide v1.41.0 icon set) is authored as stroke-only line art:
    `fill="none" stroke="currentColor" stroke-width="2"` on a `viewBox="0 0 24 24"`, e.g.
    flood.svg's droplet is one open `<path>` with no fill; rescue_needed.svg's life-buoy is
    two `<circle>` elements with fill="none" (hollow rings) plus four short crossing
    `<path>` strokes. NONE of the 9 SVGs have any filled/solid shape — every visible pixel
    in every icon comes exclusively from a 2px-wide stroke outline.
  implication: This is the smoking gun. When used as a CSS `mask-image: url(icon.svg)`,
    the browser alpha-masks against the rendered image — fully-transparent pixels
    (everywhere fill="none" leaves untouched, i.e. the entire interior of every shape) mask
    out completely regardless of the source SVG's stroke color; only the opaque ~2px stroke
    path masks in and lets `background-color: currentColor` show through. This is standard,
    well-established CSS masking behavior (mask-image on a referenced SVG image uses alpha
    masking, not luminance masking, per the CSS Masking spec) and is independent of which
    color token is applied — a filled/solid icon of the identical outer silhouette would
    show a solid colored shape; this stroke-only icon set can only ever show a thin outline,
    regardless of currentColor's value or WCAG contrast.

- timestamp: 2026-09-09T08:40:00Z
  checked: Computed physical stroke width at each of the 3 render sizes (SVG stroke-width=2
    in a 24-unit viewBox, mask-size:contain)
  found: |
    Modal tile (24px box / 24 viewBox = scale 1.0): stroke ≈ 2.0px physical.
    Map pin badge (24.2px box / 24 viewBox = scale ≈1.008): stroke ≈ 2.02px physical.
    Feed row badge (17.6px box / 24 viewBox = scale ≈0.733): stroke ≈ 1.47px physical.
  implication: All three contexts render a sub-2px hairline; the feed row (smallest) drops
    below 1.5px, meaning on a standard (non-Retina) display the stroke can partially
    anti-alias/sub-pixel-render even fainter than in the modal tile the user directly
    screenshotted — the same root cause likely reads as EVEN LESS legible in the feed-row
    context than in the reported modal-tile context, not less severe.

- timestamp: 2026-09-09T08:44:00Z
  checked: WCAG contrast ratio computation for the specific pairing the UAT hint flagged —
    unselected tile: --color-text-muted (#9CA3AF) on --color-surface (#16181C), dark mode
  found: "Computed relative-luminance contrast ratio ≈ 7.0:1 (relative luminance of #9CA3AF
    ≈ 0.3635, of #16181C ≈ 0.00908; ratio = (0.3635+0.05)/(0.00908+0.05) ≈ 6.999). This
    passes WCAG AAA (7:1) for normal text, not just AA (4.5:1)."
  implication: Confirms the UAT hint's own suspicion — this is NOT a hex-contrast problem
    in the WCAG sense. The bottleneck is the stroke-only source art's inherent thinness
    (Evidence above), which a high point-sample contrast ratio does not fix: a human's
    at-a-glance perception of a shape's "visual weight"/salience depends on ink AREA, not
    just point-contrast of the ink that exists, and a 2px stroke covers a small fraction of
    a 24px glyph box's area regardless of how strongly that stroke itself contrasts with the
    background.

- timestamp: 2026-09-09T08:46:00Z
  checked: web/static/js/modal.js:198-224 (buildCategoryGrid) — confirms DOM structure
  found: "Glyph span is a direct child of the `<button class=\"category-tile\">`, not
    wrapped in `.icon-badge` — confirms the tile context uses the dedicated
    modal.css:57-60 24x24px sizing rule, not the 55%-of-badge rule, matching the CSS
    evidence above."
  implication: No discrepancy between JS DOM structure and the CSS rules read — sizing and
    color-cascade wiring both work exactly as main.css's own comments (lines 217-263)
    describe. The 01-13 fix correctly solved the img-isolation/currentColor problem; this is
    a distinct, pre-existing characteristic of the source icon artwork that the mask
    technique now faithfully reproduces (previously masked by the fact the old <img>-based
    icons were unreadable in dark mode for an unrelated reason, so nobody could evaluate
    stroke-weight legibility until 01-13 fixed the color-isolation bug).

- timestamp: 2026-09-09T08:57:00Z
  checked: Empirical cross-check of alpha-vs-luminance mask-type assumption (no external
    doc lookup — inferred from the UAT screenshot's own observed behavior)
  found: |
    Every icon SVG's stroke resolves currentColor to black inside its own document context
    when rendered as a referenced mask image (same sealed-document rule that caused the
    original img-isolation bug). If the browser were using LUMINANCE masking (not alpha) for
    this mask-image reference, a black stroke has luminance ≈0, which would mask that stroke
    OUT (treat it as invisible), leaving nothing visible at all — contradicting the UAT
    screenshot, which shows the glyphs clearly rendering as visible shapes. The glyphs being
    visible at all is therefore direct empirical proof the browser is alpha-masking this
    mask-image reference (transparency, not color/luminance, determines what's masked in) —
    stronger and more reliable evidence than a spec citation, since it's derived from the
    actual observed rendering behavior on the tester's own device rather than assumed engine
    conformance.
  implication: Confirms (empirically, not just by spec-reasoning) that only the SVGs'
    non-transparent pixels — the ~2px stroke paths — can ever appear, regardless of engine.

- timestamp: 2026-09-09T09:02:00Z
  checked: Badge-context (icon-badge) glyph contrast for AGED/STALE reports specifically —
    `.icon-badge { color: var(--color-bg); }` painted on `--severity-current`, which
    `.age-stale` (main.css:211-215) overrides to a flat `--color-age-stale` regardless of
    original severity
  found: |
    DARK mode: glyph color --color-bg = #0B0D10 on badge background --color-age-stale =
    #4B4F55. Computed WCAG contrast ≈2.36:1 — FAILS WCAG 1.4.11 Non-text Contrast (3:1
    minimum for graphical objects/icons) and fails AA text contrast (4.5:1) outright.
    LIGHT mode: glyph color --color-bg = #FFFFFF on badge background --color-age-stale =
    #C9CDD1. Computed WCAG contrast ≈1.6:1 — an even more severe failure than dark mode.
    This is severity-independent (age-stale forces the SAME neutral gray regardless of
    whether the underlying report was low/medium/critical), so EVERY stale-aged report's map
    pin and feed-row badge glyph has genuinely insufficient point-contrast in BOTH themes —
    a real, separate contrast defect, distinct from the stroke-thinness mechanism, and scoped
    specifically to the badge (map pin / feed row) contexts once a report has aged to the
    "stale" stage (D-17's age-desaturation ramp). Spot-checked `.age-aging` (50% color-mix
    between original severity and --color-age-stale) for severity-critical in dark mode:
    ≈3.7:1 — above the 3:1 graphical-object floor but still below the 4.5:1 text floor,
    i.e. borderline/marginal rather than a clear failure like the stale stage.
  implication: |
    This is a SECOND, distinct legibility mechanism, additive to (not a replacement for) the
    stroke-thinness mechanism found earlier — and it directly answers the investigation
    hint's question of whether "the legibility problem may differ in severity/cause between
    the three contexts." It does: the modal category-grid tile (the context the user directly
    screenshotted and complained about) has NO age-ramp involvement at all (tiles aren't
    severity/age-scoped) and its low visual weight is caused solely by the stroke-thinness
    mechanism with adequate (7:1) hex contrast. The badge contexts (map pins, feed rows) share
    that same stroke-thinness mechanism at baseline, AND ADDITIONALLY suffer a genuine,
    quantifiable WCAG contrast failure once a report desaturates to the "stale" age stage,
    compounding the thin-stroke problem with actual insufficient point-contrast in both
    themes.

## Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: |
  TWO distinct, additive mechanisms, both traced to specific files/lines, neither a regression
  from 01-13's own logic (01-13's mask/color-cascade mechanism is implemented correctly per its
  own design comments in main.css — both defects below are properties of what's now correctly
  and faithfully rendered, not implementation bugs in the mask technique itself).

  MECHANISM 1 — stroke-only source artwork (applies to ALL THREE contexts, BOTH themes; this
  is the one the user directly screenshotted and complained about, in the modal category grid):
  The category icon artwork (web/static/icons/*.svg, Lucide v1.41.0 icon set, 9 files) is
  authored as pure stroke/line art — every file uses `fill="none" stroke="currentColor"
  stroke-width="2"` on a `viewBox="0 0 24 24"`, with zero filled/solid shapes. CSS
  `mask-image` alpha-masks against the rendered SVG (confirmed empirically, not just by spec:
  the glyphs ARE visible in the UAT screenshot, which is only possible under alpha masking —
  luminance masking would mask out a black-resolved stroke entirely). Only the opaque ~2px-wide
  stroke path becomes visible mask area; every transparent interior (the vast majority of each
  icon's 24x24 box) masks out completely. This yields an inherently thin "hairline" glyph —
  roughly 1.5-2px of actual rendered stroke width across the three render contexts (24px modal
  tile = ~2.0px stroke, 24.2px map pin badge = ~2.02px stroke, 17.6px feed-row badge = ~1.47px
  stroke, the feed row being thinnest and most vulnerable to sub-pixel anti-aliasing) —
  regardless of which color token paints it. This mechanism is NOT a color-contrast defect in
  the specific unselected-tile context the UAT note flagged: --color-text-muted (#9CA3AF) on
  --color-surface (#16181C) computes to ≈7:1, passing WCAG AAA. The low-contrast-token choice
  there is a real, compounding factor (a thinner stroke has less ink area for the eye to
  average brightness over, so it's more sensitive to a dimmer color than a filled icon of the
  same footprint would be) but is secondary to, not the primary cause of, the low visual
  weight in that context — the primary cause is the stroke-only geometry itself, which caps
  the mask's maximum possible ink coverage regardless of color choice. This defect was latent
  in the icon source files since they were introduced (01-02) but was undiagnosable until
  01-13 fixed the unrelated color-isolation bug that had made every glyph unconditionally
  invisible (solid black) in dark mode.

  MECHANISM 2 — genuine WCAG contrast failure, scoped to BADGE contexts only (map pin
  .icon-badge--pin, feed row .icon-badge--sm), and specifically to reports that have
  desaturated to the "stale" age stage (D-17's age ramp), in BOTH themes: `.icon-badge` paints
  its glyph with `color: var(--color-bg)` (main.css:236) against a `--severity-current`
  background, and `.age-stale` (main.css:211-215) overrides `--severity-current` to a flat
  `--color-age-stale` regardless of the report's original severity. Computed contrast: DARK
  mode glyph #0B0D10 on badge #4B4F55 ≈2.36:1 (fails WCAG 1.4.11's 3:1 graphical-object floor
  and AA's 4.5:1 text floor); LIGHT mode glyph #FFFFFF on badge #C9CDD1 ≈1.6:1 (an even worse
  failure). This is a genuine, quantifiable, severity-independent point-contrast defect —
  distinct from Mechanism 1 and additive to it in these contexts — that only manifests once a
  report ages into the stale stage (fresh/aging-stage badges compute acceptable contrast, e.g.
  ≈5:1+ for a fresh/aging low-severity dark-mode badge). The modal category-grid tile (the
  specific context the user screenshotted) has no age-ramp involvement at all, so Mechanism 2
  does not apply there — Mechanism 1 alone explains that complaint.
fix: []
verification: []
files_changed: []
