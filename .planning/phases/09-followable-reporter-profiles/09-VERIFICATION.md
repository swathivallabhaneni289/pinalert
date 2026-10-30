---
phase: 09-followable-reporter-profiles
verified: 2026-10-29T22:07:11Z
status: human_needed
score: 5/5 success criteria verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Viewing session: profile header, finished rows, trust panel line, UI-SPEC defaults 1 to 20"
    expected: "Header reads in the order picture, username, name, home area, stars with both parts, tag with counts, member since, Followers and Following, Follow with the bell; finished rows show a date only; the trust panel says 'Includes ratings by N people.'"
    why_human: "Visual judgement, layout at phone width and on the laptop"
  - test: "Follow flow by hand across two accounts"
    expected: "Opt in, follow, bell on, post as the reporter, the bell row reads Not confirmed yet then the confirmed sentence, unfollow, block, remove follower, switch the profile off; star box after Yes, mark resolved and Skip"
    why_human: "End to end feel and timing across two browsers"
  - test: "Installed phone, after Phase 3.3 is live"
    expected: "One follow note per post, no username on the lock screen, updated in place when confirmed with no second alert, no sound and no vibration, a tap opens the report"
    why_human: "Needs a real installed phone"
  - test: "h1 swap at phone width"
    expected: "Under 900px the top bar title is the page's only h1 and the username under it is a plain line; from 900px the username is the h1; the Profile not found and You blocked pages show one h1 on the laptop and nothing jumps"
    why_human: "Heading structure at two sizes and the clipped laptop seat need a screen reader or inspector in a real browser"
  - test: "Forced colours in Chromium"
    expected: "With forced colours on (Chromium emulation), the bell, the More chevron, the stars total icon and the five star control are visible in ButtonText or CanvasText"
    why_human: "Only a Chromium run with forced colours shows whether the mask glyphs survive; the contract test pins the CSS, not the paint"
  - test: "Star box status line"
    expected: "After a rating from the box under the resolve message, the box collapses to one status line and leaves 6 seconds later (RATE_HOST_LINE_MS in web/static/js/votes.js). Decide whether 6 seconds is right or the box should vanish at the tap"
    why_human: "Timing feel; changing it is one line"
  - test: "People list after a server restart"
    expected: "A Followers or Following list that was open before a restart starts again from the top when Show more is tapped, with the server's sentence shown once (fix round 3, SK-W02; the cursor is sealed with a key that is new each start)"
    why_human: "Needs a restart of the test copy; decide whether the sentence is acceptable"
---

# Phase 9: Followable reporter profiles Verification Report

**Phase Goal:** People who choose to be public, and have earned the Reliable reporter tag, can be followed, and a follower is told when they post, without exposing anyone who did not opt in.
**Verified:** 2026-10-29T22:07:11Z (main at 79d434a2)
**Status:** human_needed
**Re-verification:** No, initial verification

The code delivers all five success criteria and every automated check is green. The phase cannot close on this report: the owner checks visuals, the phone and one real device by hand, and the phase closes only on the owner's yes. I found no gap and no blocker.

## How the goal was checked

I did not trust the SUMMARY files. I read the service, store, handler, router, template and script code behind each criterion, then ran the gate myself on a scratch database that I created and dropped (p9_verify_a). No server was started, no other database was touched, no code was edited.

| Check | Result |
| ----- | ------ |
| `go build ./...` and `go vet ./...` | clean |
| `go test ./... -p 1 -count=1` on p9_verify_a | exit 0, all 21 packages with tests ok |
| 26 named test families for V1 to V20, run again with `-v` on p9_verify_a | 110 passes, 0 FAIL, 0 SKIP |
| `node --test web/*.mjs` | 1633 tests, 1633 pass |
| Debt markers (TBD, FIXME, XXX, TODO, HACK) in lines added since 6f13da00 outside .planning | none |
| Em or en dashes in added lines under web, internal, cmd, docs, README | none |
| Python or image files added since 6f13da00 | none |

The Linux CI recipe (Docker) and the TZ=UTC run were not repeated here. 09-28-SUMMARY records the TZ=UTC run as green and the real Linux check runs on GitHub at the owner's next push.

## Goal Achievement

### Observable Truths (ROADMAP success criteria, as amended by the owner)

The ROADMAP text was amended after the original wording (nickname became the Phase 7.1 username, SC3 placement and SC4 timing settled by D-06 and D-09, SC5's "remove all followers without seeing who" no longer applies because the lists are public). I verified the amended meaning, which is what 09-CONTEXT and REQUIREMENTS PROF-01 to PROF-03 now say.

| # | Truth | Status | Evidence |
| - | ----- | ------ | -------- |
| 1 | A user can switch on "Let others follow me" (off by default). Only a Reliable reporter can be followed. They appear by a unique, screened username, never the email; the Name is screened and refused if it uses reserved official words | VERIFIED | `accounts.profile_public BOOLEAN NOT NULL DEFAULT false` with a check that the time is set (migration 00020). `PeopleService.SetProfilePublic` needs a username, re-screens a pending one, runs `ScreenPublicName` (reserved words via `NameReserved`, then the OpenAI screen) and fails closed on an unfinished check; `SetProfilePublicOn` only turns on while the stored Name is still the screened one (WR-S04). `PeopleService.Follow` reads the tag through `LoadReporterSignals` and returns `ErrCannotFollow` unless it is `ReporterReliable`. No email field in the profile struct, page, template or JSON. Tests: TestProfileOptIn, TestFollowRoutes, TestProfileNotFoundIsUniform |
| 2 | A profile shows username, name, home area, stars with both parts from 5 raters, the tag with its counts, member since, Followers and Following counts, and the last 30 days of reports (live as on the map, finished coarse with the date only). Readers who voted on the reporter's reports tap 1 to 5 stars, one rating per account, changeable, never their own. Reliable at 3.5 stars | VERIFIED | `ReliableMinScore = 70` in reliability.go, cut in the single `reporterTagForScore`; `StarsText` floors so 69 never reads 3.5. `CombinePoints` is outcome alone below 5 raters, half and half from 5, integers only. `RatingTalliesForReporters` counts one rating per distinct voter cell, excludes removed, cleared, deleted and moderator-barred raters. `RaterMayRate` requires a confirm, dispute, resolve or reopen vote on one of the reporter's reports (an unresolve alone does not count) and refuses self. D-22 holds in `applyRatings` (profile off: ratings can only lower). One `ReporterSignals` caller path: every tag reader (feed, vote response, history, Activity, push dispatcher and worker, profile, follow sweep, rating service) goes through `reporterSignalsWith` and `ratingService` is plugged into all of them in cmd/server/main.go, so the tag reads the same everywhere (TestRatedReporterSameEverywhere). Profile page template draws the header in the D-13 order, two stars parts only from 5 raters, no email. Live rows keep the exact point, finished rows have no id, description, time or link (`finishedProfileRow`, `buildFinishedView` clears `ReporterProfilePath`). Tests: TestCombinePoints, TestStarsText*, TestReliabilityTable, TestRatingRoutes, TestProfileReports* |
| 3 | A user can follow and unfollow, mute or block; the Followers and Following lists are public and anyone without their own opted in profile shows only as a number; reports of people you follow are seen on their profiles with distance | VERIFIED | Routes under the verified gate in router.go: follow, unfollow, remove-follower, block, unblock, mute, unmute, notify, rating, lists, `/u/{username}`, `/me`, visibility, remove-all (all behind People read or action buckets, plus a Visibility bucket). The list read splits named from "N others" and never links a hidden entry (TestPeopleLists). Mute keeps reports visible (TestMuteKeepsReportsVisible). Block ends follows both ways through `endFollowPair`, `FollowAccount` refuses while a block exists, the sweep cleans a race (IN-S01), and the Phase 8 comment queries skip blocked authors via `person_blocks` (comments.sql). Profile reports distance is measured in the browser from Nearby's kept position (person-reports.js), never sent to the server. Settings rows, My profile in the person menu and View my profile on the phone are in the templates |
| 4 | A follower is told when a followed reporter posts, through Phase 3.2's pipeline, with no coordinates or address in the text | VERIFIED in code and test; the real device is a human check | One bell per person (off by default), `refreshFollowedPosts` writes one note per account and report through `MayNotifyAboutReport(UpdateFollowedPosted, ...)` (Live and Provisional only, never Hidden, Withheld or Retracted), drops muted and not followable people, rechecks the Reliable tag including ratings, shares the one insert and the hourly ceiling with the Reliable alert. The push worker rechecks the follow, the bell, standing, block, mute and tag at send time (`followHolds`). `followBody` reads "From someone you follow. {distance} from your home area. Not confirmed yet." and then "Confirmed by people nearby."; the title names the category and area only, no username, no coordinates. Tests: TestFollowBell*, TestFollowPush*, TestBuildPushText, TestNoNoiseWordsInUserCopy, TestPushKindsClosed |
| 5 | Switching the profile off, or the Reliable tag falling away, ends all follows and pending alerts at once; a reporter can remove followers | VERIFIED | `SetProfilePublic(off)` turns the flag off first, then `endFollowsOf` (soft end with reason profile_off, rows kept) which also deletes queued follow pushes for that person. Tag loss: evaluated on touch in `Profile` and after every rating write or removal, by `FollowSweep` (started as a runner in main.go), and at send time in `followHolds`; the computed tag already refuses a follow and a note the moment it falls (D-24). `RemoveFollower` and `RemoveAllFollowers` use the same helpers. Account deletion removes follows, blocks, ratings, mutes and queued follow pushes. Tests: TestFollowEndsOnSwitchOff, TestFollowSweep*, TestDeleteAccountEndsFollowsAndBlocks |

**Score:** 5/5 success criteria verified. Behavior-dependent truths (state transitions and cleanup ordering in 4 and 5) each have a passing named test, so none is left as present-but-unexercised.

### Required Artifacts

| Artifact | Status | Details |
| -------- | ------ | ------- |
| `internal/store/migrations/00020` to `00024` | VERIFIED | profile flag, `account_follows` (soft end, reason check), `person_blocks`, `profile_ratings`, `person_mutes`, follow note columns, follow push |
| `internal/service/stars.go`, `reliability.go`, `reporter.go`, `reportersignals.go` | VERIFIED | 70 bar, pure integer star helpers, one seam |
| `internal/service/ratings.go`, `people.go`, `personprofile.go`, `profilereports.go`, `followsweep.go`, `followend.go`, `updatesfollow.go`, `people_mute.go` | VERIFIED | substantive, wired, covered by tests |
| `internal/api/handlers/people.go`, `ratings.go`, `person_page.go`, `moderation_ratings.go` | VERIFIED | uniform 404, no id on finished rows, moderator check inside the handlers |
| `web/templates/person.html.tmpl`, `people_list_window.html.tmpl`, `settings.html.tmpl`, `account_header.html.tmpl`, `profile.html.tmpl` | VERIFIED | rendered server side, `rate-host` on index and Activity |
| `web/static/js/person.js`, `person-reports.js`, `people-list.js`, `settings-people.js`, `stars.js`, `votes.js`, `trust.js` | VERIFIED | text set with textContent; star box after resolve with Skip |
| `cmd/seedprofiles` | VERIFIED | refuses protected names, non `p9_` names and remote hosts before connecting; test passes |
| `docs/TRUST-MODEL.md`, `docs/MODERATION.md`, `docs/PUSH.md`, Policy text | VERIFIED | doc tests keep constants and test names in step with the code |

### Key Link Verification

| From | To | Status | Details |
| ---- | -- | ------ | ------- |
| `RatingService` | every tag reader | WIRED | `WithReportSignals`, `WithVotingSignals`, `WithDispatchSignals`, `WithAuthSignals`, `WithPeopleSignals`, `WithFollowSweepSignals`, `WithPushWorkerSignals` all given `ratingService` in cmd/server/main.go and router.go |
| `FollowSweep` | server lifecycle | WIRED | first entry in `runners`, runs with or without push |
| Push worker | follow facts at send time | WIRED | `followHolds` before `stillHolds` for the follow kind and its confirmed update |
| Trust block | profile | WIRED | `PublicProfilePathsForViewer` batch read; path set on live views only, cleared for every finished view |
| Resolve answer | star box | WIRED | `rate_prompt` from `VotingService.ratePromptFor`; `offerRateBox` mounts in the card or the `#rate-host` |
| Block | Phase 8 comments | WIRED | `person_blocks` in the comment visibility queries and the comment push recheck |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| PROF-01 | Opt in switch, off by default, Reliable only, username, screened Name | SATISFIED | SC1 |
| PROF-02 | Profile contents, stars with both parts from 5 raters by distinct place, ratings rules | SATISFIED | SC2 |
| PROF-03 | Follow, unfollow, mute, block, public lists, Notify me bell, silent note, ends at once, remove followers | SATISFIED in code; the phone delivery awaits the owner | SC3 to SC5 |

No requirement mapped to Phase 9 is orphaned.

### Anti-Patterns Found

None. No debt markers, no stub returns on a rendering path, no hardcoded empty props at call sites. The three reviewed and accepted items below are known and recorded.

## Findings

No BLOCKER and no gap. Notes for the record:

| Severity | Item | Detail |
| -------- | ---- | ------ |
| WARNING (owner question) | WR-E04 | D-12 says Block hides the person's comments from you, but Block works only from an opted in profile, and the comment screen has no Block entry. For a commenter with a private profile the hiding cannot be switched on. Behaviour is built and tested for opted in people. See owner question 2 |
| WARNING (owner question) | K1 | Moderators see a rater's username on the Ratings screen even while the name check is running. See owner question 1 |
| Info, accepted in the fix report | IN-S02 | The follow cap of 200 can be passed by a few follows from parallel taps from one account; it only bounds work |
| Info, accepted | IN-S03 | The Blocked and Muted counts include a person whose username is still being checked; the list withholds the name |
| Info, accepted | IN-S04 | Deleting an account hard deletes the moderator audit trail of ratings that account gave, which follows the erase-on-deletion rule |
| Info, accepted | IN-S05 | Past the fan out cap of 500 followers of one reporter the same followers are left out; far beyond this project's scale |
| Info | Deploy day | The Policy date and `CurrentTermsVersion` still read 2026-10-29. At the first public deploy the owner moves both together (README step 9), which asks everyone to agree again to the Phase 9 Policy text |
| Info | Bookkeeping | The ROADMAP plan boxes for 09-01 to 09-29 are unticked, REQUIREMENTS shows PROF-01 to PROF-03 as Pending, and 09-VALIDATION.md still says `status: draft` and `nyquist_compliant: false`. These are for whoever closes the phase after the owner says yes; I did not edit them |
| Info | Process | The templates fixer ran `python3 --version` once, against the no Python rule. Nothing was written and no Python is in the repo |

## Human Verification Required (owner check list)

Set up exactly as 09-28-SUMMARY says: phone in the iPhone simulator (safaridriver and simctl screenshots), laptop in Mac Safari only, no iPad. Use a scratch copy and never ports 8080, 8090, 8776, 8778, 8781 or 8797 (8782 is free).

Setup steps:

1. `createdb p9_view`, then `DATABASE_URL=postgres://localhost:5432/p9_view?sslmode=disable go run ./cmd/migrate`, then `go run ./cmd/seedprofiles -db postgres://localhost:5432/p9_view?sslmode=disable`. Do it just before the viewing: the live reports end 3 to 5 hours later.
2. Start the test copy on port 8782 with the scratch server recipe (own database, log mailer, development mode).
3. Sign up on the test copy with a made up address such as `viewer@example.com`, never your real email. Then run the seed again with `-viewer-email viewer@example.com`.
4. To post as a reporter, sign in as `ravi.k@example.com` with the shared password the seed printed.
5. Tear down afterwards: stop the server, `dropdb p9_view`, delete the server log.

### Check 1. The profile and the UI-SPEC defaults (one viewing session)

**Test:** Open ravi.k, anita.r, new.reporter and your own page on phone and laptop. Look at the header order (round picture, username, name, home area, stars with both parts, tag with counts, member since, Followers and Following, Follow with the bell), finished report rows showing the date only, and the "Includes ratings by N people." line in the trust panel.
**Expected:** Everything reads as drawn in 09-UI-SPEC.
**Why human:** Visual judgement.

Confirm or veto each default in 09-UI-SPEC "Defaults to confirm at the viewing session":

1. The counts sentence uses the shipped wording "7 of their last 9 reports were confirmed", not the longer one.
2. On the laptop the header is a 320px column on the left and Recent reports fills the right.
3. The round picture is a grey disc with the first letter of the username, no upload.
4. Stars are plain: one filled star and the number in the text colour; the two parts only from 5 raters; a New reporter shows no stars.
5. The home area shows on an opted in profile and on your own (also the 09-VALIDATION question Q1).
6. Following is a one tap toggle with no confirmation.
7. Mute is in the More menu, and Muted people is its own row in Settings (Q6).
8. Block lands on a "You blocked" page with Unblock.
9. Remove all followers is one button at the end of your own Followers list, with a confirm box.
10. Block, Remove follower and switching the profile off while someone follows you ask first; everything else is undone by the same control.
11. The profile bell and the top bar Updates bell share one icon; the profile bell has no words. Say if you want "Notify me" beside it.
12. The star control is five buttons, saves at once, can be changed once a day, and has "Remove my rating"; it is drawn only when you may rate.
13. The star box after "Yes, mark resolved" sits under the resolve message and its Undo and has one button, Skip.
14. Finished reports on a profile are plain rows with a coarse area, "About N km", not tappable.
15. The lock screen note has no username ("From someone you follow.").
16. Your own page when the profile is off still opens for you, with a line pointing to Settings; everyone else sees "Profile not found".
17. Commenters' names (Phase 8) stay plain text.
18. The new Policy text and the second agreement prompt (Q11).
19. A follow note ignores distance: with the bell on you hear about every post from that person.
20. One note per report: Critical, then Follow, then Changes, then Reliable.

Also from 09-VALIDATION: the 40 point note about five raters in TRUST-MODEL (B4), and stars or the "Score N out of 100." line in popups (Q5).

### Check 2. The follow flow by hand across two accounts

**Test:** iPhone simulator and Mac Safari, two accounts: opt in, follow, bell on, post from the reporter's account, watch the bell row read "Not confirmed yet" and then the confirmed sentence, unfollow, block, remove follower, switch the profile off. Also resolve a report and see the star box, then tap Skip.
**Expected:** Each step happens at once and the follow and note stop when the profile goes off.
**Why human:** End to end feel and timing.

### Check 3. The installed phone (after Phase 3.3 is live)

**Test:** On your installed phone, follow someone with the bell on and have them post.
**Expected:** One follow note per post with no username on the lock screen, updated in place when confirmed with no second alert, no sound and no vibration, and a tap opens the report.
**Why human:** Needs a real device.

### Check 4. Items from the fix reports

1. **The h1 swap at phone width.** Under 900px the top bar title is the page's h1 and the username is a plain line; from 900px the username is the h1; the Profile not found, You blocked and Couldn't load pages show one h1 on the laptop without anything jumping. Look at it in a real browser at both widths.
2. **Forced colours in Chromium.** Turn on forced colours (DevTools, Rendering, Emulate CSS forced-colors) and check the bell, the More chevron, the stars total icon and the five star control stay visible.
3. **The star box status line.** After a rating the box collapses to one status line that stays 6 seconds (`RATE_HOST_LINE_MS` in `web/static/js/votes.js`). Say whether 6 seconds is right. Making the box vanish at the tap instead is a one line change.
4. **The people list cursor after a server restart.** Open a Followers or Following list, restart the test copy, tap Show more. Since fix round 3 (SK-W02) the list starts again from the top and shows the server's sentence once. Say if that reads calmly.
5. **The hosted star box (fix round 3, SK-W01).** After the resolve that closes a report, the star box sits under windows and the phone report card, goes away when another report or window opens, and ends by itself after 30 seconds (`RATE_HOST_MAX_MS`). Check on the phone that it never covers a button.

### Two questions for the owner

**1. K1: moderators and a rater's name while the name check is still running.** On the moderator Ratings screen a moderator sees the username of someone who rated a reporter, even when that username has not yet passed its name check, because a moderator removes a rating by naming the person. Nobody else ever sees a rater's name. Should a moderator see it during the check? **Recommendation: keep it.** Hiding it would mean building some other way for a moderator to remove a rating, and the name is the simplest handle they have.

**Answered 2026-10-30:** the owner said "yes, moderators can see it". Kept as built, no change.

**2. WR-E04: blocking someone who keeps their profile private.** Today you can block a person only from their public profile. Most people keep their profile off, so if a commenter has a private profile you cannot block them to hide their comments, even though blocking is meant to do that. Should that change now? **Recommendation: accept it for now.** A later small change could put Block beside Flag on a comment, which would also work for private commenters. The block, the comment hiding and the follow clean-up already work for anyone who is public.

## Gaps Summary

No gaps. All five success criteria are met in the code and by passing tests that I ran myself on a scratch database. The remaining work is the owner's: the viewing session, the two-account follow flow, the real phone, the four fix-report items and the two questions above, then a yes.

---

_Verified: 2026-10-29T22:07:11Z_
_Verifier: Claude (gsd-verifier)_
