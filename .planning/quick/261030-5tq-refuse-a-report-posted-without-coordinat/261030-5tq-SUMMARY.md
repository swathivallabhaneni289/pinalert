---
phase: quick-261030-5tq
plan: 01
subsystem: api
tags: [validation, reports, demo, swagger]
requires: []
provides:
  - "POST /api/reports and POST /demo/api/v/{visit}/reports refuse a missing or null latitude or longitude with 400"
affects: [internal/api/handlers, docs]
key-files:
  created:
    - internal/api/handlers/submit_coordinates_e2e_test.go
  modified:
    - internal/api/handlers/reports.go
    - internal/api/handlers/demo.go
    - internal/api/handlers/demo_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - .planning/ROADMAP.md
decisions:
  - "Pointer float64 coordinates on SubmitReportRequest, the same as CastVoteRequest; no struct tag, Required said in the field comments"
metrics:
  completed: 2026-10-30
status: complete
---

# Quick Task 261030-5tq: Refuse a report posted without coordinates Summary

A report body with no latitude or longitude, or null for either, is now a 400 naming the field ("latitude is required." / "longitude is required.") on both the real post and the Demo practice post, instead of being filed at 0, 0. An explicit 0, 0 is still accepted.

## What changed

- SubmitReportRequest.Latitude and Longitude are now pointers. submitInputFrom (shared by SubmitReport and demoSubmit) returns (service.SubmitInput, error) and refuses a nil latitude first, then a nil longitude, with a service.ValidationError that writeSubmitError maps to 400.
- In SubmitReport the check sits after the expected_account_id 409 and before ValidateSubmitInput, FindReplay and gate.AdmitReport, so a refusal stores nothing and uses no allowance.
- API reference regenerated with make swag: both coordinate descriptions say Required, the POST /reports description gains one sentence, types stay number, no required array added.

## RED

Both new tests failed before the fix for the 0, 0 reason. Demo: all six subtests (latitude absent, longitude absent, latitude null, longitude null, both absent, both null) answered 201 with the report filed at 0 for the missing coordinate. Real route: "latitude absent" and "longitude absent" answered 201 (stored at 0), then "latitude null", "longitude null", "both absent" and "both null" answered 429 because the stored posts used the Burst 2 allowance; the count check then found 2 stored reports. No skip, no compile error.

## GREEN

TestSubmitReportRefusesMissingCoordinates: 6 of 6 subtests pass plus the parent. TestDemoSubmitRefusesMissingCoordinates: 6 of 6 plus the parent. No skip. The 409 on expected_account_id still wins over a missing latitude, the count stays 0 after the refusals, and the explicit 0, 0 post and a normal post both answer 201 within a Burst 2 allowance.

## Docs diff

git diff --numstat -- docs/: docs.go 3/3, swagger.json 3/3, swagger.yaml 5/2 (the two field descriptions wrap onto a second line, plus one new line in the POST /reports block). The license reads "Unlicensed (all rights reserved)" once in router.go, docs.go, swagger.json and swagger.yaml. No new path, no required array.

## Verification

- Full suite on p_q5tq_a: go test ./... -p 1 -count=1 -v: 0 "--- FAIL", 0 "FAIL", 0 panic, 2424 "--- PASS", 21 ok packages, 0 "DATABASE_URL not set". Skips: TestGeocodeLive (needs GEOCODE_LIVE=1), TestOfficialLive (needs OFFICIAL_LIVE=1), TestScreeningFixture (no recorded scores, needs the MODERATION_LIVE recording). No flake reruns were needed.
- node --test web/*.mjs: 1646 pass, 0 fail.
- go build ./..., go vet ./internal/api/... ./docs/, gofmt on the touched files, Swagger and Demo tests, and TestGeneratedAPIReferenceUsesPlainPunctuation pass.
- Scratch database p_q5tq_a dropped; .scratch-5tq deleted; STATE.md not touched; nothing pushed.

## Commits

- d7ddd333 fix(261030-5tq): refuse a report posted without a latitude or longitude
- 351ed81e docs(261030-5tq): mark backlog 999.11 done

## Deviations from Plan

None in the code. Two notes:
- The plan's own output section said not to commit the SUMMARY; the orchestrator's instruction to commit it alone as the last commit was followed.
- The plan's dash gate lists reports.go, which already holds em dashes in older comments; none were added by this change (the gate was checked on the other files and the new lines).

## Self-Check: PASSED

submit_coordinates_e2e_test.go exists, commits d7ddd333 and 351ed81e exist, no new untracked files left.
