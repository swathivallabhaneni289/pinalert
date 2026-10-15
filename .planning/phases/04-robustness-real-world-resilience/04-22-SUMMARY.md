---
phase: 04-robustness-real-world-resilience
plan: 22
subsystem: web-frontend
tags: [offline-queue, indexeddb, send-after-the-wait, held-toast, edit-and-resend, account-binding]
status: complete
requires:
  - 04-13 (showToast with a duration, the notice card, #feed-stale)
  - 04-15 (the 429 body with retry_after_seconds, serious, emergency_number)
  - 04-17 (request_id and written_at on POST /api/reports, 200 on a repeated id)
  - 04-19 (Pinalert.formatWait, lockPage, the Low data report dialog)
provides:
  - window.PinalertQueue (mount, save, flush, discard, get) in web/static/js/queue.js
  - Pinalert.submitReport errors carrying status, queueable, retryAfterSeconds, serious, emergencyNumber, and a 10 second limit
  - the Waiting to send strip on the main page and the Low data view
  - Send after the wait, the held toast, the save toasts and Edit and resend prefill in the shared report form
  - pageViewModel.AccountID rendered as data-account-id on the page shell
  - the Queue row of the notice card
key-files:
  created:
    - web/static/js/queue.js
    - web/queue_contract_test.go
  modified:
    - web/static/js/app.js
    - web/static/js/report-form.js
    - web/static/js/modal.js
    - web/static/js/feed.js
    - web/static/js/low.js
    - web/static/css/chrome.css
    - web/templates/index.html.tmpl
    - web/templates/low.html.tmpl
    - web/templates/report_form.html.tmpl
    - internal/api/handlers/page.go
requirements: [ROBUST-02]
completed: 2026-10-16
---

# Phase 4 Plan 22: Saved reports on the device, Send after the wait, and the held toast Summary

A report posted with no answer, a 10 second timeout or a 502, 503 or 504 is now saved on the device under its account in IndexedDB (memory fallback), shown in a Waiting to send strip on the main page and the Low data view, and sent again by itself on the online event, page load, a 30 second timer and Send now, never expiring and never inserted twice.

BASE: 3b45dd1b1933a7e0651df891b2a0a5a644675bec

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | ff658c5 | submitReport classification and 10 second limit, queue.js, the strip CSS, AccountID on the main page, the main page wiring and mount, 8 tests |
| 2 | 08fb1f0 | the shared form: one request id per submission, the queue path, the 429 line with the serious call link, Send after the wait, the held toast, prefill, the note, modal.js late fix guard, 5 tests |
| 3 | a391eed | the Low data wiring (strip, note slot, tag, mount, saved location sentence), the main page note slot, the notice card Queue row, 3 tests plus the Low half of TestQueuePagesWiring |

## What was built

- **app.js**: `requestTimeout(ms)` (native `AbortSignal.timeout`, else an `AbortController` with a cleared timer), `SUBMIT_TIMEOUT_MS = 10000`, `QUEUEABLE_STATUSES = [502, 503, 504]`. `submitReport` uses one `.then(onAnswer, onNoAnswer)` so a 400, 401, 422, 429 or 500 can never be rewritten as a no answer error. Errors carry `status`; a no answer, a timeout and 502, 503, 504 carry `queueable`; a 429 carries `retryAfterSeconds` (body, else `Retry-After`, else 60), `serious` and `emergencyNumber` (only when serious). A failed list refetch after a created report still resolves the report, and `held` passes through unchanged.
- **queue.js**: IndexedDB `pinalert-queue` v1, store `reports` keyed on `request_id`, index `account_id`, read with `IDBKeyRange.only`, re-checked by `belongsToAccount` on every read, render, send and resend; an in memory fallback with the honest help line. One flush per tab (Web Locks `ifAvailable`, else the `sending_until` lease), oldest first, a pass stops at the first no answer or 401, Blocked rows wait for their time, Rejected and signed out rows only retry on Send now. Exact status table, tel: link built by DOM calls, discard confirmation, focus rule, `?resend=` landing, `pinalert-queue-count` event, BroadcastChannel guard for a different account, 60 second wait refresh that touches only the wait text node.
- **report-form.js**: `newRequestId` (randomUUID, getRandomValues, Math.random fallbacks), `ensureSubmission`, `saveToQueue`, `showLimitMessage`, `saveAfterWait`, `applyPrefill`, held toast at 6000, `written_at` only from a saved row.
- **modal.js**: both geolocation callbacks return early when a place is already chosen.
- **Pages**: `data-account-id` on `#app-shell` of the main page, `#queue-strip` and `#form-note` on both pages, `queue.js` between `trust.js` and `report-form.js`, the `#notice-queue` row between `#notice-slow` and `#notice-stale`.

## Step 0 re-grep results

- Every prerequisite symbol was present: `createSeverityLabel` and `updateSeverityLabel` exports in trust.js; `showToast(message, action, durationMs)`, `formatWait`, `lockPage` in app.js; `json:"held"`, `request_id`, `written_at` in reports.go; `retry_after_seconds`, `emergency_number` in ratelimit_gate.go; `stripJSComments`; `.queue-strip` in the `TestPhase4HiddenGuard` prefix list; `.inline-note`, `.inline-error`, `.btn[hidden]`, `.btn--destructive`.
- No `p422` helper, no `PinalertQueue`, no `requestTimeout`, no `AccountID` in page.go (low.go already had it), no `queue-strip` in either template, `data-account-id` only on low.html.tmpl, `id="feed-stale"` once. `grep -c 'fetch(' app.js` was 2 and report-form.js 1 (both unchanged after). `gofmt -l web/ internal/api/handlers/` was empty before and after.
- Built `submitReport` body before the edit: one request with `.then(res => res.json().catch(...).then(body => ...))` and a refetch `.then` with no failure handler; it set only `field` and `fieldMessage`.
- The Low form title is `<h2 id="modal-title">Report an emergency</h2>` inside `div.modal-panel.report-form--low`; the shared partial has no template action beginning with a dot. 04-18 had not placed `#queue-strip` on the Low data view.
- Low position handler: `onFix` and `onFail` already leave state `typed` untouched (a late fix is only remembered, so Use my position instead appears), so no guard was added to low.js.

## RED failures

RED and GREEN share a commit per task (the owner rules call for one path limited commit per task, as 04-13 also did). Tests were written against the finished code, then exercised by behaviour proofs below. While writing, two tests failed once and were fixed in the code or the test: `TestQueueFlushTriggers` (the sort comparator lived outside `loadRows`, moved inline) and `TestQueueStripCSS` (the forced colors rule head includes the `@media` prefix, test reads it correctly).

## Clamp

The `-webkit-line-clamp: 3` shipped; `TestChromeCSSTokensOnly` accepted it, so the `max-height` fallback was not needed.

## Verification

- `go test ./... -count=1` (DATABASE_URL unset): every package ok. `go vet ./...` clean, `gofmt -l web internal cmd` empty. `node --check` passes on queue.js, app.js, report-form.js, modal.js, feed.js, low.js.
- `go test ./web/ -run TestQueue -count=1 -v`: 16 `--- PASS` (the ten pinned names plus TestQueueStripCSS, TestQueueFormCopyAndPrefill, TestQueueLateFixKeepsChosenPlace, TestQueuePagesWiring, TestQueueNoticeCardRow, TestQueueLowSavedLocation), 0 fail.
- Database sample on the disposable `p4_0422` (created by me, absent before, dropped after, confirmed gone): `DATABASE_URL=postgres://localhost:5432/p4_0422?sslmode=disable go test ./internal/... ./web/ -count=1 -p 1 -v`: 12 packages ok, 810 `--- PASS`, 0 `--- FAIL`, 1 `--- SKIP` (`TestScreeningFixture`, not database backed, waits for plan 04-27), so the database backed SKIP count is 0.
- Behaviour proofs (Node vm sandbox in the scratchpad, not committed): `proof_submit.js` runs the real `app.js` submitReport against a stubbed request: 15 checks (10 second signal, held passes through, network failure and timeout queueable with status 0, 502/503/504 queueable, 500 with body and 422 and 401 not queueable, 429 fields including the number dropped when not serious, Retry-After fallback, default 60, failed refetch still resolves the report). `proof_queue.js` runs the real `queue.js` over stub DOM and a fake IndexedDB (and again in memory mode): 48 checks (another account's row never drawn or returned, statuses and buttons per state, automatic flush skipping Rejected, Send now retrying it, Blocked text with the call link and its announcement, wait refresh keeping the link element, held toast at 6000 and row removal, 401 stopping the pass and the tab, discard confirmation order and Escape, focus to the next row then to the fallback, count event 0 after the last row leaves) and the `?resend=` landing for the same account (opens with the note, drops only `resend` from the address) and for another account (does nothing).
- Added lines contain no U+2014 or U+2013; the words light and lite appear in no new name, string or comment.

## Existing tests that read changed bodies

All stayed green unedited: `TestFetchReportsDropsOnlyResponsesOlderThanTheLastApplied` and the `fetchReports({ afterWrite: true }).then(` pin (kept verbatim), 04-13's `TestFetchReportsBackgroundRefreshAndTimeout` and `TestShowToastDuration`, 04-10's `TestReportFormOwnsTheFormLogic`, `TestReportFormPartialKeepsMainFormOrder`, `TestReportFormScriptOrder`, `TestLocationSearchUsesTextSinksAndExistingPinPlacement`, 04-19's `TestPageLockPairs`, `TestLowDataFormCopy`, `TestLowDataSearchIgnoresServerText`, 04-18's `TestLowDataAssetPolicy`, `TestNoMarkupSinksAnywhere`, `TestChromeCSSTokensOnly`, `TestPhase4HiddenGuard`.

## Deviations from Plan

1. **[Process] RED and GREEN in one commit per task**, see RED failures.
2. **[Rule 2 - small addition] `.queue-strip a.btn { text-decoration: none; }`** in chrome.css. The Sign in again control is an anchor with the `btn` class and would otherwise be underlined like a link. No `display` declared, so the `:not([hidden])` rule is not involved. `TestQueueStripCSS` still passes.
3. **[Refinement] `saveAfterWait` computes `next_try_at` from the time of the 429** (`lastLimit.at + 1000 * retryAfterSeconds`) instead of `Date.now()` at the tap, so the wait is not stretched by the seconds the person took to tap. The pinned test tokens (`next_try_at:`, `lastLimit.retryAfterSeconds`) are present.
4. **[Choice] Send now uses `aria-disabled="true"` while a pass runs** (label Sending) rather than the `disabled` attribute, so a keyboard user who pressed it keeps focus; the plain disabled look is the same (P4-A28).
5. **[Choice] `sendRow` reads the rows again before it leases a row** and uses that fresh copy, so a row another tab sent, discarded or leased is not sent from a stale copy.
6. **[Tool] `env -u DATABASE_URL` not used** (the sandbox refuses it); DATABASE_URL was unset in the shell. Long compound commands were split into plain steps.

No file outside `files_modified` was edited, and no other plan's test was edited.

## Hand off

- **04-25 (Activity):** `buildEditControl(record)` in queue.js is where the `resendHref` link variant goes (it returns `null` when `onEdit` is not a function). Activity mounts with `headingLevel: 3` and no `onEdit`, and needs `data-account-id` on `.profile-page`, `#queue-strip` and `#notice-live`. The focus rule (next row, previous row, then `focusFallback()`) and the `onEdit(record, opener, note)` arguments are already built here; 04-25 only passes `focusFallback` from activity.js and must not re-add it. The sign in link uses `/login`.
- **04-24 (Flag):** `flag.js` loads between `queue.js` and `report-form.js` on both pages.
- **04-27 (human checks, V-24):** the three `<human-check>` blocks of this plan (request blocking of `*/api/reports` for the strip, the 429 and held toast checks, the offline and two tab checks). Not run here.

## Known Stubs

None. Every control is wired. Votes, vote changes and flags are never saved (queue.js sends only through `Pinalert.submitReport`).

## Threat Flags

None beyond the plan's register: T-04-14 (account binding on every read, render, send and resend), T-04-01 (every string through `Pinalert.setText`, the tel: link built by DOM calls), T-04-05, T-04-17 and T-04-09 are mitigated as written.

## Self-Check: PASSED

queue.js, queue_contract_test.go and this summary exist; commits ff658c5, 08fb1f0 and a391eed exist on the worktree branch; the test commands above passed.
