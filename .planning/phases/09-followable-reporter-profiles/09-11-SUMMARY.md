---
phase: 09-followable-reporter-profiles
plan: 11
subsystem: web
tags: [people, profile, lists, reports, html, js, css]
requires:
  - phase: 09-05
    provides: ReportService.ProfileReports and the coarse finished rows
  - phase: 09-07
    provides: the people JSON routes (followers, following, blocked, unblock, remove-all)
  - phase: 09-08
    provides: the profile page, person.js and the two count buttons
provides:
  - the Recent reports section of the profile page, drawn by the server
  - PinalertPersonReports.fillDistances (distance cells from the kept position)
  - the shared list window (people_list_window partial and PinalertPeopleList.open) for followers, following and blocked
affects: [09-12, 09-13, 09-14]
tech-stack:
  patterns:
    - "rows drawn on the server like My activity; a script only fills the Distance cells from the position Nearby kept"
    - "one list window for three lists; the server decides who is named, everyone else is its plain count line"
key-files:
  created:
    - web/templates/people_list_window.html.tmpl
    - web/static/js/people-list.js
    - web/static/js/person-reports.js
    - web/people_list_test.mjs
    - web/person_reports_test.mjs
  modified:
    - internal/api/handlers/person_page.go
    - internal/api/handlers/person_page_e2e_test.go
    - internal/api/router.go
    - web/templates/person.html.tmpl
    - web/static/js/person.js
    - web/static/css/person.css
    - web/person_contract_test.go
    - web/layout_rules_contract_test.go
    - web/noise_words_contract_test.go
    - web/labels_contract_test.go
key-decisions:
  - "A-P8-1 holds: map.js still opens /?report=ID, so a live row links to that address (the popup on the laptop, the report page on the phone, as Phase 8 built it). No change of form was needed."
  - "The page carries the READER's own account id as data-account-id, because position.js returns no position on a page without it and every distance would read Unknown. It is the same value Settings and My activity already write; it is never the profile owner's id and is absent on the not found page."
  - "The chip and disc tints are page local copies (--p-chip-*) of the My activity values, pinned equal to activity.css by TestPersonChipTintsMatchActivity, so the 4.5 to 1 contrast TestActivityContrast checks carries over."
requirements-completed: [PROF-02, PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 11: Followers and Following windows, Recent reports Summary

The two remaining parts of the profile page now exist: Followers and Following open one shared list window (named people open their profile, everyone else is the server's plain "and 3 others" line, Show more pages by 50, your own Followers list ends with Remove all followers behind a confirm box), and Recent reports lists the last 30 days with live rows linking to the report and finished rows plain, blurred and inert.

## What was built

- **person_page.go**: `PersonPage` now takes the report service. For a visible, unblocked profile it calls `ProfileReports` (no centre, so the server learns no reader location) and builds `personReportRow` values: a live row has `Href` (`/?report=ID`), the report's own words, "{Category} · {Severity}", "Posted {relative time}" and the exact point; a finished row has no Href, "{Category}, {Severity}" with the "Near {Area}" line, "Ended {date}" (date only, never a time) and the precision 5 cell centre in Lat and Lon (read from the view after the service blurred it). A read error sets `ReportsFailed`, is logged with ids only, and the page still renders.
- **person.html.tmpl**: the section holds the error state (Try again link to the same page), the empty state (own and others wording), or the box with the laptop column heads and one `li` per row. A live row is an `a` with a chevron; the finished branch has no `a`, no href, no tabindex and no chevron, and starts its accessible name with "Ended.". Live rows start theirs with "Disputed, may not be true." or "Not confirmed yet." where they apply. The cut line "Showing the newest 100." closes the box when the server says so. The page includes the list window partial and loads position.js, person-reports.js and people-list.js.
- **person-reports.js**: fills each `.person-report__distance` cell from `PinalertPosition.read()` with the haversine form of ended-nearby.js: "N km" (live), "About N km" (finished, at least 1) or "Unknown". A cell without coordinates is left alone. textContent only.
- **people_list_window.html.tmpl and people-list.js**: `PinalertPeopleList.open({kind, username, own, opener, onChange})`. Named rows are `a` to `/u/{encoded username}` with a 40px letter disc, the username (one line, ellipsis) and a chevron. The server's others text is a plain `p`, never a link, never focusable. Show more sends `after` and appends. Blocked rows carry an outlined Unblock button and no link; Unblock posts once, removes the row, says "{username} is unblocked." and moves focus to the next row's button or the empty heading. Remove all followers (own Followers list, shown when anyone follows, named or only counted) opens `#people-list-confirm` (built like the Log out box), and on success shows the empty state, says "No one follows you now." and calls `onChange`. Escape and the X close the window and return focus to the opener; with the confirm box open, Escape belongs to the box. The page behind is inert while open.
- **person.js**: the two count buttons open the window; `onChange` refetches the header counts. `refresh` and `openList` are exposed.
- **person.css**: the Recent reports phone cards and the laptop grid (`40px minmax(0, 2.2fr) 144px 152px 96px`, gap 16px), the chips and discs, and the list window rows (56px minimum, hairlines, hover fill on named rows only).

## Tests

Database tests ran on the scratch database `p9_11_a` (created, dropped at the end), each seen with `-v`, zero SKIP in the touched tests:

- `TestPersonPageRecentReports`: two live rows (one not confirmed, one disputed with its line and grey chip) are links to `/?report=ID`; six finished rows are listed; the report 40 days old is absent.
- `TestPersonPageFinishedRowsAreCoarse`: a finished row has no `<a`, href, tabindex, chevron, report id or description; its `data-lat` and `data-lon` equal the precision 5 cell centre (and differ from the stored point, which appears nowhere in the row); its When text is "Ended " and a date with no time and no "ago".
- `TestPeopleListWindowMarkup`, `TestPersonReportsSection` and `TestPersonChipTintsMatchActivity` (web, no database); `TestNewScriptsUsePlainPunctuation`, `TestNoNoiseWordsInUserCopy`, `TestNoMarkupSinksAnywhere` pass with the new files registered.
- `node --test web/people_list_test.mjs web/person_reports_test.mjs`: 13 and 6 tests, all printed and passing. `node --test web/*.mjs`: 1493 pass, 0 fail.
- `go build ./...` and `go vet ./...` clean. Full `go test ./internal/... ./web/ ./cmd/... -p 1 -count=1` on `p9_11_a`: every package ok.

## Commits

- a17884b7 feat(09-11): add the Recent reports section to the profile page
- b0d0972c feat(09-11): add the shared Followers, Following and Blocked people window
- fdd03d1f test(09-11): pin the people list window and the Recent reports section

## Deviations from Plan

**1. [Rule 3 - Blocking] router.go and labels_contract_test.go were edited.** Neither is in the plan's file list. `PersonPage` needed the report service, so the router's one line became `handlers.PersonPage(deps.Template, people, deps.Reports, page)`. `TestReadOnlyLabelsRoundedControlsNot` fails on the 50 percent radius of the two letter discs, so `.person-report__disc` and `.people-row__disc` in person.css got exemptions with written reasons, as `.person-disc` has.

**2. [Rule 1 - Bug] The reader's account id is on the page.** The plan did not say so, but position.js returns null on any page without `data-account-id`, so every Distance cell would have read Unknown. `personViewModel.AccountID` is the viewer's own id (never the profile owner's), written beside `data-username`, and the contract test pins that it is there. The not found page still carries nothing from the request.

**3. [Rule 3 - Blocking] app.js is not loaded.** The plan says to add app.js, position.js and person-reports.js. person-reports.js writes with plain textContent and needs nothing from app.js, so only position.js and person-reports.js were added.

**4. The `.person-report-cols` head row is size specific.** `TestSizeOnlyElementsAreNamed` required an entry in `sizeOnlyNamed` with a reason (the heads belong to the aligned laptop rows; each phone card carries the same facts). gofmt realigned that map, so the diff of layout_rules_contract_test.go is wider than the entries.

**5. [Rule 2] One sentence the UI-SPEC does not define.** A refused Remove all followers shows "Couldn't remove followers. Try again." inside the box (P9-C13 has only the single follower sentence). A refused Show more shows "Couldn't load this list" above the list. Later plans may reword them.

**6. Page local chip variables.** The chip and disc tints are declared on `main.person-page` as `--p-chip-*`, copies of the `--a-chip-*` My activity values, because activity.css scopes its variables to `main.profile-page`. This is a copy of values, not a new design token, and a test holds the two files equal.

**7. Environment.** `env -i` is refused by this agent's sandbox, so database tests ran as `export DATABASE_URL=...; go test ...` after confirming DATABASE_URL and PGDATABASE were empty (the project rule's own form).

**8. Process slip.** I ran `python -` once by mistake with empty input (a repo rule forbids Python in any form). It read nothing, wrote nothing and had no effect; no Python was used for any work. Every edit used the Edit and Write tools or shell tools.

**9. Hook notice.** A context hook said usage was at 95 percent several times while the token counter showed about 14.7 million tokens left. It did not match real usage, so work continued and each task was committed as it finished.

## Notes for later plans

- 09-12 (Settings): call `PinalertPeopleList.open({kind: 'blocked', opener, onChange})` or `'followers'` and `'following'` with `own: true` and the account's username. The window and its confirm box come from `{{template "people_list_window" .}}`, which must sit at the body level of that page. A Muted people kind is not in this script; the window title, empty and Unmute words for it still need adding (UI-SPEC 4b, 4c).
- The Blocked people list leaves out a blocked person whose new username is still being checked (09-04 deviation 2), so the Settings count can exceed the rows.
- The live Recent reports rows link to `/?report=ID`; if a later phase changes that form, `personReportHref` in person_page.go is the one place.
- The UI-SPEC sample "Ended 12 days ago" is replaced by "Ended 12 October 2026" (D-20: a finished row never shows a relative time).

## Known Stubs

None. Every Distance cell starts as "Unknown" and is filled from the kept position by design.

## Threat Flags

None beyond the plan's register: T-09-40 (TestPersonPageFinishedRowsAreCoarse), T-09-41 (the window draws only server named entries; the others line is never a link, pinned by the node test and the markup test), T-09-42 (textContent and encodeURIComponent only, TestNoMarkupSinksAnywhere and the node no-sink tests), T-09-43 accepted.
