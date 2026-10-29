---
phase: 05
part: browser
base: e2a20bdd
fixed_at: 2026-10-29
review_path: .planning/phases/05-official-feed-demo-mode/05-REVIEW-browser.md
findings_in_scope: 21
fixed: 20
not_reproduced: 1
recorded: 0
status: all_fixed
---

# Phase 5 code review fix: the browser scripts

Scope: every finding of 05-REVIEW-browser.md under the triage in 05-REVIEW.md (WR-B07 is CR-B02), plus WR-T02 and WR-T05 from 05-REVIEW-templates.md (WR-T05 is WR-B06, IN-T01 is WR-B03), plus two items the orchestrator added from the server fixer: WR-S04-browser and IN-S05-browser. Each fix was written test first and committed on its own, path limited, except where the table says two ids share a commit and why.

| Finding | Outcome | Commit | Test that proves it |
|---------|---------|--------|---------------------|
| CR-B01 | fixed | 604b9807 | official_alerts_test.mjs, the seven "CR-B01:" cases (fresh limit with a fake clock, In force now, stale every five minutes, tab shown again with fake visibility, pageshow persisted, a row that ends, a row already ended) |
| CR-B02 (was WR-B07) | fixed | 29d9f26f | demo_page_test.mjs "CR-B02 app.js / votes.js / demo.js" cases; ended_nearby_test.mjs "CR-B02:" cases; official_alerts_test.mjs and official_banner_test.mjs "CR-B02:" cases; browser check below |
| WR-B01 | fixed | bfad61c9 | official_pins_test.mjs, official_banner_test.mjs, official_alerts_test.mjs "WR-B01:" cases |
| WR-B02 | fixed | c4e11421 | official_pins_test.mjs "WR-B02:" cases (4) |
| WR-B03 (and IN-T01) | fixed | 9ff64bfd | web/official_card_heading_test.mjs (new); browser check below |
| WR-B04 | fixed | 3bf8fa29 | official_pins_test.mjs "WR-B04: two cards never share a title id"; browser check below |
| WR-B05 | fixed | 76f571ed, 5220b3e9, 92c9cdf0 | welcome_flow_test.mjs "WR-B05:" cases; browser check below |
| WR-B06 (and WR-T05) | fixed | 63f73ff0 | official_alerts_test.mjs "WR-B06:" and "WR-T05:" cases; TestOfficialAlertsShell (amended) |
| WR-B08 | fixed | eeb5f154 | official_alerts_test.mjs and ended_nearby_test.mjs "WR-B08:" cases |
| WR-B09 | fixed | fd6ce675 | official_alerts_test.mjs, official_banner_test.mjs and demo_page_test.mjs "WR-B09" cases |
| IN-B01 | fixed | 0df8daba | demo_page_test.mjs "IN-B01 votes.js:" cases |
| IN-B02 | fixed | ef5f9c54 | demo_practice_test.mjs "IN-B02:" case; TestDemoPracticeFlowSource (amended) |
| IN-B03 | fixed | 26433540 | demo_page_test.mjs "IN-B03 votes.js:" case |
| IN-B04 | fixed (in the CR-B02 commit) | 29d9f26f | official_alerts_test.mjs and official_banner_test.mjs "IN-B04:" cases |
| IN-B05 | fixed | c8f7596e | login_globe_test.mjs "IN-B05:" cases |
| IN-B06 (stale comment) | fixed | 14f33eee | comment only, no behaviour, so no test |
| IN-B06 (row button name) | not reproduced | none | browser check below: both engines read "Extreme Cyclone warning 6 mandals Issued by Official IMD Chennai Issued just now Until 7:49 am" with spaces |
| WR-T02 | fixed | afb8e20e | official_alerts_test.mjs "WR-T02:" case; the expected clock strings and the pins card regex (amended) |
| WR-T05 | fixed (with WR-B06) | 63f73ff0 | see WR-B06 |
| WR-S04-browser | fixed | 5025b99a | official_alerts_test.mjs "WR-S04-browser:" cases (3) |
| IN-S05-browser | fixed (same commit as WR-S04-browser) | 5025b99a | official_alerts_test.mjs "IN-S05-browser:" cases (2) |

The counts treat IN-B06 as one fixed finding (the comment) whose second half was not reproduced, and count WR-T05 with WR-B06, IN-T01 with WR-B03 and IN-B04 on its own.

## What changed, by finding

- **CR-B01:** the minute tick of `official-alerts.js` asks again when a fresh answer reaches 1800 s (the server's `FreshWindow`) or a shown row passes its Until time after it was drawn. A stale or unavailable answer is asked again every five minutes. `visibilitychange` to visible and `pageshow` with `persisted` ask again when the last request is a minute old or more. `load()` hides every block first, so neither claim waits for the new answer.
- **CR-B02:** `app.js`, `votes.js`, `ended-nearby.js`, `official-alerts.js` and `official-banner.js` each carry a small inline check, because `api-base.js` is the file that can fail. Without `window.PinalertApiBase` they read the page's own `data-api-base` (from `#app-shell` or the main page root) with the same `SAFE_BASE` test. A page marked `data-demo="1"` with no usable base sends nothing:
  - the list and the Ended part show their load error;
  - a vote and a post fail and are not queued;
  - the banner stays hidden.

  `app.js` and `demo.js` also read `data-demo` from the page, so the Demo marks and the ended visit restart survive. `official-pins.js` is loaded on no Demo page, and its contract (TestOfficialPinsNeverCounted) bars it from reading the page, so it is unchanged.
- **WR-B01:** the pins and banner requests get a 10 second limit from the page's own timer with an AbortController, as in `app.js`. `official-alerts.js` keeps its single timer, because TestOfficialAlertsScriptSinks bans `setTimeout` as a refetch loop risk. Its minute tick gives up a request that has been out for 10 s or more: the request is aborted, its late answer is dropped and the load error shows. The limit therefore lands on the next tick, at most about 70 s after the request. The banner also hides as soon as Nearby moves to another place.
- **WR-B02:** a new `PinalertMap.closeCard()` closes the open GDACS card through its own close path. The pins layer calls it when it clears, or when a redraw removes or replaces the open pin.
- **WR-B03:** the popup heading is put in after the GDACS check: "Hazard" for a GDACS card, "Report" for a crowd report.
- **WR-B04:** card titles are numbered `official-card-title-N`.
- **WR-B05:**
  - `a[href]` (and select, textarea) joined `FOCUSABLE_SELECTOR`.
  - The browser check then showed a regression. Safari's default Tab skips links, so in WebKit, Tab from Log out passed over Try the demo and left the flow for the page body.
  - The second commit fixes it: from a control in the loop, the page now moves Tab and Shift+Tab focus itself, wrapping at the ends.
  - Four older cases that expected the browser to move focus between two controls are amended with dated notes.
- **WR-B06 and WR-T05:** `alerts.html.tmpl` drops `role="status"` from `#official-notice`. `demo_alerts.html.tmpl` belongs to the templates part, so `official-alerts.js` also removes `role` and `aria-live` from the notice at start. The notice copy, age included, is unchanged.
- **WR-B08 and IN-B01:** a 404 with the field `visit` hands over to `window.PinalertDemoEnded`, which `demo.js` sets on Demo pages only. This covers Demo Official alerts, the Ended nearby read, and votes (which includes Reopen and take back).
- **WR-B09:**
  - A stored device fix or tab centre is sent only when it carries a saved time under 30 minutes old; otherwise the server uses the home area alone.
  - The feed-centre event now says `own` and `place`, and the banner keeps a centre (with `savedAt`) only for the reader's own position or a searched place, never the boot fallback centre.
- **IN-B02:** `PRACTICE_POLL_MS` is 2000. `PRACTICE_POLL_LIMIT` went from 40 to 60, so the poll still ends after about two minutes.
- **IN-B03:** `votes.js` reads the fixed pair from the main page root on Demo My activity.
- **IN-B05:** the globe downloads nothing unless the box is `visibility: hidden`, a mark only `login-globe.css` sets.
- **WR-T02:** both `clockText` functions join the minute and am or pm with `' '`.
- **WR-S04-browser:**
  - A row with `ended: true` gets an "Ended" tag and an "Ended" time in place of Until, and is not counted.
  - An answer with only ended rows is titled "Recently ended". It never says In force now, and never says nothing is in force. That claim still needs an answer with no rows at all.
  - Empty date strings already drew no date; a case now pins that.
- **IN-S05-browser:** with SACHET unavailable and `hazards_state` fresh, only the GDACS rows are drawn under the unavailable notice, titled "Hazard events from GDACS". No SACHET row is ever drawn from an unavailable feed.

## Browser checks (port 8796, database p5_fix_b)

The server was a throwaway Go test serving the real `api.NewRouter` on 127.0.0.1:8796. It used:
- a literal throwaway session secret;
- the api package's in-memory fake mailer;
- the real Demo store;
- the official service;
- `http.DefaultTransport` replaced by one that refuses everything except 127.0.0.1. It counted 0 refused requests, so no SACHET or GDACS call was made.

Two accounts signed in through the real request-link and verify flow: one onboarded by SQL, one new. The checks used Playwright 1.63 with WebKit and Chromium. The scratch test file and scripts were deleted or kept outside the repo before committing, and no screenshots were saved.

- **CR-B02:** `/demo`, `/demo/alerts` and `/demo/activity` with `api-base.js` aborted by a route, at 320x568, 390x844 and 1470x830 in both engines. No request went to `/api/reports` or `/api/official`, every page used its `/demo/api/v/<visit>` base, and Demo Nearby kept the Demo flag. 42 of 42 passed.
- **Official alerts states:** `/alerts` with the SACHET status set fresh, then stale (2 h), then unavailable (7 h). Each ran at the three sizes, light and dark, in both engines. The notice never had `role` or `aria-live`, there was no sideways scroll, and the stale notice has U+00A0 before am or pm. 120 of 120 passed.
  - With no Nearby position the page showed "need your area", as WR-B09 now intends.
  - After a visit to Nearby with geolocation, the fresh and empty claim showed, at 390x844 in both engines.
  - A `visibilitychange` 61 s later sent a second request, in both engines.
- **GDACS card on a phone (390x844, both engines):** one seeded Red hazard near Vijayawada. Opened by keyboard on its marker, focus landed on the H2 "Hazard", and the title id was `official-card-title-1`. A Playwright tap on the pin did not open the card in either engine (focus went to the map), so the keyboard was used. This was not investigated further; it looks like the pin sat under the phone sheet at that map position.
- **Welcome focus trap (three sizes, both engines):**
  - On a phone, step 1 Tab from Continue lands on Try the demo.
  - In the Disagree box, Tab from Log out lands on Try the demo, and Tab from there wraps inside the flow.
  - 22 of 22 passed after the second WR-B05 commit. Before it, WebKit failed 3: Safari's Tab skipped the link.
- **Welcome Tab walk (after 92c9cdf0, three sizes, both engines):** on each of steps 1 to 4, focus starts on the step title and Tab is pressed 16 times.
  - Chromium: all 12 walks stay on visible controls inside `#welcome` and wrap.
  - WebKit: steps 2 and 4 pass at every size. On steps 1 and 3, the first Tab from the title (tabindex -1, which the trap leaves to the browser by design) goes to the page outside the flow.
  - Cause: Playwright WebKit's default Tab, like Safari's, stops only on text fields and tabindex 0, and steps 1 and 3 hold only links and buttons. The same happened before this round's changes, because the trap never handled Tab from the title. It is recorded here as a pre-existing follow-up, not fixed: handling Tab from the title would change the title rules that several older cases pin.
- **Banner:** not checked in a browser. Seeding a Severe SACHET alert that matches a district needs the area tables, so the node cases are the proof for the banner changes.
- **IN-B06 row name:** read from the accessibility tree on `/demo/alerts` and on a rendered row with the Official badge. Both engines put spaces between the parts.

## Suites

- `node --test web/*.mjs`: 1145 tests, 1145 pass, 0 fail (rerun after the last commit).
- Full Go suite on p5_fix_b (`go test ./... -p 1 -count=1 -v`, at 5220b3e9): 1844 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL`, 20 packages ok; only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped; no flake needed a rerun. The last commit (92c9cdf0) touches only welcome.js and its node test; `go test ./web/` passed again after it.

## For the orchestrator and the other parts

- **Templates part:** `demo_alerts.html.tmpl:56` still has `role="status"` on `#official-notice`. The script removes it at load, so the Demo is covered, but the attribute can be dropped there too.
- **New words not in the UI-SPEC:** the GDACS card heading "Hazard", the titles "Recently ended" and "Hazard events from GDACS", and the row tag "Ended". The templates part may want UI-SPEC amendments.
- **Go pins changed in web/:**
  - TestOfficialAlertsShell: a template attribute changed (WR-T05).
  - TestDemoPracticeFlowSource: `PRACTICE_POLL_MS = 2000`, which the brief asked for. This is a source pin, not a template attribute, so I flag it.
- **WR-B06 commit message:** the body names the amended test TestOfficialAlertsTemplate; its real name is TestOfficialAlertsShell. History was not rewritten.
- **Owner look:**
  - The banner now hides on every move to a new place and comes back when that place's answer arrives (WR-B01). It may blink while panning.
  - The Official alerts time limit lands on the minute tick, as explained above.
