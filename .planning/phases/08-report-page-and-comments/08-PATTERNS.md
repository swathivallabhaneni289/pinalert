# Phase 8: Report page and comments - Pattern Map

**Mapped:** 2026-10-25
**Files analyzed:** 52 new or modified (grouped below by plan wave of RESEARCH section 10)
**Analogs found:** 46 / 52 (6 have no real analog, listed at the end)

Scope note. Phases 5 and 7.1 build before this one and are not built. Phase 3.3 is fully merged on main (02e877e). Every excerpt below was read from main as of today. Anything in the section "Re-read after Phase 5, 7.1 land" must be re-grepped by the planner before a plan quotes a line number.

## File Classification

| New or modified file | Role | Data flow | Closest analog | Match |
|---|---|---|---|---|
| `internal/service/timeline.go` (BuildTimeline, VoteLogRow) | service (pure) | transform | `internal/service/history.go` ClosureOf | exact |
| `internal/service/timelinecopy.go` | utility (closed copy) | transform | `internal/service/trustcopy.go` + `trustwording_test.go` | exact |
| `internal/store/queries/votes.sql` (VoteLogForReport) | store | CRUD read | `votes.sql` CurrentVotesForReports (lines 9 to 22) | exact |
| `internal/api/handlers/timeline.go` | controller | request-response | `internal/api/handlers/trust.go` + `flag.go` | role-match |
| `internal/service/share.go` (ShareService) | service | CRUD + outbound lookup | `service/pushdispatch.go` ensureAreaName (644 to 677) + `flag.go` | role-match |
| `internal/store/migrations/NNNNN_share_token_and_comments.sql` ("next free number") | migration | CRUD | `00010_add_flag_reasons_and_note.sql`, `00011_add_push.sql` | role-match |
| `internal/store/queries/share.sql`, `comments.sql` + regenerated `internal/store/sqlc/*` | store | CRUD | `queries/updates.sql`, `queries/moderation.sql` | exact |
| `internal/api/handlers/share.go` (POST share) | controller | request-response | `handlers/flag.go` | exact |
| `internal/api/handlers/public_status.go` + `web/templates/share_status.html.tmpl` | controller + template | request-response (no session) | `handlers/legal.go` + `reader.html.tmpl` | role-match |
| `internal/api/router.go` (routes, `/r` session skip, Deps.PublicBaseURL, rate limits) | route / config | request-response | router.go 412 to 415, 469 to 523, 582 to 587, 710 to 717, 756 to 785 | exact |
| `internal/api/gate_test.go` (exempt row + near misses) | test | request-response | `TestAccessGateExemptRoutes` (319 to 411) | exact |
| `internal/api/share_routes_test.go` (no cookie, no sessions row) | test | request-response | `pwa_routes_test.go` TestPWAAssetsSetNoCookie (135 to 199) | exact |
| `internal/service/comments.go` (CommentService) | service | CRUD | `service/flag.go` | role-match |
| `internal/service/commentscopy.go`, `ratecopy.go` additions | utility | transform | `service/ratecopy.go`, `moderationcopy.go` | exact |
| `internal/service/commentworker.go` | service | batch / background | `service/modworker.go` | exact |
| `internal/service/commentmoderation.go` | service | CRUD | `service/moderation.go` | role-match |
| `internal/api/handlers/comments.go` | controller | CRUD | `handlers/flag.go`, `votes.go` | role-match |
| `internal/api/handlers/ratelimit_gate.go` (Comment, CommentAction buckets) | middleware | request-response | itself, FlagMiddleware (322 to 350) | exact |
| `cmd/server/main.go` (limit constants, PublicBaseURL, worker start) | config | n/a | existing D-11 constants block, `BASE_URL` read at ~189 | exact |
| `internal/api/handlers/moderation.go` (+ comments endpoints) | controller | CRUD | itself, ModerationQueue / modAct (196 to 284) | exact |
| `internal/service/accountdelete.go` (comments, likes, flags hooks) | service | CRUD (tx) | itself (AccountDeleteQuerier, 29 to 60) | exact |
| `internal/service/updates.go`, `updatescopy.go`, `updatesalerts.go` (two new kinds) | service | event-driven | `UpdateReliableNearby` + `InsertReliableAlert` | exact |
| `internal/store/queries/updates.sql`, `push.sql` (follow arms, kind list, insert) | store | event-driven | `FollowedOpenReports` (8 to 33), `FollowersWithChangesPush` (193 to 225), `EnqueueChangedFromUpdates` (383 to 399) | exact |
| `internal/service/pushcopy.go`, `pushworker.go` (changeEventWords, eventStillHolds) | service | event-driven | same files, existing change kinds | exact |
| `internal/testutil/db.go` Truncate + comment seed helper | utility | CRUD | `testutil/db.go` line 80, `testutil/history.go` | exact |
| `web/static/js/report-page.js` (mount agnostic builder) | component | request-response | `map.js` buildPopupCard / updatePopupCard (780 to 905, 911+) | role-match |
| `web/static/js/comments.js` | component | CRUD | `flag.js`, `mute.js`, `updates.js` | role-match |
| `web/static/js/share.js` + Share window markup in `index.html.tmpl` | component | request-response | `flag.js` + `flag_dialog.html.tmpl` | exact |
| `web/static/js/map.js` (arrangeCard order, footer, laptop host) | component | event-driven | itself (190 to 252) | exact |
| `web/static/js/flag.js` + `flag_dialog.html.tmpl` (comment mode) | component | request-response | itself | exact |
| `web/static/js/moderate.js` + `moderate.html.tmpl` (Comments section) | component | CRUD | itself, buildRow / renderList (544 to 680) | exact |
| `web/static/css/report-page.css`, `comments.css`, `share.css`, `phone.css` edits | css | n/a | `trust.css`, `phone.css`, `window.css` | role-match |
| Node tests `web/report_page_test.mjs`, `comments_test.mjs`, `share_window_test.mjs`, `popup_footer_test.mjs` | test | transform | `web/report_view_test.mjs`, `flag_dialog_test.mjs`, `mute_test.mjs` | exact |
| Go contract tests in `web/` (report page, comments, share; amend popup, layout, flag, template) | test | transform | `popup_contract_test.go`, `layout_rules_contract_test.go`, `flag_contract_test.go` | exact |
| Go e2e tests `timeline_e2e_test.go`, `share_e2e_test.go`, `comments_e2e_test.go`, `moderate_comments_e2e_test.go` | test | request-response | `handlers/flag_e2e_test.go` (p20 helpers) | exact |
| `docs/TRUST-MODEL.md` + `p20MustCite`, swag `docs/` | docs | n/a | `internal/service/trust_model_doc_test.go` | exact |

## Pattern Assignments

### `internal/service/timeline.go` (service, pure transform)

**Analog:** `internal/service/history.go` (245 lines). Pure, no ctx, no I/O, takes values and returns values.

**File header and imports** (lines 1 to 15):
```go
// history.go holds the pure functions every report history surface reads ...
// Everything here takes values and returns values. There is no context, no querier and no I/O ...
package service

import (
	"sort"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)
```

**Core pattern, prefix replay** (ClosureOf, lines 107 to 139): rows are filtered by kind, sorted stably by time then account, and the tally predicate Resolve itself uses is asked at each prefix.
```go
func ClosureOf(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) Closure {
	var content, resolution []sqlcgen.CurrentVotesForReportsRow
	for _, r := range rows {
		if r.ReportID != reportID { continue }
		switch VoteKind(r.Kind) {
		case VoteKindContent:    content = append(content, r)
		case VoteKindResolution: resolution = append(resolution, r)
		}
	}
	sort.SliceStable(resolution, func(i, j int) bool { ... })
```
Copy the shape: filter, build the standing row per (account, kind) after each log row (latest wins, same rule as the DISTINCT ON in votes.sql lines 18 to 22), call `BuildVoteTally` then `Resolve`, compare with the previous step. Do not re-count rows by hand (RESEARCH "Don't Hand-Roll").

**View words for the closed state:** `ViewOf(v, expiresAt, now)` (lines 78 to 93) returns exactly one of FeedViewActive, Disputed, Resolved, Past. Reuse it for the "Faded" and "Resolved" events and for `comments_open` (active or disputed only).

**Test pattern to copy:** `history_test.go` for table tests of ClosureOf; `trustwording_test.go` for the closed character set test of `timelinecopy.go`.

---

### `internal/service/timelinecopy.go` (utility, closed copy)

**Analog:** `internal/service/trustcopy.go` and its `trustwording_test.go` and `trustcopy_test.go`. Rules to carry (RESEARCH section 3): strings built only from constants and `strconv.Itoa`; closed character set (ASCII letters, digits, space, comma, full stop, apostrophe); the word "place" in no sentence (use "spot" and "people"); no relative time in a string (wire carries RFC 3339 `at`). Read the first 60 lines of `trustcopy.go` when planning; this mapper did not need its internals.

---

### `internal/store/queries/votes.sql` : VoteLogForReport (store, read)

**Analog:** `votes.sql` CurrentVotesForReports, lines 9 to 22:
```sql
-- name: CurrentVotesForReports :many
SELECT DISTINCT ON (report_id, account_id, kind)
    report_id, account_id, kind, value, geohash_cell, created_at
FROM votes
WHERE report_id = ANY(sqlc.arg(report_ids)::bigint[])
ORDER BY report_id, account_id, kind, created_at DESC, id DESC;
```
New query: same table, `id` added, `WHERE report_id = sqlc.arg(report_id)`, `ORDER BY id`, `LIMIT` a cap. Same header-comment style (explain why, name the privacy rule). The columns are service internal; the handler never serialises them.

---

### `internal/api/handlers/timeline.go` and `comments.go` (controller, request-response)

**Analog:** `internal/api/handlers/flag.go` (145 lines). Copy this shape exactly for every new gated JSON route.

**Imports** (lines 9 to 17):
```go
import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)
```

**Handler skeleton** (lines 73 to 145, abridged): account from context (500 if absent, a wiring bug), `parseReportID`, body cap with `http.MaxBytesReader`, `DisallowUnknownFields`, service call, then an `errors.Is` ladder mapping sentinels to `writeFieldError(w, status, field, message)`, `errors.As` for typed errors, a final `log.Printf` plus plain 500, and `writeJSON(w, http.StatusCreated, ...)`.
```go
acc, ok := account.FromContext(r.Context())
if !ok { http.Error(w, "internal server error", http.StatusInternalServerError); return }
reportID, ok := parseReportID(w, r)
r.Body = http.MaxBytesReader(w, r.Body, maxFlagBodyBytes)
dec := json.NewDecoder(r.Body); dec.DisallowUnknownFields()
...
var nr service.FlagNoteRejectedError
if errors.As(err, &nr) {
	writeFieldError(w, http.StatusUnprocessableEntity, "note", service.RejectMessage(nr.Reason))
	return
}
```
Apply: comment POST maps `TextRejected` to 422 with `RejectMessage`; closed comments to 409 with a stable field; 404 for withheld; 429 comes from the middleware, not the handler. Never put an account id, email, session or cell on the wire (`TestTrustResponseOmitsIdentifiers` style test per response).

**Swag block pattern** (flag.go lines 45 to 72): `@Summary`, `@Description` lines, `@Tags`, `@Param id path int true "Report id"`, `@Failure 401 ... 429 {object} RateLimitResponse`, `@Header 429 {integer} Retry-After`, `@Router /reports/{id}/flag [post]`. `TestSwaggerSpecCoversRoutes` (handlers/swagger_test.go) pins these, extend it.

**Timeline read specifics:** also see `handlers/trust.go` (157 lines) for how a response type is kept free of identifiers; the withheld case answers the same 404 shape as the feed (RESEARCH section 3).

---

### `internal/service/comments.go` (service, CRUD with screening)

**Analog:** `internal/service/flag.go` (263 lines). Same bones: narrow querier interface declared in the consumer, option functions, typed error with Unwrap, an ordered check list written into the doc comment.

**Narrow querier + options** (lines 115 to 149):
```go
type FlagQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	InsertReportFlag(ctx context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error)
}
type FlagService struct { q FlagQuerier; screener TextScreener }
type FlagOption func(*FlagService)
func WithFlagScreener(s TextScreener) FlagOption { ... }
func NewFlagService(q FlagQuerier, opts ...FlagOption) *FlagService { ... }
```

**Typed rejection error** (lines 100 to 113):
```go
type FlagNoteRejectedError struct{ Reason RejectReason }
func (e FlagNoteRejectedError) Error() string { return fmt.Sprintf("service: flag note rejected (%s)", e.Reason) }
func (e FlagNoteRejectedError) Unwrap() error { return ErrFlagNoteRejected }
```

**Withheld before ownership, one read, screener after the cheap checks** (lines 214 to 244): read the report context once, `meta.Withheld()` returns `ErrReportNotFound`, then ownership, then `ScreenAbuse`. Comments copy this order: validate text (cleanCommentBody), load report, withheld, closed (409), then screen, then insert. Flag stays allowed on a closed report (safety), post and like do not.

**Screening call to use (not CheckText):** `internal/service/textcheck.go` lines 84 to 118, `ScreenAbuse(ctx, s, text)`. `CheckText` (68 to 74) runs `IsGibberish` first and would refuse "Thanks" (screening.go 204 to 231: fewer than two words of two letters is gibberish). Outcomes map: `TextRejected` returns 422 and stores nothing; `TextBorderline` stores clear with `needs_review`; `TextClear` stores clear; `TextUnfinished` stores `pending` with `next_attempt_at = now()`.

**Validation excerpt to follow for the 280 rune rule:** `ValidateFlagInput` lines 166 to 188 uses `utf8.RuneCountInString(note)` after `strings.TrimSpace`. Comments add the control and `unicode.Cf` rejection from RESEARCH "Text rules".

**Screener source in the router** (router.go 440 to 444): `flags := service.NewFlagService(deps.Sessions, service.WithFlagScreener(deps.Reports.Screener()))`, guarded by `deps.Reports != nil`. Copy for the comment service.

---

### `internal/service/commentworker.go` (service, background batch)

**Analog:** `internal/service/modworker.go` (318 lines). Sibling, do not generalise `moderation_checks` (RESEARCH section 5).

**Constants to reuse by value** (lines 16 to 41): sweep 30 s, lease 60, batch 5, backoff base 30 s cap 1 h, max shift 7, call timeout 8 s, error words `CheckErrorTransient`, `CheckErrorQuota`, `CheckErrorPermanent`. `ModerationBackoff(attempts, retryAfter, factor)` (165 to 181) can be called directly; do not copy it.

**Structure to copy** (lines 44 to 157): `WorkerQuerier` interface with `var _ WorkerQuerier = (*sqlcgen.Queries)(nil)`, `WorkerOption` clock and jitter hooks, `Run` (184 to 199: sweep once at start, then ticker), `sweepAndLog`, `SweepOnce` claim then loop then `errors.Join`, `screenRow` (screener call with `context.WithTimeout`, shutdown writes nothing, `Decide`, finish in one statement), `reschedule` (classify via `errors.As(callErr, &ce)` on `*moderation.CallError`; quota or permanent moves next attempt to a time the claim never reaches).

**Claim SQL to copy** (`moderation.sql` lines 32 to 63): MATERIALIZED CTE with `FOR UPDATE ... SKIP LOCKED`, lease, `attempts = attempts + 1`, joins to `reports` so an expired or removed report is never claimed. For comments: claim from `comments` where `moderation_state = 'pending'`, join `reports` for `expires_at > now()`, return the comment body instead of the report description.

**Logging rule:** log lines carry only numeric ids, attempt, verdict word, status, wait (modworker header comment lines 100 to 101, T-04-04). Never comment text.

---

### `internal/service/commentmoderation.go` and the moderator endpoints

**Analog:** `internal/service/moderation.go` for Keep and Remove with audit row in one transaction; `internal/api/handlers/moderation.go` lines 194 to 284 for handlers.

**Handler shape** (lines 194 to 214):
```go
func ModerationQueue(svc *service.ModerationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := modGuard(w, r, http.MethodGet); !ok { return }
		q, err := svc.Queues(r.Context())
		if err != nil { log.Printf("handlers: ModerationQueue: %v", err); http.Error(w, "internal server error", 500); return }
		writeJSON(w, http.StatusOK, modQueueJSON{...})
	}
}
```
`modAct(logErr, act)` (221 to 224) is the shared POST body for keep and remove; reuse the pattern with a comment id.

**Route registration** (router.go 710 to 717): add `r.Handle("/api/moderation/comments", ...)` and `.../comments/{id}/keep|remove` inside the existing `requireModerator` group so a non moderator gets the byte equal 404.

**Client:** `moderate.js` builds rows with `Pinalert.setText` only (header comment line 9, `buildRow` 544 to 640, `renderList` 642). Add a Comments section built the same way; comment text through `setText`, never a sink.

---

### `internal/service/share.go` and `handlers/share.go` (POST /api/reports/{id}/share)

**Handler analog:** `handlers/flag.go` (above). Gated, limiter via `r.With(...)`, JSON out.

**Area lookup analog:** `internal/service/pushdispatch.go` lines 644 to 677, ensureAreaName. Factor this into a shared function; the order matters:
```go
// attempt recorded first, so a failing lookup is not retried on every call
if err := d.q.MarkAreaLookupTried(ctx, sqlcgen.MarkAreaLookupTriedParams{ReportID: ..., Now: ...}); err != nil { ... return }
callCtx, cancel := context.WithTimeout(ctx, PushAreaLookupTimeout)
name, err := d.area.ReverseArea(callCtx, lat, lon)
cancel()
...
d.q.SetReportAreaName(ctx, sqlcgen.SetReportAreaNameParams{AreaName: name, ReportID: ...})
```
The interface it relies on is at pushdispatch.go lines 110 and 121 (`ReverseArea`, `MarkAreaLookupTried`). Never call the geocoder from the public route.

**Token:** `crypto/rand`, 16 bytes, unpadded base64url; `UPDATE reports SET share_token = $1 WHERE id = $2 AND share_token IS NULL RETURNING share_token` (RESEARCH section 4). No existing token generator to copy for the column, but `internal/store/queries/magic_link_tokens.sql` and `service/auth.go` show how this repo hashes and issues single use tokens; the share token is stored plain on purpose (it is a capability, not a credential) and that choice should be written in the migration comment.

---

### `internal/api/handlers/public_status.go` + `web/templates/share_status.html.tmpl` (public, no session)

**Analog:** `internal/api/handlers/legal.go` (131 lines) for a public, server rendered, template-executing handler.

**Pattern to copy** (lines 103 to 131):
```go
func Legal(tmpl *template.Template, cfg AuthConfig, page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chrome := cfg.Chrome.withDefaults()
		vm := legalViewModel{AssetVersion: cfg.AssetVersion, Chrome: chrome, ...}   // no email field on purpose (R-07)
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, vm); err != nil { log.Printf(...); http.Error(w, "internal server error", 500); return }
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", legalCacheControl)
		_, _ = w.Write(buf.Bytes())
	}
}
```
Differences the planner must apply: `Cache-Control: no-store` (not legalCacheControl, line 27, which is `private, max-age=300` precisely because the session middleware sets a cookie; this page skips the session so it must not set one), plus `X-Robots-Tag: noindex, nofollow`. The view model carries only allowed fields (category label, severity word, what is happening pick, coarse area, status words, `ConfirmedText`, time); a `TestShare...` test asserts no description, no coordinates, no names. Templates are parsed by `ParsePageTemplate` (`template.ParseFS(web.TemplatesFS, "templates/*.tmpl")`, page.go 164 to 166), so a new `.tmpl` file is picked up automatically and is scanned by `TestUserVisibleCopyUsesPlainPunctuation`. `og:url` and the share URL come from `Deps.PublicBaseURL`, never `r.Host`.

---

### `internal/api/router.go` (routes, session skip, limits)

**Public route placement:** exempt block, lines 469 to 523. The legal pages are the model (lines 479 to 481):
```go
r.Get("/disclaimer", handlers.Legal(deps.Template, auth, handlers.LegalDisclaimerPage))
r.Get("/policy", handlers.Legal(deps.Template, auth, handlers.LegalPolicyPage))
```
Add `GET /r/{token}` here with a dated comment in the NewRouter doc block (the 3.3 amendment is the style to follow).

**Per route IP limiter, scoped with r.With** (lines 518 to 523):
```go
requestLinkLimiter := ratelimit.NewPerIP(requestLinkLimit.Every, requestLinkLimit.Burst)
r.With(requestLinkLimiter.Middleware()).Post("/api/auth/request-link", handlers.RequestLink(deps.AuthService))
```
Copy for `/r/{token}` with loose numbers (burst 120, one per second) and the CGNAT comment (lines 511 to 517 explain why never router wide).

**Session bypass, the part that needs a prefix** (lines 412 to 415 and 756 to 785):
```go
r.Use(skipSessionFor(pwaSkipSet(), deps.Session.Middleware(deps.persistSession)))
...
if r.Method == http.MethodGet && r.URL.RawPath == "" && paths[r.URL.Path] { h.ServeHTTP(w, r); return }
```
Exact path only today, so `/r/{token}` does nothing until the rule gains a prefix list. Phase 5 plan 05-04 rewrites this function (see "Re-read after"). Fallback if it has not landed: a small prefix list in the same style, same near miss tests.

**Gated routes with the flag style limiter** (lines 582 to 587): `r.With(rateGate.FlagMiddleware()).Post("/api/reports/{id}/flag", handlers.FlagReport(flags))`. New gated routes go flat inside the `r.Group` that starts at line 527 (`requireVerifiedAccount`, `requireOnboarded`), as flat literal paths, never under an `r.Route` mount (comment lines 503 to 509).

**Rate limit wiring, the four places:** `RateLimits` struct and `NewRateGate` (ratelimit_gate.go 35 to 58 and 123 to 140), `RateLimitsDefault` (router.go 272 to 288), `resolveRateLimits` fill list (router.go 298 to 306), and the constants block in `cmd/server/main.go`. Plus the `p20Open` tweak helpers in e2e tests.

---

### `internal/api/handlers/ratelimit_gate.go` : Comment and CommentAction buckets

**Analog:** itself, FlagMiddleware lines 322 to 350. Copy whole:
```go
func (g *RateGate) FlagMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acc, ok := account.FromContext(r.Context())
			if !ok { http.Error(w, "internal server error", 500); return }
			if d := g.reserveIPBackstop(r, g.ipBackstopFlag); !d.OK {
				logBlock(ipBackstopFlagBucket, acc.ID, d.RetryAfter)
				writeRateLimited(w, flagRateField, service.NetworkLimitMessage(false, d.RetryAfter), d.RetryAfter, false, "")
				return
			}
			if d := g.flag.Reserve(accountKey(acc.ID)); !d.OK {
				logBlock(flagBucketName, acc.ID, d.RetryAfter)
				writeRateLimited(w, flagRateField, service.FlagLimitMessage(d.RetryAfter), d.RetryAfter, false, "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```
New constants follow lines 84 to 86 (`flagBucketName = "flag"`, `flagRateField = "rate_flag"`): add `commentBucketName`, `commentRateField = "rate_comment"`, `rate_like`. Messages go in `service/ratecopy.go` using the existing `WaitPhrase` (see `FlagLimitMessage`). No limit number typed in this file (header comment lines 7 to 9). A comment attempt spends a token and is never refunded, because each attempt costs a screener call (compare `AdmitReport` lines 209 to 241, "a rejected post still spends an attempt").

---

### Migration (comments, likes, flags, share_token, updates kinds)

**Analogs:** `00010_add_flag_reasons_and_note.sql` for the Up/Down style, an immutable SQL helper function for the distinct reasons CHECK (lines 17 to 24), and `CHECK` constraints named per rule (lines 33 to 43). Reuse `report_flag_reasons_distinct` for `comment_flags.reasons` rather than creating a second function. `00009_add_updates_and_alerts.sql` holds `updates_kind_valid` and `updates_shape`; widening a CHECK means DROP CONSTRAINT then ADD CONSTRAINT with the longer list, with a Down that restores it. `00011_add_push.sql` is the model for the area name column and a partial unique index. Every plan says "next free number at execution time"; never hardcode.

---

### `internal/service/updates.go` and friends (two new kinds)

**Analog:** the existing `UpdateReliableNearby` end to end.

**Constants and closed lists** (updates.go lines 20 to 70, abridged):
```go
const UpdateCriticalNearby UpdateKind = "critical_nearby" // push only, never stored
var UpdateEventKinds = []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby}
var UpdateKinds = []UpdateKind{..., UpdateMore}
func (k UpdateKind) Valid() bool { for _, x := range UpdateKinds { if x == k { return true } }; return false }
```
Add `UpdateCommentOnYours` and `UpdateReplyToYours` to both lists; fix the doc counts ("five", "six") in the comments; tests that pin the closed sets must change with them.

**Write time insert analog:** `InsertReliableAlert` (updates.sql 154 to 190): `INSERT ... SELECT ... WHERE (ceiling check) ON CONFLICT (account_id, report_id) WHERE kind = 'reliable_nearby' DO NOTHING RETURNING ...`. For comments use a partial unique index on `(account_id, report_id, kind) WHERE read_at IS NULL` so 30 comments become one bell row, and skip the hourly ceiling (RESEARCH section 6). Rows carry no comment text and no username.

**Follow arms:** add a third `UNION SELECT c.report_id FROM comments c WHERE c.account_id = ...` to `FollowedOpenReports` (updates.sql 15 to 25) and a matching arm to `FollowersWithChangesPush` (push.sql 198 to 208). Without both, PAGE-02's "commenting makes the user follow" is false.

**Phone alert kind list:** `EnqueueChangedFromUpdates` hard codes `u.kind IN ('reached_live', 'hidden', 'resolved', 'fading')` (push.sql line 397). Add the two kinds, then `changeEventWords` (pushcopy.go) and `PushWorker.eventStillHolds` (pushworker.go). Push words carry no comment text and no username (`PushTitleMaxRunes` 64).

---

### `internal/service/accountdelete.go` (comment hooks)

**Analog:** itself, `AccountDeleteQuerier` lines 29 to 50, with `var _ AccountDeleteQuerier = (*sqlcgen.Queries)(nil)` at line 52 and the transaction wrapper `AccountTxFunc` / `PgxAccountTx` (lines 56 to 70). Add methods in the same one-line style (`DeleteAccountCommentLikes`, `DeleteAccountCommentFlags`, `BlankAccountComments`), call them inside the existing `fn`, and add a test next to `accountdelete_test.go`. Add the new tables to the explicit list in `testutil.Truncate` (`internal/testutil/db.go` line 80, a single TRUNCATE statement listing tables).

---

### `web/static/js/report-page.js` (component, mount agnostic builder)

**Analog:** `web/static/js/map.js` buildPopupCard (780 to 905) and updatePopupCard (911+).

**Build once, update in place.** Nodes are made once with `document.createElement`, text via `Pinalert.setText`, held in a `card` object (lines 882 to 902), and `updatePopupCard(card, report)` only writes text, classes and attributes (comment lines 907 to 910). The new page object hangs off the card as `card.page`, with its own `update(report)` that never replaces the composer or an open thread. Header style to copy:
```js
var el = document.createElement('div');
el.className = 'popup-card';
el.setAttribute('tabindex', '-1');
...
var muteControl = window.PinalertMute ? PinalertMute.createMuteControl(report) : null;   // failed script load leaves the card without it
```
Use the same guard (`window.PinalertX ?`) so a failed `report-page.js` load cannot stop the map after its first pin (WR-W04 comment, lines 866 to 868).

**Order warning, the one real trap** (arrangeCard, lines 217 to 252). On every placement:
```js
var lastParent = phone ? body : actions;
if (phone) { body.insertBefore(fade, trust); body.insertBefore(actions, trust); } else { body.insertBefore(trust, fade); }
if (mute) { lastParent.appendChild(mute.el); if (mute.error) lastParent.appendChild(mute.error); }
if (flag) { lastParent.appendChild(flag); }
```
Appending the page node to `body` is overtaken by the next `appendChild(mute.el)` and `appendChild(flag)`. Change those two appends to `body.insertBefore(..., card.page.el)` on the phone, and test the final child order across phone, laptop, phone.

**Test harness analog:** `web/report_view_test.mjs` cuts map.js between the markers `// ---- The report view and its address (plan 39)` and `// ---- End of the report view (plan 39)` (lines 22 to 30). Do not move or rename those markers. For `report-page.js` use `between(source, from, to)` plus `vm` the same way, with a fake document.

---

### The laptop host: both candidates (Open Question 1, owner viewing pending)

**Candidate A, keep the popup as built (today's laptop):**
- `map.js` `applyPopupPlacement` (lines 194 to 208) sets `popup.options.pane = phone ? 'detailPane' : 'popupPane'` and `autoPan = !reducedMotion && !phone`; the popup is `minWidth: 340, maxWidth: 340`. A wider popup with the same sections needs the composer to stop `keydown` propagation (Leaflet's keyboard handler is document level while the map has focus; `onMapKeydown` in map.js closes the popup on Escape).
- Contract tests that pin it: `web/layout_rules_contract_test.go` lines 507 to 508 (size-only entries quote the owner's sentence "the laptop ver looks good i dont want you to disturb that"); `web/popup_contract_test.go` lines 493 to 496 (the footer ban); `web/report_view_contract_test.go`, `view_strip_contract_test.go`, `sheet_contract_test.go`, `phone_layout_contract_test.go` (geometry of the detail pane).
- The popup footer (SC4, D-03) exists only here. The skill reference `.claude/skills/sketch-findings-pinalert/references/popup-structure.md` line 22 holds the visual.

**Candidate B, list column host beside the map (sketch 008 B):**
- Reuse the phone's detail pane: `phoneDetail()` (lines 188 to 192, `mediaMatches(NARROW_QUERY)`) is used for four things; split it so placement uses a new `detailInPane()` and the strip centring and Back label stay phone only. `phone.css` already lays `.leaflet-detail-pane .leaflet-popup` out as a fixed sheet and hides `.pane--list` when `html[data-popup-open]` is set (RESEARCH section 1, phone.css line ~122). The laptop version is a CSS only geometry change over `.pane--list`. The detail host `div.report-view-host` is a child of `document.body`, outside the map container (map.js 351 to 353 creates the pane), which is what keeps typing safe.
- `Pinalert.openReport(id)` (app.js `openReport`) and `noteReportOpened` / `settleReportClosed` / `onReportPopState` stay untouched; the address and history are done (RESEARCH section 1).
- Amend with dated notes, never silently: the layout_rules size-only entries (507 to 508), `TestSizeOnlyElementsAreNamed`, and the four geometry contract tests.

Both candidates share one rule: the page sections are one mount agnostic builder (`report-page.js`), so the host choice is a placement plan (08-05) and not a rewrite.

---

### `web/static/js/flag.js` + `flag_dialog.html.tmpl` (comment mode)

**Analog:** itself. The control is one text button per report with `el.dataset.flagReportId` (flag.js ~line 50) and the dialog is static markup (`flag_dialog.html.tmpl` lines 1 to 31, ids `flag-dialog`, `flag-dialog-title`, `flag-dialog-dispute`, `flag-dialog-reasons`, `flag-dialog-send`). The request is built near flag.js line 327: `fetch('/api/reports/' + encodeURIComponent(current.reportId) + '/flag', ...)` raced against `FLAG_TIMEOUT_MS`. Add a target kind to `current`; for a comment set the title "Why are you flagging this comment?", hide `#flag-dialog-dispute` ("If the report is just wrong, use Dispute instead."), post to the comment flag path. Strings reach the DOM through `Pinalert.setText` only. Update `flag_dialog_test.mjs` and `flag_contract_test.go`. Owner rules: buttons not underlined links; boxes behave like a system alert (reuse `.logout-confirm` conventions, no button outlined on open).

---

### `web/static/js/comments.js`, `share.js` (components)

No single file is an analog for a list with a composer. Use these for pieces:
- Text into the DOM: `Pinalert.setText` and `createElement` only; `TestNoMarkupSinksAnywhere` (js_sinks_contract_test.go) fails on `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`. The leading `@username` is a `<strong>` made with `createElement` (RESEARCH "Rendering a leading mention").
- Fetch with timeout and error messages kept in constants at the top of the file: `flag.js` lines 1 to 40 (labels, `FLAG_TIMEOUT_MS`, the offline, gone and generic error strings).
- Module shape: `window.PinalertFlag = (function () { 'use strict'; ... return {...}; })();` (flag.js line 13). Same for `window.PinalertReportPage`, `PinalertComments`, `PinalertShare`.
- Relative times: `Pinalert.relativeTime(iso)`; the server never sends "5 minutes ago" to the app page.
- Counter: `Array.from(text).length` so it matches the server rune count (flag note box has the same note, NOTE_MAX 300, flag.js lines 25 to 30).
- Share window: static markup beside `flag_dialog.html.tmpl`, `.modal-backdrop` and `.modal-panel` classes, `[hidden]` guard (`:not([hidden])` on any display rule). Open the window on tap and fetch the link while it is open; call `navigator.share` synchronously from the second tap inside the window.
- Script load order: `template_contract_test.go` `TestVendorMapScriptsLoadInDependencyOrder`; new scripts load after `trust.js` and `votes.js` and before `map.js` if map.js calls them at build time.

---

### Node and Go test patterns

- Node: `web/report_view_test.mjs` (source slicing with `between()` and `vm`), `flag_dialog_test.mjs`, `mute_test.mjs`. Header comment states the run command `node --test web/<file>.mjs`.
- Go e2e: `internal/api/handlers/flag_e2e_test.go` lines 1 to 80. Package `handlers_test`, identifiers prefixed per plan (`p20...`; use an `r08` prefix), a frozen `p20Clock`, `p20Open = api.BucketLimit{Every: time.Millisecond, Burst: 1000}`, and `p20Tweak(clk, mutate)` which opens every bucket first and then lets the test set the one it exercises. Add the two new buckets to that opener or frozen clock tests meet a zero bucket. Database tests run only on a disposable scratch database named for the plan; never `pinalert_test`, never `pinalert`.
- Gate: `internal/api/gate_test.go` TestAccessGateExemptRoutes (lines 307 to 411). Add the share row to `cases` and to `mustBeOK` (line 334), then a near miss loop like lines 396 to 410 with `/r`, `/r/`, `/r/x/y`, `/R/<token>`, a numeric id, a bad token, each must not answer 200.
- No cookie proof: `pwa_routes_test.go` TestPWAAssetsSetNoCookie lines 135 to 199: count `SELECT count(*) FROM sessions` before and after, assert `len(resp.Header.Values("Set-Cookie")) == 0`, and a `misses` table whose rows must set a cookie and add exactly one row.

## Shared Patterns

### Gated JSON route
**Source:** `internal/api/handlers/flag.go` lines 73 to 145, mounted at router.go 587.
**Apply to:** timeline, share POST, comments post/reply/delete/like/flag, moderator comment routes.
Account from context, `parseReportID`, capped decoder, error ladder with `writeFieldError`, swag block with 401/404/409/429.

### Text screening
**Source:** `internal/service/textcheck.go` ScreenAbuse (84 to 118), `screening.go` Decide (117 to 141) and RejectMessage (153 to 166).
**Apply to:** comments (post, reply), username check in 7.1 (synchronous only, refuse save when unfinished).
One bounded call (`InlineScreenTimeout` 3 s), nil screener passes, error becomes `TextUnfinished`. Severity is never an input (`Decide(scores, false)`).

### Withheld means not found
**Source:** `flag.go` lines 222 to 229 and 202 to 204.
**Apply to:** timeline, comments list and post, share POST. A pending or removed report answers `ErrReportNotFound` before any ownership or closed check.

### Per account rate limiting
**Source:** `ratelimit_gate.go` FlagMiddleware (322 to 350) plus the four wiring places listed under router.go.
**Apply to:** comment post and reply, like, unlike, delete. Flag of a comment reuses the existing `Flag` bucket.

### No markup parsing sink, no identifiers on the wire
**Source:** `web/js_sinks_contract_test.go` TestNoMarkupSinksAnywhere; `internal/api/handlers/trust.go` and `TestTrustResponseOmitsIdentifiers`.
**Apply to:** all new JS; all new response types (no `account_id`, `email`, `session`, `geohash`, `cell` keys).

### Closed server copy
**Source:** `internal/service/trustcopy.go`, `moderationcopy.go`, `ratecopy.go`.
**Apply to:** timeline sentences, comment messages, rate limit messages, update and push words. Plain punctuation only; `TestUserVisibleCopyUsesPlainPunctuation` scans every template, `noise_words_contract_test.go` bans buzz, vibrate, sound, ring, silent, quiet, hear in the files it lists (Settings, welcome, notify, sw).

### Guard hidden elements
**Source:** `css_contract_test.go` TestModalBackdropHiddenGuard.
**Apply to:** share window, composer, any element toggled with `hidden`: a `display` rule needs `:not([hidden])`.

## Re-read after Phase 5 and 7.1 land

These files are also changed by phases that build first. The planner must flag every plan that touches them with "re-read after they land" and re-grep line numbers.

| File | Changed by | What to re-check |
|---|---|---|
| `internal/api/router.go` | Phase 5 (05-04 rewrites `skipSessionFor` into a `sessionSkip` rule set with prefix support, adds `/demo`, `/globe/tiles/...`, refresh route), Phase 7.1 (new sign in routes) | The `/r` prefix entry; the exempt block position; `pwaSkipSet` may be renamed |
| `internal/api/pwa_routes_test.go` | Phase 5 plan 05-04 | The `TestPWAAssetsSetNoCookie` helper shape the share no cookie test copies |
| `internal/api/gate_test.go` | Phase 5 (Demo, globe rows), Phase 7.1 (new screens) | `mustBeOK` map and the case table; add the share row last |
| `cmd/server/main.go` | Phase 5 (refresh wiring), 7.1, 3.3 (push wiring already landed) | Constants block, `BASE_URL` read (~line 189), worker start order |
| `web/templates/login_gate.html.tmpl` | Phase 5 (globe partial, links row), 7.1 (password card) | Only the "Sign in to see more" link target; do not edit this file in Phase 8 |
| `web/templates/welcome.html.tmpl` | Phase 5 step 1 pair, 7.1 step 4 required | Not edited by Phase 8; the report page assumes every account has a username |
| `web/templates/settings.html.tmpl` | Phase 3.3 plan 14 (landed), Phase 5 (row 3), 7.1 (Username row) | The "Changes to reports I posted or voted on" row text if widened (A9); `settings_rows_contract_test.go`, `noise_words_contract_test.go` |
| `internal/store/queries/accounts.sql`, `internal/service/accountdelete.go` | Phase 7.1 (username column, clearing on delete) | The username batch read Phase 8 joins on; where deletion frees the username |
| `internal/service/pushkinds.go`, `alertsettings.go`, `pushdispatch.go` | Phase 5 (official kind, `official_push_on`) | `PushKind.Valid` list and `changeEventWords` while adding the two comment kinds |
| `internal/testutil/db.go` Truncate | Phase 5 (new tables), 7.1 | The one TRUNCATE list; append the comment tables last |
| `internal/store/migrations/` and `migrations_test.go` | Phase 5, 7.1 | Next free number only at execution time |
| `web/css_contract_test.go`, `web/template_contract_test.go`, `web/design_rules_contract_test.go` | Phase 5 (icons, official scripts) | `nonCategoryIcons`, script order, `phaseScripts` |

## No Analog Found

| File or part | Role | Data flow | Reason |
|---|---|---|---|
| Public share token column and its lookup | store | capability token | No other public capability token exists; `magic_link_tokens.sql` is single use and hashed, the share token is reusable and stored plain on purpose |
| Thumbs up, share, speech bubble icons | asset | n/a | Must be new registered SVG mask icons (`TestNewIconsAreRegisteredAndUsed`); follow the registration in `css_contract_test.go`, no emoji, no image files added |
| Share window as a bottom sheet on the phone | component | request-response | Only the centred `.modal-backdrop` dialogs exist; `sheet.js` / `sheet_contract_test.go` is the nearest for the sheet behaviour |
| Folded replies and "View N replies" thread control | component | CRUD | No threaded list in the repo; build from `feed.js` list row patterns and `mute.js` small stateful control |
| Open Graph `<head>` for a no session page | template | request-response | No template emits Open Graph tags; use RESEARCH "Public page template fragment" |
| `navigator.share` and clipboard fallback | component | request-response | Not used anywhere yet |

## Metadata

**Analog search scope:** `internal/service`, `internal/api`, `internal/api/handlers`, `internal/store/queries`, `internal/store/migrations`, `internal/testutil`, `web/static/js`, `web/templates`, `web/*_test.go`, `web/*.mjs`, Phase 5 `05-PATTERNS.md` and plan 05-04 headers, Phase 7.1 CONTEXT.
**Files read this pass:** history.go, flag.go, handlers/flag.go, screening.go, textcheck.go, modworker.go, ratelimit_gate.go, router.go (405 to 540, 575 to 605, 705 to 794), gate_test.go (307 to 488), pwa_routes_test.go (100 to 199), legal.go, map.js (185 to 265, 780 to 915), flag.js header, flag_dialog.html.tmpl, moderation.go handlers (194 to 224), accountdelete.go (1 to 60), updates.go (20 to 70), SQL excerpts from votes.sql, updates.sql, push.sql, moderation.sql, migration 00010.
**Not opened (named by RESEARCH, open them when a plan quotes them):** trustcopy.go, moderation.go (service), pushcopy.go, pushworker.go, ratecopy.go, report_view_test.mjs body, phone.css, mute.js, updates.js.
**Pattern extraction date:** 2026-10-25

## PATTERN MAPPING COMPLETE
