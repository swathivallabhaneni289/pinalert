---
phase: 08-report-page-and-comments
plan: 12
subsystem: api
tags: [comments, routes, rate-limits, screening, swagger]
requires:
  - phase: 08-09
    provides: CommentService with Post, List, Delete, Like, Unlike, Flag
  - phase: 08-03
    provides: the Share bucket pattern in the four limiter places
  - phase: 07.1
    provides: Account.HasUsername
provides:
  - six gated comment routes over CommentService
  - Comment and Like buckets with network backstops in all four limiter places
  - CommentLimitMessage and LikeLimitMessage
  - Deps.Comments and the main.go wiring
affects: [08-16, 08-17, 08-18, 08-19]
tech-stack:
  added: []
  patterns: [one errors.Is and errors.As ladder shared by all comment routes, no-refund per account bucket for screener-spending routes]
key-files:
  created:
    - internal/api/handlers/comments.go
    - internal/api/handlers/comments_e2e_test.go
  modified:
    - internal/api/handlers/ratelimit_gate.go
    - internal/api/handlers/flag_e2e_test.go
    - internal/service/ratecopy.go
    - internal/service/ratecopy_test.go
    - internal/api/router.go
    - cmd/server/main.go
key-decisions:
  - "A flag on your own comment answers 409 field flag, as the plan's status map says, not the 403 that the report flag handler gives for an own report."
  - "The comment flag is its own words for a repeat: 'You already flagged this comment. Moderators will see it.' (the service has no comment flag constant)."
  - "cmd/server/main_test.go and internal/api/ip_backstop_test.go needed no change: neither pins the field list or the constants."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 12: Comment routes and buckets Summary

Comments can now be listed, posted, replied to, liked, unliked, deleted and flagged over six gated JSON routes, with per account Comment and Like buckets (and network backstops) in every place the limiter is configured.

## What was built

### Buckets (commit b05fd16f)

- `RateLimits.Comment`, `RateLimits.Like`, `RateLimits.IPBackstopComment`, `RateLimits.IPBackstopLike`, each built in `NewRateGate` (fields `comment`, `like`, `ipBackstopComment`, `ipBackstopLike`).
- Constants in `ratelimit_gate.go`: `commentBucketName = "comment"`, `commentRateField = "rate_comment"`, `likeBucketName = "like"`, `likeRateField = "rate_like"`, `ipBackstopCommentBucket = "ip_backstop_comment"`, `ipBackstopLikeBucket = "ip_backstop_like"`.
- `RateGate.CommentMiddleware()` (backstop, then the account bucket, every attempt spends, no refund because each attempt may be a screener call) and `RateGate.LikeMiddleware()`.
- Defaults in `router.go` `RateLimitsDefault` and the four `fill` lines in `resolveRateLimits`: Comment `{20 s, burst 6}`, IPBackstopComment `{200 ms, burst 600}`, Like `{3 s, burst 30}`, IPBackstopLike `{30 ms, burst 3000}`.
- `cmd/server/main.go`: `commentEvery`, `commentBurst`, `likeEvery`, `likeBurst`, `ipBackstopCommentEvery`, `ipBackstopCommentBurst`, `ipBackstopLikeEvery`, `ipBackstopLikeBurst`, with the reasoning comment, and four `deps.RateLimits` entries.
- `service.CommentLimitMessage(wait)` = "You're commenting very quickly. You can comment again in {wait}. Your words are still here." and `service.LikeLimitMessage(wait)` = "You're reacting very quickly. You can try again in {wait}." (`ratecopy.go`, pinned by `TestCommentLimitMessages`).
- `p20Tweak` (flag_e2e_test.go) opens the four new buckets, with a dated amendment note.

### Routes (commit 69441271)

Registered flat in the gated group after the share route (`router.go`):

| Route | Middleware | Handler |
| --- | --- | --- |
| `GET /api/reports/{id}/comments` | none (polled) | `ListComments` (optional `after`, positive integer else 400 field `after`) |
| `POST /api/reports/{id}/comments` | `CommentMiddleware` | `PostComment` |
| `POST /api/comments/{id}/like` | `LikeMiddleware` | `LikeComment` |
| `POST /api/comments/{id}/unlike` | `LikeMiddleware` | `UnlikeComment` |
| `POST /api/comments/{id}/delete` | `LikeMiddleware` | `DeleteComment` |
| `POST /api/comments/{id}/flag` | `FlagMiddleware` (report flag bucket and backstop) | `FlagComment` |

JSON types (`handlers/comments.go`):

- `PostCommentRequest`: `words`, `reply_to` (optional; bodies capped at 8 KiB, unknown members refused).
- `CommentResponse`: `id`, `author`, `author_kind`, `reply_to_author` (omitted when empty), `is_own`, `is_reporter`, `can_delete`, `viewer_liked`, `viewer_flagged`, `likes`, `reply_count`, `at` (RFC 3339 UTC), `deleted`, `removed`, `held`, `words`, `replies` (always an array).
- `CommentListResponse`: `comments`, `comment_count`, `comments_open`, `more`, `next_after`.
- `LikeResponse`: `likes`, `viewer_liked`. `DeleteCommentResponse`: `deleted`. Flag reuses `FlagRequest` and `FlagResponse`.
- Every response sets `Cache-Control: no-store`.
- `Deps.Comments *service.CommentService`; nil builds one with the report service's screener. `main.go` builds it with `WithCommentScreener(screener)` and `WithCommentChangeNotifier(pushDispatcher)` only inside the push on branch.

Status map as built: 201 posted or flagged; 200 for like, unlike, delete, list; 400 field `words` (empty, too long, characters) with the commentscopy messages, field `id`, `after`, `body`, `reply_to`, `reasons`, `note`; 422 field `words` (comment) or `note` (flag note) with `RejectMessage(reason)`; 409 field `comments` "Comments are closed", field `username` with `CommentNeedsUsernameMessage`, field `flag` for an own comment or a repeat flag; 404 field `report` (unavailable report) or field `comment` (gone comment, also a reply to a gone comment); 403 field `comment` for another person's delete; 429 from the middleware (fields `rate_comment`, `rate_like`, `rate_flag`).

Swag blocks sit on all six handlers. A scratch `swag init` (output outside the repository) produced a `swagger.json` with the five path keys that hold the six operations; `docs/` is untouched for plan 08-19.

## Tests

On the disposable database `p8_12_a` (created, dropped at the end). New, all showing `--- PASS` at top level: `TestCommentScreening`, `TestCommentListQuery` (extra), `TestCommentResponseOmitsIdentifiers`, `TestCommentClosedRoutes`, `TestCommentNeedsUsernameRoute`, `TestCommentDeleteRoutes`, `TestCommentFlagRoutes`, `TestCommentRoutesAreGated` (six sub tests), `TestCommentRateLimit`, `TestLikeRateLimit` (comments_e2e_test.go) and `TestCommentLimitMessages` (ratecopy_test.go).

- `go build ./...` and `go vet ./...` clean; `DATABASE_URL= go test ./... -short` clean.
- Full run `go test ./... -p 1 -count=1 -v` exit 0: 6047 `--- PASS` lines, zero `--- FAIL`, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped.
- The screener call inside Post is bounded at 3 s (`InlineScreenTimeout`), inside the 10 s server write timeout, so no write deadline extension is needed.

## Deviations from Plan

None that change behavior. Choices the plan left open:

- Own comment flag is 409 (plan's status map) although the 08-09 note suggested 403; the plan won.
- The repeat flag message is written in the handler, because the service has no comment flag constant. A later plan may move it to commentscopy.go.
- Extra test `TestCommentListQuery` (the `after` value and an unknown report).
- A reply to a gone comment answers 404 field `comment`.

## Notes for later plans

- 08-16, 08-17 (comments client): the 429 body for a post is field `rate_comment` with the sentence that already says "Your words are still here."; the composer must keep its text on any non 201 answer. Likes and deletes answer 429 field `rate_like`.
- 08-18 (flag dialog comment mode): post to `/api/comments/{id}/flag` with the same body as the report flag; an own comment or a repeat flag is 409 field `flag`; a refused note is 422 field `note`.
- 08-11 (notifications): `WithCommentChangeNotifier` is wired in main.go inside the push on branch; the service must call it after a successful Post.
- A comment withheld from a viewer (held, other account) is simply absent from the list; there is no per comment 404 on the list.
- 08-19: regenerate `docs/` with `swag init -g internal/api/router.go`; the six operations are annotated with `@Tags comments`.

## Threat Flags

None. All six routes sit inside the gated group, answer no identifiers, and log ids only.

## Self-Check

Files present: `internal/api/handlers/comments.go`, `internal/api/handlers/comments_e2e_test.go`, plus the edits listed above. Commits present: b05fd16f (buckets), 69441271 (routes and tests). Acceptance greps: CommentMiddleware and LikeMiddleware 2; fill lines 4; main.go constants present; router paths 2 and 4; NewCommentService in main.go. No em or en dash and no non ASCII byte in the two new files or in the added lines of the others.

## Self-Check: PASSED
