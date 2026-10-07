---
status: testing
phase: 03-trust-model-hardening-diversity-weighted-trust
source: [03-VERIFICATION.md]
started: 2026-10-07T20:33:33.016Z
updated: 2026-10-07T20:33:33.016Z
---

## Current Test

number: 1
name: Trust counters and words (V-42 step 5)
expected: |
  Row and popup go 0, 1, 2 confirmed. The lead sentence disappears at 2 and Up to date appears with the popup score. The critical report with one place reads Too early to tell with no extra dimming.
awaiting: user response

## Tests

### 1. Trust counters and words (V-42 step 5)
detail: Trust counters and words (V-42 step 5). As A post a low severity report at R (12.9716, 77.5946), confirm as B then C (C from 12.9686, 77.5946). Post a critical report and confirm it once as B.
expected: Row and popup go 0, 1, 2 confirmed. The lead sentence disappears at 2 and Up to date appears with the popup score. The critical report with one place reads Too early to tell with no extra dimming.
why_human: Needs three signed-in profiles, a live server and Chrome location overrides. Tests prove the server values, not the rendered browser result.
result: [pending]

### 2. Panel open and close (step 6)
detail: Panel open and close (step 6). Open the trust panel by click, tap, Enter and Space, close by Escape, on a feed row and in a popup. Listen with VoiceOver.
expected: Panel opens and closes on all four inputs. VoiceOver reads the toggle name then the hint 'Shows what these mean.'
why_human: Keyboard and screen reader behavior. The web contract tests are static source checks and do not run the browser JS.
result: [pending]

### 3. Escape by surface ownership (step 7, plan 03-19)
detail: Escape by surface ownership (step 7, plan 03-19). Walk each focus position with a popup panel and a row panel open.
expected: First Escape closes the panel and focuses the toggle, the second closes the popup. Popup panel closes before a row panel when focus is in the popup.
why_human: Focus order across two surfaces is runtime browser behavior.
result: [pending]

### 4. Too far hint and refusal (steps 8 and 9, plans 03-04 and 03-18)
detail: Too far hint and refusal (steps 8 and 9, plans 03-04 and 03-18). Vote from 2 km, check the buttons, force a POST from the console, test the 0.9896 km and 1.0008 km boundary pair, wait out the 120 second bound, test silent refresh, Ask and Block location states.
expected: Refusal text 'Too far to vote. You need to be within 1 km of this report.' with HTTP 403 field location. Buttons show off within one poll, stay Tab reachable, do nothing on click. 12.9805 accepted, 12.9806 refused. Hint clears after the bound. Own Mark resolved stays enabled from far. Blocked location shows the locked sentence and sends no POST.
why_human: Needs Chrome Sensors, permission prompts and the Network panel. Server refusal is proven by TestCastVoteTooFar, but the browser hint and cache are not.
result: [pending]

### 5. Mark resolved confirmation and focus (step 10, plan 03-09)
detail: Mark resolved confirmation and focus (step 10, plan 03-09). Keyboard only on a feed row, then as reporter in list view at phone width and from the map popup at desktop width.
expected: Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body.
why_human: Focus management and screen reader announcement.
result: [pending]

### 6. Reopen on Activity (step 11, plan 03-15)
detail: Reopen on Activity (step 11, plan 03-15). Resolve own report, open Activity, open the panel, press Reopen.
expected: Counters show 0 and 0 with no numbers, panel opens with server sentences, Reopen and the Resolved label go, panel stays open with refreshed sentences, focus on the trust toggle.
why_human: Live focus and refresh behavior.
result: [pending]

### 7. Width and wrapping (step 12, plan 03-16)
detail: Width and wrapping (step 12, plan 03-16). At 320px and 200 percent text size, with a long unbroken title and URL.
expected: No horizontal scroll in the 236px feed body or the 188px Activity body. Severity word shows before the title and is read first by a screen reader. Labels stay readable and unclipped.
why_human: Layout and screen reader order.
result: [pending]

### 8. Vote layout and button look (steps 13 and 14, plan 03-11)
detail: Vote layout and button look (steps 13 and 14, plan 03-11). 600px row, 900px two pane layout, 320px Activity Reopen, light and dark, forced colors, reduced motion.
expected: Confirm and Dispute each about half width, Mark resolved full width. Pressed Confirm is the inverted fill. Forced colors shows glyphs, Highlight fill, GrayText disabled, bordered Yes. No hover transition under reduced motion.
why_human: Visual and media feature rendering.
result: [pending]

### 9. Popup card, position, reduced motion, keyboard and focus (steps 15 to 18, plans 03-17 and 03-19).
detail: Popup card, position, reduced motion, keyboard and focus (steps 15 to 18, plans 03-17 and 03-19).
expected: Panel and confirmation survive a 30 second poll with focus unmoved. Popup tip centred and card inside the view at 320 and 360px, controls not covered after autopan, no animated pan under reduced motion. Pins have names and open on Enter and Space. Focus survives poll removal and reorder. Focus returns to the marker or row on close.
why_human: Map geometry, autopan, poll timing and focus are runtime browser behavior.
result: [pending]

### 10. Look and feel and plain language check (step 19)
detail: Look and feel and plain language check (step 19). Check the toggle press fill on a Provisional row, forced colors, both themes. Ask someone who has not seen the app what the numbers mean.
expected: The two signals read as separate things (still current versus reporter) and the person finds the panel without help.
why_human: Visual judgment and comprehension, which no automated check can give. This is the check that the goal of a reader understanding the signals is met in practice.
result: [pending]

### 11. Owner housekeeping (step 20 and reminders)
detail: Owner housekeeping (step 20 and reminders). Run migrate once on the owner dev database for migration 00005 after merge. Drop p3_uat when finished.
expected: Index idx_reports_session_expires exists on the dev database.
why_human: Touches the owner's own database, which the executors and this verifier are not allowed to use.
result: [pending]

## Summary

total: 11
passed: 0
issues: 0
pending: 11
skipped: 0
blocked: 0

## Gaps
