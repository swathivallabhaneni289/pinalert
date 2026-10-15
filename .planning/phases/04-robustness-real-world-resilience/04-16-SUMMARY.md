---
phase: 04-robustness-real-world-resilience
plan: 16
subsystem: activity-page
tags: [moderation, activity, blocked]
status: blocked
requirements: [ROBUST-03, ROBUST-09]
completed: 2026-10-16
---

# Phase 4 Plan 16: Held and removed Activity rows Summary

**Status: BLOCKED at Task 1. No task commit was made. This plan needs an orchestrator decision and a plan amendment before it can run.**

## What happened

Task 1 step 0 (the census) found that the code on base 1ebcc65 no longer matches the markup the plan names, and the Task 1 template edit then broke a contract test in a file this plan does not own. Rules 6 and 10 of the executor rules say to stop and record, not to improvise or edit a foreign file. The tracked edits were reverted and the disposable database `p4_0416` was dropped.

A working Task 1 patch exists outside the repo (see Recovery below). With it, all four Task 1 end to end tests pass on `p4_0416` (TestActivityHeldAndRemovedRows, TestProfileReportAcceptsWithheld, TestRemovedPolicyPointerNeedsContact, TestActivityWrittenLine), and so do the 03-15, 02-07 and 04-07 Activity tests and TestExistingPagesRenderWithChrome. Only the foreign pin below fails.

## Deviations from Plan

### Blocking: a template edit breaks a pin in a file outside files_modified

- **Where:** `web/activity_page_contract_test.go`, `TestActivityPageTemplateMarkup`, line 101: `the card holds a link, nothing opens from a card yet (W-08)`. It rejects any `<a ` inside the `<li class="report-row` ... `</li>` text of `profile.html.tmpl`.
- **Cause:** the plan requires `<a href="/policy">` exactly once inside the removed row's note (D-24, acceptance criterion on `profile.html.tmpl`). That is a link inside a card, which the owner decision W-08 (quick task 261013-wm5) forbids.
- **Run:** `go test ./web/ -count=1` fails with only that one test after the template edit.
- **Plan rule that applies:** Task 1 step 0 says a pin broken by the template edit in a file not in this plan means stop and report.

### Plan literals that no longer exist on the base (Activity card redesign, quick task 261013-wm5)

The plan was written against the pre redesign Activity row. On the base:

1. Cross-phase prerequisite "the severity span as the first child of `.report-row__body`" is absent. The body now starts with `<span class="activity-card__tag">{{.Tag}}</span>` and then the title. The severity word is in the chip (`activity-chip__word--severity`), outside the body. `TestProfileTemplateUsesServerSeverityAndTrust` already records this as a departure.
2. There is no `<p class="report-row__meta">{{.MetaText}}</p>`. The meta line is `<p class="report-row__meta activity-card__sub">` built from Action, When and Detail. The plan's Written line anchor ("directly after the first meta paragraph") still works (I placed it after that paragraph), but the exact literal differs.
3. The state label is not appended with a plain `body.appendChild(tag)`. `activity.js` mounts the tag only inside `PinalertTrust.createStateLine('report-row__stateline', tag, trustBlock)`, which needs a trust block. For a moderated row (no trust block) the tag needs its own mount, so Task 2 must add an else branch: `body.insertBefore(tag, note)` when a `.report-row__moderation-note` exists, otherwise `body.appendChild(tag)`. Keep `!report.moderation` textually between `applyVisibilityClass(row, report)` and `createTrustBlock()` so the planned contract test window holds.
4. `displayState` in `visibility.js` has an extra 3.1 history rung (`report.history.state`). The planned insertion point (before `var state = visibilityState(report);`) still works and keeps the required order for `TestDisplayStateModeration`.

### Two things left as is, to record for the plan amendment (both outside this plan's files)

- `activityTokens` in `internal/api/handlers/activity.go` puts every withheld own post, including a resolved then removed one, under the "Not resolved" tab (it is neither retracted nor expired by its visibility value). A held or removed card will therefore count as "Not resolved". Owner decision needed on which tab a held or removed card belongs to.
- The `.activity-chip` word rules in `web/static/css/activity.css` key on `vis-retracted` and `vis-hidden` only. A `vis-held` or `vis-removed` card shows the severity word in its chip, like an expired card. That matches the intended "expired treatment", but it is not stated in the plan.

## Decision needed from the orchestrator or owner

The Policy link (D-24) needs a card to hold a link, against W-08. Pick one:

- A. Relax `TestActivityPageTemplateMarkup` to allow a link only inside `report-row__moderation-note` (a one line change in `web/activity_page_contract_test.go`, which then joins this plan's `files_modified`). Keeps the plan's template literals and acceptance greps as written.
- B. Keep W-08 and write the anchor from Go (a `template.HTML` field built from the constants and the static `/policy`). This changes the plan's template contract (`<a href="/policy">` would no longer be a template literal), so the acceptance greps need amending.

Either way the plan text should be amended for items 1 to 3 above, and the "Exact" template anchors rewritten for the card markup.

## Recovery

Saved outside the repo, in the session scratchpad:

- `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/554698ba-3aab-477a-9c2f-d24cd5912cb3/scratchpad/p416-partial/task1-tracked.patch` : the `auth.go` and `profile.html.tmpl` changes of Task 1 (`profileModeration`, `removedPolicyPointerParts`, the new `profileReport` and `profileViewModel` fields, the P4-A9 class precedence, trust omitted on moderated rows, the Written line, the note paragraph and the contact gated Policy link).
- `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/554698ba-3aab-477a-9c2f-d24cd5912cb3/scratchpad/p416-partial/activity_moderation_e2e_test.go.txt` : the Task 1 end to end test file (rename to `.go`; it imports `sqlcgen "pinalert/internal/store/sqlc"`, which the plan text spells wrongly as `.../sqlcgen`).

## Commits

None for code. This SUMMARY is the only commit.

## Known Stubs

None.

## Self-Check: FAILED

Plan not executed: blocked at Task 1 by a foreign contract pin (W-08) and by plan literals that predate the Activity card redesign. No task commits exist, by design.
