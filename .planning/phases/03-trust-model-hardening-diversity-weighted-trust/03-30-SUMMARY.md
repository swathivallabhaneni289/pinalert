---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 30
subsystem: trust-copy
tags: [wording, trustcopy, swagger, docs, gap-closure, owner-feedback]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the trustcopy.go identifiers and the A85 and A86 wording from plans 03-28 and 03-29
provides:
  - Panel sentences that say people at different spots and never the word place
  - The caveat "Counts places, not votes." retired, and shorter lines
  - A lead sentence that follows the confirming count (needs 2 people, or 1 more person)
  - Amendment row A88 and the amended contract lines in 03-UI-SPEC.md
  - docs/TRUST-MODEL.md, README.md and the swagger example in the new words
  - Labelled Phase 4 text edits (04-12 and 04-UI-SPEC)
affects: [03-UAT test 10 plain language check, Phase 4 plans 04-12 and 04-UI-SPEC, plan 03-31 (the tick mark card reads the same trust object)]
tech-stack:
  added: []
  patterns:
    - "Every filled line stays filled: only the words changed, so a later redesign changes how the lines look and not which lines exist"
key-files:
  created:
    - internal/service/trustwording_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-30-PLAN.md
  modified:
    - internal/service/trustcopy.go
    - internal/service/trustview.go
    - internal/api/handlers/trust.go
    - internal/api/handlers/reports.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - docs/TRUST-MODEL.md
    - README.md
    - internal/service/trustcopy_test.go
    - internal/service/currency_test.go
    - internal/service/reporter_test.go
    - internal/service/trustview_test.go
    - internal/service/feed_test.go
    - internal/service/trust_paths_test.go
    - internal/api/handlers/trust_e2e_test.go
    - web/trust_contract_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-20-SUMMARY.md
    - .planning/phases/04-robustness-real-world-resilience/04-12-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md
key-decisions:
  - "A88: people confirm, never places. The count is still of distinct places (a lower bound on people), and the sentences name people at different spots, so a crowd at one spot counts once without a caveat"
requirements-completed: []
status: complete
---

# Phase 3 Plan 30: The panel words are plain and people confirm Summary

## Why

At the final manual session on 2026-10-10 the owner opened the trust panel on a feed row and said the words "2 different places confirmed this" make no sense ("how can places confirm it"), and that three or four sentences is too long, especially on a phone. The panel they saw had four lines and six sentences, including the caveat "Counts places, not votes."

## What changed

Server authored copy only. No JSON field name or shape changed, no JavaScript, no CSS, and every line that was filled before is still filled.

| Line | Before | After |
|------|--------|-------|
| Lead, nobody confirmed | Needs 2 confirmations to count as confirmed. | Needs 2 people at different spots to confirm it. |
| Lead, one confirmed | (same line) | Needs 1 more person at a different spot to confirm it. |
| Confirmed, 0 | No one has confirmed this yet. Counts places, not votes. | No one has confirmed this yet. |
| Confirmed, 1 | 1 place confirmed this. Counts places, not votes. | 1 person confirmed this. |
| Confirmed, 2 or more | 2 different places confirmed this. Counts places, not votes. | Confirmed by 2 people at different spots. |
| Disputed, 0 | No one nearby has disputed this. | No one has disputed this. |
| Disputed, 1 | 1 place disputed this. | 1 person nearby disputed this. |
| Disputed, 2 or more | 2 different places disputed this. | 2 people at different spots disputed this. |
| Up to date | 2 places confirmed this in the last 2 hours. | 2 confirmations in the last 2 hours. |
| Getting old | Fewer than 2 places confirmed it in the last 2 hours. | Fewer than 2 confirmations in the last 2 hours. |
| Needs re-confirming | Gone quiet. People nearby can confirm it again. | unchanged |
| Too early to tell | Fewer than 2 places have confirmed this. | Fewer than 2 people have confirmed this yet. |
| Reporter, rated | 0 of 5 earlier reports were confirmed by people nearby. | 0 of their last 5 reports were confirmed. |
| Reporter, new | A rating needs 5 reports with a clear result. They have none so far. | Not rated yet. It needs 5 reports with a clear result. |
| Score lines (popup) | Score 99 out of 100, based on ... | Score 99 out of 100. |

The four labels (Report up to date, Report getting old, Report needs re-confirming, Too early to tell) and the reporter words are unchanged. The caveat constant is retired. The singular and plural forms are handled everywhere a number can be 1.

## Proof

- Full suite on a fresh scratch database after the merge: 390 PASS, 0 FAIL, 0 SKIP at top level (385 before, five new tests), `go vet` clean, `gofmt` lists nothing.
- New tests in `trustwording_test.go` pin that no panel sentence contains the word place in any state (Provisional with 0 and 1 confirms, every currency band, Hidden, Retracted, expired, every reporter tag), the closed character set, the plural and singular forms, and that no sentence that was filled became empty.
- The scripted browser harness holds no literal quote of the old sentences (searched), so none of its checks changed meaning. A focused re-run is still due with plan 03-31.

## Not done in this plan

- The measured before and after panel heights at 320, 360 and 390 pixels were not taken: the builder was interrupted twice (the Mac slept, then the weekly usage limit). The numbers come with plan 03-31, which changes the look.
- The look is plan 03-31 (the owner chose the tick mark checklist picture).

## Deviations

The builder agent could not finish: it was cut off by the machine sleeping and then by a usage limit, after six commits (plan, red tests, code, three document commits, plan note). The orchestrator reviewed the diff, ran the gate, merged it (merge 4a6ae83) and wrote this summary. Two en or em dashes appear in the regenerated swagger files; they come from an older handler comment (backlog item 999.8), not from this plan.
