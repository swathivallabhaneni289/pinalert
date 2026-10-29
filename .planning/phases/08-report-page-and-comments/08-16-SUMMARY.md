---
phase: 08-report-page-and-comments
plan: 16
subsystem: web
tags: [comments, report-page, composer, vanilla-js, css]
requires:
  - phase: 08-05
    provides: PinalertReportPage.create with the report-page__comments mount
  - phase: 08-07
    provides: the laptop panel and report-panel.css
  - phase: 08-10
    provides: comment_count and comments_open on feed reports
  - phase: 08-12
    provides: the comment routes (list, post) and their error fields
provides:
  - window.PinalertComments.create(report, mount) with update, open, close, refresh
  - comments.css (shared and laptop arrangement), the phone arrangement in phone.css, the pinned box in report-panel.css
  - five new contract tests and a 16 test node file
affects: [08-17, 08-18, 08-20]
tech-stack:
  added: []
  patterns: [node map by comment id so a poll updates and never rebuilds, one time node per arrangement with the other display none, key events stopped at the form]
key-files:
  created:
    - web/static/js/comments.js
    - web/static/css/comments.css
    - web/comments_test.mjs
    - web/comments_contract_test.go
  modified:
    - web/static/js/report-page.js
    - web/report_page_test.mjs
    - web/static/css/phone.css
    - web/static/css/report-panel.css
    - web/templates/index.html.tmpl
    - web/template_contract_test.go
    - web/layout_rules_contract_test.go
    - web/labels_contract_test.go
key-decisions:
  - "The textarea carries no length attribute (review W2); the UI-SPEC calls maxlength a guard, the plan and TestComposerCountsCodePoints win."
  - "A comment draws its time twice in the DOM (head line and meta line), one per arrangement, the other display none, so the laptop can run name, tag and words as one inline line."
  - "The box stays hidden until the first list answer arrives, so the open flag and the empty line are known before it shows."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 16: The comments list, threads and box Summary

The report page now lists comments with folded replies and posts through one box on the phone sheet and the laptop panel, every word as a text node, every server outcome shown, and a poll that never rebuilds the list or loses typed words.

## What was built

### comments.js (commit e4bf5c03)

`window.PinalertComments.create(report, mount)` returns `{ el, update, open, close, refresh }`. Constants on their own lines: `COMMENTS_POLL_MS = 30000`, `COMMENTS_TIMEOUT_MS = 10000`, `COMMENT_MAX = 280`, `COUNTER_FROM = 40`.

- List: GET `/api/reports/{id}/comments` on open, every 30 s while the tab is visible, after your own post and when the feed's `comment_count` changes. A node map by id updates or inserts nodes; a poll removes only ids inside the range it covered, so Show more pages stay and a failed held comment still vanishes. A poll never resets the Show more cursor once a page was loaded.
- Words: text nodes only. The one bold part is a `strong.comment__mention` made with createElement, drawn only for a reply whose words start with exactly `@` + `reply_to_author` + a space. `@ndma Evacuate now` stays plain. The fake element in the node test throws on any innerHTML or outerHTML write.
- Names and discs: own comments show the username and no tag; `is_reporter` shows `span.comment__tag` "Reporter" and the shipped `icon-badge icon-badge--sm` disc with the category glyph and the severity class; New member and Deleted account show the user glyph disc; deleted and removed show the empty disc and the placeholder line with no Reply.
- Threads: one level, `View N replies` / `View 1 reply` / `Hide replies` with `aria-expanded` and `aria-controls`; posting a reply opens its thread; open state lives in a map so a poll keeps it.
- The box: Reply puts `@name ` in only for a person shown by a username who is not you; Cancel and Escape remove the inserted mention only when nothing was typed after it; code point counter with `N left` from 40 and a status that speaks at 40, 10 and 0 only; Enter posts, Shift with Enter adds a line; Post is `aria-disabled` when empty or posting and reads `Posting…`; every key event stops at the form so the map never sees typing.
- Failures keep the words: 400, 422, 429 and 409 (field username) show the server's sentence under the field with `role="alert"`; 409 field comments replaces the box with `Comments are closed`; offline and any other failure use the plan's two sentences.
- States: three skeleton comments with `aria-busy`, the error block with Try again (focus goes to the heading), the offline sentence, `No comments yet.`, `Comments are closed`, the held line in place of Reply, `Show more comments` with `?after=next_after` and its inline error.

### report-page.js

Builds the section into its `report-page__comments` mount only when `window.PinalertComments` exists, forwards `update`, and calls `open()` / `close()` only while a host shows the page and it is opened (so the hidden laptop bubble never polls comments).

### Looks and contracts (commit d10cc12c)

- `comments.css`: shared and laptop arrangement; every selector carries `.leaflet-container .report-page .comments` (four classes) so it outranks Leaflet's paragraph reset.
- `phone.css` narrow block: username, tag and time on the top line, words in Body 16 under them, small text on `--color-text` (the sheet is tinted).
- `report-panel.css` min-width block: the box `position: sticky; bottom: 0`, full panel width, `--shadow-sheet` on focus.
- `index.html.tmpl`: `comments.css` after `report-page.css` and before `report-panel.css`; `comments.js` deferred after `share.js` and before `map.js`. The Phase 5 banner, the alerts ask, the changes ask, `data-push-key`, the share window and the Demo strip were left untouched.

## For plan 08-17 (the comment node classes)

```
li.comment[.comment--reply]
  div.comment__row
    span.comment__disc  (aria-hidden)
    div.comment__body
      p.comment__head      span.comment__author, span.comment__tag, time.comment__time--head
      p.comment__words
      p.comment__gone      (deleted or removed line)
      div.comment__meta    time.comment__time--meta, button.comment__reply, span.comment__held
  div.comment__replies (roots only)   ul.comment-thread, button.comment-thread__toggle
```

Insertion point for Like, Delete and Flag: append to `div.comment__meta` right after `button.comment__reply` (before `span.comment__held` if the held line should stay last). The record object (`rec.meta`, `rec.reply`, `rec.data`) is built in `makeRecord` and rewritten in `writeComment`; add the controls in `makeRecord`, show or hide them in `writeComment` from `data.can_delete`, `data.viewer_liked`, `data.viewer_flagged`, `data.likes`, and use `commentsOpen` for the closed rules (Like and Reply hidden, Delete and Flag kept). `placeholder` records hide the whole meta line today; Delete and Flag stay hidden there, as the UI-SPEC wants. A `p.inline-error` under a comment's meta line for `rate_like` can sit after `div.comment__meta` in `div.comment__body`.

## Tests

- `node --test web/*.mjs`: 1404 tests, all pass. `web/comments_test.mjs` has 25 tests, including the sixteen named by the plan: "text only and a strong mention", "own comment shows the username and no tag", "placeholders and names", "folded replies", "reply fills the box", "cancel removes only the inserted mention", "counter counts code points", "enter posts and shift enter adds a line", "escape never reaches the map", "rejected keeps the words", "rate limited keeps the words", "offline keeps the words", "closed replaces the box", "held own comment", "show more", "poll keeps text, focus and open thread". `web/report_page_test.mjs` gained "the comments section is built into the mount and follows the page".
- `DATABASE_URL= go test ./web/ -count=1` and `-short`: ok. Named run printed PASS for TestComposerIsNotAPill, TestComposerCountsCodePoints, TestCommentsHiddenGuard, TestComposerPinnedOnLaptopOnly, TestCommentsAreLinkedInOrder, TestNoMarkupSinksAnywhere, TestNoPillShapedControls, TestNoOpacityText, TestNewControlsMeetTouchTarget, TestSizeOnlyElementsAreNamed, TestNoUnderlineAtRest, TestVendorMapScriptsLoadInDependencyOrder, TestUserVisibleCopyUsesPlainPunctuation.
- `go vet ./...` and `gofmt -l web` print nothing. No database, server or port was used.
- NOT checked: how the list and box look and behave in a real browser (iPhone simulator Safari, Mac Safari at 1280: soft keyboard, the pinned box, a long comment's counter, arrow keys, plus, minus and Escape in the laptop box). That is the owner's viewing (08-20).

## Contract tests amended (dated 2026-10-29, "by Phase 8 plan 16")

- `labels_contract_test.go`: a separate if block exempts the comment disc head (`.comment__disc:not([hidden])`, radius 50 percent) from the no fully rounded control rule.
- `layout_rules_contract_test.go`: `touchControls` gains `.comment__reply`, `.comment-thread__toggle`, `.comments__more`, `.composer__post`, `.composer__cancel`, `.comments__retry`; `phaseScripts` gains `comments.js`; `sizeOnlyNamed` gains `.comment__time--head` and `.comment__time--meta` with their reasons.
- `template_contract_test.go`: `comments.js` joins the app modules that must load after the vendor bridge.

## Deviations from Plan

**1. [Adapted] Two time nodes per comment.** The plan lists one `time.comment__time` inside `p.comment__head`. The laptop arrangement needs the name, tag and words as one running line followed by a meta line that starts with the time, which one DOM position cannot do. A second `time.comment__time--meta` sits at the start of `div.comment__meta`; each size shows one and the other is `display: none` (named in `sizeOnlyNamed`). The plan's meta line also gained the time and a `span.comment__held`.

**2. [Adapted] Plan versus UI-SPEC on maxlength.** The UI-SPEC says maxlength is "a guard"; the plan, review W2 and the new test say no length attribute. The plan won: over 280 shows a negative `N left` and the server's own refusal sentence.

**3. [Adapted] The box's own letter disc.** The page carries no username in the DOM. The disc shows the user glyph until the person's own username is seen on one of their comments (a list answer or a 201), then their letter.

**4. [Adapted] A person with no username.** There is no client side check; the 409 field `username` sentence from the server ("Choose a username in Settings first.") is shown under the field with the words kept.

**5. [Adapted] The box waits for the first list.** It stays hidden through the skeleton and a first load error, so the open flag and the empty line are known before it appears; after the first answer it is never torn down.

None of these changed a rule 1 to 3 behaviour; no auth gate or checkpoint occurred.

## Known Stubs

None. Like, Delete and Flag are not stubs: they belong to 08-17 and 08-18.

## Threat Flags

None. No new endpoint, auth path or storage; the script only calls the two routes of 08-12 it was given.

## Self-Check

Files present: comments.js, comments.css, comments_test.mjs, comments_contract_test.go and the edits listed above. Commits present: e4bf5c03 (comments.js, report-page.js, tests), d10cc12c (looks and contract pins). No em or en dash and no `!important` in the added lines; the only non ASCII characters are the two U+2026 ellipses the UI-SPEC writes and a Telugu word in the node test. STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched.

## Self-Check: PASSED
