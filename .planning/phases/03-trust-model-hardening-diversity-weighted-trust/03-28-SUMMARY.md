---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 28
subsystem: trust-copy
tags: [wording, trustcopy, swagger, docs, gap-closure, sketch-003]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the variant H layout plans 03-21 to 03-24 and the trustcopy.go identifiers
provides:
  - The shorter panel wording of sketch 003 (S1 to S20) in trustcopy.go, with the caveat once on the confirmed sentence only
  - Amendment row A85 and the amended Panel sentence templates in 03-UI-SPEC.md
  - docs/TRUST-MODEL.md and the swagger examples in the new wording
  - Four labelled Phase 4 text edits
affects: [03-UAT Gap 3 look check, Phase 4 plans 04-12, 04-16, 04-18, 04-UI-SPEC]
tech-stack:
  added: []
  patterns:
    - "Sentence text changes under unchanged identifiers, so no caller changes"
key-files:
  created: []
  modified:
    - internal/service/trustcopy.go
    - internal/api/handlers/trust.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - docs/TRUST-MODEL.md
    - internal/service/trustcopy_test.go
    - internal/service/currency_test.go
    - internal/service/reporter_test.go
    - internal/service/trustview_test.go
    - internal/service/feed_test.go
    - internal/service/trust_paths_test.go
    - internal/api/handlers/trust_e2e_test.go
    - internal/api/handlers/activity_e2e_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-20-SUMMARY.md
    - .planning/phases/04-robustness-real-world-resilience/04-12-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md
    - .planning/phases/04-robustness-real-world-resilience/04-16-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-18-PLAN.md
key-decisions:
  - "A85: the shorter wording of sketch 003 supersedes A84's deferral for the wording only; the vote error for an expired report stays unchanged"
requirements-completed: []
status: complete
---

# Phase 3 Plan 28: The shorter panel wording Summary

Every panel sentence the server writes is now the Proposed sentence of sketch 003, with every number still real, and the caveat "Counts places, not votes." ends the confirmed sentence only, exactly once.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 (RED) | 1f88566 | test(03-28): pin the shorter panel wording (A85) |
| 2 (GREEN) | 072a3f4 | feat(03-28): shorter panel wording, caveat once (A85) |
| 3 | 361cf54 | docs(03-28): shorter panel wording in the trust model and the UI contract (A85) |
| 4 | 08f5e61 | docs(03-28): Phase 4 anchors follow the shorter panel wording |

## RED failures (Task 1)

Before trustcopy.go changed, `go test ./internal/service/ -count=1 -v` failed exactly these 18 tests, all on sentence mismatches, with no build or setup failure: TestTrustCopyCurrencyIsPlainText, TestTrustCountLabel, TestTrustCounterSentences, TestTrustCopyIsPlainText, TestTrustCopyReporterIsPlainText, TestCurrencyScoreBoundaries, TestCurrencyNotYetCorroborated, TestCurrencyRetapCannotRefresh, TestCurrencyDisputes, TestCurrencyWindowFollowsLifetime, TestReporterSignalsGroupByAccount, TestReporterSignalsNewAndNone, TestBuildTrustCarriesCurrency, TestTrustLeadSentence, TestCastVoteResultCarriesTrust, TestReporterSignalKeyedByAccount, TestCastVoteTrustMatchesNearbyTrust, TestTrustAttachedOnEveryReadPath. `go vet ./internal/service/ ./internal/api/handlers/` passed (the handlers database tests compile; the orchestrator's scratch gate proves them).

## What changed

- **trustcopy.go.** Sentences only (S1 to S20); every identifier, signature and switch shape is unchanged. `countsCaveat` is "Counts places, not votes." and is appended in `confirmedSentence` only; `disputedSentence` carries no caveat. `reporterSentence` for a New reporter now returns the rule sentence first, then "They have N so far." (or "none"). File comment cites A85.
- **trust.go.** Six `example:` tags only (S1, S5, S7, S10, S15, and the reporter S20).
- **Swagger.** `swag init -g internal/api/router.go -o docs` ran on the unedited tree first: no drift (`git status --short -- docs` empty). After the edit, every changed line in docs.go, swagger.json and swagger.yaml is an `example` line or the wrapped YAML continuation of one.
- **Documents.** TRUST-MODEL.md differs by exactly 12 lines (the eight counter table rows and four reporter template bullets). 03-UI-SPEC.md gains row A85, the 85 rows intro, the amended items 5 and 7 of the panel, the expired panel bullet, the Activity copywriting row, the whole Panel sentence templates table and the Rules line; the locked rows A1 to A84, RR-1, RR-2 and the vote error row are byte identical. The V-42 line of 03-20-SUMMARY.md keeps its text and gains the wording note.
- **Phase 4.** 04-12 changed on two lines, 04-UI-SPEC on one, 04-16 and 04-18 gained one note line and a blank line each, all tagged Phase 3 plan 03-28.
- **Vote error.** `internal/api/handlers/votes.go` and `votes_e2e_test.go` are untouched: "This report has expired." occurs once in votes.go and four times in the e2e test, byte identical.

## Test changes (existing assertions changed, all named in the plan's Pins table)

Each old pin was changed to the new verbatim sentence; none was deleted or loosened. Where one old to new pair repeats across tests, the tests are listed.

| Old text | New text | Where |
| -------- | -------- | ----- |
| `{R} different places confirmed this in the last {W}.` (R 2, W 2 hours) | `2 places confirmed this in the last 2 hours.` | trustcopy_test (TestTrustCopyCurrencyIsPlainText), currency_test (ScoreBoundaries, RetapCannotRefresh, WindowFollowsLifetime), trustview_test (BuildTrustCarriesCurrency), trust_e2e_test p14UpToDate format |
| `Confirmed earlier, but fewer than 2 places confirmed it in the last W.` | `Fewer than 2 places confirmed it in the last W.` | trustcopy_test (6 hours), currency_test (ScoreBoundaries, Disputes), trustview_test |
| `Confirmed earlier, but it has gone quiet. People nearby can confirm it again.` | `Gone quiet. People nearby can confirm it again.` | trustcopy_test, currency_test (ScoreBoundaries, Disputes) |
| `Someone nearby disputed this, so it needs K places to confirm it in the last W.` | `Disputed nearby, so it needs K places in the last W.` | trustcopy_test (3, 2 hours), currency_test Disputes (3 and 2 hours, 4 and 2 hours, 3 and 6 hours), trust_e2e_test (3, 2 hours) |
| `Fewer than 2 places have confirmed this so far.` | `Fewer than 2 places have confirmed this.` | trustcopy_test, currency_test (three subtests), trust_e2e_test |
| `Score N out of 100, based on how recently different places confirmed this.` | `Score N out of 100, based on recent confirmations.` | trustcopy_test (82), currency_test (93), trustview_test (93), trust_e2e_test (format and 0) |
| `{N} people in {N} different places confirmed this. Counts separate places, not votes.` | `{N} different places confirmed this. Counts places, not votes.` | TestTrustCountLabel (150), TestTrustCounterSentences (2, 100), trustview_test (3), trust_paths_test (2), trust_e2e_test (p07Confirmed2, p07Confirmed3, 12) |
| `1 person in 1 place confirmed this. Counts separate places, not votes.` | `1 place confirmed this. Counts places, not votes.` | TestTrustCounterSentences, trust_e2e_test p07Confirmed1 |
| `No one has confirmed this yet. Counts separate places, not votes.` | `No one has confirmed this yet. Counts places, not votes.` | TestTrustCounterSentences |
| disputed sentences with the old caveat (0, 1, 2, 100 / M 0, 1, 2) | the same sentences, new count form, no caveat: `No one nearby has disputed this.`, `1 place disputed this.`, `{M} different places disputed this.` | TestTrustCounterSentences, trustview_test, trust_paths_test, trust_e2e_test (p07NoDispute, p07Disputed1, p07Disputed2) |
| `Needs T confirmations before this counts as confirmed.` | `Needs T confirmations to count as confirmed.` | trustview_test TestTrustLeadSentence (built and literal) and TestCastVoteResultCarriesTrust, trust_paths_test, trust_e2e_test p07Lead |
| `This report has expired. It can no longer be confirmed.` | `Expired, so it can no longer be confirmed.` | trustview_test expiredLine, trust_paths_test, activity_e2e_test expiryLine |
| New reporter: `Only {J} of their reports have had a clear result so far. A rating needs 5 reports with a clear result.` (and J 1, and `None of their reports has had a clear result yet.` forms) | `A rating needs 5 reports with a clear result. They have {J} so far.` (J 1: `They have 1 so far.`, J 0: `They have none so far.`) | TestTrustCopyReporterIsPlainText and TestReporterSignalsNewAndNone (`needs` const lost its leading space and now comes first), TestReporterSignalsGroupByAccount (ids 105, 106), feed_test (J 2, J 0), trust_e2e_test p14NewReporter |
| `... based on how often their reports with a clear result were confirmed.` | `... based on how often their reports were confirmed.` | trustcopy_test (86), reporter_test scoreTail, trust_e2e_test (78) |

Structural edits the plan asked for:

- **TestTrustCounterSentences:** `tail` is " Counts places, not votes."; it is appended in the confirmed map only; the disputed map has no tail.
- **TestTrustCopyIsPlainText:** the one combined caveat check `!HasSuffix(c, tail) || !HasSuffix(d, tail)` ("a counter sentence for %d lost its caveat") was replaced by a stricter pair: the confirmed sentence must end with the caveat and contain it exactly once (`strings.Count(c, tail) != 1`), and the disputed sentence must not contain "Counts". The loop and the `assertPlainCopy` calls stay.
- `votes_e2e_test.go`, `feed_test.go` l.937 (S16), `trust_e2e_test.go` l.1105 (S16) and every web/ file are untouched.

## Green runs

- `go test ./internal/service/ -count=1`: ok (whole package, 18 RED tests and TestTrustModelDocMatchesCode included; no database, DATABASE_URL not set).
- `go test ./internal/service/ -run TestTrustModelDocMatchesCode -count=1`: ok after the TRUST-MODEL.md edit.
- `go build ./...`: pass. `go vet ./internal/... ./web/`: pass. `go test ./web/ -count=1`: ok.
- `gofmt -l` on the changed Go files and docs.go: nothing.
- Old fragment grep over internal/, docs/docs.go, swagger.json and swagger.yaml: no match. Same grep over TRUST-MODEL.md and 03-UI-SPEC.md outside the locked rows: no match.
- No em or en dash appears in any added line (checked per commit on the staged diff).

The plan precondition held: `grep -l 'TestMain\|NewTestDB' internal/service/*_test.go` printed nothing. The sandbox accepted the plain form of every command; the `env -u DATABASE_URL` prefix was not needed and no database, server, browser or port was used. The handlers package tests were not run.

## Deviations from Plan

None. The plan was executed as written. Anchors: all plan anchors were found as named (line numbers were hints only). One mechanical note: in 04-12-PLAN.md several lines begin with `    Step 1 (RED,`; the edit targeted the one that holds the old caveat text (about l.245), as the plan describes.

## Notes for the Phase 4 planner (not edited, outside this plan's four named edits)

04-12's Step 1 bans the Phase 3 fragments "gone quiet" and "no longer be confirmed" in the legal page code. The new Phase 3 sentences are "Gone quiet. ..." and "Expired, so it can no longer be confirmed.", so the second fragment still occurs in the Phase 3 expired sentence and the first differs only in capital letter. The plan only authorised the two replacement pairs it names there, so these two were left as they are.

## Live proof still owed (orchestrator, after merge)

The full gate on a scratch database (fail 0, skip 0) proves trust_e2e_test.go, activity_e2e_test.go and votes_e2e_test.go; then the browser harness shows the shorter panels. UAT Gap 3 stays open until the owner's look check.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, file access or schema change; the sentences are still built only from constants and strconv.Itoa (T-03-06 mitigated by the unchanged closed character set tests).

## Self-Check: PASSED

- Commits 1f88566, 072a3f4, 361cf54 and 08f5e61 exist on the worktree branch, each listing only its own paths.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched; nothing under .planning/sketches/, 03.1-* or 03.2-* was touched.
