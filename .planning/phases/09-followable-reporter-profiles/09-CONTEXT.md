# Phase 9: Followable reporter profiles - Context

**Gathered:** 2026-10-17 and 2026-10-18, from the owner's answers in the discuss round run by session pinalert-60, on top of the owner decisions already written in ROADMAP Phase 9 (2026-10-16 and 2026-10-17).
**Status:** Ready for UI design and planning. How the profile page looks is shown to the owner as a mockup at the UI step, not asked in words. Everything tagged CLAUDE'S DISCRETION stands unless the owner vetoes it at the viewing session. Still the first phase to cut if time runs short.

<domain>
## Phase Boundary

Everyone has their own My profile, Instagram style: picture, username, name, stars, the reporter tag, Followers and Following counts that open their lists, and the reports they posted recently. Other people's profiles are public only when they opt in (D-01); someone who has not opted in appears in lists only as a number and cannot be opened (D-03). People who opt in and hold the Reliable reporter tag can be followed; a follower can ask, with a bell on that person's profile, to be told quietly when they post. Readers can tap one to five stars to rate a reporter, inside guard rails.

Not in this phase: comments and usernames themselves (Phase 8), the notification plumbing (Phases 3.2 and 3.3).

Requirements: PROF-01, PROF-02, PROF-03.
</domain>

<decisions>
## Implementation Decisions

### How to read the status tags
- **OWNER DECIDED**: the owner said it in this round. Do not re-ask.
- **LOCKED EARLIER**: decided in the ROADMAP Phase 9 section or an earlier phase. Do not re-ask.
- **CLAUDE'S DISCRETION**: the owner was not asked. Build the default named here unless the owner vetoes it.

### Carried from the ROADMAP (locked)
- **D-01 [LOCKED EARLIER]:** Profiles are opt in ("Let others follow me", off by default), and only a person with the Reliable reporter tag can be followed. People appear by their Phase 8 username (unique, public); the Name is a profile only detail; the email is never shown.
- **D-02 [LOCKED EARLIER]:** Stars: below 5 raters they come from report outcomes only (the Phase 3 score); from 5 raters on, half outcomes and half the tapped average, both parts shown beside the total. Readers tap one to five stars on a reporter's profile: verified account only, one rating per account per reporter and changeable, never your own, only from accounts that voted on at least one of that reporter's reports, shown only with 5 or more raters. A skippable star box appears right after "Yes, mark resolved". The plan says how a tapped rating is removed or ignored when abuse is found.
- **D-03 [LOCKED EARLIER]:** Followers and Following lists are public (owner chose this over the advice to keep them private). A person without their own opted in profile appears only as a number ("and 3 others"), never by name, and cannot be tapped.
- **D-04 [LOCKED EARLIER]:** A reporter's profile opens from their name in the report's trust block, shown only for opted in reporters. My profile sits in the laptop person menu and on the phone in Settings, About you. Let others follow me, Following, Followers and Blocked people are rows in Settings, People and reports.

### The Reliable reporter tag
- **D-05 [OWNER DECIDED, 2026-10-17, chosen over the advice of 4 stars]:** The Reliable reporter tag starts at **3.5 stars**, on the combined stars of D-02. Stars are the Phase 3 reliability score (0 to 100) divided by 20, shown with one decimal. This loosens today's bar (`ReliableMinScore = 75` in `internal/service/reliability.go`, about 3.75 stars) to 70; the owner was told a few reporters near the line will gain the tag. The other tags keep their meaning: Mixed record from today's `MixedMinScore = 35` (1.75 stars) mapped the same way, and New reporter below 5 judged reports, never shown as a good tag (Phase 3 D-13, D-15).

### Where you see the people you follow
- **D-06 [OWNER DECIDED, 2026-10-17]:** The Instagram way, on profile pages, not on Nearby and not in My activity:
  - Every profile shows **Followers** and **Following** with their counts at the top; tapping either opens the list.
  - Tapping a name in a list opens that person's page with the reports they posted recently. It is the same kind of page as your own profile ("just like how you'll have your profile page and all your posts").
  - Each report on a profile shows its distance from you (SC3).
  This replaces ROADMAP SC3's suggestion of a choice in the Nearby Show menu.
- **D-07 [CLAUDE'S DISCRETION]:** "Recently" covers the last 30 days, newest first: live reports appear as they do on the map; finished ones show at a coarse location (finished Rescue reports keep 03.1 D-23). Tapping a report opens it on the report page (Phase 8).

### Being told when someone you follow posts
- **D-08 [OWNER DECIDED, 2026-10-18]:** A small **bell** sits on a followed person's profile, beside the Following button, as on Instagram. It appears as soon as you follow someone. It is **one on/off: Notify me**, off by default ("It's just one note"). There is no separate buzz option.
- **D-09 [OWNER DECIDED, 2026-10-18]:** With the bell on, you get a quiet note when they post, marked "Not confirmed yet" until people nearby confirm it. This uses the same rule as the Reliable reporter alert, which Phase 3.2 builds as one named predicate (03.2 D-29, owner amendment of 2026-10-18): the note can go as soon as the report is listable (Live or Provisional), never while it is Hidden, Withheld or Retracted, and one note per account and report. On the phone it is one silent notification, updated in place when the report is confirmed (03.3 amendment of 2026-10-18). This replaces ROADMAP SC4's "critical immediately, other reports only once Live".
- **D-10 [OWNER DECIDED, 2026-10-18]:** No buzzes anywhere in the project: every phone notification is silent, no sound and no vibration (iPhone's own settings decide the sound; the site can only ask for silence).
- **D-11 [OWNER ACCEPTED, 2026-10-18]:** The per person bell replaces the general "Someone I follow posts" switch that Phase 3.3 planned for Settings, so the choice lives in one place. Settings keeps the device's Phone notifications on/off switch, which still governs every phone notification.

### Claude's Discretion
- **D-12:** Mute and block. Mute stops every note from that person; their reports still show everywhere, because hiding emergency reports is never safe. Block also stops them following you and removes you from their Following, hides your profile from them, and hides their comments from you (Phase 8); their reports still show in the feed. Each person's profile has a Remove follower action for your own followers, and switching off "Let others follow me" or losing the Reliable tag ends all follows and pending notes at once (SC5). The old "remove all followers without seeing who they are" wording no longer applies, because the lists are public (D-03).
- **D-13:** The profile header in order: round picture, username, name, stars with both parts beside the total (D-02), the reporter tag with its counts ("7 of 9 earlier reports were confirmed by people nearby"), member since, Followers and Following, then Follow (or Following) with the bell beside it. Shown to the owner as a mockup at the UI step; the owner judges by looking.
- **D-14:** Rating abuse: a moderator can remove a tapped rating; ratings from an account later removed for abuse stop counting; burst detection (TRUSTX-03) stays a named dependency, recorded in `docs/TRUST-MODEL.md` with the lower 3.5 bar (D-05) as a reason it matters more.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `.planning/ROADMAP.md` Phase 9 section: goal, SC1 to SC5 (amended here: SC3 placement by D-06, SC4 timing by D-09, the cutoff by D-05), scope notes 1 to 3, owner decisions 2026-10-16 and 2026-10-17.
- `.planning/REQUIREMENTS.md`: PROF-01 to PROF-03.
- `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-CONTEXT.md`: D-10 to D-15 (the reliability score, tags, minimum evidence and decay).
- `internal/service/reliability.go`: `ReliableMinScore`, `MixedMinScore`, `Reliability`.
- `.planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-CONTEXT.md`: the owner amendment of 2026-10-18 and D-29 (the one predicate for notes at posting time).
- `.planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md`: the 2026-10-18 amendment (silent notifications, in place update, the follow switch moved here).
- `.planning/phases/08-report-page-and-comments/08-CONTEXT.md`: usernames (D-08, D-09), comments that block hides (D-12 here).
- `.planning/phases/03.4-app-layout/03.4-CONTEXT.md`: D-25 (Settings sections), the person menu.
- `docs/TRUST-MODEL.md`: where the rating and its limits are documented.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/templates/profile.html.tmpl` and its handler: the current My profile page from Phase 3.4, the starting point for the Instagram style header.
- `internal/service/reliability.go`: the score and tags the stars are built from.
- `web/templates/settings.html.tmpl`: the People and reports section gains the follow rows.

### Integration Points
- The trust block's "See this reporter" link (ROADMAP scope note 2).
- The 3.2 updates pipeline and the 3.3 phone channel (D-09).
- The "Yes, mark resolved" flow for the skippable star box (D-02).

</code_context>

<specifics>
## Specific Ideas

- "they should have like you know how the Instagram works it should work exactly like that"
- "if you tap one of the followers name ... it will take it to their page. So there ... you'll see all the posts they've made recently."
- "a bell or something ... this bell thing should be on the profile page"

</specifics>

<deferred>
## Deferred Ideas

None new in this round.

</deferred>
