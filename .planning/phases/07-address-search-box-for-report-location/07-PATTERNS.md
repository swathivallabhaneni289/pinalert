# Phase 07: Address Search Box for Report Location - Pattern Map

**Mapped:** 2026-09-29
**Files analyzed:** 6 (new/modified)
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/geocode/client.go` (new) | service | request-response (outbound proxy call, rate-limited) | `internal/ratelimit/perip.go` | role-match (rate-limiter composition), no direct outbound-HTTP-client analog exists |
| `internal/geocode/client_test.go` (new) | test | request-response | `internal/api/handlers/votes_e2e_test.go` (httptest patterns) | role-match |
| `internal/api/handlers/geocode.go` (new) | controller/handler | request-response (thin decode/call/encode) | `internal/api/handlers/votes.go` | exact (same decode -> service call -> switch-on-error -> writeJSON shape) |
| `internal/api/handlers/geocode_test.go` (new) | test | request-response | `internal/api/handlers/votes_e2e_test.go` | exact |
| `internal/api/router.go` (modified) | route registration | request-response | existing `/api/auth/request-link` registration in same file | exact |
| `web/static/js/modal.js` (modified) | component/client script | event-driven (debounced input) + request-response (fetch) | same file's `initLocation()`/`placeMarker()` + `web/static/js/votes.js`'s `castVote()` | exact |
| `web/templates/index.html.tmpl` (modified) | template markup | — | existing `.modal-panel` / `#coord-readout` / `#location-notice` markup in same file | exact |
| `web/static/css/modal.css` (modified) | style | — | existing modal.css design-token usage (`--color-*`, `--space-*`) | role-match (no existing text-input style to copy; new ground per CONTEXT.md) |

## Pattern Assignments

### `internal/geocode/client.go` (service, request-response)

**Analog:** `internal/ratelimit/perip.go` (for the rate-limiter composition pattern; no existing outbound-HTTP-client file exists in this codebase to copy transport code from — RESEARCH.md's Pattern 1/2 code examples are original compositions, not copied from an existing file)

**Package doc-comment convention** (perip.go lines 1-12):
```go
// Package ratelimit provides a reusable, self-pruning per-IP token-bucket
// limiter. It has no dependency on internal/service or internal/api so
// Phase 4's ROBUST-04 ... can reuse it unchanged.
//
// This package deliberately does not hand-roll bucket arithmetic — it wraps
// golang.org/x/time/rate.Limiter, this project's declared standard for
// exactly this problem (see .claude/CLAUDE.md's Supporting Libraries table...)
package ratelimit
```
Mirror this shape for `internal/geocode`'s package doc: state it has no dependency on `internal/service`/`internal/api` (same standalone-package precedent), and name why `rate.Limiter` is used rather than hand-rolled.

**Global limiter construction pattern** (perip.go lines 66-82, `NewPerIP`/`newPerIPWithIdleWindow`):
```go
func NewPerIP(every time.Duration, burst int) *PerIP {
	return newPerIPWithIdleWindow(every, burst, defaultIdleWindow)
}
```
`internal/geocode.Client` should follow the same "small exported constructor wrapping a `rate.Limiter`" shape — a `NewClient(contactEmail string) *Client` that constructs `rate.NewLimiter(rate.Every(time.Second), 1)` internally, matching how `PerIP` wraps `rate.NewLimiter` rather than exposing the raw limiter.

**Bounded-wait pattern (this phase's own composition, RESEARCH.md Pattern 1):**
```go
func (c *Client) Search(ctx context.Context, q string) ([]Result, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	if err := c.limiter.Wait(waitCtx); err != nil {
		return nil, fmt.Errorf("geocode: rate limiter wait: %w", err)
	}
	// build request, execute, parse
}
```
Note this differs from `PerIP.Allow`'s instant-reject `Allow()` call (perip.go line 123) — `PerIP` is deliberately instant-reject (per-IP anti-abuse), while the geocode global limiter must **wait**, not reject (RESEARCH.md Anti-Patterns: "Calling `Allow()` and instantly 429-ing on the global limiter"). Do not copy `PerIP.Allow`'s reject-on-exhaustion shape here — use `Wait(ctx)` instead.

**Fixed-endpoint outbound request pattern (RESEARCH.md Pattern 2, original composition — no existing outbound-call file in this repo to copy from):**
```go
const nominatimSearchURL = "https://nominatim.openstreetmap.org/search"

func (c *Client) buildRequest(ctx context.Context, q string) (*http.Request, error) {
	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "jsonv2")
	params.Set("limit", "5")
	if c.contactEmail != "" {
		params.Set("email", c.contactEmail)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		nominatimSearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Pinalert/1.0 (+"+c.contactEmail+")")
	return req, nil
}
```

**Response allowlisting pattern** — follow `reports.go`'s convention (see reportToResponse-style shaping referenced in RESEARCH.md) of shaping every outbound JSON response explicitly:
```go
type Result struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type nominatimRecord struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"` // arrives as a STRING on the wire — parse before use
	Lon         string `json:"lon"`
}
```

---

### `internal/api/handlers/geocode.go` (controller, request-response)

**Analog:** `internal/api/handlers/votes.go` (CastVote factory) — closest exact match: same decode -> service-call -> switch-on-sentinel-error -> writeJSON shape.

**File-level doc-comment convention** (votes.go lines 1-9):
```go
// votes.go is the HTTP surface of the confirm/dispute/resolve/reopen
// mechanic. ... Every trust decision ... is made in internal/service, never
// in this file. This file decodes the request, calls the service, and
// encodes the response, following reports.go's package-level "handlers
// decode/encode only" rule.
```
`geocode.go` should state the equivalent: this file validates `q`, calls `internal/geocode.Client.Search`, and shapes the response — no rate-limiting logic lives here (that's in `internal/geocode` and the router's `r.With(...)` middleware).

**Imports pattern** (votes.go lines 11-24):
```go
import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
)
```
Swap `pinalert/internal/service` for `pinalert/internal/geocode`; `account`/`chi` are not needed here (no path param, no account context needed for this route beyond the existing gate).

**Request-body-cap + decode pattern** (votes.go lines 26-29, 121-129) — adapt for a query-param instead of a body:
```go
// No body to cap here — q arrives as a query param. Validate length (3-200
// chars) before any outbound call, mirroring parseReportID's
// "write-the-error-yourself, return ok=false" contract (votes.go lines 65-73).
func parseQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 3 || len(q) > 200 {
		writeFieldError(w, http.StatusBadRequest, "q", "Search text must be between 3 and 200 characters.")
		return "", false
	}
	return q, true
}
```

**Error-mapping switch pattern** (votes.go lines 139-172) — same shape, fewer branches (no auth/ownership sentinels apply here, only upstream failure):
```go
res, err := geocodeClient.Search(r.Context(), q)
if err != nil {
	// Upstream timeout/failure maps to the friendly D-04 fallback, never a
	// panic/500 that would look like a real outage to the client.
	log.Printf("handlers: Geocode: %v", err)
	writeFieldError(w, http.StatusServiceUnavailable, "q", "Search unavailable, try tapping the map instead.")
	return
}
writeJSON(w, http.StatusOK, GeocodeResponse{Results: res})
```

**writeFieldError / writeJSON helpers** (reports.go lines 448-452) — reuse verbatim, already package-level in `internal/api/handlers`, do not redefine:
```go
func writeFieldError(w http.ResponseWriter, status int, field, message string) {
	writeJSON(w, status, ErrorResponse{Error: ErrorDetail{Field: field, Message: message}})
}
```

**Swagger annotation convention** (votes.go lines 80-99) — follow the same `@Summary`/`@Description`/`@Tags`/`@Param`/`@Success`/`@Failure`/`@Router` block shape for the new `GET /api/geocode` handler, consistent with this project's documented-API requirement.

---

### `internal/api/router.go` (route registration)

**Analog:** the existing `/api/auth/request-link` registration in this same file (lines 139-159) — the only precedent in this router for a route with its own dedicated `r.With(...)`-scoped per-IP limiter, and the only precedent for the "flat literal path, never nested under a wildcard `r.Route` mount" rule that this file's own doc comments repeatedly state (lines 139-146, 174-182).

**Flat-registration + scoped-limiter pattern** (router.go lines 154-159):
```go
requestLinkLimit := deps.RequestLinkRateLimit
if requestLinkLimit == (RequestLinkRateLimit{}) {
	requestLinkLimit = RequestLinkRateLimitDefault
}
requestLinkLimiter := ratelimit.NewPerIP(requestLinkLimit.Every, requestLinkLimit.Burst)
r.With(requestLinkLimiter.Middleware()).Post("/api/auth/request-link", handlers.RequestLink(deps.AuthService))
```
Register `GET /api/geocode` the same way — but **inside** the gated `r.Group` (lines 163-203), not in the exempt section, per RESEARCH.md's V4 Access Control note ("only reachable by a verified session, exactly like `/api/reports`"). Add a new `Deps` field (e.g. `Geocode *geocode.Client` and `GeocodeRateLimit GeocodeRateLimit`) mirroring the `Votes *service.VotingService` / `RequestLinkRateLimit RequestLinkRateLimit` field-addition precedent (router.go lines 36-40, 53-60, 63-73) — additive fields only, so existing `Deps{}` literals (e.g. `swagger_test.go`) keep compiling.

**Where to add the route** — inside `r.Group` (router.go line 163 onward), alongside the vote routes:
```go
r.Group(func(r chi.Router) {
	r.Use(requireVerifiedAccount(deps.Sessions))
	...
	geocodeLimiter := ratelimit.NewPerIP(deps.GeocodeRateLimit.Every, deps.GeocodeRateLimit.Burst)
	r.With(geocodeLimiter.Middleware()).Get("/api/geocode", handlers.Geocode(deps.Geocode))
	...
})
```
Per RESEARCH.md Open Question 2, use a fresh named constant (not `RequestLinkRateLimitDefault` verbatim) — e.g. `GeocodeRateLimitDefault = GeocodeRateLimit{Burst: 3, Every: 2 * time.Second}`.

---

### `web/static/js/modal.js` (component, event-driven + request-response)

**Analog:** same file's `initLocation()`/`placeMarker()`/`updateCoordReadout()` (lines 291-430) for pin-placement reuse (D-03), plus `votes.js`'s `castVote()` (lines 151-190) for the fetch/error-envelope idiom.

**Existing element-lookup convention** (modal.js lines 24-25):
```javascript
var coordReadout = document.getElementById('coord-readout');
var locationNotice = document.getElementById('location-notice');
```
Add `var locationSearchInput = document.getElementById('location-search');` and `var locationSearchDropdown = document.getElementById('location-search-results');` alongside these, same top-of-IIFE var block.

**Pin-placement reuse — call, do not reinvent** (modal.js lines 295-306):
```javascript
function placeMarker(lat, lon) {
	if (modalMarker) {
		modalMap.removeLayer(modalMarker);
	}
	modalMarker = L.marker([lat, lon], { draggable: true }).addTo(modalMap);
	modalMarker.on('dragend', function () {
		markTouched();
		var pos = modalMarker.getLatLng();
		updateCoordReadout(pos.lat, pos.lng);
	});
	updateCoordReadout(lat, lon);
}
```
On suggestion tap: call `placeMarker(result.lat, result.lon)` and `modalMap.setView([result.lat, result.lon], 16)` exactly as `initLocation()`'s GPS success callback does (lines 401-408) — do not write a second pin-placement code path.

**Fetch/error-envelope idiom to mirror** (votes.js lines 155-189, `castVote`):
```javascript
return fetch(url, { method: 'POST', headers: {...}, body: ... })
	.then(function (res) {
		return res.json().catch(function () { return {}; }).then(function (body) {
			if (!res.ok) {
				var fieldError = (body && body.error) || { field: 'body', message: GENERIC_FAILURE_MESSAGE };
				var err = new Error(fieldError.message);
				err.field = fieldError.field;
				err.fieldMessage = fieldError.message;
				throw err;
			}
			return body;
		});
	});
```
The new `runSearch()` (GET, no body) should follow the identical `res.json().catch(...).then(...)` -> `!res.ok` -> throw-with-`fieldMessage` shape, so the inline error display can reuse the same `err.fieldMessage` convention already established for votes.

**Anti-pattern to avoid — do NOT replicate this bug** (modal.js `showLocationDenied`, lines 420-430):
```javascript
function showLocationDenied(fallbackLat, fallbackLon) {
	...
	modalMap.on('click', function (e) { ... }); // registered fresh on every call — stacks handlers across repeated open/close
}
```
This function re-registers a `click` listener every time it runs, stacking duplicate handlers across repeated modal open/close cycles (a pre-existing bug, out of this phase's scope to fix). Wire the new search `input` listener **once**, at the IIFE's top level (near `fabButton`/`cancelButton`/`submitButton` wiring at the bottom of the file), not inside `initLocation()` — per RESEARCH.md's explicit warning.

**Teardown pattern to extend** (modal.js `resetForm()`, lines 432-453):
```javascript
function resetForm() {
	selectedCategory = null;
	...
	Pinalert.setText(coordReadout, '');
	hideShelterFields();
	discardConfirm.hidden = true;
	formTouched = false;
	preDiscardFocusEl = null;
}
```
Extend this function (do not add a parallel teardown path) to also: clear the search input's value, hide/empty the dropdown, clear any inline search error text, `clearTimeout` the pending debounce timer, and bump the sequence counter — per RESEARCH.md Pitfall 4.

**Text-insertion discipline** (`Pinalert.setText` used throughout, e.g. modal.js line 292, votes.js line 208) — every dropdown row's `name`/`display_name` text MUST go through `Pinalert.setText`/`textContent`, never `innerHTML` (T-01-03, RESEARCH.md Known Threat Patterns / XSS row).

---

### `web/templates/index.html.tmpl` (markup)

**Analog:** the existing `#coord-readout`/`#location-notice` markup block inside `.modal-panel` in this same file (referenced by modal.js lines 24-25). Read the surrounding markup directly when implementing to match indentation/class-naming conventions; add the new search `<input>` + dropdown `<div>` as a sibling block in the same location step, wrapped in a `position: relative` container per RESEARCH.md Pitfall 5 (z-index above Leaflet's own max, e.g. 1100).

### `web/static/css/modal.css` (style)

**Analog:** none — CONTEXT.md's own code-scout note confirms no `.text-input`-style class exists anywhere in this codebase yet (checked `index.html.tmpl` and `modal.css` directly: only a checkbox, range slider, and number input exist). This is new ground. Follow the existing design-token custom-property convention (`--color-*`, `--space-*`, `--font-*` in `main.css`) rather than introducing hardcoded values — same convention every other existing modal.css rule already follows.

## Shared Patterns

### Error response envelope
**Source:** `internal/api/handlers/reports.go` lines 448-452 (`writeFieldError`/`writeJSON`), used identically by `votes.go`
**Apply to:** `geocode.go` — every error path (validation, upstream timeout) uses the same `{error:{field,message}}` JSON shape; never a bespoke response shape for this one route.

### Per-IP rate limiting on a gated route
**Source:** `internal/ratelimit.PerIP` + its `Middleware()` (perip.go lines 136-158), instantiated per-route in `router.go` lines 154-159
**Apply to:** the new `/api/geocode` route's secondary per-IP throttle — reuse `PerIP` unchanged, construct with a fresh named constant rather than reusing `RequestLinkRateLimitDefault`.

### Flat route registration, never nested wildcard mounts
**Source:** `router.go`'s repeated doc-comment rule (lines 139-146, 174-182)
**Apply to:** `/api/geocode` registration — a literal `r.Get("/api/geocode", ...)` inside the gated `r.Group`, never `r.Route("/api/geocode/...", ...)`.

### Fetch + JSON error envelope on the client
**Source:** `web/static/js/votes.js` lines 155-189 (`castVote`)
**Apply to:** `modal.js`'s new `runSearch()` function — same `res.json().catch(...)`, `!res.ok` -> throw with `fieldMessage`, pattern.

### DOM text insertion via textContent only (T-01-03)
**Source:** `Pinalert.setText` calls throughout `modal.js`/`votes.js` (e.g. modal.js:292, votes.js:208, 215)
**Apply to:** every dropdown row rendering `name`/`display_name` from the geocode response — never `innerHTML`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/geocode/client.go` (outbound HTTP transport specifically) | service | request-response | No existing file in this codebase makes an outbound HTTP call to a third-party API — every existing service talks only to Postgres via sqlc. RESEARCH.md's Pattern 1/2 code is this research's own composition, not copied from an existing analog; planner should treat those RESEARCH.md examples as the reference implementation. |
| Search-box CSS (text input + dropdown styling) | style | — | No `.text-input`-style class exists anywhere in this codebase (confirmed directly). New ground — follow design tokens, no drop-in class to copy. |

## Metadata

**Analog search scope:** `internal/ratelimit/`, `internal/api/router.go`, `internal/api/handlers/` (votes.go, reports.go), `web/static/js/modal.js`, `web/static/js/votes.js`
**Files scanned:** 8 read directly (perip.go, router.go, votes.go partial, reports.go partial via grep, modal.js partial, votes.js partial), plus CONTEXT.md/RESEARCH.md
**Pattern extraction date:** 2026-09-29
