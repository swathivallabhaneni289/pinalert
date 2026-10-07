---
phase: 03-trust-model-hardening-diversity-weighted-trust
reviewed: 2026-10-08T00:00:00Z
depth: standard
files_reviewed: 31
files_reviewed_list:
  - cmd/server/main.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/page.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/trust.go
  - internal/api/handlers/votes.go
  - internal/service/auth.go
  - internal/service/currency.go
  - internal/service/feed.go
  - internal/service/geo.go
  - internal/service/reliability.go
  - internal/service/report.go
  - internal/service/reporter.go
  - internal/service/trust.go
  - internal/service/trustcopy.go
  - internal/service/trustview.go
  - internal/store/migrations/00005_add_reports_session_expires_index.sql
  - internal/store/queries/reliability.sql
  - internal/store/queries/votes.sql
  - internal/testutil/history.go
  - web/static/css/main.css
  - web/static/css/trust.css
  - web/static/js/activity.js
  - web/static/js/app.js
  - web/static/js/feed.js
  - web/static/js/map.js
  - web/static/js/trust.js
  - web/static/js/visibility.js
  - web/static/js/votes.js
  - web/templates/index.html.tmpl
  - web/templates/profile.html.tmpl
findings:
  critical: 0
  warning: 8
  info: 4
  total: 12
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-10-08
**Depth:** standard
**Files Reviewed:** 31
**Status:** issues_found

## Summary

The server side of the trust math is sound. One resolver (Resolve), one independence predicate (independentCellCount), one vote filter (filterVoteRows) feed the tally, the counters, the currency signal and the reporter rating, so N and the visibility cannot drift. The 1 km rule fails closed on NaN and Inf, the reporter exemption is decided from the database identity and not from the body, the reporter integer arithmetic does not overflow, the SQL window and cap are correct, and migration 00005 is reversible. No XSS sink exists in any reviewed JS file (no innerHTML, outerHTML, insertAdjacentHTML or eval; every string goes through Pinalert.setText) and the template attribute `data-trust` is a Go string run through html/template attribute escaping.

The defects that matter are mostly on the browser side, where the server's decision is displayed or where the voter's position is captured. They are listed by the priority order in the task: gaming of the distance rule and the reliability signal first, then server and browser disagreement, then races, then smaller items. No finding below needs a code change to the resolver itself. Spoofed coordinates, email aliasing and vote rate limiting are documented limits in docs/TRUST-MODEL.md and are not re-reported.

No critical (data loss, auth bypass or injection) issue was found.

## Warnings

### WR-01: Out of order poll responses overwrite newer state, including the default view showing Hidden reports

**File:** `web/static/js/app.js:129-174`
**Issue:** `fetchReports` has no sequence guard. Every caller (30 second poll at line 182, the post-vote refetch in votes.js:996, the Show disputed change handler in feed.js:650, Retry, pageshow) starts an independent fetch and each `.then` unconditionally assigns `state.reports`. Two concrete failures on a slow mobile link: (1) The reader ticks Show disputed, fetch A (`show_disputed=true`) starts, they untick, fetch C (default view) starts, and A returns after C. `state.reports` now holds the disputed-only set while `state.showDisputed` is false and the checkbox is off, so Hidden reports render in the default feed with live vote controls for up to 30 seconds. D-10 says the default view must never contain Hidden reports. (2) A poll starts at T, the vote is written at T+0.1, the post-vote refetch returns at T+0.5, and the poll (which the server read before the insert) lands at T+0.6. The row goes back to the pre-vote `your_vote` and counters, which is exactly the "optimistic looking window" votes.js:989-995 says it forbids, and the busy block has already been released.
**Fix:**
```js
var fetchSeq = 0;
function fetchReports() {
  var mySeq = ++fetchSeq;
  var sentDisputed = state.showDisputed;
  // ... build url from the captured values ...
  return fetch(url).then(/* parse */).then(function (body) {
    if (mySeq !== fetchSeq || sentDisputed !== state.showDisputed) {
      return; // a newer request owns the state
    }
    state.reports = (body && body.reports) || [];
    // ...
  });
}
```
Apply the same guard in the catch branch so a stale failure does not flip a good state to error.

### WR-02: A cached position of any age is reused as the voter's location, which silently defeats the 1 km rule for honest voters

**File:** `web/static/js/votes.js:264-294`
**Issue:** The freshness bound (60 seconds) only decides whether to skip the permission check. Two paths then reuse the cache with no age limit at all: `decide('granted')` falls back to `useCached()` whenever the silent fix fails (line 276-277, for example an 8 second timeout indoors), and the prompt or unknown branch returns `useCached()` for any cached entry (line 289-291). The cache lives in sessionStorage for the whole tab session. Scenario: a reader votes on a report near the market at 09:00 (fix cached). At 15:00 they are at home 20 km away, indoors, GPS times out, and they confirm another report near the market. The browser sends the 09:00 coordinates, the server sees a voter 300 m away, and a confirmation from a place the person is not standing in is counted. votes.js:6-15 states this is the one thing the module must never do. The server cannot detect it because only coordinates arrive.
**Fix:** Give stale reuse a hard ceiling that is not the 2 minute hint bound but still short, and reject beyond it.
```js
var VOTER_LOCATION_STALE_MAX_MS = 10 * 60 * 1000;
function useCachedIfUsable() {
  if (cached && cached.at !== null && Date.now() - cached.at < VOTER_LOCATION_STALE_MAX_MS) { useCached(); }
  else { deny(); }
}
```
Use it in both fallback sites. An entry with `at === null` should be treated as unusable, not as a usable stale fix.

### WR-03: The too far hint swallows the tap, so a stale fix can lock a now-nearby voter out for minutes with no request ever sent

**File:** `web/static/js/votes.js:947-949` (with `applyDistanceHint` at 601-631)
**Issue:** `applyDistanceHint` marks Confirm, Dispute and Mark resolved `aria-disabled` when the cached fix is under 120 seconds old and farther than the radius. `onControlsClick` then returns early on `aria-disabled`, so no POST and no location refresh happens. The cache is only rewritten by a vote attempt, which is now blocked, so the mark persists until the fix ages past 120 seconds and the next 30 second poll clears it. Scenario: a reader walks toward a flooded road, votes once from 1.4 km away (refused, fix cached), keeps walking, and at 300 m taps Confirm repeatedly. Nothing happens for up to 2.5 minutes. docs/TRUST-MODEL.md says the hint is advisory and "only the server decides", but the code makes the hint decide.
**Fix:** Treat the tap on a marked button as a request for a fresh fix, not a no-op. On a marked button, call `getVoterLocation({ fresh: true })`, re-run `applyDistanceHint` with the new fix, and only send the vote if it is now within range (or send it and let the server answer). At minimum, drop the mark when the fix is older than the poll interval.

### WR-04: The reporter's distance exemption is unusable in the browser because castVote always demands a GPS fix first

**File:** `web/static/js/votes.js:310-317`, `web/static/js/activity.js:124`, server side `internal/service/trust.go:398-413`
**Issue:** The server exempts the reporter's own resolve and reopen from the 1 km rule (D-01) precisely so the person who posted can close their report from anywhere, for example after leaving the area. The client does not mirror this: `castVote` unconditionally awaits `getVoterLocation`, which rejects with the "Voting needs your location" sentence when permission is denied or no fix exists. A reporter who denied location, or who is on a desktop with no fix, cannot press Mark resolved or Reopen on their own report even though the server would accept it. The browser and the server therefore disagree about what the reporter may do. (The body defaults to 0,0 in the handler, see IN-01, so the server would accept it.)
**Fix:** Add an option for own-report resolution votes: `castVote(reportId, action, { skipLocation: true })` that sends the last known fix if one exists, else `{latitude: 0, longitude: 0}`, and have `onControlsClick` and `onReopenClick` pass it when `report.is_own_report` (feed and popup) or on the Activity page, where every row is the viewer's own. Better, make the handler accept a body without coordinates for resolution kinds and keep the requirement for confirm and dispute.

### WR-05: Neutral rows consume the 30 report history cap, so a bad reporter can wipe their record with a day of unconfirmed critical reports

**File:** `internal/service/reliability.go:243-245` (cap includes neutral), `internal/service/reporter.go:128-139`, `internal/store/queries/reliability.sql:38-55`, `internal/service/report.go:396-408`
**Issue:** The history keeps the latest 30 expired reports and only then drops neutral rows (RESOLVED Q4 chose this to mirror the SQL). A critical or rescue needed report that expires with fewer than 2 confirmations and fewer than 2 disputes is neutral (D-12). Critical reports publish as Live instantly with no gate (Resolve rung 2), and there is no submit rate limit before Phase 4. Scenario: an account with 12 judged reports that mostly expired Hidden (score 20, "Unreliable reporter") files 30 critical reports in a few minutes. Each is Live on every nearby map for 24 hours. When they expire unconfirmed, the 30 newest history rows are all neutral, judged is 0, and the signal becomes "New reporter" with no score. The unreliable record is erased and the spam cost them nothing. The same mechanism keeps an honest reporter's good record from being pushed out only if they stay inactive.
**Fix:** Apply the cap after dropping neutral rows, in both places so the SQL and the Go function stay equivalent. In SQL this needs vote data, so either read a larger fixed window (for example 90 days with a higher row cap of 100) and cap judged rows in Go, or make the cap count only rows that are not critical or rescue needed. Document the choice in docs/TRUST-MODEL.md. A cheaper partial fix is to let a neutral critical report that was disputed by one place still count as unconfirmed.

### WR-06: Fade and age are computed from the visitor's clock, so the browser and the server disagree on how old a report is

**File:** `web/static/js/app.js:301-317`, `web/static/js/trust.js:483-494`, `web/static/js/app.js:277-295`
**Issue:** `ageStage` and `relativeTime` use `Date.now()` against the server's `created_at` and `expires_at`. `fadeStage` takes the worse of that and the server's currency stage. A phone clock set a day behind (common on devices without automatic time) makes `remainingFraction` huge, so a report one minute from expiry renders `age-fresh` at full saturation, reads "just now" (negative diff is clamped to 0 in `relativeTime`), and the server's stale stage is the only thing that can dim it, and the server sends no stage for critical or rescue needed reports (trust.js:482). The server already says the report "has expired" in the lead sentence while the pin looks fresh. A clock a day ahead does the opposite and dims everything. trust.js:7-11 claims the module holds no date arithmetic, yet `fadeStage` calls `ageStage`.
**Fix:** Have the server send the time stage (or its `now`) in the feed response, and compute `age` from `server_now - created_at`, or compute a client offset once from the response `Date` header and apply it to `Date.now()`.

### WR-07: Geolocation is requested at default accuracy, so the place count tracks GPS noise and the 1 km line is fuzzy

**File:** `web/static/js/votes.js:260` (options `{ timeout: 8000, maximumAge: 0 }`), `internal/service/trust.go:35`
**Issue:** `enableHighAccuracy` is not set, so phones return network or cell based fixes whose error is routinely 100 m to over a kilometre, and a desktop returns Wi-Fi or IP based fixes. A geohash 7 cell is about 150 m. Two people standing side by side with independent coarse fixes land in different cells and are counted as two "different places", which is the over-count the cell rule exists to prevent (N is documented as a lower bound on people, but here it measures noise). The same error pushes honest voters across the 1 km line. docs/TRUST-MODEL.md lists "accuracy not read" as a limit, but requesting high accuracy is a one line change that removes most of the noise, and `pos.coords.accuracy` is already in hand.
**Fix:**
```js
{ enableHighAccuracy: true, timeout: 8000, maximumAge: 0 }
```
and reject (or flag) a fix whose `pos.coords.accuracy` exceeds a documented bound such as 300 m before it is cached.

### WR-08: A failed post-vote refetch hides the entire list although the comment promises stale rows stay

**File:** `web/static/js/feed.js:398-406`
**Issue:** In the error state `render` hides `reportListEl` and shows the error panel. The comment says a fetch failure "never clears the rows", but hiding the `<ol>` removes every row from view and from the tab order while the map keeps its pins. `fetchReports` swallows its own errors (app.js:169-173), so a vote that succeeded followed by one failed refetch (or any single failed 30 second poll on a flaky link during a flood) drops the whole list behind "Couldn't load reports" even though the vote was recorded and the data on screen was at most 30 seconds old. During an emergency, stale data is more useful than none, which is the comment's own premise.
**Fix:** On error, keep the list visible when `state.reports.length > 0` and show the error as a non blocking banner with Retry; only hide the list when there is nothing to show.

## Info

### IN-01: The vote body does not require coordinates, so a missing field becomes 0,0

**File:** `internal/api/handlers/votes.go:40-45`, `133-146`
**Issue:** `CastVoteRequest` uses plain `float64` fields, so `{}` decodes as latitude 0 and longitude 0, passes range validation, and is evaluated as a real position. For a confirm or dispute this yields a 403 "Too far" (and the client then runs its silent retry), which hides a malformed request as a location refusal. It also makes WR-04's skip workaround accidentally work. The decoder also accepts trailing data after the object.
**Fix:** Use `*float64` fields and return 400 `latitude` / `longitude` when nil for confirm and dispute, and call `dec.More()` or a second `Decode` to reject trailing tokens.

### IN-02: The poll interval is hard coded in the template although PageConfig says page values are server configured

**File:** `web/templates/index.html.tmpl:29`, `internal/api/handlers/page.go:16-18`
**Issue:** `data-poll-interval-ms="30000"` is a literal, while the neighbouring attributes come from `PageConfig`. The server's comments cite the poll as the staleness bound for trust numbers, so the two can drift.
**Fix:** Add `PollIntervalMs` to `PageConfig` and `pageViewModel`, set it in `cmd/server/main.go`, and render it.

### IN-03: The Activity page shows the "Report reopened." toast before it looks at the response

**File:** `web/static/js/activity.js:138` (check at 161)
**Issue:** The toast is unconditional on a 200, and the visibility check that decides whether the row is still Resolved comes after. The comment argues the retracted outcome is unreachable, which holds today, but if Resolve ever changes the page would announce success over a row that remains Resolved.
**Fix:** Move the toast after the `updated.visibility !== RETRACTED_VISIBILITY_SLUG` test and use `PinalertVotes.resolutionOutcomeMessage('reopen', result)`, which exists for this.

### IN-04: Post-success render exceptions are reported as a failed vote

**File:** `web/static/js/votes.js:983-1019`
**Issue:** One `.catch` covers both `castVote` and the work after it (`captureRemovalFocus`, `closeResolveConfirm`, `focusAfterVote`). If any of those throws on a detached node, the reader sees "Couldn't record your vote" for a vote that was recorded, and for a resolve the confirmation is closed while the toast never shows. The reader retries and sends a duplicate.
**Fix:** Split the chain: handle the castVote rejection in its own `.catch`, and wrap the post-success rendering in a `try` that logs and never shows the failure sentence.

---

_Reviewed: 2026-10-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
