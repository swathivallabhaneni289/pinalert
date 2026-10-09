---
status: testing
phase: 03-trust-model-hardening-diversity-weighted-trust
source: [03-VERIFICATION.md]
started: 2026-10-07T20:33:33.016Z
updated: 2026-10-09T17:39:44.000Z
---

## Current Test

number: 5
name: Mark resolved confirmation and focus (step 10, plan 03-09)
expected: |
  Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body.
awaiting: the popup focus fix (see Gaps, test 5), then one human session for VoiceOver and the other human only parts

## Tests

### 1. Trust counters and words (V-42 step 5)
detail: Trust counters and words (V-42 step 5). As A post a low severity report at R (12.9716, 77.5946), confirm as B then C (C from 12.9686, 77.5946). Post a critical report and confirm it once as B.
expected: Row and popup go 0, 1, 2 confirmed. The lead sentence disappears at 2 and Up to date appears with the popup score. The critical report with one place reads Too early to tell with no extra dimming.
why_human: Needs three signed-in profiles, a live server and Chrome location overrides. Tests prove the server values, not the rendered browser result.
note: Run 2026-10-08 in Safari on the scratch server (port 8792, database p3_uat, built from b8f0c94); Chrome is not installed on this Mac. The owner posted the low report in the browser as account A. The confirms of voters B and C, and the critical report with its one confirm by B, were sent through the API by Claude (B at the report itself, C 333 m south) while the owner watched the rendered row and popup. Server values at each step: 0 confirmed with the lead sentence; 1 confirmed with the lead sentence; 2 confirmed, Live, Up to date, score 99, no lead sentence; critical report Live by the critical rule, 1 confirmed, Too early to tell, no score. The owner confirmed the critical report row on screen (1 confirmed, Too early to tell, no Unconfirmed label, not dimmed) and answered pass. Owner design remarks from this test are listed under Gaps.
result: pass

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
note: Scripted browser run 2026-10-09 (Chromium and WebKit, server built from 99d246d, own database and port, nothing in the repo touched). Chromium 23 PASS, 1 FAIL, 1 NEEDS A PERSON. WebKit 22 PASS, 1 FAIL, 2 NEEDS A PERSON. Proven in both engines: the buttons go off within one poll (aria-disabled) and stay reachable by Tab, click, Enter and Space send nothing, a forced POST gets 403 with field location and the exact sentence, the boundary pair (12.9806 refused, 12.9805 accepted), the 120 second bound (on at 30, 60 and 90 seconds, off at 120, on a fake clock), the silent refresh, the Block state (no POST, locked sentence) and own Mark resolved enabled from far. Chromium also proves the Ask state sends no location request. One cosmetic FAIL, see Gaps. Still needs a person: Ask in Safari (no permission prompt pops up) and the prompt UI itself. Correction to the V-42 script: step 8(f) is out of date, a tap on a too far button does refresh the fix silently (WR-03).
result: issue
reported: "Scripted run, not typed by the owner: after the hint clears at the 120 second bound, the earlier refusal line 'Too far to vote. You need to be within 1 km of this report.' is still on screen (check T4 S1.11b, both engines)."
severity: cosmetic

### 5. Mark resolved confirmation and focus (step 10, plan 03-09)
detail: Mark resolved confirmation and focus (step 10, plan 03-09). Keyboard only on a feed row, then as reporter in list view at phone width and from the map popup at desktop width.
expected: Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body.
why_human: Focus management and screen reader announcement.
note: Scripted browser run 2026-10-09 (Chromium and WebKit). Chromium 34 PASS, 1 FAIL, 2 NEEDS A PERSON. WebKit 30 PASS, 1 FAIL, 2 NEEDS A PERSON. Proven in both engines at desktop and phone width: focus lands on Cancel, the group has its accessible name in the accessibility tree, Cancel returns focus to Mark resolved, Escape works, Yes with a pending resolve keeps focus on that row's Mark resolved, and removal from the list moves focus to the next row, the previous row or the view toggle. One FAIL: resolving from the map popup at default motion leaves focus on the page body (reduced motion passes), see Gaps. Still needs a person: VoiceOver speaking "Mark this report resolved?, group, Cancel, button", and a re-check of the popup resolve after the fix. Tip for the owner: in Safari plain Tab skips buttons, so hold Option while pressing Tab, or tick the Advanced setting.
result: issue
reported: "Scripted run, not typed by the owner: resolving from the map popup at default motion ends with focus on the page body instead of the map container (check T5 P4, both engines)."
severity: major

### 6. Reopen on Activity (step 11, plan 03-15)
detail: Reopen on Activity (step 11, plan 03-15). Resolve own report, open Activity, open the panel, press Reopen.
expected: Counters show 0 and 0 with no numbers, panel opens with server sentences, Reopen and the Resolved label go, panel stays open with refreshed sentences, focus on the trust toggle.
why_human: Live focus and refresh behavior.
note: Run 2026-10-09 in Safari on the scratch server (port 8792, database p3_uat, built from 99d246d). The first test report (id 3) had expired, because low severity reports last 8 hours and it was made the night before, so the app correctly hid Reopen on it. A fresh report (id 9, resolved through the API as account A) was used instead. The owner saw the row first (Low, the Resolved label, 0 confirmed, 0 disputed, the Mixed record tag, the Reopen button, no score numbers), opened the panel, pressed Reopen and confirmed all six outcomes: the message "Report reopened.", the label changing from Resolved to Unconfirmed, the Reopen button going away, the panel staying open with refreshed text, an outline on the trust block, and the counters still 0 and 0. Server log: POST /api/reports/9/reopen answered 200 at 23:06:47 and vote 6 was written; the server then describes the report as Unconfirmed with the lead sentence "Needs 2 confirmations before this counts as confirmed.". The scripted run also passes this check in Chromium and WebKit (5 PASS each).
result: pass

### 7. Width and wrapping (step 12, plan 03-16)
detail: Width and wrapping (step 12, plan 03-16). At 320px and 200 percent text size, with a long unbroken title and URL.
expected: No horizontal scroll in the 236px feed body or the 188px Activity body. Severity word shows before the title and is read first by a screen reader. Labels stay readable and unclipped.
why_human: Layout and screen reader order.
result: [pending]

### 8. Vote layout and button look (steps 13 and 14, plan 03-11)
detail: Vote layout and button look (steps 13 and 14, plan 03-11). 600px row, 900px two pane layout, 320px Activity Reopen, light and dark, forced colors, reduced motion.
expected: Confirm and Dispute each about half width, Mark resolved full width. Pressed Confirm is the inverted fill. Forced colors shows glyphs, Highlight fill, GrayText disabled, bordered Yes. No hover transition under reduced motion.
why_human: Visual and media feature rendering.
note: Scripted browser run 2026-10-09. Chromium 25 PASS, 0 FAIL, 1 NEEDS A PERSON. WebKit 16 PASS, 0 FAIL. Proven: at a 600px vote row Confirm and Dispute are 296px each and Mark resolved is the full 600px on its own line, at 900px two pane the controls are 350px with Confirm and Dispute 171px each, the 320px Activity Reopen wraps to 2 lines with no clipping, the pressed Confirm fill is the inverted text token in light and dark and stays so while in flight, there is no hover transition under reduced motion (both engines), and under forced colors in Chromium the glyphs show, the fill is Highlight, disabled is GrayText, Yes is bordered and the alert glyph shows. Not provable here: real Windows High Contrast (emulation proves the CSS rules and the owner is on a Mac). Left for the owner: a quick look at the button style in Safari in the final session.
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
passed: 2
issues: 2
pending: 7
skipped: 0
blocked: 0

## Gaps

Observed live on 2026-10-08 during test 1 (Safari, scratch server on port 8792, built from b8f0c94). The counts and
words of test 1 behaved as expected through two confirmations (0 and 1: Unconfirmed with the lead sentence; the server
reports Live, Up to date, score 99 at 2). The three items below are owner design remarks, cosmetic, and belong with
test 10 (look and feel). They change locked Phase 3 UI decisions, so the fix plan must record each as an explicit
amendment to 03-UI-SPEC.md.

- truth: "The words inside the trust block (confirmed, disputed, the still current word and the reporter tag) are not underlined"
  status: failed
  reason: "User reported: all of these are underlined, including confirmed, disputed, and new reporter. I don't understand why we are underlining it. I don't like that part. Cause found while logging: the whole two row block is one button, and web/static/css/trust.css .trust-block__toggle:hover sets text-decoration: underline, so every word in the block underlines while the pointer is over it. It comes from the d4 sketch and 03-UI-SPEC.md A48 (hover underline, press fill). Whether it also shows with the pointer away is to be confirmed with the owner."
  severity: cosmetic
  test: 1
  artifacts: []
  missing: []
- truth: "The reporter tag (for example New reporter) sits in the top part of the box, beside the Unconfirmed label, not under the confirmed and disputed counts"
  status: failed
  reason: "User reported: the new reporter thing should be on the top part of the box where Unconfirmed is located, because it doesn't look really nice under the confirmed and disputed one. Note for the fix plan: the trust block is shared by the feed row, the map popup and the Activity page (D-20), and the top line is narrow on a phone (236px feed body at 320px), so a long tag such as Unreliable reporter may need to wrap."
  severity: cosmetic
  test: 1
  artifacts: []
  missing: []
- truth: "The trust panel and block text is short enough to read at a glance"
  status: failed
  reason: "User reported: works fine but I feel like the text is too long. Which text (the first panel line, the Counts separate places, not votes tail, or other wording) is not yet specified. To be settled at test 10."
  severity: cosmetic
  test: 1
  artifacts: []
  missing: []
- truth: "After Mark resolved from the map popup, keyboard focus lands on the map container, never the page body, at default motion as well as at reduced motion (V-42 step 10, A52)"
  status: failed
  reason: "Scripted browser run 2026-10-09, not typed by the owner (Chromium and WebKit): the resolve answers 200, the popup node is gone and focus is on the page body at default motion. The same check passes under reduced motion. Root cause read from the code and consistent with the run: while Leaflet's 200 ms popup fade runs, the popup controls are still attached, so findLiveControls returns the dying controls, focusAfterVote focuses the dying button and returns before focusAfterRetract runs, and when the popup node is removed focus falls to the body."
  severity: major
  test: 5
  artifacts:
    - path: "web/static/js/votes.js"
      issue: "focusAfterVote and findLiveControls treat controls that are still attached during the popup fade as live, so the map container never takes focus"
  missing:
    - "After a resolve from the popup retracts the report, focus goes to the map container even while the fading popup node is still attached; the feed row, Cancel and Yes pending paths stay unchanged"
    - "A contract test that pins that decision without depending on the fade timing; the scripted check T5 P4 is the live proof and fails today in both engines"
- truth: "The refusal line from an earlier too far vote goes away when the too far hint clears at the 120 second bound"
  status: failed
  reason: "Scripted browser run 2026-10-09, not typed by the owner (Chromium and WebKit): after the bound the button marks and the hint are off, but the error line holding 'Too far to vote. You need to be within 1 km of this report.' from the earlier refused Confirm is still on screen."
  severity: cosmetic
  test: 4
  artifacts:
    - path: "web/static/js/votes.js"
      issue: "the error line left by a refused vote is not cleared when the too far hint clears"
  missing:
    - "Decide from 03-UI-SPEC.md which rule covers the error line (clear with the hint, or on the next action), then implement it; the scripted check T4 S1.11b fails today in both engines"
