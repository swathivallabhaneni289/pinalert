---
quick_id: 261016-adr
created: 2026-10-16
source: owner request during Phase 3.1 UAT item 9, promotes backlog 999.7
status: planned
---

# Address search: India only, near you first, honest about partial matches

## Why

During Phase 3.1 UAT (item 9, 2026-10-16) the owner said, by voice: let the person type their whole
address, find it close to them, point at exactly what they typed, and make sure it is correct. Today
both searches send the text to Nominatim worldwide: a name that also exists abroad can come first
(Hyderabad in Pakistan above Hyderabad in India, "Ring Road" returned roads in Hanoi), and a long
Indian address often finds nothing because Nominatim needs every word to exist (flat numbers and
building names are rarely in OpenStreetMap). Backlog 999.7 already describes the fix.

## What changes

Two searches share `internal/geocode/client.go`: the report form's address search
(`GET /api/geocode`, `Client.Search`, used by `web/static/js/report-form.js`) and the reader's place
search over the map (`GET /api/geocode/place`, `Client.SearchReader`, used by
`web/static/js/map-tools.js`).

1. India only: every upstream request sends `countrycodes=in`.
2. Near first: when the caller knows a position (the report form's current pin or the reader's
   position, the place search's existing `near` point), look inside a box around it first
   (`viewbox` with `bounded=1`) and widen (no box) only when the box finds nothing.
   `GET /api/geocode` gains optional `lat` and `lon` query parameters for this, validated like the
   place search's; without them the search behaves as now plus `countrycodes=in`.
3. Fewer words: when the whole text finds nothing, try again without the most specific part (the
   first comma separated part, or the first word when there are no commas), at most twice, and stop
   at the first try that finds something.
4. Honest answer: when a shorter text was used, the JSON answer carries the text that matched
   (`matched_query`, present only when it differs from what was typed), and both result lists show
   one plain line above the results, for example: "No exact match for the full address. Showing
   places for: Banjara Hills, Hyderabad." Each result keeps showing its full address, and choosing a
   result moves the pin or the map exactly as now.
5. Nominatim usage policy stays met: the process wide limiter (one request a second) is waited on
   for every upstream request, the number of upstream requests per search is capped (at most four,
   including the widening and the shorter tries), the final answer is cached under the original text
   and area, and a search that runs out of limiter time answers the way a busy search answers today.

## Out of scope

Tapping the map to move the report pin (backlog 999.6). Shading or reverse geocoding. Any change to
the report pin's precision rules.

## Done when

- Client tests with an `httptest` stand in for Nominatim prove: `countrycodes=in` on every request;
  box first then widened; the shorter tries and their order; the request cap; the cache key; the
  limiter wait on every request.
- Handler tests prove `lat`/`lon` validation on `/api/geocode` and `matched_query` in both answers.
- Contract tests pin the new line's text in both scripts and that it is written as text, never markup.
- `swag` docs regenerated; `go build`, `go vet`, the full suite on a fresh database, the node tests and
  `go test ./web/` all pass; no em or en dash added; plain words in the new line.
