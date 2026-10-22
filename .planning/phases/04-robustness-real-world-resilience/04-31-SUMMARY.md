---
phase: 04-robustness-real-world-resilience
plan: 31
subsystem: moderation / flag
status: complete
tags: [gap-closure, flag, moderation, migration, accessibility]
requires: [04-20 flag route, 04-24 flag dialog, 04-26 moderator screen]
provides: [flag reasons as a set, screened flag note, moderator notes list]
affects: [report_flags table, POST /api/reports/{id}/flag, GET /api/moderation/queue, Flag window, moderator screen]
tech-stack:
  added: []
  patterns: [TEXT[] column with a CHECK backed by an immutable SQL function, panel owned Tab focus trap]
key-files:
  created:
    - internal/store/migrations/00010_add_flag_reasons_and_note.sql
    - internal/api/handlers/flag_note_e2e_test.go
    - web/flag_dialog_test.mjs
  modified:
    - internal/store/queries/moderation.sql
    - internal/store/queries/reports.sql
    - internal/store/sqlc/models.go
    - internal/store/sqlc/moderation.sql.go
    - internal/store/sqlc/reports.sql.go
    - internal/service/flag.go
    - internal/service/moderation.go
    - internal/service/report.go
    - internal/api/router.go
    - internal/api/handlers/flag.go
    - internal/api/handlers/moderation.go
    - docs/swagger.json, docs/swagger.yaml, docs/docs.go
    - docs/MODERATION.md
    - web/templates/flag_dialog.html.tmpl
    - web/static/js/flag.js
    - web/static/css/chrome.css
    - web/static/js/moderate.js
    - web/static/css/moderate.css
    - tests: internal/service/flag_test.go, internal/store/moderation_test.go, internal/store/migrations_test.go, internal/api/handlers/flag_e2e_test.go, web/flag_contract_test.go
decisions:
  - "Old reason column: migrated into the new reasons set in 00010 and dropped, so a flag has one place that says why; Down restores the first reason of each set"
  - "Old API field: the single reason field is still accepted on its own as a one element set; sent together with reasons it is a 400"
  - "Flag note: screened by ScreenAbuse (the abuse screen of CheckText, no gibberish or minimum length rule, owner follow up 2026-10-23) with the report service's own screener, plus the 300 character limit; refused is 422 on field note with RejectMessage; borderline or unfinished still stores it (moderators only read it, an outage must not stop a flag)"
  - "Flag window: the panel moves Tab focus itself, because Safari's default Tab skips tick boxes and buttons and dropped focus behind the window"
metrics:
  duration: about 75 minutes
  completed: 2026-10-23
  tasks: 3 commits of work
---

# Phase 4 Plan 31: Flag window redesign Summary

The Flag window now asks "Why are you flagging this report?", puts the Dispute line right under the title, takes one or more reasons as tick boxes and has an optional "Tell the moderators more (optional)" box of up to 300 characters that is screened like a report description, stored with the flag in a new `reasons TEXT[]` and `note` column (migration 00010) and shown only on the moderator screen, as text, with the reasons it came with.

## Commits

| Commit | What |
| ------ | ---- |
| f3c02f5 | Migration 00010, sqlc, flag service (set, note, screening), API (reasons, note, old reason), moderator queue JSON (`flags.notes`), swagger, Go tests |
| 4f14560 | Flag window template, flag.js, dialog CSS, moderator screen notes, MODERATION.md, contract tests, node test |
| 8523db5 | Rule 1 fix: the panel moves Tab focus itself so Safari cannot drop it behind the window |
| 8d71b0e, f02fda3 | This summary; MODERATION.md says a refused note still uses a flag token |
| 19b75f5 | Follow up (owner, 2026-10-23): flag notes skip the gibberish rule; new `ScreenAbuse`; the note's own gibberish sentence removed |

## What was built

**Storage (00010).** `report_flags.reasons TEXT[] NOT NULL` with one CHECK: at least one reason, one dimension, no NULL element, every element one of the five slugs, and no repeat (the small immutable function `report_flag_reasons_distinct`, created inside goose StatementBegin/End). `report_flags.note TEXT` with a CHECK of NULL or 1 to 300 characters. `report_flags_one_per_account` is untouched. The service stores the set in the dialog's order and the note trimmed, with an empty note as NULL.

**API.** `POST /api/reports/{id}/flag` takes `{"reasons": [...], "note": "..."}`. Validation: at least one reason (`Choose a reason to continue.` on field `reasons`), every reason known and none twice (`Choose each reason from the list once.`), a note of at most 300 characters after trimming (`Keep the note to 300 characters or fewer.` on field `note`). Body cap raised from 1 KiB to 4 KiB (300 four byte characters are about 1200 bytes; tested). Rate limits and every other rule of the flag service are unchanged, and the check order is unchanged except that the note is screened after the report, withheld and own report checks and before the insert, so a 404 or 403 never costs a screener call.

**Screening (changed by the follow up of 2026-10-23).** The note goes through `ScreenAbuse`, the screener half of the report description's `CheckText`, with the report service's own screener (new `ReportService.Screener()`, passed by the router through `WithFlagScreener`). It does not go through the gibberish or minimum length rule: the owner said a one word note such as "fake", "duplicate" or "old" is a perfectly good flag note. `CheckText` itself is unchanged and still runs the gibberish rule first for report descriptions. What stays for a note is the hateful, harassing and sexual reject of the abuse screen, borderline, and the 300 character limit. A refused note is 422 on field `note` with the description's own sentence (`RejectMessage`). The first version's note only gibberish sentence ("Say in a few words what is wrong with this report, or leave this box empty.") and `FlagNoteRejectMessage` are removed, since nothing used them any more. A borderline or unfinished check still stores the note: only moderators read it, and a provider outage must not stop a flag, which is a safety action.

**Moderator screen.** Per reason counts now unnest the set, so each reason counts once per flag it appears in. Every queue row (and Find) gains `flags.notes`: the open flags with a note, newest first, at most 20 per report, each `{reasons: [{reason, label}], note}`, never null, with nothing that names the flagger. The screen shows them under "What people wrote:", each note's reasons in muted text over the note, put on the page only with `Pinalert.setText` (a `<b>` in a note shows as typed). The note never reaches the author or any reader (tested on the feed).

**Window.** Title "Why are you flagging this report?"; "If the report is just wrong, use Dispute instead." under it, and the dialog is described by it; the five tick boxes keep today's labels and order in one fieldset named by the title; none ticked shows "Choose a reason to continue." on the group (aria-describedby) and focuses the first box; the note box has a real label, `maxlength="300"`, no hint text, and a count that appears only from 250 characters ("50 characters left"); a refused note shows the server's sentence under the box with `role="alert"`, `aria-invalid`, focus on the box, and the ticks and text kept; only opening the window clears them. The closing line and both buttons are unchanged. It is the same modal shell on the phone and the laptop.

## Old reason column and old API field

- **Old `reason` column: migrated and dropped.** 00010 copies each flag's single reason into a one element set (`UPDATE report_flags SET reasons = ARRAY[reason]`) before the column and its CHECK are dropped, so every existing flag keeps its reason and a flag has exactly one place that says why; keeping both columns would let them disagree. Down adds `reason` back from `reasons[1]` (the first in the dialog's order, because the service stores the set in that order) with the old CHECK, and drops the note, the set and the function. Down therefore keeps only the first reason of a multi reason flag and loses every note; the old schema had nowhere to keep either. `TestMigration00010` runs Down, Up, Down, Up on the scratch database and checks this.
- **Old `reason` API field: still accepted.** Alone it is read as a one element set, which is kindest to a page cached before this change (the old page sends `{"reason":"spam"}`; the existing e2e tests still send exactly that and pass). Sent together with `reasons` it is a 400 on field `body`, so a body never means two things. It is documented in swagger next to the new fields.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Safari keyboard focus fell behind the window**
- **Found during:** the WebKit keyboard only browser check.
- **Issue:** Safari's default Tab skips tick boxes and buttons. With the new note box, Tab from it moved focus to the locked page behind the window, and the next Tab came back to the box, so a keyboard user in default Safari could not reach the reasons or the buttons. The old trap only handled the wrap at the ends and left the rest to the browser.
- **Fix:** `onDialogKeydown` now moves focus itself, one stop of `focusableInPanel` at a time (all five boxes, the note box, the shown and enabled buttons), wrapping both ways, and a Tab from the title goes to the first box.
- **Files modified:** web/static/js/flag.js, web/flag_dialog_test.mjs
- **Commit:** 8523db5

**2. [Rule 3] Flag body cap.** The 1 KiB cap could not hold a 300 character note of four byte characters; raised to 4 KiB and tested with a 1200 byte note. Commit f3c02f5.

### Not changed, on purpose

- Account deletion (Option A, owner 2026-10-18) keeps the report text a deleted person wrote; their flag notes are kept the same way, still visible only to moderators and naming nobody.
- docs/MODERATION.md's Low data byte budget figures are the 04-27 measurement; `TestLowDataByteBudget` still passes, so they were not re-measured.

## Tests amended (each carries "Amended 2026-10-22 by plan 04-31 (owner)")

- internal/service/flag_test.go `TestFlagServiceRules`: each case sends its reason as a one element set (the empty case sends none); the validation field is `reasons`, an unknown reason has `FlagReasonsBadMessage`; the insert is compared with `reflect.DeepEqual`.
- internal/store/moderation_test.go `modFlag` helper and `TestReportFlagsUniquePerAccount`: flags are stored with `Reasons`; the "police" refusal is now `ARRAY['police']` on `reasons`.
- internal/api/handlers/flag_e2e_test.go `p20FlagRows` (reads `array_to_string(reasons, '+')`), `TestFlagOnePerAccount` (unknown reason: field `reasons`, `FlagReasonsBadMessage`; empty object: field `reasons`; the unknown key case sends `comment` because `note` is now known), `TestFlagSwaggerCoversRoute` (three properties: `reasons` array with the five slug enum, `note` string with maxLength 300, `reason` kept).
- web/flag_contract_test.go `p424RadioPattern` (checkbox), `TestFlagDialogMatchesServerReasons` (tick boxes, no radio group, Dispute line under the title and describing the dialog, element order, note box tag, hidden lines), `TestFlagDialogCopy` (new title, Dispute and label words, exactly one text box instead of none, count words), `TestFlagSendOutcomes` and `p424CheckAnswersAndWaits` (`sendFlag(reasons, note)`, the body, `showChooseReason`, `showTextError`, the 422 and reasons answers, the clears in `openFlagDialog`, `updateCount`).

## Tests added

`TestValidateFlagInput`, `TestFlagServiceNote` (with "fake", " duplicate " and "old" stored, with and without a screener, and a hateful note refused), `TestScreenAbuseSkipsGibberish` (ScreenAbuse clears one word text that CheckText still refuses, and refuses hateful, harassing and sexual scores) (service); `TestReportFlagsReasonsAndNote`, `TestMigration00010` (store, Up and Down on the scratch database); `TestFlagReasonsAndNoteEndToEnd` (API, screening, old field, queue JSON, no identity, note not in the feed, and "fake" and "duplicate" stored as notes); `TestFlagDialogNoteGeometry`, `TestModerateShowsFlagNotesAsText` (web contract); web/flag_dialog_test.mjs (7 node tests of the real flag.js).

## Verification

- `go build ./...` and `go vet ./...`: pass.
- `node --test web/*.mjs`: 637 tests, 637 pass, 0 fail.
- First run, on p4_w31: 3930 PASS lines (1356 top level), 0 FAIL, 2 SKIP.
- After the follow up, `go build ./...` and `go vet ./...` pass, `node --test web/*.mjs` is 637 of 637, and the full suite on a fresh p4_w31b (`env -i PATH HOME DATABASE_URL=... go test ./... -p 1 -count=1 -v`, created and dropped) gives 3932 PASS lines (1356 top level), 0 FAIL, 2 SKIP (TestGeocodeLive, TestScreeningFixture, the two allowed). TestCastVoteConcurrentSameAccountKeepsEveryRow passed first time in both runs.
- Browser, Playwright on the 8784 scratch server (env -i, ENV=development, MODERATOR_EMAILS=w31-mod@example.com, p4_w31), Chromium and WebKit, 390x844 and 1470x830, light and dark, keyboard only for the dialog: 8 of 8 runs pass every check (title, Dispute line between title and reasons, five rows of at least 44px, note box 16px text and at least 44px tall, fits the width, Tab to the boxes and Send, none ticked line and focus, Space ticks, refused note line with focus on the box and ticks and text kept, count at 260 characters, Escape closes and focus returns to Flag, no page errors). One real flag sent (201, control reads Flagged), then the moderator screen in all 8 runs shows the note as text with its reasons, no element made from `<b>`, no sideways scroll. Screenshots only in the scratchpad outside the repo. The iPhone simulator check is the orchestrator's. The browser pass ran before the follow up and used "fake" as the refused note; the window's handling of a 422 is unchanged and is covered by the node test, but the browser pass was not rerun.
- Server stopped and p4_w31 dropped at the end.

## Known Stubs

None.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: new free text input | internal/api/handlers/flag.go | The flag note is new user text stored server side. Mitigated: 300 character limit in Go and in a CHECK, the abuse screen (ScreenAbuse), 4 KiB body cap, shown only to moderators and only through setText, never with a flagger identity. |

## Self-Check: PASSED

All created files exist; commits f3c02f5, 4f14560 and 8523db5 are on the branch.
