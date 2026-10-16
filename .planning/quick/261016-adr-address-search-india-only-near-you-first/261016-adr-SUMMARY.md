---
quick_id: 261016-adr
completed: 2026-10-16
status: complete
source: owner request during Phase 3.1 UAT item 9, promotes backlog 999.7
---

# Quick 261016-adr: address search India only, near you first, honest about partial matches

Both searches (the report form's `GET /api/geocode` and the reader's `GET /api/geocode/place`) now
search India only. When a position is known they look in a box around it first. When the whole
text finds nothing they retry with fewer words. When a shorter text was used they say so in one
plain line above the results.

## What changed

1. **India only.** `buildRequest` in `internal/geocode/client.go` always sends `countrycodes=in`.
   It is a fixed parameter there, so no caller can leave it out.
2. **Near first.** With a position, each text is tried first inside a box of 0.5 degrees each way
   (about 55 km), using `viewbox` with `bounded=1`. Only when the box is empty is the same text
   tried again with no box. The position is the report pin for `/api/geocode`, which gains optional
   `lat`/`lon` (both or neither, checked by the place search's own `parsePlaceCentre`). For
   `/api/geocode/place` it is the map centre. The reader lane used to send `bounded=0`; it now
   follows the same box then widen rule.
3. **Fewer words.** When a text finds nothing in both its tries, its first comma separated part is
   dropped, or its first word when there are no commas. This happens at most twice, each time on
   the text before it, and stops at the first try that finds something. Parts and words are
   trimmed and empty parts skipped. A shorter text under 3 letters is never sent.
4. **Honest answer.** The JSON answer has `matched_query` only when a shorter text matched. Both
   lists then show this line above the results, as a `<p>` built with `Pinalert.setText`:

   `No exact match for the full address. Showing places for: {matched text}.`

   Example: `No exact match for the full address. Showing places for: Road No 12, Banjara Hills, Hyderabad.`

   Each result keeps its full address, and choosing one moves the pin or the map exactly as before.
   On the place search the hidden screen reader status reads the line before the result count.
5. **Usage policy.**
   - Every upstream request takes a limiter token first, on both lanes.
   - One search sends at most 4 upstream requests.
   - The final answer, an empty one included, is cached for 6 hours. The key is the original text
     plus the area, rounded to a 0.1 degree grid. Errors are never cached.
   - Running out of limiter time part way through is the busy answer (503 with each lane's
     existing message), never "No matches".
   - The report lane now has a server cache of its own (it had none before). It uses the same
     type, size and time to live as the reader cache.

### The request cap, the order, and why

The cap is **4 upstream requests per search**. At Nominatim's one request a second this keeps the
worst search to about 3 seconds of limiter waiting plus network, inside the report form's 8 second
client timeout. Four is also the smallest cap that fits the most useful order for the report form:

- With a position: full text in the box, full text widened, first shorter text in the box, first
  shorter text widened. The cap stops it there, so **the second shorter text is never tried when
  there is a position.**
- Without a position: full text, first shorter text, second shorter text (3 requests).

### Reader lane and D-16 (recorded decision, owner should know)

The plan says every upstream request waits on the limiter. The reader lane's standing rule (D-16,
D-28) is that a reader never queues ahead of a reporter. Both now hold:

- The reader's first try still takes a token only if one is free right now. This is unchanged, and
  `TestSearchReaderNeverWaits` and `TestSearchReaderDoesNotStarveSearch` pass untouched.
- Each later try (the widened one or a shorter text) waits *without reserving a token*:
  - If a reporter is already waiting (the bucket is in debt), it gives up at once.
  - Otherwise it sleeps until a token would be free and takes it only if one still is.
  - A reporter who starts in that gap gets the token, and the reader gets the busy answer.
- `TestReaderFollowUpNeverQueuesAheadOfAReporter` proves it. A reporter who starts while a reader
  is between tries is answered within one limiter interval, and the reader's next try is never sent.
- Cost: a reader's place search can now take about 3 seconds in the worst case (it used to answer
  at once or be busy). When reporters are active, a reader's multi try search is more likely to end
  busy.

### Other changes

- Every new search on the report form now aborts the previous request in flight. A server search
  can run several seconds, and an old one would otherwise keep spending the one request a second
  budget the new one needs.
- The browser cache for the report form is now keyed by the whole request (pin included) and keeps
  `matched_query` with the results, so a repeat search shows the line again.
- A pin outside the range the server accepts (a dragged pin past 180 longitude) is left out of the
  request rather than turning the search into a 400.
- The `/api/geocode` failure log now names the error class only, as the place search already did.
  The pin's area is now in the request, so a raw error string could have carried it.
- The access log is path only (`requestLogger` in `internal/api/router.go`), so the new `lat` and
  `lon` never reach it.

## Files

- `internal/geocode/client.go`: `Answer`, the search plan, the shared `run` loop, the reporter cache,
  the reader token rule and `countrycodes=in`.
- `internal/geocode/client_test.go`: existing tests adapted to the new signatures, the
  `countrycodes` key and `bounded=1`.
- `internal/geocode/search_plan_test.go` (new): shortening, plan order and cap, India only on every
  request, box then widen, shorter tries and stop at first hit, cache key, limiter wait per request
  on both lanes, busy not empty, and the D-16 proof.
- `internal/api/handlers/geocode.go`: `lat`/`lon` on `/api/geocode`, `matched_query`, the shared
  `geocodeAnswer`, and a log line with the error class only.
- `internal/api/handlers/place.go`: the `Answer` return, `geocodeAnswer`, and doc comments.
- `internal/api/handlers/geocode_test.go`, `place_test.go`: stubs return `geocode.Answer`.
- `internal/api/handlers/geocode_near_test.go` (new): pin passed through, `lat`/`lon` validation
  (9 cases, no upstream call on a refusal), `matched_query` present and absent on both routes,
  failure log holds no pin.
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`: regenerated with
  `swag init -g internal/api/router.go -o docs`.
- `web/static/js/map-tools.js`: `MESSAGE_PARTIAL_PREFIX`; `renderResults(list, matched)` draws the
  line.
- `web/static/js/report-form.js`: `SEARCH_PARTIAL_PREFIX`; `renderDropdown(results, matched)`;
  `searchPin()`; `abortSearchRequest()`; cache keyed by the request.
- `web/static/css/feed.css` (`.map-search__note`), `web/static/css/modal.css`
  (`.location-search-note`): label size, muted colour, 8 px padding, house tokens only.
- `web/address_search_contract_test.go` (new): the line's text in both scripts, built as text above
  the results, the pin sent and cached, the abort, and the styles.
- `web/map_tools_contract_test.go`, `web/low_form_contract_test.go`: function signature pins
  updated.

## Commits

| Commit | Step |
| ------ | ---- |
| 85cb9eb | test: failing client tests |
| b8188bc | feat: client India only, near first, fewer words |
| 61e727e | test: failing handler tests |
| 9751d3e | feat: handlers take the pin, return `matched_query` |
| 1a0b75c | docs: swagger regenerated |
| a43c15f | test: failing contract tests |
| bf168a1 | feat: the line in both scripts, the pin, abort, styles |

## Tests

- `go build ./...` and `go vet ./...`: clean.
- `go test ./internal/geocode/ -count=5 -race`: ok. 27 top level tests pass on a single run.
- `DATABASE_URL=postgres://localhost:5432/p4_quick_adr?sslmode=disable go test ./... -p 1 -count=1 -v`,
  run on a freshly created `p4_quick_adr` that was dropped afterwards: every package ok, 2914 `--- PASS`, 0
  `--- FAIL`, 1 `--- SKIP`. The skip is `TestScreeningFixture` (needs recorded OpenAI scores, not
  a database). 0 database backed skips.
- `node --test web/*.mjs`: 36 pass, 0 fail.
- `go test ./web/ -count=1`: ok (the 3 new contract tests included).
- No em or en dash added in any changed line.

## Left open

- **No browser check was done here.** No server was started, per the task rules. The width pass
  at 320, 360 and 390 px is still owed (see below).
- The second shorter text is never reached when there is a position, because of the cap of 4.
  If long addresses near the pin still miss, the next change could be: full text in the box, the
  shorter texts in the box, then the widened request.
- The line says "full address" on the place search too, as the plan wrote it, even when the reader
  typed a place name rather than an address.
- On the report form the line sits inside the suggestion list (`role="listbox"`). That list already
  holds buttons rather than options, so nothing new is broken. A screen reader hears the line only
  when moving through the list; the place search announces it.
- Photon was not evaluated (backlog 999.7 mentions it if results stay poor).
- Backlog 999.7 in ROADMAP.md was left alone, per the task rules.

## What a browser check should try

1. Report form, pin in Hyderabad: type `Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad`.
   Expect the line with a shorter text above the rows, and a tapped row to move the pin.
2. Report form, pin in Delhi: type `Hyderabad`. Expect Hyderabad in Telangana, never Pakistan, and
   no line.
3. Map search: type `Ring Road`. Expect Indian roads near the map centre first, nothing in Hanoi.
4. Type quickly in pauses on the report form. No "Search unavailable" should come from your own
   earlier searches.
5. Check the line at 320, 360 and 390 px in both lists. It should wrap inside the box, in muted
   14 px text with a divider under it. Check light and dark.
