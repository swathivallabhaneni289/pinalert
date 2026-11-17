---
quick_id: 261117-vuf
slug: my-activity-becomes-one-settings-row-wit
date: 2026-11-17
type: quick
---

# My activity becomes one Settings row; its page keeps Confirmed, Disputed, Ended

## Why

7.1 UAT test 7, 2026-11-17, the owner on the iPhone simulator, looking at the Settings My
activity section quick 261117-lq0 built:
- "This is the point of the updates. I don't think we need them."
- "posted by you, you confirm, you disputed, and ended nearby. I think it all should just maybe
  come under my activity. When you click that my activity ... these all should be like be shown
  in that next page"
- "can we like shorten those names, like maybe one word if that's possible"
- "posted by you ... you'll already see whatever you posted on the profile", then "You'll see the
  post twice, right?"
- Agreed table, then "yes build it", and "Knowing these updates in both laptop version and phone
  versions" (phone and laptop the same).

What the owner was told and agreed:

| Where | What you'll see |
|---|---|
| Settings | A single **My activity** row |
| My activity page | **Confirmed, Disputed, Ended** |
| Profile | Your posts. Posted is gone. |
| Bell | Your updates as now, and "See all updates" still opens the full list |

A parallel task, quick 261117-vvb, moves the held, removed and review news to the bell and adds
the flag rule. Stay out of its files (see Rules).

## What changes

### 1. Settings (web/templates/settings.html.tmpl and the Settings handler)

- The section `settings-activity-section` stays first. Its title becomes **Activity** and its
  box holds one link row, label **My activity**, `href="/profile"`, with the right chevron, built
  like the other link rows (`place-row`, `place-row__text`, `place-row__label`). Both sizes.
- Remove `ActivityLinks` from the Settings view model and the builder that made the five rows
  (`activityLinks`, `activityPartFragment` and anything else that only served them) when nothing
  else uses them. Update the dated comment above the section in the file's style.

### 2. The My activity page (/profile: activity.go, the profile handler in auth.go, profile.html.tmpl, activity.js, activity.css)

- The hub and the laptop's stacked parts show exactly three parts, in this order, with one word
  names: **Confirmed** (key `confirmed`), **Disputed** (key `disputed`), **Ended** (key `ended`).
  The phone top bar title of a part screen uses the same name. Summary lines, part lines, counts,
  empty states and the Ended loading, location and error states stay as they are.
- **Posted leaves.** The `posted` part, its cards (cards of the post kind are no longer built or
  listed here), the reporter record card, and the page's Reopen and "both undos used" controls
  that only Posted cards carried (activity.js) go, with only the code that served them. Keep the
  shared builders other pages use (votes.js `createReopenControl` serves the report's own take
  back block). `?part=posted` and the old aliases that landed on Posted now land on the list.
- **Updates leaves the list.** It is not a hub row and not drawn in the laptop's stack of parts.
  `/profile?part=updates` (the bell's "See all updates" and the Updates "Try again" link) still
  works on both sizes and shows only the Updates screen: the title **Updates**, the rows, new
  count, cut line and error state exactly as now. On that screen the phone back arrow goes to
  `/` (Nearby, where the bell is). On the laptop that address shows the Updates part alone, not
  the three parts. Do not change `fillUpdatesPart`'s body: quick 261117-vvb owns the row link
  logic inside it.
- The list and the three part screens keep the back arrow to `/settings` and no bar item current.
- Nothing else on the page changes (heading, queue strip, card layout, the star box host).

### 3. The Demo (demo_activity.html.tmpl and its builder)

The Demo's My activity mirrors the real page: Confirmed, Disputed, Ended with the same names, and
no Posted. If any Demo copy points people to Posted (for example after a practice report), change
it to name where that report shows now in the Demo. The Demo bar is unchanged.

### 4. Copy that names the old part names

- Policy (web/templates/legal_body.html.tmpl, about line 60): "Ended reports stay under My
  activity, Ended nearby, for 7 days." becomes "Ended reports stay under Ended in My activity for
  7 days." Change nothing else in the Policy (quick 261117-vvb edits other Policy lines).
- Grep templates, scripts, Go copy and docs (docs/*.md) for on screen or user facing text that
  names "Posted by you", "You confirmed", "You disputed", "Ended nearby" as a part, or tells people
  to find their posts or updates in My activity, and update it the same way. Code comments that
  describe the old layout may get a dated note in the file's style.

## Tests

- Update every Go contract test and node test that pins the five parts, their names and order,
  the Settings rows, ActivityLinks, the Posted record card, the activity page Reopen, or the
  Policy sentence. Keep each test's intent; add an "Amended 2026-11-17 (quick 261117-vuf)" note
  where the file uses that style.
- New pins: Settings' first section is titled Activity and holds one row My activity to /profile;
  the hub lists exactly confirmed, disputed, ended in that order with the one word names; the
  laptop page has no posted or updates section on /profile; `?part=posted` lands on the list;
  `?part=updates` shows only Updates with the title Updates and the back arrow to `/`; the parts
  keep the back arrow to /settings; the Demo parts.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your
  own scratch database `p9_vuf` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: Playwright WebKit (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules)
  against your own server on port 8794 with the scratch database and the log mailer, a made up
  example.com account with a username and a few confirms, disputes and ended reports nearby,
  phone at 320 and 390 wide and laptop at 1280, light and dark:
  - Settings opens with Activity first and its one My activity row looks like the other link
    rows (height, chevron, divider, the laptop's left title column);
  - the row opens the list; each of the three parts opens and Back returns to the list's
    Back target as specified; the laptop shows the three parts stacked;
  - the bell's See all updates opens Updates alone, and Back on the phone goes to Nearby;
  - nothing scrolls sideways.
  Screenshots only in your own scratchpad subfolder, never in the repo.

## Rules

- Fix only this. Do not touch internal/service, the updates service or its JSON, flags, the
  moderator screen, the bell markup or script, the profile page (person.html.tmpl,
  person_page.go), migrations, docs/MODERATION.md, or any Policy line other than the one above:
  quick 261117-vvb works on those at the same time.
- No em or en dashes in code comments, copy or commits. No Python at all.
- No repo file or commit may mention portfolio, recruiters, interviews, job applications or
  hiring.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797. Never touch the databases
  pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view.
- Write 261117-vuf-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
