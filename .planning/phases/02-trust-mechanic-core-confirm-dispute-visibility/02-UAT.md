---
status: testing
phase: 02-trust-mechanic-core-confirm-dispute-visibility
source: [02-VERIFICATION.md]
started: 2026-09-16T14:42:29Z
updated: 2026-09-16T14:42:29Z
---

## Current Test

number: 1
name: D-18 GPS-denial hard block, live in a real browser
expected: |
  Denying the browser's location prompt on Confirm/Dispute greys the buttons out and immediately
  re-enables them, shows the exact GPS-denial sentence in the row's .vote-error element, and sends
  NO network request to /api/reports/{id}/confirm|dispute (verified in DevTools Network panel — a
  request sent and then rejected is a failure even if the visible outcome looks similar). A granted
  prompt is cached in sessionStorage and not re-requested for a second vote in the same tab, but is
  re-requested in a new tab. Both the feed row and the map popup behave identically, and no state
  changes before the server responds (D-04).
awaiting: user response

## Tests

### 1. D-18 GPS-denial hard block, live in a real browser
expected: Denying the browser's location prompt on Confirm/Dispute greys the buttons out and
  immediately re-enables them, shows the exact GPS-denial sentence in the row's .vote-error
  element, and sends NO network request to /api/reports/{id}/confirm|dispute (verified in DevTools
  Network panel — a request sent and then rejected is a failure even if the visible outcome looks
  similar). A granted prompt is cached in sessionStorage and not re-requested for a second vote in
  the same tab, but is re-requested in a new tab. Both the feed row and the map popup behave
  identically, and no state changes before the server responds (D-04).
result: [pending]

### 2. D-09/D-10/D-11 Provisional dimming+label, Hidden outline treatment, and the "Show disputed reports" toggle, live in a real browser and both themes
expected: A fresh non-critical report shows BOTH a desaturated badge/border AND an explicit
  "Unconfirmed" chip, on both the feed row and the map pin popup, and the dimming visibly wins over
  the report's actual age stage. A critical report shows neither treatment. With "Show disputed
  reports" unchecked, a disputed report is absent from both list and map; checking it reveals the
  disputed report with the outline treatment (transparent badge, neutral ring, muted glyph, no
  severity tint) on both surfaces simultaneously, findable against the basemap; unchecking removes
  both together. The checkbox does not persist checked across a reload. An empty disputed result
  shows the specific "No disputed reports nearby" copy, not a blank list. All of the above holds in
  dark mode too.
result: [pending]

### 3. TRUST-08 Mark Resolved / confirmation-gate / Reopen flow, live in a real browser
expected: Tapping "Mark resolved" on someone else's report replaces the button set in place with an
  inline confirmation and sends NO request until the affirm button is tapped; Cancel restores the
  row with no request. Affirming as a non-reporter shows the "recorded, awaiting agreement" toast
  and the report stays in the feed; affirming as the reporter shows the "resolved" toast and the
  report disappears from both list and map immediately. The map popup offers the same three
  buttons, wrapping rather than causing a horizontal scrollbar. The Activity page lists the
  resolved report with the outline/"Resolved" treatment and a Reopen control; tapping Reopen shows
  the reopen-succeeded toast (never the pending one) and the row updates with no page reload, and
  the feed shows the report live again. Denying GPS on affirm hard-blocks with the GPS-denial copy
  and sends no request.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
