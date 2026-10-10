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
