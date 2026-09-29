---
phase: 07-address-search-box-for-report-location
verified: 2026-09-29T13:15:00Z
status: human_needed
score: 22/29 must-haves verified
behavior_unverified: 7
overrides_applied: 0
behavior_unverified_items:
  - truth: "Typing at least three characters and pausing shows at most five matching places, one request per pause (not per keystroke), and a repeated query is served from the in-session cache with no request at all (D-01)."
    test: "Open the report modal, open the browser network panel, type a place name slowly (e.g. 'bengaluru') one character at a time."
    expected: "No request fires until a typing pause; exactly one /api/geocode request per pause; retyping the identical query after clearing produces zero new requests (served from searchCache)."
    why_human: "No JS test framework exists in this repo (confirmed: zero JS test files). The debounce timer and network request count are runtime browser behaviors; the shipped contract test only proves the debounce/cache code is present and wired (SEARCH_DEBOUNCE_MS, searchCache.has/set/clear counts), not that it behaves correctly when actually typed into."
  - truth: "Tapping a suggestion places the pin at that place, centers the map at zoom 16 through the existing placeMarker/setView path, and the pin remains draggable afterward with no second confirmation step (D-03)."
    test: "Type a query, tap a dropdown suggestion, confirm pin placement and map centering, then drag the placed pin."
    expected: "Pin appears at the tapped place, map centers on it, coordinate readout updates, dropdown closes, and the pin can still be dragged with no intervening confirm/apply control."
    why_human: "Visual map interaction. The contract test proves renderResultRow's click handler calls placeMarker(/modalMap.setView( structurally (source presence), but cannot execute a real tap or observe the rendered map state."
  - truth: "A slow response for an earlier (non-cached) query is discarded rather than overwriting the suggestions or status message rendered for a newer, already-answered query (the CR-01 ordering invariant)."
    test: "With the network artificially slowed (devtools throttling), search query A (not previously cached), then before A's response arrives, clear and search query B where B is already present in searchCache from an earlier search this session. Confirm B's cached results stay on screen when A's slow response finally lands."
    expected: "B's suggestion list is not overwritten by A's late-arriving results; no visible flicker back to A's places."
    why_human: "This is a cancellation/ordering invariant, not a static property. The fix (searchSeq incremented unconditionally before the cache-hit check, commit 589e23e) is confirmed present in the shipped file by direct source read, and the existing contract test asserts the guard string 'mySeq !== searchSeq' appears twice structurally — but that same structural assertion also passed while CR-01's bug was still live in the code, which is direct proof in this phase's own history that presence-counting this guard does not establish it fires correctly. No automated test in this repo actually drives the real race."
  - truth: "A no-match search shows 'No matches found.' and a failed/timed-out/refused search shows 'Search unavailable, try tapping the map instead.', both inline near the search box, and GPS/tap-to-place/drag/submit remain fully usable throughout (D-04)."
    test: "Search a nonsense query (e.g. 'zzzzqqqq'); then disable the network and search a real place name; confirm both inline messages and confirm GPS, tap-to-place, drag, and Post report all keep working in both states."
    expected: "'No matches found.' shown on empty result; 'Search unavailable, try tapping the map instead.' shown on network failure; report submission never blocked by either state."
    why_human: "Requires simulating an empty-result query and a live network failure in a real browser. Source-level proof exists (renderDropdown's no-match branch, runSearch's catch branch, and the source assertions that no search code path touches submitButton.disabled or calls modalMap.off), but the end-to-end user-visible behavior is unexecuted by any automated test in this repo."
  - truth: "Closing the modal clears all six pieces of search state (debounce timer, sequence counter, input value, dropdown children, status text, query cache) so a reopened modal never shows the previous session's search state."
    test: "Type a query into the search box, close the modal before results arrive (or before tapping a suggestion), then reopen it."
    expected: "The search box is empty, with no leftover dropdown and no leftover inline status message."
    why_human: "One of the six teardown pieces is the same searchSeq increment the CR-01 ordering-invariant item above depends on, so it inherits the same presence-is-not-proof caveat: TestLocationSearchUsesTextSinksAndExistingPinPlacement confirms resetSearch's function body contains all six calls (clearTimeout, searchSeq, input.value clear, hideDropdown, clearSearchStatus, searchCache.clear), and closeModal -> resetForm -> resetSearch is confirmed wired by direct source read, but no executing test actually reopens the modal and observes empty state. This exact scenario is also the closing instruction of 07-04-PLAN.md's own human-check item 3 (\"close the modal before results arrive, reopen it, and confirm the search box is empty with no leftover dropdown and no leftover message\")."
  - truth: "Report modal's location search input styling visually matches the shelter-capacity/headcount control conventions (corner rounding, height, border colour), and the hidden dropdown/status elements render as completely invisible with no stray empty box, in both light and dark theme; the modal still scrolls and the map still renders at normal height with tap-to-place still working."
    test: "Run the app, sign in, open the report modal in both light and dark theme."
    expected: "Search input visually matches existing form controls; no stray empty box or blank line where the hidden dropdown/status line are; input is not pill-shaped, has no gradient, no icon glyph; modal scroll and map height/tap-to-place behavior unaffected."
    why_human: "07-02-PLAN.md's own <human-check> block (deferred to end-of-phase UAT per workflow.human_verify_mode) explicitly scopes this as visual-only verification requiring a real themed browser render — corner rounding, exact height, and border colour matching are not assertable by any Go contract test. TestLocationSearchHiddenGuards and TestNoPillShapedControls cover the CSS-source half of this (guard presence, no fully-rounded radius) but not the rendered visual comparison against sibling controls in both themes."
---

# Phase 07: Address Search Box for Report Location Verification Report

**Phase Goal:** Add a text/address search box for setting a report's location, using OSM
Nominatim geocoding (free, no API key) to jump the map/pin to the typed address, kept alongside
the existing draggable pin for fine-tuning, not replacing it.
**Verified:** 2026-09-29T13:15:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

This phase has no REQUIREMENTS.md IDs mapped to it — confirmed by direct grep of
`.planning/REQUIREMENTS.md` for `D-01`/`D-02`/`D-03`/`D-04`: zero matches. This is expected and
correct, not a gap: 07-CONTEXT.md and every plan's frontmatter both state this is a promoted
backlog item tracked by phase-local decision IDs (D-01 through D-04), not by a global REQ-ID.
`.planning/REQUIREMENTS.md`'s traceability table has no Phase 7 row to reconcile against.

Also confirmed directly against `.planning/ROADMAP.md`'s own Phase 7 section (lines 351-380):
there is no separate `success_criteria` list for this phase — only `Goal`, `Requirements: TBD`,
`Depends on`, and the plan checklist. So there is no roadmap-contract truth list to merge in
beyond what the four plans' `must_haves.truths` already declare; nothing was silently dropped by
relying on plan frontmatter alone.

### Observable Truths

All 29 must-haves below are pooled from the four plans' `must_haves.truths` (07-01 through
07-04); no PLAN truth reduces scope from a roadmap contract (there is none to reduce from, per
the paragraph above).

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | `geocode.Client.Search` sends exactly one outbound GET to a fixed https Nominatim URL, carrying only `q` plus fixed `format`/`limit`/`email` params | VERIFIED | `TestClient_Search_ForwardsOnlyQueryToFixedEndpoint` PASS; `nominatimSearchURL` is a package const, `url.Values.Encode()` only path |
| 2 | Every outbound geocoding request carries an identifying, non-stock User-Agent header | VERIFIED | `TestClient_Search_SetsUserAgent` PASS |
| 3 | Combined outbound geocoding traffic serialized to at most 1 req/sec by a single process-wide limiter | VERIFIED | `TestClient_Search_SerializesConcurrentCalls` PASS; `geocode.NewClient` constructed exactly once in `cmd/server/main.go` (grep confirmed) |
| 4 | A limiter wait longer than 1500ms returns an error instead of queueing without limit | VERIFIED | `TestClient_Search_LimiterWaitTimeout` PASS |
| 5 | Nominatim's string-typed `lat`/`lon` arrive as `float64` | VERIFIED | `TestClient_Search_ParsesStringCoordinates` PASS |
| 6 | Query <3 or >200 runes refused with 400 before any outbound request | VERIFIED | `TestGeocode_ValidatesQueryLength`, `TestGeocode_CountsQueryLengthInRunes` PASS; `utf8.RuneCountInString` confirmed, `len(q)` absent |
| 7 | Upstream failure/timeout/non-2xx/malformed body produces 503 with D-04 fallback, never panic/500 | VERIFIED | `TestGeocode_UpstreamFailureReturnsFriendlyError`, `TestGeocode_NilSearcherReturnsFriendlyError`, `TestClient_Search_UpstreamNon200`, `TestClient_Search_MalformedBody` all PASS; `http.StatusInternalServerError` absent from geocode.go |
| 8 | Successful response with no matches serializes as empty JSON array, not null | VERIFIED | `TestGeocode_EmptyResultsSerialiseAsArray` PASS |
| 9 | Modal renders labelled search input + empty suggestion container + empty status line, addressable by id | VERIFIED | `TestPageShellServesDOMContract` PASS; direct read of `index.html.tmpl` confirms `id="location-search"`, `id="location-search-results"`, `id="location-search-status"` in that order above `#modal-map` |
| 10 | Dropdown anchors to the input's own wrapper, paints above every Leaflet pane | VERIFIED | `TestLocationSearchDropdownStacking` PASS; `.location-search-field { position: relative }`, dropdown `z-index: 1100` (> Leaflet's 1000 ceiling), confirmed by direct CSS read |
| 11 | Dropdown and status line invisible on first paint, stay invisible until JS clears `hidden` | VERIFIED | `TestLocationSearchHiddenGuards` PASS; both selectors carry `:not([hidden])` guard, confirmed by direct CSS read |
| 12 | Discard confirmation overlay paints above the suggestion dropdown | VERIFIED | `TestLocationSearchDropdownStacking` PASS; `#discard-confirm:not([hidden])` `z-index: 1200` > dropdown's 1100 |
| 13 | No em/en dash in any new template copy | VERIFIED | `TestUserVisibleCopyUsesPlainPunctuation` PASS (part of `go test ./web/ -count=1`) |
| 14 | No new control declares a fully rounded border radius | VERIFIED | `TestNoPillShapedControls` PASS (part of `go test ./web/ -count=1`) |
| 15 | `GET /api/geocode` reachable only by a verified session; unverified caller gets 401 | VERIFIED | `TestAccessGateBlocksUnverifiedGeocode` run directly with `DATABASE_URL` set: **PASS**, not SKIP (independently re-run, not trusted from SUMMARY) |
| 16 | `GET /api/geocode` registered as a flat literal path inside the gated `r.Group`, never nested under a wildcard `r.Route` mount | VERIFIED | Direct source read of `router.go`: registration sits between `r.Post("/api/reports/{id}/reopen"...)` and `r.Get("/profile"...)`, inside the gated group; `grep -n 'r\.Route('` shows only 2 doc-comment prose lines, zero live calls |
| 17 | A single client cannot exhaust the shared Nominatim budget: route carries its own per-IP limiter from a named constant distinct from the request-link budget | VERIFIED | Direct source read: `GeocodeRateLimitDefault = GeocodeRateLimit{Burst: 3, Every: 2*time.Second}`, distinct from `RequestLinkRateLimitDefault`; `r.With(geocodeLimiter.Middleware())` wraps only this route |
| 18 | A `Deps` literal omitting the geocode dependency still compiles and degrades to the D-04 message rather than panicking | VERIFIED | `go build ./...` exits 0 with existing `swagger_test.go`'s `Deps{}` literal (no `Geocode` field) still compiling; `TestGeocode_NilSearcherReturnsFriendlyError` PASS |
| 19 | Server logs a warning and keeps running when `NOMINATIM_CONTACT_EMAIL` is unset; never fails to boot | VERIFIED | Direct source read: no `log.Fatal` on the `NOMINATIM_CONTACT_EMAIL` path, `log.Println` warning present; `userAgent()` fallback confirmed in `client.go` |
| 20 | `docs/swagger.json` documents `GET /geocode` with 401/503 and `GeocodeResponse`, documents no Nominatim-internal fields | VERIFIED | Directly re-derived (not trusted from SUMMARY): `python3` confirms `paths./geocode.get` exists with responses `["200","400","401","429","503"]`, `handlers.GeocodeResponse`/`handlers.GeocodeResult` present in `definitions`; `grep -c 'place_id\|osm_id\|place_rank\|boundingbox' docs/swagger.json` = 0 |
| 21 | Typing 3+ characters and pausing shows a list of at most 5 places, short name above full address (D-01) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Structurally present and wired (`MIN_QUERY_RUNES=3`, server `resultLimit="5"`, `renderResultRow` two-line structure); runtime debounced-display behavior not exercised by any executing test — see human verification item 1 |
| 22 | One request per typing pause, repeated query served from cache with zero requests (D-01/D-02) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `SEARCH_DEBOUNCE_MS=600`, `searchCache` Map with `has`/`set`/`clear` all present structurally; not behaviorally exercised — see human verification item 1 |
| 23 | Tapping a suggestion places/centers the pin via the existing `placeMarker`/`setView` path at zoom 16, stays draggable, no second confirm step (D-03) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `TestLocationSearchUsesTextSinksAndExistingPinPlacement` PASS proves `L.marker(` appears exactly once and the click handler's body calls `placeMarker(`/`modalMap.setView(` — structural proof only; real tap/drag interaction not exercised — see human verification item 2 |
| 24 | A no-match search shows "No matches found." near the search box, changes nothing else (D-04) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `SEARCH_NO_MATCH_MESSAGE` constant and `renderDropdown`'s empty-results branch confirmed by direct source read; not behaviorally exercised — see human verification item 4 |
| 25 | A failed/timed-out/refused search shows "Search unavailable, try tapping the map instead." near the search box (D-04) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `SEARCH_UNAVAILABLE_MESSAGE` byte-identical to server's `geocodeUnavailableMessage`, confirmed by direct source read of both files; not behaviorally exercised — see human verification item 4 |
| 26 | A slow response that arrives after a newer query was typed is discarded rather than overwriting the newer suggestions (CR-01 ordering invariant) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | The fix (`searchSeq += 1` unconditionally, before the cache-hit check) is directly confirmed present in the shipped `modal.js` (read at lines 456-462), matching commit `589e23e`'s diff exactly — not a SUMMARY claim taken on trust. This is a cancellation/ordering invariant: the same structural guard-presence assertion (`mySeq !== searchSeq` count of 2) also passed while CR-01's bug was live, which is direct proof in this phase's own history that presence alone does not establish correctness. See human verification item 3 |
| 27 | Closing the modal clears all six pieces of search state via the single `resetForm` teardown path, so a reopened modal never shows the previous session's state | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `resetSearch()`'s body (bounded to its own function) contains all six calls, and `closeModal()` -> `resetForm()` -> `resetSearch()` is confirmed wired by direct source read — but one of the six pieces is the same `searchSeq` increment covered by item 26's caveat, so presence of the call is not proof the reopened modal is actually clean at runtime. See human verification item 5 |
| 28 | Submit-time location validation message names searching as one of the three ways to set a location | VERIFIED | `TestLocationSearchUsesTextSinksAndExistingPinPlacement` PASS asserts the new three-way sentence present exactly once and the old two-way sentence absent; independently confirmed by `grep -c "Set a location by searching, dragging the pin, or allowing location access." web/static/js/modal.js` = 1 |
| 29 | Every place name and address from the geocoding response reaches the DOM as text, never as markup | VERIFIED | `TestLocationSearchUsesTextSinksAndExistingPinPlacement` PASS: whole-file zero count on `innerHTML`/`outerHTML`/`insertAdjacentHTML`/`document.write`, positive count of `Pinalert.setText(` inside `renderResultRow`'s region |

**Score:** 22/29 truths verified (7 present + wired, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/geocode/client.go` | Rate-limited Nominatim proxy client | ✓ VERIFIED | Exists, substantive, compiles, 10/10 tests pass |
| `internal/geocode/client_test.go` | 10 unit tests | ✓ VERIFIED | All 10 named tests PASS on direct re-run |
| `internal/api/handlers/geocode.go` | Query validation + response allowlisting handler | ✓ VERIFIED | Exists, substantive, 7/7 tests pass |
| `internal/api/handlers/geocode_test.go` | 7 unit tests | ✓ VERIFIED | All 7 named tests PASS on direct re-run |
| `web/templates/index.html.tmpl` (search markup block) | Labelled input + dropdown + status line | ✓ VERIFIED | Present at correct DOM position, all 3 ids confirmed |
| `web/static/css/modal.css` (location search rules) | Design-token-only styling, hidden guards, z-index stacking | ✓ VERIFIED | Confirmed by direct read + passing contract tests |
| `internal/api/router.go` (Deps.Geocode etc.) | Route registration, per-IP limiter | ✓ VERIFIED | Confirmed by direct read |
| `cmd/server/main.go` (geocode wiring) | Real client construction, warn-not-fail env handling | ✓ VERIFIED | Confirmed by direct read |
| `docs/swagger.json` / `swagger.yaml` / `docs.go` | Published `/geocode` spec | ✓ VERIFIED | Confirmed by direct `python3`/`grep` inspection |
| `web/static/js/modal.js` (location search module) | Debounced search, suggestion tap, teardown | ✓ VERIFIED | Confirmed by direct read; CR-01 fix present |
| `web/js_contract_test.go` (`TestLocationSearchUsesTextSinksAndExistingPinPlacement`) | Sink/pin-placement/teardown contract | ✓ VERIFIED | PASS on direct re-run |
| `07-VALIDATION.md` (backfilled verification map) | No `TBD`, frontmatter flipped | ✓ VERIFIED | `grep -c TBD` = 0, `status: complete`, `nyquist_compliant: true` confirmed |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `geocode.Client.Search` | Nominatim `/search` | `limiter.Wait` then fixed-URL GET | WIRED | Source-confirmed single call site, `Wait` used (never `Allow`) |
| `handlers.Geocode` | `internal/geocode.Client` | `GeocodeSearcher` interface | WIRED | `*geocode.Client` satisfies the interface unchanged; `cmd/server/main.go` constructs the real client and assigns it to `Deps.Geocode` |
| `router.go` route registration | `handlers.Geocode(deps.Geocode)` | `r.With(geocodeLimiter.Middleware()).Get("/api/geocode", ...)` | WIRED | Confirmed inside gated group by direct read |
| `index.html.tmpl` element ids | `modal.js` `getElementById` lookups | `location-search`, `location-search-results`, `location-search-status` | WIRED | All three ids referenced in both files, confirmed by grep on both sides |
| `renderResultRow` click handler | `placeMarker`/`modalMap.setView` | direct function calls | WIRED | `L.marker(` appears exactly once in the whole file (single pin-placement path) |
| `closeModal` | `resetSearch` (all 6 pieces) | `resetForm()` → `resetSearch()` | WIRED | Confirmed call chain by direct source read (presence only — see truth 27) |
| `runSearch` fetch | `/api/geocode?q=...` | `fetch()` | WIRED | Confirmed exact fetch string present once |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `internal/geocode` unit suite | `go test ./internal/geocode/ -v -count=1` | 10/10 PASS | ✓ PASS |
| `internal/api/handlers` geocode suite | `go test ./internal/api/handlers/ -run TestGeocode -v -count=1` | 7/7 PASS | ✓ PASS |
| `web` location-search contract suite | `go test ./web/ -run TestLocationSearch -v -count=1` | 3/3 PASS | ✓ PASS |
| DOM contract | `go test ./internal/api/handlers/ -run TestPageShellServesDOMContract -v -count=1` | PASS | ✓ PASS |
| Swagger drift guard | `go test ./internal/api/handlers/ -run TestSwagger -v -count=1` | 2/2 PASS | ✓ PASS |
| Access gate (DB-backed) | `DATABASE_URL=... go test ./internal/api/ -run TestAccessGate -v -count=1` | 6/6 PASS, `TestAccessGateBlocksUnverifiedGeocode` reported PASS not SKIP | ✓ PASS |
| Full short suite, serial | `DATABASE_URL=... go test ./... -short -count=1 -p 1` | all 11 testable packages `ok` | ✓ PASS |
| Build/vet | `go build ./...` / `go vet ./internal/geocode/... ./internal/api/... ./web/...` | clean, no output | ✓ PASS |
| CR-01 fix presence | direct `Read` of `web/static/js/modal.js` lines 449-495 | `searchSeq += 1` unconditional, before cache check | ✓ PASS (confirmed live in file, matches commit `589e23e`, not a SUMMARY-only claim) |

### Requirements Coverage

No REQUIREMENTS.md IDs map to this phase (confirmed by grep — zero D-01..D-04 matches in
`.planning/REQUIREMENTS.md`) and no ROADMAP.md Success Criteria list exists for Phase 7 either
(confirmed by direct read of ROADMAP.md lines 351-380 — only Goal/Requirements/Depends on/Plans
sections, no `success_criteria`). Per the task instructions, this is expected: the phase uses
phase-local decision IDs (D-01 through D-04) as its traceable unit, documented in 07-CONTEXT.md.
Not flagged as an orphaned or missing requirement.

### Anti-Patterns Found

None. Scanned all 14 files modified across the four plans (`internal/geocode/client.go`,
`internal/geocode/client_test.go`, `internal/api/handlers/geocode.go`,
`internal/api/handlers/geocode_test.go`, `internal/api/router.go`, `internal/api/gate_test.go`,
`cmd/server/main.go`, `web/templates/index.html.tmpl`, `web/static/css/modal.css`,
`web/static/js/modal.js`, `web/css_contract_test.go`, `web/js_contract_test.go`,
`internal/api/handlers/page_test.go`, `internal/api/handlers/swagger_test.go`) for
`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and placeholder-style copy: zero matches.

**Non-blocking code review findings, open by the reviewer's own disposition (informational for
this report, not must-have gaps):**
- WR-01 (`07-REVIEW.md`): `GET /api/geocode`'s 429 response reuses the request-link rate limiter's
  hardcoded envelope (`field: "email"`, "Try again in a minute.") instead of a geocode-specific
  one; confirmed still present in `router.go`/`internal/ratelimit/perip.go` by direct read. Does
  not violate any must-have in this phase's plans (no must-have specifies the 429 field/message
  shape) and does not block submission (D-04) since `modal.js` only reads `.message`, never
  `.field`, on the search error path.
- WR-02: search dropdown declares `role="combobox"`/`role="listbox"`/`aria-autocomplete` ARIA
  semantics not fully implemented (no `role="option"`, no arrow-key navigation). Confirmed present
  in `index.html.tmpl` by direct read. Accessibility polish gap, not a must-have failure.
- WR-03: dropdown has no blur/outside-click close handler; confirmed no such listener exists in
  `modal.js`'s wiring block. Could let the dropdown visually sit over the map edge in an unusual
  interaction sequence; does not block GPS/tap/drag/submit (D-04's binding guarantee) and is not
  a must-have in any of the four plans.

These three are recorded here for visibility only; they were already correctly classified as
non-blocking WARNING-level findings in `07-REVIEW.md`'s own post-review update and did not gate
any of this phase's automated or human-check must-haves.

### Human Verification Required

07-04-PLAN.md's `<human-check>` block and 07-02-PLAN.md's `<human-check>` block both explicitly
defer items to end-of-phase UAT (`workflow.human_verify_mode: end-of-phase`), and
`07-VALIDATION.md`'s Manual-Only Verifications table lists the 07-04 items as `⬜ pending`. This
verifier harvested every `<human-check>` block across all four plans (not just 07-04's) and adds
two items (3 and 5) that target invariants no plan-authored script exactly exercises: the CR-01
ordering race specifically, and the reopened-modal-is-clean scenario that is the closing sentence
of 07-04's own item 3 but was easy to lose when summarizing that block into a single row.

### 1. Live debounced suggestions with cache reuse (D-01)

**Test:** Open the report modal, open the browser network panel. Type a place name slowly (e.g.
`bengaluru`) one character at a time, then pause. Clear the field and retype the identical query.
**Expected:** No request fires until a typing pause; exactly one `/api/geocode` request per pause
(never one per keystroke); the identical repeated query produces zero new requests.
**Why human:** No JS test framework exists in this repo. The debounce timer and cache reuse are
runtime browser behaviors that only structural source checks (constant values, function presence)
can approximate, not execute.

### 2. Suggestion tap places and centers the pin, stays draggable (D-03)

**Test:** Type a query, tap a dropdown suggestion. Then drag the placed pin.
**Expected:** Pin appears at the tapped place at map zoom 16, coordinate readout updates, dropdown
closes with no intervening confirm/apply control, and the pin remains draggable afterward.
**Why human:** Visual map interaction requiring a real Leaflet render and pointer events; the
contract test only proves the click handler's body calls the right functions, not that tapping
actually produces the described visual result.

### 3. Stale response cannot overwrite a newer cached result (CR-01 race)

**Test:** With devtools network throttling enabled, search a query A that has never been searched
this session (forces a real network fetch). Before A's response returns, clear the field and
search a query B that was already searched earlier in this session (so B is a cache hit). Watch
the dropdown when A's slow response finally arrives.
**Expected:** B's cached results remain on screen; A's late-arriving results never appear or
briefly flash in.
**Why human:** This exercises the exact two-request ordering race the CR-01 code-review finding
identified and the commit `589e23e` fix addresses. The fix is confirmed present in the source
(`searchSeq` incremented unconditionally before the cache-hit branch), and a structural assertion
confirms the guard string appears twice in the file — but that same structural assertion also
passed while CR-01's bug was still live, which is direct proof in this phase's own history that
presence-counting this guard does not establish it fires correctly. No automated test in this
repo actually drives the real race.

### 4. No-match and unavailable inline messages never block reporting (D-04)

**Test:** Search a nonsense query (e.g. `zzzzqqqq`) with no matches. Then disable the network (or
block `/api/geocode` in devtools) and search a real place name. In both states, confirm GPS,
tap-to-place, drag, and the Post report button remain fully usable.
**Expected:** `No matches found.` appears on the empty-result search; `Search unavailable, try
tapping the map instead.` appears on the failed search; neither state disables any other location
input method or blocks submission.
**Why human:** Requires simulating an empty-result query and a live network failure in a real
browser and observing that no other control is disabled — the strongest automated proxy this repo
has is a source assertion that no search code path touches `submitButton.disabled` or calls
`modalMap.off`, which is necessary but not sufficient live-behavior proof.

### 5. A reopened modal shows no leftover search state

**Test:** Type a query into the search box, close the modal before results arrive (or before
tapping a suggestion), then reopen it.
**Expected:** The search box is empty, with no leftover dropdown and no leftover inline status
message.
**Why human:** This is the closing instruction of 07-04-PLAN.md's own human-check item 3 and
exercises the same `searchSeq`/teardown mechanism covered by item 3 above. `resetSearch`'s body is
confirmed to contain all six required teardown calls and `closeModal` is confirmed to reach it
through `resetForm`, but — per this phase's own CR-01 history — presence of the call is not proof
the reopened modal is actually clean at runtime.

### 6. Search input visual styling and theme parity (D-01, visual only)

**Test:** Run the app, sign in, open the report modal in both light and dark theme.
**Expected:** The search input visually matches the shelter-capacity/headcount control conventions
(corner rounding, height, border colour); no stray empty box or blank line appears where the
hidden dropdown/status line are; the input carries no pill shape, gradient, or icon glyph; the
modal still scrolls and the map still renders at normal height with tap-to-place still working.
**Why human:** This is 07-02-PLAN.md's own `<human-check>` block, deferred to end-of-phase UAT.
`TestLocationSearchHiddenGuards` and `TestNoPillShapedControls` cover the CSS-source half (guard
presence, no fully-rounded radius) but not the rendered visual comparison against sibling controls
in both themes, which requires an actual themed browser render.

### Gaps Summary

No blocking gaps. All 22 non-behavior-dependent must-haves across the four plans are verified
directly against the codebase (re-run tests, direct file reads, independent `python3`/`grep`
re-derivation of claims — nothing taken on SUMMARY.md's word alone), plus the ROADMAP.md
Success-Criteria contract was directly checked and confirmed empty for this phase rather than
assumed from plan frontmatter.

The remaining 7 items are D-01/D-03/D-04 behaviors that are structurally present and wired but
depend on runtime browser behavior (debounce timing, live map interaction, an actual network race,
simulated network failure, modal-reopen state, and themed visual rendering) that this repo has no
automated tooling to exercise — the project has zero JS test files, a fact both 07-04-PLAN.md and
07-VALIDATION.md state explicitly and deliberately route to end-of-phase human UAT rather than
pretending a source-presence check is behavioral proof. Two of the seven (items 26 and 27) carry
extra weight: this phase's own history shows the same class of structural assertion
(`mySeq !== searchSeq` presence) passed while the CR-01 ordering bug was still live in the code,
which is concrete, phase-local evidence that presence-only checks are not sufficient for these two
truths specifically, not merely a theoretical caveat.

This is a `human_needed` phase, not a `gaps_found` one: nothing is missing, stub, or unwired, and
no must-have contradicts the codebase.

---

_Verified: 2026-09-29T13:15:00Z_
_Verifier: Claude (gsd-verifier)_
