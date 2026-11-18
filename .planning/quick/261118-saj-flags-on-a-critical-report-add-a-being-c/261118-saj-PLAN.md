---
quick_id: 261118-saj
slug: flags-on-a-critical-report-add-a-being-c
date: 2026-11-18
type: quick
---

# Flags on a Critical report add a "Being checked" mark and pause its alerts, instead of hiding it

Runs after quick 261118-rz6 (removal reasons, review asks, Restore) is merged; build on top of it.

## Why

7.1 UAT, 2026-11-18. The review of quick 261117-vvb found that flags from 3 places can hide a
Critical report (severity critical or category rescue needed, `ReportMeta.Serious()`), which
disputes never can (visibility.go, the critical bypass), and a hidden one never alerts people who
arrive later. Told the owner. The owner: "what if it's just a fake news and it needs my attention,
and it would be better if people like maybe not see it?" Proposed: a Critical report stays visible
with a mark, its phone alerts stop until a moderator looks, it goes to the top of the moderator
list; Keep clears the mark and alerts work normally again; Remove works as today. The owner:
"Yeah, and make sure like in that box all these short labels are not just clumped up together. And
being checked, I think, should be like, when you hover over like the flags or something, that's
when like it should show that it's being under checked." Told: phones cannot hover, so a tap on the
mark shows the same words; disputes never move a Critical report, so a fake Critical report only
goes when a moderator removes it. Owner: "yes".

## What changes

### 1. The rule

- When a **Serious** report that is shown (`moderation_state = 'clear'`) reaches the vvb threshold
  (open flags from `FlagHideThreshold` = 3 different accounts in 3 different place cells, flags
  with no cell never count, decided in the same transaction and under the same row lock as the
  flag insert), it is **not** hidden. Instead it gets the **checking mark** (store it, for example
  `reports.flag_check_at`, migration `00028` or the next free number after rz6's).
- Non serious reports keep vvb's hide exactly as built.
- A report a moderator already kept or restored after flags never gets the mark or a hide again
  (extend the existing `review_kept_at` rule; one settled marker for both).
- A serious report that is still held (`pending`) gets the mark at the moment vvb would have hidden
  a non serious one, if the threshold is met then.
- Migration: any serious report already in `review` moves to `clear` with the mark set, so nothing
  Critical stays hidden by flags. Down: marked reports stay `clear` and the column goes.

### 2. What readers see (phone and laptop, everywhere the report's Critical tag shows)

- A small **flag mark** (an icon button) next to the report's Critical or rescue tag, on every
  surface that draws that tag: Nearby rows, the map pin popup, the report page and focus, profile
  lists, search, followed lists, Past if shown there. Add a `flag` icon to web/static/icons in the
  same style as the set (an SVG code file like the others).
- **Hover** on the laptop and **tap** on phone and laptop show one short line: **Being checked.**
  No other words, no counts, no reasons, no flagger anything. It closes on a second tap, on Escape
  and on a tap elsewhere, and never covers the report's own buttons. Keyboard: focusable, Enter or
  Space shows it, the words are announced (for example `aria-describedby` or a live tooltip),
  accessible name **Being checked**.
- Spacing: the mark and its line have clear room around them; nothing is crowded against the tag
  or the report's other labels (owner: "not just clumped up together").
- No change to the report's text, votes, place in the list, or Critical alert banner other than
  the alerts pause below.

### 3. Alerts pause

- While the mark is on, the report sends **no phone notification of any kind** (Critical first
  alerts, follow alerts, changed notes): checked when queued and again at send time, like vvb's
  gates.
- After a moderator **Keep**: the mark goes, the report is settled (flags never mark or hide it
  again), and from then on it alerts like any shown Critical report would. Nobody gets the same
  alert twice. This differs from vvb's rule for a report that was hidden and kept (that one stays
  silent); keep that rule as is for hidden reports and for rz6's Restore.
- After **Remove**: as today (with a reason, rz6), the mark goes with it.

### 4. The moderator page

- Marked reports are listed first in the flagged list, together with "Hidden by flags", each with
  a short tag **Still showing** (hidden ones keep their tag). Same buttons as now (Keep, Remove with
  its reason box). Update the section's one line if it no longer matches the order, in few words.
- Keep and Remove take the same row lock as vvb; a repeat answers 409.

### 5. The poster

- No new bell row. A marked report was never hidden, so Keep writes no "showing again" row for it.
  Remove writes the rz6 removed row as for any removal.
- The rating never moves because of the mark.

### 6. Copy and docs

- Policy (web/templates/legal_body.html.tmpl), moderation part: amend the vvb sentence in a few
  words so it says a Critical report flagged this way stays visible, marked as being checked, and
  sends no alerts until a moderator looks. Touch no other Policy line.
- docs/MODERATION.md and docs/TRUST-MODEL.md (the vvb gaming analysis and its Honest limits
  line): three accounts can no longer hide a Critical report; they can pause its alerts until a
  moderator looks, and put a mark on it; one settled marker per report. docs/PUSH.md for the pause.
- Swagger annotations if any JSON gains a field. The checking mark may be exposed to clients only
  as a boolean; never flag counts, cells or flaggers.

## Tests

- Service: 3 places on a serious report set the mark and do not hide; on a non serious report the
  hide is unchanged; a settled report never gets the mark; concurrent flags set it once; a pending
  serious report gets it when the threshold is met; the migration moves a serious `review` report
  to `clear` with the mark.
- Push: nothing queued or sent while marked (each kind); after Keep the report alerts normally and
  nobody gets a repeat; hidden and kept, and restored, reports stay silent as before.
- Visibility: a marked report is Live everywhere a clear Critical report is; extend the existing
  guards rather than writing a second rule.
- Moderator list order and the Still showing tag; Keep and Remove from the mark, 409 on a repeat.
- JSON carries only a boolean for the mark; no counts or cells (extend the existing no-leak tests).
- Node tests for the mark: hover and tap show "Being checked.", Escape and outside tap close it,
  keyboard works.
- Contract tests for the new strings and the Policy sentence; no em or en dashes.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your own
  scratch database `p9_saj` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: your own server on port 8793 with a scratch database, the log mailer and
  `MODERATOR_EMAILS` set to a made up moderator, made up example.com accounts only. Post a Critical
  report as A and an ordinary one; flag both from B, C, D at three positions; the ordinary one hides,
  the Critical one stays on Nearby and the map with the mark; hover and tap show "Being checked.";
  the moderator sees it first with Still showing; Keep clears the mark and a fresh flag set does not
  bring it back. Playwright WebKit
  (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules) at 320 and 390 wide
  (touch) and 1280 (hover), light and dark: the mark and its line are not crowded, are not clipped
  inside the popup or a row, and nothing scrolls sideways. Do not drive the iPhone simulator.
  Screenshots only in your own scratchpad subfolder, never in the repo.

## Rules

- Fix only this. Do not change the threshold, the non serious hide, disputes, comment moderation,
  rz6's asks and Restore, My activity, Settings or any Policy line other than the one above.
- No em or en dashes in code comments, copy or commits. No Python at all.
- No repo file or commit may mention portfolio, recruiters, interviews, job applications or
  hiring.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md. Do not edit UAT files.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797. Never touch the databases
  pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view, and never run a
  migration on any database but your own scratch ones.
- Write 261118-saj-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
