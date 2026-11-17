---
quick_id: 261117-vuf
slug: my-activity-becomes-one-settings-row-wit
date: 2026-11-17
status: complete
commits: 2
plan_head_before: 2fc0eb36c2686a5d69e8f92cc8957839916ea2a6
plan_head_after: 5589027a
---

# Quick 261117-vuf: My activity is one Settings row; its page lists Confirmed, Disputed, Ended

Settings now has an Activity section with a single My activity row. The My activity page lists three
parts, Confirmed, Disputed and Ended. Posted is gone, because your posts are on your profile.
Updates is no longer a row of the list; it is the screen the bell's See all updates opens. Phone and
laptop behave the same.

## What changed

| Where | Now |
|---|---|
| Settings (both sizes) | The first section is titled **Activity**. It holds one plain link row, **My activity**, to `/profile`, with a chevron. `ActivityLinks`, `activityLinks` and `activityPartFragment` are gone. |
| My activity list and laptop stack | Shows exactly **Confirmed**, **Disputed** and **Ended**, in that order. The hub lists those three, and the laptop stacks the same three sections. The back arrow goes to `/settings` and no bar item is current. |
| Part screens | The phone title is the one word name. The back arrow goes to `/settings`. |
| Posted | The `posted` part, post cards, the reporter record card, the page's Reopen and the "used both undos" sentence (`activity.js`, CSS, template hooks) are removed. `?part=posted` and `?tab=posts` now land on the list. `votes.js createReopenControl` is kept. |
| `/profile?part=updates` | Draws the Updates part alone on both sizes: title **Updates**, no hub, back arrow to `/`. Rows, new count, cut line and error state are unchanged, and the `fillUpdatesPart` body is untouched. |
| Demo | Shows the same three parts and names. The practice report is no longer a card there. No Demo copy pointed to Posted. |
| Policy | The line now reads "Ended reports stay under Ended in My activity for 7 days." |

Commits:
- b50f0df8 feat(quick-261117-vuf): My activity is one Settings row with Confirmed, Disputed, Ended
- 5589027a test(quick-261117-vuf): pin the Activity row, the three parts and the Updates screen

## Tests

- `node --test web/*.mjs`: 1649 tests, 1649 pass, 0 fail, 0 skipped.
- `go build ./...` passes, and `go vet ./...` passes.
- `go test ./... -p 1 -count=1 -v` ran on the scratch database `p9_vuf` with `env -i`, and the database was dropped afterwards. Results:
  - 22 packages ok, 0 FAIL.
  - 2489 top level PASS, 0 FAIL.
  - 4405 subtest PASS, 0 FAIL, 0 SKIP.
  - 3 top level SKIP. These are the live network and fixture checks `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture`. No database test skipped.
- New pins:
  - Settings: Activity is first, with one My activity row to `/profile` (`TestSettingsRows`).
  - The hub, laptop stack and part screens hold exactly confirmed, disputed and ended, with the one word names, and no posted or updates section on `/profile` (`TestActivityPartsRenderFromTheAddress`, `TestActivityShownParts`).
  - `part=posted` lands on the list.
  - `part=updates` shows only Updates, titled Updates, with the back arrow to `/` (`TestActivityUpdatesPartRenders`, `TestAppBarCurrentItem`).
  - Demo parts (`TestDemoActivityPageParts`).
  - The page has no Reopen hooks (`TestActivityOffersNoReopen`, `TestProfileOffersNoReopen`).

## Live look

The live look ran Playwright WebKit against a scratch server on port 8794. It used the database `p9_vuf_live`, seeded with `cmd/seedprofiles` with made up people only, and the log mailer. It signed in as `voter.a@example.com` and added a few confirms and a dispute. It checked 320 and 390 wide plus 1280, each in light and dark: 215 checks, 0 fail. Afterwards the server was stopped, the database dropped, and the log and binary deleted.

- **Settings:** Activity is the first section. The My activity row is 52px, the same as the other one line chevron rows (Muted reports and the rest). The chevron is 20px at the same right edge, and the label size matches. On the laptop the title sits in the left column.
- **List and parts:** the row opens the list. On the phone the list shows Confirmed (14), Disputed (1) and Ended (1); on the laptop the three parts are stacked. Each part opens with its one word title.
- **Bell:** See all updates opens Updates alone on all six runs, with its arrow `href` set to `/`.
- **Width:** nothing scrolls sideways on any screen.
- **Updates rows:** the account had no update rows, so only the empty state was seen live. The rows are pinned by test.
- **Screenshots:** `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/vuf/shots/`. There are 36 files: settings, list, the three parts and updates, at each size and scheme. Nothing was saved in the repo.

**Back arrow, for the owner to decide:** `app-bar.js` steps back in history when the page was opened from this site. This is unchanged since quick 261117-lq0, and that file holds the bell script, which this plan puts off limits. So:
- A part opened from the list goes Back to the list, and Back again lands on Settings.
- Updates goes Back to whatever page the bell was opened from. That was Nearby in the look; from Settings it would be Settings.

The harness checks were written to expect this history step.

## Deviations

1. `internal/api/handlers/profile_record_test.go` was deleted along with `newProfileRecord` and `profileRecord` (intentional deletion, in commit 5589027a).
2. `CanReopen` and `UndosUsed` were removed from the server's `profileReport`, not only from `activity.js`, because they served only Posted cards.
3. The Profile handler now reads `updates.List` only on `part=updates`, since that is the only screen that draws it.
4. `demoActivityData` no longer reads the practice posts.
5. Tests that followed an account's own posts across sign ins now follow its confirms: the cross session, logout and revocation tests. To make that possible, the test server in `auth_e2e_test.go` (`newAuthE2EServer`) gained the voting service. Tests that read a card's state now read the voter's card of another account's report.
6. Some files are shared with quick 261117-vvb. The edits there are line local: `withheld_e2e_test.go`, `moderator_e2e_test.go`, `legal_e2e_test.go` and `reader_contract_test.go`.
7. `docs/TRUST-MODEL.md` line 265 was not edited. It names "Ended nearby" as a reader, not as on screen copy, and vvb edits that file.

## Dead leftovers, listed and not removed

- **My activity moderation markup:** the note, the Policy pointer and the moderated branch in `activity.js` can no longer render, because the service drops withheld voted reports. That area is quick 261117-vvb's.
- **`phone.css`:** the queue strip selector `:not([data-part="posted"])` is dead.
- **`updates.css`:** `.activity-hub__new` is dead.

## Self-Check: PASSED

Both commits (b50f0df8, 5589027a) are on the branch. Every file named above exists, except the intentionally deleted `profile_record_test.go`.
