---
phase: 4
slug: robustness-real-world-resilience
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-05
---

# Phase 4: Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source of truth for the
> behavior to test map is `04-RESEARCH.md`, section "Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (go 1.26.4). The `web` package contract tests parse the embedded templates, JS and CSS (no browser). End to end tests drive `api.NewRouter` over `httptest` against real Postgres through `testutil.NewTestDB` |
| **Config file** | none (`sqlc.yaml` for code generation; Makefile targets `test`, `test-short`) |
| **Quick run command ("quick")** | `env -u DATABASE_URL go test ./... -count=1`. The `env -u` prefix is mandatory: if the owner's shell exports a URL that points at `pinalert_test`, a bare run would truncate it. Every database backed test skips without `DATABASE_URL` (through `testutil.NewTestDB`), so this needs no database |
| **Database form ("db")** | `dropdb --if-exists p4_04NN; createdb p4_04NN && DATABASE_URL="postgres://localhost:5432/p4_04NN?sslmode=disable" go test <pkgs> -run '<regex>' -count=1 -p 1; rc=$?; dropdb --if-exists p4_04NN; exit $rc` (`NewTestDB` applies the embedded migrations). Add `-race` for the limiter and idempotency tests |
| **Full suite command** | the database form over `./...` with `-p 1` on the scratch database `p4_04full`, dropped afterwards |
| **Disposable database** | **Never `pinalert_test` or `pinalert`.** `pinalert_test` is the owner's dev server database and the suite truncates it. Start no server on ports 8080 or 8090 |
| **Estimated runtime** | quick run: target under 30 seconds; full database run: to be measured by the first plan |

Reading the command cells below: `quick ./pkg/ -run 'A|B'` means the quick run restricted to that package and
test regex, `db ./pkg/ -run 'A|B'` means the database form with it. A cell with no package means the whole
tree (`./...`). Always join test names with a plain `|` (never `\|`, which Go's RE2 reads as a literal pipe and
then runs zero tests). After any `-run`, confirm the output lists tests and does not say "no tests to run".

---

## Sampling Rate

- **After every task commit:** the quick run plus the one database form command named in the plan
- **After every plan wave:** the database form over `./internal/... ./web/` with `-p 1`
- **Phase gate:** the full database form green, then the UI-SPEC manual script (Hand Off Notes, "Manual checks (V-42 additions)") on a dev server at port 8792 with `ENV=development`
- **Before `/gsd-verify-work`:** full suite must be green
- **Max feedback latency:** 30 seconds for the quick run

---

## Per-Task Verification Map

Task ids are assigned by the planner. Each row below is a required test from the research; every plan task
must cite at least one row (or a Wave 0 dependency) in its `<verify>` block. Rows V-01 to V-24 map ROBUST-01
to ROBUST-04 and ROBUST-08, the added scope groups (held state, vote throttle, moderator screen, Flag control,
legal pages, posting and voting on the Low data view), decisions D-01 to D-24 and derived rules R-01 to R-10.

| Req / Decision | Behavior | Test type | Automated command | File exists? |
|---|---|---|---|---|
| V-01 ROBUST-03 D-01 to D-07 R-06 | `Decide`: scores map to pass, borderline or reject; violence and self harm never reject; abuse is rejected at every severity including serious; the gibberish rule counts runes; the four rejection messages | unit | `quick -run 'TestDecide|TestGibberish|TestRejectMessages'` | Wave 0 |
| V-02 ROBUST-03 D-07 | Fixture on recorded provider scores (150 to 300 reports, five languages): no genuine report rejected, no serious report withheld, the abusive set rejected | unit | `quick -run TestScreeningFixture` | Wave 0, recording the scores is a human step |
| V-03 ROBUST-03 D-01 | Provider client: request shape, only the trimmed text sent, `Retry-After` honoured, 429 and 5xx transient, other 4xx permanent, timeout (fake server, no live call) | unit | `quick ./internal/moderation/` | Wave 0 |
| V-04 ROBUST-03 D-01 to D-04 | Submit: reject stores nothing; pass is clear; borderline posts clear plus `needs_review`; a timeout leaves a non-critical report pending and in no feed; a pending critical report is published; `held` in the 201 body | e2e (DB) | `db ./internal/api/handlers/ -run TestSubmitScreening` | Wave 0 |
| V-05 ROBUST-03 D-02 R-03 | Background worker: claim, backoff with jitter, `Retry-After`, a late reject removes the report (author told), a late pass releases it, an expired report is not retried | integration (DB) | `db ./internal/service/ -run TestModerationWorker` | Wave 0 |
| V-06 R-01 to R-03 | `Resolve` rung order (removed above the critical bypass), zero value neutral, the reporter rating classifier neutral, a source guard on every `Resolve` caller (comments stripped; `reliability.go` allowlisted by name with an assertion that it keeps exactly one neutral call), `withheld` appears in no view | unit | `quick -run 'TestResolveModeration|TestWithheldInNoView|TestResolveCallersMentionModeration'` | Wave 0 |
| V-07 R-06 | Rune counting at 9, 10, 1000 and 1001 characters, English and Telugu | unit | `quick -run TestValidateSubmitInputCountsRunes` | amend `internal/service/report_test.go` |
| V-08 ROBUST-04 D-10 D-11 | `PerKey` limiter: burst, refill on an injected clock, key independence, refund (cancel at the reservation instant, with the clock advanced between Reserve and Cancel, plus an interleaved reserve, deny, cancel case), idle window per bucket never hands back a full burst early, concurrent bound | unit with `-race` | `quick ./internal/ratelimit/ -race` | Wave 0 |
| V-09 ROBUST-04 D-11 D-12 | Serious and other report buckets are separate; 429 header and body fields; a reject, a 400 and a replay do not consume allowance | e2e (DB) | `db ./internal/api/handlers/ -run TestReportRateLimit` | Wave 0 |
| V-10 ROBUST-04 D-10 | The network address backstop is skipped without a known address or header setting; the spoofed header test stays green | e2e (DB) | `db ./internal/api/ -run 'TestRequestLinkLimiterIgnoresSpoofedProxyHeaders|TestIPBackstop'` | amend |
| V-11 Vote throttle D-10 D-12 (WR-01) | The 21st vote gets 429; the re-cast cooldown gets 429 with its own message; a refused vote leaves the cooldown intact; all four vote routes; swagger lists the 429s | e2e (DB) | `db ./internal/api/handlers/ -run 'TestVoteRateLimit|TestSwaggerSpecCoversRoutes'` | `TestVoteRateLimit` Wave 0; `TestSwaggerSpecCoversRoutes` exists (`internal/api/handlers/swagger_test.go`, needs no database) and is amended to require the 429 responses |
| V-12 ROBUST-02 D-19 D-20 R-05 | Replay: a second send returns 200 and no second row; two concurrent sends make one row; another account with the same request id gets its own row; `written_at` is clamped and never used for expiry; the label is sent only past 30 minutes | e2e (DB) with `-race` | `db ./internal/api/handlers/ -race -run 'TestSubmitIdempotent|TestWrittenAt'` | Wave 0 |
| V-13 ROBUST-02 D-18 to D-20 | `queue.js` source: no markup sinks, no age discard, bound to the account, goes through `Pinalert.submitReport`, a timeout with a fallback, flushes on the `online` event and on a timer | contract | `quick ./web/ -run TestQueue` | Wave 0 |
| V-14 ROBUST-02 D-19 D-20 | Flush order, never discard, a wrong account never sends (hand written fake IndexedDB) | node, optional | `node --test web/queue_logic_test.mjs` | only if the owner accepts a Node test runner (research Open Question 6) |
| V-15 ROBUST-01 D-13 D-14 D-17 | `/low` answers 200 for a verified account and 302 to `/login?low=1` logged out; the `/` cookie redirect, `/?full=1` and the redirect loop cases | e2e (DB) | `db ./internal/api/ -run TestLowDataRoutes` | Wave 0 |
| V-16 ROBUST-01 D-14 R-09 | Asset policy (no map library, no `<img>`, no raster file, no web font, no unpkg host), the raw byte budget, banned view words, hidden guards (a new walker test over the Phase 4 selectors, because `TestVoteBlockHiddenGuard` covers only `.vote-*` heads), the two track vote grid, no row `tabindex`, its own fetch boot | contract | `quick ./web/ -run 'TestLowData|TestFeedRenderKeepsRowsOnError'` | Wave 0 |
| V-17 ROBUST-08 D-21 D-22 R-08 | The bar appears once per full page, first in `<body>` inside `.top-chrome`; the D-22 sentence is exact; empty number lists drop their segments; the legal pages carry no email or account header; the contact block is absent when unset; `chrome.css` is linked; dash free | contract and render | `quick ./web/ -run 'TestDisclaimerBar|TestLegalPages|TestUserVisibleCopyUsesPlainPunctuation'` | Wave 0 for `TestDisclaimerBar` and `TestLegalPages`; `TestUserVisibleCopyUsesPlainPunctuation` exists (`web/design_rules_contract_test.go`) and walks every embedded template, so the new templates are covered automatically (the regex also runs its `...AuthJS` sibling) |
| V-18 ROBUST-08 D-23 R-07 | `/disclaimer` and `/policy` are reachable logged out with `Cache-Control: private`; the exempt route list test and the router doc comment are updated | e2e (DB) | `db ./internal/api/ -run TestAccessGateExemptRoutes` | amend |
| V-19 Moderator screen D-08 | 404 with the same status, body, `Content-Type` and no `Allow` header as an unknown route for logged out, unverified and unlisted visitors on the registered method AND on wrong methods (HEAD, PUT, GET on the POST routes, POST on `/moderate`), page and API; a listed moderator sees the queues; Keep, Remove and the audit row; Remove by id | e2e (DB) | `db ./internal/api/handlers/ -run TestModerator` | Wave 0 |
| V-20 Flag control D-09 | One flag per account (409), own report 403, withheld report 404, the flag limiter, never changes `Resolve`, the viewer marker, constant round trips | e2e (DB) | `db ./internal/api/handlers/ -run TestFlag` and `quick ./internal/service/ -run TestNearbyBatchesVoteReadsOnce` (no database needed) | `TestFlag` Wave 0; `TestNearbyBatchesVoteReadsOnce` exists (`internal/service/feed_test.go`) and is amended for `ReportExtrasForReports` |
| V-21 Low data post and vote D-15 D-16 R-09 | `createVoteBlock(id, { resolve: false })` gives two tracks; the shared trust block; the location interface of `report-form.js` | contract | `quick ./web/ -run 'TestLowDataVote|TestReportFormLocationInterface'` | Wave 0 |
| V-22 Cross cutting | A Go copy test for every server authored message (four rejections, seven rate limit messages, the 429 fields), plain characters only; one sink scan over every `web/static/js` file for the four markup parsing sinks, with a `stripJSComments` helper that handles `//` line comments (the existing `stripCSSComments` does not, and `map.js` line 11 mentions `innerHTML` in a comment) | unit and contract | `quick -run 'TestServerCopy|TestNoMarkupSinksAnywhere'` | Wave 0 |
| V-23 Migrations | New files are embedded; up and down on a scratch database; `testutil.Truncate` lists the new tables | integration (DB) | `db ./internal/store/ -run 'TestMigrationsEmbedded|TestMigrationsUpDown'` | `TestMigrationsEmbedded` exists (`internal/store/migrations_test.go`, no database, checks only migration 00001) and is amended; `TestMigrationsUpDown` and a `Truncate` table list check are new (Wave 0) |
| V-24 all | The UI-SPEC manual checks ("Manual checks (V-42 additions)" in Hand Off Notes): bar geometry, account menu reach, notice card, Low data network panel, position states, poll failure and the stale line, offline save and resend, the Flag dialog by keyboard and screen reader, the moderator screen on a phone, `unpkg.com` blocked | manual UAT | script in the phase UAT file, dev server on port 8792 with `ENV=development` | manual |

*Status for every row starts as pending and is updated by the executor and verifier.*

---

## Wave 0 Requirements

- [ ] Disposable database procedure written into every plan's verify commands (never `pinalert_test` or `pinalert`; the quick run uses the `env -u DATABASE_URL` prefix)
- [ ] `internal/ratelimit/perkey.go` and its test
- [ ] Migration, sqlc queries (`ReportExtrasForReports` and the moderation queries), `testutil.Truncate` extended, and the feed and auth Querier fakes extended (the voting fake only updates `ReportVoteContext`)
- [ ] `screening.go`, the fixture JSON (150 to 300 entries, five languages), the recorder command, and the recorded scores (a human step with a fresh zero balance OpenAI key)
- [ ] `internal/moderation` client and an `httptest` fake provider
- [ ] `Deps` knobs (generous budgets, an injected clock, a fake screener) in `newE2EServer` and `newGateTestServer`
- [ ] Optional `web/queue_logic_test.mjs` (only if the owner accepts a Node test runner)
- [ ] One source scan over every `static/js` file for the four markup sinks

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Record the provider scores for the screening fixture | ROBUST-03 (D-07) | Needs the owner's OpenAI key; tests never call the live API | Run the recorder with a fresh zero balance key against the pinned snapshot `omni-moderation-2024-09-26`, commit the recorded scores, then run `TestScreeningFixture`. This also shows whether a zero balance account is refused with `429 insufficient_quota` |
| Emergency numbers on the live government pages | ROBUST-08 (D-22) | Official pages cannot be asserted from tests (112.gov.in could not be opened in either earlier check) | Check 112, the older numbers 100, 101 and 108, and the state helpline wording for 1070 and 1077 on the live pages before any promotion. Change only the one numbers setting |
| Lawyer or mentor read of the Disclaimer and Policy pages and the retention statement | ROBUST-08 (D-23) | Legal text, research confidence is LOW | Have the two pages read before any wide promotion |
| Visitor address header on the real host | ROBUST-04 (D-10) | Depends on the undecided host | Send a canary request through the host and confirm which header carries the visitor address before setting `CLIENT_IP_HEADER`. An empty setting means no network limit on the new limiters |
| Rendered browser behavior of the bar, Low data view, queue strip, Flag dialog and moderator screen | ROBUST-01, ROBUST-02, ROBUST-08, Flag, Moderator | Layout, focus, assistive technology and real network behavior cannot be asserted from Go | Follow the UI-SPEC "Manual checks (V-42 additions)" list on a disposable database with fresh accounts, dev server on port 8792 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency under 30 seconds for the quick run
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
