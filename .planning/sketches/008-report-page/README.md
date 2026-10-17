---
sketch: 008
name: report-page
question: "When a reader opens one report, what does its own page look like on the phone and the laptop, with the trust block, the history, comments and Share, inside the approved app layout?"
winner: "B (laptop, beside the map); the phone keeps its full screen page. Locked by the owner 2026-10-17"
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
the map and that report should open", then "the laptop ver is not letting me click the report on the map and it should
open"): the pin opens its report straight away, on the phone and the laptop alike. On the laptop it opens beside the
map, and tapping another pin switches to that report. (A small "Open report" label over the pin was tried first on the
laptop and dropped: it needed a second click.)

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
5. Comments, shown clearly on the page (owner, 2026-10-17: "allow the replies", then "i want how instagram comments
   work", then "i dont need the likes i just want you to display the comment section clearly without making everything
   too clumsy", then "it looks good can we also maybe have the like thing? but not heart? and maybe we can add it for the
   laptop too"):
   - **The same list on the phone and the laptop**, right on the report page under a "Comments" heading with the count
     (replies included). No preview, no separate comments screen.
   - **Like is a thumbs up**, not a heart, in the small line under each comment right after Reply, with the count beside
     it, on both sizes. Grey like Reply; once liked the thumb is filled and in the text colour, so the state is never
     colour alone and no severity colour is used. Tap again to take it back. Rules to keep when building: one like per
     account per comment, comments only (never on the report itself), and likes never feed Confirmed, the reporter score
     or any trust number; a thumbs up on a comment must never read as Confirm.
   - Each comment: a round picture, the username, the time, the words and **Reply**. On the laptop the username and the
     words share one line with the time and Reply under them (as on Instagram on a computer); on the phone the username
     and time sit on the top line with the words under them, then Reply (as in the Instagram app), so a narrow screen
     reads cleanly. The reporter's picture carries the report's icon and colour plus a small "Reporter" tag (never colour
     alone), as other people see it. **Your own comments show only your username**: never "You", and no Reporter tag even
     on your own report (owner, after the lock: "having both you and reporter doesnt make sense. if the person you reporter
     commented it should just be just the username"). The mockup's sample username for you is priya.vja.
   - **Replies are folded** behind "View N replies", and "Hide replies" folds them again. One level only: Reply on a
     reply joins the same thread, and posting a reply opens its thread.
   - **One box.** Reply puts "@username " in the box, moves the cursor there and shows "Replying to username" with an X
     to cancel. Replying to your own comment adds no @name. A reply starting with @username shows the name in bold. Post
     turns solid once there is text; Enter posts and Shift with Enter starts a new line. The box grows as you type, and
     "N left" shows only in the last 40 of the 280 letters. The box is a rounded field with Post inside it on both
     sizes (owner, after the lock, of the plain laptop box: "i dont like this part"); on the laptop it is pinned to the
     bottom of the panel, on the phone it sits after the last comment.
   - A like, an open thread and a half typed comment all survive a vote.
   - Sample commenters have short usernames (ravi.k, anjali.r and so on); see "Owner decisions" for names.
   - Tried and dropped the same day: a red heart in its own column on the right of each comment, and on the phone a
     short preview that opened a separate comments sheet. The owner found both clumsy.
6. Share: a window (a bottom sheet on the phone) showing what people get in WhatsApp: the title and "Open to see if this
   is still true right now", with a link and no counts, so a forward can never show a stale number. Close with the X.

## Changes from sketch 007 besides the report page

- Rescue needed is gone and Traffic took its tile, pin and sample report (owner decision 2026-10-16, quick task
  261016-w8t); the Settings row, first time Alerts step, Show menu note and bell items say "critical" instead of
  "rescue and critical". The What is happening picks show for Critical only.
- The laptop notice strip shows only on Nearby and the report page (03.4 D-29: Nearby and Post a report only; the report
  page belongs to Nearby).
- Tapping a pin on Nearby opens its report straight away, on the phone and the laptop (owner, 2026-10-17). The
  highlighted card in the list keeps its own Open report button. Phase 8 adds this; 3.4 builds the pin tap as sketch 007
  has it. The hint in the mockup's top strip now says to tap a pin or a report.

## Quieter pass (owner, 2026-10-17: "make sure it doesn't look too busy")

Nothing was taken away. The trust table became the popup's trust block, the three muted lines under the title became
two, the laptop A page became one reading column instead of two side by side, the timeline lines got shorter, the empty
comments box and the always open text box went, and the section headings got smaller.

## Owner decisions (2026-10-17), locked

The owner locked this design on 2026-10-17 ("yes i like it lets lock this"): the report page, the trust block, the
history, the comments with replies and thumbs up likes, and Share, as they stand in this sketch. Changes from here go
back to the owner first.

- Laptop: **B, beside the map.** A stays in the file behind `?rp=a` for comparison only.
- Comments are wanted, **with replies**, shown clearly on the page in an Instagram like layout, with a **thumbs up like, not a heart**, on both sizes (item 5). They still need the Phase 4
  screening and per account limits, so they ship after the page and the history.
- Tapping a pin on the map opens its report, on the phone the same as on the laptop ("i like the laptop version").
- Names: every commenter has a username and **everyone sees it** (owner: "we can let the username be seen to everyone
  cause they can see it while your commenting", and commenting and replying "should work exactly like how instagram lets
  you comment and reply"). As on Instagram there are two fields: the **Username** is unique, public and shows on every
  comment; the **Name** stays optional and shows only on a public profile (Phase 9). The username is **editable in
  Settings, About you**, and checks as you type (owner: "it should check if that name is available or taken"): Checking…,
  then "is available" with Save turned on, "is taken. Try another.", "Use 3 to 20 letters, numbers, dots or underscores.",
  or "That name is kept for official use." for names like police, ndma, imd, govt, official or pinalert, so nobody can pose
  as an authority. A saved username shows on your comments at once. It passes the same screening as comments (Phase 8
  scope note 4).
- **Sign up asks for all three, and they are required** (owner, 2026-10-17: "i think we should make the username, name
  and address set up mandatory", "and that should be where their trying to sign up to use this website"). Step 4 of the
  first-time flow, About you, which runs right after the first sign in (03.4 D-27), now asks for the Username (with the
  same live check), the Name and the Home area. There is no Skip: Continue stays off until the username is available and
  the name and home area are filled in. "Address" is kept as the home area, a town or area and never a street, as the
  app already promises; it is filled in from the location step. Reading stays open without an account; the requirement
  is only for signing up. People who signed up before this ships are asked to fill in the missing ones once, at their
  next sign in. Building it: **all of it ships with Phase 8**, decided with the 3.4 session on 2026-10-17. Phase 3.4
  stays as built (step 4 keeps Skip; name and home area optional and stored, D-24b), and Phase 8 flips step 4 to
  required for all three fields at once, adds the username with its check, and asks earlier accounts to fill in the
  missing ones once. Doing name and home area in 3.4 would change step 4 twice and make existing accounts fill it in
  twice. So the Skip seen in the 3.4 build is expected until Phase 8.

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
Fourth round, phone set up as in Instagram (2026-10-17): the page shows View all 6 comments, two one line previews and Add a comment… with no sideways scroll (page 390 of 390); Add a comment… opens the sheet with the cursor in it; the sheet opens at 497 of 731px, a drag up of 100px goes full (731px), a drag down of 150px closes it; light and dark checked. The laptop box has no border (unchanged look).
Fifth round, phone spacing (2026-10-17): the gaps in the block under the report measure 4px, 4px, then 17px to Add a comment (it has a picture); the page stays 390 of 390 wide; in the sheet Reply to View replies is 25px centre to centre; dark checked; the laptop is unchanged.
Sixth round, clear list and no likes (2026-10-17): no heart anywhere on the phone or the laptop; the phone shows the full list under "Comments 6" with the box after it, in the phone frame and at 390px wide, with no sideways scroll; a reply posts into the right thread (4 replies, count 7); dark checked; the laptop only lost the hearts.
Seventh round, thumbs up likes (2026-10-17): on the phone a like on a comment and on a reply both show filled with the count up by one (6 to 7, 5 to 6) and stay after a Confirm; five like buttons with one thread open, each 32 by 44; no sideways scroll; the laptop shows the same thumbs up after Reply, checked in dark.
Eighth round, pins and lock (2026-10-17): on the laptop a pin tap opens its report beside the map (Traffic), a second pin switches it (Road blocked), and after All reports another pin opens again (Shelter); layout A and the phone also open on a pin tap; no pin label is left in the page.
Ninth round, laptop box (2026-10-17, asked by the owner after the lock): the laptop box is now the same rounded field as the phone, fully visible at the panel bottom (897 of 900px) with Replying to above it; the mockup hint is short again so the top strip stays on one line; light and dark checked.
Tenth round, your own name (2026-10-17, asked by the owner after the lock): your comment on your own report reads "priya.vja Water is still rising..." with no Reporter tag on the laptop and the phone; a reply to your own comment adds no @name and posts as priya.vja; suresh.patamata still shows Reporter to you on the shelter report.
Eleventh round, username editor (2026-10-17): kiran_77 shows taken, police.help shows kept for official use, ab shows the rule, Priya.Floods turns into priya.floods and shows available with Save on; saving changes the Settings row and your comment to priya.floods; checked on the laptop and the phone.
Twelfth round, sign up (2026-10-17): step 4 with nothing typed keeps Continue off and shows the username rule; ravi.k shows taken; ravi.patamata available but no name keeps Continue off; all three filled turns it on; clearing the home area turns it off again on the laptop; no Skip button on the phone or the laptop.
