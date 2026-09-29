---
phase: 07-address-search-box-for-report-location
reviewed: 2026-09-29T00:00:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/server/main.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - internal/api/gate_test.go
  - internal/api/handlers/geocode.go
  - internal/api/handlers/geocode_test.go
  - internal/api/handlers/page_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/router.go
  - internal/geocode/client.go
  - internal/geocode/client_test.go
  - web/css_contract_test.go
  - web/js_contract_test.go
  - web/static/css/modal.css
  - web/static/js/modal.js
  - web/templates/index.html.tmpl
findings:
  critical: 1
  warning: 3
  info: 4
  total: 8
status: issues_found
critical_resolved: 1
---

**Post-review update (2026-09-29):** CR-01 was fixed directly (commit `589e23e`) — `searchSeq` is
now incremented unconditionally at the top of `runSearch`, before the cache check, so a cache-hit
render invalidates any older in-flight fetch the same way a new fetch would. Full suite + the
three location-search contract tests re-run green after the fix. WR-01/WR-02/WR-03 and the INFO
items remain open, not blocking phase completion.

# Phase 07: Code Review Report

**Reviewed:** 2026-09-29T00:00:00Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Reviewed the address-search-box feature end to end: the Go-side rate-limited Nominatim proxy
(`internal/geocode`), its HTTP handler and router wiring (`internal/api/handlers/geocode.go`,
`internal/api/router.go`), the generated Swagger docs, and the browser-side debounced search
UI (`web/static/js/modal.js`, `web/static/css/modal.css`, `web/templates/index.html.tmpl`), plus
every test file in scope. The server-side design is careful (rune-counted validation, a fixed
outbound URL that can never become an open proxy, a bounded rate-limiter wait, an allowlisted
response shape with tests that pin it in both runtime and published-schema form). The main
defect is client-side: a sequencing bug in the debounced search lets a slow, stale network
response overwrite a just-rendered cache hit and get tapped by the visitor, silently submitting
a location that does not match what they searched for — a correctness bug that lands directly on
this project's core "confirm/dispute must be trustworthy" data path, since the underlying report
coordinates are wrong from the moment of submission. A second, smaller defect: this phase reuses
the existing per-IP rate-limit middleware for `GET /api/geocode` without adjusting its
hardcoded response shape, so a rate-limited search shows the visitor misleading retry guidance.
The remaining findings are ARIA-role and UX-polish gaps in the new search dropdown.

## Critical Issues

### CR-01: A slow, stale geocode response can silently overwrite a just-rendered cached suggestion, letting a visitor submit the wrong location

**File:** `web/static/js/modal.js:456-491` (and the tap handler at `web/static/js/modal.js:399-405`)

**Issue:** `runSearch`'s staleness guard only protects network responses against each other — it
does nothing for the interaction between a cache hit and an in-flight network request:

```js
function runSearch(query) {
  var cacheKey = query.toLowerCase();
  if (searchCache.has(cacheKey)) {
    renderDropdown(searchCache.get(cacheKey));   // <-- does not touch searchSeq
    return;
  }

  searchSeq += 1;
  var mySeq = searchSeq;
  fetch('/api/geocode?q=' + encodeURIComponent(query))
    .then(function (res) { ... if (mySeq !== searchSeq) { return; } ... renderDropdown(body.results); })
    ...
}
```

Reproduction: visitor types query A (cache miss) → `runSearch` sets `searchSeq = N`, fetch A is
in flight. The process-wide server limiter (`internal/geocode.defaultLimiterEvery` = 1s) plus
`limiterWaitTimeout` = 1500ms mean a 1-2 second in-flight window is normal, not a rare edge case.
While A is still in flight, the visitor deletes their input and retypes query B, which happens to
already be in `searchCache` (e.g. it was searched earlier in the same session). The cache-hit
branch returns immediately and renders B's results — but never increments `searchSeq`, so `mySeq`
for the still-pending fetch A remains equal to the current `searchSeq`. When fetch A's response
lands moments later, its staleness check (`mySeq !== searchSeq`) passes, and it silently replaces
the on-screen B results with A's results.

Because `renderResultRow`'s click handler calls `placeMarker` and `modalMap.setView` immediately
with no confirmation step (by design, per this file's own D-03 comment), a visitor who taps what
they believe is a B-query suggestion — because that is what is currently rendered — can actually
be tapping an A-query result that silently replaced it. `buildPayload` then submits those
coordinates as the report's location. This is a wrong-location emergency report reaching the
backend with no error, no warning, and no way for the visitor to notice before submitting — a
direct hit on the project's core value (trustworthy, location-tagged reports).

**Fix:** Claim the sequence number before the cache short-circuits, not only in the network
branch:

```js
function runSearch(query) {
  searchSeq += 1;
  var mySeq = searchSeq;

  var cacheKey = query.toLowerCase();
  if (searchCache.has(cacheKey)) {
    renderDropdown(searchCache.get(cacheKey));
    return;
  }

  fetch('/api/geocode?q=' + encodeURIComponent(query))
    .then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok) { ... }
        if (mySeq !== searchSeq) { return; }
        searchCache.set(cacheKey, body.results);
        renderDropdown(body.results);
      });
    })
    .catch(function (err) {
      if (mySeq !== searchSeq) { return; }
      ...
    });
}
```

## Warnings

### WR-01: `GET /api/geocode`'s 429 response carries the wrong field name and a misleading retry message

**File:** `internal/api/router.go:235-240` (the reuse decision — in scope); root cause in
`internal/ratelimit/perip.go:171-178` (out of the reviewed file set, cited for context); consumed
by `web/static/js/modal.js:471-474,489`

**Issue:** `router.go` wraps `GET /api/geocode` with `ratelimit.PerIP.Middleware()` — the exact
same middleware type already used for `POST /api/auth/request-link`. That middleware's refusal
path is hardcoded for the request-link use case:

```go
// writeTooManyRequests writes the standard 429 envelope. field is "email"
// deliberately — not "ip" — matching the exact UI-SPEC string...
func writeTooManyRequests(w http.ResponseWriter) {
	...
	_, _ = w.Write([]byte(`{"error":{"field":"email","message":"` + tooManyRequestsMessage + `"}}`))
}
const tooManyRequestsMessage = "Too many requests. Try again in a minute."
```

For the geocode route this produces two problems:

- **User-visible:** `modal.js`'s `runSearch` reads `body.error.message` and displays it verbatim
  via `showSearchStatus` in the location-search status line. A rate-limited search shows "Too
  many requests. Try again in a minute." even though `GeocodeRateLimitDefault` (burst 3, refill
  every 2 seconds) means the real wait is on the order of 2 seconds, not a minute — factually
  wrong retry guidance in the middle of an emergency-reporting flow.
- **Contract-only, not currently exploitable:** the envelope also carries `field: "email"` on a
  response from an endpoint that has nothing to do with email. `runSearch`'s error path only
  reads `.message` (never `.field`); `focusField(err.field)` is wired to the separate submit-time
  fetch in `handleSubmit`, not to search. So nothing currently branches on this incorrect field
  value — but it is still a broken contract on a response shape this project otherwise takes care
  to keep consistent (`geocode.go`'s own error responses always use `field: "q"`), and a future
  change that starts trusting `error.field` for search errors would silently misbehave.

No test at any layer catches this: `geocode_test.go` calls `Geocode(stub)(rec, req)` directly,
bypassing the router and its rate-limit middleware entirely, and `gate_test.go`'s geocode test
only exercises the 401 path. The mismatched 429 shape ships with a fully green test suite.

**Fix:** Give the geocode route its own 429 body (field `"q"`, message reflecting the actual
~2-second refill) instead of reusing the request-link-specific envelope — either by making
`ratelimit.PerIP.Middleware()` accept a field/message override, or by wrapping it with a small
geocode-specific responder.

### WR-02: The search suggestion list advertises `combobox`/`listbox` ARIA semantics its DOM does not implement

**File:** `web/templates/index.html.tmpl:71-75`; `web/static/js/modal.js:383-408` (`renderResultRow`)

**Issue:** The template declares a full ARIA combobox pattern:

```html
<input type="search" id="location-search" ... role="combobox" aria-expanded="false"
       aria-autocomplete="list" aria-controls="location-search-results" ...>
<div id="location-search-results" role="listbox" aria-label="Place suggestions" hidden></div>
```

but `renderResultRow` populates that `listbox` with plain `<button>` elements — no
`role="option"`, no `aria-selected`. The combobox role plus `aria-autocomplete="list"` is a
standing promise to assistive-technology users of the WAI-ARIA combobox interaction pattern:
ArrowDown moves into the listbox, Enter selects the active option, Escape dismisses just the
list, and `aria-activedescendant` tracks the active option. None of that is implemented — the
only interaction path is a mouse/touch click on a button that happens to be in the tab order
(which is why `hideDropdown` has to empty the container's children rather than only hide it, per
its own comment about `trapTab` picking up stray buttons). A screen-reader user gets announced
semantics that contradict the actual DOM and interaction model.

**Fix:** Either implement the pattern for real (rows get `role="option"` and `aria-selected`, add
ArrowDown/ArrowUp/Enter handling on the input, track `aria-activedescendant`), or drop
`role="combobox"`/`role="listbox"`/`aria-autocomplete` and let the current tab-order-of-buttons
shape stand as an honest, simpler pattern. The second is the cheaper fix and does not regress
anything already covered by `TestLocationSearchUsesTextSinksAndExistingPinPlacement`.

### WR-03: The suggestion dropdown never closes on blur or an outside click, and can then sit over the modal map

**File:** `web/static/js/modal.js` (absence — no blur/outside-click listener is registered in
the wiring block at lines 820-843); `web/static/css/modal.css:51-56`

**Issue:** The only ways the dropdown (`#location-search-results`) closes are: a row click, a
fresh render replacing it, the query dropping below `MIN_QUERY_RUNES`, or `resetSearch` on modal
close. There is no `blur` handler on `locationSearchInput` and no document-level outside-click
handler. If a visitor opens the dropdown and then taps or tabs elsewhere in the modal without
selecting a row or clearing the field — for example moving straight to drag the pin on the map,
or tabbing to Description — the dropdown stays open and rendered. Per `modal.css`, the dropdown
is `position: absolute; top: 100%; z-index: 1100`, anchored to `.location-search-field`, which in
DOM order sits directly above `#modal-map`. A dropdown left open this way visually obstructs the
top of the modal map, interfering with exactly the tap-to-place fallback D-04 names as the
no-search-needed path.

**Fix:** Close the dropdown on `locationSearchInput` blur (with the usual short delay so an
in-progress row click still registers) and/or on the first `modalMap.on('click')`, in addition to
the existing close paths.

## Info

### IN-01: Typing in the search box makes Escape destructive instead of dismissing the dropdown

**File:** `web/static/js/modal.js:435-447` (`onSearchInput` calls `markTouched()`), `684-696`
(`onKeydown`'s `Escape` branch)

**Issue:** `onSearchInput` calls `markTouched()` on the very first keystroke. Escape's handler
checks `discardConfirm.hidden` first and, since discard-confirm is unrelated to the search
dropdown, falls through to `requestClose()`, which — because `formTouched` is already `true` from
typing — shows the "Discard this report?" prompt rather than simply dismissing the open
suggestion list. A visitor who presses Escape expecting to close the dropdown they just opened
instead gets a destructive-confirmation prompt.

**Fix:** When `!locationSearchResults.hidden`, have the `Escape` branch call `hideDropdown()` (and
`clearSearchStatus()`) and return, before falling through to the existing discard-confirm logic.

### IN-02: `geocode.Client.Search` decodes the upstream response body with no size bound

**File:** `internal/geocode/client.go:176-179`

**Issue:** `json.NewDecoder(resp.Body).Decode(&records)` has no `http.MaxBytesReader` or
`io.LimitReader` in front of it. `nominatimSearchURL` is a fixed HTTPS constant, so this is low
risk in practice and out of this review's performance/DoS scope, but it's a one-line, free
defensive addition given every other outbound edge in this file is already deliberately bounded
(request timeout, limiter wait timeout, fixed URL, fixed param set).

**Fix:** `io.LimitReader(resp.Body, someReasonableCap)` before decoding.

### IN-03: `userAgent()` concatenates operator-supplied `contactEmail` into a header value with no validation

**File:** `internal/geocode/client.go:119-124`

**Issue:** `contactEmail` (from `NOMINATIM_CONTACT_EMAIL`) is folded into the `User-Agent` string
via plain concatenation with no format check. This is operator-supplied deploy configuration, not
end-user input, and Go's HTTP transport rejects CR/LF in header field values before writing the
request, so this is not exploitable today — noted only because it's an unvalidated external input
reaching a header, should this value's provenance ever change.

### IN-04: Awkward `log.Printf` call for the nil-searcher case

**File:** `internal/api/handlers/geocode.go:109`

**Issue:** `log.Printf("handlers: Geocode: %v", "nil searcher")` uses `%v` on a string literal
where a plain message would read more naturally, e.g. `log.Printf("handlers: Geocode: nil
searcher")`. Purely stylistic.

---

_Reviewed: 2026-09-29T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
