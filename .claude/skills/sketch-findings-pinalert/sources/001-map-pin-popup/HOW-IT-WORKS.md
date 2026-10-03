# How the map pin popup works (variant d4)

Open the sketch with `stage.html?variant=d4` (add `&state=unconfirmed` or `&theme=dark` to see other cases).
Parts marked SKETCH are shown working in the mockup. Parts marked BACKEND need server work that does not exist
yet: the counts, the two signals and the 1 km rule are planned for Phase 3, and comments need their own phase.

## Opening and closing
- Tapping a pin opens the popup above it, 340 pixels wide. On a phone narrower than 372 pixels it shrinks to the
  screen width minus 32 (328 on a 360 phone, 288 on a 320 phone). The map pans so the whole card is visible. SKETCH
- The X at the top right closes it. Escape closes it too. Tapping another pin replaces it. SKETCH
- If the trust explanation is open (see below), Escape closes that first, and a second Escape closes the popup. SKETCH

## Reading the card, top to bottom
1. Severity badge: a solid circle in the severity colour with the category icon (a white flame on the red circle,
   in light and in dark). Next to it a fully rounded label says Critical, Medium or Low. Critical is red text on a
   pink tint, as in the reference. Medium and Low keep the word in the normal text colour, because their hue is too
   faint to read as text, and show the hue on the tint and on a small dot before the word. A report nobody has
   confirmed yet also gets a plain outlined Unconfirmed label.
2. Title: what the person wrote. Long titles stop at four lines.
3. Meta line: category, how long ago, how far from you, in quiet grey. A shelter adds a second line with its capacity.
4. Trust block, two rows of plain text (next section): "12 confirmed | 0 disputed", then the two quality signals
   side by side, "Up to date 82 | Reliable reporter 78".
5. Three actions: Confirm and Dispute side by side, Mark resolved under them. Outlined buttons with an icon each.
6. Comments footer: a speech bubble, "Comments" over "2 comments" in quiet grey, and "View comments" on the right.

The coloured bar down the left edge repeats the severity and runs the full height, footer included. Colour is used
only for severity. As a report ages the bar and badge fade towards grey. The card has 12 pixel corners and a soft
shadow; buttons have 8 pixel corners.

## The trust block and its explanation
The two rows are one single button, about 76 pixels tall, that looks like plain text: no border, no fill. In the counts
row the words are regular and the numbers semibold, both in the normal text colour. In the signals row the words
are quiet grey and the numbers semibold. A thin vertical line separates the two counts, and another separates the
two signals. Tap anywhere on it and a panel of four short plain sentences opens right under it. Tap it again or press
Escape and the panel closes. The button announces expanded or collapsed. SKETCH

| Line | Sentence (sample wording) |
|---|---|
| 12 confirmed | 12 people in 12 different places confirmed this. Counted by place, not by vote. |
| 0 disputed | No one nearby has disputed this. (1: "1 person nearby disputed this.") |
| Up to date 82 | 2 different places confirmed this in the last 2 hours. |
| Reliable reporter 78 | 7 of 9 earlier reports were confirmed by people nearby. |

The numbers in the first two come from the report. The last two are placeholder figures for the sketch: the real
window and the reporter record must come from the server. BACKEND (Phase 3)
Counts above 99 show as 99+. A score the server cannot give yet is simply left off ("Too early to tell",
"New reporter") and its sentence says why. A report with no "Up to date" signal shows three sentences, not four.
If you vote while the panel is open, the sentences update in place.

When the two signals do not fit side by side, the second drops under the first and its thin line disappears. On a
360 phone the critical sample fits on one row; the longer labels (contested, long text) stack. On a 320 phone every
sample stacks. The two counts always stay on one row.

## The three actions
- Confirm and Dispute work like switches. Tap once to add your vote, tap again to take it back. Tapping the other
  one moves your vote across. The pressed button turns solid dark (not a colour change) and the count in the trust
  block updates.
  Screen readers hear it as pressed. SKETCH for the look; saving the vote and counting only separate places is BACKEND.
- Mark resolved asks first. The three buttons are replaced by "Mark this report resolved?" with Cancel and then
  Yes, mark resolved, side by side. Cancel is a neutral button. Yes uses the app's critical red, the same destructive
  style the app already has. If the card is too narrow for both, Yes drops under Cancel. Cancel brings the three
  buttons back. Yes shows "Marked resolved." SKETCH. Who may resolve, and what
  resolving does to the feed, is BACKEND.
- Your own report: no Confirm or Dispute (you cannot vote on yourself). Mark resolved stays, full width. SKETCH
- Too far away: you must be within 1 km to vote. All three buttons go grey and a sentence under them says
  "Too far to vote. You need to be within 1 km of this report." The reasoning is always visible, never only in a
  greyed button. SKETCH for the look; the 1 km check itself is BACKEND (Phase 3).

## The comments footer
The whole footer is one link to the full report and its thread, under a thin line. In dark mode it sits on a slightly
lighter surface than the card, as in the reference. In light mode it keeps the card surface and only the thin line
sets it apart, because the quiet grey count text would fall under the 4.5 to 1 contrast floor on the light grey.
Hover and press fill the row, turn the count to the normal text colour, thicken the underline and nudge the arrow.
- No comments: "No comments" with the link "Add a comment", so an empty thread still invites opening it.
- One comment: "1 comment" and "View comment".
- Many: "2 comments" and "View comments".
SKETCH for the link and wording. Comments themselves are BACKEND (their own phase).

## Live updates
The report list refreshes every 30 seconds. A refresh must never close an open popup, move the focus, or collapse
an open explanation. If the open report changed, its numbers update in place. If it was resolved or expired, the
card stays until the person closes it, with the new state shown. BACKEND, and a rule the build must test.

## Keyboard and screen reader
- Tab order: the trust block (one stop), Confirm, Dispute, Mark resolved, the comments link, then the close button
  (Leaflet places it last in the page). Escape closes the popup while focus is inside the card.
- Every target is at least 44 by 44 pixels. Focus shows a dark outline in both themes.
- The trust block announces expanded or collapsed, with a hidden hint that it explains the numbers. When the panel
  opens, it is the next thing in reading order after the button.
- Confirm and Dispute announce pressed or not pressed. Disabled buttons are tied to the reason sentence.
- After Mark resolved is chosen, focus moves to Cancel (it is first, before Yes, in the page and on screen). After
  Yes, focus moves to "Marked resolved." SKETCH
- Severity is a word in the label, never colour alone. Motion is short, and off if the person asked for less motion.

## States at a glance

| State | What shows |
|---|---|
| Critical | Red bar, badge, red Critical pill. Full trust block with both scores. |
| Unconfirmed | Low pill with a dot plus an Unconfirmed label, dimmed badge, only the reporter signal. |
| Too early to tell | Up to date line reads "Too early to tell" with no number. Reporter reads "New reporter". |
| Gone quiet | Grey bar and badge. Line reads "Needs re-confirming" with a low number. |
| Contested | Confirmed and disputed both high. Signals read "Getting old" and a low reporter number. |
| Your own | Mark resolved only, full width. |
| Too far | All buttons disabled with the 1 km sentence under them. |
| Long text | Title cut at four lines, counts show 99+, signals stack. Nothing overflows. |
