---
phase: quick-261029-rtx
plan: 01
type: execute
wave: 1
depends_on: []
quick_id: 261029-rtx
created: 2026-10-29
source: Owner rule given 2026-10-29. The repository is public, so no tracked file and no commit message may carry the project purpose and audience wording the owner ruled out. The orchestrator built the hit list of every affected line and supplies the rewrite rules and the search at dispatch.
status: planned
files_modified:
  - .claude/CLAUDE.md
  - .planning/PROJECT.md
  - .planning/ROADMAP.md
  - .planning/phases/01-foundation-report-map/01-07-PLAN.md
  - .planning/phases/01-foundation-report-map/01-10-PLAN.md
  - .planning/phases/01-foundation-report-map/01-11-RESEARCH.md
  - .planning/phases/01-foundation-report-map/01-SECURITY.md
  - .planning/phases/01-foundation-report-map/01-SKELETON.md
  - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-05-PLAN.md
  - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md
  - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md
  - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-02-PLAN.md
  - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-03-PLAN.md
  - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-DESIGN-BRIEF.md
  - .planning/phases/03.3-installable-app-and-phone-alerts/03.3-REVIEW-FIX-SKEPTIC.md
  - .planning/phases/04-robustness-real-world-resilience/04-27-PLAN.md
  - .planning/phases/04-robustness-real-world-resilience/04-RESEARCH.md
  - .planning/phases/05-official-feed-demo-mode/05-20-PLAN.md
  - .planning/phases/05-official-feed-demo-mode/05-20-SUMMARY.md
  - .planning/phases/05-official-feed-demo-mode/05-RESEARCH.md
  - .planning/phases/05-official-feed-demo-mode/05-VERIFICATION.md
  - .planning/phases/07-address-search-box-for-report-location/07-RESEARCH.md
  - .planning/phases/07.1-password-sign-in-and-usernames/07.1-07-PLAN.md
  - .planning/phases/07.1-password-sign-in-and-usernames/07.1-RESEARCH.md
  - .planning/phases/08-report-page-and-comments/08-19-PLAN.md
  - .planning/quick/261013-wm5-activity-page-six-tabs-laptop-and-phone/261013-wm5-PLAN.md
  - .planning/research/ARCHITECTURE.md
  - .planning/research/FEATURES.md
  - .planning/research/PITFALLS.md
  - .planning/research/STACK.md
  - .planning/research/SUMMARY.md
  - .planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md
  - PROJECT-NOTES.md
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - internal/api/router.go
autonomous: true
requirements:
  - QUICK-261029-rtx
must_haves:
  truths:
    - "The orchestrator-supplied search over every tracked file returns nothing except the known test-name false positives the orchestrator lists"
    - "Every one of the 83 distinct hit-list lines (84 entries; one table row in 02-RESEARCH.md holds two occurrences) is reworded, and only the clause carrying the project purpose and audience wording changed; every technical fact, number, decision, outcome cell, confidence tag and source on those lines is kept"
    - "The Purpose constraint in .planning/PROJECT.md and in .claude/CLAUDE.md still says the project is not a startup and that scope favors architecture, testing and documented trust-model reasoning over business metrics or growth features"
    - "Every reworded line inside the CLAUDE.md generated project block and stack block reads word for word the same as its source text in .planning/PROJECT.md or .planning/research/STACK.md, so a later regeneration gives the same result"
    - "Every changed markdown table row keeps the same number of pipe characters it had before"
    - "The Swagger license name reads Unlicensed (all rights reserved) exactly once in internal/api/router.go, docs/docs.go, docs/swagger.json and docs/swagger.yaml, edited by hand with no swag regeneration"
    - "go build ./..., go vet ./... and go test -p 1 ./internal/api/... ./docs/... pass, the tests on the scratch database q_rtx only, which is dropped at the end"
    - "The executor's own commits together touch exactly the 37 hit-list files, and their messages, plus this task's SUMMARY, also pass the orchestrator-supplied search"
  artifacts:
    - path: internal/api/router.go
      provides: "the swag annotation that is the source of the published API license name"
      contains: "@license.name Unlicensed (all rights reserved)"
    - path: docs/swagger.yaml
      provides: "the published spec, hand edited to match the annotation"
      contains: "name: Unlicensed (all rights reserved)"
    - path: docs/swagger.json
      provides: "the published spec, hand edited to match the annotation"
      contains: "\"name\": \"Unlicensed (all rights reserved)\""
    - path: docs/docs.go
      provides: "the embedded spec template, hand edited to match the annotation"
      contains: "\"name\": \"Unlicensed (all rights reserved)\""
    - path: .planning/PROJECT.md
      provides: "the source of the CLAUDE.md project block, with the Purpose scope guidance kept"
      contains: "not a startup"
    - path: .claude/CLAUDE.md
      provides: "the generated project and stack blocks, mirroring PROJECT.md and research/STACK.md"
      contains: "not a startup"
  key_links:
    - from: "internal/api/router.go @license.name"
      to: "docs/docs.go, docs/swagger.json, docs/swagger.yaml"
      via: "make swag (swag init -g internal/api/router.go -o docs) regenerates the same value, so a later regeneration by another session keeps it"
      pattern: "Unlicensed \\(all rights reserved\\)"
    - from: ".planning/PROJECT.md Constraints section"
      to: ".claude/CLAUDE.md block between GSD:project-start and GSD:project-end"
      via: "generated copy; identical words keep a regeneration stable"
      pattern: "not a startup"
    - from: ".planning/research/STACK.md"
      to: ".claude/CLAUDE.md block between GSD:stack-start and GSD:stack-end"
      via: "generated copy; the three shared table rows are byte identical today and must stay so"
      pattern: "expires after 30 days"
---

<objective>
Rewrite the project purpose and audience wording on every line listed in the hit list, so the public repository describes the project neutrally, and change the published API license name to `Unlicensed (all rights reserved)`.

Purpose: the owner ruled on 2026-10-29 that no file in the public repository and no commit message may describe the project in those terms. The rule applies to everything now in the tree, including older phase docs and the generated Swagger spec.

Output: one docs commit (33 markdown files), one license commit (4 files), and the SUMMARY (not committed by the executor). There are no behavior changes.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
The hit list is at `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/db3bf814-f249-4cfb-b797-993a4951edb9/scratchpad/neutral-wording-hits.txt`. It lives outside the repository on purpose and must never be copied into it. Each line has the form `path:line:old text`. If you cannot read it, stop and report. Do not rebuild it from memory.

The orchestrator gives two things at dispatch:
- The rewrite rules. They win over the examples in Task 1 wherever they differ.
- The search command, plus the list of known test-name false positives. If the dispatch message has no search, stop and report rather than guessing one.

Do NOT read `.planning/STATE.md`. Another session holds an uncommitted edit there.

Facts the planner checked, so you do not need to re-read for them:
- Hit list totals: 84 entries, 83 distinct lines, 37 files (33 markdown files, plus internal/api/router.go, docs/docs.go, docs/swagger.json and docs/swagger.yaml).
- Some hit-list lines do not carry a ruled-out word themselves. They are the continuation of a sentence or bullet that does (for example the second line of the Purpose bullet). Reword them together with that sentence.
- `.claude/CLAUDE.md` has generated blocks:
  - Lines 1 to 54 sit between `<!-- GSD:project-start source:PROJECT.md -->` and `<!-- GSD:project-end -->`.
  - Lines 56 to 188 sit between `<!-- GSD:stack-start source:research/STACK.md -->` and `<!-- GSD:stack-end -->`.
  - Its four project-block hits pair with .planning/PROJECT.md: the two Purpose lines, the tech stack sentence and the Legal line.
  - Its three stack-block hits pair with .planning/research/STACK.md. Those are three table rows (the hosting row, the Perspective API row and the moderation alternatives row), and today they are byte identical in both files.
- The tech stack sentence already wraps at a different point in CLAUDE.md than in PROJECT.md. Identical words are required. Identical line breaks are not.
- `01-11-RESEARCH.md` quotes older CLAUDE.md and STACK.md text in blockquotes and backticks. Those quoted copies count, and are reworded the same way as their source.
- `05-20-PLAN.md`'s hit is inside its YAML frontmatter (a double quoted string), so keep the quoting valid.
- Several hits are rows of markdown tables: Key Decisions in PROJECT.md, threat registers, the research tables and the STACK.md rows.
- The Swagger license lines today: router.go line 492 (`// @license.name ...`), docs/docs.go line 17 and docs/swagger.json line 11 (`"name": "..."` inside `"license"`), and docs/swagger.yaml line 1903 (`    name: ...` under `  license:`, an unquoted plain scalar).
- CI does not regenerate or diff the swag output (the only swag call is the `swag` target in the Makefile).
- `testutil.NewTestDB` reads DATABASE_URL, applies the embedded migrations and truncates the tables, so no separate migrate step is needed. All packages must run with `-p 1`, because they share one database.
- `internal/api/handlers/swagger_test.go` reads docs/swagger.json, but no test checks the license.
</context>

<rules>
These project rules are binding. They come from the owner and the orchestrator.

Wording:
- The same wording rule applies to everything you write. That covers the edited files, this task's SUMMARY, every commit subject and body, and code comments.
- Never use the ruled-out words, and never quote any old line from the hit list.
- Describe the change by file and count only, for example "reworded 11 lines in .planning/PROJECT.md". Call the subject "the project purpose and audience wording".

Commits:
- Commits are path limited. Run `git add -- <paths>`, then `git commit -m "<subject>" -m "<body>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- <paths>`. Every commit ends with exactly that trailer line.
- Never `git add -A` or `-a`. Never `--no-verify`. Never push. Never amend, rebase, reset or rewrite history.
- If git reports an index.lock held by another process, wait a few seconds and retry once. Never delete the lock file.
- Do not commit the PLAN or the SUMMARY. The orchestrator commits the quick task's docs.

Files:
- Never touch .planning/STATE.md, .planning/REQUIREMENTS.md or any UAT file.
- Do not touch .planning/ROADMAP.md beyond its one hit-list line (in the Phase 5 section).
- Edit no file outside the 37 hit-list files, apart from writing this task's SUMMARY.

Docs:
- Do not run `swag init` or `make swag`. Other sessions regenerate docs/ wave by wave, so a full regeneration could pull in unrelated diffs.

Tools and characters:
- No Python at all. Use the Edit tool, perl or sed for edits, and `go run` for side work.
- No images or screenshots anywhere.
- No em or en dash in any new text you write. A dash already present on a line, outside the clause you change, may stay. If the clause you replace contains one, use a comma, a period or parentheses instead.

Database:
- Only the scratch database `q_rtx`. Never pinalert_test, pinalert, p31_4_uat or any other database.
- The sandbox refuses `env -i`. Before any go test, run `test -z "$DATABASE_URL" && echo "DATABASE_URL is unset"`, then put `DATABASE_URL='postgres://localhost:5432/q_rtx?sslmode=disable'` inline in front of that one command.
- No server and no port are needed for this task.
</rules>

<tasks>

<task type="auto">
  <name>Task 1: Reword the project purpose and audience wording in the 33 markdown files, then commit them</name>
  <files>.claude/CLAUDE.md, .planning/PROJECT.md, .planning/ROADMAP.md, .planning/research/ARCHITECTURE.md, .planning/research/FEATURES.md, .planning/research/PITFALLS.md, .planning/research/STACK.md, .planning/research/SUMMARY.md, PROJECT-NOTES.md, .planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md, .planning/quick/261013-wm5-activity-page-six-tabs-laptop-and-phone/261013-wm5-PLAN.md, and the 22 phase docs under .planning/phases/ listed in frontmatter files_modified</files>
  <action>
**Step 1: guard against another session's edits.** From the repository root, record the base commit with `git rev-parse HEAD` (for the SUMMARY). Then build `ALL=$(cut -d: -f1 <hit list path> | sort -u)` (37 paths) and run both `printf '%s\n' "$ALL" | xargs git diff --quiet --` and `printf '%s\n' "$ALL" | xargs git diff --cached --quiet --`. Use the xargs form, never a bare `$ALL`: zsh does not word-split it, and the guard would then always pass. The cached check is scoped to these paths on purpose. Another session's staged files elsewhere do not matter, because every commit here is path limited. If either check fails, stop and report the dirty paths. Do not edit or commit a file that another session has changed but not committed.

**Step 2: edit by matching content.**
- For each markdown file in the hit list, Read only the region around each listed line (offset and limit), then Edit that line.
- Find each line by its content, not by its number. Line numbers may have drifted since the hit list was made, and an earlier edit in the same file can shift later lines. If a listed line has moved, find it by its text. If it is gone, record that in the SUMMARY and move on.
- The one table row in 02-RESEARCH.md that holds two occurrences needs both fixed.

**Step 3: rewrite only the clause that carries the purpose or audience.** Keep everything else on the line: every fact, number, decision, outcome cell (Pending, Good and so on), confidence tag, source and link. Replace in place. Do not reflow whole paragraphs, though a line may get a little shorter or longer. The orchestrator's rewrite rules win. Where they are silent, use these neutral patterns:
- (a) A statement of what the project is or why it exists becomes a plain description: a solo engineering project, a personal project, or a project idea from a shortlist.
- (b) The purpose used as shorthand for size or traffic becomes plain scale words: this project's small scale, low traffic, single digit users, or demo-scale traffic.
- (c) A noun for the people who would evaluate the project becomes a neutral reader: a first-time visitor, a reader of the repository, a newcomer, or a new contributor.
- (d) A choice justified by how it would look to that audience is justified by clarity instead: keeps the engineering easy to read and test, or simple to explain in the design write-up.
- (e) A demo that must stay up for that audience becomes a demo meant to stay live indefinitely.
- (f) The first deploy named after the purpose becomes the first deploy, or a personal demo deploy.
- (g) The public-repository decision row in PROJECT.md gives a neutral reason: the code and the trust model write-up are open for anyone to read.
- (h) A line that records a literal string sent during research describes that string as a Pinalert research User-Agent with a contact placeholder, without repeating its old parenthetical.
- (i) A line that lists places where the live address might be shared becomes a neutral phrase, for example sharing the live address publicly.

**Step 4: apply the Purpose and mirror rules.**
- **Purpose bullet** (PROJECT.md and the CLAUDE.md project block). Per the orchestrator, the scope guidance must survive. The bullet must still say the project is not a startup, and that scope favors architecture, testing and documented trust-model reasoning over business metrics or growth features. Only the opening description changes.
- **Mirror rule.** Edit PROJECT.md first and give the matching CLAUDE.md project-block lines exactly the same words. Then edit research/STACK.md and give the three matching CLAUDE.md stack-block rows byte identical text. CLAUDE.md is a generated copy of those blocks, so a later regeneration must reproduce your text. Do not touch any CLAUDE.md line outside the seven hit-list lines.

**Step 5: keep the file formats valid.**
- **Tables.** Every changed table row keeps the same number of pipe characters. Never add a pipe inside a cell.
- **Frontmatter.** In 05-20-PLAN.md, keep the double quotes of the frontmatter string balanced.
- **01-11-RESEARCH.md.** Reword each quoted copy so it matches how its source line now reads. Keep the blockquote and backtick markers.

**Step 6: verify, then commit.** Run the verify commands below before committing. Then make one path limited commit of exactly the 33 markdown paths. Build the path list from the hit list, as the verify commands do. In zsh, pass it with `printf '%s\n' "$MD" | xargs git add --` and `printf '%s\n' "$MD" | xargs git commit -m "<subject>" -m "<body>" -m "<trailer>" --`, never as a bare `$MD`. The commit needs:
- Subject: `docs(261029-rtx): neutral project purpose wording`
- Body: one or two sentences saying the project purpose and audience wording was reworded in 33 docs, with no behavior change. Give no examples of old or new text.
- Trailer: the Co-Authored-By line from the rules.

Record the commit sha.
  </action>
  <verify>
    <automated>cd "$(git rev-parse --show-toplevel)" && H=/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/db3bf814-f249-4cfb-b797-993a4951edb9/scratchpad/neutral-wording-hits.txt && MD=$(cut -d: -f1 "$H" | sort -u | grep -v -e '^docs/' -e '^internal/') && test "$(printf '%s\n' "$MD" | wc -l | tr -d ' ')" = 33 && test "$(printf '%s\n' "$MD" | xargs git diff --name-only -- | wc -l | tr -d ' ')" -ge 30 && printf '%s\n' "$MD" | xargs git diff -U0 -- | awk '/^(---|\+\+\+) /{next} /^@@/{for(i=1;i<=r;i++) if(R[i]!=A[i]) bad++; r=0; a=0; next} /^-/{R[++r]=gsub(/\|/,"|")} /^\+/{A[++a]=gsub(/\|/,"|")} END{for(i=1;i<=r;i++) if(R[i]!=A[i]) bad++; print "pipe mismatches: " bad+0; exit (bad>0)}'</automated>
    <automated>cd "$(git rev-parse --show-toplevel)" && ( git diff -U0 -- .claude/CLAUDE.md | sed -n 's/^+[[:space:]]*\([^+[:space:]]\)/\1/p' | while IFS= read -r l; do cat .planning/PROJECT.md .planning/research/STACK.md | tr -s ' \n' '  ' | grep -qF -- "$(printf '%s' "$l" | tr -s ' ' ' ')" || { echo "CLAUDE.md line not mirrored in its source"; exit 1; }; done ) && ( for f in .claude/CLAUDE.md .planning/PROJECT.md; do t=$(awk '/^- \*\*Purpose\*\*/,/^- \*\*Timeline\*\*/' "$f" | tr -s ' \n' '  '); for w in 'not a startup' architecture testing trust 'growth features'; do printf '%s' "$t" | grep -qF -- "$w" || { echo "Purpose bullet in $f lost: $w"; exit 1; }; done; done ) && echo "mirror and purpose ok"</automated>
    <automated>cd "$(git rev-parse --show-toplevel)" && H=/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/db3bf814-f249-4cfb-b797-993a4951edb9/scratchpad/neutral-wording-hits.txt && MD=$(cut -d: -f1 "$H" | sort -u | grep -v -e '^docs/' -e '^internal/') && a=$(printf '%s\n' "$MD" | xargs git diff -U0 -- | LC_ALL=C grep -E -c $'^\\+[^+].*(\xe2\x80\x93|\xe2\x80\x94)'); r=$(printf '%s\n' "$MD" | xargs git diff -U0 -- | LC_ALL=C grep -E -c $'^-[^-].*(\xe2\x80\x93|\xe2\x80\x94)'); echo "dash lines added $a removed $r"; test "$a" -le "$r"</automated>
    Path lists go through `printf '%s\n' "$MD" | xargs`, because zsh does not word-split an unquoted variable, and a bare `$MD` would make git diff check one nonexistent path and pass without testing anything. The `-ge 30` guard proves the diff really sees the edited files: 33 are expected, and only a listed line that was already gone may lower the count. Then run the orchestrator-supplied search restricted to the 33 markdown paths. It must return nothing. All four checks run before the commit, because they read the uncommitted diff. After the commit, `git show --name-only --format= <docs sha> | sort` lists exactly the 33 markdown paths.
  </verify>
  <done>All 83 distinct hit-list lines in the 33 markdown files are reworded, or recorded as gone. The pipe, mirror, Purpose and dash checks pass. The search finds nothing in those files. One path limited docs commit holds exactly those 33 files.</done>
</task>

<task type="auto">
  <name>Task 2: Change the Swagger license name by hand in router.go and the three generated docs files, then commit them</name>
  <files>internal/api/router.go, docs/docs.go, docs/swagger.json, docs/swagger.yaml</files>
  <action>
**Guard.** Run `git diff --quiet -- internal/api/router.go docs/`. If it fails, another session has an uncommitted change or regeneration there. Stop and report. Do not overwrite it.

**Edit by hand.** Per the orchestrator, change only the license name value in each file to exactly `Unlicensed (all rights reserved)`. Use the Edit tool, keeping each file's existing syntax:
- In internal/api/router.go, change the `// @license.name` annotation line. Keep the tab alignment of the other annotation lines.
- In docs/docs.go, change the `"name"` value inside the `"license"` object of the doc template.
- In docs/swagger.json, change the `"name"` value inside `"license"`. Keep the indentation and the trailing structure.
- In docs/swagger.yaml, change the `name:` value under `license:`. Leave it an unquoted plain scalar, as swag writes it.

Do not run `swag init` or `make swag` (rules). The four values must be byte identical, because `make swag` would regenerate the three docs files from the router.go annotation.

**Format and build.** Then run `gofmt -l` on the two Go files and `go build ./...`.

**Commit.** Make one path limited commit of exactly these 4 files:
- Subject: `docs(261029-rtx): neutral API license name`
- Body: the Swagger license name now reads Unlicensed (all rights reserved) in the annotation and in the three generated docs files, edited by hand with no regeneration.
- Trailer: the Co-Authored-By line.

Record the sha.
  </action>
  <verify>
    <automated>cd "$(git rev-parse --show-toplevel)" && test -z "$DATABASE_URL" && echo "DATABASE_URL is unset" && for f in internal/api/router.go docs/docs.go docs/swagger.json docs/swagger.yaml; do test "$(grep -cF 'Unlicensed (all rights reserved)' "$f")" = 1 && test "$(grep -c 'Unlicensed (' "$f")" = 1 || { echo "license line wrong in $f"; exit 1; }; done && test -z "$(gofmt -l internal/api/router.go docs/docs.go)" && go build ./... && go test ./internal/api/handlers -run 'TestSwagger' -count=1</automated>
    After the commit, `git show --name-only --format= <license sha> | sort` lists exactly these 4 paths.
  </verify>
  <done>All four files carry the new license name exactly once, and no other line in them changed. gofmt and go build are clean, and the Swagger tests pass. One path limited commit holds exactly those 4 files.</done>
</task>

<task type="auto">
  <name>Task 3: Whole-tree search, build, vet, tests on q_rtx, commit-set check, then the SUMMARY</name>
  <files>.planning/quick/261029-rtx-neutral-wording-for-the-project-purpose-/261029-rtx-SUMMARY.md (written, not committed)</files>
  <action>
1. **Search.** Run the orchestrator-supplied search over every tracked file. It must return nothing except the known test-name false positives the orchestrator listed. Any other hit is a line the hit list missed or an edit that did not take:
   - If it is in a hit-list file, fix it, then make a further path limited docs commit with a neutral subject.
   - If it is in a file outside the hit list, do not edit it. Report it in the SUMMARY by path and line number only.

2. **Build and vet.** Run `go build ./...` and `go vet ./...`. Both must pass.

3. **Tests on the scratch database.**
   - Run `test -z "$DATABASE_URL" && echo "DATABASE_URL is unset"`.
   - Refuse if the database already exists: if `psql -lqt | cut -d'|' -f1 | grep -qw q_rtx` succeeds, stop and report. Otherwise run `createdb q_rtx`.
   - Run `DATABASE_URL='postgres://localhost:5432/q_rtx?sslmode=disable' go test -p 1 -count=1 ./internal/api/... ./docs/...`.
   - If a test fails, rerun that one test once. This task changes no behavior. So if it still fails, report it in the SUMMARY with its name and output, and do not change code for it.
   - Always run `dropdb q_rtx` at the end, also after a failure, and confirm it is gone.
   - Give the Bash call that runs createdb, the tests and dropdb the 600000 ms timeout. The default 120 s can cut off serialized database tests, and a killed call skips dropdb. If the call is killed anyway, run `dropdb q_rtx` as its own step before any retry, or the retry refuses because q_rtx exists.

4. **Commit set.** The executor's own commits together must touch exactly the 37 distinct hit-list files (the docs sha, the license sha and any fix sha), less any file recorded as already gone (see verify). Use `git show --name-only --format=` with those shas. Do not use a commit range, because another session may merge into the same branch meanwhile.

5. **Commit messages.** Run the orchestrator-supplied search against `git show -s --format=%B` of each of your shas. It must return nothing.

6. **SUMMARY.** Write the SUMMARY using the summary template and the wording rules. It covers:
   - The base sha and every commit sha.
   - The count of lines reworded per file, plus any listed line that was already gone.
   - The pipe, mirror, Purpose and dash check results.
   - The license change.
   - The build, vet and test results (pass, fail and skip counts).
   - Confirmation that q_rtx was dropped.
   - A note for other sessions: any session that regenerates docs/ with swag must start from a base that includes the license commit, or the old name returns.

   For the search false positives, give only how many there were and that they match the orchestrator's list. Do not name them, since they may contain a ruled-out word. Then run the orchestrator-supplied search on the SUMMARY file itself. It must return nothing.
  </action>
  <verify>
    <automated>cd "$(git rev-parse --show-toplevel)" && go build ./... && go vet ./... && test -z "$DATABASE_URL" && ! psql -lqt | cut -d'|' -f1 | grep -qw q_rtx && createdb q_rtx && { DATABASE_URL='postgres://localhost:5432/q_rtx?sslmode=disable' go test -p 1 -count=1 ./internal/api/... ./docs/...; s=$?; dropdb q_rtx; ! psql -lqt | cut -d'|' -f1 | grep -qw q_rtx && test $s = 0; }</automated>
    <automated>cd "$(git rev-parse --show-toplevel)" && diff <(git show --name-only --format= DOCS_SHA LICENSE_SHA | sed '/^$/d' | sort -u) <(cut -d: -f1 /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/db3bf814-f249-4cfb-b797-993a4951edb9/scratchpad/neutral-wording-hits.txt | sort -u) && echo "commit set matches the 37 hit-list files"</automated>
    In the second command, replace DOCS_SHA and LICENSE_SHA with your shas, and add any fix sha. A file whose every listed line was recorded as already gone in the SUMMARY appears in no commit. Remove such a file from the expected side (pipe the cut output through `grep -v -x -F` with that path), and name it in the SUMMARY. Any other difference is a failure. Then the orchestrator-supplied search runs three times: over every tracked file (only the known test-name false positives), over your commit messages, and over the SUMMARY. The last two must return nothing.
  </verify>
  <done>The tree search shows only the known false positives. Build, vet and the api and docs tests on q_rtx pass, and q_rtx is gone. The commits touch exactly the 37 hit-list files. Commit messages and the SUMMARY pass the search. The SUMMARY is written and not committed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| repository and commit history to the public | Everything committed is world readable on GitHub, including messages and planning docs |
| executor to the shared working tree | Another session builds Phase 9 and merges into the same branch, and holds uncommitted edits |
| executor to local databases | The owner's dev server database sits beside the scratch one, and the test helper truncates tables |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-rtx-01 | Information disclosure | commit messages, SUMMARY, reworded lines | high | mitigate | The orchestrator-supplied search runs over the tree, over every commit message of this task and over the SUMMARY (Task 3 steps 1, 5 and 6). Messages and the SUMMARY describe changes by file and count only and never quote an old line. |
| T-rtx-02 | Tampering | shared working tree and index | high | mitigate | A dirty-path guard runs before editing (Task 1 step 1, Task 2 guard). Commits are path limited with `git commit -- <paths>`, and git add -A is never used. The per-commit file set must equal the 37 hit-list files (Task 3 step 4). There is no push and no history rewrite. |
| T-rtx-03 | Tampering | Swagger docs drifting from the annotation | medium | mitigate | router.go and the three docs files change in one commit with byte identical values, each checked exactly once by grep. A SUMMARY note tells regenerating sessions to include this commit first. |
| T-rtx-04 | Denial of service | the owner's dev database | high | mitigate | Only q_rtx is used. The run refuses if q_rtx exists, sets DATABASE_URL inline for the one command, and drops q_rtx even on failure. pinalert_test is never used. |
| T-rtx-05 | Repudiation | meaning of historical decisions and records | low | mitigate | Only the purpose or audience clause changes. Facts, numbers and outcome cells are kept, and the pipe count and mirror checks guard tables and the generated CLAUDE.md blocks. |

No package is installed and no module is added.
</threat_model>

<verification>
- The orchestrator-supplied search over the tracked tree returns only the known test-name false positives. Over this task's commit messages and SUMMARY it returns nothing.
- `go build ./...` and `go vet ./...` pass. `go test -p 1 -count=1 ./internal/api/... ./docs/...` passes on q_rtx, which is dropped afterwards.
- The executor's commits touch exactly the 37 hit-list files. STATE.md, REQUIREMENTS.md and UAT files are untouched, and ROADMAP.md changes on its one hit-list line only.
- The pipe count, mirror, Purpose and dash checks pass.
</verification>

<success_criteria>
- No tracked file describes the project purpose or audience in the ruled-out terms.
- The Purpose constraint keeps its scope guidance: not a startup, with architecture, testing and documented trust reasoning before growth features.
- The CLAUDE.md generated blocks match their sources word for word on every reworded line.
- The published API license name reads Unlicensed (all rights reserved) in all four places.
- The build, vet and tests are green, and the scratch database is gone.
</success_criteria>

<output>
Create `.planning/quick/261029-rtx-neutral-wording-for-the-project-purpose-/261029-rtx-SUMMARY.md` when done (contents listed in Task 3 step 6). Do not commit it. The orchestrator commits the quick task's docs.
</output>
