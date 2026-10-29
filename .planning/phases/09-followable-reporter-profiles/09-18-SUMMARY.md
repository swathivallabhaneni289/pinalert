---
phase: 09-followable-reporter-profiles
plan: 18
subsystem: people
tags: [mute, person-mutes, alerts, postgres, sqlc, go]
requires:
  - phase: 09-02
    provides: people.sql, person_blocks, SeedPerson, SeedBlock, SeedFollow
  - phase: 09-04
    provides: PeopleService, PeopleQuerier, publicFace, idByUsername, PersonProfile
  - phase: 09-09
    provides: ReportView.ReporterAccount()
  - phase: 09-13
    provides: the deletion order through DeleteAccountRatings
provides:
  - migration 00022 (person_mutes)
  - person_mutes.sql with 7 queries
  - PeopleService.Mute, Unmute, MutedPeople, MutedPeopleCount and MutedPeopleListCap
  - PersonProfile.Muted
  - the Reliable reporter alert skipping muted people
  - account deletion that removes the mutes the account made and the mutes of it
affects: [09-19, 09-26, 09-27, 09-29]
tech-stack:
  added: []
  patterns:
    - "a mute is a one way row read only by its owner; it filters person level notes and nothing that lists reports"
key-files:
  created:
    - internal/store/migrations/00022_add_person_mutes.sql
    - internal/store/queries/person_mutes.sql
    - internal/store/sqlc/person_mutes.sql.go
    - internal/service/people_mute.go
    - internal/service/people_mute_test.go
    - internal/service/updatesalerts_mute_test.go
  modified:
    - internal/store/sqlc/models.go
    - internal/store/migrations_test.go
    - internal/testutil/db.go
    - internal/testutil/people.go
    - internal/service/people.go
    - internal/service/personprofile.go
    - internal/service/updatesservice.go
    - internal/service/updatesalerts.go
    - internal/service/updatesservice_test.go
    - internal/service/accountdelete.go
    - internal/service/accountdelete_test.go
key-decisions:
  - "Migration number taken: 00022 (newest on main was 00021_add_profile_ratings.sql)."
  - "The list cap constant is MutedPeopleListCap (500), not MutedListCap, because MutedListCap = 100 already exists for muted reports (see Deviations 1)."
  - "MutedPeopleIDs is read once per refresh, and only when Nearby returned at least one report, so a switched off or empty refresh costs no extra read."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 18: Person mute (server side) Summary

A reader can now mute and unmute a person; a muted person's reports raise no Reliable reporter note for that reader, and nothing else changes: no feed, map, list or profile hides a report, the Critical phone path is untouched, and changes to reports the reader posted or voted on still arrive. Routes and screens are the next wave.

## What was built

- **Migration 00022** (`00022_add_person_mutes.sql`): `person_mutes (muter_id, muted_id, created_at)`, primary key `(muter_id, muted_id)`, `person_mutes_not_self` CHECK, and `idx_person_mutes_muted` for the deletion of rows that name the muted side. Header names D-12, G4 and the `report_mutes` name collision; exact Down. `TestMigration00022` covers the file (markers, no CONCURRENTLY, no `report_mutes` in code), the refusals (23514 self, 23503 unknown account, 23505 repeat), the opposite pair being a separate row, Down and Up again. `person_mutes` is in `testutil.Truncate` (the list guard test still passes).
- **person_mutes.sql** (7 queries): `MutePerson :exec` (ON CONFLICT DO NOTHING), `UnmutePerson :execrows`, `PersonMuted :one`, `MutedPeople :many` (live accounts with a username, private profiles included, newest first), `CountMutedPeople :one`, `MutedPeopleIDs :many` (no join, no limit), `DeleteAccountPersonMutes :execrows` (both directions). Each comment names D-12.
- **people_mute.go**: `Mute` (same visibility rule as `Block`: malformed, unknown, private, pending, self and a person who blocked you are one `ErrPersonNotFound`), `Unmute` (by `idByUsername`, so any live account, a well formed unknown name is nil, only a malformed one is `ErrPersonNotFound`), `MutedPeople` (names only while the screen is clear), `MutedPeopleCount`. A mute ends no follow.
- **personprofile.go**: `PersonProfile.Muted`, filled with `PersonMuted` for another person's visible profile only. False on your own page, on the Blocked by you face, and when the muted person looks at the muter (T-09-66).
- **updatesalerts.go**: `refreshReliableAlerts` reads `MutedPeopleIDs` once and skips a candidate whose `ReporterAccount()` is in the set, next to the existing own, follow and report mute skips. A read error returns like the other reads there. `UpdatesQuerier` gained `MutedPeopleIDs`.
- **accountdelete.go**: `DeleteAccountPersonMutes` runs right after `DeleteAccountRatings` and before `RevokeAccountSessions`. `wantDeleteOrder` is now 19 steps and `wantDeleteOrderKeyed` 20, with dated amendment notes; the fake gained the method.
- **testutil.SeedMute**.

## Tests

`TestMigration00022`; `TestMuteRules`, `TestMuteNeverHidesReports` (Nearby with All, Focus, History and ProfileReports list the same before and after a mute, and the profile still opens), `TestMutedPeopleList` (order, deleted and pending names left out, count, cap at 500 with 501 muted, mutes private to their owner), `TestProfileShowsMuted`, `TestDeleteAccountRemovesPersonMutes` (in `people_mute_test.go`); `TestReliableAlertSkipsMutedPerson` (muted reporter skipped for low and critical reports, unmuted reporter alerts, unmuting brings the same report back, another person's mute does not silence this reader), `TestCriticalStillArrivesFromMutedPerson` (dispatcher queues a `critical` row for a muted author; a `reached_live` changes row is still written for a report the reader voted on), `TestReliableAlertMutedReadFailure` (fake) in `updatesalerts_mute_test.go`. Each ran once with `-v` and printed its name.

Full run on the scratch database `p9_18_a` (created fresh, dropped at the end): `go test ./... -p 1 -count=1` exits 0 for every package; the only skips are the three known ones (`TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture`). `go build ./...` and `go vet ./...` are clean, and `sqlc generate` leaves no diff.

## Deviations from Plan

**1. [Rule 3 - Blocking] `MutedListCap` was already taken.** The plan said `MutedListCap = 500` in `people_mute.go`, but `internal/service/updates.go` already declares `MutedListCap = 100` (the cap on muted reports, used by `mutes.go` and `mutes_test.go`), so the package did not compile. The person constant is `MutedPeopleListCap = 500` with a comment saying why. Later plans (the route and the Muted people screen) must use `MutedPeopleListCap`.

**2. [Minor] The muted set is read only when Nearby returned a report.** The plan says "read once per refresh". The read sits after the Nearby read and runs only if there is at least one view, so a refresh that has nothing to judge makes no extra query. It is still once per refresh.

**3. [Minor] The fake of `UpdatesQuerier` records nothing for `MutedPeopleIDs`.** `fakeUpdatesQuerier.MutedPeopleIDs` answers an empty set without recording, so no existing call order assertion in `updatesservice_test.go` or `updatesalerts_test.go` changed. `TestReliableAlertMutedReadFailure` uses a small wrapper fake for the error case.

No auth gates. No threat flags beyond the plan's register: T-09-65 is covered by `TestMuteNeverHidesReports` and `TestCriticalStillArrivesFromMutedPerson`, T-09-66 by `TestProfileShowsMuted` and `TestMutedPeopleList` (only the owner reads the list), T-09-67 is accepted as planned.

## Interface widening

- `PeopleQuerier` widened by five methods (`MutePerson`, `UnmutePerson`, `PersonMuted`, `MutedPeople`, `CountMutedPeople`); the only satisfier is `*sqlcgen.Queries` (no fake exists).
- `AccountDeleteQuerier` widened by `DeleteAccountPersonMutes`; satisfiers: `*sqlcgen.Queries` and `fakeDeleteQuerier` in `internal/service/accountdelete_test.go`.
- `UpdatesQuerier` widened by `MutedPeopleIDs`; satisfiers: `*sqlcgen.Queries`, `fakeUpdatesQuerier` in `internal/service/updatesservice_test.go` (and `alertFake` and the new `mutedErrFake`, which embed it). `internal/api/router.go` and `cmd/server/main.go` pass the real store and needed no change. No fake in `feed_test.go`, `auth_test.go`, `trust_test.go`, `votes_body_test.go` or `internal/demo/memquerier.go` satisfies these three interfaces.

## Notes for 09-19 and later

- Use `MutedPeopleListCap`, not `MutedListCap`, for the Muted people list. `MutedPeople` returns `[]PersonEntry` (clear names only); `MutedPeopleCount` counts live named accounts including a pending name, so the count can exceed the list by the pending ones (same as Blocked).
- Routes should map `ErrPersonNotFound` to the uniform 404 for both Mute and Unmute. `Unmute` of a well formed unknown name succeeds, so the undo is always safe to repeat.
- `PersonProfile.Muted` is set only on another person's visible profile; the menu item reads it to show Mute or Unmute.
- Plan 09-26 (the follow note): skip muted people with `MutedPeopleIDs` at queue time and at the worker's send time check (T-09-67). The Reliable alert already queued for the phone before a mute may go out once (accepted).
- Policy: person mutes are new personal data (who muted whom). The Policy text and its date (2026-10-29) are not changed here. The Policy plan must say mutes are private to the person who made them, are never shown to the muted person, and are removed with either account on deletion.
- Plan 09-27 docs: person mute stops person level notes only; it is deliberately not applied to the Critical push, to changes for reports the reader posted or voted on, or to any feed, map, list or profile (D-12).
- `gofmt -l` reports four files that this plan did not touch (`internal/api/handlers/comments.go`, `internal/api/handlers/place_test.go`, `internal/geocode/photon.go`, `internal/store/comments_test.go`); left alone as out of scope.

## Known Stubs

None.

## Self-Check: PASSED

All created files exist (`00022_add_person_mutes.sql`, `person_mutes.sql`, `person_mutes.sql.go`, `people_mute.go`, `people_mute_test.go`, `updatesalerts_mute_test.go`); commits 9f70190f, c4342c4b and f49ed4e6 are in `git log`.
