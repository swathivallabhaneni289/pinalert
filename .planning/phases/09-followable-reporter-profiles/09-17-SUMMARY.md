---
phase: 09-followable-reporter-profiles
plan: 17
subsystem: web
tags: [ratings, stars, resolve, rate-prompt, javascript, css, go]
requires:
  - phase: 09-13
    provides: RatingService MayRate and YourRating
  - phase: 09-14
    provides: Deps.Ratings, the rating routes, the 409 and 404 answers
  - phase: 09-16
    provides: PinalertStars.createControl and stars.css
provides:
  - CastVoteResult.RatePrompt, RatePrompter, WithVotingRatePrompts
  - rate_prompt {username, profile_path} on the resolve response
  - the skippable star box (div.rate-box) after Yes, mark resolved
affects: [09-29]
tech-stack:
  added: []
  patterns:
    - "the server alone decides the box: one optional member on the resolve answer, built after the vote is written, never able to fail the vote"
    - "the box mounts after the refetch, inside the card's actions group, guarded by a live host check"
key-files:
  created:
    - internal/service/rateprompt_test.go
    - internal/api/handlers/rate_prompt_e2e_test.go
    - web/votes_stars_test.mjs
  modified:
    - internal/service/trust.go
    - internal/api/handlers/votes.go
    - cmd/server/main.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/votes.js
    - web/static/css/stars.css
    - web/templates/index.html.tmpl
    - web/templates/profile.html.tmpl
key-decisions:
  - "The prompt is built after InsertVote, so the resolve just cast counts as the reader's qualifying vote (a witness at the scene qualifies)."
  - "The box mounts after the list refetch, not right after the vote, and only while the card's popup is still open, so a resolve that closes the report shows no box instead of one that flashes and goes."
  - "The Low data page is left out of the link change: its asset policy test forbids stars.css and stars.js there, and it has no Mark resolved."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 17: The skippable star box after Yes, mark resolved Summary

After "Yes, mark resolved" on someone else's report, an eligible reader who has not rated that reporter now sees one skippable box, "Rate {username}", with the same five stars as the profile and an outlined Skip. The server decides everything: the resolve answer carries `rate_prompt` only for an opted in reporter with a clear username who has not blocked the reader, not the reader's own report, and a reader who `RatingService.MayRate` allows and who has no rating yet.

## What was built

- **Rate prompt on the vote (task 1).** `VotingService` gains `WithVotingRatePrompts(RatePrompter)`; `RatePrompter` is `MayRate` plus `YourRating`, which `RatingService` already satisfies. In `CastVote`, after the vote row is written and the trust object is built, a resolve (kind resolution, value resolve) calls `ratePromptFor`: no prompter, no reporter account, the reader's own report, or an unknown reader gives nothing; otherwise it reads the reporter through `PublicProfilePathsForViewer` (so private, pending, deleted and blocking reporters give nothing, T-09-62), then `MayRate`, then `YourRating`, and returns `RatePrompt{Username, ProfilePath: "/u/" + username}` only when allowed and unrated. Every read error is logged with the report, reporter and rater ids and leaves the prompt empty (T-09-64); the vote never fails. `CastVoteResponse.RatePrompt` is `rate_prompt,omitempty` (`RatePromptResponse{username, profile_path}`), documented in swag; `main.go` passes `WithVotingRatePrompts(ratingService)`. The prompt is on `CastVoteResult`, beside `UndosLeft`, never inside `Trust`, so the feed and the vote answer stay byte for byte equal.
- **The box (task 2).** `votes.js` calls `offerRateBox` at the end of the Yes, mark resolved success path, after `Pinalert.fetchReports`. It mounts one `div.rate-box` as the next sibling after `block.error` inside the card's `.popup-card__actions` group, before Share and above the page node (review M6). The box holds the shared control (`PinalertStars.createControl`, legend "Rate {username}", current 0), a hidden `.inline-error` line, an outlined `btn rate-box__skip` "Skip", and an empty `role="status"` line. A tap saves through the control; `onStatus` collapses the box to the one line "You rated {username} 4 stars." and moves focus to it only when focus sat on a star. Skip adds the report id to a per page skipped set, sends nothing and removes the box (focus goes to the card's Undo when focus was in the box). `onGone` removes the box quietly. The box is removed by Undo from the card (`onTakeBackClick`) and from the message (`takeBackFromToast`). It waits for `PinalertChangesAsk.settled()` when that script is present, mounts at once otherwise, and mounts nothing when `PinalertStars` is missing, the prompt has no username, the id was skipped, a box is already there, or the card's popup is gone (`rateHostIsLive`, the same open popup test the vote focus code uses). The whole path is wrapped so it can never change a vote's outcome. `stars.css` gained the `.rate-box` rules (1px `--color-border`, radius 12px, padding 16px, `--color-bg`, 8px gap, empty status line out of flow). `index.html.tmpl` and `profile.html.tmpl` link `stars.css` and `stars.js`.

## Tasks and commits

1. 18fea50b feat(09-17): carry rate_prompt on the resolve response
2. b27427c8 feat(09-17): offer the skippable star box after Yes, mark resolved

## Tests

New, each seen with `-v` and none skipped: `TestRatePromptConditions` (11 subtests: witness whose only vote is the resolve, earlier confirm, own report, private reporter, blocked by the reporter, blocked by the reader, already rated, confirm and dispute and reopen, take back, no prompter, failing prompter), `TestResolveResponseRatePrompt` (confirm answers none, resolve names the reporter and `/u/` path with exactly two members, none after a rating, none for the reporter's own resolve, none for a private reporter). `web/votes_stars_test.mjs` has 22 tests, printed by name, including the two review M6 cases (bubble then panel leaves exactly one box inside the moved card; the box sits inside the card before the page node) and a source guard that `map.js` keeps the actions order the box relies on. Mutation check: replacing the live popup test with `return true` failed the closing popup test; restored.

- `node --test web/*.mjs`: 1569 tests, 1569 pass.
- Full gate on scratch database `p9_17_a`: `go build ./...`, `go vet ./...` and `go test ./... -p 1 -count=1` all ok; `make swag` leaves no diff and `@license.name` still reads "Unlicensed (all rights reserved)". A `-v` run of the service and web packages shows only the known `TestScreeningFixture` skip. The scratch database was created by me (it did not exist) and dropped.

## Deviations from Plan

**1. [Rule 3 - Blocking] Files outside the plan's list.** Widening `VotingQuerier` with `PublicProfilePathsForViewer` needed the fakes updated: `fakeVotingQuerier` in `internal/service/trust_test.go` (new `profileRows`, `profileErr`, `profileArgs` fields, not written to `calls` so the pinned read order tests keep their result) and `bodyVoteFake` in `internal/api/handlers/votes_body_test.go` (answers nothing). The other satisfiers needed no edit: `memQuerier` (`internal/demo`) already has the method, `p05VotingFake` gets it by promotion from `fakeFeedQuerier`. The two main test harnesses now pass `WithVotingRatePrompts(ratings)` (`newE2EServerWith` in `reports_e2e_test.go`, `newTrustE2EServer` in `trust_e2e_test.go`), and `ratings_wiring_test.go` gained `WithVotingRatePrompts(ratingService)` in its main.go text list. `push_flow_e2e_test.go` and `reliable_alert_e2e_test.go` rebuild `d.Votes` without the prompter on purpose, since they do not test the box. `person_contract_test.go` `TestStarsControlLook` allowed only the text and surface colours in `stars.css`; I amended it (dated note) so rules whose selector names `.rate-box` may also use `--color-border` and `--color-bg`, as UI-SPEC 3b requires. The control's own rules are still held to the two colours.

**2. [Rule 3 - Blocking] The Low data page is not linked to the star files.** The plan says to link `stars.css` and `stars.js` in the three templates. `TestLowDataAssetPolicy` (`web/low_contract_test.go`) forbids both on the Low data page, and that page has no "Yes, mark resolved" (`low.js` builds its vote block with `resolve: false`), so `low.html.tmpl` is unchanged.

**3. [Plan assumption] No row mount on Activity or Low data.** The plan allowed for a resolve in a report row on those pages. There is none: `activity.js` only has Reopen, `ended-cards.js` only has the take back, `low.js` builds no resolve. The only host of "Yes, mark resolved" is the opened report card (`map.js` `buildPopupCard`, `createVoteBlock`), so the box has one mount. `profile.html.tmpl` (My activity) is linked anyway, as the plan says, and it is harmless there.

**4. Noise word guard.** `static/js/votes.js` was added to `noiseCheckedFiles` (its string literals are scanned now); it passed with no change to existing copy. It was not added to `phaseScripts` because the old file holds dashes in comments.

## Notes for the owner and later plans

- **A resolve that closes the report shows no box.** When the reader's resolve is the one that retracts the report (its last independent agreement), the refetch removes the marker, popup and card (`map.js` drops them for a report that leaves the Active list), so there is no card to hold the box and none is drawn. That reader can still rate from the reporter's profile (`See this reporter`). A resolve that is only waiting for agreement keeps the card, and the box shows. Making the box survive a closed card would need a host outside `map.js` (a toast or a wrapper), which the plan forbids; this is for the owner to decide.
- The box never moves focus on mount. After a tap or Skip it moves focus only when focus was already inside the box.
- `PinalertStars.createControl` is mounted with no `onStatus` text of its own beyond the collapse; the 429 and 409 lines come from the control as 09-16 built them.
- Plan 09-29 can rely on `rate_prompt` being absent from every non resolve answer and from the Demo (the Demo service has no prompter and its querier answers no profiles).

## Known Stubs

None.

## Threat Flags

None. T-09-62 (the prompt names a reporter only through `PublicProfilePathsForViewer`; tested for private and blocking reporters), T-09-63 (the box exists only when the server sent `rate_prompt`; the rating routes check again) and T-09-64 (errors while building the prompt are logged and leave it empty; `failing prompter never fails the vote`) are mitigated.

## Self-Check: PASSED

Created files exist: `internal/service/rateprompt_test.go`, `internal/api/handlers/rate_prompt_e2e_test.go`, `web/votes_stars_test.mjs`. Commits 18fea50b and b27427c8 are in `git log`.
