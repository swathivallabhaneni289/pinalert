---
quick_id: 261119-sps
slug: update-rows-show-the-report-area-name
date: 2026-11-19
status: complete
commits:
  - 90da6386 fix(quick-261119-sps): the area lookup claim is one conditional write
  - 11f0a550 feat(quick-261119-sps): update rows show the report's area name
  - 8a3a868e docs(quick-261119-sps): the area name on update rows
---

# Quick 261119-sps: update rows show the report's area name

Bell rows and Updates screen rows now read "Road blocked · Low · Benz Circle", using the same coarse
name phone alerts use. A report with no name yet gets its one lookup in the background after a signed
in read, so the name shows on the next refresh.

## What changed

- **The claim.** `MarkAreaLookupTried` is now `:execrows` with
  `ON CONFLICT ... DO UPDATE ... WHERE area_lookup_at IS NULL`. It claims the lookup only when it was
  never tried, and that includes a progress row that already exists with no attempt.
  `LookUpReportAreaName` calls out only when it won the claim. Phone alerts, Share and the update rows
  all go through it, so a report is looked up once ever, also when two callers ask at once.
- **The row.** `ReportsForUpdates` also reads `area_name` and whether the lookup was tried (LEFT JOIN
  `push_report_progress`, still no coordinate). `UpdateItem.Area` holds the stored name cut by
  `AreaWords`.
  - Absent for: an unavailable report, a finished Rescue report (the `CoarsenFinishedRescue` rule),
    the rolled up row, and a row not about one report.
  - The reader's own report rows carry it while the report is not withheld. They have no type line,
    so nothing is drawn.
- **Wire and label.**
  - `UpdateWire.area` is a new omitempty field, and swagger is regenerated (`@license.name`
    untouched, no dashes).
  - The label reads "{Category}, {Severity}, {Area}.".
  - A follow row's label already says category and severity in its report line, so its area joins
    that sentence: "Flood, Critical, Benz Circle.".
  - No area, no change to the label.
- **Where it shows.**
  - The bell (`updates.js`, new `typeLine`) and the Updates screen (`profile.html.tmpl`) both show
    "{category} · {severity} · {area}", or "{category} · {area}" when there is no severity.
  - The line is unchanged when there is no area, and there is no area when there is no type line.
- **The background lookup** (`internal/service/updatesarea.go`).
  - `GetUpdates` calls `StartAreaLookups(page.AreaMissing)` after `writeJSON`. `Profile` calls it
    only on the Updates screen and only after the template is written.
  - At most 3 lookups per read, run one after another on their own `context.Background()` with a
    bounded budget, 1.1 s apart. The geocoder refuses (`Allow`) rather than waits, and a refused
    lookup is already claimed, so parallel calls would leave reports nameless for good.
  - A failure logs the report id only. Only the report's own point is sent.
  - It is wired through the new `Deps.AreaNamer`, which is the one geocode client in `main.go`. A
    nil namer looks nothing up. The Demo and the public pages never reach it.
- **Docs.**
  - `docs/PUSH.md` and `docs/TRUST-MODEL.md` (the "What is never sent" paragraph and the "Area name
    amended" bullet) now name the update rows, on the owner's word of 2026-11-19.
  - 03.3-CONTEXT.md D-19 and D-20 have dated amendments.
  - Code comments that said "push only" or "never by the feed" are updated (`areaname.go` header,
    `pushcopy.go`, `push.sql`).

## Tests

- `go build ./...` and `go vet ./...` are clean.
- `go test ./... -p 1 -count=1 -v` on the scratch database `p9_sps`: 2540 top level tests passed,
  0 failed, 3 skipped. The skips are the existing live network and fixture tests (TestGeocodeLive,
  TestOfficialLive, TestScreeningFixture). All 22 packages ok.
- `node --test web/*.mjs`: 1715 passed, 0 failed.
- New and amended tests:
  - `TestListArea`: which rows carry the area, AreaWords cut, and the AreaMissing list.
  - `TestStartAreaLookups`:
    - caps at 3;
    - the caller does not wait (a blocking namer);
    - runs one at a time;
    - a claimed report is skipped;
    - a failing namer is not retried;
    - no namer touches nothing.
  - `TestLookUpReportAreaName`: two new cases for the claim.
  - The store claim test in `TestPushReportProgressAndRecent`: 1, then 0, and an existing untried
    row is still claimed.
  - Label, wire and Updates part cases in `TestUpdateRowLabel`, `TestUpdateWire` and
    `TestFillUpdatesPart`.
  - Over HTTP on a real Postgres:
    - `TestUpdatesAreaShowsOnTheNextRead`: the first read has no area while the namer is blocked,
      the next read has the area and the label, the Updates screen shows it, and the namer got the
      report's point.
    - `TestUpdatesAreaTwoReadsAtOnceLookUpOnce`: four parallel reads, one call.
    - `TestUpdatesAreaFailedLookupIsNotRetried`.
    - `TestUnauthenticatedRoutesNeverCallTheNamer`, which extends the D-06 guard: bell, Updates
      screen, profile, Demo, `/r/{token}` and login without a session make no call.
  - A node test for the bell's type line in `web/updates_test.mjs`.

## Live look

- Own server on port 8789 with database `p9_sps_live`, ENV=development, log mailer.
- Four made up `sps-*@example.com` accounts. Onboarding was finished by SQL on the scratch database.
- Two reports in Vijayawada went Live from three confirmations each, giving the follower B two
  "Now confirmed" rows. Push was off, so the bell was the only caller of the lookup.
- The real geocoder answered:
  - First read: `Road blocked · Low` and `Flood · Medium`. The API answer had no area.
  - Next refresh (6 s later): `Road blocked · Low · Gurunanak Nagar` and
    `Flood · Medium · Patamata Lanka`.
  - The labels read "..., Road blocked, Low, Gurunanak Nagar. ...".
  - Each report was looked up once and both names were stored.
- Playwright WebKit at 390 (touch) and 1280, light and dark, on the bell list and the Updates
  screen: every type line fits on one line, nothing is clipped, and the page never scrolls sideways
  (scrollWidth equals clientWidth at both widths).
- Screenshots are in
  `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/sps/shots/`,
  numbered 01 to 10:
  - 01 is the first read and 02 the next read.
  - 03 to 06 are 390 light and dark, bell and Updates screen.
  - 07 to 10 are 1280 light and dark, bell and Updates screen.
- Torn down afterwards: the server was stopped, `p9_sps` and `p9_sps_live` were dropped, and the
  log and cookie jars were deleted.

## Deviations

- **Policy unchanged.** `legal_body.html.tmpl` has no sentence saying a report's location is sent to
  the place search service to name its area. Its only place search line is about typed searches. As
  the plan says, nothing was changed there.
- **Existing query made conditional instead of adding a new one.** The claim reuses
  `MarkAreaLookupTried`, now conditional, and its two test callers were updated. No migration.
- **Lookups run one after another, 1.1 s apart, not in parallel.** The plan does not specify this.
  Without it the shared one request a second geocoder would refuse the 2nd and 3rd lookup of a read,
  and those reports would stay nameless forever.
- **Old migration comment not edited.** The comment in migration 00011 still says the area name is
  "never in a bell row". It was left as is, because the rules forbid touching migrations.
- **Own report rows carry `area` on the wire but draw nothing.** They have no type line, and their
  label is unchanged.
