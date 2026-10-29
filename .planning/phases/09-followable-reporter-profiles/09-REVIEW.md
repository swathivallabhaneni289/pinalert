---
phase: 09
reviewed_commit: e689824a
base_commit: 6f13da00
parts: [service, edge, browser, templates]
status: issues_found
findings:
  critical: 0
  warning: 19
  info: 23
  total: 42
---

# Phase 9 code review: index and triage

Four read only reviewers (Opus) read the Phase 9 changes on main, from the end of Phase 8 (6f13da00) to e689824a, in parallel. The wording cleanup commits named 261029-rtx were left out. The final build gate at e689824a is green.

| Part | Critical | Warning | Info | File |
|------|----------|---------|------|------|
| Service (internal/service) | 0 | 5 | 5 | 09-REVIEW-service.md |
| Edge (handlers, router, store, migrations, cmd) | 0 | 4 | 7 | 09-REVIEW-edge.md |
| Browser (web/static/js, web/*.mjs) | 0 | 6 | 5 | 09-REVIEW-browser.md |
| Templates (templates, styles, contract tests, docs, Policy) | 0 | 4 | 6 | 09-REVIEW-templates.md |

The rating arithmetic, the cell counting, the single 70 cut, the uniform not found answers, the access gate on every route, the migrations and the ON CONFLICT predicates were all traced and hold.

## Duplicates merged

- WR-S02 and IN-E03 (a failed moderator removal never restores its follows): one finding, server fixer.
- IN-S01 and IN-E01 (a Follow racing a Block): one finding, server fixer.
- K5 (handlers/updates.go) and IN-T04 (person_page.go and person.html.tmpl): the same double reading class in two places. K5 goes to the server fixer, IN-T04 to the templates fixer.

## Known items K1 to K6

- K1 (moderators see a rater's pending username): owner question, see below.
- K2 (no star box for the reader whose resolve closes the report): a UI-SPEC deviation, Warning, browser fixer.
- K3 (a confirmed update whose first follow push row was pruned skips the follow recheck): Warning, server fixer, together with WR-S03 and K6.
- K4 (stale FollowFanoutCap comment): fix now, server fixer.
- K5 (follow row accessible name says the category twice): fix now, server fixer.
- K6 (a comment push queued just before a block can still go out once): Warning, server fixer.

## Triage

Fix round 1 runs three fixers in worktrees off the commit that adds these files. Each fixer owns a fixed set of files and stops and reports if it needs a file it does not own.

**Server fixer.** Owns internal/, cmd/, internal/api/router.go, every handler except internal/api/handlers/person_page.go, the migrations (00025 only if a fix truly needs one), docs/docs.go and the swagger files, and the lines of docs/PUSH.md and docs/TRUST-MODEL.md that describe the behaviour its fixes change.
- WR-S01: the restore window starts at the removed rating's created_at.
- WR-S02 (with IN-E03): a moderator removal re-evaluates and restores even when a retry finds nothing left to remove.
- WR-S03, K3 and K6, one change to the send time rechecks. Target behaviour:
  - a queued Reliable reporter push and its confirmed update are not sent once the reader muted or blocked the reporter, either way;
  - a queued comment push is not sent once either side blocked the other (CommentPushStillHolds);
  - a confirmed update whose first push row is gone is skipped as not_eligible, never re-based on another kind.
- WR-S04: switching the profile on can never publish a Name that was not screened.
- WR-S05: the rating offer (MayRate, the profile and the resolve rate prompt) is false for a rater whose rating of that reporter a moderator removed, or who is barred from rating.
- WR-E01: GET /u/{username} goes through the PeopleRead bucket and answers 429 with a page, the way the 08-04 public status page does, never JSON. Done in router.go and the rate gate only.
- WR-E02: the visibility switch cannot drive repeated screener calls (screen only a Name not yet screened, or a tight bucket on the switch).
- WR-E03: the moderator ratings handlers check moderator rights themselves.
- K4, K5, IN-E05 (gofmt), IN-E06 (account deletion removes queued follow pushes about the deleted person; the two stale comments), IN-E07 (the blocking viewer case in the uniform 404 tests), IN-T01 (a finished Rescue report on a profile at precision 5, and TRUST-MODEL.md:685).
- Fix where cheap and safe: IN-S01 with IN-E01, IN-E02, IN-E04. Record the rest with a reason: IN-S02, IN-S03, IN-S04, IN-S05.

**Browser fixer.** Owns web/static/js and web/*.mjs. It may change a web/*_test.go contract test only where its own script change requires it, with a small local edit and a dated note.
- WR-B01 to WR-B06.
- K2: mount the star box under the Marked resolved message and its Undo in the host that survives the card closing, if one exists; otherwise record why.
- IN-B01 to IN-B05 where cheap.

**Templates fixer.** Owns web/templates, web/static/css, the shared web contract tests, internal/api/handlers/person_page.go, docs/MODERATION.md and the Policy (web/templates/legal_body.html.tmpl). It does not edit docs/PUSH.md or docs/TRUST-MODEL.md.
- WR-T01: forced-color-adjust on the Phase 9 mask glyphs, and add them to TestForcedColoursShowsTheMaskGlyphs.
- WR-T02: contrast of the laptop Recent reports column heads and the hovered row line.
- WR-T03 and WR-T04: the Policy names the Name screening by OpenAI and every item Phase 9 stores (ended follows with their reason, cleared ratings, the permanent rating bar, the switch time, the home area on a profile). Correct the MODERATION.md claim.
- IN-T02, IN-T03, IN-T04, IN-T05, IN-T06.

**Accepted:** the Policy date and CurrentTermsVersion stay 2026-10-29; the owner moves both together on deploy day (README step 9), which covers the Phase 9 lines.

**Owner questions** (for the end check list, in plain words):
- K1: moderators see a rater's username even while the name check is still running, because they act by name. Recommendation: keep it, moderators only.
- WR-E04: you can block someone only from their public profile, and most people keep their profile off, so you cannot block them to hide their comments. UI-SPEC kept commenter names unlinked on purpose. Recommendation: accept for now; a later small change could put Block beside Flag on a comment.

After the fixes: merge, the gate, a short read only skeptic that also checks the docs against the merged code, then the verifier. The phase stays In Progress until the owner says yes.
