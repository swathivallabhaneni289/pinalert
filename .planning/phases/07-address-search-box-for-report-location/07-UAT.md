---
status: complete
phase: 07-address-search-box-for-report-location
source: [07-VERIFICATION.md]
started: 2026-09-29T07:46:34Z
updated: 2026-09-29T09:05:00Z
---

## Current Test

[testing complete]

**Testing method:** the user opened the app in Safari and hit friction with DevTools; rather than
keep fighting Safari's inspector, Claude drove the remaining verification directly using a
scripted Playwright browser (Chromium + WebKit engines) against the actual running dev server,
with a real magic-link-authenticated session, real Nominatim network calls, and DOM/network
assertions in place of human eyeballing. The user's own manual pass through Test 1 in Safari
surfaced a real visual bug (see Gaps) that the scripted checks alone had not caught yet at that
point — confirming both methods were genuinely exercising the shipped code, not a mock.

## Tests

### 1. Live debounced suggestions with cache reuse (D-01)
test: Open the report modal, open the browser network panel. Type a place name slowly (e.g.
  "bengaluru") one character at a time, then pause. Clear the field and retype the identical
  query.
expected: No request fires until a typing pause; exactly one /api/geocode request per pause
  (never one per keystroke); the identical repeated query produces zero new requests.
why_human: No JS test framework exists in this repo. The debounce timer and cache reuse are
  runtime browser behaviors that only structural source checks (constant values, function
  presence) can approximate, not execute.
result: pass
evidence: |
  Scripted Playwright run (Chromium) typed "vijayawada" one character at a time, 150ms apart,
  with network-request tracing attached. 0 requests fired while typing; exactly 1 request fired
  ~1.2s after the last keystroke (past the 600ms debounce). Clearing and retyping the identical
  query produced 0 new requests (served from searchCache). While the user manually drove this
  same test in Safari, they found a real layout bug in the result dropdown, logged and fixed
  below -- the debounce/cache mechanism itself is confirmed correct.

### 2. Suggestion tap places and centers the pin, stays draggable (D-03)
test: Type a query, tap a dropdown suggestion. Then drag the placed pin.
expected: Pin appears at the tapped place at map zoom 16, coordinate readout updates, dropdown
  closes with no intervening confirm/apply control, and the pin remains draggable afterward.
why_human: Visual map interaction requiring a real Leaflet render and pointer events; the
  contract test only proves the click handler's body calls the right functions, not that
  tapping actually produces the described visual result.
result: pass
evidence: |
  Scripted Playwright run: searched "vijayawada", tapped the first suggestion. Coordinate
  readout changed to the tapped place, dropdown closed immediately (no confirm step),
  discard-confirm dialog never appeared, and a subsequent manual mouse-drag on the marker
  changed the coordinates again -- confirming it stayed draggable after placement.

### 3. Stale response cannot overwrite a newer cached result (CR-01 race)
test: With devtools network throttling enabled, search a query A that has never been searched
  this session (forces a real network fetch). Before A's response returns, clear the field and
  search a query B that was already searched earlier in this session (so B is a cache hit).
  Watch the dropdown when A's slow response finally arrives.
expected: B's cached results remain on screen; A's late-arriving results never appear or
  briefly flash in.
why_human: This exercises the exact two-request ordering race the CR-01 code-review finding
  identified and commit 589e23e's fix addresses. The fix is confirmed present in the source, and
  a structural assertion confirms the guard string appears twice in the file, but that same
  structural assertion also passed while CR-01's bug was still live — direct proof in this
  phase's own history that presence-counting this guard does not establish it fires correctly.
result: pass
evidence: |
  Scripted Playwright run with real network-delay injection (Playwright route interception):
  cached "chennai" results, then armed a 3-second artificial delay on the outbound
  /api/geocode?q=mumbai request, typed "mumbai" (fetch fires and is now in flight), then
  immediately cleared and retyped "chennai" (a cache hit, renders instantly). Waited for
  mumbai's delayed response to actually land. Chennai's results were still on screen unchanged
  -- mumbai's stale response was correctly discarded. Test methodology cross-checked: the same
  script was run against modal.js with the CR-01 fix manually reverted first, and it correctly
  failed (mumbai's results silently overwrote chennai's), confirming the test is meaningful and
  not a false positive, before being re-run against the real shipped fix (commit 589e23e).

### 4. No-match and unavailable inline messages never block reporting (D-04)
test: Search a nonsense query (e.g. "zzzzqqqq") with no matches. Then disable the network (or
  block /api/geocode in devtools) and search a real place name. In both states, confirm GPS,
  tap-to-place, drag, and the Post report button remain fully usable.
expected: "No matches found." appears on the empty-result search; "Search unavailable, try
  tapping the map instead." appears on the failed search; neither state disables any other
  location input method or blocks submission.
why_human: Requires simulating an empty-result query and a live network failure in a real
  browser and observing that no other control is disabled — the strongest automated proxy this
  repo has is a source assertion that no search code path touches submitButton.disabled or calls
  modalMap.off, which is necessary but not sufficient live-behavior proof.
result: pass
evidence: |
  Scripted Playwright run: searched "zzzzqqqq" (no matches) -> status line read exactly "No
  matches found.", submit button remained enabled, tap-to-place on the map still updated
  coordinates. Then used Playwright's network interception to force every /api/geocode request
  to fail, searched "chennai" -> status line read exactly "Search unavailable, try tapping the
  map instead." (byte-identical to the server's own fallback message), submit button remained
  enabled and clickable, the map marker remained visible/interactive throughout.

### 5. A reopened modal shows no leftover search state
test: Type a query into the search box, close the modal before results arrive (or before
  tapping a suggestion), then reopen it.
expected: The search box is empty, with no leftover dropdown and no leftover inline status
  message.
why_human: This is the closing instruction of 07-04-PLAN.md's own human-check item 3 and
  exercises the same searchSeq/teardown mechanism covered by item 3 above. resetSearch's body is
  confirmed to contain all six required teardown calls, but per this phase's own CR-01 history,
  presence of the call is not proof the reopened modal is actually clean at runtime.
result: pass
evidence: |
  Scripted Playwright run: typed "vijay" into the search box, closed the modal via Cancel
  before the debounced results arrived (this correctly triggered the discard-confirmation
  overlay since the form was touched; confirmed discard), then reopened the modal. Search box
  value was empty, #location-search-results had 0 child elements (not just hidden -- genuinely
  cleared), and the status line was hidden. Confirms resetSearch's teardown actually fires at
  runtime, not just that the call is present in source.

### 6. Search input visual styling and theme parity (D-01, visual only)
test: Run the app, sign in, open the report modal in both light and dark theme.
expected: The search input visually matches the shelter-capacity/headcount control conventions
  (corner rounding, height, border colour); no stray empty box or blank line appears where the
  hidden dropdown/status line are; the input carries no pill shape, gradient, or icon glyph; the
  modal still scrolls and the map still renders at normal height with tap-to-place still
  working.
why_human: This is 07-02-PLAN.md's own <human-check> block, deferred to end-of-phase UAT.
  TestLocationSearchHiddenGuards and TestNoPillShapedControls cover the CSS-source half but not
  the rendered visual comparison against sibling controls in both themes.
result: pass
evidence: |
  Scripted Playwright run + screenshots in both themes. Search input's computed borderRadius
  (6px) and borderColor exactly matched the shelter-headcount input's computed values in both
  light and dark theme -- confirmed visually consistent with existing form controls, not a
  pill shape, no gradient, no icon glyph (screenshots reviewed directly). Hidden
  dropdown/status elements measured height:0 / display:none in both themes -- no stray box.
  Modal map height stayed at 220px and tap-to-place kept updating coordinates in both themes.
  Note: this UAT run used the WebGL-less raster basemap fallback (headless Chromium had no
  WebGL2 context), a pre-existing, explicitly accepted limitation from plan 02-11, unrelated to
  this phase.

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

issues_found_and_fixed_during_session: 1

## Gaps

None outstanding. One issue was found and fixed during this UAT session (not left as an open
gap):

- truth: "The suggestion dropdown renders each result's primary name and secondary address
    cleanly stacked, with no overlapping or garbled text, regardless of how many results or how
    long their addresses are."
  status: fixed
  reason: "User reported (screenshot): 'it's getting like a clumsy... the half of the box is
    like cut off... I barely saw it' while manually running Test 1 in Safari. Root cause
    diagnosed via a scripted Playwright repro: `.location-search-result` had no `flex-shrink`
    override, so its implicit `flex-shrink: 1` let flexbox shrink every row down to its
    `min-height: 44px` floor whenever the summed natural height of all results exceeded
    `#location-search-results`' 200px `max-height` budget (reproduced exactly with a 5-result
    'vijay' search, ~376px of natural content vs. a 200px budget). Since `overflow: visible` is
    set on the row (needed so the secondary line can wrap), the squeezed content painted
    straight through the box and bled into the next row."
  severity: major
  test: 1
  root_cause: "Missing `flex-shrink: 0` on `.location-search-result` in web/static/css/modal.css"
  artifacts:
    - path: "web/static/css/modal.css"
      issue: "`.location-search-result` had implicit flex-shrink:1, letting rows compress below
        their content's natural height inside the scrollable dropdown"
  missing: []
  fix_commit: "0384819"
  verification: "offsetHeight now matches scrollHeight for every row (confirmed via scripted
    DOM measurement before/after); full test suite green; visually confirmed via screenshot
    with the exact reproducing query."
  debug_session: ""
