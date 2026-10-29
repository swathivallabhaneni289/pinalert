---
phase: 08
part: round3
fixed_at: 2026-10-29T20:30:00Z
review_path: .planning/phases/08-report-page-and-comments/08-REVIEW-FIX-SKEPTIC-2.md
base_commit: bdb37d71
iteration: 3
findings_in_scope: 4
fixed: 3
recorded_not_fixed: 1
skipped: 0
status: all_fixed
---

# Phase 8: review fix report, round 3

**Source review:** 08-REVIEW-FIX-SKEPTIC-2.md (SK2-01 to SK2-04), with 08-REVIEW-FIX-round2.md for context.
**Database:** p8_fix_r3 only. It was created, recreated fresh for the final run, and dropped at the end. No migration was added.

**Summary:**
- Fixed: 3 (SK2-01, SK2-03, SK2-04), one commit each.
- Recorded, not fixed: 1 (SK2-02), because the case it describes cannot happen.
- Kept: every Phase 5, 7.1 and Phase 8 behaviour, no markup sink (the new read builds nothing, it reuses placeReplies), no new enumeration oracle (the same replies_of read, same empty answers), replies folded and oldest first, the same code on phone and laptop.

## Fixed

### SK2-01 (warning): a new reply beyond the first 100 was unreachable in an open capped thread

**Commit:** ffa05230
**Files:** `web/static/js/comments.js`, `web/comments_test.mjs`
**Status:** fixed: requires human verification (it changes the merge logic).
**Applied fix:** the skeptic's cheaper count check was not used: in the reported case one reply is gone and one is new, so the counts agree and no count rule can tell. Instead the tail is read again. When a list answer for an open thread is capped and the thread's tail was read before (`coveredTo` is past the answer's last reply), `coverReplies` queues the thread. After the answer is written, `rereadTails` reads `?replies_of=<root>&reply_after=<last reply of the answer>` and passes it through `placeReplies` over the range it covers. A reply gone from the tail leaves the screen (this is the same fix), a new one lands in id order, `total` comes from the answer, and Show more replies stays only when that read itself says more are left. A failed read leaves the thread as the poll wrote it. A folded thread, or one never read past the first 100, is not read again, so there are no extra requests for them.
**Tests (seen failing first, then passing):**
- a gone reply 120 and a new reply 152 with equal counts: the gone one leaves and the new one shows, with the button hidden;
- a reread tail longer than one page keeps the held tail and Show more replies;
- a thread never read past 100 sends no extra request.

### SK2-03 (info): ListReplies answered `open: rep.open` on a finished Rescue report

**Commit:** c3ef6ae0
**Files:** `internal/service/comments.go`, `internal/service/comments_test.go`
**Applied fix:** `ListReplies` returns `CommentList{Comments: []CommentView{}}` (closed) when the report is hidden, the same as `List`.
**Test:** a subtest of `TestCommentFinishedRescueHidden` compares the two reads. Note: it passed before the change too. A finished report is never open today (`CommentsOpen` is false for Resolved and Past), so the split was not reachable. The test pins the one rule, and the code no longer depends on that coincidence.

### SK2-04 (info): TRUST-MODEL said the sentence always names the confirm count

**Commit:** 82ed6ce5
**Files:** `docs/TRUST-MODEL.md`, `web/review08_round2_contract_test.go`
**Applied fix:** the bullet now reads "(for a confirmed report the sentence names how many people nearby confirmed it; the cells, the votes and the scores are never shown)".
**Test:** `TestTrustModelSharingPageFields` (amended, with a dated note) refuses the old parenthetical and needs the new one. It failed first. `TestTrustModelDocMatchesCode` stays green.

## Recorded, not fixed

### SK2-02 (info): the toggle undercounts by one while a poll that left before your own reply is still out

Not reachable, so no code change. A successful post calls `load()`, which ends any list request already out (`stopLoad` through `loadSeq`), and the new list request leaves after the post. So no list answer that left before your reply can arrive after it. The Show more comments page and the new tail reread are the only other reads in flight, and neither answers for a thread you are replying to in that window. A test written for the case passed without any fix, which confirms it. If a later change stops a post from restarting the list, add one to the toggle for each held reply posted after the request left.

## Verification

- `go build ./...` and `go vet ./...`: clean.
- Full run, `go test ./... -p 1 -count=1 -v` on a freshly recreated p8_fix_r3: 20 packages ok, 0 FAIL. The only 3 skips are the known ones: TestGeocodeLive, TestOfficialLive and TestScreeningFixture.
- `node --test web/*.mjs`: 1452 pass, 0 fail (3 new).
- Every commit is path limited and ends with the trailer. The added lines hold no non-ASCII character. There is no Python, no image, and no sound or vibration wording. No server was started.

_Fixer: Claude (gsd-code-fixer), round 3_
