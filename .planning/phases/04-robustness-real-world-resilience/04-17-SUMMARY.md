---
phase: 04-robustness-real-world-resilience
plan: 17
subsystem: api
tags: [idempotency, request-id, written-at, swagger, feed, popup]
requires:
  - phase: 04-05
    provides: InsertReportIdempotent, ReportByRequest, the partial unique index
  - phase: 04-07
    provides: ReportView.WrittenAt, WrittenAtForDisplay, the 30 minute threshold
  - phase: 04-13
    provides: updatedAgo and the stale line in app.js
  - phase: 04-15
    provides: the pinned POST order, the stored flag and the deferred allowance refund
provides:
  - request_id retry key: a repeat from the same account answers 200 with the stored report and stores nothing
  - written_at display only time, stored clamped, sent to readers only past 30 minutes
  - Pinalert.writtenLine, the one Written line formatter, on the feed row and the popup
affects: [04-22 offline queue, 04-18, 04-27 UAT]
tech-stack:
  added: []
  patterns:
    - replay lookup before any bucket (step 3), replay at insert refunds through the existing deferred refund
    - one JS formatter guarded by a source contract test
key-files:
  created:
    - internal/service/requestid_copy_test.go
    - internal/api/handlers/submit_idempotent_e2e_test.go
    - web/written_line_contract_test.go
  modified:
    - internal/service/report.go
    - internal/service/feed_test.go
    - internal/api/handlers/reports.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/app.js
    - web/static/js/feed.js
    - web/static/js/map.js
key-decisions:
  - "A replay row is mapped through the existing insert row mapper by a struct conversion, so no createdFromRequestRow was needed"
  - "A replay's held flag is ReportMeta.Withheld of the stored row, the same predicate a first send uses"
  - "Written time is clamped twice: once in the handler on the parsed value, once in SubmitChecked against the same now that stamps created_at"
requirements-completed: [ROBUST-02]
duration: about 1h 15m
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 17: Retries never store twice, and late reports say when they were written Summary

**A post retried with the same request_id is stored once and costs no allowance, and a report that reached the server more than 30 minutes after it was written shows "Written 4 hours ago" on its feed row and popup while its expiry still counts from upload.**

## Accomplishments

- `ValidateSubmitInput` rejects a malformed `request_id` (16 to 64 letters, digits, hyphens) with `RequestIDInvalidMessage` on field `request_id`, as the last check.
- `FindReplay` (step 3 of the POST order) answers 200 with the stored report before any bucket, submit attempt or text check. The lookup is keyed on the verified account plus the request id, so another account's identical key stores its own report and a second session of the same account finds the first send.
- A repeat found only at insert (two sends racing) comes back from `InsertReportIdempotent` with no row, `SubmitChecked` turns it into `Replayed` via the account scoped lookup, and the handler answers 200 and leaves `stored` false so 04-15's deferred refund gives the allowance back at its reservation instant.
- `written_at` is parsed leniently (unreadable gives NULL), clamped in the handler and again in `SubmitChecked` against the same `now` that stamps `created_at`; a future value becomes the server time and one older than 14 days becomes exactly 14 days before `created_at`. `created_at` and `expires_at` never read it.
- `GET /api/reports` carries `written_at` only when 04-07's threshold says so. `Pinalert.writtenLine(report)` formats it once from `relativeTime`; `feed.js` shows it as a second `p.report-row__meta` right after the meta line, `map.js` as a hidden `p.popup-card__meta-line` between the meta line and the shelter line.
- Swagger regenerated once with `make swag`: `request_id` and `written_at` on `SubmitReportRequest`, `written_at` on `FeedReportResponse`, and the 200 response on `POST /reports`.

## Task Commits

1. Task 1, request_id replay: `2075681`
2. Task 2, written_at and the Written line: `3aa1966`
3. SUMMARY: committed after this file (see the final report for the sha)

## Step 0 records

- `InsertReportIdempotent` was already on `service.Querier` and `SubmitChecked` already called it (04-11/04-05 left it so). Only `ReportByRequest` was added to `Querier`.
- `Held` for a new report: `ReportMeta{Severity, Category, Moderation}.Withheld()` computed after the insert in `SubmitChecked`; the replay paths use the same expression via `replayResult`.
- `SubmitResult.Replayed bool` was already declared by 04-11, so it was not added.
- Mapper from the insert row: `createdReportFromInsertRow`; handler mapper from `SubmitResult` to the wire shape: `reportToResponse`. The replay row converts with `sqlcgen.InsertReportIdempotentRow(row)` and compiled, so `createdFromRequestRow` was not added.
- Types passed as a `service.Querier` in `internal/service/*_test.go`: `fakeFeedQuerier`, plus `fakeSubmitQuerier` and `p05VotingFake` by embedding. Only `fakeFeedQuerier` needed the stub.
- `newE2EServerWith` returns `srv, mailer, pool`; `api.RateLimits` fields are `ReportSerious`, `ReportOther`, `Vote`, `VoteChange`, `Flag`, `SubmitAttempt`, `IPBackstop`.
- Built web names found: `createRow(id)`, `updateRow(row, report)`, `refreshAgeAndTime()`, `buildMetaText(report)` in `feed.js`; `buildPopupCard(report)`, `updatePopupCard(card, report)`, `metaLines`, `shelterLine` in `map.js`; `relativeTime(iso)`, `updatedAgo(timestampMs)` in `app.js`; `stripJSComments` in `web/js_sinks_contract_test.go`.
- No CSS rule on `report-row__meta` or `popup-card__meta-line` sets `display` (checked `main.css`, `feed.css`, `phone.css`, `trust.css`), so `hidden` works on the new paragraphs.
- Baselines before the work: no `'Written '` literal under `web/static/js/`; `http.StatusOK` count inside `SubmitReport`: 0; now 2.

## Files Created/Modified

See the frontmatter. `docs/*` are generated by `make swag` only.

## Decisions Made

See `key-decisions`. One more: `p17MilliISO` lives in Task 1's commit rather than Task 2's, because `answer_has_no_internal_fields` (Task 1) needed it; the three Task 2 helpers (`p17StoredTimes`, `p17FeedEntry`) and the Task 2 tests were added in Task 2. The `written_at` field was added to that Task 1 test body in Task 2, since the strict decoder would otherwise answer 400.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `feed_test.go` stub**
- **Issue:** `Querier` gained `ReportByRequest`, so `fakeFeedQuerier` no longer satisfied it.
- **Fix:** one stub method with its comment, as the plan describes. This is the listed `feed_test.go` deviation; nothing else in that file changed.

**2. [Rule 1 - Test correctness] Expiry test compares instants, not strings**
- **Found during:** Task 2 first run.
- **Issue:** the existing `reportToResponse` writes `created_at` and `expires_at` in the server's local zone (the machine runs at +05:30), so comparing against a UTC formatted string failed.
- **Fix:** `TestWrittenAtNeverFeedsExpiry` parses the answer and compares the instant at millisecond precision. The handler's behavior is unchanged (not in scope).

**3. [Style] `ClientWrittenAt: writtenAtParam(writtenAt)` sits after a blank line and comment in the insert literal**
- Reason: gofmt aligns struct fields in one block, which would put extra spaces in the literal the acceptance text greps for. The blank line starts a new alignment block.

**4. Helper names in `written_line_contract_test.go` are `p17` prefixed** (`p17ReadStaticJS`, `p17WantAll`, `p17WantNone`, `p17WantInOrder`) so a sibling wave 5 test file in package `web` cannot collide with them.

No other test file was re-pointed. `swagger_test.go`, `reports_e2e_test.go` and `gate_test.go` are unchanged.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l` clean.
- `go test ./... -count=1` (no database): all packages ok.
- Database run on the disposable `p4_0417` (created fresh, dropped afterwards): `DATABASE_URL=postgres://localhost:5432/p4_0417?sslmode=disable go test ./internal/... ./web/ -count=1 -p 1 -v` gave 758 PASS lines and 0 FAIL. The one SKIP is `TestScreeningFixture` (no recorded scores until plan 04-27; it is not database backed), so database backed SKIP count is 0.
- `-race` run of `TestSubmitIdempotent*`, `TestWrittenAt*`, `TestReportRateLimit*`, `TestSubmitScreening*`, `TestSubmitThenNearbyReturnsReport`, `TestSwagger*` on `p4_0417`: all PASS.
- Mutation check: setting `stored = true` before the `Replayed` early return made `TestSubmitIdempotentReplayCostsNoAllowance` and `TestSubmitIdempotentConcurrent` fail (the refund is really exercised); reverted.
- `swag init` into a temporary directory is byte equal to the committed `swagger.json` and `swagger.yaml`; `docs.go` differs only by the package line, which follows the output directory name.
- Test-first note: the service tests and `TestClampWrittenAt` were run RED (compile failure) before the code. For the e2e tests the handler and service code were written before the test file, so the e2e RED state was not observed separately; the mutation check above stands in for it. `TestSubmitIdempotentSwaggerFields` was observed failing before `make swag`.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the optional `request_id` and `written_at` fields on the existing route, covered by T-04-05, T-04-17, T-04-01, T-04-03 and T-04-09 in the plan.

## For the orchestrator and the owner

- Human check V-24 (04-27 UAT) is still to do, on a disposable database and port 8792, never run by this plan: the compact feed row shows "Flood · just now" followed by "· Written 4 hours ago" on the same wrapping line (it inherits the meta paragraph's `order: 3` and dot separator from `feed.css`, no CSS was added), the popup shows the same words as a second line under the category and time line, a resend with the same `request_id` adds no row or pin, and at 320 px the line wraps inside the row. This plan owns no CSS, so a spacing tweak, if the owner wants one, belongs to a later plan.
- The offline queue that sends `request_id` and `written_at` by itself is 04-22.

## Self-Check: PASSED

Created files exist (`requestid_copy_test.go`, `submit_idempotent_e2e_test.go`, `written_line_contract_test.go`), commits `2075681` and `3aa1966` exist on `worktree-agent-aae34150c3503372f`, and all checks in Verification passed.
