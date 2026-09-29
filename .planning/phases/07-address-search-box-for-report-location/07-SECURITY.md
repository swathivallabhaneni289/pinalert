---
phase: 07
slug: address-search-box-for-report-location
status: verified
threats_open: 0
asvs_level: 1
created: 2026-09-29
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| browser to API | `GET /api/geocode?q=...` — untrusted user-typed query crosses into the server | User-typed text (3-200 runes, validated) |
| API to Nominatim | Outbound request leaves this app's trust domain to a public third-party API | The `q` value only (plus fixed `format`/`limit`/`email`) |
| Nominatim to API | The response body returning from Nominatim is untrusted third-party data | Place names, addresses, coordinates, plus fields this app must not forward (place_id, osm_id, etc.) |
| API response to DOM | `name`/`display_name` strings (OpenStreetMap contributor text) cross into the browser's DOM | Third-party place names/addresses |
| process environment to outbound header | Operator-supplied `NOMINATIM_CONTACT_EMAIL` ends up in an outbound User-Agent and query parameter | Operator's own contact address (intentional disclosure) |
| unauthenticated internet to API | `GET /api/geocode` is a new inbound surface, placed on the authenticated side of the existing verified-account gate | N/A (blocked pre-auth) |
| one client to the app's shared Nominatim budget | A single caller's request volume crosses into a resource shared by every other caller | Request rate |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01 | Tampering / Elevation of Privilege | `internal/geocode.Client.buildRequest` | high | mitigate | `nominatimSearchURL` is a package constant, the sole outbound base; only `q` is forwarded via `url.Values.Encode()`, never string concatenation — cannot become an open proxy or SSRF pivot. Verified: `nominatimSearchURL = "https://nominatim.openstreetmap.org/search"` is a const; `TestClient_Search_ForwardsOnlyQueryToFixedEndpoint` exists and passes. | closed |
| T-07-02 | Elevation of Privilege / Information Disclosure | `GET /api/geocode` registration in `internal/api/router.go` | high | mitigate | Registered inside the existing gated `r.Group` (`requireVerifiedAccount`), flat literal path, no wildcard mount. Verified: `TestAccessGateBlocksUnverifiedGeocode` exists and independently re-run against real Postgres reports PASS (401, no `results` key) — confirmed twice in this phase's verification history, not vacuous. | closed |
| T-07-03 | Tampering (stored/reflected XSS via third-party data) | `renderResultRow`/`renderDropdown` in `web/static/js/modal.js` | high | mitigate | Every string (name + full address) inserted via `Pinalert.setText` (`textContent`), never a markup-parsing sink. Verified: `TestLocationSearchUsesTextSinksAndExistingPinPlacement` exists and passes; whole-file grep for `innerHTML`/`outerHTML`/`insertAdjacentHTML`/`document.write` in `modal.js` returns 0. | closed |
| T-07-04 | Information Disclosure | `handlers.Geocode` response shaping + published `docs/swagger.json` | medium/low | mitigate | Upstream record decoded into an unexported 4-field struct; Nominatim's `place_id`/`licence`/`osm_id`/`place_rank`/`importance`/`boundingbox` structurally cannot reach the browser or the published spec. Verified: `TestGeocode_AllowlistsResponseFields` exists and passes; swagger drift guard asserts those field names are absent from `docs/swagger.json`. | closed |
| T-07-05 | Denial of Service / Input Validation | `handlers.parseGeocodeQuery` | medium | mitigate | `q` validated to 3-200 runes (`utf8.RuneCountInString`) before any outbound call — a rejected query consumes zero upstream budget. Verified: `TestGeocode_ValidatesQueryLength` and `TestGeocode_CountsQueryLengthInRunes` exist and pass. | closed |
| T-07-06 | Information Disclosure (transport) | `internal/geocode` outbound transport | medium | mitigate | `nominatimSearchURL` uses `https://`; outbound `http.Client` leaves `Transport` (and Go's default cert verification) unmodified. Verified: `TestNominatimSearchURLIsHTTPS` exists and passes. | closed |
| T-07-07 | Denial of Service (amplification of Nominatim through this app) | `internal/geocode.Client.Search` | medium | mitigate | One process-wide `rate.Limiter` (1 req/sec, burst 1) gates the single outbound call site; `Wait` bounded at 1500ms so contention degrades to D-04's inline message rather than an unbounded queue. Verified: `TestClient_Search_SerializesConcurrentCalls` and `TestClient_Search_LimiterWaitTimeout` exist and pass. | closed |
| T-07-08 | Denial of Service (inbound route + amplification abuse) | `GET /api/geocode` inbound | medium | mitigate | Per-IP token bucket (burst 3, 1 token/2s) scoped to this single route via `r.With(...)`, distinct constant from `RequestLinkRateLimitDefault`; backed by the process-wide outbound limiter regardless of caller count. Verified live: 6 rapid authenticated requests returned `200,200,200,429,429,429` during Phase 07 execution, confirming the burst-3 budget actually enforces. `geocodeLimiter`/`GeocodeRateLimitDefault` confirmed in `router.go`. | closed |
| T-07-09 | Information Disclosure (operator contact address) | `NOMINATIM_CONTACT_EMAIL` in outbound User-Agent/`email` param | low | accept | Deliberate, policy-required disclosure of the operator's own contact address to Nominatim's operators (not end users). Read from environment, never hardcoded, never returned to a browser. Accepting this is the entire point of the variable — the alternative is a stock User-Agent Nominatim's usage policy explicitly refuses. | accepted |
| T-07-10 | Tampering (wrong pin from an out-of-order response) | `runSearch` in `web/static/js/modal.js` | medium | mitigate | Monotonic `searchSeq` captured per request, compared on both success and failure paths, discarding stale responses. This is the CR-01 code-review finding: the original implementation only incremented `searchSeq` on the fetch branch, not the cache-hit branch, so a stale response could still overwrite a newer cache-hit render. Fixed in commit `589e23e` (increment moved unconditionally before the cache check). Verified at runtime, not just structurally: a scripted browser test armed a real 3-second network delay on one query, rendered a second (cached) query in the interim, then confirmed the delayed response did NOT overwrite the cached render once it arrived — and confirmed the same test correctly FAILS against the pre-fix code (methodology cross-checked both directions). | closed |
| T-07-11 | Denial of Service (self-inflicted, availability of the report flow) | `#location-search-results`/`#discard-confirm` stacking + `resetSearch` teardown | medium | mitigate | Two distinct sub-mitigations under one threat ID: (a) mandatory `:not([hidden])` CSS guards + z-index ordering so the dropdown/discard-confirm never permanently cover the map, verified by `TestLocationSearchHiddenGuards`/`TestLocationSearchDropdownStacking`; (b) `hideDropdown` empties the container's children (not just `hidden`) and `resetSearch` clears all six pieces of search state from the single `resetForm` path, verified at runtime by a scripted test that closed the modal mid-search and confirmed the reopened modal had genuinely zero leftover dropdown children, not just a hidden attribute. | closed |
| T-07-12 | Denial of Service (availability of the report flow) | `cmd/server/main.go` startup path | medium | mitigate | Warn-and-continue (never `log.Fatal`) on missing `NOMINATIM_CONTACT_EMAIL`, so a secondary convenience feature can never block boot — inverting the `SESSION_SECRET`/`RESEND_API_KEY` fail-fast pattern would violate D-04. Verified: no `log.Fatal` on the NOMINATIM_CONTACT_EMAIL path in `cmd/server/main.go`; live-confirmed the server actually boots and serves requests with the variable unset throughout this phase's UAT session. | closed |
| T-07-13 | Denial of Service (self-inflicted, request amplification from one browser) | `onSearchInput` debounce + `searchCache` | low | mitigate | Pause-based 600ms debounce, 3-rune minimum matching the server's own minimum, in-session same-query cache — all three also explicit asks in Nominatim's usage policy. These are good-citizenship controls; the enforced ceiling is the server-side process-wide limiter (T-07-07), which no client-side change can weaken. Verified at runtime: scripted test typing 10 characters with pauses produced 0 requests while typing and exactly 1 after settling; identical repeated query produced 0 new requests. | closed |
| T-07-SC | Tampering (supply chain) | package-manager installs / vendor scripts | low | accept | Phase 07 introduces zero new third-party packages on either the Go or JS side (reuses `golang.org/x/time/rate` and `internal/ratelimit`, both already in-tree). Verified: `go.mod`/`go.sum` carry no diff across this phase's commits. No `[ASSUMED]`/`[SUS]` package to verify. | accepted |

*Status: open · closed · accepted*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-09 | Operator contact email disclosed to Nominatim's operators is the intended, policy-required purpose of `NOMINATIM_CONTACT_EMAIL` — the alternative (a stock User-Agent) is explicitly refused by Nominatim's usage policy. Never disclosed to end users. | Claude (gsd-secure-phase, register authored at plan time by gsd-planner under active `workflow.security_enforcement`) | 2026-09-29 |
| AR-07-02 | T-07-SC | Zero new third-party packages introduced this phase; nothing to audit. | Claude (gsd-secure-phase) | 2026-09-29 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-29 | 14 | 12 (closed) + 2 (accepted) | 0 | Claude (gsd-secure-phase, State B — built from PLAN.md threat models authored at plan time; ASVS L1 short-circuit applied since all 14 threats classified CLOSED/accepted via direct code verification, cross-checked against this phase's independent gsd-verifier pass and live Playwright-driven runtime testing — no additional auditor subagent spawn was needed) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-29
