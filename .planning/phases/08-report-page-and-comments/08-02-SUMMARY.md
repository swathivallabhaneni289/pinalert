---
phase: 08-report-page-and-comments
plan: 02
subsystem: share
tags: [share, public-status, migration, sqlc, area-name]
requires:
  - phase: 05
    provides: PushDispatcher.ensureAreaName, AreaNamer
  - phase: 07.1
    provides: password sign in test helpers (testutil.SeedReporterAccount)
provides:
  - reports.share_token (migration 00017)
  - ShareService.EnsureLink and ShareService.PublicStatus
  - LookUpReportAreaName shared by phone alerts and Share
  - Share words (ShareFixedLine, ShareTitle, PublicAgo and the public page constants)
affects: [08-03, 08-04]
tech-stack:
  added: []
  patterns: [first-write-wins capability token, one shared bounded outside lookup]
key-files:
  created:
    - internal/store/migrations/00017_add_report_share_token.sql
    - internal/store/queries/share.sql
    - internal/store/sqlc/share.sql.go
    - internal/service/areaname.go
    - internal/service/areaname_test.go
    - internal/service/share.go
    - internal/service/sharecopy.go
    - internal/service/share_test.go
    - internal/service/sharecopy_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/sqlc/models.go
    - internal/service/pushdispatch.go
key-decisions:
  - "Only the call that minted the token may run the area lookup, so ten simultaneous taps look the area up once"
  - "A resolved report with an unknown closer reads the box line Resolved."
requirements-completed: [COORD-08]
status: complete
duration: about 1 hour
completed: 2026-10-29
---

# Phase 8 Plan 02: Share token, ShareService and the public status read

One random 22 character token per report (first write wins), a share title that carries only the category and a coarse area, and a public status read that turns a token into one of six states without ever revealing a held report.

## What was built

- Migration number used: **00017** (`00017_add_report_share_token.sql`). `reports.share_token TEXT` (nullable), `reports_share_token_shape` CHECK (`^[A-Za-z0-9_-]{22}$`), partial unique index `uq_reports_share_token`, exact Down.
- Queries in `internal/store/queries/share.sql`: `SetReportShareToken` (`WHERE share_token IS NULL RETURNING share_token`), `ReportShareFacts` (id, category, severity, happening, share_token, area_name, latitude, longitude, `area_lookup_tried` from a LEFT JOIN on push_report_progress), `ReportByShareToken` (id and moderation_state). `make sqlc` changed only `models.go` and the new `share.sql.go`.
- `internal/service/areaname.go`: `AreaNameStore`, `LookUpReportAreaName(ctx, store, namer, reportID, lat, lon, now)`. `PushDispatcher.ensureAreaName` keeps its `areaTry`, `areaDone` and `AreaName` checks and calls it; the `strings` import left pushdispatch.go with the moved body.
- `internal/service/share.go`: `ShareQuerier` (with the `sqlcgen.Queries` assertion), `ShareService`, `ShareOption`, `WithShareAreaNamer`, `WithShareClock`, `WithShareTokenSource`, `NewShareService`, `NewShareToken` (crypto/rand, 16 bytes, base64.RawURLEncoding), `ValidShareToken`, `ShareTokenLen`, `ShareLink`, `EnsureLink`, `PublicState` and its six constants, `PublicStatus`, `ShareService.PublicStatus`.
- `internal/service/sharecopy.go`: `ShareFixedLine`, `ShareTitle`, `PublicChipActive/Disputed/Resolved/Ended`, `PublicRemovedTitle`, `PublicUnavailableTitle`, `PublicTooManyTitle`, `PublicTooManyLine`, `PublicEndedSentence`, `PublicAgo`.

## For plans 08-03 and 08-04

- `PublicStatus` returns `PostedAt` and `EndedAt`; the handler computes the words with `PublicAgo(then, now)`. The service has no clock for that on purpose.
- Active sentence is `view.Trust.ConfirmedSentence` when `ConfirmedPlaces > 0`, else `UnconfirmedLine`. Disputed is `DisputedSentence`. Resolved is the `closedText` words (viewer 0, so never "you") plus a full stop. Ended is `PublicEndedSentence`.
- The title is built from the stored area name only. Nothing in the public path calls the namer.
- `EnsureLink` answers `ErrReportNotFound` for a held, removed, unknown or too old report. A handler maps it to the usual not found line.
- Token shape check is `ValidShareToken`; the handler should call it before anything else so a bad path costs nothing.
- Test helpers that 08-03 and 08-04 can copy: `newShareRig`, `report`, `voter`, `vote`, `setToken`, `padToken` in `internal/service/share_test.go` (package `service_test`).

## Deviations from Plan

### Auto-fixed or stricter than the plan

**1. [Rule 2 - Missing critical functionality] The area lookup runs only in the call that minted the token**
- **Found during:** Task 2, designing the concurrent case.
- **Issue:** the plan says to look the area up when the name is empty, the attempt is unrecorded and a namer is set. With ten simultaneous taps, a late caller can read the token the winner just wrote and see the attempt still unrecorded, so the namer is called twice. T-08-08 and the truth "looked up at most once per report" would not hold.
- **Fix:** `EnsureLink` runs `LookUpReportAreaName` only when its own `SetReportShareToken` write made the token. A call that lost the race, or finds the token already there, never looks up. The tests assert one namer call across ten simultaneous taps.
- **Consequence:** a report whose token was minted while no namer was configured is never given an area by Share. Phone alerts can still name it.
- **Files:** internal/service/share.go. **Commit:** the Task 2 commit.

**2. [Rule 3 - Blocking] Test names**
- The plan names `TestMigration<number>`; the existing file has TestMigration00016 elsewhere, so the new one is `TestMigration00017`. Added a no database test, `TestPublicStatusMalformedTokenRunsNoQuery`, for the "zero queries" truth, and made the plan's `TestPublicStatusStates` and `TestShareLink*` tests DB tests.

No other deviations.

## Verification

- `go build ./...`, `go vet ./...` exit 0; `make sqlc` leaves no further diff.
- Full run on a fresh `p8_02_a`: `go test ./... -p 1 -count=1 -v` exit 0, 5773 `--- PASS`, 0 `--- FAIL`, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture only).
- Named tests seen with `--- PASS`: TestMigration00017, TestLookUpReportAreaName, TestDispatchAreaLookupOnce, TestShareTitle, TestPublicAgo, TestValidShareToken, TestShareCopyIsPlain, TestShareLinkMintsTokenOnce, TestShareLinkWithheld, TestShareLinkAreaLookupOnce, TestPublicStatusStates.
- `go test -race -count=3 -run TestShareLinkMintsTokenOnce` passes.
- `grep crypto/rand internal/service/share.go` prints 2; code lines with `math/rand` print 0.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-08-05 to T-08-09 are mitigated as written (token shape in SQL and Go, crypto/rand only, one unavailable state for held, unknown, malformed and too old, lookup only on the authenticated call and only once, title built from category and area only).

## Self-Check: PASSED

Created files exist; commits for Task 1 and Task 2 are on the worktree branch.
