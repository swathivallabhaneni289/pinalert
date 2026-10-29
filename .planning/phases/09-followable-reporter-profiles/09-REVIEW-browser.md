---
phase: 09
part: B (browser)
reviewed_commit: e689824a
base_commit: 6f13da00
reviewed: 2026-10-30
depth: deep
status: issues_found
files_reviewed: 22
files_reviewed_list:
  - web/static/js/activity.js
  - web/static/js/comments.js
  - web/static/js/map.js
  - web/static/js/moderate.js
  - web/static/js/people-list.js
  - web/static/js/person-reports.js
  - web/static/js/person.js
  - web/static/js/settings-people.js
  - web/static/js/stars.js
  - web/static/js/trust.js
  - web/static/js/votes.js
  - web/comments_test.mjs
  - web/moderate_ratings_test.mjs
  - web/people_list_test.mjs
  - web/person_bell_test.mjs
  - web/person_mute_test.mjs
  - web/person_reports_test.mjs
  - web/person_test.mjs
  - web/settings_people_test.mjs
  - web/stars_test.mjs
  - web/trust_profile_link_test.mjs
  - web/votes_stars_test.mjs
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
---

# Phase 9: Code Review Report, part B (browser)

**Reviewed:** 2026-10-30
**Depth:** deep (each script read in full, its diff from 6f13da00, the templates it drives, and the server handlers and queries behind each call)
**Files Reviewed:** 22
**Status:** issues_found

## Summary

Every Phase 9 script writes text only. I found no markup sink: `textContent`,
`Pinalert.setText`, `setAttribute` and `createElement` throughout, and every
address is built from `encodeURIComponent`. All eleven node test files pass
(214 tests). Every username a script draws comes from a server read that checks
the screen is clear. The rate box name comes from `PublicProfilePathsForViewer`,
the profile link path from the same query, the Blocked and Muted lists filter on
`UsernameScreenClear`, and `reply_to_author` requires `rn.username_screen =
'clear'`. The one exception is the moderator ratings screen (K1). No Critical
finding.

The real problems are stale state and lost focus:

- A rating can end the rater's own follow, and the profile does not redraw.
- The trust panel's profile link is rebuilt on every poll.
- The Settings Blocked count is recomputed from a filtered list.
- A follow 409 hides the focused button.
- One valid username shape never gets a profile link at all.

No source file was modified. The tests were run with `node --test` only; no database was touched.

## Warnings

### WR-B01: "See this reporter" is never drawn for a username that starts with an underscore

**File:** `web/static/js/trust.js:50`

**Issue:** `PROFILE_PATH` is `^\/u\/[a-z\d][a-z\d_.][a-z\d_.]+$`, so the first
character must be a letter or a digit. The server's rule
(`internal/service/username.go:63` and `usernameShapeOK` at line 93) allows a
leading underscore: the shape is `[a-z0-9_.]{3,20}`, and the only extra rules are
a letter somewhere, no leading or trailing full stop, and no `..`.
`UsernameReserved` has no underscore rule either.

Scenario: a Reliable reporter named `_ravi` opts in.
- `loadProfilePaths` sends `/u/_ravi`, and `setProfileLink` silently draws nothing on every map card and every Activity row.
- `node -e` confirms `PROFILE_PATH.test('/u/_ravi') === false`.
- That reporter can still be followed by typing the address, but the D-04 way in is missing.

The plan asked for `^/u/[a-z0-9_.]{3,20}$`. SUMMARY deviation 4 rewrote it to dodge the numeral test, and that rewrite introduced the narrower first character. The bad list in `web/trust_profile_link_test.mjs:148` has no underscore case, so nothing caught it.

**Fix:** allow the underscore first and keep the no-dot-first rule:
```js
var PROFILE_PATH = /^\/u\/[a-z\d_][a-z\d_.][a-z\d_.]+$/;
```
Add `'/u/_ravi'` as a good case in `trust_profile_link_test.mjs`.

### WR-B02: The profile link is removed and rebuilt on every 30 second poll, so focus on it is lost

**File:** `web/static/js/trust.js:819-855`, called from `web/static/js/map.js:1295`

**Issue:** `setProfileLink` always runs `target.profileLink.remove()` and then
appends a new `<a>`. `updatePopupCard` runs on every poll (the map.js header
comment at line 11, and the call at line 2001), and the rest of that function
takes care to keep nodes in place ("a poll never replaces the text under a
reader's selection", line 1245). A keyboard or screen reader user who has Tabbed
to "See this reporter" in an open popup or the laptop panel has focus dropped to
`<body>` at the next poll. VoiceOver then restarts from the top of the page. The
same thing happens to a pointer user who is mid press.

**Fix:** keep the existing node when the path has not changed:
```js
if (target.profileLink && typeof path === 'string' &&
    target.profileLink.getAttribute('href') === path) {
  return;
}
```
Place this check before the removal, and add a test that two calls with the same path leave the same node.

### WR-B03: After a rating, the profile keeps a stale tag, stale stars and a follow that the server has ended

**File:** `web/static/js/person.js:236-249` (onStatus is only `say`) and `web/static/js/person.js:286-312` (`draw` never writes the star or tag fields)

**Issue:** A successful star tap or Remove my rating never calls `refresh()`.
Even when a refresh does run (after a follow or a mute), `draw()` never writes
`stars_number`, `stars_label`, `stars_from_reports`, `stars_from_ratings`,
`tag_word` or `tag_sentence`. The server side makes this a visible bug.
`RatingService.Rate` ends with `reevaluate`
(`internal/service/ratings.go:310-323`): when the write leaves the reporter
below Reliable, it calls `endFollowsOf(..., FollowEndTagLost)` and ends every
follow of that reporter, the rater's own included.

Scenario: a reader follows a Reliable reporter who has the bell on, and taps 1 star. The rating is the one that tips the score under the cut.
- The page still says "Reliable reporter" with the old total.
- It still shows Following and the pressed bell, although the follow no longer exists.
- Tapping the bell gets the 409 "Couldn't change this. Try again." with no redraw, so the reader is stuck until they reload.

This is exactly the "state that can go stale after a rating change" the brief asks about.

**Fix:**
- In `mountRating`, pass `onStatus: function (line) { say(line); refresh(); }`.
- In `draw()`, write the header fields the server already sends: `person-stars` and its parts, `person-tag` and `person-tag-sentence`. Create or remove the stars group when `stars_number` turns empty or non empty.
- Also call `refresh()` after a 409 in `toggleBell`, so a stale bell is taken away.

### WR-B04: The Settings Blocked people count is recomputed from a filtered, capped list after an Unblock

**File:** `web/static/js/settings-people.js:294-299`

**Issue:** After one Unblock, `refreshCounts('blocked')` sets the count to `answer.body.entries.length`. The two server reads disagree:
- `CountBlocked` (`internal/store/queries/people.sql:316`) counts every live blocked account that has a username.
- `PeopleService.BlockedPeople` (`internal/service/people.go:583-596`) drops each one whose screen is not clear, and caps at 500.

Scenario: a reader has blocked 3 people, and one of them changed username to a name still pending. Settings shows 3. The window lists 2. After one Unblock, the row drops from 3 to 1, not 2.

The muted path a few lines above (lines 285-292) says why reading the list length is wrong and decrements instead. The blocked path does the very thing that comment warns against.

**Fix:** treat Blocked like Muted. Take the shown count down by one per successful Unblock and never below zero, or have the server return `CountBlocked` in the list answer and use that. Add a settings_people test with a count larger than the list.

### WR-B05: A follow 409 hides the focused Follow button for every 409, cap included, and focus falls to the page

**File:** `web/static/js/person.js:420-427`

**Issue:** On any 409 from `/follow`, the code sets `els.follow.hidden = true`
while that button has focus, and `fail()` moves focus nowhere, so it lands on
`<body>`. UI-SPEC line 475 says focus stays on the control used. UI-SPEC line
205 removes Follow only for "can't follow right now". The handler also sends a
409 with the same field `follow` for `ErrFollowingCap` ("You can follow up to
200 people.", `internal/api/handlers/people.go:169`).

Scenario: a reader at the cap taps Follow.
- The button vanishes and focus drops to the page top.
- After they unfollow someone else, the Follow button stays gone until a reload.

**Fix:**
- On a 409, call `refresh()` and let `can_follow` decide whether Follow shows. Keep the error line as written.
- Then focus Follow if it is still shown, else More (`els.more.focus()`), as `toggle` already does on success.

### WR-B06: Show more hides the button that was just pressed, so keyboard focus leaves the list window

**File:** `web/static/js/people-list.js:658-661`

**Issue:** The Show more click handler sets `els.more.hidden = true` before
`load(next)`. The focused button disappears, focus goes to `<body>` (outside the
dialog panel), and nothing puts it back when the next page is appended. A
keyboard or VoiceOver user loses their place in a 50 row list. The next Tab is
caught by the trap in `onWindowKeydown` and sent to the first item, the Close X
at the top. UI-SPEC line 475 asks that focus never be dropped by a script.

**Fix:**
- Keep the button shown and disabled with `aria-busy="true"` while the page loads.
- After `draw(..., false)`, move focus to the first newly appended row link, or to the button again when there is no new row.
- On failure, leave focus on the button (it is already shown again by `failLoad`).

## Info

### IN-B01: The list window and the Settings switch drop the server's 429 sentence and the offline line

**File:**
- `web/static/js/people-list.js:415-445` (`undoOne`)
- `web/static/js/people-list.js:504-520` (`runRemoveAll`)
- `web/static/js/settings-people.js:203-205`

**Issue:** `person.js` (`refusal`, `offlineOr`) shows the server's 409 or 429
sentence, and the built offline line when the request fails while offline. These
three paths do not:
- A 429 from `PeopleActionMiddleware` shows "Couldn't unblock. Try again." in place of `PeopleLimitMessage`.
- A request rejected because the signal dropped mid call shows the generic line, not the P9-C13 offline line.

**Fix:**
- Reuse the person.js pair. For any non ok answer, use `serverMessage(answer)` when the status is 409 or 429.
- In each rejection handler, check `navigator.onLine === false` and show `MESSAGE_OFFLINE`.

### IN-B02: A profile refresh can overwrite a star save that is still in flight

**File:** `web/static/js/person.js:272-274` with `web/static/js/stars.js:296-299`

**Issue:** The person.js `busy` flag and the control's own `busy` flag are
separate. Scenario: the reader taps a star, then taps Follow before the rating
answer arrives.
- The follow's `refresh()` GET can read the old `your_rating`.
- `control.set(old)` then redraws the old stars, and the later `apply()` sets the new value.
- Depending on which answer lands last, the stars and Remove my rating can show the wrong state until the next action.

**Fix:** make `set()` a no op while the control is busy, or have `syncRating` skip `control.set` while a save is out.

### IN-B03: Unmute or Unblock in the list reports success for a person who renamed since the list was read

**File:** `web/static/js/people-list.js:415-440`, with `internal/service/people.go:470-483`

**Issue:** The row posts by the username drawn at load. `idByUsername` treats a well formed name that matches nobody as success.

Scenario: the person changes username while the window is open, and the reader taps Unmute.
- The browser removes the row and says "{old} is unmuted."
- Settings takes the Muted count down by one.
- The mute row still exists under the new name, and it reappears on the next open.

This is narrow, but the window tells the reader something that is not true.

**Fix:** have the unmute and unblock answers say whether a row changed (the `:execrows` count is already there). When it is 0, reload the list instead of removing the row.

### IN-B04: Settings decides whether to ask before turning the profile off from a count read at page load

**File:** `web/static/js/settings-people.js:208-211, 242-247`

**Issue:** `followerCount()` reads the number drawn at page load. If the shown count is 0 and someone has followed since, the switch turns off with no box. The server then ends those follows (`endFollowsOf`, `FollowEndProfileOff`) without the P9-C22 question being asked.

**Fix:** read `/api/people/{me}` before deciding, or always ask when the switch goes off.

### IN-B05: The rate box's onGone removes the box with focus inside it

**File:** `web/static/js/votes.js:2077-2079`

**Issue:** A 404 from the rating call removes the whole box while a star has focus, and focus falls to `<body>`. Skip and a successful save both take care to move focus (lines 2063-2069 and 2083-2089). onGone does not.

**Fix:** in `onGone`, check `box.contains(document.activeElement)` first, and focus `block.takeBack.undoBtn` when it can take focus, as Skip does.

## Known items (K1 to K6) that touch this part

- **K1 (09-15, a moderator sees a pending rater's username).** The browser side is `web/static/js/moderate.js:1381-1414` (`buildRatingRow`), which draws `rating.rater` as sent.
  - **Severity:** Warning. It breaks the screen-clear rule. The audience is moderators only, and the rule's own text calls this screen the one exception.
  - **Fix now:** yes, on the server. Send a fixed placeholder ("Name being checked") for a rater whose screen is not clear, and keep a stable row key that is not the name. As written, `data-mod-rating-rater` doubles as the Remove key, so a placeholder would also need a separate opaque rater key.
  - **A second effect:** `renderRatings` skips rows whose `rater` is empty, so ratings by deleted raters vanish. The moderator can read "No ratings for this person." while ratings still exist.
- **K2 (09-17, the reader whose resolve closes the report sees no star box).** This is `web/static/js/votes.js:1697-1701` with `rateHostIsLive` (2028-2034). It is deliberate: the box mounts after the refetch, when a just closed card is gone.
  - **Severity:** Info. No wrong data is shown; the rating can still be given on the profile.
  - **Fix:** later. Mount the box in the toast host when the card host is no longer live.
- **K5 (09-22, the follow row label may say the category twice).** The browser only draws the label the server sends. There is no browser fix; fix it in `updateRowLabel`.
- **K3, K4, K6** are server side (`pushdispatch.go`, `CommentPushStillHolds`), with no browser part. I found no browser path that makes them worse: comments.js hides a blocked author's comment through `data.hidden` in all six places that gate an action.

## Checked and found sound

- No markup sinks in any Phase 9 script.
- Every confirm box opens with focus on its title, closes only through its two buttons or Escape, and blocks Escape while a call is out.
- The bell shows only while `following === true && muted !== true`, in both the template and `drawBell`.
- Mute and Unmute redraw from the refetched profile.
- The Unblock and Unmute rows move focus to the next row's button or to the empty heading.
- The star control has a fieldset and legend, five named buttons, `aria-pressed` on the chosen star, and no arrow key handling. Focus returns to the star that was used, or to the first star after Remove.
- `.btn[hidden]` (chrome.css) keeps Remove my rating hidden.
- The phone and laptop share every script path. Only person.css placement differs. The Low data page carries no profile link, which SUMMARY 09-09 accepted.

---

_Reviewed: 2026-10-30_
_Reviewer: Claude (gsd-code-reviewer), part B_
_Depth: deep_
