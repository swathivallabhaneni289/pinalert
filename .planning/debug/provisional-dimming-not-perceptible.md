---
status: diagnosed
trigger: "provisional-dimming-not-perceptible: A fresh, unconfirmed (Provisional) report's badge/border is supposed to look visibly desaturated compared to a normal report, but the user cannot perceive any fading, on either the feed row or the map popup, even though the Unconfirmed chip itself displays correctly."
created: 2026-09-22T00:00:00Z
updated: 2026-09-22T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — .vis-provisional in trust.css IS applied correctly to both surfaces
  (no code defect in class application, cascade order, or specificity), but its 50% color-mix
  toward --color-age-stale produces only ~1.9-2.0:1 luminance contrast against the undimmed
  severity color in dark mode (and ~1.46:1 in light mode, in the opposite luminance direction) —
  well under the ~3:1 floor this project's own contrast tests treat as the graphical-distinguishability
  threshold elsewhere (TestBadgeGlyphContrastAcrossAgeStagesAndThemes). This is a design-strength
  gap, not a code defect.
test: traced applyVisibilityClass call sites (feed.js:215, map.js:157), confirmed CSS load order
  (index.html.tmpl: main.css -> modal.css -> feed.css -> trust.css -> auth.css), checked every
  stylesheet loaded after trust.css for a conflicting rule targeting .icon-badge/.report-row/
  --severity-current, computed color-mix() results and WCAG relative-luminance contrast ratios
  for Low/Medium severity in both themes at the shipped 50% ratio.
expecting: either a structural defect (class not applied / overridden downstream) or confirmation
  that the class applies but the resulting color delta is too small to read as "faded" without a
  side-by-side reference.
next_action: none — mode is find_root_cause_only, returning diagnosis to caller without a fix.

## Symptoms

expected: A fresh non-critical report's badge/border is visibly desaturated (D-09 Provisional
  dimming) on both the feed row and the map pin popup, clearly distinct from a normal
  (Live-visibility) report, in addition to the Unconfirmed chip.
actual: User reported (2026-09-21), with a screenshot: "I don't see the faded look but I
  definitely see the unconfirmed chip to it." The user does not perceive any fading on either
  surface, in dark mode.
errors: None.
reproduction: Test 5 in UAT (.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UAT.md).
  Post a fresh non-critical, non-rescue report and look at its badge/border on the feed row and its
  map pin popup, in dark mode.
started: Discovered during Phase 2 UAT round 3, 2026-09-21. Carried forward from UAT Test 2 (same
  round), never visually confirmed either way until this round's screenshot.

## Eliminated

- hypothesis: .vis-provisional class is not applied to the DOM element (JS bug: applyVisibilityClass
    never called, or called on the wrong element).
  evidence: feed.js:215 calls PinalertVisibility.applyVisibilityClass(row.el, report) inside
    updateRow, on the same .report-row element that lines 204-205 apply sev-*/age-* to (custom
    properties set here inherit down to the row's descendant .icon-badge). map.js:157 calls
    applyVisibilityClass(badge, report) directly on the pin's .icon-badge element (lines 147-150
    build sevClass/ageClass onto the same badge). visibility.js's visibilityState() defaults
    unrecognised/missing values to 'provisional' (fail-safe direction), and
    replacePrefixedClass strips any existing vis-* class before adding the new one — no path
    leaves the element without a vis-* class. This matches the code's own extensive comments
    describing exactly this placement split.
  timestamp: 2026-09-22

- hypothesis: --severity-current from .vis-provisional is overridden by a later, higher- or
    equal-specificity CSS rule (specificity loss / wrong variable overridden downstream).
  evidence: index.html.tmpl loads stylesheets in order main.css, modal.css, feed.css, trust.css,
    auth.css — trust.css (which declares .vis-provisional) loads AFTER main.css (which declares
    .sev-*/.age-*), so at equal specificity (one class selector each) source order alone makes
    .vis-provisional win, exactly as trust.css's own header/section comments claim and as
    TestVisibilityCascadeOverridesAgeRamp guards. Searched every stylesheet that loads after
    trust.css (auth.css) and grepped all three remaining stylesheets (feed.css, modal.css,
    auth.css) for any rule touching --severity-current, .icon-badge, or .report-row: auth.css only
    touches unrelated form-validation text colors; modal.css's severity rules are scoped to
    .severity-control (the report-submission slider, a different DOM subtree entirely); feed.css's
    .report-row rule only sets `cursor: pointer` (no color/background property). No conflicting
    rule exists anywhere in the cascade.
  timestamp: 2026-09-22

- hypothesis: The desaturation is present but rendered on too small a visual area (thin 4px left
    border) to be noticed, independent of the color math.
  evidence: The user reported the SAME failure on the map pin, where the badge is a full 44x44
    circular fill (icon-badge--pin), not a thin border — a much larger, unmissable area. If area
    were the limiting factor, the map pin (large area) should have read as visibly faded even if
    the feed row's thin border did not. Since both surfaces failed identically, the common cause
    must be the color delta itself, not the geometry of where it's painted.
  timestamp: 2026-09-22

## Evidence

- timestamp: 2026-09-22
  checked: trust.css line 128-131 (.vis-provisional rule) and main.css's severity/age token tables
    (light :root block, dark @media/[data-theme] blocks)
  found: --severity-current: color-mix(in srgb, var(--severity-base) 50%, var(--color-age-stale) 50%);
    --badge-glyph-fg: var(--color-text). Confirmed this is a flat 50/50 average of the severity
    base color and the theme's --color-age-stale token — identical in shape and ratio to
    main.css's .age-aging rule (deliberately, per the file's own comment and
    TestVisibilityCascadeOverridesAgeRamp's string-equality assertion).
  implication: The dimming mechanism used for "not yet trusted" (Provisional) is exactly the same
    strength as the mechanism used for "about a quarter of its lifetime left" (Aging) — an
    intermediate, moderate desaturation stage, not the strongest ("Stale", 100% mix) stage. This is
    a strength choice, not a defect.

- timestamp: 2026-09-22
  checked: computed color-mix() results for dark-mode Low severity
    (--color-severity-low: #34D399 = rgb(52,211,153), --color-age-stale: #4B4F55 = rgb(75,79,85)),
    then checked whether the screenshot's pixel-sampled hexes (#58947c row, #4b7d6a pin, per
    known_context) are actually reachable as ANY linear mix ratio t of those two declared endpoints
  found: 50/50 mix = rgb(64,145,119) = #409177 (matches known_context's own prediction). Testing
    the sampled hexes against the mix function R(t)=52+23t, G(t)=211-132t, B(t)=153-68t (t in
    [0,1]): for #58947c, R=88 is OUTSIDE the reachable range [52,75] for any t — no value of t
    reproduces R=88 from these two endpoints, while G=148 and B=124 independently imply t≈0.45-0.48
    (i.e. roughly consistent with the shipped 50% ratio). The pin sample (#4b7d6a) shows a similar
    pattern.
  implication: The sampled screenshot hexes are NOT a valid quantitative check on the mix ratio —
    at least one channel (R) is arithmetically unreachable from the two declared CSS endpoints at
    any ratio, most likely because a macOS screenshot's Display-P3-tagged pixel data was read by a
    pixel-picker as raw sRGB numbers (a common, non-code-related distortion for saturated greens),
    or a compression artifact. G/B being roughly consistent with a ~45-50% ratio is suggestive but
    not proof. This finding does NOT weaken the diagnosis: the class-application and contrast-math
    conclusions below are derived directly from the CSS source and the token tables, not from the
    screenshot, and stand independently of whether the sampled pixel matches the predicted hex
    exactly.

- timestamp: 2026-09-22
  checked: WCAG relative-luminance contrast ratio (same formula this project's own
    TestBadgeGlyphContrastAcrossAgeStagesAndThemes uses, applied here badge-color-vs-badge-color
    instead of glyph-vs-fill) between the undimmed severity color and the .vis-provisional 50%-mix
    color, for both Low and Medium severities, both themes
  found: Dark Low: undimmed #34D399 (L=0.496) vs dimmed #409177 (L=0.227) -> ratio ~1.97:1.
    Dark Medium: undimmed #E5A93B (L=0.453) vs dimmed #987C48 (L=0.216) -> ratio ~1.89:1.
    Light Low: undimmed #2F9E64 (L=0.260) vs dimmed #7CB69B (L=0.401) -> ratio ~1.46:1 (dimmed is
    LIGHTER/higher-luminance than undimmed in light mode, since --color-age-stale in light mode,
    #C9CDD1, is a near-white pale gray rather than a mid-dark gray, so mixing toward it raises
    luminance instead of lowering it).
    For comparison, the SAME formula applied to the fully-desaturated "Stale" age stage (100% mix,
    i.e. --severity-current: var(--color-age-stale) directly) vs. undimmed Low in dark mode gives
    ratio ~4.29:1 — more than double the Provisional ratio.
  implication: A ~1.9-2.0:1 (dark) or ~1.46:1 (light) contrast between two states that are supposed
    to read as visually distinct is a weak perceptual signal — well under the ~3:1 floor this
    project's own contrast tests use elsewhere as the graphical-distinguishability threshold. This
    quantifies exactly why the user cannot perceive the fading: the math is working as coded, but
    the coded strength (50%, "Aging"-equivalent) is roughly half as strong as the "Stale" stage
    that WOULD be clearly perceptible, and Provisional has no live report physically adjacent to it
    in the UI to serve as a side-by-side anchor (unlike Aging/Stale, which the user visually learns
    by watching one report's own border change over its lifetime).
  implication_light_mode: Light mode is quantitatively even weaker than dark mode (~1.46:1) and,
    additionally, the mix moves the color in the OPPOSITE luminance direction from dark mode
    (lighter/paler rather than darker/more neutral) — this is worth flagging for Test 10's still-open
    light-mode walkthrough as likely to reproduce (or read even less clearly, or arguably
    differently — a paler color against a white page could subjectively read as "washing out" even
    with a similar low numeric contrast) rather than being dark-mode-specific.

- timestamp: 2026-09-22
  checked: main.css's .report-row rule (border-left + background), and whether .vis-provisional
    overrides --severity-tint (the row's background wash) in addition to --severity-current
  found: .vis-provisional only overrides --severity-current and --badge-glyph-fg; it does NOT
    override --severity-tint. .report-row's background: var(--severity-tint, transparent) therefore
    stays the full-strength --color-severity-low-bg (a saturated dark-green tint, #123524 in dark
    mode) for a Provisional Low report, unchanged from a Live one.
  implication: On the FEED ROW specifically (not the map pin, which has no tint background — it
    sits on the basemap), the row's still-fully-green background wash acts as a competing visual
    anchor that reinforces "this still reads as green," working against the subtly-dimmed 4px
    border/badge's ability to register as a state change. This is a secondary, feed-row-only
    contributing factor layered on top of the primary (shared, both-surfaces) weak-contrast cause
    above — not itself sufficient to explain the map pin's identical failure, but likely why the
    row specifically reads as strongly unchanged.

- timestamp: 2026-09-22
  checked: web/css_contract_test.go for any existing automated test asserting a MINIMUM
    perceptual distance between .vis-provisional's fill and the undimmed severity fill (as opposed
    to the existing TestVisibilityCascadeOverridesAgeRamp, which only asserts the mix VALUE STRING
    matches .age-aging's, and TestBadgeGlyphContrastAcrossAgeStagesAndThemes, which asserts
    glyph-foreground-vs-fill contrast, not fill-vs-fill)
  found: No such test exists. The only guard on .vis-provisional's strength is the string-equality
    check against .age-aging's own value — which would pass unchanged even if .age-aging's own 50%
    ratio were independently discovered to be imperceptible, because it only checks the two values
    stay equal to EACH OTHER, never that either is perceptually distinct from the undimmed
    baseline.
  implication: This gap in test coverage is consistent with why the defect shipped past all
    automated gates and needed a human UAT pass to surface — nothing in the test suite could have
    caught a "technically-applied-but-too-subtle" defect of this shape.

- timestamp: 2026-09-22
  checked: map.js's buildPopupContent (the Leaflet popup box that opens on tapping a pin) vs.
    buildBadgeElement (the marker/pin icon rendered directly on the map, before any tap) and
    02-UI-SPEC.md's own target-element table for the Provisional treatment (line 82: "`.report-row`
    /pin badge gets the same desaturation mix ... | `.report-row`, map pin badge")
  found: buildBadgeElement (map.js:145-165) is what gets sev-*/age-*/vis-* classes and IS the
    severity-colored circular badge (rendered via L.divIcon directly on the map — this is "the map
    pin badge" the spec table names). buildPopupContent (map.js:170-207) — the separate DOM tree
    Leaflet opens in a popup box when the pin is tapped — contains only `.map-popup__meta` (plain
    text, no color/background rule), `.map-popup__description` (plain text), the visibility-tag
    chip (createVisibilityTag/updateVisibilityTag — this is what renders "Unconfirmed" inside the
    popup), and the vote-controls block. No element inside buildPopupContent carries a sev-*,
    age-*, or vis-* class, or reads --severity-current — there is no badge or border of any kind
    inside the popup box itself. 02-UI-SPEC.md's own Provisional row (line 82) names the target
    elements as `.report-row` and "map pin badge," NOT the popup content div; the popup markup
    section (02-UI-SPEC.md lines 155-171) never includes a badge in its template either.
  implication: The symptom's/UAT's phrase "map pin popup" is ambiguous between (a) the pin marker
    icon itself, visible on the map before/without tapping it — which DOES carry the same
    color-mix() dimming as the feed row, same root cause as above — and (b) the Leaflet popup box
    that opens on tap — which has NO severity-colored element at all, by design, per the UI-SPEC's
    own contract. The known_context's pixel-sampled "map pin badge" color is only explicable as a
    sample of (a) the marker icon, since (b) the popup box has no badge-colored pixel to sample.
    This means the primary root cause (weak contrast on the two real treated elements,
    `.report-row` and the marker/pin badge) fully explains the observed symptom without requiring
    any new badge/border element inside the popup box — but it is worth flagging explicitly so a
    gap-closure fix does not misread "map pin popup" as requiring a new badge inside
    buildPopupContent, which would contradict the UI-SPEC's own explicit target-element list.

## Resolution

root_cause: "NOT a code defect. .vis-provisional (trust.css:128-131) is applied to the correct DOM
  element on both real treated surfaces (feed.js:215 on the .report-row ancestor, map.js:157
  directly on the marker/pin's .icon-badge — these two are the exact elements 02-UI-SPEC.md line
  82 names for this treatment; the separate Leaflet popup box, map.js's buildPopupContent, has no
  badge/border element at all by design and was never meant to carry this treatment — see the
  popup-ambiguity evidence entry above), at the correct point in the cascade (trust.css loads after
  main.css in index.html.tmpl, so its --severity-current override wins over .age-fresh's at equal
  specificity, with no conflicting rule in any stylesheet that loads afterward), producing the
  color the code intends: a 50/50 color-mix() between the severity's base color and
  --color-age-stale — the exact same ratio, by deliberate design, as main.css's .age-aging rule (an
  INTERMEDIATE desaturation stage, not the strongest one). The root cause of the user's symptom is
  that this 50% ratio produces only ~1.9-2.0:1 WCAG luminance contrast against the undimmed color
  in dark mode (and an even weaker ~1.46:1, in the opposite/lightening direction, in light mode).
  There is no in-project requirement that state-vs-state contrast clear any specific ratio (WCAG's
  ~3:1 graphical-object floor is object-vs-background, not state-A-vs-state-B of the same object,
  so it is used here only as a borrowed heuristic, not a spec requirement) — the more direct,
  in-app reference point is that the SAME formula applied to this app's own strongest existing
  desaturation step, Fresh-to-Stale on the age ramp (0% to 100% mix), yields ~4.29:1, more than
  double Provisional's ~1.9-2.0:1 (dark) — i.e. Provisional's step is roughly half as strong as the
  step this app already ships and calls a real, presumably-noticeable state change elsewhere (NOTE:
  this UAT round did not separately test whether Stale itself reads as visibly faded, so treat that
  4.29:1 figure as this app's own reference point, not as independently validated). A Provisional
  report also has no live report immediately adjacent to serve as a side-by-side reference (unlike
  the age ramp, which a user learns by watching one report's own border drift over its lifetime),
  so a sub-2:1 shift reads as 'still basically the same color' at a
  glance. A secondary, feed-row-only contributing factor: .vis-provisional overrides
  --severity-current and --badge-glyph-fg but not --severity-tint, so the row's background wash
  stays the full-strength severity tint, visually reinforcing 'this still looks green/normal' even
  as the border/badge itself shifts. No test in web/css_contract_test.go asserts a minimum
  perceptual distance between the Provisional fill and the undimmed fill (the only existing guard,
  TestVisibilityCascadeOverridesAgeRamp, checks Provisional stays string-equal to Aging, never that
  either is far enough from Live) — this gap in coverage is why the shortfall shipped past every
  automated gate and needed a human UAT pass to surface."
fix: ""
verification: ""
files_changed: []
