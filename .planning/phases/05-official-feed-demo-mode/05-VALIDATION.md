---
phase: 5
slug: official-feed-demo-mode
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-25
---

# Phase 5: Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Built from `05-RESEARCH.md` "Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (`internal/...` packages and the `web` contract tests), `node:test` for `web/*.mjs`, manual and Playwright browser checks |
| **Config file** | none. CI runs `go build ./...`, `go vet ./...`, `node --test web/*.mjs`, then `go test -v -p 1 ./...` against Postgres 16 |
| **Quick run command** | `env -u DATABASE_URL go test ./internal/official/... ./internal/areas/... ./internal/demo/... ./web/ -short -count=1 && node --test web/*.mjs` |
| **Full suite command** | `go test ./... -p 1 -count=1` with `DATABASE_URL` pointing at a disposable `p5_<plan>_a` database only |
| **Estimated runtime** | about 30 seconds quick, about 6 minutes full |

Never run against `pinalert_test`, `pinalert`, `p31_4_uat` or `p34_uat`. Give every subagent its own database name and tell it so.

---

## Sampling Rate

- **After every task commit:** run the quick run command for the packages the task touched.
- **After every plan wave:** run `go vet ./...`, `node --test web/*.mjs`, and the end to end set on the plan's own scratch database.
- **Before `/gsd-verify-work`:** the full suite must be green on a scratch database. Then run the browser checks and the one real phone check.
- **Max feedback latency:** 60 seconds for the quick run.

---

## Per-Task Verification Map

Filled by the planner once the plans exist. Requirement level map from research:

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| ROBUST-05 | GDACS pins from their own table, unaffected by votes, `reports` untouched | e2e + unit | `go test ./internal/official/ -run Gdacs -count=1` | Wave 0 | pending |
| ROBUST-06 | Authority badge and pin distinct from crowd and Demo | web contract + browser | `go test ./web/ -run OfficialPin -count=1` | Wave 0 | pending |
| ROBUST-07 | Demo populated, confirm and dispute work, neighbours reach Confirmed, writes nothing | unit + e2e + browser | `go test ./internal/demo/ ./internal/api/ -run Demo -count=1` | Wave 0 | pending |
| ALERT-01 | District coverage from SACHET | unit + e2e | `go test ./internal/official/ -run 'Place\|Cover' -count=1` | Wave 0 | pending |
| ALERT-02 | Issuer, issued, until, removal on expiry | unit + web contract | `go test ./internal/official/ ./web/ -run 'Expiry\|AlertsPage' -count=1` | Wave 0 | pending |
| ALERT-03 | Banner only for Severe or Extreme covering the reader | unit + node | `go test ./internal/official/ -run Banner -count=1 && node --test web/official_banner_test.mjs` | Wave 0 | pending |
| ALERT-04 | Last checked, stale and unavailable honesty | unit + e2e | `go test ./internal/official/ ./internal/api/ -run 'Fresh\|Honest' -count=1` | Wave 0 | pending |

---

## Wave 0 Requirements

- [ ] `internal/official/testdata/`: trimmed real CAP samples (AP SDMA lightning, IMD Chennai with 23 codes, the Maharashtra Update with `references`, CWC flood), an RSS head of 5 items and a GDACS SEARCH sample, text files only
- [ ] Fake SACHET and GDACS servers (`httptest`) with ETag, failure, timeout and oversize modes
- [ ] `internal/areas` fixture: a tiny three district file, plus a test that loads the real embedded file once
- [ ] `internal/demo` parity harness: one scripted scenario runnable against both the in memory world and a real scratch database
- [ ] `internal/testutil`: the new tables added to `Truncate`, and helpers to seed a feed status, an alert, and an account with a home cell
- [ ] `web/` fixtures for the banner, alerts client, globe loader and demo client node tests
- [ ] The 3.3 push tests that pin the closed kind list and the settings rows are amended with dated notes, not rewritten

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The globe draws and turns with India first, no layout shift, still under reduce motion, zero requests in Low data view | Success criterion 5 | WebGL drawing and motion are judged by eye | Mac Safari and the iPhone simulator; network panel empty in Low data view |
| Banner and Official alerts at phone width | ALERT-03, ALERT-04 | Playwright WebKit passed layouts iPhone Safari showed broken | iPhone simulator through `safaridriver` and `simctl` screenshots |
| The Demo flow from the sign in link and from step 1: post, watch the count climb to Confirmed, dispute, the Too far message, Exit | ROBUST-07 | End to end feel and timing | iPhone simulator and Mac Safari |
| GDACS pin distinct from crowd and Demo pins | ROBUST-06 | Visual distinction | Mac Safari and the iPhone simulator |
| An installed phone receives a silent official notification, and the tap opens `/alerts` | D-11 | Needs a real installed phone | Owner's phone, after Phase 3.3 is live |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency under 60 seconds
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
