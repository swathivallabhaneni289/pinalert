---
sketch: 003
name: trust-block-redesign
question: "With nothing underlined, the reporter words as plain text at the top of the report, a cue that is not an underline to say the block opens, and shorter panel wording, does the trust block still read at a glance on the map popup, the feed row and the Activity row, in light and dark, down to 188px of text and at 200 percent text size?"
winner: "H"
tags: [trust-block, popup, feed-row, activity-row, reporter-words, disclosure, wording, light-dark]
---

# Sketch 003: Trust block redesign

## Design Question

On 2026-10-08 the owner saw the built Phase 3 interface live in Safari and said:

1. "All of these are underlined, including confirmed, disputed and new reporter. I don't understand why we are
   underlining it. I don't like that part." (The whole trust block is one button, and `trust.css`
   `.trust-block__toggle:hover` underlines every word in it.)
2. "The new reporter thing should be on the top part of the box where Unconfirmed is located, because it doesn't look
   really nice under the confirmed and disputed one."
3. "Redesign it better, but don't use that tag things, underline things, it looks really odd."
4. "Everything else, the light green is already updated, I think it's fine." (The severity tint, the severity label and
   the Unconfirmed label stay exactly as they are.)
5. Earlier: "the text is too long." (Not yet known which text, so the wording is proposed separately below.)

This sketch answers: with no underline anywhere, the reporter words as plain regular text in the header zone (no chip,
no icon, no colour), a visible cue that is not an underline to say the block opens, and shorter panel sentences, does the
block still read at a glance on all three surfaces, in light and dark, and does it survive the narrowest widths
(288px popup, 236px feed row body, 188px Activity row body) and a 200 percent text size?

Held fixed, from the owner and from the project rules: colour is for severity only and trust text is plain; type sizes
14, 16, 18, 24 and weights 400, 600; the spacing scale; modest corners; 44px targets; no gradients, no emoji, no long
dashes, no purple; one tab stop with `aria-expanded` and no `aria-live`; hover and press are background fills only, hover
softer than press, press is the existing `var(--color-border)`; no transition longer than 150ms.

## How to View

Run a static server from the repository root and open the board in Safari. Pick any free port except 8080 and 8090:

```
python3 -m http.server 8795 --bind 127.0.0.1 --directory /path/to/pinalert
open http://127.0.0.1:8795/.planning/sketches/003-trust-block-redesign/index.html
```

No network is needed: there is no Leaflet, no CDN and no font download. The page also opens from `file://` in Chromium and
WebKit, but Safari blocks file pages, so use the server. The board keeps its choices in the address, for example
`index.html#variant=f&surface=row&state=4&width=320&open=1`.

- Tabs: Current d4 (as built), E, F, G.
- Light and dark sit side by side in frames as wide as the phone, so the app's real widths apply. Under each frame a live
  check reports whether the rules hold on what the browser painted.
- Surface: Map popup, Feed row, Activity row. Report state: nine realistic reports, the eight from the brief plus a
  stress state with 99+ counts, the longest labels and a long unbroken word.
- Phone width: 320, 360, or Wide (the two pane layout: popup 340px card, list pane 434px, Activity page 640px).
- Panel: closed or open. Show state: rest, hover, press, focus, so a still picture can show what a pointer or key does.
- Text size: 100, 150, 175 or 200 percent scales the four type size tokens, which is what a larger text size does to this app.
- Panel wording: proposed or current, so layout and wording can be judged apart. The new tabs start on proposed, the
  baseline on current.
- Unconfirmed label on rows (new tabs, rows only): under the meta as today, or beside the severity label.

Files: `index.html` is the board and `stage.html` renders one frame; `data.js` holds the nine sample reports and the two
wordings, `lib.js` builds the markup (the baseline copies `trust.js`, `map.js`, `feed.js` and the Activity page exactly),
`rules.js` is the live check, `sketch.css` is mockup chrome only, and `variants/` holds one stylesheet and one script per
tab plus `common.css` for what E, F and G share.

The surfaces link the real `web/static/css/main.css`, `feed.css`, `trust.css` and `auth.css`, so tokens, sizes and states
are the app's own. The Leaflet popup is a miniature stand in (its few default rules that `trust.css` has to out rank are
copied into `sketch.css`). The vote buttons are only there for context: Confirm and Dispute flip their pressed look and
never change a count, because the app never predicts a vote.

## Variants

The header and the button are independent. E and F differ only in where the reporter words go. F and G differ only in the
button. G uses F's header because F's words sit in one place (see the numbers below). Any header can be paired with any
button, since each is a few lines of CSS scoped by `data-header` and `data-button`.

| Tab | Reporter words | Trust button |
|---|---|---|
| Current d4 (as built) | Row two of the button, beside a user icon | Two rows, underlines on hover, no cue |
| E: Pill row words | Far end of the label row, under the labels when there is no room | Two rows, chevron |
| F: Header byline | Own line under the labels, left aligned with the title | Two rows, chevron |
| G: Compact line | As F | One wrapping line, chevron |
| H: Beside the label (the owner's choice) | Beside the state label, on the line it shares with it | Two rows, chevron |

**Current d4 (as built).** The block as the owner saw it: the whole two row button underlines every word on hover, and
the reporter words sit in its second row beside a small user icon. The only sign that it opens is that hover underline, so
on a phone there is none.
Weaknesses: every word underlines on hover and there is no cue without a pointer; the reporter words sit under the counts,
a long way from the labels; the panel sentences are the long ones (281 characters for the four lines of an Unconfirmed
report).

**E: Pill row words.** The reporter words sit at the far end of the label row in plain regular text, before the close
button in the popup, and drop under the labels, left aligned, when they do not fit. The button keeps the two counts on row
one and only the still current words on row two, with a chevron that turns when the panel opens.
Weaknesses: where the words sit depends on room (measured below), so one report looks different on different pages; an
Unconfirmed label, or a long tag with its popup number, pushes them under the labels; on a wide desktop row they sit at the
far right edge, hundreds of pixels from the label (about 360px on the 640px Activity page, about 200px on the 434px list
pane); and where they do reach the far end of the popup they stop 16px short of the close button's box at a 360px phone
(about 30px short of the X mark itself), so the words and the X can read as one unit, "New reporter x", which is close to the
tag look you asked us to avoid. In its favour, rows are the most compact (up to 28px shorter than today at 236px).

E is not the recommendation. It stays here as briefed, because it is the most literal answer to "on the top part of the box
where Unconfirmed is located" and the placement is easiest to judge by trying it. If you like how it looks on the feed list,
the option is E on feed rows only and F's byline on the popup and the Activity page: one rule per surface, and the words stay
away from the X. In `variants/e.css` that is one selector change (scope the `space-between` rule to
`[data-surface="row"]`) plus F's one line rule for the other two surfaces. E on the popup is then the same as F.

**F: Header byline.** The reporter words are a plain second line under the labels, left aligned with the title, so they sit
in the same place on every report and every surface. The button is the same two rows as E, with the chevron.
Weaknesses: it costs a line (between 4px shorter and 27px taller than today in the popup at 288px, between 5px shorter and
23px taller on a row at 236px); on a 288px popup with an Unconfirmed label the top of the card is three lines before the
title; two rows of trust text stay in the button on a live report.

**G: Compact line.** F's byline header, with the button flowing the counts and the still current words as one wrapping line
and a chevron at the end. It wraps to two lines when the width runs out, which on a phone is every time.
Weaknesses: one line never happens on a phone (measured below); where it does fit, rows in one list get different heights
depending on how long the still current word is; three kinds of text (16px counts, 14px words, a chevron) share one line.

**H: Beside the label (added 2026-10-09, the owner's choice).** The reporter words are plain text directly beside the state
label, on the same line as it. In the popup the state label and the words share a line of their own under the badge row; on the
feed row and the Activity row they share the line under the meta line, and a Live report, which has no state label, shows the
words alone on that line. They wrap under the label only when there is no room. The button is the same as E and F. H's own rules
are in `variants/h.css` and `variants/h.js`.

### Recommendation

*Written for E, F and G. The owner has since chosen H, see Owner decision at the end. F's reasoning about the words keeping one
place applies to H as well.*

**F with the proposed wording**, the chevron on, and the Unconfirmed label left where it is today on rows.

- The words are in the same place everywhere. F: 100 percent of cases by construction. E: at the far end in 8 of 8 row
  states at 236px, but in only 1 to 2 of 8 popup states at any popup width and 4 of 8 Activity states at 188px. A block
  that is meant to be identical on every surface (D-20) should not have two looks.
- The cost is small and mostly a trade: F adds a line where the words used to share row two, but it also removes the
  stacked second row that the two signals made on a phone, so rows end within -5px to +23px of today.
- G only compacts where there is room (a 350px row, the Activity page at desktop width), and rows in one list then differ in
  height. It is F on every phone width. It is a close second if the owner mostly uses the app on a wide desktop.
- The chevron is the one cue that works without a pointer, and it is not an underline.

### Questions for the owner

In everyday words, with a recommendation for each.

1. Which layout do you want for the reporter words and the button: E, F or G? **F.** The words always sit in the same
   place and the button is two short rows.
2. On the feed list and the Activity page, should the Unconfirmed label move up beside the severity word (Low, Medium,
   Critical), so it sits next to the reporter words the way it does on the map popup? **No, leave it where it is.** You
   asked to keep that label as it is, and on the narrowest Activity row the move makes the top of the box three lines tall.
   The switch on the board shows it either way.
3. Is the shorter panel wording right? Accept or reject it apart from the layout. **Accept.** It is about a third shorter
   and says the same things. If "Counts places, not votes." reads oddly, "Each place counts once." is the alternative.
4. If it is still too long, drop the two score lines in the popup panel? **Not yet.** They explain the two small numbers,
   and the rest of the cut already helps.

### Decisions made inside the sketch

1. **Chevron on E and F too.** The shipped cue that the block opens is the hover underline, which a phone never shows. The
   chevron works on touch, on keyboard focus and in a screenshot, it states the open or shut state (down shut, up open),
   and it is plain: the normal text colour, 20px, 2px stroke, no fill. All three new tabs have it, at the right of the
   button, centred against one row or two. At a large text size it moves under the text, right aligned, when less than 6.85em is
   left beside it (see "Counts never split" under Measured). It turns over in 150ms and not at all under reduced motion. The
   app has no `chevron-down.svg` yet (implementer note below).
2. **Hover and press.** Press is the shipped `var(--color-border)`, unchanged. Hover is the border colour at half strength
   over whatever is behind it (`color-mix(in srgb, var(--color-border) 50%, transparent)`), so hover is always the softer
   step of the same fill, and a tinted row still shows its tint through it. The distances below are CIE76 delta E, measured
   in the browser. Around 2.3 is the smallest difference an eye notices side by side.
3. **Reporter words are plain.** Regular weight, the normal text colour, no border, background, icon, chip or pill. The
   popup number stays after the words behind the same visually hidden ", score" prefix, in the same regular weight (the
   rule "numbers semibold" is kept for the still current number in the button). They are static text in the top zone, not a
   control, so their sentence stays in the panel as its last line. A weakness all three share: the words are explained by a
   panel that another control opens.
4. **The Unconfirmed label stays where it is today on rows** (under the meta). The owner's remark was made looking at the
   popup, where the label is already in the header. The switch "Unconfirmed label on rows" shows the alternative, the label
   beside the severity word, which makes row and popup match but turns the top of a 188px Activity row into up to three lines.
   See the questions below.
5. **The toggle box is 4px wider on each side and its padding 4px deeper on the left**, so the words stay exactly where they
   are today while a fill or the inset focus ring gets room around them instead of hugging the text. The chevron and its gap
   take 24px beside the text, so the counts have 20px less room than today, until the chevron moves under the text and the
   counts get the whole width back.
6. **The clock icon stays** beside the still current words, as the owner left everything else alone. The user icon goes,
   because the reporter words carry no icon.

### Measured

All figures are the same in WebKit 26.6 and Chromium 153, panel closed, light theme.

**E: do the reporter words stay at the far end of the label row?** States with a reporter: 1 2 3 4 5 6 8 9.

| Width | Unconfirmed label under the meta (today) | Unconfirmed label beside the severity label |
|---|---|---|
| popup 288 | 1 of 8 | same |
| popup 328 | 2 of 8 | same |
| popup 340 | 2 of 8 | same |
| row 236 | 8 of 8 | 5 of 8 |
| row 276 | 8 of 8 | 7 of 8 |
| row 350 | 8 of 8 | 8 of 8 |
| activity 188 | 4 of 8 | 2 of 8 |
| activity 228 | 8 of 8 | 5 of 8 |
| activity 508 | 8 of 8 | 8 of 8 |

**G: do the counts and the still current words share one line?** States with a still current word: 3 4 5 6 9.

| Width | One line |
|---|---|
| popup 288, 328, 340 | 0 of 5 |
| row 236, 276 | 0 of 5 |
| row 350 (two pane layout) | 2 of 5 (Up to date, Getting old) |
| activity 188, 228 | 0 of 5 |
| activity 508 (desktop) | 5 of 5 |

**Counts that wrap.** A pair of counts wider than the room wraps onto two lines, the shipped fallback (A22). The chevron
costs 20px, so a pair between the new and the old room now wraps where it did not. In the nine samples that is only `99+
confirmed` and `99+ disputed` at a 288px popup; a pair such as `99+ confirmed` and `0 disputed` would also wrap on a 236px
row now. The natural widths are 186px for 7 and 1, 197px for 12 and 0, 205px for 12 and 10, 211px for 99+ and 0, 232px for
99+ and 99+. The room is 208px on a 236px row, 224px on a 288px popup and 200px on a 228px Activity row (it was 228, 244 and
220). At 188px the counts wrap for every state today and still do.

**Counts never split between number and word (repair pass).** The first version gave the chevron a column of its own, 24px at
every text size. At 200 percent text on the 188px Activity row that left 160px for a count that is 165px wide, so the number
sat on one line and its word on the next ("7", then "confirmed"), in 8 of the 9 sample states, where the shipped button keeps
"7 confirmed" whole. The button is now a wrapping flex row: the face asks for 6.85em, and the chevron moves under the text,
right aligned, when less than that is left beside the face. Because the test is the text size against the width and never the
report, the chevron sits in the same place for every report on one page. The census below counts, per surface and text size,
the counts that split although they would have fitted on a line of their own ("avoidable"), over the nine sample states, in
WebKit 26.6 and Chromium 153 (the same in both). It also ran on the version before the repair.

| Surface (text width) | Text size | Avoidable splits before the repair | After | Chevron under the text |
|---|---|---|---|---|
| Activity row (188px) | 150% | 0 | 0 | yes |
| Activity row (188px) | 175% | 2 counts, 1 state | 0 | yes |
| Activity row (188px) | 200% | 8 counts, 8 states | 0 | yes |
| Activity row (228px) | 200% | 1 count, 1 state | 0 | yes |
| Feed row (236px) | 200% | 1 count, 1 state | 0 | yes |
| Every other width tried (popup 288, 328, 340; feed row 276, 350; Activity 508) and the rows above at the smaller sizes | 150%, 175%, 200% | 0 | 0 | no |

A split that no layout can avoid is left as it is: a count wider than the whole button breaks at its space, as the shipped
button does. Only state 9 has any (`99+ confirmed` and `99+ disputed`, on the 188px Activity row at 200 percent). At 175
percent `99+ confirmed` is 183px and now fits the 184px of room, where the shipped button, with 180px of room, splits it. The
chevron line costs 24px (the 20px chevron and the 4px gap, measured) of height where it appears, and it never appears at 100 percent text, where the geometry is
identical to the first version (756 configurations per engine, every part of the block within 0.01px). The live check has a
new rule that fails any avoidable split.

**Height of the surface in px, panel closed** (current, then F; E is the same as F in the popup and up to 28px shorter on
rows):

| | popup 288 | row 236 | Activity 188 |
|---|---|---|---|
| Unconfirmed, 0 confirmed (state 1) | 392, 388 | 371, 366 (E 343) | 306, 302 (E 278) |
| Live, Up to date (state 3) | 356, 380 | 343, 366 | 290, 302 |
| Live, Getting old, medium (state 4) | 368, 380 | 355, 366 | 290, 302 |
| Critical, stress (state 9) | 392, 419 | 558, 569 | 547, 559 |

**Height with the panel open**, current wording on the current block then proposed wording on F, popup 288: state 1 589 to
545, state 4 710 to 623, state 6 729 to 623, state 9 753 to 663. The shipped popup scrolls inside itself at the viewport
height less 180px, so the proposed wording lets state 4 fit whole in an 844px tall viewport (664px of room) where the
current one does not.

**Hover and press fills**, measured in the browser (rest, hover, press, then delta E rest to hover, hover to press, rest
to press). Press is `var(--color-border)` in every row.

| Surface | Theme | Rest | Hover | Press | Rest to hover | Hover to press | Rest to press |
|---|---|---|---|---|---|---|---|
| Popup card (neutral) | light | #FFFFFF | #F0F2F4 | #E1E4E8 | 4.9 | 4.9 | 9.8 |
| Popup card (neutral) | dark | #0B0D10 | #1B1D21 | #2A2D31 | 7.2 | 7.7 | 14.8 |
| Row, Low tint | light | #E7F5EC | #E4EDEA | #E1E4E8 | 4.6 | 4.6 | 9.2 |
| Row, Low tint | dark | #123524 | #1E312B | #2A2D31 | 9.7 | 10.4 | 20.0 |
| Row, Medium tint | light | #FCF3DC | #EFECE2 | #E1E4E8 | 7.7 | 7.8 | 15.5 |
| Row, Medium tint | dark | #3A2C10 | #322D21 | #2A2D31 | 11.8 | 11.7 | 23.4 |
| Row, Critical tint | light | #FBE9E7 | #EEE7E8 | #E1E4E8 | 4.4 | 4.3 | 8.8 |
| Row, Critical tint | dark | #3A1512 | #322122 | #2A2D31 | 12.8 | 11.0 | 23.7 |
| Row, Provisional (surface colour) | light | #F1F3F4 | #E9ECEE | #E1E4E8 | 2.7 | 2.8 | 5.5 |
| Row, Provisional (surface colour) | dark | #16181C | #202327 | #2A2D31 | 5.2 | 5.0 | 10.1 |

The weakest row is the light Provisional one: rest to press is only 5.5, so hover and press are about 2.8 apart each. That
is the ceiling of the shipped press fill (`--color-surface` is the row colour there, A48), not of the hover. Both steps are
visible but subtle. A stronger step on that row would need a different press fill, which is an A48 amendment and is not
proposed here. In dark mode "lighter" is read as a smaller distance from rest, since the dark tints are close in luminance
to the press grey and differ mostly in hue. A hover fill using `color-mix` is not a bare token, so the sketch's colour token
rule is checked at rest and the hover and press fills are checked by their own rule against the border token.

## Wording proposals

Offered on their own, so they can be accepted or rejected apart from the layout. The board's Panel wording switch shows
either set in any layout. Every number stays real and every meaning stays. The new sentences stay inside the closed
character set of `TestTrustCopy` (ASCII letters, digits, space, comma, full stop, apostrophe, plus, a hyphen only inside
re-confirming), carry no relative time and need only inputs that already exist: N, M, R, the window phrase, K, the
threshold, C, J, the judged minimum and the score. The table covers every sentence form in `trustcopy.go`.

| Line | Case | Current (`trustcopy.go`) | Proposed | Chars |
|---|---|---|---|---|
| Lead | Unconfirmed label showing | Needs 2 confirmations before this counts as confirmed. | Needs 2 confirmations to count as confirmed. | 54 to 44 |
| Lead | Expired (Activity page) | This report has expired. It can no longer be confirmed. | Expired, so it can no longer be confirmed. | 55 to 42 |
| Confirmed | N is 0 | No one has confirmed this yet. Counts separate places, not votes. | No one has confirmed this yet. Counts places, not votes. | 65 to 56 |
| Confirmed | N is 1 | 1 person in 1 place confirmed this. Counts separate places, not votes. | 1 place confirmed this. Counts places, not votes. | 70 to 49 |
| Confirmed | N is 2 or more (12) | 12 people in 12 different places confirmed this. Counts separate places, not votes. | 12 different places confirmed this. Counts places, not votes. | 83 to 61 |
| Disputed | M is 0 | No one nearby has disputed this. Counts separate places, not votes. | No one nearby has disputed this. | 67 to 32 |
| Disputed | M is 1 | 1 person in 1 place disputed this. Counts separate places, not votes. | 1 place disputed this. | 69 to 22 |
| Disputed | M is 2 or more (4) | 4 people in 4 different places disputed this. Counts separate places, not votes. | 4 different places disputed this. | 80 to 33 |
| Still current | Up to date | 2 different places confirmed this in the last 2 hours. | 2 places confirmed this in the last 2 hours. | 54 to 44 |
| Still current | Getting old | Confirmed earlier, but fewer than 2 places confirmed it in the last 2 hours. | Fewer than 2 places confirmed it in the last 2 hours. | 76 to 53 |
| Still current | Needs re-confirming | Confirmed earlier, but it has gone quiet. People nearby can confirm it again. | Gone quiet. People nearby can confirm it again. | 77 to 47 |
| Still current | A standing dispute raised the bar (K = 3) | Someone nearby disputed this, so it needs 3 places to confirm it in the last 2 hours. | Disputed nearby, so it needs 3 places in the last 2 hours. | 85 to 58 |
| Still current | Too early to tell | Fewer than 2 places have confirmed this so far. | Fewer than 2 places have confirmed this. | 47 to 40 |
| Score (popup) | Still current score (99) | Score 99 out of 100, based on how recently different places confirmed this. | Score 99 out of 100, based on recent confirmations. | 75 to 51 |
| Reporter | Reliable, Mixed or Unreliable (7 of 9) | 7 of 9 earlier reports were confirmed by people nearby. | 7 of 9 earlier reports were confirmed by people nearby. | 55 to 55 |
| Reporter | New reporter, 3 judged | Only 3 of their reports have had a clear result so far. A rating needs 5 reports with a clear result. | A rating needs 5 reports with a clear result. They have 3 so far. | 101 to 65 |
| Reporter | New reporter, 1 judged | Only 1 of their reports has had a clear result so far. A rating needs 5 reports with a clear result. | A rating needs 5 reports with a clear result. They have 1 so far. | 100 to 65 |
| Reporter | New reporter, 0 judged | None of their reports has had a clear result yet. A rating needs 5 reports with a clear result. | A rating needs 5 reports with a clear result. They have none so far. | 95 to 68 |
| Score (popup) | Reporter score (78) | Score 78 out of 100, based on how often their reports with a clear result were confirmed. | Score 78 out of 100, based on how often their reports were confirmed. | 89 to 69 |

Across the nine sample reports a panel is 25 to 38 percent shorter (the four lines of an Unconfirmed report go from 281
characters to 200; the six lines of a popup for state 4 from 454 to 315).

What changes in meaning, said plainly:

- **The caveat appears once.** "Counts places, not votes." ends the confirmed line only. Today the tail "Counts separate
  places, not votes." ends both counter lines, so it is read twice. The word "separate" goes. If the shorter caveat reads
  oddly, "Each place counts once." says the same in plain words. Dropping the tail from the disputed line amends A56 and
  RR-1, which fixed both tails. The disputed line is still a count of places, and the caveat is one line above it.
- **"people in" goes** from the counter sentences. N is a count of places and a lower bound on people, so "12 different
  places confirmed this" is the exact claim and "12 people in 12 places" said it twice.
- **"Confirmed earlier, but" goes** from Getting old and Needs re-confirming. The confirmed line right above and the label
  already say it was confirmed; the line now says only what is different now.
- **The dispute sentence loses "to confirm it".** "Disputed nearby, so it needs 3 places in the last 2 hours." keeps both
  numbers and the reason.
- **The New reporter sentence states the rule once and the count once** ("A rating needs 5 reports with a clear result.
  They have 3 so far.") where it used two sentences that each said "a clear result".
- **The score lines lose their long tails.** If the panel is still too long, the two score lines are the next thing to cut:
  they only explain the two small numbers in the popup, and without them the popup panel for state 4 is four lines.
- Unchanged: the Too early to tell label, the rated reporter sentence, the toggle hint, every label and word.

## What to Look For

- Hover the trust block, press it, or use Show state. Is anything underlined anywhere? With no pointer, as on a phone, is it
  clear that the block opens? Is the chevron enough?
- Where do the reporter words sit, and do they read as plain words and not as a tag? Compare E and F on the popup and on a
  row, and look at E on a wide row.
- Open the panel and flip Panel wording. Is the proposed set short enough? Is anything now missing?
- The Provisional row (state 1 or 2, light): can you see hover and press? Dark mode, the Low and Medium rows?
- Activity row at 320 (188px of text), and Text size 150%, 175% and 200%: does anything clip, push sideways or split a count
  into "7" and "confirmed"? From about 145 percent the chevron moves under the text on this row. Does it still read as the end
  of the button?
- Use "Unconfirmed label on rows" on a Provisional row: is the label beside the severity word better than under the meta?

## Notes for the implementer

Everything below is for the build, once the layout is final. The owner chose H (see Owner decision at the end), so the notes are
written for H and say where E, F or G would differ. The sketch changes no app file. Function names, call sites and test pins were
read from the files cited, and nothing was run in the repository. The test failures were found by reading each test against the
markup and CSS the sketch draws; a reviewer's dry run on a scratch copy found the same ones for E, F and G.

**CSS, `web/static/css/trust.css`** (the sketch's `variants/common.css` is written as overrides of these rules, so drop the
`:root[data-redesign]` prefix and edit each in place):

- `.trust-block__toggle:not([hidden])`: `display: flex`, `flex-wrap: wrap`, `justify-content: flex-end`, `align-items: center`,
  `align-content: center`, `column-gap` and `row-gap` of `var(--space-xs)`, `width: calc(100% + var(--space-sm))`,
  `margin-inline: calc(-1 * var(--space-xs))`, `padding-inline: var(--space-sm) var(--space-xs)`. Keep the 44px floor, vertical
  padding, 8px corners and the 150ms transition. Drop `text-underline-offset`.
- `.trust-block__face:not([hidden])`: `display: block`, `flex: 1 1 6.85em`, `min-width: 0`. The 6.85em is load bearing: it is what
  moves the chevron under the text before a count is squeezed, so a plain `flex: 1` brings the "7" then "confirmed" split back at
  175 and 200 percent. (G only: its `display: flex` face rule, in `variants/g.css`.)
- `.trust-block__toggle:hover`: replace `text-decoration: underline` with
  `background-color: color-mix(in srgb, var(--color-border) 50%, transparent)`. Keep `:active` as `var(--color-border)` and
  after the hover rule. The focus ring stays inset.
- New: `.trust-block__chevron` (20px mask, `transform: rotate(180deg)` on `aria-expanded="true"`, a reduced motion rule and a
  forced colours rule) and `.trust-reporter`. H adds one state line per surface (`variants/h.css`). E, F and G add
  `.popup-card__pills`, `.report-row__top` and `.report-row__pills` (see Wrappers). Remove `.trust-icon--user`,
  `.trust-signal--second::before` and its forced colours entry (the reporter signal leaves the button). G only: the
  `data-button="line"` rules in `variants/g.css`. E only: `variants/e.css`. F: `variants/f.css`.
- Every new rule that declares `display` keeps `:not([hidden])`. The sketch's own face, pills, row top and G face rules lacked
  the guard until the repair pass, although this note said they had it.
- No custom property is declared and no colour is new, so the `trust.css` allowlist test still holds. `color-mix` is already
  used in `trust.css` (the Yes button hover), so no browser floor moves. A wrapping flex `<button>` was checked in WebKit 26.6
  and Chromium 153 only.
- `TestPopupCardSurfaceAndGeometry` (`web/popup_contract_test.go`, line 307) pins things the sketch's rules do not meet as
  written. (1) It bans `.trust-`, `.icon-glyph--`, `.severity-label` and `.visibility-tag` in any rule head that names
  `popup-card` or `leaflet-popup`, so `.popup-card .trust-reporter` (common.css), `.popup-card__header .trust-reporter` (f.css)
  and h.css's `.popup-card__pills .trust-reporter` cannot be copied: write the reporter rules on `.trust-reporter` alone (a
  modifier class set where `showScores` is true, such as `trust-reporter--scored`, can carry a popup only difference). (2) It pins
  `gap: var(--space-sm)` on `.popup-card__header:not([hidden])`, so keep that declaration and add `row-gap` after it, never
  replace it with two longhands. (3) It pins `padding-right: var(--space-2xl)` on that header; H's popup moves the 48px for the
  close button to the badge row, so that pin changes with H. (4) Every popup rule that declares `display` keeps the hidden guard.
- Worth deciding at build time: no hover rule in the app is guarded for touch, so on an iPhone a tap can leave the hover fill
  showing until the next tap elsewhere. Wrapping the hover fill in `@media (hover: hover)` avoids it. The sketch leaves it
  unwrapped, like the vote buttons, and has not been tried on a phone.

**Wrappers.** E, F and G draw a pills group (popup) and a row top (rows) around the labels because E needs them for its
`space-between` trick. F and G do not: the popup header is already a wrapping flex row, so the words need only `flex-basis: 100%`,
and on rows the words are a block placed after the severity label with `margin-bottom: var(--space-xs)`. This was tested by
taking the wrapper out of the live DOM, in WebKit and Chromium, for F and G on all three surfaces at 320 and 360 with states 1, 3,
4, 6 and 9: 58 of 60 renders are identical to the pixel, and the 2 that differ are the 320px popup with an Unconfirmed label, where
the label that wraps sits 4px closer to Low. It buys three things: the severity label stays the first child of the row body, the
profile template stays as it is, and so `TestRowSeverityLabelMountsFirst`, `TestProfileTemplateUsesServerSeverityAndTrust` and
the database test `TestProfileRendersSeverityLabel` need no change. H's rows add only a state line under the meta, so a label
wrapper is not needed there either (the unwrap test ran for F and G, not for H).

**Icon:** add `web/static/icons/chevron-down.svg` (Lucide `chevron-down`, `<path d="m6 9 6 6 6-6"/>`), an entry in
`nonCategoryIcons` in `web/css_contract_test.go`, and a literal `url("/static/icons/chevron-down.svg")` mask in
`trust.css` so `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon` (A39) passes. `user.svg` stays for the account menu.

**JavaScript:**

- Fold the reporter words into the `createTrustBlock` instance. The instance already holds `reporter` (a `createSignal` result with
  `.el`) and `updateTrustBlock` already refreshes it through `renderTrustSignal`, so no new export, no new call site and no new
  refresh path are needed. In `trust.js`: (a) build it plainly, a `trust-reporter` span with no `trust-icon` and no
  `trust-signal--second`, and on the popup the number as plain text behind the visually hidden ", score" prefix and not in
  `trust-num` (semibold); that is its own small builder, because `createSignal` adds the icon and the semibold number; (b) do not
  append it into `signals`, since consumers place `instance.reporter.el` themselves; (c) in `updateTrustBlock`, the early return
  for a failed `validTrust` hides only `block.el` and the words live outside it, so add `block.reporter.el.hidden = true` there,
  or the words stay on screen next to a hidden block, against the file's rule that an invalid trust object hides the whole block;
  (d) `block.signals.hidden = !state.currency`, since the reporter no longer lives in it.
- Not a separate exported builder and updater, which is what the first draft of these notes said. That adds four refresh paths
  (`feed.js` `updateRow`, `map.js` `updatePopupCard`, and the two in `activity.js`, first fill and `onReopenClick`) and a hide
  rule for the same invalid case, and it breaks `TestActivityMountsTheTrustBlock` (`web/activity_trust_contract_test.go`), which
  pins one `createTrustBlock()` with no options, exactly two `updateTrustBlock(` calls and the order createTrustBlock then
  createReopenControl. `updatePopupCard` also must not contain `createElement(` or `appendChild(` (`popup_contract_test.go`),
  so placement stays in the builders.
- A held or removed Activity row (Phase 4 plan 04-16, lines 24 and 45) has no `data-trust` and mounts no trust block, so it has no
  reporter element either.
- Placement is all a consumer does, and it never types a `trust-` class (`web/trust_contract_test.go` bans those literals in
  `feed.js` and `map.js`). `map.js buildPopupCard`: put `trust.reporter.el` after the state label in the header (H: in the state
  line that holds the label and the words). `feed.js createRow`: H puts the state label and `trust.reporter.el` in one state line
  under the meta, built where the label is built today, so the call order that `votes_contract_test.go` and `trust_contract_test.go`
  pin (meta, then `createVisibilityTag`, then `createTrustBlock`, then `createVoteBlock`) is kept. `activity.js` builds its own state
  label (line 59) and block (line 71), so it builds the same state line there, and the profile template needs no change.
- The reporter element is hidden for a `none` signal (A47) and shows nothing, no icon and no text.

**Tests and documents these changes touch** (found by reading the tests, not by running them):

- `web/trust_contract_test.go`: `TestTrustCSS` pins the toggle base rule (`display: block`, `width: 100%`, the padding,
  `text-underline-offset: 4px`), `.trust-block__toggle:hover` as `text-decoration: underline`, the no thickness check,
  `.trust-icon--user` and the `.trust-signal--second::before` rules and their forced colours entry.
  `TestTrustBlockMountedOnBothSurfaces` requires `trust-icon trust-icon--user` and `trust-signal--second` in `trust.js`.
  `TestTrustDisclosure` needs no change as long as `createTrustBlock` keeps a button, `stopPropagation` and `'Enter'`.
- `web/css_contract_test.go`: the `nonCategoryIcons` map and the user glyph comment.
- `web/popup_contract_test.go` `TestPopupCardSurfaceAndGeometry`: the pins listed under CSS.
- Only if the severity label gets a wrapper (E, F and G as drawn; not F or G built without it, and not H on rows):
  `web/labels_contract_test.go` `TestRowSeverityLabelMountsFirst` (line 77: `body.appendChild(severityLabel)` must be the first
  `body.appendChild(` in `feed.js createRow`), `web/activity_trust_contract_test.go` `TestProfileTemplateUsesServerSeverityAndTrust`
  (line 38, the regex at line 61: the row body starts with the severity label span, then `report-row__title`), and the database
  test `internal/api/handlers/activity_e2e_test.go` `TestProfileRendersSeverityLabel` (line 466, the same regex on the rendered
  page). That one needs a database through `testutil.NewTestDB`, so it does not run without one, and it must only ever be pointed
  at a disposable database.
- Wording, code: `internal/service/trustcopy.go` holds every sentence. `reporter.go` only mentions "clear result" in a comment.
- Wording, tests that pin the old sentences, in `internal/service`: `trustcopy_test.go`, `currency_test.go`, `reporter_test.go`,
  `trustview_test.go`, `feed_test.go` and `trust_paths_test.go`. A reviewer's dry run with only `trustcopy.go` changed gave 18
  failures in those six files, all old sentence mismatches or the caveat rule near line 265 of `trustcopy_test.go`, none a
  character set failure. Database tests: `internal/api/handlers/trust_e2e_test.go` and `activity_e2e_test.go` (line 412, the expiry
  line). `web/trust_contract_test.go` lists "Counts separate places" among strings that must not appear in `trust.js`, which
  stays true.
- Wording, documents: `internal/api/handlers/trust.go` (swagger examples), `docs/docs.go`, `docs/swagger.json`,
  `docs/swagger.yaml`, the sentence tables in `docs/TRUST-MODEL.md` (the two counters table at lines 99 to 108, the still current
  section from line 112, the reporter section from line 228) and the copywriting table in `03-UI-SPEC.md`. Nothing catches stale
  documents today: `TestTrustModelDocMatchesCode` still passes with the old table, so add a checklist line to the change, or extend
  that test to compare the table with `trustcopy.go`.
- Wording, a coupling: `internal/api/handlers/votes.go` (line 170) returns "This report has expired." as a vote error and
  `votes_e2e_test.go` pins it (lines 390 and 538). The proposed lead for an expired report is "Expired, so it can no longer be
  confirmed.", so one screen could say two things about an expired report. Leave the vote error or align it, at build time.
- `03-UI-SPEC.md` needs explicit amendments: D-18 (the reporter tag leaves line two of the block), A4 (the user icon goes),
  A48 (hover underline replaced by a hover fill, press and focus unchanged), A7, A56 and RR-1 (caveat once, shorter
  wording), D-19 and A50 (sentence wording, the reporter number in regular weight), A21 and A26 (the top zone), and a new
  row for the chevron. D-20 still holds: the block is identical on all three surfaces.
- The popup tab order is unchanged: close, trust button, Confirm, Dispute, Mark resolved. The reporter words are not
  focusable.

**Phase 4 plans that depend on this change** (recorded here only, the plans are untouched):

- `04-12-PLAN.md` builds `DisclaimerGlossary` from `countsCaveat`, `provisionalLeadSentence`, `expiredLeadSentence`,
  `needsReconfirmingSentence` and `tooEarlySentence` (lines 70, 137 and 228) and pins the old sentences in its acceptance criteria
  (line 238: entry 1 ends with " confirmations before this counts as confirmed.", entry 2 ends with "Counts separate places, not
  votes.", entry 8 is "This report has expired. It can no longer be confirmed."). `04-UI-SPEC.md` line 392 names the same
  anchors, including that the caveat appears "under the confirmed and disputed entry". If the wording is accepted, keep the five
  identifiers and change the 04-12 acceptance strings and the 04-UI-SPEC anchors in the same change; the caveat then appears once,
  so the "confirmed and disputed entry" anchor changes too.
- `04-18-PLAN.md` (lines 77 to 84) mounts the shared trust block on a fourth surface, the Low data rows: `createLowRow` calls
  `PinalertTrust.createTrustBlock` and `updateLowRow` calls `updateTrustBlock(row.trust, report.trust)`. With the reporter words in
  the top zone, `createLowRow` must place `row.trust.reporter.el` too, or those rows lose the reporter words the other three
  surfaces show. D-20 then covers four surfaces.

**What the sketch cannot show:** Leaflet geometry, autopan and the `100dvh` scroll of an open popup (the card heights
above are the card, not the viewport); the 30 second poll updating the words in place (the reporter words refresh through
`updateTrustBlock`, which rewrites text only and never touches the open state); real touch input.

## Verified

- After the repair, the four original tabs (the baseline, E, F, G) pass the live check with no unexpected failure: 4536 renders per
  engine, in WebKit 26.6 and Chromium 153, over three surfaces, nine states, light and dark, three widths per surface, panel closed
  and open, the Unconfirmed label in both places, text at 150, 175 and 200 percent at 320 and 200 percent at 360. The baseline fails
  only the four checks that are the point of this sketch (three on the report with no reporter signal, which has no reporter words
  to judge).
- The identity gate for the repair: the geometry of the button, face, counts, chevron, panel, reporter words, labels, titles and vote
  controls at 100 percent text, before and after the CSS change, 756 configurations per engine, all within 0.01px. The diff tool
  does detect change: it reports 818 differences between the two engines.
- The count census: no avoidable split at 150, 175 or 200 percent, on every surface and width, in all nine states, in both engines.
  The same census on the first version found the splits tabulated under Measured.
- H, which was added during the repair pass, after that one line fix: 216 renders per engine pass the live check (three surfaces,
  three widths, nine states, light and dark, and 200 percent text at 320), the census finds no avoidable count split at 150, 175 or
  200 percent, and 80 real pointer and keyboard checks pass, in both engines.
- 21 deliberate breakages (the earlier 19, plus the chevron column as a fixed grid column and a squeezed count, which the new rule
  catches) are all caught, in both engines.
- A real pointer and the keyboard, not the preview classes, in both engines (300 checks each): no underline on the toggle, any
  ancestor or any descendant at rest, on `:hover`, on `:active` and on `:focus-visible`; a fill appears on hover and a
  different one on press; the focus ring is the 2px inset ring; Enter and Space open and close; Escape closes the panel,
  including after a mouse click with focus on the body (Safari does not focus a button on click); a second Escape closes the
  popup. Reduced motion gives no transition on the button or the chevron, and normal motion never exceeds 150ms.
- Both themes at 320, 360 and wide frames were looked at in WebKit, Safari's engine, and so was the board at 360 and 900.
  The layout figures in this README were measured in both engines and match. Forced colours were checked in Chromium. The
  accessible names were read in both: the button reads "7 confirmed 1 disputed Getting old, score 52. Shows what these
  mean.", the reporter words read "Reliable reporter, score 78" before the title.
- Not run: the real Safari app, VoiceOver, a touch device, a dry run of the Go tests against the build notes above, the Go tests against the proposed sentences (the closed set was
  mirrored by hand from `trustcopy_test.go`), and real browser text zoom (200 percent is simulated by doubling the type
  tokens).

## Owner decision (2026-10-09)

The owner chose where the reporter words go: directly beside the Unconfirmed label, as plain words (no tag look, no underline). Variant **H: Beside the label** draws exactly that and is the default tab. E, F and G stay for comparison. The shorter wording is a separate decision from the layout (it breaks existing tests and touches the Phase 4 disclaimer plan).

## Repair pass

Independent reviewers checked the first version of this sketch and raised six problems. Each was checked against the running
sketch before anything was changed. Variant H was added to the board by a separate change while this pass ran. It shares the
button, so the button fix applies to it. The one change this pass made to H's own files is one line in `variants/h.css`: the
12px reach of its state line into the row padding on a 320px phone is now one spacing step (`calc(-1 * var(--space-md))`),
because the live check reads margins against the scale and failed 72 of 216 H renders (every row and Activity render at 320,
in both engines). The line has no fill or border, so nothing looks different.

| # | Finding | Verdict | What changed |
|---|---|---|---|
| 1 | In E, F and G the chevron column squeezes a count until "7" and "confirmed" land on separate lines at large text sizes | Real, fixed | `variants/common.css`: a wrapping flex button, the face asks for 6.85em, and the chevron drops under the text when less is left beside it. A new live check, "Counts keep their number and word together", two new negative controls, and 150% and 175% on the board. The census and the numbers are under Measured. |
| 2 | E puts the words in different places and next to the close X | Partly real: documented, behaviour not changed | The numbers were right and the X adjacency was missing from the README, so it is now in the E text and on the board (16px short of the button box, about 30px short of the X mark). E stays as briefed, and the row only option is written down as an option. Changing E would remove the most literal answer to "on the top part of the box where Unconfirmed is located" from the comparison, and E was already not the recommendation. |
| 3 | The test list missed three web tests and one database test | Real, fixed in the notes | All four are listed with their cause. The sketch's CSS also lacked the `:not([hidden])` guard on the face, pills, row top and G face rules, which the README said it had: fixed, with no pixel change. |
| 4 | A separate reporter builder adds four refresh paths and leaves the words on screen when the block hides | Real, fixed in the notes | The notes fold the element into the `createTrustBlock` instance, hide it in the invalid trust branch of `updateTrustBlock`, and name the call sites and tests that must not change. |
| 5 | The wording change breaks 18 tests in six files, three of them unnamed | Real, fixed in the notes | All six files, the database tests, the documents and swagger files, the vote error coupling, and a checklist line for the document table. |
| 6 | Phase 4 plans depend on the old sentences and on the block | Real, recorded, plans untouched | 04-12, 04-UI-SPEC line 392, 04-18 and 04-16 are named in the notes. |

Suggested fixes that were not taken, and why. "Chevron on the first row only": the first count is on the first row, so it still
loses 24px. A 16px chevron: it frees 4px and the count needs 5px more at 200 percent. Dropping the extra 4px of right padding: it
fixes "7 confirmed" but not "99+ confirmed", and it moves the 100 percent geometry. A rows only E: see finding 2.

Spacing, also the owner's call (2026-10-09): 16px between the state label and the reporter words, 8px between the lines of the header; phones 340px wide or narrower keep 8px between the label and the words on rows so they stay on one line. Winner: H.
