---
quick_id: 261016-tap
status: complete
completed: 2026-10-16
source: backlog 999.6, F1 in 02-UAT.md, IN-W10 in 04-REVIEW-WEB.md
commits: [6c869f8, fa6b458]
---

# Quick task 261016-tap: tap the map to move the report pin, in every case

A tap or click on the report form's small map now puts the pin there whether location is on, off
or denied. Before this, a tap only worked when location was off, and every open with location off
added the tap handler again.

## What changed

1. **Tap works in every location state** (6c869f8). The map's click handler moved out of
   `showLocationDenied` and into `initLocation`'s one time map build. It's a named function,
   `placeMarkerAtTap`, and it does what a drag does: the form counts it as an edit
   (`markFormTouched`) and the coordinate readout moves (`placeMarker` updates it). The handler has
   no flag on it. IN-W10's suggested flag belonged to the old "tap only when location is off"
   design, and adding one would bring F1 back.
2. **The handler is added once per map** (IN-W10). Reopening the form any number of times, in any
   mix of location states, still leaves one handler. One tap builds one pin and counts as one edit.
3. **Dragging works as before.** Every pin is still built by the one `placeMarker`, so a tapped
   pin can be dragged too. A tap on the pin itself never reaches the map, because Leaflet markers
   don't pass their clicks on, and Leaflet sends no click after a drag of the map or the pin.
4. **A tap before the position answers is kept.** The existing `hasMarker()` checks in the GPS
   callbacks (D-20) mean a tap made while the browser is still finding the position wins. If the
   position then fails, the "location is off" notice stays hidden because a pin is already set.
5. **Search result zoom: no change needed.** `chooseSearchResult` already sets zoom 16
   (`SEARCH_RESULT_ZOOM`), which is street level and the same zoom a GPS fix uses. A test now pins
   that a search pick lands at 16 and that a tap then moves the pin.
6. **Copy** (fa6b458). The missing location message in modal.js shows only when no pin is set, so
   naming a drag was wrong there (there's no pin to drag). It now reads:
   `Set a location by searching, tapping the map, or allowing location access.`
   This differs from the plan's literal "tap or drag". The form calls `missingMessage()` only
   behind `if (!picker.isSet())` (report-form.js line 894), so a pin never exists when it shows.
   If the owner wants the plan's wording anyway, it's one edit in modal.js plus the two tests:
   `Set a location by searching, tapping the map or dragging the pin, or allowing location access.`
   The location off notice in `index.html.tmpl` already said "tap the map to drop a pin", so it's
   unchanged. No CSS or template change.

## Files

- `web/static/js/modal.js`: `placeMarkerAtTap`, the click handler in the map build, the handler
  removed from `showLocationDenied`, the new message, comments.
- `web/tap_to_place_contract_test.go` (new): `TestReportMapTapMovesPinInEveryState` pins exactly
  one `.on('click'` in modal.js, inside the `if (!modalMap) {` build, none in
  `showLocationDenied`, `startLocation`, `clearMarker` or `chooseSearchResult`, and that the tap
  calls `markFormTouched()` and `placeMarker(e.latlng.lat, e.latlng.lng)`.
- `web/report_map_tap_test.mjs` (new): runs the real modal.js in a Node sandbox with a fake
  Leaflet and fake geolocation. Nine cases: location on, denied and missing; four reopens across
  states keep one map and one handler; a denied open then a granted open; a tap before the
  position answers (both success and failure); dragging; the missing location message; a search
  pick then a tap. Run against the old modal.js, four of them fail (location on, reopens, early
  tap, drag after tap).
- `web/js_contract_test.go`: the copy check moves to the new sentence; both older sentences are
  on its stale list.

## Test commands

- `go build ./...`: ok
- `go vet ./...`: ok
- `go test ./web/ -count=1`: ok (346 top level `--- PASS` lines with `-v`, subtests not counted,
  0 FAIL, 0 SKIP)
- `node --test web/*.mjs`: 77 tests, 77 pass, 0 fail, 0 skipped (9 of them new)
- `DATABASE_URL=postgres://localhost:5432/p4_quick_tap?sslmode=disable go test ./... -p 1 -count=1 -v`
  on a freshly created `p4_quick_tap` (dropped afterwards): every package ok, 2998 `--- PASS`
  lines counting subtests, 0 FAIL, 2 `--- SKIP` lines, both opt in live checks and not database backed
  (`TestGeocodeLive` needs GEOCODE_LIVE=1, `TestScreeningFixture` needs recorded moderation
  scores). 0 database backed skips.

## Left alone, for the owner to decide

- `internal/service/report.go:271` and `:274`, the server's own validation message, still says
  "Set a location by dragging the pin or allowing location access." It's outside this task's
  file list. The browser blocks a post with no pin before it reaches the server, so people rarely
  see it, but the wording could match the new one in a later quick task.
- `report-form.js` and `map-tools.js` were not touched (another agent is editing them).

## What a browser check should try (phone widths 320, 360 and 390 px first)

1. Location on: open the form, wait for the pin, tap somewhere else on the small map. The pin
   jumps there and the numbers under the map change.
2. Location denied (or blocked in the site settings): the notice shows; tap the map and a pin
   drops.
3. Deny, close the form, allow location, open again, then tap: the pin moves once. Repeat the
   open and close a few times; each tap still moves one pin.
4. Pan the map with a finger or the mouse: the pin must not jump when the drag ends.
5. Drag the pin: it lands where it's dropped, with no extra jump.
6. Tap on the pin itself: nothing moves.
7. Tap right after opening, before the position answers: the tapped spot stays when the
   position comes in.
8. Search a road, pick a result: the map is at street level and a tap puts the pin on the road.
9. Double tap to zoom on a phone: the pin moves to the tapped spot and the map zooms. That's
   expected; say so if it feels wrong.
10. No pin: with location denied and no tap, press Post. The message reads "Set a
    location by searching, tapping the map, or allowing location access."
11. Pinch zoom with two fingers: the pin must not jump when the fingers lift.
12. Do the taps on a normal phone that draws the vector map (OpenFreeMap), not only in the
    plain tile fallback. The node test's fake Leaflet runs the fallback path only.
