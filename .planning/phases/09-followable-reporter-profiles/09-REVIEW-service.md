---
phase: 09
part: service
letter: S
reviewed_commit: e689824a
base_commit: 6f13da00
depth: deep
reviewed: 2026-10-30
files_reviewed: 35
files_reviewed_list:
  - internal/service/accountdelete.go
  - internal/service/auth.go
  - internal/service/coarsen.go
  - internal/service/comments.go
  - internal/service/feed.go
  - internal/service/followedreports.go
  - internal/service/followend.go
  - internal/service/followsweep.go
  - internal/service/historyread.go
  - internal/service/people.go
  - internal/service/people_mute.go
  - internal/service/personprofile.go
  - internal/service/profile.go
  - internal/service/profilecopy.go
  - internal/service/profilereports.go
  - internal/service/publicname.go
  - internal/service/pushcopy.go
  - internal/service/pushdispatch.go
  - internal/service/pushkinds.go
  - internal/service/pushworker.go
  - internal/service/ratecopy.go
  - internal/service/ratings.go
  - internal/service/reliability.go
  - internal/service/report.go
  - internal/service/reporter.go
  - internal/service/reportersignals.go
  - internal/service/stars.go
  - internal/service/trust.go
  - internal/service/trustcopy.go
  - internal/service/trustfacts.go
  - internal/service/updates.go
  - internal/service/updatesalerts.go
  - internal/service/updatescopy.go
  - internal/service/updatesfollow.go
  - internal/service/updatesservice.go
status: issues_found
findings:
  critical: 0
  warning: 5
  info: 5
  total: 10
---

# Phase 9 Code Review, part S (service)

**Reviewed:** 2026-10-30, range 6f13da00..e689824a (the three 261029-rtx commits ignored)
**Depth:** deep (each service file plus the SQL it calls in internal/store/queries and the Phase 9 migrations)
**Status:** issues_found

## Summary

The core rating arithmetic is correct. I traced `CombinePoints`, `TappedHalfStars`,
`StarsText`, `applyRatings` and `RatingTalliesForReporters` by hand:

- The half up rounding is exact in integers.
- The five rater minimum is applied after cell counting.
- A New reporter (nil score) never takes ratings.
- A private profile only ever takes the lower of the outcome score and the combined score.
- A repeat tap with the same stars keeps `updated_at`, so it cannot take another rater's cell.
- The 24 hour gap is measured from the last real change.

Every production tag reader goes through `reporterSignalsWith` or `LoadReporterSignals` with
the one `RatingService`. I checked the feed, the vote answer, Activity, Focus, Past, the
profile, Follow, the sweep, the dispatcher and the worker, and both main.go and router.go.

The uniform not found holds on Profile, Follow, Block, Mute, SetNotify, Rate and the lists.
The undo actions (Unfollow, Unblock, Unmute, Clear, Remove follower) answer the same for
unknown and existing names. Every follow end goes through `endFollowPair` or `endFollowsOf`.
The one exception is the documented hard delete on account deletion, where the worker's
recheck covers the queued notes.

The defects found are at the edges:

- The moderator restore window can be defeated by one rating change.
- A failed re-evaluation after a moderator removal is lost for good.
- A person mute does not stop a queued Reliable based push.
- A race lets an unscreened Name go public.
- The profile can offer a star box that always refuses.

No Critical finding.

## Warnings

### WR-S01: Moderator restore window is anchored on the rating's last change, so one change defeats the D-21 restore

**File:** `internal/store/queries/ratings.sql:143-166` (RemoveRating, RemoveRatingsByRater), used at `internal/service/ratings.go:431-441` and `:464-467`

**Issue:** `reevaluateAfterRemoval` restores the tag_lost follows that ended at or after
`since`, and `since` is the removed rating's `updated_at`. `UpsertRating` moves `updated_at`
on every real change, and so does a Clear followed by a new rating.

Scenario:

1. At T1 a bad rater gives 1 star. This is the fifth counted cell, the tag drops, and
   `reevaluate` ends every follow with `ended_at = T1`.
2. 24 hours later the rater changes to 2 stars, so `updated_at = T2`.
3. A moderator removes the rating. `RemoveRating` returns T2, and `RestoreTagLostFollows`
   requires `ended_at >= T2`.
4. The follows that ended at T1 are never restored, even though the tag is Reliable again.

An abuser who knows about removals can make the remedy that D-21 exists for a no op by
changing the rating once. `TestModeratorRemovalRestoresFollows` rates only once, so the tests
do not cover this.

**Fix:** anchor the window on the first time the rating existed. `created_at` is never
changed by the conflict update. The tag is re-read and must be Reliable before anything is
restored, so a wider window is safe.

```sql
-- RemoveRating
RETURNING created_at;
-- RemoveRatingsByRater
RETURNING reporter_id, created_at;
```

Then pass `r.CreatedAt` in `ModeratorRemoveRater` and add a test that changes the stars
between the rating and the removal.

### WR-S02: A failed re-evaluation after a moderator removal is never retried, so the restore is lost for good

**File:** `internal/service/ratings.go:431-441`, `:456-468`

**Issue:** Each step commits on its own, outside any transaction.

- **`ModeratorRemove`.** If `reevaluateAfterRemoval` fails after `RemoveRating` committed (a
  database error in `LoadReporterSignals` or `RestoreTagLostFollows`), the moderator sees an
  error and tries again. The retry gets `pgx.ErrNoRows` from `RemoveRating` and returns nil
  at line 435 without re-evaluating, so the follows the rating cost never come back.
- **`ModeratorRemoveRater`.** The loop returns at the first failing reporter. A retry finds
  the rater already marked and `RemoveRatingsByRater` returns no rows, so the remaining
  reporters are never re-evaluated. No other path ever restores tag_lost follows, so the
  loss is permanent.

**Fix:** smallest change:

- Re-evaluate even when there was nothing left to remove. On `ErrNoRows`, read the rating
  row and, if it is removed, call `reevaluateAfterRemoval(ctx, reporterID, row.CreatedAt, now)`.
- In `ModeratorRemoveRater`, keep going after an error and return the joined errors.
- Better: run the removal and the restore in one transaction through the existing `AccountTx`
  seam.

### WR-S03: A person mute does not stop a queued Reliable reporter push or its confirmed update

**File:** `internal/service/pushworker.go:498-508` (step 4) and `:672-690` (stillHolds); `internal/service/people_mute.go:66-71`; `internal/store/queries/push.sql` EnqueueConfirmedUpdates and PushAccountHome

**Issue:** The send time recheck of a person mute exists only in `followHolds`.
`PushAccountHome.muted` reads `report_mutes` only, and `stillHolds` for `PushKindReliable`
checks the predicate alone. `Mute` deletes queued `follow` rows only.

Two ways a note still reaches a person after they mute someone:

- **A queued push.** A Reliable reporter push queued before the mute still goes out. A row in
  backoff can wait up to `PushBackoffCap` (5 minutes).
- **A confirmed update.** Hours later, when the report is confirmed, `EnqueueConfirmedUpdates`
  queues a `confirmed_update` with base reliable for every sent reliable row whatever
  `person_mutes` says. The worker sends it. On iPhone this is a fresh note, not a replacement.

This breaks D-12 ("Mute stops every note from that person") and the people_mute.go contract
("notes about that person stop"). K6 has the same shape for blocks on comment pushes.

**Fix:** in `sendRow`, for a row whose base is `reliable` (`run.base == PushKindReliable`,
which covers its confirmed update), read
`PersonMuted(MuterID: claim.AccountID, MutedID: *facts.ReporterAccountID)` and skip on true.
Optionally also delete queued reliable rows about the person in `Mute`.

### WR-S04: Racing the profile switch against a Name edit publishes an unscreened Name

**File:** `internal/service/people.go:229-283` (SetProfilePublic on) and `internal/service/profile.go:508-512, 552-578` (UpdateProfile, screenNameEdit)

**Issue:** The two calls run in this order:

- `SetProfilePublic(on)` reads `DisplayName`, makes a slow network screen call
  (`ScreenPublicName`, up to the inline timeout), then writes `profile_public = true` with no
  condition on the Name.
- `UpdateProfile` reads `profile_public` in `screenNameEdit`, skips the screen while it is
  false, then writes the new Name.

An account owner sends "Let others follow me" and, while its screen call runs, a Name edit
such as "NDMA Control Room". The edit sees the profile still off, skips the reserved word and
abuse check, and writes the Name. The switch then turns on with the old screened Name, and
the public profile shows the unscreened one.

Two parallel requests are enough, so the account owner can do this on purpose to impersonate
an agency. That is what `NameReserved` exists to stop. The username path is not affected,
because each username write sets its own screen state, and `SetProfilePublic` re-reads after
`MarkUsernameScreenClear`.

**Fix:** make the switch on write conditional on the screened Name, and refuse when it
changed:

```sql
-- SetProfilePublic, on branch
... WHERE id = $id AND deleted_at IS NULL
      AND (NOT sqlc.arg(is_public)::boolean OR display_name = sqlc.narg(screened_name)::text)
```

In Go, treat 0 rows on the on branch as `PublicNameUnfinishedMessage`, so the user tries
again. An alternative is to re-read after the write and switch off if `DisplayName` differs
from the screened value.

### WR-S05: The profile and the resolve star box offer rating to a rater whose rating a moderator removed

**File:** `internal/store/queries/ratings.sql:16-30` (RaterMayRate), `internal/service/ratings.go:137-177`, `internal/service/trust.go:408-443`, `internal/service/personprofile.go:255-266`

**Issue:** `RaterMayRate` checks `accounts.rater_removed_at` but not the pair row's
`profile_ratings.removed_at`. `YourRating` answers "none" for a removed row.

So after a moderator removes one rating, the rater sees these:

- `PersonProfile.MayRate = true` and `YourRating = 0`.
- After every resolve on that reporter's reports, `ratePromptFor` offers the star box again.
- Every tap then fails in `Rate` with `RatingNotAllowedMessage` (`UpsertRating` 0 rows,
  `RemovedAt` valid).

The rater gets an offer the server refuses every time, and the repeated prompt hints that a
moderator acted.

**Fix:** add to `RaterMayRate`:

```sql
AND NOT EXISTS (SELECT 1 FROM profile_ratings pr
                WHERE pr.rater_id = sqlc.arg(rater_id)::bigint
                  AND pr.reporter_id = sqlc.arg(reporter_id)::bigint
                  AND pr.removed_at IS NOT NULL)
```

## Info

### IN-S01: A Block racing a Follow can leave an active follow beside a block, and nothing ever cleans it

**File:** `internal/store/queries/people.sql:287-295` (comment "safe in either order"), `internal/service/people.go:551-561`; `internal/service/followsweep.go:157-166`

**Issue:** Each statement runs on its own under READ COMMITTED. `FollowAccount`'s
`NOT EXISTS (person_blocks)` is judged on the statement's snapshot.

1. A Follow statement starts before the Block row commits.
2. The block's `endFollowPair` UPDATE runs before the Follow commits. It does not see the
   new row, or it sees the old ended version.
3. The Follow then commits an active follow.

The sweep checks standing and tag only, not blocks, so the row lasts until one side acts.
Notify starts false and `SetNotify` is refused (the profile is not found), so no note goes
out. The visible effect is a follower count that includes a blocked person.
`RestoreTagLostFollows` can race the same way. The timing window is milliseconds.

**Fix:** have `Block` run `endFollowPair` a second time after a short read of
`PersonRelation`, or add a block check to `FolloweeStanding`'s use in the sweep (end the
pair with reason blocked). Also correct the "safe in either order" comment.

### IN-S02: Follow cap is read then written, so parallel follows can pass FollowingCap

**File:** `internal/service/people.go:332-347`

**Issue:** `CountFollowing` and then `FollowAccount` run with no lock or conditional insert.
Parallel taps from one account can end a few follows above 200. `SetNotify` documents the
same pattern; `Follow` does not.

**Fix:** put the count in the insert (`... AND (SELECT COUNT(*) ... ended_at IS NULL) < $cap`),
or document it the way `SetNotify` does.

### IN-S03: Blocked and Muted counts disagree with the lists for a person with a pending username

**File:** `internal/store/queries/people.sql:316-323` (CountBlocked), `internal/store/queries/person_mutes.sql` (CountMutedPeople) versus `internal/service/people.go:583-596` and `people_mute.go:95-108`

**Issue:** The counts include rows whose username is pending. The lists drop those rows in
Go. Settings can read "Blocked people, 3" and open a list of 2, with no way to reach the
third until the name clears.

**Fix:** add `AND a.username_screen = 'clear'` to both counts.

### IN-S04: Account deletion hard deletes the moderator audit trail of the ratings the account gave

**File:** `internal/store/queries/ratings.sql:207-214`, `internal/service/accountdelete.go:268-270`

**Issue:** `DeleteAccountRatings` also deletes rows with `removed_at`/`removed_by` set. A
rater removed for abuse who deletes their account erases the record of which ratings a
moderator removed. `accounts.rater_removed_at` survives on the scrubbed row. This may be
intended (G6), but D-14 calls removed rows an audit trail.

**Fix:** keep the removed rows (`DELETE ... WHERE removed_at IS NULL`), or record the choice
in docs/MODERATION.md.

### IN-S05: Follow fan out always leaves out the same followers past FollowFanoutCap

**File:** `internal/service/pushdispatch.go:104-108, 558-583`; `internal/store/queries/people.sql:151-161`

**Issue:** `NotifyFollowersOf` orders by `follower_id` with a limit of 500. The followers left
out of the phone note are always the highest ids, on every report. They get the bell note
only if their own poll runs before the single follow pass, because the report is marked
fanned once. This adds to K4.

**Fix:** document it next to the constant (see K4), or rotate the start the way the changes
sweep does.

## Known items K1 to K6 (part S view)

- **K1 (09-15, a moderator sees a pending rater username): Info, fix now, cheap.**
  `ModeratorRatingsForReporter` selects `a.username` with no screen filter. The moderator
  screen is the rule's one named exception, so this is no leak to the public. Select
  `username_screen` and mark or withhold pending names.
- **K2 (09-17, no star box for the resolve that closes the report): UI only, Info.** On the
  service side, `ratePromptFor` builds the prompt whether or not the report closed. Leave it
  for the owner, as the SUMMARY says.
- **K3 (09-26, pruned first row falls back to Critical or Reliable): Warning, fix with
  WR-S03.** When `FirstKind` is NULL, `confirmedBaseKind` picks Critical or Reliable. That
  skips `followHolds` and gates on the reliable switch (default off), so the result is one of
  two:
  - A legitimate follow update is dropped.
  - With the reliable switch on and a home area, the update is sent with no follow, block or
    mute recheck.

  Fix: when `FirstKind` is nil, skip the confirmed update; it replaces a note that is no
  longer known.
- **K4 (stale FollowFanoutCap comment): Info.** See IN-S05. Fix the comment when touching it.
- **K5 (follow row accessible name in handlers/updates.go): outside part S.** No view.
- **K6 (09-10, a comment push queued before a block goes out once): Warning, low frequency,
  in pushworker.go.** It has the same shape as WR-S03 for mutes. Both can be fixed in the
  same step 4 by reading `PersonRelation` and `PersonMuted` for every person level kind.

## Coverage notes

- These are covered by tests: distinct cell counting, the repeat tap that does not take the
  cell, the daily limit, Clear, removed and deleted raters, the private profile that only
  lowers, a New reporter never rated, the wiring guard, follow end cleanup, and the worker's
  follow rechecks.
- These are not covered: a rating change between the rating and the moderator removal
  (WR-S01), a retry after a failed restore (WR-S02), a person mute against a queued reliable
  push or confirmed update (WR-S03), the switch and Name edit race (WR-S04), and MayRate
  after a pair removal (WR-S05).
- `go vet ./internal/service/` is clean at e689824a.

_Reviewer: Claude (gsd-code-reviewer), part S_
