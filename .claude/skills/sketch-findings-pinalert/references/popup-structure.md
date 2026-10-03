# Map popup structure (variant d4)

## Design Decisions

The popup is a summary of one report, an expanded version of the feed row. Variant D4 won sketch 001 over A (row
header), B (stacked header) and C (composed zones) because it follows the owner's own reference image. Reading
order, top to bottom: badge row, title, meta, trust block (see `trust-block.md`), actions, comments footer. Colour
is used for severity only, and the card is otherwise grayscale.

| Part | Decision |
|---|---|
| Card | Neutral surface (`--color-bg`), 12px corners, `--shadow-popup`, up to 340px wide. A 4px severity bar runs down the left edge for the full height, footer included. One 16px gutter (`--space-md`) on both sides. On a phone narrower than 372px the card is the viewport minus 32: 328px on a 360 phone (329 measured), 288px on a 320 phone. |
| Header row | The category badge (the existing `icon-badge`, 20px glyph, light glyph on the severity circle in both themes), then the severity pill, then an optional neutral Unconfirmed pill. It keeps `--space-2xl` clear on the right for the close button. |
| Severity pill | A word, never colour alone: Critical, Medium or Low. Critical is the severity red on its tint (4.6:1 light, 5.0:1 dark). Medium and Low keep the word in the normal text colour, because their hue fails 4.5:1 as text. Their hue shows on the tint and on an 8px dot before the word. 14px semibold, 22px line height, fully rounded. |
| Close button | Leaflet's own anchor, restyled: 44px target centred on the badge row, its multiplication sign hidden and an X icon drawn as a mask, muted grey, fills on hover and focus. |
| Title | The report text. 18px semibold, clamped to 4 lines, wraps anywhere in a long word. 4px further in than the gutter, as in the reference. |
| Meta | The category glyph (16px, muted) beside lines of 14px muted text: category, how long ago, how far. A shelter adds its own line for capacity and headcount. Separators are drawn in the gap and clipped when a segment wraps, so no stray dot shows. |
| Actions | Under a hairline. Confirm and Dispute side by side, Mark resolved full width under them. Outlined buttons with an icon each (check, ban, shield-check), 44px tall, 8px corners, 16px semibold. Hover fills with `--color-border`. Pressed Confirm or Dispute is an inverted fill (text colour as the background), not a hue, and `aria-pressed` carries the state. Tap again to take a vote back, tap the other to move it. |
| Mark resolved | Asks first. The three buttons are replaced by "Mark this report resolved?" with Cancel (neutral, first in the DOM and on screen) and "Yes, mark resolved" (the app's critical red, wider). If the card is too narrow for both, Yes wraps under Cancel. Focus moves to Cancel, and after Yes it moves to the "Marked resolved." note. |
| Too far | All three buttons lose their fill and go grey, and a full contrast sentence sits under them: "Too far to vote. You need to be within 1 km of this report." The reason is always visible, never only inside a greyed button. |
| Your own report | No Confirm or Dispute, since you cannot vote on yourself. Mark resolved stays, full width. |
| Comments footer | The whole row is one link to the report page, under a hairline. A speech bubble, "Comments" over the count in muted grey, then "View comments" and a chevron on the right. It sits on its own surface in dark mode only, because muted text fails 4.5:1 on `--color-surface` in light. No comments reads "No comments" with "Add a comment". One comment reads "1 comment" with "View comment". |
| Motion | 150ms on background, border and colour, a 2px chevron nudge on hover. All of it is off under `prefers-reduced-motion`. |

States at a glance (all verified against the board's rules check in both themes and both engines):

| State | What shows |
|---|---|
| Critical | Red bar and badge, red Critical pill, both scores. |
| Unconfirmed | Low pill with its dot plus a neutral Unconfirmed pill, dimmed badge, only the reporter signal. |
| Too early to tell | The still current line reads "Too early to tell" with no number, the reporter reads "New reporter". |
| Gone quiet | Grey bar and badge, "Needs re-confirming" with a low number. |
| Contested | Confirmed and disputed both high, "Getting old" and a low reporter number. |
| Your own | Mark resolved only, full width. |
| Too far | All buttons disabled with the 1 km sentence under them. |
| Long text | Title cut at four lines, counts show 99+, the signals stack. Nothing overflows. |

## CSS Patterns

Every rule is scoped under `:root[data-variant="d4"]` in the sketch (drop that prefix in the app), and every rule that
touches Leaflet's own popup elements keeps the `.leaflet-container` ancestor so it out ranks `leaflet.css`. This is
the discipline `web/static/css/trust.css` already documents.

```css
/* Neutral card: the wrapper goes back to the plain page surface. */
.leaflet-container .leaflet-popup-content-wrapper {
  padding: 0;
  border-radius: 12px;
  overflow: hidden;
  background: var(--color-bg);
  box-shadow: var(--shadow-popup);
}
.d4-card {
  border-left: 4px solid var(--severity-current, var(--color-border));
  background: var(--color-bg);
  color: var(--color-text);
}
.d4-body { padding: var(--space-md); }
```

```css
/* Severity pill. The fully rounded radius needs an owner exception, see trust-block.md "Departures". */
.d4-chip {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 0 var(--space-sm);
  border: 1px solid transparent;
  border-radius: 999px;
  font-size: var(--font-size-label);
  line-height: 22px;
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}
.d4-chip--severity { background: var(--severity-tint, var(--color-surface)); }
.d4-chip--critical { color: var(--color-severity-critical); }
```

```css
/* Close button: Leaflet's anchor, X drawn as a mask in the muted grey. */
.leaflet-container .leaflet-popup a.leaflet-popup-close-button {
  top: 10px;
  right: var(--space-xs);
  display: flex;
  align-items: center;
  justify-content: center;
  width: var(--touch-target-min);
  height: var(--touch-target-min);
  border-radius: 8px;
  color: var(--color-text-muted);
}
```

```css
/* Title and actions. */
.d4-title {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  overflow: hidden;
  overflow-wrap: anywhere;
  font-size: var(--font-size-heading);
  line-height: var(--line-height-heading);
  font-weight: var(--font-weight-semibold);
}
.d4-card .vote-controls:not([hidden]) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: var(--space-sm);
}
.d4-card .vote-btn--resolve,
.d4-card .resolve-confirm { grid-column: 1 / -1; }
.d4-card .vote-btn--confirm[aria-pressed="true"]:not([hidden]),
.d4-card .vote-btn--dispute[aria-pressed="true"]:not([hidden]) {
  border-color: var(--color-text);
  background: var(--color-text);
  color: var(--color-bg);
}
```

```css
/* Footer: one link. Leaflet paints every link blue through .leaflet-container a, so the row names that ancestor. */
.leaflet-container a.d4-footer {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-height: var(--touch-target-min);
  padding: var(--space-sm) var(--space-md);
  border-top: 1px solid var(--color-border);
  background: var(--d4-footer-bg);
  color: var(--color-text);
  text-decoration: none;
}
```

The full stylesheet is `sources/001-map-pin-popup/variants/d4.css`.

## HTML Structures

```html
<div class="d4-card">                                  <!-- 4px severity bar on its left edge -->
  <div class="d4-body">
    <div class="d4-header">
      <!-- the existing icon-badge -->
      <span class="d4-chip d4-chip--severity d4-chip--critical">Critical</span>
      <!-- low and medium: class d4-chip--dotted and a leading <span class="d4-chip__dot" aria-hidden="true"> -->
      <span class="d4-chip d4-chip--neutral">Unconfirmed</span>      <!-- only when relevant -->
    </div>
    <p class="d4-title">Fire near Beach Road junction</p>
    <div class="d4-meta">
      <span class="d4-icon d4-icon--sm d4-icon--cat-fire" aria-hidden="true"></span>
      <div class="d4-meta__lines"><div class="d4-meta__line">Fire · 4 min ago · 0.6 km</div></div>
    </div>
    <div class="d4-trust">...</div>                     <!-- see trust-block.md -->
    <div class="d4-actions">
      <div class="vote-controls">Confirm, Dispute, Mark resolved, and the inline resolve confirmation</div>
      <p class="vote-note">the too far sentence, or the Marked resolved note</p>
    </div>
  </div>
  <a class="d4-footer" href="..." aria-label="View comments, 2 comments">
    <span class="d4-icon d4-icon--message-square" aria-hidden="true"></span>
    <span class="d4-footer__text"><span class="d4-footer__title">Comments</span><span class="d4-footer__count">2 comments</span></span>
    <span class="d4-footer__action"><span class="d4-footer__label">View comments</span><span class="d4-icon d4-icon--chevron-right" aria-hidden="true"></span></span>
  </a>
</div>
```

Icons are Lucide (lucide-static 1.41.0, ISC licence), the family the repo already uses, drawn as CSS masks so they take
the text colour like the category glyphs in `main.css`. The sketch builds the mask data URIs in JavaScript for
convenience. In the app they should be static files beside the existing ones in `web/static/icons/`.

## Build requirements that the sketch cannot show

These come from the four independent reviews (`sources/REVIEWS.md` has the full ordered checklist).

- `map.js` rebuilds the popup on every 30 second poll (`setPopupContent`). That closes an open Mark resolved
  confirmation, drops keyboard focus and re-enables a busy Confirm. Build the parts once per marker and update text,
  classes and attributes in place, as `feed.js createRow` and `updateRow` do. A refresh must never close an open popup,
  move focus or collapse an open explanation. If the report was resolved or expired, the card stays until closed.
- Leaflet options: `popupAnchor: [0, -22]`, autopan padding 16, popup content `max-width: calc(100vw - 2 * var(--space-md))`.
  Do not copy the sketch's `closeOnClick: false`. A height change (the resolve confirmation adds 74px) needs a
  `ResizeObserver` and `popup.update()`, plus `maxHeight` and `keepInView` so it scrolls at 400 percent zoom.
- Do not port the sketch's optimistic count bump. D-04 and `votes.js` forbid predicting a vote.
- Declare `--shadow-popup` on `main.css` `:root`. A bare `rgb(` in `trust.css` fails two contract tests.
- Add `:not([hidden])` to every new display rule, as `trust.css` requires.
- Accessibility Leaflet does not give you: every map pin is a focusable button with no name (set an aria-label after
  `addTo`), a dialog role and name on the popup, focus moved in on open and back to the marker on close, Escape while
  focus is inside, Space on the close button, the close button first in the DOM, no Leaflet fade or marker animation
  and `setView` instead of `flyTo` under reduced motion, and a forced colours rule because the glyph mask vanishes.
- Back to map needs `/?report={id}` handling or `history.back()`. Today `no-store` drops the map from the back
  forward cache, so Back lands on a reloaded, recentred map with no popup.

## What to Avoid

- Leaflet 1.9.4 popup defaults. They all break the design system: 13px Helvetica Neue text with a 13/24/13/20px
  content margin, `.leaflet-popup-content p { margin: 1.3em 0 }`, `.leaflet-container a { color: #0078A8 }` which turns
  links blue, and a 24px close button with a 12px radius and a 14px 40 percent shadow.
- The row header (A). Measured on 135 real category, time and distance strings, the meta wrapped 89 times beside the
  badge against 15 times in the stacked layouts, and a 100 character report clipped. D4 gives the title and meta the
  full width.
- Filled outlines on disabled buttons. A filled Mark resolved ties with `.vote-btn:disabled` on specificity and wins on
  source order, so a disabled button looks pressable. D4 uses outlined buttons and an explicit disabled rule.
- A comment count the backend cannot supply. Comments have no phase, table or endpoint yet. Until the report page and
  comments phase ships the footer needs a defined state (hidden, or shown without a made up number).
- Typing thresholds or the 1 km radius into a template or script. The radius comes from the page config.

## Origin

Synthesized from sketch 001 (map-pin-popup), winner D4.
Source files available in: `sources/001-map-pin-popup/`, `sources/shared/`, `sources/themes/`, `sources/REVIEWS.md`.
