---
status: testing
phase: 04-robustness-real-world-resilience
source: [04-VERIFICATION.md, 04-27-SUMMARY.md, 04-REVIEW-SERVER.md, 04-REVIEW-WEB.md]
started: 2026-10-16T18:40:00+05:30
updated: 2026-10-22T23:45:00+05:30
---

## Current Test

number: 11
name: Search a place, and tap to place the pin
expected: |
  On the laptop, in Search a place at the top, type a near spelling such as "Benz Circl" or "Patamat": suggestions appear a moment after you stop typing, the right place comes first, inside India and near you, and typing fast never shows an error. Area names end with the district and state. Then press Post a report: on the window's small map, one tap (or click) anywhere puts the pin on that exact spot. You do not have to send the report; Close is fine.
awaiting: user response

## Tests

Owner questions first (each has a recommended answer that stands unless the owner says otherwise), then one viewing session on laptop and phone, then the checks only a person can do. Resumed 2026-10-22 on the Phase 3.4 layout (test copy port 8778, database p31_4_uat, main 024f94b with Phase 3.2 merged): the screens first (6, 7, 10, 11), then the decisions as one confirmation, then the owner only checks. Automated proof so far: the full Go suite green on a fresh database after every merge (876 tests at 5f1ee0f, the one skip waits for item 12), two deep code reviews (server and web) with every warning fixed, a skeptic check of those fixes (its findings fixed in round 3), the phase check in 04-VERIFICATION.md, and a browser pass in Chromium and WebKit (its rerun on the fixed code is pending).

### 1. SC7, the bar links Disclaimer only
expected: The bar sentence stays as the owner approved it, ending with Disclaimer. Recommended: keep it, so the bar stays short on a phone; Policy is one tap away. Keeping it also records an override of roadmap success criterion 7, which said both pages are linked from the bar.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 2. The network limit
expected: Only once CLIENT_IP_HEADER is set on the real host. One network address (many phones on one mobile network can share it) gets its own allowances, on top of each account's limits: posts 2000 at once then 1 a second, votes 2000 then about 7 a second, flags 1000 then 1 every 1.2 seconds. A refusal reads "A lot of requests are coming from your network right now. You can try again in {wait}." Recommended: keep. Trade-off: someone with many accounts on one network can post more before being stopped, and a flood could use up the abuse check's quota, which keeps new reports held until a moderator acts.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 3. Low data page size
expected: The Low data page is about 160 KB as it travels (compressed), under the planned 256 KB, which the UI-SPEC counts compressed; unpacked it is about 536 KB. Measured in the browser check on 2026-10-16: the list shows in about 5 seconds on a slow connection (400 ms delay, 50 KB a second) and about 14 seconds on the slowest setting (Slow 3G). Recommended: keep it as is.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 4. The gibberish rule counts letters only (04-02, D-05)
expected: The "random characters" refusal looks only at runs of letters, so ten exclamation marks, a row of emoji or a phone number are never refused as gibberish. Recommended: keep.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 5. Six defaults chosen during planning (04-CONTEXT)
expected: (a) Flag stays the last control of every Low data row. (b) The vote limits cover all five vote routes; the take back is no longer refused by the 30 second wait, so Undo works (fixed 2026-10-16). (c) A held or removed report drops off other people's My votes list. (d) On the Past tab and on report links, Flag doesn't know you already flagged until you press it. (e) The moderator screen shows Expired or Resolved for reports older than 7 days. (f) "Written 4 hours ago" sits on the same line on the compact row. Recommended: keep all six.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine." Item (a) is superseded by the owner's 3.4 and 2026-10-22 decisions: Flag is the calm last row of a card (the phone Low data cards keep it in their corner).
### 6. Your own held or removed post on Activity (04-16)
expected: A post of yours that is held or removed counts under the "Not resolved" tab on Activity, and its card says Held or Removed. Recommended: keep, because it is still your open post.
result: pass
note: Owner, 2026-10-22, on the 3.4 layout (My activity, Posted by you): pass. The owner asked why the posts were removed; they were sample posts put in those states by hand for the test (the real check would pass them).

### 7. Disclaimer: the state helplines line and the Sachet link
expected: Checked 2026-10-16 against government sites: 112, 100, 101, 108 and 1070 match; 1077 is the district line in some states only (Andhra Pradesh uses 1070 for both). The Disclaimer page's link to NDMA's Sachet help desk lists some numbers that differ from the states' own sites (one looks a digit short). Recommended: say "in many states" for 1070 and 1077, and replace the Sachet link with a sentence telling people their state disaster authority's website lists local numbers.
result: issue
reported: "Are you sure these helpline numbers are right?" Checked again 2026-10-22 against india.gov.in/directory/helpline, the APSDMA 24x7 portal and district pages: 112, 100, 101, 1070 right; 1077 is district level in many states (not confirmed for Andhra Pradesh, which the page's "many states" and "These vary by state" already cover); 108 is the ambulance number in most states but the national portal lists 102 (National Ambulance Service). Owner: "yes add the 102 note".
severity: minor
note: The recommended change is already made (commit 2204836, 2026-10-16): the page says many states use 1070 and 1077 and the SACHET help desk link is gone. Confirm it when you look, or ask to undo it.

### 8. Read the new messages once
expected: The new messages are in the 04-UI-SPEC copy table (rows added 2026-10-16): the network limit message, "saved under a different account", "can't read the saved reports", the messages after closing the form while it was sending, and "No exact match for the full address". Recommended: keep as written.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 9. Small behaviour notes
expected: Accept unless you disagree. (a) A late removal by the automatic check shows the author "Removed after review", the same as a moderator's; the Policy page now says so. (b) Closing the report form while it is sending closes at once and a message tells the outcome; edits typed after pressing Post are dropped. (c) A report whose check record could not be saved twice stays held with no automatic recovery; moderators can still find it by number. (d) The abuse check's quota errors are not retried. (e) The developer API page (/swagger) has no unofficial bar. (f) A moderator's Keep or Remove and its audit record are now saved together. (g) Right after Undo, marking the same report resolved again waits 30 seconds; the Undo itself never waits.
result: pass
note: Owner, 2026-10-23, on the recommendations: "We already decided about the flag thing. So everything else is fine."

### 10. Viewing session: Phase 4 screens
expected: On a test copy you open on your laptop and phone: the unofficial bar in light and dark; the round account button at the top right of the phone (kept by Phase 4; the 3.2 bell will want that corner too); the phone following its own light or dark setting with no button; the Low data page; the Flag button and dialog; the moderator screen; the "waiting to send" strip on Activity; the Held and Removed labels on Activity (no outline on a phone, outlined on a laptop). Say what to move. Phase 3.1 items 3 and 10 are looked at in the same session.
result: issue
reported: "this doesnt makes sense, because like we're giving them options for the first top three, and you're explaining out of nowhere about dispute in the middle, and then you'll only able to select one. I think it should be more like we are asking why do they think it's it's an issue. Maybe we should give them like a text box also to explain what's going on."
severity: major
note: The Flag dialog part of test 10 (2026-10-22). Proposed to the owner: title "Why are you flagging this report?", the Dispute line moved under the title, reasons as tick boxes (more than one), an optional "Tell the moderators more" box (300 characters, text checked like reports, shown on the moderator screen). Needs a migration (00010), the flag API, the moderator screen. Moderator screen part of test 10 still to look at.

### 11. Viewing session: address search and placing the pin (quick tasks 261016-adr, -pho, -sfu, -sne, -tap)
expected: Type a close spelling or half a name, for example "Banjra Hills" or "Banjara Hi": suggestions appear a moment after you stop typing, the right place shows first, inside India and near you, and typing fast never shows an error. Area names end with the mandal and the state, for example "Gachibowli, Serilingampalle mandal, Telangana". A long address that isn't on the map shows the nearest road and says so. Then a tap anywhere on the report form's small map puts the pin on the exact spot, with location on or off.
result: issue
reported: "I mean the map is just zooming out. It's not pointing any place on my laptop." then "it's not taking any proper place. If I say like one of the restaurant to like make it as a pinpoint of like a famous place so that people would know, that place is only not available. That should not happen, right? We should make sure it works fine."
severity: major
note: Checked 2026-10-23 through /api/geocode/place near Benz Circle: Kanaka Durga Temple, Prakasam Barrage, Babai Hotel and Barbeque Nation are found and placed right; PVP Square is not in the map data and the answer then leads with far places of similar names (Delhi, Bhopal, Mumbai). Picking a place moves and zooms the Nearby map but draws no mark at the place. A tap on the Post a report map places the pin (checked in WebKit, location on and off). Built by plan 04-32.

### 12. Record the moderation scores
expected: With your OpenAI key, from the repository: `read -rs OPENAI_API_KEY`, then `MODERATION_LIVE=1 OPENAI_API_KEY="$OPENAI_API_KEY" go run ./cmd/modrecord`, then `unset OPENAI_API_KEY`. Afterwards TestScreeningFixture passes instead of skipping. Full steps: 04-27-SUMMARY.md, "Owner checks" item 1.
result: [pending]

### 13. Real phone on a weak network
expected: On your phone with a weak signal or the slowest network setting: the Low data page loads and you can post and confirm; offline, a report is saved and sends once when you're back online; at the largest text size nothing overlaps; VoiceOver or TalkBack reads the bar, the limit messages and the waiting strip.
result: [pending]

### 14. 112.gov.in opens
expected: The research could not open 112.gov.in from here. Open it in a browser in India and check it still calls 112 the single emergency number.
result: pass
note: 2026-10-22: the owner could not open 112.gov.in from India either, and the app links to it nowhere. The fact it was to confirm holds from other official sources: india.gov.in/directory/helpline lists 112 as the integrated helpline (ERSS: police, fire and rescue, health), and the PIB releases on the Dial 112 launch (relid 188727) and rollout (PRID 1594669) say the same. No change to the app.

### 15. Before any wide promotion (not needed before the push)
expected: A lawyer or mentor reads the Disclaimer and Policy pages. Set CONTACT_EMAIL so both pages say how to complain or ask for removal. Once a host is chosen, send the CLIENT_IP_HEADER canary (04-27-SUMMARY.md item 2). Optional: a fluent reader skims the Hindi, Telugu and Tamil sample texts.
result: [pending]

## Summary

total: 15
passed: 9
issues: 3
pending: 3
skipped: 0
blocked: 0

## Gaps

- truth: "The Disclaimer's ambulance number is right for every state"
  status: failed
  reason: "User asked to add the 102 note (2026-10-22): the national portal lists 102 as the National Ambulance Service; 108 is the number in most states"
  severity: minor
  test: 7
  artifacts: [the Disclaimer text (search: Older numbers for police), internal/api/handlers/legal_e2e_test.go pins]
  missing: ["ambulance (108, or 102 in some states)"]
- truth: "The Flag window asks why the report is a problem, lets people give every reason that applies and say more in their own words"
  status: failed
  reason: "User reported: the Dispute note sits in the middle of the options, only one reason can be picked, and there is no box to explain"
  severity: major
  test: 10
  artifacts: [web/templates/flag_dialog.html.tmpl, web/static/js/flag.js, internal/service/flag.go (one FlagReason), the flags table, the moderator queue (modFlagsJSON)]
  missing: [title as a why question, the Dispute line under the title, tick boxes, an optional note of up to 300 characters screened like report text and shown to moderators]
- truth: "Flag this report looks the same everywhere, as the calm row of the laptop Low data view, and Mute (Phase 3.2) says plainly what it does and sits with it"
  status: failed
  reason: "User reported (2026-10-22, phone report view): \"why in the phone it's saying mute this report instead of something else, and why is it just like another button? Why is it not like how it is in the laptop version for the low data view. I think we should keep it that way for all of the flagging this report thing.\""
  severity: minor
  test: 10
  artifacts: [the laptop report popup Flag (boxed, border on all sides), the 3.2 mute control (mute.js, updates.css; a small outlined button between Mark resolved and the trust card)]
  missing: [Flag as the calm full width row on the laptop popup too; Mute as a calm row above Flag with a bell off glyph, reading "Stop updates about this report" and "Get updates about this report again"; proposed to the owner, answer pending]
- truth: "A place search answers with places near the person, says plainly when nothing near matches, and the picked place is marked on the map"
  status: failed
  reason: "User reported: the map just zooms out and points at no place; a famous place used as a landmark is not available"
  severity: major
  test: 11
  artifacts: [internal/geocode/photon.go (bbox India, near bias), internal/api/handlers (GeocodePlace), web/static/js/map-tools.js (the Nearby pick), the Post a report location search]
  missing: [drop far matches when near ones exist and say so when none are near, a mark on the Nearby map at the picked place at street zoom, the tap the map hint when nothing matches]

Fix round, 2026-10-23: plans 04-30 (design fixes, merge b4c93d1), 04-31 (Flag window, migration 00010, merge f7d6b49), 04-33 (phone bottom bar at 200 percent text, merge d6e3630) and 04-32 (place search near first, the picked place marked, merge f2c1993). Gate on f2c1993: build and vet clean, node 645 of 645, Go 1370 PASS top level, 0 FAIL, only TestGeocodeLive and TestScreeningFixture skipped. Checked in the iPhone simulator's own Safari with touch taps: Stop updates about this report and Flag this report as calm rows at the bottom of an opened report; the new Flag window (why question, the Dispute line under it, five tick boxes, the optional note, Send and Go back above the bottom bar). The simulator showed no on screen keyboard (a hardware keyboard is attached), so the note box with the keyboard up is still to be seen. The 8778 copy runs f2c1993 on p31_4_uat (migrated to 00010). These gaps wait for the owner's look before they close.
