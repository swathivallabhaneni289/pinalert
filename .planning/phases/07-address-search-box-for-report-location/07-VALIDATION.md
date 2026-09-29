---
phase: 07
slug: address-search-box-for-report-location
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-29
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `net/http/httptest` — the project's exclusive test framework (36 existing `_test.go` files; zero JS test files or JS test framework exist anywhere in this repo, confirmed by search) |
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

*Drafted before PLAN.md files exist — no Task IDs to cite yet. Using CONTEXT.md's decision IDs
(D-01..D-04) as the traceable unit, per the same convention `02-VALIDATION.md` used pre-planning.
To be backfilled with Task IDs/Plan IDs once `/gsd-plan-phase 07` produces plans.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | D-02 | — | Global limiter serializes concurrent `Search` calls to ≤1/sec | unit | `go test ./internal/geocode/... -run TestClient_Search_RateLimited -v` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-02 | — | Outbound request carries a non-stock `User-Agent` header | unit | `go test ./internal/geocode/... -run TestClient_Search_SetsUserAgent -v` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-04 | — | Handler returns the friendly fallback error (not a panic/500) on upstream timeout | unit | `go test ./internal/api/handlers/... -run TestGeocode_UpstreamTimeout -v` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | — | — | Query validation rejects <3 chars / >200 chars before any outbound call | unit | `go test ./internal/api/handlers/... -run TestGeocode_ValidatesQueryLength -v` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-01 | — | Debounce fires once per pause, not per keystroke | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |
| TBD | TBD | TBD | D-03 | — | Tapping a suggestion places/centers the pin; pin stays draggable afterward | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |
| TBD | TBD | TBD | D-04 | — | A failed/empty search never disables GPS/tap/drag or the submit button | manual (no JS harness) | — | n/a — browser UAT | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/geocode/client_test.go` — new file; needs an `httptest.NewServer` fake upstream to
  assert the `User-Agent` header, rate-limiter serialization/timeout behavior, and JSON parsing of
  the Nominatim response shape (including the string-to-float64 lat/lon conversion).
- [ ] `internal/api/handlers/geocode_test.go` — new file; needs query-length validation tests and an
  upstream-failure-maps-to-friendly-error test.
- Framework install: none — `testing`/`httptest` are stdlib, already used project-wide.

*Pre-existing, not a gap this phase introduces: no JS test framework exists in this repo at all.
D-01/D-03/D-04's client-side behaviors follow this project's established convention of covering
pure-frontend interaction via manual browser UAT at phase end (see Manual-Only Verifications
below), not a new gap to close.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| Live suggestions appear as the user types, debounced (not one request per keystroke) | D-01 | No JS test framework exists in this repo; debounce timing is a client-side browser behavior | Open the report-submission modal, type a place name slowly into the new search box, confirm suggestions appear after a pause rather than after every keystroke, and confirm the browser network panel shows one request per pause, not one per keystroke |
| Tapping a suggestion places/centers the pin and leaves it draggable, matching GPS auto-fill behavior | D-03 | Visual map interaction — not asserted by Go tests | Type a query, tap a dropdown suggestion, confirm the pin appears at that location and the map centers on it, then drag the pin and confirm it still moves |
| A failed or empty search never blocks GPS/tap/drag/submit | D-04 | Requires simulating a network failure or empty-result query in a live browser | Search a nonsense query with no matches, confirm the inline "No matches found" message appears and GPS/tap/drag/submit remain fully usable; repeat with the network disabled to confirm the "search unavailable" fallback message |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
