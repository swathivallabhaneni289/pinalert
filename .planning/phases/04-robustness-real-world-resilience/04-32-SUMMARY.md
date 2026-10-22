---
phase: 04-robustness-real-world-resilience
plan: 32
subsystem: geocode, place search, report form search, Nearby map
status: complete
tags: [geocode, photon, nominatim, place-search, leaflet, gap-closure, uat-test-11]
requires: []
provides:
  - near only place answers (nearRadiusKm = 50) when the search knows a point
  - one plain nothing near sentence on Nearby and in Post a report
  - a calm place mark at the picked place on the Nearby map, at street zoom 15
affects:
  - internal/geocode (Search, SearchReader; the home area search is left as it was)
  - web/static/js/map-tools.js, map.js, report-form.js, css/main.css
tech-stack:
  added: []
  patterns:
    - far rows dropped after Photon's own top five, never before
    - one word shorter texts skipped only when a position is known
key-files:
  created:
    - internal/geocode/near.go
    - internal/geocode/near_test.go
    - web/place_near_contract_test.go
  modified:
    - internal/geocode/client.go
    - internal/geocode/photon.go
    - internal/api/handlers/geocode.go
    - internal/api/handlers/place.go
    - docs/docs.go, docs/swagger.json, docs/swagger.yaml
    - web/static/js/map-tools.js
    - web/static/js/map.js
    - web/static/js/report-form.js
    - web/static/css/main.css
    - tests listed under Amended tests
decisions:
  - "nearRadiusKm = 50 sits with the other search constants in internal/geocode/client.go"
  - "A far city or town whose name is a close spelling of the typed text is kept, so the map can still jump to another city by name (HIST-05)"
  - "Far rows are dropped after Photon's ranking keeps its five: rows six to ten are fragment guesses"
  - "With a position, a shorter text of one word is not tried ('Mall' from 'PVP Square Mall')"
  - "Nearby sends the map centre only at zoom 8 or closer (NEAR_MIN_ZOOM); further out the search stays India wide"
  - "No Nominatim fallback added: it is already in the chain, and it does not hold PVP Square either"
metrics:
  duration: about 45 minutes
  completed: 2026-10-23
---

# Phase 4 Plan 32: Place search near first, a mark at the picked place Summary

Search a place on Nearby and in Post a report now answers only with places within 50 km of the person's point. When nothing near matches, it says one plain sentence. Picking a place on Nearby moves the map to street zoom 15 and draws a calm map-pin mark there.

## What was built

**Server (internal/geocode).**
- `nearRadiusKm = 50.0` sits in the search plan constant block of `client.go`.
- `near.go` holds `answerable`, `keepNear` and `textsFor`.
- When the search has a point and is not the home area search, rows farther than 50 km are dropped. This applies to:
  - Photon's answer, after its ranking keeps five;
  - every Nominatim try, both in the fallback after Photon found nothing and in the Nominatim only plan.
- An answer with only far rows now counts as nothing found, so the existing order runs: Nominatim with the whole text, then the shorter texts.
- With a point, a shorter text of one word is not tried.
- With no point, and on the home area search (kind=area), nothing changes.
- The swag descriptions of both endpoints say this, and the docs are regenerated.

**Nearby (map-tools.js, map.js, main.css).**
- `RESULT_ZOOM` moves from 13 to 15, so a pick opens at street level.
- A pick calls `PinalertMap.setPlaceMark`, which draws a Leaflet `divIcon` holding the `map-pin.svg` glyph.
  - The mark is in `--color-text`, with a thin `--color-bg` halo so it shows on any tile.
  - It has `interactive: false`, `keyboard: false` and `pointer-events: none`, sits under report pins (`zIndexOffset -1000`) and is `aria-hidden`.
- `clearPlaceMark` runs when Back to my location lands, next to `Pinalert.clearPlace()`. A new pick replaces the old mark.
- The search sends the map centre only when the map zoom is 8 or closer.
- An empty answer with a centre sent reads "No place by that name near you. Try another name."
  - It shows in the existing `role="status" aria-live="polite"` line.
  - The first key typed after it clears it.

**Post a report (report-form.js).**
- When the pin was sent, an empty answer reads "No place by that name near you. Try another name, or tap the map to put the pin there."
- The Low data form, which has no map, ends the sentence at "Try another name."
- With no pin, the line stays "No matches found.", because "near you" would not be true.
- Typing again clears the line.
- A pick already moved the form's pin through `chooseSearchResult` (`placeMarker` plus `setView`). This is now proven by a contract test and the browser check, and the code is unchanged (truth 5).

## Before and after: the item 1 examples

These were measured near Benz Circle (16.4999, 80.6567) through `/api/geocode/place`, on scratch server 8785. "Before" is untouched main at 130b134.

| Typed | Before | After |
| --- | --- | --- |
| Kanaka Durga Temple | 5 rows: the temple and its ghat road in Vijayawada, then temples of that name at Gundugolanu (70 km), Odalarevu (140 km), Unduru (170 km) | 2 rows: the temple, then the ghat road, in that order |
| Prakasam Barrage | 4 rows: the barrage, then 3 Mangalagiri to Prakasam Barrage Main Road rows | the same 4 rows, same order |
| Babai Hotel | 5 rows: 3 in Vijayawada, Kaza (30 km), Bachupally, Hyderabad (250 km) | 4 rows: the 3 in Vijayawada, then Kaza |
| Barbeque Nation | 5 rows: Vijayawada, then Nagpur and 3 in Hyderabad | 1 row: Barbeque Nation, Nandhamuri Road, Vijayawada |
| PVP Square | 5 far offices: Mumbai, Coimbatore, Indore twice, Thiruvananthapuram | empty list, so the page shows the sentence |
| PVP Square Mall | Vasant Square Mall (Delhi), DB Mall Square (Bhopal), Goodwill Square Mall Road (Pune), Park Square Mall and Market Square Mall (Bengaluru) | empty list, so the page shows the sentence (see the note below) |

The found places stay found, in the same order, near first.

Note on "PVP Square Mall": live calls were capped at 30, so it was not measured once more after the last fix. Its "after" comes from two things:
- **The live trail of the run before that fix.** The whole text and "Square Mall" found nothing near. Only the one word "Mall" found anything: villages named Mallapalle and Mallavaram, returned with matched_query "Mall".
- **A test that reproduces that trail with the live rows.** `TestPVPSquareMallNearBenzCircleFindsNothing` shows the one word try is no longer sent and the answer is empty.

"PVP Square" was measured live on that same build. It was already empty there, and the last fix only drops its "Square" try, which had found nothing.

## Nominatim fallback

None was added.
- Nominatim is already in the chain: it runs with the whole text whenever Photon finds nothing. Since this plan, that includes when Photon finds only far rows.
- Measured on 2026-10-23 with Nominatim alone, viewbox around Benz Circle, India only:
  - "PVP Square": 0 rows
  - "PVP Square Mall": 0 rows
  - "PVP Mall": 0 rows
- The place is not in OpenStreetMap, so no free provider on that data will find it. The honest answer is the plain sentence, which points the person at another name or at a tap on the map.

## Live call tally

About 28 live upstream requests in total, under the cap of 30:
- 6 for the before answers;
- 3 for the Nominatim probe;
- 2 for a mistyped scripted search ("after-place");
- 6 and 11 for two after runs.

These are counted from the search order, not logged one by one. A cold first Photon answer gets one retry, so if any first try was slow, the true number is one higher for each such search.

Every browser check answered `/api/geocode*` from fixed JSON through Playwright `page.route`, so no browser search reached Photon.

## Verification

- `go build ./...` passed, and `go vet ./...` passed.
- `node --test web/*.mjs`: 638 tests, 638 pass, 0 fail.
- Full Go suite on p4_w32 (`env -i ... go test ./... -p 1 -count=1 -v`), exit 0:
  - 3936 `--- PASS` lines, 1361 of them top level;
  - 0 `--- FAIL`;
  - 2 `--- SKIP`, TestGeocodeLive and TestScreeningFixture, the two allowed.
  - TestCastVoteConcurrentSameAccountKeepsEveryRow passed first time.
- Browser check with Playwright, Chromium and WebKit, at 390x844 and 1470x830: 85 checks, 85 pass. Each engine and size covered:
  - Nearby: the sentence shows, is a polite live region, sends the centre, and is cleared by typing again.
  - The mark: one mark inside the map, in the text colour, `pointer-events none`, `aria-hidden`, no tab stop.
  - A report pin 15 m from the place is the element on top at its point, the mark takes no point, and a click on the pin opens the report.
  - Back to my location takes the mark away.
  - Post a report with location on and off: the right sentence; the pin sent only when there is one; typing clears the line; a pick moves the pin to 16.51734, 80.62542; a tap on the map places the pin there.
- **Not checked: dark theme.** Every browser run used the default light scheme. In dark mode the mark is `--color-text` (near white) with a 1 to 2 px `--color-bg` halo. How it reads on the dark basemap still needs a look.

## Amended tests

Each amended test carries an "Amended 2026-10-23 by plan 04-32 (owner)" note.

1. `internal/geocode/client_test.go`, TestSearchReaderBiasAndBucket: the other bucket point moved from latitude 12.5 to 12.7. At 12.5 the stand in row is 54 km away and is now dropped. At 12.7 it is 31 km away and still in another bucket.
2. `internal/geocode/search_plan_test.go`, TestBoxFirstThenWidened: the widened row is Banjara Hills, a neighbourhood 1,260 km from the Delhi position, so the answer is now empty instead of one row. The order of the two requests is unchanged.
3. `web/address_search_contract_test.go`, TestPartialMatchLineInBothSearches:
   - `renderResults(list, matched, near)` and `renderDropdown(results, matched, near)` are located with their new third argument;
   - the cached render pin is now `renderDropdown(hit.results, hit.matched, !!pin)`.
4. `web/low_form_contract_test.go`, TestLowDataSearchIgnoresServerText: `renderDropdown` is located with its new signature.
5. `web/map_tools_contract_test.go`, TestPlaceSearchScript and the pick order test:
   - `RESULT_ZOOM` moves from 13 to 15, and the comment says zoom 15;
   - `setPlaceMark` joins the pick order;
   - `renderResults` is located with its new signature;
   - `NEAR_MIN_ZOOM` and the near sentence are pinned.
6. `web/place_search_typing_test.mjs`: the fake map gains `getZoom`, `setPlaceMark` and `clearPlaceMark`, and records moves and marks.
7. `web/report_search_retry_test.mjs`: `setup` takes `pin` (default true), so a case can have no pin.

New tests:
- In `internal/geocode/near_test.go`:
  - the Benz Circle rows;
  - rows six to ten stay hidden;
  - near rows of the first five keep their order;
  - an answer with only far rows is nothing found, and Nominatim is asked once;
  - no position stays India wide;
  - a far city named as typed is kept, through Photon and through Nominatim;
  - answerable;
  - the radius;
  - textsFor;
  - the PVP Square Mall trail.
- `web/place_near_contract_test.go`: the sentences, the place mark, and the report form pick.
- 4 node tests in `place_search_typing_test.mjs` and 4 in `report_search_retry_test.mjs`.

## Deviations from Plan

### Auto-fixed issues

**1. [Rule 1, bug] Far rows dropped after Photon's top five, not before**
- **Found during:** the first live after run.
- **Issue:** dropping far rows before the ranking cut brought up Photon's rows six to ten. These are fragment guesses: Krishna Square for "PVP Square", and villages named "Mall..." for "PVP Square Mall". The cut had always hidden them, and they would have hidden the sentence.
- **Fix:** `keepNear` runs on `rankPhoton`'s five.
- **Trade off:** a near row that Photon ranks below five far ones is not shown. Photon's own position bias makes that rare.
- **Commit:** e415bbd.

**2. [Rule 1, bug] With a position, a shorter text of one word is not tried**
- **Found during:** the second live after run.
- **Issue:** once far only answers count as nothing found, the existing shorter tries ran. "PVP Square Mall" went on to "Square Mall" and then to "Mall", which found villages named Mallapalle and Mallavaram.
- **Fix:** `textsFor` keeps shorter texts of two or more words when the search has a position. Searches with no position, and the home area search, are unchanged.
- **Commit:** e415bbd.

### Choices beyond the plan's wording

**3. A far city or town named as typed is kept (truth 2).**
- **Rule:** a row past 50 km is kept only when it is a city or town (`cityLevel`), and its name is a close spelling of the typed text by the existing `closeSpelling`.
- **Why:** the Nearby place search exists to jump the map to another area (HIST-05). Without the exception, "Hyderabad" typed in Vijayawada would say "No place by that name near you." A city named as typed is never a look alike of something near.
- **Pinned by:** TestFarCityNamedAsTypedIsKept and TestAnswerable.

**4. NEAR_MIN_ZOOM = 8 on Nearby.**
- The map centre counts as the person's point only when the map shows a local area, about a 250 km wide view on a phone.
- A map zoomed out to the whole country is centred on no place anyone is in. Its search stays India wide, and an empty answer keeps the old "No matches found" line.
- With location off, the map starts at the account's home area, which is a real point, so the cut applies there.

**5. The home area search (kind=area) is exempt.** A home area is often far from where the person stands, and that search has its own typed row and Not found rules.

**6. The mark goes when Back to my location lands, not when it is pressed (truth 4).**
- It is removed in the same step that drops the place, when the position arrives.
- During the geolocation wait, which can take up to 8 seconds, the mark stays. So does the place line that still names the place, so the two never disagree.
- A pick made during the wait replaces the mark.

**Callers covered.** Only `map-tools.js` and `report-form.js` send a position. `settings.js` and `welcome.js` use kind=area, which is exempt, so no other page shows an old empty line for a near only answer.

## Notes for the orchestrator

- The scratchpad folder is shared with plan 04-31's agent.
  - It overwrote my first `serve.sh` and `signin.sh` at the scratchpad root, so I moved to a `w32/` subfolder for everything after that.
  - Its root files were left alone.
- While cleaning up a hung browser check, I ran `pkill -f "ms-playwright.*webkit"`. If 04-30 or 04-31 had a WebKit run going at 00:41 IST, it may have been ended and should be rerun.
- From about 00:10 to 00:21 IST, my first scratch server also wrote to the shared root `server.log`. 04-31's `serve.sh` writes to the same path, so whichever server started second emptied the other's log. If 04-31 had sign in failures in that window, this may be why.
- `gofmt -l` flags `internal/geocode/photon.go` and `internal/api/handlers/place_test.go`. Both were already flagged on main at 130b134, so they are out of scope and left as they were.
- These steps were skipped on purpose, per the caller's rules, and are left to the orchestrator:
  - STATE.md updates;
  - the ROADMAP.md progress tick;
  - requirement marking (the plan lists none).
- Nothing was pushed.

## Known Stubs

None.

## Self-Check: PASSED

- Files: `internal/geocode/near.go`, `internal/geocode/near_test.go` and `web/place_near_contract_test.go` exist.
- Commits on branch worktree-agent-a817278fdf033b3b6:
  - cc85203: place search answers only places near a known point;
  - 7e2d3e2: a picked place is marked at street level, and an empty near answer says so;
  - e415bbd: no fragment guesses when nothing near matches.
