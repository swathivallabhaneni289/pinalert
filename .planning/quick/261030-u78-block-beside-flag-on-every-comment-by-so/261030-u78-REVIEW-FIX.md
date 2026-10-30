---
phase: quick-261030-u78
fixed_at: 2026-10-30T17:27:39Z
review_path: .planning/quick/261030-u78-block-beside-flag-on-every-comment-by-so/261030-u78-REVIEW.md
iteration: 1
findings_in_scope: 10
fixed: 9
skipped: 1
status: partial
---

# Quick 261030-u78: Code Review Fix Report

**Fixed at:** 2026-10-30
**Source review:** 261030-u78-REVIEW.md
**Iteration:** 1
**Scope:** all findings (Critical, Warning, Info), following the orchestrator's triage

**Summary:**
- Findings in scope: 10
- Fixed: 9
- Not fixed: 1 (IN-02, as decided in triage)

## One line per finding

- CR-01: fixed in 74039c86 (requires human verification). Block is now offered only where the commenter's name is shown. The server refuses an author with no username, or with a username whose screen is not clear, with the same byte equal 404. comments.js hides Block on New member comments and the pending box variant is gone. MODERATION.md no longer promises a recheck. The plan's T-u78-11 residual is withdrawn.
- WR-01: fixed in 74039c86 (same change). A comment block can no longer add a row that Settings, Blocked people cannot list. CountBlocked is unchanged: a clear-named person who is blocked and later renamed into pending can still make the count and the list differ, but that predates this task.
- WR-02: fixed in 74039c86 (same change). A comment block cannot reach a person whose name is not shown, so it cannot reveal whether they follow the blocker.
- WR-03: fixed in 386477f0. If the comment left the list or became a placeholder, Cancel, a failure and offline focus the comments heading. A 404 always focuses the heading. writeActions keeps Block hidden on a placeholder.
- WR-04: fixed in 4a4674f6 and 51e87a71 (requires human verification). After a 204 the list reloads from the first page the way Retry does. A new listGen counter, bumped by clearList, drops a Show more, Show more replies or tail read that was already out, and the clear gives Show more back at once. 51e87a71 also drops a reply target aimed at the blocked person, with its mention, because the reset no longer goes through removeRoot. The reset briefly shows the skeleton, which hides the list and the composer for a moment after a block; typed words are kept. This is one for the owner's visual check.
- IN-01: fixed in 41939d94. sendBlock starts with its own busy guard.
- IN-02: not fixed, as decided in triage. Like and Flag have the same uneven refusal costs, and the differences reveal nothing the viewer cannot already see.
- IN-03: fixed in f78da98b. The refusal table ends with a 204 positive control on a seeded live report, which also checks the row. The subtest where a commenter blocked the caller checks the reporter to cat row exists. The malformed id subtest checks that no row was written.
- IN-04: fixed in fb33c3d5. BlockAccount inserts through `SELECT ... FROM accounts WHERE id = blocked AND deleted_at IS NULL` (regenerated with sqlc v1.31.1), so a deletion that has already committed gets no row. Under READ COMMITTED it does not catch a deletion still in flight when the statement starts. That narrower window remains, noted in the query comment, and closing it would mean locking the account row in account deletion.
- IN-05: fixed in eaed29e8. The MODERATION.md Tests line now names TestBlockCommentAuthorStopsCommentNotes and the new TestBlockAccountSkipsADeletedAccount. Escape is left as Cancel, matching the comment Delete box and the profile boxes.

## Notes

- **Swagger:** the route description now names the New member refusal. `make swag` was run, and the @license.name line is still "Unlicensed (all rights reserved)".
- **T-u78-11 is withdrawn:** its premise (a pending name is rechecked on every read) is false, and no block can now be made on a name that is not clear.
- **New tests:**
  - Go: two refusal cases in TestBlockCommentAuthorUniformRefusal (pending name, no username), the same two in TestCommentBlockAuthorUniformNotFound plus the 204 control, and TestBlockAccountSkipsADeletedAccount.
  - Node: no Block on a New member comment, a name that clears brings Block back, four WR-03 focus tests, six WR-04 tests (reload, stale reads, reply target), and the IN-01 double press.
- **SUMMARY.md is untouched, by scope:** it still carries the T-u78-11 paragraph and choice C5 (the box line for a name not shown), and both are now withdrawn.
- **Test runs on the scratch database q_u78_fix,** which was dropped afterwards:
  - `go test ./... -p 1 -count=1 -v`: 6841 PASS, 0 FAIL, 0 panic. Only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skip.
  - `node --test web/*.mjs`: 1677 tests, 1677 pass, 0 fail (after 51e87a71). The Go suite ran before 51e87a71, which touches only comments.js and its node test; `go test ./web/` passed again afterwards.

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
