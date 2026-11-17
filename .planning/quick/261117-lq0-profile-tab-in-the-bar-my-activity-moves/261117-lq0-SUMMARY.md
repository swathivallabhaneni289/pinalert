---
quick_id: 261117-lq0
slug: profile-tab-in-the-bar-my-activity-moves
date: 2026-11-17
status: complete
commits: 3
plan_head_before: 406f36ce247dc8e14311e5faa203dceb5d26251e
---

# Quick 261117-lq0: Profile tab in the bar, My activity in Settings, Public profile switch

The bars now read Nearby, Official alerts, Profile (plus + Report on the phone). My activity's five parts open from a new first section in Settings. The switch is now called Public profile, with the line "Anyone can see your profile and follow you."

## What changed

1. **The main places** (c41bd2a6)
   - `mainNavItems` is now Nearby, Official alerts and Profile, with Profile linking to `/me` and using the `user` icon. `NavProfile = "profile"` is new.
   - On your own profile, Profile is marked current, the title is My profile and there is no back arrow.
   - Someone else's profile, and the not found and load failed states, are unchanged.
   - The Demo now builds its bar from its own list (`demoNavItems`), so it still shows My activity at /demo/activity.
   - `/profile` marks no bar item current. Its back arrow goes to `/settings`, both on the list and on a part.
2. **Settings** (7f399cb4)
   - New first section, My activity, built like the other sections. It has five link rows with a chevron: Updates, Posted by you, You confirmed, You disputed, Ended nearby.
   - The rows are built in Go from `activityPartSpecs` and `activityPartHref` (`activityLinks()`), the same source the hub uses.
   - Removed:
     - the Following, Followers and View my profile rows
     - their openers and count writes in `settings-people.js`
     - `FollowingCount`
     - the laptop hide `.place-row.place-row--phone-only` and its `sizeOnlyNamed` entry
   - The switch reads Public profile, with the sub line in the Notifications row pattern.
3. **Copy** (80956af0)
   - Welcome step 4 Name help now says "Shown only if your profile is public."
   - Three Policy sentences now name Public profile: lines 107 and 123, plus a third at line 143. The plan's grep step found line 143 ("...shown on your profile while Let others follow me is on").

## Tests

- `node --test web/*.mjs`: 1649 tests, 1649 pass, 0 fail, 0 skipped.
- `go build ./...` and `go vet ./...` are clean.
- `go test ./... -p 1 -count=1 -v` on the scratch database `p9_lq0`, run with `env -i`: exit 0, 2494 top-level PASS, 0 FAIL, 3 SKIP.
  - The 3 skips are the opt-in live checks: TestGeocodeLive, TestOfficialLive and TestScreeningFixture.
  - No database test was skipped.
- New pins:
  - **Nav list, Demo and My activity (unit):** `TestNavForMarksOnlyTheCurrentPlace` pins the nav list and the Demo bar, and that My activity marks no place.
  - **Profile current (e2e):** `TestAppBarProfileCurrent` checks that:
    - `/me` lands on `/u/<you>`;
    - Profile is current on your own profile only, with the title My profile and no back arrow;
    - someone else's profile and Settings mark nothing.
  - **`/profile` bar (e2e):** `TestAppBarCurrentItem` checks that `/profile` and a part mark nothing and go back to `/settings`.
  - **Settings rows (e2e):** `TestSettingsRows` checks that My activity is the first section, with its five rows, the hub's hrefs, and no sub lines, counts or icons.
  - **Settings template:** the contract tests pin the switch label and sub line and that the three removed rows are absent.
- Every amended test has a dated "Amended 2026-11-17 (quick 261117-lq0)" note.

## Live look

Playwright WebKit against a scratch server on port 8794, using database `p9_lq0` (migrated, then seeded with `cmd/seedprofiles`) and the log mailer, signed in as the made-up `anita.r@example.com`. It ran 238 checks at phone 320 and 390 and laptop 1280, light and dark: 238 passed, 0 failed.

- **Bottom bar:** Nearby, Official alerts, Profile, Report fit at 320 with no wrapped or clipped label, every item inside the screen, and nothing scrolling sideways.
- **Profile marking:** Profile is marked on `/u/anita.r` (reached through `/me`). It is not marked on Settings, `/profile`, any part, or `/u/ravi.k`.
- **Settings:** opens with My activity first. Each of the five rows opens its part. The phone back arrow (laptop: browser Back) returns to Settings.
- **Switch:** reads Public profile with its line. The three removed rows, and the words "View my profile" and "Let others follow me", are gone.
- **Laptop:** the top bar shows Profile, and the account menu is unchanged (My profile /me, Settings /settings, Log out).

Screenshots are in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/lq0/shots/`, for example:
- `phone320-light-own-profile.png`
- `phone320-dark-settings.png`
- `phone390-light-settings-people.png`
- `laptop1280-dark-settings-top.png`
- `laptop1280-light-account-menu.png`

The server was stopped, the database dropped and the server log deleted.

## Deviations from Plan

1. **[Rule 2] The Followers count stays, as `data-followers`.** The Turn off box decides whether to ask by reading the Followers count. Removing it would have changed behaviour the plan says must stay the same. So `FollowersCount` stays in the view model, drawn only as `data-followers` on `settings-people-wrap`, and the script reads and updates it in place of the removed row. `FollowingCount` is gone.
2. **Third Policy sentence.** The Policy had a third on-screen sentence naming the old switch (line 143). The plan's grep step covers it, so it now names Public profile too.
3. **Account menu test.** `TestProfileRendersSharedHeader` pinned `href="/profile"`, which only the old bar item matched. It now pins the menu's own `/me` item, and the menu itself is unchanged.
4. **Not changed: Go comments and swagger.** Go code comments, the swagger `@Summary`, and the docs/ files that name "Let others follow me" are not on-screen copy and were left alone.
5. **Note:** `gofmt -l` reports `place_test.go`, `places_e2e_test.go`, `internal/geocode/photon.go` and `internal/store/comments_test.go`. All four were already unformatted at the base commit and were left alone.

## Self-Check: PASSED

- Commits c41bd2a6, 7f399cb4 and 80956af0 are on the branch after 406f36ce.
- None of them stages STATE.md, ROADMAP.md or REQUIREMENTS.md.
