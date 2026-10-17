---
sketch: 009
name: login-globe
question: "Can the sign in page show a real Earth turning on its axis, not a flat picture, calm enough for a page everyone must pass, and does it sit behind the card or beside it?"
winner: "A, behind the card (owner 2026-10-17: \"i like A\"), with the realistic Earth (NASA pictures, approved the same day)"
tags: [login, sign-in, globe, maplibre, webgl, phone, laptop, light-dark, low-data, phase-5]
---

# Sketch 009: The sign in page with a real turning globe

## Design question

The owner, 2026-10-17: "I want like the actual globe rotating. Um, not like a some image doing". On 2026-09-16 they had
already said the globe on today's sign in page does not read as a globe (todo
`.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`). Today's globe is `/static/img/globe.svg`, a
hand drawn wireframe used as a CSS mask and spun like a flat wheel (`web/static/css/auth.css:35-74`); it never turns on
its axis. This sketch keeps the sign in card exactly as it is and puts a real 3D Earth behind it or beside it.

## How to open it

It is served at http://127.0.0.1:8804/ (this folder). Look at it in Mac Safari for the laptop and in the iPhone simulator
for the phone. On a real phone the page fills the screen; the phone frame is for looking from a laptop. On a phone the
switch strip takes about 212px at the top, so A's globe there is about 580px. The app has no strip, and the phone frame
shows its true size (776px on 390 by 844).

The grey strip at the top is the mockup's own switch bar, not part of the app. The switches reload the page; Send inside
the card does not.

- **View:** Laptop, or Phone (the same page inside a 390 by 844 frame).
- **Globe:** A, behind the card, or B, beside the card.
- **Screen:** Sign in, Check your inbox, Email verified.
- **Theme:** Light or Dark. Without a choice it follows the device.
- **Low data view:** Off or On.
- **WebGL:** Available, or Missing (forces the path a device without WebGL2 takes).
- **Earth:** Realistic (NASA satellite pictures, the default) or Map colours (the app map's look).
- **Country borders** (Map colours only): Shown or Hidden.
- On the right, a status line says what the globe is doing: turning, still (reduce motion), paused (tab hidden), or off
  and why. In Phone view it repeats the framed page's own status, read about once a second.

Address parameters behind the switches: `view=laptop|phone`, `place=a|b`, `screen=signin|sent|verified`,
`theme=light|dark`, `low=1`, `nowebgl=1`, `earth=map`, `borders=0`. Two more for checks only: `email=` (the address
shown on Check your inbox) and `tz=1` (Map colours: zoom 1 tiles instead of zoom 0, to compare detail and weight).
Checks can read `window.__globe`:
`map`, `spinning()`, `centerLng()`, `status()`, `size()` and `errors`.

## Sign in today, and where "create account" is

There is no separate create account or sign up page. New and returning people use the same one field form at `/login`,
"Verify your email to continue", with no password, username or name. Asking for a link only stores a 5 minute single use
token; the account is created when the emailed link is opened (`VerifyToken` in `internal/service/auth.go`), which is why
the email says "No account will be created." if it is ignored. Right after that first sign in comes the four step first
time flow at `/welcome`: Welcome, Disclaimer and policy (Agree required), Your location, About you (name and home area,
optional, with Skip). Phase 8 turns step 4 into a required sign up: Username with a live check, Name and Home area, no
Skip (sketch 008, "Owner decisions"). The globe sits behind the sign in screens only (Sign in, Check your inbox, and in
this sketch Email verified, see item 18 and question 6); the first time flow at `/welcome` does not share it.

Planned: Phase 8 changes how people sign up and sign in, and the owner is still settling the details on 2026-10-17. The
current decision is `.planning/ROADMAP.md` Phase 8 scope note 6; it is not repeated here so it cannot go stale. This
sketch keeps today's single card, because that is what exists; the globe sits behind or beside whichever card the page
ends up with.

## Realistic Earth (owner, 2026-10-17)

The owner asked for "a more realistic earth" turning. They asked whether it was free and were told yes (NASA, no
account, no key, a credit only) and that NASA gives no uptime guarantee. Then they approved it: "yes i like the earth".
So the globe shows NASA Blue Marble satellite pictures by default (commit 45dfaa6), and the strip's Earth switch keeps
Map colours, the earlier OpenFreeMap look, for comparison. Source: NASA GIBS `BlueMarble_NextGeneration` in the web map
tiling (`https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/BlueMarble_NextGeneration/default/GoogleMapsCompatible_Level8/{z}/{y}/{x}.jpeg`),
which answers with `Access-Control-Allow-Origin: *`. The style is one picture layer capped at zoom 3, with no labels, no
borders and no glow (no gradient on the page); the same spin, size, fade in, give up and reduce motion rules apply. The
sections below describe the realistic Earth, with Map colours noted where it differs.

Every GIBS address tried (`best` and `std`, with and without a date) answers
`Cache-Control: max-age=0, no-store, no-cache, must-revalidate`, so browsers fetch the pictures again on every visit.
The measured cost is under Download weight, and item 21 of For the build has an option.

## Variants

- **A, behind the card (default; picked by the owner 2026-10-17, "i like A").** The Phase 1.1 place (D-06). The globe sits behind the centred card, dimmed on its
  wrapper (50 percent in light, 40 percent in dark; today's picture uses 35 percent). The card stays solid, so its
  contrast does not change. Laptop: a sphere about 92 percent of the screen height (766px on a 1440 by 900 window) shows
  all round the card. Phone: the same idea scaled to the screen height (776px on a 390 by 844 phone), so it is wider
  than the phone; with the card over the middle, only the top and the bottom of the Earth show. **This is a deliberate
  departure** from showing the whole sphere: the card fills the phone's width, so a sphere that fits the width would
  hide behind it almost completely (about 9px would show on each side). On the laptop, and in B on both sizes, the
  whole sphere shows.
- **B, beside the card.** The globe at full strength (90 percent in dark). Laptop: globe on the left (516px on a 1440 by
  900 window), card on the right. Phone: globe at the top (242px on a 390px phone) with its credit under it, then the
  card; the Low data panel needs a short scroll (the page is 934px tall on a 390 by 844 phone, 978px with the two line
  Map colours credit).
- Both have the same globe, speed, credits and behaviour on the phone and the laptop; only the place and the size differ.

## What is on the page

1. **The sign in card**, word for word from `web/templates/login_gate.html.tmpl`: "Verify your email to continue", the
   paragraph, "Email address" with the mail icon and "you@example.com", the error line, "Send verification link". Send
   behaves like `web/static/js/auth.js` without sending anything: "Sending…", then "Enter a valid email address." for an
   empty or malformed address, or the Check your inbox panel in place, with no page load. The address bar gets
   `screen=sent&email=...`, as the app writes `?sent=`.
2. **Check your inbox**, word for word from `check_inbox.html.tmpl`, with the typed address in the sentence, "Resend in
   45s" counting down to "Resend link", and "Check again" reloading into the same panel with Resend ready at once, as
   the app does. The 5 minute expiry notice appears after 5 minutes.
3. **Email verified**, from `verify_outcome.html.tmpl`: the green check, "Email verified", "Taking you to Pinalert…",
   "Continue". The 2 second move to the app is not run here, and Continue goes nowhere. Like the real page, it has no
   Low data panel.
4. **The panel under the card:** "Low data view" and "Text only, no map", or, when it is on, "Low data view is on for this
   device. It opens after you sign in." with "Turn off"; then Disclaimer and Policy, which go nowhere in the mockup.
5. **The globe**, with a credit line: "Earth pictures: NASA Blue Marble via NASA GIBS", linking to NASA's Blue Marble and
   GIBS pages (Map colours: "OpenFreeMap © OpenMapTiles Data from OpenStreetMap", the same three links as the app map).
   In A it sits in the bottom right corner on a laptop and under the Low data panel on a phone (and in a laptop window
   narrower than 1200px and shorter than 800px, where the corner would sit under the panel), on a solid page coloured
   background so the picture under it never matters; in B, under the globe. The realistic credit is one line on a laptop
   and on most phones and wraps to two on a 320px screen; the Map colours credit wraps to two on any phone. It is in the
   page from the first paint but invisible until the globe shows, so the card never moves when the globe arrives.

Nothing was added: no logo, wordmark, menu or theme button. The heading is still the first thing on the page, and
keyboards and screen readers meet the globe credit only after the card.

## How the globe is made

- **The library the app already loads.** maplibre-gl 5.24.0 from unpkg, with the same URLs and SRI hashes as
  `web/templates/index.html.tmpl:22-24,205-207`, so the browser cache is shared with the map page. A plain
  `new maplibregl.Map` with the globe projection, not Leaflet and not the bridge (the bridge draws MapLibre flat inside
  Leaflet, `web/static/js/map.js:284-313`). No three.js and no build step; the Earth pictures are NASA's tiles, drawn by
  MapLibre onto the sphere.
- **Realistic Earth (default).** A style written in the page script: one raster source, the GIBS address above, 256px
  tiles, `maxzoom: 3`, and one raster layer with no fade. Nothing else: no labels, borders, glow, style file, TileJSON,
  font or sprite. Zoom 3 is sharp enough for a 766px globe and caps the download (85 tiles at zoom 0 to 3).
- **Map colours (the switch): the app's map style, trimmed for calm.**
  `https://tiles.openfreemap.org/styles/liberty`, the same style URL as
  `map.js:306`, fetched when the page runs. Left out of the copy: every label and icon layer (left out to keep the globe
  calm; it also means no font or sprite download), the shaded relief picture (`natural_earth`, PNG tiles of about 180 to
  250 KB each), and the two pattern layers that need the sprite. The vector tiles are capped at zoom 0, so the whole
  Earth is one 49 KB tile at any globe size. Land, sea, ice, woods and country borders keep the app map's colours. There
  is no glow round the sphere: MapLibre can draw an atmosphere, but the trim removes any sky block, so it stays off
  (atmosphere-blend falls back to 0), which also keeps gradients off the page.
- **Turning.** India in the middle at the start (longitude 80, latitude 18). The latitude never changes, so the size never
  changes. One turn every 2 minutes, west to east like the real Earth, driven by requestAnimationFrame at about 30 frames
  a second, which is plenty for 3 degrees a second and kinder to phone batteries. A slow frame moves the globe at most
  0.1 seconds' worth, so it never jumps.
- **Never in the way.** The map has `interactive: false` (no touch, mouse or keyboard handlers, and Tab skips the canvas),
  and its wrapper is `aria-hidden` with `pointer-events: none`, so it never takes a tap or a scroll. MapLibre's own credit
  button is off because it would sit inside that wrapper, faded and untappable; the credit line sits outside it at full
  contrast.
- **The form first.** The card is plain HTML and works from the first paint. Nothing for the globe is downloaded until
  the page has loaded and the browser is idle (`requestIdleCallback`, or a 0.2 second timer where it is missing; it is
  missing in Safari's engine, confirmed). The globe fades in over 0.6 seconds once the map has drawn its first full
  frame, with no fade under reduce motion. Send swaps the card in place and the same map keeps turning.
- **Reduce motion.** Read in the script, because a CSS media query cannot stop a script loop, and followed live: a page
  opened with reduce motion on shows the globe still, with India in the middle. Turning it on while the globe turns
  stops it where it is, and turning it off starts it again.
- **Hidden tab.** The loop stops on `visibilitychange` and starts again when the tab is back. It also stops if the WebGL
  context is lost and resumes when MapLibre restores it.
- **Low data view on.** No globe at all, and nothing is downloaded: no library, style or tile.
- **No WebGL2, or anything fails** (the library blocked, a wrong SRI hash, the style or tile refused, the map throwing,
  or nothing has drawn 10 seconds after the download started): no globe, and no picture in its place, since the owner
  does not want a picture. WebGL2 is checked before anything downloads, so a device without it pays nothing, and then
  the page has no globe space at all. The 10 second count waits while the tab is hidden (a hidden tab draws nothing)
  and starts again when it is shown. A library or style that arrives after the globe gave up draws nothing.
- **A failure after the page has shown keeps the space.** The map is removed, but the globe's empty space and its
  hidden credit line stay, so the card never moves under someone's finger. Closing the space instead would move the card
  298px left on a 1440 by 900 laptop in B, and 249px up on a 390 by 844 phone in B. Which is better is open question 8.
- **No layout shift.** Everything the globe uses has its space from the first paint: the globe box and the credit line,
  which is written with the page and kept invisible (`visibility: hidden`) until the globe shows. The card, the Send
  button and the Low data panel stay put while the globe loads, turns, or gives up.
- **Sizing.** The globe's size comes from the sign in area (its width, and the height of the first screen), never from the
  card, so swapping the card never moves or resizes it. MapLibre's camera sits 1.5 canvas heights from the surface, so
  the sphere looks smaller than its radius; the zoom is solved for the size it looks on screen. The plain radius formula
  gave 604px where 766px was wanted.
- **Gentle on the device.** `powerPreference: 'low-power'` (MapLibre's default, `high-performance`, can wake a laptop's
  second graphics chip for a background), drawing resolution capped at 2x, about 30 frames a second.

## Checks run (2026-10-17)

Screenshots stayed in the session scratchpad, never in the repo.

- **Snapshot tool** (off screen WKWebView, `?view=laptop`): the page loads with no script errors, `window.__globe` is
  there and the map object is created, but the status stays "loading". That web view reports `document.hidden` as true,
  so no frames run and the globe never draws there; rotation was not measured with this tool. `requestIdleCallback` is
  undefined in that engine, and the fallback timer covers it.
- **Headless Chromium** (Playwright 1.63, software WebGL): the globe draws and turns in A and B, laptop and phone, light
  and dark, with no page errors. Sphere sizes measured from the canvas: 766px (A laptop), 516px (B laptop), 776px tall
  (A phone, wider than the screen on purpose), 242px (B phone), never clipped where it should not be. Software drawing
  is slow, so it turned 5.5 to 8.5 degrees in windows of 3 to 4 seconds instead of 9 to 12.
- **Headless WebKit** (Playwright's build of the Safari engine): B on the laptop draws and turns at exactly the intended
  speed, longitude 79.2 to 70.2 in 3 seconds; no errors.
- **Reduce motion** (emulated): status "still", longitude 80 to 80 over 3 seconds, globe drawn at full size.
- **Send:** "Sending…", then Check your inbox with "ravi@example.com" in the sentence and "Resend in 45s"; afterwards the
  same map object, one map canvas on the page, and the longitude still moving (79.2 to 76.1 across the swap).
- **Low data view on, and WebGL Missing:** zero requests to unpkg or OpenFreeMap. With WebGL Missing in B the card is
  centred with no empty gap.
- **Referrer:** with an email in the address (`?screen=sent&email=test%40example.com`), unpkg and the style request sent no
  Referer, and MapLibre's own TileJSON and tile requests sent only `http://127.0.0.1:8804/`, with no path or query.
  Measured for Map colours (OpenFreeMap); the NASA GIBS requests were not checked.
- **Hidden tab:** simulated only, because headless pages stay visible: overriding `document.hidden` and firing
  `visibilitychange` stopped the loop ("paused while the tab is hidden"), and it restarted afterwards.
- **Country borders Hidden:** no boundary layer left in the style, globe turning, no errors. The border facts in open
  question 3 were read from the loaded tile with `querySourceFeatures` on its boundary layer, not from a screenshot.
- **After the review fixes** (the snapshot tool, plus a scratchpad copy of it in which the page counts as visible, so
  the globe really loads and turns):
  - The globe turned at 3.00 degrees a second in B on the phone and 3.02 on the laptop (longitude 70.4 to 59.4 in
    3.7 seconds), and the drawn pixels changed between the first and last reads.
  - The card, the Send button and the Low data panel kept the same place from the first paint to "turning", in A and B,
    on 390 by 844 and 1440 by 900 (phone B: card at 388px throughout, page 978px tall).
  - Tried and put back: inline credit links with 14px padding above and below, to close the gap between the two
    wrapped lines on a phone. Each tap area measured 45px tall, but the second line's padding covered the first line's
    words: a tap on the middle of "OpenFreeMap" (390 by 844, B) hit the OpenStreetMap link.
  - In A on a phone the credit now sits over the dimmed globe, not on plain page. Its weakest contrast, dark mode over
    land, is about 4.9:1, computed from the colours and the 40 percent dim, not sampled. (Map colours. Superseded: the
    A credit now has a solid background, see Realistic Earth below.)
  - A map error before load, with an address half typed, moved nothing and kept the focus and the text.
  - With the style held back 12 seconds, the globe gave up at 10.3 seconds, the card did not move, and the style that
    came later created no map. On the hidden page the status still read "loading" after 12.6 seconds.
  - With the globe loaded, the email field is the first Tab stop and the three credit links the last, on all three
    screens. Before it loads, the credit links take no focus.
  - No credit overlap at 320 by 568, 375 by 548, 390 by 844 (with and without the strip), 950 by 760, 1024 by 640,
    1100 by 820, 1280 by 650 and 1440 by 900, and no sideways scroll.
  - With a sky block added to the fetched style, the loaded map had none.
  - The owner's rules checker, with the globe loaded, in light and dark, phone and laptop, A and B, found only what is
    copied from the app as is (the 4.34:1 muted paragraph, the input's 1px padding, the 43px wide Policy link) and its
    false emoji flag on ©.
  - In Phone view the strip's status matched the framed page's own status with the globe loading, turning, off for Low
    data view, and off for WebGL Missing.
- **Realistic Earth** (2026-10-17):
  - In Chromium and WebKit (Playwright, run by the 3.4 session): laptop A and B, phone B, phone A dark; status turning,
    no errors.
  - Picture tiles in the first 9 seconds (the visible copy of the snapshot tool; sizes from downloading each tile address
    once): 31 tiles, 248.7 KB in A and in B at 1440 by 900; 27 tiles, 229.1 KB in A at 390 by 844; 10 tiles, 101.2 KB
    in B at 390 by 844.
  - A full 2 minute turn in A at 1440 by 900: 48 different zoom 3 tiles, 377.9 KB. Repeat fetches of the same tile were
    not counted. All 85 tiles at zoom 0 to 3 come to 746.4 KB, the most a visit downloads unless a tile is fetched twice.
  - Cache headers: every GIBS address tried (`best` and `std`, with and without a date) answers
    `Cache-Control: max-age=0, no-store, no-cache, must-revalidate`.
  - The A credit over the photo, before the fix: in light mode, dark text over the dimmed sea measured 4.24 to 4.41:1,
    under 4.5:1, at one turn position (about longitude 57; the turning loop overrides a jump to another longitude). Dark
    mode measured 11 to 18:1. Fixed with a solid page coloured background behind the A credit: 16.9:1 in light and
    17.7:1 in dark, apart from a few pixels at the rounded corners.
  - The owner's rules checker on phone A, light and dark, with the photo globe loaded: only the items already known,
    which are copied from the app as is.
  - No layout shift: the card, email field and Send stayed in place from before the globe loaded to "turning", in B at
    390 by 844 (card at 344px), 375 by 548 and 320 by 568, and in A at 390 by 844 and 320 by 568. The A credit sits
    below the Low data panel (8px gap at 390 by 844), and there is no sideways scroll.
- **Not checked:** real Mac Safari and the iPhone simulator (the owner's look is the check that counts), a real hidden tab,
  a lost WebGL context, and a slow connection.

## Download weight (measured, cold cache)

KB here is 1,000 bytes.

**Realistic Earth (default).**

| What | Bytes | Kept by the browser |
|------|-------|---------------------|
| maplibre-gl.js and .css 5.24.0, shared with the map page | 285.6 KB compressed | a year |
| The globe's own code (a login only script and stylesheet) | about 6 KB compressed | an hour, or until a server restart |
| NASA pictures, first 9 seconds | 248.7 KB laptop A or B (31 tiles), 229.1 KB phone A (27), 101.2 KB phone B (10) | never (`no-store`) |
| NASA pictures, a full 2 minute turn in A on a laptop | 377.9 KB (48 tiles) | never |

- **First visit**, first 9 seconds: about 540 KB on a laptop, 520 KB on a phone in A and 393 KB on a phone in B, against
  today's 31.9 KB (about 17, 16 and 12 times). Staying for a full turn on a laptop: about 670 KB.
- **Every later visit:** the library is cached, but the pictures come again: about 100 to 250 KB in the first seconds,
  up to about 380 KB for a full turn on a laptop. All 85 tiles at zoom 0 to 3 are 746.4 KB, the most for one visit
  unless a tile is fetched twice.
- **Low data view and no WebGL2:** 0 bytes.

**Map colours (the switch).** Everything from here to the end of this section was measured before the realistic Earth,
for the OpenFreeMap look. The 346 KB first visit the owner was told on 2026-10-17 is this look's figure.

| File | Compressed body | On the wire, headers included |
|------|-----------------|-------------------------------|
| maplibre-gl.js 5.24.0, shared with the map page | 275,550 | 276,000 |
| maplibre-gl.css 5.24.0, shared with the map page | 10,078 | 10,600 |
| liberty style | 5,358 | 6,000 |
| planet TileJSON | 1,798 | 2,200 |
| one zoom 0 vector tile | 47,988 | 48,700 |
| **Total** | **340,772** | **about 343 KB** |

The globe's own code, a login only script and stylesheet built from this sketch, adds about 5.7 KB compressed (the
sketch's globe code with its comments, gzip level 5), and `globe.svg` (0.6 KB) is no longer loaded. Net, a first visit
adds about 346 KB in compressed bodies, 348 KB with headers.

**Against today.** Today's sign in page is 8 files from our own server, 31.9 KB compressed (HTML 1.0, CSS 22.8, JS 7.2,
SVG 0.9). With the globe, a first visit is about 378 KB, about 12 times today.

**Repeat visits**, extra bytes over today's page:

| When | Extra | Why |
|------|-------|-----|
| Within an hour | 0 B | Every file is still cached. |
| An hour to a day | about 6 KB | The TileJSON check (0.9 KB) and the globe's own script and stylesheet again (5.7 KB), less `globe.svg` (0.6 KB). |
| After a day | about 11 KB | The style (5.4 KB) may also come back in full. |
| After each OpenFreeMap planet build | about 50 KB more, once | A new TileJSON (1.8 KB) and a new zoom 0 tile (48 KB), because the tile address carries the build date (current build 2026-10-11). |

The library stays cached for a year. Today's page itself costs about 1 KB within the hour, and 31.9 KB after an hour or
any server restart.

**Shared with the map page.** For someone who signs in and opens the map, the 285.6 KB library is downloaded once either
way, because the URL, SRI, `crossorigin` and site match `index.html.tmpl:22-24,205-207`. Across a first session the new
bytes are about 54 to 61 KB: the zoom 0 tile and the globe's own code, plus the style and TileJSON if those are not
shared (the sketch fetches the style itself, with credentials omitted). This is reasoning only, not checked against the
running app. Someone who stops at the sign in page pays the full 346 KB.

For comparison, zoom 1 tiles (`tz=1`) fetched three tiles for the first view (345 KB, 640 KB in all) and would fetch a
fourth during a turn, for coastlines that look only slightly finer at this size. Low data view and no WebGL2: 0 bytes.

## For the build

Everything the real change has to handle. Line numbers are from main on 2026-10-17.

Phase 8 note 6 (commit 31bdcd7) replaces today's emailed link with a password and a typed 6 digit code. Items 15 and 18
belong to the link flow: they apply only if the globe is built before that change (for example in Phase 5), and go
away with it.

1. **Record the owner's override.** The 1.1 UI-SPEC banned WebGL, three.js and CDN libraries on the sign in page and asked
   for a self hosted SVG under 10 KB
   (`.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-UI-SPEC.md:143-171`). The owner overrode it
   (the 2026-09-16 todo, and 2026-10-17: "I want like the actual globe rotating"). Its reasons stay as rules: never slow
   or block the form, nothing for Low data, and a plan for devices that cannot draw it (here: no globe). The todo's range
   `auth.css:29-71` is now `auth.css:35-74`.
2. **Load order.** The same URLs and SRI as `web/templates/index.html.tmpl:22-24,205-207`, with `crossorigin="anonymous"`
   and `referrerpolicy="no-referrer"`. Never a blocking tag (the map libraries can hang,
   `web/slow_notice_contract_test.go:10-12`), never in the head, and never a `defer` tag ahead of `auth.js`: deferred
   scripts run in order, and Send works only once `auth.js` has wired it (`login_gate.html.tmpl:32,50`,
   `web/template_contract_test.go:20-26`). The sketch injects both files after `load` plus idle; an `async` tag
   written only when Low data is off would also do. Guard `requestIdleCallback`.
3. **Where the code lives.** A new login only script (for example `static/js/login-globe.js`) and a login only
   stylesheet. Not `theme.js` or any script the Low page links (`web/low_contract_test.go:204-209,251-256` bans unpkg,
   `createElement('script')` and the rest there), and not `auth.css`, which the Low page links and counts against its
   byte budget (`web/templates/low.html.tmpl:10`, `web/low_budget_contract_test.go:40-55,287-291,313-320`). Both new
   files are served like today's static files: a one hour cache (`internal/api/router.go:615`), no ETag or
   Last-Modified, and a `?v=` that changes at every server start (`cmd/server/main.go:189`), so they download again
   after an hour or a restart.
4. **Low data.** When `LowDataDefault` is true (`internal/api/handlers/auth.go:55-58,98-106`), write no globe markup, no
   script, no stylesheet and no preconnect. The panel words stay exactly as they are (`login_gate.html.tmpl:37-46`,
   `web/ways_in_contract_test.go:96-110`).
5. **No picture fallback, so today's globe goes.** Removing `globe.svg`, `.globe-bg__mask` and its keyframes
   (`auth.css:35-74`; markup in `login_gate.html.tmpl:14-15` and `verify_outcome.html.tmpl:15-16`) touches every test
   and doc that names them: `web/layout_rules_contract_test.go:144-145` (TestNoOpacityText allows an opacity only on
   `.globe-bg`, and its reason describes the mask picture at 35 percent), `web/low_contract_test.go:230-244` (the `url()`
   exemption for `.globe-bg__mask`), `web/low_budget_contract_test.go:134-153,185-192` (the one inert exception and its
   fixture) and `docs/MODERATION.md:254`.
6. **Fading.** Dim with `opacity` on the wrapper only, the one place TestNoOpacityText allows. The sketch uses 0.5 light
   and 0.4 dark for A (today 0.35), and 1 and 0.9 for B, so the allow entry's value and reason change. The fade in uses
   `opacity: 0` only inside a keyframe, which the test allows. Never set opacity from a script.
7. **Failure paths.** Probe `getContext('webgl2')` before downloading anything (the same probe as `map.js:223-235`);
   `try/catch` around `new maplibregl.Map`; an `error` event before `load` turns the globe off, and so does nothing
   having drawn 10 seconds after the download started (a hung download fires neither `load` nor `error`; the count
   waits while the tab is hidden, and a library or style that arrives later creates no map); stop on
   `webglcontextlost` and resume on `webglcontextrestored`. Pass `contextType: 'webgl2'`, or 5.24.0 quietly falls back to
   WebGL1. A failure after the first paint keeps the globe's empty space, so the card never moves (open question 8).
8. **Reduce motion and hidden tabs.** A script loop ignores the CSS media query today's globe relies on
   (`auth.css:70-74`), so read `matchMedia('(prefers-reduced-motion: reduce)')`, listen for `change`, and draw one still
   frame (`01.1-UI-SPEC.md:172-174`). Stop the loop on `visibilitychange`.
9. **Credits.** `attributionControl: false`, and a credit line outside the `aria-hidden`, `pointer-events: none`, faded
   wrapper (`login_gate.html.tmpl:14`, `auth.css:35-44`), with the same three links as `map.js:307-311`, built with
   `createElement` and `textContent` (`web/js_sinks_contract_test.go:195-198`). The credit uses the 14px label size
   (12px is kept to the closed list, `.planning/phases/03.4-app-layout/03.4-UI-SPEC.md:169`) and 44px tall links, like
   `.login-ways__links a` (`auth.css:280-289`). On a phone the credit wraps to two 44px rows with a gap between them.
   Do not close the gap with padding on inline links: the rows' tap areas then overlap, and a tap on the first line can
   open the second line's link (measured, see Checks run). For the realistic Earth the credit is "Earth pictures: NASA
   Blue Marble via NASA GIBS", linking to NASA's pages, and in A it gets a solid page coloured background
   (`background: var(--color-bg)`), because over the photo the plain credit measured under 4.5:1 in light mode.
10. **The form first, one map.** Create the map only after the page has loaded, and never create it again when the card
    swaps (`01.1-UI-SPEC.md:259-268`, `auth.js:109-114`).
11. **Size from the sign in area.** Never a bare `100vh` or `100dvh` on a wrapper
    (`web/chrome_css_contract_test.go:187-226`, `auth.css:13-23`); `--disclaimer-bar-height` is 0px on `/login`
    (`web/static/css/main.css:100-109`). Size from the area's width and first screen height, so the card swap never
    moves the globe, and solve the zoom for the size it looks on screen.
12. **Style and tiles.** Realistic Earth: write the style in the script, with one raster source (the GIBS address,
    `tileSize: 256`, `maxzoom: 3`) and one raster layer, nothing else. Map colours: fetch the reviewed liberty URL and
    trim it as above, with the openmaptiles source capped at `maxzoom: 0` (a source `maxzoom` overrides the TileJSON's),
    and drop any sky block. For either, the tests should pin the tile address or style URL and the zoom cap.
    No test covers a
    `new maplibregl.Map(` today: `web/js_contract_test.go:21-34,101-200` checks only `L.maplibreGL(` calls, `297-312`
    only `map.js` and `modal.js`, and
    the vendor tag test reads only `index.html.tmpl` (`web/template_contract_test.go:20-33`). New tests should pin the
    globe's style URL and tile cap, its WebGL2 probe, and the login tags' URL, SRI, `crossorigin` and non blocking load.
13. **Dark mode.** The realistic Earth is the same photo in both themes; only the dim changes. Map colours: the same light
    globe in both themes, like the app map (the owner reverted a dark basemap on 2026-09-23, commit 77b9804,
    `.planning/debug/resolved/map-theme-not-applied.md:4`, `main.css:55-56`); only the dim changes.
14. **No location.** The whole Earth, India in the middle, no geolocation and no `/api` call (they answer 401 before
    sign in), so no tile request reveals where the visitor is (D-05, `01.1-UI-SPEC.md:255-258`,
    `internal/api/router.go:441-452`).
15. **Referrer** (link flow; after note 6 it matters only if a new screen, such as the code screen, puts the email in
    its address). After Send the address holds the email (`?sent=`, `auth.go:91-95`, `auth.js:109-114`). MapLibre sets
    each request's referrer to the full page address itself, so a `referrerpolicy` on the tags never reaches its tile
    requests. Today the browser default trims it to the origin (measured above, for OpenFreeMap only; the realistic
    Earth's requests go to NASA GIBS); pin that with a page policy, for example `Referrer-Policy: same-origin` on
    `/login`, which still sends the Origin header on the same site POST.
16. **Content Security Policy.** None is sent today (`internal/api/router.go:349-372`, `auth.go:108-109`), so nothing
    blocks the globe. If one is added: `script-src` and `style-src` `https://unpkg.com`, `worker-src blob:` (MapLibre
    starts its worker from a blob), and `connect-src https://tiles.openfreemap.org` (style, TileJSON and tile). The
    trimmed style loads no images, so `img-src` needs nothing new (`.planning/phases/01-foundation-report-map/01-11-RESEARCH.md:53,145`).
    For the realistic Earth, allow `https://gibs.earthdata.nasa.gov` in both `connect-src` and `img-src`; which of the
    two MapLibre's picture tiles need was not confirmed.
17. **Policy text.** The Policy says "The map is drawn with files from unpkg.com and map tiles from OpenFreeMap"
    (`web/templates/legal_body.html.tmpl:95`). The globe makes those requests on the sign in page, before anyone has
    agreed to anything (consent is saved inside the gate, `router.go:561`). The sentence should name the sign in page;
    the owner picks the words. The realistic Earth adds NASA GIBS (`gibs.earthdata.nasa.gov`) as another host contacted
    before consent, in place of OpenFreeMap on this page.
18. **The pages the email link opens** (link flow; note 6 removes these pages, and its new screens, Create account, the
    code screen and Forgot password, need the same globe or plain choice). `verify_outcome.html.tmpl:10,14-16` shares
    today's globe, and the success page moves on after 2 seconds. The sketch keeps the globe on its Email verified
    screen so the look can be judged; see the open questions.
19. **Template rules.** Keep the ids `login-gate`, `login-form-panel`, `login-email`, `request-link` and `auth-error` once
    each, the column order, and no top bar or theme control (`web/ways_in_contract_test.go:78-112`,
    `web/reader_contract_test.go:215-224`, `web/chrome_contract_test.go:55,91-97`, `web/theme_contract_test.go:386-404`).
    Globe markup goes inside `#login-gate` after `.login-column` (B shows it first with `order: -1`), so the heading and
    the email field come first for keyboards and screen readers. B's credit is last in Tab order on purpose, even though
    it sits above the card on the phone. No em or en dash anywhere in a template, comments included
    (`web/design_rules_contract_test.go:60-75`).
20. **No layout shift.** Give everything the globe uses its space from the first paint. Write the credit line with the
    page and hide it with `visibility: hidden` (not `hidden` or `display: none`), then show it with the same
    `.is-ready` class as the globe, so the card never moves when the globe arrives; `visibility: hidden` also keeps its
    links out of the Tab order until then. In A, put the credit in the page flow under the Low data panel wherever
    the corner would collide with it (phones, and laptop windows narrower than 1200px and shorter than 800px).
21. **The pictures on every visit.** GIBS forbids browser caching (`no-store` on every address tried), so each visit
    downloads the pictures again (Download weight), and NASA gives no uptime guarantee. One option to weigh at planning:
    our server fetches the 85 tiles (746 KB) once, keeps them in memory and serves them with a long cache. Returning
    visitors would then download nothing, visitors' addresses would never reach NASA, and a NASA outage after the first
    fetch would not matter. Check it against NASA's terms for GIBS first, since the `no-store` header may be asking
    clients not to keep copies. Not as files in the repo (no image files in the project). Without it, the realistic
    Earth costs about 100 to 380 KB on every visit.

## Found while building (the current app, not changed here)

- Right after Send, the Check your inbox sentence reads "We sent a verification link to . Open it", because the hidden
  panel was drawn before there was an address (`web/templates/check_inbox.html.tmpl:3`) and `auth.js` only unhides it.
  The address appears after a reload. The sketch shows the address. Link flow: it goes away with Phase 8 note 6, and
  until then filling it in with `textContent` is a one line fix.
- The error line under the email field shows in grey 16px body text, not the red 14px that `.auth-error` asks for,
  because `.login-card p` (`auth.css:97`) outranks `.auth-error` (`auth.css:201`). The sketch keeps the grey to match
  today. Not the link: it still applies to any form that keeps this error line.

More, from reading the code for this sketch (2026-10-17, read only, none of it opened in a browser). Recorded here,
nothing changed. Phase 8 note 6 (commit 31bdcd7) removes the emailed link, so the items marked "link flow" go away with
it; the owner decides on the rest when Phase 8 is planned.

- Pressing Enter in the email field does nothing. The field is not inside a form; only the Send button is wired
  (`web/static/js/auth.js`), so the browser never checks the field either and an empty address comes back from the
  server as "Enter a valid email address." Today's form, not the link: it lasts until note 6 rebuilds the form.
- "Check again" never lets anyone in. It reloads `/login?sent=<email>`, and `/login` never looks at the session, so even
  after the email link has signed this browser in, it reloads the same waiting panel. The way in is the tab the email
  link opened, or the main address. Link flow.
- A signed in person who opens `/login` sees the sign in form, because `/login` never looks at the session. Not the
  link: it still applies.
- A refused Resend shows nothing. Its error line is inside the sign in card, which is hidden while Check your inbox
  shows, so "Too many requests. Try again in a minute." never appears there. Link flow; a Resend on the new code screen
  could repeat it.
- The 5 minute notice says "This link has expired. Request a new one above.", but Resend link sits below it. Link flow.
- After a reload (Check again) no timers run: Resend is ready at once and the 5 minute notice never appears. Link flow.
- The waiting panel has no way back to change a mistyped address. Link flow; the new code screen faces the same
  question.
- "Send new link" on the expired, already used and not valid pages opens `/login` with an empty field; the address is
  not carried over (`web/templates/verify_outcome.html.tmpl`). Link flow.
- "This link was already used" also shows when the same browser was just signed in by that link, for example when the
  email link is opened twice. Link flow.

## Open questions for the owner

1. **A or B?** Answered 2026-10-17: the owner picked **A**, behind the card ("i like A"), after looking at both. The
   sketch had recommended B, kept here as the record: on a phone the card covers nearly all of A's globe (only its top and bottom edges
   show, and only because the sketch makes it wider than the phone), while B shows the whole Earth on both sizes, and the
   phone is the main device. One cost of B on small phones: the globe and its credit push the card down, so on a 375 by
   548 or 320 by 568 screen the email field and Send start below the first screen (the email field at 575px and 607px;
   the credit wraps to two lines at 320) and need a scroll. On a 390 by 844 phone everything down to Send fits.
2. **Which phase builds it?** Answered 2026-10-17: **Phase 5** ("add that task to phase 5"; ROADMAP Phase 5 success
   criterion 5 and its 2026-10-17 owner decisions). The recommendation was Phase 5, which already changes the sign in screen with the Demo button
   and the explainer video link (`.planning/ROADMAP.md:592,616,619`). Phase 8 now changes this page too (Create account
   and Sign in, scope note 6), so whichever of 5 and 8 is planned second builds on the other's `login_gate.html.tmpl`,
   and Phase 8's mockup should show the globe as Phase 5 built it.
3. **Country borders?** Map colours only: the realistic Earth has no borders, so this matters only if Map colours comes
   back. The globe starts on India, and the map data's borders are not India's official map. Read from the
   tile the globe uses (2026-10-17): in Kashmir the border follows the Line of Control as a plain country border, and the
   line between Ladakh and Aksai Chin, with short stretches further south east, is marked disputed and drawn dotted.
   India's official map shows all of Jammu and Kashmir and Ladakh inside India. Recommendation: **hide them** on the
   globe; it is calmer and leaves nothing on the first screen to dispute. The Country borders switch shows both. If they
   stay, include them in the legal review CLAUDE.md already asks for.
4. **Blue sea?** Answered 2026-10-17: the owner approved the realistic Earth ("yes i like the earth"), so the colours are
   the satellite pictures' own. This overrides the 1.1 design rules' grey globe with no blue sea; record it with item 1
   of For the build.
5. **Dark mode.** The photo looks the same in both themes. A dims it a little more in dark (40 percent) than in light
   (50 percent), and B shows it at full strength in light and 90 percent in dark. Recommendation: **keep it**. Look
   with the Theme switch.
6. **The pages the email link opens** (only if the globe is built before note 6 removes the link). Recommendation:
   **the globe on the sign in page only**. The success page moves on
   after 2 seconds, so its download would be thrown away; those pages would get a plain background.
7. **Policy words.** The sign in page now contacts unpkg.com and NASA GIBS (OpenFreeMap for Map colours) before anyone
   agrees to anything.
   Recommendation: add the sign in page to the existing Policy sentence; you pick the words.
8. **If the globe fails after the page has shown, keep its empty space or close it?** This happens only when a download
   is refused or nothing has drawn after 10 seconds. Keeping the space means the card never moves, but there is an empty
   area where the globe would be (in B, left of the card on a laptop and above it on a phone). Closing it removes the
   gap but moves the card, perhaps while someone is typing: in B, 298px left on a 1440 by 900 laptop and 249px up on a
   390 by 844 phone; in A, 48px down on that phone and not at all on a laptop. Recommendation: **keep the space**, since
   a card that jumps under someone's finger is worse than a rare empty area. The sketch keeps it.
