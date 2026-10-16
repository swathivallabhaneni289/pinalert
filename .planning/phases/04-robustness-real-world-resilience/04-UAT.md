---
status: testing
phase: 04-robustness-real-world-resilience
source: [04-VERIFICATION.md, 04-27-SUMMARY.md, 04-REVIEW-SERVER.md, 04-REVIEW-WEB.md]
started: 2026-10-16T18:40:00+05:30
updated: 2026-10-16T18:40:00+05:30
---

## Current Test

number: 1
name: SC7, the bar links Disclaimer only
expected: |
  The bar at the top of every page ends with one link, Disclaimer. Policy is one tap away on the Disclaimer page, in the account menu and on the login page.
awaiting: user response

## Tests

Owner questions first (each has a recommended answer that stands unless the owner says otherwise), then one viewing session on laptop and phone, then the checks only a person can do. Automated proof so far: the full Go suite green on a fresh database after every merge (876 tests at 5f1ee0f, the one skip waits for item 12), two deep code reviews (server and web) with every warning fixed, a skeptic check of those fixes (its findings fixed in round 3), the phase check in 04-VERIFICATION.md, and a browser pass in Chromium and WebKit (its rerun on the fixed code is pending).

### 1. SC7, the bar links Disclaimer only
expected: The bar sentence stays as the owner approved it, ending with Disclaimer. Recommended: keep it, so the bar stays short on a phone; Policy is one tap away. Keeping it also records an override of roadmap success criterion 7, which said both pages are linked from the bar.
result: [pending]

### 2. The network limit
expected: Only once CLIENT_IP_HEADER is set on the real host. One network address (many phones on one mobile network can share it) gets its own allowances, on top of each account's limits: posts 2000 at once then 1 a second, votes 2000 then about 7 a second, flags 1000 then 1 every 1.2 seconds. A refusal reads "A lot of requests are coming from your network right now. You can try again in {wait}." Recommended: keep. Trade-off: someone with many accounts on one network can post more before being stopped, and a flood could use up the abuse check's quota, which keeps new reports held until a moderator acts.
result: [pending]

### 3. Low data page size
expected: The Low data page downloads about 509 KB (about 153 KB compressed) against the planned 256 KB. Decide after the browser check on a slow 3G connection, which is still to run on the fixed code.
result: [pending]

### 4. The gibberish rule counts letters only (04-02, D-05)
expected: The "random characters" refusal looks only at runs of letters, so ten exclamation marks, a row of emoji or a phone number are never refused as gibberish. Recommended: keep.
result: [pending]

### 5. Six defaults chosen during planning (04-CONTEXT)
expected: (a) Flag stays the last control of every Low data row. (b) The vote limits cover all five vote routes; the take back is no longer refused by the 30 second wait, so Undo works (fixed 2026-10-16). (c) A held or removed report drops off other people's My votes list. (d) On the Past tab and on report links, Flag doesn't know you already flagged until you press it. (e) The moderator screen shows Expired or Resolved for reports older than 7 days. (f) "Written 4 hours ago" sits on the same line on the compact row. Recommended: keep all six.
result: [pending]

### 6. Your own held or removed post on Activity (04-16)
expected: A post of yours that is held or removed counts under the "Not resolved" tab on Activity, and its card says Held or Removed. Recommended: keep, because it is still your open post.
result: [pending]

### 7. Disclaimer: the state helplines line and the Sachet link
expected: Checked 2026-10-16 against government sites: 112, 100, 101, 108 and 1070 match; 1077 is the district line in some states only (Andhra Pradesh uses 1070 for both). The Disclaimer page's link to NDMA's Sachet help desk lists some numbers that differ from the states' own sites (one looks a digit short). Recommended: say "in many states" for 1070 and 1077, and replace the Sachet link with a sentence telling people their state disaster authority's website lists local numbers.
result: [pending]

### 8. Read the new messages once
expected: The new messages are in the 04-UI-SPEC copy table (rows added 2026-10-16): the network limit message, "saved under a different account", "can't read the saved reports", the messages after closing the form while it was sending, and "No exact match for the full address". Recommended: keep as written.
result: [pending]

### 9. Small behaviour notes
expected: Accept unless you disagree. (a) A late removal by the automatic check shows the author "Removed after review", the same as a moderator's; the Policy page now says so. (b) Closing the report form while it is sending closes at once and a message tells the outcome; edits typed after pressing Post are dropped. (c) A report whose check record could not be saved twice stays held with no automatic recovery; moderators can still find it by number. (d) The abuse check's quota errors are not retried. (e) The developer API page (/swagger) has no unofficial bar. (f) A moderator's Keep or Remove and its audit record are now saved together.
result: [pending]

### 10. Viewing session: Phase 4 screens
expected: On a test copy you open on your laptop and phone: the unofficial bar in light and dark; the round account button at the top right of the phone (kept by Phase 4; the 3.2 bell will want that corner too); the phone following its own light or dark setting with no button; the Low data page; the Flag button and dialog; the moderator screen; the "waiting to send" strip on Activity; the Held and Removed labels on Activity (no outline on a phone, outlined on a laptop). Say what to move. Phase 3.1 items 3 and 10 are looked at in the same session.
result: [pending]

### 11. Viewing session: address search with typos (quick tasks 261016-adr and 261016-pho)
expected: Type a close spelling or half a name, for example "Banjra Hills" or "Banjara Hi": the right place shows first, inside India and near you. A long address that isn't on the map shows the nearest road and says so.
result: [pending]

### 12. Record the moderation scores
expected: With your OpenAI key, from the repository: `read -rs OPENAI_API_KEY`, then `MODERATION_LIVE=1 OPENAI_API_KEY="$OPENAI_API_KEY" go run ./cmd/modrecord`, then `unset OPENAI_API_KEY`. Afterwards TestScreeningFixture passes instead of skipping. Full steps: 04-27-SUMMARY.md, "Owner checks" item 1.
result: [pending]

### 13. Real phone on a weak network
expected: On your phone with a weak signal or the slowest network setting: the Low data page loads and you can post and confirm; offline, a report is saved and sends once when you're back online; at the largest text size nothing overlaps; VoiceOver or TalkBack reads the bar, the limit messages and the waiting strip.
result: [pending]

### 14. 112.gov.in opens
expected: The research could not open 112.gov.in from here. Open it in a browser in India and check it still calls 112 the single emergency number.
result: [pending]

### 15. Before any wide promotion (not needed before the push)
expected: A lawyer or mentor reads the Disclaimer and Policy pages. Set CONTACT_EMAIL so both pages say how to complain or ask for removal. Once a host is chosen, send the CLIENT_IP_HEADER canary (04-27-SUMMARY.md item 2). Optional: a fluent reader skims the Hindi, Telugu and Tamil sample texts.
result: [pending]

## Summary

total: 15
passed: 0
issues: 0
pending: 15
skipped: 0
blocked: 0

## Gaps
