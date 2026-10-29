---
phase: 09
part: edge (handlers, router, store, migrations, cmd)
part_letter: E
reviewed_commit: e689824a
base_commit: 6f13da00
reviewed: 2026-10-30
depth: deep
status: issues_found
findings:
  critical: 0
  warning: 4
  info: 7
  total: 11
known_items_assessed: [K1, K2, K3, K5, K6]
files_reviewed_list:
  - cmd/seedprofiles/main.go
  - cmd/server/main.go
  - internal/api/handlers/activity.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/comments.go
  - internal/api/handlers/moderation_ratings.go
  - internal/api/handlers/people.go
  - internal/api/handlers/people_mute.go
  - internal/api/handlers/person_page.go
  - internal/api/handlers/ratelimit_gate.go
  - internal/api/handlers/ratings.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/settings.go
  - internal/api/handlers/updates.go
  - internal/api/handlers/votes.go
  - internal/api/router.go
  - internal/demo/memquerier.go
  - internal/store/migrations/00020_add_people_profiles.sql
  - internal/store/migrations/00021_add_profile_ratings.sql
  - internal/store/migrations/00022_add_person_mutes.sql
  - internal/store/migrations/00023_add_follow_notes.sql
  - internal/store/migrations/00024_add_follow_push.sql
  - internal/store/queries/accounts.sql
  - internal/store/queries/comments.sql
  - internal/store/queries/people.sql
  - internal/store/queries/person_mutes.sql
  - internal/store/queries/profile_reports.sql
  - internal/store/queries/push.sql
  - internal/store/queries/ratings.sql
  - internal/store/queries/reports.sql
  - internal/store/queries/updates.sql
  - internal/testutil/db.go
  - internal/testutil/people.go
---

# Phase 9: Code Review, part E (edge)

**Range:** 6f13da00..e689824a (the three 261029-rtx commits ignored)
**Depth:** deep (handlers traced into internal/service where a rule lives there)
**Status:** issues_found

## Summary

What I checked:

- **Access gate.** Every new JSON route and both new pages (`/u/{username}`, `/me`) are inside the `requireVerifiedAccount` group. The three moderator rating routes are inside the `requireModerator` group.
- **No ids in JSON.** No JSON answer carries an account id, an email, a cell or a coordinate. `PersonProfile.accountID` is unexported, and finished profile rows drop their id, words and time.
- **Uniform 404.** The not found body comes from one function (`writePersonNotFound`) on every read and action route. The service decides "not found" before any count, tag or relation answer, so a private, pending, deleted or blocking person cannot be told apart from an unknown one.
- **Migrations.**
  - 00020 to 00024 are safe on an existing database: the new columns have defaults, the new CHECKs hold on the rows that exist, and the new unique indexes only widen predicates over rows that were already unique.
  - The Down lists match their sources: `updates_kind_valid` matches 00019 Up, and `push_report_progress_state_valid` matches 00011.
  - Every ON CONFLICT on `uq_updates_first_alert` and `uq_push_outbox_first` names the index predicate word for word.
- **Ended follows.** Every query that reads current follows filters `ended_at IS NULL`. The one exception is the hard delete that goes with an account.
- **Block either way.** Every query that names people checks blocks in both directions. `PublicProfilePathsForViewer` only checks one direction, which is right: a person the viewer blocked shows the "You blocked" page.
- **Counts.** The named list counts match the list predicates exactly.
- **`cmd/seedprofiles`.** It refuses the protected names, any name without the `p9_` prefix and any host that is not on this machine. It decides this before it connects, using the driver's own parse (so the service file and PGDATABASE are covered). After connecting it checks `current_database()` again.

There are no Critical findings. The four Warnings are:

- a rate bucket bypass on the profile page;
- a rate bucket that is too loose for a route that calls the shared screener;
- a moderator "rights check" in the handler that does not check rights;
- a D-12 gap: Block only works on opted in profiles, so most commenters cannot be blocked.

## Warnings

### WR-E01: The profile page `/u/{username}` skips the PeopleRead bucket the JSON reads go through

**File:** `internal/api/router.go:1109` (with `internal/api/handlers/person_page.go:174-218`)

**Issue:** `GET /api/people/{username}` and `GET /api/people/{username}/reports` sit behind `PeopleReadMiddleware`. Its doc comment says it exists to slow scraping, and that "a 404 probe pays too".

`GET /u/{username}` does the same work with no bucket at all:

- `people.Profile`, which includes the evaluate on touch `endFollowsOf` write;
- `reports.ProfileReports`, which returns up to 100 rows with live ids, descriptions and exact points.

The router comment calls this deliberate: "the page load is already behind the access gate". But the access gate is exactly what the JSON routes also have.

**Scenario:** one signed in account runs a loop over a username wordlist against `/u/<name>`. It gets a 200 or a 404 for each name, and the full recent report list of every public reporter, at network speed. Each 200 for a non Reliable public profile also runs an `UPDATE account_follows`. The 60 burst, one per second limit protects nothing while this door is open. (The HTML page does not render the follower lists, so those stay throttled.)

**Fix:** put the page behind the same bucket. Give `peopleMiddleware` an HTML flavour so a page gets a plain 429 page and not the JSON body. `NewRateGate` already receives `deps.Chrome`.

```go
r.With(rateGate.PeopleReadPageMiddleware()).Get("/u/{username}", handlers.PersonPage(deps.Template, people, deps.Reports, page))
```

The smallest form is to reuse `PeopleReadMiddleware()` and accept a JSON 429 on a page, which a person scrolling profiles at under one a second never sees.

### WR-E02: The visibility switch calls the shared screener on every off to on turn, under the loose PeopleAction bucket

**File:** `internal/api/router.go:1048`, `internal/service/people.go:213-280`

**Issue:** `PATCH /api/me/visibility` uses `PeopleAction` (burst 20, then one every 3 seconds). Every off to on change runs `ScreenPublicName` on the stored Name, which costs one `ScreenAbuse` call. A pending username costs a second call. Nothing remembers that this unchanged Name already passed.

The comment bucket is 6 then one per 20 seconds precisely because "every attempt may cost one screener call". This route is about three times looser. It also spends the same screener that report submission uses (`deps.Reports.Screener()`).

**Scenario:** a script alternates `{"profile_public":false}` and `{"profile_public":true}`. That is about 600 on turns, so about 600 moderation calls an hour, from one account and with no network address limit. A few accounts can exhaust the provider rate limit the app shares. Report screening then answers "unfinished" for everyone posting an emergency report.

**Fix:** either of these.

- Give the switch its own tight bucket. It can be the comment bucket's numbers:
  ```go
  r.With(rateGate.VisibilityMiddleware()).Patch("/api/me/visibility", handlers.UpdateVisibility(people))
  ```
  with `Visibility: BucketLimit{Every: 20 * time.Second, Burst: 6}`.
- Or skip the Name screen when the Name has not changed since it last passed. Store a `public_name_screened` value (the name or its hash) when the profile is switched on.

### WR-E03: The moderator rating handlers do not check moderator rights; `modGuard` only checks that some account is in the context

**File:** `internal/api/handlers/moderation_ratings.go:7-9, 71, 111, 140`; `internal/api/handlers/moderation.go:178-192`; `internal/api/moderator.go:62`

**Issue:** the file header says the RatingService moderator methods check no rights, "so every handler here starts with modGuard, which refuses a request that did not come through the group." That is not what the code does. `modGuard` checks the method and `account.FromContext`. `requireVerifiedAccount` puts the same `account.Account` into the context for every signed in person, so the guard cannot tell a moderator from anyone else.

Nothing is wrong today: lines 1196-1198 are inside the `requireModerator` group, and `TestModeratorRatingsHiddenFromOthers` pins that. But the brief's rule (rights checked in the handler) is not met. And if one of these lines is moved one block up, it becomes a full privilege escalation. `ModeratorRemoveRater` is permanent (`MarkRaterRemoved` has no undo route) and ends or restores other people's follows.

**Fix:** have `requireModerator` set a marker that only it can set, and make `modGuard` require it.

```go
// moderator.go
ctx := account.WithModerator(account.WithAccount(r.Context(), acc))
// moderation.go modGuard
if !account.IsModerator(r.Context()) { http.NotFound(w, r); return account.Account{}, false }
```

This covers the Phase 4 and Phase 8 moderator handlers too, since they share `modGuard`.

### WR-E04: Block works only on a visible public profile, so D-12's "hides their comments from you" is out of reach for most commenters

**File:** `internal/service/people.go:528-560` (reached from `internal/api/handlers/people.go:570-572`)

**Issue:**

- `Block` returns `ErrPersonNotFound` unless `publicFace` holds: opted in, clear username, not blocking you.
- Profiles are off by default (D-01). A comment shows its author's username whenever the screen is clear, whatever `profile_public` says (`comments.sql` join `au`).
- The comment UI has no Block entry (`web/static/js/comments.js` only draws the Comment hidden placeholder). The only way to block is the profile page, and for a private person that page is the uniform not found page.

D-12 says Block "hides their comments from you (Phase 8)". Plan 09-10 built the comment side of that carefully, but it can only fire for people who opted in.

The stated reason, "so a block cannot be used to probe who has a private profile", does not need this restriction. `Unblock`, `Unfollow` and `Unmute` already answer 204 for any well formed username, known or not, and reveal nothing.

**Scenario:** a private account posts abusive replies under someone's report. The person tapping the name gets "Profile not found" and has no way to block them.

**Fix:** resolve the name the way `Unblock` does. Use `idByUsername`, answer 204 for a well formed name that matches nobody, block any live account, and keep the 404 for a malformed name only. Then add Block to the comment row's menu (a UI change outside this part). If the owner prefers the current rule, write it into 09-CONTEXT D-12 as accepted, because today the decision text promises more than the code does.

## Info

### IN-E01: A Follow racing a Block (or a profile switch off) can leave an active follow, despite the comment saying nothing can slip in

**File:** `internal/store/queries/people.sql:48-73, 287-295`; `internal/service/people.go:520-560`

**Issue:** under READ COMMITTED, `FollowAccount`'s `NOT EXISTS person_blocks` check can run before the `BlockAccount` insert commits. `endFollowPair` can then run before the follow insert commits and see no row. The follow commits afterwards and stays active.

The sweep's `FolloweeStanding` does not look at blocks, so nothing ends it. The impact is small:

- `SetNotify` refuses a blocked pair, so no bell can be turned on;
- the worker rechecks blocks at send time;
- the follow still shows in both counts.

The same gap exists for `SetProfilePublic(false)`, but there the sweep repairs it.

**Fix:** take `SELECT ... FOR UPDATE` on the followee's `accounts` row in both `Block` and `Follow`, in one transaction each. Or add a both-directions block test to `FolloweeStanding`, or to a sweep query that ends blocked pairs.

### IN-E02: The list cursor carries the exact follow time, and a crafted cursor can bisect anyone's follow time

**File:** `internal/service/people.go:733-759` (read through `internal/api/handlers/people.go:320-336`)

**Issue:** `next` is base64 of `created_at` in microseconds plus a username. Any reader of a public list gets the exact time the 50th named follower followed. They can also send their own `after=<t>:<name>` cursors and learn, by bisection, when each named follower followed (about 40 reads per person under PeopleRead). No screen shows follow times.

**Fix:** sign the cursor (an HMAC with a server key), so only cursors the server issued are accepted. Or page by an opaque row position.

### IN-E03: A failed moderator removal cannot be retried into its follow restore

**File:** `internal/service/ratings.go:421-470` (through `internal/api/handlers/moderation_ratings.go:127-131, 151-155`)

**Issue:** `RemoveRating` / `RemoveRatingsByRater` commit first, then `reevaluateAfterRemoval` runs outside any transaction. If that step fails, the moderator sees a 500. A retry then finds nothing to remove (`ErrNoRows` returns nil, and `removed` is 0), so `RestoreTagLostFollows` never runs. The follow sweep never restores tag_lost follows.

**Fix:** run the removal and the re-evaluation in one transaction. Or on a retry, re-evaluate whenever the row is already removed by this moderator.

### IN-E04: The Phase 9 JSON bodies accept trailing data and any Content-Type

**File:** `internal/api/handlers/people.go:700-707, 750-757`; `internal/api/handlers/ratings.go:82-89`; `internal/api/handlers/moderation_ratings.go:115-122`

**Issue:** each decodes one value with `DisallowUnknownFields`, but never checks for a second value. `{"on":true}{"x":1}` and `{"stars":4} garbage` are accepted. The auth and push decoders (`credentials.go:136-141`, `push.go:80-85`) refuse both trailing data and a non JSON media type. The cookie is SameSite=Lax, so this is not a CSRF hole, only weaker strictness than the routes the brief compares against.

**Fix:** after `Decode`, add `if _, err := dec.Token(); !errors.Is(err, io.EOF) { 400 }`. Optionally share one `decodeStrict` helper with `decodePushBody`.

### IN-E05: `comments.go` is not gofmt clean

**File:** `internal/api/handlers/comments.go:66`

**Issue:** the `Held` field lost one space of alignment in 58787b0f, so `gofmt -l` lists the file. CI does not run gofmt, so nothing fails today.

**Fix:** run `gofmt -w internal/api/handlers/comments.go`.

### IN-E06: Account deletion leaves queued follow pushes about the deleted person, and two comments disagree with the code

**File:** `internal/service/accountdelete.go:258-266`; `internal/store/queries/push.sql:445-448`; `internal/service/pushkinds.go:43`

**Issue:** the `DeleteQueuedFollowPushesForFollowee` comment lists "the profile ... was deleted" as a trigger. Deletion hard deletes the follows but never calls it. The followers' queued `follow` rows about the deleted reporter's reports wait until the worker skips them (`ActiveFollowNotify` is now false). PUSH.md's list rightly leaves deletion out, so only the SQL comment is wrong. `pushkinds.go:43` still says `uq_push_outbox_first` covers critical and reliable only.

**Fix:** call `q.DeleteQueuedFollowPushesForFollowee(ctx, accountID)` in the deletion transaction, before `DeleteAccountFollows`. Update the `pushkinds.go` comment to name follow.

### IN-E07: The uniform 404 tests skip the blocking viewer on the later routes

**File:** `internal/api/handlers/people_mute_e2e_test.go:123-131`, `people_notify_e2e_test.go:~152`, `ratings_e2e_test.go:176-181`

**Issue:** `TestProfileNotFoundIsUniform` covers unknown, private, pending, deleted and blocking, but only for the four reads and follow. The mute, notify and rating tests cover private, unknown or malformed, but not "the person blocked the caller", and block itself has no byte equality row. The code is uniform today: every path goes through `publicFace(..., rel.BlockedViewer)`. Nothing pins it for the routes added in plans 14, 19 and 23.

**Fix:** extend the table in `TestProfileNotFoundIsUniform` with POSTs to `/block`, `/mute`, `/notify` (`{"on":true}`), `/rating` (`{"stars":3}`) for all five names, and compare the bodies byte for byte.

## Known items touching this part

| Id | Where | Confirmed | Severity | Fix now? |
|----|-------|-----------|----------|----------|
| K1 | `internal/store/queries/ratings.sql:125-141` (`ModeratorRatingsForReporter` filters `username IS NOT NULL` only), sent at `moderation_ratings.go:93-99` | Yes | Info | No. It is moderator only and is the documented exception. Optional: add `a.username_screen` to the row and mark a pending name in the tool, so a moderator knows it is unscreened. |
| K2 | The server sends `rate_prompt` on every resolve (`internal/service/trust.go:704-709`; no early return before it). The box is dropped client side by `rateHostIsLive(block)` in `web/static/js/votes.js:2145` once the closing resolve moves the row out of the live list | Yes (cause is in votes.js, outside this part) | Warning | Yes, in the UI part. The closing reader is the best placed rater and never gets the box. Host the box on the Past row or in a page level host when the live row is gone. |
| K3 | `push.sql` `ClaimDuePushes` first_kind LEFT JOIN, with `confirmedBaseKind` falling back to Critical or Reliable | Yes | Info | No. `EnqueueConfirmedUpdates` needs the first row still sent, so a NULL first_kind needs the 7 day `PushOutboxRetention` prune to land between the enqueue and the claim. That is seconds against days. |
| K4 | `pushdispatch.go` | Not in this part | none | none |
| K5 | `internal/api/handlers/updates.go:187-206` | Yes. For a follow note, `item.ReportLine` is already "Flood, Critical." (`updatesservice.go:524`), and line 198 appends "Flood, Critical." again, so the row label reads "New. ravi.k posted a report. Flood, Critical. Flood, Critical. ..." | Warning (screen reader users hear it on every follow row) | Yes. Use `if item.Category != "" && item.Kind != service.UpdateFollowedPosted {`. |
| K6 | `internal/store/queries/push.sql:666-700` (`CommentPushStillHolds`) | Yes. The write side (`comments.sql` InsertCommentUpdate) skips a blocked commenter, but a note queued before the block still passes the send time check | Warning (one note, once) | Yes, one clause. Add `AND NOT EXISTS (SELECT 1 FROM person_blocks pb WHERE pb.blocker_id = sqlc.arg(recipient_account_id)::bigint AND pb.blocked_id = c.account_id)` inside the EXISTS. |

---

_Reviewed: 2026-10-30_
_Reviewer: Claude (gsd-code-reviewer), part E_
_Depth: deep_
