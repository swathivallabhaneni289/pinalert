---
phase: 04-robustness-real-world-resilience
plan: 25
subsystem: web-frontend
tags: [offline-queue, activity-page, edit-and-resend, account-binding, node-test]
status: complete
requires:
  - 04-21 (Activity back link follows the device default)
  - 04-22 (queue.js, the strip, pageViewModel.AccountID, focusFallback option)
provides:
  - profileViewModel.AccountID rendered as data-account-id on .profile-page
  - the hidden #queue-strip, body level #notice-live and the queue.js tag on the Activity page
  - the resendHref mount option in queue.js (safeResendHref, createResendLink)
  - activity.js mount, resendHref(record) and queueFocusFallback()
  - web/queue_logic_test.mjs (node --test, not in CI)
key-files:
  created:
    - internal/api/handlers/activity_queue_e2e_test.go
    - web/queue_activity_contract_test.go
    - web/queue_logic_test.mjs
  modified:
    - internal/api/handlers/auth.go
    - web/templates/profile.html.tmpl
    - web/static/js/queue.js
    - web/static/js/activity.js
requirements: [ROBUST-02]
completed: 2026-10-16
---

# Phase 4 Plan 25: Saved reports on the Activity page and the Edit and resend link Summary

The Activity page now shows the author's own waiting reports (h3 "Waiting to send" strip with Send now and Discard report) bound to the verified account id, and a rejected row's Edit and resend is a safe link to `/low?resend=ID` or `/?resend=ID`.

BASE: 12b848f4ccd7cdf0c944e57ffeb48cc91d9de8ba

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | 5846244 | AccountID on profileViewModel and `data-account-id` on `.profile-page`, hidden `#queue-strip` under the Activity heading, body level `#notice-live`, `queue.js` tag, one mount call in activity.js, two database tests |
| 2 | b4ece5b | `resendHref` option, `safeResendHref`, `createResendLink` in queue.js; `resendHref(record)` and `queueFocusFallback()` in activity.js; `TestQueueActivityStrip` and `TestQueueEditAndResendLink` |
| 3 | 5ffacc5 | `web/queue_logic_test.mjs`, four cases on the real queue.js |

## Census (Task 1 step 0)

- (a) `data-account-id="{{.AccountID}}"` on `#app-shell`; the field is `AccountID int64` on `pageViewModel`, filled from `acc.ID`. Reused by name on `profileViewModel`.
- (b) `<div class="queue-strip" id="queue-strip" hidden></div>`, no children and no hard coded heading; `queue.js` builds the heading (`h3` when `headingLevel` is 3). Copied byte for byte.
- (c) The heading rule in chrome.css is `.queue-strip__heading`, selected by class and not by `h2`, so the h3 is styled the same. chrome.css not edited.
- (d) Mount options `strip`, `accountId`, `headingLevel`, `onEdit`, `focusFallback`. `feed.js` and `low.js` pass `getAttribute('data-account-id')` (a string); `mount` does `String(...)` and requires digits. Activity passes `dataset.accountId`. `queue.js` never names the report form module.
- (e) `moveFocusAfterRemoval(index)`: next row, previous row, then `focusFallback()` last. Its row selector is `'button, a'`, which already admits a link, so nothing was widened and the test makes no queue.js focus assertion.
- (f) Label constant `EDIT_LABEL = 'Edit and resend'`, literal count in comment stripped code: 1. The Sign in again link has class `btn`; the new link reuses it. `flush` orders by `written_at` oldest first, then by request id. Statuses: waiting, no_answer, rejected, blocked, signed_out.
- (g) Back link in profile.html.tmpl: one `profile-back-link`, `href="{{if .LowDataDefault}}/low{{else}}/{{end}}"` rendered by the server.
- (h) activity.js before: one `addEventListener`, one `Pinalert.showToast(`, one `PinalertTrust.focusToggle(`, the `var list = document.querySelector('.profile-report-list');` line once. After: same counts.
- (i) Template order held: `.top-chrome`, theme toggle, `<main class="profile-page">`, `<h2>Activity</h2>`, `{{if .Reports}}`, scripts app, votes, visibility, trust, (queue added), activity, activity-tabs, account-menu.
- (j) `#notice-live` is exactly the 04-13 element; copied.
- (k) The one `profileViewModel{` literal in tests is keyed.
- (l) Not measured (app.js is not in this plan).
- (m) `/profile` is still the Activity route.

One difference from the plan text: Phase 3.1 put `activity-tabs.js` after `activity.js`; the queue.js tag sits between trust.js and activity.js as asked and the order test checks the seven scripts the plan names.

## RED and GREEN

- Task 1 RED: both new database tests failed on the base (no attribute, no strip). GREEN: both pass on `p4_0425`.
- Task 2 RED: `TestQueueActivityStrip` and `TestQueueEditAndResendLink` failed (no resendHref, no focus fallback). GREEN: both pass.
- Task 3: the account binding case was proved to bite: with `belongsToAccount` weakened to `return !!record;` case 3 failed and the other three passed; the mutation was reverted (queue.js is unchanged by it). The fake IndexedDB deliberately ignores the key range so the check inside queue.js is what is tested.

## Verification

- `go test ./... -count=1` with no DATABASE_URL: every package ok.
- `DATABASE_URL=postgres://localhost:5432/p4_0425?sslmode=disable go test ./internal/... ./web/ -count=1 -p 1 -v`: 12 packages ok, 829 `--- PASS`, 0 FAIL, 1 SKIP (`TestScreeningFixture`, not database backed), so database backed SKIP count is 0. `p4_0425` created by me, absent before, dropped after and confirmed gone.
- `go test ./internal/api/handlers/ -run 'TestActivity|TestProfile'` on `p4_0425`: includes `TestActivityRendersOwnAccountID` and `TestActivityQueueMarkupInBothBranches`, all pass.
- `node --test --test-reporter=tap web/queue_logic_test.mjs`: 4 pass, 0 fail.
- `node --check` on queue.js and activity.js, `gofmt -l` and `go vet` clean. Added lines contain no U+2014 or U+2013. No package.json, ci.yml and web/embed.go unchanged.

## Decisions (the owner can veto)

- The Activity page supplies its own `queueFocusFallback()` through 04-22's existing `focusFallback` option (first non hidden button or link in the report list, else the back link), because the page has no focusable report row and no Report an emergency button.
- Task 3 (the Node test) was done, not declined.

## Deviations from Plan

None to the plan's behaviour. The strip container sits directly after `<h2>Activity</h2>`, above the subtitle, logout button and tabs, exactly as the plan says. Long compound verify one liners were split into plain steps (sandbox refusal), and `env -u DATABASE_URL` was replaced by running with DATABASE_URL unset.

## Known Stubs

None.

## Threat Flags

None beyond the plan register (T-04-14, T-04-01, T-04-13, T-04-05 mitigated as written). The resend link sits in the strip, outside every Activity card, so the one link per card pin is not touched.

## For 04-27 (human checks, V-24)

Run the Task 2 human check from the plan (database `p4_0425`, port 8792): block `*/api/reports`, post, open Activity, Send now, rejected row with Edit and resend to `/?resend=ID` and `/low?resend=ID` after visiting `/low`, discard focus lands on a list control or the back link, account A and B on one browser, 320 and 1280 widths. The Low data byte budget now includes this plan's queue.js additions (about 1 KB).

## Self-Check: PASSED

All created files exist (activity_queue_e2e_test.go, queue_activity_contract_test.go, queue_logic_test.mjs, this summary), commits 5846244, b4ece5b and 5ffacc5 exist on the worktree branch, and the test commands above passed.
