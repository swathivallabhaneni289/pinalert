---
phase: 05-official-feed-demo-mode
plan: 15
subsystem: nearby-banner
tags: [official-alerts, banner, nearby, accessibility, live-region, session-storage]
status: complete

requires:
  - phase: 05-08
    provides: GET /api/official/alerts with the server's banner pick (D-16)
  - phase: 05-09
    provides: pinalert.nearbyCentre reader (official-alerts.js), level chip tints and icons
  - phase: 05-13
    provides: pinalert:feed-centre event from app.js, window.PinalertApiBase
provides:
  - the one line Nearby banner (markup, CSS, official-banner.js)
  - window.PinalertOfficialBanner.render(view)
  - the tab's Nearby centre kept under pinalert.nearbyCentre for this account
affects: [05-19, 05-22]

key-files:
  created:
    - web/static/js/official-banner.js
    - web/official_banner_test.mjs
    - web/official_banner_contract_test.go
  modified:
    - web/templates/index.html.tmpl
    - web/static/css/feed.css
    - web/static/css/phone.css

key-decisions:
  - "render takes the whole view, not only the banner field: the script itself gates on state === 'fresh' (defence in depth for ALERT-04) and on a Severe or Extreme level with an event phrase and an area, so a stale view that still carried a banner field is hidden."
  - "A failed request, an HTTP error or an unreadable body hides the banner but keeps what was last announced, so the same alert returning is not announced twice; a view with no pick resets it, so a new alert after a gap is announced."
  - "A changed place is fetched after a 500 ms settle and never closer than 1 s to the previous fetch; the same place again is not fetched; a 5 minute interval refreshes the place last fetched. An older slow answer is dropped by a request counter."
  - "The comma and the full stop of the accessible name are visually hidden spans in the template, the level word is written by the script, so the name reads Extreme. Cyclone warning, Krishna district. 2 more alerts. with no aria-label."

requirements-completed: [ALERT-03, ALERT-04]

duration: about 1h 45m
completed: 2026-10-29
---

# Phase 5 Plan 15: The Nearby banner Summary

One slim, still, 44px row at the top of the Nearby list says "Cyclone warning · Ntr district +1 >" only when the server's pick exists and the feed is fresh, follows the place Nearby is showing, and says nothing when the feed cannot vouch for it.

## Commits

- 36772831 test(05-15): pin the Nearby banner's one line shape, rule and words (RED)
- 28a9d81c feat(05-15): draw the Nearby banner as one quiet line that follows the place (GREEN)
- 9080bfea docs(05-15): say where the phone banner sits in its comment

RED was run before Task 2: `TestBannerMarkup`, `TestBannerOneLineAndTarget`, `TestBannerLevelIconAndTint`, `TestBannerNoUnderlineNoAnimation` and `TestBannerScriptSinks` failed (old two line markup, no `.alert-banner__area` rule, no script) and the node file could not load the missing script. After Task 2 all five pass and the 26 node cases pass.

## What was built

- **Markup (index.html.tmpl).** The opening tag `<a class="alert-banner" id="alert-banner" href="/alerts" hidden>` is unchanged, so every 3.4 pin on it holds. Inside: `#alert-banner-level` (visually hidden), the aria-hidden icon span, `#alert-banner-event`, the aria-hidden `&middot;`, a visually hidden ", ", `#alert-banner-area`, a visually hidden ". ", `#alert-banner-more` (aria-hidden, hidden), `#alert-banner-more-sr`, the chevron. `official-banner.js` loads deferred right after `app.js`. The old title and line spans are gone.
- **CSS (feed.css, phone.css).** `.alert-banner:not([hidden])` is a flex row, `min-height: var(--touch-target-min)`, `white-space: nowrap`, 14px label type, 8px radius on the laptop; only `.alert-banner__area` shrinks (overflow hidden, ellipsis, `min-width: 0`); icon, event, dot, `+N` and chevron are `flex: none`. Extreme is the critical tint and border with `octagon-alert.svg` in `--color-tone-bad`; Severe the medium tint and border with `triangle-alert.svg` in `--color-tone-caution`. Hover and focus keep `text-decoration: none`; focus ring offset -2px; forced colours give a `CanvasText` border and icon. No animation, transition or opacity. `phone.css` keeps the flush row (margin 0, padding 0 16px, `border-width: 1px 0`, radius 0) in the level colour; its place after the handle and the list head is as 3.4 built it.
- **Script (official-banner.js).** `follow()` listens for `pinalert:feed-centre`; `fetchBanner(lat, lon)` fetches `<base>/official/alerts?lat=&lon=` (base as official-alerts.js reads it: `PinalertApiBase`, else the app shell's `data-api-base`, else `/api`); `render(view)` draws only a `fresh` view with a Severe or Extreme pick; `announce(pick)` writes `Official alert: Cyclone warning, Ntr district.` to `#notice-live` only on a first show or a more serious alert; `rememberCentre(lat, lon)` writes `{"account":"<id>","lat":n,"lon":n}` to `sessionStorage` under `pinalert.nearbyCentre`, only when the app shell has a `data-account-id` (the Demo stores nothing), silent on a storage error. Constants `REFRESH_MS = 300000`, `NEARBY_CENTRE_KEY`. Feed strings go in through `textContent` only.

## Browser check (Task 3)

Real `api.NewRouter` over database `p5_15`, a log mailer that wrote the sign in link to a file, a SACHET fetcher that is registered with a 240 hour gap and would fail if reached (it never ran), 127.0.0.1 only, Playwright WebKit and Chromium. Reader at 16.5062, 80.6480 (district 749, "Ntr district"; the plan text says Krishna district, the boundary file names this one). Both engines gave the same numbers, light and dark. No image is saved in the repo (screenshots stayed in the scratch folder).

Fit table (height of the row, width left for the area, characters of the area shown before the ellipsis):

| Width | Case | Height | Area width | Area characters | Wraps |
|-------|------|--------|-----------|-----------------|-------|
| 320 | Extreme, "Cyclone warning", "+1" | 44px | 52px | 6 of 12 ("Ntr di") | no |
| 390 | same | 44px | 69px | 12 of 12 | no |
| 1470 (laptop box) | same | 44px | 69px | 12 of 12 | no |
| 320 | Extreme, "Thunderstorm warning", "+2" | 44px | 8px | 0 of 12 | no |
| 390 | same | 44px | 69px | 12 of 12 | no |
| 1470 | same | 44px | 69px | 12 of 12 | no |
| 320 | Severe, "Heavy rain warning", no count | 44px | 63px | 8 of 12 | no |
| 390 and 1470 | same | 44px | 69px | 12 of 12 | no |

**For the viewing session (UI-SPEC 2a asks for a report, not a wrap).** At 320 with the longest phrase, "Thunderstorm warning" and "+2", the area gets 8px: under the 5 characters the spec names, so the row reads "Thunderstorm warning · N +2" with a clipped letter. The row stays one line at 44px, the full words are in the accessible name, and nothing wraps. A later choice is whether, under about 5 characters, the dot and area should drop out. Not done here: the plan says report.

Other measured results (both engines, 320, 390, 1470, light and dark):

- Row geometry: laptop box has a 1px level border, 8px radius, 12px padding; phone is flush (0px side borders, 0 radius, 16px padding, 1px top and bottom in the level colour), chevron 16px from the right edge on the phone, 13px on the laptop. No horizontal page overflow at any width.
- Level look: Extreme background rgb(251,233,231) light and rgb(58,21,18) dark, border `--color-severity-critical`, octagon; Severe on the medium tint with the triangle. The laptop follows the theme toggle, not the system setting, so laptop dark was checked with `pinalert_theme=dark` (same tint, border and icon as phone dark).
- The whole row opens `/alerts`: a tap at the far right edge of the row, away from the words, navigated to `/alerts` in both engines. The stored centre was `{"account":"1","lat":16.5062,"lon":80.648}` (chromium) and account 2 (webkit).
- Nothing animates: computed `text-decoration-line: none`, `animation-name: none`, zero running animations on the row and its children; with `prefers-reduced-motion: reduce` every measured value (height, width, area width, characters, colours, border, decoration) was identical to no preference.
- D-16: with the district alerts cancelled and only a state wide Severe alert (Andhra Pradesh) in force, the route returned `fresh` with `banner: null` and the banner stayed hidden at every width in both engines.
- Stale (ALERT-04): with the alerts restored and the last good check set to 2 hours ago, the route returned `stale` with `banner: null` and the banner stayed hidden at every width in both engines. Restored to 3 minutes ago it came back.
- Low data (`/low`): no `#alert-banner` and no `official-banner.js` in the page, in both engines at every width.
- Severe only (Extremes cancelled): the banner showed the Severe look, "Heavy rain warning · Ntr district", no "+N".

## Contrast

The banner uses the same six pairs as the plan 05-09 chips (`--color-text` on the critical and medium tints; `--color-tone-bad` and `--color-tone-caution` icons on them, both themes), so `TestOfficialLevelContrast` in `web/official_contrast_test.go` already covers them; no new pair was added.

## 3.4 pins

No pin needed moving, so no "Amended" note was added. The existing pins (`phone_layout_contract_test.go` line 332, `view_strip_contract_test.go` TestViewStripMarkup, `sheet_contract_test.go` section regex, `show_menu_contract_test.go` opacity scan) name only the opening tag, the id, the order of the banner against the list head and the absence of faded text; none names the old inner spans. All pass unchanged, with every old case kept.

## Verification

- `DATABASE_URL= go test ./web/ -count=1`: ok. `node --test web/*.mjs`: 1016 pass, 0 fail (26 of them the banner cases). `go vet ./...` clean.
- Full suite on `p5_15` (`go test ./... -p 1 -count=1 -v`): 5090 `--- PASS`, `--- FAIL` 0, `^FAIL` 0, panics 0; every package `ok`; SKIP lines exactly TestGeocodeLive, TestOfficialLive, TestScreeningFixture. No known flake appeared, so no rerun was needed. The five `TestBanner*` tests PASS.
- Acceptance greps: opening tag count 1; `alert-banner__event` count 1; `official-banner.js` count 1 in the template; old `alert-banner__title|__line` count 0; `innerHTML` on a code line of the script 0; no em or en dash in any file I wrote (feed.css already held a few in its old comments, untouched).
- `p5_15` dropped and confirmed gone with `psql -lqt`. The scratch server is stopped and nothing listens on 8796.

## Deviations from Plan

**1. [Rule 3 - Blocking] Port 8797 was taken, used 8796.** A long running `python -m http.server 8797 --bind 127.0.0.1` (pid 44791, started about 20 days ago, serving the main repo directory) holds 8797; it is not mine and was left alone. The scratch server ran on 8796 instead, a port in the Phase 5 range that no plan of this wave names and that was free. It is stopped.

**2. [Interpretation] `render(view)` instead of `render(banner)`.** The artifact list names `render(banner)`; the behaviours ask the script to hide a view whose state is stale or unavailable even if a banner field were present, which needs the state. `window.PinalertOfficialBanner.render` therefore takes the whole view. Plan 05-19 reads `banner.demo` from the same view.

**3. [Rule 1 - Test] One contract assertion tightened.** `TestBannerMarkup` first counted every mention of `official-banner.js` in the template (a comment of mine named it); it now counts `/static/js/official-banner.js` and my template comment avoids the filename, so the plan's acceptance grep prints 1.

**4. [Scratch] Throwaway server.** The sandbox does not allow starting the server with `ENV=development`, so a throwaway Go test (`zz_p5_15_scratch_test.go`, literal throwaway session secret, log mailer to a file, SACHET fetcher never reached) served the real router on 127.0.0.1:8796 over `p5_15`. It was deleted before any commit; `git status` shows only my files.

No file outside the plan's list was touched; the plan's `view_strip_contract_test.go`, `phone_layout_contract_test.go` and `sheet_contract_test.go` needed no change.

## Known Stubs

None. The Demo's sample banner and its Demo tag are plan 05-19's; the script already follows `PinalertApiBase` and stores nothing without a `data-account-id`, so they plug in.

## Threat Flags

None. No new endpoint, auth path or storage beyond the plan's register: T-05-80 (fresh only, node and browser checks), T-05-81 (`textContent` only, `TestBannerScriptSinks`), T-05-82 (tab only, account keyed, none for the Demo), T-05-83 (500 ms settle, 1 s minimum gap, 5 minute refresh) are each covered by a node case.

## Self-Check: PASSED

- web/static/js/official-banner.js, web/official_banner_test.mjs, web/official_banner_contract_test.go: present.
- Commits 36772831, 28a9d81c, 9080bfea present in `git log`.
