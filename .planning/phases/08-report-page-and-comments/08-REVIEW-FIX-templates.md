---
phase: 08
part: templates
fixed_at: 2026-10-29T15:00:00Z
review_path: .planning/phases/08-report-page-and-comments/08-REVIEW-templates.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
accepted: 1
status: all_fixed
---

# Phase 8: Code Review Fix Report, templates part

**Source review:** 08-REVIEW-templates.md, triage in 08-REVIEW.md
**Scope:** WR-T01 to WR-T07, IN-T02 and IN-T03. IN-T01 is accepted: the Policy date and CurrentTermsVersion are not moved.

Each finding has a failing test first, then the fix, in its own commit. The new pins are in `web/review08_templates_fix_contract_test.go`.

## Checks

- `go test ./web/` passes (674 tests).
- `node --test web/*.mjs` passes: 1422 pass, 0 fail.
- The full Go suite passed on the scratch database p8_fix_t with 0 FAIL: 20 packages ok, 6200 PASS lines and 3 SKIP. The database was dropped after the run.

## Fixed

### WR-T01: the laptop comment box is pinned to the panel bottom
**Commit:** 34ca2320. **Files:** `web/static/css/report-panel.css`, `web/comments_contract_test.go`, the new test file.
- The panel box is now `position: fixed`, with the panel's own left, width, right hairline and bottom, and margin 0. A sticky box only held while `.comments` was on screen.
- The popup content keeps `--panel-composer-height` free at the end of the scroll, and 44px more while Replying to shows. That height is the box at rest, built from its tokens.
- `TestPanelComposerPinnedToPanelBottom` checks this against the panel rule's own values. `TestComposerPinnedOnLaptopOnly` was amended (dated note) to accept fixed.
- Measured on a static copy of the panel markup with the real stylesheets, at 1280x800 in Chromium and WebKit, light and dark:
  - the box bottom is 800 at scroll 0 and at the end;
  - the box is 79px high, left 0 and 520 wide;
  - the last comment ends 23.7px above the box, also while replying.
- **Left over, needs human verification:** a taller field or an error line while typing can cover the last lines. The real fix is to move the box node to be the last child of the popup content, which is in comments.js or map.js (the browser fixer's files). The owner's laptop check should confirm the real app has no transform on an ancestor.

### WR-T02: the public page names its icon
**Commit:** dbae4119. **Files:** `web/templates/share_status.html.tmpl`, the new test file.
Added `<link rel="icon" href="/icons/app.svg" type="image/svg+xml">` to the head. The server fixer owns the `/favicon.ico` skip.

### WR-T03: no ring on the Comments heading
**Commit:** 14b01a98. **Files:** `web/static/css/comments.css`, the new test file.
Added `.comments__heading:focus { outline: none; }`. The heading is never a Tab stop.

### WR-T04: the Policy says every checked comment keeps its check result
**Commit:** 5843e380. **Files:** `web/templates/legal_body.html.tmpl`, the new test file.
The line now says that for every comment the check's result, its scores and which model gave them are kept. The pin reads only the template, not the service code.

### WR-T05: the Policy and README name everything the public page shows
**Commit:** 647c4dfb. **Files:** `web/templates/legal_body.html.tmpl`, `README.md`, the new test file.
Both now name the severity, what is happening for a Critical report and the confirmation count. The pin also checks that the page still shows those fields.

### WR-T06: TRUST-MODEL Sharing bullet 1
**Commit:** 07b9e6e0. **Files:** `docs/TRUST-MODEL.md`, the new test file.
Bullet 1 now says a held or too old report answers like an unknown one, and a removed one says it was removed (D-06). `TestTrustModelDocMatchesCode` stays green.

### WR-T07: pins for the muted overrides and the pinned box
**Commit:** d507b0ae. **Files:** the new test file.
- `TestPhoneSheetHasNoMutedText` needs a `--color-text` override (or display none) in phone.css's narrow block for every muted line in comments.css and report-page.css.
- The placeholder and the off Post are allow-listed, each with a reason: both sit on the field's `--color-bg`.
- To prove the test bites, one override was removed: the test failed, then the file was restored.
- The pinned box is held by the WR-T01 test, and the public page icon by the WR-T02 test.

### IN-T02: the services line names comment checks
**Commit:** 6380b51f. **Files:** `web/templates/legal_body.html.tmpl`, `web/review71_templates_fix_contract_test.go` (amended, dated note).
The line now reads "Report, comment and username checks: OpenAI, as described above."

### IN-T03: the Share buttons share one row at 320 wide
**Commit:** 25158785. **Files:** `web/static/css/share.css`, the new test file.
- The side padding is now 8px. This is only the buttons' least room, so they still look the same.
- Measured in Chromium and WebKit: at 320 both buttons are 140 by 44 on one row, with "Copy link" and with "Link copied". At 360 they are 160 each.

## Accepted

IN-T01: as triaged, the owner moves the date and the terms version together on deploy day.

---

_Fixer: Claude (gsd-code-fixer), templates part_
