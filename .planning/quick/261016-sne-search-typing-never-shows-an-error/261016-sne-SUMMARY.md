---
quick_id: 261016-sne
completed: 2026-10-16
status: complete
source: owner request "make sure we have no errors on that part" (address search) and the open items of quick 261016-sfu
---

# Quick 261016-sne: search while typing never shows an error

Both address searches now let one person type a whole address without being refused, and a refusal
or a busy answer during typing is retried quietly instead of showing a failure line.

## What changed

### 1. Budgets (`internal/api/router.go`, `cmd/server/main.go`, `internal/api/handlers/geocode.go`)

- `GET /api/geocode`, the report form's search, no longer has the per IP middleware. Its handler
  now keys a budget by the verified account under `geocode:<id>`, the same way
  `/api/geocode/place` uses `place:<id>`. The two keys never share a budget.
- Both budgets are now **8 searches at once, then 1 a second**:
  - `GeocodeRateLimitDefault`: was 3, then 1 every 2 s, per IP.
  - `PlaceRateLimitDefault`: was 3, then 1 every 4 s, per account.
- `cmd/server/main.go` sets both budgets itself, so its constants changed too. A new test,
  `TestSearchBudgetsMatchTheRouterDefaults`, keeps them equal to the router defaults.
- **Why these numbers.** Both searches run after a 300 ms pause in typing (261016-sfu).
  - So one person typing one address sends a search about every 300 ms, in short bursts.
  - A burst of 8 covers one address typed with several pauses.
  - A refill of 1 a second keeps a little behind steady typing. The quiet retry below absorbs
    that gap.
  - The old numbers ran out within one address.
- **Why per account.** Carrier NAT, or a host behind a proxy without CLIENT_IP_HEADER, puts many
  people behind one address (D-16). A per IP budget would make them share these 8 searches.
- **Not changed:** the process wide Photon and Nominatim limiters inside `internal/geocode`. They
  still bound total traffic to both services.
- The reasons are written in a comment beside the defaults in `router.go`.
- **What a refusal says now.** A refused report search answers 429 with field `q` and the place
  search's existing words, "Too many searches. Wait a few seconds and try again."
  - Before, it got the per IP middleware's "Too many requests. Try again in a minute." with field
    `email`. "A minute" was wrong for this budget.
  - No new words were written.
- A request with no verified account is a 500, the same as the place handler. The route is inside
  the gate, so this only catches a programming error.
- Swagger: one line in each search's description now names the budget. It was regenerated with
  `swag init -g internal/api/router.go -o docs`.

### 2. Report form quiet retry (`web/static/js/report-form.js`, search part only)

- Every search on the report form starts from typing. It has no Enter search.
- On a 429 or a 503:
  - no failure line shows;
  - the list on screen stays;
  - no `unavailable` outcome is reported (the Low data page reads that outcome).
- The latest text is tried again after the answer's `Retry-After`, or 1 second when it sent none,
  at most 3 times (`SEARCH_QUIET_RETRIES`, `SEARCH_RETRY_MS`).
  - That makes 4 requests in all. The failure line shows only when the 4th fails, the same
    line as before.
  - A `Retry-After` above 10 s is cut to 10 s (`SEARCH_RETRY_MAX_MS`). The server never sends one
    on these routes today, so 1 s is the real path.
- The retry waits on `searchDebounceTimer`, so new text and closing the form both cancel it. New
  text starts its own count of 3.
- **Unchanged:** a timeout, a network error and any other status show the line at once, as before.
  A retried 8 s timeout would leave someone waiting about 30 s in silence.
- Still one `fetch(` in the file. A retry goes back through `runSearch(query)`.
- Posting, closing, the saved reports and `queue.js` are untouched.

### 3. Map search quiet retry (`web/static/js/map-tools.js`)

- A typed search answered 429 or 503 is retried the same way: same numbers, same wait.
- It uses `typingTimer` for the wait, so any of these cancel a pending try:
  - newer typing;
  - Enter or the button;
  - Escape or Close;
  - a press outside;
  - a pick.
- When the last try fails, the old list closes and the failure line shows, as it does after Enter.
  - **This changes a 261016-sfu rule.** Before, a typed search never showed a failure line.
  - The plan's done list asks for "the failure line after the last try, for both scripts", so
    I followed it. The file header says so.
  - To keep the map always quiet while typing instead, drop the `lastTry` branch in `search`.
- A network error from typing stays quiet and is not tried again, as before.
- Enter and the button show every line as before, and are never retried.

## Files

- `internal/api/handlers/geocode.go`: account check, per account budget, 429 line, header comment,
  swagger line.
- `internal/api/handlers/place.go`: `PlaceBudget` comment and swagger line.
- `internal/api/router.go`: no per IP middleware on `/api/geocode`, new defaults, comments.
- `cmd/server/main.go`: constants 8 and 1 s, comments.
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`: regenerated.
- `web/static/js/report-form.js`: quiet retry in the search part.
- `web/static/js/map-tools.js`: quiet retry and the header rule.
- Tests:
  - `internal/api/handlers/geocode_budget_test.go` (new): no account, per account keys, 8 then 1 a
    second, nil budget.
  - `internal/api/handlers/geocode_route_e2e_test.go` (new): two accounts behind one address, 8
    then 429, then 1 more a second later.
  - `internal/api/handlers/place_route_e2e_test.go`: 8 instead of 3, plus the refill.
  - `internal/api/handlers/geocode_test.go`, `geocode_near_test.go`: requests now carry an account;
    `Geocode(stub, nil)`.
  - `internal/api/search_budget_test.go` (new): pins both defaults.
  - `cmd/server/main_test.go` (new): the program's constants equal the defaults.
  - `web/report_search_retry_test.mjs` (new): 8 node tests.
  - `web/place_search_typing_test.mjs`: 7 new node tests; the fake answer can carry headers.
  - `web/search_follow_ups_contract_test.go`: `setTimeout(` count in map-tools.js is now 2, and
    the new `TestSearchQuietRetry` pins the constants and the retry shape in both scripts.

## Commits

| Commit | Step |
| --- | --- |
| e4f87d6 | test: failing tests for per account budgets of 8 then 1 a second |
| 6dabab5 | feat: both searches get a per account budget of 8, then 1 a second |
| bbcb85f | test: failing tests for the quiet retry of both searches |
| d74695e | feat: the report form retries a refused or busy search quietly |
| 5996ca0 | feat: the map's place search retries a refused or busy typed search quietly |

## Tests

- `go build ./...`, `go vet ./...`: clean. `gofmt -l internal cmd web`: nothing.
- `DATABASE_URL=postgres://localhost:5432/p4_quick_sne?sslmode=disable go test ./... -p 1 -count=1 -v`:
  - run on a freshly created `p4_quick_sne`, dropped afterwards;
  - all 14 packages ok (`cmd/server` now has a test);
  - 3005 `--- PASS`, 0 `--- FAIL`, 2 `--- SKIP`;
  - the two skips are `TestGeocodeLive` (needs `GEOCODE_LIVE=1`) and `TestScreeningFixture` (needs
    recorded OpenAI scores). Neither needs a database, so 0 database backed skips.
- `node --test web/*.mjs`: 83 pass, 0 fail. That was 68; there are 15 new.
- `go test ./web/ -count=1`: ok.
- No test calls Photon or Nominatim.
- No em or en dash in any added line, checked over `git diff cf5d6f6`.

## Left open

- **Busy and down look the same.** The report search answers every failure with the same 503. So
  the quiet retry also covers a real outage that answers fast, and the line shows a few seconds
  later than before.
  - A slow outage still ends in the browser's 8 s limit. That is not retried, so its line is not
    delayed.
  - Telling busy from down would need a marker from `internal/geocode`. It was left out to keep the
    limiters as they are.
- **No Retry-After.** Neither search route sends one. The browser waits 1 s, which matches the 1 a
  second refill.
- **The map now shows a failure line from typing** after the last try (see section 3).
- **Older 261016-sfu items unchanged:** the mandal in labels, and Telugu or Hindi script search.
- **No browser check was done.** No server was started, per the task rules.

## What a browser check should try

1. Report form, pin in Hyderabad. Type the Flat 4B address at a normal speed, with short pauses.
   - No "Too many searches" or "Search unavailable" line should show.
   - Suggestions should keep up and end on the Road No 12 rows.
2. Report form, typing fast for a long while: keep editing the text for 15 to 20 seconds with
   short pauses.
   - No line should show.
   - If a search is refused, the list for the text before stays and updates about a second
     later.
3. Map search: type `Banj`, then keep typing to `Banjara Hills Road No 12` with pauses.
   - Suggestions update after each pause and never stop for good.
   - No Searching line and no failure line.
4. Map search, Enter: type `Ring Road`, press Enter. Searching shows, then the list. Enter still
   shows any failure line at once.
5. Map search, a pending try: while a list is up, type fast, then press Escape or tap the map.
   No list should come back a second later.
6. Two phones or two browsers signed in to different accounts on the same Wi-Fi. Both type at the
   same time; neither should be refused because of the other.
7. To see the last try's line, block the geocoder in DevTools so it answers 503, or take the
   network down after the page loads, then type in either search.
   - A 503 shows the failure line about 3 seconds after the first failure.
   - With the network off, the map stays quiet and the report form shows its line at once.
8. Phone widths 320, 360 and 390 px: the list stays in place under the search row during a quiet
   retry. Nothing should jump.
