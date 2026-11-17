# Phone notifications

This document is for whoever runs Pinalert. It says what phone notifications are, how to switch them on for a deployment, what they can and cannot promise, how delivery works, and what the owner has to check on real devices. The trust rules (who gets which note and why) are in [TRUST-MODEL.md](./TRUST-MODEL.md), under "Phone alerts". This file is about running it.

## What they are

A phone notification is a short note on the lock screen of a phone or a computer, sent by the server through the browser's own push service, even when Pinalert is closed. People are asked where it makes sense, and can change their mind in Settings, under Notifications. Since 2026-11-03 (quick 261103-jal, an owner decision) that box holds two rows only:

- Alerts on this device. The one switch, for the browser or installed app they are using. Turning it on asks the browser for permission. Its sub line says what it covers: Critical reports and severe official alerts near your home area, and posts by people you follow. The device switch is the only way to stop these notes on a device.
- How far from my home area. 1, 2, 5 or 10 km, or an Add a home area first button when there is no home area. It applies to Critical reports and to the Reliable reporter row in the bell, not to official alerts, which follow the alert's own district.

What reaches the phone, on a device whose switch is on:

- Critical reports near the home area, within the chosen distance.
- Severe official alerts for the area. These are government alerts copied from NDMA SACHET, not reports; see "Official alerts" below.
- Notes about people the person follows, with Notify me on for that person (Phase 9, D-11). The Notify me bell on the followed person's profile is the switch, one person at a time, off by default. See "Follow notes" below.
- The confirmed update of a Critical note or a follow note, which replaces it in place.

What stays in the bell and never reaches a phone:

- A Reliable reporter posts near me. Since 2026-11-03 this row of the bell is shown to every account that has a home area, with no switch, and it never becomes a phone note. It looks back at most an hour (`ReliableBellLookBack`), so nothing older is ever back filled.
- Changes and comments on reports a person posted, voted on or commented on.
- The confirmed update of a Reliable note, if one was sent before that date. A row of those kinds that was queued earlier is skipped when the worker reaches it.

The four stored switch columns of the older versions (Reliable, Critical, Changes and official) are kept in the database for the SQL defaults and no longer decide delivery. A request that still carries their keys (an old cached page) is accepted and ignored.

Two small pop-ups ask in the app's own words before the browser's prompt. At the end of sign up one asks Get alerts for critical reports near {home area}? A home area is always set by then, because step 4 of sign up requires one; the pop-up keeps its own home area search only as a fallback. It has no tick box and sends nothing but the turn on, and the home area when it was picked there. The older pop-up that asked Tell you when this report changes? after a first post, confirm or dispute was removed on 2026-11-03; a device that was asked before may still hold its small browser key, which the Policy still names. The pop-up does not appear in an iPhone Safari tab (alerts need the Home Screen app there) or in a Mac Dock app (see Honest limits). Refusing there changes nothing else. Phase 7.1 added two more asks for the iPhone and iPad: a sign up in a Safari tab ends with a small box that shows the three install steps and a Not now button, and once the person opens Pinalert from the Home Screen and signs in, Nearby asks Get alerts for critical reports near {home area}? one time on that device, again with no tick box. Each is asked once per account on a browser.

Phone notifications are optional. With no keys set the whole feature is off and the app works as before.

## Setting up the keys

Phone notifications need a VAPID key pair, which identifies this server to the push services.

1. Make the pair once, on your own machine: `go run ./cmd/vapidkeys`. It prints `VAPID_PUBLIC_KEY=...` and `VAPID_PRIVATE_KEY=...` to standard output and writes no file.
2. Store both values as secrets in the host's settings, as `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY`. Never put them in the repo, in an example env file, in a README, in a ticket or in a chat. The private key can send as this site.
3. Set a contact for the push services. `VAPID_SUBJECT` is used first, then `CONTACT_EMAIL`, then `BASE_URL` when it is an https address. The value is an email address or an https address. With keys set and none of these usable the server stops at startup and says which variable to set.

What the server does at startup:

- Both keys unset: it logs one warning and runs without phone notifications. The notification controls show as unsupported on every device. Nothing else changes.
- Both keys set and valid: it starts the dispatcher, which decides who gets a note, and the worker, which sends it. Both run inside the server process.
- Only one key set, a key that is not a P-256 key, a pair that does not belong together, or a bad contact: the server stops at startup. The message names the variable and never prints a key.

Rotating the keys. The browser ties each subscription to the public key it subscribed with. If the pair changes, every stored device stops working (Apple answers that the key does not match). When a person opens Settings, the page compares the browser subscription with its own key, and a device whose subscription was made with the old key shows as off even while the server still holds its row. One tap on On this device then drops the old subscription and makes a new one with the new key. A device that does not open Settings gets nothing until it does, and the server removes its old row once the last good send is more than 7 days old. Rotate only when you have a plan to ask everyone again. Keys are never generated by the server, in development too.

## Https, and the push services the server accepts

Browsers allow push only on an https address. `http://localhost` also works, so a desktop check can run on a laptop with no certificate. Android and iPhone need the real https address, so they wait for the hosting choice.

The server only sends to push services it knows, and refuses any other endpoint a browser hands it:

| Service | Host the server accepts |
|---------|-------------------------|
| Apple (iPhone, and Safari on a Mac) | any host ending in `push.apple.com` |
| Google (Chrome, Edge on Android and desktop) | `fcm.googleapis.com` |
| Mozilla (Firefox) | `updates.push.services.mozilla.com` |
| Microsoft | any host ending in `notify.windows.com` |

Only the Apple entry is settled. The other three are the usual names for those services and are still to be confirmed from real devices. Every stored subscription keeps the host it came from, so after the owner's device checks, run this against the owner's own database and compare the answer with the table:

```sql
SELECT endpoint_host, count(*) FROM push_subscriptions GROUP BY endpoint_host;
```

A host that is missing from the table makes registration answer 400 with the line "This browser's notification service is not supported." for that browser. Adding one is a change to `DefaultEndpointPolicy` in `internal/push/endpoint.go`.

## What a lock screen shows

Only these things:

- The pick the reporter chose, such as Water rising, or the category and Critical when there was no pick.
- near and a coarse area name, cut to 28 characters. If the area lookup found nothing the area is left out.
- A distance in whole kilometres, or Under 1 km, from the person's own home area.
- Not confirmed yet., until people nearby confirm the report, and then Confirmed by people nearby.

A follow note shows the same things in the same order, with one different line: its body opens with From someone you follow. It names nobody, not even the username of the person followed, and it has no report words.

A comment note shows only a fixed phrase after the category: "{Category} report near {area} has a new comment" or "has a new reply". It never carries the comment's words, a username or a count.

It never shows the reporter's description, name or email, a phone number, a street, a coordinate, a vote count, or anyone's score. The area name is kept for the lock screen only. It is in no feed, bell or page response, with one exception added by Phase 8: the Share window title and the public report page `/r/{token}` also carry it, as a coarse area, so the link preview can say where. That looked up name is the same one, stored once per report.

The What is happening pick stays optional on the report form, as Phase 3.4 built it. A Critical report posted without one gets its category and Critical in the title instead (the older words). Making the pick required is a question for the owner and is not decided here.

Tapping a note opens that report on Nearby.

## Follow notes

Phase 9 added a phone note for a person someone follows. The trust side, under the one rule `MayNotifyAboutReport`, is in TRUST-MODEL.md under "Follows, mute, block and follow notes". For whoever runs the server:

- One row, two channels. The bell step of the follower's refresh writes a followed_posted row, and the phone note is queued from exactly that row, so a phone note never exists without its bell row. The row is the follower's choice: Notify me must be on for that person, and only reports posted after it was switched on get a note.
- The words. The title is the pick, else the category, then near and the coarse area. The body is "From someone you follow.", then the distance from the follower's home area if there is one, then Not confirmed yet. Once people nearby confirm the report it becomes "Confirmed by people nearby." with the distance. There is no username on the lock screen, no report words and no coordinate.
- Distance and home area are ignored for the decision. The follow is the reason the person gets it, so there is no radius check and no home area is needed. A follower with a home area gets the distance words, and a follower without one gets the note with no distance words at all.
- Checked again just before each send. The worker reads again that the follow is still active, the bell is still on, the followed person is still followable, no block exists in either direction, the follower has not muted them, and the reporter still holds the Reliable reporter tag, read with the tapped ratings. It stops at the first thing that no longer holds, and the skip reason is not_eligible. So a note stops the moment the follow, the bell, the profile, the tag, a block or a mute goes, without waiting for the sweep. A Reliable reporter note queued before 2026-11-03, and the confirmed update that replaces one, are skipped as not_eligible whatever a mute or a block says, because that kind never reaches a phone now (`TestQueuedReliablePushStopsAfterMuteOrBlock`, `TestConfirmedUpdateOfReliablePushStopsAfterMuteOrBlock`). The SQL check that a queued comment note stops once either person blocked the other is kept (`TestCommentPushStillHoldsAfterABlock`). A confirmed update whose first row was pruned is skipped as not_eligible, because the kind it replaces is no longer known and a guess would skip these checks (`TestConfirmedUpdateWithoutItsFirstRowIsSkipped`).
- A waiting note is deleted when it no longer should go. Every way a follow or the bell ends (Unfollow, Remove follower, Remove all followers, Block either way, switching the profile off, the sweep, a lost tag found on a touch, a rating change or a moderator removal that ends follows, a Mute, deleting the person's account, and the bell switched off) deletes the queued follow notes of that pair at once. A note a worker already holds is not deleted under it, and the send time check covers it. A bell row stays as history and reads "This report is no longer available" when it no longer applies.
- Order and limits. The dispatcher runs Critical, then Follow, then the confirmed update. It has no Reliable branch, and its change branch only refreshes the bells of one report's own people early and queues nothing (since 2026-11-03; the Reliable early refresh and the periodic changes sweep were removed by review WR-04 of quick 261103-jal, because they fed no phone note and could hold up a Critical fan out on the single dispatcher goroutine). A follower already queued as a Critical note is never queued as a follow note for the same report, because the two share one first alert row per person and report. One note per report per device: an Apple endpoint gets only the first push about a report, and on other browsers the confirmed update replaces the follow note in place through its tag and is not counted. A follow note counts toward the limit of 4 an hour and is held back once the limit is reached. Critical is never held back and still counts toward the 4, so the follow note is held first.
- `FollowFanoutCap` is 500. The pass for one report refreshes the bells of at most 500 followers. A follower past it is not refreshed by the pass, but if their own bell poll has already written the row, the phone note is still queued for it. The cap bounds the work of one pass and does not promise that nobody past it gets a phone note.
- Mute scope (D-12). A mute stops person level notes only: the follow note and the Reliable reporter row in the bell about that person's reports. It does not stop a Critical note, a change row about a report you posted or voted on, or anything in a feed, map, list or profile.
- Silent like every other note. The site asks for no sound and no vibration, and nothing in the app promises either.

## Official alerts

Phase 5 added a second lane for government alerts. The trust side of it is in TRUST-MODEL.md under "Official alerts on the phone"; the whole design is in [OFFICIAL-ALERTS.md](./OFFICIAL-ALERTS.md). For whoever runs the server:

- Its own outbox and worker. Rows go in the table `official_push_outbox` when a refresh stores an alert version, inside the same database transaction, and the official worker sends them through the same guarded sender and the same answer rules as the report worker. It starts with the report worker, only when the VAPID keys are set, and shuts down with it. Without the VAPID keys no rows are queued either. Report notes and official notes never share a queue.
- Who and when. A Severe or Extreme alert that names the person's home district, or an Extreme alert that covers their whole state. A state wide Severe alert queues nobody. It is checked again just before each send, and nothing is sent from a feed that is not fresh (the alert must have been seen, or the feed checked, in the last 30 minutes).
- Silent. The site asks every official note for no sound and no vibration, like all the others. An iPhone, and some Android settings, can still decide otherwise.
- Words. Closed tables only, for example "Cyclone warning for NTR district" and "Extreme. Issued by IMD." No feed text reaches the lock screen. A tap opens Official alerts (`/alerts`) and nothing else.
- Limits of its own. At most 3 official notes per account in a rolling 6 hours, at most 2000 people per alert, and a life of at most 2 hours. They never count toward the 4 an hour of the report kinds.
- One note per alert series on an Apple device. SACHET sends an Update under a new guid that names the earlier alert, so an alert and its updates are one series, and they share one Topic and Tag. Android and desktop browsers replace the earlier note in place. An Apple endpoint ignores the tag, so it gets only the first push about a series and the rest stays on Official alerts. This is the owner's few notifications rule from 3.3 check 8. It is per account and series, and a device added later, or one whose first push failed, gets nothing more about that series; it only ever means fewer notes.
- A sleeping host sends nothing, and no alert reaches a phone until some request or the outside cron wakes the server and the refresh stores it. The cron is set up in OFFICIAL-ALERTS.md.

## Honest limits

- Sound and vibration. The site asks every notification for no sound and no vibration, and never sets a vibration pattern. It cannot force that. An iPhone follows its own settings, Settings, Notifications, Pinalert, and an Android phone or a computer follows the notification settings the person chose for the browser. Nothing in the app promises silence or a buzz.
- One notification per report on every device. iPhone, and Safari on a Mac, ignore the tag that makes Android and desktop browsers swap an old note for a new one. Apple endpoints therefore get only the first push about a report; later news about it, the confirmed update, is not sent there and stays in the bell. The first note on an iPhone keeps reading Not confirmed yet, and tapping it opens the report, which shows the current state. Android and desktop browsers replace the note in place, silently. The rule is per account and report: once any device of the account got a push about a report, an Apple device gets nothing more about it, and that includes a device added later, because there is no per device send record. A person with an iPhone and another device gets the update on the other device only.
- A sleeping host sends nothing. The dispatcher and the worker run inside the one server process. On a free host that spins down after a quiet spell, no note leaves until some request wakes it, and that includes the fades in about an hour change note. After it wakes, a report older than an hour is no longer pushed as a new report, and a queued note past its expiry is dropped.
- iPhone needs the installed app. A Safari tab on iPhone cannot receive push at all. The person must add Pinalert to the Home Screen (iOS 16.4 or later) and turn Alerts on this device on inside the installed app. Settings shows Add Pinalert to your Home Screen first, with the three steps, until they do.
- An installed iPhone app starts signed out, and that now works. The Home Screen app keeps its own cookies, separate from Safari's, but sign in is no longer an emailed link that opens in Safari (Phase 7.1). The person signs in inside the installed app with the email and password, or with a typed 6 digit code for a new account or a forgotten password, so the sign in never leaves the app. Sign up in a Safari tab ends with a small box showing the three install steps (plan 07.1-13b), and the installed app then asks about alerts once on Nearby (plan 07.1-13c, one ask per device and account). Settings stays the way to change your mind. A Home Screen app can also lose its sign in cookie after a stretch of not being used (reports say about 7 days), while the push subscription itself keeps working until it is removed. A note still arrives; tapping it may land on the sign in page. A Mac Dock app (Safari, File, Add to Dock) starts signed out too, like an installed iPhone app (owner check 10). The Mac's Safari subscription already covers that Mac, so the Dock app is not asked to turn alerts on: a second subscription on one Mac would make every report notify twice.
- Sign out. Plain Log out removes the push subscriptions of the browser that signed out, Log out of all devices removes every one of the account, and Delete my account removes them and the queued notes. The browser keeps a small flag saying the person turned notifications on, tied to their account; it is not cleared on log out. Signing back in to the same account on that browser registers the device again by itself on the first page after signing in (Nearby, Low data or the first-time flow), without opening Settings, only when the browser's kept subscription was made with the current key, and without asking the browser anything. A different account on a shared phone does not match the flag and is not registered.
- A report most nearby people dispute. A Critical report cannot be hidden by disputes, so it stays Live and in the feed. Its notification follows the feed: it is sent while the report is fresh, it still reads Not confirmed yet, and it never gets the confirmed update. Push has no dispute rule of its own (review note N3).
- The fades in about an hour row and the other change rows. 3.2 writes them in the bell for any report a person posted or voted on that is still active, confirmed or not. Since 2026-11-03 (quick 261103-jal, an owner decision) none of them reaches a phone: the dispatcher refreshes the bells of the people of one voted report early and queues no change note, and the bell poll writes every row for everyone else, and a change row that was queued before that date is skipped as not_eligible when the worker reaches it. The worker keeps its older checks for those rows (a report must still show the event, and a now disputed row needs an earlier confirmation), as a second layer behind that skip. A confirmation from someone at a new spot on a report about to fade cancels that fade, so no fading row follows it (backlog 999.12).
- Comment rows (Phase 8). A new comment on your report, or a reply to your comment, reaches the bell as one unread row per kind and report. A person who commented on a report follows it for the bell. Since 2026-11-03 these rows do not reach a phone either; a queued comment note is skipped like the change rows above. A like or a flag writes nothing.
- Hidden for review (quick 261117-vvb, 2026-11-17). A report that flags from three places hid is withheld, so it sends no push of any kind while it is hidden. The rows that tell its author it was hidden, shown again or removed are bell only and never reach a phone or an email. Once a moderator kept it, the report is never pushed again: the Critical and follow branches skip it and a first alert queued before the hide is skipped as not_eligible.
- Counts. A Critical report near the home area always gets through, because it is not kept in the bell and a dropped one would show nowhere, but it counts toward the limit. The follow kind stops at 4 notes in a rolling hour, Critical ones counted, so Critical is held never and counted always. When a follow note is dropped by the cap nothing extra is shown, and the bell still lists it. See TRUST-MODEL.md for the reporter caps.

## How delivery works

The dispatcher reads each report fresh, decides who should get a note, and queues one row per person in a table called the outbox. The worker claims due rows, reads the report, the mute and the home area again, checks that the kind may reach a phone at all, builds the words from the current facts, and sends to each of the person's devices. A note that was right when queued but is wrong now is skipped.

- At least once. A crash, or a lost database write, between a push service accepting a note and the server marking the row sent makes the server send the row again after its lease runs out. On Android and desktop browsers the second send shows as the same notification, replaced through its tag. On an iPhone it may show twice.
- Only one sender per row. A row is claimed with a lease. The worker renews the lease just before it works the row, and only the holder of the current lease may mark the row. So a second server instance never sends a row the first one is sending, though two instances are still not a supported setup.
- Once one device accepted a row, the person's other devices that failed get no retry for that row.
- A rejected device is removed carefully. The server counts answers that point at the device: a 400, a 403, or an endpoint that no longer passes the allowlist. It removes the device only after `PushDeviceFailureLimit` (5) of them in a row, and only when the device's last success is missing or older than `PushDeviceStaleAfter` (7 days). So a push service outage, a network problem on our side, or a mistake in our own setup such as wrong keys cannot wipe devices that worked last week. A 429, a 5xx, a timeout or a network error never counts against a device and is retried later. A push the service accepted resets the count. A 404 or a 410 means the subscription is gone and removes the device at once. Other answers (401, 413 and the rest) end that row as failed and count nothing.
- Retries wait longer each time, honour the push service's own Retry-After, and stop at the report's expiry or after 4 tries.
- Lifetimes. A note waits for an offline device for as long as the report has left, at most an hour for Critical and 30 minutes for the others.

## What the push service can see

The words of a note travel encrypted, end to end between the server and the browser, so Google, Mozilla, Apple and Microsoft cannot read them. Three headers travel in the clear because the push service needs them: the Topic, which is r and the report id (for an official note it is a short hash of the alert series' first guid, so it names no place), the TTL, and the Urgency, which is high for Critical and normal for the rest (for an official note, high for Extreme and normal for Severe). So a push service can see report ids, and how urgent each push is, and when and to which subscription it was sent. It cannot see the title, the body, the area or the distance.

## Owner device checks

None of these have been run. They need real devices and, for two of them, hosting that does not exist yet. They are written as steps for the owner.

0. Before any of them, on the owner's own dev database, after saying yes: `make migrate`, which adds migration 00011 (the push tables and columns), and, from Phase 5, 00012 (the official alert tables) and 00013 (the official outbox and the Settings column), and, from Phase 7.1, 00014 (passwords, usernames, typed codes and sign in limits; it signs out every account that has no password yet, so the owner's own browser signs out), 00015 (drops the old emailed link table) and 00016 (the password epoch on accounts and sessions; it signs nobody out), and, from Phase 8, 00017 (the share token on reports), 00018 (comments, likes, comment flags and the moderator action log) and 00019 (the two comment kinds of the bell and their one unread row cap), and, from Phase 9, 00020 (the profile switch, follows and blocks), 00021 (tapped ratings), 00022 (person mutes), 00023 (the Notify me flag and the follow note kind) and 00024 (the follow kind in the phone outbox; its Down deletes the queued follow rows first). Nothing else touches that database.

1. Desktop, now, on `http://localhost`. Make a throwaway key pair with `go run ./cmd/vapidkeys` and export both values in the shell that starts a scratch server, together with a contact (`VAPID_SUBJECT`). Use a free port and a scratch database, not the dev server. Sign in on Chrome or Safari on the Mac, set a home area, open Settings, Notifications, turn on Alerts on this device and allow it. Close the tab. From a second account, post a Critical report with a pick near that home area. Expect a notification with no sound, titled with the pick and the area, reading Not confirmed yet.; tapping it opens the report on Nearby. The app can be installed from the browser's own install control and shows the white pin icon. Then have two people at different spots confirm it and expect the same notification to change in place to Confirmed by people nearby. This is for Chrome; Safari on a Mac is an Apple endpoint and keeps the first note (Honest limits). Stop the server and drop the scratch database afterwards, and do not keep the throwaway keys.

2. Android, after the https host is chosen. Set the real keys as host secrets. In Chrome, install from the browser's own install control, allow alerts in the pop-up at the end of sign up, or in Settings, close the app, and have a Critical report posted near the home area. Expect a notification without sound or vibration showing the pick, the area, the distance and Not confirmed yet.; after two confirmations it changes in place to Confirmed by people nearby.; a tap opens the report.

3. iPhone, after https. Sign in now works inside the installed app (Phase 7.1: a password or a typed code, no link), so this check can run. In Safari, sign up ends with the install steps box, and Settings shows Add Pinalert to your Home Screen first; follow the three steps, open the installed app and sign in. Nearby then asks about alerts once; tap Allow (or turn on Alerts on this device in Settings). A Critical report near the home area arrives as one notification with the pick, the area, the distance and Not confirmed yet. No second note arrives when it is confirmed. Whether it makes a sound follows the iPhone's own settings.

4. After checks 1 to 3, run the SQL above and compare the hosts with the table. Report any host that is not in it.

5. On the real host, once it exists, leave the host quiet until it sleeps, then post a Critical report and note how long the first notification takes to arrive. The post wakes the host and pokes the dispatcher, so a note should follow within seconds of the host being up.

6. Official alerts, after check 2 or 3 and with the host on https, the cron set up (the GitHub secret `OFFICIAL_REFRESH_TOKEN`, the variable `PINALERT_BASE_URL`, and the same token as a host secret) and the VAPID keys set. With the phone installed and a home area saved in a district that has a Severe or Extreme alert in force, expect one notification with no sound or vibration, titled with the event and the district and reading the level and the issuer; a tap opens Official alerts. Nothing arrives for a state wide Severe alert. If no real alert is in force, the owner may wait for one, because a test alert would have to be written into the database by hand. An iPhone follows its own settings for sound. Record how long the note took after the alert first showed on the feed.

7. Follow note, after check 1 or 2. With two accounts on a scratch server: the first has Public profile on (named Let others follow me before quick 261117-lq0, amended 2026-11-17) and holds the Reliable reporter tag, the second follows the first and switches Notify me on, with On this device on. Post a report from the first account. Expect one notification titled with the category and the area, whose body opens From someone you follow. and ends Not confirmed yet., with no username on the lock screen; a tap opens the report. Then have two people at different spots confirm it and expect the note to change in place on Chrome or Edge, and no second note on an iPhone or Safari. Then switch the bell off from the second account and post again from the first: expect nothing, and expect a note still waiting to be gone. Do not use the owner's dev server or its database.
