---
quick_id: 261016-pho
completed: 2026-10-16
status: complete
source: owner request after quick 261016-adr (address search must match close spellings, fast, no errors)
---

# Quick 261016-pho: search that matches close spellings and half typed words

Both searches (the report form's `GET /api/geocode` and the map's `GET /api/geocode/place`) now ask
Photon (photon.komoot.io, OpenStreetMap data, by komoot) first. Photon matches typos and half typed
words: in the live check `Banjra Hills`, `Hyderbad`, `Banjara Hi` and `Gachib` all found the right
place, in 180 to 790 ms. Nominatim, with the India only, near first, fewer words plan of quick
261016-adr, now runs only when Photon errors, times out or finds nothing. The JSON answers keep
their shape, so no browser code changed.

## What changed

1. **Photon first** (`internal/geocode/photon.go`). Each Photon request sends:
   - `q`, `limit=10`, `lang=en`, and `bbox=68.0,6.4,97.5,37.1` (the India box);
   - `lat` and `lon` of the pin (report form) or the map centre (place search) when known.

   No `email` is sent to Photon. The User-Agent is the same one Nominatim gets (`Pinalert/1.0 (+contact)`).
2. **India only.** The box also covers parts of Pakistan, Nepal, Bangladesh and Sri Lanka, so every
   answer is kept only when its `countrycode` is `IN` (any case). A Photon answer with only foreign
   rows counts as nothing found, and the search falls back to Nominatim.
3. **Coordinates.** Photon sends GeoJSON coordinates longitude first; a test pins lat 17.4, lon 78.4.
   A row with a missing, NaN or out of range coordinate is skipped, never clamped.
4. **Labels** (`photonLabel`). The label is built from these parts, in order:
   - name, house number, street, locality, district, city, state.

   Empty parts, the text `null` and repeated parts (in any case) are skipped. So a house reads
   `8-2-293/82, Road No. 12, Banjara Hills, Hyderabad, Telangana`, and a city never reads
   `Hyderabad, Hyderabad`. A house with no name keeps `name` empty. Both scripts already show
   `display_name` when `name` is empty.
5. **Repeated rows.** A row whose label reads the same as one already kept is dropped. The live
   check showed `Hyderabad, Telangana` twice (city and district) and one ring road twice (split
   OpenStreetMap ways). This was found after the live run and fixed with a test (commit 097ca22).
6. **Ranking.** See the rule below. After ranking, the list is cut to 5.
7. **Fallback.** Nominatim runs when Photon:
   - answers with a non 2xx status;
   - sends a body that does not decode;
   - takes over 2 seconds;
   - finds nothing in India;
   - or, for the report form, has no limiter token within 0.5 s.

   `matched_query` and the "No exact match for the full address. Showing places for: ..." line work
   exactly as before, because they come from the unchanged Nominatim plan.
8. **The caller going away is not a fallback.** If the browser aborts, or the server shuts down,
   while Photon is being asked, the search ends with that error and Nominatim is never called. The
   error goes through `scrubURLError`, so it holds no search text and no position (tested).
9. **Constructors.**
   - `NewClient(contactEmail)` is unchanged, so `cmd/server/main.go` only had a comment updated.
   - `newClientWithBaseURL` is now a Nominatim only test seam (Photon off), so every existing
     client and search plan test runs unchanged.
   - `newClientWithProviders` takes both URLs and both limiter rates.
10. **Policy page and README** name Photon (see the sentences below).
11. **Swagger.** The descriptions of both routes now say Photon first and Nominatim as the fallback,
    regenerated with `swag init -g internal/api/router.go -o docs`. Only the description text changed.

## The ranking rule

A **place** is a Photon answer whose OpenStreetMap key is one of these:

- `place`, `boundary`, `highway`, `landuse`, `building`.

These cover towns, districts and localities, roads, named neighbourhoods and houses. Everything
else (shop, amenity, office, tourism, railway and so on) is a point of interest.

A place is **lifted** to the top when its name is a close spelling of the typed text. Case,
punctuation and spacing are ignored. Close means an edit distance (counted in letters, so Telugu
and Devanagari work) of at most:

| Typed text length | Letters that may differ |
| --- | --- |
| under 5 letters | 0 (the exact name only) |
| 5 to 8 letters | 1 |
| over 8 letters | 2 |

Lifted places come first, then everything else. Each group keeps Photon's own order (which already
favours places near the pin). Then repeated labels are dropped and the list is cut to 5.

Why this rule and not "places always above shops":

- In "Hyderbad" the shop named `Hyderbad` is itself the exact match for the typo. So "the match
  is fuzzy" has to be judged from the place's side: Hyderabad is one letter from what was typed.
- A point of interest is never lifted and never pushed down. So an exact hospital, school or
  relief camp name keeps the place Photon gave it. A test pins `Apollo Hospital` staying above
  `Apollo Hospital Road`.
- When a place and a point of interest share an exact name (`Ameerpet` the area and a station
  called Ameerpet), the place comes first.
- Short texts need the exact name, or every three letter word would be close to dozens of places.

Tests: `TestPhotonRanksCloseSpellingsOfPlacesFirst` (Hyderbad, Apollo Hospital, Ameerpet, Gachib,
Abid) and `TestCloseSpelling`.

## The limits and why

| Limit | Value | Why |
| --- | --- | --- |
| Photon limiter | 2 a second, burst 2, whole process | Photon's public server asks only for a reasonable number of requests and is meant for search as you type. One person typing with the 600 ms pause never waits, and many people cannot flood it. |
| Report form wait for a Photon token | 0.5 s, then Nominatim | A busy Photon should not make someone filing a report wait longer. |
| Photon time limit | 2 s per request | A slow Photon must leave time for the Nominatim fallback inside the report form's 8 s limit. |
| Photon rows asked for | 10, cut to 5 shown | The India filter and the dropped repeats still leave five. |
| Requests per search | 4 in total across both services | Plan item 4 and the 261016-adr cap. A search that asked Photon has 3 Nominatim tries left. With a pin: full text in the box, full text widened, first shorter text in the box. Without a pin: full text and the two shorter texts. A Photon answer served from the cache costs nothing, so the Nominatim plan then gets all 4. |
| Nominatim | unchanged: 1 a second, bounded 1.5 s wait per try | Nominatim's usage policy. |

Worst case on the report form is about 0.5 s (Photon wait) + 2 s (Photon time limit) + about 2 s
of Nominatim limiter waits + network for 3 Nominatim tries. That is roughly 5 to 6 s, inside the
8 s client timeout. Live, the Flat 4B address, which went all the way to Nominatim, took 3.53 s.

**Caches.**

- Photon has its own cache (same type, 256 entries, 6 hours, the same key of the text plus the
  area on a 0.1 degree grid). Both searches share it, since a Photon answer does not depend on
  who asked.
- A Photon "nothing found" is cached too, so the next identical search skips Photon and goes
  straight to the lane's Nominatim cache.
- A Photon error is never cached.
- The lanes' Nominatim caches work as before, with one change: an empty Nominatim answer that
  followed a Photon **failure** is not cached. Photon may be back on the next search, and an empty
  answer must not hide it for six hours.

**D-16 (a place search never queues ahead of a report search).**

- On Photon's limiter the place search takes a token only if one is free right now. This is the
  same rule as its first Nominatim try, now shared as `takeReaderTokenFrom`.
- When none is free the place search answers busy at once (the existing "Search unavailable. Try
  again in a moment.") and does not go on to Nominatim. A busy Photon never moves place searches
  onto the Nominatim budget that report searches fall back to. This is a choice the plan left open.
- `TestReaderNeverQueuesAheadOfAReporterOnPhoton` proves it. With a report search waiting for
  Photon's next token, a place search returns busy in under 100 ms and sends nothing to either
  service.
- The existing D-16 tests (`TestSearchReaderNeverWaits`, `TestSearchReaderDoesNotStarveSearch`,
  `TestReaderFollowUpNeverQueuesAheadOfAReporter`) pass unchanged.

## New Policy sentences

In "What Pinalert keeps", the map paragraph now reads (new sentences in the middle):

> The map is drawn with files from unpkg.com and map tiles from OpenFreeMap, or from OpenStreetMap
> on devices that can't draw the vector map. A typed place search goes through Pinalert to Photon,
> a search service run by komoot. Pinalert sends what you typed and the position of your pin or the
> middle of the map. If Photon can't answer, the search goes to OpenStreetMap's Nominatim service
> instead. The places in the results come from OpenStreetMap. Those services see the request the
> way any website does.

`TestLegalPagesRender` now pins the whole sentence run from "A typed place search" to "the way any
website does". Before, nothing pinned this paragraph. The "Last updated" date was already
16 October 2026.

README (step 2 of setup):

- It now says both searches ask Photon first, because it matches close spellings and half typed
  words.
- It says they fall back to Nominatim only when Photon fails, takes over 2 seconds or finds nothing.
- It says `NOMINATIM_CONTACT_EMAIL` goes into the User-Agent both services receive.

## Live check

`GEOCODE_LIVE=1 go test ./internal/geocode/ -run TestGeocodeLive -count=1 -v`
(`internal/geocode/live_test.go`, skipped unless `GEOCODE_LIVE=1`).

- It was run once, on 2026-10-16, through the report form's `Search`.
- Both limiters were set to one request a second, with a 1.2 s pause between searches.
- Hyderabad pin: 17.385, 78.4867. Delhi pin: 28.6139, 77.2090.
- "First results" shows the first three labels.
- This run was **before** the repeated row fix (commit 097ca22). The doubled `Hyderabad, Telangana`
  and `Inner Ring Road` rows below now show once each. That was not re-run live; it is covered by
  `TestPhotonDropsRepeatedLabels`.
- In the Telugu row, one Nominatim label had an en dash in its OpenStreetMap data
  (`Medchal-Malkajgiri`). It is written here with a hyphen.

| Typed | Pin | Answered by | Took | Matched text | First results |
| --- | --- | --- | --- | --- | --- |
| Banjra Hills | Hyderabad | photon | 790ms | | Banjara Hills, Ward 92 Venkateshwara Colony, Telangana; Ward 93 Banjara Hills, Telangana; Banjara Hills Road Number 11, Gaffar Khan Colony, Ward 93 Banjara Hills, Telangana |
| Hyderbad | Hyderabad | photon | 340ms | | Hyderabad, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |
| Banjara Hi | Hyderabad | photon | 240ms | | Banjara Hills, Ward 92 Venkateshwara Colony, Telangana; Banjara Hills Road Number 7, Banjara Hills, Ward 92 Venkateshwara Colony, Telangana; Banjara Hills Road Number 8, Banjara Hills, Ward 92 Venkateshwara Colony, Telangana |
| Gachib | Hyderabad | photon | 280ms | | Gachibowli, Telangana; gachibowli rd, Kala Daira, Ward 28 Azampura, Telangana; Gachibowli - Miyapur Highway, Ward 104 Kondapur, Telangana |
| Road No 12 Banjara Hills Hyderabad | Hyderabad | photon | 320ms | | Plot No 70, Road No 12 Koushik Co-Operative Society, Banjara Hills, Bhavani Nagar, Ward 93 Banjara Hills, Hyderabad, Telangana; Flat No 33, Durga Enclave, Road No 12, Banjara Hills, Motilal Nehru Nagar, Ward 149 Begumpet, Hyderabad, Telangana; Plot No 36, Road No 12, Mla Colony, Banjara Hills, Doolapally, Hyderabad, Telangana |
| Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad | Hyderabad | nominatim | 3.53s | | nothing |
| Hyderabad | Delhi | photon | 490ms | | Hyderabad, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |
| Ring Road | Hyderabad | photon | 550ms | | Inner Ring Road, new balaji nagar, Jillelaguda, Telangana; Inner Ring Road, new balaji nagar, Jillelaguda, Telangana; Inner Ring Road, Khayyam Nagar, Ward 61 Attapur, Telangana |
| హైదరాబాద్ (Hyderabad in Telugu) | Hyderabad | nominatim | 1.55s | | University of Hyderabad, CyberZon Approach road, Lingampally, ... Telangana, 500019, India; Indian Institute of Technology Hyderabad, NH65, ... Sangareddy, Telangana, 502285, India; Jawaharlal Nehru Technological University, Hyderabad, ... Medchal-Malkajgiri, Telangana, 500085, India |
| Hyderbad | none | photon | 180ms | | Hyderabad, Telangana; Hyderabad, Telangana; Hyderabad, Zaheerabad, Telangana |

Read:

- The typo and prefix cases are all found by Photon in under a second: Banjra Hills, Hyderbad,
  Banjara Hi, Gachib.
- Hyderbad shows the city first, both with a pin and without one.
- Hyderabad from a Delhi pin: the first three rows are all Hyderabad, Telangana, none in Pakistan.
- The Road No 12 address finds plots on Road No 12 with built labels and no `null`.
- Ring Road near Hyderabad gives Hyderabad's Inner Ring Road.

## Files

- `internal/geocode/photon.go` (new): the Photon request, India filter, labels, close spelling,
  ranking, repeated row drop, Photon limiter rules and `askPhoton`.
- `internal/geocode/client.go`: the client's Photon fields, `newClientWithProviders`, the shared
  `search` order for both lanes, the request budget passed to `run`, `Answer.provider` (unexported,
  never in the JSON) and `takeReaderTokenFrom`.
- `internal/geocode/photon_test.go` (new): 18 top level tests.
- `internal/geocode/live_test.go` (new): the live check.
- `internal/api/handlers/geocode.go`, `place.go`: swagger descriptions and the package comment.
  There is no handler code change.
- `internal/api/handlers/legal_e2e_test.go`: pins the new Policy sentences.
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`: regenerated.
- `web/templates/policy.html.tmpl`, `README.md`, `cmd/server/main.go` (comment only).
- Not touched: `web/static/js/report-form.js`, `web/static/js/queue.js` and every browser file.

## Commits

| Commit | Step |
| --- | --- |
| 6993dd0 | test: failing client tests for Photon first |
| 61da74e | feat: Photon first, Nominatim as fallback |
| 81c4780 | test: Policy must name Photon |
| a92a2eb | docs: Policy page and README |
| 5912b1f | docs: route descriptions, swagger regenerated |
| a0fce37 | test: live check behind GEOCODE_LIVE=1 |
| 45ffe72 | test: failing test for repeated rows |
| 097ca22 | fix: drop rows that repeat a label |

## Tests

- `go build ./...` and `go vet ./...`: clean. `gofmt -l internal/`: nothing.
- `go test ./internal/geocode/ -race -count=3`: ok. On a single `-v` run there are 45 top level
  `--- PASS` (27 before), 48 subtests and 1 `--- SKIP` (`TestGeocodeLive`).
- `DATABASE_URL=postgres://localhost:5432/p4_quick_pho?sslmode=disable go test ./... -p 1 -count=1 -v`:
  - run on a freshly created `p4_quick_pho`, dropped afterwards;
  - all 13 packages ok, 2960 `--- PASS`, 0 `--- FAIL`, 2 `--- SKIP`;
  - the skips are `TestGeocodeLive` (needs `GEOCODE_LIVE=1`) and `TestScreeningFixture` (needs
    recorded OpenAI scores). Neither needs a database, so there are 0 database backed skips.
- `node --test web/*.mjs`: 36 pass, 0 fail.
- `go test ./web/ -count=1`: ok.
- No em or en dash in any added line (`git diff 9cf3349 | grep '^+'` finds none).

## Left open

- **Telugu and Hindi script.** Photon found nothing for `హైదరాబాద్`, so the search fell to
  Nominatim, which gave universities with Hyderabad in their name, not the city. The likely cause
  is that Photon's data holds English, German, French and the local `name` tag only (this is what
  Photon's README says of the published dumps; the public server was not checked), and
  Hyderabad's `name` tag is English. Whether leaving out `lang=en` changes this was not tested,
  since the live check runs once. Script searches work only as far as Nominatim's answers do.
- **The live check ran through the report form's search only.** The map search shares the same
  code path and is covered by the unit tests on both lanes and by browser check item 6.
- **The Flat 4B address still finds nothing.** Photon misses the whole text. The Nominatim fallback
  now has 3 tries, so it never reaches `Road No 12, Banjara Hills, Hyderabad`, the text that would
  have matched. Photon itself finds that shorter text (row 5).
  - A likely next step is to let Photon try the shorter texts before Nominatim. That counts against
    the cap of 4, so it is a design change for the owner, not made here.
- **Labels without the city.** Some Photon rows in Hyderabad carry a ward as the district and no
  city, for example `Banjara Hills, Ward 92 Venkateshwara Colony, Telangana`. Adding Photon's
  `county` part might bring back "Hyderabad". The plan's part list left it out, and it was not
  checked live.
- **No browser check was done.** No server was started, per the task rules.
- The browser's 600 ms pause is unchanged (out of scope in the plan).

## What a browser check should try

1. Report form, pin in Hyderabad:
   - Type `Banjra Hills`. Expect Banjara Hills first, no "No exact match" line, and a tapped row
     moves the pin.
   - Then type `Banjara Hi` and `Gachib`. Each should answer in about a second.
2. Report form, type `Hyderbad`. Expect `Hyderabad, Telangana` first, listed once, above any shop.
3. Report form, pin in Delhi, type `Hyderabad`. Expect Hyderabad, Telangana, never Pakistan.
4. Report form, type `Road No 12 Banjara Hills Hyderabad`. Expect plots on Road No 12 with labels
   that have no `null` and no empty commas. A row with no name shows its full label as the main
   text.
5. Report form, type the Flat 4B address. Expect "No matches found." in about 4 s, and no "Search
   unavailable".
6. Map search: type `Ring Road`. Expect Hyderabad's Inner Ring Road when the map is on Hyderabad.
   Choosing it moves the map and the "Showing reports near ..." line uses the name.
7. Type quickly in pauses on both searches. No "Search unavailable" should appear from your own
   typing.
8. Policy page: the map paragraph names Photon and Nominatim, in light and dark, at 320 px.
