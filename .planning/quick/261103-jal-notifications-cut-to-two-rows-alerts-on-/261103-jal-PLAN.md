---
phase: quick-261103-jal
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/service/pushdispatch.go
  - internal/service/pushworker.go
  - internal/service/pushworker_official.go
  - internal/service/updatesalerts.go
  - internal/service/updates.go
  - internal/service/pushkinds.go
  - internal/service/alertsettings.go
  - internal/store/queries/push.sql
  - internal/store/queries/official_push.sql
  - internal/store/queries/updates.sql
  - internal/store/sqlc/push.sql.go
  - internal/store/sqlc/official_push.sql.go
  - internal/store/sqlc/updates.sql.go
  - internal/api/handlers/me.go
  - internal/api/handlers/settings.go
  - internal/api/handlers/page.go
  - internal/api/handlers/low.go
  - internal/api/router.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - web/templates/settings.html.tmpl
  - web/templates/welcome.html.tmpl
  - web/templates/installed_alerts_ask.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/low.html.tmpl
  - web/templates/changes_ask.html.tmpl
  - web/static/js/settings-alerts.js
  - web/static/js/welcome.js
  - web/static/js/installed-ask.js
  - web/static/js/changes-ask.js
  - web/static/js/votes.js
  - web/static/js/report-form.js
  - web/static/css/auth.css
  - web/static/css/places.css
  - internal/service/pushnotif2_db_test.go
  - internal/service/pushdispatch_test.go
  - internal/service/pushdispatch_db_test.go
  - internal/service/pushworker_test.go
  - internal/service/pushworker_fix_test.go
  - internal/service/pushworker_hidden_db_test.go
  - internal/service/pushworker_repeat_db_test.go
  - internal/service/pushworker_follow_test.go
  - internal/service/followpush_test.go
  - internal/service/pushcomments_test.go
  - internal/service/pushworker_official_test.go
  - internal/service/pushworker_official_db_test.go
  - internal/service/updatesalerts_test.go
  - internal/service/alertsettings_test.go
  - internal/official/pushenqueue_db_test.go
  - internal/store/official_push_test.go
  - internal/api/handlers/push_flow_e2e_test.go
  - internal/api/handlers/reliable_alert_e2e_test.go
  - internal/api/handlers/alerts_settings_e2e_test.go
  - internal/api/handlers/push_settings_e2e_test.go
  - internal/api/handlers/official_settings_e2e_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/handlers/places_e2e_test.go
  - internal/api/handlers/page_test.go
  - internal/api/handlers/welcome_e2e_test.go
  - web/settings_rows_contract_test.go
  - web/settings_alerts_test.mjs
  - web/changes_ask_contract_test.go
  - web/changes_ask_pages_contract_test.go
  - web/changes_ask_test.mjs
  - web/changes_ask_hooks_test.mjs
  - web/demo_contract_test.go
  - web/low_contract_test.go
  - web/low_budget_contract_test.go
  - web/noise_words_contract_test.go
  - web/phone_markup_contract_test.go
  - web/review71_templates_fix_contract_test.go
  - web/report_form_session_test.mjs
  - web/votes_stars_test.mjs
  - web/installed_ask_test.mjs
  - web/installed_ask_contract_test.go
  - web/welcome_alerts_test.mjs
  - web/welcome_contract_test.go
  - docs/PUSH.md
  - docs/OFFICIAL-ALERTS.md
  - docs/TRUST-MODEL.md
  - docs/MODERATION.md
  - README.md
  - .planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md
autonomous: true
requirements:
  - QUICK-261103-jal
must_haves:
  truths:
    - "D1: Settings, Notifications holds exactly two rows: Alerts on this device (the device switch, with the sub line Critical reports and severe official alerts near your home area, and posts by people you follow.) and How far from my home area (the distance select, or an Add a home area first button that opens the Home area window when there is no home area)"
    - "D1: the install, blocked and unsupported variants of the device row keep their own sub lines and their label also reads Alerts on this device"
    - "D2: a device whose switch is on gets Critical reports near the home area within the chosen distance, Severe official alerts for the area, follow notes (still gated by the Notify me bell) and the confirmed update of a Critical or follow note, whatever the stored critical_push and official_push flags say"
    - "D3: no Reliable reporter note, no change note, no comment note and no confirmed update whose base is Reliable is queued or sent to any phone, whatever the stored reliable_alerts and changes_push flags say; a row of those kinds already queued is skipped at send time"
    - "D3: the bell refreshes the dispatcher did on those paths still run, so bell rows arrive as early as before"
    - "D4: every account with a home area gets the Reliable reporter bell row with no switch, with every existing cap, mute, block and the Not confirmed yet wording; only reports posted after the stored enabled_at (when the row has one) or within ReliableBellLookBack (one hour) of the refresh can raise it, so nothing old is back filled"
    - "D4: changes and comments on your reports stay in the bell exactly as today"
    - "D5: the sign up alerts pop-up and the installed iPhone app ask have no Reliable tick box and send no Reliable request; Allow still turns the device on and saves a missing home area; Not now and the titles are unchanged"
    - "D6: the Tell you when this report changes? pop-up is gone: its partial, its script, its includes on Nearby and Low data, its hooks in votes.js and report-form.js, the server flag that decided it, and its tests"
    - "D7: PATCH /api/me/alerts answers only alert_radius_km and has_home_area; a body that still carries reliable_alerts, critical_push, changes_push or official_push is accepted and those keys are ignored; the generated API reference documents only alert_radius_km"
    - "D8: no migration; the four columns stay, unused for delivery, with a dated comment at the queries and the settings struct"
    - "D9: the 4 notes per hour limit, the Critical never held but counted rule, the Apple first note rule and the Mac Dock app no ask rule are unchanged and still pinned by tests"
    - "D10: docs/PUSH.md, docs/OFFICIAL-ALERTS.md, docs/TRUST-MODEL.md, docs/MODERATION.md and README.md describe the two row box and what reaches the phone, and 03.3-CONTEXT.md carries the dated owner amendment at the top of its amendments"
  artifacts:
    - path: "internal/service/pushworker.go"
      provides: "kindSwitchOn answers from the kind alone: critical and follow yes, reliable and changed never, a confirmed update follows its base"
      contains: "func (w *PushWorker) kindSwitchOn("
    - path: "internal/service/updatesalerts.go"
      provides: "refreshReliableAlerts for every account with a home area, start point enabled_at or ReliableBellLookBack"
      contains: "ReliableBellLookBack"
    - path: "internal/service/updates.go"
      provides: "The named look back constant"
      contains: "ReliableBellLookBack = time.Hour"
    - path: "internal/store/queries/official_push.sql"
      provides: "OfficialPushCandidates without the stored official switch predicate"
      contains: "name: OfficialPushCandidates"
    - path: "internal/service/pushnotif2_db_test.go"
      provides: "D9 tests a, b and d on a real database"
      contains: "func Test"
    - path: "internal/api/handlers/me.go"
      provides: "AlertSettingsRequest with the four legacy keys accepted and ignored, AlertSettingsResponse with alert_radius_km and has_home_area only"
      contains: "swaggerignore"
    - path: "web/templates/settings.html.tmpl"
      provides: "The two row Notifications box"
      contains: "Alerts on this device"
  key_links:
    - from: "internal/service/pushworker.go"
      to: "kindSwitchOn"
      via: "step 4 of the send time recheck, home.Muted or not kindSwitchOn(run.kind, claim.FirstKind)"
      pattern: "kindSwitchOn\\(run\\.kind, claim\\.FirstKind\\)"
    - from: "internal/service/pushdispatch.go"
      to: "UpdatesRefresher.Refresh"
      via: "reliableBranch and changesFor keep their refresh loops with no enqueue after them"
      pattern: "d\\.updates\\.Refresh"
    - from: "web/static/js/settings-alerts.js"
      to: "/api/me/alerts"
      via: "the distance select saves alert_radius_km at once"
      pattern: "alert_radius_km"
---

<objective>
Owner, 2026-11-03, while testing Settings on a test copy: "I feel like the notification part is like a little bit complicated. Why do we have like on this device and the other things, when you could like keep it a little simpler and just have things that are like really important rather than having everything on there?" Asked to choose, the owner delegated: "decide anything about the notifications things, what are we going to do about it? ... what will be a best options". The decisions D1 to D10 below were made on that delegation and are locked.

Purpose: one device switch and one distance in Settings; the phone carries only what is really important (Critical reports near home, severe official alerts, follow notes); everything else lives in the bell; the sign up and installed app asks lose the Reliable tick box; the per report changes pop-up is removed. This reverses parts of 03.3 Q1 (2026-10-16, four phone kinds each with a switch) and the 2026-10-27 pop-up shape.

Output: delivery rules in the service layer (no migration), the two row Settings box, the narrowed alerts API with regenerated swagger, the simpler pop-ups, the changes pop-up removed with its tests, docs and the 03.3 amendment, and the full suite green on the scratch database q_notif2_a.
</objective>

## Why one plan with four ordered tasks

The orchestrator asked for one quick plan. The four tasks run in order in one executor and each ends with a green build and a path limited commit. Task 1 changes who gets what (service and SQL, the biggest test churn). Task 2 narrows the settings struct, the API and the Settings box, and needs Task 1 first because the bell rule must stop reading the flags before the API stops writing them. Task 3 removes the pop-up pieces; it shares internal/api/router.go and internal/api/handlers/page.go concerns with Task 2, so it runs after it. Task 4 is docs and the full gate. About 85 files are listed; most are tests. If the executor's context runs low, finish the current task, commit it, and stop with a SUMMARY note naming the tasks left; never leave a task half committed.

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md
@docs/PUSH.md
@internal/service/pushdispatch.go
@internal/service/pushworker.go
@internal/service/updatesalerts.go
@internal/service/alertsettings.go
@web/templates/settings.html.tmpl
@web/static/js/settings-alerts.js

## Locked decisions (from the orchestrator, plan exactly these)

- D1 Settings, Notifications has two rows only. Row 1 label Alerts on this device (was On this device): the device switch, with one sub line under the label: "Critical reports and severe official alerts near your home area, and posts by people you follow." Its other variants keep their existing sub lines (needs install: "Add Pinalert to your Home Screen first"; blocked: "Blocked in your browser settings. Allow notifications for Pinalert there, then come back."; unsupported: "This browser can't show notifications. Updates still show in the bell.") and their label also becomes Alerts on this device. Row 2 How far from my home area, the distance select as today; with no home area it is the existing Add a home area first button pattern that opens the Home area window. Removed rows: Critical reports near my home area, Severe official alerts for my area, Changes and comments on reports I posted, voted on or commented on, A Reliable reporter posts near me, with their no home area variants, the JS that drove them and their CSS if it becomes unused.
- D2 What reaches the phone (a device whose switch is on): Critical reports near the home area within the distance; Severe official alerts for the area; follow notes (unchanged, still gated by the Notify me bell on each follow); confirmed updates whose base kind is one of those. The stored flags critical_push and official_push are IGNORED: both count as on for every subscribed device. The device switch is the only way to stop them on a device.
- D3 What never reaches the phone: Reliable reporter notes (PushKindReliable), changes or comments on reports you posted, voted on or commented on (PushKindChanged, comment notes) and confirmed updates whose base is Reliable. The stored flags reliable_alerts and changes_push are IGNORED for phone delivery. Rows of these kinds already queued must not be sent (kindSwitchOn in pushworker.go returns false for them). Stop enqueueing them at all (EnqueueReliableFromUpdates and the changes and comment paths in pushdispatch.go), but keep any bell refresh those paths do.
- D4 The bell. Changes and comments on your reports stay in the bell exactly as today. Reliable reporter posts near the home area show in the bell for EVERY account with a home area, with no switch. Keep every existing cap, mute, block and the Not confirmed yet wording. It must never back fill a pile of old reports when this ships; the start point is stated below (C1).
- D5 Sign up alerts pop-up (#welcome-alerts-ask, onAskAllow and saveReliable in welcome.js): remove the Reliable tick box and the Reliable save. Allow still turns this device on and saves a missing home area; Not now unchanged; title unchanged. The installed iPhone ask (installed_alerts_ask partial, installed-ask.js) has the same tick box and save; remove them the same way.
- D6 Remove the Tell you when this report changes? pop-up entirely: its partial, its script, its includes and hooks in pages, its tests, and any server side flag or route that only existed for it. No migration.
- D7 PATCH /api/me/alerts: the four boolean keys leave the response and the documented request; alert_radius_km stays. A request that still carries the removed keys (an old cached page) is accepted and the keys are ignored. Regenerate swagger with make swag and keep the router's swag license line exactly "// @license.name Unlicensed (all rights reserved)".
- D8 No migration. The columns stay, unused for delivery, with a short dated comment where the queries and the settings struct are.
- D9 Tests: flip every test that pins the removed rows, tick box, pop-up or keys; add (a) stored reliable_alerts true and changes_push true give NO phone note of those kinds, at enqueue and at send time; (b) stored critical_push false and official_push false with a subscribed device still give the Critical and official phone notes; (c) an account with a home area and reliable_alerts false gets the Reliable bell row; (d) follow notes still reach the phone; (e) the Settings page renders exactly the two rows with the new label and sub line; (f) the sign up pop-up has no Reliable tick box. Keep the 4 notes per hour limit, the Critical rule for it, the Apple first note rule and the Mac Dock app no ask rule (07.1 D-18) unchanged.
- D10 Docs: docs/PUSH.md, docs/OFFICIAL-ALERTS.md and any doc or README text that lists the switches or the removed pop-up; a dated section "## Owner amendment, 2026-11-03 (two row notifications box, quick 261103-jal)" at the top of the amendments in 03.3-CONTEXT.md. Phase 3.2's Reliable filter on Nearby stays untouched.

## Claude's discretion choices made by the planner (record each in the SUMMARY)

- C1 (D4 start point). In refreshReliableAlerts, since is the stored enabled_at when the account_alert_settings row has one (enabled_at is set only while the old Reliable switch is on, so this keeps today's behaviour exactly for people who had switched it on), else now minus ReliableBellLookBack, a new named constant of one hour in internal/service/updates.go next to ReliableAlertsPerReporterPerHour. A report created before since never raises the row. No new query and no migration: the fake clock in updatesalerts_test.go drives it, and on the day this ships an account that never switched the alert on hears only about Reliable reports posted in the last hour, under the existing per reporter cap and hourly ceiling.
- C2 (D3, Reliable branch). reliableBranch in pushdispatch.go stays as a bell refresh only: the reporter tag check, the reporter caps, the capped and fanned marks (MarkReliableFanout) and the Refresh loop stay, so the 30 second sweep does not repeat the history read and the refreshes for a fanned report; only the EnqueueReliableFromUpdates call goes. Its recipient filter no longer reads the stored flags: every subscribed neighbour in range for a report that is not serious, none for a serious one (those neighbours already got the Critical push, as an account with Critical on did before). Their bell row then arrives at once, as it did for opted in accounts.
- C3 (D3, changes paths). changesBranch, ChangesSweepOnce and changesFor keep their Refresh calls and no longer read GetPushKindSettings or call EnqueueChangedFromUpdates. The accounts they refresh are still picked by FollowersWithChangesPush and AccountsWithChangesPush, which read the stored changes_push_on flag. That flag can no longer change after this task, so the set is frozen; it now only decides whose bell the dispatcher refreshes early. The bell poll (GET /api/updates) stays the producer for everyone else, exactly as today. Say this in a comment at both queries.
- C4 (D7 legacy keys). AlertSettingsRequest keeps the four keys as *bool fields tagged swaggerignore:"true", decoded and then ignored, because the handler uses DisallowUnknownFields and dropping the fields would turn an old cached page into a 400. A body that names only legacy keys answers 200 with the stored settings and changes nothing. An empty object {} still answers 400 field body with "Nothing to save.". Any other unknown key still answers 400 field body.
- C5 (D6 and the Policy). The Policy line in web/templates/legal_body.html.tmpl that names the list of accounts asked whether to be told when a report changes stays as it is. Editing the Policy text requires moving policyLastUpdated and service.CurrentTermsVersion, which makes every account agree to the terms again, and that is not part of this decision. The list stays on devices that were asked before. TestPolicyNamesOlderStorageKeys keeps the line and marks that key retired (no script check for it) with a dated comment. This is flagged to the owner in the SUMMARY.
- C6 (D8, SQL kept). No query is deleted: EnqueueReliableFromUpdates, EnqueueChangedFromUpdates, GetPushKindSettings, UpsertPushKindSettings, GetOfficialPushSetting and UpsertOfficialPushSetting stay in SQL and in sqlc output with their store tests, and tests use them to seed values stored before this change. Production code stops calling the ones this plan names. Querier interfaces may keep their method lists; only remove a method from an interface when nothing in that package calls it and its test fakes still compile.
- C7 (D3, worker paths kept). The worker's Reliable and changed send paths (personNotMutedOrBlocked, eventStillHolds, reportWasConfirmed, the comment recheck) stay in place behind kindSwitchOn, as a defensive layer for rows queued before this change. BuildPushText keeps its Reliable and changed words. Removing them is a separate cleanup and is not done here.
- C8 (official skip reason). The worker stops reading the stored official switch, so the switched_off skip reason is no longer produced. Its constant stays with a comment, because the official_push_outbox CHECK constraint lists it (D8) and rows skipped before this change carry it.
- C9 (D1 radius row). The distance row gets a classless wrapper div like the device row, so the box's hairline rule (.place-box > * + *) draws one line per row, with two variants inside: the select row (hidden without a home area) and a new button row with id settings-radius-nohome (hidden with a home area) whose label is How far from my home area, whose sub line is Add a home area first, with the chevron and aria-haspopup dialog, opening the Home area window through window.PinalertSettings.openWindow('home') the way the removed no home rows did. Focus returns to the select when a home area now exists, else to that button.
- C10 (AlertSettings struct). service.AlertSettings keeps AlertRadiusKm and HasHomeArea only; ReliableAlertsOn, EnabledAt, CriticalPushOn, ChangesPushOn and OfficialPushOn leave it. service.AlertSettingsUpdate keeps AlertRadiusKm only. UpdateAlertSettings reads the stored row and passes its stored reliable_alerts_on through UpsertAlertSettings unchanged, so enabled_at (the C1 start point) is never moved by a distance change.

## Facts the planner already established (do not re-derive)

- The bell poll is the producer: GET /api/updates (internal/api/handlers/updates.go GetUpdates) calls UpdatesService.Refresh before it lists. The dispatcher's Refresh calls only make rows exist sooner. cmd/server/main.go wraps the refresher (lateUpdates).
- refreshReliableAlerts (internal/service/updatesalerts.go) today returns nil on pgx.ErrNoRows from GetAlertSettings and when ReliableAlertsOn is false or EnabledAt is not valid. Both early returns must go: no row means DefaultAlertRadiusKm and the look back. It already uses DefaultAlertRadiusKm when the stored radius is not valid. The fake in updatesalerts_test.go answers ErrNoRows when settings is nil; alertOn(radius) stamps EnabledAt three hours before updatesNow; candidates in the existing tests are 10 to 50 minutes old, and one is 4 hours old (before the switch).
- UpsertAlertSettings (internal/store/queries/updates.sql) sets enabled_at to now when reliable_alerts_on goes from off to on, keeps it while on, and sets NULL when off. DisableReliableAlerts is called from UpdateProfile's clear path and stays.
- criticalBranch filters recipients with filterRecipients(..., func(r) bool { return r.CriticalPushOn }); a nil want accepts every row (internal/service/pushrecipients.go).
- reliableBranch's filter today is r.ReliableAlertsOn && !(serious && r.CriticalPushOn). PushHomeCandidates selects accounts with a home cell in the list, a live subscription and no mute on the report.
- changesFor reads GetPushKindSettings for changes_enabled_at, then calls EnqueueChangedFromUpdates. PushDispatchQuerier lists GetPushKindSettings, EnqueueReliableFromUpdates and EnqueueChangedFromUpdates; the dispatcher calls each only from the code this plan removes.
- kindSwitchOn(kind, first, h PushAccountHomeRow) is called once, at pushworker.go around line 528, as home.Muted || !w.kindSwitchOn(run.kind, claim.FirstKind, home). Nothing in tests calls it directly. Step 3 (stillHolds and the comment recheck) runs before it, so a changed row still runs eventStillHolds before it is skipped.
- EnqueueConfirmedUpdates still selects sent rows of kind critical, reliable or follow. A confirmed update for an old sent Reliable note is therefore still queued, and kindSwitchOn skips it at send time. No SQL change is needed for D3.
- Official lane: OfficialPushCandidates (internal/store/queries/official_push.sql) filters AND COALESCE(s.official_push_on, true) through a LEFT JOIN on account_alert_settings; internal/official/pushenqueue.go calls it. The official worker (internal/service/pushworker_official.go) step 5 reads GetOfficialPushSetting and skips with officialSkipSwitchedOff ("switched_off"); step 6 reads GetAccountHomeCell and already skips a gone account as not eligible, so dropping step 5 loses nothing. The skip reason set is pinned at pushworker_official_test.go line 46 and in the CHECK listed by internal/store/migrations_test.go around line 1454.
- make sqlc runs sqlc generate (sqlc v1.31.1 matches the generated headers). make swag runs swag init -g internal/api/router.go -o docs (swag v1.16.4 matches go.mod). Query comments become Go doc comments in internal/store/sqlc, so any SQL comment edit needs make sqlc. After make sqlc, git diff --stat internal/store/sqlc/ must name only push.sql.go, official_push.sql.go and updates.sql.go.
- PATCH /api/me/alerts (internal/api/handlers/me.go UpdateAlertSettings) decodes with DisallowUnknownFields. There is no GET route for the alert settings: the GET side is the Settings page view model (internal/api/handlers/settings.go). The router comment above the PATCH route is in internal/api/router.go around line 1152.
- swagger_test.go around lines 928 to 931 pins the field lists of handlers.AlertSettingsRequest and handlers.AlertSettingsResponse. web/design_rules_contract_test.go TestGeneratedAPIReferenceUsesPlainPunctuation fails on a long dash in docs/docs.go, docs/swagger.json or docs/swagger.yaml.
- The changes pop-up is drawn only when pageViewModel.ChangesAsk (page.go) or the Low data view model's ChangesAsk (low.go) is true, decided by changesAskFor in page.go, which is the only reader of PageConfig.AlertSettings (type AlertSettingsLookup). router.go around line 786 sets page.AlertSettings = accounts. index.html.tmpl around lines 247 and 248 and low.html.tmpl around lines 113 and 114 include the partial and its script inside one if block.
- votes.js: offerChangesAsk is called after a saved confirm or dispute (around line 650) and defined around line 660; offerRateBox (around line 2330) waits on the pop-up's settled promise before mounting the star box, guarded by rateGeneration. report-form.js: offerChangesAsk is called twice after an accepted post (around lines 1442 and 1453) and defined around line 1485. Each is guarded by a check that the pop-up object exists, so removing the script alone would leave dead hooks.
- The Reliable tick box: welcome.html.tmpl around line 219 (label class welcome-ask__check, input id welcome-alerts-reliable); welcome.js reads it as askReliable (around line 224), disables it in setAskDisabled (around line 1286), builds saveReliable (around line 1566) and chains it in onAskAllow (around lines 1606 and 1620); ALERTS_URL (line 102) is used only by saveReliable. installed_alerts_ask.html.tmpl line 15 has the same label with input id installed-alerts-ask-reliable; installed-ask.js reads it in start and setBusy, and saveReliable plus ALERTS_URL, REQUEST_TIMEOUT_MS and the fetchFn option exist only for it. auth.css around lines 708 to 731 styles the label for both pop-ups (the rule and its comment go); the #installed-alerts-ask-title margin rule stays.
- The Policy (web/templates/legal_body.html.tmpl line 190) names the changes asked list; TestPolicyNamesOlderStorageKeys (web/review71_templates_fix_contract_test.go around line 286) requires the named script to still use pinalert.changesAsked. policyLastUpdated (internal/api/handlers/legal.go) and service.CurrentTermsVersion (internal/service/profile.go) are both 2026-10-29 and must not move.
- The device guide copy "On this device, alerts work once Pinalert is on your Home Screen." is the install guide lead, not the Settings label; it stays, and web/noise_words_contract_test.go and internal/api/handlers/welcome_e2e_test.go pin it.
- The Low data byte budget (web/low_budget_contract_test.go) fails only when the page grows past the budget; removing code needs no re-measure. Its dated comments about the changes script are history; add one dated line saying the script was removed by this quick task and leave the numbers.
- Doc tests: TestTrustModelDocMatchesCode (internal/service/trust_model_doc_test.go) checks the TRUST-MODEL headings list, the constants table, backticked identifiers and a list of test names the document must cite, which includes TestFollowPushOrderCriticalFollowReliable, TestFollowCleanupOnEveryEndPath and TestMuteNeverHidesReports. Keep the names of cited tests even when their bodies change. web/review71_templates_fix_contract_test.go TestPushDocHomeAreaSearchIsFallback pins the PUSH.md sentence "A home area is always set by then, because step 4 of sign up requires one; the pop-up keeps its own home area search only as a fallback." and TestPushDocStepZeroNamesEveryPhase71Migration pins owner step 0. web/round2_docs_contract_test.go and web/round3_docs_contract_test.go pin other doc lines.
- Test conventions: an existing test that changes gets a short comment line "Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box): ..." above the change, the files' own convention. The push DB rig is newPushDBRig in internal/service/pushdispatch_db_test.go (package service): r.reader(prefix, cellK, radius, critical, reliable, changes) seeds an account with a session, a home cell, a device and, when radius is not 0, a settings row through r.setSettings, which writes the stored flags by SQL (enabled_at base minus 3 hours when reliable); r.report, r.vote, r.eval, r.outbox, r.count, r.progress; r.updates is the real UpdatesService on the rig clock; r.cur moves the clock; testutil.SeedReliableReporter makes a Reliable reporter. Official DB rigs live in internal/official/pushenqueue_db_test.go and internal/service/pushworker_official_db_test.go.

## Executor rules (binding, carried verbatim from the orchestrator)

- Path limited commits, each message ending with a blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; never stage STATE.md, ROADMAP.md or REQUIREMENTS.md; the SUMMARY is committed alone, last.
- Tests only on a fresh scratch database named q_notif2_a (createdb it; never pinalert_test, pinalert, p31_4_uat, p34_uat, p9_view, p3_view, p4_uat). First confirm DATABASE_URL and PGDATABASE are empty, then set DATABASE_URL inline for that database in the same Bash call. Full `go build ./...`, `go vet ./...`, `go test ./... -p 1 -count=1 -v` and `node --test web/*.mjs`; only TestGeocodeLive, TestOfficialLive and TestScreeningFixture may skip. Drop q_notif2_a at the end.
- No Python in any form (not even python3 --version). No images or screenshots in the repo. No em or en dashes in code comments, copy, docs or commit messages.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797, and never touch the processes on them.
- This must print nothing: `git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder`.
- Never push. Never rewrite history.

## How to run things (planner notes on the rules above)

- The one database command form, every time, in one Bash call: test -z "$DATABASE_URL" && test -z "$PGDATABASE" && DATABASE_URL='postgres://localhost:5432/q_notif2_a?sslmode=disable' go test ... . The sandbox refuses env -i. Create the database once at the start of Task 1 (if psql -lqt | cut -d'|' -f1 | grep -qw q_notif2_a finds a leftover of this task, dropdb it first, then createdb q_notif2_a). Drop it at the end of Task 4.
- Logs go to .scratch-jal/ at the repository root, never staged, deleted at the end of Task 4.
- Commits: git rm -- for deleted files, git add -- for new and changed files, then git commit -m "subject" -m "body" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- followed by every path of that task. Another session's STATE.md edit and several untracked folders are already in the tree; leave them alone.
- Use the Edit tool for edits and the Write tool for new files. Check every touched file for long dashes with grep -n -e $'\xe2\x80\x94' -e $'\xe2\x80\x93' <files>, never by pasting the characters.
- gofmt -l lists unrelated files already; run gofmt -w only on Go files this plan touches.
- Keep comment density and naming like the surrounding code; every new or changed doc comment names quick 261103-jal and the owner decision of 2026-11-03.
- Fix code, never weaken or skip a test. A test changes only where it pins a removed row, tick box, pop-up, key or delivery rule, or uses a Reliable or changed note as the vehicle for a rule this plan keeps; in the second case re-express it with a follow note or with a Critical note plus its confirmed update, never delete it.
- No server is started. No migration (the next free number would be 00025; needing one is a stop and report).
- Every phone notification stays silent; no wording promises sound or vibration.
- Do not touch Phase 3.2's Reliable filter on Nearby (Show menu, kept_by_reliable_filter), the device guide copy, the Policy text, policyLastUpdated or CurrentTermsVersion.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Delivery rules in the service layer (D2, D3, D4, D8)</name>
  <files>internal/service/pushdispatch.go, internal/service/pushworker.go, internal/service/pushworker_official.go, internal/service/updatesalerts.go, internal/service/updates.go, internal/service/pushkinds.go, internal/store/queries/push.sql, internal/store/queries/official_push.sql, internal/store/queries/updates.sql, internal/store/sqlc/push.sql.go, internal/store/sqlc/official_push.sql.go, internal/store/sqlc/updates.sql.go, internal/service/pushnotif2_db_test.go, internal/service/pushdispatch_test.go, internal/service/pushdispatch_db_test.go, internal/service/pushworker_test.go, internal/service/pushworker_fix_test.go, internal/service/pushworker_hidden_db_test.go, internal/service/pushworker_repeat_db_test.go, internal/service/pushworker_follow_test.go, internal/service/followpush_test.go, internal/service/pushcomments_test.go, internal/service/pushworker_official_test.go, internal/service/pushworker_official_db_test.go, internal/service/updatesalerts_test.go, internal/official/pushenqueue_db_test.go, internal/store/official_push_test.go, internal/api/handlers/push_flow_e2e_test.go, internal/api/handlers/reliable_alert_e2e_test.go</files>
  <behavior>
    - (a) A reader with a home area, a device and stored reliable_alerts_on true and changes_push_on true (seeded by SQL): a non serious report by a Reliable reporter near the home area, evaluated with an actor and swept, queues no reliable row for the reader, and a vote that changes a report the reader posted, followed by ChangesSweepOnce, queues no changed row; the reader's bell still holds the reliable_nearby row (the Refresh still ran). Then rows of kind reliable, changed and confirmed_update (whose first row is a sent reliable row) inserted as queued by SQL are all skipped as not_eligible by one worker pass, and the fake sender records no send.
    - (b) A reader with a device and stored critical_push_on false and official_push_on false: a Critical report near the home area queues and sends one critical note; a Severe official alert for the home district queues one official row (OfficialPushCandidates includes the reader) and the official worker sends it.
    - (c) refreshReliableAlerts writes the reliable_nearby row for an account with a home area and no settings row, for one with reliable_alerts_on false, and for one with the switch on but no stamp; it writes nothing for an account with no home cell. With no stamp, a Reliable report posted more than ReliableBellLookBack before now gets no row and one posted inside it does (no back fill). With a stamp, a report posted before the stamp gets no row (today's rule). Every existing exclusion still holds (own report, followed, muted report, muted person, follow noted, not Reliable, expired, per reporter cap, hourly ceiling).
    - (d) A follower with the Notify me bell on, stored critical, changes, reliable and official flags all false, and a device: the followed person's report queues one follow row and the worker sends it.
    - ReliableBellLookBack is exactly time.Hour, pinned by a test.
    - The worker's 4 per hour cap still holds back the next non Critical note after four sent notes, Critical is still never held but counted, and an Apple endpoint still gets only the first push about a report: the tests that used Reliable or changed notes for this now use follow notes or a Critical note and its confirmed update.
    - The Reliable branch with stored flags ignored refreshes every subscribed neighbour in range for a non serious Reliable report, none for a serious one, marks the report fanned or capped exactly as before, and queues nothing.
  </behavior>
  <action>
Step A, scratch database and logs. Confirm both variables are empty, drop a leftover q_notif2_a only if it exists, createdb q_notif2_a, mkdir -p .scratch-jal.

Step B, RED. Write internal/service/pushnotif2_db_test.go (package service, header comment naming quick 261103-jal and the owner decision of 2026-11-03, D2, D3, D9) with TestStoredReliableAndChangesFlagsNeverReachThePhone (a), TestStoredCriticalAndOfficialOffStillReachThePhone (b, the Critical part; the official part goes in Step D) and TestFollowNoteReachesThePhoneWithEveryFlagOff (d), built on newPushDBRig and its helpers, with the stored flags written by SQL (r.setSettings, plus an UPDATE for official_push_on). For the send side use the worker test rig the other worker DB tests use (pushworker_repeat_db_test.go or pushworker_hidden_db_test.go show how a worker with a fake sender is built on the same pool). In updatesalerts_test.go add TestReliableBellLookBackIsNamed and the look back cases of (c) (use alertView ages either side of one hour). Run them with the database form to .scratch-jal/red1.log; they must fail, never skip.

Step C, GREEN, production code.
1. internal/service/updates.go: add ReliableBellLookBack = time.Hour next to ReliableAlertsPerReporterPerHour, with a doc comment: how far back the Reliable reporter bell row looks for an account whose stored row has no enabled_at (owner 2026-11-03, quick 261103-jal, D4), so the bell never fills with old reports when the switch goes away.
2. internal/service/updatesalerts.go refreshReliableAlerts per D4 and C1: no early return on ErrNoRows (defaults: DefaultAlertRadiusKm and the look back), no reliable_alerts_on gate, since is enabled_at when valid else now minus ReliableBellLookBack, and the CreatedAt check compares against since. Keep every other step, the home cell read, the Nearby read, the person mute read, the follow noted skip, the oldest first order, the per reporter cap and InsertReliableAlert exactly as they are. Rewrite the file header and the function's doc comment: the update is for every signed in reader with a home area, with no switch; step 1 reads the distance and the stamp when a row exists; the TRUSTX-03 paragraph now says the rule no longer reaches the phone at all (removed 2026-11-03), and that widening the bell to every account with a home area is softened by the same cap, ceiling and Not confirmed yet wording.
3. internal/service/pushdispatch.go: criticalBranch passes nil as the want filter (D2). reliableBranch per C2: the filter becomes the not serious test, the EnqueueReliableFromUpdates call goes, the caps, the marks and the Refresh loop stay. changesFor per C3: keep the Refresh, drop the GetPushKindSettings read and the EnqueueChangedFromUpdates call; changesBranch and ChangesSweepOnce keep their loops. Remove EnqueueReliableFromUpdates, EnqueueChangedFromUpdates and GetPushKindSettings from PushDispatchQuerier. Rewrite the file header's Kinds list and the one first alert paragraph, and the doc comments of EvaluateReport, reliableBranch, changesBranch, ChangesSweepOnce and changesFor, so they say what reaches the phone now and that these paths refresh bells only (D3).
4. internal/service/pushworker.go: kindSwitchOn takes (kind PushKind, first *string) and answers from the kind alone: critical and follow true, reliable and changed false, a confirmed update answers for its base kind (unknown base false, as today). Update the call site and the step 4 comment (the mute is read again, and whether the kind may reach a phone at all). Doc comment names D2, D3, C7 and says the stored switches are no longer read.
5. internal/service/pushworker_official.go: drop step 5 (the stored switch read and the switched_off skip) and renumber the step comments; keep officialSkipSwitchedOff with a comment per C8; remove GetOfficialPushSetting from the worker's querier interface only if nothing in the file still calls it and the test fake still compiles.
6. internal/service/pushkinds.go: comments only. On PushKindReliable and PushKindChanged say no new row is queued since 2026-11-03 and a queued one is never sent (D3). On DefaultCriticalPushOn, DefaultChangesPushOn and DefaultOfficialPushOn say the stored columns and these defaults are kept for the SQL COALESCE only and no longer decide delivery (D8). No value changes.
7. SQL (D8, D2): in internal/store/queries/official_push.sql remove the stored official switch predicate from OfficialPushCandidates and its now unneeded LEFT JOIN, and rewrite its comment (every account with a home cell, not deleted, with a live device, under the given prefixes). Add one short dated comment (owner decision 2026-11-03, quick 261103-jal: the column is kept and no longer decides delivery) to GetPushKindSettings, PushHomeCandidates, PushAccountHome, FollowersWithChangesPush (plus the C3 sentence), AccountsWithChangesPush (plus the C3 sentence), EnqueueReliableFromUpdates and EnqueueChangedFromUpdates (not called since this change, kept with their store tests, C6) in push.sql; GetOfficialPushSetting and UpsertOfficialPushSetting in official_push.sql; GetAlertSettings in updates.sql (enabled_at is still the start point of the Reliable bell row when set, C1). Then make sqlc and check the diff stat names only the three generated files.

Step D, tests to flip or re-pin, each with the dated Amended line.
- internal/service/updatesalerts_test.go: rewrite TestReliableAlertsSwitchedOff as the (c) table (keep its name or rename it; it is not in the TRUST-MODEL must cite list, check with grep before renaming); keep TestReliableAlerts, TestReliableAlertsCapPerReporter, TestReliableAlertSeedsTheCursor and TestReliableAlertsOnRealHistory green; if TestReliableAlertsSource pins a string that moved, update the string, not the rule.
- internal/service/pushdispatch_test.go: the fake keeps its methods; the call order checks that name EnqueueReliableFromUpdates (around lines 744 and 789) and the ChangesSweepOnce test (around line 1269) now assert that no enqueue of those kinds is called and that the Refresh calls still happen.
- internal/service/pushdispatch_db_test.go: TestPushReliable (no reliable rows for any reader whatever its flags; every subscribed reader in range has the reliable_nearby bell row after the evaluation for the non serious report; the Critical report goes out as critical to all three readers, R1 included; the fanned mark stays); TestPushChangedRaceWithBell (no changed rows; the bell rows stay); TestPushRecipients and TestPushCaps where a stored Critical flag false used to exclude an account.
- internal/service/pushworker_test.go: TestWorkerSkipsWhenNoLongerEligible (stored Critical off now sends; every reliable and changed case is skipped as not_eligible whatever the flags); TestWorkerBuildsWordsAtSendTime and TestWorkerMessageOptions (the reliable and changed cases now assert a skip; the words of those kinds stay pinned by the BuildPushText tests in pushcopy_test.go, which do not change); TestWorkerCapPerAccountHour (re-pin with follow rows), TestWorkerAppleFirstPushOnly (re-pin with a Critical note and its confirmed update, or follow), TestWorkerNoDeviceOrNoHome and TestWorkerReadsVoteLogOnlyForHidden (keep what they pin about reads; the outcome for a changed row is now a skip).
- internal/service/pushworker_fix_test.go: TestQueuedReliablePushStopsAfterMuteOrBlock and TestConfirmedUpdateOfReliablePushStopsAfterMuteOrBlock become: the row is never sent, with or without the mute or block (keep both names, the TRUST-MODEL and PUSH.md cite them); TestCommentPushStillHoldsAfterABlock tests the SQL check and stays as it is.
- internal/service/pushworker_hidden_db_test.go TestWorkerHiddenNeedsEarlierConfirmation: both change notes are now skipped, the bell row stays; TestReportWasConfirmed stays as it is.
- internal/service/pushworker_repeat_db_test.go TestWorkerCriticalNeverHeldButCounted: re-pin the held kind with follow rows.
- internal/service/pushcomments_test.go: comment notes are never sent; the SQL level CommentPushStillHolds checks stay.
- internal/service/followpush_test.go and pushworker_follow_test.go: only where a test expects a reliable row to be queued; TestFollowPushOrderCriticalFollowReliable keeps its name (TRUST-MODEL cites it) and now pins Critical over follow and no reliable row.
- internal/service/pushworker_official_test.go (the switch is off case around line 485) and internal/service/pushworker_official_db_test.go (the switch off case around line 340): the row is now sent. Add the official half of (b) to the DB test with both stored flags false.
- internal/official/pushenqueue_db_test.go TestOfficialPushEnqueue: the switched off account is now queued. internal/store/official_push_test.go: where OfficialPushCandidates is expected to leave out an account with official_push_on false, it now includes it.
- internal/api/handlers/push_flow_e2e_test.go: TestPhoneAlertCapShowsNothingExtra re-pins the held note with a follow note (or the next non Critical kind that still pushes); the changes_push PATCH around line 494 and the cases that expect a change note on the phone now expect none and the bell row; TestPhoneAlertCriticalEndToEnd stays green.
- internal/api/handlers/reliable_alert_e2e_test.go: the cases that switch the alert off and expect no bell row (around lines 389 and 399) now expect the row while a home area exists; add the (c) case over HTTP: an account with a home area that never sent a PATCH gets the Reliable bell row through GET /api/updates; TestReliableAlertOverTheBell and TestReliableAlertAtPostingWhenNotCritical keep their names.
- Then run every package this task touches (internal/service, internal/official, internal/store, internal/api/handlers, internal/api) with the database form; any other failing test that used a Reliable or changed note to pin a kept rule is re-pinned the same way and named in the SUMMARY.

Step E, gofmt -w the touched Go files, go build ./..., go vet ./..., then commit "fix(quick-261103-jal): the phone gets Critical, official and follow notes only, the bell gets Reliable reports for everyone" with a body naming D2, D3, D4, D8, C1, C2, C3, C6 to C8 and the trailer, path limited to this task's files.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && DATABASE_URL='postgres://localhost:5432/q_notif2_a?sslmode=disable' go test -count=1 -p 1 ./internal/service/ ./internal/official/ ./internal/store/ -v 2>&1 | tee .scratch-jal/verify1.log | grep -cE '^(--- FAIL|FAIL|panic)'; grep -E -- '--- SKIP' .scratch-jal/verify1.log | grep -vcE 'TestOfficialLive|TestScreeningFixture'; grep -cE -- '--- PASS: (TestStoredReliableAndChangesFlagsNeverReachThePhone|TestStoredCriticalAndOfficialOffStillReachThePhone|TestFollowNoteReachesThePhoneWithEveryFlagOff|TestReliableBellLookBackIsNamed)' .scratch-jal/verify1.log</automated>
  </verify>
  <done>The first count is 0 (no failure), the second is 0 (the only skips are TestOfficialLive and TestScreeningFixture, two of the three allowed) and the third is 4. go build ./... and go vet ./... are clean, internal/api/handlers and internal/api pass on q_notif2_a, and git diff --stat for internal/store/sqlc names only push.sql.go, official_push.sql.go and updates.sql.go. One path limited commit with the trailer.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: The settings struct, the alerts API and the two row Settings box (D1, D7, D8)</name>
  <files>internal/service/alertsettings.go, internal/service/alertsettings_test.go, internal/service/updatesalerts_test.go, internal/api/handlers/me.go, internal/api/handlers/settings.go, internal/api/router.go, docs/docs.go, docs/swagger.json, docs/swagger.yaml, web/templates/settings.html.tmpl, web/static/js/settings-alerts.js, web/static/css/places.css, web/settings_rows_contract_test.go, web/settings_alerts_test.mjs, internal/api/handlers/alerts_settings_e2e_test.go, internal/api/handlers/push_settings_e2e_test.go, internal/api/handlers/official_settings_e2e_test.go, internal/api/handlers/swagger_test.go, internal/api/handlers/places_e2e_test.go</files>
  <behavior>
    - PATCH /api/me/alerts with {"alert_radius_km":2} answers 200 with exactly {"alert_radius_km":2,"has_home_area":...} and stores 2; a bad distance answers 400 field alert_radius_km with "Choose 1, 2, 5 or 10 km.".
    - A body with alert_radius_km plus any of the four legacy keys answers 200, saves the distance, and leaves the four stored columns and enabled_at exactly as they were (read back by SQL).
    - A body with only legacy keys (for example {"critical_push":false} or {"reliable_alerts":true}) answers 200 with the stored settings and changes nothing; {} answers 400 field body "Nothing to save."; {"alert_radius_km":5,"account_id":2} answers 400 field body; a malformed body answers 400 field body; an anonymous caller gets 401.
    - The response JSON never holds reliable_alerts, critical_push, changes_push or official_push, and docs/swagger.json documents none of them.
    - (e) GET /settings renders the Notifications box with exactly two direct children, the device row wrapper and the distance wrapper; the device switch row's label is Alerts on this device with the sub line Critical reports and severe official alerts near your home area, and posts by people you follow.; the install, blocked and unsupported variants read Alerts on this device with their own sub lines; with a home area the select shows and the no home button is hidden, without one the reverse.
    - settings-alerts.js: picking a distance saves alert_radius_km at once, a failed or offline save puts the previous option back with the one error line, the no home button opens the Home area window and focus returns to the select (home area now saved) or to the button; the device row behaves exactly as before.
  </behavior>
  <action>
1. internal/service/alertsettings.go per C10 and D7: AlertSettings keeps AlertRadiusKm and HasHomeArea; AlertSettingsUpdate keeps AlertRadiusKm; AlertSettings() reads GetAlertSettings for the distance (no row: DefaultAlertRadiusKm) and HomeAreaCell, and stops reading GetPushKindSettings and GetOfficialPushSetting; UpdateAlertSettings refuses a nil distance with AlertsNothingToSaveMessage and a bad one with AlertRadiusMessage, reads the stored row (no row: switch false), then calls UpsertAlertSettings with the stored reliable_alerts_on, the new distance and now, and answers the stored distance and HasHomeArea. Remove AlertsNeedHomeAreaMessage if nothing else uses it. Rewrite the doc comments (file header, the struct with the D8 note naming the four kept columns and owner decision 2026-11-03, the two methods). Leave the querier interface in profile.go as it is (C6).
2. internal/api/handlers/me.go per C4: AlertSettingsRequest has AlertRadiusKm *int (json alert_radius_km, example 5) and the four legacy *bool fields with their old json names and swaggerignore:"true", with a comment saying they are accepted from old cached pages and ignored (owner 2026-11-03, quick 261103-jal, D7). AlertSettingsResponse has AlertRadiusKm and HasHomeArea only. The handler: after the strict decode, when AlertRadiusKm is nil and any legacy key is present, answer 200 with the stored settings from accounts.AlertSettings (Cache-Control no-store as now); otherwise call UpdateAlertSettings with the distance. Rewrite the swag block: summary Choose how far from the home area alerts reach; the description says the one key alert_radius_km (1, 2, 5 or 10 km, 5 until changed), measured from the middle of the saved home area, that it applies to Critical reports and the Reliable reporter bell row, that whether a device gets notifications is decided only by that device's own registration, that older keys from earlier versions are accepted and ignored, and that the answer is the stored distance and whether a home area is saved. Plain punctuation only: no long dashes.
3. internal/api/router.go: rewrite the comment above the PATCH route to say it saves the distance from the home area (Phase 3.2, NOTIF-05, D-26; narrowed by quick 261103-jal). Do not touch the swag header lines; the license line stays exactly as it is.
4. make swag. Check: grep -c critical_push docs/swagger.json is 0; grep -c 'Unlicensed (all rights reserved)' is 1 in each of internal/api/router.go, docs/docs.go, docs/swagger.json and docs/swagger.yaml; the long dash byte grep over the three generated files is empty; git diff --stat -- docs/ names only those three files.
5. internal/api/handlers/settings.go: settingsViewModel drops AlertsOn, CriticalPushOn, ChangesPushOn and OfficialPushOn and keeps AlertRadiusKm, HasHomeArea, AlertRadii and PushPublicKey; update the field comments and the vm literal.
6. web/templates/settings.html.tmpl per D1 and C9: the box holds the device row wrapper and the distance wrapper only. The device switch label gets the label text Alerts on this device and the sub line span (class place-row__sub) with the D1 sentence; the install, blocked and unsupported variants change only their label text. The Critical, official, Changes and Reliable wrappers and rows are deleted. The distance wrapper holds the existing select row (hidden without a home area, same ids) and the new no home button row. Update the comments above the box and the install guide window (they name the four wrappers and the On this device row). Keep the section title, its line and the two error lines.
7. web/static/js/settings-alerts.js: delete the Reliable, Critical, official and Changes controls, their saved values, saveKind, their change listeners, the three no home click handlers and their focus branches; CONTROLS and saveAlerts handle alert_radius_km only; redrawAlerts shows the select row or the no home button; openHomeFrom and the window observer return focus per C9; the exported PinalertSettingsAlerts keeps openWindow, closeWindow, loadMuted, unmute, logOutAll, openDeleteConfirm, deleteAccount, saveAlerts, redrawAlerts and drawDevice (saveKind goes). Rewrite the header and section comments to describe the two rows. The device code stays as it is apart from comments that name the label.
8. web/static/css/places.css: only the comment above the guarded .place-row display rule that names the On this device and Critical variants; it now names the device and distance variants. No rule changes.
9. Tests, each with the dated Amended line: internal/service/alertsettings_test.go (the radius only service; a distance save keeps reliable_alerts_on and enabled_at as stored, seeded by SQL; TestCriticalDefaultMatchesSQL stays); internal/service/updatesalerts_test.go (any UpdateAlertSettings call that set the Reliable switch becomes a SQL seed or goes, since D4 needs no switch); internal/api/handlers/alerts_settings_e2e_test.go, push_settings_e2e_test.go and official_settings_e2e_test.go (the behavior list above, the stored columns read back by SQL; keep each file's anonymous and bad body cases); swagger_test.go around lines 928 to 931 (request lists alert_radius_km only, because swaggerignore keeps the legacy fields out; response lists alert_radius_km and has_home_area); internal/api/handlers/places_e2e_test.go (the button id list now holds settings-device-install then settings-radius-nohome where the three no home rows were; the wanted strings swap the Reliable label for Alerts on this device and keep Add a home area first) and add the (e) check there or in a sibling test: extract the box from the rendered page and require exactly the two wrappers as its direct children, the label and the sub line; web/settings_rows_contract_test.go (TestSettingsAlertsScriptContract, TestPhoneNotificationsSection, TestPhoneNotificationsRowsNeedAHomeArea, TestSettingsAlertsSavesAtOnce, TestNotificationsSectionRows, TestNotificationsDeviceStates, TestNotificationsNoFollowOrOfficialRow and TestNotificationsOfficialRow now pin the two rows, the new label and sub line, the four variant labels, the radius no home button, and the absence of the four removed labels and switch names; keep each test name unless it no longer describes anything, then fold it into TestNotificationsSectionRows); web/settings_alerts_test.mjs (the Reliable switch tests around lines 854 to 951 become the same save, error, offline and next save cases on the distance select; the no home tests around lines 1004 to 1101 move to the radius no home button; the device tests stay).
10. Run go build ./..., go vet ./..., node --test web/settings_alerts_test.mjs, go test -count=1 ./web/ (no database), and the database form over ./internal/service/ ./internal/api/... to .scratch-jal/task2.log. Any other test that fails only because it read a removed struct field, view model field or response key (likely candidates: internal/service/accountdelete_test.go, internal/service/commentlikes_test.go, internal/api/handlers/me_e2e_test.go, internal/api/handlers/delete_account_e2e_test.go) is updated the same way, added to this commit and named in the SUMMARY. Commit "feat(quick-261103-jal): Settings has two notification rows, the alerts API takes only the distance" with a body naming D1, D7, D8, C4, C9, C10 and the trailer, path limited to this task's files.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && DATABASE_URL='postgres://localhost:5432/q_notif2_a?sslmode=disable' go test -count=1 -p 1 ./internal/service/ ./internal/api/... ./web/ 2>&1 | tee .scratch-jal/verify2.log | grep -cE '^(--- FAIL|FAIL|panic)'; node --test web/settings_alerts_test.mjs 2>&1 | grep -E '^# (pass|fail)'; cat web/templates/settings.html.tmpl web/static/js/settings-alerts.js | grep -cE 'settings-(critical|official|changes|reliable)'; sed -n '/^type AlertSettingsResponse struct/,/^}/p' internal/api/handlers/me.go | grep -cE 'reliable_alerts|critical_push|changes_push|official_push'; grep -c critical_push docs/swagger.json</automated>
  </verify>
  <done>The Go failure count is 0, node reports fail 0, and the last three counts are 0. The license line appears once in each of the four files, the generated docs have no long dash, and Settings renders exactly the two rows. One path limited commit with the trailer.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: The simpler pop-ups and the changes pop-up removed (D5, D6)</name>
  <files>web/templates/welcome.html.tmpl, web/static/js/welcome.js, web/templates/installed_alerts_ask.html.tmpl, web/static/js/installed-ask.js, web/static/css/auth.css, web/templates/changes_ask.html.tmpl, web/static/js/changes-ask.js, web/templates/index.html.tmpl, web/templates/low.html.tmpl, web/static/js/votes.js, web/static/js/report-form.js, internal/api/handlers/page.go, internal/api/handlers/low.go, internal/api/router.go, internal/api/handlers/page_test.go, internal/api/handlers/welcome_e2e_test.go, web/changes_ask_contract_test.go, web/changes_ask_pages_contract_test.go, web/changes_ask_test.mjs, web/changes_ask_hooks_test.mjs, web/demo_contract_test.go, web/low_contract_test.go, web/low_budget_contract_test.go, web/noise_words_contract_test.go, web/phone_markup_contract_test.go, web/review71_templates_fix_contract_test.go, web/report_form_session_test.mjs, web/votes_stars_test.mjs, web/installed_ask_test.mjs, web/installed_ask_contract_test.go, web/welcome_alerts_test.mjs, web/welcome_contract_test.go</files>
  <behavior>
    - (f) The rendered sign up page's alerts pop-up holds no checkbox at all, and its words still match the installed app ask (TestInstalledAlertsAskMatchesSignUp), which holds none either.
    - Sign up pop-up, Allow: enable is still the first real statement of the tap; then the picked home area is saved when one was picked (with the WR-05 error line and retry kept), then first-run-done; no request ever goes to /api/me/alerts. Not now and Escape save nothing and finish sign up.
    - Installed app ask, Allow: enable first, then the dialog closes; no request is sent. The once per device and account list, the location wait, the page locked skip, the Mac Dock app never asked rule and Not now are unchanged.
    - Nearby and the Low data view render no changes pop-up and link no changes script for any account, with or without a push key; a saved confirm or dispute and an accepted post call nothing of the removed pop-up; the star box after Yes, mark resolved mounts at once.
    - The Policy still lists the retired changes asked key (C5) and TestPolicyNamesOlderStorageKeys passes with it marked retired.
  </behavior>
  <action>
1. Sign up pop-up (D5): in welcome.html.tmpl delete the tick box label inside the alerts pop-up (nothing else in the dialog changes). In welcome.js delete the askReliable read, its line in setAskDisabled, saveReliable, the wantReliable capture and the Reliable step of the onAskAllow chain (the home area step now leads straight to first-run-done with the same false return that keeps the pop-up open after a failed home save), and ALERTS_URL if nothing else uses it. Rewrite the header paragraph of plan 03.3-19 and the comments of saveAskHome and onAskAllow so they no longer name the tick box or the Reliable switch, and reword the three comment lines that name the removed script by file name (around lines 59, 119 and 1342: the install asked list format, the account id shape and the older plain id format) so they describe the format without naming that file.
2. Installed app ask (D5): in installed_alerts_ask.html.tmpl delete the tick box label and update the partial's comment. In installed-ask.js delete the reliable element read, its setBusy lines, saveReliable, wantReliable and the save step (Allow becomes enable, then finish whatever enable answers), ALERTS_URL, REQUEST_TIMEOUT_MS and the fetchFn option and parameter (unknown options are ignored, so a test that still passes it keeps working until it is edited); keep the run options comment true. Rewrite the header's ALLOW paragraph and the two comment lines that name the removed script by file name (around line 32, the list of small dialogs on Nearby, which now holds this ask and the star box only, and around line 61, the account id shape).
3. web/static/css/auth.css: delete the two rules for the tick box label and its checkbox and their comment block; keep the #installed-alerts-ask-title margin rule.
4. The changes pop-up (D6): git rm web/templates/changes_ask.html.tmpl and web/static/js/changes-ask.js. In index.html.tmpl and low.html.tmpl delete the whole if block that includes the partial and its script. In page.go delete changesAskFor, the ChangesAsk field and its use in Page, the AlertSettings field of PageConfig and the AlertSettingsLookup type; in low.go delete the ChangesAsk field and its use; in router.go delete the line that sets page.AlertSettings and its comment. In votes.js delete offerChangesAsk and its call after a saved confirm or dispute, and in offerRateBox delete the wait on the pop-up so the box mounts at once through the existing no wait branch (keep rateGeneration where Undo still uses it, and update the function's doc comment). In report-form.js delete offerChangesAsk and its two calls.
5. Tests, each existing one changed with the dated Amended line: git rm web/changes_ask_contract_test.go, web/changes_ask_pages_contract_test.go, web/changes_ask_test.mjs and web/changes_ask_hooks_test.mjs; internal/api/handlers/page_test.go (delete the changesAskFor and ChangesAsk tests and any PageConfig AlertSettings setup); web/report_form_session_test.mjs (delete the five changes ask tests around lines 586 to 641, withChangesAsk and the changesAsk load option); web/votes_stars_test.mjs (delete the tests that need the pop-up to be pending or settled, around lines 499, 511, 701, 828 and 845, and the changesAsk load option; the Undo cases they guarded are covered by the existing Undo tests around lines 667 and 684, check they stay green); web/demo_contract_test.go (around lines 143 and 180, drop the entries that name the deleted partial and script); web/low_contract_test.go (around lines 179 to 200, drop the changes script from the later map and its special case); web/low_budget_contract_test.go (one dated comment line, numbers unchanged); web/noise_words_contract_test.go (drop the two deleted files from the scanned list around lines 54 and 55 and its comment); web/phone_markup_contract_test.go (drop the case for the deleted partial around line 85); web/review71_templates_fix_contract_test.go (TestReliableTickBoxSharedLook becomes a test that no template has a checkbox in either alerts pop-up and no stylesheet styles the removed label; TestPolicyNamesOlderStorageKeys keeps the Policy line and marks the changes asked key retired per C5, with no script check for it); web/installed_ask_test.mjs (the Reliable tests around lines 416, 459 and 657 become: Allow calls enable first and sends no request; the fake no longer builds the tick box); web/installed_ask_contract_test.go (the words match without the tick box); web/welcome_alerts_test.mjs (the named pop-up tests around lines 775 to 950 drop the tick box and the Reliable request; the order is enable, then the home area save when picked, then first-run-done, and no request goes to the alerts address); web/welcome_contract_test.go (TestWelcomeAlertsAsk and TestWelcomeAlertsScript pin the absence of the tick box, its id and the Reliable save, for D9 f); internal/api/handlers/welcome_e2e_test.go (around lines 557 to 571 the rendered pop-up holds no checkbox; the device guide lead around line 598 stays).
6. Run go build ./..., go vet ./..., node --test web/*.mjs, go test -count=1 ./web/ and the database form over ./internal/api/... to .scratch-jal/task3.log. Commit "feat(quick-261103-jal): the alerts asks lose the Reliable tick box and the changes pop-up is gone" with a body naming D5, D6, C5 and the trailer, path limited to this task's files, the four deleted tests and the two deleted files included.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && DATABASE_URL='postgres://localhost:5432/q_notif2_a?sslmode=disable' go test -count=1 -p 1 ./internal/api/... ./web/ 2>&1 | tee .scratch-jal/verify3.log | grep -cE '^(--- FAIL|FAIL|panic)'; node --test web/*.mjs 2>&1 | grep -E '^# (pass|fail)'; test ! -e web/templates/changes_ask.html.tmpl && test ! -e web/static/js/changes-ask.js && echo gone; git grep -nE 'PinalertChangesAsk|changes_ask|changes-ask|ChangesAsk' -- web/static web/templates internal | wc -l; cat web/templates/welcome.html.tmpl web/templates/installed_alerts_ask.html.tmpl web/static/js/welcome.js web/static/js/installed-ask.js web/static/css/auth.css | grep -cE 'welcome-ask__check|alerts-reliable|alerts-ask-reliable'</automated>
  </verify>
  <done>The Go failure count is 0, node reports fail 0, "gone" is printed, and the last two counts are 0. git diff for web/templates/legal_body.html.tmpl, internal/api/handlers/legal.go and internal/service/profile.go is empty. One path limited commit with the trailer.</done>
</task>

<task type="auto">
  <name>Task 4: Docs, the 03.3 amendment and the full gate (D10)</name>
  <files>docs/PUSH.md, docs/OFFICIAL-ALERTS.md, docs/TRUST-MODEL.md, docs/MODERATION.md, README.md, .planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md</files>
  <action>
1. docs/PUSH.md: rewrite the What they are list as the two Settings rows (Alerts on this device and How far from my home area) plus a plain list of what reaches the phone (Critical reports near the home area, Severe official alerts for the area, notes about people you follow with Notify me on, and the confirmed update of the first and the last) and of what stays in the bell only (Reliable reporter posts near the home area, now for everyone with a home area with no switch; changes and comments on reports you posted, voted on or commented on). Rewrite the pop-ups paragraph: the sign up pop-up and the installed app ask have no tick box; the changes pop-up was removed on 2026-11-03; keep the pinned sentence "A home area is always set by then, because step 4 of sign up requires one; the pop-up keeps its own home area search only as a fallback." word for word. Update every later mention of the device row to Alerts on this device, the Follow notes Order and limits bullet (Critical, then Follow; the Reliable branch refreshes bells only), and the Honest limits bullets on the fades note, comment notes and Counts (bell only for the change and comment kinds; the cap of 4 an hour now holds back follow notes, with Critical counted and never held). Keep owner step 0 and the device checks' structure; check 1 to 3 and 7 use the new label.
2. docs/OFFICIAL-ALERTS.md: the Severe and Extreme alerts bullet (around line 100) and the send time list (around line 106): no Settings switch; the device switch decides; the switch is on check is gone.
3. docs/TRUST-MODEL.md (keep every heading text exactly as it is): Reliable reporter alerts says who gets one now (every signed in reader with a home area, no switch), the start point (C1, naming ReliableBellLookBack in backticks), that it never reaches a phone since 2026-11-03, and restates why: the alert used to be opt in, and it is now shown to everyone with a home area in the bell only, softened by the same per reporter cap, hourly ceiling and Not confirmed yet wording; the TRUSTX-03 dependency is met for the phone by not pushing this kind at all. Phone alerts: the kinds list becomes what reaches the phone and what stays in the bell; the gate paragraph says the worker checks the kind may reach a phone at all instead of the person's switches. The open gap paragraph: Reliable no longer pushes, the follow kind still pushes on the strength of the tag, so burst detection stays a dependency for it. The comments line (around line 686), the privacy line (around line 889: the device switch is the control, and a person with no home area gets no Critical or official note) and the out of scope line (around line 910) follow. Keep every cited test name present.
4. docs/MODERATION.md: one dated line in the Low data view size budget section saying the changes pop-up and its script were removed on 2026-11-03 by quick 261103-jal, the figures stand because the page only got smaller.
5. README.md Phone notifications paragraph: Critical reports near the home area, severe official alerts and people you follow with Notify me on; no change notes; people turn it on with Alerts on this device in Settings, under Notifications.
6. .planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md: insert above the line "## Owner amendments, 2026-10-27 and 2026-10-28 (owner checks, 03.3-UAT.md)" a new section titled exactly "## Owner amendment, 2026-11-03 (two row notifications box, quick 261103-jal)". Its first line quotes the owner and says the owner delegated the decision and that it replaces anything below that disagrees. Then one plain bullet each for D1 to D6, and a line naming what it replaces: 03.3 Q1 and the five kinds of the 2026-10-16 late amendment (each kind with its own switch), the 2026-10-27 Ask where it makes sense bullet for the Reliable tick box and the first post or vote pop-up, and 03.2 D-26's opt in switch for the Reliable bell row; and that Phase 3.2's Reliable filter on Nearby stays.
7. Doc tests: go test -count=1 ./web/ and the database form for ./internal/service/ -run 'TestTrustModelDocMatchesCode|Doc' to .scratch-jal/docs.log. A doc test that pins a sentence these decisions make false is amended with the dated line and named in the SUMMARY; a pinned sentence that is still true stays word for word.
8. Commit "docs(quick-261103-jal): the two row notifications box in the docs and the 03.3 amendment" with a body naming D10 and the trailer, path limited to the six docs files plus any doc test amended in step 7.
9. Full gate, each to .scratch-jal/: go build ./... ; go vet ./... ; the database form with go test ./... -p 1 -count=1 -v (every SKIP line must belong to TestGeocodeLive, TestOfficialLive or TestScreeningFixture) ; node --test web/*.mjs ; the neutral wording grep from the rules (prints nothing) ; the long dash byte grep over every file this plan touched (prints nothing) ; git status shows nothing of this plan unstaged. Then dropdb q_notif2_a and delete .scratch-jal.
10. Write .planning/quick/261103-jal-notifications-cut-to-two-rows-alerts-on-/261103-jal-SUMMARY.md (frontmatter like the other quick SUMMARYs; what changed per task with commit shas; C1 to C10 as decisions; the C3 frozen flag note, the C5 Policy note and the D4 widening of the Reliable bell row as owner facing notes; every test changed, added or deleted; the gate results) and commit it alone, last, with "docs(quick-261103-jal): summary" and the trailer.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && DATABASE_URL='postgres://localhost:5432/q_notif2_a?sslmode=disable' go test ./... -p 1 -count=1 -v 2>&1 | tee .scratch-jal/full.log | grep -cE '^(--- FAIL|FAIL|panic)'; grep -E -- '--- SKIP' .scratch-jal/full.log | grep -vcE 'TestGeocodeLive|TestOfficialLive|TestScreeningFixture'; node --test web/*.mjs 2>&1 | grep -E '^# fail'; git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder | wc -l; grep -c '^## Owner amendment, 2026-11-03 (two row notifications box, quick 261103-jal)$' .planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md</automated>
  </verify>
  <done>The failure count is 0, the unexpected skip count is 0, node reports fail 0, the wording count is 0 and the amendment heading count is 1, placed before the 2026-10-27 heading. q_notif2_a is dropped and .scratch-jal is gone. The docs commit, then the SUMMARY commit alone and last, each path limited with the trailer. Nothing pushed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser to PATCH /api/me/alerts | an untrusted JSON body, now with one honoured key and four accepted and ignored legacy keys |
| dispatcher and worker to the push services | who gets a lock screen note; a wrong rule reaches someone's pocket |
| report store to the bell (refreshReliableAlerts) | a Reliable tag, which is farmable, now reaches every account with a home area in the bell |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-jal-01 | Tampering | PATCH /api/me/alerts legacy keys | low | mitigate | DisallowUnknownFields stays; the four legacy fields are decoded into *bool and never passed to the service (C4); tests read the stored columns back by SQL to prove nothing changed |
| T-jal-02 | Information disclosure | phone notes after the change | medium | mitigate | kindSwitchOn answers false for reliable and changed rows, so a row queued before the change is skipped at send time; test (a) proves both enqueue and send sides |
| T-jal-03 | Spoofing | Reliable tag widened to every bell (D4) | medium | accept | owner decision 2026-11-03; the per reporter cap (ReliableAlertsPerReporterPerHour), the hourly ceiling (UpdatesHourlyCeiling), the Not confirmed yet wording, mute and block all stay; the phone is no longer reached by this kind, which closes the TRUSTX-03 push dependency for it; recorded in TRUST-MODEL.md |
| T-jal-04 | Denial of service | reliableBranch refreshes every subscribed neighbour of a non serious Reliable report | low | accept | bounded by PushRecipientsPerReport, the reporter cap of 2 Reliable reports an hour and the fanned mark that stops sweep repeats (C2); personal project scale |
| T-jal-05 | Denial of service | refreshReliableAlerts now runs a Nearby read on every bell poll of every account with a home area | low | accept | the same read the feed does, bounded by the chosen distance; at this scale the cost is small and documented in the SUMMARY |
| T-jal-06 | Repudiation | stored flags kept but ignored (D8) | low | accept | dated comments at the queries and the struct say the columns no longer decide delivery; no migration |
| T-jal-07 | Information disclosure | Policy storage list keeps a retired key (C5) | low | accept | the key may still sit on devices that were asked before; keeping the line keeps the Policy complete without asking every account to agree again; flagged to the owner |
</threat_model>

<verification>
- go build ./... and go vet ./... are clean.
- go test ./... -p 1 -count=1 -v on q_notif2_a passes; only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skip.
- node --test web/*.mjs passes.
- D9 a to f each have a named passing test: TestStoredReliableAndChangesFlagsNeverReachThePhone, TestStoredCriticalAndOfficialOffStillReachThePhone plus the official send case, the (c) cases in updatesalerts_test.go and reliable_alert_e2e_test.go, TestFollowNoteReachesThePhoneWithEveryFlagOff, the two row check of Task 2 and the no tick box checks of Task 3.
- The swagger license line and plain punctuation checks hold; the neutral wording grep prints nothing; no long dash in any touched file.
</verification>

<success_criteria>
The owner opens Settings and sees two notification rows; a phone with Alerts on this device on gets Critical, severe official and follow notes and nothing else, whatever was stored before; every account with a home area sees Reliable reporter posts near home in the bell without old ones flooding in; the sign up and installed asks have no tick box; no changes pop-up appears after a post or a vote; the docs and the 03.3 context say so; nothing pushed, no migration, the full suite green.
</success_criteria>

<output>
Create `.planning/quick/261103-jal-notifications-cut-to-two-rows-alerts-on-/261103-jal-SUMMARY.md` when done, committed alone and last.
</output>

## Source coverage audit

| Source | Item | Covered by |
|--------|------|-----------|
| GOAL | Owner 2026-11-03: a simpler notifications part with only the really important things | Tasks 1 to 4 |
| CONTEXT | D1 two rows, label, sub line, variants, radius no home button, removed rows, JS and CSS | Task 2 (C9), Task 3 (auth.css) |
| CONTEXT | D2 Critical, official, follow and their confirmed updates reach the phone, two flags ignored | Task 1 |
| CONTEXT | D3 Reliable, changed, comment and Reliable confirmed updates never reach the phone, send time skip, no enqueue, bell refresh kept | Task 1 (C2, C3, C7) |
| CONTEXT | D4 bell: changes kept, Reliable for every home area account, caps kept, start point | Task 1 (C1) |
| CONTEXT | D5 sign up and installed ask lose the tick box and the Reliable save | Task 3 |
| CONTEXT | D6 changes pop-up removed with includes, hooks, server flag and tests | Task 3 (C5) |
| CONTEXT | D7 API narrowed, legacy keys accepted and ignored, swagger regenerated, license line kept | Task 2 (C4) |
| CONTEXT | D8 no migration, dated comments at queries and struct | Task 1 (SQL), Task 2 (struct), C6 |
| CONTEXT | D9 tests a to f and the kept cap, Critical, Apple and Mac Dock rules | Tasks 1 to 3 |
| CONTEXT | D10 docs and the 03.3 amendment | Task 4 |
| REQ | No requirement IDs for a quick task; QUICK-261103-jal stands in | frontmatter |
| Excluded | Phase 3.2 Reliable filter on Nearby (kept on purpose), Policy text and terms version (C5) | not touched |
