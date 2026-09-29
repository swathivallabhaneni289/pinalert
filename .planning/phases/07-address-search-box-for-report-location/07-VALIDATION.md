---
phase: 07
slug: address-search-box-for-report-location
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-29
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `net/http/httptest` — the project's exclusive test framework (zero JS test files or JS test framework exist anywhere in this repo, confirmed by search) |
| **Config file** | none — driven by `go.mod`; CI config at `.github/workflows/ci.yml` |
| **Quick run command** | `go test ./internal/geocode/... ./internal/api/...` |
| **Full suite command** | `go test ./...` (against local Postgres `pinalert_test`, matching CI) |
| **Estimated runtime** | ~30-60 seconds (full suite, single worker against shared Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/geocode/... ./internal/api/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green, plus the manual browser UAT items below
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

Backfilled after all four plans of this phase executed. Every automated row below was proven, not
trusted: each command was run by name with `-v` and with `DATABASE_URL` exported, and its output
was confirmed to contain a `--- PASS: <name>` line for the exact test named in its `-run` pattern.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| Task 1 | 07-01 | 1 | D-02 | T-07-07 | Global limiter serializes concurrent `Search` calls to at most 1/sec | unit | `go test ./internal/geocode/ -run '^TestClient_Search_SerializesConcurrentCalls$' -v` | ✅ | ✅ green |
| Task 1 | 07-01 | 1 | D-02 | — | Outbound request carries a non-stock `User-Agent` header | unit | `go test ./internal/geocode/ -run '^TestClient_Search_SetsUserAgent$' -v` | ✅ | ✅ green |
| Task 2 | 07-01 | 1 | D-04 | — | Handler returns the friendly fallback error (not a panic or a 500) on upstream failure | unit | `go test ./internal/api/handlers/ -run '^TestGeocode_UpstreamFailureReturnsFriendlyError$' -v` | ✅ | ✅ green |
| Task 2 | 07-01 | 1 | D-04 | T-07-05 | Query validation rejects fewer than 3 or more than 200 runes before any outbound call | unit | `go test ./internal/api/handlers/ -run '^TestGeocode_ValidatesQueryLength$' -v` | ✅ | ✅ green |
| Task 2 | 07-01 | 1 | D-04 | T-07-04 | A successful response allowlists to exactly name, display_name, lat, lon per result | unit | `go test ./internal/api/handlers/ -run '^TestGeocode_AllowlistsResponseFields$' -v` | ✅ | ✅ green |
| Task 2 | 07-01 | 1 | D-04 | — | An empty result set serialises as a non-null array | unit | `go test ./internal/api/handlers/ -run '^TestGeocode_EmptyResultsSerialiseAsArray$' -v` | ✅ | ✅ green |
| Task 2 | 07-01 | 1 | D-04 | — | The response sets a private Cache-Control header | unit | `go test ./internal/api/handlers/ -run '^TestGeocode_SetsPrivateCacheControl$' -v` | ✅ | ✅ green |
| Task 1 | 07-02 | 1 | D-01 | — | Report modal renders the labelled search input, suggestion container and status line, all addressable by id | unit | `go test ./internal/api/handlers/ -run '^TestPageShellServesDOMContract$' -v` | ✅ | ✅ green |
| Task 2 | 07-02 | 1 | D-04 | T-07-11 | Suggestion dropdown and inline status line stay invisible on first paint until JS clears hidden | unit | `go test ./web/ -run '^TestLocationSearchHiddenGuards$' -v` | ✅ | ✅ green |
| Task 2 | 07-02 | 1 | D-04 | T-07-11 | Suggestion dropdown anchors above every Leaflet pane and below the discard confirmation overlay | unit | `go test ./web/ -run '^TestLocationSearchDropdownStacking$' -v` | ✅ | ✅ green |
| Task 1 | 07-03 | 2 | D-04 | T-07-02 | GET /api/geocode is reachable only by a verified session; an unverified caller gets 401 with no results key. Skips without DATABASE_URL, but ran and passed with it exported for this backfill | unit | `go test ./internal/api/ -run '^TestAccessGateBlocksUnverifiedGeocode$' -v` | ✅ | ✅ green |
| Task 3 | 07-03 | 2 | D-04 | T-07-04 | Published spec documents /geocode's 401/503 responses and GeocodeResponse, and never a Nominatim-internal record field | unit | `go test ./internal/api/handlers/ -run '^TestSwaggerSpecCoversRoutes$' -v` | ✅ | ✅ green |
| Task 3 | 07-04 | 3 | D-01, D-03, D-04 | T-07-03 | Every place name and address reaches the DOM through Pinalert.setText, never a markup-parsing sink; exactly one pin-placement code path (L.marker() appears once); the input listener is wired once outside initLocation; the submit-time location message names all three ways to set a location | unit | `go test ./web/ -run '^TestLocationSearchUsesTextSinksAndExistingPinPlacement$' -v` | ✅ | ✅ green |
| Task 2 | 07-04 | 3 | D-01 | — | Debounce fires once per typing pause, not per keystroke, and a repeated query is served from the in-session cache with no request at all | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |
| Task 1 | 07-04 | 3 | D-03 | — | Tapping a suggestion places and centres the pin through the existing pin-placement path; the pin stays draggable afterward with no extra confirmation step | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |
| Task 2 | 07-04 | 3 | D-04 | — | A failed, empty, slow, or unavailable search never disables GPS, tap-to-place, drag, or the submit button | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

All 13 automated rows above were run by name with `-v` and `DATABASE_URL` exported, and each
produced a `--- PASS: <name>` line for the exact test its `-run` pattern names, including
`TestAccessGateBlocksUnverifiedGeocode`, which reported PASS (not SKIP) because a real
`DATABASE_URL` was available for this backfill.

---

## Wave 0 Requirements

- [x] `internal/geocode/client_test.go`: 10 tests against an `httptest.NewServer` fake upstream,
  asserting the `User-Agent` header, rate-limiter serialization/timeout behavior, and JSON parsing
  of the Nominatim response shape (including the string-to-float64 lat/lon conversion).
- [x] `internal/api/handlers/geocode_test.go`: 7 tests covering query-length validation and an
  upstream-failure-maps-to-friendly-error test.
- Framework install: none. `testing`/`httptest` are stdlib, already used project-wide.

*Pre-existing, not a gap this phase introduces: no JS test framework exists in this repo at all.
D-01/D-03/D-04's client-side behaviors follow this project's established convention of covering
pure-frontend interaction via manual browser UAT at phase end (see Manual-Only Verifications
below), not a new gap to close. The sink and pin-placement structural properties those same
decisions depend on are additionally locked in by
`TestLocationSearchUsesTextSinksAndExistingPinPlacement`, a Go test reading the embedded
`modal.js` bytes.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| Live suggestions appear as the user types, debounced (not one request per keystroke) | D-01 | No JS test framework exists in this repo; debounce timing is a client-side browser behavior | Open the report-submission modal, type a place name slowly into the new search box, confirm suggestions appear after a pause rather than after every keystroke, and confirm the browser network panel shows one request per pause, not one per keystroke |
| Tapping a suggestion places/centers the pin and leaves it draggable, matching GPS auto-fill behavior | D-03 | Visual map interaction — not asserted by Go tests | Type a query, tap a dropdown suggestion, confirm the pin appears at that location and the map centers on it, then drag the pin and confirm it still moves |
| A failed or empty search never blocks GPS/tap/drag/submit | D-04 | Requires simulating a network failure or empty-result query in a live browser | Search a nonsense query with no matches, confirm the inline "No matches found." message appears and GPS/tap/drag/submit remain fully usable; repeat with the network disabled to confirm the "Search unavailable, try tapping the map instead." fallback message |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies. Every task across all four
  plans (2 + 2 + 3 + 3 = 10 tasks) carries exactly one `<automated>` verify block, confirmed by
  grep against each `PLAN.md`.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify. Every task has one.
- [x] Wave 0 covers all MISSING references. Both files listed under Wave 0 Requirements exist and
  their tests pass.
- [x] No watch-mode flags. No `-watch` flag appears in any command this map cites.
- [x] Feedback latency < 60s. Every automated command above completed in under 3 seconds in
  practice, well inside the 60 second ceiling.
- [x] `nyquist_compliant` set to the affirmative value in frontmatter. Every task across all four
  plans carried an `<automated>` verify that actually ran, confirmed against the four
  `-SUMMARY.md` files and by direct grep against each `PLAN.md`.

**Approval:** all waves (1 through 3) validated. The three D-01/D-03/D-04 manual rows remain
pending, deferred to `/gsd-verify-work 7` per `workflow.human_verify_mode: end-of-phase`; every
automated row is green.
