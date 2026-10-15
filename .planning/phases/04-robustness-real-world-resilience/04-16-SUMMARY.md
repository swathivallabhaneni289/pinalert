---
phase: 04-robustness-real-world-resilience
plan: 16
subsystem: activity-page
tags: [moderation, activity, trust, held, removed, written-line]
requires: [04-01, 04-05, 04-06, 04-07, 04-08, 04-12]
provides:
  - profileModeration, removedPolicyPointerParts and the moderation fields of the Activity row
  - data-moderation on held and removed rows, no data-trust on them
  - displayState removed and held rungs, the two Activity labels in visibility.js
  - vis-held and vis-removed row and badge rules, report-row__moderation-note rules
key-files:
  created:
    - internal/api/handlers/activity_moderation_e2e_test.go
    - web/activity_moderation_contract_test.go
  modified:
    - internal/api/handlers/auth.go
    - web/templates/profile.html.tmpl
    - web/static/js/visibility.js
    - web/static/js/activity.js
    - web/static/css/trust.css
    - web/votes_contract_test.go
    - web/activity_page_contract_test.go
key-decisions:
  - "data-moderation keys on Resolve's answer (withheld plus reason), never on the raw moderation column, because a pending serious report is public (D-01, D-02)"
  - "The Policy link is the one allowed link in a card (owner decision A), so TestActivityPageTemplateMarkup was relaxed for exactly that anchor"
duration: about 1 hour
completed: 2026-10-16
status: complete
requirements: [ROBUST-03, ROBUST-09]
---

# Phase 4 Plan 16: Held and removed Activity rows Summary

An author now sees, on their own Activity page, "Waiting for a text check" or "Removed after review" with the exact Go sentence for a report that is in no feed, with no trust block, counters, Reopen or vote control, and a late report shows "Written 4 hours ago".

## Commits

- 7a027d1 feat(04-16): tell the author why a report is held or removed on the Activity page (Task 1: `auth.go`, `profile.html.tmpl`, the e2e file, and the W-08 test relaxation)
- 9dc3b05 feat(04-16): label held and removed Activity rows after load (Task 2: `visibility.js`, `activity.js`, `trust.css`, `votes_contract_test.go`, the web contract file, the copy drift test)
- The SUMMARY commit follows.

## What was built

- **Server** (`auth.go`): `profileModeration` reads `ar.Visibility != service.VisibilityWithheld` and the reason, so removed is `removed`, any other withheld is `held`, and a pending serious report (published) gets none. `newProfileReport` keeps its signature, marshals trust only when there is no moderation (still exactly one `json.Marshal(trustToResponse(ar.Trust))` line), writes the P4-A9 first paint class (removed, then held only while unexpired, then 03-15's rules), and fills `ModerationNote` (the Go constants), `PolicyPointer` and `WrittenText` (`Written ` plus `profileRelativeTime`). `Profile` fills the three pointer parts from `removedPolicyPointerParts`.
- **Template**: `data-trust` only when not moderated, `data-moderation` when moderated, the Written line as a second `report-row__meta` paragraph after the card's meta line, the note paragraph with the Policy link only when `$.Chrome.ContactEmail` is set.
- **visibility.js**: `displayState` gets the removed and held rungs first (held only while `report.expired !== true`), the label table gets `held` and `removed`; `VISIBILITY_STATES`, `VISIBILITY_REASONS` and `visibilityState` are unchanged.
- **activity.js**: the row report object reads `moderation`; a moderated row mounts no trust block and its label goes in with `body.insertBefore(tag, note)`.
- **trust.css**: `.vis-held` and `.vis-removed` with the exact expired declarations (and badge rules), each in its own rule, plus the note and note link rules. No new custom property, colour literal or `!important`.

## Deviations from Plan

The orchestrator decided option A for the blocking item and told me to adapt the plan's literals to the real card markup. Every change:

**1. [Orchestrator decision A] Relaxed W-08 test.** `web/activity_page_contract_test.go` (added to this plan's scope) `TestActivityPageTemplateMarkup`: a card may now hold exactly one link, `<a href="/policy">`, and only inside `report-row__moderation-note`. The test cuts the note paragraphs out and fails on any other `<a ` in the card, and fails if a note holds more than that one anchor. The comment records that W-08's intent stands.

**2. [Rule 3 - literal] Card markup (quick task 261013-wm5).**
- The severity word is `activity-chip__word--severity` in the chip, not a first child of `.report-row__body`. The e2e "severity still reads Critical" check reads `activity-chip__word--severity">Critical<`.
- There is no `MetaText` paragraph. The meta line is `<p class="report-row__meta activity-card__sub">`. The Written line (`<p class="report-row__meta">Written 4 hours ago</p>`) is written directly after it, and the note after the Written line.
- The state tag was mounted only through `createStateLine` with a trust block. For a moderated row the tag is inserted before the note.

**3. [Rule 3 - foreign pin] `body.appendChild(tag)` is banned in `activity.js`** by `TestTrustStateLineMarkup` (`web/trust_topzone_contract_test.go`, plan 03-23). I did not edit that test. The moderated branch calls `body.insertBefore(tag, moderationNote)` always; when the row has no note the reference is null and the DOM appends. This satisfies the plan's `insertBefore(tag` contract. In practice every moderated row except the held and expired one has a note; a held and expired row has none and reads Expired.

**4. [Rule 3 - import path]** The plan spells `pinalert/internal/store/sqlcgen`; the real import is `sqlcgen "pinalert/internal/store/sqlc"`.

**5. [Rule 3 - Task 1 verify]** The planned command ran as separate steps (sandbox refuses the long one liner and `$(...)`): `printenv DATABASE_URL` empty, then plain `go test`; `createdb p4_0416` with no `dropdb --if-exists` first (it did not exist), dropped at the end. The node check ran from a scratchpad script file (`p416check.js`), same assertions, printed `moderation displayState ok`.

**6. [Census note]** Step 0 results. All prerequisite symbols exist (`newProfileReport` signature, `profileExpiredDisplayState`, the one marshal line, `data-expired` and `data-trust` on the li, `trust.js` tag, `readRowTrust`, `trustBlocks`, `displayState`, `stateWord`, the helpers `p15*`, `stripJSComments(src string) string`, `jsFunctionBody`). `newE2EServerWith(t, tweak func(d *api.Deps, q *sqlcgen.Queries)) (*httptest.Server, *recordingMailer, *pgxpool.Pool)`; the default server builds `service.NewReportService(queries)` with no screener, so submitted reports are stored clear. Only `.vis-expired` is `--severity-current: var(--color-border); --severity-tint: transparent; --badge-glyph-fg: var(--color-text-muted);` with badge `background: transparent; outline: 1px solid var(--color-border);`. Tests that read `report-row__meta` or the li attributes: `activity_trust_contract_test.go` (li contains `data-trust="{{.TrustJSON}}"`, `.TrustJSON` once; still holds), `activity_page_contract_test.go` (the W-08 pin, relaxed), `compact_row_contract_test.go` and `labels_contract_test.go` (CSS only).

## For the owner (found, left unfixed, outside this plan's files)

1. `activityTokens` in `internal/api/handlers/activity.go` puts every withheld own post, including a resolved then removed one, under the "Not resolved" tab and counts it there. Which tab a held or removed card belongs to is an owner decision.
2. The `.activity-chip` word rules in `web/static/css/activity.css` key on `vis-retracted` and `vis-hidden` only. A held or removed card shows the severity word in its chip, like an expired card; it shows neither "Resolved" nor "Not resolved". Probably what is wanted, but not stated in the plan.

## Verification

- RED: Task 1's four e2e tests failed before the code (4 of 4; the pending critical and clear subtests already passed). Task 2's four web tests and the copy test failed before the code.
- `DATABASE_URL=postgres://localhost:5432/p4_0416?sslmode=disable go test ./... -count=1 -p 1 -v`: every package `ok`, 741 `--- PASS`, 1 `--- SKIP`. The one SKIP is `TestScreeningFixture` (240 fixture entries have no recorded scores until 04-27); it is not a database backed test. Database backed SKIP count: 0.
- Handlers package alone on `p4_0416`: 142 PASS, 0 SKIP (run before Task 2).
- With DATABASE_URL unset, `go vet ./...` and `go test ./... -count=1` pass.
- Added lines contain no U+2014 or U+2013. `p4_0416` was created fresh and dropped.
- The flaky `TestCastVoteConcurrentSameAccountKeepsEveryRow` did not fail.

## For later plans

- 04-17: the Written line on the Activity page reuses `report-row__meta`; the feed row and popup lines are 04-17's.
- 04-25: `activity.js` now reads `moderation` and inserts the label before `.report-row__moderation-note`; it still has one `addEventListener` and one `Pinalert.showToast(`.
- 04-27: run the Task 2 human check (V-24) against a card layout: the label sits above the sentence in the card body; note link style is `--color-text` with underline.

## Known Stubs

None.

## Threat Flags

None. The only new link is the static `/policy`, rendered only with a contact address; no flag counts, scores or moderator identity appear.

## Self-Check: PASSED

7a027d1 and 9dc3b05 exist; `activity_moderation_e2e_test.go`, `web/activity_moderation_contract_test.go`, and the edited files are present; full suite green on `p4_0416` with no database backed SKIP.
