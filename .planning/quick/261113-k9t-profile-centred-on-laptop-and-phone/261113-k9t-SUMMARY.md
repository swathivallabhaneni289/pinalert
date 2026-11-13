---
phase: quick-261113-k9t
plan: 01
subsystem: profile page (centred header, laptop and phone)
tags: [profile, centred, laptop, phone, owner-decision-2026-11-13]
requires:
  - Quick 261109-n25 (fewer words, small laptop buttons)
provides:
  - Profile header drawn as one centred column on the laptop and the phone, the DOM order equal to the reading order
affects:
  - web/templates/person.html.tmpl
  - web/static/css/person.css
  - web/static/js/person.js
  - web/person_band_contract_test.go
  - web/person_contract_test.go
  - web/person_test.mjs
  - internal/api/handlers/person_page_e2e_test.go
key-files:
  modified:
    - web/templates/person.html.tmpl
    - web/static/css/person.css
    - web/static/js/person.js
    - web/person_band_contract_test.go
    - web/person_contract_test.go
    - web/person_test.mjs
    - internal/api/handlers/person_page_e2e_test.go
decisions:
  - The centred header (option D of the owner's mockup) replaces the 261108-fku band, the 261108-hm7 grid and the 261109-n25 left aligned header on the laptop, and replaces the phone header too
  - No wrapper divs: the letter, username, name, action row, counts row, details line, state line or Your rating, and error line are direct children of the header
  - Clock icon mask lives in person.css (page scoped, the activity.css precedent), app-bar.css is untouched
metrics:
  tasks: 1 (two commits)
  completed: 2026-11-13
status: complete
---

# Phase quick-261113-k9t Plan 01: The profile, centred, on the laptop and the phone Summary

The profile header is now one centred column on both sizes, as the owner picked ("lets do the centered", option D of the mockup): the letter, the username, the name, the action row, a counts row with thin lines between the cells, the details line with small icons, then the private line on your own page or Your rating on someone else's. Recent reports stays a full width section under it. This replaces the 261108-fku, 261108-hm7 and 261109-n25 header layouts on the laptop and replaces the phone header too; the 261109-n25 words, the small 32px laptop buttons and the phone cards are kept.

## Commits

| Part | Commit | Subject |
| ---- | ------ | ------- |
| Task E | 14505bd4 | fix(261113-k9t): the profile centred on the laptop and the phone |
| Task E follow up | cd122cbc | fix(261113-k9t): equal width counts cells on the phone |

## What changed

- person.html.tmpl: `#person-band`, `#person-record`, `.person-ident` and `.person-counts` are gone. The header holds, in reading order, the disc, the h1 and the phone username line, the name, `#person-actions`, `#person-stats` (the stars group when present, Followers, Following, the reporter cell), `.person-facts` (Member since with a clock, the home area with a map pin, both aria-hidden), then `#person-state-line` on your own page or `#person-rating` on someone else's, then `#person-action-error`. Every id, aria attribute and hidden flag the server and person.js use is kept.
- person.css: a centred column on both sizes. Letter 88px on the phone and 104px on the laptop (token calcs), username at the heading size on the phone and the display size on the laptop h1. Counts row: cells share the width on the phone (the reporter cell wraps to its own full width row when the stars cell is there, keyed on `:has(> .person-stars)`), at least 140px with the reporter cell at most 260px on the laptop, a 1px `--color-border` line between cells, no outer box, count buttons at least 44px tall. Details stacked on the phone, side by side on the laptop. State line and Your rating centred. The empty and error state of Recent reports sit in a 1px bordered box with a 12px radius on both sizes. The More menu hangs from the right edge of the action row on both sizes. Forced colours keeps the cell lines, the state box edge and the button borders. No CSS `order` property, no new token, no hex outside the old chip tints.
- person.js: `drawStars` puts a stars group built after a rating first in the counts row (it used to go in front of the reporter cell), and its word is Stars.
- Tests, each with a dated Amended line naming this decision: `TestPersonProfileBand` became `TestPersonProfileCentred` (person_band_contract_test.go), `TestPersonPageBand` became `TestPersonPageCentred` (person_page_e2e_test.go, own and other pages, DOM order and the counts row order), the header order, copy and rating mount pins in person_contract_test.go, and the fake page of person_test.mjs (header holds the error line and the rating box directly, a stars group built by the script is the first cell of a four cell row).

## Results

- Gate on q_k9t_a (dropped afterwards), on the final tree cd122cbc: `go build ./...` and `go vet ./...` clean; `go test ./... -p 1 -count=1 -v`: exit 0, 2457 top level `--- PASS`, 6819 `--- PASS` counting subtests, 0 `--- FAIL`, no `FAIL` line, 21 packages ok, exactly three skips (TestGeocodeLive, TestOfficialLive, TestScreeningFixture). `node --test web/*.mjs`: 1627 tests, 1627 pass, 0 fail, 0 skipped. The wording check printed nothing. The touched files held 0 em or en dashes before and after (and none in this summary).
- Browser probes, WebKit, final run 2232 checks, all PASS (`probes-detail.txt`): 1280 and 1440 in light, dark, light with data-theme dark and dark with data-theme light (each really draws the palette it names), 900 light, 320 and 390 light and dark; own page with no reports (voter.a), own page with reports and stars (ravi.k), ravi.k seen by voter.a, new.reporter seen by voter.a. Per page: no horizontal scroll; the visible username is the phone line below 900 and the h1 from 900; the disc, username, name, action row, counts row, details line, state line and Your rating (with its legend and its stars row) centred on the page content within 2px; top to bottom order and visible DOM order as listed; Tab order (Alt+Tab in WebKit) is the visible actions, then Followers, then Following, then the rating stars; the counts row has no outer border, a 1px line on the left of each cell that shares the row and none on a cell that wraps; cells hold the stars group (when present), Followers, Following and the reporter cell in that order; at least 140px wide cells on the laptop and the reporter cell at most 260px; on the phone with four cells the reporter cell is on its own full width row and the other three are one row of equal width, and with three cells they are one row of equal width within 1px; at 320 no cell overflows, nothing spills, no chip is clipped and no chip word is broken mid-word; details stacked on the phone and side by side on the laptop, icons 16px, masked and aria-hidden; letter 88px or 104px; phone buttons and bell at least 44px tall, laptop buttons and bell 30 to 34px, count buttons at least 44px on both sizes; empty state box 1px border, 12px radius, left aligned text and its left edge on Recent reports' left edge; at least 32px (phone) or 48px (laptop) between the header and Recent reports; the More menu inside the viewport, under More with its right edge on More's, and every item hit at its centre, on both sizes including 320.
- Shots (no image is in the repo), in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/k9t-exec/shots/`: `own-noreports` and `other-stars` (ravi.k seen by voter.a) at 1280 light and dark, 390 light and dark and 320 light, plus `own-reports` and `other-new` at the same sizes. Viewed next to the mockup shots m2-d-own, m2-d-other, p3-d-own and p3-d-other.

## Deviations from Plan

1. The word under the stars number is drawn Stars (it was stars); the plan's step 5 allows it and the copy pin was amended.
2. The details line is drawn at the body size, not the label size. The mockup draws 15px, which sits between the two tokens, and body is the nearer for the width of the line.
3. The More menu `min-width` went from 220px to 200px so the menu, hung from the right edge of the row, stays inside a 320px viewport when More is the only visible action (the probe caught -5.6px at 220).
4. `app-icon--clock` is declared in person.css as `main.person-page .app-icon--clock` (the activity.css page scoped precedent), not in app-bar.css, which is not in the plan's file list. One literal url, the icon registration tests pass.
5. The phone reporter chip is drawn at the caption size (the mockup draws 12px) so "New reporter" fits an equal width cell; at 320 the chip text may wrap onto two lines inside its cell, which the plan allows. A first build gave the reporter cell more room (`flex-grow: 1.6`); it did not match the drawing's equal cells, so the second commit removed it.
6. Task E landed as two commits instead of one (the second is the equal width fix, found when the first build was viewed next to p3-d-own).
7. The tests were renamed, not only amended (`TestPersonProfileBand` to `TestPersonProfileCentred`, `TestPersonPageBand` to `TestPersonPageCentred`) because every pin of the band described layouts that are gone; the helper `fkuDivBlock` was removed with the old e2e test, and the e2e fixture usernames are `ctr_viewer` and `ctr_owner`.
8. The empty state box sits in the base rules, so the error state of Recent reports gets the same box (the plan names the empty state and the error state together).
9. [Harness] A stray file was written by mistake to `/private/tmp/claude-501/-Users-swathivallabhaneni/dummy-ignore`, outside the scratchpad, and removed straight away (with its empty folder). Nothing else outside `k9t-exec` was written. Databases created and dropped: q_k9t_a, q_k9t_view, p9_k9t_seed (dropped at once after the `createdb -T` copy). Port 8794 only.

## Known Stubs

None.

## Threat Flags

None. Only a template, a stylesheet, a script and their tests changed.

## Self-Check: PASSED

- Commits 14505bd4 and cd122cbc exist on the branch and list every changed file; nothing was pushed; STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched; no migration; no image in the repo.
- q_k9t_a and q_k9t_view are dropped, port 8794 is free, state-*.json and seed.log are deleted.
