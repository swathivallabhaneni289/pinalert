---
sketch: 001
name: map-pin-popup
question: "Does the seven level hierarchy (badge, title, meta, trust, signals, actions, comments) scan quickly in a 300px Leaflet popup on a phone, in light and dark?"
winner: "d4"
tags: [popup, leaflet, trust-block, comments-entry, light-dark]
---

# Sketch 001: Map pin popup

## Design Question

When someone taps a map pin, the popup must show the report in this exact order: a small category badge,
the title, a meta line, the trust block, the two signals, the three actions, and a restrained comments
entry. It has to stay easy to scan at about 300px wide on a phone, feel like an expanded version of the
existing feed row, and look identical in intent in light and dark.

## How to View

```
open .planning/sketches/001-map-pin-popup/index.html
```

Needs a network connection: Leaflet, MapLibre and the OpenFreeMap basemap load from the same CDNs the
app uses. Light and dark render side by side in phone sized frames, so the app's real media queries run.
Under each frame a live check reports whether the design rules hold on what the browser painted.

## Variants

The two variants differ in one thing only, the header. Both default to the neutral Comments strip and the
outlined Mark resolved, and blocks are grouped by whitespace with no dividers.

- **A: Row header.** The badge sits beside the title, exactly as in the feed row. Title and meta share a
  narrower column, so long reports clip sooner. The meta line drops below the header at the full width.
- **B: Stacked (recommended).** The badge stands alone on the first line, as the owner's list orders it,
  and the title, meta and everything below use the full 265px. In 135 real category, time and distance
  strings the meta wraps 15 times here against 89 in A, and a 100 character report shows whole instead of
  clipped.

Two switches on the board show the alternatives: the Comments row on the tint (under a hairline) instead of
a neutral strip, and the filled Mark resolved (the existing inverted style) instead of outlined. The strip
sits on the plain surface, so its muted count keeps 4.5:1 and it reads as navigation. Outlined matters
because the filled button measures 14.4:1 against the tint while Confirm and Dispute measure 1.2:1, so the
closing action would be the loudest control in the popup.

- **C: Composed (added after the owner said the popup felt like one long stack of details).** Four zones
  instead of a list: the row header of A, a two column trust block (votes on the left, signals on the right,
  plain text), one row of actions with Mark resolved as a quiet text action beside Confirm and Dispute, and a
  one line Comments footer. 274px tall on the sample against 343px for B. The word Comments stays in the link
  for screen readers but is not drawn, since "2 comments" and "View comments" already say it. C has passed the
  rules check in all nine report states in both themes and engines, but has not had an independent review.

- **D4: From your reference (winner).** Built from the picture the owner posted, and the design that was chosen.
  A severity label (a fully rounded pill) sits beside the category icon, every detail line has an icon, the two
  counts and the two signals are split by thin vertical lines, and the whole trust block is one button that opens
  a four sentence panel. The three actions are outlined buttons with icons, and the footer is a comments row with
  an icon and an arrow. The card is up to 340px wide, neutral, with 12px corners. Everything works in the sketch.
  `HOW-IT-WORKS.md` describes the behaviour and `variants/d4.css` and `variants/d4.js` hold the source. It passes
  the rules check in all nine report states in both themes and both engines, with the radius exemption listed
  under Decision below.

A report state menu covers the owner's sample plus eight more: too early to tell, gone quiet, unconfirmed
shelter, contested, the longest category label, your own report, too far to vote, and long text with 99+.

The independent reviews and the build checklist are in `.planning/sketches/REVIEWS.md`. No image files are
kept: open the review board to see every view.

## What to Look For

- Can you read the seven levels in order in about two seconds?
- Does the trust block read as plain text, and does Comments stay quieter than the emergency information?
- At 320 wide and in the long text state, does anything wrap badly or get clipped?
- Tap Confirm, Dispute and Mark resolved, then View comments.

## Constraints That Shaped It (target stack)

Leaflet 1.9.4 ships popup defaults that all break the design system. The mockup carries the overrides
production needs, each written with the `.leaflet-container` ancestor so it out ranks `leaflet.css` on
specificity, the discipline `trust.css` already documents:

- 13px Helvetica Neue text and a 13/24/13/20px content margin: replaced by the system stack, 14px and no margin.
- `.leaflet-popup-content p { margin: 1.3em 0 }`: beats any class only margin and explains loose paragraph
  spacing. Paragraphs get no margin and every gap is a flex gap on the spacing scale.
- `.leaflet-container a { color: #0078A8 }`: turns the Comments link blue, so the row names its own tag
  and ancestor to stay grayscale.
- A 24px close button (under the 44px floor), a 12px radius and a 14px 40% shadow: replaced by a 44px
  button, an 8px radius and the report button's existing `0 2px 8px` shadow.
- A filled Mark resolved ties with `.vote-btn:disabled` on specificity and wins on source order, so a
  disabled Mark resolved still looks pressable. An extra rule fixes it.
- The popup is rebuilt on every 30 second poll (`setPopupContent`), so nothing here may hold state in
  the DOM that a rebuild would lose.

## Verified

- 200 renders across both sketches pass the in-browser rules check in Chromium and WebKit, run fresh after
  the review fixes: all eight report states, both variants, both themes, popup widths 320, 360 and 412 and
  report view widths 320, 360 and 1100. The measured popup width is 301px (289px on a 320px phone).
- A scripted walkthrough passes in both engines: votes change the plain text counts, the resolve
  confirmation flows, View comments opens sketch 002 with the thread, a posted comment joins the end, Back
  to map returns.
- 28 measured checks pass in both engines and both themes: pressed Confirm and Dispute text is 12 to 15:1,
  the comment box edge is 4.83:1 light and 7.66:1 dark, disabled buttons name their reason, the signals
  stack cleanly, and a 320px popup keeps a 16px gutter.
- Independent reviews: the spec audit found no blocker and no major issue against the owner's 20
  requirements. The accessibility audit ran axe-core 4.10.2 in 96 configurations and found zero violations
  in the popup and report view.
- After the counts tweak, D4 was re-run headless in WebKit and Chromium across all nine report states in both
  themes (18 renders in each engine): the counts row is regular weight, the numbers inside it are semibold, and
  the rules check reports no violations.

## Decided in the Sketch

- The separators in the meta line and the signals line are drawn in the gap and clipped when a segment
  wraps, so no dot or bar is stranded. The typed text is still exactly the owner's strings.
- Pressed Confirm and Dispute keep the severity hue on the border and fill and put the text on the main
  colour (the existing pressed text is 3.01:1 on the light tint).
- On a 320px phone the popup is 288px wide so it keeps a 16px gutter each side.
- The Comments label is regular weight so only "View comments" carries weight.

## Decision

Winner: **D4, From your reference**, the design the owner built from their own reference image (the board tab
"D: From your reference"). Variants A, B and C are kept for the record. One tweak was made when it was chosen:
in the counts row the number is semibold and the word regular (both had been semibold).

D4 departs from decisions already locked for Phase 3 in `03-CONTEXT.md` and `03-VALIDATION.md`. The UI contract
should record each one as an explicit amendment:

- The severity label is a fully rounded pill (`999px`) and the card has 12px corners. V-40 forbids `999px` in
  trust CSS and the site design rules ban pill shapes, so this needs an owner exception for labels. The sketch
  checker allows it for D4 only (`maxRadius: 12`, `.d4-chip` exempt).
- The words in the signals row are quiet grey where D-18 says normal text colour. The counts row stays in the
  normal text colour. Grey passes contrast on D4's plain surface but not on the tinted feed row.
- The trust block is one button that opens a four sentence panel. D-19 gives each counter, word and tag its own
  hover, tap and focus reason (V-39 tests that). D-20 says the block is identical on the feed row, the popup and
  the Activity page, so the row and the Activity page need a decision too.
- The popup is up to 340px wide where the design brief said about 300, and its surface is neutral, which undoes
  quick task 260923-mb0 (the severity tint on the popup).
- The too far sentence is always visible under the buttons, where D-22 shows the reason on hover, tap or focus.
- The footer is a comments row, but comments have no phase, table or endpoint yet. Until the report page and
  comments phase ships, the footer needs a defined state (hidden, or shown without a made up count).

## Open Decisions

`REVIEWS.md` (2026-09-30) was written before D4 existed. Its decision 1 (header) is replaced by D4's own header.
Decisions 2, 3 and 7 (Mark resolved weight, title source, longest category label) and the forced colours part of
decision 8 still need an answer in the Phase 3 UI contract. Decisions 4 to 6 (report view, numbers on it,
composer) belong to the report page and comments phase.

## Handoff for the Build

The full ordered checklist (owner gate, backend, CSS, JavaScript, accessibility) is in
`.planning/sketches/REVIEWS.md`. The three things to know first: comments and the report page have no phase
and no backend yet, the trust numbers need the Phase 3 backend before they can be shown honestly, and
`map.js` must stop rebuilding the open popup on every 30 second poll.
