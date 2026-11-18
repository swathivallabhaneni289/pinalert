---
quick_id: 261118-rz6
slug: removed-reports-get-a-reason-the-poster-
date: 2026-11-18
status: complete
commits: 4
plan_head_before: 2f38a91687e53047e781932f98e55aae1a089f42
plan_head_after: 0e106d81
---

# Quick 261118-rz6: removed reports get a reason, the poster can ask for review, the moderator can restore

A moderator now removes a report with one of five reasons. The author's bell row opens a small box that says why and lets them ask for a review, up to 3 times per report, one at a time. The moderator sees the asks first on their screen, with the note, and answers Restore or Keep removed. A Removed list lets them restore any report removed in the last 30 days. Phone and laptop are the same. Reports only; comment moderation is unchanged.

## What changed

1. **Migration 00027** (`internal/store/migrations/00027_add_removal_reasons_and_review_asks.sql`, e2ec5cb6)
   - `reports.removed_reason`: closed list of spam, abusive, false, dangerous, other and text_check.
   - `reports.removed_at`: when the current removal happened. Both are set only on a removed report (CHECK). Old removed reports have no reason; their time is backfilled.
   - `moderation_actions.reason`: the five moderator reasons, on a remove only. The action CHECK gains restore and keep_removed.
   - `report_review_asks`: report and account `ON DELETE CASCADE`, a note of at most 200 characters, the answer, when and who answered. A partial unique index allows one open ask per report.
   - Two bell kinds: `own_restored` and `own_stays_removed`. `uq_updates_own_report` now covers all six own kinds.
   - Down deletes the two new bell kinds and the restore and keep_removed audit rows, then restores the old checks and index. A restored report stays clear.
2. **Own rows repeat** (`moderation.sql`)
   - Both writers (`InsertOwnReportUpdate` and the late check's `FinishCheckAndSetState`) upsert on the same key, moving `created_at` to now and clearing `read_at`.
   - A repeated event comes back as the newest unread row, never a second row. It keeps its id, so the bell's read mark (largest id shown) still clears it.
3. **Remove needs a reason**
   - `ModerationService.Remove(ctx, id, moderator, reason)` refuses anything but the five with `ErrRemovalReasonMissing`.
   - The route reads `{"reason": ...}`: 400 on `reason` ("Pick a reason."), and 400 on `body` for a malformed body.
   - The late text check stores `text_check`. Keep is unchanged.
4. **Restore and Keep removed** (`moderation.go`)
   - Both take the check row lock, then the report row lock. They answer 409 for a report not removed, and Keep removed answers 409 with no open ask. Nothing is written on a 409.
   - Restore: removed to clear, reason and time cleared, open flags dismissed, `review_kept_at` stamped, any open ask closed as restored, a `restore` audit row and `own_restored`.
   - Keep removed: ask closed as kept_removed, a `keep_removed` audit row and `own_stays_removed`.
   - Routes `POST /api/moderation/reports/{id}/restore` and `/keep-removed`, moderator only like the others.
5. **The author's API** (`service/reviewask.go`, `handlers/reviewask.go`)
   - `GET` and `POST /api/reports/{id}/review-ask`, gated like the flag route. The answer is `{why, state}` with state can_ask, asked or no_asks_left.
   - Only the author of a removed report. Everything else is the same 404, and no session is 401.
   - The ask locks the report row, then decides open and the cap of 3 in the same transaction (`PgxReviewAskTx`, wired in `cmd/server/main.go`).
   - 409 lines: "You already asked. Wait for the answer." and "No asks left."
   - The note is trimmed: over 200 characters is a 400, blank is none. It is never sent to the text check, never logged and in no public answer.
6. **Bell and full updates list**
   - The removed and stays removed rows carry `review_report_id` while the report is still removed. `link` and `PolicyLink` are gone.
   - Those rows are buttons (`data-review-ask`) that open the box, with the new mark while unread, in the bell (`updates.js`) and the full list (`profile.html.tmpl`, `fillUpdatesPart`).
   - The back row opens `/?report=<id>`. Once a report is restored, its old removed rows are plain.
   - The box is `review_ask_box.html.tmpl`, the Log out box, included as a child of body on the five pages that load `updates.js`.
   - The box shows the words agreed with the owner: Removed, "Why: X" when known, then "Note (optional)" with Close and Ask for review, or "Asked.", or "No asks left.".
   - A 404 on open (a report restored since) closes quietly and reads the bell again.
7. **Moderator screen** (`moderate.html.tmpl`, `moderate.js`, `moderate.css`)
   - Review asks comes first: Why, "Note: ...", Restore and Keep removed.
   - Removed comes after Text check not finished: Why and Restore. Both have heading counts from the server.
   - Remove report now opens a "Why remove?" box built on `.logout-confirm`. The title takes focus. Spam, Abusive, False, Dangerous and Other are one radio group of real buttons with arrow keys and a check on the picked one. Then Cancel and Remove; Remove stays off until a reason is picked.
   - The inline "Remove this report?" confirmation and its CSS are gone.
   - Restore drops the report from both lists, Keep removed from Review asks only.
8. **Copy and docs** (194b1b8f)
   - Policy, What moderators can do, one sentence added: "The bell also says why it was removed, and its author can ask for a review there, up to 3 times." Nothing else in the Policy moved.
   - MODERATION.md: a new section on reasons, asks and the cap, Restore, Keep removed and the Removed list. The own rows table has the two new rows, the removed row no longer opens the Policy, and a dated note explains the repeat rule.
   - TRUST-MODEL.md: no sentence became false (a removal is still neutral for the rating), so it is unchanged.
   - Swagger regenerated: the review-ask routes and `review_report_id`, and the `link` field removed. The `@license.name` line is unchanged.

## Tests

- **`go test ./... -p 1 -count=1 -v`** on scratch `p9_rz6` with `env -i`:
  - Exit 0, 22 packages ok.
  - 7004 PASS lines at every level (2518 top level), 0 FAIL, 3 SKIP: TestGeocodeLive, TestOfficialLive and TestScreeningFixture, the same three as the vvb baseline.
- **`node --test web/*.mjs`**: 1670 pass, 0 fail, 0 skipped.
- **`go build ./...` and `go vet ./...`**: clean.
- **New tests** (f130c513):
  - **Service, on Postgres:** `TestRemoveNeedsAReason`, `TestLateTextCheckRecordsTextCheck`, `TestReviewAskRules`, `TestReviewAskConcurrent`, `TestRestoreAndKeepRemoved`, `TestRestoreUnderTheRowLock`, `TestRestoredReportCountsInTheRating`, `TestUpdatesListBoxRows` and `TestPushRestoredReportNeverPushes`.
  - **End to end:** `TestReviewAskFlow`.
  - **Migration:** `TestMigration00027`.
  - **Contract:** `web/review_ask_contract_test.go`.
  - **Node:** `web/moderate_remove_test.mjs` (8 tests), and 9 new box and row tests in `web/updates_test.mjs`.
- **Amended pins**, each with a dated note:
  - The moderator copy, script and CSS pins.
  - The Cancel count of the ratings box.
  - The update kind counts.
  - The audit columns.
  - The swagger enum.
  - The vvb flow's removed row.
- **Mutation checks:**
  - **Asks without the transaction.** A burst of 12 asks hits the unique index 20 times, which surfaces as errors; the test fails.
  - **Restore without the `review_kept_at` stamp.** The restored report is pushed to both readers; the push test fails.

## Live look

- **Setup**
  - Own server on port 8793 over `p9_rz6_live`, with the log mailer and `MODERATOR_EMAILS=rz6-mod@example.com`.
  - Six made up example.com accounts, signed up through the API with the logged code.
  - Playwright WebKit drove the browser. The iPhone simulator was not used.
- **Results**: 93 of 93 checks passed. They are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/rz6/live-results.txt`.
- **The run, step by step**
  1. A posted two reports.
  2. The moderator opened Remove report through Find at 320, 390 and 1280, light and dark. The Why remove? box was whole at every size: inside the viewport and on top at its corners and middle. Remove was off and the title focused on open, False turned Remove on, and Cancel removed nothing. Then Remove with False at 320 removed it (`removed false`).
  3. A's bell row "Your report was removed after review." opened the box from inside the bell dropdown at 320, 390 and 1280, light and dark. It was whole every time and said "Why: False" with the note box and Ask for review.
  4. A asked with a note. The box showed "Asked." in place, and later opens showed "Why: False" and "Asked.". Close closed it every time.
  5. The moderator saw the ask under Review asks with "Why: False" and "Note: ..." at 390 and 1280, light and dark, then pressed Keep removed. The ask left Review asks and the report stayed on Removed.
  6. A's bell showed "Your report stays removed." with the new mark. It opened the box and A asked again.
  7. The moderator pressed Restore. The report left both lists and was clear, kept and with no reason.
  8. A's bell said "Your report is back.", opening `/?report=1`, and the report was on Nearby again. Three flags from three places did not hide it.
  9. Report 2 was removed with Spam; a Remove with no reason answered 400. It showed "Why: Spam" on the Removed list at 320 and 1280. Restore straight from that list brought it back to Nearby.
  10. No screen scrolled sideways. No bell row opens the Policy. The note is in no bell answer. 0 phone notes were queued.
- **Screenshots**, all in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/rz6/shots/`:
  - The moderator's box: `step1-why-remove-{320,390,1280}-{light,dark}.png` and `step1-why-remove-picked-*.png`.
  - The author's box opened from the bell: `step2-box-{320,390,1280}-{light,dark}.png`, `step2-asked-390-light.png` and `step2-removed-bell-*.png`.
  - Review asks: `step3-review-asks-{390,1280}-{light,dark}.png`.
  - The stays removed row and the second ask: `step4-stays-bell-390-light.png` and `step4-asked-again-390-light.png`.
  - The Removed list: `step6-removed-list-{320,1280}-{light,dark}.png`.
  - The final bell and full list: `step7-final-bell-*.png` and `step7-final-list-*.png`.
- **For the owner to judge**
  - At 320 "Ask for review" wraps to two lines inside its button (`step2-box-320-dark.png`).
  - A press inside the author's box closes the bell list behind it, by the bell's existing outside press rule, so after Close focus goes to the bell.

## Deviations from the plan

1. **[Rule 2] `reports.removed_at`**, not named in the plan. The Removed list needs "last 30 days, newest first", and a late text check removal writes no audit row.
   - The column is set and cleared with the reason, and old removed reports are backfilled from the latest Remove action, else the rejecting check, else when the report was made.
   - A CHECK keeps the reason and time on removed reports only.
2. **A note over 200 characters answers 400** ("Keep the note to 200 characters or fewer.") rather than being cut. This follows the flag note. The box's `maxlength="200"` means the page never sends more.
3. **One failure line in the author's box**, "Couldn't send. Try again." It comes from the template and shows only when a request fails (no signal or a server error). The plan listed no failure state.
4. **The removed and stays removed rows open the box only while the report is still removed.** After a Restore they are plain rows, because the box would 404.
5. **Escape closes both boxes, like the Log out box.** "Only its buttons close it" was read as no outside press closing it, which holds.
6. **Find of a removed report still shows "Removed after review." with no Restore.** The plan names Restore only on the two lists.
7. **Commit order.** The Policy sentence pin (`TestReviewAskServerWords`) is in the test commit f130c513, and the sentence itself is in the docs commit 194b1b8f. f130c513 alone fails that one pin; HEAD passes. History was not rewritten.
8. **Short row lines on the two new lists**, matching the other lists' row lines:
   - Review asks: "Restore shows it again. Keep removed tells the author it stays removed."
   - Removed: "Restore shows it again and tells the author."
   - Neither section has a subline under its heading.
9. **The two new moderator sections are wired only when their parts are on the page**, like Comments. The existing ratings harness keeps working unchanged.

## Out of scope, noticed

`gofmt -l` lists four files this task did not touch: `internal/api/handlers/place_test.go`, `internal/api/handlers/places_e2e_test.go`, `internal/geocode/photon.go` and `internal/store/comments_test.go`. They were left as they are. The amended `swagger_test.go` was not gofmt clean after the docs commit; 0e106d81 formats it.

## Self-Check: PASSED

- The migration, `reviewask.go` (service and handler), `review_ask_box.html.tmpl`, `review_ask_test.go`, `review_ask_e2e_test.go`, `push_restore_db_test.go`, `moderate_remove_test.mjs` and `review_ask_contract_test.go` exist.
- e2ec5cb6, f130c513, 194b1b8f and 0e106d81 are ancestors of HEAD.
- `.planning/STATE.md`, ROADMAP.md and REQUIREMENTS.md were not touched.
- The scratch databases `p9_rz6` and `p9_rz6_live` were dropped, the server on 8793 was stopped, and its log and binary were deleted.
