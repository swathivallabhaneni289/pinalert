---
phase: 08-report-page-and-comments
plan: 19
subsystem: docs
tags: [swagger, trust-model, moderation, push, readme, policy]
requires:
  - phase: 08-01 to 08-18
    provides: the routes, fields and rules the documents describe
provides:
  - "docs/ regenerated once with swag v1.16.4, every Phase 8 path in the spec"
  - "TestSwaggerSpecCoversRoutes extended to the Phase 8 routes, answers and enums"
  - "TRUST-MODEL.md sections Report history, Comments and likes, Sharing"
  - "MODERATION.md Comments section, new limits rows, retention text"
  - "PUSH.md comment note rule, new Settings label, migrations 00017 to 00019"
  - "README.md report page, sharing, https BASE_URL, public page limit"
  - "Policy lists what comments and share links keep"
key-files:
  modified:
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - internal/api/handlers/swagger_test.go
    - internal/api/handlers/comments.go
    - internal/api/handlers/reports.go
    - docs/TRUST-MODEL.md
    - internal/service/trust_model_doc_test.go
    - docs/MODERATION.md
    - docs/PUSH.md
    - README.md
    - web/templates/legal_body.html.tmpl
requirements-completed: []
status: complete
completed: 2026-10-29
---

# Phase 8 Plan 19: Docs and API reference Summary

The published API reference, the trust model, the moderation notes, the push notes, the README and the Policy now describe Phase 8 as built.

## Swag version

`make swag` ran with swag v1.16.4 (the local binary; CLAUDE.md lists v1.16.6). Run twice: once for the whole docs/ diff, once after the fix below. Phases 5 and 7.1 left nothing unregenerated that showed in the diff beyond reordering of paths.

## Commits

- 0fb6fe13 docs(08-19): regenerate the API reference and pin the Phase 8 routes
- 0ab743c7 fix(08-19): keep the word withheld out of the published API reference
- 435fcd1a docs(08-19): trust model, moderation and push text for the report page
- 378a1537 docs(08-19): README and Policy name what comments and sharing keep

## What changed

- Spec: timeline, share, comments (get, post), comment like, unlike, delete and flag, the two comment kinds in the updates kind enum, `comment_count` and `comments_open`. The moderator comment routes are left out on purpose (08-13). `/r/{token}` is HTML and stays out. The 7.1 absence assertion for `/auth/request-link` is kept.
- Drift guard: a table over each path, method and answer (401 on all, 404, 409, 422, 429 with Retry-After where they apply), the updates kind enum in full, the two feed fields, and an assertion that the moderator comment paths are absent. Dated note "Amended 2026-10-29 by Phase 8 plan 19".
- TRUST-MODEL.md: history built from the vote log, upward steps only, trust block is the current truth after a take back; comments and likes never feed a number (`TestLikesNeverFeedTrust`); anyone signed in may comment from anywhere and why that is safe; the Reporter tag linkability as an accepted risk (D-02); sharing by random token, no count in the preview, area name now also in the share title and public page (amends 03.3 D-19 and D-20 for those two surfaces); new Settings label; comment notes follow the fading note's rule (active Live), one per report per device, silent. `p20MustCite` gained `TestTimelineMatchesTrust`, `TestLikesNeverFeedTrust`, `TestShareOpenGraph`, `TestSharePageSetsNoCookie`.
- MODERATION.md: Comments section, three new bucket rows and three backstop rows, retention text.
- PUSH.md: comment phrases, the shared predicate, area name line amended, new label, migrations 00017 to 00019 in the owner step.
- README.md: BASE_URL roots share links and og:url and must be https outside development; the report page and sharing in plain words; no per address limit on the public page without `CLIENT_IP_HEADER`; migrations 00017 to 00019 (the 00012 to 00016 text is kept).

## Orchestrator addition

`web/templates/legal_body.html.tmpl` (the Policy): four lines in "What Pinalert keeps" (comments and replies, likes and comment flags with notes, delete and account deletion effects, the share link and what it shows). Beyond the list, I also added one paragraph under "How reports are checked" saying comments go through the same check, and put "comment, like" into the Limits sentence, because both would otherwise be false. "Last updated" and `service.CurrentTermsVersion` were not touched. No Policy pin test lists these lines, so none was amended; the section headings are unchanged.

## Deviations from Plan

**1. [Rule 1 - Bug] The word withheld reached the published spec**
- **Found during:** full test run, `TestSubmitScreeningSwaggerFields` (it bans internal moderation words in docs/swagger.json).
- **Issue:** two swag descriptions written in 08-10 and 08-12 said "withheld" (the comment list and the `comments_open` field).
- **Fix:** reworded in `comments.go` and `reports.go`, regenerated. Commit 0ab743c7.

## Verification

- `go build ./...`, `go vet ./...` clean.
- `go test ./... -p 1 -count=1 -v` on the disposable database p8_19_a: exit 0, 2168 top level PASS, 0 FAIL, 20 packages ok, only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped. p8_19_a dropped.
- No em or en dash and no non ASCII character added. The one touched line that says "no sound and no vibration" was already there.

## Known Stubs

None.

## Self-Check: PASSED

- All four commits exist on the worktree branch; the changed files exist.
