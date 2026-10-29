---
phase: 08
part: templates
reviewed_commit: 67370b92
reviewed_at: 2026-10-29T12:30:00Z
depth: standard
status: issues_found
findings:
  critical: 0
  warning: 7
  info: 4
  total: 11
files_reviewed: 43
files_reviewed_list:
  - web/templates/share_status.html.tmpl
  - web/templates/share_window.html.tmpl
  - web/templates/comment_delete_confirm.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/settings.html.tmpl
  - web/templates/moderate.html.tmpl
  - web/templates/legal_body.html.tmpl
  - web/static/css/report-page.css
  - web/static/css/report-panel.css
  - web/static/css/share.css
  - web/static/css/comments.css
  - web/static/css/phone.css
  - web/static/css/public.css
  - web/static/css/moderate.css
  - web/static/css/trust.css
  - web/static/icons/link.svg
  - web/static/icons/share-2.svg
  - web/static/icons/thumbs-up.svg
  - web/static/icons/thumbs-up-filled.svg
  - web/chrome_contract_test.go
  - web/comments_contract_test.go
  - web/css_contract_test.go
  - web/flag_contract_test.go
  - web/labels_contract_test.go
  - web/layout_rules_contract_test.go
  - web/moderate_contract_test.go
  - web/mute_contract_test.go
  - web/phone_markup_contract_test.go
  - web/popup_contract_test.go
  - web/public_page_contract_test.go
  - web/pwa_contract_test.go
  - web/report_page_contract_test.go
  - web/report_panel_contract_test.go
  - web/settings_rows_contract_test.go
  - web/share_contract_test.go
  - web/template_contract_test.go
  - web/theme_contract_test.go
  - README.md
  - docs/TRUST-MODEL.md
  - docs/MODERATION.md
  - docs/PUSH.md
  - docs/docs.go
  - docs/swagger.yaml
---

# Phase 8: Code Review, templates, styles, contract tests and docs

**Reviewed:** 2026-10-29, main at 67370b92 (scope: the `(08-` commits after 88cbe8ad)
**Depth:** standard, with a browser pass.

What I read and ran:
- Every changed template and stylesheet in full, the four new icons, the Policy diff, the README and docs diffs, and the diff of every contract test in scope.
- `go test ./web/ -count=1` passes. `node --test` on `comments_test.mjs`, `popup_footer_test.mjs`, `report_page_test.mjs`, `share_window_test.mjs`, `flag_dialog_test.mjs` and `auth_flow_test.mjs` passes (125 of 125).

How the geometry was measured:
- The real server ran on port 8798 against the scratch database `p8_rev_t`, with three seeded onboarded accounts, a Critical flood report with two confirmations, four comments (one reply, one 120 character unbroken word, one like) and a share token.
- Engines and sizes:
  - The phone report page at 320x568, 360x640 and 390x844, light and dark: Chromium only.
  - The laptop panel at 1280x800 and 1440x900, light and dark: Chromium and WebKit.
  - The Share window at 320x568 and 1280x800, the public page at 320 and 1440 (light and dark, JavaScript off) and Settings at 320: Chromium and WebKit.
  - Server behaviour was probed with curl.
- Playwright WebKit is not iPhone Simulator Safari. The UI-SPEC Hand Off and the owner's own notes say only the simulator proves a phone layout, so the phone results below are not final until the owner's simulator check.
- The server was stopped and the database dropped afterwards. No screenshots were saved.

**Status:** issues_found

## Summary

No Critical. Seven Warnings:

- The laptop comment box is not pinned to the bottom of the panel. It sticks only while the Comments section is on screen.
- The public page has no icon link, so a browser's automatic `/favicon.ico` request sets a session cookie and writes a sessions row. That breaks the "no cookie" promise.
- The Comments heading draws a focus ring after a keyboard Delete, and the UI-SPEC says it must not.
- The Policy says only borderline comments keep their check scores, but every checked comment keeps them.
- The Policy and README describe the public page as showing less than it does.
- TRUST-MODEL's Sharing section contradicts itself about removed reports.
- The contract tests do not hold the muted text rule or the pinned box.

What holds, measured:

| Area | Result |
|---|---|
| Overflow | None at 320, 360 or 390 in Chromium, in either theme. |
| Reply meta line at 320 | Three 44 by 44 controls fit with 12px gaps and no overlap. |
| Contrast | Every text node is at 4.5 to 1 or more: 40 nodes in the phone sheet (Chromium), 58 in the laptop panel and the whole public page (Chromium and WebKit). |
| Muted text | Sits only on `--color-bg`. The phone sheet is tinted (`rgb(251 233 231)` light, `rgb(58 21 18)` dark), and every muted line there is switched to `--color-text` by phone.css. |
| Laptop panel | 520px wide from the top bar to the bottom edge. The map takes the rest, the Nearby list is gone, and Back is 89 by 44. |
| Open report row | 44px high, named "Open report, 4 comments". |
| Share window | The sheet sits on the bottom edge at 320x568 with both buttons on screen, and the backdrop covers the whole viewport. The title takes focus with no ring, and Tab goes to the X with a 2px ring. |
| Public page with no script | Follows dark mode through `prefers-color-scheme`. Sign in is 288 by 48 on the phone and 220 by 44 on the laptop, and Disclaimer and Policy are each 44 high. |
| Settings | The widened label wraps to three lines inside a 96px row at 320 and does not squeeze the switch. |

The hard rules hold in every added line:
- no `!important`, no dash, no pill radius and no opacity on text;
- no underlined action;
- `:not([hidden])` on every display rule of a hideable element.

Every contract test amendment carries a dated "Amended 2026-10-29 by Phase 8 plan N" note.

## Warnings

### WR-T01: The laptop comment box is not pinned to the panel bottom (sure: high, measured)

**File:** `web/static/css/report-panel.css:183-190`. The box is a child of `.comments`, built by comments.js inside `.report-page`.

**Issue:** `position: sticky; bottom: 0` is bounded by the sticky element's parent. Here the parent is the `.comments` section, not the panel's scroll box. So the box sticks only while the Comments section overlaps the panel. Measured with the same results in Chromium and WebKit, at 1280x800 and 1440x900, light and dark (wrapper scrollHeight 1451):

| Scroll position | Box top | Panel bottom | Box visible |
|---|---|---|---|
| 0 | 1172 | 800 | no |
| Middle | 770 | 800 | partly |
| End | 697 | 800 | yes |

A reader who opens a report on a laptop sees no comment box at all until they scroll down past the trust block and the history.

D-02 and the locked sketch 008 (`index.html:709` and `:757`) say "On the laptop the box is pinned to the bottom of the panel". The sketch appends `composerHtml` after the sections, as a sibling of the scrolling body (`index.html:1771`), so it is always pinned. UI-SPEC 2b says the same. The "last comment reachable" half is met: there are 16px of air at the end.

**Fix (pick one):**
- On the panel only, move the composer node to be the last child of `.leaflet-popup-content`, so its parent spans the whole scroll. That is a placement change in comments.js or map.js.
- Or pin it in report-panel.css with `position: fixed; left: 0; bottom: 0; width: 520px; box-sizing: border-box`, and give the wrapper a `padding-bottom` equal to the box height, so the last line stays reachable. This is safe because the panel popup is already `position: fixed` at left 0, which proves no ancestor has a transform.

**Test:** a browser probe at 1280x800. With the panel open at `scrollTop` 0, the composer's bottom equals the wrapper's bottom. At the end of the scroll, the last comment's bottom is at least 16px above the composer's top.

### WR-T02: The public page lets the browser's favicon request set a session cookie (sure: high on the server, medium on which browsers ask)

**File:** `web/templates/share_status.html.tmpl:18-31`. The routes are `internal/api/router.go:1218` and `internal/api/handlers/pwa.go:47-58`.

**Issue:** Every other page carries `<link rel="icon" href="/icons/app.svg" type="image/svg+xml">`:
- `app_head.html.tmpl:5`;
- the demo pages, which skip the app head, add it by hand (`demo.html.tmpl:34`, `demo_activity.html.tmpl:29`).

`share_status` has none. With no icon link, desktop browsers ask for `/favicon.ico`. That path is not in the session skip set (`/demo`, `/static`, `/r` and the PWA paths).

Measured with curl on the scratch server: `GET /favicon.ico` with no cookie answers 404 with `Set-Cookie: pinalert_session=...` and writes a sessions row. So a WhatsApp reader with no account who opens `/r/{token}` in a browser that fetches the favicon ends up with a Pinalert cookie and a row. That contradicts:
- UI-SPEC 6a ("no cookie set");
- TRUST-MODEL "Sharing" ("The page sets no cookie and writes no session row, even for a first visit");
- the reason `/r` joined the skip set (router.go:1212-1217).

`TestSharePageSetsNoCookie` checks only the page's own response, so it passes. Headless Playwright does not ask for favicons, so my browser pass could not show the request itself.

**Fix:** add `<link rel="icon" href="/icons/app.svg" type="image/svg+xml">` to the head of `share_status.html.tmpl`. `/icons/app.svg` is in `PWAPublicPaths` and skips the session. Optionally also add `/favicon.ico` to the skip set as a plain 404, for clients that ignore the link.

**Test:** in `public_page_contract_test.go`, assert that the template holds that exact icon link. Add a router test that `GET /favicon.ico` with no cookie sets no `Set-Cookie`, if the skip set is extended.

### WR-T03: The Comments heading draws a focus ring after a keyboard Delete (sure: high in Chromium, measured)

**File:** `web/static/css/comments.css:48-55`. Focus is moved at `web/static/js/comments.js:216, 730, 1388`.

**Issue:** UI-SPEC 7d says: "focus moves to the Comments heading (`tabindex="-1"`, no ring)". comments.css has no `:focus` rule for `.comments__heading`.

Measured in Chromium: Tab to Delete, press Enter, focus Delete in the box, press Enter. Focus lands on `H2.comments__heading`, with `outline-style: solid` and 2px, because `:focus-visible` matches after a keyboard action.

The Share window title (`share.css:66-68`), the Delete box title (`.logout-confirm__title:focus`) and the first-time flow all suppress this. This is the same class of defect as 07.1 WR-T06.

**Fix:**
```css
.leaflet-container .report-page .comments .comments__heading:focus {
  outline: none;
}
```
This is safe because the heading is not a Tab stop.

**Test:** a contract test that comments.css holds `.comments__heading:focus` with `outline: none`, mirroring the `.share-window__title:focus` pin.

### WR-T04: The Policy says only borderline comments keep their check scores, but every checked comment does (sure: high)

**File:** `web/templates/legal_body.html.tmpl:132`. The code is `internal/service/comments.go:348-366` and `internal/service/textcheck.go:116`.

**Issue:** The Policy reads: "A comment is checked by the same service as a report, and for a borderline one the check's result and scores are kept, without the text". The code does more:
- `ScreenAbuse` returns `Scores` and `Model` for a clear text too (textcheck.go:116).
- `Post` stores `verdict = 'pass'`, the scores JSON, the model and `checked_at` on every comment whose check answered (comments.go:348). That is every clear and every borderline comment.
- `FinishCommentCheck` does the same for a held comment once its check passes.

So the Policy understates what is kept for nearly every comment. MODERATION.md is accurate ("the words, with the screen verdict and scores").

**Fix (choose one):**
- Change the line to "for every comment the check's result, its scores and which model gave them are kept with the comment".
- Or store scores only for borderline comments, as the Policy says. That is a service change, for the server reviewer.

**Test:** a Policy pin in `review71_templates_fix_contract_test.go` style. When `comments.go` sets `arg.Verdict` outside a borderline branch, the "What Pinalert keeps" list must not say "for a borderline one".

### WR-T05: The Policy and README describe the public page as showing less than it does (sure: high)

**File:**
- `web/templates/legal_body.html.tmpl:135`;
- `README.md` ("The public page shows only the category, a rough area, whether it is still active, disputed, resolved or ended, and when");
- the page itself is `web/templates/share_status.html.tmpl:44-53`.

**Issue:** The Policy line says anyone with the link sees "only the report's kind, a rough area, its state in one short sentence and when". The page also shows:
- the severity (dot and word);
- for a Critical report with a pick, the "What is happening" value;
- the confirmation count, inside the sentence: "Confirmed by 4 people at different spots."

The README sentence also omits the severity, the count and the pick. TRUST-MODEL's Sharing bullet 3 lists them correctly. The word "only" makes both lines a promise that a reader with no account sees less than they do.

**Fix:** Policy: "can see only the report's kind and severity, a rough area, what is happening for a Critical report, its state and how many people confirmed it in one short sentence, and when". Make the README match.

**Test:** a Policy pin that names "severity" in the share line, beside the existing `TestSharePageShowsOnlyAllowedFields`.

### WR-T06: TRUST-MODEL's Sharing section contradicts itself on a removed report (sure: high)

**File:** `docs/TRUST-MODEL.md`, section "Sharing", bullets 1 and 3. The behaviour is `internal/api/handlers/public_status.go:123-126`.

**Issue:**
- Bullet 1: "a token for a held, removed or too old report answers like an unknown one".
- Bullet 3: "A report a moderator removed says so; a held, unknown or too old one reads as unavailable".

The handler answers a removed report with 410 and "This report was removed", and an unknown one with 404 and "This report is no longer available". So bullet 1 is untrue, and it is a trust model document a reviewer reads for exactly this property.

**Fix:** make bullet 1 "a token for a held or too old report answers like an unknown one; a removed one says it was removed (D-06)".

**Test:** none needed beyond the docs edit. Optionally pin that the doc does not contain "held, removed or too old report answers like an unknown one".

### WR-T07: The contract tests do not hold the muted text rule or the pinned box (sure: high)

**File:**
- `web/comments_contract_test.go:120-140` (`TestComposerPinnedOnLaptopOnly`);
- `web/phone.css` narrow block, comments overrides;
- the missing contrast pairs (`web/chrome_css_contract_test.go`, `TestPhase4ContrastPairs`).

**Issue:** Three promises have no pin that would fail if a later change broke them:
- **P8-R8.** The phone sheet is severity tinted. The rule that muted text sits only on `--color-bg` holds today only because phone.css lists, by hand, 13 selectors that switch comments.css's muted colours to `--color-text`. A new muted line in comments.css passes every test while it renders muted on the tint. UI-SPEC P8-R8 also promised named pairs for the Reporter tag, the share card band and the Open report row hover. None was added, but the token pairs those three use (`--color-text` on `--color-surface`, `--color-text-muted` on `--color-bg`) are already pinned in `TestPhase4ContrastPairs` (`web/chrome_css_contract_test.go:650-657`), so this half is only a naming gap. The override gap stands.
- **The pinned box.** `TestComposerPinnedOnLaptopOnly` checks only that `position: sticky` appears inside report-panel.css's min-width block. That is how WR-T01 shipped green.
- **The public page.** `TestSharePageSetsNoCookie` checks the page's own response, not the requests the page causes (WR-T02).

**Fix:**
- Add a test that, for every comments.css or report-page.css declaration `color: var(--color-text-muted)`, phone.css's narrow block holds a rule with the same head that sets `color: var(--color-text)`, or an allow list entry with a reason (for example the placeholder and the off Post, which sit on the field's `--color-bg`).
- Pin the box placement from WR-T01's fix: for example, that the composer is the last child of the popup content in panel mode, or that the fixed rule and the wrapper padding exist.

**Test:** the tests above.

## Info

### IN-T01: The Policy text changed again with no date or terms version change

**File:** `web/templates/legal_body.html.tmpl:105, 117, 132-135, 167`; `internal/service/profile.go:48`

Commit 378a1537 adds comment, like, share and storage lines, and says "Last updated and the terms version are not moved". The template header (line 10) and the Policy's own "When this text changes, everyone is asked to read it and agree again" are therefore not true for accounts that agreed on 2026-10-29. This is the gap 07.1 WR-T02 found. The owner accepted it as a deploy day step (07.1-17-SUMMARY:137, 07.1-REVIEW-FIX-server.md:135), and nothing is deployed, so I list it only to keep the deploy step in view. That step must now also cover the Phase 8 lines.

### IN-T02: "Other services" still names only report and username checks for OpenAI

**File:** `web/templates/legal_body.html.tmpl:150`

The new paragraph at line 105 says comments go to the same service, and the services list reads "Report and username checks: OpenAI". Suggest "Report, comment and username checks: OpenAI, as described above."

### IN-T03: The two Share buttons stack at 320 wide

**File:** `web/static/css/share.css:224-232`

At 320 the actions row is 288px wide. Copy link at `min-width: max-content` (24px padding on each side, the 20px glyph, an 8px gap and the words) needs more than half of it, so WhatsApp and Copy link wrap onto two full width rows (measured: tops 688 and 740). At 360, 390 and 430 they sit side by side at 160, 175 and 195px. UI-SPEC 5a says "The phone stacks nothing; at 200 percent text they wrap". Both rows stay 44px and on screen, so nothing breaks. Lowering the side padding to `--space-md` inside the window would keep them on one row at 320.

### IN-T04: The rate limited public page says "Wait a minute" but sends Retry-After 1

**File:** `internal/api/handlers/public_status.go:34`; the line is `service.PublicTooManyLine`

`Retry-After: 1` means one second, while the page tells the reader to wait a minute. Use `60` or the limiter's real wait. This is a handlers detail, for the server reviewer.

## Checked and clean

- **Templates:** every value in `share_status` goes through html/template, attributes included, and it has no script, no app head and no theme script.
  - The h1, `og:title` and `<title>` are one field. `og:description` is the fixed line. `og:url` and `og:image` are rooted in `BASE_URL`, never Host.
  - The share window and the delete box are a `role="dialog"` with `aria-modal` and `aria-labelledby`, and the `.logout-confirm` dialog respectively, with `tabindex="-1"` titles. The delete box is a direct child of body.
  - The moderator Comments section matches the other three, and the Settings label is P8-C17 exactly.
- **Icons:** `share-2.svg`, `link.svg`, `thumbs-up.svg` and `thumbs-up-filled.svg` exist. The filled twin differs only by `fill="currentColor"`, which still masks correctly. `message-square.svg` (used by trust.css) exists. `@keyframes share-sheet-up` is defined in share.css, and reduced motion turns off the sheet slide and both skeleton pulses.
- **Docs against code:** README's https rule for `BASE_URL` exists (`cmd/server/main.go:323-330`, `publicBaseURL`). MODERATION's Limits rows match `cmd/server/main.go:118-128` and `:167-173` (6 per 20s, 30 per 3s, 10 per 30s; backstops 600 per 200ms, 3000 per 30ms, 1000 per 1.2s). `/api/moderation/comments` is absent from `docs/swagger.yaml`, as MODERATION says, and the comment and share schemas there name no account id, email or cell.
- **Copy:** no em or en dash and no noise word in any added template, CSS comment or doc line I read.
- **Contract test amendments:** each changed assertion has a dated note and a replacement pin of the new state, and none is weakened silently:
  - chrome, pwa and theme each exempt `share_status` by name, backed by `TestPublicPageHasNoScript`;
  - the four icons join `nonCategoryIcons` and `laterIcons`;
  - flag.js's open and send order moves to `showDialog` and `flagPath`, still ordered;
  - mute's `place()` keeps the Mute, error, Flag order;
  - the moderator section count goes from 3 to 4;
  - the popup footer ban keeps its negative checks and adds positive ones;
  - the script order gains three modules;
  - the touch target and size only lists grow, each entry with its reason;
  - four 50 percent radius exemptions, each scoped to one exact head.

  The gaps are the missing pins in WR-T07.

---

_Reviewed: 2026-10-29_
_Reviewer: Claude (gsd-code-reviewer), templates part_
_Depth: standard, with a browser pass on scratch port 8798 and database p8_rev_t (both torn down)_
