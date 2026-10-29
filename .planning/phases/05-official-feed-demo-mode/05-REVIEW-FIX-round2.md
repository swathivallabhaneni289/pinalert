---
phase: 05
part: round2
base: 2417b4ac
fixed_at: 2026-10-29
review_path: .planning/phases/05-official-feed-demo-mode/05-REVIEW-FIX-SKEPTIC.md
findings_in_scope: 4
counts:
  fixed: 4
  not_reproduced: 0
  recorded_only: 0
status: all_fixed
---

# Phase 5 review fix, round 2: the four new problems of the skeptic pass

Scope: NP-1 to NP-4 of 05-REVIEW-FIX-SKEPTIC.md. Each fix was written test first (the new cases failed before the change) and committed on its own, path limited, with the trailer.

| Id | Outcome | Commit | Test that proves it |
|----|---------|--------|---------------------|
| NP-1 (warning) | fixed | 7a6b8a2c | web/official_alerts_test.mjs, the three "NP-1:" cases (an idle Demo page over 31 and then 90 more minutes, with the tab shown again and a restored page, makes one request and never restarts; Try again on an ended visit hands over once; an automatic ask that meets an ended visit never hands over); browser check below |
| NP-2 (warning) | fixed | 48313882, 5b7a00e1 | web/official_pins_test.mjs, the three "NP-2:" cases (stale answer, failed refresh, replaced and dropped pin: the opener is already off the map when the card closes); web/official_card_close_test.mjs (new), three "NP-2:" cases on the real closeOfficialPopup text (a phone card whose pin went focuses the map; the same on a laptop; a phone card whose pin is still there focuses the pin); browser check below |
| NP-3 (info) | fixed | 0675ab22 | internal/api/handlers/demo_page_e2e_test.go TestDemoVisitLimitFailsClosed (the three Demo pages served with no limiter: a missing, unknown or malformed visit answers 429 and the store stays empty; a living visit answers 200) |
| NP-4 (warning) | fixed | 00ea2da3 | web/official_alerts_test.mjs, the seven "NP-4:" cases (five minute stale ask, fresh limit, tab shown again, restored page, two open rows, a focused row that left the answer, focus moved elsewhere); browser check below |

## What changed

- **NP-1** (`official-alerts.js`): on a Demo page (`data-demo="1"` on `#official-page`, or `window.PinalertDemoPage`), the minute tick only moves the status line, and `visibilitychange` and `pageshow` ask nothing. The Demo's sample answer is always fresh with age 0, and its world ends after 30 idle minutes, so an automatic ask could only meet an ended visit. The WR-B08 hand over to `PinalertDemoEnded` now runs only for loads the reader started: the first load and Try again. An automatic ask that meets an ended visit (possible only on a page that is not the Demo) shows the load error, and its Try again is the press that starts the Demo again. So no visit token is spent without a press. No visible word changed: the error block and Try again are the existing ones, so the UI-SPEC is unchanged. The server's ended sentence ("This Demo has ended. Reload the page to start again.") is still only in the 404 body; it is not drawn on this page, because no path reaches it without a press now.
- **NP-2** (`official-pins.js`, then `map.js`):
  - First commit: `clearAll` and both removal branches of `drawPins` take the marker off the map before they close the card. `closeOfficialPopup` in map.js then finds the opener gone and puts focus on the map container.
  - The browser check showed that this was enough on a laptop, but not at 390x844, where focus still fell to the body in both engines. On a phone the card sits in the detail pane outside the map container. The container's focusin listener therefore never set `popupFocusInside`, and `closeOfficialPopup` did not know that focus had been in the card. That was true before this round as well, whatever the order.
  - Second commit: `closeOfficialPopup` also counts focus that is inside the card when it runs.
  - Side effect for the owner to know: a phone card closed while its pin is still on the map now also hands focus back to the pin, as a laptop card already did. The crowd report close path (`onPopupClose`) was not touched.
- **NP-3** (`demo_page.go`): `demoVisitFor` treats a missing charge function as a refused one (`!ok || !charge()`). A page served without `demoVisitLimiter` therefore never starts a world. A move with a valid, living visit is still free. `store.Create()` has no other caller.
- **NP-4** (`official-alerts.js`):
  - Each row button carries `data-row-key` (group kind, source, id). Panel ids change on every rebuild; this key does not.
  - An automatic ask notes the focused row and every open row. After the new answer is drawn, it opens those rows again and puts focus back on the same row.
  - When that row has left the answer, focus goes to the title of the block now shown (given `tabindex="-1"`), never the page body.
  - Focus the reader moved somewhere else is never pulled back.
  - Over a stale or unavailable answer, the ask no longer blanks the page, because that drawing claims nothing that can go out of date.
  - Over a fresh answer it still blanks first, so the CR-B01 claim never waits for the new answer. The existing CR-B01 cases, including "the claim is not kept up while the new answer is out" on a tab shown again, pass unchanged.
  - The reader's own Try again still blanks without restoring, as before.

No old test case was changed or removed. The fake map of official_pins_test.mjs gained a dated "Amended 2026-10-29 ... (NP-2)" note for its new record of whether the opener is still on the map.

## Recorded, not changed

- **A hung automatic ask still loses focus.** `giveUpIfHung` (the WR-B01 limit on the minute tick) calls `renderError` without taking and keeping the reader's place. An automatic ask that hangs for 10 seconds or more therefore still empties the list and drops focus. This needs a request that never answers to show. A small follow-up: take the place in `giveUpIfHung` and keep it after `renderError`.
- **An idle Demo Official alerts page ages in place.** It no longer asks again, so after a long idle the status line climbs ("Last checked 2 hours ago") under "In force now", and a sample row can pass its Until time. This is made-up data behind the Demo strip, and a press (Try again, a bar link, a reload) starts a fresh visit.

## Browser check

Setup:
- The server was a throwaway Go test in internal/api/handlers. It served the real `api.NewRouter` through `newE2EServerWith`, with a literal throwaway session secret, the recording mailer and the real Demo store, on 127.0.0.1:8796, over database p5_fix_r2.
- The official service was nil, so no SACHET or GDACS call was possible. `/api/official/alerts` and `/api/official/pins` were answered with fixed JSON through Playwright `page.route`.
- One account was signed in through the real request-link and verify flow and onboarded by SQL.
- Playwright 1.63, Chromium and WebKit, at 390x844 and 1470x830, with service workers blocked so that `page.route` sees every request. Without that, WebKit's later asks went through sw.js to the real server.
- The scratch test file was deleted before committing, the scripts stayed in the session scratchpad, and no screenshots were taken.

Results: 36 of 36 checks passed.
- **NP-1:** `/demo/alerts` with `page.clock` installed before load was left for 31 minutes, then `visibilitychange` and `pageshow` (persisted) were fired, then it was left for 90 more minutes. In each engine and size there was one alerts request, no navigation, the same address, and the sample answer still drawn.
- **NP-2:** a GDACS pin on Nearby was opened by keyboard (focus inside the card), then `PinalertOfficialPins.drawPins({state: 'stale'})` ran.
  - After the second commit, focus is on `.leaflet-container` in each engine and size, never the body.
  - With only the first commit, 390x844 left focus on the body in both engines.
- **NP-4:** on `/alerts`, the second row was focused and opened with Enter. Then each trigger ran: a stale answer for six minutes, a fresh answer to the 30 minute limit, and a fresh answer for 70 seconds followed by `visibilitychange`.
  - Each trigger made a second request.
  - Focus was then on the row button keyed `reader:sachet:42` of the new list, with `aria-expanded="true"` and its panel shown.

## Suites

- `node --test web/*.mjs` at 5b7a00e1: 1161 tests, 1161 pass, 0 fail.
- Full Go suite on p5_fix_r2 (`go test ./... -p 1 -count=1 -v`) at 5b7a00e1: 1866 top level `--- PASS` (5293 with subtests), 0 `--- FAIL`, 0 `^FAIL`, 20 packages ok. Only TestGeocodeLive, TestOfficialLive and TestScreeningFixture were skipped. No flake needed a rerun. It was also run once at 00ea2da3 with the same result.
- p5_fix_r2 was dropped afterwards (`psql -lqt` no longer lists it), and the server on 8796 was stopped.
