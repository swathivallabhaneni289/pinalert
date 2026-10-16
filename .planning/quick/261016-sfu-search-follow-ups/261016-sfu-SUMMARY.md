---
quick_id: 261016-sfu
completed: 2026-10-16
status: complete
source: open items of quick 261016-pho and the owner's request that typing a place is quick
---

# Quick 261016-sfu: whole addresses, city in labels, quicker suggestions

The whole Flat 4B address now finds Road No 12 rows, with the "No exact match for the full
address. Showing places for: ..." line. Live, the report form took 3.36 s and the map's place search
took 1.53 s. Before this task the same address found nothing.

What changed for people:
- The report form now searches after a 300 ms pause instead of 600 ms.
- The map's place search now suggests while you type, after the same pause.
- Photon rows that have no city now carry their county. In Hyderabad that county turned out to be a
  mandal, not the city, so this only partly met its goal (see Left open).

## What changed

### 1. The search order (`internal/geocode/client.go`, `photon.go`)

When Photon finds nothing for the whole text, the search now runs this order. It stops at the first
request that finds something, and the cap stays at four upstream requests.

| Step | Service | Text | Notes |
| --- | --- | --- | --- |
| 1 | Photon | whole text | as in 261016-pho |
| 2 | Nominatim | whole text | once; India only; with a position it sends a `viewbox` around it but no `bounded`, so near places come first and an address in another city is still found |
| 3 | Photon | first shorter text | sets `matched_query` when it finds something |
| 4 | Photon | second shorter text | sets `matched_query` when it finds something |

- The shorter texts are the same ones as before (`searchTexts`): drop the first comma part, or the
  first word when there are no commas.
- A text with no shorter form stops after step 2. Example: `Hyderabad` sends Photon, then Nominatim.

**When Photon fails.** An error, its 2 s limit or no token for the first try all count as Photon
failing. The search then runs the 261016-pho Nominatim plan with the 3 requests left, exactly as
before: boxed, widened, then the first shorter text.

**Once Photon fails in a search, it is not asked again in that search.**
- If a shorter text's Photon try fails (error, time limit, or no token in time on the report
  search), that text and any text left go to Nominatim, the same way as step 2, inside the cap.
- So a Photon outage never costs several 2 s timeouts in one search.

**A Nominatim error in step 2 ends the search with that error,** which shows "Search unavailable".
- That covers a non 2xx status, a time limit, or the reader finding no free token.
- It is never turned into an empty list, because the shorter texts were never sent.

**Caching.**
- Photon's cache now also keeps the answer for each shorter text, under that text and the area.
  Typing the shorter text itself later is answered from that cache, with no `matched_query`.
- The final answer, `matched_query` included, goes into the search's own cache under what was typed.
- An empty answer after any Photon failure is still not cached, and errors never are.

**Limiter tokens.** Each service's tries are counted from 0.
- **Report search:**
  - Photon's first try waits at most 0.5 s for a token, as before.
  - A shorter text's Photon try waits up to 1.5 s, the same as a Nominatim try.
  - Why 1.5 s: at one Photon request a second, the second shorter text's token is most of a second
    away. With the 0.5 s wait it would never be reached (pinned by
    `TestReporterWaitsLongerForALaterPhotonToken`).
- **Place search (D-16):**
  - The first try on each service takes a token only if one is free right now. Busy is answered
    at once, as before.
  - A later Photon try waits without reserving a token, and gives up busy when a report search is
    waiting.
  - `TestReaderShorterPhotonTryNeverQueuesAheadOfAReporter` proves it. While the reader waits
    between its Photon tries, a report search goes first, and the reader returns busy without
    sending its shorter text.
  - The 261016-pho D-16 tests pass unchanged.

### 2. Labels (`photonLabel`)

- A Photon row with no city now uses its `county` in the city's place.
- Empty parts, `null` and repeated parts are still skipped.
- **A change made after the live run:** a row whose Photon `type` is `city`, `county`, `state` or
  `country` never takes the county.
  - Why: live, Hyderabad the city carried `Bahadurpura mandal` as its county. It read
    `Hyderabad, Bahadurpura mandal, Telangana`.
  - That label no longer matched the district row `Hyderabad, Telangana`, so the repeated
    Hyderabad row that 261016-pho removed was back.
  - With the fix the city reads `Hyderabad, Telangana` again, and the repeat is dropped.
  - Pinned by `TestPhotonCityRowGetsNoCounty`, built from the live shape. The live check was not
    run again.

### 3. Report form pause

- `SEARCH_DEBOUNCE_MS` is now 300. It was 600.
- That constant is the only change in `report-form.js`.
- `TestReportSearchPauseIs300` pins it. Nothing pinned it before.

### 4. The map's place search while typing (`web/static/js/map-tools.js`)

- **When it runs.** After a 300 ms pause in typing (`TYPING_PAUSE_MS`) with at least 3 letters,
  it runs the same `search` and draws the same list. The count is read out the same way as after
  Enter: the quiet status line.
- **Newer wins.** A newer search ends the older one, which is aborted and its answer dropped.
  Before, a second submit during a request was ignored.
- **Enter during a typed search.**
  - For the same text, Enter takes that search over: Searching shows, and any failure line will
    show. No second request is sent.
  - Enter during a pause searches at once and cancels the pause.
- **A typed search is quiet.**
  - No Searching line, no "Type at least 3 letters." and no failure line. That covers a 503, a
    429 and a network error.
  - On a failure, the screen stays as it was. The list for the earlier text stays until Enter asks.
  - Enter and the button still show every line.
- **Under 3 letters while typing** closes the list quietly.
- **A pause on the text already shown** asks nothing again. A trailing space counts as the same
  text.
- **Ending a pause or a typed search.** Escape, Close, a press outside the search and a pick all
  stop a pending pause or a typed search still out. A press outside does this even when nothing is
  on screen yet.
- **Back to my location.** Typing alone does not cancel a pending Back to my location; Enter, the
  button and a pick still do (IN-07).
- **The file header.** It said nothing is sent while the reader types (D-14, T-31-51). It now
  states the new rules.

### 5. Route descriptions

- The swagger descriptions of `GET /api/geocode` and `GET /api/geocode/place` now name the new
  order.
- They were regenerated with `swag init -g internal/api/router.go -o docs`. Only description text
  changed.
- README and the Policy page still read true ("fall back to Nominatim only when Photon fails,
  takes over 2 seconds or finds nothing"), so they are unchanged.

## Live check

`GEOCODE_LIVE=1 go test ./internal/geocode/ -run TestGeocodeLive -count=1 -v`, run once on
2026-10-16.

- Both limiters were set to one request a second, with a 1.2 s pause between searches.
- Pins: Hyderabad 17.385, 78.4867; Delhi 28.6139, 77.2090.
- "First results" shows the first three labels.
- One new row runs the Flat 4B address through the place search with no pin.

Two notes on the table:
- **It was run before the city row fix (e4eb0a7).** In the three Hyderabad city rows (Hyderbad,
  Hyderabad from Delhi, Hyderbad with no pin), the first label now reads `Hyderabad, Telangana` and
  the second, repeated one is dropped. That was not re-run live; it is covered by
  `TestPhotonCityRowGetsNoCounty`.
- **One dash replaced.** In the Telugu row, one Nominatim label had an en dash in its OpenStreetMap
  data (`Medchal-Malkajgiri`). It is written here with a hyphen, and that row is shortened.

| Typed | Search | Pin | Answered by | Took | Matched text | First results |
| --- | --- | --- | --- | --- | --- | --- |
| Banjra Hills | report | Hyderabad | photon | 1.18s | | Banjara Hills, Ward 92 Venkateshwara Colony, Shaikpet mandal, Telangana; Ward 93 Banjara Hills, Shaikpet mandal, Telangana; Banjara Hills Road Number 11, Gaffar Khan Colony, Ward 93 Banjara Hills, Shaikpet mandal, Telangana |
| Hyderbad | report | Hyderabad | photon | 670ms | | Hyderabad, Bahadurpura mandal, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |
| Banjara Hi | report | Hyderabad | photon | 280ms | | Banjara Hills, Ward 92 Venkateshwara Colony, Shaikpet mandal, Telangana; Banjara Hills Road Number 7, Banjara Hills, Ward 92 Venkateshwara Colony, Shaikpet mandal, Telangana; Banjara Hills Road Number 8, ... Shaikpet mandal, Telangana |
| Gachib | report | Hyderabad | photon | 370ms | | Gachibowli, Serilingampalle mandal, Telangana; gachibowli rd, Kala Daira, Ward 28 Azampura, Amberpet mandal, Telangana; Gachibowli - Miyapur Highway, Ward 104 Kondapur, Serilingampalle mandal, Telangana |
| Road No 12 Banjara Hills Hyderabad | report | Hyderabad | photon | 520ms | | Plot No 70, Road No 12 Koushik Co-Operative Society, Banjara Hills, Bhavani Nagar, Ward 93 Banjara Hills, Hyderabad, Telangana; Flat No 33, Durga Enclave, Road No 12, Banjara Hills, Motilal Nehru Nagar, Ward 149 Begumpet, Hyderabad, Telangana; Plot No 36, Road No 12, Mla Colony, Banjara Hills, Doolapally, Hyderabad, Telangana |
| Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad | report | Hyderabad | photon | 3.36s | Sai Residency, Road No 12, Banjara Hills, Hyderabad | Ramakrishna Residency, Road No. 12, Bhavani Nagar, Ward 93 Banjara Hills, hyderabad, Telangana; Flat No 33, Durga Enclave, Road No 12, Banjara Hills, Motilal Nehru Nagar, Ward 149 Begumpet, Hyderabad, Telangana; Plot No 70, Road No 12 Koushik Co-Operative Society, Banjara Hills, Bhavani Nagar, Ward 93 Banjara Hills, Hyderabad, Telangana |
| Hyderabad | report | Delhi | photon | 520ms | | Hyderabad, Bahadurpura mandal, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |
| Ring Road | report | Hyderabad | photon | 500ms | | Inner Ring Road, new balaji nagar, Jillelaguda, Balapur mandal, Telangana; Inner Ring Road, Khayyam Nagar, Ward 61 Attapur, Rajendranagar mandal, Telangana; Inner Ring Road, Ward 10 Uppal, Uppal mandal, Telangana |
| హైదరాబాద్ (Hyderabad in Telugu) | report | Hyderabad | nominatim | 1.41s | | University of Hyderabad, ... Ranga Reddy, Telangana, 500046, India; Indian Institute of Technology Hyderabad, ... Sangareddy, Telangana, 502001, India; Jawaharlal Nehru Technological University, Hyderabad, ... Medchal-Malkajgiri, Telangana, 500085, India |
| Hyderbad | report | none | photon | 670ms | | Hyderabad, Bahadurpura mandal, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |
| Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad | place | none | photon | 1.53s | Sai Residency, Road No 12, Banjara Hills, Hyderabad | Ramakrishna Residency, Road No. 12, Bhavani Nagar, Ward 93 Banjara Hills, hyderabad, Telangana; Flat No 33, Durga Enclave, Road No 12, Banjara Hills, Motilal Nehru Nagar, Ward 149 Begumpet, Hyderabad, Telangana; Plot No 36, Road No 12, Mla Colony, Banjara Hills, Doolapally, Hyderabad, Telangana |

What the table shows:

- **Flat 4B now works.** It finds Road No 12 rows on both searches, and `matched_query` is set, so
  the "No exact match" line shows.
  - It was found on the first shorter text, so step 4 was not needed.
  - The report search's 3.36 s includes waiting about a second for each Photon token, at the live
    check's one a second. In production Photon allows two a second with a burst of 2, so this
    should be quicker.
- **The typo and prefix rows still answer from Photon in under 1.2 s.**
- **Labels on rows below a city now end with a mandal.** For example `..., Shaikpet mandal,
  Telangana`. This is the county; it is not the city (see Left open).
- **The Telugu row is unchanged from 261016-pho.**

## Report form worst case against its 8 s limit

These numbers are for a search where Photon finds nothing, with every limit hit:

| Step | Most token wait | Most request time | Total for the step |
| --- | --- | --- | --- |
| Photon, whole text | 0.5 s | 2 s | 2.5 s |
| Nominatim, whole text | 1.5 s | 5 s | 6.5 s |
| Photon, first shorter text | 1.5 s | 2 s | 3.5 s |
| Photon, second shorter text | 1.5 s | 2 s | 3.5 s |

- In total that is at most 5 s of token waits plus 11 s of request time limits: 16 s.
- The browser gives up at 8 s (`SEARCH_TIMEOUT_MS`). That cancels the search on the server, and
  the person sees "Search unavailable".
- The Photon failure path is unchanged from 261016-pho. Its bound is higher, since three Nominatim
  tries can each take 5 s.
- Normal case: with no queue, the waits are near zero, so the search takes about four network
  round trips. That was 1.5 s live for the place search and 3.36 s for the report search with
  one a second limiters.

## Files

- `internal/geocode/client.go`:
  - `search` now takes a try index for both services;
  - `afterPhotonFoundNothing` (the new order) is new;
  - `nearParams` (a viewbox, no bounded) is new;
  - comments are updated.
- `internal/geocode/photon.go`:
  - the reporter waits up to 1.5 s for a later Photon token;
  - the reader's Photon try takes its index;
  - `askPhoton` takes the text and builds its own cache key;
  - the `County` and `Type` fields and `cityLevelTypes` are new;
  - the label rule and the limiter comment are updated.
- `internal/geocode/order_test.go` (new): 9 top level tests for the order, the cap, the matched
  text, caching, errors and D-16.
- `internal/geocode/photon_test.go`:
  - the "nothing found" fallback case was moved to the order tests;
  - the cap test now uses a failing Photon;
  - there are 4 new label cases, `TestPhotonReadsTheCounty` and `TestPhotonCityRowGetsNoCounty`.
- `internal/geocode/live_test.go`: a Search column and the place search row.
- `web/static/js/map-tools.js`: suggestions while typing.
- `web/static/js/report-form.js`: `SEARCH_DEBOUNCE_MS = 300` only.
- `web/search_follow_ups_contract_test.go` (new): `TestReportSearchPauseIs300` and
  `TestPlaceSearchSuggestsWhileTyping`.
- `web/place_search_typing_test.mjs` (new): 11 node tests that run the real `map-tools.js` with
  fake timers and a fetch answered by hand.
- `web/map_tools_contract_test.go`, `web/address_search_contract_test.go`,
  `web/review_fix_a3_contract_test.go`: read the new `search(query, typed)` and drop the "no type
  ahead" and "ignored while in flight" pins.
- `internal/api/handlers/geocode.go`, `place.go`, `docs/docs.go`, `docs/swagger.json`,
  `docs/swagger.yaml`: route descriptions.
- Not touched: `queue.js` and the rest of `report-form.js`.

## Commits

| Commit | Step |
| --- | --- |
| d0c25ab | test: failing tests for the order when Photon finds nothing |
| 3e918ca | feat: Nominatim whole text, then Photon shorter texts |
| 129fe33 | test: failing label tests for a row with no city |
| e464a1e | feat: labels take the county when there is no city |
| 011b801 | test: failing tests for the 300 ms pause and the map search while typing |
| 2f17f42 | feat: the map's place search suggests while typing |
| a01210e | feat: the report form searches after a 300 ms pause |
| 2ae554b | test: older contract tests read `search(query, typed)` |
| daaeb63 | docs: route descriptions, swagger regenerated |
| 6e5990b | test: live check place row; failing test for the city row's county |
| e4eb0a7 | fix: a city level row never takes the county |

Commit a01210e went in while `go test ./web/` was red. A pipe into `tail` hid the exit status. The
cause was two older contract tests that still read `search(query)`, and 2ae554b fixed them. History
was left as it is.

## Tests

- `go build ./...` and `go vet ./...`: clean. `gofmt -l internal/`: nothing.
- `go test ./internal/geocode/ -race -count=3`: ok.
  - On one `-v` run: 56 top level `--- PASS` (45 before), 70 subtests and 1 `--- SKIP`
    (`TestGeocodeLive`).
- `DATABASE_URL=postgres://localhost:5432/p4_quick_sfu?sslmode=disable go test ./... -p 1 -count=1 -v`:
  - run on a freshly created `p4_quick_sfu`, dropped afterwards;
  - all 13 packages ok, 2997 `--- PASS`, 0 `--- FAIL`, 2 `--- SKIP`;
  - the two skips are `TestGeocodeLive` (needs `GEOCODE_LIVE=1`) and `TestScreeningFixture` (needs
    recorded OpenAI scores). Neither needs a database, so there are 0 database backed skips.
- `node --test web/*.mjs`: 68 pass, 0 fail (11 new).
- `go test ./web/ -count=1`: ok.
- No em or en dash in any added line, checked by byte pattern over `git diff 225944d`.

## Left open

- **The county does not name the city in Hyderabad.**
  - Photon's `county` there is a mandal (Shaikpet, Bahadurpura, Serilingampalle). So area and
    street rows now end `..., Shaikpet mandal, Telangana`, not `..., Hyderabad, Telangana`.
  - No field Photon sent was seen to name Hyderabad for these rows. Only the labels were seen live,
    not the raw properties.
  - This is the owner's choice:
    - keep the mandal (more place detail, longer labels);
    - drop the county again (labels as in 261016-pho);
    - or look for another source for the city.
- **Suggestions can stop during fast typing.**
  - Both searches now ask more often. They also have per person budgets:
    - the place search, per account: 3 at once, then 1 every 4 s;
    - the report search, per IP: 3 at once, then 1 every 2 s.
  - The place search also answers busy at once when Photon's two tokens are used. One long
    address can use three Photon tokens.
  - On the map, a refused or busy typed search shows nothing, so suggestions just stop until the
    budget refills. Enter then shows the real line.
  - The report form has no quiet path. A 429 from typing there shows its failure line, which the
    300 ms pause makes more likely. Its search code was left alone apart from the constant.
  - Changing the budget numbers, or quieting the report form's typed failures, is the owner's call.
- **Telugu and Hindi script search** is unchanged from 261016-pho.
- **No browser check was done.** No server was started, per the task rules.

## What a browser check should try

1. Report form, pin in Hyderabad. Type the Flat 4B address. Expect:
   - "No exact match for the full address. Showing places for: Sai Residency, Road No 12, Banjara
     Hills, Hyderabad.";
   - Road No 12 rows, in a few seconds;
   - a tapped row moves the pin.
2. Report form: type `Banjra Hills` at a normal speed. Suggestions should appear about 300 ms after
   you stop, a little sooner than before.
3. Map search, typing:
   - Type `Banj` and stop. Suggestions appear with no Enter and no Searching line.
   - Keep typing to `Banjara Hills`. The list updates after each pause.
   - A screen reader hears "N results.".
4. Map search, short text: type `Hy`. Nothing is sent and no "Type at least 3 letters." line shows.
   Press Enter and the line shows.
5. Map search, fast typing: type quickly with short pauses for a while. You should never see
   "Search unavailable" or "Too many searches". Suggestions may just pause. Press Enter: any real
   message shows then.
6. Map search, Enter during a pause: type `Ring Road` and press Enter straight away. One search,
   Searching shows, and the list appears.
7. Map search, a pick: tap a suggestion. The map moves, "Showing reports near ..." uses the name,
   and no late list pops back up.
8. Map search, ending a pause: type and, within 300 ms, press Escape, Close or tap the map. No list
   appears afterwards.
9. Labels: type `Hyderbad`. `Hyderabad, Telangana` shows once, at the top. Area rows such as
   Banjara Hills end with a mandal and Telangana, with no `null`.
10. Phone widths: check 320, 360 and 390 px. The list shows under the phone search row while
    typing, and the keyboard does not hide the first row.
