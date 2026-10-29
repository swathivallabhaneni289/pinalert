---
phase: 08
part: round2
fixed_at: 2026-10-29T19:30:00Z
review_path: .planning/phases/08-report-page-and-comments/08-REVIEW-FIX-SKEPTIC.md
base_commit: b3fc0213
iteration: 2
findings_in_scope: 7
fixed: 4
recorded_not_fixed: 3
skipped: 0
status: all_fixed
---

# Phase 8: review fix report, round 2

**Source review:** 08-REVIEW-FIX-SKEPTIC.md (SK-01 to SK-07), with the server, browser and templates fix reports for context.
**Database:** p8_fix_r2 only. It was created, recreated for the final run, and dropped at the end.

**Summary:**
- In scope: 7. These are the 2 warnings (SK-01, SK-02) and the 5 info items.
- Fixed: 4 (SK-01, SK-02, SK-04, SK-07). Each one was test first: the new or amended test was seen failing, then passing. Each finding has its own commits.
- Recorded, not fixed: 3 (SK-03, SK-05, SK-06), one line each below.
- Kept:
  - every Phase 5, 7.1 and Phase 8 behaviour;
  - the 3.3 names;
  - no markup sink (the new button and error line are built with `element` and `Pinalert.setText`);
  - no new enumeration oracle;
  - few notifications, one per report per device (no push code changed).
- No migration was added. 00019 is still the last one. The Policy date and `service.CurrentTermsVersion` were not moved.

## Fixed

### SK-01 (warning): a thread with more than 100 replies kept the newest out of reach, and a new reply vanished at the next poll

**Commits:** 424bbfd3 (server), c12fb7d8 (browser)
**Status:** fixed: requires human verification. This changes how the paging and merge logic works.

**Design:** forward reply paging, oldest first, with the replies still folded. The same code runs on the phone and the laptop.

**Server (424bbfd3):**
- `CommentsForReport` gains `reply_after`. The replies CTE lists only replies with a larger id. The page list passes 0, so its answer is unchanged. `shown_replies` is not affected and stays the whole thread's count.
- `CommentService.ListReplies(report, viewer, rootID, after)` reuses List's report check and its one visibility rule. It reads with `after_id = root - 1`, `root_cap 1` and `reply_cap = cap + 1`. The answer is the root alone, carrying up to 100 replies after the cursor. `More` is set from the extra row, and `NextAfter` is the last reply id. These all answer the same empty list: a reply id, another report's comment, an unknown id, and a removed root with nothing shown. So the answer tells nothing that List does not.
- `GET /api/reports/{id}/comments` takes `replies_of` and `reply_after`. The answer uses the same response shape as before. A bad value gets 400 with its field. These also get 400: `reply_after` without `replies_of`, and `after` together with `replies_of`.
- The swagger docs are regenerated: two params and one description paragraph.
- The shared List code is now split out into `commentThreadsFromRows` and `commentCount`, so both reads use it.

**Browser (c12fb7d8):**
- The toggle counts `max(thread total, replies read)`. The total comes from `reply_count`. Your own new reply adds one until the next answer counts it, and your own delete takes one off.
- `placeReplies` takes the range the answer covers, the same way `mergeRoots` does. A list answer whose `reply_count` is larger than the replies it sent covers only up to its last reply. Replies after that range stay. A reply you posted after the request left also stays, through a `postStamp` that `answerStamp` provides. Replies are always placed in id order.
- A real button, "Show more replies", sits under the replies of an open thread. It shows while replies after the covered range are left to read. It reads `?replies_of=<root>&reply_after=<last covered>` and merges the page through the same range rule. A failed read shows "Couldn't load more replies. Try again." and keeps the button, so a second press asks again. It reuses the Show more comments rule in comments.css: 44px, bordered, `--color-text`. So no phone muted override is needed, and it is added to the 44px touch list.

**Tests:**
- `TestCommentRepliesPaged` (service): cap + 2 replies. Page 1 holds 100 with More set. Page 2 holds the last 2, oldest first, with More false. A reply id, a foreign root, an unknown id and a removed root each answer an empty list.
- `TestCommentReplyPagesQuery` (handler e2e): the whole thread, the cursor, the empty answers and every 400.
- Four node tests in comments_test.mjs:
  - with 101 replies and 100 sent, the toggle says "View 101 replies";
  - Show more replies sends the right path and lands reply 101 last;
  - a later poll carrying 100 keeps it;
  - your own reply 103 survives a capped poll, and a page read afterwards places reply 102 before it;
  - a failed read says so and can be pressed again;
  - a thread sent whole still drops a reply that went.

**Known limit:** replies after the first 100 of a thread are not read again by the poll. A delete, a removal or a like count there stays stale until the panel is reopened. This is the same limit as SK-05 for middle pages of top level comments.

### SK-02 (warning): TRUST-MODEL Sharing bullet 3 said less than the page shows

**Commit:** 050d5cef
**Files:** `docs/TRUST-MODEL.md`, `web/review08_round2_contract_test.go` (new)
**Applied fix:** bullet 3 now lists the What is happening pick for a Critical report. The trust facts clause now reads: "a comment, or the trust details beyond the one sentence (the sentence names how many people nearby confirmed it; the cells, the votes and the scores are never shown)". The citation of `TestSharePageShowsOnlyAllowedFields` is kept.
**Test:** `TestTrustModelSharingPageFields` refuses the old "a comment or the trust facts" words and needs the new ones. `TestTrustModelDocMatchesCode` stays green.

### SK-04 (info): the access log redacted only a literal /r/ prefix

**Commit:** 4fc5cb6a
**Files:** `internal/api/router.go`, `internal/api/share_routes_test.go`
**Applied fix:** `loggedPath` now takes the URL. It writes `/r/{token}` when the escaped path starts with `/r/`, or when `path.Clean` of the decoded path does. Every other path is logged as before.
**Test:** `TestShareTokenNeverInAccessLog` (amended, with a dated note) adds `//r/<token>`, `/%72/<token>`, `/./r/<token>` and `/%2Fr/<token>`. Before the fix, the token was in the log for all four.

### SK-07 (info): the Policy said every comment keeps a check result

**Commit:** c06fdc32
**Files:** `web/templates/legal_body.html.tmpl`, `web/review08_templates_fix_contract_test.go`
**Applied fix:** the line now reads "When the check finishes on a comment, its result, its scores and which model gave them are kept with the comment." This matches `comments.go`, which writes the result only when the check gave a model or scores.
**Test:** `TestPolicyCommentCheckKeptForEvery` (amended, with a dated note) refuses "For every comment the check's result" and needs the new sentence.

## Recorded, not fixed

### SK-03 (info): the send time comment check can skip a correct push in a narrow commit time race
Not small. A sound fix needs the comment id on the update row, or a new compare against the claimed bell row, which changes a push query and its worker tests. The error leans toward silence, which the owner's few notifications rule allows.

### SK-05 (info): middle pages of top level comments are not refreshed after Show more is used twice
Not small. Refreshing them needs a list of page cursors read one per tick, which changes the paging logic again. It is recorded as a limit, and it now applies to replies after the first 100 of a thread as well.

### SK-06 (info): TestPhoneSheetHasNoMutedText matches overrides by last compound selector only
Not small. Comparing specificity, or covering media blocks, means rewriting the test's matcher. Today's overrides use the full chain, so the page itself is right.

## Verification

- `go build ./...` and `go vet ./...`: clean.
- Full run, `go test ./... -p 1 -count=1 -v` on a freshly recreated p8_fix_r2. Results: 20 packages ok, 0 FAIL, 2185 top level tests passed (6223 with subtests). The only 3 skips are the known ones: TestGeocodeLive, TestOfficialLive and TestScreeningFixture. p8_fix_r2 was dropped afterwards.
- `node --test web/*.mjs`: 1449 pass, 0 fail.
- Every commit is path limited and ends with the trailer. The added lines hold no non-ASCII character. There is no Python, no image, and no sound or vibration wording. No server was started.

_Fixer: Claude (gsd-code-fixer), round 2_
