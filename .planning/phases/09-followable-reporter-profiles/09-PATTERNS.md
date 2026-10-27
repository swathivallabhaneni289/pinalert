# Phase 9: Followable reporter profiles - Pattern Map

**Mapped:** 2026-10-25
**Files analyzed:** 41 new or modified (code, SQL, web, docs) plus the test pins that move
**Analogs found:** 38 / 41 (3 have no analog)
**Source state:** main at 02e877e (Phase 3.3 fully merged, 15 of 15 plans). Phases 5, 7.1 and 8 are NOT built; see "Re-read after they land".

Line numbers below were read from main at 02e877e. Phases 5, 7.1 and 8 edit several of the same files, so re-grep by the quoted symbol, not the line number, at planning time.

## Re-read after they land (flag for the planner)

Phases 5, 7.1 and 8 build before this phase. The files below are touched by them AND by Phase 9. Every Phase 9 plan that edits one must re-read it from the merged tree first, and migrations take "the next free number at execution time" (main has 00011 as the last one; never hardcode a number).

| File | Who else changes it | What to re-check |
|------|--------------------|------------------|
| `internal/service/pushkinds.go` | Phase 5 plan 05-11 (moves `PushKindOfficial` into `PushKinds`, leaves `ReservedPushKinds = [follow]`, `TestPushKindsClosed` pins "follow is not Valid"); Phase 8 (comment kind) | Final shape of `PushKinds` and `ReservedPushKinds`; Phase 9 flips `follow` in and empties the reserved list |
| `internal/service/pushworker.go` | Phase 5 (adds `pushworker_official.go`, edits shared worker), Phase 8 | `stillHolds`, `confirmedBaseKind`, `kindSwitchOn`, step 5 distance check |
| `internal/service/pushcopy.go` | Phase 5 (`pushcopy_official.go`), Phase 8 | `BuildPushText` switch and the "no reporter text field" test |
| `internal/service/updates.go`, `updatescopy.go`, `updatesservice.go` | Phase 8 (comment kind in `updates_kind_valid`) | `UpdateKinds`, `UpdateEventKinds`, `MayNotifyAboutReport` allowance branch |
| `internal/store/queries/push.sql`, `updates.sql` | Phase 5 (`official_push.sql` sibling), Phase 8 (additions to both) | Current text of `uq_push_outbox_first`, `ON CONFLICT ... WHERE kind IN (...)` clauses, `updates_kind_valid` |
| `internal/store/migrations/*` | Phases 5, 7.1, 8 each add one or more | Copy the CHECK text of `push_outbox_kind_valid` and `updates_kind_valid` from the newest migration that changed them, not from 00009 or 00011 |
| `internal/service/accountdelete.go` (+ test) | Phase 5 (deletes official subscriptions), 7.1 (credentials, codes), 8 (comments, likes) | `AccountDeleteQuerier` method list and the order that `accountdelete_test.go` pins |
| `internal/service/profile.go` | Phase 5, 7.1 (username, reserved words, Name required) | Name screening hook (gap G1), `cleanText`, any exported reserved word check |
| `internal/service/alertsettings.go`, `internal/api/handlers/alerts.go`, `handlers/me.go` | Phase 5 (official switch row) | Settings model fields, `AlertSettingsRequest` |
| `internal/api/handlers/settings.go`, `web/templates/settings.html.tmpl`, `web/static/js/settings.js`, `settings-alerts.js` | Phase 5 (Official alerts row), 7.1 (Username row in About you), 8 | Row order in People and reports, About you rows, view model fields |
| `internal/api/router.go` | all three | Where the gated group ends, `RateLimits` wiring, `skipSessionFor` rule set (Phase 5 plan 05-04 generalises it) |
| `internal/api/handlers/ratelimit_gate.go`, `cmd/server/main.go`, `api.RateLimitsDefault`, `resolveRateLimits` | Phase 8 (comment and like buckets) | A new bucket must be added in all four places (Phase 8 research line 360) |
| `internal/api/gate_test.go` (`TestAccessGateExemptRoutes`) | Phases 5, 7.1, 8 | Phase 9 adds NO exempt rows; its new paths must show as 401 when anonymous |
| `internal/api/handlers/*_e2e_test.go` helpers `newVerifiedClient`, `verifySession`, `newE2EServerWith` | Phase 7.1 rewrites sign in (magic link replaced by password and typed code) | Call helpers by name only; never copy the old request-link steps |
| `web/templates/login_gate.html.tmpl`, `web/static/js/auth.js` | Phases 5 and 7.1 | Not edited by Phase 9; listed only so Phase 9 tests do not depend on their ids |
| `web/static/js/map.js`, `trust.js`, `votes.js`, `feed.js`, `web/templates/index.html.tmpl`, `report_view` pieces | Phase 8 (report page, popup footer, comment count) | Where "See this reporter" goes in the explanation panel; the star box after resolve; `TestPopupCardSurfaceAndGeometry` amendments |
| `docs/TRUST-MODEL.md`, `docs/PUSH.md`, `docs/MODERATION.md`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | all three | Regenerate swag last with one binary and commit the whole `docs/` diff; PUSH.md and the "Phone alerts" section already exist on main |
| `web/places_contract_test.go`, `places_e2e_test.go`, `settings_rows_contract_test.go`, `layout_rules_contract_test.go`, `css_contract_test.go` | Phase 5 plans list several; 7.1 and 8 add pins | `waitingSettingsLabels` ban list, `sizeOnlyNamed`, `touchControls`, `phaseScripts`, `noiseCheckedFiles`, `laterIcons` |
| Phase 8 comment read/count query | Phase 8 | A "comments visible to viewer" fragment used by list and count, so block (D-12) can hide a blocked person's comments. If missing, Phase 9 edits Phase 8 queries |
| Phase 7.1 `accounts.username` | Phase 7.1 | Column name, case rule, charset (no `@`, no `/`), lookup helper, reserved word function |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/service/stars.go` (new) | utility (pure) | transform | `internal/service/reliability.go` (`reporterTagForScore`) | exact |
| `internal/service/reliability.go` (mod: 75 to 70, comments) | service (pure) | transform | itself | exact |
| `internal/service/reporter.go` (mod: `WithRatings` option, new fields) | service (pure) | transform | itself (`ReporterSignals`) | exact |
| `internal/service/ratings.go` (new) | service | CRUD | `internal/service/mutes.go` | role-match |
| `internal/service/people.go` (new) | service | CRUD | `internal/service/mutes.go` | role-match |
| `internal/service/profilereports.go` (new) | service | request-response (read) | `internal/service/historyread.go` (`Focus`, `ReporterHistoryReports` path) | role-match |
| `internal/service/profilecopy.go` (new) | utility | transform | `internal/service/trustcopy.go` | exact |
| `internal/service/followsweep.go` (new) | service | batch / event-driven (ticker) | `internal/service/pushdispatch.go` (`Run` ticker, sweep) | role-match |
| `internal/service/coarsen.go` (mod: profile coarsener) | utility (pure) | transform | itself (`CoarsenFinishedRescue`) | exact |
| `internal/service/updates.go` (mod: `UpdateFollowedPosted`) | model / predicate | transform | itself (`UpdateReliableNearby`) | exact |
| `internal/service/updatesalerts.go` or sibling `updatesfollow.go` (new step) | service | event-driven (poll) | `internal/service/updatesalerts.go` (`refreshReliableAlerts`) | exact |
| `internal/service/updatescopy.go` (mod) | utility | transform | itself | exact |
| `internal/service/pushkinds.go` (mod: follow valid) | model | n/a | itself | exact |
| `internal/service/pushdispatch.go` (mod: `followBranch`, ratings in `reporterIsReliable`) | service | event-driven | itself (`reliableBranch`) | exact |
| `internal/service/pushworker.go` (mod: `stillHolds`, step 5, `confirmedBaseKind`, `kindSwitchOn`) | service | event-driven | itself | exact |
| `internal/service/pushcopy.go` (mod: follow case) | utility | transform | itself | exact |
| `internal/service/accountdelete.go` (mod: graph + ratings cleanup) | service | CRUD | itself | exact |
| `internal/service/report.go`, `trust.go`, `historyread.go`, `auth.go` (mod: pass ratings into `ReporterSignals`; `reporter_profile_path` in `baseView`) | service | request-response | themselves | exact |
| `internal/store/migrations/<next free>_profiles_and_ratings.sql` (new) | migration | n/a | `00009_add_updates_and_alerts.sql`, `00011_add_push.sql` | exact |
| `internal/store/migrations/<next free>_follow_notes.sql` (new) | migration | n/a | `00011_add_push.sql` | exact |
| `internal/store/queries/people.sql` (new) | model (sqlc) | CRUD | `queries/updates.sql` mute block (lines 268 to 318) | exact |
| `internal/store/queries/ratings.sql` (new) | model (sqlc) | CRUD / aggregate | `queries/reliability.sql` + `updates.sql` | role-match |
| `internal/store/queries/push.sql`, `updates.sql` (mod) | model (sqlc) | event-driven | themselves | exact |
| `internal/api/handlers/people.go` (new) | controller | request-response | `internal/api/handlers/updates.go` (`MuteReport`, `muteHandler`) and `me.go` (`UpdateAlertSettings`) | exact |
| `internal/api/handlers/ratings.go` (new; incl. moderator routes) | controller | request-response | `handlers/updates.go` + `handlers/moderation.go` | role-match |
| `internal/api/handlers/person_page.go` (new, `GET /u/{username}` and `GET /me`) | controller | request-response (HTML) | `handlers/settings.go` (`SettingsPage`) | exact |
| `internal/api/handlers/settings.go` (mod: new counts and flag) | controller | request-response | itself | exact |
| `internal/api/handlers/ratelimit_gate.go` (mod: `Rate`, `People` buckets) | middleware | request-response | itself | exact |
| `internal/api/router.go` (mod: routes, services) | config / route | request-response | itself | exact |
| `cmd/server/main.go` (mod: start sweep, bucket constants) | config | batch | itself (dispatcher/worker start) | exact |
| `web/templates/person.html.tmpl` (new) | component | request-response | `web/templates/settings.html.tmpl` (+ `profile.html.tmpl` for list scaffolding) | role-match |
| `web/static/js/person.js` (new) | component | request-response | `web/static/js/settings-alerts.js` (windows, switch save) and `mute.js` | role-match |
| `web/static/css/person.css` (new) | config | n/a | existing `place-*` and `app-window` CSS | role-match |
| `web/static/icons/star.svg` (new) | asset | n/a | `web/static/icons/bell.svg` | exact |
| `web/templates/settings.html.tmpl` + `settings.js` / `settings-alerts.js` (mod: 4 rows, 2 windows) | component | CRUD | the Muted reports row and window | exact |
| `web/templates/account_header.html.tmpl` (mod: My profile item) | component | n/a | itself | exact |
| `web/static/js/trust.js`, `votes.js` (mod: "See this reporter", star box) | component | request-response | themselves | exact |
| `docs/TRUST-MODEL.md`, `docs/PUSH.md`, `docs/MODERATION.md` (mod) | docs | n/a | existing sections | exact |
| Tests that move (see section) | test | n/a | themselves | exact |

## Pattern Assignments

### `internal/service/stars.go` (utility, pure transform)

**Analog:** `internal/service/reliability.go` (constants block at lines 140 to 151, `reporterTagForScore` at 202 to 210) and `internal/service/reporter.go` header.

**File header and purity pattern** (`reporter.go` lines 1 to 10): a leading comment saying the file is pure, `now` is a parameter, no I/O. Copy that style. New constants (`MinRatersForStars`, the tapped weight, follow caps) must also be added to `p20ConstFiles` / `p20Constants` in `trust_model_doc_test.go` and to the constants table in `docs/TRUST-MODEL.md`.

**Constant style to copy** (`reliability.go` 148 to 151):
```go
	// ReliableMinScore is the lowest score tagged Reliable reporter (D-15, Q2).
	ReliableMinScore = 75
	// MixedMinScore is the lowest score tagged Mixed record (D-15, Q2).
	MixedMinScore = 35
```
Phase 9 changes `ReliableMinScore` to 70 here and rewrites the comment doc invariants (I1) and the doc comment near lines 234 to 238 ("75 or more", "boundaries 75, 35 and 25 are exact").

**Core pattern to implement** (from RESEARCH Pattern 2; integer math, floor to a tenth so 69 reads 3.4 and never 3.5):
```go
const MinRatersForStars = 5

func CombinePoints(outcome, raters, tapSum int) int {
	if raters < MinRatersForStars {
		return outcome
	}
	num := int64(outcome)*int64(raters) + 20*int64(tapSum)
	den := int64(2 * raters)
	return int((2*num + den) / (2 * den)) // round half up
}
func StarsTenths(points int) int { return points / 2 } // floor
```
Never use `%.1f` on a float for stars.

---

### `internal/service/reporter.go` (modify: `ReporterSignals` option)

**Analog:** itself. Current signature (line 123):
```go
func ReporterSignals(accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time) map[int64]ReporterSignal {
```
Final loop to extend (lines 150 to 153):
```go
	signals := make(map[int64]ReporterSignal, len(requested))
	for id := range requested {
		signals[id] = newReporterSignal(ScoreReliability(outcomes[id], now))
	}
```
Add a variadic `opts ...SignalOption` (26 test sites keep compiling). Inside, only when `rel.Score != nil` apply `CombinePoints`, then recut the tag with `reporterTagForScore` (so New stays New, D-13). Keep `Reliability.Score` as the outcomes part; `ReporterSignal.Score` becomes combined points. `newReporterSignal` (lines 50 to 63) copies the Score pointer; keep that copy discipline. The five production callers that must all pass `WithRatings(...)`:
- `internal/service/report.go:1018` (Nearby feed), loader helper goes beside `reporterHistoryParams` at `report.go:824`
- `internal/service/trust.go:591` (vote response)
- `internal/service/historyread.go:214` (`rc.signals`)
- `internal/service/auth.go:622` (Activity)
- `internal/service/pushdispatch.go:640` inside `reporterIsReliable` (easy to forget)

**Viewer dependent field rule:** `reporter_profile_path` goes on the report row in `baseView` (`historyread.go` around lines 296 to 325, beside `viewer_vote` and `is_own_report`), never inside `Trust`, or the parity tests `TestFeedSignalsMatchCastVoteSignals` and `TestFeedTrustMatchesCastVoteTrust` break. JSON key names must not match `cell|account|session|email|voter`.

---

### `internal/service/ratings.go` and `internal/service/people.go` (service, CRUD)

**Analog:** `internal/service/mutes.go` (whole file, 174 lines).

**Querier + compile time assertion + clock option pattern** (mutes.go lines 22 to 56):
```go
type MuteQuerier interface {
	ReportForMute(ctx context.Context, reportID int64) (sqlcgen.ReportForMuteRow, error)
	MuteReport(ctx context.Context, arg sqlcgen.MuteReportParams) error
	...
}

// The generated Queries value satisfies the narrow querier.
var _ MuteQuerier = (*sqlcgen.Queries)(nil)

type MuteService struct {
	q   MuteQuerier
	now func() time.Time
}
type MuteOption func(*MuteService)
func WithMuteClock(now func() time.Time) MuteOption { ... }

// NewMuteService stores the querier. It never dereferences it, so a router
// built with a nil store still constructs.
func NewMuteService(q MuteQuerier, opts ...MuteOption) *MuteService {
```
Copy exactly: narrow querier per service, `var _ X = (*sqlcgen.Queries)(nil)`, `With<Service>Clock` option, constructor that never dereferences.

**Error handling pattern** (mutes.go lines 66 to 91): `errors.Is(err, pgx.ErrNoRows)` maps to a sentinel (`ErrReportNotFound`); other errors wrap as `fmt.Errorf("service: Mute: reading report %d: %w", ...)`. Account id always comes in as a parameter from the gate, never from a request value (file header comment).

**Idempotent write and "always works" unwrite** (mutes.go 93 to 101): `Unmute` never reads the target so it works on anything; copy for Unfollow, Unmute person, Unblock, and rating Clear.

**Multi statement atomicity:** for opt out (delete follows, queued follow pushes) and block (insert block, delete follows both ways) use the `AccountTxFunc` pattern from `accountdelete.go` (`run := func(q AccountDeleteQuerier) error {...}`, tx may be nil in test harnesses, lines 80 to 140), or the single statement CTE style of `MuteReport` (updates.sql 275 to 290: `WITH muted AS (INSERT ... ON CONFLICT DO NOTHING RETURNING ...) DELETE ...`).

**Text screening for the Name (gap G1):** reuse `CheckText` / `ScreenAbuse` from `internal/service/textcheck.go`; fail closed when the check is `unfinished` (switch does not turn on). The reserved word check comes from Phase 7.1 (re-read).

---

### `internal/service/profilereports.go` (service, read through the one shared pipeline)

**Analog:** `internal/service/historyread.go` (`loadReadContext(withSignals true)`, `classify`, `baseView`, `buildFinishedView`, `Focus`). Candidate query analog: `ReporterHistoryReports` in `internal/store/queries/reliability.sql` (lateral per session, `idx_reports_session_expires`).

Pattern: a new candidate read keyed by account (reports whose session belongs to the account, `created_at > now - 30 days`, newest first, cap 100 plus one), then reuse the existing pipeline unchanged. Never write a second visibility rule. Any direct `Resolve` call must carry stored moderation state (`TestResolveCallersMentionModeration`); going through `classify` satisfies it. A `FeedViewNone` report is listed nowhere. Show Active, Disputed, Resolved, Past.

**Coarsening hook:** add a flag on `viewInput` so `buildFinishedView` calls a profile coarsener; see next entry. Finished rows are plain, never links (Focus does not coarsen non Rescue finished reports). Live rows link to `/?report=ID`.

---

### `internal/service/coarsen.go` (modify: `CoarsenFinishedForProfile`)

**Analog:** itself, `CoarsenFinishedRescue` (lines 41 to 60 in the read above). Copy its shape verbatim: pure, by value, all leaking fields change together:
```go
	lat, lon, cell := CoarsePoint(r.Latitude, r.Longitude)
	r.Latitude = lat
	r.Longitude = lon
	r.Geohash = cell
	r.Description = ""
	r.DistanceKm = nil
	if centre != nil {
		d := haversineKm(centre.Lat, centre.Lon, lat, lon)
		r.DistanceKm = &d
	}
```
`CoarsePoint` currently fixes precision at `ApproxGeohashPrecision = 6`; the profile coarsener needs precision 5 (research A4), so add a precision parameter or a sibling function. Rescue keeps precision 6 with no text (03.1 D-23). Distance is always to the coarse point, never the exact one.

---

### `internal/service/profilecopy.go` (utility, transform)

**Analog:** `internal/service/trustcopy.go` (lines near 174 and 230: "Score N out of 100." sentences) and `historycopy.go`. Every profile sentence is built on the server; JS only draws strings with `textContent`. Keep "Score N out of 100." where it is and make N the combined points so the word and the number agree. Plain punctuation only (no em or en dashes; a test scans).

---

### `internal/service/updates.go` + `updatescopy.go` (modify: new kind `followed_posted`)

**Analog:** itself. Add `UpdateFollowedPosted UpdateKind = "followed_posted"` beside `UpdateReliableNearby` (line 26), add to `UpdateEventKinds` (line 47) and `UpdateKinds` (line 51), then extend the one predicate:

`MayNotifyAboutReport` (updates.go line 248): the allowance branch today is `if kind == UpdateReliableNearby {`. Change to `kind == UpdateReliableNearby || kind == UpdateFollowedPosted`; never a second copy (D-29). Touch points to widen: `updatescopy.go:44` and `:65` (the two `case UpdateReliableNearby:` branches), `handlers/updates.go:42` (swagger enum), `handlers/activity.go:195`, the `updates_kind_valid` CHECK, and the tests that pin the closed list. Row words are built at read time from the current trust object, like the Reliable note.

---

### `internal/service/updatesalerts.go` / new follow step (service, event-driven poll)

**Analog:** `refreshReliableAlerts` (updatesalerts.go lines 66 to 185). Copy the structure:
1. Read settings; `pgx.ErrNoRows` returns nil with no error.
2. Filter candidates with the single predicate:
```go
		if !MayNotifyAboutReport(UpdateReliableNearby, v.Visibility, v.ExpiresAt, now) {
			continue
		}
		if v.Trust.Reporter.Tag != ReporterReliable {
			continue
		}
		if v.IsOwnReport || v.ViewerFollows || v.ViewerMuted {
			continue
		}
```
For follow: swap in `UpdateFollowedPosted`, read `account_follows` rows with `notify = true`, candidate reports are the followees' reports with `expires_at > now` and created after `notify_since`, cap with a `FollowedReportsCap`-style constant, skip person mutes and blocked pairs, tag must be Reliable now (through ratings).
3. Insert with the same ceiling/baseline CTE style as `InsertReliableAlert` (updates.sql line 154 onward) and treat `pgx.ErrNoRows` from the insert as the normal "duplicate or over the hourly ceiling" answer:
```go
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("writing alert for report %d: %w", v.ID, err)
		}
```
**One note per account and report across both producers:** the Reliable step skips a report whose reporter the reader follows with the bell on; widen the unique index `uq_updates_reliable_once` (or add a sibling) and update every `ON CONFLICT ... WHERE kind = 'reliable_nearby'` clause to match. A mismatched inference predicate is a runtime SQL error, not a compile error.

The step runs on every 30 second poll inside `Refresh`, so bound it (caps as named constants in the docs constants table).

---

### `internal/service/pushkinds.go` (modify)

**Analog:** itself. Today (lines 38 to 45):
```go
var PushKinds = []PushKind{PushKindCritical, PushKindReliable, PushKindChanged, PushKindConfirmedUpdate}
var ReservedPushKinds = []PushKind{PushKindOfficial, PushKindFollow}
```
Phase 9 adds `PushKindFollow` to `PushKinds` and removes it from `ReservedPushKinds` (on the Phase 5 shape, `ReservedPushKinds` becomes empty). Flip `TestPushKindsClosed`, `TestPushCapsAreNamed`. Keep `TestNotificationsNoFollowOrOfficialRow` true (web/settings_rows_contract_test.go:692): the per person bell replaces the Settings switch (D-11), so Settings gets no follow switch row.

---

### `internal/service/pushdispatch.go` (modify: `followBranch`, ratings in `reporterIsReliable`)

**Analog:** `reliableBranch` (lines ~440 to 540) and `reporterIsReliable` (lines 628 to 641).

Pattern to copy for `followBranch` (runs before `reliableBranch`, so follow wins the shared first alert slot): guard on `f.ReporterAccountID == nil`, `MayNotifyAboutReport(UpdateFollowedPosted, ...)`, the `PushInitialWindow` check, the fanned/capped state check; then Refresh each follower with the bell on:
```go
	if d.updates != nil {
		for _, r := range recipients {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := d.updates.Refresh(ctx, r.AccountID); err != nil {
				// The text of this error is not logged; it comes from a layer
				// that reads reports and places.
				log.Printf("push dispatch: refresh failed for account %d, report %d", r.AccountID, f.ID)
			}
		}
	}
	if _, err := d.q.EnqueueReliableFromUpdates(ctx, sqlcgen.EnqueueReliableFromUpdatesParams{
		ExpiresAt: pushEarlier(f.ExpiresAt, run.now.Add(PushOtherTTL)),
		Now:       run.now,
		ReportID:  f.ID,
	}); err != nil {
```
Write `EnqueueFollowFromUpdates` as a sibling of `EnqueueReliableFromUpdates` ("one row, two channels": the bell step writes the rule, push only follows it). `reporterIsReliable` (lines 628 to 641) calls `ReporterSignals([]int64{accountID}, history, votes, now)`; it must pass the ratings or a phone note says Reliable while the feed disagrees.

---

### `internal/service/pushworker.go` (modify)

**Analog:** itself. Four places, each a closed list today:

`stillHolds` (lines ~455 to 472): add
```go
	case PushKindFollow:
		return MayNotifyAboutReport(UpdateFollowedPosted, f.Vis, f.ExpiresAt, run.now) &&
			w.followStillHolds(ctx, run) // row exists, notify on, reporter public, Reliable now, no mute or block
```
`confirmedBaseKind` (lines ~505 to 517): the switch `case PushKindCritical, PushKindReliable:` gains `PushKindFollow`.

`kindSwitchOn` (lines ~521 to 535): follow has no account switch; the check is the follow row, handled in `stillHolds`.

**Step 5 distance exception** (lines ~375 to 391): today
```go
	if run.kind != PushKindChanged {
		if !haveHome || distance == nil {
			return w.skip(ctx, run, pushSkipNoHome)
		}
		if *distance > float64(home.AlertRadiusKm) {
			return w.skip(ctx, run, pushSkipOutOfRange)
		}
	}
```
A follow note, and a confirmed update whose base is a follow, must be exempt, or followers with no home area silently get nothing. `CountAccountPushesInWindow` (push.sql) kind list also gains follow so `PushPerAccountPerHour = 4` counts it.

---

### `internal/service/pushcopy.go` (modify)

**Analog:** itself (`BuildPushText`, `PushTextInput`). Add a `follow` case and a confirmed update built from a follow base. `PushTextInput` deliberately has no reporter name field (`TestBuildPushTextHasNoReporterTextField`); keep the lock screen free of username, text and coordinates. No noise words (`TestNoNoiseWordsInUserCopy`), no promise of sound or vibration (owner rule).

---

### `internal/service/followsweep.go` (service, ticker batch)

**Analog:** `pushdispatch.go` `Run` and its interval constants (`PushChangesSweepInterval` is 5 minutes), wired in `cmd/server/main.go` next to the dispatcher and worker. Pattern: plain `time.Ticker` goroutine (stack rule: no cron library), distinct followees in batches of 200 (`MaxReporterAccountsPerRead`), compute signals with ratings, delete follows and queued follow pushes of those no longer Reliable or no longer public. Pair it with a send time recheck (worker) and evaluate on touch (profile read, rating write).

---

### `internal/service/accountdelete.go` (modify)

**Analog:** itself. The doc comment on `DeleteAccount` (lines ~82 to 104) lists the person's own rows that are deleted (updates, cursors, mutes, alert settings, queued phone alerts, subscriptions). Add: follows both ways, person mutes and blocks the account made, ratings the account gave, queued follow pushes; `ScrubDeletedAccount` also sets `profile_public = false`. `AccountDeleteQuerier` is a narrow interface and `accountdelete_test.go` pins the interface and the order, so it is amended. The "no effect on others" sentence becomes false for ratings (gap G6): state it in `docs/TRUST-MODEL.md`.

---

### `internal/store/migrations/<next free>_profiles_and_ratings.sql` and `<next free>_follow_notes.sql` (migration)

**Analog:** `00011_add_push.sql` (named CHECK constraints, partial unique indexes, header comments naming the decision each table serves) and `00009_add_updates_and_alerts.sql` (`report_mutes`).

Style to copy (00011):
```sql
-- +goose Up
CREATE TABLE push_outbox (
    ...
    CONSTRAINT push_outbox_kind_valid CHECK (
        kind IN ('critical', 'reliable', 'changed', 'confirmed_update')),
    ...
);
CREATE UNIQUE INDEX uq_push_outbox_first
    ON push_outbox (account_id, report_id)
    WHERE kind IN ('critical', 'reliable');
```
Schema migration: `accounts.profile_public` (+ timestamp), `account_follows`, `person_mutes`, `person_blocks`, `profile_ratings` with named CHECKs and composite primary keys (shape in RESEARCH Part 3). Notification migration: drop and re-add `push_outbox_kind_valid` (add `'follow'`), recreate `uq_push_outbox_first` with `kind IN ('critical','reliable','follow')`, widen `updates_kind_valid`; copy the current CHECK text from the newest migration that changed it (Phase 5 may have). Both need an exact `-- +goose Down`. Never hardcode the file number. Names are person scoped: not `follows`, `mutes`, `blocks` (those mean report level).

---

### `internal/store/queries/people.sql` and `ratings.sql` (sqlc)

**Analog:** `queries/updates.sql` mute block (lines 268 to 318): `ReportForMute :one`, `MuteReport :exec` (CTE insert with `ON CONFLICT DO NOTHING` plus delete), `MutedReportsForAccount :many` with `sqlc.arg(...)::type` casts and `LIMIT sqlc.arg(row_limit)::int`, `CountMutedReports :one`.
```sql
-- name: CountMutedReports :one
SELECT COUNT(*)::bigint
FROM report_mutes
WHERE account_id = sqlc.arg(account_id)::bigint;
```
Rules: every query carries a `-- name:` line and a comment saying which decision it serves; keyset pagination for Followers and Following (`created_at, follower_id`, 50 per page); list queries split named (public, not deleted, username not null) from "others" counts; blocked pairs hidden. Run `make sqlc` and commit generated files. Eligibility (`RaterMayRate`) and aggregates (`RatingTalliesForReporters`) are in RESEARCH Code Examples; the history read in `reliability.sql` is the aggregate-by-account analog.

---

### `internal/api/handlers/people.go` and `ratings.go` (controller, request-response)

**Analog:** `handlers/updates.go` for body free actions, `handlers/me.go` for JSON body actions.

Body free action handler (updates.go `MuteReport`/`muteHandler`, lines 324 to 390):
```go
// @Summary Turn updates off for a report
// @Tags reports
// @Param id path int true "Report id"
// @Success 204 "Updates are off for this report"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/mute [post]
func MuteReport(mutes *service.MuteService) http.HandlerFunc {
	return muteHandler("MuteReport", mutes.Mute)
}
...
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		reportID, ok := parseReportID(w, r)
		if !decodeEmptyBody(w, r) { return }
```
JSON body handler (me.go `UpdateAlertSettings`, lines 233 to 276): `account.FromContext` guard, `http.MaxBytesReader(w, r.Body, maxMeBodyBytes)`, `dec.DisallowUnknownFields()`, `writeFieldError(w, 400, "body", "Request body is missing or malformed.")`, `errors.As(err, &ve)` for `service.ValidationError` mapped to `writeFieldError(w, 400, ve.Field, ve.Message)`, `Cache-Control: no-store`, `writeJSON`. Copy for `notify` `{"on":bool}` and `rating` `{"stars":1..5}`. Mime check pattern: `decodePushBody` in `handlers/push.go` (strict `application/json`).

Routes: flat literal paths in the gated group, POST verb paths (`/api/people/{username}/follow`, `unfollow`, `notify`, `mute`, `unmute`, `block`, `unblock`, `remove-follower`, `rating`, `rating/clear`); GET `/api/people/{username}`, `/followers`, `/following`, `/reports`. Unknown, private, deleted and blocked all answer the same 404 body and status. Swag comments use plain punctuation (backlog 999.8: long dashes leak into the public `/swagger`); `@Tags people`; then `make swag` and extend `TestSwaggerSpecCoversRoutes`.

**Moderator rating routes:** register inside the existing moderator group with `r.Handle` (router.go lines 710 to 717), the group already answers a byte equal 404 to everyone else:
```go
	r.Group(func(r chi.Router) {
		r.Use(requireModerator(deps.Sessions, deps.ModeratorEmails))
		r.Handle("/api/moderation/reports/{id}/keep", handlers.ModerationKeep(moderation))
```
Handler analog `handlers/moderation.go`; screen analog `moderate.html.tmpl` + `moderate.js`.

---

### `internal/api/handlers/person_page.go` (controller, HTML)

**Analog:** `handlers/settings.go` `SettingsPage` (lines 63 to 123).
```go
func SettingsPage(tmpl *template.Template, accounts *service.AccountService, mutes *service.MuteService, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: SettingsPage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		...
		chrome := cfg.Chrome.withDefaults()
		vm := settingsViewModel{ AssetVersion: cfg.AssetVersion, Chrome: chrome, AccountID: acc.ID,
			Nav: navFor(NavSettings, navTitle("Settings"), navBack("/")), ... }
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, settingsTemplateName, vm); err != nil {
```
Carries no swag block (HTML, outside `@BasePath /api`). Routes: `GET /u/{username}` and `GET /me` (303 to `/u/<my username>`). Do NOT edit `profile.html.tmpl` or `handlers.Profile`: `/profile` is My activity (`<title>Pinalert: My activity</title>`, pinned by `profile_nav_contract_test.go`). A failed read is a logged 500.

---

### `internal/api/handlers/settings.go` + `web/templates/settings.html.tmpl` (modify)

**Analog:** the Muted reports plumbing. View model field `MutedCount int` (settings.go lines 38 to 41) is read through `mutes.MutedCount` with a nil check and a logged 500 on failure (lines 76 to 87). Add `ProfilePublic bool`, `FollowingCount`, `FollowersCount`, `BlockedCount` the same way.

Template row to copy (settings.html.tmpl lines 150 to 163, section People and reports):
```html
        <button type="button" class="place-row" id="settings-muted-row" aria-haspopup="dialog">
          <span class="place-row__text">
            <span class="place-row__label">Muted reports</span>
          </span>
          <span class="place-row__value" id="settings-muted-value">{{.MutedCount}}</span>
          <span class="app-icon app-icon--chevron-right" aria-hidden="true"></span>
        </button>
```
Order fixed by 03.4 D-25: Let others follow me (switch), Following, Followers, Muted reports (exists), Blocked people. Rewrite the section line ("Reports you have turned updates off for."). Window markup to copy (lines 280 to 308): `app-window app-window--edit` with `app-window__panel role="dialog" aria-modal="true"`, `app-window__head`, close button with `app-icon--x`, loading skeleton rows, `<ul class="muted-list" ... hidden>`, empty state, error state with a retry button. Switch row: copy the Critical reports switch row in the `#settings-alerts-box` of the same template. Plain punctuation, no dashes.

---

### `web/static/js/person.js` and `settings-alerts.js`/`settings.js` additions (component)

**Analog:** `web/static/js/settings-alerts.js` (window wiring at lines ~43 to 81: `mutedWin`, `mutedRow`, `mutedValue`, `mutedLoading`, `mutedList` looked up by id; `saveKind` at 736 to 743 sends `{field: value === true}` through `saveAlerts`) and `web/static/js/mute.js` for the mute and unmute fetch calls.

Rules (contract tests): `textContent` only, no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write` (`TestNoMarkupSinksAnywhere`); no thresholds in trust JS (`TestTrustModuleHasNoThresholdsOrSinks`); new scripts join `phaseScripts` (plain punctuation) and `noiseCheckedFiles`; new controls join `touchControls` (44px via `--touch-target-min`); star buttons use `aria-pressed`, no dark border on the chosen item (`TestNoDarkBorderOnChosenItems`); no pill shapes; no underlined text links (actions are real buttons); phone and laptop carry the same features (phone only elements named in `sizeOnlyNamed`).

---

### `web/static/icons/star.svg` and `bell` reuse (asset)

**Analog:** `web/static/icons/bell.svg` / `bell-off.svg` (reuse for the per person bell, D-08). New `star.svg` is drawn in the repo, no icon package, no image files; register in `laterIcons` (`TestNewIconsAreRegisteredAndUsed` also compares the glyph with the reference). CSS mask icon pattern `app-icon app-icon--<name>`.

---

### `web/templates/account_header.html.tmpl` (modify)

**Analog:** itself (lines 8 to 18). Add a first menu item for My profile linking the constant `/me`:
```html
    <a role="menuitem" class="account-menu__item" href="/settings">
      <span class="auth-icon auth-icon--settings" aria-hidden="true"></span>
      Settings
    </a>
```
This template is included by many page structs; linking to `/me` avoids a username field on every model (research Pitfall 12). Amend `TestPersonMenuItems` (web/places_contract_test.go:17 to 27, expects exactly 2 `role="menuitem"` and bans "My profile") and `TestSettingsBackAndNoGear` (menu text "Settings Log out") with a dated owner note. `ways_in_contract_test.go:215` pins `account-menu__item`.

---

### `internal/api/handlers/ratelimit_gate.go`, `router.go`, `cmd/server/main.go` (config)

**Analog:** `RateLimits` struct (ratelimit_gate.go line 35 on): per account `BucketLimit` fields like `Flag`. A new bucket must be added in four places or config silently falls back: the struct and `NewRateGate`; the constants block in `cmd/server/main.go` and its `Deps.RateLimits` wiring; `api.RateLimitsDefault`; `resolveRateLimits` in `router.go`. Also add the field to the e2e `p20Open` helpers (frozen clock tests) and check `internal/api/ip_backstop_test.go`. Buckets are keyed by account, never by IP (CGNAT). Phase 8 adds buckets in the same four places; re-read.

Service construction in `router.go` (line ~459): `mutes := service.NewMuteService(deps.Sessions)`; copy for `people` and `ratings` services. Routes registered in the gated `r.Group` (router.go lines 527 to 670), e.g. `r.Post("/api/reports/{id}/mute", handlers.MuteReport(mutes))`, `r.Get("/settings", handlers.SettingsPage(deps.Template, accounts, mutes, page))`, `r.Patch("/api/me/alerts", handlers.UpdateAlertSettings(accounts))`. Do not touch the gate exempt list.

---

## Shared Patterns

### One authority per rule
**Source:** `internal/service/updates.go` `MayNotifyAboutReport` (D-29), `reliability.go` `reporterTagForScore`, `historyread.go` `classify`/`buildFinishedView`.
**Apply to:** follow note step, push dispatcher, push worker, profile reports, the sweep. A second place that compares a score to 70, or a second visibility or eligibility copy, is a bug.

### Account id from the gate, never from the request
**Source:** `handlers/settings.go` lines 64 to 70 and `handlers/me.go` 234 to 240.
```go
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
```
**Apply to:** every new handler.

### Error mapping
**Source:** `handlers/me.go` 257 to 266: `errors.As(err, &ve)` for `service.ValidationError` to `writeFieldError(..., ve.Field, ve.Message)`; anything else is `log.Printf("handlers: <Name>: %v", err)` plus a 500. Not found sentinels (`ErrReportNotFound` pattern) map to a 404 that does not reveal why.

### Strict JSON bodies, no CSRF token
**Source:** `me.go` 242 to 249 (`MaxBytesReader`, `DisallowUnknownFields`); SameSite Lax cookie. Empty body actions use `decodeEmptyBody`.

### Responses carry `Cache-Control: no-store` for account data
**Source:** `me.go` 267, `settings.go` 119.

### Narrow querier + compile time assertion
**Source:** `mutes.go` lines 22 to 33. **Apply to:** every new service.

### Closed lists are repeated in many places
Widen every one together: `push_outbox_kind_valid`, `uq_push_outbox_first` and its `ON CONFLICT` predicates (`EnqueueInitialPush`, `EnqueueReliableFromUpdates`), `EnqueueConfirmedUpdates`, the `first_kind` LEFT JOIN in `ClaimDuePushes`, `EnqueueChangedFromUpdates` NOT EXISTS list, `CountAccountPushesInWindow`, `updates_kind_valid`, `CountReporterFanouts` / `push_report_progress` if a per reporter cap is kept. Source: push.sql, updates.sql, 00011.

### Privacy
Same 404 for unknown, private, deleted, blocked. No `@`, account, session, cell, voter or email in feed JSON. Never expose `home_area_cell`. Lock screen has no username. Finished profile rows are not links. Name and username are user text: `html/template` escaping and `textContent`.

### Generated code
`make sqlc` and `make swag` after query or annotation changes; commit the generated files. Run with one binary version.

## Tests that move (amend with a dated owner note, do not delete checks that still hold)

| Test or pin | Change |
|-------------|--------|
| `internal/service/reliability_test.go:259, 448-450` | score 71 flips Mixed to Reliable; `expectedTag` reads the constants |
| `internal/service/reporter_test.go:212, 246` | account 102 at 71 becomes Reliable |
| `internal/service/trustfacts_test.go:255`, `trustwording_test.go:435` | "Mixed record" to "Reliable reporter" (Good tone) |
| `internal/service/trust_model_doc_test.go` (lines 59 to 78, 224 to 243, 71-72, 232-233) | constants table 75 to 70, new constants added |
| `docs/TRUST-MODEL.md` lines 303, 307, 329, 330, 341, 358 to 361, 374, 529, 555, 577-578, 612, 619 | numbers, anchor rows (70 exact: 6C2X; 69: 4C+2U), "Display only" wording, hand off line |
| Do not touch | `feed_test.go:1443`, `profile_record_test.go:36`, `trustfacts_test.go:127`, `trustwording_test.go:337` ("Score 72" is the Current score scale) |
| `internal/service/pushkinds_test.go` (`TestPushKindsClosed`, `TestPushCapsAreNamed`) | follow valid, reserved list changes |
| `internal/service/accountdelete_test.go` | interface and order pins |
| `web/places_contract_test.go` (`TestPersonMenuItems`, `waitingSettingsLabels`, chevron row count), `places_e2e_test.go` (bans on "View my profile", "My profile", "Let others follow me", "Following", "Followers", "Blocked people" at ~185 and 256), `settings_rows_contract_test.go` | amend; keep `TestNotificationsNoFollowOrOfficialRow` true |
| `TestSwaggerSpecCoversRoutes`, anonymous 401 table | new routes added |
| `TestFeedSignalsMatchCastVoteSignals`, `TestFeedTrustMatchesCastVoteTrust` | extend with a seeded rated reporter |
| Full suite on a scratch database right after the 70 change | record every flipped DB seeded fixture (`reliable_filter_e2e_test.go`, `reliable_alert_e2e_test.go`, `pushdispatch_db_test.go`, `testutil/history.go`) in the plan SUMMARY |

Database rule for every executor: never point `DATABASE_URL` at `pinalert_test` or `pinalert`; use a uniquely named scratch database; never use ports 8080 or 8090.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/service/followsweep.go` (the follow ended-by-tag-loss sweep logic itself) | service | batch | A ticker exists (`pushdispatch.go` `Run`), but nothing deletes graph rows because a computed value changed; use RESEARCH 5.3 |
| Letter avatar (round tinted circle, first letter of username) | component | n/a | No avatar exists; no images rule; draw in CSS (research G5) |
| Moderator "remove this person's ratings" action | controller/service | CRUD | Moderator group only has report Keep and Remove; model the routes on `ModerationKeep`/`ModerationRemove` and add a `removed_at`/`removed_by` stamp (gap G2, G3) |

## Metadata

**Analog search scope:** `internal/service`, `internal/api` (router and handlers), `internal/store/queries`, `internal/store/migrations`, `web/templates`, `web/static/js`, `web/static/icons`; phase docs 05, 07.1, 08 plans and research for overlap.
**Files scanned:** about 45 (read in full or by targeted range).
**Pattern extraction date:** 2026-10-25

## PATTERN MAPPING COMPLETE
