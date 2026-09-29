# Phase 07: Address Search Box for Report Location - Research

**Researched:** 2026-09-29
**Domain:** OSM Nominatim geocoding integration in a Go/chi backend + no-build-step vanilla JS frontend
**Confidence:** MEDIUM (the technical facts below are HIGH confidence and directly verified; the one
open risk — Nominatim's autocomplete-usage policy versus D-01's live-typing UX — is a judgment
call this research surfaces but does not resolve, since D-01 is locked)

## Summary

Two facts, both directly verified this session, jointly decide the architecture question CONTEXT.md
left open: **build a thin Go server-side proxy in front of Nominatim; do not call it from the
browser.** First, browsers refuse to let JavaScript set the `User-Agent` header on a `fetch`/`XHR`
request — Chrome silently drops it — so a client-side-only call structurally cannot satisfy
Nominatim's usage policy requirement that "stock User-Agents as set by http libraries will not do."
Second, Nominatim's stated rate limit (1 request/second) is a *global* ceiling on this app's traffic
to the service, not a per-client one; a per-browser debounce cannot enforce that, only one
server-side choke point can. A Go proxy also gets a third, practical benefit this codebase already
leans on: it is unit-testable with `httptest` the same way every other Go package here is (36
existing `_test.go` files, zero JS test files in this repo) — a client-side-only implementation
would be the first completely untested code path in the app.

The proxy sits behind the existing verified-account gate (the report modal is already
unreachable pre-login), forwards only the `q` parameter to a fixed Nominatim URL constant (never a
user-supplied URL), and enforces the 1 req/sec ceiling with one process-wide
`golang.org/x/time/rate.Limiter`, backed by a secondary per-IP limiter (reusing the existing
`internal/ratelimit.PerIP`) so one client can't starve the shared token from everyone else. On the
client, the search box is a new, small IIFE-scoped block of code inside `modal.js` — mirroring
`placeMarker()`/`updateCoordReadout()` for pin placement and `votes.js`'s `castVote` fetch/error
pattern for the network call — with a pause-based debounce (not a compliance control; the server
enforces the real ceiling regardless of what the client does), a minimum query length, and a
same-query cache to avoid duplicate requests, all of which are Nominatim policy asks anyway.

**One risk this research surfaces but does not resolve:** Nominatim's usage policy states plainly,
"[Auto-complete search] is not yet supported by Nominatim and you must not implement such a service
on the client side using the API" [CITED: operations.osmfoundation.org/policies/nominatim/]. D-01
("live suggestions as the person types") is exactly an autocomplete UI. Moving the call server-side
resolves the *literal* violation the policy names (client-side implementation calling the API
directly) but not its spirit — the app is still, in effect, offering autocomplete against the free
shared Nominatim instance. Given this is a locked decision and a low-traffic portfolio project, the
practical risk is being rate-limited or IP-blocked under load, which D-04's existing "never blocks
submission" design already absorbs gracefully (GPS/tap/drag keep working). If the user later judges
this risk unacceptable, **Photon** (komoot, OSM-data-based, no API key, purpose-built for
autocomplete) is a drop-in swap with the same response shape family — see Alternatives Considered.
This is not a recommendation to revisit D-01; it's the escape hatch if the policy tension proves
real in practice.

**Primary recommendation:** Add a Go proxy handler (`internal/geocode` + `internal/api/handlers`)
behind the existing gate, enforcing a single global 1 req/sec limiter plus a per-IP secondary
limiter; keep the search box's client-side debounce as a UX knob (600ms, pause-based, 3-char
minimum, session-scoped result cache), reusing `modal.js`'s existing pin-placement functions and
`votes.js`'s fetch/error-envelope pattern.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Search input + debounce timing | Browser / Client | — | Pure UX timing; no server round trip needed to decide when to fire a request |
| Dropdown rendering | Browser / Client | — | DOM-only; must use `textContent`/`Pinalert.setText`, never `innerHTML` (T-01-03 codebase-wide rule) |
| Pin placement on suggestion tap | Browser / Client | — | Mirrors existing `placeMarker()`/`updateCoordReadout()` in `modal.js` exactly (D-03) |
| Geocoding query execution | API / Backend | External service (Nominatim) | Only the server can set a compliant `User-Agent`; only the server can see and throttle *all* clients' combined traffic against the 1 req/sec global ceiling |
| Rate-limit / policy compliance | API / Backend | — | Single choke point; a per-browser debounce cannot enforce a global limit |
| Response shaping (field allowlist) | API / Backend | — | Proxy returns only `{name, display_name, lat, lon}`, not Nominatim's full record (place_id, licence text, osm_id, etc.) — reduces information disclosure and keeps the client contract stable if Nominatim's schema changes |
| Graceful failure / fallback UX | Browser / Client | — | D-04: inline message only, GPS/tap/drag remain fully functional regardless of search-box state |

## Standard Stack

### Core
No new third-party dependency is required on either side.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http` | go1.26.4 (project's pinned version, `go.mod`) [VERIFIED: go.mod] | Outbound HTTP client to Nominatim + inbound proxy handler | Already the project's only HTTP client/handler mechanism; no reason to add a client library for one GET request |
| Go stdlib `net/url` | go1.26.4 | Encodes the `q` param via `url.Values.Encode()` | Prevents query-string injection; never hand-concatenate the outbound URL |
| `golang.org/x/time/rate` | v0.16.0 (already in `go.mod`) [VERIFIED: go.mod] | Process-wide 1 req/sec limiter guarding the outbound Nominatim call | Already the project's declared standard for exactly this problem (CLAUDE.md Supporting Libraries; `internal/ratelimit` already wraps it) |
| `internal/ratelimit.PerIP` (this repo) | existing | Secondary per-IP/session throttle on the inbound `/api/geocode` route | Already built, already tested, already the pattern used for `POST /api/auth/request-link` (`RequestLinkRateLimit` in `router.go`) — reuse, don't reimplement |
| Browser `window.fetch` | native | Client → proxy network call | Already the project's only network-call mechanism (`votes.js`, `auth.js`, `app.js`) — no library, no build step |

### Supporting
None beyond the above — this phase adds no new npm/Go packages.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Go proxy in front of Nominatim | Direct browser → Nominatim `fetch` | **Rejected.** Cannot set a compliant `User-Agent` from browser JS [VERIFIED: MDN forbidden-header behavior, corroborated by web search]; cannot enforce a *global* rate ceiling from N independent browser tabs; would be the only untested code path in a Go-test-only codebase. |
| Nominatim (locked by phase description/CONTEXT.md) | **Photon** (komoot, OSM-data-based) | Purpose-built for autocomplete/live search — the policy tension in the Summary does not apply to it. No API key. Not evaluated in depth here since Nominatim is already decided; recorded as the escape hatch if the autocomplete-policy risk materializes (e.g., the app's IP gets rate-limited/blocked in practice). |
| Nominatim (locked) | LocationIQ (commercial layer over Nominatim data, explicit autocomplete support) | Requires an API key — directly excluded by CLAUDE.md's "no API key" framing of this feature and the free-tier-only budget constraint. Not recommended. |
| Public `nominatim.openstreetmap.org` | Self-hosted Nominatim instance | Only worth it at real scale; adds real infrastructure cost against the project's free-tier-only budget. Not needed at portfolio-project traffic. |

**Installation:** none — no `go get` / `npm install` needed for this phase.

## Package Legitimacy Audit

Not applicable. This phase introduces zero new third-party packages on either the Go or JS side —
it only reuses `golang.org/x/time/rate` and `internal/ratelimit`, both already present in `go.mod`
and already used elsewhere in this codebase. No `npm`/`pip`/`cargo` install occurs. The Package
Legitimacy Gate protocol is skipped for this reason; nothing to audit.

## Architecture Patterns

### System Architecture Diagram

```
Browser (report-submission modal, already behind login)
  │
  │ 1. keystroke in new #location-search input
  ▼
Debounce timer (600ms pause, min 3 chars)         ── UX knob only; NOT the compliance control
  │
  │ 2. GET /api/geocode?q=<query>   (sequence number N captured in closure)
  ▼
Go handler  internal/api/handlers.Geocode          ── registered inside the EXISTING gated
  │                                                    r.Group in router.go, flat path, no
  │ 3. validate q (3-200 chars) — reject before        r.Route wildcard mount (matches every
  │    any outbound call                                other /api/* route's registration style)
  │
  │ 4. r.With(geocodeIPLimiter.Middleware())          ── secondary per-IP/session throttle,
  │    (internal/ratelimit.PerIP, reused)                anti-abuse only
  ▼
internal/geocode.Client.Search(ctx, q)
  │
  │ 5. globalLimiter.Wait(ctx)  (rate.Every(1s), burst 1)  ── THE actual policy-compliance
  │    bounded by a short ctx timeout (~1.5s)                 control: serializes ALL of this
  │                                                            app's outbound calls to Nominatim
  │                                                            to <=1/sec regardless of how many
  │                                                            users are typing concurrently
  ▼
http.Client (explicit Timeout, e.g. 5s) → GET https://nominatim.openstreetmap.org/search
  │   Headers: User-Agent: "Pinalert/1.0 (+contact from env var)"
  │   Query:   q=<value>, format=jsonv2, limit=5, email=<from env var>
  ▼
Nominatim JSON response (full record: place_id, licence, osm_id, class, type, importance, ...)
  │
  │ 6. Go handler ALLOWLISTS fields — returns only {name, display_name, lat, lon} as an array
  ▼
Browser receives slim JSON
  │
  │ 7. Discard if sequence number N is not the latest one issued (stale-response guard)
  ▼
Render dropdown (0 / 1 / N results) — name/display_name via textContent only, never innerHTML
  │
  │ 8. Tap a suggestion
  ▼
placeMarker(lat, lon) + modalMap.setView(...)     ── EXACT same functions GPS auto-fill already
  (existing modal.js functions, unchanged)             calls; pin remains draggable afterward (D-03)
```

### Recommended Project Structure
```
internal/
├── geocode/
│   ├── client.go        # Client{httpClient, limiter, baseURL, contactEmail}; Search(ctx, q) ([]Result, error)
│   └── client_test.go    # httptest fake upstream: asserts UA header, limiter serialization, timeout behavior
├── api/
│   ├── router.go          # add r.Get("/api/geocode", ...) inside the EXISTING gated r.Group
│   └── handlers/
│       └── geocode.go     # thin HTTP glue: parse q, validate length, call geocode.Client, write JSON
web/
├── static/js/
│   └── modal.js           # add search-input wiring at IIFE top level (NOT inside initLocation())
└── templates/
    └── index.html.tmpl    # add search input + dropdown markup inside .modal-panel, above/below #modal-map
```

`internal/geocode` is a standalone package with no dependency on `internal/service` — same shape as
`internal/ratelimit` — so it stays trivially unit-testable and reusable if a future phase needs
geocoding elsewhere.

### Pattern 1: Global rate limiter with bounded wait, not instant rejection
**What:** A single process-wide `rate.Limiter` (`rate.Every(1*time.Second)`, burst 1) guards the
one outbound call site to Nominatim. Two simultaneous typers must not both fail — the second
caller's request should **wait** for the next token rather than being instantly rejected.
**When to use:** Any time multiple concurrent requests share one external-API budget.
**Example:**
```go
// Source: golang.org/x/time/rate godoc (Limiter.Wait) — this project's existing standard
// per CLAUDE.md; pattern is this research's own composition, not copied from a doc example.
func (c *Client) Search(ctx context.Context, q string) ([]Result, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	if err := c.limiter.Wait(waitCtx); err != nil {
		// Deadline exceeded under contention — surfaces as the same
		// "search unavailable" path the client already handles for any
		// other geocode failure (D-04). Never panics, never blocks past
		// the bounded timeout.
		return nil, fmt.Errorf("geocode: rate limiter wait: %w", err)
	}
	// ... build request, execute, parse (below) ...
}
```

### Pattern 2: Fixed-endpoint outbound request — never a user-controlled URL
**What:** Only the `q` value is ever forwarded; the base URL is a Go constant; the query string is
built with `url.Values.Encode()`, never string concatenation.
**When to use:** Any server-side proxy to a third-party API, to avoid becoming an open proxy /
SSRF vector.
**Example:**
```go
// Source: this research's own composition, following net/url's documented Values.Encode()
// contract [CITED: pkg.go.dev/net/url].
const nominatimSearchURL = "https://nominatim.openstreetmap.org/search"

func (c *Client) buildRequest(ctx context.Context, q string) (*http.Request, error) {
	params := url.Values{}
	params.Set("q", q)              // the ONLY user-influenced value
	params.Set("format", "jsonv2")
	params.Set("limit", "5")
	if c.contactEmail != "" {
		params.Set("email", c.contactEmail) // from env var, never hardcoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		nominatimSearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	// Stock User-Agents "will not do" per policy — this is the whole reason
	// this call happens server-side and not in the browser.
	req.Header.Set("User-Agent", "Pinalert/1.0 (+"+c.contactEmail+")")
	return req, nil
}
```

### Pattern 3: Client-side debounce is a UX knob, not the compliance control
**What:** A pause-based debounce (fire only after N ms of no new keystrokes, not on every
keystroke) that exists to reduce dropdown flicker and duplicate network chatter — the *actual*
1 req/sec enforcement lives entirely in the Go proxy's global limiter (Pattern 1), so this number
can be tuned for UX feel without reasoning about policy compliance.
**When to use:** Any live-search-as-you-type input.
**Example:**
```javascript
// Source: this research's own composition. Structural idiom matches app.js's existing
// toastTimer (clearTimeout + setTimeout single-shot pattern); network idiom matches
// votes.js's castVote (fetch -> res.json().catch(() => ({})) -> res.ok check -> error
// carrying fieldMessage) — no debounce-on-input precedent exists elsewhere in this
// codebase (checked votes.js and auth.js directly; auth.js's timers are countdown
// intervals, not input debounce).
var DEBOUNCE_MS = 600;
var MIN_QUERY_LENGTH = 3;
var debounceTimer = null;
var latestSeq = 0;
var resultCache = new Map(); // session-scoped; cleared on closeModal via resetForm()

function onSearchInput() {
  markTouched();
  window.clearTimeout(debounceTimer);
  var query = searchInput.value.trim();
  if (query.length < MIN_QUERY_LENGTH) {
    hideDropdown();
    return;
  }
  debounceTimer = window.setTimeout(function () {
    runSearch(query);
  }, DEBOUNCE_MS);
}

function runSearch(query) {
  var key = query.toLowerCase();
  if (resultCache.has(key)) {
    renderDropdown(resultCache.get(key));
    return;
  }
  var mySeq = ++latestSeq; // stale-response guard
  fetch('/api/geocode?q=' + encodeURIComponent(query))
    .then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok) {
          var msg = (body && body.error && body.error.message) ||
            'Search unavailable, try tapping the map instead.';
          throw new Error(msg);
        }
        return body;
      });
    })
    .then(function (results) {
      if (mySeq !== latestSeq) { return; } // a newer query has since been typed — discard
      resultCache.set(key, results);
      renderDropdown(results);
    })
    .catch(function (err) {
      if (mySeq !== latestSeq) { return; }
      showSearchError((err && err.message) || 'Search unavailable, try tapping the map instead.');
    });
}
```

### Anti-Patterns to Avoid
- **Wiring search listeners inside `initLocation()`:** `initLocation()` runs on every `openModal()`
  call. `modal.js`'s existing `showLocationDenied()` already has a latent bug where it calls
  `modalMap.on('click', ...)` on every open, stacking duplicate handlers across repeated open/close
  cycles without ever removing the old ones. **Do not replicate this bug** for the search box — wire
  the search input's `input` listener once, at the IIFE's top level (alongside `fabButton`/
  `cancelButton`/`submitButton` at the bottom of the file), not inside `initLocation()`. Fixing the
  pre-existing `showLocationDenied()` duplication is out of this phase's scope; just don't add a
  second instance of the same class of bug.
- **Calling `Allow()` and instantly 429-ing on the global limiter:** rejects the second of two
  simultaneous typers even though a 1-second wait would have served them both. Use `Wait(ctx)` with
  a bounded timeout instead (Pattern 1).
- **Passing through Nominatim's full JSON record to the client:** unnecessary information
  disclosure (`place_id`, `osm_id`, raw `licence` string, `place_rank`, etc.) and couples the
  client's dropdown code to Nominatim's exact schema. Allowlist to `{name, display_name, lat, lon}`
  server-side.
- **Re-sorting results by `importance` client-side:** Nominatim already returns results in
  relevance order; re-sorting adds no value and risks disagreeing with Nominatim's own ranking.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Global outbound rate limiting | A homemade counter+timestamp map | `golang.org/x/time/rate.Limiter` | Already this project's declared standard (CLAUDE.md, `internal/ratelimit`'s own doc comment names this exact anti-pattern) |
| Per-IP/session request throttling on the new route | A new limiter type | `internal/ratelimit.PerIP` (existing, tested) | Already built, already the pattern used for `POST /api/auth/request-link`; reusing it costs nothing and keeps one limiter implementation in the codebase |
| Outbound URL construction | String concatenation (`baseURL + "?q=" + q`) | `net/url.Values.Encode()` | Prevents malformed/injectable query strings; stdlib, zero new dependency |

**Key insight:** every piece of infrastructure this phase needs (rate limiting, per-IP throttling,
fetch/error-envelope conventions, DOM-text-insertion discipline) already exists in this codebase in
a directly reusable form. This phase's actual new code is small: one Go package, one handler, one
route registration, and one self-contained block inside `modal.js`.

## Common Pitfalls

### Pitfall 1: Nominatim's autocomplete-usage prohibition vs. D-01's live-typing UX
**What goes wrong:** The app is built exactly as a client-side autocomplete against the free shared
Nominatim instance, which the usage policy explicitly names as unsupported/prohibited when
implemented client-side.
**Why it happens:** D-01 (locked) explicitly wants a "Google Maps"-style live dropdown.
**How to avoid:** Move the call server-side (removes the *literal* client-side violation), enforce
the real 1 req/sec ceiling with one global limiter, add a min-3-char gate and a same-query cache
(both explicitly requested by the policy), and rely on D-04's existing "never blocks submission"
design to absorb the failure mode gracefully if Nominatim rate-limits or blocks the app's traffic
under real usage. This does not make the spirit-of-the-policy tension disappear — it is documented
here as an accepted risk, with Photon named as the swap-in escape hatch if it materializes.
**Warning signs:** Repeated `429`/`403` responses from Nominatim in server logs; "Search
unavailable" showing up disproportionately often in manual testing.

### Pitfall 2: Browser JS cannot set a compliant `User-Agent`
**What goes wrong:** A developer tries `fetch(url, {headers: {'User-Agent': '...'}})` directly from
the browser, sees no error, and assumes it worked.
**Why it happens:** Browsers silently drop/ignore the header rather than throwing — there is no
error to notice [VERIFIED: web search corroborating MDN's forbidden-request-header behavior; Chrome
specifically drops it from fetch even where the spec no longer strictly forbids it].
**How to avoid:** Never attempt to set `User-Agent` from client JS for this purpose — it's a Go
proxy responsibility, full stop.
**Warning signs:** Nominatim treats the request as coming from a "stock" HTTP-library User-Agent
and may throttle/block it faster than an identified one would be.

### Pitfall 3: Stale dropdown from an out-of-order response
**What goes wrong:** User types "Mumbai", the request is slow; user then types "Chennai" and gets a
fast response; the slow "Mumbai" response arrives last and overwrites the correct "Chennai"
dropdown.
**Why it happens:** Two in-flight fetches racing, with no ordering guarantee on which resolves
first.
**How to avoid:** Capture a monotonically increasing sequence number in the closure at request
time; on response, discard if it's not the latest issued (see Pattern 3's `mySeq !== latestSeq`
check).
**Warning signs:** Dropdown flickers to a result set that doesn't match the currently-typed text.

### Pitfall 4: Stale UI surviving modal close/reopen
**What goes wrong:** `resetForm()`/`closeModal()` currently clears the marker, coord readout,
shelter fields, and discard-confirm state — but nothing yet clears a pending debounce timer or a
populated dropdown. A slow response can resolve into a dropdown in an already-closed modal, or a
reopened modal can briefly show the previous session's stale results.
**Why it happens:** New state (search input value, dropdown, debounce timer, in-flight sequence
counter) isn't part of the existing teardown path.
**How to avoid:** Extend `resetForm()` to also: clear the search input's value, hide/empty the
dropdown, clear any inline search error text, `clearTimeout` the pending debounce timer, and bump
the sequence counter so any in-flight fetch's eventual response is discarded on arrival.
**Warning signs:** Manual UAT: open modal, type a query, close before results arrive, reopen —
check for a leftover dropdown or error message from the prior session.

### Pitfall 5: Dropdown rendered underneath the Leaflet map
**What goes wrong:** Leaflet's own panes/controls use z-index up to 1000; a dropdown positioned
against `.modal-panel` (rather than a wrapper around just the search input) can end up rendered
behind `#modal-map`.
**Why it happens:** Leaflet's stacking context is easy to underestimate at a glance.
**How to avoid:** Wrap the search input in its own `position: relative` container, anchor the
dropdown to that wrapper (`position: absolute`, `top: 100%`), and give it a z-index clearly above
Leaflet's own maximum (e.g. `1100`).
**Warning signs:** Dropdown appears to render but is visually clipped by or hidden under the map.

## Code Examples

### Go: response shaping (allowlist, not passthrough)
```go
// Source: this research's own composition, following the project's existing convention
// (e.g. handlers.CastVote) of shaping every outbound JSON response explicitly rather than
// re-marshaling an upstream struct.
type Result struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// nominatimRecord is the shape actually returned by
// https://nominatim.openstreetmap.org/search?format=jsonv2 — verified live this session
// with a single, properly-identified, non-bulk request. lat/lon arrive as STRINGS, not
// numbers, and must be parsed before being handed to the client (this codebase's
// coordReadout/placeMarker functions already expect numeric lat/lon).
type nominatimRecord struct {
	Name        string `json:"name"`         // jsonv2-specific; not always populated
	DisplayName string `json:"display_name"` // always populated
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}
```
Live-verified example record (single request, `q=Bengaluru`, `format=jsonv2`, `limit=2`):
```json
{"place_id":252163534,"licence":"Data © OpenStreetMap contributors, ODbL 1.0. http://osm.org/copyright","osm_type":"relation","osm_id":7902476,"lat":"12.9767936","lon":"77.5900820","category":"boundary","type":"administrative","place_rank":14,"importance":0.6364646632219508,"addresstype":"city","name":"Bengaluru","display_name":"Bengaluru, Bangalore North, Bengaluru Urban, Karnataka, India","boundingbox":["12.8334905","13.1426196","77.4598797","77.7840639"]}
```
`[VERIFIED: live nominatim.openstreetmap.org/search call, single request, User-Agent
"Pinalert-Research/0.1" set]` — confirms `name`, `display_name`, `lat`, `lon` are the fields to
carry through, and that `lat`/`lon` are JSON strings on the wire.

### JavaScript: dropdown row (name primary, display_name secondary, textContent only)
```javascript
// Source: this research's own composition, following Pinalert.setText's existing
// textContent-only convention (T-01-03).
function renderResultRow(result) {
  var row = document.createElement('button');
  row.type = 'button';
  row.className = 'location-search-result';

  var primary = document.createElement('span');
  Pinalert.setText(primary, result.name || result.display_name);
  row.appendChild(primary);

  // name is jsonv2-specific and not always populated — when present, show the
  // remainder of display_name as a muted secondary line; when absent, fall back
  // to the full display_name as the only line (no secondary element).
  if (result.name) {
    var secondary = document.createElement('span');
    secondary.className = 'location-search-result-secondary';
    Pinalert.setText(secondary, result.display_name);
    row.appendChild(secondary);
  }

  row.addEventListener('click', function () {
    placeMarker(result.lat, result.lon);
    modalMap.setView([result.lat, result.lon], 16);
    markTouched();
    hideDropdown();
  });
  return row;
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `format=json` (legacy) | `format=jsonv2` | jsonv2 has been the documented recommended format for some time | Cleaner `name`/`addresstype` fields, consistent `licence` attribution string per record; this research's verification call and code examples use `jsonv2` throughout |

**Deprecated/outdated:** none specific to this integration — Nominatim's `/search` endpoint and
usage policy are both stable, long-lived surfaces; no recent breaking change found.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 600ms debounce + 3-char minimum is the right UX-feel starting point | Pattern 3 / Code Examples | Low — it's explicitly a tunable UX knob, not a compliance control; easy to adjust post-UAT with no architectural change |
| A2 | Adding Nominatim's optional `email=` param (sourced from an env var, not hardcoded) meaningfully reduces block risk | Pattern 2 | Low-medium — policy doesn't mandate it, but Nominatim's own docs suggest it as a good-citizen identifier alongside User-Agent; omitting it doesn't break compliance, just loses a mitigation |
| A3 | The existing Leaflet map's OpenStreetMap attribution control (already rendered in `modal.js` for both the vector and raster basemap layers) is sufficient attribution coverage for geocoding search results too, since both are the same ODbL-licensed OSM dataset and the control is visible for the modal's entire lifetime | Summary / Architecture | Low — if judged insufficient, the fix is a one-line addition to the existing attribution string, not a new UI surface |
| A4 | A global rate.Limiter with burst=1 (rather than 2+) is an adequate starting value for this project's expected concurrency (single-digit simultaneous users at portfolio-project traffic) | Pattern 1 | Low — tunable constant; a real concurrency spike would surface as queued/timed-out searches (D-04's existing failure path), not a crash |

**If this table is empty:** N/A — see above.

## Open Questions (RESOLVED)

Both questions below were resolved during `/gsd-plan-phase 07`; each question's original text is
kept for the record, with the resolution recorded under its recommendation.

1. **Does the autocomplete-usage-policy risk (Pitfall 1) actually manifest under this app's real
   traffic, or is it purely theoretical at portfolio-project scale?**
   - What we know: the policy text is unambiguous; the app's likely real-world traffic (a solo
     developer's portfolio demo) is very low.
   - What's unclear: whether Nominatim's abuse detection flags low-volume-but-shaped-like-autocomplete
     traffic, or only genuinely high-volume abuse.
   - Recommendation: ship as specified (server-side proxy, global limiter, min-length gate, cache),
     rely on D-04's graceful-degradation path if blocking occurs, and keep Photon noted as the
     documented swap-in if it does.
   - RESOLVED: shipped exactly as recommended, all four controls. The server-side proxy and the
     process-wide limiter are plan 07-01's `internal/geocode` client; the min-length gate is
     `minGeocodeQueryRunes`, mirrored client-side as `MIN_QUERY_RUNES`; the same-query cache and the
     pause-based debounce are plan 07-04's `searchCache` and `SEARCH_DEBOUNCE_MS`; and D-04's
     graceful-degradation path is the inline unavailable message wired in plan 07-04. Photon remains
     documented here as the swap-in and is deliberately not wired, so the theoretical-versus-real
     question needs no answer before shipping.

2. **Should the `/api/geocode` route's per-IP secondary limiter (Pattern 1's step 4) use a burst/refill
   pair distinct from `RequestLinkRateLimitDefault` (burst 5, 1/60s)?**
   - What we know: that existing default was tuned for the very different shape of "request a login
     link" traffic.
   - What's unclear: the right budget for "type into a search box" traffic, which is bursty within a
     single report submission but rare across a session.
   - Recommendation: planner picks a fresh, explicitly-named constant (e.g. burst 3, 1 per 2s) rather
     than reusing `RequestLinkRateLimitDefault` verbatim — different traffic shape, different budget.
   - RESOLVED: yes, a distinct pair. Plan 07-03 adds `GeocodeRateLimitDefault` in `internal/api`,
     fed by its own `geocodeRateLimitBurst` and `geocodeRateLimitEvery` constants in `cmd/server`.
     `RequestLinkRateLimitDefault` is left untouched, so the login-link budget and the search-box
     budget can be tuned independently.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `nominatim.openstreetmap.org` (external service) | Geocoding search | ✓ (verified live this session) | n/a (hosted service) | D-04: inline "Search unavailable" message; GPS/tap/drag continue to work |
| `golang.org/x/time/rate` | Global + per-IP rate limiting | ✓ | v0.16.0 (`go.mod`) | — |
| `internal/ratelimit` (this repo) | Per-IP secondary throttle | ✓ | existing, in-tree | — |
| Go stdlib `net/http`, `net/url`, `encoding/json` | Proxy handler | ✓ | go1.26.4 toolchain | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** Nominatim itself (network/service outage or rate-block) —
falls back to the inline error message per D-04; no code-level fallback provider is wired in for
this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `net/http/httptest` — the project's exclusive test framework (36 existing `_test.go` files; **zero JS test files or JS test framework exist anywhere in this repo**, confirmed by search) |
| Config file | none — driven by `go.mod`; CI config at `.github/workflows/ci.yml` |
| Quick run command | `go test ./internal/geocode/... ./internal/api/...` |
| Full suite command | `go test ./...` (against local Postgres `pinalert_test`, matching CI) |

### Phase Requirements → Test Map
This phase has no `REQUIREMENTS.md` IDs (TBD per phase description — a promoted backlog item, not
tied to a requirement). Using CONTEXT.md's decision IDs (D-01..D-04) as the traceable unit instead:

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-02 | Global limiter serializes concurrent `Search` calls to ≤1/sec | unit | `go test ./internal/geocode/... -run TestClient_Search_RateLimited -v` | ❌ Wave 0 |
| D-02 | Outbound request carries a non-stock `User-Agent` header | unit | `go test ./internal/geocode/... -run TestClient_Search_SetsUserAgent -v` | ❌ Wave 0 |
| D-04 | Handler returns the friendly fallback error (not a panic/500) on upstream timeout | unit | `go test ./internal/api/handlers/... -run TestGeocode_UpstreamTimeout -v` | ❌ Wave 0 |
| — | Query validation rejects <3 chars / >200 chars before any outbound call | unit | `go test ./internal/api/handlers/... -run TestGeocode_ValidatesQueryLength -v` | ❌ Wave 0 |
| D-01 | Debounce fires once per pause, not per keystroke | manual (no JS harness) | — | n/a — browser UAT |
| D-03 | Tapping a suggestion places/centers the pin; pin stays draggable afterward | manual (no JS harness) | — | n/a — browser UAT |
| D-04 | A failed/empty search never disables GPS/tap/drag or the submit button | manual (no JS harness) | — | n/a — browser UAT |

### Sampling Rate
- **Per task commit:** `go test ./internal/geocode/... ./internal/api/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`, plus the manual browser UAT items
  above — consistent with this project's established convention (`workflow.human_verify_mode:
  end-of-phase`, e.g. `02-UAT.md`'s precedent for GPS-denial and click-through behaviors that have
  no JS test harness in this codebase).

### Wave 0 Gaps
- [ ] `internal/geocode/client_test.go` — new file; needs an `httptest.NewServer` fake upstream to
  assert the `User-Agent` header, rate-limiter serialization/timeout behavior, and JSON parsing of
  the `nominatimRecord` shape (including the string-to-float64 lat/lon conversion).
- [ ] `internal/api/handlers/geocode_test.go` — new file; needs query-length validation tests and an
  upstream-failure-maps-to-friendly-error test.
- Framework install: none — `testing`/`httptest` are stdlib, already used project-wide.
- **Pre-existing, not a gap this phase introduces:** no JS test framework exists in this repo at
  all. D-01/D-03/D-04's client-side behaviors follow this project's established convention of
  covering pure-frontend interaction via manual browser UAT at phase end, not a new gap to close.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Route sits behind the existing verified-account session gate; no new auth surface |
| V3 Session Management | no | Reuses the existing session middleware unchanged |
| V4 Access Control | yes | `/api/geocode` registered inside the existing gated `r.Group` in `router.go` — only reachable by a verified session, exactly like `/api/reports` |
| V5 Input Validation | yes | Query param `q` validated server-side (3-200 chars) before any outbound call; encoded via `url.Values.Encode()`, never string-concatenated into the outbound URL |
| V6 Cryptography | no | No new crypto operations; outbound call is plain HTTPS to a public API, default Go TLS verification |
| V9 Communications | yes | Outbound request must use `https://` (not `http://`) to Nominatim; Go's default `http.Client` TLS verification applies unmodified — never disable cert verification |
| V13 API and Web Service | yes | New route follows this router's existing documented conventions: flat registration (no `r.Route` wildcard mount at an overlapping prefix), explicit rate limiting via `r.With(...)`, same `{error:{field,message}}` JSON envelope as every other route |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Open-proxy / SSRF via a user-controlled outbound URL | Tampering / Elevation of Privilege | Base URL is a Go constant (`nominatimSearchURL`); only the `q` value is ever forwarded, built via `url.Values.Encode()` — never accept or forward a caller-supplied URL, host, or path fragment |
| DoS against this app's own `/api/geocode` route, or amplification abuse of Nominatim via this app | Denial of Service | Two-layer limiting: process-wide global `rate.Limiter` (the actual policy-compliance control) + secondary per-IP `internal/ratelimit.PerIP` throttle on the inbound route (anti-abuse, prevents one client starving the shared token) |
| XSS via `name`/`display_name` rendered in the dropdown | Tampering | Insert via `textContent`/`Pinalert.setText` only, never `innerHTML` — matches this codebase's existing T-01-03 discipline, which already treats "the server's own message" and "third-party data" identically: the rule is about the sink, not the author |
| Information disclosure via passthrough of Nominatim's full record | Information Disclosure | Go handler allowlists the response to exactly `{name, display_name, lat, lon}` before returning to the client — never re-marshal or forward the upstream JSON verbatim |

## Additional Implementation Notes (non-optional, not covered elsewhere)

- **`validate()`'s existing location-error copy is now stale.** `modal.js`'s `validate()` currently
  returns `'Set a location by dragging the pin or allowing location access.'` when no marker is
  placed. This phase adds a third way to set a location; the planner should update this string to
  mention search (e.g. `'Set a location by searching, dragging the pin, or allowing location
  access.'`) as part of this phase's copy changes, not a follow-up.
- **`resetForm()` must be extended**, not left alone — see Pitfall 4. This is a required task in the
  plan, not an optional nicety, because a stale dropdown/error surviving a modal close/reopen is a
  visible regression, not a cosmetic one.
- **Register the new route flat, inside the existing gated `r.Group`.** `router.go`'s own doc
  comments state at length that every `/api/*` path in this router is registered as a flat literal
  path, never nested under an `r.Route("/api/...", ...)` wildcard mount, specifically because a
  literal sibling (`/api/reports`) already sits at that prefix. A plan that nests `/api/geocode`
  under a new wildcard mount will conflict with this documented convention.

## Sources

### Primary (HIGH confidence)
- `https://operations.osmfoundation.org/policies/nominatim/` — fetched and read directly this
  session: rate limit (1 req/sec, 4 req/min for sustained bulk scripts), required attribution,
  User-Agent/Referer requirement, explicit autocomplete/client-side prohibition, caching requirement,
  single-machine/single-thread bulk-geocoding restriction.
- Live verification call: `curl -H "User-Agent: Pinalert-Research/0.1 (...)"
  "https://nominatim.openstreetmap.org/search?q=Bengaluru&format=jsonv2&limit=2"` — single request,
  properly identified, confirms the actual response schema (`name`, `display_name`, `lat`, `lon` as
  strings, `importance`, `place_rank`, `licence`, etc.).
- This repository, read directly: `web/static/js/modal.js`, `web/static/js/votes.js`,
  `web/static/js/auth.js`, `web/templates/index.html.tmpl`, `web/static/css/modal.css`,
  `web/static/css/main.css`, `internal/ratelimit/perip.go`, `internal/api/router.go`, `go.mod`,
  `.github/workflows/ci.yml`.

### Secondary (MEDIUM confidence)
- `https://nominatim.org/release-docs/latest/api/Search/` — fetched for the `/search` endpoint's
  documented parameters (`format`, `addressdetails`, `limit`) and field list; cross-checked against
  the live verification call above, which matched.
- Web search corroborating that browsers treat `User-Agent` as a header client JS cannot reliably
  set on `fetch`/`XHR` (MDN forbidden-header-name behavior; Chrome-specific silent-drop behavior) —
  multiple independent sources agreed.

### Tertiary (LOW confidence)
- None used for load-bearing claims in this document.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every reused piece (`rate.Limiter`, `PerIP`) already
  exists and is already tested in this codebase.
- Architecture (proxy vs. direct call): HIGH — decided by two directly-verified technical facts
  (forbidden `User-Agent` header, global-vs-per-client rate limit), not by preference.
- Autocomplete-policy risk: MEDIUM — the policy text is unambiguous (HIGH), but how it interacts
  with this app's actual low traffic is a judgment call this research surfaces rather than resolves.
- Pitfalls: HIGH — mostly derived from direct code reading (existing `modal.js`/`votes.js` behavior)
  plus one directly-verified browser platform fact.

**Research date:** 2026-09-29
**Valid until:** ~30 days (Nominatim's policy and API surface are stable; re-check if this phase
slips past a month, since usage policies can change without notice per CONTEXT.md's own instruction
to verify rather than trust a cached note).
