---
phase: 08-report-page-and-comments
plan: 10
subsystem: feed-read
tags: [comments, follow, feed, api]
requires:
  - phase: 08-08
    provides: ReportExtrasForReports comment_count and viewer_commented
  - phase: 08-09
    provides: service.CommentsOpen
provides:
  - ViewerFollows with a commented arm
  - ReportView.CommentCount and ReportView.CommentsOpen on every read path
  - comment_count and comments_open on FeedReportResponse
affects: [08-07 popup row, 08-16 report page, 08-11, 08-15]
tech-stack:
  added: []
  patterns: [extras row carried through classified so History and Focus read the count without a second query]
key-files:
  created:
    - internal/api/handlers/comment_count_e2e_test.go
  modified:
    - internal/service/follow.go
    - internal/service/follow_test.go
    - internal/service/feed.go
    - internal/service/report.go
    - internal/service/historyread.go
    - internal/api/handlers/reports.go
key-decisions:
  - "classified (historyread.go) carries commentCount and viewerCommented from the extras row, so baseView fills the count and the follow arm for History, Past and Focus the same way Nearby does."
  - "CommentsOpen is computed from the visibility and expiry each path already resolved; no second Resolve."
patterns-established:
  - "Finished (Past and resolved) views carry the real comment_count and a false comments_open; the Past tab still forces viewer_follows false as before."
metrics:
  tasks: 2
  files: 7
status: complete
---

# Phase 8 Plan 10: Comment count, open state and the commenter follow Summary

The feed, history and report link reads now carry the real comment count and the server's open or closed word for comments, and a viewer whose only link to a report is a clear comment follows it.

## Commits

- e8d68fac: feat(08-10): commenter follows the report, comment count and open state on every read
- 6d993e12: feat(08-10): comment_count and comments_open on the wire

## What changed

- `ViewerFollows(rows, reportID, viewerAccountID, isOwnReport, commented)`: the commented argument is the extras row's `ViewerCommented` (clear and not deleted only, review W8). Viewer 0 still follows nothing.
- `ReportView.CommentCount int` and `ReportView.CommentsOpen bool`, filled in `NearbyReports` (report.go) and in `baseView` (historyread.go, shared by History, Past and Focus). `CommentsOpen` calls `CommentsOpen(vis, expiresAt, now)` with values already resolved.
- `FeedReportResponse.comment_count` and `comments_open`, always present, with swag example tags. The name `comment_count` matches what map.js reads (08-07). docs/ is left for plan 08-19.

## Tests

- `TestCommenterFollowsReportInFeed` and two added cases in `TestViewerFollows` (follow_test.go, dated amendment note; every other assertion kept).
- `TestCommentCountOnFeed`: two clear comments, one reply, one pending and one deleted give 3 to the author, the pending comment's own author and a commenter, on the feed and on the report link; a report with none gives 0.
- `TestCommentsOpenOnFeed`: active true, resolved false; a commenter follows, and stops following once the comment is deleted.
- Full run on p8_10_a: `go test ./... -p 1 -count=1` passes; the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. TestNearbyResponseOmitsSessionID, TestPhoneAlertAreaNameStaysOffFeedAndBell and TestViewerStateOnTheWire pass unchanged.

## Deviations from Plan

None. The plan executed as written. The Phase 5 Demo twin needed no change (demo tests pass).

## Notes for later plans

- 08-11 and 08-15 must keep their SQL follow arms in step with the commented arm here (a clear, not deleted comment).
- gofmt -l reports internal/api/handlers/place_test.go, internal/geocode/photon.go and internal/store/comments_test.go; these were not touched by this plan and were left alone.

## Self-Check: PASSED

Both commits exist (e8d68fac, 6d993e12), the created test file exists, `grep -c 'json:"comment_count"'` and `json:"comments_open"` each print 1, no non-ASCII characters in the changed lines.
