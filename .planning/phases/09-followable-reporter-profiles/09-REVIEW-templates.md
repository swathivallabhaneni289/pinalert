---
phase: 09
part: T
scope: templates, styles, contract tests, docs and the Policy
reviewed: 2026-10-30
reviewed_commit: e689824a
base_commit: 6f13da00
depth: standard
files_reviewed: 30
files_reviewed_list:
  - web/templates/person.html.tmpl
  - web/templates/people_list_window.html.tmpl
  - web/templates/settings.html.tmpl
  - web/templates/account_header.html.tmpl
  - web/templates/moderate.html.tmpl
  - web/templates/legal_body.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/profile.html.tmpl
  - web/static/css/person.css
  - web/static/css/stars.css
  - web/static/css/trust.css
  - web/static/css/places.css
  - web/static/css/moderate.css
  - web/person_contract_test.go
  - web/moderate_ratings_contract_test.go
  - web/trust_profile_link_contract_test.go
  - web/layout_rules_contract_test.go
  - web/moderate_contract_test.go
  - web/places_contract_test.go
  - web/settings_rows_contract_test.go
  - web/phase4_gates_contract_test.go
  - web/css_contract_test.go
  - web/labels_contract_test.go
  - web/noise_words_contract_test.go
  - web/chrome_contract_test.go
  - web/ways_in_contract_test.go
  - web/trust_contract_test.go
  - docs/TRUST-MODEL.md
  - docs/PUSH.md
  - docs/MODERATION.md
findings:
  critical: 0
  warning: 4
  info: 6
  total: 10
status: issues_found
---

# Phase 9: Code Review Report, part T (templates, styles, contract tests, docs, Policy)

**Reviewed:** 2026-10-30
**Depth:** standard, with checks traced into the handler, the service and the migrations where a template, a doc or the Policy makes a claim about them
**Files reviewed:** 30
**Status:** issues_found

## Summary

The profile page follows UI-SPEC 1b in order: disc and username, Name, stars, tag and sentence, Member since, home area, the counts, the actions, the rating mount, then the state line. It carries `noindex` in the markup and `X-Robots-Tag` plus `no-store` from the handler. Landmarks, labels, `aria-pressed` on the bell, Following and the stars, and the dialogs all match `#settings-delete-confirm`. Every display rule on an element that can be hidden is guarded with `:not([hidden])` or covered by `.btn[hidden]`. There is no `!important`, and no chosen item gets a dark border. The not found branch writes no request data.

The docs match the code on every constant I checked: `ReliableMinScore` 70, `FollowFanoutCap` 500, `RatingChangeGap` 24h, `ModeratorRatingsCap` 200, the rate buckets 60/1s, 20/3s and 10/30s, `PushPerAccountPerHour` 4, `NotifyFollowsCap` 50, `FollowedPostsCap` 200 and the rest of the TRUST-MODEL table. They also match on the dispatcher order (Critical, Follow, Reliable, Changes, then the confirmed update), the `CombinePoints` formula and both worked examples, `TappedHalfStars`, and the follow branch's "already written row is still queued" behaviour. No added doc or template line holds an em or en dash.

What is wrong:
- In forced colours, the new mask glyphs vanish. This hits the bell icon, the five rating stars, the header star and the More chevron. The project fixed this exact defect once before (code review part 3 WR-01), and the Phase 9 glyphs were left out.
- The laptop Recent reports column heads put muted text on the surface colour, at 4.34 to 1.
- The Policy does not say the Name now goes to OpenAI.
- The Policy leaves out several stored items: ended follows with their reason, cleared ratings, and the permanent rating bar.

## Warnings

### WR-T01: Forced colours: the Phase 9 mask glyphs paint Canvas on Canvas and vanish (the bell, the rating stars, the stars total, the More chevron)

**File:** `web/static/css/person.css:101-115` (`.person-stars__icon`), `:231-245` (`.person-more__chevron`), `:275-294` (`.person-bell__icon`), `:818-829` (the forced colours block); `web/static/css/stars.css:66-85` (`.stars-control__glyph`), `:93-97` (the forced colours block)

**Issue:** Each glyph is a box with `background-color` behind a mask. A forced colours theme replaces `background-color` with Canvas, so the glyph draws Canvas on Canvas and disappears. app-bar.css:425-444 spells this out and fixes it for `.app-icon`, and trust.css fixes it for the new `.trust-panel__profile-chevron` (trust.css:1185-1188, inside the forced colours block). None of the four Phase 9 glyphs is an `.app-icon`, and neither file's forced colours block opts them out. Both blocks only add borders.

What a forced colours reader sees:
- **The Notify me bell** is an empty bordered square. Its only name is visually hidden text, so a sighted Windows High Contrast user cannot tell what the button is. The "state is not colour alone, the icon differs too" rule of UI-SPEC 1d also fails: both icons are gone, and only the Highlight border on the pressed state is left.
- **The rating control** on a profile, and the same control in the star box after "Yes, mark resolved" on Nearby and My activity, is five invisible 44px targets. They have `border: 0` and `background: none`. Only the pressed star gets a Highlight border.
- **The stars total** loses its star, though the words still read. **More** loses its chevron, though the word stays.

`TestForcedColoursShowsTheMaskGlyphs` (web/review34_part3_contract_test.go:59) lists only four glyphs, so nothing catches this.

**Fix:**
```css
/* person.css, inside @media (forced-colors: active) */
.person-bell__icon,
.person-more__chevron {
  forced-color-adjust: none;
  background-color: ButtonText;
}
.person-stars__icon {
  forced-color-adjust: none;
  background-color: CanvasText;
}

/* stars.css, inside @media (forced-colors: active) */
.stars-control__glyph {
  forced-color-adjust: none;
  background-color: ButtonText;
}
```
Then add the four parts (person.css `.person-bell__icon`, `.person-more__chevron`, `.person-stars__icon`; stars.css `.stars-control__glyph`) to the `glyphs` list in `TestForcedColoursShowsTheMaskGlyphs`.

### WR-T02: The laptop Recent reports column heads are muted text on the surface colour, 4.34 to 1 in the light theme

**File:** `web/static/css/person.css:763-775` (`.person-report-cols`: `background: var(--color-surface); color: var(--color-text-muted)`)

**Issue:**
- In the light theme, `#6B7280` on `#F1F3F4` is 4.34 to 1. I computed it with the WCAG formula in a scratch Go program. The text is Label 14px/600, which is not large text, so the 4.5 to 1 rule applies and this fails.
- It breaks the owner rule "Muted text only on --color-bg" (UI-SPEC line 44), and the UI-SPEC Color section says text on the column head row takes `--color-text`. UI-SPEC 5b's "`--color-text-muted`" is the line that conflicts. The binding rules win.
- The My activity column head row that this one copies uses `--color-text` (activity.css:495, `.activity-cols`).
- Smaller case: a live row's `.person-report__label` (muted, person.css:511-517) sits on `--color-surface` while the row is hovered (person.css:464-466), so the hover state is also 4.34 to 1.

**Fix:** In `.person-report-cols`, change `color: var(--color-text-muted);` to `color: var(--color-text);`. For the hover, either give `a.person-report__body:hover .person-report__label` the color `var(--color-text)`, or keep the hover fill off the rows as My activity does.

### WR-T03: The Policy does not say the Name is now sent to OpenAI

**File:** `web/templates/legal_body.html.tmpl:106` and `:170`

**Issue:**
- Phase 9 sends the Name to the moderation provider when Let others follow me goes on, and again on every Name edit while it is on (`ScreenPublicName` calls `ScreenAbuse` at internal/service/publicname.go:81-85; MODERATION.md "Names on a public profile").
- The Policy names only reports (line 96), comments (105) and usernames (106) as text sent to OpenAI. The "Other services" list says "Report, comment and username checks: OpenAI" (line 170).
- A person's real name going to a third party is exactly the kind of transfer the Policy must name. The project rule is that every new personal data item belongs in the Policy.

**Fix:**
- Add a sentence after line 106: "When you switch on Let others follow me, and whenever you change your name while it is on, your name is checked by the same service. Only the name is sent, never your account, your sign in, your network address or your location. A name the check refuses, or cannot finish checking, is not shown."
- Change line 170 to "Report, comment, username and name checks: OpenAI, as described above."

### WR-T04: The Policy leaves out personal data that Phase 9 stores: ended follows with their reason, cleared ratings, the permanent rating bar, and the switch time

**File:** `web/templates/legal_body.html.tmpl:120-131`, `:142`, `:150-152`; `docs/MODERATION.md:404-409`

**Issue:** Checked against migrations 00020, 00021 and 00023 and the queries:

- **Ended follows are kept.** `account_follows` rows are never deleted when a follow ends. They keep `ended_at` and an `end_reason` from `unfollowed`, `removed`, `blocked`, `profile_off` or `tag_lost` until the account is deleted. A `blocked` reason row outlives an Unblock, so the store keeps a record that a block happened between two people after the block itself is gone. The Policy speaks only in the present ("Who you follow and who follows you, and who you have blocked or muted", line 150) and says switching off "ends every follow" (line 122). MODERATION.md:405 does say so ("each follow kept with an end time and a reason after it ends, until the account goes"), and it then claims "The Policy page says the same in general", which is not true for this item.
- **Remove my rating does not remove the rating.** `ClearRating` (internal/store/queries/ratings.sql:64-76) only stamps `cleared_at`. The row keeps the stars. `ModeratorRatingsForReporter` returns cleared rows, so a moderator still sees "{rater} gave {n} stars, Cleared by the rater". A reader who taps "Remove my rating" will believe it is gone. Line 152 names only moderator removals.
- **The permanent rating bar.** `accounts.rater_removed_at` and `rater_removed_by` record that a moderator barred the account from rating for good. The Policy says moderators can "remove abusive ratings" (line 129) but not that they can stop an account from ever rating again, or that this mark is kept.
- **The switch time.** `accounts.profile_public_at` is the time the profile was switched on. It is small but unlisted.
- **The home area line.** Line 142 still says the home area is "used for alerts", while line 124 says it is shown on the profile. Line 141 was updated for the Name; line 142 was not.

**Fix:**
- In "What Pinalert keeps", change line 150 to: "Who you follow and who follows you, and who you have blocked or muted. When a follow ends, it is kept with the time it ended and why (unfollowed, removed, blocked, profile switched off, or tag lost) until you delete your account. Follows are public on a profile that is switched on. Blocks, mutes and ended follows are private."
- Extend line 152 with: "When you remove your own rating it stops counting, but the rating stays stored and a moderator can still see it. If a moderator removes every rating you gave, your account can no longer rate, and that is kept with the moderator and the time."
- Add "when you switched your profile on" to the profile line.
- Change line 142 to end "used for alerts, and shown on your profile while Let others follow me is on."
- In MODERATION.md:404-409, add cleared ratings and the rater bar to the list of kept data, so that doc stays true too.

## Info

### IN-T01: A finished Rescue report on a profile gets a finer point than any other finished report

**File:** `internal/service/coarsen.go:55-59` (documented at `docs/TRUST-MODEL.md:685`)

**Issue:**
- `CoarsenFinishedForProfile` uses `ApproxGeohashPrecision` (6, about 1.2 by 0.6 km) for Rescue and `ProfileCoarsePrecision` (5, about 5 km) for everything else. So the most sensitive category gets the least blur.
- Because `CoarsePoint` is the same precision 6 cell centre the Past tab and the voted card use (auth.go:476-480), the profile row's `data-lat`/`data-lon` equal the anonymous Past row's point exactly.
- That defeats the D-20 reason given at TRUST-MODEL.md:685 ("would match the anonymous row of the Past tab at that point") for this one category.
- This is Info and not a Warning only because Rescue needed was retired for new reports on 2026-10-16 (report.go:84-87), and a profile lists 30 days. It can reach a stored legacy row only until about 2026-11-15. It sits on the service reviewer's side of the line, but the doc presents it as intended.

**Fix:** Use the coarser of the two precisions: drop the Rescue branch, keep `r.Description = ""`, and reword the TRUST-MODEL sentence.

### IN-T02: Heading structure: two h1 on the phone, none on the laptop not found and You blocked pages

**File:** `web/templates/person.html.tmpl:32`, `:42`, `:154`; `web/templates/app_bar.html.tmpl:30`; `web/static/css/phone.css:540`

**Issue:**
- Below 900px, the top bar's `<h1 class="app-topbar__title">` ("Profile" or "My profile") shows, and so does the username h1. Settings (`.place-head`, places.css:18) and My activity (`.activity-heading`, activity.css:97) hide their own h1 on the phone to keep one. Nearby clips its title for the same reason.
- At 900px and up, the top bar title is hidden. The not found page and the You blocked page then have only an h2, so there is no h1 at all.
- `TestPersonPageHeaderOrder` counts `<h1` in person.html.tmpl only, so it cannot see the included one.

**Fix:**
- On the phone, either make the username a visually styled h2, or clip the top bar title on this page as Nearby does.
- Make the two state titles h1 when the header is not drawn.

### IN-T03: Some copy checks in the profile contract test pass on ids and field names, not on visible words

**File:** `web/person_contract_test.go:108-112`

**Issue:** `strings.Contains(tmpl, "Follow")` passes on `FollowersText`, `"Block"` on `.BlockedByYou`, `"More"` on `aria-label="More"`, and `"stars"` on `person-stars`. Removing the Follow, Block or More button's visible label would not fail the test, even though the test says it pins P9-C6 and P9-C7 "verbatim".

**Fix:** Match the element text, for example `>Follow</button>`, `>Following</button>`, `>Block</button>`, `<span>More</span>`, and `">stars</span>`.

### IN-T04: A live Recent reports row reads its state two or three times to a screen reader

**File:** `web/templates/person.html.tmpl:126-127`; `internal/api/handlers/person_page.go:112-117`

**Issue:** The visually hidden prefix, the visible line and the chip each say the state, so the row name repeats it:
- A disputed row reads "Disputed, may not be true." (prefix), then "Disputed: may not be true" (the visible `DisputedLine`), then "Disputed" (the chip).
- A row that is not confirmed reads "Not confirmed yet." and then the chip "Not confirmed yet".

This is the same class as K5.

**Fix:** Mark the chip `aria-hidden="true"` when an `AccessiblePrefix` is set, and mark the visible disputed line `aria-hidden="true"`. The prefix already starts the name, as UI-SPEC "Interaction and Accessibility" asks.

### IN-T05: The "Couldn't load this profile" state (UI-SPEC Surface 6, P9-C12) is not built

**File:** `internal/api/handlers/person_page.go:181-184`, `:222-225`; `web/templates/person.html.tmpl`

**Issue:** A server error gives a bare `text/plain` "internal server error" with no frame, no bars and no Try again. The string "Couldn't load this profile" appears nowhere in the code. The Settings page behaves the same way, so this is a pattern, not a regression.

**Fix:** Render the template with an `Error` flag and a state box ("Couldn't load this profile", "Check your connection and try again.", a Try again link), or record this as a deliberate deviation in a SUMMARY.

### IN-T06: Stale comments leave Muted people out of the shared list window

**File:** `web/templates/people_list_window.html.tmpl:2`; `web/templates/settings.html.tmpl:326`; `web/static/css/person.css:602-604`; `web/person_contract_test.go:144-147`

**Issue:** These comments say the window serves "Followers, Following and Blocked people". Since plan 09-20 it also serves Muted people (`data-kind="muted"`).

**Fix:** Add Muted people to each comment.

## Known items K1 to K6 (assessed, not counted)

- **K1 (09-15, moderators see a pending rater's username).** This is the one documented exception to the UsernameScreenClear rule. MODERATION.md says so and leaves it open for the owner. Policy line 129 says moderators see raters "by username", which stays true. Risk is low: only moderators see it, and the tool needs a name to act. **Info. Fix later**, at the owner's choice, by labelling the row "(pending)" rather than hiding it.
- **K2 (09-17, the reader whose Yes, mark resolved closes the report sees no star box).** This is in votes.js and the server's rate prompt, not in my files. stars.css `.rate-box` is ready for it. The person most likely to judge the reporter is the one who closes the report, and UI-SPEC 3b names that moment. **Warning. Fix now** if the prompt is only being dropped by the closing path, since the box and its CSS already exist.
- **K3 (09-26, a confirmed update whose first follow row was pruned falls back to Critical or Reliable and skips the follow recheck).** PUSH.md:84 says "So a note stops the moment the follow, the bell, the profile, the tag, a block or a mute goes". TRUST-MODEL.md (the section "Checked again at send time") says the same. Both are false for that confirmed update, which can still reach a follower who has since blocked or muted the reporter. **Warning. Fix now in code**: keep the base as follow when the only first row of that pair was a follow row, or recheck the follow facts whenever a follow row ever existed. Otherwise qualify both doc lines.
- **K4 (09-26, stale FollowFanoutCap comment).** Confirmed. pushdispatch.go:103-107 says "only the phone note is left out" for a follower past the cap. `followBranch` (pushdispatch.go:575-583) queues a follow push from every followed_posted row of the report, including rows the follower's own poll wrote. PUSH.md and TRUST-MODEL.md describe the code correctly. **Info. Fix now**, a one line comment change.
- **K5 (09-22, the follow row's accessible name may say the category twice).** This is in updates.go, not my files. The same double wording shows up on the profile rows (IN-T04). **Info. Fix with IN-T04.**
- **K6 (09-10, a comment push queued just before a block can still go out once).** TRUST-MODEL.md:734 says a block "hides their comments from you ... and never notifies you of them". With K6, "never" is false for one queued push. Block is a safety control against harassment, so a comment note from the person you just blocked is the worst single case. **Warning. Fix now in code**: add the person_blocks check, in either direction, to `CommentPushStillHolds`, which keeps the doc true. Otherwise change the doc to "does not notify you of new ones".

---

_Reviewed: 2026-10-30_
_Reviewer: Claude (gsd-code-reviewer), part T_
_Depth: standard_
