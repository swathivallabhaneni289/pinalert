---
quick_id: 261016-tap
created: 2026-10-16
source: backlog 999.6 (owner, Phase 2 UAT 2026-10-06) and the owner's 2026-10-16 note that the position must be accurate
status: planned
---

# Tap the map to move the report pin, in every case

## Why

Owner, by voice, 2026-10-16: people report things outside, on a road or by a building; the flat
number doesn't matter, but the position should be accurate. Search now gets a person to the right
road or area (quick tasks 261016-adr, -pho, -sfu); the last step is putting the pin on the exact spot.
On the report form's small map the pin can only be dragged while location is on, and a tap moves it
only when location is off (`showLocationDenied` in `web/static/js/modal.js`). The owner asked for tap
to move the pin back on 2026-10-06 (999.6, finding F1 in 02-UAT.md). The web review also found that
the tap handler is added again on every open where location is off (IN-W10 in 04-REVIEW-WEB.md).

## What changes

1. A tap or click anywhere on the report form's map moves the pin there, whether location is on, off
   or denied, and does the same follow up work a drag does today (coordinates, any accuracy or
   "approximate" state, the place line, whatever the drag handler updates).
2. The map click handler is registered once per map, never again on each open (IN-W10).
3. Dragging keeps working as today.
4. After a search result is chosen, the map is close enough to place the pin on a road (street level
   zoom) if it isn't already; read where the pick is handled and change it only if it lives in
   modal.js.
5. If the form's hint about the pin says only "drag", it says "tap or drag" in plain words, and the
   copy tests follow.

## Done when

Contract tests pin one registration of the click handler and that it runs in every location state;
node tests with a fake Leaflet if the repo has that pattern; `go test ./web/`, `node --test web/*.mjs`,
`go build`, `go vet` and the full suite on a fresh database pass. No em or en dash added.
