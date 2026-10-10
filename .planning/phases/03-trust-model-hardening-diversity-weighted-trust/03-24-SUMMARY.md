---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 24
subsystem: docs
tags: [trust-model, docs, gap-closure, reporter-tag, state-line]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: plan 03-23, the createStateLine placement helper and TestTrustStateLineMarkup
provides:
  - docs/TRUST-MODEL.md says both signals show at once, the still current word in the trust block and the reporter tag beside the state label
  - The traceability row and the Phase 8 hand off name createStateLine
affects: [Phase 3 close, Phase 8 report page]
tech-stack:
  added: []
  patterns:
    - "Doc claims about tests are checked by TestTrustModelDocMatchesCode"
key-files:
  created: []
  modified:
    - docs/TRUST-MODEL.md
key-decisions:
  - "No sentence outside the four named lines changes, so UAT Gap 3 (shorter wording, A84) stays open"
requirements-completed: []
status: complete
---

# Phase 3 Plan 24: Trust model doc corrections for the reporter tag placement Summary

Four lines of docs/TRUST-MODEL.md now describe the shipped layout (A75, D-18): the reporter tag sits as plain words beside the state label, not in a signals row, and the placement helper is named for later surfaces (D-20).

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 | 95b1688 | docs(03-24): place the reporter tag beside the state label in the trust model |

## What changed

Exactly four physical lines, found by their words (diff is 4 added, 4 removed):

1. Section "Tags and sentences", the "New reporter never looks like a good tag." paragraph: "The feed row and the Activity page show the tag." became the A75 sentence naming the feed row, the Activity page and the popup, the tag as plain words beside the state label, at the top of the report and outside the trust block's button, alone on that line when there is no state label.
2. Section "How the roadmap word score is met for the reporter": "The two signals sit side by side on every report." became "Both signals show at once on every report without opening anything: the still current word in the trust block and the reporter tag beside the state label (A75)."
3. Table "Where the roadmap words are met", row "two distinct trust signals side by side": second cell rewritten, third cell now `BuildTrust`, `createStateLine`, fourth cell now also cites `TestTrustStateLineMarkup`.
4. Section "Hand offs to later phases": the Phase 8 bullet now says the report page places the reporter words with the same helper as the other surfaces, `createStateLine`.

Lines that use "beside" to mean "in addition to" were left alone. No heading changed.

## Verify

The plan's single automated verify block was run as separate plain commands (the sandbox refuses the long compound form, and `env -u` was dropped per the repo rules; DATABASE_URL was never set):

- `git diff --numstat HEAD -- docs/TRUST-MODEL.md` before the commit: `4	4`.
- `go test ./internal/service/ -run TestTrustModelDocMatchesCode -count=1`: ok.
- `grep -c 'beside the state label'`: 3 (needs at least 3).
- `grep -c '`createStateLine`'`: 2, present.
- The traceability row cites `TestTrustStateLineMarkup`.
- The old sentence "The two signals sit side by side on every report." count: 0.
- Em or en dash bytes in the document: 0.
- `git status --porcelain` before the commit listed only docs/TRUST-MODEL.md, so nothing under internal, web, cmd, the Phase 4 plans or the sketches changed.
- `git show --stat --format= HEAD` lists only docs/TRUST-MODEL.md.

## Precondition grep

- `web/static/js/trust.js` line 582 exports `createStateLine: createStateLine`. The shipped helper name is `createStateLine`, which equals `PinalertTrust.createStateLine` as 03-UI-SPEC.md A75 states. No spec flag needed.
- `TestTrustStateLineMarkup` exists in `web/trust_topzone_contract_test.go`; `TestTrustBlockMountedOnBothSurfaces` in `web/trust_contract_test.go`; `TestFeedSignalsMatchCastVoteSignals` in `internal/api/handlers/trust_e2e_test.go`.

## Deviations from Plan

None. The plan was executed as written. No existing test assertion was changed.

## Open items for the orchestrator and owner

- UAT Gap 3 (shorter panel wording, A84) is still open; no sentence, trustcopy.go line, wording identifier or wording test changed here.
- Live proof of variant H (A75 to A83) in Chromium and WebKit is left to the orchestrator after the merge, as the plan says.
- Owner item for phase close: ROADMAP Success Criterion 2 says "side by side", TRUST-07 does not; the traceability row keeps the roadmap phrase as its first cell. The owner reviews both by hand.

## Known Stubs

None.

## Self-Check: PASSED

- docs/TRUST-MODEL.md modified, commit 95b1688 exists on this branch.
