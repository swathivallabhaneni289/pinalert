---
quick_id: 261117-vvb
slug: flags-from-3-places-hide-a-post-for-revi
date: 2026-11-17
status: complete
commits: 3
plan_head_before: 2fc0eb36c2686a5d69e8f92cc8957839916ea2a6
plan_head_after: e611460c
---

# Quick 261117-vvb: flags from 3 places hide a post for review, and the bell tells the poster

When people in three different places flag a report, it is hidden until a moderator keeps it or removes it. Its author is told in the bell, on phone and laptop. Keep brings the report back for good. Remove points the author to the Policy. The flagger's position is stored only as a place cell of about 150 m, and that cell never leaves the server.

## What changed

1. **Migration 00026** (`internal/store/migrations/00026_add_review_hide.sql`, d318da49)
   - Adds `report_flags.flagger_cell`: nullable, 7 characters.
   - Adds the fourth state `review` to `reports_moderation_state_valid`.
   - Adds `reports.review_kept_at`.
   - Adds the four bell kinds `own_shown`, `own_review`, `own_kept` and `own_removed`.
   - Adds the unique index `uq_updates_own_report`, so each row is written at most once per report and kind.
   - Down deletes the four kinds and moves any `review` report back to `pending`.
2. **The flag** (`internal/service/flag.go`, handler, d318da49)
   - `FlagInput.Position` is optional. A half position is a 400 on the missing field, and a position outside the map is a 400, as on the vote route.
   - Only the cell (`voterGeohashPrecision`, 7) is stored.
   - `writeFlag` runs in one transaction (`PgxFlagTx`, wired in `cmd/server/main.go` and the router):
     1. Lock the report row (`LockReportModeration`, `FOR NO KEY UPDATE`).
     2. Recheck withheld.
     3. Insert the flag.
     4. `HideReportForReview`: clear and not kept, with open flags from at least `FlagHideThreshold` (3) distinct cells.
     5. Only when the report was hidden, write the `own_review` row.
3. **Withheld everywhere** (`visibility.go`)
   - `ModerationReview` is withheld at every severity, with resolve reason `hidden_for_review`.
   - Every read that already fails closed on withheld leaves it out.
   - SQL audit, three places:
     - the text check claim and `FinishCheckAndSetState` skip `review`;
     - the comment check claim skips held comments on a hidden report.
4. **Moderator** (`moderation.go`, `moderate.js`)
   - `act` locks the report row after the check row and picks the author's bell row from the state before:
     - Keep of a held report: `own_shown`.
     - Keep of a hidden report: `own_kept`, and stamps `review_kept_at`.
     - Remove: `own_removed`.
   - The bell row is written in the same transaction as the action.
   - `FlagQueue` lists `review` rows first. The screen marks them "Hidden by flags" and has its own Keep and Remove line.
5. **Late text check** (`modworker.go`)
   - `FinishCheckAndSetState` writes `own_shown` or `own_removed` in the same statement, only when it moved the report.
   - A pending serious report that passes gets no row, because it was already showing.
6. **Bell and full list**
   - `List` handles the four kinds before the "no longer available" branch. Each row is one fixed line, with no report words, category or trust.
   - `PolicyLink` becomes the closed wire value `link: "policy"`.
   - In `updates.js`, `rowHref` opens the Policy page for that value.
   - In `fillUpdatesPart`, `Href = "/policy"`. This is the only change to `activity.go`.
7. **Push**
   - A hidden report is withheld, so no push goes out for it.
   - The four rows are not in `EnqueueChangedFromUpdates`, so they never reach a phone.
   - `pushReportFacts.Kept` stops a kept report at the Critical and follow branches, and at send time for every kind except changed.
8. **Copy and docs** (e611460c)
   - Policy, What moderators can do: "Its author is told in the bell, whether a moderator removed it or the late automatic check did." and "One flag never hides a report. When people in three different places flag it, it is hidden until a moderator keeps or removes it. Its author is told in the bell when it is hidden, shown again or removed, and is never told who flagged it." The Ended line was not touched.
   - MODERATION.md: the rule amended with a dated note, plus a new "Hidden for review" section.
   - TRUST-MODEL.md: the gaming analysis and an Honest limits line.
   - PUSH.md: one bullet.
   - Swagger regenerated.

## Tests

- **`go test ./... -p 1 -count=1 -v`** on scratch `p9_vvb_run3` with `env -i`:
  - Exit 0, 22 packages ok.
  - 6991 PASS lines at every level (2507 top level), 0 FAIL, 3 SKIP: TestGeocodeLive, TestOfficialLive and TestScreeningFixture.
  - The baseline at 2fc0eb36 on `p9_vvb_base` had the same 3 skips, with 6922 PASS lines (2494 top level).
- **`node --test web/*.mjs`**: 1653 pass, 0 fail, 0 skipped.
- **`go build ./...` and `go vet ./...`**: clean.
- **New tests** (c92ac22b):
  - `TestFlagReviewHide`, `TestFlagReviewConcurrent`, `TestReviewUntouchedByTextCheck`, `TestReviewRuleIsStateBased`, `TestHeldCommentOnHiddenReportWaits` and `TestOwnShownAndRemovedRows`.
  - `TestReviewIsWithheldEverywhere`, `TestPushReviewHideAndKeep`, `TestServerCopyReviewHide`, `TestValidateFlagPosition`, `TestFlagStoresOnlyTheCell` and `TestMigration00026`.
  - `TestReviewHideFlow` (end to end).
  - Node tests for the Flag window position and the bell's own rows.
  - Extended: `TestWithheldReportsInNoFeed`, the rating neutrality grid, the moderator fake and the copy and swagger pins, each with a dated note.
  - Mutation check: with the report lock removed, the "second and third place at once" race fails 12 times in 15 runs. With the lock it passes every time.

## Live look

- **Setup**
  - Own server on port 8793 over `p9_vvb`, with the log mailer and `MODERATOR_EMAILS=vvb-mod@example.com`.
  - Eight made up example.com accounts, signed up through the API with the logged code. Onboarding columns were written in the scratch database.
  - Playwright WebKit drove the browser.
- **Results**: 37 of 37 checks passed. They are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/vvb/live-results.txt`.
- **The run, step by step**
  1. A posted two reports.
  2. B, C and D flagged report 1 through the real Flag window at 390 wide, each with a seeded cached fix about 330 m apart. Each body carried latitude and longitude.
  3. Two places left it shown. The third hid it, and three distinct cells were stored.
  4. It left Nearby for A, B and E.
  5. A's bell showed "Your report is hidden while it's reviewed." as a plain row.
  6. The moderator screen showed "Hidden by flags" at 390 and 1280, light and dark.
  7. Keep, pressed on the screen, brought it back and stamped it.
  8. A's bell showed "Your report is showing again. Sorry for the wait.", linking to `/?report=1`.
  9. Fresh flags from E, F and G at three new places did not hide it.
  10. Removing report 2 gave "Your report was removed after review.", linking to `/policy`.
  11. No row said "no longer available", 0 phone notes were queued, and no screen scrolled sideways.
- **Screenshots**, all in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/vvb/shots/`. Each step comes in 390 and 1280, light and dark:
  - Step 1, the hidden row in the bell: `step1-hidden-bell-{390,1280}-{light,dark}.png`.
  - Step 2, the moderator screen: `step2-moderate-{390,1280}-{light,dark}.png`.
  - Step 3, the bell with all three rows: `step3-final-bell-{390,1280}-{light,dark}.png`.
  - Step 3, the full updates list: `step3-final-list-{390,1280}-{light,dark}.png`.
- **For the owner to judge**
  - The plain hidden row has no "new" accent. The two link rows above it do (see `step3-final-bell-390-light.png`). This follows the existing pinned rule that a plain row carries no new mark, and the full list's template (`profile.html.tmpl`) belongs to quick 261117-vuf. The row still counts in the badge and is marked read like the others.
  - In the full list, the own rows sit indented where other rows show a severity dot.
- **About the binary**: the server was built before the last change, which evaluates the hide after every flag. No step shown depends on it.

## Deviations from the plan

1. **[Rule 2] Policy, What Pinalert keeps.**
   - The flags line now says a flag also keeps a rough square about 150 metres wide when the page knows where you are, never your exact spot.
   - The new stored cell needed disclosing. The plan named only the moderation lines.
2. **The Flag window line** "A flag never hides a post by itself." became "One flag never hides a post." The old line could read as a promise that flags never hide. The rest of the window is unchanged, and the line is shared with comment flags, where it is still true.
3. **[Rule 2] The comment check worker skips held comments on a hidden report**, as it does for a removed one. Otherwise a comment_on_yours row and push poke could follow for a report nobody can see.
4. **The hide is evaluated after every flag, a cellless one included.** A cellless flag still never counts toward the hide. This way a pending serious report that met the rule hides once the late check clears it (`TestReviewRuleIsStateBased`).
5. **A kept report is gated from push** at the Critical and follow branches and at send time for first alerts. Without the gate, a re-run would push it to readers who subscribed later.
6. **Added the resolve reason `hidden_for_review`** to the closed list, with tests updated.
7. **The Down migration maps `review` to `pending`**, the old schema's nearest hidden state that a moderator can still act on.
8. **Report flags survive Delete my account.** Only comment flags are deleted, so a hidden report always keeps its open flags and stays on the moderator's list until Keep or Remove.

## Known limits

- The own rows count toward `UpdatesHourlyCeiling` for change rows, like the other stored kinds. It is harmless at four rows per report at most.
- A true report can stay hidden while the only moderator is away, and its time keeps running (accepted by the owner, documented in TRUST-MODEL.md).

## Self-Check: PASSED

- The migration, `flag_review_test.go` and `review_hide_e2e_test.go` exist.
- d318da49, c92ac22b and e611460c are ancestors of HEAD.
- `.planning/STATE.md`, ROADMAP.md and REQUIREMENTS.md were not touched.
