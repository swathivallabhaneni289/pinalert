---
quick_id: 261118-srq
slug: settings-gets-an-add-to-dock-row-on-mac-
date: 2026-11-18
type: quick
---

# Settings gets an Add to Dock row on Mac Safari and a one click Install app row in Chrome and Edge

Runs in parallel with quick 261118-rz6 (moderation and the bell). Stay out of its files.

## Why

7.1 UAT test 8 (Mac Dock app), 2026-11-18. The owner had to be told where File > Add to Dock is,
then: "I think we should add something like instructions ... on MacBook, because like I think it
does on something like this on the iPhone, right? Or maybe just built in this on in this setting,
so it just adds simply." Offered a sign up pop-up plus a Settings row; the owner: "It's going to be
like a lot of pop-ups and maybe that'll be distracting for the users. We'll just add something on
the settings." Agreed: Settings only, no pop-up. A Mac Safari tab already gets alerts without the
Dock app, so this is a convenience. Then "cant you do it simultaneously".

## What changes

### 1. One row in Settings, in the About Pinalert section, first (above Disclaimer)

Built like the other rows there (`place-row`, `place-row__text`, `place-row__label`, chevron where
the row opens a window). No sub line. Phone and laptop use the same rules; which row shows depends
only on what the browser can do:

- **Mac Safari in a normal tab** (Safari 17 or newer on a Mac, the versions that have Add to Dock;
  not an iPad in desktop mode, which also says Macintosh, so use the existing `isAppleMobile`
  rule; not Chrome, Edge, Firefox or another browser on the Mac): the row **Add to Dock** opens a
  small window built like the existing `device-guide-window` (an `app-window` with its X):
  - Title: **Add to Dock**
  - Step 1: Choose **File** > **Add to Dock**.
  - Step 2: Click **Add**, then open Pinalert from your Dock.
  Reuse the device guide's step list styles (`device-guide__steps`, tiles with icons from the set);
  no lead line, no hint line.
- **A browser that offers a one click install** (it fires `beforeinstallprompt`: Chrome and Edge on
  a Mac, Windows, Linux, ChromeOS or Android): the row **Install app**, no chevron, a button that
  calls the saved event's `prompt()`. The browser's own confirm box follows. On `appinstalled`, or
  when the choice is accepted, the row hides. If the user dismisses, the row stays (the browser
  may fire the event again).
- **Everywhere else** (already installed, meaning `display-mode: standalone` or
  `navigator.standalone`; iPhone and iPad, which keep their own Home Screen guide in Notifications;
  Firefox; anything unknown): no row.
- The row is `hidden` in the HTML and shown by script only when one of the cases above is true, so
  without script nothing shows.

### 2. Code

- Put the browser checks next to the existing ones in web/static/js/notify.js (`isInstalled`,
  `isAppleMobile`, `isMacDockApp`) as small pure functions the node tests can call with a user agent
  and flags, and wire the row in the Settings script. Capture `beforeinstallprompt` early enough
  that it is not missed (it can fire before Settings' script runs; listen in a script that loads on
  every page or in the head of Settings, and keep the event).
- No server change is needed unless the template needs a flag. No migration.

## Tests

- Node: the detection matrix (Mac Safari 17, 18 and 26 in a tab: Add to Dock; Mac Safari 16: no
  row; Mac Safari installed: none; desktop mode iPad: none; iPhone: none; Chrome and Edge on a Mac
  with and without the event; Firefox: none); the Install button calls `prompt()` once and hides on
  `appinstalled` and on an accepted choice, stays on a dismissed one; the Add to Dock row opens the
  window, X closes it, focus goes in and back.
- Contract tests: the exact strings (Add to Dock, Install app, the two steps), the row's place
  (first in About Pinalert), hidden by default, no em or en dashes.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your own
  scratch database `p9_srq` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: your own server on port 8794 with a scratch database and the log mailer, a made up
  example.com account. Playwright WebKit with a Mac Safari 26 user agent at 1280 (light and dark):
  the row shows first in About Pinalert, opens the window, the window reads right and fits.
  Playwright Chromium (if installed) at 1280 with a synthetic `beforeinstallprompt`: the Install
  app row shows and calls `prompt()`. A phone width (390) in WebKit with an iPhone user agent: no
  row. Nothing scrolls sideways. Do not drive the iPhone simulator. Screenshots only in your own
  scratchpad subfolder, never in the repo.

## Rules

- Fix only this. Do not change the Notifications section, the iPhone install guide or pop-up, the
  alerts pop-ups, or anything in moderation, flags, updates, the bell, the moderator page, the
  Policy or migrations (quick 261118-rz6 works on those now).
- No em or en dashes in code comments, copy or commits. No Python at all.
- Keep all wording neutral, as the owner's wording rule asks.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md. Do not edit UAT files.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782, 8793, 8795, 8796, 8797, 8798 or 8799. Never
  touch the databases pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view, or
  any p9_rz6 database, and never run a migration on any database but your own scratch ones.
- Write 261118-srq-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
