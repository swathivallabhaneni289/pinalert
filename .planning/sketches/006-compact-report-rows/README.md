---
sketch: 006
name: compact-report-rows
question: "How small can the report rows in the laptop list get before they stop being easy to read, with everything else on the page left alone, and what does each size take out of the row?"
winner: A
tags: [feed-row, list, laptop, density, tabs, light-dark]
---

# Sketch 006: Compact report rows

## Winner: A, the phone card (owner, 2026-10-14)

The owner picked size A: "i like A- for the mockups" (by voice; the trailing dash is read as a pause). A row is about 125px,
five rows fit on an 800px laptop screen, and the laptop list uses the card the phone already has. B (79px) was the
recommendation and was not taken. Of the five departures at the end of this file, 1, 2, 4 and 5 apply to A. Departure 3
(severity word first) belongs to B only and does not apply. The build contract is in
`.planning/phases/03.1-report-history-views/03.1-CONTEXT.md` (D-20, D-21).

## Direction (owner, 2026-10-13, by voice)

The owner first said the reports "look really big, can we maybe design it better", and gave a dashboard picture as one
example of a nicer page. A few minutes later they withdrew the picture: "just ignore the picture, but you know what I mean
when I say the list looks really big, that's what I meant. I think I want you to work on that part. Not like you have to
change everything."

So this sketch is only about the rows. The page, the map and the words stay as they are. The one addition is the tab strip
the owner asked for in the same message (Active, Disputed, Past, in place of the "Show disputed reports" checkbox), drawn at
the top of the pane because that is where the rows live. The dashboard picture is not a reference.

## How to view

Serve the repo root with a no cache static server (Safari blocks file pages), then open
`http://127.0.0.1:8799/.planning/sketches/006-compact-report-rows/index.html`. Address parameters: `variant=a|b|c`,
`tab=active|disputed|past`, `theme=light|dark`. Click a row to select it (in C it also opens in place). Names, numbers and
sentences are sample data. The tokens come from `web/static/css/main.css` through `../themes/default.css`.

## Measured today (the real app, scratch copy, eight sample reports, Chromium)

| Where | Row height | What is on screen |
|---|---|---|
| Laptop 1280 by 800 (list pane 624px) | 323px (300px for a quieter row) | two rows and a bit |
| Laptop 1440 by 900 (list pane 704px) | 323px (300px) | two rows and a bit |
| Phone 390 by 844 (the phone card) | 154px | three rows |

A laptop row carries a category badge, a severity label, the whole title, a meta line, a state line, the trust block (both
counters, the currency word and a chevron), Confirm and Dispute, and Mark resolved. The buttons and the trust block are
about 190px of the 323.

## The three sizes (measured in the browser on the same eight reports)

| | A: Phone card | B: Tight card | C: One line |
|---|---|---|---|
| Average row height | 125px | 79px | 48px (an opened row is about 130px) |
| Rows that fit on 800px | 5 | 8 | 8, with room to spare |
| Look | the card the phone already has: dot, severity word, category and time, title up to two lines, one trust line | category badge, title up to two lines, one facts line | severity word, title cut to one line, counters on the right |
| Cost to build | lowest, the phone rules lifted out of the 900px query | low, a new row layout on the laptop | low, plus an open in place state |
| Risk | none | the severity word is no longer the first text of the row | a rescue request is cut after about 40 characters until the row is opened |

What stays in every size: the severity word, the category, the time, both counters and the state word (D-16, and "state is
never colour alone"). The left severity bar and the tint stay, so the page keeps its laptop look.

What leaves the row in every size: Confirm, Dispute and Mark resolved (the opened report and the map popup already have
them, exactly as on the phone), the clock line, the divider and the chevron.

## Recommendation

B. It is a quarter of today's height, shows all eight sample reports on one screen, keeps the category badge that makes the
list easy to scan, and never cuts a title below two lines. A is the cheapest and makes the laptop match the phone. C is the
densest but cuts the text of the most urgent reports, which is the wrong trade in an emergency.

## Departures the build must record (whichever size wins)

1. **A55, a title is never clamped.** A and B clamp at two lines, C at one line until opened. The popup and the opened row show
   the whole text. The phone card already made this departure.
2. **Vote controls on the row.** Removed on the laptop as they were on the phone. Voting is in the opened report. The browser
   checks that vote from a feed row at laptop width (T8, T11.8 and others) change by design, the way they did for the phone.
3. **A26 and A45, the severity word is the first text of the row (B only).** B reads title first and puts the severity word at
   the start of the facts line. If B wins, either the word moves to the front of the title line or the DOM order is facts
   line first with the title placed above it by layout. The UI contract has to pick one.
4. **State line (A75, A81).** The state word moves into the facts line in all three sizes.
5. **Facts chevron.** The row no longer toggles the facts panel in place (the popup holds the trust card). `web/activity_trust_contract_test.go`
   and the feed row trust tests need the same update the phone layout needed.

## What is not decided here

- The Active, Disputed and Past tab strip is drawn only to show where the rows sit. Its wording, counts on the tabs
  (deferred), and the Past tab content (the owner's Past picture, Resolved and Expired groups, last 7 days) belong to the
  Phase 3.1 context and UI contract.
- The phone keeps its own card. This sketch is the laptop list at 900px and up.
