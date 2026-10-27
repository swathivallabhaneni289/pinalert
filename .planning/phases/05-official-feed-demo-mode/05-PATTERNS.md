# Phase 5: Official Feed & Demo Mode - Pattern Map

**Mapped:** 2026-10-25
**Files analyzed:** 41 (new and modified, grouped where they share one analog)
**Analogs found:** 38 / 41 (3 have no analog: embedded district data, `cmd/buildareas`, the globe JS)

**Phase 3.3 warning (re-grep at execution time).** Phase 3.3 (session pinalert-60) is landing on main now. On disk at mapping time: migration `00011_add_push.sql`, `internal/push/*`, `internal/service/push{kinds,copy,recipients,dispatch,subs,facts}.go`, `internal/store/queries/push.sql`, `web/static/js/sw.js`, and plans `03.3-01` to `03.3-15` (SUMMARY files only for 01 to 09). NOT on disk yet: the send worker (`pushworker.go`, plan `03.3-11`), the Settings Notifications rows (`03.3-14`), the five step welcome flow (`03.3-13`), `notify.js` (`03.3-06`, check), the `PATCH /api/me/alerts` kind switches (`03.3-12`). Every excerpt below marked "3.3" must be re-read after those plans merge. Run `ls internal/store/migrations` first: 00011 is the newest, expect Phase 5 to take 00012 and 00013 (never hard code; RESEARCH G8).

## File Classification

| New / Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/store/migrations/0001N_official_alerts.sql` (+ push part) | migration | CRUD | `00011_add_push.sql`, `00009_add_updates_and_alerts.sql` | exact |
| `internal/store/migrations_test.go` (`TestMigration0001N`) | test | batch | `TestMigration00011` (line 916) | exact |
| `internal/store/queries/official.sql` + regenerated `internal/store/sqlc/*` | store | CRUD | `queries/push.sql`, `sqlc.yaml` | exact |
| `internal/testutil/db.go` Truncate, helpers in `testutil/push.go` | utility | CRUD | `testutil.Truncate` (line 74), `testutil/push.go` | exact |
| `internal/official/client.go` (SACHET RSS/CAP + GDACS fetch) | service | request-response (outbound HTTP) | `internal/geocode/client.go` | role-match |
| `internal/official/client_test.go`, `live_test.go` | test | request-response | `geocode/client_test.go`, `geocode/live_test.go` | exact |
| `internal/official/cap.go` (parse), `place.go` (point in polygon), `fresh.go`, `banner.go`, `pushwords.go` | utility | transform | `internal/service/trustcopy.go`, `geo.go`, `currency.go` | role-match |
| `internal/official/refresh.go` (RefreshIfDue, ticker, lazy kick) | service | batch / event-driven | `service/pushdispatch.go` `Run`/`SweepOnce` | role-match |
| `internal/areas/*` + `internal/areas/data/districts.json.gz`, `cmd/buildareas` | utility / cmd | file-I/O, transform | none (`go:embed` of `web`, `cmd/vapidkeys` for a stdlib-only cmd) | no analog |
| `internal/service/pushkinds.go` (widen `PushKind.Valid`), official enqueue | service | event-driven | `service/pushkinds.go`, `pushdispatch.go` `criticalBranch` | exact |
| `internal/service/alertsettings.go` (`official_push_on`) | service | CRUD | itself plus 3.3 plan 12 | exact |
| `internal/demo/*` (world, seed, querier, neighbour planner) | service | CRUD in memory | `service.Querier`, `VotingQuerier` | role-match |
| `internal/api/handlers/official.go` (`/api/official/alerts`, `/pins`, refresh) | controller | request-response | `handlers/geocode.go`, `handlers/alerts.go` | role-match |
| `internal/api/handlers/demo.go` | controller | request-response | `handlers/reports.go`, `votes.go`, `alerts.go` | role-match |
| `internal/api/handlers/alerts.go` (fill the page) + `alerts.html.tmpl` | controller + template | request-response | itself (empty state) | exact |
| `internal/api/router.go` (public routes, skipSessionFor, refresh route) | route / config | request-response | router.go lines 466-535, 754-780 | exact |
| `internal/api/gate_test.go` (new exempt rows) | test | request-response | `TestAccessGateExemptRoutes` (line 319) | exact |
| `web/templates/index.html.tmpl` `#alert-banner` | template | request-response | itself (lines 65-71) | exact |
| `web/templates/login_gate.html.tmpl`, `login_ways_links.html.tmpl`, `login_globe.html.tmpl` | template | request-response | `login_gate.html.tmpl` | exact |
| `web/templates/welcome.html.tmpl` step 1 pair | template | request-response | `#welcome-continue-1` (line 67) | exact |
| `web/templates/settings.html.tmpl` row 3 | template | request-response | 3.3 plan 14 Critical row (not landed) | role-match |
| `web/templates/demo*.html.tmpl`, `demo_strip` partial | template | request-response | `alerts.html.tmpl`, `index.html.tmpl` | role-match |
| `web/static/js/official-alerts.js`, `official-banner.js`, `official-pins.js`, `demo.js`, `api-base.js` | component | request-response | `updates.js`, `settings-alerts.js`, `votes.js` | role-match |
| `web/static/js/login-globe.js` | component | streaming (WebGL) | none (maplibre 5.24.0 already loaded by the map page) | no analog |
| `web/static/js/sw.js` (allow `/alerts` tap target) | config | event-driven | `sw.js` `safeTarget` (line 35) | exact |
| `web/static/css/official.css`, `demo.css`, `login-globe.css`; edits to `feed.css`, `welcome.css`, `auth.css` | css | n/a | `feed.css` banner block, `places.css`, `auth.css` | exact |
| `web/static/icons/octagon-alert.svg`, `triangle-alert.svg` | asset | n/a | `circle-alert.svg`, `search.svg` | exact |
| `web/*_contract_test.go` additions, `web/css_contract_test.go` (`nonCategoryIcons`) | test | transform | `p31mRule`, `p16MustDecl`, `p13MustContain`, `jsFunctionBody` | exact |
| `web/official_banner_test.mjs`, `official_alerts_test.mjs`, `demo_test.mjs` | test | transform | `settings_alerts_test.mjs`, `show_menu_test.mjs` | exact |
| `.github/workflows/official-refresh.yml` | config | batch | `.github/workflows/ci.yml` (only workflow) | partial |
| `cmd/server/main.go` wiring | config | n/a | existing push wiring (3.3 plan 11, not landed) | re-grep |

## Pattern Assignments

### `internal/official/client.go` (service, outbound HTTP)

**Analog:** `internal/geocode/client.go` (read in full to line 330).

**Constant base URL, no caller supplied host** (lines 40-48), copy this rule so the poller is never an open proxy:
```go
// nominatimSearchURL is the only outbound base URL this package ever calls.
// It is a constant precisely so this proxy can never become an open proxy
// (T-07-01): no caller-supplied URL, host, or path fragment is ever read or
// forwarded.
const nominatimSearchURL = "https://nominatim.openstreetmap.org/search"
```
Phase 5 note: SACHET item links come from the feed itself, so add a host check (RESEARCH G2: off host link refused) that the geocode client never needed.

**Timeouts and client struct** (lines 50-62, 117-138): `httpClientTimeout = 5 * time.Second`, `&http.Client{Timeout: ...}`, a `rate.Limiter` from `golang.org/x/time/rate`. Use a longer timeout for CAP (RESEARCH G2 sets the numbers).

**User-Agent** (lines 158-163), build the official one the same way (RESEARCH line 224 gives `Pinalert/1.0 (+https://<host>; contact <email>)`):
```go
func (c *Client) userAgent() string {
	if c.contactEmail != "" {
		return "Pinalert/1.0 (+" + c.contactEmail + ")"
	}
	return "Pinalert/1.0 (+https://github.com/swathivallabhaneni289/pinalert)"
}
```
**Request building** (lines 190-210): `http.NewRequestWithContext`, `url.Values.Encode()` never string concatenation, `req.Header.Set("User-Agent", c.userAgent())`.

**Test seam constructors** (lines 144-149, 152-167): an exported `NewClient(contactEmail)` plus unexported `newClientWithBaseURL(baseURL, ...)` so tests drive an `httptest` server. Copy that pair for SACHET and GDACS base URLs.

**Error logging rule** (handlers/geocode.go 195-198): log only an error class, never the URL query or body: `log.Printf("handlers: Geocode: search failed (%s)", placeErrorClass(err))`.

**Tests:** `geocode/client_test.go` lines 20-48 (`TestClient_Search_SetsUserAgent`: `httptest.NewServer`, capture `r.Header.Get("User-Agent")`, assert prefix `Pinalert/` and not `Go-http-client/1.1`) and lines 50-75 (forwards only fixed params). Add 500, timeout, garbage XML, oversize body and empty 200 cases on the same shape.

**Live switch** (`geocode/live_test.go` lines 1-37): file comment documents the command, `if os.Getenv("GEOCODE_LIVE") != "1" { t.Skip(...) }`. Copy as `OFFICIAL_LIVE=1` in `internal/official/live_test.go`. Same family: `MODERATION_LIVE`.

---

### `internal/official/refresh.go` (service, batch with ticker)

**Analog:** `internal/service/pushdispatch.go` (`Run` line 208, `SweepOnce` 249, `NewPushDispatcher` 180, option funcs `WithDispatchClock` 160, `WithDispatchSweepInterval` 169). It is the in repo example of a `time.NewTicker` goroutine (line 215) with an injectable clock and a `SweepOnce(ctx)` you call directly from tests. Copy: ctx-cancelled `Run`, a `SweepOnce`-style single idempotent function (RESEARCH Pattern 1: one `RefreshIfDue` behind three triggers: cron endpoint, ticker, lazy kick), `DispatchOption` style functional options. Do not read the dispatcher's internals for logic, only its shape. Re-grep: 3.3 plan 11 adds the worker; reuse its ticker conventions once landed.

---

### `internal/store/migrations/0001N_*.sql` (migration)

**Analog:** `00011_add_push.sql` (header comment style, lines 1-12; tables 19-72; Down at 137-148).
- Header: `-- +goose Up`, then a `-- Phase N:` paragraph naming the decisions served.
- Plain `CREATE TABLE ... BIGSERIAL PRIMARY KEY`, named `CONSTRAINT x_valid CHECK (...)` for every closed list (`push_outbox_kind_valid`, `..._state_valid`).
- Foreign keys carry NO `ON DELETE` (RESEARCH G8, repo convention).
- Partial unique indexes for dedupe: `CREATE UNIQUE INDEX uq_push_outbox_first ON push_outbox (account_id, report_id) WHERE kind IN ('critical','reliable');` Copy for one outbox row per account and alert.
- `-- +goose Down`: reverse order, `DROP INDEX`, `ALTER TABLE ... DROP COLUMN`, `DROP TABLE` (lines 137-148).
- No `CONCURRENTLY` (goose runs in a transaction).
- The 00011 comment (line 54) says "Phase 5 adds official ... with their own migrations". The `push_outbox_kind_valid` CHECK lists only four kinds and `push_outbox_update_shape` ties `update_id` to `changed`; RESEARCH G7 chose a sibling outbox table, so no change to these constraints.
- Setting column: `account_alert_settings.official_push_on BOOLEAN NOT NULL DEFAULT true` and read with `COALESCE(s.official_push_on, true)` (3.3 pattern for critical, see 00011 lines about `critical_push_on`).

**Test:** copy `TestMigration00011` (`internal/store/migrations_test.go` 916-1010): `MigrationsFS.ReadFile`, locate `+goose Up`/`+goose Down`, strip comment lines then check no `CONCURRENTLY`, loop of constraint names that must be in `text[:down]`, `DROP TABLE x;` strings in `text[down:]`, then `DATABASE_URL` skip, `goose.SetBaseFS(MigrationsFS)`, `goose.Up`, and a `t.Cleanup` that re-runs `goose.Up` and deletes only its own rows. Use a disposable database, never `pinalert_test`.

### `internal/store/queries/official.sql` + sqlc
**Analog:** `queries/push.sql` header (lines 1-20: file level comment on privacy and the clock rule; `-- name: X :one/:many/:exec`; `sqlc.arg(name)::type`; time passed as `sqlc.arg(now)::timestamptz`, never `now()` in business queries; `ON CONFLICT (...) DO UPDATE`). `sqlc.yaml`: `sql_package: "pgx/v5"`, package `sqlcgen`, output `internal/store/sqlc` committed, `timestamptz` non null overrides to `time.Time`. Run `sqlc generate` and commit the output.

### `internal/testutil/db.go` (utility)
`Truncate` (line 74-81) is a single `TRUNCATE ... RESTART IDENTITY CASCADE` listing every table; add the new official tables to both the statement and the doc comment (lines 67-73). Add seed helpers in the style of `testutil/push.go`.

---

### `internal/api/router.go` (public routes, session skip)

**Public routes before the gate** (lines 466-481):
```go
r.Get("/login", handlers.LoginGate(deps.Template, auth))
...
r.Get("/disclaimer", handlers.Legal(deps.Template, auth, handlers.LegalDisclaimerPage))
r.Get("/policy", handlers.Legal(deps.Template, auth, handlers.LegalPolicyPage))
```
Register `/demo/...` here, outside `r.Group(... requireVerifiedAccount ...)` (line 527). Update the doc comment block at lines 340-370 that lists "The exact exempt set" with a dated amendment (the 3.3 plan 04 amendment at 359-370 is the template).

**Per IP limiter scoped to a route with `r.With`** (lines 522-523): copy for the Demo routes and the refresh endpoint:
```go
requestLinkLimiter := ratelimit.NewPerIP(requestLinkLimit.Every, requestLinkLimit.Burst)
r.With(requestLinkLimiter.Middleware()).Post("/api/auth/request-link", ...)
```
Never `r.Use` it router wide (comment 514-521).

**Session skip** (lines 415, 754-780). Current `skipSessionFor(paths map[string]bool, next)` is GET plus exact path only, fed by `pwaSkipSet()` (from `handlers.PWAPublicPaths`, pwa.go line 47). Phase 5 must extend this one function (method set, exact path set, the `/demo` prefix, `POST /internal/official/refresh`, the 85 `/globe/tiles/...` paths if the server proxies them). Keep the `r.URL.RawPath == ""` rule. This is a 3.3 file (plan `03.3-04`, SUMMARY `03.3-04-SUMMARY.md`): re-grep at execution time.

**Gate test** (`gate_test.go` 307-417): add rows to the `cases` table (struct `name, method, path, body`) and to `mustBeOK` for pages that must answer 200; update the "seventeen routes" comment and add a dated amendment note exactly like the 3.3 plan 04 note in lines 312-317. Add near-miss rows (`/demo/../`, `/internal/official/refresh` with GET) in the style of lines 396-412, plus a test that `POST /api/official/*` stays 401. Rows skip without `DATABASE_URL` (`newGateTestServer`).

**Gated routes** (line 644): `r.Get("/alerts", handlers.AlertsPage(deps.Template, page))` stays gated; add `/api/official/alerts` and `/api/official/pins` inside the group next to it.

---

### `internal/api/handlers/official.go`, `demo.go` (controllers)

**Analogs:** `handlers/alerts.go` (page handler), `handlers/geocode.go` lines 155-200 (JSON handler).

Page handler shape (alerts.go 14-60): `const alertsTemplateName`, a view model with `AssetVersion`, `Chrome: cfg.Chrome.withDefaults()`, `AccountID`, `Nav: navFor(NavAlerts)`; `account.FromContext` with the "should be impossible" 500; `w.Header().Set("Cache-Control", "no-store")`; `tmpl.ExecuteTemplate`; log with `log.Printf("handlers: AlertsPage: %v", err)`. The Demo pages carry no account (no `data-account-id`, UI-SPEC Hand Off point 5), so do not call `account.FromContext` there.

JSON handler shape (geocode.go): per caller budget check then `writeFieldError(w, http.StatusTooManyRequests, "q", msg)`, nil dependency degrades to a 503 message not a panic, parse then call then `log` class only. The standard error envelope is `writeFieldError`. Reuse `reportViewToResponse`, `reportToResponse` and the vote response builder from `reports.go`/`votes.go` for Demo JSON so the real JS works unchanged (RESEARCH G5, line 327). `AlertsPage` must keep the rule in its comment: never say no alert is in force unless the feed is fresh and empty (T-34-17, ALERT-04).

**Refresh endpoint:** no analog for a bearer token route. Use `crypto/subtle.ConstantTimeCompare` (RESEARCH G1). Answer 404 when `OFFICIAL_REFRESH_TOKEN` is unset (same fail closed idea as the moderator group, router.go 700-710, which answers 404 not 401).

---

### `internal/demo/*` (in memory Querier and VotingQuerier)

**Analogs:** `service.Querier` (`internal/service/report.go` 463-520) and `service.VotingQuerier` (`internal/service/trust.go` 265-273). Both are small consumer defined interfaces designed to be faked. The Demo implements them over a per visit in memory world so `NewReportService(q, opts...)` (report.go 525) and `NewVotingService(q, WithVotingClock(fn))` (trust.go 295, 288) run the real `VisibilityResolver` and `trustcopy.go` sentences. Copy the constructor and clock option style. Methods to implement include `InsertReportIdempotent`, `NearbyReports`, `CurrentVotesForReports`, `ReportVoteContext`, `ResolutionUndoCounts`, `InsertVote` and the history reads; re-read both interfaces in full at plan time (the lists above are partial, the `Querier` has more methods after line 477). The "differential test" (RESEARCH line 880) runs one scripted scenario against this world and a real scratch database.

Neighbour confirmations must stop at `service.IndependentAgreementThreshold` (visibility.go line 58) and come from distinct precision 7 cells.

---

### `internal/service/pushkinds.go` and official push (event-driven)
`PushKindOfficial` already exists as a reserved name (lines 31-35, `ReservedPushKinds` line 48). `Valid()` (50-57) loops `PushKinds`; Phase 5 widens that list and flips the pinned test in `pushkinds_test.go`. Caps constants (lines 62-80) are pinned by `TestPushCapsAreNamed`. Words: closed tables like `service/pushcopy.go` (`SeverityWord`, `AreaWords`, `BuildPushText` line 209) and must pass `push.ValidTopic`, `PushTitleMaxRunes`, the noise word test and the no dash test. Delivery: `internal/push` `Sender`/`Message{Payload, TTL, Urgency, Topic}` (sender.go 52-56); `Urgency` high or normal. Recipient reading: `service/pushrecipients.go` (`candidateCells` line 49, `filterRecipients` 100) and `HomeAreaAnchor` (alertsettings.go 56). RESEARCH line 274: test the home cell centre plus four corners. Settings default: `DefaultAlertRadiusKm`-style constant `DefaultOfficialPushOn = true` with a pin test. Silent only: no sound or vibration anywhere (D-11, MEMORY "No buzzes"), `sw.js` already builds notifications with `silent: true`.
3.3 items to re-grep: plan `03.3-11` (worker lane and its single dispatch seam, RESEARCH Open Question 5), plan `03.3-12` (PATCH `/api/me/alerts` kind switches), plan `03.3-14` (Settings rows), plan `03.3-13` (step 5, replaced on 2026-10-27 by quick 261027-t5w's pop-up after step 4; Phase 5 adds nothing to the first-time flow).

---

### `web/templates/alerts.html.tmpl` and `index.html.tmpl` banner

`alerts.html.tmpl` (read in full): head partial `{{template "app_head" .AssetVersion}}`, per page CSS links with `?v={{.AssetVersion}}`, `{{template "app_topbar" .}}`, `<main class="place-page" data-account-id="{{.AccountID}}">`, `<h1 class="place-head">`, `<section class="place-section">`, the honest empty state in `.empty-state.place-empty` (lines 30-36) which Phase 5 replaces, `{{template "app_bottombar" .}}`, deferred scripts at the end. Add `official.css` and `official-alerts.js` the same way.

`index.html.tmpl` lines 65-71: the existing hidden banner link to reshape per D-07/UI-SPEC Surface 2:
```html
<a class="alert-banner" id="alert-banner" href="/alerts" hidden>
  <span class="alert-banner__icon" aria-hidden="true"></span>
  <span class="alert-banner__text">
    <strong class="alert-banner__title" id="alert-banner-title"></strong>
    <span class="alert-banner__line" id="alert-banner-line"></span>
  </span>
</a>
```
No JS writes it yet (grep of `web/static/js` finds no `alert-banner`); CSS lives in `feed.css` with a phone override in `phone.css` (`.pane--list > .alert-banner`).

### `web/templates/login_gate.html.tmpl` and welcome

Login: the flat globe is the `<div class="globe-bg" aria-hidden="true"><div class="globe-bg__mask"></div></div>` (lines 13-15) and the links row is line 41: `<p class="login-ways__links"><a href="/disclaimer">Disclaimer</a> <a href="/policy">Policy</a></p>` inside `.login-ways`. Extract the row into `login_ways_links.html.tmpl` and the globe into `login_globe.html.tmpl` included after `.login-column` (UI-SPEC 5d). Note `login_gate.html.tmpl` is also what Phase 8 replaces (D-14); re-read before editing. `verify_outcome.html.tmpl` loses its flat globe.

Welcome step 1: `<div class="welcome__actions"><button type="button" class="btn btn--primary" id="welcome-continue-1">Continue</button></div>` (lines 66-68). Add the secondary Try the demo link beside it. Plan `03.3-13` changes this file to five steps: re-read.

---

### Web contract tests (Go)

Helpers to reuse, all in package `web`:
- `p31mRule(t, file, media, head) p16CSSRule` (`map_tools_contract_test.go` 459): merged declarations of one selector in one media prelude; fails if none.
- `p16MustDecl(t, label, rule, map[string]string)` (`labels_contract_test.go` 275): exact declaration values.
- `p13MustContain / p13MustNotContain / p13MustMatch(t, label, text, needles...)` (`trust_contract_test.go` 71-93).
- `jsFunctionBody(t, text, name)` (`votes_contract_test.go` 19): slices `function name` up to the next `\n  function `, so module level two space indent is required in the new JS.
- `nonCategoryIcons` map (`css_contract_test.go` 683): register `octagon-alert.svg` and `triangle-alert.svg` with a comment in the `search.svg` style, referenced by a test (line 807 area).
- Pattern for pinning the Try the demo link and the globe tags: see `ways_in_contract_test.go` and `legal_contract_test.go` (same login page row).
- Also add Phase 5 files to the punctuation, pill, opacity and `[hidden]` guard tests (`design_rules_contract_test.go`, `no_underline_contract_test.go`, `noise_words_contract_test.go`); and a dated amendment on `TestNoOpacityText`.
- `places_contract_test.go` 165-234: `waitingSettingsLabels` holds "Severe official alerts for my area"; remove it and invert `TestNotificationsNoFollowOrOfficialRow` (RESEARCH line 391). 3.3 plan 14 may rename these: re-grep.

### Node tests (`web/*.mjs`)
**Analog:** `web/settings_alerts_test.mjs` (header lines 1-30): a block comment listing every behaviour pinned, `node:test` + `node:assert/strict` + `fs`, loads the real script from `web/static/js` in a sandbox with a minimal element model, fake timers and a hand answered `fetch`, and the line "Run it with: node --test web/x.mjs". Same family: `show_menu_test.mjs`, `updates_test.mjs`, `reader_test.mjs`. Rule: strings reach the page through `textContent`, never `innerHTML` (also pinned by `js_sinks_contract_test.go`). CI runs all `web/*.mjs`.

### `web/static/js/sw.js`
`safeTarget(u)` (line 35) only allows `/?report=<digits>` or `/`; an official notification tapping through to `/alerts` needs a deliberate widening plus the node test `pwa_sw_test.mjs`. This is a 3.3 file (plan `03.3-04`): re-grep. Payload text goes only into notification fields (header comment).

### CI cron workflow
Only `.github/workflows/ci.yml` exists; use RESEARCH "The cron workflow (sketch)" (line 785) as the source, and mimic ci.yml's job and Go setup. No in repo scheduled workflow analog.

## Shared Patterns

### Access gate and session bypass
**Source:** `internal/api/router.go` 466-535, 754-780; `internal/api/gate_test.go` 319-417.
**Apply to:** Demo routes, refresh endpoint, any globe tile route. Public routes before the gated group, exact paths only, the bypass and the route list built from one slice (`PWAPublicPaths` precedent), tests for 200 on exempt, near misses not 200, and a `sessions` row count unchanged test (RESEARCH Pitfall 2).

### Outbound HTTP identity
**Source:** `internal/geocode/client.go` 40-62, 158-210. **Apply to:** SACHET, GDACS and any NASA tile fetch.

### Honest unknowns in copy
**Source:** `handlers/alerts.go` 14-30 comment (T-34-17), `service/trustcopy.go`. Server authors the sentence; the page renders it with `textContent`. Never "No alerts" unless fresh and empty.

### Injectable clock
**Source:** `service.WithVotingClock` (trust.go 288), `WithDispatchClock` (pushdispatch.go 160). **Apply to:** every Phase 5 service that reads time (freshness, expiry, Demo timeline).

### Per IP limiting
**Source:** `internal/ratelimit/perip.go` `NewPerIP(every, burst)` (line 66), `.Middleware()` (144), `.Allow(key)` (113). **Apply to:** Demo routes (no account), refresh endpoint, tiles. Per account budgets (`placeBudget`, router 620-632) do not fit the Demo.

### Project rules that bind every new file
No em or en dashes in prose, comments or copy; no underlined text links (real buttons); no sound or vibration; no images in the repo (icons are SVG masks, the NASA pictures are not stored without a terms check, D-13 item 21); no Python; Go only. Do not use `:8080` or `:8090`. Tests run against a disposable database, never `pinalert_test`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/areas/data/districts.json.gz`, `internal/areas/*.go` | utility | file-I/O, transform | No embedded data file exists; closest idiom is `web/embed.go` (`go:embed`) for loading, and ray casting in RESEARCH "Point in polygon" |
| `cmd/buildareas` | cmd | batch | Existing cmds (`cmd/vapidkeys`, `cmd/modrecord`, `cmd/migrate`) are the shape only; stdlib only, run by hand |
| `web/static/js/login-globe.js` | component | streaming | No WebGL JS in the repo on the sign in page; maplibre-gl 5.24.0 is loaded by the map page (see `index.html.tmpl` script tags with SRI) and sketch `009-login-globe/README.md` "For the build" items 1 to 21 is the spec |
| `internal/api/handlers/official.go` refresh bearer token | controller | request-response | No bearer token route exists; moderator 404-when-unconfigured is the nearest idea |

## Metadata

**Analog search scope:** `internal/geocode`, `internal/store` (migrations, queries, tests), `internal/service` (push, trust, visibility, alertsettings), `internal/api` (router, gate_test), `internal/api/handlers`, `internal/ratelimit`, `internal/push`, `internal/testutil`, `web/` (templates, contract tests, mjs), `.planning/phases/03.3-*` plan titles.
**Files scanned:** about 60 read or grepped.
**Pattern extraction date:** 2026-10-25
