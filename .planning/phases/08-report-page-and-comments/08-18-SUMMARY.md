---
phase: 08-report-page-and-comments
plan: 18
subsystem: web
tags: [flag-dialog, moderation, comments, vanilla-js]
requires:
  - phase: 08-12
    provides: POST /api/comments/{id}/flag
  - phase: 08-13
    provides: GET /api/moderation/comments and the keep and remove routes
  - phase: 08-16
    provides: comments.js (precondition check)
provides:
  - PinalertFlag.openForComment({ commentId, opener, onSent })
  - the moderator screen's Comments section
affects: [08-17, 08-19]
tech-stack:
  added: []
  patterns: [one dialog with a mode switch in showDialog, comment rows keyed by data-mod-comment-key and routed through the existing root click listener]
key-files:
  created: []
  modified:
    - web/static/js/flag.js
    - web/flag_dialog_test.mjs
    - web/flag_contract_test.go
    - web/static/js/moderate.js
    - web/templates/moderate.html.tmpl
    - web/static/css/moderate.css
    - web/moderate_contract_test.go
key-decisions:
  - "Report mode writes its own title back on every open, so a comment open can never leave the comment title behind; flag_dialog.html.tmpl is unchanged."
  - "Comment rows use their own data attributes (data-mod-comment-key, data-mod-comment-action) and the existing delegated click listener, so the file still has exactly three addEventListener calls and one fetch call."
  - "Remove comment posts at once, as the plan says; there is no inline confirmation like the report Remove has."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 18: Comment flag mode and moderator Comments section Summary

The built Flag dialog now flags comments in its own words through `openForComment`, and the moderator screen lists flagged, near the line and unchecked comments with Keep and Remove, all built from text nodes.

## What was built

### Task 1: Flag dialog comment mode (commit c7d2bdc4)

- `flag.js`: `current.target` is `'report'` or `'comment'`. `openFlagDialog` and the new `openForComment({ commentId, opener, onSent })` both call `showDialog`, which writes the title ("Why are you flagging this report?" or "Why are you flagging this comment?") through `Pinalert.setText`, hides `#flag-dialog-dispute` in comment mode and drops the panel's `aria-describedby` (restored in report mode).
- `flagPath()` picks `/api/comments/{id}/flag` or `/api/reports/{id}/flag`; the request, body and timeout are the same single `fetch`.
- Answers in comment mode: 201 shows "Flag sent. Moderators will see it."; 409 shows "You already flagged this comment. Moderators will see it."; both call `onSent` after the dialog closes and the toast shows. 404 shows "This comment is no longer available." 422 and 429 behave as for reports. A comment flag never touches a report control. Focus returns to the opener.
- `flag_dialog.html.tmpl` is unchanged.
- Tests: `flag_dialog_test.mjs` gained "comment mode title", "comment mode hides the dispute line", "comment mode posts to the comment path", "comment mode repeat toast", "comment mode shows a refused note and a gone comment in its own words" and "report mode unchanged"; `TestFlagDialogCommentMode` added.

### Task 2: Moderator Comments section (commit 9db7d4f7)

- `moderate.html.tmpl`: a fourth `section.mod-section` with the h2 "Comments", `ol#mod-list-comments`, count span and `p.mod-empty#mod-empty-comments` reading exactly "No comments to review." (P8-C19, W12).
- `moderate.js`: `loadComments` fetches `/api/moderation/comments` right after the report queue (inside `loadLists`, so Refresh lists covers both). Rows carry "Comment {id}", the reason words joined with the middle dot, "{Category} report {report id}" and the time, the comment words, the flag count and flag reasons when flagged, Keep comment and Remove comment buttons, and a one line explanation. A press posts to `/api/moderation/comments/{id}/keep` or `/remove`, sets `aria-disabled` plus `btn--off` while in flight, removes the row on 200, and shows the existing error line (404 and 409 say "This comment was already handled. Refresh the lists."). Everything goes through `Pinalert.setText`.
- `moderate.css`: one rule, `.mod-row__text--comment { white-space: pre-line; }`, so typed line breaks survive.
- Test: `TestModerateCommentsSection`.

## Tests

- `node --test web/*.mjs`: 1410 pass, 0 fail (13 in flag_dialog_test.mjs, 6 new).
- `DATABASE_URL= go test ./web/ -run TestFlag -v`: all 12 pass including `TestFlagDialogCommentMode`.
- `DATABASE_URL= go test ./web/ -run 'TestModerateCommentsSection|TestNoMarkupSinksAnywhere|TestUserVisibleCopyUsesPlainPunctuation' -v`: pass.
- `DATABASE_URL= go test ./... -short -count=1` and `go vet ./...`: clean. No database was used.

## Deviations from Plan

### Amended tests (rule 9, each carries a dated note in the file)

- `flag_contract_test.go` `TestFlagSendOutcomes`: the request path moved into `flagPath`, so sendFlag now asserts `fetch(flagPath()` and a separate assertion keeps the report path text in `flagPath`.
- `flag_contract_test.go` `TestFlagSendOutcomes` (clearing order) and `TestFlagDialogLocksPage` (open order): the open steps moved from `openFlagDialog` into `showDialog`; the same orders are now asserted on `showDialog`, and `openFlagDialog` must reach it.
- `moderate_contract_test.go` `TestModeratePageAssets`: `<h2` and `class="mod-section"` counts 3 to 4. The `mod-section__line` count stays 3 (no hint line was added).
- `moderate.js` `loadLists`: the refresh line and toast now speak for the queue and the comment list together (noted in the code).

No auto-fixed bugs and no auth gates.

## Notes for later plans

- 08-17 calls `PinalertFlag.openForComment({ commentId, opener, onSent })`; the page that holds the comment control must load `flag.js` and include the `flag_dialog.html.tmpl` partial (index and Low pages already do).
- `onSent` runs after a 201 or an already flagged answer, after the dialog closed and focus returned to the opener, so 08-17 can relabel or disable its control there.
- In comment mode, if the opener is not shown when the dialog closes, focus is left where the browser puts it (there is no row to fall back to).
- Remove comment has no confirmation step, as the plan specifies; if the owner wants the report style confirmation later, `buildRemoveConfirm` is the pattern.

## Known Stubs

None.

## Threat Flags

None. T-08-77 (comment words to a moderator): `moderate.js` writes only through `Pinalert.setText`, and `TestNoMarkupSinksAnywhere` and `TestModerateCommentsSection` pass. T-08-79: `report mode unchanged` passes.

## Self-Check: PASSED

Files present: the seven modified files listed above and this summary. Commits present: c7d2bdc4 (Task 1), 9db7d4f7 (Task 2). Only the middle dot (U+00B7, already used in the file) is non-ASCII in the added lines; no em or en dash.
