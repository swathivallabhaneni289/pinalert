# Sketch Reviews, 2026-09-30

Four independent read only reviews of sketches 001 (map pin popup) and 002 (full report view and thread):
spec compliance, design critique, accessibility, and production fit. This file keeps the findings the
build needs. Everything in the first two sections is already applied to the sketches. The rest is the
handoff for the build.

## Outcome

- **Spec:** no blocker and no major issue against the owner's 20 requirements. Extras the spec did not
  ask for (Unconfirmed chip, too far note, four line title clamp, Reporter label, composer) were judged
  defensible and are flagged as decisions below.
- **Design:** ranked B (stacked header, neutral strip, outlined Mark resolved, no hairlines) first, the row
  header second. Measured on 135 real category, time and distance strings from the app's own labels, the
  row header wrapped the meta line on 89 and the stacked header on 15, and a 100 character report was
  clipped in the row header and shown whole in the stacked one.
- **Accessibility:** axe-core found zero violations in 96 configurations. Two inherited blockers and
  several build behaviours are listed below.
- **Production fit:** the proposed CSS ports with one token fix. The real cost is backend and JavaScript.

## Applied to the sketches

- Header: A and B now differ only in the header. Both default to the neutral strip and outlined Mark
  resolved. B, the recommendation, has no hairlines. Meta sits under the header at full width in both.
- Comments strip: label regular weight, only "View comments" is semibold and underlined, 48px row, no hover
  fill (it dropped the muted count under 4.5:1).
- Close button moved to line up with the badge row. Popup 288px on a 320px phone. Title uses balanced wrap.
- Meta separators and the signals separator are drawn in the gap and clipped when a segment wraps, so no
  dot or bar is stranded. Typed text is still exactly the owner's strings.
- Shelter capacity and headcount take their own meta line. A storm sample (longest label) was added.
- Pressed Confirm and Dispute keep the text on the main colour (3.01:1 became 12 to 15:1). Yes, mark
  resolved renders critical red as designed. A disabled Mark resolved looks disabled.
- Disabled buttons point at the too far sentence. Non breaking space keeps "1 km" together.
- Sketch 002: title uses the heading size on phones and in the panel, Post comment is a plain button,
  the comment box edge is 4.83:1 light and 7.66:1 dark, the count is a status region.

## Decisions for the owner

1. Header: stacked (B, recommended) or badge beside the title (A).
2. Mark resolved outlined in the popup and report view only, or everywhere including the feed row.
   Everywhere is consistent but reverses the Phase 2 decision in `trust.css` that made it primary weight.
3. Title source: a report has only a description (10 to 1000 characters). Keep it as the title with a
   popup clamp and full text in body weight on the report view, or add a short title field.
4. Report view: page (A) or side panel beside the map (B). They are identical at 360px. From 900px B keeps
   the pin in view but needs the three vendor scripts and a second map init on that page.
5. Numbers (82, 78) on the report view: Phase 3 decision D-09 limits them to the popup.
6. Composer in the first comments release, or read only.
7. May the longest category label ("Storm/Cyclone damage") wrap in the meta line? It now wraps cleanly.
8. The meta drops the severity word, so severity is bar, tint and a hidden "Critical." for screen readers.
   In forced colours mode the bar and tint vanish.

## Build checklist, ordered by dependency

**Owner gate (blocks the Comments row and sketch 002)**
- Comments have no phase, no table, no endpoint, no moderation code. `PROJECT.md` lines 167 to 170 defer
  them until moderation and free text abuse are scoped. Scope them as their own phase first. The popup
  can ship without the row and the rest of the hierarchy stands.

**Backend (Phase 3 first)**
- Confirmed and disputed counts, the still current word and score, the reporter tag and score, and the
  1 km vote rule do not exist yet. `BuildVoteTally` already computes `ConfirmCells` and `DisputeCells`
  (`internal/service/trust.go` 141 to 198) but `Nearby` and `CastVote` drop the tally. Add the Phase 3 trust
  object to `FeedReportResponse` and `CastVoteResponse` (D-21) and regenerate `docs/swagger.json`.
- Do not show a bare confirmed count before the 1 km rule: today "12 confirmed" could include voters on the
  other side of the planet.
- `distance_km` is measured from the map's first centre, not the reader's current position, and no distance
  formatter exists. Add one `formatDistance`, and drop the segment when the centre was the fallback city.
- A single report read and a gated `/reports/{id}` page (no store, one visibility authority, a defined answer
  for expired or retracted reports), and a comments migration, sqlc queries, one batched count inside
  `Nearby` (there is a query count test), a moderation hook, a per account rate limit, and an `is_reporter`
  boolean per comment instead of an account id on the wire.

**CSS (`web/static/css/trust.css` unless noted)**
- Declare `--shadow-popup: 0 2px 8px rgb(0 0 0 / 25%)` on `main.css` `:root`. `rgb(` in `trust.css` fails two
  contract tests, and the custom property allowlist forbids declaring it there.
- Flatten the review switches into one unconditional set of rules once the owner picks. Merge the close
  button rule into the existing one at `trust.css:469` (a test looks it up by exact head). The flattened tip
  rule must come after the combined rule at `trust.css:426` to win at equal specificity.
- Add `:not([hidden])` to every new display rule, as `trust.css:17` requires.
- Edit `trust.css:61` to `72` so pressed text is the main colour, and add the critical red rule for Yes, mark
  resolved with a head that beats `.vote-btn:not([hidden])` while keeping the existing rule.
- Assert font sizes, weights and no `!important` in the planned trust CSS test: no Go test does today.

**JavaScript (`map.js`, `votes.js`, `app.js`)**
- `map.js` rebuilds the popup on every 30 second poll (`setPopupContent`). That closes an open Mark resolved
  confirmation, drops keyboard focus and re-enables a busy Confirm. Build the parts once per marker and
  update text, classes and attributes in place, as `feed.js createRow` and `updateRow` do. Call `setLatLng`
  only when coordinates change.
- Contract tests pin the rebuild design (the description anchor order, the `setPopupContent(` literal). Edit
  those two in the same change and say why. Keep `className: popupStateClasses(report)` inline in `bindPopup`.
- Leaflet options: `popupAnchor: [0, -22]`, `minWidth` and `maxWidth` 300, autopan padding 16, popup
  content `max-width: calc(100vw - 2 * var(--space-md))`. Do not copy the sketch's `closeOnClick: false`.
- A height change (the resolve confirmation adds 74px) grows the popup upward with no autopan. Watch the
  content with a `ResizeObserver` and call `popup.update()`. Use `maxHeight` and `keepInView` so it scrolls
  at 400 percent zoom.
- Do not port the sketch's optimistic count bump: D-04 and `votes.js` forbid predicting a vote. The report
  view needs an after vote hook filled from the trust object, because `votes.js` ends every vote with
  `fetchReports()` for the feed.
- The too far pattern must be chosen once: the sketch uses native `disabled` plus a visible sentence with 1 km
  typed in. Phase 3 plans `aria-disabled`, a hover, tap and focus reason, and a radius from the page config.
- Back to map needs `/?report={id}` handling or `history.back()`. Today `no-store` drops the map from the
  back forward cache, so Back lands on a reloaded, recentred map with no popup.
- `report.html.tmpl` is checked by the theme, design rule and profile navigation tests the day it lands:
  follow `profile.html.tmpl` exactly and do not restyle `.profile-page` or `.profile-back-link`.

**Accessibility (existing defects this work touches, and behaviour Leaflet does not provide)**
- Every map pin is a focusable button with no name (WCAG 4.1.2). Set an aria-label after `addTo`.
- `closeResolveConfirm` hides the focused Cancel button and never restores focus.
- Popup: dialog role and name, focus moved in on open and back to the marker on close, Escape while focus is
  inside, Space on the close button, close button first in the DOM.
- Reduced motion: no Leaflet fade or marker animation, `setView` instead of `flyTo`.
- Forced colours: the glyph mask disappears. Add a forced colours rule.
- Disabled buttons need `aria-describedby` or `aria-disabled` so keyboard users can learn why.
