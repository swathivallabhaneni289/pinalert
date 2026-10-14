---
sketch: 004
name: activity-page
question: "Can the Activity page look exactly like the owner's picture, as a working page at desktop and phone width, and what does copying it change for the build?"
winner: "B"
tags: [activity, tabs, rows, side-menu, picture, light-dark]
---

# Sketch 004: Activity page

Sample data throughout. Every name, number and sentence is made up. Nothing in this folder changes the app, and no file
under `web/` or `internal/` was touched.

## Owner direction, 2026-10-10: exactly as pasted

The owner sent two pictures on 2026-10-09 as the look they want for the Activity page. The first preview adapted that look to
Pinalert's own tokens and kept the confirmed counts. The owner's answer was: "I wanted to look exactly how I pasted so do that
accordingly." So **variant B is the direction** and variant A is kept only for comparison. The earlier decisions in this
sketch (three tabs, counts kept, no distance, no side menu, icon colour for severity) are withdrawn. Where the picture
conflicts with the house rules, this README records the conflict instead of adapting the design.

**Size, same day.** The owner then asked: "i want it to look like the picture but decrease the size accordingly and update it."
Variant B now draws the desktop page at 80 percent of the picture's measured size: title 32 instead of 40, rows about 98
high instead of 122, round icons 51 instead of 64, side menu 221 wide instead of 276. It is one unit, `--u` (0.8px), in
`picture.css` and in the board, so the size moves with one number (the phone layout breakpoint follows it, 900 times 0.8 is
720). The phone layout keeps the picture's own phone sizes, which are already about 80 percent of the desktop ones. The sizes
quoted under "Departures" are the picture's own, so multiply them by 0.8 for what is drawn.

**Approved.** The owner answered "ok" to variant B at 80 percent on 2026-10-10. Nothing in this sketch is pending with them.
The build comes with Phase 3.2, after Phase 3 closes and Phase 3.1 adds the report link.

## How to view

Serve the repo root with a server that sends `Cache-Control: no-store` (Safari blocks `file://` pages, and variant A links the
app's real stylesheets by relative path), then open the board. Variant B is the default tab.

```
http://127.0.0.1:8799/.planning/sketches/004-activity-page/index.html
```

`picture.html` is variant B alone: a responsive page, desktop layout from 720px wide and phone layout below it. `stage.html`
is one frame of variant A.

## The reference, in words (the project keeps no images)

**Picture 1, the look.** A screen with a top bar (a dark logo square and "Pinalert", a pill with "127.0.0.1" and a reload icon,
a bell with a red dot, a round account button), a side menu (Map, Reports, Alerts, Activity highlighted, Account), a page
called Activity with one line of help, six text tabs with a count chip on each (All, My posts 3, My votes 5, My disputes 2,
Resolved 8, Not resolved 4) with All as a white pill with a dark bar under it, and seven white cards with a thin bar on the
left. Each card has a round coloured icon (flame, tick, chat bubble, house, ban sign, clock, link), a grey tag (Your post, Your
vote, Your dispute, System update), a bold title, a grey line ("You posted this report, 4 min ago, 0.6 km"), a chip on the
right (Critical, Medium, Low, Resolved, Not resolved) and an arrow. To the right of the screen, a box titled "What each tab
shows" with a chip and one sentence for each of the six tabs, and below it a phone example (status bar, back arrow and
Activity, tabs clipped at "My dispu", three rows with shorter sub lines).

**Picture 2, the tab meanings.** All: a chronological feed of your reports, votes, disputes and important updates to reports
you follow. My posts: every report you created, active, resolved, expired or hidden. My votes: reports you confirmed or
disputed, with what you did and the report's current status. Resolved and Active: quick filters for reports you posted, kept
inside My posts. Picture 1's own box lists six top level tabs, which is what variant B draws.

## Variant B: exactly the picture

Built in the picture's own sizes and colours, measured from it, with the system font stack. The board shows the picture's
composition scaled to the window: the page at 922 wide (1152 at 80 percent), the phone layout at 446 wide, and "What each tab shows" as board text
beside them, never in the page. The seven rows and the six tab counts are the picture's. A tab selects itself and filters the
seven rows with the meaning the picture's box gives it (My votes is confirmed or disputed, My disputes is disputed, Resolved is
marked resolved, Not resolved is still active). The counts do not add up to the seven rows, and no rows were invented to make
them. Tabs work by mouse and by keyboard (arrow keys, Home, End), and a press on a row says where it would go.

**Choices where the picture contradicts itself.** The image generator drew rows 5 to 7 shorter, with a lighter title weight, and
gave the left bars a mix of grey, amber and black. Variant B uses the height of rows 1 to 3 for every row, one semibold title
weight, one rule for the left bar (it takes the colour of the round icon, which gives the red bar the phone example shows on
its first row and the amber bar row 6 has), and one column for the sentences in the right box. The chevron, the typeface (the
picture's is not one the app ships) and exact letter widths differ a little for the same reason. The 127.0.0.1 pill is the
browser's own address bar, captured from the screenshot the picture was made from. It is drawn because the owner asked for
exact, it does nothing, and it cannot ship as a dead control.

## Departures the build must record

One run of `shared/rules-check.js` on variant B, not a pass or fail gate. It fails on most items by design, because the picture
does not use the app's tokens. Each item needs a recorded amendment to the UI contract, or a change to the picture, before the
page is built in the app. For this page only, the owner's picture overrides the house rules.

- **Type sizes** off the 14, 16, 18, 24 scale: 40 (title), 23 (logo name), 19.5 and 19 (help line, side menu), 15, 14.5 (phone
  title), 13, 12, and 26 (phone title). **Weights** 500 and 700, where the house has 400 and 600.
- **Spacing** off the 4, 8, 16, 24, 32, 48 scale in about 160 places (35, 43, 20, 17, 27 and others).
- **Corners** over 8px: 9, 10, 14 and 16 on cards, tags, chips and the selected tab, and fully round count chips, which are the
  pill shapes the site design rules forbid for buttons.
- **Targets** under 44px: the bell (40), the account button (41), the phone tabs (38), the back arrow (22).
- **Contrast** under 4.5 to 1 in two places, both from the picture's own colours: the amber "Medium" chip text on its tint
  (about 3.0 to 1) and one grey line (about 4.2 to 1). The app's own rule is 4.5 to 1 for every piece of text.
- **Colours**: every colour is the picture's own, none is an app token, and the round icons and bars are coloured by the kind
  of event, where the house rule is colour for severity only.
- **Borders**: a 2px left bar on every card, where the house allows hairlines and the 4px severity bar.

## Consequences for the build (carried into the Phase 3.2 note)

- The rows show no confirmed counts, no state word and no reporter words, so the Phase 3 rule that both trust signals show on
  every report without opening anything (TRUST-07, D-20) and `activity_trust_contract_test.go` need an amendment first.
- Reopen has no place on the row. The poster's way to undo a resolve needs a new home, most likely the opened report.
- The distance needs a reader location. The server stores none and the page is server rendered.
- The side menu and top bar mean a new app shell, which is the design pass (backlog 999.10) or part of Phase 3.2.
- The address bar pill needs a decision at the discuss step: drop it, or turn it into search.
- Five of the six tabs and the "System update" rows need data that does not exist yet: votes by account, and the updates of
  Phase 3.2. "Comment added" needs Phase 8.

## Variant A: adapted to Pinalert's look (kept for comparison)

The first preview. Neutral cards, three tabs, the confirmed counts, the state word and Reopen kept, no distance, no side menu,
icon colour for severity. It reuses sketch 003's trust block (variant H). Its README sections from the first round are
superseded by the owner direction above. Its live rules check passes at the normal text size at 320, 360 and 640 wide, in
light and dark.

## What was checked

Variant B was driven in WebKit (Safari's engine) and Chromium: no console errors, no sideways scroll at 1152, 900, 730, 700, 446 and 360
wide, the desktop layout from 720 up and the phone layout below it, tabs by mouse and by keyboard, a row press, and the board
composition scaling to a 1300 wide window. It was compared by eye with the picture, region by region, in two rounds. It has not
been opened in the owner's own Safari window. In Safari the Tab key skips buttons and links unless "Press Tab to highlight each
item on a webpage" is ticked in Settings, Advanced.

## Built: quick task 261013-wm5 (Activity page with six tabs)

The app's Activity page (`/profile`) now has the six tabs of variant B, at laptop and phone width, on real data. Files:
`web/static/css/activity.css` (the laptop look and the shared card rules), `web/static/js/activity-tabs.js`,
`web/templates/profile.html.tmpl`, the phone rules in `web/static/css/phone.css`, and on the server migration 00006, the
`VotedReportsByAccount` query, `ActivityPageForAccount` and `internal/api/handlers/activity.go`. Laptop copies variant B at
80 percent with one variable, `--u: 0.8px`. Phone copies the owner's picture 05 look in app tokens, as sketch 005 does.

### The 03.2 open points, answered

1. **Past and Hidden reports.** A report that expired without being resolved, and an expired hidden report, sit only under
   All and their kind tab (My posts, or My votes and My disputes). Resolved wins over Expired, so a resolved report is
   under Resolved whether or not it expired. Not resolved is a report that is neither expired nor resolved, so Live,
   Unconfirmed and Disputed ones that are still active.
2. **All carries a count**, the number of distinct reports on the page.
3. **The tab is in the address.** All is `/profile`, the others `/profile?tab=posts`, `votes`, `disputes`, `resolved` and
   `not-resolved`. The server matches the key exactly and case sensitively. Anything else (empty, unknown, upper case,
   padded) shows All. The tabs are links, so they work with no script. `activity-tabs.js` switches in place, writes the tab
   with `history.replaceState` (a filter adds no Back steps, as the feed filter does), and Back and a bookmark land on the
   same tab because the server renders it.
4. **The whole card opens the report** is the rule for later. Nothing handles `/?report=ID` yet (the app reads only
   `show_disputed` from the address), so the arrow is not drawn and the card opens nothing. The cursor on a card is the
   default one.
5. **Reopen stays on the card**, on the account's own resolved, unexpired posts only. A voted card never offers it. After a
   Reopen the chip and the state word update at once from the response. Counts and tab membership stay as rendered until
   the next load.

### What is not built, and why

- **System update rows.** Phase 3.2's updates table does not exist.
- **Comment added.** Phase 8.
- **The distance on the sub line.** The server stores no reader location.
- **The arrow and the open report link.** See point 4.
- **The top bar, the side menu, the address pill, the bell and the avatar.** Each would be a dead control or needs the
  app wide shell (backlog 999.10). The existing account button, back link, email line and theme toggle stay.
- **Arrow keys between tabs.** The tabs are plain links in a nav, so Tab and Enter work and nothing roves.

### Departures from the picture and from the house rules

Laptop first, then phone.

- **Type sizes and weights** off the 14, 16, 18, 24 scale and the two weights, at 80 percent: title 32, help line 15.6,
  tabs 12.8, tag 11.2, title 14.4, sub line 12.8, chip 12, count 11.2, weights 500 and 700.
- **Targets.** The picture's tabs are 35px at this size. They are 44px tall (`min-height: 44px`, not 44u), a fix.
- **Spacing and corners** off the scale, in `calc(N * var(--u))`: cards 11.2px corners, the selected tab 12.8px, the tag
  and the chip 8px, the card bar 1.6px (the picture's 2u).
- **Fully round count chips** (`calc(13 * var(--u))`). They are counts inside a link, not buttons, so the pill rule for
  buttons does not apply, but the shape is the picture's and it is recorded here.
- **Colours** are the picture's own, as variables on the page, and **the round icon and the left bar are coloured by the
  kind of event** (post #D15744, vote #459471, dispute #788089), where the house rule is colour for severity only. The
  white glyph on those three measures 4.07, 3.67 and 4.00 to 1, above the 3 to 1 that a graphic needs.
- **Weights 500 and 700** on the tabs and the title, where the house has 400 and 600.
- **Cards grow past the picture's height.** The picture's card is 122u high. The page keeps both trust counters, the state
  word, the trust panel and Reopen, so a card has `min-height: calc(122 * var(--u))` and grows with them.
- **The severity word moved into the chip.** The body starts with the tag and then the title. The chip holds three words,
  the severity word, Resolved and Not resolved, and CSS shows exactly one by the card's display state class
  (`vis-retracted` shows Resolved, `vis-hidden` shows Not resolved, anything else shows Critical, Medium or Low), so the
  chip follows a Reopen with no script. Where the chip says Resolved, the state word is not drawn (one fact once). A
  Disputed state word and a Not resolved chip are both drawn, because they are two facts.
- **Reporter words** are hidden on the account's own posts at both widths, because the record card shows the account's
  label once. They show on voted cards, which are another person's record.
- **The record card** sits between the help line and the tabs at laptop width in the same card language, and stays on the
  phone. It is built from the account's own posts, never from the merged card list.
- **The app's own words are kept**: "1 hour ago", "Reliable reporter", "Unconfirmed". The help line is kept as drawn
  ("A quick view of what's happening with your reports.") although the page now also lists voted reports.
- **The chat bubble icon.** `web/static/icons/message-square.svg` is one more Lucide product icon for Your dispute, of the
  same kind as the others in that folder. It is not a picture.
- **Phone strip.** A grey track (`--color-surface`) with a white selected tab (`--color-bg`), tab text in the normal text
  colour because the muted colour reads 4.34 to 1 on the track, 44px tabs, and a strip that clips and scrolls inside
  itself while the page never scrolls sideways.
- **Phone card.** A 48px event icon, the tag with the state word right after it, and the chip at the right edge of the
  first line. The title, the sub line, the trust line and Reopen follow from the body column to the card's right edge. The
  action phrase ("You posted this report") is hidden on phones, because the tag says the same. The help line is hidden.
  At 320px a long state word or chip drops to the next line instead of overflowing.
- **Dark theme.** The picture is light only. Every Activity colour maps to the app's dark tokens (card surface, border,
  text, muted text, the dark severity tints for the chips). The event colours stay the same in both themes.

### Contrast, computed (WCAG ratio, 4.5 to 1 for text)

| Pair | Light | Dark |
|---|---|---|
| Title on page | 19.22 | 17.68 |
| Help line on page | 5.74 | 7.66 |
| Card title on card | 18.49 | 16.15 |
| Sub line on card | 4.67 | 7.00 |
| Tab on page | 5.27 | 7.66 |
| Tab on hover fill | 4.78 | 7.00 |
| Selected tab text | 19.35 | 16.15 |
| Count on its chip | 7.54 | 12.57 |
| Tag on its fill | 5.93 | 12.57 |
| Critical chip | 4.54 (picture's #C6473E gave 4.21, darkened to #BE433A) | 5.02 |
| Medium chip | 4.61 (picture's amber gave 3.00, now the app's caution tone #946200) | 6.51 |
| Low chip | 4.57 | 7.00 |
| Resolved chip | 5.15 | 12.57 |
| Not resolved chip | 4.61 | 5.02 |

`web/activity_page_contract_test.go` recomputes every pair from the shipped values and fails under 4.5 (3.0 for the icon
glyph pairs).

### Contract tests that changed, and what each now allows

| Test | File | Change |
|---|---|---|
| `TestMigrationsEmbedded` | `internal/store/migrations_test.go` | A 00006 block like the 00005 one. Nothing weakened. |
| `TestActivityForAccountReadsReporterHistoryOnce` | `internal/service/auth_test.go` | The call log gains `VotedReportsByAccount` second, because the page reads the votes in the same pass. Every other assertion kept. |
| `TestProfileTemplateUsesServerSeverityAndTrust` | `web/activity_trust_contract_test.go` | The body starts with the tag and then the title, and the chip carries the severity word once. Trust JSON, expired and hook checks kept. |
| `TestProfileRendersSeverityLabel` | `internal/api/handlers/activity_e2e_test.go` | The same departure in the rendered page. The sev class check kept. |
| `TestProfileWithoutReportsRendersNoRecord` | `internal/api/handlers/activity_e2e_test.go` | The visible empty state of an empty account is the All tab's. "No reports yet" stays as the hidden My posts one. No record card, as before. |
| `nonCategoryIcons` | `web/css_contract_test.go` | Registers `message-square.svg`. |
| `phoneBaseHideHeads`, `TestPhoneRulesLiveInTheNarrowBlock` | `web/phone_layout_contract_test.go` | Four phone only classes instead of five. |
| `TestPhoneActivityStylesheet` | `web/phone_layout_contract_test.go` | Grid areas gain a tabs row and an empty row. The state word has margin 0. |

The external browser harness (`pinalert-browser-checks`), its T11 Activity checks, change by design with the new card and
were not run for this task. Run it before relying on its Activity results.

### Questions for the owner

1. **Resolved and Not resolved include the reports you voted on**, as the six tab picture shows, not only your own posts.
   Recommendation: keep it. Narrowing it later is a one line change.
2. **When you press Mark resolved on someone else's report, it does not show under My votes.** The picture says My votes
   means reports you confirmed or disputed. Recommendation: keep it.
3. **The laptop picture has a side menu and a top bar.** Not built. Recommendation: build them later with the app wide
   design pass (backlog 999.10), because today Reports, Alerts, Account, the bell and the address pill would lead nowhere.

### The one thing only the owner can do

Apply migration 00006 to your own databases (`make migrate` with your own `DATABASE_URL`). It adds one index. The page works
before you do; the votes read is just a sequential scan until the index exists.

### What was checked

Full suite on three fresh disposable databases, 1407 passed, 0 failed, 0 skipped each time at the end. In Chromium and
WebKit, on a scratch server with seeded data: the Activity page at 1280 by 800 (six tabs on one row, each 44px tall, title
32px, icon 51px, chip right of the body, record card present, no arrow, no top bar or side menu), at 390 by 844 and at 320
by 568 (no sideways page scroll, the strip scrolls inside itself at 390, the selected tab is the page colour on the grey
track), each tab clicked (the visible cards equal the count, the address becomes the tab's address, `/profile` for All, the
page does not scroll), Back from another page landing on My votes, the trust panel opening on a card that was hidden at
load, Reopen changing the chip from Resolved to the severity word with the toast, and the dark theme at 390 and 1280. The
feed page at 1280 by 800 and 900 by 800 shows zero computed differences between the base build and this build, in both
engines, over the app shell, panes, map, report row and its badge, add button, theme toggle, header, toast, bottom bar and
feed heading.
