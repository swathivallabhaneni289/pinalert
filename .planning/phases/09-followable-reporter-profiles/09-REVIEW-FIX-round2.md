---
phase: 09
part: round2
fixed_at: 2026-10-30
review_path: .planning/phases/09-followable-reporter-profiles/09-REVIEW.md
iteration: 2
base_commit: 58e58e36
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 9 code review fixes, round 2

**Sources:** 09-REVIEW.md (K2), 09-REVIEW-FIX-browser.md (K2, IN-B03), 09-REVIEW-FIX-server.md (the browser and docs handoffs).
**Database used:** p9_fix_round2_a (created for this run, dropped at the end).
**Checks at the last code commit:** `go test ./... -p 1 -count=1 -v` all packages ok, 0 FAIL, 2409 top level passes, only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped. `node --test web/*.mjs`: 1633 tests, 1633 pass.

## Fixed

### K2: the star box for the reader whose resolve closes the report

**Commits:** 37f88bad, 7e1a14b5 (a gofmt fix of one amended test line)
**Files:** `web/static/js/votes.js`, `web/static/css/stars.css`, `web/templates/index.html.tmpl`, `web/templates/profile.html.tmpl`, `web/votes_stars_test.mjs`, `web/rate_host_contract_test.go` (new), `web/person_contract_test.go`

- A persistent `<div id="rate-host"></div>` sits directly after the message (`#toast`) on index and profile (the Activity page). It has no role and no aria-live. `stars.css` places it: fixed, centred like the message (over the list pane from 900px, `.app-shell > #rate-host`), above the message while the message shows and at the message's own place when it does not (`#toast:not([hidden]) + #rate-host` sets the lift), z-index 1190 under the message's 1200, `:empty` takes no room, and the box inside drops its top margin. The phone base line is the message's own (above the bottom bar). Same rules on phone and laptop.
- `votes.js`: when `rateHostIsLive(block)` is false and `#rate-host` exists, `mountRateBox` mounts the box there (`offerRateBox` no longer gives up on a closed card when the host exists). A page without the host or without the star script shows nothing, as before. While the card is live the box still goes in the card.
- In the host the box is not a live region (no role or aria-live on the host or the box), mounting never moves focus, and the box goes away on Skip, on a 404 (focus goes to the message's Undo if it is still there), on Undo in the message or in the card (the existing `removeRateBox` paths), and after a rating.
- Decision to check: after a rating the box collapses to its one status line, as in the card, and leaves 6 seconds later (`RATE_HOST_LINE_MS`). The status line keeps its `role="status"` (and the error line its `role="alert"`) so a screen reader hears the result; both are empty until used. I did not call `showToast` for the result, because any later message empties the toast and would drop the resolve's Undo. If you want the box gone at the tap with no line, it is a one line change.
- Pages: index and profile carry the host. **Low data is left out**: `TestLowDataAssetPolicy` pins its exact stylesheet and script lists and a byte budget, and it loads neither `stars.js` nor `stars.css`, so a box could never be mounted there (the call returns early without the star script). **demo and demo_activity are left out** for the same reason (no `stars.js`, no account, a static page), so a host there would be dead markup. `TestRateHostIsBesideTheMessageOnPagesThatCarryStars` pins both choices and will fail if someone adds `stars.js` to one of them without a host.
- Tests: eight new cases in `votes_stars_test.mjs` (six fail against the old `votes.js`); the new Go contract test file pins the template placement, the CSS rules and the script. `TestStarsControlLook` banned a bare `#` in `stars.css`; amended with a dated note to ban a hex colour instead, because the sheet now holds the `#rate-host` and `#toast` selectors. The two older "closed card gets no box" tests are unchanged and still pass (they run on a page without a host).

### IN-B03: Unmute or Unblock for a person who renamed since the list was read

**Commit:** 5120c51b
**Files:** `internal/service/people.go`, `internal/service/people_mute.go`, `internal/api/handlers/people.go`, `internal/api/handlers/people_mute.go`, `internal/api/handlers/people_changed_e2e_test.go` (new), `web/static/js/people-list.js`, `web/people_list_test.mjs`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`

- Agreed in one place, `handlers.ChangedHeader` = `Pinalert-Changed`: the 204 of `POST /api/people/{username}/unmute` and `/unblock` carries `1` when the caller's own row was deleted and `0` when nothing was there. The status stays 204 for every well formed name, and the `0` is the same for a name nobody holds and for a person never muted or blocked, so nothing about other people leaks. A malformed name is still the uniform 404 with no header. Other people routes carry no header.
- Service: `UnmuteChanged` and `UnblockChanged` return the `:execrows` count as a bool; `Unmute` and `Unblock` keep their signatures and call them.
- `people-list.js`: on `0` it does not remove the row or say "is unmuted"; it reads the list again, shows "That person is no longer listed under that name. The list is up to date now." in the window's message place, focuses the first row button, and does not call `onChange`, so Settings does not take its count down. A missing header (an older server) still removes the row as before.
- `make swag` run: only the two 204 answers gained a `Pinalert-Changed` header; `@license.name` still reads "Unlicensed (all rights reserved)" (router.go:507, docs.go:17).
- Tests: `TestUnmuteAndUnblockSayWhetherARowChanged` (rename scenario, both routes, over the real router and Postgres), `TestChangedHeaderIsAgreedInOnePlace` (the script and swagger name the handler's header), and four node cases.

### Browser handoff: `rate_visibility` on the Let others follow me switch

**Commit:** 5eea3d3a
**Files:** `web/static/js/settings-people.js`, `web/settings_people_test.mjs`

The save already put the switch back and showed the server sentence for any 429 that carries one, so `rate_visibility` (same body shape and Retry-After as `rate_people_action`) already took the same path as the other 429s. No logic change was needed. This commit names the field in the script header and pins both fields with a test that also checks the next try is sent clean. It passes before the commit by design (a pin of behaviour that already held).

### Docs handoff

**Commit:** 97ac1e21
**Files:** `docs/MODERATION.md`, `docs/TRUST-MODEL.md`, `web/round2_docs_contract_test.go` (new)

- MODERATION.md rate table: new Visibility row (account, 6, 1 per 20 seconds, `visibilityBurst`, `visibilityEvery`, only when turning the profile on). The "three people rows" sentence now says four, with a paragraph on the `rate_visibility` 429 and on `/u/{username}` sharing the People reads bucket and answering a 429 page with Retry-After, never JSON. The test reads `visibilityBurst` and `visibilityEvery` from `cmd/server/main.go`, so the row cannot drift.
- MODERATION.md restore window: "since the earliest removed rating was first written". The file had only one such line (310; there was no second at 312).
- Finished Rescue reports on a profile at precision 5: TRUST-MODEL.md:685 was already fixed by the server round. Two more stale places fixed: the Phase 8 and 9 hand off line (it sent profiles through `CoarsenFinishedRescue`) now names `CoarsenFinishedForProfile` at precision 5, and the finished Rescue blur paragraph says a profile does not use that blur. MODERATION.md and PUSH.md have no such line (pinned). docs/PUSH.md had no stale line, and TRUST-MODEL.md:719 already says "first written".

## Not fixed

None.

## Notes

- Commit 7e1a14b5 is a separate commit only because my K2 edit left one test line not gofmt clean and I may not amend.
- Nothing outside the four items was changed. No server was started, no port was bound, no Python was used.

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer), part round2_
_Iteration: 2_
