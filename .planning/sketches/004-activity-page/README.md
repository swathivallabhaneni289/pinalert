---
sketch: 004
name: activity-page
question: "With the owner's reference picture adapted to Pinalert's own tokens, does the Activity page still read at a glance at 360 and 640 wide, in light and dark, while keeping the confirmed counts, the state word and the Reopen button on each row?"
winner: null
tags: [activity, tabs, rows, trust-block, light-dark]
---

# Sketch 004: Activity page

Sample data throughout. Every name, number and sentence is made up. Nothing in this folder changes the app, and no
file under `web/` or `internal/` was touched.

## Design question

The owner supplied two pictures on 2026-10-09 as the look they want for the Activity page. This sketch adapts that look
to Pinalert's own colours, sizes and corners and asks one thing: does the page still read at a glance at 360 and 640 wide,
in light and dark, with the trust pieces Phase 3 requires still on each row?

## How to view

Serve the repo root with a server that sends `Cache-Control: no-store` (Safari blocks `file://` pages, and the stage links
the app's real stylesheets by relative path), then open the board:

```
http://127.0.0.1:8799/.planning/sketches/004-activity-page/index.html
```

The board shows the page at phone width and at 640, in light and dark, with the rules check live under each frame.
`stage.html` is one frame, driven by the query parameters `theme`, `tab`, `filter`, `counts`, `data`, `text` and `id`.

## The reference, in words (the project keeps no images)

**Picture 1, the look.** A page called Activity with one line of help under it. A row of text tabs with a count chip on
each: All, My posts 3, My votes 5, My disputes 2, Resolved 8, Not resolved 4. The selected tab (All) is a white bordered
button with a dark underline. Under the tabs, a list of white cards with a thin coloured bar on the left. Each card has a
round coloured icon (flame, tick, chat bubble, house, ban sign, clock, link), a small grey tag (Your post, Your vote, Your
dispute, System update), a bold title, a grey line such as "You posted this report, 4 min ago, 0.6 km", a chip on the right
(Critical, Medium, Low, or Resolved and Not resolved on the system rows) and an arrow. A left side menu (Map, Reports,
Alerts, Activity, Account), a top bar with a bell and an account icon, a panel called "What each tab shows" and a phone
example complete the picture.

**Picture 2, the tab meanings.** All: a chronological feed combining your reports, votes, disputes and important updates
to reports you follow. My posts: every report you created, including whether it is active, resolved, expired or hidden.
My votes: reports you confirmed or disputed, each entry showing what you did and the report's current status. Resolved and
Active: quick filters for the current state of reports you posted, kept as filters within My posts and not as separate
top level tabs.

## Variant

One variant, **A: The picture, adapted**, on purpose: the owner's decisions were already made (see below), so there was
nothing to compare. The one live choice is a board switch, "Counts, state word and reporter words on each row", set to
Kept (the sketch's recommendation) or Like the picture.

## Decisions taken in this sketch (the owner can veto each in one word)

| Decision | Why |
|----------|-----|
| Three tabs: All, My posts, My votes. My posts has an All, Active, Resolved filter. Disputes sit inside My votes. Say "six" for the picture's six tabs. | Picture 2 recommends exactly this, and three tabs fit at 320 wide with no scrolling. |
| Each row keeps the trust block, the state word and the reporter words, and the poster's resolved row keeps Reopen. Say "like the picture" to drop them. | Dropping them leaves nowhere to reopen a report, and the counts are the point of the app. TRUST-07 and D-20 require both trust signals on every report without opening anything, so dropping them needs a recorded amendment. |
| No distance on rows. | The server stores no reader location and this page is server rendered. |
| No left side menu or top bar. | That belongs to the app wide design pass (backlog 999.10) and to Phase 5's Alerts work. |
| The round icon keeps its colour for severity, not for the kind of event. The kind is the small label (Your post, Your vote, Your dispute, Update). | The design rule is colour for severity only. |
| The small label is plain text, not a tag. | The row already has two rounded labels (severity and state), and sketch 003 took the tag look off the reporter words for the same reason. |
| Update rows are drawn dashed and labelled as arriving with Phase 3.2. | They need the in app updates, which do not exist yet. Phase 8 adds the "Comment added" kind. |
| The arrow opens `/?report=ID`, the link Phase 3.1 adds. The whole card also opens it for a pointer or a finger. | The link is the keyboard and screen reader way in, and it is one target and one name for the head line and the title together. |

## What to look for

1. At 360 wide, does a row read in one pass: who did what (the small label), how serious (the severity label), what it is
   (the title), when (the line under it), how well backed (state word, reporter words, counts)?
2. Switch the counts switch to Like the picture. The page is calmer, and also says nothing about Disputed or Resolved.
3. Do two chevrons on one row confuse? The one beside the title points right and opens the report. The one in the trust block
   points down and turns.
4. Larger text (150 and 200 percent): the tab strip scrolls sideways instead of wrapping, and the label pair wraps onto two lines.
5. The wide frame: the title and the trust block use the full width.

## What changes from the Activity page that ships today (the UI contract has to record each)

1. Rows are neutral cards (page colour, hairline, 12px corners) with the 4px severity bar, not flat rows tinted by severity.
   The map list rows keep their tint, so the two now differ until the design pass.
2. The first line is the small label on the left and the severity label on the right. The severity label was the first line
   of the body. The title carries an arrow, and the first two lines are one link.
3. The email line leaves the page. The account menu already shows it.
4. Tabs, count chips (8px corners) and the Active and Resolved filter are new. The selected tab is underlined with a 1px line,
   not the picture's 2px, because the shared rules allow hairlines and the 4px severity bar and nothing thicker.
5. The trust block is variant H of sketch 003, unchanged, reused by reference.

## Data this needs (for the Phase 3.2 planner)

- **My posts** is already served: `ActivityForAccount` returns every report the account filed.
- **My votes** is a new account scoped read over the `votes` table: the latest content vote (confirm or dispute) per report.
  The poster's own resolve and reopen belong under My posts. The only votes index is report first, so an account scoped read
  would want a new index, which is a migration the owner runs.
- **Tab counts** are real: My posts is the number of reports filed, My votes the number of distinct reports voted on.
- **All** is built from posts, votes and, once Phase 3.2 exists, updates, newest first. One report can appear more than once
  (posted, then updated).
- **The arrow target** is Phase 3.1's `report` query name. Held or removed reports appear in no view.
- **Update rows** need Phase 3.2's updates table. "Comment added" needs Phase 8.

## Open points for the Phase 3.2 discussion

1. Past (expired) posts show under All only in this sketch. Do they need a filter of their own?
2. A Disputed (hidden) post counts as Active here, because nothing has closed it. Is that right?
3. Should All carry a count? The picture's All has none.
4. Should the tab and the filter live in the address (`?tab=posts&filter=resolved`) so Back and a bookmark land on the same view?
5. Should the whole card open the report, or only the title line?
6. If the owner chooses Like the picture, TRUST-07, D-20 and `activity_trust_contract_test.go` need a recorded amendment first.

## Rules check

`shared/rules-check.js` (sizes 14, 16, 18, 24, weights 400 and 600, spacing scale, contrast 4.5 to 1, hairlines and the 4px
bar, no gradients, no heavy shadows, 44px targets, tokens, no long dashes, no emoji, no sideways overflow) plus this
sketch's own checks (corners of 8px or less on tabs, chips and filter buttons, the tab pattern, one 44px link per row, the
trust pieces as asked, Reopen only on the poster's resolved row, the two labels on one line, nothing scrolls sideways).

Measured in WebKit (Safari's engine) on 2026-10-09: widths 320, 360 and 640, light and dark, the three tabs, counts kept and
dropped, the posts filters, and the empty state, all clean at the normal text size. At 150 and 200 percent text the tab strip
scrolls sideways and the label pair wraps, by design. Pointer and keyboard behaviour (tabs, arrow keys, Home and End, the
filter, the row press, the trust toggle, Reopen) was driven in WebKit and Chromium. In Safari the Tab key skips buttons and
links unless "Press Tab to highlight each item on a webpage" is ticked in Settings, Advanced.
