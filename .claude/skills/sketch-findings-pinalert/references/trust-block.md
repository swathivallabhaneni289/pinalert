# Trust block (variant d4)

## Design Decisions

The trust block is two rows of plain text inside ONE button. Tapping or clicking it opens an inline panel of short
sentences directly under it, and tapping again or pressing Escape closes it. The detail lives in the panel, never on
the surface, so the surface stays short ("Confirmed by three what?" is answered one tap away).

**Row 1, the counts.** `12 confirmed | 0 disputed`. Both counts are always shown, zeros included. The number is
semibold and the word regular, both in the normal text colour (D-16). A 1px hairline sits just under half way across
and the second count starts 24px after it. Counts above 99 show as `99+`. The two counts always stay on one row.

**Row 2, the two signals.** A clock icon and the still current word with its score, then a user icon and the reporter
tag with its score: `Up to date 82 | Reliable reporter 78`. The words are regular weight in the muted grey, the numbers
are semibold in the normal text colour. When the pair does not fit on one row the second signal drops under the first
and its hairline goes with it (the hairline is drawn in the gap and clipped by the row, so a wrapped signal never leaves
a stray bar). A signal that has no score shows no number: "Too early to tell" and "New reporter" carry none (R-01, R-07).
The numbers appear in the popup only (D-09).

**The button.** It looks like plain text: no border, no fill, 8px corners, at least 44px tall (about 76px with both
rows). Hover and press underline it, press thickens the underline, and keyboard focus draws the inset ring. The rows are
`span`s because a button may only hold phrasing content. It announces expanded or collapsed, and carries a visually
hidden hint, ". Shows what these mean."

**The panel.** Up to four sentences, one per line, 14px, normal text colour, no floating tooltip. A report with no
still current signal shows three. If the person votes while the panel is open the sentences update in place.

| Line | Sample sentence (sketch wording, see "Departures") |
|---|---|
| Confirmed | `12 people in 12 different places confirmed this. Counted by place, not by vote.` One place: `1 person in 1 place confirmed this.` None: `No one has confirmed this yet.` (the tail is kept) |
| Disputed | `No one nearby has disputed this.` One: `1 person nearby disputed this.` Many: `4 people nearby disputed this.` |
| Still current | Up to date: `2 different places confirmed this in the last 2 hours.` Too early to tell: `Too few people nearby have confirmed this yet to judge.` Getting old: `Few recent confirmations. It fades unless someone confirms it again.` Needs re-confirming: `No one has confirmed this lately. It fades unless someone confirms it again.` |
| Reporter | `7 of 9 earlier reports were confirmed by people nearby.` New reporter: `This is their first report, so there is no record yet.` |

The still current and reporter figures in the sketch are placeholders. In the app every sentence must carry the real
numbers from the server's trust object (D-19, D-21), and the browser holds no thresholds.

**Keyboard and screen reader.** Tab order inside the card: the trust block (one stop), Confirm, Dispute, Mark resolved,
the comments link, then the close button. Escape closes the panel first and returns focus to the button, and a second
Escape closes the popup. The sketch listens for Escape on the document in the capture phase, because Safari does not
focus a button on click, so the key would otherwise never reach the card. There is no `aria-live` on the block, because
the 30 second poll would re-announce it. Every string reaches the DOM through `textContent`, never a markup sink.

## CSS Patterns

```css
.d4-trust { margin-top: var(--space-sm); border-top: 1px solid var(--color-border); }

/* One button spans both rows and looks like plain text. */
.d4-trust__toggle {
  display: block;
  width: 100%;
  min-height: var(--touch-target-min);
  padding: var(--space-sm) var(--space-xs);
  border: 0;
  border-radius: 8px;
  background: transparent;
  font-family: var(--font-family);
  text-align: left;
  color: var(--color-text);
  cursor: pointer;
  text-underline-offset: 3px;
}
.d4-trust__toggle:hover { text-decoration: underline; }
.d4-trust__toggle:focus-visible { outline-offset: -2px; }
```

```css
/* Counts: the first column holds its text, the hairline sits just under half way, the second count starts 24px after
   it. The word is regular, the number inside it is semibold (.d4-num), the same pairing the signals row uses. */
.d4-row--counts {
  display: grid;
  grid-template-columns: minmax(max-content, 47fr) minmax(0, 53fr);
  font-size: var(--font-size-body);
  line-height: 32px;
  font-weight: var(--font-weight-regular);
}
.d4-count-cell--second { position: relative; padding-left: var(--space-lg); }
.d4-count-cell--second::before {
  content: "";
  position: absolute;
  top: 8px;
  bottom: 8px;
  left: 0;
  width: 1px;
  background: var(--color-border);
}
.d4-count { white-space: nowrap; }
.d4-num { font-weight: var(--font-weight-semibold); color: var(--color-text); }
```

```css
/* Signals: icon, quiet words, strong number, a hairline in the gap, the second signal. The row clips its own
   overflow so a wrapped signal takes its hairline with it. */
.d4-row--signals {
  display: flex;
  flex-wrap: wrap;
  column-gap: var(--space-md);
  overflow: hidden;
  font-size: var(--font-size-label);
  line-height: 28px;
  font-weight: var(--font-weight-regular);
  color: var(--color-text-muted);
}
.d4-signal { position: relative; display: flex; align-items: center; gap: var(--space-sm); white-space: nowrap; }
.d4-signal--second::before { content: ""; position: absolute; left: -8px; width: 1px; background: var(--color-border); }

/* The panel, plain text under the block. */
.d4-explain:not([hidden]) { display: grid; gap: var(--space-xs); margin: 0; padding: 0 var(--space-xs) var(--space-sm); }
.d4-explain__line { margin: 0; font-size: var(--font-size-label); line-height: var(--line-height-label); color: var(--color-text); }
```

The full stylesheet is `sources/001-map-pin-popup/variants/d4.css`.

## HTML Structures

```html
<div class="d4-trust">
  <button class="d4-trust__toggle" type="button" aria-expanded="false" aria-controls="d4-explain-REPORT_ID">
    <span class="d4-row d4-row--counts">
      <span class="d4-count-cell"><span class="d4-count"><span class="d4-num">12</span> confirmed</span></span>
      <span class="d4-count-cell d4-count-cell--second"><span class="d4-count"><span class="d4-num">0</span> disputed</span></span>
    </span>
    <span class="d4-row d4-row--signals">
      <span class="d4-signal"><span class="d4-icon d4-icon--sm d4-icon--clock" aria-hidden="true"></span>
        <span class="d4-signal__text">Up to date <span class="d4-num">82</span></span></span>
      <span class="d4-signal d4-signal--second"><span class="d4-icon d4-icon--sm d4-icon--user" aria-hidden="true"></span>
        <span class="d4-signal__text">Reliable reporter <span class="d4-num">78</span></span></span>
    </span>
    <span class="visually-hidden">. Shows what these mean.</span>
  </button>
  <div class="d4-explain" id="d4-explain-REPORT_ID" hidden>
    <p class="d4-explain__line">...</p>   <!-- confirmed, disputed, still current (when present), reporter -->
  </div>
</div>
```

## Departures from locked Phase 3 decisions

D4 follows the owner's reference image, and it differs from decisions already locked in `03-CONTEXT.md` and
`03-VALIDATION.md`. The UI contract has to record each one as an explicit amendment, or bring D4 back in line.

| D4 does | Locked decision it departs from |
|---|---|
| Severity label is a fully rounded pill (`999px`) and the card has 12px corners. The sketch checker is loosened for D4 only (`maxRadius: 12`, `.d4-chip` exempt). | V-40 forbids `999px` in trust CSS (`TestNoPillShapedControls`), and the site design rules ban pill shapes. Needs an owner exception for labels. |
| Signal words are quiet grey (`--color-text-muted`). The counts row stays in the normal text colour. | D-18: normal text colour, because muted text fails AA on the tinted feed row. Grey passes on D4's plain surface, so the row and the popup would differ. |
| Small clock and user icons sit beside the two signals. | D-18: no chips, bars, colours or icons in the block. |
| One disclosure button with a panel of sentences, one tab stop. | D-19: each counter, word and tag has its own hover, tap and focus reason (up to three focus stops per row). V-39 tests Escape, focus, hover and click. D-20: the block is identical on the row, the popup and the Activity page, so the row and the Activity page need a decision too. |
| The Unconfirmed pill has no explanation in the panel. | D-17: hovering the chip explains "Needs 2 confirmations before this counts as confirmed." |
| The confirmed sentence ends "Counted by place, not by vote." | D-19 approved wording: "Counts separate places, not votes." |
| Too far is a native `disabled` button plus an always visible sentence that types out "1 km". | D-22 and V-36: `aria-disabled`, a reason on hover, tap and focus, and a radius from the page config, never typed into a template or script. |
| The popup is up to 340px wide, and its surface is neutral. | The design brief said about 300px. A neutral surface undoes quick task 260923-mb0, which gave the popup the severity tint on purpose. |
| A comments footer with a count. | Comments have no phase, table or endpoint. Never show a count that does not exist. |

## What to Avoid

- Hover only reasons. Most phones have no hover, so tap and focus are required (D-19). The panel is click and tap based.
- The native `title` attribute alone as the reason.
- Chips, bars, meters, stars or colour for trust information. Colour is for severity only; trust text is plain.
- Showing a number when a score is null. Phase 6 treats null as unknown.
- Predicting a vote in the count (the optimistic bump). The server's trust object is the only source.
- Putting the 0 to 100 numbers on the feed row. They are for the popup only (D-09).
- `aria-live` on the block.

## Origin

Synthesized from sketch 001 (map-pin-popup), winner D4, with D-09, D-16 to D-22, R-01 and R-07 from
`03-CONTEXT.md` and V-36, V-39, V-40 from `03-VALIDATION.md`.
Source files available in: `sources/001-map-pin-popup/`, `sources/REVIEWS.md`.
