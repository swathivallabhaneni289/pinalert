---
phase: 08-report-page-and-comments
plan: 20
subsystem: phase-gate
tags: [gate, validation, phase-9-contract, owner-check-list]
requires:
  - phase: 08-report-page-and-comments
    provides: plans 08-01 to 08-19 merged (main at 70d291a1)
provides:
  - the full gate result on a fresh scratch database
  - proof of A-P8-1 to A-P8-10
  - the owner's human check list for Phase 8
affects: [phase-09-followable-reporter-profiles]
tech-stack:
  added: []
  patterns: []
key-files:
  created: [.planning/phases/08-report-page-and-comments/08-20-SUMMARY.md]
  modified: []
key-decisions:
  - "Docker was not running and was not started, so the Linux check is owed at push time. The full Go suite was run with TZ=UTC instead."
  - "The owner's viewing session was not run (it is not the executor's to run). It is written below as the human check list."
requirements-completed: []
duration: about 25 min
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 20: The phase gate and the owner's check list Summary

**All A-P8 assumptions hold on the merged tree; Go, vet and node suites are green on a fresh scratch database; the Linux run and the owner's viewing are still owed.**

## First lines: what is false or owed

- A-P8-1 to A-P8-10: none is false. Every one holds (table below).
- Owed 1: the Linux CI check. `docker info` failed, Docker Desktop was not started. Run the owner's Linux recipe before the push.
- Owed 2: the owner's viewing session and the three real device checks (check list below).
- One note on VALIDATION V22: its first command runs `TestCommentCountExtras` in `./internal/service`, but that test lives in `./internal/store`. In `./internal/service` the command prints "no tests to run" and exits 0. Run in `./internal/store` it passes. VALIDATION.md was not edited (rule 5); fix the path at the next planning pass.

## Gate results (macOS, Go 1.26.4, fresh database p8_20_a, TZ=UTC)

| Step | Result |
|------|--------|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `node --test web/*.mjs` | 1422 tests, 1422 pass, 0 fail, 0 skipped |
| `TZ=UTC go test ./... -p 1 -count=1 -v` with DATABASE_URL on p8_20_a | exit 0; every package `ok`; 2168 top level `--- PASS`; 0 `--- FAIL` |
| Skips in the full run | only TestGeocodeLive, TestOfficialLive, TestScreeningFixture (the three allowed) |
| Linux CI run | NOT RUN (Docker down). Owed at push time. |
| Databases | p8_20_a created (it did not exist), dropped at the end; count after is 0. p8_20_view never created (the viewing is not mine to run). `git status --short` is clean. |

DATABASE_URL and PGDATABASE were empty before the run. Only p8_20_a was touched.

## V1 to V26

Each command of 08-VALIDATION.md was run by name with `-v` (DB commands on p8_20_a). Every one exits 0, prints its named tests as PASS, and shows no `--- SKIP` and no `--- FAIL`.

| V | Result | Tests passing |
|---|--------|---------------|
| V1 | green | node 31, Go 5 |
| V2 | green | 3 |
| V3 | green | 4 |
| V4 | green | 4 |
| V5 | green | TestAccessGateExemptRoutes |
| V6 | green | TestSharePageSetsNoCookie |
| V7 | green | TestShareOpenGraph |
| V8 | green | TestSharePageStates |
| V9 | green | node 14 |
| V10 | green | service 35, handlers TestCommentScreening |
| V11 | green | 2 rate limit tests, then api and cmd/server 47 |
| V12 | green | TestNoMarkupSinksAnywhere, node 37 |
| V13 | green | 4 |
| V14 | green | TestCommentsClosed |
| V15 | green | 2 |
| V16 | green | node 10 |
| V17 | green | 10 |
| V18 | green | 2 |
| V19 | green | 4 |
| V20 | green | 3 |
| V21 | green | 2 |
| V22 | green, with the path note above | TestCommentCountExtras (internal/store), node 12, TestPopupFooter |
| V23 | green | web short run 636, node flag dialog 13 |
| V24 | green | TestSwaggerSpecCoversRoutes |
| V25 | green | TestDeleteAccountComments |
| V26 | green | TestTrustModelDocMatchesCode |

## Phase 9 contract: A-P8-1 to A-P8-10

| Item | Holds | Proof output |
|------|-------|--------------|
| A-P8-1 | yes | `grep -c 'function noteReportOpened' web/static/js/map.js` printed 1; `node --test web/report_view_test.mjs` passed 21 of 21 |
| A-P8-2 | yes | `00018_add_comments.sql` holds CREATE TABLE comments; `name: CommentsForReport` count 1; `comment_count` in reports.sql count 3; `grep -rln comment_count internal/store/queries/` lists reports.sql and comments.sql |
| A-P8-3 | yes | the last file listing `updates_kind_valid` is `00019_add_comment_update_kinds.sql` |
| A-P8-4 | yes | router.go fill count 3; flag_e2e_test.go `Comment: p20Open`, `Like: p20Open`, `Share: p20Open` each count 1 |
| A-P8-5 | yes | `function resolveToastAction` in votes.js count 1; no commit with a `(08-` scope touches votes.js or trust.js |
| A-P8-6 | yes | `BlankAccountComments` in accountdelete.go count 2 |
| A-P8-7 | yes | the three trust.js functions count 3 |
| A-P8-8 | yes | `"/r/{token}"` in router.go count 1; TestAccessGateExemptRoutes passes |
| A-P8-9 | yes | `'comment_on_yours', 'reply_to_yours'` in push.sql count 1 |
| A-P8-10 | yes | TestCommentsForReportVisibility (internal/store) and TestCommentListNames (internal/service) pass |

## Success criteria SC1 to SC5

All five hold by the automated evidence above (V1 to V9, V10 to V22). What only a person can judge is in the check list.

## Deviations from Plan

1. The Linux CI run was replaced by a second full Go run with TZ=UTC, because Docker was not running (project rule: do not start it). The Linux check is owed.
2. Task 2 (the owner's viewing session) was not run. The project rule says it is not the executor's to run and that no server is started. Its items are the human check list below, so no answer for D-18 or the other defaults was recorded. All of them stay "owner has not answered yet", so the built defaults stand.
3. Found while running V9: the Share window never calls `navigator.share`. The wa.me chooser link is used, as UI-SPEC set it (08-06 recorded this). VALIDATION V9 still says "from the second tap". Worth a wording fix later.

No source file was changed by this plan. No auth gate.

## Owner check list (the viewing, to run on a scratch copy)

Phone checks use the iPhone simulator (never the iPad). Laptop checks use Mac Safari. Show each, one at a time, with a recommendation.

Answers to fill in later: D-18 and each default in UI-SPEC "Defaults to confirm at the viewing session":
1. D-18, the laptop host. Default A: the bubble stays, the Open report row opens the panel with Back. Option B: a pin tap opens the report at once. Recommendation: keep A (the owner said the laptop looks good as it is).
2. Phone order kept as shipped (deviation from D-01).
3. The comment count shows in the Comments heading and the Open report row only.
4. The public page says "removed" only for a moderator removal.
5. "Sign in to see more" is a plain link to /login.
6. Anyone signed in may comment, from anywhere.
7. No report page, comments, timeline or Share in the Low data view.
8. "Comment removed" for a moderator removal, "Comment deleted" for the author's own.
9. A Nearby row tap on the laptop opens the bubble (B would open the panel).

What to look at:

1. Phone report page (iPhone simulator, 320, 360 and 390, light and dark, rotate across 900px): the history ("What happened so far"), Share, comments, thumbs up likes, replies with @username, Delete, Flag, closed comments, a removed and a deleted comment, a held comment, and the long Settings switch label wrapping (08-15 flagged it). Nothing should jump on a poll.
2. Laptop (Mac Safari, 1280 and 1440): tap a pin, Open report, switch pins, Back, a Nearby row tap. Type in the comment box with arrow keys, plus, minus and Escape; the map must not move.
3. The share window, then the public `/r/{token}` page with scripts blocked; it needs no sign in, shows no count and no verified claim. The WhatsApp link preview needs a public https copy (or a tunnel the owner trusts); wait up to 10 seconds.
4. The moderator Comments section: Keep and Remove.
5. The comment phone note on the owner's own phone after Phase 3.3 is live, with Changes on: another account replies. The bell shows "New reply to your comment". The note is silent and one per report per device.

Things the build executors asked the owner to look at:
- The laptop sticky comment box (08-16).
- The phone box under the soft keyboard (08-16).
- Remove comment has no confirm step (08-18); `buildRemoveConfirm` is the pattern if the owner wants one.
- After a tap on empty map, focus stays on the map (08-07).
- A pin near the left edge may leave view when the panel opens (08-07).
- The comment trust line on a disputed report.
- Escape in the comment box closes an open trust panel first.

## Owner steps still owed outside this plan

- Migrations 00017 (share token), 00018 (comments), 00019 (comment update kinds) are new, and 00012 to 00016 are also still owed. The owner runs `make migrate` on the dev database only after saying yes.
- The Policy date and CurrentTermsVersion stay 2026-10-29 until deploy day (08-19).

## Self-Check: PASSED

Checked: the SUMMARY file exists in the worktree; p8_20_a is dropped (count 0); `git status --short` was clean before this file was written; no dash characters of the long kind were written.
