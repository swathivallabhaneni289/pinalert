---
quick_id: 261016-pho
created: 2026-10-16
source: owner request after quick 261016-adr (address search must match close spellings, fast, no errors)
status: planned
---

# Search that matches close spellings and half typed words

## Why

Owner, by voice, 2026-10-16: "The search should work exactly accurate. If they type something close,
it should match it. Make sure we have no errors on that part, because if it takes forever to type,
it's not going to work out."

Live checks on 2026-10-16 (India only, near Hyderabad):

| Typed | Nominatim (today) | Photon (photon.komoot.io) |
| --- | --- | --- |
| Banjra Hills (typo) | nothing | Banjara Hills, Telangana |
| Banjara Hi (half typed) | Banjara Nagar (wrong) | Banjara Hills |
| Hyderbad (typo) | shops named "Hyderbad" | a shop first, Hyderabad second |
| Road No 12 Banjara Hills Hyderabad | Road No. 12, Banjara Hills | plots on Road No 12 (name empty, label must be built) |
| Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad | nothing | nothing (fewer words needed) |

Nominatim needs every word spelled right and its usage policy does not allow search as you type.
Photon is OpenStreetMap based, free, needs no key, matches close spellings and prefixes, and takes a
position to search near.

## What changes (server side; the JSON answers keep their shape)

1. Photon becomes the first provider for both searches (`Client.Search` behind `GET /api/geocode` and
   `Client.SearchReader` behind `GET /api/geocode/place`): `q`, `lat`/`lon` of the pin or map centre when
   known, `limit`, and the India box; every answer is then filtered to `countrycode == "IN"` (the box
   alone also covers parts of Pakistan, Nepal, Bangladesh and Sri Lanka).
2. Ranking: when the match is fuzzy, places, districts, localities, roads and houses rank above shops
   and other points of interest, so "Hyderbad" shows the city first. Use Photon's layer or osm_tag
   filters, or a light reorder in Go; a test pins the "Hyderbad" case.
3. Labels: house level answers come back with no name. Build the label from name, housenumber, street,
   locality, district, city and state, skipping missing parts, never printing "null" or empty commas.
4. Photon has its own limiter, a short timeout (about 2 seconds) and its own cache. Nominatim (with the
   India only, near first and fewer words logic of quick 261016-adr) is used only when Photon errors,
   times out or finds nothing. The cap of four upstream requests per search and the rule that a place
   search never queues ahead of a report search (D-16) still hold; their existing tests stay green.
5. `matched_query` and the "No exact match for the full address. Showing places for: ..." line keep
   working when the Nominatim fallback used a shorter text.
6. Privacy and credit: the Policy page names Photon (by komoot) wherever it names the place search
   provider today, since typed text and a position now go to a second third party; OpenStreetMap
   credit still covers the results; README gains one line on the provider choice and the fallback.
7. A live check, off by default (an environment variable such as `GEOCODE_LIVE=1`, like
   `MODERATION_LIVE`), runs about ten real queries and prints a table: Banjra Hills, Hyderbad,
   Banjara Hi, Gachib, the Road No 12 address, the Flat 4B address, Hyderabad from a Delhi pin, Ring Road
   near Hyderabad, and one name in Telugu or Hindi script. Run it once, respecting both services'
   limits, and put the table in the SUMMARY.

## Out of scope

The browser's pause before searching (600 ms today; a shorter pause is a separate one line change after
the review fixes merge). Tapping the map to move the pin (999.6). Self hosting Photon.

## Done when

Unit tests with `httptest` stand ins for both providers prove the order, the India filter, the ranking
case, the labels, the fallback on error, timeout and empty answers, the cap, D-16 and the cache. Handler
tests keep passing unchanged where the answer shape is unchanged. The live check table shows the typo and
prefix cases found. `go build`, `go vet`, the full suite on a fresh database, `node --test web/*.mjs` and
`go test ./web/` pass. No em or en dash added.
