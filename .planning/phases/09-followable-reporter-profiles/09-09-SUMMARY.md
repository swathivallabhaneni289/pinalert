---
phase: 09-followable-reporter-profiles
plan: 09
subsystem: api
tags: [profiles, trust-block, privacy, reports, activity, swag]
requires:
  - phase: 09-02
    provides: PublicProfilePathsForViewer
  - phase: 09-07
    provides: gated people routes and /u/{username}
provides:
  - ReportView.ReporterProfilePath and an unexported reporter account id with an accessor
  - reporter_profile_path on report JSON rows and data-reporter-profile-path on Activity cards
  - PinalertTrust.setProfileLink and the See this reporter row
affects: [09-10, 09-11, 09-13, 09-26]
tech-stack:
  added: []
  patterns:
    - "viewer dependent fields sit on the row beside is_own_report, never inside the trust object"
    - "one batched path read per request, skipped for viewer 0 and for the viewer's own account"
key-files:
  created:
    - internal/service/reporterpath_test.go
    - internal/api/handlers/reporter_path_e2e_test.go
    - web/trust_profile_link_test.mjs
    - web/trust_profile_link_contract_test.go
  modified:
    - internal/service/feed.go
    - internal/service/report.go
    - internal/service/historyread.go
    - internal/service/auth.go
    - internal/service/feed_test.go
    - internal/service/auth_test.go
    - internal/demo/memquerier.go
    - internal/api/handlers/reports.go
    - internal/api/handlers/auth.go
    - web/templates/profile.html.tmpl
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/trust.js
    - web/static/js/map.js
    - web/static/js/activity.js
    - web/static/css/trust.css
    - web/trust_contract_test.go
key-decisions:
  - "A finished view never carries the path: buildFinishedView clears it, Focus skips the read for a finished view, and the Activity voted row sets it only for an Active or Disputed view (D-20, review B1)."
  - "setProfileLink draws only /u/{username} paths of at least three characters from [a-z0-9_.]; the 20 character cap is left to the server because trust.js may hold no numeral other than 0 and 1."
requirements-completed: [PROF-01, PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 09: See this reporter Summary

A report row now carries a viewer dependent `reporter_profile_path` (`/u/{username}`) only for an opted in reporter with a clear name who has not blocked the reader, only on a live report and never for the reader's own, and the trust block's explanation panel (and the popup card) shows it as a last line "See this reporter".

## What was built

- **Service.** `ReportView.ReporterProfilePath` sits beside `IsOwnReport`, never inside `Trust`. `ReportView.reporterAccountID` is unexported with the accessor `ReporterAccount()` for the mute and follow note plans. `loadProfilePaths` (feed.go) makes one `PublicProfilePathsForViewer` read over the page's distinct reporter accounts, none for viewer 0 and none for the viewer's own account. `Nearby`, `Focus` (live views only) and `ActivityPageForAccount` (voted rows) use it. `buildFinishedView` clears the path, so Resolved and Expired views never carry it on Focus, Past or Activity.
- **Wire.** `FeedReportResponse.ReporterProfilePath` with `json:"reporter_profile_path,omitempty"` (one mapper serves the feed, Focus and History). Swag regenerated. The Activity page is server rendered, so the path travels on the card as `data-reporter-profile-path` (`profileReport.ReporterProfilePath`, `profile.html.tmpl`).
- **UI.** `PinalertTrust.setProfileLink(target, path)` adds or removes `a.trust-panel__profile` as the last child of the block's panel (or of the popup card, which has no separate panel). Visible words "See this reporter", a hidden " {username}" span, a chevron-right mask icon. `map.js` calls it after every `updateTrustCard`, `activity.js` after the row's `updateTrustBlock`. `trust.css` gives it a 44px minimum row, top hairline, no underline, hover surface fill, inset focus ring, and a forced colours rule for the chevron.
- **Tests.** `TestReporterProfilePath`, `TestFinishedViewsCarryNoProfilePath` (database), `TestReporterPathOnFeedAndVote`, `TestReporterPathOmitsIdentifiers` (e2e), `web/trust_profile_link_test.mjs` (6 node tests), `TestTrustProfileLinkRow`.

## Task commits

1. Viewer dependent path on report views: a6f0eaed
2. JSON field, Activity data attribute, swag, e2e tests: b3081358
3. See this reporter row, CSS, node and contract tests: 1d3c626f

## Tests

Scratch database `p9_09_a` (created fresh, dropped at the end). `go build ./...` and `go vet ./...` clean. `go test ./internal/... ./web/ -p 1 -count=1`: every package ok (service 17s, handlers 211s, store, api, demo, web and the rest). `node --test web/*.mjs`: 1458 pass, 0 fail. New tests were each seen with `-v`: the two service tests, the two e2e tests and the parity guards `TestFeedTrustMatchesCastVoteTrust`, `TestTrustResponseOmitsIdentifiers`, `TestFeedSignalsMatchCastVoteSignals` all PASS with no SKIP. `TestSwaggerSpecCoversRoutes` passes; `make swag` leaves no diff after the commit.

## Deviations from Plan

**1. [Rule 3 - Blocking] Querier widened, three fakes updated.** `Querier` and `AuthQuerier` gained `PublicProfilePathsForViewer`. Satisfying types updated: `fakeFeedQuerier` (`internal/service/feed_test.go`), `fakeAuthQuerier` (`internal/service/auth_test.go`) and the demo `memQuerier` (`internal/demo/memquerier.go`, answers nothing). The fakes do not write to their call logs, so the pinned read order tests keep their result.

**2. [Rule 3 - Blocking] Activity is server rendered.** The plan names `internal/api/handlers/activity.go`, but its cards are built in `newProfileReport` (`handlers/auth.go`) and drawn by `web/templates/profile.html.tmpl`. Both were edited to carry the path as a data attribute for `activity.js`; `handlers/activity.go` itself needed no change. The Activity JSON the plan assumed does not exist.

**3. [Rule 3 - Blocking] Two existing contract tests tripped on the new code.** `web/trust_contract_test.go` forbids the text `.reporter` in map.js, which also matched `report.reporter_profile_path`; the test now sets that one exact reference aside before the check (comment names this plan). The other failures (`TestTrustCSS`, `TestFactsListIsTheOnlySourceOfRows`, `TestTrustCardIsAlwaysOpen`) were fixed in my own code: the `:not([hidden])` guard on the row's display rule, the semibold weight token, and the function placement in trust.js (the body helper reads up to the next function, so the new function goes after `focusToggle` and its constants at the top). The shared lists named by the plan (layout_rules, css_contract, noise_words) were not touched.

**4. [Rule 3 - Blocking] Path pattern has no length cap in trust.js.** The plan asks for `{3,20}`, but `TestTrustModuleHasNoThresholdsOrSinks` allows no numeral other than 0 and 1 on a code line. The pattern is `^/u/[a-z\d][a-z\d_.][a-z\d_.]+$`: at least three characters, no slash, no dots only, no uppercase, no scheme. The 20 character cap stays the server's rule, and the character set alone keeps a crafted value from becoming a link.

**5. Popup card has no panel.** UI-SPEC 7d speaks of the explanation panel; the popup's trust card is always open and has no panel, so the row is its last child (after the facts list). The Activity block gets it as the last child of `.trust-block__panel`.

## Notes for later plans

- `ReportView.ReporterAccount()` returns the reporter's account for service code. It is never serialised.
- The Low data page (`low.js`) is left as it is: it has no profile link.
- The vote response is unchanged and carries no report row, so it has no path; the trust parity tests still pass with the new field.
- `readContext.profilePaths` is filled only by Focus; History, the counts and the profile read leave it nil.
- Pre-existing gofmt drift in `place_test.go`, `photon.go` and `comments_test.go` was left alone.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-09-33 to T-09-36, T-09-98), all mitigated and tested.

## Self-Check: PASSED

All created files exist and commits a6f0eaed, b3081358 and 1d3c626f are on the branch.
