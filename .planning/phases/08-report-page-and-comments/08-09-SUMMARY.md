---
phase: 08-report-page-and-comments
plan: 09
subsystem: service
tags: [comments, moderation, screening, likes, flags, access-rules]
requires:
  - phase: 08-08
    provides: comments tables, CommentsForReport, comment_count, seed helpers
  - phase: 07.1
    provides: usernames and the username screen
provides:
  - CommentService with Post, List, Delete, Like, Unlike, Flag
  - CleanCommentBody and CommentMaxRunes
  - the comment words (commentscopy.go)
  - proof that likes and comment flags never move a trust number
affects: [08-10, 08-11, 08-12, 08-13, 08-14, 08-15, 09-10]
tech-stack:
  added: []
  patterns: [one shared report gate for every comment action, AST scan of the trust files, ScreenAbuse instead of the gibberish prefilter]
key-files:
  created:
    - internal/service/comments.go
    - internal/service/commenttext.go
    - internal/service/commentscopy.go
    - internal/service/commenttext_test.go
    - internal/service/comments_test.go
    - internal/service/commentlikes_test.go
  modified: []
key-decisions:
  - "Like, Unlike and Flag read the comment first, then run the report check on the comment's own report, so a deleted comment on a withheld report still answers not found."
  - "A flag on a comment of a finished Rescue report answers ErrCommentGone, because that page lists no comments."
  - "The report check mirrors Focus completely: withheld, unknown, ended over HistoryWindow ago, resolved over HistoryWindow ago, and any state no view lists are all ErrReportNotFound."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 09: Comment service Summary

One service now holds every comment rule: cleaned and bounded text, screening with a held outcome, replies that join a thread, the closed state, delete, likes and comment flags, and the list a viewer reads. No route, notification or worker was added.

## What was built

### Text rules (`commenttext.go`, `commentscopy.go`)

- `CommentMaxRunes = 280`; `CleanCommentBody(s) (string, error)`; errors `ErrCommentEmpty`, `ErrCommentTooLong`, `ErrCommentCharacters`.
- Order: line feeds normalised and ends trimmed, control and `unicode.Cf` characters refused except U+200C and U+200D, joiners and white space trimmed together from both ends (one `TrimFunc`), non empty test over letters, marks, numbers, punctuation and symbols, joiner placement (each joiner must follow a letter, mark or symbol), three or more line feeds collapsed to two, then the code point count. The constants for the two joiners already existed in `profile.go` and are reused.
- Messages: `CommentAuthorNewMember`, `CommentAuthorDeletedAccount`, `CommentsClosedMessage`, `CommentEmptyMessage`, `CommentTooLongMessage`, `CommentCharactersMessage`, `CommentNeedsUsernameMessage`, `CommentGoneMessage`, `CommentNotYoursMessage`, `CommentOwnFlagMessage`.

### The service (`comments.go`)

- `CommentQuerier` (with `var _ CommentQuerier = (*sqlcgen.Queries)(nil)`), `CommentService`, `CommentOption`, `WithCommentScreener`, `WithCommentClock`, `WithCommentChangeNotifier` (stored only), `NewCommentService`.
- `CommentPageSize = 50`, `CommentReplyCap = 500`, `CommentsOpen(v Visibility, expiresAt, now) bool`.
- Types: `CommentInput{ReportID, AccountID, HasUsername, Words, ReplyTo}`, `CommentView`, `CommentList{Comments, Count, Open, More, NextAfter}`, `LikeState{Likes, ViewerLiked}`, `CommentRejectedError{Reason}` (Unwrap to `ErrCommentRejected`).
- Methods: `Post(ctx, CommentInput) (CommentView, error)`, `List(ctx, reportID, viewerAccountID, after) (CommentList, error)`, `Delete(ctx, commentID, accountID) error`, `Like`, `Unlike(ctx, commentID, accountID) (LikeState, error)`, `Flag(ctx, commentID, accountID, FlagInput) error`.
- Errors: `ErrCommentsClosed`, `ErrCommentNeedsUsername`, `ErrCommentGone`, `ErrCommentNotYours`, `ErrCommentOwnFlag`, `ErrCommentRejected`. `ErrReportNotFound`, `ErrAlreadyFlagged`, `FlagNoteRejectedError` and `ValidationError` are reused from earlier code.

### How the rules were met

- **Screening:** `ScreenAbuse` only, `Decide(scores, false)` through it. Clear stores clear (verdict pass, scores and model kept when the provider answered); borderline stores clear with `needs_review` and verdict borderline; rejected returns `CommentRejectedError` and stores nothing; unfinished stores `pending` with `next_attempt_at = now` and returns `Held`. A nil screener stores clear with no verdict. The check happens after the cheap checks, so a refused text, a closed report or a reply to a gone comment costs no provider call.
- **Report gate (`reportFor`):** one read of `ReportVoteContext` and `CurrentVotesForReports`, then `Resolve`, `ViewOf`, `ClosureOf`. It returns `ErrReportNotFound` for unknown, withheld, ended or resolved more than `HistoryWindow` ago, and any state no view lists. The finished Rescue test reuses `CoarsenFinishedRescue` (finished means the resolved or past view).
- **Replies:** the target must be on the same report, clear and not deleted. `parent_id` is the target's root, `reply_to_id` the comment answered. `ReplyToAuthor` is shown only when the answered author's screen is clear and the account is not deleted.
- **List:** one `CommentsForReport` call (50 roots plus one, reply cap 500), threads grouped, the extra root dropped into `More` and `NextAfter`, `Count` from `ReportExtrasForReports`. Placeholders carry no words, name, likes, tag or viewer flags whatever the row holds. `IsReporter` is false on the viewer's own comments and on placeholders. A finished Rescue report returns an empty list, `Open` false.
- **Delete:** `MarkCommentDeleted`, then on zero rows `CommentTarget` to tell unknown (gone), not yours, and already deleted (nil). No report check, so it works on a closed, withheld or too old report.
- **Like, Unlike, Flag:** comment first, then the report gate (not found), then closed (Like and Unlike), then gone, then own (Flag), then the note screen, then the write. None calls the notifier or writes an `updates` or outbox row.

## Tests

All run on the disposable database `p8_09_a` (created, then dropped at the end).

- `commenttext_test.go`: `TestCleanCommentBody` (about 40 rows), `TestCommentShortReplies` (also asserts `IsGibberish` is true for each, which shows why `CheckText` would be wrong), `TestCommentCopyWording`.
- `comments_test.go`: `TestCommentScreeningOutcomes`, `TestCommentReplyJoinsThread`, `TestCommentReplyTargetMustBeVisible`, `TestCommentsClosed`, `TestCommentDelete`, `TestCommentNeedsUsername`, `TestCommentWithheld`, `TestCommentListNames`, `TestCommentFinishedRescueHidden`, `TestCommentFlag`, `TestCommentLikeAccess`.
- `commentlikes_test.go`: `TestLikeOnePerAccount`, `TestLikesNeverFeedTrust` (an AST scan over `trust.go`, `trustfacts.go`, `trustview.go`, `trustcopy.go`, `visibility.go`, `confirmed.go`, `reliability.go`, `reliablefilter.go`, `currency.go`, plus comment stripped scans of `votes.sql` and `reliability.sql`; then `BuildTrust` as JSON and the vote rows compared before and after ten likes and a flag).
- Full run `go test ./... -p 1 -count=1 -v` exit 0: 6017 `--- PASS` lines, 20 packages ok, zero `--- FAIL`, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped. `go build ./...` and `go vet ./...` clean. `-short` run of `./internal/service/` clean. Every new test name shows `--- PASS` once at the top level.

## Deviations from Plan

None that change behavior. Small choices the plan left open:

- The Focus rule is mirrored fully (also a resolved report whose closure is older than `HistoryWindow`, and a view of none), not only the expiry test.
- `Flag` on a finished Rescue report answers `ErrCommentGone`; the plan did not say.
- Two extra tests beyond the plan list: `TestCommentCopyWording` and `TestCommentLikeAccess`.
- `Post` reads the new row back with `CommentTarget` to get the author's own username (the insert returns only id and time), so a pending screen still shows the author their own name.

An early background run of the full suite was started with output redirected to a file in /tmp by mistake. It was killed within seconds and the file removed; the full run reported above was a fresh one.

## Notes for later plans

- 08-11 (notifications): `WithCommentChangeNotifier` is stored, never called. Call it after a successful `Post` only. `ReportChangeNotifier.ReportChanged(reportID, actorAccountID)` is the existing interface.
- 08-12 (routes): map `ErrReportNotFound` to 404, `ErrCommentsClosed` to 409, `CommentRejectedError` and `FlagNoteRejectedError` to 422 with `RejectMessage`, `ErrCommentGone` to 404 or 410 with `CommentGoneMessage`, `ErrCommentNotYours` to 403, `ErrCommentOwnFlag` to 403, `ErrAlreadyFlagged` to 409, the three text errors to 400 or 422 with the matching message. Pass `HasUsername` from the account. The screener call is bounded at 3 s (`InlineScreenTimeout`), inside the 10 s server write timeout. Logs must keep ids and outcome words only; the service writes no log line of its own except `ScreenAbuse`'s existing one, which carries no text.
- 08-13 and 08-14: held comments are `moderation_state = 'pending'` with `next_attempt_at` set; borderline are clear with `needs_review`. Skip rows with `deleted_at` set.
- Phase 9 (09-10): the block filter goes in `CommentsForReport` and `comment_count` only. `CommentService` shapes whatever the query returns.
- Policy plan: comments add stored personal data tied to an account (the comment text, the likes an account gave, the comment flags and flag notes an account made, all removed or blanked on account deletion by 08-08). The Policy and `CurrentTermsVersion` were not touched here.

## Self-Check

Files present: `internal/service/comments.go`, `commenttext.go`, `commentscopy.go`, `commenttext_test.go`, `comments_test.go`, `commentlikes_test.go`. Commits present: 5bbaac74 (text rules and words), 933e8078 (post, reply, list, delete), a052c2f3 (likes and flags). Acceptance greps: no `IsGibberish` or `CheckText` in code lines of `commenttext.go` and `comments.go`; `ScreenAbuse` present in `comments.go`. No non ASCII byte, so no dash, in any file of this plan.

## Self-Check: PASSED
