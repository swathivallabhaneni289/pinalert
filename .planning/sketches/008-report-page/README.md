---
sketch: 008
name: report-page
question: "When a reader opens one report, what does its own page look like on the phone and the laptop, with the trust block, the history, comments and Share, inside the approved app layout?"
winner: "B (laptop, beside the map); the phone keeps its full screen page"
tags: [report-page, trust-block, history, comments, share, phase-8, phone, laptop, light-dark]
---

# Sketch 008: The report's own page (Phase 8)

## Design question

Phase 8 gives every report its own page. This sketch shows it inside a copy of the approved app layout (sketch 007), so
the owner can click from Nearby into a report the same way the real app will work.

## How to open it

Open `index.html` in a browser, or serve this folder and open `/?view=laptop` or `/?view=phone`. Tap a report in the
Nearby list: the card shows **Open report**. Or tap its pin on the map (owner, 2026-10-17: "if we click on the report on
the map, it should let me open the report", then "makes sure the phone ver also should work the same if anyone clicks on
the map and that report should open"):
- **Phone:** the pin opens its report straight away.
- **Laptop:** while the list shows, a small label with the title and **Open report** appears over the pin; once a report
  is open beside the map, tapping any pin opens that report.

Handy address parameters: `?view=phone|laptop`, `rp=a|b` (the laptop variant, B by default), `open=rep&i=0..5` (open report number
0 to 5 straight away), `share=1` (with `open=rep`, opens Share).

## Variants (laptop only; the phone is the same in both)

- **A: its own page.** The map with only this report's pin on the left, the report on the right in one reading column
  (at most 720px wide): the report, its trust block and actions, then "What happened so far", then "Comments".
- **B: beside the map (the owner's pick, 2026-10-17: "i like beside the map").** Nearby stays on screen; the list on the left is replaced by the report, and the map keeps every
  pin with this one enlarged. "All reports" goes back to the list. Tapping another pin opens that report.
- **Phone (both):** a full screen page with Back, the title "Report", the bell and the gear, a map strip, then the
  report, trust block, actions, history and comments stacked. The map strip keeps every pin with this one enlarged, the
  same as the laptop map beside the report, and tapping another pin opens that report.

## What is on the page

1. The report: category icon and title, then two short lines: the severity chip with the reporter words beside it
   (sketch 003 H, as shipped), and one muted line with category, place, distance and how long ago. Then the reporter's
   own words.
2. The trust block, the same one as the popup (sketch 003 H, as shipped), which is what Phase 8 success criterion 1 asks
   for: "N confirmed | N disputed" on row one, how current on row two, a chevron, and the sentences one tap away, worded
   like `internal/service/trustcopy.go`. It stays open while you vote. (The first version used a four row table; it was
   the busiest thing on the page and did not match the popup.)
3. Actions: Confirm and Dispute (tap again to take a vote back), or Mark resolved with Undo on your own report (the first
   sample report is yours, as in sketch 007's My activity), plus Share. Quiet links: Stop updates on this report
   (Phase 3.2 muting) and Flag (Phase 4).
4. What happened so far: a timeline built from the vote log. It never names a voter or says where they were.
5. Comments that work like Instagram's (owner, 2026-10-17: "allow the replies", then "i want how instagram comments
   work"):
   - Each comment: a round picture, then the **bold username and the words on one line**, then a small grey line with
     the time ("27m") and **Reply**. A **heart** on the right likes it, with the count under it; liked is a filled red
     heart. The reporter's picture carries the report's icon and colour plus a small "Reporter" tag (never colour alone).
   - **Replies are folded** behind "View N replies" (a short line before it), and "Hide replies" folds them again. One
     level only: Reply on a reply joins the same thread, and posting a reply opens its thread.
   - **One box for everything.** Reply puts "@username " in the box, moves the cursor there and shows "Replying to
     username" with an X to cancel. Replying to your own comment adds no @name. A reply starting with @username shows the
     name in bold. Post is a word that turns solid once there is text; Enter posts and Shift with Enter starts a new line.
     The box grows as you type, and "N left" shows only in the last 40 of the 280 letters.
   - **Laptop:** the comments sit in the panel, oldest first, with the box pinned to the bottom of the panel, as on
     Instagram's post view on a computer.
   - **Phone:** the page shows "View all N comments" (or "No comments yet. Add one"), which opens a comments sheet from
     the bottom with the box pinned inside it, as in the Instagram app. Close with the X, a tap outside or Escape.
   - The count beside Comments includes replies. A like, an open thread and a half typed comment all survive a vote.
   - Likes are new. The rules to keep when building: one like per account per comment, comments only (never on the
     report itself), and they never feed Confirmed, the reporter score or any trust number. A heart must never read as
     Confirm.
   - Departures from the house rules, copied from Instagram on purpose: the liked heart is red, which the MANIFEST keeps
     for severity. Left out: Instagram's emoji reaction row above the box (no emoji in this app).
   - Sample commenters have short usernames (ravi.k, anjali.r and so on) because Instagram shows one on every comment.
     Who may show a name is the open question below.
6. Share: a window (a bottom sheet on the phone) showing what people get in WhatsApp: the title and "Open to see if this
   is still true right now", with a link and no counts, so a forward can never show a stale number. Close with the X.

## Changes from sketch 007 besides the report page

- Rescue needed is gone and Traffic took its tile, pin and sample report (owner decision 2026-10-16, quick task
  261016-w8t); the Settings row, first time Alerts step, Show menu note and bell items say "critical" instead of
  "rescue and critical". The What is happening picks show for Critical only.
- The laptop notice strip shows only on Nearby and the report page (03.4 D-29: Nearby and Post a report only; the report
  page belongs to Nearby).
- Tapping a pin on Nearby opens its report: straight away on the phone, through a small label with Open report on the
  laptop while the list shows (owner, 2026-10-17). On the phone a label could sit under the half open list, so the pin
  opens the report itself. The highlighted card keeps its own Open report button. Phase 8 adds this; 3.4 builds the pin
  tap as sketch 007 has it.

## Quieter pass (owner, 2026-10-17: "make sure it doesn't look too busy")

Nothing was taken away. The trust table became the popup's trust block, the three muted lines under the title became
two, the laptop A page became one reading column instead of two side by side, the timeline lines got shorter, the empty
comments box and the always open text box went, and the section headings got smaller.

## Owner decisions (2026-10-17)

- Laptop: **B, beside the map.** A stays in the file behind `?rp=a` for comparison only.
- Comments are wanted, **with replies**, and they should **work like Instagram's** (item 5). They still need the Phase 4
  screening and per account limits, so they ship after the page and the history.
- Tapping a pin on the map opens its report, on the phone the same as on the laptop ("i like the laptop version").
- Names: the owner answered "yes" to the recommendation, read as: every commenter has a short username, asked for the
  first time they comment (not at sign up, so posting a report stays quick). It passes the same screening as comments
  (Phase 8 scope note 4). The full profile page stays opt in (Phase 9). To be confirmed in discuss-phase 8.

## Open questions for the owner's look

- None left from this sketch.

## Checks run

Off screen snapshots at 1440 by 900 (laptop, light and dark) and 390 by 844 (phone), kept in the session scratchpad and
never in the repo: Nearby with a tapped card, both laptop variants, the phone page, Share on the phone, and a scripted
Confirm plus a posted comment (the trust line, the pressed state, the timeline and the comment count all updated).
Second round (2026-10-17): the trust panel stays open after Confirm and shows "1 confirmed" with the one person sentence;
the pin label shows on the laptop and the phone and opens the report; a scripted reply posts into the right thread,
"Show 1 more reply" opens the thread, Reply on a reply puts the box in that thread with focus in it, and every Reply tap
area measures 44px.
Third round, Instagram style (2026-10-17): a like survives a vote (7 likes, red heart); Reply on a reply fills
"@suresh.patamata " with focus in the box and shows "Replying to suresh.patamata"; the posted reply lands as the fourth
reply in the right thread and the count goes from 6 to 7; Reply on your own comment adds no @name and Enter posts it into
that thread; View and Hide replies work; the laptop box sits flush with the panel bottom with the last comment above it;
on the phone "View all 6 comments" opens the sheet, its box sits at the sheet bottom, and a username never breaks across
lines; the heart is 32 by 44 and Reply 44 tall; light and dark checked.
