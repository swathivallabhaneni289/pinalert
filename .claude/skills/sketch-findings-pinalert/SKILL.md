---
name: sketch-findings-pinalert
description: Validated design decisions, CSS patterns and visual direction from the Pinalert sketches, covering the map pin popup and its trust block (variant d4, built from the owner's reference). Load during UI work on report popups, feed rows and the Activity trust block.
---

<context>
## Project: pinalert

Pinalert reads as a serious emergency utility: neutral, restrained and credible, built for someone checking a
situation quickly on a phone. It is not a social feed, a dashboard or a startup landing page. The base is grayscale,
colour is reserved for severity, and the existing four type sizes (14, 16, 18, 24), two weights (400, 600), spacing
scale (4, 8, 16, 24, 32, 48, 64), modest corners and 44px targets all stay. Trust information is plain text. The map
popup is a summary of one report. Comments live on a larger report page, never inside the popup.

Reference points: the existing feed row (`web/static/css/main.css` `.report-row`, `web/static/js/feed.js`
`createRow`), `03-DESIGN-BRIEF.md` and `03-CONTEXT.md` D-16 to D-20 for the trust block wording and rules, the
Activity page (`web/templates/profile.html.tmpl`) as the frame for a full page view, and the owner's own reference
image for the popup (not stored in the project, by the owner's rule of saving no images). Explicitly not: social media
cards, dashboards, startup landing pages.

Sketch sessions wrapped: 2026-10-03 (sketched 2026-09-30 and 2026-10-01).
</context>

<design_direction>
## Overall Direction

The popup is a neutral 12px card with the severity colour kept to the left bar, the badge and a severity pill. Top to
bottom it reads: badge row, title, meta, the trust block, three outlined actions, and a comments footer. The trust
block is two rows of plain text inside one button that opens a short panel of sentences, so the surface stays quiet
and the reasons are one tap away. Reasons that apply (such as being too far to vote) are visible text, never hidden
inside a greyed button. Colour carries severity only, state is never colour alone, and every target is at least 44px.

This is a mockup. Names, numbers and sentences in the sketch are sample data. Where the sketch departs from decisions
already locked for Phase 3, the departures are listed in `references/trust-block.md` under "Departures from locked
Phase 3 decisions", and the Phase 3 UI contract must record each as an explicit amendment.
</design_direction>

<findings_index>
## Design Areas

| Area | Reference | Key Decision |
|------|-----------|--------------|
| Popup structure | references/popup-structure.md | Neutral 12px card up to 340px, severity bar, pill label, outlined actions with an inline resolve confirmation, one link comments footer, plus the build requirements the reviews found. |
| Trust block | references/trust-block.md | Counts row and signals row inside one disclosure button that opens an inline panel of sentences. Number semibold, word regular. Nine documented departures from locked Phase 3 decisions. |

## Theme

The winning theme file is at `sources/themes/default.css`. It pulls the real design tokens from
`web/static/css/main.css` by reference and redeclares nothing, so a sketch cannot drift from the app.

## Source Files

Original sketch files are preserved in `sources/`. Open `sources/001-map-pin-popup/index.html` through a local
server (Safari blocks file:// pages), with the repo root as the server root so `../../../web/static/` resolves.
`sources/REVIEWS.md` is the ordered build checklist from four independent reviews (spec, design, accessibility,
production fit).
</findings_index>

<metadata>
## Processed Sketches

- 001-map-pin-popup (winner D4)

Not processed: 002-full-report-view-and-thread (no winner; it waits for the report page and comments phase, then a
later wrap-up can pick it up).
</metadata>
