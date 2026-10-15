---
phase: 04-robustness-real-world-resilience
plan: 27
subsystem: docs-and-gates
tags: [contract-tests, byte-budget, docs, launch-checklist, regeneration-check]
requires:
  - phase: 04-robustness-real-world-resilience
    provides: plans 04-01 to 04-26, 04-28 and 04-29 (all merged on the base)
provides:
  - TestPhase4NoBareLabels and TestPhase4NoBannedViewWords over the 18 files Phase 4 added
  - TestLowDataByteBudget with a measured budget (509094 bytes measured, 585728 bytes budget)
  - docs/MODERATION.md, the README launch checklist and Run locally lines
  - Phase 1 and Phase 3 contract amendments, sketch findings Flag this report rows
affects: [phase 4 verification, phase 5 seeded data]
tech-stack:
  added: []
  patterns: [external test package web_test beside package web to import handlers]
key-files:
  created:
    - web/phase4_gates_contract_test.go
    - web/low_budget_contract_test.go
    - docs/MODERATION.md
  modified:
    - README.md
    - .planning/phases/01-foundation-report-map/01-UI-SPEC.md
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .claude/skills/sketch-findings-pinalert/references/popup-structure.md
    - .claude/skills/sketch-findings-pinalert/sources/REVIEWS.md
key-decisions:
  - "Raw bytes are budgeted (measured total plus 15 percent, rounded up to 1024), not compressed bytes"
  - "docs/TRUST-MODEL.md left untouched because the sentence the plan extends does not exist on the base"
requirements-completed: []
status: complete
duration: about 2 hours
completed: 2026-10-16
---

# Phase 4 Plan 27: Close the phase Summary

Two source gates over the 18 files Phase 4 added, a measured and pinned Low data byte budget, the moderation design record, the launch checklist and the earlier contract amendments are done and committed. Everything that needs the owner (the fixture recording, the V-24 walk, the numbers check, the legal read, the host canary) is listed under Owner checks and is not done.

Status note: the plan's Task 3 owner steps are open by design, and one Task 2 item (docs/TRUST-MODEL.md) is blocked by a failed Step 0 precondition. Tasks 1 and 2 (minus that item) and Task 3 parts A and B are complete.

## Commits

| Task | Commit | What |
| ---- | ------ | ---- |
| 1 | 96ab4c0 | `web/phase4_gates_contract_test.go`, `web/low_budget_contract_test.go` |
| 2 | 3f6701d | `docs/MODERATION.md`, README, 01-UI-SPEC, 03-UI-SPEC, popup-structure.md, REVIEWS.md |
| 2 follow up | 8e6c1f8 | `docs/MODERATION.md`: moderator page script order (asked for by 04-26) and the flag token note |
| SUMMARY | the commit that adds this file | this file |

Task 3 made no repository change: the regeneration checks and the suite runs write nothing, and the recording is the owner's.

## Step 0 records

- BASE: `5b9e51e5493711226ca994624eeb451efed5ceaa`.
- FIRST04: `ee7705c` (`test(04-03): add failing PerKey reserve and refund tests`), found with `git log --reverse --format='%h %s' -E --grep='^[a-z]+\(04-[0-9][0-9]\)'`. The plain `(04-` grep matches docs commits of the planner and Phase 3 commits first, so it was tightened to the Conventional Commit form.
- The range `ee7705c^..HEAD` over `web/templates web/static/js web/static/css` with `--diff-filter=A` lists exactly the 18 paths in `p427Phase4Files`. No extra Phase 4 file.
- The same range over `web/static/icons` lists nothing: Phase 4 added no icon file (P4-A14).
- light or lite whole word hits in the 18 files, comments included: exactly one, `templates/policy.html.tmpl:56` `<li>Your Light or Dark choice.</li>`, the allowlisted phrase. No hit in any comment and none in a `.css` file, so the three CSS theme syntax substrings are unused by this set.
- README shape: `## Run locally` with its export block, and the numbered list after `Before sharing a deployed instance with real visitors:` (items 1 and 2). No separate launch checklist section exists, so the new items continue that list as 3 to 8.
- `handlers.PageConfig.VoteRadiusKm` exists. `p427RenderLowPage` uses the same literal as `TestLowPageRenders` (04-18), which has no further field.
- Phase 3.2 bell: `/low` links no file beyond the 7 stylesheets, 10 scripts and the icons named by the plan; anything it later adds is counted automatically.
- The accent items in 03-UI-SPEC are at lines 321 and 322 and match the plan's anchors; the A60, A65 and A68 rows, the `| Modal cancel |` row, the `| Actions |` row and the REVIEWS JavaScript group all exist exactly as described.

## Low data byte budget (D-14)

Measured 2026-10-16 with `go test ./web/ -run TestLowDataByteBudget -v` after 04-24 and 04-25:

| Part | Bytes |
| ---- | ----- |
| rendered `/low` HTML | 10338 |
| stylesheets (7 files) | 178817 |
| scripts (10 files) | 308607 |
| icons (25 files, each counted once) | 11332 |
| other | 0 |
| total | 509094 |
| gzip level 5, same files | 153048 (informational) |
| budget (total plus 15 percent, rounded up to 1024) | 585728 |

Finding for the owner: the total is above RESEARCH's 256 KB starting guess (262144 bytes), by about 247 KB. Nothing was dropped or split, as the plan requires. The five largest files: `static/js/votes.js` 81123, `static/css/trust.css` 63693, `static/js/app.js` 45625, `static/js/report-form.js` 41375, `static/css/main.css` 37271. The scripts alone are 308607 raw bytes, so whether a smaller Low data view is wanted is a product decision for a later phase. Compressed with gzip level 5, the same files are about 153 KB.

The `/low` page links 25 of the 26 icon files in `web/static/icons`; `message-square.svg` is not referenced by it. No icon file was dropped to meet the budget.

## Docs values where the code differed from the plan (code wins)

- The recorder paces itself at 150 requests and 6,000 characters a minute (`recorderRequestsPerMinute`, `recorderRunesPerMinute`), not "under 10,000 tokens a minute". `docs/MODERATION.md` states the code's figures.
- The constants block in `cmd/server/main.go` holds 14 names, not 13: the vote change bucket also has `voteChangeBurst`. The docs table lists all 14.
- Flags give nothing back (04-20 decision 1), while report posts refund their allowance on every outcome except a stored report. The docs Limits section says both.
- Fixture counts: 200 genuine entries (40 each in en, hi, hinglish, ta, te) and 40 abusive. So the accuracy sentence reads "under about 3 in 200 (about 1.5 percent)".
- `docs/MODERATION.md` gibberish rule: one letter 10 or more times in a row, or fewer than two words of two or more letters (from `IsGibberish`).

## Deviations from Plan

### Not done: Task 2 item 7, `docs/TRUST-MODEL.md` (Step 0 failed, stopped by the rule)

Step 0 of item 7 requires `grep -c -F 'The two readings differ only for' docs/TRUST-MODEL.md` to print 1. It prints 0. The headings `### Outcomes` and `### Window, half life and the 30 report cap` each exist once. As built by Phase 3 plan 03-10, the cap section no longer has the "two readings" wording: it describes one rule (drop neutral reports first, then keep the latest 30 judged) with a bullet "The limit that remains: 100 neutral reports in a row can still push an old record out of the read, so the Phase 4 rate limits are what bound that."

Because the plan says to stop and report instead of improvising, `docs/TRUST-MODEL.md` is unchanged and the `trust-model-ok` automated check from Task 2 does not pass. Suggested text for the orchestrator or a gap closure (the R-02 fact is true in the code and is covered by `TestModeratedReportsAreNeutralForRating` from 04-29):

- Under `### Outcomes`, after the last bullet of "Phase 3.1 reuses this classifier and adds no rule to it", add a bullet: "Under Phase 4 R-02, a held report (pending, not serious) and a removed report are neutral whatever their votes: the rating never counts a report for or against its author because of a moderation outcome, and `ClassifyOutcome` returns neutral first when `ReportMeta.Withheld()` is true."
- Edit two of the plan has no sentence to extend. If wanted, extend the "limit that remains" bullet to say that held and removed reports are neutral too and so count toward those 100 neutral rows (04-29 records this as its T-04-16 residual).
- Note `TestTrustModelDocMatchesCode` (in `internal/service/trust_model_doc_test.go`) reads that file; run it after any edit.

### Process notes, no behaviour change

- The plan's verify commands use `env -u DATABASE_URL` and `$(...)` one liners, which the sandbox refuses. `DATABASE_URL` is unset in the shell, so plain `go test` is the same run. The four Task 2 shell checks were run as separate plain greps (results below).
- An extra test, `TestP427HelpersSelfCheck`, in `web/low_budget_contract_test.go` checks `p427BudgetFor`, the url target parser (including the media block and the inert globe rule) and the link parser. It is not in the plan.
- A context usage hook repeatedly reported "92% used" while the harness counter showed about 14.8 million tokens left; it had no effect on the work.

Auto-fix rules 1 to 3: none triggered.

## Verification results

- Task 1: `go vet ./web/` clean, `gofmt -l` clean on both new files, and these pass: `TestPhase4NoBareLabels`, `TestPhase4NoBannedViewWords`, `TestLowDataByteBudget`, `TestP427HelpersSelfCheck`, `TestLowDataAssetPolicy`, `TestLowDataNoBannedWords`, `TestNoMarkupSinksAnywhere`, `TestUserVisibleCopyUsesPlainPunctuation`. The gates passed at once on the 18 files, so no other plan's file needed a fix and no allowlist entry was added beyond the Policy page phrase.
- Task 2 checks, as plain greps: docs headings 11 of 11; zero U+2014 or U+2013 in `docs/MODERATION.md` and in the lines added to README; zero whole words light or lite in the doc; `Thresholds in use: RejectScoreThreshold 0.85, BorderlineScoreThreshold 0.40` matches `screening.go`; the numbers `509094 bytes` and `585728 bytes` appear in the doc; all 13 plan constant names appear; README counts (`CONTACT_EMAIL` outside `NOMINATIM_`: 3, each `EMERGENCY_*`: 2, `CLIENT_IP_HEADER`: 4, `MODERATOR_EMAILS`: 3, `OPENAI_API_KEY`: 3, `MODERATION_MODEL`: 1, `docs/MODERATION.md`: 2) and one README line holds both `CLIENT_IP_HEADER` and `place search`. The contract edits were made with unique matches. `trust-model-ok` not run (item 7 skipped).
- Regeneration (Task 3 A): `sqlc diff` printed nothing and exited 0; `swag init -g internal/api/router.go -o <scratch>/docs` produced `docs.go`, `swagger.json` and `swagger.yaml` identical to the committed files; `git status` showed no change under `internal/store/sqlc` or the docs files.
- Full suite (Task 3 B) on the disposable `p4_0427` (created fresh, dropped afterwards; `psql -lqt | grep -c -w p4_0427` printed 0): `go build ./...` and `go vet ./...` clean; `DATABASE_URL=postgres://localhost:5432/p4_0427?sslmode=disable go test ./... -count=1 -p 1 -v`: every package `ok`, no FAIL. The only `--- SKIP` is `TestScreeningFixture` (by design until the owner records; not database backed), so the database backed SKIP count is 0.
- Race subset on `p4_0427`: `TestPerKeyConcurrentBound`, `TestPerKeyRefundAtReservationInstant`, `TestSubmitIdempotentConcurrent`, `TestInsertReportIdempotentConcurrent`, `TestSubmitIdempotentReplayCostsNoAllowance`, `TestReportRateLimitRefunds`, `TestVoteRateLimit` and `TestFlagRateLimit` all `--- PASS` with `-race` in `internal/ratelimit`, `internal/api/handlers` and `internal/store`.
- Fixture state before recording: `TestScreeningFixtureShape`, `TestDecide`, `TestDecideIgnoresSeverity`, `TestDecideThresholdsOrdered` PASS; `TestScreeningFixture` SKIP.
- The known flaky `TestCastVoteConcurrentSameAccountKeepsEveryRow` did not fail in any run.
- Quick run `go test ./... -count=1` with `DATABASE_URL` unset passes for every package, before and after Task 2.

## Owner checks

Nothing below was run by this plan. No server was started, no browser, no recorder.

### 1. Record the fixture scores (D-07, T-04-04), human check 5

On your own machine, from the repository root. Create the disposable database first only if you also run the UAT (the recorder itself needs no database).

```bash
read -rs OPENAI_API_KEY
MODERATION_LIVE=1 OPENAI_API_KEY="$OPENAI_API_KEY" go run ./cmd/modrecord
unset OPENAI_API_KEY
```

Look for: a final line `modrecord: recorded N entries, M already recorded` (finished) or a line naming `insufficient_quota` (this key has no usable quota; that answers the zero balance question). Then:

- If it finished, the diff filter must print nothing:
  `git diff -U0 -- internal/service/testdata/screening/genuine.json internal/service/testdata/screening/abusive.json | grep -E '^[-+]' | grep -v -E '^(---|\+\+\+) ' | grep -v -E '^[-+] *("model": "[^"]*",?|"scores": \{\}?,?|"[a-z/_-]+": -?[0-9.]+([eE][-+]?[0-9]+)?,?|\},?)$'`
  and `git diff -- internal/service/testdata/screening/ | grep -c -E 'sk-[A-Za-z0-9_-]{20,}'` must print 0.
- Then `go test ./internal/service/ -run '^TestScreeningFixture$' -count=1 -v` must show `--- PASS: TestScreeningFixture` and no `--- SKIP`.
- Branches (plan Task 3 section C): complete and passing, replace the `Recording status:` line in `docs/MODERATION.md` (date, `omni-moderation-2024-09-26`, 200 genuine and 40 abusive entries, thresholds unchanged), rerun the docs check and commit the two JSON files plus the doc with `git commit -- <paths>`; complete but failing on thresholds, change only `RejectScoreThreshold` and `BorderlineScoreThreshold` in `internal/service/screening.go` (0 < borderline < reject < 1), rerun `TestDecide`, `TestDecideIgnoresSeverity`, `TestDecideThresholdsOrdered` and `TestScreeningFixture`, update both doc lines, commit those four paths; stopped early, restore the two JSON files with `git restore --source=HEAD -- internal/service/testdata/screening/genuine.json internal/service/testdata/screening/abusive.json`, set the `Recording status:` line to the stop reason and commit only the doc. Never edit fixture text, `serious` or `expect`, and never commit a partial recording.
- Hold the recording if the optional fluent reader skim in 04-28 asks for any `hi`, `te` or `ta` entry to be reworded (see item 3).

### 2. Government numbers, legal read, host canary (D-22, D-23, D-10), human checks 6 to 8

- Numbers: check 112, the older numbers 100, 101 and 108, and the 1070 and 1077 wording on 112.gov.in and your state disaster management authority's site. If anything differs, change only `EMERGENCY_PRIMARY`, `EMERGENCY_OLDER` or `EMERGENCY_STATE_LINES` in the deployment environment. Record date, pages and outcome. Status now: open.
- Legal read: a lawyer or mentor reads the Disclaimer and Policy pages before any wide promotion. Status now: open (gates promotion, not the phase).
- Host canary: once a host is chosen, send one canary request through it to learn which header carries the visitor address before setting `CLIENT_IP_HEADER`. Status now: open (no host chosen).

### 3. The V-24 walk on one disposable database (human check, steps 0 to 9 of the plan)

Setup: `dropdb --if-exists p4_0427; createdb p4_0427`, then `DATABASE_URL="postgres://localhost:5432/p4_0427?sslmode=disable" make migrate`, then start with `PORT=8792 BASE_URL=http://127.0.0.1:8792 ENV=development DATABASE_URL="postgres://localhost:5432/p4_0427?sslmode=disable" MODERATOR_EMAILS="<your test inbox>" make run` (CONTACT_EMAIL, CLIENT_IP_HEADER, OPENAI_API_KEY unset; the log shows the development WARNING). Sign in account A (the moderator address) and account B in two browser profiles through the dev mailer links in the server log. Port 8792 only; keep the laptop lid open; save no screenshots. Teardown: stop the server and `dropdb --if-exists p4_0427`. Record every failed step with its plan id.

Then walk, in plan order, reading every borrowed block's database as `p4_0427`, its port as 8792, and skipping any createdb, dropdb, migrate, server start or stop line inside it (the plan's section 2 of the human check explains this). Each block is in the plan file named here:

| Plan | Block (line in `04-NN-PLAN.md`) | Gist |
| ---- | ------------------------------ | ---- |
| 04-04 | 325 | recorder; skip here, it is item 1 above |
| 04-06 | 271 | unofficial bar and numbers on every full page, in Light and Dark |
| 04-08 | 178 | geometry check: at 320x568 and 1280x800 on the map and Activity pages the account trigger sits fully below the bar with a 16px gap, and the map and login pages do not scroll at default text size |
| 04-09 | 569 | network backstop (configuration C: restart with `CLIENT_IP_HEADER` as the block says, then without it) |
| 04-10 | 293 | report form hint, map picker modal |
| 04-11 | 391 | submit screening, held and rejected reports |
| 04-12 | 341 | Disclaimer and Policy pages; with `CONTACT_EMAIL=contact@example.com` for this session only (configuration B), contact blocks appear, then restart without it and they are gone |
| 04-13 | 310 and 363 | slow connection notice |
| 04-14 | 227 | worker retry at end of phase |
| 04-16 | 315 | Activity page held and removed cards |
| 04-17 | 392 | idempotent submit and the Written line (compact row and popup wrap at 320px) |
| 04-18 | 393 | Low data view |
| 04-19 | 249, 302 and 376 | Low data posting, voting and the real phone block |
| 04-20 | 373 | Flag route |
| 04-21 | 235, 284 and 325 | account menu and the Low data default |
| 04-22 | 292, 363 and 419 | offline queue |
| 04-23 | 364 | moderator routes |
| 04-24 | 416 | Flag dialog |
| 04-25 | 290 | queue strip on the Activity page |
| 04-26 | 305 | moderator screen client (phone widths, Telugu and Hindi text, offline Refresh) |
| 04-28 | 257 | optional: a fluent reader skims ten random fixture entries; if any `hi`, `te` or `ta` entry needs rewording, record the ids, send them to gap closure for 04-28 and hold item 1 |

Also walk the UI-SPEC "Manual checks (V-42 additions)" paragraph in full, in Light and Dark, at the sizes it names. Where UI-SPEC copy differs from the owner decision of 2026-10-08 (04-CONTEXT D-05 note), the owner's strings win: the gibberish refusal reads "Say what is happening so people nearby can help, for example: flood on Main Road, three people on a roof." and both report forms show the hint "Describe what is happening, for example: water is knee deep on Main Road and three people need help."

### 4. Real phone checks

The last 04-19 block (line 376) asks for a real phone on the Low data view. Follow that block as written. The standing rule is phone first: judge every layout at 320, 360 and 390 px wide before laptop width.

### 5. Product decisions that earlier plans recorded for you to veto or decide

- 04-02: the gibberish rule counts only letter runs, so ten exclamation marks, a row of emoji or a phone number are never a run (first owner veto item).
- 04-07: a voted report that resolves to withheld is dropped from the Activity votes list; only its author sees it.
- 04-11 and 04-29 (T-04-16): a report whose check row could not be written has no automatic recovery (it stays held, or stays published if serious), and 100 neutral rows in a row can push an old rating record out of the read. Ask 04-14 or 04-23 to also pick such reports up if wanted.
- 04-14: no attempt count cap for transient errors (the report's expiry is the bound); quota and permanent errors are never retried and the report stays in the moderator Pending list.
- 04-16: a held or removed own post is counted under the "Not resolved" tab on Activity, and its chip shows the severity word. Which tab it belongs to is an owner decision.
- 04-17: spacing of the Written line is not styled by that plan; a tweak would be a later plan.
- 04-20: a flag that ends in 400, 404 or 409 still uses a token; expired reports can be flagged; an author's own withheld report answers 404.
- 04-23: the moderator queue does not list a pending report that has no check row (Find by id still returns it, Remove works, Keep answers 409); the audit row is written before the state change; two moderators acting at once may write two audit rows.
- 04-25: the Activity page supplies its own focus fallback for the queue strip.
- 04-29: R-02 rule is one line in `ClassifyOutcome`; veto it to make every pending report neutral.

## Known Stubs

None. The only file with no content to read yet is the fixture scores, which are empty by design until the owner records.

## Threat Flags

None. This plan adds no route, handler, table, query, template, CSS rule or server string. The README and docs add settings guidance only; examples use `yourdomain.com` and `example.com`, and no key appears anywhere.

## Self-Check: PASSED (with one open item)

Created files exist (`web/phase4_gates_contract_test.go`, `web/low_budget_contract_test.go`, `docs/MODERATION.md`) and commits `96ab4c0`, `3f6701d` and `8e6c1f8` exist on `worktree-agent-ae16f8b4a2e8afd30`. The open item is Task 2 item 7 (`docs/TRUST-MODEL.md`), skipped on a failed Step 0 precondition and described above, plus the owner steps, which are not part of what this run could do.
