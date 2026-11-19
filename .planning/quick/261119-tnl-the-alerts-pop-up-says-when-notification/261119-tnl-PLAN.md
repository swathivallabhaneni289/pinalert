---
quick_id: 261119-tnl
slug: the-alerts-pop-up-says-when-notification
date: 2026-11-19
type: quick
---

# The sign up alerts pop-up says when notifications are blocked, and where to allow them

Closes the open 7.1 UAT gap for test 2 ("After Allow in the sign up alerts pop-up, a person whose
browser refuses notifications is told so, and where to allow them").

## Why

Found on 2026-11-03: the owner pressed Allow in the sign up alerts pop-up on Mac Safari while Safari
held Deny for the site. `onAskAllow` in web/static/js/welcome.js ignores what
`PinalertNotify.enable` answers, so sign up finished with no message and alerts stayed off. Asked on
2026-11-19 with a recommended line; the owner: "You can just like tell them to like allow them by
making sure they go to the settings or something. I guess that's how usually websites do, if you
want to like, if they want to like let us use the location or something."

## What changes (web/static/js/welcome.js, web/templates/welcome.html.tmpl)

- When Allow's `PinalertNotify.enable` answers `state: 'blocked'` (the browser already refuses, or
  the person just chose Don't Allow in the browser's or the Mac's own box), the pop-up does not
  close. Everything else Allow does still happens first (the home area save keeps its own error
  rule). The pop-up then shows one line in place of its buttons' question:
  **Notifications are turned off for Pinalert. Allow them in your browser or device settings to get
  alerts.**
  and one button, **OK**, which finishes sign up exactly as Allow does today (`postDone`). Not now
  and Allow are hidden while the line shows. The words cover a browser tab and the Dock or Home
  Screen app, where the switch lives in the device's own settings.
- Every other answer (`ok: true`, a dismissed box answered as `ready` and `refused`, `unsupported`,
  `offline`, `session`, a thrown error, or no answer) behaves exactly as today.
- Behaviour like the other boxes built on `.logout-confirm`: the line is announced (role status or
  alert, as the box's existing error line does), focus moves to OK, only OK closes it, Escape does
  nothing new, the page stays still. Same on phone and laptop.
- The Settings row's own Blocked line is not changed.

## Tests

- Node (the existing welcome test file): Allow answered `blocked` keeps the pop-up open, shows the
  line and OK, hides Not now and Allow, does not call postDone until OK; OK calls postDone once; the
  home area is still saved before the line shows; every other answer finishes as today (one case
  each for ok, ready refused, unsupported, a rejected promise); a double tap on OK finishes once.
- Contract: the exact line and the OK button in welcome.html.tmpl, hidden by default; no em or en
  dash; if TestWelcomeAlertsAsk pins the dialog's buttons, amend it with a dated note.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your own
  scratch database `p9_tnl` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: your own server on port 8783 with a scratch database, ENV=development and the log
  mailer, a made up example.com account signing up. Playwright WebKit
  (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules) with notification
  permission denied for the context (or `Notification.requestPermission` stubbed to answer
  'denied'): after step 4 press Allow, the line and OK show, OK lands on Nearby. At 390 (touch) and
  1280, light and dark: the line fits the box, nothing scrolls sideways. Screenshots only in your
  own scratchpad subfolder, never in the repo. Do not drive the iPhone simulator.

## Rules

- Fix only this. Do not change the pop-up's title, the home area ask, the install pop-up, the
  Settings rows, notify.js's answers, push code or anything server side.
- No em or en dashes in code comments, copy or commits. Never run python or python3, not even
  --version.
- Keep all wording neutral, as the owner's wording rule asks.
- Path limited commits (`git commit -- <paths>`; for a new file `git add -- <path>` first), each
  ending with the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage
  .planning/STATE.md, ROADMAP.md or REQUIREMENTS.md. Do not edit UAT files.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782, 8788, 8789 or 8797 (8788 and 8789 belong to
  quick 261119-sps, running at the same time). Never touch the databases pinalert_test, pinalert,
  p31_4_uat, p34_uat, p3_view, p4_uat, p9_view or any p9_sps database, and never run a migration on
  any database but your own scratch ones.
- Write 261119-tnl-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
