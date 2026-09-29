---
status: testing
phase: 07-address-search-box-for-report-location
source: [07-VERIFICATION.md]
started: 2026-09-29T07:46:34Z
updated: 2026-09-29T07:46:34Z
---

## Current Test

number: 1
name: Live debounced suggestions with cache reuse (D-01)
expected: |
  No request fires until a typing pause; exactly one /api/geocode request per pause (never one
  per keystroke); the identical repeated query produces zero new requests (served from
  searchCache).
awaiting: user response

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
result: [pending]

### 2. Suggestion tap places and centers the pin, stays draggable (D-03)
test: Type a query, tap a dropdown suggestion. Then drag the placed pin.
expected: Pin appears at the tapped place at map zoom 16, coordinate readout updates, dropdown
  closes with no intervening confirm/apply control, and the pin remains draggable afterward.
why_human: Visual map interaction requiring a real Leaflet render and pointer events; the
  contract test only proves the click handler's body calls the right functions, not that
  tapping actually produces the described visual result.
result: [pending]

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
result: [pending]

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
result: [pending]

### 5. A reopened modal shows no leftover search state
test: Type a query into the search box, close the modal before results arrive (or before
  tapping a suggestion), then reopen it.
expected: The search box is empty, with no leftover dropdown and no leftover inline status
  message.
why_human: This is the closing instruction of 07-04-PLAN.md's own human-check item 3 and
  exercises the same searchSeq/teardown mechanism covered by item 3 above. resetSearch's body is
  confirmed to contain all six required teardown calls, but per this phase's own CR-01 history,
  presence of the call is not proof the reopened modal is actually clean at runtime.
result: [pending]

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
result: [pending]

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0
blocked: 0

## Gaps
