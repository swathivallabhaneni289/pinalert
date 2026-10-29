---
phase: 09-followable-reporter-profiles
plan: 23
subsystem: service, api
tags: [notify-me, bell, mute, people-routes, swagger]
requires:
  - phase: 09-21
    provides: SetFollowNotify, TurnOffFollowNotify, CountNotifyFollows, ActiveFollowNotify, NotifyFollowsFor, NotifyFollowsCap
  - phase: 09-18
    provides: person mutes (PersonMuted, Mute)
  - phase: 09-07
    provides: the people routes, the PeopleAction bucket, peopleFailure
provides:
  - PeopleService.SetNotify with ErrNotifyUnavailable, ErrNotifyCap, NotifyCapMessage, NotifyUnavailableMessage
  - Mute turns the bell off
  - PersonProfile.Notify and the notify key of the profile JSON
  - POST /api/people/{username}/notify with NotifyRequest and NotifyResponse
affects: [09-24, 09-25, 09-26]
tech-stack:
  patterns:
    - "a switch route takes a strict body with a pointer field so a missing value is a 400, and answers the saved state"
    - "a write that can race a mute re-reads the mute after the write and undoes itself"
key-files:
  created:
    - internal/service/people_notify_test.go
    - internal/api/handlers/people_notify_e2e_test.go
  modified:
    - internal/service/people.go
    - internal/service/people_mute.go
    - internal/service/personprofile.go
    - internal/api/handlers/people.go
    - internal/api/router.go
    - internal/api/people_gate_test.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "A person the caller may not see (unknown, private, pending, deleted, blocking the caller, malformed) is ErrPersonNotFound and the one 404, as T-09-82 asks; the plan text listed 'not visible' under ErrNotifyUnavailable too, and the threat model won."
  - "Both on and off need an active follow and no mute, so off without a follow is the same 409 as on. A mute already turned the bell off, so nothing is lost."
  - "A repeat 'on' for a bell that is already on never meets the cap; only a new bell is counted."
  - "SetNotify re-reads the mute after turning a bell on; a mute that landed in between turns the bell off again and the call is refused."
  - "Profile.Notify is read only when the viewer follows and has not muted, so it is false on your own page, for anyone else, and while muted."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 23: The Notify me switch Summary

A follower can switch Notify me on and off per followed person under the follow, mute and cap-of-50 rules, the profile JSON says whether the viewer's bell is on, and muting a person turns their bell off.

## What was built

- **service/people.go**: `SetNotify(ctx, followerID, username, on)`. Order of answers: malformed, unknown or unseen person is `ErrPersonNotFound`; yourself, a person you blocked, a person you do not follow or a person you muted is `ErrNotifyUnavailable`; a new bell over `NotifyFollowsCap` is `ErrNotifyCap`; otherwise `SetFollowNotify` (zero rows after a race is `ErrNotifyUnavailable`). `PeopleQuerier` gained the four notify queries (no fake satisfies it; only `*sqlcgen.Queries`). `NotifyCapMessage` is built from the constant ("You can turn on Notify me for up to 50 people."), `NotifyUnavailableMessage` is "Couldn't change this. Try again.".
- **service/people_mute.go**: `Mute` calls `TurnOffFollowNotify` for the pair after writing the mute; `Unmute` leaves the bell off.
- **service/personprofile.go**: `PersonProfile.Notify`, filled from `ActiveFollowNotify` for another person's profile only while the viewer follows and has not muted.
- **handlers/people.go**: `NotifyPerson` (strict body `{"on": bool}`, `on` required, 400 field `on` otherwise; 200 `{"notify": bool}`), `NotifyRequest`, `NotifyResponse`, `PersonProfileResponse.notify`, and `peopleFailure` maps the two new errors to 409 with field `notify`.
- **router.go**: `POST /api/people/{username}/notify` in the PeopleAction bucket. Swag regenerated; the `@license.name` line still reads "Unlicensed (all rights reserved)".

## Tests

- New (service, database): `TestSetNotifyRules` (on, off, repeat keeps the first time, fresh time after off, profile field, not following, muted, blocked, yourself, unseen people, cap of 50 with a repeat at the cap and room after one is turned off), `TestMuteTurnsBellOff`, `TestUnfollowForgetsBell`, `TestBellEndsWithEveryFollowEnd` (unfollow, block by either side, Remove follower, Remove all followers, profile switched off, sweep tag_lost: each leaves `ActiveFollowNotify` false and `NotifyFollowsFor` without the followee).
- New (e2e): `TestNotifyRoutes` (follow then on, profile true, off, 409 without a follow, mute hides and refuses, Unmute leaves it off, the 51st bell with 49 seeded, strict bodies, the byte-identical 404 for private, unknown and malformed names).
- Amended: `TestAccessGateBlocksPeopleRoutes` and `TestSwaggerSpecCoversRoutes` list the new route.
- Full run on the scratch database p9_23_a: `go build ./...`, `go vet ./...` and `go test ./... -p 1 -count=1` exit 0, 20 packages ok, 2333 top level tests passed, only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped. The five new tests ran with zero SKIP lines. p9_23_a was dropped.

## Deviations from Plan

**1. [Interpretation] Unseen person is ErrPersonNotFound, not ErrNotifyUnavailable**
- The plan's artifact line lists "not visible" under `ErrNotifyUnavailable`, while its Task 2 text and T-09-82 say the route answers the uniform 404 for an invisible person. I followed the threat model and Task 2; `ErrNotifyUnavailable` covers the follow, mute, block and self cases.

**2. [Rule 2 - Missing critical] The mute is re-read after a bell is turned on**
- **Issue:** a mute landing between the first mute read and the write would leave a muted person's bell on.
- **Fix:** after `SetFollowNotify(on)` the mute is read again; if present the bell is turned off and the call returns `ErrNotifyUnavailable`. The profile also reports Notify false while muted.
- **Files modified:** internal/service/people.go, internal/service/personprofile.go
- **Commit:** 0138a5ff

Other: the plan listed the route handler in handlers/people.go, so it is there (the mute handlers live in people_mute.go). The e2e test seeds the 49 extra bells with one SQL statement instead of 50 API calls.

## Commits

- 0138a5ff: feat(09-23): add SetNotify, the mute rule and the profile Notify field
- 3077b313: feat(09-23): add the Notify me route and the profile notify field

## Notes for 09-24 (the bell button)

- Read `notify` from the profile JSON. It is false while muted, so the bell hides on `muted` (UI-SPEC 1e) and shows only when `following && !muted`.
- POST `/api/people/{username}/notify` with `{"on": true|false}`; success is 200 `{"notify": bool}`. A 409 carries `error.field = "notify"` and the sentence to show in the inline error: "Couldn't change this. Try again." or "You can turn on Notify me for up to 50 people.". A 404 is the uniform profile not found.
- Off by default; Unfollow, Mute, Block, Remove follower and the sweep all leave it off, and following again starts off.
- Settings has no Someone I follow posts switch (D-11); nothing here adds one.
- For 09-22 and 09-25: the send time check stays `ActiveFollowNotify` plus the mute check; this plan only guarantees the bell column is cleared on every follow end and on Mute.

## Known Stubs

None.

## Threat Flags

None. T-09-80 (the follow row is keyed by the gate's account id, never a request value), T-09-81 (cap of 50 enforced in SetNotify, tested at 50 and 51) and T-09-82 (the uniform 404, tested byte for byte) are mitigated as planned.

## Self-Check: PASSED
