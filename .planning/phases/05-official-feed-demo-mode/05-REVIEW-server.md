---
phase: 05
part: server
reviewed_at: 2026-10-29T00:18:48Z
reviewed_commit: 1f44177d
depth: standard
status: issues_found
findings:
  critical: 1
  warning: 8
  info: 11
  total: 20
---

# Phase 5 server review: official alerts, official push lane, Demo, globe tiles

Scope: the 56 server files in the orchestrator's list (Go, SQL, the workflow). Read in full: the official package, the official, Demo, cron door and globe handlers, the router, the push lane, account deletion, alert settings, migrations 00012 and 00013, both query files and the workflow. Read as their Phase 5 diff only: auth.go, legal.go, low.go, page.go, me.go, settings.go, nav.go, reports.go, votes.go, profile.go, pushkinds.go and pushworker.go. Only grepped or skimmed: internal/demo/memquerier.go, internal/demo/seed.go, cmd/buildareas/main.go and internal/testutil/official.go (a test seeding helper). "Checked and clean" covers only what was read. Tests were read only to judge what is proven. `go build ./...` and `go vet` on the touched packages are clean, and `DATABASE_URL= go test -short -race` passes for `internal/official`, `internal/demo` and `internal/areas`. No database was used, so every finding about SQL behaviour comes from reading the queries. Each one is marked verified in code or unverified.

## Critical Issues

### CR-S01: A cancelled or replaced alert stays listed, and can raise the banner, when its Cancel or Update was stored first

**File:** `internal/official/refresher.go:458` (newest first order), `internal/official/store.go:63-99`, `internal/store/queries/official.sql:84-111` and `:145-169`

**Issue:** The refresher fetches and saves new items newest first (`selectNew` sorts by pubDate descending, and the save loop at `refresher.go:393-419` follows that order). A Cancel C, or an Update U under a new guid, is newer than the alert X it withdraws, so it is saved first. Its `CancelOfficialAlertsByIdentifier` or `SupersedeOfficialAlertsByIdentifier` then matches no row, because X is not stored yet. X is saved later in the same run through `UpsertOfficialAlert`, which writes `cancelled_at = EXCLUDED.cancelled_at` (NULL for an Alert) and `superseded_at = NULL`. `OfficialAlertsInForce` checks only those two columns, so X is listed as in force until its own expiry. If X is Severe or Extreme on the reader's district, `pickBanner` chooses it. The push path already knows about this order: `OfficialAlertWithdrawnByStored` (`official_push.sql:248-259`, used at `pushenqueue.go:93`) refuses to queue X. The display path has no such check.

The same upsert also resurrects a withdrawn alert when its own guid is fetched again with a new pubDate but the same `cap_identifier`: the cancel or supersede mark set by another guid is cleared, and nothing sets it again.

Failure scenario (verified in code, not run on a database): a cold start or a database reset leaves about 100 unseen items, and 40 are fetched per run, newest first. IMD issued an Extreme cyclone warning X for Krishna 20 hours ago, and two hours ago issued an Update U under a new guid that downgrades it to Moderate. U is fetched in run 1; X is fetched in run 2 or 3. From then on the page lists both X (Extreme) and U (Moderate) and the Nearby banner reads "Cyclone warning · Krishna district" in red, for a warning the issuer withdrew. The same happens inside one run when X and its Cancel are both new. This breaks ALERT-04 and the core promise that nothing withdrawn is shown as current.

**Fix:** Settle the marks at write time, so the view, `SkipStaleOfficialPushes` and worker step 2 all see the same truth. Add one query and call it in `saveVersion` after the upsert of an Alert or Update (not for a Cancel row, which is already cancelled):

```sql
-- name: MarkWithdrawnByStored :exec
-- A message stored earlier, under another guid, may already withdraw this one:
-- the refresher stores the newest first.
UPDATE official_alerts a
SET cancelled_at = CASE WHEN EXISTS (
        SELECT 1 FROM official_alerts n
        WHERE n.guid <> a.guid AND n.msg_type = 'Cancel'
          AND a.cap_identifier = ANY(string_to_array(n.references_text, ' ')))
      THEN COALESCE(a.cancelled_at, sqlc.arg(now)::timestamptz) ELSE a.cancelled_at END,
    superseded_at = CASE WHEN EXISTS (
        SELECT 1 FROM official_alerts n
        WHERE n.guid <> a.guid AND n.msg_type = 'Update'
          AND a.cap_identifier = ANY(string_to_array(n.references_text, ' ')))
      THEN COALESCE(a.superseded_at, sqlc.arg(now)::timestamptz) ELSE a.superseded_at END
WHERE a.id = sqlc.arg(id)::bigint;
```

```go
// store.go, after the upsert and the area rows, when !cancel:
if err := qt.MarkWithdrawnByStored(ctx, sqlcgen.MarkWithdrawnByStoredParams{ID: stored.ID, Now: now}); err != nil {
    return SavedVersion{}, nil, fmt.Errorf("official: applying stored withdrawals: %w", err)
}
```

The hook then reads the mark through `OfficialAlertForPush` and refuses as `withdrawn`, so `OfficialAlertWithdrawnByStored` becomes a second guard.

**Test that proves it:** a database test in `refresher_db_test.go` that serves, in one fake feed, the Cancel (pubDate 2) and its target (pubDate 1), refreshes once, and asserts the target is not in `OfficialAlertsInForce` and `Service.Alerts` has a nil banner. A second case serves an Update under a new guid before its target, across two runs with `CAPPerRun` items between them. A third re-fetches a cancelled target under its own guid with a new pubDate and the same identifier and asserts it stays cancelled. `TestRefreshUpdateAndCancel` covers only the target first order.

## Warnings

### WR-S01: A partial SACHET run marks the feed fresh while unfetched items may still be in force

**File:** `internal/official/refresher.go:385-387`, `:421-427`, `:313`

**Issue:** `Fetch` succeeds whenever at least one needed message was saved. With `leftover` (more than 40 new items) or with some CAP failures, it keeps the old validators but still returns success, so `run` calls `MarkFeedSuccess` and `last_success_at` moves to now. The page then says fresh and "Last checked just now", and an empty list in a fresh state is read as "no official alerts in force".

Failure scenario (verified in code): after a cold start or a database reset, 100 unseen items arrive and the 40 newest are stored. An Extreme cyclone warning for the reader's district, issued 18 hours ago and still in force, is in the other 60. For the next one to three runs (MinRefreshGap 4 minutes, so up to about 15 minutes with the ticker and the 10 minute cron), the page says fresh and shows no alert for that district.

**Fix:** Do not move `last_success_at` while `leftover` is true: return a distinct result (for example `res.Partial = true`) and in `run` record the attempt and the counts but skip `MarkFeedSuccess` while partial. Leave the failed CAP case as it is, or count it as partial only for the first few runs, because one CAP that fails every time must not keep the feed unavailable for ever. That last point needs an owner decision.

**Test:** in `refresher_test.go` with the fake stores, serve `CAPPerRun + 10` new items, run once, and assert `success` was not called and `LastSuccessAt` is unchanged; run again and assert it moves.

### WR-S02: When the 55 second budget ends during the CAP fetches, the run saves nothing and makes no progress

**File:** `internal/official/refresher.go:393-419` and `:313`, `internal/official/sachet.go:291-313`

**Issue:** `FetchCAPs` returns only when every fetch has finished, and the saves start after it, with the same run context. If the deadline passes while the fetches run, every `save` fails at `Begin` on the expired context, so `failed == len(need)` and the run fails with nothing stored, although many messages were fetched. The next run fetches the same 40 again. The success mark at line 313 also uses the run context, so a run that saved everything just before the deadline can fail to record its success, and it records no failure either.

Failure scenario (verified in code; SACHET was fast on 2026-10-29, 40 CAPs inside 8.5 s): on a day when SACHET answers each CAP in about 4 s or more, 40 CAPs at 3 at a time take more than 55 s. Every run then fetches 41 documents, stores none, and the feed drifts to stale and then unavailable while SACHET is up.

**Fix:** Save each message as its result arrives (FetchCAPs can take a callback or return a channel), so the work done before the deadline is kept. At the least, run the saves and the success mark on `context.WithTimeout(context.WithoutCancel(ctx), markTimeout)`, as the failure mark already does, and skip saves for items whose fetch failed.

**Test:** a fake `sachetAPI` whose `FetchCAPs` returns 40 good results only after the context ended; assert at least those results are stored, or, with the minimal fix, that `save` received a live context.

### WR-S03: GDACS request A never clears is_current, so an event GDACS stopped calling current stays on the map

**File:** `internal/official/gdacs.go:284-292`, `:294-305`

**Issue:** `filterGdacsA` drops every feature GDACS marks not current, so the stored row of an A event is never updated with `is_current = false`. Only events that request B also returns (EQ, TC and FL inside the South Asia box) get the flip, which is what the `filterGdacsB` comment describes. Every volcano and wildfire, and every A event outside the box, keeps `is_current = true` and stays in `CurrentOfficialHazards` until `COALESCE(to_date, last_seen_at)` plus 48 hours. `Pins` serves all current rows to every reader, so the map draws them.

Failure scenario (verified in code): a Red cyclone in the Bay of Bengal outside the box edge, or a wildfire anywhere, with a `todate` a day ahead, is dropped from GDACS's current list. Pinalert keeps drawing it as a current pin for up to three days.

**Fix:** In `filterGdacsA`, drop only droughts and keep non-current events so the upsert stores `is_current = false`; `CurrentOfficialHazards` already filters on it. Request A only asks for Orange and Red, so this stores no extra noise.

**Test:** in `gdacs_db_test.go`, flip `iscurrent` to false in the A fixture (`e.srv.a`) for an event outside the box and assert it leaves `CurrentOfficialHazards`. The existing subtest flips only `e.srv.b`.

### WR-S04: GDACS rows that already ended are listed and counted as "In force now", and a missing GDACS date is sent as the year 1

**File:** `internal/official/view.go:301-316`, `:416-451`; `internal/api/handlers/official.go:259-260`

**Issue:** `hazardsNear` keeps hazards up to 48 hours after their end, which suits the map. `BuildView` adds them to the reader and home groups with no end check (alerts are filtered at line 268, hazards are not), and `v.Count` counts them. The page draws the title "In force now" with its own count of the group rows (`countRows(view)` in `web/static/js/official-alerts.js:638-650`), which includes those hazard rows too. Separately, `hazardEntry` copies `FromDate` and `ToDate` into `IssuedAt` and `ExpiresAt`, and `officialViewToResponse` formats a zero time as `0001-01-01T00:00:00Z`. `Date.parse` accepts that, so `clockWithDay` prints a made up "Until" time and `relativeIssued` a huge age. The pins route sends null for the same zero dates, so the two routes disagree.

Failure scenario (verified in code): an Orange earthquake in the reader's district 30 hours ago (GDACS `todate` equals the event time) is listed under "In force now" with "Until" yesterday, and counted.

**Fix:** Keep the row, because D-12 lists an event in the reader's district and an earthquake's `todate` is about its event time: put hazards whose `ToDate` has passed in their own "Recent, ended" group (or mark the row ended), leave them out of `Count`, and have the page title count exclude them. Do not drop them from the list. In `officialViewToResponse`, write `""` for a zero `IssuedAt` or `ExpiresAt` (the page already draws nothing for an unparseable time).

**Test:** a `view_test.go` case with a hazard whose `ToDate` is 30 hours before `Now` (assert it is listed, marked ended and not counted), and a handler test with a zero `FromDate` asserting `issued_at` is empty.

### WR-S05: The push candidate read is capped before the area filter, so people in the affected district can be skipped silently

**File:** `internal/store/queries/official_push.sql:45-66`, `internal/official/pushenqueue.go:114-147`

**Issue:** `OfficialPushCandidates` returns the first `recipientCap * candidateFactor` (8000) accounts nationwide, ordered by id, and only then are their cells matched to the alert. Once more than 8000 accounts have a home area, the switch on and a live device, an account in the alert's district whose id is past the first 8000 is never queued, and nothing logs it.

Failure scenario (verified in code; latent at today's scale): 9000 eligible accounts, mostly in Maharashtra, and a Severe flood warning for Krishna. The Krishna accounts with ids above the first 8000 eligible ones get no notification.

**Fix:** Page through candidates by id (`WHERE a.id > $after ORDER BY a.id LIMIT $page`) until `recipientCap` matches are queued or no rows are left. Alternatively pre-filter in SQL by the geohash prefixes that cover the alert's district boxes, then match in Go as now.

**Test:** in `pushenqueue_db_test.go`, lower `recipientCap` to 2 (the seam exists), make the 8 lowest id candidates live outside the district and one higher id candidate inside, and assert that candidate is queued.

### WR-S06: Demo visit limiter (orchestrator item c): it charges navigation, not new worlds, and a few addresses can end every live visit

**File:** `internal/api/handlers/demo.go:131-143`, `internal/api/handlers/demo_page.go:117-124` and `:194-213`, `internal/demo/store.go:97-121`, `internal/ratelimit/perip.go:120-131`

**Issue:** Memory is bounded independently of the rate: `MaxWorlds = 100`, each world capped at 40 reports and 400 votes, idle worlds dropped after 30 minutes, and least recently used eviction. That part is clean. The limiter itself has three problems:

1. All three Demo pages (`/demo`, `/demo/alerts`, `/demo/activity`) sit behind one per address bucket (burst 30, then one every 6 s), and a page that reuses a living world through `?v=` still spends a token. Behind a carrier NAT address, many visitors share 30 page loads and then 10 a minute; moving between the three places of one visit costs the same as starting new worlds.
2. The limiter does not protect other visitors' worlds. Creating 100 worlds evicts every world idle longer than the newest one. A visitor whose tab is in the background (no polling) loses the world and sees "This Demo has ended" on return. Four IPv4 addresses using their burst, or one IPv6 host rotating addresses inside its /64 (each full address is its own key), can do this continuously.
3. Without `CLIENT_IP_HEADER` behind a proxy, every Demo visitor shares one key, so the 30 burst is site wide. That is a deployment note, not a code bug.

`PerIP` has an idle sweep but no entry cap, so rotating IPv6 addresses grow the map until the 10 minute sweep. This is the pre-existing package, now used by three more public, sessionless limiters in this phase.

**Fix:** Call the visit limiter only on the `store.Create` path: pass the limiter into `demoVisitFor` and check `Allow` just before `store.Create()`, and drop `r.With(demoVisitLimiter(...))` from the three page routes. Key IPv6 addresses by their /64 in the Demo limiters (and ideally in `ratelimit`), and give `PerIP` a maximum entry count. Optionally exempt worlds touched in the last minute from eviction and answer 503 "The Demo is busy" when every world is that fresh.

**Test:** a handler test that opens `/demo`, then requests `/demo/alerts?v=<id>` and `/demo/activity?v=<id>` 40 times with a burst of 1, and asserts no 429; and a store test that creates 100 worlds after one touched world and asserts the touched world survives.

### WR-S07: District names reach pages and lock screens in raw LGD form (orchestrator item a)

**File:** `internal/areas/areas.go:136-138`; consumers `internal/official/service.go:448` and `:480`, `internal/service/pushworker_official.go:497`, `internal/demo/alerts.go:65-67`

**Issue:** `District.Name` is the LGD `dtname` as it is. Verified in the embedded file: 749 "Ntr", 515 "Spsr Nellore", 504 "Y.S.R.", 520 "Visakhapatanam" (misspelt), 608 "S.A.S Nagar", 303 and 304 "24 Paraganas North" and "24 Paraganas South", and the Delhi districts 77 to 85 and 670 as bare "Central", "East", "North", "North East", "North West", "South", "South West", "West", "South East". These names appear in "Where you are: Ntr district", in the banner ("Flood warning · Ntr district"), in the districts line, in a phone notification title ("Flood warning for Ntr district"), and in the Demo, whose pretend position is in NTR, so every Demo visitor sees "Ntr district". In Delhi, "East district" does not say which city.

I grepped every reader of `District.Name`: the four sites above, and nothing compares a district name with feed text (placement uses LGD codes and state names only). So a display table cannot change any match.

**Fix (smallest correct):** one code keyed table in `internal/areas`, applied in `Load` right where `District` is built, so every consumer gets it and a rebuild of the embedded file keeps it:

```go
// displayNames replaces LGD dtname values that read badly on a page or a lock
// screen. Keyed by LGD district code; names only, never used for matching.
var displayNames = map[int]string{
    749: "NTR", 515: "Nellore", 504: "YSR Kadapa", 520: "Visakhapatnam",
    608: "SAS Nagar", 303: "North 24 Parganas", 304: "South 24 Parganas",
    77: "Central Delhi", 78: "East Delhi", 79: "New Delhi", 80: "North Delhi",
    81: "North East Delhi", 82: "North West Delhi", 83: "South Delhi",
    84: "South West Delhi", 85: "West Delhi", 670: "South East Delhi",
}
// in Load: name := d.Name; if n, ok := displayNames[d.Code]; ok { name = n }
```

The same table can later hold state words, which also fixes IN-S07.

**Test:** in `areas_test.go` (package areas, so no import of internal/demo), every key of `displayNames` exists in `Default()` and `ByCode(749).Name == "NTR"`; in `internal/demo`'s own tests, `areas.Default().Locate(PretendLat, PretendLon)` returns "NTR" and `SampleView(nil, now).Banner.Area == "NTR district"`.

### WR-S08: The flaky EXPLAIN test depends on leftover planner statistics (orchestrator item b)

**File:** `internal/store/votes_test.go:565-639`

**Issue:** The test seeds no votes and runs `EXPLAIN` with `enable_seqscan = off`, so the planner chooses between `idx_votes_account_kind_report_created` and `idx_votes_report_account_kind_created` by cost alone. `TRUNCATE` empties the table but leaves the column statistics from earlier runs in `pg_statistic`, and `ANALYZE` on an empty table does not replace them. On a used database whose stats say one or two accounts own most votes, the literal `account_id = 1` looks unselective, and a full scan of the report leading index (already in `report_id` order for `DISTINCT ON`) can cost less. That fits "sometimes picks the other index on a used database". This root cause is my reading; it was not run, because no database was used.

**Fix (stable):** inside the transaction the test already rolls back, remove the competitor before the `EXPLAIN`. Postgres DDL is transactional, so the rollback restores it:

```go
if _, err := tx.Exec(ctx, "DROP INDEX idx_votes_report_account_kind_created"); err != nil {
    t.Fatalf("dropping the competing index inside the test transaction: %v", err)
}
```

With sequential scans off and the other index gone, the plan names the account index whenever it can serve the query, which is what the test means to prove. The cost is a brief ACCESS EXCLUSIVE lock on `votes` held by the test transaction, harmless on a disposable database. A second option that keeps both indexes: seed votes across about 200 accounts and run `ANALYZE votes` inside the transaction before the `EXPLAIN` (also rolled back), which makes the statistics known instead of leftover.

**Test:** run the test 50 times (`-count=50`) on a scratch database after first running the full store suite on it, and expect no failure.

## Info

### IN-S01: Chrome.DemoPage is dead (orchestrator item d)

**File:** `internal/api/handlers/chrome.go:142-150`, `internal/api/handlers/demo_page.go:140`
**Issue:** Set once and read nowhere: no template and no Go code reads `.Chrome.DemoPage` since plan 05-17 moved the form guard to `.Demo`. The JavaScript flag `window.PinalertDemoPage` comes from `data-demo` and is unrelated. Scenario: a later template author guards on `.Chrome.DemoPage` believing it is the live flag, while the form reads `.Demo`, and the two drift.
**Fix:** remove the field, the assignment and the `Chrome` comment, and update the history comment in `web/report_form_contract_test.go:175`.
**Test:** the build proves nothing reads it; a grep test over `web/templates` for `Chrome.DemoPage` returning no match keeps it gone.

### IN-S02: The push hook's comment contradicts D-16

**File:** `internal/official/pushenqueue.go:15-16`
**Issue:** It says a state level Severe alert "is on the Nearby banner". D-16 says it never raises the banner, and `pickBanner` (`view.go:579`, rank 0 only) is correct. Scenario: a maintainer "fixes" the banner to match the comment and the Andhra Pradesh lightning alerts put the banner up almost every day.
**Fix:** "it is listed under Across <state> on the alerts page only".
**Test:** none needed for the comment; the existing banner case for a state only Severe alert already pins the code.

### IN-S03: The CAP fetch goroutines have no recover

**File:** `internal/official/sachet.go:304-307`
**Issue:** The recover in `RefreshAll` runs in the feed goroutine and cannot catch a panic inside the `FetchCAPs` workers. Scenario: a parser bug on one hostile CAP message panics in a worker and ends the whole server process. GDACS has `searchSafely` for this reason.
**Fix:** a deferred recover in the worker that sets `results[i].Err` to a network `fetchError`.
**Test:** a client test with a `fetchCAP` seam that panics for one item, asserting `FetchCAPs` returns a network error for that item and results for the others.

### IN-S04: After a leader panic, single flight waiters get a zero result with a nil error

**File:** `internal/official/refresher.go:269-276`
**Issue:** On a panic in `run`, the deferred cleanup closes `done` with `c.res` and `c.err` still zero. Scenario: the ticker's run panics while the cron call waits on it; the cron summary reports that feed as a clean, not skipped run with no error.
**Fix:** in a deferred recover, set `c.err` to an error before `close(c.done)`, then re-panic.
**Test:** a fetcher that blocks until a second caller waits, then panics; assert the waiter gets a non nil error.

### IN-S05: GDACS rows are hidden whenever SACHET is unavailable

**File:** `internal/official/view.go:236-238`
**Issue:** `BuildView` returns before the hazard step when the SACHET state is unavailable, even if GDACS is fresh. Scenario: SACHET is down for 7 hours during a cyclone; the GDACS cyclone row for the reader's district, checked 5 minutes ago, is not listed. The direction is safe (nothing stale is shown), but a fresh row is lost.
**Fix:** build the reader and home hazard groups before that return, or document the rule.
**Test:** a `view_test.go` case with SACHET unavailable, GDACS fresh and one hazard in the reader's district.

### IN-S06: The series walk deduplicates on (id, depth), not on id

**File:** `internal/store/queries/official_push.sql:278-290`
**Issue:** `UNION` over `(id, depth)` keeps walking a references cycle until depth 20. Scenario: two messages that name each other make the walk repeat both rows 21 times before the final `DISTINCT`. It is bounded, so this is waste, not a hang.
**Fix:** recurse on `id` alone (`walk(id)` with `UNION`), which ends a cycle at once, and keep a depth guard only if wanted.
**Test:** a database test with a two message cycle asserting two members, one with `names_member` false or the earliest picked as root.

### IN-S07: Two state title functions disagree

**File:** `internal/official/view.go:706-717` versus `internal/service/pushcopy_official.go:193-205`
**Issue:** The view's `titleName` keeps commas inside words. Scenario: a reader in state code 38 sees "Across Dadra,nagar Haveli,daman & Diu", while the lock screen says "Dadra Nagar Haveli Daman and Diu".
**Fix:** one function (or the display table of WR-S07) used by both.
**Test:** `titleName` on the boundary name of state code 38 equals the lock screen words.

### IN-S08: Demo rows name real issuers

**File:** `internal/demo/seed.go:291`, `:306`, `:321`
**Issue:** Sample rows show "Issued by CWC", "IMD Amaravati" and "Andhra Pradesh SDMA", while their details line says "Made up for the Demo. Not from any issuer." This is plan 05-06's choice and every row has the Demo tag. Scenario: a cropped screenshot of a Demo row is forwarded as a real CWC warning.
**Fix (optional):** "Sample issuer" or "Demo (styled as IMD)".
**Test:** a `SampleView` assertion that no row's `Issuer` equals a name from the closed issuer table.

### IN-S09: GDACS's answer to an empty search is unverified

**File:** `internal/official/gdacs.go:412-425`
**Issue:** Unverified: neither fixture nor `live_test.go` shows what GDACS sends for a search with no matches. Scenario: if it is a 404 or a body without `features`, a quiet 14 day window for request A fails the whole GDACS run, and pins go to unavailable while GDACS is up.
**Fix:** capture one live empty answer; if it is a 404 or a message, treat it as an empty list for that request.
**Test:** a fake upstream that answers request A with that captured body, asserting the run succeeds with request B's hazards stored.

### IN-S10: Official tables have no retention and cap_identifier has no index

**File:** `internal/store/migrations/00012_add_official_alerts.sql`
**Issue:** `official_alerts`, `official_alert_areas` and `official_hazards` grow for ever (about 100 alert rows a day), and the reference checks scan the whole table with `string_to_array` for each stored version. Scenario: after a year, every stored version scans about 36,000 rows twice inside its transaction. Speed is out of scope here, but the growth is unbounded.
**Fix (later):** prune rows past expiry plus a week, and index `cap_identifier`.
**Test:** a prune query test that keeps in force rows and removes expired ones.

### IN-S11: With VAPID off, official push rows are queued and never cleared

**File:** `cmd/server/main.go:488-504`, `internal/official/pushenqueue.go:114-147`
**Issue:** The hook queues whenever an account has a stored subscription, but only the worker (started only when push is on) skips stale rows, and `PruneOfficialPushOutbox` never deletes queued rows. Scenario: VAPID keys are removed for a while; every Severe alert adds rows that stay queued until push comes back, then are skipped as stale.
**Fix:** skip the hook when push is off, or run `SkipStaleOfficialPushes` from the refresher too.
**Test:** a service test with push off asserting no queued rows remain after a refresh plus one sweep.

## Checked and clean

- **Session skip set** (`internal/api/router.go:914-967`): exact GET and POST sets plus the `/demo` and `/static` prefixes, refusing an encoded path, `//`, dot segments, other methods and other case. The gate (`gate.go:79-109`) fails closed when no session is in context, so a path trick can only avoid writing a sessions row, never reach a gated route. `/api/official/alerts` and `/api/official/pins` are inside the gated group.
- **Cron door** (`official_refresh.go`): 404 for an unset token and for every method but POST, the per address limiter only on a configured POST, sha256 of both sides with `subtle.ConstantTimeCompare`, the token never logged, and `SetWriteDeadline` called only after the token check. Chi's compress writer has `Unwrap`, so the response controller reaches the connection. The workflow uses `permissions: {}`, a secret env and no checkout.
- **SSRF and bodies:** SACHET and GDACS call constant hosts only, redirects are refused (`ErrUseLastResponse`), the CAP address is built from a guid that passed `^[0-9]{10,20}$` through `url.Values`, and bodies are capped (1 MB, 256 KB, 2 MB) with the one extra byte check. Errors carry fixed words, never a URL.
- **GDACS links:** `SafeGdacsURL` requires the `https://www.gdacs.org/` prefix and refuses spaces, quotes, angle brackets, backslash and non ASCII; it is applied at parse, at read and again in the pins response.
- **Feed text:** `CleanText` drops control and bidirectional characters and caps lengths; text is rendered with textContent (stated in the response docs).
- **D-16** is applied three times and agrees: `qualifies` in the hook, `pickBanner` in the view (district Severe or Extreme, state Extreme only), and worker step 2.
- **Push lane:** the official cap counts `official_push_outbox` only, and 3.3's `CountAccountPushesInWindow` reads `push_outbox` only, so neither counts the other. Every mark presents the held lease and reports rows changed; the lease is renewed before work; writes after sending use a `WithoutCancel` context. Topic is 22 base64url characters from the series root guid, and the Apple rule reads `OfficialPushSentForSeries` over the walked series ids. At least once delivery is as documented.
- **Logs:** refresher, hook, worker and door log ids, counts, a status, a host and a closed class word only; no endpoint, key, payload, area name or alert text.
- **Concurrency:** `RefreshAll` writes each result to its own index and waits; per feed recover; `FetchHazards` joins both requests, keeps the first error and cancels the other; single flight closes `done` after the result is set; `kickSet` is mutex guarded; the go version is 1.26, so loop variables are per iteration. The race detector is clean on the short tests.
- **Claim:** `MarkFeedAttempt` claims with a conditional upsert against the cutoff, so two instances cannot both run inside the gap.
- **Account deletion** removes `official_push_outbox` rows inside the deletion transaction (`accountdelete.go:163-168`).
- **Migrations 00012 and 00013:** Down drops in dependency order (hazards, areas, alerts, status; then outbox and the column), and the CHECK lists match the Go closed sets.
- **Area joins:** `OfficialAlertsInForce` uses `EXISTS` for the filter and `array_agg(DISTINCT ...)` for the codes, so one alert is one row.
- **Demo writes nothing:** each world runs the real services over `memQuerier` with no screener and no notifier, synthetic ids and session ids that cannot collide with real ones, 128 bit visit ids checked by shape before lookup, the Exit address from a two value allowlist, and `no-store` on every answer.
- **Globe tiles:** 85 exact paths, a constant NASA host, refused redirects, a 1 MB cap, a JPEG content type check, single flight per tile and a 60 second failure memory, so memory is bounded by 85 tiles.

---

_Reviewed: 2026-10-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
