---
status: testing
phase: 02-trust-mechanic-core-confirm-dispute-visibility
source: [02-VERIFICATION.md]
started: 2026-09-16T14:42:29Z
updated: 2026-09-18T20:15:00Z
---

## Current Test

number: 4
name: D-18 GPS-denial hard block — DevTools/sessionStorage/map-popup re-confirmation
expected: |
  Denying the location prompt sends zero network requests (verified in the Network panel, not just
  by visible outcome) and shows the exact GPS-denial copy; a granted prompt is cached per session
  (not re-prompted same-tab, re-prompted new-tab); the map popup behaves identically to the feed row.
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
result: [pending]
context: Carried forward from this same UAT's Test 1, which passed on the core behavior but never
  independently re-confirmed these three specifics live after the Location Services root cause was
  fixed. Unresolved by any of the three gap-closure plans (none touch votes.js's GPS-transport code).

### 5. Provisional dimming/"Unconfirmed" chip and the "No disputed reports nearby" empty state
expected: A fresh non-critical report shows both a desaturated badge/border and an explicit
  "Unconfirmed" chip, on both the feed row and the map pin popup. An empty disputed-view result
  shows the specific "No disputed reports nearby" copy, not a blank list.
result: [pending]
context: Carried forward from this same UAT's Test 2 — never visually confirmed either way (only
  checked via API response, not a screenshot). Unresolved by any gap-closure plan (none touch
  visibility.js or the badge/chip rendering path).

### 6. 02-08 gap closure: Mark Resolved button row is replaced, not doubled up
expected: In a real browser (both themes), tap Mark resolved on someone else's report on the feed
  row and on a map pin popup. The three primary buttons visually disappear the instant the
  confirmation appears, rather than rendering beside it.
result: [pending]
context: Fix is a CSS selector-head guard (:not([hidden])), proven present by static parsing and by
  the two touch-target contract tests that previously failed to catch its absence. Static CSS
  parsing cannot prove a real browser's rendered layout actually stops painting the three buttons —
  this repo shipped exactly that class of bug once already from a test suite that stayed green
  throughout.

### 7. 02-08 gap closure: map popup "Mark this report resolved?" heading legibility
expected: Open a map pin popup in dark mode and tap Mark resolved. The heading text is clearly
  legible against the popup's own background, at contrast comparable to the buttons beside it.
  Repeat in light mode and confirm nothing regressed (the fix's own falsifiable prediction is that
  the pre-fix bug never reproduced in light mode).
result: [pending]
context: TestMapPopupSurfaceIsThemeAware proves the override's declarations exist, resolve through
  the correct CSS custom properties, and clear 4.5:1 WCAG contrast arithmetically — it does not
  composite the page in a real browser. The plan's own SUMMARY explicitly leaves the light-mode
  falsifiability check and the live dark-mode legibility check to this step.

### 8. 02-09 gap closure: Safari Back-after-Reopen refetch
expected: Resolve a report, open Activity, tap Reopen, press the browser's Back button (Safari
  first, then one Chromium browser). The feed shows the reopened report live immediately with no
  manual reload, and the Network panel shows a fresh GET /api/reports fired on the restore.
result: [pending]
context: The pageshow/event.persisted fix is proven wired and correctly ordered by static
  inspection only. The pre-fix Safari bfcache symptom this fix targets was never independently
  reproduced live in the original UAT session — the diagnosis is the most likely explanation, not a
  confirmed one.

### 9. 02-09 gap closure: disputed-filter reload persistence, no unfiltered flash
expected: Check "Show disputed reports", reload the page. The box is still checked and the disputed
  view shows from the very first paint, with no visible flash of the default feed first. Copy the
  URL with the parameter set, open in a new tab, confirm it loads the disputed view directly.
result: [pending]
context: The "restored before the first fetch" guarantee rests on an argument about script
  execution order that 02-09-SUMMARY itself flags as an argument, not a proof, and names what would
  invalidate it. Static tests confirm the code shape; they cannot observe whether a real browser
  ever paints an unfiltered frame first.

### 10. 02-10 gap closure: theme toggle — no-flash, native controls, post-logout, full light-mode walkthrough
expected: Set the theme to Dark, reload — no flash of light first. Set Light, log out — the login
  gate renders light rather than snapping to a dark OS default. Toggle Dark — native checkboxes and
  the scrollbar also go dark. Walk all of Phase 2's UI once in light mode (never done — every UAT
  screenshot this phase was dark) and report anything illegible.
result: [pending]
context: Presence of a non-deferred head script and a color-scheme CSS declaration is provable by
  static inspection (done, both pass); a real paint-flash timing effect and real rendered legibility
  across a theme never once visually inspected in this project cannot be. 02-10-SUMMARY states
  explicitly this walkthrough has not been performed.

### 11. 02-10 gap closure: Activity "Back to map" link, live click-through
expected: From the Activity page, click "Back to map" and confirm it lands on the main feed/map
  view.
result: [pending]
context: Structurally verified (link present, contract test passes) but not separately re-run live
  by the phase verifier.

## Summary

total: 11
passed: 1
issues: 2
pending: 8
skipped: 0
blocked: 0

## Gaps

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
