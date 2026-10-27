# Phase 9: plan review, round 1 (2026-10-27)

Four read only reviews ran on the first 27 plans: three plan checkers (haiku, plans 01 to 09, 10 to 18, 19 to 27) and one trust and privacy review (opus) of plans 01, 03, 04, 05, 06, 13, 14, 21, 22, 25, 26. Every item below goes to the planner as revision round 1, with the orchestrator's decision. Decisions that go beyond 09-CONTEXT are recorded there as D-20 to D-24.

## Blockers

| Id | Plans | Finding | Decision and fix |
|----|-------|---------|------------------|
| B1 | 05, 09 | A finished report on a profile can be traced to its exact point: 09-09 puts `reporter_profile_path` on every ReportView, finished Focus and Past rows included (`buildFinishedView` calls `baseView`), report ids are sequential, and the profile row's category, severity and times match the anonymous Past row. | Never set `reporter_profile_path` on a finished view (clear it in `buildFinishedView`), with a test that a Resolved or Expired Focus or Past row has no path. Profile finished rows show the date only, never a time (D-20). |
| B2 | 13, 14 | Ratings skip the independence rule: five accounts in one cell, or the reporter's own second accounts, count fully; five raters lift a score of 39 to 70 or drop 100 to 60, and dropping the tag deletes every follow for good. | Count raters by distinct voter cell of their qualifying vote (COUNT DISTINCT cell for the 5 rater threshold and for the average, one rating per cell, the most recent); follows end softly (`ended_at` and a reason) so a moderator removal of ratings can restore follows lost to them (D-21). |
| B3 | 13 | Tallies count only while the profile is on, so switching it off wipes one star ratings and can restore the tag. | While the profile is off the tapped half can only lower the score: points are `min(outcome, combined)`; test that switching the profile off never raises the tag (D-22). |
| B4 | 27, research 1.5 | The documented "plus or minus 30 point swing" understates it: the tapped half moves the score by (20 x average minus outcome) / 2, up to 40 either way. | TRUST-MODEL says plainly: five eligible raters in five different places can drop any reporter below Reliable and lift any outcome score of 39 or more to it; what the cell rule and soft ending do about it. |
| C1 | 13 | `CombinePoints(86, 4, 4)` is specified as 86 but the formula has no below 5 raters guard (it gives 53). | Guard at the top: fewer than `MinRatersForStars` distinct raters returns the outcome score. Re-check every expected value. |
| C2 | 13 | Eligibility excludes a rater whose only vote is an unresolve; D-02 says "voted on at least one of that reporter's reports". | Kept: an unresolve is only the undo of one's own resolve, so "voted" means a confirm, dispute, resolve or reopen vote (D-23, interpretation recorded). |
| C3 | 06 | Tag loss ends follows "within one sweep interval"; SC5 and D-12 say "at once". | From the moment the tag falls away no follow note is produced or sent (produce time and send time rechecks of the computed tag) and the profile cannot be followed; the stored follows end softly at the next sweep or touch. Reword the truth to that (D-24). |

## Warnings

| Id | Plans | Finding | Fix |
|----|-------|---------|-----|
| W1 | 04, 13, 26 | Follows ended on profile open (09-04) and on a rating or moderator removal (09-13) leave queued follow pushes. | One `endFollowsOf(ctx, followee, reason)` helper (soft end plus queued push delete) used by every path. |
| W2 | 25 | The follow fan out repeats on every evaluation when no push was queued. | Mark the report fanned after one pass. |
| W3 | 22 | The Reliable step skips every report by a notified followee, even when the follow step wrote nothing, so a follower can get no note at all. | Skip in the Reliable step only the report ids the follow step actually wrote. |
| W4 | 13 | A rater whose ratings a moderator removed can rate again. | A rater removed flag that `RaterMayRate` refuses. |
| W5 | 13, 14 | With few raters the tapped part reveals one person's exact stars. | Show the tapped part rounded to half stars; document the limit. |
| W6 | 14 | The trust block can say Reliable reporter without saying part of it comes from ratings. | From 5 raters, the trust panel adds "Includes ratings by N people." (UI-SPEC copy added). |
| W7 | 04 | Unfollow, Unblock and Remove follower answer 404 for unknown and 204 for private or pending names. | 204 for any well formed username. |
| W8 | 14, 26 | `TestRatingsSourceWiredEverywhere` misses `WithPushWorkerSignals`; its file paths and option count are not given. | Paths `../../cmd/server/main.go` and `../api/router.go`, the exact list of option calls, the worker included. |
| W9 | 18 | Uses 09-09's `ReporterAccount()` without depending on it. | `depends_on` gains 09-09. |
| W10 | 10 | Edits Phase 8 code at wave 2 with a hedged acceptance check. | Wave after 09-08, Phase 8 merged as a stated precondition, exact Phase 8 file names taken from its SUMMARY at execution. |
| W11 | 01, 13, 14, 16, 18, 19, 23 | Files that Phases 5, 7.1 or 8 also change lack the re-read note (fixture files in 01, `migrations_test.go`, `ratelimit_gate.go`, `people.go`, the person page files, `swagger_test.go`). | Add "re-read after Phases 5, 7.1 and 8 land, in full" to each. |
| W12 | 15 | "Remove all by {rater}" has no defined confirm box. | Name the `.logout-confirm` box, its title, text and buttons. |
| W13 | 14 | A grep and awk count as acceptance. | A named test per file. |
| W14 | 20, 24 | 09-20 uses 09-19's routes and 09-24 uses 09-23's route in the same wave. | Add the dependencies and re-wave. |
| W15 | 23 | The 50 person cap sentence is not in the UI-SPEC; bell end paths other than unfollow and mute are untested. | Keep the sentence (added to the UI-SPEC with the other planning copy); add tests for Block, Remove follower, profile off and the sweep. |
| W16 | 27 | `git status --porcelain docs/` cannot pass after editing docs; the plan carries docs, Policy, a seed tool and the gate. | Check only the generated swag files with `git diff --stat`; move the seed tool and the final gate to a new 09-28. |
| W17 | 27 | PROF-01 to PROF-03 refresh. | Done at planning by the orchestrator (D-19); 09-27 no longer edits REQUIREMENTS.md. |

Owner answer on push (2026-10-27, via pinalert-60): the iPhone Home Screen install stays and phone notifications stay, so waves 15 and 16 stand.

## Change after round 1 (orchestrator, 2026-10-27)

The planner read D-21's "reporter's own cell" as the report's own cell. That cell is the scene, where genuine witnesses stand, and Phase 3's confirmation count does not exclude it, so the rule would mostly block real raters. It was dropped: raters still count by distinct voter cell with a 5 cell threshold, and follows end softly. A witness at the scene can rate (TestWitnessAtTheSceneCanRate). The remaining risk, one person controlling accounts in five places, is accepted and documented (threat T-09-103).

# Round 2 (2026-10-27, opus, focused)

Every round 1 item CLOSED with a named test; structure clean across 29 plans in 20 waves; each tail wave from 7 to 20 can be cut without breaking an earlier one. Two warnings went to the planner:

| Id | Plans | Finding | Fix |
|----|-------|---------|-----|
| R2-W1 | 22 | A follow note in the bell keeps the username and link after the report finishes (D-20). | A finished report's followed_posted row shows the unavailable line with no username and no link; TestFollowBellFinishedReportDropsName. |
| R2-W2 | 14 | The cross surface test has no private profile leg (D-22). | Add the leg: outcome 86, five one star raters in five cells, profile off, every reader 53 Mixed record, and still 53 after switching it on. |

# Cross check against the Phase 8 plans (2026-10-27, opus)

Phase 8 was planned after Phase 9 (commit 2d2f10b). All ten assumptions A-P8-1 to A-P8-10 hold. Fixes sent to the planner:

| Id | Plans | Finding | Fix |
|----|-------|---------|-----|
| M1 | 10 T2 | The block skip for comment notifications was put in Go, but Phase 8's one insert is the SQL `InsertCommentUpdate` (08-11). | Add the person_blocks NOT EXISTS to `InsertCommentUpdate` next to its self, mute and deleted skips; add comments.sql and its sqlc file to the task files. |
| M2 | 10 T1 | No placeholder exists for a comment hidden by a block. | `gone = 'hidden'`, read "Comment hidden" (UI-SPEC P9-C31, a default for the owner); Phase 8's placeholder zeros apply; the block test sits inside 08-08's "shown" CTE; add the service mapping and comments.js files. |
| M3 | 10 T1 | `reply_to_author` would still show "@B" to someone who blocked B. | NULL it when the viewer blocked the replied to author; test it in TestBlockHidesCommentsFromBlocker. |
| M4 | 21 | List positions and the Down migration not pinned. | Literal nine kind CHECK; UpdateFollowedPosted after UpdateReplyToYours and before UpdateMore; the Down restores 08-11's eight kind text; uq_updates_comment_unread untouched. |
| M5 | 02, 13, 18 | Deletion order ambiguous. | Follows, Blocks, Ratings, PersonMutes immediately before RevokeAccountSessions, after every Phase 5, 7.1 and 8 step; pin the tail of wantDeleteOrder. |
| M6 | 17 | Star box host under default A not stated. | Mount inside the card node after the resolve controls, so it moves with the card into the laptop panel; two new test cases. |
| M7 | 25 | The 08-15 `u.kind IN` list must stay. | Assert it unchanged. |
| M8, M9 | 01, 11 | Wording. | Correct A-P8-9's description; spell out bubble on the laptop under default A, page on the phone. |

# Execution notes added after planning

- **2026-10-27, a 3.3 fix is agreed but not yet built** (03.3-UAT.md gap for test 4): the first post or vote opens a small "Tell you when this report changes?" pop-up (likely in votes.js and report-form.js), and the sign up pop-up gains an unticked tick box for Reliable reporter alerts. Plan 09-17's star box after "Yes, mark resolved" also lives in votes.js: re-read votes.js on top of that fix, and make sure the two never open at once (the star box waits until the changes pop-up is answered or closed). Plan 09-12's Settings rows stay "only for changing your mind later", per the owner principle.
