---
quick_id: 261118-srq
slug: settings-gets-an-add-to-dock-row-on-mac-
date: 2026-11-18
status: complete
commits: 2
plan_head_before: f4049806d808aef1b37450b5b943f84da470c911
plan_head_after: 1091ea75
---

# Quick 261118-srq: Settings gets Add to Dock on Mac Safari and Install app in Chrome and Edge

One new first row in About Pinalert. Safari 17 or newer in a Mac tab shows **Add to Dock**, which
opens a small window with the two steps. A browser that offers a one click install (Chrome and
Edge) shows **Install app**, which brings up the browser's own confirm box. Every other browser
gets no row. Settings only, no pop-up.

## What changed

- **web/templates/settings.html.tmpl**: a hidden wrapper `settings-app-row` first in the About
  Pinalert box (above Disclaimer). It holds two hidden buttons: `settings-dock-row` (Add to Dock,
  chevron, `aria-haspopup="dialog"`) and `settings-install-row` (Install app, no chevron). There is
  no sub line. A new body level `settings-dock-window` (an `app-window app-window--edit` with its X)
  titled Add to Dock holds a `device-guide__steps` list with two tiles: square-plus with "Choose
  **File** > **Add to Dock**." and map-pin with "Click **Add**, then open Pinalert from your Dock."
  There is no lead line and no hint line. `install-app.js` loads in the head, not deferred.
- **web/static/js/notify.js**: three pure functions sit next to `isInstalled`, `isAppleMobile` and
  `isMacDockApp`. `isMacSafari(agent, touch)` checks for Safari 17 or newer on a Mac; a desktop
  mode iPad, Chrome, Edge, Firefox, Opera and DuckDuckGo are excluded. `installRow(agent, touch,
  installed, hasOffer)` answers 'dock', 'install' or ''. `installRowHere(hasOffer)` runs
  installRow on this browser. The header line from decision 5 gets a dated amendment. notify.js
  still has no listener and never names the install event, so TestNotifyLeavesInstallToTheBrowser
  holds unchanged.
- **web/static/js/install-app.js** (new): only Settings loads it, in the head. It listens for
  `beforeinstallprompt`, calls preventDefault and keeps the event, and listens for `appinstalled`.
  It exposes `window.PinalertInstall` (hasOffer, isDone, isBusy, prompt, watch). `prompt()` shows
  each offer once, and a second tap while the box is showing does nothing. An accepted choice or
  appinstalled marks it done. A dismissed choice does not.
- **web/static/js/settings-alerts.js**: a new "Add to Dock and Install app" section. `drawAppRow`
  asks `installRowHere` and shows one row or none. The dock window joins the existing `windows`
  registry, so it opens with focus on its title, locks the page, keeps Tab inside, and closes with
  the X or Escape with focus back on the row. Install app calls `PinalertInstall.prompt()` from
  the tap and redraws when the answer comes. A dismissed choice keeps the row, while an accepted
  one or appinstalled hides it. Without its markup, notify.js or install-app.js, the script
  draws no row.
- **web/static/css/places.css**: one rule, `.place-page--settings .place-box > [hidden]:first-child
  + *::before { content: none; }`. A hidden first child is still a sibling, so without this rule
  Disclaimer drew a hairline along the top of the box.

## Tests

- New `web/settings_app_row_test.mjs` (12 tests) covers:
  - the installRow matrix:
    - Mac Safari 17, 18 and 26 in a tab show Add to Dock;
    - Mac Safari 16 shows no row;
    - Mac Safari when already installed shows none;
    - a desktop mode iPad, iPhone and iPad show none;
    - Chrome and Edge on a Mac show nothing without an offer and Install app with one;
    - Chrome on Windows and Android with an offer show Install app;
    - Firefox and DuckDuckGo show none;
  - installRowHere with standalone display mode;
  - install-app.js: the offer is kept and preventDefault is called; prompt runs once and a second
    tap does nothing; accepted, dismissed and a re-offer; appinstalled; a throwing prompt; a
    throwing watcher; no writes to the page and no requests.
- `web/settings_alerts_test.mjs` gains 12 tests on the real Settings script:
  - the row is hidden with no answer, on an iPhone, Firefox or installed app, and without its
    markup;
  - Add to Dock shows alone;
  - the window opens with focus on its title; the X and Escape close it with focus back on the
    row; Tab stays inside; a click on the window closes nothing;
  - with the real install-app.js loaded: the offer shows the row; prompt is called once; dismissed
    keeps the row and a tap with no new offer prompts nothing; accepted hides it; appinstalled
    hides it.
- New `web/settings_app_row_contract_test.go` (4 tests) pins:
  - the exact strings and the row's place, first in About Pinalert above Disclaimer, Policy and
    Low data view;
  - hidden by default (wrapper and both buttons), with a chevron on Add to Dock only;
  - the window's title, the two steps, only the X as a button, and no lead or hint line; the icon
    masks exist in a stylesheet Settings links;
  - install-app.js in the head without defer, loaded once;
  - no em or en dashes.
- Amended, with dated notes: `places_contract_test.go` now expects twelve chevrons instead of
  eleven (Add to Dock adds one). install-app.js joins `phaseScripts` in
  `layout_rules_contract_test.go` and `noiseCheckedFiles` in `noise_words_contract_test.go`.
  TestNotifyLeavesInstallToTheBrowser got a doc comment note; its assertions are unchanged.

Results:

- `node --test web/*.mjs`: 1678 tests, 1678 pass, 0 fail, 0 skipped.
- `go build ./...`: OK.
- `go vet ./...`: OK.
- `go test ./... -p 1 -count=1 -v` with `env -i` on the scratch database `p9_srq`: 6969 PASS
  lines, 0 FAIL, 3 SKIP, exit 0. The three skips are the opt-in live network tests
  (TestGeocodeLive, TestOfficialLive, TestScreeningFixture), not database skips. The first run
  had 1 FAIL, `internal/api/handlers` TestSettingsRows, which pins the Settings button ids. It
  was fixed in commit 1091ea75 (see deviation 6). `p9_srq` was dropped after the run.

## Live look

Setup: the scratch server ran on port 8794 against `p9_srq_live`, migrated and seeded with
`cmd/seedprofiles` (made up example.com people). It used the log mailer (`ENV=development`) and
signed in as the made up ravi.k@example.com with the seed's shared password. Playwright 1.63
drove it. Result: **35 checks pass, 0 fail.**

- **WebKit, Mac Safari 26 agent, 1280 wide, light and dark:**
  - Add to Dock is the first row in About Pinalert, with Disclaimer's line under it.
  - The window opens with focus on its title, reads "Add to Dock" with the two steps word for
    word, and both icons draw.
  - The panel is 480 by 260 at (400, 270) and fits the view; nothing scrolls sideways.
  - Escape and the X both close it with focus back on the row.
- **WebKit, Mac Safari 16 agent:** no row. Disclaimer is the first row with no line at the top.
- **WebKit, iPhone agent, 390 wide:**
  - No row, no stray line along the top of the box, nothing scrolls sideways.
  - The Notifications device row is as before. The scratch server has no VAPID keys, so it is
    the unsupported variant.
- **Chromium, Mac Chrome agent, 1280 wide, with a synthetic `beforeinstallprompt`:**
  - No row before the offer. The offer is kept (defaultPrevented), and then Install app shows
    first, with no chevron.
  - Two taps call `prompt()` once. A dismissed choice keeps the row; a new offer prompts again;
    accepted hides the row.
  - A separate page showed `appinstalled` hides the row. Nothing scrolls sideways.

Screenshots (scratchpad only, none in the repo):

- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/safari26-1280-light-about.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/safari26-1280-light-window.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/safari26-1280-dark-about.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/safari26-1280-dark-window.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/iphone-390-about.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/srq/chrome-1280-install-row.png

The server was stopped, `p9_srq_live` was dropped, and the server log and binary were deleted.

## Deviations from the plan

1. **The install event listener is not in notify.js.** The plan says to put the browser checks in
   notify.js, and they are there (pure functions). The existing contract test
   TestNotifyLeavesInstallToTheBrowser, from decision 5, forbids notify.js from holding
   `beforeinstallprompt`, `appinstalled`, `.prompt(` or any `addEventListener`. So the capture
   lives in a new small script, install-app.js, loaded in the head of Settings; the plan names that
   as one of its two options. Because it is Settings only, the browser's own small install bar on
   Android is stopped on the Settings page only.
2. **Chevron count contract.** `places_contract_test.go` pinned exactly eleven chevrons in
   Settings. Add to Dock needs one, so the count is now twelve, with a dated amendment note.
3. **One CSS rule added** (places.css) so a hidden first row does not leave a stray hairline at the
   top of the About Pinalert box. The plan did not name it, but without it the iPhone, Firefox and
   older Safari would show a visible defect.
4. **A tap on Install app after a dismissed choice does nothing until the browser offers again.**
   This is because a browser offer can be shown only once. The row stays, as the plan says.
5. **Step icons:** square-plus for step 1 and map-pin for step 2. The plan said "icons from the
   set" without naming them. The check icon's mask rule is only in feed.css, which Settings does
   not load, so it was not used.
6. **The server side end to end test was updated.**
   `internal/api/handlers/places_e2e_test.go` TestSettingsRows pins the exact list of Settings
   button ids. The two new hidden buttons now sit after Blocked people in that list, with a dated
   note (commit 1091ea75). That file already failed `gofmt -l` before this change because of one
   mis-indented comment line; that line was left as it is.

## Commits

- 3cb1392f feat(quick-261118-srq): Settings gets Add to Dock on Mac Safari and Install app in Chrome and Edge
- 1091ea75 test(quick-261118-srq): Settings button rows include the hidden Add to Dock and Install app rows

Nothing in Notifications, the iPhone guide or its pop-up, the alerts pop-ups, moderation, flags,
updates, the bell, the moderator page, the Policy or migrations was touched. No server change and
no migration.
