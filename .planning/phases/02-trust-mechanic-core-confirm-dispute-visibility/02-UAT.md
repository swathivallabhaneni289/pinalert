---
status: complete
phase: 02-trust-mechanic-core-confirm-dispute-visibility
source: [02-VERIFICATION.md]
started: 2026-09-16T14:42:29Z
updated: 2026-09-22T13:15:00Z
---

## Current Test

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
result: issue
reported: "Hidden outline treatment confirmed correct on both the feed row and map pin (transparent
  badge, muted glyph, 'Disputed' label matching VISIBILITY_TAG_LABELS.hidden exactly). Toggle
  reveal/hide confirmed on both surfaces together. Critical-severity bypass confirmed (no chip, no
  treatment, always live, matches D-06). Checkbox correctly resets to unchecked on a real reload,
  matching the original spec — but the user wants this changed so reloading keeps the current
  filtered view instead. The user also wants a manual light/dark toggle added so light mode can be
  verified/used without touching OS settings. Two items were never visually confirmed either way:
  the Provisional dimmed+'Unconfirmed'-chip treatment (only checked via API response, not a
  screenshot) and the 'No disputed reports nearby' empty-state copy. Light-mode rendering of any of
  this was never checked at all (every screenshot this session was dark mode)."
severity: minor

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
result: issue
reported: "Non-reporter affirm correctly showed the 'awaiting agreement' toast and the report
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
severity: major

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
expected: A fresh non-critical report shows both a desaturated badge/border and an explicit
  "Unconfirmed" chip, on both the feed row and the map pin popup. An empty disputed-view result
  shows the specific "No disputed reports nearby" copy, not a blank list.
result: issue
reported: "I don't see the faded look but I definitely see the unconfirmed chip to it" (screenshot
  2026-09-21, dark mode, a fresh Low-severity Flood report, feed row and map popup). THE USER DOES
  NOT SEE THE FADING/DESATURATION. The required desaturated badge/border is not perceivable, so
  half of the Provisional treatment (D-09) FAILS on both surfaces, even though the Unconfirmed chip
  itself is visible and correct. Separately confirmed live in the same session: the 'No disputed
  reports nearby' empty state renders with the Show disputed reports box ticked and no live reports
  (that half of this test passes)."
severity: major
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
expected: In a real browser (both themes), tap Mark resolved on someone else's report on the feed
  row and on a map pin popup. The three primary buttons visually disappear the instant the
  confirmation appears, rather than rendering beside it.
result: issue
reported: "Confirm/Dispute now correctly disappear (that part of the fix works). But the 'Mark
  resolved' button itself (.vote-btn--resolve) is STILL visibly rendered next to the Yes/Cancel
  confirmation box instead of hiding — it stretches to match the taller confirmation box's height
  (flex align-items: stretch), rendering as an odd tall white/light rectangle with black text that
  looks visually broken against the dark theme. Both openResolveConfirm/closeResolveConfirm's JS
  selectors and the CSS both target .vote-btn--resolve identically to .vote-btn--confirm/--dispute
  (which now hide correctly), so the cause of this one button behaving differently is not yet
  understood — needs live DevTools inspection next session."
severity: major

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
expected: Check "Show disputed reports", reload the page. The box is still checked and the disputed
  view shows from the very first paint, with no visible flash of the default feed first. Copy the
  URL with the parameter set, open in a new tab, confirm it loads the disputed view directly.
result: issue
reported: "still does that" (2026-09-21), and on the follow-up question the user picked "Report shows
  under disputed": a report nobody disputed still appears when Show disputed reports is ticked. The
  user considers their answers complete.
severity: minor
notes: Same finding as the exclusive-filter gap below (backlog 999.2, approved for fixing). The
  reload-persistence, no-flash and new-tab halves of this test were NOT separately observed. Retest
  them after the exclusive-filter fix, when they become easy to judge: with a live report present
  the disputed view will be empty, so any flash of the default feed shows the report row briefly.
context: The "restored before the first fetch" guarantee rests on an argument about script
  execution order that 02-09-SUMMARY itself flags as an argument, not a proof, and names what would
  invalidate it. Static tests confirm the code shape; they cannot observe whether a real browser
  ever paints an unfiltered frame first.

### 10. 02-10 gap closure: theme toggle — no-flash, native controls, post-logout, full light-mode walkthrough
expected: Set the theme to Dark, reload — no flash of light first. Set Light, log out — the login
  gate renders light rather than snapping to a dark OS default. Toggle Dark — native checkboxes and
  the scrollbar also go dark. Walk all of Phase 2's UI once in light mode (never done — every UAT
  screenshot this phase was dark) and report anything illegible.
result: issue
reported: "for the light theme thing, even if I log out or reload, if I change the setting from dark
  mode to light mode, it changes accordingly, it works fine" (2026-09-21/22). Two things raised
  alongside that pass: (1) the 'Theme: Dark' text label 'looks really wordy', the user does not want
  a words-based toggle and asked for research into how other apps present this control. (2) 'even if
  i change it to the dark mode the map doesn't change much and same goes to when i change it to the
  light mode', the map basemap looks the same regardless of theme.
severity: minor
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

## Summary

total: 11
passed: 5
issues: 6
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "The map basemap itself visibly changes to match the app's Light/Dark theme, not just the surrounding chrome (badges, buttons, panels)."
  status: failed
  reason: "User reported (2026-09-21/22, during Test 10): 'even if i change it to the dark mode the map doesn't change much and same goes to the um when i change it to the light mode'."
  severity: minor
  test: 10
  root_cause: "By design gap, not a defect: map.js hardcodes a single MapLibre style, 'https://tiles.openfreemap.org/styles/liberty' (a light basemap), with no theme-awareness at all, and the WebGL-less fallback is the standard OpenStreetMap raster tile server (also light-only, no dark variant exists). theme.js only ever touches document.documentElement's data-theme attribute, which main.css's app-chrome rules read; nothing in map.js listens for it. RESEARCHED 2026-09-22: OpenFreeMap does publish a ready-made dark vector style at the same free, no-key endpoint pattern as the current one, confirmed live (HTTP 200): https://tiles.openfreemap.org/styles/dark, alongside liberty, bright, positron and fiord. No paid tier or new vendor needed for the vector path. The OSM raster fallback has no equivalent free dark style; that path would keep the light raster tiles even after this fix, and needs its own decision (accept the mismatch, or drop dark-theme parity for the no-WebGL fallback only)."
  artifacts:
    - path: "web/static/js/map.js"
      issue: "MapLibre style URL is a single hardcoded light style with no data-theme awareness"
  missing:
    - "Swap the MapLibre style URL between the liberty (light) and dark OpenFreeMap styles based on document.documentElement's data-theme attribute (and the system preference when data-theme is absent), re-applying on theme.js's toggle click without a full page reload if practical. Decide and document what the raster (no-WebGL) fallback does in dark mode, since OpenFreeMap has no free dark raster tiles to match."
  debug_session: ""

- truth: "With Show disputed reports ticked, the list and map show ONLY disputed reports; a fresh live or provisional report does not appear."
  status: failed
  reason: "User reported (2026-09-21, during Test 9): 'still pops up in the disputed reports when I click the show disputed reports... I still see the report, the issue I just added.' Same objection first raised 2026-09-18 (backlog 999.2). The user has now asked for this twice."
  severity: minor
  test: 9
  root_cause: "Not a defect: additive reveal is the documented design (D-10). GET /api/reports?show_disputed=true returns the normal feed plus Hidden reports (internal/api/handlers/reports.go ~line 388), and feed.js only swaps to the disputed empty state when the whole list is empty. Changing it reverses locked decision D-10 and needs the user's go-ahead. GIVEN 2026-09-21: the user asked again 'if no one disputed it why am I seeing that in the disputed area, it should not pop up there', so the exclusive filter is approved."
  artifacts:
    - path: "internal/api/handlers/reports.go"
      issue: "show_disputed=true adds Hidden reports to the default feed instead of replacing it"
    - path: "web/static/js/feed.js"
      issue: "Disputed empty state only shows when the combined list is empty"
  missing:
    - "Make show_disputed an exclusive filter: return only Hidden reports when set, and update the empty-state, toggle copy and any tests that assert the additive contract. Promotes backlog 999.2 into gap closure."
  debug_session: ""

- truth: "The app looks and feels polished: buttons, backgrounds and surfaces look intentionally designed, and interactions and transitions are smooth, not like a beginner's first web project."
  status: failed
  reason: "User reported (2026-09-21, while on the Activity page during Test 8, screenshot attached): 'the buttons and like the way the website is designed right now looks like a star beginner doing to start... I don't want it to look like that I want it to be like looking smooth transitions very smooth process I don't know these buttons and the background I don't know I think we should fix it'. Also flagged that a button label for the light/dark feature 'looks really odd' and asked for it to be fixed."
  severity: major
  test: general (raised during Test 8, not a failure of Test 8's own expectation)
  root_cause: "Not diagnosed. Broad visual-design and motion-polish scope, larger than a gap fix. Two concrete labels are named as odd: the account menu's theme control reads 'Theme: System' (account_header.html.tmpl line 12, id theme-toggle) and the Activity page's Reopen button reads 'Reopen · not actually resolved' (votes.js line 79, REOPEN_BUTTON_LABEL). The user has not said which one they meant, possibly both."
  artifacts:
    - path: "web/static/css/main.css"
      issue: "Global styling, buttons and backgrounds judged unpolished by the user; no transition/motion system observed"
    - path: "web/static/css/trust.css"
      issue: "Vote/resolve/reopen button styling judged unpolished by the user"
    - path: "web/templates/profile.html.tmpl"
      issue: "The 'Back to map' link works but its placement (top-left, above the email) and plain underlined-text styling look odd to the user (reported 2026-09-21)"
    - path: "web/templates/account_header.html.tmpl"
      issue: "CONFIRMED 2026-09-22: the 'Theme: Dark'/'Theme: Light'/'Theme: System' text label is the odd control the user meant. Their words: 'it looks really wordy and I don't like it that way'. They asked for research into how other apps and sites present a theme toggle before redesigning it (e.g. an icon-only sun/moon control cycling the same three modes, common on iOS/Android system settings and most major sites, versus a text label). Do this research as part of the wider polish pass, then replace the wordy text label with whatever pattern the research and the site-design-rules.md constraints (no emoji icons; use a real SVG glyph, matching the .auth-icon mask system already used for user.svg/log-out.svg/arrow-left.svg) land on."
    - path: "web/static/js/votes.js"
      issue: "REOPEN_BUTTON_LABEL 'Reopen · not actually resolved' named as odd (unconfirmed which control)"
  missing:
    - "A UI design pass (contract first, e.g. via /gsd-ui-phase) covering buttons, surfaces, background, hover/press states and smooth transitions, checked in both themes"
    - "Reword the odd button label(s) once the user confirms which one they meant"
    - "HARD CONSTRAINTS from the user (2026-09-21), copy into any UI-SPEC: the site must not look vibe coded. Never use purple gradients, pill-shaped buttons (user confirmed), fake reviews or fake metrics, hero text of any kind (user confirmed: 'no hero text'), emoji icons, em or en dashes in any user-visible copy, or over-the-top scroll animations. Smooth but restrained transitions only."
    - "Sweep existing user-visible copy for em dashes: <title> tags in login_gate, verify_outcome and profile templates, login_gate and check_inbox body copy, index.html.tmpl location-off notice, auth.js rate-limit error. Known pill radii: .view-toggle and #toast in main.css."
  debug_session: ""

- truth: "A fresh non-critical report's badge/border is visibly desaturated (D-09 Provisional dimming) on both the feed row and the map pin popup, in addition to the Unconfirmed chip."
  status: failed
  reason: "User reported: I don't see the faded look but I definitely see the unconfirmed chip to it. The user does not see any fading on a fresh unconfirmed report, on the feed row or the map popup, so the desaturation half of the Provisional treatment does not work for them."
  severity: major
  test: 5
  root_cause: "Likely cause (hypothesis, to be confirmed at diagnosis): .vis-provisional IS applied (screenshot badge pixels #58947c row, #4b7d6a pin, not the undimmed #34D399), but the 50/50 mix in trust.css line 128 (--severity-base with --color-age-stale) is too weak to be perceived as 'faded' for a Low-severity green in dark mode. The fix needs a much stronger, unmistakable dimming (e.g. lower opacity or a larger desaturation) checked in both themes; Test 10 covers the light-mode look."
  artifacts:
    - path: "web/static/css/trust.css"
      issue: ".vis-provisional mixes only 50% toward --color-age-stale; too weak to perceive for Low severity in dark mode"
  missing:
    - "Strengthen the provisional dimming until it is unmistakable without a side-by-side reference, then re-check in both themes"
  debug_session: ""

- truth: "Tapping 'Mark resolved' hides the .vote-btn--resolve button itself (not just Confirm/Dispute) once the inline Yes/Cancel confirmation appears, on the feed row."
  status: failed
  reason: "User reported (live, against the freshly-restarted server running today's code): Confirm/Dispute now correctly disappear, but the Mark resolved button itself stays visibly rendered next to the confirmation box, stretched tall by flexbox align-items:stretch, rendering as an odd white/black rectangle against the dark theme."
  severity: major
  test: 6
  root_cause: "Not yet found. openResolveConfirm/closeResolveConfirm in votes.js target '.vote-btn--confirm, .vote-btn--dispute, .vote-btn--resolve' identically (all three), and trust.css's .vote-btn--resolve rule declares only background/color/border-color, no display override of its own — the same shape as .vote-btn--confirm/--dispute, which now hide correctly after the 02-08 fix. Why this one button behaves differently is unexplained from static reading alone; needs live DevTools inspection (computed styles / element inspector) next session."
  artifacts:
    - path: "web/static/js/votes.js"
      issue: "openResolveConfirm's hidden=true assignment on .vote-btn--resolve does not appear to take visual effect, despite an identical code path working for .vote-btn--confirm/--dispute"
  missing:
    - "Live-inspect the resolve button's computed 'display'/'hidden' state in DevTools to find what's different about it vs. confirm/dispute"
  debug_session: ""

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
