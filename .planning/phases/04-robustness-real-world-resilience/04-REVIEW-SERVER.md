---
phase: 04-robustness-real-world-resilience
reviewed: 2026-10-16T08:10:00Z
depth: deep
files_reviewed: 38
files_reviewed_list:
  - cmd/modrecord/main.go
  - cmd/server/main.go
  - internal/api/gate.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/chrome.go
  - internal/api/handlers/flag.go
  - internal/api/handlers/legal.go
  - internal/api/handlers/low.go
  - internal/api/handlers/moderation.go
  - internal/api/handlers/page.go
  - internal/api/handlers/ratelimit_gate.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/votes.go
  - internal/api/moderator.go
  - internal/api/router.go
  - internal/moderation/client.go
  - internal/moderation/loader.go
  - internal/ratelimit/perkey.go
  - internal/service/auth.go
  - internal/service/feed.go
  - internal/service/flag.go
  - internal/service/historyread.go
  - internal/service/moderation.go
  - internal/service/moderationcopy.go
  - internal/service/modworker.go
  - internal/service/ratecopy.go
  - internal/service/reliability.go
  - internal/service/report.go
  - internal/service/reporter.go
  - internal/service/screening.go
  - internal/service/textcheck.go
  - internal/service/trust.go
  - internal/service/visibility.go
  - internal/store/migrations/00007_add_moderation_and_idempotency.sql
  - internal/store/queries/moderation.sql
  - internal/store/queries/reliability.sql
  - internal/store/queries/reports.sql
  - internal/store/queries/votes.sql
findings:
  critical: 0
  warning: 3
  info: 9
  total: 12
status: issues_found
---

# Phase 4: Code Review Report (server half)

**Reviewed:** 2026-10-16T08:10:00Z
**Depth:** deep
**Files Reviewed:** 38
**Status:** issues_found

## Summary

Scope: the Go and SQL files of Phase 4 changed since 2a765a4 (the web half is in 04-REVIEW-WEB.md).

What was checked and held up:

- Withheld content. Every public read path resolves the stored moderation state through the one `Resolve`: Nearby (`report.go:893-920`), History, RecentHistoryCounts and Focus (`historyread.go:193-247`, `ViewOf` in `history.go:78` sends `VisibilityWithheld` to `FeedViewNone`), the Activity votes list (`auth.go:624-629`), the vote route (`trust.go:441`), the flag route (`flag.go:138`) and the reporter rating (`reliability.go:72`, fed through `reporter.go:48`). A report missing from the extras read fails closed on every path. `withheld_e2e_test.go` covers live, disputed, past and a faded withheld report on a link. The Low data page has no server POST handler at all: its only server route is `GET /low`, and its actions go through the same JSON routes.
- Idempotent submit. The unique index is partial and keyed on (account, request id); `InsertReportIdempotent` repeats the column pair and predicate so Postgres infers it; `ReportByRequest` is always account scoped; the race path (no row back) replays and refunds the allowance.
- CLIENT_IP_HEADER. When unset the router installs `ClientIPFromRemoteAddr` and no header is ever read; when set, chi's `ClientIPFromHeader` takes the last value and sets nothing on a missing or bad value, and `reserveIPBackstop` then skips rather than falling back.
- Request log writes the escaped path only; the rate limit block log carries a bucket, a numeric account id and a wait.
- Migration 00007: constraints, partial indexes and the down order (child tables first, then the index, constraints and columns) are sound.
- Time handling: the whole suite (`./internal/... ./cmd/...`) passes with `TZ=UTC` on a scratch database, which mirrors the Linux CI clock zone. The quota reschedule writes back `expires_at` read from the database, so the microsecond value matches the claim predicate exactly.

Three Warnings remain. One is a verified Postgres race in which a late worker reject removes a report a moderator just kept, contradicting the guarantee written in `modworker.go` and `moderation.sql`. One is the network backstop shape, which is shared by every action of every account behind one carrier NAT address. One is a pre-existing privacy leak of email addresses into the server log.

## Warnings

### WR-S01: A late worker reject can remove a report a moderator just kept

**File:** `internal/store/queries/moderation.sql:93-100` (SetReportModerationState), `internal/service/modworker.go:94-98, 271-287`
**Issue:** `SetReportModerationState` guards the report update with `EXISTS (SELECT 1 FROM moderation_checks mc WHERE ... mc.status = 'pending')`. Under READ COMMITTED, when the worker's UPDATE waits on the reports row lock that `KeepReport` holds, Postgres rechecks the WHERE clause against the new reports row but evaluates the `EXISTS` on `moderation_checks` with the statement's original snapshot, which still sees `pending`. So the guard does not see Keep's close of the check.

Verified on a scratch database (p4_rev_srv, migrated to version 7): one report with `moderation_state = 'pending'` and a pending check. Session A ran the `KeepReport` statement inside an open transaction (held for 3 s so the overlap is reliable). Session B, started 1 s later, ran the worker's `UPDATE reports SET moderation_state = 'removed' ... AND EXISTS (... status = 'pending')`. It blocked, and after A committed it reported `UPDATE 1`. Final state: `moderation_state = removed`, check `status = done`. In production Keep is a single autocommit statement, so the window is only while that statement overlaps the worker's UPDATE: narrow, but real. The outcome: the moderator's Keep answers 200, the audit row says `keep`, and the report is in fact removed, which is terminal and cannot be undone through Keep (409). The comment at `modworker.go:94-98` ("cannot be undone by a late reject") and the query comment at `moderation.sql:89-92` claim the opposite.
**Fix:** close the check and move the report in one statement, driving the report update from the rows the check update actually changed. The check row update then waits on Keep's lock on the same check row and rechecks `status` against the new version:
```sql
-- name: FinishCheckAndSetState :exec
WITH fin AS (
    UPDATE moderation_checks
    SET status = 'done', verdict = sqlc.arg(verdict)::text, scores = sqlc.narg(scores)::jsonb,
        model = sqlc.narg(model), needs_review = sqlc.arg(needs_review)::bool,
        checked_at = now(), lease_until = NULL
    WHERE report_id = sqlc.arg(report_id) AND status = 'pending'
    RETURNING report_id
)
UPDATE reports r
SET moderation_state = sqlc.arg(moderation_state)
FROM fin
WHERE r.id = fin.report_id AND r.moderation_state <> 'removed';
```
KeepReport's main UPDATE does not read its CTEs, so it may lock the report row before the check row, the reverse of this statement, and a deadlock between the two is then possible. Postgres aborts one side and both outcomes are safe: a Keep 500 that the moderator retries answers an honest 409, and a worker error lets the lease lapse on a check that is already closed. To remove the deadlock too, make Keep take the check row first as well, for example a transaction that runs SELECT ... FOR UPDATE on the check row before KeepReport. One statement also removes the crash window that the "state first, check second" ordering in `screenRow` exists to handle. Add a store test that runs Keep inside an open transaction and the worker write concurrently.

### WR-S02: The network backstop is one small bucket per address shared by every account and every action

**File:** `internal/api/handlers/ratelimit_gate.go:150-159, 204, 237, 287`; `cmd/server/main.go:94-95`; `internal/api/router.go:235`
**Issue:** `reserveIPBackstop` takes from one `ipBackstop` bucket keyed only on the client address, and it is called by report submit, all five vote routes and the flag route. The shape is 60 at once, then 1 per 20 seconds (3 per minute sustained), never refunded. CONTEXT D-10 records the backstop as "very loose", and CLAUDE.md names carrier NAT as the India specific collateral risk; the numbers are a research default, not one of the owner's D-11 allowances. Scenario: during a flood, a carrier NAT IPv4 address fronts many phones. Three active voters behind it confirm and dispute 20 reports each (60 tokens). From then on every account behind that address, including people who have not acted at all, gets one action per 20 seconds, and the 429 says "You're voting very quickly" to a person who voted once. It is off until `CLIENT_IP_HEADER` is set, but applies the moment the header is set on the real host.
**Fix:** make the backstop genuinely loose relative to a shared address, for example a burst in the low thousands with a refill of several per second, or split it per action family so votes cannot starve report submission. Use a neutral message for a backstop refusal instead of the personal "You're voting very quickly". Record the chosen numbers as an owner decision beside D-11.

### WR-S03: Email addresses reach the server log on any database error in sign in (pre-existing)

**File:** `internal/service/auth.go:175, 282, 286`, logged by `internal/api/handlers/auth.go:179, 232`
**Issue:** `RequestLink` wraps a cooldown claim failure as `"claiming email cooldown for %s: %w"` with the normalised address, and `VerifyToken` wraps account errors as `"resolving account for %s"` and `"reading resolved account for %s"`. The handlers log these with `%v`. A database blip during an emergency therefore writes each visitor's email into a log the host stores, which contradicts the project privacy goal that no email reaches a log (review focus item 6). This code predates Phase 4 (commit ddab5fa), but it sits in a listed file.
**Fix:** drop the address from the wrapped messages, for example `fmt.Errorf("service: claiming email cooldown: %w", err)`, and the same for the two account reads. A unit test can assert the error string holds no `@`.

## Info

### IN-S01: The take back shares the 30 second resolution cooldown, so the 8 second toast Undo always fails (owner veto item)

**File:** `internal/api/router.go:442, 451`; `internal/api/handlers/ratelimit_gate.go:256-261`; client: `web/static/js/app.js:175`
**Issue:** sharing the `resolution` cooldown key between resolve and the take back is a RECOMMENDED DEFAULT (04-CONTEXT.md line 293), so it is listed here and not as a defect. Its consequence conflicts with the Phase 3.1 owner answer ("a short Undo on the message"): after a successful Mark resolved, the toast Undo is visible for `TOAST_ACTION_MS` = 8000 ms, but `voteChangeKey(acct, report, resolution)` blocks any resolution vote for 30 s, so every toast Undo answers 429 `rate_vote_change` "You changed this vote a moment ago". `TestVoteRateLimitChangeCooldown` already shows resolve then reopen giving 429; unresolve uses the same key.
**Fix:** for the owner at UAT. If the short Undo should work, exempt `VoteUnresolve` from the cooldown (the undo budget of two already bounds it) or give it its own key.

### IN-S02: Moderator JSON times are emitted in the server's local zone with microseconds

**File:** `internal/api/handlers/moderation.go:83-85, 128-130`
**Issue:** `modRowJSON` carries raw `time.Time`. pgx v5 scans timestamptz into the local zone unless `ScanLocation` is set (it is not, `store.NewPool` adds no codec), so the owner's Mac emits `+05:30` and Linux CI emits `Z`, both with microseconds, while every other API time is `rfc3339Milli` in UTC. Nothing breaks today (the client parses both), but a string comparison test would differ between the two machines, which is the class of the earlier Linux only failure.
**Fix:** send strings built with `.UTC().Format(rfc3339Milli)` like `reportToResponse`.

### IN-S03: The InlineScreenTimeout comment states the wrong client timeout

**File:** `internal/service/textcheck.go:49-53`
**Issue:** the comment says "the screener client timeout is 10 s"; `moderation.requestTimeout` is 3 s (`internal/moderation/client.go:32`). The reasoning still holds, but the stated number is wrong.
**Fix:** correct the comment to 3 s.

### IN-S04: The moderation worker and the HTTP server have no graceful shutdown

**File:** `cmd/server/main.go:301-315`
**Issue:** the worker runs on `context.Background()` and the server uses `ListenAndServe` with no signal handling, so the shutdown branch in `screenRow` (`ctx.Err() != nil`) is unreachable in production. A deploy kills an in flight call; the lease lapses after 60 s and the check is screened again, so no data is lost.
**Fix:** derive the worker context from `signal.NotifyContext` and call `srv.Shutdown` on it.

### IN-S05: A retry sent while the first send is still screening can get a 429 for a report that will be stored

**File:** `internal/api/handlers/reports.go:490-519`
**Issue:** `FindReplay` sees nothing while the first send is still inside its up to 3 s text check, so the retry reserves an allowance too. If the first send took the last token, the retry answers 429 "You've posted several reports" although the first send then stores the report; a later retry replays 200. On a weak connection (the Phase 4 target) a queued row can briefly show a limit message for a report that is already posted.
**Fix:** acceptable as is; if wanted, when the allowance refuses and a request id is present, re-run `FindReplay` before answering 429.

### IN-S06: A replay of a removed report answers held true

**File:** `internal/service/report.go:506-510`
**Issue:** `replayResult` sets `Held` from `Withheld()`, which is true for removed as well as pending. A retry of a report a moderator (or a late reject) removed answers `held: true`, which the client words as waiting for a text check.
**Fix:** only the author sees this; either document it, or add a `removed` flag to the response.

### IN-S07: A worker reject is shown to the author as "removed after review" though no person reviewed it

**File:** `internal/service/modworker.go:253-254`, `internal/service/moderationcopy.go:58-60`
**Issue:** a late automatic reject sets `removed`, which resolves to `ReasonRemovedAfterReview` and the Activity sentence "This report was removed after review". For a serious report that was public while its check ran, nothing reaches the review list (`needs_review` is false on reject) and removal is terminal, so no moderator ever looks at it.
**Fix:** a copy and policy question for the owner; one option is to set `needs_review` on a worker reject of a serious report so it appears in the review list.

### IN-S08: The CLIENT_IP_HEADER startup warning understates the effect of leaving it unset

**File:** `cmd/server/main.go:188-189`
**Issue:** the warning says "limits stay per account", but the login link and geocode per IP limiters then key on the host proxy's peer address, so on Render every visitor shares one login link bucket (5, then 1 per minute). README item 5 says this correctly; the log line does not.
**Fix:** reword the warning to mention the shared login link and place search buckets.

### IN-S09: Replay lookups run before any limiter

**File:** `internal/api/handlers/reports.go:490`
**Issue:** a valid body with an unused request id costs one indexed read before the submit attempt bucket, so the lookup itself is unthrottled. The cost is small (index on account and request id) and the attempt bucket follows immediately for a miss.
**Fix:** none needed; noted for completeness.

---

_Reviewed: 2026-10-16T08:10:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
