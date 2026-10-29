---
phase: 09
part: templates
fixed_at: 2026-10-30
review_path: .planning/phases/09-followable-reporter-profiles/09-REVIEW-templates.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 9: Code Review Fix Report, templates

Base commit c2014ea9. IN-T01 belongs to the server fixer by the triage and is not in this part.

## Fixed

### WR-T01: Forced colours for the Phase 9 mask glyphs
**Commit:** 6037997d
**Files:** `web/static/css/person.css`, `web/static/css/stars.css`, `web/review34_part3_contract_test.go`
The bell, the More chevron, the stars total and the five star control opt out of forced colour adjustment and take ButtonText or CanvasText. The four glyphs are in `TestForcedColoursShowsTheMaskGlyphs`, which failed before the CSS change.

### WR-T02: Contrast of the Recent reports column heads and the hovered row line
**Commit:** ff089261
**Files:** `web/static/css/person.css`, `web/person_contract_test.go`
The column heads take `--color-text`, and a hovered live row's line takes `--color-text`, so muted text never sits on `--color-surface`. `TestPersonColumnHeadsAndHoverKeepContrast` pins both and computes `--color-text` on `--color-surface` from main.css in the light and dark themes (both above 4.5 to 1).

### WR-T03: The Policy names the Name check by OpenAI
**Commit:** d487b064
**Files:** `web/templates/legal_body.html.tmpl`, `internal/api/handlers/legal_e2e_test.go`, `web/review71_templates_fix_contract_test.go`
A new paragraph in "How reports are checked" says the Name is checked when Let others follow me goes on and on each edit while it is on, that only the name is sent, and that a refused or unfinished name is not shown. The services list now reads "Report, comment, username and name checks: OpenAI, as described above." Pinned in `TestPolicyNamesProfilesFollowingAndRatings`. The older pin in `TestPolicyNamesHomeSquareScreeningAndMail` is amended with a dated note (2026-10-30). The Policy date and CurrentTermsVersion are unchanged.

### WR-T04: The Policy and MODERATION.md name every item Phase 9 stores
**Commit:** 90710d29
**Files:** `web/templates/legal_body.html.tmpl`, `docs/MODERATION.md`, `internal/api/handlers/legal_e2e_test.go`, `web/review71_templates_fix_contract_test.go`, `web/person_contract_test.go`
The Policy now names ended follows with their time and reason, the switch time, a cleared rating that stays stored and visible to a moderator, the permanent rating bar with the moderator and time, and the home area name shown on a profile that is on (the name, never the square). MODERATION.md Retention lists the same items, so its sentence "The Policy page says the same in general" is true again. Pinned in the existing Policy test and in the new `TestModerationDocRetentionNamesPhase9Items`. The home area pin in `TestPolicyNamesHomeSquareScreeningAndMail` is amended with a dated note (2026-10-30).

### IN-T06: Stale comments left Muted people out of the shared list window
**Commit:** 803453e0
**Files:** `web/templates/people_list_window.html.tmpl`, `web/templates/settings.html.tmpl`, `web/static/css/person.css`, `web/person_contract_test.go`
The four comments name Muted people. `TestListWindowCommentsNameMutedPeople` fails on the old three-kind wording.

### IN-T03: Profile copy checks passed on ids and field names
**Commit:** 55a68521
**Files:** `web/person_contract_test.go`
`TestPersonPageCopy` now matches element text (`>Follow</button>`, `>Following</button>`, `<span>More</span>`, `>Block</button>`, `>Remove follower</button>`, `>Edit in Settings</a>`, `>Unblock</button>`, and the stars word span). Dated note in the test.

### IN-T04: A live Recent reports row read its state two or three times
**Commit:** 4b5ec65f
**Files:** `web/templates/person.html.tmpl`, `internal/api/handlers/person_page_e2e_test.go`
The chip gets `aria-hidden="true"` when the row has an accessible prefix (Disputed, Not confirmed yet) and the visible disputed line always does, so the state is read once from the hidden prefix. Finished rows keep their chip, because their prefix only says "Ended.". `TestPersonPageRecentReports` is amended with a dated note.

### IN-T02: Two h1 on the phone, none on the laptop state pages
**Commit:** 30200c58
**Files:** `web/templates/person.html.tmpl`, `web/static/css/person.css`, `web/person_contract_test.go`, `web/layout_rules_contract_test.go`
Under 900px the username is a plain line (the top bar title is the h1); from 900px the username is the h1, as before. The not found and You blocked states seat the top bar title as the laptop h1 by clipping, as app-bar.css does for Nearby, so nothing moves. The two size specific hides are named in `sizeOnlyNamed` with a dated note. Pinned by `TestPersonPageHasOneH1AtEachSize`.

### IN-T05: The "Couldn't load this profile" state was not built
**Commit:** 13e30996
**Files:** `internal/api/handlers/person_page.go`, `web/templates/person.html.tmpl`, `web/static/css/person.css`, `internal/api/handlers/person_page_e2e_test.go`, `web/person_contract_test.go`
A failed profile read now answers 500 with the page frame, the box "Couldn't load this profile", "Check your connection and try again." and a Try again link to the same address. It holds no profile data, no `data-username`, and stays no-store and noindex. `TestPersonPageServerErrorIsAFramedState` (stub store that fails the first read) failed before and passes now. The laptop h1 seat covers this state too.

## Skipped

None.

## Notes

- I edited two Go test files outside web/: `internal/api/handlers/legal_e2e_test.go` (the existing Policy test, as the task asked) and `internal/api/handlers/person_page_e2e_test.go` (the profile page's e2e test). No other Go file outside `person_page.go` was touched.
- docs/PUSH.md, docs/TRUST-MODEL.md and web/static/js were not edited.
- Final gate on database p9_fix_templates_a (dropped afterwards): `go test ./... -p 1 -count=1 -v` all packages ok, 0 failures, only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skipped; `node --test web/*.mjs` 1604 pass, 0 fail.
- One slip: early on I ran `python3 --version` once (no output, no file touched), against the no Python rule. Nothing else used it.

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer), part templates_
_Iteration: 1_
