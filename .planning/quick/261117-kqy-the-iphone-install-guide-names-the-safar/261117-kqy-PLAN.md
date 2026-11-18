---
quick_id: 261117-kqy
slug: the-iphone-install-guide-names-the-safar
date: 2026-11-17
type: quick
files_modified:
  - web/templates/device_guide.html.tmpl
  - web/static/css/device-guide.css
  - web/static/icons/ellipsis.svg
  - web/static/js/notify.js
  - web/static/js/welcome.js
  - web/static/js/settings-alerts.js
  - Go contract tests in web/ and node tests in web/*.mjs that pin step 1
---

# The iPhone install guide names the Safari 26 Menu button with three dots

## Why

7.1 UAT test 7, 2026-11-17, owner on the iPhone 17 Pro simulator (iOS 26.5): the install
pop-up says "Tap the **Share** button.", but Safari 26 on iPhone (default Compact layout)
does not show Share on screen. It is inside the three dots button at the bottom right,
next to the address. The owner: "we should be like more specific on where the share button
is on Safari", then "naming it something and then like show these three dots also. But I
think we should like say menu or something."

Older iPhone Safari (18 and earlier) shows Share in the bar at the bottom of the screen.

## What changes

Step 1 of the shared install guide partial `device_guide` (web/templates/device_guide.html.tmpl,
shown in the welcome install pop-up `welcome-install-ask` and the Settings guide window
`device-guide-window`) gets three wordings. Exactly one shows:

| Variant | When | Tile icon | Words |
|---|---|---|---|
| `menu` | iPhone Safari 26 or newer | new `ellipsis` icon (three dots) | `Tap <strong>Menu</strong> <span aria-hidden="true">(•••)</span> at the bottom right, then <strong>Share</strong>.` |
| `bar` | iPhone Safari 25 or older | `share` icon (as now) | `Tap <strong>Share</strong> at the bottom of the screen.` |
| `plain` | everything else, and the default before or without the script | `share` icon (as now) | `Tap the <strong>Share</strong> button.` (today's line, unchanged) |

The dots are three U+2022 BULLET characters in parentheses, hidden from screen readers, so
VoiceOver reads "Tap Menu at the bottom right, then Share." Steps 2 and 3, the lead line and
everything else in the guide stay exactly as they are.

### Detection (one shared function)

Add to web/static/js/notify.js and export on `window.PinalertNotify`:
`shareStep(agent, maxTouchPoints)` returning `'menu'`, `'bar'` or `'plain'` (pure, takes its
inputs so node tests can feed user agent strings), plus a small helper that applies it to a
guide root (sets the variant on the step 1 element) reading `navigator.userAgent` and
`navigator.maxTouchPoints`.

Rules:
- Only an iPhone user agent (contains `iPhone`) gets `menu` or `bar`. An iPad, a desktop-mode
  iPad (`Macintosh` with more than one touch point), and anything else get `plain` (no iPad
  checks are done in this project, so the iPad keeps today's wording).
- Safari only: the agent must carry a `Version/N` token and none of the other iOS browser
  tokens (`CriOS`, `FxiOS`, `EdgiOS`, `OPiOS`, `OPT/`, `YaBrowser`, `DuckDuckGo`, `GSA/`,
  `Brave`, `Focus`, and in-app web views with no `Version/` token). Otherwise `plain`.
- Use the major number from `Version/N`, not the `CPU iPhone OS` number: Safari 26 freezes
  that at 18_6 or 18_7 (WebKit bug 298473; e.g.
  `Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1`).
- N >= 26 gives `menu`; N < 26 gives `bar`.

Apply it in both places the guide shows: when the welcome install pop-up opens (welcome.js)
and when the Settings guide window opens (settings-alerts.js), or once at page load in each,
whichever matches the surrounding code. Follow the file's existing idiom and comment density.

### Markup and style

Pick the simplest markup that keeps today's look: one step 1 row inside the bordered list
(no extra top border on the first visible row, no empty row, the dividers between the three
visible rows unchanged), its tile showing the matching icon, its text the matching words.
Hidden variants must be `hidden` or `display: none` so screen readers skip them. The default
markup with no script shows `plain`, so nothing breaks before the script runs.

New icon web/static/icons/ellipsis.svg: Lucide static `ellipsis` (three circles r=1 at
cx 5, 12, 19, cy 12), copied in the exact format of web/static/icons/share.svg (the
`@license lucide-static` comment, `width="100%"`, `height="100%"`, stroke attributes,
`aria-hidden`, `focusable`). Add `.app-icon--ellipsis` in device-guide.css next to
`.app-icon--share`. Check the icon set contract tests (any test that lists icons or checks
the license header) and add it where they expect.

## Tests

- Node (web/*.mjs, beside the closest existing harness, e.g. notify_isolation_test.mjs or a
  new small file): `shareStep` with real agent strings:
  iOS 26 Safari (`Version/26.0`, OS frozen 18_6) gives menu; `Version/26.1` with `18_7` gives
  menu; `Version/27.0` gives menu; iOS 18.5 Safari (`Version/18.5`) gives bar; iOS 17 Safari
  gives bar; Chrome iOS (`CriOS/130`), Firefox iOS (`FxiOS/`), Edge iOS (`EdgiOS/`) give
  plain; an in-app web view with no `Version/` gives plain; iPad Safari 26 gives plain; a
  desktop-mode iPad (`Macintosh`, maxTouchPoints 5) gives plain; a Mac (`Macintosh`,
  maxTouchPoints 0) gives plain; empty or missing agent gives plain.
  Plus the apply helper on a fake guide: only the matching variant is visible.
- Go contract tests in web/ that pin step 1 (grep `Tap the <strong>Share</strong> button`,
  `app-icon--share`, `device-guide__step` in web/*_test.go and the review33/review71 fix
  contracts, settings_rows_contract_test.go, welcome_contract_test.go,
  noise_words_contract_test.go, css_contract_test.go): keep their intent and pin the three
  wordings, the default being plain, the aria-hidden dots, and the ellipsis icon rule.
- Run `node --test web/*.mjs` and `go test ./web/` (no database). If any Go test outside web/
  is touched, use your own scratch database `p9_kqy` (createdb, drop after), never
  pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view.
- Live look: Playwright WebKit (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules)
  on a static harness or a server on port 8794 only, with the iPhone 26, iPhone 18 and Chrome
  iOS agents at 320 and 390 wide: one step 1 row, right words, right icon, dividers as before,
  nothing scrolls sideways. Screenshots only in your own scratchpad subfolder
  (`<scratchpad>/kqy/`), never in the repo.

## Rules

- Fix only this. Do not touch other copy, the pop-up layout, steps 2 and 3, or the lead.
- No em or en dashes in code comments, copy or commits. No Python at all.
- Keep all wording neutral, as the owner's wording rule asks.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797.
- Write 261117-kqy-SUMMARY.md in this folder when done (what changed, test counts, the live
  look result, any deviation) and commit it.
</content>
</invoke>
