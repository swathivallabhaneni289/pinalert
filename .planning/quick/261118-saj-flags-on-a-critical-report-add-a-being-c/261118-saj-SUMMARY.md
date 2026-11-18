---
quick_id: 261118-saj
slug: flags-on-a-critical-report-add-a-being-c
date: 2026-11-18
status: complete
commits: 7
plan_head_before: e98bee6070a7623d222e593da290d50d61679ddb
plan_head_after: 68878cc4
---

# Quick 261118-saj: flags on a Critical report add a Being checked mark and pause its alerts

Flags from three places no longer hide a Critical report (severity critical, or a stored Rescue needed report). The report stays shown with a small flag mark next to its tag. Hover, a tap, Enter or Space show one line, "Being checked.". Its phone alerts pause until a moderator looks, and it comes first on the moderator's flagged list, tagged Still showing. Keep takes the mark off for good, and the report then alerts like any shown Critical report. Remove works as before. Non serious reports keep the vvb hide exactly.

## What changed

1. **Migration 00028** (`00028_add_flag_check_mark.sql`, 9a08d5f5)
   - Adds `reports.flag_check_at`.
   - Up moves every serious report in `review` back to `clear` with the mark set.
   - Down drops the column. Marked reports stay `clear`.
2. **The rule** (`flag.go`, `moderation.sql`)
   - `writeFlag` reads `Serious()` from the locked row.
   - For a serious report it calls the new `MarkReportForFlagCheck`. It uses the same threshold and the same distinct-cell count, in the same transaction and under the same lock.
   - A report gets the mark only when it is shown (`clear`, or `pending` and serious), not marked yet and not settled.
   - The mark writes no bell row. Non serious reports still go through `HideReportForReview`, unchanged.
3. **One settled marker**
   - The mark is on while `flag_check_at IS NOT NULL AND review_kept_at IS NULL`.
   - `KeepReport` now also matches a clear marked report and stamps `review_kept_at`. That takes the mark off and stops flags from marking or hiding the report again.
   - Keep leaves `flag_check_at` set. That is how push tells "kept from the mark" (alerts again) from "kept after a hide" or "restored" (silent, as before).
   - These clear `flag_check_at`:
     - `RemoveReport`
     - `RestoreReport`
     - the late text check removal in `FinishCheckAndSetState`
4. **Alerts pause** (`pushfacts.go`, `pushdispatch.go`, `pushworker.go`, `push.sql`)
   - `pushReportFacts.FlagCheck` is the mark. `Kept` is now "settled and not kept from the mark".
   - While the mark is on, these queue nothing:
     - the Critical branch
     - the follow branch
     - the confirmed update branch
   - The Critical and follow branches are not marked fanned, so the first evaluation after Keep serves every reader.
   - `EnqueueChangedFromUpdates` leaves marked reports out.
   - `sendRow` skips every kind about a marked report as `not_eligible`, changed notes included.
5. **Exposure**
   - `ReportExtrasForReports` returns `being_checked`. It is the one batched read behind Nearby, focus, Past, profile and followed.
   - `ReportView.BeingChecked` carries it, and the feed answer has `being_checked`, always present and a boolean only.
   - The moderator row carries `being_checked` too, from the flagged list and the one report answer.
   - `FlagQueue` orders `review OR marked` first.
   - Swagger regenerated.
6. **The mark on screen** (f24b893b)
   - `PinalertTrust.createCheckMark` and `updateCheckMark` in `trust.js`, in their own section.
   - The mark is a 24px button with a 44px hit area and the new Lucide `flag.svg`.
   - Its name is "Being checked". It is described by its tooltip line, "Being checked.".
   - A mouse resting on it shows the line. A tap, a click, Enter or Space keep it open until a second press, Escape or a press elsewhere.
   - Escape closes the line first, through `closeOneLevel`.
   - At most one line is open on the page. A press on the mark never reaches the row or the card behind it.
   - Where it is drawn, right after the severity tag:
     - Nearby rows, in `feed.js`. Search results are rows here too.
     - The map popup, the report page and focus, all in `map.js`.
     - The Low data rows, in `low.js`.
7. **Moderator** (`moderate.js`, `moderate.html.tmpl`)
   - A marked shown report shows "Still showing" in place of the showing word.
   - It keeps Keep, Remove and its main page link.
   - The section line now reads "People used Flag this report. Hidden by flags and Still showing first, then most flags."
8. **Copy and docs** (a1f6911d)
   - Policy, moderation part, one sentence added after the hide sentence: "A Critical report flagged this way stays visible, marked as being checked, and sends no alerts until a moderator looks."
   - MODERATION.md: a new "Being checked" section, plus amended rule and moderator lines.
   - TRUST-MODEL.md: the vvb Critical bullet is struck and amended, and the Honest limits line is amended.
   - PUSH.md: a pause bullet.
   - The flag route's swagger text.

## Tests

- **`go test ./... -p 1 -count=1 -v`** on scratch `p9_saj` with `env -i`:
  - Exit 0, 22 packages ok.
  - 7041 PASS lines (2534 top level), 0 FAIL.
  - 3 SKIP: TestGeocodeLive, TestOfficialLive and TestScreeningFixture, the same three as before.
- **`node --test web/*.mjs`**: 1705 pass, 0 fail, 0 skipped.
- **`go build ./...` and `go vet ./...`**: clean.
- **New tests**
  - **Service, on Postgres** (`TestFlagCheckMark`, `TestFlagCheckConcurrent`):
    - three places mark a serious report and never hide it;
    - a stored rescue report counts as serious;
    - the medium hide is unchanged;
    - a pending serious report is marked at once and keeps the mark through the pass;
    - a late removal clears it;
    - Keep settles it, and a repeat Keep returns `ErrAlreadyHandled`;
    - a fresh set of flags neither marks nor hides it;
    - Remove, then a repeat, then Restore never re-marks;
    - a report kept after a hide is never marked;
    - list order and Find;
    - Nearby and Focus agree, and a marked report is Live.
  - **Push** (`TestPushFlagCheckPausesUntilKeep`, `TestPushFlagCheckConfirmedUpdate`, `TestPushFlagCheckChangedNotQueued`, `TestWorkerFlagCheckGate`): nothing is queued or sent for each kind, and after Keep there is one row per reader, with no repeat.
  - **Migration:** `TestMigration00028`.
  - **End to end:** `TestFlagCheckFlow` (feed, moderator, Keep 409, Remove 409, no cells or counts in the feed).
  - **Wire:** `TestFeedReportWireBeingCheckedKey`.
  - **Contract:** `web/check_mark_contract_test.go`, which pins the words, the wiring, the Policy sentence and no dashes.
  - **Node:** `web/check_mark_test.mjs` (10 tests), and one Still showing test in `moderate_remove_test.mjs`.
- **Amended pins**, each with a dated note:
  - `TestFlagReviewHide` (a critical report is not hidden).
  - `TestReviewRuleIsStateBased`, now on a non serious report.
  - The flagged section line in `moderate_contract_test.go` and `moderator_e2e_test.go`.
  - The A23 listener count and the A5 tooltip ban in `trust_contract_test.go`. Both now read `trust.js` without the mark's own section.
  - The icon registers in `css_contract_test.go`.
  - The Policy pin in `legal_e2e_test.go`.
- **Mutation check:** dropping the FlagCheck gate in the Critical branch fails `TestPushFlagCheckPausesUntilKeep`.

## Live look

- **Setup**
  - Own server on port 8793 over `p9_saj_live`, with the log mailer and `MODERATOR_EMAILS=saj-mod@example.com`.
  - Nine made up example.com accounts.
  - Playwright WebKit at 320 and 390 with touch, and 1280 with hover, light and dark. The iPhone simulator was not used.
- **Results**: 118 of 118 checks passed. They are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/saj/live-results.txt`.
- **The run, step by step**
  1. A posted a Critical report and an ordinary one. B, C and D flagged both from three positions.
  2. The ordinary report hid (`review`). The Critical report stayed on Nearby and the map, `live`, with `being_checked` true and no phone row queued.
  3. On every size and scheme the row mark sat on the tag's line with 8px of room. It was not clipped and nothing scrolled sideways.
  4. On the phone sizes, a tap showed "Being checked."; a second tap and a tap elsewhere closed it.
  5. On the laptop, hover showed it and leaving hid it. A click kept it, and Escape closed it. Enter showed it, the name was "Being checked", and Space closed it. No press on the mark opened the report.
  6. In the opened report (the popup on the laptop, the report view on the phone), the mark sat beside the tag. Its line was not clipped by the card and covered none of the report's buttons.
  7. On Low data at 320, 390 and 1280, the line was whole and covered no button.
  8. The moderator at 390 and 1280, light and dark, saw the hidden and the marked report first, "Still showing" on the marked one and the new section line.
  9. The moderator pressed Keep. The report was settled with `flag_check_at` kept and `being_checked` false.
  10. E, F and G flagged it from three new places (201 each). It stayed clear and unmarked, and the 390 row showed no mark.
- **Screenshots**, all in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/saj/shots/`:
  - Rows: `row-{320,390,1280}-{light,dark}.png` and `row-open-*.png`.
  - The opened report: `card-{320,390,1280}-{light,dark}.png` and `card-open-*.png`.
  - Low data: `low-320-light.png`, `low-390-dark.png` and `low-1280-light.png`.
  - The moderator screen: `moderator-{390,1280}-{light,dark}.png` and `moderator-after-keep-1280-light.png`.
  - After Keep: `row-after-keep-390-{light,dark}.png`.
- **Fixed from the live look** (fa6d51b1)
  - The phone row first had only 4px between the tag and the mark. The phone row gap is 4px, so the mark now adds 4px of its own.
  - On the Low data phone card at 320 the mark crowded Flag this report and its line landed on Flag. The mark now sits under the tag there, as on the laptop card.
- **Teardown**: the server was stopped, `p9_saj` and `p9_saj_live` were dropped, and the server log, binary and session state file were deleted.

## Deviations from the plan

1. **Profile lists carry no mark.**
   - The person page's live row is one link (`<a>`), and a button cannot sit inside a link. My activity draws the severity only as hidden text.
   - The row opens the report, and the report shows the mark.
   - Past and followed lists draw no visible severity tag either, so they get no mark. "Search" is the Nearby list centred on a place, which has it.
   - `being_checked` reaches every `ReportView`, so a later change can draw it there without server work.
2. **Low data rows got the mark too.** They are Nearby rows on the Low data page. The mark sits under the tag there, not beside it (see the live look).
3. **A first alert queued before the mark and skipped while it was on is not queued again for that person after Keep.** The one first alert per person and report index blocks it, which keeps "nobody gets the same alert twice". Alerts are normally sent within seconds and the mark needs three flags, so the window is small. This is documented in MODERATION.md and PUSH.md.
4. **The moderator tag replaces the showing word.** "Showing to everyone" next to "Still showing" would say the same thing twice. A marked report that already expired keeps its Expired word and gets no Still showing tag.
5. **The A5 "no hover, no tooltip" pin in `trust_contract_test.go`** now reads `trust.js` without the mark's own section. The owner asked for hover on this mark. The trust block itself still has none.
6. **The vvb test of a pending serious report** (`TestReviewRuleIsStateBased`) was rewritten on a low report, to keep proving the state based rule. Its serious case is now in `TestFlagCheckMark`.

## Known limits

- The mark is reported on finished views (Past, Ended) when one is still on, but no finished surface draws a visible tag, so nothing shows there.
- `gofmt -l` still lists the four untouched files rz6 noted: `place_test.go`, `places_e2e_test.go`, `photon.go` and `comments_test.go`.

## Self-Check: PASSED

- These exist: `00028_add_flag_check_mark.sql`, `flag_check_test.go`, `push_flag_check_db_test.go`, `pushworker_flag_check_test.go`, `flag_check_e2e_test.go`, `check_mark_test.mjs`, `check_mark_contract_test.go` and `web/static/icons/flag.svg`.
- 9a08d5f5, 655b0022, f24b893b, a1f6911d and fa6d51b1 are ancestors of HEAD.
- `.planning/STATE.md`, ROADMAP.md, REQUIREMENTS.md and the UAT files were not touched.

## Follow-up after the final check (68878cc4)

The final check found two plan items not met. Both are now built. Deviation 1 above is replaced by the first item.

1. **Profile lists carry the mark.**
   - The public profile's live row shows "Flood · Critical" as visible words, so the mark belongs there.
   - The row is one link, so the mark sits inside it after the label words, as text, not a second control:
     - its words, "Being checked.", are part of the link's name;
     - a mouse resting on it shows the line, and leaving hides it;
     - a tap or click on it shows the line instead of opening the report, until a second press, Escape or a press elsewhere (`person-reports.js`);
     - the line opens above it, so the laptop box's clipping never cuts it.
   - Keyboard users hear the words in the link and get the full mark in the opened report.
   - My activity, Past and followed lists draw no visible severity tag, so they still get no mark.
   - Files: `person_page.go` (`BeingChecked`), `person.html.tmpl`, `person.css` and `person-reports.js`.
   - Tests: `TestPersonReportRowsBeingChecked`, `web/person_check_mark_test.mjs` (4 tests) and the contract test.
2. **The existing visibility guard is extended.**
   - `TestReviewIsWithheldEverywhere` now also proves that a clear serious report resolves Live by the critical bypass at every severity, category and tally, and is listable. That is the state a marked report keeps.
   - It also proves that `ReportMeta` has only its three fields and that `visibility.go` never names the mark.
   - `TestFlagCheckFlow` now confirms a marked report and reads its comments and history (200). The hidden ordinary report answers 404 on each.
3. **After Keep without a direct evaluation.** Nothing pokes the dispatcher on Keep. The 30 second sweep (`RecentReportsForPush`) looks back `PushChangedWindow` (2 hours), which covers the 1 hour `PushInitialWindow`, so a report kept inside its first hour is served by the next sweep.

**Tests after the follow-up**
- `go test ./... -p 1 -count=1 -v` on a fresh `p9_saj` with `env -i`: exit 0, 22 packages ok. 7043 PASS lines (2535 top level), 0 FAIL, the same 3 SKIP.
- `node --test web/*.mjs`: 1709 pass, 0 fail, 0 skipped.
- `go build ./...` and `go vet ./...`: clean.

**Live look, rerun on a fresh `p9_saj_live`**
- Everything above plus the profile `/u/saj_asha` at 320, 390 and 1280, light and dark.
- 160 of 160 checks passed, in the same `live-results.txt`.
- On the profile:
  - the mark sits after "Flood · Critical" with 8px of room;
  - it is whole, and the link's words include "Being checked.";
  - a tap (phones) or hover (laptop) shows the line without leaving the page, and the line is whole;
  - a second tap, or a click then Escape, closes it.
- Screenshots: `profile-{320,390,1280}-{light,dark}.png`, in the same shots folder. The other screenshot names are unchanged from the rerun.

**Teardown**: the server was stopped, `p9_saj` and `p9_saj_live` were dropped, and the server log, binary and session state were deleted.

## Review fixes

All three review findings were confirmed in the code and fixed. None was skipped.

1. **Nearby: the line opened below the flag even at the bottom of the phone list** (should fix, `trust.css` and `trust.js`).
   - `showCheckLine` now places the line. It measures the line against every box around the mark that clips (the list, the sheet, the map), the bottom of the app bar and the top bar.
   - If the line would pass that room below the flag, and above it has more room, the mark gets `check-mark--above`, which opens the line above the flag. Closing drops it.
   - Commit `62e5d6b6`.
2. **Profile: the line opened above the mark even under the sticky top bar** (should fix, `person.css` and `person-reports.js`).
   - `showMark` does the same measure. If the line would sit under `.top-chrome` or past the top of `.app-content`, and below it has more room, the mark gets `data-tip-below`, which opens the line below the mark. Closing drops it.
   - The profile page does not load `trust.js`, so it has its own small copy of the measure. An attribute is used because the row is server drawn.
   - Commit `62e5d6b6`.
3. **docs/MODERATION.md "What readers see" left out the profile rows** (should fix).
   - The list now names the live rows of a public profile. One clause says that there the mark is text inside the row's link, not a button: a mouse resting on it or a tap shows the line, and its words are part of the link's name.
   - Commit `1476e915`.

**Tests**
- New node tests: 3 in `web/check_mark_test.mjs` (opens above when below would pass the list and the app bar; stays below with room; stays below when above has less room) and 2 in `web/person_check_mark_test.mjs` (opens below under the top bar, closing drops it; stays above with room).
- `node --test web/*.mjs`: 1714 pass, 0 fail, 0 skipped.
- `go test ./... -p 1 -count=1 -v` on a fresh `p9_saj_fix` with `env -i`: exit 0, 22 packages ok. 7043 PASS lines (2535 top level), 0 FAIL, the same 3 SKIP.

**Live look** on port 8793 over `p9_saj_fix` with the reviewer's made up example.com data, Playwright WebKit:
- Nearby, touch, flag 30px and 6px above the app bar at 320x640 (light) and 320x568 (dark): the line opens above the flag, whole, nothing on top of it at its centre or corners, nothing scrolls sideways. With room (200px at 320x640, every case at 390x844) it stays below.
- Profile `/u/look_asha`, touch, mark 20px and 6px under the top bar at 320, 360 and 390: the line opens below the mark, whole and on top. At 44px it stays above.
- Laptop 1280, light and dark, hover: Nearby stays below, profile stays above, both whole.
- One probe at 320x568 with the flag 200px above the bar did not open: the flag had scrolled under the list's sticky head (`#feed-count`), so the tap hit the head. That is the probe's scroll, not the line; at 100px it opens below and whole.
- Screenshots in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/saj-fix/shots/`: `rows-edge-320x640-{6,30,200}.png`, `rows-edge-320x568-{6,30,100}.png`, `rows-edge-390x844-{6,30,200}.png`, `profile-topedge-{320,360,390}-{6,20,44}.png`, `{nearby,profile}-hover-1280-{light,dark}.png`.

**Teardown**: the server was stopped, `p9_saj_fix` was dropped, and the server binary, log and session state were deleted.
