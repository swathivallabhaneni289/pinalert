---
status: testing
phase: 03-trust-model-hardening-diversity-weighted-trust
source: [03-VERIFICATION.md]
started: 2026-10-07T20:33:33.016Z
updated: 2026-10-10T09:20:00.000Z
---

## Current Test

number: 5
name: Mark resolved confirmation and focus (step 10, plan 03-09)
expected: |
  Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body.
awaiting: one human session at the end of Phase 3 (VoiceOver, Ask in Safari, look and feel, make migrate), after the open redesign decision

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
note: Scripted browser run 2026-10-10 (Chromium and WebKit, server built from head 52d9825, own database and port, nothing in the repo touched). Chromium 10 PASS, 0 FAIL, 1 NEEDS A PERSON. WebKit 10 PASS, 0 FAIL, 1 NEEDS A PERSON. Proven in both engines, on a feed row and in the popup: the toggle is a button whose accessible name is its counters followed by the hint "Shows what these mean." (read from the browser accessibility tree, aria-expanded false), and the panel opens by click, tap, Enter and Space and closes by Escape with focus on the toggle (T2.3, all four inputs on both surfaces). Still needs a person: VoiceOver speaking the toggle name, then the hint, then collapsed or expanded (T2.2).
result: [pending]

### 3. Escape by surface ownership (step 7, plan 03-19)
detail: Escape by surface ownership (step 7, plan 03-19). Walk each focus position with a popup panel and a row panel open.
expected: First Escape closes the panel and focuses the toggle, the second closes the popup. Popup panel closes before a row panel when focus is in the popup.
why_human: Focus order across two surfaces is runtime browser behavior.
note: Scripted browser run 2026-10-10 (same run as test 2). Chromium 9 PASS, 0 FAIL, 0 NEEDS A PERSON. WebKit 9 PASS, 0 FAIL, 0 NEEDS A PERSON. Proven in both engines: with the popup panel open, focus on Confirm, on the popup card, on the popup close button or on the pin each give the same result, where the first Escape closes the panel and focuses its toggle and the second closes the popup (T3.1). With a row panel open, focus on Confirm or on the row itself, Escape closes the row panel and focuses its toggle (T3.2). With a popup panel and a row panel open at once and focus in the popup, Escape closes the popup panel first, the next Escape closes the popup and leaves the row panel open, and with focus back on the row Escape closes the row panel (T3.3 to T3.5). No human only part is left in this test beyond the owner accepting the scripted evidence.
result: [pending]

### 4. Too far hint and refusal (steps 8 and 9, plans 03-04 and 03-18)
detail: Too far hint and refusal (steps 8 and 9, plans 03-04 and 03-18). Vote from 2 km, check the buttons, force a POST from the console, test the 0.9896 km and 1.0008 km boundary pair, wait out the 120 second bound, test silent refresh, Ask and Block location states.
expected: Refusal text 'Too far to vote. You need to be within 1 km of this report.' with HTTP 403 field location. Buttons show off within one poll, stay Tab reachable, send no vote on click, Enter or Space, and a tap first refreshes the cached fix silently (WR-03), so the vote goes ahead only if the fresh fix is within the radius. 12.9805 accepted, 12.9806 refused. Hint clears after the bound. Own Mark resolved stays enabled from far. Blocked location shows the locked sentence and sends no POST.
why_human: Needs Chrome Sensors, permission prompts and the Network panel. Server refusal is proven by TestCastVoteTooFar, but the browser hint and cache are not.
note: Scripted browser run 2026-10-09 (Chromium and WebKit, server built from 99d246d, own database and port, nothing in the repo touched). Chromium 23 PASS, 1 FAIL, 1 NEEDS A PERSON. WebKit 22 PASS, 1 FAIL, 2 NEEDS A PERSON. Proven in both engines: the buttons go off within one poll (aria-disabled) and stay reachable by Tab, click, Enter and Space send nothing, a forced POST gets 403 with field location and the exact sentence, the boundary pair (12.9806 refused, 12.9805 accepted), the 120 second bound (on at 30, 60 and 90 seconds, off at 120, on a fake clock), the silent refresh, the Block state (no POST, locked sentence) and own Mark resolved enabled from far. Chromium also proves the Ask state sends no location request. One cosmetic FAIL before the fix, see Gaps. Re-run on 2026-10-09 after the fix (plan 03-25, merged in 584cef3) on the merged code: Chromium 24 PASS, 0 FAIL, 1 NEEDS A PERSON and WebKit 23 PASS, 0 FAIL, 2 NEEDS A PERSON. S1.11b now passes in both engines (the refusal line is not visible after the 120 second bound, on a fake clock). Still needs a person: Ask in Safari (no permission prompt pops up) and the prompt UI itself. Step 8(f) in 03-20-SUMMARY.md was corrected by plan 03-25, since a tap on a too far button does refresh the fix silently (WR-03).
result: [pending]

### 5. Mark resolved confirmation and focus (step 10, plan 03-09)
detail: Mark resolved confirmation and focus (step 10, plan 03-09). Keyboard only on a feed row, then as reporter in list view at phone width and from the map popup at desktop width.
expected: Focus lands on Cancel, the group name is announced, focus returns to Mark resolved, and after removal focus goes to the replacement or previous row or the view toggle, never the body.
why_human: Focus management and screen reader announcement.
note: Scripted browser run 2026-10-09 (Chromium and WebKit). Chromium 34 PASS, 1 FAIL, 2 NEEDS A PERSON. WebKit 30 PASS, 1 FAIL, 2 NEEDS A PERSON. Proven in both engines at desktop and phone width: focus lands on Cancel, the group has its accessible name in the accessibility tree, Cancel returns focus to Mark resolved, Escape works, Yes with a pending resolve keeps focus on that row's Mark resolved, and removal from the list moves focus to the next row, the previous row or the view toggle. One FAIL before the fix: resolving from the map popup at default motion left focus on the page body (reduced motion passed), see Gaps. Re-run on 2026-10-09 after the fix (plan 03-25, merged in 584cef3) on the merged code: Chromium 35 PASS, 0 FAIL, 2 NEEDS A PERSON and WebKit 31 PASS, 0 FAIL, 2 NEEDS A PERSON. Check P4 now lands focus on the map container at default motion and at reduced motion in both engines. Still needs a person: VoiceOver speaking "Mark this report resolved?, group, Cancel, button". Tip for the owner: in Safari plain Tab skips buttons, so hold Option while pressing Tab, or tick the Advanced setting.
result: [pending]

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
note: Scripted browser run 2026-10-10 (same run as test 2). Chromium 10 PASS, 0 FAIL, 1 NEEDS A PERSON. WebKit 10 PASS, 0 FAIL, 1 NEEDS A PERSON. Proven in both engines at 320px with a long unbroken title and a long URL: the feed row body is 236px and the Activity row body is 188px, there is no horizontal overflow at 100 percent text with panels closed and open, and none at 200 percent text (the script doubles the font size tokens) with panels open. The severity word sits before the title in the layout and in the accessibility snapshot order, and every read only label grows with the text (font 14px to 28px) and none is clipped. Still needs a person: the same on a real phone with the system text size at 200 percent, and a screen reader speaking the severity word first (T7.4).
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
note: Scripted browser run 2026-10-10 (same run as test 2). Chromium 34 PASS, 2 FAIL, 1 NEEDS A PERSON. WebKit 33 PASS, 3 FAIL, 1 NEEDS A PERSON. Proven in both engines: at 320x568 and 360x740, at default and at reduced motion, the popup tip is centred on the pin, the card is inside the view and the close button and trust toggle are not covered, when opened, with the panel open and with the confirmation open, and also after a tap on a list row (T9.1, T9.3); there is no animated pan under reduced motion, while default motion does animate, which proves the detector (T9.2, T9.2c); at 320x256 the card scrolls inside itself and stays inside the view, and Escape closes the panel first and the popup second (T9.4, T9.5); rotating across 900px keeps the popup placed, with the tip 6.5px off centre after the rotation, which is the documented A59 trade off while focus is inside the card (T9.7); the trust panel and the Mark resolved confirmation survive a 30 second poll with focus unmoved, in the popup and in a row (T9.8); pins are keyboard buttons named with category, severity word, age and state, and Enter and Space open a dialog named by its title (T9.9, T9.10); Space on the close button closes the popup (T9.6, T9.11); focus on a later row's trust toggle survives a poll that removes an earlier row and a poll that adds a row above it (T9.12); a popup opened from its pin returns focus to the marker on Escape (T9.15). Three checks failed before the fix, see Gaps: T9.14 (both engines), T9.13 (both engines) and T9.16 (WebKit only). Re-run on 2026-10-10 after the fix (plan 03-26, merged in 2cc0158) on the merged code, tests 2 to 10 in one run: 309 PASS, 0 FAIL, 18 NEEDS A PERSON. For test 9, Chromium 36 PASS, 0 FAIL, 1 NEEDS A PERSON and WebKit 35 PASS, 0 FAIL, 1 NEEDS A PERSON, with one NOTE. T9.14 now passes in both engines (after Enter on a row at 1000px focus is on the popup card, after one Escape focus is on that row) and so does T9.13 (900 ms after a poll removes the report whose popup holds focus, focus is on the neighbouring row). T9.16 passes in Chromium (the marker takes focus) and is a NOTE in WebKit, an accepted browser difference, see Gaps. Tests 2, 3, 4, 5, 6, 7, 8 and 10 have the same numbers as before. Re-run on 2026-10-10 after plan 03-27 (merged in 745ebda) on the merged code, tests 2 to 10 in one run: 314 PASS, 0 FAIL, 19 NOTE, 18 NEEDS A PERSON. For test 9, Chromium 38 PASS, 0 FAIL, 1 NEEDS A PERSON and WebKit 38 PASS, 0 FAIL, 1 NEEDS A PERSON, each with two NOTE lines (the setup line and the documented A59 rotation trade off, T9.7b). T9.16 is strict again (the check no longer has a NOTE branch: a mouse close of a pin opened popup must leave focus on the marker, FAIL otherwise) and now passes in WebKit too: right after the mouse press focus is on div#map because the browser moved it, and after the close focus is on the marker (the pointerdown flag in map.js, see Gaps). Two guards were added to the same section and pass in both engines: T9.16b (a mouse close of a row opened popup returns focus to that row) and T9.16c (a click on the map away from the popup closes it and leaves focus off the marker). Run on the code before the fix, the strict checks fail in WebKit as expected (T9.16 and T9.16b FAIL, T9.16c PASS) and everything passes in Chromium. Still needs a person: how the popup card looks (T9.17), in light and dark, in forced colours, at 320 and 360px.
result: [pending]

### 10. Look and feel and plain language check (step 19)
detail: Look and feel and plain language check (step 19). Check the toggle press fill on a Provisional row, forced colors, both themes. Ask someone who has not seen the app what the numbers mean.
expected: The two signals read as separate things (still current versus reporter) and the person finds the panel without help.
why_human: Visual judgment and comprehension, which no automated check can give. This is the check that the goal of a reader understanding the signals is met in practice.
note: Scripted browser run 2026-10-10 (same run as test 2). Chromium 10 PASS, 0 FAIL, 2 NEEDS A PERSON. WebKit 6 PASS, 0 FAIL, 2 NEEDS A PERSON (forced colors is not run in WebKit, since macOS Safari has no forced colors mode to test). The mechanical half is proven on a Provisional row (Unconfirmed): hovering the trust toggle shows a cue, an underline, in light and dark in both engines; pressing it fills it with the border token (rgb 225, 228, 232 in light, rgb 42, 45, 49 in dark), and press differs from hover by the fill and not by line thickness (T10.1 to T10.3); under forced colors in Chromium, in light and dark, the toggle still shows a non colour cue (underline plus border) on hover and on press. Still needs a person: whether the two signals (still current, reporter) read as separate facts in both themes and in forced colors (T10.4), and the plain language check with someone who has not seen the app (T10.5).
result: [pending]

### 11. Owner housekeeping (step 20 and reminders)
detail: Owner housekeeping (step 20 and reminders). Run migrate once on the owner dev database for migration 00005 after merge. Drop p3_uat when finished.
expected: Index idx_reports_session_expires exists on the dev database.
why_human: Touches the owner's own database, which the executors and this verifier are not allowed to use.
result: [pending]

## Summary

total: 11
passed: 2
issues: 0
pending: 9
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
  status: resolved
  fix_shipped: "2026-10-09, plan 03-25, commits 53fb069 (code and TestPopupResolveFocusIgnoresAFadingPopup) merged in 584cef3. A closed map popup is no longer treated as live, so focusAfterRetract runs and focus goes to the map container. Verified three ways: the web contract tests, the full suite on a fresh scratch database (373 PASS, 0 FAIL, 0 SKIP), and the scripted browser check T5 P4 re-run on the merged code, PASS at default and reduced motion in Chromium and WebKit (focus on div#map)."
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
  status: resolved
  fix_shipped: "2026-10-09, plan 03-25, commit a001850 (code and TestTooFarRefusalClearsWithTheHint) merged in 584cef3. The refusal line clears together with the hint, but only when the hint was showing before a render, is hidden after it, and the line holds the server's too far refusal; every other error message is left alone. Verified by the web contract tests, the full suite (373 PASS, 0 SKIP) and the scripted check T4 S1.11b, PASS in Chromium and WebKit."
  reason: "Scripted browser run 2026-10-09, not typed by the owner (Chromium and WebKit): after the bound the button marks and the hint are off, but the error line holding 'Too far to vote. You need to be within 1 km of this report.' from the earlier refused Confirm is still on screen."
  severity: cosmetic
  test: 4
  artifacts:
    - path: "web/static/js/votes.js"
      issue: "the error line left by a refused vote is not cleared when the too far hint clears"
  missing:
    - "Decide from 03-UI-SPEC.md which rule covers the error line (clear with the hint, or on the next action), then implement it; the scripted check T4 S1.11b fails today in both engines"
- truth: "At 1000px and wider, pressing Enter on a feed row opens the popup with focus on the popup card, and one Escape closes it with focus back on that row (A74, V-42 step 18)"
  status: resolved
  fix_shipped: "2026-10-10, plan 03-26, commit a6d1b9a (code and TestListRowActivationOpensThePopupOnce) merged in 2cc0158. flyTo in map.js asks Leaflet to open the popup only when it is closed, so the second call from the A74 seam no longer removes and appends the card again. The opener claim after it is unchanged, so one Escape returns focus to the row. Verified three ways: the web contract tests, the full suite on a fresh scratch database (375 PASS, 0 FAIL, 0 SKIP), and the scripted check T9.14 re-run on the merged code, PASS in Chromium and WebKit (after Enter focus is on the popup card, after one Escape focus is on that row)."
  reason: "Scripted browser run 2026-10-10, not typed by the owner (Chromium and WebKit, check T9.14): Enter on the row opens the popup, but focus ends on the page body and not on the popup card, and after one Escape the popup is closed and focus is still on the body, not on the row. The focus event log is the same in both engines: call focus() on div.popup-card, out li.report-row, in div.popup-card, then out div.popup-card in Chromium (WebKit logs no blur because the node is removed). The card takes focus and loses it again with nothing to catch it, as if the popup were opened twice. Root cause, read from the code and then confirmed by a throwaway copy where the fix made T9.14 pass in both engines: the popup is opened twice. activateRow in feed.js calls Pinalert.select(id, 'list'), whose map.js subscriber calls flyTo(id) and opens the popup (popupopen focuses the card). Then activateRow calls PinalertMap.flyTo(id, { opener: 'list' }), the A74 seam, and flyTo calls marker.openPopup() a second time on a popup that is already open. Leaflet's openPopup runs update() on an open popup, which removes the card node and appends it again, so the focused card loses focus, and no second popupopen fires to give it back. With focus on the body, Escape reaches nothing and the popup stays open."
  severity: major
  test: 9
  artifacts:
    - path: "web/static/js/map.js"
      issue: "flyTo calls marker.openPopup() even when the popup is already open, and Leaflet re appends the card, dropping its focus"
  missing:
    - "A row activation opens the popup once: flyTo does not call openPopup on a popup that is already open, and the list still claims the opener (A74), so focus ends on the card and one Escape returns it to the row"
    - "A contract test that pins the guard in flyTo; the scripted check T9.14 is the live proof and fails today in both engines"
- truth: "When a poll removes the report whose popup holds keyboard focus, focus does not fall to the page body"
  status: resolved
  fix_shipped: "2026-10-10, plan 03-26, commit 9531778 (code and TestFocusAfterRemovalIgnoresAFadingPopup) merged in 2cc0158. focusIsLost in feed.js treats focus on the page body, or inside a popup that map.js has marked closed (data-popup-open not true), as lost, so focusAfterRemoval runs its existing order (neighbouring row, then the map, then the view toggle) even while Leaflet's 200 ms fade keeps the node attached. No timer, animation event or style read is involved. Verified three ways: the web contract tests, the full suite on a fresh scratch database (375 PASS, 0 FAIL, 0 SKIP), and the scripted check T9.13 re-run in real time on the merged code, PASS in Chromium and WebKit (900 ms after the poll, focus is on the neighbouring row)."
  reason: "Scripted browser run 2026-10-10, not typed by the owner (Chromium and WebKit, check T9.13): focus was on the popup card, the next poll removed that report (the report is gone), and 900 ms later focus is on the page body. The focus event log ends with the card losing focus (out div.popup-card in Chromium) and nothing taking it. Root cause, read from the code and then confirmed by a throwaway copy where the fix made T9.13 pass in both engines (focus 900 ms later on the neighbouring row): the same class as the Mark resolved defect fixed by plan 03-25. The poll's render removes the marker, so the popup closes and map.js clears data-popup-open at once, but Leaflet keeps the closing popup node attached for a 200 ms fade. focusAfterRemoval in feed.js returns early because document.activeElement is the fading card, which is not the body and is still attached. When Leaflet removes the node 200 ms later, focus falls to the body and nothing catches it. A frozen clock hides the fade, which is why T9.12 and the earlier tests pass."
  severity: minor
  test: 9
  artifacts:
    - path: "web/static/js/feed.js"
      issue: "focusAfterRemoval treats focus on the fading popup card as live, so the row, map and view toggle fallbacks never run"
  missing:
    - "A poll that removes the report whose popup holds focus moves focus to the next or previous row, else the map, even while the fading popup node is still attached; focus on a live element is still left alone"
    - "A contract test that pins that decision without depending on the fade timing; the scripted check T9.13 (real time) is the live proof and fails today in both engines"
- truth: "Closing a pin opened popup with the mouse on its close button returns focus to the marker (A74)"
  status: resolved
  fix_shipped: "2026-10-10, plan 03-27, commit 50092af (code and TestPopupClosePressKeepsTheFocusAnswer) merged in 745ebda. map.js records the focus inside answer on the pointerdown of the popup close button (the press comes before the browser moves focus), and onPopupClose uses that answer and counts the map container as lost focus only after such a press, so the existing return runs and the marker, or the row that opened the popup, takes focus. Every other press clears the answer, so a click away from the popup does not move focus to the marker. The scripted check T9.16 is strict again (the NOTE branch added after the first acceptance is removed). Verified three ways: the web contract tests, the full suite on a fresh scratch database (380 PASS, 0 FAIL, 0 SKIP), and the scripted checks re-run on the merged code, T9.16 PASS in Chromium and WebKit (in WebKit focus is on div#map right after the press and on the marker after the close), with the new guards T9.16b (row opened popup, mouse close, focus back on the row) and T9.16c (click on the map away from the popup leaves focus off the marker) PASS in both engines. Run before the fix, the same strict checks failed in WebKit (T9.16 and T9.16b)."
  decision: "2026-10-10: first accepted as a browser difference with no code change, then reversed the same day when the owner asked for everything to be fixed before the human checks. A74 says the return applies to Escape and the close button alike, and the popup surface text records the flag with focusin and focusout listeners 'or the close click', so a mouse close has to return focus in every engine. Analysis kept for the record: WebKit does not focus an anchor on a mouse press, so focus moves from the card to the nearest focusable ancestor, the map container (events out div.popup-card, in div#map), and map.js onMapFocusIn cleared the flag before the click closed the popup. Chromium focuses the anchor, so the flag stayed set. One accepted edge: a press on the close button that is followed by a close through another route with no new press (Escape after the pointer left the button) still returns focus to the marker, which is harmless."
  reason: "Scripted browser run 2026-10-10, not typed by the owner (WebKit only, check T9.16): the popup closes, and focus is on the map container (div#map) and not on the marker. Chromium passes the same check, and Escape from the pin passes in both engines (T9.15). An earlier agent changed that check from FAIL to a NOTE for the WebKit case, which was a change to the check and not to the app; plan 03-27 restored the strict expectation."
  severity: cosmetic
  test: 9
  artifacts:
    - path: "web/static/js/map.js"
      issue: "onMapFocusIn cleared popupFocusInside when WebKit moved focus to the map container on the mouse press, so onPopupClose saw no focus and returned none"
  missing:
    - "A press on the popup close button made while focus is inside the popup returns focus to the opener on close in WebKit too, and a click away from the popup does not"
    - "A contract test that pins the pointerdown flag; the scripted check T9.16 (strict) is the live proof"

Security residuals closed by the same plan, 2026-10-10 (plan 03-27; not UAT gaps, no pending count changes): UF-03-01 an unknown category shows Other and never raw text (commit 0c65f0d), UF-03-03 NaN lat, lon and radius_km are a 400 field error instead of an empty feed (commit 3e8ba86), IN-01 a vote without coordinates is a 400 unless it is the reporter's own resolve or reopen (commit d3bf109; trailing data after the JSON body is still accepted, so IN-01 is only partly closed), and the three files that gofmt listed are formatted (commit f5289b2). Details are in 03-SECURITY.md, section Residuals fixed in 03-27.
