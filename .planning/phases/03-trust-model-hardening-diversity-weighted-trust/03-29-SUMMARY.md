---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 29
subsystem: trust-copy
tags: [wording, trustcopy, swagger, docs, gap-closure, sketch-003]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the trustcopy.go identifiers and the A85 wording from plan 03-28
provides:
  - The three scored still current words with a subject (Report up to date, Report getting old, Report needs re-confirming)
  - Amendment row A86 and the amended contract lines in 03-UI-SPEC.md
  - docs/TRUST-MODEL.md Word column and the swagger example in the new words
  - Three labelled Phase 4 text edits
affects: [03-UAT test 10 plain language check, Phase 4 plans 04-12 and 04-UI-SPEC]
tech-stack:
  added: []
  patterns:
    - "Label text changes under unchanged identifiers, so no caller changes"
key-files:
  created: []
  modified:
    - internal/service/trustcopy.go
    - internal/service/currency.go
    - internal/api/handlers/trust.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - docs/TRUST-MODEL.md
    - internal/service/trustcopy_test.go
    - internal/service/currency_test.go
    - internal/service/trustview_test.go
    - internal/api/handlers/trust_e2e_test.go
    - web/trust_contract_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-20-SUMMARY.md
    - .planning/phases/04-robustness-real-world-resilience/04-12-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md
key-decisions:
  - "A86: the three scored still current words name their subject; the third word, Report needs re-confirming, is an assumption the owner confirms at the final session"
requirements-completed: []
status: complete
---

# Phase 3 Plan 29: The still current words get a subject Summary

The time based labels now read "Report up to date", "Report getting old" and "Report needs re-confirming" (D-18 as amended by A86); "Too early to tell", every panel sentence, the band names and slugs, the JSON shape and the vote error are unchanged.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 (RED) | 375dbc6 | test(03-29): pin the still current words with a subject (A86) |
| 2 (GREEN) | b2b1e1b | feat(03-29): the still current words name the report (A86) |
| 3 | 997aad2 | docs(03-29): the still current words with a subject in the docs, the contract and Phase 4 (A86) |

## RED failures (Task 1)

Before trustcopy.go changed, `go test ./internal/service/ -count=1 -v` failed exactly four tests, all on word mismatches, with no build or setup failure: TestTrustCopyCurrencyIsPlainText, TestCurrencyRetapCannotRefresh, TestCurrencyDisputes, TestBuildTrustCarriesCurrency. `go vet ./internal/service/ ./internal/api/handlers/ ./web/` passed (trust_e2e_test.go compiles; the orchestrator's scratch gate proves it). `go test ./web/ -run 'TestTrustModuleHasNoThresholdsOrSinks|TestTrustCSS'` passed after the ban list gained the new words (P7 is a guard).

## Test changes (existing assertions changed, all named in the plan's Pins table)

Each old pin now pins the new word; none was deleted or loosened.

| Pin | Where | Old text | New text |
| --- | ----- | -------- | -------- |
| P1 | trustcopy_test.go TestTrustCopyCurrencyIsPlainText "exact copy" | `"Up to date"`, `"Getting old"`, `"Needs re-confirming"` | `"Report up to date"`, `"Report getting old"`, `"Report needs re-confirming"` |
| P2 | currency_test.go TestCurrencyRetapCannotRefresh (l.328) | `"Needs re-confirming"` | `"Report needs re-confirming"` |
| P3 | currency_test.go TestCurrencyDisputes (l.375, l.453) | `"Needs re-confirming"` | `"Report needs re-confirming"` |
| P4 | trustview_test.go TestBuildTrustCarriesCurrency (l.274, l.284) | `"Up to date"`, `"Getting old"` | `"Report up to date"`, `"Report getting old"` |
| P5 | trust_e2e_test.go (database test) p14UpToDate (l.834) and TestFeedSignalsMatchCastVoteSignals step 3 (l.964) | `"Up to date"`, `"Needs re-confirming"` | `"Report up to date"`, `"Report needs re-confirming"` |
| P6 | trust_e2e_test.go step 4 comment (l.974) | `so the report reads Up to date again.` | `so the report reads Report up to date again.` |
| P7 | web/trust_contract_test.go TestTrustModuleHasNoThresholdsOrSinks, `trust.js server copy` list | every entry kept | one inserted line: `"Report up to date", "Report getting old", "Report needs re-confirming",` |

## What changed

- **trustcopy.go.** The three label constant values (C1), the header comment now cites A86 (C2 line 2) and the label block comment is the four line text from the plan. No sentence, identifier, signature or caller changed. `currencyLabelTooEarly` is untouched.
- **currency.go.** Two comments reworded to name the band (C3).
- **handlers/trust.go.** `CurrencyResponse.Label` example tag is "Report up to date" (C4).
- **Swagger.** Drift check first, on the unedited tree: `/Users/swathivallabhaneni/go/bin/swag init -g internal/api/router.go -o docs` left `git status --short` empty, so there was no drift. After the edit the same command changed exactly one line in each of docs.go (l.630), swagger.json (l.623) and swagger.yaml (l.68): the label example, "Up to date" to "Report up to date". The plain form of the command ran in the sandbox (no `env -u` needed).
- **TRUST-MODEL.md.** Three Word cells only; band cells, the boundary table and the prose are untouched.
- **03-UI-SPEC.md.** Row A86 once, the intro now says 86 rows A1 to A86, and 11 lines carry the tag (amended by A86) (U3 to U6, U8 to U14), plus the markup sample in U7. A1 to A85, RR-1, RR-2, D rows, Q3, the Panel sentence templates (the Case column names bands) and Open item 4 are byte identical (no locked row appears among the removed lines). Added by a small local script that asserted one match per anchor.
- **03-20-SUMMARY.md.** V1 to V3 appended; old text and the 03-28 note kept.
- **Phase 4.** X1 and X2 in 04-12-PLAN.md and X3 in 04-UI-SPEC.md, each tagged Phase 3 plan 03-29. The stale feed announcement "Back online. The list is up to date." was left alone.
- **Vote error.** votes.go and votes_e2e_test.go are untouched: "This report has expired." occurs once in votes.go and four times in the e2e test.

## Green runs

- `go test ./internal/service/ -count=1`: ok (whole package).
- `go test ./internal/service/ -run TestTrustModelDocMatchesCode -count=1`: ok after the TRUST-MODEL.md edit.
- `go build ./...`, `go vet ./internal/... ./web/`: pass. `go test ./web/ -count=1`: ok.
- `gofmt -l internal web`: nothing.
- Old quoted word grep over internal/, docs/docs.go and docs/swagger.json: no match. Old words in 03-UI-SPEC.md remain only in Q3, A85, A86, the four "Still current" template rows and Open item 4 (the allowed set). 04-12-PLAN.md and 04-UI-SPEC.md hold no old word.
- The precondition held: `grep -l 'TestMain\|NewTestDB' internal/service/*_test.go` printed nothing. No database, server, browser or port was used. The handlers, store and api package tests were not run.
- No em or en dash appears in any added line (checked on the diff of each task).

## Deviations from Plan

None. The plan was executed as written. All anchors were found as named; line numbers matched the plan's hints. The verify one-liners were run as separate commands because the sandbox refuses long compound git commands.

## Live proof still owed (orchestrator, after merge)

The full gate on a scratch database (fail 0, skip 0) proves trust_e2e_test.go, then the browser harness (with its currency word list updated) shows the new words. The owner confirms the third word at the final session.

## Known Stubs

None.

## Threat Flags

None. The words are constants only (T-03-06 held by the unchanged closed character set tests; the hyphen sits inside re-confirming), the docs greps cover T-03-29-01 and the trust.js ban list covers T-03-29-02.

## Self-Check: PASSED

- Commits 375dbc6, b2b1e1b and 997aad2 exist on the worktree branch, each listing only its own paths.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched; nothing under .planning/sketches/, 03.1-* or 03.2-* was touched.
