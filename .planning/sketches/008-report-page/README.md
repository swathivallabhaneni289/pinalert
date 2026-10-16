---
sketch: 008
name: report-page
question: "When a reader opens one report, what does its own page look like on the phone and the laptop, with the trust block, the history, comments and Share, inside the approved app layout?"
winner: null
tags: [report-page, trust-block, history, comments, share, phase-8, phone, laptop, light-dark]
---

# Sketch 008: The report's own page (Phase 8)

## Design question

Phase 8 gives every report its own page. This sketch shows it inside a copy of the approved app layout (sketch 007), so
the owner can click from Nearby into a report the same way the real app will work.

## How to open it

Open `index.html` in a browser, or serve this folder and open `/?view=laptop` or `/?view=phone`. Tap a report in the
Nearby list (or its pin): the card shows **Open report**. Tap it to open the report's page.

Handy address parameters: `?view=phone|laptop`, `rp=a|b` (the laptop variant), `open=rep&i=0..5` (open report number
0 to 5 straight away), `share=1` (with `open=rep`, opens Share).

## Variants (laptop only; the phone is the same in both)

- **A: its own page.** The map with only this report's pin on the left, the report on the right, using the whole width:
  the report, its trust block and actions at the top, then "What happened so far" and "Comments" side by side.
- **B: beside the map.** Nearby stays on screen; the list on the left is replaced by the report, and the map keeps every
  pin with this one enlarged. "All reports" goes back to the list. Tapping another pin opens that report.
- **Phone (both):** a full screen page with Back, the title "Report", the bell and the gear, a map strip with the pin,
  then the report, trust block, actions, history and comments stacked.

## What is on the page

1. The report: category icon, title, severity chip and category, place and distance, when it was posted, the
   reporter's own words.
2. The trust block, worded like the app today (`internal/service/trustcopy.go`): Confirmed, Disputed, How current,
   Reporter.
3. Actions: Confirm and Dispute (tap again to take a vote back), or Mark resolved with Undo on your own report (the first
   sample report is yours, as in sketch 007's My activity), plus Share. Quiet links: Stop updates on this report
   (Phase 3.2 muting) and Flag (Phase 4).
4. What happened so far: a timeline built from the vote log. It never names a voter or says where they were.
5. Comments: oldest first, "Reporter" tag on the reporter's own comments, everyone else "Someone nearby", a box with a
   280 letter count and Post. Posting adds the comment to the list in the mockup.
6. Share: a window (a bottom sheet on the phone) showing what people get in WhatsApp: the title and "Open to see if this
   is still true right now", with a link and no counts, so a forward can never show a stale number. Close with the X.

## Changes from sketch 007 besides the report page

- Rescue needed is gone and Traffic took its tile, pin and sample report (owner decision 2026-10-16, quick task
  261016-w8t); the Settings row, first time Alerts step, Show menu note and bell items say "critical" instead of
  "rescue and critical". The What is happening picks show for Critical only.
- The laptop notice strip shows only on Nearby and the report page (03.4 D-29: Nearby and Post a report only; the report
  page belongs to Nearby).

## Open questions for the owner's look

- Laptop: A (its own page) or B (beside the map)?
- Are comments wanted at all, or only the history? Comments are free text and need the Phase 4 screening and limits.
- Is "Someone nearby" the right name for other commenters?

## Checks run

Off screen snapshots at 1440 by 900 (laptop, light and dark) and 390 by 844 (phone), kept in the session scratchpad and
never in the repo: Nearby with a tapped card, both laptop variants, the phone page, Share on the phone, and a scripted
Confirm plus a posted comment (the trust line, the pressed state, the timeline and the comment count all updated).
