# Pinalert design brief (for another AI or designer)

Copy everything below the line into the other AI. It is written to stand alone.

---

## Prompt to paste first

Design how a single emergency report should look in Pinalert, on the map popup and in the list row, in
light and dark mode, at phone width (360 px) and desktop. Follow the rules and content spec below
exactly. Deliver: (1) a report row and a map popup for each state in the states table, (2) the hover,
tap and focus explanation component, (3) the disabled vote button state, (4) the same block on the
Activity page, (5) spacing, alignment, weights and which existing tokens you used. Do not invent new
colors, sizes or components beyond what is listed. Keep every string within the wording given, or
propose plain language alternatives that are shorter, never longer.

## 1. What Pinalert is

A local, verified information feed for emergencies such as floods, cyclones, fires and earthquakes in
India. People near an affected area post short, location tagged updates (a flooded road, an open
shelter, a power outage, someone needing rescue). Other people nearby confirm or dispute each report,
so a report backed by several independent nearby confirmations can be trusted, and an unconfirmed
one is clearly flagged. Reports fade and expire so the feed shows what is true right now. It is a
live, trustworthy alternative to unverified WhatsApp forwards. It is a portfolio project, not a
startup, so the goal is a polished, intentional, credible tool rather than a flashy consumer app.

Who uses it: ordinary people on phones, often stressed, sometimes on weak networks or with the power
out at night. Everyone must sign in with a verified email (magic link) before they can view, report
or vote. Reading has to be fast and unambiguous.

The core promise: when the app says a report is "confirmed", that number must be a true statement
about independent people in different places, and hard to fake.

## 2. Non negotiable design rules (from the owner)

- Neutral utility look: grayscale base. Color is reserved for severity only (green, amber, red). No
  brand color, no alarm look, no decoration.
- It must never look "vibe coded" or AI generated. Forbidden: purple or any gradient heavy
  backgrounds, pill shaped buttons (modest rounded rectangles are fine), fake reviews or invented
  metrics or stats of any kind, hero text or headline banners, emoji used as icons (use real SVG icons
  or plain text), over the top scroll animations.
- No em dashes or en dashes anywhere the user can read (titles, labels, error messages, helper copy).
  Use commas, periods, colons or parentheses.
- Plain language a first time user understands at once ("Confirmed by three what?" is the test).
  Short. Details go in a hover or tap explanation, not on the surface. Not wordy, not clumsy.
- Transitions are smooth but restrained: short hover, press, focus and state changes only.
- Light and dark mode are equal citizens (dark is a distinct palette, not an inverted light one).
  Both must work at every state below. A manual sun or moon toggle exists.
- Accessible: text contrast at least 4.5:1, state is never shown by color alone, touch targets at
  least 44 px tall, everything reachable and understandable by keyboard and screen reader.
- Mobile first. A report row body is only about 236 to 276 px wide on a 320 to 360 px phone.

## 3. Existing visual system (reuse it, do not replace it)

- Font: system UI stack (San Francisco, Segoe UI, Roboto). Only four sizes: 14 px label, 16 px body,
  18 px heading, 24 px display. Only two weights: 400 regular and 600 semibold.
- Spacing scale: 4, 8, 16, 24, 32, 48, 64 px. Modest corner radius (no pills).
- Light palette: background #FFFFFF, surface #F1F3F4, border #E1E4E8, text #1A1D21, muted text #6B7280
  (muted text fails contrast on tinted rows, so trust text uses the main text color), focus ring
  #1A1D21.
- Dark palette: background #0B0D10, surface #16181C, border #2A2D31, text #F3F4F6, muted #9CA3AF,
  focus ring #F3F4F6.
- Severity (traffic light), light / dark: low #2F9E64 / #34D399, medium #C48419 / #E5A93B, critical
  #C0392B / #E5695C. Row tints, light / dark: low #E7F5EC / #123524, medium #FCF3DC / #3A2C10,
  critical #FBE9E7 / #3A1512.
- Category badge: a white glyph on a solid color circle (the map pin pattern). Nine categories with
  simple outlined glyphs: Flood, Earthquake, Fire, Storm or cyclone damage, Road blocked, Power
  outage, Shelter open, Rescue needed, Other.
- Report row: a 4 px colored left border in the severity color plus a faint severity tinted
  background, a small round category badge, then title, then meta line.
- Age fade: as a report nears its expiry it desaturates toward gray in three stages (Fresh, Aging,
  Stale). It is desaturation, not opacity. Critical reports last 24 hours, low and medium 8 hours.
- Severity is also a 1 to 3 slider in the report form: "1 · Low", "2 · Medium", "3 · Critical".
- Map: Leaflet with a vector basemap (OpenFreeMap "liberty" style, with an OpenStreetMap raster
  fallback). Pins are the category badges. Popups are Leaflet popups, maximum about 300 px wide, and
  are rebuilt on every refresh (the feed refreshes every 30 seconds).

## 4. Screens that exist today (context)

- Sign in gate: a slowly rotating globe as a continuous ambient background with the email form on
  top, magic link, no password.
- Main screen: map and list side by side on wide screens; on phones the map is the default with a
  one tap toggle to the list. A floating round "report" button sits over the map with a small sun or
  moon theme toggle below it. A "Show disputed" filter toggle switches the list and map to disputed
  reports only (empty state: "No disputed reports nearby. Reports only show up here if enough nearby
  people have disputed them.").
- Report form: a modal over the map with a draggable pin, an address search box, a 3 by 3 category
  icon grid, the severity slider, a description, and shelter capacity for shelter reports.
- Account menu: opened from a header icon: the signed in email, "Activity", "Log out".
- Activity page: everything the signed in person posted, and their voting history, in one section.
- Toasts for vote results and errors.
- Not built yet, but leave room: clicking a report will later open a full report page like a post on
  X, with a comment thread of what people are saying about the situation.

## 5. The report display (the focus of this brief)

Every report appears in two places with the same content: a **list row** and a **map popup** (opened
by tapping a pin). The Activity page shows the same block on the person's own reports. The reporter
sees exactly what everyone else sees on their own reports, no separate view.

Top to bottom, a report shows:

1. Category badge (glyph on colored circle), severity styling (left border and tint), and the age fade.
2. Title (the report text) and a meta line: category, relative time ("4 min ago"), distance, plus
   shelter status and headcount for shelter reports. Meta can wrap onto several lines.
3. A state chip, only when relevant: **Unconfirmed** (new non critical report still waiting for a
   second independent confirmation) or **Disputed** (only seen in the Show disputed view). A normal
   confirmed report has no chip.
4. **The trust block (new, the main design job).** Two short lines of plain text in the normal text
   color, no chips, bars, meters, stars, icons or color:
   - Line 1, two counters side by side, always both shown, zeros included: **3** confirmed  **1**
     disputed. The number is semibold, the label regular. Examples: `3 confirmed  1 disputed`,
     `0 confirmed  0 disputed`, `12 confirmed  0 disputed`, `99+ confirmed`.
   - Line 2, two signals side by side: a "still current" word and a "reporter" tag, for example
     `Up to date | Reliable reporter`. On a very narrow phone they stack.
5. The vote controls (below the trust block): **Confirm** and **Dispute** for people who did not post
   the report, and **Mark resolved** where allowed. The reporter cannot confirm or dispute their own
   report (those buttons are hidden for them) but can mark it resolved. "Reopen" exists only on the
   Activity page.

### The words (use these; each has a meaning shown on hover)

Still current (line 2, left). The window is one quarter of the report's life (2 hours for 8 hour
reports, 6 hours for 24 hour reports):
- **Up to date**: at least 2 different places confirmed it inside the recent window.
- **Getting old**: it was confirmed by 2 places, but not recently.
- **Needs re-confirming**: it has gone quiet, and the wording invites people nearby to confirm again.
- **Too early to tell**: fewer than 2 places have confirmed it. Only shown on critical and rescue
  needed reports (other unconfirmed reports already carry the Unconfirmed chip). Same neutral wording
  for every severity, never a warning.

Reporter tag (line 2, right). It describes the person's track record on earlier, finished reports:
- **Reliable reporter**, **Mixed record**, **Unreliable reporter**, and **New reporter** (fewer than 5
  finished reports, so no rating yet). "New reporter" must never look like a good tag. The tag is
  public and appears on every report including critical ones, but it is only a label: it never hides,
  dims or blocks a report. The reporter's identity or email is never shown.

### Hover, tap and keyboard explanations (required)

Every counter, word, tag and the Unconfirmed chip shows one short sentence of reason with real
numbers when the person hovers (desktop), taps (touch) or focuses it (keyboard). It must be
dismissible with Escape, stay open while the pointer is over it, and not steal focus. Approved
sentences:
- Confirmed counter: "3 people in 3 different places confirmed this. Counts separate places, not
  votes."
- Disputed counter (proposed): "1 person nearby disputed this."
- Up to date: "2 different places confirmed this in the last 2 hours."
- Reliable reporter: "5 of 7 earlier reports were confirmed by people nearby."
- Unconfirmed chip: "Needs 2 confirmations before this counts as confirmed."
- Too far to vote (see below): "Too far to vote. You need to be within 1 km of this report."
Sentences for the other words and tags follow the same short, factual pattern with their real numbers.

### Map popup extras

The popup shows the same two lines. Its only addition: a small number right after each word on line
2 (score 0 to 100), for example `Up to date 82` and `Reliable reporter 78`. No "/100", no bar, no
icon. When a signal has no score (Too early to tell, New reporter) no number is shown.

### Vote buttons when the person is too far away

A vote from more than 1 km from the report is refused by the server. So, once the person's location
is known, Confirm, Dispute and Mark resolved are shown but disabled for reports over 1 km away, and a
hover, tap or focus shows the "Too far to vote" sentence. Before the location is known the buttons
are normal (the first tap asks for location permission). The reporter's own Mark resolved stays
enabled anywhere. A natively disabled button cannot show a reason, so design this as a visually
disabled state that still receives hover and focus.

### States the design must cover (light and dark, row and popup)

| State | What shows |
|-------|------------|
| Live, well confirmed, current | `12 confirmed  0 disputed`, `Up to date | Reliable reporter`, no chip |
| New non critical report, not yet confirmed (Provisional) | Unconfirmed chip, `0 confirmed  0 disputed`, line 2 shows only the reporter tag, dimmed like an aging report |
| Exactly 1 confirmation | Unconfirmed chip and `1 confirmed  0 disputed` together (accepted), line 2 tag only |
| Confirmed earlier, gone quiet | `4 confirmed  0 disputed`, `Needs re-confirming | Mixed record`, row fades |
| Contested but still live | `5 confirmed  4 disputed`, `Getting old | Reliable reporter` |
| Critical report with no confirmations | `0 confirmed  0 disputed`, `Too early to tell | New reporter`, severity red styling, no extra dimming |
| Critical report that others out dispute | red styling stays, `1 confirmed  3 disputed`, never hidden |
| Disputed and hidden (Show disputed view only) | Disputed chip, both counters, line 2 shows only the reporter tag |
| Own report on the Activity page | same block, plus the reporter's own Reopen action for resolved reports |
| Resolved or expired (Activity only) | counters and reporter tag only, no still current word |
| Reporter with Unreliable tag | same neutral text, no color, no dimming of the row |
| Too far from the report | Confirm, Dispute and Mark resolved visually disabled with the hover reason |

### Fade and severity interaction

Non critical reports fade by whichever is worse: the time based stage or the still current stage
(Up to date is fresh, Getting old is aging, Needs re-confirming is stale). Critical and rescue needed
rows keep only the time based fade so a soft signal never mutes a possible emergency. Color stays
for severity only; trust text never uses color.

### Sample data for mockups

1. Fire near Beach Road junction. Critical. 4 min ago. 0.6 km. 12 confirmed, 0 disputed. Up to date | Reliable reporter.
2. Water over the underpass. Medium. 20 min ago. 1.1 km. 5 confirmed, 0 disputed. Up to date | Mixed record.
3. Road blocked by fallen tree. Low. 6 h ago, expires soon. 2 confirmed, 0 disputed. Needs re-confirming | New reporter.
4. Shelter open at Government School (capacity Limited, about 150 people). Low. 35 min ago. 1 confirmed, 0 disputed, Unconfirmed chip. New reporter.
5. Rescue needed on the terrace of a flooded house. Critical. 2 min ago. 0 confirmed, 0 disputed. Too early to tell | New reporter.
6. Power outage in the whole street. Medium. 3 h ago. 5 confirmed, 4 disputed. Getting old | Unreliable reporter.

## 6. What the reporter tag and counters mean (so labels stay honest)

- "Confirmed" counts different places (about 130 to 150 m squares), not votes and not exactly people:
  ten people in one spot count once. A person can only count if they are within 1 km of the report and
  have a verified email account. So the number is a lower bound on people.
- "Disputed" is counted the same way. Enough disputes hide a non critical report from the main view.
  Critical and rescue needed reports are never hidden by disputes.
- "Still current" is about recency of confirmation, not truth: two different places must have
  confirmed within the recent window.
- The reporter tag is a slow moving track record over the last 90 days of finished reports: confirmed
  by others counts for, disputed counts against, and expiring with no confirmation counts against
  (critical and rescue needed reports are exempt from that last penalty). Under 5 finished reports it
  is "New reporter".

## 7. Constraints so designs are buildable

- Server rendered HTML templates plus vanilla JavaScript. No framework, no build step, no CSS
  preprocessor, no new libraries. Plain CSS with variables.
- Use only the existing tokens, four font sizes and two weights. No new colors.
- Everything text is set as plain text (no HTML from report content). Copy is fixed strings plus
  numbers.
- The feed refreshes every 30 seconds, so the popup and row must tolerate being rebuilt.
- Up to three new keyboard focus stops per row (counters, words, tag), so keep the focus order tidy.
- The longest counter line, `99+ confirmed  99+ disputed`, must wrap gracefully on a narrow phone.

## 8. Ask the design AI to return

Mockups of the states above (light and dark, phone and desktop), the popup, the explanation component
(hover, tap, focus, Escape), the disabled vote state, the Activity page block, plus annotations for
spacing, alignment, weights, tokens used, and any wording you would shorten. Flag anything that would
need a new token or component so it can be discussed before it is built.
