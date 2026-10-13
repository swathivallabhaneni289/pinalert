---
sketch: 005
name: phone-layout
question: "Can the feed, the report detail and the Activity page look like the owner's two pictures under 900px, with the layout at 900px and up unchanged, and what does copying them change?"
winner: "built"
tags: [phone, feed, detail, activity, bottom-bar, picture, light-dark]
---

# Sketch 005: phone layout

This one is built, not mocked. The code is `web/static/css/phone.css`, small additions to the two page templates, a few
lines of `feed.js` and `map.js`, and the reporter record on the Activity page. This file is the text only record of what
was copied from the owner's pictures, what was left out, and every place where the picture and the house rules disagree.

## Direction

The owner pasted two pictures on 2026-10-13 and said the site is mostly for phones (2026-10-10). The rule they set for
pictures: copy the look as drawn, and write every conflict with the house rules in the docs instead of adapting the look.
Under 900px wide (the existing 899.98px breakpoint) the feed, the report detail and the Activity page follow the pictures.
At 900px and up nothing changes. That is checked with computed style and geometry probes at 1280 by 800 and at 900 by 800
against a baseline taken before the first edit.

## The pictures in words (the project keeps no images)

**Picture A, screen "02 Feed, map + list".** Map on top, about 40 percent of the height, always visible. Category chips
float over the top of the map in one row that scrolls sideways (All selected dark with white text, Flood, Rescue, Shelter,
Power, more clipped). Map pins are round dots with a white ring (red critical, green low, grey unconfirmed). Under the map a
list headed "Nearby now" (bold, left) with "5 active · 1 km" in grey on the right. Rows are rounded cards: Critical has a
soft red tint, Low a soft green tint, unconfirmed is white. First line small: a coloured dot, the bold severity word
("Critical", "Low", "Low · Unconfirmed"), then category, minutes, distance in grey ("Flood · 51 min · 0.3 km"). Then the
report text in bold, up to 2 lines. Then one trust line: tinted rows "1 confirmed · Too early to tell"; the green row "2
confirmed · Up to date · Reliable reporter" in green; the white row "Waiting for a second confirmation" in grey. A round
black add button with a plus, bottom right, floating above the list and above the bottom bar. Bottom bar: three tabs, Feed,
Past, Activity, the active one bold black with a small dot above it. No map or list switch button on phones. Tapping a row
highlights its pin; tapping a pin scrolls to its row.

**Picture A, screen "03 Report detail" (phone, full screen).** A short map strip at the top with a round white back arrow. A
severity chip (outlined pill with dot and word, for example "Low"). The title in large bold. A grey meta line "Power outage
· 50 min ago · 0.4 km away". A white trust card: top line "2 confirmed · 0 disputed" with "Up to date" in green on the
right, then tick rows each with a green round tick, a bold heading and a grey sub line ("Seen from 2 different places /
Counts places, not votes.", "No disputes / No one nearby has disputed this.", "Still happening / 2 confirmations in the last
2 hours.", "Reliable reporter / 5 of 5 earlier reports were confirmed."). Below the card: "Fades in 7 h unless confirmed
again." Two big buttons side by side: Confirm (black fill, white text) and Dispute (white). A green centred "Mark resolved"
link. A small grey centred hint "You need to be within 1 km to confirm or dispute." The page background is a very light
green tint when the severity is Low.

**Picture B, screen "05 Activity".** Large bold "Activity" title on the left, grey "Log out" on the right. A white rounded
card: small grey "Your reporter record", bold "Reliable" under it, "5 of 5 confirmed" on the right. A segmented control
(grey track, white selected segment): "My reports" and "My votes". Report cards (white, rounded): first line a coloured dot
and bold severity word on the left ("Low" green, "Medium" amber, "Critical" red) and a status word on the right, coloured:
Active (green), Waiting (amber), Resolved (grey), Expired (grey). Then the report text in bold. Then a grey line: "2
confirmed · 0 disputed · 50 min ago", "Needs one more confirmation · 5 min ago", "Resolved by you · yesterday", "Not
re-confirmed · 2 days ago". The same bottom bar with Activity active. Owner: "use the same instructions we used for the
laptop version, the disputes and other things should also be added". Everything the laptop Activity page has stays on the
phone page in the picture's style.

## Not built

Three drawn items are not built. Each needs data or a feature that does not exist, and a dead control is not allowed
(sketch 004 already rules dead controls out). The handback to the owner asks about all three.

| Drawn item | Why it is not built | What it needs |
|---|---|---|
| The Past tab in the bottom bar | No history view exists. The feed query drops expired reports. | Phase 3.1 history views, which still has an open owner question about phone history tabs. |
| The category chips over the map | Filtering is Phase 3.1 scope, and there is an open owner question on whether a filter may hide critical or rescue reports. | Phase 3.1 filters plus the owner's answer on hiding critical or rescue reports. |
| The My reports and My votes segmented control (Task 3) | No read of votes by account exists anywhere. The built laptop Activity page lists only the account's own reports, and a test guards against promising a voting history. | A read of votes by account, which no phase has yet. |

The bottom bar therefore has two tabs, Feed and Activity.

## Choices where the picture is silent or contradicts itself

- **Sizes.** The pictures are words, not measurements. Sizes, spacing and corners use the existing tokens: type sizes 14,
  16, 18 and 24, weights 400 and 600, spacing 4 to 64, corners up to 8px, targets of 44px. Nothing is measured off a picture.
- **The provisional word.** Picture A says "Unconfirmed" on one screen and "Waiting" on another. The existing app word
  "Unconfirmed" stays everywhere.
- **Counters.** Both trust counters stay on every row (D-16). The picture's trust line shows one count and the currency
  word; the app shows "1 confirmed · 0 disputed" and then the currency word.
- **The radius in the count line.** The picture shows "5 active · 1 km" as sample data. The phone shows the number of
  rendered reports and the radius the feed was actually asked for (`data-default-radius-km`, 10 km by default). Nothing is a
  fixed figure. The word is "disputed" instead of "active" while Show disputed is on, because then the rows are the
  disputed ones.
- **Show disputed reports.** The checkbox is not drawn, but it is an existing laptop feature and the owner asked that the
  disputes stay. It sits under the heading in the label size.
- **The theme toggle.** The pictures do not draw it. It moves to the top right corner of the map at the medium space, clear
  of the zoom control at the top left and of the add button at the bottom right.
- **The add button.** Black round with a white plus, as drawn. It floats at the right medium space, above the bottom bar.
- **The account button.** Not drawn, so it is hidden on phones. Its three items stay reachable: Activity through the bottom
  bar, the email and Log out on the Activity page.
- **Tap a row, tap a pin.** Tapping a row opens the report detail (Task 2). The open pin is the highlighted pin, drawn with a
  wider ring in the text colour. Tapping a pin opens the same detail and scrolls only the list pane to that row, never the
  page.

## Wording kept from the app

Every drawn string maps to an existing app string where one exists, and each mismatch is listed here.

| Drawn | Kept from the app | Why |
|---|---|---|
| "Flood · 51 min" | "Flood · 51 min ago" | The existing row meta line (feed.js buildMetaText) |
| "0.3 km" | "0.3 km away" | The existing distance words (app.js formatDistance), shown only when the feed is around the reader's own position (A94) |
| "Waiting for a second confirmation" | The two counters, then the currency word | It is not an app string. The row shows the counters and the Unconfirmed word. |
| "Waiting" | "Unconfirmed" | See the provisional word above |
| "1 confirmed · Too early to tell" | "1 confirmed · 0 disputed · Too early to tell" | D-16: both counters on every row |
| "Reliable reporter" | "Reliable reporter" | Same words, server written |

## Departures from the house rules (Task 1: the feed)

Each item names the rule, the pictured look and what was done. Where a contract test refused the pictured look, the test is
named with the exact head it now lets through, and every other assertion in it is unchanged.

1. **Tokens instead of measured sizes.** See above. No raw hex, no font size literal, no weight outside the two tokens, no
   corner above 8px (the dots are 50 percent and are marks, not controls). Every control is at least 44px tall.
2. **Trust line colours (A3 and D-18: colour is reserved for severity, trust text is the normal text colour).** The pictured
   green row is reproduced. The trust line takes the good tone when the row's currency fact is good, and the muted colour
   on a provisional (white) card. The good tone is used only where it clears 4.5 to 1 on the card colour in both themes: on
   the page colour (5.10 light, 10.12 dark), on the Low tint (4.53 light, 7.00 dark) and on the Medium tint (4.61 light,
   7.06 dark). It is not used on the Critical tint (4.35 light fails), so a Critical row keeps the normal text colour. Muted
   on the page colour is 4.83 light and 7.66 dark. Contract test status: `TestTrustCSS` reads `trust.css` only and
   `phone.css` is not read by it, so no assertion needed a change; the departure is recorded here for the same reason the
   test's own comment gives for the rule.
3. **Meta and distance colour.** The picture draws these in grey. Muted fails 4.5 to 1 on every tint in the light theme
   (4.34 on the surface, 4.30 on Low, 4.37 on Medium, 4.12 on Critical), so on a tinted card they take the normal text
   colour (14.43 to 15.04 light, 12.23 to 14.72 dark). They are muted only on white cards, where muted clears 4.5 to 1.
4. **The row title is clamped to two lines (A55: a title is never clamped).** The picture draws up to two lines. The one
   head `#report-list .report-row .report-row__title` in `phone.css` clamps it. The detail shows the whole text. Updated:
   `TestRowTitleWrapsLongWords` in `web/labels_contract_test.go` (exact head, `phone.css` only).
5. **The current tab dot is round.** The bottom bar draws a 6px dot above the word of the current tab. It is a mark, not a
   control shape. Updated: `TestReadOnlyLabelsRoundedControlsNot` in `web/labels_contract_test.go` (exact head, `phone.css`
   only).
6. **The severity label loses its pill.** On a phone card it is a dot and a bold word with no border and no fill (A81 drew
   it as a bordered label). Critical gets the same 8px dot that Medium and Low already carry, through a phone only
   `::before`. The word is the normal text colour, so the Critical red word is gone from phone cards and the dot carries the
   colour.
7. **No category glyph on pins or rows.** The pictures draw plain dots. The category is still in the row meta line and in
   the pin's accessible name. Pins stay a 44 by 44 tap box with a 20px dot and a 3px ring centred in it.
8. **No vote buttons on rows.** Voting happens in the detail, as drawn. The too far note and the vote error are hidden on
   phone rows with them. A reader on a phone confirms, disputes or resolves from the detail.
9. **No chevron and no clock on rows.** The trust line reads as plain text, as drawn. The toggle is still a button that opens
   the facts panel in place, and its screen reader hint is kept, but the visible open cue is gone on phones.
10. **No left severity bar.** Cards are rounded 8px with the severity tint. A provisional or hidden card is white with a 1px
    hairline in the border token.
11. **A87 is amended on phones only.** A87 said a pin selection never scrolls the list, because on a wide screen the
    document scrolls and carried the map away. Below 900px the list is its own scroll box under a fixed height map, so
    moving it never moves the map. `revealRowInListPane` sets the list pane's own scroll position and nothing else. The page
    is never scrolled, `applySelectionHighlight` has no scroll in it, and the subscriber is still the single `onSelect`
    handler. Updated: the doc comment of `TestMapSelectionNeverScrollsThePage` in `web/popup_contract_test.go`, every
    assertion kept.
12. **D-A is not applied on phones.** Decision D-A put the theme toggle below the add button. On phones the bottom right
    corner holds only the add button above the bar, and the toggle moves to the top right corner of the map.
13. **The account button is hidden on phones.** Activity is a tab in the bottom bar. The email and Log out are on the
    Activity page.
14. **The bottom bar.** Two tabs, Feed and Activity, 44px tall each, muted text with the current tab in the normal text
    colour and semibold. No pill shapes. It sits at z index 1010, above the add button and the toggles at 1000 and below
    the account header at 1050 and the modal at 1100.
15. **The dots between the parts of a line** are CSS generated content with an empty alternative text, after a plain
    fallback declaration for an engine that does not know the alternative form. A screen reader does not read them.
16. **The state word next to the severity word is plain text.** On phone cards the visibility tag (Unconfirmed) loses its
    rounded outline and reads as a bold word after the severity word.

## Departures from the house rules (Task 2: the report detail)

The detail is the existing popup card, opened on phones as a full screen sheet under a short map strip. One card, one set
of focus rules, one Escape path. Nothing in the card's markup, focus handling or Escape handling changed.

1. **Mechanism.** `map.js` creates a pane named `detailPane` inside the map container (a child of `#map`, outside the map
   pane's transform) and writes `options.pane` and `options.autoPan` onto every popup before it can open: the detail pane
   and no Leaflet autopan under 900px, the normal popup pane and the old autopan at 900px and up. A resize that crosses
   900px closes the open popup through the existing close path, so focus returns to its opener. The sheet itself is
   `phone.css` only: a fixed popup, with `left`, `bottom` and `transform` carrying the important flag because Leaflet
   rewrites those three inline on every update and zoom, and the content `width` the same for the same reason. Those four
   are the only important flags anywhere, each under a `.leaflet-detail-pane` head, and a contract test pins that.
2. **The strip.** 192px (three times the 64px token), 64px when the screen is under 360px tall. The sheet starts under it
   and runs to the bottom of the screen, covering the list, the add button and the bottom bar. The z index is 1080, above
   the bar (1010) and the account header (1050), below the modal (1100) and the toast (1200).
3. **Where the pin sits.** `onPopupOpen` sets the view and offsets the map so the pin is in the middle of the strip (96px
   down). A tap on a list row also runs the list opener's second `flyTo` (in `activateRow`), which this task must not change
   and which puts the pin back in the middle of the whole map, 169px down, still inside the strip. A tap on a pin keeps
   it at 96px. Both are inside the strip, which the probe checks.
4. **Not drawn in the detail.** The category badge and the meta glyph (the picture shows the severity pill alone and a
   plain grey meta line). The card has no left severity bar. The title is not clamped (the popup clamps at four lines on a
   wide screen) and takes the display size in the semibold weight.
5. **Meta line and fade line colour.** The picture draws them grey. The sheet is tinted and muted fails 4.5 to 1 on every
   tint in the light theme, so both take the normal text colour. The trust card keeps its white surface, so its own muted
   sub lines stay muted there.
6. **The tint.** The sheet takes the severity tint the popup container already carries, which gives the very light green
   page on a confirmed Low report. An Unconfirmed report's sheet takes the surface grey (the app's own treatment of a
   provisional report) and a hidden report's sheet the page colour, not green.
7. **The severity pill** is the page colour with an outline in the current severity colour and the dot, so it reads on a
   tinted sheet as drawn. A Critical pill gets the dot too.
8. **Vote buttons.** Confirm and Dispute sit side by side, 44px tall each. Confirm at rest is the filled look (the text
   colour fill), Dispute at rest is the page colour with the hairline. The button icons are not drawn at rest. States,
   checked in both themes and both engines:
   - Confirm at rest: filled, no mark. Hover: the fill moves 15 percent toward the page colour.
   - Pressed (Confirm or Dispute): trust.css's filled look kept, plus the button's own mark (check or ban) and a 2px inset
     ring in the page colour. This is what tells a pressed Confirm from the filled Confirm at rest.
   - Busy (disabled): the outlined muted look of trust.css, so a busy Confirm no longer looks filled. A pressed button that
     is busy stays filled (A46).
   - Too far (aria-disabled): the same outlined muted look. A pressed button that is too far stays filled.
9. **Mark resolved** is a centred text link look: no fill, no border at rest, underlined, in the good tone on a Low,
   Medium or provisional sheet (4.53, 4.61 and 4.58 to 1 in the light theme, 7.00 or more in the dark theme) and in the
   normal text colour on a Critical sheet (the good tone is 4.35 to 1 there). It is still a 44px tall button, spans both
   columns, and keeps the disabled and too far looks of trust.css. This departs from A14 (outlined everywhere).
   The resolve confirmation and Reopen keep their look. The too far note and the vote error are centred.
10. **The back arrow.** Leaflet's close anchor is moved to the top left of the strip as a round white 44px arrow with the
    popup shadow token. It keeps the accessible name `Close popup` (the picture draws no word). Updated:
    `TestReadOnlyLabelsRoundedControlsNot` in `web/labels_contract_test.go` (exact head, `phone.css` only), the same kind of
    exemption `.profile-back-link` already has.
11. **Zoom control and theme toggle.** Both step aside while the sheet is open, so the strip shows only the map, the pin
    and the back arrow.
12. **The too far sentence.** The picture draws an always shown grey hint "You need to be within 1 km to confirm or
    dispute." The app's own sentence `Too far to vote. You need to be within 1 km of this report.` is shown only when the
    reader is too far, as everywhere else (A66). No second hint was added.
### Browser harness checks that changed by design

`~/code/pinalert-browser-checks` was run against a build of this worktree (`BUILD_FROM=worktree`). After Task 2 the first
full run gave 359 PASS and 68 FAIL; T2 and T5 P failed only while another browser was running and pass alone. After Task 3
the full run on an idle machine gave 351 PASS, 54 NOTE, 22 NEEDS A PERSON and 76 FAIL. Every FAIL comes from a change
the owner's pictures require, none from a defect:

| Checks | Why they changed |
|---|---|
| T5 V0 and L0 (Tab to the view toggle) | The map and list switch button is dropped on phones, so Tab cannot reach it. |
| T7 feed, T8 layout, pressed look light and dark, forced colors near, motion reduce, motion no-preference and C8, T9 B 320 and B 360, T11 feed row at 320px (T11.8e, 8f, 8m, 8n), T12 widths 320x568, 320x640, 360x740 and 390x844 | Each opens the app at a width under 900px and starts by clicking `#view-toggle`, or presses Confirm on a feed row. The switch is dropped on phones and the row vote controls are hidden there (voting is in the detail, as drawn). C8 fails only because the Confirm press before it never happened. |
| T11.2f, T11.6 (panel closed and open), T11.8g and T11.8h, all on the Activity row at 320px | They expect the old row body of 188px beside the round category badge. The badge is not drawn on phones, so the body is 254px wide, and T11.2f looks for the reporter words, which the phone card hides because the record card shows the same label once. The failing lines of T11.8g and T11.8h are only that width expectation; nothing is reported as clipped or sticking out at 100 or 200 percent text, and the probe shows no sideways scroll at 390px. |
| T9.1 (popup tip centred on the pin, card inside the view), T9.2c (control: the autopan IS animated), T9.4 (320x256 the card scrolls inside itself, tip under the pin), T9.7 (rotating across 900px keeps the tip near the pin), all at 320x568 and 360x740 | These measure a floating popup with a tip beside its pin and Leaflet's own autopan. Under 900px the detail is a full screen sheet with no tip and autopan off. The sheet geometry they report (top 192, full width, bottom at the screen edge) is the one the probe checks. |

The checks at 900px and up are unchanged and pass.

## Departures and choices (Task 3: the Activity page)

The laptop Activity page keeps everything it has. On a phone it is restyled in picture B's look and gains two things that
exist only below 900px: the reporter record card and the Log out text button. Both are in the markup at every width and
hidden by `phone.css` at 900px and up, so the page at 900px and up has the same layout, which the probe against the
baseline confirms.

1. **The record card is the server's own signal.** `internal/api/handlers/auth.go` builds `profileViewModel.Record` from
   the account's own reporter signal (the first Activity row's `Trust.Reporter`, which `ActivityForAccount` gives every row
   from the account's own history). The label is copied as the server wrote it and the counts are the real judged and
   corroborated numbers. No new query, no new service call, nothing recomputed. An account with no reports, or a signal
   with no label, gets no card.
2. **`Reliable reporter` instead of `Reliable`.** The picture's bold word is `Reliable`. The app keeps one wording for the
   tag, so the card shows the server's `Reliable reporter` (and `New reporter`, `Mixed record`, `Unreliable reporter`).
3. **`0 of 0 confirmed` for a new reporter.** The picture shows `5 of 5 confirmed`. A new reporter has no judged reports
   yet, and the real numbers are shown, nothing is invented.
4. **Log out** is a copy of the account header's form (same method and action, one submit button, no other field), a muted
   44px text button on the right of the title. It keeps a right margin of the touch target plus the small space so it
   clears the theme toggle fixed at the top right. The account button itself is hidden on phones.
5. **The email stays** under the title in the muted label size. It is not drawn, but it is an existing laptop item.
6. **The back link is hidden** on phones (the bottom bar replaces it). Its markup stays, once, before the email.
7. **White page, hairline cards.** The picture's grey page would put the muted email and the grey Log out at 4.34 to 1,
   under 4.5. The page stays the page colour and the cards (the record and every report) are white with the 1px border
   token hairline and 8px corners.
8. **State words.** Unconfirmed is the amber tone (5.24 light, 9.33 dark on the page colour), Disputed the red tone (5.44
   light, 6.03 dark), Resolved and Expired the muted colour (4.83 light, 7.66 dark). Live shows no word, because the app
   gives Live no word by contract, so the picture's green Active is not drawn. `Waiting` stays `Unconfirmed`.
9. **The trust line.** One muted line per card: both counters and the currency word, then the meta line (category and
   time). The reporter words are hidden on each row because the record card shows the same server label once. The
   chevron and the clock are not drawn; the toggle still opens the facts panel in place. Reopen keeps its control and its
   error line under the line.
10. **Status lines not built.** The picture's `Needs one more confirmation`, `Resolved by you · yesterday` and `Not
    re-confirmed` are not app strings, so the row shows the counters, the currency word and the meta line instead.
11. **The segmented control is not built** (see Not built). The page lists the account's own reports only, as the laptop
    page does.

## Questions for the owner

Each has a recommendation, in plain words.

1. **The Past tab.** The picture has Feed, Past and Activity. There is no history view yet, so the bar has two tabs.
   Recommendation: add Past when Phase 3.1 builds the history view, and not before.
2. **The category chips over the map.** Flood, Rescue, Shelter, Power and the rest. Recommendation: build them with Phase
   3.1's filters once you say whether a filter may hide critical and rescue reports. Until then there is no filter that
   could hide a rescue report.
3. **My votes.** The segmented control on the Activity page. Nothing in the app can read votes by account yet.
   Recommendation: plan a read of votes by account as its own small piece of work, then add the control.

## Contract tests updated

| Test | File | Change |
|---|---|---|
| `TestRowTitleWrapsLongWords` | `web/labels_contract_test.go` | Lets exactly `#report-list .report-row .report-row__title` in `phone.css` clamp |
| `TestReadOnlyLabelsRoundedControlsNot` | `web/labels_contract_test.go` | Lets exactly two `phone.css` heads use a 50 percent radius: the current tab dot and the detail back arrow |
| `TestMapSelectionNeverScrollsThePage` | `web/popup_contract_test.go` | Doc comment only, names the phone amendment of A87 |

`web/phone_layout_contract_test.go` also pins, for the detail: the sheet rules and the four important flags, the detail
pane creation, the per popup pane and autoPan writes, the resize close, the pan helper standing down and the pin centred
in the strip only behind the phone check, and for the Activity page: the Log out form matching the account header's,
the record card inside the `with` block, the two tab bar, the single back link and the grid rules. New Go tests:
`TestNewProfileRecord` in `internal/api/handlers/profile_record_test.go`, and `TestProfileRendersTheReporterRecord`,
`TestProfileWithoutReportsRendersNoRecord` and `TestProfileRendersLogoutAndBottomNav` in `activity_e2e_test.go`. No other
existing assertion was weakened.

New file `web/phone_layout_contract_test.go` pins: the stylesheet load order, that every phone rule sits in the 899.98px
block (apart from the one grouped hide rule), token only values, no head equal to an exact lookup head, the bottom bar
markup with two tabs and no Past link, the heading and count slot, and the `feed.js` count, distance and list scroll rules.
