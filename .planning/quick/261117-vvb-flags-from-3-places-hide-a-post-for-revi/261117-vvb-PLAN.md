---
quick_id: 261117-vvb
slug: flags-from-3-places-hide-a-post-for-revi
date: 2026-11-17
type: quick
---

# Flags from 3 places hide a post for review, and the bell tells the poster

## Why

7.1 UAT test 7, 2026-11-17. With the Posted part leaving My activity (quick 261117-vuf, in
parallel), the poster's held and removed notes needed a new home. The owner:
- "if it's like under review or something, it will like straight up pop up in the notifications.
  And if they want to like see that the progress or anything ... built in the notifications"
- "if people are like complaining about this one particular post, I think we should definitely
  take an action", "How does other websites or apps deal with" it. Told: big apps never remove by
  flag count alone, community apps hide after several independent complaints until a person
  looks. Proposed: hidden for review once 3 people in 3 different places flag it, the poster told
  in the bell. Owner: "yes do that".
- Agreed table ("Is this right?" then "yes build it"), and "Knowing these updates in both laptop
  version and phone versions" (phone and laptop the same).

This reverses the documented rule "A flag never hides or changes a report. Only a moderator's
Remove does." (docs/MODERATION.md:186) for reports only. Comments are unchanged.

## What changes

### 1. A flag records the flagger's place

- The report Flag request may carry the flagger's current position (lat, lon), read from the same
  position source a vote uses on that page. No new location prompt: with no known position the
  flag is sent without one, exactly as today.
- The server turns the position into a place cell at the voter cell precision
  (`voterGeohashPrecision`, 7, the precision confirmations count by) and stores only the cell on
  the flag, in a new nullable column (migration `00026`, the next free number; Down drops it).
  The position itself is never stored. The cell is service internal: never serialised to any
  client, never shown on the moderator screen, never logged. Invalid lat or lon is refused the
  way the vote route refuses it.

### 2. Hidden for review

- A report is hidden for review when its open (not dismissed) flags come from at least **3
  different accounts in 3 different place cells** (`FlagHideThreshold = 3`). A flag with no cell
  never counts toward it, but still reaches the moderator as now. Decide the hide in the same
  transaction as the flag insert, so concurrent flags cannot skip or repeat the change.
- Only a shown report (`moderation_state = 'clear'`) that a moderator has not already kept after
  a review hide can be hidden this way. Held (pending) and removed reports never move to it.
- Hidden for review is withheld everywhere, exactly like a held or removed report: feeds, map,
  report page and focus, Past, profile, search, comments, share, votes and flags (404), push
  recipients. It fails closed through the existing visibility path. Prefer a new
  `moderation_state` value `'review'` (extend the CHECK in 00026) so reads that test `= 'clear'`
  exclude it. Audit every read of `moderation_state` in SQL and Go for a place that would treat
  `'review'` wrongly; in particular the text check retry worker must never pick it up or clear it,
  and nothing that tests `<> 'removed'` or `= 'pending'` may let it through.
- The time a report lasts keeps running while it is hidden. Nothing is extended.
- A report hidden for review sends no push of any kind, and a kept report is not pushed again.
- A review hide never counts against the poster's outcome score, stars or reporter tag. After
  Keep the report counts as if it was never hidden; after Remove it counts the way a removal
  counts today.

### 3. Only a moderator ends it

- The moderator screen (phone and laptop) lists reports hidden for review first, marked **Hidden
  by flags**, with the usual reasons, notes, counts, Keep and Remove. No flagger identity and no
  cell is shown, as now.
- **Keep** puts it back to clear, dismisses its open flags, and marks the report (for example a
  `review_kept_at` column in 00026) so flags can never hide it again. Later flags still reach the
  moderator.
- **Remove** works as today.

### 4. The bell tells the poster

Written straight into the poster's own updates, one row per event, whatever their follows or
mutes. Bell only: never a phone notification, never an email. They show in the bell list and in
the full updates list (`/profile?part=updates`) on phone and laptop, unread like other rows.

| Event | The row says | Tapping it opens |
|---|---|---|
| A held report (text check not finished at posting) becomes shown: pending to clear, by the late check or a moderator Keep | Your report passed the text check and is now showing. | The report (`/?report=<id>`) |
| Hidden for review by flags | Your report is hidden while it's reviewed. | Nothing (a plain row) |
| A moderator keeps it after a review hide | Your report is showing again. Sorry for the wait. | The report |
| Removed: a moderator's Remove from any state, or the late text check | Your report was removed after review. | The Policy page (`/policy`) |

- No row while a report waits for the text check: the message at posting already says so.
- A row about the poster's own withheld report must not turn into the "no longer available"
  wording, and must not show the report's words. Work out the row link and plain logic in
  `fillUpdatesPart` (internal/api/handlers/activity.go) and the bell's own rendering; touch
  nothing else in activity.go.
- The existing follow path (`DecideUpdate`) writes nothing for a withheld report and stays so.

### 5. Copy and docs

- Policy (web/templates/legal_body.html.tmpl), in the moderation part: one or two short plain
  sentences that say a report flagged by people in three different places is hidden until a
  moderator keeps or removes it, and that its author is told in the bell when it is hidden, shown
  again or removed. Replace the sentence that says the author "sees Removed after review" with
  the bell. Change nothing about Ended (quick 261117-vuf edits that line).
- docs/MODERATION.md: amend the "A flag never hides" rule with a dated note and describe the
  rule, the place cells, the Keep mark, the moderator list and the bell rows.
- docs/TRUST-MODEL.md: the gaming analysis in plain words. One person with three accounts and
  three faked positions can hide a report until a moderator looks; what limits it (one flag per
  account, the flag allowance, an account needs an emailed code, Keep blocks a second hide, so at
  most one hide per report); the remaining risk that a true report stays hidden while the only
  moderator is away, accepted by the owner on 2026-11-17. docs/PUSH.md if push behaviour text
  needs it.
- The Flag window's own words do not change, unless they promise that a flag never hides.

## Tests

- Service: 2 cells do not hide; 3 accounts in one cell do not hide; 3 accounts in 3 cells hide;
  flags with no cell never count; a dismissed flag does not count; concurrent flags hide once;
  after Keep no later flag hides; pending and removed never become review; the text check worker
  never touches review.
- Fail closed: every public read excludes a review report (extend the existing withheld guards,
  such as the one behind TestWithheldInNoView, rather than writing a second rule).
- Moderator Keep and Remove from review, and pending to clear, write the right bell row exactly
  once for the poster; no push on hide or Keep; the outcome score is unaffected by a review hide.
- Migration 00026 up and down, in the repo's migration tests.
- The flag route accepts an optional position and refuses a bad one; the cell is in no JSON.
- The updates rows: the plain row, the Policy link, no "no longer available" for these kinds,
  in the bell and the full list.
- Node tests for the Flag dialog sending the position when known and none when not.
- Contract tests for the new copy and the Policy lines.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your
  own scratch database `p9_vvb` (createdb, migrate the way the repo's tests do, drop it after).
  Run migrations only on your own scratch databases.
- Live look: your own server on port 8793 with the scratch database, the log mailer and
  `MODERATOR_EMAILS` set to a made up moderator, made up example.com accounts only. Post a report
  as A; flag it from B, C and D at three different positions; A's report is gone from Nearby for
  everyone and A's bell shows the hidden row; the moderator keeps it; A's bell shows the showing
  again row and a fresh flag does not hide it; remove another report and A's bell shows the
  removed row with the Policy link. Look at the bell rows with Playwright WebKit
  (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules) at 390 wide and at
  1280, light and dark. Screenshots only in your own scratchpad subfolder, never in the repo.

## Rules

- Fix only this. Do not touch the My activity page's parts or hub, Settings, the profile page
  (person.html.tmpl, person_page.go), the Demo, or the Policy's Ended line: quick 261117-vuf works
  on those at the same time. In internal/api/handlers/activity.go touch only `fillUpdatesPart`
  and helpers only it uses. Do not delete moderation copy constants the activity page uses.
- Comments keep "a flag never hides a comment".
- No em or en dashes in code comments, copy or commits. No Python at all.
- No repo file or commit may mention portfolio, recruiters, interviews, job applications or
  hiring.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797. Never touch the databases
  pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view, and never run a
  migration on any database but your own scratch ones.
- Write 261117-vvb-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
