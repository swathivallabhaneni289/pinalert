# Phase 3: Trust Model Hardening, Diversity Weighted Trust - Pattern Map

**Mapped:** 2026-10-04
**Files analyzed:** 40 new or modified (grouped below)
**Analogs found:** 36 / 40 (no analog: `trust.js` tooltip/disclosure, six icon masks' new CSS usage beyond existing icons, `00005` index migration has a partial analog only, `docs/TRUST-MODEL.md`)

Source of the file list: 03-RESEARCH.md "Change map (ask 1)" and "Tests (ask 7)", 03-UI-SPEC.md "Hand Off Notes". Standing constraints: PORT 8792 on 127.0.0.1 for any run or UAT instruction, disposable database only (never `pinalert_test` or `pinalert`), no new packages, no JS build step, no images, no em or en dashes in prose or reader visible strings (new `trust.js` and new templates are scanned whole file, so no dashes even in comments).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/service/geo.go` (+ `geo_test.go`) | utility (pure) | transform | `internal/service/visibility.go` (pure, time only) and `report.go` `BoundingBox` | role-match |
| `internal/service/currency.go` | utility (pure) | transform | `internal/service/visibility.go` | exact |
| `internal/service/reliability.go` | utility (pure) | transform / batch | `internal/service/visibility.go` + `trust.go` `BuildVoteTally` | role-match |
| `internal/service/trustcopy.go` | utility (pure copy table) | transform | `visibility.go` `ResolveReasons` closed sets, `trust.go` `gpsDeniedMessage` | role-match |
| `internal/service/trust.go` (modify: row filter, `BuildTrust`, `CastVote` distance step, `ReporterHistoryReports` on `VotingQuerier`) | service | request-response | itself | exact |
| `internal/service/geohash_precision_test.go` | test (package `service`) | transform | `internal/service/trust_test.go` (package `service`) | exact |
| `internal/service/feed.go` (`ReportView.Trust`) | model | transform | itself | exact |
| `internal/service/report.go` `Nearby` (attach Trust, `Querier` gains method) | service | batch read | itself | exact |
| `internal/service/auth.go` `ActivityForAccount` (`ActivityReport.Trust`, `AuthQuerier` gains method) | service | batch read | itself | exact |
| `internal/store/queries/reliability.sql` | sqlc query | CRUD read | `queries/votes.sql` `CurrentVotesForReports`, `reports.sql` `ReportsByAccount` | role-match |
| `internal/store/queries/votes.sql` (`ReportVoteContext` adds `r.latitude, r.longitude`) | sqlc query | CRUD read | itself | exact |
| `internal/store/migrations/00005_*.sql` | migration | DDL | `00004_create_votes.sql` (index block) | role-match |
| `internal/store/*_test.go` (index by name, bounds, window edge) | test (DB) | CRUD | `internal/store/reports_test.go` `TestNearbyReportsUsesIndex`, `votes_test.go` | exact |
| `internal/api/handlers/trust.go` (`TrustResponse`) | handler model | transform | `reports.go` `reportViewToResponse` | role-match |
| `internal/api/handlers/votes.go` (403 `location`, `Trust` in `CastVoteResponse`) | handler | request-response | itself | exact |
| `internal/api/handlers/reports.go` (`FeedReportResponse.Trust`) | handler | request-response | itself | exact |
| `internal/api/handlers/auth.go` (`profileReport.TrustJSON`) | handler | request-response | `newProfileReport` in same file | exact |
| `internal/api/handlers/page.go` + `cmd/server/main.go` (`VoteRadiusKm`) | config | request-response | `DefaultRadiusKm` plumbing in same files | exact |
| `internal/api/handlers/swagger_test.go`, `votes_e2e_test.go`, `feed_visibility_e2e_test.go`, `activity_e2e_test.go` | test (e2e) | request-response | themselves | exact |
| `web/templates/index.html.tmpl`, `profile.html.tmpl` | template | request-response | themselves | exact |
| `web/static/js/trust.js` (new) | component (IIFE module) | event-driven / transform | `web/static/js/visibility.js` | exact (shape), none for tooltip |
| `web/static/js/votes.js` (modify) | component | request-response | itself | exact |
| `web/static/js/feed.js`, `map.js`, `activity.js`, `visibility.js`, `app.js` | component | event-driven | themselves | exact |
| `web/static/css/trust.css`, `main.css` | stylesheet | n/a | themselves | exact |
| six icon SVGs (`clock`, `check`, `ban`, `shield-check`, `rotate-ccw`, `x`) | static asset | n/a | `web/static/icons/circle-alert.svg` | exact |
| `web/votes_contract_test.go`, `web/js_contract_test.go`, `web/css_contract_test.go` | test (contract) | text scan | themselves | exact |
| `docs/TRUST-MODEL.md`, README pointer | docs | n/a | none | no analog |

## Pattern Assignments

### Pure service files: `geo.go`, `currency.go`, `reliability.go`, `trustcopy.go`

**Analog:** `internal/service/visibility.go` (the Phase 2 pure function house style).

Copy this shape: a file header comment naming decision ids, `package service`, import only `time` (visibility.go is the "time only parallelism seam", so `currency.go`, `reliability.go`, `geo.go`, `trustcopy.go` should import no sqlc types; keep sqlc row handling in `trust.go`), closed string types with a `Valid()` method and an ordered `var` list, `now` injected as a parameter (never `time.Now()` inside the pure function).

```go
// visibility.go lines 14-40
type Visibility string
const (
	VisibilityHidden      Visibility = "hidden"
	...
)
var Visibilities = []Visibility{VisibilityHidden, VisibilityProvisional, VisibilityLive, VisibilityRetracted}
func (v Visibility) Valid() bool {
	for _, x := range Visibilities { if x == v { return true } }
	return false
}
```

```go
// visibility.go lines 66-80: closed, serialized slug set whose strings are frozen
type ResolveReason string
var ResolveReasons = []ResolveReason{ReasonResolved, ReasonCriticalBypass, ...}
```

Use the same pattern for the new `fade_stage` slugs and tag slugs in `trustcopy.go`. Single shared named constant for a shared number (pattern: `IndependentAgreementThreshold = 2`, visibility.go line ~57) so `MaxVoteDistanceKm` is one constant read by `geo.go`, the handler swagger text, and `PageConfig.VoteRadiusKm`.

**geo.go specifics:** `BoundingBox` in `report.go` line 250 is the existing geo helper to stay consistent with (same earth radius 6371 as the SQL in `reports.sql` `NearbyReports`, which uses `6371 * acos(least(1.0, ...))`; clamp the same way in `haversineKm`). Do not touch `NearbyReports` (guarded by `TestNearbyReportsUsesIndex`, `TestExpiryReadTimePredicate`).

**Test analog:** `visibility_test.go` and `trust_test.go` table tests with injected `now`. `geo_test.go` can be `service_test`; `geohash_precision_test.go` must be package `service` to read unexported `voterGeohashPrecision` (same as `trust_test.go`).

---

### `internal/service/trust.go` (modify)

**Analog:** itself. Excerpts the plan must preserve or extend:

**Imports** (lines 8-17): `context, errors, time, pgx, geohash, sqlcgen "pinalert/internal/store/sqlc"`.

**Row filter extraction (D-04, `ConfirmedPlaces`):** `BuildVoteTally` (lines 141-200) is the loop to extract. Keep the invariants: skip rows for other reports, reporter content rows skipped (T-02-05), `independentCellCount` is the one distinct-cell implementation (lines 116-122), input must already be per account deduped by `CurrentVotesForReports`. The extracted filter should yield the confirm cell slice and the dispute cell slice that `BuildVoteTally` and `BuildTrust` both consume, so N and `ConfirmedPlaces` cannot drift:

```go
// trust.go 148-166
isReporter := reporterAccountID != nil && row.AccountID == *reporterAccountID
switch VoteKind(row.Kind) {
case VoteKindContent:
	if isReporter { continue }
	switch VoteValue(row.Value) {
	case VoteConfirm:  confirmCells = append(confirmCells, row.GeohashCell)
	case VoteDispute:  disputeCells = append(disputeCells, row.GeohashCell)
	}
```

**Interface growth pattern** (lines 214-220): each consumer interface names shared generated methods separately by design, so `VotingQuerier` gains `ReporterHistoryReports` and so do `Querier` (report.go) and `AuthQuerier` (auth.go), plus three fakes.

```go
type VotingQuerier interface {
	InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
}
```

**CastVote step order is part of the spec** (doc comment lines 258-289, code 290-362). The distance step (D-01) slots between step 3 (self vote block) and step 5 (cell computation), with reporter exemption rule per RESEARCH "Distance rule". It must return before `InsertVote` (zero `InsertVote` calls on refusal, asserted via the `calls` log in `fakeVotingQuerier`). Update the numbered doc comment, because a "step order test" is planned.

**Sentinel error pattern** (lines 94-106):
```go
var ErrCannotVoteOwnReport = errors.New("service: cannot vote on your own report")
var ErrReportExpired = errors.New("service: report has expired")
var ErrReportNotFound = errors.New("service: report not found")
```
Add `ErrVoterTooFar` in `geo.go` in the same style.

**Clock:** `CastVote` uses `now := time.Now().UTC()` at line 331. RESEARCH calls for "clock options" in W3; keep one capture of `now` per call and pass it to pure functions.

**Comment to update:** `voterGeohashPrecision` comment (lines 21-31) closes assumption A3 (D-03).

---

### `internal/store/queries/votes.sql` `ReportVoteContext` (modify) and `reliability.sql` (new)

**Analog:** `queries/votes.sql`. Select list change only:
```sql
SELECT r.severity, r.category, r.expires_at, s.account_id AS reporter_account_id
FROM reports r
LEFT JOIN sessions s ON r.session_id = s.session_id
WHERE r.id = sqlc.arg(report_id);
```
Add `r.latitude, r.longitude`; keep the LEFT JOIN rationale (c) in the comment block, and keep "zero rows means the report does not exist". Run `make sqlc` (generated code in `internal/store/sqlc`, not hand edited).

**reliability.sql analog:** `ReportsByAccount` (reports.sql, joins through sessions to the bound account, no `session_id` in the select list per T-01-02) and `CurrentVotesForReports` (batched `ANY(sqlc.arg(...)::bigint[])`, explicit `sqlc.arg` naming, long comment block explaining each non obvious choice). The query must be bounded (window plus limit per RESEARCH "Reliability history read"), use `sqlc.arg`, and carry a comment per non obvious decision. Do not select `session_id` or `geohash_cell` into anything that reaches a response.

**Migration `00005`:** analog `00004_create_votes.sql` index block (goose `-- +goose Up` / `-- +goose Down`, comment explaining column order and direction mirror the query, not style):
```sql
CREATE INDEX idx_votes_report_account_kind_created
    ON votes (report_id, account_id, kind, created_at DESC, id DESC);
```
Down drops the index. Embedded via `internal/store/migrations.go`; `migrations_test.go` is the analog for "migration embedded and index present in `pg_indexes`".

**Store test analog:** `reports_test.go` `TestNearbyReportsUsesIndex` (lines 66-110): `testutil.NewTestDB(t)`, `testutil.SeedReports`, `ANALYZE`, `EXPLAIN (ANALYZE, FORMAT TEXT) <sql>`, scan plan lines, assert `Index Scan`/`Bitmap Index Scan` present and no `Seq Scan on reports`. Seed helpers: `votes_test.go` `seedReportForVotes`, `seedAccountForVotes`, `voterEmail` (lines 17-55). These tests need a disposable database only.

---

### `internal/service/report.go` `Nearby` and `auth.go` `ActivityForAccount` (modify)

**Analog:** themselves. Both already do one batched `CurrentVotesForReports` for the page then per report `BuildVoteTally` + `Resolve`. Attach `Trust` in that same loop; the reliability history read must be batched per page, not per report (the "Nearby batches vote reads once" test `TestNearbyBatchesVoteReadsOnce` exists, so extend rather than add per report round trips). Empty page returns early with zero extra queries (report.go Nearby step 2, `return []ReportView{}, nil`). `ActivityForAccount` (auth.go 342-403) captures `now` once, validates category and severity once with fallbacks (`CategoryOther`, `SeverityLow`); keep that. `ActivityReport` is a struct with `Row, Category, Severity, Visibility, Reason`: add `Trust`.

`ReportView` (feed.go lines 21-28) embeds `Report` plus `Visibility, VisibilityReason, ViewerVote, IsOwnReport`; add `Trust` there. Viewer-specific fields make it unsafe to cache across callers, so keep Trust viewer independent.

---

### `internal/api/handlers/votes.go` (modify) and `trust.go` (new)

**Analog:** `votes.go` itself.

Error map ordering (lines 132-152): sentinel `errors.Is` branches first, then `errors.As` for `ValidationError`, then log and 500. Add `ErrVoterTooFar` as 403 with field `location`:
```go
case errors.Is(err, service.ErrCannotVoteOwnReport):
	writeFieldError(w, http.StatusForbidden, "account", "You can't vote on your own report.")
	return
```
Copy for the new branch: `writeFieldError(w, http.StatusForbidden, "location", "<copy from trustcopy/UI-SPEC, no dashes>")`.

Response struct with swagger enum tags (lines 47-50):
```go
type CastVoteResponse struct {
	Visibility string `json:"visibility" enums:"hidden,provisional,live,retracted" example:"live"`
	Reason     string `json:"reason" enums:"resolved,..." example:"confirmed"`
}
```
Add `Trust TrustResponse \`json:"trust"\``. The doc comment currently says "carries no vote counts"; update it. Update `@Description` and add nothing new to `@Failure` (403 already listed); the swagger comment must mention the distance rule. Regenerate docs with `make swag`; extend `swagger_test.go` (substring drift guard, lines 77 and 119).

**`trust.go` analog:** `reports.go` `reportViewToResponse` (lines 172-199): explicit field by field mapping, `*string` for nullable (`capacityStatus`, `yourVote`). `Currency.Score` and `Reporter.Score` are `*int` with explicit `null` (D-08), so use no `omitempty` on them. One mapper `trustToResponse` shared by feed, vote, and profile (same equality asserted between feed and vote response). Response bodies must contain no `account_id` or `geohash_cell` (e2e test).

**Profile:** `auth.go` `newProfileReport(ar service.ActivityReport, now time.Time) profileReport` (line 348) and struct (line 259); add `TrustJSON` there (JSON marshalled once, rendered into a `data-trust` attribute with html/template escaping, never `template.HTML`). Template analog: `profile.html.tmpl` lines 29-32 (`data-report-id`, `data-visibility`, `data-visibility-reason`, conditional `data-can-reopen`).

---

### `page.go`, `cmd/server/main.go`, `index.html.tmpl` (D-22 `VoteRadiusKm`)

**Analog:** how `DefaultRadiusKm` flows: `PageConfig` field, `pageViewModel` field, copied in `Page` (page.go lines 19-62), set in `cmd/server/main.go` lines 144-149 (`PageConfig{FallbackLat:..., DefaultRadiusKm: defaultRadiusKm, AssetVersion: ...}`), rendered as `data-default-radius-km="{{.DefaultRadiusKm}}"` on `#app-shell` (index.html.tmpl line 27). Add `data-vote-radius-km="{{.VoteRadiusKm}}"` the same way, value from `service.MaxVoteDistanceKm`. `page_test.go` is the test analog. JS reads it from `dataset` with no `Pinalert.config` literal in `votes.js` (contract test forbids that string there).

---

### `web/static/js/trust.js` (new)

**Analog:** `web/static/js/visibility.js` (exact module shape).

Header comment style, strict IIFE, closed allowlists with safe fallback direction (unrecognised means not yet trusted), `window.PinalertX = {...}` export, DOM only through `document.createElement` and `Pinalert.setText`:
```js
// visibility.js 21-22, 168-176
(function () {
  'use strict';
  ...
  window.PinalertVisibility = {
    VISIBILITY_STATES: VISIBILITY_STATES,
    createVisibilityTag: createVisibilityTag,
    updateVisibilityTag: updateVisibilityTag
  };
}());
```
Builder plus idempotent updater pair (`createVisibilityTag()` builds with `hidden` attribute and no state; `updateVisibilityTag(tag, report)` is the only place state reaches the DOM, null guarded, runs on every 30 second poll). Copy this create/update split for `createTrustBlock` / `updateTrustBlock`. `replacePrefixedClass` (visibility.js line 72) is a per module helper; copy rather than export.

Rules (from tests): no markup sink (no innerHTML etc.), no numeric threshold identifiers, no `aria-live`, no dash characters anywhere in the file including comments (whole file scan), tooltip handles Escape, focus, hover, click. `PinalertTrust.fadeStage` replaces `Pinalert.ageStage` at all consumers; the existing `ageStage` (app.js lines 276-292: fresh above 0.25 remaining, aging at or above 0.125, else stale) is the shape to supersede, and no consumer may call `Pinalert.ageStage(` afterwards. Call sites to change: `feed.js` lines 205 and 355, `map.js` lines 168, 242, 284. The client must compute nothing the server did not send (T-02-04 comment in visibility.js lines 6-9); `fade_stage` comes from `Trust.Currency`.

No analog: the disclosure/tooltip behaviour (Escape ownership by surface). Use 03-UI-SPEC "Hand Off Notes" and RESEARCH "Front end".

Script order (profile test `TestProfilePageLoadsTrustAssetsInDependencyOrder`, votes_contract_test.go 1909-1958, requires exactly one occurrence each, `defer`, and `?v={{.AssetVersion}}`): feed page `app.js, votes.js, visibility.js, trust.js, map.js, modal.js, feed.js, account-menu.js`; Activity `app.js, votes.js, visibility.js, trust.js, activity.js, account-menu.js`. Tag format:
```html
<script src="/static/js/visibility.js?v={{.AssetVersion}}" defer></script>
```

---

### `web/static/js/votes.js` (modify; D-02, D-22)

**Analog:** itself. Contract tests constrain this file hard (`web/votes_contract_test.go`):

- Exactly one `navigator.geolocation.getCurrentPosition(` and exactly one `fetch(` (lines 54-128), `castVote(reportId, action)` signature anchor, `getVoterLocation(` appears before `fetch(` in body, no `Pinalert.config`, no `setView(`, at least one `reject(`. Do not write any of these literals in a `//` comment (comment stripping handles only `/* */`).
- `TestVoterLocationIsCachedPerSession` anchors `function getVoterLocation()`: change anchor to the new signature (for example `getVoterLocation(opts)`); keep one `sessionStorage.getItem(` and one `setItem(`, get before prompt. Existing function (lines 98-146) reads the cache, falls to the single prompt, writes the cache, rejects with `fieldMessage = GPS_DENIED_MESSAGE` on denial.
- D-02 silent refresh: reuse the single prompt call and single fetch site (add an option to `getVoterLocation`, do not add a second call).
- `applyDistanceHint` must be a separate function and use `aria-disabled`, never `.disabled =`; `applyOwnReportRule` and `updateVoteBlock` bodies must not contain the word `disabled`; `setBlockBusy` owns `disabled` (`TestOwnReportRuleRemovesControlsRatherThanDisablingThem`, line 395). Add a test for `applyDistanceHint` mirroring that one.
- Error mapping pattern in `castVote` (lines 168-194): non ok responses turn `body.error {field, message}` into `Error` with `.field` and `.fieldMessage`; the new 403 `location` field flows through this unchanged.
- `castVote` result now includes `trust`; reopen and vote UI update the block from `CastVoteResponse.trust` (D-20, R-08).
- `votes.js` exports `window.PinalertVotes` (line 507).

---

### `feed.js`, `map.js`, `activity.js` (modify)

**Analog:** themselves. Hard constraints from contract tests:
- `feed.js` and `map.js` may not contain `vote-btn`, `vote-controls`, `vote-error`, `your_vote`, `is_own_report`, `visibility-tag`, `vis-provisional`, `vis-hidden`, `visibility_reason`, `report.visibility` (`TestBothSurfacesMountTheSameVoteBlock` line 462, `TestBothSurfacesMountTheSameVisibilityTag` line 863). The trust block is applied through `PinalertTrust.*` builders only.
- Ordering in `createRow` and `buildPopupContent`: `body.appendChild(meta)` before `createVisibilityTag(` before `createVoteBlock(`; place the trust block call so these order anchors still hold (read the test before choosing the position).
- `map.js` `bindPopup(` className option must stay exactly `popupStateClasses(report)`; `setPopupContent(` must remain; `unbindPopup(` forbidden (`TestMapPopupCarriesReportStateClasses`, js_contract_test.go). `buildPopupContent` passes `{showScores: true}`; feed and Activity omit it (D-09).
- `activity.js`: no `fetch(`, no `navigator.geolocation`, exactly one `addEventListener`, exactly one `Pinalert.showToast(`, no `Threshold`, `accountID`, `is_own_report`, `REOPEN_PENDING_TOAST`, `resolutionOutcomeMessage`; header says it computes and fetches nothing and reads server attributes from the row (lines 1-16). Read `data-trust` JSON from the row and hand it to `PinalertTrust`; tooltip listeners live in `trust.js`. It sets `expired` from `dataset.expired` and holds no `vis-expired` literal. `visibility.js` keeps `VISIBILITY_STATES` at four values (`TestVisibilityTagCopyAndFallback`).

---

### CSS: `trust.css`, `main.css`

**Analog:** the existing `trust.css` rules and the sketch skill (`.claude/skills/sketch-findings-pinalert/SKILL.md`: validated popup and trust block design, variant d4). Plans touching popup, feed row, or Activity trust block should load that skill and 03-UI-SPEC. Constraints (UI-SPEC Hand Off Notes): every new `display` rule has `:not([hidden])` guard; no hex, `rgb(`, `rgba(`, `hsl(`; no `!important`; no `999px`; no new custom properties in `trust.css` beyond the three allowlisted (`--severity-current`, `--severity-tint`, `--badge-glyph-fg`, test near line 730 of votes_contract_test.go); spacing as `var(--space-*)` tokens, 44px as `var(--touch-target-min)`; each new icon is a literal `url("/static/icons/NAME.svg")` mask, and no selector head contains `.icon-glyph--`; keep the exact heads listed in the UI-SPEC table. `main.css` edits are limited to `--shadow-popup` in `:root`, `.report-row__meta` colour, and `overflow-wrap: anywhere` on the title. Existing long dashes in `trust.css` comments are not rewritten.

**Icon analog:** `web/static/icons/circle-alert.svg` (and `check-circle.svg`); copy its viewBox/stroke conventions for the six new files. No raster images.

---

### Tests to update (Phase 2 fixtures that break)

| File | Fix (from RESEARCH Tests) |
|---|---|
| `internal/service/trust_test.go` (10 `ReportVoteContextRow{}` literals, 20 `CastVoteInput{}`) | one shared helper giving `Latitude: 12.9716, Longitude: 77.5946`; extend `fakeVotingQuerier` (line ~360, records `calls` and `insertedVotes`) with `ReporterHistoryReports` |
| `feed_test.go` `fakeFeedQuerier`, `auth_test.go` `fakeAuthQuerier` | add the new method |
| `votes_e2e_test.go` lines 282, 355; `feed_visibility_e2e_test.go` lines 87, 132, 288 | voter C becomes `(12.9686, 77.5946)`; keep `assertDistinctCells` |
| `web/votes_contract_test.go` | anchors listed in the votes.js section above |

E2E helpers to reuse: `newE2EServer`, `newVerifiedClient`, `postVote`, `decodeVoteResponse`, `floodReportBody`, `getNearby`, `decodeFeedResponse`. Test DB: `testutil.NewTestDB(t)` truncates and skips without `DATABASE_URL`; point it only at a freshly created disposable database. `make test` is `go test ./... -v -p 1`.

## Shared Patterns

### One authority, no recomputation (T-02-04)
**Source:** `visibility.go` header and `trust.go` `CastVote` step 8. All surfaces (feed, vote response, Activity) get `Trust` from one builder in `service`; handlers only map; JS only renders. Add the same property test style: feed `trust` equals vote response `trust` for the same data.

### Server computed identity and cells
**Source:** `trust.go` `CastVoteInput` (no geohash field) and `CastVoteRequest` decoder with `DisallowUnknownFields` (votes.go lines 85-95). Never accept distance, cell or trust from a client.

### Error mapping
**Source:** `votes.go` lines 132-157, `writeFieldError(w, status, field, message)` and `ValidationError{Field, Message}`.

### Batched reads, `now` captured once
**Source:** `report.go` `Nearby` doc comment (7 ordered steps) and `auth.go` `ActivityForAccount`. Empty page returns before any batched query.

### Dash and text policy
New reader visible strings (Go copy, templates, `trust.js`) contain no U+2014 or U+2013; `design_rules_contract_test.go` scans templates whole file. Pure copy lives in `trustcopy.go` so the wire carries it and JS does not duplicate it.

### Run and UAT instructions (override of RESEARCH line 312)
Any manual run uses `PORT=8792`, binds `127.0.0.1`, `BASE_URL=http://127.0.0.1:8792`, and a freshly created disposable database; never 8080, 8090, `pinalert_test` or `pinalert`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `trust.js` disclosure/tooltip (hover, focus, click, Escape ownership by surface) | component | event-driven | No existing tooltip or disclosure widget in `web/static/js`; follow 03-UI-SPEC and RESEARCH "Front end". Only `modal.js` handles focus and Escape, loosely related. |
| `docs/TRUST-MODEL.md` | docs | n/a | No `docs/` prose analog beyond generated swagger; content per RESEARCH "Distance rule" and "Score formulas". |
| Migration `00005` | migration | DDL | Only partial: `00004` index block; this is index only, no table. |
| Concurrency test (24 goroutines, `-race`) | test | CRUD | Closest is the votes concurrency test in `internal/store/votes_test.go` (`voterEmail`, `voterGeohashCell` helpers) but it lives at store level, not service plus handler. |

## Metadata

**Analog search scope:** `internal/service`, `internal/store` (queries, migrations, tests), `internal/api/handlers`, `cmd/server/main.go`, `web/static/js`, `web/static/css`, `web/static/icons`, `web/templates`, `web/*_contract_test.go`.
**Files read or scanned:** about 25 (partial reads for large files).
**Pattern extraction date:** 2026-10-04
