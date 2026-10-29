---
phase: 08-report-page-and-comments
plan: 15
subsystem: service
tags: [comments, push, changes, settings, sqlc]
requires:
  - phase: 08-11
    provides: UpdateCommentOnYours, UpdateReplyToYours, the comment bell rows and the ReportChanged poke
  - phase: 03.3
    provides: the Changes push kind, the worker, the dispatcher
provides:
  - comment events in the phone pipeline through the existing Changes kind
  - the commented arm of FollowersWithChangesPush
  - the fixed lock screen phrases for the two comment kinds
  - the reworded Settings switch
affects: [08-19, 08-20, 09-25, 09-26]
tech-stack:
  added: []
  patterns: [fixed phrase table for lock screen words, send time predicate reused from the fading note]
key-files:
  created:
    - internal/service/pushcomments_test.go
  modified:
    - internal/store/queries/push.sql
    - internal/store/sqlc/push.sql.go
    - internal/service/pushcopy.go
    - internal/service/pushworker.go
    - internal/service/pushcopy_test.go
    - web/templates/settings.html.tmpl
    - web/settings_rows_contract_test.go
key-decisions:
  - "A comment note's title is '{Category} report near {area} has a new comment' (or 'has a new reply'), built from the category and never from 'Your report', because a reply note goes to the author of a comment who may not own the report. When the whole line would pass 64 runes the area is dropped, never the category."
  - "eventStillHolds lets both comment kinds through only through MayNotifyAboutReport (active and Live), as the plan says; it does not also test Confirmed, exactly like the fading note."
requirements-completed: [PAGE-02]
duration: single session
completed: 2026-10-29
status: complete
---

# Phase 8 Plan 15: Comment events on the phone Summary

A new comment on your report and a reply to your comment now reach a phone through the existing Changes notification kind, with fixed words only and for active Live reports only, and commenting makes you a push follower.

## The exact kind list for Phase 9

`EnqueueChangedFromUpdates` now reads:

```
AND u.kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'comment_on_yours', 'reply_to_yours')
```

Plans 09-25 and 09-26 copy this list and append their follow kind (A-P8-9). `changeEventWords` gained `UpdateCommentOnYours` "has a new comment" and `UpdateReplyToYours` "has a new reply".

## What was built

- push.sql: the widened kind list, and a third arm in `FollowersWithChangesPush` (`SELECT c.account_id FROM comments c WHERE c.report_id = ... AND c.deleted_at IS NULL AND c.moderation_state = 'clear'`), the same filter as the commented arm of `FollowedOpenReports` (08-11) and `viewer_commented` behind the feed's ViewerFollows. `make sqlc` changes only push.sql.go; `VoteLogForReport` is untouched.
- pushcopy.go: the two phrases in `changeEventWords`; `BuildPushText` builds a comment title from the category (and the area when the whole line fits 64 runes). The body is the body every Changed note has (subject, area, distance), so it holds no comment words, no username and no count. Body building moved into `changedBody`, behaviour unchanged for the four older events.
- pushworker.go: `eventStillHolds` cases for both kinds through `MayNotifyAboutReport(event, ...)`, with a comment naming D-07 and repo rule 15. A comment on a Provisional, disputed (Hidden), resolved, expired or withheld report stays in the bell.
- settings.html.tmpl: the Changes row label is now "Changes and comments on reports I posted, voted on or commented on". No other row changed, so the chevron count pin in places_contract_test.go needed no amendment.
- Unchanged and still green: `deviceSkipsRepeatPush`, `AccountPushSentAboutReport`, `heldByHourlyCap`, `anyAppleDevice`, the official alerts lane, the 3.3 names. A comment note counts toward the hourly limit and an Apple endpoint gets only the first note about a report, because those rules sit in the worker after `stillHolds`, which I did not touch.

## Tests

- New in `internal/service/pushcomments_test.go`: `TestPushCommentWords` (fixed phrases, 64 and 120 rune budgets over every category, three areas, both events, both ownership values, no comment word, no at sign, no noise word, plain characters, area dropped before the cap, unknown category gives no text), `TestEventStillHoldsComment` (7 report states times both kinds), `TestCommenterFollowsReportForPush` (follow arm for clear, deleted, removed and held comments; a comment on your report queues one changed row for the reporter and none for the commenter; Changes off queues nothing but the bell row exists; a reply queues one reply_to_yours row for the answered author), `TestLikesNotifyNoPush` (ten likes and a flag queue no outbox row, write no updates row, poke nothing).
- Amended with dated notes (rule 10): `pushcopy_test.go` budget sweep and noise sweep now include the two comment events; `web/settings_rows_contract_test.go` pins the new label.
- Mutation check: with the kind list narrowed back in the generated query, `TestCommenterFollowsReportForPush` fails on the queue and reply subtests; restored with `make sqlc`.
- Full run on a fresh `p8_15_a`: `go test ./... -p 1 -count=1 -v` exit 0, every package ok, only `TestGeocodeLive`, `TestOfficialLive` and `TestScreeningFixture` skipped. `go build ./...` and `go vet ./...` clean. The database was dropped.

## Deviations from Plan

None - plan executed as written. One reading choice: the plan's example title "Flood report near Benz Circle has a new comment" is used as the exact title; the plan's own test line ("ending with 'has a new comment'") agrees.

## Notes for later plans

- 08-19: docs/PUSH.md and docs/TRUST-MODEL.md still quote the old Settings label "Changes to reports I posted or voted on"; update them. Document that comment notes follow the fading note's rule (active Live), not the Confirmed flag, so a disputed Critical report (which stays Live) can still send a comment note.
- 08-20: the Settings label is long; it wraps on a phone. Check it at the owner viewing.
- Phase 9 (09-25, 09-26): copy the kind list above.

## Known Stubs

None.

## Threat Flags

None. T-08-67 (words on a lock screen) is covered by `TestPushCommentWords`, T-08-68 by `TestEventStillHoldsComment`, T-08-69 by `TestLikesNotifyNoPush`.
