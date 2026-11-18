---
quick_id: 261118-uoe
slug: the-review-ask-box-buttons-fit-on-one-li
date: 2026-11-18
status: complete
commits: 2
plan_head_before: bb4e8ca9b19ea63db499b561392e5b91f36e057f
plan_head_after: 7e65ecdd
---

# Quick 261118-uoe: the review ask box buttons fit on one line, and account deletion blanks review ask notes

Two follow ups to quick 261118-rz6. In the author's box for a removed report, Close and Ask for review now stack full width with the action on top, so neither label wraps. Deleting an account now blanks the notes on its asks for review and keeps the ask rows.

## What changed

1. **Account deletion blanks the notes on asks for review** (ffa69d48)
   - New query `BlankAccountReviewAskNotes` in `internal/store/queries/review_asks.sql`. It sets `note` to NULL on every ask the account made, and sqlc regenerated `review_asks.sql.go`.
   - `AccountDeletionService.DeleteAccount` runs the query in the same transaction as the rest of the scrub. It comes right after the person mutes step and just before the sessions are revoked.
   - The ask rows stay, with their report, time, answer and answering moderator. The moderator's history and the cap of 3 stay correct, and an open ask stays open without its note.
   - `docs/MODERATION.md` Retention now says that deleting an account blanks the notes on its asks for review and keeps the rest of each ask. The Policy is unchanged.
   - No migration, and nothing in `moderation.sql`, the flag code, the push code, the moderator page, report cards or visibility was touched.
2. **The poster's box buttons** (7e65ecdd, `web/static/css/updates.css`, this box only)
   - **The problem, measured in WebKit:**
     - On one line each, Close needs 92.7px and Ask for review 157.6px. With the Log out box's 8px gap, that is 258px side by side.
     - The box's button row is 238px wide at 320, so the two cannot sit side by side there even when sized to their labels.
     - In equal halves, Ask for review wrapped onto two lines at every width: 320, 360, 390 and also 1280, where the row is 310px.
   - **The fix:** the plan's fallback, as one rule for this box. `.review-ask .logout-confirm__actions` is `column-reverse` with no wrap, and its buttons are `flex: none`. The buttons stack full width at every width, Ask for review on top and Close under it, with the shared 8px gap.
   - The Log out, Delete and other `.logout-confirm` boxes keep their row of equal buttons. The words and the template are unchanged.

## Tests

- **`go test ./... -p 1 -count=1 -v`** on the scratch database `p9_uoe` with `env -i`:
  - Exit 0, 22 packages ok.
  - 7011 PASS lines at every level (2525 top level), 0 FAIL.
  - 3 SKIP: TestGeocodeLive, TestOfficialLive and TestScreeningFixture, the same three as rz6.
- **`node --test web/*.mjs`**: 1694 pass, 0 fail, 0 skipped.
- **`go build ./...` and `go vet ./...`**: clean.
- **New tests**
  - `TestDeleteAccountBlanksReviewAskNotes` (Postgres) seeds the account's asks: one answered Keep removed by a moderator and one open, plus another account's ask.
    - A deletion whose scrub fails keeps both notes.
    - A real deletion leaves both notes NULL. Both rows stay with their report, times, answer and moderator, and the open ask stays open.
    - The other account's note is untouched.
  - `TestReviewAskButtonsStack` (`web/review_ask_buttons_contract_test.go`) pins three things:
    - The scoped stacking rule, which sets no gap of its own.
    - The absence of any unscoped `.logout-confirm__actions` rule in updates.css.
    - auth.css's shared row, which is still a wrapping flex row with the 8px gap and equal `flex: 1 1 0` buttons.
- **Amended pins**, each with a dated note:
  - The deletion step order lists gained the new step: 20 to 21 steps, and 21 to 22 with a keyer.
  - The fake querier gained the method.
- **Mutation check:** pointing the blanking at the wrong account failed the new test on all three assertions. Restored before the commit.

## Live look

- **Setup**
  - Own server on port 8794 over `p9_uoe_live`, with the log mailer and `MODERATOR_EMAILS=uoe-mod@example.com`.
  - Made up example.com accounts, signed up through the API with the logged code.
  - The moderator removed a report with False, and the poster opened the box from the bell row.
  - Playwright WebKit drove the browser. The iPhone simulator was not used.
- **Before the change:** 34 of 42 checks passed. The 8 failures were Ask for review on two lines at every size and scheme. Results are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/uoe/live-results-before.txt`.
- **After the change: 77 of 77 checks passed.** The checks ran at 320, 360 and 390 (touch, tapped) and at 1280, light and dark, and found:
  - Both labels on one line.
  - Stacked with the action on top, both buttons the full row width (238, 278, 308 and 310px).
  - An 8px gap, equal to the Log out box's.
  - The box whole and on top at its corners and middle.
  - Nothing scrolling sideways.
- **The Asked. state** after an ask, at 320, 390 and 1280, light and dark: Close alone, on one line and full width, and Close closes the box.
- Results are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/uoe/live-results-after.txt`.
- **Screenshots**, all in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/uoe/shots/`:
  - `before-box-{320,360,390,1280}-{light,dark}.png`
  - `after-box-{320,360,390,1280}-{light,dark}.png`
  - `after-asked-{320,390,1280}-{light,dark}.png`
- **Teardown:** the server is stopped, `p9_uoe` and `p9_uoe_live` are dropped, and the server log (it held the typed codes) and the binary are deleted.

## Deviations from the plan

- **Stacking at every width.** Side by side does not fit at 320 even when sized to the labels (258px needed, 238px there), so the plan's fallback applies. I read "as one rule for this box" as no breakpoint: the box stacks on phone and laptop alike.
- **Tab order.** The template keeps Close before Ask for review, a markup order that `TestReviewAskBoxMarkup` pins, so `column-reverse` puts the action on top visually. Tab therefore reaches Close (bottom) before Ask for review (top). The plan named only the CSS file, so the markup was left alone. For the owner to judge.
- **Live look script.** The first after run counted the ask's `201 Created` as a failure, because the script wrongly expected 200. The script was fixed and the whole run repeated, 77 of 77.
- **Ledger.** The worktree guard refused the plan ledger file write, so `plan_head_before` is the known base bb4e8ca9, and the commit count was measured with `git rev-list --count bb4e8ca9..HEAD` before this summary.

## Self-Check: PASSED

- Commits ffa69d48 and 7e65ecdd are on the branch.
- `web/review_ask_buttons_contract_test.go` exists.
- No em or en dash in any changed file.
- No `.planning/STATE.md`, `ROADMAP.md`, `REQUIREMENTS.md` or UAT file was staged.
