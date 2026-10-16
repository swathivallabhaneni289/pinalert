---
quick_id: 261016-sfu
created: 2026-10-16
source: open items of quick 261016-pho and the owner's request that typing a place is quick
status: planned
---

# Search follow ups: whole addresses, city in labels, quicker suggestions

## Why

Quick 261016-pho made Photon the first search and Nominatim the backup. Its live check left gaps
against the owner's request ("let them type their whole address", "if it takes forever to type, it's
not going to work out"):
- A whole flat address ("Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad") still finds
  nothing: Nominatim's shorter tries never reach "Road No 12, Banjara Hills, Hyderabad", which Photon
  does find.
- Some Photon labels have a ward but no city ("Banjara Hills, Ward 92 ..., Telangana").
- The report form waits 600 ms after typing stops before it searches, and the map's place search only
  searches on Enter or the button.

## What changes

1. Search order inside the cap of four upstream requests: Photon with the whole text, then Nominatim
   with the whole text (it is better at exact full addresses), then Photon with the first shorter text,
   then Photon with the second shorter text. Stop at the first answer that finds something. A shorter
   text that matched sets `matched_query`, so the "No exact match for the full address. Showing places
   for: ..." line shows. Keep every rule of 261016-pho otherwise: India only, the ranking, the caches,
   the per service limiters and time limits, a place search never queuing ahead of a report search
   (D-16), a busy place search answering busy at once.
2. Labels: when a Photon row has no city, use its county (or the nearest larger place part Photon
   sends) so the label names the city; still no "null", no empty or repeated parts.
3. The report form's pause before searching drops from 600 ms to 300 ms (`SEARCH_DEBOUNCE_MS`).
4. The map's place search also suggests while typing: after a 300 ms pause and at least 3 letters it
   runs the same search and shows the same result list; Enter and the button still work; a newer search
   aborts the older one; no "Search unavailable" from the person's own typing; the screen reader hears
   the result count the way it does today after Enter.

## Done when

Unit tests pin the new order, the cap and D-16; label tests pin the county case; contract tests pin
300 ms and the map search's typing path; node tests if the map search logic has a pure part.
The GEOCODE_LIVE check (one request a second to each service) shows the Flat 4B address finding a
Road No 12 row with the "No exact match" line. `go build`, `go vet`, the full suite on a fresh database,
`node --test web/*.mjs` and `go test ./web/` pass. No em or en dash added.
