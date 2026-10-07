---
phase: 02
slug: trust-mechanic-core-confirm-dispute-visibility
status: verified
threats_open: 0
asvs_level: 1
created: 2026-10-07
---

# Phase 02: Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

**Scope of this result.** `threats_open: 0` is a statement about the threat register the plans authored (11 rows, 10 unique ids). It is not a claim that the confirm/dispute mechanic already resists trivial gaming. The code review's finding that it does not yet (CR-01) sits outside that register, is not accepted here, is still open, and is carried forward below with the place it is tracked.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| browser to vote API | Untrusted input reaches the four vote routes (`confirm`, `dispute`, `resolve`, `reopen`). Only a validated report id and two floats may pass. Identity is read from the verified-account gate, never from the request. | Report id (path), latitude and longitude (4 KiB body cap, unknown keys rejected) |
| device to browser (geolocation) | The browser's geolocation prompt is the only source of a voter's position. A denied prompt blocks the vote. No default or substitute coordinate exists anywhere in the flow. | Voter latitude and longitude (sensitive location) |
| verified gate to handler | `requireVerifiedAccount` resolves the caller's account before any vote, report or Activity handler runs. Handlers consume it and never re-derive it. | Account id (internal only) |
| handler to service to store | The store performs no validation. Closed-enum checks, the self-vote block and the server-side cell derivation all happen in `VotingService.CastVote`. | `CastVoteInput`, voter geohash cell (service internal) |
| application to PostgreSQL | Concurrent writers append to `votes`. The schema has no shared counter and no unique key, so there is no contended row. | Vote rows (account id, kind, value, voter cell) |
| vote data to resolver | The tally `BuildVoteTally` builds is the only input `Resolve` sees. Only a server-side reporter identity match can set the two privileged booleans. | Distinct-cell counts, `ReporterResolved`, `ReporterReopened` |
| every read path to resolver | The feed and map JSON, the vote response and the Activity page each call `Resolve`. None derives visibility itself. | Visibility state and reason slug |
| server response to public reader | Hand-declared response types carry the boolean `is_own_report`, never an account id, email, session id or voter cell. | Report fields, visibility, the caller's own vote |
| server value to DOM class or attribute | Visibility, reason, category and severity values are checked against fixed lists before reaching a class name or attribute. Text goes through `Pinalert.setText`. Go-built classes are guarded by `Visibility.Valid()`. | Server-authored slugs, report-authored text |
| browser storage to root attribute | The stored theme mode is validated against a fixed two-element list before the single `setAttribute` call. | `localStorage` theme string |
| client to third-party tile host | Basemap requests carry the visitor's viewport to the OpenFreeMap origin, and to the OpenStreetMap raster fallback. | Approximate visitor location (accepted, T-01-19) |
| server to outbound email | The verification email's disclaimer is a fixed literal and the link is HTML-escaped before it enters the markup. | Magic link, recipient address |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Spoofing (forged voter position, multi-account vote stuffing) | `VotingService.CastVote` cell derivation, `independentCellCount`, `CastVoteRequest` decoder, `votes.js` transport | high | mitigate | The independence predicate is server computed end to end and stacks on the verified-account gate. Verified: `CastVoteInput` has six fields and no geohash field (`internal/service/trust.go:236`); the cell comes from `geohash.EncodeWithPrecision(lat, lon, 7)` inside `CastVote` (`trust.go:337`); `independentCellCount` counts distinct cells over a read already deduped per account (`trust.go:116`, `DISTINCT ON` in `queries/votes.sql`); `CastVoteRequest` declares exactly latitude and longitude and the decoder runs `DisallowUnknownFields` under a 4 KiB cap (`handlers/votes.go:29,40,121`); `votes.js` takes coordinates from `navigator.geolocation` only and rejects rather than substituting (`votes.js:91`). PASS in this run: `TestBuildVoteTally`, `TestCastVoteReturnsFreshlyResolvedVisibility`, `TestCastVoteRejectsUnknownBodyField`, `TestConfirmsFromOneCellStayProvisional`, `TestVoteTransportHasNoLocationFallback`. A grep of templates, JS, service and handler strings for overclaiming words (fraud, tamper proof, guarantee, foolproof) finds none. Residuals the plans documented inside this threat, not separately accepted: GPS spoofing by devtools override or a mock-location app is not preventable at this budget (PITFALLS.md Pitfall 4), and the `sessionStorage` coordinate cache is readable by same-origin script (session scoped, no escalation). What stands on top of this is in the carried-forward section below. | closed |
| T-02-02 | Tampering / Repudiation (a cast vote silently not counted) | `votes` table, `InsertVote`, `CurrentVotesForReports` under concurrent load | high | mitigate | Structural: the schema has no cached counter and no unique key over `(report_id, account_id, kind)`, so there is no row to read then write. Verified: `00004_create_votes.sql` declares only the primary key and one non-unique index; `InsertVote` is a bare `INSERT` with no conflict clause and no lock (`queries/votes.sql`); `CastVote` opens no transaction (`trust.go:290`). PASS against real Postgres in this run: `TestCastVoteConcurrentSameAccountKeepsEveryRow`, `TestCastVoteConcurrent`, `TestVotesHaveNoUniqueKeyBeyondPrimaryKey`. | closed |
| T-02-03 | Elevation of Privilege (unauthorised resolve or reopen) | `isRetracted`, `BuildVoteTally`, vote route registration, `ListableInFeed`, `votes.js` and `activity.js` | high | mitigate | Four halves, all present. Decision: `isRetracted` admits exactly two ways in (the reporter's own resolve, or 2 independent resolve cells) and two ways out (the reporter's own reopen, or 2 independent reopen cells), with no "any account" branch (`visibility.go:155`, threshold at `visibility.go:45`). Write and tally: `BuildVoteTally` sets `ReporterResolved` and `ReporterReopened` only on a server-side match against the reporter id loaded by `ReportVoteContext` (`trust.go:149,165`), and `CastVote` takes the account id from the gate's context (`votes.go:105,133`). Reachability: all four routes sit inside the one `r.Group` wrapped by `requireVerifiedAccount` (`router.go:199,224-227`). Read: `ListableInFeed` excludes Retracted from both views and fails closed on an unknown state (`feed.go:62`). The UI adds no bypass: only the four allowlisted actions can be sent (`votes.js:43,156`) and `activity.js` carries no identity or threshold expression. PASS in this run: `TestResolve_RetractedAndReopen`, `TestBuildVoteTallyOnlyReporterSetsReporterReopened`, `TestCastVoteRequiresVerifiedAccount`, `TestNearbyExcludesRetractedFromBothViews`, `TestFeedNeverReturnsRetractedReports`, `TestProfileReopenIsInstantForTheReporter`, `TestResolveActionAllowlistIsUnchanged`, `TestReopenHasNoClientSideIdentityOrThreshold`, `TestReopenSuccessCopyIsAlwaysTheReopenedString`. The CSS guards in plans 02-08 and 02-13 make the inline confirmation behave as intended; they carry no authorisation logic and are not relied on here. | closed |
| T-02-04 | Tampering / Information Disclosure (resolver bypass from a read path) | `Resolve`, `ReportService.Nearby`, `ListableInFeed`, the Activity page, `feed.js` and `activity.js` | high | mitigate | `Resolve` is the single authority and is pure (`visibility.go:199`, no I/O, no context). Verified: its only three non-test call sites are the feed (`report.go:450`), the vote response (`trust.go:361`) and the Activity page (`auth.go:378`); the `NearbyReports` query block is unchanged since Phase 1 (last touched 2026-09-06) and holds no vote, visibility or retraction predicate, only Phase 1's read-time `expires_at > now()`; `handlers/reports.go` makes no call into the resolver; the feed filter is a switch over the resolver's own `Visibility` and nothing else (`feed.go:62`); the client refetches instead of filtering (`TestShowDisputedUsesOneSharedQueryParam`). Drift, recorded: the plan's literal grep gate for resolver references in `handlers/reports.go` returned 0 when written and returns 1 now, from a comment at `reports.go:338` added by 02-09. There is no code reference, so the property holds. PASS in this run: `TestFeedVisibilityMatchesCastVoteResponse` (real router and Postgres), `TestResolve_IsTotalAndDeterministic`, `TestNearbyReportsUsesIndex`, the partition subtests of `TestListableInFeed` and `TestNearbyHidesDisputedUnlessRequested`, `TestActivityForAccountResolvesVisibility`, `TestActivityModuleReusesTheSharedLabelMapAndTransport`. | closed |
| T-02-05 | Tampering (a reporter confirming their own report) | `CastVote` self-vote block, `BuildVoteTally`, `handlers.CastVote`, `applyOwnReportRule` | high | mitigate | Two server-side layers, with the hidden button as courtesy only. Verified: `CastVote` returns `ErrCannotVoteOwnReport` before `InsertVote`, scoped to content votes so the reporter's instant resolve and reopen still work, and a null reporter account never matches a caller (`trust.go:322`); the handler maps it to 403 (`votes.go:147`); `BuildVoteTally` also drops the reporter's own content rows at read time (`trust.go:153`). `castVote` in `votes.js` has no self-vote check of its own, so a wrong `is_own_report` flag would show the server's refusal instead of a silent no-op. PASS in this run: `TestCastVoteRejectsReporterContentVote`, `TestBuildVoteTallyExcludesReporterOwnContentVote`, `TestCastVoteAllowsReporterResolutionVote`, `TestCastVoteTreatsNullReporterAsNobody`, `TestCastVoteRejectsReporterSelfVote`, `TestReporterCanResolveOwnReportInstantly`, `TestOwnReportRuleRemovesControlsRatherThanDisablingThem`. | closed |
| T-01-02 | Information Disclosure (account identity or voter location reaching a public response) | `FeedReportResponse`, `CastVoteResponse`, the published OpenAPI spec | medium | mitigate | Verified: response types are declared field by field and carry no account id, email, session id or voter cell (`handlers/reports.go`, `handlers/votes.go:54`); the reporter id reduces to the boolean `is_own_report`; `your_vote` is the caller's own vote only (`ViewerContentVote` is keyed by the caller's account id); the report's `geohash` field is the report's own precision 8 value from Phase 1, not a voter cell; `docs/swagger.json` contains no `session_id`, `reporter_account` or `geohash_cell` (its only `email` fields belong to the login endpoint). PASS in this run: `TestNearbyReportsExcludesSessionID`, `TestSwaggerSpecCoversRoutes`. | closed |
| T-01-03 | Tampering (stored XSS via report-authored content) | every first-party JS module that builds DOM, and every Go template | high | mitigate | Verified: a grep of all ten first-party JS files for `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, `eval(`, `new Function` and `srcdoc` finds only a comment (`map.js:11`), including Phase 7's `modal.js` additions; non-test Go has no `template.HTML`, `template.JS`, `template.URL` or similar cast and no `text/template`, so `html/template` escaping covers every rendered value. Contract tests PASS in this run: `TestOwnReportRuleRemovesControlsRatherThanDisablingThem` (votes.js), `TestVisibilityTagCopyAndFallback` (visibility.js), `TestActivityModuleHasNoMarkupParsingSink`, `TestThemeModuleUsesNoMarkupParsingSink`. Non-blocking note: standing sink gates exist for 7 of the 10 modules (account-menu, auth, modal, theme, votes, visibility, activity). `app.js`, `feed.js` and `map.js`, which render report-authored text, have none, so the Phase 1 concern recorded in STATE.md is narrowed by this phase, not closed. The property itself holds today by direct inspection. | closed |
| T-01-17 | Tampering (unvalidated server or stored value reflected into a class name or attribute) | `visibility.js`, `app.js`, `newProfileReport`, `theme.js`, the `feed.js` URL reader, the `map.js` and `modal.js` basemap | medium | mitigate | Verified: `visibility.js` checks state and reason against fixed lists with `indexOf` before any class or attribute write and falls back to Provisional, never Live (`visibility.js:30,39,94,105`); `iconClass` validates category against `CATEGORIES` (`app.js:247`) and `severityClass` checks severity against `SEVERITIES` (`app.js:232`); the Go side guards the `vis-` class with `Visibility.Valid()`, again falling back to Provisional (`handlers/auth.go:368`); `theme.js` accepts a stored mode only if it is in the two-element `MODES` list and has one `setAttribute` call site, downstream of that check; `feed.js` collapses `show_disputed` to a boolean by comparing against the literal `'true'` and never interpolates it (`feed.js:517`). The basemap code has one literal vector style URL and one literal raster URL per file (`map.js:95,104`, `modal.js:559,568`), pinned by `TestBasemapBranchesPointAtCorrectHosts`. PASS in this run: that test, `TestVisibilityTagCopyAndFallback`, `TestThemeModuleValidatesStoredModeBeforeReflectingIt`. Drift, recorded: plan 02-11's root-attribute-to-tile-URL selector was reverted in `77b9804`, so that boundary no longer exists (closed by removal), and `theme.js` is now a two-state toggle (quick tasks 260923-qwi and 260923-rra) whose guard is the list check above. | closed |
| T-01-17 (static CSS) | Tampering (unvalidated value reflected into a class name or attribute) | static selector heads and declarations in `trust.css` and `main.css` (plans 02-08, 02-13, 02-14) | low | accept | Plan time disposition: those plans write only static selectors and declarations, so no runtime value is reflected and there is nothing to validate. Recorded to keep the register complete. See AR-02-01. | accepted |
| T-01-19 | Information Disclosure (visitor location inferred from map tile requests) | the OpenFreeMap tile origin and the OpenStreetMap raster fallback | low | accept | Plan time disposition (02-11): every map already streams the viewport to its tile host, and a second raster vendor would add a new recipient for a degraded path. 02-11's second style path was reverted in `77b9804`, so the exposure is now exactly Phase 1's: one vector style host and one raster fallback host, both pinned as whole literals by `TestBasemapBranchesPointAtCorrectHosts` (PASS in this run). See AR-02-02. | accepted |
| T-01-27 | Information Disclosure (sensitive content in an outbound email) | `internal/mailer/resend.go` | medium | mitigate | Verified: the disclaimer is a fixed literal with no interpolation (`resend.go:21`) and keeps both halves of the sentence (safe to ignore, no account created); the link is HTML-escaped before it enters the markup (`resend.go:67`); the rendered body is asserted to carry the full sentence (`TestVerificationEmailHTML`, `resend_test.go:24`, PASS in this run). | closed |

*Status: open · closed · accepted*
*Severity: critical > high > medium > low. Only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`.*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*
*The register is the union of the 15 plans' threat models, one row per unique id, dispositioned by the plan that owns it. Rows the other plans mark n/a (untouched) are pointers to the owning plan, not dispositions, so they are not repeated. T-01-17 has two rows because it carries two dispositions.*
*No supply chain row (`T-02-SC`): the plans record no new package for this phase, and `go.mod` and `go.sum` have no commit since 2026-09-14.*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-01-17 (static CSS) | Plans 02-08, 02-13 and 02-14 write only static selector heads and declarations. No runtime value is reflected anywhere, so there is nothing to validate. | gsd-planner at plan time (plans executed with the owner's approval), recorded by Claude (gsd-secure-phase) | 2026-10-07 |
| AR-02-02 | T-01-19 | Every map already sends its viewport to the tile host. Plan 02-11 added a second style on the same origin, since reverted, and no new host. A second raster vendor would add a recipient for a degraded path. | gsd-planner at plan time (plans executed with the owner's approval), recorded by Claude (gsd-secure-phase) | 2026-10-07 |

*Accepted risks do not resurface in future audit runs.*

*The residuals inside T-02-01 (GPS spoofing, the session cached coordinates) were documented by the plans as limitations within a mitigated threat, not as accepted threats, so they are not in this log. The carried-forward items below are not accepted either.*

---

## Carried Forward From The Code Review (outside the register, not counted, not accepted)

These come from `02-REVIEW.md` and are repeated in `02-VERIFICATION.md` as advisory items the verifier asked to keep in front of the owner. They were not threats in the plan time register, so they do not change `threats_open`. They are kept out of the Accepted Risks Log on purpose, because that log never resurfaces and the owner has not accepted them.

| Review id | What stands today | What it does and does not allow | Tracked in |
|-----------|-------------------|---------------------------------|------------|
| CR-01 (STATE.md calls it "WR-01") | The four vote routes have no per account or per session throttle. Voter coordinates are client supplied and cannot be proven. A new verified account costs one email click: the request-link route has a 45 second per address cooldown (`service/auth.go:46`) and a per IP bucket of burst 5 with one token per 60 seconds (`router.go:89`). | One account re-voting cannot add independent cells, because `CurrentVotesForReports` keeps only the latest row per report, account and kind. So the missing throttle mostly means append-only row growth and request volume. The real cost of gaming the predicate is fresh verified accounts times coordinates in different precision 7 cells (roughly 150 m each). With `IndependentAgreementThreshold` at 2, two such accounts satisfy it. This is the property the project calls its Core Value. | ROBUST-04 in Phase 4: plan 04-03 (per key limiter) and plan 04-09 (limiter on the four vote routes), written and not executed. TRUST-05 (diversity weighting) and TRUST-07 (reliability decay) in Phase 3, planned and not executed. STATE.md suggests pulling the vote limiter forward if a public demo comes before Phase 4. |
| WR-01 (review) | `POST /api/reports` has no per account or per IP limiter. | One verified session can flood the feed. A resource and usability risk in exactly the scenario the app exists for. | ROBUST-04 in Phase 4: plan 04-15, written and not executed. |
| WR-02 (review) | The cached voter location has no staleness or plausibility check. | The accuracy of the independence signal degrades within a session. Not an adversarial bug by itself. | Phase 3 reliability work. |

Re-check this section when Phase 4's ROBUST-04 plans execute. Phase 4's own security review is the natural place to close or reopen it.

---

## Verification Evidence

- **Method.** ASVS level 1, State B (no prior SECURITY.md), register authored at plan time, so the short circuit applied and the auditor subagent was not spawned. Each mitigation was checked directly against the current code (file and line references are in the register), then the full suite was run.
- **Commit under audit.** `c97273f`.
- **Test run.** `go test ./... -p 1 -count=1 -v` with a scrubbed environment against a disposable database, `p2_secure_audit`, created for this run and dropped afterward. 255 top level tests PASS, 0 FAIL, 0 SKIP, so every database backed test actually ran. All 36 test names the plans cite as controls were found and PASS. `pinalert_test` and `pinalert` were not touched.
- **Out of register.** Phase 7's `GET /api/geocode` was added after this phase executed, inside the same verified gate. It is covered by `07-SECURITY.md` and is not part of this register.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-07 | 11 rows (10 unique ids) | 9 closed + 2 accepted | 0 | Claude (gsd-secure-phase, State B, built from the 15 plans' threat models authored at plan time; ASVS L1 short circuit applied after direct code verification and a full database backed test run, so no auditor subagent was spawned) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-07 (the register only; what remains open outside it is in the carried-forward section)
