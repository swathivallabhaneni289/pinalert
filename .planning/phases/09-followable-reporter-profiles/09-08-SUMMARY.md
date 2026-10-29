---
phase: 09-followable-reporter-profiles
plan: 08
subsystem: web
tags: [people, profile, page, follow, block, html, js]
requires:
  - phase: 09-04
    provides: PeopleService.Profile, MyUsername, PersonProfile, ErrPersonNotFound
  - phase: 09-07
    provides: the twelve gated people JSON routes and their sentences
provides:
  - GET /u/{username} and GET /me in the gated group, no-store and noindex
  - the profile page (person.html.tmpl, person.css) for your own page and anyone else's
  - person.js (Follow, Following, More menu, Remove follower, Block, Unblock)
  - star-filled.svg and the web contract tests that pin the page
affects: [09-09, 09-11, 09-13, 09-14]
tech-stack:
  patterns:
    - "the page is drawn by the server; both Follow and Following are in the HTML and the script only toggles hidden flags after a refetch of the JSON profile"
    - "one not found template branch that carries nothing from the request, rendered through a buffer so a template failure is a clean 500"
key-files:
  created:
    - internal/api/handlers/person_page.go
    - internal/api/handlers/person_page_e2e_test.go
    - web/templates/person.html.tmpl
    - web/static/css/person.css
    - web/static/js/person.js
    - web/static/icons/star-filled.svg
    - web/person_contract_test.go
    - web/person_test.mjs
  modified:
    - internal/api/router.go
    - internal/api/people_gate_test.go
    - web/css_contract_test.go
    - web/layout_rules_contract_test.go
    - web/noise_words_contract_test.go
    - web/chrome_contract_test.go
    - web/labels_contract_test.go
key-decisions:
  - "The page uses its own nav key (navPerson), not NavSettings, because NavSettings hides the phone top bar gear and UI-SPEC 1a keeps it."
  - "The Following toggle has the id person-unfollow, because person-following-button is the Following count button."
  - "A /me for an account with no username redirects to /settings instead of answering an error."
requirements-completed: [PROF-01, PROF-02, PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 08: The profile page Summary

The profile page exists: one server rendered page for your own profile and anyone else's with the header in the D-13 order, Follow and Following, the More menu, the Block and Remove follower confirm box, the Blocked by you state and one byte equal not found, plus /me.

## What was built

- **person_page.go**: `PersonPage` reads the account from context, calls `people.Profile`, renders into a buffer, then sets Cache-Control no-store, X-Robots-Tag noindex and the status (404 for `ErrPersonNotFound`, 200 otherwise). `MyProfilePage` answers 303 to `/u/<username>` (path escaped). Both routes are registered next to `/settings` in the gated group with no rate bucket.
- **person.html.tmpl**: the Settings frame (app_head, app_topbar, notice-live, app_bottombar, the person.css link), `main#person-page` with `data-username` and `data-self` written only when there is a profile. Header ids in UI-SPEC 1b order: disc, h1 username, Name, stars (only when the server sends a number), tag chip and sentence, Member since, home area, Followers and Following buttons, the action row, the inline error. Then the Recent reports section (empty container for 09-11), the Blocked by you box and `dialog#person-confirm` built like `#settings-delete-confirm`.
- **person.css**: phone stack as the base and the 320px | 1fr laptop grid in the one min-width 900px block. Tokens only, no new token, every display rule guarded with `:not([hidden])`, controls 44px through `--touch-target-min`, the stars in `--color-text`.
- **person.js**: `window.PinalertPerson.init()`. Follow and Following call the JSON route, refetch the profile and draw the counts and flags by textContent, announce the status line and keep focus on the button now in place. The More menu follows the Show menu keys. Remove follower and Block open the confirm box with the UI-SPEC titles and texts; a refusal shows the sentence inside the box; a confirmed Block shows the blocked box and focuses its heading. Unblock posts, announces and reloads. A 404 reloads, a 409 or 429 shows the server's sentence, other failures show the P9-C13 sentence, offline shows the built line.
- **star-filled.svg**: the Lucide v1.41.0 star path fetched from unpkg (not typed from memory), fill kept, same licence line as bell.svg.

## Tests

Database tests ran on the scratch database `p9_08_a` (created, dropped at the end), each seen with `-v`:

- `TestPersonPage`, `TestPersonPageNotFoundIsUniform` (unknown, private, pending, deleted, blocking and a malformed name: byte equal 404 bodies that never echo the name), `TestMyProfileRedirect`, `TestPersonPageBlockedByYou` (including Unblock redrawing the profile), and the two HTML rows in `TestAccessGateBlocksPeopleRoutes` (302 to /login).
- `web/person_test.mjs`: 16 tests, all printed and passing (follow redraws from the refetch, busy state, unfollow with no box, menu keys, hidden item skipped, both confirm boxes with exact words, Cancel sends nothing, blocked box focus, 409, 404, 429, failure sentences, offline, Unblock, Escape refused while busy, no dialog support, no markup sink).
- `TestPersonPageHeaderOrder`, `TestPersonPageNoindexAndFrame`, `TestPersonPageCopy` in web/person_contract_test.go.
- Full `go test ./internal/... ./web/ ./cmd/... -p 1 -count=1` on `p9_08_a`: every package ok; the only skips are TestGeocodeLive, TestOfficialLive and TestScreeningFixture. `go build ./...` and `go vet ./...` clean. `node --test web/*.mjs`: 1468 pass, 0 fail.

## Commits

- f296f855 feat(09-08): add the profile page, /me and the gated /u routes
- 4d4e926c feat(09-08): add person.js for follow, the More menu, Remove follower, Block and Unblock
- e4701ca0 test(09-08): register the profile page with the web contract tests

## Deviations from Plan

**1. [Rule 3 - Blocking] chrome_contract_test.go and labels_contract_test.go were edited.** They are not in the plan's file list. `TestDisclaimerBarOnEveryFullPage` fails for any new full page without a table entry, so `person.html.tmpl` was added there as "top". `TestReadOnlyLabelsRoundedControlsNot` fails on the 50 percent radius of the letter disc, so `.person-disc` in person.css got an exemption with the same written reason the comment and share discs have (a drawing, not a control).

**2. [Rule 3 - Blocking] The reader window is not on the page.** The plan says to copy the Settings frame including `reader_dialog`, `reader.js` and `legal.css`, but `TestReaderOpeners` allows the reader only on Nearby and Settings, and the profile opens no legal text. They were left out, with the `Legal` field of the view model.

**3. [Rule 1 - Bug] The not found and nav handling needed a key other than NavSettings.** Using the Settings key hides the gear on the phone top bar (UI-SPEC 1a keeps it), so the handler uses its own `navPerson` key. A contract test pins that person_page.go never uses the Settings key.

**4. The plan's id list was extended.** Added `person-unfollow` (the Following toggle, because `person-following-button` is the count button), `person-edit`, `person-blocked-title`, `person-blocked-error`, `person-reports` and the `person-confirm-*` ids.

**5. Environment.** `env -i` is refused by this agent's sandbox, so the database tests ran as `export DATABASE_URL=...; go test ...` after confirming DATABASE_URL and PGDATABASE were empty (the project rule's own form).

## Notes for later plans

- 09-11: the Followers and Following buttons are drawn with `aria-haspopup="dialog"` and no handler; fill `#person-reports` and open the list windows from them. person.js already updates the count buttons from the refetch.
- Mute, the Notify me bell and the rating control are not drawn here; the action row has room after Following and before More.
- `personViewModel` has no Legal field and the page loads no reader.
- TestPersonMenuItems (web/places_contract_test.go) still bans "My profile" in account_header.html.tmpl; the plan that adds the person menu item must amend it.

## Known Stubs

None. The `#person-reports` container is empty on purpose for plan 09-11.

## Threat Flags

None beyond the plan's register: stored XSS (html/template plus textContent only, TestNoMarkupSinksAnywhere and the node test), uniform 404 (TestPersonPageNotFoundIsUniform), noindex and no-store on every branch including the 303, and the confirm box before Block (T-09-29 to T-09-32).

## Self-Check: PASSED

All files listed under key-files exist, and commits f296f855, 4d4e926c and e4701ca0 are on the branch.
