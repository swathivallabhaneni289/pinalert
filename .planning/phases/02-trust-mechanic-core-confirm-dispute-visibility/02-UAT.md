---
status: diagnosed
phase: 02-trust-mechanic-core-confirm-dispute-visibility
source: [02-VERIFICATION.md]
started: 2026-09-16T14:42:29Z
updated: 2026-09-22T14:00:00Z
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
  status: failed
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
  status: failed
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
  status: failed
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
  status: failed
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
