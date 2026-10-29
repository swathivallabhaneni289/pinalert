---
phase: 09
part: server
fixed_at: 2026-10-30
review_path: .planning/phases/09-followable-reporter-profiles/09-REVIEW.md
iteration: 1
base_commit: c2014ea9
findings_in_scope: 24
fixed: 20
skipped: 4
status: partial
---

# Phase 9 code review fixes, part server

**Source review:** 09-REVIEW.md (triage), 09-REVIEW-service.md, 09-REVIEW-edge.md, and the K items of 09-REVIEW-browser.md and 09-REVIEW-templates.md.
**Database used:** p9_fix_server_a (created for this run, dropped at the end).
**No migration was needed.** 00025 was not written.

Every fix has a test. Each of these was seen failing on the code before the fix and passing after: WR-S01, WR-S02, WR-S03, K3, K6, WR-S04, WR-S05, WR-E01, WR-E02, WR-E03, IN-E04, IN-E06 (account deletion), K5, IN-T01, IN-S01 with IN-E01, and IN-E02. These are pins of text or of behaviour that already held, so they pass before and after by design: IN-E05 (a gofmt pin), IN-E07 (the uniform 404 table), K4 and the IN-E06 comment and doc wording (a source text pin). Three existing tests were amended with dated notes because the fix changes the behaviour they pinned: the pruned first row cases of the push worker tests (K3), the Rescue precision tests (IN-T01), and the account deletion step order list (IN-E06).

## Fixed

| Finding | Commit | What changed |
|---------|--------|--------------|
| WR-S01 | 6d76e3c9 | The moderator removal restore window starts at the removed rating's `created_at`, not its last change. `RemoveRating` and `RemoveRatingsByRater` now return `created_at`. Test changes the stars after the follows ended, then removes. |
| WR-S02 (with IN-E03) | 6c9776aa | A retry of `ModeratorRemove` that finds the rating already removed re-evaluates and restores. `ModeratorRemoveRater` re-evaluates every reporter of the rater's removed ratings (new query `RemovedRatingsByRater`) and joins errors instead of stopping at the first. |
| WR-S05 | 85a4f6ec | `RaterMayRate` is false for a pair whose rating a moderator removed, so the profile and the resolve rate prompt no longer offer a box every tap of which is refused. |
| WR-S04 | 9e5271ec | New query `SetProfilePublicOn` turns the profile on only while the stored Name is still the Name just screened (0 rows gives the try again sentence). `UpdateAccountProfile` takes `name_screened`: a Name that was not screened lands only while the profile is still off, judged inside the statement; the service then reads back and refuses with the try again sentence if it did not land. Both orders of the race are closed. |
| WR-S03, K3, K6 (one change) | 84f6b124 | The send time recheck in `sendRow`: a Reliable reporter note and its confirmed update are skipped as not_eligible once the reader muted the reporter or either blocked the other; a confirmed update whose first row is gone is skipped as not_eligible, never re-based (`confirmedBaseKind` now returns a known flag); `CommentPushStillHolds` is false once either side blocked the commenter. PUSH.md and TRUST-MODEL.md lines updated. One commit for the three because they are one change in one function. |
| WR-E01 | 8acc43d4, 2018d94d | `GET /u/{username}` takes a token from the same per account PeopleRead bucket (`PeopleReadPageMiddleware` in the rate gate, wired in router.go). A refusal is a 429 HTML page with Retry-After, no-store and noindex, built in the rate gate with the public page's stylesheets, never JSON. The second commit replaced a middle dot in the page title with plain ASCII. |
| WR-E02 | 58ee8cf0 | New `Visibility` bucket (burst 6, one every 20 seconds, per account, the Comment numbers) in handlers.RateLimits, api.RateLimitsDefault, the resolver and cmd/server/main.go. `UpdateVisibility` takes a token only for an off to on change, so turning the profile off is never refused by it. 429 field `rate_visibility`. |
| WR-E03 | dfb3d561 | `account.WithModerator` and `account.IsModerator`; `requireModerator` sets the marker; `modGuard` answers the plain 404 without it, before the method check. Covers the Phase 4 and 8 moderator handlers too. |
| K4 and IN-E06 | cff7aef6 | The FollowFanoutCap comment now tells the truth (also notes IN-S05). Account deletion calls `DeleteQueuedFollowPushesForFollowee` before `DeleteAccountFollows`. The `pushkinds.go` comment names follow. PUSH.md lists deleting the account. |
| K5 | f3a30d1b | A follow row's accessible name no longer repeats "Flood, Critical." |
| IN-E05 | ef3155ce | `comments.go` is gofmt clean; a test keeps the handlers sources gofmt clean. |
| IN-E04 | d95c1756 | `decodeOneValue` refuses anything after the one JSON value on the notify, rating, visibility and moderator rating removal bodies. The Content-Type check was not added: the browser script is another fixer's and a missing header there would turn a working call into a 415. |
| IN-E07 | b75cc50b | Test: POST block, mute, notify and rating answer byte equal 404 for unknown, private, pending, deleted and blocking names. |
| IN-T01 | 3b273622 | `CoarsenFinishedForProfile` has no Rescue branch: every finished report on a profile is the precision 5 cell. TRUST-MODEL.md:685 reworded. |
| IN-S01 and IN-E01 | 7a2cea15 | New query `BlockedActiveFollowPairs`; the follow sweep ends every active follow beside a block, either direction, with reason blocked, through `endFollowPair`. The "safe in either order" comment is corrected. |
| IN-E02 | 58d9657c | The people list cursor carries an HMAC (random per process key), so only a server issued cursor is accepted. A cursor from before a restart reads "That page could not be read". |

## Accepted or not fixed, with the reason

- **IN-S02** (the follow cap is read then written): accepted. Parallel taps from one account can end a few follows above 200. It harms nobody, the cap only bounds the work of the Following list and the sweep, and a conditional insert would add a count subquery to every follow. `SetNotify` documents the same pattern.
- **IN-S03** (Blocked and Muted counts include a person whose username is still pending): accepted. The number is how many people you blocked or muted, which is true; the list withholds a name that has not cleared its check. Hiding them from the count would make the number lie about a block that is in force.
- **IN-S04** (account deletion hard deletes the moderator audit trail of ratings the account gave): accepted. Erasing the person's own rows on deletion is the rule of research G6, and `accounts.rater_removed_at` survives on the scrubbed row. Keeping the removed rows would keep a link from a deleted person to other people. Owner decision if the audit trail should win.
- **IN-S05** (the fan out leaves out the same followers past the cap): accepted at this scale, 500 followers of one reporter is far beyond a personal project. The comment next to `FollowFanoutCap` now says so (commit cff7aef6).
- **K1** (a moderator sees a rater's pending username): owner question, left as it is (moderators only, they act by name).
- **WR-E04** (Block works only from an opted in profile): owner question, left as it is. The triage recommends accepting it for now.
- **K2** is the browser fixer's. **IN-T02 to IN-T06 and WR-T01 to WR-T04** are the templates fixer's.

## Needs a file this fixer does not own

- `docs/MODERATION.md` (templates fixer) has a rate limit table (line 343 area). It should gain two rows: the Visibility bucket (account, burst 6, one per 20 seconds, `visibilityBurst`, `visibilityEvery`, only for turning the profile on) and a note that the profile page `/u/{username}` shares the People read bucket and answers a 429 page.

## Checks

- Full `go test ./... -p 1 -count=1 -v` on p9_fix_server_a at the last code commit: every package ok, 0 FAIL, 2397 top level tests passed (6632 passes with subtests), 3 skips (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, the allowed three).
- `node --test web/*.mjs`: 1604 tests, 1604 pass, 0 fail.
- `make swag` was not run: no swagger annotation changed. The `UpdateVisibility` handler's 429 was already documented.
- A late commit (d84dcb93) only reworded one TRUST-MODEL line for WR-S01 and WR-S02 after the full run; the doc test that checks cited test names passed.

## For the other fixers

- **Browser fixer:** `PATCH /api/me/visibility` can now answer 429 with the field `rate_visibility` (the same body shape and Retry-After as `rate_people_action`) when the profile is turned on too often. web/ should show that message the way it shows the other people 429s.
- **Templates fixer:** docs/MODERATION.md needs, in its rate limit table (line 343 area), a row for the Visibility bucket (account, burst 6, one per 20 seconds, `visibilityBurst`, `visibilityEvery`, only when turning the profile on) and a note that `/u/{username}` shares the People read bucket and answers a 429 page. Its line 310 and 312 ("since the earliest removed rating was written") should say "first written" to match WR-S01. Its line about finished Rescue reports on a profile, if any, should say precision 5 (IN-T01).

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer), part server_
_Iteration: 1_
