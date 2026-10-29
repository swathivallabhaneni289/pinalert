---
phase: 09-followable-reporter-profiles
plan: 15
subsystem: moderation
tags: [ratings, stars, moderator, routes, dialog, go, vanilla-js]
requires:
  - phase: 09-13
    provides: RatingService.ModeratorList, ModeratorRemove, ModeratorRemoveRater, ErrPersonNotFound
  - phase: 09-14
    provides: Deps.Ratings, the ratings source wired into every tag reader
provides:
  - GET /api/moderation/people/{username}/ratings
  - POST /api/moderation/people/{username}/ratings/remove
  - POST /api/moderation/raters/{username}/remove-ratings
  - the Ratings section and the Remove all box of the moderator screen
affects: [09-27, 09-29]
tech-stack:
  added: []
  patterns:
    - "moderator routes registered with r.Handle inside the requireModerator group, the group is the only rights check"
    - "a native dialog.logout-confirm inside the page root so the page's one delegated click handler reaches its buttons"
key-files:
  created:
    - internal/api/handlers/moderation_ratings.go
    - internal/api/handlers/moderator_ratings_e2e_test.go
    - web/moderate_ratings_contract_test.go
    - web/moderate_ratings_test.mjs
  modified:
    - internal/api/router.go
    - web/templates/moderate.html.tmpl
    - web/static/js/moderate.js
    - web/static/css/moderate.css
    - web/moderate_contract_test.go
    - web/phase4_gates_contract_test.go
key-decisions:
  - "The single removal answers the reporter's refreshed list, so the screen redraws from what is stored."
  - "A rater with an unscreened (pending) username is still named on the moderator screen, a moderator only exception."
  - "The Remove all box sits inside main, not as a direct child of body."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 15: The moderator ratings tool Summary

A moderator can now type a reporter's username, read who gave how many stars and when, remove one rating, or remove everything one rater gave behind a box that asks first. The reporter's stars ignore the removed ratings on the next read, and nobody who is not a moderator can reach any of it.

## What was built

- **Routes (task 1).** `internal/api/handlers/moderation_ratings.go` holds `ModerationRatingsList`, `ModerationRatingRemove` and `ModerationRaterRemoveAll`, registered with `r.Handle` inside the existing `requireModerator` group in `router.go`. The `RatingService` moderator methods check no rights, so the group is the only check; each handler also starts with `modGuard` (method, the moderator's account from the context, `no-store`). No swag blocks, like the other moderator routes. Errors: `ErrPersonNotFound` is 404 on field `username` "No such person."; a malformed or extra key body is 400 on field `rater`; an unexpected error is a logged 500.
- **Screen (task 2).** The Ratings section (heading, line, field, Show ratings, `#mod-ratings-error`, `#mod-ratings-status`, `#mod-ratings-list`) and the Remove all dialog are in `moderate.html.tmpl`. `moderate.js` fetches the list through the existing `apiRequest` (which gained an optional JSON payload), draws rows with `Pinalert.setText` only, gives Remove and Remove all by {rater} to live rows, redraws from the removal answer, and asks in a native `dialog.logout-confirm` before the removal by rater. The box has the title with `tabindex="-1"` taking the first focus, the sentence, an inline error line, Cancel and a destructive Remove all, no close X, no click handler on the dialog, Escape blocked only while a removal is being sent. Disabled buttons use `aria-disabled` with `.btn--off`. `moderate.css` adds three spacing rules.

## Tasks and commits

1. 9035a3fc feat(09-15): add the moderator ratings routes
2. 721dc655 feat(09-15): add the Ratings section to the moderator screen
3. 7bc7a58f test(09-15): pin that a rater with an unscreened username is named and removable

## Tests

New, each seen with `-v` and none skipped:

- `TestModeratorRatings` (list, remove one with the audit stamp and a repeat, remove all by one rater who rated two reporters answering removed 2 then 0, empty list is `[]`, bad names and bodies, wrong method 405, the unscreened username case), `TestModeratorRatingsHiddenFromOthers` (anonymous, signed in and signed in with a username, three routes, five methods, byte equal to both the report Keep route and an unknown route, no removal happened, the public profile names no rater), `TestRatingsIgnoreRemoved` (five one star raters lower the tag and number and add a ratings part; removing one, or removing all by one rater, returns the tag, number and no ratings part to the outcome only reading). Same name as a service test of plan 09-13, different package.
- `TestModeratorRatingsSection` and `TestModeratorRatingsNamesStayOnTheModeratorScreen` (web, Go).
- `web/moderate_ratings_test.mjs`: 11 node tests.

Mutation check: sending Remove all straight to the request instead of opening the box made 5 of the 11 node tests fail; restored (HEAD verified).

Gates: `go build ./...` and `go vet ./...` clean; `node --test web/*.mjs` 1520 of 1520; full `go test ./... -p 1 -count=1` on scratch database `p9_15_a` ok for every package, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skip. The scratch database was dropped.

## Deviations from Plan

**1. [Rule 1 - Inconsistency] The single removal answers the refreshed list.** The plan names `{"ratings": [...]}` and `{"removed": n}`. `ModeratorRemove` succeeds quietly whether or not a rating was there, so `{"removed": 1}` could be false. The remove one route answers `{"ratings": [...]}` (the list as stored now); only the remove by rater route answers `{"removed": n}`.

**2. [Rule 2 - Missing copy] Strings P9-C30 does not name.** "No such person." (the screen's line for an unknown username), "Enter the reporter's username." (empty field), "Removed the rating by {rater}." (single removal status), "Removing" (busy label of Remove all), the singular "Removed 1 rating by {rater}." (the plan's wording would read "1 ratings"), and the server 400 sentence "Send the rater's username." All plain words, no dashes.

**3. [Rule 3 - Blocking] The box sits inside main.** The plan says to build it like `#settings-delete-confirm`, which is a direct child of body. The page's single delegated click handler only reaches buttons inside `.mod-page`, so the dialog is the last child of main. A native modal dialog is in the top layer either way.

**4. [Rule 3 - Blocking] Small dated amendments to two shared contract tests** (the plan's list named only the new files). `web/moderate_contract_test.go`: `<h2` count 4 to 6, `class="mod-section"` 4 to 5, `mod-section__line` 3 to 4, `addEventListener(` 3 to 5 (two places), and the `apiRequest` anchor gains the fourth argument. `web/phase4_gates_contract_test.go`: a `p427BareLabelAllowlist` letting `templates/moderate.html.tmpl` hold the bare label Cancel (the plan's box has Cancel and Remove all, like the Delete box). The three lists plan 09-16 owns were not touched.

**5. [Decision] A rater with a `pending` username is named on this screen.** The notes say a username shows only when its screen is clear. `ModeratorRatingsForReporter` (09-13) filters only on a username existing, and the tool acts by name, so hiding a pending name would make that rating impossible to remove. Kept as a deliberate moderator only exception, pinned by a subtest and by `TestModeratorRatingsNamesStayOnTheModeratorScreen` (no other file reaches the rater routes). Raised as an open question below.

No auth gates. No stubs. No threat flags beyond the plan's register: T-09-56 (byte equal 404, tested), T-09-57 (names only here, tested), T-09-58 (`removed_by` and `removed_at` stamped, tested).

## Notes for later plans

- **Policy plan:** moderators can see rater usernames and who rated whom. The Policy text and its date (2026-10-29) are not changed here.
- **Open question for the owner:** keep showing a pending (unscreened) rater username to moderators, or hide it and give moderators another way to act? Default taken: show it.
- The moderator tool cannot reach a rater with no username or a deleted account (the list skips them); harmless since 7.1 requires a username.
- Plan 09-27 documentation can mention the three routes and that a removal is an audit row (`removed_by`, `removed_at`).
- `TestRatingsIgnoreRemoved` now exists in two packages (service from 09-13, handlers here); `-run` by name runs both if both packages are named.

## Known Stubs

None.

## Self-Check: PASSED

Created files exist (`moderation_ratings.go`, `moderator_ratings_e2e_test.go`, `moderate_ratings_contract_test.go`, `moderate_ratings_test.mjs`); commits 9035a3fc, 721dc655 and 7bc7a58f are in `git log`.
