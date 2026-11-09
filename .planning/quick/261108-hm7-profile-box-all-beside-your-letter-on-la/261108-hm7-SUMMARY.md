---
phase: quick-261108-hm7
plan: 01
subsystem: profile page layout (laptop)
tags: [profile, laptop, band, grid, owner-decision-2026-11-08]
requires:
  - Quick 261108-fku (the band and its two wrapper divs, with its review fixes)
provides:
  - Profile box from 900px with every part close beside the letter disc, the action row right after the username, no line across the box
affects:
  - web/static (person.css)
key-files:
  created: []
  modified:
    - web/static/css/person.css
    - web/person_band_contract_test.go
    - internal/api/handlers/person_page_e2e_test.go
decisions:
  - Grid columns auto auto minmax(0, 1fr) with the areas disc/user/actions, name, counts, record; no markup change
metrics:
  tasks: 1
  completed: 2026-11-09
status: complete
---

# Phase quick-261108-hm7 Plan 01: Profile box, all beside your letter Summary

From 900px the profile box is the owner's "All beside your letter" drawing: the letter disc in the first column and, close beside it from top to bottom, the username with the action row right after it on the same line, the name, the two counts, the stars, the tag with its sentence, Member since with the home area, then the rating and the error line, with no line across the box and nothing at the far right. The phone is byte for byte the 261108-fku build.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| C | cbd8c54a | fix(261108-hm7): the profile box sits all beside the letter on the laptop |

## What changed

- person.css, inside the one `@media (min-width: 900px)` block only. `.person-band` is a grid with columns `auto auto minmax(0, 1fr)` and the areas `"disc user actions"`, `"disc name name"`, `"disc counts counts"`, `"disc record record"`. `.person-ident` and `.person-ident__text` take `display: contents`, so the disc, the h1 and the name are grid items. The disc is `align-self: start`, the h1 and the action row are `align-self: center` (same line, a 16px column gap), the action row is `justify-self: start`. The counts are `justify-self: start` with `margin-inline-start: calc((var(--space-12) + 1px) * -1)`, so the text of Followers lines up with the username (the button keeps its padding and its transparent 1px edge). The record is a wrapping flex row with no border, margin or padding. The rating mount and the action error line take `grid-column: 2 / -1` and are auto placed in the rows after the record, so an empty or hidden one adds no row and no gap. The More menu keeps `right: 0; left: auto` on the action row, so it hangs under More. The fku comments that described the far right placement and the line are replaced; the forced colours rule for the record line is removed (the line is gone). No new token, no hex, no order property, no overflow on the band.
- person.html.tmpl: unchanged. The phone base rules (`.person-band, .person-record { display: contents }` and everything else outside the laptop block) are unchanged.
- Tests: `TestPersonProfileBand` now pins the new columns and areas, the wrappers with display contents, the disc, username, name, counts and action row placement, no far right placement anywhere in the laptop block, no record border, padding or margin, the rating column, and the phone base rules as before (each changed pin carries a dated Amended line). `TestPersonPageBand` gained the phone reading order check of the band's markup. `git grep` found no other test pinning the old placement (person_contract_test.go and person_test.mjs check ids and script behaviour only).

## Known conflict with a house rule (recorded as the owner's drawing, copied as drawn)

- Tab order (WCAG 2.4.3 Focus Order). The DOM order is unchanged because the phone depends on it (identity, record, counts, actions, rating, error line). On the laptop the action row is drawn next to the username, so Tab reaches Followers and Following before the action row while the eye sees the action row first. Every control stays reachable, and the order runs top to bottom except for the action row. No CSS order property is used. The same note is in the person.css comment. Probed: Tab starts with Followers then Following and the action row comes after them, on all three pages at all three widths.

## Results

- Laptop probes, WebKit, 1326 checks, all PASS, in `hm7-exec/probes-detail.txt`: widths 900, 1280, 1440; light, dark, light with data-theme dark, dark with data-theme light (each variant checked to really draw the palette it names); the own page with no reports, the own page with reports, and ravi.k's page seen by another person (stars, Follow row, Your rating); plus no horizontal scroll at 320 and 390. Per page: the disc first and at the box's inner top; the name, Followers text, stars, tag chip, Member since and Your rating each within 2px of the username's left edge; the action row centred on the username line (within 4px) and starting less than 48px right of it; order username, name, counts, stars, tag, Member since (then rating); Member since and the home area on one row; nothing but the action row starting right of 60% of the box width; the record has no top border; no empty rows left under the last item; the box and the Recent reports section share left edge and width within 1px; the own page state line is under the box and outside it; counts keep aria-haspopup dialog, weight 600 or more, 44px height, transparent edge and the surface hover fill; Edit in Settings keeps its edge; the empty state box keeps its border and radius; the More menu is inside the viewport, hangs below More with its right edge on More's, and every item is hit at its own centre; Tab order as above.
- Phone proof: before is 3dfdc48a (built from `git archive`, server-before), after is the final tree (server-after), both on port 8794 against the same scratch database, back to back, same sign in states, 8 pairs (320 and 390; own page with no reports, own page with reports, someone else's page with stars and the Follow row, not found), compared with Buffer.equals: 8 of 8 byte identical. A before against before self test came out 8 of 8 identical first. Masking: Playwright WebKit, deviceScaleFactor 1, reducedMotion reduce, a fresh context per page, one warm up load, a 2400px tall viewport so the whole page is in the shot, full page shot with Playwright's mask option over `.top-chrome` and `nav.app-bar`. The two bars are masked because the bell count moves with the bell poll; neither bar is touched by this work, so the proof still covers every pixel it can change. Comparison file: `hm7-exec/compare.txt`.
- Shots (1280, light and dark): own page with no reports, own page with reports, ravi.k's page seen by voter.a, and the More menu open on ravi.k's page, in `hm7-exec/shots/`. No image is in the repo.
- Gate on q_hm7_a: `go build ./...` and `go vet ./...` clean; `go test ./... -p 1 -count=1 -v`: 2455 top level `--- PASS`, 6817 `--- PASS` counting subtests, 0 `--- FAIL`, no `FAIL` line, 21 packages ok, exit 0, exactly three skips (TestGeocodeLive, TestOfficialLive, TestScreeningFixture); `node --test web/*.mjs`: 1624 tests, 1624 pass, 0 fail, 0 skipped. The wording check printed nothing. The three touched files hold no em or en dash, before or after (0 and 0).

## Deviations from Plan

None to the code or the plan's scope. Notes on the harness only:

1. [Harness] The scratch database's four live reports were moved two days back in created_at (q_hm7_view only), as in fku, so relative times read the same in every run.
2. [Harness] Playwright came from an existing npx cache (version 1.63.0, WebKit 2359); the scripts in the scratch folder require it by absolute path.
3. [Harness] The laptop "Tab" checks use Alt+Tab, because WebKit does not focus buttons on plain Tab (Safari's default), as in fku.
4. The plan's check "action row's top within 4px of the username's top band" was implemented as the two centres within 4px, because the action row (44px) is taller than the username line and the plan says it is vertically centred on it.
5. [Harness] The coordinator's session stopped on an API timeout partway; work resumed from the uncommitted tree with nothing redone beyond the phone shots.

## Known Stubs

None.

## Threat Flags

None. Only a stylesheet and tests changed.

## Self-Check: PASSED

- Commit cbd8c54a exists on the branch and lists person.css, person_band_contract_test.go and person_page_e2e_test.go.
- Nothing was pushed; STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched; no migration; no image in the repo.
