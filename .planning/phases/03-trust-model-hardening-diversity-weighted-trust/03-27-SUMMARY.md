---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 27
subsystem: api
tags: [validation, vote-body, nan, category-label, focus, webkit, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the A74 popup opener seam and the T-03-01 reporter exemption
provides:
  - NaN lat, lon and radius_km are a 400 field error on GET /api/reports
  - A vote body without coordinates is a 400 unless it is the reporter's own resolve or reopen
  - One categoryLabel helper, so an unknown category shows Other and never the raw value
  - A mouse close of a pin opened popup returns focus to the marker in WebKit
affects: [03-UAT live checks T9.16 and tests 2 to 10]
tech-stack:
  added: []
  patterns:
    - "Pointer fields in a request struct to tell an absent key from a real zero"
    - "Pointerdown flag that survives the browser's own focus move"
key-files:
  created:
    - internal/api/handlers/nearby_validation_test.go
    - internal/api/handlers/votes_body_test.go
  modified:
    - internal/api/handlers/geocode_test.go
    - internal/mailer/resend.go
    - web/js_contract_test.go
    - internal/api/handlers/reports.go
    - internal/api/handlers/votes.go
    - internal/service/trust.go
    - internal/service/trust_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/app.js
    - web/static/js/map.js
    - web/static/js/feed.js
    - web/popup_contract_test.go
key-decisions:
  - "IN-01 uses the narrower rule: a location is required for every vote except the reporter's own resolve and reopen"
  - "UF-03-01: one categoryLabel helper in app.js, following iconClass"
requirements-completed: []
status: complete
---

# Phase 3 Plan 27: Small review residuals bundle Summary

Five independent commits close the last small residuals of the Phase 3 reviews: formatting, NaN query values, coordinate free votes, the raw category fallback and the WebKit mouse close focus return.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 | f5289b2 | style(03-27): gofmt the three files the security review listed |
| 2 | 3e8ba86 | fix(03-27): NaN lat, lon and radius_km are a 400 field error, not an empty feed |
| 3 | d3bf109 | fix(03-27): a vote without coordinates is a 400 unless it is the reporter's own resolve or reopen |
| 4 | 0c65f0d | fix(03-27): an unknown category shows the word Other, never the raw value |
| 5 | 50092af | fix(03-27): a mouse close of a pin opened popup returns focus to the marker in WebKit too |

## What changed

- **Task 1 (T5).** `gofmt -w` on exactly geocode_test.go, resend.go and js_contract_test.go. No hand edit.
- **Task 2 (UF-03-03).** `strconv.ParseFloat` accepts the unsigned words `NaN` and `nan`, and NaN passes every `<` and `>` range check, so the feed came back empty. `parseCoordinate` and the radius_km block now also refuse `math.IsNaN(parsed)` with the existing "must be a number." messages. `TestNearbyRejectsNaNQueryValues` (14 subtests) failed on the six unsigned cases before the change and passes now; the signed forms and the controls (`lat=abc`, `lat=Inf`) passed both times.
- **Task 3 (IN-01).** `CastVoteRequest.Latitude` and `.Longitude` are now `*float64`. Neither key (or JSON null for both) sets `CastVoteInput.LocationMissing`; exactly one missing key is an immediate 400 naming that key ("latitude is required." or "longitude is required."). In `CastVote`, a new step between the expiry check and the distance refusal returns `ValidationError{Field: "latitude"}` when `LocationMissing` is set and the vote is not the reporter's own resolution vote. `DisallowUnknownFields`, the 4 KiB cap and the decode error path are unchanged. Service test `TestCastVoteRequiresALocationExceptForTheReportersOwnResolution` (did not compile before the change) and handler test `TestVoteBodyWithoutCoordinates` (four 400 cases failed before: they answered 403 "too far") now pass. The handler fake returns the inserted vote's row from `CurrentVotesForReports` and an empty slice from `ReporterHistoryReports`, so the 200 cases run the whole service path. A JSON null counts as absent, pinned by `{"latitude":null,"longitude":77.6}` from a non reporter on confirm: 400 field latitude.
- **Task 4 (UF-03-01).** app.js gains `categoryLabel(category)`: the table entry for an own property, else `CATEGORY_LABELS.other`, exported next to `CATEGORY_LABELS`. `updatePopupCard`, `pinName` and `buildMetaText` call it. `TestCategoryLabelNeverShowsRawText` is new and the one needle in `TestPinOpensOnSpace` was changed to `Pinalert.categoryLabel(`; both failed before the JS change. modal.js was left alone.
- **Task 5 (T9.16 follow up).** map.js implements exactly the validated code: a `closePressHadFocus` flag set by a `pointerdown` listener on the map container when the press lands on `.leaflet-popup-close-button` while `popupFocusInside` is true, read and cleared in `onPopupClose`, which then also counts focus on the map container as lost. `onMapFocusIn`, `onMapFocusOut`, `returnFocusToOpener` and the Escape path are untouched. `TestPopupClosePressKeepsTheFocusAnswer` is new and failed before the change.

## The narrower IN-01 rule, and why

The first draft of the finding said to reject a body without coordinates. That would break the reporter's own resolve and reopen for any client that sends no location, because the exemption lives in the service and is decided from the database reporter id (T-03-01), not from anything in the body. So the rule is narrower: a location is required for every vote except the reporter's own resolve and reopen. No keys is 400 field latitude from the service for everyone else; one key is 400 naming the missing key from the handler for everyone. The reporter's own confirm is still `ErrCannotVoteOwnReport`, an expired report is still `ErrReportExpired`, and the browser (which sends an explicit 0,0 for the reporter, votes.js `ownReportLocation`) is unchanged.

## Swagger

`swag init -g internal/api/router.go -o docs` ran successfully in the worktree for Tasks 2 and 3. New `@Description` lines were added to `NearbyReports` and `CastVote`; no existing line was reworded. The generated diff also carries the new field comments of `CastVoteRequest`. `TestSwaggerSpecCoversRoutes` and `TestSwaggerDocServed` pass.

## Accepted case (Task 5)

A stale flag is accepted: if a close button press is followed by a close through another route with no new press (for example Escape after the pointer left the button), the flag is still true and focus returns to the marker, which is harmless. There is deliberately no clear in `onPopupOpen`, because this exact code was validated in the browser.

## Out of scope, stated as the plan asked

- Trailing data after the JSON object (the second clause of IN-01): not handled.
- Any change to votes.js: none.
- modal.js line 250 (`Pinalert.CATEGORY_LABELS[category] || category`, the same raw fallback on the submit form, which offers only the fixed chip list): left as is.

## Test commands run and results

`echo "[${DATABASE_URL:-}]"` printed `[]`. The sandbox refused the `env -i` form, so the plain form was used after that echo. No database, server, browser or port was used.

- `go build ./...`: pass.
- `go vet ./internal/service/ ./internal/api/handlers/ ./internal/mailer/ ./web/`: pass.
- `go test ./internal/api/handlers/ -run 'TestNearbyRejectsNaNQueryValues|TestVoteBodyWithoutCoordinates|TestSwaggerSpecCoversRoutes|TestSwaggerDocServed' -count=1`: ok.
- `go test ./internal/service/ -run 'TestCastVoteRequiresALocation|TestCastVoteValidatesInput' -count=1`: ok.
- `go test ./web/ -count=1`: ok (0 FAIL, 0 SKIP in the verbose run before the final commit).
- `gofmt -l cmd internal web`: prints nothing.

## Live proof still owed (orchestrator, after merge)

The full gate on a scratch database and the scripted browser checks for tests 2 to 10, with T9.16 strict in Chromium and WebKit.

## Deviations from Plan

None. The plan was executed as written. One cosmetic choice: the CastVoteRequest field comments say the location is required except on the reporter's own resolve and reopen, and the pointer rationale sits in the type comment, so the swagger schema text stays API facing.

## Known Stubs

None.

## Threat Flags

None. The vote body change only narrows what is accepted; the reporter exemption still comes only from the database row.

## Self-Check: PASSED

- All five commits exist on the worktree branch, each listing only its own paths.
- nearby_validation_test.go and votes_body_test.go exist.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched.
