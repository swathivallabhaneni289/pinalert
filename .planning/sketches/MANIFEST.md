# Sketch Manifest

## Design Direction

Pinalert reads as a serious emergency utility: neutral, restrained and credible, built for someone
checking a situation quickly on a phone. Not a social feed, not a dashboard, not a startup landing
page. Grayscale base, colour reserved for severity, the existing four type sizes and two weights, the
existing spacing scale, modest corners and 44px targets. Trust information is plain text. The map popup
is a summary of one report, and comments live on a larger report view, never inside the popup.

## Reference Points

- The existing feed row (`web/static/css/main.css` `.report-row`, `web/static/js/feed.js` `createRow`).
  The popup and the report view are expanded versions of it.
- `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-DESIGN-BRIEF.md` and
  `03-CONTEXT.md` D-16 to D-20: the trust block wording and rules.
- The Activity page (`web/templates/profile.html.tmpl`, `.profile-page`, `.profile-back-link`) as the
  frame for a full page view.
- Explicitly not: social media cards, dashboards, startup landing pages.

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | map-pin-popup | Does the seven level hierarchy (badge, title, meta, trust, signals, actions, comments) scan quickly in a 300px Leaflet popup on a phone, in light and dark? | d4 (From your reference) | popup, leaflet, trust-block, comments-entry, light-dark |
| 002 | full-report-view-and-thread | What does View comments open, and how does a plain chronological thread look? | none yet, waits for the report page and comments phase | report-view, comments, thread, layout |

Sketch 001 is closed: D4 won. Its README lists where D4 departs from the locked Phase 3 decisions. Sketch 002
stays open until the report page and comments phase is scoped.

## Tooling

- `themes/default.css` pulls the real design tokens from `web/static/css/main.css` by reference, so a
  sketch cannot drift from the app. It redeclares nothing.
- `shared/sketch-shared.js` holds the sample reports and the DOM builders both sketches use, so the
  popup and the report view render the same report block.
- `shared/rules-check.js` is an in-browser checker. It reads computed styles and reports any font size
  outside 14, 16, 18, 24, any weight outside 400 and 600, spacing off the scale, radius over 8px
  (circles excepted), gradients, heavy shadows, targets under 44px, text under 4.5:1, colour that is not
  a token or is severity colour on the wrong report, long dashes, emoji, overflow, and the specified top
  to bottom order. Each review board shows the result live under every frame.
- Each `index.html` is a review board. Each `stage.html` is one rendered frame, driven by the query
  parameters `theme`, `variant`, `state`, and for sketch 001 `comments` and `resolve`.
- `REVIEWS.md` holds the four independent reviews, what changed because of them, the open decisions and the
  ordered build checklist.
- The sketches load Leaflet, MapLibre and the OpenFreeMap basemap from the same CDNs the app uses, so
  they need a network connection. All names, numbers and comment text in them are sample data.
