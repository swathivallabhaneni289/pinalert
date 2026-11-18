---
quick_id: 261118-uoe
slug: the-review-ask-box-buttons-fit-on-one-li
date: 2026-11-18
type: quick
---

# The review ask box buttons fit on one line, and account deletion blanks review ask notes

Two follow ups to quick 261118-rz6, found in its live look and review. Runs in parallel with quick
261118-saj (the Critical mark: moderation, flags, push, report cards, the moderator page). Stay out
of its area.

## Why

- The owner asked on 2026-11-18 that the short labels in these boxes are "not just clumped up
  together". The rz6 live look found that at 320 wide the poster's box wraps **Ask for review** onto
  two lines inside its button (scratchpad rz6/shots/step2-box-320-light.png), which reads cramped.
- docs/MODERATION.md says deleting an account blanks its comments and removes its likes and flags,
  but an account's review asks keep the person's free text note after deletion (account deletion is
  a soft scrub, so the asks table's ON DELETE CASCADE never fires). A person's own written words
  should go with the account the same way their comments do.

## What changes

### 1. The poster's box buttons (web/static/css/updates.css, the review ask box only)

- **Close** and **Ask for review** each stay on one line at every width from 320 up, on phone and
  laptop, light and dark, with the same gap between them as the Log out box. Size the two buttons
  to their labels (or let the action button take the extra room) instead of forcing equal halves.
  If they still cannot fit side by side at 320, stack them full width, the action on top, as one
  rule for this box. Do not change the words or any other box.

### 2. Account deletion (internal/service/accountdelete.go and its query)

- In the same transaction as the rest of the scrub, blank the note on every review ask the deleted
  account made (set it to NULL). Keep the ask rows themselves (report, time, answer, answering
  moderator), so the moderator history and the 3 ask cap stay correct. An open ask stays open
  without its note.
- docs/MODERATION.md Retention: say deleting an account blanks the notes on its asks for review and
  keeps the rest. Policy: no change (it makes no promise about these notes).

## Tests

- The deletion test (in the style of the existing account deletion tests) proves the note is NULL
  after deletion, the ask row and its answer remain, and other accounts' notes are untouched.
- A node or contract check pins the button rule for this box only.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your own
  scratch database `p9_uoe` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: your own server on port 8794 with a scratch database, the log mailer and
  `MODERATOR_EMAILS` set to a made up moderator, made up example.com accounts only. Remove a
  report, open the poster's box from the bell, Playwright WebKit
  (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules) at 320, 360, 390
  (touch) and 1280, light and dark: both labels on one line, the box not crowded, nothing scrolls
  sideways. Screenshots only in your own scratchpad subfolder, never in the repo. Do not drive the
  iPhone simulator.

## Rules

- Fix only this. Do not touch moderation.sql, flag code, push code, the moderator page, report
  cards, visibility or migrations (quick 261118-saj works there now). No new migration.
- No em or en dashes in code comments, copy or commits. No Python at all.
- Keep all wording neutral, as the owner's wording rule asks.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md. Do not edit UAT files.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782, 8793, 8795, 8796, 8797, 8798 or 8799. Never
  touch the databases pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat, p9_view or any
  p9_saj or p9_gate database, and never run a migration on any database but your own scratch ones.
- Write 261118-uoe-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
