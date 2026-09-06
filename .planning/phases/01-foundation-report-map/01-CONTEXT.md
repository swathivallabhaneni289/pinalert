# Phase 1: Foundation — Report & Map - Context

**Gathered:** 2026-09-05
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers the base reporting loop as a Walking Skeleton (first phase, new project,
MVP mode): a visitor can submit a location-tagged emergency report and see it — alongside other
nearby reports — on a live map, without creating an account. Covers FOUND-01 through FOUND-06,
OPS-01, OPS-02. Confirm/dispute, the trust mechanic, and coordination features are later phases —
this phase is submit → store → view only, done as a real thin end-to-end slice.

</domain>

<decisions>
## Implementation Decisions

### Report categories (scope correction — also update PROJECT.md/REQUIREMENTS.md)
- **D-01:** Expanded from the originally-scoped 5 flood/cyclone-specific categories to **9
  categories**: Flood, Earthquake, Fire, Storm/Cyclone damage, Road blocked, Power outage,
  Shelter open, Rescue needed, Other. Rationale: Pinalert is pitched as a general local emergency
  feed, not flood-only, and the original list under-covered that. This changes REQUIREMENTS.md
  FOUND-02's category enum and PROJECT.md's "Report categories" line — the orchestrator should
  update both after this CONTEXT.md is committed, not just this phase's plan.

### Report submission flow
- **D-02:** Location is set by GPS pre-filling a draggable marker on the map; the visitor can
  drag to correct. Not tap-only, not GPS-locked.
- **D-03:** The submission form is a modal that opens over the map (not a separate page) —
  visitor never loses spatial context of where they're reporting.
- **D-04:** Category selection is a 3×3 icon grid, single tap. "Other" gets a generic flag icon.
- **D-05:** Severity (low/medium/critical) is set via a **slider**, not buttons — styled with a
  smooth animated transition as it moves, and each position shows both a number and a label
  ("1 · Low", "2 · Medium", "3 · Critical") so the level is unambiguous at a glance. Accessibility
  is non-negotiable given the use case: large touch target, high contrast, keyboard/screen-reader
  operable — not a visual-only gradient people have to guess at.

### Landing view & layout
- **D-06:** Split view — map and list side-by-side on wider screens.
- **D-07:** On narrow/mobile screens, defaults to map view with a one-tap toggle to reveal the
  list (matches the product's spatial identity; list is one tap away, not the default).
- **D-08:** List is sorted severity-first (critical always on top), then newest-first within each
  severity band. Not pure distance, not pure chronological.
- **D-09:** "Submit a new report" is a floating action button, persistent over the map, always
  reachable in one tap regardless of scroll position.

### Visual tone
- **D-10:** Overall visual direction is **neutral utility** — grayscale/minimal base, not an
  "alarm" aesthetic (no bold red/orange branding) and not a heavily-branded consumer look either.
  Reads as a calm, credible monitoring tool. Color is reserved almost entirely for severity
  signaling, not decoration.
- **D-11:** Severity color mapping is traffic-light: green (low) / amber (medium) / red
  (critical).
- **D-12:** Category icons (grid + map pins) use simple, minimal glyphs (Lucide/Feather style),
  refined per the approved theme reference (see `assets/01-theme-reference.png`): rendered as a
  white glyph on a solid color-coded circular badge (map-pin/marker pattern), not a bare colored
  line icon on a neutral background. Badge color follows category or severity context as shown in
  the reference.
- **D-13:** Dark mode ships from Phase 1, built with CSS variables from the start (not retrofitted
  later). Directly relevant use case: checking the feed during a power outage at night. Dark mode
  is a genuinely distinct palette (not an inverted light theme) — see reference image.
- **D-16:** Report list rows encode severity with a colored left border accent + a subtly tinted
  row background (e.g., critical = red accent + faint red-tinted background), on top of the
  numbered severity label — approved per the theme reference image.
- **D-17:** The expiry fade (D-15) is implemented as **desaturation toward gray** as a report ages
  (Fresh → Aging → Stale), not literal opacity reduction — confirmed against the theme reference's
  "Age ramp" panel. Reads more clearly as "this is going stale" than plain fading.

### Expiry behavior
- **D-14:** Default expiry is two-tiered by **severity**, not category: Critical severity (any
  category) = 24 hours; Low/Medium severity = 8 hours. This is a flat default for Phase 1 only —
  per-category or trust-weighted tuning is expected once confirm/dispute exists (Phase 2/3) to
  extend a report's life via re-confirmation.
- **D-15:** As a report approaches its expiry time, it visually fades (decreasing opacity) over
  roughly the last 25% of its lifetime, rather than disappearing abruptly at the cutoff. This
  reinforces the "reflects what's true right now" framing from PROJECT.md's Core Value, and once
  Phase 2 ships, gives a visual cue that a fading report needs re-confirmation.

### Claude's Discretion
- Exact geohash/session-identity storage mechanism (cookie vs. localStorage) — pick whichever is
  more robust for anonymous, no-signup persistence; not discussed as a user-facing decision.
- Exact CSS variable naming/theming implementation for dark mode — implementation detail.
- Exact icon set/library choice within "simple outlined line icons" (e.g., Lucide vs. Feather vs.
  Heroicons outline) — visually equivalent for this decision, pick one and be consistent.
- Precise fade curve/easing for expiry and severity-slider transitions — "smooth" was specified,
  exact timing/easing function is an implementation detail.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level context and locked decisions
- `.planning/PROJECT.md` — Core Value, Active requirements, Constraints, Key Decisions (session
  identity, rate limiting, hosting, stack corrections)
- `.planning/REQUIREMENTS.md` — v1 requirement IDs this phase covers (FOUND-01..06, OPS-01,
  OPS-02) — **note the category enum in FOUND-02 is stale per D-01 above and needs updating**
- `PROJECT-NOTES.md` (repo root) — full original feature/tech plan and data model sketch

### Visual reference
- `.planning/phases/01-foundation-report-map/assets/01-theme-reference.png` — user-approved
  light/dark theme reference. Canonical visual target for severity colors, icon-badge treatment,
  list-row severity accents, and the expiry desaturation ramp. See `<specifics>` below for what
  it does and does not cover.

### Architecture and stack (from ecosystem research)
- `.planning/research/ARCHITECTURE.md` — component boundaries, the `VisibilityResolver` pattern
  (not fully exercised until Phase 2, but the data model built in this phase — timestamptz,
  append-only vote log shape — must not preclude it), bounding-box+Haversine query pattern,
  read-time expiry predicate requirement
- `.planning/research/STACK.md` — exact library choices: `go-chi/chi/v5`, `jackc/pgx/v5` +
  `sqlc`, `mmcloughlin/geohash`, Leaflet 1.9.4 + OSM tiles via CDN
- `.planning/research/PITFALLS.md` — Pitfall 1 (naive full-table Haversine scan — must use
  indexed bounding-box prefilter from the first migration) and Pitfall 5 (timezone/expiry bugs —
  use `timestamptz`, not bare `timestamp`) both apply directly to this phase's schema work

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None yet — this is the first implementation phase of a greenfield project. No prior code,
  components, or patterns exist to reuse.

### Established Patterns
- None yet.

### Integration Points
- N/A — this phase establishes the foundation other phases integrate with (anonymous session
  identity, reports schema, map rendering) rather than integrating with anything existing.

</code_context>

<specifics>
## Specific Ideas

- Severity slider should feel "a little creative" but must stay clearly accessible — the user
  explicitly does not want a purely decorative slider that sacrifices clarity; numbers + labels
  are mandatory, not just a color gradient.
- The map+list split view and the "neutral utility, color only for severity" visual direction were
  chosen deliberately to read as a credible monitoring tool rather than an alarmist or heavily
  branded consumer app — this framing should inform visual choices throughout, not just this
  phase.
- **Approved visual theme reference:** `assets/01-theme-reference.png` (light/dark side-by-side
  preview) is the canonical visual target for the whole product, not just Phase 1. It shows: the
  traffic-light severity palette applied to numbered labels and category-badge pins; white glyph
  icons on solid color-coded circular badges; report-list rows with a colored left-border accent
  + tinted background keyed to severity; and an "Age ramp" panel demonstrating the expiry fade as
  color desaturation (Fresh → Aging → Stale) rather than opacity reduction. Only 4 of the 9
  category icons are shown in the reference (fire, shelter, flood, one unlabeled) — the remaining
  icons (earthquake, storm/cyclone, road blocked, power outage, rescue needed, other) need to be
  designed in the same style, not re-litigated.
  **Scope note:** the reference mockup shows "Confirmed by N nearby" and "Unconfirmed · expires
  soon" states, which are Phase 2 (Trust Engine) behavior — Phase 1 has no voting yet, so its
  actual shipped screens show reports without confirm counts. The reference is the target for the
  product's visual system across phases, not a literal Phase 1 screen mockup.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 1 scope. (Per-category expiry tuning, trust-weighted expiry,
and confirm/dispute-driven fade behavior are already correctly scoped to Phase 2/3 per ROADMAP.md,
not raised as new scope here.)

</deferred>

---

*Phase: 01-foundation-report-map*
*Context gathered: 2026-09-05*
