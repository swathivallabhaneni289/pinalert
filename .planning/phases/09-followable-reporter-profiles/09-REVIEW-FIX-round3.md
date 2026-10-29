---
phase: 09
part: round3
fixed_at: 2026-10-30
review_path: .planning/phases/09-followable-reporter-profiles/09-SKEPTIC-1.md
iteration: 3
base_commit: cb751de7
findings_in_scope: 8
fixed: 7
skipped: 0
accepted: 1
status: all_fixed
---

# Phase 9 code review fixes, round 3

**Source:** 09-SKEPTIC-1.md (SK-W01 to SK-W03, SK-I01 to SK-I05).
**Database used:** p9_fix_round3_a (created for this run, dropped at the end).
**Checks at the last code commit (2d400025):** `go test ./... -p 1 -count=1 -v` all 21 packages ok, 0 FAIL, 2421 top level passes (2409 before), only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped. `node --test web/*.mjs`: 1646 tests, 1646 pass (1633 before).

## Fixed

### SK-W03: the people list cursor is sealed and bound to its list

**Commit:** 621eb070
**Files:** `internal/service/people.go`, `internal/service/people_cursor_test.go` (amended, dated note), `internal/service/people_cursor_seal_test.go` (new), `internal/service/people_cursor_list_test.go` (new), `web/round3_docs_contract_test.go` (new), `docs/TRUST-MODEL.md`

- The cursor is now AES-256-GCM (standard library only) under the same per process random key, with a fresh random nonce each time, base64url of nonce plus sealed bytes. The follow time and the username cannot be read from it. The key is used only inside the cipher and never logged.
- The sealed payload holds the direction (`f` Followers, `g` Following), the list owner's id, the time and the username. `decodePeopleCursor` takes the owner and direction and refuses a cursor from another owner or the other direction with the same calm 400 "That page could not be read." Only Followers and Following use a cursor (Blocked and Muted are not paged).
- The old HMAC tag code is gone. `TestPeopleCursorIsSigned` was amended with a dated note (new signatures, and the "changed time" case now flips every byte of the sealed value).
- Failed first: `TestPeopleCursorHidesTheFollowTime` showed the old cursor decoding to `1790843400000123:asha_k`. New tests: cursor hides the time, random nonce, bound to its list (unit), and a database test that a Followers cursor is refused on another person's Followers and on the same person's Following.

### SK-W02: a refused cursor starts the list again

**Commit:** d86295f4
**Files:** `web/static/js/people-list.js`, `web/people_list_test.mjs`, `docs/TRUST-MODEL.md`, `web/round3_docs_contract_test.go`

- When Show more is answered with a 400, the window now reads the list again from the top (`load('')`), draws it fresh (not appended) and shows the server's sentence once in the window's message place. Show more then carries the new cursor. If the read from the top also fails, the list shows its error state with Try again (which also starts from the top), so nothing loops.
- TRUST-MODEL.md (Public lists paragraph) now says exactly that. MODERATION.md does not mention the cursor.
- Two node tests failed first (refused cursor restarts; no loop when the restart also fails).

### SK-I03: the /moderate page checks the moderator marker

**Commit:** 50d0c3ca
**Files:** `internal/api/handlers/moderation.go`, `internal/api/handlers/modguard_test.go`, `internal/api/handlers/moderator_e2e_test.go` (helper amended with a dated note)

- `ModeratePage` answers the plain not found when `account.IsModerator` is false, judged before the method, the same as `modGuard`. The old page tests now carry the marker through their helper (`modRenderPage`); the no marker case is pinned for GET, POST and DELETE, with no `Allow` header and no email in the answer.

### SK-I02: the Visibility bucket doc says what the code does

**Commit:** 2dd5d5a1
**Files:** `docs/MODERATION.md`, `internal/api/handlers/visibility_already_on_e2e_test.go` (new), `web/round3_docs_contract_test.go`

- Making an already on request free would need a stored state read before the bucket (or a service change that charges from inside the screener path), which is not cheap. So the doc was corrected: the bucket takes a token for every request that asks for on, including one that finds the profile already on; turning it off is never limited.
- An end to end test on the real router pins it (burst 2: on, on again while already on, then 429), and a doc pin checks the wording and that the old sentence is gone.

### SK-I01: the lost Name race comments

**Commit:** 1bdc0b8d
**Files:** `internal/service/profile.go`, `internal/store/queries/accounts.sql`, `internal/store/sqlc/accounts.sql.go`, `internal/service/profile_race_comment_test.go` (new)

- Comments only. A Name that loses the race is not saved and the person is told to try again, but the username written first and the home area of the same statement have already landed. The query comment, its generated copy (`sqlc diff` is empty) and the G1 and `UpdateProfile` comments now say so.
- No code change: the suggested `WHERE` change would skip the home area but not the username, which is a separate earlier write, so it would still not make the sentence "changes nothing" true. A text pin keeps the comments honest.

### SK-I04: no hosted star box after the toast Undo

**Commit:** 207906a5
**Files:** `web/static/js/votes.js`, `web/votes_stars_test.mjs`

- Each offer takes a number per report; both Undo paths (the card's and the toast's) move it on through `takeRateBoxBack`. A box that waited for the first post or vote pop-up mounts only if its number still stands. A later resolve of the same report takes a new number and is offered a box again.
- Two node tests failed first (toast Undo with a pending pop-up, card Undo with a pending pop-up plus the later resolve).

### SK-W01: the hosted star box never covers controls and has an end

**Commit:** 2d400025
**Files:** `web/static/css/stars.css`, `web/static/js/votes.js`, `web/static/js/map.js`, `web/votes_stars_test.mjs`, `web/rate_host_contract_test.go`

- Stacking: `#rate-host` is z-index 1070, above the page menus (1060) and under the phone report card (1080), every `.app-window` (1100) and the message (1200). The old pin of 1190 in `TestRateHostStyle` was amended with a dated note. A new test reads the numbers from the shipped stylesheets and keeps the order.
- Ends of a hosted box (a box inside an open card is unchanged):
  - another report opens: `map.js` `onPopupOpen` calls `PinalertVotes.dismissHostedRateBoxes()` before anything else (so the official card path too);
  - a window or dialog opens: a MutationObserver on `hidden` and `open` (subtree) that exists only while a hosted box does, and drops the box when an `.app-window` or `dialog` becomes open (a window closing changes nothing). A browser without MutationObserver relies on the stacking alone;
  - its own end: 30 seconds (`RATE_HOST_MAX_MS`), given once more if focus is inside the box. After a rating the existing 6 second line takes over, and the box's timers are all stopped whenever the box leaves by any road (`rateCleanups`). I chose a fixed time over tying it to the message, because the message's Undo time is 8 seconds, too short to rate in.
- Same on phone and laptop, not a live region, never takes focus on mount.
- Eight node tests and two Go contract tests failed first.

## Accepted, with the reason

### SK-I05: the K3 skip can hold back the confirmed update of a Critical alert

Accepted, no change. It needs the Critical first row to be pruned (`PushOutboxRetention`, 7 days) while the report is still live, which is practically unreachable. The skeptic confirmed the Critical first alert is never held back by any new recheck, and the old "falls back to Critical" path is gone on purpose (triage target K3).

## Notes

- Commit trailers use the exact text the rules file gave (`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`), as asked.
- Nothing needed a file outside the ones listed. No server was started, no port bound, no Python used, no image saved.
- Not checked here (as in the skeptic's own list): the rendered placement of `#rate-host` in a real phone simulator. The stacking and removal rules are covered by tests that read the shipped CSS and run the real scripts in a fake page.

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer), part round3_
_Iteration: 3_
