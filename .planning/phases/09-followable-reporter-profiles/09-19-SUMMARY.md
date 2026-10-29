---
phase: 09-followable-reporter-profiles
plan: 19
subsystem: people
tags: [mute, routes, swagger, go]
requires:
  - phase: 09-07
    provides: the people routes, PeopleRead and PeopleAction buckets, personAction, p907 test helpers
  - phase: 09-18
    provides: PeopleService.Mute, Unmute, MutedPeople and PersonProfile.Muted
provides:
  - POST /api/people/{username}/mute and /unmute (PeopleAction bucket)
  - GET /api/me/muted-people (PeopleRead bucket)
  - muted on the profile JSON
affects: [09-20, 09-27, 09-29]
tech-stack:
  added: []
  patterns:
    - "the mute handlers reuse personAction and peopleFailure, so errors and the uniform 404 are the Block ones"
key-files:
  created:
    - internal/api/handlers/people_mute.go
    - internal/api/handlers/people_mute_e2e_test.go
  modified:
    - internal/api/handlers/people.go
    - internal/api/router.go
    - internal/api/handlers/swagger_test.go
    - internal/api/people_gate_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "The person test is TestPersonMuteRoutes because TestMuteRoutes already names the report mute test in updates_e2e_test.go."
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 19: Person mute routes Summary

Mute and unmute are documented, rate limited routes in the gated group, the caller can read their own muted list, and the profile JSON says whether the caller muted that person.

## What was built

- **people_mute.go**: `MutePerson` and `UnmutePerson` are `personAction` over `people.Mute` and `people.Unmute` (empty body, 204, `ErrPersonNotFound` is the uniform 404). `MutedPeopleList` answers 200 with `{entries:[{username}]}` and `Cache-Control: no-store`, on the shape of `BlockedPeopleList`. `MutedPeopleResponse` is the wire type. Swag annotations use tag `people`, plain punctuation, and the 204, 401, 404, 429 answers.
- **people.go**: `PersonProfileResponse.Muted` (`json:"muted"`), filled from `PersonProfile.Muted`.
- **router.go**: the two POSTs use `PeopleActionMiddleware`, the list uses `PeopleReadMiddleware`, flat literal paths in the gated group (T-09-68 and T-09-70: the account id comes only from the gate; T-09-69: Mute refuses an invisible profile with the uniform 404).
- **Swag**: `make swag` regenerated the three docs files, additions only; the `@license.name` line still reads "Unlicensed (all rights reserved)". A second `make swag` leaves no diff. `TestSwaggerSpecCoversRoutes` pins the three paths and their answers.

## Tests

- `TestPersonMuteRoutes`: mute 204 with no body, profile `muted` true, list names the person with only a username key, repeat 204 with one row, unmute 204 and flag false, repeat unmute 204; the muted person sees `muted` false on their own page and on the muter's page and has an empty list; a private person, a pending name, a nobody and yourself on mute all answer the one uniform 404 byte equal to the profile 404 and write no row; a person who went private after the mute stays in the list and unmute removes them; unmute of a well formed unknown name is 204 and leaves the list alone (review W7); a malformed name is the uniform 404; a body with content is 400.
- `TestMuteKeepsReportsVisible`: the muted person's live report is in `GET /api/reports` for the muter, the profile opens and `/api/people/{u}/reports` lists the report, both before and after the mute.
- The three routes are in `TestAccessGateBlocksPeopleRoutes` (401 anonymously) and `TestSwaggerSpecCoversRoutes`.
- Each new test ran once with `-v` and printed its name, zero SKIP.
- Full `go test ./... -p 1 -count=1` on the scratch database `p9_19_a` (created fresh, dropped at the end): exit 0, 2322 passing tests, the only skips are the three known ones (`TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture`). `go build ./...` and `go vet ./...` are clean.

## Deviations from Plan

**1. [Rule 3 - Blocking] `TestMuteRoutes` was already taken.** `internal/api/handlers/updates_e2e_test.go` has a `TestMuteRoutes` for the report mute routes, so the package did not compile with a second one. The person test is named `TestPersonMuteRoutes`. Later plans and the validation file should use that name for the person routes.

No auth gates. No threat flags beyond the plan's register.

## Notes for 09-20

- Wire shape for the screens: `POST /api/people/{username}/mute` and `/unmute` answer 204 with no body; `GET /api/me/muted-people` answers `{"entries":[{"username":"..."}]}`, newest first, at most `MutedPeopleListCap` (500); a profile carries `muted` (bool).
- `muted` is the viewer's own state: false on your own page, on a Blocked by you face, and when the muted person looks at the muter. The menu item should read it to choose Mute or Unmute and must not show anything to the muted person.
- Unmute is safe to repeat and works for a person who went private since, so an Undo on the Muted people screen can always call it.
- The Settings row count can use `MutedPeopleCount` (it can exceed the list by the people whose new username is pending).
- `gofmt -l` still lists `internal/api/handlers/comments.go` and `internal/api/handlers/place_test.go`, which this plan did not touch.

## Known Stubs

None.

## Self-Check: PASSED

Created files exist (`people_mute.go`, `people_mute_e2e_test.go`, this summary); commits d83f6464 and 977dae3d are in `git log`.
