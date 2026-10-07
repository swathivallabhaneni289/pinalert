---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 15
subsystem: ui
tags: [go, javascript, activity-page, trust-block, expired-state, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-07 trustToResponse mapper and CastVoteResponse.trust, 03-09 shared Reopen control, 03-13 window.PinalertTrust"
provides:
  - "Activity rows carry data-trust (json.Marshal(trustToResponse(ar.Trust))), data-expired and a server rendered severity label"
  - "activity.js mounts the shared trust block per row and refreshes it from the Reopen response, focus to the toggle"
  - "PinalertVisibility.displayState and stateWord, with the Expired label and vis-expired row class"
affects: [03-16, 03-19, 03-20]
tech-stack:
  added: []
  patterns:
    - "One mapper for every surface: the Activity JSON is the same Go mapper output as the feed and vote response"
    - "Server decides expiry; the browser reads a boolean attribute and compares no dates"
key-files:
  created:
    - web/activity_trust_contract_test.go
  modified:
    - internal/api/handlers/auth.go
    - internal/api/handlers/activity_e2e_test.go
    - web/templates/profile.html.tmpl
    - web/static/js/activity.js
    - web/static/js/visibility.js
key-decisions:
  - "Resolved wins over Expired: a Retracted report keeps vis-retracted and no Reopen even when expired."
  - "The expired tag writes no data-visibility-reason, because that slug explains the resolver state, not expiry."
requirements-completed: [TRUST-05, TRUST-07, D-20, R-08]
status: complete
duration: 35min
completed: 2026-10-08
---

# Phase 3 Plan 15: Trust block on the Activity page Summary

Each report a reporter posted now shows the shared trust block on the Activity page, filled from the same mapper output the feed and vote response carry, refreshed from the Reopen response, with a server decided Expired state and a visible severity word before every title.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Activity rows carry the shared trust JSON, an expired flag and a severity word | 00177a6 |
| 2 | The reporter sees the shared trust block, refreshed from the Reopen response | 4d8d61f |
| 3 | An expired, unresolved report reads Expired, decided once by displayState | a6d6ec8 |

## RED then GREEN

- Task 1 RED: `TestProfileTemplateUsesServerSeverityAndTrust` failed with four errors (no `data-trust`, no `data-expired`, `.TrustJSON` occurs 0 times, no severity label first). On `p3_0315`, `TestProfileRowsCarryTrustJSON`, `TestProfileMarksExpiredReports` and `TestProfileRendersSeverityLabel` all `--- FAIL`. GREEN: all three `--- PASS`, together with the pre-existing `TestProfileWithholdsReopenOnAnExpiredRetractedReport` and `TestProfileReopenIsInstantForTheReporter`; `ok  pinalert/internal/api/handlers`. Web side: `TestProfileTemplateUsesServerSeverityAndTrust`, `TestProfilePageLoadsTrustAssetsInDependencyOrder`, `TestProfilePageCarriesTheReportIdAndVisibilityHooks`, `TestUserVisibleCopyUsesPlainPunctuation` all `--- PASS`.
- Task 2 RED: `TestActivityMountsTheTrustBlock` failed (`calls createTrustBlock 0 times, want 1`, `calls updateTrustBlock 0 times, want 2`, `could not locate "function readRowTrust(row)"`). GREEN: pass; `go test ./web/ -count=1 -v` shows no `--- FAIL`, with `TestReopenControlIsSharedFromVotes` and `TestProfilePageLoadsTrustAssetsInDependencyOrder` passing.
- Task 3 RED: `TestDisplayStateDecidesExpired` failed (`visibility.js must declare displayState(report) and stateWord(report)`), `TestActivityReadsExpiredFromTheServer` failed (`contains "expired: row.dataset.expired === 'true'" 0 times, want 1`), and the node behaviour check failed at HEAD with `TypeError: V.displayState is not a function`. GREEN: node prints `displayState behaviour ok`; `go test ./web/ -count=1 -v` gives `ok  pinalert/web` with `TestDisplayStateDecidesExpired`, `TestActivityReadsExpiredFromTheServer`, `TestActivityMountsTheTrustBlock`, `TestVisibilityTagCopyAndFallback`, `TestResolutionOutcomeSlugMatchesVisibilityStates`, `TestBothSurfacesMountTheSameVisibilityTag`, `TestActivityModuleReusesTheSharedLabelMapAndTransport`, `TestReopenAppliesTheServerAnswerRatherThanGuessing` all `--- PASS`.
- Full suite on `p3_0315`: `go test ./... -count=1 -p 1` all packages `ok` (api, api/handlers, auth, geocode, mailer, ratelimit, service, session, store, testutil, web). The database was dropped afterwards (`psql -l` shows no p3_0315).
- Other gates: `node --check` passes for activity.js and visibility.js; `go vet ./internal/api/handlers/ ./web/` clean; `gofmt -l` on this plan's three Go files prints nothing.

## 03-14 landed?

No. `TrustResponse` in this base (05d0de6) still has the seven original keys, with no `currency` or `reporter`. The deep equality between the Activity `data-trust`, the confirm response trust and the Reopen response trust held on that shape. The currency label assertions in `TestProfileMarksExpiredReports` are conditional on a `currency` object being present, so they are untested here and will bite automatically once 03-14 lands. Nothing pins the key list.

## Long dash line counts (baselines from the plan)

- auth.go 21 (baseline 21), activity_e2e_test.go 7 (7), activity.js 8 (at most 9), visibility.js 11 (11), profile.html.tmpl 0, activity_trust_contract_test.go 0.

## Deviations from Plan

- [Rule 3 - Sandbox] The sandbox refused compound verify one-liners (command substitution, heredocs, `$(...)` with a Postgres URL). I split the plan's create, test, drop wrapper into three plain commands: `dropdb --if-exists p3_0315; createdb p3_0315`, then each `DATABASE_URL=... go test` run, then `dropdb --if-exists p3_0315` after the full suite. Only `p3_0315` was ever used. The long node behaviour one-liner was moved verbatim into a scratch file outside the repo and run with `node <file> <path>`; it prints `displayState behaviour ok`. The grep and perl count gates were run as separate commands (perl for the dash counts).
- The plan's `<human-check>` in Task 2 (database `p3_0315_uat`, server on 8792) was NOT run. It is deferred to the project owner at the end of the phase and plan 03-20 folds it into V-42.
- Added one test helper beyond the plan's list: `p15InOrder` in `web/activity_trust_contract_test.go`, used for the ordering assertions.

No other deviations. No STATE.md, ROADMAP.md or REQUIREMENTS.md edits.

## For later plans

- 03-16: the Activity row now carries the server rendered `span.severity-label.severity-label--{level}` and, after load, `vis-expired` and `visibility-tag--expired`, which its CSS styles (`TestExpiredRowTreatmentMatchesRetracted`).
- 03-19: `PinalertVisibility.stateWord(report)` is the accessor for the pin name state word. The feed and map never set `expired`.

## Known Stubs

None.

## Threat Flags

None. The only new surface is `data-trust`, whose keys come from the guarded mapper; `TestProfileRowsCarryTrustJSON` checks every key against the identifier pattern and the raw attribute for `@` and both emails.

## Self-Check: PASSED

- Files present: web/activity_trust_contract_test.go, internal/api/handlers/auth.go, internal/api/handlers/activity_e2e_test.go, web/templates/profile.html.tmpl, web/static/js/activity.js, web/static/js/visibility.js.
- Commits present: 00177a6, 4d8d61f, a6d6ec8.
