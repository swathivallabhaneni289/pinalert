---
status: complete
phase: 02-trust-mechanic-core-confirm-dispute-visibility
source: [02-VERIFICATION.md]
started: 2026-09-16T14:42:29Z
updated: 2026-10-07T15:48:21Z
---

<!-- Gap-closure round 2 (plans 02-11..02-14, wave 9) shipped 2026-09-23. Full commit range:
     bab6292..04cdeae. All code fixes pass automated tests against a disposable database
     (never pinalert_test) and were merged cleanly. This does NOT mean every gap is closed:
     four of the five fixes address things a passing test cannot fully confirm (the user
     perceiving a color change, seeing a map repaint, or a UI symptom whose live cause was
     never confirmed). Only the disputed-filter gap's underlying defect is unambiguously
     fixed by code; its own UAT test still needs a live retest for the parts that only
     became observable once the fix landed. See each gap entry below for the honest
     per-gap status; a shipped fix is not a resolved gap until a live retest passes.
     2026-10-06: live retest session opened. Frontmatter status moved from
     fixes_shipped_pending_retest to testing. Tests 5, 6, 9 and 10 were reopened as [pending]
     (their earlier answers are kept under previous_* keys), test 12 (light mode walkthrough)
     was added, and tests 2 and 3 stay as issue until the tests holding their halves pass.
     2026-10-06 later the same day: all twelve tests passed (tests 2 and 3 by reference) and the
     file was closed with status complete. See the Findings section for what was raised. -->

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. D-18 GPS-denial hard block, live in a real browser
expected: Denying the browser's location prompt on Confirm/Dispute greys the buttons out and
  immediately re-enables them, shows the exact GPS-denial sentence in the row's .vote-error
  element, and sends NO network request to /api/reports/{id}/confirm|dispute (verified in DevTools
  Network panel — a request sent and then rejected is a failure even if the visible outcome looks
  similar). A granted prompt is cached in sessionStorage and not re-requested for a second vote in
  the same tab, but is re-requested in a new tab. Both the feed row and the map popup behave
  identically, and no state changes before the server responds (D-04).
result: pass
notes: Exact GPS_DENIED_MESSAGE copy confirmed verbatim in the row's .vote-error element on a real
  deny (root cause of the initial no-prompt confusion was macOS System Settings > Privacy &
  Security > Location Services being off entirely — not a Pinalert or Safari site-permission bug;
  user re-enabled it). Buttons observed functioning (not stuck disabled) after the denial. NOT
  independently re-confirmed live after the Location Services fix: DevTools Network-tab
  no-request check, sessionStorage same-tab-no-reprompt/new-tab-reprompt caching behavior, and map
  popup parity for the deny path specifically. No defect found in anything actually observed; these
  remain untested rather than failed.

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
result: pass
previous_result: issue
previous_reported: "Hidden outline treatment confirmed correct on both the feed row and map pin (transparent
  badge, muted glyph, 'Disputed' label matching VISIBILITY_TAG_LABELS.hidden exactly). Toggle
  reveal/hide confirmed on both surfaces together. Critical-severity bypass confirmed (no chip, no
  treatment, always live, matches D-06). Checkbox correctly resets to unchecked on a real reload,
  matching the original spec — but the user wants this changed so reloading keeps the current
  filtered view instead. The user also wants a manual light/dark toggle added so light mode can be
  verified/used without touching OS settings. Two items were never visually confirmed either way:
  the Provisional dimmed+'Unconfirmed'-chip treatment (only checked via API response, not a
  screenshot) and the 'No disputed reports nearby' empty-state copy. Light-mode rendering of any of
  this was never checked at all (every screenshot this session was dark mode)."
previous_severity: minor
retest_result: "2026-10-06, closed by reference. Its halves were retested live: provisional dimming and the Unconfirmed chip in dark mode (test 5, user: 'the colors thing is fine') and in light mode (test 12, user: 'they're all visible'); the Show disputed filter, reload and no flash (test 9, user: 'it works', plus the server log); the theme control (test 10, user: 'It works fine'); the empty state wording in light mode (test 12 step 3). The Hidden outline look was confirmed in dark mode on 2026-09-18 and the critical bypass on 2026-09-18. In this round a genuinely disputed report was created (report 4, two other accounts voting from two places, resolver answer hidden) and the server log shows the user ticked Show disputed reports at 15:05:08 and the server returned that report (417 bytes). The user reported no problem but did not describe the look in words, so the Disputed outline in light mode rests on that and on the contract tests that resolve its colours in all four theme sources. The two change requests inside this test (keep the filter across a reload, add a manual light and dark control) are shipped and passed in tests 9 and 10."

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
result: pass
previous_result: issue
previous_reported: "Non-reporter affirm correctly showed the 'awaiting agreement' toast and the report
  stayed in the feed. Reporter's own report resolved instantly as expected. Activity page correctly
  lists the resolved report. But: (1) on both the feed row and the map popup, the original
  Confirm/Dispute/Mark-resolved buttons stay visibly rendered right next to the new inline Yes/
  Cancel confirmation instead of being replaced by it — user described the result as buttons that
  'look too big'/doubled up. (2) In the map popup specifically, the 'Mark this report resolved?'
  confirmation heading text renders barely legible/washed out. (3) After tapping Reopen on the
  Activity page and navigating back to the main feed, the reopened report did not show as live
  again until a manual page reload — contradicts the 'no page reload' requirement. GPS-deny-on-
  affirm was not independently live-tested but verified by code inspection to share the exact same
  castVote/getVoterLocation code path already confirmed in Test 1, so treated as covered by
  construction rather than untested. User also flagged the Activity page has no way to navigate
  back to the main map other than the browser's own back button."
previous_severity: major
retest_result: "2026-10-06, closed by reference. Sub-issue 1 (the original buttons stayed beside the Yes and Cancel box): test 6, user: 'it's fine', and the screenshot of the popup shows the three buttons replaced. Sub-issue 2 (the popup heading barely legible): test 7 passed on 2026-09-21, and the heading is clearly legible in the 2026-10-06 light mode screenshot. Sub-issue 3 (stale feed after Reopen then Back): test 8 passed in Safari on 2026-09-21 (the Chromium half was never run). The Activity page back link: test 11 passed. The non reporter affirm toast, the reporter resolve and the Activity Reopen were confirmed on 2026-09-18, and the reporter resolve was seen again on 2026-10-06 (POST /api/reports/2/resolve returned 200 and the report left the list)."

### 4. D-18 GPS-denial hard block — DevTools/sessionStorage/map-popup re-confirmation
expected: Denying the location prompt sends zero network requests (verified in the Network panel,
  not just by visible outcome) and shows the exact GPS-denial copy; a granted prompt is cached per
  session (not re-prompted same-tab, re-prompted new-tab); the map popup behaves identically to the
  feed row.
result: pass
notes: Verified via freshly-run structural proof rather than live browser observation (Web
  Inspector unavailable to the user). TestVoteTransportHasNoLocationFallback proves castVote's
  fetch() call is structurally unreachable unless the location promise resolves (reject() on
  deny, no fallback-to-default-center code exists) — ran now, PASS. TestVoterLocationIsCachedPerSession
  proves sessionStorage is checked before the prompt is raised and localStorage is never used
  (session-scoped, not cross-tab) — ran now, PASS. Exact GPS_DENIED_MESSAGE copy already visually
  confirmed live in this same UAT's Test 1. Map-popup parity not independently re-clicked this
  round but shares the identical unmodified code path (map.js unchanged since 02-06).

### 5. Provisional dimming/"Unconfirmed" chip and the "No disputed reports nearby" empty state
expected: |
  Setup first (it also covers the next few checks). Start your dev server the way you normally do.
  It is not running right now, and it must be started fresh to serve the latest code. Log in using
  Safari, then add two reports near you: one with severity "1 · Low" and one with severity
  "3 · Critical".

  Now look at them in dark mode, on the list and on the map. If the app opens in light mode, tap the
  round sun/moon button at the bottom right once.
  1. The Low report's row has an "Unconfirmed" chip, and its badge and border look washed out and
     greyish, clearly less colourful than a normal report.
  2. The Critical report has no chip and keeps its full strong colour, because critical reports go
     live straight away. Compare the two side by side. The colours differ, but the Low one should
     look clearly drained next to the Critical one.
  3. On the map, the Low report's pin badge looks faded in the same way.
  4. Tap the Low report's pin. The popup shows the "Unconfirmed" chip, and its box is a soft neutral
     grey, not green.
previous_expected: A fresh non-critical report shows both a desaturated badge/border and an explicit
  "Unconfirmed" chip, on both the feed row and the map pin popup. An empty disputed-view result
  shows the specific "No disputed reports nearby" copy, not a blank list.
result: pass
previous_result: issue
retest: "2026-10-06, after plan 02-13 (20/80 mix toward stale, neutral row wash) and quick task 260923-mb0 (popup tint). The empty state half already passed live on 2026-09-21 and is not retested. Dark mode here, light mode is covered by test 12."
retest_result: "2026-10-06 about 14:27 IST, dark mode, scratch server on port 8792. User, verbatim: 'other than that, I think the colors thing is fine.' The word 'that' was two unrelated findings, recorded as F1 and F2 under Findings outside this phase. The user posted a Low and a Critical report, looked at the list, the pins and the popup, and raised no colour complaint. Not asked about one by one: the pin badge on the map and the grey popup box. Light mode is still covered by test 12."
previous_reported: "I don't see the faded look but I definitely see the unconfirmed chip to it" (screenshot
  2026-09-21, dark mode, a fresh Low-severity Flood report, feed row and map popup). THE USER DOES
  NOT SEE THE FADING/DESATURATION. The required desaturated badge/border is not perceivable, so
  half of the Provisional treatment (D-09) FAILS on both surfaces, even though the Unconfirmed chip
  itself is visible and correct. Separately confirmed live in the same session: the 'No disputed
  reports nearby' empty state renders with the Show disputed reports box ticked and no live reports
  (that half of this test passes)."
previous_severity: major
context: Carried forward from this same UAT's Test 2 — never visually confirmed either way (only
  checked via API response, not a screenshot). Unresolved by any gap-closure plan (none touch
  visibility.js or the badge/chip rendering path).
notes: The user's observation is the finding: the fading is NOT visible to them. The chip half
  passes on both surfaces; the fading half fails on both. Supporting evidence from pixel-sampling
  the screenshot (context for the fix, not a reason to discount the report): row badge measured
  #58947c and map pin #4b7d6a, versus the undimmed dark-mode Low colour --color-severity-low
  #34D399, so .vis-provisional appears to be applied (trust.css line 128 mixes --severity-base
  50/50 with --color-age-stale #4B4F55, predicting about #409177) but the effect is too weak to
  perceive. Hypothesis, not confirmed: a Low-severity green blended toward dark grey still reads as
  an ordinary green with no live report beside it. Whether it is also too weak in light mode is
  unchecked (Test 10).

### 6. 02-08 gap closure: Mark Resolved button row is replaced, not doubled up
expected: |
  There is already a report from the second account in your list: the Medium "Road blocked" a
  little north east of your pin. Use that one. You are still logged in as the first account.
  1. On the list, find that report and tap "Mark resolved". The Confirm, Dispute and Mark resolved
     buttons should all disappear and be replaced by one question box ("Mark this report
     resolved?") with "Yes, mark resolved" and "Cancel". No leftover tall light rectangle should
     remain beside it.
  2. Tap Cancel (not Yes). The three buttons come back.
  3. Repeat on the same report's map pin popup. Do this within about half a minute, because the map
     refreshes itself every 30 seconds and may reset an open question. If it resets on its own, tell
     me, but that is a separate thing from this check.
  4. Do steps 1 and 2 once in light mode and once in dark mode.
  Optional extra: I made one more test report ("Power outage") that two other accounts disputed.
  Tick "Show disputed reports" to see it, with the outlined "Disputed" look, in light mode and in
  dark mode. Untick the box afterwards.
previous_expected: In a real browser (both themes), tap Mark resolved on someone else's report on the feed
  row and on a map pin popup. The three primary buttons visually disappear the instant the
  confirmation appears, rather than rendering beside it.
result: pass
previous_result: issue
retest_result: "2026-10-06 about 15:28 IST, scratch server on port 8792, Safari. User, verbatim, with a screenshot of the map popup in light mode: 'it's fine but I just don't like the way it's looking right now'. The screenshot shows the popup after tapping Mark resolved: the Confirm, Dispute and Mark resolved buttons are gone and only the question box with Yes and Cancel is shown, with no leftover tall rectangle. The server log shows no POST to /api/reports/3/resolve, so the user cancelled or left it open, as asked. The user did not describe the list row or dark mode separately, so 'it's fine' is the whole answer. The look complaint (grey popup surface, a white card inside it, boxes inside boxes) is a design finding, F5, not a failure of this test. The original live cause of this symptom was never identified, so this passes without proof of cause: the 02-13 guard is in place and the symptom did not recur on a freshly started server running today's code."
retest: "2026-10-06, after plan 02-13's defensive hidden-attribute guard on .vote-btn--resolve. The live cause was never identified. If it still fails, the decisive step is a live Safari Web Inspector (Elements and Computed) check at the moment the stray rectangle shows, or repeating in Chrome."
previous_reported: "Confirm/Dispute now correctly disappear (that part of the fix works). But the 'Mark
  resolved' button itself (.vote-btn--resolve) is STILL visibly rendered next to the Yes/Cancel
  confirmation box instead of hiding — it stretches to match the taller confirmation box's height
  (flex align-items: stretch), rendering as an odd tall white/light rectangle with black text that
  looks visually broken against the dark theme. Both openResolveConfirm/closeResolveConfirm's JS
  selectors and the CSS both target .vote-btn--resolve identically to .vote-btn--confirm/--dispute
  (which now hide correctly), so the cause of this one button behaving differently is not yet
  understood — needs live DevTools inspection next session."
previous_severity: major

### 7. 02-08 gap closure: map popup "Mark this report resolved?" heading legibility
expected: Open a map pin popup in dark mode and tap Mark resolved. The heading text is clearly
  legible against the popup's own background, at contrast comparable to the buttons beside it.
  Repeat in light mode and confirm nothing regressed (the fix's own falsifiable prediction is that
  the pre-fix bug never reproduced in light mode).
result: pass
notes: User confirmed live — legible now (this was checked against the freshly-restarted server
  serving today's code; the earlier "white box" popup surface issue was a stale-server artifact,
  not this bug).

### 8. 02-09 gap closure: Safari Back-after-Reopen refetch
expected: Resolve a report, open Activity, tap Reopen, press the browser's Back button (Safari
  first, then one Chromium browser). The feed shows the reopened report live immediately with no
  manual reload, and the Network panel shows a fresh GET /api/reports fired on the restore.
result: pass
notes: User confirmed live in Safari on 2026-09-21 ("yes it did"): after Reopen on the Activity page,
  pressing Safari's Back arrow showed the reopened report live in the feed with no manual reload.
  Limits, stated honestly: the Chromium half (Chrome or Edge) was not run, and the Network panel
  check for a fresh GET /api/reports was not observed (Web Inspector unavailable to the user), so
  the pass rests on the visible outcome in Safari only.
context: The pageshow/event.persisted fix is proven wired and correctly ordered by static
  inspection only. The pre-fix Safari bfcache symptom this fix targets was never independently
  reproduced live in the original UAT session — the diagnosis is the most likely explanation, not a
  confirmed one.

### 9. 02-09 gap closure: disputed-filter reload persistence, no unfiltered flash
expected: |
  Nobody has disputed any report on screen. There are three now: your Low and Critical ones, plus
  a test one from the second account.
  1. Tick "Show disputed reports". The list should switch to the "No disputed reports nearby"
     message, and every pin should disappear from the map too.
  2. With the box still ticked, reload the page. The box should still be ticked, and you should see
     the disputed view straight away, with no quick flash of your normal list first.
  3. Copy the page address (it now ends with show_disputed=true) and open it in a new tab. That tab
     should open straight into the disputed view.
  4. Untick the box. All three reports come back on the list and the map.
  Optional: if you already have a report that other people disputed, tick the box again. It should
  be the only report shown, with the outlined "Disputed" look.
previous_expected: Check "Show disputed reports", reload the page. The box is still checked and the disputed
  view shows from the very first paint, with no visible flash of the default feed first. Copy the
  URL with the parameter set, open in a new tab, confirm it loads the disputed view directly.
result: pass
previous_result: issue
retest_result: "2026-10-06 about 14:50 IST, scratch server on port 8792. User, verbatim: 'Yeah, it works, but every time I refresh the page, it's asking me if I want to allow for the location.' The location prompt is a separate finding, F4. Server log corroboration: the tick at 14:41:38 returned an empty list (15 bytes, so live and unconfirmed reports no longer appear under the filter, which is the 02-12 fix), the reload at 14:41:40 requested /?show_disputed=true, and the first report fetch after it (14:41:45) already carried show_disputed=true with no plain fetch before it, so no flash of the normal list. Not seen in the log by 14:49: opening the address in a new tab, and unticking the box. Both use the same load path as the reload. Positive direction checked from the server at about 15:08 IST: with report 4 disputed by two other accounts from two different places (resolver answer: hidden, reason disputed), the normal list returned reports 1 and 3 only and the disputed view returned report 4 alone."
retest: "2026-10-06, after plan 02-12 made the disputed filter a true partition (live and unconfirmed reports no longer show under it). This is the first time the reload, no flash and new tab halves can be judged. The optional last step needs a report disputed from other accounts in other places, so it is not required."
previous_reported: "still does that" (2026-09-21), and on the follow-up question the user picked "Report shows
  under disputed": a report nobody disputed still appears when Show disputed reports is ticked. The
  user considers their answers complete.
previous_severity: minor
notes: Same finding as the exclusive-filter gap below (backlog 999.2, approved for fixing). The
  reload-persistence, no-flash and new-tab halves of this test were NOT separately observed. Retest
  them after the exclusive-filter fix, when they become easy to judge: with a live report present
  the disputed view will be empty, so any flash of the default feed shows the report row briefly.
context: The "restored before the first fetch" guarantee rests on an argument about script
  execution order that 02-09-SUMMARY itself flags as an argument, not a proof, and names what would
  invalidate it. Static tests confirm the code shape; they cannot observe whether a real browser
  ever paints an unfiltered frame first.

### 10. 02-10 gap closure: theme toggle — no-flash, native controls, post-logout, full light-mode walkthrough
expected: |
  The theme control is now a round button with only an icon (no words). It sits at the bottom right
  of the map page, just under the round report button, and it is also on the Activity page.
  1. Tap it. The whole app should switch between dark and light straight away. The map itself looks
     the same in both (that was your decision).
  2. Choose Dark and reload the page: it should come back dark with no flash of light first. Choose
     Light and reload: it should come back light with no flash of dark first.
  3. In either theme, the "Show disputed reports" tick box and the page scrollbar should follow the
     theme (a dark box and a dark scrollbar in dark mode).
  4. The report button and the theme button sit one above the other without touching each other or
     any other control, and the sun or moon icon is centred in its circle.
  5. Last step: choose Light, then log out (account button, top right). The login screen should also
     be light, not snap back to dark. Do not type an email there. Tell me when you see it and I will
     log you back in.
previous_expected: Set the theme to Dark, reload — no flash of light first. Set Light, log out — the login
  gate renders light rather than snapping to a dark OS default. Toggle Dark — native checkboxes and
  the scrollbar also go dark. Walk all of Phase 2's UI once in light mode (never done — every UAT
  screenshot this phase was dark) and report anything illegible.
result: pass
previous_result: issue
retest_result: "2026-10-06 about 14:58 IST, scratch server on port 8792. User, verbatim: 'It works fine.' This covers what the user looked at: the button, the theme surviving a reload, the tick box and scrollbar, and the button layout. Those are visual, so the server log only shows the reload at 14:57:01. Step 5 (log out and look at the login screen) was NOT exercised: the server log shows no logout by 14:57:26, so it was moved into test 12 as an optional last step. It is also covered by TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage and was confirmed live on the old three way control on 2026-09-21 and 22 (same head script, same storage key)."
retest: "2026-10-06, after quick tasks 260923-qwi (two state icon toggle, no System mode, no text label) and 260923-rra (moved out of the account menu into a floating button under the report button). The old three way control no longer exists, so the expected wording was rewritten. The full light mode walkthrough moved to test 12."
previous_reported: "for the light theme thing, even if I log out or reload, if I change the setting from dark
  mode to light mode, it changes accordingly, it works fine" (2026-09-21/22). Two things raised
  alongside that pass: (1) the 'Theme: Dark' text label 'looks really wordy', the user does not want
  a words-based toggle and asked for research into how other apps present this control. (2) 'even if
  i change it to the dark mode the map doesn't change much and same goes to when i change it to the
  light mode', the map basemap looks the same regardless of theme.
previous_severity: minor
notes: The core requirement (persists across reload and logout, no snap to a dark OS default, no
  reported flash) passes on the user's own account. The two follow-on items are recorded as gaps
  below rather than failing this test outright, since the user described the toggle itself as
  working. Native checkbox/scrollbar dark styling was not explicitly confirmed or denied; left open,
  not re-asked given the length of this session.
context: Presence of a non-deferred head script and a color-scheme CSS declaration is provable by
  static inspection (done, both pass); a real paint-flash timing effect and real rendered legibility
  across a theme never once visually inspected in this project cannot be. 02-10-SUMMARY states
  explicitly this walkthrough has not been performed.

### 11. 02-10 gap closure: Activity "Back to map" link, live click-through
expected: From the Activity page, click "Back to map" and confirm it lands on the main feed/map
  view.
result: pass
notes: User confirmed live (2026-09-21): the Back to map link works and returns to the map/feed,
  including after reopening a report. Visual complaint recorded separately in the polish gap: the
  link's placement and its plain underlined-text styling look odd to the user (screenshot shows it
  as underlined white text floating top-left above the email, not styled like the app's buttons).
  Not a functional failure, so this test passes.
context: Structurally verified (link present, contract test passes) but not separately re-run live
  by the phase verifier.

### 12. Light mode walkthrough (never done in this project)
expected: |
  Light mode has never been checked screen by screen. First untick "Show disputed reports" if it is
  still ticked (it may be), so your three reports show again. Then make sure you are in light mode
  (tap the round sun/moon button if you are not) and go through these, telling me if anything is
  hard to read, too faint, or looks broken:
  1. The map page: your Low report should still look clearly faded with a readable "Unconfirmed"
     chip, and your Critical report should look strong. Check the list and the map pins.
  2. Tap a pin. The popup box, its close button, and its Confirm, Dispute and Mark resolved buttons
     are all readable. (The second account's report has all three buttons. Yours only has Mark
     resolved.)
  3. Tick "Show disputed reports". The "No disputed reports nearby" message reads clearly. Untick it.
  4. On one of your own reports, tap "Mark resolved". The question and its "Yes, mark resolved" and
     "Cancel" buttons are readable. Tap Cancel.
  5. Open Activity from the account menu. The page, its "Back to map" arrow, and your report rows
     are readable.
  6. Optional: drag the browser window narrow, like a phone. Nothing should overlap or scroll
     sideways.
  7. Optional last step: log out. The login screen should be light too. Do not type an email there.
     Tell me when you see it and I will log you back in. Switch back to the theme you prefer after
     that.
result: pass
retest_result: "2026-10-06 about 15:05 IST, light mode, scratch server on port 8792. User, verbatim: 'Yeah, they're all visible.' The server log shows what was done: at 15:02:59 the user tapped Mark resolved and then Yes, mark resolved on their own Critical report (id 2) instead of Cancel. POST /api/reports/2/resolve returned 200 and the report left the feed, which is the reporter instant resolve behaviour working. The user then opened Activity at 15:03:07. No logout by 15:04, so optional step 7 (login screen in light mode) and optional step 6 (narrow window) were not done. Not seen live in light mode: the outlined Disputed look, because no report was disputed during this test. A disputed report was created afterwards for test 6's optional extra."
context: "Added 2026-10-06 from the 02-10 plan's own human check step 6 and the Test 10 and Test 2 leftovers: every UAT screenshot this phase was dark mode, so light mode rendering of the Provisional dimming, the Disputed outline, the resolve confirmation, the popup and the Activity page was never inspected. Also covers the light mode half of test 5."

## Summary

total: 12
passed: 12
issues: 0
pending: 0
skipped: 0
blocked: 0

retest_session_2026_10_06: |
  Live retest of the round 2 fixes plus the post round 2 quick tasks (260923-mb0 popup tint,
  260923-qwi and 260923-rra icon theme toggle). Presentation order: 5, 9, 10, 12, then 6 last,
  because 6 needs a report from a second account. Tests 2 and 3 are composites that close by
  reference only when the tests holding their halves pass (2 needs 5, 9, 10 and 12; 3 needs 6, as
  7, 8 and 11 already passed). Counts above: issues = tests 2 and 3, pending = none
  (tests 5, 6, 9, 10 and 12 passed on 2026-10-06, test 5 in dark mode, test 12 in light mode).
  Tests 2 and 3 were closed by reference on 2026-10-06 once the tests holding their halves passed,
  with the honest caveats written in each test's retest_result. Five findings (F1 to F5) were
  raised during the session. None of them is a Phase 2 test failure, so none blocks closing this
  file, but F1, F2 and F5 are owner priorities and are routed after this phase closes.

gap_closure_round_2: |
  2026-09-23, plans 02-11..02-14 (wave 9), commit range bab6292..04cdeae. All 4 plans merged
  clean, full test suite green against a disposable database. Honest per-gap status:
  1 of 6 gaps: status resolved (disputed-filter-not-exclusive, code-level defect, code fix).
  4 of 6 gaps: status fix_shipped_pending_retest or defensive_fix_shipped_cause_unconfirmed
  (map-theme, provisional-dimming, resolve-button, and the mechanical half of ui-visual-polish)
  — code shipped and passes automated tests, but each needs a live look before calling it done.
  1 of 6 gaps: ui-visual-polish's larger scope (motion, tokens, surface elevation, theme-toggle
  icon) deliberately deferred to a future /gsd-ui-phase design pass, not touched here.
  Dev server must be restarted before any retest (CSS/JS are go:embed'd at build time).

## Findings outside this phase

<!-- Raised by the owner during the 2026-10-06 retest. They belong to other phases, so they are
     kept out of the Tests list (a failing test there would block closing Phase 2) and out of the
     Gaps list (which feeds the Phase 2 gap planner). The route for each is decided after the
     Phase 2 checks. -->

- id: F1
  title: "Tapping the report form's map does not move the pin when location is on"
  raised: "2026-10-06 during test 5. User: 'the pin wont move the pin I click somewhere on the map, it's not doing that, but I have to drag it around to like actually place it correctly from where it is originally located.'"
  belongs_to: "Phase 1 report form (design decision D-02), reused by Phase 7"
  cause: "CONFIRMED by reading web/static/js/modal.js. modalMap.on('click') is registered only inside showLocationDenied(), the tap to place FALLBACK for when location is off. When geolocation succeeds, initLocation() places one draggable marker and nothing handles a tap. This matches D-02 as written (draggable marker, tap to place only as a fallback) but not what the owner expects."
  fix_sketch: "Register the click handler once, when the map is created, so a tap always moves the pin in every case. Keep dragging. Update the notice wording. Add a contract test that the click handler is not tied to the denied branch."
  size: "small, one quick task"

- id: F2
  title: "Address search finds nothing for longer addresses and can return places far away"
  raised: "2026-10-06 during test 5. User: 'when I type in my address or like when I put in the correct road or something, it's not taking it right. I think we should fix that part.'"
  belongs_to: "Phase 7 (address search box)"
  evidence: "The scratch server log shows 12 searches in 35 seconds, all HTTP 200. The first four (3 to 13 characters) returned results, the last eight (17 to 36 characters, 2 to 4 words) all returned an empty result list. Reproduced with generic road names on 2026-10-06: 'Ring Road' returned five roads in Hanoi, Vietnam; 'Bandar Road Vijayawada' returned relevant local roads; a house number plus road returned shop names that merely contain 'Door No'. The same query sent straight to Nominatim with an India filter and a Vijayawada area bias moved 'Ring Road' to Delhi, so a soft bias alone is not enough."
  cause: "CONFIRMED by reading internal/geocode/client.go and by the experiments above. (1) The outbound request sets only q, format and limit: no country filter and no area bias, so results are ranked worldwide. (2) Nominatim free text search needs every word to exist in OpenStreetMap, so one unknown word (colony name, door number, 'near ...') gives an empty list, and nothing retries with fewer words."
  fix_sketch: "Send countrycodes=in. Look near the pin first (a viewbox around the report map centre, bounded) and widen to all of India only if that finds nothing. When both are empty, retry once with the leading words dropped and label the answer as closest match. Keep the one request per second policy limiter. If results are still poor, evaluate Photon as a second free source."
  size: "medium, a short plan or a quick task; needs the owner to approve the behaviour first"

- id: F3
  title: "Popup looks like it has no voting and only a few details"
  raised: "2026-10-06 during test 5. User: 'I feel like the voting and the other stuff is not added yet. I guess that's in phase three.'"
  belongs_to: "Phase 2 (voting, already built) and Phase 3 (trust details, not built)"
  answer: "Not a defect. Voting is built: applyOwnReportRule in web/static/js/votes.js removes Confirm and Dispute from a reporter's own reports (D-03), and both test reports were the owner's own, so only the basics and Mark resolved showed. A report from a second test account (id 3, Road blocked, Medium) was added so Confirm, Dispute and Mark resolved can be seen. The richer details (confirmed by N nearby, still current, reporter reliability, one popup card) are Phase 3 plans 03-13 and 03-17, planned but not executed."

- id: F4
  title: "The browser asks for location permission on every page reload"
  raised: "2026-10-06 during test 9. User: 'every time I refresh the page, it's asking me if I want to allow for the location.'"
  belongs_to: "Phase 1 map (centre on the visitor) and the browser's per site setting"
  cause: "CONFIRMED by reading web/static/js/map.js: centerOnVisitor() calls navigator.geolocation.getCurrentPosition on every page load, with no Permissions API check and no remembered position. The prompt appears whenever the browser has not remembered an earlier Allow, and Safari is not remembering it for the plain http test address http://127.0.0.1:8792. The report form (modal.js) and the vote buttons (votes.js, cached per tab in sessionStorage) also ask when used. Not a Phase 2 defect."
  tester_workaround: "Safari, Settings, Websites, Location: set 127.0.0.1 to Allow."
  product_options: "(a) leave as is, which is standard behaviour, and rely on browsers remembering the answer on the deployed https site; (b) check navigator.permissions first and start on the fallback centre until the visitor taps a locate button; (c) remember the last known position and centre on it at once. Owner decision, not urgent."

- id: F5
  title: "The popup and its Mark resolved question look odd: a grey surface, a white card inside it, boxes inside boxes"
  raised: "2026-10-06 during test 6, with a screenshot of the map popup in light mode. User: 'it's fine but I just don't like the way it's looking right now it looks odd by having that background and then these text boxes have a separate box and they all look really odd I think we should concentrate on the designing part of this project too ... I think we should fix it.'"
  belongs_to: "The deferred design polish (gap ui-visual-polish, which is not on the roadmap) and Phase 3's popup card"
  observed: "The popup surface is a neutral grey tint (quick task 260923-mb0). Inside it the Unconfirmed chip and the Mark resolved question each sit in their own white bordered box, and Yes and Cancel are two more bordered boxes. The first line reads 'Road blocked · 2 · Medium · 59 min ago', so the slider's number leaks into the severity label."
  already_planned: "Phase 3's UI contract (03-UI-SPEC.md) replaces this popup with one neutral white card, a 4px severity bar, a header with the severity and state labels, a trust block, a two column vote grid with Mark resolved full width, and a resolve question on its own line above its buttons with Cancel first (amendments A2, A16, A19 and A21; plans 03-09, 03-11, 03-13 and 03-17). Polishing the old popup now would be thrown away."
  not_planned: "Everything outside Phase 3's surfaces: buttons and surfaces in general, a radius, shadow and motion token scale, smooth transitions, background and surface elevation, and light and dark polish across the whole app. This is the open part of gap ui-visual-polish."
  route: "Owner decision on where the design pass goes. Recommended: add it right after Phase 3 as its own inserted phase, so it styles the new popup and vote buttons once instead of twice."

- id: F6
  title: "Long dashes in the public API reference page"
  raised: "2026-10-07 by the second review round of the Phase 2 verification report, not by the owner."
  belongs_to: "Plan 02-14's dash cleanup, which scanned templates and client scripts but not generated docs"
  observed: "docs/docs.go (7 lines), docs/swagger.json (7) and docs/swagger.yaml (8) carry long dashes copied from handler and struct comments (for example the show_disputed description). The router serves /swagger/* to any visitor. The owner's rule is no long dashes anywhere a reader can see them."
  fix_sketch: "Rewrite those source comments in plain punctuation, run make swag, and extend TestUserVisibleCopyUsesPlainPunctuation to scan the generated docs so it cannot regress."

tracked_as: "Added to ROADMAP.md as backlog items on 2026-10-07: F1 is 999.6, F2 is 999.7, F6 is 999.8, F4 is 999.9, and the design pass (F5 plus the open part of gap ui-visual-polish) is 999.10. The owner has not yet said whether the design pass goes before or after Phase 3."

## Gaps

- truth: "The map basemap itself visibly changes to match the app's Light/Dark theme, not just the surrounding chrome (badges, buttons, panels)."
  status: withdrawn_by_user_decision
  fix_shipped: "2026-09-23, plan 02-11, commits 288a5d2/fb88e64/a4f1c0d. Both map.js and modal.js resolved the theme from document.documentElement's data-theme and re-styled via a MutationObserver, switching between the liberty style and a newly-added dark OpenFreeMap style. All named unit/contract tests passed."
  withdrawn: "2026-09-23: user tried the dark map live (screenshot) and reported 'map is not visible at all', then explicitly asked to revert: 'I think we can just leave it like how it used to be even if like the user selects the dark mode only the background of like the report side only that will change the map will remain the same.' This is a stated preference, not a bug report on the fix itself; the dark OpenFreeMap style rendered but read as low-contrast/hard-to-parse against the rest of the dark UI. Reverted in commit 77b9804: web/static/js/map.js, web/static/js/modal.js and web/js_contract_test.go restored to their pre-02-11 state (bab6292). The map now always uses the original liberty style regardless of theme, matching the user's explicit instruction; only the surrounding chrome (badges, buttons, panels) follows Light/Dark, as it already did. No further action needed on this gap. TestMapBasemapFollowsTheAppTheme and the two-style host allowlist no longer exist post-revert, by design."
  reason: "User reported (2026-09-21/22, during Test 10): 'even if i change it to the dark mode the map doesn't change much and same goes to the um when i change it to the light mode'."
  severity: minor
  test: 10
  root_cause: "CONFIRMED by debug agent 2026-09-22 (.planning/debug/map-theme-not-applied.md). Both map surfaces hardcode a single light-only basemap with no theme signal path at all, not just one file: map.js's init() (the main map) hardcodes 'https://tiles.openfreemap.org/styles/liberty' (line 74) plus a light-only OSM raster fallback (line 83); modal.js's initLocation() (the report-submission pin-drop map) independently duplicates the identical hardcoded style and fallback (lines 370, 379), a deliberate per-file duplication convention in this codebase, not an oversight, so a fix touching only map.js would leave the report modal's map light-only in dark mode. theme.js sets/removes document.documentElement's data-theme attribute and dispatches no event and calls no shared function; it is contractually self-contained (TestThemeModuleHasNoAppShellDependency, web/theme_contract_test.go:115, fails the build if theme.js's source ever contains the literal 'Pinalert', since it also runs unmodified on pages with no map). Confirmed NOT a load-order race: theme.js applies data-theme synchronously in <head> before body parsing; map.js and modal.js load deferred at end of body, so data-theme is already stable and synchronously readable at each map's construction time. The gap is pure absence of integration code between an already-correct theme signal and two independent, already-duplicated map surfaces."
  artifacts:
    - path: "web/static/js/map.js"
      issue: "init() hardcodes the light MapLibre style/raster fallback with no data-theme read at construction, and discards the L.maplibreGL() layer reference needed to call setStyle() later"
    - path: "web/static/js/modal.js"
      issue: "initLocation() independently duplicates the identical hardcoded light style/raster fallback for the report-submission pin-drop map (lines 370, 379) — must be fixed alongside map.js, not instead of it"
    - path: "web/theme_contract_test.go"
      issue: "TestThemeModuleHasNoAppShellDependency (line 115) build-gates theme.js against containing the literal 'Pinalert', constraining how any fix may connect theme.js's mode changes to the two map modules"
  missing:
    - "Initial load (both map.js and modal.js): resolve the effective mode via document.documentElement.getAttribute('data-theme') || matchMedia('(prefers-color-scheme: dark)').matches, and construct with the liberty or dark OpenFreeMap style accordingly."
    - "Toggle-after-load (the actual reported symptom): without modifying theme.js's no-app-shell-dependency contract, use a MutationObserver on document.documentElement watching attributeFilter:['data-theme'] in map.js and modal.js, calling the retained maplibre layer's getMaplibreMap().setStyle(url) — re-resolving the mode fresh each time, since toggling to System REMOVES the attribute rather than setting a value, so a cached mode would go stale."
    - "Guard required: getMaplibreMap() may not exist yet if the toggle fires before the async geolocation callback's first setView() call (up to an 8000ms window per map.js's own documented timeout), and modal.js's modalMap may be null if the report modal has never been opened; any dynamic handler must no-op safely in both cases rather than throw."
    - "Decide and document what the raster (no-WebGL) fallback does in dark mode: OpenFreeMap has no free dark raster tiles to match, so this is an explicit accept-the-mismatch or find-an-alternative decision, not a silent drop."
  debug_session: ".planning/debug/map-theme-not-applied.md"

- truth: "With Show disputed reports ticked, the list and map show ONLY disputed reports; a fresh live or provisional report does not appear."
  status: resolved
  fix_shipped: "2026-09-23, plan 02-12, commits 10bca11/aa37950/1f649f9. internal/service/feed.go's ListableInFeed is now a true partition (Live/Provisional return !includeDisputed; Hidden returns includeDisputed). Verified against a disposable Postgres database (never pinalert_test): TestListableInFeed (10/10 subtests), TestNearbyHidesDisputedUnlessRequested (5/5), TestShowDisputedRevealsHiddenReports, and three sibling tests the executor found and fixed as a same-file deviation, all pass. Swagger docs and 02-RESEARCH.md's diagram were corrected and regenerated. This defect is code-level, not perceptual, so status: resolved is warranted here (unlike the other four gaps in this round). RETEST STILL NEEDED, separately: Test 9 below (reload persistence, no-unfiltered-flash, new-tab load) was never independently observed and only becomes judgeable now that the disputed view is genuinely empty when nothing is disputed."
  live_retest: "PASSED 2026-10-06 (test 9). User: 'Yeah, it works'. Server log: the tick returned an empty list while three live or unconfirmed reports existed, the reload's first fetch already carried show_disputed=true with no plain fetch before it, and with report 4 genuinely disputed the normal list returned reports 1 and 3 only while the disputed view returned report 4 alone."
  reason: "User reported (2026-09-21, during Test 9): 'still pops up in the disputed reports when I click the show disputed reports... I still see the report, the issue I just added.' Same objection first raised 2026-09-18 (backlog 999.2). The user has now asked for this twice."
  severity: minor
  test: 9
  root_cause: "CONFIRMED by debug agent 2026-09-22 (.planning/debug/disputed-filter-not-exclusive.md), one layer deeper than the orchestrator's preliminary guess: the additive union is not in the HTTP handler, it's in internal/service/feed.go's Visibility.ListableInFeed(includeDisputed bool), whose switch treats VisibilityLive and VisibilityProvisional as unconditionally listable (ignoring includeDisputed entirely) and only makes VisibilityHidden conditional. Called once per report from service.Nearby() at report.go:451, the sole visibility filter behind GET /api/reports (the Activity/profile page's ReportsByAccount has no visibility filter and is unaffected either way). This is deliberate, documented, tested behavior as of today's code, not an accidental wiring bug: asserted in ListableInFeed's own doc comment, 02-RESEARCH.md's data-flow diagram, and two named tests (feed_test.go's TestNearbyHidesDisputedUnlessRequested, whose third subtest is literally named 'the toggle adds Hidden reports, never removes Provisional ones', and handlers/feed_visibility_e2e_test.go's TestShowDisputedRevealsHiddenReports, whose failure message reads 'the toggle should add reports, not replace the view'). Correction to the earlier note: D-10/D-11's actual locked text in 02-CONTEXT.md does not itself mandate additivity, only that Hidden reports be reachable via the toggle; the additive reading was introduced one layer below the locked decision, in 02-RESEARCH.md's diagram and feed.go's implementation/tests, so this fix does not contradict the locked decision's wording. The client (feed.js/app.js/map.js) does zero filtering of its own and renders the server's array verbatim; feed.js's empty-state branch (line 318, picking 'general' vs 'disputed' copy) is already correct and needs no change, it just moves from a rare path to the common one, and map.js's existing marker-reconcile-by-id loop already clears pins correctly on an empty result."
  artifacts:
    - path: "internal/service/feed.go"
      issue: "ListableInFeed's switch returns true unconditionally for Live/Provisional instead of !includeDisputed; the Hidden case is the only one gated on includeDisputed"
    - path: "internal/service/feed_test.go"
      issue: "TestListableInFeed and TestNearbyHidesDisputedUnlessRequested assert the additive contract by name and must be rewritten, not left to fail"
    - path: "internal/api/handlers/feed_visibility_e2e_test.go"
      issue: "TestShowDisputedRevealsHiddenReports's controlID assertion locks in the additive contract at the HTTP level; must flip to expect the undisputed control report's absence"
    - path: ".planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md"
      issue: "Data-flow diagram (lines 168-169) states the additive contract '?show_disputed=true adds {Hidden} (D-10/D-11)'; needs the same one-line correction"
    - path: "internal/api/handlers/reports.go"
      issue: "Swagger @Description/@Param text (lines 305-306, 315) documents the additive behavior; regenerate docs/docs.go and docs/swagger.json|yaml via the swag toolchain afterward, do not hand-edit the generated files"
  missing:
    - "Make ListableInFeed a true partition: case VisibilityLive, VisibilityProvisional: return !includeDisputed, alongside the existing case VisibilityHidden: return includeDisputed, VisibilityRetracted staying false unconditionally."
    - "Update the two named tests and the 02-RESEARCH.md diagram to match the exclusive contract, then update and regenerate the swagger doc comment."
    - "No change needed in feed.js, app.js or map.js (confirmed already correct for this fix), and no change needed for the Activity/profile page (unaffected). Critical-bypass reports stay Live even when heavily disputed (D-06) and are unaffected by this fix, worth a one-line note in the fix's own commit or plan."
    - "Re-run UAT Test 9's reload-persistence and no-unfiltered-flash checks after this lands; they become easy to judge for the first time once the disputed view is genuinely empty when nothing is disputed."
  debug_session: ".planning/debug/disputed-filter-not-exclusive.md"

- truth: "The app looks and feels polished: buttons, backgrounds and surfaces look intentionally designed, and interactions and transitions are smooth, not like a beginner's first web project."
  status: partially_fix_shipped_rest_deferred
  fix_shipped: "2026-09-23, plan 02-14, commits af04ca2/33295b5/686b1d0. Only the mechanical half of this gap was closed, exactly as scoped: all 10 confirmed em-dash sites rewritten to plain punctuation (one more site found during execution than the debug session listed, a third copy of the rate-limit sentence in auth.go; all three copies are now byte-identical), and the two 999px pill radii (.view-toggle, #toast) changed to 8px matching .category-tile. Both are gated by new standing tests (web/design_rules_contract_test.go). THE LARGER COMPLAINT THIS GAP IS ABOUT IS UNCHANGED BY DESIGN: the hover/active/transition motion system, the radius/shadow/duration token scale, the background/surface-elevation treatment, and the theme-toggle icon redesign are explicitly deferred to a future /gsd-ui-phase design contract, not a code patch. Do not read this gap as closed; the 'looks like a beginner project' impression the user described is the still-open part."
  update_2026_10_06: "The theme toggle icon item shipped through quick tasks 260923-qwi (two state icon button) and 260923-rra (floating button under the report button), and the owner confirmed it works (test 10). The rest of this gap is still open and is now an owner priority: on 2026-10-06 the owner said 'we should concentrate on the designing part of this project too' after seeing the popup (finding F5). Phase 3's UI contract already redesigns the popup card, the vote buttons, the trust block and the Activity rows. NOT covered anywhere yet: the radius, shadow and motion token scale, smooth transitions, background and surface elevation, and light and dark polish for the rest of the app. This is not on the roadmap. Decision pending on where the design pass goes (recommended: right after Phase 3)."
  reason: "User reported (2026-09-21, while on the Activity page during Test 8, screenshot attached): 'the buttons and like the way the website is designed right now looks like a star beginner doing to start... I don't want it to look like that I want it to be like looking smooth transitions very smooth process I don't know these buttons and the background I don't know I think we should fix it'. Also flagged that a button label for the light/dark feature 'looks really odd' and asked for it to be fixed."
  severity: major
  test: general (raised during Test 8, not a failure of Test 8's own expectation)
  root_cause: "CONFIRMED by debug agent 2026-09-22 (.planning/debug/ui-visual-polish.md). Design-consistency gap, not a single defect, with a structural cause: the approved 02-UI-SPEC.md (6/6 dimensions passed) never mentions hover, transition, motion or radius anywhere in the document, so of roughly 11 distinct button-family selectors built against that contract, only ONE (.profile-back-link, added later in plan 02-10) has any hover/active/transition at all; every other control (.btn/.btn--primary/.btn--destructive, .vote-btn and its 5 state variants, .account-trigger, .view-toggle, .fab, .category-tile, .account-menu__item) is flat and instant, confirmed exhaustively by grepping :hover/:active/transition across all 5 CSS files. Second independent driver: effectively one flat background tone (--color-bg) reused almost everywhere with no surface-elevation system, matching the user's own 'background' complaint. No radius/shadow/transition-duration token layer exists alongside the project's already-established --space-*/--color-* tokens. Re-swept site-design-rules.md against the live code: 9 user-visible em-dash occurrences across 8 files (2 previously unrecorded: internal/mailer/resend.go's real verification-email disclaimer, and internal/ratelimit/perip.go's rate-limit message, which textually duplicates auth.js's client-side copy), and 2 pill-radius (999px) violations (.view-toggle, #toast). No gradient, emoji, fake-metrics, or scroll-triggered-animation violations found (the app's only 3 animations, an ambient globe rotation, a loading-skeleton pulse and a 200ms toast entry, are all restrained). One unresolved borderline call: login_gate.html.tmpl's <h1>Verify your email to continue</h1> reads as a functional gate heading, not a marketing hero, on plain inspection, but deserves an explicit design-pass ruling rather than an assumption. The 'wordy' theme label is confirmed (account_header.html.tmpl:11-13); votes.js's REOPEN_BUTTON_LABEL 'Reopen · not actually resolved' uses a middle dot (U+00B7), not a dash, so it does not itself violate the dash rule, and the user has still not confirmed this is the control they meant."
  artifacts:
    - path: ".planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md"
      issue: "The approved design contract is silent on hover, transition, motion and radius, the structural root cause of the flat, instant, inconsistent button family"
    - path: "web/static/css/main.css"
      issue: ".btn/.btn--primary/.btn--destructive, .view-toggle, .fab, .category-tile all lack hover/active/transition; .view-toggle (line 595) and #toast (line 642) use a 999px pill radius; no radius/shadow/transition-duration token scale exists"
    - path: "web/static/css/trust.css"
      issue: ".vote-btn and its 5 state variants (lines 36-320) all lack hover/active/transition"
    - path: "web/static/css/auth.css"
      issue: ".account-trigger and .account-menu__item lack hover/active/transition; .profile-back-link (lines 386-421) is the one working precedent, 150ms ease with a prefers-reduced-motion override, that a polish pass should generalize"
    - path: "web/templates/account_header.html.tmpl"
      issue: "CONFIRMED 2026-09-22: the 'Theme: Dark'/'Theme: Light'/'Theme: System' text label (lines 11-13) is the odd control the user meant, 'it looks really wordy and I don't like it that way'. They asked for research into how other apps present a theme toggle before redesigning it. Candidate: reuse the existing .auth-icon masked-SVG system already used for user.svg/log-out.svg/arrow-left.svg, consistent with the no-emoji-icon rule, rather than a text label."
    - path: "web/static/js/votes.js"
      issue: "REOPEN_BUTTON_LABEL (line 79) 'Reopen · not actually resolved' uses a middle dot, not a dash; not itself a site-design-rules.md violation, and still unconfirmed as the control the user meant"
    - path: "9 files with user-visible em dashes"
      issue: "web/templates/profile.html.tmpl:6, verify_outcome.html.tmpl:6, login_gate.html.tmpl:6 and :19, check_inbox.html.tmpl:3, index.html.tmpl:71, web/static/js/auth.js:126, internal/mailer/resend.go:21 (real email sent to users), internal/ratelimit/perip.go:29 (duplicates auth.js's client-side message)"
  missing:
    - "Bundle into THIS gap-closure round (mechanical, no design judgment needed): replace all 9 em-dash occurrences with plain punctuation across the 8 files above; check for any contract/unit test asserting the exact current string before editing perip.go or auth.js's shared rate-limit message, since they textually duplicate each other."
    - "Bundle into THIS round with a caveat: change the two 999px pill radii (.view-toggle, #toast) to a modest rounded rect, 8px is a defensible interim choice since it already matches .category-tile and the #account-menu panel rather than inventing a third value; the later UI phase may re-tokenize it."
    - "DEFER to /gsd-ui-phase (a proper UI-SPEC design contract, not a code fix): a hover/active/transition motion system applied consistently across the whole button family, generalizing .profile-back-link's working 150ms-ease-plus-reduced-motion pattern; a radius/shadow/transition-duration token scale alongside the existing --space-*/--color-* tokens; a background/surface-elevation treatment (whether the modal panel and primary buttons should sit on --color-surface rather than --color-bg); the theme-toggle icon redesign (user asked for prior research, now delivered above); REOPEN_BUTTON_LABEL wording, once the user confirms it is in scope, via the project's existing Copywriting Contract process; and an explicit ruling on login_gate.html.tmpl's <h1>, recording a considered decision rather than an assumption."
  debug_session: ".planning/debug/ui-visual-polish.md"

- truth: "A fresh non-critical report's badge/border is visibly desaturated (D-09 Provisional dimming) on both the feed row and the map pin popup, in addition to the Unconfirmed chip."
  status: resolved
  fix_shipped: "2026-09-23, plan 02-13, commits 8c5f391/9e5a97b/79cd442. .vis-provisional's color-mix strengthened from 50/50 to 20/80 toward --color-age-stale, plus --severity-tint neutralized on the feed row so the background wash no longer stays full-strength green. Measured chroma ratios after the fix: 0.15-0.23 (well under the 0.30 ceiling), luminance contrast 2.93-3.16:1 dark mode and 1.71-1.83:1 light mode (both above their floors). A new standing test, TestProvisionalDimmingIsPerceptiblyDistinctFromLive, was confirmed RED against the old 50/50 mix (chroma ~0.46-0.52, matching the diagnosis's prediction almost exactly) and GREEN after the fix. A computed contrast number is not the same thing as the user perceiving it, which is exactly what this gap is about, so this is NOT marked resolved. Retest: look at a fresh unconfirmed report's badge and border on the feed row and the map marker icon (not the popup, which never had a badge by design), in both themes, and confirm it now visibly reads as faded next to a normal report."
  live_retest: "PASSED 2026-10-06. Dark mode (test 5, user: 'I think the colors thing is fine') and light mode (test 12, user: 'they're all visible', a general answer to a checklist whose first item asked about the Low report looking faded with a readable Unconfirmed chip; the user did not separately say it looked faded, and light mode is the numerically weaker case, so this half rests on that general answer plus the contrast gate). The pin badge and the grey popup box were not asked about one by one."
  reason: "User reported: I don't see the faded look but I definitely see the unconfirmed chip to it. The user does not see any fading on a fresh unconfirmed report, on the feed row or the map popup, so the desaturation half of the Provisional treatment does not work for them."
  severity: major
  test: 5
  root_cause: "CONFIRMED by debug agent 2026-09-22 (.planning/debug/provisional-dimming-not-perceptible.md): NOT a code defect, .vis-provisional is applied correctly to both real target elements (feed.js's .report-row ancestor, map.js's marker .icon-badge), matching 02-UI-SPEC.md's own target list, with correct cascade order (guarded by an existing test, TestVisibilityCascadeOverridesAgeRamp). The defect is design strength: .vis-provisional's 50/50 color-mix toward --color-age-stale is deliberately the SAME ratio as the intermediate .age-aging stage, not the strongest step. Computed WCAG contrast between dimmed and undimmed: dark-mode Low ~1.97:1, dark-mode Medium ~1.89:1, light-mode Low ~1.46:1 (and light mode shifts LIGHTER, not darker, since light-mode --color-age-stale is a near-white gray, the opposite direction from dark mode). For comparison, this app's own strongest existing step, Fresh to Stale at 100% mix, reaches ~4.29:1, more than double. No test in css_contract_test.go asserts a minimum perceptual distance between Provisional and the undimmed baseline (the only existing guard checks Provisional stays string-equal to Aging, never that either is far enough from Live), which is why this shipped past every automated gate. Secondary factor, feed row only: .vis-provisional overrides --severity-current and --badge-glyph-fg but NOT --severity-tint, so the row's background wash stays full-strength severity color, reinforcing 'still looks green' even as the border dims; this does not apply to the map marker, which has no tint background, explaining why the row specifically reads as more unchanged than the pin. Terminology correction: the UAT's 'map pin popup' phrasing is ambiguous. The marker icon on the map (buildBadgeElement) gets the dimming treatment; the separate popup box that opens on tap (buildPopupContent) contains no badge/border element at all, by design, matching the UI-SPEC's own target list. The pixel-sampled colors in the earlier note could only have come from the marker icon, not the popup interior; a fix must not add a badge inside the popup, that would contradict the approved UI-SPEC."
  artifacts:
    - path: "web/static/css/trust.css"
      issue: ".vis-provisional (line 128) uses the same 50% mix ratio as the intermediate .age-aging stage instead of a stronger, more perceptible one, and does not touch --severity-tint (the feed row's background wash stays full strength)"
    - path: "web/static/css/main.css"
      issue: ".age-aging (line ~227) declares the identical mix value, deliberately reused per trust.css's own documented rationale; any ratio change to .vis-provisional must account for this shared value, not just edit it in isolation"
    - path: "web/css_contract_test.go"
      issue: "TestVisibilityCascadeOverridesAgeRamp asserts .vis-provisional and .age-aging stay string-equal; changing the ratio will break this test and requires a deliberate decision to sever that reuse, not an accidental regression"
  missing:
    - "Strengthen .vis-provisional's mix ratio well past 50%, roughly 80%+ toward --color-age-stale, to approach the ~3:1-ish contrast this app's own Stale stage already achieves at 100%. This is a product decision (Provisional would read almost as gray as an about-to-expire report), not just a number change, and needs TestVisibilityCascadeOverridesAgeRamp's string-equality assertion updated or removed alongside trust.css's 'Provisional reads like Aging' rationale comment."
    - "Consider having .vis-provisional also override --severity-tint toward a neutral/muted tint, removing the feed-row-only 'still looks green' background-wash confound that the map marker does not share."
    - "Re-verify in light mode once fixed (Test 10 is still open): light mode's contrast is even weaker (~1.46:1) and shifts in the opposite luminance direction from dark mode, so this is very likely to reproduce there too, not a dark-mode-only issue."
    - "Add a contrast-distance test to css_contract_test.go asserting a minimum perceptual gap between Provisional and Live, not just Provisional-equals-Aging, to close the coverage gap that let this ship."
  debug_session: ".planning/debug/provisional-dimming-not-perceptible.md"

- truth: "Tapping 'Mark resolved' hides the .vote-btn--resolve button itself (not just Confirm/Dispute) once the inline Yes/Cancel confirmation appears, on the feed row."
  status: resolved_cause_unconfirmed
  fix_shipped: "2026-09-23, plan 02-13, commit 9e5a97b (same plan as the dimming fix, same file). Applied ONLY the low-risk defensive fix the inconclusive debug session recommended: added the :not([hidden]) guard to trust.css's .vote-btn--resolve rule, for consistency with every sibling rule in that block. This is NOT a confirmed repair; extensive headless testing in both Chromium and WebKit never reproduced the symptom at all, in isolation, with full DOM ancestry, or under a simulated background poll. Retest: repeat the original steps (Mark resolved on someone else's report, feed row, dark theme). If it still happens, the decisive next step is a live DevTools Elements/Computed panel check at the moment it appears, on the actual device, not more static investigation. The map popup surface was never covered by this investigation either and may need its own check."
  live_retest: "PASSED 2026-10-06 (test 6). User: 'it's fine', with a screenshot of the map popup in light mode showing the three buttons replaced by the question box. The symptom did not recur on a freshly started server running today's code. The original cause was never identified, so this is recorded as resolved with the cause unconfirmed: a stale server in the earlier session remains the most likely explanation."
  reason: "User reported (live, against the freshly-restarted server running today's code): Confirm/Dispute now correctly disappear, but the Mark resolved button itself stays visibly rendered next to the confirmation box, stretched tall by flexbox align-items:stretch, rendering as an odd white/black rectangle against the dark theme."
  severity: major
  test: 6
  root_cause: "INVESTIGATION INCONCLUSIVE (debug agent, 2026-09-22, .planning/debug/resolve-button-not-hidden.md). Extensive testing found no reproducible cause: static reading of votes.js/trust.css confirms openResolveConfirm/closeResolveConfirm hide all three buttons identically and symmetrically, and .vote-btn--resolve declares no display property, so it cannot contest the 02-08 fix's .vote-btn:not([hidden]) guard the way an unguarded display rule would. Cross-file grep found no other rule touching .vote-btn--resolve or any competing specificity/!important. Git history confirms the code tested against is byte-identical to what's in the repo now (no drift since the 02-08 fix landed 2026-09-18). Built an empirical headless reproduction in both Chromium and WebKit, using the real CSS load order and the real createVoteBlock/updateVoteBlock code, first in isolation then with full real DOM ancestry matching feed.js's actual row structure, then simulating a background poll landing mid-confirmation (mirroring feed.js's 30s poll): in every case, in both engines, all three buttons hide correctly with no stray rectangle. Also checked map.js's popup-rebuild-on-every-render mechanism; real, but would cause a full reset of all three buttons, not the specific confirm/dispute-hidden-but-resolve-visible asymmetry reported, and only affects the map popup, not the feed row. One genuine but currently inert finding: trust.css's .vote-btn--resolve rule (in the Mark Resolved block) is the ONLY rule in that block missing the :not([hidden]) guard every other display-bearing rule in the file carries; it sets no display property itself so this had zero observed effect in every test here, but its own background/color (light background, near-black text) is exactly the 'white/light rectangle with black text' color signature the user described, and stays computed even while hidden. Circumstantial, not causal, since display:none was confirmed to win every time tested. Two live possibilities remain unruled-out: a genuine device/browser-specific quirk (an actual iOS Safari rendering path or touch-timing race) not reproducible in desktop Chromium/WebKit, or a stale-server/stale-tab artifact in the tester's own session (the same category Test 7's map-popup legibility issue independently turned out to be, on that occasion traced to a stale server). Scoping note: 'on the feed row' in this truth line traces to a prior summarization, not a verbatim user quote; the original report named both the feed row and the map popup, and the map popup surface specifically was not covered by this investigation."
  artifacts:
    - path: "web/static/js/votes.js"
      issue: "openResolveConfirm's hidden=true assignment on .vote-btn--resolve behaves identically to .vote-btn--confirm/--dispute in every test run; no defect found here"
    - path: "web/static/css/trust.css"
      issue: "The .vote-btn--resolve rule in the Mark Resolved block is the one rule in that block missing the :not([hidden]) guard every sibling display-bearing rule carries; harmless in every test performed, but a defensive, convention-consistent fix regardless"
  missing:
    - "Defensive fix, low risk, apply now: add the :not([hidden]) guard to trust.css's .vote-btn--resolve rule for consistency with every other rule in the same block, even though causality was not proven."
    - "If the symptom recurs after that fix ships: the decisive next step is a live DevTools Elements/Computed panel check at the moment the white rectangle appears, on the actual device, confirming whether the element truly carries hidden='' and which rule wins for display. That single observation resolves this decisively."
    - "Separately confirm the map popup surface, which this investigation did not test (only the feed row was covered)."
  debug_session: ".planning/debug/resolve-button-not-hidden.md"

- truth: "Tapping 'Mark resolved' replaces the Confirm/Dispute/Mark-resolved button row IN PLACE with the inline Yes/Cancel confirmation (D-15), on both the feed row and the map popup."
  status: resolved
  reason: "User reported: the original three buttons stay visible next to the new Yes/Cancel confirmation instead of being replaced, on both the feed row and the map popup — looks oversized/doubled up."
  severity: major
  test: 3
  root_cause: "trust.css line 30's .vote-btn rule has no :not([hidden]) guard, unlike every other display rule in the same file. votes.js's openResolveConfirm() correctly sets hidden=true on the three buttons, but the browser's native [hidden]{display:none} user-agent rule loses to the equal-specificity author .vote-btn{display:inline-flex} rule (author styles win ties over user-agent styles), so the buttons stay rendered despite the hidden attribute."
  artifacts:
    - path: "web/static/css/trust.css"
      issue: ".vote-btn selector (line 30) missing the :not([hidden]) guard every other display rule in this file carries"
  missing:
    - "Change `.vote-btn {` to `.vote-btn:not([hidden]) {` in trust.css, matching the file's own documented pattern"
  debug_session: ""

- truth: "The 'Mark this report resolved?' confirmation heading is legible (--color-text on --color-bg) in the map popup, same as the feed row."
  status: resolved
  reason: "User reported: the confirmation text box that appeared on the map popup doesn't seem visible/legible."
  severity: major
  test: 3
  root_cause: "Investigated but INCONCLUSIVE from static analysis alone. Fetched leaflet@1.9.4's actual shipped CSS (unpkg) and checked every color/opacity/!important declaration touching .leaflet-popup*: the only color rule is `.leaflet-popup-content-wrapper, .leaflet-popup-tip { color: #333 }` at specificity (0,1,0), which .resolve-confirm p's own `color: var(--color-text)` rule at (0,1,1) should out-specify and win on paper. No !important, no higher-specificity popup text rule, and no opacity/fade rule scoped to just the heading (the .leaflet-fade-anim popup-open fade affects the whole popup uniformly, not selectively the heading, and doesn't match the symptom of sibling buttons rendering at full contrast while only the heading is washed out). Needs live DevTools computed-styles inspection to see what's actually being applied at render time — this could not be resolved by reading source alone."
  artifacts:
    - path: "web/static/css/trust.css"
      issue: ".resolve-confirm p color rule is correct by static specificity analysis; the discrepancy with the live rendering is unexplained without a browser inspector"
  missing:
    - "Live-inspect the rendered popup DOM's computed color/opacity for the confirmHeading <p> to find what's actually overriding it, then fix that specific rule"
  debug_session: ""

- truth: "After tapping Reopen on the Activity page, navigating back to the main feed shows the report live again immediately, with no manual reload needed."
  status: resolved
  reason: "User reported: had to manually refresh the page after going back to the map to see the reopened report's updated state — going back showed stale data instead."
  severity: major
  test: 3
  root_cause: "page.go correctly sets Cache-Control: no-store on the main document (DEC-Q), which per spec should make Chrome/Firefox exclude the page from back-forward-cache (bfcache) entirely. Confirmed no-store is NOT set on GET /api/reports itself either (only static assets get a Cache-Control header, in router.go's staticFileServer — the reports JSON endpoint sets none), though that's a separate, lower-priority gap from the bfcache issue. User is on Safari, and Safari's WebKit Page Cache is documented to NOT reliably honor Cache-Control: no-store for bfcache eligibility the way Chromium/Firefox do — so a Safari Back navigation can still restore a frozen pre-reopen page snapshot without re-running JS or refetching, despite the header being correct. This is the most likely explanation but is a browser-behavior claim, not something confirmed via live repro in this session."
  artifacts:
    - path: "web/static/js/feed.js"
      issue: "No pageshow/event.persisted listener to force a re-fetch when the page is restored from bfcache"
  missing:
    - "Add a window.addEventListener('pageshow', ...) handler that calls Pinalert.fetchReports() when event.persisted is true, as a defensive fix alongside the existing Cache-Control: no-store header"
  debug_session: ""

- truth: "The Activity/profile page offers a way to navigate back to the main map/feed view."
  status: resolved
  reason: "User reported: after viewing Activity, there's no way to go back to the main map to see what's happening, other than the browser's own back button."
  severity: minor
  test: 3
  root_cause: "Confirmed via inspection: web/templates/profile.html.tmpl has no back/home navigation link at all."
  artifacts:
    - path: "web/templates/profile.html.tmpl"
      issue: "No link back to the main feed/map view"
  missing:
    - "Add a 'Back to map' (or similar) link/button on the Activity page pointing to the main feed route"
  debug_session: ""

- truth: "Reloading the page while 'Show disputed reports' is checked keeps the same filtered view instead of resetting to the default feed."
  status: resolved
  reason: "User explicitly requested this behavior change: a page refresh currently resets the checkbox to unchecked and returns to the default feed view; the user wants it to stay on the disputed view instead."
  severity: minor
  test: 2
  root_cause: "By design, not a pre-existing bug: the checkbox (02-UI-SPEC.md) is a plain <input type=checkbox> with no checked attribute, no localStorage, and no URL query-param wiring — Phase 2 deliberately scoped it as an ephemeral client-side filter, not a persisted preference. This gap changes that scope at the user's explicit request."
  artifacts:
    - path: "web/templates/index.html.tmpl"
      issue: "show-disputed-toggle checkbox has no persistence mechanism"
    - path: "web/static/js/feed.js"
      issue: "showDisputedToggleEl listener does not read/write persisted state (e.g. a URL query param) on load"
  missing:
    - "Persist the toggle's checked state (e.g. via a URL query parameter or localStorage) and restore it on page load"
  debug_session: ""

- truth: "A person can switch between light and dark mode from within the app (e.g. in Settings) rather than only via the OS-level appearance setting."
  status: resolved
  reason: "User explicitly requested this new feature so light mode can be verified and used without changing OS-level system settings."
  severity: minor
  test: 2
  root_cause: "New feature, not a defect. main.css already has :root[data-theme=\"dark\"] and :root[data-theme=\"light\"] override blocks specifically prepared for this (its own header comment: 'so a future theme toggle needs no restructuring'), but no UI control or JS exists yet to set the data-theme attribute."
  artifacts:
    - path: "web/static/css/main.css"
      issue: "data-theme override blocks exist but nothing in the UI/JS sets the attribute"
  missing:
    - "Add a toggle control (e.g. in a Settings section) that sets document.documentElement.dataset.theme and persists the choice (e.g. localStorage)"
  debug_session: ""
