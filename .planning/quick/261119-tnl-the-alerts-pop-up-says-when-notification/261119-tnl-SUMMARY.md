---
quick_id: 261119-tnl
slug: the-alerts-pop-up-says-when-notification
date: 2026-11-19
status: complete
commits:
  - 4c92774f fix(welcome): the alerts pop-up says when notifications are blocked
---

# Quick 261119-tnl Summary: the sign up alerts pop-up says when notifications are blocked

After Allow in the sign up alerts pop-up, a browser or device that refuses notifications now keeps the
pop-up open with one line and one OK button, instead of finishing sign up with no word. Closes the
7.1 UAT gap for test 2.

## What changed

- **web/templates/welcome.html.tmpl:** the alerts pop-up holds a hidden line,
  `<p id="welcome-alerts-off-line" role="status" hidden>`, reading "Notifications are turned off for
  Pinalert. Allow them in your browser or device settings to get alerts.", after the error line and
  before the buttons. A hidden OK button, `welcome-alerts-ok` (primary, `aria-describedby` the line),
  sits after Allow in the same row.
- **web/static/js/welcome.js:**
  - `onAskAllow` keeps what `PinalertNotify.enable` answered in `askAnswer`, so a second Allow after
    a failed home area save still knows the answer.
  - The home area save runs first and keeps its own error rule. Then an answer with
    `state: 'blocked'` calls `showAskOff`:
    - Not now and Allow are hidden.
    - The line and OK show, and `aria-busy` is removed so the line is announced.
    - OK takes focus.
  - OK (`onAskOk`) sends first-run-done once. A double tap sends nothing more.
  - A failed first-run-done puts the pop-up back as drawn (`hideAskOff` in `doneFailed`).
  - Every other answer finishes as before. So does a page without the line or OK.
- **web/static/css/welcome.css:** one rule, `#welcome-alerts-off-line:not([hidden])`, sets only the
  16px margin above OK.
- **web/welcome_alerts_test.mjs:** the fake page draws the line and OK. The fake notify has `blocked`
  and `unsupported` modes. There are 15 new "blocked: " cases.
- **web/welcome_contract_test.go:** TestWelcomeAlertsAsk is amended with a dated note (2026-11-19). It
  now counts 3 paragraphs and 3 buttons, pins the exact line and the OK button (both hidden), their
  order and their ids.

## Tests

| Check | Result |
|-------|--------|
| `node --test web/*.mjs` | 1729 pass, 0 fail (welcome_alerts_test.mjs: 106 pass) |
| `go build ./...`, `go vet ./...` | clean |
| `go test -p 1 ./...` with `DATABASE_URL` on scratch `p9_tnl` | 2534 top level tests pass, 0 fail, 3 skips (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, which need outside services) |

`p9_tnl` and `p9_tnl_live` were created for this run and dropped after it.

## Live look

Setup:
- Own server on port 8783 with database `p9_tnl_live`, `ENV=development` and the log mailer.
- A throwaway VAPID pair, so the page has a push key.
- Playwright WebKit. The notification permission reads `default` at load, so the pop-up opens.
  `Notification.requestPermission` answers `denied`.
- Each run signs up a made up `@example.com` account by password and code, and walks steps 1 to 3.
  The home area is saved through the profile API, so no outside place search runs. Username and name
  are typed, then Continue.

All four runs passed: phone 390 touch light, phone 390 touch dark, laptop 1280 light and laptop
1280 dark. In each one:
- The pop-up opened with Not now and Allow, and no line.
- After Allow, the browser was asked once. The pop-up stayed open with the exact line and OK. Not now
  and Allow were hidden, and focus was on OK.
- The line sits inside the box, the box sits inside the window, nothing scrolls sideways and the box
  does not scroll.
- Escape left the pop-up open.
- OK landed on Nearby (`/`).

Screenshots (scratchpad only, not in the repo), in
`/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/tnl/shots/`:
- `phone390-light-2-blocked.png`, `phone390-dark-2-blocked.png`, `laptop1280-light-2-blocked.png`,
  `laptop1280-dark-2-blocked.png` (the line and OK)
- `*-1-ask.png` (the pop-up before Allow) and `*-3-nearby.png` (after OK) for each of the four runs

## Deviations from Plan

1. **[Rule 2] One CSS rule in web/static/css/welcome.css.** The plan named only welcome.js and the
   template. Without a rule, the line's default paragraph margins would not match the box. The rule
   sets margin only, with no display, colour or motion.
2. **[Rule 3] The fake page and fake notify in welcome_alerts_test.mjs were extended.** They now draw
   the new elements and answer `blocked` and `unsupported`, which the new cases need.
3. **Phone width run used an Android user agent.** On a Mac, WebKit with touch on reads as an iPad.
   An iPad tab gets the install pop-up, not the alerts pop-up. The iPhone simulator was not used, as
   the plan says.
4. **Escape while the line shows** does nothing at all (the pop-up stays busy). Only OK closes it, as
   the plan asks.

None of these change the title, the home area ask, the install pop-up, the Settings rows, notify.js,
the push code or anything on the server.

## Review fixes

One finding, confirmed and fixed (commits 01b11d91 and 1ae7f172).

- **Escape pressed a few times while the line and OK show stranded sign up on step 4.** WebKit and
  Chromium honour a prevented Escape only twice, then close the dialog themselves. Nothing listened
  for that close, so first-run-done never went out and Continue stayed off until a reload. Point 4
  under Deviations above came from a single press. Fix: a close listener on the alerts pop-up
  (`onAskClose` in web/static/js/welcome.js). It finishes sign up as OK does, but only while OK
  shows and has not been pressed. Any other close sends nothing. While the pop-up is open, Escape
  still adds nothing.
- Six new Node cases in web/welcome_alerts_test.mjs (commits 01b11d91 and 1ae7f172):
  a browser close while the line shows (exactly
  one first-run-done, then Nearby), a close after OK was pressed, the close after a failed
  first-run-done (nothing more, and Continue still finishes), a close while the home area save is
  out, a close before any answer, and a step change while the line shows (goTo puts the pop-up
  away only when it is not answered, so it stays open and no close can finish sign up early).
  Without the listener, the first case fails.
- welcome_contract_test.go said there must be no close listener. A dated amendment now allows this
  one listener and pins what its body does.
- Tests after both commits: `node --test web/*.mjs` 1735 pass, 0 fail.
  `go test -p 1 ./...` on a fresh scratch `p9_tnl_fix`:
  2534 top level pass, 0 fail, 3 skips (the same three as above).
- Live check on port 8783 with `p9_tnl_fix`, Playwright WebKit at 1280. The browser box answered
  denied, then Escape was pressed three times. The pop-up stayed open with focus on OK after presses
  1 and 2. Press 3 closed it, one first-run-done went out and the page landed on Nearby (`/`). The
  normal OK run also landed on `/` with one first-run-done. The server was stopped and `p9_tnl_fix`
  dropped afterwards.
