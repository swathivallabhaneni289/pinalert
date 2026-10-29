# Phase 9 code review fix, part B (browser)

**Fixed:** 2026-10-30
**Source review:** 09-REVIEW-browser.md (triage in 09-REVIEW.md)
**Base:** c2014ea9

Each fix is one commit with a test that failed before and passes after. Where a
fix changed what an older test pinned, the pin was amended with a dated note.
The full Go run (`go test ./... -p 1 -count=1 -v`, scratch database
p9_fix_browser_a, dropped afterwards) is green with only TestGeocodeLive,
TestOfficialLive and TestScreeningFixture skipped. `node --test web/*.mjs`:
1619 tests, all pass.

## Fixed

| Finding | Commit | What changed |
|---------|--------|--------------|
| WR-B01 | f4a308a0 | trust.js PROFILE_PATH allows a leading underscore (still not a leading dot). |
| WR-B02 | cdaf2e69 | setProfileLink keeps the same node when the path is unchanged and the node is still in the panel, so a poll no longer drops focus. |
| WR-B03 | 0c26039e | person.js: a saved or removed rating now refreshes the profile; draw writes the stars number, label, both parts, the tag word and its sentence (building or removing the stars group as the server's number appears or goes); a 409 on the bell refreshes and moves focus to More when the bell is gone. Pins amended in person_test.mjs (two). |
| WR-B04 | da123777 | settings-people.js: Blocked people goes down by one per Unblock, like Muted, and the list is not read for the count. Pin amended in settings_people_test.mjs. |
| WR-B05 | f0d7bd58 | person.js: a follow 409 refreshes and lets the server's can_follow decide; focus stays on Follow, or goes to More when Follow is gone. Pin amended in person_test.mjs (the 409 test split in two: cannot follow, and cap). |
| WR-B06 | c2e1453b | people-list.js: Show more stays shown, disabled and aria-busy while loading, a second tap sends nothing, and focus goes to the first new row (or back to the button). |
| IN-B01 | 6f3f19ef | people-list.js (Unblock, Unmute, Remove all) and settings-people.js: the server's 409 or 429 sentence is shown, and the offline line when the signal drops mid call. |
| IN-B02 | a5bb961e, 2c2d4623 | stars.js: set() does nothing while a save is out. The second commit strengthened the test (it first passed against the old code). |
| IN-B04 | b90c3fa7 | settings-people.js: when the page shows zero followers, the count is read once more before the switch turns off; a follower who arrived since makes the box ask. A failed read keeps the shown zero. Pin amended in settings_people_test.mjs. |
| IN-B05 | 97064bf0 | votes.js: a 404 on a rating hands focus to Undo (as Skip does) and destroys the control so its settle step cannot focus a removed star. |

Contract tests under web/*_test.go were not touched and pass.

## Not fixed

### K2: the star box for the reader whose resolve closes the report

**Reason:** no host that survives the card closing exists without a template and
CSS change. The only candidate named in the review, the toast (`#toast`, the
element that carries "Marked resolved" and Undo), cannot hold the box as it is:

- it is a dark `--color-text` box that is laid out as one flex row (text, then
  Undo) with `width: max-content`; the light bordered `.rate-box` with five 44px
  stars and Skip does not fit that row on a phone and has no style inside it
  (main.css `#toast.toast--action`, stars.css `.rate-box`);
- it hides after 8 seconds, and any later `showToast` (including the Undo
  result) empties it with `setText(toastEl, '')`; UI-SPEC 3b says the box stays
  until the report view closes;
- it is a `role="status"` live region, so putting a control inside it is a
  poor place for focus.

**What is needed (templates fixer or a later change):**

1. A persistent host next to the toast, for example a node with the id
   `rate-host`, and one CSS rule (owned by the templates fixer) that places it
   above the toast, on both phone and laptop, with the same look as `.rate-box`
   and a z-index under the toast. The pages that load votes.js are
   `index.html.tmpl`, `low.html.tmpl`, `profile.html.tmpl`, `demo.html.tmpl`
   and `demo_activity.html.tmpl`. votes.js could also create the host node
   itself on first use, so the hard need is then only the CSS rule, not a
   template edit.
2. Then a small votes.js change (not done here, because without the host's CSS
   it would draw an unplaced box): when `rateHostIsLive(block)` is false,
   `mountRateBox` mounts into that host, and the existing Undo and Skip paths
   (`removeRateBox`) already remove it. `rateBoxes`, `rateSkipped` and
   `buildRateBox` need no change.

### IN-B03: Unmute or Unblock for a person who renamed since the list was read

**Reason:** the fix needs the server's unmute and unblock answers to say whether
a row changed (the `:execrows` count is already there). Those are handler and
service files, owned by the server fixer. Left for a later round; the browser
side is one line (reload the list when the answer says 0 rows).

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer), part browser_
