---
phase: 8
slug: report-page-and-comments
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-25
---

# Phase 8: Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Built from `08-RESEARCH.md` "Validation Architecture", its "OpenAPI and contract tests" list and its common pitfalls.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) with a real Postgres for DB tests, `node:test` with `vm` sandboxes for browser scripts, Go contract tests that read the embedded `web/static` and `web/templates` trees, manual iPhone simulator, Mac Safari and real phone checks |
| **Config file** | none. Postgres comes from `DATABASE_URL` (tests skip without it) and `testutil.NewTestDB` applies the embedded migrations. CI (`.github/workflows/ci.yml`) runs `go build ./...`, `go vet ./...`, `node --test web/*.mjs`, then `go test -v -p 1 ./...` against Postgres 16 |
| **Quick run command** | `env -u DATABASE_URL go test ./internal/service/ ./web/ -short -count=1 && node --test web/*.mjs` |
| **Full suite command** | `DATABASE_URL=postgres://localhost:5432/p8_<plan>_a?sslmode=disable go test ./... -p 1 -count=1`, then `node --test web/*.mjs`. The database is a disposable one, created for the run (refuse if the name exists) and dropped after |
| **Estimated runtime** | about 30 seconds quick, about 6 minutes full (an estimate at the size of the Phase 5 suite, recheck after Wave 0) |

Never run against `pinalert_test`, `pinalert`, `p31_4_uat` or `p34_uat`. Give every subagent its own database name and tell it so.

---

## Sampling Rate

- **After every task commit:** run the quick run command for the packages the task touched, plus the single `node --test web/<file>.mjs` the task touches. A task that touches a DB backed package also runs that package's tests with `DATABASE_URL` set to the plan's own `p8_<plan>_a` database.
- **After every plan wave:** run `go vet ./...`, `node --test web/*.mjs`, and `go test ./... -p 1` on the plan's own scratch database (owner recipe: per plan databases, own merge gate). A merge that touches `internal/store/queries` or a migration re-runs `sqlc generate` first, because two plans in one wave both regenerate `internal/store/sqlc/*.go`.
- **Before `/gsd-verify-work`:** the full suite must be green on a fresh scratch database. Then run the Linux CI check recipe from owner memory, then the owner's viewing (phone in the iPhone simulator, laptop in Mac Safari only, no iPad), the one real phone check and the WhatsApp preview check.
- **Max feedback latency:** 60 seconds for the quick run.
- **Ship order is the sampling order (D-17).** The page, the timeline and Share are verified first, then comments, which need the moderation hook and the limiter.

---

## Per-Task Verification Map

Filled by the planner once the plans exist. Requirement level map from research. The Automated Command column holds a label; the commands are in the block below the table.

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| PAGE-01, SC1 | A report opens to its page with the same trust block and a way back; the trust block is the popup's own nodes, not a copy; Flag and Stop updates stay below the page node after a resize | node + contract | V1 | partial: `report_view_test.mjs` exists, the rest Wave 0 | pending |
| PAGE-01, SC2 | Timeline replays the vote log, one event per milestone, no names or cells; wording is closed copy with plain punctuation; counts equal the trust block counts at every step | unit | V2 | Wave 0 | pending |
| PAGE-01, SC2 | Timeline response omits account, session and cell keys; a withheld report is 404 | e2e (DB) | V3 | Wave 0 | pending |
| COORD-08, SC5 | Share returns a token link; the public page shows only allowed fields; no comments, names, description or coordinates | e2e (DB) | V4 | Wave 0 | pending |
| COORD-08, SC5 | The public page needs no account: exempt row answers 200, near misses (`/r`, `/r/`, `/r/x/y`, upper case, a bad token, encoded paths, other methods) do not | e2e (DB) | V5 | exists, extend | pending |
| COORD-08, SC5 | The public page sets no cookie and writes no sessions row, with and without a user agent | e2e (DB) | V6 | Wave 0 (copy `TestPWAAssetsSetNoCookie`) | pending |
| COORD-08, SC5 | Open Graph tags carry no count and no "verified"; `og:url` is absolute from config, never from `Host` | unit + e2e | V7 | Wave 0 | pending |
| COORD-08, SC5 | A numeric id is not a valid public route; an unknown token is 404; removed says removed; held says unavailable | e2e (DB) | V8 | Wave 0 | pending |
| COORD-08 | Share window shows the title and the fixed line, no count; Copy link; `navigator.share` is called from the second tap | node | V9 | Wave 0 | pending |
| PAGE-02, SC3 | A clear comment posts, a borderline one posts and is listed, a rejected one keeps the box, an unfinished check holds it; short replies ("Thanks", "ok", a lone emoji, short Hindi and Telugu replies) are accepted; 280 code point limit counted in runes (a 4 byte emoji and a Telugu word), control and format characters refused | unit + e2e (DB) | V10 | Wave 0 | pending |
| PAGE-02, SC3 | Per account limits and messages for comment and like; 429 shape; every bucket exists in all four places and `RateLimitsDefault` equals the `cmd/server/main.go` constants | e2e (DB) | V11 | Wave 0 | pending |
| PAGE-02, SC3 | No markup sink anywhere in the new scripts; comment text is never rendered through a sink; a mention is a strong element | contract + node | V12 | contract exists and covers new files, node Wave 0 | pending |
| PAGE-02, D-14 | Delete own; a placeholder when replies exist; the reporter cannot delete others; a flag never hides | unit + e2e (DB) | V13 | Wave 0 | pending |
| PAGE-02, D-15 | Comments close after resolve or expiry; delete and flag still allowed | unit | V14 | Wave 0 | pending |
| PAGE-02, D-02 | One like per account; likes never touch trust, currency, reliability or visibility | unit | V15 | Wave 0 | pending |
| PAGE-02, D-02 | Page state survives a vote or a poll: the open thread, the liked state and the composer text and focus | node | V16 | Wave 0 | pending |
| PAGE-02, D-12 | Comment worker retries, backs off, stops at report expiry, never double screens | unit | V17 | Wave 0 (mirror `modworker_test.go`) | pending |
| PAGE-02, D-14 | Moderator Comments list, Keep and Remove; a non moderator gets the byte equal 404 | e2e (DB) | V18 | Wave 0 | pending |
| PAGE-02, D-16 | A comment and a reply write an update for the right recipient only; mute, self and deleted are respected; likes write none | unit + e2e (DB) | V19 | Wave 0 | pending |
| PAGE-02, D-16 | The commenter is in the follow set in both queries (`FollowedOpenReports`, `FollowersWithChangesPush`) | e2e (DB) | V20 | Wave 0 | pending |
| PAGE-02, D-16 | Push words for the comment kinds carry no comment text and no username; the worker still holds the check | unit | V21 | Wave 0 | pending |
| PAGE-02, SC4, D-03 | `comment_count` is a real count (visible comments only), absent before comments ship; the popup footer renders the server count and the zero and one forms | unit + e2e (DB) + node + contract | V22 | Wave 0 | pending |
| PAGE-01, PAGE-02, COORD-08 | Amended contract pins: popup footer ban lifted with a dated note, size only entries, 44px targets, flag dialog mode, script order, plain punctuation, icons registered | contract + node | V23 | amended | pending |
| API docs | New routes documented with 401, 404, 409 and 429 where they apply | e2e | V24 | exists, extend | pending |
| PAGE-02 | Account deletion blanks comments, removes likes given and comment flags | e2e (DB) | V25 | Wave 0 | pending |
| Docs | `docs/TRUST-MODEL.md` stays true and cites its tests (comments and likes never feed any trust number) | unit | V26 | exists, extend | pending |

*Status: pending, green, red, flaky*

Commands for the table, one per row. They are written as plain text because a pipe inside a table cell needs escaping, and a copied backslash pipe is a literal pipe in a Go regular expression and matches no test. Commands marked DB need `DATABASE_URL` set to the plan's own scratch database; a skipped DB test is not a pass.

```
# V1
node --test web/report_view_test.mjs web/report_page_test.mjs
go test ./web -run 'TestReportPage|TestPopupCardSurfaceAndGeometry|TestReportPageReusesTrustBlock' -count=1
# V2
go test ./internal/service -run 'TestBuildTimeline|TestTimelineWording|TestTimelineMatchesTrust' -count=1
# V3 (DB)
go test ./internal/api/handlers -run TestTimeline -p 1 -count=1
# V4 (DB)
go test ./internal/api/handlers -run TestSharePage -p 1 -count=1
# V5 (DB)
go test ./internal/api -run TestAccessGateExemptRoutes -p 1 -count=1
# V6 (DB)
go test ./internal/api -run TestSharePageSetsNoCookie -p 1 -count=1
# V7 (DB)
go test ./internal/api/handlers -run TestShareOpenGraph -p 1 -count=1
# V8 (DB)
go test ./internal/api/handlers -run TestSharePageStates -p 1 -count=1
# V9
node --test web/share_window_test.mjs
# V10 (DB)
go test ./internal/service -run 'TestComment|TestCommentShortReplies|TestCleanCommentBody' -count=1
go test ./internal/api/handlers -run TestCommentScreening -p 1 -count=1
# V11 (DB)
go test ./internal/api/handlers -run 'TestCommentRateLimit|TestLikeRateLimit' -p 1 -count=1
go test ./internal/api/ ./cmd/server/ -p 1 -count=1
# V12
go test ./web -run TestNoMarkupSinksAnywhere -count=1
node --test web/comments_test.mjs
# V13 (DB)
go test ./internal/service ./internal/api/handlers -run 'TestCommentDelete|TestCommentFlag' -p 1 -count=1
# V14
go test ./internal/service -run TestCommentsClosed -count=1
# V15
go test ./internal/service -run 'TestLikeOnePerAccount|TestLikesNeverFeedTrust' -count=1
# V16
node --test web/report_page_test.mjs
# V17
go test ./internal/service -run TestCommentWorker -count=1
# V18 (DB)
go test ./internal/api/handlers -run TestModerateComments -p 1 -count=1
# V19 (DB)
go test ./internal/service -run 'TestCommentUpdates|TestLikesNotify' -count=1
# V20 (DB)
go test ./internal/service -run TestCommenterFollowsReport -count=1
# V21
go test ./internal/service -run 'TestPushCommentWords|TestEventStillHoldsComment' -count=1
# V22 (DB)
go test ./internal/service -run TestCommentCountExtras -count=1
node --test web/popup_footer_test.mjs
go test ./web -run TestPopupFooter -count=1
# V23
go test ./web/ -short -count=1
node --test web/flag_dialog_test.mjs
# V24 (DB)
go test ./internal/api/handlers -run TestSwaggerSpecCoversRoutes -p 1 -count=1
# V25 (DB)
go test ./internal/service -run TestDeleteAccountComments -count=1
# V26
go test ./internal/service -run TestTrustModelDocMatchesCode -count=1
```

**Executors treat "no tests to run" as a failure.** `go test -run` exits 0 when the pattern matches nothing, so a typo or a test not yet written passes silently. Run each Wave 0 command once before the code exists and see it fail for a missing test, then pass after.

---

## Wave 0 Requirements

- [ ] `internal/service/timeline_test.go`, `internal/service/timelinecopy_test.go`: events, milestones, take backs, closed copy
- [ ] `internal/service/comments_test.go`, `commentworker_test.go`, `commentupdates_test.go`, `commentlikes_test.go`
- [ ] `internal/api/handlers/timeline_e2e_test.go`, `share_e2e_test.go`, `comments_e2e_test.go`, `moderate_comments_e2e_test.go`
- [ ] `internal/api/share_routes_test.go`: no cookie, no sessions row, near misses; extend `gate_test.go`
- [ ] `web/report_page_test.mjs`, `web/comments_test.mjs`, `web/share_window_test.mjs`, `web/popup_footer_test.mjs`
- [ ] `web/report_page_contract_test.go`, `web/comments_contract_test.go`, `web/share_contract_test.go`; amended entries, with dated notes and not rewrites, in `popup_contract_test.go` (footer ban A11), `layout_rules_contract_test.go` (size only entries, touch targets, the owner's 2026-10-19 sentence if a B build ships), `flag_contract_test.go` and `flag_dialog_test.mjs` (comment mode), `template_contract_test.go` (script order)
- [ ] A test helper in `internal/testutil` to seed comments, likes and a vote log, and the new tables added to the explicit list in `Truncate`
- [ ] The new rate limit fields added to the p20 style helpers (`p20Tweak`, `p20Open`) so frozen clock tests find every bucket open, and `ip_backstop_test.go` checked for the test that pins `cmd/server/main.go` against the defaults
- [ ] `/r` added to the session bypass: Phase 5's prefix aware `sessionSkip` if it has landed, otherwise the small prefix list fallback in the same style; the near miss rows and the no cookie test pin either shape
- [ ] Phase 7.1 outputs re-grepped before comments work starts (username column and batch read, deletion frees the username, the reserved word function, `requireOnboarded` still passes `/api/`, the exempt list); the plan fails if they are missing
- [ ] Framework install: none

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The owner's viewing session: the laptop host (B, the report beside the map, against a wider popup, Open Question 1), the shipped phone order against D-01 (deviation 1b), the footer count as the "Comments N" heading (Q2), the removed and unavailable wording (Q3, Q7), who may comment (Q5), the limiter numbers, timeline density, the Settings switch wording, and a hidden finished Rescue thread | PAGE-01, D-01, D-02, D-06, D-13 | Plan 08-05 is a checkpoint: the owner decides layout by looking, and said "the laptop ver looks good i dont want you to disturb that" on 2026-10-19 | One batched session on a scratch test copy (own port, never 8080 or 8090, scratch database). Show B and the fallback side by side. Phone in the iPhone simulator, laptop in Mac Safari only, no iPad. Show it, do not ask in words |
| The report page, timeline, folded replies, composer after the last comment, thumbs up, Share bottom sheet and Flag below the page, at phone width | PAGE-01, PAGE-02, SC1, D-02 | Playwright WebKit passed layouts that iPhone Safari showed broken | iPhone simulator through `safaridriver` and `simctl` screenshots, plus Mac Safari for the laptop. The comment box is pinned to the bottom of the panel on a laptop |
| Typing in the comment box on the laptop does not move the map or close the popup (arrow keys, plus, minus, Escape) | PAGE-02, Pitfall 9 | Real keyboard focus inside Leaflet is not reproduced by a contract test | Mac Safari on the laptop host, once with the docked panel and once with the wider popup if that is what ships |
| Share opens the system share sheet from the second tap and Copy link works; a reply to your comment shows in the bell and arrives as a silent phone note | COORD-08, D-04, D-16 | `navigator.share` needs a real user gesture on a real phone, and a push needs a real installed phone | Owner's phone, after Phase 3.3 is live. No sound and no vibration |
| The WhatsApp preview: paste the shared link into a WhatsApp chat and wait up to 10 seconds; the preview shows the title ("{Category} report near {area}") and "Open to see if this is still true right now", with no count and no "verified"; the page opens without signing in and "Sign in to see more" goes to `/login` | COORD-08, D-04, D-05, D-06, SC5 | WhatsApp's crawler is outside our control, Meta says previews are best effort and may fall back to a small one, and the URL must be public HTTPS built from `Deps.PublicBaseURL` | Deployed copy (or a tunnel the owner trusts), a message to self in WhatsApp. Measure `/icons/app-512.png` as served: `og:image` stays only if it is under 600 KB, otherwise it is omitted. The `wa.me/?text=` button is optional and dropped if it misbehaves |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency under 60 seconds
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
