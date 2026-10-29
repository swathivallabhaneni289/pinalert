---
phase: quick-261030-5my
plan: 01
subsystem: api-docs
tags: [swagger, punctuation, contract-test]
requires: []
provides:
  - Plain punctuation in the generated API reference (docs/docs.go, docs/swagger.json, docs/swagger.yaml)
  - TestGeneratedAPIReferenceUsesPlainPunctuation standing test
affects: [web/design_rules_contract_test.go, internal/api/router.go, internal/api/handlers/reports.go, docs/]
key-files:
  modified:
    - web/design_rules_contract_test.go
    - internal/api/router.go
    - internal/api/handlers/reports.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - .planning/ROADMAP.md
status: complete
completed: 2026-10-30
---

# Quick 261030-5my: Plain punctuation in the generated API reference

Three Go doc comments that swag copies into the public /swagger reference now use a colon or a full stop, the docs were regenerated with `make swag`, and a standing test scans the three generated files so a long dash cannot return. Backlog 999.8 is closed.

## What changed

- `internal/api/router.go` line 502: "Swagger 2.0: swag (the generator ..." (colon).
- `internal/api/handlers/reports.go` line 145: "report: confirm, dispute, or null." and line 152: "rendering a button that will 403. That is a convenience for the client,".
- `make swag` (swag v1.16.4, matching go.mod): `git diff --numstat -- docs/` showed 3 insertions and 3 deletions in each of the three files, nothing else. The license name line is untouched and appears once in router.go and once in each generated file.
- `web/design_rules_contract_test.go`: new `TestGeneratedAPIReferenceUsesPlainPunctuation`, placed after `TestUserVisibleCopyUsesPlainPunctuation`. It reads `../docs/docs.go`, `../docs/swagger.json` and `../docs/swagger.yaml`, fails with Fatalf on a read error or when the title "Pinalert API" is missing, and reports each line with a long dash. Needles are built with `string(rune(0x2014))` and `string(rune(0x2013))`, so the file gained no literal dash character (the byte escape grep count stays 2, the two existing constants).
- `.planning/ROADMAP.md`: the 999.8 heading now reads "(DONE as quick task 261030-5my, 2026-10-30)", body untouched.

## Verification

- RED before the fix: the new test reported exactly 9 lines (3 per generated file).
- GREEN after the fix: the byte escape grep finds no dash in the three generated files.
- `go test -count=1 ./web/` passes (whole package, no database).
- `go test -short -count=1 ./internal/api/handlers/ -run 'Swagger|ChangedHeader'` passes.
- `gofmt -l` on the four touched Go files is empty; `go build ./...` and `go vet ./web/ ./internal/api/... ./docs/` pass.
- Each rewritten sentence appears once per generated file; "Unlicensed (all rights reserved)" appears once in each of the four files.
- The wording grep (git grep for the forbidden project-purpose words) prints nothing.
- No database was used, no server started, DATABASE_URL and PGDATABASE were empty.

## Commits

- a8d43e70: docs(261030-5my): plain punctuation in the generated API reference (six task files)
- 53daf749: docs(261030-5my): mark backlog 999.8 done (.planning/ROADMAP.md, one line)

## Deviations from Plan

- The plan's verify commands use `cd` into the main checkout path; they were run from the worktree instead, with the same checks.
- Per the orchestrator's instruction, this SUMMARY is committed alone as the last commit, and STATE.md, ROADMAP progress tools and REQUIREMENTS were not touched.
- No other deviations. No stubs, no new threat surface.
