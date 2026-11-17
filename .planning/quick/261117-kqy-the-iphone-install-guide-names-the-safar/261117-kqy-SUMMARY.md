---
quick_id: 261117-kqy
slug: the-iphone-install-guide-names-the-safar
date: 2026-11-17
status: complete
commits: 1
plan_head_before: 520cf5ce6511040cce3dfe2d8554942d2195b7ee
plan_head_after: f5f256c5
key-files:
  created:
    - web/static/icons/ellipsis.svg
    - web/share_step_test.mjs
  modified:
    - web/templates/device_guide.html.tmpl
    - web/static/css/device-guide.css
    - web/static/js/notify.js
    - web/static/js/welcome.js
    - web/static/js/settings-alerts.js
    - web/noise_words_contract_test.go
    - web/css_contract_test.go
    - web/review33_templates_fix_contract_test.go
---

# Quick 261117-kqy: the iPhone install guide names the Safari 26 Menu

Step 1 of the install guide now says where Share is on each iPhone Safari:
"Tap **Menu** (•••) at the bottom right, then **Share**." on Safari 26 or
newer, "Tap **Share** at the bottom of the screen." on older Safari, and the
old "Tap the **Share** button." everywhere else.

## What changed

| Variant | When | Icon | Words |
|---|---|---|---|
| menu | iPhone Safari 26+ | new ellipsis (three dots) | Tap **Menu** (•••) at the bottom right, then **Share**. |
| bar | iPhone Safari 25 or older | share | Tap **Share** at the bottom of the screen. |
| plain | everything else, and the markup default | share | Tap the **Share** button. (unchanged) |

- `device_guide.html.tmpl`: step 1 is still one `<li>` with one tile. Its tile
  holds the share and ellipsis glyphs, and its text holds three spans. Each node
  carries `data-share-for` (the variants it belongs to). Bar and menu carry
  `hidden`, so plain shows without the script. The dots are three U+2022
  characters in an `aria-hidden` span. Steps 2 and 3 and the lead are untouched.
- `device-guide.css`: `.app-icon--ellipsis` sits next to `.app-icon--share`.
  A guard, `.device-guide__tile .app-icon[hidden] { display: none }`, is
  needed because `.app-icon { display: block }` outranks the browser's own
  `[hidden]` rule.
- `ellipsis.svg`: Lucide static `ellipsis`, in the same format as share.svg.
- `notify.js`: `shareStep(agent, maxTouchPoints)` returns menu, bar or plain.
  Only an iPhone agent with Safari's `Version/N` and none of the other iOS
  browser tokens can get menu or bar. N >= 26 gives menu. `applyShareStep(root)`
  only flips `hidden` and `data-share-step` on nodes that are already there. Both
  are exported on `window.PinalertNotify`, and the header comment names the one
  exception.
- `welcome.js` applies it when the install pop-up opens, before `showModal`.
  `settings-alerts.js` applies it once at load inside its `deviceReady` block.
  Both check that the function exists and wrap the call in try, so an older
  notify.js keeps the plain step.

## Tests

- `node --test web/*.mjs`: 1649 pass, 0 fail (the new web/share_step_test.mjs
  adds 5 tests). They cover real agents for iOS 26 (18_6 frozen), 26.1 (18_7),
  27, 18.5 and 17, plus Chrome, Firefox and Edge on iOS (Edge has a `Version/`
  token), an in-app view, iPad 26, a desktop mode iPad, a Mac, and an empty,
  missing or null agent. Every other-browser token is also checked on a
  Safari 26 string, and the apply helper runs on the real template's step 1.
- `go test ./web/`: ok, 680 top level tests pass, 0 fail, 0 skip.
  TestDeviceGuideCopy pins the three wordings, plain as the default, the hidden
  attributes, and the aria-hidden dots. The new TestDeviceGuideShareStepIcons
  pins the hidden guard and both mask rules. ellipsis.svg is registered in
  nonCategoryIcons and laterIcons. TestLowDataByteBudget still passes.
- No Go test outside web/ was touched, so no database was used.

## Live look

Playwright WebKit ran on a scratch Go harness on port 8794. The harness
rendered the real partial inside the install pop-up markup, with the welcome
page's stylesheets and the real notify.js. It checked the iPhone 26, iPhone 18
and Chrome iOS agents at 320 and 390 wide, in light and dark:

- Each run shows one step 1 row with the right words and the right icon:
  ellipsis for iPhone 26, share for the other two.
- The dividers are as before: none above step 1, 1px above steps 2 and 3.
- Nothing scrolls sideways: the page scrollWidth equals the width and the box
  scrollWidth equals its clientWidth.
- With the script skipped, the iPhone 26 agent shows the plain step.

Screenshots were kept in the scratchpad only, and the harness was stopped
afterwards.

## Deviations from plan

None in behaviour. Implementation choices the plan left open:
- `shareStep` takes `maxTouchPoints` as the plan specifies, but the iPhone token
  alone decides. A desktop mode iPad says Macintosh, so it is plain at any touch
  count, and the comment says so.
- The icon CSS contract went into review33_templates_fix_contract_test.go, next
  to the existing guide CSS test.
