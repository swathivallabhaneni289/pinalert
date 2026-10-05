# Phase 4: Robustness, Real-World Resilience - Pattern Map

**Mapped:** 2026-10-05
**Files analyzed:** 52 (new or edited, grouped where one pattern serves several)
**Analogs found:** 48 / 52 (4 have only a partial analog, listed at the end)
**Sources:** 04-CONTEXT.md, 04-RESEARCH.md (file additions, Areas A to F, collisions table), 04-UI-SPEC.md "Hand Off Notes" only.

Reading note for the planner. Phases 3, 3.1, 3.2 are not built. Every excerpt below is TODAY's code (line numbers are today's). Where Phase 4 edits a file Phase 3 also edits, the row says "Phase 3 plan 03-NN edits this first" (from the `files_modified` frontmatter of the Phase 3 plans). Re-grep the built file before writing any task action. Never write a migration number: take the next free one at execution time (today the last is `00004_create_votes.sql`, 03-06 adds `00005`).

## Corrections after the code check (2026-10-05)

A 32 agent read only check against the real code corrected this file and `04-RESEARCH.md` in place (see the "Corrections after the code check" section at the top of RESEARCH for the full list). The ones that matter here: refunds must use `CancelAt(reservation instant)` (a later `Cancel()` is a silent no-op), idle windows are burst times refill interval per bucket, the moderator paths are registered with `r.Handle` so wrong methods cannot leak through chi's 405, `Submit` has one caller, `ReportExtrasForReports` goes on `Querier` and `AuthQuerier` only, the `Resolve` guard test strips comments and allowlists `reliability.go`, the all files JS sink test needs a `//` aware stripper, and `low.js` must subscribe to the shared store because `votes.js` refetches after every vote.

## File Classification

### Go backend

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/moderation/client.go` | outbound client | request-response (HTTP, timeout, retry) | `internal/geocode/client.go` | role-match (OpenAI call is POST JSON, geocode is GET) |
| `internal/moderation/loader.go` (`LoadScreener(env)`) | config / loader | startup selection | `internal/mailer/loader.go` | exact |
| `internal/moderation/client_test.go` | test | httptest upstream | `internal/geocode/client_test.go` | exact |
| `internal/moderation/loader_test.go` | test | env matrix | `internal/mailer/loader_test.go` | exact |
| `internal/moderation/testdata/*.json` | fixture | batch | none in repo | no analog (see end) |
| `internal/service/screening.go` (pure `Decide`, gibberish rule, copy) | service (pure policy) | transform | `internal/service/visibility.go` (`Resolve`) | role-match |
| `internal/service/screening_test.go`, `screening_fixture_test.go` | test | table driven | `internal/service/visibility_test.go` | role-match |
| `internal/service/modworker.go` (background sweep) | service (worker) | batch / timer | `internal/ratelimit/perip.go` `sweepLoop` | partial |
| `internal/service/moderation.go` (state, queue reads, Keep, Remove, Flag) | service | CRUD | `internal/service/trust.go` (`VotingService`) | role-match |
| `internal/service/visibility.go` (edit) | service (pure) | transform | itself | exact |
| `internal/service/report.go` (edit: `Submit`, `Querier`, `Nearby`) | service | CRUD | itself | exact |
| `internal/service/trust.go` (edit: `CastVote` withheld refusal) | service | CRUD | itself | exact |
| `internal/service/auth.go` (edit: `ActivityForAccount`, `AuthQuerier`) | service | CRUD | itself | exact |
| `internal/service/feed.go` (edit: `ListableInFeed`) | service | transform | itself | exact |
| `internal/ratelimit/perkey.go` (+ test) | utility | in-memory token bucket | `internal/ratelimit/perip.go` | exact |
| `internal/api/handlers/ratelimit_gate.go` (429 bodies) | middleware | request-response | `internal/ratelimit/perip.go` `Middleware` + `writeTooManyRequests` | role-match |
| `internal/api/handlers/reports.go` (edit) | controller | request-response | itself | exact |
| `internal/api/handlers/votes.go` (edit) | controller | request-response | itself | exact |
| `internal/api/handlers/flag.go`, `moderation.go` | controller | request-response | `internal/api/handlers/votes.go` `CastVote` | exact |
| `internal/api/handlers/legal.go`, `low.go` | controller (page) | request-response | `internal/api/handlers/page.go` `Page` | exact |
| `internal/api/handlers/page.go` (edit: bare `/` redirect, Chrome embed) | controller | request-response | itself | exact |
| `internal/api/handlers/auth.go` (edit: Activity rows, `data-account-id`) | controller | request-response | itself | exact |
| `internal/api/moderator.go` (`requireModerator`) | middleware | request-response | `internal/api/gate.go` `requireVerifiedAccount` | exact |
| `internal/api/gate.go` (edit: `/low` redirect to `/login?low=1`) | middleware | request-response | itself | exact |
| `internal/api/router.go` (edit) | config / route table | request-response | itself | exact |
| `cmd/server/main.go` (edit: constants, wiring, worker start) | config | startup | itself | exact |
| `internal/store/migrations/NNNNN_*.sql` | migration | schema | `00003_add_email_cooldowns.sql`, `00004_create_votes.sql` | exact |
| `internal/store/queries/moderation.sql` (+ edits to `reports.sql`, `votes.sql`) | sqlc query | CRUD / batched read | `internal/store/queries/votes.sql`, `email_cooldowns.sql` | exact |
| `internal/store/sqlc/*.go` | generated | n/a | committed generated files | exact (run `make sqlc`) |
| `internal/testutil/db.go` (edit `Truncate`) | test util | n/a | itself | exact |
| Go tests: `*_e2e_test.go`, `gate_test.go`, `feed_test.go` fakes | test | request-response | `reports_e2e_test.go`, `feed_test.go` | exact |

### Web

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `web/templates/disclaimer_bar.html.tmpl` | partial | render | `web/templates/theme_toggle.html.tmpl`, `account_header.html.tmpl` | exact |
| `web/templates/disclaimer.html.tmpl`, `policy.html.tmpl` | full page | render | `web/templates/verify_outcome.html.tmpl` | role-match |
| `web/templates/low.html.tmpl` | full page | render + client fetch | `web/templates/profile.html.tmpl` (shell) and `index.html.tmpl` (app-shell data attributes) | role-match |
| `web/templates/moderate.html.tmpl` | full page | render + client fetch | `web/templates/profile.html.tmpl` | role-match |
| shared report form partial | partial | render | the modal block in `index.html.tmpl` | exact (extract) |
| edits to `index.html.tmpl`, `login_gate.html.tmpl`, `verify_outcome.html.tmpl`, `profile.html.tmpl`, `account_header.html.tmpl` | full pages | render | themselves | exact |
| `web/static/js/queue.js` | module | IndexedDB, event-driven | `web/static/js/votes.js` (module shape, `window.PinalertX`) | partial |
| `web/static/js/flag.js` | module | event-driven | `web/static/js/votes.js` `createVoteBlock` | role-match |
| `web/static/js/report-form.js` | module (extracted from modal.js) | request-response | `web/static/js/modal.js` | exact (move) |
| `web/static/js/low.js` | module | fetch + render | `web/static/js/feed.js` + `map.js` `centerOnVisitor` | role-match |
| `web/static/js/moderate.js` | module | fetch + render | `web/static/js/activity.js` | role-match |
| `web/static/js/slow-notice.js` | module (head, not deferred) | timer | `web/static/js/theme.js` (head, non-deferred) | role-match |
| edits to `app.js`, `votes.js`, `feed.js`, `map.js`, `modal.js`, `visibility.js`, `activity.js`, `account-menu.js` | modules | n/a | themselves | exact |
| `web/static/css/chrome.css`, `legal.css`, `low.css`, `moderate.css`, `modal-map.css` | stylesheet | n/a | `feed.css` (hidden override form), `auth.css` | role-match |
| Contract tests in `web/*_contract_test.go` (new and re-pointed) | test | static source scan | `web/theme_contract_test.go`, `web/votes_contract_test.go`, `web/design_rules_contract_test.go` | exact |

## Pattern Assignments

### `internal/moderation/loader.go` (loader, startup selection)

**Analog:** `internal/mailer/loader.go` lines 34 to 50 (the whole function). Copy the three way branch exactly.

```go
func LoadMailer(env string) (Mailer, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey != "" {
		from := os.Getenv("RESEND_FROM")
		if from == "" { from = defaultFrom }
		return NewResendMailer(apiKey, from), nil
	}
	if env != "development" {
		return nil, fmt.Errorf("mailer: RESEND_API_KEY must be set outside ENV=development")
	}
	log.Println("WARNING: RESEND_API_KEY unset; mail will be printed to the log instead of sent")
	return NewLogMailer(), nil
}
```

Copy: key set wins regardless of env; unset outside `development` returns an error naming the variable (main.go turns it into `log.Fatal`, the loader never fatals itself, so it stays testable); the dev fallback logs a WARNING. The dev fallback for moderation should be a deterministic stub screener that approves everything locally and never touches the network (R-10 for demo data is separate: seeded rows never call the provider at all).
Differ: model env var is the dated snapshot `omni-moderation-2024-09-26` (pin it as a constant, CONTEXT Claude's Discretion). Return a concrete struct or a one method interface (see interface note below).

**Test analog:** `internal/mailer/loader_test.go` lines 13 to 83: `t.Setenv("RESEND_API_KEY", ...)` per subtest, assert the concrete type (`m.(*resendMailer)`), the error text names the variable, no call is ever made on the constructed client. Copy the five case matrix (key set with production, key set with development, no key with development, no key with empty env, no key with production); the last two both assert that the error names the variable and that the returned value is nil.

### `internal/moderation/client.go` (outbound client, 3 s deadline, bounded retry)

**Analog:** `internal/geocode/client.go`. Package comment (lines 1 to 16): standalone package with no import of `internal/service` or `internal/api`. Constants block with named timeouts (lines 30 to 54). Struct plus unexported constructor taking a base URL so tests drive an `httptest` server (lines 81 to 109):

```go
type Client struct {
	httpClient   *http.Client
	limiter      *rate.Limiter
	baseURL      string
	contactEmail string
}
func NewClient(contactEmail string) *Client {
	return newClientWithBaseURL(nominatimSearchURL, contactEmail, defaultLimiterEvery, defaultLimiterBurst)
}
func newClientWithBaseURL(baseURL, contactEmail string, every time.Duration, burst int) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: httpClientTimeout},
		limiter:    rate.NewLimiter(rate.Every(every), burst),
		baseURL:    baseURL, contactEmail: contactEmail,
	}
}
```

Request build (lines 131 to 146): fixed base URL constant so the client can never become an open proxy; `http.NewRequestWithContext`; body built with a typed struct, never string concat. Search (lines 154 to 170): limiter `Wait` bounded by its own context timeout, then `httpClient.Do`, `defer resp.Body.Close()`, errors wrapped with `fmt.Errorf("geocode: ...: %w", err)` naming the operation only.
Differ: (1) POST with `Authorization: Bearer <key>` and JSON `{"model":..., "input": text}`; never log the key or the text. (2) Status mapping the screener must expose to the service: `429`, `5xx` and timeout map to a "could not finish" outcome carrying `Retry-After`; `insufficient_quota` is not retried. (3) Inline path makes ONE attempt within about 3 s (RESEARCH diagram); retries live in the background worker, not in a loop inside the handler. (4) Process wide limiter here should mirror the free tier (250 per minute) and fall into the pending path when spent, rather than `Wait` (geocode waits, moderation must not queue a person). (5) Only the cleaned description string is sent (D-01).
**Interface seam:** copy `handlers.GeocodeSearcher` (`internal/api/handlers/geocode.go` lines 14 to 24): a one method consumer defined interface in the consumer package (`service` for the screener), with a test stub; a nil value must degrade, not panic (see `TestGeocode_NilSearcherReturnsFriendlyError`).

**Test analog:** `internal/geocode/client_test.go`: `TestClient_Search_UpstreamNon200` (line 207), `_MalformedBody` (225), `_ContextCancelled` (244), `_LimiterWaitTimeout` (174), `TestNominatimSearchURLIsHTTPS` (270, a constant guard worth copying for the OpenAI URL). Add tests for `Retry-After` parsing, jitter bounds, and that no retry occurs on `insufficient_quota`.

### `internal/service/screening.go` (pure policy)

**Analog:** `internal/service/visibility.go` `Resolve` (lines 199 to 213) and its doc comment style (lines 161 to 198): "the order is the specification", pure, no I/O, no `context`, exhaustive table test. Make `Decide(scores, serious) Verdict` pure the same way, with the reject, borderline and pass rungs in a documented order. Rune counting: use `utf8.RuneCountInString` (the pattern in `internal/api/handlers/geocode.go` lines 31 to 38, quoted: "counted with utf8.RuneCountInString rather than len ... this is an India-facing app"). R-06: `ValidateSubmitInput` in `report.go` counts bytes with `len`; fix only the description check the screening touches.
**Test analog:** `internal/service/visibility_test.go` (table driven over `Resolve`). The D-07 fixture test lives beside it and loads `testdata/*.json` of RECORDED scores (no live call in tests, per the mailer rule "tests never make live calls").

### `internal/service/visibility.go` (edit: moderation rung)

**Today (lines 71 to 87, 199 to 213):**

```go
type ReportMeta struct {
	Severity Severity
	Category Category
}
func (m ReportMeta) criticalBypass() bool {
	return m.Severity == SeverityCritical || m.Category == CategoryRescueNeeded
}
func Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, ResolveReason) {
	if tally.isRetracted() { return VisibilityRetracted, ReasonResolved }
	if meta.criticalBypass() { return VisibilityLive, ReasonCriticalBypass }
	...
```

Per RESEARCH Moderation state model items 3 to 5: add `Moderation ModerationState` to `ReportMeta` (zero value `clear`), add a fifth `VisibilityWithheld` to `Visibilities` (line 24) so `Valid()` (line 28) accepts it, add two `ResolveReason` slugs to `ResolveReasons` (lines 54 to 69), put `removed` ABOVE the Retracted rung and `pending && !serious` next, keep the rest. Add an exported `Serious()` wrapper rather than renaming `criticalBypass` (Phase 3 docs cite the name; R-04). The reasons are a wire contract: `docs/swagger.json` enums and drift guards change (see handlers). Phase 3 plan 03-03 `ClassifyOutcome` builds `ReportMeta{Severity: SeverityLow, Category: CategoryOther}`: the zero value `clear` keeps it neutral (R-02).

### `internal/service/report.go` (edit: `Submit`, `Querier`, `Nearby`)

**Analog:** itself. `Querier` (lines 286 to 291) and the batched read pattern in `Nearby` (lines 428 to 463):

```go
voteRows, err := s.q.CurrentVotesForReports(ctx, ids)
reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
...
meta := ReportMeta{Severity: r.Severity, Category: r.Category}
vis, reason := Resolve(meta, tally, now)
if !vis.ListableInFeed(q.IncludeDisputed) { continue }
```

Add `ReportExtrasForReports(ctx, ids, viewerAccountID)` as ONE more batched call after the two existing ones, only when the page is non empty (the empty page early return at line 417 stays). Pass `Moderation` into `meta` at line 449. Add the method to `Querier` and `AuthQuerier` (`auth.go` 51) and to their two fakes (`fakeFeedQuerier` in `feed_test.go` lines 16 to 50 is the shape: it records a `callLog`, `lastXIDs`, and returns canned rows and an error field; `fakeAuthQuerier` in `auth_test.go`). `VotingQuerier` does NOT get it: `CastVote` reads `moderation_state` through the extended `ReportVoteContext` row, so only `fakeVotingQuerier.ReportVoteContext` and its `voteContextRow` literals change (`trust_test.go`).
`Submit` (lines 307 to 346): `now` is captured once server side at line 312 and `expires_at` is computed from severity; this stays. Add the idempotent insert (`InsertReportIdempotent`, see queries) and the held flag. `created_at` and `expires_at` are never taken from the client (R-05); `client_written_at` is a display only column.
Collisions (RESEARCH table rows 3, 4, 5): Phase 3 plans 03-05 and 03-12 edit `report.go`, `trust.go`, `auth.go` first. Phase 4 adds a source guard test (comments stripped before matching, because `feed.go` line 47 mentions `Resolve()` in a comment): every non test `.go` file in `internal/service` that calls `Resolve(` outside comments must also contain `Moderation` in code, except `reliability.go`, which is allowlisted by name for Phase 3 plan 03-03's one deliberate neutral call (the test asserts it still has exactly one non comment `Resolve(` call). See RESEARCH Moderation state model item 7.
**Test analog for the new batched call:** `internal/service/feed_test.go` `TestNearbyBatchesVoteReadsOnce` (lines 439 to 486): asserts `countCalls(fake.callLog, "X") == 1` for a 25 report page and `== 0` for an empty page. Copy for `ReportExtrasForReports`.

### `internal/service/trust.go` (edit: `CastVote` withheld refusal)

**Analog:** itself, lines 290 to 312. Insert the withheld refusal right after step 2:

```go
rc, err := s.q.ReportVoteContext(ctx, in.ReportID)
if err != nil {
	if errors.Is(err, pgx.ErrNoRows) { return CastVoteResult{}, ErrReportNotFound }
	return CastVoteResult{}, err
}
```

Add `r.moderation_state` to the `ReportVoteContext` select in `queries/votes.sql` (the query ends with `FROM reports r LEFT JOIN sessions s ON r.session_id = s.session_id WHERE r.id = sqlc.arg(report_id)`; Phase 3 plan 03-01 also edits this select: sequential) and answer `ErrReportNotFound` for a withheld report (so a vote on a report the viewer cannot see is a 404, and `CastVoteResponse` never carries a fifth enum value). Pass `Moderation: ModerationState(rc.ModerationState)` into the `ReportMeta` built at line 360. This amends Phase 3's pinned `TestCastVoteCheckOrder`: list it as a test that changes. Keep limiter code OUT of `trust.go`.

### `internal/ratelimit/perkey.go` (utility, in memory token bucket with reserve and refund)

**Analog:** `internal/ratelimit/perip.go`. Reuse the structure: entry with limiter plus `lastSeen` (lines 34 to 37), `mu sync.Mutex` map (50 to 56), constructor pair `NewPerIP` and unexported `newPerIPWithIdleWindow` as the test seam (66 to 82), `sweepLoop` plus `sweep(now)` (90 to 109), `Allow` (113 to 124):

```go
func (p *PerIP) Allow(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[key]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(p.limit, p.burst)}
		p.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter.Allow()
}
```

Differ (RESEARCH Area C): key is the account id string, not the IP; add `Reserve(key) Decision{OK, RetryAfter, Cancel}` with a refund built on `rate.Limiter.ReserveN(now, 1)` and `Reservation.CancelAt(now)` using the SAME captured instant, wrapped in `sync.Once` (never `Reserve()`, `Cancel()` or `CancelAt` at a later time: those are silent no-ops for a zero delay grant, and a test with a frozen clock would hide that), an injectable clock for tests, and an idle window per bucket of at least burst times refill interval, because evicting a bucket that has not refilled to full hands back a full burst (the 10 minute default `defaultIdleWindow` is too short for the report buckets): 120 minutes for the other-report bucket (8 x 15), 25 minutes for the serious bucket (5 x 5), 20 minutes for the Flag bucket (10 x 2), 5 minutes for votes (20 x 15 s), 30 seconds for the re-cast cooldown, 2 minutes for the submit attempt bucket, 20 minutes for the IP backstop; size it per instance from those products. Keep the package free of `internal/service` and `internal/api` imports (package comment, lines 1 to 5). Keep the "do not hand roll bucket arithmetic" rule (lines 6 to 11): wrap `golang.org/x/time/rate`.
`PerIP.Middleware` (lines 144 to 158) must NOT be reused unchanged: it falls back to `r.RemoteAddr` host (line 149) and `writeTooManyRequests` (lines 175 to 179) hardcodes `"field":"email"` and the message at line 29, and sends no `Retry-After`. Leave `perip.go` byte identical, because `perip_test.go` and `TestAccessGateExemptRoutes` style tests pin the body (`agreedRateLimitSentence` in `web/design_rules_contract_test.go`).
**Test analog:** `internal/ratelimit/perip_test.go`: `TestAllowsBurstThenRefuses` (16), `TestRefillAfterInterval` (31), `TestIndependentKeysHaveIndependentBudgets` (51), `TestConcurrentAllowDoesNotRace` (70), `TestIdleEntryIsEvicted` (90). The two report buckets being independent is the same test as line 51, per bucket instance.

### `internal/api/handlers/ratelimit_gate.go` (429 body with `Retry-After`)

**Analog for the envelope:** `writeFieldError` as used in `votes.go` (field plus message) and `ratelimit.writeTooManyRequests` for the 429 status write. Write the new body as `{"error":{"field":...,"message":...},"retry_after_seconds":N,"serious":bool,"emergency_number":"112"}` plus `w.Header().Set("Retry-After", strconv.Itoa(secs))` before `WriteHeader`. The `serious` flag comes from `ReportMeta.Serious()` (R-04), the number from the one numbers setting (D-22). Never document an account field in swagger (name guard `(?i)cell|account|session|email`, collisions row 9). Every block is logged with `log.Printf` (D-12); the handler logging style is `log.Printf("handlers: CastVote: %v", err)` (`votes.go`).
**Client IP backstop:** router today uses `middleware.ClientIPFromRemoteAddr` (`router.go` line 153). Use `middleware.ClientIPFromHeader(name)` when a deploy setting names a header; when the resolved address is empty skip the network limit entirely, never a shared bucket (CONTEXT Discretion). `internal/api/ratelimit_spoof_test.go` is the existing spoof test: read it before touching the ClientIP wiring.

### `internal/api/handlers/reports.go` (edit: `SubmitReport`)

**Analog:** itself, lines 41 to 56 (`SubmitReportRequest`), 63 to 78 (`ReportResponse`), 241 to 289 (`SubmitReport`). Current handler order: session id from context (243), `MaxBytesReader` then `DisallowUnknownFields` decode (249 to 257), build `service.SubmitInput` (265), `svc.Submit`, `errors.As` to `service.ValidationError` then 400 via `writeFieldError`, else `log.Printf` and generic 500 (275 to 285), `writeJSON(w, http.StatusCreated, ...)` (287).

```go
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields()
var req SubmitReportRequest
if err := dec.Decode(&req); err != nil {
	writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
	return
}
```

Add `RequestID string \`json:"request_id,omitempty" example:"..."\`` and `WrittenAt *string` explicitly (the decoder rejects unknown fields). Required order (RESEARCH "Handler order for POST /api/reports", the order is the specification): decode, validate (runes), account scoped replay lookup (account from `account.FromContext`, as `NearbyReports` does at line 329, never from the body), reserve allowance (serious or other bucket), prefilter, provider call, insert, 201. A replayed `request_id` answers 200 with the stored report and does NOT reserve an allowance or call the provider. `ReportResponse` gains `held bool`; `FeedReportResponse` gains optional `written_at` and the viewer flagged marker; `Visibility` enum (line 114) is unchanged (withheld never reaches the feed) but `VisibilityReason` enum (line 119) is NOT extended for the feed either, since withheld rows are not listable: only the Activity page sees them. Swag annotations (`@Failure 429`, `@Failure 422`) follow the block at lines 227 to 240. Collision: Phase 3 plans 03-04 and 03-07 edit `reports.go` and `votes.go` first; regenerate swagger ONCE at the end (`docs/docs.go`, `swagger.json`, `swagger.yaml`) and re check `swagger_test.go` guards.

### `internal/api/handlers/votes.go` (edit: vote limiter 429, flag handler pair)

**Analog:** `CastVote` lines 103 to 190. The error switch ordering (lines 150 to 170) puts `errors.Is` sentinels before `errors.As(ValidationError)`; keep it. The vote limiter wraps routes via `r.With(...)` in the router (do not put limiter code in the handler or `trust.go`); the cooldown for re casting the same vote refunds on a non 2xx outcome (RESEARCH diagram). `flag.go` is a near copy of `CastVote`: same `account.FromContext` guard (104 to 112), `parseReportID` (114), `MaxBytesReader` plus `DisallowUnknownFields` (119 to 127), then `ErrReportNotFound` to 404 for a withheld or missing report. `moderation.go` handlers are the same shape again, each behind `requireModerator`.

### `internal/api/moderator.go` (middleware, 404 for everyone else on every method)

**Analog:** `internal/api/gate.go` `requireVerifiedAccount` (lines roughly 30 to 70). Core:

```go
sessionID, ok := session.FromContext(r.Context())
if ok && q != nil {
	row, err := q.GetAccountBySessionID(r.Context(), sessionID)
	switch {
	case err == nil:
		ctx := account.WithAccount(r.Context(), account.Account{ID: row.ID, Email: row.Email})
		next.ServeHTTP(w, r.WithContext(ctx)); return
	case errors.Is(err, pgx.ErrNoRows):
	default:
		log.Printf("api: requireVerifiedAccount: looking up account for session: %v", err)
	}
}
```

Copy the closure shape and the `GetAccountBySessionID` lookup, but give `requireModerator` its OWN lookup (RESEARCH "Moderator gate with its own lookup") and compare the verified email (case folded) against the configured list from `cmd/server/main.go`; every failure, including a lookup error and a logged out visitor, answers a plain 404 with no redirect and no JSON body that reveals the route exists. Do not stack it inside a context that already redirects to `/login`. Method trap (verified against chi v5.3.2 `routeHTTP`): chi answers 405 plus `Allow` for a matched path with an unregistered method BEFORE group middleware runs, so register every moderator path with `r.Handle` (all methods) inside the gate group, never `r.Get` or `r.Post`, and let each handler answer a real moderator's wrong method with its own 405; the byte equality test table also covers wrong methods (HEAD, PUT, GET on the POST routes, POST on `/moderate`). Existing gate tests: `internal/api/gate_test.go` (`newGateTestServer`, `newGateJarClient`) are the harness to copy for the "404 to everyone else" test.
`gate.go` edit: today a logged out non `/api/` path gets `Cache-Control: no-store` and `http.Redirect(w, r, "/login", 302)`. Add a branch: path `/low` redirects to `/login?low=1`. The existing test pins `GET /` to exactly `/login` and stays untouched.

### `internal/api/router.go` (edit)

**Analog:** itself. Exempt routes registered flat above the group (lines 160 to 195); gated routes inside `r.Group(func(r chi.Router){ r.Use(requireVerifiedAccount(deps.Sessions)) ... })` (199 to 252); route scoped limiter via `r.With(limiter.Middleware()).Post(...)` (195, 240):

```go
geocodeLimiter := ratelimit.NewPerIP(geocodeLimit.Every, geocodeLimit.Burst)
r.With(geocodeLimiter.Middleware()).Get("/api/geocode", handlers.Geocode(deps.Geocode))
```

Add `GET /disclaimer`, `GET /policy` in the exempt block (set `Cache-Control: private, max-age=300`, not public, because the session middleware sets a cookie on any first visit: RESEARCH Area F caching row). Update the doc comment "The exempt set is exactly: ..." (lines 138 to 143) and `TestAccessGateExemptRoutes` (`gate_test.go` line 280, a five case table `{name, method, path, body}`: add two rows). Add `GET /low`, the flag route, vote limiter via `r.With(...)` on the four vote routes (lines 224 to 227), and a separate `r.Group` with `requireModerator` for `/moderate` and `/api/moderation/*`. Deps additions follow the existing additive style: zero valued fields keep every `Deps{}` literal (such as `swagger_test.go`'s) compiling, with a `...Default` var as in `GeocodeRateLimitDefault` (line 109). Optionally `middleware.Compress(5, ...)` (RESEARCH Area D: the router installs no compression today).

### `cmd/server/main.go` (edit: constants, wiring, worker)

**Analog:** itself. Named budget constants with a rationale comment (lines 38 to 59):

```go
const (
	requestLinkRateLimitBurst = 5
	requestLinkRateLimitEvery = 60 * time.Second
)
```

Add the D-11 numbers the same way, all in one place: serious burst 5 / every 5 min, other burst 8 / every 15 min, votes burst 20 / every 15 s, re cast cooldown 30 s, Report button burst 10 / every 2 min. Load `LoadScreener(env)` next to `mailer.LoadMailer(env)` (line 110) and `log.Fatalf` on error. Optional env reads follow `NOMINATIM_CONTACT_EMAIL` (line 128): warn when unset (`CONTACT_EMAIL`, `CLIENT_IP_HEADER`, moderator email list, emergency numbers). Start the worker goroutine with the process (time.Ticker, one sweep at boot). `deps := api.Deps{...}` at line 134 is where each new field is wired. Phase 3 plan 03-18 also edits `main.go` and `page.go` (`PageConfig.VoteRadiusKm`): sequential.

### Migrations and queries

**Migration analog:** `internal/store/migrations/00003_add_email_cooldowns.sql` (goose header `-- +goose Up` / `-- +goose Down`, a comment block stating WHY each constraint exists, `DROP TABLE` in Down). `00004_create_votes.sql` is the long form comment style for a table with deliberate omissions. Per RESEARCH Moderation state model: columns on `reports` (`moderation_state TEXT NOT NULL DEFAULT 'clear'`, `request_id`, `request_account_id BIGINT REFERENCES accounts(id)`, `client_written_at`, partial unique index `UNIQUE (request_account_id, request_id) WHERE request_id IS NOT NULL`) so `NearbyReports` (pinned by `TestNearbyReportsQuerySourceHasExpectedShape` and the EXPLAIN test in `internal/store/reports_test.go`) is untouched; side tables `moderation_checks`, `report_flags` (UNIQUE report_id, account_id), `moderation_actions`. Existing raw INSERTs in `internal/testutil/seed.go` keep working through the defaults. Migrations are an explicit step (`cmd/migrate/main.go`), never on boot. Check `internal/store/migrations_test.go` for a migration count or name pin and update it.
**Query analog for the idempotent insert:** `internal/store/queries/email_cooldowns.sql` lines 11 to 16 (single statement atomic claim; "no row means refused" surfaces as `pgx.ErrNoRows`):

```sql
INSERT INTO email_cooldowns (email, last_requested_at)
VALUES (sqlc.arg(email), sqlc.arg(requested_at))
ON CONFLICT (email) DO UPDATE
SET last_requested_at = EXCLUDED.last_requested_at
WHERE email_cooldowns.last_requested_at <= sqlc.arg(cooldown_cutoff)
RETURNING last_requested_at;
```

Copy for `InsertReportIdempotent ... ON CONFLICT (request_account_id, request_id) WHERE request_id IS NOT NULL DO NOTHING RETURNING ...`; no row back then run the account scoped `ReportByRequest`. Style: `-- name: X :one|:many|:exec`, a comment block explaining the atomicity, `sqlc.arg(name)` parameters, a `RETURNING` list that never includes `session_id` (T-01-02).
**Query analog for the batched read:** `votes.sql` `CurrentVotesForReports` (`WHERE report_id = ANY(sqlc.arg(report_ids)::bigint[])`), used for `ReportExtrasForReports(report_ids, viewer_account_id)` (LEFT JOIN `report_flags` on the viewer). Worker claim: `FOR UPDATE SKIP LOCKED` with a lease column (no analog exists, see the end).
sqlc config (`sqlc.yaml`): `sql_package: pgx/v5`, `emit_pointers_for_null_types: true`, non null `timestamptz` maps to `time.Time`. Generated files under `internal/store/sqlc/` are committed (CI has no sqlc); run `make sqlc` after Phase 3 and 3.1 land, and read the committed structs, not names quoted in plans.
`internal/testutil/db.go` line 77 lists tables explicitly: `TRUNCATE accounts, magic_link_tokens, email_cooldowns, reports, sessions, votes RESTART IDENTITY CASCADE`. Add the new tables there (Phase 3 plan 03-06 also adds `testutil/history.go`). Remember the standing rule: only a disposable database, never `pinalert_test` or `pinalert`.

### Go handler and e2e test patterns

**Real router over real Postgres:** `internal/api/handlers/reports_e2e_test.go` lines 38 to 71 (`newE2EServer`: `testutil.NewTestDB`, `session.NewManager`, `handlers.ParsePageTemplate`, `api.Deps{...}`, `httptest.NewServer(router)`, `t.Cleanup(srv.Close)`) plus `verifySession` (lines 78 to 85) and `postReport` (88+). Add the new Deps fields to this harness (stub screener, small limiter budgets) so each e2e test can force the 429, the pending path and the replay.
**Handler unit test with an interface stub:** `internal/api/handlers/geocode_test.go` lines 17 to 44: a local `stubSearcher` recording calls and `gotQuery`, plus `decodeErrorBody`. Copy for a `stubScreener` (returns canned scores, an error, or a timeout) and assert zero calls when the replay or limiter short circuits first.
**Service test with a fake Querier:** `feed_test.go` `fakeFeedQuerier` (above). Fakes gain `ReportExtrasForReports` in two service test files (`feed_test.go`, `auth_test.go`); `trust_test.go` changes only `fakeVotingQuerier.ReportVoteContext` and its `voteContextRow` literals.

### `internal/service/moderation.go`, `modworker.go`

**Analog (service shape):** `VotingService` in `trust.go` lines 208 to 229: a consumer defined `Querier` interface, a struct holding it, `NewXService(q)`, step by step doc comment where "the order is part of the specification" and sentinel errors (`ErrReportNotFound`, line 106) mapped by the handler's `errors.Is` switch. Keep Keep and Remove as small methods that write `moderation_actions` and flip `moderation_state` in one statement (no transaction exists in the store today: `sqlcgen.Queries.WithTx` exists but no service uses it).
**Analog (background loop):** `ratelimit.PerIP.sweepLoop` (`perip.go` 90 to 96): `time.NewTicker`, `defer ticker.Stop()`, `for range ticker.C`. Differ: take a `context.Context` so it can stop in tests, claim due `moderation_checks` rows with a lease, backoff with jitter across sweeps, honour `Retry-After`. No analog exists for the SQL claim.

### Templates

**Full page set derivation (R-08):** `web/theme_contract_test.go` `findFullPageTemplates` lines 30 to 62: walks `TemplatesFS`, treats any `.tmpl` containing `<html` as a full page, and fatals when fewer than 4 are found. The bar partial MUST NOT contain the text `<html`, even in a comment (it would be classed as a full page). New "bar on every page" test: for each page from `findFullPageTemplates`, require one call to the bar partial inside `.top-chrome`, first in `<body>`. `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` (line 79) already requires `theme.js` in the head of every full page, so the four new full pages (`low`, `disclaimer`, `policy`, `moderate`) inherit that check; copy the head from `profile.html.tmpl` lines 7 to 11:

```html
<link rel="stylesheet" href="/static/css/main.css?v={{.AssetVersion}}">
...
<script src="/static/js/theme.js?v={{.AssetVersion}}"></script>
```

and the deferred tail from lines 54 to 58 (`<script src="/static/js/app.js?v={{.AssetVersion}}" defer></script>` and so on). Script order per page is fixed in UI-SPEC Hand Off Notes "Script order" (main page, Low data view, Activity, moderator, login and legal pages). The main page keeps the three blocking unpkg tags (`index.html.tmpl` lines 124 to 131) before the deferred scripts (133 to 139); `slow-notice.js` goes first in `<head>`, before every `<link>`.
**One template set:** `ParsePageTemplate` is `template.ParseFS(web.TemplatesFS, "templates/*.tmpl")` (`page.go` line ~58): every partial is parsed into the same set and runs with the CALLING page's view model, so a missing field is an execute time 500, not a compile error. Embed one `Chrome` struct in `pageViewModel` (`page.go` lines 31 to 43), `loginGateViewModel`, `verifyOutcomeViewModel`, `profileViewModel` and the new models, and add a test that renders every full page. No template funcs exist; tests call `ParsePageTemplate` directly.
**Partial analogs:** `web/templates/theme_toggle.html.tmpl` and `account_header.html.tmpl` (called with `{{template ...}}`; the header renders `{{.Email}}`). Legal pages must NOT call the account header and must render no email (R-07, DEC-Q: `Page` sets `no-store` because of the email).
**Page handler analog:** `handlers.Page` (`page.go` lines 64 to 90): `account.FromContext` guard that logs and 500s, builds a `pageViewModel` from `PageConfig`, `ExecuteTemplate`. `LowPage`, the moderator page and `Legal(tmpl, cfg)` copy this. `Page` gains the bare `/` redirect: cookie `pinalert_low_data=1` and no query string gets 302 to `/low`; `/?full=1` clears the cookie; any main page address with a query string is never redirected.
**Dash gate:** `TestUserVisibleCopyUsesPlainPunctuation` (`web/design_rules_contract_test.go` lines 75 to 93) scans every `.tmpl` line for U+2014 and U+2013. The long Disclaimer and Policy text must be fully dash free, including template comments.

### JS module patterns (all new modules)

**Namespace:** `app.js` line 16 `window.Pinalert = (function () { 'use strict'; ... return { ... setText, showToast ... }; }());` (exports at lines 335 to 347). `votes.js` and `visibility.js` attach siblings as `window.PinalertVotes = {...}` / `window.PinalertVisibility = {...}`. New shared modules should follow suit: `window.PinalertQueue`, `window.PinalertFlag`, `window.PinalertReportForm`. Page scoped modules export nothing and guard on their root element, like `activity.js` lines 15 to 24:

```js
(function () {
  'use strict';
  var list = document.querySelector('.profile-report-list');
  if (!list) {
    return;
  }
```

`account-menu.js` (lines 12 to 30) uses the same guard on `#account-menu-trigger`. `moderate.js` and `low.js` follow this.
**Text sink rule:** `Pinalert.setText(el, value)` (`app.js` 298 to 300) is `el.textContent = value === null || value === undefined ? '' : String(value)`. Every report authored or server authored string reaches the DOM through it, with DOM calls for the 112 link (`document.createElement('a')`, `href = 'tel:' + number`). No `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write` in any file: a new test scans every `web/static/js` file with comments stripped. The only existing stripper, `stripCSSComments` (`web/css_contract_test.go`), removes `/* */` blocks only, and `map.js` line 11 mentions `innerHTML` in a `//` comment, so write a small `stripJSComments` (strip `/* */` blocks; treat `//` as a comment only at the start of a line after whitespace or after whitespace, never inside a string or a URL such as `https://`) and also reword that `map.js` comment (a Phase 4 edited file). The per file `modal.js` scan (`web/js_contract_test.go` lines 579 to 590) is a raw count that works only because modal.js avoids the words; it is not a model of comment stripping, and the raw `modal.js` and `theme.js` scans stay as they are.
**Deferred script order:** all page scripts are `defer` and run in document order, so a module that calls another must load after it: `app.js`, `votes.js`, `visibility.js`, `trust.js` (created by Phase 3 plan 03-13), then `queue.js`, `flag.js`, `report-form.js`, then `map.js`, `modal.js`, `feed.js` (they call the earlier ones). `theme.js` and `slow-notice.js` are the only non deferred scripts (head).
**Pinned counts to respect** (RESEARCH Area D "Other landmines" and Area E): `app.js` has exactly two `fetch(` and one `show_disputed`; `votes.js` exactly one `fetch(` (file wide, `TestVoteTransportHasNoLocationFallback`) and its one `getCurrentPosition(` pinned only as living inside `getVoterLocation()` after the `sessionStorage.getItem(` call (`TestVoterLocationIsCachedPerSession`); `feed.js` zero `fetch(`; `activity.js` exactly one `addEventListener`, no `fetch(`, one `Pinalert.showToast(`. So: queue sends call `Pinalert.submitReport`; 429 handling goes inside `castVote`'s existing fetch; the Activity strip's listeners live in `queue.js`, mounted by one call from `activity.js`; `low.js` voting reuses `getVoterLocation` (`votes.js` line 98, the `sessionStorage` cached lookup that never falls back to a default position, T-02-01).

### `web/static/js/app.js` (edit)

**Analog:** itself. `submitReport` lines 186 to 212: `fetch('/api/reports', {method:'POST', ...}).then(res => res.json().catch(() => ({})).then(body => { if (!res.ok) { ... throw err (with err.field, err.fieldMessage) } return body.report; }))` then `fetchReports()`. Extend the thrown error to classify queueable outcomes (no answer, timeout via `AbortSignal.timeout` accepting both `TimeoutError` and `AbortError`, 502, 503, 504) and pass `retry_after_seconds`, `serious`, the number, and `held` through. `showToast` (lines 310 to 323) is a single implementation with a hardcoded `3000`: add an optional duration parameter to this one function (`TestToastHasExactlyOneImplementation`). `fetchReports` gets the `hasLoaded` background refresh rule and a 10 s timeout (Phase 3.1 rewrites it for `view`: plan against the built file, keep exactly two `fetch(`).

### `web/static/js/modal.js` and `report-form.js` (the riskiest refactor)

**Analog:** `modal.js` (848 lines). Leaflet coupling points to cut along: `var modalMarker = null` (line 51), `placeMarker` (333, the only `L.marker(` at 337), `renderResultRow` (383), `initLocation` returns early if `typeof L === 'undefined'` (532 to 538), reset clears the marker (652), `validate()` requires `modalMarker` (739), `buildPayload()` reads `modalMarker.getLatLng()` (763), submit calls `Pinalert.submitReport(payload)` (795). Extraction rule from UI-SPEC: `report-form.js` owns category builder, severity, description, shelter fields, validation, payload, submit, discard confirmation and typed place search behind a small location interface (get current lat and lon, choose a typed result, report whether a location is set); `modal.js` keeps the Leaflet implementation (single `L.marker(`, `placeMarker(`, `modalMap.setView(`, the main form's one missing location message); `low.js` supplies the map free one. Plan it as its own pure refactor plan that merges green BEFORE Low data uses it (RESEARCH Area D). Re-point (do not delete) `TestIconGlyphsAreClassDriven` and `TestLocationSearchUsesTextSinksAndExistingPinPlacement` in `web/js_contract_test.go` (list in UI-SPEC "Tests that change"); the latter's checks for the three element ids, the `locationSearchInput.addEventListener('input'` registration after `function resetForm`, `resetSearch` and `renderResultRow` move to `report-form.js`.

### `web/static/js/votes.js`, `feed.js`, `map.js`, `visibility.js`, `activity.js` (edits)

- `votes.js` line 198 `function createVoteBlock(reportId) {` becomes `createVoteBlock(reportId, options)` with `{ resolve: false }` skipping Mark resolved, all inside the same function body (the four resolve strings, the two `controls.appendChild(` anchors and exactly two `addEventListener` stay in that one function: P4-A15). Two tests anchor on the literal header via `jsFunctionBody(t, text, "createVoteBlock(reportId)")` (`web/votes_contract_test.go` lines 19 to 33: `jsFunctionBody` finds `"function " + name` then cuts at the next `"\n  function "`, so every guard stays bounded to one function). Re point both, plus those Phase 3 plan 03-09 adds. Phase 3 plans 03-04, 03-09, 03-17, 03-18 edit `votes.js` first (retry, `createVoteBlock(reportId)`, hint); 429 handling goes in the existing `castVote` fetch.
- `feed.js` lines ~297 to 306: on `state.status === 'error'` it hides the list (`setHidden(reportListEl, true)`) although rows are kept. D-20 and P4-A19 change this to a stale line "Last updated N minutes ago" with the rows still shown. `votes_contract_test.go` and `feed_freshness_contract_test.go` read these bodies. Phase 3 plans 03-09, 03-13, 03-16, 03-17 touch `feed.js` and `map.js` first (`buildPopupCard`).
- `map.js` lines 133 to 157: `centerOnVisitor` calls `Pinalert.setCenter`, `Pinalert.fetchReports()`, `Pinalert.startPolling()` only after `getCurrentPosition` resolves, fails, or its 8000 ms timeout. `low.js` replicates this boot sequence itself, but starts from the stored last known position first (D-16) and never loads `map.js`. On the main page only that first fetch and `startPolling` come from `centerOnVisitor`: `Pinalert.fetchReports()` is also called by `feed.js` (retry button, show disputed toggle, pageshow restore), by `votes.js` after every successful vote or resolve, and by `submitReport` in `app.js`. The Low data page loads `votes.js` but not `feed.js`, so every vote there refetches through the shared store: `low.js` must subscribe (`Pinalert.subscribe`) and have called `Pinalert.setCenter` before the first vote.
- `visibility.js` exports `window.PinalertVisibility = { VISIBILITY_STATES, ..., createVisibilityTag, updateVisibilityTag }` (end of file); `displayState` gains held (only while unexpired) and removed (P4-A9). The consumer literal tests for `vis-` names change.
- `activity.js` reads `dataset.moderation`, calls the single `queue.js` mount; keep its one `addEventListener`.

### `web/static/js/queue.js` (IndexedDB queue)

**Analog:** none for IndexedDB (see end). Module shape: `votes.js` (`window.PinalertVotes`) and `getVoterLocation`'s defensive style: wrap storage in try and catch, treat an unreadable store as absent, and say why in a comment (lines 98 to 115 above). Record shape and rules are in RESEARCH Area E (`{ request_id, account_id, payload, written_at, status, attempts, next_try_at, last_error, serious }`, key path `request_id`, index on `account_id`, `account_id` read from a server rendered `data-account-id`, never the email).

### `web/static/js/flag.js`, `moderate.js`, `low.js`, `slow-notice.js`

- `flag.js`: builder returning a DOM node like `createVoteBlock`; a source test like `TestVoteClickDoesNotActivateItsRow` (in `web/votes_contract_test.go`) pins the two `stopPropagation` listeners (P4-A7); build once per card and update in place (collisions row 11).
- `moderate.js`: guard on root like `activity.js`; full report text and score words only through `Pinalert.setText`; Refresh failure line.
- `low.js`: own row function using `PinalertTrust.createTrustBlock` (Phase 3 plan 03-13), `PinalertVotes.createVoteBlock(id, { resolve: false })`, `PinalertVisibility.createVisibilityTag`; no `tabindex`, no row listener; reads `#app-shell` data attributes the way `app.js` lines 49 to 71 do (Bengaluru fallbacks).
- `slow-notice.js`: head, non deferred, about 20 lines, same placement discipline as `theme.js` (`TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` shows how head order is asserted).

### CSS

**Analog for hidden capable elements:** `feed.css` id plus `[hidden]` override with `display: none` (the "feed.css form" UI-SPEC cites) because `.empty-state` and `.error-state` declare unguarded `display: flex` in `main.css`. Every new `display` rule on an element that can be hidden carries `:not([hidden])` in its head (`TestVoteBlockHiddenGuard` walks every CSS file but inspects only selector heads containing `.vote-error`, `.vote-controls` or `.vote-btn`, and `TestAccountMenuHiddenGuard` inspects only the account menu; Phase 3 plan 03-11 adds its own test for `.vote-note` and `.resolve-confirm`, so Phase 4 must add its own walker test over its new selectors such as `.flag-control`, `.queue-strip`, `.notice-card` and the Low data and moderator classes). `chrome.css` carries one `.btn[hidden] { display: none; }`. No hex, `rgb(`, `rgba(`, `hsl(`; no `!important`; no `999px`; only four font sizes and two weights; geometry as tokens; `--disclaimer-bar-height` declared once in `main.css` `:root` plus its two media blocks. Phase 3 plans 03-11, 03-13, 03-16, 03-17 own `trust.css` and edit `main.css` first. `TestThemeToggleFloatsClearOfTheReportButton` (`web/theme_contract_test.go` line 472) and the Phase 3 popup max height test change (P4-A2 to P4-A4). The new Low data and legal CSS must contain no "light" or "lite" token (theme toggle files excluded; scan splits identifiers on hyphens, underscores and capitals so `aria-live="polite"` is not a false hit).

### Contract tests (new)

**Analog:** `web/theme_contract_test.go` (derive from `TemplatesFS` and `fs.ReadFile`, fail with an explanatory message), `web/votes_contract_test.go` (`jsFunctionBody` bounded scans; "Honest limit of this test's claim" doc comments), `web/design_rules_contract_test.go` (`templatesWithPlainPunctuationCoverage` at line 36, the agreed copy sentence const pattern at line ~100, `TestNoPillShapedControls`). Add, per CONTEXT and UI-SPEC "New tests": bar on every full page (derived), D-22 sentence equals the rendered bar, legal pages render no email, Low data page references no map library, `<img>`, raster or font and sums under the byte budget (measure raw bytes of every linked local asset, about 256 KB starting guess, RESEARCH Area D), all `static/js` files free of the four markup sinks, Go side copy test for server authored strings, forbidden bare labels in Phase 4 files only. Phase 4 Go tests that change: `TestAccessGateExemptRoutes`, `TestCastVoteCheckOrder` (Phase 3's), `TestNearbyBatchesVoteReadsOnce` neighbours.

## Shared Patterns

### Verified account in request context
**Source:** `internal/api/gate.go` (sets), `internal/api/handlers/reports.go` lines 329 to 333 and `votes.go` lines 104 to 112 (read):
```go
acc, ok := account.FromContext(r.Context())
if !ok {
	http.Error(w, "internal server error", http.StatusInternalServerError)
	return
}
```
**Apply to:** every new gated handler (flag, low page, moderator handlers). A missing account is a routing bug, so answer 500 with `http.Error(w, "internal server error", http.StatusInternalServerError)`, never a redirect; `handlers.Page` also logs (`log.Printf("handlers: Page: no verified account in request context ...")`), the JSON handlers (`NearbyReports`, `CastVote`) do not, and new API handlers follow the JSON handlers unless the plan adds a log line. Rate limit keys and replay lookups use `acc.ID`, never the session id.

### Error envelope and logging
**Source:** `handlers.writeFieldError(w, status, field, message)` and the closing lines of `CastVote`: `log.Printf("handlers: X: %v", err)` then `http.Error(w, "internal server error", 500)`; database errors never reach a client. New 4xx fields: `422` rejection names the problem, the body carries the same `{error:{field,message}}` shape plus the 429 extras.

### Closed enum plus drift guard
**Source:** `visibility.go` `Visibilities`, `ResolveReasons`, `Valid()`; swagger `enums:` tags in `reports.go` lines 43, 114, 119; `swagger_test.go`. **Apply to:** any new wire value (`withheld`, new reasons, flag reasons, moderation states). Add to the Go slice, the `Valid()` source, the swag `enums:` tag, and regenerate docs.

### Outbound client with development fallback, no live calls in tests
**Source:** `mailer.LoadMailer` plus `handlers.GeocodeSearcher` plus httptest based client tests. **Apply to:** the moderation client and its loader.

### Copy rules
**Source:** `web/design_rules_contract_test.go`. No em or en dashes anywhere (templates, JS strings, Go strings), contracted forms, the owner fixed strings verbatim (D-22 bar sentence, "Taking long? Open the low data view.", "Waiting for a text check", "Removed after review", "Text only, no map", "Low data view", "Open full version", "Last updated N minutes ago", "Written 4 hours ago").

### Test safety
**Source:** owner rules and `internal/testutil/db.go` (`NewTestDB`, when `DATABASE_URL` is set, applies every embedded migration to that database and then runs `TRUNCATE accounts, magic_link_tokens, email_cooldowns, reports, sessions, votes RESTART IDENTITY CASCADE` at the start of every test that calls it; when `DATABASE_URL` is unset it skips before connecting). Plans must tell every executor: never run the suite against `pinalert_test` or `pinalert`, use a disposable database, tests run `-p 1` in CI (`Makefile`, `.github/workflows/ci.yml`).

## No Analog Found

| File or concern | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/moderation/testdata/genuine.json`, `abusive.json` | fixture | batch | No recorded provider fixtures exist. Use RESEARCH Area A "D-07 fixture"; record scores once with a real key, commit them, never call live in tests |
| `internal/service/modworker.go` SQL claim (`FOR UPDATE SKIP LOCKED` plus lease) | worker | batch | Only analog is the `PerIP.sweepLoop` ticker; no queue or lease claim query exists in `internal/store/queries/` |
| `web/static/js/queue.js` IndexedDB layer | module | event-driven | No IndexedDB, `online` event or Web Locks code exists. Closest module shapes are `votes.js` and `getVoterLocation` (defensive storage access) |
| Compression middleware (`middleware.Compress`) | middleware | n/a | Router installs none today; use chi's built in per RESEARCH Open Question 2 |

## Landmines found while mapping

1. `ratelimit.PerIP.Middleware` falls back to `r.RemoteAddr` and `writeTooManyRequests` hardcodes `"field":"email"` and has no `Retry-After`: do not reuse it on report, vote or flag routes; leave `perip.go` byte identical (pinned by tests and `agreedRateLimitSentence`).
2. `defaultIdleWindow` is 10 minutes (`perip.go` line 45), but an entry may only be evicted once its bucket would have refilled to full anyway (burst times refill interval: 120 minutes for the other-report bucket, 25 for the serious bucket): copying the sweep unchanged evicts a drained bucket early and returns a full burst. Size the idle window per bucket (see the perkey.go entry).
3. A partial containing the text `<html` (even in a comment) is classed as a full page by `findFullPageTemplates`.
4. One template set means a shared partial executes against each page's own view model: a missing `Chrome` field is a runtime 500 on that page only. Test render every full page.
5. `TestCastVoteCheckOrder` (Phase 3 plan 03-01) pins the `CastVote` step order; the withheld refusal changes it.
6. `createVoteBlock(reportId)` literal header is anchored by at least two Phase 2 tests plus Phase 3 plan 03-09's helpers; changing the signature breaks them unless re pointed together.
7. Every Phase 3 plan that edits `reports.go`, `votes.go`, `trust.go`, `auth.go`, `votes.js`, `feed.js`, `map.js`, `trust.css`, `main.css`, `index.html.tmpl`, `page.go`, `main.go` lands first: the excerpts above will have shifted. All 20 Phase 3 plan files exist on disk now (03-16 and 03-19 were not yet committed when this was checked); plans 03-14 and 03-16 to 03-20 were still being written when the excerpts were mapped, so re-check their `files_modified` lists.

## Metadata

**Analog search scope:** `internal/` (service, api, handlers, ratelimit, mailer, geocode, store, testutil), `cmd/`, `web/` (templates, static js and css, contract tests).
**Files read:** about 30 (targeted ranges for large files). Phase 3 plans read only for `files_modified` frontmatter.
**Pattern extraction date:** 2026-10-05
